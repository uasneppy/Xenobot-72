// Package eventmsg stores the custom event messages of a group: one message per event (a manual
// punishment such as ban or mute, an automatic action such as a warn or a flood notice, or a screening
// notice such as a failed captcha, a member requirement or an AI spam deletion, or a lockdown's public
// message) and
// destination (the group itself, or the private copy sent to the punished person), each with a text, at most one media item, URL buttons and a Silent switch,
// plus the group's two options of the private copies. A chat's messages and options are read as
// one struct per chat through the cache key event_messages:<chat>, which is listed in skipLocal,
// and every write is a column-targeted upsert that drops that key.
//
// Events and destinations are validated here, never by a database rule, so a later phase adds
// an event or a destination with no migration.
package eventmsg

import (
	"errors"
	"slices"
)

// The events a message can be stored for.
const (
	// EventBan is the reply of a permanent /ban.
	EventBan = "ban"
	// EventTimedBan is the reply of /tban.
	EventTimedBan = "tban"
	// EventKick is the reply of /kick.
	EventKick = "kick"
	// EventMute is the reply of a permanent /mute.
	EventMute = "mute"
	// EventTimedMute is the reply of /tmute.
	EventTimedMute = "tmute"
	// EventUnban is the reply of /unban.
	EventUnban = "unban"
	// EventUnmute is the reply of /unmute.
	EventUnmute = "unmute"
	// EventWarn is the warning posted below the warn limit, for /warn, /dwarn, /swarn and the warns
	// of the blacklist and the forward blocklist.
	EventWarn = "warn"
	// EventWarnLimit is the message posted when a member reaches the warn limit and the limit
	// action (kick, mute or ban) took effect.
	EventWarnLimit = "warnlimit"
	// EventFlood is the notice posted after antiflood muted, kicked or banned a sender.
	EventFlood = "flood"
	// EventBlacklist is the notice posted after a blacklisted word got its sender muted, kicked or
	// banned.
	EventBlacklist = "blacklist"
	// EventForwardBlock is the notice posted after a forward from a blocked source got its sender
	// muted, kicked or banned. It never names the source or the Staff Group.
	EventForwardBlock = "fwdblock"
	// EventCaptcha is the notice posted after a member failed the captcha (the time ran out, or the
	// web check proved a failed solve) and the group's failure action (kick, ban or mute) took effect.
	EventCaptcha = "captcha"
	// EventAISpam is the notice posted after the AI spam filter deleted a message. It is off by
	// default (OffByDefault): a group turns it on by writing its group message's Silent switch false.
	EventAISpam = "aispam"
	// EventReqWarn is the notice of a member requirement warn, including the variants posted when the
	// warn reached the limit and the limit action (kick, ban or mute) took effect.
	EventReqWarn = "mreqwarn"
	// EventReqKick is the notice posted after a member requirement kicked a member.
	EventReqKick = "mreqkick"
	// EventReqBan is the notice posted after a member requirement banned a member.
	EventReqBan = "mreqban"
	// EventReqMute is the notice posted after a member requirement muted a member. It carries the
	// member's "I fixed it" button, so the send path never silences it.
	EventReqMute = "mreqmute"
	// EventLockdown is the reply of a /lockdown that started a lockdown, also after an anonymous admin's
	// proof tap. Every refusal and "already active" answer keeps its own text and is not this event.
	EventLockdown = "lockdown"
)

// The destinations a message can be stored for.
const (
	// DestGroup is the message posted in the group itself.
	DestGroup = "group"
	// DestPM is the private copy of the event sent to the punished person in the bot's private
	// chat. It is off until its Silent switch is written false (see DefaultSilent).
	DestPM = "pm"
)

// Events lists every event a message can be stored for, in the order the settings page shows
// them: the manual punishments, then the automatic actions (Phase 23), then the screening notices
// (Phase 24), then the lockdown messages (Phase 25).
var Events = []string{
	EventBan,
	EventTimedBan,
	EventKick,
	EventMute,
	EventTimedMute,
	EventUnban,
	EventUnmute,
	EventWarn,
	EventWarnLimit,
	EventFlood,
	EventBlacklist,
	EventForwardBlock,
	EventCaptcha,
	EventAISpam,
	EventReqWarn,
	EventReqKick,
	EventReqBan,
	EventReqMute,
	EventLockdown,
}

// Dests lists every destination a message can be stored for.
var Dests = []string{
	DestGroup,
	DestPM,
}

var (
	// ErrUnknownEvent is returned, before any write, for an event that is not in Events.
	ErrUnknownEvent = errors.New("unknown event message event")
	// ErrUnknownDest is returned, before any write, for a destination that is not in Dests.
	ErrUnknownDest = errors.New("unknown event message destination")
)

// ValidEvent reports whether event is one of Events.
func ValidEvent(event string) bool {
	return slices.Contains(Events, event)
}

// ValidDest reports whether dest is one of Dests.
func ValidDest(dest string) bool {
	return slices.Contains(Dests, dest)
}

// DefaultSilent is the Silent switch of a destination that has no stored row. Silent means
// nothing is sent to that destination: the group message is sent by default (false), and a
// private copy is off until its switch is written (true). A part writer that inserts a new row
// stores this value too, so storing a text alone never turns a private copy on.
func DefaultSilent(dest string) bool {
	return dest == DestPM
}

// OffByDefault reports whether an event's group message is off until the group turns it on: true for
// the AI spam notice only, which is new behaviour no group had before Phase 24.
func OffByDefault(event string) bool {
	return event == EventAISpam
}

// DefaultSilentFor is the Silent switch of an event and destination that has no stored row:
// DefaultSilent(dest), or true for the group message of an event that is off by default. Like a
// private copy, such a group message stays off until its switch is written false, and a part writer
// that inserts a new row stores this value, so storing a text alone never turns it on.
func DefaultSilentFor(event, dest string) bool {
	return DefaultSilent(dest) || (dest == DestGroup && OffByDefault(event))
}

// validate checks an event and a destination and returns the matching error for the first that
// is unknown.
func validate(event, dest string) error {
	if !ValidEvent(event) {
		return ErrUnknownEvent
	}
	if !ValidDest(dest) {
		return ErrUnknownDest
	}
	return nil
}
