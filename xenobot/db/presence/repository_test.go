//go:build testtools

package presence

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/gorm"

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
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatPresenceArchived{})
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
			CatStickerEnabled:  false,
			ArchiveEnabled:     false,
			ArchiveChatID:      0,
			ArchiveThreadID:    0,
			ArchiveThreshold:   DefaultArchiveThreshold,
			ArchiveEmojiMode:   ArchiveEmojiModePositive,
			ArchiveDelivery:    ArchiveDeliveryCopy,
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
		want := Defaults()
		want.ConsensusEnabled = true
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
		wantNew := Defaults()
		wantNew.RetractEnabled = false
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
			want := Defaults()
			want.ConsensusEnabled = true
			want.ConsensusThreshold = v
			want.RetractEnabled = false
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
		badLow := &models.ChatPresence{
			ChatID: newChat(t), ConsensusThreshold: 1, RetractEnabled: true, AIReactMode: AIReactModeOff,
			ArchiveThreshold: DefaultArchiveThreshold, ArchiveEmojiMode: ArchiveEmojiModePositive, ArchiveDelivery: ArchiveDeliveryCopy,
		}
		if err := db.DB.Create(badLow).Error; err == nil {
			t.Error("threshold 1 bypassed chk_chat_presence_consensus_threshold")
		}
		badHigh := &models.ChatPresence{
			ChatID: newChat(t), ConsensusThreshold: 21, RetractEnabled: true, AIReactMode: AIReactModeOff,
			ArchiveThreshold: DefaultArchiveThreshold, ArchiveEmojiMode: ArchiveEmojiModePositive, ArchiveDelivery: ArchiveDeliveryCopy,
		}
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
		want := Defaults()
		want.ConsensusEnabled = true
		want.ConsensusThreshold = 7
		want.RetractEnabled = false
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
		wantFresh := Defaults()
		wantFresh.AIReactMode = AIReactModeLow
		if got, err := Get(ctx, freshChat); err != nil || got != wantFresh {
			t.Fatalf("brand-new SetAIReactMode(low): Get = %+v, %v, want %+v", got, err, wantFresh)
		}

		// Rolling-deploy cache safety: legacy Phase 31 payload without "ar" or Phase 33 tags normalizes cleanly.
		type legacyPhase31Settings struct {
			ConsensusEnabled   bool `msgpack:"ce"`
			ConsensusThreshold int  `msgpack:"ct"`
			RetractEnabled     bool `msgpack:"re"`
		}
		legacyChat := newChat(t)
		if err := utilsCache.GetMarshal().Set(ctx, CacheKey(legacyChat), legacyPhase31Settings{
			ConsensusEnabled:   true,
			ConsensusThreshold: 5,
			RetractEnabled:     false,
		}); err != nil {
			t.Fatalf("seed legacy cache entry: %v", err)
		}
		wantLegacy := Defaults()
		wantLegacy.ConsensusEnabled = true
		wantLegacy.ConsensusThreshold = 5
		wantLegacy.RetractEnabled = false
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
		wantEmpty := Defaults()
		wantEmpty.ConsensusThreshold = 4
		if got, err := Get(ctx, emptyCacheChat); err != nil || got != wantEmpty {
			t.Fatalf("empty-mode cached Get = %+v, %v, want %+v", got, err, wantEmpty)
		}

		// Raw DB inserts with invalid ai_react_mode must be rejected by chk_chat_presence_ai_react_mode.
		badHigh := &models.ChatPresence{
			ChatID: newChat(t), ConsensusThreshold: 3, RetractEnabled: true, AIReactMode: "high",
			ArchiveThreshold: DefaultArchiveThreshold, ArchiveEmojiMode: ArchiveEmojiModePositive, ArchiveDelivery: ArchiveDeliveryCopy,
		}
		if err := db.DB.Create(badHigh).Error; err == nil {
			t.Error("ai_react_mode \"high\" bypassed chk_chat_presence_ai_react_mode")
		}
		badEmpty := &models.ChatPresence{
			ChatID: newChat(t), ConsensusThreshold: 3, RetractEnabled: true, AIReactMode: "",
			ArchiveThreshold: DefaultArchiveThreshold, ArchiveEmojiMode: ArchiveEmojiModePositive, ArchiveDelivery: ArchiveDeliveryCopy,
		}
		if err := db.DB.Create(badEmpty).Error; err == nil {
			t.Error("ai_react_mode \"\" bypassed chk_chat_presence_ai_react_mode")
		}
	})

	t.Run("SetCatStickerEnabled column-targeted upsert and cache invalidation", func(t *testing.T) {
		utilsCache.SetupTestMemoryMarshaler(t)
		chatID := newChat(t)
		if _, err := Get(ctx, chatID); err != nil { // warm cache
			t.Fatal(err)
		}
		if err := SetConsensusEnabled(ctx, chatID, true, 21); err != nil {
			t.Fatal(err)
		}
		if err := SetCatStickerEnabled(ctx, chatID, true, 22); err != nil {
			t.Fatal(err)
		}
		want := Defaults()
		want.ConsensusEnabled = true
		want.CatStickerEnabled = true
		if got, err := Get(ctx, chatID); err != nil || got != want {
			t.Fatalf("after SetCatStickerEnabled(true): Get = %+v, %v, want %+v", got, err, want)
		}
		if fresh, err := GetFresh(ctx, chatID); err != nil || fresh != want {
			t.Fatalf("after SetCatStickerEnabled(true): GetFresh = %+v, %v, want %+v", fresh, err, want)
		}

		if err := SetCatStickerEnabled(ctx, chatID, false, 23); err != nil {
			t.Fatal(err)
		}
		want.CatStickerEnabled = false
		if got, err := Get(ctx, chatID); err != nil || got != want {
			t.Fatalf("after SetCatStickerEnabled(false): Get = %+v, %v, want %+v", got, err, want)
		}
	})

	t.Run("SetArchiveTarget, ClearArchiveTarget, and SetArchiveEnabled unlinked refusal", func(t *testing.T) {
		utilsCache.SetupTestMemoryMarshaler(t)
		chatID := newChat(t)

		// Refuses SetArchiveEnabled(true) with no row / unlinked ArchiveChatID.
		if err := SetArchiveEnabled(ctx, chatID, true, 30); !errors.Is(err, ErrInvalidValue) {
			t.Fatalf("SetArchiveEnabled(true) on missing row = %v, want ErrInvalidValue", err)
		}
		if rowCount(t, chatID) != 0 {
			t.Fatal("failed SetArchiveEnabled(true) created a row")
		}

		// SetArchiveTarget rejects targetChatID == 0 without writing.
		if err := SetArchiveTarget(ctx, chatID, 0, 10, 30); !errors.Is(err, ErrInvalidValue) {
			t.Fatalf("SetArchiveTarget(0) = %v, want ErrInvalidValue", err)
		}
		if rowCount(t, chatID) != 0 {
			t.Fatal("failed SetArchiveTarget(0) created a row")
		}

		// Set unrelated fields first to verify ClearArchiveTarget does not clobber them.
		if err := SetConsensusEnabled(ctx, chatID, true, 31); err != nil {
			t.Fatal(err)
		}
		if err := SetCatStickerEnabled(ctx, chatID, true, 31); err != nil {
			t.Fatal(err)
		}

		if err := SetArchiveTarget(ctx, chatID, -100999888, 77, 32); err != nil {
			t.Fatalf("SetArchiveTarget: %v", err)
		}
		want := Defaults()
		want.ConsensusEnabled = true
		want.CatStickerEnabled = true
		want.ArchiveChatID = -100999888
		want.ArchiveThreadID = 77
		want.ArchiveEnabled = true
		if got, err := Get(ctx, chatID); err != nil || got != want {
			t.Fatalf("after SetArchiveTarget: Get = %+v, %v, want %+v", got, err, want)
		}

		// Can disable and re-enable archiving while target remains linked.
		if err := SetArchiveEnabled(ctx, chatID, false, 33); err != nil {
			t.Fatal(err)
		}
		want.ArchiveEnabled = false
		if got, err := Get(ctx, chatID); err != nil || got != want {
			t.Fatalf("after SetArchiveEnabled(false): Get = %+v, %v, want %+v", got, err, want)
		}
		if err := SetArchiveEnabled(ctx, chatID, true, 34); err != nil {
			t.Fatal(err)
		}
		want.ArchiveEnabled = true
		if got, err := Get(ctx, chatID); err != nil || got != want {
			t.Fatalf("after SetArchiveEnabled(true): Get = %+v, %v, want %+v", got, err, want)
		}

		// ClearArchiveTarget zeroes target & disables archive without clobbering ConsensusEnabled or CatStickerEnabled.
		if err := ClearArchiveTarget(ctx, chatID, 35); err != nil {
			t.Fatal(err)
		}
		want.ArchiveChatID = 0
		want.ArchiveThreadID = 0
		want.ArchiveEnabled = false
		if got, err := Get(ctx, chatID); err != nil || got != want {
			t.Fatalf("after ClearArchiveTarget: Get = %+v, %v, want %+v", got, err, want)
		}

		// Now SetArchiveEnabled(true) must fail again because ArchiveChatID == 0.
		if err := SetArchiveEnabled(ctx, chatID, true, 36); !errors.Is(err, ErrInvalidValue) {
			t.Fatalf("SetArchiveEnabled(true) after ClearArchiveTarget = %v, want ErrInvalidValue", err)
		}
	})

	t.Run("SetArchiveThreshold, SetArchiveEmojiMode, SetArchiveDelivery validation and CHECK constraints", func(t *testing.T) {
		utilsCache.SetupTestMemoryMarshaler(t)
		badChat := newChat(t)
		for _, bad := range []int{0, 1, -5, 51} {
			if ValidArchiveThreshold(bad) {
				t.Errorf("ValidArchiveThreshold(%d) = true, want false", bad)
			}
			if err := SetArchiveThreshold(ctx, badChat, bad, 1); !errors.Is(err, ErrInvalidValue) {
				t.Errorf("SetArchiveThreshold(%d) = %v, want ErrInvalidValue", bad, err)
			}
		}
		for _, bad := range []string{"", "negative", "ALL", "positive "} {
			if ValidArchiveEmojiMode(bad) {
				t.Errorf("ValidArchiveEmojiMode(%q) = true, want false", bad)
			}
			if err := SetArchiveEmojiMode(ctx, badChat, bad, 1); !errors.Is(err, ErrInvalidValue) {
				t.Errorf("SetArchiveEmojiMode(%q) = %v, want ErrInvalidValue", bad, err)
			}
		}
		for _, bad := range []string{"", "link", "COPY", "forward "} {
			if ValidArchiveDelivery(bad) {
				t.Errorf("ValidArchiveDelivery(%q) = true, want false", bad)
			}
			if err := SetArchiveDelivery(ctx, badChat, bad, 1); !errors.Is(err, ErrInvalidValue) {
				t.Errorf("SetArchiveDelivery(%q) = %v, want ErrInvalidValue", bad, err)
			}
		}
		if rowCount(t, badChat) != 0 {
			t.Fatal("invalid archive setters inserted a row")
		}

		chatID := newChat(t)
		if err := SetArchiveThreshold(ctx, chatID, 25, 40); err != nil {
			t.Fatal(err)
		}
		if err := SetArchiveEmojiMode(ctx, chatID, ArchiveEmojiModeAll, 41); err != nil {
			t.Fatal(err)
		}
		if err := SetArchiveDelivery(ctx, chatID, ArchiveDeliveryForward, 42); err != nil {
			t.Fatal(err)
		}
		want := Defaults()
		want.ArchiveThreshold = 25
		want.ArchiveEmojiMode = ArchiveEmojiModeAll
		want.ArchiveDelivery = ArchiveDeliveryForward
		if got, err := Get(ctx, chatID); err != nil || got != want {
			t.Fatalf("after archive option setters: Get = %+v, %v, want %+v", got, err, want)
		}

		// Raw DB inserts violating CHECK constraints must fail.
		for _, tc := range []struct {
			name string
			row  *models.ChatPresence
		}{
			{"archive_threshold=1", &models.ChatPresence{
				ChatID: newChat(t), ConsensusThreshold: 3, RetractEnabled: true, AIReactMode: AIReactModeOff,
				ArchiveThreshold: 1, ArchiveEmojiMode: ArchiveEmojiModePositive, ArchiveDelivery: ArchiveDeliveryCopy,
			}},
			{"archive_threshold=51", &models.ChatPresence{
				ChatID: newChat(t), ConsensusThreshold: 3, RetractEnabled: true, AIReactMode: AIReactModeOff,
				ArchiveThreshold: 51, ArchiveEmojiMode: ArchiveEmojiModePositive, ArchiveDelivery: ArchiveDeliveryCopy,
			}},
			{"archive_emoji_mode=bad", &models.ChatPresence{
				ChatID: newChat(t), ConsensusThreshold: 3, RetractEnabled: true, AIReactMode: AIReactModeOff,
				ArchiveThreshold: 5, ArchiveEmojiMode: "negative", ArchiveDelivery: ArchiveDeliveryCopy,
			}},
			{"archive_delivery=bad", &models.ChatPresence{
				ChatID: newChat(t), ConsensusThreshold: 3, RetractEnabled: true, AIReactMode: AIReactModeOff,
				ArchiveThreshold: 5, ArchiveEmojiMode: ArchiveEmojiModePositive, ArchiveDelivery: "link",
			}},
		} {
			if err := db.DB.Create(tc.row).Error; err == nil {
				t.Errorf("%s bypassed CHECK constraint", tc.name)
			}
		}
	})

	t.Run("rolling-deploy cache normalization for pre-Phase-33 cached entries", func(t *testing.T) {
		utilsCache.SetupTestMemoryMarshaler(t)
		type prePhase33Settings struct {
			ConsensusEnabled   bool   `msgpack:"ce"`
			ConsensusThreshold int    `msgpack:"ct"`
			RetractEnabled     bool   `msgpack:"re"`
			AIReactMode        string `msgpack:"ar"`
		}
		chatID := newChat(t)
		if err := utilsCache.GetMarshal().Set(ctx, CacheKey(chatID), prePhase33Settings{
			ConsensusEnabled:   true,
			ConsensusThreshold: 4,
			RetractEnabled:     false,
			AIReactMode:        AIReactModeLow,
		}); err != nil {
			t.Fatalf("seed pre-Phase-33 cache: %v", err)
		}
		want := Settings{
			ConsensusEnabled:   true,
			ConsensusThreshold: 4,
			RetractEnabled:     false,
			AIReactMode:        AIReactModeLow,
			CatStickerEnabled:  false,
			ArchiveEnabled:     false,
			ArchiveChatID:      0,
			ArchiveThreadID:    0,
			ArchiveThreshold:   DefaultArchiveThreshold,
			ArchiveEmojiMode:   ArchiveEmojiModePositive,
			ArchiveDelivery:    ArchiveDeliveryCopy,
		}
		if got, err := Get(ctx, chatID); err != nil || got != want {
			t.Fatalf("pre-Phase-33 cached Get = %+v, %v, want %+v", got, err, want)
		}
	})

	t.Run("TryClaimArchived enforces once-only claim per (chat_id, message_id)", func(t *testing.T) {
		chatID := newChat(t)

		if claimed, err := TryClaimArchived(ctx, 0, 10); claimed || !errors.Is(err, ErrInvalidValue) {
			t.Fatalf("TryClaimArchived(0, 10) = (%v, %v), want (false, ErrInvalidValue)", claimed, err)
		}
		if claimed, err := TryClaimArchived(ctx, chatID, 0); claimed || !errors.Is(err, ErrInvalidValue) {
			t.Fatalf("TryClaimArchived(chatID, 0) = (%v, %v), want (false, ErrInvalidValue)", claimed, err)
		}

		claimed, err := TryClaimArchived(ctx, chatID, 101)
		if err != nil || !claimed {
			t.Fatalf("first TryClaimArchived(101) = (%v, %v), want (true, nil)", claimed, err)
		}
		claimedDup, err := TryClaimArchived(ctx, chatID, 101)
		if err != nil || claimedDup {
			t.Fatalf("duplicate TryClaimArchived(101) = (%v, %v), want (false, nil)", claimedDup, err)
		}
		claimedOther, err := TryClaimArchived(ctx, chatID, 102)
		if err != nil || !claimedOther {
			t.Fatalf("distinct msg TryClaimArchived(102) = (%v, %v), want (true, nil)", claimedOther, err)
		}
	})

	t.Run("ReplacePortableTx and ClearTx preserve archive target and skip unlinked archiving", func(t *testing.T) {
		utilsCache.SetupTestMemoryMarshaler(t)

		// Target without archive target: ArchiveEnabled=true is forced false and archiveSkipped=true.
		unlinkedChat := newChat(t)
		var skipped bool
		err := db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var txErr error
			skipped, txErr = ReplacePortableTx(tx, unlinkedChat, Settings{
				ConsensusEnabled:   true,
				ConsensusThreshold: 99, // normalizes to DefaultConsensusThreshold
				RetractEnabled:     false,
				AIReactMode:        "invalid", // normalizes to off
				CatStickerEnabled:  true,
				ArchiveEnabled:     true,
				ArchiveChatID:      -100123456, // must be ignored/stripped
				ArchiveThreadID:    99,         // must be ignored/stripped
				ArchiveThreshold:   1,          // normalizes to DefaultArchiveThreshold
				ArchiveEmojiMode:   "",         // normalizes to positive
				ArchiveDelivery:    "bad",      // normalizes to copy
			}, 50)
			return txErr
		})
		if err != nil {
			t.Fatalf("ReplacePortableTx unlinked: %v", err)
		}
		if !skipped {
			t.Fatal("expected archiveSkipped=true when target chat has no archive target")
		}
		gotUnlinked, err := GetFresh(ctx, unlinkedChat)
		if err != nil {
			t.Fatal(err)
		}
		wantUnlinked := Settings{
			ConsensusEnabled:   true,
			ConsensusThreshold: DefaultConsensusThreshold,
			RetractEnabled:     false,
			AIReactMode:        AIReactModeOff,
			CatStickerEnabled:  true,
			ArchiveEnabled:     false,
			ArchiveChatID:      0,
			ArchiveThreadID:    0,
			ArchiveThreshold:   DefaultArchiveThreshold,
			ArchiveEmojiMode:   ArchiveEmojiModePositive,
			ArchiveDelivery:    ArchiveDeliveryCopy,
		}
		if gotUnlinked != wantUnlinked {
			t.Fatalf("unlinked after ReplacePortableTx = %+v, want %+v", gotUnlinked, wantUnlinked)
		}

		// Target with existing archive target preserves archive_chat_id and archive_thread_id.
		linkedChat := newChat(t)
		if err := SetArchiveTarget(ctx, linkedChat, -100777666, 12, 51); err != nil {
			t.Fatal(err)
		}
		err = db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var txErr error
			skipped, txErr = ReplacePortableTx(tx, linkedChat, Settings{
				ConsensusEnabled:   true,
				ConsensusThreshold: 6,
				RetractEnabled:     false,
				AIReactMode:        AIReactModeMedium,
				CatStickerEnabled:  true,
				ArchiveEnabled:     true,
				ArchiveChatID:      -100999999, // must NOT overwrite existing target
				ArchiveThreadID:    555,        // must NOT overwrite existing thread
				ArchiveThreshold:   15,
				ArchiveEmojiMode:   ArchiveEmojiModeAll,
				ArchiveDelivery:    ArchiveDeliveryForward,
			}, 52)
			return txErr
		})
		if err != nil {
			t.Fatalf("ReplacePortableTx linked: %v", err)
		}
		if skipped {
			t.Fatal("expected archiveSkipped=false when target chat already has archive target")
		}
		gotLinked, err := GetFresh(ctx, linkedChat)
		if err != nil {
			t.Fatal(err)
		}
		wantLinked := Settings{
			ConsensusEnabled:   true,
			ConsensusThreshold: 6,
			RetractEnabled:     false,
			AIReactMode:        AIReactModeMedium,
			CatStickerEnabled:  true,
			ArchiveEnabled:     true,
			ArchiveChatID:      -100777666,
			ArchiveThreadID:    12,
			ArchiveThreshold:   15,
			ArchiveEmojiMode:   ArchiveEmojiModeAll,
			ArchiveDelivery:    ArchiveDeliveryForward,
		}
		if gotLinked != wantLinked {
			t.Fatalf("linked after ReplacePortableTx = %+v, want %+v", gotLinked, wantLinked)
		}

		// ClearTx deletes row so GetFresh returns Defaults().
		if err := db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			return ClearTx(tx, linkedChat)
		}); err != nil {
			t.Fatalf("ClearTx: %v", err)
		}
		if rowCount(t, linkedChat) != 0 {
			t.Fatal("ClearTx left chat_presence row behind")
		}
		afterClear, err := GetFresh(ctx, linkedChat)
		if err != nil {
			t.Fatal(err)
		}
		if afterClear != Defaults() {
			t.Fatalf("after ClearTx: GetFresh = %+v, want Defaults() %+v", afterClear, Defaults())
		}
	})
}
