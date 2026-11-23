package eventmsg

import (
	"context"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/utils/content"
	fukuerrors "github.com/uasneppy/fuku_robot/fuku/utils/errors"
	"github.com/uasneppy/fuku_robot/fuku/utils/formatting"
	"github.com/uasneppy/fuku_robot/fuku/utils/keyboard"
)

// Validation limits shared by Copy Settings (Phase 29) and backup import (Phase 30), matching
// Telegram limits and the Messages settings editor (fuku/modules/settings_edit_check.go and
// fuku/modules/settings_messages.go).
const (
	MaxTextLength       = 4096
	MaxCaptionLength    = 1024
	MaxButtons          = 100
	MaxRowButtons       = 8
	MreqTextReserve     = 600
	LockdownTextReserve = 400
)

// rulesTokenPattern matches the {rules}, {rules:same} and {rules:up} tokens that add a rules
// button at send time, matching settingsEditRulesTokenPattern.
var rulesTokenPattern = regexp.MustCompile(`(?s){rules(:(same|up))?}`)

// SkippedRow records one candidate row refused by ValidateMessage during ReplaceAllValidated.
type SkippedRow struct {
	Event  string
	Dest   string
	Reason string
}

// WriteReport summarizes the outcome of ReplaceAllValidated.
type WriteReport struct {
	Written int
	Skipped []SkippedRow
}

// allowedMediaType reports whether mediaType is one of the media kinds the message editor stores
// (text and video notes are refused).
func allowedMediaType(mediaType int) bool {
	switch mediaType {
	case db.STICKER, db.DOCUMENT, db.PHOTO, db.AUDIO, db.VOICE, db.VIDEO, db.ANIMATION:
		return true
	default:
		return false
	}
}

// ValidateMessage validates and normalizes one candidate event message row against the same rules
// as the Messages settings editor. It returns the normalized message and an empty reason on
// success, or an empty Message and a machine-readable refusal reason on failure.
func ValidateMessage(msg Message) (Message, string) {
	if !ValidEvent(msg.Event) {
		return Message{}, "unknown_event"
	}
	if !ValidDest(msg.Dest) {
		return Message{}, "unknown_dest"
	}
	if msg.Dest == DestPM && !HasPM(msg.Event) {
		return Message{}, "pm_not_supported"
	}

	hasMedia := false
	if msg.FileID != "" || msg.MediaType != 0 {
		if msg.FileID == "" || msg.MediaType == 0 || !allowedMediaType(msg.MediaType) {
			return Message{}, "invalid_media"
		}
		if NoMedia(msg.Event, msg.Dest) {
			return Message{}, "media_not_allowed"
		}
		hasMedia = true
	}

	var buttons []models.Button
	if len(msg.Buttons) > 0 {
		buttons = make([]models.Button, len(msg.Buttons))
		for i, btn := range msg.Buttons {
			if strings.TrimSpace(btn.Name) == "" {
				return Message{}, "invalid_button"
			}
			if !content.ValidButtonURL(btn.Url) {
				return Message{}, "invalid_button_url"
			}
			buttons[i] = btn
		}
	}

	allButtons := buttons
	if token := rulesTokenPattern.FindStringSubmatch(msg.Text); token != nil {
		rulesBtn := models.Button{Name: "Rules", Url: "https://t.me/bot", SameLine: token[2] == "same"}
		if token[2] == "up" {
			allButtons = append([]models.Button{rulesBtn}, buttons...)
		} else {
			allButtons = append(append([]models.Button(nil), buttons...), rulesBtn)
		}
	}
	widest := 0
	for _, row := range keyboard.BuildKeyboard(allButtons) {
		if len(row) > widest {
			widest = len(row)
		}
	}
	if len(allButtons)+ReservedButtonsFor(msg.Event, msg.Dest) > MaxButtons || widest > MaxRowButtons {
		return Message{}, "buttons_limit"
	}

	rawLimit := MaxTextLength
	if hasMedia {
		rawLimit = MaxCaptionLength
	}
	effectiveLimit := rawLimit - ReservedTextFor(msg.Event, msg.Dest)
	if utf8.RuneCountInString(msg.Text) > rawLimit || formatting.VisibleLength(msg.Text) > effectiveLimit {
		return Message{}, "text_too_long"
	}

	out := Message{
		Event:     msg.Event,
		Dest:      msg.Dest,
		Text:      msg.Text,
		FileID:    msg.FileID,
		MediaType: msg.MediaType,
		Buttons:   buttons,
		Silent:    msg.Silent,
	}
	if NoSilent(out.Event, out.Dest) {
		out.Silent = false
	}
	return out, ""
}

// ReplaceAllValidated validates all candidate rows, deduplicates valid rows by (event, dest)
// (last valid wins), atomically replaces chatID's chat_event_messages and
// chat_event_message_options in a single database transaction, and drops event_messages:<chat>
// after commit. Rows that fail ValidateMessage are skipped and reported in WriteReport.Skipped
// without aborting valid rows.
func ReplaceAllValidated(ctx context.Context, chatID int64, rows []Message, opts Options, by int64) (WriteReport, error) {
	if db.DB == nil {
		return WriteReport{}, fukuerrors.Wrap(errNoDatabase, "replace event messages")
	}

	var report WriteReport
	dedup := make(map[string]Message, len(rows))
	order := make([]string, 0, len(rows))
	for _, row := range rows {
		valid, reason := ValidateMessage(row)
		if reason != "" {
			report.Skipped = append(report.Skipped, SkippedRow{
				Event:  row.Event,
				Dest:   row.Dest,
				Reason: reason,
			})
			continue
		}
		key := rowKey(valid.Event, valid.Dest)
		if _, seen := dedup[key]; !seen {
			order = append(order, key)
		}
		dedup[key] = valid
	}

	toWrite := make([]Message, 0, len(order))
	for _, key := range order {
		row := dedup[key]
		if !row.HasContent() && row.Silent == DefaultSilentFor(row.Event, row.Dest) {
			continue
		}
		toWrite = append(toWrite, row)
	}

	now := time.Now()
	err := db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("chat_id = ?", chatID).Delete(&models.ChatEventMessage{}).Error; err != nil {
			return err
		}
		for _, row := range toWrite {
			rec := models.ChatEventMessage{
				ChatID:    chatID,
				Event:     row.Event,
				Dest:      row.Dest,
				Text:      row.Text,
				FileID:    row.FileID,
				MediaType: row.MediaType,
				Buttons:   models.ButtonArray(row.Buttons),
				Silent:    row.Silent,
				UpdatedBy: by,
				UpdatedAt: now,
			}
			if err := tx.Create(&rec).Error; err != nil {
				return err
			}
		}
		if !opts.PMDetails && !opts.PMLog && !opts.PlainTone {
			if err := tx.Where("chat_id = ?", chatID).Delete(&models.ChatEventMessageOptions{}).Error; err != nil {
				return err
			}
		} else {
			optRow := models.ChatEventMessageOptions{
				ChatID:    chatID,
				PMDetails: opts.PMDetails,
				PMLog:     opts.PMLog,
				PlainTone: opts.PlainTone,
				UpdatedBy: by,
				UpdatedAt: now,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "chat_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"pm_details", "pm_log", "plain_tone", "updated_by", "updated_at"}),
			}).Create(&optRow).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return WriteReport{}, fukuerrors.Wrapf(err, "replace event messages of chat %d", chatID)
	}

	cache.DeleteCache(CacheKey(chatID))
	report.Written = len(toWrite)
	return report, nil
}
