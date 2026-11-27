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

func TestChatBriefReplyFree(t *testing.T) {
	t.Run("argument parser", func(t *testing.T) {
		cases := []struct {
			name     string
			arg      string
			hasReply bool
			wantMode chatBriefMode
			wantN    int
			wantDur  time.Duration
			wantPM   bool
			wantOK   bool
		}{
			{name: "reply empty", arg: "", hasReply: true, wantMode: chatBriefModeReply, wantOK: true},
			{name: "reply pm", arg: "PM", hasReply: true, wantMode: chatBriefModeReply, wantPM: true, wantOK: true},
			{name: "reply conflicting positional", arg: "50", hasReply: true, wantOK: false},
			{name: "reply conflicting positional with pm", arg: "2h pm", hasReply: true, wantOK: false},
			{name: "count min", arg: "5", wantMode: chatBriefModeCount, wantN: 5, wantOK: true},
			{name: "count max", arg: "200 pm", wantMode: chatBriefModeCount, wantN: 200, wantPM: true, wantOK: true},
			{name: "count below min", arg: "4", wantOK: false},
			{name: "count zero", arg: "0", wantOK: false},
			{name: "count above max", arg: "201", wantOK: false},
			{name: "count signed", arg: "+10", wantOK: false},
			{name: "me", arg: "Me", wantMode: chatBriefModeMe, wantOK: true},
			{name: "me pm", arg: "me pm", wantMode: chatBriefModeMe, wantPM: true, wantOK: true},
			{name: "duration 1m", arg: "1m", wantMode: chatBriefModeTime, wantDur: time.Minute, wantOK: true},
			{name: "duration 2H pm", arg: "2H pm", wantMode: chatBriefModeTime, wantDur: 2 * time.Hour, wantPM: true, wantOK: true},
			{name: "duration 7d", arg: "7d", wantMode: chatBriefModeTime, wantDur: 7 * 24 * time.Hour, wantOK: true},
			{name: "duration 30s unsupported", arg: "30s", wantOK: false},
			{name: "duration 0m", arg: "0m", wantOK: false},
			{name: "duration 30d over cap", arg: "30d", wantOK: false},
			{name: "empty without reply", arg: "", wantOK: false},
			{name: "pm only without reply", arg: "pm", wantOK: false},
			{name: "unknown token", arg: "foo", wantOK: false},
			{name: "extra trailing token", arg: "2h pm extra", wantOK: false},
			{name: "pm before anchor", arg: "pm 2h", wantOK: false},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				mode, n, dur, toPM, ok := chatBriefParseArgs(tc.arg, tc.hasReply)
				if ok != tc.wantOK || mode != tc.wantMode || n != tc.wantN || dur != tc.wantDur || toPM != tc.wantPM {
					t.Fatalf("chatBriefParseArgs(%q, %v) = (%v, %d, %v, %v, %v), want (%v, %d, %v, %v, %v)",
						tc.arg, tc.hasReply, mode, n, dur, toPM, ok,
						tc.wantMode, tc.wantN, tc.wantDur, tc.wantPM, tc.wantOK)
				}
			})
		}
	})

	t.Run("count me duration topic isolation and pm delivery", func(t *testing.T) {
		env := newChatEnv(t)
		chatLogTurnOn(t, env)
		chatSwap(t, &chatBriefUserHourlyBudget, int64(100))
		deliverBrief := func(cmd *gotgbot.Message) {
			env.mr.FastForward(chatBriefCooldown + time.Second)
			env.deliver(cmd)
		}

		base := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
		chatSwap(t, &raidNow, func() time.Time { return base })

		alice := env.member("Alice")
		bob := env.member("Bob")
		carol := env.member("Carol")
		asker := env.member("Dave")

		topicMsg := func(from *gotgbot.User, threadID int64, at time.Time, text string) *gotgbot.Message {
			m := env.message(from, text)
			m.Chat.Title = "Facility <7>"
			m.Date = at.Unix()
			if threadID > 0 {
				m.IsTopicMessage = true
				m.MessageThreadId = threadID
			}
			return m
		}
		topicCmd := func(from *gotgbot.User, threadID int64, text string, target *gotgbot.Message) *gotgbot.Message {
			m := env.commandMessage(from, text, target)
			m.Chat.Title = "Facility <7>"
			m.Date = base.Unix()
			if threadID > 0 {
				m.IsTopicMessage = true
				m.MessageThreadId = threadID
			}
			return m
		}

		// Topic 10 messages (some older than 2h, some inside 2h, with Alice anchored in the middle).
		env.deliver(topicMsg(bob, 10, base.Add(-4*time.Hour), "t10-ancient"))
		env.deliver(topicMsg(carol, 10, base.Add(-3*time.Hour), "t10-before-alice"))
		env.deliver(topicMsg(alice, 10, base.Add(-90*time.Minute), "t10-alice-anchor"))
		env.deliver(topicMsg(bob, 10, base.Add(-45*time.Minute), "t10-after-1"))
		env.deliver(topicMsg(carol, 10, base.Add(-20*time.Minute), "t10-after-2"))
		env.deliver(topicMsg(bob, 10, base.Add(-10*time.Minute), "t10-after-3"))
		// Topic 20 noise that must never leak into Topic 10 queries.
		env.deliver(topicMsg(alice, 20, base.Add(-5*time.Minute), "t20-other-topic"))

		// 1. /unrollshort 5 in Topic 10: gets last 5 in Topic 10 only (excludes t10-ancient & t20-other-topic).
		env.provider.script(chatAnswerText("Summary of last 5 in topic 10."))
		deliverBrief(topicCmd(asker, 10, "/unrollshort 5", nil))
		calls := env.provider.calls()
		if len(calls) != 1 {
			t.Fatalf("count provider calls = %d, want 1", len(calls))
		}
		if strings.Contains(calls[0].User, "t10-ancient") || strings.Contains(calls[0].User, "t20-other-topic") {
			t.Fatalf("count query leaked outside range/topic:\n%s", calls[0].User)
		}
		for _, want := range []string{"t10-before-alice", "t10-alice-anchor", "t10-after-3"} {
			if !strings.Contains(calls[0].User, want) {
				t.Fatalf("count query missing %q:\n%s", want, calls[0].User)
			}
		}
		last := env.lastSend()
		if fmt.Sprint(last["message_thread_id"]) != "10" {
			t.Fatalf("topic 10 reply thread_id = %v, want 10", last["message_thread_id"])
		}

		// 2. /unrolllong 20 in Topic 10: uses chatBriefLong instruction and MaxTokens.
		env.provider.script(chatAnswerText("Detailed summary."))
		deliverBrief(topicCmd(asker, 10, "/unrolllong 20", nil))
		calls = env.provider.calls()
		if len(calls) != 2 || calls[1].Static != chatBriefInstruction(chatBriefLong) || calls[1].MaxTokens != chatBriefMaxTokens(chatBriefLong) {
			t.Fatalf("long count call mismatch: %+v", calls)
		}

		// 3. /unrollshort me for Alice in Topic 10: starts at t10-alice-anchor, excludes earlier & Topic 20.
		env.provider.script(chatAnswerText("Since Alice spoke."))
		deliverBrief(topicCmd(alice, 10, "/unrollshort me", nil))
		calls = env.provider.calls()
		if len(calls) != 3 {
			t.Fatalf("me provider calls = %d, want 3", len(calls))
		}
		if strings.Contains(calls[2].User, "t10-before-alice") || strings.Contains(calls[2].User, "t20-other-topic") {
			t.Fatalf("me query leaked earlier or other-topic messages:\n%s", calls[2].User)
		}
		if !strings.Contains(calls[2].User, "t10-alice-anchor") || !strings.Contains(calls[2].User, "t10-after-3") {
			t.Fatalf("me query missing expected stretch:\n%s", calls[2].User)
		}

		// 4. /unrollshort 2h in Topic 10: uses raidNow()-2h cutoff against msg.Date (At).
		env.provider.script(chatAnswerText("Last 2 hours."))
		deliverBrief(topicCmd(asker, 10, "/unrollshort 2h", nil))
		calls = env.provider.calls()
		if len(calls) != 4 {
			t.Fatalf("duration provider calls = %d, want 4", len(calls))
		}
		if strings.Contains(calls[3].User, "t10-before-alice") || strings.Contains(calls[3].User, "t10-ancient") || strings.Contains(calls[3].User, "t20-other-topic") {
			t.Fatalf("2h query leaked older or other-topic messages:\n%s", calls[3].User)
		}
		if !strings.Contains(calls[3].User, "t10-alice-anchor") || !strings.Contains(calls[3].User, "t10-after-3") {
			t.Fatalf("2h query missing messages inside 2h:\n%s", calls[3].User)
		}

		// 5. /unrollshort 5 pm: sends PM with escaped group title header and brief_note_pm in group.
		env.provider.script(chatAnswerText("Private catch-up."))
		groupBefore := len(env.sends())
		deliverBrief(topicCmd(asker, 10, "/unrollshort 5 pm", nil))
		pmCalls := env.client.callsFor("sendMessage")
		var pmParams map[string]any
		for _, c := range pmCalls {
			if fmt.Sprint(c.Params["chat_id"]) == fmt.Sprint(asker.Id) {
				pmParams = c.Params
			}
		}
		if pmParams == nil {
			t.Fatalf("expected PM sendMessage to user %d", asker.Id)
		}
		pmText := fmt.Sprint(pmParams["text"])
		if !strings.Contains(pmText, "Facility &lt;7&gt;") || !strings.Contains(pmText, "Private catch-up.") {
			t.Fatalf("PM brief %q missing escaped title or body", pmText)
		}
		if pmParams["message_thread_id"] != nil && fmt.Sprint(pmParams["message_thread_id"]) != "0" {
			t.Fatalf("PM brief has non-zero message_thread_id = %v", pmParams["message_thread_id"])
		}
		if groupSends := env.sends(); len(groupSends) != groupBefore+1 || groupSends[len(groupSends)-1] != chatCanned(t, "brief_note_pm") {
			t.Fatalf("group sends after PM = %q, want trailing brief_note_pm", groupSends)
		}

		// 6. PM 403 -> brief_pm_blocked in group.
		env.fake.refuse(asker.Id)
		env.provider.script(chatAnswerText("Blocked PM catch-up."))
		groupBefore = len(env.sends())
		deliverBrief(topicCmd(asker, 10, "/unrolllong 20 pm", nil))
		if groupSends := env.sends(); len(groupSends) != groupBefore+1 || groupSends[len(groupSends)-1] != chatCanned(t, "brief_pm_blocked") {
			t.Fatalf("group sends after 403 PM = %q, want trailing brief_pm_blocked", groupSends)
		}

		// 7. Empty topic 30 -> brief_empty with zero provider calls.
		callsBefore := len(env.provider.calls())
		for _, cmd := range []string{"/unrollshort 5", "/unrollshort me", "/unrollshort 2h"} {
			groupBefore = len(env.sends())
			deliverBrief(topicCmd(asker, 30, cmd, nil))
			if groupSends := env.sends(); len(groupSends) != groupBefore+1 || groupSends[len(groupSends)-1] != chatCanned(t, "brief_empty") {
				t.Fatalf("%s on empty topic answered %q, want brief_empty", cmd, groupSends)
			}
		}
		if len(env.provider.calls()) != callsBefore {
			t.Fatalf("empty topic made provider calls: got %d, want %d", len(env.provider.calls()), callsBefore)
		}

		// 8. Invalid args and /judge reply-only check -> usage lines, zero provider calls.
		admin := env.judgeAdmin("JudgeAdmin")
		existingMsg := topicMsg(bob, 10, base, "reply target")
		for _, tc := range []struct {
			who     *gotgbot.User
			cmd     string
			target  *gotgbot.Message
			wantKey string
		}{
			{asker, "/unrollshort 0", nil, "brief_usage_short"},
			{asker, "/unrollshort 201", nil, "brief_usage_short"},
			{asker, "/unrollshort 30s", nil, "brief_usage_short"},
			{asker, "/unrollshort 30d", nil, "brief_usage_short"},
			{asker, "/unrollshort foo", nil, "brief_usage_short"},
			{asker, "/unrollshort 50", existingMsg, "brief_usage_short"},
			{asker, "/unrolllong 0", nil, "brief_usage_long"},
			{admin, "/judge 5", nil, "judge_usage"},
		} {
			groupBefore = len(env.sends())
			deliverBrief(topicCmd(tc.who, 10, tc.cmd, tc.target))
			if groupSends := env.sends(); len(groupSends) != groupBefore+1 || groupSends[len(groupSends)-1] != chatCanned(t, tc.wantKey) {
				t.Fatalf("%s answered %q, want %s", tc.cmd, groupSends, tc.wantKey)
			}
		}
		if len(env.provider.calls()) != callsBefore {
			t.Fatalf("invalid commands made provider calls: got %d, want %d", len(env.provider.calls()), callsBefore)
		}
	})
}
