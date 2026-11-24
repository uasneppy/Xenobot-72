package warns

import (
	"context"
	"errors"
	"time"
	"unicode/utf8"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/chats"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/db/user"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
)

// checkWarnSettingsContext reads the chat's warn settings and creates the default row for a
// chat the bot knows. A read or write that fails is returned, never replaced by the defaults,
// so the cache loader stores nothing for it and a caller that punishes can fail closed.
func checkWarnSettingsContext(ctx context.Context, chatID int64) (warnrc *models.WarnSettings, err error) {
	defaultWarnSettings := &models.WarnSettings{ChatId: chatID, WarnLimit: 3, WarnMode: "mute"}
	warnrc = &models.WarnSettings{}
	err = db.DB.WithContext(ctx).Where("chat_id = ?", chatID).First(warnrc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if !db.ChatExistsContext(ctx, chatID) {
			log.Debugf("[Database][checkWarnSettings]: Chat %d doesn't exist, returning default settings", chatID)
			return defaultWarnSettings, nil
		}

		warnrc = defaultWarnSettings
		result := db.DB.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "chat_id"}},
			DoNothing: true,
		}).Create(warnrc)
		if result.Error != nil {
			log.Errorf("[Database] checkWarnSettings: %v", result.Error)
			return nil, result.Error
		} else if result.RowsAffected == 0 {
			if err := db.DB.WithContext(ctx).Where("chat_id = ?", chatID).First(warnrc).Error; err != nil {
				log.Errorf("[Database] checkWarnSettings reload: %v", err)
				return nil, err
			}
		}
	} else if err != nil {
		log.Errorf("[Database][checkWarnSettings]: %d - %v", chatID, err)
		return nil, err
	}
	return warnrc, nil
}

func checkWarns(userId, chatId int64) (warnrc *models.Warns) {
	return checkWarnsContext(context.Background(), userId, chatId)
}

func checkWarnsContext(ctx context.Context, userId, chatId int64) (warnrc *models.Warns) {
	defaultWarnSrc := &models.Warns{UserId: userId, ChatId: chatId, NumWarns: 0, Reasons: make(models.StringArray, 0)}
	warnrc = &models.Warns{}
	err := db.DB.WithContext(ctx).Where("user_id = ? AND chat_id = ?", userId, chatId).First(warnrc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if !db.ChatExistsContext(ctx, chatId) {
			log.Debugf("[Database][checkWarns]: Chat %d doesn't exist, returning default settings", chatId)
			return defaultWarnSrc
		}

		warnrc = defaultWarnSrc
		result := db.DB.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "chat_id"}},
			DoNothing: true,
		}).Create(warnrc)
		if result.Error != nil {
			log.Errorf("[Database] checkWarns: %v", result.Error)
		} else if result.RowsAffected == 0 {
			if err := db.DB.WithContext(ctx).Where("user_id = ? AND chat_id = ?", userId, chatId).First(warnrc).Error; err != nil {
				log.Errorf("[Database] checkWarns reload: %v", err)
				warnrc = defaultWarnSrc
			}
		}
	} else if err != nil {
		log.Errorf("[Database][checkUserWarns]: %d - %v", userId, err)
		warnrc = defaultWarnSrc
	}
	return
}

// ErrWarnExpiryOutOfRange is SetWarnExpiryContext's refusal of a value outside 0 to
// models.WarnExpiryMaxDays; nothing is written.
var ErrWarnExpiryOutOfRange = errors.New("warn expiry out of range")

// WarnUser records one warn dated now. It is WarnUserAt on time.Now(), kept for its callers.
func WarnUser(userId, chatId int64, reason string) (int, []string, error) {
	return WarnUserAt(context.Background(), userId, chatId, reason, time.Now())
}

// warnRowAt dates the member's row for a write at now: a row with no update time (never stored)
// is dated now, then the times are normalised and, while expiry is on, the warns that no longer
// count are dropped, so num_warns, the reasons and the times all keep the counted suffix. It reports
// how many warns counted before the write.
func warnRowAt(warnrc *models.Warns, expiryDays int, now time.Time) int {
	if warnrc.UpdatedAt.IsZero() {
		warnrc.UpdatedAt = now
	}
	count, reasons, times := warnrc.Counted(expiryDays, now)
	if expiryDays > 0 {
		warnrc.NumWarns = count
		warnrc.Reasons = models.StringArray(reasons)
	}
	warnrc.WarnTimes = models.Int64Array(times)
	return count
}

