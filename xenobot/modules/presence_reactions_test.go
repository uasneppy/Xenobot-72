//go:build testtools

package modules

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/uasneppy/Xenobot-72/xenobot/db/chatconv"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/db/presence"
	utilsCache "github.com/uasneppy/Xenobot-72/xenobot/utils/cache"
)

type presenceWireCall struct {
	Path   string
	Params map[string]string
}

type presenceReactionTestEnv struct {
	t          *testing.T
	chatID     int64
	mr         *miniredis.Miniredis
	rc         *redis.Client
	bot        *gotgbot.Bot
	dispatcher *ext.Dispatcher
	nextUpdate atomic.Int64

	mu       sync.Mutex
	calls    []presenceWireCall
	parseErr error
	// availJSON, when set, is the available_reactions array getChat answers with.
	availJSON string
	// reactFail, when set, is the description setMessageReaction fails with (a 400).
	reactFail string
}

func newPresenceReactionEnv(t *testing.T, chatID int64) *presenceReactionTestEnv {
	t.Helper()
	withFastPresencePacer(t)
	utilsCache.SetupTestMemoryMarshaler(t)
	mr, rc := withChatRedis(t)
	chatJobs.wait()
	t.Cleanup(chatJobs.wait)

	// Default clean safety seams; individual tests override as needed.
	chatSwap(t, &presenceStaffLookup, func(int64) *models.StaffGroup { return nil })
	chatSwap(t, &presenceLockdownLookup, func(int64) (*models.ChatLockdown, error) { return nil, nil })
	chatSwap(t, &presenceIsAdmin, func(*gotgbot.Bot, int64, int64) bool { return false })
	chatSwap(t, &presenceConsensusCooldown, presenceConsensusCooldown)

	env := &presenceReactionTestEnv{
		t:      t,
		chatID: chatID,
		mr:     mr,
		rc:     rc,
	}
	env.bot = newWireTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/bottest/getChat":
			params, err := decodeTelegramWireParams(r)
			env.recordCall(r.URL.Path, params, err)
			env.mu.Lock()
			avail := env.availJSON
			env.mu.Unlock()
			extra := ""
			if avail != "" {
				extra = `,"available_reactions":` + avail
			}
			_, _ = w.Write([]byte(`{"ok":true,"result":{"id":` + params["chat_id"] + `,"type":"supergroup"` + extra + `}}`))
		case "/bottest/setMessageReaction":
			params, err := decodeTelegramWireParams(r)
			env.recordCall(r.URL.Path, params, err)
			env.mu.Lock()
			fail := env.reactFail
			env.mu.Unlock()
			if fail != "" {
				_, _ = w.Write([]byte(`{"ok":false,"error_code":400,"description":"Bad Request: ` + fail + `"}`))
				return
			}
			_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
		default:
			params, err := decodeTelegramWireParams(r)
			env.recordCall(r.URL.Path, params, err)
			_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
		}
	})
	env.dispatcher = ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadPresence(env.dispatcher)
	return env
}

func (e *presenceReactionTestEnv) recordCall(path string, params map[string]string, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err != nil && e.parseErr == nil {
		e.parseErr = err
	}
	e.calls = append(e.calls, presenceWireCall{Path: path, Params: params})
}

func (e *presenceReactionTestEnv) callsFor(method string) []presenceWireCall {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.parseErr != nil {
		e.t.Fatalf("decodeTelegramWireParams failed: %v", e.parseErr)
	}
	want := "/bottest/" + method
	var out []presenceWireCall
	for _, c := range e.calls {
		if c.Path == want {
			out = append(out, c)
		}
	}
	return out
}

func (e *presenceReactionTestEnv) allCalls() []presenceWireCall {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.parseErr != nil {
		e.t.Fatalf("decodeTelegramWireParams failed: %v", e.parseErr)
	}
	return append([]presenceWireCall(nil), e.calls...)
}

func (e *presenceReactionTestEnv) dispatchReaction(mru *gotgbot.MessageReactionUpdated) {
	e.t.Helper()
	e.dispatchReactionAs(e.nextUpdate.Add(1), mru)
}

// dispatchReactionAs delivers a reaction update with an explicit update_id, to replay updates out of
// order.
func (e *presenceReactionTestEnv) dispatchReactionAs(updateID int64, mru *gotgbot.MessageReactionUpdated) {
	e.t.Helper()
	upd := &gotgbot.Update{
		UpdateId:        updateID,
		MessageReaction: mru,
	}
	if err := e.dispatcher.ProcessUpdate(e.bot, upd, nil); err != nil {
		e.t.Fatalf("ProcessUpdate(MessageReaction): %v", err)
	}
}

func emojiReactions(emojis ...string) []gotgbot.ReactionType {
	out := make([]gotgbot.ReactionType, 0, len(emojis))
	for _, emoji := range emojis {
		out = append(out, gotgbot.ReactionTypeEmoji{Emoji: emoji})
	}
	return out
}

func TestPresenceDiffReactions(t *testing.T) {
	cases := []struct {
		name        string
		oldList     []gotgbot.ReactionType
		newList     []gotgbot.ReactionType
		wantAdded   []string
		wantRemoved []string
	}{
		{
			name:        "both empty",
			oldList:     nil,
			newList:     nil,
			wantAdded:   nil,
			wantRemoved: nil,
		},
		{
			name:        "FE0F heart equals normalized heart (no diff)",
			oldList:     emojiReactions("❤️"),
			newList:     emojiReactions("❤"),
			wantAdded:   nil,
			wantRemoved: nil,
		},
		{
			name:        "FE0F heart added returns normalized heart",
			oldList:     nil,
			newList:     emojiReactions("❤️"),
			wantAdded:   []string{"❤"},
			wantRemoved: nil,
		},
		{
			name: "filters custom, paid, and unsupported emojis",
			oldList: []gotgbot.ReactionType{
				gotgbot.ReactionTypeCustomEmoji{CustomEmojiId: "123"},
				gotgbot.ReactionTypePaid{},
				gotgbot.ReactionTypeEmoji{Emoji: "😂"},
			},
			newList: []gotgbot.ReactionType{
				gotgbot.ReactionTypeCustomEmoji{CustomEmojiId: "456"},
				gotgbot.ReactionTypeEmoji{Emoji: "😂"},
				gotgbot.ReactionTypeEmoji{Emoji: "🔥"},
			},
			wantAdded:   []string{"🔥"},
			wantRemoved: nil,
		},
		{
			name:        "deterministic add, keep, and remove diff",
			oldList:     emojiReactions("👍", "🔥", "🎉"),
			newList:     emojiReactions("🔥", "👏", "❤️"),
			wantAdded:   []string{"👏", "❤"},
			wantRemoved: []string{"👍", "🎉"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			added, removed := presenceDiffReactions(tc.oldList, tc.newList)
			if !slices.Equal(added, tc.wantAdded) || !slices.Equal(removed, tc.wantRemoved) {
				t.Errorf("presenceDiffReactions() = (added=%v, removed=%v), want (added=%v, removed=%v)",
					added, removed, tc.wantAdded, tc.wantRemoved)
			}
		})
	}
}

