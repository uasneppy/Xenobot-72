//go:build testtools

package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/config"
	"github.com/uasneppy/Xenobot-72/xenobot/db/memberreq"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/namecanon"
)

const (
	featherlessModTestKey   = "rc_FAKEMODERATIONTESTKEY0123456789abcdefgh"
	featherlessModTestModel = "google/gemma-3-27b-it"
)

func featherlessVerdictAnswer(probability float64) string {
	body, _ := json.Marshal(map[string]float64{"spam_probability": probability})
	raw, _ := json.Marshal(map[string]any{
		"model": featherlessModTestModel,
		"choices": []any{map[string]any{
			"index":         0,
			"message":       map[string]any{"role": "assistant", "content": string(body)},
			"finish_reason": "stop",
		}},
		"usage": map[string]any{"prompt_tokens": 200, "completion_tokens": 12},
	})
	return string(raw)
}

func featherlessRefusalAnswer() string {
	return `{"model":"` + featherlessModTestModel + `","choices":[{"index":0,"message":{"role":"assistant","content":""},"finish_reason":"content_filter"}]}`
}

func TestModerationAIProviderSelection(t *testing.T) {
	cases := []struct {
		name      string
		enabled   bool
		anthropic string
		flKey     string
		flModel   string
		choice    string
		want      string
	}{
		{"none configured", true, "", "", "", "", ""},
		{"anthropic only", true, raidAITestKey, "", "", "", "anthropic"},
		{"featherless key without model stays inert", true, "", featherlessModTestKey, "", "", ""},
		{"featherless key and model auto-selected", true, "", featherlessModTestKey, featherlessModTestModel, "", "featherless"},
		{"both configured defaults to anthropic", true, raidAITestKey, featherlessModTestKey, featherlessModTestModel, "", "anthropic"},
		{"explicit featherless wins when both configured", true, raidAITestKey, featherlessModTestKey, featherlessModTestModel, "featherless", "featherless"},
		{"explicit anthropic with no anthropic key is inert", true, "", featherlessModTestKey, featherlessModTestModel, "anthropic", ""},
		{"explicit featherless without model is inert", true, raidAITestKey, featherlessModTestKey, "", "featherless", ""},
		{"ENABLE_AISPAM=false disables featherless", false, "", featherlessModTestKey, featherlessModTestModel, "featherless", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.AppConfig
			mreqSwap(t, &cfg.EnableAISpam, tc.enabled)
			mreqSwap(t, &cfg.AnthropicAPIKey, tc.anthropic)
			mreqSwap(t, &cfg.FeatherlessAPIKey, tc.flKey)
			mreqSwap(t, &cfg.FeatherlessModel, tc.flModel)
			mreqSwap(t, &cfg.AIModerationProvider, tc.choice)

			if got := moderationAIProvider(); got != tc.want {
				t.Errorf("moderationAIProvider() = %q, want %q", got, tc.want)
			}
			wantConfigured := tc.want != ""
			if raidAIImageConfigured() != wantConfigured || memberReqAIConfigured() != wantConfigured {
				t.Errorf("configured = raid:%v mreq:%v, want %v", raidAIImageConfigured(), memberReqAIConfigured(), wantConfigured)
			}
		})
	}
}

