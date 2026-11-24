//go:build testtools

package modules

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/alicebob/miniredis/v2"
	gocache "github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/marshaler"
	gocache_store "github.com/eko/gocache/store/redis/v4"
	"github.com/redis/go-redis/v9"

	"github.com/uasneppy/Xenobot-72/xenobot/config"
	"github.com/uasneppy/Xenobot-72/xenobot/db"
	dbcache "github.com/uasneppy/Xenobot-72/xenobot/db/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/db/captcha"
	"github.com/uasneppy/Xenobot-72/xenobot/db/lockdown"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/db/raiddetection"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/namecanon"
)

// phase28InProcessMiniredis starts an in-process miniredis server and connects a real
// *redis.Client to it over net.Pipe() using mr.Server().ServeConn, avoiding any localhost
// TCP dials so Redis Lua scripts run identically in sandboxed environments and CI.
func phase28InProcessMiniredis(t *testing.T) *miniredis.Miniredis {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() error = %v", err)
	}
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{
		Addr: "in-process",
		Dialer: func(_ context.Context, _, _ string) (net.Conn, error) {
			clientConn, serverConn := net.Pipe()
			go mr.Server().ServeConn(serverConn)
			return clientConn, nil
		},
	})
	t.Cleanup(func() { _ = client.Close() })

	previousMarshal, previousManager, previousClient := cache.GetCacheState()
	manager := gocache.New[any](gocache_store.NewRedis(client))
	cache.SetCacheState(marshaler.New(manager), manager, client)
	t.Cleanup(func() {
		cache.SetCacheState(previousMarshal, previousManager, previousClient)
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("in-process miniredis ping error = %v", err)
	}
	return mr
}

type phase28FailRoundTripper struct {
	t *testing.T
}

func (f phase28FailRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	f.t.Errorf("unexpected outbound HTTP request to %s", req.URL)
	return nil, errors.New("outbound HTTP disabled in Phase 28 golden test")
}

func phase28SwapRaidNow(t *testing.T, now time.Time) *time.Time {
	t.Helper()
	current := now
	previous := raidNow
	raidNow = func() time.Time { return current }
	t.Cleanup(func() { raidNow = previous })
	return &current
}

func phase28NewLockdownBotClient() *moduleBotClient {
	client := newModuleBotClient()
	client.responses["getChat"] = json.RawMessage(
		`{"id":-1001,"type":"supergroup","title":"Test Chat","permissions":{"can_send_messages":true,"can_send_audios":true,"can_send_documents":true,"can_send_photos":true,"can_send_videos":true,"can_send_video_notes":true,"can_send_voice_notes":true,"can_send_polls":true,"can_send_other_messages":true,"can_add_web_page_previews":true}}`,
	)
	return client
}

