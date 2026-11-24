//go:build testtools

package antiflood

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	gocache "github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/marshaler"
	gocache_store "github.com/eko/gocache/store/redis/v4"
	"github.com/redis/go-redis/v9"

	"github.com/uasneppy/Xenobot-72/xenobot/config"
	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	utilsCache "github.com/uasneppy/Xenobot-72/xenobot/utils/cache"
)

// floodTimedRow reads the chat's antiflood row straight from the table; ok is false with no row.
func floodTimedRow(t *testing.T, chatID int64) (models.AntifloodSettings, bool) {
	t.Helper()
	var rows []models.AntifloodSettings
	if err := db.DB.Where("chat_id = ?", chatID).Find(&rows).Error; err != nil {
		t.Fatalf("read the antiflood row of chat %d: %v", chatID, err)
	}
	if len(rows) == 0 {
		return models.AntifloodSettings{}, false
	}
	return rows[0], true
}

// floodTimedMiniredis puts a miniredis-backed cache in front of the repository until the test ends.
func floodTimedMiniredis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	manager := gocache.New[any](gocache_store.NewRedis(client))
	previousMarshal, previousManager, previousClient := utilsCache.GetCacheState()
	utilsCache.SetCacheState(marshaler.New(manager), manager, client)

	previousConfig := config.AppConfig
	cfg := config.Config{}
	if previousConfig != nil {
		cfg = *previousConfig
	}
	cfg.DisableCache = false
	config.AppConfig = &cfg
	cache.ResetLocalForTest()

	t.Cleanup(func() {
		config.AppConfig = previousConfig
		cache.ResetLocalForTest()
		_ = client.Close()
		utilsCache.SetCacheState(previousMarshal, previousManager, previousClient)
	})
	return mr
}

// floodTimedKeys lists the cache keys of the chat's antiflood settings.
func floodTimedKeys(mr *miniredis.Miniredis, chatID int64) []string {
	var keys []string
	for _, key := range mr.Keys() {
		if strings.Contains(key, "antiflood") && strings.Contains(key, strconv.FormatInt(chatID, 10)) {
			keys = append(keys, key)
		}
	}
	return keys
}

// TestSetFloodActionDuration proves the write of a timed flood action (Phase 26, D-05): SetFloodActionContext
// stores the action and its seconds in one write that both readers return, a first write on a chat with no row
// stores the model's limit 5 (D-27), the two-argument SetFloodMode stores seconds 0, an unchanged action and
// seconds write nothing, and every write drops antiflood:<chat>. It runs on SQLite in make test.
func TestSetFloodActionDuration(t *testing.T) {
	skipIfNoDb(t)
	ctx := context.Background()
	fresh := func(t *testing.T) int64 {
		t.Helper()
		chatID := -time.Now().UnixNano()
		t.Cleanup(func() {
			db.DB.Where("chat_id = ?", chatID).Delete(&models.AntifloodSettings{})
			cache.DeleteCache(cache.CacheKey("antiflood", chatID))
		})
		return chatID
	}

	t.Run("action and seconds stored together, the first write stores limit 5", func(t *testing.T) {
		chatID := fresh(t)
		if err := SetFloodActionContext(ctx, chatID, "tmute", 1800); err != nil {
			t.Fatalf("SetFloodActionContext() error = %v", err)
		}
		row, ok := floodTimedRow(t, chatID)
		if !ok || row.Action != "tmute" || row.ActionSecs != 1800 || row.Limit != 5 {
			t.Fatalf("row = %+v (present %v), want tmute, 1800 and limit 5", row, ok)
		}
		read, err := GetAntifloodSettingsContext(ctx, chatID)
		if err != nil || read.Action != "tmute" || read.ActionSecs != 1800 {
			t.Fatalf("uncached read = %+v, %v; want tmute and 1800", read, err)
		}
		cached, err := GetAntifloodSettingsCachedContext(ctx, chatID)
		if err != nil || cached.Action != "tmute" || cached.ActionSecs != 1800 {
			t.Fatalf("cached read = %+v, %v; want tmute and 1800", cached, err)
		}

		if err := SetFloodMode(chatID, "ban"); err != nil {
			t.Fatalf("SetFloodMode(ban) error = %v", err)
		}
		row, _ = floodTimedRow(t, chatID)
		if row.Action != "ban" || row.ActionSecs != 0 || row.Limit != 5 {
			t.Fatalf("row after SetFloodMode(ban) = %+v, want ban, 0 and limit 5", row)
		}
	})

	t.Run("an unchanged action and seconds write nothing", func(t *testing.T) {
		chatID := fresh(t)
		if err := SetFloodActionContext(ctx, chatID, "tban", 86400); err != nil {
			t.Fatalf("SetFloodActionContext() error = %v", err)
		}
		before, _ := floodTimedRow(t, chatID)
		time.Sleep(10 * time.Millisecond)
		if err := SetFloodActionContext(ctx, chatID, "tban", 86400); err != nil {
			t.Fatalf("repeated SetFloodActionContext() error = %v", err)
		}
		after, _ := floodTimedRow(t, chatID)
		if !after.UpdatedAt.Equal(before.UpdatedAt) {
			t.Fatalf("updated_at moved from %v to %v, want no write", before.UpdatedAt, after.UpdatedAt)
		}

		// The default mute with no row is a no-op too, as SetFloodMode has always been.
		muteChat := fresh(t)
		if err := SetFloodActionContext(ctx, muteChat, "mute", 0); err != nil {
			t.Fatalf("SetFloodActionContext(mute, 0) error = %v", err)
		}
		if row, ok := floodTimedRow(t, muteChat); ok {
			t.Fatalf("a default mute write created %+v, want no row", row)
		}
	})

	t.Run("every write drops the antiflood key", func(t *testing.T) {
		mr := floodTimedMiniredis(t)
		chatID := fresh(t)
		writes := []struct {
			name  string
			write func() error
		}{
			{"SetFloodActionContext", func() error { return SetFloodActionContext(ctx, chatID, "tmute", 600) }},
			{"SetFloodActionContext seconds only", func() error { return SetFloodActionContext(ctx, chatID, "tmute", 3600) }},
			{"SetFloodMode", func() error { return SetFloodMode(chatID, "kick") }},
		}
		for _, w := range writes {
			if _, err := GetAntifloodSettingsCachedContext(ctx, chatID); err != nil {
				t.Fatalf("%s: warm the cache: %v", w.name, err)
			}
			if keys := floodTimedKeys(mr, chatID); len(keys) == 0 {
				t.Fatalf("%s: the read cached no antiflood key", w.name)
			}
			if err := w.write(); err != nil {
				t.Fatalf("%s() error = %v", w.name, err)
			}
			if keys := floodTimedKeys(mr, chatID); len(keys) != 0 {
				t.Fatalf("%s: keys after the write = %v, want none", w.name, keys)
			}
		}
		got, err := GetAntifloodSettingsCachedContext(ctx, chatID)
		if err != nil || got.Action != "kick" || got.ActionSecs != 0 {
			t.Fatalf("read after the writes = %+v, %v; want kick and 0", got, err)
		}
	})
}
