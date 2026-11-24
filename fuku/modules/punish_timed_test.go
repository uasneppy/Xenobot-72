//go:build testtools

package modules

import (
	"testing"
	"time"

	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
)

// TestPunishTimedCore holds the pure core of the timed punishments: the duration grammar and its
// bounds, the short and the translated wording of a duration, the end date and its local refusal, and
// the one rule that says whether a stored action acts.
func TestPunishTimedCore(t *testing.T) {
	t.Run("parsePunishDuration", func(t *testing.T) {
		for word, want := range map[string]int{"1m": 60, "2h": 7200, "1d": 86400, "52w": 52 * 604800, "366d": 366 * 86400} {
			got, ok := parsePunishDuration(word)
			if !ok || got != want {
				t.Errorf("parsePunishDuration(%q) = %d, %v; want %d, true", word, got, ok, want)
			}
		}
		for _, word := range []string{"0m", "367d", "1.5h", "2H", "12", "", "-1d", "53w"} {
			if got, ok := parsePunishDuration(word); ok {
				t.Errorf("parsePunishDuration(%q) = %d, true; want a refusal", word, got)
			}
		}
	})

	t.Run("punishDurationShort", func(t *testing.T) {
		for secs, want := range map[int]string{5400: "90m", 7200: "2h", 86400: "1d", 604800: "1w", 2592000: "30d", 60: "1m"} {
			if got := punishDurationShort(secs); got != want {
				t.Errorf("punishDurationShort(%d) = %q, want %q", secs, got, want)
			}
		}
	})

	t.Run("punishDurationLabel", func(t *testing.T) {
		withRealLocales(t)
		cases := []struct {
			secs   int
			key    string
			amount int
		}{
			{1209600, "staff_act_duration_weeks", 2},
			{172800, "staff_act_duration_days", 2},
			{7200, "staff_act_duration_hours", 2},
			{5400, "staff_act_duration_minutes", 90},
		}
		for _, lang := range []string{"uk", "en"} {
			tr := i18n.MustNewTranslator(lang)
			for _, c := range cases {
				want, _ := tr.GetString(c.key, i18n.TranslationParams{"n": c.amount})
				if want == "" {
					t.Fatalf("%s: %s has no text", lang, c.key)
				}
				if got := punishDurationLabel(tr, c.secs); got != want {
					t.Errorf("%s: punishDurationLabel(%d) = %q, want %q", lang, c.secs, got, want)
				}
			}
		}
	})

	t.Run("punishUntil and punishUntilTooClose", func(t *testing.T) {
		now := time.Unix(1_800_000_000, 0)
		mreqSwap(t, &punishNow, func() time.Time { return now })
		if _, ok := punishUntil(now, 29); ok {
			t.Error("punishUntil(29 s) accepted, want a refusal")
		}
		if _, ok := punishUntil(now, punishMaxSecs+1); ok {
			t.Error("punishUntil(366 days + 1 s) accepted, want a refusal")
		}
		if until, ok := punishUntil(now, 7200); !ok || until != now.Unix()+7200 {
			t.Errorf("punishUntil(7200) = %d, %v; want %d, true", until, ok, now.Unix()+7200)
		}
		if !punishUntilTooClose(now.Unix() + 34) {
			t.Error("an end 34 s ahead is not too close, want too close")
		}
		if punishUntilTooClose(now.Unix() + 36) {
			t.Error("an end 36 s ahead is too close, want it sent")
		}
	})

	t.Run("autoActionEffective", func(t *testing.T) {
		effective := []struct {
			action string
			secs   int
		}{{"mute", 0}, {"kick", 0}, {"ban", 0}, {"tmute", 60}, {"tban", punishMaxSecs}}
		for _, c := range effective {
			if !autoActionEffective(c.action, c.secs) {
				t.Errorf("autoActionEffective(%q, %d) = false, want true", c.action, c.secs)
			}
		}
		inert := []struct {
			action string
			secs   int
		}{{"tmute", 0}, {"tban", 59}, {"tban", punishMaxSecs + 1}, {"warn", 0}, {"none", 0}, {"delete", 0}, {"", 0}}
		for _, c := range inert {
			if autoActionEffective(c.action, c.secs) {
				t.Errorf("autoActionEffective(%q, %d) = true, want false", c.action, c.secs)
			}
		}
		for _, secs := range []int{0, 59, 60, 7200, punishMaxSecs, punishMaxSecs + 1} {
			if punishDurationValid(secs) != models.PunishDurationValid(secs) {
				t.Errorf("punishDurationValid(%d) disagrees with models.PunishDurationValid", secs)
			}
		}
		if !(autoPunishment{Action: "tmute"}).Timed() || !(autoPunishment{Action: "tban"}).Timed() || (autoPunishment{Action: "mute"}).Timed() {
			t.Error("autoPunishment.Timed must hold for tmute and tban only")
		}
	})

	t.Run("applyTimedPunishment clamps 366d below upper limit", func(t *testing.T) {
		now := time.Unix(1_800_000_000, 0)
		mreqSwap(t, &punishNow, func() time.Time { return now })
		bot := newModuleTestBot(&moduleBotClient{})
		until, err := applyTimedPunishment(bot, -1001, 42, "tmute", punishMaxSecs)
		if err != nil {
			t.Fatalf("applyTimedPunishment(punishMaxSecs) error = %v", err)
		}
		want := now.Unix() + int64(punishMaxSecs) - int64(punishMaxMargin/time.Second)
		if until != want {
			t.Fatalf("applyTimedPunishment(punishMaxSecs) until = %d, want %d", until, want)
		}
		short, err := applyTimedPunishment(bot, -1001, 42, "tban", 3600)
		if err != nil {
			t.Fatalf("applyTimedPunishment(3600) error = %v", err)
		}
		if short != now.Unix()+3600 {
			t.Fatalf("applyTimedPunishment(3600) until = %d, want %d", short, now.Unix()+3600)
		}
	})
}
