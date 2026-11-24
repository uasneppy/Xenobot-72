//go:build testtools

package backup

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/chats"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/db/user"
)

// punishImportChat is a fresh chat the bot knows, with every backup row of it removed when the test ends.
func punishImportChat(t *testing.T, users ...int64) int64 {
	t.Helper()
	chatID := -(time.Now().UnixNano()%1_000_000_000_000 + 2_000_000_000_000)
	require.NoError(t, chats.EnsureChatInDb(chatID, "punish_import"))
	t.Cleanup(func() {
		cleanupBackupChat(t, chatID)
		if len(users) > 0 {
			require.NoError(t, db.DB.Where("user_id IN ?", users).Delete(&models.User{}).Error)
		}
	})
	return chatID
}

// TestWarnsExportCountsWhatCounts proves that the warns export carries what counts: with the group's
// expiry on a member's expired warns are left out (num_warns, reasons and times), a member with nothing
// counted is left out, and the times travel only when the row stores one per warn; with expiry off
// every warn is exported as stored and no time travels.
func TestWarnsExportCountsWhatCounts(t *testing.T) {
	skipIfNoDb(t)
	now := time.Now()
	day := int64(24 * 60 * 60)
	base := now.UnixNano()%1_000_000_000 + 6_000_000_000
	mixed, expired, legacy := base, base+1, base+2

	seed := func(t *testing.T, expiry int) int64 {
		t.Helper()
		chatID := punishImportChat(t, mixed, expired, legacy)
		for _, id := range []int64{mixed, expired, legacy} {
			require.NoError(t, user.EnsureUserInDb(id, "", ""))
		}
		require.NoError(t, db.DB.Create(&models.WarnSettings{ChatId: chatID, WarnLimit: 5, WarnMode: "ban", ExpiryDays: expiry}).Error)
		require.NoError(t, db.DB.Create(&models.Warns{
			UserId: mixed, ChatId: chatID, NumWarns: 3, Reasons: models.StringArray{"old1", "old2", "new"},
			WarnTimes: models.Int64Array{now.Unix() - 8*day, now.Unix() - 8*day, now.Unix() - 60},
		}).Error)
		require.NoError(t, db.DB.Create(&models.Warns{
			UserId: expired, ChatId: chatID, NumWarns: 1, Reasons: models.StringArray{"gone"},
			WarnTimes: models.Int64Array{now.Unix() - 9*day},
		}).Error)
		// A row saved before warn times existed: dated at its last change, here a minute ago.
		require.NoError(t, db.DB.Create(&models.Warns{
			UserId: legacy, ChatId: chatID, NumWarns: 2, Reasons: models.StringArray{"a", "b"},
		}).Error)
		return chatID
	}
	byUser := func(t *testing.T, chatID int64) map[int64]models.Warns {
		t.Helper()
		exported, err := exportWarnsData(chatID)
		require.NoError(t, err)
		out := map[int64]models.Warns{}
		for _, row := range exported.Warns {
			out[row.UserId] = row
		}
		return out
	}

	t.Run("expiry on", func(t *testing.T) {
		rows := byUser(t, seed(t, 7))
		require.Contains(t, rows, mixed)
		assert.Equal(t, 1, rows[mixed].NumWarns)
		assert.Equal(t, models.StringArray{"new"}, rows[mixed].Reasons)
		assert.Equal(t, models.Int64Array{now.Unix() - 60}, rows[mixed].WarnTimes)
		assert.NotContains(t, rows, expired, "a member with no counted warn is left out")
		require.Contains(t, rows, legacy)
		assert.Equal(t, 2, rows[legacy].NumWarns, "an old row is dated at its last change and still counts")
		assert.Empty(t, rows[legacy].WarnTimes, "an incomplete row's times never travel")
	})

	t.Run("expiry off", func(t *testing.T) {
		rows := byUser(t, seed(t, 0))
		require.Len(t, rows, 3)
		assert.Equal(t, 3, rows[mixed].NumWarns)
		assert.Equal(t, models.StringArray{"old1", "old2", "new"}, rows[mixed].Reasons)
		assert.Equal(t, 1, rows[expired].NumWarns)
		for id, row := range rows {
			assert.Empty(t, row.WarnTimes, "no time travels with expiry off (user %d)", id)
		}
	})
}

