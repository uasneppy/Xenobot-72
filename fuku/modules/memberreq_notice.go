package modules

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/fuku_robot/fuku/db/deletion"
	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/db/memberreq"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
	"github.com/uasneppy/fuku_robot/fuku/utils/formatting"
	"github.com/uasneppy/fuku_robot/fuku/utils/media"
	"github.com/uasneppy/fuku_robot/fuku/utils/ratelimit"
)

// The member requirements notice: one message per episode, in the group's language, that
// names the member and every broken rule. The member's name is a token spliced in after
// translation (memberReqMention escapes it), so it never passes through the translator and a
// name such as "{rules}" is printed literally.

// memberReqCallbackNamespace is the namespace of the "I fixed it" button (mreq|v1|...). The
// handler that answers it is memberReqFixedCallback (memberreq_fixed.go).
const memberReqCallbackNamespace = "mreq"

// memberReqNamePlaceholder stands for the member's mention in a template until it is spliced.
const memberReqNamePlaceholder = "<<mreq-name>>"

// memberReqRulesSeparator joins the rule phrases of a notice; all 8 languages use the comma.
const memberReqRulesSeparator = ", "

// memberReqNoticeRateLimitWait is how long a mute notice waits after the pacer gave up on it
// before it tries again. It is a variable so tests can shorten it.
var memberReqNoticeRateLimitWait = 2 * time.Second

// memberReqMuteNoticeBudget is the longest a mute notice keeps retrying after the pacer gave
// up on it, a safety bound on top of the caller's own context.
const memberReqMuteNoticeBudget = 5 * time.Minute

// memberReqUndeliveredTimeout bounds the lift of a mute whose notice could not be delivered
// (one paced getChat, one paced restrictChatMember), which must finish even when the run that
// posted the notice was cancelled.
const memberReqUndeliveredTimeout = 30 * time.Second

// memberReqText translates key, logging the error and returning "" for a missing key, as the
// other modules' helpers do. The text may hold the name token but never a name.
func memberReqText(tr *i18n.Translator, key string, params ...i18n.TranslationParams) string {
	text, err := tr.GetString(key, params...)
	if err != nil {
		log.Errorf("[MemberReq] translate %s: %v", key, err)
	}
	return text
}

// memberReqRulesText lists the rules in the broken mask as their phrases in rule order,
// joined by a comma and a space.
func memberReqRulesText(tr *i18n.Translator, broken int) string {
	phrases := make([]string, 0, len(memberReqRules))
	for _, rule := range memberReqRules {
		if broken&rule.Bit != 0 {
			phrases = append(phrases, memberReqText(tr, rule.PhraseKey))
		}
	}
	return strings.Join(phrases, memberReqRulesSeparator)
}

// memberReqStillBrokenText is the answer of "I fixed it" to a member who still breaks rules in
// the mask: one to four rules are named (mreq_fixed_still_broken, the phrases in rule order),
// five or more are counted (mreq_fixed_still_broken_many), so the answer always fits Telegram's
// 200 code points. The mask holds only rules whose punishment is not Off. It counts the set
// bits of the table's rows and names no rule by its identity.
func memberReqStillBrokenText(tr *i18n.Translator, broken int) string {
	count := 0
	for _, rule := range memberReqRules {
		if broken&rule.Bit != 0 {
			count++
		}
	}
	if count > memberReqStillBrokenListMax {
		return memberReqText(tr, "mreq_fixed_still_broken_many", i18n.TranslationParams{"n": strconv.Itoa(count)})
	}
	return memberReqText(tr, "mreq_fixed_still_broken", i18n.TranslationParams{"rules": memberReqRulesText(tr, broken)})
}

// memberReqStillBrokenListMax is the most broken rules an answer names; more are counted.
const memberReqStillBrokenListMax = 4

// memberReqNoticeParams are the template parameters of a notice. The name is the token; the
// numbers are the warn count and limit when a warn was recorded.
func memberReqNoticeParams(tr *i18n.Translator, broken int, warn *warnApplyResult) i18n.TranslationParams {
	params := i18n.TranslationParams{
		"name":  memberReqNamePlaceholder,
		"rules": memberReqRulesText(tr, broken),
	}
	if warn != nil {
		params["n"] = strconv.Itoa(warn.Count)
		params["limit"] = strconv.Itoa(warn.Limit)
	}
	return params
}

