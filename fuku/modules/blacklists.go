package modules

import (
	"context"
	"errors"
	"fmt"
	"html"
	"slices"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/fuku_robot/fuku/db/blacklists"
	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/db/lang"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
	"github.com/uasneppy/fuku_robot/fuku/utils/chat_status"
	"github.com/uasneppy/fuku_robot/fuku/utils/formatting"
	"github.com/uasneppy/fuku_robot/fuku/utils/helpers"
	"github.com/uasneppy/fuku_robot/fuku/utils/keyword_matcher"
	"github.com/uasneppy/fuku_robot/fuku/utils/tracing"
)

var blacklistsModule = moduleStruct{
	moduleName:   "Blacklists",
	handlerGroup: 7,
}

// blacklistWordPreviewRunes is how many code points of a too-long word its refusal shows.
const blacklistWordPreviewRunes = 20

// blacklistActionValues are the actions /blacklistaction accepts, and so the actions the
// menu may write, with seconds 0. The forward blocklist shares this vocabulary
// (TestForwardBlockVocabulary), so the timed actions are not in it.
var blacklistActionValues = []string{"mute", "kick", "warn", "ban", "none"}

// blacklistTimedActions are the timed actions (Phase 26, D-09): written for every word only with a
// duration of punishMinSecs to punishMaxSecs, and inert when stored with none.
var blacklistTimedActions = []string{"tban", "tmute"}

// blacklistWriteSeams are the repository writes the apply functions call, a variable so a
// test can make one fail.
type blacklistWriteSeams struct {
	// Add stores one word with the action and its seconds (0 for a plain action).
	Add    func(ctx context.Context, chatID int64, word, action string, secs int) error
	Remove func(ctx context.Context, chatID int64, word string) error
	// RemoveByID deletes one row by ID only when it belongs to the chat and returns its word.
	RemoveByID func(ctx context.Context, chatID, id int64) (word string, removed bool, err error)
	// SetAction sets the action and its seconds of every word of the chat.
	SetAction func(ctx context.Context, chatID int64, action string, secs int) (int64, error)
}

// blacklistWrites is the one place the blacklist commands and the settings menu reach the
// repository's writes through.
var blacklistWrites = blacklistWriteSeams{
	Add:        blacklists.AddBlacklistTimedContext,
	Remove:     blacklists.RemoveBlacklistContext,
	RemoveByID: blacklists.RemoveBlacklistByID,
	SetAction:  blacklists.SetBlacklistActionTimedContext,
}

// blacklistAddOutcome is what applyBlacklistAdd did overall; its zero value is applied.
type blacklistAddOutcome int

const (
	// blacklistAdded means at least one word was stored.
	blacklistAdded blacklistAddOutcome = iota
	// blacklistNothingAdded means every word was already listed or refused.
	blacklistNothingAdded
	// blacklistAddSaveFailed means no word was stored and at least one write failed.
	blacklistAddSaveFailed
)

// blacklistAddResult is what applyBlacklistAdd returns for the caller to word.
type blacklistAddResult struct {
	Outcome blacklistAddOutcome
	// Added are the stored words, lower-cased and raw (the caller escapes them).
	Added []string
	// Already are the words that were already listed, lower-cased.
	Already []string
	// TooLong are the previews of the words over the length limit, as the command shows them.
	TooLong []string
	// TooBroad are the words refused because one common character would match almost every
	// message (keywordTooBroad), as typed.
	TooBroad []string
	// Failed counts the words whose write failed.
	Failed int
}

// blacklistRemoveResult is what applyBlacklistRemove returns.
type blacklistRemoveResult struct {
	// Removed are the words that were deleted, lower-cased and in the order given.
	Removed []string
	// Failed counts the words whose delete failed.
	Failed int
}

// blacklistRemoveByIDOutcome is what applyBlacklistRemoveByID did; its zero value is removed.
type blacklistRemoveByIDOutcome int

