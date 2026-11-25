package modules

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/db/chatconv"
	"github.com/uasneppy/Xenobot-72/xenobot/db/lang"
	"github.com/uasneppy/Xenobot-72/xenobot/db/staff"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/chatprompt"
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
	// chatBriefUnscoredNameRunes is how many runes of names the not-scored line lists at most.
	chatBriefUnscoredNameRunes = 300
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
	// dest is where a /judge verdict goes (chatconv.JudgeToGroup, JudgeToPrivate or JudgeToStaff);
	// briefs always go to the group.
	dest int
	// requester is the first name of who asked for a verdict, for the header outside the group.
	requester string
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
	// judgeTo is the group's stored default destination of a /judge verdict.
	judgeTo int
	open    bool
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
	return chatBriefGate{provider: settings.Provider, judgeTo: settings.JudgeTo, open: true}
}

// chatBriefCommand is the body of /unrollshort and /unrolllong: it prepares the job and queues it.
func chatBriefCommand(c *helpers.CommandContext, kind chatBriefKind, usageKey string) error {
	if job, _, ok := chatBriefPrepare(c, kind, usageKey); ok {
		chatBriefEnqueue(job)
	}
	return ext.EndGroups
}

// chatBriefPrepare is the shared part of every brief command. It checks, in order: a group, a
// sender that counts and is not a bot (anonymous admins and channels get nothing), the replied-to
// message (none answers the usage), the gate, and that the replied-to message is in the log (absent
// answers brief_too_old and makes no provider call). It returns the job and the open gate.
func chatBriefPrepare(c *helpers.CommandContext, kind chatBriefKind, usageKey string) (chatBriefJob, chatBriefGate, bool) {
	msg, chat := c.Msg, c.Chat
	if chat.Type != "group" && chat.Type != "supergroup" {
		return chatBriefJob{}, chatBriefGate{}, false
	}
	userID, ok := raidFloodSenderCounts(c.Bot, c.Ctx)
	if !ok || c.User.IsBot {
		return chatBriefJob{}, chatBriefGate{}, false
	}
	reply := extraction.ReplyTarget(msg)
	if reply == nil {
		chatReplyText(c, usageKey)
		return chatBriefJob{}, chatBriefGate{}, false
	}
	gate := chatBriefGateOf(chat.Id)
	if !gate.open {
		if gate.replyKey != "" {
			chatReplyText(c, gate.replyKey)
		}
		return chatBriefJob{}, gate, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), chatWatchTimeout)
	defer cancel()
	present, err := chatLogHas(ctx, chatRedisClient(), chat.Id, reply.MessageId)
	if err != nil {
		log.WithField("chat_id", chat.Id).Debugf("[Chat] the message log could not be read: %s", chatRedisErrClass(err))
		return chatBriefJob{}, gate, false
	}
	if !present {
		chatReplyText(c, "brief_too_old")
		return chatBriefJob{}, gate, false
	}
	job := chatBriefJob{
		bot: c.Bot, chatID: chat.Id, chatTitle: chat.Title, userID: userID, messageID: msg.MessageId,
		fromID: reply.MessageId, provider: gate.provider, kind: kind,
	}
	if msg.IsTopicMessage {
		job.threadID = msg.MessageThreadId
	}
	return job, gate, true
}

// unrollShort answers /unrollshort.
func (moduleStruct) unrollShort(c *helpers.CommandContext) error {
	return chatBriefCommand(c, chatBriefShort, "brief_usage_short")
}

// unrollLong answers /unrolllong.
func (moduleStruct) unrollLong(c *helpers.CommandContext) error {
	return chatBriefCommand(c, chatBriefLong, "brief_usage_long")
}

// chatJudgeStaffLink reads the group's Staff Group link fresh from the database; a variable only so
// a test can make it fail.
var chatJudgeStaffLink = staff.GetLinkOfGroupFresh

// chatJudgeDestination reads the optional argument of /judge: none is the stored default (a value
// outside the stored ones reads as the group), and one
// word of group, pm or staff (in any case) overrides it. Anything else is not a destination.
func chatJudgeDestination(argument string, stored int) (int, bool) {
	words := strings.Fields(argument)
	switch {
	case len(words) == 0 && chatconv.ValidJudgeTo(stored):
		return stored, true
	case len(words) == 0:
		return chatconv.JudgeToGroup, true
	case len(words) > 1:
		return 0, false
	}
	switch strings.ToLower(words[0]) {
	case "group":
		return chatconv.JudgeToGroup, true
	case "pm":
		return chatconv.JudgeToPrivate, true
	case "staff":
		return chatconv.JudgeToStaff, true
	}
	return 0, false
}

