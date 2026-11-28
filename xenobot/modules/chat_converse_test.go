package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/db/chatconv"
	"github.com/uasneppy/Xenobot-72/xenobot/db/presence"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/claude"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/featherless"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/gemini"
)

// chatFuncProvider adapts a function to chatProvider for the converse tests.
type chatFuncProvider func(ctx context.Context, req chatRequest) (chatAnswer, error)

func (f chatFuncProvider) converse(ctx context.Context, req chatRequest) (chatAnswer, error) {
	return f(ctx, req)
}

func chatTestImages() []chatAttachment {
	return []chatAttachment{{MIMEType: "image/jpeg", Data: []byte{0xFF, 0xD8, 0xFF}}}
}

// TestChatConverseVisionFallback pins the Featherless vision fallback: a 400/422 on an image
// request is retried once without the images and marks the answer TextOnly, the first error is
// swallowed (it counts toward nothing), nothing is remembered between calls, and every other
// shape (no images, another provider, a key rejection, a shutdown) is passed through unchanged.
func TestChatConverseVisionFallback(t *testing.T) {
	reject := &featherless.HTTPError{Status: http.StatusBadRequest, Type: "invalid_request_error"}
	cases := []struct {
		name       string
		provider   string
		images     bool
		firstErr   error
		secondErr  error
		shutdown   bool
		wantCalls  int
		wantErr    bool
		wantTextOn bool
	}{
		{"400 with images retries without them", chatconv.ProviderFeatherless, true, reject, nil, false, 2, false, true},
		{"422 with images retries without them", chatconv.ProviderFeatherless, true, &featherless.HTTPError{Status: http.StatusUnprocessableEntity}, nil, false, 2, false, true},
		{"the retry's own failure is the result", chatconv.ProviderFeatherless, true, reject, &featherless.HTTPError{Status: http.StatusInternalServerError}, false, 2, true, false},
		{"no images, no retry", chatconv.ProviderFeatherless, false, reject, nil, false, 1, true, false},
		{"another provider, no retry", chatconv.ProviderAnthropic, true, &claude.HTTPError{Status: http.StatusBadRequest}, nil, false, 1, true, false},
		{"a rejected key is not a vision problem", chatconv.ProviderFeatherless, true, &featherless.HTTPError{Status: http.StatusBadRequest, Type: "authentication_error"}, nil, false, 1, true, false},
		{"a 500 is not a vision problem", chatconv.ProviderFeatherless, true, &featherless.HTTPError{Status: http.StatusInternalServerError}, nil, false, 1, true, false},
		{"shutdown skips the retry", chatconv.ProviderFeatherless, true, reject, nil, true, 1, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			var seen []int
			chatSwap(t, &chatProviderOf, func(string) chatProvider {
				return chatFuncProvider(func(_ context.Context, req chatRequest) (chatAnswer, error) {
					mu.Lock()
					defer mu.Unlock()
					seen = append(seen, len(req.Attachments))
					if len(seen) == 1 {
						return chatAnswer{}, tc.firstErr
					}
					return chatAnswer{Text: "ok"}, tc.secondErr
				})
			})
			root, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.shutdown {
				cancel()
			}
			req := chatRequest{Static: "s", User: "u"}
			if tc.images {
				req.Attachments = chatTestImages()
			}
			answer, err := chatConverse(context.Background(), root, chatCall{Lane: "test", ChatID: 1}, tc.provider, req)
			if len(seen) != tc.wantCalls {
				t.Fatalf("provider calls = %d, want %d", len(seen), tc.wantCalls)
			}
			if tc.wantCalls == 2 && (seen[0] != 1 || seen[1] != 0) {
				t.Fatalf("attachments per call = %v, want [1 0]", seen)
			}
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if answer.TextOnly != tc.wantTextOn {
				t.Fatalf("TextOnly = %v, want %v", answer.TextOnly, tc.wantTextOn)
			}
		})
	}
}

