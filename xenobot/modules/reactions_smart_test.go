//go:build testtools

package modules

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/uasneppy/Xenobot-72/xenobot/config"
	dbcache "github.com/uasneppy/Xenobot-72/xenobot/db/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/db/presence"
	"github.com/uasneppy/Xenobot-72/xenobot/db/reactions"
)

func TestStaticReactionsUnicodeAndOptions(t *testing.T) {
	t.Run("matchStaticReactionKeyword Unicode word boundaries", func(t *testing.T) {
		cases := []struct {
			name         string
			text         string
			keyword      string
			wordBoundary bool
			want         bool
		}{
			{name: "ukrainian whole word", text: "Привіт, друже!", keyword: "привіт", wordBoundary: true, want: true},
			{name: "ukrainian rejects prefix inside word", text: "Щирі привітання!", keyword: "привіт", wordBoundary: true, want: false},
			{name: "ukrainian substring matches", text: "Щирі привітання!", keyword: "привіт", wordBoundary: false, want: true},
			{name: "ukrainian i yi ye g", text: "Їжак і ґанок є!", keyword: "їжак", wordBoundary: true, want: true},
			{name: "russian whole word", text: "Мой кот!", keyword: "кот", wordBoundary: true, want: true},
			{name: "russian rejects derivative", text: "Мой котик!", keyword: "кот", wordBoundary: true, want: false},
			{name: "english whole word", text: "Hi BOT!", keyword: "bot", wordBoundary: true, want: true},
			{name: "english rejects superstring", text: "A robot arrived", keyword: "bot", wordBoundary: true, want: false},
			{name: "underscore counts as word rune", text: "use bot_1 here", keyword: "bot", wordBoundary: true, want: false},
			{name: "digit counts as word rune", text: "use v20 here", keyword: "v2", wordBoundary: true, want: false},
			{name: "digit boundary passes punctuation", text: "use v2! here", keyword: "v2", wordBoundary: true, want: true},
			{name: "multi-word phrase", text: "Well, GOOD MORNING!", keyword: "good morning", wordBoundary: true, want: true},
			{name: "multi-word phrase rejects trailing letter", text: "Well, good mornings!", keyword: "good morning", wordBoundary: true, want: false},
			{name: "second occurrence matches after bounded first", text: "привітання але привіт!", keyword: "привіт", wordBoundary: true, want: true},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if got := matchStaticReactionKeyword(tc.text, tc.keyword, tc.wordBoundary); got != tc.want {
					t.Errorf("matchStaticReactionKeyword(%q, %q, %v) = %v, want %v", tc.text, tc.keyword, tc.wordBoundary, got, tc.want)
				}
			})
		}
	})

	t.Run("caption matching chance cooldowns and safety gates", func(t *testing.T) {
		env := newChatReactEnv(t)
		LoadReactions(env.dispatcher)
		env.mu.Lock()
		env.presSettings.AIReactMode = presence.AIReactModeOff
		env.mu.Unlock()

		rules := []reactions.Rule{
			{ID: 1, Keyword: "alpha", Emoji: "👍", WordBoundary: true, MatchCaption: false, ChancePct: 100, CooldownSecs: 0},
			{ID: 2, Keyword: "cap", Emoji: "🔥", WordBoundary: true, MatchCaption: true, ChancePct: 100, CooldownSecs: 0},
			{ID: 3, Keyword: "chance", Emoji: "🎉", WordBoundary: true, MatchCaption: false, ChancePct: 50, CooldownSecs: 0},
			{ID: 4, Keyword: "cool", Emoji: "🤝", WordBoundary: false, MatchCaption: false, ChancePct: 100, CooldownSecs: 60},
		}
		chatSwap(t, &reactionWrites.Rules, func(context.Context, int64) ([]reactions.Rule, error) {
			return rules, nil
		})
		chatSwap(t, &staticChatCooldown, 0)
		alice := env.member("Alice")

		// 1. Caption-only skips MatchCaption=false, fires MatchCaption=true.
		capNo := env.message(alice, "")
		capNo.Caption = "only alpha here"
		env.deliver(capNo)
		if got := len(env.callsFor("setMessageReaction")); got != 0 {
			t.Fatalf("caption with MatchCaption=false set %d reactions, want 0", got)
		}

		capYes := env.message(alice, "")
		capYes.Caption = "a cap!"
		env.deliver(capYes)
		calls := env.callsFor("setMessageReaction")
		if len(calls) != 1 || !strings.Contains(calls[0].Params["reaction"], "🔥") {
			t.Fatalf("caption with MatchCaption=true calls = %+v, want 🔥", calls)
		}

		// 2. ChancePct gating via staticReactionRoll.
		var rollResult bool
		chatSwap(t, &staticReactionRoll, func(pct int) bool {
			return pct >= 100 || rollResult
		})
		env.deliver(env.message(alice, "chance"))
		if got := len(env.callsFor("setMessageReaction")); got != 1 {
			t.Fatalf("failed chance roll reacted (%d calls)", got)
		}
		rollResult = true
		env.deliver(env.message(alice, "chance"))
		if got := len(env.callsFor("setMessageReaction")); got != 2 {
			t.Fatalf("passed chance roll did not react (%d calls)", got)
		}

		// 3. Per-rule Redis cooldown lives outside xenobot:cache: and lets another matching rule win.
		ruleKey := staticRuleCooldownKey(env.chatID, 4)
		if strings.HasPrefix(ruleKey, "xenobot:cache:") || !strings.HasPrefix(ruleKey, presenceKeyPrefix) {
			t.Fatalf("staticRuleCooldownKey = %q, want under %s outside xenobot:cache:", ruleKey, presenceKeyPrefix)
		}
		env.deliver(env.message(alice, "cool"))
		if got := len(env.callsFor("setMessageReaction")); got != 3 {
			t.Fatalf("first cool message calls = %d, want 3", got)
		}
		if !env.mr.Exists(ruleKey) {
			t.Fatalf("expected Redis rule cooldown key %q to exist", ruleKey)
		}
		// While rule 4 ("cool") is on cooldown, "alpha cool" skips rule 4 and fires rule 1 ("alpha").
		env.deliver(env.message(alice, "alpha cool"))
		calls = env.callsFor("setMessageReaction")
		if len(calls) != 4 || !strings.Contains(calls[3].Params["reaction"], "👍") {
			t.Fatalf("fallback rule during cooldown = %+v, want 👍", calls)
		}
		// Still cooled down for "cool" alone.
		env.deliver(env.message(alice, "cool"))
		if got := len(env.callsFor("setMessageReaction")); got != 4 {
			t.Fatalf("cooled-down rule reacted early (%d calls)", got)
		}
		// Advance miniredis clock past 60s -> rule 4 fires again.
		env.mr.FastForward(61 * time.Second)
		env.deliver(env.message(alice, "cool"))
		if got := len(env.callsFor("setMessageReaction")); got != 5 {
			t.Fatalf("rule after cooldown expiry calls = %d, want 5", got)
		}

		// 4. Per-chat anti-burst cooldown lives outside xenobot:cache: and prevents bursts.
		chatKey := staticChatCooldownKey(env.chatID)
		if strings.HasPrefix(chatKey, "xenobot:cache:") || !strings.HasPrefix(chatKey, presenceKeyPrefix) {
			t.Fatalf("staticChatCooldownKey = %q, want under %s outside xenobot:cache:", chatKey, presenceKeyPrefix)
		}
		chatSwap(t, &staticChatCooldown, 10*time.Second)
		env.deliver(env.message(alice, "alpha"))
		if got := len(env.callsFor("setMessageReaction")); got != 6 {
			t.Fatalf("first burst message calls = %d, want 6", got)
		}
		if !env.mr.Exists(chatKey) {
			t.Fatalf("expected Redis chat cooldown key %q", chatKey)
		}
		env.deliver(env.message(alice, "alpha"))
		if got := len(env.callsFor("setMessageReaction")); got != 6 {
			t.Fatalf("second burst message within chat cooldown reacted (%d calls)", got)
		}
		env.mr.FastForward(11 * time.Second)
		env.deliver(env.message(alice, "alpha"))
		if got := len(env.callsFor("setMessageReaction")); got != 7 {
			t.Fatalf("message after chat cooldown expiry calls = %d, want 7", got)
		}
		chatSwap(t, &staticChatCooldown, 0)

		// 5. Staff Group skip with 0 lockdown queries, plus private/channel/slash/bot/sender_chat skips.
		var lockdownCalls int
		chatSwap(t, &presenceLockdownLookup, func(int64) (*models.ChatLockdown, error) {
			lockdownCalls++
			return nil, nil
		})
		chatSwap(t, &presenceStaffLookup, func(int64) *models.StaffGroup {
			return &models.StaffGroup{ChatID: env.chatID}
		})
		env.deliver(env.message(alice, "alpha"))
		if got := len(env.callsFor("setMessageReaction")); got != 7 || lockdownCalls != 0 {
			t.Fatalf("staff group: reactions=%d lockdownCalls=%d, want 7 and 0", got, lockdownCalls)
		}
		chatSwap(t, &presenceStaffLookup, func(int64) *models.StaffGroup { return nil })

		priv := env.message(alice, "alpha")
		priv.Chat.Type = "private"
		env.deliver(priv)

		ch := env.message(alice, "alpha")
		ch.Chat.Type = "channel"
		env.deliver(ch)

		env.deliver(env.message(alice, "/alpha"))

		botUser := &gotgbot.User{Id: 888, IsBot: true, FirstName: "OtherBot"}
		env.deliver(env.message(botUser, "alpha"))

		anon := env.message(alice, "alpha")
		anon.SenderChat = &gotgbot.Chat{Id: -100555, Type: "channel"}
		env.deliver(anon)

		if got := len(env.callsFor("setMessageReaction")); got != 7 || lockdownCalls != 0 {
			t.Fatalf("disallowed senders: reactions=%d lockdownCalls=%d, want 7 and 0", got, lockdownCalls)
		}

		// 6. Active lockdown and DB error both fail closed.
		chatSwap(t, &presenceLockdownLookup, func(int64) (*models.ChatLockdown, error) {
			return &models.ChatLockdown{ChatID: env.chatID}, nil
		})
		env.deliver(env.message(alice, "alpha"))
		if got := len(env.callsFor("setMessageReaction")); got != 7 {
			t.Fatalf("active lockdown reacted (%d calls)", got)
		}

		chatSwap(t, &presenceLockdownLookup, func(int64) (*models.ChatLockdown, error) {
			return nil, errors.New("db down")
		})
		env.deliver(env.message(alice, "alpha"))
		if got := len(env.callsFor("setMessageReaction")); got != 7 {
			t.Fatalf("lockdown lookup error did not fail closed (%d calls)", got)
		}
	})

	t.Run("applyReactionSetOptions bounds and missing row", func(t *testing.T) {
		var seamCalled bool
		chatSwap(t, &reactionWrites.SetOptionsByID, func(context.Context, int64, int64, bool, bool, int, int) (*models.Reactions, error) {
			seamCalled = true
			return nil, nil
		})
		for _, bad := range []struct{ ch, cd int }{
			{0, 0},
			{101, 0},
			{100, -1},
			{100, 86401},
		} {
			res := applyReactionSetOptions(context.Background(), -1001, 1, true, false, bad.ch, bad.cd)
			if res.Outcome != reactionOptionsInvalid {
				t.Errorf("applyReactionSetOptions(ch=%d, cd=%d) = %v, want reactionOptionsInvalid", bad.ch, bad.cd, res.Outcome)
			}
		}
		if seamCalled {
			t.Fatalf("bounds rejection called SetOptionsByID")
		}
		res := applyReactionSetOptions(context.Background(), -1001, 999, true, false, 50, 30)
		if res.Outcome != reactionOptionsAbsent {
			t.Errorf("missing row outcome = %v, want reactionOptionsAbsent", res.Outcome)
		}
	})
}

