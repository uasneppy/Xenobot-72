package modules

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/Xenobot-72/xenobot/config"
	"github.com/uasneppy/Xenobot-72/xenobot/db/chatconv"
	"github.com/uasneppy/Xenobot-72/xenobot/db/disabling"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/chatprompt"
)

// briefCommand builds a /unrollshort message from from, replying to target (nil for no reply).
func (e *chatEnv) briefCommand(from *gotgbot.User, target *gotgbot.Message) *gotgbot.Message {
	const name = "/unrollshort"
	msg := e.message(from, name)
	msg.Entities = []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: int64(len(name))}}
	msg.ReplyToMessage = target
	return msg
}

// briefTalk delivers a short conversation and returns its messages.
func briefTalk(e *chatEnv, alice, bob, carol *gotgbot.User) []*gotgbot.Message {
	first := e.message(alice, "before the story")
	second := e.message(bob, "the elevator is stuck <again>")
	third := e.message(carol, "I called maintenance")
	fourth := e.message(alice, "")
	fourth.Caption = "see the photo"
	fourth.Photo = []gotgbot.PhotoSize{{FileId: "f", FileUniqueId: "u", Width: 1, Height: 1}}
	msgs := []*gotgbot.Message{first, second, third, fourth}
	for _, msg := range msgs {
		e.deliver(msg)
	}
	return msgs
}

func (e *chatEnv) markDeleted(id int64) {
	e.fake.mu.Lock()
	e.fake.deleted[id] = true
	e.fake.mu.Unlock()
}

func TestChatBriefShort(t *testing.T) {
	env := newChatEnv(t)
	chatLogTurnOn(t, env)
	alice, bob, carol, dave := env.member("Alice"), env.member("Bob"), env.member("Carol"), env.member("Dave")
	msgs := briefTalk(env, alice, bob, carol)
	if len(env.provider.calls()) != 0 || len(env.sends()) != 0 {
		t.Fatal("talking alone started a brief")
	}

	env.provider.script(chatAnswerText("Bob said the lift is stuck & Carol called maintenance."))
	cmd := env.briefCommand(dave, msgs[1])
	env.markDeleted(cmd.MessageId) // the command deletion removes it before the reply goes out
	env.deliver(cmd)

	calls := env.provider.calls()
	if len(calls) != 1 {
		t.Fatalf("provider calls = %d, want 1", len(calls))
	}
	req := calls[0]
	for _, want := range []string{"Bob: the elevator is stuck <again>", "Carol: I called maintenance", "Alice: see the photo [photo]"} {
		if !strings.Contains(req.User, want) {
			t.Errorf("the request lacks %q:\n%s", want, req.User)
		}
	}
	for _, unwanted := range []string{"before the story", "/unrollshort"} {
		if strings.Contains(req.User, unwanted) {
			t.Errorf("the request holds %q", unwanted)
		}
	}
	if req.Static != chatBriefInstruction(chatBriefShort) || req.Static == chatprompt.Static() {
		t.Error("the static text is not the brief instruction")
	}
	if req.MaxTokens != chatBriefMaxTokens(chatBriefShort) {
		t.Errorf("MaxTokens = %d", req.MaxTokens)
	}
	for _, text := range []string{req.Static, req.Dynamic, req.User} {
		for _, id := range []int64{alice.Id, bob.Id, carol.Id, dave.Id, env.chatID} {
			if strings.Contains(text, strconv.FormatInt(id, 10)) {
				t.Fatalf("the ID %d reached the provider", id)
			}
		}
	}

	sends := env.sends()
	if len(sends) != 1 {
		t.Fatalf("sends = %q, want one brief", sends)
	}
	params := env.lastSend()
	text := fmt.Sprint(params["text"])
	for _, want := range []string{chatCanned(t, "brief_header_short"), "Bob said the lift is stuck &amp; Carol called maintenance.", chatCanned(t, "brief_ai_notice")} {
		if !strings.Contains(text, want) {
			t.Errorf("the brief %q lacks %q", text, want)
		}
	}
	if params["parse_mode"] != "HTML" {
		t.Errorf("parse_mode = %v, want HTML", params["parse_mode"])
	}
	reply := chatReplyTo(t, params)
	if reply.MessageId != cmd.MessageId || !reply.AllowSendingWithoutReply {
		t.Errorf("reply parameters %+v, want the command with allow_sending_without_reply", reply)
	}
}

