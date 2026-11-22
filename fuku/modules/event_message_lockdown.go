package modules

import (
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
	"github.com/uasneppy/fuku_robot/fuku/utils/formatting"
)

// The lockdown's public messages (Phase 25) reach the event message engine only through this file.
// TestPMCopyCallers forbids every engine entry in files named lockdown*, raid* and staff*, so the
// lockdown files stay free of the engine and call the postLockdown*Event adapters below instead, the
// way forwardblock_notice.go serves the forward blocklist watcher (D-26). Each adapter is called after
// the lockdown or the lift it announces took effect, from the place today's default is sent, so a
// message never delays, undoes or skips the lock, the snapshot, the worker, the alert or the lift (D-03).
// A lockdown message is never queued for deletion, custom or default (D-08): a lockdown site's Schedule
// does nothing. The lockdown events have no private copy spec, so the engine never offers a copy (D-04).
// The alert, its reminders, the raid-scale removals and their reports, the lift tally, the cannot-lock
// warning and every Staff Group surface never come here (TestEventEngineCallers).

// lockdownFacts are what a lockdown message's default says beside its person, read from the action that
// took effect. For a start: the bot's missing rights (NoDelete, NoInvite), read from the live membership
// startLockdownRow returned.
type lockdownFacts struct {
	NoDelete bool
	NoInvite bool
}

// lockdownStartFacts reads a start's facts from the bot's live membership that startLockdownRow
// returned: a missing delete or invite right never blocks the lock, it only adds a note.
func lockdownStartFacts(bot gotgbot.MergedChatMember) lockdownFacts {
	return lockdownFacts{NoDelete: !bot.CanDeleteMessages, NoInvite: !bot.CanInviteUsers}
}

// lockdownStartNoteLines are the technical notes of a start, in today's order: approved users are muted
// too, then the no-delete note and the no-invite note when the bot lacks that right. The default replies
// and the notes appended after a custom text are built from these lines, so the two never drift.
func lockdownStartNoteLines(tr *i18n.Translator, f lockdownFacts) []string {
	lines := []string{lockdownText(tr, "lockdown_note_approved_muted")}
	if f.NoDelete {
		lines = append(lines, lockdownText(tr, "lockdown_note_no_delete"))
	}
	if f.NoInvite {
		lines = append(lines, lockdownText(tr, "lockdown_note_no_invite"))
	}
	return lines
}

// lockdownStartNotes are the technical notes the bot appends after a custom start text (D-07, D-16),
// joined by line breaks.
func lockdownStartNotes(tr *i18n.Translator, f lockdownFacts) string {
	return lockdownJoinLines(lockdownStartNoteLines(tr, f))
}

// lockdownStartDefaultReply is the default reply of a /lockdown that started a lockdown, byte for byte
// the reply sent before Phase 25: the started line, the approved-muted note, "Locked by" with the
// mention of in.Target (the starter as recorded: the tapper after an anonymous admin's proof), the
// reason line when in.Reason is set, then the no-delete and no-invite notes of in.Lockdown, with
// formatting.Shtml().
func lockdownStartDefaultReply(tr *i18n.Translator, in eventDefaultInput) (string, *gotgbot.SendMessageOpts) {
	notes := lockdownStartNoteLines(tr, in.Lockdown)
	parts := []string{
		lockdownText(tr, "lockdown_started"),
		notes[0],
		lockdownSplice(
			lockdownText(tr, "lockdown_started_by", i18n.TranslationParams{"name": lockdownNameToken}),
			lockdownNameToken, lockdownMention(in.Target.Id, in.Target.FirstName),
		),
	}
	if line := lockdownReasonLine(tr, in.Reason); line != "" {
		parts = append(parts, line)
	}
	parts = append(parts, notes[1:]...)
	return strings.TrimSpace(strings.Join(parts, "\n")), formatting.Shtml()
}

// lockdownEventPerson is the person a lockdown message's person placeholders name (D-13): the starter
// or lifter as the lockdown row records them (the capped name, no last name, no username), or the bot
// itself when the action is not to be named by a person (an automatic lockdown, an anonymous admin),
// never the anonymous admin's proof tapper.
func lockdownEventPerson(b *gotgbot.Bot, userID int64, name string, named bool) gotgbot.User {
	if named {
		return gotgbot.User{Id: userID, FirstName: name}
	}
	return b.User
}

// lockdownEventSite is the eventSite of a lockdown message: the group, its translator, the acting admin,
// the message replied to and the topic (0 for none), whether a trigger raised it, and the site's default
// send, today's exact call. Its Schedule does nothing, because a lockdown message is never queued for
// deletion, custom or default (D-08).
func lockdownEventSite(
	b *gotgbot.Bot, chat *gotgbot.Chat, tr *i18n.Translator, actor eventActor, replyTo, threadID int64, automatic bool,
	sendDefault func(text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error),
) eventSite {
	return eventSite{
		Bot:         b,
		Chat:        chat,
		Tr:          tr,
		Actor:       actor,
		ReplyTo:     replyTo,
		ThreadID:    threadID,
		Automatic:   automatic,
		SendDefault: sendDefault,
		Schedule:    func(*gotgbot.Message) {},
	}
}

// postLockdownStartEvent posts the reply of a /lockdown whose startLockdownRow answered
// lockdownStartStarted (EVT-06): the group's stored lockdown message, or the default
// (lockdownStartDefaultReply) when none is stored. It goes where today's reply goes, as a reply to the
// command in its topic. {admin} is the starter's recorded name, or "an admin" for an anonymous admin
// (the proof tap's mark); the person placeholders name the starter, or the bot for an anonymous admin
// (D-13); {chatname} is the group's title and {reason} the stored reason. A custom text is followed,
// after one line break, by the technical notes (lockdownStartNotes). Silent posts nothing; the lockdown
// stands either way.
//
// The default is today's msg.Reply with formatting.Shtml(), not paced, as before. A custom message is
// sent with media.SendGreetingMessage, which takes no context, so its request is bounded only by the
// bot's HTTP client timeout. A send error is logged with today's line and changes nothing else.
func postLockdownStartEvent(
	b *gotgbot.Bot, ctx *ext.Context, msg *gotgbot.Message, tr *i18n.Translator, actor *gotgbot.User, name string,
	row *models.ChatLockdown, bot gotgbot.MergedChatMember,
) {
	chat := ctx.EffectiveChat
	if chat == nil {
		chat = &msg.Chat
	}
	anonymous := actorIsAnonymous(ctx)
	facts := lockdownStartFacts(bot)
	starter := gotgbot.User{Id: actor.Id, FirstName: name}
	text, opts := lockdownStartDefaultReply(tr, eventDefaultInput{Target: starter, Reason: row.Reason, Lockdown: facts})
	var threadID int64
	if msg.IsTopicMessage {
		threadID = msg.MessageThreadId
	}
	site := lockdownEventSite(b, chat, tr, eventActor{User: &starter, Anonymous: anonymous}, msg.MessageId, threadID, false,
		func(text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
			if text == "" {
				return nil, nil
			}
			return msg.Reply(b, text, opts)
		})
	err := postEvent(site, eventReply{
		Event:   eventmsg.EventLockdown,
		Target:  lockdownEventPerson(b, actor.Id, name, !anonymous),
		Values:  eventValues{Reason: row.Reason},
		Default: text,
		Opts:    opts,
		Note:    lockdownStartNotes(tr, facts),
	})
	if err != nil {
		log.Errorf("[Lockdown] reply: %v", err)
	}
}
