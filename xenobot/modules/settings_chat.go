package modules

import (
	"context"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/db/chatconv"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
)

// The Conversations page (page p, Phase 20) is where a group's owner turns Xenobot-72's
// conversations on or off, picks the provider and says who may talk to the bot. It is drawn and
// opened only for the live creator (OwnerOnly) and only in the bot's private chat (PrivateOnly):
// in a menu opened in the group the owner sees the framework's note and no page code runs, and
// every control and action refuses a press from a group menu again before any read or write.
// Every control declares settingsRightOwner, judged on the press's own live getChatMember record.
// Turning conversations on takes two taps through the shared confirm card, whose text is the data
// notice; turning them off takes one. The Message log switch (control l) is separate: one tap each
// way, because the page's notice already says what is kept and what is sent. There is no command to
// compare with.

const (
	// settingsPageChat is the p value of the page.
	settingsPageChat = "p"
	// settingsChatControlSwitch is the on/off toggle (tg only, v 0 or 1).
	settingsChatControlSwitch = "e"
	// settingsChatControlProvider is the provider choice (st only, v a, g or f).
	settingsChatControlProvider = "m"
	// settingsChatControlAudience is the audience choice (st only, v e or a).
	settingsChatControlAudience = "w"
	// settingsChatControlLog is the Message log toggle (tg only, v 0 or 1).
	settingsChatControlLog = "l"
	// settingsChatControlJudge is the /judge verdict destination choice (st only, v g, p or s).
	settingsChatControlJudge = "j"
	// settingsChatControlMemory is the per-member memories toggle (tg only, v 0 or 1).
	settingsChatControlMemory = "r"
	// settingsActChatYes is the On card's Yes (fields: e).
	settingsActChatYes = "py"
)

// settingsChatProviderCodes maps a button's v to the stored provider.
var settingsChatProviderCodes = map[string]string{
	"a": chatconv.ProviderAnthropic,
	"g": chatconv.ProviderGoogle,
	"f": chatconv.ProviderFeatherless,
}

// settingsChatAudienceCodes maps a button's v to the stored audience.
var settingsChatAudienceCodes = map[string]int{"e": chatconv.AudienceEveryone, "a": chatconv.AudienceAdminsApproved}

// settingsChatJudgeCodes maps a button's v to the stored /judge destination: the group, the admin's
// private chat and the linked Staff Group.
var settingsChatJudgeCodes = map[string]int{"g": chatconv.JudgeToGroup, "p": chatconv.JudgeToPrivate, "s": chatconv.JudgeToStaff}

// settingsChatJudgeKeys is the name key of each destination, for the body line and the buttons.
var settingsChatJudgeKeys = map[int]string{
	chatconv.JudgeToGroup:   "settings_btn_chat_judge_group",
	chatconv.JudgeToPrivate: "settings_btn_chat_judge_pm",
	chatconv.JudgeToStaff:   "settings_btn_chat_judge_staff",
}

// settingsChatJudgeKey is the name key of a stored destination; a value outside the stored ones
// reads as the group, as the command reads it.
func settingsChatJudgeKey(to int) string {
	if key, ok := settingsChatJudgeKeys[to]; ok {
		return key
	}
	return settingsChatJudgeKeys[chatconv.JudgeToGroup]
}

// settingsChatRead reads the setting fresh; a variable only so a test can make it fail.
var settingsChatRead = chatconv.GetFresh

func init() {
	registerSettingsPage(settingsPage{
		Key:         settingsPageChat,
		Order:       89,
		Icon:        "🗨",
		TitleKey:    "settings_page_chat",
		OwnerOnly:   true,
		PrivateOnly: true,
		MainState:   settingsChatMainState,
		Open:        settingsChatOpen,
		Controls: map[string]settingsControl{
			settingsChatControlSwitch: {
				Right: settingsRightOwner, Set: settingsChatSetEnabled,
				Actions: []string{settingsActToggle}, Validate: settingsEditSwitchValue,
			},
			settingsChatControlProvider: {
				Right: settingsRightOwner, Set: settingsChatSetProvider,
				Actions: []string{settingsActSet}, Validate: func(v string) bool { _, ok := settingsChatProviderCodes[v]; return ok },
			},
			settingsChatControlAudience: {
				Right: settingsRightOwner, Set: settingsChatSetAudience,
				Actions: []string{settingsActSet}, Validate: func(v string) bool { _, ok := settingsChatAudienceCodes[v]; return ok },
			},
			settingsChatControlLog: {
				Right: settingsRightOwner, Set: settingsChatSetLog,
				Actions: []string{settingsActToggle}, Validate: settingsEditSwitchValue,
			},
			settingsChatControlMemory: {
				Right: settingsRightOwner, Set: settingsChatSetMemories,
				Actions: []string{settingsActToggle}, Validate: settingsEditSwitchValue,
			},
			settingsChatControlJudge: {
				Right: settingsRightOwner, Set: settingsChatSetJudge,
				Actions: []string{settingsActSet}, Validate: func(v string) bool { _, ok := settingsChatJudgeCodes[v]; return ok },
			},
		},
		Actions: map[string]settingsPageAction{
			settingsActChatYes: {
				Fields: []string{"e"}, Right: settingsRightOwner,
				// The owner field c exists only on a private menu's buttons, so a group menu's
				// press is an invalid request before any lookup.
				Validate: func(fields map[string]string) bool { return fields["c"] != "" && fields["e"] != "" },
				Run:      settingsChatYes,
			},
		},
	})
}