// TestChatEmptyAnswerNeverTripsBreaker: an answer that finished with no visible text (thinking used
// the whole output budget, a blank completion) is shown as the malfunction line but is neither a
// failure nor a success for the group's breaker, so six in a row do not pause anything.
func TestChatEmptyAnswerNeverTripsBreaker(t *testing.T) {
	env := newChatEnv(t)
	malfunction := chatCanned(t, "chat_reply_malfunction")
	starved := chatAnswerErr(&gemini.EmptyAnswerError{FinishReason: "MAX_TOKENS", ThoughtsTokens: 512})
	blank := chatAnswerErr(&featherless.EmptyAnswerError{FinishReason: "length"})
	env.provider.script(starved, blank, starved, blank, starved, blank)
	for i := range 6 {
		env.deliver(env.mention(env.member(fmt.Sprintf("E%d", i)), "hi"))
	}
	if got := len(env.provider.calls()); got != 6 {
		t.Fatalf("provider calls = %d, want 6: empty answers must not pause the group", got)
	}
	if got := env.sends(); fmt.Sprint(got) != fmt.Sprint([]string{malfunction}) {
		t.Fatalf("sends = %q, want the malfunction line once", got)
	}
	assertChatBreakersClean(t)
}

func TestChatEmptyAnswerClassification(t *testing.T) {
	for name, err := range map[string]error{
		"gemini":      &gemini.EmptyAnswerError{FinishReason: "STOP"},
		"featherless": &featherless.EmptyAnswerError{},
		"claude":      fmt.Errorf("%w: stop end_turn", claude.ErrEmptyAnswer),
		"wrapped":     fmt.Errorf("lane: %w", &gemini.EmptyAnswerError{FinishReason: "MAX_TOKENS"}),
	} {
		if !chatEmptyAnswer(err) {
			t.Errorf("%s: not classified as an empty answer", name)
		}
		if got := chatErrorClass(err); got != "empty answer" {
			t.Errorf("%s: class = %q, want \"empty answer\"", name, got)
		}
	}
	for name, err := range map[string]error{
		"no reply": gemini.ErrNoReply, "http 500": &gemini.HTTPError{Status: 500}, "transport": errors.New("reset"),
	} {
		if chatEmptyAnswer(err) {
			t.Errorf("%s: wrongly classified as an empty answer", name)
		}
	}
}

// TestChatConverseLogsDurationAndTokens: every provider call leaves one line with the lane,
// provider, duration, token counts, cache and thought tokens, and for an empty answer its finish
// reason, and none of it carries a message, a name or a key.
func TestChatConverseLogsDurationAndTokens(t *testing.T) {
	var buf bytes.Buffer
	old, oldLevel := log.StandardLogger().Out, log.GetLevel()
	log.SetOutput(&buf)
	log.SetLevel(log.DebugLevel)
	t.Cleanup(func() { log.SetOutput(old); log.SetLevel(oldLevel) })

	const secretText = "SENTINEL-MEMBER-TEXT"
	chatSwap(t, &chatProviderOf, func(string) chatProvider {
		return chatFuncProvider(func(_ context.Context, req chatRequest) (chatAnswer, error) {
			if strings.Contains(req.User, "empty") {
				return chatAnswer{}, &gemini.EmptyAnswerError{FinishReason: "MAX_TOKENS", InputTokens: 900, ThoughtsTokens: 512}
			}
			return chatAnswer{Text: secretText, InputTokens: 1200, OutputTokens: 40, CacheReadTokens: 1100, ThoughtTokens: 7}, nil
		})
	})
	root := context.Background()
	if _, err := chatConverse(root, root, chatCall{Lane: "reaction", ChatID: -100777}, chatconv.ProviderGoogle, chatRequest{User: "hello " + secretText}); err != nil {
		t.Fatal(err)
	}
	if _, err := chatConverse(root, root, chatCall{Lane: "inspect", ChatID: -100777}, chatconv.ProviderGoogle, chatRequest{User: "empty"}); err == nil {
		t.Fatal("want the empty answer error")
	}
	out := buf.String()
	var lines []map[string]any
	for _, raw := range strings.Split(strings.TrimSpace(out), "\n") {
		var entry map[string]any
		if json.Unmarshal([]byte(raw), &entry) == nil && entry["msg"] == "[Chat] provider call" {
			lines = append(lines, entry)
		}
	}
	if len(lines) != 2 {
		t.Fatalf("provider call lines = %d, want 2:\n%s", len(lines), out)
	}
	wantFields := []map[string]any{
		{"lane": "reaction", "provider": "google", "outcome": "answered", "in": 1200.0, "out": 40.0, "cache_read": 1100.0, "thoughts": 7.0},
		{"lane": "inspect", "provider": "google", "outcome": "empty", "finish": "MAX_TOKENS", "thoughts": 512.0, "in": 900.0},
	}
	for i, want := range wantFields {
		if _, ok := lines[i]["duration_ms"]; !ok {
			t.Errorf("line %d has no duration_ms: %v", i, lines[i])
		}
		for k, v := range want {
			if lines[i][k] != v {
				t.Errorf("line %d %s = %v, want %v (%v)", i, k, lines[i][k], v, lines[i])
			}
		}
	}
	if strings.Contains(out, secretText) {
		t.Errorf("a log line carries message text:\n%s", out)
	}
}