func TestChatBriefRefusals(t *testing.T) {
	env := newChatEnv(t)
	chatLogTurnOn(t, env)
	alice, bob, carol := env.member("Alice"), env.member("Bob"), env.member("Carol")
	msgs := briefTalk(env, alice, bob, carol)
	asker := env.member("Dave")

	noCalls := func(t *testing.T, wantSends ...string) {
		t.Helper()
		if n := len(env.provider.calls()); n != 0 {
			t.Fatalf("a refusal made %d provider calls", n)
		}
		if got := env.sends(); fmt.Sprint(got) != fmt.Sprint(wantSends) {
			t.Fatalf("sends = %q, want %q", got, wantSends)
		}
	}

	t.Run("no reply target answers the usage", func(t *testing.T) {
		env.deliver(env.briefCommand(asker, nil))
		noCalls(t, chatCanned(t, "brief_usage_short"))
	})
	t.Run("a message that is not in the log answers too old", func(t *testing.T) {
		env.deliver(env.briefCommand(asker, env.message(alice, "sent before the log was on")))
		noCalls(t, chatCanned(t, "brief_usage_short"), chatCanned(t, "brief_too_old"))
	})
	t.Run("the log off answers the hint", func(t *testing.T) {
		if err := chatconv.SetLogEnabled(context.Background(), env.chatID, false, 1); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = chatconv.SetLogEnabled(context.Background(), env.chatID, true, 1) })
		env.deliver(env.briefCommand(asker, msgs[1]))
		noCalls(t, chatCanned(t, "brief_usage_short"), chatCanned(t, "brief_too_old"), chatCanned(t, "brief_off_hint"))
	})
	t.Run("a stored provider this server cannot run answers unavailable", func(t *testing.T) {
		withChatProviders(t, true, false, true) // the stored provider is anthropic
		before := len(env.sends())
		env.deliver(env.briefCommand(asker, msgs[1]))
		if got := env.sends(); len(got) != before+1 || got[before] != chatCanned(t, "brief_unavailable") {
			t.Fatalf("sends = %q", got)
		}
		if n := len(env.provider.calls()); n != 0 {
			t.Fatalf("%d provider calls", n)
		}
	})
	t.Run("a disabled command, a private chat and an anonymous admin get no brief", func(t *testing.T) {
		before := len(env.sends())
		if err := disabling.DisableCMD(env.chatID, "unrollshort"); err != nil {
			t.Fatal(err)
		}
		env.deliver(env.briefCommand(asker, msgs[1]))
		_ = disabling.EnableCMD(env.chatID, "unrollshort")

		private := env.briefCommand(asker, msgs[1])
		private.Chat = gotgbot.Chat{Id: asker.Id, Type: "private"}
		env.deliver(private)

		anonymous := env.briefCommand(&gotgbot.User{Id: 1087968824, IsBot: true, FirstName: "Group"}, msgs[1])
		anonymous.SenderChat = &gotgbot.Chat{Id: env.chatID, Type: "supergroup"}
		env.deliver(anonymous)

		if n := len(env.provider.calls()); n != 0 {
			t.Fatalf("%d provider calls", n)
		}
		for _, text := range env.sends()[before:] {
			if strings.Contains(text, chatCanned(t, "brief_ai_notice")) {
				t.Fatalf("a brief was posted: %q", text)
			}
		}
	})
}

func TestChatBriefLimits(t *testing.T) {
	env := newChatEnv(t)
	chatLogTurnOn(t, env)
	alice, bob, carol := env.member("Alice"), env.member("Bob"), env.member("Carol")
	msgs := briefTalk(env, alice, bob, carol)
	asker := env.member("Dave")
	ask := func() { env.deliver(env.briefCommand(asker, msgs[1])) }
	notice := chatCanned(t, "brief_ai_notice")
	briefs := func() int {
		n := 0
		for _, text := range env.sends() {
			if strings.Contains(text, notice) {
				n++
			}
		}
		return n
	}

	ask()
	ask() // inside the cooldown: silent
	if briefs() != 1 || len(env.provider.calls()) != 1 || len(env.sends()) != 1 {
		t.Fatalf("cooldown: %d briefs, %d calls, sends %q", briefs(), len(env.provider.calls()), env.sends())
	}

	// The user budget: the second brief of an hour is refused with the allowance line, once.
	chatSwap(t, &chatBriefUserHourlyBudget, 1)
	env.mr.FastForward(time.Minute)
	ask()
	env.mr.FastForward(time.Minute)
	ask()
	allowance := chatCanned(t, "brief_allowance")
	lines := 0
	for _, text := range env.sends() {
		if text == allowance {
			lines++
		}
	}
	if lines != 1 || briefs() != 1 || len(env.provider.calls()) != 1 {
		t.Fatalf("budget: %d allowance lines, %d briefs, %d calls", lines, briefs(), len(env.provider.calls()))
	}

	// A paused breaker answers the malfunction line, once, with no provider call.
	chatSwap(t, &chatUserHourlyBudget, chatUserHourlyBudget)
	chatSwap(t, &chatBriefUserHourlyBudget, 100)
	chatBreaker(chatconv.ProviderAnthropic).pauseProvider(raidNow(), 5*time.Minute)
	env.mr.FastForward(time.Minute)
	ask()
	env.mr.FastForward(time.Minute)
	ask()
	malfunction := chatCanned(t, "brief_malfunction")
	count := 0
	for _, text := range env.sends() {
		if text == malfunction {
			count++
		}
	}
	if count != 1 || len(env.provider.calls()) != 1 {
		t.Fatalf("paused breaker: %d malfunction lines, %d calls", count, len(env.provider.calls()))
	}
}

