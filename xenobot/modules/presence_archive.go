package modules

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/db/lang"
	"github.com/uasneppy/Xenobot-72/xenobot/db/presence"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/actionlog"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/chat_status"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/helpers"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/tracing"
)

var (
	// presenceArchiveClaim atomically claims (chat_id, message_id) in PostgreSQL so each
	// message is archived at most once across restarts and replicas. Swappable in tests.
	presenceArchiveClaim = presence.TryClaimArchived
	// presenceArchiveRelease gives a claim back after a delivery that failed for a reason that may
	// pass. Swappable in tests.
	presenceArchiveRelease = presence.ReleaseArchived
	// presenceArchiveDeliverTimeout bounds one delivery (copy or forward), which runs on its own
	// context from chatContext() and not on what is left of the reaction handler's budget.
	presenceArchiveDeliverTimeout = 10 * time.Second
	// presenceArchiveSetTTL is how long per-message distinct reactor sets live outside xenobot:cache:.
	presenceArchiveSetTTL = 72 * time.Hour
	// presenceArchiveDoneTTL is how long the fast Redis once-only latch lives outside xenobot:cache:.
	presenceArchiveDoneTTL = 7 * 24 * time.Hour
)

// presencePositiveEmojis is the positive subset of supportedReactionEmoji used when
// archive_emoji_mode is 'positive'.
var presencePositiveEmojis = map[string]struct{}{
	"❤":   {},
	"👍":   {},
	"🔥":   {},
	"🥰":   {},
	"👏":   {},
	"😁":   {},
	"🎉":   {},
	"🤩":   {},
	"🙏":   {},
	"👌":   {},
	"🕊":   {},
	"😍":   {},
	"🐳":   {},
	"❤‍🔥": {},
	"💯":   {},
	"🤣":   {},
	"⚡":   {},
	"🏆":   {},
	"🍓":   {},
	"🍾":   {},
	"💋":   {},
	"😇":   {},
	"🤝":   {},
	"✍":   {},
	"🤗":   {},
	"🫡":   {},
	"💘":   {},
	"🦄":   {},
	"😘":   {},
	"😎":   {},
	"🆒":   {},
}

func setarchivePendingKey(targetChatID, messageID int64) string {
	return fmt.Sprintf("xenobot:setarchive:%d:%d", targetChatID, messageID)
}

// presenceArchiveBotCanPost checks whether the bot has live permission to post messages in target.
// Package variable only so tests can stub or observe live checks.
var presenceArchiveBotCanPost = func(b *gotgbot.Bot, target *gotgbot.Chat) bool {
	if b == nil || target == nil {
		return false
	}
	member, res, err := chat_status.FetchBotMember(b, target.Id)
	if err != nil || res != chat_status.BotMemberFound {
		return false
	}
	if target.Type == "channel" {
		if member.Status == gotgbot.ChatMemberStatusCreator {
			return true
		}
		return member.Status == gotgbot.ChatMemberStatusAdministrator && member.CanPostMessages
	}
	switch member.Status {
	case gotgbot.ChatMemberStatusCreator, gotgbot.ChatMemberStatusAdministrator, gotgbot.ChatMemberStatusMember:
		return true
	case gotgbot.ChatMemberStatusRestricted:
		return member.IsMember && member.CanSendMessages
	default:
		return false
	}
}

func archiveForwardOrigin(msg *gotgbot.Message) (chat *gotgbot.Chat, messageID int64, ok bool) {
	if msg == nil || msg.ForwardOrigin == nil {
		return nil, 0, false
	}
	origin := msg.ForwardOrigin.MergeMessageOrigin()
	if origin.Chat == nil || (origin.Chat.Type != "channel" && origin.Chat.Type != "supergroup") {
		return nil, 0, false
	}
	return origin.Chat, origin.MessageId, true
}

