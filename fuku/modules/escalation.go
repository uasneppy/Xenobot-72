package modules

import (
	"context"
	"errors"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/fuku_robot/fuku/db/escalation"
)

// The escalation ladder's resolver (Phase 26, TUNE-02). The flood, the blacklist and the forward
// blocklist call escalate right before they punish a real user, so a member's 1st, 2nd and later
// offences in one group get the steps of that group's ladder. The counting is one conditional
// upsert in PostgreSQL (escalation.CountOffence), so every replica agrees; this file starts no
// goroutine, keeps no Redis key and adds no shutdown drain.

// escalationCountTimeout bounds the count statement. GORM hands the context to the driver, so the
// bound is enforced; a slow or failing count never stalls the message pipeline for longer.
const escalationCountTimeout = 2 * time.Second

// escalationLadderLookup reads a group's ladder; escalationCount counts one offence. Tests swap
// them to make the ladder unreadable or the count fail.
var (
	escalationLadderLookup = escalation.Get
	escalationCount        = escalation.CountOffence
)

// escalationWarnGates let one Warn line per chat per hour through for each kind of failure.
var escalationWarnGates [2]memberDeleteWarnGate

const (
	escalationWarnLadder = iota
	escalationWarnCount
)

// escalate returns the punishment a member's offence gets: p itself when it is not an effective
// punishment (a warn, delete only or inert action), when the sender is not a real user (an ID of 0
// or less, so a sender chat; a Telegram service account; the bot itself), when the ladder is off
// or has no step, and when the ladder cannot be read; otherwise the ladder's step for the member's
// count, min(count, len(steps)), or step 1 when the count cannot be established (an error or the
// 2 s bound), so a guess never punishes harder than the first step. source is "flood", "blacklist"
// or "fwdblock" and is only logged.
func escalate(ctx context.Context, b *gotgbot.Bot, chatID, userID int64, source string, p autoPunishment) autoPunishment {
	if !autoActionEffective(p.Action, p.Secs) || userID <= 0 || staffServiceUserIDs[userID] || (b != nil && userID == b.Id) {
		return p
	}
	ladder, err := escalationLadderLookup(ctx, chatID)
	if err != nil {
		if escalationWarnGates[escalationWarnLadder].allow(chatID, punishNow()) {
			log.Warnf("[Escalation] the ladder of chat %d could not be read: %s", chatID, escalationErrClass(err))
		}
		return p
	}
	if ladder == nil || !ladder.Enabled || len(ladder.Steps) == 0 {
		return p
	}
	step, count := 0, 0
	countCtx, cancel := context.WithTimeout(ctx, escalationCountTimeout)
	defer cancel()
	count, err = escalationCount(countCtx, chatID, userID, punishNow(), time.Duration(ladder.ForgetDays)*24*time.Hour)
	if err != nil {
		if escalationWarnGates[escalationWarnCount].allow(chatID, punishNow()) {
			log.Warnf("[Escalation] the offence of user %d in chat %d could not be counted, step 1 applies: %s",
				userID, chatID, escalationErrClass(err))
		}
		count = 1
	}
	step = count
	if step < 1 {
		step = 1
	}
	if step > len(ladder.Steps) {
		step = len(ladder.Steps)
	}
	s := ladder.Steps[step-1]
	log.WithFields(log.Fields{"chatId": chatID, "userId": userID, "source": source, "count": count, "step": step}).
		Debug("[Escalation] ladder step chosen")
	return autoPunishment{Action: s.Action, Secs: s.Secs}
}

// escalationErrClass names an error without its text, which can hold a statement or an ID.
func escalationErrClass(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "canceled"
	default:
		return "error"
	}
}
