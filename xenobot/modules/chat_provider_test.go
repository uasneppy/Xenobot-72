package modules

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/uasneppy/Xenobot-72/xenobot/config"
	"github.com/uasneppy/Xenobot-72/xenobot/db/chatconv"
)

// chatProviderStandIn is one httptest server that answers all three providers' conversation calls
// and records every body it received.
type chatProviderStandIn struct {
	mu     sync.Mutex
	bodies []string
}

func (s *chatProviderStandIn) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	s.mu.Lock()
	s.bodies = append(s.bodies, string(body))
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch {
	case strings.HasSuffix(r.URL.Path, ":generateContent"):
		_, _ = io.WriteString(w, `{"candidates":[{"content":{"role":"model","parts":[{"text":"Gemini unit on duty."}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":4}}`)
	case strings.HasSuffix(r.URL.Path, "/v1/chat/completions"):
		_, _ = io.WriteString(w, `{"model":"Qwen/Qwen3-32B","choices":[{"message":{"role":"assistant","content":"Featherless unit on duty."},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":4}}`)
	default:
		_, _ = io.WriteString(w, `{"model":"claude-haiku-5-5","stop_reason":"end_turn","content":[{"type":"text","text":"Claude unit on duty."}],"usage":{"input_tokens":10,"output_tokens":4}}`)
	}
}

