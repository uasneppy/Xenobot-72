// Package modtuning stores each group's manual moderation defaults, appeal link, and reason
// shortcuts.
//
// Both cache keys (chat_manual_defaults:<chat> and chat_reason_shortcuts:<chat>) are listed in
// fuku/db/cache/local.go skipLocal so writes take effect on the very next command on every
// replica. Reads never create rows; writes use column-targeted upserts and invalidate their key.
package modtuning

import (
	"context"
	"errors"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	fukuerrors "github.com/uasneppy/fuku_robot/fuku/utils/errors"
)

const (
	defaultsCacheModule  = "chat_manual_defaults"
	shortcutsCacheModule = "chat_reason_shortcuts"
	cacheTTL             = cache.DefaultCacheTTL
)

var (
	errNoDatabase = errors.New("database not initialized")
	// ErrInvalidValue is returned before any DB write when a duration, URL, word, or reason is
	// outside the table's allowed bounds.
	ErrInvalidValue = errors.New("modtuning: invalid value")
)

// Defaults is a chat's manual command defaults and appeal button URL. Zero value means no default
// durations (permanent), reason not required, and no appeal link.
type Defaults struct {
	DefaultBanSecs  int    `msgpack:"b" json:"default_ban_secs"`
	DefaultMuteSecs int    `msgpack:"m" json:"default_mute_secs"`
	ReasonRequired  bool   `msgpack:"r" json:"reason_required"`
	AppealURL       string `msgpack:"a" json:"appeal_url"`
}

// ShortcutEntry is one stored reason shortcut row.
type ShortcutEntry struct {
	ID     int64  `msgpack:"i" json:"id"`
	Word   string `msgpack:"w" json:"word"`
	Reason string `msgpack:"r" json:"reason"`
}

// DefaultsCacheKey is the cache key of a chat's manual defaults. Its prefix is in skipLocal.
func DefaultsCacheKey(chatID int64) string {
	return cache.CacheKey(defaultsCacheModule, chatID)
}

// ShortcutsCacheKey is the cache key of a chat's reason shortcuts map. Its prefix is in skipLocal.
func ShortcutsCacheKey(chatID int64) string {
	return cache.CacheKey(shortcutsCacheModule, chatID)
}

// ValidDefaultDuration reports whether secs is allowed for default ban/mute duration: 0 (unset) or
// models.PunishDurationValid (60..31622400).
func ValidDefaultDuration(secs int) bool {
	return secs == 0 || models.PunishDurationValid(secs)
}

// GetDefaults returns the chat's manual defaults through the cache (skipLocal), or the zero
// Defaults for a chat with no row. It creates nothing and returns DB/cache load errors.
func GetDefaults(ctx context.Context, chatID int64) (Defaults, error) {
	defs, err := cache.GetFromCacheOrLoad(ctx, DefaultsCacheKey(chatID), cacheTTL, func(ctx context.Context) (*Defaults, error) {
		d, err := GetDefaultsFresh(ctx, chatID)
		if err != nil {
			return nil, err
		}
		return &d, nil
	})
	if err != nil {
		return Defaults{}, err
	}
	if defs == nil {
		return Defaults{}, nil
	}
	return *defs, nil
}

// GetDefaultsFresh reads the chat's manual defaults directly from PostgreSQL without the cache. A
// chat with no row returns Defaults{}, nil and creates nothing.
func GetDefaultsFresh(ctx context.Context, chatID int64) (Defaults, error) {
	if db.DB == nil {
		return Defaults{}, fukuerrors.Wrap(errNoDatabase, "read manual defaults")
	}
	var row models.ChatManualDefaults
	err := db.DB.WithContext(ctx).Where("chat_id = ?", chatID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Defaults{}, nil
	}
	if err != nil {
		return Defaults{}, fukuerrors.Wrapf(err, "read manual defaults of chat %d", chatID)
	}
	return Defaults{
		DefaultBanSecs:  row.DefaultBanSecs,
		DefaultMuteSecs: row.DefaultMuteSecs,
		ReasonRequired:  row.ReasonRequired,
		AppealURL:       row.AppealURL,
	}, nil
}

// SetDefaultBanSecs writes default_ban_secs with a column-targeted upsert and drops the cache key.
func SetDefaultBanSecs(ctx context.Context, chatID int64, secs int, by int64) error {
	if !ValidDefaultDuration(secs) {
		return ErrInvalidValue
	}
	row := defaultRow(chatID, by)
	row.DefaultBanSecs = secs
	return upsertDefaults(ctx, row, "default_ban_secs")
}

