package modules

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
	"github.com/uasneppy/fuku_robot/fuku/utils/extraction"
)

// The timed automatic punishments (Phase 26, TUNE-01): the warn limit, the flood and the blacklist can
// apply a timed mute (tmute) or a timed ban (tban) with a duration stored beside the action. This file
// is the core the three rules share: the bounds, the one rule that says whether a stored action acts,
// the duration's grammar and wording, the end date and the one helper that sends the request.

const (
	// punishMinSecs is the shortest duration of a timed punishment (models.PunishMinSecs, 60 s, the
	// shortest ParseDurationToken token, 1m).
	punishMinSecs = models.PunishMinSecs
	// punishMaxSecs is the longest duration of a timed punishment (models.PunishMaxSecs, 366 days,
	// Telegram's longest finite restriction). A longer one is refused, never made permanent.
	punishMaxSecs = models.PunishMaxSecs
	// punishMinLead is how far ahead an end date must still be when its request is sent: Telegram
	// treats an until_date less than 30 s away as permanent, plus 5 s for the request's flight (the
	// staff rule's lead, staffUntilMinLead).
	punishMinLead = 35 * time.Second
)

// punishNow is the clock of the timed punishments (their end dates; later the escalation ladder and
// the warn expiry). It is a variable so a test can move time.
var punishNow = time.Now

// errPunishTimeRanOut is returned, with nothing sent, instead of a timed mute or ban whose end date
// cannot be sent: the duration is outside what extraction.TemporaryUntilDate accepts, or the end is
// under punishMinLead away. So a timed punishment can never become a permanent one.
var errPunishTimeRanOut = errors.New("the timed punishment's end date cannot be sent")

// punishTimeRanOutWarn lets one line per chat per hour through when a timed punishment's end date was
// refused locally.
var punishTimeRanOutWarn memberDeleteWarnGate

// autoPunishment is one automatic punishment as a site applies it: the action word (mute, tmute, kick,
// tban or ban), its seconds (0 for an untimed action) and, once applied, the end date in unix seconds
// (0 for none).
type autoPunishment struct {
	Action string
	Secs   int
	Until  int64
}

// Timed reports whether the punishment is a timed mute or ban (tmute or tban), whatever its seconds.
func (p autoPunishment) Timed() bool {
	return p.Action == "tmute" || p.Action == "tban"
}

// punishDurationValid reports whether secs is a duration a timed punishment may store and apply:
// punishMinSecs to punishMaxSecs inclusive, the bounds of models.PunishDurationValid.
func punishDurationValid(secs int) bool {
	return secs >= punishMinSecs && secs <= punishMaxSecs
}

// autoActionEffective is the one rule of what a stored automatic action does (D-08): mute, kick and
// ban act; tmute and tban act only with a valid duration, so a timed mode stored with no duration (0)
// stays inert, exactly as before Phase 26; anything else (warn, none, delete, an empty mode) is not a
// punishment of this kind.
func autoActionEffective(action string, secs int) bool {
	switch action {
	case "mute", "kick", "ban":
		return true
	case "tmute", "tban":
		return punishDurationValid(secs)
	}
	return false
}

// parsePunishDuration reads one duration word with the grammar of /tban, /tmute and the staff
// commands (extraction.ParseDurationToken: <digits><m|h|d|w>, lower case) and returns its seconds
// when it is a matched token with no error within punishMinSecs to punishMaxSecs. Anything else (a
// zero, over 366 days, "1.5h", "2H", "12", "", "-1d") is false.
func parsePunishDuration(word string) (int, bool) {
	spec, matched, err := extraction.ParseDurationToken(word)
	if !matched || err != nil {
		return 0, false
	}
	secs := int(spec.Seconds)
	if !punishDurationValid(secs) {
		return 0, false
	}
	return secs, true
}

// punishDurationUnits are the units of a duration's wording, largest first: the seconds of one unit
// and its short letter (punishDurationLabel reads each unit's locale key by its literal name, so make
// check-translations sees it).
var punishDurationUnits = []struct {
	secs   int
	letter string
}{
	{7 * 24 * 60 * 60, "w"},
	{24 * 60 * 60, "d"},
	{60 * 60, "h"},
	{60, "m"},
}

