package modules

import (
	"context"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/db/modtuning"
	"github.com/uasneppy/Xenobot-72/xenobot/db/staff"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
)

// The Reason shortcuts page of the settings menu (page rs, Phase 27, TUNE-05) is a table of the list engine
// (settings_list.go) on the engine's plain-text two-step add (settings_list_input.go): the page lists the chat's
// reason shortcuts eight a page (escaped "word -> full text"), deletes one behind a two-tap card that binds the
// row, and adds one from two plain-text messages (the shortcut word, then the full reason text). Every write routes
// through applyReasonShortcutAdd and applyReasonShortcutRemoveByID in modtuning_settings.go; an existing word is
// refused, never overwritten. It is private-only (a menu opened in the group shows only the framework's note and
// runs no DB reads) and hidden in Staff Groups.

const (
	// settingsPageReasons is the p value of the Reason shortcuts page.
	settingsPageReasons = "rs"
)

// The reads of the page, variables only so a test can count or fail them.
var (
	// settingsReasonsRead lists the chat's reason shortcuts (ID, word, full text).
	settingsReasonsRead = settingsListReader(modtuning.ListShortcutsFresh, settingsReasonsEntryOf)
	// settingsReasonsEntry reads one reason shortcut by ID, only when it belongs to the chat.
	settingsReasonsEntry = settingsListEntryReader(modtuning.GetShortcutFresh, settingsReasonsEntryOf)
	// settingsReasonsCount counts the chat's reason shortcuts.
	settingsReasonsCount = modtuning.CountShortcutsFresh
	// settingsReasonsExists reports whether the chat already stores a shortcut word.
	settingsReasonsExists = modtuning.ExistsShortcutFresh
	// settingsReasonsStaffLookup reads a chat's Staff Group row fresh for Hidden.
	settingsReasonsStaffLookup = staff.GetStaffGroupFresh
)

// settingsReasonsEntryOf adapts a stored shortcut row to the engine's entry, with the full reason as Detail.
func settingsReasonsEntryOf(row modtuning.ShortcutEntry) settingsListEntry {
	return settingsListEntry{ID: row.ID, Text: row.Word, Detail: row.Reason}
}

// settingsReasonsHidden hides the page in Staff Groups (definite fresh row); an error shows the page.
func settingsReasonsHidden(group *gotgbot.Chat) bool {
	row, err := settingsReasonsStaffLookup(group.Id)
	return err == nil && row != nil
}

// settingsReasonsPrompt returns the add prompt lines before the footer: word + rules for step 1 ("w"),
// full text + rules for step 2 ("r").
func settingsReasonsPrompt(v settingsView, part string) []string {
	if part == "r" {
		return []string{
			settingsText(v.Tr, "settings_rsn_wait_text"),
			settingsText(v.Tr, "settings_rsn_wait_text_rules", i18n.TranslationParams{"max": models.ReasonShortcutTextMaxRunes}),
		}
	}
	return []string{
		settingsText(v.Tr, "settings_rsn_wait_word"),
		settingsText(v.Tr, "settings_rsn_wait_word_rules", i18n.TranslationParams{"max": models.ReasonShortcutWordMaxRunes}),
	}
}

// settingsReasonsExistsText is the refusal of a shortcut word the chat already has, naming it escaped.
func settingsReasonsExistsText(tr *i18n.Translator, word string) string {
	return settingsListEntryText(tr, "settings_rsn_refuse_exists", "word", word)
}

// settingsReasonsCapText is the refusal of an Add at the cap of models.ReasonShortcutsCapMax shortcuts.
func settingsReasonsCapText(tr *i18n.Translator) string {
	return settingsText(tr, "settings_rsn_refuse_cap", i18n.TranslationParams{"max": models.ReasonShortcutsCapMax})
}

// settingsReasonsCheck validates a normalised shortcut word in step 1 and again fresh before step-2 claim:
// shape via checkReasonShortcutWord, fresh duplicate existence via settingsReasonsExists, and fresh cap via
// settingsReasonsCount. Read errors return an error so the engine keeps the wait.
func settingsReasonsCheck(ctx context.Context, chatID int64, norm string, tr *i18n.Translator) (string, error) {
	switch checkReasonShortcutWord(norm) {
	case reasonWordBlank:
		return settingsText(tr, "settings_entries_refuse_blank"), nil
	case reasonWordMultiWord, reasonWordLeadingSlash, reasonWordTooLong:
		return settingsText(tr, "settings_rsn_refuse_word", i18n.TranslationParams{"max": models.ReasonShortcutWordMaxRunes}), nil
	}
	exists, err := settingsReasonsExists(ctx, chatID, norm)
	if err != nil {
		return "", err
	}
	if exists {
		return settingsReasonsExistsText(tr, norm), nil
	}
	count, err := settingsReasonsCount(ctx, chatID)
	if err != nil {
		return "", err
	}
	if count >= models.ReasonShortcutsCapMax {
		return settingsReasonsCapText(tr), nil
	}
	return "", nil
}

