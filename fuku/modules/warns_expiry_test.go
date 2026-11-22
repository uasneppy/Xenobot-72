//go:build testtools

package modules

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/db/warns"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
	"github.com/uasneppy/fuku_robot/fuku/utils/formatting"
)

// warnExpirySite is warnTimedSite (real locales, member 42, a clean chat) with the group's warn expiry
// stored and a moveable punishment clock; at sets the clock and every /warn after it is dated there.
func warnExpirySite(t *testing.T, days int) (s *botSite, at func(time.Time)) {
	t.Helper()
	s = warnTimedSite(t)
	timedGoldenChatRow(t, s)
	clock := s.now
	mreqSwap(t, &punishNow, func() time.Time { return clock })
	if err := warns.SetWarnExpiryContext(context.Background(), s.chat.Id, days); err != nil {
		t.Fatalf("SetWarnExpiryContext(%d) error = %v", days, err)
	}
	return s, func(when time.Time) { clock = when }
}

// warnExpiryWarn sends /warn reason as the administrator, replying to member 42, once per reason.
func warnExpiryWarn(t *testing.T, s *botSite, reasons ...string) {
	t.Helper()
	for _, reason := range reasons {
		noticeGoldenWarn(t, s, warnsModule.warnUser, "/warn "+reason)
	}
}

// warnExpiryRow reads member 42's warns_users row straight from the table; NumWarns -1 means no row.
func warnExpiryRow(t *testing.T, s *botSite) models.Warns {
	t.Helper()
	var rows []models.Warns
	if err := db.DB.Where("user_id = ? AND chat_id = ?", botSiteTarget.Id, s.chat.Id).Find(&rows).Error; err != nil {
		t.Fatalf("read warns_users: %v", err)
	}
	if len(rows) == 0 {
		return models.Warns{NumWarns: -1}
	}
	return rows[0]
}

