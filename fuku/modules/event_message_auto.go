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
