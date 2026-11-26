package modules

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/db/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/ratelimit"
)

const (
	// presenceAvailReactTTL is how long a chat's AvailableReactions list stays cached under xenobot:cache:.
	presenceAvailReactTTL = 20 * time.Minute
	// presenceBotReactTTL is how long bot_react metadata lives outside xenobot:cache: for thumbs-down retraction.
	presenceBotReactTTL = 24 * time.Hour
	// presenceRetractUserCooldown is the extended per-user cooldown applied after a thumbs-down retraction.
	presenceRetractUserCooldown = 24 * time.Hour
	// presenceReactCallTimeout bounds one Telegram request on the reaction path.
	presenceReactCallTimeout = 5 * time.Second
)

// presenceReactionPacer paces outgoing reaction API calls on dedicated Redis keys isolated from
// chatPacer, so reaction bursts never delay /ask or conversation replies. Swappable in tests.
var presenceReactionPacer = ratelimit.NewTelegramPacer(ratelimit.TelegramPacerOptions{
	NextKey:    "xenobot:presence:pace:next",
	BlockKey:   "xenobot:presence:pace:block",
	Interval:   100 * time.Millisecond,
	MaxRetries: 1,
	MaxWait:    2 * time.Second,
})

// presencePaced runs one Telegram call under presenceReactionPacer, bounded by presenceReactCallTimeout.
func presencePaced(ctx context.Context, call func(context.Context) error) error {
	return presenceReactionPacer.Do(ctx, func(callCtx context.Context) error {
		bounded, cancel := context.WithTimeout(callCtx, presenceReactCallTimeout)
		defer cancel()
		return call(bounded)
	})
}

// presenceStandardEmojis extracts standard emoji reactions from list, normalizes each (stripping
// U+FE0F), filters against supportedReactionEmoji, and deduplicates while preserving order.
func presenceStandardEmojis(list []gotgbot.ReactionType) []string {
	if len(list) == 0 {
		return nil
	}
	out := make([]string, 0, len(list))
	seen := make(map[string]struct{}, len(list))
	for _, rt := range list {
		if rt == nil {
			continue
		}
		merged := rt.MergeReactionType()
		if merged.Type != "emoji" || merged.Emoji == "" {
			continue
		}
		emoji := normaliseReactionEmoji(merged.Emoji)
		if emoji == "" || !slices.Contains(supportedReactionEmoji, emoji) {
			continue
		}
		if _, dup := seen[emoji]; dup {
			continue
		}
		seen[emoji] = struct{}{}
		out = append(out, emoji)
	}
	return out
}

// presenceAvailReactKey returns the cache key (inside xenobot:cache:) for a chat's AvailableReactions.
func presenceAvailReactKey(chatID int64) string {
	return cache.CacheKey("avail_react", chatID)
}

// presenceAvailReactions is the msgpack-cached AvailableReactions summary of one chat.
type presenceAvailReactions struct {
	All     bool     `msgpack:"a"`
	Allowed []string `msgpack:"e"`
}

// presenceAllowedEmoji reports whether normalized emoji is allowed in chatID by intersecting
// supportedReactionEmoji with the chat's cached AvailableReactions (20m TTL). An empty/nil
// AvailableReactions means all 73 standard emojis are allowed; a custom-only list rejects every
// standard emoji. On getChat errors it fails closed (false) and logs error class at Debug.
func presenceAllowedEmoji(ctx context.Context, b *gotgbot.Bot, chatID int64, emoji string) bool {
	if b == nil || emoji == "" {
		return false
	}
	avail, err := cache.GetFromCacheOrLoad(ctx, presenceAvailReactKey(chatID), presenceAvailReactTTL, func(loadCtx context.Context) (presenceAvailReactions, error) {
		var info *gotgbot.ChatFullInfo
		callErr := presencePaced(loadCtx, func(callCtx context.Context) error {
			var err error
			info, err = b.GetChatWithContext(callCtx, chatID, nil)
			return err
		})
		if callErr != nil {
			return presenceAvailReactions{}, callErr
		}
		if info == nil || len(info.AvailableReactions) == 0 {
			return presenceAvailReactions{All: true}, nil
		}
		return presenceAvailReactions{
			All:     false,
			Allowed: presenceStandardEmojis(info.AvailableReactions),
		}, nil
	})
	if err != nil {
		log.WithField("chat_id", chatID).Debugf("[Presence] getChat for available reactions failed: %s", chatSendErrClass(err))
		return false
	}
	if avail.All {
		return true
	}
	return slices.Contains(avail.Allowed, emoji)
}

// presenceBotReact is the compact 24h metadata stored outside xenobot:cache: whenever the bot sets
// a reaction so thumbs-down retraction can authorize the message author and clear the reaction.
type presenceBotReact struct {
	Emoji    string `json:"e"`
	AuthorID int64  `json:"a"`
	Source   string `json:"s"`
}

