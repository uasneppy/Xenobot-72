//go:build testtools

package featherless

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	testKey   = "rc_FAKEFAKEFAKEFAKEFAKEFAKEFAKEFAKE012345"
	testModel = "Qwen/Qwen3-32B"
)

type recorded struct {
	Path     string
	RawQuery string
	Header   http.Header
	Body     []byte
}

type recorder struct {
	mu   sync.Mutex
	reqs []recorded
}

func (r *recorder) all() []recorded {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]recorded(nil), r.reqs...)
}

func newServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *recorder) {
	t.Helper()
	rec := &recorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rec.mu.Lock()
		rec.reqs = append(rec.reqs, recorded{Path: r.URL.EscapedPath(), RawQuery: r.URL.RawQuery, Header: r.Header.Clone(), Body: body})
		rec.mu.Unlock()
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	return server, rec
}

func newClient(server *httptest.Server) *Client {
	return &Client{Endpoint: server.URL, APIKey: testKey, Model: testModel, HTTP: server.Client()}
}

func respond(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

func chatAnswer(finish, content string) string {
	raw, _ := json.Marshal(map[string]any{
		"model": testModel,
		"choices": []any{map[string]any{
			"index":         0,
			"message":       map[string]any{"role": "assistant", "content": content},
			"finish_reason": finish,
		}},
		"usage": map[string]any{
			"prompt_tokens":     11500,
			"completion_tokens": 90,
			"prompt_tokens_details": map[string]any{
				"cached_tokens": 9800,
			},
		},
	})
	return string(raw)
}

func sampleRequest() ConverseRequest {
	return ConverseRequest{Static: "STATIC", Dynamic: "Group: Test", User: "Resident: hello", MaxTokens: 2000}
}

func TestValidModel(t *testing.T) {
	for _, tc := range []struct {
		name string
		want bool
	}{
		{"Qwen/Qwen3-32B", true},
		{"google/gemma-3-27b-it", true},
		{"meta-llama/Meta-Llama-3.1-8B-Instruct", true},
		{"deepseek-ai/DeepSeek-V3-0324", true},
		{"org_1/model.v2", true},
		{"", false},
		{"rc_FAKEFAKEKEY0123456789", false},
		{"claude-haiku-5-5", false},
		{"../v1/files", false},
		{"org/../secret", false},
		{"a/b/c", false},
		{"Qwen/Qwen3 32B", false},
		{"Qwen/Qwen3?key=1", false},
		{"https://api.featherless.ai/v1", false},
		{"org/" + strings.Repeat("a", 125), false},
	} {
		if got := ValidModel(tc.name); got != tc.want {
			t.Errorf("ValidModel(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestConverseRequestShape(t *testing.T) {
	t.Run("text only sends plain user string and static-first system", func(t *testing.T) {
		server, rec := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
			respond(w, http.StatusOK, chatAnswer("stop", "Access granted."))
		})
		reply, err := newClient(server).Converse(context.Background(), sampleRequest())
		if err != nil {
			t.Fatalf("Converse: %v", err)
		}
		if reply.Text != "Access granted." || reply.Outcome != OutcomeAnswered || reply.InputTokens != 11500 || reply.OutputTokens != 90 || reply.CacheReadTokens != 9800 {
			t.Fatalf("reply = %+v", reply)
		}
		req := rec.all()[0]
		if req.Path != "/v1/chat/completions" {
			t.Errorf("path = %s", req.Path)
		}
		if req.RawQuery != "" {
			t.Errorf("query = %q, key must never be in URL", req.RawQuery)
		}
		if req.Header.Get("Authorization") != "Bearer "+testKey {
			t.Error("Authorization header missing or wrong")
		}

		var body struct {
			Model               string `json:"model"`
			MaxCompletionTokens int    `json:"max_completion_tokens"`
			Messages            []struct {
				Role    string `json:"role"`
				Content any    `json:"content"`
			} `json:"messages"`
		}
		if err := json.Unmarshal(req.Body, &body); err != nil {
			t.Fatal(err)
		}
		if body.Model != testModel || body.MaxCompletionTokens != 2000 || len(body.Messages) != 2 {
			t.Fatalf("body = %+v", body)
		}
		if body.Messages[0].Role != "system" || body.Messages[0].Content != "STATIC\n\nGroup: Test" {
			t.Errorf("system message = %+v", body.Messages[0])
		}
		if body.Messages[1].Role != "user" || body.Messages[1].Content != "Resident: hello" {
			t.Errorf("user message = %+v", body.Messages[1])
		}
	})

	t.Run("endpoint with trailing /v1 is normalised", func(t *testing.T) {
		server, rec := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
			respond(w, http.StatusOK, chatAnswer("stop", "ok"))
		})
		c := newClient(server)
		c.Endpoint = server.URL + "/v1/"
		if _, err := c.Converse(context.Background(), sampleRequest()); err != nil {
			t.Fatalf("Converse: %v", err)
		}
		if got := rec.all()[0].Path; got != "/v1/chat/completions" {
			t.Errorf("path = %s, want /v1/chat/completions", got)
		}
	})

	t.Run("multimodal request sends text before base64 image_url", func(t *testing.T) {
		server, rec := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
			respond(w, http.StatusOK, chatAnswer("stop", "A sticker."))
		})
		in := sampleRequest()
		imgBytes := []byte("RIFF\x10\x00\x00\x00WEBPVP8 sticker")
		in.Images = []Image{{MIMEType: "image/webp", Data: imgBytes}}
		reply, err := newClient(server).Converse(context.Background(), in)
		if err != nil || reply.Text != "A sticker." {
			t.Fatalf("reply = %+v err = %v", reply, err)
		}
		var body struct {
			Messages []struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
		}
		if err := json.Unmarshal(rec.all()[0].Body, &body); err != nil {
			t.Fatal(err)
		}
		var parts []struct {
			Type     string `json:"type"`
			Text     string `json:"text"`
			ImageURL *struct {
				URL string `json:"url"`
			} `json:"image_url"`
		}
		if err := json.Unmarshal(body.Messages[1].Content, &parts); err != nil {
			t.Fatal(err)
		}
		if len(parts) != 2 || parts[0].Type != "text" || parts[0].Text != in.User {
			t.Fatalf("first part = %+v, want text prompt first", parts)
		}
		wantURL := "data:image/webp;base64," + base64.StdEncoding.EncodeToString(imgBytes)
		if parts[1].Type != "image_url" || parts[1].ImageURL == nil || parts[1].ImageURL.URL != wantURL {
			t.Fatalf("second part = %+v, want data URL", parts[1])
		}
	})
}

