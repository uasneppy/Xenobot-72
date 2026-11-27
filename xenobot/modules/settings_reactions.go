package modules

import (
	"context"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/db/reactions"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
)

// The Reactions page of the settings menu (page x) is the fifth table of the list engine (settings_list.go). It
// lists the chat's reactions (a keyword and the emoji the bot sets on a message that contains it) eight a page,
// adds one from one typed message "keyword emoji", and deletes one behind the engine's two-tap card that binds the
// row. Every write goes through the apply functions of reactions.go, the functions /addreaction and
// /removereaction write through, so there is no second write path; a keyword the chat already has is refused,
// never overwritten. It reads through the fresh, chat-scoped, non-creating readers of the reactions repository and
// is private-only: a menu opened in the group shows only the framework's note.

const (
	// settingsPageReactions is the p value of the Reactions page.
	settingsPageReactions = "x"
	// settingsActReactionsOpts opens the paged rule-options picker (field v).
	settingsActReactionsOpts = "xo"
	// settingsActReactionsRule opens one rule's options screen (field r).
	settingsActReactionsRule = "xr"
	// settingsActReactionsSetOpt applies one per-rule option (fields r, k, v).
	settingsActReactionsSetOpt = "xs"
	// settingsReactionKeywordMax is the longest keyword the page accepts, in code points.
	settingsReactionKeywordMax = 100
	// settingsReactionExamples are the valid emoji the refusal of an unsupported one names.
	settingsReactionExamples = "👍 ❤ 🔥 🎉 😁 🤔 👏 🙏"
)

// The reads of the page, variables only so a test can count or fail them.
var (
	// settingsReactionsRead lists the chat's reactions (ID, keyword and emoji).
	settingsReactionsRead = settingsListReader(reactions.ListReactionEntriesFresh, settingsReactionsEntryOf)
	// settingsReactionsEntry reads one reaction by ID, only when it belongs to the chat.
	settingsReactionsEntry = settingsListEntryReader(reactions.GetReactionEntryFresh, settingsReactionsEntryOf)
	// settingsReactionsCount counts the chat's reactions.
	settingsReactionsCount = settingsListCounter(reactions.CountReactionsFresh)
)

// settingsReactionsEntryOf is the engine's entry of a stored reaction: the keyword, with the emoji as its detail.
func settingsReactionsEntryOf(row models.Reactions) settingsListEntry {
	return settingsListEntry{ID: int64(row.ID), Text: row.Keyword, Detail: row.Emoji}
}

// settingsReactionsPrompt is the lines of the add prompt before its footer.
func settingsReactionsPrompt(v settingsView, _ string) []string {
	return []string{
		settingsText(v.Tr, "settings_rct_wait"),
		settingsText(v.Tr, "settings_rct_wait_rules", i18n.TranslationParams{"max": settingsReactionKeywordMax}),
	}
}

// settingsReactionsCapText is the refusal of an Add at the cap.
func settingsReactionsCapText(tr *i18n.Translator) string {
	return settingsText(tr, "settings_rct_refuse_cap", i18n.TranslationParams{"max": reactionsCapMax})
}

// settingsReactionsAddGate is consulted before the Add press writes a wait: at the cap (a fresh count) page 1 is
// redrawn with the cap notice and no wait is created. A count that cannot be read is the cannot-open edit.
func settingsReactionsAddGate(p *settingsPress) *settingsScreen {
	return settingsListCapGate(p, settingsPageReactions, settingsReactionsCount, reactionsCapMax, settingsReactionsCapText)
}

// settingsReactionsRemove deletes one reaction by its row ID through applyReactionRemoveByID.
func settingsReactionsRemove(ctx context.Context, chatID, id int64) settingsListRemoval {
	result := applyReactionRemoveByID(ctx, chatID, id)
	switch result.Outcome {
	case reactionRemoved:
		return settingsListRemoval{Outcome: settingsListRemoved, Text: result.Keyword}
	case reactionRemoveAbsent:
		return settingsListRemoval{Outcome: settingsListRemoveGone}
	}
	return settingsListRemoval{Outcome: settingsListRemoveFailed}
}

