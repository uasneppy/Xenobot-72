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
		{"non_standard_cat", "🐱", FormatPlain, nil, StatusInvalidEmoji, ""},
		{"non_standard_joy", "😂", FormatPlain, nil, StatusInvalidEmoji, ""},
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
