//go:build testtools

package reactions

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/cache"
	utilsCache "github.com/uasneppy/Xenobot-72/xenobot/utils/cache"
)

func skipIfNoDb(t *testing.T) {
	t.Helper()
	if db.DB == nil {
		t.Fatal("test database was not initialized")
	}
}

func TestReactionsRoundTrip(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	t.Cleanup(func() {
		_ = ResetReactions(chatID)
		cache.DeleteCache(cache.CacheKey("reactions", chatID))
	})

	if got := GetReactions(chatID); len(got) != 0 {
		t.Fatalf("GetReactions on fresh chat = %v, want empty", got)
	}

	if err := AddReaction(chatID, "hello", "👋"); err != nil {
		t.Fatalf("AddReaction() error = %v", err)
	}
	if err := AddReaction(chatID, "bye", "🚀"); err != nil {
		t.Fatalf("AddReaction(second) error = %v", err)
	}
	if got := GetReactions(chatID); got["hello"] != "👋" || got["bye"] != "🚀" {
		t.Fatalf("GetReactions = %v, want hello=👋 bye=🚀", got)
	}

	// Upsert: re-adding the same keyword updates the emoji, not a duplicate.
	if err := AddReaction(chatID, "hello", "😀"); err != nil {
		t.Fatalf("AddReaction(upsert) error = %v", err)
	}
	if got := GetReactions(chatID); got["hello"] != "😀" || len(got) != 2 {
		t.Fatalf("GetReactions after upsert = %v, want hello=😀 and 2 entries", got)
	}

	if err := RemoveReaction(chatID, "bye"); err != nil {
		t.Fatalf("RemoveReaction() error = %v", err)
	}
	got := GetReactions(chatID)
	if _, ok := got["bye"]; ok || len(got) != 1 {
		t.Fatalf("GetReactions after remove = %v, want only hello", got)
	}

	if err := ResetReactions(chatID); err != nil {
		t.Fatalf("ResetReactions() error = %v", err)
	}
	if got := GetReactions(chatID); len(got) != 0 {
		t.Fatalf("GetReactions after reset = %v, want empty", got)
	}
}

func TestReactionFreshReadersAndRemoveByID(t *testing.T) {
	skipIfNoDb(t)
	ctx := context.Background()
	chatID, other := -time.Now().UnixNano(), -time.Now().UnixNano()-1
	t.Cleanup(func() {
		_ = ResetReactions(chatID)
		_ = ResetReactions(other)
	})

	if rows, err := ListReactionEntriesFresh(ctx, chatID); err != nil || len(rows) != 0 {
		t.Fatalf("fresh list of an empty chat = %v, %v", rows, err)
	}
	if added, err := AddReactionContext(ctx, chatID, "hello", "👍", false); err != nil || !added {
		t.Fatalf("first add = %v, %v", added, err)
	}
	if added, err := AddReactionContext(ctx, chatID, "hello", "🔥", false); err != nil || added {
		t.Fatalf("add without overwrite of an existing keyword = %v, %v, want not added", added, err)
	}
	if got := GetReactions(chatID)["hello"]; got != "👍" {
		t.Fatalf("emoji after a refused overwrite = %q", got)
	}
	if added, err := AddReactionContext(ctx, chatID, "hello", "🔥", true); err != nil || !added {
		t.Fatalf("overwrite = %v, %v", added, err)
	}
	if got := GetReactions(chatID)["hello"]; got != "🔥" {
		t.Fatalf("emoji after overwrite = %q, want the cache dropped and the new emoji", got)
	}
	if _, err := AddReactionContext(ctx, other, "foreign", "👍", false); err != nil {
		t.Fatal(err)
	}
	rows, err := ListReactionEntriesFresh(ctx, chatID)
	if err != nil || len(rows) != 1 || rows[0].Keyword != "hello" || rows[0].Emoji != "🔥" {
		t.Fatalf("fresh list = %+v, %v", rows, err)
	}
	if n, err := CountReactionsFresh(ctx, chatID); err != nil || n != 1 {
		t.Fatalf("count = %d, %v", n, err)
	}
	if ok, err := ReactionExistsFresh(ctx, chatID, "hello"); err != nil || !ok {
		t.Fatalf("exists = %v, %v", ok, err)
	}
	foreign, err := ListReactionEntriesFresh(ctx, other)
	if err != nil || len(foreign) != 1 {
		t.Fatal(foreign, err)
	}
	if row, err := GetReactionEntryFresh(ctx, chatID, int64(foreign[0].ID)); err != nil || row != nil {
		t.Fatalf("another chat's ID read %+v, %v, want nil", row, err)
	}
	if row, err := RemoveReactionByID(ctx, chatID, int64(foreign[0].ID)); err != nil || row != nil {
		t.Fatalf("another chat's ID removed %+v, %v", row, err)
	}
	row, err := RemoveReactionByID(ctx, chatID, int64(rows[0].ID))
	if err != nil || row == nil || row.Keyword != "hello" {
		t.Fatalf("remove by ID = %+v, %v", row, err)
	}
	if n, _ := CountReactionsFresh(ctx, chatID); n != 0 {
		t.Fatalf("count after remove = %d", n)
	}
	if n, _ := CountReactionsFresh(ctx, other); n != 1 {
		t.Fatalf("the other chat lost its reaction")
	}
}

