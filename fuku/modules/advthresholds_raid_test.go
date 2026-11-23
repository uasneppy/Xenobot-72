//go:build testtools

package modules

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/advthresholds"
	dbcache "github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/lockdown"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/db/raiddetection"
	"github.com/uasneppy/fuku_robot/fuku/utils/cache"
)

func cleanupAdvThresholdsChat(t *testing.T, chatID int64) {
	t.Helper()
	phase28CleanupRaidChat(t, chatID)
	t.Cleanup(func() {
		if db.DB != nil {
			_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatAdvancedThresholds{}).Error
		}
		dbcache.DeleteCache(advthresholds.CacheKey(chatID))
	})
}

// TestAdvancedThresholdDefaultsMatchConstants is the Phase 28 drift guard: it compares every
// default constant in fuku/db/models and fuku/db/advthresholds against the real compiled module
// constant so any future constant edit without updating the advanced thresholds spec fails fast.
func TestAdvancedThresholdDefaultsMatchConstants(t *testing.T) {
	checks := []struct {
		name string
		got  int
		want int
	}{
		{"DefaultRaidJoinWindowSecs", models.DefaultRaidJoinWindowSecs, int(raidJoinWindow / time.Second)},
		{"DefaultRaidFloodWindowSecs", models.DefaultRaidFloodWindowSecs, int(raidFloodWindow / time.Second)},
		{"DefaultRaidAIWindowSecs", models.DefaultRaidAIWindowSecs, int(raidAIWindow / time.Second)},
		{"DefaultRaidFloodNewPercent", models.DefaultRaidFloodNewPercent, raidFloodNewPercent},
		{"DefaultRaidFloodMinAccounts", models.DefaultRaidFloodMinAccounts, raidFloodMinAccounts},
		{"DefaultRaidRecentHours", models.DefaultRaidRecentHours, int(raidRecentWindow / time.Hour)},
		{"DefaultRaidAIImageBarPct", models.DefaultRaidAIImageBarPct, int(math.Round(raidAIImageBar * 100))},
		{"DefaultMemberReqAIBarPct", models.DefaultMemberReqAIBarPct, int(math.Round(memberReqAIBar * 100))},
		{"DefaultMemberReqCapPerMin", models.DefaultMemberReqCapPerMin, memberReqCapPerMinute},
		{"DefaultMemberReqPauseMins", models.DefaultMemberReqPauseMins, int(memberReqPause / time.Minute)},
		{"DefaultLockdownReminderMins", models.DefaultLockdownReminderMins, int(lockdownReminderEvery / time.Minute)},
		{"DefaultCaptchaTimeoutMins", models.DefaultCaptchaTimeoutMins, 0},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %d, want compiled constant %d", c.name, c.got, c.want)
		}
	}

	specs := advthresholds.AllSpecs()
	if len(specs) != 12 {
		t.Fatalf("advthresholds.AllSpecs() len = %d, want 12", len(specs))
	}
	for _, spec := range specs {
		if spec.Min > spec.Max {
			t.Errorf("spec %s has Min=%d > Max=%d", spec.Field, spec.Min, spec.Max)
		}
		if spec.Default != 0 && (spec.Default < spec.Min || spec.Default > spec.Max) {
			t.Errorf("spec %s Default=%d outside [%d, %d]", spec.Field, spec.Default, spec.Min, spec.Max)
		}
	}

	zero := advthresholds.Thresholds{}
	if advRaidJoinWindow(zero) != raidJoinWindow ||
		advRaidFloodWindow(zero) != raidFloodWindow ||
		advRaidAIWindow(zero) != raidAIWindow ||
		advRaidFloodNewPercent(zero) != raidFloodNewPercent ||
		advRaidFloodMinAccounts(zero) != raidFloodMinAccounts ||
		advRaidRecentWindow(zero) != raidRecentWindow ||
		advRaidAIImageBar(zero) != raidAIImageBar ||
		advMemberReqAIBar(zero) != memberReqAIBar ||
		advMemberReqCapPerMin(zero) != memberReqCapPerMinute ||
		advMemberReqPause(zero) != memberReqPause ||
		advLockdownReminderEvery(zero) != lockdownReminderEvery ||
		advCaptchaTimeout(zero, 5) != 5 {
		t.Error("zero Thresholds resolved helpers drifted from compiled defaults")
	}
}

