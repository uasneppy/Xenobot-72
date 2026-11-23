//go:build testtools

package advthresholds

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	utilsCache "github.com/uasneppy/fuku_robot/fuku/utils/cache"
)

var advThresholdsChatSeq atomic.Int64

func advThresholdsChat(t *testing.T) int64 {
	t.Helper()
	utilsCache.SetupTestMemoryMarshaler(t)
	chatID := -time.Now().UnixNano() - advThresholdsChatSeq.Add(1)
	t.Cleanup(func() {
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatAdvancedThresholds{})
		cache.DeleteCache(CacheKey(chatID))
		cache.ResetLocalForTest()
	})
	return chatID
}

func advThresholdsRowCount(t *testing.T, chatID int64) int64 {
	t.Helper()
	var n int64
	if err := db.DB.Model(&models.ChatAdvancedThresholds{}).Where("chat_id = ?", chatID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func TestAdvancedThresholdsCRUD(t *testing.T) {
	ctx := context.Background()

	t.Run("unconfigured chat returns zero thresholds and creates no rows", func(t *testing.T) {
		chatID := advThresholdsChat(t)
		got, err := Get(ctx, chatID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got != (Thresholds{}) {
			t.Fatalf("Get = %+v, want zero", got)
		}
		if got.CustomizedCount() != 0 {
			t.Fatalf("CustomizedCount = %d, want 0", got.CustomizedCount())
		}
		fresh, err := GetFresh(ctx, chatID)
		if err != nil || fresh != (Thresholds{}) {
			t.Fatalf("GetFresh = %+v, %v; want zero, nil", fresh, err)
		}
		if advThresholdsRowCount(t, chatID) != 0 {
			t.Fatal("reading an unconfigured chat created a chat_advanced_thresholds row")
		}
	})

	t.Run("all 12 fields accept min and max bounds and reject invalid values without modifying DB", func(t *testing.T) {
		specs := AllSpecs()
		if len(specs) != 12 {
			t.Fatalf("AllSpecs len = %d, want 12", len(specs))
		}
		if len(AllAreas()) != 6 {
			t.Fatalf("AllAreas len = %d, want 6", len(AllAreas()))
		}

		invalidChat := advThresholdsChat(t)
		if err := SetField(ctx, invalidChat, Field("unknown"), 10, 1); !errors.Is(err, ErrInvalidField) {
			t.Errorf("SetField(unknown) = %v, want ErrInvalidField", err)
		}
		if err := ResetField(ctx, invalidChat, Field("unknown"), 1); !errors.Is(err, ErrInvalidField) {
			t.Errorf("ResetField(unknown) = %v, want ErrInvalidField", err)
		}
		if err := ResetArea(ctx, invalidChat, Area("unknown"), 1); !errors.Is(err, ErrInvalidArea) {
			t.Errorf("ResetArea(unknown) = %v, want ErrInvalidArea", err)
		}
		for _, spec := range specs {
			badValues := []int{-1, spec.Max + 1}
			if spec.Min > 1 {
				badValues = append(badValues, spec.Min-1)
			}
			for _, bad := range badValues {
				if err := SetField(ctx, invalidChat, spec.Field, bad, 1); !errors.Is(err, ErrInvalidValue) {
					t.Errorf("SetField(%s, %d) = %v, want ErrInvalidValue", spec.Field, bad, err)
				}
			}
		}
		if advThresholdsRowCount(t, invalidChat) != 0 {
			t.Fatal("invalid writes created a chat_advanced_thresholds row")
		}

		chatID := advThresholdsChat(t)
		for _, spec := range specs {
			if err := SetField(ctx, chatID, spec.Field, spec.Min, 10); err != nil {
				t.Fatalf("SetField(%s, min=%d): %v", spec.Field, spec.Min, err)
			}
			got, err := GetFresh(ctx, chatID)
			if err != nil {
				t.Fatalf("GetFresh after min(%s): %v", spec.Field, err)
			}
			if got.Value(spec.Field) != spec.Min || !got.IsCustomized(spec.Field) {
				t.Errorf("%s after min = %d (customized=%v), want %d (true)", spec.Field, got.Value(spec.Field), got.IsCustomized(spec.Field), spec.Min)
			}

			if err := SetField(ctx, chatID, spec.Field, spec.Max, 11); err != nil {
				t.Fatalf("SetField(%s, max=%d): %v", spec.Field, spec.Max, err)
			}
			got, err = GetFresh(ctx, chatID)
			if err != nil {
				t.Fatalf("GetFresh after max(%s): %v", spec.Field, err)
			}
			if got.Value(spec.Field) != spec.Max {
				t.Errorf("%s after max = %d, want %d", spec.Field, got.Value(spec.Field), spec.Max)
			}
		}
		got, err := GetFresh(ctx, chatID)
		if err != nil {
			t.Fatal(err)
		}
		if got.CustomizedCount() != 12 {
			t.Fatalf("CustomizedCount after setting all 12 fields = %d, want 12", got.CustomizedCount())
		}
	})

	t.Run("column-targeted upserts preserve siblings, reset field/area/all, and invalidate cache", func(t *testing.T) {
		chatID := advThresholdsChat(t)
		// Prime cache with zero thresholds.
		if _, err := Get(ctx, chatID); err != nil {
			t.Fatal(err)
		}

		if err := SetField(ctx, chatID, FieldRaidJoinWindow, 120, 21); err != nil {
			t.Fatalf("SetField(jsw): %v", err)
		}
		got, err := Get(ctx, chatID)
		if err != nil {
			t.Fatal(err)
		}
		if got.RaidJoinWindowSecs != 120 || got.CustomizedCount() != 1 {
			t.Fatalf("after jsw write (cached read) = %+v", got)
		}

		if err := SetField(ctx, chatID, FieldRaidFloodWindow, 90, 22); err != nil {
			t.Fatalf("SetField(flw): %v", err)
		}
		if err := SetField(ctx, chatID, FieldRaidAIWindow, 300, 23); err != nil {
			t.Fatalf("SetField(aiw): %v", err)
		}
		if err := SetField(ctx, chatID, FieldLockdownReminder, 30, 24); err != nil {
			t.Fatalf("SetField(lrm): %v", err)
		}
		if err := SetField(ctx, chatID, FieldCaptchaTimeout, 20, 25); err != nil {
			t.Fatalf("SetField(ctm): %v", err)
		}

		got, err = Get(ctx, chatID)
		if err != nil {
			t.Fatal(err)
		}
		if got.RaidJoinWindowSecs != 120 || got.RaidFloodWindowSecs != 90 || got.RaidAIWindowSecs != 300 ||
			got.LockdownReminderMins != 30 || got.CaptchaTimeoutMins != 20 {
			t.Fatalf("after sibling writes = %+v", got)
		}
		if got.AreaCustomizedCount(AreaRaidWindows) != 3 || got.AreaCustomizedCount(AreaLockdownReminder) != 1 || got.CustomizedCount() != 5 {
			t.Fatalf("counts: rw=%d lr=%d total=%d", got.AreaCustomizedCount(AreaRaidWindows), got.AreaCustomizedCount(AreaLockdownReminder), got.CustomizedCount())
		}

		// ResetField zeroes only that single column.
		if err := ResetField(ctx, chatID, FieldRaidJoinWindow, 26); err != nil {
			t.Fatalf("ResetField(jsw): %v", err)
		}
		got, err = Get(ctx, chatID)
		if err != nil {
			t.Fatal(err)
		}
		if got.RaidJoinWindowSecs != 0 || got.RaidFloodWindowSecs != 90 || got.RaidAIWindowSecs != 300 ||
			got.LockdownReminderMins != 30 || got.CaptchaTimeoutMins != 20 {
			t.Fatalf("after ResetField(jsw) = %+v", got)
		}

		// ResetArea zeroes every column in AreaRaidWindows while preserving other areas.
		if err := ResetArea(ctx, chatID, AreaRaidWindows, 27); err != nil {
			t.Fatalf("ResetArea(rw): %v", err)
		}
		got, err = Get(ctx, chatID)
		if err != nil {
			t.Fatal(err)
		}
		if got.RaidJoinWindowSecs != 0 || got.RaidFloodWindowSecs != 0 || got.RaidAIWindowSecs != 0 {
			t.Fatalf("ResetArea(rw) did not clear all rw columns: %+v", got)
		}
		if got.LockdownReminderMins != 30 || got.CaptchaTimeoutMins != 20 {
			t.Fatalf("ResetArea(rw) clobbered other areas: %+v", got)
		}

		// ResetAll removes the row and invalidates cache.
		if err := ResetAll(ctx, chatID); err != nil {
			t.Fatalf("ResetAll: %v", err)
		}
		if advThresholdsRowCount(t, chatID) != 0 {
			t.Fatal("ResetAll left a chat_advanced_thresholds row behind")
		}
		got, err = Get(ctx, chatID)
		if err != nil {
			t.Fatal(err)
		}
		if got != (Thresholds{}) {
			t.Fatalf("after ResetAll Get = %+v, want zero", got)
		}
	})

	t.Run("concurrent writes to different columns never clobber each other", func(t *testing.T) {
		chatID := advThresholdsChat(t)
		if err := SetField(ctx, chatID, FieldRaidRecentHours, 12, 1); err != nil {
			t.Fatal(err)
		}
		writers := []func() error{
			func() error { return SetField(ctx, chatID, FieldRaidJoinWindow, 45, 2) },
			func() error { return SetField(ctx, chatID, FieldRaidFloodNewPercent, 50, 3) },
			func() error { return SetField(ctx, chatID, FieldMemberReqCap, 25, 4) },
			func() error { return SetField(ctx, chatID, FieldCaptchaTimeout, 15, 5) },
		}
		var wg sync.WaitGroup
		errs := make([]error, len(writers))
		for i, fn := range writers {
			wg.Add(1)
			go func(idx int, f func() error) {
				defer wg.Done()
				errs[idx] = f()
			}(i, fn)
		}
		wg.Wait()
		for i, err := range errs {
			if err != nil {
				if db.DB.Name() == "postgres" {
					t.Fatalf("concurrent writer %d: %v", i, err)
				}
				if retryErr := writers[i](); retryErr != nil {
					t.Fatalf("retry writer %d: %v", i, retryErr)
				}
			}
		}
		got, err := GetFresh(ctx, chatID)
		if err != nil {
			t.Fatal(err)
		}
		want := Thresholds{
			RaidJoinWindowSecs:  45,
			RaidFloodNewPercent: 50,
			RaidRecentHours:     12,
			MemberReqCapPerMin:  25,
			CaptchaTimeoutMins:  15,
		}
		if got != want {
			t.Fatalf("after concurrent writes = %+v, want %+v", got, want)
		}
	})
}
