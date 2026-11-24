package modules

import (
	"context"
	"strconv"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/db/escalation"
	"github.com/uasneppy/Xenobot-72/xenobot/db/staff"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
)

// The Escalation page (page j, Phase 26, TUNE-02, D-31): the owner turns a group's escalation ladder on,
// defines its steps (up to five, each a mute, kick, ban, timed mute or timed ban) and says how long a member's
// offences are remembered. It exists only in the bot's private chat (PrivateOnly: a group menu shows the
// note and no page code runs) and is hidden in a Staff Group, so no ladder can look like a Staff Group
// default. There is no command, so every write goes through one apply function (escalation_settings.go)
// and the drift and authority tests stand in for a parity test. Every write needs settingsRightRestrict on
// the press's live member record; viewing and opening a step or a duration screen need the admin check alone.

const (
	// settingsPageEscalation is the p value of the page.
	settingsPageEscalation = "j"
	// The action codes: the toggle, the forget choice, a step screen, an untimed step, a step's duration screen,
	// a duration preset and Remove last.
	settingsActEscToggle   = "je"
	settingsActEscForget   = "jf"
	settingsActEscStep     = "js"
	settingsActEscSet      = "ja"
	settingsActEscDuration = "jd"
	settingsActEscTimed    = "jt"
	settingsActEscRemove   = "jr"
)

// settingsEscRead reads the ladder fresh for the page; a variable only so a test can make it fail or count.
var settingsEscRead = escalation.GetFresh

// settingsEscStaffLookup reads a chat's Staff Group row fresh for the page's Hidden: its own seam, a variable
// only so a test can make it fail.
var settingsEscStaffLookup = staff.GetStaffGroupFresh

// settingsEscUntimed are the actions of the untimed choice row, in order.
var settingsEscUntimed = []string{"mute", "kick", "ban"}

func init() {
	// The owner field c exists only on a private menu's buttons, so every Validate requires it: a group menu's
	// press is an invalid request in step 1, before any lookup.
	private := func(check func(map[string]string) bool) func(map[string]string) bool {
		return func(fields map[string]string) bool { return fields["c"] != "" && check(fields) }
	}
	registerSettingsPage(settingsPage{
		Key:            settingsPageEscalation,
		Order:          43,
		Icon:           "🪜",
		TitleKey:       "settings_page_esc",
		MainState:      settingsEscMainState,
		Open:           settingsEscOpen,
		Hidden:         settingsEscHidden,
		PrivateOnly:    true,
		PrivateNoteKey: "settings_esc_group_note",
		Actions: map[string]settingsPageAction{
			settingsActEscToggle: {
				Fields: []string{"v"}, Right: settingsRightRestrict, Run: settingsEscRunToggle,
				Validate: private(func(f map[string]string) bool { return settingsEditSwitchValue(f["v"]) }),
			},
			settingsActEscForget: {
				Fields: []string{"v"}, Right: settingsRightRestrict, Run: settingsEscRunForget,
				Validate: private(func(f map[string]string) bool { _, ok := settingsEscForgetOf(f["v"]); return ok }),
			},
			settingsActEscStep: {
				Fields: []string{"t"}, Right: settingsRightAdmin, Run: settingsEscRunStep,
				Validate: private(func(f map[string]string) bool { _, ok := settingsEscStepOf(f["t"]); return ok }),
			},
			settingsActEscSet: {
				Fields: []string{"t", "v"}, Right: settingsRightRestrict, Run: settingsEscRunSet,
				Validate: private(func(f map[string]string) bool {
					_, ok := settingsEscStepOf(f["t"])
					return ok && slicesContainsString(settingsEscUntimed, f["v"])
				}),
			},
			settingsActEscDuration: {
				Fields: []string{"t", "k"}, Right: settingsRightAdmin, Run: settingsEscRunDuration,
				Validate: private(func(f map[string]string) bool {
					_, ok := settingsEscStepOf(f["t"])
					_, timed := settingsWarnsTimedMode(f["k"])
					return ok && timed
				}),
			},
			settingsActEscTimed: {
				Fields: []string{"t", "k", "v"}, Right: settingsRightRestrict, Run: settingsEscRunTimed,
				Validate: private(func(f map[string]string) bool {
					_, ok := settingsEscStepOf(f["t"])
					_, timed := settingsWarnsTimedMode(f["k"])
					_, preset := settingsDurationPreset(f["v"])
					return ok && timed && preset
				}),
			},
			settingsActEscRemove: {
				Fields: []string{"t"}, Right: settingsRightRestrict, Run: settingsEscRunRemove,
				Validate: private(func(f map[string]string) bool { _, ok := settingsEscStepOf(f["t"]); return ok }),
			},
		},
	})
}

