package modules

import (
	"fmt"
	"html"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/fuku_robot/fuku/i18n"
	"github.com/uasneppy/fuku_robot/fuku/utils/formatting"
)

// The Automatic family's events (Phase 23): the warn, the warn limit, the flood notice, the blacklist
// notice and the forward blocklist notice. This file holds their default replies, each an
// eventDefaultReply that returns the text and options the send site sent before it had custom
// messages, byte for byte (TestAutoEventDefaults compares them with the goldens of
// TestLegacyPunishmentNoticeRequests), and the words a custom text fills into {action} and {reason}.

// warnDefaultReply is the default warning: warns_user_warning with the mention of the member's first
// name, the warn count and the limit, then the warns_warning_reason line with the escaped reason when
// one was given. The keyboard is the bot's rows the site computed (Remove warn, and the Rules button
// while the group's rules are set); the site adds the reply to the triggering message. It returns
// today's request byte for byte.
func warnDefaultReply(tr *i18n.Translator, in eventDefaultInput) (string, *gotgbot.SendMessageOpts) {
	temp, _ := tr.GetString("warns_user_warning")
	text := fmt.Sprintf(temp, formatting.MentionHtml(in.Target.Id, in.Target.FirstName), in.Count, in.Limit)
	if in.Reason != "" {
		reasonTemp, _ := tr.GetString("warns_warning_reason")
		text += fmt.Sprintf(reasonTemp, html.EscapeString(in.Reason))
	}
	return text, &gotgbot.SendMessageOpts{
		ParseMode:          formatting.HTML,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
		ReplyMarkup:        &gotgbot.InlineKeyboardMarkup{InlineKeyboard: in.Rows},
	}
}

// warnLimitKeys are the warn limit's texts by limit action.
var warnLimitKeys = map[string]string{
	"kick": "warns_limit_kicked",
	"mute": "warns_limit_muted",
	"ban":  "warns_limit_banned",
}

// warnLimitDefaultReply is the default warn limit message: the limit action's text (warns_limit_kicked,
// _muted or _banned by in.Action, with the count, the limit and the mention of the member's first name;
// none for a mode the code applies nothing for), then "\n - " and each stored reason, escaped. The
// options carry an empty keyboard, which Telegram receives as an empty reply_markup; the site adds the
// reply to the triggering message. It returns today's request byte for byte.
func warnLimitDefaultReply(tr *i18n.Translator, in eventDefaultInput) (string, *gotgbot.SendMessageOpts) {
	var text string
	if key, ok := warnLimitKeys[in.Action]; ok {
		temp, _ := tr.GetString(key)
		text = fmt.Sprintf(temp, in.Count, in.Limit, formatting.MentionHtml(in.Target.Id, in.Target.FirstName))
	}
	var sb strings.Builder
	for _, reason := range in.Reasons {
		fmt.Fprintf(&sb, "\n - %s", html.EscapeString(reason))
	}
	return text + sb.String(), &gotgbot.SendMessageOpts{
		ParseMode:          formatting.HTML,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
		ReplyMarkup:        &gotgbot.InlineKeyboardMarkup{},
	}
}

// floodWords are the untranslated words the flood's default notice splices in by flood action (D-13:
// the default keeps them; a custom text's {action} is translated instead).
var floodWords = map[string]string{
	"mute": "muted",
	"kick": "kicked",
	"ban":  "banned",
}

// floodDefaultReply is the default flood notice: antiflood_checkflood_perform_action with the mention
// of the sender's full name (a sender chat's title) and today's English word for in.Action (none for
// an inert action). The keyboard is the bot's rows the site computed (the Unmute row of a mute, the
// Unban row of a banned sender chat, none otherwise), sent by value; the site adds the topic. It
// returns today's request byte for byte.
func floodDefaultReply(tr *i18n.Translator, in eventDefaultInput) (string, *gotgbot.SendMessageOpts) {
	temp, _ := tr.GetString("antiflood_checkflood_perform_action")
	name := formatting.GetFullName(in.Target.FirstName, in.Target.LastName)
	text := fmt.Sprintf(temp, formatting.MentionHtml(in.Target.Id, name), floodWords[in.Action])
	return text, &gotgbot.SendMessageOpts{
		ParseMode:   formatting.HTML,
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: in.Rows},
	}
}

// blacklistNoticeKeys are the punishment notices of the blacklist and the forward blocklist by action.
var blacklistNoticeKeys = map[string]string{
	"mute": "blacklists_bl_watcher_muted_user",
	"kick": "blacklists_bl_watcher_kicked_user",
	"ban":  "blacklists_bl_watcher_banned_user",
}

// blacklistNoticeDefaultReply is the default punishment notice of the blacklist and the forward
// blocklist, which send one template with a different reason: blacklists_bl_watcher_muted_user,
// _kicked_user or _banned_user by in.Action with the mention of the sender's full name (a sender chat's
// title) and in.Reason spliced as today, unescaped (the blacklist's filled reason, or the forward
// blocklist's staff_fwdblock_reason). The options are formatting.Shtml(). An action with no notice has
// no text. It returns today's request byte for byte.
func blacklistNoticeDefaultReply(tr *i18n.Translator, in eventDefaultInput) (string, *gotgbot.SendMessageOpts) {
	key, ok := blacklistNoticeKeys[in.Action]
	if !ok {
		return "", formatting.Shtml()
	}
	temp, _ := tr.GetString(key)
	name := formatting.GetFullName(in.Target.FirstName, in.Target.LastName)
	return fmt.Sprintf(temp, formatting.MentionHtml(in.Target.Id, name), in.Reason), formatting.Shtml()
}

// The default replies are held to the eventDefaultReply shape at compile time.
var (
	_ eventDefaultReply = warnDefaultReply
	_ eventDefaultReply = warnLimitDefaultReply
	_ eventDefaultReply = floodDefaultReply
	_ eventDefaultReply = blacklistNoticeDefaultReply
)

// eventActionWords maps an action's mode word to the locale key of its translated word.
var eventActionWords = map[string]string{
	"warn": "eventmsg_action_warn",
	"mute": "eventmsg_action_mute",
	"kick": "eventmsg_action_kick",
	"ban":  "eventmsg_action_ban",
}

// eventActionWord is the word {action} takes for an action in the translator's language (the
// group's): a short participle such as "muted" that reads after the member's name or after a label
// (D-13). Any other action (an inert mode the code applies nothing for) has no word and renders as
// nothing. The default texts never use it: the flood's default keeps its untranslated English word.
func eventActionWord(tr *i18n.Translator, action string) string {
	key, ok := eventActionWords[action]
	if !ok {
		return ""
	}
	word, _ := tr.GetString(key)
	return word
}

// floodEventReason is the reason {reason} takes for a flood notice in the translator's language: the
// flood has no reason of its own today, so the custom message gets this fixed phrase (D-14).
//
//nolint:unused // first production caller arrives in plan 23-04
func floodEventReason(tr *i18n.Translator) string {
	reason, _ := tr.GetString("eventmsg_reason_flood")
	return reason
}