// warnReason is the stored reason of a warn: the reason cut to 3000 bytes of valid UTF-8, or the
// translated "no reason" text.
func warnReason(reason string) string {
	if reason != "" {
		if len(reason) > 3000 {
			reason = reason[:3000]
			for !utf8.ValidString(reason) {
				reason = reason[:len(reason)-1]
			}
		}
		return reason
	}
	tr := i18n.MustNewTranslator("uk")
	noReason, _ := tr.GetString("db_warn_no_reason")
	if noReason == "" {
		noReason = "Без причини"
	}
	return noReason
}

// lockWarnChat takes the row lock on the chat that serializes warn writes of one chat.
func lockWarnChat(tx *gorm.DB, chatId int64) error {
	// ponytail: this serializes warnings per chat; use per-user advisory
	// locks only if moderation write throughput becomes material.
	var chat models.Chat
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id").
		Where("chat_id = ?", chatId).
		Take(&chat).Error
}

// lockMemberWarns reads the member's row under a row lock; found is false when the member has none.
func lockMemberWarns(tx *gorm.DB, userId, chatId int64) (warnrc *models.Warns, found bool, err error) {
	warnrc = &models.Warns{}
	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND chat_id = ?", userId, chatId).
		First(warnrc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &models.Warns{UserId: userId, ChatId: chatId, Reasons: models.StringArray{}}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return warnrc, true, nil
}

// readWarnExpiry reads the chat's warn expiry in the transaction; a chat with no settings row has
// expiry off.
func readWarnExpiry(tx *gorm.DB, chatId int64) (int, error) {
	var row models.WarnSettings
	err := tx.Where("chat_id = ?", chatId).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return row.ExpiryDays, nil
}

// WarnUserAt records one warn dated now and returns the member's counted warns and their reasons,
// so the warn limit compares what counts. In one transaction, under the chat's and the member's row
// locks, it reads the chat's settings (creating the default row when there is none), dates the
// member's existing warns (NormalisedTimes), drops the ones that no longer count while expiry is on,
// then appends this warn and its time. With expiry off nothing is ever dropped and every warn counts.
func WarnUserAt(ctx context.Context, userId, chatId int64, reason string, now time.Time) (int, []string, error) {
	var numWarns int
	var reasons []string

	if err := chats.EnsureChatInDb(chatId, ""); err != nil {
		return 0, nil, err
	}
	if err := user.EnsureUserInDb(userId, "", ""); err != nil {
		return 0, nil, err
	}

	err := db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockWarnChat(tx, chatId); err != nil {
			return err
		}

		warnSettings := &models.WarnSettings{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("chat_id = ?", chatId).
			First(warnSettings).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				warnSettings = &models.WarnSettings{ChatId: chatId, WarnLimit: 3, WarnMode: "mute"}
				if err := tx.Create(warnSettings).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		}

		warnrc, _, err := lockMemberWarns(tx, userId, chatId)
		if err != nil {
			return err
		}
		warnRowAt(warnrc, warnSettings.ExpiryDays, now)

		warnrc.NumWarns++
		warnrc.Reasons = append(warnrc.Reasons, warnReason(reason))
		warnrc.WarnTimes = append(warnrc.WarnTimes, now.Unix())

		if err := tx.Save(warnrc).Error; err != nil {
			return err
		}

		numWarns = warnrc.NumWarns
		reasons = []string(warnrc.Reasons)
		return nil
	})
	if err != nil {
		log.Errorf("[Database] WarnUser: %v", err)
		return 0, nil, err
	}

	cache.DeleteCache(cache.CacheKey("warns", userId, chatId))
	cache.DeleteCache(cache.CacheKey("warn_settings", chatId))

	return numWarns, reasons, nil
}

// RemoveWarn removes the member's newest warn. It is RemoveWarnAt on time.Now(), kept for its
// callers.
func RemoveWarn(userId, chatId int64) (bool, error) {
	return RemoveWarnAt(context.Background(), userId, chatId, time.Now())
}