// memberReqLimitMode is the warn-limit variant a limit action's notice uses: a timed mute or ban
// (tmute, tban) takes the mute or ban variant, with no requirement text of its own (Phase 26, D-14);
// any other mode is itself.
func memberReqLimitMode(mode string) string {
	switch mode {
	case "tmute":
		return "mute"
	case "tban":
		return "ban"
	}
	return mode
}

// memberReqNoticeKey chooses the template of a punishment: kick, ban and mute have their own;
// a warn is the plain warn notice, or the variant of the group's warn mode when the limit
// action took effect (a refused limit action is the plain notice, so a removal that did not
// happen is never claimed). A timed limit action takes the mute or ban variant.
func memberReqNoticeKey(action int, warn *warnApplyResult) string {
	switch action {
	case models.RequirementActionKick:
		return "mreq_notice_kick"
	case models.RequirementActionBan:
		return "mreq_notice_ban"
	case models.RequirementActionMute:
		return "mreq_notice_mute"
	}
	if warn != nil && warn.LimitReached && warn.LimitApplied {
		switch mode := memberReqLimitMode(warn.Mode); mode {
		case "kick", "ban", "mute":
			return "mreq_notice_warn_limit_" + mode
		}
	}
	return "mreq_notice_warn"
}

// memberReqNoticeText builds the notice of a punishment: the template of the punishment
// translated with the broken rules and the warn numbers, then the member's mention spliced in.
//
// A warn, kick or mute notice ends with one trailing sentence per remedy-bearing broken rule (the
// RemedyKey of the rule's table row: the photo remedy, then the AI remedy), in rule order and each
// after one space, appended after translation, so the member learns what to do about a photo the
// bot cannot see or a name that reads as spam. The ban notice and the warn-limit notices never
// carry one: a banned or already removed member has nothing to fix there.
func memberReqNoticeText(tr *i18n.Translator, user gotgbot.User, broken, action int, warn *warnApplyResult) string {
	key := memberReqNoticeKey(action, warn)
	text := memberReqText(tr, key, memberReqNoticeParams(tr, broken, warn))
	for _, remedy := range memberReqNoticeRemedies(key, broken) {
		text += " " + memberReqText(tr, remedy)
	}
	fallback := memberReqText(tr, "mreq_name_unknown", i18n.TranslationParams{"id": strconv.FormatInt(user.Id, 10)})
	return lockdownSplice(text, memberReqNamePlaceholder, memberReqMention(user, fallback))
}

// memberReqNoticeDefault is the default reply builder of a requirement notice for the punishment
// action (warn, kick, ban or mute): memberReqNoticeText for in.Target and the broken rules in.Rules,
// with, for a warn, the warn count in.Count and the group's limit in.Limit, and in.Action naming the
// limit action that took effect (kick, ban or mute: the warn-limit variant) or nothing for the plain
// warn. The options are those of every requirement notice: HTML with link previews off, never a reply, and the
// keyboard in.Rows only when it has rows (the mute's "I fixed it"), so a notice with no button sends
// no reply_markup at all. It returns today's request byte for byte.
func memberReqNoticeDefault(action int) eventDefaultReply {
	return func(tr *i18n.Translator, in eventDefaultInput) (string, *gotgbot.SendMessageOpts) {
		warn := memberReqNoticeWarnOf(action, in)
		opts := &gotgbot.SendMessageOpts{
			ParseMode:          formatting.HTML,
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
		}
		if len(in.Rows) > 0 {
			opts.ReplyMarkup = gotgbot.InlineKeyboardMarkup{InlineKeyboard: in.Rows}
		}
		return memberReqNoticeText(tr, in.Target, in.Rules, action, warn), opts
	}
}

// memberReqNoticeWarnOf is the warn result a requirement notice's builders read from the default input: for a warn,
// the count in.Count and the limit in.Limit, and the limit action in.Action when one took effect (kick, ban or mute:
// the warn-limit variant); nil for the other actions.
func memberReqNoticeWarnOf(action int, in eventDefaultInput) *warnApplyResult {
	if action != models.RequirementActionWarn {
		return nil
	}
	warn := &warnApplyResult{Count: in.Count, Limit: in.Limit}
	switch mode := memberReqLimitMode(in.Action); mode {
	case "kick", "ban", "mute":
		warn.Mode, warn.LimitReached, warn.LimitApplied = mode, true, true
	}
	return warn
}

