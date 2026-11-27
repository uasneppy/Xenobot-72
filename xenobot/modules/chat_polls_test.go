//go:build testtools

package modules

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/uasneppy/Xenobot-72/xenobot/db/chatconv"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/db/presence"
	utilsCache "github.com/uasneppy/Xenobot-72/xenobot/utils/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/chatprompt"
)

type chatPollWireCall struct {
	Method string
	Params map[string]string
}

type chatPollEnv struct {
	t          *testing.T
	chatID     int64
	mr         *miniredis.Miniredis
	rc         *redis.Client
	bot        *gotgbot.Bot
	dispatcher *ext.Dispatcher
	provider   *chatFakeProvider

	mu           sync.Mutex
	convSettings chatconv.Settings
	presSettings presence.Settings
	staffGroup   *models.StaffGroup
	lockdown     *models.ChatLockdown
	lockdownErr  error
	admins       map[int64]bool
	approved     map[int64]bool
	sendMsgErr   string
	calls        []chatPollWireCall
	nextUpdateID int64
	nextMsgID    int64
	nextSentID   int64
	nextPollID   int64
	blockSend    chan struct{}
}

func newChatPollEnv(t *testing.T) *chatPollEnv {
	t.Helper()
	withRealLocales(t)
	utilsCache.SetupTestMemoryMarshaler(t)
	withFastPresencePacer(t)
	mr, rc := withChatRedis(t)
	withChatRuntime(t)
	withChatProviders(t, true, true, true)
	withChatFeatherless(t, "test-featherless-secret-key", "Qwen/Qwen3-32B")

	chatSwap(t, &chatToyChatCooldown, chatToyChatCooldown)
	chatSwap(t, &chatToyUserCooldown, chatToyUserCooldown)
	chatSwap(t, &chatToyUserHourlyBudget, chatToyUserHourlyBudget)
	chatSwap(t, &chatToyChatHourlyBudget, chatToyChatHourlyBudget)
	chatSwap(t, &chatToyGlobalHourlyBudget, chatToyGlobalHourlyBudget)

	env := &chatPollEnv{
		t:      t,
		chatID: uniqueModuleChatID(),
		mr:     mr,
		rc:     rc,
		convSettings: chatconv.Settings{
			Enabled:    true,
			LogEnabled: true,
			Provider:   chatconv.ProviderAnthropic,
			Audience:   chatconv.AudienceEveryone,
		},
		presSettings: presence.Settings{
			ReferendumEnabled: true,
		},
		admins:       map[int64]bool{},
		approved:     map[int64]bool{},
		provider:     &chatFakeProvider{},
		nextUpdateID: 1,
		nextMsgID:    1000,
		nextSentID:   9000,
		nextPollID:   500,
	}

	chatSwap(t, &presenceStaffLookup, func(int64) *models.StaffGroup {
		env.mu.Lock()
		defer env.mu.Unlock()
		return env.staffGroup
	})
	chatSwap(t, &presenceLockdownLookup, func(int64) (*models.ChatLockdown, error) {
		env.mu.Lock()
		defer env.mu.Unlock()
		return env.lockdown, env.lockdownErr
	})
	chatSwap(t, &chatSettingsLookup, func(_ context.Context, _ int64) (chatconv.Settings, error) {
		env.mu.Lock()
		defer env.mu.Unlock()
		return env.convSettings, nil
	})
	chatSwap(t, &presenceSettingsGet, func(_ context.Context, _ int64) (presence.Settings, error) {
		env.mu.Lock()
		defer env.mu.Unlock()
		return env.presSettings, nil
	})
	chatSwap(t, &chatAudienceAdmin, func(_ *gotgbot.Bot, _ int64, userID int64) bool {
		env.mu.Lock()
		defer env.mu.Unlock()
		return env.admins[userID]
	})
	chatSwap(t, &chatAudienceApproved, func(_ context.Context, _ int64, userID int64) (bool, error) {
		env.mu.Lock()
		defer env.mu.Unlock()
		return env.approved[userID], nil
	})
	chatSwap(t, &chatProviderOf, func(string) chatProvider { return env.provider })

	env.bot = newWireTestBot(t, env.handleTelegram)
	env.bot.User = gotgbot.User{Id: 999, IsBot: true, FirstName: "Xenobot-72", Username: "xenobot72"}

	env.dispatcher = ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadChat(env.dispatcher)
	LoadPresence(env.dispatcher)
	return env
}