// RemoveWarnAt removes the member's newest counted warn and answers removed only when one counted.
// With expiry off it answers as before: removed while num_warns is above 0. With expiry on the warns
// that no longer count are dropped first, in the same transaction, so a member whose warns have all
// expired is answered false and keeps no expired warn.
func RemoveWarnAt(ctx context.Context, userId, chatId int64, now time.Time) (bool, error) {
	var removed, changed bool

	err := db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockWarnChat(tx, chatId); err != nil {
			return err
		}
		expiryDays, err := readWarnExpiry(tx, chatId)
		if err != nil {
			return err
		}
		warnrc, found, err := lockMemberWarns(tx, userId, chatId)
		if err != nil || !found {
			return err
		}

		stored := warnrc.NumWarns
		warnRowAt(warnrc, expiryDays, now)
		changed = warnrc.NumWarns != stored
		if warnrc.NumWarns > 0 {
			warnrc.NumWarns--
			if len(warnrc.Reasons) > 0 {
				warnrc.Reasons = warnrc.Reasons[:len(warnrc.Reasons)-1]
			}
			warnrc.WarnTimes = warnrc.WarnTimes[:len(warnrc.WarnTimes)-1]
			removed = true
		}
		if !removed && !changed {
			return nil
		}
		return tx.Save(warnrc).Error
	})
	if err != nil {
		log.Errorf("[Database] RemoveWarn: %v", err)
		return false, err
	}

	if removed || changed {
		cache.DeleteCache(cache.CacheKey("warns", userId, chatId))
		cache.DeleteCache(cache.CacheKey("warn_settings", chatId))
	}

	return removed, nil
}

// ResetUserWarns deletes the member's warns. It is ResetUserWarnsAt on time.Now(), kept for its
// callers.
func ResetUserWarns(userId, chatId int64) (bool, error) {
	return ResetUserWarnsAt(context.Background(), userId, chatId, time.Now())
}

// ResetUserWarnsAt deletes the member's warn row. With expiry off it answers removed when a row was
// deleted, as before (also a row with no warn left); with expiry on it answers removed only when a
// warn counted, and a row holding only expired warns is deleted all the same.
func ResetUserWarnsAt(ctx context.Context, userId, chatId int64, now time.Time) (bool, error) {
	var removed, deleted bool
	err := db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		expiryDays, err := readWarnExpiry(tx, chatId)
		if err != nil {
			return err
		}
		counted := -1
		if expiryDays > 0 {
			warnrc, found, err := lockMemberWarns(tx, userId, chatId)
			if err != nil || !found {
				return err
			}
			counted, _, _ = warnrc.Counted(expiryDays, now)
		}
		result := tx.Where("user_id = ? AND chat_id = ?", userId, chatId).Delete(&models.Warns{})
		if result.Error != nil {
			return result.Error
		}
		deleted = result.RowsAffected > 0
		removed = deleted && counted != 0
		return nil
	})
	if err != nil {
		log.Errorf("[Database] ResetUserWarns: %v", err)
		return false, err
	}
	if deleted {
		cache.DeleteCache(cache.CacheKey("warns", userId, chatId))
		cache.DeleteCache(cache.CacheKey("warn_settings", chatId))
	}
	return removed, nil
}

func GetWarns(userId, chatId int64) (int, []string) {
	return GetWarnsContext(context.Background(), userId, chatId)
}

// GetWarnsContext is GetWarnsAt on time.Now().
func GetWarnsContext(ctx context.Context, userId, chatId int64) (int, []string) {
	return GetWarnsAt(ctx, userId, chatId, time.Now())
}

// warnCache is the member's warn row as the warns:<user>:<chat> key holds it: the stored warns,
// their times and the row's update time in unix seconds. A value cached by an earlier build has no
// times and an update time of 0, so its warns all count (NormalisedTimes).
type warnCache struct {
	NumWarns  int
	Reasons   []string
	Times     []int64
	UpdatedAt int64
}

