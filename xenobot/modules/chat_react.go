package modules

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"sync/atomic"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/db/presence"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/chatprompt"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/error_handling"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/extraction"
)

// Contextual AI Emoji Reactions (Phase 32): unaddressed group messages that pass the Owner×Admin
// double gate (PRES-04), safety checks, and cross-replica Redis Lua throttle (REACT-01) enter a
// dedicated bounded worker lane (chatPresenceQueue) sharing StopChat (chatJobs, chatDraining,
// chatContext()) with zero new shutdown drains and zero starvation of chatQueue or chatBriefQueue.

const (
	// chatPresenceQueueCapacity bounds queued background reaction jobs; a full queue sheds.
	chatPresenceQueueCapacity = 32
	// chatPresenceWorkerCount is how many background AI reactions run concurrently per replica.
	chatPresenceWorkerCount = 2
	// chatReactJobTimeout bounds one reaction job (palette, context, media download, LLM, send).
	chatReactJobTimeout = 45 * time.Second
	// chatReactGapTTL expires idle per-chat message-gap counters.
	chatReactGapTTL = 24 * time.Hour
	// chatReactRecentLogLimit is how many preceding same-thread log messages are passed as context.
	chatReactRecentLogLimit = 5
	// chatReactFeatherlessTextOnlyTTL caches per-model text-only degradation after a vision 400/422.
	chatReactFeatherlessTextOnlyTTL = 24 * time.Hour
	// chatReactFeatherlessTextOnlyPrefix lives inside xenobot:cache: so CLEAR_CACHE_ON_STARTUP clears it.
	chatReactFeatherlessTextOnlyPrefix = "xenobot:cache:featherless_text_only:"
)

// chatReactProfile defines the per-mode pre-AI throttle thresholds.
type chatReactProfile struct {
	minGap       int
	chatCooldown time.Duration
	userCooldown time.Duration
	chancePct    int
}

var (
	// chatReactProfiles holds the low and medium pre-AI throttle profiles; package var for tests.
	chatReactProfiles = map[string]chatReactProfile{
		presence.AIReactModeLow: {
			minGap:       30,
			chatCooldown: 15 * time.Minute,
			userCooldown: 10 * time.Minute,
			chancePct:    12,
		},
		presence.AIReactModeMedium: {
			minGap:       15,
			chatCooldown: 8 * time.Minute,
			userCooldown: 5 * time.Minute,
			chancePct:    25,
		},
	}

	// Dedicated hourly budgets for contextual AI reactions ("react:b"), isolated from conversations.
	// Package variables only so tests can shrink them.
	chatReactUserHourlyBudget   int64 = 6
	chatReactChatHourlyBudget   int64 = 20
	chatReactGlobalHourlyBudget int64 = 200

	chatPresenceQueue chan chatReactJob
	// chatReactProcess runs one reaction job; package variable only so tests can intercept it.
	chatReactProcess = chatReactProcessJob
	// chatReactRoll decides the probability roll using math/rand/v2 (never Lua math.random).
	chatReactRoll = func(chancePct int) bool {
		return chancePct > 0 && rand.IntN(100) < chancePct // #nosec G404 -- non-cryptographic reaction sampling
	}
)

// Per-replica atomic counters exposed in the developer /stats output.
var (
	chatReactSentCount   atomic.Int64
	chatReactNoneCount   atomic.Int64
	chatReactShedCount   atomic.Int64
	chatReactCappedCount atomic.Int64
)

// chatReactStatsSection formats the AI reactions counters for developer /stats: numbers only.
func chatReactStatsSection() string {
	return fmt.Sprintf(
		"\n    <b>AI reactions:</b> sent %d, none %d, shed %d, capped %d",
		chatReactSentCount.Load(), chatReactNoneCount.Load(), chatReactShedCount.Load(), chatReactCappedCount.Load(),
	)
}

// chatReactJob is one unaddressed group message queued for background contextual AI reaction.
type chatReactJob struct {
	bot        *gotgbot.Bot
	chatID     int64
	chatTitle  string
	userID     int64
	messageID  int64
	threadID   int64
	provider   string
	logEnabled bool
	turn       chatprompt.Turn
	quoted     *chatprompt.Turn
	quoteText  string
	media      *chatMediaRef
}

