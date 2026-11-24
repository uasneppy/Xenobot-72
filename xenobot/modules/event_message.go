package modules

import (
	"context"
	"errors"
	"html"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/deletion"
	"github.com/uasneppy/Xenobot-72/xenobot/db/eventmsg"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/formatting"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/keyboard"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/media"
)

// The event message engine (Phase 21, generic since Phase 23). A group can store its own message for
// an event (a manual punishment such as ban, or a warn); the reply of that event is then the stored
// message instead of the default text. A send site describes where and how it posts (eventSite) and
// calls postEvent after the punishment was applied (D-03): a message never delays, undoes or skips a
// punishment. The moderation pipeline's reply step is one such site (replyEvent), warnThisUser another.
// With no row, an empty row or a read that fails, the site's own default send runs, which is literally
// the call the site made before it had custom messages (TestLegacyModerationRequests and
// TestLegacyPunishmentNoticeRequests pin them).
// No stored text, caption, button or file ID is ever logged: a line names the chat ID and the event.

// eventMessagesLookup reads a chat's stored event messages. It is a seam so a test can make the
// read fail; the default is the cached read, whose key is in skipLocal.
var eventMessagesLookup = eventmsg.Get

// eventMessageReadTimeout bounds the one read of a reply. The punishment has already happened, so a
// slow read only delays the reply, and a read that runs out sends the default.
var eventMessageReadTimeout = 2 * time.Second

// eventValues are the event's own values, filled into the event placeholders of a custom text. A
// field with no value for the event (a permanent ban has no duration) renders as nothing (D-09).
// Action is the translated word of the action taken (eventActionWord), Count and Limit the numbers of
// a warn or a flood (0 renders as nothing), Window a time window's text and Percent a share of a
// count (the flood trigger's share of new members, 0 renders as nothing).
type eventValues struct {
	Reason   string
	Duration string
	Until    time.Time
	Action   string
	Count    int
	Limit    int
	Window   string
	Percent  int
}

// eventActor is the admin who issued the command. Anonymous is set for an anonymous admin, whose
// name {admin} never shows (D-12).
type eventActor struct {
	User      *gotgbot.User
	Anonymous bool
}

// eventDefaultInput is what a default reply builder needs: the punished member, the typed reason
// and the duration text of a timed punishment. The automatic events also pass the warn or message
// count and the limit, the action's mode word (warn, mute, kick or ban, never translated), the stored
// reasons of a warn limit and the bot's own keyboard rows the site computed, so a builder stays pure.
// Rules is the mask of the broken member requirement rules, read only by the requirement builders.
// Window (seconds), Percent and Lockdown are read only by the lockdown builders (event_message_lockdown.go):
// an automatic trigger's window and share of new members, and what a lockdown message's default says
// beside its person (the bot's missing rights for a start; the manual change, whether the lift finished
// and the joiners being unbanned for a lift). Secs is the duration in seconds of an effective timed
// action (tmute, tban), read only by the timed branches of the default builders; 0 for every other
// action, so every untimed default stays byte for byte.
type eventDefaultInput struct {
	Target   gotgbot.User
	Reason   string
	TimeVal  string
	Count    int
	Limit    int
	Action   string
	Reasons  []string
	Rows     [][]gotgbot.InlineKeyboardButton
	Rules    int
	Window   int
	Percent  int
	Lockdown lockdownFacts
	Secs     int
}

// eventDefaultReply builds an event's default reply: the text and the options the send site sent
// before Phase 21, byte for byte, in the translator's language (the group's).
type eventDefaultReply func(tr *i18n.Translator, in eventDefaultInput) (string, *gotgbot.SendMessageOpts)

