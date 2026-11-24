package modules

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/fuku_robot/fuku/db/advthresholds"
)

// advThresholdOutcome is the typed outcome of an advanced threshold apply or reset helper.
type advThresholdOutcome int

const (
	advThresholdSaved advThresholdOutcome = iota
	advThresholdInvalid
	advThresholdFailed
)

const advThresholdsReadTimeout = 2 * time.Second

// advThresholdsLookup reads a chat's cached advanced thresholds; swapped by tests to stage read
// errors or verify hot-path lookup behavior.
var advThresholdsLookup = advthresholds.Get

// advThresholdsWrites holds the repository write functions; swapped by tests to stage write
// failures.
var advThresholdsWrites = struct {
	SetField   func(ctx context.Context, chatID int64, field advthresholds.Field, value int, updatedBy int64) error
	ResetField func(ctx context.Context, chatID int64, field advthresholds.Field, updatedBy int64) error
	ResetArea  func(ctx context.Context, chatID int64, area advthresholds.Area, updatedBy int64) error
	ResetAll   func(ctx context.Context, chatID int64) error
}{
	SetField:   advthresholds.SetField,
	ResetField: advthresholds.ResetField,
	ResetArea:  advthresholds.ResetArea,
	ResetAll:   advthresholds.ResetAll,
}

// advThresholdsFor loads a chat's advanced thresholds under a bounded timeout. On error it logs
// one IDs-only Warn line and returns zero Thresholds{} (compiled defaults) so a transient DB or
// cache fault never disables raid detection, screening, reminders, or captcha (P28-04).
func advThresholdsFor(ctx context.Context, chatID int64) advthresholds.Thresholds {
	if ctx == nil {
		ctx = context.Background()
	}
	readCtx, cancel := context.WithTimeout(ctx, advThresholdsReadTimeout)
	defer cancel()
	th, err := advThresholdsLookup(readCtx, chatID)
	if err != nil {
		log.Warnf("[AdvThresholds] thresholds of chat %d could not be read; using defaults", chatID)
		return advthresholds.Thresholds{}
	}
	return th
}

// advRaidJoinWindow returns the effective join surge window for t, falling back to raidJoinWindow.
func advRaidJoinWindow(t advthresholds.Thresholds) time.Duration {
	if t.RaidJoinWindowSecs > 0 {
		return time.Duration(t.RaidJoinWindowSecs) * time.Second
	}
	return raidJoinWindow
}

// advRaidFloodWindow returns the effective flood window for t, falling back to raidFloodWindow.
func advRaidFloodWindow(t advthresholds.Thresholds) time.Duration {
	if t.RaidFloodWindowSecs > 0 {
		return time.Duration(t.RaidFloodWindowSecs) * time.Second
	}
	return raidFloodWindow
}

// advRaidAIWindow returns the effective AI burst window for t, falling back to raidAIWindow.
func advRaidAIWindow(t advthresholds.Thresholds) time.Duration {
	if t.RaidAIWindowSecs > 0 {
		return time.Duration(t.RaidAIWindowSecs) * time.Second
	}
	return raidAIWindow
}

// advRaidFloodNewPercent returns the effective new-member flood percentage for t.
func advRaidFloodNewPercent(t advthresholds.Thresholds) int {
	if t.RaidFloodNewPercent > 0 {
		return t.RaidFloodNewPercent
	}
	return raidFloodNewPercent
}

// advRaidFloodMinAccounts returns the effective minimum new accounts for flood triggers.
func advRaidFloodMinAccounts(t advthresholds.Thresholds) int {
	if t.RaidFloodMinAccounts > 0 {
		return t.RaidFloodMinAccounts
	}
	return raidFloodMinAccounts
}

// advRaidRecentWindow returns the effective new-member age window for t.
func advRaidRecentWindow(t advthresholds.Thresholds) time.Duration {
	if t.RaidRecentHours > 0 {
		return time.Duration(t.RaidRecentHours) * time.Hour
	}
	return raidRecentWindow
}

