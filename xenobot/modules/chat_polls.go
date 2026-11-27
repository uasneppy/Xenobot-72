package modules

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"strings"
	"sync/atomic"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/db/lang"
	"github.com/uasneppy/Xenobot-72/xenobot/db/presence"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/chatprompt"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/error_handling"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/extraction"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/helpers"
)

const (
	// chatToyQueueCapacity bounds queued Interactive Group Toy commands (/referendum, /inspect).
	chatToyQueueCapacity = 16
	// chatToyWorkerCount runs toy AI calls without starving /ask or /unroll* lanes.
	chatToyWorkerCount = 2
	// chatToyJobTimeout bounds one queued toy job or poll tie-breaker execution.
	chatToyJobTimeout = 60 * time.Second
	// chatReferendumOpenPeriod is Telegram's auto-close period for /referendum polls (15 minutes).
	chatReferendumOpenPeriod int64 = 900
	// chatPollMetaTTL outlives chatReferendumOpenPeriod so closed-poll tie-breakers resolve cleanly.
	chatPollMetaTTL = 25 * time.Hour
	// chatPollTieClaimTTL bounds the once-only cross-replica tie-breaker latch per poll ID.
	chatPollTieClaimTTL = 24 * time.Hour

	chatToyBudgetFamily           = "toy:b"
	chatReferendumSaidMalfunction = "referendum:malfunction"
	chatReferendumSaidAllowance   = "referendum:allowance"
)

var (
	// Toy cooldown and hourly budget caps; package variables only so tests can adjust them.
	chatToyChatCooldown       = 30 * time.Second
	chatToyUserCooldown       = 20 * time.Second
	chatToyUserHourlyBudget   int64 = 6
	chatToyChatHourlyBudget   int64 = 30
	chatToyGlobalHourlyBudget int64 = 300

	chatToyQueue   chan chatToyTask
	chatToyProcess = chatToyRunTask
)

// Per-replica Interactive Group Toys counters exposed via developer /stats.
var (
	chatToyAnsweredCount    atomic.Int64
	chatToyRefusedCount     atomic.Int64
	chatToyFailedCount      atomic.Int64
	chatToyShedCount        atomic.Int64
	chatToyCappedCount      atomic.Int64
	chatPollTiesBrokenCount atomic.Int64
)

// chatToyTask is one queued Interactive Group Toy job joining StopChat.
type chatToyTask struct {
	chatID int64
	userID int64
	run    func()
}

type chatToyKind int

const (
	chatToyReferendum chatToyKind = iota
	chatToyInspect
)

type chatToyGate struct {
	provider   string
	logEnabled bool
	replyKey   string
	open       bool
}

type chatReferendumJob struct {
	bot       *gotgbot.Bot
	chatID    int64
	chatTitle string
	userID    int64
	messageID int64
	threadID  int64
	provider  string
	reply     *chatprompt.Turn
	recent    []chatprompt.Turn
	topic     string
}

func (j chatReferendumJob) asChat() chatJob {
	return chatJob{
		bot:               j.bot,
		chatID:            j.chatID,
		chatTitle:         j.chatTitle,
		userID:            j.userID,
		messageID:         j.messageID,
		threadID:          j.threadID,
		provider:          j.provider,
		allowMissingReply: true,
	}
}

type chatPollMeta struct {
	ChatID    int64  `json:"chat_id"`
	MessageID int64  `json:"message_id"`
	ThreadID  int64  `json:"thread_id,omitempty"`
	Lang      string `json:"lang"`
	Question  string `json:"question"`
}

// chatToysStatsSection formats the Interactive Group Toys counters for developer /stats.
func chatToysStatsSection() string {
	return fmt.Sprintf(
		"\n    <b>Toys:</b> answered %d, refused %d, failed %d, shed %d, capped %d, ties_broken %d",
		chatToyAnsweredCount.Load(), chatToyRefusedCount.Load(), chatToyFailedCount.Load(),
		chatToyShedCount.Load(), chatToyCappedCount.Load(), chatPollTiesBrokenCount.Load(),
	)
}

func chatToysEnsureWorkers() {
	chatToyQueue = make(chan chatToyTask, chatToyQueueCapacity)
	for range chatToyWorkerCount {
		go chatToyWorker()
	}
}

