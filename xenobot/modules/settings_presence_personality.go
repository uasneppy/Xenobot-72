package modules

import (
	"context"
	"errors"
	"html"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/db/presence"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
)

// The Personality sub-page (page pr, sub-page pf, Phase 36) is where a group's admins set how often the bot
// reacts to each kind of message (text, stickers, photos), the topics AI reactions are limited to, and the
// extra names the bot answers to. It is an action of the Presence page (private only, hidden in a Staff
// Group like the page itself), so it adds no page, Main cell or row to Presence beyond one shared cell.
// Frequencies are three cycle buttons that send the absolute target; the two lists are typed replies run by
// the typed-reply door (settingsTypedParts), and every write goes through an applyPresence* function.

const (
	// settingsPresenceControlReactText, ...Sticker and ...Photo are the frequency controls (st only, v 0..3).
	settingsPresenceControlReactText    = "qt"
	settingsPresenceControlReactSticker = "qs"
	settingsPresenceControlReactPhoto   = "qp"

	// settingsActPresencePersonality opens the Personality sub-page on pr (private only).
	settingsActPresencePersonality = "pf"
	// settingsActPresencePersonalityEdit starts the typed list of the part t (private only).
	settingsActPresencePersonalityEdit = "pw"
	// settingsActPresencePersonalityClear clears the list of the part t (private only).
	settingsActPresencePersonalityClear = "px"
	// settingsActPresencePersonalityCancel ends the wait of the part t (private only).
	settingsActPresencePersonalityCancel = "pv"

	// settingsPresencePartTopics and settingsPresencePartNames are the typed parts of page pr.
	settingsPresencePartTopics = "k"
	settingsPresencePartNames  = "n"
)

// settingsPresenceFreqOrder is the order the cycle buttons step through: never, then more and more often.
var settingsPresenceFreqOrder = []int{
	presence.ReactFreqOff, presence.ReactFreqRare, presence.ReactFreqNormal, presence.ReactFreqOften,
}

var settingsPresenceFreqKeys = map[int]string{
	presence.ReactFreqOff:    "settings_pf_freq_off",
	presence.ReactFreqRare:   "settings_pf_freq_rare",
	presence.ReactFreqNormal: "settings_pf_freq_normal",
	presence.ReactFreqOften:  "settings_pf_freq_often",
}

// settingsPresenceNextFreq is the frequency the cycle button of a kind sends: the one after the stored.
func settingsPresenceNextFreq(current int) int {
	for i, f := range settingsPresenceFreqOrder {
		if f == current {
			return settingsPresenceFreqOrder[(i+1)%len(settingsPresenceFreqOrder)]
		}
	}
	return presence.ReactFreqOff
}

// settingsPresenceValidFreq accepts exactly one digit, 0 to 3.
func settingsPresenceValidFreq(v string) bool {
	return len(v) == 1 && v[0] >= '0' && v[0] <= '3'
}

// settingsPresenceValidPart accepts exactly one of the two typed parts.
func settingsPresenceValidPart(t string) bool {
	return t == settingsPresencePartTopics || t == settingsPresencePartNames
}

func settingsPresencePartFields(fields map[string]string) bool {
	return fields["c"] != "" && settingsPresenceValidPart(fields["t"])
}

// settingsPresencePersonalityActions are the page actions the Presence page registers for the sub-page.
func settingsPresencePersonalityActions() map[string]settingsPageAction {
	return map[string]settingsPageAction{
		settingsActPresencePersonality: {
			Right: settingsRightAdmin, Validate: func(f map[string]string) bool { return f["c"] != "" },
			Run: settingsPresenceRunPersonality,
		},
		settingsActPresencePersonalityEdit: {
			Fields: []string{"t"}, Right: settingsRightChangeInfo, Validate: settingsPresencePartFields,
			Run: settingsPresenceRunPersonalityEdit,
		},
		settingsActPresencePersonalityClear: {
			Fields: []string{"t"}, Right: settingsRightChangeInfo, Validate: settingsPresencePartFields,
			Run: settingsPresenceRunPersonalityClear,
		},
		settingsActPresencePersonalityCancel: {
			Fields: []string{"t"}, Right: settingsRightAdmin, Validate: settingsPresencePartFields,
			Run: settingsPresenceRunPersonalityCancel,
		},
	}
}

