//go:build testtools

package modtuning

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	utilsCache "github.com/uasneppy/fuku_robot/fuku/utils/cache"
)

var modTuningChatSeq atomic.Int64

func modTuningChat(t *testing.T) int64 {
	t.Helper()
	utilsCache.SetupTestMemoryMarshaler(t)
	chatID := -time.Now().UnixNano() - modTuningChatSeq.Add(1)
	t.Cleanup(func() {
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatManualDefaults{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatReasonShortcut{})
		cache.DeleteCache(DefaultsCacheKey(chatID))
		cache.DeleteCache(ShortcutsCacheKey(chatID))
		cache.ResetLocalForTest()
	})
	return chatID
}

func defaultsRowCount(t *testing.T, chatID int64) int64 {
	t.Helper()
	var n int64
	if err := db.DB.Model(&models.ChatManualDefaults{}).Where("chat_id = ?", chatID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func TestManualDefaultsAndShortcutsCRUD(t *testing.T) {
	ctx := context.Background()

	t.Run("unconfigured chat returns zero defaults and creates no rows", func(t *testing.T) {
		chatID := modTuningChat(t)
		got, err := GetDefaults(ctx, chatID)
		if err != nil {
			t.Fatalf("GetDefaults: %v", err)
		}
		if got != (Defaults{}) {
			t.Fatalf("GetDefaults = %+v, want zero", got)
		}
		fresh, err := GetDefaultsFresh(ctx, chatID)
		if err != nil || fresh != (Defaults{}) {
			t.Fatalf("GetDefaultsFresh = %+v, %v; want zero, nil", fresh, err)
		}
		if defaultsRowCount(t, chatID) != 0 {
			t.Fatal("reading an unconfigured chat created a chat_manual_defaults row")
		}

		shortcuts, err := GetShortcuts(ctx, chatID)
		if err != nil {
			t.Fatalf("GetShortcuts: %v", err)
		}
		if len(shortcuts) != 0 {
			t.Fatalf("GetShortcuts = %v, want empty", shortcuts)
		}
		count, err := CountShortcutsFresh(ctx, chatID)
		if err != nil || count != 0 {
			t.Fatalf("CountShortcutsFresh = %d, %v; want 0, nil", count, err)
		}
	})

	t.Run("column-targeted upserts preserve sibling columns, invalidate cache, and round-trip bool false", func(t *testing.T) {
		chatID := modTuningChat(t)
		// Prime cache with zero defaults.
		if _, err := GetDefaults(ctx, chatID); err != nil {
			t.Fatal(err)
		}

		if err := SetDefaultBanSecs(ctx, chatID, 3600, 11); err != nil {
			t.Fatalf("SetDefaultBanSecs: %v", err)
		}
		got, err := GetDefaults(ctx, chatID)
		if err != nil {
			t.Fatal(err)
		}
		if got.DefaultBanSecs != 3600 || got.DefaultMuteSecs != 0 || got.ReasonRequired || got.AppealURL != "" {
			t.Fatalf("after SetDefaultBanSecs: %+v", got)
		}

		if err := SetDefaultMuteSecs(ctx, chatID, 1800, 12); err != nil {
			t.Fatalf("SetDefaultMuteSecs: %v", err)
		}
		if err := SetReasonRequired(ctx, chatID, true, 13); err != nil {
			t.Fatalf("SetReasonRequired(true): %v", err)
		}
		if err := SetAppealURL(ctx, chatID, "https://example.org/appeal", 14); err != nil {
			t.Fatalf("SetAppealURL: %v", err)
		}

		got, err = GetDefaults(ctx, chatID)
		if err != nil {
			t.Fatal(err)
		}
		want := Defaults{
			DefaultBanSecs:  3600,
			DefaultMuteSecs: 1800,
			ReasonRequired:  true,
			AppealURL:       "https://example.org/appeal",
		}
		if got != want {
			t.Fatalf("after all writes = %+v, want %+v", got, want)
		}

		// Round-trip reason_required true -> false (catches any GORM default: tag bug).
		if err := SetReasonRequired(ctx, chatID, false, 15); err != nil {
			t.Fatalf("SetReasonRequired(false): %v", err)
		}
		got, err = GetDefaults(ctx, chatID)
		if err != nil {
			t.Fatal(err)
		}
		if got.ReasonRequired {
			t.Fatal("ReasonRequired stayed true after SetReasonRequired(false)")
		}
		if got.DefaultBanSecs != 3600 || got.DefaultMuteSecs != 1800 || got.AppealURL != "https://example.org/appeal" {
			t.Fatalf("SetReasonRequired(false) clobbered sibling columns: %+v", got)
		}

		// Clear ban duration back to 0 and appeal URL back to "".
		if err := SetDefaultBanSecs(ctx, chatID, 0, 16); err != nil {
			t.Fatalf("SetDefaultBanSecs(0): %v", err)
		}
		if err := SetAppealURL(ctx, chatID, "", 17); err != nil {
			t.Fatalf("SetAppealURL(empty): %v", err)
		}
		got, err = GetDefaults(ctx, chatID)
		if err != nil {
			t.Fatal(err)
		}
		if got.DefaultBanSecs != 0 || got.DefaultMuteSecs != 1800 || got.ReasonRequired || got.AppealURL != "" {
			t.Fatalf("after clears = %+v", got)
		}
	})

	t.Run("concurrent writes to different columns never clobber each other", func(t *testing.T) {
		chatID := modTuningChat(t)
		if err := SetReasonRequired(ctx, chatID, true, 1); err != nil {
			t.Fatal(err)
		}
		writers := []func() error{
			func() error { return SetDefaultBanSecs(ctx, chatID, 7200, 2) },
			func() error { return SetDefaultMuteSecs(ctx, chatID, 600, 3) },
			func() error { return SetAppealURL(ctx, chatID, "https://example.com/help", 4) },
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
		got, err := GetDefaultsFresh(ctx, chatID)
		if err != nil {
			t.Fatal(err)
		}
		want := Defaults{
			DefaultBanSecs:  7200,
			DefaultMuteSecs: 600,
			ReasonRequired:  true,
			AppealURL:       "https://example.com/help",
		}
		if got != want {
			t.Fatalf("after concurrent writes = %+v, want %+v", got, want)
		}
	})

	t.Run("invalid durations, appeal URL, and shortcut values are rejected", func(t *testing.T) {
		chatID := modTuningChat(t)
		for _, bad := range []int{-1, 1, 59, models.PunishMaxSecs + 1} {
			if err := SetDefaultBanSecs(ctx, chatID, bad, 1); !errors.Is(err, ErrInvalidValue) {
				t.Errorf("SetDefaultBanSecs(%d) = %v, want ErrInvalidValue", bad, err)
			}
			if err := SetDefaultMuteSecs(ctx, chatID, bad, 1); !errors.Is(err, ErrInvalidValue) {
				t.Errorf("SetDefaultMuteSecs(%d) = %v, want ErrInvalidValue", bad, err)
			}
		}
		tooLongURL := "https://example.org/" + strings.Repeat("a", models.ManualAppealURLMaxRunes)
		if err := SetAppealURL(ctx, chatID, tooLongURL, 1); !errors.Is(err, ErrInvalidValue) {
			t.Errorf("SetAppealURL(too long) = %v, want ErrInvalidValue", err)
		}
		if defaultsRowCount(t, chatID) != 0 {
			t.Fatal("invalid writes created a row")
		}

		if added, err := AddShortcut(ctx, chatID, "", "reason", 1); added || !errors.Is(err, ErrInvalidValue) {
			t.Errorf("AddShortcut(empty word) = %v, %v; want false, ErrInvalidValue", added, err)
		}
		if added, err := AddShortcut(ctx, chatID, strings.Repeat("w", models.ReasonShortcutWordMaxRunes+1), "reason", 1); added || !errors.Is(err, ErrInvalidValue) {
			t.Errorf("AddShortcut(long word) = %v, %v; want false, ErrInvalidValue", added, err)
		}
		if added, err := AddShortcut(ctx, chatID, "spam", "", 1); added || !errors.Is(err, ErrInvalidValue) {
			t.Errorf("AddShortcut(empty reason) = %v, %v; want false, ErrInvalidValue", added, err)
		}
		if added, err := AddShortcut(ctx, chatID, "spam", strings.Repeat("r", models.ReasonShortcutTextMaxRunes+1), 1); added || !errors.Is(err, ErrInvalidValue) {
			t.Errorf("AddShortcut(long reason) = %v, %v; want false, ErrInvalidValue", added, err)
		}
	})

	t.Run("shortcuts CRUD, duplicate rejection, cache invalidation, and foreign-chat isolation", func(t *testing.T) {
		chatA := modTuningChat(t)
		chatB := modTuningChat(t)

		// Prime cache for chatA with empty map.
		if m, err := GetShortcuts(ctx, chatA); err != nil || len(m) != 0 {
			t.Fatalf("initial GetShortcuts = %v, %v", m, err)
		}

		added, err := AddShortcut(ctx, chatA, "spam", "Unsolicited advertising", 10)
		if err != nil || !added {
			t.Fatalf("AddShortcut(spam) = %v, %v; want true, nil", added, err)
		}
		added, err = AddShortcut(ctx, chatA, "flood", "Repeated messages", 10)
		if err != nil || !added {
			t.Fatalf("AddShortcut(flood) = %v, %v; want true, nil", added, err)
		}

		// Duplicate word must not overwrite existing reason.
		added, err = AddShortcut(ctx, chatA, "spam", "Overwritten text", 11)
		if err != nil || added {
			t.Fatalf("duplicate AddShortcut(spam) = %v, %v; want false, nil", added, err)
		}

		// Cached read sees newly added entries with original "spam" text intact.
		m, err := GetShortcuts(ctx, chatA)
		if err != nil {
			t.Fatalf("GetShortcuts: %v", err)
		}
		if m["spam"] != "Unsolicited advertising" || m["flood"] != "Repeated messages" || len(m) != 2 {
			t.Fatalf("GetShortcuts = %v", m)
		}

		exists, err := ExistsShortcutFresh(ctx, chatA, "spam")
		if err != nil || !exists {
			t.Fatalf("ExistsShortcutFresh(spam) = %v, %v; want true, nil", exists, err)
		}
		exists, err = ExistsShortcutFresh(ctx, chatA, "missing")
		if err != nil || exists {
			t.Fatalf("ExistsShortcutFresh(missing) = %v, %v; want false, nil", exists, err)
		}
		count, err := CountShortcutsFresh(ctx, chatA)
		if err != nil || count != 2 {
			t.Fatalf("CountShortcutsFresh = %d, %v; want 2, nil", count, err)
		}

		entries, err := ListShortcutsFresh(ctx, chatA)
		if err != nil || len(entries) != 2 {
			t.Fatalf("ListShortcutsFresh = %+v, %v", entries, err)
		}
		if entries[0].Word != "spam" || entries[1].Word != "flood" || entries[0].ID >= entries[1].ID {
			t.Fatalf("unexpected ordering: %+v", entries)
		}

		// GetShortcutFresh and RemoveShortcutByID refuse foreign-chat IDs.
		foreignEntry, err := GetShortcutFresh(ctx, chatB, entries[0].ID)
		if err != nil || foreignEntry != nil {
			t.Fatalf("GetShortcutFresh(foreign) = %+v, %v; want nil, nil", foreignEntry, err)
		}
		word, removed, err := RemoveShortcutByID(ctx, chatB, entries[0].ID)
		if err != nil || removed || word != "" {
			t.Fatalf("RemoveShortcutByID(foreign) = %q, %v, %v; want '', false, nil", word, removed, err)
		}

		// Own-chat lookup and delete succeed and invalidate cache.
		ownEntry, err := GetShortcutFresh(ctx, chatA, entries[0].ID)
		if err != nil || ownEntry == nil || ownEntry.Word != "spam" {
			t.Fatalf("GetShortcutFresh(own) = %+v, %v", ownEntry, err)
		}
		word, removed, err = RemoveShortcutByID(ctx, chatA, entries[0].ID)
		if err != nil || !removed || word != "spam" {
			t.Fatalf("RemoveShortcutByID(own) = %q, %v, %v; want spam, true, nil", word, removed, err)
		}
		// Deleting a second time is a clean no-op.
		word, removed, err = RemoveShortcutByID(ctx, chatA, entries[0].ID)
		if err != nil || removed || word != "" {
			t.Fatalf("second RemoveShortcutByID = %q, %v, %v; want '', false, nil", word, removed, err)
		}

		m, err = GetShortcuts(ctx, chatA)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := m["spam"]; ok || m["flood"] != "Repeated messages" || len(m) != 1 {
			t.Fatalf("GetShortcuts after remove = %v", m)
		}
	})
}
