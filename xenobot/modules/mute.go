package modules

import (
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/db/eventmsg"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/chat_status"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/extraction"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/formatting"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/helpers"
)

var mutesModule = moduleStruct{moduleName: "Mutes"}

func muteTargetValidation(c *moderationCtx, t *target) error {
	return validateTarget(c, t.userID, true, "common_user_not_in_chat", "mutes_mute_admin_error", "mutes_restrict_self_error")
}

// muteDefaultReply is the mute event's default reply (an eventDefaultReply): the text and the
// Unmute keyboard /mute and /dmute sent before Phase 21, byte for byte (TestLegacyModerationRequests).
func muteDefaultReply(tr *i18n.Translator, in eventDefaultInput) (string, *gotgbot.SendMessageOpts) {
	baseStr, _ := tr.GetString("mutes_mute_message")
	text := fmt.Sprintf(baseStr, formatting.MentionHtml(in.Target.Id, in.Target.FirstName)) +
		moderationReasonLine(tr, "mutes_reason_suffix", in.Reason)
	return text, &gotgbot.SendMessageOpts{
		ParseMode: formatting.HTML,
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{
			InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
				{
					{
						Text:         trS(tr, "mutes_unmute_button"),
						CallbackData: encodeCallbackData("unrestrict", map[string]string{"a": "unmute", "u": fmt.Sprint(in.Target.Id)}),
					},
				},
			},
		},
	}
}

// tmuteDefaultReply is the timed mute event's default reply (an eventDefaultReply): the text and the
// options /tmute sent before Phase 21, byte for byte (TestLegacyModerationRequests).
func tmuteDefaultReply(tr *i18n.Translator, in eventDefaultInput) (string, *gotgbot.SendMessageOpts) {
	temp, _ := tr.GetString("mutes_tmute_message")
	text := fmt.Sprintf(temp, formatting.MentionHtml(in.Target.Id, in.Target.FirstName), in.TimeVal) +
		moderationReasonLine(tr, "mutes_reason_suffix", in.Reason)
	return text, formatting.Shtml()
}

// unmuteDefaultReply is the unmute event's default reply (an eventDefaultReply) without the lockdown
// note, which the site appends: the text and the options /unmute sent before Phase 21.
func unmuteDefaultReply(tr *i18n.Translator, in eventDefaultInput) (string, *gotgbot.SendMessageOpts) {
	temp, _ := tr.GetString("mutes_unmute_message")
	return fmt.Sprintf(temp, formatting.MentionHtml(in.Target.Id, in.Target.FirstName)), formatting.Shtml()
}

// muteReplyWithButton is the success reply of /mute and /dmute, the mute event: the default reply
// built by muteDefaultReply, or the group's stored mute message, through replyEvent after the mute.
// When a default mute duration applied (t.untilDate > 0), it emits EventTimedMute with duration and until.
func muteReplyWithButton(c *moderationCtx, t *target) error {
	muteUser, err := c.Bot.GetChat(t.userID, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	member := eventTargetOf(muteUser)
	if t.untilDate > 0 {
		tmuteIn := eventDefaultInput{Target: member, Reason: t.reason, TimeVal: t.timeVal}
		text, opts := tmuteDefaultReply(c.Tr, tmuteIn)
		err = c.replyEvent(eventReply{
			Event:        eventmsg.EventTimedMute,
			Target:       member,
			Values:       eventValues{Reason: t.reason, Duration: t.timeVal, Until: eventUntil(t.untilDate)},
			Default:      text,
			PlainDefault: func() string { return tmutePlainDefaultReply(c.Tr, tmuteIn) },
			Opts:         opts,
		})
	} else {
		muteIn := eventDefaultInput{Target: member, Reason: t.reason, TimeVal: t.timeVal}
		text, opts := muteDefaultReply(c.Tr, muteIn)
		err = c.replyEvent(eventReply{
			Event:        eventmsg.EventMute,
			Target:       member,
			Values:       eventValues{Reason: t.reason},
			Default:      text,
			PlainDefault: func() string { return mutePlainDefaultReply(c.Tr, muteIn) },
			Opts:         opts,
		})
	}
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func extractUserOnly(c *moderationCtx) (target, error) {
	uid := extraction.ExtractUser(c.Bot, c.Ctx)
	if uid == -1 {
		return target{}, fmt.Errorf("extraction failed")
	}
	if chat_status.IsChannelId(uid) {
		text, _ := c.Tr.GetString("common_anonymous_user_error")
		_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return target{}, err
		}
		return target{}, fmt.Errorf("anonymous user")
	}
	if uid == 0 {
		noUserKey := "common_no_user_specified"
		text, _ := c.Tr.GetString(noUserKey)
		_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return target{}, err
		}
		return target{}, fmt.Errorf("no user")
	}
	return target{userID: uid}, nil
}