// GetWarnsAt returns the member's counted warns and their reasons at now, counted with the chat's
// warn expiry read through LoadWarnSettingsContext (which creates no row); a settings read that
// fails counts with expiry off, so a display never counts fewer warns on a guess.
func GetWarnsAt(ctx context.Context, userId, chatId int64, now time.Time) (int, []string) {
	cached, err := cache.GetFromCacheOrLoad(ctx,
		cache.CacheKey("warns", userId, chatId),
		cache.CacheTTLWarnSettings,
		func(ctx context.Context) (warnCache, error) {
			w := checkWarnsContext(ctx, userId, chatId)
			value := warnCache{NumWarns: w.NumWarns, Reasons: []string(w.Reasons), Times: []int64(w.WarnTimes)}
			if !w.UpdatedAt.IsZero() {
				value.UpdatedAt = w.UpdatedAt.Unix()
			}
			return value, nil
		},
	)
	if err != nil {
		return 0, nil
	}
	expiryDays := 0
	if settings, err := LoadWarnSettingsContext(ctx, chatId); err == nil {
		expiryDays = settings.ExpiryDays
	}
	row := models.Warns{NumWarns: cached.NumWarns, Reasons: models.StringArray(cached.Reasons), WarnTimes: models.Int64Array(cached.Times)}
	if cached.UpdatedAt != 0 {
		row.UpdatedAt = time.Unix(cached.UpdatedAt, 0)
	}
	count, reasons, _ := row.Counted(expiryDays, now)
	return count, reasons
}

// SetWarnExpiryTx stores the chat's warn expiry in days (0 is off) inside caller-supplied
// transaction tx. The caller is responsible for dropping warn_settings:<chat> after commit.
func SetWarnExpiryTx(tx *gorm.DB, chatId int64, days int) error {
	if days < 0 || days > models.WarnExpiryMaxDays {
		return ErrWarnExpiryOutOfRange
	}
	if tx == nil {
		return errors.New("database not initialized")
	}
	err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"warn_expiry_days"}),
	}).Create(&models.WarnSettings{ChatId: chatId, WarnLimit: 3, WarnMode: "mute", ExpiryDays: days}).Error
	if err != nil {
		log.Errorf("[Database] SetWarnExpiry: %v", err)
		return err
	}
	return nil
}

// SetWarnExpiryContext stores the chat's warn expiry in days (0 is off) in one column-targeted
// upsert of warn_expiry_days: a chat with no row gets one with limit 3 and mode mute, and an existing
// row has only the expiry updated, so a concurrent limit or mode write is never clobbered. A value
// outside 0 to models.WarnExpiryMaxDays is ErrWarnExpiryOutOfRange before any write. It drops
// warn_settings:<chat>.
func SetWarnExpiryContext(ctx context.Context, chatId int64, days int) error {
	if days < 0 || days > models.WarnExpiryMaxDays {
		return ErrWarnExpiryOutOfRange
	}
	if err := SetWarnExpiryTx(db.DB.WithContext(ctx), chatId, days); err != nil {
		return err
	}
	cache.DeleteCache(cache.CacheKey("warn_settings", chatId))
	return nil
}

func SetWarnLimit(chatId int64, warnLimit int) error {
	// Single-statement upsert: no read-modify-write, concurrent limit/mode sets can't clobber.
	err := db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"warn_limit"}),
	}).Create(&models.WarnSettings{ChatId: chatId, WarnLimit: warnLimit, WarnMode: "mute"}).Error
	if err != nil {
		log.Errorf("[Database] SetWarnLimit: %v", err)
		return err
	}
	cache.DeleteCache(cache.CacheKey("warn_settings", chatId))
	return nil
}

// SetWarnMode stores the action at the warn limit with no duration (warn_mode_secs 0). It is
// SetWarnModeContext with seconds 0, kept for its callers.
func SetWarnMode(chatId int64, warnMode string) error {
	return SetWarnModeContext(context.Background(), chatId, warnMode, 0)
}

// SetWarnModeTx stores the action at the warn limit and its duration in seconds inside
// caller-supplied transaction tx. The caller is responsible for dropping warn_settings:<chat>
// after commit.
func SetWarnModeTx(tx *gorm.DB, chatID int64, mode string, secs int) error {
	if tx == nil {
		return errors.New("database not initialized")
	}
	err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"warn_mode", "warn_mode_secs"}),
	}).Create(&models.WarnSettings{ChatId: chatID, WarnLimit: 3, WarnMode: mode, WarnModeSecs: secs}).Error
	if err != nil {
		log.Errorf("[Database] SetWarnMode: %v", err)
		return err
	}
	return nil
}