func TestPresenceRetract(t *testing.T) {
	const (
		chatID   int64 = -100888001
		msgID    int64 = 500
		authorID int64 = 1001
		adminID  int64 = 2002
		otherID  int64 = 3003
	)
	groupChat := gotgbot.Chat{Id: chatID, Type: "supergroup"}

	seedBotReact := func(env *presenceReactionTestEnv) {
		presenceWriteBotReact(context.Background(), env.rc, chatID, msgID, presenceBotReact{
			Emoji:    "🔥",
			AuthorID: authorID,
			Source:   "cons",
		})
	}

	retractEnabledSettings := func(enabled bool) {
		chatSwap(t, &presenceSettingsGet, func(context.Context, int64) (presence.Settings, error) {
			return presence.Settings{RetractEnabled: enabled, ConsensusThreshold: 3}, nil
		})
	}

	t.Run("message author retracts silently with reaction=[] and zero getChatMember", func(t *testing.T) {
		env := newPresenceReactionEnv(t, chatID)
		retractEnabledSettings(true)
		seedBotReact(env)

		var adminChecks atomic.Int32
		chatSwap(t, &presenceIsAdmin, func(*gotgbot.Bot, int64, int64) bool {
			adminChecks.Add(1)
			return true
		})

		env.dispatchReaction(&gotgbot.MessageReactionUpdated{
			Chat:        groupChat,
			MessageId:   msgID,
			User:        &gotgbot.User{Id: authorID, FirstName: "Author"},
			NewReaction: emojiReactions("👎"),
		})

		if got := adminChecks.Load(); got != 0 {
			t.Errorf("presenceIsAdmin calls = %d, want 0 for message author", got)
		}
		if sends := env.callsFor("sendMessage"); len(sends) != 0 {
			t.Errorf("sendMessage calls = %d, want 0", len(sends))
		}
		clears := env.callsFor("setMessageReaction")
		if len(clears) != 1 {
			t.Fatalf("setMessageReaction calls = %d, want 1", len(clears))
		}
		if clears[0].Params["reaction"] != "[]" {
			t.Fatalf("wire reaction = %q, want \"[]\"", clears[0].Params["reaction"])
		}
		wireJSON, err := json.Marshal(map[string]json.RawMessage{"reaction": json.RawMessage(clears[0].Params["reaction"])})
		if err != nil || !strings.Contains(string(wireJSON), `"reaction":[]`) {
			t.Fatalf("wireJSON = %s (err=%v), want \"reaction\":[]", wireJSON, err)
		}
		if env.mr.Exists(presenceBotReactKey(chatID, msgID)) {
			t.Error("bot_react key still present after retraction")
		}
		cdKey := presenceUserCooldownKey(chatID, authorID)
		if strings.HasPrefix(cdKey, utilsCache.DataCachePrefix) {
			t.Fatalf("user cooldown key %q must live outside %q", cdKey, utilsCache.DataCachePrefix)
		}
		if !env.mr.Exists(cdKey) {
			t.Errorf("user cooldown key %q missing after retraction", cdKey)
		}
		if ttl := env.mr.TTL(cdKey); ttl < 23*time.Hour || ttl > 24*time.Hour {
			t.Errorf("user cooldown TTL = %v, want ~24h", ttl)
		}
	})

	t.Run("group admin (non-author) retracts and extends admin cooldown", func(t *testing.T) {
		env := newPresenceReactionEnv(t, chatID)
		retractEnabledSettings(true)
		seedBotReact(env)

		var adminChecks atomic.Int32
		chatSwap(t, &presenceIsAdmin, func(_ *gotgbot.Bot, cid, uid int64) bool {
			adminChecks.Add(1)
			return cid == chatID && uid == adminID
		})

		env.dispatchReaction(&gotgbot.MessageReactionUpdated{
			Chat:        groupChat,
			MessageId:   msgID,
			User:        &gotgbot.User{Id: adminID, FirstName: "Admin"},
			NewReaction: emojiReactions("👎"),
		})

		if got := adminChecks.Load(); got != 1 {
			t.Errorf("presenceIsAdmin calls = %d, want 1", got)
		}
		clears := env.callsFor("setMessageReaction")
		if len(clears) != 1 || clears[0].Params["reaction"] != "[]" {
			t.Fatalf("setMessageReaction calls = %+v, want 1 clear with reaction=[]", clears)
		}
		if !env.mr.Exists(presenceUserCooldownKey(chatID, adminID)) {
			t.Error("admin cooldown key not extended")
		}
		if sends := env.callsFor("sendMessage"); len(sends) != 0 {
			t.Errorf("sendMessage calls = %d, want 0", len(sends))
		}
	})

	t.Run("anonymous group admin retracts without getChatMember or nil User panic", func(t *testing.T) {
		env := newPresenceReactionEnv(t, chatID)
		retractEnabledSettings(true)
		seedBotReact(env)

		var adminChecks atomic.Int32
		chatSwap(t, &presenceIsAdmin, func(*gotgbot.Bot, int64, int64) bool {
			adminChecks.Add(1)
			return true
		})

		env.dispatchReaction(&gotgbot.MessageReactionUpdated{
			Chat:        groupChat,
			MessageId:   msgID,
			User:        nil,
			ActorChat:   &gotgbot.Chat{Id: chatID, Type: "supergroup"},
			NewReaction: emojiReactions("👎"),
		})

		if got := adminChecks.Load(); got != 0 {
			t.Errorf("presenceIsAdmin calls = %d, want 0 for anonymous group admin", got)
		}
		clears := env.callsFor("setMessageReaction")
		if len(clears) != 1 || clears[0].Params["reaction"] != "[]" {
			t.Fatalf("setMessageReaction calls = %+v, want 1 clear with reaction=[]", clears)
		}
		if env.mr.Exists(presenceBotReactKey(chatID, msgID)) {
			t.Error("bot_react key still present after anonymous admin retraction")
		}
		if sends := env.callsFor("sendMessage"); len(sends) != 0 {
			t.Errorf("sendMessage calls = %d, want 0", len(sends))
		}
	})

	t.Run("external channel is ignored", func(t *testing.T) {
		env := newPresenceReactionEnv(t, chatID)
		retractEnabledSettings(true)
		seedBotReact(env)

		var adminChecks atomic.Int32
		chatSwap(t, &presenceIsAdmin, func(*gotgbot.Bot, int64, int64) bool {
			adminChecks.Add(1)
			return true
		})

		env.dispatchReaction(&gotgbot.MessageReactionUpdated{
			Chat:        groupChat,
			MessageId:   msgID,
			User:        nil,
			ActorChat:   &gotgbot.Chat{Id: -100999888, Type: "channel"},
			NewReaction: emojiReactions("👎"),
		})

		if got := adminChecks.Load(); got != 0 {
			t.Errorf("presenceIsAdmin calls = %d, want 0", got)
		}
		if len(env.allCalls()) != 0 {
			t.Errorf("Telegram calls = %d, want 0", len(env.allCalls()))
		}
		if !env.mr.Exists(presenceBotReactKey(chatID, msgID)) {
			t.Error("external channel deleted bot_react key")
		}
	})

	t.Run("non-author regular member is ignored and keeps bot_react intact", func(t *testing.T) {
		env := newPresenceReactionEnv(t, chatID)
		retractEnabledSettings(true)
		seedBotReact(env)

		var adminChecks atomic.Int32
		chatSwap(t, &presenceIsAdmin, func(*gotgbot.Bot, int64, int64) bool {
			adminChecks.Add(1)
			return false
		})

		env.dispatchReaction(&gotgbot.MessageReactionUpdated{
			Chat:        groupChat,
			MessageId:   msgID,
			User:        &gotgbot.User{Id: otherID, FirstName: "Troll"},
			NewReaction: emojiReactions("👎"),
		})

		if got := adminChecks.Load(); got != 1 {
			t.Errorf("presenceIsAdmin calls = %d, want 1", got)
		}
		if len(env.callsFor("setMessageReaction")) != 0 {
			t.Errorf("setMessageReaction calls = %d, want 0", len(env.callsFor("setMessageReaction")))
		}
		if !env.mr.Exists(presenceBotReactKey(chatID, msgID)) {
			t.Error("non-author regular member deleted bot_react key")
		}
	})

	t.Run("thumbs-down on non-bot-reacted message makes zero Telegram calls", func(t *testing.T) {
		env := newPresenceReactionEnv(t, chatID)
		retractEnabledSettings(true)

		var adminChecks atomic.Int32
		chatSwap(t, &presenceIsAdmin, func(*gotgbot.Bot, int64, int64) bool {
			adminChecks.Add(1)
			return true
		})

		env.dispatchReaction(&gotgbot.MessageReactionUpdated{
			Chat:        groupChat,
			MessageId:   msgID + 99,
			User:        &gotgbot.User{Id: otherID, FirstName: "Member"},
			NewReaction: emojiReactions("👎"),
		})

		if got := adminChecks.Load(); got != 0 {
			t.Errorf("presenceIsAdmin calls = %d, want 0 when bot_react is absent", got)
		}
		if len(env.allCalls()) != 0 {
			t.Errorf("Telegram calls = %d, want 0 when bot_react is absent", len(env.allCalls()))
		}
	})

	t.Run("RetractEnabled false is ignored", func(t *testing.T) {
		env := newPresenceReactionEnv(t, chatID)
		retractEnabledSettings(false)
		seedBotReact(env)

		env.dispatchReaction(&gotgbot.MessageReactionUpdated{
			Chat:        groupChat,
			MessageId:   msgID,
			User:        &gotgbot.User{Id: authorID, FirstName: "Author"},
			NewReaction: emojiReactions("👎"),
		})

		if len(env.allCalls()) != 0 {
			t.Errorf("Telegram calls = %d, want 0 when RetractEnabled is false", len(env.allCalls()))
		}
		if !env.mr.Exists(presenceBotReactKey(chatID, msgID)) {
			t.Error("bot_react deleted while RetractEnabled is false")
		}
	})

	t.Run("Staff Group is ignored even when bot_react exists", func(t *testing.T) {
		env := newPresenceReactionEnv(t, chatID)
		retractEnabledSettings(true)
		seedBotReact(env)
		chatSwap(t, &presenceStaffLookup, func(id int64) *models.StaffGroup {
			if id == chatID {
				return &models.StaffGroup{ChatID: chatID}
			}
			return nil
		})

		env.dispatchReaction(&gotgbot.MessageReactionUpdated{
			Chat:        groupChat,
			MessageId:   msgID,
			User:        &gotgbot.User{Id: authorID, FirstName: "Author"},
			NewReaction: emojiReactions("👎"),
		})

		if len(env.allCalls()) != 0 {
			t.Errorf("Telegram calls = %d, want 0 in Staff Group", len(env.allCalls()))
		}
		if !env.mr.Exists(presenceBotReactKey(chatID, msgID)) {
			t.Error("bot_react deleted in Staff Group")
		}
	})

	t.Run("concurrent and second thumbs-down taps only clear once", func(t *testing.T) {
		env := newPresenceReactionEnv(t, chatID)
		retractEnabledSettings(true)
		seedBotReact(env)
		chatSwap(t, &presenceIsAdmin, func(*gotgbot.Bot, int64, int64) bool { return true })

		start := make(chan struct{})
		var wg sync.WaitGroup
		for _, uid := range []int64{authorID, adminID} {
			wg.Add(1)
			go func(id int64) {
				defer wg.Done()
				<-start
				env.dispatchReaction(&gotgbot.MessageReactionUpdated{
					Chat:        groupChat,
					MessageId:   msgID,
					User:        &gotgbot.User{Id: id, FirstName: "Tapper"},
					NewReaction: emojiReactions("👎"),
				})
			}(uid)
		}
		close(start)
		wg.Wait()

		// Third sequential tap after already cleared.
		env.dispatchReaction(&gotgbot.MessageReactionUpdated{
			Chat:        groupChat,
			MessageId:   msgID,
			User:        &gotgbot.User{Id: authorID, FirstName: "Author"},
			NewReaction: emojiReactions("👎"),
		})

		if clears := env.callsFor("setMessageReaction"); len(clears) != 1 {
			t.Fatalf("setMessageReaction calls = %d, want exactly 1", len(clears))
		}
	})
}