// TestWarnExpiry drives the warn commands and the warn limit with the group's warn expiry on: warns are
// stored through the real /warn handler at a moved punishment clock, and /warns, the limit, /rmwarn, the
// Remove warn button, /resetwarns, /warnings and the AI spam warn count all count only the warns newer
// than the expiry. With expiry off every warn counts and no line is added.
func TestWarnExpiry(t *testing.T) {
	withModerationGoldenLocales(t)
	tr := i18n.MustNewTranslator("uk")
	note := func(days int) string {
		text, _ := tr.GetString("warns_expiry_note", i18n.TranslationParams{"duration": punishDurationLabel(tr, days*86400)})
		return text
	}
	day := 24 * time.Hour
	noWarns, _ := tr.GetString("warns_no_warns_to_remove")
	removedBy, _ := tr.GetString("warns_removed_by")
	removedText := fmt.Sprintf(removedBy, formatting.MentionHtml(timedGoldenAdmin.Id, timedGoldenAdmin.FirstName))

	t.Run("display", func(t *testing.T) {
		s, at := warnExpirySite(t, 7)
		at(s.now.Add(-8 * day))
		warnExpiryWarn(t, s, "old one", "old two")
		at(s.now)
		warnExpiryWarn(t, s, "new & <b>")
		timedGoldenCommand(t, s, warnsModule.warns, timedGoldenAdmin, "/warns", true)
		temp, _ := tr.GetString("warns_user_warnings_list")
		want := fmt.Sprintf(temp, 1, 3) + "\n - new & <b>\n" + note(7)
		if got := warnTimedLastText(t, s); got != want {
			t.Errorf("/warns = %q, want %q", got, want)
		}
	})

	t.Run("limit", func(t *testing.T) {
		s, at := warnExpirySite(t, 7)
		at(s.now.Add(-8 * day))
		warnExpiryWarn(t, s, "old one", "old two")
		at(s.now.Add(-day))
		warnExpiryWarn(t, s, "recent")
		at(s.now)
		warnExpiryWarn(t, s, "now one")
		if n := len(s.client.callsFor("restrictChatMember")); n != 0 {
			t.Fatalf("the warn counted 2 of 3 but %d restrict calls were made", n)
		}
		if row := warnExpiryRow(t, s); row.NumWarns != 2 || !reflect.DeepEqual([]string(row.Reasons), []string{"recent", "now one"}) {
			t.Fatalf("row = %d %v, want the two counted warns only", row.NumWarns, row.Reasons)
		}
		warnExpiryWarn(t, s, "now two")
		if n := len(s.client.callsFor("restrictChatMember")); n != 1 {
			t.Fatalf("restrict calls = %d, want 1 at the third counted warn", n)
		}
		if row := warnExpiryRow(t, s); row.NumWarns != -1 {
			t.Fatalf("row after the limit = %+v, want the warns reset", row)
		}
	})

	t.Run("rmwarn", func(t *testing.T) {
		s, at := warnExpirySite(t, 7)
		at(s.now.Add(-8 * day))
		warnExpiryWarn(t, s, "old")
		at(s.now)
		timedGoldenCommand(t, s, warnsModule.removeWarn, timedGoldenAdmin, "/rmwarn", true)
		if got := warnTimedLastText(t, s); got != noWarns {
			t.Fatalf("/rmwarn with every warn expired = %q, want %q", got, noWarns)
		}
		if row := warnExpiryRow(t, s); row.NumWarns != 0 || len(row.Reasons) != 0 {
			t.Fatalf("row = %d %v, want the expired warn dropped", row.NumWarns, row.Reasons)
		}
		warnExpiryWarn(t, s, "n1", "n2")
		timedGoldenCommand(t, s, warnsModule.removeWarn, timedGoldenAdmin, "/rmwarn", true)
		if got := warnTimedLastText(t, s); got != removedText {
			t.Fatalf("/rmwarn = %q, want %q", got, removedText)
		}
		if row := warnExpiryRow(t, s); row.NumWarns != 1 || !reflect.DeepEqual([]string(row.Reasons), []string{"n1"}) {
			t.Fatalf("row = %d %v, want [n1]", row.NumWarns, row.Reasons)
		}
	})

	t.Run("button", func(t *testing.T) {
		s, at := warnExpirySite(t, 7)
		lastEdit := func() string {
			calls := s.client.callsFor("editMessageText")
			if len(calls) == 0 {
				t.Fatal("no editMessageText request")
			}
			return fmt.Sprint(calls[len(calls)-1].Params["text"])
		}
		at(s.now.Add(-8 * day))
		warnExpiryWarn(t, s, "old")
		at(s.now)
		timedGoldenButton(t, s)
		if got := lastEdit(); got != noWarns {
			t.Fatalf("Remove warn with every warn expired = %q, want %q", got, noWarns)
		}
		warnExpiryWarn(t, s, "n1", "n2")
		timedGoldenButton(t, s)
		if got := lastEdit(); got != removedText {
			t.Fatalf("Remove warn = %q, want %q", got, removedText)
		}
		if row := warnExpiryRow(t, s); row.NumWarns != 1 {
			t.Fatalf("row = %d warns, want 1", row.NumWarns)
		}
	})

	t.Run("resetwarns", func(t *testing.T) {
		s, at := warnExpirySite(t, 7)
		at(s.now.Add(-8 * day))
		warnExpiryWarn(t, s, "old")
		at(s.now)
		timedGoldenCommand(t, s, warnsModule.resetWarns, timedGoldenAdmin, "/resetwarns", true)
		if got := warnTimedLastText(t, s); got != noWarns {
			t.Fatalf("/resetwarns with every warn expired = %q, want %q", got, noWarns)
		}
		if row := warnExpiryRow(t, s); row.NumWarns != -1 {
			t.Fatalf("row = %+v, want it deleted", row)
		}
	})

	t.Run("warnings", func(t *testing.T) {
		s, _ := warnExpirySite(t, 30)
		timedGoldenCommand(t, s, warnsModule.warnings, timedGoldenAdmin, "/warnings", false)
		temp, _ := tr.GetString("warns_settings_display")
		line, _ := tr.GetString("warns_settings_expiry", i18n.TranslationParams{"duration": punishDurationLabel(tr, 30*86400)})
		if got, want := warnTimedLastText(t, s), fmt.Sprintf(temp, 3, "mute")+"\n"+line; got != want {
			t.Errorf("/warnings = %q, want %q", got, want)
		}
	})

	t.Run("aispam_count", func(t *testing.T) {
		s, at := warnExpirySite(t, 7)
		wall := time.Now()
		at(wall.Add(-8 * day))
		warnExpiryWarn(t, s, "old one", "old two")
		at(wall)
		warnExpiryWarn(t, s, "new")
		if got := aispamWarnCount(botSiteTarget.Id, s.chat.Id); got != 1 {
			t.Errorf("aispamWarnCount = %d, want the 1 counted warn", got)
		}
	})

	t.Run("off", func(t *testing.T) {
		s, at := warnExpirySite(t, 0)
		if err := warns.SetWarnLimit(s.chat.Id, 5); err != nil {
			t.Fatalf("SetWarnLimit() error = %v", err)
		}
		at(s.now.Add(-400 * day))
		warnExpiryWarn(t, s, "old one", "old two")
		at(s.now)
		warnExpiryWarn(t, s, "new")
		timedGoldenCommand(t, s, warnsModule.warns, timedGoldenAdmin, "/warns", true)
		temp, _ := tr.GetString("warns_user_warnings_list")
		if got, want := warnTimedLastText(t, s), fmt.Sprintf(temp, 3, 5)+"\n - old one\n - old two\n - new"; got != want {
			t.Errorf("/warns = %q, want %q", got, want)
		}
		timedGoldenCommand(t, s, warnsModule.warnings, timedGoldenAdmin, "/warnings", false)
		display, _ := tr.GetString("warns_settings_display")
		if got, want := warnTimedLastText(t, s), fmt.Sprintf(display, 5, "mute"); got != want {
			t.Errorf("/warnings = %q, want %q", got, want)
		}
	})
}
