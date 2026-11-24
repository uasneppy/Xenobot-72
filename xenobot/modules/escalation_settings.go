package modules

import (
	"context"
	"errors"

	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/db/escalation"
)

// The escalation ladder's apply functions (Phase 26, TUNE-02, D-27, D-31, D-32): one write path per
// control of the Escalation page. There is no command, so the page is the only door. None of them
// judges a person's rights (the page does that before it calls), sends anything, logs a name or
// starts a goroutine. Each reads the ladder fresh, checks its rule against that read, writes through
// escalationWrites and returns the ladder read back fresh, or the read before on a refusal or a
// failed save.

// escalationApplyOutcome is what an apply function did.
type escalationApplyOutcome int

const (
	// escalationApplied means the ladder holds the requested value.
	escalationApplied escalationApplyOutcome = iota
	// escalationExampleAdded means the ladder was turned on with no step, so the example ladder was stored.
	escalationExampleAdded
	// escalationStale means the screen the press came from no longer matches the stored ladder (another
	// admin changed it); nothing was written.
	escalationStale
	// escalationLastStepRefused means the only step of a ladder that is on was not removed.
	escalationLastStepRefused
	// escalationInvalid means the step, index or value is not a stored one; nothing was written.
	escalationInvalid
	// escalationSaveFailed means a read or a write failed; the stored ladder is unchanged.
	escalationSaveFailed
)

// escalationApplyResult is the typed result of an apply function. Ladder is the ladder read back
// after a write, or the one read before a refusal; nil only when no read succeeded.
type escalationApplyResult struct {
	Outcome escalationApplyOutcome
	Ladder  *escalation.Ladder
	Err     error
}

// escalationExampleSteps is the ladder a first turn-on stores: a timed mute of an hour, a timed mute
// of a day, then a ban for every later offence.
var escalationExampleSteps = []escalation.Step{{Action: "tmute", Secs: 3600}, {Action: "tmute", Secs: 86400}, {Action: "ban"}}

// escalationForgetPresets are the forget periods the page offers, in days.
var escalationForgetPresets = []int{1, 7, 30, 90}

// escalationWrites stores the ladder. Its fields are variables only so a test can make a write fail.
var escalationWrites = struct {
	SetEnabledChecked     func(ctx context.Context, chatID int64, on bool, exampleSteps []escalation.Step, by int64) (*escalation.Ladder, bool, error)
	SetStepChecked        func(ctx context.Context, chatID int64, n int, s escalation.Step, by int64) (*escalation.Ladder, error)
	RemoveLastStepChecked func(ctx context.Context, chatID int64, n int, by int64) (*escalation.Ladder, error)
	SetForgetDays         func(ctx context.Context, chatID int64, days int, by int64) error
	ClearOffences         func(ctx context.Context, chatID int64) error
}{
	SetEnabledChecked:     escalation.SetEnabledChecked,
	SetStepChecked:        escalation.SetStepChecked,
	RemoveLastStepChecked: escalation.RemoveLastStepChecked,
	SetForgetDays:         escalation.SetForgetDays,
	ClearOffences:         escalation.ClearOffences,
}

// escalationRead reads the ladder fresh; a variable so a test can make it fail.
var escalationRead = escalation.GetFresh

func escalationForgetPreset(days int) bool {
	for _, preset := range escalationForgetPresets {
		if preset == days {
			return true
		}
	}
	return false
}

// escalationBefore reads the ladder before a write; a failed read is a failed save with nothing written.
func escalationBefore(ctx context.Context, chatID int64) (*escalation.Ladder, *escalationApplyResult) {
	before, err := escalationRead(ctx, chatID)
	if err != nil || before == nil {
		log.Warnf("[Escalation] the ladder of chat %d could not be read before a change", chatID)
		return nil, &escalationApplyResult{Outcome: escalationSaveFailed, Err: err}
	}
	return before, nil
}

// escalationFinish reads the ladder back after a successful write. A failed read-back leaves Ladder nil
// (the write landed).
func escalationFinish(ctx context.Context, chatID int64, outcome escalationApplyOutcome) escalationApplyResult {
	after, err := escalationRead(ctx, chatID)
	if err != nil {
		after = nil
	}
	return escalationApplyResult{Outcome: outcome, Ladder: after}
}

