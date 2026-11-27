package modules

import (
	"context"
	"errors"
	"strconv"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/db/chatconv"
	"github.com/uasneppy/Xenobot-72/xenobot/db/presence"
	"github.com/uasneppy/Xenobot-72/xenobot/db/staff"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
)

// The Presence page (page pr, Phase 31/32, PRES-03/PRES-04/REACT-01) configures admin-controlled
// Interactive Presence reactions (zero-AI Emoji Consensus and 👎 Retraction, plus Owner×Admin
// double-gated Contextual AI Emoji Reactions) strictly separately from Owner-only AI Conversations
// (page p). It is PrivateOnly (a group menu shows settings_pr_group_note with zero DB reads),
// hidden in Staff Groups with fail-closed defense-in-depth on every callback, and all controls
// require live settingsRightChangeInfo.

const (
	// settingsPagePresence is the p value of the Presence page.
	settingsPagePresence = "pr"
	// settingsPresenceControlConsensus is the Emoji Consensus toggle (tg only, v 0 or 1).
	settingsPresenceControlConsensus = "c"
	// settingsPresenceControlThreshold is the Emoji Consensus threshold stepper (st only, v 2..20).
	settingsPresenceControlThreshold = "ct"
	// settingsPresenceControlRetract is the 👎 Retraction toggle (tg only, v 0 or 1).
	settingsPresenceControlRetract = "r"
	// settingsPresenceControlAIReact is the Contextual AI Reactions mode selector (st only, v o/l/m).
	settingsPresenceControlAIReact = "ai"
)

// settingsPresenceAIReactCodes maps a segmented button's v code to the stored AI reaction mode.
var settingsPresenceAIReactCodes = map[string]string{
	"o": presence.AIReactModeOff,
	"l": presence.AIReactModeLow,
	"m": presence.AIReactModeMedium,
}

// settingsPresenceAIReactKeys maps each stored AI reaction mode to its short localized label key.
var settingsPresenceAIReactKeys = map[string]string{
	presence.AIReactModeOff:    "settings_btn_pr_ai_off",
	presence.AIReactModeLow:    "settings_btn_pr_ai_low",
	presence.AIReactModeMedium: "settings_btn_pr_ai_medium",
}

func settingsPresenceNormalizeAIReactMode(mode string) string {
	if presence.ValidAIReactMode(mode) {
		return mode
	}
	return presence.AIReactModeOff
}

func settingsPresenceAIReactKey(mode string) string {
	if key, ok := settingsPresenceAIReactKeys[mode]; ok {
		return key
	}
	return settingsPresenceAIReactKeys[presence.AIReactModeOff]
}

// Test seams for Staff Group checks and fresh Presence/Conversations reads.
var (
	settingsPresenceStaffLookup = staff.GetStaffGroupFresh
	settingsPresenceRead        = presence.GetFresh
	settingsPresenceChatRead    = chatconv.GetFresh
)

var errSettingsPresenceBlocked = errors.New("presence settings page is unavailable for this chat")

func init() {
	registerSettingsPage(settingsPage{
		Key:            settingsPagePresence,
		Order:          91,
		Icon:           "✨",
		TitleKey:       "settings_page_presence",
		OwnerOnly:      false,
		PrivateOnly:    true,
		PrivateNoteKey: "settings_pr_group_note",
		Hidden:         settingsPresenceHidden,
		MainState:      settingsPresenceMainState,
		Open:           settingsPresenceOpen,
		Controls: map[string]settingsControl{
			settingsPresenceControlConsensus: {
				Right: settingsRightChangeInfo, Set: settingsPresenceSetConsensus,
				Actions: []string{settingsActToggle}, Validate: settingsEditSwitchValue,
			},
			settingsPresenceControlThreshold: {
				Right: settingsRightChangeInfo, Set: settingsPresenceSetThreshold,
				Actions: []string{settingsActSet}, Validate: settingsPresenceValidThreshold,
			},
			settingsPresenceControlRetract: {
				Right: settingsRightChangeInfo, Set: settingsPresenceSetRetract,
				Actions: []string{settingsActToggle}, Validate: settingsEditSwitchValue,
			},
			settingsPresenceControlAIReact: {
				Right: settingsRightChangeInfo, Set: settingsPresenceSetAIReact,
				Actions:  []string{settingsActSet},
				Validate: func(v string) bool { _, ok := settingsPresenceAIReactCodes[v]; return ok },
			},
		},
	})
}

// settingsPresenceValidThreshold accepts only canonical base-10 integers in 2..20 (rejecting
// leading zeros, signs, and out-of-range values before any I/O).
func settingsPresenceValidThreshold(v string) bool {
	n, ok := settingsParseNumber(v, 2)
	return ok && strconv.Itoa(n) == v && presence.ValidConsensusThreshold(n)
}

