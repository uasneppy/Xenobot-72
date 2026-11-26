package modules

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/redis/go-redis/v9"

	"github.com/uasneppy/Xenobot-72/xenobot/config"
	"github.com/uasneppy/Xenobot-72/xenobot/db/chatconv"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/claude"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/gemini"
)

// The cross-cutting safeguards of the message log and the brief commands (quick task 261125-0iq):
// logs carry IDs only, the provider request is minimal, StopChat ends a running brief and nothing
// works without Redis.

// TestChatBriefLogsCarryIDsOnly sweeps the failure paths of a short brief, a long brief and a
// verdict at debug level: no line may hold a member's text, their first name, the provider's own
// message, a key value, a reply or the Staff Group's title. The live checks of /judge belong to the
// shared lockdownAuthorityMember and get no sentinel here.
func TestChatBriefLogsCarryIDsOnly(t *testing.T) {
	hook := mreqLogCapture(t)
	const (
		secretText   = "zebra-sentinel-brief-text"
		secretName   = "Quasimodo"
		secretReply  = "walrus-sentinel-brief-answer"
		providerText = "octopus-sentinel-provider-message"
		keyValue     = "chat-test-anthropic-value"
		staffTitle   = "Heron-sentinel-staff-title"
	)
	env := newChatEnv(t)
	chatLogTurnOn(t, env)
	chatSwap(t, &chatBriefUserHourlyBudget, 100)
	chatSwap(t, &chatBriefChatHourlyBudget, 100)
	t.Cleanup(func() { cache.InvalidateAdminCache(env.chatID) })
	alice, bob := env.member(secretName), env.member("Bob")
	first := env.message(alice, secretText)
	env.deliver(first)
	env.deliver(env.message(bob, secretText+" again"))
	admin, asker := env.judgeAdmin("Ada"), env.member("Dave")

	run := func(from *gotgbot.User, command string, step func(context.Context, chatRequest) (chatAnswer, error)) {
		chatSwap(t, &chatBreakers, map[string]*memberReqAIBreaker{
			chatconv.ProviderAnthropic: newMemberReqAIBreaker(), chatconv.ProviderGoogle: newMemberReqAIBreaker(), chatconv.ProviderFeatherless: newMemberReqAIBreaker(),
		})
		if step != nil {
			env.provider.script(step)
		}
		env.judge(from, command, first)
	}

	// Provider failures of every kind, one per command.
	run(asker, "/unrollshort", chatAnswerErr(&claude.HTTPError{Status: 500, Type: "api_error", Message: providerText}))
	run(asker, "/unrolllong", chatAnswerErr(&gemini.HTTPError{Status: 429, GRPCStatus: "RESOURCE_EXHAUSTED", Message: providerText}))
	run(admin, "/judge", chatAnswerErr(errors.New(providerText+" "+keyValue)))
	run(asker, "/unrollshort", chatAnswerErr(&claude.HTTPError{Status: 401, Type: "authentication_error", Message: providerText}))
	// A refusal and a blank answer.
	run(admin, "/judge", func(context.Context, chatRequest) (chatAnswer, error) { return chatAnswer{Outcome: chatRefused}, nil })
	run(asker, "/unrolllong", chatAnswerText("  "))
	// A send Telegram refuses.
	env.client.errors["sendMessage"] = &gotgbot.TelegramError{Code: 400, Description: providerText}
	run(asker, "/unrollshort", chatAnswerText(secretReply))
	delete(env.client.errors, "sendMessage")
	// A blocked private chat.
	env.fake.refuse(admin.Id)
	run(admin, "/judge pm", chatAnswerText(secretReply))
	// No Staff Group, a link that cannot be read, and a Staff Group that refuses the verdict.
	run(admin, "/judge staff", nil)
	readLink := chatJudgeStaffLink
	chatJudgeStaffLink = func(int64) (*models.StaffGroupLink, error) { return nil, errors.New(providerText) }
	run(admin, "/judge staff", nil)
	chatJudgeStaffLink = readLink
	judgeLinkStaff(t, env, staffTitle)
	env.client.errors["sendMessage"] = &gotgbot.TelegramError{Code: 500, Description: providerText}
	run(admin, "/judge staff", chatAnswerText(secretReply))
	delete(env.client.errors, "sendMessage")
	// A Redis fault, at the log watcher and at the command.
	env.mr.SetError("sentinel redis fault " + providerText)
	env.deliver(env.message(alice, secretText))
	run(asker, "/unrollshort", nil)
	env.mr.SetError("")

	lines := mreqLogLines(hook)
	var chatLines int
	for _, line := range lines {
		if strings.Contains(line, "[Chat]") {
			chatLines++
		}
		for _, value := range []string{secretText, secretName, secretReply, providerText, keyValue, staffTitle} {
			if strings.Contains(line, value) {
				t.Fatalf("a log line carries %q: %s", value, line)
			}
		}
	}
	if chatLines == 0 {
		t.Fatal("no [Chat] line was logged, so the sweep proves nothing")
	}
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"HTTP 500 api_error", "HTTP 401 authentication_error", "Telegram 500", "redis error"} {
		if !strings.Contains(joined, want) {
			t.Errorf("no log line names %q", want)
		}
	}
}