func (e *chatPollEnv) handleTelegram(w http.ResponseWriter, r *http.Request) {
	method := strings.TrimPrefix(r.URL.Path, "/bottest/")
	params, _ := decodeTelegramWireParams(r)

	e.mu.Lock()
	e.calls = append(e.calls, chatPollWireCall{Method: method, Params: params})
	sendErr := e.sendMsgErr
	block := e.blockSend
	e.mu.Unlock()

	if block != nil && (method == "sendMessage" || method == "sendPoll") {
		select {
		case <-r.Context().Done():
			return
		case <-block:
		}
	}

	w.Header().Set("Content-Type", "application/json")
	switch method {
	case "sendPoll":
		e.mu.Lock()
		e.nextSentID++
		msgID := e.nextSentID
		e.nextPollID++
		pollID := fmt.Sprintf("poll-%d", e.nextPollID)
		e.mu.Unlock()
		_, _ = fmt.Fprintf(
			w,
			`{"ok":true,"result":{"message_id":%d,"date":1,"chat":{"id":%d,"type":"supergroup"},"poll":{"id":%q,"question":"Q","options":[{"text":"A","voter_count":0},{"text":"B","voter_count":0}],"is_closed":false,"is_anonymous":false,"type":"regular"}}}`,
			msgID, e.chatID, pollID,
		)
	case "sendMessage":
		if sendErr != "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintf(w, `{"ok":false,"error_code":400,"description":%q}`, sendErr)
			return
		}
		e.mu.Lock()
		e.nextSentID++
		msgID := e.nextSentID
		e.mu.Unlock()
		_, _ = fmt.Fprintf(w, `{"ok":true,"result":{"message_id":%d,"date":1,"chat":{"id":%d,"type":"supergroup"},"text":"ok"}}`, msgID, e.chatID)
	default:
		_, _ = io.WriteString(w, `{"ok":true,"result":true}`)
	}
}

func (e *chatPollEnv) callsFor(method string) []chatPollWireCall {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out []chatPollWireCall
	for _, c := range e.calls {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

func (e *chatPollEnv) member(name string) *gotgbot.User {
	return &gotgbot.User{Id: uniqueLinkOwnerID(), FirstName: name}
}

func (e *chatPollEnv) message(from *gotgbot.User, threadID int64, text string) *gotgbot.Message {
	e.nextMsgID++
	msg := &gotgbot.Message{
		MessageId: e.nextMsgID,
		Date:      time.Now().Unix(),
		From:      from,
		Text:      text,
		Chat:      gotgbot.Chat{Id: e.chatID, Type: "supergroup", Title: "Facility 7"},
	}
	if threadID != 0 {
		msg.IsTopicMessage = true
		msg.MessageThreadId = threadID
	}
	return msg
}

func (e *chatPollEnv) referendumCmd(from *gotgbot.User, threadID int64, arg string, reply *gotgbot.Message) *gotgbot.Message {
	text := "/referendum"
	if arg != "" {
		text += " " + arg
	}
	msg := e.message(from, threadID, text)
	msg.Entities = []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: int64(len("/referendum"))}}
	msg.ReplyToMessage = reply
	return msg
}

func (e *chatPollEnv) deliverMessage(msg *gotgbot.Message) {
	e.t.Helper()
	e.nextUpdateID++
	if err := e.dispatcher.ProcessUpdate(e.bot, &gotgbot.Update{UpdateId: e.nextUpdateID, Message: msg}, nil); err != nil {
		e.t.Fatalf("ProcessUpdate(Message): %v", err)
	}
	chatJobs.wait()
}

func (e *chatPollEnv) deliverPoll(p *gotgbot.Poll) {
	e.t.Helper()
	e.nextUpdateID++
	if err := e.dispatcher.ProcessUpdate(e.bot, &gotgbot.Update{UpdateId: e.nextUpdateID, Poll: p}, nil); err != nil {
		e.t.Fatalf("ProcessUpdate(Poll): %v", err)
	}
	chatJobs.wait()
}