// settingsPresencePersonalityControls are the three frequency controls.
func settingsPresencePersonalityControls() map[string]settingsControl {
	control := func(kind string) settingsControl {
		return settingsControl{
			Right: settingsRightChangeInfo, Set: settingsPresenceSetReactFreq(kind),
			Actions: []string{settingsActSet}, Validate: settingsPresenceValidFreq,
		}
	}
	return map[string]settingsControl{
		settingsPresenceControlReactText:    control(presence.KindText),
		settingsPresenceControlReactSticker: control(presence.KindSticker),
		settingsPresenceControlReactPhoto:   control(presence.KindPhoto),
	}
}

func init() {
	typed := func(handler func(*settingsInputReply)) settingsTypedPage {
		return settingsTypedPage{
			CancelCode: settingsActPresencePersonalityCancel, Right: settingsRightChangeInfo,
			Typed: handler, WaitEndedKey: "settings_pf_wait_ended",
		}
	}
	topics := typed(settingsPresenceTypedTopics)
	topics.Part = settingsPresencePartTopics
	names := typed(settingsPresenceTypedNames)
	names.Part = settingsPresencePartNames
	settingsTypedParts[settingsTypedPartKey(settingsPagePresence, settingsPresencePartTopics)] = topics
	settingsTypedParts[settingsTypedPartKey(settingsPagePresence, settingsPresencePartNames)] = names
}

// settingsPresenceFreqWord is the translated word of a stored frequency.
func settingsPresenceFreqWord(tr *i18n.Translator, freq int) string {
	return settingsText(tr, settingsPresenceFreqKeys[freq])
}

// settingsPresenceListLine is the sub-page's line of a stored list: its entries escaped and joined, or the
// "none set" word.
func settingsPresenceListLine(tr *i18n.Translator, key, paramName string, list []string) string {
	shown := settingsText(tr, "settings_pf_none")
	if len(list) > 0 {
		escaped := make([]string, len(list))
		for i, e := range list {
			escaped[i] = html.EscapeString(e)
		}
		shown = strings.Join(escaped, ", ")
	}
	return settingsText(tr, key, i18n.TranslationParams{paramName: shown})
}