func settingsChatReadSettings(chatID int64) (chatconv.Settings, error) {
	ctx, cancel := context.WithTimeout(context.Background(), settingsReadTimeout)
	defer cancel()
	return settingsChatRead(ctx, chatID)
}

// settingsChatMainState is the page's Main line: Off, or On and the provider's short name. While the
// server cannot run the stored provider the line still shows the stored state; the page explains.
func settingsChatMainState(_ *gotgbot.Bot, tr *i18n.Translator, group *gotgbot.Chat) (string, bool) {
	stored, err := settingsChatReadSettings(group.Id)
	if err != nil {
		return "", false
	}
	if !stored.Enabled {
		return settingsText(tr, "settings_state_off_plain"), true
	}
	return settingsText(tr, "settings_state_chat_on", i18n.TranslationParams{"provider": chatProviderShortName(stored.Provider)}), true
}

// settingsChatOpen is the page's Open. A read error is the cannot-open edit.
func settingsChatOpen(p *settingsPress, notice string) (settingsScreen, error) {
	stored, err := settingsChatReadSettings(p.group.Id)
	if err != nil {
		return settingsScreen{}, err
	}
	return renderSettingsChat(p.view(), stored, notice), nil
}

// settingsChatDataNotice is the data notice for provider, the same text on the page and the card.
func settingsChatDataNotice(tr *i18n.Translator, provider string) string {
	return settingsText(tr, "settings_chat_data_notice", i18n.TranslationParams{"provider": chatProviderName(provider)})
}

