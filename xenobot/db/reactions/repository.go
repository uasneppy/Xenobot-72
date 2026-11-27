package reactions

import (
	"context"
	"errors"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
)

const (
	// DefaultChancePct is the default trigger probability (100%) for static reaction rules.
	DefaultChancePct = 100
	// MinChancePct and MaxChancePct bound per-rule trigger chance percentages.
	MinChancePct = 1
	MaxChancePct = 100

	// MinCooldownSecs and MaxCooldownSecs bound per-rule cooldown seconds (0..24h).
	MinCooldownSecs = 0
	MaxCooldownSecs = 86400
)

// ErrInvalidOption is returned, before any DB write, when a rule option is out of bounds.
var ErrInvalidOption = errors.New("reactions: invalid option")

// Rule is the cached per-rule representation of one chat's static reaction configuration.
type Rule struct {
	ID           uint   `msgpack:"id"`
	Keyword      string `msgpack:"k"`
	Emoji        string `msgpack:"e"`
	WordBoundary bool   `msgpack:"wb"`
	MatchCaption bool   `msgpack:"mc"`
	ChancePct    int    `msgpack:"ch"`
	CooldownSecs int    `msgpack:"cd"`
}

// ValidChancePct reports whether n is within [MinChancePct, MaxChancePct].
func ValidChancePct(n int) bool {
	return n >= MinChancePct && n <= MaxChancePct
}

// ValidCooldownSecs reports whether n is within [MinCooldownSecs, MaxCooldownSecs].
func ValidCooldownSecs(n int) bool {
	return n >= MinCooldownSecs && n <= MaxCooldownSecs
}

func normalizeRule(r Rule) Rule {
	if r.ChancePct <= 0 || r.ChancePct > MaxChancePct {
		r.ChancePct = DefaultChancePct
	}
	if r.CooldownSecs < 0 || r.CooldownSecs > MaxCooldownSecs {
		r.CooldownSecs = 0
	}
	return r
}

func normalizeReactionRow(r *models.Reactions) {
	if r == nil {
		return
	}
	if r.ChancePct <= 0 || r.ChancePct > MaxChancePct {
		r.ChancePct = DefaultChancePct
	}
	if r.CooldownSecs < 0 || r.CooldownSecs > MaxCooldownSecs {
		r.CooldownSecs = 0
	}
}

func reactionsCacheKey(chatID int64) string {
	return cache.CacheKey("reactions", chatID)
}

func reactionRulesCacheKey(chatID int64) string {
	return cache.CacheKey("reaction_rules", chatID)
}

func invalidateReactionCaches(chatID int64) {
	cache.DeleteCache(reactionsCacheKey(chatID))
	cache.DeleteCache(reactionRulesCacheKey(chatID))
}

func GetReactions(chatID int64) map[string]string {
	return GetReactionsContext(context.Background(), chatID)
}

func GetReactionsContext(ctx context.Context, chatID int64) map[string]string {
	cacheKey := reactionsCacheKey(chatID)
	result, err := cache.GetFromCacheOrLoad(ctx, cacheKey, cache.CacheTTLReactions, func(ctx context.Context) (map[string]string, error) {
		var rows []*models.Reactions
		if err := db.GetRecordsContext(ctx, &rows, models.Reactions{ChatID: chatID}); err != nil {
			log.Errorf("[Database] GetReactions: %v - chat:%d", err, chatID)
			return map[string]string{}, err
		}
		out := make(map[string]string, len(rows))
		for _, r := range rows {
			out[r.Keyword] = r.Emoji
		}
		return out, nil
	})
	if err != nil || result == nil {
		return map[string]string{}
	}
	return result
}