const (
	// blacklistRemovedByID means the chat's row was deleted.
	blacklistRemovedByID blacklistRemoveByIDOutcome = iota
	// blacklistRemoveGone means no row of this chat has the ID: already removed, never
	// existed or another chat's row, which are all one answer.
	blacklistRemoveGone
	// blacklistRemoveByIDSaveFailed means the delete failed.
	blacklistRemoveByIDSaveFailed
)

// blacklistRemoveByIDResult is what applyBlacklistRemoveByID returns.
type blacklistRemoveByIDResult struct {
	Outcome blacklistRemoveByIDOutcome
	// Word is the stored word of the deleted row, for the caller's notice.
	Word string
	// Err is the repository error of a failed delete, for the caller's own handling.
	Err error
}

// blacklistActionOutcome is what applyBlacklistAction did; its zero value is applied.
type blacklistActionOutcome int

const (
	// blacklistActionApplied means every word of the chat now has the action.
	blacklistActionApplied blacklistActionOutcome = iota
	// blacklistActionNoWords means the update succeeded and matched no row because the chat
	// has no words; the command still answers its success text (research C-02).
	blacklistActionNoWords
	// blacklistActionUnknown means the action is not one of blacklistActionValues and not a
	// timed action with a duration (a timed action with seconds 0 is unknown, the pinned reply);
	// nothing was written.
	blacklistActionUnknown
	// blacklistActionSaveFailed means the update failed.
	blacklistActionSaveFailed
	// blacklistActionBadDuration means a timed action came with a duration outside
	// punishMinSecs to punishMaxSecs (a word that is not a duration reads as -1); nothing was
	// written.
	blacklistActionBadDuration
)

// blacklistActionResult is what applyBlacklistAction returns.
type blacklistActionResult struct {
	Outcome blacklistActionOutcome
	// Rows is how many words the update matched.
	Rows int64
	// Err is the repository error of a failed update, for the caller's own handling.
	Err error
}

// blacklistSortWords splits typed words into those within the length limit and the previews
// of those over it (the first 20 code points and an ellipsis when longer than that).
func blacklistSortWords(words []string) (valid, tooLong []string) {
	valid = make([]string, 0, len(words))
	for _, word := range words {
		runes := []rune(word)
		if len(runes) <= listKeywordMaxRunes {
			valid = append(valid, word)
			continue
		}
		preview := word
		if len(runes) > blacklistWordPreviewRunes {
			preview = string(runes[:blacklistWordPreviewRunes]) + "..."
		}
		tooLong = append(tooLong, preview)
	}
	return valid, tooLong
}

// blacklistSharedAction is the action and the seconds a new word takes (A19-3, extended by Phase
// 26 D-21): the ones every word of the chat already has when the action is one of
// blacklistActionValues (with seconds 0) or an effective timed action, else warn and 0 (a chat with
// no words, mixed actions or seconds, a timed action stored with no duration, or a stored value the
// commands never write).
func blacklistSharedAction(existing models.BlacklistSettingsSlice) (string, int) {
	shared, secs, seen := "", 0, false
	for _, row := range existing {
		if row == nil {
			continue
		}
		if seen && (row.Action != shared || row.ActionSecs != secs) {
			return "warn", 0
		}
		shared, secs, seen = row.Action, row.ActionSecs, true
	}
	switch {
	case slices.Contains(blacklistActionValues, shared) && secs == 0:
		return shared, 0
	case slices.Contains(blacklistTimedActions, shared) && autoActionEffective(shared, secs):
		return shared, secs
	}
	return "warn", 0
}

// blacklistShownAction is how the commands show an action: an effective timed action as the action
// and the short duration ("tmute 2h"), anything else as stored.
func blacklistShownAction(action string, secs int) string {
	if slices.Contains(blacklistTimedActions, action) && autoActionEffective(action, secs) {
		return action + " " + punishDurationShort(secs)
	}
	return action
}

