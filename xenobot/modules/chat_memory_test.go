//go:build testtools

package modules

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/db/chatconv"
	"github.com/uasneppy/Xenobot-72/xenobot/db/membermem"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/chatprompt"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/featherless"
)

const chatMemoryTalk = "I just adopted two cats and I love hiking on weekends"

// newChatMemoryEnv is a conversation group with the owner's memories switch on, the writer's cooldown and
// the conversation cooldown short, and the group's notes cleaned up at the end.
func newChatMemoryEnv(t *testing.T) *chatEnv {
	t.Helper()
	env := newChatEnv(t)
	chatSwap(t, &chatCooldown, time.Millisecond)
	if err := chatconv.SetMemoriesEnabled(context.Background(), env.chatID, true, 1); err != nil {
		t.Fatalf("SetMemoriesEnabled: %v", err)
	}
	t.Cleanup(func() { _ = membermem.DeleteChat(context.Background(), env.chatID) })
	return env
}

func chatMemoryNotes(t *testing.T, chatID, userID int64) []string {
	t.Helper()
	notes, err := membermem.Get(context.Background(), chatID, userID)
	if err != nil {
		t.Fatalf("membermem.Get: %v", err)
	}
	return notes
}

func chatMemorySeed(t *testing.T, chatID, userID int64, notes ...string) {
	t.Helper()
	if _, err := membermem.Replace(context.Background(), chatID, userID, notes); err != nil {
		t.Fatalf("seed notes: %v", err)
	}
}

// TestChatMemoryWriterAndPrompt: after a reply to a member's own message the writer lane stores that
// member's notes, and the next conversation turn for that member (and only that member, in this group
// only) carries them.
func TestChatMemoryWriterAndPrompt(t *testing.T) {
	env := newChatMemoryEnv(t)
	alice, bob := env.member("Alice"), env.member("Bob")
	env.provider.script(
		chatAnswerText("Noted. Acknowledged."),
		chatAnswerText(`{"notes":["Has two cats","Enjoys hiking"]}`),
	)
	env.deliver(env.mention(alice, chatMemoryTalk))

	calls := env.provider.calls()
	if len(calls) != 2 {
		t.Fatalf("provider calls = %d, want the reply and the writer", len(calls))
	}
	writer := calls[1]
	if writer.Static != chatprompt.MemoryStatic() || writer.MaxTokens != chatprompt.MemoryMaxTokens {
		t.Errorf("the writer's request is not the code-owned one: max_tokens=%d", writer.MaxTokens)
	}
	for _, want := range []string{chatMemoryTalk, "Noted. Acknowledged.", "(none)"} {
		if !strings.Contains(writer.User, want) {
			t.Errorf("the writer's user text lacks %q:\n%s", want, writer.User)
		}
	}
	for _, forbidden := range []string{"Alice", "Facility 7", "@" + env.bot.Username} {
		if strings.Contains(writer.User+writer.Dynamic+writer.Static, forbidden) {
			t.Errorf("the writer's request names %q", forbidden)
		}
	}
	if got := chatMemoryNotes(t, env.chatID, alice.Id); strings.Join(got, "|") != "Has two cats|Enjoys hiking" {
		t.Fatalf("stored notes = %q", got)
	}
	if chatMemoryWrittenCount.Load() == 0 {
		t.Error("the written counter did not move")
	}

	// Alice's next turn carries her notes; the writer's cooldown keeps it from running again.
	env.mr.FastForward(time.Second)
	env.provider.script(chatAnswerText("Second answer."))
	env.deliver(env.mention(alice, "what should I feed them tonight?"))
	calls = env.provider.calls()
	if len(calls) != 3 {
		t.Fatalf("provider calls = %d, want one more (the reply only, the writer is cooling down)", len(calls))
	}
	if !strings.Contains(calls[2].User, "- Has two cats") || !strings.Contains(calls[2].User, "[What this speaker told you") {
		t.Errorf("Alice's turn lacks her notes:\n%s", calls[2].User)
	}
	if strings.Contains(calls[2].Static+calls[2].Dynamic, "Has two cats") {
		t.Error("the notes reached the cached system text")
	}

	// Bob, in the same group, gets none of Alice's notes, and his own writer run stores his.
	env.mr.FastForward(time.Second)
	env.provider.script(chatAnswerText("Bob answer."), chatAnswerText(`{"notes":["Plays chess every evening"]}`))
	env.deliver(env.mention(bob, "I play chess every evening with my brother"))
	calls = env.provider.calls()
	if len(calls) != 5 || strings.Contains(calls[3].User, "Has two cats") {
		t.Fatalf("Bob's turn: %d calls, or it carried Alice's notes:\n%s", len(calls), calls[3].User)
	}
	if got := chatMemoryNotes(t, env.chatID, bob.Id); strings.Join(got, "|") != "Plays chess every evening" {
		t.Errorf("Bob's notes = %q", got)
	}
	if got := chatMemoryNotes(t, env.chatID, alice.Id); len(got) != 2 {
		t.Errorf("Bob's writer touched Alice's notes: %q", got)
	}

	// After the writer's cooldown the next run sees the stored notes and replaces the whole list.
	env.mr.FastForward(chatMemoryCooldown + time.Minute)
	env.provider.script(chatAnswerText("Third answer."), chatAnswerText(`{"notes":["Has three cats","Enjoys hiking"]}`))
	env.deliver(env.mention(alice, "update: I adopted a third cat and I still love hiking"))
	calls = env.provider.calls()
	if len(calls) != 7 || !strings.Contains(calls[6].User, "- Has two cats\n- Enjoys hiking") {
		t.Fatalf("the writer did not see the stored notes (%d calls):\n%s", len(calls), calls[len(calls)-1].User)
	}
	if got := chatMemoryNotes(t, env.chatID, alice.Id); strings.Join(got, "|") != "Has three cats|Enjoys hiking" {
		t.Errorf("a replace is not the whole list: %q", got)
	}
}

