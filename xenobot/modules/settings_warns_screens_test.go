//go:build testtools

package modules

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
)

// settingsWarnsStoredModes are the stored actions a page can meet: the three the commands
// write and the three the table's CHECK also allows.
var settingsWarnsStoredModes = []string{"ban", "kick", "mute", "tban", "tmute", ""}

// settingsWarnsViews is a private and a group menu view with the longest identifiers a button
// carries: a 14-character chat ID and a 10-digit user ID.
func settingsWarnsViews(tr *i18n.Translator, title string) []struct {
	name string
	view settingsView
} {
	return []struct {
		name string
		view settingsView
	}{
		{"private", settingsView{
			Tr: tr, Private: true, OwnerKey: "c", OwnerValue: "-1001234567890",
			GroupID: -1001234567890, GroupTitle: title, OpenerName: title,
		}},
		{"group", settingsView{
			Tr: tr, OwnerKey: "u", OwnerValue: "9999999999",
			GroupID: -1001234567890, GroupTitle: title, OpenerName: title,
		}},
	}
}

func TestSettingsWarnsCallbackBytes(t *testing.T) {
	newSettingsEnv(t) // installs the marker locale the translator reads
	tr := i18n.MustNewTranslator("en")
	for _, variant := range settingsWarnsViews(tr, "G") {
		t.Run(variant.name, func(t *testing.T) {
			largest := 0
			for _, limit := range []int{1, 3, 100} {
				for _, mode := range settingsWarnsStoredModes {
					name := fmt.Sprintf("limit %d mode %q", limit, mode)
					screen := renderSettingsWarns(variant.view, &models.WarnSettings{WarnLimit: limit, WarnMode: mode}, "")
					if screen.Omitted != 0 {
						t.Errorf("%s omitted %d buttons for overflow, want none", name, screen.Omitted)
					}
					if widths := settingsWarnsWidths(screen.Keyboard.InlineKeyboard); fmt.Sprint(widths) != "[5 3 2 1 2]" {
						t.Errorf("%s keyboard widths = %v, want [5 3 2 1 2]", name, widths)
					}
					for _, button := range settingsAllButtons(screen.Keyboard.InlineKeyboard) {
						n := len(button.CallbackData)
						if n < 1 || n > 64 {
							t.Errorf("%s button %q has %d bytes of callback data, want 1..64", name, button.Text, n)
						}
						largest = max(largest, n)
					}
				}
			}
			want := 48
			if !variant.view.Private {
				want = 44
			}
			if largest != want {
				t.Errorf("the largest button is %d bytes, want %d", largest, want)
			}

			// The warn expiry (plan 26-07): the page with every expiry the screen offers keeps its rows and its largest
			// button; the expiry screen, with each preset stored and with a stored value that is no preset, has rows
			// [2 2 2 1 2] and its largest button is the 90-day preset.
			largestExpiry := 0
			for _, days := range append(slices.Clone(settingsWarnsExpiryPresets), 5) {
				name := fmt.Sprintf("expiry %d", days)
				stored := &models.WarnSettings{WarnLimit: 100, WarnMode: "tmute", WarnModeSecs: punishMaxSecs, ExpiryDays: days}
				page := renderSettingsWarns(variant.view, stored, "")
				if widths := settingsWarnsWidths(page.Keyboard.InlineKeyboard); fmt.Sprint(widths) != "[5 3 2 1 2]" {
					t.Errorf("%s page widths = %v, want [5 3 2 1 2]", name, widths)
				}
				screen := renderSettingsWarnsExpiry(variant.view, stored)
				if widths := settingsWarnsWidths(screen.Keyboard.InlineKeyboard); fmt.Sprint(widths) != "[2 2 2 1 2]" {
					t.Errorf("%s expiry screen widths = %v, want [2 2 2 1 2]", name, widths)
				}
				for i, s := range []settingsScreen{page, screen} {
					if s.Omitted != 0 {
						t.Errorf("%s omitted %d buttons for overflow, want none", name, s.Omitted)
					}
					for _, button := range settingsAllButtons(s.Keyboard.InlineKeyboard) {
						n := len(button.CallbackData)
						if n < 1 || n > 64 {
							t.Errorf("%s button %q has %d bytes of callback data, want 1..64", name, button.Text, n)
						}
						if i == 0 {
							if n > want {
								t.Errorf("%s page button %q has %d bytes, more than the page's largest %d", name, button.Text, n, want)
							}
							continue
						}
						largestExpiry = max(largestExpiry, n)
					}
				}
			}
			wantExpiry := 38 // a=nx&c=<14-character chat>&v=90
			if !variant.view.Private {
				wantExpiry = 34
			}
			if largestExpiry != wantExpiry {
				t.Errorf("the expiry screen's largest button is %d bytes, want %d", largestExpiry, wantExpiry)
			}
		})
	}
}