func unquoteWireParam(raw string) string {
	var s string
	if err := json.Unmarshal([]byte(raw), &s); err == nil {
		return s
	}
	return raw
}

func TestReferendumDoubleGateAndSafety(t *testing.T) {
	env := newChatPollEnv(t)
	alice := env.member("Alice")
	admin := env.member("Admin")
	approved := env.member("Approved")
	env.admins[admin.Id] = true
	env.approved[approved.Id] = true

	t.Run("private chat refused", func(t *testing.T) {
		beforeMsgs := len(env.callsFor("sendMessage"))
		priv := env.referendumCmd(alice, 0, "cats vs dogs", nil)
		priv.Chat = gotgbot.Chat{Id: alice.Id, Type: "private"}
		env.deliverMessage(priv)
		if got := len(env.callsFor("sendPoll")); got != 0 {
			t.Fatalf("sendPoll calls = %d, want 0", got)
		}
		if got := len(env.provider.calls()); got != 0 {
			t.Fatalf("provider calls = %d, want 0", got)
		}
		msgs := env.callsFor("sendMessage")
		if len(msgs) != beforeMsgs+1 {
			t.Fatalf("sendMessage calls = %d, want %d", len(msgs), beforeMsgs+1)
		}
	})

	t.Run("staff group silent", func(t *testing.T) {
		beforeMsgs := len(env.callsFor("sendMessage"))
		env.mu.Lock()
		env.staffGroup = &models.StaffGroup{ChatID: env.chatID}
		env.mu.Unlock()
		env.deliverMessage(env.referendumCmd(alice, 0, "tea or coffee", nil))
		env.mu.Lock()
		env.staffGroup = nil
		env.mu.Unlock()

		if len(env.callsFor("sendMessage")) != beforeMsgs || len(env.callsFor("sendPoll")) != 0 || len(env.provider.calls()) != 0 {
			t.Fatal("staff group should stay completely silent")
		}
	})

	t.Run("lockdown silent", func(t *testing.T) {
		beforeMsgs := len(env.callsFor("sendMessage"))
		env.mu.Lock()
		env.lockdown = &models.ChatLockdown{ChatID: env.chatID}
		env.mu.Unlock()
		env.deliverMessage(env.referendumCmd(alice, 0, "tea or coffee", nil))
		env.mu.Lock()
		env.lockdown = nil
		env.mu.Unlock()

		if len(env.callsFor("sendMessage")) != beforeMsgs || len(env.callsFor("sendPoll")) != 0 || len(env.provider.calls()) != 0 {
			t.Fatal("locked-down group should stay completely silent")
		}
	})

	t.Run("owner conversations off answers off hint", func(t *testing.T) {
		beforeMsgs := len(env.callsFor("sendMessage"))
		env.mu.Lock()
		env.convSettings.Enabled = false
		env.mu.Unlock()
		env.deliverMessage(env.referendumCmd(alice, 0, "tea or coffee", nil))
		env.mu.Lock()
		env.convSettings.Enabled = true
		env.mu.Unlock()

		msgs := env.callsFor("sendMessage")
		if len(msgs) != beforeMsgs+1 {
			t.Fatalf("sendMessage calls = %d, want %d", len(msgs), beforeMsgs+1)
		}
		if got := unquoteWireParam(msgs[len(msgs)-1].Params["text"]); got != chatCanned(t, "referendum_off_hint") {
			t.Fatalf("reply = %q, want %q", got, chatCanned(t, "referendum_off_hint"))
		}
		if len(env.provider.calls()) != 0 {
			t.Fatal("expected zero provider calls when owner Conversations is off")
		}
	})

	t.Run("admin referendum toggle off answers off hint", func(t *testing.T) {
		beforeMsgs := len(env.callsFor("sendMessage"))
		env.mu.Lock()
		env.presSettings.ReferendumEnabled = false
		env.mu.Unlock()
		env.deliverMessage(env.referendumCmd(alice, 0, "tea or coffee", nil))
		env.mu.Lock()
		env.presSettings.ReferendumEnabled = true
		env.mu.Unlock()

		msgs := env.callsFor("sendMessage")
		if len(msgs) != beforeMsgs+1 {
			t.Fatalf("sendMessage calls = %d, want %d", len(msgs), beforeMsgs+1)
		}
		if got := unquoteWireParam(msgs[len(msgs)-1].Params["text"]); got != chatCanned(t, "referendum_off_hint") {
			t.Fatalf("reply = %q, want %q", got, chatCanned(t, "referendum_off_hint"))
		}
		if len(env.provider.calls()) != 0 {
			t.Fatal("expected zero provider calls when admin ReferendumEnabled is off")
		}
	})

	t.Run("audience admins approved blocks non-approved silently and allows admin or approved", func(t *testing.T) {
		chatSwap(t, &chatToyChatCooldown, 0)
		chatSwap(t, &chatToyUserCooldown, 0)
		env.mu.Lock()
		env.convSettings.Audience = chatconv.AudienceAdminsApproved
		env.mu.Unlock()

		beforeMsgs := len(env.callsFor("sendMessage"))
		env.deliverMessage(env.referendumCmd(alice, 0, "tea or coffee", nil))
		if len(env.callsFor("sendMessage")) != beforeMsgs || len(env.callsFor("sendPoll")) != 0 || len(env.provider.calls()) != 0 {
			t.Fatal("non-approved member should be blocked silently by AudienceAdminsApproved")
		}

		env.provider.script(
			chatAnswerText(`{"question":"Best shift snack?","options":["Tempe","Coffee"]}`),
			chatAnswerText(`{"question":"Backup generator test?","options":["Now","Later"]}`),
		)
		env.deliverMessage(env.referendumCmd(admin, 0, "snack debate", nil))
		env.deliverMessage(env.referendumCmd(approved, 0, "generator debate", nil))
		if got := len(env.callsFor("sendPoll")); got != 2 {
			t.Fatalf("sendPoll calls for admin + approved = %d, want 2", got)
		}
		env.mu.Lock()
		env.convSettings.Audience = chatconv.AudienceEveryone
		env.mu.Unlock()
	})

	t.Run("missing context replies need_context and spends nothing", func(t *testing.T) {
		env.mr.FlushAll()
		beforeCalls := len(env.provider.calls())
		beforePolls := len(env.callsFor("sendPoll"))
		beforeMsgs := len(env.callsFor("sendMessage"))

		env.deliverMessage(env.referendumCmd(alice, 0, "", nil))

		if len(env.provider.calls()) != beforeCalls {
			t.Fatal("missing context must make zero provider calls")
		}
		if len(env.callsFor("sendPoll")) != beforePolls {
			t.Fatal("missing context must send zero polls")
		}
		msgs := env.callsFor("sendMessage")
		if len(msgs) != beforeMsgs+1 {
			t.Fatalf("sendMessage calls = %d, want %d", len(msgs), beforeMsgs+1)
		}
		if got := unquoteWireParam(msgs[len(msgs)-1].Params["text"]); got != chatCanned(t, "referendum_need_context") {
			t.Fatalf("reply = %q, want %q", got, chatCanned(t, "referendum_need_context"))
		}
		if keys := env.mr.Keys(); len(keys) != 0 {
			t.Fatalf("missing context wrote Redis keys %v, want none", keys)
		}
	})

	t.Run("malformed JSON answers malfunction without tripping breaker", func(t *testing.T) {
		chatSwap(t, &chatToyChatCooldown, 0)
		chatSwap(t, &chatToyUserCooldown, 0)
		beforeMsgs := len(env.callsFor("sendMessage"))
		env.provider.script(chatAnswerText("not json at all"))
		env.deliverMessage(env.referendumCmd(alice, 0, "topic here", nil))

		msgs := env.callsFor("sendMessage")
		if len(msgs) != beforeMsgs+1 {
			t.Fatalf("sendMessage calls = %d, want %d", len(msgs), beforeMsgs+1)
		}
		if got := unquoteWireParam(msgs[len(msgs)-1].Params["text"]); got != chatCanned(t, "referendum_malfunction") {
			t.Fatalf("reply = %q, want %q", got, chatCanned(t, "referendum_malfunction"))
		}
		assertChatBreakersClean(t)
	})
}