// renderSettingsPresencePersonality draws the sub-page purely from the stored settings.
func renderSettingsPresencePersonality(v settingsView, stored presence.Settings, convUnlocked bool, notice string) settingsScreen {
	textFreq := stored.ReactFreqOf(presence.KindText)
	stickerFreq := stored.ReactFreqOf(presence.KindSticker)
	photoFreq := stored.ReactFreqOf(presence.KindPhoto)
	body := []string{
		settingsText(v.Tr, "settings_pf_intro"),
		"",
		settingsText(v.Tr, "settings_pf_text_line", i18n.TranslationParams{"freq": settingsPresenceFreqWord(v.Tr, textFreq)}),
		settingsText(v.Tr, "settings_pf_sticker_line", i18n.TranslationParams{"freq": settingsPresenceFreqWord(v.Tr, stickerFreq)}),
		settingsText(v.Tr, "settings_pf_photo_line", i18n.TranslationParams{"freq": settingsPresenceFreqWord(v.Tr, photoFreq)}),
		settingsPresenceListLine(v.Tr, "settings_pf_topics_line", "topics", stored.TopicList()),
		settingsPresenceListLine(v.Tr, "settings_pf_names_line", "names", stored.BotNameList()),
	}
	// The frequencies and the topics act through AI reactions (which need the owner's Conversations switch)
	// and the cat stickers: with neither on, say so, so "Often" is never set and silently ignored.
	aiActive := stored.AIReactMode != presence.AIReactModeOff && stored.AIReactMode != "" && convUnlocked
	if !aiActive && !stored.CatStickerEnabled {
		body = append(body, "", settingsText(v.Tr, "settings_pf_inactive_note"))
	}
	body = append(body, "", settingsText(v.Tr, "settings_pf_hint"))

	k := newSettingsKeyboard(v)
	cycle := func(control, labelKey string, current int) {
		label := settingsText(v.Tr, labelKey, i18n.TranslationParams{"freq": settingsPresenceFreqWord(v.Tr, current)})
		if button, ok := k.button(label, map[string]string{
			"a": settingsActSet, "p": settingsPagePresence, "t": control,
			"v": strconv.Itoa(settingsPresenceNextFreq(current)),
		}); ok {
			k.row(button)
		}
	}
	cycle(settingsPresenceControlReactText, "settings_btn_pf_text", textFreq)
	cycle(settingsPresenceControlReactSticker, "settings_btn_pf_sticker", stickerFreq)
	cycle(settingsPresenceControlReactPhoto, "settings_btn_pf_photo", photoFreq)
	listRow := func(part, setKey string) {
		var row []gotgbot.InlineKeyboardButton
		if set, ok := k.button(settingsText(v.Tr, setKey), map[string]string{
			"a": settingsActPresencePersonalityEdit, "t": part,
		}); ok {
			row = append(row, set)
		}
		if clear, ok := k.button(settingsText(v.Tr, "settings_btn_pf_clear"), map[string]string{
			"a": settingsActPresencePersonalityClear, "t": part,
		}); ok {
			row = append(row, clear)
		}
		k.row(row...)
	}
	listRow(settingsPresencePartTopics, "settings_btn_pf_topics")
	listRow(settingsPresencePartNames, "settings_btn_pf_names")

	var nav []gotgbot.InlineKeyboardButton
	if back, ok := k.button(settingsText(v.Tr, "settings_btn_back"), map[string]string{
		"a": settingsActOpen, "p": settingsPagePresence,
	}); ok {
		nav = append(nav, back)
	}
	if closeBtn, ok := k.button(settingsText(v.Tr, "settings_btn_close"), map[string]string{"a": settingsActClose}); ok {
		nav = append(nav, closeBtn)
	}
	k.row(nav...)
	return k.screen(settingsCompose(v, "settings_pf_title", body, notice, false))
}

// settingsPresencePrompt is the wait prompt of a typed list: the part's instruction with its limits, the
// wait's end time and one Cancel button.
func renderSettingsPresencePersonalityPrompt(v settingsView, part string, until time.Time, replaced bool) settingsScreen {
	key, limit, runes := "settings_pf_wait_topics", presence.MaxReactTopics, presence.MaxTopicRunes
	if part == settingsPresencePartNames {
		key, limit, runes = "settings_pf_wait_names", presence.MaxBotNames, presence.MaxBotNameRunes
	}
	body := []string{
		settingsText(v.Tr, key, i18n.TranslationParams{"max": limit, "runes": runes}),
		"",
		settingsText(v.Tr, "settings_edit_wait_footer", i18n.TranslationParams{"time": formatChatTime(until, chatClockOf(v.GroupID), "15:04")}),
	}
	if replaced {
		body = append(body, "", settingsText(v.Tr, "settings_edit_replaced_note"))
	}
	k := newSettingsKeyboard(v)
	if cancel, ok := k.button(settingsText(v.Tr, "settings_btn_edit_cancel"), map[string]string{
		"a": settingsActPresencePersonalityCancel, "t": part,
	}); ok {
		k.row(cancel)
	}
	return k.screen(settingsCompose(v, "settings_pf_title", body, "", false))
}

func settingsPresencePersonalityRedraw(p *settingsPress, notice string) *settingsScreen {
	stored, err := settingsPresenceReadSettings(p.group.Id)
	if err != nil {
		log.Warnf("[Settings] the presence setting of chat %d could not be read: %v", p.group.Id, err)
		p.cannotOpen()
		return nil
	}
	screen := renderSettingsPresencePersonality(p.view(), stored, settingsPresenceConvUnlockedForView(p.group.Id), notice)
	return &screen
}