// punishDurationUnit returns the amount and the unit of secs in the largest unit that divides it evenly
// (minutes, rounded down, for anything that no larger unit divides).
func punishDurationUnit(secs int) (int, int) {
	for i, unit := range punishDurationUnits {
		if secs%unit.secs == 0 || i == len(punishDurationUnits)-1 {
			return secs / unit.secs, i
		}
	}
	return 0, len(punishDurationUnits) - 1
}

// punishDurationLabel is a duration in the translator's language (the group's), built from the
// staff_act_duration_* keys with the largest unit that divides the seconds evenly: "2 год" for 7200
// in Ukrainian, "1 week(s)" for 604800 in English. It is what {duration} of an automatic event shows.
func punishDurationLabel(tr *i18n.Translator, secs int) string {
	amount, unit := punishDurationUnit(secs)
	params := i18n.TranslationParams{"n": amount}
	var text string
	switch punishDurationUnits[unit].letter {
	case "w":
		text, _ = tr.GetString("staff_act_duration_weeks", params)
	case "d":
		text, _ = tr.GetString("staff_act_duration_days", params)
	case "h":
		text, _ = tr.GetString("staff_act_duration_hours", params)
	default:
		text, _ = tr.GetString("staff_act_duration_minutes", params)
	}
	return text
}

// punishDurationShort is a duration in the command grammar, untranslated, with the same largest-unit
// rule: "90m" for 5400, "2h" for 7200, "1d", "1w", "30d" for 2592000. The commands' replies and
// displays show a timed mode as the mode and this form ("tmute 2h").
func punishDurationShort(secs int) string {
	amount, unit := punishDurationUnit(secs)
	return strconv.Itoa(amount) + punishDurationUnits[unit].letter
}

// punishUntil is the end date of a timed punishment of secs seconds from now, in unix seconds, through
// extraction.TemporaryUntilDate: false for a duration under 30 s or over 366 days.
func punishUntil(now time.Time, secs int) (int64, bool) {
	return extraction.TemporaryUntilDate(now.Unix(), int64(secs))
}

// punishUntilTooClose reports whether an end date is under punishMinLead away from punishNow() (or
// past), so the request must not be sent: Telegram would read it as permanent.
func punishUntilTooClose(until int64) bool {
	return until-punishNow().Unix() < int64(punishMinLead/time.Second)
}

// applyTimedPunishment sends a timed mute (tmute: restrictChatMember with MutedPermissions and the
// until_date) or a timed ban (tban: banChatMember with the until_date) of userID in chatID, and returns
// the end date it sent. The end is computed from punishNow() immediately before the request, with no
// pacer or wait in between, so the same value goes into the request and into a message's {until}
// (D-11). When punishUntil refuses the duration or punishUntilTooClose holds, nothing is sent, one
// gated Warn naming the chat is logged and errPunishTimeRanOut is returned. Any other action is a
// programming error: an error and nothing sent. The request takes no context, like the untimed
// punishments: an in-flight request is bounded only by the bot's HTTP client timeout (30 s), never
// by a shorter per-request bound (D-12).
func applyTimedPunishment(b *gotgbot.Bot, chatID, userID int64, action string, secs int) (int64, error) {
	if action != "tmute" && action != "tban" {
		return 0, fmt.Errorf("applyTimedPunishment: %q is not a timed action", action)
	}
	until, ok := punishUntil(punishNow(), secs)
	if !ok || punishUntilTooClose(until) {
		if punishTimeRanOutWarn.allow(chatID, punishNow()) {
			log.Warnf("[Punish] the end date of a timed %s in chat %d cannot be sent; nothing was sent", action, chatID)
		}
		return 0, errPunishTimeRanOut
	}
	var err error
	if action == "tmute" {
		_, err = b.RestrictChatMember(chatID, userID, MutedPermissions, &gotgbot.RestrictChatMemberOpts{UntilDate: until})
	} else {
		_, err = b.BanChatMember(chatID, userID, &gotgbot.BanChatMemberOpts{UntilDate: until})
	}
	if err != nil {
		return 0, err
	}
	return until, nil
}