func TestConverseAnswers(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		outcome Outcome
		text    string
		noReply bool
	}{
		{"stop", chatAnswer("stop", "Procedure complete."), OutcomeAnswered, "Procedure complete.", false},
		{"uppercase stop", chatAnswer("STOP", "ok"), OutcomeAnswered, "ok", false},
		{"strips leading think block", chatAnswer("stop", "<think>internal thoughts</think>\nFinal reply."), OutcomeAnswered, "Final reply.", false},
		{"unclosed think block is no reply", chatAnswer("length", "<think>still thinking..."), 0, "", true},
		{"ignores reasoning_content", `{"model":"Qwen/Qwen3-32B","choices":[{"message":{"role":"assistant","reasoning_content":"plan","content":"Answer."},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"cached_tokens":7}}`, OutcomeAnswered, "Answer.", false},
		{"length with text", chatAnswer("length", "Half"), OutcomeTruncated, "Half", false},
		{"length blank", chatAnswer("length", "   "), 0, "", true},
		{"content_filter", chatAnswer("content_filter", ""), OutcomeRefused, "", false},
		{"message refusal", `{"choices":[{"message":{"role":"assistant","content":"","refusal":"Cannot comply."},"finish_reason":"stop"}]}`, OutcomeRefused, "", false},
		{"tool_calls is no reply", chatAnswer("tool_calls", "x"), 0, "", true},
		{"no choices", `{"usage":{}}`, 0, "", true},
		{"no message", `{"choices":[{"finish_reason":"stop"}]}`, 0, "", true},
		{"blank stop", chatAnswer("stop", "  "), 0, "", true},
		{"unreadable", `nope`, 0, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server, _ := newServer(t, func(w http.ResponseWriter, _ *http.Request) { respond(w, http.StatusOK, tc.body) })
			reply, err := newClient(server).Converse(context.Background(), sampleRequest())
			if tc.noReply {
				if !errors.Is(err, ErrNoReply) {
					t.Fatalf("err = %v, want ErrNoReply", err)
				}
				return
			}
			if err != nil || reply.Outcome != tc.outcome || reply.Text != tc.text {
				t.Fatalf("reply %+v err %v", reply, err)
			}
		})
	}
}