func TestPresenceConsensus(t *testing.T) {
	const (
		chatID int64 = -100888100
		msgID  int64 = 700
	)
	groupChat := gotgbot.Chat{Id: chatID, Type: "supergroup"}

	consensusSettings := func(threshold int, retract bool) {
		chatSwap(t, &presenceSettingsGet, func(context.Context, int64) (presence.Settings, error) {
			return presence.Settings{
				ConsensusEnabled:   true,
				ConsensusThreshold: threshold,
				RetractEnabled:     retract,
			}, nil
		})
	}

	reactUser := func(env *presenceReactionTestEnv, mID, uid int64, oldEmojis, newEmojis []string) {
		env.dispatchReaction(&gotgbot.MessageReactionUpdated{
			Chat:        groupChat,
			MessageId:   mID,
			User:        &gotgbot.User{Id: uid, FirstName: "Member"},
			OldReaction: emojiReactions(oldEmojis...),
			NewReaction: emojiReactions(newEmojis...),
		})
	}

	t.Run("3 distinct members mirror once (IsBig false, Source cons) and 4th is ignored", func(t *testing.T) {
		env := newPresenceReactionEnv(t, chatID)
		consensusSettings(3, true)

		reactUser(env, msgID, 11, nil, []string{"🔥"})
		reactUser(env, msgID, 12, nil, []string{"🔥"})
		if got := len(env.callsFor("setMessageReaction")); got != 0 {
			t.Fatalf("setMessageReaction after 2 reactions = %d, want 0", got)
		}

		reactUser(env, msgID, 13, nil, []string{"🔥"})
		sets := env.callsFor("setMessageReaction")
		if len(sets) != 1 {
			t.Fatalf("setMessageReaction after 3 reactions = %d, want 1", len(sets))
		}
		if isBig, ok := sets[0].Params["is_big"]; ok && isBig != "false" {
			t.Errorf("is_big = %q, want absent or false", isBig)
		}
		if !strings.Contains(sets[0].Params["reaction"], "🔥") {
			t.Errorf("wire reaction = %q, want 🔥", sets[0].Params["reaction"])
		}
		rec, ok := presenceReadBotReact(context.Background(), env.rc, chatID, msgID)
		if !ok || rec.Emoji != "🔥" || rec.Source != "cons" {
			t.Fatalf("bot_react = (%+v, %v), want Emoji=🔥 Source=cons", rec, ok)
		}

		// 4th distinct member must NOT re-react (once-only latch).
		reactUser(env, msgID, 14, nil, []string{"🔥"})
		if got := len(env.callsFor("setMessageReaction")); got != 1 {
			t.Fatalf("setMessageReaction after 4th reaction = %d, want 1", got)
		}
	})

	t.Run("single-user toggle-farming never triggers", func(t *testing.T) {
		env := newPresenceReactionEnv(t, chatID)
		consensusSettings(3, true)

		reactUser(env, msgID, 21, nil, []string{"🔥"})
		reactUser(env, msgID, 21, []string{"🔥"}, nil)
		reactUser(env, msgID, 21, nil, []string{"🔥"})
		reactUser(env, msgID, 21, []string{"🔥"}, nil)
		reactUser(env, msgID, 21, nil, []string{"🔥"})

		if got := len(env.callsFor("setMessageReaction")); got != 0 {
			t.Fatalf("toggle-farming triggered %d reactions, want 0", got)
		}
		card, err := env.rc.SCard(context.Background(), presenceConsensusUsersKey(chatID, msgID, "🔥")).Result()
		if err != nil || card != 1 {
			t.Errorf("SCARD after toggle-farming = (%d, %v), want (1, nil)", card, err)
		}
	})

	t.Run("Premium user stacking 3 different emojis never triggers", func(t *testing.T) {
		env := newPresenceReactionEnv(t, chatID)
		consensusSettings(3, true)

		reactUser(env, msgID, 31, nil, []string{"🔥"})
		reactUser(env, msgID, 31, []string{"🔥"}, []string{"🔥", "👍"})
		reactUser(env, msgID, 31, []string{"🔥", "👍"}, []string{"🔥", "👍", "🎉"})

		if got := len(env.callsFor("setMessageReaction")); got != 0 {
			t.Fatalf("Premium multi-emoji stacking triggered %d reactions, want 0", got)
		}
		for _, emoji := range []string{"🔥", "👍", "🎉"} {
			card, err := env.rc.SCard(context.Background(), presenceConsensusUsersKey(chatID, msgID, emoji)).Result()
			if err != nil || card != 1 {
				t.Errorf("SCARD(%s) = (%d, %v), want (1, nil)", emoji, card, err)
			}
		}
	})

	t.Run("hostile emojis never mirror", func(t *testing.T) {
		env := newPresenceReactionEnv(t, chatID)
		consensusSettings(3, true)

		for _, hostile := range []string{"👎", "🤡", "🤬", "💩"} {
			reactUser(env, msgID, 41, nil, []string{hostile})
			reactUser(env, msgID, 42, nil, []string{hostile})
			reactUser(env, msgID, 43, nil, []string{hostile})
			if env.mr.Exists(presenceConsensusUsersKey(chatID, msgID, hostile)) {
				t.Errorf("hostile emoji %s wrote a consensus set key", hostile)
			}
		}
		if got := len(env.callsFor("setMessageReaction")); got != 0 {
			t.Fatalf("hostile emojis triggered %d reactions, want 0", got)
		}
	})

	t.Run("anonymous admin and channel excluded from distinct-member count", func(t *testing.T) {
		env := newPresenceReactionEnv(t, chatID)
		consensusSettings(3, true)

		// Anonymous group admin + external channel + 2 real members = only 2 distinct real members.
		env.dispatchReaction(&gotgbot.MessageReactionUpdated{
			Chat:        groupChat,
			MessageId:   msgID,
			ActorChat:   &gotgbot.Chat{Id: chatID, Type: "supergroup"},
			NewReaction: emojiReactions("🔥"),
		})
		env.dispatchReaction(&gotgbot.MessageReactionUpdated{
			Chat:        groupChat,
			MessageId:   msgID,
			ActorChat:   &gotgbot.Chat{Id: -100999777, Type: "channel"},
			NewReaction: emojiReactions("🔥"),
		})
		reactUser(env, msgID, 51, nil, []string{"🔥"})
		reactUser(env, msgID, 52, nil, []string{"🔥"})
		if got := len(env.callsFor("setMessageReaction")); got != 0 {
			t.Fatalf("anonymous actors counted toward consensus: got %d reactions, want 0", got)
		}

		// 3rd real member crosses threshold.
		reactUser(env, msgID, 53, nil, []string{"🔥"})
		if got := len(env.callsFor("setMessageReaction")); got != 1 {
			t.Fatalf("3rd real member reactions = %d, want 1", got)
		}
	})

	t.Run("chat cooldown blocks second message without burning its done latch, and expires cleanly", func(t *testing.T) {
		env := newPresenceReactionEnv(t, chatID)
		consensusSettings(3, true)

		// First message mirrors and sets cons:cd.
		for _, uid := range []int64{61, 62, 63} {
			reactUser(env, msgID, uid, nil, []string{"🔥"})
		}
		if got := len(env.callsFor("setMessageReaction")); got != 1 {
			t.Fatalf("msg 1 setMessageReaction = %d, want 1", got)
		}
		if !env.mr.Exists(presenceConsensusCooldownKey(chatID)) {
			t.Fatal("chat cooldown key missing after msg 1")
		}

		// Second message hits threshold while cooldown is active -> blocked, and cons:done:<msgID+1> NOT burned.
		for _, uid := range []int64{61, 62, 63} {
			reactUser(env, msgID+1, uid, nil, []string{"👍"})
		}
		if got := len(env.callsFor("setMessageReaction")); got != 1 {
			t.Fatalf("msg 2 during cooldown triggered reaction (total=%d, want 1)", got)
		}
		if env.mr.Exists(presenceConsensusDoneKey(chatID, msgID+1)) {
			t.Error("cons:done burned for msg 2 while chat cooldown was active")
		}

		// Once cooldown expires/deleted, msg 2 mirrors on next crossing (or msg 3 mirrors cleanly).
		env.mr.Del(presenceConsensusCooldownKey(chatID))
		reactUser(env, msgID+1, 64, nil, []string{"👍"})
		if got := len(env.callsFor("setMessageReaction")); got != 2 {
			t.Fatalf("msg 2 after cooldown cleared = %d, want 2", got)
		}
	})

	t.Run("configurable threshold via SetConsensusThreshold(4) uses stored value", func(t *testing.T) {
		dbChatID := uniqueModuleChatID()
		presenceCleanup(t, dbChatID)
		env := newPresenceReactionEnv(t, dbChatID)
		chatSwap(t, &presenceSettingsGet, presence.Get)

		ctx := context.Background()
		if err := presence.SetConsensusEnabled(ctx, dbChatID, true, 1); err != nil {
			t.Fatalf("SetConsensusEnabled: %v", err)
		}
		if err := presence.SetConsensusThreshold(ctx, dbChatID, 4, 1); err != nil {
			t.Fatalf("SetConsensusThreshold(4): %v", err)
		}

		dbChat := gotgbot.Chat{Id: dbChatID, Type: "supergroup"}
		for _, uid := range []int64{71, 72, 73} {
			env.dispatchReaction(&gotgbot.MessageReactionUpdated{
				Chat:        dbChat,
				MessageId:   msgID,
				User:        &gotgbot.User{Id: uid, FirstName: "Member"},
				NewReaction: emojiReactions("🔥"),
			})
		}
		if got := len(env.callsFor("setMessageReaction")); got != 0 {
			t.Fatalf("threshold 4 triggered after 3 reactions (%d calls)", got)
		}

		env.dispatchReaction(&gotgbot.MessageReactionUpdated{
			Chat:        dbChat,
			MessageId:   msgID,
			User:        &gotgbot.User{Id: 74, FirstName: "Member4"},
			NewReaction: emojiReactions("🔥"),
		})
		if got := len(env.callsFor("setMessageReaction")); got != 1 {
			t.Fatalf("threshold 4 after 4th reaction = %d, want 1", got)
		}
	})

	t.Run("Staff Group skips completely with zero Redis writes and zero Telegram calls", func(t *testing.T) {
		env := newPresenceReactionEnv(t, chatID)
		consensusSettings(3, true)
		chatSwap(t, &presenceStaffLookup, func(id int64) *models.StaffGroup {
			if id == chatID {
				return &models.StaffGroup{ChatID: chatID}
			}
			return nil
		})

		for _, uid := range []int64{81, 82, 83} {
			reactUser(env, msgID, uid, nil, []string{"🔥"})
		}
		if len(env.allCalls()) != 0 {
			t.Errorf("Staff Group Telegram calls = %d, want 0", len(env.allCalls()))
		}
		if keys := env.mr.Keys(); len(keys) != 0 {
			t.Errorf("Staff Group wrote Redis keys: %v", keys)
		}
	})

	t.Run("active lockdown is checked lazily at threshold and fails closed on DB error", func(t *testing.T) {
		env := newPresenceReactionEnv(t, chatID)
		consensusSettings(3, true)

		var (
			lockdownCalls atomic.Int32
			lockdownRow   *models.ChatLockdown
			lockdownErr   error
		)
		chatSwap(t, &presenceLockdownLookup, func(int64) (*models.ChatLockdown, error) {
			lockdownCalls.Add(1)
			return lockdownRow, lockdownErr
		})

		// Reactions 1 and 2 must NOT query lockdown (lazy!).
		lockdownRow = &models.ChatLockdown{ChatID: chatID, State: models.LockdownStateActive}
		reactUser(env, msgID, 91, nil, []string{"🔥"})
		reactUser(env, msgID, 92, nil, []string{"🔥"})
		if got := lockdownCalls.Load(); got != 0 {
			t.Fatalf("lockdown lookup called %d times before threshold, want 0", got)
		}

		// Reaction 3 crosses threshold -> queries lockdown once and blocks mirroring.
		reactUser(env, msgID, 93, nil, []string{"🔥"})
		if got := lockdownCalls.Load(); got != 1 {
			t.Fatalf("lockdown lookup calls at threshold = %d, want 1", got)
		}
		if got := len(env.callsFor("setMessageReaction")); got != 0 {
			t.Fatalf("active lockdown mirrored %d reactions, want 0", got)
		}
		if env.mr.Exists(presenceConsensusDoneKey(chatID, msgID)) {
			t.Error("active lockdown burned cons:done latch")
		}

		// DB error fails closed too (msgID+1).
		lockdownRow = nil
		lockdownErr = errors.New("postgres unavailable")
		reactUser(env, msgID+1, 91, nil, []string{"🔥"})
		reactUser(env, msgID+1, 92, nil, []string{"🔥"})
		if got := lockdownCalls.Load(); got != 1 {
			t.Fatalf("lockdown lookup called before threshold on msg 2: got %d, want 1", got)
		}
		reactUser(env, msgID+1, 93, nil, []string{"🔥"})
		if got := lockdownCalls.Load(); got != 2 {
			t.Fatalf("lockdown lookup calls on msg 2 threshold = %d, want 2", got)
		}
		if got := len(env.callsFor("setMessageReaction")); got != 0 {
			t.Fatalf("lockdown DB error mirrored %d reactions, want 0 (fail closed)", got)
		}
	})

	t.Run("best-effort chat_log author lookup lets author retract with zero getChatMember", func(t *testing.T) {
		env := newPresenceReactionEnv(t, chatID)
		consensusSettings(3, true)

		const logAuthorID int64 = 777
		if err := chatLogRecord(env.rc, chatID, chatLogEntry{
			MessageID: msgID,
			UserID:    logAuthorID,
			Name:      "Alice",
			Text:      "great news",
		}); err != nil {
			t.Fatalf("chatLogRecord: %v", err)
		}

		for _, uid := range []int64{101, 102, 103} {
			reactUser(env, msgID, uid, nil, []string{"🔥"})
		}
		rec, ok := presenceReadBotReact(context.Background(), env.rc, chatID, msgID)
		if !ok || rec.AuthorID != logAuthorID || rec.Source != "cons" {
			t.Fatalf("bot_react = (%+v, %v), want AuthorID=%d Source=cons", rec, ok, logAuthorID)
		}

		var adminChecks atomic.Int32
		chatSwap(t, &presenceIsAdmin, func(*gotgbot.Bot, int64, int64) bool {
			adminChecks.Add(1)
			return false
		})

		// Now author 777 reacts 👎 -> clears without calling presenceIsAdmin!
		reactUser(env, msgID, logAuthorID, nil, []string{"👎"})
		if got := adminChecks.Load(); got != 0 {
			t.Errorf("presenceIsAdmin calls = %d, want 0 for chat_log author", got)
		}
		sets := env.callsFor("setMessageReaction")
		if len(sets) != 2 {
			t.Fatalf("setMessageReaction calls = %d, want 2 (mirror + clear)", len(sets))
		}
		if sets[1].Params["reaction"] != "[]" {
			t.Fatalf("retract wire reaction = %q, want \"[]\"", sets[1].Params["reaction"])
		}
	})
}