// settingsReactionsTyped is the door's handler of a typed reply to the page's wait (one step): exactly two
// tokens, the keyword and the emoji. A refusal keeps the wait with the reason and the Cancel button: not two
// tokens, a keyword over settingsReactionKeywordMax code points, an emoji that is not one of Telegram's
// reactions, a keyword the chat already has, the cap reached. The reaction is then claimed and stored through
// applyReactionAdd without overwrite; anything but an added reaction gives the claim back.
func settingsReactionsTyped(r *settingsInputReply, _ settingsListPart) {
	fields := strings.Fields(r.msg.GetText())
	if len(fields) == 0 {
		r.refuse(r.text("settings_edit_refuse_need_text"))
		return
	}
	if len(fields) != 2 {
		r.refuse(r.text("settings_rct_refuse_two"))
		return
	}
	keyword := strings.ToLower(fields[0])
	if utf8.RuneCountInString(keyword) > settingsReactionKeywordMax {
		r.refuse(r.text("settings_rct_refuse_long", i18n.TranslationParams{"max": settingsReactionKeywordMax}))
		return
	}
	claim, claimed := r.claimWait()
	if !claimed {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), settingsReadTimeout)
	defer cancel()
	result := applyReactionAdd(ctx, r.wait.GroupID, keyword, fields[1], false)
	if result.Outcome != reactionAdded {
		r.releaseWait(claim)
		switch result.Outcome {
		case reactionBadEmoji:
			r.refuse(r.text("settings_rct_refuse_emoji", i18n.TranslationParams{"examples": settingsReactionExamples}))
		case reactionExists:
			r.refuse(settingsListEntryText(r.tr, "settings_rct_refuse_exists", "keyword", keyword))
		case reactionCapReached:
			r.refuse(settingsReactionsCapText(r.tr))
		default:
			r.keep("common_settings_save_failed")
		}
		return
	}
	r.finishWait(claim)
	log.Infof("[Settings] user %d added a reaction to chat %d", r.user.Id, r.wait.GroupID)
	notice := settingsNoticeOK(settingsListEntryText(r.tr, "settings_rct_saved", "keyword", keyword))
	if settingsInputIsAlbum(r.msg) {
		notice += "\n" + settingsText(r.tr, "settings_edit_album_cut")
	}
	r.sendSavedList(*r.list, keyword, notice)
	r.closePrompt()
}

var (
	settingsReactionChancePresets   = []int{100, 50, 25, 10}
	settingsReactionCooldownPresets = []int{0, 30, 120, 600}
)

func settingsReactionsAddRow(k *settingsKeyboard, _ []settingsListEntry) []gotgbot.InlineKeyboardButton {
	button, ok := k.button(settingsText(k.view.Tr, "settings_btn_rct_opts"), map[string]string{
		"a": settingsActReactionsOpts,
		"v": "1",
	})
	if !ok {
		return nil
	}
	return []gotgbot.InlineKeyboardButton{button}
}

func settingsReactionsValidOptValue(k, v string) bool {
	switch k {
	case "wb", "mc":
		return settingsEditSwitchValue(v)
	case "ch":
		return v == "10" || v == "25" || v == "50" || v == "100"
	case "cd":
		return v == "0" || v == "30" || v == "120" || v == "600"
	}
	return false
}

func settingsReactionsSetOptValidate(fields map[string]string) bool {
	if !settingsListPrivateField(fields) {
		return false
	}
	if _, ok := settingsListParseID(fields["r"]); !ok {
		return false
	}
	return settingsReactionsValidOptValue(fields["k"], fields["v"])
}

func settingsReactionsCooldownLabel(tr *i18n.Translator, secs int) string {
	switch {
	case secs <= 0:
		return settingsText(tr, "settings_rct_cd_off")
	case secs >= 3600 && secs%3600 == 0:
		return settingsText(tr, "settings_adv_unit_h", i18n.TranslationParams{"n": secs / 3600})
	case secs >= 60 && secs%60 == 0:
		return settingsText(tr, "settings_adv_unit_m", i18n.TranslationParams{"n": secs / 60})
	default:
		return settingsText(tr, "settings_rct_cd_val", i18n.TranslationParams{"secs": secs})
	}
}

