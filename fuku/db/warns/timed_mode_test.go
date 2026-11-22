package warns

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/chats"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
)

// storedWarnSettings reads the chat's warns_settings row straight from the table.
func storedWarnSettings(t *testing.T, chatID int64) models.WarnSettings {
	t.Helper()
	var row models.WarnSettings
	if err := db.DB.Where("chat_id = ?", chatID).First(&row).Error; err != nil {
		t.Fatalf("read the warns_settings row of chat %d: %v", chatID, err)
	}
	return row
}

// TestSetWarnModeDuration proves the write of a timed warn mode: SetWarnModeContext stores the mode
// and its seconds in one upsert and keeps the limit, the two-argument SetWarnMode stores seconds 0,
// a limit write and a mode write of one chat never clobber each other, and every write drops the
// chat's warn_settings key. It runs on SQLite in make test and on PostgreSQL in the integrity run.
func TestSetWarnModeDuration(t *testing.T) {
	skipIfNoDb(t)
	ctx := context.Background()

	known := func(t *testing.T) int64 {
		t.Helper()
		chatID := -time.Now().UnixNano()
		if err := chats.EnsureChatInDb(chatID, "timed-warn-mode"); err != nil {
			t.Fatalf("EnsureChatInDb() error = %v", err)
		}
		t.Cleanup(func() {
			db.DB.Where("chat_id = ?", chatID).Delete(&models.WarnSettings{})
			db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{})
			cache.DeleteCache(cache.CacheKey("warn_settings", chatID))
		})
		return chatID
	}

	t.Run("timed mode and seconds stored together, limit kept", func(t *testing.T) {
		chatID := known(t)
		if err := SetWarnLimit(chatID, 7); err != nil {
			t.Fatalf("SetWarnLimit() error = %v", err)
		}
		if err := SetWarnModeContext(ctx, chatID, "tmute", 7200); err != nil {
			t.Fatalf("SetWarnModeContext() error = %v", err)
		}
		row := storedWarnSettings(t, chatID)
		if row.WarnMode != "tmute" || row.WarnModeSecs != 7200 || row.WarnLimit != 7 {
			t.Fatalf("row = limit %d, mode %q, secs %d; want 7, tmute, 7200", row.WarnLimit, row.WarnMode, row.WarnModeSecs)
		}

		if err := SetWarnMode(chatID, "ban"); err != nil {
			t.Fatalf("SetWarnMode() error = %v", err)
		}
		row = storedWarnSettings(t, chatID)
		if row.WarnMode != "ban" || row.WarnModeSecs != 0 || row.WarnLimit != 7 {
			t.Fatalf("row after SetWarnMode(ban) = limit %d, mode %q, secs %d; want 7, ban, 0", row.WarnLimit, row.WarnMode, row.WarnModeSecs)
		}
	})

	t.Run("a missing row is created with limit 3", func(t *testing.T) {
		chatID := known(t)
		if err := SetWarnModeContext(ctx, chatID, "tban", 86400); err != nil {
			t.Fatalf("SetWarnModeContext() error = %v", err)
		}
		row := storedWarnSettings(t, chatID)
		if row.WarnMode != "tban" || row.WarnModeSecs != 86400 || row.WarnLimit != 3 {
			t.Fatalf("row = limit %d, mode %q, secs %d; want 3, tban, 86400", row.WarnLimit, row.WarnMode, row.WarnModeSecs)
		}
	})

	t.Run("limit and mode writes never clobber each other", func(t *testing.T) {
		chatID := known(t)
		if err := SetWarnModeContext(ctx, chatID, "mute", 0); err != nil {
			t.Fatalf("seed the row: %v", err)
		}
		var wg sync.WaitGroup
		errs := make(chan error, 2)
		wg.Add(2)
		go func() {
			defer wg.Done()
			errs <- SetWarnLimit(chatID, 9)
		}()
		go func() {
			defer wg.Done()
			errs <- SetWarnModeContext(ctx, chatID, "tmute", 3600)
		}()
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("concurrent write error = %v", err)
			}
		}
		row := storedWarnSettings(t, chatID)
		if row.WarnLimit != 9 || row.WarnMode != "tmute" || row.WarnModeSecs != 3600 {
			t.Fatalf("row = limit %d, mode %q, secs %d; want 9, tmute, 3600", row.WarnLimit, row.WarnMode, row.WarnModeSecs)
		}
	})

	t.Run("every write drops the warn_settings key", func(t *testing.T) {
		mr := useMiniredisCache(t)
		chatID := known(t)
		writes := []struct {
			name  string
			write func() error
		}{
			{"SetWarnModeContext", func() error { return SetWarnModeContext(ctx, chatID, "tmute", 600) }},
			{"SetWarnMode", func() error { return SetWarnMode(chatID, "kick") }},
		}
		for _, w := range writes {
			if _, err := GetWarnSettingErrContext(ctx, chatID); err != nil {
				t.Fatalf("%s: warm the cache: %v", w.name, err)
			}
			if keys := warnSettingsKeys(mr, chatID); len(keys) == 0 {
				t.Fatalf("%s: the read cached no warn_settings key", w.name)
			}
			if err := w.write(); err != nil {
				t.Fatalf("%s() error = %v", w.name, err)
			}
			if keys := warnSettingsKeys(mr, chatID); len(keys) != 0 {
				t.Fatalf("%s: keys after the write = %v, want none", w.name, keys)
			}
		}
		got, err := GetWarnSettingErrContext(ctx, chatID)
		if err != nil {
			t.Fatalf("GetWarnSettingErrContext() error = %v", err)
		}
		if got.WarnMode != "kick" || got.WarnModeSecs != 0 {
			t.Fatalf("read after the writes = mode %q, secs %d; want kick, 0", got.WarnMode, got.WarnModeSecs)
		}
	})
}
