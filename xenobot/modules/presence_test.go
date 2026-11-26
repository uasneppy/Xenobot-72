//go:build testtools

package modules

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/db/presence"
	utilsCache "github.com/uasneppy/Xenobot-72/xenobot/utils/cache"
)

func presenceCleanup(t *testing.T, chatID int64) {
	t.Helper()
	t.Cleanup(func() {
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatPresence{})
		cache.DeleteCache(presence.CacheKey(chatID))
	})
}

func TestPresenceSafetyGate(t *testing.T) {
	t.Run("chat type allowed", func(t *testing.T) {
		cases := []struct {
			name string
			chat *gotgbot.Chat
			want bool
		}{
			{"nil chat", nil, false},
			{"private", &gotgbot.Chat{Id: 100, Type: "private"}, false},
			{"channel", &gotgbot.Chat{Id: -1001, Type: "channel"}, false},
			{"group", &gotgbot.Chat{Id: -1002, Type: "group"}, true},
			{"supergroup", &gotgbot.Chat{Id: -1003, Type: "supergroup"}, true},
		}
		for _, tc := range cases {
			if got := presenceChatTypeAllowed(tc.chat); got != tc.want {
				t.Errorf("%s: presenceChatTypeAllowed() = %v, want %v", tc.name, got, tc.want)
			}
		}
	})

	t.Run("message allowed", func(t *testing.T) {
		bot := newModuleTestBot(newModuleBotClient())
		groupChat := gotgbot.Chat{Id: -100555001, Type: "supergroup", Title: "Lab"}
		staffChat := gotgbot.Chat{Id: -100555002, Type: "supergroup", Title: "Staff"}
		member := &gotgbot.User{Id: 42001, FirstName: "Alice"}

		lockdownCalls := 0
		chatSwap(t, &presenceStaffLookup, func(chatID int64) *models.StaffGroup {
			if chatID == staffChat.Id {
				return &models.StaffGroup{ChatID: staffChat.Id}
			}
			return nil
		})
		chatSwap(t, &presenceLockdownLookup, func(chatID int64) (*models.ChatLockdown, error) {
			lockdownCalls++
			return nil, nil
		})

		ctxOf := func(msg *gotgbot.Message) *ext.Context {
			return ext.NewContext(bot, &gotgbot.Update{UpdateId: 1, Message: msg}, nil)
		}

		cases := []struct {
			name   string
			ctx    *ext.Context
			wantOK bool
			wantID int64
		}{
			{
				name:   "ordinary group member",
				ctx:    ctxOf(&gotgbot.Message{MessageId: 1, Chat: groupChat, From: member, Text: "hello team"}),
				wantOK: true,
				wantID: member.Id,
			},
			{
				name:   "ordinary legacy group member",
				ctx:    ctxOf(&gotgbot.Message{MessageId: 2, Chat: gotgbot.Chat{Id: -555003, Type: "group"}, From: member, Text: "hi"}),
				wantOK: true,
				wantID: member.Id,
			},
			{
				name:   "private chat rejected",
				ctx:    ctxOf(&gotgbot.Message{MessageId: 3, Chat: gotgbot.Chat{Id: member.Id, Type: "private"}, From: member, Text: "hello"}),
				wantOK: false,
			},
			{
				name:   "channel rejected",
				ctx:    ctxOf(&gotgbot.Message{MessageId: 4, Chat: gotgbot.Chat{Id: -100999, Type: "channel"}, From: member, Text: "post"}),
				wantOK: false,
			},
			{
				name:   "slash command rejected",
				ctx:    ctxOf(&gotgbot.Message{MessageId: 5, Chat: groupChat, From: member, Text: "/help"}),
				wantOK: false,
			},
			{
				name: "bot_command entity rejected",
				ctx: ctxOf(&gotgbot.Message{
					MessageId: 6, Chat: groupChat, From: member, Text: "/ping",
					Entities: []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: 5}},
				}),
				wantOK: false,
			},
			{
				name: "anonymous channel sender_chat rejected",
				ctx: ctxOf(&gotgbot.Message{
					MessageId:  7,
					Chat:       groupChat,
					From:       &gotgbot.User{Id: 136817688, FirstName: "Channel", IsBot: true},
					SenderChat: &gotgbot.Chat{Id: -100777, Type: "channel"},
					Text:       "hello from channel",
				}),
				wantOK: false,
			},
			{
				name: "anonymous group admin sender_chat rejected",
				ctx: ctxOf(&gotgbot.Message{
					MessageId:  8,
					Chat:       groupChat,
					From:       &gotgbot.User{Id: 1087968824, FirstName: "Group", IsBot: true},
					SenderChat: &gotgbot.Chat{Id: groupChat.Id, Type: "supergroup"},
					Text:       "hello anonymously",
				}),
				wantOK: false,
			},
			{
				name:   "bot sender rejected",
				ctx:    ctxOf(&gotgbot.Message{MessageId: 9, Chat: groupChat, From: &gotgbot.User{Id: 88801, FirstName: "HelperBot", IsBot: true}, Text: "beep"}),
				wantOK: false,
			},
			{
				name:   "staff group rejected",
				ctx:    ctxOf(&gotgbot.Message{MessageId: 10, Chat: staffChat, From: member, Text: "mod talk"}),
				wantOK: false,
			},
		}

		for _, tc := range cases {
			gotID, gotOK := presenceMessageAllowed(bot, tc.ctx)
			if gotOK != tc.wantOK || gotID != tc.wantID {
				t.Errorf("%s: presenceMessageAllowed() = (%d, %v), want (%d, %v)", tc.name, gotID, gotOK, tc.wantID, tc.wantOK)
			}
		}
		if lockdownCalls != 0 {
			t.Errorf("presenceMessageAllowed called lockdown lookup %d times, want 0 (must stay lazy)", lockdownCalls)
		}
	})

	t.Run("lockdown fail closed", func(t *testing.T) {
		const chatID int64 = -100555010

		chatSwap(t, &presenceLockdownLookup, func(id int64) (*models.ChatLockdown, error) {
			if id != chatID {
				t.Errorf("lockdown lookup chatID = %d, want %d", id, chatID)
			}
			return nil, nil
		})
		if presenceIsLockedDown(chatID) {
			t.Error("no active lockdown: got true, want false")
		}

		chatSwap(t, &presenceLockdownLookup, func(int64) (*models.ChatLockdown, error) {
			return &models.ChatLockdown{ChatID: chatID, State: models.LockdownStateActive}, nil
		})
		if !presenceIsLockedDown(chatID) {
			t.Error("active lockdown row: got false, want true")
		}

		chatSwap(t, &presenceLockdownLookup, func(int64) (*models.ChatLockdown, error) {
			return nil, errors.New("postgres timeout")
		})
		if !presenceIsLockedDown(chatID) {
			t.Error("lockdown DB error: got false, want true (fail closed)")
		}
	})

	t.Run("anonymous group admin", func(t *testing.T) {
		chat := gotgbot.Chat{Id: -100555020, Type: "supergroup"}
		cases := []struct {
			name string
			mru  *gotgbot.MessageReactionUpdated
			want bool
		}{
			{"nil update", nil, false},
			{
				name: "normal user",
				mru: &gotgbot.MessageReactionUpdated{
					Chat: chat,
					User: &gotgbot.User{Id: 123, FirstName: "Bob"},
				},
				want: false,
			},
			{
				name: "external linked channel",
				mru: &gotgbot.MessageReactionUpdated{
					Chat:      chat,
					ActorChat: &gotgbot.Chat{Id: -100999999, Type: "channel"},
				},
				want: false,
			},
			{
				name: "both user and actor_chat nil",
				mru: &gotgbot.MessageReactionUpdated{
					Chat: chat,
				},
				want: false,
			},
			{
				name: "user and matching actor_chat both set",
				mru: &gotgbot.MessageReactionUpdated{
					Chat:      chat,
					User:      &gotgbot.User{Id: 123, FirstName: "Bob"},
					ActorChat: &gotgbot.Chat{Id: chat.Id, Type: "supergroup"},
				},
				want: false,
			},
			{
				name: "anonymous group admin",
				mru: &gotgbot.MessageReactionUpdated{
					Chat:      chat,
					ActorChat: &gotgbot.Chat{Id: chat.Id, Type: "supergroup"},
				},
				want: true,
			},
		}
		for _, tc := range cases {
			if got := presenceIsAnonGroupAdmin(tc.mru); got != tc.want {
				t.Errorf("%s: presenceIsAnonGroupAdmin() = %v, want %v", tc.name, got, tc.want)
			}
		}
	})
}