// SetWarnModeContext stores the action at the warn limit and its duration in seconds (0 for a
// plain action) in one single-statement upsert: a chat with no row gets one with limit 3, and an
// existing row has only warn_mode and warn_mode_secs updated, so a concurrent limit write is never
// clobbered (see SetWarnLimit). It validates nothing (the caller does) and drops warn_settings:<chat>.
func SetWarnModeContext(ctx context.Context, chatID int64, mode string, secs int) error {
	if err := SetWarnModeTx(db.DB.WithContext(ctx), chatID, mode, secs); err != nil {
		return err
	}
	cache.DeleteCache(cache.CacheKey("warn_settings", chatID))
	return nil
}

func GetWarnSetting(chatId int64) *models.WarnSettings {
	return GetWarnSettingContext(context.Background(), chatId)
}

// GetWarnSettingContext is GetWarnSettingErrContext for display paths: a read that fails
// answers the defaults (limit 3, mode mute), which are not cached.
func GetWarnSettingContext(ctx context.Context, chatId int64) *models.WarnSettings {
	settings, err := GetWarnSettingErrContext(ctx, chatId)
	if err != nil {
		return &models.WarnSettings{ChatId: chatId, WarnLimit: 3, WarnMode: "mute"}
	}
	return settings
}

// GetWarnSettingErrContext reads the chat's warn settings through the warn_settings cache key
// and returns a failed read as an error: nothing is cached for it, so the next read asks the
// database again. A chat with no row gets the default row created, as before.
func GetWarnSettingErrContext(ctx context.Context, chatId int64) (*models.WarnSettings, error) {
	cached, err := cache.GetFromCacheOrLoad(ctx,
		cache.CacheKey("warn_settings", chatId),
		cache.CacheTTLWarnSettings,
		func(ctx context.Context) (models.WarnSettings, error) {
			w, err := checkWarnSettingsContext(ctx, chatId)
			if err != nil {
				return models.WarnSettings{}, err
			}
			return *w, nil
		},
	)
	if err != nil {
		return nil, err
	}
	return &cached, nil
}

// LoadWarnSettingsContext reads the chat's warn settings through the same cache key and TTL
// as GetWarnSettingContext, but returns the read error instead of hiding it and never inserts
// a row. A chat with no row reads the defaults (limit 3, mode mute) and that default is not
// cached, so the command loader's next read still finds its miss and behaves as before. A row
// that exists is cached exactly as the command's loader would cache it.
func LoadWarnSettingsContext(ctx context.Context, chatId int64) (*models.WarnSettings, error) {
	cached, err := cache.GetFromCacheOrLoad(ctx,
		cache.CacheKey("warn_settings", chatId),
		cache.CacheTTLWarnSettings,
		func(ctx context.Context) (models.WarnSettings, error) {
			var row models.WarnSettings
			if err := db.DB.WithContext(ctx).Where("chat_id = ?", chatId).First(&row).Error; err != nil {
				return models.WarnSettings{}, err
			}
			return row, nil
		},
	)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &models.WarnSettings{ChatId: chatId, WarnLimit: 3, WarnMode: "mute"}, nil
	}
	if err != nil {
		return nil, err
	}
	return &cached, nil
}

func GetAllChatWarns(chatId int64) int {
	var count int64
	err := db.DB.Model(&models.Warns{}).Where("chat_id = ?", chatId).Count(&count).Error
	if err != nil {
		log.Errorf("[Database] GetAllChatWarns: %v", err)
		return 0
	}
	return int(count)
}

func ResetAllChatWarns(chatId int64) error {
	var userIds []int64
	if err := db.DB.Model(&models.Warns{}).Where("chat_id = ?", chatId).Pluck("user_id", &userIds).Error; err != nil {
		log.Errorf("[Database] ResetAllChatWarns: %v", err)
		return err
	}

	err := db.DB.Where("chat_id = ?", chatId).Delete(&models.Warns{}).Error
	if err != nil {
		log.Errorf("[Database] ResetAllChatWarns: %v", err)
		return err
	}
	for _, userId := range userIds {
		cache.DeleteCache(cache.CacheKey("warns", userId, chatId))
	}
	cache.DeleteCache(cache.CacheKey("warn_settings", chatId))
	return nil
}

func LoadWarnsStats() (warnedUsers, warnChats int64) {
	warnedUsers = db.CountRows(&models.Warns{}, "")
	warnChats = db.CountDistinct(&models.Warns{}, "chat_id", "")
	return
}