// memberReqNoticeAppended is what the site appends after a custom requirement notice for the punishment action
// (memberReqNoticeNote with the template key the default notice would use for the same input, so the remedies match the
// default's): the Messages page's preview of a requirement event (settingsMsgEvent.Appended) shows it after the text.
func memberReqNoticeAppended(action int) func(tr *i18n.Translator, in eventDefaultInput) string {
	return func(tr *i18n.Translator, in eventDefaultInput) string {
		return memberReqNoticeNote(tr, in.Rules, memberReqNoticeKey(action, memberReqNoticeWarnOf(action, in)))
	}
}

// memberReqRuleBits is the broken-rule mask of the rules named by their codes in memberReqRules (an unknown code adds
// nothing), so a table that names sample rules never repeats a bit.
func memberReqRuleBits(codes ...string) int {
	mask := 0
	for _, rule := range memberReqRules {
		if slices.Contains(codes, rule.Code) {
			mask |= rule.Bit
		}
	}
	return mask
}

// memberReqNoticeNoteReserve is the room, in UTF-16 units, a custom requirement text leaves for the
// note the bot appends after it (memberReqNoticeNote): the editor holds it back from the text limit.
// The worst note (all eight rules and both remedies) is measured in every language against it.
const memberReqNoticeNoteReserve = 600

// memberReqNoticeNote is what follows a custom requirement text (D-05, D-14): a newline, the
// requirements line (eventmsg_mreq_rules with the broken rules' phrases in rule order) and, after one
// space each, the remedy sentences the notice with this template key carries
// (memberReqNoticeRemedies). The engine appends it to a custom text after its own newline, so a blank
// line precedes the requirements line. It names no member and no group.
func memberReqNoticeNote(tr *i18n.Translator, broken int, key string) string {
	note := "\n" + memberReqText(tr, "eventmsg_mreq_rules", i18n.TranslationParams{"rules": memberReqRulesText(tr, broken)})
	for _, remedy := range memberReqNoticeRemedies(key, broken) {
		note += " " + memberReqText(tr, remedy)
	}
	return note
}

// memberReqNoticeRemedies returns the locale keys of the remedy sentences the notice with this
// template key carries for the broken rules, in table order: only the plain warn, kick and mute
// notices carry any, and only for a broken rule whose row has a RemedyKey. It names no rule by its
// code, so a later rule with a remedy is one table field.
func memberReqNoticeRemedies(key string, broken int) []string {
	switch key {
	case "mreq_notice_warn", "mreq_notice_kick", "mreq_notice_mute":
	default:
		return nil
	}
	var keys []string
	for _, rule := range memberReqRules {
		if rule.RemedyKey != "" && broken&rule.Bit != 0 {
			keys = append(keys, rule.RemedyKey)
		}
	}
	return keys
}

// memberReqPostNotice sends a notice that is not an event (today only the per-chat cap's pause
// notice) as a new message (never a reply) in HTML with link previews off, through the paced retry
// loop of every requirement notice (memberReqNoticeSend), queues it for deletion
// (memberReqScheduleNotice) and returns its message ID. A keyboard with no rows sends none.
func memberReqPostNotice(ctx context.Context, b *gotgbot.Bot, chatID int64, text string, keyboard gotgbot.InlineKeyboardMarkup) (int64, error) {
	opts := &gotgbot.SendMessageOpts{
		ParseMode:          formatting.HTML,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
	}
	withButton := len(keyboard.InlineKeyboard) > 0
	if withButton {
		opts.ReplyMarkup = keyboard
	}
	sent, err := memberReqNoticeSend(ctx, withButton, func(callCtx context.Context) (*gotgbot.Message, error) {
		return b.SendMessageWithContext(callCtx, chatID, text, opts)
	})
	if err != nil {
		return 0, err
	}
	memberReqScheduleNotice(sent, withButton)
	return sent.MessageId, nil
}

