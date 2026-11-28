package modules

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/redis/go-redis/v9"

	"github.com/uasneppy/Xenobot-72/xenobot/db/chatconv"
	"github.com/uasneppy/Xenobot-72/xenobot/db/disabling"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/db/presence"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/claude"
)

// TestChatWatcherAnswers holds the three ways a message is addressed: a mention (in a text and in
// a caption) and a reply to a message the conversation feature sent. Each gets one plain reply to
// the addressed message, the exchange is remembered and the next request carries it.
func TestChatWatcherAnswers(t *testing.T) {
	env := newChatEnv(t)
	alice, bob := env.member("Alice"), env.member("Bob")

	first := env.mention(alice, "what is tempe?")
	env.deliver(first)
	sends := env.sends()
	if len(sends) != 1 || sends[0] != chatFakeDefaultReply {
		t.Fatalf("sends = %q, want the provider's reply once", sends)
	}
	params := env.lastSend()
	if reply := chatReplyTo(t, params); reply.MessageId != first.MessageId || reply.AllowSendingWithoutReply {
		t.Fatalf("reply_parameters = %+v, want a reply to %d that is never sent without it", reply, first.MessageId)
	}
	if _, set := params["parse_mode"]; set {
		t.Fatalf("parse_mode = %v, want plain text", params["parse_mode"])
	}
	request := env.provider.calls()[0]
	if !strings.Contains(request.User, "Alice: what is tempe?") || strings.Contains(request.User, "@"+env.bot.Username) {
		t.Fatalf("user text %q, want the speaker's line without the bot's handle", request.User)
	}
	if !strings.Contains(request.Dynamic, "Facility 7") {
		t.Fatalf("dynamic text %q, want the group title", request.Dynamic)
	}

	// Bob replies to the bot's answer (message 9001 in the fake): an address, and the request now
	// carries the first exchange.
	answer := env.message(bob, "and soy?")
	answer.ReplyToMessage = &gotgbot.Message{MessageId: 9001, Date: 1, Chat: answer.Chat, From: &env.bot.User, Text: chatFakeDefaultReply}
	env.deliver(answer)
	if got := len(env.sends()); got != 2 {
		t.Fatalf("sends after the reply = %d, want 2", got)
	}
	second := env.provider.calls()[1].User
	if !strings.Contains(second, "Alice: what is tempe?") || !strings.Contains(second, "Xenobot-72: "+chatFakeDefaultReply) || !strings.Contains(second, "Bob: and soy?") {
		t.Fatalf("second request %q, want the first exchange and Bob's line", second)
	}

	// A mention in a caption.
	photo := env.message(env.member("Carol"), "")
	handle := "@" + env.bot.Username
	photo.Caption = handle + " look"
	photo.Photo = []gotgbot.PhotoSize{{FileId: "p", FileUniqueId: "pu", Width: 10, Height: 10}}
	photo.CaptionEntities = []gotgbot.MessageEntity{{Type: "mention", Offset: 0, Length: int64(len(handle))}}
	env.deliver(photo)
	if got := len(env.sends()); got != 3 {
		t.Fatalf("sends after a caption mention = %d, want 3", got)
	}
	if len(env.passed) != 3 {
		t.Fatalf("handlers after the watcher saw %d messages, want 3: the watcher ends no update", len(env.passed))
	}
}

