package modules

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/utils/error_handling"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/extraction"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/helpers"
)

// The brief commands (/unrollshort): a member replies to a message and the bot posts an AI brief of
// that message and every logged message after it. The messages come from the group's rolling
// message log (chat_log.go), which exists only while the owner's Message log switch is on. A brief
// reuses the conversation runtime: the group's stored provider and its adapters, the provider's
// breaker, the chat pacer and the pool's root context, so StopChat ends a brief too and no
// shutdown drain is added. It has its own queue and one dedicated worker, so briefs never starve
// conversation replies, and its own cooldown and hourly budgets.

const (
	// chatBriefQueueCapacity is how many brief jobs wait at most; a full queue sheds.
	chatBriefQueueCapacity = 16
	// chatBriefWorkerCount is how many briefs are prepared at once on a replica.
	chatBriefWorkerCount = 1
	// chatBriefSaidMalfunction and chatBriefSaidAllowance gate the canned lines of briefs apart
	// from the conversation's own lines.
	chatBriefSaidMalfunction = "brief_malfunction"
	chatBriefSaidAllowance   = "brief_allowance"
)

var (
	// chatBriefCooldown is the least time between two briefs asked by one member in one group.
	chatBriefCooldown = 20 * time.Second
	// chatBriefUserHourlyBudget, chatBriefChatHourlyBudget and chatBriefGlobalHourlyBudget are the
	// briefs one member may ask in one group, one group may get and the whole bot may give in one
	// UTC hour. Variables only so a test can shrink them.
	chatBriefUserHourlyBudget   int64 = 6
	chatBriefChatHourlyBudget   int64 = 30
	chatBriefGlobalHourlyBudget int64 = 300

	chatBriefQueue chan chatBriefJob
	// chatBriefProcess runs one job; a variable only so a test can install a gated one, and so the
	// worker's goroutine reaches the job function through a variable like chatProcess.
	chatBriefProcess = chatBriefProcessJob
)

// The in-memory counters of briefs, per replica, for the developer /stats.
var (
	chatBriefAnsweredCount atomic.Int64
	chatBriefRefusedCount  atomic.Int64
	chatBriefFailedCount   atomic.Int64
	chatBriefShedCount     atomic.Int64
	chatBriefCappedCount   atomic.Int64
)

// chatBriefStatsSection is the brief part of the developer /stats: numbers only.
func chatBriefStatsSection() string {
	return fmt.Sprintf(
		"\n    <b>Briefs:</b> answered %d, refused %d, failed %d, shed %d, capped %d",
		chatBriefAnsweredCount.Load(), chatBriefRefusedCount.Load(), chatBriefFailedCount.Load(),
		chatBriefShedCount.Load(), chatBriefCappedCount.Load(),
	)
}

// chatBriefJob is one brief waiting to be written. It holds IDs only for Telegram and the guards.
type chatBriefJob struct {
	bot       *gotgbot.Bot
	chatID    int64
	chatTitle string
	userID    int64
	// messageID is the command message the brief answers.
	messageID int64
	threadID  int64
	// fromID is the message the stretch starts at.
	fromID   int64
	provider string
	kind     chatBriefKind
}

// asChat is the job as the conversation helpers take it: the command message is the reply target,
// and a reply survives the command's deletion.
func (j chatBriefJob) asChat() chatJob {
	return chatJob{
		bot: j.bot, chatID: j.chatID, chatTitle: j.chatTitle, userID: j.userID, messageID: j.messageID,
		threadID: j.threadID, provider: j.provider, allowMissingReply: true,
	}
}

// chatBriefGate is what the server and the group's settings say about a brief command.
type chatBriefGate struct {
	// provider is the stored provider when the gate is open.
	provider string
	// replyKey is the locale line to answer with when the gate is closed; "" answers nothing.
	replyKey string
	open     bool
}

// chatBriefGateOf applies the checks in order: the server can run conversations at all, the cached
// setting (an error answers nothing), the log switch, the stored provider and Redis (no Redis
// answers nothing). It judges no person's rights: a brief is for everyone.
func chatBriefGateOf(chatID int64) chatBriefGate {
	if !chatAnyProviderConfigured() {
		return chatBriefGate{replyKey: "brief_unavailable"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), chatWatchTimeout)
	defer cancel()
	settings, err := chatSettingsLookup(ctx, chatID)
	if err != nil {
		log.WithField("chat_id", chatID).Debug("[Chat] the conversation setting could not be read, no brief")
		return chatBriefGate{}
	}
	switch {
	case !settings.LogEnabled:
		return chatBriefGate{replyKey: "brief_off_hint"}
	case !chatProviderConfigured(settings.Provider):
		return chatBriefGate{replyKey: "brief_unavailable"}
	case chatRedisClient() == nil:
		return chatBriefGate{}
	}
	return chatBriefGate{provider: settings.Provider, open: true}
}

