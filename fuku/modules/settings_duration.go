package modules

import (
	"strconv"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/fuku_robot/fuku/i18n"
)

// The shared duration screen (Phase 26, D-19): a timed mute or ban set from a settings page picks its duration
// here, from eight presets. The Warnings page is its first user (plan 26-03); the Antiflood and Blacklist pages
// and the escalation ladder reuse it. Any other duration, from 1 minute to 366 days, is set by command, and a
// page draws such a stored duration in its own action line with no preset marked.

// settingsDurationPresets are the durations the screen offers, in seconds: 10 minutes, 1 hour, 6 hours, 12 hours,
// 1 day, 3 days, 7 days and 30 days (P-04), drawn two to a row in this order.
var settingsDurationPresets = []int{600, 3600, 21600, 43200, 86400, 259200, 604800, 2592000}

// settingsDurationPresetDigits is the length of the longest preset's decimal form (2592000), the longest v a press
// may carry.
const settingsDurationPresetDigits = 7

// settingsDurationPreset reads the v of a preset press: exactly the canonical decimal seconds of one of
// settingsDurationPresets (no sign, space, leading zero or unit), so a forged duration is refused in step 1 of the
// press pipeline, before any I/O.
func settingsDurationPreset(v string) (int, bool) {
	secs, ok := settingsParseNumber(v, settingsDurationPresetDigits)
	if !ok || strconv.Itoa(secs) != v {
		return 0, false
	}
	for _, preset := range settingsDurationPresets {
		if preset == secs {
			return secs, true
		}
	}
	return 0, false
}

// settingsDurationIntroKey is the intro of the duration screen of a timed action: the timed mute's or the timed
// ban's.
func settingsDurationIntroKey(action string) string {
	if action == "tban" {
		return "settings_duration_intro_tban"
	}
	return "settings_duration_intro_tmute"
}

// renderSettingsDuration draws the duration screen: the header naming the page titleKey, the intro of the timed
// action (settings_duration_intro_tmute or _tban), the hint that any other duration is set by command, then the
// presets two to a row (each punishDurationLabel in the view's language, the selected one prefixed with
// settingsSelectedPrefix; a selected value that is no preset marks nothing), then Back (backFields, the page or
// screen the caller came from) and Close. press builds the callback fields of one preset. It is pure.
func renderSettingsDuration(v settingsView, titleKey, action string, selected int,
	press func(secs int) map[string]string, backFields map[string]string, notice string,
) settingsScreen {
	body := []string{
		settingsText(v.Tr, settingsDurationIntroKey(action)),
		"",
		settingsText(v.Tr, "settings_duration_hint"),
	}
	k := newSettingsKeyboard(v)
	for i := 0; i < len(settingsDurationPresets); i += 2 {
		var row []gotgbot.InlineKeyboardButton
		for _, secs := range settingsDurationPresets[i:min(i+2, len(settingsDurationPresets))] {
			label := punishDurationLabel(v.Tr, secs)
			if secs == selected {
				label = settingsSelectedPrefix + label
			}
			if button, ok := k.button(label, press(secs)); ok {
				row = append(row, button)
			}
		}
		k.row(row...)
	}
	var nav []gotgbot.InlineKeyboardButton
	if back, ok := k.button(settingsText(v.Tr, "settings_btn_back"), backFields); ok {
		nav = append(nav, back)
	}
	if closeButton, ok := k.button(settingsText(v.Tr, "settings_btn_close"), map[string]string{"a": settingsActClose}); ok {
		nav = append(nav, closeButton)
	}
	k.row(nav...)
	return k.screen(settingsCompose(v, titleKey, body, notice, false))
}

// settingsTimedWord is the word of an effective timed action on every page that offers one (the Warnings page;
// later Antiflood, Blacklist and the escalation ladder): settings_opt_timed with the settings_opt_tmute or
// settings_opt_tban word and punishDurationLabel ("Timed mute, 1 hour(s)"). ok is false for anything else (an
// untimed action, a timed action with no valid duration, an unknown word), so the caller keeps its own word for an
// untimed or inert action.
func settingsTimedWord(tr *i18n.Translator, action string, secs int) (string, bool) {
	if (action != "tmute" && action != "tban") || !autoActionEffective(action, secs) {
		return "", false
	}
	word := settingsText(tr, "settings_opt_tmute")
	if action == "tban" {
		word = settingsText(tr, "settings_opt_tban")
	}
	return settingsText(tr, "settings_opt_timed", i18n.TranslationParams{
		"action": word, "duration": punishDurationLabel(tr, secs),
	}), true
}