// extractUserAndOptionalReason extracts the target user with the exact same error messages as
// extractUserOnly while retaining any trailing reason text for silent punishment commands
// (/sban, /skick, /smute).
func extractUserAndOptionalReason(c *moderationCtx) (target, error) {
	uid, reason := extraction.ExtractUserAndText(c.Bot, c.Ctx)
	if uid == -1 {
		return target{}, fmt.Errorf("extraction failed")
	}
	if chat_status.IsChannelId(uid) {
		text, _ := c.Tr.GetString("common_anonymous_user_error")
		_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return target{}, err
		}
		return target{}, fmt.Errorf("anonymous user")
	}
	if uid == 0 {
		noUserKey := "common_no_user_specified"
		text, _ := c.Tr.GetString(noUserKey)
		_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return target{}, err
		}
		return target{}, fmt.Errorf("no user")
	}
	return target{userID: uid, reason: reason}, nil
}

func moderationTmute(m *moduleStruct) *moderationCommand {
	return &moderationCommand{
		module:   m,
		gates:    []gateFn{standardModGates},
		extract:  extractFromArgs,
		validate: muteTargetValidation,
		execute: func(c *moderationCtx, t *target) error {
			_time := extractTemporalTarget(c, t)
			if _time == -1 {
				return ext.EndGroups
			}
			if manualApplyReasonRequired(c, t.reason, c.manualDefaults()) {
				return errManualReasonRequired
			}
			t.reason = expandManualReason(c.Chat.Id, t.reason)
			_, err := c.Chat.RestrictMember(c.Bot, t.userID, MutedPermissions,
				&gotgbot.RestrictChatMemberOpts{UntilDate: _time},
			)
			return err
		},
		reply: func(c *moderationCtx, t *target) error {
			muteUser, err := c.Bot.GetChat(t.userID, nil)
			if err != nil {
				log.Error(err)
				return err
			}

			member := eventTargetOf(muteUser)
			tmuteIn := eventDefaultInput{Target: member, Reason: t.reason, TimeVal: t.timeVal}
			text, opts := tmuteDefaultReply(c.Tr, tmuteIn)
			err = c.replyEvent(eventReply{
				Event:        eventmsg.EventTimedMute,
				Target:       member,
				Values:       eventValues{Reason: t.reason, Duration: t.timeVal, Until: eventUntil(t.untilDate)},
				Default:      text,
				PlainDefault: func() string { return tmutePlainDefaultReply(c.Tr, tmuteIn) },
				Opts:         opts,
			})
			if err != nil {
				log.Error(err)
				return err
			}
			return nil
		},
		logAction: "tmute",
	}
}

func moderationMute(m *moduleStruct) *moderationCommand {
	return &moderationCommand{
		module:        m,
		gates:         []gateFn{standardModGates},
		extract:       extractFromArgs,
		validate:      muteTargetValidation,
		reasonChecked: true,
		execute: func(c *moderationCtx, t *target) error {
			return manualMuteMember(c, t)
		},
		reply:     muteReplyWithButton,
		logAction: "mute",
	}
}

func moderationSmute(m *moduleStruct) *moderationCommand {
	return &moderationCommand{
		module:        m,
		gates:         []gateFn{deleteModGates},
		extract:       extractUserAndOptionalReason,
		validate:      muteTargetValidation,
		reasonChecked: true,
		execute: func(c *moderationCtx, t *target) error {
			return manualMuteMember(c, t)
		},
		reply: func(c *moderationCtx, t *target) error {
			_ = helpers.DeleteMessageWithErrorHandling(c.Bot, c.Chat.Id, c.Msg.MessageId)
			return nil
		},
		logAction: "smute",
	}
}

func moderationDmute(m *moduleStruct) *moderationCommand {
	return &moderationCommand{
		module:  m,
		gates:   []gateFn{deleteModGates},
		extract: extractFromArgs,
		validate: func(c *moderationCtx, t *target) error {
			if extraction.ReplyTarget(c.Msg) == nil {
				text, _ := c.Tr.GetString("mute_reply_to_dmute")
				if text == "" {
					text, _ = c.Tr.GetString("common_no_reply_to_message")
				}
				_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
				if err != nil {
					log.Error(err)
					return err
				}
				return fmt.Errorf("no reply")
			}
			if replyTargetMismatch(c.Msg, t.userID) {
				return refuseReplyMismatch(c)
			}
			return muteTargetValidation(c, t)
		},
		reasonChecked: true,
		execute: func(c *moderationCtx, t *target) error {
			_, err := extraction.ReplyTarget(c.Msg).Delete(c.Bot, nil)
			if err != nil {
				log.Error(err)
				return err
			}
			return manualMuteMember(c, t)
		},
		reply:     muteReplyWithButton,
		logAction: "dmute",
	}
}