func TestCatStickerMicroTriggers(t *testing.T) {
	env := newChatReactEnv(t)
	LoadReactions(env.dispatcher)
	prevEnableChat := config.AppConfig.EnableChat
	config.AppConfig.EnableChat = false
	t.Cleanup(func() { config.AppConfig.EnableChat = prevEnableChat })
	env.mu.Lock()
	env.presSettings.AIReactMode = presence.AIReactModeOff
	env.presSettings.CatStickerEnabled = false
	env.mu.Unlock()

	chatSwap(t, &reactionWrites.Rules, func(context.Context, int64) ([]reactions.Rule, error) {
		return nil, nil
	})
	chatSwap(t, &catStickerRoll, func() bool { return true })
	alice := env.member("Alice")

	stickerMsg := func(emoji string) *gotgbot.Message {
		m := env.message(alice, "")
		m.Sticker = &gotgbot.Sticker{FileId: "stk-1", FileUniqueId: "u-stk-1", Emoji: emoji}
		return m
	}

	// 1. CatStickerEnabled=false -> no reaction.
	env.deliver(stickerMsg("🐱"))
	if got := len(env.callsFor("setMessageReaction")); got != 0 {
		t.Fatalf("disabled cat sticker reacted (%d calls)", got)
	}

	// 2. Non-cat sticker makes zero presenceSettingsGet or lockdown calls.
	var presReads, lockdownCalls int
	origGet := presenceSettingsGet
	chatSwap(t, &presenceSettingsGet, func(ctx context.Context, id int64) (presence.Settings, error) {
		presReads++
		return origGet(ctx, id)
	})
	chatSwap(t, &presenceLockdownLookup, func(int64) (*models.ChatLockdown, error) {
		lockdownCalls++
		return nil, nil
	})
	env.mu.Lock()
	env.presSettings.CatStickerEnabled = true
	env.mu.Unlock()
	env.deliver(stickerMsg("🐶"))
	if presReads != 0 || lockdownCalls != 0 || len(env.callsFor("setMessageReaction")) != 0 {
		t.Fatalf("non-cat sticker: presReads=%d lockdownCalls=%d reactions=%d", presReads, lockdownCalls, len(env.callsFor("setMessageReaction")))
	}

	// 3. Cat glyph (including black cat and variation selector) reacts with allowed 👀 or ❤ behind Redis cooldown.
	catKey := catStickerCooldownKey(env.chatID)
	if strings.HasPrefix(catKey, "xenobot:cache:") || !strings.HasPrefix(catKey, presenceKeyPrefix) {
		t.Fatalf("catStickerCooldownKey = %q, want under %s outside xenobot:cache:", catKey, presenceKeyPrefix)
	}
	env.deliver(stickerMsg("🐈‍⬛"))
	calls := env.callsFor("setMessageReaction")
	if len(calls) != 1 {
		t.Fatalf("cat sticker reactions = %d, want 1", len(calls))
	}
	if !strings.Contains(calls[0].Params["reaction"], "👀") && !strings.Contains(calls[0].Params["reaction"], "❤") {
		t.Fatalf("cat sticker chose %q, want 👀 or ❤", calls[0].Params["reaction"])
	}
	if !env.mr.Exists(catKey) {
		t.Fatalf("expected cat cooldown key %q", catKey)
	}
	if got := len(env.provider.calls()); got != 0 {
		t.Fatalf("cat sticker made %d AI calls, want 0", got)
	}

	// 4. Cooldown blocks second cat sticker; fast-forward clears it.
	env.deliver(stickerMsg("😻"))
	if got := len(env.callsFor("setMessageReaction")); got != 1 {
		t.Fatalf("cat sticker during cooldown reacted (%d calls)", got)
	}
	env.mr.FastForward(catStickerCooldown + time.Second)

	// 5. AvailableReactions restricts choice: only 👀 allowed -> always 👀; neither 👀 nor ❤ -> 0 reactions.
	env.mu.Lock()
	env.availReactions = `[{"type":"emoji","emoji":"👀"},{"type":"emoji","emoji":"🔥"}]`
	env.mu.Unlock()
	dbcache.DeleteCache(presenceAvailReactKey(env.chatID))
	env.deliver(stickerMsg("😸"))
	calls = env.callsFor("setMessageReaction")
	if len(calls) != 2 || !strings.Contains(calls[1].Params["reaction"], "👀") {
		t.Fatalf("restricted palette cat reaction = %+v, want 👀", calls)
	}

	env.mr.FastForward(catStickerCooldown + time.Second)
	env.mu.Lock()
	env.availReactions = `[{"type":"emoji","emoji":"🔥"}]`
	env.mu.Unlock()
	dbcache.DeleteCache(presenceAvailReactKey(env.chatID))
	env.deliver(stickerMsg("🐱"))
	if got := len(env.callsFor("setMessageReaction")); got != 2 {
		t.Fatalf("cat sticker with neither 👀 nor ❤ allowed reacted (%d calls)", got)
	}
	env.mu.Lock()
	env.availReactions = ""
	env.mu.Unlock()
	dbcache.DeleteCache(presenceAvailReactKey(env.chatID))

	// 6. Staff Group & active lockdown block cat sticker.
	env.mr.FastForward(catStickerCooldown + time.Second)
	chatSwap(t, &presenceStaffLookup, func(int64) *models.StaffGroup { return &models.StaffGroup{ChatID: env.chatID} })
	env.deliver(stickerMsg("🐱"))
	if got := len(env.callsFor("setMessageReaction")); got != 2 {
		t.Fatalf("staff group cat sticker reacted (%d calls)", got)
	}
	chatSwap(t, &presenceStaffLookup, func(int64) *models.StaffGroup { return nil })

	chatSwap(t, &presenceLockdownLookup, func(int64) (*models.ChatLockdown, error) {
		return &models.ChatLockdown{ChatID: env.chatID}, nil
	})
	env.deliver(stickerMsg("🐱"))
	if got := len(env.callsFor("setMessageReaction")); got != 2 {
		t.Fatalf("locked-down cat sticker reacted (%d calls)", got)
	}
}

