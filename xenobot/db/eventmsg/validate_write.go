package eventmsg

import (
	"context"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/content"
	xenoerrors "github.com/uasneppy/Xenobot-72/xenobot/utils/errors"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/formatting"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/keyboard"
)

// Validation limits shared by Copy Settings (Phase 29) and backup import (Phase 30), matching
// Telegram limits and the Messages settings editor (xenobot/modules/settings_edit_check.go and
// xenobot/modules/settings_messages.go).
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

	hasMedia, reason := validateMessageMedia(msg)
	if reason != "" {
		return Message{}, reason
	}
	buttons, reason := validateMessageButtons(msg)
	if reason != "" {
		return Message{}, reason
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

// ReplaceAllValidatedTx validates all candidate rows, deduplicates valid rows by (event, dest)
// (last valid wins), and replaces chatID's chat_event_messages and chat_event_message_options
// inside caller-supplied transaction tx. Rows that fail ValidateMessage are skipped and reported
// in WriteReport.Skipped without aborting valid rows. The caller is responsible for invalidating
// CacheKey(chatID) after tx commits.
func ReplaceAllValidatedTx(tx *gorm.DB, chatID int64, rows []Message, opts Options, by int64) (WriteReport, error) {
	if tx == nil {
		return WriteReport{}, xenoerrors.Wrap(errNoDatabase, "replace event messages")
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
	if err := tx.Where("chat_id = ?", chatID).Delete(&models.ChatEventMessage{}).Error; err != nil {
		return WriteReport{}, xenoerrors.Wrapf(err, "replace event messages of chat %d", chatID)
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
			return WriteReport{}, xenoerrors.Wrapf(err, "replace event messages of chat %d", chatID)
		}
	}
	if !opts.PMDetails && !opts.PMLog && !opts.PlainTone {
		if err := tx.Where("chat_id = ?", chatID).Delete(&models.ChatEventMessageOptions{}).Error; err != nil {
			return WriteReport{}, xenoerrors.Wrapf(err, "replace event messages of chat %d", chatID)
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
			return WriteReport{}, xenoerrors.Wrapf(err, "replace event messages of chat %d", chatID)
		}
	}

	report.Written = len(toWrite)
	return report, nil
}

// ReplaceAllValidated validates all candidate rows, deduplicates valid rows by (event, dest)
// (last valid wins), atomically replaces chatID's chat_event_messages and
// chat_event_message_options in a single database transaction, and drops event_messages:<chat>
// after commit. Rows that fail ValidateMessage are skipped and reported in WriteReport.Skipped
// without aborting valid rows.
func ReplaceAllValidated(ctx context.Context, chatID int64, rows []Message, opts Options, by int64) (WriteReport, error) {
	if db.DB == nil {
		return WriteReport{}, xenoerrors.Wrap(errNoDatabase, "replace event messages")
	}

	var report WriteReport
	var innerErr error
	err := db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		report, innerErr = ReplaceAllValidatedTx(tx, chatID, rows, opts, by)
		return innerErr
	})
	if err != nil {
		if innerErr != nil {
			return WriteReport{}, err
		}
		return WriteReport{}, xenoerrors.Wrapf(err, "replace event messages of chat %d", chatID)
	}

	cache.DeleteCache(CacheKey(chatID))
	return report, nil
}

// validateMessageMedia checks the media part of msg and reports whether it carries media.
// Events with mandatory bot-appended text (ReservedTextFor > 0, such as lockdown notices)
// also refuse captionless media (db.STICKER) so custom text and technical notes are never dropped.
func validateMessageMedia(msg Message) (bool, string) {
	if msg.FileID == "" && msg.MediaType == 0 {
		return false, ""
	}
	if msg.FileID == "" || msg.MediaType == 0 || !allowedMediaType(msg.MediaType) {
		return false, "invalid_media"
	}
	if NoMedia(msg.Event, msg.Dest) {
		return false, "media_not_allowed"
	}
	if msg.MediaType == db.STICKER && ReservedTextFor(msg.Event, msg.Dest) > 0 {
		return false, "invalid_media"
	}
	return true, ""
}

// validateMessageButtons checks the buttons of msg, with the {rules} button the text adds and the
// bot's reserved buttons counted against Telegram's keyboard limits.
func validateMessageButtons(msg Message) ([]models.Button, string) {
	var buttons []models.Button
	if len(msg.Buttons) > 0 {
		buttons = make([]models.Button, len(msg.Buttons))
		for i, btn := range msg.Buttons {
			if strings.TrimSpace(btn.Name) == "" {
				return nil, "invalid_button"
			}
			if !content.ValidButtonURL(btn.Url) {
				return nil, "invalid_button_url"
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
		return nil, "buttons_limit"
	}
	return buttons, ""
}