func TestChatBriefProviderAnswers(t *testing.T) {
	env := newChatEnv(t)
	chatLogTurnOn(t, env)
	msgs := briefTalk(env, env.member("Alice"), env.member("Bob"), env.member("Carol"))
	ask := func() {
		env.mr.FastForward(time.Minute)
		env.deliver(env.briefCommand(env.member("Dave"), msgs[1]))
	}

	env.provider.script(func(context.Context, chatRequest) (chatAnswer, error) {
		return chatAnswer{Outcome: chatRefused}, nil
	})
	ask()
	if got := env.sends(); len(got) != 1 || got[0] != chatCanned(t, "brief_refused") {
		t.Fatalf("a refusal answered %q", got)
	}

	env.provider.script(chatAnswerErr(errors.New("provider down")))
	ask()
	if got := env.sends(); len(got) != 2 || got[1] != chatCanned(t, "brief_malfunction") {
		t.Fatalf("a provider error answered %q", got)
	}

	// A truncated answer keeps its text and ends with an ellipsis; a blank one is a malfunction.
	env.mr.Del(chatSaidKey(env.chatID, chatBriefSaidMalfunction))
	env.provider.script(func(context.Context, chatRequest) (chatAnswer, error) {
		return chatAnswer{Text: "Cut off here", Outcome: chatTruncated}, nil
	})
	ask()
	if text := fmt.Sprint(env.lastSend()["text"]); !strings.Contains(text, "Cut off here…") {
		t.Fatalf("a truncated answer was posted as %q", text)
	}
	env.provider.script(chatAnswerText("   "))
	ask()
	if got := env.sends(); got[len(got)-1] != chatCanned(t, "brief_malfunction") {
		t.Fatalf("a blank answer answered %q", got[len(got)-1])
	}
}

func TestChatBriefQueue(t *testing.T) {
	env := newChatEnv(t)
	gate := make(chan struct{})
	chatSwap(t, &chatBriefProcess, func(chatBriefJob) { <-gate })
	t.Cleanup(func() {
		select {
		case <-gate:
		default:
			close(gate)
		}
	})
	job := chatBriefJob{bot: env.bot, chatID: env.chatID, userID: 1, messageID: 1, fromID: 1, provider: chatconv.ProviderAnthropic}
	shed := chatBriefShedCount.Load()
	if !chatBriefEnqueue(job) {
		t.Fatal("the first job was shed")
	}
	deadline := time.Now().Add(2 * time.Second)
	for len(chatBriefQueue) != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	for i := range chatBriefQueueCapacity {
		if !chatBriefEnqueue(job) {
			t.Fatalf("job %d was shed before the queue was full", i)
		}
	}
	if chatBriefEnqueue(job) {
		t.Fatal("a full queue accepted a job")
	}
	if chatBriefShedCount.Load() != shed+1 {
		t.Error("a full queue did not count the shed job")
	}
	close(gate)
	chatJobs.wait()

	// Draining sheds too.
	restoreChatPool(t)
	chatDraining.Store(true)
	if chatBriefEnqueue(job) {
		t.Fatal("a draining process accepted a job")
	}
}

