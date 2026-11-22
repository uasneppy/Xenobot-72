//go:build testtools

package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/uasneppy/fuku_robot/fuku/db/aispam"
	"github.com/uasneppy/fuku_robot/fuku/db/deletion"
	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
	"github.com/uasneppy/fuku_robot/fuku/utils/formatting"
	"github.com/uasneppy/fuku_robot/fuku/utils/ratelimit"
)

// aispamNoticeEnv is one aispam dispatcher over the screening golden site (real locales, a fresh supergroup,
// member 42 Ann & <Bo> Lee, a 600 s bot:punish_auto rule) with the TypeSafe stand-in answering 0.95, miniredis,
// a fast notice pacer and private copy pacer, a client that answers the member's private chat, and the AI burst
// trigger off for the chat (so seven delete verdicts never start a lockdown whose notice would be counted).
type aispamNoticeEnv struct {
	*botSite
	dispatcher *ext.Dispatcher
	redis      *miniredis.Miniredis
	nextID     int64
}

// aispamNoticeSpamText is the text every member posts; the stand-in judges every message spam.
const aispamNoticeSpamText = "BUY CHEAP COINS NOW t.me/scam"

func newAISpamNoticeEnv(t *testing.T) *aispamNoticeEnv {
	t.Helper()
	resetAISpamRuntime()
	t.Cleanup(resetAISpamRuntime)
	withAISpamTestConfig(t, "typesafe-test-key", true, 0)
	withAISpamJevServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, aispamDecisionBody(0.95, "scam"))
	})
	mr := withMiniredisServer(t)
	withFastPMCopyPacer(t)
	withFastAISpamNoticePacer(t)
	mreqSwap(t, &raidSettingsLookup, func(context.Context, int64) (*models.ChatRaidDetection, error) {
		return &models.ChatRaidDetection{AIEnabled: false, AIThreshold: 5}, nil
	})
	s := newScreenGoldenSite(t)
	s.bot.BotClient = eventPMTestClient{BotClient: s.bot.BotClient, script: &eventPMScript{}}
	eventPMCleanup(t, s.chat.Id)
	t.Cleanup(pmCopyJobs.wait)
	if err := aispam.SetAISpamEnabled(s.chat.Id, true); err != nil {
		t.Fatalf("SetAISpamEnabled() error = %v", err)
	}
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadAISpam(dispatcher)
	return &aispamNoticeEnv{botSite: s, dispatcher: dispatcher, redis: mr, nextID: 7100}
}

// withFastAISpamNoticePacer installs a notice pacer built from the production options with a 1 ms interval (the
// 2 s MaxWait kept, so a block a minute away still fails at once) and puts the previous one back.
func withFastAISpamNoticePacer(t *testing.T) {
	t.Helper()
	opts := aispamNoticePacerOptions
	opts.Interval = time.Millisecond
	opts.RetryAfterUnit = 5 * time.Millisecond
	mreqSwap(t, &aispamNoticePacer, ratelimit.NewTelegramPacer(opts))
}

// spam posts one spam message of from (in topic thread when it is not 0) through the dispatcher and waits for the
// check, and returns the message's ID.
func (e *aispamNoticeEnv) spam(t *testing.T, from gotgbot.User, thread int64) int64 {
	t.Helper()
	e.nextID++
	update := aispamMessageUpdate(e.chat, from, e.nextID, aispamNoticeSpamText)
	if thread != 0 {
		update.Message.IsTopicMessage = true
		update.Message.MessageThreadId = thread
	}
	aispamProcessRawUpdate(t, e.dispatcher, e.bot, update)
	waitAISpamChecks()
	return e.nextID
}

// turnOn switches the group's AI spam notice on, as the Messages page's Notice switch does (Silent false).
func (e *aispamNoticeEnv) turnOn(t *testing.T) {
	t.Helper()
	if err := eventmsg.SetSilent(t.Context(), e.chat.Id, eventmsg.EventAISpam, eventmsg.DestGroup, false, 1); err != nil {
		t.Fatalf("SetSilent(aispam, group, false) error = %v", err)
	}
}

// notices are the sends to the group (the dump mirror is off, so every one is a notice), after the jobs.
func (e *aispamNoticeEnv) notices(t *testing.T) []eventPMCall {
	t.Helper()
	group, _ := eventPMCalls(t, e.botSite)
	return group
}

// wantDeleted fails unless every message ID was deleted exactly once, in the group.
func (e *aispamNoticeEnv) wantDeleted(t *testing.T, ids ...int64) {
	t.Helper()
	deleted := map[string]int{}
	for _, call := range e.client.callsFor("deleteMessage") {
		if fmt.Sprint(call.Params["chat_id"]) != strconv.FormatInt(e.chat.Id, 10) {
			t.Fatalf("deleteMessage in chat %v, want %d", call.Params["chat_id"], e.chat.Id)
		}
		deleted[fmt.Sprint(call.Params["message_id"])]++
	}
	for _, id := range ids {
		if n := deleted[strconv.FormatInt(id, 10)]; n != 1 {
			t.Fatalf("deletes of message %d = %d, want one: every spam message is deleted", id, n)
		}
	}
}