// applyBlacklistAdd stores the typed words for the chat, each lower-cased with the action and the
// seconds the chat's words share (blacklistSharedAction), and reports which were added, already listed,
// too long, too broad or failed. existing is the caller's read of the chat's current words.
// It judges no person's rights and logs the chat ID and the error only, never a word.
func applyBlacklistAdd(ctx context.Context, chatID int64, words []string, existing models.BlacklistSettingsSlice) blacklistAddResult {
	valid, tooLong := blacklistSortWords(words)
	result := blacklistAddResult{TooLong: tooLong}
	action, secs := blacklistSharedAction(existing)
	seen := make(map[string]struct{}, len(existing)+len(valid))
	for _, word := range existing.Triggers() {
		seen[word] = struct{}{}
	}
	for _, word := range valid {
		if keywordTooBroad(word) {
			result.TooBroad = append(result.TooBroad, word)
			continue
		}
		word = strings.ToLower(word)
		if _, exists := seen[word]; exists {
			result.Already = append(result.Already, word)
			continue
		}
		if err := blacklistWrites.Add(ctx, chatID, word, action, secs); err != nil {
			log.WithFields(log.Fields{
				"chatId": chatID,
				"error":  err,
			}).Error("[Blacklists] Failed to add a blacklist word")
			result.Failed++
			continue
		}
		seen[word] = struct{}{}
		result.Added = append(result.Added, word)
	}
	switch {
	case len(result.Added) > 0:
		result.Outcome = blacklistAdded
	case result.Failed > 0:
		result.Outcome = blacklistAddSaveFailed
	default:
		result.Outcome = blacklistNothingAdded
	}
	return result
}

// applyBlacklistRemove deletes each typed word that is in existing (the caller's read of
// the chat's words) and reports the ones that went and how many deletes failed. It logs the
// chat ID and the error only, never a word.
func applyBlacklistRemove(ctx context.Context, chatID int64, words, existing []string) blacklistRemoveResult {
	var result blacklistRemoveResult
	for _, word := range words {
		word = strings.ToLower(word)
		if !slices.Contains(existing, word) {
			continue
		}
		if err := blacklistWrites.Remove(ctx, chatID, word); err != nil {
			log.WithFields(log.Fields{
				"chatId": chatID,
				"error":  err,
			}).Error("[Blacklists] Failed to remove a blacklist word")
			result.Failed++
			continue
		}
		result.Removed = append(result.Removed, word)
	}
	return result
}

// applyBlacklistRemoveByID deletes one word by its row ID, only when the row belongs to the
// chat, so an ID of another chat deletes nothing. It judges no person's rights and logs the
// chat ID and the error only, never the word.
func applyBlacklistRemoveByID(ctx context.Context, chatID, id int64) blacklistRemoveByIDResult {
	word, removed, err := blacklistWrites.RemoveByID(ctx, chatID, id)
	if err != nil {
		log.WithFields(log.Fields{
			"chatId": chatID,
			"error":  err,
		}).Error("[Blacklists] Failed to remove a blacklist word by ID")
		return blacklistRemoveByIDResult{Outcome: blacklistRemoveByIDSaveFailed, Err: err}
	}
	if !removed {
		return blacklistRemoveByIDResult{Outcome: blacklistRemoveGone}
	}
	return blacklistRemoveByIDResult{Outcome: blacklistRemovedByID, Word: word}
}

// applyBlacklistAction sets the action and its seconds of every word of the chat (D-09). The
// caller lower-cases the action. A value of blacklistActionValues stores seconds 0 (it clears a
// duration); a timed action (tban, tmute) with seconds 0 is blacklistActionUnknown and one with
// seconds outside punishMinSecs to punishMaxSecs is blacklistActionBadDuration, both writing
// nothing; a valid timed action stores both columns in one update; anything else writes nothing. A
// chat with no words is blacklistActionNoWords, which /blacklistaction answers like a success. It
// judges no person's rights and logs the chat ID and the error only.
func applyBlacklistAction(ctx context.Context, chatID int64, action string, secs int) blacklistActionResult {
	switch {
	case slices.Contains(blacklistActionValues, action):
		secs = 0
	case slices.Contains(blacklistTimedActions, action) && secs == 0:
		return blacklistActionResult{Outcome: blacklistActionUnknown}
	case slices.Contains(blacklistTimedActions, action) && !punishDurationValid(secs):
		return blacklistActionResult{Outcome: blacklistActionBadDuration}
	case !slices.Contains(blacklistTimedActions, action):
		return blacklistActionResult{Outcome: blacklistActionUnknown}
	}
	rows, err := blacklistWrites.SetAction(ctx, chatID, action, secs)
	if err != nil {
		log.WithFields(log.Fields{
			"chatId": chatID,
			"error":  err,
		}).Error("[Blacklists] Failed to persist blacklist action")
		return blacklistActionResult{Outcome: blacklistActionSaveFailed, Err: err}
	}
	if rows == 0 {
		return blacklistActionResult{Outcome: blacklistActionNoWords}
	}
	return blacklistActionResult{Outcome: blacklistActionApplied, Rows: rows}
}