func slicesContainsString(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

// settingsEscStepOf reads a step index of a press: one digit, 1 to escalation.MaxSteps, canonical.
func settingsEscStepOf(v string) (int, bool) {
	n, ok := settingsParseNumber(v, 1)
	if !ok || n < 1 || n > escalation.MaxSteps || strconv.Itoa(n) != v {
		return 0, false
	}
	return n, true
}

// settingsEscForgetOf reads the v of a forget press: the canonical decimal days of one of escalationForgetPresets.
func settingsEscForgetOf(v string) (int, bool) {
	days, ok := settingsParseNumber(v, 2)
	if !ok || strconv.Itoa(days) != v || !escalationForgetPreset(days) {
		return 0, false
	}
	return days, true
}

// settingsEscHidden is the page's Hidden: true only for a definite Staff Group row, read fresh. An unreadable
// row shows the page.
func settingsEscHidden(group *gotgbot.Chat) bool {
	row, err := settingsEscStaffLookup(group.Id)
	return err == nil && row != nil
}

func settingsEscReadNow(chatID int64) (*escalation.Ladder, error) {
	ctx, cancel := context.WithTimeout(context.Background(), settingsReadTimeout)
	defer cancel()
	return settingsEscRead(ctx, chatID)
}

// settingsEscMainState is the page's Main line: Off, or the number of steps.
func settingsEscMainState(_ *gotgbot.Bot, tr *i18n.Translator, group *gotgbot.Chat) (string, bool) {
	ladder, err := settingsEscReadNow(group.Id)
	if err != nil || ladder == nil {
		return "", false
	}
	if !ladder.Enabled {
		return settingsText(tr, "settings_state_off_plain"), true
	}
	return settingsText(tr, "settings_state_esc_steps", i18n.TranslationParams{"n": len(ladder.Steps)}), true
}

// settingsEscOpen draws the page. A read error is the cannot-open edit.
func settingsEscOpen(p *settingsPress, notice string) (settingsScreen, error) {
	ladder, err := settingsEscReadNow(p.group.Id)
	if err != nil {
		return settingsScreen{}, err
	}
	if ladder == nil {
		d := escalation.Defaults()
		ladder = &d
	}
	return renderSettingsEscalation(p.view(), ladder, notice), nil
}

// settingsEscActionWord is the word of a step: the timed word for a timed step, the option word otherwise.
func settingsEscActionWord(tr *i18n.Translator, s escalation.Step) string {
	if word, ok := settingsTimedWord(tr, s.Action, s.Secs); ok {
		return word
	}
	return settingsText(tr, "settings_opt_"+s.Action)
}

// renderSettingsEscalation draws the page: the intro, the state, one line per step (the last one "n and
// later"), the forget line and the hint; then the toggle, the step buttons, Add and Remove last, the forget
// choice, Back and Close. It is pure and names no Staff Group, member or count of offences.
func renderSettingsEscalation(v settingsView, ladder *escalation.Ladder, notice string) settingsScreen {
	body := []string{
		settingsText(v.Tr, "settings_esc_intro"),
		"",
		settingsText(v.Tr, "settings_esc_state", i18n.TranslationParams{"state": settingsStateWord(v.Tr, ladder.Enabled)}),
	}
	if len(ladder.Steps) == 0 {
		body = append(body, settingsText(v.Tr, "settings_esc_no_steps"))
	}
	for i, s := range ladder.Steps {
		key := "settings_esc_step_line"
		if i == len(ladder.Steps)-1 {
			key = "settings_esc_step_line_last"
		}
		body = append(body, settingsText(v.Tr, key, i18n.TranslationParams{"n": i + 1, "action": settingsEscActionWord(v.Tr, s)}))
	}
	body = append(body,
		settingsText(v.Tr, "settings_esc_forget_line", i18n.TranslationParams{"days": punishDurationLabel(v.Tr, ladder.ForgetDays*86400)}),
		"", settingsText(v.Tr, "settings_esc_hint"))

	k := newSettingsKeyboard(v)
	toggle := settingsText(v.Tr, "settings_btn_esc_toggle", i18n.TranslationParams{"state": settingsStateWord(v.Tr, ladder.Enabled)})
	if button, ok := k.button(toggle, map[string]string{"a": settingsActEscToggle, "v": settingsBoolValue(!ladder.Enabled)}); ok {
		k.row(button)
	}
	var steps []gotgbot.InlineKeyboardButton
	for i := range ladder.Steps {
		if button, ok := k.button(strconv.Itoa(i+1), map[string]string{"a": settingsActEscStep, "t": strconv.Itoa(i + 1)}); ok {
			steps = append(steps, button)
		}
	}
	k.row(steps...)
	var edit []gotgbot.InlineKeyboardButton
	if len(ladder.Steps) < escalation.MaxSteps {
		if button, ok := k.button(settingsText(v.Tr, "settings_btn_esc_add"),
			map[string]string{"a": settingsActEscStep, "t": strconv.Itoa(len(ladder.Steps) + 1)}); ok {
			edit = append(edit, button)
		}
	}
	if len(ladder.Steps) > 0 {
		if button, ok := k.button(settingsText(v.Tr, "settings_btn_esc_remove"),
			map[string]string{"a": settingsActEscRemove, "t": strconv.Itoa(len(ladder.Steps))}); ok {
			edit = append(edit, button)
		}
	}
	k.row(edit...)
	var forget []gotgbot.InlineKeyboardButton
	for _, days := range escalationForgetPresets {
		label := punishDurationLabel(v.Tr, days*86400)
		if days == ladder.ForgetDays {
			label = settingsSelectedPrefix + label
		}
		if button, ok := k.button(label, map[string]string{"a": settingsActEscForget, "v": strconv.Itoa(days)}); ok {
			forget = append(forget, button)
		}
	}
	k.row(forget...)
	k.navRow(false)
	return k.screen(settingsCompose(v, "settings_page_esc", body, notice, false))
}

func settingsBoolValue(on bool) string {
	if on {
		return "1"
	}
	return "0"
}

// renderSettingsEscStep draws step n's screen: the intro, the step's current line when it exists, the choice
// [Mute | Kick | Ban] with the stored action marked, the timed row [Timed mute | Timed ban] with the stored
// timed action marked, then Back to the page and Close.
func renderSettingsEscStep(v settingsView, ladder *escalation.Ladder, n int, notice string) settingsScreen {
	body := []string{settingsText(v.Tr, "settings_esc_step_intro", i18n.TranslationParams{"n": n})}
	var current escalation.Step
	if n <= len(ladder.Steps) {
		current = ladder.Steps[n-1]
		body = append(body, settingsText(v.Tr, "settings_esc_step_line", i18n.TranslationParams{
			"n": n, "action": settingsEscActionWord(v.Tr, current),
		}))
	}
	k := newSettingsKeyboard(v)
	var untimed []gotgbot.InlineKeyboardButton
	for _, action := range settingsEscUntimed {
		label := settingsText(v.Tr, "settings_opt_"+action)
		if current.Action == action {
			label = settingsSelectedPrefix + label
		}
		if button, ok := k.button(label, map[string]string{"a": settingsActEscSet, "t": strconv.Itoa(n), "v": action}); ok {
			untimed = append(untimed, button)
		}
	}
	k.row(untimed...)
	var timed []gotgbot.InlineKeyboardButton
	for _, cell := range []struct{ k, key, mode string }{
		{settingsWarnsTimedMute, "settings_opt_tmute", "tmute"},
		{settingsWarnsTimedBan, "settings_opt_tban", "tban"},
	} {
		label := settingsText(v.Tr, cell.key)
		if current.Action == cell.mode {
			label = settingsSelectedPrefix + label
		}
		if button, ok := k.button(label, map[string]string{"a": settingsActEscDuration, "t": strconv.Itoa(n), "k": cell.k}); ok {
			timed = append(timed, button)
		}
	}
	k.row(timed...)
	k.row(settingsEscBackRow(k, v)...)
	return k.screen(settingsCompose(v, "settings_page_esc", body, notice, false))
}

// settingsEscBackRow is Back to the Escalation page and Close.
func settingsEscBackRow(k *settingsKeyboard, v settingsView) []gotgbot.InlineKeyboardButton {
	var nav []gotgbot.InlineKeyboardButton
	if back, ok := k.button(settingsText(v.Tr, "settings_btn_back"), map[string]string{"a": settingsActOpen, "p": settingsPageEscalation}); ok {
		nav = append(nav, back)
	}
	if closeButton, ok := k.button(settingsText(v.Tr, "settings_btn_close"), map[string]string{"a": settingsActClose}); ok {
		nav = append(nav, closeButton)
	}
	return nav
}

// settingsEscPageScreen draws the page from a ladder; with none given it reads one fresh, and an unreadable
// ladder is the cannot-open edit.
func settingsEscPageScreen(p *settingsPress, ladder *escalation.Ladder, notice string) *settingsScreen {
	if ladder == nil {
		var err error
		ladder, err = settingsEscReadNow(p.group.Id)
		if err != nil || ladder == nil {
			log.Warnf("[Settings] the escalation ladder of chat %d could not be read: %v", p.group.Id, err)
			p.cannotOpen()
			return nil
		}
	}
	screen := renderSettingsEscalation(p.view(), ladder, notice)
	return &screen
}

// settingsEscNotice is the notice line of an apply outcome: a note for the example ladder, a warning for a
// refusal or a failed save, none for a stored value.
func settingsEscNotice(p *settingsPress, outcome escalationApplyOutcome) string {
	switch outcome {
	case escalationExampleAdded:
		return settingsNoticeOK(settingsText(p.tr, "settings_esc_example_added"))
	case escalationStale:
		return settingsNoticeWarn(settingsText(p.tr, "settings_esc_stale"))
	case escalationLastStepRefused:
		return settingsNoticeWarn(settingsText(p.tr, "settings_esc_refuse_last_step"))
	case escalationInvalid:
		return settingsNoticeWarn(settingsText(p.tr, "common_callback_invalid_request"))
	case escalationSaveFailed:
		return settingsNoticeWarn(settingsText(p.tr, "common_settings_save_failed"))
	}
	return ""
}

// settingsEscFromResult redraws the page from an apply result: the ladder it read back (or the one before a
// refusal), else a fresh read.
func settingsEscFromResult(p *settingsPress, result escalationApplyResult) *settingsScreen {
	return settingsEscPageScreen(p, result.Ladder, settingsEscNotice(p, result.Outcome))
}

func settingsEscContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), settingsReadTimeout)
}

