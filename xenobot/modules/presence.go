package modules

import (
	"context"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

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
// per-user cooldowns, AI reaction cooldowns/gap counters, or Emoji Consensus sets and latches.
const presenceKeyPrefix = "xenobot:presence:"

// presenceStaticReactedKey is the ctx.Data boolean handoff key set by group 8 static/cat-sticker
// reactions (Phase 33) and respected by group 13 contextual AI reactions so AI reactions never
// overwrite a static reaction.
const presenceStaticReactedKey = "presence.static_reacted"

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

// presenceReactionStampKey holds the update_id of the newest reaction update applied for one user on
// one message (consensus and archive share it), so a late update is recognised and dropped.
func presenceReactionStampKey(chatID, msgID, userID int64) string {
	return presenceChatTag(chatID) + "react:stamp:" + strconv.FormatInt(msgID, 10) + ":" + strconv.FormatInt(userID, 10)
}

// presenceConsensusCooldownKey gates per-chat Emoji Consensus firing across replicas.
func presenceConsensusCooldownKey(chatID int64) string {
	return presenceChatTag(chatID) + "cons:cd"
}

// presenceConsensusDoneKey is the once-only per-message Emoji Consensus latch.
func presenceConsensusDoneKey(chatID, msgID int64) string {
	return presenceChatTag(chatID) + "cons:done:" + strconv.FormatInt(msgID, 10)
}

// presenceAIReactGapKey holds the per-chat non-slash member message counter between AI reactions.
// It shares the {<chatID>} Redis Cluster hash tag with presenceAIReactChatCooldownKey and
// presenceUserCooldownKey so one Lua script checks and updates all three atomically.
func presenceAIReactGapKey(chatID int64) string {
	return presenceChatTag(chatID) + "ai:gap"
}

// presenceAIReactChatCooldownKey gates per-chat Contextual AI Emoji Reactions across replicas.
func presenceAIReactChatCooldownKey(chatID int64) string {
	return presenceChatTag(chatID) + "ai:cd:chat"
}

// presenceArchiveUsersKey holds the Redis SET of distinct non-bot, non-author member user IDs who
// have an active qualifying reaction on msgID for Highlights Archive.
func presenceArchiveUsersKey(chatID, msgID int64) string {
	return presenceChatTag(chatID) + "arch:u:" + strconv.FormatInt(msgID, 10)
}

// presenceArchiveDoneKey is the fast per-message Redis latch indicating msgID was already claimed
// in PostgreSQL (or permanently skipped) for Highlights Archive.
func presenceArchiveDoneKey(chatID, msgID int64) string {
	return presenceChatTag(chatID) + "arch:done:" + strconv.FormatInt(msgID, 10)
}

// presencePollKey holds JSON metadata ({chat_id, message_id, thread_id, lang, question}) for an
// active /referendum poll so closed Update.Poll payloads (which omit chat_id) route back to their
// origin chat and thread.
func presencePollKey(pollID string) string {
	return presenceKeyPrefix + "poll:" + pollID
}

// presencePollTieKey is the atomic cross-replica once-only tie-breaker claim key for a closed poll.
func presencePollTieKey(pollID string) string {
	return presenceKeyPrefix + "poll:tie:" + pollID
}

// presenceToyChatCooldownKey gates per-chat Interactive Group Toys invocations across replicas.
func presenceToyChatCooldownKey(chatID int64) string {
	return presenceChatTag(chatID) + "toy:cd"
}

// presenceToyUserCooldownKey gates per-user Interactive Group Toys invocations within a chat.
func presenceToyUserCooldownKey(chatID, userID int64) string {
	return presenceChatTag(chatID) + "toy:cd:u:" + strconv.FormatInt(userID, 10)
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

// The update's own memo of the two per-message reads the group 13 watchers share, kept in ctx.Data:
// chatWatch (name addressing) and chatReactWatch (reactions) both need the chat's presence settings and
// the Staff Group answer, so each is asked once per update, whichever watcher asks first.
const (
	presenceStaffMemoKey    = "xenobot:presence:staffmemo"
	presenceSettingsMemoKey = "xenobot:presence:settingsmemo"
)

type presenceSettingsMemo struct {
	settings presence.Settings
	err      error
}

// presenceIsStaffGroupMemo is presenceIsStaffGroup answered once per update.
func presenceIsStaffGroupMemo(ctx *ext.Context, chatID int64) bool {
	if ctx == nil || ctx.Data == nil {
		return presenceIsStaffGroup(chatID)
	}
	if known, ok := ctx.Data[presenceStaffMemoKey].(bool); ok {
		return known
	}
	staffGroup := presenceIsStaffGroup(chatID)
	ctx.Data[presenceStaffMemoKey] = staffGroup
	return staffGroup
}

// presenceSettingsOnce reads the chat's presence settings once per update (an error is memoised too, so a
// failing read is not retried by the second watcher).
func presenceSettingsOnce(ctx *ext.Context, chatID int64, timeout time.Duration) (presence.Settings, error) {
	if ctx != nil && ctx.Data != nil {
		if memo, ok := ctx.Data[presenceSettingsMemoKey].(presenceSettingsMemo); ok {
			return memo.settings, memo.err
		}
	}
	readCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	settings, err := presenceSettingsGet(readCtx, chatID)
	if ctx != nil && ctx.Data != nil {
		ctx.Data[presenceSettingsMemoKey] = presenceSettingsMemo{settings: settings, err: err}
	}
	return settings, err
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
	if presenceIsStaffGroupMemo(ctx, chat.Id) {
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
	AIReactMode        func(ctx context.Context, chatID int64, mode string, by int64) error
	CatStickerEnabled  func(ctx context.Context, chatID int64, enabled bool, by int64) error
	ArchiveEnabled     func(ctx context.Context, chatID int64, enabled bool, by int64) error
	ArchiveTarget      func(ctx context.Context, chatID, targetChatID, targetThreadID, by int64) error
	ClearArchiveTarget func(ctx context.Context, chatID, by int64) error
	ArchiveThreshold   func(ctx context.Context, chatID int64, threshold int, by int64) error
	ArchiveEmojiMode   func(ctx context.Context, chatID int64, mode string, by int64) error
	ArchiveDelivery    func(ctx context.Context, chatID int64, delivery string, by int64) error
	AmbientContext     func(ctx context.Context, chatID int64, enabled bool, by int64) error
	VocativeEnabled    func(ctx context.Context, chatID int64, enabled bool, by int64) error
	PMBriefReact       func(ctx context.Context, chatID int64, enabled bool, by int64) error
	ReferendumEnabled  func(ctx context.Context, chatID int64, enabled bool, by int64) error
	InspectEnabled     func(ctx context.Context, chatID int64, enabled bool, by int64) error
	ReactFreq          func(ctx context.Context, chatID int64, kind string, freq int, by int64) error
	ReactTopics        func(ctx context.Context, chatID int64, raw string, by int64) error
	BotNames           func(ctx context.Context, chatID int64, raw string, by int64) error
}{
	ConsensusEnabled:   presence.SetConsensusEnabled,
	ConsensusThreshold: presence.SetConsensusThreshold,
	RetractEnabled:     presence.SetRetractEnabled,
	AIReactMode:        presence.SetAIReactMode,
	CatStickerEnabled:  presence.SetCatStickerEnabled,
	ArchiveEnabled:     presence.SetArchiveEnabled,
	ArchiveTarget:      presence.SetArchiveTarget,
	ClearArchiveTarget: presence.ClearArchiveTarget,
	ArchiveThreshold:   presence.SetArchiveThreshold,
	ArchiveEmojiMode:   presence.SetArchiveEmojiMode,
	ArchiveDelivery:    presence.SetArchiveDelivery,
	AmbientContext:     presence.SetAmbientContext,
	VocativeEnabled:    presence.SetVocativeEnabled,
	PMBriefReact:       presence.SetPMBriefReact,
	ReferendumEnabled:  presence.SetReferendumEnabled,
	InspectEnabled:     presence.SetInspectEnabled,
	ReactFreq:          presence.SetReactFreq,
	ReactTopics:        presence.SetReactTopics,
	BotNames:           presence.SetBotNames,
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

// applyPresenceAIReactMode stores the Contextual AI Emoji Reactions mode ('off', 'low', 'medium')
// for chatID, rejecting invalid values with zero database calls.
func applyPresenceAIReactMode(ctx context.Context, chatID int64, mode string, by int64) presenceApplyResult {
	if !presence.ValidAIReactMode(mode) {
		return presenceApplyResult{Outcome: presenceInvalidValue}
	}
	if err := presenceWrites.AIReactMode(ctx, chatID, mode, by); err != nil {
		log.WithField("chat_id", chatID).Warn("[Presence] saving ai_react_mode failed")
		return presenceApplyResult{Outcome: presenceSaveFailed, Err: err}
	}
	return presenceReadBack(ctx, chatID)
}

// applyPresenceCatStickerEnabled toggles Cat-Sticker Micro-Triggers for chatID and reads back
// the stored row.
func applyPresenceCatStickerEnabled(ctx context.Context, chatID int64, enabled bool, by int64) presenceApplyResult {
	if err := presenceWrites.CatStickerEnabled(ctx, chatID, enabled, by); err != nil {
		log.WithField("chat_id", chatID).Warn("[Presence] saving cat_sticker_enabled failed")
		return presenceApplyResult{Outcome: presenceSaveFailed, Err: err}
	}
	return presenceReadBack(ctx, chatID)
}

// applyPresenceArchiveEnabled toggles Highlights Archive for chatID. Enabling without a linked
// destination fails validation (presenceInvalidValue).
func applyPresenceArchiveEnabled(ctx context.Context, chatID int64, enabled bool, by int64) presenceApplyResult {
	if err := presenceWrites.ArchiveEnabled(ctx, chatID, enabled, by); err != nil {
		if errors.Is(err, presence.ErrInvalidValue) {
			return presenceApplyResult{Outcome: presenceInvalidValue}
		}
		log.WithField("chat_id", chatID).Warn("[Presence] saving archive_enabled failed")
		return presenceApplyResult{Outcome: presenceSaveFailed, Err: err}
	}
	return presenceReadBack(ctx, chatID)
}

// applyPresenceArchiveTarget links targetChatID (and optional targetThreadID) as chatID's
// Highlights Archive destination, rejecting invalid target IDs before any database call.
func applyPresenceArchiveTarget(ctx context.Context, chatID, targetChatID, targetThreadID, by int64) presenceApplyResult {
	if targetChatID == 0 || targetThreadID < 0 {
		return presenceApplyResult{Outcome: presenceInvalidValue}
	}
	if err := presenceWrites.ArchiveTarget(ctx, chatID, targetChatID, targetThreadID, by); err != nil {
		if errors.Is(err, presence.ErrInvalidValue) {
			return presenceApplyResult{Outcome: presenceInvalidValue}
		}
		log.WithField("chat_id", chatID).Warn("[Presence] saving archive target failed")
		return presenceApplyResult{Outcome: presenceSaveFailed, Err: err}
	}
	return presenceReadBack(ctx, chatID)
}

// applyPresenceClearArchiveTarget unlinks chatID's Highlights Archive destination and disables
// archiving without overwriting unrelated presence settings.
func applyPresenceClearArchiveTarget(ctx context.Context, chatID, by int64) presenceApplyResult {
	if err := presenceWrites.ClearArchiveTarget(ctx, chatID, by); err != nil {
		log.WithField("chat_id", chatID).Warn("[Presence] clearing archive target failed")
		return presenceApplyResult{Outcome: presenceSaveFailed, Err: err}
	}
	return presenceReadBack(ctx, chatID)
}

// applyPresenceArchiveThreshold stores the Highlights Archive distinct-member threshold (2..50)
// for chatID, rejecting out-of-range values with zero database calls.
func applyPresenceArchiveThreshold(ctx context.Context, chatID int64, threshold int, by int64) presenceApplyResult {
	if !presence.ValidArchiveThreshold(threshold) {
		return presenceApplyResult{Outcome: presenceInvalidValue}
	}
	if err := presenceWrites.ArchiveThreshold(ctx, chatID, threshold, by); err != nil {
		log.WithField("chat_id", chatID).Warn("[Presence] saving archive_threshold failed")
		return presenceApplyResult{Outcome: presenceSaveFailed, Err: err}
	}
	return presenceReadBack(ctx, chatID)
}

// applyPresenceArchiveEmojiMode stores the Highlights Archive emoji filter ('positive', 'all')
// for chatID, rejecting invalid values with zero database calls.
func applyPresenceArchiveEmojiMode(ctx context.Context, chatID int64, mode string, by int64) presenceApplyResult {
	if !presence.ValidArchiveEmojiMode(mode) {
		return presenceApplyResult{Outcome: presenceInvalidValue}
	}
	if err := presenceWrites.ArchiveEmojiMode(ctx, chatID, mode, by); err != nil {
		log.WithField("chat_id", chatID).Warn("[Presence] saving archive_emoji_mode failed")
		return presenceApplyResult{Outcome: presenceSaveFailed, Err: err}
	}
	return presenceReadBack(ctx, chatID)
}

// applyPresenceArchiveDelivery stores the Highlights Archive delivery mode ('copy', 'forward')
// for chatID, rejecting invalid values with zero database calls.
func applyPresenceArchiveDelivery(ctx context.Context, chatID int64, delivery string, by int64) presenceApplyResult {
	if !presence.ValidArchiveDelivery(delivery) {
		return presenceApplyResult{Outcome: presenceInvalidValue}
	}
	if err := presenceWrites.ArchiveDelivery(ctx, chatID, delivery, by); err != nil {
		log.WithField("chat_id", chatID).Warn("[Presence] saving archive_delivery failed")
		return presenceApplyResult{Outcome: presenceSaveFailed, Err: err}
	}
	return presenceReadBack(ctx, chatID)
}

// applyPresenceAmbientContext toggles ambient room context for chatID and reads back the stored row.
func applyPresenceAmbientContext(ctx context.Context, chatID int64, enabled bool, by int64) presenceApplyResult {
	if err := presenceWrites.AmbientContext(ctx, chatID, enabled, by); err != nil {
		log.WithField("chat_id", chatID).Warn("[Presence] saving ambient_context failed")
		return presenceApplyResult{Outcome: presenceSaveFailed, Err: err}
	}
	return presenceReadBack(ctx, chatID)
}

// applyPresenceVocativeEnabled toggles vocative name-addressing for chatID and reads back the stored row.
func applyPresenceVocativeEnabled(ctx context.Context, chatID int64, enabled bool, by int64) presenceApplyResult {
	if err := presenceWrites.VocativeEnabled(ctx, chatID, enabled, by); err != nil {
		log.WithField("chat_id", chatID).Warn("[Presence] saving vocative_enabled failed")
		return presenceApplyResult{Outcome: presenceSaveFailed, Err: err}
	}
	return presenceReadBack(ctx, chatID)
}

// applyPresencePMBriefReact toggles silent ✍ PM briefs for chatID and reads back the stored row.
func applyPresencePMBriefReact(ctx context.Context, chatID int64, enabled bool, by int64) presenceApplyResult {
	if err := presenceWrites.PMBriefReact(ctx, chatID, enabled, by); err != nil {
		log.WithField("chat_id", chatID).Warn("[Presence] saving pm_brief_react failed")
		return presenceApplyResult{Outcome: presenceSaveFailed, Err: err}
	}
	return presenceReadBack(ctx, chatID)
}

// applyPresenceReferendumEnabled toggles /referendum polls for chatID and reads back the stored row.
func applyPresenceReferendumEnabled(ctx context.Context, chatID int64, enabled bool, by int64) presenceApplyResult {
	if err := presenceWrites.ReferendumEnabled(ctx, chatID, enabled, by); err != nil {
		log.WithField("chat_id", chatID).Warn("[Presence] saving referendum_enabled failed")
		return presenceApplyResult{Outcome: presenceSaveFailed, Err: err}
	}
	return presenceReadBack(ctx, chatID)
}

// applyPresenceReactFreq stores how often the bot reacts to one kind of message (presence.KindText,
// KindSticker or KindPhoto), rejecting an unknown kind or frequency with zero database calls.
func applyPresenceReactFreq(ctx context.Context, chatID int64, kind string, freq int, by int64) presenceApplyResult {
	if !presence.ValidReactKind(kind) || !presence.ValidReactFreq(freq) {
		return presenceApplyResult{Outcome: presenceInvalidValue}
	}
	if err := presenceWrites.ReactFreq(ctx, chatID, kind, freq, by); err != nil {
		log.WithField("chat_id", chatID).Warn("[Presence] saving a reaction frequency failed")
		return presenceApplyResult{Outcome: presenceSaveFailed, Err: err}
	}
	return presenceReadBack(ctx, chatID)
}

// applyPresenceReactTopics stores the topics AI reactions are limited to (raw is a typed list; empty
// clears it). A list the repository refuses is an invalid value with nothing written; the refusal's
// reason is the returned Err (presence.ErrListTooMany, ErrListTooLong or ErrListInvalid).
func applyPresenceReactTopics(ctx context.Context, chatID int64, raw string, by int64) presenceApplyResult {
	if _, err := presence.NormalizeTopics(raw); err != nil {
		return presenceApplyResult{Outcome: presenceInvalidValue, Err: err}
	}
	if err := presenceWrites.ReactTopics(ctx, chatID, raw, by); err != nil {
		log.WithField("chat_id", chatID).Warn("[Presence] saving react_topics failed")
		return presenceApplyResult{Outcome: presenceSaveFailed, Err: err}
	}
	return presenceReadBack(ctx, chatID)
}

// applyPresenceBotNames stores the extra names the bot answers to (raw is a typed list; empty clears
// it), with the same refusals as applyPresenceReactTopics.
func applyPresenceBotNames(ctx context.Context, chatID int64, raw string, by int64) presenceApplyResult {
	if _, err := presence.NormalizeBotNames(raw); err != nil {
		return presenceApplyResult{Outcome: presenceInvalidValue, Err: err}
	}
	if err := presenceWrites.BotNames(ctx, chatID, raw, by); err != nil {
		log.WithField("chat_id", chatID).Warn("[Presence] saving bot_names failed")
		return presenceApplyResult{Outcome: presenceSaveFailed, Err: err}
	}
	return presenceReadBack(ctx, chatID)
}

// applyPresenceInspectEnabled toggles /inspect verdicts for chatID and reads back the stored row.
func applyPresenceInspectEnabled(ctx context.Context, chatID int64, enabled bool, by int64) presenceApplyResult {
	if err := presenceWrites.InspectEnabled(ctx, chatID, enabled, by); err != nil {
		log.WithField("chat_id", chatID).Warn("[Presence] saving inspect_enabled failed")
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