func (m moduleStruct) addBlacklist(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	connectedChat := chat_status.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	chat := connectedChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	args := ctx.Args()[1:]
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	var text string

	if !chat_status.IsUserAdmin(b, chat.Id, user.Id) {
		return ext.EndGroups
	}
	if !chat_status.IsBotAdmin(b, ctx, chat) {
		return ext.EndGroups
	}
	if !chat_status.CanUserRestrict(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_restrict_cmd_error", "chat_status_restrict_button_error")
		return ext.EndGroups
	}
	if !chat_status.CanBotRestrict(b, ctx, chat) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_restrict_group_error", "chat_status_bot_restrict_error")
		return ext.EndGroups
	}

	if len(args) == 0 {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_blacklist_give_bl_word")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if len(args) >= 1 {
		updateCtx := tracing.UpdateContext(ctx)
		existing := blacklists.GetBlacklistSettingsContext(updateCtx, chat.Id)
		result := applyBlacklistAdd(updateCtx, chat.Id, args, existing)

		if len(result.TooLong) > 0 {
			text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_blacklist_word_too_long")
			_, err := msg.Reply(b, fmt.Sprintf(text, strings.Join(result.TooLong, ", ")), formatting.Shtml())
			if err != nil {
				log.Error(err)
			}
		}
		if len(result.TooBroad) > 0 {
			escaped := make([]string, 0, len(result.TooBroad))
			for _, word := range result.TooBroad {
				escaped = append(escaped, html.EscapeString(word))
			}
			text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_blacklist_word_too_broad")
			_, err := msg.Reply(b, fmt.Sprintf(text, strings.Join(escaped, ", ")), formatting.Shtml())
			if err != nil {
				log.Error(err)
			}
		}
		if len(result.Added) == 0 && len(result.Already) == 0 && result.Failed == 0 {
			return ext.EndGroups
		}

		if len(result.Already) >= 1 {
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_blacklist_already_blacklisted")
			text += temp + fmt.Sprintf("\n - %s\n\n", strings.Join(result.Already, "\n - "))
		}
		if len(result.Added) >= 1 {
			newBlacklist := make([]string, 0, len(result.Added))
			for _, word := range result.Added {
				newBlacklist = append(newBlacklist, fmt.Sprintf("<code>%s</code>", html.EscapeString(word)))
			}
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_blacklist_added_bl")
			text += temp + fmt.Sprintf("\n - %s\n\n", strings.Join(newBlacklist, "\n - "))
		}
		if result.Failed > 0 {
			temp, _ := tr.GetString("common_settings_save_failed")
			text += temp
		}

		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
	}

	return ext.EndGroups
}

