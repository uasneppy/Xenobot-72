package antiflood

import (
	"context"
	"errors"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
)

const defaultFloodsettingsMode string = "mute"

func GetFlood(chatID int64) *models.AntifloodSettings {
	return GetFloodContext(context.Background(), chatID)
}

func GetFloodContext(ctx context.Context, chatID int64) (floodSrc *models.AntifloodSettings) {
	floodSrc, err := GetAntifloodSettingsCachedContext(ctx, chatID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &models.AntifloodSettings{ChatId: chatID, Limit: 0, Action: defaultFloodsettingsMode}
		}
		log.Errorf("[Database][GetFlood]: %v", err)
		return &models.AntifloodSettings{ChatId: chatID, Limit: 0, Action: defaultFloodsettingsMode}
	}
	return floodSrc
}

func upsertChatField(ctx context.Context, chatID int64, updates map[string]any) error {
	if err := db.DB.WithContext(ctx).Where("chat_id = ?", chatID).
		Assign(updates).
		FirstOrCreate(&models.AntifloodSettings{}).Error; err != nil {
		log.Errorf("[Database] upsertChatField: %v - %d", err, chatID)
		return err
	}
	cache.DeleteCache(cache.CacheKey("antiflood", chatID))
	return nil
}

func SetFlood(chatID int64, limit int) error {
	return SetFloodContext(context.Background(), chatID, limit)
}

func SetFloodContext(ctx context.Context, chatID int64, limit int) error {
	floodSrc, err := GetAntifloodSettingsCachedContext(ctx, chatID)
	if err != nil {
		return err
	}

	if floodSrc.Limit == limit {
		return nil
	}

	// Update only flood_limit: writing back the cached action would clobber a
	// concurrent SetFloodMode with a stale value.
	updates := map[string]any{
		"chat_id":     chatID,
		"flood_limit": limit,
	}
	return upsertChatField(ctx, chatID, updates)
}

func SetFloodMode(chatID int64, mode string) error {
	return SetFloodModeContext(context.Background(), chatID, mode)
}

// SetFloodModeContext stores a plain flood action with seconds 0 (SetFloodActionContext with 0): a
// no-op when that action is already stored with no duration.
func SetFloodModeContext(ctx context.Context, chatID int64, mode string) error {
	return SetFloodActionContext(ctx, chatID, mode, 0)
}

// SetFloodActionContext stores the flood action and its duration in seconds (action_secs, 0 for a
// plain action) in one write of only those two columns, so a concurrent limit or delete write is never
// clobbered. It writes nothing when the cached settings already hold both values; otherwise the write
// creates a missing row (with the model's flood_limit default of 5, D-27) and drops antiflood:<chat>.
// The database check refuses an action or seconds outside the stored ones.
func SetFloodActionContext(ctx context.Context, chatID int64, mode string, secs int) error {
	floodSrc, err := GetAntifloodSettingsCachedContext(ctx, chatID)
	if err != nil {
		return err
	}
	if floodSrc.Action == mode && floodSrc.ActionSecs == secs {
		return nil
	}
	updates := map[string]any{
		"chat_id":     chatID,
		"action":      mode,
		"action_secs": secs,
	}
	return upsertChatField(ctx, chatID, updates)
}

// GetFloodFreshContext reads a chat's antiflood_settings row directly from the database without
// using the cache and without creating a row. When no row exists, it returns default settings
// (Limit: 0, Action: "mute", ActionSecs: 0), false, nil.
func GetFloodFreshContext(ctx context.Context, chatID int64) (*models.AntifloodSettings, bool, error) {
	if db.DB == nil {
		return nil, false, errors.New("database not initialized")
	}
	var row models.AntifloodSettings
	err := db.DB.WithContext(ctx).Where("chat_id = ?", chatID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &models.AntifloodSettings{ChatId: chatID, Limit: 0, Action: defaultFloodsettingsMode}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &row, true, nil
}

// SetFloodActionPreserveOffTx writes ONLY action and action_secs for chatID inside
// caller-supplied transaction tx, preserving an existing row's flood_limit and
// delete_antiflood_message (and inserting flood_limit = 0 when absent). The caller is responsible
// for dropping antiflood:<chat> after commit.
func SetFloodActionPreserveOffTx(tx *gorm.DB, chatID int64, mode string, secs int) error {
	if tx == nil {
		return errors.New("database not initialized")
	}
	now := time.Now()
	const sqlStmt = `INSERT INTO antiflood_settings (chat_id, flood_limit, action, action_secs, delete_antiflood_message, created_at, updated_at)
VALUES (?, 0, ?, ?, ?, ?, ?)
ON CONFLICT (chat_id) DO UPDATE SET
	action = EXCLUDED.action,
	action_secs = EXCLUDED.action_secs,
	updated_at = EXCLUDED.updated_at`
	if err := tx.Exec(sqlStmt, chatID, mode, secs, false, now, now).Error; err != nil {
		log.Errorf("[Database] SetFloodActionPreserveOffContext: %v - %d", err, chatID)
		return err
	}
	return nil
}

// SetFloodActionPreserveOffContext writes ONLY action and action_secs for chatID, preserving an
// existing row's flood_limit and delete_antiflood_message. When chatID has no antiflood_settings
// row yet, it inserts flood_limit = 0 explicitly (bypassing GORM's default:5 tag) so Copy Settings
// never silently turns antiflood on for an unconfigured target group (P29-04). Drops
// antiflood:<chat> after a successful write.
func SetFloodActionPreserveOffContext(ctx context.Context, chatID int64, mode string, secs int) error {
	if db.DB == nil {
		return errors.New("database not initialized")
	}
	if err := SetFloodActionPreserveOffTx(db.DB.WithContext(ctx), chatID, mode, secs); err != nil {
		return err
	}
	cache.DeleteCache(cache.CacheKey("antiflood", chatID))
	return nil
}

func SetFloodMsgDel(chatID int64, val bool) error {
	return SetFloodMsgDelContext(context.Background(), chatID, val)
}

func SetFloodMsgDelContext(ctx context.Context, chatID int64, val bool) error {
	floodSrc, err := GetAntifloodSettingsCachedContext(ctx, chatID)
	if err != nil {
		return err
	}
	if floodSrc.DeleteAntifloodMessage == val {
		return nil
	}
	updates := map[string]any{
		"chat_id":                  chatID,
		"delete_antiflood_message": val,
	}
	return upsertChatField(ctx, chatID, updates)
}

func LoadAntifloodStats() (antiCount int64) {
	var totalCount int64
	var noAntiCount int64

	err := db.DB.Model(&models.AntifloodSettings{}).Count(&totalCount).Error
	if err != nil {
		log.Errorf("[Database] LoadAntifloodStats: %v", err)
		return 0
	}

	err = db.DB.Model(&models.AntifloodSettings{}).Where("flood_limit = ?", 0).Count(&noAntiCount).Error
	if err != nil {
		log.Errorf("[Database] LoadAntifloodStats: %v", err)
		return 0
	}

	antiCount = totalCount - noAntiCount

	return
}
