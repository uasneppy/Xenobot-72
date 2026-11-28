package modules

import (
	"context"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/Xenobot-72/xenobot/db/presence"
)

func archiveFixSettings(t *testing.T, targetChatID int64) {
	t.Helper()
	chatSwap(t, &presenceSettingsGet, func(context.Context, int64) (presence.Settings, error) {
		return presence.Settings{
			ArchiveEnabled:   true,
			ArchiveChatID:    targetChatID,
			ArchiveThreshold: 2,
			ArchiveEmojiMode: presence.ArchiveEmojiModePositive,
			ArchiveDelivery:  presence.ArchiveDeliveryCopy,
		}, nil
	})
}

// TestPresenceArchiveDeliveryFailureReleasesClaim: the durable claim and the Redis latch are written
// before the send, so a send that fails for a reason that may pass (a 5xx, a timeout) must give them
// back, or that highlight is lost for good. A definitive refusal (a 4xx) keeps them: retrying
// cannot succeed.
func TestPresenceArchiveDeliveryFailureReleasesClaim(t *testing.T) {
	const targetChatID int64 = -100999000222

	t.Run("a 502 releases the claim and the next reaction delivers", func(t *testing.T) {
		env := newArchiveReactionEnv(t)
		chatID := uniqueModuleChatID()
		presenceArchiveCleanup(t, chatID)
		chat := gotgbot.Chat{Id: chatID, Type: "supergroup"}
		archiveFixSettings(t, targetChatID)
		const msgID int64 = 61

		env.mu.Lock()
		env.copyCode, env.copyDesc = 502, "Bad Gateway"
		env.mu.Unlock()
		env.react(chat, msgID, &gotgbot.User{Id: 911, FirstName: "A"}, nil, []string{"🔥"})
		env.react(chat, msgID, &gotgbot.User{Id: 912, FirstName: "B"}, nil, []string{"👍"})
		if got := len(env.callsFor("copyMessage")); got != 1 {
			t.Fatalf("failed delivery attempts = %d, want 1", got)
		}
		if env.mr.Exists(presenceArchiveDoneKey(chatID, msgID)) {
			t.Fatal("a failed delivery left the Redis latch behind")
		}
		if claimed, err := presence.TryClaimArchived(context.Background(), chatID, msgID); err != nil || !claimed {
			t.Fatalf("a failed delivery left the durable claim behind (claimed=%v err=%v)", claimed, err)
		}
		if _, err := presence.ReleaseArchived(context.Background(), chatID, msgID); err != nil {
			t.Fatal(err)
		}

		env.mu.Lock()
		env.copyCode = 0
		env.mu.Unlock()
		env.react(chat, msgID, &gotgbot.User{Id: 913, FirstName: "C"}, nil, []string{"❤"})
		if got := len(env.callsFor("copyMessage")); got != 2 {
			t.Fatalf("delivery after the failure: copy calls = %d, want 2", got)
		}
		if !env.mr.Exists(presenceArchiveDoneKey(chatID, msgID)) {
			t.Fatal("a delivered highlight must set the Redis latch")
		}
	})

	t.Run("a definitive 400 keeps the claim", func(t *testing.T) {
		env := newArchiveReactionEnv(t)
		chatID := uniqueModuleChatID()
		presenceArchiveCleanup(t, chatID)
		chat := gotgbot.Chat{Id: chatID, Type: "supergroup"}
		archiveFixSettings(t, targetChatID)
		const msgID int64 = 62

		env.mu.Lock()
		env.copyCode, env.copyDesc = 400, "Bad Request: message to copy not found"
		env.mu.Unlock()
		env.react(chat, msgID, &gotgbot.User{Id: 921, FirstName: "A"}, nil, []string{"🔥"})
		env.react(chat, msgID, &gotgbot.User{Id: 922, FirstName: "B"}, nil, []string{"👍"})
		env.react(chat, msgID, &gotgbot.User{Id: 923, FirstName: "C"}, nil, []string{"❤"})
		if got := len(env.callsFor("copyMessage")); got != 1 {
			t.Fatalf("copy calls = %d, want 1: a definitive refusal is not retried", got)
		}
		if !env.mr.Exists(presenceArchiveDoneKey(chatID, msgID)) {
			t.Fatal("a definitive refusal must keep the latch")
		}
	})
}