// TestImportSanitizesPunishmentSettings proves that an import stores 0 for a duration that is not a
// valid duration of a timed action and for an expiry outside 0 to 365, drops warn times of the wrong
// length, decreasing or in the future (dating that member's warns at the import), keeps valid ones,
// never refuses the file over them, and reads an old file with none of the new keys as zeros.
func TestImportSanitizesPunishmentSettings(t *testing.T) {
	skipIfNoDb(t)
	now := time.Now()
	base := now.UnixNano()%1_000_000_000 + 6_500_000_000
	short, falling, future, valid := base, base+1, base+2, base+3
	t1, t2 := now.Unix()-3600, now.Unix()-60

	t.Run("tampered values", func(t *testing.T) {
		chatID := punishImportChat(t, short, falling, future, valid)
		bkp := NewBackupFormat(chatID, "tampered", 1, []string{BackupModuleWarns, BackupModuleAntiflood, BackupModuleBlacklists})
		bkp.Data[BackupModuleWarns] = map[string]any{
			"warn_settings": map[string]any{"warn_limit": 3, "warn_mode": "tmute", "warn_mode_secs": 99999999, "warn_expiry_days": 400},
			"warns": []any{
				map[string]any{"user_id": short, "num_warns": 2, "warns": []any{"a", "b"}, "warn_times": []any{t1}},
				map[string]any{"user_id": falling, "num_warns": 2, "warns": []any{"a", "b"}, "warn_times": []any{t2, t1}},
				map[string]any{"user_id": future, "num_warns": 1, "warns": []any{"a"}, "warn_times": []any{now.Unix() + 3600}},
				map[string]any{"user_id": valid, "num_warns": 2, "warns": []any{"a", "b"}, "warn_times": []any{t1, t2}},
			},
		}
		bkp.Data[BackupModuleAntiflood] = map[string]any{
			"settings": map[string]any{"limit": 5, "action": "tmute", "action_secs": 10},
		}
		bkp.Data[BackupModuleBlacklists] = map[string]any{
			"entries": []any{map[string]any{"word": "scam", "action": "tban", "action_secs": -5}},
		}
		require.NoError(t, ImportChatData(chatID, bkp, nil), "a tampered file is imported, never refused")

		var settings models.WarnSettings
		require.NoError(t, db.DB.Where("chat_id = ?", chatID).Take(&settings).Error)
		assert.Equal(t, "tmute", settings.WarnMode)
		assert.Equal(t, 0, settings.WarnModeSecs, "an out-of-range warn duration is stored as 0 (inert)")
		assert.Equal(t, 0, settings.ExpiryDays, "an expiry over 365 is stored as 0 (off)")

		var flood models.AntifloodSettings
		require.NoError(t, db.DB.Where("chat_id = ?", chatID).Take(&flood).Error)
		assert.Equal(t, 0, flood.ActionSecs, "a flood duration under a minute is stored as 0")

		var word models.BlacklistSettings
		require.NoError(t, db.DB.Where("chat_id = ?", chatID).Take(&word).Error)
		assert.Equal(t, 0, word.ActionSecs, "a negative word duration is stored as 0")

		var rows []models.Warns
		require.NoError(t, db.DB.Where("chat_id = ?", chatID).Find(&rows).Error)
		require.Len(t, rows, 4)
		for _, row := range rows {
			switch row.UserId {
			case valid:
				assert.Equal(t, models.Int64Array{t1, t2}, row.WarnTimes, "valid times are kept")
			default:
				assert.Empty(t, row.WarnTimes, "invalid times of user %d are dropped", row.UserId)
				assert.WithinDuration(t, now, row.UpdatedAt, time.Minute, "user %d's warns are dated at the import", row.UserId)
			}
		}
	})

	t.Run("valid values are kept", func(t *testing.T) {
		chatID := punishImportChat(t)
		bkp := NewBackupFormat(chatID, "valid", 1, []string{BackupModuleWarns, BackupModuleAntiflood, BackupModuleBlacklists})
		bkp.Data[BackupModuleWarns] = map[string]any{
			"warn_settings": map[string]any{"warn_limit": 3, "warn_mode": "tban", "warn_mode_secs": 86400, "warn_expiry_days": 365},
		}
		bkp.Data[BackupModuleAntiflood] = map[string]any{"settings": map[string]any{"limit": 5, "action": "tmute", "action_secs": 60}}
		bkp.Data[BackupModuleBlacklists] = map[string]any{
			"entries": []any{map[string]any{"word": "scam", "action": "kick", "action_secs": 3600}},
		}
		require.NoError(t, ImportChatData(chatID, bkp, nil))
		var settings models.WarnSettings
		require.NoError(t, db.DB.Where("chat_id = ?", chatID).Take(&settings).Error)
		assert.Equal(t, 86400, settings.WarnModeSecs)
		assert.Equal(t, 365, settings.ExpiryDays)
		var flood models.AntifloodSettings
		require.NoError(t, db.DB.Where("chat_id = ?", chatID).Take(&flood).Error)
		assert.Equal(t, 60, flood.ActionSecs)
		var word models.BlacklistSettings
		require.NoError(t, db.DB.Where("chat_id = ?", chatID).Take(&word).Error)
		assert.Equal(t, 0, word.ActionSecs, "a plain action keeps no duration")
	})

	t.Run("an old file reads as zeros", func(t *testing.T) {
		chatID := punishImportChat(t, valid)
		bkp := NewBackupFormat(chatID, "old", 1, []string{BackupModuleWarns, BackupModuleAntiflood, BackupModuleBlacklists})
		bkp.Data[BackupModuleWarns] = map[string]any{
			"warn_settings": map[string]any{"warn_limit": 4, "warn_mode": "ban"},
			"warns":         []any{map[string]any{"user_id": valid, "num_warns": 1, "warns": []any{"a"}}},
		}
		bkp.Data[BackupModuleAntiflood] = map[string]any{"settings": map[string]any{"limit": 7, "action": "tmute"}}
		bkp.Data[BackupModuleBlacklists] = map[string]any{
			"entries": []any{map[string]any{"word": "scam", "action": "tban"}},
		}
		require.NoError(t, ImportChatData(chatID, bkp, nil))
		var settings models.WarnSettings
		require.NoError(t, db.DB.Where("chat_id = ?", chatID).Take(&settings).Error)
		assert.Equal(t, 0, settings.WarnModeSecs)
		assert.Equal(t, 0, settings.ExpiryDays)
		var flood models.AntifloodSettings
		require.NoError(t, db.DB.Where("chat_id = ?", chatID).Take(&flood).Error)
		assert.Equal(t, "tmute", flood.Action)
		assert.Equal(t, 0, flood.ActionSecs, "a timed action with no duration stays inert")
		var word models.BlacklistSettings
		require.NoError(t, db.DB.Where("chat_id = ?", chatID).Take(&word).Error)
		assert.Equal(t, 0, word.ActionSecs)
		var row models.Warns
		require.NoError(t, db.DB.Where("chat_id = ? AND user_id = ?", chatID, valid).Take(&row).Error)
		assert.Equal(t, 1, row.NumWarns)
		assert.Empty(t, row.WarnTimes)
		assert.WithinDuration(t, now, row.UpdatedAt, time.Minute, "warns with no times are dated at the import")
	})
}
