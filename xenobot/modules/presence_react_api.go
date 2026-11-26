package modules

import (
	"context"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/redis/go-redis/v9"

	"github.com/uasneppy/Xenobot-72/xenobot/db/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/ratelimit"
)

const (
	presenceAvailReactTTL       = 20 * time.Minute
	presenceBotReactTTL         = 24 * time.Hour
	presenceRetractUserCooldown = 24 * time.Hour
	presenceReactCallTimeout    = 5 * time.Second
)

var presenceReactionPacer = ratelimit.NewTelegramPacer(ratelimit.TelegramPacerOptions{
	NextKey:    "xenobot:presence:pace:next",
	BlockKey:   "xenobot:presence:pace:block",
	Interval:   100 * time.Millisecond,
	MaxRetries: 1,
	MaxWait:    2 * time.Second,
})

func presenceAvailReactKey(chatID int64) string {
	return cache.CacheKey("avail_react", chatID)
}

type presenceAvailReactions struct {
	All     bool     `msgpack:"a"`
	Allowed []string `msgpack:"e"`
}

type presenceBotReact struct {
	Emoji    string `json:"e"`
	AuthorID int64  `json:"a"`
	Source   string `json:"s"`
}

func presenceStandardEmojis(list []gotgbot.ReactionType) []string { return nil }

func presenceAllowedEmoji(ctx context.Context, b *gotgbot.Bot, chatID int64, emoji string) bool {
	return false
}

func presenceWriteBotReact(ctx context.Context, client *redis.Client, chatID, msgID int64, rec presenceBotReact) {
}

func presenceReadBotReact(ctx context.Context, client *redis.Client, chatID, msgID int64) (presenceBotReact, bool) {
	return presenceBotReact{}, false
}

func presenceClaimBotReact(ctx context.Context, client *redis.Client, chatID, msgID int64) bool {
	return false
}

func presenceExtendUserCooldown(ctx context.Context, client *redis.Client, chatID, userID int64) {}

func presenceSetReaction(ctx context.Context, b *gotgbot.Bot, chatID, msgID, authorID int64, rawEmoji, source string) bool {
	return false
}

func presenceClearReaction(ctx context.Context, b *gotgbot.Bot, chatID, msgID int64) bool {
	return false
}