// chatReactGateScript atomically checks and updates three same-slot {<chatID>} keys:
//   - KEYS[1]: presenceAIReactGapKey(chatID)
//   - KEYS[2]: presenceAIReactChatCooldownKey(chatID)
//   - KEYS[3]: presenceUserCooldownKey(chatID, userID) (shared with 24h 👎 retraction cooldown)
//
// ARGV: [1] gapTTLSecs, [2] minGap, [3] chatCooldownSecs, [4] userCooldownSecs, [5] win (0 or 1).
// Existing KEYS[3] TTL (such as a 24h retraction cooldown) is never overwritten.
var chatReactGateScript = redis.NewScript(`
local gap = redis.call('INCR', KEYS[1])
local gapTTL = tonumber(ARGV[1])
if gapTTL and gapTTL > 0 then
	redis.call('EXPIRE', KEYS[1], gapTTL)
end
local minGap = tonumber(ARGV[2]) or 1
if gap < minGap then
	return 0
end
if redis.call('EXISTS', KEYS[2]) == 1 or redis.call('EXISTS', KEYS[3]) == 1 then
	return 0
end
if tonumber(ARGV[5]) ~= 1 then
	return 0
end
if gapTTL and gapTTL > 0 then
	redis.call('SET', KEYS[1], '0', 'EX', gapTTL)
else
	redis.call('SET', KEYS[1], '0')
end
local chatCD = tonumber(ARGV[3])
if chatCD and chatCD > 0 then
	redis.call('SET', KEYS[2], '1', 'EX', chatCD)
end
local userCD = tonumber(ARGV[4])
if userCD and userCD > 0 then
	redis.call('SET', KEYS[3], '1', 'EX', userCD)
end
return 1
`)

func chatPresenceEnsureWorkers() {
	chatPresenceQueue = make(chan chatReactJob, chatPresenceQueueCapacity)
	for range chatPresenceWorkerCount {
		go chatPresenceWorker()
	}
}

func chatPresenceWorker() {
	defer error_handling.RecoverFromPanic("chatPresenceWorker", "Chat")
	for job := range chatPresenceQueue {
		func() {
			defer chatJobs.finish()
			defer error_handling.RecoverFromPanic("chatReactProcess", "Chat")
			if chatContext().Err() != nil {
				chatReactShedCount.Add(1)
				return
			}
			chatReactProcess(job)
		}()
	}
}

// chatPresenceEnqueue hands a reaction job to the dedicated presence lane, shedding silently when
// draining or full without delaying chatQueue.
func chatPresenceEnqueue(job chatReactJob) bool {
	chatEnsureWorkers()
	chatJobs.start()
	if chatDraining.Load() {
		chatJobs.finish()
		chatReactShedCount.Add(1)
		return false
	}
	select {
	case chatPresenceQueue <- job:
		return true
	default:
		chatJobs.finish()
		chatReactShedCount.Add(1)
		log.WithFields(log.Fields{"chat_id": job.chatID, "user_id": job.userID}).Debug("[Chat] presence queue full, reaction shed")
		return false
	}
}

// chatExtractReactionMediaRef extracts only downloadable photos or stickers for reaction vision
// (never PDFs, audio, voice, videos, or generic documents), with zero I/O.
func chatExtractReactionMediaRef(msg *gotgbot.Message) (chatMediaRef, bool) {
	if msg == nil {
		return chatMediaRef{}, false
	}
	switch {
	case len(msg.Photo) > 0:
		if ref, ok := raidAIPhotoRef(msg.Photo); ok {
			return chatMediaRef{FileID: ref.FileID, Kind: chatMediaImage}, true
		}
	case msg.Sticker != nil:
		return chatExtractStickerRef(msg.Sticker)
	}
	return chatMediaRef{}, false
}

