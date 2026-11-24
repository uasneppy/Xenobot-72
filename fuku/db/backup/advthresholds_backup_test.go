//go:build testtools

package backup

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/chats"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
)

// TestAdvThresholdsExcludedFromBackup proves that chat_advanced_thresholds is never part of backup
// export, import, or clear/reset.
func TestAdvThresholdsExcludedFromBackup(t *testing.T) {
	skipIfNoDb(t)

	srcChat := time.Now().UnixNano()
	dstChat := srcChat + 1
	require.NoError(t, chats.EnsureChatInDb(srcChat, "advthresholds_source"))
	require.NoError(t, chats.EnsureChatInDb(dstChat, "advthresholds_destination"))
	t.Cleanup(func() {
		cleanupBackupChat(t, srcChat)
		cleanupBackupChat(t, dstChat)
		chatIDs := []int64{srcChat, dstChat}
		require.NoError(t, db.DB.Where("chat_id IN ?", chatIDs).Delete(&models.ChatAdvancedThresholds{}).Error)
	})

	require.NoError(t, db.DB.Create(&models.ChatAdvancedThresholds{
		ChatId:               srcChat,
		RaidJoinWindowSecs:   120,
		RaidFloodWindowSecs:  90,
		RaidAIWindowSecs:     300,
		RaidFloodNewPercent:  40,
		RaidFloodMinAccounts: 5,
		RaidRecentHours:      48,
		RaidAIImageBarPct:    75,
		MemberReqAIBarPct:    80,
		MemberReqCapPerMin:   25,
		MemberReqPauseMins:   15,
		LockdownReminderMins: 30,
		CaptchaTimeoutMins:   20,
		UpdatedBy:            42,
		UpdatedAt:            time.Now(),
	}).Error)

	count := func(chatID int64) int64 {
		var n int64
		require.NoError(t, db.DB.Model(&models.ChatAdvancedThresholds{}).Where("chat_id = ?", chatID).Count(&n).Error)
		return n
	}

	exported, err := ExportChatData(srcChat, "source", 42, nil)
	require.NoError(t, err)
	for module := range exported.Data {
		assert.True(t, IsValidModule(module), "export holds only backup modules, got %q", module)
	}
	for _, name := range []string{"chat_advanced_thresholds", "advthresholds", "advanced_thresholds"} {
		_, err = ExportChatData(srcChat, "source", 42, []string{name})
		assert.Error(t, err, "%s is not a backup module", name)
	}
	raw, err := exported.ToJSON()
	require.NoError(t, err)
	for _, needle := range []string{
		"chat_advanced_thresholds",
		"raid_join_window_secs",
		"raid_flood_window_secs",
		"raid_ai_window_secs",
		"raid_flood_new_percent",
		"raid_flood_min_accounts",
		"raid_recent_hours",
		"raid_ai_image_bar_pct",
		"member_req_ai_bar_pct",
		"member_req_cap_per_min",
		"member_req_pause_mins",
		"lockdown_reminder_mins",
		"captcha_timeout_mins",
	} {
		assert.NotContains(t, string(raw), needle, "export must not contain advanced thresholds data or columns")
	}

	decoded, err := BackupFormatFromJSON(raw)
	require.NoError(t, err)
	require.NoError(t, ImportChatData(dstChat, decoded, nil))
	assert.Zero(t, count(dstChat), "import must not create chat_advanced_thresholds rows")

	require.NoError(t, ClearChatData(srcChat, nil))
	assert.EqualValues(t, 1, count(srcChat), "clear must leave chat_advanced_thresholds untouched")
}