// eventReply is one event's reply as a send site hands it to replyEvent: the event key, the punished
// member, the event's values and the default reply already built (text and options). Note is a line
// the site appends to its default (the /unmute lockdown note); a custom text is followed by it too.
// PMNote is the private wording of Note, followed after a private copy's text, custom or default.
// KeepRows are the bot's own rows a custom message keeps (nil means the rows of Opts, the Unban row of
// a ban); with KeepRowsLast they follow the custom and {rules} buttons instead of preceding them (the
// warn's Remove warn row). The site sets both, never the stored row, so a custom message cannot drop them.
// Required makes the group message ignore a stored Silent switch (a notice that carries the member's only
// way out, the requirement mute's "I fixed it", is always posted), and TextOnly ignores a stored media file
// (an event whose custom message is text and buttons only). Both are set by the site.
// Appeal marks an automatic event whose applied action is a ban (ban or tban) so postEventSent attaches
// the group's stored appeal button (TUNE-06, P27-04). AppealURL is populated once by postEventSent (or a
// Messages preview) when eventSupportsAppeal holds and is never read for non-ban events.
type eventReply struct {
	Event        string
	Target       gotgbot.User
	Values       eventValues
	Default      string
	PlainDefault func() string
	Opts         *gotgbot.SendMessageOpts
	Note         string
	PMNote       string
	KeepRows     [][]gotgbot.InlineKeyboardButton
	KeepRowsLast bool
	Required     bool
	TextOnly     bool
	Appeal       bool
	AppealURL    string
}

// eventSupportsAppeal reports whether r is a user ban event eligible for the group's appeal button
// (TUNE-06, P27-04): EventBan, EventTimedBan, EventReqBan, or an automatic event with r.Appeal set.
// Channel bans (negative target ID) never carry an appeal button.
func eventSupportsAppeal(r eventReply) bool {
	if r.Target.Id < 0 {
		return false
	}
	if r.Appeal {
		return true
	}
	switch r.Event {
	case eventmsg.EventBan, eventmsg.EventTimedBan, eventmsg.EventReqBan:
		return true
	}
	return false
}

// eventAppealRow builds the single-button URL row for the group's appeal link, or nil when unset or
// invalid (re-checked defensively on read, T-27-09).
func eventAppealRow(tr *i18n.Translator, appealURL string) []gotgbot.InlineKeyboardButton {
	cleaned, ok := validateAppealURL(appealURL)
	if !ok || cleaned == "" {
		return nil
	}
	label := trS(tr, "eventmsg_btn_appeal")
	if label == "" {
		return nil
	}
	return []gotgbot.InlineKeyboardButton{{Text: label, Url: cleaned}}
}

// eventAppendAppealRow appends appealRow as the final row of a fresh copy of rows when non-empty and
// adding it stays within Telegram's 100-button keyboard limit (settingsEditMaxButtons, T-27-10).
func eventAppendAppealRow(rows [][]gotgbot.InlineKeyboardButton, appealRow []gotgbot.InlineKeyboardButton) [][]gotgbot.InlineKeyboardButton {
	if len(appealRow) == 0 || pmCopyButtonCount(rows)+len(appealRow) > settingsEditMaxButtons {
		return rows
	}
	out := make([][]gotgbot.InlineKeyboardButton, 0, len(rows)+1)
	for _, row := range rows {
		out = append(out, append([]gotgbot.InlineKeyboardButton(nil), row...))
	}
	out = append(out, append([]gotgbot.InlineKeyboardButton(nil), appealRow...))
	return out
}

// eventSite is where and how one send site posts an event's reply: the bot, the group and its
// translator (the group's language), the acting admin (empty for an automatic action, which reads as
// "an admin"), the message the reply answers (0 for none) and the topic it goes to (0 for none), the
// deletion kind it is queued under, whether an automatic rule raised it, and the site's own default
// send, which is exactly the call the site made before it had custom messages.
//
// The hooks below are optional (Phase 24) and their zero value is the behaviour every earlier site has:
// SendCustom sends a composed custom message (nil: media.SendGreetingMessage to the site's chat), and
// Schedule takes care of what the group step posted, default or custom alike (nil: queue it under Kind
// with scheduleBotMessage). A site with a deletion of its own (the captcha notice deletes itself after
// 30 s and never enters the queue) sets Schedule. Admit is an admission gate (nil: always admitted): it is
// asked once, after the read and only when the group message or the private copy would be sent, and a false
// answer posts nothing and offers no private copy (the AI spam notice's rate limit). The site bounds its own
// call; the context it is given carries no deadline.
type eventSite struct {
	Bot         *gotgbot.Bot
	Chat        *gotgbot.Chat
	Tr          *i18n.Translator
	Actor       eventActor
	ReplyTo     int64
	ThreadID    int64
	Kind        string
	Automatic   bool
	SendDefault func(text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error)
	SendCustom  func(msg media.GreetingMessage) (*gotgbot.Message, error)
	Schedule    func(sent *gotgbot.Message)
	Admit       func(ctx context.Context) bool
}

