package modules

import (
	"context"
	"errors"
	"html"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/fuku_robot/fuku/db/modtuning"
	"github.com/uasneppy/fuku_robot/fuku/db/staff"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
)

// The Manual defaults page (page q, Phase 27, TUNE-04, TUNE-06): lets the owner or an allowed admin with
// can_restrict_members configure default /ban and /mute durations (Permanent/Off or 8 presets), the
// reason-required switch, and the ban appeal link (typed reply in private chat + Remove). It exists only
// in the bot's private chat (PrivateOnly: a group menu shows the note and no DB read runs) and is hidden
// in a Staff Group. Every write routes through applyDefaultBanSecs, applyDefaultMuteSecs, applyReasonRequired,
// or applyAppealURL with live settingsRightRestrict; viewing, duration screens, and Cancel need settingsRightAdmin.

const (
	// settingsPageModDefaults is the p value of the Manual defaults page.
	settingsPageModDefaults = "q"

	// Action codes of the Manual defaults page.
	settingsActModDefDur          = "md"
	settingsActModDefSetDur       = "mt"
	settingsActModDefReason       = "mr"
	settingsActModDefAppealPrompt = "ma"
	settingsActModDefAppealCancel = "mk"
	settingsActModDefAppealRemove = "mx"

	// Duration picker target kinds (k field).
	settingsModDefKindBan  = "b"
	settingsModDefKindMute = "m"

	// Typed wait part code (t field) for the appeal link.
	settingsModDefAppealPart = "a"

	// Maximum number of runes shown for a stored appeal URL on the page screen.
	settingsModDefAppealDisplayMaxRunes = 64
)

// settingsModDefaultsRead reads a chat's manual defaults fresh; a variable only so a test can fail or count it.
var settingsModDefaultsRead = modtuning.GetDefaultsFresh

// settingsModDefaultsStaffLookup reads a chat's Staff Group row fresh for Hidden; a variable only so a test can fail it.
var settingsModDefaultsStaffLookup = staff.GetStaffGroupFresh

func init() {
	// The owner field c exists only on private-menu buttons, so every Validate requires it: a group-menu
	// press fails in step 1 before any Telegram or DB I/O.
	private := func(check func(map[string]string) bool) func(map[string]string) bool {
		return func(fields map[string]string) bool { return fields["c"] != "" && check(fields) }
	}
	registerSettingsPage(settingsPage{
		Key:            settingsPageModDefaults,
		Order:          41,
		Icon:           "⚖️",
		TitleKey:       "settings_page_moddefaults",
		MainState:      settingsModDefaultsMainState,
		Open:           settingsModDefaultsOpen,
		Hidden:         settingsModDefaultsHidden,
		PrivateOnly:    true,
		PrivateNoteKey: "settings_moddef_group_note",
		Actions: map[string]settingsPageAction{
			settingsActModDefDur: {
				Fields: []string{"k"}, Right: settingsRightAdmin, Run: settingsModDefRunDur,
				Validate: private(func(f map[string]string) bool { return settingsModDefValidKind(f["k"]) }),
			},
			settingsActModDefSetDur: {
				Fields: []string{"k", "v"}, Right: settingsRightRestrict, Run: settingsModDefRunSetDur,
				Validate: private(func(f map[string]string) bool {
					_, ok := settingsModDefDurationValue(f["v"])
					return settingsModDefValidKind(f["k"]) && ok
				}),
			},
			settingsActModDefReason: {
				Fields: []string{"v"}, Right: settingsRightRestrict, Run: settingsModDefRunReason,
				Validate: private(func(f map[string]string) bool { return settingsEditSwitchValue(f["v"]) }),
			},
			settingsActModDefAppealPrompt: {
				Right: settingsRightRestrict, Run: settingsModDefRunAppealPrompt,
				Validate: private(func(map[string]string) bool { return true }),
			},
			settingsActModDefAppealCancel: {
				Fields: []string{"t"}, Right: settingsRightAdmin, Run: settingsModDefRunAppealCancel,
				Validate: private(func(f map[string]string) bool { return f["t"] == settingsModDefAppealPart }),
			},
			settingsActModDefAppealRemove: {
				Right: settingsRightRestrict, Run: settingsModDefRunAppealRemove,
				Validate: private(func(map[string]string) bool { return true }),
			},
		},
	})
	settingsTypedPages[settingsPageModDefaults] = settingsTypedPage{
		Part:         settingsModDefAppealPart,
		CancelCode:   settingsActModDefAppealCancel,
		Right:        settingsRightRestrict,
		Typed:        settingsModDefAppealTyped,
		WaitEndedKey: "settings_moddef_appeal_wait_ended",
	}
}

