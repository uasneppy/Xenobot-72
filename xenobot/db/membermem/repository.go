// Package membermem stores the short notes the conversation feature keeps about one member of one
// group. A note is private to that group: every query is keyed by the group's chat ID and the
// member's user ID, and nothing here reads across groups.
//
// The rows are read fresh and never cached, so a delete (the member's /forgetme, the owner turning the
// memories or conversations off) is seen by every replica at the next read and no cache key can serve
// a deleted note.
package membermem

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	xenoerrors "github.com/uasneppy/Xenobot-72/xenobot/utils/errors"
)

const (
	// MaxNotes is the most notes kept about one member in one group.
	MaxNotes = 12
	// MaxNoteRunes is the longest note, in runes.
	MaxNoteRunes = 200
)

var errNoDatabase = errors.New("database not initialized")

// Normalize cleans a list of notes: each note has control and format characters dropped and its white
// space collapsed, a note that is empty or longer than MaxNoteRunes is dropped, repeats that differ only
// by letter case are dropped, and at most MaxNotes are kept, the first ones.
func Normalize(notes []string) []string {
	out := make([]string, 0, min(len(notes), MaxNotes))
	seen := make(map[string]struct{}, len(notes))
	for _, raw := range notes {
		note := Clean(raw)
		if note == "" || utf8.RuneCountInString(note) > MaxNoteRunes {
			continue
		}
		key := strings.ToLower(note)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, note)
		if len(out) == MaxNotes {
			break
		}
	}
	return out
}

// Clean drops control, format and private-use characters, collapses white space (a new line included)
// and trims, so a note is one plain line.
func Clean(s string) string {
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

// Get returns the member's notes in this group, fresh from the database (none for a member with no
// row). It creates nothing and returns its error.
func Get(ctx context.Context, chatID, userID int64) ([]string, error) {
	if db.DB == nil {
		return nil, xenoerrors.Wrap(errNoDatabase, "read member memories")
	}
	var row models.ChatMemberMemory
	err := db.DB.WithContext(ctx).Where("chat_id = ? AND user_id = ?", chatID, userID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, xenoerrors.Wrapf(err, "read the memories of member %d in chat %d", userID, chatID)
	}
	return split(row.Notes), nil
}

// Replace stores the member's whole list of notes in this group (normalised first) with one upsert; an
// empty list deletes the row. It reports the notes it stored.
func Replace(ctx context.Context, chatID, userID int64, notes []string) ([]string, error) {
	if db.DB == nil {
		return nil, xenoerrors.Wrap(errNoDatabase, "write member memories")
	}
	kept := Normalize(notes)
	if len(kept) == 0 {
		_, err := Delete(ctx, chatID, userID)
		return nil, err
	}
	row := &models.ChatMemberMemory{ChatID: chatID, UserID: userID, Notes: strings.Join(kept, "\n"), UpdatedAt: time.Now()}
	err := db.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"notes", "updated_at"}),
	}).Create(row).Error
	if err != nil {
		return nil, xenoerrors.Wrapf(err, "write the memories of member %d in chat %d", userID, chatID)
	}
	return kept, nil
}

// ReplaceIf stores notes only while the member's stored list is still exactly the one the caller read
// (expected, nil for no row), and reports whether it wrote. The check and the write are one transaction
// with the row locked, so a list the member deleted meanwhile (their /forgetme, the owner turning the
// feature off) is never brought back by a writer that read it before the delete: a row that is gone
// while expected holds notes, or whose notes changed, writes nothing. An empty list deletes the row (only
// while it still matches).
func ReplaceIf(ctx context.Context, chatID, userID int64, expected, notes []string) (bool, error) {
	if db.DB == nil {
		return false, xenoerrors.Wrap(errNoDatabase, "write member memories")
	}
	kept := Normalize(notes)
	want := Normalize(expected)
	applied := false
	err := db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row models.ChatMemberMemory
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("chat_id = ? AND user_id = ?", chatID, userID).Take(&row).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			if len(want) > 0 {
				return nil // the notes the caller saw were deleted: write nothing
			}
			if len(kept) == 0 {
				applied = true
				return nil
			}
			res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.ChatMemberMemory{
				ChatID: chatID, UserID: userID, Notes: strings.Join(kept, "\n"), UpdatedAt: time.Now(),
			})
			if res.Error != nil {
				return res.Error
			}
			applied = res.RowsAffected == 1
			return nil
		case err != nil:
			return err
		}
		if !slices.Equal(split(row.Notes), want) {
			return nil // someone else changed them since the caller read them
		}
		var res *gorm.DB
		if len(kept) == 0 {
			res = tx.Where("chat_id = ? AND user_id = ?", chatID, userID).Delete(&models.ChatMemberMemory{})
		} else {
			res = tx.Model(&models.ChatMemberMemory{}).Where("chat_id = ? AND user_id = ?", chatID, userID).
				Updates(map[string]any{"notes": strings.Join(kept, "\n"), "updated_at": time.Now()})
		}
		if res.Error != nil {
			return res.Error
		}
		applied = true
		return nil
	})
	if err != nil {
		return false, xenoerrors.Wrapf(err, "write the memories of member %d in chat %d", userID, chatID)
	}
	return applied, nil
}

// Delete removes the member's notes in this group and reports whether there were any.
func Delete(ctx context.Context, chatID, userID int64) (bool, error) {
	if db.DB == nil {
		return false, xenoerrors.Wrap(errNoDatabase, "delete member memories")
	}
	res := db.DB.WithContext(ctx).Where("chat_id = ? AND user_id = ?", chatID, userID).Delete(&models.ChatMemberMemory{})
	if res.Error != nil {
		return false, xenoerrors.Wrapf(res.Error, "delete the memories of member %d in chat %d", userID, chatID)
	}
	return res.RowsAffected > 0, nil
}

// DeleteChat removes every member's notes in this group.
func DeleteChat(ctx context.Context, chatID int64) error {
	if db.DB == nil {
		return xenoerrors.Wrap(errNoDatabase, "delete chat member memories")
	}
	if err := db.DB.WithContext(ctx).Where("chat_id = ?", chatID).Delete(&models.ChatMemberMemory{}).Error; err != nil {
		return xenoerrors.Wrapf(err, "delete the member memories of chat %d", chatID)
	}
	return nil
}

func split(stored string) []string {
	if strings.TrimSpace(stored) == "" {
		return nil
	}
	return Normalize(strings.Split(stored, "\n"))
}