// settingsEscRunToggle serves je: the absolute target v, 0 or 1.
func settingsEscRunToggle(p *settingsPress) *settingsScreen {
	ctx, cancel := settingsEscContext()
	defer cancel()
	return settingsEscFromResult(p, applyEscalationEnabled(ctx, p.group.Id, p.fields["v"] == "1", p.query.From.Id))
}

// settingsEscRunForget serves jf: the forget period in days, a preset.
func settingsEscRunForget(p *settingsPress) *settingsScreen {
	days, ok := settingsEscForgetOf(p.fields["v"])
	if !ok {
		p.answer(settingsText(p.tr, "common_callback_invalid_request"), true)
		return nil
	}
	ctx, cancel := settingsEscContext()
	defer cancel()
	return settingsEscFromResult(p, applyEscalationForget(ctx, p.group.Id, days, p.query.From.Id))
}

// settingsEscStepScreen reads the ladder and draws step n's screen; a step beyond the next free index is
// stale (another admin changed the ladder) and redraws the page with the stale notice.
func settingsEscStepScreen(p *settingsPress, n int) (*escalation.Ladder, *settingsScreen, bool) {
	ladder, err := settingsEscReadNow(p.group.Id)
	if err != nil || ladder == nil {
		log.Warnf("[Settings] the escalation ladder of chat %d could not be read: %v", p.group.Id, err)
		p.cannotOpen()
		return nil, nil, false
	}
	if n > len(ladder.Steps)+1 {
		return ladder, settingsEscPageScreen(p, ladder, settingsEscNotice(p, escalationStale)), false
	}
	return ladder, nil, true
}

