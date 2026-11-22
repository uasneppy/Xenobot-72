//go:build testtools

package modules

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/uasneppy/fuku_robot/fuku/i18n"
)

// settingsDurationTestPress is the longest field set a caller of the duration screen builds today: the
// Warnings page's timed press.
func settingsDurationTestPress(secs int) map[string]string {
	return map[string]string{"a": "nt", "k": "m", "v": strconv.Itoa(secs)}
}

// TestSettingsDurationScreen holds the shared duration screen (D-19): the preset parser refuses anything that is
// not a preset's canonical decimal, the screen draws the eight presets two to a row with the stored one marked,
// and every cell, body and button fits in all 8 languages. settingsTimedWord reads an effective timed action only.
func TestSettingsDurationScreen(t *testing.T) {
	t.Run("the preset parser", func(t *testing.T) {
		for _, secs := range settingsDurationPresets {
			if got, ok := settingsDurationPreset(strconv.Itoa(secs)); !ok || got != secs {
				t.Errorf("settingsDurationPreset(%d) = %d, %v, want the preset", secs, got, ok)
			}
		}
		for _, v := range []string{"", "0", "0600", "+600", "601", "2592001", "1h", " 600", "600 ", "-600", "60O"} {
			if got, ok := settingsDurationPreset(v); ok {
				t.Errorf("settingsDurationPreset(%q) = %d, true, want refused", v, got)
			}
		}
	})

	withRealLocales(t)

	t.Run("the shape and the mark", func(t *testing.T) {
		tr := i18n.MustNewTranslator("en")
		view := settingsWarnsViews(tr, "G")[0].view
		back := map[string]string{"a": settingsActOpen, "p": settingsPageWarns}
		for _, selected := range []int{0, 3600, 2592000, 5400} {
			screen := renderSettingsDuration(view, "settings_page_warns", "tmute", selected, settingsDurationTestPress, back, "")
			if widths := settingsWarnsWidths(screen.Keyboard.InlineKeyboard); fmt.Sprint(widths) != "[2 2 2 2 2]" {
				t.Fatalf("selected %d: keyboard widths = %v, want [2 2 2 2 2]", selected, widths)
			}
			marked := 0
			for i, secs := range settingsDurationPresets {
				button := screen.Keyboard.InlineKeyboard[i/2][i%2]
				fields := settingsButtonFields(t, button)
				if fields["a"] != "nt" || fields["k"] != "m" || fields["v"] != strconv.Itoa(secs) || fields["c"] == "" {
					t.Errorf("preset %d fields = %v, want a=nt k=m v=%d and the owner field", secs, fields, secs)
				}
				if strings.HasPrefix(button.Text, settingsSelectedPrefix) {
					marked++
					if secs != selected {
						t.Errorf("selected %d: preset %d (%q) is marked", selected, secs, button.Text)
					}
				}
			}
			wantMarked := 0
			if _, ok := settingsDurationPreset(strconv.Itoa(selected)); ok {
				wantMarked = 1
			}
			if marked != wantMarked {
				t.Errorf("selected %d: %d cells are marked, want %d", selected, marked, wantMarked)
			}
			nav := screen.Keyboard.InlineKeyboard[4]
			if fields := settingsButtonFields(t, nav[0]); fields["a"] != settingsActOpen || fields["p"] != settingsPageWarns {
				t.Errorf("Back fields = %v, want the caller's back fields", fields)
			}
			if fields := settingsButtonFields(t, nav[1]); fields["a"] != settingsActClose {
				t.Errorf("Close fields = %v, want a=cl", fields)
			}
		}
		tmute := renderSettingsDuration(view, "settings_page_warns", "tmute", 0, settingsDurationTestPress, back, "")
		tban := renderSettingsDuration(view, "settings_page_warns", "tban", 0, settingsDurationTestPress, back, "")
		if !strings.Contains(tmute.Text, settingsText(tr, "settings_duration_intro_tmute")) ||
			!strings.Contains(tban.Text, settingsText(tr, "settings_duration_intro_tban")) {
			t.Errorf("the intros do not follow the action: tmute %q, tban %q", tmute.Text, tban.Text)
		}
		if !strings.Contains(tmute.Text, settingsText(tr, "settings_duration_hint")) {
			t.Errorf("the screen %q does not carry the hint", tmute.Text)
		}
	})

	t.Run("the timed word", func(t *testing.T) {
		tr := i18n.MustNewTranslator("en")
		if word, ok := settingsTimedWord(tr, "tmute", 3600); !ok || word != "Timed mute, 1 hour(s)" {
			t.Errorf("settingsTimedWord(tmute, 3600) = %q, %v, want Timed mute, 1 hour(s)", word, ok)
		}
		if word, ok := settingsTimedWord(tr, "tban", 5400); !ok || word != "Timed ban, 90 minute(s)" {
			t.Errorf("settingsTimedWord(tban, 5400) = %q, %v, want Timed ban, 90 minute(s)", word, ok)
		}
		for _, tc := range []struct {
			action string
			secs   int
		}{{"tmute", 0}, {"tban", 30}, {"mute", 3600}, {"warn", 3600}, {"", 3600}} {
			if word, ok := settingsTimedWord(tr, tc.action, tc.secs); ok || word != "" {
				t.Errorf("settingsTimedWord(%q, %d) = %q, %v, want no word", tc.action, tc.secs, word, ok)
			}
		}
	})

	t.Run("every language fits", func(t *testing.T) {
		title := strings.Repeat("&", 64)
		for _, code := range []string{"uk", "en", "es", "fr", "hi", "id", "pt", "ru"} {
			tr := i18n.MustNewTranslator(code)
			notice := settingsNoticeWarn(settingsText(tr, "connections_is_user_connected_bot_not_admin"))
			for _, variant := range settingsWarnsViews(tr, title) {
				back := map[string]string{"a": settingsActOpen, "p": settingsPageWarns}
				largest := 0
				for _, action := range []string{"tmute", "tban"} {
					label := fmt.Sprintf("%s %s %s", code, variant.name, action)
					screen := renderSettingsDuration(variant.view, "settings_page_warns", action, 2592000,
						settingsDurationTestPress, back, notice)
					if screen.Omitted != 0 {
						t.Errorf("%s omitted %d buttons", label, screen.Omitted)
					}
					if units := panelUTF16Len(screen.Text); units > 3800 || units == 0 {
						t.Errorf("%s is %d UTF-16 units, want 1..3800", label, units)
					}
					for _, button := range settingsAllButtons(screen.Keyboard.InlineKeyboard) {
						if runes := utf8.RuneCountInString(button.Text); runes > 20 || strings.TrimSpace(button.Text) == "" {
							t.Errorf("%s cell %q has %d code points, want 1..20", label, button.Text, runes)
						}
						if n := len(button.CallbackData); n < 1 || n > 64 {
							t.Errorf("%s button %q has %d bytes of callback data, want 1..64", label, button.Text, n)
						}
						largest = max(largest, len(button.CallbackData))
					}
				}
				// The largest button is the 30-day preset: 47 bytes in a private menu, 43 in a group menu.
				if want := map[bool]int{true: 47, false: 43}[variant.view.Private]; largest != want {
					t.Errorf("%s %s: the largest button is %d bytes, want %d", code, variant.name, largest, want)
				}
			}
			for _, secs := range settingsDurationPresets {
				for _, action := range []string{"tmute", "tban"} {
					word, ok := settingsTimedWord(tr, action, secs)
					if !ok || strings.ContainsAny(word, "{}") {
						t.Errorf("%s: settingsTimedWord(%s, %d) = %q, %v, want a filled word", code, action, secs, word, ok)
					}
				}
			}
		}
	})
}
