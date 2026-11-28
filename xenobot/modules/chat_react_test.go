//go:build testtools

package modules

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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

	"github.com/uasneppy/Xenobot-72/xenobot/config"
	"github.com/uasneppy/Xenobot-72/xenobot/db/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/db/chatconv"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/db/presence"
	utilsCache "github.com/uasneppy/Xenobot-72/xenobot/utils/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/chatprompt"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/claude"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/featherless"
)

type chatReactWireCall struct {
	Method string
	Params map[string]string
}

type chatReactEnv struct {
	t          *testing.T
	chatID     int64
	mr         *miniredis.Miniredis
	rc         *redis.Client
	bot        *gotgbot.Bot
	dispatcher *ext.Dispatcher
	provider   *chatFakeProvider

	mu              sync.Mutex
	convSettings    chatconv.Settings
	presSettings    presence.Settings
	availReactions  string // raw JSON array for getChat available_reactions; empty means omitted (all 73)
	reactErr        string // non-empty returns Telegram 400 with this description on setMessageReaction
	files           map[string][]byte
	calls           []chatReactWireCall
	fileDownloads   int
	nextUpdateID    int64
	nextMessageID   int64
	nextSendMessage int64
}

func newChatReactEnv(t *testing.T) *chatReactEnv {
	t.Helper()
	withRealLocales(t)
	utilsCache.SetupTestMemoryMarshaler(t)
	withFastPresencePacer(t)
	mr, rc := withChatRedis(t)
	withChatRuntime(t)
	withChatProviders(t, true, true, true)
	withChatFeatherless(t, "test-featherless-secret-key", "Qwen/Qwen3-32B")

	chatSwap(t, &chatReactUserHourlyBudget, chatReactUserHourlyBudget)
	chatSwap(t, &chatReactChatHourlyBudget, chatReactChatHourlyBudget)
	chatSwap(t, &chatReactGlobalHourlyBudget, chatReactGlobalHourlyBudget)
	chatSwap(t, &chatReactProfiles, map[string]chatReactProfile{
		presence.AIReactModeLow:    {minGap: 1, chatCooldown: 0, userCooldown: 0, chancePct: 100},
		presence.AIReactModeMedium: {minGap: 1, chatCooldown: 0, userCooldown: 0, chancePct: 100},
	})
	chatSwap(t, &chatReactRoll, func(chancePct int) bool { return chancePct > 0 })
	chatSwap(t, &presenceStaffLookup, func(int64) *models.StaffGroup { return nil })
	chatSwap(t, &presenceLockdownLookup, func(int64) (*models.ChatLockdown, error) { return nil, nil })

	env := &chatReactEnv{
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
			ConsensusEnabled:   true,
			ConsensusThreshold: presence.DefaultConsensusThreshold,
			RetractEnabled:     true,
			AIReactMode:        presence.AIReactModeMedium,
		},
		files:           map[string][]byte{},
		provider:        &chatFakeProvider{},
		nextUpdateID:    1,
		nextMessageID:   1000,
		nextSendMessage: 9000,
	}

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
	chatSwap(t, &chatProviderOf, func(string) chatProvider { return env.provider })

	env.bot = newWireTestBot(t, env.handleTelegram)
	env.bot.User = gotgbot.User{Id: 999, IsBot: true, FirstName: "Xenobot-72", Username: "xenobot72"}

	env.dispatcher = ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadChat(env.dispatcher)
	return env
}

func (e *chatReactEnv) handleTelegram(w http.ResponseWriter, r *http.Request) {
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
	e.calls = append(e.calls, chatReactWireCall{Method: method, Params: params})
	avail := e.availReactions
	reactErr := e.reactErr
	e.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	switch method {
	case "getChat":
		if avail != "" {
			_, _ = fmt.Fprintf(w, `{"ok":true,"result":{"id":%d,"type":"supergroup","available_reactions":%s}}`, e.chatID, avail)
			return
		}
		_, _ = fmt.Fprintf(w, `{"ok":true,"result":{"id":%d,"type":"supergroup"}}`, e.chatID)
	case "setMessageReaction":
		if reactErr != "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintf(w, `{"ok":false,"error_code":400,"description":%q}`, reactErr)
			return
		}
		_, _ = io.WriteString(w, `{"ok":true,"result":true}`)
	case "getFile":
		id := strings.Trim(params["file_id"], `"`)
		_, _ = fmt.Fprintf(w, `{"ok":true,"result":{"file_id":%q,"file_unique_id":"u-%s","file_size":100,"file_path":"files/%s"}}`, id, id, id)
	case "sendMessage":
		e.mu.Lock()
		e.nextSendMessage++
		msgID := e.nextSendMessage
		e.mu.Unlock()
		_, _ = fmt.Fprintf(w, `{"ok":true,"result":{"message_id":%d,"date":1,"chat":{"id":%d,"type":"supergroup"},"text":"ok"}}`, msgID, e.chatID)
	default:
		_, _ = io.WriteString(w, `{"ok":true,"result":true}`)
	}
}