func TestSettingsWarnsPageFitsEveryLocale(t *testing.T) {
	withRealLocales(t)
	titles := map[string]string{
		"ampersands": strings.Repeat("&", 64),
		"astral":     strings.Repeat("😀", 64),
		"multibyte":  strings.Repeat("Ж", 64),
	}
	for _, code := range []string{"uk", "en", "es", "fr", "hi", "id", "pt", "ru"} {
		t.Run(code, func(t *testing.T) {
			tr := i18n.MustNewTranslator(code)
			notices := []string{
				settingsNoticeWarn(fmt.Sprintf(settingsText(tr, "warns_mode_unknown"), strings.Repeat("a", settingsFloodModeMaxLen))),
				settingsNoticeWarn(settingsText(tr, "connections_is_user_connected_bot_not_admin")),
				settingsNoticeWarn(settingsText(tr, "common_settings_save_failed")),
			}
			for titleName, title := range titles {
				for _, variant := range settingsWarnsViews(tr, title) {
					for _, mode := range []string{"tmute", ""} {
						for _, notice := range notices {
							label := fmt.Sprintf("%s %s %s mode %q", code, titleName, variant.name, mode)
							screen := renderSettingsWarns(variant.view, &models.WarnSettings{WarnLimit: 100, WarnMode: mode}, notice)
							if units := panelUTF16Len(screen.Text); units > 3800 || units == 0 {
								t.Errorf("%s is %d UTF-16 units, want 1..3800", label, units)
							}
							if widths := settingsWarnsWidths(screen.Keyboard.InlineKeyboard); fmt.Sprint(widths) != "[5 3 2 1 2]" {
								t.Errorf("%s keyboard widths = %v, want [5 3 2 1 2]", label, widths)
							}
							if screen.Omitted != 0 {
								t.Errorf("%s omitted %d buttons", label, screen.Omitted)
							}
							for _, placeholder := range []string{"{n}", "{action}", "%!", "%s"} {
								if strings.Contains(screen.Text, placeholder) {
									t.Errorf("%s page keeps the unfilled %s", label, placeholder)
								}
							}
							for _, button := range settingsAllButtons(screen.Keyboard.InlineKeyboard) {
								if strings.TrimSpace(button.Text) == "" {
									t.Errorf("%s has a button with an empty label", label)
								}
							}
						}
					}
					// The fullest state of plan 26-07 (an effective timed action with the longest label, expiry 90 days)
					// under the longest notice, and the expiry screen with 90 days and with 14 (the weeks' label) stored.
					fullest := &models.WarnSettings{WarnLimit: 100, WarnMode: "tmute", WarnModeSecs: punishMaxSecs, ExpiryDays: 90}
					screens := map[string]settingsScreen{}
					for i, notice := range notices {
						screens[fmt.Sprintf("page notice %d", i)] = renderSettingsWarns(variant.view, fullest, notice)
					}
					for _, days := range []int{90, 14} {
						stored := *fullest
						stored.ExpiryDays = days
						screens[fmt.Sprintf("expiry screen %d", days)] = renderSettingsWarnsExpiry(variant.view, &stored)
					}
					for name, screen := range screens {
						label := fmt.Sprintf("%s %s %s %s", code, titleName, variant.name, name)
						if units := panelUTF16Len(screen.Text); units > 3800 || units == 0 {
							t.Errorf("%s is %d UTF-16 units, want 1..3800", label, units)
						}
						if screen.Omitted != 0 {
							t.Errorf("%s omitted %d buttons", label, screen.Omitted)
						}
						if strings.ContainsAny(screen.Text, "{}") || strings.Contains(screen.Text, "%!") {
							t.Errorf("%s keeps an unfilled placeholder: %q", label, screen.Text)
						}
						for _, row := range screen.Keyboard.InlineKeyboard {
							ceiling := 20
							if len(row) == 1 {
								ceiling = 40 // a full-width button: the expiry button
							}
							for _, button := range row {
								if runes := len([]rune(button.Text)); runes > ceiling || strings.TrimSpace(button.Text) == "" {
									t.Errorf("%s button %q has %d code points, want 1..%d", label, button.Text, runes, ceiling)
								}
							}
						}
					}
				}
			}
		})
	}
}