// setArchive is /setarchive. In a channel (step 1), it records a one-hour pending marker for the
// post and asks for the forward. In a group or supergroup, it either binds a forwarded marked post
// from a non-Staff destination, binds the current forum topic directly when invoked inside a topic
// with Change Info permission, or explains how to link a destination. Staff Groups are refused as
// both source and destination.
func (moduleStruct) setArchive(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	if chat == nil || msg == nil {
		return ext.EndGroups
	}
	tr := ctxTr(ctx)
	if chat.Type == "channel" {
		if presenceIsStaffGroup(chat.Id) {
			replyHTML(b, msg, trS(tr, "archive_staff_refused"))
			return ext.EndGroups
		}
		if err := storePendingMarker(setarchivePendingKey(chat.Id, msg.MessageId), []byte("1")); err != nil {
			log.Errorf("[Presence] failed to store pending setarchive for channel %d: %v", chat.Id, err)
			replyHTML(b, msg, trS(tr, "common_settings_save_failed"))
			return ext.EndGroups
		}
		replyHTML(b, msg, trS(tr, "archive_forward_to_group"))
		return ext.EndGroups
	}
	if !presenceChatTypeAllowed(chat) {
		replyHTML(b, msg, trS(tr, "archive_group_only"))
		return ext.EndGroups
	}
	from := requireUser(b, ctx)
	if from == nil {
		return ext.EndGroups
	}
	if presenceIsStaffGroup(chat.Id) {
		replyHTML(b, msg, trS(tr, "archive_staff_refused"))
		return ext.EndGroups
	}
	if msg.ForwardOrigin != nil {
		if originChat, _, ok := archiveForwardOrigin(msg); ok && presenceIsStaffGroup(originChat.Id) {
			replyHTML(b, msg, trS(tr, "archive_staff_refused"))
			return ext.EndGroups
		}
		if bindSetArchiveForward(b, ctx) {
			return ext.EndGroups
		}
	} else if msg.IsTopicMessage && msg.MessageThreadId > 0 {
		if !chat_status.RequireUserAdmin(b, ctx, chat, from.Id) || !chat_status.CanUserChangeInfo(b, ctx, chat, from.Id) {
			chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
			return ext.EndGroups
		}
		if !presenceArchiveBotCanPost(b, chat) {
			replyHTML(b, msg, trS(tr, "archive_bot_cannot_post"))
			return ext.EndGroups
		}
		res := applyPresenceArchiveTarget(tracing.UpdateContext(ctx), chat.Id, chat.Id, msg.MessageThreadId, from.Id)
		if res.Outcome != presenceApplied {
			replyHTML(b, msg, trS(tr, "common_settings_save_failed"))
			return ext.EndGroups
		}
		replyHTML(b, msg, trS(tr, "archive_set_ok"))
		actionlog.Settings(b, chat, from, "set archive destination")
		return ext.EndGroups
	}
	replyHTML(b, msg, trS(tr, "archive_need_target"))
	return ext.EndGroups
}

// bindSetArchiveForward is step 2 of /setarchive, shared by the command and the group-11 forward
// watcher. Admin and Staff Group checks run before consuming the pending marker so non-admins and
// refused Staff Groups never burn the token.
func bindSetArchiveForward(b *gotgbot.Bot, ctx *ext.Context) bool {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	if msg == nil || !presenceChatTypeAllowed(chat) {
		return false
	}
	if helpers.IsAutomaticForward(msg) {
		return false
	}
	originChat, originMsgID, ok := archiveForwardOrigin(msg)
	if !ok || originMsgID == 0 {
		return false
	}
	key := setarchivePendingKey(originChat.Id, originMsgID)
	if _, ok := peekPendingMarker(key); !ok {
		return false
	}
	tr := ctxTr(ctx)
	if presenceIsStaffGroup(chat.Id) || presenceIsStaffGroup(originChat.Id) {
		replyHTML(b, msg, trS(tr, "archive_staff_refused"))
		return true
	}
	from := chat_status.RequireUser(b, ctx)
	if from == nil {
		return false
	}
	if !chat_status.RequireUserAdmin(b, ctx, chat, from.Id) || !chat_status.CanUserChangeInfo(b, ctx, chat, from.Id) {
		return false
	}
	if !presenceArchiveBotCanPost(b, originChat) {
		replyHTML(b, msg, trS(tr, "archive_bot_cannot_post"))
		return true
	}
	if !consumePendingMarker(key) {
		return false
	}
	res := applyPresenceArchiveTarget(tracing.UpdateContext(ctx), chat.Id, originChat.Id, 0, from.Id)
	if res.Outcome != presenceApplied {
		if err := storePendingMarker(key, []byte("1")); err != nil {
			log.Errorf("[Presence] could not restore setarchive marker for chat %d: %v", originChat.Id, err)
		}
		replyHTML(b, msg, trS(tr, "common_settings_save_failed"))
		return true
	}
	replyHTML(b, msg, trS(tr, "archive_set_ok"))
	actionlog.Settings(b, chat, from, "set archive destination")
	return true
}