// settingsEscRunStep serves js: step n's screen (the next free index adds a step).
func settingsEscRunStep(p *settingsPress) *settingsScreen {
	n, _ := settingsEscStepOf(p.fields["t"])
	ladder, stale, ok := settingsEscStepScreen(p, n)
	if !ok {
		return stale
	}
	screen := renderSettingsEscStep(p.view(), ladder, n, "")
	return &screen
}

// settingsEscRunSet serves ja: an untimed step n.
func settingsEscRunSet(p *settingsPress) *settingsScreen {
	n, _ := settingsEscStepOf(p.fields["t"])
	ctx, cancel := settingsEscContext()
	defer cancel()
	return settingsEscFromResult(p, applyEscalationStep(ctx, p.group.Id, n, escalation.Step{Action: p.fields["v"]}, p.query.From.Id))
}

// settingsEscRunDuration serves jd: the shared duration screen of step n's timed mute or ban, the stored preset
// marked only while that timed action is the step's; Back returns to the step screen.
func settingsEscRunDuration(p *settingsPress) *settingsScreen {
	n, _ := settingsEscStepOf(p.fields["t"])
	k := p.fields["k"]
	mode, _ := settingsWarnsTimedMode(k)
	ladder, stale, ok := settingsEscStepScreen(p, n)
	if !ok {
		return stale
	}
	selected := 0
	if n <= len(ladder.Steps) {
		if s := ladder.Steps[n-1]; s.Action == mode && autoActionEffective(s.Action, s.Secs) {
			selected = s.Secs
		}
	}
	press := func(secs int) map[string]string {
		return map[string]string{"a": settingsActEscTimed, "t": strconv.Itoa(n), "k": k, "v": strconv.Itoa(secs)}
	}
	back := map[string]string{"a": settingsActEscStep, "t": strconv.Itoa(n)}
	screen := renderSettingsDuration(p.view(), "settings_page_esc", mode, selected, press, back, "")
	return &screen
}

// settingsEscRunTimed serves jt: a timed step n with a preset duration.
func settingsEscRunTimed(p *settingsPress) *settingsScreen {
	n, _ := settingsEscStepOf(p.fields["t"])
	mode, _ := settingsWarnsTimedMode(p.fields["k"])
	secs, _ := settingsDurationPreset(p.fields["v"])
	ctx, cancel := settingsEscContext()
	defer cancel()
	return settingsEscFromResult(p, applyEscalationStep(ctx, p.group.Id, n, escalation.Step{Action: mode, Secs: secs}, p.query.From.Id))
}

// settingsEscRunRemove serves jr: Remove last, with t the index the screen showed as last.
func settingsEscRunRemove(p *settingsPress) *settingsScreen {
	n, _ := settingsEscStepOf(p.fields["t"])
	ctx, cancel := settingsEscContext()
	defer cancel()
	return settingsEscFromResult(p, applyEscalationRemoveStep(ctx, p.group.Id, n, p.query.From.Id))
}