// TestChatWatcherIgnores holds every message that must get no reply and, for the unaddressed ones,
// no I/O at all.
func TestChatWatcherIgnores(t *testing.T) {
	env := newChatEnv(t)
	member := env.member("Dana")
	handle := "@" + env.bot.Username
	mentionEntity := []gotgbot.MessageEntity{{Type: "mention", Offset: 0, Length: int64(len(handle))}}

	cases := []struct {
		name string
		msg  func() *gotgbot.Message
	}{
		{"unaddressed text", func() *gotgbot.Message { return env.message(member, "hello everyone") }},
		{"another bot's mention", func() *gotgbot.Message {
			msg := env.message(member, "@someotherbot hi")
			msg.Entities = []gotgbot.MessageEntity{{Type: "mention", Offset: 0, Length: 13}}
			return msg
		}},
		{"a command", func() *gotgbot.Message {
			msg := env.message(member, "/start "+handle)
			msg.Entities = []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: 6}}
			return msg
		}},
		{"a reply to a moderation notice", func() *gotgbot.Message {
			msg := env.message(member, "why?")
			msg.ReplyToMessage = &gotgbot.Message{MessageId: 7777, Date: 1, Chat: msg.Chat, From: &env.bot.User, Text: "Banned."}
			return msg
		}},
		{"a private chat", func() *gotgbot.Message {
			msg := env.mention(member, "hi")
			msg.Chat = gotgbot.Chat{Id: member.Id, Type: "private"}
			return msg
		}},
		{"a bot", func() *gotgbot.Message {
			bot := &gotgbot.User{Id: uniqueLinkOwnerID(), FirstName: "Other", IsBot: true}
			return env.mention(bot, "hi")
		}},
		{"an anonymous admin", func() *gotgbot.Message {
			msg := env.mention(&gotgbot.User{Id: 1087968824, FirstName: "Group", IsBot: true}, "hi")
			msg.SenderChat = &gotgbot.Chat{Id: env.chatID, Type: "supergroup"}
			return msg
		}},
		{"a channel", func() *gotgbot.Message {
			msg := env.mention(&gotgbot.User{Id: 136817688, FirstName: "Channel", IsBot: true}, "hi")
			msg.SenderChat = &gotgbot.Chat{Id: -1009999, Type: "channel"}
			return msg
		}},
	}
	for _, tc := range cases {
		msg := tc.msg()
		if tc.name == "a command" {
			msg.Entities = append(msg.Entities, gotgbot.MessageEntity{Type: "mention", Offset: 7, Length: int64(len(handle))})
		}
		before := env.callCount()
		env.deliver(msg)
		if sends := env.sends(); len(sends) != 0 {
			t.Fatalf("%s: sends = %q, want none", tc.name, sends)
		}
		if tc.name == "unaddressed text" {
			if after := env.callCount(); after != before {
				t.Fatalf("unaddressed text made %d Telegram calls, want none", after-before)
			}
			if keys := env.mr.Keys(); len(keys) != 0 {
				t.Fatalf("unaddressed text touched Redis: %v", keys)
			}
		}
	}
	if len(env.provider.calls()) != 0 {
		t.Fatalf("the provider was asked %d times, want never", len(env.provider.calls()))
	}

	// Conversations off: a mention gets nothing.
	if err := chatconv.SetEnabled(context.Background(), env.chatID, false, 1); err != nil {
		t.Fatalf("SetEnabled off: %v", err)
	}
	msg := env.message(member, handle+" hi")
	msg.Entities = mentionEntity
	env.deliver(msg)
	if sends := env.sends(); len(sends) != 0 {
		t.Fatalf("off: sends = %q, want none", sends)
	}
}

// TestChatWatcherAudience holds the "administrators and approved members" audience: a plain member
// gets nothing, an administrator and an approved member get a reply, and an approvals read that
// fails means no reply.
func TestChatWatcherAudience(t *testing.T) {
	env := newChatEnv(t)
	if err := chatconv.SetAudience(context.Background(), env.chatID, chatconv.AudienceAdminsApproved, 1); err != nil {
		t.Fatalf("SetAudience: %v", err)
	}
	plain, admin, approved, unknown := env.member("Plain"), env.member("Admin"), env.member("Approved"), env.member("Unknown")
	chatSwap(t, &chatAudienceAdmin, func(_ *gotgbot.Bot, _ int64, userID int64) bool { return userID == admin.Id })
	chatSwap(t, &chatAudienceApproved, func(_ context.Context, _ int64, userID int64) (bool, error) {
		if userID == unknown.Id {
			return false, errors.New("approvals down")
		}
		return userID == approved.Id, nil
	})

	for _, tc := range []struct {
		who  *gotgbot.User
		want int
	}{{plain, 0}, {admin, 1}, {approved, 2}, {unknown, 2}} {
		env.deliver(env.mention(tc.who, "status?"))
		if got := len(env.sends()); got != tc.want {
			t.Fatalf("%s: sends = %d, want %d", tc.who.FirstName, got, tc.want)
		}
	}
}

