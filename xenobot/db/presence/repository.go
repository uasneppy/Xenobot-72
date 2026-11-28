// Package presence stores each group's admin-configurable Interactive Presence settings and
// durable Highlights Archive deduplication records.
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

	// DefaultArchiveThreshold is the default distinct-member count before Highlights Archive fires.
	DefaultArchiveThreshold = 5
	// MinArchiveThreshold and MaxArchiveThreshold bound the configurable archive threshold.
	MinArchiveThreshold = 2
	MaxArchiveThreshold = 50

	// ArchiveEmojiModePositive and ArchiveEmojiModeAll are the stored values for archive_emoji_mode.
	ArchiveEmojiModePositive = "positive"
	ArchiveEmojiModeAll      = "all"

	// ArchiveDeliveryCopy and ArchiveDeliveryForward are the stored values for archive_delivery.
	ArchiveDeliveryCopy    = "copy"
	ArchiveDeliveryForward = "forward"

	cacheTTL = cache.DefaultCacheTTL
)

var (
	errNoDatabase = errors.New("database not initialized")
	// ErrInvalidValue is returned, before any write, for a value outside the stored bounds or invalid state.
	ErrInvalidValue = errors.New("presence: invalid value")
)

// Settings is a chat's Interactive Presence settings.
type Settings struct {
	ConsensusEnabled   bool   `msgpack:"ce"`
	ConsensusThreshold int    `msgpack:"ct"`
	RetractEnabled     bool   `msgpack:"re"`
	AIReactMode        string `msgpack:"ar"`
	CatStickerEnabled  bool   `msgpack:"cs"`
	ArchiveEnabled     bool   `msgpack:"ae"`
	ArchiveChatID      int64  `msgpack:"ac"`
	ArchiveThreadID    int64  `msgpack:"at"`
	ArchiveThreshold   int    `msgpack:"ah"`
	ArchiveEmojiMode   string `msgpack:"am"`
	ArchiveDelivery    string `msgpack:"ad"`
	AmbientContext     bool   `msgpack:"ax"`
	VocativeEnabled    bool   `msgpack:"ve"`
	PMBriefReact       bool   `msgpack:"pb"`
	ReferendumEnabled  bool   `msgpack:"rf"`
	InspectEnabled     bool   `msgpack:"in"`
}