// settingsReasonsCheckPlain validates the step-2 plain full reason text, returning a refusal string to keep the
// wait or "" when accepted.
func settingsReasonsCheckPlain(text string, tr *i18n.Translator) string {
	_, ok, tooLong := checkReasonShortcutText(text)
	if tooLong {
		return settingsText(tr, "settings_rsn_refuse_text_long", i18n.TranslationParams{"max": models.ReasonShortcutTextMaxRunes})
	}
	if !ok {
		return settingsText(tr, "settings_entries_refuse_blank")
	}
	return ""
}

// settingsReasonsApplyPlain stores the shortcut through applyReasonShortcutAdd. A duplicate or cap race between
// steps ends the wait as taken without overwriting; a DB error returns failed so the claim is released.
func settingsReasonsApplyPlain(ctx context.Context, chatID int64, norm, text string, by int64, tr *i18n.Translator) settingsListApplyResult {
	switch applyReasonShortcutAdd(ctx, chatID, norm, text, by) {
	case reasonShortcutSaved:
		return settingsListApplyResult{}
	case reasonShortcutExists:
		return settingsListApplyResult{Outcome: settingsListApplyTaken, Text: settingsReasonsExistsText(tr, norm)}
	case reasonShortcutCap:
		return settingsListApplyResult{Outcome: settingsListApplyTaken, Text: settingsReasonsCapText(tr)}
	default:
		return settingsListApplyResult{Outcome: settingsListApplyFailed}
	}
}

// settingsReasonsRemove deletes one shortcut by its row ID through applyReasonShortcutRemoveByID.
func settingsReasonsRemove(ctx context.Context, chatID, id int64) settingsListRemoval {
	word, outcome := applyReasonShortcutRemoveByID(ctx, chatID, id)
	switch outcome {
	case reasonShortcutRemoved:
		return settingsListRemoval{Outcome: settingsListRemoved, Text: word}
	case reasonShortcutGone:
		return settingsListRemoval{Outcome: settingsListRemoveGone}
	default:
		return settingsListRemoval{Outcome: settingsListRemoveFailed}
	}
}

// settingsReasonsAddGate blocks Add when the chat is already at models.ReasonShortcutsCapMax shortcuts.
func settingsReasonsAddGate(p *settingsPress) *settingsScreen {
	return settingsListCapGate(p, settingsPageReasons, settingsReasonsCount, models.ReasonShortcutsCapMax, settingsReasonsCapText)
}

// settingsReasonsDef is the list-engine table of the Reason shortcuts page.
func settingsReasonsDef() settingsListDef {
	return settingsListDef{
		Key:          settingsPageReasons,
		Order:        45,
		Icon:         "🏷",
		TitleKey:     "settings_page_reasons",
		PagerCode:    "sq",
		AskCode:      "sx",
		YesCode:      "sy",
		Right:        settingsRightRestrict,
		CountMax:     models.ReasonShortcutsCapMax,
		Hidden:       settingsReasonsHidden,
		Remove:       settingsReasonsRemove,
		DeleteAskKey: "settings_rsn_delete_ask",
		RemovedKey:   "settings_rsn_removed",
		Read: func(ctx context.Context, chatID int64) ([]settingsListEntry, error) {
			return settingsReasonsRead(ctx, chatID)
		},
		Entry: func(ctx context.Context, chatID, id int64) (*settingsListEntry, error) {
			return settingsReasonsEntry(ctx, chatID, id)
		},
		Count:      func(ctx context.Context, chatID int64) (int, error) { return settingsReasonsCount(ctx, chatID) },
		EntryParam: "word",
		IntroKey:   "settings_rsn_intro",
		CountKey:   "settings_rsn_count",
		EmptyKey:   "settings_rsn_empty",
		AddCode:    "sm",
		CancelCode: "sc",
		AddKey:     "settings_btn_rsn_add",
		Prompt:     settingsReasonsPrompt,
		AddGate:    settingsReasonsAddGate,
		TwoStep: &settingsListTwoStep{
			Normalise:  normaliseReasonShortcutWord,
			Check:      settingsReasonsCheck,
			CheckPlain: settingsReasonsCheckPlain,
			ApplyPlain: settingsReasonsApplyPlain,
			KeyLineKey: "settings_rsn_word_line",
			SavedKey:   "settings_rsn_saved",
		},
	}
}

func init() {
	registerSettingsList(settingsReasonsDef())
}
