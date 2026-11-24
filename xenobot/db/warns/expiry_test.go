package warns

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/db/chats"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/db/user"
)

// storedWarns reads the member's warns_users row straight from the table.
func storedWarns(t *testing.T, userID, chatID int64) models.Warns {
	t.Helper()
	var row models.Warns
	if err := db.DB.Where("user_id = ? AND chat_id = ?", userID, chatID).First(&row).Error; err != nil {
		t.Fatalf("read the warns_users row of user %d in chat %d: %v", userID, chatID, err)
	}
	return row
}

// TestWarnExpiryWrites proves the dated warn writes: one time per warn, the warns that no longer
// count dropped at the next write while expiry is on and never with it off, the newest counted warn
// removed, the reset's answer, one count per warn under concurrent writes, the expiry upsert and its
// cache drop, and the counted read after a moved clock. It runs on SQLite in make test and on
// PostgreSQL in the integrity run.
func TestWarnExpiryWrites(t *testing.T) {
	skipIfNoDb(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)
	daysAgo := func(d int) time.Time { return now.Add(-time.Duration(d) * 24 * time.Hour) }

	member := func(t *testing.T, expiry int) (chatID, userID int64) {
		t.Helper()
		chatID = -time.Now().UnixNano()
		userID = time.Now().UnixNano()%1_000_000_000 + 7_000_000_000
		if err := chats.EnsureChatInDb(chatID, "warn-expiry"); err != nil {
			t.Fatalf("EnsureChatInDb() error = %v", err)
		}
		if err := user.EnsureUserInDb(userID, "warn-expiry", "Expiry"); err != nil {
			t.Fatalf("EnsureUserInDb() error = %v", err)
		}
		t.Cleanup(func() {
			db.DB.Where("chat_id = ?", chatID).Delete(&models.Warns{})
			db.DB.Where("chat_id = ?", chatID).Delete(&models.WarnSettings{})
			db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{})
			db.DB.Where("user_id = ?", userID).Delete(&models.User{})
			cache.DeleteCache(cache.CacheKey("warn_settings", chatID))
			cache.DeleteCache(cache.CacheKey("warns", userID, chatID))
		})
		if err := SetWarnExpiryContext(ctx, chatID, expiry); err != nil {
			t.Fatalf("SetWarnExpiryContext(%d) error = %v", expiry, err)
		}
		return chatID, userID
	}
	warnAt := func(t *testing.T, userID, chatID int64, reason string, at time.Time) (int, []string) {
		t.Helper()
		n, reasons, err := WarnUserAt(ctx, userID, chatID, reason, at)
		if err != nil {
			t.Fatalf("WarnUserAt(%s) error = %v", reason, err)
		}
		return n, reasons
	}

	t.Run("one time per warn", func(t *testing.T) {
		chatID, userID := member(t, 0)
		warnAt(t, userID, chatID, "a", daysAgo(2))
		warnAt(t, userID, chatID, "b", daysAgo(1))
		row := storedWarns(t, userID, chatID)
		want := models.Int64Array{daysAgo(2).Unix(), daysAgo(1).Unix()}
		if row.NumWarns != 2 || !reflect.DeepEqual(row.WarnTimes, want) {
			t.Fatalf("row = %d warns, times %v; want 2 and %v", row.NumWarns, row.WarnTimes, want)
		}
	})

	t.Run("expiry on drops an old warn at the next warn", func(t *testing.T) {
		chatID, userID := member(t, 7)
		warnAt(t, userID, chatID, "old", daysAgo(8))
		n, reasons := warnAt(t, userID, chatID, "new", now)
		if n != 1 || !reflect.DeepEqual(reasons, []string{"new"}) {
			t.Fatalf("WarnUserAt = %d %v, want 1 [new]", n, reasons)
		}
		row := storedWarns(t, userID, chatID)
		if row.NumWarns != 1 || !reflect.DeepEqual([]string(row.Reasons), []string{"new"}) || !reflect.DeepEqual(row.WarnTimes, models.Int64Array{now.Unix()}) {
			t.Fatalf("row = %d %v %v, want only the new warn", row.NumWarns, row.Reasons, row.WarnTimes)
		}
	})

	t.Run("expiry off never drops", func(t *testing.T) {
		chatID, userID := member(t, 0)
		warnAt(t, userID, chatID, "old", daysAgo(400))
		n, reasons := warnAt(t, userID, chatID, "new", now)
		if n != 2 || !reflect.DeepEqual(reasons, []string{"old", "new"}) {
			t.Fatalf("WarnUserAt = %d %v, want 2 [old new]", n, reasons)
		}
		removed, err := RemoveWarnAt(ctx, userID, chatID, now)
		if err != nil || !removed {
			t.Fatalf("RemoveWarnAt = %v %v, want true", removed, err)
		}
		if row := storedWarns(t, userID, chatID); row.NumWarns != 1 || !reflect.DeepEqual([]string(row.Reasons), []string{"old"}) {
			t.Fatalf("row = %d %v, want the old warn kept", row.NumWarns, row.Reasons)
		}
	})

	t.Run("remove takes the newest counted warn", func(t *testing.T) {
		chatID, userID := member(t, 7)
		warnAt(t, userID, chatID, "a", daysAgo(3))
		warnAt(t, userID, chatID, "b", daysAgo(1))
		removed, err := RemoveWarnAt(ctx, userID, chatID, now)
		if err != nil || !removed {
			t.Fatalf("RemoveWarnAt = %v %v, want true", removed, err)
		}
		if row := storedWarns(t, userID, chatID); row.NumWarns != 1 || !reflect.DeepEqual([]string(row.Reasons), []string{"a"}) || len(row.WarnTimes) != 1 {
			t.Fatalf("row = %d %v %v, want [a] with one time", row.NumWarns, row.Reasons, row.WarnTimes)
		}
	})

	t.Run("remove with every warn expired answers false and drops them", func(t *testing.T) {
		chatID, userID := member(t, 7)
		warnAt(t, userID, chatID, "a", daysAgo(9))
		warnAt(t, userID, chatID, "b", daysAgo(8))
		removed, err := RemoveWarnAt(ctx, userID, chatID, now)
		if err != nil || removed {
			t.Fatalf("RemoveWarnAt = %v %v, want false", removed, err)
		}
		if row := storedWarns(t, userID, chatID); row.NumWarns != 0 || len(row.Reasons) != 0 || len(row.WarnTimes) != 0 {
			t.Fatalf("row = %d %v %v, want the expired warns dropped", row.NumWarns, row.Reasons, row.WarnTimes)
		}
	})

	t.Run("reset answers by the counted warns", func(t *testing.T) {
		chatID, userID := member(t, 7)
		warnAt(t, userID, chatID, "a", daysAgo(9))
		removed, err := ResetUserWarnsAt(ctx, userID, chatID, now)
		if err != nil || removed {
			t.Fatalf("ResetUserWarnsAt with only an expired warn = %v %v, want false", removed, err)
		}
		var n int64
		db.DB.Model(&models.Warns{}).Where("user_id = ? AND chat_id = ?", userID, chatID).Count(&n)
		if n != 0 {
			t.Fatalf("the expired row was kept (%d rows), want it deleted", n)
		}
		warnAt(t, userID, chatID, "b", now)
		if removed, err := ResetUserWarnsAt(ctx, userID, chatID, now); err != nil || !removed {
			t.Fatalf("ResetUserWarnsAt with a counted warn = %v %v, want true", removed, err)
		}

		offChat, offUser := member(t, 0)
		if err := db.DB.Create(&models.Warns{UserId: offUser, ChatId: offChat, NumWarns: 0, Reasons: models.StringArray{}}).Error; err != nil {
			t.Fatalf("seed an empty row: %v", err)
		}
		if removed, err := ResetUserWarnsAt(ctx, offUser, offChat, now); err != nil || !removed {
			t.Fatalf("ResetUserWarnsAt of an empty row with expiry off = %v %v, want true (as before)", removed, err)
		}
		if removed, err := ResetUserWarnsAt(ctx, offUser, offChat, now); err != nil || removed {
			t.Fatalf("ResetUserWarnsAt with no row = %v %v, want false", removed, err)
		}
	})

	t.Run("concurrent warns keep one count each", func(t *testing.T) {
		chatID, userID := member(t, 30)
		const writers = 8
		start := make(chan struct{})
		errs := make(chan error, writers)
		var wg sync.WaitGroup
		for i := range writers {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				_, _, err := WarnUserAt(ctx, userID, chatID, fmt.Sprintf("w%d", i), now)
				errs <- err
			}(i)
		}
		close(start)
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("concurrent WarnUserAt error = %v", err)
			}
		}
		if row := storedWarns(t, userID, chatID); row.NumWarns != writers || len(row.Reasons) != writers || len(row.WarnTimes) != writers {
			t.Fatalf("row = %d warns, %d reasons, %d times; want %d of each", row.NumWarns, len(row.Reasons), len(row.WarnTimes), writers)
		}
	})

	t.Run("the expiry upsert writes only its column and drops the key", func(t *testing.T) {
		mr := useMiniredisCache(t)
		chatID, _ := member(t, 0)
		if err := SetWarnLimit(chatID, 6); err != nil {
			t.Fatalf("SetWarnLimit() error = %v", err)
		}
		if err := SetWarnModeContext(ctx, chatID, "tban", 3600); err != nil {
			t.Fatalf("SetWarnModeContext() error = %v", err)
		}
		if _, err := GetWarnSettingErrContext(ctx, chatID); err != nil {
			t.Fatalf("warm the cache: %v", err)
		}
		if keys := warnSettingsKeys(mr, chatID); len(keys) == 0 {
			t.Fatal("the read cached no warn_settings key")
		}
		if err := SetWarnExpiryContext(ctx, chatID, 14); err != nil {
			t.Fatalf("SetWarnExpiryContext() error = %v", err)
		}
		if keys := warnSettingsKeys(mr, chatID); len(keys) != 0 {
			t.Fatalf("keys after the write = %v, want none", keys)
		}
		row := storedWarnSettings(t, chatID)
		if row.ExpiryDays != 14 || row.WarnLimit != 6 || row.WarnMode != "tban" || row.WarnModeSecs != 3600 {
			t.Fatalf("row = limit %d, mode %q, secs %d, expiry %d; want 6, tban, 3600, 14", row.WarnLimit, row.WarnMode, row.WarnModeSecs, row.ExpiryDays)
		}
		for _, bad := range []int{-1, models.WarnExpiryMaxDays + 1} {
			if err := SetWarnExpiryContext(ctx, chatID, bad); err != ErrWarnExpiryOutOfRange {
				t.Errorf("SetWarnExpiryContext(%d) error = %v, want ErrWarnExpiryOutOfRange", bad, err)
			}
		}
		if row := storedWarnSettings(t, chatID); row.ExpiryDays != 14 {
			t.Errorf("a refused value changed the expiry to %d", row.ExpiryDays)
		}
		missing := -time.Now().UnixNano()
		if err := chats.EnsureChatInDb(missing, "warn-expiry-new"); err != nil {
			t.Fatalf("EnsureChatInDb() error = %v", err)
		}
		t.Cleanup(func() {
			db.DB.Where("chat_id = ?", missing).Delete(&models.WarnSettings{})
			db.DB.Where("chat_id = ?", missing).Delete(&models.Chat{})
		})
		if err := SetWarnExpiryContext(ctx, missing, models.WarnExpiryMaxDays); err != nil {
			t.Fatalf("SetWarnExpiryContext on a chat with no row error = %v", err)
		}
		if row := storedWarnSettings(t, missing); row.ExpiryDays != models.WarnExpiryMaxDays || row.WarnLimit != 3 || row.WarnMode != "mute" {
			t.Fatalf("new row = limit %d, mode %q, expiry %d; want 3, mute, %d", row.WarnLimit, row.WarnMode, row.ExpiryDays, models.WarnExpiryMaxDays)
		}
	})

	t.Run("the read counts with a moved clock", func(t *testing.T) {
		useMiniredisCache(t)
		chatID, userID := member(t, 7)
		warnAt(t, userID, chatID, "a", daysAgo(5))
		warnAt(t, userID, chatID, "b", daysAgo(1))
		if n, reasons := GetWarnsAt(ctx, userID, chatID, now); n != 2 || len(reasons) != 2 {
			t.Fatalf("GetWarnsAt(now) = %d %v, want 2", n, reasons)
		}
		if n, reasons := GetWarnsAt(ctx, userID, chatID, now.Add(3*24*time.Hour)); n != 1 || !reflect.DeepEqual(reasons, []string{"b"}) {
			t.Fatalf("GetWarnsAt(now+3d) = %d %v, want 1 [b]", n, reasons)
		}
		if n, _ := GetWarnsAt(ctx, userID, chatID, now.Add(10*24*time.Hour)); n != 0 {
			t.Fatalf("GetWarnsAt(now+10d) = %d, want 0", n)
		}
	})
}