// TestChatMemoryForgetDuringWriter: a /forgetme (or any other change to the member's notes) that lands
// while the writer is waiting for the provider is not undone by the writer's own write.
func TestChatMemoryForgetDuringWriter(t *testing.T) {
	env := newChatMemoryEnv(t)
	alice := env.member("Alice")
	chatMemorySeed(t, env.chatID, alice.Id, "Has two cats")

	env.provider.script(
		chatAnswerText("Noted. Acknowledged."),
		func(ctx context.Context, _ chatRequest) (chatAnswer, error) {
			if _, err := membermem.Delete(ctx, env.chatID, alice.Id); err != nil {
				t.Errorf("simulated /forgetme: %v", err)
			}
			return chatAnswer{Text: `{"notes":["Has two cats","Enjoys hiking"]}`}, nil
		},
	)
	shed := chatMemoryShedCount.Load()
	env.deliver(env.mention(alice, chatMemoryTalk))
	if got := chatMemoryNotes(t, env.chatID, alice.Id); len(got) != 0 {
		t.Fatalf("the writer brought back notes the member had forgotten: %q", got)
	}
	if chatMemoryShedCount.Load() == shed {
		t.Error("the discarded write was not counted as shed")
	}

	// The same guard covers a change made by someone else while the provider thought.
	env.mr.FastForward(chatMemoryCooldown + time.Minute)
	chatMemorySeed(t, env.chatID, alice.Id, "Has two cats")
	env.provider.script(
		chatAnswerText("Second answer."),
		func(ctx context.Context, _ chatRequest) (chatAnswer, error) {
			if _, err := membermem.Replace(ctx, env.chatID, alice.Id, []string{"Moved to Lviv"}); err != nil {
				t.Errorf("concurrent write: %v", err)
			}
			return chatAnswer{Text: `{"notes":["Has two cats","Enjoys hiking"]}`}, nil
		},
	)
	env.deliver(env.mention(alice, "I have two cats and I enjoy hiking every weekend"))
	if got := chatMemoryNotes(t, env.chatID, alice.Id); strings.Join(got, "|") != "Moved to Lviv" {
		t.Errorf("the writer overwrote a list that changed under it: %q", got)
	}
}