func chatToyWorker() {
	defer error_handling.RecoverFromPanic("chatToyWorker", "Chat")
	for task := range chatToyQueue {
		func() {
			defer chatJobs.finish()
			defer error_handling.RecoverFromPanic("chatToyProcess", "Chat")
			if chatContext().Err() != nil {
				chatToyShedCount.Add(1)
				return
			}
			chatToyProcess(task)
		}()
	}
}

func chatToyRunTask(task chatToyTask) {
	if task.run != nil {
		task.run()
	}
}

// chatToysEnqueue hands a toy task to the shared StopChat toy lane, shedding silently when
// draining or full without delaying chatQueue.
func chatToysEnqueue(task chatToyTask) bool {
	chatEnsureWorkers()
	chatJobs.start()
	if chatDraining.Load() {
		chatJobs.finish()
		chatToyShedCount.Add(1)
		return false
	}
	select {
	case chatToyQueue <- task:
		return true
	default:
		chatJobs.finish()
		chatToyShedCount.Add(1)
		log.WithFields(log.Fields{"chat_id": task.chatID, "user_id": task.userID}).Debug("[Chat] toy queue full, command shed")
		return false
	}
}

func chatToyGateKeys(kind chatToyKind) (offKey, unavailKey string) {
	if kind == chatToyInspect {
		return "inspect_off_hint", "inspect_unavailable"
	}
	return "referendum_off_hint", "referendum_unavailable"
}

func chatToyFeatureEnabled(pres presence.Settings, kind chatToyKind) bool {
	if kind == chatToyInspect {
		return pres.InspectEnabled
	}
	return pres.ReferendumEnabled
}

// chatToyGateOf enforces Owner Conversations + configured provider, Admin Presence toggle,
// Audience membership, and Redis availability for /referendum and /inspect.
func chatToyGateOf(b *gotgbot.Bot, chatID, userID int64, kind chatToyKind) chatToyGate {
	offKey, unavailKey := chatToyGateKeys(kind)
	if !chatAnyProviderConfigured() {
		return chatToyGate{replyKey: unavailKey}
	}
	ctx, cancel := context.WithTimeout(context.Background(), chatWatchTimeout)
	defer cancel()
	conv, err := chatSettingsLookup(ctx, chatID)
	if err != nil {
		log.WithField("chat_id", chatID).Debug("[Chat] conversation settings lookup failed, toy skipped")
		return chatToyGate{}
	}
	pres, err := presenceSettingsGet(ctx, chatID)
	if err != nil {
		log.WithField("chat_id", chatID).Debug("[Presence] presence settings lookup failed, toy skipped")
		return chatToyGate{}
	}
	if !chatAudienceAllows(ctx, b, conv.Audience, chatID, userID) {
		return chatToyGate{}
	}
	if !conv.Enabled || !chatToyFeatureEnabled(pres, kind) {
		return chatToyGate{replyKey: offKey}
	}
	if !chatProviderConfigured(conv.Provider) {
		return chatToyGate{replyKey: unavailKey}
	}
	if presenceRedisClient() == nil {
		return chatToyGate{}
	}
	return chatToyGate{provider: conv.Provider, logEnabled: conv.LogEnabled, open: true}
}

// referendumCommand answers /referendum [topic] in groups by drafting a native Telegram poll from
// the replied debate message, recent same-thread log context, and/or command topic argument.
func (moduleStruct) referendumCommand(c *helpers.CommandContext) error {
	msg, chat := c.Msg, c.Chat
	if msg == nil || !presenceChatTypeAllowed(chat) {
		return ext.EndGroups
	}
	userID, ok := raidFloodSenderCounts(c.Bot, c.Ctx)
	if !ok || c.User == nil || c.User.IsBot {
		return ext.EndGroups
	}
	if presenceIsStaffGroup(chat.Id) || presenceIsLockedDown(chat.Id) {
		return ext.EndGroups
	}

	gate := chatToyGateOf(c.Bot, chat.Id, userID, chatToyReferendum)
	if !gate.open {
		if gate.replyKey != "" {
			chatReplyText(c, gate.replyKey)
		}
		return ext.EndGroups
	}

	var threadID int64
	if msg.IsTopicMessage {
		threadID = msg.MessageThreadId
	}
	replyTurn, replyMsgID := chatReferendumReplyTurn(msg)
	topic := strings.TrimSpace(chatprompt.StripMention(chatCommandArgument(chatMessageText(msg)), c.Bot.Username))
	recent := chatReferendumRecentTurns(chat.Id, threadID, msg.MessageId, replyMsgID, gate.logEnabled)

	if replyTurn == nil && topic == "" && len(recent) == 0 {
		chatReplyText(c, "referendum_need_context")
		return ext.EndGroups
	}

	job := chatReferendumJob{
		bot:       c.Bot,
		chatID:    chat.Id,
		chatTitle: chat.Title,
		userID:    userID,
		messageID: msg.MessageId,
		threadID:  threadID,
		provider:  gate.provider,
		reply:     replyTurn,
		recent:    recent,
		topic:     topic,
	}
	chatToysEnqueue(chatToyTask{
		chatID: chat.Id,
		userID: userID,
		run:    func() { chatReferendumProcess(job) },
	})
	return ext.EndGroups
}

