package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSupportedPaletteCountAndNormalisation(t *testing.T) {
	if got := len(supportedReactionEmoji); got != 73 {
		t.Fatalf("len(supportedReactionEmoji) = %d, want 73", got)
	}
	for _, e := range supportedReactionEmoji {
		if strings.Contains(e, "\ufe0f") {
			t.Errorf("supportedReactionEmoji contains unstripped U+FE0F: %q", e)
		}
	}
	// Verify variation-selector forms normalize cleanly to standard entries.
	for _, tc := range []struct {
		raw  string
		want string
	}{
		{"❤️", "❤"},
		{"❤️‍🔥", "❤‍🔥"},
		{"✍️", "✍"},
		{"🕊️", "🕊"},
		{"☃️", "☃"},
		{"🤷‍♂️", "🤷‍♂"},
		{"🤷‍♀️", "🤷‍♀"},
	} {
		out := ParseReaction(tc.raw, FormatPlain, nil)
		if out.Status != StatusValidEmoji || out.Emoji != tc.want {
			t.Errorf("ParseReaction(%q) = %+v, want valid_emoji %q", tc.raw, out, tc.want)
		}
	}
}

func TestParseReactionEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		format  string
		allowed []string
		wantSt  ParseStatus
		wantEm  string
	}{
		{"plain_standard", "🤡", FormatPlain, nil, StatusValidEmoji, "🤡"},
		{"plain_none", "NONE", FormatPlain, nil, StatusValidNone, "NONE"},
		{"plain_none_lower_punct", "  none. ", FormatPlain, nil, StatusValidNone, "NONE"},
		{"json_standard", `{"emoji":"🔥"}`, FormatJSON, nil, StatusValidEmoji, "🔥"},
		{"json_fenced", "```json\n{\"emoji\":\"❤️\"}\n```", FormatJSON, nil, StatusValidEmoji, "❤"},
		{"json_none", `{"emoji":"NONE"}`, FormatJSON, nil, StatusValidNone, "NONE"},
		{"json_null", `{"emoji":null}`, FormatJSON, nil, StatusValidNone, "NONE"},
		{"json_missing_key_empty_obj", `{}`, FormatJSON, nil, StatusParseError, ""},
		{"json_wrong_key", `{"reaction":"🔥"}`, FormatJSON, nil, StatusParseError, ""},
		{"json_top_null", `null`, FormatJSON, nil, StatusParseError, ""},
		{"json_non_string_emoji", `{"emoji":123}`, FormatJSON, nil, StatusParseError, ""},
		{"non_standard_cat", "🐱", FormatPlain, nil, StatusInvalidEmoji, ""},
		{"non_standard_joy", "😂", FormatPlain, nil, StatusInvalidEmoji, ""},
		{"non_standard_zwj_family", "👨‍👩‍👧‍👦", FormatPlain, nil, StatusInvalidEmoji, ""},
		{"cyrillic_word_uk", "ні", FormatPlain, nil, StatusParseError, ""},
		{"cyrillic_word_ru", "нет", FormatPlain, nil, StatusParseError, ""},
		{"multi_emoji", "🔥👏", FormatPlain, nil, StatusMultiEmoji, ""},
		{"chatty_multiline", "Sure!\n🔥", FormatPlain, nil, StatusParseError, ""},
		{"chat_restricted_ok", "🔥", FormatPlain, restrictedPalette, StatusValidEmoji, "🔥"},
		{"chat_restricted_disallowed", "🤡", FormatPlain, restrictedPalette, StatusDisallowedByChat, "🤡"},
		{"empty", "   ", FormatPlain, nil, StatusParseError, ""},
		{"bad_json", `not json`, FormatJSON, nil, StatusParseError, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseReaction(tc.raw, tc.format, tc.allowed)
			if got.Status != tc.wantSt || got.Emoji != tc.wantEm {
				t.Errorf("ParseReaction(%q, %s) = %+v, want status=%s emoji=%q",
					tc.raw, tc.format, got, tc.wantSt, tc.wantEm)
			}
		})
	}
}

func TestMockMatrixCapturesThinkingTruncationAndPassesAt256Plus(t *testing.T) {
	cfg, cleanup := startMockProviders()
	defer cleanup()

	ctx := context.Background()
	// Gemini at MaxTokens=128 triggers ErrNoReply thinking-token truncation on Friday probe.
	cell128 := runSingleCell(ctx, cfg, "gemini", 128, FormatPlain, defaultProbes[0])
	if cell128.Parse.Status != StatusTruncatedNoReply {
		t.Fatalf("gemini max=128 status = %s, want %s (raw=%q err=%q)",
			cell128.Parse.Status, StatusTruncatedNoReply, cell128.RawText, cell128.ErrorClass)
	}
	// Gemini at MaxTokens=256 succeeds cleanly with thoughts reported.
	cell256 := runSingleCell(ctx, cfg, "gemini", 256, FormatPlain, defaultProbes[0])
	if !cell256.Parse.IsClean() || cell256.ThoughtsTokens == 0 {
		t.Fatalf("gemini max=256 = %+v, want clean parse with thoughts tokens", cell256)
	}

	// Featherless at MaxTokens=128 triggers ErrNoReply unclosed <think> truncation.
	f128 := runSingleCell(ctx, cfg, "featherless", 128, FormatJSON, defaultProbes[0])
	if f128.Parse.Status != StatusTruncatedNoReply {
		t.Fatalf("featherless max=128 status = %s, want %s", f128.Parse.Status, StatusTruncatedNoReply)
	}
	// Featherless at MaxTokens=256 strips <think> and parses clean JSON.
	f256 := runSingleCell(ctx, cfg, "featherless", 256, FormatJSON, defaultProbes[0])
	if !f256.Parse.IsClean() || f256.Parse.Emoji != "🫡" {
		t.Fatalf("featherless max=256 = %+v, want clean 🫡", f256)
	}

	// Claude parses cleanly across both formats and restricted palette.
	cRestricted := runSingleCell(ctx, cfg, "claude", 256, FormatPlain, defaultProbes[4])
	if cRestricted.Parse.Status != StatusValidEmoji || cRestricted.Parse.Emoji != "🔥" {
		t.Fatalf("claude restricted = %+v, want valid 🔥", cRestricted)
	}
}