// memberReqNoticeSend is the paced retry loop every requirement notice is sent through (the
// requirement site's custom message and default, and the pause notice): each try goes through
// the requirement pacer and is bounded by memberReqCallTimeout. It schedules nothing.
//
// A notice that carries the button (a mute) is never dropped by pacing: when the pacer gives
// up with ratelimit.ErrRateLimited it waits memberReqNoticeRateLimitWait and tries again until
// the context or memberReqMuteNoticeBudget ends. Any other failure (a Telegram refusal, a
// timeout) is tried again once, for every notice; a notice without a button is not retried
// after ErrRateLimited, because nobody is waiting on it.
func memberReqNoticeSend(ctx context.Context, withButton bool, send func(context.Context) (*gotgbot.Message, error)) (*gotgbot.Message, error) {
	if withButton {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, memberReqMuteNoticeBudget)
		defer cancel()
	}
	retried := false
	for {
		var sent *gotgbot.Message
		err := memberReqPaced(ctx, func(callCtx context.Context) error {
			var sendErr error
			sent, sendErr = send(callCtx)
			return sendErr
		})
		if err == nil {
			return sent, nil
		}
		switch {
		case errors.Is(err, ratelimit.ErrRateLimited) && withButton:
			if !memberReqSleep(ctx, memberReqNoticeRateLimitWait) {
				return nil, err
			}
		case errors.Is(err, ratelimit.ErrRateLimited), retried:
			return nil, err
		default:
			retried = true
		}
	}
}

// memberReqEventOf is the event of a requirement notice for the hold's punishment: the warn
// (with its warn-limit variants), kick, ban or mute notice.
func memberReqEventOf(action int) string {
	switch action {
	case models.RequirementActionKick:
		return eventmsg.EventReqKick
	case models.RequirementActionBan:
		return eventmsg.EventReqBan
	case models.RequirementActionMute:
		return eventmsg.EventReqMute
	}
	return eventmsg.EventReqWarn
}

// memberReqEventTarget is the member as the engine sees them: a copy whose first and last names
// and username are stripped of bidirectional controls (memberReqStripBidi) and cut to
// memberReqNameCap runes, so a name cannot reorder a custom requirement text (D-15). The names the
// rules judge are never touched.
func memberReqEventTarget(u gotgbot.User) gotgbot.User {
	clean := func(s string) string {
		s = memberReqStripBidi(s)
		if runes := []rune(s); len(runes) > memberReqNameCap {
			s = string(runes[:memberReqNameCap])
		}
		return s
	}
	u.FirstName, u.LastName, u.Username = clean(u.FirstName), clean(u.LastName), clean(u.Username)
	return u
}