// admit asks the site's admission gate, and admits everything when the site has none.
func (site eventSite) admit() bool {
	return site.Admit == nil || site.Admit(context.Background())
}

// sendCustom sends a composed custom message through the site's SendCustom, or with
// media.SendGreetingMessage to the site's chat when the site has none.
func (site eventSite) sendCustom(msg media.GreetingMessage) (*gotgbot.Message, error) {
	if site.SendCustom != nil {
		return site.SendCustom(msg)
	}
	return media.SendGreetingMessage(site.Bot, site.Chat.Id, msg)
}

// schedule hands what the group step posted (nil when the send posted nothing) to the site's
// Schedule, or queues it under the site's kind when the site has none.
func (site eventSite) schedule(sent *gotgbot.Message) {
	if site.Schedule != nil {
		site.Schedule(sent)
		return
	}
	scheduleBotMessage(sent, site.Kind)
}

// eventTokenNames are the event placeholders. {count}, {action}, {limit}, {window} and {percent} belong
// to the event (empty for the manual events), so {count} in an event message is never the member count.
var eventTokenNames = []string{"reason", "admin", "duration", "until", "action", "count", "limit", "window", "percent"}

// eventTokenSentinel is the marker an event placeholder is turned into before the Welcome
// placeholders are filled. The stored text is HTML, where a literal '<' is always escaped, so the
// marker can never come from the owner's text, and a value filled in later is never filled again.
func eventTokenSentinel(name string) string {
	return "<<xenobot-event-" + name + ">>"
}

// eventPlaceholderValues are the escaped values of the event placeholders for one reply. A
// placeholder with no value is the empty string, never raw. {until} is shown in the chat's zone.
func eventPlaceholderValues(tr *i18n.Translator, chatID int64, actor eventActor, v eventValues) map[string]string {
	values := make(map[string]string, len(eventTokenNames))
	for _, name := range eventTokenNames {
		values[name] = ""
	}
	values["reason"] = html.EscapeString(v.Reason)
	values["admin"] = eventAdminName(tr, actor)
	values["duration"] = html.EscapeString(v.Duration)
	values["action"] = html.EscapeString(v.Action)
	values["window"] = html.EscapeString(v.Window)
	if v.Count > 0 {
		values["count"] = strconv.Itoa(v.Count)
	}
	if v.Limit > 0 {
		values["limit"] = strconv.Itoa(v.Limit)
	}
	if v.Percent > 0 {
		values["percent"] = strconv.Itoa(v.Percent)
	}
	if !v.Until.IsZero() {
		values["until"] = formatChatTime(v.Until, chatClockOf(chatID), "02.01.2006 15:04")
	}
	return values
}

// eventAdminName is the escaped full name of the acting admin, or the translated "an admin"
// (eventmsg_admin_anon, the translator's language: the group's) for an anonymous admin or an actor
// the reply cannot name. It never names the Staff Group (D-12).
func eventAdminName(tr *i18n.Translator, actor eventActor) string {
	if actor.Anonymous || actor.User == nil {
		word, _ := tr.GetString("eventmsg_admin_anon")
		return html.EscapeString(word)
	}
	return html.EscapeString(formatting.GetFullName(actor.User.FirstName, actor.User.LastName))
}

// eventAnonActorKey is the ctx.Data key that marks the command's issuer as an anonymous admin. The
// anonymous admin's proof tap re-enters the command with the tapper as the sender, so the mark is
// carried from the original update to the reply in the update's data.
const eventAnonActorKey = "event_message_anonymous_actor"

// markAnonymousActor marks the update's issuer as an anonymous admin, so the event reply reads
// {admin} as "an admin" although the re-entered command's sender is the tapper. verifyAnonymousAdmin
// calls it right before the re-entry.
func markAnonymousActor(ctx *ext.Context) {
	if ctx == nil {
		return
	}
	if ctx.Data == nil {
		ctx.Data = make(map[string]any)
	}
	ctx.Data[eventAnonActorKey] = true
}

