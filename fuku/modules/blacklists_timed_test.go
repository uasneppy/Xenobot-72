//go:build testtools

package modules

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/uasneppy/fuku_robot/fuku/db"
	dbcache "github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/deletion"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
)

// blacklistTimedStored reads the chat's words straight from the table as word=action/secs, sorted.
func blacklistTimedStored(t *testing.T, chatID int64) []string {
	t.Helper()
	var rows []models.BlacklistSettings
	if err := db.DB.Where("chat_id = ?", chatID).Find(&rows).Error; err != nil {
		t.Fatalf("read blacklists: %v", err)
	}
	words := make([]string, 0, len(rows))
	for _, row := range rows {
		words = append(words, fmt.Sprintf("%s=%s/%d", row.Word, row.Action, row.ActionSecs))
	}
	slices.Sort(words)
	return words
}

// TestApplyBlacklistActionTimed holds the one write path of the blacklist action with a duration (D-09): a valid
// timed action stores both columns on every word, a plain action stores seconds 0, a timed action with no
// duration is the pinned unknown outcome, a duration outside 60 s to 366 days is the bad-duration outcome,
// neither refusal writes, and a chat with no words is the no-words outcome.
func TestApplyBlacklistActionTimed(t *testing.T) {
	ctx := context.Background()
	chatID := blacklistApplyChat(t)
	blacklistApplySeed(t, chatID, "warn", "spam", "eggs")

	if got := applyBlacklistAction(ctx, chatID, "tmute", 7200); got.Outcome != blacklistActionApplied || got.Rows != 2 {
		t.Fatalf("applyBlacklistAction(tmute, 7200) = %+v, want applied over 2 rows", got)
	}
	if got, want := blacklistTimedStored(t, chatID), []string{"eggs=tmute/7200", "spam=tmute/7200"}; !slices.Equal(got, want) {
		t.Fatalf("stored = %q, want %q", got, want)
	}

	// A plain action stores seconds 0, also when it is handed seconds.
	if got := applyBlacklistAction(ctx, chatID, "ban", 3600); got.Outcome != blacklistActionApplied {
		t.Fatalf("applyBlacklistAction(ban, 3600) = %+v, want applied", got)
	}
	if got, want := blacklistTimedStored(t, chatID), []string{"eggs=ban/0", "spam=ban/0"}; !slices.Equal(got, want) {
		t.Fatalf("stored after ban = %q, want %q", got, want)
	}

	for _, c := range []struct {
		action string
		secs   int
		want   blacklistActionOutcome
	}{
		{"tban", 0, blacklistActionUnknown},
		{"tmute", 0, blacklistActionUnknown},
		{"tmute", 59, blacklistActionBadDuration},
		{"tban", 31622401, blacklistActionBadDuration},
		{"tban", -1, blacklistActionBadDuration},
		{"delete", 3600, blacklistActionUnknown},
	} {
		if got := applyBlacklistAction(ctx, chatID, c.action, c.secs); got.Outcome != c.want || got.Rows != 0 || got.Err != nil {
			t.Errorf("applyBlacklistAction(%s, %d) = %+v, want outcome %v and nothing written", c.action, c.secs, got, c.want)
		}
	}
	if got, want := blacklistTimedStored(t, chatID), []string{"eggs=ban/0", "spam=ban/0"}; !slices.Equal(got, want) {
		t.Fatalf("a refusal changed the rows: %q, want %q", got, want)
	}

	empty := blacklistApplyChat(t)
	if got := applyBlacklistAction(ctx, empty, "tban", 604800); got.Outcome != blacklistActionNoWords || got.Rows != 0 {
		t.Errorf("a chat with no words = %+v, want no words over 0 rows", got)
	}
}