func TestReactionRuleOptionsAndCache(t *testing.T) {
	skipIfNoDb(t)
	utilsCache.SetupTestMemoryMarshaler(t)

	ctx := context.Background()
	chatID := -time.Now().UnixNano()
	otherChatID := chatID - 1
	t.Cleanup(func() {
		_ = ResetReactions(chatID)
		_ = ResetReactions(otherChatID)
	})

	if !ValidChancePct(MinChancePct) || !ValidChancePct(MaxChancePct) || ValidChancePct(0) || ValidChancePct(101) {
		t.Fatal("ValidChancePct boundary check failed")
	}
	if !ValidCooldownSecs(MinCooldownSecs) || !ValidCooldownSecs(MaxCooldownSecs) || ValidCooldownSecs(-1) || ValidCooldownSecs(86401) {
		t.Fatal("ValidCooldownSecs boundary check failed")
	}

	if err := AddReaction(chatID, "hello", "👋"); err != nil {
		t.Fatalf("AddReaction: %v", err)
	}
	if added, err := AddReactionContext(ctx, chatID, "gg", "🔥", false); err != nil || !added {
		t.Fatalf("AddReactionContext = %v, %v", added, err)
	}
	if added, err := AddReactionContext(ctx, otherChatID, "foreign", "👍", false); err != nil || !added {
		t.Fatalf("AddReactionContext(other) = %v, %v", added, err)
	}

	rules, err := GetReactionRulesContext(ctx, chatID)
	if err != nil {
		t.Fatalf("GetReactionRulesContext: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("len(rules) = %d, want 2", len(rules))
	}
	for i, r := range rules {
		if r.WordBoundary || r.MatchCaption || r.ChancePct != DefaultChancePct || r.CooldownSecs != 0 {
			t.Fatalf("rule[%d] defaults = %+v, want wb=false mc=false ch=100 cd=0", i, r)
		}
	}

	fresh, err := ListReactionEntriesFresh(ctx, chatID)
	if err != nil || len(fresh) != 2 {
		t.Fatalf("ListReactionEntriesFresh = %+v, %v", fresh, err)
	}
	if fresh[0].ChancePct != DefaultChancePct || fresh[0].WordBoundary || fresh[0].MatchCaption || fresh[0].CooldownSecs != 0 {
		t.Fatalf("fresh[0] defaults = %+v", fresh[0])
	}

	// Prime both caches before SetReactionOptionsByID.
	if got := GetReactionsContext(ctx, chatID); got["hello"] != "👋" {
		t.Fatalf("GetReactionsContext = %v", got)
	}
	if _, err := GetReactionRulesContext(ctx, chatID); err != nil {
		t.Fatalf("warm GetReactionRulesContext: %v", err)
	}

	ruleID := int64(rules[0].ID)
	updated, err := SetReactionOptionsByID(ctx, chatID, ruleID, true, true, 35, 120)
	if err != nil {
		t.Fatalf("SetReactionOptionsByID(valid): %v", err)
	}
	if updated == nil || !updated.WordBoundary || !updated.MatchCaption || updated.ChancePct != 35 || updated.CooldownSecs != 120 {
		t.Fatalf("updated = %+v, want wb=true mc=true ch=35 cd=120", updated)
	}

	// Both cache keys must have been evicted on write.
	var staleLegacy map[string]string
	if _, err := utilsCache.GetMarshal().Get(ctx, reactionsCacheKey(chatID), &staleLegacy); err == nil {
		t.Fatalf("reactions:<chat> cache key was not invalidated: %v", staleLegacy)
	}
	var staleRules []Rule
	if _, err := utilsCache.GetMarshal().Get(ctx, reactionRulesCacheKey(chatID), &staleRules); err == nil {
		t.Fatalf("reaction_rules:<chat> cache key was not invalidated: %v", staleRules)
	}

	rulesAfter, err := GetReactionRulesContext(ctx, chatID)
	if err != nil || len(rulesAfter) != 2 {
		t.Fatalf("GetReactionRulesContext after edit = %+v, %v", rulesAfter, err)
	}
	if !rulesAfter[0].WordBoundary || !rulesAfter[0].MatchCaption || rulesAfter[0].ChancePct != 35 || rulesAfter[0].CooldownSecs != 120 {
		t.Fatalf("rulesAfter[0] = %+v, want wb=true mc=true ch=35 cd=120", rulesAfter[0])
	}

	freshAfter, err := ListReactionEntriesFresh(ctx, chatID)
	if err != nil || len(freshAfter) != 2 {
		t.Fatalf("ListReactionEntriesFresh after edit = %+v, %v", freshAfter, err)
	}
	if !freshAfter[0].WordBoundary || !freshAfter[0].MatchCaption || freshAfter[0].ChancePct != 35 || freshAfter[0].CooldownSecs != 120 {
		t.Fatalf("freshAfter[0] = %+v", freshAfter[0])
	}

	// Reset booleans back to false and cooldown back to 0 (proves no boolean GORM default: trap).
	resetRow, err := SetReactionOptionsByID(ctx, chatID, ruleID, false, false, 100, 0)
	if err != nil {
		t.Fatalf("SetReactionOptionsByID(reset): %v", err)
	}
	if resetRow == nil || resetRow.WordBoundary || resetRow.MatchCaption || resetRow.ChancePct != 100 || resetRow.CooldownSecs != 0 {
		t.Fatalf("resetRow = %+v, want wb=false mc=false ch=100 cd=0", resetRow)
	}
	rulesReset, err := GetReactionRulesContext(ctx, chatID)
	if err != nil || rulesReset[0].WordBoundary || rulesReset[0].MatchCaption || rulesReset[0].ChancePct != 100 || rulesReset[0].CooldownSecs != 0 {
		t.Fatalf("rulesReset[0] = %+v, %v", rulesReset, err)
	}

	// Cross-chat IDOR refusal: foreign rule ID returns (nil, nil) and leaves foreign row intact.
	foreignRows, err := ListReactionEntriesFresh(ctx, otherChatID)
	if err != nil || len(foreignRows) != 1 {
		t.Fatalf("ListReactionEntriesFresh(other) = %+v, %v", foreignRows, err)
	}
	if foreignUpdated, err := SetReactionOptionsByID(ctx, chatID, int64(foreignRows[0].ID), true, true, 10, 60); err != nil || foreignUpdated != nil {
		t.Fatalf("cross-chat SetReactionOptionsByID = %+v, %v, want (nil, nil)", foreignUpdated, err)
	}
	foreignAfter, err := GetReactionEntryFresh(ctx, otherChatID, int64(foreignRows[0].ID))
	if err != nil || foreignAfter == nil || foreignAfter.WordBoundary || foreignAfter.MatchCaption || foreignAfter.ChancePct != DefaultChancePct || foreignAfter.CooldownSecs != 0 {
		t.Fatalf("foreign row mutated = %+v, %v", foreignAfter, err)
	}

	// Invalid bounds must return ErrInvalidOption with zero DB mutations.
	for _, tc := range []struct {
		chance   int
		cooldown int
	}{
		{0, 0},
		{-1, 0},
		{101, 0},
		{50, -1},
		{50, 86401},
	} {
		row, err := SetReactionOptionsByID(ctx, chatID, ruleID, true, true, tc.chance, tc.cooldown)
		if !errors.Is(err, ErrInvalidOption) || row != nil {
			t.Fatalf("SetReactionOptionsByID(%d, %d) = %+v, %v, want (nil, ErrInvalidOption)", tc.chance, tc.cooldown, row, err)
		}
	}
	unchanged, err := GetReactionEntryFresh(ctx, chatID, ruleID)
	if err != nil || unchanged == nil || unchanged.WordBoundary || unchanged.MatchCaption || unchanged.ChancePct != 100 || unchanged.CooldownSecs != 0 {
		t.Fatalf("invalid option mutated DB row: %+v, %v", unchanged, err)
	}

	// Legacy cached Rule entry with ChancePct == 0 normalizes to 100 on read.
	legacyChatID := otherChatID - 1
	if err := utilsCache.GetMarshal().Set(ctx, reactionRulesCacheKey(legacyChatID), []Rule{
		{ID: 77, Keyword: "hi", Emoji: "👋", WordBoundary: true, MatchCaption: false, ChancePct: 0, CooldownSecs: 0},
	}); err != nil {
		t.Fatalf("seed legacy cached rules: %v", err)
	}
	legacyRules, err := GetReactionRulesContext(ctx, legacyChatID)
	if err != nil || len(legacyRules) != 1 || legacyRules[0].ChancePct != DefaultChancePct || !legacyRules[0].WordBoundary {
		t.Fatalf("legacy GetReactionRulesContext = %+v, %v, want ChancePct=100", legacyRules, err)
	}
}
