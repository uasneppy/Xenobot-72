// Package chatconv stores each group's conversation setting.
package chatconv

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	xenoerrors "github.com/uasneppy/Xenobot-72/xenobot/utils/errors"
)

const (
	// ProviderAnthropic and ProviderGoogle are the stored provider values.
	ProviderAnthropic = "anthropic"
	ProviderGoogle    = "google"

	// AudienceEveryone and AudienceAdminsApproved are the stored audience values.
	AudienceEveryone       = 0
	AudienceAdminsApproved = 1

	cacheTTL = cache.DefaultCacheTTL
)

var (
	errNoDatabase = errors.New("database not initialized")
	// ErrInvalidValue is returned, before any write, for a provider or audience outside the
	// stored values.
	ErrInvalidValue = errors.New("chatconv: invalid value")
)

// Settings is a chat's conversation setting.
type Settings struct {
	Enabled  bool   `msgpack:"e"`
	Provider string `msgpack:"p"`
	Audience int    `msgpack:"a"`
	// LogEnabled is the owner's Message log switch. A value an older build cached has no field
	// and decodes to off.
	LogEnabled bool `msgpack:"l"`
}

// Defaults is the setting of a chat with no row: off, Anthropic, everyone.
func Defaults() Settings {
	return Settings{Provider: ProviderAnthropic, Audience: AudienceEveryone}
}

// ValidProvider reports whether p is a stored provider value.
func ValidProvider(p string) bool {
	return p == ProviderAnthropic || p == ProviderGoogle
}

// ValidAudience reports whether a is a stored audience value.
func ValidAudience(a int) bool {
	return a == AudienceEveryone || a == AudienceAdminsApproved
}

// CacheKey is the cache key of a chat's setting. Its prefix is listed in skipLocal.
func CacheKey(chatID int64) string {
	return cache.CacheKey("chat_conv", chatID)
}

// Get returns the chat's setting through the cache, the defaults for a chat with no row. It
// creates nothing and returns its error.
func Get(ctx context.Context, chatID int64) (Settings, error) {
	return cache.GetFromCacheOrLoad(ctx, CacheKey(chatID), cacheTTL, func(ctx context.Context) (Settings, error) {
		return GetFresh(ctx, chatID)
	})
}

// GetFresh is Get without the cache.
func GetFresh(ctx context.Context, chatID int64) (Settings, error) {
	if db.DB == nil {
		return Settings{}, xenoerrors.Wrap(errNoDatabase, "read chat conversation setting")
	}
	var row models.ChatConversation
	err := db.DB.WithContext(ctx).Where("chat_id = ?", chatID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Defaults(), nil
	}
	if err != nil {
		return Settings{}, xenoerrors.Wrapf(err, "read the conversation setting of chat %d", chatID)
	}
	return Settings{Enabled: row.Enabled, Provider: row.Provider, Audience: int(row.Audience), LogEnabled: row.LogEnabled}, nil
}

// SetEnabled writes the on/off switch with one column-targeted upsert and drops the key.
func SetEnabled(ctx context.Context, chatID int64, enabled bool, by int64) error {
	row := defaultRow(chatID, by)
	row.Enabled = enabled
	return upsert(ctx, row, "enabled")
}

// SetProvider writes the provider, refusing a value outside the stored ones before any write.
func SetProvider(ctx context.Context, chatID int64, provider string, by int64) error {
	if !ValidProvider(provider) {
		return ErrInvalidValue
	}
	row := defaultRow(chatID, by)
	row.Provider = provider
	return upsert(ctx, row, "provider")
}

// SetAudience writes the audience, refusing a value outside the stored ones before any write.
func SetAudience(ctx context.Context, chatID int64, audience int, by int64) error {
	if !ValidAudience(audience) {
		return ErrInvalidValue
	}
	row := defaultRow(chatID, by)
	row.Audience = int16(audience) // #nosec G115 -- validated as 0 or 1
	return upsert(ctx, row, "audience")
}

// SetLogEnabled writes the Message log switch with one column-targeted upsert and drops the key.
func SetLogEnabled(ctx context.Context, chatID int64, on bool, by int64) error {
	row := defaultRow(chatID, by)
	row.LogEnabled = on
	return upsert(ctx, row, "log_enabled")
}

func defaultRow(chatID, by int64) *models.ChatConversation {
	d := Defaults()
	return &models.ChatConversation{
		ChatID: chatID, Provider: d.Provider, Audience: int16(d.Audience), UpdatedBy: by, UpdatedAt: time.Now(),
	}
}

// upsert inserts the row or updates only column, updated_by and updated_at, so two owners'
// presses on two controls never clobber each other, and drops the cache key.
func upsert(ctx context.Context, row *models.ChatConversation, column string) error {
	if db.DB == nil {
		return xenoerrors.Wrap(errNoDatabase, "write chat conversation setting")
	}
	err := db.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.AssignmentColumns([]string{column, "updated_by", "updated_at"}),
	}).Create(row).Error
	if err != nil {
		return xenoerrors.Wrapf(err, "write the conversation %s of chat %d", column, row.ChatID)
	}
	cache.DeleteCache(CacheKey(row.ChatID))
	return nil
}
