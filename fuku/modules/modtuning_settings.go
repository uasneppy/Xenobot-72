package modules

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/fuku_robot/fuku/db/modtuning"
)

// modTuningOutcome is the typed outcome of a manual command tuning apply helper.
type modTuningOutcome int

const (
	modTuningSaved modTuningOutcome = iota
	modTuningInvalid
	modTuningFailed
)

const manualDefaultsReadTimeout = 2 * time.Second

// errManualReasonRequired is returned internally when a manual punishment command is refused for
// missing a required reason after the refusal reply has already been sent.
var errManualReasonRequired = errors.New("manual punishment reason required")

// manualDefaultsLookup reads a chat's manual defaults; swapped by tests to stage read failures.
var manualDefaultsLookup = modtuning.GetDefaults

// modTuningWrites holds the repository write functions; swapped by tests to stage write failures.
var modTuningWrites = struct {
	SetDefaultBanSecs  func(ctx context.Context, chatID int64, secs int, by int64) error
	SetDefaultMuteSecs func(ctx context.Context, chatID int64, secs int, by int64) error
	SetReasonRequired  func(ctx context.Context, chatID int64, on bool, by int64) error
}{
	SetDefaultBanSecs:  modtuning.SetDefaultBanSecs,
	SetDefaultMuteSecs: modtuning.SetDefaultMuteSecs,
	SetReasonRequired:  modtuning.SetReasonRequired,
}

// applyDefaultBanSecs validates and stores a chat's default manual ban duration (0 = permanent, or
// a valid timed punishment duration). Logs IDs only, never user text.
func applyDefaultBanSecs(ctx context.Context, chatID int64, secs int, by int64) modTuningOutcome {
	if secs != 0 && !punishDurationValid(secs) {
		return modTuningInvalid
	}
	if err := modTuningWrites.SetDefaultBanSecs(ctx, chatID, secs, by); err != nil {
		log.Warnf("[ModTuning] default ban duration of chat %d could not be saved (by=%d)", chatID, by)
		return modTuningFailed
	}
	return modTuningSaved
}

// applyDefaultMuteSecs validates and stores a chat's default manual mute duration (0 = permanent,
// or a valid timed punishment duration). Logs IDs only, never user text.
func applyDefaultMuteSecs(ctx context.Context, chatID int64, secs int, by int64) modTuningOutcome {
	if secs != 0 && !punishDurationValid(secs) {
		return modTuningInvalid
	}
	if err := modTuningWrites.SetDefaultMuteSecs(ctx, chatID, secs, by); err != nil {
		log.Warnf("[ModTuning] default mute duration of chat %d could not be saved (by=%d)", chatID, by)
		return modTuningFailed
	}
	return modTuningSaved
}

// applyReasonRequired stores whether manual moderation commands require a reason. Logs IDs only.
func applyReasonRequired(ctx context.Context, chatID int64, on bool, by int64) modTuningOutcome {
	if err := modTuningWrites.SetReasonRequired(ctx, chatID, on, by); err != nil {
		log.Warnf("[ModTuning] reason-required setting of chat %d could not be saved (by=%d)", chatID, by)
		return modTuningFailed
	}
	return modTuningSaved
}

// manualDefaultsFor loads a chat's manual defaults under a bounded timeout. On error it logs one
// Warn naming only the chat ID and fails open to zero defaults so an outage never blocks moderation.
func manualDefaultsFor(chatID int64) modtuning.Defaults {
	ctx, cancel := context.WithTimeout(context.Background(), manualDefaultsReadTimeout)
	defer cancel()
	defs, err := manualDefaultsLookup(ctx, chatID)
	if err != nil {
		log.Warnf("[ModTuning] manual defaults of chat %d could not be read; failing open", chatID)
		return modtuning.Defaults{}
	}
	return defs
}

// manualDefaultUntil computes the Telegram until_date for a configured default duration, guarded by
// punishDurationValid and punishUntilTooClose so Telegram never reads a too-close expiry as
// permanent. Returns 0, false when unset or invalid.
func manualDefaultUntil(secs int) (int64, bool) {
	if secs == 0 || !punishDurationValid(secs) {
		return 0, false
	}
	until, ok := punishUntil(punishNow(), secs)
	if !ok || punishUntilTooClose(until) {
		return 0, false
	}
	return until, true
}

// manualApplyReasonRequired refuses a manual punishment command when reason_required is enabled
// and reason is blank. The refusal is sent via replyValidationError (never queued under
// bot:punish_manual) and returns true so the caller exits without executing the punishment.
func manualApplyReasonRequired(c *moderationCtx, reason string, defs modtuning.Defaults) bool {
	if !defs.ReasonRequired || strings.TrimSpace(reason) != "" {
		return false
	}
	text, _ := c.Tr.GetString("moddef_reason_required")
	_ = replyValidationError(c, text)
	return true
}

// manualBanMember executes a manual user ban, applying the chat's default ban duration when set
// (P27-01) and populating t.untilDate and t.timeVal for EventTimedBan.
func manualBanMember(c *moderationCtx, t *target) error {
	defs := c.manualDefaults()
	if defs.DefaultBanSecs > 0 {
		if until, ok := manualDefaultUntil(defs.DefaultBanSecs); ok {
			if _, err := c.Chat.BanMember(c.Bot, t.userID, &gotgbot.BanChatMemberOpts{UntilDate: until}); err != nil {
				return err
			}
			t.untilDate = until
			t.timeVal = punishDurationLabel(c.Tr, defs.DefaultBanSecs)
			return nil
		}
	}
	_, err := c.Chat.BanMember(c.Bot, t.userID, nil)
	return err
}

// manualMuteMember executes a manual user mute, applying the chat's default mute duration when set
// (P27-01) and populating t.untilDate and t.timeVal for EventTimedMute.
func manualMuteMember(c *moderationCtx, t *target) error {
	defs := c.manualDefaults()
	if defs.DefaultMuteSecs > 0 {
		if until, ok := manualDefaultUntil(defs.DefaultMuteSecs); ok {
			if _, err := c.Chat.RestrictMember(c.Bot, t.userID, MutedPermissions, &gotgbot.RestrictChatMemberOpts{UntilDate: until}); err != nil {
				return err
			}
			t.untilDate = until
			t.timeVal = punishDurationLabel(c.Tr, defs.DefaultMuteSecs)
			return nil
		}
	}
	_, err := c.Chat.RestrictMember(c.Bot, t.userID, MutedPermissions, nil)
	return err
}