// SetDefaultMuteSecs writes default_mute_secs with a column-targeted upsert and drops the cache key.
func SetDefaultMuteSecs(ctx context.Context, chatID int64, secs int, by int64) error {
	if !ValidDefaultDuration(secs) {
		return ErrInvalidValue
	}
	row := defaultRow(chatID, by)
	row.DefaultMuteSecs = secs
	return upsertDefaults(ctx, row, "default_mute_secs")
}

// SetReasonRequired writes reason_required with a column-targeted upsert and drops the cache key.
func SetReasonRequired(ctx context.Context, chatID int64, on bool, by int64) error {
	row := defaultRow(chatID, by)
	row.ReasonRequired = on
	return upsertDefaults(ctx, row, "reason_required")
}

// SetAppealURL writes appeal_url (empty clears it) with a column-targeted upsert and drops the
// cache key.
func SetAppealURL(ctx context.Context, chatID int64, rawURL string, by int64) error {
	if utf8.RuneCountInString(rawURL) > models.ManualAppealURLMaxRunes {
		return ErrInvalidValue
	}
	row := defaultRow(chatID, by)
	row.AppealURL = rawURL
	return upsertDefaults(ctx, row, "appeal_url")
}

// ReplaceDefaultsTx validates defs and replaces all four manual-default columns of chatID inside
// caller-supplied transaction tx. The caller is responsible for dropping DefaultsCacheKey(chatID)
// after commit.
func ReplaceDefaultsTx(tx *gorm.DB, chatID int64, defs Defaults, by int64) error {
	if !ValidDefaultDuration(defs.DefaultBanSecs) ||
		!ValidDefaultDuration(defs.DefaultMuteSecs) ||
		utf8.RuneCountInString(defs.AppealURL) > models.ManualAppealURLMaxRunes {
		return ErrInvalidValue
	}
	if tx == nil {
		return fukuerrors.Wrap(errNoDatabase, "write manual defaults")
	}
	if defs == (Defaults{}) {
		if err := tx.Where("chat_id = ?", chatID).Delete(&models.ChatManualDefaults{}).Error; err != nil {
			return fukuerrors.Wrapf(err, "reset manual defaults of chat %d", chatID)
		}
		return nil
	}
	row := defaultRow(chatID, by)
	row.DefaultBanSecs = defs.DefaultBanSecs
	row.DefaultMuteSecs = defs.DefaultMuteSecs
	row.ReasonRequired = defs.ReasonRequired
	row.AppealURL = defs.AppealURL
	return upsertDefaultsTx(tx, row, "default_ban_secs", "default_mute_secs", "reason_required", "appeal_url")
}

// ReplaceDefaults atomically replaces all four manual-default columns of chatID and drops
// chat_manual_defaults:<chat>. Invalid durations or over-long appeal URLs return ErrInvalidValue
// before touching the database. When defs is the zero Defaults, any existing row is deleted.
func ReplaceDefaults(ctx context.Context, chatID int64, defs Defaults, by int64) error {
	var tx *gorm.DB
	if db.DB != nil {
		tx = db.DB.WithContext(ctx)
	}
	if err := ReplaceDefaultsTx(tx, chatID, defs, by); err != nil {
		return err
	}
	cache.DeleteCache(DefaultsCacheKey(chatID))
	return nil
}

func defaultRow(chatID, by int64) *models.ChatManualDefaults {
	return &models.ChatManualDefaults{
		ChatId:    chatID,
		UpdatedBy: by,
		UpdatedAt: time.Now(),
	}
}

func upsertDefaultsTx(tx *gorm.DB, row *models.ChatManualDefaults, columns ...string) error {
	if tx == nil {
		return fukuerrors.Wrap(errNoDatabase, "write manual defaults")
	}
	err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.AssignmentColumns(append(columns, "updated_by", "updated_at")),
	}).Create(row).Error
	if err != nil {
		return fukuerrors.Wrapf(err, "write manual defaults of chat %d", row.ChatId)
	}
	return nil
}

func upsertDefaults(ctx context.Context, row *models.ChatManualDefaults, columns ...string) error {
	if db.DB == nil {
		return fukuerrors.Wrap(errNoDatabase, "write manual defaults")
	}
	if err := upsertDefaultsTx(db.DB.WithContext(ctx), row, columns...); err != nil {
		return err
	}
	cache.DeleteCache(DefaultsCacheKey(row.ChatId))
	return nil
}