func settingsPresenceClamp(n int) int {
	if n < presence.MinConsensusThreshold {
		return presence.MinConsensusThreshold
	}
	if n > presence.MaxConsensusThreshold {
		return presence.MaxConsensusThreshold
	}
	return n
}

func settingsPresenceReadSettings(chatID int64) (presence.Settings, error) {
	ctx, cancel := context.WithTimeout(context.Background(), settingsReadTimeout)
	defer cancel()
	return settingsPresenceRead(ctx, chatID)
}

func settingsPresenceReadChat(chatID int64) (chatconv.Settings, error) {
	ctx, cancel := context.WithTimeout(context.Background(), settingsReadTimeout)
	defer cancel()
	return settingsPresenceChatRead(ctx, chatID)
}

// settingsPresenceConvUnlocked reports whether Owner-controlled Conversations is enabled with a
// server-configured AI provider for chatID. When the server has no AI provider configured it
// returns false, nil with zero database reads.
func settingsPresenceConvUnlocked(chatID int64) (bool, error) {
	if !chatAnyProviderConfigured() {
		return false, nil
	}
	conv, err := settingsPresenceReadChat(chatID)
	if err != nil {
		return false, err
	}
	return conv.Enabled && chatProviderConfigured(conv.Provider), nil
}

// settingsPresenceConvUnlockedForView degrades a transient Conversations read error to false
// (showing the locked note) when rendering page pr; enabling AI reactions still fails closed.
func settingsPresenceConvUnlockedForView(chatID int64) bool {
	unlocked, err := settingsPresenceConvUnlocked(chatID)
	if err != nil {
		log.WithField("chat_id", chatID).Warn("[Settings] reading conversation state for presence page failed")
		return false
	}
	return unlocked
}

// settingsPresenceHidden is true only for a definite Staff Group row. An unreadable row hides
// nothing here; MainState, Open, and every control fail closed instead.
func settingsPresenceHidden(group *gotgbot.Chat) bool {
	row, err := settingsPresenceStaffLookup(group.Id)
	return err == nil && row != nil
}

type settingsPresenceGate int

const (
	settingsPresenceFree settingsPresenceGate = iota
	settingsPresenceStaff
	settingsPresenceUnreadable
)

func settingsPresenceStaffCheck(chatID int64) settingsPresenceGate {
	row, err := settingsPresenceStaffLookup(chatID)
	switch {
	case err != nil:
		log.Warnf("[Settings] the Staff Group row of chat %d could not be read for the presence page: %v", chatID, err)
		return settingsPresenceUnreadable
	case row != nil:
		return settingsPresenceStaff
	}
	return settingsPresenceFree
}

// settingsPresenceGroupRefused refuses a press from a group menu before any DB read or write.
func settingsPresenceGroupRefused(p *settingsPress) bool {
	if p.private {
		return false
	}
	p.answer(settingsText(p.tr, "common_callback_invalid_request"), true)
	return true
}

// settingsPresenceStaffGate enforces fail-closed Staff Group refusal before any read or write.
// A Staff Group receives the generic invalid-request alert (never naming Staff Group); a lookup
// failure edits to cannot-open. Both return blocked=true with nothing written.
func settingsPresenceStaffGate(p *settingsPress) bool {
	switch settingsPresenceStaffCheck(p.group.Id) {
	case settingsPresenceStaff:
		p.answer(settingsText(p.tr, "common_callback_invalid_request"), true)
		return true
	case settingsPresenceUnreadable:
		p.cannotOpen()
		return true
	case settingsPresenceFree:
	}
	return false
}

// settingsPresenceMainState reports the Main status block summary: Off when all active Presence
// features are off, or "{n} on" when 1..3 features are active (counting Contextual AI Reactions
// only when both chat_presence.AIReactMode != "off" and Owner Conversations is enabled/configured).
func settingsPresenceMainState(_ *gotgbot.Bot, tr *i18n.Translator, group *gotgbot.Chat) (string, bool) {
	if settingsPresenceStaffCheck(group.Id) != settingsPresenceFree {
		return "", false
	}
	stored, err := settingsPresenceReadSettings(group.Id)
	if err != nil {
		return "", false
	}
	active := 0
	if stored.ConsensusEnabled {
		active++
	}
	if stored.RetractEnabled {
		active++
	}
	if settingsPresenceNormalizeAIReactMode(stored.AIReactMode) != presence.AIReactModeOff {
		if unlocked, err := settingsPresenceConvUnlocked(group.Id); err == nil && unlocked {
			active++
		}
	}
	if active == 0 {
		return settingsText(tr, "settings_state_off_plain"), true
	}
	return settingsText(tr, "settings_state_presence_on", i18n.TranslationParams{"n": active}), true
}