// TestBlacklistActionTimedCommand drives /blacklistaction's timed form and /addblacklist through the handlers in
// Ukrainian with the real locales: the reply names the short duration, every word stores the seconds, a bad
// duration writes nothing, the current-mode reply shows the short form, and a new word takes the shared action
// and seconds, or warn and 0 once the words are mixed (A19-3 extended, D-21).
func TestBlacklistActionTimedCommand(t *testing.T) {
	withModerationGoldenLocales(t)
	tr := i18n.MustNewTranslator("uk")
	s := newNoticeGoldenSite(t, deletion.KindBotManual)
	timedGoldenCleanup(t, s.chat.Id)
	blacklistApplySeed(t, s.chat.Id, "warn", "spam", "eggs")
	changed := func(shown string) string {
		temp, _ := tr.GetString("blacklists_set_bl_action_changed_mode")
		return fmt.Sprintf(temp, shown)
	}

	timedGoldenCommand(t, s, blacklistsModule.setBlacklistAction, timedGoldenAdmin, "/blacklistaction tmute 2h", false)
	if got, want := warnTimedLastText(t, s), changed("tmute 2h"); got != want {
		t.Errorf("tmute 2h: reply = %q, want %q", got, want)
	}
	if got, want := blacklistTimedStored(t, s.chat.Id), []string{"eggs=tmute/7200", "spam=tmute/7200"}; !slices.Equal(got, want) {
		t.Fatalf("stored after tmute 2h = %q, want %q", got, want)
	}

	timedGoldenCommand(t, s, blacklistsModule.setBlacklistAction, timedGoldenAdmin, "/blacklistaction TBAN 1w", false)
	if got, want := warnTimedLastText(t, s), changed("tban 1w"); got != want {
		t.Errorf("TBAN 1w: reply = %q, want %q", got, want)
	}
	if got, want := blacklistTimedStored(t, s.chat.Id), []string{"eggs=tban/604800", "spam=tban/604800"}; !slices.Equal(got, want) {
		t.Fatalf("stored after TBAN 1w = %q, want %q", got, want)
	}

	bad, _ := tr.GetString("blacklists_set_bl_action_bad_duration")
	for _, text := range []string{"/blacklistaction tmute 0m", "/blacklistaction tban xyz", "/blacklistaction tmute 400d"} {
		timedGoldenCommand(t, s, blacklistsModule.setBlacklistAction, timedGoldenAdmin, text, false)
		if got := warnTimedLastText(t, s); got != bad {
			t.Errorf("%s: reply = %q, want %q", text, got, bad)
		}
	}
	if got, want := blacklistTimedStored(t, s.chat.Id), []string{"eggs=tban/604800", "spam=tban/604800"}; !slices.Equal(got, want) {
		t.Fatalf("a bad duration changed the rows: %q, want %q", got, want)
	}

	timedGoldenCommand(t, s, blacklistsModule.setBlacklistAction, timedGoldenAdmin, "/blacklistaction", false)
	current, _ := tr.GetString("blacklists_set_bl_action_current_mode")
	if got, want := warnTimedLastText(t, s), fmt.Sprintf(current, "tban 1w"); got != want {
		t.Errorf("current mode = %q, want %q", got, want)
	}

	timedGoldenCommand(t, s, blacklistsModule.addBlacklist, timedGoldenAdmin, "/addblacklist egg", false)
	if got := blacklistTimedStored(t, s.chat.Id); !slices.Contains(got, "egg=tban/604800") {
		t.Fatalf("stored after /addblacklist egg = %q, want egg with tban and 604800", got)
	}

	// Mixed seconds give a new word warn and 0.
	if err := db.DB.Model(&models.BlacklistSettings{}).Where("chat_id = ? AND word = ?", s.chat.Id, "egg").
		Update("action_secs", 3600).Error; err != nil {
		t.Fatalf("mix the seconds: %v", err)
	}
	blacklistTimedDropCache(s.chat.Id)
	timedGoldenCommand(t, s, blacklistsModule.addBlacklist, timedGoldenAdmin, "/addblacklist ham", false)
	if got := blacklistTimedStored(t, s.chat.Id); !slices.Contains(got, "ham=warn/0") {
		t.Fatalf("stored after /addblacklist ham on mixed words = %q, want ham with warn and 0", got)
	}
}

// blacklistTimedDropCache drops the chat's cached blacklist after a row was written straight to the table.
func blacklistTimedDropCache(chatID int64) {
	dbcache.DeleteCache(dbcache.CacheKey("blacklist", chatID))
	dbcache.ResetLocalForTest()
}
