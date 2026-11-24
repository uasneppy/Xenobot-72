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

// TestEscalationStaysOutOfBackup proves that the escalation ladder and the offence counts are
// never a backup module: the export names neither table nor any of their columns, an import writes
// no row into the destination chat, and a reset leaves the source rows alone (D-37).
func TestEscalationStaysOutOfBackup(t *testing.T) {
	skipIfNoDb(t)

	srcChat := time.Now().UnixNano()
	dstChat := srcChat + 1
	require.NoError(t, chats.EnsureChatInDb(srcChat, "escalation_source"))
	require.NoError(t, chats.EnsureChatInDb(dstChat, "escalation_destination"))
	t.Cleanup(func() {
		cleanupBackupChat(t, srcChat)
		cleanupBackupChat(t, dstChat)
		chatIDs := []int64{srcChat, dstChat}
		require.NoError(t, db.DB.Where("chat_id IN ?", chatIDs).Delete(&models.ChatEscalation{}).Error)
		require.NoError(t, db.DB.Where("chat_id IN ?", chatIDs).Delete(&models.ChatOffence{}).Error)
	})
	require.NoError(t, db.DB.Create(&models.ChatEscalation{
		ChatId: srcChat, Enabled: true,
		Step1Action: "tmute", Step1Secs: 3600, Step2Action: "ban",
		ForgetDays: 14, UpdatedBy: 42,
	}).Error)
	require.NoError(t, db.DB.Create(&[]models.ChatOffence{
		{ChatId: srcChat, UserId: 7001, Offences: 2, LastOffenceUnix: 1_800_000_000},
		{ChatId: srcChat, UserId: 7002, Offences: 1, LastOffenceUnix: 1_800_000_100},
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
	for _, name := range []string{"chat_escalation", "escalation", "chat_offences", "offences"} {
		_, err = ExportChatData(srcChat, "source", 42, []string{name})
		assert.Error(t, err, "%s is not a backup module", name)
	}
	raw, err := exported.ToJSON()
	require.NoError(t, err)
	for _, name := range []string{
		"chat_escalation", "chat_offences", "step1_action", "step1_secs", "step5_action", "forget_days",
		"last_offence_unix", "offences",
	} {
		assert.NotContains(t, string(raw), name, "the export file names no escalation table or column")
	}
	decoded, err := BackupFormatFromJSON(raw)
	require.NoError(t, err)
	require.NoError(t, ImportChatData(dstChat, decoded, nil))
	assert.Zero(t, count(&models.ChatEscalation{}, dstChat), "an import creates no chat_escalation row")
	assert.Zero(t, count(&models.ChatOffence{}, dstChat), "an import creates no chat_offences row")

	require.NoError(t, ClearChatData(srcChat, nil))
	assert.EqualValues(t, 1, count(&models.ChatEscalation{}, srcChat), "a reset leaves the ladder in place")
	assert.EqualValues(t, 2, count(&models.ChatOffence{}, srcChat), "a reset leaves the offence counts in place")
}