func TestShutdownEndsPresenceReaction(t *testing.T) {
	withFastPresencePacer(t)
	utilsCache.SetupTestMemoryMarshaler(t)
	_, rc := withChatRedis(t)
	chatJobs.wait()
	chatSwap(t, &chatStopWait, 2*time.Second)
	chatSwap(t, &presenceStaffLookup, func(int64) *models.StaffGroup { return nil })
	chatSwap(t, &presenceSettingsGet, func(context.Context, int64) (presence.Settings, error) {
		return presence.Settings{RetractEnabled: true, ConsensusThreshold: 3}, nil
	})
	restoreChatPool(t)

	const (
		chatID   int64 = -100888999
		msgID    int64 = 900
		authorID int64 = 501
	)
	presenceWriteBotReact(context.Background(), rc, chatID, msgID, presenceBotReact{
		Emoji:    "🔥",
		AuthorID: authorID,
		Source:   "cons",
	})

	started := make(chan struct{})
	testDone := make(chan struct{})
	defer close(testDone)

	var serverCalls atomic.Int32
	bot := newWireTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bottest/setMessageReaction" {
			if serverCalls.Add(1) == 1 {
				close(started)
			}
			select {
			case <-r.Context().Done():
			case <-testDone:
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	})

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadPresence(dispatcher)

	done := make(chan error, 1)
	go func() {
		done <- dispatcher.ProcessUpdate(bot, &gotgbot.Update{
			UpdateId: 1,
			MessageReaction: &gotgbot.MessageReactionUpdated{
				Chat:        gotgbot.Chat{Id: chatID, Type: "supergroup"},
				MessageId:   msgID,
				User:        &gotgbot.User{Id: authorID, FirstName: "Author"},
				NewReaction: emojiReactions("👎"),
			},
		}, nil)
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for setMessageReaction request to reach server")
	}

	stopStart := time.Now()
	if !StopChat() {
		t.Fatal("StopChat() timed out while presence reaction call was in flight")
	}
	if elapsed := time.Since(stopStart); elapsed >= chatStopWait {
		t.Errorf("StopChat() took %v, expected well inside %v", elapsed, chatStopWait)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("ProcessUpdate returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("dispatched reaction goroutine did not exit after StopChat()")
	}

	// Late update arriving while chatDraining is true must be shed immediately without hitting the server.
	presenceWriteBotReact(context.Background(), rc, chatID, msgID+1, presenceBotReact{
		Emoji:    "🔥",
		AuthorID: authorID,
		Source:   "cons",
	})
	if err := dispatcher.ProcessUpdate(bot, &gotgbot.Update{
		UpdateId: 2,
		MessageReaction: &gotgbot.MessageReactionUpdated{
			Chat:        gotgbot.Chat{Id: chatID, Type: "supergroup"},
			MessageId:   msgID + 1,
			User:        &gotgbot.User{Id: authorID, FirstName: "Author"},
			NewReaction: emojiReactions("👎"),
		},
	}, nil); err != nil {
		t.Fatalf("late ProcessUpdate returned error: %v", err)
	}
	if got := serverCalls.Load(); got != 1 {
		t.Fatalf("server setMessageReaction calls = %d, want 1 (late reaction must be shed)", got)
	}
}