// GetShortcuts returns the chat's reason shortcuts map (word -> reason) through the cache
// (skipLocal). It creates nothing and returns DB/cache load errors.
func GetShortcuts(ctx context.Context, chatID int64) (map[string]string, error) {
	shortcuts, err := cache.GetFromCacheOrLoad(ctx, ShortcutsCacheKey(chatID), cacheTTL, func(ctx context.Context) (map[string]string, error) {
		return GetShortcutsFresh(ctx, chatID)
	})
	if err != nil {
		return nil, err
	}
	if shortcuts == nil {
		return map[string]string{}, nil
	}
	return shortcuts, nil
}

// GetShortcutsFresh reads the chat's reason shortcuts map directly from PostgreSQL without cache.
func GetShortcutsFresh(ctx context.Context, chatID int64) (map[string]string, error) {
	entries, err := ListShortcutsFresh(ctx, chatID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(entries))
	for _, entry := range entries {
		out[entry.Word] = entry.Reason
	}
	return out, nil
}

// ListShortcutsFresh lists the chat's reason shortcuts ordered by id, fresh and uncached.
func ListShortcutsFresh(ctx context.Context, chatID int64) ([]ShortcutEntry, error) {
	if db.DB == nil {
		return nil, fukuerrors.Wrap(errNoDatabase, "list reason shortcuts")
	}
	var rows []models.ChatReasonShortcut
	err := db.DB.WithContext(ctx).
		Select("id", "word", "reason").
		Where("chat_id = ?", chatID).
		Order("id").
		Find(&rows).Error
	if err != nil {
		return nil, fukuerrors.Wrapf(err, "list reason shortcuts of chat %d", chatID)
	}
	out := make([]ShortcutEntry, len(rows))
	for i, r := range rows {
		out[i] = ShortcutEntry{ID: r.ID, Word: r.Word, Reason: r.Reason}
	}
	return out, nil
}

// GetShortcutFresh reads one reason shortcut by ID scoped to chatID. A missing ID or an ID owned
// by another chat returns nil, nil.
func GetShortcutFresh(ctx context.Context, chatID, id int64) (*ShortcutEntry, error) {
	if db.DB == nil {
		return nil, fukuerrors.Wrap(errNoDatabase, "read reason shortcut")
	}
	var row models.ChatReasonShortcut
	err := db.DB.WithContext(ctx).Where("id = ? AND chat_id = ?", id, chatID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fukuerrors.Wrapf(err, "read reason shortcut %d of chat %d", id, chatID)
	}
	return &ShortcutEntry{ID: row.ID, Word: row.Word, Reason: row.Reason}, nil
}

// CountShortcutsFresh counts the chat's stored reason shortcuts, fresh and uncached.
func CountShortcutsFresh(ctx context.Context, chatID int64) (int, error) {
	if db.DB == nil {
		return 0, fukuerrors.Wrap(errNoDatabase, "count reason shortcuts")
	}
	var n int64
	err := db.DB.WithContext(ctx).Model(&models.ChatReasonShortcut{}).Where("chat_id = ?", chatID).Count(&n).Error
	if err != nil {
		return 0, fukuerrors.Wrapf(err, "count reason shortcuts of chat %d", chatID)
	}
	return int(n), nil
}

// ExistsShortcutFresh reports whether chatID already stores a shortcut for word, fresh and uncached.
func ExistsShortcutFresh(ctx context.Context, chatID int64, word string) (bool, error) {
	if db.DB == nil {
		return false, fukuerrors.Wrap(errNoDatabase, "check reason shortcut")
	}
	var n int64
	err := db.DB.WithContext(ctx).Model(&models.ChatReasonShortcut{}).
		Where("chat_id = ? AND word = ?", chatID, word).
		Count(&n).Error
	if err != nil {
		return false, fukuerrors.Wrapf(err, "check reason shortcut of chat %d", chatID)
	}
	return n > 0, nil
}