func chatReferendumRecentTurns(chatID, threadID, msgID, replyMsgID int64, logEnabled bool) []chatprompt.Turn {
	if !logEnabled {
		return nil
	}
	client := presenceRedisClient()
	if client == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), chatLogReadTimeout)
	entries, err := chatLogRecent(ctx, client, chatID, threadID, msgID, chatAmbientContextLimit)
	cancel()
	if err != nil {
		log.WithField("chat_id", chatID).Debugf("[Chat] referendum thread log lookup skipped: %s", chatRedisErrClass(err))
		return nil
	}
	return chatReferendumTurnsFromLog(entries, msgID, replyMsgID)
}

func chatReferendumReplyTurn(msg *gotgbot.Message) (*chatprompt.Turn, int64) {
	reply := extraction.ReplyTarget(msg)
	if reply == nil {
		return nil, 0
	}
	text := strings.TrimSpace(chatMessageText(reply))
	mediaLabel := chatMediaLabel(reply)
	switch {
	case mediaLabel != "" && text != "":
		text = mediaLabel + " " + text
	case text == "":
		text = mediaLabel
	}
	if text == "" {
		return nil, reply.MessageId
	}
	turn := chatprompt.Turn{Text: text}
	if reply.From != nil {
		turn.Name = reply.From.FirstName
	}
	return &turn, reply.MessageId
}