// settingsPresenceOpen loads and renders the Presence page in private chats for non-Staff groups.
func settingsPresenceOpen(p *settingsPress, notice string) (settingsScreen, error) {
	if !p.private || settingsPresenceStaffCheck(p.group.Id) != settingsPresenceFree {
		return settingsScreen{}, errSettingsPresenceBlocked
	}
	stored, err := settingsPresenceReadSettings(p.group.Id)
	if err != nil {
		return settingsScreen{}, err
	}
	return renderSettingsPresence(p.view(), stored, settingsPresenceConvUnlockedForView(p.group.Id), notice), nil
}

// settingsPresenceAIReactRow renders the 3-button segmented [Off | Low | Medium] selector row,
// prefixing settingsSelectedPrefix onto the currently stored mode.
func settingsPresenceAIReactRow(k *settingsKeyboard, stored string) []gotgbot.InlineKeyboardButton {
	var row []gotgbot.InlineKeyboardButton
	for _, code := range []string{"o", "l", "m"} {
		mode := settingsPresenceAIReactCodes[code]
		label := settingsText(k.view.Tr, settingsPresenceAIReactKeys[mode])
		if mode == stored {
			label = settingsSelectedPrefix + label
		}
		if button, ok := k.button(label, map[string]string{
			"a": settingsActSet, "p": settingsPagePresence, "t": settingsPresenceControlAIReact, "v": code,
		}); ok {
			row = append(row, button)
		}
	}
	return row
}

// renderSettingsPresence draws the Presence page purely from stored settings with zero DB reads.
func renderSettingsPresence(v settingsView, stored presence.Settings, convUnlocked bool, notice string) settingsScreen {
	consensusState := settingsStateWord(v.Tr, stored.ConsensusEnabled)
	retractState := settingsStateWord(v.Tr, stored.RetractEnabled)
	threshold := settingsPresenceClamp(stored.ConsensusThreshold)
	aiMode := settingsPresenceNormalizeAIReactMode(stored.AIReactMode)
	aiModeLabel := settingsText(v.Tr, settingsPresenceAIReactKey(aiMode))

	body := []string{
		settingsText(v.Tr, "settings_pr_intro"),
		"",
		settingsText(v.Tr, "settings_pr_consensus_line", i18n.TranslationParams{"state": consensusState}),
		settingsText(v.Tr, "settings_pr_threshold_line", i18n.TranslationParams{"count": threshold}),
		settingsText(v.Tr, "settings_pr_retract_line", i18n.TranslationParams{"state": retractState}),
		settingsText(v.Tr, "settings_pr_ai_line", i18n.TranslationParams{"mode": aiModeLabel}),
	}
	if !convUnlocked {
		body = append(body, settingsText(v.Tr, "settings_pr_ai_locked_note"))
	}
	body = append(body, "", settingsText(v.Tr, "settings_pr_hint"))

	k := newSettingsKeyboard(v)
	consensusLabel := settingsText(v.Tr, "settings_btn_pr_consensus", i18n.TranslationParams{"state": consensusState})
	k.row(settingsToggleButton(k, settingsPagePresence, settingsPresenceControlConsensus, consensusLabel, stored.ConsensusEnabled)...)
	k.row(settingsStepperRow(
		k,
		settingsPagePresence,
		settingsPresenceControlThreshold,
		threshold,
		[]int{-5, -1, 1, 5},
		settingsPresenceClamp,
		strconv.Itoa(threshold),
	)...)
	retractLabel := settingsText(v.Tr, "settings_btn_pr_retract", i18n.TranslationParams{"state": retractState})
	k.row(settingsToggleButton(k, settingsPagePresence, settingsPresenceControlRetract, retractLabel, stored.RetractEnabled)...)
	k.row(settingsPresenceAIReactRow(k, aiMode)...)
	k.navRow(false)

	return k.screen(settingsCompose(v, "settings_page_presence", body, notice, false))
}

func settingsPresenceRedraw(p *settingsPress, notice string) *settingsScreen {
	stored, err := settingsPresenceReadSettings(p.group.Id)
	if err != nil {
		log.Warnf("[Settings] the presence setting of chat %d could not be read: %v", p.group.Id, err)
		p.cannotOpen()
		return nil
	}
	screen := renderSettingsPresence(p.view(), stored, settingsPresenceConvUnlockedForView(p.group.Id), notice)
	return &screen
}