func TestPresenceKeys(t *testing.T) {
	const (
		chatID int64 = -1001234567890
		msgID  int64 = 456
		userID int64 = 789
	)
	emoji := "🔥"
	wantTag := "{-1001234567890}"

	keys := map[string]struct {
		got  string
		want string
	}{
		"bot_react": {
			got:  presenceBotReactKey(chatID, msgID),
			want: "xenobot:presence:{-1001234567890}:bot_react:456",
		},
		"user_cooldown": {
			got:  presenceUserCooldownKey(chatID, userID),
			want: "xenobot:presence:{-1001234567890}:cd:u:789",
		},
		"consensus_users": {
			got:  presenceConsensusUsersKey(chatID, msgID, emoji),
			want: "xenobot:presence:{-1001234567890}:cons:u:456:" + hex.EncodeToString([]byte(emoji)),
		},
		"consensus_cooldown": {
			got:  presenceConsensusCooldownKey(chatID),
			want: "xenobot:presence:{-1001234567890}:cons:cd",
		},
		"consensus_done": {
			got:  presenceConsensusDoneKey(chatID, msgID),
			want: "xenobot:presence:{-1001234567890}:cons:done:456",
		},
	}

	for name, tc := range keys {
		if tc.got != tc.want {
			t.Errorf("%s: got %q, want %q", name, tc.got, tc.want)
		}
		if !strings.HasPrefix(tc.got, presenceKeyPrefix) {
			t.Errorf("%s: key %q does not have prefix %q", name, tc.got, presenceKeyPrefix)
		}
		if strings.HasPrefix(tc.got, utilsCache.DataCachePrefix) {
			t.Errorf("%s: key %q must sit outside %q", name, tc.got, utilsCache.DataCachePrefix)
		}
		if !strings.Contains(tc.got, wantTag) {
			t.Errorf("%s: key %q missing Cluster hash tag %q", name, tc.got, wantTag)
		}
	}
}

