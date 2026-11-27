package modules

import (
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/utils/actionlog"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/chat_status"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/helpers"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/tracing"
)

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