func (e *chatReactEnv) callsFor(method string) []chatReactWireCall {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out []chatReactWireCall
	for _, c := range e.calls {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

func (e *chatReactEnv) downloads() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.fileDownloads
}

func (e *chatReactEnv) member(name string) *gotgbot.User {
	return &gotgbot.User{Id: uniqueLinkOwnerID(), FirstName: name}
}

func (e *chatReactEnv) message(from *gotgbot.User, text string) *gotgbot.Message {
	e.nextMessageID++
	return &gotgbot.Message{
		MessageId: e.nextMessageID,
		Date:      time.Now().Unix(),
		From:      from,
		Text:      text,
		Chat:      gotgbot.Chat{Id: e.chatID, Type: "supergroup", Title: "Facility 7"},
	}
}

func (e *chatReactEnv) deliver(msg *gotgbot.Message) {
	e.t.Helper()
	e.nextUpdateID++
	if err := e.dispatcher.ProcessUpdate(e.bot, &gotgbot.Update{UpdateId: e.nextUpdateID, Message: msg}, nil); err != nil {
		e.t.Fatalf("ProcessUpdate: %v", err)
	}
	chatJobs.wait()
}

func TestChatReactDoubleGateAndSafety(t *testing.T) {
	env := newChatReactEnv(t)
	alice := env.member("Alice")
	env.provider.script(chatAnswerText(`{"emoji":"🔥"}`))

	// 1. Owner conv off while Admin AI mode is medium -> blocked (PRES-04).
	env.mu.Lock()
	env.convSettings.Enabled = false
	env.presSettings.AIReactMode = presence.AIReactModeMedium
	env.mu.Unlock()
	env.deliver(env.message(alice, "shipped the release!"))
	if got := len(env.provider.calls()); got != 0 {
		t.Fatalf("conv disabled: provider calls = %d, want 0", got)
	}

	// 2. Owner conv on while Admin AI mode is off -> blocked + zero operational Redis I/O.
	env.mu.Lock()
	env.convSettings.Enabled = true
	env.presSettings.AIReactMode = presence.AIReactModeOff
	env.mu.Unlock()
	env.deliver(env.message(alice, "still off"))
	if got := len(env.provider.calls()); got != 0 {
		t.Fatalf("ai_react_mode off: provider calls = %d, want 0", got)
	}
	if keys := env.mr.Keys(); len(keys) != 0 {
		t.Fatalf("ai_react_mode off touched Redis: %v", keys)
	}

	// 3. Server EnableChat off with both settings enabled -> blocked.
	env.mu.Lock()
	env.presSettings.AIReactMode = presence.AIReactModeMedium
	env.mu.Unlock()
	prevEnable := config.AppConfig.EnableChat
	config.AppConfig.EnableChat = false
	env.deliver(env.message(alice, "enable chat off"))
	config.AppConfig.EnableChat = prevEnable
	if got := len(env.provider.calls()); got != 0 {
		t.Fatalf("EnableChat off: provider calls = %d, want 0", got)
	}

	// 4. Both gates open -> fires AI reaction.
	msg := env.message(alice, "we finally fixed the flaky race")
	env.deliver(msg)
	if got := len(env.callsFor("setMessageReaction")); got != 1 {
		t.Fatalf("both gates open: setMessageReaction calls = %d, want 1", got)
	}

	// 5. Turning Owner conv off immediately stops AI reactions while zero-AI flags stay independent.
	env.mu.Lock()
	env.convSettings.Enabled = false
	env.mu.Unlock()
	env.deliver(env.message(alice, "after owner disables conversations"))
	if got := len(env.callsFor("setMessageReaction")); got != 1 {
		t.Fatalf("after disabling conv: setMessageReaction calls = %d, want still 1", got)
	}
	if !env.presSettings.ConsensusEnabled || !env.presSettings.RetractEnabled {
		t.Fatal("zero-AI presence flags unexpectedly altered")
	}
	env.mu.Lock()
	env.convSettings.Enabled = true
	env.mu.Unlock()

	// 6. Safety gates: static_reacted handoff, Staff Group, lockdown (active & error), bot,
	// anonymous channel, slash command, non-scorable voice message.
	beforeCalls := len(env.provider.calls())

	// 6a. presenceStaticReactedKey handoff skips AI reactions.
	staticMsg := env.message(alice, "cat photo")
	extCtx := ext.NewContext(env.bot, &gotgbot.Update{UpdateId: 9999, Message: staticMsg}, map[string]any{
		presenceStaticReactedKey: true,
	})
	chatReactWatch(env.bot, extCtx)
	chatJobs.wait()

	// 6b. Staff Group skipped.
	prevStaff := presenceStaffLookup
	presenceStaffLookup = func(int64) *models.StaffGroup { return &models.StaffGroup{ChatID: env.chatID} }
	env.deliver(env.message(alice, "staff discussion"))
	presenceStaffLookup = prevStaff

	// 6c. Active lockdown and lockdown lookup error both fail closed without calling provider.
	shedBefore := chatReactShedCount.Load()
	prevLockdown := presenceLockdownLookup
	presenceLockdownLookup = func(int64) (*models.ChatLockdown, error) { return &models.ChatLockdown{ChatID: env.chatID}, nil }
	env.deliver(env.message(alice, "during active lockdown"))
	presenceLockdownLookup = func(int64) (*models.ChatLockdown, error) { return nil, errors.New("pg down") }
	env.deliver(env.message(alice, "during lockdown db error"))
	presenceLockdownLookup = prevLockdown
	if chatReactShedCount.Load() < shedBefore+2 {
		t.Fatalf("lockdown fail-closed shed count did not increment by 2 (before=%d after=%d)", shedBefore, chatReactShedCount.Load())
	}

	// 6d. Bot sender, anonymous channel, slash command, and voice-only message skipped.
	botMsg := env.message(&gotgbot.User{Id: uniqueLinkOwnerID(), FirstName: "OtherBot", IsBot: true}, "beep")
	env.deliver(botMsg)

	chanMsg := env.message(&gotgbot.User{Id: 136817688, FirstName: "Channel", IsBot: true}, "announcement")
	chanMsg.SenderChat = &gotgbot.Chat{Id: -100555, Type: "channel"}
	env.deliver(chanMsg)

	slashMsg := env.message(alice, "/status")
	env.deliver(slashMsg)

	voiceMsg := env.message(alice, "")
	voiceMsg.Voice = &gotgbot.Voice{FileId: "v1", Duration: 3}
	env.deliver(voiceMsg)

	if got := len(env.provider.calls()); got != beforeCalls {
		t.Fatalf("safety-gated messages made %d provider calls, want 0", got-beforeCalls)
	}

	// 7. Addressed @mention triggers Conversations reply ONLY, never an AI reaction.
	env.provider.script(chatAnswerText("Notice. Acknowledged."))
	handle := "@" + env.bot.Username
	mention := env.message(alice, handle+" status report")
	mention.Entities = []gotgbot.MessageEntity{{Type: "mention", Offset: 0, Length: int64(len(handle))}}
	env.deliver(mention)
	if got := len(env.callsFor("sendMessage")); got != 1 {
		t.Fatalf("@mention sendMessage calls = %d, want 1", got)
	}
	if got := len(env.callsFor("setMessageReaction")); got != 1 {
		t.Fatalf("@mention unexpectedly triggered AI reaction: calls = %d, want still 1", got)
	}
}

func TestChatReactRedisLuaGateAndBudget(t *testing.T) {
	t.Run("gap, probability miss preserving gap, cooldowns, and 24h retraction TTL preserved", func(t *testing.T) {
		env := newChatReactEnv(t)
		alice, bob := env.member("Alice"), env.member("Bob")

		chatSwap(t, &chatReactProfiles, map[string]chatReactProfile{
			presence.AIReactModeMedium: {
				minGap:       3,
				chatCooldown: 5 * time.Second,
				userCooldown: 20 * time.Second,
				chancePct:    25,
			},
		})
		var rollWin atomic.Bool
		rollWin.Store(true)
		chatSwap(t, &chatReactRoll, func(int) bool { return rollWin.Load() })

		var enqueued atomic.Int64
		chatSwap(t, &chatReactProcess, func(chatReactJob) { enqueued.Add(1) })

		// Messages 1 and 2 are below minGap (3).
		env.deliver(env.message(alice, "msg 1"))
		env.deliver(env.message(alice, "msg 2"))
		if enqueued.Load() != 0 {
			t.Fatalf("below minGap: enqueued = %d, want 0", enqueued.Load())
		}

		// Message 3 hits minGap=3, but probability roll misses -> gap stays >= 3!
		rollWin.Store(false)
		env.deliver(env.message(alice, "msg 3 miss"))
		if enqueued.Load() != 0 {
			t.Fatalf("roll miss: enqueued = %d, want 0", enqueued.Load())
		}

		// Message 4 immediately wins because gap was preserved >= minGap.
		rollWin.Store(true)
		env.deliver(env.message(alice, "msg 4 win"))
		if enqueued.Load() != 1 {
			t.Fatalf("after preserved gap win: enqueued = %d, want 1", enqueued.Load())
		}

		// Per-chat cooldown blocks even Bob after 3 messages.
		for i := range 3 {
			env.deliver(env.message(bob, fmt.Sprintf("bob during chat cd %d", i)))
		}
		if enqueued.Load() != 1 {
			t.Fatalf("during chat cooldown: enqueued = %d, want still 1", enqueued.Load())
		}

		// Expire chat cooldown (5s) while Alice's user cooldown (20s) is still active.
		env.mr.FastForward(6 * time.Second)
		env.deliver(env.message(alice, "alice still in user cd"))
		if enqueued.Load() != 1 {
			t.Fatalf("alice during user cooldown: enqueued = %d, want still 1", enqueued.Load())
		}
		// Bob is not in user cooldown and gap is already >= 3 -> succeeds!
		env.deliver(env.message(bob, "bob after chat cd"))
		if enqueued.Load() != 2 {
			t.Fatalf("bob after chat cooldown: enqueued = %d, want 2", enqueued.Load())
		}

		// 24h 👎 retraction cooldown on Alice must block AI reactions AND keep its 24h TTL intact.
		env.mr.FastForward(25 * time.Second)
		presenceExtendUserCooldown(context.Background(), env.rc, env.chatID, alice.Id)
		for i := range 4 {
			env.deliver(env.message(alice, fmt.Sprintf("alice retracted %d", i)))
		}
		if enqueued.Load() != 2 {
			t.Fatalf("alice under 24h retraction cooldown: enqueued = %d, want still 2", enqueued.Load())
		}
		ttl := env.mr.TTL(presenceUserCooldownKey(env.chatID, alice.Id))
		if ttl < 23*time.Hour {
			t.Fatalf("retraction user cooldown TTL overwritten: got %v, want > 23h", ttl)
		}
	})

	t.Run("dedicated react:b hourly budget isolated from conversation b budget", func(t *testing.T) {
		env := newChatReactEnv(t)
		chatSwap(t, &chatReactUserHourlyBudget, int64(1))
		chatSwap(t, &chatReactChatHourlyBudget, int64(2))
		alice, bob, carol := env.member("Alice"), env.member("Bob"), env.member("Carol")
		env.provider.script(
			chatAnswerText(`{"emoji":"🔥"}`),
			chatAnswerText(`{"emoji":"👏"}`),
			chatAnswerText(`{"emoji":"👀"}`),
		)

		cappedBefore := chatReactCappedCount.Load()

		// Alice 1st succeeds, 2nd hits user hourly budget (1).
		env.deliver(env.message(alice, "alice 1"))
		env.deliver(env.message(alice, "alice 2 capped"))
		if got := len(env.provider.calls()); got != 1 {
			t.Fatalf("after alice 2: provider calls = %d, want 1", got)
		}
		if chatReactCappedCount.Load() != cappedBefore+1 {
			t.Fatalf("capped count = %d, want %d", chatReactCappedCount.Load(), cappedBefore+1)
		}
		// Alice's conversation "b" budget must remain completely untouched.
		if res := chatReserve(env.rc, env.chatID, alice.Id); res != chatBudgetReserved {
			t.Fatalf("conversation budget affected by react:b: got %v, want chatBudgetReserved", res)
		}

		// Bob takes the 2nd chat unit; Carol hits chat hourly budget (2).
		env.deliver(env.message(bob, "bob 1"))
		env.deliver(env.message(carol, "carol capped by chat"))
		if got := len(env.provider.calls()); got != 2 {
			t.Fatalf("after bob+carol: provider calls = %d, want 2", got)
		}
		if chatReactCappedCount.Load() != cappedBefore+2 {
			t.Fatalf("capped count after chat cap = %d, want %d", chatReactCappedCount.Load(), cappedBefore+2)
		}
	})
}

func TestChatReactContextVisionAndFeatherlessFallback(t *testing.T) {
	t.Run("same-thread chat_log, ReplyTarget, msg.Quote, and LogEnabled=false", func(t *testing.T) {
		env := newChatReactEnv(t)
		env.mu.Lock()
		env.convSettings.LogEnabled = true
		env.mu.Unlock()
		alice, bob := env.member("Alice"), env.member("Bob")

		// Seed log across thread 0, thread 10 (6 entries + triggering ID), and thread 20.
		for _, entry := range []chatLogEntry{
			{MessageID: 100, UserID: bob.Id, Name: "OtherRoot", Text: "root thread leak", Thread: 0},
			{MessageID: 101, UserID: bob.Id, Name: "T10Oldest", Text: "dropped oldest", Thread: 10},
			{MessageID: 102, UserID: bob.Id, Name: "T10One", Text: "first kept", Thread: 10},
			{MessageID: 103, UserID: bob.Id, Name: "T10Two", Media: "[photo]", Text: "caption two", Thread: 10},
			{MessageID: 104, UserID: bob.Id, Name: "T10Three", Text: "third kept", Thread: 10},
			{MessageID: 105, UserID: bob.Id, Name: "T10Four", Text: "fourth kept", Thread: 10},
			{MessageID: 106, UserID: bob.Id, Name: "T10Five", Text: "fifth kept", Thread: 10},
			{MessageID: 107, UserID: bob.Id, Name: "OtherTopic", Text: "topic 20 leak", Thread: 20},
		} {
			if err := chatLogRecord(env.rc, env.chatID, entry); err != nil {
				t.Fatalf("chatLogRecord %d: %v", entry.MessageID, err)
			}
		}

		env.provider.script(chatAnswerText(`{"emoji":"🔥"}`), chatAnswerText(`{"emoji":"👀"}`))
		msg := env.message(alice, "that deploy actually worked")
		msg.MessageId = 108
		msg.IsTopicMessage = true
		msg.MessageThreadId = 10
		msg.ReplyToMessage = &gotgbot.Message{
			MessageId: 106, Date: 1, Chat: msg.Chat, From: bob, Text: "deploying on friday night",
		}
		msg.Quote = &gotgbot.TextQuote{Text: "friday night"}
		env.deliver(msg)

		calls := env.provider.calls()
		if len(calls) != 1 {
			t.Fatalf("provider calls = %d, want 1", len(calls))
		}
		userPrompt := calls[0].User
		for _, mustHave := range []string{
			"[Recent thread context, oldest first]",
			"T10One: first kept",
			"T10Two: [photo] caption two",
			"T10Five: fifth kept",
			"[Message being replied to]",
			"Bob: deploying on friday night",
			"[Quoted excerpt]",
			"friday night",
		} {
			if !strings.Contains(userPrompt, mustHave) {
				t.Errorf("prompt missing %q:\n%s", mustHave, userPrompt)
			}
		}
		for _, mustNotHave := range []string{"root thread leak", "topic 20 leak", "dropped oldest"} {
			if strings.Contains(userPrompt, mustNotHave) {
				t.Errorf("prompt leaked %q:\n%s", mustNotHave, userPrompt)
			}
		}
		if strings.Index(userPrompt, "T10One: first kept") > strings.Index(userPrompt, "T10Five: fifth kept") {
			t.Errorf("recent thread context not oldest-first:\n%s", userPrompt)
		}

		// With LogEnabled == false: omit [Recent thread context], still include ReplyTarget + Quote.
		env.mu.Lock()
		env.convSettings.LogEnabled = false
		env.mu.Unlock()
		msg2 := env.message(alice, "second message")
		msg2.ReplyToMessage = &gotgbot.Message{MessageId: 106, Date: 1, Chat: msg2.Chat, From: bob, Text: "replied text"}
		msg2.Quote = &gotgbot.TextQuote{Text: "quoted part"}
		env.deliver(msg2)

		userPrompt2 := env.provider.calls()[1].User
		if strings.Contains(userPrompt2, "[Recent thread context") {
			t.Errorf("LogEnabled=false unexpectedly included thread log:\n%s", userPrompt2)
		}
		if !strings.Contains(userPrompt2, "Bob: replied text") || !strings.Contains(userPrompt2, "quoted part") {
			t.Errorf("LogEnabled=false missing reply/quote:\n%s", userPrompt2)
		}
	})

	t.Run("photo and sticker vision, Featherless 400 text_only cache+retry, and custom-only skip", func(t *testing.T) {
		env := newChatReactEnv(t)
		alice := env.member("Alice")
		jpegBytes := raidAITestJPEG("react-cat")
		webpBytes := raidAITestWebP("react-sticker")
		env.mu.Lock()
		env.files["files/p800"] = jpegBytes
		env.files["files/thumb1"] = webpBytes
		env.mu.Unlock()

		// 1. Photo and animated-sticker thumbnail downloaded via chat_media.go on Claude.
		env.provider.script(chatAnswerText(`{"emoji":"❤"}`), chatAnswerText(`{"emoji":"🔥"}`))
		photoMsg := env.message(alice, "")
		photoMsg.Caption = "my new cat"
		photoMsg.Photo = photoSizes()
		env.deliver(photoMsg)

		stickerMsg := env.message(alice, "")
		stickerMsg.Sticker = &gotgbot.Sticker{
			FileId:     "anim1",
			IsAnimated: true,
			Emoji:      "😺",
			Thumbnail:  &gotgbot.PhotoSize{FileId: "thumb1", Width: 90, Height: 90},
		}
		env.deliver(stickerMsg)

		calls := env.provider.calls()
		if len(calls) != 2 {
			t.Fatalf("vision provider calls = %d, want 2", len(calls))
		}
		if len(calls[0].Attachments) != 1 || calls[0].Attachments[0].MIMEType != "image/jpeg" {
			t.Fatalf("photo attachments = %+v, want 1 image/jpeg", calls[0].Attachments)
		}
		if len(calls[1].Attachments) != 1 || calls[1].Attachments[0].MIMEType != "image/webp" {
			t.Fatalf("sticker attachments = %+v, want 1 image/webp", calls[1].Attachments)
		}
		if !strings.Contains(calls[1].User, "[sticker 😺]") {
			t.Fatalf("sticker user prompt missing [sticker 😺]: %s", calls[1].User)
		}

		// 2. Featherless 400 (!KeyRejected) on an image request retries once without the image and
		// remembers nothing, so the next photo is attempted with its image again.
		const model = "test/text-only-model"
		chatSwap(t, &config.AppConfig.ChatFeatherlessModel, model)
		env.mu.Lock()
		env.convSettings.Provider = chatconv.ProviderFeatherless
		env.mu.Unlock()

		env.provider.script(
			chatAnswerErr(&featherless.HTTPError{Status: http.StatusBadRequest, Type: "invalid_request_error"}),
			chatAnswerText(`{"emoji":"👀"}`),
			chatAnswerText(`{"emoji":"🔥"}`),
		)
		downloadsBefore := env.downloads()
		fPhoto1 := env.message(alice, "")
		fPhoto1.Caption = "look at this chart"
		fPhoto1.Photo = photoSizes()
		env.deliver(fPhoto1)

		calls = env.provider.calls()
		if len(calls) != 4 {
			t.Fatalf("featherless retry calls = %d, want 4", len(calls))
		}
		if len(calls[2].Attachments) != 1 || len(calls[3].Attachments) != 0 {
			t.Fatalf("featherless retry attachments = (%d, %d), want (1, 0)", len(calls[2].Attachments), len(calls[3].Attachments))
		}
		if env.downloads() != downloadsBefore+1 {
			t.Fatalf("first featherless photo downloads = %d, want %d", env.downloads(), downloadsBefore+1)
		}
		if env.mr.Exists("xenobot:cache:featherless_text_only:" + model) {
			t.Fatal("a vision rejection must not leave a text-only marker behind")
		}
		assertChatBreakersClean(t)

		// The next photo is attempted with its image again: nothing was remembered, so one
		// rejected request can never switch vision off for later requests or other chats.
		downloadsBefore2 := env.downloads()
		fPhoto2 := env.message(alice, "")
		fPhoto2.Caption = "another photo"
		fPhoto2.Photo = photoSizes()
		env.deliver(fPhoto2)
		if env.downloads() != downloadsBefore2+1 {
			t.Fatalf("second photo downloads = %d, want %d (vision is attempted every time)", env.downloads(), downloadsBefore2+1)
		}
		if len(env.provider.calls()[4].Attachments) != 1 {
			t.Fatal("second photo was sent without its image")
		}

		// 3. Custom-only AvailableReactions skips download and LLM call completely.
		cache.DeleteCache(presenceAvailReactKey(env.chatID))
		env.mu.Lock()
		env.availReactions = `[{"type":"custom_emoji","custom_emoji_id":"999999"}]`
		env.convSettings.Provider = chatconv.ProviderAnthropic
		env.mu.Unlock()

		shedBefore := chatReactShedCount.Load()
		dlBeforeCustom := env.downloads()
		callsBeforeCustom := len(env.provider.calls())
		customPhoto := env.message(alice, "")
		customPhoto.Caption = "custom only chat"
		customPhoto.Photo = photoSizes()
		env.deliver(customPhoto)
		if env.downloads() != dlBeforeCustom || len(env.provider.calls()) != callsBeforeCustom {
			t.Fatalf("custom-only chat unexpectedly downloaded (%d) or called LLM (%d)", env.downloads()-dlBeforeCustom, len(env.provider.calls())-callsBeforeCustom)
		}
		if chatReactShedCount.Load() != shedBefore+1 {
			t.Fatalf("custom-only shed count = %d, want %d", chatReactShedCount.Load(), shedBefore+1)
		}
	})
}

func TestChatReactProvidersAndNonTrippingShedding(t *testing.T) {
	t.Run("real Claude, Gemini, and Featherless adapters send MaxTokens=chatprompt.ReactionMaxTokens, zero PII, is_big=false, bot_react ai", func(t *testing.T) {
		var (
			mu     sync.Mutex
			bodies []string
		)
		providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			mu.Lock()
			bodies = append(bodies, string(raw))
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			switch {
			case strings.HasSuffix(r.URL.Path, ":generateContent"):
				_, _ = io.WriteString(w, `{"candidates":[{"content":{"role":"model","parts":[{"text":"{\"emoji\":\"🔥\"}"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":12,"candidatesTokenCount":6}}`)
			case strings.HasSuffix(r.URL.Path, "/v1/chat/completions"):
				_, _ = io.WriteString(w, `{"model":"Qwen/Qwen3-32B","choices":[{"message":{"role":"assistant","content":"{\"emoji\":\"👀\"}"},"finish_reason":"stop"}],"usage":{"prompt_tokens":12,"completion_tokens":6}}`)
			default:
				_, _ = io.WriteString(w, `{"model":"claude-haiku-5-5","stop_reason":"end_turn","content":[{"type":"text","text":"{\"emoji\":\"❤️\"}"}],"usage":{"input_tokens":12,"output_tokens":6}}`)
			}
		}))
		t.Cleanup(providerServer.Close)

		env := newChatReactEnv(t)
		chatSwap(t, &chatProviderOf, chatProviderFor)
		chatSwap(t, &chatAnthropicEndpoint, providerServer.URL)
		chatSwap(t, &chatGeminiEndpoint, providerServer.URL)
		chatSwap(t, &chatFeatherlessEndpoint, providerServer.URL)
		chatSwap(t, &chatProviderHTTP, providerServer.Client())
		chatSwap(t, &config.AppConfig.ChatAnthropicModel, "claude-haiku-5-5")
		chatSwap(t, &config.AppConfig.ChatGeminiModel, "gemini-4-hydrogen")
		chatSwap(t, &config.AppConfig.ChatFeatherlessModel, "Qwen/Qwen3-32B")

		alice := env.member("Alice")
		alice.Username = "alice_secret_handle"

		sentBefore := chatReactSentCount.Load()
		cases := []struct {
			provider  string
			wantEmoji string
		}{
			{chatconv.ProviderAnthropic, "❤"},
			{chatconv.ProviderGoogle, "🔥"},
			{chatconv.ProviderFeatherless, "👀"},
		}
		var msgs []*gotgbot.Message
		for _, tc := range cases {
			env.mu.Lock()
			env.convSettings.Provider = tc.provider
			env.mu.Unlock()
			m := env.message(alice, "look at this great "+tc.provider+" news")
			msgs = append(msgs, m)
			env.deliver(m)
		}

		if chatReactSentCount.Load() != sentBefore+3 {
			t.Fatalf("chatReactSentCount = %d, want %d", chatReactSentCount.Load(), sentBefore+3)
		}
		if stats := chatReactStatsSection(); !strings.Contains(stats, "<b>AI reactions:</b>") || !strings.Contains(stats, fmt.Sprintf("sent %d", chatReactSentCount.Load())) {
			t.Fatalf("chatReactStatsSection() = %q, want formatted AI reactions counters", stats)
		}

		mu.Lock()
		gotBodies := append([]string(nil), bodies...)
		mu.Unlock()
		if len(gotBodies) != 3 {
			t.Fatalf("wire provider calls = %d, want 3", len(gotBodies))
		}

		// Verify zero PII (user ID, chat ID, username) and MaxTokens == ReactionMaxTokens on all 3 wire formats.
		forbidden := []string{strconv.FormatInt(alice.Id, 10), strconv.FormatInt(env.chatID, 10), "alice_secret_handle"}
		for i, b := range gotBodies {
			for _, bad := range forbidden {
				if strings.Contains(b, bad) {
					t.Errorf("provider body[%d] leaked %q", i, bad)
				}
			}
		}
		var claudeReq struct {
			MaxTokens int `json:"max_tokens"`
		}
		var geminiReq struct {
			GenerationConfig struct {
				MaxOutputTokens int `json:"maxOutputTokens"`
			} `json:"generationConfig"`
		}
		var featherlessReq struct {
			MaxCompletionTokens int `json:"max_completion_tokens"`
		}
		if err := json.Unmarshal([]byte(gotBodies[0]), &claudeReq); err != nil || claudeReq.MaxTokens != chatprompt.ReactionMaxTokens {
			t.Errorf("claude max_tokens = %d (err=%v), want %d", claudeReq.MaxTokens, err, chatprompt.ReactionMaxTokens)
		}
		if err := json.Unmarshal([]byte(gotBodies[1]), &geminiReq); err != nil || geminiReq.GenerationConfig.MaxOutputTokens != chatprompt.ReactionMaxTokens {
			t.Errorf("gemini maxOutputTokens = %d (err=%v), want %d", geminiReq.GenerationConfig.MaxOutputTokens, err, chatprompt.ReactionMaxTokens)
		}
		if err := json.Unmarshal([]byte(gotBodies[2]), &featherlessReq); err != nil || featherlessReq.MaxCompletionTokens != chatprompt.ReactionMaxTokens {
			t.Errorf("featherless max_completion_tokens = %d (err=%v), want %d", featherlessReq.MaxCompletionTokens, err, chatprompt.ReactionMaxTokens)
		}

		reactCalls := env.callsFor("setMessageReaction")
		if len(reactCalls) != 3 {
			t.Fatalf("setMessageReaction calls = %d, want 3", len(reactCalls))
		}
		for i, tc := range cases {
			if isBig, ok := reactCalls[i].Params["is_big"]; ok && isBig != "false" {
				t.Errorf("provider %s set is_big=%q, want false/omitted", tc.provider, isBig)
			}
			rec, ok := presenceReadBotReact(context.Background(), env.rc, env.chatID, msgs[i].MessageId)
			if !ok || rec.Emoji != tc.wantEmoji || rec.AuthorID != alice.Id || rec.Source != "ai" {
				t.Errorf("bot_react[%s] = %+v (ok=%v), want emoji=%q author=%d source=ai", tc.provider, rec, ok, tc.wantEmoji, alice.Id)
			}
			key := presenceBotReactKey(env.chatID, msgs[i].MessageId)
			if strings.HasPrefix(key, "xenobot:cache:") || env.mr.TTL(key) < 23*time.Hour {
				t.Errorf("bot_react key %q namespace or TTL (%v) wrong", key, env.mr.TTL(key))
			}
		}
		assertChatBreakersClean(t)
	})

	t.Run("non-tripping shedding across parse, lookalike, provider, and Telegram 400 errors plus pre-paused breaker", func(t *testing.T) {
		env := newChatReactEnv(t)
		chatSwap(t, &chatReactUserHourlyBudget, int64(20))
		alice := env.member("Alice")

		noneBefore := chatReactNoneCount.Load()
		shedBefore := chatReactShedCount.Load()

		env.provider.script(
			chatAnswerText(`{"emoji":"NONE"}`),
			func(context.Context, chatRequest) (chatAnswer, error) { return chatAnswer{Outcome: chatRefused}, nil },
			chatAnswerText(`not json at all`),
			chatAnswerText(`{"emoji":"😂"}`),
			chatAnswerText(`{"emoji":"🐱"}`),
			chatAnswerErr(claude.ErrNoReply),
			chatAnswerErr(featherless.ErrNoReply),
			chatAnswerErr(&featherless.HTTPError{Status: http.StatusUnprocessableEntity, Type: "invalid_request_error"}),
			chatAnswerErr(&featherless.HTTPError{Status: http.StatusBadRequest, Type: "invalid_request_error"}),
		)
		for i := range 9 {
			env.deliver(env.message(alice, fmt.Sprintf("shed case %d", i)))
		}

		// Telegram 400 REACTION_INVALID sheds cleanly and evicts avail_react cache.
		env.mu.Lock()
		env.reactErr = "Bad Request: REACTION_INVALID"
		env.mu.Unlock()
		env.provider.script(chatAnswerText(`{"emoji":"🔥"}`))
		env.deliver(env.message(alice, "telegram 400 invalid"))
		env.mu.Lock()
		env.reactErr = ""
		env.mu.Unlock()

		if chatReactNoneCount.Load() != noneBefore+2 {
			t.Errorf("chatReactNoneCount = %d, want %d", chatReactNoneCount.Load(), noneBefore+2)
		}
		if chatReactShedCount.Load() != shedBefore+8 {
			t.Errorf("chatReactShedCount = %d, want %d", chatReactShedCount.Load(), shedBefore+8)
		}
		assertChatBreakersClean(t)

		// Pre-paused breaker skips provider call cleanly.
		callsBeforePause := len(env.provider.calls())
		chatBreaker(chatconv.ProviderAnthropic).pauseProvider(raidNow(), 5*time.Minute)
		env.deliver(env.message(alice, "while breaker paused"))
		if len(env.provider.calls()) != callsBeforePause {
			t.Fatalf("pre-paused breaker still called provider: got %d extra calls", len(env.provider.calls())-callsBeforePause)
		}
	})
}

