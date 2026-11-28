package modules

import (
	"context"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/Xenobot-72/xenobot/db/presence"
)

// The delayed consensus retry is a timer; the shared test binary keeps it off unless a test asks for
// it, so a timer armed by one test can never fire inside another.
func init() { presenceConsensusRetryMax = 0 }

func consensusFixSettings(t *testing.T, threshold int) {
	t.Helper()
	chatSwap(t, &presenceSettingsGet, func(context.Context, int64) (presence.Settings, error) {
		return presence.Settings{ConsensusEnabled: true, ConsensusThreshold: threshold}, nil
	})
}

func consensusFixReact(env *presenceReactionTestEnv, chat gotgbot.Chat, msgID, uid int64, oldEmojis, newEmojis []string) {
	env.dispatchReaction(&gotgbot.MessageReactionUpdated{
		Chat: chat, MessageId: msgID, User: &gotgbot.User{Id: uid, FirstName: "Member"},
		OldReaction: emojiReactions(oldEmojis...), NewReaction: emojiReactions(newEmojis...),
	})
}

// TestPresenceConsensusClaimsOnlyWhatItUses: the palette is checked before anything is claimed, and
// a reaction that cannot be set gives the chat cooldown and the message latch back, so one failure
// never locks the chat or the message out of consensus.
func TestPresenceConsensusClaimsOnlyWhatItUses(t *testing.T) {
	const (
		chatID int64 = -100888200
		msgID  int64 = 710
	)
	chat := gotgbot.Chat{Id: chatID, Type: "supergroup"}

	t.Run("an emoji the chat does not allow claims nothing", func(t *testing.T) {
		env := newPresenceReactionEnv(t, chatID)
		consensusFixSettings(t, 3)
		env.mu.Lock()
		env.availJSON = `[{"type":"emoji","emoji":"👍"}]`
		env.mu.Unlock()
		for _, uid := range []int64{1, 2, 3} {
			consensusFixReact(env, chat, msgID, uid, nil, []string{"🔥"})
		}
		if got := len(env.callsFor("setMessageReaction")); got != 0 {
			t.Fatalf("setMessageReaction calls = %d, want 0 for a disallowed emoji", got)
		}
		if env.mr.Exists(presenceConsensusCooldownKey(chatID)) || env.mr.Exists(presenceConsensusDoneKey(chatID, msgID)) {
			t.Fatal("a disallowed emoji burned the chat cooldown or the message latch")
		}
		// An allowed emoji on another message still mirrors right away.
		for _, uid := range []int64{4, 5, 6} {
			consensusFixReact(env, chat, msgID+1, uid, nil, []string{"👍"})
		}
		if got := len(env.callsFor("setMessageReaction")); got != 1 {
			t.Fatalf("allowed emoji did not mirror: calls = %d, want 1", got)
		}
	})

	t.Run("the first allowed emoji of several that crossed is mirrored", func(t *testing.T) {
		env := newPresenceReactionEnv(t, chatID)
		consensusFixSettings(t, 2)
		env.mu.Lock()
		env.availJSON = `[{"type":"emoji","emoji":"👍"}]`
		env.mu.Unlock()
		consensusFixReact(env, chat, msgID, 1, nil, []string{"🔥", "👍"})
		consensusFixReact(env, chat, msgID, 2, nil, []string{"🔥", "👍"})
		sets := env.callsFor("setMessageReaction")
		if len(sets) != 1 || sets[0].Params["reaction"] == "" {
			t.Fatalf("setMessageReaction calls = %d, want 1 for the allowed emoji", len(sets))
		}
	})

	t.Run("a failed set gives the claims back so the next reaction retries", func(t *testing.T) {
		env := newPresenceReactionEnv(t, chatID)
		consensusFixSettings(t, 3)
		env.mu.Lock()
		env.reactFail = "message to react not found"
		env.mu.Unlock()
		for _, uid := range []int64{11, 12, 13} {
			consensusFixReact(env, chat, msgID, uid, nil, []string{"🔥"})
		}
		if got := len(env.callsFor("setMessageReaction")); got != 1 {
			t.Fatalf("failing set attempts = %d, want 1", got)
		}
		if env.mr.Exists(presenceConsensusCooldownKey(chatID)) || env.mr.Exists(presenceConsensusDoneKey(chatID, msgID)) {
			t.Fatal("a failed mirror left the chat cooldown or the message latch behind")
		}

		env.mu.Lock()
		env.reactFail = ""
		env.mu.Unlock()
		consensusFixReact(env, chat, msgID, 14, nil, []string{"🔥"})
		if got := len(env.callsFor("setMessageReaction")); got != 2 {
			t.Fatalf("retry after the failure: calls = %d, want 2", got)
		}
		if !env.mr.Exists(presenceConsensusDoneKey(chatID, msgID)) {
			t.Fatal("a successful mirror must set the once-only latch")
		}
	})
}