// presenceWriteBotReact stores 24h bot_react metadata in Redis (best effort).
func presenceWriteBotReact(ctx context.Context, client *redis.Client, chatID, msgID int64, rec presenceBotReact) {
	if client == nil {
		return
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		return
	}
	wCtx, cancel := context.WithTimeout(ctx, chatRedisTimeout)
	defer cancel()
	if err := client.Set(wCtx, presenceBotReactKey(chatID, msgID), raw, presenceBotReactTTL).Err(); err != nil {
		log.WithField("chat_id", chatID).Debug("[Presence] writing bot_react metadata failed")
	}
}

// presenceReadBotReact reads and decodes bot_react metadata from Redis.
func presenceReadBotReact(ctx context.Context, client *redis.Client, chatID, msgID int64) (presenceBotReact, bool) {
	if client == nil {
		return presenceBotReact{}, false
	}
	rCtx, cancel := context.WithTimeout(ctx, chatRedisTimeout)
	defer cancel()
	raw, err := client.Get(rCtx, presenceBotReactKey(chatID, msgID)).Bytes()
	if err != nil {
		return presenceBotReact{}, false
	}
	var rec presenceBotReact
	if err := json.Unmarshal(raw, &rec); err != nil {
		return presenceBotReact{}, false
	}
	return rec, true
}

// presenceClaimBotReact atomically deletes the bot_react key and returns true only for the single
// caller that won the delete, preventing duplicate clear calls across concurrent replicas.
func presenceClaimBotReact(ctx context.Context, client *redis.Client, chatID, msgID int64) bool {
	if client == nil {
		return false
	}
	dCtx, cancel := context.WithTimeout(ctx, chatRedisTimeout)
	defer cancel()
	deleted, err := client.Del(dCtx, presenceBotReactKey(chatID, msgID)).Result()
	return err == nil && deleted > 0
}

// presenceExtendUserCooldown records an extended 24h per-user cooldown outside xenobot:cache:.
func presenceExtendUserCooldown(ctx context.Context, client *redis.Client, chatID, userID int64) {
	if client == nil || userID <= 0 {
		return
	}
	wCtx, cancel := context.WithTimeout(ctx, chatRedisTimeout)
	defer cancel()
	if err := client.Set(wCtx, presenceUserCooldownKey(chatID, userID), "1", presenceRetractUserCooldown).Err(); err != nil {
		log.WithFields(log.Fields{"chat_id": chatID, "user_id": userID}).Debug("[Presence] extending user cooldown failed")
	}
}

// presenceSetReaction normalizes rawEmoji (stripping U+FE0F), validates it against
// supportedReactionEmoji and the chat's cached AvailableReactions, sends IsBig: false through
// presenceReactionPacer, and records 24h bot_react metadata on success. Telegram 400 errors are
// shed silently at Debug (evicting avail_react on REACTION_INVALID) without touching chatBreakers.
func presenceSetReaction(ctx context.Context, b *gotgbot.Bot, chatID, msgID, authorID int64, rawEmoji, source string) bool {
	if b == nil {
		return false
	}
	emoji := normaliseReactionEmoji(rawEmoji)
	if emoji == "" || !slices.Contains(supportedReactionEmoji, emoji) {
		return false
	}
	if !presenceAllowedEmoji(ctx, b, chatID, emoji) {
		return false
	}
	err := presencePaced(ctx, func(callCtx context.Context) error {
		_, callErr := b.SetMessageReactionWithContext(callCtx, chatID, msgID, &gotgbot.SetMessageReactionOpts{
			Reaction: []gotgbot.ReactionType{gotgbot.ReactionTypeEmoji{Emoji: emoji}},
			IsBig:    false,
		})
		return callErr
	})
	if err != nil {
		if strings.Contains(err.Error(), "REACTION_INVALID") {
			cache.DeleteCache(presenceAvailReactKey(chatID))
		}
		log.WithField("chat_id", chatID).Debugf("[Presence] setMessageReaction failed: %s", chatSendErrClass(err))
		return false
	}
	if client := presenceRedisClient(); client != nil {
		presenceWriteBotReact(ctx, client, chatID, msgID, presenceBotReact{
			Emoji:    emoji,
			AuthorID: authorID,
			Source:   source,
		})
	}
	return true
}

// presenceClearReaction clears the bot's reaction on msgID by passing an explicit non-nil empty
// slice []gotgbot.ReactionType{} so gotgbot/v2 serializes "reaction":[] on the wire. Errors are
// logged at Debug by error class only and never trip chatBreakers.
func presenceClearReaction(ctx context.Context, b *gotgbot.Bot, chatID, msgID int64) bool {
	if b == nil {
		return false
	}
	err := presencePaced(ctx, func(callCtx context.Context) error {
		_, callErr := b.SetMessageReactionWithContext(callCtx, chatID, msgID, &gotgbot.SetMessageReactionOpts{
			Reaction: []gotgbot.ReactionType{},
		})
		return callErr
	})
	if err != nil {
		log.WithField("chat_id", chatID).Debugf("[Presence] clearing reaction failed: %s", chatSendErrClass(err))
		return false
	}
	return true
}