func TestPresenceApply(t *testing.T) {
	ctx := context.Background()

	t.Run("write and read back", func(t *testing.T) {
		chatID := uniqueModuleChatID()
		presenceCleanup(t, chatID)

		r := applyPresenceConsensusEnabled(ctx, chatID, true, 10)
		if r.Outcome != presenceApplied || r.Settings == nil || !r.Settings.ConsensusEnabled || r.Settings.ConsensusThreshold != 3 || !r.Settings.RetractEnabled {
			t.Fatalf("ConsensusEnabled true: got %+v", r)
		}

		r = applyPresenceConsensusThreshold(ctx, chatID, 7, 10)
		if r.Outcome != presenceApplied || r.Settings == nil || r.Settings.ConsensusThreshold != 7 || !r.Settings.ConsensusEnabled {
			t.Fatalf("ConsensusThreshold 7: got %+v", r)
		}

		r = applyPresenceRetractEnabled(ctx, chatID, false, 10)
		if r.Outcome != presenceApplied || r.Settings == nil || r.Settings.RetractEnabled || r.Settings.ConsensusThreshold != 7 {
			t.Fatalf("RetractEnabled false: got %+v", r)
		}
	})

	t.Run("invalid threshold writes nothing", func(t *testing.T) {
		chatID := uniqueModuleChatID()
		presenceCleanup(t, chatID)

		writeCalls := 0
		chatSwap(t, &presenceWrites.ConsensusThreshold, func(ctx context.Context, id int64, threshold int, by int64) error {
			writeCalls++
			return presence.SetConsensusThreshold(ctx, id, threshold, by)
		})

		for _, bad := range []int{-5, 0, 1, 21, 100} {
			r := applyPresenceConsensusThreshold(ctx, chatID, bad, 10)
			if r.Outcome != presenceInvalidValue || r.Settings != nil || r.Err != nil {
				t.Errorf("threshold %d: got %+v, want presenceInvalidValue", bad, r)
			}
		}
		if writeCalls != 0 {
			t.Errorf("writer called %d times for invalid thresholds, want 0", writeCalls)
		}
	})

	t.Run("save failure returns presenceSaveFailed", func(t *testing.T) {
		chatID := uniqueModuleChatID()
		presenceCleanup(t, chatID)
		boom := errors.New("db down")

		chatSwap(t, &presenceWrites.ConsensusEnabled, func(context.Context, int64, bool, int64) error { return boom })
		chatSwap(t, &presenceWrites.ConsensusThreshold, func(context.Context, int64, int, int64) error { return boom })
		chatSwap(t, &presenceWrites.RetractEnabled, func(context.Context, int64, bool, int64) error { return boom })

		if r := applyPresenceConsensusEnabled(ctx, chatID, true, 10); r.Outcome != presenceSaveFailed || !errors.Is(r.Err, boom) {
			t.Errorf("ConsensusEnabled failure: got %+v", r)
		}
		if r := applyPresenceConsensusThreshold(ctx, chatID, 5, 10); r.Outcome != presenceSaveFailed || !errors.Is(r.Err, boom) {
			t.Errorf("ConsensusThreshold failure: got %+v", r)
		}
		if r := applyPresenceRetractEnabled(ctx, chatID, false, 10); r.Outcome != presenceSaveFailed || !errors.Is(r.Err, boom) {
			t.Errorf("RetractEnabled failure: got %+v", r)
		}
	})

	t.Run("read-back failure keeps presenceApplied", func(t *testing.T) {
		chatID := uniqueModuleChatID()
		presenceCleanup(t, chatID)

		chatSwap(t, &presenceReadFresh, func(context.Context, int64) (presence.Settings, error) {
			return presence.Settings{}, errors.New("read failed")
		})
		r := applyPresenceConsensusEnabled(ctx, chatID, true, 10)
		if r.Outcome != presenceApplied || r.Settings != nil {
			t.Fatalf("read-back failure: got %+v, want presenceApplied with nil Settings", r)
		}
	})
}