func renderSettingsReactionsOpts(v settingsView, entries []settingsListEntry, page int, notice string) settingsScreen {
	k := newSettingsKeyboard(v)
	body := []string{settingsText(v.Tr, "settings_rct_opts_intro")}
	if len(entries) == 0 {
		body = append(body, "", settingsText(v.Tr, "settings_rct_opts_empty"))
	} else {
		page = settingsListClampPage(page, len(entries))
		pages := settingsListPages(len(entries))
		body = append(body, "")
		first := (page - 1) * settingsListPageSize
		shown := entries[first:min(first+settingsListPageSize, len(entries))]
		for i, e := range shown {
			body = append(body, settingsListLine(v, settingsListDef{}, first+i+1, e))
		}
		if pages > 1 {
			body = append(body, "", settingsText(v.Tr, "staff_panel_page", i18n.TranslationParams{"page": page, "pages": pages}))
		}
		var row []gotgbot.InlineKeyboardButton
		for i, e := range shown {
			id := strconv.FormatInt(e.ID, 10)
			if len(id) > settingsListIDMaxDigits {
				log.Errorf("[Settings] reaction options button skipped: the row ID has %d digits", len(id))
				k.omitted++
				continue
			}
			label := "⚙ " + strconv.Itoa(first+i+1)
			if button, ok := k.button(label, map[string]string{"a": settingsActReactionsRule, "r": id}); ok {
				row = append(row, button)
			}
			if len(row) == 4 {
				k.row(row...)
				row = nil
			}
		}
		k.row(row...)
		var pager []gotgbot.InlineKeyboardButton
		if page > 1 {
			if prev, ok := k.button(settingsText(v.Tr, "staff_panel_prev"), map[string]string{"a": settingsActReactionsOpts, "v": strconv.Itoa(page - 1)}); ok {
				pager = append(pager, prev)
			}
		}
		if page < pages {
			if next, ok := k.button(settingsText(v.Tr, "staff_panel_next"), map[string]string{"a": settingsActReactionsOpts, "v": strconv.Itoa(page + 1)}); ok {
				pager = append(pager, next)
			}
		}
		k.row(pager...)
	}
	var nav []gotgbot.InlineKeyboardButton
	if back, ok := k.button(settingsText(v.Tr, "settings_btn_back"), map[string]string{"a": settingsActOpen, "p": settingsPageReactions}); ok {
		nav = append(nav, back)
	}
	if closeBtn, ok := k.button(settingsText(v.Tr, "settings_btn_close"), map[string]string{"a": settingsActClose}); ok {
		nav = append(nav, closeBtn)
	}
	k.row(nav...)
	return k.screen(settingsCompose(v, "settings_rct_opts_title", body, notice, false))
}

func renderSettingsReactionRule(v settingsView, row models.Reactions, notice string) settingsScreen {
	chancePct := int(row.ChancePct)
	if !reactions.ValidChancePct(chancePct) {
		chancePct = reactions.DefaultChancePct
	}
	cooldownSecs := row.CooldownSecs
	if !reactions.ValidCooldownSecs(cooldownSecs) {
		cooldownSecs = 0
	}
	wbState := settingsStateWord(v.Tr, row.WordBoundary)
	mcState := settingsStateWord(v.Tr, row.MatchCaption)

	ruleHeader := "<code>" + settingsListLabel(row.Keyword, settingsListLineMax) + "</code>" +
		settingsListDetailSep + settingsListLabel(row.Emoji, settingsListLineMax)
	body := []string{
		ruleHeader,
		"",
		settingsText(v.Tr, "settings_rct_rule_wb_line", i18n.TranslationParams{"state": wbState}),
		settingsText(v.Tr, "settings_rct_rule_cap_line", i18n.TranslationParams{"state": mcState}),
		settingsText(v.Tr, "settings_rct_rule_chance_line", i18n.TranslationParams{"pct": chancePct}),
		settingsText(v.Tr, "settings_rct_rule_cd_line", i18n.TranslationParams{"cd": settingsReactionsCooldownLabel(v.Tr, cooldownSecs)}),
	}

	k := newSettingsKeyboard(v)
	idStr := strconv.FormatUint(uint64(row.ID), 10)
	nextWB := "1"
	if row.WordBoundary {
		nextWB = "0"
	}
	wbLabel := settingsText(v.Tr, "settings_btn_rct_wb", i18n.TranslationParams{"state": wbState})
	if btn, ok := k.button(wbLabel, map[string]string{"a": settingsActReactionsSetOpt, "r": idStr, "k": "wb", "v": nextWB}); ok {
		k.row(btn)
	}

	nextMC := "1"
	if row.MatchCaption {
		nextMC = "0"
	}
	mcLabel := settingsText(v.Tr, "settings_btn_rct_cap", i18n.TranslationParams{"state": mcState})
	if btn, ok := k.button(mcLabel, map[string]string{"a": settingsActReactionsSetOpt, "r": idStr, "k": "mc", "v": nextMC}); ok {
		k.row(btn)
	}

	chanceRow := make([]gotgbot.InlineKeyboardButton, 0, len(settingsReactionChancePresets))
	for _, pct := range settingsReactionChancePresets {
		label := strconv.Itoa(pct) + "%"
		if pct == chancePct {
			label = settingsSelectedPrefix + label
		}
		if btn, ok := k.button(label, map[string]string{"a": settingsActReactionsSetOpt, "r": idStr, "k": "ch", "v": strconv.Itoa(pct)}); ok {
			chanceRow = append(chanceRow, btn)
		}
	}
	k.row(chanceRow...)

	cdRow := make([]gotgbot.InlineKeyboardButton, 0, len(settingsReactionCooldownPresets))
	for _, secs := range settingsReactionCooldownPresets {
		label := settingsReactionsCooldownLabel(v.Tr, secs)
		if secs == cooldownSecs {
			label = settingsSelectedPrefix + label
		}
		if btn, ok := k.button(label, map[string]string{"a": settingsActReactionsSetOpt, "r": idStr, "k": "cd", "v": strconv.Itoa(secs)}); ok {
			cdRow = append(cdRow, btn)
		}
	}
	k.row(cdRow...)

	var nav []gotgbot.InlineKeyboardButton
	if back, ok := k.button(settingsText(v.Tr, "settings_btn_back"), map[string]string{"a": settingsActReactionsOpts, "v": "1"}); ok {
		nav = append(nav, back)
	}
	if closeBtn, ok := k.button(settingsText(v.Tr, "settings_btn_close"), map[string]string{"a": settingsActClose}); ok {
		nav = append(nav, closeBtn)
	}
	k.row(nav...)
	return k.screen(settingsCompose(v, "settings_rct_rule_title", body, notice, false))
}