func settingsPresenceFromResult(p *settingsPress, result presenceApplyResult) *settingsScreen {
	if result.Outcome != presenceApplied {
		return settingsPresenceRedraw(p, settingsNoticeWarn(settingsText(p.tr, "common_settings_save_failed")))
	}
	if result.Settings == nil {
		return settingsPresenceRedraw(p, "")
	}
	screen := renderSettingsPresence(p.view(), *result.Settings, settingsPresenceConvUnlockedForView(p.group.Id), "")
	return &screen
}

func settingsPresenceSetStored(
	p *settingsPress,
	same func(presence.Settings) bool,
	apply func(context.Context) presenceApplyResult,
) *settingsScreen {
	if settingsPresenceGroupRefused(p) || settingsPresenceStaffGate(p) {
		return nil
	}
	return settingsPresenceApplyStored(p, same, apply)
}

func settingsPresenceApplyStored(
	p *settingsPress,
	same func(presence.Settings) bool,
	apply func(context.Context) presenceApplyResult,
) *settingsScreen {
	stored, err := settingsPresenceReadSettings(p.group.Id)
	if err != nil {
		log.Warnf("[Settings] the presence setting of chat %d could not be read: %v", p.group.Id, err)
		p.cannotOpen()
		return nil
	}
	if same(stored) {
		screen := renderSettingsPresence(p.view(), stored, settingsPresenceConvUnlockedForView(p.group.Id), "")
		return &screen
	}
	ctx, cancel := context.WithTimeout(context.Background(), settingsReadTimeout)
	defer cancel()
	return settingsPresenceFromResult(p, apply(ctx))
}

func settingsPresenceSetConsensus(p *settingsPress, value string) *settingsScreen {
	enabled := value == "1"
	return settingsPresenceSetStored(
		p,
		func(stored presence.Settings) bool { return stored.ConsensusEnabled == enabled },
		func(ctx context.Context) presenceApplyResult {
			return applyPresenceConsensusEnabled(ctx, p.group.Id, enabled, p.query.From.Id)
		},
	)
}

func settingsPresenceSetThreshold(p *settingsPress, value string) *settingsScreen {
	if !settingsPresenceValidThreshold(value) {
		p.answer(settingsText(p.tr, "common_callback_invalid_request"), true)
		return nil
	}
	threshold, _ := strconv.Atoi(value)
	return settingsPresenceSetStored(
		p,
		func(stored presence.Settings) bool { return stored.ConsensusThreshold == threshold },
		func(ctx context.Context) presenceApplyResult {
			return applyPresenceConsensusThreshold(ctx, p.group.Id, threshold, p.query.From.Id)
		},
	)
}

func settingsPresenceSetRetract(p *settingsPress, value string) *settingsScreen {
	enabled := value == "1"
	return settingsPresenceSetStored(
		p,
		func(stored presence.Settings) bool { return stored.RetractEnabled == enabled },
		func(ctx context.Context) presenceApplyResult {
			return applyPresenceRetractEnabled(ctx, p.group.Id, enabled, p.query.From.Id)
		},
	)
}

// settingsPresenceSetAIReact enforces Owner×Admin double-gating (PRES-04): enabling Low or Medium
// requires Owner-enabled Conversations with a configured provider (failing closed on DB read error
// and alerting settings_pr_ai_need_owner when locked), whereas setting Off always succeeds so an
// admin can turn AI reactions off even after Conversations is disabled.
func settingsPresenceSetAIReact(p *settingsPress, value string) *settingsScreen {
	if settingsPresenceGroupRefused(p) || settingsPresenceStaffGate(p) {
		return nil
	}
	mode, ok := settingsPresenceAIReactCodes[value]
	if !ok {
		p.answer(settingsText(p.tr, "common_callback_invalid_request"), true)
		return nil
	}
	if mode != presence.AIReactModeOff {
		unlocked, err := settingsPresenceConvUnlocked(p.group.Id)
		if err != nil {
			log.WithField("chat_id", p.group.Id).Warn("[Settings] conversation lookup failed when enabling presence AI reactions")
			p.cannotOpen()
			return nil
		}
		if !unlocked {
			p.answer(settingsText(p.tr, "settings_pr_ai_need_owner"), true)
			return nil
		}
	}
	return settingsPresenceApplyStored(
		p,
		func(stored presence.Settings) bool {
			return settingsPresenceNormalizeAIReactMode(stored.AIReactMode) == mode
		},
		func(ctx context.Context) presenceApplyResult {
			return applyPresenceAIReactMode(ctx, p.group.Id, mode, p.query.From.Id)
		},
	)
}
