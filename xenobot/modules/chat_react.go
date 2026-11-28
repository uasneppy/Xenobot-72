package modules

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/db/chatconv"
	"github.com/uasneppy/Xenobot-72/xenobot/db/presence"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/chatprompt"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/error_handling"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/extraction"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/featherless"
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
	// userCooldownSecs is the per-user cooldown the gate started, so a job that never reacts can
	// give it back (chatReactReleaseCooldowns).
	userCooldownSecs int
	// topics limit the reaction to messages about them (the group's admins set them); empty is no
	// limit. A NONE answer to a topic-limited job is an off-topic message, not a turn: the job gives
	// its cooldowns back, so the next message is tried, within the hourly budgets.
	topics []string
}

// chatReactKindOf is the kind of message a reaction frequency is set for.
func chatReactKindOf(msg *gotgbot.Message) string {
	switch {
	case msg.Sticker != nil:
		return presence.KindSticker
	case len(msg.Photo) > 0:
		return presence.KindPhoto
	}
	return presence.KindText
}

// chatReactScaleProfile applies a group's frequency for one kind of message to the AI reaction
// mode's own throttle: Rare halves the chance and doubles the gap and both cooldowns, Often doubles
// the chance (at most 100) and halves them. Normal (the default) and Off (which never gets here)
// leave the profile as it is.
func chatReactScaleProfile(p chatReactProfile, freq int) chatReactProfile {
	switch freq {
	case presence.ReactFreqRare:
		p.minGap *= 2
		p.chatCooldown *= 2
		p.userCooldown *= 2
		p.chancePct = max(1, p.chancePct/2)
	case presence.ReactFreqOften:
		p.minGap = max(1, p.minGap/2)
		p.chatCooldown /= 2
		p.userCooldown /= 2
		p.chancePct = min(100, p.chancePct*2)
	}
	return p
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

// chatReactReleaseScript gives back the cooldowns the gate started for a job that never reacted:
//   - KEYS[1]: presenceAIReactChatCooldownKey(chatID)
//   - KEYS[2]: presenceUserCooldownKey(chatID, userID)
//
// ARGV[1] is the user cooldown the gate set. KEYS[2] is shared with the 24 h thumbs-down retraction
// cooldown, so it is deleted only while its TTL is no longer than the gate's own (a longer TTL
// belongs to a retraction that landed meanwhile).
var chatReactReleaseScript = redis.NewScript(`
redis.call('DEL', KEYS[1])
local ttl = redis.call('TTL', KEYS[2])
if ttl >= 0 and ttl <= tonumber(ARGV[1]) then
	redis.call('DEL', KEYS[2])
end
return 1
`)

// chatReactReleaseCooldowns gives the chat and user cooldowns back after a job that was shed,
// capped, failed or answered nothing usable. The gate starts them when it enqueues, so without this
// a dropped job would still cost the chat 8 to 15 quiet minutes. Best effort, on its own context.
func chatReactReleaseCooldowns(client *redis.Client, job chatReactJob) {
	if client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), chatRedisTimeout)
	defer cancel()
	if err := chatReactReleaseScript.Run(ctx, client, []string{
		presenceAIReactChatCooldownKey(job.chatID),
		presenceUserCooldownKey(job.chatID, job.userID),
	}, job.userCooldownSecs).Err(); err != nil {
		log.WithField("chat_id", job.chatID).Debugf("[Presence] AI reaction cooldown release failed: %s", chatRedisErrClass(err))
	}
}

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