// TestChatBriefEndToEnd is the whole path through the real dispatcher with only Telegram and the
// provider's server faked: the owner turns the Message log on with one tap on the private page,
// members talk, a member replies /unrollshort to a message, the real Anthropic adapter asks the
// stand-in and the brief is posted; turning the log off deletes the ring.
func TestChatBriefEndToEnd(t *testing.T) {
	withChatProviders(t, true, true, true)
	settings := settingsChatEnv(t)
	owner := settings.newOwner(settings.chat)
	private := settingsPrivateChat(owner)
	msgID := settingsChatOpenPrivate(t, settings, owner)
	if settingsChatRows(t, settings.chat.Id) != 0 {
		t.Fatal("opening the page created a row")
	}
	_, keyboard := settings.lastEdit(owner.Id, msgID)
	toggle, ok := settingsChatFind(t, keyboard, func(f map[string]string) bool { return f["a"] == "tg" && f["t"] == settingsChatControlLog })
	if !ok {
		t.Fatalf("no Message log toggle on the page: %+v", keyboard)
	}
	settings.pressRaw(private, owner, msgID, toggle.CallbackData)
	if got := settingsChatStored(t, settings.chat.Id); !got.LogEnabled || got.Enabled {
		t.Fatalf("one tap stored %+v", got)
	}

	env := newChatEnvFor(t, settings.chat.Id)
	standIn := &chatProviderStandIn{}
	server := httptest.NewServer(http.HandlerFunc(standIn.handler))
	t.Cleanup(server.Close)
	chatSwap(t, &chatProviderOf, chatProviderFor)
	chatSwap(t, &chatAnthropicEndpoint, server.URL)
	chatSwap(t, &chatProviderHTTP, server.Client())
	chatSwap(t, &config.AppConfig.ChatAnthropicModel, "claude-haiku-5-5")

	msgs := briefTalk(env, env.member("Alice"), env.member("Bob"), env.member("Carol"))
	if len(standIn.bodies) != 0 {
		t.Fatal("talking called the provider")
	}
	env.deliver(env.briefCommand(env.member("Dave"), msgs[1]))
	sends := env.sends()
	if len(sends) != 1 || !strings.Contains(sends[0], "Claude unit on duty.") || !strings.Contains(sends[0], chatCanned(t, "brief_ai_notice")) {
		t.Fatalf("sends = %q, want the brief", sends)
	}
	if len(standIn.bodies) != 1 {
		t.Fatalf("provider calls = %d, want 1", len(standIn.bodies))
	}
	var body struct {
		MaxTokens int `json:"max_tokens"`
		Messages  []struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(standIn.bodies[0]), &body); err != nil || len(body.Messages) != 1 {
		t.Fatalf("body: %v", err)
	}
	if body.MaxTokens != chatBriefMaxTokens(chatBriefShort) {
		t.Errorf("max_tokens = %d", body.MaxTokens)
	}
	if user := body.Messages[0].Content[0].Text; !strings.Contains(user, "Bob: the elevator is stuck") || strings.Contains(user, "before the story") {
		t.Errorf("the request holds the wrong stretch:\n%s", user)
	}

	// Off deletes the ring.
	if !env.mr.Exists(chatLogKey(env.chatID)) {
		t.Fatal("no ring was written")
	}
	_, keyboard = settings.lastEdit(owner.Id, msgID)
	off, ok := settingsChatFind(t, keyboard, func(f map[string]string) bool { return f["t"] == settingsChatControlLog && f["v"] == "0" })
	if !ok {
		t.Fatalf("no Off toggle: %+v", keyboard)
	}
	settings.pressRaw(private, owner, msgID, off.CallbackData)
	if settingsChatStored(t, settings.chat.Id).LogEnabled || env.mr.Exists(chatLogKey(env.chatID)) {
		t.Fatal("Off left the log on or the ring behind")
	}
}

func TestChatBriefLong(t *testing.T) {
	env := newChatEnv(t)
	chatLogTurnOn(t, env)
	alice, bob, carol := env.member("Alice"), env.member("Bob"), env.member("Carol")
	msgs := briefTalk(env, alice, bob, carol)
	asker := env.member("Dave")

	env.deliver(env.commandMessage(asker, "/unrolllong", nil))
	env.deliver(env.commandMessage(asker, "/unrolllong", env.message(alice, "not logged")))
	if got := env.sends(); len(got) != 2 || got[0] != chatCanned(t, "brief_usage_long") || got[1] != chatCanned(t, "brief_too_old") {
		t.Fatalf("refusals %q, want the long usage and too old", got)
	}
	if n := len(env.provider.calls()); n != 0 {
		t.Fatalf("a refusal made %d provider calls", n)
	}

	env.provider.script(chatAnswerText("Bob reported the lift. Carol called maintenance."))
	env.deliver(env.commandMessage(asker, "/unrolllong", msgs[1]))
	calls := env.provider.calls()
	if len(calls) != 1 || calls[0].MaxTokens != chatBriefMaxTokens(chatBriefLong) || calls[0].Static != chatBriefInstruction(chatBriefLong) {
		t.Fatalf("provider calls %+v, want one long request", calls)
	}
	last := env.sends()[len(env.sends())-1]
	for _, want := range []string{chatCanned(t, "brief_header_long"), "Bob reported the lift.", chatCanned(t, "brief_ai_notice")} {
		if !strings.Contains(last, want) {
			t.Errorf("the brief %q lacks %q", last, want)
		}
	}
}