// actorIsAnonymous reports whether the command's issuer is an anonymous admin: the update carries
// the proof tap's mark, or the sender is the group itself (an anonymous admin in a group whose
// AnonAdmin mode lets the command run at once, where the moderation context's user is the Group
// Anonymous Bot).
func actorIsAnonymous(ctx *ext.Context) bool {
	if ctx == nil {
		return false
	}
	if marked, _ := ctx.Data[eventAnonActorKey].(bool); marked {
		return true
	}
	return ctx.EffectiveSender != nil && ctx.EffectiveSender.IsAnonymousAdmin()
}

// renderEventText fills a stored text for one reply. Each event placeholder is first turned into
// its sentinel, then formatting.FormattingReplacer fills the Welcome placeholders for the target and
// the group (and adds the {rules} button), then every sentinel is replaced by its escaped value in
// one pass. So a reason that contains {first} and a name that contains {reason} stay as typed, and
// an unknown {word} of the owner's text stays as typed.
func renderEventText(b *gotgbot.Bot, chat *gotgbot.Chat, target *gotgbot.User, stored string, buttons []db.Button, values map[string]string) (string, []db.Button) {
	marks := make([]string, 0, 2*len(eventTokenNames))
	fills := make([]string, 0, 2*len(eventTokenNames))
	for _, name := range eventTokenNames {
		marks = append(marks, "{"+name+"}", eventTokenSentinel(name))
		fills = append(fills, eventTokenSentinel(name), values[name])
	}
	marked := strings.NewReplacer(marks...).Replace(stored)
	filled, btns := formatting.FormattingReplacer(b, chat, target, marked, buttons)
	return strings.NewReplacer(fills...).Replace(filled), btns
}

// eventBotRows are the bot's own keyboard rows of a default reply (the Unban row of a ban), which a
// custom message keeps first. Options with no inline keyboard have none.
func eventBotRows(opts *gotgbot.SendMessageOpts) [][]gotgbot.InlineKeyboardButton {
	if opts == nil {
		return nil
	}
	switch markup := opts.ReplyMarkup.(type) {
	case gotgbot.InlineKeyboardMarkup:
		return markup.InlineKeyboard
	case *gotgbot.InlineKeyboardMarkup:
		if markup != nil {
			return markup.InlineKeyboard
		}
	}
	return nil
}

// composeEventMessage builds the custom message of one reply from the stored row. With a custom text
// the text is filled (renderEventText, which also adds the {rules} button) and followed by r.Note on
// a new line; with none the text is r.Default, the default reply's own text with its reason line (and
// its note, which the site already appended). The media is the row's file and type (text when there
// is no file). The keyboard is the bot's own rows (r.KeepRows, or the rows of the default's options:
// the Unban row of a ban) followed by the custom and rules buttons, or after them with r.KeepRowsLast;
// with neither there is no keyboard. The caller sets the reply target and the topic.
func composeEventMessage(b *gotgbot.Bot, chat *gotgbot.Chat, tr *i18n.Translator, r eventReply, row eventmsg.Message, actor eventActor) media.GreetingMessage {
	text := r.Default
	buttons := row.Buttons
	if row.Text != "" {
		target := r.Target
		text, buttons = renderEventText(b, chat, &target, row.Text, row.Buttons, eventPlaceholderValues(tr, chat.Id, actor, r.Values))
		if r.Note != "" {
			text += "\n" + r.Note
		}
	}
	msg := media.GreetingMessage{Text: text, FileID: row.FileID, MsgType: row.MediaType}
	if row.FileID == "" || row.MediaType == 0 ||
		(row.MediaType == db.STICKER && (r.Note != "" || eventmsg.ReservedTextFor(r.Event, eventmsg.DestGroup) > 0)) {
		msg.FileID, msg.MsgType = "", db.TEXT
	}
	kept := r.KeepRows
	if kept == nil {
		kept = eventBotRows(r.Opts)
	}
	custom := keyboard.BuildKeyboard(buttons)
	// A fresh slice: the default's options are reused by the refusal fallback and must not change.
	var rows [][]gotgbot.InlineKeyboardButton
	if r.KeepRowsLast {
		rows = append(append(rows, custom...), kept...)
	} else {
		rows = append(append(rows, kept...), custom...)
	}
	rows = eventAppendAppealRow(rows, eventAppealRow(tr, r.AppealURL))
	if len(rows) > 0 {
		msg.Keyboard = &gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows}
	}
	return msg
}