func settingsPresencePersonalityFromResult(p *settingsPress, result presenceApplyResult, okNotice string) *settingsScreen {
	if result.Outcome != presenceApplied {
		return settingsPresencePersonalityRedraw(p, settingsNoticeWarn(settingsText(p.tr, "common_settings_save_failed")))
	}
	if result.Settings == nil {
		return settingsPresencePersonalityRedraw(p, okNotice)
	}
	screen := renderSettingsPresencePersonality(p.view(), *result.Settings, settingsPresenceConvUnlockedForView(p.group.Id), okNotice)
	return &screen
}

// settingsPresenceRunPersonality opens the sub-page (a private menu, outside a Staff Group).
func settingsPresenceRunPersonality(p *settingsPress) *settingsScreen {
	if settingsPresenceGroupRefused(p) || settingsPresenceStaffGate(p) {
		return nil
	}
	return settingsPresencePersonalityRedraw(p, "")
}

// settingsPresenceSetReactFreq is the Set of a frequency control: a press of the stored value writes
// nothing and shows no notice.
func settingsPresenceSetReactFreq(kind string) func(*settingsPress, string) *settingsScreen {
	return func(p *settingsPress, value string) *settingsScreen {
		if settingsPresenceGroupRefused(p) || settingsPresenceStaffGate(p) {
			return nil
		}
		freq, err := strconv.Atoi(value)
		if err != nil {
			p.answer(settingsText(p.tr, "common_callback_invalid_request"), true)
			return nil
		}
		stored, err := settingsPresenceReadSettings(p.group.Id)
		if err != nil {
			log.Warnf("[Settings] the presence setting of chat %d could not be read: %v", p.group.Id, err)
			p.cannotOpen()
			return nil
		}
		if stored.ReactFreqOf(kind) == freq {
			screen := renderSettingsPresencePersonality(p.view(), stored, settingsPresenceConvUnlockedForView(p.group.Id), "")
			return &screen
		}
		ctx, cancel := context.WithTimeout(context.Background(), settingsReadTimeout)
		defer cancel()
		return settingsPresencePersonalityFromResult(p, applyPresenceReactFreq(ctx, p.group.Id, kind, freq, p.query.From.Id), "")
	}
}

// settingsPresenceRunPersonalityEdit writes the user's one wait for a typed list and edits the menu
// message into the prompt. Without Redis, or when the wait cannot be written, it answers
// settings_edit_unavailable and edits nothing.
func settingsPresenceRunPersonalityEdit(p *settingsPress) *settingsScreen {
	if settingsPresenceGroupRefused(p) || settingsPresenceStaffGate(p) {
		return nil
	}
	part := p.fields["t"]
	ctx, cancel := context.WithTimeout(context.Background(), settingsReadTimeout)
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
		Page:        settingsPagePresence,
		Part:        part,
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
	screen := renderSettingsPresencePersonalityPrompt(p.view(), part, until, settingsEditWaitReplaced(previous, p.group.Id, settingsPagePresence, part))
	return &screen
}

// settingsPresenceRunPersonalityCancel serves the prompt's Cancel and redraws the sub-page.
func settingsPresenceRunPersonalityCancel(p *settingsPress) *settingsScreen {
	ctx, cancel := context.WithTimeout(context.Background(), settingsReadTimeout)
	defer cancel()
	outcome, err := settingsInputCancel(ctx, p.query.From.Id, p.group.Id, settingsPagePresence, p.fields["t"], p.query.Message.GetMessageId())
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
	return settingsPresencePersonalityRedraw(p, notice)
}

// settingsPresenceRunPersonalityClear clears the list of a part. An empty list writes nothing.
func settingsPresenceRunPersonalityClear(p *settingsPress) *settingsScreen {
	if settingsPresenceGroupRefused(p) || settingsPresenceStaffGate(p) {
		return nil
	}
	part := p.fields["t"]
	stored, err := settingsPresenceReadSettings(p.group.Id)
	if err != nil {
		log.Warnf("[Settings] the presence setting of chat %d could not be read: %v", p.group.Id, err)
		p.cannotOpen()
		return nil
	}
	empty := len(stored.TopicList()) == 0
	if part == settingsPresencePartNames {
		empty = len(stored.BotNameList()) == 0
	}
	if empty {
		screen := renderSettingsPresencePersonality(p.view(), stored, settingsPresenceConvUnlockedForView(p.group.Id), "")
		return &screen
	}
	ctx, cancel := context.WithTimeout(context.Background(), settingsReadTimeout)
	defer cancel()
	by := p.query.From.Id
	if part == settingsPresencePartNames {
		return settingsPresencePersonalityFromResult(p, applyPresenceBotNames(ctx, p.group.Id, "", by),
			settingsNoticeOK(settingsText(p.tr, "settings_pf_cleared_names")))
	}
	return settingsPresencePersonalityFromResult(p, applyPresenceReactTopics(ctx, p.group.Id, "", by),
		settingsNoticeOK(settingsText(p.tr, "settings_pf_cleared_topics")))
}