func (m moduleStruct) removeBlacklist(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	connectedChat := chat_status.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	chat := connectedChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	args := ctx.Args()[1:]
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	if !chat_status.IsUserAdmin(b, chat.Id, user.Id) {
		return ext.EndGroups
	}
	if !chat_status.IsBotAdmin(b, ctx, chat) {
		return ext.EndGroups
	}
	if !chat_status.CanUserRestrict(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_restrict_cmd_error", "chat_status_restrict_button_error")
		return ext.EndGroups
	}
	if !chat_status.CanBotRestrict(b, ctx, chat) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_restrict_group_error", "chat_status_bot_restrict_error")
		return ext.EndGroups
	}

	if len(args) == 0 {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_unblacklist_give_bl_word")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else {
		updateCtx := tracing.UpdateContext(ctx)
		allBlWords := blacklists.GetBlacklistSettingsContext(updateCtx, chat.Id).Triggers()
		removedBlacklists := applyBlacklistRemove(updateCtx, chat.Id, args, allBlWords).Removed
		if len(removedBlacklists) <= 0 {
			text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_unblacklist_no_removed_bl")
			_, err := msg.Reply(b, text, nil)
			if err != nil {
				log.Error(err)
				return err
			}
		} else {
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_unblacklist_removed_bl")
			_, err := msg.Reply(b, fmt.Sprintf(temp, strings.Join(removedBlacklists, ", ")), nil)
			if err != nil {
				log.Error(err)
				return err
			}
		}
	}
	return ext.EndGroups
}

func (m moduleStruct) listBlacklists(b *gotgbot.Bot, ctx *ext.Context) error {
	return m.listBlacklistsNamed(b, ctx, "blacklists")
}

// listBlocklists is /blocklists: it honours its own disabled name and, as before, the shared
// name blacklists, so a chat that disabled /blacklists does not show the words through the alias.
func (m moduleStruct) listBlocklists(b *gotgbot.Bot, ctx *ext.Context) error {
	return m.listBlacklistsNamed(b, ctx, "blocklists", "blacklists")
}