// renderSettingsChat draws the page. It is pure: the server's configuration is read through
// chatProviderConfigured, which reads config.AppConfig only. The page names no Staff Group, no
// member and no count.
func renderSettingsChat(v settingsView, stored chatconv.Settings, notice string) settingsScreen {
	audienceKey := "settings_chat_audience_everyone"
	if stored.Audience == chatconv.AudienceAdminsApproved {
		audienceKey = "settings_chat_audience_admins"
	}
	state := settingsStateWord(v.Tr, stored.Enabled)
	body := []string{
		settingsText(v.Tr, "settings_chat_intro"),
		"",
		settingsText(v.Tr, "settings_chat_status", i18n.TranslationParams{"state": state}),
		settingsText(v.Tr, "settings_chat_provider_line", i18n.TranslationParams{"provider": chatProviderName(stored.Provider)}),
		settingsText(v.Tr, "settings_chat_audience_line", i18n.TranslationParams{"audience": settingsText(v.Tr, audienceKey)}),
	}
	logState := settingsStateWord(v.Tr, stored.LogEnabled)
	body = append(body, settingsText(v.Tr, "settings_chat_log_status", i18n.TranslationParams{"state": logState}),
		settingsText(v.Tr, "settings_chat_memory_status", i18n.TranslationParams{"state": settingsStateWord(v.Tr, stored.MemoriesEnabled)}),
		settingsText(v.Tr, "settings_chat_judge_line", i18n.TranslationParams{"target": settingsText(v.Tr, settingsChatJudgeKey(stored.JudgeTo))}))
	anyConfigured := chatAnyProviderConfigured()
	switch {
	case !anyConfigured:
		body = append(body, "🚫 "+settingsText(v.Tr, "settings_chat_not_set_up"))
	case !chatProviderConfigured(stored.Provider):
		body = append(body, "🚫 "+settingsText(v.Tr, "settings_chat_provider_missing"))
	}
	body = append(body, "", settingsText(v.Tr, "settings_chat_hint"), "", settingsChatDataNotice(v.Tr, stored.Provider),
		"", settingsText(v.Tr, "settings_chat_log_notice", i18n.TranslationParams{"provider": chatProviderName(stored.Provider)}),
		"", settingsText(v.Tr, "settings_chat_memory_notice", i18n.TranslationParams{"provider": chatProviderName(stored.Provider)}))

	k := newSettingsKeyboard(v)
	if anyConfigured || stored.Enabled {
		label := settingsText(v.Tr, "settings_btn_chat_toggle", i18n.TranslationParams{"state": state})
		k.row(settingsToggleButton(k, settingsPageChat, settingsChatControlSwitch, label, stored.Enabled)...)
	}
	var providers []gotgbot.InlineKeyboardButton
	for _, code := range []string{"a", "g", "f"} {
		provider := settingsChatProviderCodes[code]
		if !chatProviderConfigured(provider) {
			continue
		}
		label := chatProviderName(provider)
		if provider == stored.Provider {
			label = settingsSelectedPrefix + label
		}
		if button, ok := k.button(label, map[string]string{"a": settingsActSet, "p": settingsPageChat, "t": settingsChatControlProvider, "v": code}); ok {
			providers = append(providers, button)
		}
	}
	// Put at most two providers on a row so brand names stay readable on narrow screens (still <= 8 rows with the Presence shortcut).
	for i := 0; i < len(providers); i += 2 {
		end := min(i+2, len(providers))
		k.row(providers[i:end]...)
	}
	var audience []gotgbot.InlineKeyboardButton
	for _, choice := range []struct{ code, key string }{{"e", "settings_btn_chat_everyone"}, {"a", "settings_btn_chat_admins"}} {
		label := settingsText(v.Tr, choice.key)
		if settingsChatAudienceCodes[choice.code] == stored.Audience {
			label = settingsSelectedPrefix + label
		}
		if button, ok := k.button(label, map[string]string{"a": settingsActSet, "p": settingsPageChat, "t": settingsChatControlAudience, "v": choice.code}); ok {
			audience = append(audience, button)
		}
	}
	k.row(audience...)
	if anyConfigured || stored.LogEnabled {
		label := settingsText(v.Tr, "settings_btn_chat_log", i18n.TranslationParams{"state": logState})
		k.row(settingsToggleButton(k, settingsPageChat, settingsChatControlLog, label, stored.LogEnabled)...)
	}
	k.row(settingsChatJudgeRow(k, stored.JudgeTo)...)
	// The Presence shortcut shares its row with the memories toggle (drawn like the log toggle, so Off is
	// always reachable): the page stays inside its 8-row budget with all three providers configured.
	var lastRow []gotgbot.InlineKeyboardButton
	if anyConfigured || stored.MemoriesEnabled {
		label := settingsText(v.Tr, "settings_btn_chat_memory", i18n.TranslationParams{"state": settingsStateIcon(stored.MemoriesEnabled)})
		lastRow = append(lastRow, settingsToggleButton(k, settingsPageChat, settingsChatControlMemory, label, stored.MemoriesEnabled)...)
	}
	if btn, ok := k.button(settingsText(v.Tr, "settings_btn_chat_presence"), map[string]string{
		"a": settingsActOpen, "p": settingsPagePresence,
	}); ok {
		lastRow = append(lastRow, btn)
	}
	k.row(lastRow...)
	k.navRow(false)
	return k.screen(settingsCompose(v, "settings_page_chat", body, notice, false))
}

// settingsChatJudgeRow is the /judge destination choice: one row of three buttons, the stored one
// marked. It names no Staff Group: the third label is the feature's name only.
func settingsChatJudgeRow(k *settingsKeyboard, stored int) []gotgbot.InlineKeyboardButton {
	var row []gotgbot.InlineKeyboardButton
	for _, code := range []string{"g", "p", "s"} {
		to := settingsChatJudgeCodes[code]
		label := settingsText(k.view.Tr, settingsChatJudgeKeys[to])
		if to == stored {
			label = settingsSelectedPrefix + label
		}
		if button, ok := k.button(label, map[string]string{"a": settingsActSet, "p": settingsPageChat, "t": settingsChatControlJudge, "v": code}); ok {
			row = append(row, button)
		}
	}
	return row
}