// TestChatMemoryNotOffered: no writer call and no notes in the prompt when the owner's switch is off,
// the message is too short to hold a fact, the group is a Staff Group, the provider's breaker is paused,
// the budgets are spent or the member's writer cooldown is running.
func TestChatMemoryNotOffered(t *testing.T) {
	type tc struct {
		name  string
		setup func(t *testing.T, env *chatEnv, userID int64)
		text  string
	}
	cases := []tc{
		{"switch off", func(t *testing.T, env *chatEnv, _ int64) {
			if err := chatconv.SetMemoriesEnabled(context.Background(), env.chatID, false, 1); err != nil {
				t.Fatal(err)
			}
		}, chatMemoryTalk},
		{"message too short", func(*testing.T, *chatEnv, int64) {}, "I have two cats"},
		{"staff group", func(t *testing.T, env *chatEnv, _ int64) {
			chatSwap(t, &presenceStaffLookup, func(int64) *models.StaffGroup { return &models.StaffGroup{ChatID: env.chatID} })
		}, chatMemoryTalk},
		{"budget spent", func(t *testing.T, _ *chatEnv, _ int64) { chatSwap(t, &chatMemoryUserHourlyBudget, 0) }, chatMemoryTalk},
		{"cooldown running", func(t *testing.T, env *chatEnv, userID int64) {
			if err := env.rc.Set(context.Background(), chatMemoryCooldownKey(env.chatID, userID), "1", time.Hour).Err(); err != nil {
				t.Fatal(err)
			}
		}, chatMemoryTalk},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env := newChatMemoryEnv(t)
			alice := env.member("Alice")
			chatMemorySeed(t, env.chatID, alice.Id, "Has a dog")
			c.setup(t, env, alice.Id)
			env.provider.script(chatAnswerText("Acknowledged."))
			env.deliver(env.mention(alice, c.text))
			calls := env.provider.calls()
			if len(calls) != 1 {
				t.Fatalf("provider calls = %d, want only the reply", len(calls))
			}
			hasNotes := strings.Contains(calls[0].User, "Has a dog")
			if c.name == "switch off" || c.name == "staff group" {
				if hasNotes {
					t.Errorf("notes reached a prompt while the feature is off here:\n%s", calls[0].User)
				}
			} else if !hasNotes {
				t.Errorf("the speaker's stored notes are missing from the prompt:\n%s", calls[0].User)
			}
			if got := chatMemoryNotes(t, env.chatID, alice.Id); strings.Join(got, "|") != "Has a dog" {
				t.Errorf("notes changed to %q", got)
			}
		})
	}

	t.Run("breaker paused", func(t *testing.T) {
		env := newChatMemoryEnv(t)
		alice := env.member("Alice")
		env.provider.script(chatAnswerText("Acknowledged."))
		// The reply goes out, then the provider's breaker pauses the group before the writer runs.
		chatSwap(t, &chatMemoryProcess, func(task chatMemoryTask) {
			chatBreaker(task.provider).pauseProvider(raidNow(), time.Hour)
			chatMemoryProcessTask(task)
		})
		env.deliver(env.mention(alice, chatMemoryTalk))
		if got := len(env.provider.calls()); got != 1 {
			t.Fatalf("provider calls = %d, want only the reply (a paused breaker shed the writer)", got)
		}
	})
}

// TestChatMemoryFailuresChangeNothing: a provider error, an empty answer, a refusal, an answer that is not
// a notes object, and a switch turned off while the provider was thinking leave the stored notes as they
// were, and none of them is a failure of the group's conversations.
func TestChatMemoryFailuresChangeNothing(t *testing.T) {
	env := newChatMemoryEnv(t)
	chatSwap(t, &chatMemoryCooldown, time.Millisecond)
	chatSwap(t, &chatMemoryUserHourlyBudget, 100)
	chatSwap(t, &chatMemoryChatHourlyBudget, 100)
	alice := env.member("Alice")
	chatMemorySeed(t, env.chatID, alice.Id, "Has a dog")

	failures := []func(context.Context, chatRequest) (chatAnswer, error){
		chatAnswerErr(errors.New("provider exploded")),
		chatAnswerText("not json at all"),
		chatAnswerText(`{"notes":"Has a cat"}`),
		chatAnswerText(`{"notes":["a","b","c","d","e","f","g","h","i","j","k","l","m"]}`),
		func(context.Context, chatRequest) (chatAnswer, error) { return chatAnswer{Outcome: chatRefused}, nil },
		chatAnswerErr(featherless.ErrEmptyAnswer),
		chatAnswerText(`{"notes":["Has a cat"]}`), // valid, but the owner switched memories off meanwhile
	}
	failedBefore := chatMemoryFailedCount.Load()
	for i, step := range failures {
		env.mr.FastForward(time.Second)
		if i == len(failures)-1 {
			chatSwap(t, &chatMemorySettingsFresh, func(context.Context, int64) (chatconv.Settings, error) {
				return chatconv.Settings{Enabled: true, MemoriesEnabled: false, Provider: chatconv.ProviderAnthropic}, nil
			})
		}
		env.provider.script(chatAnswerText("Acknowledged."), step)
		env.deliver(env.mention(alice, chatMemoryTalk))
		if got := chatMemoryNotes(t, env.chatID, alice.Id); strings.Join(got, "|") != "Has a dog" {
			t.Fatalf("failure %d changed the notes to %q", i, got)
		}
	}
	if chatMemoryFailedCount.Load() == failedBefore {
		t.Error("the failed counter did not move")
	}
	if chatBreaker(chatconv.ProviderAnthropic).isPaused(env.chatID, raidNow()) {
		t.Error("memory failures paused the group's conversations")
	}
	// The group's conversations still answer.
	env.mr.FastForward(time.Second)
	chatSwap(t, &chatMemoryProcess, func(chatMemoryTask) {})
	env.provider.script(chatAnswerText("Still here."))
	sent := len(env.sends())
	env.deliver(env.mention(alice, "are you still there, after all that?"))
	if len(env.sends()) != sent+1 {
		t.Error("the conversation stopped answering after memory failures")
	}
}