// captureSetArchiveForward is the group-11 watcher for a forward of a marked /setarchive post that
// carries no command text. It never ends an update.
func (moduleStruct) captureSetArchiveForward(b *gotgbot.Bot, ctx *ext.Context) error {
	bindSetArchiveForward(b, ctx)
	return ext.ContinueGroups
}

// unsetArchive is /unsetarchive: unlinks and disables the group's Highlights Archive destination.
func (moduleStruct) unsetArchive(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	if msg == nil {
		return ext.EndGroups
	}
	from := requireUser(b, ctx)
	if from == nil {
		return ext.EndGroups
	}
	tr := ctxTr(ctx)
	if !presenceChatTypeAllowed(chat) {
		replyHTML(b, msg, trS(tr, "archive_group_only"))
		return ext.EndGroups
	}
	if presenceIsStaffGroup(chat.Id) {
		replyHTML(b, msg, trS(tr, "archive_staff_refused"))
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, chat, from.Id) || !chat_status.CanUserChangeInfo(b, ctx, chat, from.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}
	runCtx := tracing.UpdateContext(ctx)
	cur, err := presenceReadFresh(runCtx, chat.Id)
	if err != nil {
		replyHTML(b, msg, trS(tr, "common_settings_save_failed"))
		return ext.EndGroups
	}
	if cur.ArchiveChatID == 0 {
		replyHTML(b, msg, trS(tr, "archive_not_set"))
		return ext.EndGroups
	}
	res := applyPresenceClearArchiveTarget(runCtx, chat.Id, from.Id)
	if res.Outcome != presenceApplied {
		replyHTML(b, msg, trS(tr, "common_settings_save_failed"))
		return ext.EndGroups
	}
	replyHTML(b, msg, trS(tr, "archive_unset_ok"))
	actionlog.Settings(b, chat, from, "unset archive destination")
	return ext.EndGroups
}

func normalizeArchiveEmojiMode(mode string) string {
	if presence.ValidArchiveEmojiMode(mode) {
		return mode
	}
	return presence.ArchiveEmojiModePositive
}

func presenceArchiveFilterEmojis(emojis []string, mode string) []string {
	if len(emojis) == 0 {
		return nil
	}
	positiveOnly := mode == presence.ArchiveEmojiModePositive
	var out []string
	for _, emoji := range emojis {
		if _, hostile := presenceConsensusHostile[emoji]; hostile {
			continue
		}
		if positiveOnly {
			if _, ok := presencePositiveEmojis[emoji]; !ok {
				continue
			}
		}
		out = append(out, emoji)
	}
	return out
}

func presenceArchiveAuthorID(ctx context.Context, client *redis.Client, chatID, msgID int64) int64 {
	if authorID := presenceLogAuthorOf(ctx, client, chatID, msgID); authorID != 0 {
		return authorID
	}
	if rec, ok := presenceReadBotReact(ctx, client, chatID, msgID); ok {
		return rec.AuthorID
	}
	return 0
}

func presenceArchiveOp(mru *gotgbot.MessageReactionUpdated, mode string) string {
	if len(presenceArchiveFilterEmojis(presenceStandardEmojis(mru.NewReaction), mode)) > 0 {
		return "add"
	}
	_, removed := presenceDiffReactions(mru.OldReaction, mru.NewReaction)
	if len(presenceArchiveFilterEmojis(removed, mode)) > 0 {
		return "rem"
	}
	return ""
}