func settingsModDefValidKind(k string) bool {
	return k == settingsModDefKindBan || k == settingsModDefKindMute
}

// settingsModDefDurationValue accepts canonical "0" (Permanent / no default) or one of the 8 settingsDurationPresets.
func settingsModDefDurationValue(v string) (int, bool) {
	if v == "0" {
		return 0, true
	}
	return settingsDurationPreset(v)
}

func settingsModDefContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), settingsReadTimeout)
}

func settingsModDefaultsReadNow(chatID int64) (modtuning.Defaults, error) {
	ctx, cancel := settingsModDefContext()
	defer cancel()
	return settingsModDefaultsRead(ctx, chatID)
}

// settingsModDefaultsHidden hides the page in Staff Groups (definite fresh row); an error shows the page.
func settingsModDefaultsHidden(group *gotgbot.Chat) bool {
	row, err := settingsModDefaultsStaffLookup(group.Id)
	return err == nil && row != nil
}

// settingsModDefaultsMainState shows Default when none of the 4 settings are active, or the count of active ones.
func settingsModDefaultsMainState(_ *gotgbot.Bot, tr *i18n.Translator, group *gotgbot.Chat) (string, bool) {
	defs, err := settingsModDefaultsReadNow(group.Id)
	if err != nil {
		return "", false
	}
	count := 0
	if defs.DefaultBanSecs > 0 {
		count++
	}
	if defs.DefaultMuteSecs > 0 {
		count++
	}
	if defs.ReasonRequired {
		count++
	}
	if defs.AppealURL != "" {
		count++
	}
	if count == 0 {
		return settingsText(tr, "settings_state_default"), true
	}
	return settingsText(tr, "settings_state_moddef_n", i18n.TranslationParams{"n": count}), true
}

// settingsModDefaultsOpen draws the Manual defaults page. A read error produces the cannot-open edit.
func settingsModDefaultsOpen(p *settingsPress, notice string) (settingsScreen, error) {
	defs, err := settingsModDefaultsReadNow(p.group.Id)
	if err != nil {
		return settingsScreen{}, err
	}
	return renderSettingsModDefaults(p.view(), defs, notice), nil
}

func settingsModDefaultsShow(p *settingsPress, notice string) *settingsScreen {
	screen, err := settingsModDefaultsOpen(p, notice)
	if err != nil {
		log.Warnf("[Settings] manual defaults of chat %d could not be read: %v", p.group.Id, err)
		p.cannotOpen()
		return nil
	}
	return &screen
}

func settingsModDefDurationLabel(tr *i18n.Translator, secs int) string {
	if secs > 0 {
		return punishDurationLabel(tr, secs)
	}
	return settingsText(tr, "settings_moddef_permanent")
}

func settingsModDefTruncateRunes(s string, maxRunes int) string {
	if maxRunes <= 0 || utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	count := 0
	for i := range s {
		if count == maxRunes-1 {
			return s[:i] + "…"
		}
		count++
	}
	return s
}