// TestChatMemoryAcceptNotes: a note that carries a link, an @handle, angle brackets or a long number is
// dropped, never edited.
func TestChatMemoryAcceptNotes(t *testing.T) {
	in := []string{
		"Likes hiking",
		"Blog is https://example.com/me",
		"Reachable as @someone",
		"Phone is +380 50 123 45 67",
		"Card ends 1234567890",
		"Uses <b>bold</b> a lot",
		"Has 3 cats and 2 dogs",
		"likes hiking",
	}
	got := chatMemoryAcceptNotes(in)
	if strings.Join(got, "|") != "Likes hiking|Has 3 cats and 2 dogs" {
		t.Errorf("accepted %q", got)
	}
}

// TestChatMemoryLogsCarryIDsOnly: no line of the writer path holds the member's text, the bot's reply, a
// note or the provider's words, whether the update worked or failed.
func TestChatMemoryLogsCarryIDsOnly(t *testing.T) {
	env := newChatMemoryEnv(t)
	chatSwap(t, &chatMemoryCooldown, time.Millisecond)
	chatSwap(t, &chatMemoryUserHourlyBudget, 100)
	chatSwap(t, &chatMemoryChatHourlyBudget, 100)
	alice := env.member("Zeppelinfan")
	var lines []string
	hook := &chatMemoryLogHook{lines: &lines}
	logrus.AddHook(hook)
	prev := logrus.GetLevel()
	logrus.SetLevel(logrus.DebugLevel)
	t.Cleanup(func() { logrus.SetLevel(prev); hook.off.Store(true) })

	talk := "my secret hobby is collecting zeppelin stamps " + "sentinelmessage"
	env.provider.script(chatAnswerText("reply sentinelreply"), chatAnswerText(`{"notes":["Collects zeppelin stamps sentinelnote"]}`))
	env.deliver(env.mention(alice, talk))
	env.mr.FastForward(time.Second)
	env.provider.script(chatAnswerText("reply two sentinelreply"), chatAnswerErr(errors.New("boom sentinelerror")))
	env.deliver(env.mention(alice, talk+" again"))
	env.mr.FastForward(time.Second)
	env.provider.script(chatAnswerText("reply three sentinelreply"), chatAnswerText("garbage sentinelgarbage"))
	env.deliver(env.mention(alice, talk+" and again"))

	hook.off.Store(true)
	for _, line := range lines {
		for _, sentinel := range []string{"sentinelmessage", "sentinelreply", "sentinelnote", "sentinelerror", "sentinelgarbage", "zeppelin", "Zeppelinfan"} {
			if strings.Contains(line, sentinel) {
				t.Errorf("a log line holds %q: %s", sentinel, line)
			}
		}
	}
}

type chatMemoryLogHook struct {
	lines *[]string
	off   atomic.Bool
}

func (h *chatMemoryLogHook) Levels() []logrus.Level { return logrus.AllLevels }
func (h *chatMemoryLogHook) Fire(e *logrus.Entry) error {
	if h.off.Load() {
		return nil
	}
	s, _ := e.String()
	*h.lines = append(*h.lines, s)
	return nil
}