// Defaults returns the settings of a chat with no row: Emoji Consensus off with threshold 3,
// thumbs-down retraction on, Contextual AI Emoji Reactions off, Cat-Sticker Micro-Triggers off,
// Highlights Archive off (threshold 5, positive emojis, copy delivery), Ambient Room Context off,
// Vocative Name-Addressing off, PM Brief Reaction off, Referendum off, and Inspect off.
func Defaults() Settings {
	return Settings{
		ConsensusEnabled:   false,
		ConsensusThreshold: DefaultConsensusThreshold,
		RetractEnabled:     true,
		AIReactMode:        AIReactModeOff,
		CatStickerEnabled:  false,
		ArchiveEnabled:     false,
		ArchiveChatID:      0,
		ArchiveThreadID:    0,
		ArchiveThreshold:   DefaultArchiveThreshold,
		ArchiveEmojiMode:   ArchiveEmojiModePositive,
		ArchiveDelivery:    ArchiveDeliveryCopy,
		AmbientContext:     false,
		VocativeEnabled:    false,
		PMBriefReact:       false,
		ReferendumEnabled:  false,
		InspectEnabled:     false,
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

// ValidArchiveThreshold reports whether n is within [MinArchiveThreshold, MaxArchiveThreshold].
func ValidArchiveThreshold(n int) bool {
	return n >= MinArchiveThreshold && n <= MaxArchiveThreshold
}

// ValidArchiveEmojiMode reports whether mode is one of the allowed archive_emoji_mode values.
func ValidArchiveEmojiMode(mode string) bool {
	return mode == ArchiveEmojiModePositive || mode == ArchiveEmojiModeAll
}

// ValidArchiveDelivery reports whether mode is one of the allowed archive_delivery values.
func ValidArchiveDelivery(mode string) bool {
	return mode == ArchiveDeliveryCopy || mode == ArchiveDeliveryForward
}

func normalizeConsensusThreshold(n int) int {
	if ValidConsensusThreshold(n) {
		return n
	}
	return DefaultConsensusThreshold
}

func normalizeAIReactMode(mode string) string {
	if ValidAIReactMode(mode) {
		return mode
	}
	return AIReactModeOff
}

func normalizeArchiveThreshold(n int) int {
	if ValidArchiveThreshold(n) {
		return n
	}
	return DefaultArchiveThreshold
}

func normalizeArchiveEmojiMode(mode string) string {
	if ValidArchiveEmojiMode(mode) {
		return mode
	}
	return ArchiveEmojiModePositive
}

func normalizeArchiveDelivery(mode string) string {
	if ValidArchiveDelivery(mode) {
		return mode
	}
	return ArchiveDeliveryCopy
}

func normalizeSettings(s Settings) Settings {
	s.ConsensusThreshold = normalizeConsensusThreshold(s.ConsensusThreshold)
	s.AIReactMode = normalizeAIReactMode(s.AIReactMode)
	s.ArchiveThreshold = normalizeArchiveThreshold(s.ArchiveThreshold)
	s.ArchiveEmojiMode = normalizeArchiveEmojiMode(s.ArchiveEmojiMode)
	s.ArchiveDelivery = normalizeArchiveDelivery(s.ArchiveDelivery)
	return s
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
	return normalizeSettings(s), nil
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
	return normalizeSettings(Settings{
		ConsensusEnabled:   row.ConsensusEnabled,
		ConsensusThreshold: int(row.ConsensusThreshold),
		RetractEnabled:     row.RetractEnabled,
		AIReactMode:        row.AIReactMode,
		CatStickerEnabled:  row.CatStickerEnabled,
		ArchiveEnabled:     row.ArchiveEnabled,
		ArchiveChatID:      row.ArchiveChatID,
		ArchiveThreadID:    row.ArchiveThreadID,
		ArchiveThreshold:   int(row.ArchiveThreshold),
		ArchiveEmojiMode:   row.ArchiveEmojiMode,
		ArchiveDelivery:    row.ArchiveDelivery,
		AmbientContext:     row.AmbientContext,
		VocativeEnabled:    row.VocativeEnabled,
		PMBriefReact:       row.PMBriefReact,
		ReferendumEnabled:  row.ReferendumEnabled,
		InspectEnabled:     row.InspectEnabled,
	}), nil
}

// SetConsensusEnabled writes consensus_enabled with one column-targeted upsert and drops the cache key.
func SetConsensusEnabled(ctx context.Context, chatID int64, enabled bool, by int64) error {
	row := defaultRow(chatID, by)
	row.ConsensusEnabled = enabled
	return upsertColumns(ctx, row, []string{"consensus_enabled"})
}

// SetConsensusThreshold writes consensus_threshold, refusing values outside [2, 20] before any write.
func SetConsensusThreshold(ctx context.Context, chatID int64, threshold int, by int64) error {
	if !ValidConsensusThreshold(threshold) {
		return ErrInvalidValue
	}
	row := defaultRow(chatID, by)
	row.ConsensusThreshold = int16(threshold) // #nosec G115 -- validated as 2 to 20
	return upsertColumns(ctx, row, []string{"consensus_threshold"})
}

// SetRetractEnabled writes retract_enabled with one column-targeted upsert and drops the cache key.
func SetRetractEnabled(ctx context.Context, chatID int64, enabled bool, by int64) error {
	row := defaultRow(chatID, by)
	row.RetractEnabled = enabled
	return upsertColumns(ctx, row, []string{"retract_enabled"})
}

// SetAIReactMode writes ai_react_mode, refusing values outside ('off', 'low', 'medium') before any write.
func SetAIReactMode(ctx context.Context, chatID int64, mode string, by int64) error {
	if !ValidAIReactMode(mode) {
		return ErrInvalidValue
	}
	row := defaultRow(chatID, by)
	row.AIReactMode = mode
	return upsertColumns(ctx, row, []string{"ai_react_mode"})
}

// SetCatStickerEnabled writes cat_sticker_enabled with one column-targeted upsert and drops the cache key.
func SetCatStickerEnabled(ctx context.Context, chatID int64, enabled bool, by int64) error {
	row := defaultRow(chatID, by)
	row.CatStickerEnabled = enabled
	return upsertColumns(ctx, row, []string{"cat_sticker_enabled"})
}

// SetArchiveEnabled writes archive_enabled, refusing enabled=true with ErrInvalidValue while the
// chat has no linked archive_chat_id.
func SetArchiveEnabled(ctx context.Context, chatID int64, enabled bool, by int64) error {
	if db.DB == nil {
		return xenoerrors.Wrap(errNoDatabase, "write chat presence setting")
	}
	if !enabled {
		row := defaultRow(chatID, by)
		row.ArchiveEnabled = false
		return upsertColumns(ctx, row, []string{"archive_enabled"})
	}
	err := db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current models.ChatPresence
		err := tx.Where("chat_id = ?", chatID).Take(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && current.ArchiveChatID == 0) {
			return ErrInvalidValue
		}
		if err != nil {
			return xenoerrors.Wrapf(err, "read the presence setting of chat %d", chatID)
		}
		row := defaultRow(chatID, by)
		row.ArchiveEnabled = true
		return upsertColumnsTx(tx, row, []string{"archive_enabled"})
	})
	if err != nil {
		return err
	}
	cache.DeleteCache(CacheKey(chatID))
	return nil
}