func settingsReactionsDrawOpts(p *settingsPress, page int, notice string) (settingsScreen, error) {
	def, _ := settingsListByPage(settingsPageReactions)
	entries, err := settingsListLoad(p, def)
	if err != nil {
		return settingsScreen{}, err
	}
	return renderSettingsReactionsOpts(p.view(), entries, page, notice), nil
}

func settingsReactionsRunOpts(p *settingsPress) *settingsScreen {
	page, _ := settingsListPageField(p.fields)
	screen, err := settingsReactionsDrawOpts(p, page, "")
	if err != nil {
		log.Warnf("[Settings] reaction options list of chat %d could not be read: %v", p.group.Id, err)
		p.cannotOpen()
		return nil
	}
	return &screen
}

func settingsReactionsRunRule(p *settingsPress) *settingsScreen {
	id, ok := settingsListParseID(p.fields["r"])
	if !ok {
		p.answer(settingsText(p.tr, "common_callback_invalid_request"), true)
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), settingsReadTimeout)
	defer cancel()
	row, err := reactionWrites.Entry(ctx, p.group.Id, id)
	if err != nil {
		log.Warnf("[Settings] reaction entry %d of chat %d could not be read: %v", id, p.group.Id, err)
		p.cannotOpen()
		return nil
	}
	if row == nil {
		screen, err := settingsReactionsDrawOpts(p, 1, settingsNoticeWarn(settingsText(p.tr, "settings_entries_already_removed")))
		if err != nil {
			log.Warnf("[Settings] reaction options list of chat %d could not be read: %v", p.group.Id, err)
			p.cannotOpen()
			return nil
		}
		return &screen
	}
	screen := renderSettingsReactionRule(p.view(), *row, "")
	return &screen
}

