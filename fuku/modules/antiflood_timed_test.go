//go:build testtools

package modules

import (
	"context"
	"fmt"
	"testing"

	"github.com/uasneppy/fuku_robot/fuku/db/deletion"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
)

// TestApplyFloodModeTimed holds the one write path of the flood action with a duration: a valid timed action
// stores both columns, a plain action stores seconds 0, a timed action with no duration is the pinned unknown
// outcome, a duration outside 60 s to 366 days is the bad-duration outcome, neither refusal writes, and a failed
// write is a save failure.
func TestApplyFloodModeTimed(t *testing.T) {
	ctx := context.Background()

	t.Run("a valid timed action stores the action and the seconds", func(t *testing.T) {
		chatID := floodApplyFreshChat(t)
		got := applyFloodMode(ctx, chatID, "tmute", 1800)
		if got.Outcome != floodApplied || got.Err != nil {
			t.Fatalf("applyFloodMode(tmute, 1800) = %+v, want floodApplied", got)
		}
		if got.Stored == nil || got.Stored.Action != "tmute" || got.Stored.ActionSecs != 1800 {
			t.Fatalf("read back = %+v, want tmute and 1800", got.Stored)
		}
		if row := floodApplyStored(t, chatID); row.Action != "tmute" || row.ActionSecs != 1800 || row.Limit != 5 {
			t.Fatalf("stored row = %+v, want tmute, 1800 and the first-write limit 5", row)
		}
	})

	t.Run("a plain action stores seconds 0", func(t *testing.T) {
		chatID := floodApplyFreshChat(t)
		if got := applyFloodMode(ctx, chatID, "tban", 86400); got.Outcome != floodApplied {
			t.Fatalf("seed applyFloodMode(tban, 86400) = %+v", got)
		}
		got := applyFloodMode(ctx, chatID, "ban", 0)
		if got.Outcome != floodApplied || got.Stored == nil || got.Stored.Action != "ban" || got.Stored.ActionSecs != 0 {
			t.Fatalf("applyFloodMode(ban, 0) = %+v, want ban and 0", got)
		}
		// A plain action ignores the seconds it is handed.
		got = applyFloodMode(ctx, chatID, "kick", 3600)
		if got.Outcome != floodApplied || got.Stored == nil || got.Stored.Action != "kick" || got.Stored.ActionSecs != 0 {
			t.Fatalf("applyFloodMode(kick, 3600) = %+v, want kick and 0", got)
		}
	})

	t.Run("a timed action with no duration is unknown and writes nothing", func(t *testing.T) {
		for _, mode := range []string{"tban", "tmute"} {
			chatID := floodApplyFreshChat(t)
			got := applyFloodMode(ctx, chatID, mode, 0)
			if got.Outcome != floodUnknownMode || got.Stored != nil || got.Err != nil {
				t.Fatalf("applyFloodMode(%s, 0) = %+v, want floodUnknownMode", mode, got)
			}
			if n := floodApplyRowCount(t, chatID); n != 0 {
				t.Fatalf("applyFloodMode(%s, 0) left %d rows, want none", mode, n)
			}
		}
	})

	t.Run("a duration out of bounds is a bad duration and writes nothing", func(t *testing.T) {
		for _, c := range []struct {
			mode string
			secs int
		}{{"tmute", 59}, {"tban", 31622401}, {"tmute", -1}} {
			chatID := floodApplyFreshChat(t)
			got := applyFloodMode(ctx, chatID, c.mode, c.secs)
			if got.Outcome != floodBadDuration || got.Stored != nil || got.Err != nil {
				t.Fatalf("applyFloodMode(%s, %d) = %+v, want floodBadDuration", c.mode, c.secs, got)
			}
			if n := floodApplyRowCount(t, chatID); n != 0 {
				t.Fatalf("applyFloodMode(%s, %d) left %d rows, want none", c.mode, c.secs, n)
			}
		}
	})

	t.Run("a failed write is a save failure", func(t *testing.T) {
		floodApplyBrokenDB(t)
		got := applyFloodMode(ctx, uniqueModuleChatID(), "tmute", 600)
		if got.Outcome != floodSaveFailed || got.Err == nil || got.Stored != nil {
			t.Fatalf("applyFloodMode(tmute, 600) on a broken database = %+v, want floodSaveFailed", got)
		}
	})
}