func chatReferendumTurnsFromLog(entries []chatLogEntry, cmdMsgID, replyMsgID int64) []chatprompt.Turn {
	if len(entries) == 0 {
		return nil
	}
	turns := make([]chatprompt.Turn, 0, len(entries))
	for _, entry := range entries {
		if entry.MessageID == cmdMsgID || (replyMsgID > 0 && entry.MessageID == replyMsgID) {
			continue
		}
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

func chatReferendumProcess(job chatReferendumJob) {
	root := chatContext()
	ctx, cancel := context.WithTimeout(root, chatToyJobTimeout)
	defer cancel()

	client := presenceRedisClient()
	if client == nil || !chatProviderConfigured(job.provider) {
		chatToyShedCount.Add(1)
		return
	}
	if !chatReferendumReserve(ctx, client, job) {
		return
	}

	chatSendTyping(ctx, job.bot, job.asChat())
	langCode, language := chatLanguageName(ctx, job.chatID)
	req := chatRequest{
		Static:    chatprompt.ReferendumStatic(),
		Dynamic:   chatprompt.ReferendumDynamic(job.chatTitle, language),
		User:      chatprompt.ReferendumUser(job.reply, job.recent, job.topic),
		MaxTokens: chatprompt.ReferendumMaxTokens,
	}
	answer, err := chatProviderOf(job.provider).converse(ctx, req)
	if err != nil {
		if root.Err() != nil || errors.Is(err, context.Canceled) {
			chatToyShedCount.Add(1)
			return
		}
		chatToyFailedCount.Add(1)
		chatRecordFailure(job.asChat(), err)
		chatSayGated(ctx, job.bot, client, job.asChat(), chatReferendumSaidMalfunction, "referendum_malfunction", chatMalfunctionGate)
		return
	}
	chatBreaker(job.provider).clear(job.chatID)

	if answer.Outcome == chatRefused {
		chatReferendumReplyRefusal(ctx, job)
		return
	}
	draft, ok := chatprompt.ParseReferendumJSON(answer.Text)
	if !ok {
		chatToyFailedCount.Add(1)
		chatSayGated(ctx, job.bot, client, job.asChat(), chatReferendumSaidMalfunction, "referendum_malfunction", chatMalfunctionGate)
		return
	}
	sanitized, ok := chatSanitizeReferendumDraft(draft)
	if !ok {
		chatToyFailedCount.Add(1)
		chatSayGated(ctx, job.bot, client, job.asChat(), chatReferendumSaidMalfunction, "referendum_malfunction", chatMalfunctionGate)
		return
	}

	chatReferendumSendPoll(ctx, root, client, job, langCode, sanitized)
}

func chatReferendumReserve(ctx context.Context, client *redis.Client, job chatReferendumJob) bool {
	if chatToyChatCooldown > 0 && !chatClaimCooldown(ctx, client, presenceToyChatCooldownKey(job.chatID), chatToyChatCooldown) {
		chatToyShedCount.Add(1)
		return false
	}
	if chatToyUserCooldown > 0 && !chatClaimCooldown(ctx, client, presenceToyUserCooldownKey(job.chatID, job.userID), chatToyUserCooldown) {
		chatToyShedCount.Add(1)
		return false
	}
	if chatBreaker(job.provider).isPaused(job.chatID, raidNow()) {
		chatToyShedCount.Add(1)
		chatSayGated(ctx, job.bot, client, job.asChat(), chatReferendumSaidMalfunction, "referendum_malfunction", chatMalfunctionGate)
		return false
	}
	switch chatReserveWith(client, job.chatID, job.userID, chatToyBudgetFamily,
		chatToyUserHourlyBudget, chatToyChatHourlyBudget, chatToyGlobalHourlyBudget) {
	case chatBudgetReserved:
		return true
	case chatBudgetCapped:
		chatToyCappedCount.Add(1)
		chatSayGated(ctx, job.bot, client, job.asChat(), chatReferendumSaidAllowance, "referendum_allowance", chatAllowanceGate)
		return false
	default:
		chatToyShedCount.Add(1)
		return false
	}
}

func chatReferendumReplyRefusal(ctx context.Context, job chatReferendumJob) {
	chatToyRefusedCount.Add(1)
	text := chatCannedText(job.chatID, "referendum_refused")
	if text == "" {
		return
	}
	if _, err := chatSendReply(ctx, job.bot, job.asChat(), text); err != nil {
		log.WithFields(log.Fields{"chat_id": job.chatID, "user_id": job.userID}).Debugf("[Chat] referendum refusal line could not be sent: %s", chatSendErrClass(err))
	}
}

func chatReferendumSendPoll(ctx, root context.Context, client *redis.Client, job chatReferendumJob, langCode string, sanitized chatprompt.ReferendumDraft) {
	pollOpts := make([]gotgbot.InputPollOption, len(sanitized.Options))
	for i, opt := range sanitized.Options {
		pollOpts[i] = gotgbot.InputPollOption{Text: opt}
	}
	isAnon := false
	var sent *gotgbot.Message
	err := chatPaced(ctx, func(callCtx context.Context) error {
		var callErr error
		sent, callErr = job.bot.SendPollWithContext(callCtx, job.chatID, sanitized.Question, pollOpts, &gotgbot.SendPollOpts{
			MessageThreadId:     job.threadID,
			IsAnonymous:         &isAnon,
			Type:                "regular",
			OpenPeriod:          chatReferendumOpenPeriod,
			AllowAddingOptions:  false,
			DisableNotification: true,
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId:                job.messageID,
				AllowSendingWithoutReply: true,
			},
		})
		return callErr
	})
	if err != nil {
		if root.Err() != nil || errors.Is(err, context.Canceled) {
			chatToyShedCount.Add(1)
			return
		}
		chatToyFailedCount.Add(1)
		log.WithFields(log.Fields{"chat_id": job.chatID, "user_id": job.userID}).Warnf("[Chat] referendum sendPoll failed: %s", chatSendErrClass(err))
		return
	}
	chatToyAnsweredCount.Add(1)

	if sent == nil || sent.Poll == nil || sent.Poll.Id == "" {
		return
	}
	meta := chatPollMeta{
		ChatID:    job.chatID,
		MessageID: sent.MessageId,
		ThreadID:  job.threadID,
		Lang:      langCode,
		Question:  sanitized.Question,
	}
	raw, mErr := json.Marshal(meta)
	if mErr != nil {
		return
	}
	wCtx, wCancel := context.WithTimeout(ctx, chatRedisTimeout)
	defer wCancel()
	if sErr := client.Set(wCtx, presencePollKey(sent.Poll.Id), string(raw), chatPollMetaTTL).Err(); sErr != nil {
		log.WithField("chat_id", job.chatID).Debugf("[Chat] poll metadata write failed: %s", chatRedisErrClass(sErr))
	}
}

func chatSanitizeReferendumDraft(draft chatprompt.ReferendumDraft) (chatprompt.ReferendumDraft, bool) {
	question := chatprompt.Clean(chatSanitizeText(draft.Question), chatprompt.ReferendumQuestionMaxRunes)
	if question == "" {
		return chatprompt.ReferendumDraft{}, false
	}
	options := make([]string, 0, len(draft.Options))
	seen := make(map[string]struct{}, len(draft.Options))
	for _, opt := range draft.Options {
		cleaned := chatprompt.Clean(chatSanitizeText(opt), chatprompt.ReferendumOptionMaxRunes)
		if cleaned == "" {
			continue
		}
		key := strings.ToLower(cleaned)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		options = append(options, cleaned)
		if len(options) == chatprompt.ReferendumMaxOptions {
			break
		}
	}
	if len(options) < chatprompt.ReferendumMinOptions {
		return chatprompt.ReferendumDraft{}, false
	}
	return chatprompt.ReferendumDraft{Question: question, Options: options}, true
}

// handlePollUpdate is the group-0 Update.Poll handler. Every execution joins StopChat
// (chatJobs + chatDraining + chatContext()) so SIGTERM cancels in-flight tie-breakers without
// adding a shutdown drain.
func (moduleStruct) handlePollUpdate(b *gotgbot.Bot, ctx *ext.Context) (result error) {
	result = ext.EndGroups
	defer error_handling.RecoverFromPanic("handlePollUpdate", "Chat")

	chatJobs.start()
	if chatDraining.Load() {
		chatJobs.finish()
		return ext.EndGroups
	}
	defer chatJobs.finish()

	runCtx, cancel := context.WithTimeout(chatContext(), chatToyJobTimeout)
	defer cancel()

	chatHandlePollUpdate(runCtx, b, ctx)
	return ext.EndGroups
}

func chatHandlePollUpdate(runCtx context.Context, b *gotgbot.Bot, ctx *ext.Context) {
	if b == nil || ctx == nil || ctx.Poll == nil {
		return
	}
	p := ctx.Poll
	tiedOptions, maxVotes := chatPollTiedOptions(p)
	if len(tiedOptions) < 2 {
		return
	}
	meta, ok := chatPollClaimTie(runCtx, p.Id)
	if !ok {
		return
	}

	langCode := meta.Lang
	if langCode == "" {
		langCode = lang.GetGroupLanguage(runCtx, meta.ChatID)
	}
	verdictHTML, aborted := chatPollTieBreakerVerdict(runCtx, meta, p, tiedOptions, maxVotes, langCode)
	if aborted || runCtx.Err() != nil || chatContext().Err() != nil {
		return
	}
	if verdictHTML == "" {
		verdictHTML = chatCannedTextWithLang(langCode, "referendum_tiebreaker_fallback")
		if verdictHTML == "" {
			return
		}
	}

	body := verdictHTML
	if header := chatCannedTextWithLang(langCode, "referendum_tiebreaker_header"); header != "" {
		body = header + "\n" + verdictHTML
	}
	chatPollSendTieBreaker(runCtx, b, meta, body)
}

func chatPollTiedOptions(p *gotgbot.Poll) ([]string, int64) {
	if !p.IsClosed || p.TotalVoterCount <= 0 || p.Id == "" {
		return nil, 0
	}
	var maxVotes int64
	for _, opt := range p.Options {
		if opt.VoterCount > maxVotes {
			maxVotes = opt.VoterCount
		}
	}
	if maxVotes <= 0 {
		return nil, 0
	}
	var tied []string
	for _, opt := range p.Options {
		if opt.VoterCount == maxVotes {
			tied = append(tied, opt.Text)
		}
	}
	if len(tied) < 2 {
		return nil, 0
	}
	return tied, maxVotes
}

func chatPollClaimTie(runCtx context.Context, pollID string) (chatPollMeta, bool) {
	client := presenceRedisClient()
	if client == nil {
		return chatPollMeta{}, false
	}
	rCtx, rCancel := context.WithTimeout(runCtx, chatRedisTimeout)
	rawMeta, err := client.Get(rCtx, presencePollKey(pollID)).Result()
	rCancel()
	if err != nil {
		return chatPollMeta{}, false
	}
	var meta chatPollMeta
	if json.Unmarshal([]byte(rawMeta), &meta) != nil || meta.ChatID == 0 || meta.MessageID == 0 {
		return chatPollMeta{}, false
	}
	if presenceIsStaffGroup(meta.ChatID) || presenceIsLockedDown(meta.ChatID) {
		return chatPollMeta{}, false
	}
	cCtx, cCancel := context.WithTimeout(runCtx, chatRedisTimeout)
	won, err := client.SetNX(cCtx, presencePollTieKey(pollID), "1", chatPollTieClaimTTL).Result()
	cCancel()
	if err != nil || !won {
		return chatPollMeta{}, false
	}
	return meta, true
}

func chatPollSendTieBreaker(runCtx context.Context, b *gotgbot.Bot, meta chatPollMeta, body string) {
	err := chatPaced(runCtx, func(callCtx context.Context) error {
		_, sendErr := b.SendMessageWithContext(callCtx, meta.ChatID, body, &gotgbot.SendMessageOpts{
			ParseMode:          "HTML",
			MessageThreadId:    meta.ThreadID,
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId:                meta.MessageID,
				AllowSendingWithoutReply: false,
			},
		})
		return sendErr
	})
	if err != nil {
		if !chatReplyTargetGone(err) {
			log.WithField("chat_id", meta.ChatID).Debugf("[Chat] poll tie-breaker could not be sent: %s", chatSendErrClass(err))
		}
		return
	}
	chatPollTiesBrokenCount.Add(1)
}

