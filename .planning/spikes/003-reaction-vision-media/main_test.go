package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFixturesValidBytesAndSniffedMIME(t *testing.T) {
	fixtures := buildFixtures()
	if len(fixtures) != 6 {
		t.Fatalf("len(fixtures) = %d, want 6", len(fixtures))
	}
	for _, f := range fixtures {
		mime, ok := sniffImageMIME(f.Data)
		if !ok {
			t.Errorf("fixture %s sniffImageMIME failed (got %q)", f.ID, mime)
		}
		if mime != f.MIMEType {
			t.Errorf("fixture %s sniffed %q, declared %q", f.ID, mime, f.MIMEType)
		}
		if len(f.Data) == 0 {
			t.Errorf("fixture %s empty bytes", f.ID)
		}
	}
	// Reject non-image bytes.
	if mime, ok := sniffImageMIME([]byte("not an image at all")); ok {
		t.Errorf("sniffImageMIME accepted plain text as %q", mime)
	}
}

func TestVisionOverheadAndFeatherlessAutoDegradation(t *testing.T) {
	cfg, cleanup := startMockProviders(true)
	defer cleanup()

	fixtures := buildFixtures()
	ctx := context.Background()

	// 1. Claude vision on captionless cat photo sees cat -> reacts ❤ and reports +320 image token overhead.
	cat := fixtures[0]
	cRes := evaluateFixture(ctx, cfg, "claude", 384, cat, true, true)
	if cRes.ParseStatus != "valid_emoji" || cRes.Emoji != "❤" || !cRes.ImageSent {
		t.Fatalf("claude cat = %+v, want valid ❤ with image sent", cRes)
	}
	if cRes.ImageTokenOverhead != 320 {
		t.Fatalf("claude image token overhead = %d, want 320", cRes.ImageTokenOverhead)
	}

	// 2. Gemini vision on mundane invoice abstains cleanly (NONE) even with image attached.
	invoice := fixtures[3]
	gRes := evaluateFixture(ctx, cfg, "gemini", 384, invoice, true, true)
	if gRes.ParseStatus != "valid_none" || gRes.Emoji != "NONE" || gRes.ImageTokenOverhead != 258 {
		t.Fatalf("gemini invoice = %+v, want valid NONE with +258 image token overhead", gRes)
	}

	// 3. Featherless vision with text-only model returning HTTP 400 automatically degrades
	// to text-only fallback on sticker_clown_roast and still produces 🤡 from [sticker 🤡] + thread!
	clownSticker := fixtures[5]
	fAuto := evaluateFixture(ctx, cfg, "featherless_vision", 384, clownSticker, false, true)
	if !fAuto.DegradedFromHTTP400 || fAuto.Mode != "auto_degraded_from_400" || fAuto.ImageSent {
		t.Fatalf("featherless auto fallback = %+v, want DegradedFromHTTP400", fAuto)
	}
	if fAuto.ParseStatus != "valid_emoji" || fAuto.Emoji != "🤡" {
		t.Fatalf("featherless degraded clown sticker = %+v, want 🤡", fAuto)
	}

	// 4. Featherless text-only fallback on captionless cat photo sees only [photo] -> abstains cleanly (NONE).
	fText := evaluateFixture(ctx, cfg, "featherless_text_fallback", 384, cat, false, true)
	if fText.ImageSent || fText.ParseStatus != "valid_none" || !fText.ExpectMatch {
		t.Fatalf("featherless text-only cat = %+v, want clean NONE abstention", fText)
	}
}

func TestHTTPUploadAndForensicLogNoSecretLeak(t *testing.T) {
	secret := "rc_SUPER_SECRET_FEATHERLESS_KEY"
	fixtures := buildFixtures()
	store := &ForensicStore{}
	h := newHTTPHandler(ProviderConfig{FeatherlessKey: secret, FeatherlessModel: "Qwen/Qwen3-32B"}, store, fixtures, true)

	// Upload a valid PNG image via /api/upload in mock mode.
	payload, _ := json.Marshal(map[string]any{
		"targets":          []string{"claude", "featherless_text_fallback"},
		"kind":             "sticker",
		"sticker_emoji":    "🤡",
		"image_base64":     "data:image/png;base64," + base64.StdEncoding.EncodeToString(fixtures[5].Data),
		"mock":             true,
		"measure_baseline": true,
		"overrides":        map[string]string{"featherless_key": secret},
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/upload", bytes.NewReader(payload)))
	if rec.Code != http.StatusOK {
		t.Fatalf("/api/upload status = %d body=%s", rec.Code, rec.Body.String())
	}

	// Reject unsupported non-image upload cleanly with HTTP 400.
	badPayload, _ := json.Marshal(map[string]any{
		"image_base64": base64.StdEncoding.EncodeToString([]byte("hello world not an image")),
		"mock":         true,
	})
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/upload", bytes.NewReader(badPayload)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid image upload status = %d, want 400", rec.Code)
	}

	// Check /api/logs has zero secret leakage.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/logs", nil))
	if strings.Contains(rec.Body.String(), secret) {
		t.Fatal("API secret leaked in /api/logs")
	}
}