// chatBriefCommand is the body of /unrollshort. The handler checks, in order: a group, a sender
// that counts and is not a bot (anonymous admins and channels get nothing), the replied-to message
// (none answers the usage), the gate, and that the replied-to message is in the log (absent
// answers brief_too_old and makes no provider call); then it queues the job.
func chatBriefCommand(c *helpers.CommandContext, kind chatBriefKind, usageKey string) error {
	msg, chat := c.Msg, c.Chat
	if chat.Type != "group" && chat.Type != "supergroup" {
		return ext.EndGroups
	}
	userID, ok := raidFloodSenderCounts(c.Bot, c.Ctx)
	if !ok || c.User.IsBot {
		return ext.EndGroups
	}
	reply := extraction.ReplyTarget(msg)
	if reply == nil {
		chatReplyText(c, usageKey)
		return ext.EndGroups
	}
	gate := chatBriefGateOf(chat.Id)
	if !gate.open {
		if gate.replyKey != "" {
			chatReplyText(c, gate.replyKey)
		}
		return ext.EndGroups
	}
	ctx, cancel := context.WithTimeout(context.Background(), chatWatchTimeout)
	defer cancel()
	present, err := chatLogHas(ctx, chatRedisClient(), chat.Id, reply.MessageId)
	if err != nil {
		log.WithField("chat_id", chat.Id).Debugf("[Chat] the message log could not be read: %s", chatRedisErrClass(err))
		return ext.EndGroups
	}
	if !present {
		chatReplyText(c, "brief_too_old")
		return ext.EndGroups
	}
	job := chatBriefJob{
		bot: c.Bot, chatID: chat.Id, chatTitle: chat.Title, userID: userID, messageID: msg.MessageId,
		fromID: reply.MessageId, provider: gate.provider, kind: kind,
	}
	if msg.IsTopicMessage {
		job.threadID = msg.MessageThreadId
	}
	chatBriefEnqueue(job)
	return ext.EndGroups
}

// unrollShort answers /unrollshort.
func (moduleStruct) unrollShort(c *helpers.CommandContext) error {
	return chatBriefCommand(c, chatBriefShort, "brief_usage_short")
}

func chatBriefEnsureWorkers() {
	chatBriefQueue = make(chan chatBriefJob, chatBriefQueueCapacity)
	for range chatBriefWorkerCount {
		go chatBriefWorker()
	}
}

func chatBriefWorker() {
	defer error_handling.RecoverFromPanic("chatBriefWorker", "Chat")
	for job := range chatBriefQueue {
		func() {
			defer chatJobs.finish()
			defer error_handling.RecoverFromPanic("chatBriefProcess", "Chat")
			if chatContext().Err() != nil {
				chatBriefShedCount.Add(1)
				return
			}
			chatBriefProcess(job)
		}()
	}
}

// chatBriefEnqueue hands a job to the brief worker, shedding it when the queue is full or the
// process is draining. A shed job gets no reply and one Debug line at most.
func chatBriefEnqueue(job chatBriefJob) bool {
	chatEnsureWorkers()
	chatJobs.start()
	if chatDraining.Load() {
		chatJobs.finish()
		chatBriefShedCount.Add(1)
		return false
	}
	select {
	case chatBriefQueue <- job:
		return true
	default:
		chatJobs.finish()
		chatBriefShedCount.Add(1)
		log.WithFields(log.Fields{"chat_id": job.chatID, "user_id": job.userID}).Debug("[Chat] brief queue full, command shed")
		return false
	}
}

// chatBriefCooldownKey is one member's brief cooldown in one group.
func chatBriefCooldownKey(chatID, userID int64) string {
	return chatKeyPrefix + "brief:cd:{" + strconv.FormatInt(chatID, 10) + "}:" + strconv.FormatInt(userID, 10)
}

// chatBriefSay answers the command with one canned line in the group's language, as a reply that
// survives the command's deletion.
func chatBriefSay(ctx context.Context, job chatBriefJob, key string) {
	text := chatCannedText(job.chatID, key)
	if text == "" {
		return
	}
	if _, err := chatSendReply(ctx, job.bot, job.asChat(), text); err != nil {
		log.WithFields(log.Fields{"chat_id": job.chatID, "user_id": job.userID}).Debugf("[Chat] the %s line could not be sent: %s", key, chatSendErrClass(err))
	}
}

