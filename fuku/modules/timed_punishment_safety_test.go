//go:build testtools

package modules

import (
	"html"
	"strings"
	"testing"

	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
)

// TestTimedNoticeTextsFitEveryLocale fills each timed default text with a 64-rune name full of markup, the longest
// duration label of the language and a 200-character reason: it fits 4096 UTF-16 units, leaves no placeholder
// unfilled and shows the name only escaped (D-15, D-17).
func TestTimedNoticeTextsFitEveryLocale(t *testing.T) {
	withRealLocales(t)
	name := strings.Repeat(`<&"'>`, 13)[:64]
	reason := strings.Repeat("р&<", 67)[:200]
	keys := []string{
		"warns_limit_tmuted", "warns_limit_tbanned",
		"antiflood_checkflood_timed_mute", "antiflood_checkflood_timed_ban",
		"blacklists_bl_watcher_tmuted_user", "blacklists_bl_watcher_tbanned_user",
	}
	for _, code := range []string{"uk", "en", "es", "fr", "hi", "id", "pt", "ru"} {
		t.Run(code, func(t *testing.T) {
			tr := i18n.MustNewTranslator(code)
			longest := ""
			for _, secs := range []int{models.PunishMinSecs, 3600, 86400, 604800, 2592000, models.PunishMaxSecs} {
				if label := punishDurationLabel(tr, secs); len([]rune(label)) > len([]rune(longest)) {
					longest = label
				}
			}
			for _, key := range keys {
				text, err := tr.GetString(key, i18n.TranslationParams{
					"user": html.EscapeString(name), "duration": longest, "reason": html.EscapeString(reason), "count": 100, "limit": 100,
				})
				if err != nil || text == "" {
					t.Errorf("%s/%s: %q, %v", code, key, text, err)
					continue
				}
				if units := panelUTF16Len(text); units > 4096 {
					t.Errorf("%s/%s is %d UTF-16 units, want at most 4096", code, key, units)
				}
				if strings.Contains(text, "{") || strings.Contains(text, "%") {
					t.Errorf("%s/%s left a placeholder unfilled: %q", code, key, text)
				}
				if strings.Contains(text, name) {
					t.Errorf("%s/%s shows the name unescaped", code, key)
				}
				if !strings.Contains(text, html.EscapeString(name)) {
					t.Errorf("%s/%s lost the name", code, key)
				}
			}
		})
	}
}