// listBlacklistsNamed lists the chat's words unless one of the disabled names covers the command.
func (m moduleStruct) listBlacklistsNamed(b *gotgbot.Bot, ctx *ext.Context, names ...string) error {
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	msg := ctx.EffectiveMessage
	for _, name := range names {
		if chat_status.CheckDisabledCmd(b, msg, name) {
			return ext.EndGroups
		}
	}
	connectedChat := chat_status.IsUserConnected(b, ctx, false, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	chat := connectedChat

	var (
		replyMsgId     int64
		blacklistsText string
	)

	if reply := msg.ReplyToMessage; reply != nil {
		replyMsgId = reply.MessageId
	} else {
		replyMsgId = msg.MessageId
	}

	blSrc := blacklists.GetBlacklistSettingsContext(tracing.UpdateContext(ctx), chat.Id)
	triggers := blSrc.Triggers()
	slices.Sort(triggers)
	var sb strings.Builder
	for _, i := range triggers {
		fmt.Fprintf(&sb, "\n - <code>%s</code>", html.EscapeString(i))
	}
	blacklistsText += sb.String()

	if blacklistsText != "" {
		temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_ls_bl_list_bl")
		blacklistsText = temp + blacklistsText
	} else {
		blacklistsText, _ = tr.GetString(strings.ToLower(m.moduleName) + "_ls_bl_no_blacklisted")
	}

	_, err := msg.Reply(b,
		blacklistsText,
		&gotgbot.SendMessageOpts{
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId:                replyMsgId,
				AllowSendingWithoutReply: true,
			},
			ParseMode: formatting.HTML,
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// blacklistActionArgs reads the arguments of /blacklistaction: one word is the lower-cased action
// with seconds 0 (a timed word alone is then the pinned unknown reply); exactly two words whose
// first is a timed action are that action and the second word's duration, or -1 when it is not a
// valid duration (applyBlacklistAction answers bad duration). Any other two or more words are not
// an action (ok false), the pinned choose-an-option reply.
func blacklistActionArgs(args []string) (action string, secs int, ok bool) {
	switch len(args) {
	case 1:
		return strings.ToLower(args[0]), 0, true
	case 2:
		action = strings.ToLower(args[0])
		if !slices.Contains(blacklistTimedActions, action) {
			return "", 0, false
		}
		secs, valid := parsePunishDuration(args[1])
		if !valid {
			secs = -1
		}
		return action, secs, true
	}
	return "", 0, false
}

func (m moduleStruct) setBlacklistAction(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	connectedChat := chat_status.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	chat := connectedChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	args := ctx.Args()[1:]
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	var rMsg string

	if !chat_status.CanUserRestrict(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_restrict_cmd_error", "chat_status_restrict_button_error")
		return ext.EndGroups
	}
	if !chat_status.CanBotRestrict(b, ctx, chat) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_restrict_group_error", "chat_status_bot_restrict_error")
		return ext.EndGroups
	}

	if len(args) == 0 {
		words := blacklists.GetBlacklistSettingsContext(tracing.UpdateContext(ctx), chat.Id)
		currAction := words.Action()
		if len(words) > 0 && words[0] != nil {
			currAction = blacklistShownAction(words[0].Action, words[0].ActionSecs)
		}
		temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_set_bl_action_current_mode")
		rMsg = fmt.Sprintf(temp, html.EscapeString(currAction))
	} else if action, secs, ok := blacklistActionArgs(args); ok {
		result := applyBlacklistAction(tracing.UpdateContext(ctx), chat.Id, action, secs)
		switch result.Outcome {
		case blacklistActionUnknown:
			rMsg, _ = tr.GetString(strings.ToLower(m.moduleName) + "_set_bl_action_choose_correct_option")
		case blacklistActionBadDuration:
			rMsg, _ = tr.GetString("blacklists_set_bl_action_bad_duration")
		case blacklistActionSaveFailed:
			rMsg, _ = tr.GetString("blacklists_set_bl_action_update_failed")
		default:
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_set_bl_action_changed_mode")
			rMsg = fmt.Sprintf(temp, html.EscapeString(blacklistShownAction(action, secs)))
		}
	} else {
		rMsg, _ = tr.GetString(strings.ToLower(m.moduleName) + "_set_bl_action_choose_correct_option")
	}
	_, err := msg.Reply(b, rMsg, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

func (m moduleStruct) rmAllBlacklists(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	if !chat_status.RequireGroup(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_group_only_error", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.RequireUserOwner(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_owner_cmd_error", "chat_status_owner_button_error", chat_status.WithReply())
		return ext.EndGroups
	}

	text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_rm_all_bl_ask")
	yesText, _ := tr.GetString("button_yes")
	noText, _ := tr.GetString("button_no")
	_, err := msg.Reply(b, text,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{
						{
							Text:         yesText,
							CallbackData: encodeCallbackData("rmAllBlacklist", map[string]string{"a": "yes"}),
						},
						{
							Text:         noText,
							CallbackData: encodeCallbackData("rmAllBlacklist", map[string]string{"a": "no"}),
						},
					},
				},
			},
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (m moduleStruct) buttonHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	user := query.From
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	if query.Message == nil {
		text, _ := tr.GetString("common_callback_message_unavailable")
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return ext.EndGroups
	}

	if !chat_status.RequireUserOwner(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_owner_cmd_error", "chat_status_owner_button_error", chat_status.WithReply())
		return ext.EndGroups
	}

	creatorAction := ""
	if decoded, ok := decodeCallbackData(query.Data, "rmAllBlacklist"); ok {
		creatorAction, _ = decoded.Field("a")
	}
	if creatorAction == "" {
		log.Warnf("[Blacklists] Invalid callback data format: %s", query.Data)
		return answerInvalidCallback(b, ctx, query)
	}
	var helpText string

	switch creatorAction {
	case "yes":
		chatID := query.Message.GetChat().Id
		if err := blacklists.RemoveAllBlacklist(chatID); err != nil {
			log.WithFields(log.Fields{
				"chatId": chatID,
				"error":  err,
			}).Error("Failed to remove all blacklists")
			helpText, _ = tr.GetString("common_settings_save_failed")
			break
		}
		helpText, _ = tr.GetString(strings.ToLower(m.moduleName) + "_rm_all_bl_button_handler_yes")
	case "no":
		helpText, _ = tr.GetString(strings.ToLower(m.moduleName) + "_rm_all_bl_button_handler_no")
	default:
		return answerInvalidCallback(b, ctx, query)
	}

	_, _, err := query.Message.EditText(b, &gotgbot.EditMessageTextOpts{Text: helpText, ParseMode: formatting.HTML})
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = query.Answer(b,
		&gotgbot.AnswerCallbackQueryOpts{
			Text: helpText,
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (m moduleStruct) blacklistWatcher(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender
	if user == nil {
		return ext.ContinueGroups
	}
	if user.IsAnonymousAdmin() {
		return ext.ContinueGroups
	}

	blSettings := blacklists.GetBlacklistSettingsContext(tracing.UpdateContext(ctx), chat.Id)
	triggers := blSettings.Triggers()
	if len(triggers) == 0 {
		return ext.ContinueGroups
	}

	msg := ctx.EffectiveMessage
	matchText := buildModerationMatchText(msg)
	if matchText == "" {
		return ext.ContinueGroups
	}

	cache := keyword_matcher.GetNamedCache("blacklists")
	matcher := cache.GetOrCreateMatcher(chat.Id, triggers)

	firstPattern, found := matcher.FirstMatch(matchText)
	if !found {
		return ext.ContinueGroups
	}
	i := firstPattern
	matched := blSettings.Find(i)
	if matched == nil {
		return ext.ContinueGroups
	}

	if !user.IsAnonymousChannel() && user.IsUser() && user.Id() > 0 && chat_status.IsUserAdminForUpdate(b, ctx, chat.Id, user.Id()) {
		return ext.ContinueGroups
	}
	if !user.IsAnonymousChannel() && user.IsUser() && user.Id() > 0 && chat_status.IsApprovedForUpdate(b, ctx, chat.Id, user.Id()) {
		return ext.ContinueGroups
	}

	if !chat_status.IsBotAdmin(b, ctx, chat) {
		return ext.ContinueGroups
	}

	reason := matched.Reason
	if reason == "" {
		reason = "Blacklisted word: '%s'"
	}

	_ = helpers.DeleteMessageWithErrorHandling(b, chat.Id, msg.MessageId)
	filled := fmt.Sprintf(reason, i)
	if matched.Action == "warn" {
		if user.IsAnonymousChannel() {
			return ext.ContinueGroups
		}

		err := warnsModule.warnThisUser(b, ctx, user.Id(), filled, "warn", warnSourceAuto)
		if err != nil {
			if !errors.Is(err, ext.EndGroups) && !errors.Is(err, ext.ContinueGroups) {
				log.Error(err)
			}
			return err
		}
		return ext.ContinueGroups
	}
	p := autoPunishment{Action: matched.Action, Secs: matched.ActionSecs}
	if !autoActionEffective(p.Action, p.Secs) {
		// none, a timed action stored with no duration and a stored action the watcher applies
		// nothing for post nothing (D-08).
		return ext.ContinueGroups
	}
	p = escalate(tracing.UpdateContext(ctx), b, chat.Id, user.Id(), "blacklist", p)
	applied, notice, err := blacklistPunish(b, ctx, user, p)
	if err != nil {
		log.Error(err)
		return err
	}
	if !notice {
		return ext.ContinueGroups
	}
	if err = blacklistNotice(b, ctx, user, applied, filled); err != nil {
		log.Error(err)
		return err
	}
	return ext.ContinueGroups
}

// blacklistPunish applies p, an effective action of the matched word, to the sender, the one place
// the blacklist's punishment is decided (escalate has already resolved p through the escalation ladder). It returns the
// punishment as applied (Until set for a timed one) and whether a notice is due; an error is a
// punishment Telegram refused, which the watcher logs, after which nothing is posted.
//   - mute, kick and ban send today's requests: a mute or a kick of an anonymous channel sends nothing
//     and posts nothing, a ban of one bans the channel (banChatSenderChat);
//   - an effective timed mute or ban (a duration of 1m to 366d) goes through applyTimedPunishment, and
//     a sender chat gets nothing and no notice (D-13: a sender chat has no timed form, and
//     banChatSenderChat would ban it for good). An end date refused locally (errPunishTimeRanOut,
//     logged by the helper) punishes nobody and posts nothing, the blacklist's inert path (the message
//     is already deleted).
func blacklistPunish(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.Sender, p autoPunishment) (applied autoPunishment, notice bool, err error) {
	chat := ctx.EffectiveChat
	if p.Timed() {
		if !user.IsUser() {
			return p, false, nil
		}
		p.Until, err = applyTimedPunishment(b, chat.Id, user.Id(), p.Action, p.Secs)
		if errors.Is(err, errPunishTimeRanOut) {
			return autoPunishment{Action: p.Action}, false, nil
		}
		return p, err == nil, err
	}
	switch p.Action {
	case "mute":
		if user.IsAnonymousChannel() {
			return p, false, nil
		}
		_, err = b.RestrictChatMember(chat.Id, user.Id(), MutedPermissions, nil)
	case "ban":
		if user.IsAnonymousChannel() {
			_, err = b.BanChatSenderChat(chat.Id, user.Id(), nil)
		} else {
			_, err = b.BanChatMember(chat.Id, user.Id(), nil)
		}
	case "kick":
		if user.IsAnonymousChannel() {
			return p, false, nil
		}
		err = kickMember(b, chat.Id, user.Id())
	}
	return p, err == nil, err
}

// blacklistNotice posts the blacklist's punishment notice after the mute, kick or ban, or the timed mute
// or ban p applied (D-03), through the event message engine: a group's stored blacklist message replaces
// the default, with {action} the word of the action applied and {reason} the reason the default shows (the
// stored reason filled with the matched word), escaped by the engine while the untimed default splices it
// as today (D-14); a timed punishment also fills {duration} and {until} and its default names the duration
// with the reason escaped (D-15, D-16). The site and the reply are the forward blocklist's too
// (listNoticeSite, listNoticeReply); the untimed default is today's request byte for byte, queued as an
// automatic punishment message.
func blacklistNotice(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.Sender, p autoPunishment, reason string) error {
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	reply := listNoticeReply(tr, eventmsg.EventBlacklist, eventSenderTarget(user), p, reason)
	return postEvent(listNoticeSite(b, ctx.EffectiveChat, tr), reply)
}

func LoadBlacklists(dispatcher *ext.Dispatcher) {
	SetModuleEnabled(blacklistsModule.moduleName, true)

	dispatcher.AddHandler(helpers.NewBotCommand("blacklists", blacklistsModule.listBlacklists))
	helpers.AddCmdToDisableable("blacklists")
	dispatcher.AddHandler(helpers.NewBotCommand("blocklists", blacklistsModule.listBlocklists))
	helpers.AddCmdToDisableable("blocklists")
	dispatcher.AddHandler(helpers.NewBotCommand("addblacklist", blacklistsModule.addBlacklist))
	dispatcher.AddHandler(helpers.NewBotCommand("blacklist", blacklistsModule.addBlacklist))
	dispatcher.AddHandler(helpers.NewBotCommand("addblocklist", blacklistsModule.addBlacklist))
	dispatcher.AddHandler(helpers.NewBotCommand("blocklist", blacklistsModule.addBlacklist))
	dispatcher.AddHandler(helpers.NewBotCommand("rmblacklist", blacklistsModule.removeBlacklist))
	dispatcher.AddHandler(helpers.NewBotCommand("rmblocklist", blacklistsModule.removeBlacklist))
	dispatcher.AddHandler(helpers.NewBotCommand("unblacklist", blacklistsModule.removeBlacklist))
	dispatcher.AddHandler(helpers.NewBotCommand("blaction", blacklistsModule.setBlacklistAction))
	dispatcher.AddHandler(helpers.NewBotCommand("blacklistaction", blacklistsModule.setBlacklistAction))
	helpers.MultiCommand(dispatcher, []string{"remallbl", "rmallbl", "rmblocklistall", "rmallblocklist"}, blacklistsModule.rmAllBlacklists)
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("rmAllBlacklist"), blacklistsModule.buttonHandler))
	dispatcher.AddHandlerToGroup(handlers.NewMessage(func(msg *gotgbot.Message) bool {
		return msg.Text != "" || msg.Caption != ""
	}, blacklistsModule.blacklistWatcher), blacklistsModule.handlerGroup)
}

func init() {
	RegisterLegacyModule("Blacklists", 240, LoadBlacklists)
}
