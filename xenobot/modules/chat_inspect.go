package modules

import (
	"context"
	"errors"
	"html"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/utils/chatprompt"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/extraction"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/helpers"
)

const (
	chatInspectSaidMalfunction = "inspect:malfunction"
	chatInspectSaidAllowance   = "inspect:allowance"
)

type chatInspectJob struct {
	bot            *gotgbot.Bot
	chatID         int64
	chatTitle      string
	userID         int64
	messageID      int64
	targetMsgID    int64
	targetAuthorID int64
	threadID       int64
	provider       string
	target         chatprompt.Turn
	quoted         *chatprompt.Turn
	quoteText      string
	mediaDesc      string
	media          *chatMediaRef
}

func (j chatInspectJob) asChat() chatJob {
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

func (j chatInspectJob) asReactJob() chatReactJob {
	return chatReactJob{
		bot:      j.bot,
		chatID:   j.chatID,
		userID:   j.userID,
		provider: j.provider,
		media:    j.media,
	}
}

// inspectCommand answers /inspect in groups when replying to a text, photo, sticker, or image
// document with a deadpan 2-3 line Facility Inspection verdict and a validated bot_react emoji.
func (moduleStruct) inspectCommand(c *helpers.CommandContext) error {
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

	reply := extraction.ReplyTarget(msg)
	if reply == nil {
		chatReplyText(c, "inspect_usage")
		return ext.EndGroups
	}

	gate := chatToyGateOf(c.Bot, chat.Id, userID, chatToyInspect)
	if !gate.open {
		if gate.replyKey != "" {
			chatReplyText(c, gate.replyKey)
		}
		return ext.EndGroups
	}

	job, ok := chatInspectJobFromReply(c.Bot, msg, reply, userID, gate)
	if !ok {
		chatReplyText(c, "inspect_usage")
		return ext.EndGroups
	}

	chatToysEnqueue(chatToyTask{
		chatID: chat.Id,
		userID: userID,
		run:    func() { chatInspectProcess(job) },
	})
	return ext.EndGroups
}

// chatInspectReplyMedia is the picture /inspect shows the model: a photo, a sticker, the preview
// frame of a GIF, video or round video message, or an image file. A PDF or text file is not one.
func chatInspectReplyMedia(reply *gotgbot.Message) (chatMediaRef, bool) {
	if ref, ok := chatExtractReactionMediaRef(reply); ok {
		return ref, true
	}
	if docRef, ok := chatExtractMediaRef(reply); ok && docRef.Kind == chatMediaImage {
		return docRef, true
	}
	return chatMediaRef{}, false
}

// chatInspectMediaDesc is the media section of the /inspect prompt. A preview frame says so, and a
// picture that could not be attached says that too, so the verdict never describes what the model
// was not shown.
func chatInspectMediaDesc(label string, ref chatMediaRef, hasMedia, unseen bool) string {
	switch {
	case hasMedia && ref.Frame != chatFrameNone:
		return strings.TrimSpace(label + " — " + chatprompt.FrameNote(1, 1, ref.Frame.noun()))
	case unseen:
		return strings.TrimSpace(label + " — " + chatprompt.UnseenPictureNote)
	default:
		return label
	}
}

func chatInspectThreadID(msg, reply *gotgbot.Message) int64 {
	if msg.IsTopicMessage {
		return msg.MessageThreadId
	}
	if reply.IsTopicMessage {
		return reply.MessageThreadId
	}
	return 0
}

func chatInspectQuotedTurn(b *gotgbot.Bot, reply *gotgbot.Message) *chatprompt.Turn {
	nested := extraction.ReplyTarget(reply)
	if nested == nil || (nested.From != nil && b != nil && nested.From.Id == b.Id) {
		return nil
	}
	qText := strings.TrimSpace(chatMessageText(nested))
	if qText == "" {
		qText = chatMediaLabel(nested)
	}
	if qText == "" {
		return nil
	}
	q := &chatprompt.Turn{Text: qText}
	if nested.From != nil {
		q.Name = nested.From.FirstName
	}
	return q
}

func chatInspectJobFromReply(b *gotgbot.Bot, msg, reply *gotgbot.Message, userID int64, gate chatToyGate) (chatInspectJob, bool) {
	targetText := strings.TrimSpace(chatMessageText(reply))
	mediaLabel := chatMediaLabel(reply)
	mediaRef, hasMedia := chatInspectReplyMedia(reply)
	if targetText == "" && mediaLabel == "" && !hasMedia {
		return chatInspectJob{}, false
	}
	// A picture the bot cannot get (a video with no preview frame, an image over the size cap)
	// leaves only its label: with no words to go on there is nothing to judge, and a verdict about
	// a picture the model never saw would be invented.
	unseen := !hasMedia && chatMessageHasPicture(reply)
	if unseen && targetText == "" {
		return chatInspectJob{}, false
	}

	turnText := targetText
	if turnText == "" {
		turnText = mediaLabel
	}
	var targetName string
	var targetAuthorID int64
	if reply.From != nil {
		targetName = reply.From.FirstName
		if reply.SenderChat == nil {
			targetAuthorID = reply.From.Id
		}
	}

	job := chatInspectJob{
		bot:            b,
		chatID:         msg.Chat.Id,
		chatTitle:      msg.Chat.Title,
		userID:         userID,
		messageID:      msg.MessageId,
		targetMsgID:    reply.MessageId,
		targetAuthorID: targetAuthorID,
		threadID:       chatInspectThreadID(msg, reply),
		provider:       gate.provider,
		target: chatprompt.Turn{
			Name: targetName,
			Text: turnText,
		},
		mediaDesc: chatInspectMediaDesc(mediaLabel, mediaRef, hasMedia, unseen),
		quoted:    chatInspectQuotedTurn(b, reply),
	}
	if reply.Quote != nil {
		job.quoteText = strings.TrimSpace(reply.Quote.Text)
	}
	if hasMedia {
		ref := mediaRef
		job.media = &ref
	}
	return job, true
}

func chatInspectProcess(job chatInspectJob) {
	root := chatContext()
	ctx, cancel := context.WithTimeout(root, chatToyJobTimeout)
	defer cancel()

	client := presenceRedisClient()
	if client == nil || !chatProviderConfigured(job.provider) {
		chatToyShedCount.Add(1)
		return
	}
	if presenceIsStaffGroup(job.chatID) || presenceIsLockedDown(job.chatID) {
		chatToyShedCount.Add(1)
		return
	}
	if !chatInspectReserve(ctx, client, job) {
		return
	}

	chatSendTyping(ctx, job.bot, job.asChat())

	palette, _ := presenceAllowedPalette(ctx, job.bot, job.chatID)
	if root.Err() != nil {
		chatToyShedCount.Add(1)
		return
	}
	media := chatReactDownloadVision(ctx, job.asReactJob())
	if root.Err() != nil {
		chatToyShedCount.Add(1)
		return
	}
	if job.media != nil && len(media.Attachments) == 0 {
		// The picture was expected and could not be fetched. A verdict from its label alone would be
		// invented, so say so with the malfunction line (the provider is healthy: not a breaker failure).
		chatToyFailedCount.Add(1)
		chatSayGated(ctx, job.bot, client, job.asChat(), chatInspectSaidMalfunction, "inspect_malfunction", chatMalfunctionGate)
		return
	}

	_, language := chatLanguageName(ctx, job.chatID)
	req := chatRequest{
		Static:      chatprompt.InspectStatic(),
		Dynamic:     chatprompt.InspectDynamic(job.chatTitle, language, palette),
		User:        chatprompt.InspectUser(job.target, job.quoted, job.quoteText, job.mediaDesc),
		MaxTokens:   chatprompt.InspectMaxTokens,
		Attachments: media.Attachments,
	}
	answer, err := chatConverse(ctx, root, chatCall{Lane: "inspect", ChatID: job.chatID}, job.provider, req)
	if err != nil {
		if root.Err() != nil || errors.Is(err, context.Canceled) {
			chatToyShedCount.Add(1)
			return
		}
		chatToyFailedCount.Add(1)
		chatRecordFailure(job.asChat(), err)
		chatSayGated(ctx, job.bot, client, job.asChat(), chatInspectSaidMalfunction, "inspect_malfunction", chatMalfunctionGate)
		return
	}
	chatBreaker(job.provider).clear(job.chatID)

	if answer.TextOnly {
		// The model rejected the image and only a retry without it answered. A verdict about a picture
		// it never saw would be invented, so say so with the malfunction line instead. Not a breaker
		// failure: the provider is healthy, the model just cannot see.
		chatToyFailedCount.Add(1)
		chatSayGated(ctx, job.bot, client, job.asChat(), chatInspectSaidMalfunction, "inspect_malfunction", chatMalfunctionGate)
		return
	}
	if answer.Outcome == chatRefused {
		chatInspectReplyRefusal(ctx, job)
		return
	}
	parsed, ok := chatprompt.ParseInspectJSON(answer.Text, palette)
	if !ok {
		chatToyFailedCount.Add(1)
		chatSayGated(ctx, job.bot, client, job.asChat(), chatInspectSaidMalfunction, "inspect_malfunction", chatMalfunctionGate)
		return
	}
	verdict := chatCutRunes(chatSanitizeText(parsed.Verdict), chatprompt.InspectVerdictMaxRunes)
	if verdict == "" {
		chatToyFailedCount.Add(1)
		chatSayGated(ctx, job.bot, client, job.asChat(), chatInspectSaidMalfunction, "inspect_malfunction", chatMalfunctionGate)
		return
	}

	chatInspectSendVerdict(ctx, root, job, parsed, verdict)
}

func chatInspectReserve(ctx context.Context, client *redis.Client, job chatInspectJob) bool {
	// The per-chat cooldown is shared with /referendum (AGENTS.md: both toys use chatToyChatCooldown),
	// so several members cannot each run /inspect back to back and flood the chat with verdicts.
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
		chatSayGated(ctx, job.bot, client, job.asChat(), chatInspectSaidMalfunction, "inspect_malfunction", chatMalfunctionGate)
		return false
	}
	switch chatReserveWith(client, job.chatID, job.userID, chatToyBudgetFamily,
		chatToyUserHourlyBudget, chatToyChatHourlyBudget, chatToyGlobalHourlyBudget) {
	case chatBudgetReserved:
		return true
	case chatBudgetCapped:
		chatToyCappedCount.Add(1)
		chatSayGated(ctx, job.bot, client, job.asChat(), chatInspectSaidAllowance, "inspect_allowance", chatAllowanceGate)
		return false
	default:
		chatToyShedCount.Add(1)
		return false
	}
}