func TestStaticAndCatReactionsHandoffToGroup13(t *testing.T) {
	env := newChatReactEnv(t)
	LoadReactions(env.dispatcher)
	env.provider.script(chatAnswerText(`{"emoji":"🔥"}`))
	env.mu.Lock()
	env.presSettings.AIReactMode = presence.AIReactModeMedium
	env.presSettings.CatStickerEnabled = true
	env.mu.Unlock()

	chatSwap(t, &staticChatCooldown, 0)
	chatSwap(t, &catStickerRoll, func() bool { return true })
	chatSwap(t, &reactionWrites.Rules, func(context.Context, int64) ([]reactions.Rule, error) {
		return []reactions.Rule{
			{ID: 10, Keyword: "hello", Emoji: "👍", WordBoundary: true, ChancePct: 100},
		}, nil
	})
	alice := env.member("Alice")

	// 1. Static rule in group 8 reacts 👍 and sets presenceStaticReactedKey -> group 13 makes 0 AI calls.
	staticMsg := env.message(alice, "hello team!")
	ctxStatic := ext.NewContext(env.bot, &gotgbot.Update{UpdateId: 1001, Message: staticMsg}, nil)
	if err := reactionsModule.checkReactions(env.bot, ctxStatic); err != ext.ContinueGroups {
		t.Fatalf("checkReactions(static) = %v", err)
	}
	if reacted, _ := ctxStatic.Data[presenceStaticReactedKey].(bool); !reacted {
		t.Fatalf("ctx.Data[%q] = %v, want true", presenceStaticReactedKey, ctxStatic.Data[presenceStaticReactedKey])
	}
	env.deliver(env.message(alice, "hello again!"))
	if calls := env.callsFor("setMessageReaction"); len(calls) != 2 || !strings.Contains(calls[1].Params["reaction"], "👍") {
		t.Fatalf("group 8+13 static dispatch reactions = %+v, want 👍", calls)
	}
	if got := len(env.provider.calls()); got != 0 {
		t.Fatalf("group 13 AI called %d times after static reaction, want 0", got)
	}

	// 2. Cat sticker in group 8 reacts 👀/❤ and sets presenceStaticReactedKey -> group 13 makes 0 AI calls.
	env.mr.FastForward(catStickerCooldown + time.Second)
	catMsg := env.message(alice, "")
	catMsg.Sticker = &gotgbot.Sticker{FileId: "stk-cat", FileUniqueId: "u-stk-cat", Emoji: "🐱"}
	ctxCat := ext.NewContext(env.bot, &gotgbot.Update{UpdateId: 1002, Message: catMsg}, nil)
	if err := reactionsModule.checkReactions(env.bot, ctxCat); err != ext.ContinueGroups {
		t.Fatalf("checkReactions(cat) = %v", err)
	}
	if reacted, _ := ctxCat.Data[presenceStaticReactedKey].(bool); !reacted {
		t.Fatalf("ctx.Data[%q] after cat sticker = %v, want true", presenceStaticReactedKey, ctxCat.Data[presenceStaticReactedKey])
	}
	env.mr.FastForward(catStickerCooldown + time.Second)
	catMsg2 := env.message(alice, "")
	catMsg2.Sticker = &gotgbot.Sticker{FileId: "stk-cat-2", FileUniqueId: "u-stk-cat-2", Emoji: "😹"}
	env.deliver(catMsg2)
	if got := len(env.callsFor("setMessageReaction")); got != 4 {
		t.Fatalf("cat sticker dispatch total reactions = %d, want 4", got)
	}
	if got := len(env.provider.calls()); got != 0 {
		t.Fatalf("group 13 AI called %d times after cat reaction, want 0", got)
	}

	// 3. Unmatched message falls through group 8 to group 13 AI reaction.
	env.deliver(env.message(alice, "what a wild evening"))
	if got := len(env.provider.calls()); got != 1 {
		t.Fatalf("unmatched message AI provider calls = %d, want 1", got)
	}
}