// judgeCommand answers /judge. Its authority is the descriptor's live check of the sender
// (requireLockdownAuthority(false)), which also sends the anonymous proof prompt, so this body is
// never entered for an anonymous first message. After the shared brief checks it reads the
// destination (an unknown or extra argument answers the usage and spends nothing) and, for the
// Staff Group, reads the link fresh: an error or no link answers in the group and makes no AI call.
func (moduleStruct) judgeCommand(c *helpers.CommandContext) error {
	job, gate, ok := chatBriefPrepare(c, chatBriefJudge, "judge_usage")
	if !ok {
		return ext.EndGroups
	}
	dest, ok := chatJudgeDestination(chatCommandArgument(c.Msg.Text), gate.judgeTo)
	if !ok {
		chatReplyText(c, "judge_usage")
		return ext.EndGroups
	}
	if dest == chatconv.JudgeToStaff {
		link, err := chatJudgeStaffLink(c.Chat.Id)
		if err != nil {
			chatReplyText(c, "judge_staff_check_failed")
			return ext.EndGroups
		}
		if link == nil {
			chatReplyText(c, "judge_no_staff")
			return ext.EndGroups
		}
	}
	job.dest = dest
	job.requester = c.User.FirstName
	chatBriefEnqueue(job)
	return ext.EndGroups
}

// judgeAnon is the re-entry of /judge after an anonymous admin's proof tap. anonPipelineHandler has
// already re-run the descriptor's checks, the live check of the tapper included.
func judgeAnon(b *gotgbot.Bot, ctx *ext.Context) error {
	c, err := helpers.BuildCommandContext(b, ctx)
	if err != nil {
		return ext.EndGroups
	}
	return chatModule.judgeCommand(c)
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
	chatBriefFinish(ctx, job, client, stretch, answer)
}

// chatBriefFinish turns the provider's answer into messages and delivers them: a refusal answers
// brief_refused, a blank answer brief_malfunction, and anything else is split, escaped, framed by
// the kind's header and closing block and sent to the job's destination.
func chatBriefFinish(ctx context.Context, job chatBriefJob, client *redis.Client, stretch []chatLogEntry, answer chatAnswer) {
	chat := job.asChat()
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
	header, closing := chatBriefFrame(job, stretch)
	messages := chatBriefMessages(header, parts, closing)
	switch job.dest {
	case chatconv.JudgeToPrivate:
		chatJudgeDeliverPrivate(ctx, job, client, messages)
	case chatconv.JudgeToStaff:
		chatJudgeDeliverStaff(ctx, job, messages)
	default:
		chatBriefDeliver(ctx, job, messages)
	}
	log.WithFields(log.Fields{"chat_id": job.chatID, "user_id": job.userID}).Debugf("[Chat] brief written (%d input, %d output tokens)", answer.InputTokens, answer.OutputTokens)
}

// chatCannedTextWith is a canned line in the group's language with its placeholders filled; every
// value must already be escaped.
func chatCannedTextWith(chatID int64, key string, params i18n.TranslationParams) string {
	tr := i18n.MustNewTranslator(lang.GetGroupLanguage(context.Background(), chatID))
	text, err := tr.GetString(key, params)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(text)
}

// chatBriefFrame is the header and the closing block of a kind. A verdict sent outside the group
// names the group and who asked for it; its closing block names the participants left off the
// roster (chatBriefParticipants) and always ends with the AI-opinion notice.
func chatBriefFrame(job chatBriefJob, stretch []chatLogEntry) (header, closing string) {
	if job.kind != chatBriefJudge {
		headerKey := "brief_header_short"
		if job.kind == chatBriefLong {
			headerKey = "brief_header_long"
		}
		return chatCannedText(job.chatID, headerKey), chatCannedText(job.chatID, "brief_ai_notice")
	}
	header = chatCannedText(job.chatID, "judge_header")
	if job.dest != chatconv.JudgeToGroup {
		header = chatCannedTextWith(job.chatID, "judge_header_other", i18n.TranslationParams{
			"group": html.EscapeString(chatprompt.Clean(job.chatTitle, 64)),
			"name":  html.EscapeString(chatprompt.Clean(job.requester, chatprompt.MaxNameRunes)),
		})
	}
	closing = chatCannedText(job.chatID, "judge_ai_notice")
	if _, unscored := chatBriefParticipants(stretch); len(unscored) > 0 {
		names := chatJudgeUnscoredNames(job.chatID, unscored)
		closing = chatCannedTextWith(job.chatID, "judge_unscored_note", i18n.TranslationParams{"names": names}) + "\n\n" + closing
	}
	return header, closing
}