// TestShutdownEndsChatMemory: a writer job inside a hung provider call is ended by StopChat's cancel,
// writes nothing, and a later offer is shed.
func TestShutdownEndsChatMemory(t *testing.T) {
	env := newChatMemoryEnv(t)
	chatSwap(t, &chatStopWait, 2*time.Second)
	alice := env.member("Alice")
	started := make(chan struct{})
	env.provider.script(chatAnswerText("Acknowledged."), func(ctx context.Context, _ chatRequest) (chatAnswer, error) {
		close(started)
		<-ctx.Done()
		return chatAnswer{}, ctx.Err()
	})
	restoreChatPool(t)
	env.nextUpdate++
	if err := env.dispatcher.ProcessUpdate(env.bot, &gotgbot.Update{UpdateId: env.nextUpdate, Message: env.mention(alice, chatMemoryTalk)}, nil); err != nil {
		t.Fatalf("ProcessUpdate: %v", err)
	}
	<-started
	if !StopChat() {
		t.Fatal("StopChat timed out")
	}
	if got := chatMemoryNotes(t, env.chatID, alice.Id); len(got) != 0 {
		t.Errorf("a cut-off writer stored %q", got)
	}
	shed := chatMemoryShedCount.Load()
	if chatMemoryEnqueue(chatMemoryTask{chatID: env.chatID, userID: alice.Id, provider: chatconv.ProviderAnthropic}) {
		t.Error("a writer job was queued while draining")
	}
	if chatMemoryShedCount.Load() != shed+1 {
		t.Error("the late offer was not counted as shed")
	}
}

// TestChatMemoryGuards: the writer's file makes no Telegram call, and only the conversation request
// builder puts a member's notes in a prompt (never a brief, a verdict, a reaction or a toy).
func TestChatMemoryGuards(t *testing.T) {
	src, err := os.ReadFile("chat_memory.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"SendMessage", ".Reply(", "ReplyParameters", "*gotgbot.Bot", "chatSendReply", "chatPaced", "chatSayGated", "chatRecordFailure("} {
		if strings.Contains(string(src), forbidden) {
			t.Errorf("chat_memory.go names %s: the writer must make no Telegram call and never count a provider failure", forbidden)
		}
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "chat_prompt.go" {
			continue
		}
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, banned := range []string{"UserWithMemory(", "MemorySection(", "memoryNotes"} {
			if strings.Contains(string(body), banned) && name != "chat_run.go" && name != "chat_memory.go" {
				t.Errorf("%s uses %s: only the conversation prompt may carry a member's notes", name, banned)
			}
		}
	}
}

// TestForgetMeCommand: /forgetme deletes the sender's own notes of this group, says whether there were any,
// and touches no other member and no other group.
func TestForgetMeCommand(t *testing.T) {
	env := newChatMemoryEnv(t)
	alice, bob := env.member("Alice"), env.member("Bob")
	otherChat := uniqueModuleChatID()
	t.Cleanup(func() { _ = membermem.DeleteChat(context.Background(), otherChat) })
	chatMemorySeed(t, env.chatID, alice.Id, "Has two cats")
	chatMemorySeed(t, env.chatID, bob.Id, "Plays chess")
	chatMemorySeed(t, otherChat, alice.Id, "Collects stamps")

	command := func(from *gotgbot.User) []string {
		msg := env.message(from, "/forgetme")
		msg.Entities = []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: int64(len("/forgetme"))}}
		env.deliver(msg)
		return env.sends()
	}
	sends := command(alice)
	if len(sends) != 1 || sends[0] != chatCanned(t, "chat_forgetme_done") {
		t.Fatalf("/forgetme answered %q", sends)
	}
	if got := chatMemoryNotes(t, env.chatID, alice.Id); len(got) != 0 {
		t.Errorf("Alice's notes remain: %q", got)
	}
	if got := chatMemoryNotes(t, env.chatID, bob.Id); len(got) != 1 {
		t.Errorf("Bob's notes changed: %q", got)
	}
	if got := chatMemoryNotes(t, otherChat, alice.Id); len(got) != 1 {
		t.Errorf("Alice's notes of another group changed: %q", got)
	}
	sends = command(alice)
	if len(sends) != 2 || sends[1] != chatCanned(t, "chat_forgetme_nothing") {
		t.Fatalf("a second /forgetme answered %q", sends)
	}
	// A bot sender gets nothing.
	sent := len(env.sends())
	botUser := env.member("Robot")
	botUser.IsBot = true
	command(botUser)
	if len(env.sends()) != sent {
		t.Error("a bot got an answer")
	}
}