// renderSettingsModDefaults draws page q: intro, current state of all 4 settings, controls, Back and Close.
func renderSettingsModDefaults(v settingsView, defs modtuning.Defaults, notice string) settingsScreen {
	appealDisplay := settingsText(v.Tr, "settings_moddef_appeal_none")
	if defs.AppealURL != "" {
		appealDisplay = html.EscapeString(settingsModDefTruncateRunes(defs.AppealURL, settingsModDefAppealDisplayMaxRunes))
	}
	body := []string{
		settingsText(v.Tr, "settings_moddef_intro"),
		"",
		settingsText(v.Tr, "settings_moddef_line_ban", i18n.TranslationParams{"value": settingsModDefDurationLabel(v.Tr, defs.DefaultBanSecs)}),
		settingsText(v.Tr, "settings_moddef_line_mute", i18n.TranslationParams{"value": settingsModDefDurationLabel(v.Tr, defs.DefaultMuteSecs)}),
		settingsText(v.Tr, "settings_moddef_line_reason", i18n.TranslationParams{"state": settingsStateWord(v.Tr, defs.ReasonRequired)}),
		settingsText(v.Tr, "settings_moddef_line_appeal", i18n.TranslationParams{"value": appealDisplay}),
	}

	k := newSettingsKeyboard(v)
	var durRow []gotgbot.InlineKeyboardButton
	if btn, ok := k.button(settingsText(v.Tr, "settings_btn_moddef_ban_dur"),
		map[string]string{"a": settingsActModDefDur, "k": settingsModDefKindBan}); ok {
		durRow = append(durRow, btn)
	}
	if btn, ok := k.button(settingsText(v.Tr, "settings_btn_moddef_mute_dur"),
		map[string]string{"a": settingsActModDefDur, "k": settingsModDefKindMute}); ok {
		durRow = append(durRow, btn)
	}
	k.row(durRow...)

	reasonLabel := settingsText(v.Tr, "settings_btn_moddef_reason", i18n.TranslationParams{"state": settingsStateWord(v.Tr, defs.ReasonRequired)})
	if btn, ok := k.button(reasonLabel, map[string]string{"a": settingsActModDefReason, "v": settingsBoolValue(!defs.ReasonRequired)}); ok {
		k.row(btn)
	}

	var appealRow []gotgbot.InlineKeyboardButton
	if btn, ok := k.button(settingsText(v.Tr, "settings_btn_moddef_appeal_set"),
		map[string]string{"a": settingsActModDefAppealPrompt}); ok {
		appealRow = append(appealRow, btn)
	}
	if defs.AppealURL != "" {
		if btn, ok := k.button(settingsText(v.Tr, "settings_btn_moddef_appeal_remove"),
			map[string]string{"a": settingsActModDefAppealRemove}); ok {
			appealRow = append(appealRow, btn)
		}
	}
	k.row(appealRow...)

	k.navRow(false)
	return k.screen(settingsCompose(v, "settings_page_moddefaults", body, notice, false))
}

// renderSettingsModDefDuration draws the duration picker for /ban (k="b") or /mute (k="m"): intro, full-width
// Off/Permanent button, the 8 settingsDurationPresets two per row, and Back to page q + Close.
func renderSettingsModDefDuration(v settingsView, kind string, selected int, notice string) settingsScreen {
	introKey := "settings_moddef_dur_intro_mute"
	if kind == settingsModDefKindBan {
		introKey = "settings_moddef_dur_intro_ban"
	}
	body := []string{settingsText(v.Tr, introKey)}

	k := newSettingsKeyboard(v)
	offLabel := settingsText(v.Tr, "settings_btn_moddef_dur_off")
	if selected == 0 {
		offLabel = settingsSelectedPrefix + offLabel
	}
	if btn, ok := k.button(offLabel, map[string]string{"a": settingsActModDefSetDur, "k": kind, "v": "0"}); ok {
		k.row(btn)
	}

	for i := 0; i < len(settingsDurationPresets); i += 2 {
		var row []gotgbot.InlineKeyboardButton
		for _, secs := range settingsDurationPresets[i:min(i+2, len(settingsDurationPresets))] {
			label := punishDurationLabel(v.Tr, secs)
			if secs == selected {
				label = settingsSelectedPrefix + label
			}
			if btn, ok := k.button(label, map[string]string{"a": settingsActModDefSetDur, "k": kind, "v": strconv.Itoa(secs)}); ok {
				row = append(row, btn)
			}
		}
		k.row(row...)
	}

	var nav []gotgbot.InlineKeyboardButton
	if back, ok := k.button(settingsText(v.Tr, "settings_btn_back"), map[string]string{"a": settingsActOpen, "p": settingsPageModDefaults}); ok {
		nav = append(nav, back)
	}
	if closeBtn, ok := k.button(settingsText(v.Tr, "settings_btn_close"), map[string]string{"a": settingsActClose}); ok {
		nav = append(nav, closeBtn)
	}
	k.row(nav...)
	return k.screen(settingsCompose(v, "settings_page_moddefaults", body, notice, false))
}

