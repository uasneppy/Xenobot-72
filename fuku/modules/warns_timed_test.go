//go:build testtools

package modules

import (
	"context"
	"fmt"
	"testing"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/deletion"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
)

// warnTimedStored reads the chat's warns_settings row straight from the table; ok is false with no row.
func warnTimedStored(t *testing.T, chatID int64) (models.WarnSettings, bool) {
	t.Helper()
	var rows []models.WarnSettings
	if err := db.DB.Where("chat_id = ?", chatID).Find(&rows).Error; err != nil {
		t.Fatalf("read the warns_settings row of chat %d: %v", chatID, err)
	}
	if len(rows) == 0 {
		return models.WarnSettings{}, false
	}
	return rows[0], true
}

// warnTimedLastText is the text of the site's last sendMessage request.
func warnTimedLastText(t *testing.T, s *botSite) string {
	t.Helper()
	calls := s.client.callsFor("sendMessage")
	if len(calls) == 0 {
		t.Fatal("no sendMessage request")
	}
	return fmt.Sprint(calls[len(calls)-1].Params["text"])
}

// TestApplyWarnModeTimed holds the one write path of the warn mode with a duration: a valid timed mode
// stores both columns, a plain mode stores seconds 0, a timed mode with no duration is the pinned unknown
// outcome, a duration outside 60 s to 366 days is the bad-duration outcome, and neither refusal writes.
func TestApplyWarnModeTimed(t *testing.T) {
	ctx := context.Background()

	t.Run("a valid timed mode stores the mode and the seconds", func(t *testing.T) {
		chatID := warnApplyFreshChat(t)
		got := applyWarnMode(ctx, chatID, "tmute", 7200)
		if got.Outcome != warnSettingsApplied || got.Key != "warns_mode_updated_tmute" || got.Err != nil {
			t.Fatalf("applyWarnMode(tmute, 7200) = %+v, want warnSettingsApplied", got)
		}
		row, ok := warnTimedStored(t, chatID)
		if !ok || row.WarnMode != "tmute" || row.WarnModeSecs != 7200 {
			t.Fatalf("stored row = %+v (present %v), want tmute and 7200", row, ok)
		}
		if got.Stored == nil || got.Stored.WarnModeSecs != 7200 {
			t.Fatalf("read back = %+v, want seconds 7200", got.Stored)
		}
	})

	t.Run("a plain mode stores seconds 0", func(t *testing.T) {
		chatID := warnApplyFreshChat(t)
		if got := applyWarnMode(ctx, chatID, "tban", 86400); got.Outcome != warnSettingsApplied {
			t.Fatalf("seed applyWarnMode(tban, 86400) = %+v", got)
		}
		got := applyWarnMode(ctx, chatID, "ban", 0)
		if got.Outcome != warnSettingsApplied || got.Key != "warns_mode_updated_ban" {
			t.Fatalf("applyWarnMode(ban, 0) = %+v, want warnSettingsApplied", got)
		}
		if row, ok := warnTimedStored(t, chatID); !ok || row.WarnMode != "ban" || row.WarnModeSecs != 0 {
			t.Fatalf("stored row = %+v (present %v), want ban and 0", row, ok)
		}
	})

	t.Run("a timed mode with no duration is unknown and writes nothing", func(t *testing.T) {
		chatID := warnApplyFreshChat(t)
		got := applyWarnMode(ctx, chatID, "tban", 0)
		if got.Outcome != warnSettingsUnknownMode || got.Key != "warns_mode_unknown" || got.Stored != nil {
			t.Fatalf("applyWarnMode(tban, 0) = %+v, want warnSettingsUnknownMode", got)
		}
		if n := warnApplyRows(t, chatID); n != 0 {
			t.Fatalf("rows = %d, want none", n)
		}
	})

	t.Run("a duration out of bounds is a bad duration and writes nothing", func(t *testing.T) {
		for _, secs := range []int{59, punishMaxSecs + 1, -1} {
			chatID := warnApplyFreshChat(t)
			got := applyWarnMode(ctx, chatID, "tmute", secs)
			if got.Outcome != warnSettingsBadDuration || got.Key != "warns_mode_bad_duration" || got.Stored != nil {
				t.Fatalf("applyWarnMode(tmute, %d) = %+v, want warnSettingsBadDuration", secs, got)
			}
			if n := warnApplyRows(t, chatID); n != 0 {
				t.Fatalf("applyWarnMode(tmute, %d) left %d rows, want none", secs, n)
			}
		}
	})

	t.Run("a failed write is a save failure", func(t *testing.T) {
		chatID := warnApplyFreshChat(t)
		warnApplyFailingWrites(t)
		got := applyWarnMode(ctx, chatID, "tmute", 600)
		if got.Outcome != warnSettingsSaveFailed || got.Key != "common_settings_save_failed" || got.Err == nil {
			t.Fatalf("applyWarnMode(tmute, 600) = %+v, want warnSettingsSaveFailed", got)
		}
	})
}

