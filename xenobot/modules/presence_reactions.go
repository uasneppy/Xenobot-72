package modules

import (
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/reaction"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/db/presence"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/chat_status"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/error_handling"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/helpers"
)

const (
	// presenceReactionTimeout bounds one MessageReactionUpdated execution derived from chatContext().
	presenceReactionTimeout = 3 * time.Second
	// presenceConsensusSetTTL is how long distinct-reactor emoji sets live outside xenobot:cache:.
	presenceConsensusSetTTL = 48 * time.Hour
	// presenceConsensusDoneTTL is how long the per-message once-only consensus latch lives.
	presenceConsensusDoneTTL = 48 * time.Hour
)

var (
	// presenceConsensusCooldown gates per-chat Emoji Consensus firing across replicas.
	// Package variable only so tests can shorten it.
	presenceConsensusCooldown = 5 * time.Minute

	// presenceSettingsGet reads a chat's skipLocal cached Interactive Presence settings.
	// Package variable only so tests can inject errors or fixtures.
	presenceSettingsGet = presence.Get

	// presenceIsAdmin checks whether userID is an administrator of chatID.
	// Package variable only so tests can count or stub live getChatMember calls.
	presenceIsAdmin = func(b *gotgbot.Bot, chatID, userID int64) bool {
		return chat_status.IsUserAdmin(b, chatID, userID)
	}
)

// presenceConsensusHostile lists normalized standard emojis that Emoji Consensus must never mirror.
var presenceConsensusHostile = map[string]struct{}{
	"👎": {},
	"🤬": {},
	"🤮": {},
	"💩": {},
	"🤡": {},
	"🖕": {},
	"😡": {},
	"💔": {},
	"😢": {},
	"😭": {},
}

// presenceConsensusUpdateScript atomically updates one emoji's distinct-reactor SET
// (SADD on "add", SREM otherwise), sets/renews its expiry, and returns SCARD.
var presenceConsensusUpdateScript = redis.NewScript(`
if ARGV[1] == 'add' then
	redis.call('SADD', KEYS[1], ARGV[2])
else
	redis.call('SREM', KEYS[1], ARGV[2])
end
local ttl = tonumber(ARGV[3])
if ttl and ttl > 0 then
	redis.call('EXPIRE', KEYS[1], ttl)
end
return redis.call('SCARD', KEYS[1])
`)

// presenceConsensusClaimScript atomically claims both the per-chat cooldown (KEYS[1]) and the
// per-message once-only latch (KEYS[2]) in one round trip so a refusal on either never burns the
// other. Both keys carry the {<chatID>} hash tag for Redis Cluster slot compatibility.
var presenceConsensusClaimScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 1 or redis.call('EXISTS', KEYS[2]) == 1 then
	return 0