// TestChatGuards holds the guards of a job: the cooldown, each hourly budget with its once-an-hour
// line, the breaker with its once-in-5-minutes line, a provider failure, a refusal, a blank answer,
// no Redis and a provider that is not configured.
func TestChatGuards(t *testing.T) {
	t.Run("cooldown", func(t *testing.T) {
		env := newChatEnv(t)
		member := env.member("Eve")
		env.deliver(env.mention(member, "one"))
		env.deliver(env.mention(member, "two"))
		if got := len(env.sends()); got != 1 {
			t.Fatalf("sends = %d, want 1: the second message is inside the cooldown", got)
		}
	})

	t.Run("budgets", func(t *testing.T) {
		env := newChatEnv(t)
		chatSwap(t, &chatCooldown, time.Millisecond)
		chatSwap(t, &chatUserHourlyBudget, int64(1))
		chatSwap(t, &chatChatHourlyBudget, int64(2))
		alice, bob, carol := env.member("Alice"), env.member("Bob"), env.member("Carol")
		allowance := chatCanned(t, "chat_reply_allowance")

		env.deliver(env.mention(alice, "one"))
		env.mr.FastForward(time.Second)
		env.deliver(env.mention(alice, "two")) // over Alice's own allowance: the line, once
		env.mr.FastForward(time.Second)
		env.deliver(env.mention(alice, "three")) // over again: the line is gated
		env.deliver(env.mention(bob, "one"))
		env.deliver(env.mention(carol, "one")) // over the group's allowance: still gated
		want := []string{chatFakeDefaultReply, allowance, chatFakeDefaultReply}
		if got := env.sends(); fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("sends = %q, want %q", got, want)
		}
		if got := len(env.provider.calls()); got != 2 {
			t.Fatalf("provider calls = %d, want 2", got)
		}
	})

	t.Run("failures, breaker and refusal", func(t *testing.T) {
		env := newChatEnv(t)
		malfunction := chatCanned(t, "chat_reply_malfunction")
		failure := chatAnswerErr(&claude.HTTPError{Status: 500, Type: "api_error"})
		env.provider.script(failure, failure, failure, failure, failure)
		for i := range 6 {
			env.deliver(env.mention(env.member(fmt.Sprintf("M%d", i)), "hi"))
		}
		// Five failures pause the group; the sixth message is not sent to the provider. The
		// malfunction line is said once in five minutes.
		if got := len(env.provider.calls()); got != 5 {
			t.Fatalf("provider calls = %d, want 5", got)
		}
		if got := env.sends(); fmt.Sprint(got) != fmt.Sprint([]string{malfunction}) {
			t.Fatalf("sends = %q, want the malfunction line once", got)
		}

		env2 := newChatEnv(t)
		env2.provider.script(
			func(context.Context, chatRequest) (chatAnswer, error) { return chatAnswer{Outcome: chatRefused}, nil },
			chatAnswerText("   "),
		)
		env2.deliver(env2.mention(env2.member("R"), "something"))
		if got := env2.sends(); fmt.Sprint(got) != fmt.Sprint([]string{chatCanned(t, "chat_reply_refused")}) {
			t.Fatalf("refusal: sends = %q, want the refusal line", got)
		}
		if !chatIsOwnMessage(env2.rc, env2.chatID, 9001) {
			t.Fatalf("the refusal line is not remembered as the feature's own message")
		}
		env2.deliver(env2.mention(env2.member("B"), "something"))
		if got := env2.sends(); len(got) != 2 || got[1] != malfunction {
			t.Fatalf("blank answer: sends = %q, want the malfunction line", got)
		}
	})

	t.Run("a rejected key pauses the provider for every group", func(t *testing.T) {
		env := newChatEnv(t)
		env.provider.script(chatAnswerErr(&claude.HTTPError{Status: 401, Type: "authentication_error"}))
		env.deliver(env.mention(env.member("K"), "hi"))
		if !chatBreaker(chatconv.ProviderAnthropic).isPaused(uniqueModuleChatID(), raidNow()) {
			t.Fatalf("a rejected key did not pause the provider for another group")
		}
	})

	t.Run("no redis or no provider", func(t *testing.T) {
		env := newChatEnv(t)
		chatSwap(t, &chatRedisClient, func() *redis.Client { return nil })
		env.deliver(env.mention(env.member("N"), "hi"))
		withChatProviders(t, false, true, true)
		env.deliver(env.mention(env.member("O"), "hi"))
		if got := len(env.sends()); got != 0 {
			t.Fatalf("sends = %d, want none", got)
		}
	})
}