func TestClassifyAndNames(t *testing.T) {
	jpeg := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}

	t.Run("Classify image returns probability and sends json_object + image_url", func(t *testing.T) {
		server, rec := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
			respond(w, http.StatusOK, chatAnswer("stop", `{"spam_probability": 0.92}`))
		})
		v, err := newClient(server).Classify(context.Background(), Image{MIMEType: "image/jpeg", Data: jpeg})
		if err != nil || v.SpamProbability != 0.92 {
			t.Fatalf("verdict = %+v err = %v", v, err)
		}
		if !bytes.Contains(rec.all()[0].Body, []byte(`"response_format":{"type":"json_object"}`)) {
			t.Errorf("request missing json_object response_format: %s", rec.all()[0].Body)
		}
		if !bytes.Contains(rec.all()[0].Body, []byte(`"data:image/jpeg;base64,`)) {
			t.Errorf("request missing data URL: %s", rec.all()[0].Body)
		}
	})

	t.Run("ClassifyNames sends escaped JSON and handles fenced or reasoned JSON", func(t *testing.T) {
		for _, body := range []string{
			chatAnswer("stop", `{"spam_probability":0.15}`),
			chatAnswer("stop", "<think>not spam</think>\n```json\n{\"spam_probability\": 0.15}\n```"),
		} {
			server, rec := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
				respond(w, http.StatusOK, body)
			})
			v, err := newClient(server).ClassifyNames(context.Background(), Names{First: `Ada "Quote"`, Last: "Lovelace", Username: "ada"})
			if err != nil || v.SpamProbability != 0.15 {
				t.Fatalf("verdict = %+v err = %v", v, err)
			}
			if !bytes.Contains(rec.all()[0].Body, []byte(`\"first_name\":\"Ada \\\"Quote\\\"\"`)) {
				t.Errorf("names not JSON-escaped in request: %s", rec.all()[0].Body)
			}
		}
	})

	t.Run("verdict refusals and malformed answers", func(t *testing.T) {
		cases := []struct {
			name    string
			body    string
			blocked bool
		}{
			{"content_filter", chatAnswer("content_filter", ""), true},
			{"refusal field", `{"choices":[{"message":{"refusal":"no"},"finish_reason":"stop"}]}`, true},
			{"truncated", chatAnswer("length", `{"spam_probability":0.9}`), false},
			{"out of range high", chatAnswer("stop", `{"spam_probability":1.5}`), false},
			{"out of range negative", chatAnswer("stop", `{"spam_probability":-0.1}`), false},
			{"missing field", chatAnswer("stop", `{"other":0.5}`), false},
			{"trailing garbage", chatAnswer("stop", `{"spam_probability":0.5} extra`), false},
			{"not json", chatAnswer("stop", `spam`), false},
			{"unclosed think", chatAnswer("stop", `<think>{"spam_probability":0.5}`), false},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				server, _ := newServer(t, func(w http.ResponseWriter, _ *http.Request) { respond(w, http.StatusOK, tc.body) })
				_, err := newClient(server).ClassifyNames(context.Background(), Names{First: "Bob"})
				if !errors.Is(err, ErrNoVerdict) {
					t.Fatalf("err = %v, want ErrNoVerdict", err)
				}
				if got := errors.Is(err, ErrBlocked); got != tc.blocked {
					t.Errorf("ErrBlocked = %v, want %v (err %v)", got, tc.blocked, err)
				}
			})
		}
	})
}