func chatReactBuildJob(
	b *gotgbot.Bot,
	msg *gotgbot.Message,
	tgChat *gotgbot.Chat,
	userID int64,
	conv chatconv.Settings,
	text string,
	mediaRef chatMediaRef,
	hasMedia bool,
) chatReactJob {
	turnText := text
	if turnText == "" {
		turnText = chatMediaLabel(msg)
	}
	job := chatReactJob{
		bot:        b,
		chatID:     tgChat.Id,
		chatTitle:  tgChat.Title,
		userID:     userID,
		messageID:  msg.MessageId,
		provider:   conv.Provider,
		logEnabled: conv.LogEnabled,
		turn:       chatprompt.Turn{Text: turnText},
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
	return job
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
	// The group's frequency for this kind of message is decided before any Redis I/O: Off never
	// reacts to it, Rare and Often scale the throttle.
	kind := chatReactKindOf(msg)
	freq := pres.ReactFreqOf(kind)
	if freq == presence.ReactFreqOff {
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
	profile = chatReactScaleProfile(profile, freq)
	win := 0
	if chatReactRoll(profile.chancePct) {
		win = 1
	}
	if !chatReactTakeGate(client, chat.Id, userID, profile, win) {
		return
	}

	job := chatReactBuildJob(b, msg, chat, userID, conv, text, mediaRef, hasMedia)
	job.userCooldownSecs = chatReactCooldownSecs(profile.userCooldown)
	job.topics = pres.TopicList()
	if !chatPresenceEnqueue(job) {
		chatReactReleaseCooldowns(client, job)
	}
}

func chatIsFeatherlessVisionReject(err error) bool {
	var fErr *featherless.HTTPError
	return errors.As(err, &fErr) &&
		(fErr.Status == http.StatusBadRequest || fErr.Status == http.StatusUnprocessableEntity) &&
		!fErr.KeyRejected()
}

func chatReactRecentTurns(ctx context.Context, client *redis.Client, job chatReactJob) []chatprompt.Turn {
	if !job.logEnabled {
		return nil
	}
	entries, err := chatLogRecentThread(ctx, client, job.chatID, job.threadID, job.messageID, chatReactRecentLogLimit)
	if err != nil {
		log.WithField("chat_id", job.chatID).Debugf("[Presence] recent thread log lookup skipped: %s", chatRedisErrClass(err))
		return nil
	}
	if len(entries) == 0 {
		return nil
	}
	turns := make([]chatprompt.Turn, 0, len(entries))
	for _, entry := range entries {
		var text string
		switch {
		case entry.Media != "" && entry.Text != "":
			text = entry.Media + " " + entry.Text
		case entry.Text != "":
			text = entry.Text
		default:
			text = entry.Media
		}
		if text == "" {
			continue
		}
		turns = append(turns, chatprompt.Turn{Name: entry.Name, Text: text})
	}
	return turns
}

func chatReactDownloadVision(ctx context.Context, job chatReactJob) chatDownloadedMedia {
	if job.media == nil {
		return chatDownloadedMedia{}
	}
	return chatDownloadMedia(ctx, job.bot, job.chatID, job.userID, job.provider, []chatMediaRef{*job.media})
}

// chatReactApplyAnswer acts on the model's answer and reports whether the job used its turn: a
// reaction was set, or the model deliberately chose none. A refusal, a parse error and a failed
// set are not a turn, so the caller gives the cooldowns back.
func chatReactApplyAnswer(ctx context.Context, job chatReactJob, palette []string, answer chatAnswer) bool {
	if answer.Outcome == chatRefused {
		chatReactNoneCount.Add(1)
		return false
	}
	emoji, status := chatprompt.ParseReactionJSON(answer.Text, palette)
	switch status {
	case chatprompt.ReactionParseNone:
		chatReactNoneCount.Add(1)
		return len(job.topics) == 0
	case chatprompt.ReactionParseEmoji:
		if presenceSetReaction(ctx, job.bot, job.chatID, job.messageID, job.userID, emoji, "ai") {
			chatReactSentCount.Add(1)
			return true
		}
		chatReactShedCount.Add(1)
		return false
	default:
		log.WithFields(log.Fields{"chat_id": job.chatID, "user_id": job.userID}).Debug("[Presence] AI reaction parse error")
		chatReactShedCount.Add(1)
		return false
	}
}

// chatReactProcessJob runs one background contextual AI reaction job under chatContext().
// It enforces lazy fail-closed lockdown, read-only breaker status, dedicated "react:b" budgets,
// allowed chat palette, same-thread context + photo/sticker vision with Featherless 400/422
// text-only fallback, and silent non-tripping error shedding (never mutating chatBreakers).
func chatReactProcessJob(job chatReactJob) {
	root := chatContext()
	ctx, cancel := context.WithTimeout(root, chatReactJobTimeout)
	defer cancel()

	client := presenceRedisClient()
	// The gate started the chat and user cooldowns when it enqueued this job. A job that ends
	// without reacting (shed, capped, a provider error, a refusal, a failed set) gives them back, so
	// only a real turn costs the chat its quiet period.
	used := false
	defer func() {
		if !used {
			chatReactReleaseCooldowns(client, job)
		}
	}()
	if client == nil || !chatProviderConfigured(job.provider) {
		chatReactShedCount.Add(1)
		return
	}
	if presenceIsLockedDown(job.chatID) {
		chatReactShedCount.Add(1)
		return
	}
	if chatBreaker(job.provider).isPaused(job.chatID, raidNow()) {
		chatReactShedCount.Add(1)
		return
	}
	switch chatReserveWith(client, job.chatID, job.userID, "react:b",
		chatReactUserHourlyBudget, chatReactChatHourlyBudget, chatReactGlobalHourlyBudget) {
	case chatBudgetCapped:
		chatReactCappedCount.Add(1)
		return
	case chatBudgetFault:
		chatReactShedCount.Add(1)
		return
	}

	palette, ok := presenceAllowedPalette(ctx, job.bot, job.chatID)
	if !ok || len(palette) == 0 {
		chatReactShedCount.Add(1)
		return
	}

	recentTurns := chatReactRecentTurns(ctx, client, job)
	media := chatReactDownloadVision(ctx, job)
	if root.Err() != nil {
		chatReactShedCount.Add(1)
		return
	}

	_, language := chatLanguageName(ctx, job.chatID)
	req := chatRequest{
		Static:      chatprompt.ReactionStatic(),
		Dynamic:     chatprompt.ReactionDynamic(job.chatTitle, language, palette, job.topics),
		User:        chatprompt.ReactionUser(recentTurns, job.quoted, job.quoteText, job.turn),
		MaxTokens:   chatprompt.ReactionMaxTokens,
		Attachments: media.Attachments,
	}

	answer, err := chatConverse(ctx, root, chatCall{Lane: "reaction", ChatID: job.chatID}, job.provider, req)
	if err != nil {
		log.WithFields(log.Fields{"chat_id": job.chatID, "user_id": job.userID}).Debugf("[Presence] AI reaction provider error: %s", chatErrorClass(err))
		chatReactShedCount.Add(1)
		return
	}
	used = chatReactApplyAnswer(ctx, job, palette, answer)
}
