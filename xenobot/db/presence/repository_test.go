//go:build testtools

package presence

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	utilsCache "github.com/uasneppy/Xenobot-72/xenobot/utils/cache"
)

var testChatSeq atomic.Int64

func newChat(t *testing.T) int64 {
	t.Helper()
	chatID := -(time.Now().UnixNano() + testChatSeq.Add(1))
	t.Cleanup(func() {
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatPresence{})
		cache.DeleteCache(CacheKey(chatID))
	})
	return chatID
}

func rowCount(t *testing.T, chatID int64) int64 {
	t.Helper()
	var n int64
	if err := db.DB.Model(&models.ChatPresence{}).Where("chat_id = ?", chatID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func TestChatPresenceSettings(t *testing.T) {
	ctx := context.Background()

	t.Run("missing row returns defaults without inserting a row", func(t *testing.T) {
		chatID := newChat(t)
		want := Settings{
			ConsensusEnabled:   false,
			ConsensusThreshold: DefaultConsensusThreshold,
			RetractEnabled:     true,
			AIReactMode:        AIReactModeOff,
		}
		if Defaults() != want {
			t.Fatalf("Defaults() = %+v, want %+v", Defaults(), want)
		}
		got, err := Get(ctx, chatID)
		if err != nil || got != want {
			t.Fatalf("Get = %+v, %v, want %+v", got, err, want)
		}
		fresh, err := GetFresh(ctx, chatID)
		if err != nil || fresh != want {
			t.Fatalf("GetFresh = %+v, %v, want %+v", fresh, err, want)
		}
		if rowCount(t, chatID) != 0 {
			t.Fatal("a read created a row")
		}
	})

	t.Run("column-targeted upserts preserve defaults and explicit false", func(t *testing.T) {
		chatID := newChat(t)
		if _, err := Get(ctx, chatID); err != nil { // warm cache
			t.Fatal(err)
		}

		if err := SetConsensusEnabled(ctx, chatID, true, 7); err != nil {
			t.Fatal(err)
		}
		want := Settings{ConsensusEnabled: true, ConsensusThreshold: 3, RetractEnabled: true, AIReactMode: AIReactModeOff}
		if got, err := Get(ctx, chatID); err != nil || got != want {
			t.Fatalf("after SetConsensusEnabled(true): %+v, %v, want %+v", got, err, want)
		}
		if fresh, err := GetFresh(ctx, chatID); err != nil || fresh != want {
			t.Fatalf("GetFresh after SetConsensusEnabled(true): %+v, %v, want %+v", fresh, err, want)
		}

		// Explicit Go false on an existing chat preserves ConsensusEnabled and ConsensusThreshold.
		if err := SetRetractEnabled(ctx, chatID, false, 8); err != nil {
			t.Fatal(err)
		}
		want.RetractEnabled = false
		if got, err := Get(ctx, chatID); err != nil || got != want {
			t.Fatalf("after SetRetractEnabled(false): %+v, %v, want %+v", got, err, want)
		}
		if fresh, err := GetFresh(ctx, chatID); err != nil || fresh != want {
			t.Fatalf("GetFresh after SetRetractEnabled(false): %+v, %v, want %+v", fresh, err, want)
		}

		// Turning ConsensusEnabled back off leaves RetractEnabled=false intact.
		if err := SetConsensusEnabled(ctx, chatID, false, 9); err != nil {
			t.Fatal(err)
		}
		want.ConsensusEnabled = false
		if got, err := Get(ctx, chatID); err != nil || got != want {
			t.Fatalf("after SetConsensusEnabled(false): %+v, %v, want %+v", got, err, want)
		}
		var row models.ChatPresence
		if err := db.DB.Where("chat_id = ?", chatID).Take(&row).Error; err != nil {
			t.Fatal(err)
		}
		if row.UpdatedBy != 9 {
			t.Errorf("updated_by = %d, want 9", row.UpdatedBy)
		}

		// Explicit Go false on a brand-new chat with no prior row persists false (no GORM default:true override).
		brandNewChat := newChat(t)
		if err := SetRetractEnabled(ctx, brandNewChat, false, 11); err != nil {
			t.Fatal(err)
		}
		wantNew := Settings{ConsensusEnabled: false, ConsensusThreshold: 3, RetractEnabled: false, AIReactMode: AIReactModeOff}
		if got, err := Get(ctx, brandNewChat); err != nil || got != wantNew {
			t.Fatalf("brand-new SetRetractEnabled(false) Get = %+v, %v, want %+v", got, err, wantNew)
		}
		if fresh, err := GetFresh(ctx, brandNewChat); err != nil || fresh != wantNew {
			t.Fatalf("brand-new SetRetractEnabled(false) GetFresh = %+v, %v, want %+v", fresh, err, wantNew)
		}
	})

	t.Run("threshold validation accepts 2..20 and rejects out-of-range without writing", func(t *testing.T) {
		badChat := newChat(t)
		for _, bad := range []int{1, 0, -1, 21} {
			if ValidConsensusThreshold(bad) {
				t.Errorf("ValidConsensusThreshold(%d) = true, want false", bad)
			}
			if err := SetConsensusThreshold(ctx, badChat, bad, 1); !errors.Is(err, ErrInvalidValue) {
				t.Errorf("SetConsensusThreshold(%d) = %v, want ErrInvalidValue", bad, err)
			}
		}
		if rowCount(t, badChat) != 0 {
			t.Fatal("an invalid threshold inserted a row")
		}

		chatID := newChat(t)
		if err := SetConsensusEnabled(ctx, chatID, true, 5); err != nil {
			t.Fatal(err)
		}
		if err := SetRetractEnabled(ctx, chatID, false, 5); err != nil {
			t.Fatal(err)
		}
		for _, v := range []int{MinConsensusThreshold, 5, MaxConsensusThreshold} {
			if !ValidConsensusThreshold(v) {
				t.Errorf("ValidConsensusThreshold(%d) = false, want true", v)
			}
			if err := SetConsensusThreshold(ctx, chatID, v, 6); err != nil {
				t.Fatalf("SetConsensusThreshold(%d): %v", v, err)
			}
			want := Settings{ConsensusEnabled: true, ConsensusThreshold: v, RetractEnabled: false, AIReactMode: AIReactModeOff}
			if got, err := Get(ctx, chatID); err != nil || got != want {
				t.Fatalf("after SetConsensusThreshold(%d): Get = %+v, %v, want %+v", v, got, err, want)
			}
			if fresh, err := GetFresh(ctx, chatID); err != nil || fresh != want {
				t.Fatalf("after SetConsensusThreshold(%d): GetFresh = %+v, %v, want %+v", v, fresh, err, want)
			}
		}
	})

	t.Run("cache invalidation on writes and DB CHECK constraint enforcement", func(t *testing.T) {
		utilsCache.SetupTestMemoryMarshaler(t)
		chatID := newChat(t)
		// Prime cache with defaults.
		if got, err := Get(ctx, chatID); err != nil || got != Defaults() {
			t.Fatalf("prime Get = %+v, %v", got, err)
		}
		if err := SetConsensusThreshold(ctx, chatID, 7, 10); err != nil {
			t.Fatal(err)
		}
		if got, err := Get(ctx, chatID); err != nil || got.ConsensusThreshold != 7 {
			t.Fatalf("cached Get after SetConsensusThreshold(7) = %+v, %v", got, err)
		}
		if err := SetConsensusEnabled(ctx, chatID, true, 10); err != nil {
			t.Fatal(err)
		}
		if got, err := Get(ctx, chatID); err != nil || !got.ConsensusEnabled || got.ConsensusThreshold != 7 {
			t.Fatalf("cached Get after SetConsensusEnabled(true) = %+v, %v", got, err)
		}
		if err := SetRetractEnabled(ctx, chatID, false, 10); err != nil {
			t.Fatal(err)
		}
		if got, err := Get(ctx, chatID); err != nil || got.RetractEnabled || !got.ConsensusEnabled || got.ConsensusThreshold != 7 {
			t.Fatalf("cached Get after SetRetractEnabled(false) = %+v, %v", got, err)
		}

		// Raw DB insert with out-of-range consensus_threshold must be rejected by CHECK constraint.
		badLow := &models.ChatPresence{ChatID: newChat(t), ConsensusThreshold: 1, RetractEnabled: true, AIReactMode: AIReactModeOff}
		if err := db.DB.Create(badLow).Error; err == nil {
			t.Error("threshold 1 bypassed chk_chat_presence_consensus_threshold")
		}
		badHigh := &models.ChatPresence{ChatID: newChat(t), ConsensusThreshold: 21, RetractEnabled: true, AIReactMode: AIReactModeOff}
		if err := db.DB.Create(badHigh).Error; err == nil {
			t.Error("threshold 21 bypassed chk_chat_presence_consensus_threshold")
		}
	})

	t.Run("ai_react_mode validation, column-targeted upsert, rolling-deploy cache normalization, and CHECK constraint", func(t *testing.T) {
		utilsCache.SetupTestMemoryMarshaler(t)

		// Validation rejects invalid modes before any DB write.
		badChat := newChat(t)
		for _, bad := range []string{"", "high", "OFF", "medium "} {
			if ValidAIReactMode(bad) {
				t.Errorf("ValidAIReactMode(%q) = true, want false", bad)
			}
			if err := SetAIReactMode(ctx, badChat, bad, 1); !errors.Is(err, ErrInvalidValue) {
				t.Errorf("SetAIReactMode(%q) = %v, want ErrInvalidValue", bad, err)
			}
		}
		if rowCount(t, badChat) != 0 {
			t.Fatal("an invalid ai_react_mode inserted a row")
		}

		// Column-targeted upserts preserve ConsensusEnabled, ConsensusThreshold, RetractEnabled and invalidate cache.
		chatID := newChat(t)
		if err := SetConsensusEnabled(ctx, chatID, true, 12); err != nil {
			t.Fatal(err)
		}
		if err := SetConsensusThreshold(ctx, chatID, 7, 12); err != nil {
			t.Fatal(err)
		}
		if err := SetRetractEnabled(ctx, chatID, false, 12); err != nil {
			t.Fatal(err)
		}
		// Warm cache before updating ai_react_mode.
		want := Settings{ConsensusEnabled: true, ConsensusThreshold: 7, RetractEnabled: false, AIReactMode: AIReactModeOff}
		if got, err := Get(ctx, chatID); err != nil || got != want {
			t.Fatalf("warm Get = %+v, %v, want %+v", got, err, want)
		}
		for _, mode := range []string{AIReactModeLow, AIReactModeMedium, AIReactModeOff} {
			if !ValidAIReactMode(mode) {
				t.Errorf("ValidAIReactMode(%q) = false, want true", mode)
			}
			if err := SetAIReactMode(ctx, chatID, mode, 13); err != nil {
				t.Fatalf("SetAIReactMode(%q): %v", mode, err)
			}
			want.AIReactMode = mode
			if got, err := Get(ctx, chatID); err != nil || got != want {
				t.Fatalf("after SetAIReactMode(%q): Get = %+v, %v, want %+v", mode, got, err, want)
			}
			if fresh, err := GetFresh(ctx, chatID); err != nil || fresh != want {
				t.Fatalf("after SetAIReactMode(%q): GetFresh = %+v, %v, want %+v", mode, fresh, err, want)
			}
		}

		// Brand-new chat upsert preserves defaults for other columns.
		freshChat := newChat(t)
		if err := SetAIReactMode(ctx, freshChat, AIReactModeLow, 14); err != nil {
			t.Fatal(err)
		}
		wantFresh := Settings{ConsensusEnabled: false, ConsensusThreshold: DefaultConsensusThreshold, RetractEnabled: true, AIReactMode: AIReactModeLow}
		if got, err := Get(ctx, freshChat); err != nil || got != wantFresh {
			t.Fatalf("brand-new SetAIReactMode(low): Get = %+v, %v, want %+v", got, err, wantFresh)
		}

		// Rolling-deploy cache safety: legacy Phase 31 payload without "ar" tag decodes "" and normalizes to "off".
		type legacySettings struct {
			ConsensusEnabled   bool `msgpack:"ce"`
			ConsensusThreshold int  `msgpack:"ct"`
			RetractEnabled     bool `msgpack:"re"`
		}
		legacyChat := newChat(t)
		if err := utilsCache.GetMarshal().Set(ctx, CacheKey(legacyChat), legacySettings{
			ConsensusEnabled:   true,
			ConsensusThreshold: 5,
			RetractEnabled:     false,
		}); err != nil {
			t.Fatalf("seed legacy cache entry: %v", err)
		}
		wantLegacy := Settings{ConsensusEnabled: true, ConsensusThreshold: 5, RetractEnabled: false, AIReactMode: AIReactModeOff}
		if got, err := Get(ctx, legacyChat); err != nil || got != wantLegacy {
			t.Fatalf("rolling-deploy legacy Get = %+v, %v, want %+v", got, err, wantLegacy)
		}

		// Explicit empty AIReactMode in cache also normalizes safely to AIReactModeOff.
		emptyCacheChat := newChat(t)
		if err := utilsCache.GetMarshal().Set(ctx, CacheKey(emptyCacheChat), Settings{
			ConsensusEnabled:   false,
			ConsensusThreshold: 4,
			RetractEnabled:     true,
			AIReactMode:        "",
		}); err != nil {
			t.Fatalf("seed empty ai_react_mode cache entry: %v", err)
		}
		wantEmpty := Settings{ConsensusEnabled: false, ConsensusThreshold: 4, RetractEnabled: true, AIReactMode: AIReactModeOff}
		if got, err := Get(ctx, emptyCacheChat); err != nil || got != wantEmpty {
			t.Fatalf("empty-mode cached Get = %+v, %v, want %+v", got, err, wantEmpty)
		}

		// Raw DB inserts with invalid ai_react_mode must be rejected by chk_chat_presence_ai_react_mode.
		badHigh := &models.ChatPresence{ChatID: newChat(t), ConsensusThreshold: 3, RetractEnabled: true, AIReactMode: "high"}
		if err := db.DB.Create(badHigh).Error; err == nil {
			t.Error("ai_react_mode \"high\" bypassed chk_chat_presence_ai_react_mode")
		}
		badEmpty := &models.ChatPresence{ChatID: newChat(t), ConsensusThreshold: 3, RetractEnabled: true, AIReactMode: ""}
		if err := db.DB.Create(badEmpty).Error; err == nil {
			t.Error("ai_react_mode \"\" bypassed chk_chat_presence_ai_react_mode")
		}
	})
}