func TestPresencePMBriefReaction(t *testing.T) {
	const (
		chatID int64 = -100888777
		userID int64 = 4201
		msgID  int64 = 500
	)
	groupChat := gotgbot.Chat{Id: chatID, Type: "supergroup", Title: "Room <A>"}

	setupPMBrief := func(t *testing.T, pmBriefEnabled, logEnabled bool, audience int, refuseUserID int64) (*presenceReactionTestEnv, *chatFakeProvider) {
		t.Helper()
		withRealLocales(t)
		withChatProviders(t, true, true, true)
		withChatRuntime(t)

		env := newPresenceReactionEnv(t, chatID)
		provider := &chatFakeProvider{}
		chatSwap(t, &chatProviderOf, func(string) chatProvider { return provider })
		chatSwap(t, &presencePMBriefCooldown, presencePMBriefCooldown)
		chatSwap(t, &presenceSettingsGet, func(context.Context, int64) (presence.Settings, error) {
			return presence.Settings{PMBriefReact: pmBriefEnabled, RetractEnabled: true, ConsensusThreshold: 3}, nil
		})
		chatSwap(t, &chatSettingsLookup, func(context.Context, int64) (chatconv.Settings, error) {
			return chatconv.Settings{
				Provider:   chatconv.ProviderAnthropic,
				LogEnabled: logEnabled,
				Audience:   audience,
			}, nil
		})
		if refuseUserID != 0 {
			env.bot = newWireTestBot(t, func(w http.ResponseWriter, r *http.Request) {
				params, err := decodeTelegramWireParams(r)
				env.recordCall(r.URL.Path, params, err)
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/bottest/sendMessage" && params["chat_id"] == strconv.FormatInt(refuseUserID, 10) {
					w.WriteHeader(http.StatusForbidden)
					_, _ = w.Write([]byte(`{"ok":false,"error_code":403,"description":"Forbidden: bot was blocked by the user"}`))
					return
				}
				_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
			})
		}
		return env, provider
	}

	seedTopicTalk := func(t *testing.T, rc *redis.Client) {
		t.Helper()
		for _, entry := range []chatLogEntry{
			{MessageID: msgID - 1, UserID: 101, Name: "Alice", Text: "before-anchor-t7", Thread: 7, At: 1700000000},
			{MessageID: msgID, UserID: 102, Name: "Bob", Text: "anchor-t7", Thread: 7, At: 1700000010},
			{MessageID: msgID + 1, UserID: 103, Name: "Carol", Text: "other-topic-t9", Thread: 9, At: 1700000020},
			{MessageID: msgID + 2, UserID: 101, Name: "Alice", Text: "followup-t7", Thread: 7, At: 1700000030},
		} {
			if err := chatLogRecord(rc, chatID, entry); err != nil {
				t.Fatalf("chatLogRecord(%d): %v", entry.MessageID, err)
			}
		}
	}

	t.Run("happy path sends PM brief for same topic and zero group messages", func(t *testing.T) {
		env, provider := setupPMBrief(t, true, true, chatconv.AudienceEveryone, 0)
		seedTopicTalk(t, env.rc)
		provider.script(chatAnswerText("Bob & Alice discussed topic 7."))

		env.dispatchReaction(&gotgbot.MessageReactionUpdated{
			Chat:        groupChat,
			MessageId:   msgID,
			User:        &gotgbot.User{Id: userID, FirstName: "Reader"},
			NewReaction: emojiReactions("✍"),
		})
		chatJobs.wait()

		calls := provider.calls()
		if len(calls) != 1 {
			t.Fatalf("provider calls = %d, want 1", len(calls))
		}
		if strings.Contains(calls[0].User, "before-anchor-t7") || strings.Contains(calls[0].User, "other-topic-t9") {
			t.Fatalf("PM brief request leaked earlier or other-topic messages:\n%s", calls[0].User)
		}
		if !strings.Contains(calls[0].User, "anchor-t7") || !strings.Contains(calls[0].User, "followup-t7") {
			t.Fatalf("PM brief request missing topic 7 stretch:\n%s", calls[0].User)
		}

		if actions := env.callsFor("sendChatAction"); len(actions) != 0 {
			t.Fatalf("sendChatAction calls = %d, want 0 (silent PM)", len(actions))
		}
		sends := env.callsFor("sendMessage")
		if len(sends) != 1 {
			t.Fatalf("sendMessage calls = %d, want 1 PM only", len(sends))
		}
		if sends[0].Params["chat_id"] != strconv.FormatInt(userID, 10) {
			t.Fatalf("sendMessage chat_id = %q, want %d", sends[0].Params["chat_id"], userID)
		}
		if tid := sends[0].Params["message_thread_id"]; tid != "" && tid != "0" {
			t.Fatalf("PM message_thread_id = %q, want empty/0", tid)
		}
		if !strings.Contains(sends[0].Params["text"], "Room &lt;A&gt;") || !strings.Contains(sends[0].Params["text"], "Bob &amp; Alice discussed topic 7.") {
			t.Fatalf("PM text = %q", sends[0].Params["text"])
		}
	})

	t.Run("403 PM blocked posts zero group messages or typing", func(t *testing.T) {
		env, provider := setupPMBrief(t, true, true, chatconv.AudienceEveryone, userID)
		seedTopicTalk(t, env.rc)
		provider.script(chatAnswerText("Silent fail on 403."))
		failedBefore := chatBriefFailedCount.Load()

		env.dispatchReaction(&gotgbot.MessageReactionUpdated{
			Chat:        groupChat,
			MessageId:   msgID,
			User:        &gotgbot.User{Id: userID, FirstName: "Blocked"},
			NewReaction: emojiReactions("✍️"),
		})
		chatJobs.wait()

		if len(provider.calls()) != 1 {
			t.Fatalf("provider calls = %d, want 1", len(provider.calls()))
		}
		if chatBriefFailedCount.Load() != failedBefore+1 {
			t.Fatalf("chatBriefFailedCount did not increment on 403")
		}
		if actions := env.callsFor("sendChatAction"); len(actions) != 0 {
			t.Fatalf("sendChatAction calls = %d, want 0", len(actions))
		}
		for _, call := range env.callsFor("sendMessage") {
			if call.Params["chat_id"] == strconv.FormatInt(chatID, 10) {
				t.Fatalf("403 PM leaked message to group: %+v", call)
			}
		}
	})

	t.Run("safety gates, double-gating, unlogged message, and per-user cooldown", func(t *testing.T) {
		// 1. PMBriefReact=false
		env, provider := setupPMBrief(t, false, true, chatconv.AudienceEveryone, 0)
		seedTopicTalk(t, env.rc)
		env.dispatchReaction(&gotgbot.MessageReactionUpdated{
			Chat: groupChat, MessageId: msgID, User: &gotgbot.User{Id: userID, FirstName: "U"}, NewReaction: emojiReactions("✍"),
		})
		chatJobs.wait()
		if len(provider.calls()) != 0 || len(env.allCalls()) != 0 {
			t.Fatalf("PMBriefReact=false triggered work")
		}

		// 2. LogEnabled=false
		env, provider = setupPMBrief(t, true, false, chatconv.AudienceEveryone, 0)
		seedTopicTalk(t, env.rc)
		env.dispatchReaction(&gotgbot.MessageReactionUpdated{
			Chat: groupChat, MessageId: msgID, User: &gotgbot.User{Id: userID, FirstName: "U"}, NewReaction: emojiReactions("✍"),
		})
		chatJobs.wait()
		if len(provider.calls()) != 0 || len(env.allCalls()) != 0 {
			t.Fatalf("LogEnabled=false triggered work")
		}

		// 3. Anonymous user, bot user, Staff Group, lockdown, AudienceAdminsApproved, unlogged msg
		env, provider = setupPMBrief(t, true, true, chatconv.AudienceEveryone, 0)
		seedTopicTalk(t, env.rc)

		// Anonymous admin (User == nil)
		env.dispatchReaction(&gotgbot.MessageReactionUpdated{
			Chat: groupChat, MessageId: msgID, User: nil, ActorChat: &groupChat, NewReaction: emojiReactions("✍"),
		})
		// Bot user
		env.dispatchReaction(&gotgbot.MessageReactionUpdated{
			Chat: groupChat, MessageId: msgID, User: &gotgbot.User{Id: userID, IsBot: true, FirstName: "Bot"}, NewReaction: emojiReactions("✍"),
		})
		// Unlogged message
		env.dispatchReaction(&gotgbot.MessageReactionUpdated{
			Chat: groupChat, MessageId: msgID + 999, User: &gotgbot.User{Id: userID + 1, FirstName: "U"}, NewReaction: emojiReactions("✍"),
		})
		chatJobs.wait()
		if len(provider.calls()) != 0 || len(env.allCalls()) != 0 {
			t.Fatalf("anon/bot/unlogged triggered work: calls=%d tg=%d", len(provider.calls()), len(env.allCalls()))
		}

		// Staff Group
		chatSwap(t, &presenceStaffLookup, func(int64) *models.StaffGroup { return &models.StaffGroup{ChatID: chatID} })
		env.dispatchReaction(&gotgbot.MessageReactionUpdated{
			Chat: groupChat, MessageId: msgID, User: &gotgbot.User{Id: userID + 2, FirstName: "U"}, NewReaction: emojiReactions("✍"),
		})
		chatJobs.wait()
		chatSwap(t, &presenceStaffLookup, func(int64) *models.StaffGroup { return nil })

		// Active lockdown
		chatSwap(t, &presenceLockdownLookup, func(int64) (*models.ChatLockdown, error) {
			return &models.ChatLockdown{ChatID: chatID, State: models.LockdownStateActive}, nil
		})
		env.dispatchReaction(&gotgbot.MessageReactionUpdated{
			Chat: groupChat, MessageId: msgID, User: &gotgbot.User{Id: userID + 3, FirstName: "U"}, NewReaction: emojiReactions("✍"),
		})
		chatJobs.wait()
		chatSwap(t, &presenceLockdownLookup, func(int64) (*models.ChatLockdown, error) { return nil, nil })

		if len(provider.calls()) != 0 || len(env.allCalls()) != 0 {
			t.Fatalf("staff/lockdown triggered work")
		}

		// AudienceAdminsApproved rejects non-approved non-admin user
		env, provider = setupPMBrief(t, true, true, chatconv.AudienceAdminsApproved, 0)
		seedTopicTalk(t, env.rc)
		chatSwap(t, &chatAudienceAdmin, func(*gotgbot.Bot, int64, int64) bool { return false })
		chatSwap(t, &chatAudienceApproved, func(context.Context, int64, int64) (bool, error) { return false, nil })
		env.dispatchReaction(&gotgbot.MessageReactionUpdated{
			Chat: groupChat, MessageId: msgID, User: &gotgbot.User{Id: userID + 4, FirstName: "U"}, NewReaction: emojiReactions("✍"),
		})
		chatJobs.wait()
		if len(provider.calls()) != 0 || len(env.allCalls()) != 0 {
			t.Fatalf("AudienceAdminsApproved non-approved triggered work")
		}

		// Per-user cooldown prevents rapid tapping from draining brief budgets
		env, provider = setupPMBrief(t, true, true, chatconv.AudienceEveryone, 0)
		seedTopicTalk(t, env.rc)
		env.dispatchReaction(&gotgbot.MessageReactionUpdated{
			Chat: groupChat, MessageId: msgID, User: &gotgbot.User{Id: userID, FirstName: "Tapper"}, NewReaction: emojiReactions("✍"),
		})
		chatJobs.wait()
		env.dispatchReaction(&gotgbot.MessageReactionUpdated{
			Chat: groupChat, MessageId: msgID + 2, User: &gotgbot.User{Id: userID, FirstName: "Tapper"}, NewReaction: emojiReactions("✍"),
		})
		chatJobs.wait()
		if got := len(provider.calls()); got != 1 {
			t.Fatalf("rapid tap provider calls = %d, want 1", got)
		}
		if got := len(env.callsFor("sendMessage")); got != 1 {
			t.Fatalf("rapid tap sendMessage calls = %d, want 1", got)
		}
	})

	t.Run("StopChat cancels and drains in-flight ✍ PM brief job with zero new drains", func(t *testing.T) {
		restoreChatPool(t)
		env, provider := setupPMBrief(t, true, true, chatconv.AudienceEveryone, 0)
		chatSwap(t, &chatStopWait, 2*time.Second)
		seedTopicTalk(t, env.rc)

		started := make(chan struct{})
		provider.script(func(ctx context.Context, _ chatRequest) (chatAnswer, error) {
			close(started)
			<-ctx.Done()
			return chatAnswer{}, ctx.Err()
		})

		done := make(chan struct{})
		go func() {
			defer close(done)
			env.dispatchReaction(&gotgbot.MessageReactionUpdated{
				Chat:        groupChat,
				MessageId:   msgID,
				User:        &gotgbot.User{Id: userID, FirstName: "Reader"},
				NewReaction: emojiReactions("✍"),
			})
		}()

		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for ✍ PM brief provider call to start")
		}

		stopStart := time.Now()
		if !StopChat() {
			t.Fatal("StopChat() timed out during ✍ PM brief job")
		}
		if elapsed := time.Since(stopStart); elapsed >= chatStopWait {
			t.Errorf("StopChat() took %v, expected well inside %v", elapsed, chatStopWait)
		}

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("dispatchReaction goroutine did not exit")
		}

		// Late ✍ reaction while draining is shed immediately.
		env.dispatchReaction(&gotgbot.MessageReactionUpdated{
			Chat:        groupChat,
			MessageId:   msgID,
			User:        &gotgbot.User{Id: userID + 1, FirstName: "Late"},
			NewReaction: emojiReactions("✍"),
		})
		if len(provider.calls()) != 1 {
			t.Fatalf("provider calls = %d, want 1", len(provider.calls()))
		}
		if len(env.callsFor("sendMessage")) != 0 {
			t.Fatalf("sendMessage calls = %d, want 0 after StopChat", len(env.callsFor("sendMessage")))
		}
	})
}