// memberReqPostEventNotice posts the one notice of a requirement episode through the event
// engine (postEventSent), after the punishment took effect, and returns the posted message's ID,
// or 0 with no error when nothing was posted (a Silent warn, kick or ban notice). The site posts a
// new message (never a reply: the member's message may be deleted) with no topic, and sends both
// the custom message and the default through the paced retry loop (memberReqNoticeSend), so a
// mute notice is never dropped by pacing; the default is today's request byte for byte. What was
// posted, custom or default, is queued by memberReqScheduleNotice (the 24 hour cap for a mute).
//
// The reply is text only (a stored media file is ignored) and, for a mute, required (a stored
// Silent is ignored) and keeps the "I fixed it" row below the custom buttons, because the notice is
// the member's only way out of the mute. A custom text is followed by the requirements line and the
// remedies of the default (memberReqNoticeAppended). {reason} is the broken rules' phrases,
// {action} the punishment's word (for a warn whose limit action took effect, that action's word),
// {count} and {limit} a warn's numbers. A refused custom message is replaced by the default; an
// error means neither was delivered, which the caller handles as before.
func memberReqPostEventNotice(ctx context.Context, b *gotgbot.Bot, c memberReqCase, tr *i18n.Translator, user gotgbot.User, hold *models.ChatRequirementHold, warn *warnApplyResult) (int64, error) {
	rows := memberReqNoticeKeyboard(tr, hold.Action, hold.ID, user.Id).InlineKeyboard
	withButton := len(rows) > 0
	in := eventDefaultInput{Target: user, Rules: hold.Rules, Rows: rows}
	values := eventValues{Reason: memberReqRulesText(tr, hold.Rules)}
	switch hold.Action {
	case models.RequirementActionKick:
		values.Action = eventActionWord(tr, "kick")
	case models.RequirementActionBan:
		values.Action = eventActionWord(tr, "ban")
	case models.RequirementActionMute:
		values.Action = eventActionWord(tr, "mute")
	default:
		values.Action = eventActionWord(tr, "warn")
		if warn != nil {
			in.Count, in.Limit = warn.Count, warn.Limit
			values.Count, values.Limit = warn.Count, warn.Limit
			if warn.LimitReached && warn.LimitApplied {
				switch mode := memberReqLimitMode(warn.Mode); mode {
				case "kick", "ban", "mute":
					in.Action = mode
					values.Action = eventActionWord(tr, mode)
				}
			}
		}
	}
	text, opts := memberReqNoticeDefault(hold.Action)(tr, in)
	chatID := c.Chat.Id
	site := eventSite{
		Bot:       b,
		Chat:      &c.Chat,
		Tr:        tr,
		Kind:      deletion.KindBotAuto,
		Automatic: true,
		SendDefault: func(text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
			return memberReqNoticeSend(ctx, withButton, func(callCtx context.Context) (*gotgbot.Message, error) {
				return b.SendMessageWithContext(callCtx, chatID, text, opts)
			})
		},
		SendCustom: func(msg media.GreetingMessage) (*gotgbot.Message, error) {
			return memberReqNoticeSend(ctx, withButton, func(context.Context) (*gotgbot.Message, error) {
				return media.SendGreetingMessage(b, chatID, msg)
			})
		},
		Schedule: func(sent *gotgbot.Message) { memberReqScheduleNotice(sent, withButton) },
	}
	reply := eventReply{
		Event:        memberReqEventOf(hold.Action),
		Target:       memberReqEventTarget(user),
		Values:       values,
		Default:      text,
		PlainDefault: func() string { return memberReqPlainNoticeDefault(hold.Action)(tr, in) },
		Opts:         opts,
		Note:         memberReqNoticeAppended(hold.Action)(tr, in),
		TextOnly:     true,
		Required:     hold.Action == models.RequirementActionMute,
	}
	if withButton {
		reply.KeepRows, reply.KeepRowsLast = rows, true
	}
	post, err := postEventSent(site, reply)
	if err != nil {
		return 0, err
	}
	if post.Msg == nil {
		return 0, nil
	}
	return post.Msg.MessageId, nil
}

// memberReqScheduleNotice queues a notice that was just sent for deletion by the automatic
// punishments timer (MREQ-15, D-11), after the send succeeded and from the sent message's own
// date. A notice with the "I fixed it" button is queued at the 24 hour cap whatever the timer says
// (D-14), because the button is the member's only self-service way out of the mute; every other
// notice follows the timer directly. With the kind Off nothing is written, and a failure of the
// queue never reaches the caller.
func memberReqScheduleNotice(sent *gotgbot.Message, withButton bool) {
	if withButton {
		scheduleBotMessageCapped(sent, deletion.KindBotAuto)
		return
	}
	scheduleBotMessage(sent, deletion.KindBotAuto)
}

