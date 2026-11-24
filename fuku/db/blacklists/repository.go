package blacklists

import (
	"context"
	"errors"
	"strings"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
)

// AddBlacklist stores a word for the chat with the warn action. It is the wrapper
// of AddBlacklistContext that the older callers use.
func AddBlacklist(chatId int64, trigger string) error {
	return AddBlacklistContext(context.Background(), chatId, trigger, "warn")
}

// AddBlacklistContext stores a lower-cased word for the chat with the given action and seconds 0.
// It is the wrapper of AddBlacklistTimedContext that the older callers use.
func AddBlacklistContext(ctx context.Context, chatId int64, trigger, action string) error {
	return AddBlacklistTimedContext(ctx, chatId, trigger, action, 0)
}

// AddBlacklistTimedContext stores a lower-cased word for the chat with the given action and its
// seconds (the duration of a timed action, 0 for a plain one) and drops the chat's cached blacklist
// after a successful insert.
func AddBlacklistTimedContext(ctx context.Context, chatId int64, trigger, action string, secs int) error {
	blacklist := &models.BlacklistSettings{
		ChatId:     chatId,
		Word:       strings.ToLower(trigger),
		Action:     action,
		ActionSecs: secs,
		Reason:     "Blacklisted word: '%s'",
	}

	err := db.CreateRecordContext(ctx, blacklist)
	if err != nil {
		log.Errorf("[Database] AddBlacklist: %v - %d", err, chatId)
		return err
	}

	cache.DeleteCache(cache.CacheKey("blacklist", chatId))
	return nil
}

// RemoveBlacklist deletes one word of the chat. It is the wrapper of
// RemoveBlacklistContext that the older callers use.
func RemoveBlacklist(chatId int64, trigger string) error {
	return RemoveBlacklistContext(context.Background(), chatId, trigger)
}

// RemoveBlacklistContext deletes the lower-cased word of the chat and drops the chat's
// cached blacklist only when a row went.
func RemoveBlacklistContext(ctx context.Context, chatId int64, trigger string) error {
	result := db.DB.WithContext(ctx).Where("chat_id = ? AND word = ?", chatId, strings.ToLower(trigger)).Delete(&models.BlacklistSettings{})
	if result.Error != nil {
		log.Errorf("[Database] RemoveBlacklist: %v - %d", result.Error, chatId)
		return result.Error
	}

	if result.RowsAffected > 0 {
		cache.DeleteCache(cache.CacheKey("blacklist", chatId))
	}
	return nil
}

func RemoveAllBlacklist(chatId int64) error {
	err := db.DB.Where("chat_id = ?", chatId).Delete(&models.BlacklistSettings{}).Error
	if err != nil {
		log.Errorf("[Database] RemoveAllBlacklist: %v - %d", err, chatId)
		return err
	}

	cache.DeleteCache(cache.CacheKey("blacklist", chatId))
	return nil
}

// SetBlacklistAction sets the action of every word of the chat. It is the wrapper of
// SetBlacklistActionContext that the older callers use.
func SetBlacklistAction(chatId int64, action string) error {
	_, err := SetBlacklistActionContext(context.Background(), chatId, action)
	return err
}

// SetBlacklistActionContext sets the lower-cased action of every word of the chat with seconds
// 0. It is the wrapper of SetBlacklistActionTimedContext that the older callers use.
func SetBlacklistActionContext(ctx context.Context, chatId int64, action string) (int64, error) {
	return SetBlacklistActionTimedContext(ctx, chatId, action, 0)
}