// SetArchiveTarget links targetChatID (and optional targetThreadID) as chatID's Highlights Archive
// destination and enables archiving, dropping the cache key.
func SetArchiveTarget(ctx context.Context, chatID, targetChatID, targetThreadID, by int64) error {
	if targetChatID == 0 {
		return ErrInvalidValue
	}
	row := defaultRow(chatID, by)
	row.ArchiveChatID = targetChatID
	row.ArchiveThreadID = targetThreadID
	row.ArchiveEnabled = true
	return upsertColumns(ctx, row, []string{"archive_chat_id", "archive_thread_id", "archive_enabled"})
}

// ClearArchiveTarget zeroes archive_chat_id and archive_thread_id and disables archiving, without
// overwriting unrelated presence settings, and drops the cache key.
func ClearArchiveTarget(ctx context.Context, chatID, by int64) error {
	row := defaultRow(chatID, by)
	row.ArchiveChatID = 0
	row.ArchiveThreadID = 0
	row.ArchiveEnabled = false
	return upsertColumns(ctx, row, []string{"archive_chat_id", "archive_thread_id", "archive_enabled"})
}

// SetArchiveThreshold writes archive_threshold, refusing values outside [2, 50] before any write.
func SetArchiveThreshold(ctx context.Context, chatID int64, threshold int, by int64) error {
	if !ValidArchiveThreshold(threshold) {
		return ErrInvalidValue
	}
	row := defaultRow(chatID, by)
	row.ArchiveThreshold = int16(threshold) // #nosec G115 -- validated as 2 to 50
	return upsertColumns(ctx, row, []string{"archive_threshold"})
}

// SetArchiveEmojiMode writes archive_emoji_mode, refusing values outside ('positive', 'all') before any write.
func SetArchiveEmojiMode(ctx context.Context, chatID int64, mode string, by int64) error {
	if !ValidArchiveEmojiMode(mode) {
		return ErrInvalidValue
	}
	row := defaultRow(chatID, by)
	row.ArchiveEmojiMode = mode
	return upsertColumns(ctx, row, []string{"archive_emoji_mode"})
}

// SetArchiveDelivery writes archive_delivery, refusing values outside ('copy', 'forward') before any write.
func SetArchiveDelivery(ctx context.Context, chatID int64, delivery string, by int64) error {
	if !ValidArchiveDelivery(delivery) {
		return ErrInvalidValue
	}
	row := defaultRow(chatID, by)
	row.ArchiveDelivery = delivery
	return upsertColumns(ctx, row, []string{"archive_delivery"})
}

// SetAmbientContext writes ambient_context with one column-targeted upsert and drops the cache key.
func SetAmbientContext(ctx context.Context, chatID int64, enabled bool, by int64) error {
	row := defaultRow(chatID, by)
	row.AmbientContext = enabled
	return upsertColumns(ctx, row, []string{"ambient_context"})
}

// SetVocativeEnabled writes vocative_enabled with one column-targeted upsert and drops the cache key.
func SetVocativeEnabled(ctx context.Context, chatID int64, enabled bool, by int64) error {
	row := defaultRow(chatID, by)
	row.VocativeEnabled = enabled
	return upsertColumns(ctx, row, []string{"vocative_enabled"})
}

