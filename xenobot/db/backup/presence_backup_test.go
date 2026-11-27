//go:build testtools

package backup

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/chats"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/db/presence"
	"github.com/uasneppy/Xenobot-72/xenobot/db/reactions"
	utilscache "github.com/uasneppy/Xenobot-72/xenobot/utils/cache"
)

// TestBackupPresenceArchiveStripped proves COMM-05 and T-33-16 for BackupModulePresence:
//   - exports omit archive_chat_id and archive_thread_id from JSON,
//   - importing into an unlinked chat keeps archive_chat_id=0, archive_thread_id=0, forces
//     archive_enabled=false, and sets report.ArchiveChannelSkipped=true (even with tampered JSON),
//   - importing into a chat with an existing /setarchive target preserves its own target and sets
//     report.ArchiveChannelSkipped=false,
//   - out-of-range fields are clamped to safe defaults and cache is invalidated post-commit,
//   - nil settings leaves destination untouched,
//   - ClearModuleData(BackupModulePresence) removes the row and invalidates cache, and
//   - multi-module transaction rollback restores prior state.
func TestBackupPresenceArchiveStripped(t *testing.T) {
	skipIfNoDb(t)
	utilscache.SetupTestMemoryMarshaler(t)
	ctx := context.Background()

	t.Run("export omits archive target and unlinked import forces archive off", func(t *testing.T) {
		srcChat := time.Now().UnixNano()
		dstChat := srcChat + 1
		require.NoError(t, chats.EnsureChatInDb(srcChat, "presence_export_src"))
		require.NoError(t, chats.EnsureChatInDb(dstChat, "presence_unlinked_dst"))
		t.Cleanup(func() {
			cleanupBackupChat(t, srcChat)
			cleanupBackupChat(t, dstChat)
		})

		require.NoError(t, db.DB.Create(&models.ChatPresence{
			ChatID:             srcChat,
			ConsensusEnabled:   true,
			ConsensusThreshold: 6,
			RetractEnabled:     false,
			AIReactMode:        presence.AIReactModeLow,
			CatStickerEnabled:  true,
			ArchiveEnabled:     true,
			ArchiveChatID:      -100777888,
			ArchiveThreadID:    42,
			ArchiveThreshold:   8,
			ArchiveEmojiMode:   presence.ArchiveEmojiModeAll,
			ArchiveDelivery:    presence.ArchiveDeliveryForward,
			AmbientContext:     true,
			VocativeEnabled:    true,
			PMBriefReact:       true,
			ReferendumEnabled:  true,
			InspectEnabled:     true,
		}).Error)

		// Prime destination cache before import so cache invalidation is verified.
		_, err := presence.Get(ctx, dstChat)
		require.NoError(t, err)

		exported, err := ExportChatData(srcChat, "src", 1, []string{BackupModulePresence})
		require.NoError(t, err)
		raw, err := exported.ToJSON()
		require.NoError(t, err)
		assert.NotContains(t, string(raw), "archive_chat_id")
		assert.NotContains(t, string(raw), "archive_thread_id")

		decoded, err := BackupFormatFromJSON(raw)
		require.NoError(t, err)
		report, err := ImportChatDataReport(dstChat, decoded, []string{BackupModulePresence})
		require.NoError(t, err)
		assert.True(t, report.ArchiveChannelSkipped)

		got, err := presence.Get(ctx, dstChat)
		require.NoError(t, err)
		assert.True(t, got.ConsensusEnabled)
		assert.Equal(t, 6, got.ConsensusThreshold)
		assert.False(t, got.RetractEnabled)
		assert.Equal(t, presence.AIReactModeLow, got.AIReactMode)
		assert.True(t, got.CatStickerEnabled)
		assert.False(t, got.ArchiveEnabled)
		assert.Zero(t, got.ArchiveChatID)
		assert.Zero(t, got.ArchiveThreadID)
		assert.Equal(t, 8, got.ArchiveThreshold)
		assert.Equal(t, presence.ArchiveEmojiModeAll, got.ArchiveEmojiMode)
		assert.Equal(t, presence.ArchiveDeliveryForward, got.ArchiveDelivery)
		assert.True(t, got.AmbientContext)
		assert.True(t, got.VocativeEnabled)
		assert.True(t, got.PMBriefReact)
		assert.True(t, got.ReferendumEnabled)
		assert.True(t, got.InspectEnabled)
	})

	t.Run("tampered JSON archive target ignored on unlinked and linked targets", func(t *testing.T) {
		unlinkedChat := time.Now().UnixNano()
		linkedChat := unlinkedChat + 1
		require.NoError(t, chats.EnsureChatInDb(unlinkedChat, "presence_tamper_unlinked"))
		require.NoError(t, chats.EnsureChatInDb(linkedChat, "presence_tamper_linked"))
		t.Cleanup(func() {
			cleanupBackupChat(t, unlinkedChat)
			cleanupBackupChat(t, linkedChat)
		})

		ownTarget := int64(-100555666)
		ownThread := int64(99)
		require.NoError(t, presence.SetArchiveTarget(ctx, linkedChat, ownTarget, ownThread, 1))
		require.NoError(t, presence.SetArchiveEnabled(ctx, linkedChat, false, 1))
		// Prime cache on linkedChat before import.
		_, err := presence.Get(ctx, linkedChat)
		require.NoError(t, err)

		tamperedJSON := `{
			"version":"1.1",
			"bot_name":"Xenobot-72",
			"chat_id":1,
			"modules":["presence"],
			"data":{"presence":{"settings":{
				"consensus_enabled":true,
				"consensus_threshold":999,
				"retract_enabled":false,
				"ai_react_mode":"bogus",
				"cat_sticker_enabled":true,
				"archive_enabled":true,
				"archive_chat_id":-100999,
				"archive_thread_id":77,
				"archive_threshold":1,
				"archive_emoji_mode":"invalid",
				"archive_delivery":"drop"
			}}}
		}`
		bkp, err := BackupFormatFromJSON([]byte(tamperedJSON))
		require.NoError(t, err)

		unlinkedReport, err := ImportChatDataReport(unlinkedChat, bkp, nil)
		require.NoError(t, err)
		assert.True(t, unlinkedReport.ArchiveChannelSkipped)

		unlinkedGot, err := presence.GetFresh(ctx, unlinkedChat)
		require.NoError(t, err)
		assert.Zero(t, unlinkedGot.ArchiveChatID)
		assert.Zero(t, unlinkedGot.ArchiveThreadID)
		assert.False(t, unlinkedGot.ArchiveEnabled)
		assert.True(t, unlinkedGot.ConsensusEnabled)
		assert.Equal(t, presence.DefaultConsensusThreshold, unlinkedGot.ConsensusThreshold)
		assert.Equal(t, presence.AIReactModeOff, unlinkedGot.AIReactMode)
		assert.Equal(t, presence.DefaultArchiveThreshold, unlinkedGot.ArchiveThreshold)
		assert.Equal(t, presence.ArchiveEmojiModePositive, unlinkedGot.ArchiveEmojiMode)
		assert.Equal(t, presence.ArchiveDeliveryCopy, unlinkedGot.ArchiveDelivery)
		assert.False(t, unlinkedGot.AmbientContext)
		assert.False(t, unlinkedGot.VocativeEnabled)
		assert.False(t, unlinkedGot.PMBriefReact)
		assert.False(t, unlinkedGot.ReferendumEnabled)
		assert.False(t, unlinkedGot.InspectEnabled)

		linkedReport, err := ImportChatDataReport(linkedChat, bkp, nil)
		require.NoError(t, err)
		assert.False(t, linkedReport.ArchiveChannelSkipped)

		linkedGot, err := presence.Get(ctx, linkedChat)
		require.NoError(t, err)
		assert.Equal(t, ownTarget, linkedGot.ArchiveChatID)
		assert.Equal(t, ownThread, linkedGot.ArchiveThreadID)
		assert.True(t, linkedGot.ArchiveEnabled)
		assert.True(t, linkedGot.CatStickerEnabled)
		assert.False(t, linkedGot.AmbientContext)
		assert.False(t, linkedGot.VocativeEnabled)
		assert.False(t, linkedGot.PMBriefReact)
		assert.False(t, linkedGot.ReferendumEnabled)
		assert.False(t, linkedGot.InspectEnabled)
	})

	t.Run("nil settings leaves target untouched and ClearModuleData resets row and cache", func(t *testing.T) {
		chatID := time.Now().UnixNano()
		require.NoError(t, chats.EnsureChatInDb(chatID, "presence_nil_and_clear"))
		t.Cleanup(func() { cleanupBackupChat(t, chatID) })

		require.NoError(t, presence.SetConsensusEnabled(ctx, chatID, true, 1))
		require.NoError(t, presence.SetArchiveTarget(ctx, chatID, -100123123, 12, 1))

		bkp := NewBackupFormat(chatID, "nil", 1, []string{BackupModulePresence})
		bkp.Data[BackupModulePresence] = map[string]any{}
		report, err := ImportChatDataReport(chatID, bkp, nil)
		require.NoError(t, err)
		assert.False(t, report.ArchiveChannelSkipped)

		kept, err := presence.Get(ctx, chatID)
		require.NoError(t, err)
		assert.True(t, kept.ConsensusEnabled)
		assert.Equal(t, int64(-100123123), kept.ArchiveChatID)

		require.NoError(t, ClearModuleData(chatID, BackupModulePresence))

		var count int64
		require.NoError(t, db.DB.Model(&models.ChatPresence{}).Where("chat_id = ?", chatID).Count(&count).Error)
		assert.Zero(t, count)

		cleared, err := presence.Get(ctx, chatID)
		require.NoError(t, err)
		assert.Equal(t, presence.Defaults(), cleared)
	})

	t.Run("rollback atomicity with failing sibling module", func(t *testing.T) {
		chatID := time.Now().UnixNano()
		require.NoError(t, chats.EnsureChatInDb(chatID, "presence_rollback"))
		t.Cleanup(func() { cleanupBackupChat(t, chatID) })

		require.NoError(t, presence.SetConsensusEnabled(ctx, chatID, false, 1))
		bkp := NewBackupFormat(chatID, "rb", 1, []string{BackupModulePresence, BackupModuleCaptcha})
		bkp.Data[BackupModulePresence] = map[string]any{
			"settings": map[string]any{
				"consensus_enabled": true,
			},
		}
		bkp.Data[BackupModuleCaptcha] = map[string]any{
			"settings": map[string]any{
				"enabled": true, "captcha_mode": "invalid", "timeout": 2,
				"failure_action": "kick", "max_attempts": 3,
			},
		}
		_, err := ImportChatDataReport(chatID, bkp, nil)
		require.Error(t, err)

		got, err := presence.GetFresh(ctx, chatID)
		require.NoError(t, err)
		assert.False(t, got.ConsensusEnabled)
	})
}

