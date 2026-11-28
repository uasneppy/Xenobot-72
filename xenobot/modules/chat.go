package modules

import (
	"context"
	"strings"
	"unicode"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/db/membermem"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/chatprompt"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/helpers"
)

// The Chat module (Phase 20): Xenobot-72 answers members who address it, in groups whose owner
// turned conversations on from the bot's private chat. It never converses in a private chat. It
// also keeps a group's message log while the owner's Message log switch is on and writes briefs of
// it (/unrollshort, /unrolllong) and an admin's AI verdict (/judge); see chat_log.go and chat_brief.go.

var chatModule = moduleStruct{moduleName: "Chat"}

// unrollShortDesc is /unrollshort: everyone may run it. It checks its own name, so disabling it
// silences it. The handler and the brief pool are in chat_brief.go.
var unrollShortDesc = helpers.CommandDescriptor{
	Name:           "unrollshort",
	Disableable:    true,
	RequiredChecks: []helpers.CheckFunc{helpers.CheckDisabled("unrollshort"), helpers.RequireGroup()},
}

// unrollLongDesc is /unrolllong: a detailed brief, for everyone, checking its own name.
var unrollLongDesc = helpers.CommandDescriptor{
	Name:           "unrolllong",
	Disableable:    true,
	RequiredChecks: []helpers.CheckFunc{helpers.CheckDisabled("unrolllong"), helpers.RequireGroup()},
}

// judgeDesc is /judge: administrators only. Its authority is requireLockdownAuthority(false), a live
// getChatMember of the sender that never reads the admin cache and always sends an anonymous admin
// the proof prompt; it sits last so a disabled command or a private chat costs no Telegram call.
// The tap re-enters through anonPipelineHandler, which re-runs these checks for the tapper.
var judgeDesc = helpers.CommandDescriptor{
	Name:        "judge",
	Disableable: true,
	RequiredChecks: []helpers.CheckFunc{
		helpers.CheckDisabled("judge"), helpers.RequireGroup(), requireLockdownAuthority(false),
	},
}

// referendumDesc is /referendum: everyone allowed by chat Audience may spark an AI-drafted
// native poll when Owner Conversations and Admin Referendum are both on.
var referendumDesc = helpers.CommandDescriptor{
	Name:           "referendum",
	Disableable:    true,
	RequiredChecks: []helpers.CheckFunc{helpers.CheckDisabled("referendum"), helpers.RequireGroup()},
}

// inspectDesc is /inspect: everyone allowed by chat Audience may request a deadpan Facility
// Inspection of a replied message, photo, or sticker when Owner Conversations and Admin Inspect
// are both on.
var inspectDesc = helpers.CommandDescriptor{
	Name:           "inspect",
	Disableable:    true,
	RequiredChecks: []helpers.CheckFunc{helpers.CheckDisabled("inspect"), helpers.RequireGroup()},
}

// forgetMeDesc is /forgetme: every member may delete the notes the conversation feature keeps about them in
// this group. It works whatever the owner's switches say, so a member can always remove their notes.
var forgetMeDesc = helpers.CommandDescriptor{
	Name:           "forgetme",
	Disableable:    true,
	RequiredChecks: []helpers.CheckFunc{helpers.CheckDisabled("forgetme"), helpers.RequireGroup()},
}

// LoadChat registers /ask (alias /xeno), /unrollshort, /unrolllong, /judge, /referendum, /inspect,
// the conversation watcher, alone at its own handler group after every other group, with no
// setter (new messages of real users only), and the message log watcher at the group after it.
func LoadChat(dispatcher *ext.Dispatcher) {
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "ask",
		Disableable:    true,
		RequiredChecks: []helpers.CheckFunc{helpers.CheckDisabled("ask")},
	}, chatModule.ask)
	// /xeno honours its own name and /ask's, so disabling /ask silences both.
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "xeno",
		Disableable:    true,
		RequiredChecks: []helpers.CheckFunc{helpers.CheckDisabled("xeno"), helpers.CheckDisabled("ask")},
	}, chatModule.ask)
	helpers.WrapCommand(dispatcher, unrollShortDesc, chatModule.unrollShort)
	helpers.WrapCommand(dispatcher, unrollLongDesc, chatModule.unrollLong)
	helpers.WrapCommand(dispatcher, judgeDesc, chatModule.judgeCommand)
	helpers.WrapCommand(dispatcher, referendumDesc, chatModule.referendumCommand)
	helpers.WrapCommand(dispatcher, inspectDesc, chatModule.inspectCommand)
	helpers.WrapCommand(dispatcher, forgetMeDesc, chatModule.forgetMe)
	dispatcher.AddHandlerToGroup(handlers.NewMessage(message.All, chatModule.chatWatcher), chatWatchGroup)
	// The log watcher is the only handler of its group. It sees edits, which the conversation
	// watcher does not, and ends no update.
	dispatcher.AddHandlerToGroup(handlers.NewMessage(message.All, chatModule.chatLogWatcher).SetAllowEdited(true), chatLogGroup)

	SetModuleEnabled(chatModule.moduleName, true)
	DefaultHelpRegistry().AltHelpOptions["Chat"] = []string{"conversations", "ask", "xeno", "unrollshort", "unrolllong", "judge", "messagelog", "referendum", "inspect", "forgetme", "memories"}
	log.Info("[Modules] Chat module loaded")
}

