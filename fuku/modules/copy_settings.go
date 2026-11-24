package modules

import (
	"context"
	"errors"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/advthresholds"
	"github.com/uasneppy/fuku_robot/fuku/db/antiflood"
	"github.com/uasneppy/fuku_robot/fuku/db/blacklists"
	"github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/escalation"
	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/db/modtuning"
	"github.com/uasneppy/fuku_robot/fuku/db/warns"
)

// copySettingsOutcome classifies the result of applyCopySettings.
type copySettingsOutcome int

const (
	copySettingsSaved copySettingsOutcome = iota
	copySettingsFailed
)

// copySettingsResult reports the outcome of one applyCopySettings run.
type copySettingsResult struct {
	Outcome         copySettingsOutcome
	MessagesWritten int
	MessagesSkipped int
	Err             error
}

// copySettingsSnapshot holds a fresh, non-creating read of all 6 Copy Settings areas from a
// source group before any target write begins.
type copySettingsSnapshot struct {
	EventMessages       []eventmsg.Message
	EventOptions        eventmsg.Options
	WarnMode            string
	WarnModeSecs        int
	WarnExpiryDays      int
	FloodAction         string
	FloodActionSecs     int
	BlacklistAction     string
	BlacklistActionSecs int
	Escalation          escalation.Ladder
	ManualDefaults      modtuning.Defaults
	ReasonShortcuts     []modtuning.ShortcutEntry
	AdvThresholds       advthresholds.Thresholds
}

var (
	errCopySameChat   = errors.New("copy settings: source and target chats must differ")
	errCopyNoDatabase = errors.New("copy settings: database not initialized")
)

// loadCopyWarnSettingsFresh reads a source chat's warn_mode, warn_mode_secs, and warn_expiry_days
// directly from the database without using the cache and without creating a row.
func loadCopyWarnSettingsFresh(ctx context.Context, chatID int64) (mode string, secs, expiryDays int, err error) {
	if db.DB == nil {
		return "", 0, 0, errCopyNoDatabase
	}
	var row models.WarnSettings
	err = db.DB.WithContext(ctx).Where("chat_id = ?", chatID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "mute", 0, 0, nil
	}
	if err != nil {
		return "", 0, 0, err
	}
	mode = strings.ToLower(row.WarnMode)
	secs = row.WarnModeSecs
	if mode == "" {
		mode = "mute"
		secs = 0
	}
	return mode, secs, row.ExpiryDays, nil
}