// TestSetFloodModeTimedCommand drives /setfloodmode's timed forms and /flood through the handlers in Ukrainian
// with the real locales: the reply names the short duration, a bad duration writes nothing, a plain action with
// an extra word stores the plain action, and /flood shows the timed word.
func TestSetFloodModeTimedCommand(t *testing.T) {
	withModerationGoldenLocales(t)
	tr := i18n.MustNewTranslator("uk")
	site := func(t *testing.T) *botSite {
		t.Helper()
		s := newNoticeGoldenSite(t, deletion.KindBotManual)
		timedGoldenCleanup(t, s.chat.Id)
		return s
	}
	success := func(shown string) string {
		temp, _ := tr.GetString("antiflood_setfloodmode_success")
		return fmt.Sprintf(temp, shown)
	}
	shows := func(limit int, word string) string {
		temp, _ := tr.GetString("antiflood_flood_show_settings")
		return fmt.Sprintf(temp, limit, word)
	}

	t.Run("tmute 30m", func(t *testing.T) {
		s := site(t)
		timedGoldenCommand(t, s, antifloodModule.setFloodMode, timedGoldenAdmin, "/setfloodmode tmute 30m", false)
		if got, want := warnTimedLastText(t, s), success("tmute 30m"); got != want {
			t.Errorf("reply = %q, want %q", got, want)
		}
		if row := floodApplyStored(t, s.chat.Id); row.Action != "tmute" || row.ActionSecs != 1800 {
			t.Fatalf("stored row = %+v, want tmute and 1800", row)
		}
		timedGoldenCommand(t, s, antifloodModule.flood, timedGoldenAdmin, "/flood", false)
		if got, want := warnTimedLastText(t, s), shows(5, "muted for 30m"); got != want {
			t.Errorf("/flood = %q, want %q", got, want)
		}
	})

	t.Run("TBAN 1d", func(t *testing.T) {
		s := site(t)
		timedGoldenCommand(t, s, antifloodModule.setFloodMode, timedGoldenAdmin, "/setfloodmode TBAN 1d", false)
		if got, want := warnTimedLastText(t, s), success("tban 1d"); got != want {
			t.Errorf("reply = %q, want %q", got, want)
		}
		if row := floodApplyStored(t, s.chat.Id); row.Action != "tban" || row.ActionSecs != 86400 {
			t.Fatalf("stored row = %+v, want tban and 86400", row)
		}
		timedGoldenCommand(t, s, antifloodModule.flood, timedGoldenAdmin, "/flood", false)
		if got, want := warnTimedLastText(t, s), shows(5, "banned for 1d"); got != want {
			t.Errorf("/flood = %q, want %q", got, want)
		}
	})

	t.Run("bad durations write nothing", func(t *testing.T) {
		want, _ := tr.GetString("antiflood_setfloodmode_bad_duration")
		for _, text := range []string{"/setfloodmode tmute 0m", "/setfloodmode tban xyz", "/setfloodmode tmute 400d"} {
			s := site(t)
			timedGoldenCommand(t, s, antifloodModule.setFloodMode, timedGoldenAdmin, text, false)
			if got := warnTimedLastText(t, s); got != want {
				t.Errorf("%s: reply = %q, want %q", text, got, want)
			}
			if n := floodApplyRowCount(t, s.chat.Id); n != 0 {
				t.Errorf("%s: %d rows were written, want none", text, n)
			}
		}
	})

	t.Run("ban x stores plain ban", func(t *testing.T) {
		s := site(t)
		if got := applyFloodMode(context.Background(), s.chat.Id, "tmute", 600); got.Outcome != floodApplied {
			t.Fatalf("seed applyFloodMode(tmute, 600) = %+v", got)
		}
		timedGoldenCommand(t, s, antifloodModule.setFloodMode, timedGoldenAdmin, "/setfloodmode ban x", false)
		if got, want := warnTimedLastText(t, s), success("ban"); got != want {
			t.Errorf("reply = %q, want %q", got, want)
		}
		if row := floodApplyStored(t, s.chat.Id); row.Action != "ban" || row.ActionSecs != 0 {
			t.Fatalf("stored row = %+v, want ban and 0", row)
		}
	})
}