func chatPollTieBreakerVerdict(runCtx context.Context, meta chatPollMeta, p *gotgbot.Poll, tiedOptions []string, maxVotes int64, langCode string) (string, bool) {
	conv, convErr := chatSettingsLookup(runCtx, meta.ChatID)
	pres, presErr := presenceSettingsGet(runCtx, meta.ChatID)
	if convErr != nil || presErr != nil || !conv.Enabled || !chatProviderConfigured(conv.Provider) || !pres.ReferendumEnabled {
		return "", false
	}
	if chatBreaker(conv.Provider).isPaused(meta.ChatID, raidNow()) {
		return "", false
	}
	question := meta.Question
	if strings.TrimSpace(p.Question) != "" {
		question = p.Question
	}
	req := chatRequest{
		Static:    chatprompt.TieBreakerStatic(),
		Dynamic:   chatprompt.TieBreakerDynamic("", chatLanguageDisplayName(langCode)),
		User:      chatprompt.TieBreakerUser(question, tiedOptions, maxVotes, p.TotalVoterCount),
		MaxTokens: chatprompt.TieBreakerMaxTokens,
	}
	answer, err := chatProviderOf(conv.Provider).converse(runCtx, req)
	if err != nil {
		if chatContext().Err() != nil || errors.Is(err, context.Canceled) {
			return "", true
		}
		log.WithField("chat_id", meta.ChatID).Debugf("[Chat] poll tie-breaker AI error, using fallback: %s", chatErrorClass(err))
		return "", false
	}
	chatBreaker(conv.Provider).clear(meta.ChatID)
	if answer.Outcome == chatRefused {
		return "", false
	}
	cleaned := chatFilterReply(chatprompt.ParseTieBreakerText(answer.Text))
	if cleaned == "" {
		return "", false
	}
	return html.EscapeString(cleaned), false
}

func chatLanguageDisplayName(code string) string {
	name, err := i18n.MustNewTranslator(code).GetString("language_name")
	if err != nil || strings.TrimSpace(name) == "" {
		return code
	}
	return name
}

func chatCannedTextWithLang(code, key string) string {
	tr := i18n.MustNewTranslator(code)
	text, err := tr.GetString(key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(text)
}