// GetReactionRulesContext loads the chat's ordered static reaction rules through the skipLocal
// reaction_rules cache, normalizing legacy ChancePct==0 entries to 100.
func GetReactionRulesContext(ctx context.Context, chatID int64) ([]Rule, error) {
	rules, err := cache.GetFromCacheOrLoad(ctx, reactionRulesCacheKey(chatID), cache.CacheTTLReactions, func(ctx context.Context) ([]Rule, error) {
		var rows []models.Reactions
		err := db.DB.WithContext(ctx).
			Select("id", "keyword", "emoji", "word_boundary", "match_caption", "chance_pct", "cooldown_secs").
			Where("chat_id = ?", chatID).
			Order("id").
			Find(&rows).Error
		if err != nil {
			log.Errorf("[Database] GetReactionRulesContext: %v - chat:%d", err, chatID)
			return nil, err
		}
		out := make([]Rule, len(rows))
		for i, r := range rows {
			out[i] = normalizeRule(Rule{
				ID:           r.ID,
				Keyword:      r.Keyword,
				Emoji:        r.Emoji,
				WordBoundary: r.WordBoundary,
				MatchCaption: r.MatchCaption,
				ChancePct:    int(r.ChancePct),
				CooldownSecs: r.CooldownSecs,
			})
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	out := make([]Rule, len(rules))
	for i, r := range rules {
		out[i] = normalizeRule(r)
	}
	return out, nil
}

func AddReaction(chatID int64, keyword, emoji string) error {
	r := &models.Reactions{
		ChatID:    chatID,
		Keyword:   keyword,
		Emoji:     emoji,
		ChancePct: DefaultChancePct,
	}
	err := db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}, {Name: "keyword"}},
		DoUpdates: clause.AssignmentColumns([]string{"emoji", "updated_at"}),
	}).Create(r).Error
	if err != nil {
		log.Errorf("[Database] AddReaction: %v - chat:%d keyword:%s", err, chatID, keyword)
		return err
	}
	invalidateReactionCaches(chatID)
	return nil
}

func RemoveReaction(chatID int64, keyword string) error {
	result := db.DB.Where("chat_id = ? AND keyword = ?", chatID, keyword).Delete(&models.Reactions{})
	if result.Error != nil {
		log.Errorf("[Database] RemoveReaction: %v - chat:%d keyword:%s", result.Error, chatID, keyword)
		return result.Error
	}
	invalidateReactionCaches(chatID)
	return nil
}

func ResetReactions(chatID int64) error {
	if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.Reactions{}).Error; err != nil {
		log.Errorf("[Database] ResetReactions: %v - chat:%d", err, chatID)
		return err
	}
	invalidateReactionCaches(chatID)
	return nil
}

func LoadReactionsStats() (reactionsNum, reactionChats int64) {
	reactionsNum = db.CountRows(&models.Reactions{}, "")
	reactionChats = db.CountDistinct(&models.Reactions{}, "chat_id", "")
	return
}

// AddReactionContext stores one reaction. With overwrite it is the upsert /addreaction has always made, and
// added is true. Without it an existing keyword is left as it is (ON CONFLICT DO NOTHING) and added says
// whether the row was inserted. Both cache keys are dropped after any write that changed a row.
func AddReactionContext(ctx context.Context, chatID int64, keyword, emoji string, overwrite bool) (added bool, err error) {
	r := &models.Reactions{ChatID: chatID, Keyword: keyword, Emoji: emoji, ChancePct: DefaultChancePct}
	conflict := clause.OnConflict{Columns: []clause.Column{{Name: "chat_id"}, {Name: "keyword"}}, DoNothing: true}
	if overwrite {
		conflict = clause.OnConflict{
			Columns:   []clause.Column{{Name: "chat_id"}, {Name: "keyword"}},
			DoUpdates: clause.AssignmentColumns([]string{"emoji", "updated_at"}),
		}
	}
	result := db.DB.WithContext(ctx).Clauses(conflict).Create(r)
	if result.Error != nil {
		log.Errorf("[Database] AddReaction: %v - chat:%d", result.Error, chatID)
		return false, result.Error
	}
	added = overwrite || result.RowsAffected == 1
	if added {
		invalidateReactionCaches(chatID)
	}
	return added, nil
}