// settingsModDefRunDur serves md: opens the default ban or mute duration screen.
func settingsModDefRunDur(p *settingsPress) *settingsScreen {
	defs, err := settingsModDefaultsReadNow(p.group.Id)
	if err != nil {
		log.Warnf("[Settings] manual defaults of chat %d could not be read: %v", p.group.Id, err)
		p.cannotOpen()
		return nil
	}
	kind := p.fields["k"]
	selected := defs.DefaultMuteSecs
	if kind == settingsModDefKindBan {
		selected = defs.DefaultBanSecs
	}
	screen := renderSettingsModDefDuration(p.view(), kind, selected, "")
	return &screen
}

// settingsModDefRunSetDur serves mt: stores a default ban or mute duration (0 or preset) and redraws page q.
func settingsModDefRunSetDur(p *settingsPress) *settingsScreen {
	kind := p.fields["k"]
	secs, ok := settingsModDefDurationValue(p.fields["v"])
	if !settingsModDefValidKind(kind) || !ok {
		p.answer(settingsText(p.tr, "common_callback_invalid_request"), true)
		return nil
	}
	ctx, cancel := settingsModDefContext()
	defer cancel()
	var outcome modTuningOutcome
	if kind == settingsModDefKindBan {
		outcome = applyDefaultBanSecs(ctx, p.group.Id, secs, p.query.From.Id)
	} else {
		outcome = applyDefaultMuteSecs(ctx, p.group.Id, secs, p.query.From.Id)
	}
	return settingsModDefaultsShow(p, settingsModDefNotice(p.tr, outcome, "settings_moddef_saved_dur"))
}

// settingsModDefRunReason serves mr: toggles reason_required and redraws page q.
func settingsModDefRunReason(p *settingsPress) *settingsScreen {
	ctx, cancel := settingsModDefContext()
	defer cancel()
	outcome := applyReasonRequired(ctx, p.group.Id, p.fields["v"] == "1", p.query.From.Id)
	return settingsModDefaultsShow(p, settingsModDefNotice(p.tr, outcome, "settings_moddef_saved_reason"))
}

// settingsModDefRunAppealRemove serves mx: clears appeal_url and redraws page q.
func settingsModDefRunAppealRemove(p *settingsPress) *settingsScreen {
	ctx, cancel := settingsModDefContext()
	defer cancel()
	outcome := applyAppealURL(ctx, p.group.Id, "", p.query.From.Id)
	return settingsModDefaultsShow(p, settingsModDefNotice(p.tr, outcome, "settings_moddef_removed_appeal"))
}

func settingsModDefNotice(tr *i18n.Translator, outcome modTuningOutcome, savedKey string) string {
	switch outcome {
	case modTuningSaved:
		return settingsNoticeOK(settingsText(tr, savedKey))
	case modTuningInvalid:
		return settingsNoticeWarn(settingsText(tr, "common_callback_invalid_request"))
	default:
		return settingsNoticeWarn(settingsText(tr, "common_settings_save_failed"))
	}
}

// renderSettingsModDefAppealPrompt draws the typed-reply prompt for setting the appeal link.
func renderSettingsModDefAppealPrompt(v settingsView, until time.Time, replaced bool) settingsScreen {
	return renderSettingsTypedPrompt(v, "settings_page_moddefaults", "settings_moddef_appeal_wait", "settings_moddef_appeal_wait_rules",
		settingsActModDefAppealCancel, settingsModDefAppealPart, until, replaced)
}

