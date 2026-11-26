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