// settingsPresenceRefusalKey is the refusal text of a list the repository refused.
func settingsPresenceRefusalKey(part string, err error) string {
	prefix := "settings_pf_refuse_topics_"
	if part == settingsPresencePartNames {
		prefix = "settings_pf_refuse_names_"
	}
	switch {
	case errors.Is(err, presence.ErrListTooMany):
		return prefix + "many"
	case errors.Is(err, presence.ErrListTooLong):
		return prefix + "long"
	}
	return prefix + "invalid"
}

func settingsPresenceTypedTopics(r *settingsInputReply) {
	settingsPresenceTypedList(r, settingsPresencePartTopics)
}

func settingsPresenceTypedNames(r *settingsInputReply) {
	settingsPresenceTypedList(r, settingsPresencePartNames)
}

// settingsPresenceTypedList is the door's handler of a typed list: text only, the list checked by the
// repository's own normaliser, a Staff Group refused (the wait ends), then claimed and stored through the
// apply function. A refusal keeps the wait; a failed save releases the claim and keeps it; a saved list
// finishes the wait, sends the sub-page as a new message and closes the prompt.
func settingsPresenceTypedList(r *settingsInputReply, part string) {
	text := strings.TrimSpace(r.msg.GetText())
	if text == "" {
		r.refuse(r.text("settings_edit_refuse_need_text"))
		return
	}
	limits := i18n.TranslationParams{
		"max":   presence.MaxReactTopics,
		"runes": presence.MaxTopicRunes,
	}
	var list []string
	var err error
	if part == settingsPresencePartNames {
		limits = i18n.TranslationParams{"max": presence.MaxBotNames, "runes": presence.MaxBotNameRunes}
		list, err = presence.NormalizeBotNames(text)
	} else {
		list, err = presence.NormalizeTopics(text)
	}
	if err != nil {
		r.refuse(r.text(settingsPresenceRefusalKey(part, err), limits))
		return
	}
	if len(list) == 0 {
		r.refuse(r.text("settings_pf_refuse_blank"))
		return
	}
	if settingsPresenceStaffCheck(r.wait.GroupID) != settingsPresenceFree {
		r.end("common_callback_invalid_request")
		return
	}
	claim, claimed := r.claimWait()
	if !claimed {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), settingsReadTimeout)
	defer cancel()
	var result presenceApplyResult
	savedKey := "settings_pf_saved_topics"
	if part == settingsPresencePartNames {
		result = applyPresenceBotNames(ctx, r.wait.GroupID, text, r.user.Id)
		savedKey = "settings_pf_saved_names"
	} else {
		result = applyPresenceReactTopics(ctx, r.wait.GroupID, text, r.user.Id)
	}
	if result.Outcome != presenceApplied {
		r.releaseWait(claim)
		r.refuse(r.text("common_settings_save_failed"))
		return
	}
	r.finishWait(claim)
	log.Infof("[Settings] user %d typed the %s list for chat %d", r.user.Id, part, r.wait.GroupID)
	stored := presence.Settings{}
	if result.Settings != nil {
		stored = *result.Settings
	} else if read, readErr := settingsPresenceReadSettings(r.wait.GroupID); readErr == nil {
		stored = read
	}
	screen := renderSettingsPresencePersonality(r.view(), stored, settingsPresenceConvUnlockedForView(r.wait.GroupID), settingsNoticeOK(settingsText(r.tr, savedKey)))
	r.send(screen.Text, &screen.Keyboard, false)
	r.closePrompt()
}