func TestAdvancedThresholdsRaidWiring(t *testing.T) {
	t.Run("join_window_and_recent_ttl", func(t *testing.T) {
		mr := phase28InProcessMiniredis(t)
		base := time.Date(2026, 11, 23, 14, 0, 0, 0, time.UTC)
		phase28SwapRaidNow(t, base)
		ctx := context.Background()

		// 1. Short join surge window (15s) excludes join at now-20s and fires with TriggerWindow == 15.
		chatID := uniqueModuleChatID()
		cleanupAdvThresholdsChat(t, chatID)
		if err := advthresholds.SetField(ctx, chatID, advthresholds.FieldRaidJoinWindow, 15, 1); err != nil {
			t.Fatalf("SetField(jsw=15): %v", err)
		}
		phase28SeedJoinedAt(t, chatID, 1001, base.Add(-20*time.Second))
		phase28SeedJoinedAt(t, chatID, 1002, base.Add(-10*time.Second))

		count, err := recordRaidJoins(chatID, []int64{1003})
		if err != nil {
			t.Fatalf("recordRaidJoins: %v", err)
		}
		if count != 2 {
			t.Fatalf("recordRaidJoins(jsw=15) count = %d, want 2 (1002 at -10s and 1003 at now; 1001 at -20s excluded)", count)
		}

		botClient := phase28NewLockdownBotClient()
		bot := newModuleTestBot(botClient)
		chat := gotgbot.Chat{Id: chatID, Type: "supergroup", Title: "Join Window 15s"}
		raidRecordAndFire(bot, &chat, &models.ChatRaidDetection{
			ChatID:        chatID,
			JoinEnabled:   true,
			JoinThreshold: 5,
		}, []int64{1004, 1005, 1006})
		raidPosts.wait()

		active, err := lockdown.GetActiveFresh(chatID)
		if err != nil {
			t.Fatalf("lockdown.GetActiveFresh: %v", err)
		}
		if active == nil {
			t.Fatal("expected active join-surge lockdown, got nil")
		}
		if active.TriggerKind != models.LockdownTriggerJoinSurge || active.TriggerWindow != 15 || active.TriggerCount != 5 {
			t.Fatalf("join surge lockdown = (kind=%s, window=%d, count=%d), want (join_surge, 15, 5)",
				active.TriggerKind, active.TriggerWindow, active.TriggerCount)
		}

		// 2. Extended recent window (168h) sets joined TTL to 169h and keeps a 48h-old join.
		longChat := uniqueModuleChatID()
		cleanupAdvThresholdsChat(t, longChat)
		if err := advthresholds.SetField(ctx, longChat, advthresholds.FieldRaidRecentHours, 168, 1); err != nil {
			t.Fatalf("SetField(rna=168): %v", err)
		}
		phase28SeedJoinedAt(t, longChat, 2001, base.Add(-48*time.Hour))
		if _, err := recordRaidJoins(longChat, []int64{2002}); err != nil {
			t.Fatalf("recordRaidJoins(rna=168): %v", err)
		}
		client := cache.GetRedisClient()
		longKey := raidKey(longChat, raidPartJoined)
		if _, err := client.ZScore(ctx, longKey, "2001").Result(); err != nil {
			t.Fatalf("48h-old join 2001 trimmed with rna=168: %v", err)
		}
		longTTL := mr.TTL(longKey)
		if longTTL <= 168*time.Hour || longTTL > 169*time.Hour {
			t.Fatalf("joined TTL with rna=168 = %s, want > 168h and <= 169h", longTTL)
		}

		// 3. Short recent window (1h) trims a 2h-old join while keeping a 30m-old join.
		shortChat := uniqueModuleChatID()
		cleanupAdvThresholdsChat(t, shortChat)
		if err := advthresholds.SetField(ctx, shortChat, advthresholds.FieldRaidRecentHours, 1, 1); err != nil {
			t.Fatalf("SetField(rna=1): %v", err)
		}
		phase28SeedJoinedAt(t, shortChat, 3001, base.Add(-2*time.Hour))
		phase28SeedJoinedAt(t, shortChat, 3002, base.Add(-30*time.Minute))
		if _, err := recordRaidJoins(shortChat, []int64{3003}); err != nil {
			t.Fatalf("recordRaidJoins(rna=1): %v", err)
		}
		shortKey := raidKey(shortChat, raidPartJoined)
		if _, err := client.ZScore(ctx, shortKey, "3001").Result(); !errors.Is(err, redis.Nil) {
			t.Fatalf("2h-old join 3001 ZScore err = %v, want redis.Nil (trimmed with rna=1)", err)
		}
		if _, err := client.ZScore(ctx, shortKey, "3002").Result(); err != nil {
			t.Fatalf("30m-old join 3002 unexpectedly trimmed with rna=1: %v", err)
		}
	})

	t.Run("flood_window_share_floor_recent_and_ttl", func(t *testing.T) {
		mr := phase28InProcessMiniredis(t)
		base := time.Date(2026, 11, 23, 14, 0, 0, 0, time.UTC)
		now := phase28SwapRaidNow(t, base)
		ctx := context.Background()

		chatID := uniqueModuleChatID()
		cleanupAdvThresholdsChat(t, chatID)

		if err := raiddetection.SetFloodEnabled(chatID, true); err != nil {
			t.Fatalf("SetFloodEnabled: %v", err)
		}
		if err := raiddetection.SetFloodThreshold(chatID, 10); err != nil {
			t.Fatalf("SetFloodThreshold: %v", err)
		}
		for _, kv := range []struct {
			field advthresholds.Field
			val   int
		}{
			{advthresholds.FieldRaidFloodWindow, 120},
			{advthresholds.FieldRaidFloodNewPercent, 40},
			{advthresholds.FieldRaidFloodMinAccounts, 2},
			{advthresholds.FieldRaidRecentHours, 2},
		} {
			if err := advthresholds.SetField(ctx, chatID, kv.field, kv.val, 1); err != nil {
				t.Fatalf("SetField(%s=%d): %v", kv.field, kv.val, err)
			}
		}

		// 2 joiners inside 2h recent window (1h ago), 1 joiner outside 2h recent window (3h ago).
		phase28SeedJoinedAt(t, chatID, 4001, base.Add(-1*time.Hour))
		phase28SeedJoinedAt(t, chatID, 4002, base.Add(-1*time.Hour))
		phase28SeedJoinedAt(t, chatID, 4003, base.Add(-3*time.Hour))

		var dbReads atomic.Int64
		cbName := fmt.Sprintf("test:advthresholds_flood_reads_%d", chatID)
		if err := db.DB.Callback().Query().After("gorm:query").Register(cbName, func(tx *gorm.DB) {
			if tx.Statement != nil && tx.Statement.Table == "chat_advanced_thresholds" {
				dbReads.Add(1)
			}
		}); err != nil {
			t.Fatalf("register query callback: %v", err)
		}
		t.Cleanup(func() {
			_ = db.DB.Callback().Query().Remove(cbName)
		})

		botClient := phase28NewLockdownBotClient()
		bot := newModuleTestBot(botClient)
		chat := gotgbot.Chat{Id: chatID, Type: "supergroup", Title: "Flood Tuned"}

		// First 5 messages at now-50s (outside default 30s window, inside configured 120s window),
		// remaining 5 messages at now. Total: 10 messages, 4 from 2 new accounts (40%), 6 from 3h-old 4003.
		type msgSpec struct {
			uid int64
			at  time.Time
		}
		specs := []msgSpec{
			{4001, base.Add(-50 * time.Second)},
			{4001, base.Add(-50 * time.Second)},
			{4003, base.Add(-50 * time.Second)},
			{4003, base.Add(-50 * time.Second)},
			{4003, base.Add(-50 * time.Second)},
			{4002, base},
			{4002, base},
			{4003, base},
			{4003, base},
			{4003, base},
		}
		for i, s := range specs {
			*now = s.at
			msgCtx := newModuleMessageContext(bot, chat, gotgbot.User{Id: s.uid, FirstName: "U"}, fmt.Sprintf("flood %d", i))
			msgCtx.EffectiveMessage.MessageId = int64(800 + i)
			if err := raidFloodCountUpdate(bot, msgCtx, false); err == nil {
				t.Fatalf("raidFloodCountUpdate(%d) returned nil, want ContinueGroups", i)
			}
		}
		raidPosts.wait()

		// Warm hot-path check: 1 initial DB read filled Redis cache, remaining 9 messages made 0 DB reads.
		if !mr.Exists(advthresholds.CacheKey(chatID)) {
			t.Fatalf("expected Redis cache key %s to be populated", advthresholds.CacheKey(chatID))
		}
		if got := dbReads.Load(); got != 1 {
			t.Fatalf("chat_advanced_thresholds DB reads across 10 flood messages = %d, want 1 (0 extra DB queries on warm path)", got)
		}

		active, err := lockdown.GetActiveFresh(chatID)
		if err != nil {
			t.Fatalf("lockdown.GetActiveFresh: %v", err)
		}
		if active == nil {
			t.Fatal("expected active flood lockdown with tuned window=120s share=40% floor=2, got nil")
		}
		if active.TriggerKind != models.LockdownTriggerFlood || active.TriggerWindow != 120 || active.TriggerPercent != 40 || active.TriggerCount != 10 {
			t.Fatalf("flood lockdown = (kind=%s, window=%d, percent=%d, count=%d), want (flood, 120, 40, 10)",
				active.TriggerKind, active.TriggerWindow, active.TriggerPercent, active.TriggerCount)
		}

		// Derived flood TTL = max(raidFloodTTL=2m, 120s*2=240s) = 240s.
		for _, part := range []string{raidPartFloodItems, raidPartFloodNew, raidPartFloodAccounts} {
			ttl := mr.TTL(raidKey(chatID, part))
			if ttl <= 2*time.Minute || ttl > 240*time.Second {
				t.Fatalf("flood key %s TTL = %s, want > 2m and <= 240s", part, ttl)
			}
		}

		// Verify 3h-old joiner 4003 was excluded from new-member accounts and from raidSnapshotFlooders.
		client := cache.GetRedisClient()
		faKey := raidKey(chatID, raidPartFloodAccounts)
		if _, err := client.ZScore(ctx, faKey, "4003").Result(); !errors.Is(err, redis.Nil) {
			t.Fatalf("3h-old joiner 4003 unexpectedly counted as new in flood accounts: err=%v", err)
		}
		if err := client.ZAdd(ctx, faKey, redis.Z{Score: float64(base.UnixMilli()), Member: "4003"}).Err(); err != nil {
			t.Fatalf("ZAdd 4003 to fa: %v", err)
		}
		saved, err := raidSnapshotFlooders(chatID)
		if err != nil {
			t.Fatalf("raidSnapshotFlooders: %v", err)
		}
		if saved != 2 {
			t.Fatalf("raidSnapshotFlooders(rna=2h) = %d, want 2 (1h joiners kept, 3h joiner excluded)", saved)
		}
		flooders, err := readRaidFlooders(chatID)
		if err != nil {
			t.Fatalf("readRaidFlooders: %v", err)
		}
		for _, f := range flooders {
			if f.UserID == 4003 {
				t.Fatalf("raidSnapshotFlooders included 3h-old joiner 4003 with rna=2h: %+v", flooders)
			}
		}
	})

	t.Run("ai_window_recent_and_ttl", func(t *testing.T) {
		mr := phase28InProcessMiniredis(t)
		base := time.Date(2026, 11, 23, 14, 0, 0, 0, time.UTC)
		now := phase28SwapRaidNow(t, base)
		ctx := context.Background()

		chatID := uniqueModuleChatID()
		cleanupAdvThresholdsChat(t, chatID)

		if err := raiddetection.SetAIEnabled(chatID, true); err != nil {
			t.Fatalf("SetAIEnabled: %v", err)
		}
		if err := raiddetection.SetAIThreshold(chatID, 5); err != nil {
			t.Fatalf("SetAIThreshold: %v", err)
		}
		if err := advthresholds.SetField(ctx, chatID, advthresholds.FieldRaidAIWindow, 300, 1); err != nil {
			t.Fatalf("SetField(aiw=300): %v", err)
		}
		if err := advthresholds.SetField(ctx, chatID, advthresholds.FieldRaidRecentHours, 1, 1); err != nil {
			t.Fatalf("SetField(rna=1): %v", err)
		}

		// 30m-old joiner is inside 1h; 2h-old joiner is outside 1h.
		phase28SeedJoinedAt(t, chatID, 5001, base.Add(-30*time.Minute))
		phase28SeedJoinedAt(t, chatID, 5002, base.Add(-2*time.Hour))
		phase28SeedJoinedAt(t, chatID, 5003, base.Add(-30*time.Minute))

		client := cache.GetRedisClient()
		isNew, _, err := raidAICheckCandidate(client, chatID, 5001, "h1")
		if err != nil || !isNew {
			t.Fatalf("raidAICheckCandidate(30m joiner, rna=1h) = (isNew=%v, err=%v), want (true, nil)", isNew, err)
		}
		isNew, _, err = raidAICheckCandidate(client, chatID, 5002, "h2")
		if err != nil || isNew {
			t.Fatalf("raidAICheckCandidate(2h joiner, rna=1h) = (isNew=%v, err=%v), want (false, nil)", isNew, err)
		}

		botClient := phase28NewLockdownBotClient()
		bot := newModuleTestBot(botClient)
		chat := gotgbot.Chat{Id: chatID, Type: "supergroup", Title: "AI Tuned"}

		// 2 verdicts at now-200s (outside default 120s, inside configured 300s), 3 at now.
		*now = base.Add(-200 * time.Second)
		raidAIRecordVerdict(bot, chat, 5001, "m:1")
		raidAIRecordVerdict(bot, chat, 5002, "m:2")
		*now = base
		raidAIRecordVerdict(bot, chat, 5001, "m:3")
		raidAIRecordVerdict(bot, chat, 5002, "m:4")
		raidAIRecordVerdict(bot, chat, 5003, "m:5")
		raidAIFires.wait()
		raidPosts.wait()

		active, err := lockdown.GetActiveFresh(chatID)
		if err != nil {
			t.Fatalf("lockdown.GetActiveFresh: %v", err)
		}
		if active == nil {
			t.Fatal("expected active AI burst lockdown with aiw=300s, got nil")
		}
		if active.TriggerKind != models.LockdownTriggerAIBurst || active.TriggerWindow != 300 || active.TriggerCount != 5 {
			t.Fatalf("AI lockdown = (kind=%s, window=%d, count=%d), want (ai_burst, 300, 5)",
				active.TriggerKind, active.TriggerWindow, active.TriggerCount)
		}

		// Derived AI TTL = max(raidAITTL=5m, 300s*2=600s) = 600s.
		for _, part := range []string{raidPartAIVerdicts, raidPartAIAccounts} {
			ttl := mr.TTL(raidKey(chatID, part))
			if ttl <= 5*time.Minute || ttl > 600*time.Second {
				t.Fatalf("AI key %s TTL = %s, want > 5m and <= 600s", part, ttl)
			}
		}

		// raidSnapshotAISpammers must keep 30m joiners (5001, 5003) and exclude 2h joiner (5002).
		saved, err := raidSnapshotAISpammers(chatID)
		if err != nil {
			t.Fatalf("raidSnapshotAISpammers: %v", err)
		}
		if saved != 2 {
			t.Fatalf("raidSnapshotAISpammers(rna=1h) = %d, want 2 (5001 and 5003 kept, 5002 excluded)", saved)
		}
		spammers, err := readRaidAISpammers(chatID)
		if err != nil {
			t.Fatalf("readRaidAISpammers: %v", err)
		}
		for _, sp := range spammers {
			if sp.UserID == 5002 {
				t.Fatalf("raidSnapshotAISpammers included 2h-old joiner 5002 with rna=1h: %+v", spammers)
			}
		}
	})

	t.Run("apply_normalizes_default_and_rejects_out_of_bounds", func(t *testing.T) {
		_ = phase28InProcessMiniredis(t)
		ctx := context.Background()
		chatID := uniqueModuleChatID()
		cleanupAdvThresholdsChat(t, chatID)

		if out := applyAdvancedThreshold(ctx, chatID, advthresholds.Field("nope"), 60, 1); out != advThresholdInvalid {
			t.Fatalf("applyAdvancedThreshold(unknown) = %v, want advThresholdInvalid", out)
		}
		if out := applyAdvancedThreshold(ctx, chatID, advthresholds.FieldRaidJoinWindow, 14, 1); out != advThresholdInvalid {
			t.Fatalf("applyAdvancedThreshold(jsw=14) = %v, want advThresholdInvalid", out)
		}
		if out := applyAdvancedThreshold(ctx, chatID, advthresholds.FieldRaidJoinWindow, 601, 1); out != advThresholdInvalid {
			t.Fatalf("applyAdvancedThreshold(jsw=601) = %v, want advThresholdInvalid", out)
		}

		if out := applyAdvancedThreshold(ctx, chatID, advthresholds.FieldRaidJoinWindow, 120, 1); out != advThresholdSaved {
			t.Fatalf("applyAdvancedThreshold(jsw=120) = %v, want advThresholdSaved", out)
		}
		if out := applyAdvancedThreshold(ctx, chatID, advthresholds.FieldRaidFloodWindow, 90, 1); out != advThresholdSaved {
			t.Fatalf("applyAdvancedThreshold(flw=90) = %v, want advThresholdSaved", out)
		}
		if out := applyAdvancedThreshold(ctx, chatID, advthresholds.FieldLockdownReminder, 30, 1); out != advThresholdSaved {
			t.Fatalf("applyAdvancedThreshold(lrm=30) = %v, want advThresholdSaved", out)
		}

		// Submitting the compiled default (60 for jsw) normalizes storage to 0 (Default state, P28-01).
		if out := applyAdvancedThreshold(ctx, chatID, advthresholds.FieldRaidJoinWindow, 60, 2); out != advThresholdSaved {
			t.Fatalf("applyAdvancedThreshold(jsw=60 default) = %v, want advThresholdSaved", out)
		}
		got, err := advthresholds.GetFresh(ctx, chatID)
		if err != nil {
			t.Fatalf("GetFresh: %v", err)
		}
		if got.RaidJoinWindowSecs != 0 {
			t.Fatalf("RaidJoinWindowSecs after submitting default 60 = %d, want 0", got.RaidJoinWindowSecs)
		}
		if got.RaidFloodWindowSecs != 90 || got.LockdownReminderMins != 30 {
			t.Fatalf("sibling fields after default normalization = %+v, want flw=90 lrm=30", got)
		}

		// resetAdvancedArea clears only AreaRaidWindows and preserves AreaLockdownReminder.
		if out := resetAdvancedArea(ctx, chatID, advthresholds.AreaRaidWindows, 3); out != advThresholdSaved {
			t.Fatalf("resetAdvancedArea(rw) = %v, want advThresholdSaved", out)
		}
		got, err = advthresholds.GetFresh(ctx, chatID)
		if err != nil {
			t.Fatalf("GetFresh after resetAdvancedArea: %v", err)
		}
		if got.RaidJoinWindowSecs != 0 || got.RaidFloodWindowSecs != 0 || got.RaidAIWindowSecs != 0 {
			t.Fatalf("AreaRaidWindows not cleared: %+v", got)
		}
		if got.LockdownReminderMins != 30 {
			t.Fatalf("resetAdvancedArea(rw) clobbered LockdownReminderMins = %d, want 30", got.LockdownReminderMins)
		}

		if out := resetAdvancedThreshold(ctx, chatID, advthresholds.FieldLockdownReminder, 4); out != advThresholdSaved {
			t.Fatalf("resetAdvancedThreshold(lrm) = %v, want advThresholdSaved", out)
		}
		got, err = advthresholds.GetFresh(ctx, chatID)
		if err != nil {
			t.Fatalf("GetFresh after resetAdvancedThreshold: %v", err)
		}
		if got.CustomizedCount() != 0 {
			t.Fatalf("CustomizedCount after resetting all = %d, want 0 (%+v)", got.CustomizedCount(), got)
		}
	})

	t.Run("fail_to_default_on_lookup_error", func(t *testing.T) {
		_ = phase28InProcessMiniredis(t)
		base := time.Date(2026, 11, 23, 14, 0, 0, 0, time.UTC)
		phase28SwapRaidNow(t, base)

		chatID := uniqueModuleChatID()
		cleanupAdvThresholdsChat(t, chatID)

		prevLookup := advThresholdsLookup
		advThresholdsLookup = func(context.Context, int64) (advthresholds.Thresholds, error) {
			return advthresholds.Thresholds{}, errors.New("simulated transient db error")
		}
		t.Cleanup(func() { advThresholdsLookup = prevLookup })

		th := advThresholdsFor(context.Background(), chatID)
		if advRaidJoinWindow(th) != 60*time.Second ||
			advRaidFloodWindow(th) != 30*time.Second ||
			advRaidAIWindow(th) != 120*time.Second ||
			advRaidRecentWindow(th) != 24*time.Hour {
			t.Fatalf("fail-to-default resolved = (%s,%s,%s,%s), want (60s,30s,120s,24h)",
				advRaidJoinWindow(th), advRaidFloodWindow(th), advRaidAIWindow(th), advRaidRecentWindow(th))
		}

		// Raid detection remains active on compiled defaults even when threshold lookup errors.
		phase28SeedJoinedAt(t, chatID, 6001, base.Add(-30*time.Second))
		botClient := phase28NewLockdownBotClient()
		bot := newModuleTestBot(botClient)
		chat := gotgbot.Chat{Id: chatID, Type: "supergroup", Title: "Fail-To-Default"}
		raidRecordAndFire(bot, &chat, &models.ChatRaidDetection{
			ChatID:        chatID,
			JoinEnabled:   true,
			JoinThreshold: 3,
		}, []int64{6002, 6003})
		raidPosts.wait()

		active, err := lockdown.GetActiveFresh(chatID)
		if err != nil {
			t.Fatalf("lockdown.GetActiveFresh: %v", err)
		}
		if active == nil {
			t.Fatal("expected raid join surge to stay active on lookup error, got nil")
		}
		if active.TriggerKind != models.LockdownTriggerJoinSurge || active.TriggerWindow != 60 || active.TriggerCount != 3 {
			t.Fatalf("fail-to-default lockdown = (kind=%s, window=%d, count=%d), want (join_surge, 60, 3)",
				active.TriggerKind, active.TriggerWindow, active.TriggerCount)
		}
	})
}