// renderSettingsChatCard draws the On card: the question, the data notice and [Yes | No].
func renderSettingsChatCard(v settingsView, provider string) settingsScreen {
	k := newSettingsKeyboard(v)
	settingsCardRows(k, map[string]string{"a": settingsActChatYes}, settingsPageChat)
	body := []string{settingsText(v.Tr, "settings_chat_confirm"), "", settingsChatDataNotice(v.Tr, provider)}
	return k.screen(settingsCompose(v, "settings_page_chat", body, "", false))
}

// settingsChatGroupRefused refuses a press from a menu opened in the group: conversations are
// turned on only in the bot's private chat. It answers the invalid-request alert and edits
// nothing, before any read or write.
func settingsChatGroupRefused(p *settingsPress) bool {
	if p.private {
		return false
	}
	p.answer(settingsText(p.tr, "common_callback_invalid_request"), true)
	return true
}

// settingsChatRedraw reads the setting and redraws the page with notice; a read error is the
// cannot-open edit.
func settingsChatRedraw(p *settingsPress, notice string) *settingsScreen {
	stored, err := settingsChatReadSettings(p.group.Id)
	if err != nil {
		log.Warnf("[Settings] the conversation setting of chat %d could not be read: %v", p.group.Id, err)
		p.cannotOpen()
		return nil
	}
	screen := renderSettingsChat(p.view(), stored, notice)
	return &screen
}

// settingsChatFromResult draws the page from an apply result: the read-back row, or a fresh read
// with the save-failed notice when the write failed.
func settingsChatFromResult(p *settingsPress, result chatApplyResult) *settingsScreen {
	if result.Outcome != chatApplied {
		return settingsChatRedraw(p, settingsNoticeWarn(settingsText(p.tr, "common_settings_save_failed")))
	}
	if result.Settings == nil {
		return settingsChatRedraw(p, "")
	}
	screen := renderSettingsChat(p.view(), *result.Settings, "")
	return &screen
}

// settingsChatUnavailable refuses a press that needs a provider this server cannot run (a stale
// or forged button): the alert names the reason, nothing is written, and the page is redrawn so
// the stale screen is repaired.
func settingsChatUnavailable(p *settingsPress) *settingsScreen {
	key := "settings_chat_provider_missing"
	if !chatAnyProviderConfigured() {
		key = "settings_chat_not_set_up"
	}
	p.answer(settingsText(p.tr, key), true)
	if screen := settingsChatRedraw(p, ""); screen != nil {
		p.edit(*screen)
	}
	return nil
}

// settingsChatSetEnabled serves the toggle. 0 writes off at once; 1 draws the card when the
// stored provider can run, and writes nothing.
func settingsChatSetEnabled(p *settingsPress, value string) *settingsScreen {
	if settingsChatGroupRefused(p) {
		return nil
	}
	if value == "0" {
		ctx, cancel := context.WithTimeout(context.Background(), settingsReadTimeout)
		defer cancel()
		return settingsChatFromResult(p, applyChatEnabled(ctx, p.group.Id, false, p.query.From.Id))
	}
	stored, err := settingsChatReadSettings(p.group.Id)
	if err != nil {
		p.cannotOpen()
		return nil
	}
	if !chatProviderConfigured(stored.Provider) {
		return settingsChatUnavailable(p)
	}
	if stored.Enabled {
		screen := renderSettingsChat(p.view(), stored, "")
		return &screen
	}
	screen := renderSettingsChatCard(p.view(), stored.Provider)
	return &screen
}

// settingsChatYes writes on after the card's expiry check and the provider check again. The press
// already ran the whole pipeline, so a presser who is no longer the live creator never gets here.
func settingsChatYes(p *settingsPress) *settingsScreen {
	if settingsChatGroupRefused(p) {
		return nil
	}
	expired, ok := settingsCardExpired(p.fields)
	if !ok {
		p.answer(settingsText(p.tr, "common_callback_invalid_request"), true)
		return nil
	}
	if expired {
		return settingsChatRedraw(p, settingsNoticeWarn(settingsText(p.tr, "settings_card_expired")))
	}
	stored, err := settingsChatReadSettings(p.group.Id)
	if err != nil {
		p.cannotOpen()
		return nil
	}
	if !chatProviderConfigured(stored.Provider) {
		return settingsChatUnavailable(p)
	}
	ctx, cancel := context.WithTimeout(context.Background(), settingsReadTimeout)
	defer cancel()
	return settingsChatFromResult(p, applyChatEnabled(ctx, p.group.Id, true, p.query.From.Id))
}