end
redis.call('SET', KEYS[1], '1', 'EX', ARGV[1])
redis.call('SET', KEYS[2], '1', 'EX', ARGV[2])
return 1
`)

// LoadPresence registers the MessageReactionUpdated router at handler group 0, the /setarchive
// and /unsetarchive commands, and the group-11 forward capture watcher.
func LoadPresence(dispatcher *ext.Dispatcher) {
	dispatcher.AddHandler(handlers.NewReaction(reaction.All, presenceModule.handleReaction))
	dispatcher.AddHandler(helpers.NewBotCommand("setarchive", presenceModule.setArchive).SetAllowChannel(true))
	dispatcher.AddHandler(helpers.NewBotCommand("unsetarchive", presenceModule.unsetArchive))
	dispatcher.AddHandlerToGroup(
		handlers.NewMessage(message.All, presenceModule.captureSetArchiveForward),
		logHandlerGroup,
	)
	SetModuleEnabled(presenceModule.moduleName, true)
	log.Info("[Modules] Presence module loaded")
}

func init() {
	RegisterLegacyModule("Presence", 252, LoadPresence)
}

// presenceDiffReactions computes deterministic standard-emoji set differences added = new \ old
// and removed = old \ new, stripping U+FE0F and ignoring custom or unsupported reactions.
func presenceDiffReactions(oldList, newList []gotgbot.ReactionType) (added, removed []string) {
	oldEmojis := presenceStandardEmojis(oldList)
	newEmojis := presenceStandardEmojis(newList)
	if len(oldEmojis) == 0 && len(newEmojis) == 0 {
		return nil, nil
	}

	oldSet := make(map[string]struct{}, len(oldEmojis))
	for _, emoji := range oldEmojis {
		oldSet[emoji] = struct{}{}
	}
	newSet := make(map[string]struct{}, len(newEmojis))
	for _, emoji := range newEmojis {
		newSet[emoji] = struct{}{}
		if _, existed := oldSet[emoji]; !existed {
			added = append(added, emoji)
		}
	}
	for _, emoji := range oldEmojis {
		if _, still := newSet[emoji]; !still {
			removed = append(removed, emoji)
		}
	}
	return added, removed
}

// presenceLogAuthorOf resolves msgID's author from the chat's Redis message log (best effort).
// Returns 0 when the log is absent, Redis fails, or the entry cannot be decoded.
func presenceLogAuthorOf(ctx context.Context, client *redis.Client, chatID, msgID int64) int64 {
	if client == nil || msgID <= 0 {
		return 0
	}
	rCtx, cancel := context.WithTimeout(ctx, chatRedisTimeout)
	defer cancel()
	id := strconv.FormatInt(msgID, 10)
	raw, err := client.ZRangeArgs(rCtx, redis.ZRangeArgs{
		Key:     chatLogKey(chatID),
		Start:   id,
		Stop:    id,
		ByScore: true,
	}).Result()
	if err != nil || len(raw) == 0 {
		return 0
	}
	var entry chatLogEntry
	if err := json.Unmarshal([]byte(raw[0]), &entry); err != nil {
		return 0
	}
	return entry.UserID
}

// handleReaction is the group-0 MessageReactionUpdated handler. Every execution joins StopChat
// (chatJobs + chatDraining + chatContext()) so SIGTERM cancels in-flight calls without adding a
// shutdown drain.
func (moduleStruct) handleReaction(b *gotgbot.Bot, ctx *ext.Context) (result error) {
	result = ext.EndGroups
	defer error_handling.RecoverFromPanic("handleReaction", "Presence")

	chatJobs.start()
	if chatDraining.Load() {
		chatJobs.finish()
		return ext.EndGroups
	}
	defer chatJobs.finish()

	runCtx, cancel := context.WithTimeout(chatContext(), presenceReactionTimeout)
	defer cancel()

	presenceHandleReaction(runCtx, b, ctx)
	return ext.EndGroups
}

func presenceHandleReaction(runCtx context.Context, b *gotgbot.Bot, ctx *ext.Context) {
	if b == nil || ctx == nil {
		return
	}
	mru := ctx.MessageReaction
	if mru == nil || !presenceChatTypeAllowed(&mru.Chat) {
		return
	}
	if mru.User != nil && mru.User.IsBot {
		return
	}
	added, removed := presenceDiffReactions(mru.OldReaction, mru.NewReaction)
	if len(added) == 0 && len(removed) == 0 {
		return
	}
	if presenceIsStaffGroup(mru.Chat.Id) {
		return
	}
	settings, err := presenceSettingsGet(runCtx, mru.Chat.Id)
	if err != nil {
		log.WithField("chat_id", mru.Chat.Id).Debug("[Presence] reading presence settings failed")
		return
	}
	if settings.RetractEnabled && slices.Contains(added, "👎") {
		presenceHandleRetract(runCtx, b, mru)
	}
	if settings.ConsensusEnabled && mru.User != nil {
		presenceHandleConsensus(runCtx, b, mru, settings.ConsensusThreshold, added, removed)
	}
	if settings.ArchiveEnabled && settings.ArchiveChatID != 0 && mru.User != nil {
		presenceHandleArchive(runCtx, b, mru, settings)
	}
}

// presenceHandleRetract handles thumbs-down retraction (REACT-05): checks Redis bot_react before
// any Telegram API call, authorizes only the message author, an anonymous group admin, or a group
// admin, atomically claims bot_react, clears the bot's reaction silently, extends the user's
// cooldown (when non-anonymous), and posts zero group messages.
func presenceHandleRetract(ctx context.Context, b *gotgbot.Bot, mru *gotgbot.MessageReactionUpdated) {
	client := presenceRedisClient()
	if client == nil {
		return
	}
	chatID, msgID := mru.Chat.Id, mru.MessageId
	rec, ok := presenceReadBotReact(ctx, client, chatID, msgID)
	if !ok {
		return
	}

	authorized := false
	switch {
	case presenceIsAnonGroupAdmin(mru):
		authorized = true
	case mru.User == nil:
		authorized = false
	case rec.AuthorID != 0 && mru.User.Id == rec.AuthorID:
		authorized = true
	default:
		authorized = presenceIsAdmin(b, chatID, mru.User.Id)
	}
	if !authorized {
		return
	}

	if !presenceClaimBotReact(ctx, client, chatID, msgID) {
		return
	}
	presenceClearReaction(ctx, b, chatID, msgID)
	if mru.User != nil && ctx.Err() == nil {
		presenceExtendUserCooldown(ctx, client, chatID, mru.User.Id)
	}
}

// presenceHandleConsensus tracks distinct non-anonymous real member reactions per non-hostile
// standard emoji (REACT-04). When any added emoji reaches threshold, it lazily verifies active
// lockdown status (fail-closed), atomically claims the per-chat cooldown and per-message latch,
// resolves the message author from chat_log (best effort), and mirrors the emoji once.
func presenceHandleConsensus(ctx context.Context, b *gotgbot.Bot, mru *gotgbot.MessageReactionUpdated, threshold int, added, removed []string) {
	if mru.User == nil {
		return
	}
	client := presenceRedisClient()
	if client == nil {
		return
	}
	if !presence.ValidConsensusThreshold(threshold) {
		threshold = presence.DefaultConsensusThreshold
	}

	chatID, msgID, userID := mru.Chat.Id, mru.MessageId, mru.User.Id
	userArg := strconv.FormatInt(userID, 10)
	setTTLSecs := max(int(presenceConsensusSetTTL.Seconds()), 1)

	for _, emoji := range removed {
		if _, hostile := presenceConsensusHostile[emoji]; hostile {
			continue
		}
		if ctx.Err() != nil {
			return
		}
		if _, err := presenceConsensusUpdate(ctx, client, chatID, msgID, emoji, "rem", userArg, setTTLSecs); err != nil {
			log.WithField("chat_id", chatID).Debugf("[Presence] consensus set remove failed: %s", chatRedisErrClass(err))
		}
	}

	var triggerEmoji string
	for _, emoji := range added {
		if _, hostile := presenceConsensusHostile[emoji]; hostile {
			continue
		}
		if ctx.Err() != nil {
			return
		}
		count, err := presenceConsensusUpdate(ctx, client, chatID, msgID, emoji, "add", userArg, setTTLSecs)
		if err != nil {
			log.WithField("chat_id", chatID).Debugf("[Presence] consensus set add failed: %s", chatRedisErrClass(err))
			continue
		}
		if triggerEmoji == "" && count >= int64(threshold) {
			triggerEmoji = emoji
		}
	}
	if triggerEmoji == "" {
		return
	}

	if presenceIsLockedDown(chatID) {
		return
	}
	if !presenceConsensusClaim(ctx, client, chatID, msgID) {
		return
	}
	authorID := presenceLogAuthorOf(ctx, client, chatID, msgID)
	presenceSetReaction(ctx, b, chatID, msgID, authorID, triggerEmoji, "cons")
}

func presenceConsensusUpdate(ctx context.Context, client *redis.Client, chatID, msgID int64, emoji, op, userArg string, ttlSecs int) (int64, error) {
	rCtx, cancel := context.WithTimeout(ctx, chatRedisTimeout)
	defer cancel()
	return presenceConsensusUpdateScript.Run(
		rCtx,
		client,
		[]string{presenceConsensusUsersKey(chatID, msgID, emoji)},
		op,
		userArg,
		ttlSecs,
	).Int64()
}

func presenceConsensusClaim(ctx context.Context, client *redis.Client, chatID, msgID int64) bool {
	cdSecs := max(int(presenceConsensusCooldown.Seconds()), 1)
	doneSecs := max(int(presenceConsensusDoneTTL.Seconds()), 1)
	rCtx, cancel := context.WithTimeout(ctx, chatRedisTimeout)
	defer cancel()
	claimed, err := presenceConsensusClaimScript.Run(
		rCtx,
		client,
		[]string{presenceConsensusCooldownKey(chatID), presenceConsensusDoneKey(chatID, msgID)},
		cdSecs,
		doneSecs,
	).Int()
	if err != nil {
		log.WithField("chat_id", chatID).Debugf("[Presence] consensus claim failed: %s", chatRedisErrClass(err))
		return false
	}
	return claimed == 1
}
