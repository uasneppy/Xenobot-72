package modules

import (
	"context"
	"encoding/hex"
	"strconv"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/db/lockdown"
	"github.com/uasneppy/Xenobot-72/xenobot/db/presence"
	"github.com/uasneppy/Xenobot-72/xenobot/db/staff"
)

// presenceModule owns the Interactive Presence module state and handlers.
var presenceModule = moduleStruct{moduleName: "Presence"}

// presenceKeyPrefix is the operational Redis namespace for Interactive Presence. It sits strictly
// outside xenobot:cache: so CLEAR_CACHE_ON_STARTUP never wipes bot_react retraction records,
// per-user cooldowns, or Emoji Consensus sets and latches.
const presenceKeyPrefix = "xenobot:presence:"

// presenceRedisClient returns the Redis client shared with the conversation feature, or nil when
// Redis is unavailable. Swapped automatically in tests via withChatRedis (or directly via chatSwap).
var presenceRedisClient = func() *redis.Client {
	return chatRedisClient()
}

func presenceChatTag(chatID int64) string {
	return presenceKeyPrefix + "{" + strconv.FormatInt(chatID, 10) + "}:"
}

// presenceBotReactKey holds 24h metadata ({e, a, s}) for one bot reaction on a message.
func presenceBotReactKey(chatID, msgID int64) string {
	return presenceChatTag(chatID) + "bot_react:" + strconv.FormatInt(msgID, 10)
}

// presenceUserCooldownKey holds a per-user cooldown (extended on thumbs-down retraction).
func presenceUserCooldownKey(chatID, userID int64) string {
	return presenceChatTag(chatID) + "cd:u:" + strconv.FormatInt(userID, 10)
}

// presenceConsensusUsersKey holds the Redis SET of distinct non-bot user IDs who reacted with emoji
// on msgID. Hex-encoding the UTF-8 emoji avoids colons or whitespace issues in Redis keys.
func presenceConsensusUsersKey(chatID, msgID int64, emoji string) string {
	return presenceChatTag(chatID) + "cons:u:" + strconv.FormatInt(msgID, 10) + ":" + hex.EncodeToString([]byte(emoji))
}

// presenceConsensusCooldownKey gates per-chat Emoji Consensus firing across replicas.
func presenceConsensusCooldownKey(chatID int64) string {
	return presenceChatTag(chatID) + "cons:cd"
}

// presenceConsensusDoneKey is the once-only per-message Emoji Consensus latch.
func presenceConsensusDoneKey(chatID, msgID int64) string {
	return presenceChatTag(chatID) + "cons:done:" + strconv.FormatInt(msgID, 10)
}

// Shared safety gate seams (PRES-02). Package variables only so tests can inject fixtures/errors.
var (
	// presenceStaffLookup is the cached skipLocal Staff Group gate for hot paths.
	presenceStaffLookup = staff.GetStaffGroup
	// presenceLockdownLookup reads active lockdowns straight from PostgreSQL and MUST be invoked
	// lazily only when an action is about to fire.
	presenceLockdownLookup = lockdown.GetActiveFresh
)

// presenceChatTypeAllowed reports whether chat is a group or supergroup (rejecting nil, private
// chats, and channels).
func presenceChatTypeAllowed(chat *gotgbot.Chat) bool {
	return chat != nil && (chat.Type == "group" || chat.Type == "supergroup")
}

// presenceIsStaffGroup reports whether chatID is registered as a Staff Group.
func presenceIsStaffGroup(chatID int64) bool {
	return presenceStaffLookup(chatID) != nil
}

// presenceIsLockedDown checks whether chatID has an active lockdown in PostgreSQL. It fails closed
// (returns true) on any database error so Interactive Presence stays silent during outages.
func presenceIsLockedDown(chatID int64) bool {
	row, err := presenceLockdownLookup(chatID)
	if err != nil {
		log.WithField("chat_id", chatID).Debug("[Presence] active lockdown lookup failed; failing closed")
		return true
	}
	return row != nil
}

// presenceMessageAllowed enforces the shared message-path safety gate (PRES-02): rejects private
// chats, channels, slash-prefixed commands, anonymous channels/admins (sender_chat), bots, and
// Staff Groups. Active lockdown checks (presenceIsLockedDown) are intentionally left lazy for
// callers right before firing so hot-path messages never hit PostgreSQL.
func presenceMessageAllowed(b *gotgbot.Bot, ctx *ext.Context) (int64, bool) {
	if b == nil || ctx == nil {
		return 0, false
	}
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	if msg == nil || !presenceChatTypeAllowed(chat) {
		return 0, false
	}
	if raidFloodIsSlashText(msg) {
		return 0, false
	}
	userID, ok := raidFloodSenderCounts(b, ctx)
	if !ok || ctx.EffectiveSender.User.IsBot {
		return 0, false
	}
	if presenceIsStaffGroup(chat.Id) {
		return 0, false
	}
	return userID, true
}