// TestChatFilterReply holds the output filter.
func TestChatFilterReply(t *testing.T) {
	long := strings.Repeat("Procedure complete. ", 80)
	cases := []struct {
		name, in, want string
	}{
		{"links", "Read https://example.com/x and www.example.org or t.me/joinchat now", "Read [link removed] and [link removed] or [link removed] now"},
		{"pings", "Ask @admin_bob or @x.", "Ask @\u200badmin_bob or @\u200bx."},
		{"blank", "   ", ""},
	}
	for _, tc := range cases {
		if got := chatFilterReply(tc.in); got != tc.want {
			t.Fatalf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
	got := chatFilterReply(long)
	if n := len([]rune(got)); n > chatReplyMaxRunes || !strings.HasSuffix(got, ".") {
		t.Fatalf("long reply cut to %d runes ending %q, want at most %d at a sentence end", n, got[len(got)-5:], chatReplyMaxRunes)
	}
}

// TestShutdownEndsChat holds the stop: a job inside a hung provider call is ended by StopChat
// within its wait, sends nothing and remembers nothing, and a later message starts no job.
func TestShutdownEndsChat(t *testing.T) {
	env := newChatEnv(t)
	chatSwap(t, &chatStopWait, 2*time.Second)
	started := make(chan struct{})
	env.provider.script(func(ctx context.Context, _ chatRequest) (chatAnswer, error) {
		close(started)
		<-ctx.Done()
		return chatAnswer{}, ctx.Err()
	})
	restoreChatPool(t)

	msg := env.mention(env.member("S"), "hi")
	env.nextUpdate++
	if err := env.dispatcher.ProcessUpdate(env.bot, &gotgbot.Update{UpdateId: env.nextUpdate, Message: msg}, nil); err != nil {
		t.Fatalf("ProcessUpdate: %v", err)
	}
	<-started
	if !StopChat() {
		t.Fatalf("StopChat timed out")
	}
	env.deliver(env.mention(env.member("T"), "late"))
	if sends := env.sends(); len(sends) != 0 {
		t.Fatalf("sends = %q, want none", sends)
	}
	if keys := env.mr.Keys(); strings.Contains(fmt.Sprint(keys), ":h") || strings.Contains(fmt.Sprint(keys), ":mine") {
		t.Fatalf("a cut-off job left memory: %v", keys)
	}
	if got := len(env.provider.calls()); got != 1 {
		t.Fatalf("provider calls = %d, want 1", got)
	}
}

// restoreChatPool gives the pool its draining flag and root context back after a test ran
// StopChat.
func restoreChatPool(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		chatJobs.wait()
		chatDraining.Store(false)
		chatPoolContext.mu.Lock()
		chatPoolContext.ctx, chatPoolContext.cancel = context.WithCancel(context.Background())
		chatPoolContext.mu.Unlock()
	})
}

// TestChatDeletedMessageGetsNoReply holds that a message the blacklist deleted gets no reply,
// though the blacklist passes the update on and the watcher sees it: the reply is never sent
// without its reply target. A clean mention in the same group is still answered.
func TestChatDeletedMessageGetsNoReply(t *testing.T) {
	env := newChatEnv(t)
	LoadBlacklists(env.dispatcher)
	settingsListCleanup(t, env.chatID)
	settingsListSeedBlacklistAction(t, env.chatID, "spamword", "none")

	bad := env.mention(env.member("Spammer"), "buy spamword now")
	env.deliver(bad)
	if deletes := env.client.callsFor("deleteMessage"); len(deletes) != 1 {
		t.Fatalf("deleteMessage calls = %d, want the blacklist's one", len(deletes))
	}
	if sends := env.sends(); len(sends) != 0 {
		t.Fatalf("sends = %q, want none for a deleted message", sends)
	}
	if got := chatShedCount.Load(); got == 0 {
		t.Fatalf("the dropped reply was not counted as shed")
	}

	env.deliver(env.mention(env.member("Resident"), "hello"))
	if sends := env.sends(); len(sends) != 1 {
		t.Fatalf("sends = %q, want one reply to the clean mention", sends)
	}
}