// ListReactionEntriesFresh lists the chat's reactions (ID, keyword, emoji, and options) by ID. It is
// fresh, chat-scoped and uncached, returns its read error and creates nothing.
func ListReactionEntriesFresh(ctx context.Context, chatID int64) ([]models.Reactions, error) {
	var rows []models.Reactions
	err := db.DB.WithContext(ctx).
		Select("id", "keyword", "emoji", "word_boundary", "match_caption", "chance_pct", "cooldown_secs").
		Where("chat_id = ?", chatID).
		Order("id").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for i := range rows {
		normalizeReactionRow(&rows[i])
	}
	return rows, nil
}

// GetReactionEntryFresh reads one reaction by its ID, only when it belongs to the chat: a missing ID and the ID
// of another chat's row both return nil and a nil error. It is fresh and creates nothing.
func GetReactionEntryFresh(ctx context.Context, chatID, id int64) (*models.Reactions, error) {
	var row models.Reactions
	err := db.DB.WithContext(ctx).Where("id = ? AND chat_id = ?", id, chatID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	normalizeReactionRow(&row)
	return &row, nil
}

// SetReactionOptionsByID updates a single rule's options scoped strictly to chatID and id.
// Out-of-bounds chancePct or cooldownSecs return ErrInvalidOption before any DB call; a missing or
// cross-chat ID returns (nil, nil).
func SetReactionOptionsByID(ctx context.Context, chatID, id int64, wordBoundary, matchCaption bool, chancePct, cooldownSecs int) (*models.Reactions, error) {
	if !ValidChancePct(chancePct) || !ValidCooldownSecs(cooldownSecs) {
		return nil, ErrInvalidOption
	}
	result := db.DB.WithContext(ctx).Model(&models.Reactions{}).
		Where("id = ? AND chat_id = ?", id, chatID).
		Updates(map[string]any{
			"word_boundary": wordBoundary,
			"match_caption": matchCaption,
			"chance_pct":    int16(chancePct),
			"cooldown_secs": cooldownSecs,
			"updated_at":    time.Now(),
		})
	if result.Error != nil {
		log.Errorf("[Database] SetReactionOptionsByID: %v - chat:%d id:%d", result.Error, chatID, id)
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	invalidateReactionCaches(chatID)
	return GetReactionEntryFresh(ctx, chatID, id)
}

// CountReactionsFresh counts the chat's reactions, fresh and with the error returned.
func CountReactionsFresh(ctx context.Context, chatID int64) (int64, error) {
	var n int64
	err := db.DB.WithContext(ctx).Model(&models.Reactions{}).Where("chat_id = ?", chatID).Count(&n).Error
	return n, err
}

// ReactionExistsFresh reports whether the chat has a reaction for keyword, fresh and with the error returned.
func ReactionExistsFresh(ctx context.Context, chatID int64, keyword string) (bool, error) {
	var n int64
	err := db.DB.WithContext(ctx).Model(&models.Reactions{}).Where("chat_id = ? AND keyword = ?", chatID, keyword).Count(&n).Error
	return n > 0, err
}

// RemoveReactionByID deletes one reaction by its ID, only when it belongs to the chat, in one conditional
// statement, and returns the row that went (nil when none did). Both cache keys are dropped only when a row went.
func RemoveReactionByID(ctx context.Context, chatID, id int64) (*models.Reactions, error) {
	var rows []models.Reactions
	result := db.DB.WithContext(ctx).Clauses(clause.Returning{}).Where("id = ? AND chat_id = ?", id, chatID).Delete(&rows)
	if result.Error != nil {
		log.Errorf("[Database] RemoveReactionByID: %v - chat:%d", result.Error, chatID)
		return nil, result.Error
	}
	if len(rows) != 1 {
		return nil, nil
	}
	invalidateReactionCaches(chatID)
	return &rows[0], nil
}