// SetBlacklistActionTimedTx sets the lower-cased action and its seconds of every word of chatId
// inside caller-supplied transaction tx. The caller is responsible for dropping blacklist:<chat>
// after commit.
func SetBlacklistActionTimedTx(tx *gorm.DB, chatId int64, action string, secs int) (int64, error) {
	if tx == nil {
		return 0, errors.New("database not initialized")
	}
	result := tx.Model(&models.BlacklistSettings{}).Where("chat_id = ?", chatId).
		Updates(map[string]any{"action": strings.ToLower(action), "action_secs": secs})
	if result.Error != nil {
		log.Errorf("[Database] SetBlacklistAction: %v - %d", result.Error, chatId)
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// SetBlacklistActionTimedContext sets the lower-cased action and its seconds (the duration of a
// timed action, 0 for a plain one) of every word of the chat in one update of both columns,
// drops the chat's cached blacklist after a successful update and returns how many rows the
// update matched (zero for a chat with no words).
func SetBlacklistActionTimedContext(ctx context.Context, chatId int64, action string, secs int) (int64, error) {
	rows, err := SetBlacklistActionTimedTx(db.DB.WithContext(ctx), chatId, action, secs)
	if err != nil {
		return 0, err
	}
	cache.DeleteCache(cache.CacheKey("blacklist", chatId))
	return rows, nil
}

func GetBlacklistSettings(chatId int64) models.BlacklistSettingsSlice {
	return GetBlacklistSettingsContext(context.Background(), chatId)
}

func GetBlacklistSettingsContext(ctx context.Context, chatId int64) models.BlacklistSettingsSlice {
	cacheKey := cache.CacheKey("blacklist", chatId)
	result, err := cache.GetFromCacheOrLoad(ctx, cacheKey, cache.CacheTTLBlacklist, func(ctx context.Context) (models.BlacklistSettingsSlice, error) {
		var blacklists []*models.BlacklistSettings
		err := db.GetRecordsContext(ctx, &blacklists, models.BlacklistSettings{ChatId: chatId})
		if err != nil {
			log.Errorf("[Database] GetBlacklistSettings: %v - %d", err, chatId)
			return models.BlacklistSettingsSlice{}, err
		}
		return models.BlacklistSettingsSlice(blacklists), nil
	})
	if err != nil {
		return models.BlacklistSettingsSlice{}
	}
	return result
}

// ListBlacklistEntriesFresh reads the chat's blacklist rows (ID, word, action and its seconds)
// with one query. It is fresh, chat-scoped and uncached, returns its error, and creates nothing.
func ListBlacklistEntriesFresh(ctx context.Context, chatId int64) ([]models.BlacklistSettings, error) {
	var rows []models.BlacklistSettings
	err := db.DB.WithContext(ctx).Select("id", "word", "action", "action_secs").Where("chat_id = ?", chatId).Order("id").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// GetBlacklistEntryFresh reads one blacklist row by its ID, only when it belongs to the
// chat: a missing ID and the ID of another chat's row both return nil and a nil error. It is
// fresh, chat-scoped and uncached, returns its read error, and creates nothing.
func GetBlacklistEntryFresh(ctx context.Context, chatId, id int64) (*models.BlacklistSettings, error) {
	var row models.BlacklistSettings
	err := db.DB.WithContext(ctx).Where("id = ? AND chat_id = ?", id, chatId).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// CountBlacklistFresh counts the chat's blacklist rows. It is fresh, chat-scoped and
// uncached, returns its error, and creates nothing.
func CountBlacklistFresh(ctx context.Context, chatId int64) (int64, error) {
	var count int64
	err := db.DB.WithContext(ctx).Model(&models.BlacklistSettings{}).Where("chat_id = ?", chatId).Count(&count).Error
	return count, err
}

// RemoveBlacklistByID deletes one blacklist row by its ID, only when it belongs to the chat,
// in one conditional statement, and returns the stored word of the row that went. A missing ID
// and the ID of another chat's row delete nothing and return removed false. The chat's cached
// blacklist is dropped only when a row went.
func RemoveBlacklistByID(ctx context.Context, chatId, id int64) (word string, removed bool, err error) {
	var rows []models.BlacklistSettings
	result := db.DB.WithContext(ctx).Clauses(clause.Returning{}).Where("id = ? AND chat_id = ?", id, chatId).Delete(&rows)
	if result.Error != nil {
		log.Errorf("[Database] RemoveBlacklistByID: %v - %d", result.Error, chatId)
		return "", false, result.Error
	}
	if len(rows) != 1 {
		return "", false, nil
	}
	cache.DeleteCache(cache.CacheKey("blacklist", chatId))
	return rows[0].Word, true, nil
}

func LoadBlacklistsStats() (blacklistTriggers, blacklistChats int64) {
	err := db.DB.Model(&models.BlacklistSettings{}).Count(&blacklistTriggers).Error
	if err != nil {
		log.Errorf("[Database] LoadBlacklistsStats (triggers): %v", err)
		return 0, 0
	}

	err = db.DB.Model(&models.BlacklistSettings{}).Distinct("chat_id").Count(&blacklistChats).Error
	if err != nil {
		log.Errorf("[Database] LoadBlacklistsStats (chats): %v", err)
		return blacklistTriggers, 0
	}

	return
}
