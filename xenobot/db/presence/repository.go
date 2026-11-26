// Package presence stores each group's admin-configurable Interactive Presence settings.
package presence

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
	// DefaultConsensusThreshold is the default distinct-member count before Emoji Consensus fires.
	DefaultConsensusThreshold = 3
	// MinConsensusThreshold and MaxConsensusThreshold bound the configurable consensus threshold.
	MinConsensusThreshold = 2
	MaxConsensusThreshold = 20

	// AIReactModeOff, AIReactModeLow, and AIReactModeMedium are the stored values for ai_react_mode.
	AIReactModeOff    = "off"
	AIReactModeLow    = "low"
	AIReactModeMedium = "medium"

	cacheTTL = cache.DefaultCacheTTL
)

var (
	errNoDatabase = errors.New("database not initialized")
	// ErrInvalidValue is returned, before any write, for a value outside the stored bounds.
	ErrInvalidValue = errors.New("presence: invalid value")
)

// Settings is a chat's Interactive Presence settings.
type Settings struct {
	ConsensusEnabled   bool   `msgpack:"ce"`
	ConsensusThreshold int    `msgpack:"ct"`
	RetractEnabled     bool   `msgpack:"re"`
	AIReactMode        string `msgpack:"ar"`
}

// Defaults returns the settings of a chat with no row: Emoji Consensus off with threshold 3,
// thumbs-down retraction on, and Contextual AI Emoji Reactions off.
func Defaults() Settings {
	return Settings{
		ConsensusEnabled:   false,
		ConsensusThreshold: DefaultConsensusThreshold,
		RetractEnabled:     true,
		AIReactMode:        AIReactModeOff,
	}
}

// ValidConsensusThreshold reports whether n is within [MinConsensusThreshold, MaxConsensusThreshold].
func ValidConsensusThreshold(n int) bool {
	return n >= MinConsensusThreshold && n <= MaxConsensusThreshold
}

// ValidAIReactMode reports whether mode is one of the allowed ai_react_mode values.
func ValidAIReactMode(mode string) bool {
	return mode == AIReactModeOff || mode == AIReactModeLow || mode == AIReactModeMedium
}

func normalizeAIReactMode(mode string) string {
	if ValidAIReactMode(mode) {
		return mode
	}
	return AIReactModeOff
}

// CacheKey returns the skipLocal cache key of a chat's presence settings.
func CacheKey(chatID int64) string {
	return cache.CacheKey("chat_presence", chatID)
}

// Get returns the chat's settings through the cache, or Defaults for a chat with no row. It
// creates no row and returns its error.
func Get(ctx context.Context, chatID int64) (Settings, error) {
	s, err := cache.GetFromCacheOrLoad(ctx, CacheKey(chatID), cacheTTL, func(ctx context.Context) (Settings, error) {
		return GetFresh(ctx, chatID)
	})
	if err != nil {
		return Settings{}, err
	}
	s.AIReactMode = normalizeAIReactMode(s.AIReactMode)
	return s, nil
}

// GetFresh is Get without the cache.
func GetFresh(ctx context.Context, chatID int64) (Settings, error) {
	if db.DB == nil {
		return Settings{}, xenoerrors.Wrap(errNoDatabase, "read chat presence setting")
	}
	var row models.ChatPresence
	err := db.DB.WithContext(ctx).Where("chat_id = ?", chatID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Defaults(), nil
	}
	if err != nil {
		return Settings{}, xenoerrors.Wrapf(err, "read the presence setting of chat %d", chatID)
	}
	return Settings{
		ConsensusEnabled:   row.ConsensusEnabled,
		ConsensusThreshold: int(row.ConsensusThreshold),
		RetractEnabled:     row.RetractEnabled,
		AIReactMode:        normalizeAIReactMode(row.AIReactMode),
	}, nil
}

// SetConsensusEnabled writes consensus_enabled with one column-targeted upsert and drops the cache key.
func SetConsensusEnabled(ctx context.Context, chatID int64, enabled bool, by int64) error {
	row := defaultRow(chatID, by)
	row.ConsensusEnabled = enabled
	return upsert(ctx, row, "consensus_enabled")
}

// SetConsensusThreshold writes consensus_threshold, refusing values outside [2, 20] before any write.
func SetConsensusThreshold(ctx context.Context, chatID int64, threshold int, by int64) error {
	if !ValidConsensusThreshold(threshold) {
		return ErrInvalidValue
	}
	row := defaultRow(chatID, by)
	row.ConsensusThreshold = int16(threshold) // #nosec G115 -- validated as 2 to 20
	return upsert(ctx, row, "consensus_threshold")
}

// SetRetractEnabled writes retract_enabled with one column-targeted upsert and drops the cache key.
func SetRetractEnabled(ctx context.Context, chatID int64, enabled bool, by int64) error {
	row := defaultRow(chatID, by)
	row.RetractEnabled = enabled
	return upsert(ctx, row, "retract_enabled")
}

// SetAIReactMode writes ai_react_mode, refusing values outside ('off', 'low', 'medium') before any write.
func SetAIReactMode(ctx context.Context, chatID int64, mode string, by int64) error {
	if !ValidAIReactMode(mode) {
		return ErrInvalidValue
	}
	row := defaultRow(chatID, by)
	row.AIReactMode = mode
	return upsert(ctx, row, "ai_react_mode")
}

func defaultRow(chatID, by int64) *models.ChatPresence {
	d := Defaults()
	return &models.ChatPresence{
		ChatID:             chatID,
		ConsensusEnabled:   d.ConsensusEnabled,
		ConsensusThreshold: int16(d.ConsensusThreshold), // #nosec G115 -- DefaultConsensusThreshold is 3
		RetractEnabled:     d.RetractEnabled,
		AIReactMode:        d.AIReactMode,
		UpdatedBy:          by,
		UpdatedAt:          time.Now(),
	}
}

// upsert inserts the row or updates only column, updated_by and updated_at, so concurrent admin
// presses on different controls never clobber each other, and drops the cache key immediately.
func upsert(ctx context.Context, row *models.ChatPresence, column string) error {
	if db.DB == nil {
		return xenoerrors.Wrap(errNoDatabase, "write chat presence setting")
	}
	err := db.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.AssignmentColumns([]string{column, "updated_by", "updated_at"}),
	}).Create(row).Error
	if err != nil {
		return xenoerrors.Wrapf(err, "write the presence %s of chat %d", column, row.ChatID)
	}
	cache.DeleteCache(CacheKey(row.ChatID))
	return nil
}