// settingsModDefRunAppealPrompt serves ma: opens a typed-reply wait for the appeal link.
func settingsModDefRunAppealPrompt(p *settingsPress) *settingsScreen {
	ctx, cancel := settingsModDefContext()
	defer cancel()
	nonce, err := newSettingsInputNonce()
	if err != nil {
		log.Errorf("[Settings] no nonce for a wait: %v", err)
		p.answer(settingsText(p.tr, "settings_edit_unavailable"), true)
		return nil
	}
	until := time.Now().Add(settingsInputTTL)
	previous, err := settingsInputSet(ctx, p.query.From.Id, settingsInputWait{
		GroupID:     p.group.Id,
		Page:        settingsPageModDefaults,
		Part:        settingsModDefAppealPart,
		PromptMsgID: p.query.Message.GetMessageId(),
		Nonce:       nonce,
		ExpiresAt:   until,
	})
	if err != nil {
		if !errors.Is(err, errSettingsInputUnavailable) {
			log.Warnf("[Settings] the wait of user %d could not be written: %v", p.query.From.Id, err)
		}
		p.answer(settingsText(p.tr, "settings_edit_unavailable"), true)
		return nil
	}
	screen := renderSettingsModDefAppealPrompt(p.view(), until, settingsEditWaitReplaced(previous, p.group.Id, settingsPageModDefaults, settingsModDefAppealPart))
	return &screen
}

// settingsModDefRunAppealCancel serves mc: cancels the active appeal-link wait and redraws page q.
func settingsModDefRunAppealCancel(p *settingsPress) *settingsScreen {
	ctx, cancel := settingsModDefContext()
	defer cancel()
	outcome, err := settingsInputCancel(ctx, p.query.From.Id, p.group.Id, settingsPageModDefaults, p.fields["t"], p.query.Message.GetMessageId())
	notice := ""
	switch {
	case errors.Is(err, errSettingsInputUnavailable):
		p.answer(settingsText(p.tr, "settings_edit_unavailable"), true)
	case err != nil:
		log.Warnf("[Settings] the wait of user %d could not be cancelled: %v", p.query.From.Id, err)
		p.answer(settingsText(p.tr, "settings_edit_unavailable"), true)
		return nil
	case outcome == settingsInputCancelDeleted:
		notice = settingsNoticeOK(settingsText(p.tr, "settings_edit_cancelled"))
	case outcome == settingsInputCancelExpired:
		p.answer(settingsText(p.tr, "settings_edit_wait_expired"), true)
	default:
		p.answer(settingsText(p.tr, "settings_edit_prompt_stale"), true)
	}
	return settingsModDefaultsShow(p, notice)
}

// settingsModDefAppealTyped is the door's handler for a typed appeal URL: requires plain text, validates
// through validateAppealURL (refusing with wait kept on invalid input), claims the wait, writes through
// applyAppealURL, logs IDs only, sends the refreshed page q, and closes the prompt.
func settingsModDefAppealTyped(r *settingsInputReply) {
	if r.msg.Text == "" {
		r.refuse(r.text("settings_edit_refuse_need_text"))
		return
	}
	raw := strings.TrimSpace(r.msg.Text)
	cleaned, ok := validateAppealURL(raw)
	if raw == "" || !ok {
		r.refuse(r.text("settings_moddef_appeal_refuse_url"))
		return
	}
	claim, claimed := r.claimWait()
	if !claimed {
		return
	}
	ctx, cancel := settingsModDefContext()
	defer cancel()
	outcome := applyAppealURL(ctx, r.wait.GroupID, cleaned, r.user.Id)
	if outcome != modTuningSaved {
		r.releaseWait(claim)
		if outcome == modTuningInvalid {
			r.refuse(r.text("settings_moddef_appeal_refuse_url"))
		} else {
			r.refuse(r.text("common_settings_save_failed"))
		}
		return
	}
	r.finishWait(claim)
	log.Infof("[Settings] user %d updated appeal link for chat %d", r.user.Id, r.wait.GroupID)
	defs, err := settingsModDefaultsReadNow(r.wait.GroupID)
	if err != nil {
		defs = modtuning.Defaults{AppealURL: cleaned}
	}
	screen := renderSettingsModDefaults(r.view(), defs, settingsNoticeOK(settingsText(r.tr, "settings_moddef_saved_appeal")))
	r.send(screen.Text, &screen.Keyboard, false)
	r.closePrompt()
}