// TestApplyChatMemories: On writes only the switch, Off deletes the group's notes (and only its own) and a
// failed delete is a failed save; turning conversations off deletes them too.
func TestApplyChatMemories(t *testing.T) {
	ctx := context.Background()
	chatID, otherChat := uniqueModuleChatID(), uniqueModuleChatID()
	chatConvCleanup(t, chatID)
	chatConvCleanup(t, otherChat)
	t.Cleanup(func() {
		_ = membermem.DeleteChat(ctx, chatID)
		_ = membermem.DeleteChat(ctx, otherChat)
	})
	withChatRuntime(t)
	chatMemorySeed(t, chatID, 41, "Has a dog")
	chatMemorySeed(t, otherChat, 41, "Has a cat")

	if res := applyChatMemories(ctx, chatID, true, 1); res.Outcome != chatApplied || res.Settings == nil || !res.Settings.MemoriesEnabled {
		t.Fatalf("On = %+v", res)
	}
	if got := chatMemoryNotes(t, chatID, 41); len(got) != 1 {
		t.Fatalf("turning memories on deleted notes: %q", got)
	}

	chatSwap(t, &chatNotesDeleteChat, func(context.Context, int64) error { return errors.New("db down") })
	if res := applyChatMemories(ctx, chatID, false, 1); res.Outcome != chatSaveFailed {
		t.Fatalf("Off with a failing delete = %+v, want a failed save", res)
	}
	chatSwap(t, &chatNotesDeleteChat, membermem.DeleteChat)
	if res := applyChatMemories(ctx, chatID, false, 1); res.Outcome != chatApplied || res.Settings == nil || res.Settings.MemoriesEnabled {
		t.Fatalf("Off = %+v", res)
	}
	if got := chatMemoryNotes(t, chatID, 41); len(got) != 0 {
		t.Errorf("Off left notes: %q", got)
	}
	if got := chatMemoryNotes(t, otherChat, 41); len(got) != 1 {
		t.Errorf("Off deleted another group's notes: %q", got)
	}

	// Conversations off take the notes with them, and the memories switch itself is left as the owner set it.
	_ = applyChatMemories(ctx, chatID, true, 1)
	chatMemorySeed(t, chatID, 42, "Plays chess")
	if res := applyChatEnabled(ctx, chatID, false, 1); res.Outcome != chatApplied {
		t.Fatalf("conversations Off = %+v", res)
	}
	if got := chatMemoryNotes(t, chatID, 42); len(got) != 0 {
		t.Errorf("conversations Off left notes: %q", got)
	}
	if got, _ := chatconv.GetFresh(ctx, chatID); !got.MemoriesEnabled {
		t.Error("conversations Off changed the memories switch")
	}
	chatMemorySeed(t, chatID, 43, "Sings")
	chatSwap(t, &chatNotesDeleteChat, func(context.Context, int64) error { return errors.New("db down") })
	if res := applyChatEnabled(ctx, chatID, false, 1); res.Outcome != chatSaveFailed {
		t.Fatalf("conversations Off with a failing delete = %+v, want a failed save", res)
	}
}