func TestReferendumPollFlowAndTopicIsolation(t *testing.T) {
	env := newChatPollEnv(t)
	alice := env.member("Alice")
	bob := env.member("Bob")

	// Seed topic 10 and topic 20 messages via chatLogWatcher.
	t10First := env.message(alice, 10, "Topic10-pineapple belongs on pizza")
	t20Other := env.message(bob, 20, "Topic20-secret-unrelated-topic")
	t10ReplyTarget := env.message(bob, 10, "Topic10-never put fruit on dough")
	env.deliverMessage(t10First)
	env.deliverMessage(t20Other)
	env.deliverMessage(t10ReplyTarget)

	env.provider.script(chatAnswerText(`{"question":"Pineapple on https://evil.example/pizza @bob?","options":["Yes @alice","No https://t.me/spam","Maybe"]}`))
	cmd := env.referendumCmd(alice, 10, "settle pizza law", t10ReplyTarget)
	env.deliverMessage(cmd)

	calls := env.provider.calls()
	if len(calls) != 1 {
		t.Fatalf("provider calls = %d, want 1", len(calls))
	}
	req := calls[0]
	if req.MaxTokens != chatprompt.ReferendumMaxTokens {
		t.Fatalf("MaxTokens = %d, want %d", req.MaxTokens, chatprompt.ReferendumMaxTokens)
	}
	if !strings.Contains(req.User, "Topic10-pineapple belongs on pizza") {
		t.Errorf("request User missing topic 10 context:\n%s", req.User)
	}
	if !strings.Contains(req.User, "Topic10-never put fruit on dough") {
		t.Errorf("request User missing replied debate message:\n%s", req.User)
	}
	if !strings.Contains(req.User, "settle pizza law") {
		t.Errorf("request User missing command argument:\n%s", req.User)
	}
	if strings.Contains(req.User, "Topic20-secret-unrelated-topic") {
		t.Fatalf("topic isolation violated — topic 20 leaked into topic 10 request:\n%s", req.User)
	}

	polls := env.callsFor("sendPoll")
	if len(polls) != 1 {
		t.Fatalf("sendPoll calls = %d, want 1", len(polls))
	}
	wire := polls[0].Params
	if wire["message_thread_id"] != "10" {
		t.Errorf("message_thread_id = %q, want 10", wire["message_thread_id"])
	}
	if wire["disable_notification"] != "true" {
		t.Errorf("disable_notification = %q, want true", wire["disable_notification"])
	}
	if wire["open_period"] != strconv.FormatInt(chatReferendumOpenPeriod, 10) {
		t.Errorf("open_period = %q, want %d", wire["open_period"], chatReferendumOpenPeriod)
	}
	if wire["allow_adding_options"] != "" && wire["allow_adding_options"] != "false" {
		t.Errorf("allow_adding_options = %q, want omitted or false", wire["allow_adding_options"])
	}
	if wire["is_anonymous"] != "false" {
		t.Errorf("is_anonymous = %q, want false", wire["is_anonymous"])
	}
	q := unquoteWireParam(wire["question"])
	if strings.Contains(q, "https://") || strings.Contains(q, "@bob") {
		t.Fatalf("unsanitized question sent in poll: %q", q)
	}
	if !strings.Contains(q, "[link removed]") || !strings.Contains(q, "@\u200bbob") {
		t.Fatalf("question missing expected sanitization: %q", q)
	}
	optsJSON := wire["options"]
	if strings.Contains(optsJSON, "https://") || strings.Contains(optsJSON, "@alice") {
		t.Fatalf("unsanitized options sent in poll: %s", optsJSON)
	}

	// Verify dedicated toy:b budget keys (isolated from b, brief:b, react:b).
	keys := strings.Join(env.mr.Keys(), " ")
	for _, wantSub := range []string{"xenobot:chat:toy:b:u:", "xenobot:chat:toy:b:c:", "xenobot:chat:toy:b:g:"} {
		if !strings.Contains(keys, wantSub) {
			t.Errorf("Redis missing budget key prefix %q in %q", wantSub, keys)
		}
	}

	// Verify poll metadata persisted at xenobot:presence:poll:<poll_id> with TTL > OpenPeriod (25h).
	pollKey := presencePollKey("poll-501")
	rawMeta, err := env.mr.Get(pollKey)
	if err != nil {
		t.Fatalf("missing Redis poll metadata %s: %v", pollKey, err)
	}
	var meta chatPollMeta
	if err := json.Unmarshal([]byte(rawMeta), &meta); err != nil {
		t.Fatalf("unmarshal poll metadata: %v", err)
	}
	if meta.ChatID != env.chatID || meta.ThreadID != 10 || meta.MessageID != 9001 || meta.Question != q {
		t.Fatalf("unexpected poll metadata: %+v", meta)
	}
	if ttl := env.mr.TTL(pollKey); ttl <= time.Duration(chatReferendumOpenPeriod)*time.Second || ttl > chatPollMetaTTL {
		t.Fatalf("poll metadata TTL = %v, want > %ds and <= %v", ttl, chatReferendumOpenPeriod, chatPollMetaTTL)
	}

	// Per-chat cooldown blocks another member in the same chat within 30s.
	env.deliverMessage(env.referendumCmd(bob, 10, "another debate", nil))
	if len(env.provider.calls()) != 1 || len(env.callsFor("sendPoll")) != 1 {
		t.Fatalf("per-chat cooldown failed: provider=%d sendPoll=%d", len(env.provider.calls()), len(env.callsFor("sendPoll")))
	}

	if stats := chatStatsSection(); !strings.Contains(stats, "<b>Toys:</b>") {
		t.Fatalf("chatStatsSection missing Toys section: %q", stats)
	}
}