// SetPMBriefReact writes pm_brief_react with one column-targeted upsert and drops the cache key.
func SetPMBriefReact(ctx context.Context, chatID int64, enabled bool, by int64) error {
	row := defaultRow(chatID, by)
	row.PMBriefReact = enabled
	return upsertColumns(ctx, row, []string{"pm_brief_react"})
}

// SetReferendumEnabled writes referendum_enabled with one column-targeted upsert and drops the cache key.
func SetReferendumEnabled(ctx context.Context, chatID int64, enabled bool, by int64) error {
	row := defaultRow(chatID, by)
	row.ReferendumEnabled = enabled
	return upsertColumns(ctx, row, []string{"referendum_enabled"})
}

// SetInspectEnabled writes inspect_enabled with one column-targeted upsert and drops the cache key.
func SetInspectEnabled(ctx context.Context, chatID int64, enabled bool, by int64) error {
	row := defaultRow(chatID, by)
	row.InspectEnabled = enabled
	return upsertColumns(ctx, row, []string{"inspect_enabled"})
}

// TryClaimArchived durably claims (chatID, msgID) in chat_presence_archived via ON CONFLICT DO
// NOTHING, returning true iff this caller inserted the row.
func TryClaimArchived(ctx context.Context, chatID, msgID int64) (bool, error) {
	if chatID == 0 || msgID <= 0 {
		return false, ErrInvalidValue
	}
	if db.DB == nil {
		return false, xenoerrors.Wrap(errNoDatabase, "claim archived presence message")
	}
	res := db.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}, {Name: "message_id"}},
		DoNothing: true,
	}).Create(&models.ChatPresenceArchived{
		ChatID:     chatID,
		MessageID:  msgID,
		ArchivedAt: time.Now(),
	})
	if res.Error != nil {
		return false, xenoerrors.Wrapf(res.Error, "claim archived message %d in chat %d", msgID, chatID)
	}
	return res.RowsAffected == 1, nil
}

// ReleaseArchived removes the claim TryClaimArchived made for (chatID, msgID), so a highlight whose
// delivery failed for a reason that may pass (a timeout, a 5xx, a rate limit) can be claimed and
// delivered again by a later reaction. It reports whether a row was removed.
func ReleaseArchived(ctx context.Context, chatID, msgID int64) (bool, error) {
	if chatID == 0 || msgID <= 0 {
		return false, ErrInvalidValue
	}
	if db.DB == nil {
		return false, xenoerrors.Wrap(errNoDatabase, "release archived presence message")
	}
	res := db.DB.WithContext(ctx).
		Where("chat_id = ? AND message_id = ?", chatID, msgID).
		Delete(&models.ChatPresenceArchived{})
	if res.Error != nil {
		return false, xenoerrors.Wrapf(res.Error, "release archived message %d in chat %d", msgID, chatID)
	}
	return res.RowsAffected == 1, nil
}

// ReplacePortableTx writes all portable presence settings into tx for chatID while preserving
// the target chat's own archive_chat_id and archive_thread_id. When s.ArchiveEnabled is true
// and the target chat has no linked archive target, archive_enabled is forced false and
// archiveSkipped is returned true. The caller is responsible for dropping CacheKey(chatID) after
// the transaction commits.
func ReplacePortableTx(tx *gorm.DB, chatID int64, s Settings, by int64) (archiveSkipped bool, err error) {
	if tx == nil {
		return false, xenoerrors.Wrap(errNoDatabase, "write chat presence setting")
	}
	var existing models.ChatPresence
	err = tx.Where("chat_id = ?", chatID).Take(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, xenoerrors.Wrapf(err, "read presence setting of chat %d", chatID)
	}
	var existingArchiveChatID, existingArchiveThreadID int64
	if err == nil {
		existingArchiveChatID = existing.ArchiveChatID
		existingArchiveThreadID = existing.ArchiveThreadID
	}

	norm := normalizeSettings(s)
	if norm.ArchiveEnabled && existingArchiveChatID == 0 {
		norm.ArchiveEnabled = false
		archiveSkipped = true
	}

	row := &models.ChatPresence{
		ChatID:             chatID,
		ConsensusEnabled:   norm.ConsensusEnabled,
		ConsensusThreshold: int16(norm.ConsensusThreshold), // #nosec G115 -- normalized to 2..20
		RetractEnabled:     norm.RetractEnabled,
		AIReactMode:        norm.AIReactMode,
		CatStickerEnabled:  norm.CatStickerEnabled,
		ArchiveEnabled:     norm.ArchiveEnabled,
		ArchiveChatID:      existingArchiveChatID,
		ArchiveThreadID:    existingArchiveThreadID,
		ArchiveThreshold:   int16(norm.ArchiveThreshold), // #nosec G115 -- normalized to 2..50
		ArchiveEmojiMode:   norm.ArchiveEmojiMode,
		ArchiveDelivery:    norm.ArchiveDelivery,
		AmbientContext:     norm.AmbientContext,
		VocativeEnabled:    norm.VocativeEnabled,
		PMBriefReact:       norm.PMBriefReact,
		ReferendumEnabled:  norm.ReferendumEnabled,
		InspectEnabled:     norm.InspectEnabled,
		UpdatedBy:          by,
		UpdatedAt:          time.Now(),
	}
	portableCols := []string{
		"consensus_enabled",
		"consensus_threshold",
		"retract_enabled",
		"ai_react_mode",
		"cat_sticker_enabled",
		"archive_enabled",
		"archive_threshold",
		"archive_emoji_mode",
		"archive_delivery",
		"ambient_context",
		"vocative_enabled",
		"pm_brief_react",
		"referendum_enabled",
		"inspect_enabled",
	}
	if err := upsertColumnsTx(tx, row, portableCols); err != nil {
		return false, err
	}
	return archiveSkipped, nil
}