func TestWebPSniffingBaselineLatencyExclusionAndFallbackFailure(t *testing.T) {
	// 1. Minimal valid RIFF WEBP header bytes (Telegram sticker format) must sniff as image/webp
	// and pass /api/upload cleanly.
	minimalWebP := []byte{
		'R', 'I', 'F', 'F', 0x1a, 0x00, 0x00, 0x00,
		'W', 'E', 'B', 'P', 'V', 'P', '8', ' ',
		0x0e, 0x00, 0x00, 0x00, 0x30, 0x01, 0x00, 0x9d,
		0x01, 0x2a, 0x01, 0x00, 0x01, 0x00, 0x00, 0x34,
		0x25, 0x9f,
	}
	if mime, ok := sniffImageMIME(minimalWebP); !ok || mime != "image/webp" {
		t.Fatalf("sniffImageMIME(webp) = (%q, %v), want (image/webp, true)", mime, ok)
	}
	store := &ForensicStore{}
	h := newHTTPHandler(ProviderConfig{}, store, buildFixtures(), true)
	upPayload, _ := json.Marshal(map[string]any{
		"targets":          []string{"claude"},
		"kind":             "sticker",
		"sticker_emoji":    "😼",
		"image_base64":     "data:image/webp;base64," + base64.StdEncoding.EncodeToString(minimalWebP),
		"mock":             true,
		"measure_baseline": false,
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/upload", bytes.NewReader(upPayload)))
	if rec.Code != http.StatusOK {
		t.Fatalf("webp /api/upload status = %d body=%s", rec.Code, rec.Body.String())
	}

	// 2. LatencyMS must reflect ONLY the vision call, NOT the optional measureBaseline text probe.
	slowBaseSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Messages []struct {
				Content []struct{ Type string } `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&b)
		hasImg := false
		if len(b.Messages) > 0 {
			for _, p := range b.Messages[0].Content {
				if p.Type == "image" {
					hasImg = true
				}
			}
		}
		if !hasImg {
			// Slow text-only baseline call (150ms) should NOT inflate vision LatencyMS.
			time.Sleep(150 * time.Millisecond)
		}
		inTok := 400
		if hasImg {
			inTok = 720
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "m", "type": "message", "role": "assistant", "model": "claude-haiku-5-5",
			"stop_reason": "end_turn",
			"content":     []any{map[string]any{"type": "text", "text": "❤"}},
			"usage":       map[string]any{"input_tokens": inTok, "output_tokens": 5},
		})
	}))
	defer slowBaseSrv.Close()
	fixtures := buildFixtures()
	vRes := evaluateFixture(context.Background(), ProviderConfig{
		AnthropicKey: "k", AnthropicModel: "claude-haiku-5-5", AnthropicEndpoint: slowBaseSrv.URL,
	}, "claude", 384, fixtures[0], true, true)
	if vRes.ImageTokenOverhead != 320 {
		t.Fatalf("ImageTokenOverhead = %d, want 320", vRes.ImageTokenOverhead)
	}
	if vRes.LatencyMS >= 120 {
		t.Fatalf("LatencyMS = %d ms, appears to include 150ms baseline probe!", vRes.LatencyMS)
	}

	// 3. If Featherless returns HTTP 400 on BOTH image_url AND text-only retry (e.g. broken model),
	// DegradedFromHTTP400 must be false, Mode must be vision_and_fallback_failed, and secret must not leak.
	secret := "rc_SECRET_ECHO_400"
	always400 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"type":"invalid_request_error","message":"bad model for key ` + secret + `"}}`))
	}))
	defer always400.Close()
	failRes := evaluateFixture(context.Background(), ProviderConfig{
		FeatherlessKey: secret, FeatherlessModel: "Qwen/Broken", FeatherlessEndpoint: always400.URL,
	}, "featherless_vision", 384, fixtures[5], false, true)
	if failRes.DegradedFromHTTP400 || failRes.Mode != "vision_and_fallback_failed" {
		t.Fatalf("double-400 result = %+v, want DegradedFromHTTP400=false Mode=vision_and_fallback_failed", failRes)
	}
	if failRes.DegradeTrigger == "" || strings.Contains(failRes.DegradeTrigger, secret) || strings.Contains(failRes.ErrorClass, secret) {
		t.Fatalf("bad trigger/redaction: trigger=%q err=%q", failRes.DegradeTrigger, failRes.ErrorClass)
	}

	// 4. Prose does not leak into Emoji field.
	if st, em := parsePlainReaction("This meme is hilarious 🔥"); st != "parse_error" || em != "" {
		t.Fatalf("prose parse = (%q, %q), want (parse_error, \"\")", st, em)
	}
}