func chatInspectReplyRefusal(ctx context.Context, job chatInspectJob) {
	chatToyRefusedCount.Add(1)
	text := chatCannedText(job.chatID, "inspect_refused")
	if text == "" {
		return
	}
	if _, err := chatSendReply(ctx, job.bot, job.asChat(), text); err != nil {
		log.WithFields(log.Fields{"chat_id": job.chatID, "user_id": job.userID}).Debugf("[Chat] inspect refusal line could not be sent: %s", chatSendErrClass(err))
	}
}

func chatInspectSendVerdict(ctx, root context.Context, job chatInspectJob, parsed chatprompt.InspectResult, verdict string) {
	body := html.EscapeString(verdict)
	if header := chatCannedText(job.chatID, "inspect_header"); header != "" {
		body = header + "\n" + body
	}
	err := chatPaced(ctx, func(callCtx context.Context) error {
		_, sendErr := job.bot.SendMessageWithContext(callCtx, job.chatID, body, &gotgbot.SendMessageOpts{
			ParseMode:          "HTML",
			MessageThreadId:    job.threadID,
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId:                job.targetMsgID,
				AllowSendingWithoutReply: false,
			},
		})
		return sendErr
	})
	if err != nil {
		if root.Err() != nil || errors.Is(err, context.Canceled) || chatReplyTargetGone(err) {
			chatToyShedCount.Add(1)
			return
		}
		chatToyFailedCount.Add(1)
		log.WithFields(log.Fields{"chat_id": job.chatID, "user_id": job.userID}).Warnf("[Chat] inspect reply failed: %s", chatSendErrClass(err))
		return
	}
	chatToyAnsweredCount.Add(1)

	if parsed.EmojiValid && parsed.Emoji != "" {
		presenceSetReaction(ctx, job.bot, job.chatID, job.targetMsgID, job.targetAuthorID, parsed.Emoji, "inspect")
	}
}
