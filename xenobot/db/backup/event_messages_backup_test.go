//go:build testtools

package backup

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/chats"
	"github.com/uasneppy/Xenobot-72/xenobot/db/eventmsg"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	utilscache "github.com/uasneppy/Xenobot-72/xenobot/utils/cache"
)

func countEventMessageRows(t *testing.T, chatID int64) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.DB.Model(&models.ChatEventMessage{}).Where("chat_id = ?", chatID).Count(&n).Error)
	return n
}

func countEventMessageOptionRows(t *testing.T, chatID int64) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.DB.Model(&models.ChatEventMessageOptions{}).Where("chat_id = ?", chatID).Count(&n).Error)
	return n
}

// TestEventMessagesInBackup proves MSG-10, P30-02, P30-03, and T-30-03: custom event messages
// (group and pm destinations) and their options export under BackupModuleMessages ("messages"),
// round-trip through JSON and ImportChatDataReport with cache invalidation, roll back atomically
// when a sibling module fails in the same transaction, clear cleanly on ClearChatData (/reset)
// with cache invalidation, and leave tuning and staff tables outside AllExportableModules().
func TestEventMessagesInBackup(t *testing.T) {
	skipIfNoDb(t)
	utilscache.SetupTestMemoryMarshaler(t)

	ctx := context.Background()
	srcChat := time.Now().UnixNano()
	dstChat := srcChat + 1
	require.NoError(t, chats.EnsureChatInDb(srcChat, "event_messages_source"))
	require.NoError(t, chats.EnsureChatInDb(dstChat, "event_messages_destination"))
	t.Cleanup(func() {
		cleanupBackupChat(t, srcChat)
		cleanupBackupChat(t, dstChat)
	})

	// Seed source chat with group message (text + photo media + URL button), PM message,
	// non-default silent-only group row, and all three options enabled.
	require.NoError(t, eventmsg.SetText(ctx, srcChat, eventmsg.EventBan, eventmsg.DestGroup, "CUSTOM-EVENT-TEXT", 42))
	require.NoError(t, eventmsg.SetMedia(ctx, srcChat, eventmsg.EventBan, eventmsg.DestGroup, "CUSTOM-FILE-ID", db.PHOTO, 42))
	require.NoError(t, eventmsg.SetButtons(ctx, srcChat, eventmsg.EventBan, eventmsg.DestGroup,
		[]models.Button{{Name: "CUSTOM-BUTTON", Url: "https://example.org/custom"}}, 42))
	require.NoError(t, eventmsg.SetText(ctx, srcChat, eventmsg.EventBan, eventmsg.DestPM, "CUSTOM-PM-TEXT", 42))
	require.NoError(t, eventmsg.SetSilent(ctx, srcChat, eventmsg.EventBan, eventmsg.DestPM, false, 42))
	require.NoError(t, eventmsg.SetSilent(ctx, srcChat, eventmsg.EventKick, eventmsg.DestGroup, true, 42))
	require.NoError(t, eventmsg.SetPMDetails(ctx, srcChat, true, 42))
	require.NoError(t, eventmsg.SetPMLog(ctx, srcChat, true, 42))
	require.NoError(t, eventmsg.SetPlainTone(ctx, srcChat, true, 42))

	// Seed destination chat with stale message and prime its cache so invalidation is tested.
	require.NoError(t, eventmsg.SetText(ctx, dstChat, eventmsg.EventWarn, eventmsg.DestGroup, "STALE-WARN", 1))
	primedDst, err := eventmsg.Get(ctx, dstChat)
	require.NoError(t, err)
	_, hasStale := primedDst.Lookup(eventmsg.EventWarn, eventmsg.DestGroup)
	require.True(t, hasStale)

	require.EqualValues(t, 3, countEventMessageRows(t, srcChat))
	require.EqualValues(t, 1, countEventMessageOptionRows(t, srcChat))

	// Export all modules and verify "messages" is present with serialized messages + options.
	exported, err := ExportChatData(srcChat, "source", 42, nil)
	require.NoError(t, err)
	assert.Len(t, exported.Data, len(AllExportableModules()))
	assert.Contains(t, exported.Modules, BackupModuleMessages)
	assert.Contains(t, exported.Data, BackupModuleMessages)

	raw, err := exported.ToJSON()
	require.NoError(t, err)
	for _, needle := range []string{
		`"messages"`, `"options"`, "CUSTOM-EVENT-TEXT", "CUSTOM-FILE-ID", "CUSTOM-BUTTON",
		"CUSTOM-PM-TEXT", `"pm_details"`, `"pm_log"`, `"plain_tone"`,
	} {
		assert.Contains(t, string(raw), needle, "exported JSON should carry custom messages field %s", needle)
	}

	// Import into destination chat and verify report, DB rows, options, and cache invalidation.
	decoded, err := BackupFormatFromJSON(raw)
	require.NoError(t, err)
	report, err := ImportChatDataReport(dstChat, decoded, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, report.Skipped)
	assert.EqualValues(t, 3, countEventMessageRows(t, dstChat))
	assert.EqualValues(t, 1, countEventMessageOptionRows(t, dstChat))

	// Reading via cached eventmsg.Get proves eventmsg.CacheKey(dstChat) was invalidated on commit.
	for _, check := range []struct {
		name string
		read func() (*eventmsg.Messages, error)
	}{
		{name: "cached", read: func() (*eventmsg.Messages, error) { return eventmsg.Get(ctx, dstChat) }},
		{name: "fresh", read: func() (*eventmsg.Messages, error) { return eventmsg.GetFresh(ctx, dstChat) }},
	} {
		t.Run("restored_"+check.name, func(t *testing.T) {
			got, readErr := check.read()
			require.NoError(t, readErr)
			_, staleSurvived := got.Lookup(eventmsg.EventWarn, eventmsg.DestGroup)
			assert.False(t, staleSurvived, "stale destination custom message must be replaced")

			banGroup, ok := got.Lookup(eventmsg.EventBan, eventmsg.DestGroup)
			require.True(t, ok)
			assert.Equal(t, "CUSTOM-EVENT-TEXT", banGroup.Text)
			assert.Equal(t, "CUSTOM-FILE-ID", banGroup.FileID)
			assert.Equal(t, db.PHOTO, banGroup.MediaType)
			assert.Equal(t, []models.Button{{Name: "CUSTOM-BUTTON", Url: "https://example.org/custom"}}, banGroup.Buttons)
			assert.False(t, banGroup.Silent)

			banPM, ok := got.Lookup(eventmsg.EventBan, eventmsg.DestPM)
			require.True(t, ok)
			assert.Equal(t, "CUSTOM-PM-TEXT", banPM.Text)
			assert.False(t, banPM.Silent)

			kickGroup, ok := got.Lookup(eventmsg.EventKick, eventmsg.DestGroup)
			require.True(t, ok)
			assert.True(t, kickGroup.Silent)

			assert.Equal(t, eventmsg.Options{PMDetails: true, PMLog: true, PlainTone: true}, got.Options)
		})
	}

	// T-30-03: multi-module rollback atomicity when a later module fails inside withBackupTx.
	rollbackBkp := NewBackupFormat(dstChat, "rollback", 1, []string{BackupModuleMessages, BackupModuleCaptcha})
	rollbackBkp.Data[BackupModuleMessages] = map[string]interface{}{
		"messages": []interface{}{
			map[string]interface{}{"event": eventmsg.EventMute, "dest": eventmsg.DestGroup, "text": "SHOULD-ROLL-BACK"},
		},
		"options": map[string]interface{}{"pm_details": false, "pm_log": false, "plain_tone": false},
	}
	rollbackBkp.Data[BackupModuleCaptcha] = map[string]interface{}{
		"settings": map[string]interface{}{
			"enabled": true, "captcha_mode": "invalid", "timeout": 2,
			"failure_action": "kick", "max_attempts": 3,
		},
	}
	require.Error(t, ImportChatData(dstChat, rollbackBkp, nil))
	afterRollback, err := eventmsg.GetFresh(ctx, dstChat)
	require.NoError(t, err)
	_, leakedMute := afterRollback.Lookup(eventmsg.EventMute, eventmsg.DestGroup)
	assert.False(t, leakedMute, "rolled-back messages import must not commit")
	banAfterRollback, ok := afterRollback.Lookup(eventmsg.EventBan, eventmsg.DestGroup)
	require.True(t, ok)
	assert.Equal(t, "CUSTOM-EVENT-TEXT", banAfterRollback.Text)
	assert.Equal(t, eventmsg.Options{PMDetails: true, PMLog: true, PlainTone: true}, afterRollback.Options)

	// P30-02: full reset (ClearChatData(srcChat, nil)) and single-module reset
	// (ClearChatData(dstChat, []string{BackupModuleMessages})) clear both tables and invalidate cache.
	_, err = eventmsg.Get(ctx, srcChat)
	require.NoError(t, err)
	_, err = eventmsg.Get(ctx, dstChat)
	require.NoError(t, err)

	require.NoError(t, ClearChatData(srcChat, nil))
	require.NoError(t, ClearChatData(dstChat, []string{BackupModuleMessages}))

	for _, chatID := range []int64{srcChat, dstChat} {
		assert.Zero(t, countEventMessageRows(t, chatID), "reset must clear chat_event_messages for chat %d", chatID)
		assert.Zero(t, countEventMessageOptionRows(t, chatID), "reset must clear chat_event_message_options for chat %d", chatID)

		cachedAfterReset, getErr := eventmsg.Get(ctx, chatID)
		require.NoError(t, getErr)
		assert.Empty(t, cachedAfterReset.Rows)
		assert.Equal(t, eventmsg.Options{}, cachedAfterReset.Options)

		freshAfterReset, freshErr := eventmsg.GetFresh(ctx, chatID)
		require.NoError(t, freshErr)
		assert.Empty(t, freshAfterReset.Rows)
		assert.Equal(t, eventmsg.Options{}, freshAfterReset.Options)
	}

	// P30-03: tuning and staff tables stay excluded from backup modules.
	for _, excluded := range []string{
		"escalation", "chat_escalation",
		"offences", "chat_offences",
		"manual_defaults", "chat_manual_defaults",
		"reason_shortcuts", "chat_reason_shortcuts",
		"advanced_thresholds", "chat_advanced_thresholds",
		"conversations", "chat_conversations",
		"staff", "staff_groups",
	} {
		assert.False(t, IsValidModule(excluded), "tuning/staff name %q must stay out of backups", excluded)
	}
}