// TestChatProvidersRequest drives all three real adapters through the watcher: each answers, the
// request names no user ID, chat ID or username, and the static system prefix is the same bytes
// across groups and providers, which is what makes provider prompt caching hit.
func TestChatProvidersRequest(t *testing.T) {
	standIn := &chatProviderStandIn{}
	server := httptest.NewServer(http.HandlerFunc(standIn.handler))
	t.Cleanup(server.Close)

	envA := newChatEnv(t)
	envB := newChatEnv(t)
	envC := newChatEnv(t)
	chatSwap(t, &chatProviderOf, chatProviderFor)
	chatSwap(t, &chatAnthropicEndpoint, server.URL)
	chatSwap(t, &chatGeminiEndpoint, server.URL)
	chatSwap(t, &chatFeatherlessEndpoint, server.URL)
	chatSwap(t, &chatProviderHTTP, server.Client())
	cfg := config.AppConfig
	chatSwap(t, &cfg.ChatAnthropicModel, "claude-haiku-5-5")
	chatSwap(t, &cfg.ChatGeminiModel, "gemini-4-hydrogen")
	chatSwap(t, &cfg.ChatFeatherlessModel, "Qwen/Qwen3-32B")

	alice := envA.member("Alice")
	alice.Username = "alice_handle"
	envA.deliver(envA.mention(alice, "status report"))

	if err := chatconv.SetProvider(context.Background(), envB.chatID, chatconv.ProviderGoogle, 1); err != nil {
		t.Fatalf("SetProvider google: %v", err)
	}
	bob := envB.member("Bob")
	bob.Username = "bob_handle"
	envB.deliver(envB.mention(bob, "status report"))

	if err := chatconv.SetProvider(context.Background(), envC.chatID, chatconv.ProviderFeatherless, 1); err != nil {
		t.Fatalf("SetProvider featherless: %v", err)
	}
	carol := envC.member("Carol")
	carol.Username = "carol_handle"
	envC.deliver(envC.mention(carol, "status report"))

	if got := envA.sends(); len(got) != 1 || got[0] != "Claude unit on duty." {
		t.Fatalf("anthropic group sends = %q", got)
	}
	if got := envB.sends(); len(got) != 1 || got[0] != "Gemini unit on duty." {
		t.Fatalf("google group sends = %q", got)
	}
	if got := envC.sends(); len(got) != 1 || got[0] != "Featherless unit on duty." {
		t.Fatalf("featherless group sends = %q", got)
	}
	if len(standIn.bodies) != 3 {
		t.Fatalf("provider bodies = %d, want 3", len(standIn.bodies))
	}
	forbidden := []string{
		strconv.FormatInt(alice.Id, 10), strconv.FormatInt(bob.Id, 10), strconv.FormatInt(carol.Id, 10),
		strconv.FormatInt(envA.chatID, 10), strconv.FormatInt(envB.chatID, 10), strconv.FormatInt(envC.chatID, 10),
		"alice_handle", "bob_handle", "carol_handle",
	}
	for _, body := range standIn.bodies {
		for _, value := range forbidden {
			if strings.Contains(body, value) {
				t.Fatalf("a provider body carries %q", value)
			}
		}
	}

	var claudeBody struct {
		System []struct {
			Text string `json:"text"`
		} `json:"system"`
	}
	var geminiBody struct {
		SystemInstruction struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"systemInstruction"`
	}
	var featherlessBody struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(standIn.bodies[0]), &claudeBody); err != nil || len(claudeBody.System) != 2 {
		t.Fatalf("anthropic body: %v, %d system blocks", err, len(claudeBody.System))
	}
	if err := json.Unmarshal([]byte(standIn.bodies[1]), &geminiBody); err != nil || len(geminiBody.SystemInstruction.Parts) != 2 {
		t.Fatalf("gemini body: %v", err)
	}
	if err := json.Unmarshal([]byte(standIn.bodies[2]), &featherlessBody); err != nil || len(featherlessBody.Messages) != 2 {
		t.Fatalf("featherless body: %v", err)
	}
	if claudeBody.System[0].Text != geminiBody.SystemInstruction.Parts[0].Text {
		t.Fatalf("the static system block differs between Anthropic and Google")
	}
	if !strings.HasPrefix(featherlessBody.Messages[0].Content, claudeBody.System[0].Text) {
		t.Fatalf("Featherless system message does not start with the static system prefix")
	}

	// The output cap reaches all three providers: the default for a conversation turn and a
	// request's own MaxTokens (a brief's) when it sets one.
	capOf := func(body string, provider string) int {
		t.Helper()
		var parsed struct {
			MaxTokens           int `json:"max_tokens"`
			MaxCompletionTokens int `json:"max_completion_tokens"`
			GenerationConfig    struct {
				MaxOutputTokens int `json:"maxOutputTokens"`
			} `json:"generationConfig"`
		}
		if err := json.Unmarshal([]byte(body), &parsed); err != nil {
			t.Fatalf("%s body: %v", provider, err)
		}
		switch provider {
		case chatconv.ProviderGoogle:
			return parsed.GenerationConfig.MaxOutputTokens
		case chatconv.ProviderFeatherless:
			return parsed.MaxCompletionTokens
		default:
			return parsed.MaxTokens
		}
	}
	if got := capOf(standIn.bodies[0], chatconv.ProviderAnthropic); got != chatMaxOutputTokens {
		t.Errorf("anthropic default cap = %d, want %d", got, chatMaxOutputTokens)
	}
	if got := capOf(standIn.bodies[1], chatconv.ProviderGoogle); got != chatMaxOutputTokens {
		t.Errorf("google default cap = %d, want %d", got, chatMaxOutputTokens)
	}
	if got := capOf(standIn.bodies[2], chatconv.ProviderFeatherless); got != chatMaxOutputTokens {
		t.Errorf("featherless default cap = %d, want %d", got, chatMaxOutputTokens)
	}
	for _, provider := range chatProviders {
		if _, err := chatProviderFor(provider).converse(context.Background(), chatRequest{Static: "s", Dynamic: "d", User: "u", MaxTokens: 1234}); err != nil {
			t.Fatalf("%s converse: %v", provider, err)
		}
		if got := capOf(standIn.bodies[len(standIn.bodies)-1], provider); got != 1234 {
			t.Errorf("%s request cap = %d, want 1234", provider, got)
		}
	}
}