func TestPollTieBreaker(t *testing.T) {
	seedPollMeta := func(t *testing.T, env *chatPollEnv, pollID string, msgID, threadID int64) {
		t.Helper()
		meta := chatPollMeta{
			ChatID:    env.chatID,
			MessageID: msgID,
			ThreadID:  threadID,
			Lang:      "en",
			Question:  "Coffee or Tea?",
		}
		raw, err := json.Marshal(meta)
		if err != nil {
			t.Fatal(err)
		}
		if err := env.mr.Set(presencePollKey(pollID), string(raw)); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("ignores unknown, open, zero-vote, and non-tied polls", func(t *testing.T) {
		env := newChatPollEnv(t)
		seedPollMeta(t, env, "poll-known", 701, 10)

		cases := []*gotgbot.Poll{
			{Id: "poll-unknown", IsClosed: true, TotalVoterCount: 4, Options: []gotgbot.PollOption{{Text: "A", VoterCount: 2}, {Text: "B", VoterCount: 2}}},
			{Id: "poll-known", IsClosed: false, TotalVoterCount: 4, Options: []gotgbot.PollOption{{Text: "A", VoterCount: 2}, {Text: "B", VoterCount: 2}}},
			{Id: "poll-known", IsClosed: true, TotalVoterCount: 0, Options: []gotgbot.PollOption{{Text: "A", VoterCount: 0}, {Text: "B", VoterCount: 0}}},
			{Id: "poll-known", IsClosed: true, TotalVoterCount: 5, Options: []gotgbot.PollOption{{Text: "A", VoterCount: 3}, {Text: "B", VoterCount: 2}}},
		}
		for _, p := range cases {
			env.deliverPoll(p)
		}
		if len(env.provider.calls()) != 0 || len(env.callsFor("sendMessage")) != 0 {
			t.Fatalf("expected zero AI/send calls for ignored polls, got ai=%d send=%d", len(env.provider.calls()), len(env.callsFor("sendMessage")))
		}
	})

	t.Run("tied closed poll posts once-only across duplicate/concurrent updates in original topic", func(t *testing.T) {
		env := newChatPollEnv(t)
		seedPollMeta(t, env, "poll-tied", 812, 42)
		env.provider.script(chatAnswerText("Coffee wins <by decree> & administrative fiat."))

		tiedPoll := &gotgbot.Poll{
			Id:              "poll-tied",
			Question:        "Coffee or Tea?",
			IsClosed:        true,
			TotalVoterCount: 4,
			Options: []gotgbot.PollOption{
				{Text: "Coffee", VoterCount: 2},
				{Text: "Tea", VoterCount: 2},
			},
		}

		var wg sync.WaitGroup
		for range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = env.dispatcher.ProcessUpdate(env.bot, &gotgbot.Update{UpdateId: 77, Poll: tiedPoll}, nil)
			}()
		}
		wg.Wait()
		chatJobs.wait()

		msgs := env.callsFor("sendMessage")
		if len(msgs) != 1 {
			t.Fatalf("sendMessage calls = %d, want exactly 1 via SETNX claim", len(msgs))
		}
		wire := msgs[0].Params
		if wire["message_thread_id"] != "42" {
			t.Errorf("message_thread_id = %q, want 42", wire["message_thread_id"])
		}
		if unquoteWireParam(wire["parse_mode"]) != "HTML" {
			t.Errorf("parse_mode = %q, want HTML", wire["parse_mode"])
		}
		var reply gotgbot.ReplyParameters
		if err := json.Unmarshal([]byte(wire["reply_parameters"]), &reply); err != nil {
			t.Fatalf("unmarshal reply_parameters: %v", err)
		}
		if reply.MessageId != 812 || reply.AllowSendingWithoutReply {
			t.Errorf("reply_parameters = %+v, want MessageId=812 AllowSendingWithoutReply=false", reply)
		}
		text := unquoteWireParam(wire["text"])
		if !strings.Contains(text, "Facility Tie-Breaker") || !strings.Contains(text, "Coffee wins &lt;by decree&gt; &amp; administrative fiat.") {
			t.Errorf("unexpected tie-breaker body: %q", text)
		}
	})

	t.Run("staff group and lockdown block tie-breaker", func(t *testing.T) {
		env := newChatPollEnv(t)
		seedPollMeta(t, env, "poll-staff", 813, 0)
		seedPollMeta(t, env, "poll-lock", 814, 0)

		env.mu.Lock()
		env.staffGroup = &models.StaffGroup{ChatID: env.chatID}
		env.mu.Unlock()
		env.deliverPoll(&gotgbot.Poll{
			Id: "poll-staff", IsClosed: true, TotalVoterCount: 2,
			Options: []gotgbot.PollOption{{Text: "A", VoterCount: 1}, {Text: "B", VoterCount: 1}},
		})
		env.mu.Lock()
		env.staffGroup = nil
		env.lockdown = &models.ChatLockdown{ChatID: env.chatID}
		env.mu.Unlock()
		env.deliverPoll(&gotgbot.Poll{
			Id: "poll-lock", IsClosed: true, TotalVoterCount: 2,
			Options: []gotgbot.PollOption{{Text: "A", VoterCount: 1}, {Text: "B", VoterCount: 1}},
		})
		env.mu.Lock()
		env.lockdown = nil
		env.mu.Unlock()

		if len(env.provider.calls()) != 0 || len(env.callsFor("sendMessage")) != 0 {
			t.Fatal("staff group / lockdown should block tie-breaker silently")
		}
	})

	t.Run("deadpan fallback when AI fails and deleted poll stays silent", func(t *testing.T) {
		env := newChatPollEnv(t)
		seedPollMeta(t, env, "poll-fallback", 815, 9)
		env.provider.script(chatAnswerErr(errors.New("provider boom")))

		env.deliverPoll(&gotgbot.Poll{
			Id: "poll-fallback", IsClosed: true, TotalVoterCount: 2,
			Options: []gotgbot.PollOption{{Text: "A", VoterCount: 1}, {Text: "B", VoterCount: 1}},
		})

		msgs := env.callsFor("sendMessage")
		if len(msgs) != 1 {
			t.Fatalf("sendMessage calls = %d, want 1 fallback", len(msgs))
		}
		text := unquoteWireParam(msgs[0].Params["text"])
		if !strings.Contains(text, "Deadlock recorded.") {
			t.Errorf("expected English tie-breaker fallback, got %q", text)
		}

		// Deleted poll target (Telegram 400 reply not found) stays silent without incrementing ties_broken.
		seedPollMeta(t, env, "poll-deleted", 816, 9)
		beforeBroken := chatPollTiesBrokenCount.Load()
		env.mu.Lock()
		env.sendMsgErr = "Bad Request: message to be replied not found"
		env.mu.Unlock()
		env.deliverPoll(&gotgbot.Poll{
			Id: "poll-deleted", IsClosed: true, TotalVoterCount: 2,
			Options: []gotgbot.PollOption{{Text: "A", VoterCount: 1}, {Text: "B", VoterCount: 1}},
		})
		if chatPollTiesBrokenCount.Load() != beforeBroken {
			t.Fatal("deleted poll should not increment chatPollTiesBrokenCount")
		}
	})
}

