package modules

import (
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/poll"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/reaction"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/db/chatconv"
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

	// presencePMBriefCooldown gates per-user ✍ PM brief reactions in one chat.
	// Package variable only so tests can shorten it.
	presencePMBriefCooldown = 60 * time.Second

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
//
//   - KEYS[1]: the reactor SET; KEYS[2]: the per-(message, user) stamp of the newest update applied.
//
// ARGV: [1] "add" or "rem", [2] user ID, [3] TTL seconds, [4] the update's update_id (0 = unstamped).
//
// Reaction updates are handled concurrently, so one user's add and remove can arrive out of order.
// An update older than the stamp is dropped (-1 is returned, nothing changes), so a late "add" can
// never resurrect a reaction the user already took back. The same update_id may touch several
// emojis, so only a strictly older one is stale.
var presenceConsensusUpdateScript = redis.NewScript(`
local stamp = tonumber(ARGV[4]) or 0
if stamp > 0 then
	local last = tonumber(redis.call('GET', KEYS[2]) or '0') or 0
	if stamp < last then
		return -1
	end
	redis.call('SET', KEYS[2], ARGV[4], 'EX', ARGV[3])
end
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
// other. Both keys carry the {<chatID>} hash tag for Redis Cluster slot compatibility. It returns
// {1, 0} when claimed, {0, 0} when the message already has its mirror, and {2, pttl_ms} while the
// chat cooldown runs, so the caller can retry the message when the cooldown ends.
var presenceConsensusClaimScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[2]) == 1 then
	return {0, 0}
end
if redis.call('EXISTS', KEYS[1]) == 1 then
	return {2, redis.call('PTTL', KEYS[1])}
end
redis.call('SET', KEYS[1], '1', 'EX', ARGV[1])
redis.call('SET', KEYS[2], '1', 'EX', ARGV[2])
return {1, 0}
`)

// LoadPresence registers the MessageReactionUpdated router at handler group 0, the /setarchive
// and /unsetarchive commands, and the group-11 forward capture watcher.
func LoadPresence(dispatcher *ext.Dispatcher) {
	dispatcher.AddHandler(handlers.NewReaction(reaction.All, presenceModule.handleReaction))
	dispatcher.AddHandler(handlers.NewPoll(poll.All, chatModule.handlePollUpdate))
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
	if mru == nil || !presenceChatTypeAllowed(&mru.Chat) || (mru.User != nil && mru.User.IsBot) {
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
	var updateID int64
	if ctx.Update != nil {
		updateID = ctx.UpdateId
	}
	presenceDispatchReaction(runCtx, b, mru, settings, added, removed, updateID)
}

// presenceStepContext is the context of one reaction sub-feature. Retraction, consensus, archive
// and the PM brief each get their own budget derived from chatContext(), so a slow step (a cold
// getChat behind the pacer, a live admin check) cannot leave the next one with what is left of a
// shared 3 s, and StopChat still ends every one of them.
func presenceStepContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(chatContext(), presenceReactionTimeout)
}

func presenceDispatchReaction(runCtx context.Context, b *gotgbot.Bot, mru *gotgbot.MessageReactionUpdated, settings presence.Settings, added, removed []string, updateID int64) {
	if settings.RetractEnabled && slices.Contains(added, "👎") {
		stepCtx, cancel := presenceStepContext()
		presenceHandleRetract(stepCtx, b, mru)
		cancel()
	}
	if mru.User == nil {
		return
	}
	if settings.ConsensusEnabled {
		stepCtx, cancel := presenceStepContext()
		presenceHandleConsensus(stepCtx, b, mru, settings.ConsensusThreshold, added, removed, updateID)
		cancel()
	}
	if settings.ArchiveEnabled && settings.ArchiveChatID != 0 {
		stepCtx, cancel := presenceStepContext()
		presenceHandleArchive(stepCtx, b, mru, settings, updateID)
		cancel()
	}
	if settings.PMBriefReact && slices.Contains(added, "✍") {
		stepCtx, cancel := presenceStepContext()
		presenceHandlePMBrief(stepCtx, b, mru)
		cancel()
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
// standard emoji (REACT-04). When any added emoji reaches threshold, it tries to mirror it once
// (presenceConsensusTry). An update older than the user's newest applied one is dropped, so
// concurrent handling cannot leave a phantom reactor (updateID 0 disables the check).
func presenceHandleConsensus(ctx context.Context, b *gotgbot.Bot, mru *gotgbot.MessageReactionUpdated, threshold int, added, removed []string, updateID int64) {
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
		count, err := presenceConsensusUpdate(ctx, client, chatID, msgID, userID, emoji, "rem", userArg, setTTLSecs, updateID)
		if err != nil {
			log.WithField("chat_id", chatID).Debugf("[Presence] consensus set remove failed: %s", chatRedisErrClass(err))
		}
		if count < 0 {
			return // a newer update of this user was already applied
		}
	}

	var crossed []string
	for _, emoji := range added {
		if _, hostile := presenceConsensusHostile[emoji]; hostile {
			continue
		}
		if ctx.Err() != nil {
			return
		}
		count, err := presenceConsensusUpdate(ctx, client, chatID, msgID, userID, emoji, "add", userArg, setTTLSecs, updateID)
		if err != nil {
			log.WithField("chat_id", chatID).Debugf("[Presence] consensus set add failed: %s", chatRedisErrClass(err))
			continue
		}
		if count < 0 {
			return
		}
		if count >= int64(threshold) {
			crossed = append(crossed, emoji)
		}
	}
	if len(crossed) == 0 {
		return
	}
	if wait := presenceConsensusTry(ctx, b, client, chatID, msgID, crossed); wait > 0 {
		presenceConsensusScheduleRetry(b, chatID, msgID, crossed, threshold, wait, 1)
	}
}

// presenceConsensusTry mirrors the first allowed emoji of crossed on msgID, once. The palette and
// the lockdown are checked before anything is claimed, and the claim (the chat cooldown and the
// per-message latch) is given back when the reaction could not be set, so a failure of any kind
// (an emoji the chat does not allow, a shed pacer call, a Telegram 400, an expired budget) never
// locks the chat or the message out. It returns how long the chat cooldown still has to run when
// that is the only thing that stopped the mirror, else 0.
func presenceConsensusTry(ctx context.Context, b *gotgbot.Bot, client *redis.Client, chatID, msgID int64, crossed []string) time.Duration {
	if presenceIsLockedDown(chatID) {
		return 0
	}
	emoji := ""
	for _, candidate := range crossed {
		if presenceAllowedEmoji(ctx, b, chatID, candidate) {
			emoji = candidate
			break
		}
	}
	if emoji == "" {
		return 0
	}
	claim, wait := presenceConsensusClaim(ctx, client, chatID, msgID)
	switch claim {
	case presenceClaimWon:
	case presenceClaimCooldown:
		return wait
	default:
		return 0
	}
	authorID := presenceLogAuthorOf(ctx, client, chatID, msgID)
	if !presenceSetReaction(ctx, b, chatID, msgID, authorID, emoji, "cons") {
		presenceConsensusRelease(client, chatID, msgID)
	}
	return 0
}

// presenceConsensusRelease gives back the cooldown and the latch after a failed mirror, on its
// own short context because the one that failed may already be expired.
func presenceConsensusRelease(client *redis.Client, chatID, msgID int64) {
	if client == nil {
		return
	}
	rCtx, cancel := context.WithTimeout(context.Background(), chatRedisTimeout)
	defer cancel()
	if err := client.Del(rCtx, presenceConsensusCooldownKey(chatID), presenceConsensusDoneKey(chatID, msgID)).Err(); err != nil {
		log.WithField("chat_id", chatID).Debugf("[Presence] consensus claim release failed: %s", chatRedisErrClass(err))
	}
}

var (
	// presenceConsensusRetryJitter is added to the remaining cooldown before a delayed retry, so
	// replicas and messages that waited on the same cooldown do not all fire at once. Package
	// variable only so tests can shorten it.
	presenceConsensusRetryJitter = 2 * time.Second
	// presenceConsensusRetryMax bounds the delayed retries one replica holds.
	presenceConsensusRetryMax = 256
	// presenceConsensusMaxAttempts is how many times one message is retried after cooldowns.
	presenceConsensusMaxAttempts = 3

	presenceConsensusPendingMu sync.Mutex
	presenceConsensusPending   = map[string]struct{}{}
)

// presenceConsensusScheduleRetry retries a message that reached its threshold while the chat
// cooldown ran. The threshold is only checked when a reaction is added, so without this a burst of
// reactions followed by silence would never fire once the cooldown ended. The retry is a timer, not
// a goroutine: it holds nothing while it waits, joins StopChat when it fires (chatJobs,
// chatDraining) and is lost on a restart, which only costs one mirror. One retry per (chat,
// message) is held at a time, up to presenceConsensusRetryMax per replica.
func presenceConsensusScheduleRetry(b *gotgbot.Bot, chatID, msgID int64, crossed []string, threshold int, wait time.Duration, attempt int) {
	if attempt > presenceConsensusMaxAttempts || chatDraining.Load() {
		return
	}
	key := strconv.FormatInt(chatID, 10) + ":" + strconv.FormatInt(msgID, 10)
	presenceConsensusPendingMu.Lock()
	if _, held := presenceConsensusPending[key]; held || len(presenceConsensusPending) >= presenceConsensusRetryMax {
		presenceConsensusPendingMu.Unlock()
		return
	}
	presenceConsensusPending[key] = struct{}{}
	presenceConsensusPendingMu.Unlock()

	delay := max(wait, time.Second) + presenceConsensusRetryJitter
	emojis := slices.Clone(crossed)
	time.AfterFunc(delay, func() {
		presenceConsensusPendingMu.Lock()
		delete(presenceConsensusPending, key)
		presenceConsensusPendingMu.Unlock()
		presenceConsensusRetry(b, chatID, msgID, emojis, threshold, attempt)
	})
}

// presenceConsensusRetry is the delayed second look at a message that crossed its threshold during
// the chat cooldown: it re-reads the settings and the reactor counts (members may have taken their
// reactions back) and tries again, with the same palette, lockdown and claim rules.
func presenceConsensusRetry(b *gotgbot.Bot, chatID, msgID int64, emojis []string, threshold, attempt int) {
	defer error_handling.RecoverFromPanic("presenceConsensusRetry", "Presence")
	chatJobs.start()
	if chatDraining.Load() {
		chatJobs.finish()
		return
	}
	defer chatJobs.finish()

	ctx, cancel := presenceStepContext()
	defer cancel()
	client := presenceRedisClient()
	if client == nil {
		return
	}
	settings, err := presenceSettingsGet(ctx, chatID)
	if err != nil || !settings.ConsensusEnabled {
		return
	}
	if presence.ValidConsensusThreshold(settings.ConsensusThreshold) {
		threshold = settings.ConsensusThreshold
	}
	var crossed []string
	for _, emoji := range emojis {
		rCtx, rCancel := context.WithTimeout(ctx, chatRedisTimeout)
		count, err := client.SCard(rCtx, presenceConsensusUsersKey(chatID, msgID, emoji)).Result()
		rCancel()
		if err == nil && count >= int64(threshold) {
			crossed = append(crossed, emoji)
		}
	}
	if len(crossed) == 0 {
		return
	}
	if wait := presenceConsensusTry(ctx, b, client, chatID, msgID, crossed); wait > 0 {
		presenceConsensusScheduleRetry(b, chatID, msgID, crossed, threshold, wait, attempt+1)
	}
}

// presenceConsensusUpdate applies one add or remove to an emoji's reactor SET and returns its
// size, or -1 when the update is older than the user's newest applied one and was dropped.
func presenceConsensusUpdate(ctx context.Context, client *redis.Client, chatID, msgID, userID int64, emoji, op, userArg string, ttlSecs int, updateID int64) (int64, error) {
	rCtx, cancel := context.WithTimeout(ctx, chatRedisTimeout)
	defer cancel()
	return presenceConsensusUpdateScript.Run(
		rCtx,
		client,
		[]string{presenceConsensusUsersKey(chatID, msgID, emoji), presenceReactionStampKey(chatID, msgID, userID)},
		op,
		userArg,
		ttlSecs,
		updateID,
	).Int64()
}

// presenceClaim is the result of claiming a message's mirror.
type presenceClaim int

const (
	// presenceClaimNone is a refusal (already mirrored) or an error.
	presenceClaimNone presenceClaim = iota
	// presenceClaimWon means this caller holds the cooldown and the latch.
	presenceClaimWon
	// presenceClaimCooldown means the chat cooldown is running; the wait is its remaining time.
	presenceClaimCooldown
)

func presenceConsensusClaim(ctx context.Context, client *redis.Client, chatID, msgID int64) (presenceClaim, time.Duration) {
	cdSecs := max(int(presenceConsensusCooldown.Seconds()), 1)
	doneSecs := max(int(presenceConsensusDoneTTL.Seconds()), 1)
	rCtx, cancel := context.WithTimeout(ctx, chatRedisTimeout)
	defer cancel()
	res, err := presenceConsensusClaimScript.Run(
		rCtx,
		client,
		[]string{presenceConsensusCooldownKey(chatID), presenceConsensusDoneKey(chatID, msgID)},
		cdSecs,
		doneSecs,
	).Int64Slice()
	if err != nil || len(res) < 2 {
		if err != nil {
			log.WithField("chat_id", chatID).Debugf("[Presence] consensus claim failed: %s", chatRedisErrClass(err))
		}
		return presenceClaimNone, 0
	}
	switch res[0] {
	case 1:
		return presenceClaimWon, 0
	case 2:
		return presenceClaimCooldown, time.Duration(max(res[1], 0)) * time.Millisecond
	}
	return presenceClaimNone, 0
}

// presencePMBriefCooldownKey gates per-user ✍ PM brief reactions in one chat, outside xenobot:cache:.
func presencePMBriefCooldownKey(chatID, userID int64) string {
	return presenceKeyPrefix + "pmbrief:cd:{" + strconv.FormatInt(chatID, 10) + "}:" + strconv.FormatInt(userID, 10)
}

// presenceHandlePMBrief handles a ✍ reaction (CHAT-05): enforces Owner LogEnabled + provider,
// lazy fail-closed lockdown check, Audience gate, per-user Redis cooldown, and logged message
// resolution before queueing a silent PM brief under StopChat. Posts zero group messages.
func presenceHandlePMBrief(ctx context.Context, b *gotgbot.Bot, mru *gotgbot.MessageReactionUpdated) {
	if mru == nil || mru.User == nil || !chatAnyProviderConfigured() {
		return
	}
	client := presenceRedisClient()
	if client == nil {
		return
	}
	conv, err := chatSettingsLookup(ctx, mru.Chat.Id)
	if err != nil || !conv.LogEnabled || !chatProviderConfigured(conv.Provider) {
		return
	}
	if presenceIsLockedDown(mru.Chat.Id) {
		return
	}
	if !chatAudienceAllows(ctx, b, conv.Audience, mru.Chat.Id, mru.User.Id) {
		return
	}
	rCtx, cancel := context.WithTimeout(ctx, chatRedisTimeout)
	defer cancel()
	claimed, err := client.SetNX(rCtx, presencePMBriefCooldownKey(mru.Chat.Id, mru.User.Id), "1", presencePMBriefCooldown).Result()
	if err != nil || !claimed {
		return
	}
	entry, found, err := chatLogEntryAt(ctx, client, mru.Chat.Id, mru.MessageId)
	if err != nil || !found {
		return
	}
	chatBriefEnqueue(chatBriefJob{
		bot:       b,
		chatID:    mru.Chat.Id,
		chatTitle: mru.Chat.Title,
		userID:    mru.User.Id,
		messageID: mru.MessageId,
		threadID:  entry.Thread,
		fromID:    mru.MessageId,
		provider:  conv.Provider,
		kind:      chatBriefShort,
		dest:      chatconv.JudgeToPrivate,
		silentPM:  true,
	})
}