// TestSettingsChatMemories: the owner's toggle on the Conversations page shares its row with the Presence
// shortcut, turns the feature on in one tap, and Off deletes the group's notes; a group menu, an
// administrator and a server with no provider are refused as for the Message log.
func TestSettingsChatMemories(t *testing.T) {
	withChatProviders(t, true, true, true)
	env := settingsChatEnv(t)
	t.Cleanup(func() { _ = membermem.DeleteChat(context.Background(), env.chat.Id) })
	owner := env.newOwner(env.chat)
	private := settingsPrivateChat(owner)
	msgID := settingsChatOpenPrivate(t, env, owner)
	text, keyboard := env.lastEdit(owner.Id, msgID)
	for _, want := range []string{"settings_chat_memory_status", "settings_chat_memory_notice"} {
		if !strings.Contains(text, staffMarker(want)) {
			t.Fatalf("the page lacks %s:\n%s", want, text)
		}
	}
	if len(keyboard) > 8 {
		t.Fatalf("the page has %d rows, over the 8-row private budget", len(keyboard))
	}
	toggle, ok := settingsChatFind(t, keyboard, func(f map[string]string) bool { return f["a"] == "tg" && f["t"] == settingsChatControlMemory })
	if !ok || settingsButtonFields(t, toggle)["v"] != "1" {
		t.Fatalf("no memories toggle with v=1: %+v", keyboard)
	}
	var sharedRow bool
	for _, row := range keyboard {
		var hasToggle, hasPresence bool
		for _, button := range row {
			hasToggle = hasToggle || button.CallbackData == toggle.CallbackData
			hasPresence = hasPresence || strings.Contains(button.Text, staffMarker("settings_btn_chat_presence"))
		}
		sharedRow = sharedRow || (hasToggle && hasPresence)
	}
	if !sharedRow {
		t.Error("the memories toggle does not share a row with the Presence shortcut")
	}

	env.pressRaw(private, owner, msgID, toggle.CallbackData)
	if got := settingsChatStored(t, env.chat.Id); !got.MemoriesEnabled || got.Enabled || got.LogEnabled {
		t.Fatalf("one tap stored %+v, want only the memories switch on", got)
	}
	chatMemorySeed(t, env.chat.Id, 51, "Has a dog")
	_, keyboard = env.lastEdit(owner.Id, msgID)
	off, ok := settingsChatFind(t, keyboard, func(f map[string]string) bool { return f["a"] == "tg" && f["t"] == settingsChatControlMemory })
	if !ok || settingsButtonFields(t, off)["v"] != "0" {
		t.Fatal("the on toggle does not send v=0")
	}
	// A press of the stored value keeps the notes.
	env.pressRaw(private, owner, msgID, toggle.CallbackData)
	if got := chatMemoryNotes(t, env.chat.Id, 51); len(got) != 1 {
		t.Errorf("a press of the stored value deleted the notes: %q", got)
	}
	env.pressRaw(private, owner, msgID, off.CallbackData)
	if settingsChatStored(t, env.chat.Id).MemoriesEnabled {
		t.Fatal("Off did not turn the switch off")
	}
	if got := chatMemoryNotes(t, env.chat.Id, 51); len(got) != 0 {
		t.Errorf("Off left notes: %q", got)
	}

	// Refusals write nothing.
	env.setAdminsAllowed(env.chat.Id, true)
	groupMsg := env.openMenu(owner)
	env.press(owner, groupMsg, env.groupFields(owner, "a", "tg", "p", settingsPageChat, "t", settingsChatControlMemory, "v", "1"))
	if answer, alert := env.lastAnswer(); !alert || !strings.Contains(answer, staffMarker("common_callback_invalid_request")) {
		t.Errorf("a group press answered %q (alert %v)", answer, alert)
	}
	env.pressPrivate(env.admin, 79, env.privateFields(env.chat.Id, "a", "tg", "p", settingsPageChat, "t", settingsChatControlMemory, "v", "1"))
	if answer, alert := env.lastAnswer(); !alert || !strings.Contains(answer, staffMarker("settings_access_owner_only")) {
		t.Errorf("an administrator's press answered %q (alert %v)", answer, alert)
	}
	env.pressPrivate(owner, 80, env.privateFields(env.chat.Id, "a", "st", "p", settingsPageChat, "t", settingsChatControlMemory, "v", "1"))
	if _, alert := env.lastAnswer(); !alert {
		t.Error("an st press of the toggle was not refused")
	}
	if settingsChatStored(t, env.chat.Id).MemoriesEnabled {
		t.Fatal("a refused press turned memories on")
	}
	withChatProviders(t, false, false, false)
	env.pressPrivate(owner, 81, env.privateFields(env.chat.Id, "a", "tg", "p", settingsPageChat, "t", settingsChatControlMemory, "v", "1"))
	if answer, alert := env.lastAnswer(); !alert || !strings.Contains(answer, staffMarker("settings_chat_not_set_up")) {
		t.Errorf("On with no provider answered %q (alert %v)", answer, alert)
	}
	if settingsChatStored(t, env.chat.Id).MemoriesEnabled {
		t.Fatal("On with no provider wrote the switch")
	}
}