func TestShutdownEndsChatToys(t *testing.T) {
	t.Run("StopChat ends in-flight /referendum provider call and sheds late command", func(t *testing.T) {
		env := newChatPollEnv(t)
		chatSwap(t, &chatStopWait, 2*time.Second)
		restoreChatPool(t)

		started := make(chan struct{})
		env.provider.script(func(ctx context.Context, _ chatRequest) (chatAnswer, error) {
			close(started)
			<-ctx.Done()
			return chatAnswer{}, ctx.Err()
		})

		alice := env.member("Alice")
		env.nextUpdateID++
		if err := env.dispatcher.ProcessUpdate(env.bot, &gotgbot.Update{
			UpdateId: env.nextUpdateID,
			Message:  env.referendumCmd(alice, 0, "cats vs dogs", nil),
		}, nil); err != nil {
			t.Fatalf("ProcessUpdate: %v", err)
		}
		<-started

		if !StopChat() {
			t.Fatal("StopChat timed out waiting for /referendum")
		}
		assertChatBreakersClean(t)

		// Late /referendum after StopChat must shed cleanly with zero provider/sendPoll calls.
		env.deliverMessage(env.referendumCmd(env.member("Bob"), 0, "tea vs coffee", nil))
		if len(env.provider.calls()) != 1 {
			t.Fatalf("provider calls = %d, want 1", len(env.provider.calls()))
		}
		if len(env.callsFor("sendPoll")) != 0 || len(env.callsFor("sendMessage")) != 0 {
			t.Fatal("expected zero polls or messages during shutdown")
		}
	})

	t.Run("StopChat ends in-flight handlePollUpdate tie-breaker and sheds late update", func(t *testing.T) {
		env := newChatPollEnv(t)
		chatSwap(t, &chatStopWait, 2*time.Second)
		restoreChatPool(t)

		meta := chatPollMeta{ChatID: env.chatID, MessageID: 950, Lang: "en", Question: "Tabs or spaces?"}
		raw, err := json.Marshal(meta)
		if err != nil {
			t.Fatal(err)
		}
		if err := env.mr.Set(presencePollKey("poll-shut-1"), string(raw)); err != nil {
			t.Fatal(err)
		}
		if err := env.mr.Set(presencePollKey("poll-shut-2"), string(raw)); err != nil {
			t.Fatal(err)
		}

		started := make(chan struct{})
		var once sync.Once
		env.provider.script(func(ctx context.Context, _ chatRequest) (chatAnswer, error) {
			once.Do(func() { close(started) })
			<-ctx.Done()
			return chatAnswer{}, ctx.Err()
		})

		var wg sync.WaitGroup
		var procErr atomic.Value
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := env.dispatcher.ProcessUpdate(env.bot, &gotgbot.Update{
				UpdateId: 201,
				Poll: &gotgbot.Poll{
					Id: "poll-shut-1", IsClosed: true, TotalVoterCount: 2,
					Options: []gotgbot.PollOption{{Text: "Tabs", VoterCount: 1}, {Text: "Spaces", VoterCount: 1}},
				},
			}, nil); err != nil {
				procErr.Store(err)
			}
		}()

		<-started
		if !StopChat() {
			t.Fatal("StopChat timed out waiting for handlePollUpdate")
		}
		wg.Wait()
		if v := procErr.Load(); v != nil {
			t.Fatalf("ProcessUpdate error: %v", v)
		}
		assertChatBreakersClean(t)

		// Late tie-breaker update after StopChat must shed cleanly.
		env.deliverPoll(&gotgbot.Poll{
			Id: "poll-shut-2", IsClosed: true, TotalVoterCount: 2,
			Options: []gotgbot.PollOption{{Text: "Tabs", VoterCount: 1}, {Text: "Spaces", VoterCount: 1}},
		})
		if len(env.provider.calls()) != 1 {
			t.Fatalf("provider calls = %d, want 1", len(env.provider.calls()))
		}
		if len(env.callsFor("sendMessage")) != 0 {
			t.Fatal("expected zero tie-breaker sends during shutdown")
		}
	})
}