// TestEventMessagesImportValidationAndSkipped proves MSG-10 / T-30-02: every imported custom
// message row passes eventmsg.ValidateMessage via ReplaceAllValidatedTx; invalid or unknown
// future rows are skipped and counted in ImportReport.Skipped while valid rows and options apply.
func TestEventMessagesImportValidationAndSkipped(t *testing.T) {
	skipIfNoDb(t)
	utilscache.SetupTestMemoryMarshaler(t)

	ctx := context.Background()
	dstChat := time.Now().UnixNano()
	require.NoError(t, chats.EnsureChatInDb(dstChat, "event_messages_validation"))
	t.Cleanup(func() { cleanupBackupChat(t, dstChat) })

	bkp := NewBackupFormat(dstChat, "untrusted", 1, []string{BackupModuleMessages})
	bkp.Data[BackupModuleMessages] = MessagesBackup{
		Messages: []models.ChatEventMessage{
			// 1 valid row.
			{
				Event:   eventmsg.EventBan,
				Dest:    eventmsg.DestGroup,
				Text:    "Valid custom ban",
				Buttons: models.ButtonArray{{Name: "Rules", Url: "https://example.org/rules"}},
			},
			// 1 invalid unknown future event key ("unknown_event").
			{
				Event: "future_v99_event",
				Dest:  eventmsg.DestGroup,
				Text:  "from the future",
			},
			// 2 invalid destination ("unknown_dest").
			{
				Event: eventmsg.EventBan,
				Dest:  "channel",
				Text:  "bad dest",
			},
			// 3 invalid PM on lockdown event ("pm_not_supported").
			{
				Event: eventmsg.EventLockdown,
				Dest:  eventmsg.DestPM,
				Text:  "no pm for lockdown",
			},
			// 4 invalid disallowed media on member requirement ("media_not_allowed").
			{
				Event:     eventmsg.EventReqWarn,
				Dest:      eventmsg.DestGroup,
				FileID:    "mreq-photo",
				MediaType: db.PHOTO,
			},
			// 5 invalid button URL ("invalid_button_url").
			{
				Event:   eventmsg.EventWarn,
				Dest:    eventmsg.DestGroup,
				Text:    "xss button",
				Buttons: models.ButtonArray{{Name: "Click", Url: "javascript:alert(1)"}},
			},
			// 6 invalid over-limit text on member requirement (3497 > 4096 - 600 = 3496, "text_too_long").
			{
				Event: eventmsg.EventReqMute,
				Dest:  eventmsg.DestGroup,
				Text:  strings.Repeat("a", eventmsg.MaxTextLength-eventmsg.MreqTextReserve+1),
			},
			// 7 invalid over-limit caption on lockdown photo (625 > 1024 - 400 = 624, "text_too_long").
			{
				Event:     eventmsg.EventLockdown,
				Dest:      eventmsg.DestGroup,
				Text:      strings.Repeat("a", eventmsg.MaxCaptionLength-eventmsg.LockdownTextReserve+1),
				FileID:    "ld-photo",
				MediaType: db.PHOTO,
			},
		},
		Options: &models.ChatEventMessageOptions{
			PMDetails: true,
			PMLog:     false,
			PlainTone: true,
		},
	}

	raw, err := bkp.ToJSON()
	require.NoError(t, err)
	decoded, err := BackupFormatFromJSON(raw)
	require.NoError(t, err)

	report, err := ImportChatDataReport(dstChat, decoded, nil)
	require.NoError(t, err)
	assert.Equal(t, 7, report.Skipped)
	assert.EqualValues(t, 1, countEventMessageRows(t, dstChat))

	stored, err := eventmsg.GetFresh(ctx, dstChat)
	require.NoError(t, err)
	require.Len(t, stored.Rows, 1)
	ban, ok := stored.Lookup(eventmsg.EventBan, eventmsg.DestGroup)
	require.True(t, ok)
	assert.Equal(t, "Valid custom ban", ban.Text)
	assert.Equal(t, []models.Button{{Name: "Rules", Url: "https://example.org/rules"}}, ban.Buttons)
	assert.Equal(t, eventmsg.Options{PMDetails: true, PMLog: false, PlainTone: true}, stored.Options)
}

