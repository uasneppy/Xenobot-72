package modules

import (
	"fmt"
	"html"

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

// warnDefaultReply is held to the eventDefaultReply shape at compile time.
var _ eventDefaultReply = warnDefaultReply

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