func TestHTTPAPIAndNoSecretLeakInForensicLog(t *testing.T) {
	secret := "sk-ant-SUPER-SECRET-KEY-99999"
	base := ProviderConfig{
		AnthropicKey:     secret,
		AnthropicModel:   "claude-haiku-5-5",
		GeminiModel:      "gemini-4-hydrogen",
		FeatherlessModel: "Qwen/Qwen3-32B",
	}
	store := &ForensicStore{}
	h := newHTTPHandler(base, store, true)

	// 1. GET /api/config must report presence booleans only, never the key.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/api/config status = %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), secret) {
		t.Fatal("API secret leaked in /api/config")
	}

	// 2. POST /api/run in mock mode runs cells and populates /api/logs.
	body := bytes.NewBufferString(`{"providers":["claude","gemini"],"max_tokens":[128,256],"formats":["plain"],"mock":true,"overrides":{"anthropic_key":"` + secret + `"}}`)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/run", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("/api/run status = %d body=%s", rec.Code, rec.Body.String())
	}
	var runResp struct {
		Results []CellResult    `json:"results"`
		Buckets []BucketSummary `json:"buckets"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &runResp); err != nil {
		t.Fatalf("decode /api/run: %v", err)
	}
	if len(runResp.Results) != 2*2*len(defaultProbes) {
		t.Fatalf("results = %d, want %d", len(runResp.Results), 2*2*len(defaultProbes))
	}

	// 3. GET /api/logs exports forensic JSON with zero secret leakage.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/logs", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/api/logs status = %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), secret) {
		t.Fatal("API secret leaked in /api/logs")
	}
	var snap ForensicReport
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("decode /api/logs: %v", err)
	}
	if snap.TotalCells != len(runResp.Results) || snap.EventCount == 0 {
		t.Fatalf("unexpected forensic snapshot: cells=%d events=%d", snap.TotalCells, snap.EventCount)
	}
}

func TestSummarizeBucketsAndErrorRedaction(t *testing.T) {
	// 1. SummarizeBuckets excludes custom probes and averages tokens over answered cells only.
	results := []CellResult{
		{Provider: "gemini", MaxTokens: 128, Format: FormatPlain, ProbeID: "p1", Outcome: "answered", ThoughtsTokens: 96, OutputTokens: 6, Parse: ParseOutcome{Status: StatusValidEmoji, Emoji: "🔥"}},
		{Provider: "gemini", MaxTokens: 128, Format: FormatPlain, ProbeID: "p2", Outcome: "no_reply", ThoughtsTokens: 0, OutputTokens: 0, Parse: ParseOutcome{Status: StatusTruncatedNoReply}},
		{Provider: "gemini", MaxTokens: 128, Format: FormatPlain, ProbeID: "custom", Outcome: "answered", ThoughtsTokens: 50, OutputTokens: 5, Parse: ParseOutcome{Status: StatusValidNone, Emoji: "NONE"}},
	}
	buckets := SummarizeBuckets(results)
	if len(buckets) != 1 {
		t.Fatalf("buckets = %d, want 1", len(buckets))
	}
	if buckets[0].Total != 2 {
		t.Fatalf("bucket total = %d, want 2 (custom excluded)", buckets[0].Total)
	}
	if buckets[0].AvgThoughtTokens != 96 {
		t.Fatalf("AvgThoughtTokens = %.1f, want 96.0 (answered-only average)", buckets[0].AvgThoughtTokens)
	}

	// 2. Scrubbed HTTP 400 error message is surfaced in ErrorClass without leaking the API key.
	secret := "sk-ant-ECHOED-BY-UPSTREAM-400"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"invalid_request_error","message":"bad key ` + secret + ` rejected"}}`))
	}))
	defer srv.Close()

	cfg := ProviderConfig{AnthropicKey: secret, AnthropicModel: "claude-haiku-5-5", AnthropicEndpoint: srv.URL}
	cell := runSingleCell(context.Background(), cfg, "claude", 256, FormatPlain, defaultProbes[0])
	if cell.Parse.Status != StatusHTTPError {
		t.Fatalf("status = %s, want %s", cell.Parse.Status, StatusHTTPError)
	}
	if strings.Contains(cell.ErrorClass, secret) {
		t.Fatalf("API secret leaked in ErrorClass: %q", cell.ErrorClass)
	}
	if !strings.Contains(cell.ErrorClass, "invalid_request_error") || !strings.Contains(cell.ErrorClass, "[REDACTED]") {
		t.Fatalf("ErrorClass = %q, want scrubbed message with [REDACTED]", cell.ErrorClass)
	}

	// 3. Cancelled context stops runMatrixConcurrent immediately without hanging.
	mCfg, cleanup := startMockProviders()
	defer cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	store := &ForensicStore{mockMode: true}
	got := runMatrixConcurrent(ctx, mCfg, store, []string{"claude", "gemini"}, []string{FormatPlain}, []int{256}, defaultProbes)
	if len(got) != 0 {
		t.Fatalf("cancelled matrix ran %d cells, want 0", len(got))
	}
}