// noticeKeys are the fuku:aispam:notice:* keys in miniredis.
func (e *aispamNoticeEnv) noticeKeys() []string {
	var keys []string
	for _, key := range e.redis.Keys() {
		if strings.HasPrefix(key, "fuku:aispam:notice:") {
			keys = append(keys, key)
		}
	}
	return keys
}

// aispamNoticeMembers returns n fresh members with plain names.
func aispamNoticeMembers(n int) []gotgbot.User {
	members := make([]gotgbot.User, n)
	for i := range members {
		members[i] = gotgbot.User{Id: uniqueLinkOwnerID(), FirstName: "Spammer" + strconv.Itoa(i)}
	}
	return members
}

// aispamNoticeCancelClient cancels the /aispam pool's context right after the delete went through, as a shutdown
// drain that lands between the deletion and the notice would.
type aispamNoticeCancelClient struct {
	gotgbot.BotClient
}

func (c aispamNoticeCancelClient) RequestWithContext(ctx context.Context, token string, method string, params map[string]any, opts *gotgbot.RequestOpts) (json.RawMessage, error) {
	response, err := c.BotClient.RequestWithContext(ctx, token, method, params, opts)
	if method == "deleteMessage" {
		aispamCancel()
	}
	return response, err
}

func TestAISpamNotice(t *testing.T) {
	withModerationGoldenLocales(t)
	tr := i18n.MustNewTranslator("uk")
	member := noticeGoldenTarget

	// off by default: nothing stored, so the message is deleted, nothing is posted, nothing is queued, and the
	// notice spends no Redis command (no fuku:aispam:notice:* key exists).
	t.Run("off by default", func(t *testing.T) {
		e := newAISpamNoticeEnv(t)
		id := e.spam(t, member, 0)
		e.wantDeleted(t, id)
		if got := e.notices(t); len(got) != 0 {
			t.Fatalf("group sends = %d, want none: the notice is off by default", len(got))
		}
		e.wantNoRows(e.chat.Id)
		if keys := e.noticeKeys(); len(keys) != 0 {
			t.Fatalf("notice keys = %v, want none", keys)
		}
	})

	// default on: a spam message in a topic is deleted, then one notice with the default text naming the member is
	// posted in that topic, never a reply, and queued under bot:punish_auto at 600 s.
	t.Run("default on", func(t *testing.T) {
		e := newAISpamNoticeEnv(t)
		e.turnOn(t)
		id := e.spam(t, member, 77)
		e.wantDeleted(t, id)
		notices := e.notices(t)
		if len(notices) != 1 {
			t.Fatalf("group sends = %d, want the one notice", len(notices))
		}
		methods := eventMsgMethods(e.client)
		if deleted, sent := eventMsgIndex(methods, "deleteMessage"), eventMsgIndex(methods, "sendMessage"); deleted < 0 || deleted > sent {
			t.Fatalf("calls = %v, want the deletion before the notice", methods)
		}
		params := eventPMParams(t, notices[0])
		want, _ := tr.GetString("aispam_notice_deleted", i18n.TranslationParams{
			"name": formatting.MentionHtml(member.Id, formatting.GetFullName(member.FirstName, member.LastName)),
		})
		if params["text"] != want {
			t.Fatalf("notice text = %q, want %q", params["text"], want)
		}
		if fmt.Sprint(params["message_thread_id"]) != "77" {
			t.Fatalf("notice message_thread_id = %v, want the deleted message's topic 77", params["message_thread_id"])
		}
		if _, ok := params["reply_parameters"]; ok {
			t.Fatalf("notice reply_parameters = %v, want none: the notice is never a reply", params["reply_parameters"])
		}
		e.wantRow(e.chat.Id, 9001, deletion.KindBotAuto, 600*time.Second)
	})

	// custom: the group's stored text, filled with the member's escaped name and mention (the username, as every
	// Welcome placeholder reads it), the delete word and the AI reason.
	t.Run("custom", func(t *testing.T) {
		e := newAISpamNoticeEnv(t)
		if err := eventmsg.SetText(t.Context(), e.chat.Id, eventmsg.EventAISpam, eventmsg.DestGroup, "{fullname} ({mention}) {action}: {reason}", 1); err != nil {
			t.Fatalf("SetText(aispam) error = %v", err)
		}
		e.turnOn(t)
		id := e.spam(t, member, 0)
		e.wantDeleted(t, id)
		notices := e.notices(t)
		if len(notices) != 1 {
			t.Fatalf("group sends = %d, want the one custom notice", len(notices))
		}
		want := html.EscapeString(formatting.GetFullName(member.FirstName, member.LastName)) + " (@" + member.Username + ") " +
			html.EscapeString(eventActionWord(tr, "delete")) + ": " + html.EscapeString(aispamEventReason(tr))
		if got := eventPMParams(t, notices[0])["text"]; got != want {
			t.Fatalf("custom notice text = %q, want %q", got, want)
		}
		e.wantRow(e.chat.Id, 9001, deletion.KindBotAuto, 600*time.Second)
	})

	// caps: with the minute clock stopped, one member's second spam inside five minutes is deleted with no second
	// notice; five other members then each post one spam: all are deleted and exactly two more notices are posted.
	t.Run("caps", func(t *testing.T) {
		e := newAISpamNoticeEnv(t)
		e.turnOn(t)
		minute := time.Date(2026, 11, 22, 12, 0, 5, 0, time.UTC)
		mreqSwap(t, &aispamNoticeNow, func() time.Time { return minute })

		ids := []int64{e.spam(t, member, 0), e.spam(t, member, 0)}
		if got := len(e.notices(t)); got != 1 {
			t.Fatalf("notices after one member's two spams = %d, want one", got)
		}
		for _, other := range aispamNoticeMembers(5) {
			ids = append(ids, e.spam(t, other, 0))
		}
		e.wantDeleted(t, ids...)
		if got := len(e.notices(t)); got != aispamNoticePerMinute {
			t.Fatalf("notices in one minute = %d, want the group's cap of %d", got, aispamNoticePerMinute)
		}
	})

	// quiet failures: with the caps' Redis client taken away, with the notice pacer blocked a minute ahead and with
	// the delete refused by a 403, every spam is handled as before and no notice is posted.
	t.Run("quiet failures", func(t *testing.T) {
		e := newAISpamNoticeEnv(t)
		e.turnOn(t)
		members := aispamNoticeMembers(3)

		previous := aispamNoticeRedisClient
		aispamNoticeRedisClient = func() *redis.Client { return nil }
		noRedis := e.spam(t, members[0], 0)
		aispamNoticeRedisClient = previous
		e.wantDeleted(t, noRedis)
		if got := len(e.notices(t)); got != 0 {
			t.Fatalf("notices without Redis = %d, want none (fail closed)", got)
		}

		if err := e.redis.Set(aispamNoticePacerOptions.BlockKey, "1"); err != nil {
			t.Fatalf("set the pacer block: %v", err)
		}
		e.redis.SetTTL(aispamNoticePacerOptions.BlockKey, time.Minute)
		blocked := e.spam(t, members[1], 0)
		e.redis.Del(aispamNoticePacerOptions.BlockKey)
		e.wantDeleted(t, noRedis, blocked)
		if got := len(e.notices(t)); got != 0 {
			t.Fatalf("notices with the pacer blocked = %d, want none", got)
		}

		e.client.mu.Lock()
		e.client.errors["deleteMessage"] = &gotgbot.TelegramError{Method: "deleteMessage", Code: 403, Description: "Forbidden: not enough rights"}
		e.client.mu.Unlock()
		e.spam(t, members[2], 0)
		if got := len(e.client.callsFor("deleteMessage")); got != 3 {
			t.Fatalf("delete attempts = %d, want three (the refused one included)", got)
		}
		if got := len(e.notices(t)); got != 0 {
			t.Fatalf("notices after a refused delete = %d, want none", got)
		}
		e.wantNoRows(e.chat.Id)
	})

	// shutdown: the pool's context is cancelled right after the delete went through; the message is deleted and no
	// notice is posted. The pool is restored afterwards.
	t.Run("shutdown", func(t *testing.T) {
		e := newAISpamNoticeEnv(t)
		restoreAISpamPool(t)
		e.turnOn(t)
		e.bot.BotClient = aispamNoticeCancelClient{BotClient: e.bot.BotClient}
		id := e.spam(t, member, 0)
		e.wantDeleted(t, id)
		if got := len(e.notices(t)); got != 0 {
			t.Fatalf("notices after the shutdown cancelled the pool = %d, want none", got)
		}
		if keys := e.noticeKeys(); len(keys) != 0 {
			t.Fatalf("notice keys = %v, want none: a cancelled check spends no cap", keys)
		}
	})

	// private copy: the notice and its private copy on; after the notice the member receives the default aispam
	// private copy with the AI reason line. A second spam inside five minutes is deleted with no notice and no copy.
	t.Run("private copy", func(t *testing.T) {
		e := newAISpamNoticeEnv(t)
		e.turnOn(t)
		storeEventPM(t, e.chat.Id, eventmsg.EventAISpam, "", true)
		first := e.spam(t, member, 0)
		want := eventPMAutoDefault(t, "eventmsg_pm_aispam", i18n.TranslationParams{
			"group": html.EscapeString(e.chat.Title), "action": eventActionWord(tr, "delete"),
		}, html.EscapeString(aispamEventReason(tr)))
		eventPMAutoWant(t, e.botSite, want)

		second := e.spam(t, member, 0)
		e.wantDeleted(t, first, second)
		eventPMAutoWant(t, e.botSite, want) // still exactly one private copy
		if got := len(e.notices(t)); got != 1 {
			t.Fatalf("notices = %d, want the first one only", got)
		}
	})
}
