package presence

import (
	"context"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Reaction frequency of one kind of message (text, sticker, photo). Zero is the value a chat has
// today, so an unset column and an old cached value read as Normal.
const (
	// ReactFreqNormal is the stored default: the AI reaction mode's own rate.
	ReactFreqNormal = 0
	// ReactFreqOff never reacts to that kind.
	ReactFreqOff = 1
	// ReactFreqRare reacts about half as often.
	ReactFreqRare = 2
	// ReactFreqOften reacts about twice as often.
	ReactFreqOften = 3

	// KindText, KindSticker and KindPhoto name the kinds a frequency is set for.
	KindText    = "text"
	KindSticker = "sticker"
	KindPhoto   = "photo"

	// MaxReactTopics and MaxTopicRunes bound the topics an AI reaction is limited to.
	MaxReactTopics = 10
	MaxTopicRunes  = 24
	// MaxBotNames and MaxBotNameRunes bound the extra names the bot answers to.
	MaxBotNames     = 8
	MaxBotNameRunes = 32
	minBotNameRunes = 2
)

var (
	// ErrListTooMany is returned for more entries than the list allows.
	ErrListTooMany = errors.New("presence: too many entries")
	// ErrListTooLong is returned for an entry longer than the list allows.
	ErrListTooLong = errors.New("presence: entry too long")
	// ErrListInvalid is returned for an entry with a character the list does not allow, or one that
	// is too short.
	ErrListInvalid = errors.New("presence: invalid entry")
)

// ValidReactFreq reports whether n is a stored frequency.
func ValidReactFreq(n int) bool {
	return n >= ReactFreqNormal && n <= ReactFreqOften
}

// ValidReactKind reports whether kind is a message kind a frequency is set for.
func ValidReactKind(kind string) bool {
	return kind == KindText || kind == KindSticker || kind == KindPhoto
}

// ReactFreqOf returns the stored frequency of a kind.
func (s Settings) ReactFreqOf(kind string) int {
	switch kind {
	case KindText:
		return normalizeReactFreq(s.ReactTextFreq)
	case KindSticker:
		return normalizeReactFreq(s.ReactStickerFreq)
	case KindPhoto:
		return normalizeReactFreq(s.ReactPhotoFreq)
	}
	return ReactFreqNormal
}

func normalizeReactFreq(n int) int {
	if ValidReactFreq(n) {
		return n
	}
	return ReactFreqNormal
}

// TopicList is the stored topics as a list (empty for none).
func (s Settings) TopicList() []string { return splitStoredList(s.ReactTopics) }

// BotNameList is the stored extra bot names as a list (empty for none).
func (s Settings) BotNameList() []string { return splitStoredList(s.BotNames) }

func splitStoredList(stored string) []string {
	if strings.TrimSpace(stored) == "" {
		return nil
	}
	parts := strings.Split(stored, "\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// splitInputList splits a typed list on newlines, commas, semicolons and the CJK and Arabic commas.
func splitInputList(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case '\n', '\r', ',', ';', '，', '、', '،':
			return true
		}
		return false
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = cleanListEntry(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// cleanListEntry drops control, format and private-use characters, collapses white space and trims.
func cleanListEntry(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsSpace(r):
			b.WriteRune(' ')
		case unicode.IsControl(r), unicode.Is(unicode.Cf, r), unicode.Is(unicode.Co, r):
		default:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// dedupeFold removes repeats that differ only by letter case, keeping the first.
func dedupeFold(list []string) []string {
	seen := make(map[string]struct{}, len(list))
	out := list[:0]
	for _, e := range list {
		key := strings.ToLower(e)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, e)
	}
	return out
}

// NormalizeTopics reads a typed topic list (separated by commas, semicolons or new lines) into at most
// MaxReactTopics topics of at most MaxTopicRunes each, without repeats. A topic may hold words and
// spaces but no angle brackets, braces or backticks (it is shown to a model, so it stays plain). It
// returns the list, or one of ErrListTooMany, ErrListTooLong and ErrListInvalid.
func NormalizeTopics(raw string) ([]string, error) {
	list := dedupeFold(splitInputList(raw))
	if len(list) > MaxReactTopics {
		return nil, ErrListTooMany
	}
	for _, t := range list {
		if utf8.RuneCountInString(t) > MaxTopicRunes {
			return nil, ErrListTooLong
		}
		if strings.ContainsAny(t, "<>{}`[]\\|") {
			return nil, ErrListInvalid
		}
	}
	return list, nil
}

// NormalizeBotNames reads a typed list of extra names the bot answers to: at most MaxBotNames names of
// MaxBotNameRunes runes at most, each one word of letters, digits, hyphen, underscore or apostrophe
// (the name must start with a letter or digit, so it can never start with / or @), without repeats.
func NormalizeBotNames(raw string) ([]string, error) {
	list := dedupeFold(splitInputList(raw))
	if len(list) > MaxBotNames {
		return nil, ErrListTooMany
	}
	for _, n := range list {
		runes := utf8.RuneCountInString(n)
		if runes > MaxBotNameRunes {
			return nil, ErrListTooLong
		}
		if runes < minBotNameRunes {
			return nil, ErrListInvalid
		}
		for i, r := range n {
			ok := unicode.IsLetter(r) || unicode.IsDigit(r)
			if i > 0 {
				ok = ok || r == '-' || r == '_' || r == '\''
			}
			if !ok {
				return nil, ErrListInvalid
			}
		}
	}
	return list, nil
}

// SetReactFreq writes the frequency of one kind with a column-targeted upsert and drops the cache key.
func SetReactFreq(ctx context.Context, chatID int64, kind string, freq int, by int64) error {
	if !ValidReactKind(kind) || !ValidReactFreq(freq) {
		return ErrInvalidValue
	}
	row := defaultRow(chatID, by)
	var col string
	switch kind {
	case KindText:
		row.ReactTextFreq, col = int16(freq), "react_text_freq" // #nosec G115 -- validated as 0 to 3
	case KindSticker:
		row.ReactStickerFreq, col = int16(freq), "react_sticker_freq" // #nosec G115 -- validated as 0 to 3
	default:
		row.ReactPhotoFreq, col = int16(freq), "react_photo_freq" // #nosec G115 -- validated as 0 to 3
	}
	return upsertColumns(ctx, row, []string{col})
}

// SetReactTopics validates raw with NormalizeTopics and stores the topics (one per line; empty
// clears them) with a column-targeted upsert, dropping the cache key.
func SetReactTopics(ctx context.Context, chatID int64, raw string, by int64) error {
	list, err := NormalizeTopics(raw)
	if err != nil {
		return err
	}
	row := defaultRow(chatID, by)
	row.ReactTopics = strings.Join(list, "\n")
	return upsertColumns(ctx, row, []string{"react_topics"})
}

// SetBotNames validates raw with NormalizeBotNames and stores the names (one per line; empty clears
// them) with a column-targeted upsert, dropping the cache key.
func SetBotNames(ctx context.Context, chatID int64, raw string, by int64) error {
	list, err := NormalizeBotNames(raw)
	if err != nil {
		return err
	}
	row := defaultRow(chatID, by)
	row.BotNames = strings.Join(list, "\n")
	return upsertColumns(ctx, row, []string{"bot_names"})
}