// memberReqSleep waits d and reports true, or returns false as soon as the context ends.
func memberReqSleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// memberReqUndeliveredMute is the rule that no member is left muted without the notice that
// carries their "I fixed it" button (UI-SPEC section 7): when the mute notice still could not
// be delivered after its retry, the mute is lifted through the shared unmute code (so an
// active lockdown's stored permissions apply) and the hold goes back to a pending backoff row,
// so the member's next message within memberreq.StaleClaim adds nothing and the first one
// after it starts the episode again with a fresh notice. It runs on a context that survives
// the run's cancellation and logs one Warn line with the chat and user IDs only.
//
// The notice can wait up to memberReqMuteNoticeBudget (5 minutes) for a rate limit, so an
// admin may have restricted, unmuted or banned the member, or the member may have left, since
// the mute was placed. Before the lift one paced live getChatMember (memberReqLiveMember)
// must still show the requirement marker (isMemberReqMute against hold.MuteUntil). When it
// does not, nothing is lifted and no restrictChatMember is sent, so an admin's restriction is
// never overwritten with the group's defaults; the hold still goes back to a pending backoff
// row and one Warn line says so. When the read itself fails the mute is lifted anyway: a read
// failure on top of a failed notice is a double fault, and a member left under a 330 day mute
// with no button is the worse outcome (this fails toward lifting on purpose).
//
// When Telegram also refuses the lift the mute stays and the hold stays punished, so the
// rejoin reset and an admin's /unmute still end it (an accepted limit: it takes two refusals
// in a row). A failed revert is logged at Error and leaves the hold punished with the mute
// lifted, which the member's next message treats as a held member.
//
// It reports whether the mute was lifted, so the join entry can welcome a joiner whose mute did
// not stay (Q4); a member who no longer carries the marker, or whose lift Telegram refused, is not
// reported lifted.
func memberReqUndeliveredMute(ctx context.Context, b *gotgbot.Bot, c memberReqCase, hold *models.ChatRequirementHold) bool {
	liftCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), memberReqUndeliveredTimeout)
	defer cancel()
	if live, readErr := memberReqLiveMember(liftCtx, b, c.Chat.Id, c.User.Id); readErr == nil && !isMemberReqMute(live, hold.MuteUntil) {
		log.Warnf("[MemberReq] the mute notice for user %d in chat %d was not delivered, but the member no longer carries the requirement mute, so nothing was lifted",
			c.User.Id, c.Chat.Id)
		memberReqRevertUndelivered(ctx, c, hold)
		return false
	}
	perms, _, err := memberReqUnmutePermissions(liftCtx, b, c.Chat.Id)
	if err == nil {
		err = memberReqLiftMute(liftCtx, b, c.Chat.Id, c.User.Id, perms)
	}
	if err != nil {
		log.Warnf("[MemberReq] the mute notice for user %d in chat %d was not delivered and the mute could not be lifted, so it stays: %s",
			c.User.Id, c.Chat.Id, memberReqErrText(err))
		return false
	}
	log.Warnf("[MemberReq] the mute notice for user %d in chat %d was not delivered, so the mute was lifted", c.User.Id, c.Chat.Id)
	memberReqRevertUndelivered(ctx, c, hold)
	return true
}

// memberReqRevertUndelivered turns the hold of an undelivered mute notice back into a pending
// backoff row, on a context that survives the run's cancellation. A failure is logged at
// Error with the chat and user IDs only.
func memberReqRevertUndelivered(ctx context.Context, c memberReqCase, hold *models.ChatRequirementHold) {
	recordCtx, recordCancel := memberReqRecordCtx(ctx)
	defer recordCancel()
	if _, err := memberreq.RevertHoldToPending(recordCtx, hold.ID); err != nil {
		log.Errorf("[MemberReq] reverting the hold of user %d in chat %d to pending failed: %s", c.User.Id, c.Chat.Id, memberReqErrText(err))
	}
}

// memberReqFixedButton is the one full-width "I fixed it" button of a mute notice. Its data
// carries only numbers: a=f, h the hold ID and u the member, at most 52 bytes. Anyone else's
// tap is refused by the handler, so the button grants nothing by itself.
func memberReqFixedButton(tr *i18n.Translator, holdID, userID int64) gotgbot.InlineKeyboardMarkup {
	data := encodeCallbackData(memberReqCallbackNamespace, map[string]string{
		"a": "f",
		"h": strconv.FormatInt(holdID, 10),
		"u": strconv.FormatInt(userID, 10),
	})
	if data == "" {
		return gotgbot.InlineKeyboardMarkup{}
	}
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{
		{Text: memberReqText(tr, "mreq_btn_fixed"), CallbackData: data},
	}}}
}

// memberReqNoticeKeyboard is the keyboard of a notice: the button for a mute, none for a warn
// (a warn-limit mute is the warn module's mute and is never lifted by the button), a kick or
// a ban.
func memberReqNoticeKeyboard(tr *i18n.Translator, action int, holdID, userID int64) gotgbot.InlineKeyboardMarkup {
	if action != models.RequirementActionMute {
		return gotgbot.InlineKeyboardMarkup{}
	}
	return memberReqFixedButton(tr, holdID, userID)
}