// presenceIsAnonGroupAdmin reports whether a MessageReactionUpdated came from an anonymous
// administrator of the same group (User == nil and ActorChat.Id == Chat.Id). External linked
// channels (ActorChat.Id != Chat.Id) and ordinary users return false.
func presenceIsAnonGroupAdmin(mru *gotgbot.MessageReactionUpdated) bool {
	return mru != nil && mru.User == nil && mru.ActorChat != nil && mru.ActorChat.Id == mru.Chat.Id
}

// presenceWrites wraps the database writers for Interactive Presence settings; fields are
// variables only so tests can inject write failures.
var presenceWrites = struct {
	ConsensusEnabled   func(ctx context.Context, chatID int64, enabled bool, by int64) error
	ConsensusThreshold func(ctx context.Context, chatID int64, threshold int, by int64) error
	RetractEnabled     func(ctx context.Context, chatID int64, enabled bool, by int64) error
}{
	ConsensusEnabled:   presence.SetConsensusEnabled,
	ConsensusThreshold: presence.SetConsensusThreshold,
	RetractEnabled:     presence.SetRetractEnabled,
}

// presenceReadFresh reads a chat's Interactive Presence settings straight from PostgreSQL after a
// write; a variable only so tests can make read-back fail.
var presenceReadFresh = presence.GetFresh

// presenceApplyOutcome describes the result of an applyPresence* writer.
type presenceApplyOutcome int

const (
	// presenceApplied means the write succeeded.
	presenceApplied presenceApplyOutcome = iota
	// presenceInvalidValue means the supplied value failed bounds validation; nothing was written.
	presenceInvalidValue
	// presenceSaveFailed means the database write failed; stored settings are unchanged.
	presenceSaveFailed
)

// presenceApplyResult is the typed outcome of an applyPresence* call. Settings is the freshly read
// row after a successful write, or nil if read-back failed or nothing was written.
type presenceApplyResult struct {
	Outcome  presenceApplyOutcome
	Settings *presence.Settings
	Err      error
}

// applyPresenceConsensusEnabled toggles Emoji Consensus for chatID and reads back the stored row.
func applyPresenceConsensusEnabled(ctx context.Context, chatID int64, enabled bool, by int64) presenceApplyResult {
	if err := presenceWrites.ConsensusEnabled(ctx, chatID, enabled, by); err != nil {
		log.WithField("chat_id", chatID).Warn("[Presence] saving consensus_enabled failed")
		return presenceApplyResult{Outcome: presenceSaveFailed, Err: err}
	}
	return presenceReadBack(ctx, chatID)
}

// applyPresenceConsensusThreshold stores the Emoji Consensus distinct-member threshold (2..20) for
// chatID, rejecting out-of-range values with zero database calls.
func applyPresenceConsensusThreshold(ctx context.Context, chatID int64, threshold int, by int64) presenceApplyResult {
	if !presence.ValidConsensusThreshold(threshold) {
		return presenceApplyResult{Outcome: presenceInvalidValue}
	}
	if err := presenceWrites.ConsensusThreshold(ctx, chatID, threshold, by); err != nil {
		log.WithField("chat_id", chatID).Warn("[Presence] saving consensus_threshold failed")
		return presenceApplyResult{Outcome: presenceSaveFailed, Err: err}
	}
	return presenceReadBack(ctx, chatID)
}

// applyPresenceRetractEnabled toggles thumbs-down reaction retraction for chatID and reads back
// the stored row.
func applyPresenceRetractEnabled(ctx context.Context, chatID int64, enabled bool, by int64) presenceApplyResult {
	if err := presenceWrites.RetractEnabled(ctx, chatID, enabled, by); err != nil {
		log.WithField("chat_id", chatID).Warn("[Presence] saving retract_enabled failed")
		return presenceApplyResult{Outcome: presenceSaveFailed, Err: err}
	}
	return presenceReadBack(ctx, chatID)
}

func presenceReadBack(ctx context.Context, chatID int64) presenceApplyResult {
	settings, err := presenceReadFresh(ctx, chatID)
	if err != nil {
		log.WithField("chat_id", chatID).Warn("[Presence] reading back presence settings failed")
		return presenceApplyResult{Outcome: presenceApplied}
	}
	return presenceApplyResult{Outcome: presenceApplied, Settings: &settings}
}