// TestChatAskCommand holds /ask and /xeno: a question is answered like a mention; with no text the
// usage is shown; in a private chat and in a group with conversations off a hint is the answer;
// disabling /ask silences both names for a member.
func TestChatAskCommand(t *testing.T) {
	env := newChatEnv(t)
	member := env.member("Gale")
	command := func(from *gotgbot.User, text string, chat gotgbot.Chat) *gotgbot.Message {
		msg := env.message(from, text)
		msg.Chat = chat
		word := strings.Fields(text)[0]
		msg.Entities = []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: int64(len(word))}}
		return msg
	}
	group := gotgbot.Chat{Id: env.chatID, Type: "supergroup", Title: "Facility 7"}

	env.deliver(command(member, "/ask what is a ration?\nexactly", group))
	if sends := env.sends(); len(sends) != 1 || sends[0] != chatFakeDefaultReply {
		t.Fatalf("/ask sends = %q, want the provider's reply", sends)
	}
	if user := env.provider.calls()[0].User; !strings.Contains(user, "Gale: what is a ration? exactly") {
		t.Fatalf("/ask user text %q, want the question after the command", user)
	}

	env.deliver(command(env.member("Hal"), "/xeno status", group))
	env.deliver(command(env.member("Ivy"), "/ask", group))
	sends := env.sends()
	if len(sends) != 3 || sends[1] != chatFakeDefaultReply || !strings.Contains(sends[2], "/ask") {
		t.Fatalf("sends = %q, want the /xeno reply and the usage", sends)
	}

	private := command(member, "/ask hello", gotgbot.Chat{Id: member.Id, Type: "private"})
	env.deliver(private)
	if got := env.client.callsFor("sendMessage"); fmt.Sprint(got[len(got)-1].Params["chat_id"]) != fmt.Sprint(member.Id) {
		t.Fatalf("/ask in private got no answer in the private chat")
	}

	if err := disabling.DisableCMD(env.chatID, "ask"); err != nil {
		t.Fatalf("DisableCMD(ask): %v", err)
	}
	t.Cleanup(func() { _ = disabling.EnableCMD(env.chatID, "ask") })
	env.deliver(command(env.member("Kit"), "/xeno status", group))
	env.deliver(command(env.member("Lee"), "/ask status", group))
	if got := len(env.sends()); got != 3 {
		t.Fatalf("with /ask disabled sends = %d, want still 3", got)
	}
	_ = disabling.EnableCMD(env.chatID, "ask")

	if err := chatconv.SetEnabled(context.Background(), env.chatID, false, 1); err != nil {
		t.Fatalf("SetEnabled off: %v", err)
	}
	env.deliver(command(env.member("Jo"), "/ask hello", group))
	sends = env.sends()
	if len(sends) != 4 || !strings.Contains(sends[3], "/settings") {
		t.Fatalf("off: sends = %q, want the off hint", sends)
	}
	if got := len(env.provider.calls()); got != 2 {
		t.Fatalf("provider calls = %d, want 2", got)
	}
}

func chatTopicMsg(env *chatEnv, from *gotgbot.User, threadID int64, text string) *gotgbot.Message {
	msg := env.message(from, text)
	if threadID > 0 {
		msg.IsTopicMessage = true
		msg.MessageThreadId = threadID
	}
	return msg
}

func chatTopicMention(env *chatEnv, from *gotgbot.User, threadID int64, text string) *gotgbot.Message {
	msg := env.mention(from, text)
	if threadID > 0 {
		msg.IsTopicMessage = true
		msg.MessageThreadId = threadID
	}
	return msg
}