// advRaidAIImageBar returns the effective raid AI image spam probability bar for t.
func advRaidAIImageBar(t advthresholds.Thresholds) float64 {
	if t.RaidAIImageBarPct > 0 {
		return float64(t.RaidAIImageBarPct) / 100.0
	}
	return raidAIImageBar
}

// advMemberReqAIBar returns the effective member requirements AI spam-name probability bar for t.
func advMemberReqAIBar(t advthresholds.Thresholds) float64 {
	if t.MemberReqAIBarPct > 0 {
		return float64(t.MemberReqAIBarPct) / 100.0
	}
	return memberReqAIBar
}

// advMemberReqCapPerMin returns the effective member requirements punishment cap per minute,
// falling back to the package variable seam memberReqCapPerMinute when unset.
func advMemberReqCapPerMin(t advthresholds.Thresholds) int {
	if t.MemberReqCapPerMin > 0 {
		return t.MemberReqCapPerMin
	}
	return memberReqCapPerMinute
}

// advMemberReqPause returns the effective member requirements pause duration, falling back to
// the package variable seam memberReqPause when unset.
func advMemberReqPause(t advthresholds.Thresholds) time.Duration {
	if t.MemberReqPauseMins > 0 {
		return time.Duration(t.MemberReqPauseMins) * time.Minute
	}
	return memberReqPause
}

// advLockdownReminderEvery returns the effective lockdown reminder interval, falling back to
// the package variable seam lockdownReminderEvery when unset.
func advLockdownReminderEvery(t advthresholds.Thresholds) time.Duration {
	if t.LockdownReminderMins > 0 {
		return time.Duration(t.LockdownReminderMins) * time.Minute
	}
	return lockdownReminderEvery
}

// advCaptchaTimeout returns the effective captcha timeout in minutes, deferring to base
// (captcha_settings.timeout) when unset.
func advCaptchaTimeout(t advthresholds.Thresholds, base int) int {
	if t.CaptchaTimeoutMins > 0 {
		return t.CaptchaTimeoutMins
	}
	return base
}

// applyAdvancedThreshold validates field and value against safe bounds, normalizes a value equal
// to the item's compiled default to 0 (P28-01), writes via a column-targeted upsert, and logs IDs
// only on failure.
func applyAdvancedThreshold(ctx context.Context, chatID int64, field advthresholds.Field, value int, by int64) advThresholdOutcome {
	spec, ok := advthresholds.SpecFor(field)
	if !ok {
		return advThresholdInvalid
	}
	if value != 0 && (value < spec.Min || value > spec.Max) {
		return advThresholdInvalid
	}
	if spec.Default != 0 && value == spec.Default {
		value = 0
	}
	if err := advThresholdsWrites.SetField(ctx, chatID, field, value, by); err != nil {
		log.Warnf("[AdvThresholds] threshold %s of chat %d could not be saved (by=%d)", field, chatID, by)
		return advThresholdFailed
	}
	return advThresholdSaved
}

// resetAdvancedThreshold resets one threshold field to default (0) and logs IDs only on failure.
func resetAdvancedThreshold(ctx context.Context, chatID int64, field advthresholds.Field, by int64) advThresholdOutcome {
	if _, ok := advthresholds.SpecFor(field); !ok {
		return advThresholdInvalid
	}
	if err := advThresholdsWrites.ResetField(ctx, chatID, field, by); err != nil {
		log.Warnf("[AdvThresholds] threshold %s of chat %d could not be reset (by=%d)", field, chatID, by)
		return advThresholdFailed
	}
	return advThresholdSaved
}

// resetAdvancedArea resets every threshold in area to default (0) and logs IDs only on failure.
func resetAdvancedArea(ctx context.Context, chatID int64, area advthresholds.Area, by int64) advThresholdOutcome {
	if !advthresholds.ValidArea(area) {
		return advThresholdInvalid
	}
	if err := advThresholdsWrites.ResetArea(ctx, chatID, area, by); err != nil {
		log.Warnf("[AdvThresholds] area %s of chat %d could not be reset (by=%d)", area, chatID, by)
		return advThresholdFailed
	}
	return advThresholdSaved
}
