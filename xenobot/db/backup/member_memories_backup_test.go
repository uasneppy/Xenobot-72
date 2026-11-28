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

// TestChatMemoriesStayOutOfBackup proves that the per-member memories are never a backup module: the
// export names none of them, an import writes no memory row into the destination chat, and a reset
// leaves the source group's notes alone (a member's notes are deleted only by /forgetme or by the owner
// turning the feature off).
func TestChatMemoriesStayOutOfBackup(t *testing.T) {
	skipIfNoDb(t)

	srcChat := time.Now().UnixNano()
	dstChat := srcChat + 1
	require.NoError(t, chats.EnsureChatInDb(srcChat, "mem_source"))
	require.NoError(t, chats.EnsureChatInDb(dstChat, "mem_destination"))
	t.Cleanup(func() {
		cleanupBackupChat(t, srcChat)
		cleanupBackupChat(t, dstChat)
		require.NoError(t, db.DB.Where("chat_id IN ?", []int64{srcChat, dstChat}).Delete(&models.ChatMemberMemory{}).Error)
	})
	require.NoError(t, db.DB.Create(&models.ChatMemberMemory{ChatID: srcChat, UserID: 42, Notes: "Likes cats", UpdatedAt: time.Now()}).Error)

	countRows := func(chatID int64) int64 {
		var n int64
		require.NoError(t, db.DB.Model(&models.ChatMemberMemory{}).Where("chat_id = ?", chatID).Count(&n).Error)
		return n
	}

	exported, err := ExportChatData(srcChat, "source", 42, nil)
	require.NoError(t, err)
	for module := range exported.Data {
		assert.True(t, IsValidModule(module), "export holds only backup modules, got %q", module)
	}
	for _, name := range []string{"chat_member_memories", "memories", "member_memories"} {
		_, err = ExportChatData(srcChat, "source", 42, []string{name})
		assert.Error(t, err, "%s is not a backup module", name)
	}
	raw, err := exported.ToJSON()
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "Likes cats", "the export file holds no member note")
	assert.NotContains(t, string(raw), "chat_member_memories", "the export file names no memory table")
	decoded, err := BackupFormatFromJSON(raw)
	require.NoError(t, err)
	require.NoError(t, ImportChatData(dstChat, decoded, nil))
	assert.Zero(t, countRows(dstChat), "an import creates no memory row")

	require.NoError(t, ClearChatData(srcChat, nil))
	assert.EqualValues(t, 1, countRows(srcChat), "a reset leaves the member notes in place")
}