// readCopySettingsSnapshot reads all 6 tuning and message areas from srcChatID fresh without
// creating any rows on the source group.
func readCopySettingsSnapshot(ctx context.Context, srcChatID int64) (copySettingsSnapshot, string, error) {
	var snap copySettingsSnapshot

	srcMsgs, err := eventmsg.GetFresh(ctx, srcChatID)
	if err != nil {
		return snap, "read_event_messages", err
	}
	keys := make([]string, 0, len(srcMsgs.Rows))
	for k := range srcMsgs.Rows {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	snap.EventMessages = make([]eventmsg.Message, 0, len(keys))
	for _, k := range keys {
		snap.EventMessages = append(snap.EventMessages, srcMsgs.Rows[k])
	}
	snap.EventOptions = srcMsgs.Options

	snap.WarnMode, snap.WarnModeSecs, snap.WarnExpiryDays, err = loadCopyWarnSettingsFresh(ctx, srcChatID)
	if err != nil {
		return snap, "read_warn_settings", err
	}

	srcFlood, _, err := antiflood.GetFloodFreshContext(ctx, srcChatID)
	if err != nil {
		return snap, "read_antiflood", err
	}
	snap.FloodAction = strings.ToLower(srcFlood.Action)
	snap.FloodActionSecs = srcFlood.ActionSecs
	if snap.FloodAction == "" {
		snap.FloodAction = "mute"
		snap.FloodActionSecs = 0
	}

	blRows, err := blacklists.ListBlacklistEntriesFresh(ctx, srcChatID)
	if err != nil {
		return snap, "read_blacklists", err
	}
	if len(blRows) > 0 && blRows[0].Action != "" {
		snap.BlacklistAction = strings.ToLower(blRows[0].Action)
		snap.BlacklistActionSecs = blRows[0].ActionSecs
	} else {
		snap.BlacklistAction = "warn"
		snap.BlacklistActionSecs = 0
	}

	srcLadder, err := escalation.GetFresh(ctx, srcChatID)
	if err != nil {
		return snap, "read_escalation", err
	}
	snap.Escalation = *srcLadder

	snap.ManualDefaults, err = modtuning.GetDefaultsFresh(ctx, srcChatID)
	if err != nil {
		return snap, "read_manual_defaults", err
	}
	if snap.ManualDefaults.AppealURL != "" {
		if cleaned, ok := validateAppealURL(snap.ManualDefaults.AppealURL); ok {
			snap.ManualDefaults.AppealURL = cleaned
		} else {
			snap.ManualDefaults.AppealURL = ""
		}
	}

	snap.ReasonShortcuts, err = modtuning.ListShortcutsFresh(ctx, srcChatID)
	if err != nil {
		return snap, "read_reason_shortcuts", err
	}

	snap.AdvThresholds, err = advthresholds.GetFresh(ctx, srcChatID)
	if err != nil {
		return snap, "read_advthresholds", err
	}

	return snap, "", nil
}

// writeCopySettingsTx applies all 9 target table writes inside one caller-supplied transaction so
// any mid-way failure rolls back every earlier table cleanly.
func writeCopySettingsTx(tx *gorm.DB, dstChatID, by int64, snap copySettingsSnapshot) (eventmsg.WriteReport, string, error) {
	rep, err := eventmsg.ReplaceAllValidatedTx(tx, dstChatID, snap.EventMessages, snap.EventOptions, by)
	if err != nil {
		return rep, "write_event_messages", err
	}
	if err := warns.SetWarnModeTx(tx, dstChatID, snap.WarnMode, snap.WarnModeSecs); err != nil {
		return rep, "write_warn_mode", err
	}
	if err := warns.SetWarnExpiryTx(tx, dstChatID, snap.WarnExpiryDays); err != nil {
		return rep, "write_warn_expiry", err
	}
	if err := antiflood.SetFloodActionPreserveOffTx(tx, dstChatID, snap.FloodAction, snap.FloodActionSecs); err != nil {
		return rep, "write_antiflood", err
	}
	if _, err := blacklists.SetBlacklistActionTimedTx(tx, dstChatID, snap.BlacklistAction, snap.BlacklistActionSecs); err != nil {
		return rep, "write_blacklist_action", err
	}
	if err := escalation.ReplaceLadderTx(tx, dstChatID, snap.Escalation, by); err != nil {
		return rep, "write_escalation", err
	}
	if err := modtuning.ReplaceDefaultsTx(tx, dstChatID, snap.ManualDefaults, by); err != nil {
		return rep, "write_manual_defaults", err
	}
	if err := modtuning.ReplaceShortcutsTx(tx, dstChatID, snap.ReasonShortcuts, by); err != nil {
		return rep, "write_reason_shortcuts", err
	}
	if err := advthresholds.ReplaceAllTx(tx, dstChatID, snap.AdvThresholds, by); err != nil {
		return rep, "write_advthresholds", err
	}
	return rep, "", nil
}

// dropCopySettingsTargetCaches invalidates every target cache key touched by applyCopySettings.
// Called only after the enclosing transaction commits.
func dropCopySettingsTargetCaches(dstChatID int64) {
	cache.DeleteCache(eventmsg.CacheKey(dstChatID))
	cache.DeleteCache(cache.CacheKey("warn_settings", dstChatID))
	cache.DeleteCache(cache.CacheKey("antiflood", dstChatID))
	cache.DeleteCache(cache.CacheKey("blacklist", dstChatID))
	cache.DeleteCache(escalation.CacheKey(dstChatID))
	cache.DeleteCache(modtuning.DefaultsCacheKey(dstChatID))
	cache.DeleteCache(modtuning.ShortcutsCacheKey(dstChatID))
	cache.DeleteCache(advthresholds.CacheKey(dstChatID))
}

// applyCopySettings performs an atomic, one-time full-mirror copy of all 6 named tuning and
// message areas across 9 tables from srcChatID to dstChatID (P29-01, P29-03, P29-04):
//  1. chat_event_messages + chat_event_message_options (via eventmsg.ReplaceAllValidatedTx)
//  2. warns_settings.{warn_mode, warn_mode_secs, warn_expiry_days} (preserving target warn_limit)
//  3. antiflood_settings.{action, action_secs} (preserving target flood_limit including 0)
//  4. blacklists.{action, action_secs} on existing target rows only (never copying words)
//  5. chat_escalation (full ladder replace)
//  6. chat_manual_defaults + chat_reason_shortcuts (full replace; appeal_url re-validated)
//  7. chat_advanced_thresholds (full replace)
//
// All 9 writes execute in a single database transaction so a mid-way failure rolls back all
// target tables cleanly and drops no cache keys. On commit it drops every target cache key,
// leaves srcChatID completely untouched, creates no ongoing link, and never touches excluded
// tables (Staff Group links, approvals, warns_users, chat_offences, lockdowns, blacklist words,
// greetings, rules, notes, filters, locks, chat_settings_access).
// Logs contain only chat IDs, user ID, counts, and step names — never group titles, message texts,
// file IDs, button URLs, or appeal URLs.
func applyCopySettings(ctx context.Context, srcChatID, dstChatID, by int64) copySettingsResult {
	if srcChatID == 0 || dstChatID == 0 || srcChatID == dstChatID {
		return copySettingsResult{Outcome: copySettingsFailed, Err: errCopySameChat}
	}

	snap, step, err := readCopySettingsSnapshot(ctx, srcChatID)
	if err != nil {
		log.Warnf("[CopySettings] step %s failed (src=%d dst=%d by=%d)", step, srcChatID, dstChatID, by)
		return copySettingsResult{Outcome: copySettingsFailed, Err: err}
	}

	if db.DB == nil {
		log.Warnf("[CopySettings] step write_tx failed (src=%d dst=%d by=%d)", srcChatID, dstChatID, by)
		return copySettingsResult{Outcome: copySettingsFailed, Err: errCopyNoDatabase}
	}

	var rep eventmsg.WriteReport
	failedStep := "write_tx"
	txErr := db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var wErr error
		rep, failedStep, wErr = writeCopySettingsTx(tx, dstChatID, by, snap)
		return wErr
	})
	if txErr != nil {
		if failedStep == "" {
			failedStep = "write_tx"
		}
		log.Warnf("[CopySettings] step %s failed (src=%d dst=%d by=%d)", failedStep, srcChatID, dstChatID, by)
		return copySettingsResult{Outcome: copySettingsFailed, Err: txErr}
	}

	dropCopySettingsTargetCaches(dstChatID)

	if len(rep.Skipped) > 0 {
		log.Warnf("[CopySettings] skipped invalid event messages (src=%d dst=%d by=%d written=%d skipped=%d)",
			srcChatID, dstChatID, by, rep.Written, len(rep.Skipped))
	}

	log.Infof("[CopySettings] completed copy (src=%d dst=%d by=%d messages_written=%d messages_skipped=%d)",
		srcChatID, dstChatID, by, rep.Written, len(rep.Skipped))

	return copySettingsResult{
		Outcome:         copySettingsSaved,
		MessagesWritten: rep.Written,
		MessagesSkipped: len(rep.Skipped),
	}
}
