package modules

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/fuku_robot/fuku/db/chats"
	"github.com/uasneppy/fuku_robot/fuku/db/deletion"
	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
	"github.com/uasneppy/fuku_robot/fuku/utils/error_handling"
	"github.com/uasneppy/fuku_robot/fuku/utils/formatting"
	"github.com/uasneppy/fuku_robot/fuku/utils/helpers"
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

// warnLimitTimedKeys are the warn limit's texts of an effective timed limit action, which name the
// duration; they are read only when in.Secs is set (D-15).
var warnLimitTimedKeys = map[string]string{
	"tmute": "warns_limit_tmuted",
	"tban":  "warns_limit_tbanned",
}

// warnLimitDefaultReply is the default warn limit message: the limit action's text (warns_limit_kicked,
// _muted or _banned by in.Action, with the count, the limit and the mention of the member's first name;
// none for a mode the code applies nothing for), then "\n - " and each stored reason, escaped. The
// options carry an empty keyboard, which Telegram receives as an empty reply_markup; the site adds the
// reply to the triggering message. It returns today's request byte for byte. An effective timed limit
// action (in.Action tmute or tban with in.Secs set) reads warns_limit_tmuted or _tbanned instead, with
// the count, the limit, the mention and the duration in the translator's language; a timed mode with no
// duration has no text, as before.
func warnLimitDefaultReply(tr *i18n.Translator, in eventDefaultInput) (string, *gotgbot.SendMessageOpts) {
	var text string
	if key, ok := warnLimitTimedKeys[in.Action]; ok && in.Secs > 0 {
		text, _ = tr.GetString(key, i18n.TranslationParams{
			"count": in.Count, "limit": in.Limit, "user": formatting.MentionHtml(in.Target.Id, in.Target.FirstName),
			"duration": punishDurationLabel(tr, in.Secs),
		})
	} else if key, ok := warnLimitKeys[in.Action]; ok {
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
	_ eventDefaultReply = captchaFailureDefaultReply
	_ eventDefaultReply = aispamNoticeDefaultReply
)

// eventActionWords maps an action's mode word to the locale key of its translated word.
var eventActionWords = map[string]string{
	"warn": "eventmsg_action_warn",
	"mute": "eventmsg_action_mute",
	"kick": "eventmsg_action_kick",
	"ban":  "eventmsg_action_ban",
	// delete is the AI spam filter's deletion (Phase 24).
	"delete": "eventmsg_action_delete",
	// A timed mute or ban reads the mute or ban word; its time is in {duration} and {until} (Phase 26,
	// D-16). The sites ask for it only for an effective timed action.
	"tmute": "eventmsg_action_mute",
	"tban":  "eventmsg_action_ban",
}

// eventActionWord is the word {action} takes for an action in the translator's language (the
// group's): a short participle such as "muted" that reads after the member's name or after a label
// (D-13). A timed mute or ban (tmute, tban) reads the mute or ban word, so a site asks for it only when
// the timed action is effective. Any other action (an inert mode the code applies nothing for) has no
// word and renders as nothing. The default texts never use it: the flood's default keeps its untranslated English word.
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
func floodEventReason(tr *i18n.Translator) string {
	reason, _ := tr.GetString("eventmsg_reason_flood")
	return reason
}

// aispamNoticeDefaultReply is the default AI spam notice (new in Phase 24, off by default): the
// aispam_notice_deleted text with {name} the escaped mention of the member's full name, in HTML with
// link previews off. The site adds the deleted message's topic; it is never a reply. It says only that
// a message was removed: never the provider, a model, a probability or a category (D-36).
func aispamNoticeDefaultReply(tr *i18n.Translator, in eventDefaultInput) (string, *gotgbot.SendMessageOpts) {
	name := formatting.GetFullName(in.Target.FirstName, in.Target.LastName)
	text, _ := tr.GetString("aispam_notice_deleted", i18n.TranslationParams{"name": formatting.MentionHtml(in.Target.Id, name)})
	return text, &gotgbot.SendMessageOpts{
		ParseMode:          formatting.HTML,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
	}
}

// captchaEventReason is the reason {reason} takes for a failed captcha in the translator's language: the
// captcha failure has no reason of its own, so a custom message gets this fixed phrase (D-14).
func captchaEventReason(tr *i18n.Translator) string {
	reason, _ := tr.GetString("eventmsg_reason_captcha")
	return reason
}

// aispamEventReason is the reason {reason} takes for an AI spam deletion in the translator's language. It
// says only that the message looked like spam: never the provider, a model, a probability or a category
// (D-36).
func aispamEventReason(tr *i18n.Translator) string {
	reason, _ := tr.GetString("eventmsg_reason_aispam")
	return reason
}

// eventSenderTarget is the punished sender of an automatic notice (the flood, the blacklist and the
// forward blocklist) as the member a custom message names: a user with their names and username, or,
// for a sender chat, its ID and title as the first name. The default notices mention the same ID and
// full name (Sender.Name), so the two can never name different senders.
func eventSenderTarget(sender *gotgbot.Sender) gotgbot.User {
	switch {
	case sender == nil:
		return gotgbot.User{}
	case sender.Chat != nil:
		return gotgbot.User{Id: sender.Chat.Id, FirstName: sender.Chat.Title}
	case sender.User != nil:
		return gotgbot.User{
			Id: sender.User.Id, FirstName: sender.User.FirstName, LastName: sender.User.LastName, Username: sender.User.Username,
		}
	}
	return gotgbot.User{}
}

// listNoticeSite is the site of the blacklist's and the forward blocklist's punishment notices, which
// post alike: in the group with no reply and no topic, queued as an automatic punishment message, with
// no acting admin ({admin} reads "an admin") and the default sent with b.SendMessage, today's call.
func listNoticeSite(b *gotgbot.Bot, chat *gotgbot.Chat, tr *i18n.Translator) eventSite {
	return eventSite{
		Bot:       b,
		Chat:      chat,
		Tr:        tr,
		Kind:      deletion.KindBotAuto,
		Automatic: true,
		SendDefault: func(text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
			return b.SendMessage(chat.Id, text, opts)
		},
	}
}

// listNoticeReply is the event of a blacklist or forward blocklist punishment notice: the default built
// by blacklistNoticeDefaultReply (today's request, the reason spliced as is), and the values a custom
// text fills, the same reason (which the engine escapes) and the action's translated word. Neither event
// has a count, a limit or a bot row (D-11, D-15).
func listNoticeReply(tr *i18n.Translator, event string, target gotgbot.User, action, reason string) eventReply {
	text, opts := blacklistNoticeDefaultReply(tr, eventDefaultInput{Target: target, Action: action, Reason: reason})
	return eventReply{
		Event:   event,
		Target:  target,
		Values:  eventValues{Reason: reason, Action: eventActionWord(tr, action)},
		Default: text,
		Opts:    opts,
	}
}

// The screening notices (Phase 24). The captcha failure notice is posted by handleCaptchaTimeout, which
// the timer, the cleanup tick, restart recovery and the web check's Fail and Expire all reach, so one
// site covers the timeout and the failed web check alike. captcha.go calls only postCaptchaFailureNotice
// and names neither the engine nor a deletion producer.

// captchaFailureNoticeLife is how long the captcha failure notice stays before it deletes itself, in
// process. The notice never enters the deletion queue (DEL-07). It is a variable so a test can shorten it.
var captchaFailureNoticeLife = 30 * time.Second

// captchaFailureDefaultReply is the default captcha failure notice: buildCaptchaFailureMessage's text for
// in.Action (kick, ban or mute) with the mention of the member's first name ("User" when it is empty) and,
// when in.Count messages were held while the captcha was pending, their count. The options are only the
// HTML parse mode, so the request carries no link preview, keyboard or reply field. It returns today's
// request byte for byte.
func captchaFailureDefaultReply(tr *i18n.Translator, in eventDefaultInput) (string, *gotgbot.SendMessageOpts) {
	name := in.Target.FirstName
	if name == "" {
		name = "User"
	}
	return buildCaptchaFailureMessage(tr, in.Action, in.Target.Id, name, int64(in.Count)),
		&gotgbot.SendMessageOpts{ParseMode: formatting.HTML}
}

// captchaFailureNoticeDelete is the captcha site's schedule: the notice that was posted (custom or
// default) deletes itself after captchaFailureNoticeLife, as before Phase 24, and is never queued. A send
// that posted nothing schedules nothing.
func captchaFailureNoticeDelete(bot *gotgbot.Bot, chatID int64) func(*gotgbot.Message) {
	return func(sent *gotgbot.Message) {
		if sent == nil {
			return
		}
		time.AfterFunc(captchaFailureNoticeLife, func() {
			defer error_handling.RecoverFromPanic("captchaFailureMsgDelete", "captcha")
			_ = helpers.DeleteMessageWithErrorHandling(bot, chatID, sent.MessageId)
		})
	}
}

// postCaptchaFailureNotice posts the notice of a failed captcha once the failure action took effect: the
// group's custom captcha message, or the default (captchaFailureDefaultReply) when none is stored. It goes
// to the group as a new message with no reply and no topic, names no acting admin ({admin} reads "an
// admin"), fills {reason} with the captcha's fixed phrase, {action} with the failure action's word, {count} with the messages held while the captcha
// was pending and, for a mute, {until} with the end of the 24 hour mute. The chat's title comes from the
// stored chats row, with no Telegram call. The default is sent with helpers.SendMessageWithErrorHandling,
// today's call, and either message deletes itself after captchaFailureNoticeLife.
func postCaptchaFailureNotice(bot *gotgbot.Bot, chatID int64, tr *i18n.Translator, target gotgbot.User, action string, stored int64) error {
	chat := &gotgbot.Chat{Id: chatID, Type: "supergroup", Title: chats.GetChatSettings(chatID).ChatName}
	text, opts := captchaFailureDefaultReply(tr, eventDefaultInput{Target: target, Action: action, Count: int(stored)})
	values := eventValues{Reason: captchaEventReason(tr), Action: eventActionWord(tr, action), Count: int(stored)}
	if action == "mute" {
		values.Until = time.Now().Add(captchaFailureMuteSpan)
	}
	site := eventSite{
		Bot:       bot,
		Chat:      chat,
		Tr:        tr,
		Automatic: true,
		SendDefault: func(text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
			return helpers.SendMessageWithErrorHandling(bot, chatID, text, opts)
		},
		Schedule: captchaFailureNoticeDelete(bot, chatID),
	}
	return postEvent(site, eventReply{Event: eventmsg.EventCaptcha, Target: target, Values: values, Default: text, Opts: opts})
}
