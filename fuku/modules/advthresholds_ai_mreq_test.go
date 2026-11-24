//go:build testtools

package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/fuku_robot/fuku/config"
	"github.com/uasneppy/fuku_robot/fuku/db/advthresholds"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/db/raiddetection"
	"github.com/uasneppy/fuku_robot/fuku/utils/cache"
	"github.com/uasneppy/fuku_robot/fuku/utils/namecanon"
)

type advRoundTripFunc func(*http.Request) (*http.Response, error)

func (f advRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func advHTTPResponse(status int, contentType string, body []byte) *http.Response {
	hdr := make(http.Header)
	if contentType != "" {
		hdr.Set("Content-Type", contentType)
	}
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     hdr,
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
}

func TestAdvancedThresholdsAIAndMemberReq(t *testing.T) {
	t.Run("raid_ai_image_bar_cached_and_fresh", func(t *testing.T) {
		phase28InProcessMiniredis(t)
		ctx := context.Background()

		var providerCalls atomic.Int64
		prevEnable := config.AppConfig.EnableAISpam
		prevKey := config.AppConfig.AnthropicAPIKey
		prevModel := config.AppConfig.AnthropicModel
		prevClaudeHTTP := raidAIClaudeHTTP
		prevFileHTTP := raidAIFileHTTPClient

		config.AppConfig.EnableAISpam = true
		config.AppConfig.AnthropicAPIKey = "phase28-test-anthropic-key"
		config.AppConfig.AnthropicModel = raidAITestModel
		raidAIClaudeHTTP = &http.Client{
			Transport: advRoundTripFunc(func(_ *http.Request) (*http.Response, error) {
				providerCalls.Add(1)
				return advHTTPResponse(http.StatusOK, "application/json", []byte(raidAIClaudeAnswer(0.75))), nil
			}),
		}
		raidAIFileHTTPClient = &http.Client{
			Transport: advRoundTripFunc(func(_ *http.Request) (*http.Response, error) {
				return advHTTPResponse(http.StatusOK, "image/jpeg", raidAITestJPEG("p28-fresh")), nil
			}),
		}
		t.Cleanup(func() {
			config.AppConfig.EnableAISpam = prevEnable
			config.AppConfig.AnthropicAPIKey = prevKey
			config.AppConfig.AnthropicModel = prevModel
			raidAIClaudeHTTP = prevClaudeHTTP
			raidAIFileHTTPClient = prevFileHTTP
		})

		chatA := uniqueModuleChatID()
		chatB := uniqueModuleChatID()
		chatC := uniqueModuleChatID()
		cleanupAdvThresholdsChat(t, chatA)
		cleanupAdvThresholdsChat(t, chatB)
		cleanupAdvThresholdsChat(t, chatC)

		for _, cid := range []int64{chatA, chatB, chatC} {
			if err := raiddetection.SetAIEnabled(cid, true); err != nil {
				t.Fatalf("SetAIEnabled(%d): %v", cid, err)
			}
		}
		if err := advthresholds.SetField(ctx, chatA, advthresholds.FieldRaidAIImageBar, 70, 1); err != nil {
			t.Fatalf("SetField(chatA, rib=70): %v", err)
		}
		if err := advthresholds.SetField(ctx, chatB, advthresholds.FieldRaidAIImageBar, 90, 1); err != nil {
			t.Fatalf("SetField(chatB, rib=90): %v", err)
		}
		// chatC stays unconfigured (0 -> compiled default 0.85).

		botClient := newModuleBotClient()
		botClient.responses["getFile"] = json.RawMessage(`{"file_id":"f-fresh-075","file_unique_id":"p28-fresh-075","file_size":32,"file_path":"photos/p28-fresh.jpg"}`)
		bot := newModuleTestBot(botClient)

		client := cache.GetRedisClient()
		spamBefore := raidAISpam.Load()
		keptBefore := raidAIKept.Load()

		// 1. Fresh Anthropic answer (0.75) on Chat A (bar 0.70): counts as spam and records verdict.
		raidAIProcessImage(ctx, raidAIJob{
			bot:       bot,
			chat:      gotgbot.Chat{Id: chatA, Type: "supergroup", Title: "Chat A 70"},
			userID:    5001,
			messageID: 101,
			itemKey:   "m:101",
			media:     &raidAIMedia{FileID: "f-fresh-075", FileUniqueID: "p28-fresh-075", Kind: "photo"},
		})
		if got := providerCalls.Load(); got != 1 {
			t.Fatalf("provider calls after fresh image = %d, want 1", got)
		}
		if gotSpam, gotKept := raidAISpam.Load()-spamBefore, raidAIKept.Load()-keptBefore; gotSpam != 1 || gotKept != 0 {
			t.Fatalf("fresh 0.75 on bar 0.70 deltas = (spam=%d, kept=%d), want (1, 0)", gotSpam, gotKept)
		}
		if n, err := client.ZCard(ctx, raidKey(chatA, raidPartAIVerdicts)).Result(); err != nil || n != 1 {
			t.Fatalf("chatA raid AI items ZCard = (%d, %v), want (1, nil)", n, err)
		}

		// 2. Same image now cached at 0.75 in fuku:raid:aiv:p28-fresh-075:
		// Chat B (bar 0.90) keeps it; Chat C (default 0.85) keeps it; 0 extra provider calls.
		raidAIProcessImage(ctx, raidAIJob{
			bot:       bot,
			chat:      gotgbot.Chat{Id: chatB, Type: "supergroup", Title: "Chat B 90"},
			userID:    5002,
			messageID: 102,
			itemKey:   "m:102",
			media:     &raidAIMedia{FileID: "f-fresh-075", FileUniqueID: "p28-fresh-075", Kind: "photo"},
		})
		raidAIProcessImage(ctx, raidAIJob{
			bot:       bot,
			chat:      gotgbot.Chat{Id: chatC, Type: "supergroup", Title: "Chat C default"},
			userID:    5003,
			messageID: 103,
			itemKey:   "m:103",
			media:     &raidAIMedia{FileID: "f-fresh-075", FileUniqueID: "p28-fresh-075", Kind: "photo"},
		})
		if got := providerCalls.Load(); got != 1 {
			t.Fatalf("provider calls after cached reuse = %d, want 1", got)
		}
		if gotSpam, gotKept := raidAISpam.Load()-spamBefore, raidAIKept.Load()-keptBefore; gotSpam != 1 || gotKept != 2 {
			t.Fatalf("cached 0.75 on bar 0.90 & 0.85 deltas = (spam=%d, kept=%d), want (1, 2)", gotSpam, gotKept)
		}
		for _, cid := range []int64{chatB, chatC} {
			if n, err := client.ZCard(ctx, raidKey(cid, raidPartAIVerdicts)).Result(); err != nil || n != 0 {
				t.Fatalf("chat %d raid AI items ZCard = (%d, %v), want (0, nil)", cid, n, err)
			}
		}

		// 3. Explicitly seeded cached probability 0.75 in fuku:raid:aiv:* acts immediately on Chat A.
		raidAIStoreVerdict(client, "p28-seeded-075", 0.75)
		raidAIProcessImage(ctx, raidAIJob{
			bot:       bot,
			chat:      gotgbot.Chat{Id: chatA, Type: "supergroup", Title: "Chat A 70"},
			userID:    5004,
			messageID: 104,
			itemKey:   "m:104",
			media:     &raidAIMedia{FileID: "f-seeded-075", FileUniqueID: "p28-seeded-075", Kind: "photo"},
		})
		if got := providerCalls.Load(); got != 1 {
			t.Fatalf("provider calls after seeded cache = %d, want 1", got)
		}
		if gotSpam, gotKept := raidAISpam.Load()-spamBefore, raidAIKept.Load()-keptBefore; gotSpam != 2 || gotKept != 2 {
			t.Fatalf("seeded 0.75 on bar 0.70 deltas = (spam=%d, kept=%d), want (2, 2)", gotSpam, gotKept)
		}
		if n, err := client.ZCard(ctx, raidKey(chatA, raidPartAIVerdicts)).Result(); err != nil || n != 2 {
			t.Fatalf("chatA raid AI items ZCard after seeded 0.75 = (%d, %v), want (2, nil)", n, err)
		}
	})

	t.Run("memberreq_ai_bar_cross_chat_singleflight", func(t *testing.T) {
		phase28InProcessMiniredis(t)
		ctx := context.Background()

		prevEnable := config.AppConfig.EnableAISpam
		prevKey := config.AppConfig.AnthropicAPIKey
		prevModel := config.AppConfig.AnthropicModel
		prevHTTP := memberReqAIHTTP

		var providerCalls atomic.Int64
		entered := make(chan struct{}, 1)
		release := make(chan struct{})
		var releaseOnce sync.Once
		unlock := func() { releaseOnce.Do(func() { close(release) }) }

		config.AppConfig.EnableAISpam = true
		config.AppConfig.AnthropicAPIKey = "phase28-test-anthropic-key"
		config.AppConfig.AnthropicModel = raidAITestModel
		memberReqAIHTTP = &http.Client{
			Transport: advRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				providerCalls.Add(1)
				select {
				case entered <- struct{}{}:
				default:
				}
				select {
				case <-release:
				case <-req.Context().Done():
					return nil, req.Context().Err()
				}
				return advHTTPResponse(http.StatusOK, "application/json", []byte(raidAIClaudeAnswer(0.80))), nil
			}),
		}
		t.Cleanup(func() {
			unlock()
			config.AppConfig.EnableAISpam = prevEnable
			config.AppConfig.AnthropicAPIKey = prevKey
			config.AppConfig.AnthropicModel = prevModel
			memberReqAIHTTP = prevHTTP
		})

		chatA := uniqueModuleChatID()
		chatB := uniqueModuleChatID()
		cleanupAdvThresholdsChat(t, chatA)
		cleanupAdvThresholdsChat(t, chatB)

		if err := advthresholds.SetField(ctx, chatA, advthresholds.FieldMemberReqAIBar, 70, 1); err != nil {
			t.Fatalf("SetField(chatA, mrb=70): %v", err)
		}
		if err := advthresholds.SetField(ctx, chatB, advthresholds.FieldMemberReqAIBar, 95, 1); err != nil {
			t.Fatalf("SetField(chatB, mrb=95): %v", err)
		}

		names := &memberReqNames{
			First:    fmt.Sprintf("CryptoPromo%d", -chatA),
			Last:     "Signals",
			Username: fmt.Sprintf("cryptosignals_%d", -chatA),
		}
		mode := memberReqLookupMode{Names: names, Joining: true}

		touchA := &memberReqLookupTouch{}
		touchB := &memberReqLookupTouch{}
		ctxA := context.WithValue(ctx, memberReqLookupTouchKey{}, touchA)
		ctxB := context.WithValue(ctx, memberReqLookupTouchKey{}, touchB)

		resA := make(chan memberReqVerdict, 1)
		resB := make(chan memberReqVerdict, 1)

		// Start Chat A (bar 0.70) first so it becomes the singleflight leader for "n:<key>".
		go func() {
			resA <- memberReqAIRule(ctxA, nil, chatA, 6101, mode)
		}()

		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for Chat A leader to enter Anthropic request")
		}

		// Start Chat B (bar 0.95) while Chat A's flight is paused inside the provider call.
		go func() {
			resB <- memberReqAIRule(ctxB, nil, chatB, 6102, mode)
		}()

		deadline := time.Now().Add(5 * time.Second)
		for !touchB.touched.Load() {
			if time.Now().After(deadline) {
				t.Fatal("timed out waiting for Chat B follower to join singleflight")
			}
			time.Sleep(5 * time.Millisecond)
		}
		// Give Chat B a moment after memberReqNoteFlight to enter DoChan and park on the shared flight.
		time.Sleep(25 * time.Millisecond)

		unlock()

		var verdictA, verdictB memberReqVerdict
		select {
		case verdictA = <-resA:
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for Chat A verdict")
		}
		select {
		case verdictB = <-resB:
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for Chat B verdict")
		}

		if got := providerCalls.Load(); got != 1 {
			t.Fatalf("Anthropic provider calls = %d, want 1 (shared cross-chat singleflight)", got)
		}
		if !touchA.touched.Load() || !touchB.touched.Load() {
			t.Fatalf("expected both callers to enter singleflight (touchA=%v, touchB=%v)", touchA.touched.Load(), touchB.touched.Load())
		}
		if verdictA != memberReqBreach {
			t.Fatalf("Chat A (bar 0.70, p=0.80) verdict = %v, want memberReqBreach", verdictA)
		}
		if verdictB != memberReqPass {
			t.Fatalf("Chat B (bar 0.95, p=0.80) verdict = %v, want memberReqPass (must not inherit Chat A breach)", verdictB)
		}
	})

	t.Run("memberreq_ai_watcher_fast_path_respects_lowered_bar", func(t *testing.T) {
		phase28InProcessMiniredis(t)
		ctx := context.Background()

		prevEnable := config.AppConfig.EnableAISpam
		prevKey := config.AppConfig.AnthropicAPIKey
		prevModel := config.AppConfig.AnthropicModel
		config.AppConfig.EnableAISpam = true
		config.AppConfig.AnthropicAPIKey = "phase28-test-anthropic-key"
		config.AppConfig.AnthropicModel = raidAITestModel
		t.Cleanup(func() {
			config.AppConfig.EnableAISpam = prevEnable
			config.AppConfig.AnthropicAPIKey = prevKey
			config.AppConfig.AnthropicModel = prevModel
		})

		chatLowered := uniqueModuleChatID()
		chatDefault := uniqueModuleChatID()
		cleanupAdvThresholdsChat(t, chatLowered)
		cleanupAdvThresholdsChat(t, chatDefault)

		if err := advthresholds.SetField(ctx, chatLowered, advthresholds.FieldMemberReqAIBar, 70, 1); err != nil {
			t.Fatalf("SetField(chatLowered, mrb=70): %v", err)
		}

		names := memberReqNames{
			First:    fmt.Sprintf("WatchPromo%d", -chatLowered),
			Last:     "Deals",
			Username: fmt.Sprintf("watchdeals_%d", -chatLowered),
		}
		canon, ok := namecanon.Of(names.First, names.Last, names.Username)
		if !ok {
			t.Fatal("namecanon.Of failed for judgeable test name")
		}
		verdictKey := memberReqAIVerdictKey(config.AppConfig.AnthropicModel, canon)
		client := cache.GetRedisClient()
		if err := client.Set(ctx, verdictKey, "0.80", time.Hour).Err(); err != nil {
			t.Fatalf("seed cached probability 0.80: %v", err)
		}

		settingsAIOn := &models.ChatRequirements{
			ChatID:   chatLowered,
			AIAction: models.RequirementActionWarn,
		}
		settingsAIOff := &models.ChatRequirements{
			ChatID:      chatLowered,
			PhotoAction: models.RequirementActionWarn,
			AIAction:    models.RequirementActionOff,
		}

		// Lowered bar (0.70): cached 0.80 is NOT below 0.70, so watcher must NOT fast-path skip.
		if satisfied, _ := memberReqLookupCachedWith(settingsAIOn, chatLowered, 7001, "pass", names, false); satisfied {
			t.Fatal("memberReqLookupCachedWith(bar=0.70, cached=0.80) satisfied = true, want false")
		}

		// Default bar (0.90): cached 0.80 IS below 0.90, so watcher fast-path skips.
		settingsDefaultAIOn := &models.ChatRequirements{
			ChatID:   chatDefault,
			AIAction: models.RequirementActionWarn,
		}
		if satisfied, _ := memberReqLookupCachedWith(settingsDefaultAIOn, chatDefault, 7001, "pass", names, false); !satisfied {
			t.Fatal("memberReqLookupCachedWith(bar=default 0.90, cached=0.80) satisfied = false, want true")
		}

		// When rules.AI is false (either AI rule Off or ignoreAI == true), advThresholdsLookup must be called 0 times.
		var lookupCalls atomic.Int64
		origLookup := advThresholdsLookup
		advThresholdsLookup = func(c context.Context, cid int64) (advthresholds.Thresholds, error) {
			lookupCalls.Add(1)
			return origLookup(c, cid)
		}
		t.Cleanup(func() { advThresholdsLookup = origLookup })

		_, _ = memberReqLookupCachedWith(settingsAIOff, chatLowered, 7001, "pass", names, false)
		_, _ = memberReqLookupCachedWith(settingsAIOn, chatLowered, 7001, "pass", names, true)
		if got := lookupCalls.Load(); got != 0 {
			t.Fatalf("advThresholdsLookup calls when rules.AI is false = %d, want 0", got)
		}

		// Sanity check: calling with rules.AI == true performs exactly 1 lookup.
		_, _ = memberReqLookupCachedWith(settingsAIOn, chatLowered, 7001, "pass", names, false)
		if got := lookupCalls.Load(); got != 1 {
			t.Fatalf("advThresholdsLookup calls when rules.AI is true = %d, want 1", got)
		}
	})

	t.Run("memberreq_ai_baseline_isolated_per_chat", func(t *testing.T) {
		mr := phase28InProcessMiniredis(t)
		ctx := context.Background()

		mreqSwap(t, &config.AppConfig.EnableAISpam, true)
		mreqSwap(t, &config.AppConfig.AnthropicAPIKey, "test-key")
		mreqSwap(t, &config.AppConfig.AnthropicModel, "test-model")

		chatPermissive := uniqueModuleChatID()
		chatStrict := uniqueModuleChatID()
		cleanupAdvThresholdsChat(t, chatPermissive)
		cleanupAdvThresholdsChat(t, chatStrict)
		if err := advthresholds.SetField(ctx, chatPermissive, advthresholds.FieldMemberReqAIBar, 95, 1); err != nil {
			t.Fatalf("SetField(permissive bar=95): %v", err)
		}
		if err := advthresholds.SetField(ctx, chatStrict, advthresholds.FieldMemberReqAIBar, 60, 1); err != nil {
			t.Fatalf("SetField(strict bar=60): %v", err)
		}

		names := memberReqNames{First: "Crypto", Last: "Signals", Username: "wr04_iso_user"}
		canon, ok := namecanon.Of(names.First, names.Last, names.Username)
		if !ok {
			t.Fatal("namecanon.Of() = false")
		}
		verdictKey := memberReqAIVerdictKey(config.AppConfig.AnthropicModel, canon)
		if err := mr.Set(verdictKey, "0.80"); err != nil {
			t.Fatalf("mr.Set(verdict): %v", err)
		}

		const userID int64 = 7501
		// Pass in permissive chat (0.80 < 0.95) records Chat A's baseline only.
		if got := memberReqAIRule(ctx, nil, chatPermissive, userID, memberReqLookupMode{Names: &names}); got != memberReqPass {
			t.Fatalf("memberReqAIRule(permissive=95, prob=0.80) = %v, want memberReqPass", got)
		}
		if !mr.Exists(memberReqAIBaselineKey(chatPermissive, userID)) {
			t.Fatal("expected baseline key in permissive chat")
		}
		if mr.Exists(memberReqAIBaselineKey(chatStrict, userID)) {
			t.Fatal("baseline key leaked into strict chat")
		}

		// Strict chat (0.80 >= 0.60) must NOT be satisfied by permissive chat's baseline.
		settingsStrict := &models.ChatRequirements{
			ChatID:   chatStrict,
			AIAction: models.RequirementActionWarn,
		}
		if satisfied, _ := memberReqLookupCachedWith(settingsStrict, chatStrict, userID, "pass", names, false); satisfied {
			t.Fatal("memberReqLookupCachedWith(strict=60) satisfied = true, want false")
		}
		if got := memberReqAIRule(ctx, nil, chatStrict, userID, memberReqLookupMode{Names: &names}); got != memberReqBreach {
			t.Fatalf("memberReqAIRule(strict=60, prob=0.80) = %v, want memberReqBreach", got)
		}
	})

	t.Run("memberreq_cap_and_pause_custom", func(t *testing.T) {
		mr := phase28InProcessMiniredis(t)
		withStaffLocale(t)
		ctx := context.Background()

		base := time.Date(2026, 11, 23, 15, 0, 10, 0, time.UTC)
		phase28SwapRaidNow(t, base)

		chatID := uniqueModuleChatID()
		cleanupAdvThresholdsChat(t, chatID)
		if err := advthresholds.SetField(ctx, chatID, advthresholds.FieldMemberReqCap, 3, 1); err != nil {
			t.Fatalf("SetField(mrc=3): %v", err)
		}
		if err := advthresholds.SetField(ctx, chatID, advthresholds.FieldMemberReqPause, 20, 1); err != nil {
			t.Fatalf("SetField(mrp=20): %v", err)
		}

		botClient := newModuleBotClient()
		bot := newModuleTestBot(botClient)

		for i := 0; i < 3; i++ {
			if !memberReqCapAllow(ctx, bot, chatID, int64(8000+i)) {
				t.Fatalf("memberReqCapAllow(user %d) = false, want true within custom cap 3/min", i+1)
			}
			if mr.Exists(memberReqPausedKey(chatID)) {
				t.Fatalf("pause latch set prematurely after %d users", i+1)
			}
		}

		if memberReqCapAllow(ctx, bot, chatID, 8003) {
			t.Fatal("memberReqCapAllow(4th user) = true, want false over custom cap 3/min")
		}
		if !mr.Exists(memberReqPausedKey(chatID)) {
			t.Fatal("pause latch not set after 4th user")
		}
		if ttl := mr.TTL(memberReqPausedKey(chatID)); ttl != 20*time.Minute {
			t.Fatalf("pause latch TTL = %s, want 20m0s", ttl)
		}

		sent := botClient.callsFor("sendMessage")
		if len(sent) != 1 {
			t.Fatalf("pause notice sendMessage calls = %d, want 1", len(sent))
		}
		text := fmt.Sprint(sent[0].Params["text"])
		wantFields := []string{staffMarker("mreq_notice_paused"), "20", "3"}
		if got := strings.Join(strings.Fields(text), " "); got != strings.Join(wantFields, " ") {
			t.Fatalf("pause notice = %q, want %q (minutes=20, n=3)", got, strings.Join(wantFields, " "))
		}
	})

	t.Run("memberreq_cap_and_pause_preserves_package_var_seams_when_unset", func(t *testing.T) {
		mr := phase28InProcessMiniredis(t)
		withStaffLocale(t)
		ctx := context.Background()

		base := time.Date(2026, 11, 23, 15, 5, 10, 0, time.UTC)
		phase28SwapRaidNow(t, base)

		prevCap := memberReqCapPerMinute
		prevPause := memberReqPause
		memberReqCapPerMinute = 2
		memberReqPause = 3 * time.Minute
		t.Cleanup(func() {
			memberReqCapPerMinute = prevCap
			memberReqPause = prevPause
		})

		chatID := uniqueModuleChatID()
		cleanupAdvThresholdsChat(t, chatID)

		botClient := newModuleBotClient()
		bot := newModuleTestBot(botClient)

		for i := 0; i < 2; i++ {
			if !memberReqCapAllow(ctx, bot, chatID, int64(8100+i)) {
				t.Fatalf("memberReqCapAllow(user %d) = false, want true within seam cap 2/min", i+1)
			}
			if mr.Exists(memberReqPausedKey(chatID)) {
				t.Fatalf("pause latch set prematurely after %d users", i+1)
			}
		}

		if memberReqCapAllow(ctx, bot, chatID, 8102) {
			t.Fatal("memberReqCapAllow(3rd user) = true, want false over seam cap 2/min")
		}
		if !mr.Exists(memberReqPausedKey(chatID)) {
			t.Fatal("pause latch not set after 3rd user")
		}
		if ttl := mr.TTL(memberReqPausedKey(chatID)); ttl != 3*time.Minute {
			t.Fatalf("pause latch TTL = %s, want 3m0s (from package-var seam)", ttl)
		}

		sent := botClient.callsFor("sendMessage")
		if len(sent) != 1 {
			t.Fatalf("pause notice sendMessage calls = %d, want 1", len(sent))
		}
		text := fmt.Sprint(sent[0].Params["text"])
		wantFields := []string{staffMarker("mreq_notice_paused"), "3", "2"}
		if got := strings.Join(strings.Fields(text), " "); got != strings.Join(wantFields, " ") {
			t.Fatalf("pause notice = %q, want %q (minutes=3, n=2)", got, strings.Join(wantFields, " "))
		}
	})
}
