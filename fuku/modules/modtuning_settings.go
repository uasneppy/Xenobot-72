package modules

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/db/modtuning"
)

// modTuningOutcome is the typed outcome of a manual command tuning apply helper.
type modTuningOutcome int

const (
	modTuningSaved modTuningOutcome = iota
	modTuningInvalid
	modTuningFailed
)

// reasonWordOutcome classifies a normalized reason shortcut word.
type reasonWordOutcome int

const (
	reasonWordOK reasonWordOutcome = iota
	reasonWordBlank
	reasonWordMultiWord
	reasonWordLeadingSlash
	reasonWordTooLong
)

// reasonShortcutAddOutcome is the typed outcome of applyReasonShortcutAdd.
type reasonShortcutAddOutcome int

const (
	reasonShortcutSaved reasonShortcutAddOutcome = iota
	reasonShortcutExists
	reasonShortcutCap
	reasonShortcutInvalid
	reasonShortcutFailed
)

// reasonShortcutRemoveOutcome is the typed outcome of applyReasonShortcutRemoveByID.
type reasonShortcutRemoveOutcome int

const (
	reasonShortcutRemoved reasonShortcutRemoveOutcome = iota
	reasonShortcutGone
	reasonShortcutRemoveFailed
)

const manualDefaultsReadTimeout = 2 * time.Second

// errManualReasonRequired is returned internally when a manual punishment command is refused for
// missing a required reason after the refusal reply has already been sent.
var errManualReasonRequired = errors.New("manual punishment reason required")

// manualDefaultsLookup reads a chat's manual defaults; swapped by tests to stage read failures.
var manualDefaultsLookup = modtuning.GetDefaults

// manualShortcutsLookup reads a chat's reason shortcuts map; swapped by tests to stage read
// failures or assert zero lookups on empty/multi-word reasons.
var manualShortcutsLookup = modtuning.GetShortcuts

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

// reasonShortcutWrites holds the shortcut repository operations; swapped by tests to stage failures.
var reasonShortcutWrites = struct {
	CountFresh  func(ctx context.Context, chatID int64) (int, error)
	ExistsFresh func(ctx context.Context, chatID int64, word string) (bool, error)
	Add         func(ctx context.Context, chatID int64, word, reason string, by int64) (bool, error)
	RemoveByID  func(ctx context.Context, chatID, id int64) (string, bool, error)
}{
	CountFresh:  modtuning.CountShortcutsFresh,
	ExistsFresh: modtuning.ExistsShortcutFresh,
	Add:         modtuning.AddShortcut,
	RemoveByID:  modtuning.RemoveShortcutByID,
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

// normaliseReasonShortcutWord trims Unicode whitespace and lower-cases a shortcut word (TUNE-05,
// P27-03).
func normaliseReasonShortcutWord(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// checkReasonShortcutWord validates an already-normalized reason shortcut word: non-empty, single
// word with no whitespace, no leading '/', and at most models.ReasonShortcutWordMaxRunes (32) runes.
func checkReasonShortcutWord(word string) reasonWordOutcome {
	if word == "" {
		return reasonWordBlank
	}
	if strings.IndexFunc(word, unicode.IsSpace) >= 0 {
		return reasonWordMultiWord
	}
	if strings.HasPrefix(word, "/") {
		return reasonWordLeadingSlash
	}
	if utf8.RuneCountInString(word) > models.ReasonShortcutWordMaxRunes {
		return reasonWordTooLong
	}
	return reasonWordOK
}

// checkReasonShortcutText trims raw and checks that it is non-empty and at most
// models.ReasonShortcutTextMaxRunes (500) runes.
func checkReasonShortcutText(raw string) (trimmed string, ok bool, tooLong bool) {
	trimmed = strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false, false
	}
	if utf8.RuneCountInString(trimmed) > models.ReasonShortcutTextMaxRunes {
		return "", false, true
	}
	return trimmed, true, false
}

// applyReasonShortcutAdd validates word and text, checks fresh duplicate and per-chat cap (50), and
// inserts the shortcut with ON CONFLICT DO NOTHING so an existing word is never overwritten. Logs
// IDs only, never user text.
func applyReasonShortcutAdd(ctx context.Context, chatID int64, word, text string, by int64) reasonShortcutAddOutcome {
	norm := normaliseReasonShortcutWord(word)
	if checkReasonShortcutWord(norm) != reasonWordOK {
		return reasonShortcutInvalid
	}
	trimmed, ok, _ := checkReasonShortcutText(text)
	if !ok {
		return reasonShortcutInvalid
	}
	exists, err := reasonShortcutWrites.ExistsFresh(ctx, chatID, norm)
	if err != nil {
		log.Warnf("[ModTuning] reason shortcut existence check for chat %d failed (by=%d)", chatID, by)
		return reasonShortcutFailed
	}
	if exists {
		return reasonShortcutExists
	}
	count, err := reasonShortcutWrites.CountFresh(ctx, chatID)
	if err != nil {
		log.Warnf("[ModTuning] reason shortcut count for chat %d failed (by=%d)", chatID, by)
		return reasonShortcutFailed
	}
	if count >= models.ReasonShortcutsCapMax {
		return reasonShortcutCap
	}
	added, err := reasonShortcutWrites.Add(ctx, chatID, norm, trimmed, by)
	if err != nil {
		log.Warnf("[ModTuning] reason shortcut for chat %d could not be saved (by=%d)", chatID, by)
		return reasonShortcutFailed
	}
	if !added {
		return reasonShortcutExists
	}
	return reasonShortcutSaved
}

// applyReasonShortcutRemoveByID deletes one reason shortcut scoped to chatID, returning the stored
// word (for the UI notice) and typed outcome. Logs IDs only, never user text.
func applyReasonShortcutRemoveByID(ctx context.Context, chatID, id int64) (string, reasonShortcutRemoveOutcome) {
	word, removed, err := reasonShortcutWrites.RemoveByID(ctx, chatID, id)
	if err != nil {
		log.Warnf("[ModTuning] reason shortcut %d of chat %d could not be removed", id, chatID)
		return "", reasonShortcutRemoveFailed
	}
	if !removed {
		return "", reasonShortcutGone
	}
	return word, reasonShortcutRemoved
}

// expandManualReason expands a single-word manual punishment reason matching a saved shortcut in
// chatID (case-insensitive) to its stored full plain text. Empty or multi-word reasons return
// immediately without touching DB or cache. Read errors fail open with one IDs-only Warn.
func expandManualReason(chatID int64, reason string) string {
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" || strings.IndexFunc(trimmed, unicode.IsSpace) >= 0 {
		return reason
	}
	ctx, cancel := context.WithTimeout(context.Background(), manualDefaultsReadTimeout)
	defer cancel()
	shortcuts, err := manualShortcutsLookup(ctx, chatID)
	if err != nil {
		log.Warnf("[ModTuning] reason shortcuts of chat %d could not be read; failing open", chatID)
		return reason
	}
	if full := shortcuts[strings.ToLower(trimmed)]; full != "" {
		return full
	}
	return reason
}