// TestChatBriefRequestIsMinimal reads the real provider bodies of a verdict (Anthropic) and of a
// short brief (Google) over members with usernames and large user IDs, in a group linked to a Staff
// Group: no user ID, username, chat ID, Staff Group ID or title and no key value is in a body. The
// verdict's roster is part of the user text and holds first names only.
func TestChatBriefRequestIsMinimal(t *testing.T) {
	standIn := &chatProviderStandIn{}
	server := httptest.NewServer(http.HandlerFunc(standIn.handler))
	t.Cleanup(server.Close)

	env := newChatEnv(t)
	chatLogTurnOn(t, env)
	t.Cleanup(func() { cache.InvalidateAdminCache(env.chatID) })
	chatSwap(t, &chatProviderOf, chatProviderFor)
	chatSwap(t, &chatAnthropicEndpoint, server.URL)
	chatSwap(t, &chatGeminiEndpoint, server.URL)
	chatSwap(t, &chatFeatherlessEndpoint, server.URL)
	chatSwap(t, &chatProviderHTTP, server.Client())
	cfg := config.AppConfig
	chatSwap(t, &cfg.ChatAnthropicModel, "claude-haiku-5-5")
	chatSwap(t, &cfg.ChatGeminiModel, "gemini-4-hydrogen")
	chatSwap(t, &cfg.ChatFeatherlessModel, "Qwen/Qwen3-32B")
	const staffTitle = "Heron-sentinel-staff-title"
	staffChat, _ := judgeLinkStaff(t, env, staffTitle)

	alice, bob := env.member("Alice"), env.member("Bob")
	alice.Username, bob.Username = "alice_handle_x7", "bob_handle_x7"
	first := env.message(alice, "it started here")
	env.deliver(first)
	env.deliver(env.message(bob, "no, it did not"))
	env.deliver(env.message(alice, "yes it did"))
	admin := env.judgeAdmin("Ada")
	admin.Username = "ada_handle_x7"

	env.judge(admin, "/judge staff", first)
	if err := chatconv.SetProvider(context.Background(), env.chatID, chatconv.ProviderGoogle, 1); err != nil {
		t.Fatalf("SetProvider google: %v", err)
	}
	env.judge(bob, "/unrollshort", first)
	if err := chatconv.SetProvider(context.Background(), env.chatID, chatconv.ProviderFeatherless, 1); err != nil {
		t.Fatalf("SetProvider featherless: %v", err)
	}
	env.judge(alice, "/unrollshort", first)

	if len(standIn.bodies) != 3 {
		t.Fatalf("provider bodies = %d, want 3", len(standIn.bodies))
	}
	if !strings.Contains(standIn.bodies[0], "Alice") || !strings.Contains(standIn.bodies[1], "Bob") || !strings.Contains(standIn.bodies[2], "Alice") {
		t.Fatal("the bodies hold no first name, so the sweep proves nothing")
	}
	forbidden := []string{
		strconv.FormatInt(alice.Id, 10), strconv.FormatInt(bob.Id, 10), strconv.FormatInt(admin.Id, 10),
		strconv.FormatInt(env.chatID, 10), strconv.FormatInt(staffChat, 10), staffTitle,
		alice.Username, bob.Username, admin.Username, cfg.AnthropicAPIKey, cfg.GeminiAPIKey, cfg.FeatherlessAPIKey,
	}
	for i, body := range standIn.bodies {
		for _, value := range forbidden {
			if strings.Contains(body, value) {
				t.Fatalf("provider body %d carries %q", i, value)
			}
		}
	}
	if got := env.sendsTo(staffChat); len(got) != 1 {
		t.Fatalf("the Staff Group got %d messages, want the one verdict", len(got))
	}
}