func TestShutdownEndsChatReact(t *testing.T) {
	env := newChatReactEnv(t)
	chatSwap(t, &chatStopWait, 2*time.Second)
	started := make(chan struct{})
	env.provider.script(func(ctx context.Context, _ chatRequest) (chatAnswer, error) {
		close(started)
		<-ctx.Done()
		return chatAnswer{}, ctx.Err()
	})
	restoreChatPool(t)

	msg := env.message(env.member("ShutdownUser"), "hang right here")
	env.nextUpdateID++
	if err := env.dispatcher.ProcessUpdate(env.bot, &gotgbot.Update{UpdateId: env.nextUpdateID, Message: msg}, nil); err != nil {
		t.Fatalf("ProcessUpdate: %v", err)
	}
	<-started
	if !StopChat() {
		t.Fatal("StopChat timed out waiting for hung reaction job")
	}

	// Late message after StopChat must be shed without calling provider or sending a reaction.
	env.deliver(env.message(env.member("LateUser"), "after shutdown"))
	if got := len(env.callsFor("setMessageReaction")); got != 0 {
		t.Fatalf("setMessageReaction calls = %d, want 0", got)
	}
	if got := len(env.provider.calls()); got != 1 {
		t.Fatalf("provider calls = %d, want 1", got)
	}
}