// TestOldExportWithoutMessagesAndOmittedFields proves backward compatibility for MSG-10:
// (a) an old export without "messages" in modules/data imports cleanly and leaves existing
// custom messages and options untouched;
// (b) an export with "messages": {} decodes omitted fields to zeros/defaults and clears existing
// custom messages and options;
// (c) raw table-name aliases stay rejected by IsValidModule and ExportChatData.
func TestOldExportWithoutMessagesAndOmittedFields(t *testing.T) {
	skipIfNoDb(t)
	utilscache.SetupTestMemoryMarshaler(t)

	ctx := context.Background()
	dstChat := time.Now().UnixNano()
	require.NoError(t, chats.EnsureChatInDb(dstChat, "old_export_messages"))
	t.Cleanup(func() { cleanupBackupChat(t, dstChat) })

	// Seed destination chat with custom messages and options.
	require.NoError(t, eventmsg.SetText(ctx, dstChat, eventmsg.EventBan, eventmsg.DestGroup, "EXISTING-BAN", 42))
	require.NoError(t, eventmsg.SetText(ctx, dstChat, eventmsg.EventBan, eventmsg.DestPM, "EXISTING-PM", 42))
	require.NoError(t, eventmsg.SetSilent(ctx, dstChat, eventmsg.EventBan, eventmsg.DestPM, false, 42))
	require.NoError(t, eventmsg.SetPMDetails(ctx, dstChat, true, 42))
	require.NoError(t, eventmsg.SetPMLog(ctx, dstChat, true, 42))
	require.NoError(t, eventmsg.SetPlainTone(ctx, dstChat, true, 42))
	_, err := eventmsg.Get(ctx, dstChat)
	require.NoError(t, err)

	// (a) Old export JSON without "messages" imports cleanly and preserves custom messages.
	oldRaw := fmt.Sprintf(`{
		"version": "1.1",
		"bot_name": "Xenobot-72",
		"chat_id": %d,
		"chat_name": "legacy",
		"exported_by": 1,
		"modules": ["rules", "greetings"],
		"data": {
			"rules": {"settings": {"rules": "imported rules"}},
			"greetings": {"settings": {"welcome_settings": {"should_welcome": true, "welcome_text": "hi"}}}
		}
	}`, dstChat)
	oldBkp, err := BackupFormatFromJSON([]byte(oldRaw))
	require.NoError(t, err)

	oldReport, err := ImportChatDataReport(dstChat, oldBkp, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, oldReport.Skipped)

	rulesData, err := exportRulesData(dstChat)
	require.NoError(t, err)
	require.NotNil(t, rulesData.Settings)
	assert.Equal(t, "imported rules", rulesData.Settings.Rules)

	preserved, err := eventmsg.GetFresh(ctx, dstChat)
	require.NoError(t, err)
	banGroup, ok := preserved.Lookup(eventmsg.EventBan, eventmsg.DestGroup)
	require.True(t, ok)
	assert.Equal(t, "EXISTING-BAN", banGroup.Text)
	banPM, ok := preserved.Lookup(eventmsg.EventBan, eventmsg.DestPM)
	require.True(t, ok)
	assert.Equal(t, "EXISTING-PM", banPM.Text)
	assert.False(t, banPM.Silent)
	assert.Equal(t, eventmsg.Options{PMDetails: true, PMLog: true, PlainTone: true}, preserved.Options)

	// (b) Export with "messages": {} decodes omitted fields to defaults and clears custom messages/options.
	emptyObjRaw := fmt.Sprintf(`{
		"version": "1.1",
		"bot_name": "Xenobot-72",
		"chat_id": %d,
		"chat_name": "empty_messages",
		"exported_by": 1,
		"modules": ["messages"],
		"data": {
			"messages": {}
		}
	}`, dstChat)
	emptyObjBkp, err := BackupFormatFromJSON([]byte(emptyObjRaw))
	require.NoError(t, err)

	emptyReport, err := ImportChatDataReport(dstChat, emptyObjBkp, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, emptyReport.Skipped)
	assert.Zero(t, countEventMessageRows(t, dstChat))
	assert.Zero(t, countEventMessageOptionRows(t, dstChat))

	clearedCached, err := eventmsg.Get(ctx, dstChat)
	require.NoError(t, err)
	assert.Empty(t, clearedCached.Rows)
	assert.Equal(t, eventmsg.Options{}, clearedCached.Options)

	clearedFresh, err := eventmsg.GetFresh(ctx, dstChat)
	require.NoError(t, err)
	assert.Empty(t, clearedFresh.Rows)
	assert.Equal(t, eventmsg.Options{}, clearedFresh.Options)

	// (c) Table-name aliases stay rejected; only "messages" is valid.
	assert.True(t, IsValidModule(BackupModuleMessages))
	for _, alias := range []string{
		"chat_event_messages",
		"event_messages",
		"chat_event_message_options",
		"event_message_options",
	} {
		assert.False(t, IsValidModule(alias), "alias %q must not be a valid backup module", alias)
		_, exportErr := ExportChatData(dstChat, "alias_check", 1, []string{alias})
		assert.Error(t, exportErr, "ExportChatData must reject alias %q", alias)
	}
}
