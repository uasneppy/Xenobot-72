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
)

// TestModTuningExcludedFromBackup proves that chat_manual_defaults and chat_reason_shortcuts are
// never part of backup export, import, or clear/reset.
func TestModTuningExcludedFromBackup(t *testing.T) {
	skipIfNoDb(t)

	srcChat := time.Now().UnixNano()
	dstChat := srcChat + 1
	require.NoError(t, chats.EnsureChatInDb(srcChat, "modtuning_source"))
	require.NoError(t, chats.EnsureChatInDb(dstChat, "modtuning_destination"))
	t.Cleanup(func() {
		cleanupBackupChat(t, srcChat)
		cleanupBackupChat(t, dstChat)
		chatIDs := []int64{srcChat, dstChat}
		require.NoError(t, db.DB.Where("chat_id IN ?", chatIDs).Delete(&models.ChatManualDefaults{}).Error)
		require.NoError(t, db.DB.Where("chat_id IN ?", chatIDs).Delete(&models.ChatReasonShortcut{}).Error)
	})

	require.NoError(t, db.DB.Create(&models.ChatManualDefaults{
		ChatId:          srcChat,
		DefaultBanSecs:  3600,
		DefaultMuteSecs: 1800,
		ReasonRequired:  true,
		AppealURL:       "https://example.org/appeal",
		UpdatedBy:       42,
		UpdatedAt:       time.Now(),
	}).Error)
	require.NoError(t, db.DB.Create(&[]models.ChatReasonShortcut{
		{ChatId: srcChat, Word: "spam", Reason: "Unsolicited ads", CreatedBy: 42, CreatedAt: time.Now()},
		{ChatId: srcChat, Word: "flood", Reason: "Repeated messages", CreatedBy: 42, CreatedAt: time.Now()},
	}).Error)

	count := func(model any, chatID int64) int64 {
		var n int64
		require.NoError(t, db.DB.Model(model).Where("chat_id = ?", chatID).Count(&n).Error)
		return n
	}

	exported, err := ExportChatData(srcChat, "source", 42, nil)
	require.NoError(t, err)
	for module := range exported.Data {
		assert.True(t, IsValidModule(module), "export holds only backup modules, got %q", module)
	}
	for _, name := range []string{"chat_manual_defaults", "chat_reason_shortcuts", "modtuning", "manual_defaults", "reason_shortcuts"} {
		_, err = ExportChatData(srcChat, "source", 42, []string{name})
		assert.Error(t, err, "%s is not a backup module", name)
	}
	raw, err := exported.ToJSON()
	require.NoError(t, err)
	for _, needle := range []string{
		"chat_manual_defaults", "chat_reason_shortcuts", "default_ban_secs", "default_mute_secs",
		"reason_required", "appeal_url", "https://example.org/appeal", "Unsolicited ads",
	} {
		assert.NotContains(t, string(raw), needle, "export must not contain modtuning data or columns")
	}

	decoded, err := BackupFormatFromJSON(raw)
	require.NoError(t, err)
	require.NoError(t, ImportChatData(dstChat, decoded, nil))
	assert.Zero(t, count(&models.ChatManualDefaults{}, dstChat), "import must not create chat_manual_defaults rows")
	assert.Zero(t, count(&models.ChatReasonShortcut{}, dstChat), "import must not create chat_reason_shortcuts rows")

	require.NoError(t, ClearChatData(srcChat, nil))
	assert.EqualValues(t, 1, count(&models.ChatManualDefaults{}, srcChat), "clear must leave chat_manual_defaults untouched")
	assert.EqualValues(t, 2, count(&models.ChatReasonShortcut{}, srcChat), "clear must leave chat_reason_shortcuts untouched")
}