// TestPresenceConsensusDropsStaleUpdates: reaction updates are handled concurrently, so a user's
// "add" can arrive after their later "remove". The older update must not put them back.
func TestPresenceConsensusDropsStaleUpdates(t *testing.T) {
	const (
		chatID int64 = -100888201
		msgID  int64 = 720
	)
	chat := gotgbot.Chat{Id: chatID, Type: "supergroup"}
	env := newPresenceReactionEnv(t, chatID)
	consensusFixSettings(t, 3)
	user := &gotgbot.User{Id: 81, FirstName: "Member"}
	setKey := presenceConsensusUsersKey(chatID, msgID, "🔥")

	// update 21 (remove) is applied before update 20 (add).
	env.dispatchReactionAs(21, &gotgbot.MessageReactionUpdated{Chat: chat, MessageId: msgID, User: user, OldReaction: emojiReactions("🔥")})
	env.dispatchReactionAs(20, &gotgbot.MessageReactionUpdated{Chat: chat, MessageId: msgID, User: user, NewReaction: emojiReactions("🔥")})
	if env.mr.Exists(setKey) {
		members, _ := env.mr.SMembers(setKey)
		if len(members) != 0 {
			t.Fatalf("a stale add resurrected a removed reaction: %v", members)
		}
	}

	// In order, both apply; the same update_id may touch several emojis.
	env.dispatchReactionAs(30, &gotgbot.MessageReactionUpdated{Chat: chat, MessageId: msgID, User: user, NewReaction: emojiReactions("🔥", "👍")})
	for _, emoji := range []string{"🔥", "👍"} {
		members, _ := env.mr.SMembers(presenceConsensusUsersKey(chatID, msgID, emoji))
		if len(members) != 1 {
			t.Fatalf("update 30 did not record %s: %v", emoji, members)
		}
	}
	// A different user is not affected by this user's stamp.
	env.dispatchReactionAs(5, &gotgbot.MessageReactionUpdated{Chat: chat, MessageId: msgID, User: &gotgbot.User{Id: 82, FirstName: "Other"}, NewReaction: emojiReactions("🔥")})
	if members, _ := env.mr.SMembers(setKey); len(members) != 2 {
		t.Fatalf("another user's older update was dropped: %v", members)
	}
}