// chatJudgeUnscoredNames joins the escaped labels of the participants left off the roster with a
// comma and a space, up to chatBriefUnscoredNameRunes runes, and names the count of the rest.
func chatJudgeUnscoredNames(chatID int64, unscored []string) string {
	var shown []string
	used := 0
	for _, name := range unscored {
		cost := utf8.RuneCountInString(name) + 2
		if len(shown) > 0 && used+cost > chatBriefUnscoredNameRunes {
			break
		}
		shown = append(shown, html.EscapeString(name))
		used += cost
	}
	names := strings.Join(shown, ", ")
	if rest := len(unscored) - len(shown); rest > 0 {
		names += " " + chatCannedTextWith(chatID, "judge_unscored_more", i18n.TranslationParams{"count": rest})
	}
	return names
}

// chatBriefDeliver posts the messages in order through the chat pacer: the first as a reply to the
// command (allowed to survive its deletion), later ones as plain messages in the same topic. A send
// error is counted and logged by class only, and stops the rest.
func chatBriefDeliver(ctx context.Context, job chatBriefJob, messages []string) {
	if err := chatBriefSendAll(ctx, job, job.chatID, job.threadID, true, messages); err != nil {
		chatBriefFailedCount.Add(1)
		log.WithFields(log.Fields{"chat_id": job.chatID, "user_id": job.userID}).Warnf("[Chat] the brief could not be sent: %s", chatSendErrClass(err))
		return
	}
	chatBriefAnsweredCount.Add(1)
}

// chatJudgeDeliverPrivate sends a verdict to the requester's own private chat (the user ID from the
// update's sender, never from text) with no reply target and no topic. A 403 (the requester never
// started the bot or blocked it) answers judge_pm_blocked in the group and nothing else; any other
// error is counted failed with brief_malfunction. A delivered verdict leaves one note in the group.
func chatJudgeDeliverPrivate(ctx context.Context, job chatBriefJob, client *redis.Client, messages []string) {
	err := chatBriefSendAll(ctx, job, job.userID, 0, false, messages)
	switch {
	case err == nil:
		chatBriefAnsweredCount.Add(1)
		chatBriefSay(ctx, job, "judge_note_pm")
	case settingsEditIsForbidden(err):
		chatBriefFailedCount.Add(1)
		chatBriefSay(ctx, job, "judge_pm_blocked")
	default:
		chatBriefFailedCount.Add(1)
		log.WithFields(log.Fields{"chat_id": job.chatID, "user_id": job.userID}).Warnf("[Chat] the verdict could not be sent in private: %s", chatSendErrClass(err))
		chatSayGated(ctx, job.bot, client, job.asChat(), chatBriefSaidMalfunction, "brief_malfunction", chatMalfunctionGate)
	}
}

// chatJudgeDeliverStaff sends a verdict to the linked Staff Group, read fresh again right before the
// send (the chat ID comes from that row and never from text). No link answers judge_no_staff in the
// group; a failed read judge_staff_check_failed; a failed send judge_staff_send_failed. A delivered
// verdict leaves one note in the group that names no Staff Group.
func chatJudgeDeliverStaff(ctx context.Context, job chatBriefJob, messages []string) {
	link, err := chatJudgeStaffLink(job.chatID)
	switch {
	case err != nil:
		chatBriefFailedCount.Add(1)
		chatBriefSay(ctx, job, "judge_staff_check_failed")
		return
	case link == nil:
		chatBriefFailedCount.Add(1)
		chatBriefSay(ctx, job, "judge_no_staff")
		return
	}
	if err := chatBriefSendAll(ctx, job, link.StaffChatID, 0, false, messages); err != nil {
		chatBriefFailedCount.Add(1)
		log.WithFields(log.Fields{"chat_id": job.chatID, "user_id": job.userID}).Warnf("[Chat] the verdict could not be sent to the Staff Group: %s", chatSendErrClass(err))
		chatBriefSay(ctx, job, "judge_staff_send_failed")
		return
	}
	chatBriefAnsweredCount.Add(1)
	chatBriefSay(ctx, job, "judge_note_staff")
}

// chatBriefSendAll posts the messages in order to one chat and stops at the first error. Only the
// first message of a reply goes as a reply to the command.
func chatBriefSendAll(ctx context.Context, job chatBriefJob, chatID, threadID int64, asReply bool, messages []string) error {
	for i, text := range messages {
		if err := chatBriefSend(ctx, job, chatID, threadID, text, asReply && i == 0); err != nil {
			return err
		}
	}
	return nil
}

// chatBriefSend posts one HTML message of a brief with link previews off.
func chatBriefSend(ctx context.Context, job chatBriefJob, chatID, threadID int64, text string, asReply bool) error {
	opts := &gotgbot.SendMessageOpts{
		ParseMode:          "HTML",
		MessageThreadId:    threadID,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
	}
	if asReply {
		opts.ReplyParameters = &gotgbot.ReplyParameters{MessageId: job.messageID, AllowSendingWithoutReply: true}
	}
	return chatPaced(ctx, func(callCtx context.Context) error {
		_, err := job.bot.SendMessageWithContext(callCtx, chatID, text, opts)
		return err
	})
}