// TestChatReactReleasesCooldownsForUnusedJob: the gate starts the chat and user cooldowns when it
// enqueues, but a job that ends without reacting (provider error, refusal, a failed set) gives them
// back, while a job that reacts, or deliberately chooses NONE, keeps them.
func TestChatReactReleasesCooldownsForUnusedJob(t *testing.T) {
	cases := []struct {
		name     string
		answer   func(context.Context, chatRequest) (chatAnswer, error)
		wantKept bool
	}{
		{"provider error releases", chatAnswerErr(&gemini.HTTPError{Status: 500}), false},
		{"empty answer releases", chatAnswerErr(&gemini.EmptyAnswerError{FinishReason: "MAX_TOKENS"}), false},
		{"refusal releases", func(context.Context, chatRequest) (chatAnswer, error) { return chatAnswer{Outcome: chatRefused}, nil }, false},
		{"unparsable answer releases", chatAnswerText("not json"), false},
		{"a reaction keeps them", chatAnswerText(`{"emoji":"🔥"}`), true},
		{"NONE keeps them", chatAnswerText(`{"emoji":"NONE"}`), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newChatReactEnv(t)
			chatSwap(t, &chatReactProfiles, map[string]chatReactProfile{
				presence.AIReactModeMedium: {minGap: 1, chatCooldown: 10 * time.Minute, userCooldown: 5 * time.Minute, chancePct: 100},
			})
			alice := env.member("Alice")
			env.provider.script(tc.answer)
			env.deliver(env.message(alice, "shipped the release!"))
			if got := len(env.provider.calls()); got != 1 {
				t.Fatalf("provider calls = %d, want 1", got)
			}
			chatKey := presenceAIReactChatCooldownKey(env.chatID)
			userKey := presenceUserCooldownKey(env.chatID, alice.Id)
			if got := env.mr.Exists(chatKey); got != tc.wantKept {
				t.Fatalf("chat cooldown present = %v, want %v", got, tc.wantKept)
			}
			if got := env.mr.Exists(userKey); got != tc.wantKept {
				t.Fatalf("user cooldown present = %v, want %v", got, tc.wantKept)
			}
		})
	}

	t.Run("a longer retraction cooldown on the user key survives the release", func(t *testing.T) {
		env := newChatReactEnv(t)
		alice := env.member("Alice")
		userKey := presenceUserCooldownKey(env.chatID, alice.Id)
		if err := env.rc.Set(context.Background(), userKey, "1", 24*time.Hour).Err(); err != nil {
			t.Fatal(err)
		}
		chatReactReleaseCooldowns(env.rc, chatReactJob{chatID: env.chatID, userID: alice.Id, userCooldownSecs: 300})
		if !env.mr.Exists(userKey) {
			t.Fatal("the release deleted a 24 h retraction cooldown")
		}
	})
}