// TestPresenceConsensusRetriesAfterCooldown: a message that reaches its threshold while the chat
// cooldown runs is retried when the cooldown ends, instead of waiting for one more reaction that,
// after a burst, never comes.
func TestPresenceConsensusRetriesAfterCooldown(t *testing.T) {
	const (
		chatID int64 = -100888202
		msgA   int64 = 730
		msgB   int64 = 731
	)
	chat := gotgbot.Chat{Id: chatID, Type: "supergroup"}
	env := newPresenceReactionEnv(t, chatID)
	consensusFixSettings(t, 3)
	chatSwap(t, &presenceConsensusCooldown, time.Second)
	chatSwap(t, &presenceConsensusRetryJitter, 100*time.Millisecond)
	chatSwap(t, &presenceConsensusRetryMax, 16)

	for _, uid := range []int64{1, 2, 3} {
		consensusFixReact(env, chat, msgA, uid, nil, []string{"🔥"})
	}
	// Message B crosses inside the cooldown and is not mirrored yet.
	for _, uid := range []int64{4, 5, 6} {
		consensusFixReact(env, chat, msgB, uid, nil, []string{"👍"})
	}
	if got := len(env.callsFor("setMessageReaction")); got != 1 {
		t.Fatalf("calls right after the burst = %d, want 1 (B waits for the cooldown)", got)
	}
	if env.mr.Exists(presenceConsensusDoneKey(chatID, msgB)) {
		t.Fatal("a refused claim must not burn B's latch")
	}

	// miniredis expires keys only when told to: end the chat cooldown now, the timer fires after it.
	env.mr.FastForward(2 * time.Second)
	deadline := time.Now().Add(5 * time.Second)
	for len(env.callsFor("setMessageReaction")) < 2 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	chatJobs.wait()
	if got := len(env.callsFor("setMessageReaction")); got != 2 {
		t.Fatalf("calls after the cooldown = %d, want 2: B was never retried", got)
	}
	if !env.mr.Exists(presenceConsensusDoneKey(chatID, msgB)) {
		t.Fatal("the retried mirror must set B's latch")
	}
}

func TestPresenceConsensusRetryRechecksTheCount(t *testing.T) {
	const (
		chatID int64 = -100888203
		msgA   int64 = 740
		msgB   int64 = 741
	)
	chat := gotgbot.Chat{Id: chatID, Type: "supergroup"}
	env := newPresenceReactionEnv(t, chatID)
	consensusFixSettings(t, 3)
	chatSwap(t, &presenceConsensusCooldown, time.Second)
	chatSwap(t, &presenceConsensusRetryJitter, 100*time.Millisecond)
	chatSwap(t, &presenceConsensusRetryMax, 16)

	for _, uid := range []int64{1, 2, 3} {
		consensusFixReact(env, chat, msgA, uid, nil, []string{"🔥"})
	}
	for _, uid := range []int64{4, 5, 6} {
		consensusFixReact(env, chat, msgB, uid, nil, []string{"👍"})
	}
	// One member takes the reaction back before the cooldown ends: the count is below the
	// threshold again and the retry must not mirror.
	consensusFixReact(env, chat, msgB, 6, []string{"👍"}, nil)
	env.mr.FastForward(2 * time.Second) // the cooldown ends, so only the count can stop the retry
	time.Sleep(2500 * time.Millisecond)
	chatJobs.wait()
	if got := len(env.callsFor("setMessageReaction")); got != 1 {
		t.Fatalf("calls = %d, want 1: the retry mirrored a message that fell below its threshold", got)
	}
}

// TestPresenceMirrorKeepsStoredAuthor: a mirror made with the message log off cannot resolve the
// author; it must keep the author an earlier reaction of the bot recorded.
func TestPresenceMirrorKeepsStoredAuthor(t *testing.T) {
	const (
		chatID int64 = -100888204
		msgID  int64 = 750
	)
	env := newPresenceReactionEnv(t, chatID)
	ctx := context.Background()
	if !presenceSetReaction(ctx, env.bot, chatID, msgID, 4242, "🔥", "static") {
		t.Fatal("first reaction was not set")
	}
	if !presenceSetReaction(ctx, env.bot, chatID, msgID, 0, "👍", "cons") {
		t.Fatal("mirror was not set")
	}
	rec, ok := presenceReadBotReact(ctx, env.rc, chatID, msgID)
	if !ok || rec.AuthorID != 4242 || rec.Emoji != "👍" || rec.Source != "cons" {
		t.Fatalf("bot_react = (%+v, %v), want the mirror with author 4242 kept", rec, ok)
	}
}