func moderationUnmute(m *moduleStruct) *moderationCommand {
	return &moderationCommand{
		module:  m,
		gates:   []gateFn{standardModGates},
		extract: extractUserOnly,
		validate: func(c *moderationCtx, t *target) error {
			if !chat_status.IsUserInChat(c.Bot, c.Chat, t.userID) {
				text, _ := c.Tr.GetString("common_user_not_in_chat")
				_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
				if err != nil {
					log.Error(err)
					return err
				}
				return errUserNotInChat
			}
			if t.userID == c.Bot.Id {
				text, _ := c.Tr.GetString("mutes_restrict_self_error")
				if text == "" {
					text, _ = c.Tr.GetString("common_cannot_target_self")
				}
				_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
				if err != nil {
					log.Error(err)
					return err
				}
				return errTargetIsBot
			}
			return nil
		},
		execute: func(c *moderationCtx, t *target) error {
			chat, err := c.Bot.GetChat(c.Chat.Id, nil)
			if err != nil {
				log.Error(err)
				return err
			}
			unmutePermissions, err := resolveUnmutePermissions(chat)
			if err != nil {
				log.Error(err)
				return err
			}
			_, err = c.Chat.RestrictMember(c.Bot, t.userID, unmutePermissions, nil)
			return err
		},
		reply: func(c *moderationCtx, t *target) error {
			muteUser, err := c.Bot.GetChat(t.userID, nil)
			if err != nil {
				log.Error(err)
				return err
			}

			member := eventTargetOf(muteUser)
			unmuteIn := eventDefaultInput{Target: member}
			text, opts := unmuteDefaultReply(c.Tr, unmuteIn)
			// During a lockdown the user is still muted by the locked default
			// permissions, so say they can talk once it lifts. A failed read only
			// skips the note: the unmute itself already went through. A custom
			// unmute text is followed by the same note (D-11). The private copy carries the
			// note's private wording (PMNote), after a custom text or the default.
			var note, pmNote string
			if active, lookupErr := lockdownSnapshotLookup(c.Chat.Id); lookupErr != nil {
				log.Errorf("[Mutes] unmute reply: read lockdown of chat %d: %v", c.Chat.Id, lookupErr)
			} else if active != nil {
				note, _ = c.Tr.GetString("mutes_unmute_lockdown_note")
				pmNote, _ = c.Tr.GetString("eventmsg_pm_unmute_lockdown")
				text += "\n" + note
			}
			err = c.replyEvent(eventReply{
				Event:   eventmsg.EventUnmute,
				Target:  member,
				Default: text,
				PlainDefault: func() string {
					plain := unmutePlainDefaultReply(c.Tr, unmuteIn)
					if note != "" {
						plain += "\n" + note
					}
					return plain
				},
				Opts:   opts,
				Note:   note,
				PMNote: pmNote,
			})
			if err != nil {
				log.Error(err)
				return err
			}
			return nil
		},
		logAction: "unmute",
	}
}

func (m moduleStruct) tMute(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationTmute(&m).run(b, ctx)
}

func (m moduleStruct) mute(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationMute(&m).run(b, ctx)
}

func (m moduleStruct) sMute(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationSmute(&m).run(b, ctx)
}

func (m moduleStruct) dMute(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationDmute(&m).run(b, ctx)
}

func (m moduleStruct) unmute(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationUnmute(&m).run(b, ctx)
}

var (
	muteDesc   = helpers.CommandDescriptor{Name: "mute"}
	smuteDesc  = helpers.CommandDescriptor{Name: "smute"}
	tmuteDesc  = helpers.CommandDescriptor{Name: "tmute"}
	dmuteDesc  = helpers.CommandDescriptor{Name: "dmute"}
	unmuteDesc = helpers.CommandDescriptor{Name: "unmute"}
)

func initMuteDescs() {
	muteDesc.RequiredChecks = restrictChecks("mute")
	smuteDesc.RequiredChecks = deleteRestrictChecks("smute")
	tmuteDesc.RequiredChecks = restrictChecks("tmute")
	dmuteDesc.RequiredChecks = deleteRestrictChecks("dmute")
	unmuteDesc.RequiredChecks = restrictChecks("unmute")
}

func LoadMutes(dispatcher *ext.Dispatcher) {
	SetModuleEnabled(mutesModule.moduleName, true)
	initMuteDescs()

	helpers.WrapCommand(dispatcher, muteDesc, pipelineHandler(mutesModule.mute))
	helpers.WrapCommand(dispatcher, smuteDesc, pipelineHandler(mutesModule.sMute))
	helpers.WrapCommand(dispatcher, tmuteDesc, pipelineHandler(mutesModule.tMute))
	helpers.WrapCommand(dispatcher, dmuteDesc, pipelineHandler(mutesModule.dMute))
	helpers.WrapCommand(dispatcher, unmuteDesc, pipelineHandler(mutesModule.unmute))
}

func init() {
	RegisterLegacyModule("Mutes", 80, LoadMutes)
	initMuteDescs()
	RegisterAnonymousAdminHandler("mute", anonPipelineHandler(muteDesc, mutesModule.mute))
	RegisterAnonymousAdminHandler("smute", anonPipelineHandler(smuteDesc, mutesModule.sMute))
	RegisterAnonymousAdminHandler("dmute", anonPipelineHandler(dmuteDesc, mutesModule.dMute))
	RegisterAnonymousAdminHandler("tmute", anonPipelineHandler(tmuteDesc, mutesModule.tMute))
	RegisterAnonymousAdminHandler("unmute", anonPipelineHandler(unmuteDesc, mutesModule.unmute))
}