// AddShortcut inserts one reason shortcut with ON CONFLICT (chat_id, word) DO NOTHING so a
// duplicate never overwrites an existing entry. The cache key is dropped only when a row is added.
func AddShortcut(ctx context.Context, chatID int64, word, reason string, by int64) (bool, error) {
	if word == "" || utf8.RuneCountInString(word) > models.ReasonShortcutWordMaxRunes ||
		reason == "" || utf8.RuneCountInString(reason) > models.ReasonShortcutTextMaxRunes {
		return false, ErrInvalidValue
	}
	if db.DB == nil {
		return false, fukuerrors.Wrap(errNoDatabase, "add reason shortcut")
	}
	row := &models.ChatReasonShortcut{
		ChatId:    chatID,
		Word:      word,
		Reason:    reason,
		CreatedBy: by,
		CreatedAt: time.Now(),
	}
	res := db.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}, {Name: "word"}},
		DoNothing: true,
	}).Create(row)
	if res.Error != nil {
		return false, fukuerrors.Wrapf(res.Error, "add reason shortcut of chat %d", chatID)
	}
	added := res.RowsAffected == 1
	if added {
		cache.DeleteCache(ShortcutsCacheKey(chatID))
	}
	return added, nil
}

func validateShortcuts(entries []ShortcutEntry) error {
	if len(entries) > models.ReasonShortcutsCapMax {
		return ErrInvalidValue
	}
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if entry.Word == "" || utf8.RuneCountInString(entry.Word) > models.ReasonShortcutWordMaxRunes ||
			entry.Reason == "" || utf8.RuneCountInString(entry.Reason) > models.ReasonShortcutTextMaxRunes {
			return ErrInvalidValue
		}
		if _, dup := seen[entry.Word]; dup {
			return ErrInvalidValue
		}
		seen[entry.Word] = struct{}{}
	}
	return nil
}

// ReplaceShortcutsTx validates entries and replaces all chat_reason_shortcuts rows of chatID
// inside caller-supplied transaction tx. The caller is responsible for dropping
// ShortcutsCacheKey(chatID) after commit.
func ReplaceShortcutsTx(tx *gorm.DB, chatID int64, entries []ShortcutEntry, by int64) error {
	if err := validateShortcuts(entries); err != nil {
		return err
	}
	if tx == nil {
		return fukuerrors.Wrap(errNoDatabase, "replace reason shortcuts")
	}
	now := time.Now()
	if err := tx.Where("chat_id = ?", chatID).Delete(&models.ChatReasonShortcut{}).Error; err != nil {
		return fukuerrors.Wrapf(err, "replace reason shortcuts of chat %d", chatID)
	}
	for _, entry := range entries {
		rec := models.ChatReasonShortcut{
			ChatId:    chatID,
			Word:      entry.Word,
			Reason:    entry.Reason,
			CreatedBy: by,
			CreatedAt: now,
		}
		if err := tx.Create(&rec).Error; err != nil {
			return fukuerrors.Wrapf(err, "replace reason shortcuts of chat %d", chatID)
		}
	}
	return nil
}

// ReplaceShortcuts atomically replaces all chat_reason_shortcuts rows of chatID in one
// transaction and drops chat_reason_shortcuts:<chat> after commit. Count, word/reason bounds,
// and duplicate words are validated before opening the transaction so invalid input leaves the
// database untouched.
func ReplaceShortcuts(ctx context.Context, chatID int64, entries []ShortcutEntry, by int64) error {
	if err := validateShortcuts(entries); err != nil {
		return err
	}
	if db.DB == nil {
		return fukuerrors.Wrap(errNoDatabase, "replace reason shortcuts")
	}
	err := db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return ReplaceShortcutsTx(tx, chatID, entries, by)
	})
	if err != nil {
		return err
	}
	cache.DeleteCache(ShortcutsCacheKey(chatID))
	return nil
}

// RemoveShortcutByID deletes one reason shortcut by ID only when it belongs to chatID. It returns
// the removed word and true when a row was deleted (dropping the cache key), or "", false, nil
// when absent or owned by another chat.
func RemoveShortcutByID(ctx context.Context, chatID, id int64) (word string, removed bool, err error) {
	if db.DB == nil {
		return "", false, fukuerrors.Wrap(errNoDatabase, "remove reason shortcut")
	}
	var rows []models.ChatReasonShortcut
	res := db.DB.WithContext(ctx).
		Clauses(clause.Returning{}).
		Where("id = ? AND chat_id = ?", id, chatID).
		Delete(&rows)
	if res.Error != nil {
		return "", false, fukuerrors.Wrapf(res.Error, "remove reason shortcut %d of chat %d", id, chatID)
	}
	if len(rows) != 1 {
		return "", false, nil
	}
	cache.DeleteCache(ShortcutsCacheKey(chatID))
	return rows[0].Word, true, nil
}