// eventMessageRefused reports whether Telegram definitely refused a custom message: a 4xx other than
// 429, or the media sender's ErrNoPermission (it turns a permission refusal into that error). The
// default reply is then sent once (D-13). A 429, a 5xx or a transport error is not a refusal.
func eventMessageRefused(err error) bool {
	return isTelegramClientError(err) || errors.Is(err, media.ErrNoPermission)
}

// eventTargetOf is the punished member as a user, from the getChat answer a send site already read.
func eventTargetOf(info *gotgbot.ChatFullInfo) gotgbot.User {
	if info == nil {
		return gotgbot.User{}
	}
	return gotgbot.User{Id: info.Id, FirstName: info.FirstName, LastName: info.LastName, Username: info.Username}
}

// replyEvent sends the success reply of a moderation command's event through postEvent with the
// moderation pipeline's site (site): a reply to the command, in its topic, queued under
// bot:punish_manual, with the default sent through Msg.Reply as before the engine existed.
func (c *moderationCtx) replyEvent(r eventReply) error {
	return postEvent(c.site(), r)
}

// site is the moderation pipeline's eventSite: the command's chat, translator and issuer (anonymous
// for an anonymous admin, also after the proof tap), a reply to the command in its topic, the manual
// punishment kind, and the default sent through Msg.Reply (sendSuccess's call, byte for byte).
func (c *moderationCtx) site() eventSite {
	site := eventSite{
		Bot:     c.Bot,
		Chat:    c.Chat,
		Tr:      c.Tr,
		Actor:   eventActor{User: c.User, Anonymous: actorIsAnonymous(c.Ctx)},
		ReplyTo: c.Msg.MessageId,
		Kind:    deletion.KindBotManual,
		SendDefault: func(text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
			return c.Msg.Reply(c.Bot, text, opts)
		},
	}
	if c.Msg.IsTopicMessage {
		site.ThreadID = c.Msg.MessageThreadId
	}
	return site
}

// postEvent posts one event's reply for any send site and then offers its private copy, through
// postEventSent, and returns the group step's error.
func postEvent(site eventSite, r eventReply) error {
	_, err := postEventSent(site, r)
	return err
}

// postEventSent posts one event's reply for any send site and then offers its private copy, and
// returns what the group step posted. The punishment has already been applied (D-03). It reads the
// group's stored messages once (bounded by eventMessageReadTimeout): a read error is one Warn, the
// default (after the site's admission gate) and no private copy (the switch is unknown, and unknown
// never sends); for an event that is off by default (eventmsg.OffByDefault) a read error posts nothing,
// because its switch is unknown too. Otherwise the site's admission gate is asked when the group
// message or the private copy would be sent (a refusal posts and offers nothing), the group step runs
// (postEventGroup), and then, whatever it did, the private copy is offered (offerReplyPM), which never
// blocks. The error is the group step's.
func postEventSent(site eventSite, r eventReply) (eventGroupPost, error) {
	if eventSupportsAppeal(r) && r.AppealURL == "" {
		r.AppealURL = manualAppealURLFor(site.Chat.Id)
	}
	readCtx, cancel := context.WithTimeout(context.Background(), eventMessageReadTimeout)
	msgs, err := eventMessagesLookup(readCtx, site.Chat.Id)
	cancel()
	if err != nil {
		if eventmsg.OffByDefault(r.Event) {
			log.Warnf("[EventMessage] the messages of chat %d could not be read for %s, nothing was posted", site.Chat.Id, r.Event)
			return eventGroupPost{}, nil
		}
		log.Warnf("[EventMessage] the messages of chat %d could not be read for %s, the default was sent", site.Chat.Id, r.Event)
		if !site.admit() {
			return eventGroupPost{}, nil
		}
		return sendDefaultPost(site, r)
	}
	if site.Admit != nil && (eventGroupWouldPost(r, msgs) || eventPMWouldOffer(site, r, msgs)) && !site.admit() {
		log.Debugf("[EventMessage] the %s message of chat %d for user %d was not admitted, nothing was posted", r.Event, site.Chat.Id, r.Target.Id)
		return eventGroupPost{}, nil
	}
	if msgs.Options.PlainTone && r.PlainDefault != nil {
		if plain := r.PlainDefault(); plain != "" {
			r.Default = plain
		}
	}
	post, err := postEventGroup(site, r, msgs)
	offerReplyPM(site, r, msgs, post)
	return post, err
}