func settingsReactionsRunSetOpt(p *settingsPress) *settingsScreen {
	id, ok := settingsListParseID(p.fields["r"])
	k := p.fields["k"]
	v := p.fields["v"]
	if !ok || !settingsReactionsValidOptValue(k, v) {
		p.answer(settingsText(p.tr, "common_callback_invalid_request"), true)
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), settingsReadTimeout)
	defer cancel()
	row, err := reactionWrites.Entry(ctx, p.group.Id, id)
	if err != nil {
		log.Warnf("[Settings] reaction entry %d of chat %d could not be read: %v", id, p.group.Id, err)
		p.cannotOpen()
		return nil
	}
	if row == nil {
		p.answer("", false)
		screen, drawErr := settingsReactionsDrawOpts(p, 1, settingsNoticeWarn(settingsText(p.tr, "settings_entries_already_removed")))
		if drawErr != nil {
			log.Warnf("[Settings] reaction options list of chat %d could not be read: %v", p.group.Id, drawErr)
			p.cannotOpen()
			return nil
		}
		return &screen
	}

	wb := row.WordBoundary
	mc := row.MatchCaption
	ch := int(row.ChancePct)
	if !reactions.ValidChancePct(ch) {
		ch = reactions.DefaultChancePct
	}
	cd := row.CooldownSecs
	if !reactions.ValidCooldownSecs(cd) {
		cd = 0
	}
	switch k {
	case "wb":
		wb = v == "1"
	case "mc":
		mc = v == "1"
	case "ch":
		ch, _ = strconv.Atoi(v)
	case "cd":
		cd, _ = strconv.Atoi(v)
	}

	result := applyReactionSetOptions(ctx, p.group.Id, id, wb, mc, ch, cd)
	switch result.Outcome {
	case reactionOptionsUpdated:
		p.answer("", false)
		screen := renderSettingsReactionRule(p.view(), *result.Row, "")
		return &screen
	case reactionOptionsAbsent:
		p.answer("", false)
		screen, drawErr := settingsReactionsDrawOpts(p, 1, settingsNoticeWarn(settingsText(p.tr, "settings_entries_already_removed")))
		if drawErr != nil {
			log.Warnf("[Settings] reaction options list of chat %d could not be read: %v", p.group.Id, drawErr)
			p.cannotOpen()
			return nil
		}
		return &screen
	case reactionOptionsInvalid:
		p.answer(settingsText(p.tr, "common_callback_invalid_request"), true)
		return nil
	default:
		p.answer("", false)
		screen := renderSettingsReactionRule(p.view(), *row, settingsNoticeWarn(settingsText(p.tr, "common_settings_save_failed")))
		return &screen
	}
}

func settingsReactionsActions() map[string]settingsPageAction {
	return map[string]settingsPageAction{
		settingsActReactionsOpts: {
			Fields: []string{"v"},
			Right:  settingsRightAdmin,
			Validate: func(fields map[string]string) bool {
				_, ok := settingsListPageField(fields)
				return settingsListPrivateField(fields) && ok
			},
			Run: settingsReactionsRunOpts,
		},
		settingsActReactionsRule: {
			Fields: []string{"r"},
			Right:  settingsRightAdmin,
			Validate: func(fields map[string]string) bool {
				_, ok := settingsListParseID(fields["r"])
				return settingsListPrivateField(fields) && ok
			},
			Run: settingsReactionsRunRule,
		},
		settingsActReactionsSetOpt: {
			Fields:   []string{"r", "k", "v"},
			Right:    settingsRightChangeInfo,
			Validate: settingsReactionsSetOptValidate,
			Run:      settingsReactionsRunSetOpt,
		},
	}
}

// settingsReactionsDef is the table of the Reactions page.
func settingsReactionsDef() settingsListDef {
	return settingsListDef{
		Key:   settingsPageReactions,
		Order: 73,
		// 👍, not the plan's 🎭: the Anonymous admins page already shows 🎭 on Main.
		Icon:      "👍",
		TitleKey:  "settings_page_reactions",
		PagerCode: "xg",
		AskCode:   "xx",
		YesCode:   "xy",
		// Every reaction write declares the right /addreaction and /removereaction ask for.
		Right:        settingsRightChangeInfo,
		CountMax:     reactionsCapMax,
		Remove:       settingsReactionsRemove,
		DeleteAskKey: "settings_rct_delete_ask",
		RemovedKey:   "settings_rct_removed",
		EntryParam:   "keyword",
		Read: func(ctx context.Context, chatID int64) ([]settingsListEntry, error) {
			return settingsReactionsRead(ctx, chatID)
		},
		Entry: func(ctx context.Context, chatID, id int64) (*settingsListEntry, error) {
			return settingsReactionsEntry(ctx, chatID, id)
		},
		Count:       func(ctx context.Context, chatID int64) (int, error) { return settingsReactionsCount(ctx, chatID) },
		IntroKey:    "settings_rct_intro",
		CountKey:    "settings_rct_count",
		EmptyKey:    "settings_rct_empty",
		AddCode:     "xa",
		CancelCode:  "xc",
		AddKey:      "settings_btn_rct_add",
		AddRowExtra: settingsReactionsAddRow,
		Prompt:      settingsReactionsPrompt,
		AddGate:     settingsReactionsAddGate,
		Typed:       settingsReactionsTyped,
		Actions:     settingsReactionsActions(),
	}
}

func init() {
	registerSettingsList(settingsReactionsDef())
}