// settingsChatSetProvider stores the provider. A provider this server cannot run is refused before
// any write; the stored one writes nothing.
func settingsChatSetProvider(p *settingsPress, value string) *settingsScreen {
	if settingsChatGroupRefused(p) {
		return nil
	}
	provider := settingsChatProviderCodes[value]
	if !chatProviderConfigured(provider) {
		return settingsChatUnavailable(p)
	}
	stored, err := settingsChatReadSettings(p.group.Id)
	if err != nil {
		p.cannotOpen()
		return nil
	}
	if stored.Provider == provider {
		screen := renderSettingsChat(p.view(), stored, "")
		return &screen
	}
	ctx, cancel := context.WithTimeout(context.Background(), settingsReadTimeout)
	defer cancel()
	return settingsChatFromResult(p, applyChatProvider(ctx, p.group.Id, provider, p.query.From.Id))
}

// settingsChatSetAudience stores who may talk to the bot; the stored value writes nothing.
func settingsChatSetAudience(p *settingsPress, value string) *settingsScreen {
	audience := settingsChatAudienceCodes[value]
	return settingsChatSetStored(p, func(stored chatconv.Settings) bool { return stored.Audience == audience },
		func(ctx context.Context) chatApplyResult {
			return applyChatAudience(ctx, p.group.Id, audience, p.query.From.Id)
		})
}

// settingsChatSetStored is the shared body of a choice that needs no provider: a group menu is
// refused first, a press of the stored value redraws with no write, and anything else is written
// through apply and redrawn from the read-back.
func settingsChatSetStored(p *settingsPress, same func(chatconv.Settings) bool, apply func(context.Context) chatApplyResult) *settingsScreen {
	if settingsChatGroupRefused(p) {
		return nil
	}
	stored, err := settingsChatReadSettings(p.group.Id)
	if err != nil {
		p.cannotOpen()
		return nil
	}
	if same(stored) {
		screen := renderSettingsChat(p.view(), stored, "")
		return &screen
	}
	ctx, cancel := context.WithTimeout(context.Background(), settingsReadTimeout)
	defer cancel()
	return settingsChatFromResult(p, apply(ctx))
}

// settingsChatSetSwitch serves a provider-backed on/off toggle of the Conversations page (the Message log,
// the memories): 0 writes off at once through apply, which also deletes what the switch kept; 1 is one tap
// with no card (the page's notice discloses what is kept and sent), refused when the stored provider cannot
// run here, redrawn without a write when the switch is already on, and otherwise written.
func settingsChatSetSwitch(p *settingsPress, value string, isOn func(chatconv.Settings) bool,
	apply func(ctx context.Context, chatID int64, on bool, by int64) chatApplyResult) *settingsScreen {
	if settingsChatGroupRefused(p) {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), settingsReadTimeout)
	defer cancel()
	if value == "0" {
		return settingsChatFromResult(p, apply(ctx, p.group.Id, false, p.query.From.Id))
	}
	stored, err := settingsChatReadSettings(p.group.Id)
	if err != nil {
		p.cannotOpen()
		return nil
	}
	if !chatProviderConfigured(stored.Provider) {
		return settingsChatUnavailable(p)
	}
	if isOn(stored) {
		screen := renderSettingsChat(p.view(), stored, "")
		return &screen
	}
	return settingsChatFromResult(p, apply(ctx, p.group.Id, true, p.query.From.Id))
}

// settingsChatSetLog serves the Message log toggle; turning it off deletes the log.
func settingsChatSetLog(p *settingsPress, value string) *settingsScreen {
	return settingsChatSetSwitch(p, value, func(s chatconv.Settings) bool { return s.LogEnabled }, applyChatLog)
}

// settingsChatSetMemories serves the memories toggle; turning it off deletes every member's notes of the
// chat.
func settingsChatSetMemories(p *settingsPress, value string) *settingsScreen {
	return settingsChatSetSwitch(p, value, func(s chatconv.Settings) bool { return s.MemoriesEnabled }, applyChatMemories)
}

// settingsChatSetJudge stores where a /judge verdict goes by default; the stored value writes
// nothing. It needs no provider.
func settingsChatSetJudge(p *settingsPress, value string) *settingsScreen {
	to := settingsChatJudgeCodes[value]
	return settingsChatSetStored(p, func(stored chatconv.Settings) bool { return stored.JudgeTo == to },
		func(ctx context.Context) chatApplyResult {
			return applyChatJudgeTo(ctx, p.group.Id, to, p.query.From.Id)
		})
}