// escalationFailed logs a failed write by chat ID and returns the save failure with the ladder before it.
func escalationFailed(ctx context.Context, chatID int64, before *escalation.Ladder, err error) escalationApplyResult {
	log.Warnf("[Escalation] the ladder of chat %d could not be saved", chatID)
	if before == nil {
		if read, readErr := escalationRead(ctx, chatID); readErr == nil {
			before = read
		}
	}
	return escalationApplyResult{Outcome: escalationSaveFailed, Ladder: before, Err: err}
}

// applyEscalationEnabled turns the ladder on or off inside a single locked transaction. Turning it
// on with no step stores the example ladder (escalationExampleAdded); turning it on with steps
// stores only the switch. Turning it off stores the switch and deletes the group's offence counts,
// so turning it on again starts fresh; a failed delete is logged and does not undo the switch,
// because counts left behind age out by comparison (D-27).
func applyEscalationEnabled(ctx context.Context, chatID int64, on bool, by int64) escalationApplyResult {
	after, exampleAdded, err := escalationWrites.SetEnabledChecked(ctx, chatID, on, escalationExampleSteps, by)
	if err != nil {
		return escalationFailed(ctx, chatID, nil, err)
	}
	if !on {
		if err := escalationWrites.ClearOffences(ctx, chatID); err != nil {
			log.Warnf("[Escalation] the offence counts of chat %d could not be cleared", chatID)
		}
	}
	if exampleAdded {
		return escalationApplyResult{Outcome: escalationExampleAdded, Ladder: after}
	}
	return escalationApplyResult{Outcome: escalationApplied, Ladder: after}
}

// applyEscalationStep stores step n (1-based) inside a single locked transaction: it replaces an
// existing step or appends the next one, so n may be at most len(steps)+1; a larger n is stale. An
// invalid step or an n outside 1 to escalation.MaxSteps is invalid; both write nothing.
func applyEscalationStep(ctx context.Context, chatID int64, n int, s escalation.Step, by int64) escalationApplyResult {
	if n < 1 || n > escalation.MaxSteps || !escalation.ValidStep(s) {
		return escalationApplyResult{Outcome: escalationInvalid}
	}
	ladder, err := escalationWrites.SetStepChecked(ctx, chatID, n, s, by)
	if errors.Is(err, escalation.ErrStaleStep) {
		return escalationApplyResult{Outcome: escalationStale, Ladder: ladder}
	}
	if err != nil {
		return escalationFailed(ctx, chatID, ladder, err)
	}
	return escalationApplyResult{Outcome: escalationApplied, Ladder: ladder}
}

// applyEscalationRemoveStep clears the last step inside a single locked transaction: n must equal
// len(steps), else the screen is stale. The only step of a ladder that is on is refused; with the
// ladder off it is cleared.
func applyEscalationRemoveStep(ctx context.Context, chatID int64, n int, by int64) escalationApplyResult {
	if n < 1 || n > escalation.MaxSteps {
		return escalationApplyResult{Outcome: escalationInvalid}
	}
	ladder, err := escalationWrites.RemoveLastStepChecked(ctx, chatID, n, by)
	if errors.Is(err, escalation.ErrStaleStep) {
		return escalationApplyResult{Outcome: escalationStale, Ladder: ladder}
	}
	if errors.Is(err, escalation.ErrLastStepEnabled) {
		return escalationApplyResult{Outcome: escalationLastStepRefused, Ladder: ladder}
	}
	if err != nil {
		return escalationFailed(ctx, chatID, ladder, err)
	}
	return escalationApplyResult{Outcome: escalationApplied, Ladder: ladder}
}

// applyEscalationForget stores how many days a member's offences are remembered: one of
// escalationForgetPresets; anything else is invalid and writes nothing.
func applyEscalationForget(ctx context.Context, chatID int64, days int, by int64) escalationApplyResult {
	if !escalationForgetPreset(days) {
		return escalationApplyResult{Outcome: escalationInvalid}
	}
	before, refused := escalationBefore(ctx, chatID)
	if refused != nil {
		return *refused
	}
	if err := escalationWrites.SetForgetDays(ctx, chatID, days, by); err != nil {
		return escalationFailed(ctx, chatID, before, err)
	}
	return escalationFinish(ctx, chatID, escalationApplied)
}