// ask answers /ask <text>: in a private chat it says conversations happen in groups; in a group
// whose conversations are off it says how the owner turns them on; with no text it shows the
// usage; otherwise it hands the text to the conversation pool like an addressed message, after
// the same checks. Anonymous admins and channels get nothing.
func (moduleStruct) ask(c *helpers.CommandContext) error {
	msg, chat := c.Msg, c.Chat
	if chat.Type == "private" {
		chatReplyText(c, "chat_private_refused")
		return ext.EndGroups
	}
	if chat.Type != "group" && chat.Type != "supergroup" {
		return ext.EndGroups
	}
	userID, ok := raidFloodSenderCounts(c.Bot, c.Ctx)
	if !ok || c.User.IsBot {
		return ext.EndGroups
	}
	gate, provider := chatAccept(c.Bot, chat.Id, userID)
	switch gate {
	case chatGateOff:
		chatReplyText(c, "chat_off_hint")
		return ext.EndGroups
	case chatGateSilent:
		return ext.EndGroups
	}
	text := strings.TrimSpace(chatprompt.StripMention(chatCommandArgument(chatMessageText(msg)), c.Bot.Username))
	job := chatJobOf(c.Bot, msg, userID, provider)
	if text == "" && len(job.media) == 0 {
		chatReplyText(c, "chat_ask_usage")
		return ext.EndGroups
	}
	if text != "" {
		job.turn.Text = text
		job.hasText = true
	} else {
		job.hasText = false
		label := chatMediaLabel(msg)
		if label == "" {
			label = "[attachment]"
		}
		job.turn.Text = label
	}
	chatEnqueue(job)
	return ext.EndGroups
}

// chatCommandArgument is the text after a command's first word, line breaks kept.
func chatCommandArgument(text string) string {
	i := strings.IndexFunc(text, unicode.IsSpace)
	if i < 0 {
		return ""
	}
	return strings.TrimSpace(text[i:])
}

// forgetMe answers /forgetme: it deletes the sender's own notes of this group, nobody else's, and says
// whether there were any. Anonymous admins, channels and bots get nothing, as for /ask.
func (moduleStruct) forgetMe(c *helpers.CommandContext) error {
	chat := c.Chat
	if chat.Type != "group" && chat.Type != "supergroup" {
		return ext.EndGroups
	}
	userID, ok := raidFloodSenderCounts(c.Bot, c.Ctx)
	if !ok || c.User.IsBot {
		return ext.EndGroups
	}
	ctx, cancel := context.WithTimeout(context.Background(), chatMemoryReadTimeout)
	defer cancel()
	removed, err := membermem.Delete(ctx, chat.Id, userID)
	switch {
	case err != nil:
		log.WithFields(log.Fields{"chat_id": chat.Id, "user_id": userID}).Warn("[Chat] /forgetme could not delete the member notes")
		chatReplyText(c, "chat_forgetme_failed")
	case removed:
		chatReplyText(c, "chat_forgetme_done")
	default:
		chatReplyText(c, "chat_forgetme_nothing")
	}
	return ext.EndGroups
}

// chatReplyText answers the command with one locale line, in HTML like every reply.
func chatReplyText(c *helpers.CommandContext, key string) {
	text, err := c.Tr.GetString(key)
	if err != nil || strings.TrimSpace(text) == "" {
		return
	}
	if _, err := c.Msg.Reply(c.Bot, text, &gotgbot.SendMessageOpts{ParseMode: "HTML"}); err != nil {
		log.WithField("chat_id", c.Chat.Id).Debugf("[Chat] the /ask answer could not be sent: %s", chatSendErrClass(err))
	}
}

func init() {
	RegisterLegacyModule("Chat", 245, LoadChat)
	RegisterAnonymousAdminHandler("judge", anonPipelineHandler(judgeDesc, judgeAnon))
}