func chatReactCooldownSecs(d time.Duration) int {
	if d <= 0 {
		return 0
	}
	secs := int(d / time.Second)
	if secs == 0 {
		return 1
	}
	return secs
}

func chatReactTakeGate(client *redis.Client, chatID, userID int64, profile chatReactProfile, win int) bool {
	ctx, cancel := context.WithTimeout(context.Background(), chatRedisTimeout)
	defer cancel()
	allowed, err := chatReactGateScript.Run(ctx, client, []string{
		presenceAIReactGapKey(chatID),
		presenceAIReactChatCooldownKey(chatID),
		presenceUserCooldownKey(chatID, userID),
	},
		chatReactCooldownSecs(chatReactGapTTL),
		profile.minGap,
		chatReactCooldownSecs(profile.chatCooldown),
		chatReactCooldownSecs(profile.userCooldown),
		win,
	).Int()
	if err != nil {
		log.WithField("chat_id", chatID).Debugf("[Presence] AI reaction gate script failed: %s", chatRedisErrClass(err))
		return false
	}
	return allowed == 1
}

// chatReactWatch evaluates an unaddressed group message from group 13, cheapest checks first,
// preserving zero operational Redis I/O whenever EnableChat is off or AIReactMode is "off".
func chatReactWatch(b *gotgbot.Bot, ctx *ext.Context) {
	if !chatAnyProviderConfigured() {
		return
	}
	if ctx != nil && ctx.Data != nil {
		if reacted, _ := ctx.Data[presenceStaticReactedKey].(bool); reacted {
			return
		}
	}
	userID, ok := presenceMessageAllowed(b, ctx)
	if !ok {
		return
	}
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	text := strings.TrimSpace(chatMessageText(msg))
	mediaRef, hasMedia := chatExtractReactionMediaRef(msg)
	if text == "" && !hasMedia && msg.Sticker == nil {
		return
	}

	lookupCtx, cancel := context.WithTimeout(context.Background(), chatWatchTimeout)
	defer cancel()
	pres, err := presenceSettingsGet(lookupCtx, chat.Id)
	if err != nil || !presence.ValidAIReactMode(pres.AIReactMode) || pres.AIReactMode == presence.AIReactModeOff {
		return
	}
	conv, err := chatSettingsLookup(lookupCtx, chat.Id)
	if err != nil || !conv.Enabled || !chatProviderConfigured(conv.Provider) {
		return
	}
	client := presenceRedisClient()
	if client == nil {
		return
	}
	profile, ok := chatReactProfiles[pres.AIReactMode]
	if !ok {
		return
	}
	win := 0
	if chatReactRoll(profile.chancePct) {
		win = 1
	}
	if !chatReactTakeGate(client, chat.Id, userID, profile, win) {
		return
	}

	turnText := text
	if turnText == "" {
		turnText = chatMediaLabel(msg)
	}
	job := chatReactJob{
		bot:        b,
		chatID:     chat.Id,
		chatTitle:  chat.Title,
		userID:     userID,
		messageID:  msg.MessageId,
		provider:   conv.Provider,
		logEnabled: conv.LogEnabled,
		turn: chatprompt.Turn{
			Text: turnText,
		},
	}
	if msg.From != nil {
		job.turn.Name = msg.From.FirstName
	}
	if msg.IsTopicMessage {
		job.threadID = msg.MessageThreadId
	}
	if reply := extraction.ReplyTarget(msg); reply != nil {
		quotedText := strings.TrimSpace(chatMessageText(reply))
		if quotedText == "" {
			quotedText = chatMediaLabel(reply)
		}
		if quotedText != "" {
			quoted := chatprompt.Turn{Text: quotedText}
			if reply.From != nil {
				quoted.Name = reply.From.FirstName
			}
			job.quoted = &quoted
		}
	}
	if msg.Quote != nil {
		job.quoteText = strings.TrimSpace(msg.Quote.Text)
	}
	if hasMedia {
		ref := mediaRef
		job.media = &ref
	}
	chatPresenceEnqueue(job)
}

func chatReactProcessJob(_ chatReactJob) {
	chatReactShedCount.Add(1)
}
