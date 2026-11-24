//go:build testtools

package blacklists

import (
	"context"
	"testing"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	utilsCache "github.com/uasneppy/Xenobot-72/xenobot/utils/cache"
)

// timedActionChats returns two chat IDs no other test of the package uses, with their rows and
// cache entries deleted before the test and when it ends.
func timedActionChats(t *testing.T) (chatA, chatB int64) {
	t.Helper()
	utilsCache.SetupTestMemoryMarshaler(t)
	chatA, chatB = -1009926005001, -1009926005002
	clean := func() {
		db.DB.Where("chat_id IN ?", []int64{chatA, chatB}).Delete(&models.BlacklistSettings{})
		cache.DeleteCache(cache.CacheKey("blacklist", chatA))
		cache.DeleteCache(cache.CacheKey("blacklist", chatB))
		cache.ResetLocalForTest()
	}
	clean()
	t.Cleanup(clean)
	return chatA, chatB
}

// timedActionRows reads the chat's words straight from the table as word=action/secs, by ID.
func timedActionRows(t *testing.T, chatID int64) map[string]models.BlacklistSettings {
	t.Helper()
	var rows []models.BlacklistSettings
	if err := db.DB.Where("chat_id = ?", chatID).Order("id").Find(&rows).Error; err != nil {
		t.Fatalf("read the words of chat %d: %v", chatID, err)
	}
	out := make(map[string]models.BlacklistSettings, len(rows))
	for _, row := range rows {
		out[row.Word] = row
	}
	return out
}

// TestSetBlacklistActionDuration proves the writes of a timed blacklist action (Phase 26, D-06, D-09):
// SetBlacklistActionTimedContext stores the action and its seconds on every word of the chat in one update,
// returns the rows matched and drops blacklist:<chat>; the two-argument SetBlacklistAction stores seconds 0;
// AddBlacklistTimedContext stores a word with both; another chat's words are untouched. It runs on SQLite in
// make test.
func TestSetBlacklistActionDuration(t *testing.T) {
	skipIfNoDb(t)
	ctx := context.Background()
	chatA, chatB := timedActionChats(t)
	for _, word := range []string{"spam", "eggs"} {
		if err := AddBlacklist(chatA, word); err != nil {
			t.Fatalf("AddBlacklist(%q) error = %v", word, err)
		}
	}
	if err := AddBlacklistContext(ctx, chatB, "other", "kick"); err != nil {
		t.Fatalf("AddBlacklistContext(other) error = %v", err)
	}

	// Fill the cache, so the read after the write proves the key was dropped.
	if got := GetBlacklistSettingsContext(ctx, chatA); len(got) != 2 || got[0].ActionSecs != 0 {
		t.Fatalf("cached read before the write = %+v, want two words with seconds 0", got)
	}

	rows, err := SetBlacklistActionTimedContext(ctx, chatA, "TMUTE", 7200)
	if err != nil || rows != 2 {
		t.Fatalf("SetBlacklistActionTimedContext() = %d, %v, want 2 and nil", rows, err)
	}
	for word, row := range timedActionRows(t, chatA) {
		if row.Action != "tmute" || row.ActionSecs != 7200 {
			t.Errorf("word %q = %s/%d, want tmute/7200", word, row.Action, row.ActionSecs)
		}
	}
	for _, row := range GetBlacklistSettingsContext(ctx, chatA) {
		if row.Action != "tmute" || row.ActionSecs != 7200 {
			t.Errorf("cached word %q = %s/%d after the write, want tmute/7200 (the key was not dropped)", row.Word, row.Action, row.ActionSecs)
		}
	}
	if other := timedActionRows(t, chatB)["other"]; other.Action != "kick" || other.ActionSecs != 0 {
		t.Errorf("another chat's word = %s/%d, want kick/0", other.Action, other.ActionSecs)
	}

	if err := SetBlacklistAction(chatA, "ban"); err != nil {
		t.Fatalf("SetBlacklistAction(ban) error = %v", err)
	}
	for word, row := range timedActionRows(t, chatA) {
		if row.Action != "ban" || row.ActionSecs != 0 {
			t.Errorf("word %q = %s/%d after SetBlacklistAction(ban), want ban/0", word, row.Action, row.ActionSecs)
		}
	}

	if err := AddBlacklistTimedContext(ctx, chatA, "Ham", "tban", 604800); err != nil {
		t.Fatalf("AddBlacklistTimedContext() error = %v", err)
	}
	if ham := timedActionRows(t, chatA)["ham"]; ham.Action != "tban" || ham.ActionSecs != 604800 {
		t.Errorf("added word = %s/%d, want tban/604800", ham.Action, ham.ActionSecs)
	}
	listed, err := ListBlacklistEntriesFresh(ctx, chatA)
	if err != nil {
		t.Fatalf("ListBlacklistEntriesFresh() error = %v", err)
	}
	for _, row := range listed {
		if row.Word == "ham" && row.ActionSecs != 604800 {
			t.Errorf("the fresh list read seconds %d for ham, want 604800", row.ActionSecs)
		}
	}

	none, err := SetBlacklistActionTimedContext(ctx, -1009926005003, "tmute", 7200)
	if err != nil || none != 0 {
		t.Errorf("a chat with no words: %d, %v, want 0 and nil", none, err)
	}
}
