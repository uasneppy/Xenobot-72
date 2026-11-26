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
		want := Settings{ConsensusEnabled: true, ConsensusThreshold: 3, RetractEnabled: true}
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
		wantNew := Settings{ConsensusEnabled: false, ConsensusThreshold: 3, RetractEnabled: false}
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
			want := Settings{ConsensusEnabled: true, ConsensusThreshold: v, RetractEnabled: false}
			if got, err := Get(ctx, chatID); err != nil || got != want {
				t.Fatalf("after SetConsensusThreshold(%d): Get = %+v, %v, want %+v", v, got, err, want)
			}
			if fresh, err := GetFresh(ctx, chatID); err != nil || fresh != want {
				t.Fatalf("after SetConsensusThreshold(%d): GetFresh = %+v, %v, want %+v", v, fresh, err, want)
			}
		}
	})

	t.Run("cache invalidation on writes and DB CHECK constraint enforcement", func(t *testing.T) {
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
		badLow := &models.ChatPresence{ChatID: newChat(t), ConsensusThreshold: 1, RetractEnabled: true}
		if err := db.DB.Create(badLow).Error; err == nil {
			t.Error("threshold 1 bypassed chk_chat_presence_consensus_threshold")
		}
		badHigh := &models.ChatPresence{ChatID: newChat(t), ConsensusThreshold: 21, RetractEnabled: true}
		if err := db.DB.Create(badHigh).Error; err == nil {
			t.Error("threshold 21 bypassed chk_chat_presence_consensus_threshold")
		}
	})
}