func chatAmbientSection(userPrompt string) string {
	const header = "[Recent room chatter in this topic, oldest first]\n"
	idx := strings.Index(userPrompt, header)
	if idx < 0 {
		return ""
	}
	rest := userPrompt[idx+len(header):]
	if end := strings.Index(rest, "\n\n["); end >= 0 {
		return rest[:end]
	}
	return rest
}

// TestChatAmbientContext holds CHAT-02: Owner×Admin double-gating, Staff Group exclusion, strict
// forum-topic isolation, chatAmbientContextLimit cap, and deduplication against chatReadHistory,
// quotedMessageID, and triggering messageID.
func TestChatAmbientContext(t *testing.T) {
	t.Run("double-gating and staff group", func(t *testing.T) {
		env := newChatEnv(t)
		chatSwap(t, &chatCooldown, time.Millisecond)
		conv := chatconv.Settings{Enabled: true, LogEnabled: true, Provider: chatconv.ProviderAnthropic, Audience: chatconv.AudienceEveryone}
		pres := presence.Settings{AmbientContext: false}
		var staffRow *models.StaffGroup
		chatSwap(t, &chatSettingsLookup, func(context.Context, int64) (chatconv.Settings, error) { return conv, nil })
		chatSwap(t, &presenceSettingsGet, func(context.Context, int64) (presence.Settings, error) { return pres, nil })
		chatSwap(t, &presenceStaffLookup, func(int64) *models.StaffGroup { return staffRow })

		alice, bob := env.member("Alice"), env.member("Bob")
		// Seed a Topic A log entry via group 14 and directly.
		env.deliver(chatTopicMsg(env, alice, 10, "topic-a chatter before"))

		// 1. AmbientContext=false -> omitted.
		env.deliver(chatTopicMention(env, bob, 10, "first ping"))
		if sec := chatAmbientSection(env.provider.calls()[0].User); sec != "" {
			t.Fatalf("AmbientContext=false leaked ambient section: %q", sec)
		}

		// 2. LogEnabled=false with AmbientContext=true -> omitted even if log entries exist in Redis.
		conv.LogEnabled = false
		pres.AmbientContext = true
		env.mr.FastForward(time.Second)
		env.deliver(chatTopicMention(env, bob, 10, "second ping"))
		if sec := chatAmbientSection(env.provider.calls()[1].User); sec != "" {
			t.Fatalf("LogEnabled=false leaked ambient section: %q", sec)
		}

		// 3. Staff Group with both enabled -> omitted.
		conv.LogEnabled = true
		staffRow = &models.StaffGroup{ChatID: env.chatID}
		env.mr.FastForward(time.Second)
		env.deliver(chatTopicMention(env, bob, 10, "third ping"))
		if sec := chatAmbientSection(env.provider.calls()[2].User); sec != "" {
			t.Fatalf("Staff Group leaked ambient section: %q", sec)
		}
	})

	t.Run("topic isolation, deduplication, and cap", func(t *testing.T) {
		env := newChatEnv(t)
		chatSwap(t, &chatCooldown, time.Millisecond)
		conv := chatconv.Settings{Enabled: true, LogEnabled: true, Provider: chatconv.ProviderAnthropic, Audience: chatconv.AudienceEveryone}
		pres := presence.Settings{AmbientContext: true}
		chatSwap(t, &chatSettingsLookup, func(context.Context, int64) (chatconv.Settings, error) { return conv, nil })
		chatSwap(t, &presenceSettingsGet, func(context.Context, int64) (presence.Settings, error) { return pres, nil })
		chatSwap(t, &presenceStaffLookup, func(int64) *models.StaffGroup { return nil })

		alice, bob, carol := env.member("Alice"), env.member("Bob"), env.member("Carol")

		// Prior @mention exchange in Topic A (stored in both chatReadHistory and chat_log).
		prior := chatTopicMention(env, alice, 10, "prior mention in topic A")
		env.deliver(prior)

		// Deliver 10 messages across Topic A (Thread: 10), Topic B (Thread: 20), and General (Thread: 0).
		for i := 1; i <= 5; i++ {
			env.deliver(chatTopicMsg(env, bob, 10, fmt.Sprintf("topic-a line %d", i)))
		}
		for i := 1; i <= 3; i++ {
			env.deliver(chatTopicMsg(env, carol, 20, fmt.Sprintf("topic-b secret %d", i)))
		}
		for i := 1; i <= 2; i++ {
			env.deliver(chatTopicMsg(env, carol, 0, fmt.Sprintf("general secret %d", i)))
		}
		quoted := chatTopicMsg(env, bob, 10, "topic-a quoted line")
		env.deliver(quoted)

		// Triggering message in Topic A replying to quoted.
		env.mr.FastForward(time.Second)
		trigger := chatTopicMention(env, alice, 10, "triggering question")
		trigger.ReplyToMessage = quoted
		env.deliver(trigger)

		calls := env.provider.calls()
		if len(calls) != 2 {
			t.Fatalf("provider calls = %d, want 2", len(calls))
		}
		userPrompt := calls[1].User
		amb := chatAmbientSection(userPrompt)
		if amb == "" {
			t.Fatalf("expected [Recent room chatter in this topic, oldest first] in %q", userPrompt)
		}
		for i := 1; i <= 5; i++ {
			want := fmt.Sprintf("Bob: topic-a line %d", i)
			if !strings.Contains(amb, want) {
				t.Fatalf("ambient %q missing %q", amb, want)
			}
		}
		// Topic B and General must not appear anywhere in the prompt.
		for _, forbidden := range []string{"topic-b secret", "general secret"} {
			if strings.Contains(userPrompt, forbidden) {
				t.Fatalf("prompt leaked other thread %q: %q", forbidden, userPrompt)
			}
		}
		// Deduplication: prior mention, quoted message, and trigger must NOT appear in ambient section.
		for _, dup := range []string{"prior mention in topic A", "topic-a quoted line", "triggering question"} {
			if strings.Contains(amb, dup) {
				t.Fatalf("ambient section duplicated %q: %q", dup, amb)
			}
		}
		if !strings.Contains(userPrompt, "Alice: prior mention in topic A") {
			t.Fatalf("expected prior mention in conversation history: %q", userPrompt)
		}
		if !strings.Contains(userPrompt, "Bob: topic-a quoted line") {
			t.Fatalf("expected quoted message in reply context: %q", userPrompt)
		}

		// Exceed chatAmbientContextLimit with more Topic A lines than the window holds.
		lastLine := chatAmbientContextLimit + 4
		for i := 6; i <= lastLine; i++ {
			env.deliver(chatTopicMsg(env, bob, 10, fmt.Sprintf("topic-a line %d", i)))
		}
		env.mr.FastForward(time.Second)
		env.deliver(chatTopicMention(env, alice, 10, "capped check"))
		ambCapped := chatAmbientSection(env.provider.calls()[2].User)
		lines := strings.Split(strings.TrimSpace(ambCapped), "\n")
		if len(lines) > chatAmbientContextLimit {
			t.Fatalf("ambient lines = %d, want <= %d: %q", len(lines), chatAmbientContextLimit, ambCapped)
		}
		// The oldest lines are outside the newest-limit window; the newest remains.
		if strings.Contains(ambCapped, "topic-a line 1\n") || !strings.Contains(ambCapped, fmt.Sprintf("topic-a line %d", lastLine)) {
			t.Fatalf("ambient window unexpected: %q", ambCapped)
		}
	})
}