// eventGroupMessage is the group message of a reply as the group step reads it: the resolved row
// (eventmsg's Resolve, so an event that is off by default reads Silent with no row) with its media
// dropped when the reply is text only, and whether the step posts nothing (Silent, unless the reply is
// required).
func eventGroupMessage(r eventReply, msgs *eventmsg.Messages) (row eventmsg.Message, silent bool) {
	row = msgs.Resolve(r.Event, eventmsg.DestGroup)
	if r.TextOnly {
		row.FileID, row.MediaType = "", 0
	}
	return row, row.Silent && !r.Required
}

// eventGroupWouldPost reports whether the group step would post anything for this reply.
func eventGroupWouldPost(r eventReply, msgs *eventmsg.Messages) bool {
	_, silent := eventGroupMessage(r, msgs)
	return !silent
}

// eventPMWouldOffer reports whether offerReplyPM would offer a private copy: an event with a spec whose
// private copy is on, for a target that can receive one.
func eventPMWouldOffer(site eventSite, r eventReply, msgs *eventmsg.Messages) bool {
	if _, ok := eventPMSpecFor(r.Event); !ok {
		return false
	}
	return !msgs.Resolve(r.Event, eventmsg.DestPM).Silent && !pmCopySkipTarget(site.Bot, r.Target.Id)
}

// eventGroupPost is what the group step of a reply posted: the message (nil when nothing was posted) and the
// keyboard rows it was sent with (the bot's own rows of the default, or a custom message's composed rows), which
// the Details button of the private copy keeps (pmCopyAddDetails).
type eventGroupPost struct {
	Msg  *gotgbot.Message
	Rows [][]gotgbot.InlineKeyboardButton
}

// postEventGroup is the group step of an event reply and returns what it posted (no message when
// nothing was posted) with the keyboard rows the message was sent with. It reads the row through
// eventGroupMessage (Resolve, the media dropped for a text-only reply). Silent posts and schedules
// nothing, unless the reply is required; no row, or a row with no content, is the default through
// the site's own send; otherwise the custom message is sent where the site's default goes (site.ReplyTo,
// site.ThreadID) through the site's custom send, and handed to the site's schedule (by default queued
// under site.Kind, the default's kind, D-15). A custom message Telegram
// definitely refuses (eventMessageRefused) is replaced by the default, sent once; any other send error
// is returned, as the default's send error always was.
func postEventGroup(site eventSite, r eventReply, msgs *eventmsg.Messages) (eventGroupPost, error) {
	row, silent := eventGroupMessage(r, msgs)
	if silent {
		log.Debugf("[EventMessage] the %s message of chat %d is silent, nothing was posted", r.Event, site.Chat.Id)
		return eventGroupPost{}, nil
	}
	if !row.HasContent() {
		return sendDefaultPost(site, r)
	}

	msg := composeEventMessage(site.Bot, site.Chat, site.Tr, r, row, site.Actor)
	msg.ReplyMsgID = site.ReplyTo
	msg.ThreadID = site.ThreadID
	sent, err := site.sendCustom(msg)
	if err != nil {
		if eventMessageRefused(err) {
			log.Warnf("[EventMessage] the custom %s message of chat %d was refused, the default was sent", r.Event, site.Chat.Id)
			return sendDefaultPost(site, r)
		}
		return eventGroupPost{}, err
	}
	site.schedule(sent)
	post := eventGroupPost{Msg: sent}
	if msg.Keyboard != nil {
		post.Rows = msg.Keyboard.InlineKeyboard
	}
	return post, nil
}