// TestSetWarnModeTimedCommand drives /setwarnmode's timed forms and /warnings through the handlers in
// Ukrainian with the real locales: the reply names the duration, a bad duration writes nothing, a plain
// mode with an extra word stores the plain mode, and /warnings shows the short form.
func TestSetWarnModeTimedCommand(t *testing.T) {
	withModerationGoldenLocales(t)
	tr := i18n.MustNewTranslator("uk")
	site := func(t *testing.T) *botSite {
		t.Helper()
		s := newNoticeGoldenSite(t, deletion.KindBotManual)
		timedGoldenCleanup(t, s.chat.Id)
		return s
	}

	t.Run("tmute 2h", func(t *testing.T) {
		s := site(t)
		timedGoldenCommand(t, s, warnsModule.setWarnMode, timedGoldenAdmin, "/setwarnmode tmute 2h", false)
		want, _ := tr.GetString("warns_mode_updated_tmute", i18n.TranslationParams{"duration": punishDurationLabel(tr, 7200)})
		if got := warnTimedLastText(t, s); got != want {
			t.Errorf("reply = %q, want %q", got, want)
		}
		if row, ok := warnTimedStored(t, s.chat.Id); !ok || row.WarnMode != "tmute" || row.WarnModeSecs != 7200 {
			t.Fatalf("stored row = %+v (present %v), want tmute and 7200", row, ok)
		}

		timedGoldenCommand(t, s, warnsModule.warnings, timedGoldenAdmin, "/warnings", false)
		display, _ := tr.GetString("warns_settings_display")
		if got, want := warnTimedLastText(t, s), fmt.Sprintf(display, 3, "tmute 2h"); got != want {
			t.Errorf("/warnings = %q, want %q", got, want)
		}
	})

	t.Run("TBAN 1d", func(t *testing.T) {
		s := site(t)
		timedGoldenCommand(t, s, warnsModule.setWarnMode, timedGoldenAdmin, "/setwarnmode TBAN 1d", false)
		want, _ := tr.GetString("warns_mode_updated_tban", i18n.TranslationParams{"duration": punishDurationLabel(tr, 86400)})
		if got := warnTimedLastText(t, s); got != want {
			t.Errorf("reply = %q, want %q", got, want)
		}
		if row, ok := warnTimedStored(t, s.chat.Id); !ok || row.WarnMode != "tban" || row.WarnModeSecs != 86400 {
			t.Fatalf("stored row = %+v (present %v), want tban and 86400", row, ok)
		}
	})

	t.Run("bad durations write nothing", func(t *testing.T) {
		for _, text := range []string{"/setwarnmode tmute xyz", "/setwarnmode tmute 0m", "/setwarnmode tmute 400d"} {
			s := site(t)
			timedGoldenCommand(t, s, warnsModule.setWarnMode, timedGoldenAdmin, text, false)
			want, _ := tr.GetString("warns_mode_bad_duration")
			if got := warnTimedLastText(t, s); got != want {
				t.Errorf("%s: reply = %q, want %q", text, got, want)
			}
			if row, ok := warnTimedStored(t, s.chat.Id); ok {
				t.Errorf("%s: a row was written: %+v", text, row)
			}
		}
	})

	t.Run("ban 1d stores plain ban", func(t *testing.T) {
		s := site(t)
		timedGoldenCommand(t, s, warnsModule.setWarnMode, timedGoldenAdmin, "/setwarnmode ban 1d", false)
		want, _ := tr.GetString("warns_mode_updated_ban")
		if got := warnTimedLastText(t, s); got != want {
			t.Errorf("reply = %q, want %q", got, want)
		}
		if row, ok := warnTimedStored(t, s.chat.Id); !ok || row.WarnMode != "ban" || row.WarnModeSecs != 0 {
			t.Fatalf("stored row = %+v (present %v), want ban and 0", row, ok)
		}
	})
}
