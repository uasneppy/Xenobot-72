//go:build testtools

package modules

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/uasneppy/Xenobot-72/xenobot/db/escalation"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
)

// settingsEscScreens draws every screen of the Escalation page for the longest stored shapes: a full ladder of
// every action, a one-step ladder, an empty one, each step screen with every stored action and each timed
// duration screen.
func settingsEscScreens(v settingsView) map[string]settingsScreen {
	full := &escalation.Ladder{Enabled: true, ForgetDays: 90, Steps: []escalation.Step{
		{Action: "tmute", Secs: 31622400}, {Action: "tban", Secs: 2592000}, {Action: "kick"}, {Action: "mute"}, {Action: "ban"},
	}}
	one := &escalation.Ladder{Enabled: true, ForgetDays: 1, Steps: []escalation.Step{{Action: "tban", Secs: 600}}}
	empty := &escalation.Ladder{ForgetDays: 30}
	screens := map[string]settingsScreen{}
	for name, ladder := range map[string]*escalation.Ladder{"full": full, "one": one, "empty": empty} {
		for _, notice := range []string{
			"", settingsNoticeOK(settingsText(v.Tr, "settings_esc_example_added")), settingsNoticeWarn(settingsText(v.Tr, "settings_esc_stale")),
			settingsNoticeWarn(settingsText(v.Tr, "settings_esc_refuse_last_step")), settingsNoticeWarn(settingsText(v.Tr, "common_settings_save_failed")),
		} {
			screens[fmt.Sprintf("page %s %.12s", name, notice)] = renderSettingsEscalation(v, ladder, notice)
		}
	}
	for n := 1; n <= escalation.MaxSteps; n++ {
		screens[fmt.Sprintf("step %d", n)] = renderSettingsEscStep(v, full, n, "")
	}
	for _, mode := range []string{"tmute", "tban"} {
		press := func(secs int) map[string]string {
			return map[string]string{"a": settingsActEscTimed, "t": "5", "k": mode[1:2], "v": fmt.Sprint(secs)}
		}
		back := map[string]string{"a": settingsActEscStep, "t": "5"}
		screens["duration "+mode] = renderSettingsDuration(v, "settings_page_esc", mode, 2592000, press, back, "")
	}
	return screens
}

// TestSettingsEscalationCallbackBytes holds every button of every screen at 64 bytes in the private menu (the
// only menu the page exists in), with the longest group ID and opener, and none omitted.
func TestSettingsEscalationCallbackBytes(t *testing.T) {
	newSettingsEnv(t) // installs the marker locale the translator reads
	v := settingsView{Tr: i18n.MustNewTranslator("en"), Private: true, OwnerKey: "c", OwnerValue: "-1001234567890",
		GroupID: -1001234567890, GroupTitle: "G", OpenerName: "G"}
	largest := 0
	for name, screen := range settingsEscScreens(v) {
		if screen.Omitted != 0 {
			t.Errorf("%s omitted %d buttons", name, screen.Omitted)
		}
		for _, row := range screen.Keyboard.InlineKeyboard {
			for _, button := range row {
				largest = max(largest, len(button.CallbackData))
				if len(button.CallbackData) > 64 {
					t.Errorf("%s: %q is %d bytes", name, button.CallbackData, len(button.CallbackData))
				}
			}
		}
	}
	if largest == 0 {
		t.Fatal("no callback data measured")
	}
}

// TestSettingsEscalationPageFitsEveryLocale holds every screen under 3800 UTF-16 units with a 64-rune title in
// 8 languages, the row shapes, and the half-width cells at 20 code points.
func TestSettingsEscalationPageFitsEveryLocale(t *testing.T) {
	withRealLocales(t)
	titles := map[string]string{"ampersands": strings.Repeat("&", 64), "astral": strings.Repeat("😀", 64), "multibyte": strings.Repeat("Ж", 64)}
	wantShape := map[string]string{"full": "[1 5 1 4 2]", "one": "[1 1 2 4 2]", "empty": "[1 1 4 2]"}
	for _, code := range []string{"uk", "en", "es", "fr", "hi", "id", "pt", "ru"} {
		t.Run(code, func(t *testing.T) {
			tr := i18n.MustNewTranslator(code)
			for titleName, title := range titles {
				v := settingsView{Tr: tr, Private: true, OwnerKey: "c", OwnerValue: "-1001234567890",
					GroupID: -1001234567890, GroupTitle: title, OpenerName: title}
				for name, screen := range settingsEscScreens(v) {
					label := code + " " + titleName + " " + name
					if units := panelUTF16Len(screen.Text); units == 0 || units > 3800 {
						t.Errorf("%s is %d UTF-16 units, want 1..3800", label, units)
					}
					if screen.Omitted != 0 {
						t.Errorf("%s omitted %d buttons", label, screen.Omitted)
					}
					for _, row := range screen.Keyboard.InlineKeyboard {
						for _, button := range row {
							limit := 40
							if len(row) > 1 {
								limit = 20
							}
							if runes := utf8.RuneCountInString(button.Text); runes > limit {
								t.Errorf("%s: button %q has %d code points, limit %d", label, button.Text, runes, limit)
							}
						}
					}
					for prefix, want := range wantShape {
						if strings.HasPrefix(name, "page "+prefix+" ") {
							if got := fmt.Sprint(settingsEscRowShape(screen.Keyboard.InlineKeyboard)); got != want {
								t.Errorf("%s rows = %s, want %s", label, got, want)
							}
						}
					}
				}
			}
		})
	}
}