// sendDefaultPost sends the default reply through the site's own send (today's call, byte for byte
// when no appeal link is set), hands it to the site's schedule (by default queued under site.Kind)
// and returns it with the rows it was sent with. When r.AppealURL is set and fits within Telegram's
// 100-button limit, a shallow copy of r.Opts carries the appeal button row after the bot's own rows
// so r.Opts is never mutated and pmCopyAddDetails preserves the appeal button.
func sendDefaultPost(site eventSite, r eventReply) (eventGroupPost, error) {
	opts := r.Opts
	hasAppeal := false
	if appealRow := eventAppealRow(site.Tr, r.AppealURL); len(appealRow) > 0 {
		baseRows := eventBotRows(r.Opts)
		rows := eventAppendAppealRow(baseRows, appealRow)
		if len(rows) > len(baseRows) {
			var copied gotgbot.SendMessageOpts
			if r.Opts != nil {
				copied = *r.Opts
			}
			copied.ReplyMarkup = gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows}
			opts = &copied
			hasAppeal = true
		}
	}
	sent, err := site.SendDefault(r.Default, opts)
	if err != nil && hasAppeal && eventMessageRefused(err) {
		log.Warnf("[EventMessage] the default %s message of chat %d with appeal button was refused, retrying without appeal button", r.Event, site.Chat.Id)
		opts = r.Opts
		sent, err = site.SendDefault(r.Default, opts)
	}
	if err != nil {
		return eventGroupPost{}, err
	}
	site.schedule(sent)
	return eventGroupPost{Msg: sent, Rows: eventBotRows(opts)}, nil
}

// offerReplyPM offers the private copy of an event reply after the group step (D-11). It offers
// nothing for an event without a spec or a private copy whose switch is off (Resolve: no pm row, or a
// pm row whose switch was never turned on, reads as Silent) and for a target that can never receive one
// (pmCopySkipTarget, checked before the render). The copy is composed here, in the group's language and
// with the site's actor, together with the default private text a refused custom copy falls back
// to, and sent by a job (offerEventPM), so the handler returns at once. The offer also carries the group's
// private copy options, the group message the group step posted (none when it was Silent or failed) with its
// rows, and the Details button's label in the group's language, for the Details button of a copy that is not
// delivered, and the member's first name with the group's translator (only while the log option is on) for its log
// line. ByRule carries site.Automatic, so a reply an automatic rule raised is capped even when its event's spec is
// manual (the blacklist's and the forward blocklist's warns, D-28), while /warn is not.
func offerReplyPM(site eventSite, r eventReply, msgs *eventmsg.Messages, post eventGroupPost) {
	spec, ok := eventPMSpecFor(r.Event)
	if !ok {
		return
	}
	row := msgs.Resolve(r.Event, eventmsg.DestPM)
	if row.Silent {
		return
	}
	if pmCopySkipTarget(site.Bot, r.Target.Id) {
		return
	}
	offer := eventPMOffer{
		Bot:        site.Bot,
		Chat:       *site.Chat,
		Event:      r.Event,
		UserID:     r.Target.Id,
		Message:    composeEventPM(site.Bot, site.Chat, site.Tr, spec, r, row, site.Actor, msgs.Options.PlainTone),
		Custom:     row.HasContent(),
		AppealRow:  eventAppealRow(site.Tr, r.AppealURL),
		Options:    msgs.Options,
		TargetName: r.Target.FirstName,
		ByRule:     site.Automatic,
	}
	if post.Msg != nil {
		offer.GroupMsgID = post.Msg.MessageId
		offer.GroupRows = post.Rows
	}
	if offer.Options.PMDetails {
		offer.DetailsLabel, _ = site.Tr.GetString("eventmsg_btn_details")
	}
	if offer.Options.PMLog {
		offer.Tr = site.Tr
	}
	if offer.Custom {
		offer.Fallback = eventPMDefaultText(site.Tr, spec, site.Chat, r.Values, r.PMNote, msgs.Options.PlainTone)
	}
	offerEventPM(offer)
}