func presenceArchiveAlreadyDone(ctx context.Context, client *redis.Client, doneKey string) bool {
	eCtx, eCancel := context.WithTimeout(ctx, chatRedisTimeout)
	defer eCancel()
	exists, err := client.Exists(eCtx, doneKey).Result()
	return err == nil && exists > 0
}

func presenceArchiveTrackAndCheck(ctx context.Context, client *redis.Client, mru *gotgbot.MessageReactionUpdated, settings presence.Settings, op string, updateID int64) bool {
	chatID, msgID := mru.Chat.Id, mru.MessageId
	setTTLSecs := max(int(presenceArchiveSetTTL.Seconds()), 1)
	rCtx, cancel := context.WithTimeout(ctx, chatRedisTimeout)
	count, err := presenceConsensusUpdateScript.Run(
		rCtx,
		client,
		[]string{presenceArchiveUsersKey(chatID, msgID), presenceReactionStampKey(chatID, msgID, mru.User.Id)},
		op,
		strconv.FormatInt(mru.User.Id, 10),
		setTTLSecs,
		updateID,
	).Int64()
	cancel()
	if err != nil {
		log.WithField("chat_id", chatID).Debugf("[Presence] archive set %s failed: %s", op, chatRedisErrClass(err))
		return false
	}
	threshold := settings.ArchiveThreshold
	if !presence.ValidArchiveThreshold(threshold) {
		threshold = presence.DefaultArchiveThreshold
	}
	if count < 0 || op != "add" || count < int64(threshold) { // count < 0: a newer update of this user was applied
		return false
	}
	doneKey := presenceArchiveDoneKey(chatID, msgID)
	if presenceArchiveAlreadyDone(ctx, client, doneKey) || presenceIsLockedDown(chatID) {
		return false
	}
	claimed, err := presenceArchiveClaim(ctx, chatID, msgID)
	if err != nil {
		log.WithField("chat_id", chatID).Debug("[Presence] archive claim failed")
		return false
	}
	wCtx, wCancel := context.WithTimeout(ctx, chatRedisTimeout)
	_ = client.Set(wCtx, doneKey, "1", presenceArchiveDoneTTL).Err()
	wCancel()
	return claimed
}

// presenceHandleArchive tracks distinct non-bot, non-author member reactions on msgID for
// Highlights Archive (COMM-04). When the distinct count reaches settings.ArchiveThreshold, it checks
// the fast Redis latch, lazily verifies active lockdown status (fail-closed), claims (chat_id,
// message_id) once in PostgreSQL, and delivers the highlight via CopyMessage or ForwardMessage.
func presenceHandleArchive(ctx context.Context, b *gotgbot.Bot, mru *gotgbot.MessageReactionUpdated, settings presence.Settings, updateID int64) {
	if b == nil || mru == nil || mru.User == nil || mru.User.IsBot || !presenceChatTypeAllowed(&mru.Chat) {
		return
	}
	if !settings.ArchiveEnabled || settings.ArchiveChatID == 0 {
		return
	}
	chatID, msgID := mru.Chat.Id, mru.MessageId
	if msgID <= 0 || presenceIsStaffGroup(chatID) || presenceIsStaffGroup(settings.ArchiveChatID) {
		return
	}
	client := presenceRedisClient()
	if client == nil {
		return
	}
	if authorID := presenceArchiveAuthorID(ctx, client, chatID, msgID); authorID != 0 && mru.User.Id == authorID {
		return
	}
	op := presenceArchiveOp(mru, normalizeArchiveEmojiMode(settings.ArchiveEmojiMode))
	if op == "" || ctx.Err() != nil {
		return
	}
	if !presenceArchiveTrackAndCheck(ctx, client, mru, settings, op, updateID) {
		return
	}
	presenceArchiveDeliverAndSettle(ctx, client, b, mru, settings)
}