// TestBackupReactionsLegacyNormalization proves T-33-18: legacy v1.0-v1.2 reaction exports
// without Phase 33 fields normalize ChancePct == 0 to 100, clamp out-of-range option values,
// preserve explicit Phase 33 options, and invalidate both reactions:<chat> and
// reaction_rules:<chat> caches on import and clear.
func TestBackupReactionsLegacyNormalization(t *testing.T) {
	skipIfNoDb(t)
	utilscache.SetupTestMemoryMarshaler(t)
	ctx := context.Background()

	chatID := time.Now().UnixNano()
	require.NoError(t, chats.EnsureChatInDb(chatID, "reactions_legacy_norm"))
	t.Cleanup(func() { cleanupBackupChat(t, chatID) })

	// Seed stale rule and prime both reactions:<chat> and reaction_rules:<chat> caches.
	require.NoError(t, reactions.AddReaction(chatID, "old", "👍"))
	assert.Equal(t, map[string]string{"old": "👍"}, reactions.GetReactionsContext(ctx, chatID))
	staleRules, err := reactions.GetReactionRulesContext(ctx, chatID)
	require.NoError(t, err)
	require.Len(t, staleRules, 1)

	legacyJSON := fmt.Sprintf(`{
		"version":"1.0",
		"bot_name":"Xenobot-72",
		"chat_id":%d,
		"modules":["reactions"],
		"data":{"reactions":{"reactions":[
			{"keyword":"legacy","emoji":"🔥"},
			{"keyword":"oob","emoji":"🎉","chance_pct":150,"cooldown_secs":999999},
			{"keyword":"custom","emoji":"❤","word_boundary":true,"match_caption":true,"chance_pct":42,"cooldown_secs":300}
		]}}
	}`, chatID)

	bkp, err := BackupFormatFromJSON([]byte(legacyJSON))
	require.NoError(t, err)
	require.NoError(t, ImportChatData(chatID, bkp, nil))

	gotMap := reactions.GetReactionsContext(ctx, chatID)
	assert.Equal(t, map[string]string{
		"legacy": "🔥",
		"oob":    "🎉",
		"custom": "❤",
	}, gotMap)

	gotRules, err := reactions.GetReactionRulesContext(ctx, chatID)
	require.NoError(t, err)
	require.Len(t, gotRules, 3)

	byKeyword := make(map[string]reactions.Rule, len(gotRules))
	for _, r := range gotRules {
		byKeyword[r.Keyword] = r
	}

	assert.Equal(t, 100, byKeyword["legacy"].ChancePct)
	assert.False(t, byKeyword["legacy"].WordBoundary)
	assert.False(t, byKeyword["legacy"].MatchCaption)
	assert.Equal(t, 0, byKeyword["legacy"].CooldownSecs)

	assert.Equal(t, 100, byKeyword["oob"].ChancePct)
	assert.Equal(t, 0, byKeyword["oob"].CooldownSecs)

	assert.True(t, byKeyword["custom"].WordBoundary)
	assert.True(t, byKeyword["custom"].MatchCaption)
	assert.Equal(t, 42, byKeyword["custom"].ChancePct)
	assert.Equal(t, 300, byKeyword["custom"].CooldownSecs)

	// ClearModuleData must invalidate both reactions and reaction_rules caches.
	require.NoError(t, ClearModuleData(chatID, BackupModuleReactions))
	assert.Empty(t, reactions.GetReactionsContext(ctx, chatID))
	clearedRules, err := reactions.GetReactionRulesContext(ctx, chatID)
	require.NoError(t, err)
	assert.Empty(t, clearedRules)
}
