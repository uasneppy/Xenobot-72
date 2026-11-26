// Package presence stores each group's admin-configurable Interactive Presence settings.
package presence

import (
	"context"
	"errors"

	"github.com/uasneppy/Xenobot-72/xenobot/db/cache"
)

const (
	// DefaultConsensusThreshold is the default distinct-member count before Emoji Consensus fires.
	DefaultConsensusThreshold = 3
	// MinConsensusThreshold and MaxConsensusThreshold bound the configurable consensus threshold.
	MinConsensusThreshold = 2
	MaxConsensusThreshold = 20
)

var (
	// ErrInvalidValue is returned, before any write, for a value outside the stored bounds.
	ErrInvalidValue = errors.New("presence: invalid value")
)

// Settings is a chat's Interactive Presence settings.
type Settings struct {
	ConsensusEnabled   bool `msgpack:"ce"`
	ConsensusThreshold int  `msgpack:"ct"`
	RetractEnabled     bool `msgpack:"re"`
}

// Defaults returns the settings of a chat with no row.
func Defaults() Settings {
	return Settings{}
}

// ValidConsensusThreshold reports whether n is within [MinConsensusThreshold, MaxConsensusThreshold].
func ValidConsensusThreshold(n int) bool {
	return false
}

// CacheKey returns the skipLocal cache key of a chat's presence settings.
func CacheKey(chatID int64) string {
	return cache.CacheKey("chat_presence", chatID)
}

// Get returns the chat's settings through the cache.
func Get(ctx context.Context, chatID int64) (Settings, error) {
	return Settings{}, errors.New("not implemented")
}

// GetFresh returns the chat's settings directly from the database.
func GetFresh(ctx context.Context, chatID int64) (Settings, error) {
	return Settings{}, errors.New("not implemented")
}

// SetConsensusEnabled writes consensus_enabled with a column-targeted upsert.
func SetConsensusEnabled(ctx context.Context, chatID int64, enabled bool, by int64) error {
	return errors.New("not implemented")
}

// SetConsensusThreshold writes consensus_threshold with a column-targeted upsert.
func SetConsensusThreshold(ctx context.Context, chatID int64, threshold int, by int64) error {
	return errors.New("not implemented")
}

// SetRetractEnabled writes retract_enabled with a column-targeted upsert.
func SetRetractEnabled(ctx context.Context, chatID int64, enabled bool, by int64) error {
	return errors.New("not implemented")
}