func featherlessError(errType, code, message string) string {
	raw, _ := json.Marshal(map[string]any{
		"error": map[string]any{"type": errType, "code": code, "message": message},
	})
	return string(raw)
}

func TestErrorsAndSecurity(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		retryAfter string
		body       string
		retryable  bool
		rejected   bool
		calls      int
	}{
		{"400 invalid", 400, "", featherlessError("invalid_request_error", "max_tokens_too_large", "too big"), false, false, 1},
		{"401 unauthorized", 401, "", featherlessError("authentication_error", "invalid_api_key", "bad key"), false, true, 1},
		{"402 billing", 402, "", featherlessError("billing_error", "insufficient_quota", "no credit"), false, true, 1},
		{"403 forbidden", 403, "", featherlessError("permission_error", "forbidden", "no access"), false, true, 1},
		{"404 not found", 404, "", `{"detail":"model not found"}`, false, false, 1},
		{"429 long retry-after", 429, "15", featherlessError("rate_limit_error", "rate_limit_exceeded", "slow down"), true, false, 1},
		{"429 huge retry-after does not overflow", 429, "18446744074", featherlessError("rate_limit_error", "rate_limit_exceeded", "slow down"), true, false, 1},
		{"429 insufficient quota not retried", 429, "", featherlessError("rate_limit_error", "insufficient_quota", "no quota"), false, true, 1},
		{"429 billing error not retried", 429, "", featherlessError("billing_error", "", "billing issue"), false, true, 1},
		{"429 bare retried once", 429, "", ``, true, false, 2},
		{"503 insufficient capacity retried once", 503, "", featherlessError("server_error", "capacity", "busy"), true, false, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server, rec := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
				if tc.retryAfter != "" {
					w.Header().Set("Retry-After", tc.retryAfter)
				}
				respond(w, tc.status, tc.body)
			})
			_, err := newClient(server).Converse(context.Background(), sampleRequest())
			var httpErr *HTTPError
			if !errors.As(err, &httpErr) {
				t.Fatalf("err = %v, want *HTTPError", err)
			}
			if httpErr.Status != tc.status || httpErr.Retryable() != tc.retryable || httpErr.KeyRejected() != tc.rejected {
				t.Errorf("status %d retryable %v rejected %v", httpErr.Status, httpErr.Retryable(), httpErr.KeyRejected())
			}
			if got := len(rec.all()); got != tc.calls {
				t.Errorf("requests = %d, want %d", got, tc.calls)
			}
		})
	}

	t.Run("short retry then success", func(t *testing.T) {
		calls := 0
		server, _ := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
			calls++
			if calls == 1 {
				w.Header().Set("Retry-After", "1")
				respond(w, 429, featherlessError("rate_limit_error", "concurrency", "busy"))
				return
			}
			respond(w, 200, chatAnswer("stop", "recovered"))
		})
		reply, err := newClient(server).Converse(context.Background(), sampleRequest())
		if err != nil || reply.Text != "recovered" || calls != 2 {
			t.Fatalf("reply %+v err %v calls %d", reply, err, calls)
		}
	})

	t.Run("the key never reaches an error", func(t *testing.T) {
		server, _ := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
			respond(w, 401, featherlessError("authentication_error", "invalid_api_key", "bad key: "+testKey))
		})
		_, err := newClient(server).Converse(context.Background(), sampleRequest())
		var httpErr *HTTPError
		if !errors.As(err, &httpErr) || strings.Contains(err.Error(), testKey) || strings.Contains(httpErr.Message, testKey) {
			t.Fatalf("key leaked: %v / %q", err, httpErr.Message)
		}
	})

	t.Run("transport error carries no URL or key", func(t *testing.T) {
		c := &Client{Endpoint: "http://127.0.0.1:1", APIKey: testKey, Model: testModel}
		_, err := c.Converse(context.Background(), sampleRequest())
		if err == nil || strings.Contains(err.Error(), testKey) || strings.Contains(err.Error(), "/v1/chat/completions") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("redirect is not followed", func(t *testing.T) {
		server, rec := newServer(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/x", http.StatusFound) })
		c := &Client{Endpoint: server.URL, APIKey: testKey, Model: testModel}
		if _, err := c.Converse(context.Background(), sampleRequest()); err == nil || len(rec.all()) != 1 {
			t.Fatalf("err %v requests %d", err, len(rec.all()))
		}
	})

	t.Run("oversized answer is cut and unreadable", func(t *testing.T) {
		server, _ := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
			respond(w, 200, `{"choices":[{"message":{"content":"`+strings.Repeat("a", maxResponseBytes)+`"},"finish_reason":"stop"}]}`)
		})
		if _, err := newClient(server).Converse(context.Background(), sampleRequest()); !errors.Is(err, ErrNoReply) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("not configured and invalid inputs make no request", func(t *testing.T) {
		server, rec := newServer(t, func(w http.ResponseWriter, _ *http.Request) { respond(w, 200, chatAnswer("stop", "ok")) })
		c := newClient(server)
		c.APIKey = ""
		if _, err := c.Converse(context.Background(), sampleRequest()); !errors.Is(err, ErrNotConfigured) {
			t.Error(err)
		}
		c = newClient(server)
		c.Model = "not-a-hf-id"
		if _, err := c.Converse(context.Background(), sampleRequest()); !errors.Is(err, ErrInvalidModel) {
			t.Error(err)
		}
		in := sampleRequest()
		in.Images = []Image{{MIMEType: "image/gif", Data: []byte("GIF89a")}}
		if _, err := newClient(server).Converse(context.Background(), in); !errors.Is(err, ErrUnsupportedImage) {
			t.Error(err)
		}
		if _, err := newClient(server).Classify(context.Background(), Image{MIMEType: "image/jpeg"}); !errors.Is(err, ErrUnsupportedImage) {
			t.Error(err)
		}
		if _, err := newClient(server).ClassifyNames(context.Background(), Names{}); !errors.Is(err, ErrUnsupportedNames) {
			t.Error(err)
		}
		if n := len(rec.all()); n != 0 {
			t.Errorf("requests = %d, want 0", n)
		}
	})

	t.Run("retry-after HTTP date and overflow bounds", func(t *testing.T) {
		future := time.Now().Add(30 * time.Second).UTC().Format(http.TimeFormat)
		if d := parseRetryAfter(future); d < 20*time.Second || d > 35*time.Second {
			t.Errorf("delay = %v", d)
		}
		if d := parseRetryAfter("18446744074"); d != maxParsedDelay {
			t.Errorf("parseRetryAfter(18446744074) = %v, want %v", d, maxParsedDelay)
		}
		if d := parseRetryAfter("99999999999999999999999"); d != maxParsedDelay {
			t.Errorf("parseRetryAfter(huge) = %v, want %v", d, maxParsedDelay)
		}
		if d := parseRetryAfter("-5"); d != 0 {
			t.Errorf("parseRetryAfter(-5) = %v, want 0", d)
		}
		if d := parseRetryAfter("-99999999999999999999999"); d != 0 {
			t.Errorf("parseRetryAfter(-huge) = %v, want 0", d)
		}
	})
}