// TestChatReactMovingPictures proves a GIF, a video and a round video message reach the reaction
// lane as their preview frame, with the prompt saying so, and that a picture that cannot be fetched
// is shed instead of being reacted to from its label.
func TestChatReactMovingPictures(t *testing.T) {
	env := newChatReactEnv(t)
	alice := env.member("Alice")
	thumb := &gotgbot.PhotoSize{FileId: "thumb1", Width: 90, Height: 90}
	env.mu.Lock()
	env.files["files/thumb1"] = raidAITestJPEG("react-frame")
	env.mu.Unlock()

	for _, tc := range []struct {
		name     string
		set      func(*gotgbot.Message)
		wantNoun string
	}{
		{"GIF", func(m *gotgbot.Message) { m.Animation = &gotgbot.Animation{FileId: "gif1", Thumbnail: thumb} }, "a GIF"},
		{"video", func(m *gotgbot.Message) { m.Video = &gotgbot.Video{FileId: "vid1", Thumbnail: thumb} }, "a video"},
		{"round video message", func(m *gotgbot.Message) { m.VideoNote = &gotgbot.VideoNote{FileId: "note1", Thumbnail: thumb} }, "a round video message"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := len(env.provider.calls())
			env.provider.script(chatAnswerText(`{"emoji":"🔥"}`))
			msg := env.message(alice, "")
			tc.set(msg)
			env.deliver(msg)

			calls := env.provider.calls()
			if len(calls) != before+1 {
				t.Fatalf("provider calls = %d, want %d", len(calls), before+1)
			}
			got := calls[before]
			if len(got.Attachments) != 1 || got.Attachments[0].MIMEType != "image/jpeg" {
				t.Fatalf("attachments = %+v, want the one JPEG preview frame", got.Attachments)
			}
			if !strings.Contains(got.User, "only one still preview frame of "+tc.wantNoun) {
				t.Errorf("prompt does not say the image is a preview frame of %s:\n%s", tc.wantNoun, got.User)
			}
		})
	}

	t.Run("an unfetchable frame with no words is shed, not reacted to from its label", func(t *testing.T) {
		before := len(env.provider.calls())
		shedBefore := chatReactShedCount.Load()
		msg := env.message(alice, "")
		msg.Animation = &gotgbot.Animation{FileId: "gif2", Thumbnail: &gotgbot.PhotoSize{FileId: "missing-frame", Width: 90, Height: 90}}
		env.deliver(msg)
		if len(env.provider.calls()) != before {
			t.Fatal("the provider was asked to react to a picture that was never attached")
		}
		if chatReactShedCount.Load() != shedBefore+1 {
			t.Fatalf("shed count = %d, want %d", chatReactShedCount.Load(), shedBefore+1)
		}
	})

	t.Run("an unfetchable frame with a caption still reacts to the words", func(t *testing.T) {
		before := len(env.provider.calls())
		env.provider.script(chatAnswerText(`{"emoji":"👀"}`))
		msg := env.message(alice, "")
		msg.Caption = "my cat doing a flip"
		msg.Video = &gotgbot.Video{FileId: "vid2", Thumbnail: &gotgbot.PhotoSize{FileId: "missing-frame", Width: 90, Height: 90}}
		env.deliver(msg)
		calls := env.provider.calls()
		if len(calls) != before+1 {
			t.Fatalf("provider calls = %d, want %d", len(calls), before+1)
		}
		if len(calls[before].Attachments) != 0 {
			t.Fatalf("attachments = %+v, want none", calls[before].Attachments)
		}
	})
}