// TestChatVocativeAddressing holds CHAT-03: opt-in leading name addressing across EN/UK,
// forward/audience/staff-group guards, and strict mutual exclusion with chatReactWatch.
func TestChatVocativeAddressing(t *testing.T) {
	env := newChatEnv(t)
	chatSwap(t, &chatCooldown, time.Millisecond)
	conv := chatconv.Settings{Enabled: true, Provider: chatconv.ProviderAnthropic, Audience: chatconv.AudienceEveryone}
	pres := presence.Settings{VocativeEnabled: false, AIReactMode: presence.AIReactModeMedium}
	var staffRow *models.StaffGroup
	chatSwap(t, &chatSettingsLookup, func(context.Context, int64) (chatconv.Settings, error) { return conv, nil })
	chatSwap(t, &presenceSettingsGet, func(context.Context, int64) (presence.Settings, error) { return pres, nil })
	chatSwap(t, &presenceStaffLookup, func(int64) *models.StaffGroup { return staffRow })
	chatSwap(t, &chatReactProfiles, map[string]chatReactProfile{
		presence.AIReactModeMedium: {minGap: 1, chatCooldown: 0, userCooldown: 0, chancePct: 100},
	})
	chatSwap(t, &chatReactRoll, func(chancePct int) bool { return chancePct > 0 })
	var reactJobs atomic.Int32
	chatSwap(t, &chatReactProcess, func(chatReactJob) { reactJobs.Add(1) })

	alice, bob, plain := env.member("Alice"), env.member("Bob"), env.member("Plain")

	// 1. VocativeEnabled=false: no Conversations reply, falls through to chatReactWatch.
	env.deliver(env.message(alice, "Xenobot, what's up?"))
	if got := len(env.sends()); got != 0 {
		t.Fatalf("VocativeEnabled=false sends = %d, want 0", got)
	}
	if got := reactJobs.Load(); got != 1 {
		t.Fatalf("VocativeEnabled=false reactJobs = %d, want 1", got)
	}

	// 2. VocativeEnabled=true: EN and UK vocatives reply with stripped prefix and zero AI reactions.
	pres.VocativeEnabled = true
	env.mr.FastForward(time.Second)
	env.deliver(env.message(alice, "Xenobot, check the reactor"))
	env.mr.FastForward(time.Second)
	env.deliver(env.message(bob, "Кзенобот, перевір реактор"))

	if got := len(env.sends()); got != 2 {
		t.Fatalf("vocative sends = %d, want 2", got)
	}
	if got := reactJobs.Load(); got != 1 {
		t.Fatalf("vocative triggered AI reactions: reactJobs = %d, want still 1", got)
	}
	calls := env.provider.calls()
	if !strings.Contains(calls[0].User, "Alice: check the reactor") || strings.Contains(calls[0].User, "Xenobot,") {
		t.Fatalf("EN vocative prompt = %q, want stripped prefix", calls[0].User)
	}
	if !strings.Contains(calls[1].User, "Bob: перевір реактор") || strings.Contains(calls[1].User, "Кзенобот,") {
		t.Fatalf("UK vocative prompt = %q, want stripped prefix", calls[1].User)
	}

	// 3. Forwarded message starting with "Xenobot, hi" is NOT treated as vocative (falls through to chatReactWatch).
	fwd := env.message(alice, "Xenobot, hi")
	fwd.ForwardOrigin = gotgbot.MessageOriginUser{Date: 1, SenderUser: *bob}
	env.mr.FastForward(time.Second)
	env.deliver(fwd)
	if got := len(env.sends()); got != 2 {
		t.Fatalf("forwarded sends = %d, want still 2", got)
	}
	if got := reactJobs.Load(); got != 2 {
		t.Fatalf("forwarded reactJobs = %d, want 2", got)
	}

	// 4. AudienceAdminsApproved: non-approved member is silently ignored and does NOT trigger AI reaction.
	conv.Audience = chatconv.AudienceAdminsApproved
	chatSwap(t, &chatAudienceAdmin, func(*gotgbot.Bot, int64, int64) bool { return false })
	chatSwap(t, &chatAudienceApproved, func(context.Context, int64, int64) (bool, error) { return false, nil })
	env.mr.FastForward(time.Second)
	env.deliver(env.message(plain, "Xenobot, hi"))
	if got := len(env.sends()); got != 2 {
		t.Fatalf("audience-rejected sends = %d, want still 2", got)
	}
	if got := reactJobs.Load(); got != 2 {
		t.Fatalf("audience-rejected reactJobs = %d, want still 2", got)
	}

	// 5. Staff Group: zero replies and zero AI reactions.
	conv.Audience = chatconv.AudienceEveryone
	staffRow = &models.StaffGroup{ChatID: env.chatID}
	env.mr.FastForward(time.Second)
	env.deliver(env.message(alice, "Xenobot, hi"))
	if got := len(env.sends()); got != 2 {
		t.Fatalf("staff group sends = %d, want still 2", got)
	}
	if got := reactJobs.Load(); got != 2 {
		t.Fatalf("staff group reactJobs = %d, want still 2", got)
	}
}
