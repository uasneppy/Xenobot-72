package modules

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
)

// The punishment notice of the forward blocklist watcher lives in its own file, outside the staff*
// files, because it is a message of the linked group (an automatic punishment message, like the
// blacklist's and the member requirements') and so is queued for deletion, while the never-delete
// guard (TestMemberDeletionExcludedMessages) forbids any deletion producer in a staff file: a Staff
// Group card or summary must never reach one. The notice is never posted into a Staff Group. It is also
// the one way the forward blocklist watcher reaches the event message engine for a notice
// (TestPMCopyCallers forbids postEvent in staff files): its punishments are the linked group's own
// automatic actions, like a blacklist word's (D-30).

// forwardBlockNotice posts the punishment notice of the forward blocklist's punishment as applied (a
// mute, kick or ban, or the timed mute or ban a linked group's escalation ladder chose) in the linked group, through the event message engine (postEvent) with the blacklist's site and
// reply: the group's stored forward blocklist message replaces the default, which is the blacklist's
// punishment notice in the group's language, and both are queued as automatic punishment messages. The
// reason is the fixed translated forwardBlockReason and the member is the sender: nothing about the
// blocked source or the Staff Group is in scope here, so no placeholder can name them (D-06, D-31).
func forwardBlockNotice(b *gotgbot.Bot, ctx *ext.Context, userID int64, p autoPunishment) error {
	tr := ctxTr(ctx)
	target := eventSenderTarget(ctx.EffectiveSender)
	target.Id = userID
	reply := listNoticeReply(tr, eventmsg.EventForwardBlock, target, p, forwardBlockReason(tr))
	return postEvent(listNoticeSite(b, &ctx.EffectiveMessage.Chat, tr), reply)
}
