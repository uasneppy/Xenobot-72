//go:build testtools

package modules

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/Xenobot-72/xenobot/db/chatconv"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/db/presence"
)

// TestChatReactScaleProfile holds the pure frequency maths: Rare halves the chance and doubles the
// gap and both cooldowns, Often doubles the chance (at most 100) and halves them, Normal is the
// profile as it is.
func TestChatReactScaleProfile(t *testing.T) {
	base := chatReactProfile{minGap: 15, chatCooldown: 8 * time.Minute, userCooldown: 5 * time.Minute, chancePct: 25}
	cases := []struct {
		name string
		in   chatReactProfile
		freq int
		want chatReactProfile
	}{
		{"normal", base, presence.ReactFreqNormal, base},
		{"rare", base, presence.ReactFreqRare, chatReactProfile{minGap: 30, chatCooldown: 16 * time.Minute, userCooldown: 10 * time.Minute, chancePct: 12}},
		{"often", base, presence.ReactFreqOften, chatReactProfile{minGap: 7, chatCooldown: 4 * time.Minute, userCooldown: 150 * time.Second, chancePct: 50}},
		{"rare chance never reaches zero", chatReactProfile{minGap: 1, chancePct: 1}, presence.ReactFreqRare, chatReactProfile{minGap: 2, chancePct: 1}},
		{"often chance capped at 100", chatReactProfile{minGap: 1, chancePct: 70}, presence.ReactFreqOften, chatReactProfile{minGap: 1, chancePct: 100}},
	}
	for _, tc := range cases {
		if got := chatReactScaleProfile(tc.in, tc.freq); got != tc.want {
			t.Errorf("%s: got %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

func TestChatReactKindOf(t *testing.T) {
	cases := map[string]*gotgbot.Message{
		presence.KindText:    {Text: "hi"},
		presence.KindSticker: {Sticker: &gotgbot.Sticker{FileId: "s"}},
		presence.KindPhoto:   {Photo: []gotgbot.PhotoSize{{FileId: "p"}}},
	}
	for want, msg := range cases {
		if got := chatReactKindOf(msg); got != want {
			t.Errorf("kind of %v = %q, want %q", want, got, want)
		}
	}
}

// TestChatReactFrequencyPerKind: a kind set to Off never reaches Redis or the provider while the
// other kinds keep reacting, and Rare and Often change the roll the gate makes.
func TestChatReactFrequencyPerKind(t *testing.T) {
	t.Run("off is silent for its kind only and does no Redis I/O", func(t *testing.T) {
		env := newChatReactEnv(t)
		alice := env.member("Alice")
		env.mu.Lock()
		env.presSettings.ReactPhotoFreq = presence.ReactFreqOff
		env.presSettings.ReactStickerFreq = presence.ReactFreqOff
		env.mu.Unlock()

		photo := env.message(alice, "")
		photo.Photo = []gotgbot.PhotoSize{{FileId: "p1", FileUniqueId: "up1", Width: 800, Height: 600}}
		env.deliver(photo)
		sticker := env.message(alice, "")
		sticker.Sticker = &gotgbot.Sticker{FileId: "s1", FileUniqueId: "us1", Emoji: "😀"}
		env.deliver(sticker)
		if got := len(env.provider.calls()); got != 0 {
			t.Fatalf("off kinds made %d provider calls, want 0", got)
		}
		if keys := env.mr.Keys(); len(keys) != 0 {
			t.Fatalf("off kinds touched Redis: %v", keys)
		}

		env.provider.script(chatAnswerText(`{"emoji":"🔥"}`))
		env.deliver(env.message(alice, "shipped it"))
		if got := len(env.callsFor("setMessageReaction")); got != 1 {
			t.Fatalf("text kept reacting: reactions = %d, want 1", got)
		}
	})

	t.Run("text off blocks text", func(t *testing.T) {
		env := newChatReactEnv(t)
		alice := env.member("Alice")
		env.mu.Lock()
		env.presSettings.ReactTextFreq = presence.ReactFreqOff
		env.mu.Unlock()
		env.deliver(env.message(alice, "shipped it"))
		if got := len(env.provider.calls()); got != 0 {
			t.Fatalf("text off made %d provider calls, want 0", got)
		}
	})

	t.Run("rare and often scale the roll", func(t *testing.T) {
		env := newChatReactEnv(t)
		alice := env.member("Alice")
		chatSwap(t, &chatReactProfiles, map[string]chatReactProfile{
			presence.AIReactModeMedium: {minGap: 1, chancePct: 40},
		})
		var rolled atomic.Int64
		chatSwap(t, &chatReactRoll, func(chancePct int) bool { rolled.Store(int64(chancePct)); return false })
		for freq, want := range map[int]int64{
			presence.ReactFreqNormal: 40,
			presence.ReactFreqRare:   20,
			presence.ReactFreqOften:  80,
		} {
			env.mu.Lock()
			env.presSettings.ReactTextFreq = freq
			env.mu.Unlock()
			rolled.Store(0)
			env.deliver(env.message(alice, "a message"))
			if got := rolled.Load(); got != want {
				t.Errorf("frequency %d rolled %d, want %d", freq, got, want)
			}
		}
	})
}

// TestChatReactTopics: the group's topics reach the prompt, and a NONE answer to a topic-limited job
// gives its cooldowns back so the next message is tried, while without topics a NONE keeps them.
func TestChatReactTopics(t *testing.T) {
	slow := map[string]chatReactProfile{
		presence.AIReactModeMedium: {minGap: 1, chatCooldown: 5 * time.Minute, userCooldown: 5 * time.Minute, chancePct: 100},
	}
	t.Run("topics reach the prompt and NONE gives the cooldown back", func(t *testing.T) {
		env := newChatReactEnv(t)
		chatSwap(t, &chatReactProfiles, slow)
		alice := env.member("Alice")
		env.mu.Lock()
		env.presSettings.ReactTopics = "memes\ncats"
		env.mu.Unlock()
		env.provider.script(chatAnswerText(`{"emoji":"NONE"}`), chatAnswerText(`{"emoji":"🔥"}`))

		env.deliver(env.message(alice, "the weather is mild"))
		env.deliver(env.message(alice, "look at this funny meme"))
		calls := env.provider.calls()
		if len(calls) != 2 {
			t.Fatalf("provider calls = %d, want 2 (the off-topic NONE must not start a cooldown)", len(calls))
		}
		for _, want := range []string{"memes", "cats"} {
			if !strings.Contains(calls[0].Dynamic, want) {
				t.Errorf("prompt lacks topic %q: %q", want, calls[0].Dynamic)
			}
		}
		if got := len(env.callsFor("setMessageReaction")); got != 1 {
			t.Fatalf("reactions = %d, want 1", got)
		}
	})
	t.Run("without topics NONE keeps the cooldown", func(t *testing.T) {
		env := newChatReactEnv(t)
		chatSwap(t, &chatReactProfiles, slow)
		alice := env.member("Alice")
		env.provider.script(chatAnswerText(`{"emoji":"NONE"}`), chatAnswerText(`{"emoji":"🔥"}`))
		env.deliver(env.message(alice, "the weather is mild"))
		env.deliver(env.message(alice, "look at this funny meme"))
		if got := len(env.provider.calls()); got != 1 {
			t.Fatalf("provider calls = %d, want 1 (a deliberate NONE is a turn)", got)
		}
	})
}

// TestChatBotNamesAddressing: the group's own names are answered to under name addressing, only
// while the switch is on, and a name a group set never leaks into another group.
func TestChatBotNamesAddressing(t *testing.T) {
	env := newChatEnv(t)
	chatSwap(t, &chatCooldown, time.Millisecond)
	conv := chatconv.Settings{Enabled: true, Provider: chatconv.ProviderAnthropic, Audience: chatconv.AudienceEveryone}
	pres := presence.Settings{VocativeEnabled: true, BotNames: "Спайк\nBuddy"}
	chatSwap(t, &chatSettingsLookup, func(context.Context, int64) (chatconv.Settings, error) { return conv, nil })
	chatSwap(t, &presenceSettingsGet, func(context.Context, int64) (presence.Settings, error) { return pres, nil })
	chatSwap(t, &presenceStaffLookup, func(int64) *models.StaffGroup { return nil })
	alice := env.member("Alice")

	env.deliver(env.message(alice, "Спайк, перевір реактор"))
	env.mr.FastForward(time.Second)
	env.deliver(env.message(alice, "buddy check the reactor"))
	if got := len(env.sends()); got != 2 {
		t.Fatalf("custom-name sends = %d, want 2", got)
	}
	calls := env.provider.calls()
	if !strings.Contains(calls[0].User, "Alice: перевір реактор") || strings.Contains(calls[0].User, "Спайк") {
		t.Fatalf("custom name not stripped: %q", calls[0].User)
	}
	if !strings.Contains(calls[1].User, "Alice: check the reactor") {
		t.Fatalf("custom name (case-insensitive) not stripped: %q", calls[1].User)
	}

	// A name nobody set is not an address.
	env.mr.FastForward(time.Second)
	env.deliver(env.message(alice, "Rex, check the reactor"))
	if got := len(env.sends()); got != 2 {
		t.Fatalf("unset name answered: sends = %d, want still 2", got)
	}

	// The switch off disables every name, built-in and custom.
	pres.VocativeEnabled = false
	env.mr.FastForward(time.Second)
	env.deliver(env.message(alice, "Спайк, ще раз"))
	if got := len(env.sends()); got != 2 {
		t.Fatalf("vocative off answered: sends = %d, want still 2", got)
	}
}

// TestCatStickerRollFor: Off never fires, Normal is the roll, Rare needs an extra coin and Often
// gets a second roll.
func TestCatStickerRollFor(t *testing.T) {
	var roll, coin bool
	chatSwap(t, &catStickerRoll, func() bool { return roll })
	chatSwap(t, &catStickerCoin, func() bool { return coin })
	type c struct {
		freq       int
		roll, coin bool
		want       bool
	}
	for _, tc := range []c{
		{presence.ReactFreqOff, true, true, false},
		{presence.ReactFreqNormal, true, false, true},
		{presence.ReactFreqNormal, false, true, false},
		{presence.ReactFreqRare, true, true, true},
		{presence.ReactFreqRare, true, false, false},
		{presence.ReactFreqRare, false, true, false},
		{presence.ReactFreqOften, true, false, true},
		{presence.ReactFreqOften, false, false, false},
	} {
		roll, coin = tc.roll, tc.coin
		if got := catStickerRollFor(tc.freq); got != tc.want {
			t.Errorf("freq %d roll %v coin %v: got %v, want %v", tc.freq, tc.roll, tc.coin, got, tc.want)
		}
	}
}

// TestChatWatchSharesPresenceReads: name addressing and the reaction watcher need the same two reads (the
// presence settings and the Staff Group answer); each is made once per delivered message, in every branch.
func TestChatWatchSharesPresenceReads(t *testing.T) {
	env := newChatReactEnv(t)
	alice := env.member("Alice")
	var settingsReads, staffReads atomic.Int64
	origGet := presenceSettingsGet
	chatSwap(t, &presenceSettingsGet, func(ctx context.Context, id int64) (presence.Settings, error) {
		settingsReads.Add(1)
		return origGet(ctx, id)
	})
	var staffRow *models.StaffGroup
	chatSwap(t, &presenceStaffLookup, func(int64) *models.StaffGroup {
		staffReads.Add(1)
		return staffRow
	})
	chatSwap(t, &chatReactRoll, func(int) bool { return false })

	measure := func(name string, msg *gotgbot.Message, wantSettings, wantStaff int64) {
		t.Helper()
		settingsReads.Store(0)
		staffReads.Store(0)
		env.deliver(msg)
		if got := settingsReads.Load(); got != wantSettings {
			t.Errorf("%s: presence settings reads = %d, want %d", name, got, wantSettings)
		}
		if got := staffReads.Load(); got != wantStaff {
			t.Errorf("%s: Staff Group lookups = %d, want %d", name, got, wantStaff)
		}
	}

	// An unaddressed text: one of each, shared by both watchers.
	measure("plain text", env.message(alice, "just chatting"), 1, 1)
	// A vocative group: the same, whether or not the text starts with a name.
	env.mu.Lock()
	env.presSettings.VocativeEnabled = true
	env.presSettings.BotNames = "Spike"
	env.mu.Unlock()
	measure("vocative group, plain text", env.message(alice, "just chatting"), 1, 1)
	env.provider.script(chatAnswerText("Notice. Acknowledged."))
	// (An answered message costs one more Staff Group lookup, the reply job's own ambient-context check.)
	measure("custom name", env.message(alice, "Spike, status?"), 1, 2)
	// A sticker has no text: the reaction watcher alone asks, once.
	sticker := env.message(alice, "")
	sticker.Sticker = &gotgbot.Sticker{FileId: "s", FileUniqueId: "us", Emoji: "😀"}
	measure("sticker", sticker, 1, 1)
	// A forward is never name addressing: still one read for the reaction watcher.
	fwd := env.message(alice, "Spike, hi")
	fwd.ForwardOrigin = gotgbot.MessageOriginUser{Date: 1, SenderUser: *alice}
	measure("forward", fwd, 1, 1)
	// A Staff Group: the lookup answers once and nothing else is read.
	staffRow = &models.StaffGroup{ChatID: env.chatID}
	measure("staff group", env.message(alice, "Spike, hi"), 0, 1)
}