// chatBriefProcessJob writes and sends one brief, in order: Redis and the provider still
// available, the cooldown (a loser is silent), the provider's breaker, the hourly budgets, the
// typing action, the stretch from the log, the request, the provider call, the answer's text and
// its delivery.
func chatBriefProcessJob(job chatBriefJob) {
	root := chatContext()
	ctx, cancel := context.WithTimeout(root, chatJobTimeout)
	defer cancel()
	chat := job.asChat()

	client := chatRedisClient()
	if client == nil || !chatProviderConfigured(job.provider) {
		chatBriefShedCount.Add(1)
		return
	}
	if !chatClaimCooldown(ctx, client, chatBriefCooldownKey(job.chatID, job.userID), chatBriefCooldown) {
		chatBriefShedCount.Add(1)
		return
	}
	breaker := chatBreaker(job.provider)
	if breaker.isPaused(job.chatID, raidNow()) {
		chatBriefShedCount.Add(1)
		chatSayGated(ctx, job.bot, client, chat, chatBriefSaidMalfunction, "brief_malfunction", chatMalfunctionGate)
		return
	}
	switch chatReserveWith(client, job.chatID, job.userID, "brief:b",
		chatBriefUserHourlyBudget, chatBriefChatHourlyBudget, chatBriefGlobalHourlyBudget) {
	case chatBudgetCapped:
		chatBriefCappedCount.Add(1)
		chatSayGated(ctx, job.bot, client, chat, chatBriefSaidAllowance, "brief_allowance", chatAllowanceGate)
		return
	case chatBudgetFault:
		chatBriefShedCount.Add(1)
		return
	}

	chatSendTyping(ctx, job.bot, chat)
	entries, err := chatLogSince(ctx, client, job.chatID, job.fromID)
	if err != nil {
		chatBriefShedCount.Add(1)
		log.WithField("chat_id", job.chatID).Debugf("[Chat] the message log could not be read: %s", chatRedisErrClass(err))
		return
	}
	stretch := chatBriefStretch(entries, job.fromID)
	if stretch == nil {
		chatBriefSay(ctx, job, "brief_too_old")
		return
	}
	_, language := chatLanguageName(ctx, job.chatID)
	date := time.Now().In(chatClock(ctx, job.chatID)).Format("2006-01-02")
	request := chatBriefRequest(job.kind, stretch, job.chatTitle, language, date)

	answer, err := chatProviderOf(job.provider).converse(ctx, request)
	if err != nil {
		if root.Err() != nil || errors.Is(err, context.Canceled) {
			chatBriefShedCount.Add(1)
			return
		}
		chatBriefFailedCount.Add(1)
		chatRecordFailure(chat, err)
		chatSayGated(ctx, job.bot, client, chat, chatBriefSaidMalfunction, "brief_malfunction", chatMalfunctionGate)
		return
	}
	breaker.clear(job.chatID)
	if answer.Outcome == chatRefused {
		chatBriefRefusedCount.Add(1)
		chatBriefSay(ctx, job, "brief_refused")
		return
	}
	text := chatSanitizeText(answer.Text)
	if text != "" && answer.Outcome == chatTruncated {
		text += "…"
	}
	parts := chatBriefParts(text, chatBriefPartUnits)
	if len(parts) == 0 {
		chatBriefFailedCount.Add(1)
		chatSayGated(ctx, job.bot, client, chat, chatBriefSaidMalfunction, "brief_malfunction", chatMalfunctionGate)
		return
	}
	for i := range parts {
		parts[i] = html.EscapeString(parts[i])
	}
	messages := chatBriefMessages(chatCannedText(job.chatID, "brief_header_short"), parts, chatCannedText(job.chatID, "brief_ai_notice"))
	chatBriefDeliver(ctx, job, messages)
	log.WithFields(log.Fields{"chat_id": job.chatID, "user_id": job.userID}).Debugf("[Chat] brief written (%d input, %d output tokens)", answer.InputTokens, answer.OutputTokens)
}

// chatBriefDeliver posts the messages in order through the chat pacer: the first as a reply to the
// command (allowed to survive its deletion), later ones as plain messages in the same topic. A send
// error is counted and logged by class only, and stops the rest.
func chatBriefDeliver(ctx context.Context, job chatBriefJob, messages []string) {
	for i, text := range messages {
		if err := chatBriefSend(ctx, job, text, i == 0); err != nil {
			chatBriefFailedCount.Add(1)
			log.WithFields(log.Fields{"chat_id": job.chatID, "user_id": job.userID}).Warnf("[Chat] the brief could not be sent: %s", chatSendErrClass(err))
			return
		}
	}
	chatBriefAnsweredCount.Add(1)
}

// chatBriefSend posts one HTML message of a brief with link previews off.
func chatBriefSend(ctx context.Context, job chatBriefJob, text string, asReply bool) error {
	opts := &gotgbot.SendMessageOpts{
		ParseMode:          "HTML",
		MessageThreadId:    job.threadID,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
	}
	if asReply {
		opts.ReplyParameters = &gotgbot.ReplyParameters{MessageId: job.messageID, AllowSendingWithoutReply: true}
	}
	return chatPaced(ctx, func(callCtx context.Context) error {
		_, err := job.bot.SendMessageWithContext(callCtx, job.chatID, text, opts)
		return err
	})
}
