//go:build testtools

package presence

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
)

func TestNormalizeTopics(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
		err  error
	}{
		{"empty", "  ", []string{}, nil},
		{"commas, semicolons and lines", "memes, cats;  dogs\nfood", []string{"memes", "cats", "dogs", "food"}, nil},
		{"repeats by case collapse", "Cats, cats , CATS", []string{"Cats"}, nil},
		{"words and spaces stay", "office  drama,  retro games", []string{"office drama", "retro games"}, nil},
		{"control and format characters are dropped", "me​mes\x00, ca‮ts", []string{"memes", "cats"}, nil},
		{"unicode topics", "мемі, коти", []string{"мемі", "коти"}, nil},
		{"eleven topics", "a1,a2,a3,a4,a5,a6,a7,a8,a9,a10,a11", nil, ErrListTooMany},
		{"a long topic", strings.Repeat("x", MaxTopicRunes+1), nil, ErrListTooLong},
		{"markup is refused", "memes, <b>cats</b>", nil, ErrListInvalid},
		{"template braces are refused", "{admin}", nil, ErrListInvalid},
	}
	for _, tc := range cases {
		got, err := NormalizeTopics(tc.in)
		if !errors.Is(err, tc.err) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.err)
			continue
		}
		if tc.err == nil && strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestNormalizeBotNames(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
		err  error
	}{
		{"empty", "", []string{}, nil},
		{"several", "Xeno, Ксенобот ,бот-72", []string{"Xeno", "Ксенобот", "бот-72"}, nil},
		{"apostrophe and underscore inside", "o'bot, x_bot", []string{"o'bot", "x_bot"}, nil},
		{"a leading slash is refused", "/xeno", nil, ErrListInvalid},
		{"a leading at sign is refused", "@xeno", nil, ErrListInvalid},
		{"a leading hyphen is refused", "-xeno", nil, ErrListInvalid},
		{"a space inside is refused", "mr bot", nil, ErrListInvalid},
		{"one character is too short", "x", nil, ErrListInvalid},
		{"too long", strings.Repeat("x", MaxBotNameRunes+1), nil, ErrListTooLong},
		{"nine names", "n1,n2,n3,n4,n5,n6,n7,n8,n9", nil, ErrListTooMany},
	}
	for _, tc := range cases {
		got, err := NormalizeBotNames(tc.in)
		if !errors.Is(err, tc.err) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.err)
			continue
		}
		if tc.err == nil && strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestPersonalitySettings(t *testing.T) {
	ctx := context.Background()

	t.Run("a chat with no row reads normal frequencies, no topics and no names", func(t *testing.T) {
		chatID := newChat(t)
		s, err := GetFresh(ctx, chatID)
		if err != nil {
			t.Fatal(err)
		}
		for _, kind := range []string{KindText, KindSticker, KindPhoto} {
			if got := s.ReactFreqOf(kind); got != ReactFreqNormal {
				t.Errorf("%s frequency = %d, want normal", kind, got)
			}
		}
		if len(s.TopicList()) != 0 || len(s.BotNameList()) != 0 {
			t.Errorf("topics %v names %v, want none", s.TopicList(), s.BotNameList())
		}
		if rowCount(t, chatID) != 0 {
			t.Fatal("a read created a row")
		}
	})

	t.Run("writes are column-targeted and never clobber each other", func(t *testing.T) {
		chatID := newChat(t)
		if err := SetReactFreq(ctx, chatID, KindSticker, ReactFreqOff, 7); err != nil {
			t.Fatal(err)
		}
		if err := SetReactFreq(ctx, chatID, KindPhoto, ReactFreqOften, 7); err != nil {
			t.Fatal(err)
		}
		if err := SetReactTopics(ctx, chatID, "memes, cats", 7); err != nil {
			t.Fatal(err)
		}
		if err := SetBotNames(ctx, chatID, "Xeno, Ксенобот", 7); err != nil {
			t.Fatal(err)
		}
		if err := SetAIReactMode(ctx, chatID, AIReactModeLow, 7); err != nil {
			t.Fatal(err)
		}
		s, err := Get(ctx, chatID)
		if err != nil {
			t.Fatal(err)
		}
		if s.ReactFreqOf(KindText) != ReactFreqNormal || s.ReactFreqOf(KindSticker) != ReactFreqOff || s.ReactFreqOf(KindPhoto) != ReactFreqOften {
			t.Errorf("frequencies = %d/%d/%d", s.ReactFreqOf(KindText), s.ReactFreqOf(KindSticker), s.ReactFreqOf(KindPhoto))
		}
		if got := strings.Join(s.TopicList(), "|"); got != "memes|cats" {
			t.Errorf("topics = %q", got)
		}
		if got := strings.Join(s.BotNameList(), "|"); got != "Xeno|Ксенобот" {
			t.Errorf("names = %q", got)
		}
		if s.AIReactMode != AIReactModeLow {
			t.Errorf("a later write to another column lost its value: mode %q", s.AIReactMode)
		}
		// Clearing writes the empty list and a Go zero frequency.
		if err := SetReactTopics(ctx, chatID, "", 7); err != nil {
			t.Fatal(err)
		}
		if err := SetReactFreq(ctx, chatID, KindSticker, ReactFreqNormal, 7); err != nil {
			t.Fatal(err)
		}
		s, _ = Get(ctx, chatID)
		if len(s.TopicList()) != 0 || s.ReactFreqOf(KindSticker) != ReactFreqNormal {
			t.Errorf("clear did not write: topics %v sticker %d", s.TopicList(), s.ReactFreqOf(KindSticker))
		}
	})

	t.Run("invalid values are refused before any write", func(t *testing.T) {
		chatID := newChat(t)
		if err := SetReactFreq(ctx, chatID, "voice", ReactFreqOff, 1); !errors.Is(err, ErrInvalidValue) {
			t.Errorf("unknown kind: %v", err)
		}
		if err := SetReactFreq(ctx, chatID, KindText, 4, 1); !errors.Is(err, ErrInvalidValue) {
			t.Errorf("frequency 4: %v", err)
		}
		if err := SetReactTopics(ctx, chatID, "<b>x</b>", 1); !errors.Is(err, ErrListInvalid) {
			t.Errorf("markup topic: %v", err)
		}
		if err := SetBotNames(ctx, chatID, "/x", 1); !errors.Is(err, ErrListInvalid) {
			t.Errorf("slash name: %v", err)
		}
		if rowCount(t, chatID) != 0 {
			t.Fatal("a refused write created a row")
		}
	})

	t.Run("the table refuses a frequency outside 0..3 and an oversized list", func(t *testing.T) {
		chatID := newChat(t)
		if err := SetReactTopics(ctx, chatID, "memes", 1); err != nil {
			t.Fatal(err)
		}
		if err := db.DB.Model(&models.ChatPresence{}).Where("chat_id = ?", chatID).Update("react_text_freq", 4).Error; err == nil {
			t.Error("react_text_freq = 4 was accepted")
		}
		if err := db.DB.Model(&models.ChatPresence{}).Where("chat_id = ?", chatID).Update("bot_names", strings.Repeat("x", 401)).Error; err == nil {
			t.Error("a 401-character bot_names was accepted")
		}
	})

	t.Run("a stored value that does not validate is normalised on a portable replace", func(t *testing.T) {
		chatID := newChat(t)
		s := Defaults()
		s.ReactTextFreq = 9
		s.ReactTopics = "<script>"
		s.BotNames = "/evil"
		tx := db.DB.Begin()
		defer tx.Rollback()
		if _, err := ReplacePortableTx(tx, chatID, s, 1); err != nil {
			t.Fatal(err)
		}
		var row models.ChatPresence
		if err := tx.Where("chat_id = ?", chatID).Take(&row).Error; err != nil {
			t.Fatal(err)
		}
		if row.ReactTextFreq != 0 || row.ReactTopics != "" || row.BotNames != "" {
			t.Errorf("imported junk reached the table: %+v", row)
		}
	})
}