func TestRaidAIFeatherless(t *testing.T) {
	t.Run("photo, sticker and image document judged through Featherless AI", func(t *testing.T) {
		env := newRaidAIImageEnv(t)
		cfg := config.AppConfig
		mreqSwap(t, &cfg.AnthropicAPIKey, "")
		mreqSwap(t, &cfg.FeatherlessAPIKey, featherlessModTestKey)
		mreqSwap(t, &cfg.FeatherlessModel, featherlessModTestModel)
		mreqSwap(t, &cfg.AIModerationProvider, "")

		var mu sync.Mutex
		var bodies [][]byte
		var authHeaders []string
		flServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			mu.Lock()
			bodies = append(bodies, b)
			authHeaders = append(authHeaders, r.Header.Get("Authorization"))
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, featherlessVerdictAnswer(0.93))
		}))
		t.Cleanup(flServer.Close)
		mreqSwap(t, &raidAIFeatherlessEndpoint, flServer.URL)
		mreqSwap(t, &raidAIFeatherlessHTTP, flServer.Client())

		env.serve("files/stkfl", raidAIServed{status: http.StatusOK, body: raidAITestWebP("fl-sticker")})
		env.serve("files/docfl", raidAIServed{status: http.StatusOK, body: raidAITestJPEG("fl-doc")})

		members := env.newMembers(3, 2*time.Hour)
		env.sendMedia(members[0], gotgbot.Message{Photo: photoNamed("flphoto")})
		env.settleAI()
		env.sendMedia(members[1], gotgbot.Message{Sticker: &gotgbot.Sticker{FileId: "stkfl", FileUniqueId: "ustkfl", Type: "regular"}})
		env.settleAI()
		env.sendMedia(members[2], gotgbot.Message{Document: &gotgbot.Document{FileId: "docfl", FileUniqueId: "udocfl", FileName: "ad.jpg", MimeType: "image/jpeg"}})
		env.settleAI()

		if got := env.claudeRequests(); got != 0 {
			t.Fatalf("Anthropic received %d requests, want 0", got)
		}
		mu.Lock()
		defer mu.Unlock()
		if len(bodies) != 3 {
			t.Fatalf("Featherless received %d requests, want 3", len(bodies))
		}
		for i, auth := range authHeaders {
			if auth != "Bearer "+featherlessModTestKey {
				t.Errorf("request %d auth = %q", i, auth)
			}
		}
		if !bytes.Contains(bodies[0], []byte("data:image/jpeg;base64,")) ||
			!bytes.Contains(bodies[1], []byte("data:image/webp;base64,")) ||
			!bytes.Contains(bodies[2], []byte("data:image/jpeg;base64,")) {
			t.Error("Featherless image requests missing expected data URLs")
		}
		if got := aiVerdictSetSize(env.lockdownEnv, "av"); got != 3 {
			t.Errorf("verdict items = %d, want 3", got)
		}
	})

	t.Run("content_filter counts as Blocked and 401 pauses breaker without logging key", func(t *testing.T) {
		env := newRaidAIImageEnv(t)
		cfg := config.AppConfig
		mreqSwap(t, &cfg.AnthropicAPIKey, "")
		mreqSwap(t, &cfg.FeatherlessAPIKey, featherlessModTestKey)
		mreqSwap(t, &cfg.FeatherlessModel, featherlessModTestModel)

		var calls atomic.Int64
		flServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if calls.Add(1) == 1 {
				_, _ = io.WriteString(w, featherlessRefusalAnswer())
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"error":{"type":"authentication_error","message":"bad key `+featherlessModTestKey+`"}}`)
		}))
		t.Cleanup(flServer.Close)
		mreqSwap(t, &raidAIFeatherlessEndpoint, flServer.URL)
		mreqSwap(t, &raidAIFeatherlessHTTP, flServer.Client())

		out := &lockedBuffer{}
		hook := captureLogs(t, log.DebugLevel)
		log.SetOutput(out)
		before := GetRaidAIStats()

		member := env.newMembers(1, 2*time.Hour)[0]
		env.sendMedia(member, gotgbot.Message{Photo: photoNamed("fl-refuse")})
		env.settleAI()
		env.sendMedia(member, gotgbot.Message{Photo: photoNamed("fl-unauth")})
		env.settleAI()
		env.sendMedia(member, gotgbot.Message{Photo: photoNamed("fl-paused")})
		env.settleAI()

		got := GetRaidAIStats()
		if got.Blocked-before.Blocked != 1 {
			t.Errorf("Blocked grew by %d, want 1", got.Blocked-before.Blocked)
		}
		if got.Failed-before.Failed != 1 {
			t.Errorf("Failed grew by %d, want 1", got.Failed-before.Failed)
		}
		if calls.Load() != 2 {
			t.Errorf("Featherless calls = %d, want 2 (third paused by breaker)", calls.Load())
		}
		if strings.Contains(everythingLogged(out, hook), featherlessModTestKey) {
			t.Fatal("Featherless API key leaked into raid AI logs")
		}
	})
}

func TestMemberReqAIFeatherless(t *testing.T) {
	env, _ := mreqAIEnvOf(t, map[memberreq.Rule]int{memberreq.RuleAI: models.RequirementActionWarn}, true)
	cfg := config.AppConfig
	mreqSwap(t, &cfg.AnthropicAPIKey, "")
	mreqSwap(t, &cfg.FeatherlessAPIKey, featherlessModTestKey)
	mreqSwap(t, &cfg.FeatherlessModel, featherlessModTestModel)
	mreqSwap(t, &cfg.AIModerationProvider, "featherless")

	var mu sync.Mutex
	var bodies []string
	var nextAnswer atomic.Value
	nextAnswer.Store(featherlessVerdictAnswer(0.95))

	flServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(raw))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, nextAnswer.Load().(string))
	}))
	t.Cleanup(flServer.Close)
	mreqSwap(t, &memberReqFeatherlessEndpoint, flServer.URL)
	mreqSwap(t, &memberReqFeatherlessHTTP, flServer.Client())

	readBodies := func() []string {
		mu.Lock()
		defer mu.Unlock()
		out := make([]string, len(bodies))
		copy(out, bodies)
		return out
	}

	// 1. Spam name is punished.
	spammer := mreqAIMember(env, "Crypto Bonus 24/7")
	env.mreqJoin(spammer)
	gotBodies := readBodies()
	if got := len(gotBodies); got != 1 {
		t.Fatalf("Featherless calls = %d, want 1", got)
	}
	if !strings.Contains(gotBodies[0], "Crypto Bonus 24/7") {
		t.Errorf("request body missing member name: %s", gotBodies[0])
	}
	if got := mreqCountText(env.mreqSentTexts(), staffMarker("mreq_notice_warn")); got != 1 {
		t.Fatalf("warn notices = %d, want 1 (texts %v)", got, env.mreqSentTexts())
	}
	if row := env.mreqWarnRow(spammer.Id); row == nil || row.NumWarns != 1 {
		t.Fatalf("warns row = %+v, want 1 warn", row)
	}

	// 2. Ordinary name passes and writes baseline.
	welcomesBefore := mreqAIWelcomes(env)
	nextAnswer.Store(featherlessVerdictAnswer(0.05))
	clean := mreqAIMember(env, "Olena")
	env.mreqJoin(clean)
	if got := len(readBodies()); got != 2 {
		t.Fatalf("Featherless calls = %d, want 2", got)
	}
	if got := mreqAIWelcomes(env) - welcomesBefore; got != 1 {
		t.Errorf("new welcomes = %d, want 1 for ordinary name", got)
	}
	if row := env.mreqWarnRow(clean.Id); row != nil && row.NumWarns > 0 {
		t.Errorf("ordinary name was warned: %+v", row)
	}

	// 3. Content filter refusal counts as blocked and punishes nobody.
	nextAnswer.Store(featherlessRefusalAnswer())
	beforeBlocked := memberReqAIStats.blocked.Load()
	welcomesBefore = mreqAIWelcomes(env)
	blockedMember := mreqAIMember(env, "RefusedName")
	env.mreqJoin(blockedMember)
	if row := env.mreqWarnRow(blockedMember.Id); row != nil && row.NumWarns > 0 {
		t.Fatalf("refused member was warned: %+v", row)
	}
	if got := mreqAIWelcomes(env) - welcomesBefore; got != 1 {
		t.Errorf("new welcomes = %d, want 1 after refused AI check", got)
	}
	if memberReqAIStats.blocked.Load()-beforeBlocked != 1 {
		t.Errorf("blocked counter did not increment")
	}
}

func TestMemberReqAIFeatherlessWatcherCacheKey(t *testing.T) {
	env, _ := mreqAIEnvOf(t, map[memberreq.Rule]int{memberreq.RuleAI: models.RequirementActionWarn}, true)
	cfg := config.AppConfig
	mreqSwap(t, &cfg.AnthropicAPIKey, raidAITestKey)
	mreqSwap(t, &cfg.AnthropicModel, raidAITestModel)
	mreqSwap(t, &cfg.FeatherlessAPIKey, featherlessModTestKey)
	mreqSwap(t, &cfg.FeatherlessModel, featherlessModTestModel)
	mreqSwap(t, &cfg.AIModerationProvider, "featherless")

	ctx := context.Background()
	redisClient := cache.GetRedisClient()
	names := memberReqNames{First: "Crypto", Last: "Bonus", Username: "cryptobonus_watch"}
	canon, ok := namecanon.Of(names.First, names.Last, names.Username)
	if !ok {
		t.Fatal("namecanon.Of returned false for judgeable test name")
	}

	// Plant a stale Anthropic pass verdict only: under Featherless, the watcher must ignore it.
	anthropicKey := memberReqAIVerdictKey(cfg.AnthropicModel, canon)
	featherlessKey := memberReqAIVerdictKey(cfg.FeatherlessModel, canon)
	if anthropicKey == featherlessKey {
		t.Fatal("Anthropic and Featherless verdict keys collided")
	}
	if err := redisClient.Set(ctx, anthropicKey, "0.05", time.Hour).Err(); err != nil {
		t.Fatalf("seed Anthropic verdict: %v", err)
	}

	readWatch := func() memberReqAIWatchRead {
		pipe := redisClient.Pipeline()
		read := memberReqAIWatchQueue(ctx, pipe, env.chat.Id, 9001, names)
		_, _ = pipe.Exec(ctx)
		return read
	}
	if readWatch().satisfied() {
		t.Fatal("memberReqAIWatchQueue satisfied = true from stale Anthropic verdict under Featherless")
	}

	settings := &models.ChatRequirements{ChatID: env.chat.Id, AIAction: models.RequirementActionWarn}
	passVal := memberReqPassValue(settings, names)
	if err := redisClient.Set(ctx, memberReqPassKey(env.chat.Id, 9001), passVal, time.Hour).Err(); err != nil {
		t.Fatalf("seed pass key: %v", err)
	}
	if satisfied, _ := memberReqLookupCachedWith(settings, env.chat.Id, 9001, passVal, names, false); satisfied {
		t.Fatal("memberReqLookupCachedWith satisfied = true from stale Anthropic verdict under Featherless")
	}

	// Once Featherless's own pass verdict is cached, the watcher fast path must hit.
	if err := redisClient.Set(ctx, featherlessKey, "0.05", time.Hour).Err(); err != nil {
		t.Fatalf("seed Featherless verdict: %v", err)
	}
	if !readWatch().satisfied() {
		t.Fatal("memberReqAIWatchQueue satisfied = false with Featherless pass verdict cached")
	}
	if satisfied, _ := memberReqLookupCachedWith(settings, env.chat.Id, 9001, passVal, names, false); !satisfied {
		t.Fatal("memberReqLookupCachedWith satisfied = false with Featherless pass verdict cached")
	}
}