// presenceArchiveDeliverAndSettle delivers a highlight whose claim this caller holds, on its own
// bounded context: the claim and the latch were written before the send, and the send is a paced
// Telegram call that must not share what is left of the handler's budget. When the delivery fails
// for a reason that may pass (a timeout, a rate limit, a 5xx, a transport error) both the durable
// claim and the Redis latch are given back, so the next reaction on the message tries again; a
// definitive refusal (a Telegram 4xx other than 429: the message is gone, the bot may not post in
// the archive) keeps them, as retrying cannot succeed.
func presenceArchiveDeliverAndSettle(ctx context.Context, client *redis.Client, b *gotgbot.Bot, mru *gotgbot.MessageReactionUpdated, settings presence.Settings) {
	deliverCtx, cancel := context.WithTimeout(chatContext(), presenceArchiveDeliverTimeout)
	defer cancel()
	err := presenceDeliverArchive(deliverCtx, b, mru, settings)
	if err == nil || isTelegramClientError(err) {
		return
	}
	chatID, msgID := mru.Chat.Id, mru.MessageId
	settleCtx, settleCancel := context.WithTimeout(context.WithoutCancel(ctx), chatRedisTimeout)
	defer settleCancel()
	if _, relErr := presenceArchiveRelease(settleCtx, chatID, msgID); relErr != nil {
		log.WithField("chat_id", chatID).Debug("[Presence] archive claim release failed")
	}
	if delErr := client.Del(settleCtx, presenceArchiveDoneKey(chatID, msgID)).Err(); delErr != nil {
		log.WithField("chat_id", chatID).Debugf("[Presence] archive latch release failed: %s", chatRedisErrClass(delErr))
	}
}

func presenceArchiveJumpURL(chat *gotgbot.Chat, msgID int64) string {
	if chat == nil || msgID <= 0 {
		return ""
	}
	if chat.Username != "" {
		return fmt.Sprintf("https://t.me/%s/%d", chat.Username, msgID)
	}
	if chat.Type == "supergroup" && strings.HasPrefix(strconv.FormatInt(chat.Id, 10), "-100") {
		return chat_status.GetMessageLinkFromMessageId(chat, msgID)
	}
	return ""
}

func presenceDeliverArchive(ctx context.Context, b *gotgbot.Bot, mru *gotgbot.MessageReactionUpdated, settings presence.Settings) error {
	if b == nil || mru == nil {
		return nil
	}
	chatID, msgID := mru.Chat.Id, mru.MessageId
	if settings.ArchiveDelivery == presence.ArchiveDeliveryForward {
		var opts *gotgbot.ForwardMessageOpts
		if settings.ArchiveThreadID > 0 {
			opts = &gotgbot.ForwardMessageOpts{MessageThreadId: settings.ArchiveThreadID}
		}
		err := presencePaced(ctx, func(callCtx context.Context) error {
			_, callErr := b.ForwardMessageWithContext(callCtx, settings.ArchiveChatID, chatID, msgID, opts)
			return callErr
		})
		if err != nil {
			log.WithField("chat_id", chatID).Debugf("[Presence] archive forward failed: %s", chatSendErrClass(err))
		}
		return err
	}

	jumpURL := presenceArchiveJumpURL(&mru.Chat, msgID)
	var opts *gotgbot.CopyMessageOpts
	if settings.ArchiveThreadID > 0 || jumpURL != "" {
		opts = &gotgbot.CopyMessageOpts{
			MessageThreadId: settings.ArchiveThreadID,
		}
		if jumpURL != "" {
			tr := i18n.MustNewTranslator(lang.GetGroupLanguage(ctx, chatID))
			label := trS(tr, "archive_jump_button")
			if label == "" {
				label = "↗️ Jump to message"
			}
			opts.ReplyMarkup = gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{{Text: label, Url: jumpURL}},
				},
			}
		}
	}
	err := presencePaced(ctx, func(callCtx context.Context) error {
		_, callErr := b.CopyMessageWithContext(callCtx, settings.ArchiveChatID, chatID, msgID, opts)
		return callErr
	})
	if err != nil {
		log.WithField("chat_id", chatID).Debugf("[Presence] archive copy failed: %s", chatSendErrClass(err))
	}
	return err
}