func phase28CleanupRaidChat(t *testing.T, chatID int64) {
	t.Helper()
	t.Cleanup(func() {
		raidAIJobs.wait()
		raidAIFires.wait()
		raidPosts.wait()
		if db.DB == nil {
			return
		}
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatLockdownAlert{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatLockdownBurst{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatLockdown{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatRaidDetection{}).Error
		dbcache.DeleteCache(dbcache.CacheKey("raid_detection", chatID))
	})
}

func phase28SeedJoinedAt(t *testing.T, chatID, userID int64, at time.Time) {
	t.Helper()
	client := cache.GetRedisClient()
	if client == nil {
		t.Fatal("redis client is nil")
	}
	err := client.ZAdd(
		context.Background(),
		raidKey(chatID, raidPartJoined),
		redis.Z{Score: float64(at.UnixMilli()), Member: strconv.FormatInt(userID, 10)},
	).Err()
	if err != nil {
		t.Fatalf("ZAdd joined(%d, %d) error = %v", chatID, userID, err)
	}
}

// TestLegacyPhase28DefaultThresholds characterizes the exact default threshold behaviors
// recorded on unchanged commit 654c8311 before Phase 28 makes them per-group tunable (MSG-11).
// Never edit or re-record this file in later plans: any failure here for an unconfigured group
// is a default-behavior regression.
func TestLegacyPhase28DefaultThresholds(t *testing.T) {
	t.Run("raid_join_window_60s_and_recent_24h", func(t *testing.T) {
		mr := phase28InProcessMiniredis(t)
		base := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
		phase28SwapRaidNow(t, base)

		chatID := uniqueModuleChatID()
		phase28CleanupRaidChat(t, chatID)

		// Seed three historical joins: 25h ago (outside 24h recent window -> trimmed),
		// 61s ago (outside 60s surge window, inside 24h -> kept in set, excluded from surge count),
		// and 59s ago (inside 60s surge window -> counted).
		phase28SeedJoinedAt(t, chatID, 1001, base.Add(-25*time.Hour))
		phase28SeedJoinedAt(t, chatID, 1002, base.Add(-61*time.Second))
		phase28SeedJoinedAt(t, chatID, 1003, base.Add(-59*time.Second))

		count, err := recordRaidJoins(chatID, []int64{1004})
		if err != nil {
			t.Fatalf("recordRaidJoins() error = %v", err)
		}
		if count != 2 {
			t.Fatalf("recordRaidJoins() count = %d, want 2 (1003 at -59s and 1004 at now)", count)
		}

		client := cache.GetRedisClient()
		joinedKey := raidKey(chatID, raidPartJoined)
		if _, err := client.ZScore(context.Background(), joinedKey, "1001").Result(); !errors.Is(err, redis.Nil) {
			t.Fatalf("user 1001 (now-25h) ZScore err = %v, want redis.Nil (trimmed past 24h)", err)
		}
		if _, err := client.ZScore(context.Background(), joinedKey, "1002").Result(); err != nil {
			t.Fatalf("user 1002 (now-61s) ZScore err = %v, want member kept within 24h", err)
		}
		if _, err := client.ZScore(context.Background(), joinedKey, "1003").Result(); err != nil {
			t.Fatalf("user 1003 (now-59s) ZScore err = %v, want present", err)
		}
		if _, err := client.ZScore(context.Background(), joinedKey, "1004").Result(); err != nil {
			t.Fatalf("user 1004 (now) ZScore err = %v, want present", err)
		}

		ttl := mr.TTL(joinedKey)
		if ttl <= 24*time.Hour || ttl > 25*time.Hour {
			t.Fatalf("joined set TTL = %s, want > 24h and <= 25h", ttl)
		}

		// Trigger a live join surge on the unchanged defaults and verify TriggerWindow == 60.
		botClient := phase28NewLockdownBotClient()
		bot := newModuleTestBot(botClient)
		chat := gotgbot.Chat{Id: chatID, Type: "supergroup", Title: "Phase28 Join Surge"}
		raidRecordAndFire(bot, &chat, &models.ChatRaidDetection{
			ChatID:        chatID,
			JoinEnabled:   true,
			JoinThreshold: 5,
		}, []int64{1005, 1006, 1007})
		raidPosts.wait()

		active, err := lockdown.GetActiveFresh(chatID)
		if err != nil {
			t.Fatalf("lockdown.GetActiveFresh() error = %v", err)
		}
		if active == nil {
			t.Fatal("expected active join-surge lockdown, got nil")
		}
		if active.TriggerKind != models.LockdownTriggerJoinSurge || active.TriggerWindow != 60 || active.TriggerCount != 5 {
			t.Fatalf("join surge lockdown = (kind=%s, window=%d, count=%d), want (join_surge, 60, 5)",
				active.TriggerKind, active.TriggerWindow, active.TriggerCount)
		}
	})

	t.Run("raid_flood_window_30s_share_60_floor_3_recent_24h", func(t *testing.T) {
		mr := phase28InProcessMiniredis(t)
		base := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
		now := phase28SwapRaidNow(t, base)

		chatID := uniqueModuleChatID()
		phase28CleanupRaidChat(t, chatID)

		// 3 joiners within 24h (23h ago) and 1 joiner past 24h (25h ago).
		phase28SeedJoinedAt(t, chatID, 2001, base.Add(-23*time.Hour))
		phase28SeedJoinedAt(t, chatID, 2002, base.Add(-23*time.Hour))
		phase28SeedJoinedAt(t, chatID, 2003, base.Add(-23*time.Hour))
		phase28SeedJoinedAt(t, chatID, 2004, base.Add(-25*time.Hour))

		client := cache.GetRedisClient()

		// Item at now-31s must age out once an item is recorded at now (30s window).
		*now = base.Add(-31 * time.Second)
		if items, newItems, accounts, err := recordRaidFloodItem(client, chatID, 2001, "m:old"); err != nil {
			t.Fatalf("recordRaidFloodItem(old) error = %v", err)
		} else if items != 1 || newItems != 1 || accounts != 1 {
			t.Fatalf("recordRaidFloodItem(old) = (%d,%d,%d), want (1,1,1)", items, newItems, accounts)
		}

		*now = base
		if items, newItems, accounts, err := recordRaidFloodItem(client, chatID, 2001, "m:1"); err != nil {
			t.Fatalf("recordRaidFloodItem(m:1) error = %v", err)
		} else if items != 1 || newItems != 1 || accounts != 1 {
			t.Fatalf("recordRaidFloodItem(m:1) = (%d,%d,%d), want (1,1,1) with m:old excluded", items, newItems, accounts)
		}
		if items, newItems, accounts, err := recordRaidFloodItem(client, chatID, 2002, "m:2"); err != nil {
			t.Fatalf("recordRaidFloodItem(m:2) error = %v", err)
		} else if items != 2 || newItems != 2 || accounts != 2 {
			t.Fatalf("recordRaidFloodItem(m:2) = (%d,%d,%d), want (2,2,2)", items, newItems, accounts)
		}
		if items, newItems, accounts, err := recordRaidFloodItem(client, chatID, 2003, "m:3"); err != nil {
			t.Fatalf("recordRaidFloodItem(m:3) error = %v", err)
		} else if items != 3 || newItems != 3 || accounts != 3 {
			t.Fatalf("recordRaidFloodItem(m:3) = (%d,%d,%d), want (3,3,3)", items, newItems, accounts)
		}
		// User 2004 joined 25h ago -> counts toward total items, NOT toward newItems/accounts.
		if items, newItems, accounts, err := recordRaidFloodItem(client, chatID, 2004, "m:4"); err != nil {
			t.Fatalf("recordRaidFloodItem(m:4) error = %v", err)
		} else if items != 4 || newItems != 3 || accounts != 3 {
			t.Fatalf("recordRaidFloodItem(m:4, 25h joiner) = (%d,%d,%d), want (4,3,3)", items, newItems, accounts)
		}

		for _, part := range []string{raidPartFloodItems, raidPartFloodNew, raidPartFloodAccounts} {
			ttl := mr.TTL(raidKey(chatID, part))
			if ttl <= 0 || ttl > 2*time.Minute {
				t.Fatalf("flood key %s TTL = %s, want > 0 and <= 2m", part, ttl)
			}
		}

		// Pin 60% new-member share and 3-account floor directly.
		if fire, pct := raidFloodCounts(10, 6, 3, 10); !fire || pct != 60 {
			t.Fatalf("raidFloodCounts(10,6,3,10) = (%v,%d), want (true,60)", fire, pct)
		}
		if fire, pct := raidFloodCounts(10, 5, 3, 10); fire || pct != 50 {
			t.Fatalf("raidFloodCounts(10,5,3,10) = (%v,%d), want (false,50)", fire, pct)
		}
		if fire, pct := raidFloodCounts(10, 6, 2, 10); fire || pct != 60 {
			t.Fatalf("raidFloodCounts(10,6,2,10) = (%v,%d), want (false,60)", fire, pct)
		}

		// Also put 2004 (25h joiner) in flood accounts to verify raidSnapshotFlooders keeps
		// only <=24h joiners (2001, 2002, 2003) and drops 2004.
		if err := client.ZAdd(context.Background(), raidKey(chatID, raidPartFloodAccounts),
			redis.Z{Score: float64(base.UnixMilli()), Member: "2004"}).Err(); err != nil {
			t.Fatalf("ZAdd flood accounts error = %v", err)
		}
		saved, err := raidSnapshotFlooders(chatID)
		if err != nil {
			t.Fatalf("raidSnapshotFlooders() error = %v", err)
		}
		if saved != 3 {
			t.Fatalf("raidSnapshotFlooders() = %d, want 3 (23h joiners kept, 25h joiner dropped)", saved)
		}
		flooders, err := readRaidFlooders(chatID)
		if err != nil {
			t.Fatalf("readRaidFlooders() error = %v", err)
		}
		if len(flooders) != 3 {
			t.Fatalf("readRaidFlooders() len = %d, want 3", len(flooders))
		}
		for _, f := range flooders {
			if f.UserID == 2004 {
				t.Fatalf("readRaidFlooders() unexpectedly included 25h joiner 2004: %+v", flooders)
			}
		}

		// End-to-end flood trigger via raidFloodCountUpdate verifying TriggerWindow == 30 and TriggerPercent == 60.
		fireChatID := uniqueModuleChatID()
		phase28CleanupRaidChat(t, fireChatID)
		if err := raiddetection.SetFloodEnabled(fireChatID, true); err != nil {
			t.Fatalf("raiddetection.SetFloodEnabled() error = %v", err)
		}
		if err := raiddetection.SetFloodThreshold(fireChatID, 10); err != nil {
			t.Fatalf("raiddetection.SetFloodThreshold() error = %v", err)
		}
		phase28SeedJoinedAt(t, fireChatID, 2101, base.Add(-1*time.Hour))
		phase28SeedJoinedAt(t, fireChatID, 2102, base.Add(-1*time.Hour))
		phase28SeedJoinedAt(t, fireChatID, 2103, base.Add(-1*time.Hour))

		botClient := phase28NewLockdownBotClient()
		bot := newModuleTestBot(botClient)
		fireChat := gotgbot.Chat{Id: fireChatID, Type: "supergroup", Title: "Phase28 Flood"}
		senders := []int64{2101, 2101, 2102, 2102, 2103, 2103, 2104, 2104, 2104, 2104}
		for i, uid := range senders {
			ctx := newModuleMessageContext(bot, fireChat, gotgbot.User{Id: uid, FirstName: "U"}, fmt.Sprintf("spam %d", i))
			ctx.EffectiveMessage.MessageId = int64(500 + i)
			if err := raidFloodCountUpdate(bot, ctx, false); err == nil {
				t.Fatalf("raidFloodCountUpdate(%d) returned nil, want ContinueGroups", i)
			}
		}
		raidPosts.wait()

		active, err := lockdown.GetActiveFresh(fireChatID)
		if err != nil {
			t.Fatalf("lockdown.GetActiveFresh(flood) error = %v", err)
		}
		if active == nil {
			t.Fatal("expected active flood lockdown, got nil")
		}
		if active.TriggerKind != models.LockdownTriggerFlood || active.TriggerWindow != 30 || active.TriggerPercent < 60 || active.TriggerCount != 10 {
			t.Fatalf("flood lockdown = (kind=%s, window=%d, percent=%d, count=%d), want (flood, 30, >=60, 10)",
				active.TriggerKind, active.TriggerWindow, active.TriggerPercent, active.TriggerCount)
		}
	})

	t.Run("raid_ai_window_120s_and_recent_24h", func(t *testing.T) {
		mr := phase28InProcessMiniredis(t)
		base := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
		now := phase28SwapRaidNow(t, base)

		chatID := uniqueModuleChatID()
		phase28CleanupRaidChat(t, chatID)

		phase28SeedJoinedAt(t, chatID, 3001, base.Add(-23*time.Hour))
		phase28SeedJoinedAt(t, chatID, 3002, base.Add(-25*time.Hour))

		client := cache.GetRedisClient()
		isNew, _, err := raidAICheckCandidate(client, chatID, 3001, "h1")
		if err != nil || !isNew {
			t.Fatalf("raidAICheckCandidate(23h joiner) = (isNew=%v, err=%v), want (true, nil)", isNew, err)
		}
		isNew, _, err = raidAICheckCandidate(client, chatID, 3002, "h2")
		if err != nil || isNew {
			t.Fatalf("raidAICheckCandidate(25h joiner) = (isNew=%v, err=%v), want (false, nil)", isNew, err)
		}

		// Verdict at now-121s must age out once a verdict is recorded at now (120s window).
		*now = base.Add(-121 * time.Second)
		if items, accounts, err := recordRaidAIVerdict(client, chatID, 3001, "m:old"); err != nil {
			t.Fatalf("recordRaidAIVerdict(old) error = %v", err)
		} else if items != 1 || accounts != 1 {
			t.Fatalf("recordRaidAIVerdict(old) = (%d,%d), want (1,1)", items, accounts)
		}

		*now = base
		if items, accounts, err := recordRaidAIVerdict(client, chatID, 3001, "m:1"); err != nil {
			t.Fatalf("recordRaidAIVerdict(m:1) error = %v", err)
		} else if items != 1 || accounts != 1 {
			t.Fatalf("recordRaidAIVerdict(m:1) = (%d,%d), want (1,1) with m:old excluded", items, accounts)
		}
		if items, accounts, err := recordRaidAIVerdict(client, chatID, 3002, "m:2"); err != nil {
			t.Fatalf("recordRaidAIVerdict(m:2) error = %v", err)
		} else if items != 2 || accounts != 2 {
			t.Fatalf("recordRaidAIVerdict(m:2) = (%d,%d), want (2,2)", items, accounts)
		}
		if items, accounts, err := recordRaidAIVerdict(client, chatID, 3003, "m:3"); err != nil {
			t.Fatalf("recordRaidAIVerdict(m:3) error = %v", err)
		} else if items != 3 || accounts != 3 {
			t.Fatalf("recordRaidAIVerdict(m:3) = (%d,%d), want (3,3)", items, accounts)
		}

		for _, part := range []string{raidPartAIVerdicts, raidPartAIAccounts} {
			ttl := mr.TTL(raidKey(chatID, part))
			if ttl <= 0 || ttl > 5*time.Minute {
				t.Fatalf("AI burst key %s TTL = %s, want > 0 and <= 5m", part, ttl)
			}
		}

		saved, err := raidSnapshotAISpammers(chatID)
		if err != nil {
			t.Fatalf("raidSnapshotAISpammers() error = %v", err)
		}
		if saved != 1 {
			t.Fatalf("raidSnapshotAISpammers() = %d, want 1 (only 23h joiner 3001 kept)", saved)
		}
		spammers, err := readRaidAISpammers(chatID)
		if err != nil {
			t.Fatalf("readRaidAISpammers() error = %v", err)
		}
		if len(spammers) != 1 || spammers[0].UserID != 3001 {
			t.Fatalf("readRaidAISpammers() = %+v, want single entry for user 3001", spammers)
		}

		// End-to-end AI burst trigger verifying TriggerWindow == 120.
		fireChatID := uniqueModuleChatID()
		phase28CleanupRaidChat(t, fireChatID)
		if err := raiddetection.SetAIEnabled(fireChatID, true); err != nil {
			t.Fatalf("raiddetection.SetAIEnabled() error = %v", err)
		}
		if err := raiddetection.SetAIThreshold(fireChatID, 5); err != nil {
			t.Fatalf("raiddetection.SetAIThreshold() error = %v", err)
		}
		botClient := phase28NewLockdownBotClient()
		bot := newModuleTestBot(botClient)
		fireChat := gotgbot.Chat{Id: fireChatID, Type: "supergroup", Title: "Phase28 AI Burst"}
		verdicts := []struct {
			uid  int64
			item string
		}{
			{3101, "m:101"},
			{3101, "m:102"},
			{3102, "m:103"},
			{3102, "m:104"},
			{3103, "m:105"},
		}
		for _, v := range verdicts {
			raidAIRecordVerdict(bot, fireChat, v.uid, v.item)
		}
		raidAIFires.wait()
		raidPosts.wait()

		active, err := lockdown.GetActiveFresh(fireChatID)
		if err != nil {
			t.Fatalf("lockdown.GetActiveFresh(ai_burst) error = %v", err)
		}
		if active == nil {
			t.Fatal("expected active AI burst lockdown, got nil")
		}
		if active.TriggerKind != models.LockdownTriggerAIBurst || active.TriggerWindow != 120 || active.TriggerCount != 5 {
			t.Fatalf("AI burst lockdown = (kind=%s, window=%d, count=%d), want (ai_burst, 120, 5)",
				active.TriggerKind, active.TriggerWindow, active.TriggerCount)
		}
	})

	t.Run("raid_ai_image_bar_085", func(t *testing.T) {
		phase28InProcessMiniredis(t)

		prevEnable := config.AppConfig.EnableAISpam
		prevKey := config.AppConfig.AnthropicAPIKey
		prevHTTP := raidAIClaudeHTTP
		config.AppConfig.EnableAISpam = true
		config.AppConfig.AnthropicAPIKey = "phase28-test-anthropic-key"
		raidAIClaudeHTTP = &http.Client{Transport: phase28FailRoundTripper{t: t}}
		t.Cleanup(func() {
			config.AppConfig.EnableAISpam = prevEnable
			config.AppConfig.AnthropicAPIKey = prevKey
			raidAIClaudeHTTP = prevHTTP
		})

		if !(0.85 >= raidAIImageBar) || (0.84 >= raidAIImageBar) {
			t.Fatalf("raidAIImageBar = %v, want 0.85 spam and 0.84 kept", raidAIImageBar)
		}

		client := cache.GetRedisClient()
		raidAIStoreVerdict(client, "p28-img-085", 0.85)
		raidAIStoreVerdict(client, "p28-img-084", 0.84)

		botClient := newModuleBotClient()
		bot := newModuleTestBot(botClient)
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Phase28 Image Bar"}

		spamBefore := raidAISpam.Load()
		keptBefore := raidAIKept.Load()

		raidAIProcessImage(context.Background(), raidAIJob{
			bot:       bot,
			chat:      chat,
			userID:    4001,
			messageID: 101,
			itemKey:   "m:101",
			media:     &raidAIMedia{FileID: "f-085", FileUniqueID: "p28-img-085", Kind: "photo"},
		})
		if gotSpam, gotKept := raidAISpam.Load()-spamBefore, raidAIKept.Load()-keptBefore; gotSpam != 1 || gotKept != 0 {
			t.Fatalf("cached 0.85 deltas = (spam=%d, kept=%d), want (1, 0)", gotSpam, gotKept)
		}

		raidAIProcessImage(context.Background(), raidAIJob{
			bot:       bot,
			chat:      chat,
			userID:    4002,
			messageID: 102,
			itemKey:   "m:102",
			media:     &raidAIMedia{FileID: "f-084", FileUniqueID: "p28-img-084", Kind: "photo"},
		})
		if gotSpam, gotKept := raidAISpam.Load()-spamBefore, raidAIKept.Load()-keptBefore; gotSpam != 1 || gotKept != 1 {
			t.Fatalf("cached 0.84 deltas = (spam=%d, kept=%d), want (1, 1)", gotSpam, gotKept)
		}
	})

	t.Run("memberreq_ai_bar_090", func(t *testing.T) {
		if got := memberReqAIVerdictOf(0.90); got != memberReqBreach {
			t.Fatalf("memberReqAIVerdictOf(0.90) = %v, want memberReqBreach", got)
		}
		if got := memberReqAIVerdictOf(0.89); got != memberReqPass {
			t.Fatalf("memberReqAIVerdictOf(0.89) = %v, want memberReqPass", got)
		}
		if !memberReqAIWatchSatisfied("0.89", "", false, "h") {
			t.Fatal("memberReqAIWatchSatisfied(0.89) = false, want true")
		}
		if memberReqAIWatchSatisfied("0.90", "", false, "h") {
			t.Fatal("memberReqAIWatchSatisfied(0.90) = true, want false")
		}

		canon, ok := namecanon.Of("Promo", "Bot", "promobot")
		if !ok {
			t.Fatal("namecanon.Of failed for judgeable test name")
		}
		breachPlan := memberReqAIDecide(canon, "k", memberReqAIState{verdict: "0.90"}, true)
		if !breachPlan.cached || breachPlan.verdict != memberReqBreach {
			t.Fatalf("memberReqAIDecide(0.90) = %+v, want cached breach", breachPlan)
		}
		passPlan := memberReqAIDecide(canon, "k", memberReqAIState{verdict: "0.89"}, true)
		if !passPlan.cached || passPlan.verdict != memberReqPass {
			t.Fatalf("memberReqAIDecide(0.89) = %+v, want cached pass", passPlan)
		}
	})

	t.Run("memberreq_cap_10_and_pause_10m", func(t *testing.T) {
		mr := phase28InProcessMiniredis(t)
		withStaffLocale(t)

		base := time.Date(2026, 11, 23, 12, 0, 10, 0, time.UTC)
		phase28SwapRaidNow(t, base)

		botClient := newModuleBotClient()
		bot := newModuleTestBot(botClient)
		chatID := uniqueModuleChatID()

		for i := 0; i < 10; i++ {
			if !memberReqCapAllow(context.Background(), bot, chatID, int64(6000+i)) {
				t.Fatalf("memberReqCapAllow(user %d) = false, want true within default cap 10/min", i+1)
			}
			if mr.Exists(memberReqPausedKey(chatID)) {
				t.Fatalf("pause latch set prematurely after %d users", i+1)
			}
		}

		if memberReqCapAllow(context.Background(), bot, chatID, 6010) {
			t.Fatal("memberReqCapAllow(11th user) = true, want false over default cap 10/min")
		}
		if !mr.Exists(memberReqPausedKey(chatID)) {
			t.Fatal("pause latch not set after 11th user")
		}
		if ttl := mr.TTL(memberReqPausedKey(chatID)); ttl != 10*time.Minute {
			t.Fatalf("pause latch TTL = %s, want 10m0s", ttl)
		}

		sent := botClient.callsFor("sendMessage")
		if len(sent) != 1 {
			t.Fatalf("pause notice sendMessage calls = %d, want 1", len(sent))
		}
		text := fmt.Sprint(sent[0].Params["text"])
		wantFields := []string{staffMarker("mreq_notice_paused"), "10", "10"}
		if got := strings.Join(strings.Fields(text), " "); got != strings.Join(wantFields, " ") {
			t.Fatalf("pause notice = %q, want %q (minutes=10, n=10)", got, strings.Join(wantFields, " "))
		}
	})

	t.Run("lockdown_reminder_every_1h", func(t *testing.T) {
		start := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
		ld := &models.ChatLockdown{AlertStartedAt: &start}

		cases := []struct {
			offset time.Duration
			want   int
		}{
			{59 * time.Minute, 0},
			{60 * time.Minute, 1},
			{61 * time.Minute, 1},
			{120 * time.Minute, 2},
		}
		for _, tc := range cases {
			if got := lockdownReminderSeq(ld, start.Add(tc.offset)); got != tc.want {
				t.Fatalf("lockdownReminderSeq(+%s) = %d, want %d", tc.offset, got, tc.want)
			}
		}
	})

	t.Run("captcha_timeout_default_2m_and_configured_5m", func(t *testing.T) {
		phase28InProcessMiniredis(t)
		rawLocale, err := os.ReadFile("../../locales/en.yml")
		if err != nil {
			t.Fatalf("read locales/en.yml error = %v", err)
		}
		restoreI18n, err := i18n.OverrideManagerForTest(string(rawLocale))
		if err != nil {
			t.Fatalf("OverrideManagerForTest() error = %v", err)
		}
		t.Cleanup(restoreI18n)
		if db.DB != nil && db.DB.Config != nil {
			prevNowFunc := db.DB.NowFunc
			db.DB.NowFunc = func() time.Time { return time.Now().UTC() }
			t.Cleanup(func() { db.DB.NowFunc = prevNowFunc })
		}

		cases := []struct {
			name       string
			setTimeout int // 0 leaves default 2m untouched
			wantMins   int
		}{
			{name: "default_2m", setTimeout: 0, wantMins: 2},
			{name: "configured_5m", setTimeout: 5, wantMins: 5},
		}

		for _, tc := range cases {
			chat := gotgbot.Chat{
				Id:    uniqueModuleChatID(),
				Type:  "supergroup",
				Title: "Phase28 Captcha " + tc.name,
			}
			chatID := chat.Id
			t.Cleanup(func() {
				if db.DB == nil {
					return
				}
				_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.CaptchaAttempts{}).Error
				_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.CaptchaSettings{}).Error
				_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
				dbcache.DeleteCache(dbcache.CacheKey("captcha_settings", chatID))
				dbcache.DeleteCache(dbcache.CacheKey("captcha_pending", chatID))
			})

			if err := captcha.SetCaptchaEnabled(chatID, true); err != nil {
				t.Fatalf("%s: SetCaptchaEnabled() error = %v", tc.name, err)
			}
			if tc.setTimeout > 0 {
				if err := captcha.SetCaptchaTimeout(chatID, tc.setTimeout); err != nil {
					t.Fatalf("%s: SetCaptchaTimeout(%d) error = %v", tc.name, tc.setTimeout, err)
				}
			}

			botClient := newModuleBotClient()
			bot := newModuleTestBot(botClient)
			ctx := newModuleMessageContext(bot, chat, gotgbot.User{Id: 777000, FirstName: "Telegram"}, "join")

			before := time.Now().UTC()
			if err := SendCaptcha(bot, ctx, 42, "Member"); err != nil {
				t.Fatalf("%s: SendCaptcha() error = %v", tc.name, err)
			}
			after := time.Now().UTC()

			attempt, err := captcha.GetCaptchaAttemptIncludingExpired(42, chatID)
			if err != nil {
				t.Fatalf("%s: GetCaptchaAttemptIncludingExpired() error = %v", tc.name, err)
			}
			if attempt == nil {
				t.Fatalf("%s: captcha attempt not found", tc.name)
			}

			wantDur := time.Duration(tc.wantMins) * time.Minute
			minExp := before.Add(wantDur - 5*time.Second)
			maxExp := after.Add(wantDur + 5*time.Second)
			if attempt.ExpiresAt.Before(minExp) || attempt.ExpiresAt.After(maxExp) {
				t.Fatalf("%s: attempt.ExpiresAt = %s, want within [%s, %s]",
					tc.name, attempt.ExpiresAt.Format(time.RFC3339), minExp.Format(time.RFC3339), maxExp.Format(time.RFC3339))
			}

			var challengeText string
			if photos := botClient.callsFor("sendPhoto"); len(photos) > 0 {
				challengeText = fmt.Sprint(photos[len(photos)-1].Params["caption"])
			} else if msgs := botClient.callsFor("sendMessage"); len(msgs) > 0 {
				challengeText = fmt.Sprint(msgs[len(msgs)-1].Params["text"])
			} else {
				t.Fatalf("%s: SendCaptcha sent neither sendPhoto nor sendMessage", tc.name)
			}
			wantToken := fmt.Sprintf("<b>%d minutes</b>", tc.wantMins)
			if !strings.Contains(challengeText, wantToken) {
				t.Fatalf("%s: challenge text %q does not contain timeout %q", tc.name, challengeText, wantToken)
			}
		}
	})
}