// TestShutdownEndsChatBriefs holds that a brief is one of the jobs StopChat drains: a brief inside a
// provider call that never answers is counted in chatJobs, is ended by StopChat inside its wait,
// sends nothing and spends no budget beyond its one reserved unit, and a later /unrollshort starts
// no job and asks no provider.
func TestShutdownEndsChatBriefs(t *testing.T) {
	env := newChatEnv(t)
	chatLogTurnOn(t, env)
	chatSwap(t, &chatStopWait, 2*time.Second)
	alice, bob := env.member("Alice"), env.member("Bob")
	first := env.message(alice, "it started here")
	env.deliver(first)
	env.deliver(env.message(bob, "and went on"))
	started := make(chan struct{})
	env.provider.script(func(ctx context.Context, _ chatRequest) (chatAnswer, error) {
		close(started)
		<-ctx.Done()
		return chatAnswer{}, ctx.Err()
	})
	restoreChatPool(t)

	env.nextUpdate++
	command := env.briefCommand(bob, first)
	if err := env.dispatcher.ProcessUpdate(env.bot, &gotgbot.Update{UpdateId: env.nextUpdate, Message: command}, nil); err != nil {
		t.Fatalf("ProcessUpdate: %v", err)
	}
	<-started
	select {
	case <-chatJobs.idleChan():
		t.Fatal("a running brief is not counted in chatJobs")
	default:
	}
	budget := judgeBudgetKeys(env)
	if !StopChat() {
		t.Fatal("StopChat timed out")
	}
	shed := chatBriefShedCount.Load()
	env.deliver(env.briefCommand(alice, first))
	if sends := env.sends(); len(sends) != 0 {
		t.Fatalf("sends = %q, want none", sends)
	}
	if got := len(env.provider.calls()); got != 1 {
		t.Fatalf("provider calls = %d, want 1", got)
	}
	if got := judgeBudgetKeys(env); got != budget {
		t.Fatalf("budget keys went from %d to %d after the stop", budget, got)
	}
	if chatBriefShedCount.Load() != shed+1 {
		t.Fatal("the late command was not shed")
	}
}

// TestChatBriefInertWithoutRedis holds that with no Redis client the log watcher stores nothing and
// /unrollshort, /unrolllong and /judge answer nothing and ask no provider.
func TestChatBriefInertWithoutRedis(t *testing.T) {
	env := newChatEnv(t)
	chatLogTurnOn(t, env)
	t.Cleanup(func() { cache.InvalidateAdminCache(env.chatID) })
	chatSwap(t, &chatRedisClient, func() *redis.Client { return nil })
	alice := env.member("Alice")
	first := env.message(alice, "it started here")
	env.deliver(first)
	admin := env.judgeAdmin("Ada")
	for _, command := range []string{"/unrollshort", "/unrolllong", "/judge", "/judge pm"} {
		env.judge(admin, command, first)
	}
	if sends := env.sends(); len(sends) != 0 {
		t.Fatalf("sends = %q, want none", sends)
	}
	if n := len(env.provider.calls()); n != 0 {
		t.Fatalf("provider calls = %d, want none", n)
	}
	if keys := env.mr.Keys(); len(keys) != 0 {
		t.Fatalf("Redis holds %v, want nothing", keys)
	}
	if got := env.sendsTo(admin.Id); len(got) != 0 {
		t.Fatalf("the admin got %q in private", got)
	}
}
