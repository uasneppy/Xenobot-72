//go:build testtools

package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/uasneppy/Xenobot-72/xenobot/config"
	"github.com/uasneppy/Xenobot-72/xenobot/db/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/db/chatconv"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/db/presence"
	utilsCache "github.com/uasneppy/Xenobot-72/xenobot/utils/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/chatprompt"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/featherless"
)

type chatInspectWireCall struct {
	Method string
	Params map[string]string
}

type chatInspectEnv struct {
	t          *testing.T
	chatID     int64
	mr         *miniredis.Miniredis
	rc         *redis.Client
	bot        *gotgbot.Bot
	dispatcher *ext.Dispatcher
	provider   *chatFakeProvider

	mu             sync.Mutex
	convSettings   chatconv.Settings
	presSettings   presence.Settings
	staffGroup     *models.StaffGroup
	lockdown       *models.ChatLockdown
	lockdownErr    error
	admins         map[int64]bool
	approved       map[int64]bool
	availReactions string
	sendMsgErr     string
	files          map[string][]byte
	fileDownloads  int
	calls          []chatInspectWireCall
	nextUpdateID   int64
	nextMsgID      int64
	nextSentID     int64
}

func newChatInspectEnv(t *testing.T) *chatInspectEnv {
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

	env := &chatInspectEnv{
		t:      t,
		chatID: uniqueModuleChatID(),
		mr:     mr,
		rc:     rc,
		convSettings: chatconv.Settings{
			Enabled:  true,
			Provider: chatconv.ProviderAnthropic,
			Audience: chatconv.AudienceEveryone,
		},
		presSettings: presence.Settings{
			RetractEnabled: true,
			InspectEnabled: true,
		},
		admins:       map[int64]bool{},
		approved:     map[int64]bool{},
		files:        map[string][]byte{},
		provider:     &chatFakeProvider{},
		nextUpdateID: 1,
		nextMsgID:    2000,
		nextSentID:   9500,
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
	chatSwap(t, &presenceIsAdmin, func(_ *gotgbot.Bot, _ int64, userID int64) bool {
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

func (e *chatInspectEnv) handleTelegram(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/file/bottest/") {
		rel := strings.TrimPrefix(r.URL.Path, "/file/bottest/")
		e.mu.Lock()
		e.fileDownloads++
		body, ok := e.files[rel]
		e.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
		return
	}

	method := strings.TrimPrefix(r.URL.Path, "/bottest/")
	params, _ := decodeTelegramWireParams(r)

	e.mu.Lock()
	e.calls = append(e.calls, chatInspectWireCall{Method: method, Params: params})
	avail := e.availReactions
	sendErr := e.sendMsgErr
	e.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	switch method {
	case "getChat":
		if avail != "" {
			_, _ = fmt.Fprintf(w, `{"ok":true,"result":{"id":%d,"type":"supergroup","available_reactions":%s}}`, e.chatID, avail)
			return
		}
		_, _ = fmt.Fprintf(w, `{"ok":true,"result":{"id":%d,"type":"supergroup"}}`, e.chatID)
	case "getFile":
		id := strings.Trim(params["file_id"], `"`)
		_, _ = fmt.Fprintf(w, `{"ok":true,"result":{"file_id":%q,"file_unique_id":"u-%s","file_size":100,"file_path":"files/%s"}}`, id, id, id)
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

func (e *chatInspectEnv) callsFor(method string) []chatInspectWireCall {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out []chatInspectWireCall
	for _, c := range e.calls {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

func (e *chatInspectEnv) downloads() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.fileDownloads
}

func (e *chatInspectEnv) member(name string) *gotgbot.User {
	return &gotgbot.User{Id: uniqueLinkOwnerID(), FirstName: name}
}

func (e *chatInspectEnv) message(from *gotgbot.User, threadID int64, text string) *gotgbot.Message {
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

func (e *chatInspectEnv) inspectCmd(from *gotgbot.User, threadID int64, reply *gotgbot.Message) *gotgbot.Message {
	msg := e.message(from, threadID, "/inspect")
	msg.Entities = []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: int64(len("/inspect"))}}
	msg.ReplyToMessage = reply
	return msg
}

func (e *chatInspectEnv) deliverMessage(msg *gotgbot.Message) {
	e.t.Helper()
	e.nextUpdateID++
	if err := e.dispatcher.ProcessUpdate(e.bot, &gotgbot.Update{UpdateId: e.nextUpdateID, Message: msg}, nil); err != nil {
		e.t.Fatalf("ProcessUpdate(Message): %v", err)
	}
	chatJobs.wait()
}

func (e *chatInspectEnv) deliverReaction(mru *gotgbot.MessageReactionUpdated) {
	e.t.Helper()
	e.nextUpdateID++
	if err := e.dispatcher.ProcessUpdate(e.bot, &gotgbot.Update{UpdateId: e.nextUpdateID, MessageReaction: mru}, nil); err != nil {
		e.t.Fatalf("ProcessUpdate(MessageReaction): %v", err)
	}
	chatJobs.wait()
}

func TestInspectGatesAndUsage(t *testing.T) {
	env := newChatInspectEnv(t)
	alice := env.member("Alice")
	bob := env.member("Bob")
	admin := env.member("Admin")
	approved := env.member("Approved")
	env.admins[admin.Id] = true
	env.approved[approved.Id] = true
	target := env.message(bob, 0, "I rewired the coolant pump with duct tape.")

	t.Run("AltHelpOptions includes referendum and inspect", func(t *testing.T) {
		opts := DefaultHelpRegistry().AltHelpOptions["Chat"]
		var hasRef, hasInsp bool
		for _, opt := range opts {
			if opt == "referendum" {
				hasRef = true
			}
			if opt == "inspect" {
				hasInsp = true
			}
		}
		if !hasRef || !hasInsp {
			t.Fatalf("AltHelpOptions[Chat] = %v, want both referendum and inspect", opts)
		}
	})

	t.Run("missing reply answers inspect_usage and spends nothing", func(t *testing.T) {
		beforeMsgs := len(env.callsFor("sendMessage"))
		env.deliverMessage(env.inspectCmd(alice, 0, nil))
		if len(env.provider.calls()) != 0 {
			t.Fatal("missing reply must make zero AI calls")
		}
		if keys := env.mr.Keys(); len(keys) != 0 {
			t.Fatalf("missing reply wrote Redis keys %v, want none", keys)
		}
		msgs := env.callsFor("sendMessage")
		if len(msgs) != beforeMsgs+1 {
			t.Fatalf("sendMessage calls = %d, want %d", len(msgs), beforeMsgs+1)
		}
		if got := unquoteWireParam(msgs[len(msgs)-1].Params["text"]); got != chatCanned(t, "inspect_usage") {
			t.Fatalf("reply = %q, want %q", got, chatCanned(t, "inspect_usage"))
		}
	})

	t.Run("empty service message reply answers inspect_usage and spends nothing", func(t *testing.T) {
		beforeMsgs := len(env.callsFor("sendMessage"))
		emptyMsg := env.message(bob, 0, "")
		env.deliverMessage(env.inspectCmd(alice, 0, emptyMsg))
		if len(env.provider.calls()) != 0 {
			t.Fatal("empty target must make zero AI calls")
		}
		msgs := env.callsFor("sendMessage")
		if len(msgs) != beforeMsgs+1 {
			t.Fatalf("sendMessage calls = %d, want %d", len(msgs), beforeMsgs+1)
		}
		if got := unquoteWireParam(msgs[len(msgs)-1].Params["text"]); got != chatCanned(t, "inspect_usage") {
			t.Fatalf("reply = %q, want %q", got, chatCanned(t, "inspect_usage"))
		}
	})

	t.Run("private chat refused", func(t *testing.T) {
		beforeMsgs := len(env.callsFor("sendMessage"))
		priv := env.inspectCmd(alice, 0, target)
		priv.Chat = gotgbot.Chat{Id: alice.Id, Type: "private"}
		env.deliverMessage(priv)
		if len(env.provider.calls()) != 0 {
			t.Fatal("private chat must make zero AI calls")
		}
		if len(env.callsFor("sendMessage")) != beforeMsgs+1 {
			t.Fatalf("private chat should answer group-only refusal")
		}
	})

	t.Run("staff group and lockdown silent", func(t *testing.T) {
		beforeMsgs := len(env.callsFor("sendMessage"))
		env.mu.Lock()
		env.staffGroup = &models.StaffGroup{ChatID: env.chatID}
		env.mu.Unlock()
		env.deliverMessage(env.inspectCmd(alice, 0, target))

		env.mu.Lock()
		env.staffGroup = nil
		env.lockdown = &models.ChatLockdown{ChatID: env.chatID}
		env.mu.Unlock()
		env.deliverMessage(env.inspectCmd(alice, 0, target))

		env.mu.Lock()
		env.lockdown = nil
		env.mu.Unlock()
		if len(env.callsFor("sendMessage")) != beforeMsgs || len(env.provider.calls()) != 0 {
			t.Fatal("staff group and lockdown must stay completely silent")
		}
	})

	t.Run("owner conversations off and admin InspectEnabled off answer inspect_off_hint", func(t *testing.T) {
		beforeMsgs := len(env.callsFor("sendMessage"))
		env.mu.Lock()
		env.convSettings.Enabled = false
		env.mu.Unlock()
		env.deliverMessage(env.inspectCmd(alice, 0, target))

		env.mu.Lock()
		env.convSettings.Enabled = true
		env.presSettings.InspectEnabled = false
		env.mu.Unlock()
		env.deliverMessage(env.inspectCmd(alice, 0, target))

		env.mu.Lock()
		env.presSettings.InspectEnabled = true
		env.mu.Unlock()

		msgs := env.callsFor("sendMessage")
		if len(msgs) != beforeMsgs+2 {
			t.Fatalf("sendMessage calls = %d, want %d", len(msgs), beforeMsgs+2)
		}
		for i := beforeMsgs; i < len(msgs); i++ {
			if got := unquoteWireParam(msgs[i].Params["text"]); got != chatCanned(t, "inspect_off_hint") {
				t.Fatalf("reply[%d] = %q, want %q", i, got, chatCanned(t, "inspect_off_hint"))
			}
		}
		if len(env.provider.calls()) != 0 {
			t.Fatal("off gates must make zero AI calls")
		}
	})

	t.Run("audience admins approved blocks regular user silently and allows admin/approved", func(t *testing.T) {
		chatSwap(t, &chatToyUserCooldown, 0)
		env.mu.Lock()
		env.convSettings.Audience = chatconv.AudienceAdminsApproved
		env.mu.Unlock()

		beforeMsgs := len(env.callsFor("sendMessage"))
		env.deliverMessage(env.inspectCmd(alice, 0, target))
		if len(env.callsFor("sendMessage")) != beforeMsgs || len(env.provider.calls()) != 0 {
			t.Fatal("non-approved user should be blocked silently under AudienceAdminsApproved")
		}

		env.provider.script(
			chatAnswerText(`{"verdict":"Structurally dubious.\nApproved for now.","emoji":"👀"}`),
			chatAnswerText(`{"verdict":"Acceptable tape density.","emoji":"👍"}`),
		)
		env.deliverMessage(env.inspectCmd(admin, 0, target))
		env.deliverMessage(env.inspectCmd(approved, 0, target))
		if got := len(env.provider.calls()); got != 2 {
			t.Fatalf("provider calls for admin+approved = %d, want 2", got)
		}
		if got := len(env.callsFor("sendMessage")); got != beforeMsgs+2 {
			t.Fatalf("sendMessage calls = %d, want %d", got, beforeMsgs+2)
		}
		env.mu.Lock()
		env.convSettings.Audience = chatconv.AudienceEveryone
		env.mu.Unlock()
	})

	t.Run("per-user cooldown and dedicated toy:b budget isolated from b and react:b", func(t *testing.T) {
		env.mr.FlushAll()
		chatSwap(t, &chatToyUserCooldown, 20*time.Second)
		chatSwap(t, &chatToyUserHourlyBudget, int64(1))
		beforeMsgs := len(env.callsFor("sendMessage"))
		beforeAI := len(env.provider.calls())

		env.provider.script(chatAnswerText(`{"verdict":"Passable.\nBarely.","emoji":"👀"}`))
		env.deliverMessage(env.inspectCmd(alice, 0, target))
		if got := len(env.provider.calls()); got != beforeAI+1 {
			t.Fatalf("first inspect AI calls = %d, want %d", got, beforeAI+1)
		}

		// Second call within 20s is shed by per-user cooldown.
		env.deliverMessage(env.inspectCmd(alice, 0, target))
		if got := len(env.provider.calls()); got != beforeAI+1 {
			t.Fatalf("cooldown inspect AI calls = %d, want still %d", got, beforeAI+1)
		}

		// Fast-forward cooldown -> hits toy:b hourly budget (1) and sends inspect_allowance once.
		env.mr.FastForward(25 * time.Second)
		env.deliverMessage(env.inspectCmd(alice, 0, target))
		if got := len(env.provider.calls()); got != beforeAI+1 {
			t.Fatalf("budget-capped inspect AI calls = %d, want still %d", got, beforeAI+1)
		}
		msgs := env.callsFor("sendMessage")
		if len(msgs) != beforeMsgs+2 {
			t.Fatalf("sendMessage calls = %d, want %d", len(msgs), beforeMsgs+2)
		}
		if got := unquoteWireParam(msgs[len(msgs)-1].Params["text"]); got != chatCanned(t, "inspect_allowance") {
			t.Fatalf("allowance reply = %q, want %q", got, chatCanned(t, "inspect_allowance"))
		}

		// Alice's conversation "b" and reaction "react:b" budgets remain untouched.
		if res := chatReserve(env.rc, env.chatID, alice.Id); res != chatBudgetReserved {
			t.Fatalf("conversation b budget affected by toy:b: %v", res)
		}
		if res := chatReserveWith(env.rc, env.chatID, alice.Id, "react:b", 1, 10, 100); res != chatBudgetReserved {
			t.Fatalf("react:b budget affected by toy:b: %v", res)
		}
	})

	t.Run("malformed JSON and refusal stay clean on breakers", func(t *testing.T) {
		env.mr.FlushAll()
		chatSwap(t, &chatToyUserCooldown, 0)
		beforeMsgs := len(env.callsFor("sendMessage"))
		env.provider.script(
			chatAnswerText("invalid json"),
			func(context.Context, chatRequest) (chatAnswer, error) { return chatAnswer{Outcome: chatRefused}, nil },
		)
		env.deliverMessage(env.inspectCmd(alice, 0, target))
		env.deliverMessage(env.inspectCmd(alice, 0, target))
		msgs := env.callsFor("sendMessage")
		if len(msgs) != beforeMsgs+2 {
			t.Fatalf("sendMessage calls = %d, want %d", len(msgs), beforeMsgs+2)
		}
		if got := unquoteWireParam(msgs[beforeMsgs].Params["text"]); got != chatCanned(t, "inspect_malfunction") {
			t.Fatalf("malformed JSON reply = %q, want %q", got, chatCanned(t, "inspect_malfunction"))
		}
		if got := unquoteWireParam(msgs[beforeMsgs+1].Params["text"]); got != chatCanned(t, "inspect_refused") {
			t.Fatalf("refusal reply = %q, want %q", got, chatCanned(t, "inspect_refused"))
		}
		assertChatBreakersClean(t)
	})
}

func TestInspectVisionAndRetraction(t *testing.T) {
	t.Run("photo and sticker vision, MaxTokens=InspectMaxTokens, U+FE0F strip, AllowSendingWithoutReply=false, bot_react, and thumbs-down retraction", func(t *testing.T) {
		env := newChatInspectEnv(t)
		chatSwap(t, &chatToyUserCooldown, 0)
		alice := env.member("Alice")
		bob := env.member("Bob")
		carol := env.member("Carol")

		env.mu.Lock()
		env.files["files/p800"] = raidAITestJPEG("inspect-photo")
		env.files["files/thumb1"] = raidAITestWebP("inspect-sticker")
		env.mu.Unlock()

		// 1. Photo reply in topic 12 -> downloads photo, sends the inspect token cap, strips U+FE0F ("🔥\ufe0f" -> "🔥"),
		// sanitizes links/mentions, replies to targetMsgID with AllowSendingWithoutReply=false, stamps bot_react.
		photoTarget := env.message(bob, 12, "")
		photoTarget.Caption = "check https://evil.example @bob"
		photoTarget.Photo = photoSizes()

		env.provider.script(chatAnswerText("{\"verdict\":\"Containment breach <level 2> @carol https://bad.example\\nTape insufficient.\",\"emoji\":\"🔥\ufe0f\"}"))
		env.deliverMessage(env.inspectCmd(alice, 12, photoTarget))

		calls := env.provider.calls()
		if len(calls) != 1 {
			t.Fatalf("provider calls = %d, want 1", len(calls))
		}
		if calls[0].MaxTokens != chatprompt.InspectMaxTokens {
			t.Fatalf("MaxTokens = %d, want %d", calls[0].MaxTokens, chatprompt.InspectMaxTokens)
		}
		if len(calls[0].Attachments) != 1 || calls[0].Attachments[0].MIMEType != "image/jpeg" {
			t.Fatalf("photo attachments = %+v, want 1 image/jpeg", calls[0].Attachments)
		}

		sends := env.callsFor("sendMessage")
		if len(sends) != 1 {
			t.Fatalf("sendMessage calls = %d, want 1", len(sends))
		}
		wire := sends[0].Params
		if wire["message_thread_id"] != "12" {
			t.Errorf("message_thread_id = %q, want 12", wire["message_thread_id"])
		}
		if unquoteWireParam(wire["parse_mode"]) != "HTML" {
			t.Errorf("parse_mode = %q, want HTML", wire["parse_mode"])
		}
		var reply gotgbot.ReplyParameters
		if err := json.Unmarshal([]byte(wire["reply_parameters"]), &reply); err != nil {
			t.Fatalf("unmarshal reply_parameters: %v", err)
		}
		if reply.MessageId != photoTarget.MessageId || reply.AllowSendingWithoutReply {
			t.Errorf("reply_parameters = %+v, want MessageId=%d AllowSendingWithoutReply=false", reply, photoTarget.MessageId)
		}
		verdictText := unquoteWireParam(wire["text"])
		if !strings.Contains(verdictText, chatCanned(t, "inspect_header")) ||
			!strings.Contains(verdictText, "&lt;level 2&gt;") ||
			!strings.Contains(verdictText, "@\u200bcarol") ||
			!strings.Contains(verdictText, "[link removed]") ||
			strings.Contains(verdictText, "https://bad.example") {
			t.Fatalf("unexpected sanitized verdict HTML: %q", verdictText)
		}

		reacts := env.callsFor("setMessageReaction")
		if len(reacts) != 1 {
			t.Fatalf("setMessageReaction calls = %d, want 1", len(reacts))
		}
		if reacts[0].Params["message_id"] != strconv.FormatInt(photoTarget.MessageId, 10) {
			t.Errorf("reaction message_id = %q, want %d", reacts[0].Params["message_id"], photoTarget.MessageId)
		}
		if isBig, ok := reacts[0].Params["is_big"]; ok && isBig != "false" {
			t.Errorf("is_big = %q, want false/omitted", isBig)
		}
		if !strings.Contains(reacts[0].Params["reaction"], `"🔥"`) {
			t.Errorf("reaction wire = %q, want 🔥 (VS16 stripped)", reacts[0].Params["reaction"])
		}

		rec, ok := presenceReadBotReact(context.Background(), env.rc, env.chatID, photoTarget.MessageId)
		if !ok || rec.Emoji != "🔥" || rec.AuthorID != bob.Id || rec.Source != "inspect" {
			t.Fatalf("bot_react = %+v (ok=%v), want emoji=🔥 author=%d source=inspect", rec, ok, bob.Id)
		}
		if ttl := env.mr.TTL(presenceBotReactKey(env.chatID, photoTarget.MessageId)); ttl < 23*time.Hour {
			t.Fatalf("bot_react TTL = %v, want ~24h", ttl)
		}

		// Non-author non-admin Carol reacting 👎 must NOT clear the reaction.
		env.deliverReaction(&gotgbot.MessageReactionUpdated{
			Chat:        photoTarget.Chat,
			MessageId:   photoTarget.MessageId,
			User:        carol,
			NewReaction: emojiReactions("👎"),
		})
		if len(env.callsFor("setMessageReaction")) != 1 {
			t.Fatal("unauthorized non-author cleared /inspect reaction")
		}

		// Target message author Bob reacting 👎 clears reaction silently with [] and 0 group messages!
		sendsBeforeRetract := len(env.callsFor("sendMessage"))
		env.deliverReaction(&gotgbot.MessageReactionUpdated{
			Chat:        photoTarget.Chat,
			MessageId:   photoTarget.MessageId,
			User:        bob,
			NewReaction: emojiReactions("👎"),
		})
		if len(env.callsFor("sendMessage")) != sendsBeforeRetract {
			t.Fatal("👎 retraction must post zero group messages")
		}
		reactsAfter := env.callsFor("setMessageReaction")
		if len(reactsAfter) != 2 || reactsAfter[1].Params["reaction"] != "[]" {
			t.Fatalf("expected reaction clear [], got %+v", reactsAfter)
		}
		if env.mr.Exists(presenceBotReactKey(env.chatID, photoTarget.MessageId)) {
			t.Fatal("bot_react key still exists after 👎 retraction")
		}

		// 2. Animated sticker thumbnail downloaded via chat_media.go and [sticker 😺] in prompt.
		stickerTarget := env.message(bob, 0, "")
		stickerTarget.Sticker = &gotgbot.Sticker{
			FileId:     "anim1",
			IsAnimated: true,
			Emoji:      "😺",
			Thumbnail:  &gotgbot.PhotoSize{FileId: "thumb1", Width: 90, Height: 90},
		}
		env.provider.script(chatAnswerText(`{"verdict":"Feline morale asset certified.","emoji":"❤"}`))
		env.deliverMessage(env.inspectCmd(alice, 0, stickerTarget))

		calls = env.provider.calls()
		if len(calls) != 2 {
			t.Fatalf("provider calls = %d, want 2", len(calls))
		}
		if len(calls[1].Attachments) != 1 || calls[1].Attachments[0].MIMEType != "image/webp" {
			t.Fatalf("sticker attachments = %+v, want 1 image/webp", calls[1].Attachments)
		}
		if !strings.Contains(calls[1].User, "[sticker 😺]") {
			t.Fatalf("sticker user prompt missing [sticker 😺]: %s", calls[1].User)
		}
	})

	t.Run("Featherless 400/422 vision rejection retries without the image, posts no invented verdict, and remembers nothing", func(t *testing.T) {
		env := newChatInspectEnv(t)
		chatSwap(t, &chatToyUserCooldown, 0)
		chatSwap(t, &chatToyChatCooldown, 0)
		alice := env.member("Alice")
		bob := env.member("Bob")

		const model = "test/inspect-text-only-model"
		chatSwap(t, &config.AppConfig.ChatFeatherlessModel, model)
		env.mu.Lock()
		env.convSettings.Provider = chatconv.ProviderFeatherless
		env.files["files/p800"] = raidAITestJPEG("featherless-inspect")
		env.mu.Unlock()

		photoTarget1 := env.message(bob, 0, "")
		photoTarget1.Caption = "server rack"
		photoTarget1.Photo = photoSizes()

		env.provider.script(
			chatAnswerErr(&featherless.HTTPError{Status: http.StatusUnprocessableEntity, Type: "invalid_request_error"}),
			chatAnswerText(`{"verdict":"Text-only guess about a picture it never saw.","emoji":"👀"}`),
			chatAnswerText(`{"verdict":"Verdict from a model that saw the image.","emoji":"🔥"}`),
		)
		downloadsBefore := env.downloads()
		env.deliverMessage(env.inspectCmd(alice, 0, photoTarget1))

		calls := env.provider.calls()
		if len(calls) != 2 {
			t.Fatalf("featherless retry calls = %d, want 2", len(calls))
		}
		if len(calls[0].Attachments) != 1 || len(calls[1].Attachments) != 0 {
			t.Fatalf("featherless retry attachments = (%d, %d), want (1, 0)", len(calls[0].Attachments), len(calls[1].Attachments))
		}
		if env.downloads() != downloadsBefore+1 {
			t.Fatalf("downloads = %d, want %d", env.downloads(), downloadsBefore+1)
		}
		if env.mr.Exists("xenobot:cache:featherless_text_only:" + model) {
			t.Fatal("a vision rejection must not leave a text-only marker behind")
		}
		for _, send := range env.callsFor("sendMessage") {
			if strings.Contains(send.Params["text"], "never saw") {
				t.Fatalf("a verdict built on a text-only retry was posted: %s", send.Params["text"])
			}
		}
		if got := len(env.callsFor("sendMessage")); got != 1 {
			t.Fatalf("sendMessage calls = %d, want exactly the malfunction line", got)
		}
		assertChatBreakersClean(t)

		// The next photo is attempted with its image again.
		photoTarget2 := env.message(bob, 0, "")
		photoTarget2.Caption = "second rack"
		photoTarget2.Photo = photoSizes()
		downloadsBefore2 := env.downloads()
		env.deliverMessage(env.inspectCmd(alice, 0, photoTarget2))
		if env.downloads() != downloadsBefore2+1 {
			t.Fatalf("second photo downloads = %d, want %d (vision is attempted every time)", env.downloads()-downloadsBefore2, 1)
		}
		if len(env.provider.calls()[2].Attachments) != 1 {
			t.Fatal("second photo was sent without its image")
		}
	})

	t.Run("lookalike/disallowed emoji keeps text verdict without reaction, and deleted target stays silent", func(t *testing.T) {
		env := newChatInspectEnv(t)
		chatSwap(t, &chatToyUserCooldown, 0)
		alice := env.member("Alice")
		bob := env.member("Bob")

		// Restrict chat palette so 😂 and 🐱 and 🔥 are invalid, only 👍 is allowed.
		cache.DeleteCache(presenceAvailReactKey(env.chatID))
		env.mu.Lock()
		env.availReactions = `[{"type":"emoji","emoji":"👍"}]`
		env.mu.Unlock()

		target := env.message(bob, 0, "look at this")
		env.provider.script(
			chatAnswerText(`{"verdict":"Lookalike emoji verdict still posts.","emoji":"😂"}`),
			chatAnswerText(`{"verdict":"Cat lookalike verdict still posts.","emoji":"🐱"}`),
			chatAnswerText(`{"verdict":"Disallowed emoji verdict still posts.","emoji":"🔥"}`),
		)
		for range 3 {
			env.deliverMessage(env.inspectCmd(alice, 0, target))
		}
		if got := len(env.callsFor("sendMessage")); got != 3 {
			t.Fatalf("sendMessage calls = %d, want 3 text verdicts even with invalid emojis", got)
		}
		if got := len(env.callsFor("setMessageReaction")); got != 0 {
			t.Fatalf("setMessageReaction calls = %d, want 0 for lookalike/disallowed emojis", got)
		}

		// Deleted target (Telegram 400 reply not found) stays silent and does NOT stamp reaction.
		env.mu.Lock()
		env.sendMsgErr = "Bad Request: message to be replied not found"
		env.mu.Unlock()
		env.provider.script(chatAnswerText(`{"verdict":"Target gone.","emoji":"👍"}`))
		env.deliverMessage(env.inspectCmd(alice, 0, target))
		if got := len(env.callsFor("setMessageReaction")); got != 0 {
			t.Fatalf("deleted target unexpectedly called setMessageReaction (%d)", got)
		}
	})
}

func TestShutdownEndsInspect(t *testing.T) {
	env := newChatInspectEnv(t)
	chatSwap(t, &chatStopWait, 2*time.Second)
	restoreChatPool(t)

	started := make(chan struct{})
	env.provider.script(func(ctx context.Context, _ chatRequest) (chatAnswer, error) {
		close(started)
		<-ctx.Done()
		return chatAnswer{}, ctx.Err()
	})

	alice := env.member("Alice")
	bob := env.member("Bob")
	target := env.message(bob, 0, "inspect me during shutdown")

	env.nextUpdateID++
	if err := env.dispatcher.ProcessUpdate(env.bot, &gotgbot.Update{
		UpdateId: env.nextUpdateID,
		Message:  env.inspectCmd(alice, 0, target),
	}, nil); err != nil {
		t.Fatalf("ProcessUpdate: %v", err)
	}
	<-started

	if !StopChat() {
		t.Fatal("StopChat timed out waiting for /inspect")
	}
	assertChatBreakersClean(t)

	// Late /inspect after StopChat must shed cleanly with zero provider/send/reaction calls.
	env.deliverMessage(env.inspectCmd(env.member("Carol"), 0, target))
	if len(env.provider.calls()) != 1 {
		t.Fatalf("provider calls = %d, want 1", len(env.provider.calls()))
	}
	if len(env.callsFor("sendMessage")) != 0 || len(env.callsFor("setMessageReaction")) != 0 {
		t.Fatal("expected zero messages or reactions during shutdown")
	}
}