// ClearTx deletes chatID's chat_presence row inside caller-supplied transaction tx so subsequent
// reads return Defaults(). The caller is responsible for dropping CacheKey(chatID) after commit.
func ClearTx(tx *gorm.DB, chatID int64) error {
	if tx == nil {
		return xenoerrors.Wrap(errNoDatabase, "clear chat presence setting")
	}
	if err := tx.Where("chat_id = ?", chatID).Delete(&models.ChatPresence{}).Error; err != nil {
		return xenoerrors.Wrapf(err, "clear presence setting of chat %d", chatID)
	}
	return nil
}

func defaultRow(chatID, by int64) *models.ChatPresence {
	d := Defaults()
	return &models.ChatPresence{
		ChatID:             chatID,
		ConsensusEnabled:   d.ConsensusEnabled,
		ConsensusThreshold: int16(d.ConsensusThreshold), // #nosec G115 -- DefaultConsensusThreshold is 3
		RetractEnabled:     d.RetractEnabled,
		AIReactMode:        d.AIReactMode,
		CatStickerEnabled:  d.CatStickerEnabled,
		ArchiveEnabled:     d.ArchiveEnabled,
		ArchiveChatID:      d.ArchiveChatID,
		ArchiveThreadID:    d.ArchiveThreadID,
		ArchiveThreshold:   int16(d.ArchiveThreshold), // #nosec G115 -- DefaultArchiveThreshold is 5
		ArchiveEmojiMode:   d.ArchiveEmojiMode,
		ArchiveDelivery:    d.ArchiveDelivery,
		AmbientContext:     d.AmbientContext,
		VocativeEnabled:    d.VocativeEnabled,
		PMBriefReact:       d.PMBriefReact,
		ReferendumEnabled:  d.ReferendumEnabled,
		InspectEnabled:     d.InspectEnabled,
		UpdatedBy:          by,
		UpdatedAt:          time.Now(),
	}
}

// upsertColumns inserts the row or updates only cols, updated_by, and updated_at, so concurrent
// admin presses on different controls never clobber each other, and drops the cache key.
func upsertColumns(ctx context.Context, row *models.ChatPresence, cols []string) error {
	if db.DB == nil {
		return xenoerrors.Wrap(errNoDatabase, "write chat presence setting")
	}
	if err := upsertColumnsTx(db.DB.WithContext(ctx), row, cols); err != nil {
		return err
	}
	cache.DeleteCache(CacheKey(row.ChatID))
	return nil
}

func upsertColumnsTx(tx *gorm.DB, row *models.ChatPresence, cols []string) error {
	if tx == nil {
		return xenoerrors.Wrap(errNoDatabase, "write chat presence setting")
	}
	assign := make([]string, 0, len(cols)+2)
	assign = append(assign, cols...)
	assign = append(assign, "updated_by", "updated_at")
	err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.AssignmentColumns(assign),
	}).Create(row).Error
	if err != nil {
		return xenoerrors.Wrapf(err, "write presence settings of chat %d", row.ChatID)
	}
	return nil
}
