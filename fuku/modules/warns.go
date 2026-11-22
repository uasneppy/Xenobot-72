package modules

import (
	"context"
	"fmt"
	"html"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/fuku_robot/fuku/db/deletion"
	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/db/lang"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/db/rules"
	"github.com/uasneppy/fuku_robot/fuku/db/warns"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
	"github.com/uasneppy/fuku_robot/fuku/utils/chat_status"
	"github.com/uasneppy/fuku_robot/fuku/utils/extraction"
	"github.com/uasneppy/fuku_robot/fuku/utils/formatting"
	"github.com/uasneppy/fuku_robot/fuku/utils/helpers"
	"github.com/uasneppy/fuku_robot/fuku/utils/tracing"
)

var warnsModule = moduleStruct{moduleName: "Warns"}

// Serializes limit-punish + reset per (chat,user): two concurrent limit hits must
// not both punish and then wipe each other's freshly added warn with ResetUserWarns.
// ponytail: fixed 32 shards; resize only if warn-punish contention shows up.
var warnPunishShards [32]sync.Mutex

func warnPunishLock(chatID, userID int64) *sync.Mutex {
	return &warnPunishShards[uint64(chatID*31+userID)%uint64(len(warnPunishShards))]
}

// warnSettingsModes are the plain actions /setwarnmode and the settings page may write. The timed
// modes tban and tmute are written beside them only with a valid duration (warnTimedModes, D-07); the
// table's CHECK also allows the empty string, which applies nothing at the limit and is never written
// (D-17).
var warnSettingsModes = []string{"ban", "kick", "mute"}

// warnTimedModes are the timed limit actions, accepted only with a duration in punishMinSecs to
// punishMaxSecs (D-09).
var warnTimedModes = []string{"tban", "tmute"}

// warnSettingsOutcome is what an apply function did with its input.
type warnSettingsOutcome int

const (
	// warnSettingsApplied is the zero value: the value was valid and stored.
	warnSettingsApplied warnSettingsOutcome = iota
	// warnSettingsOutOfRange means the limit was outside 1..100; nothing was written.
	warnSettingsOutOfRange
	// warnSettingsUnknownMode means the action was not ban, kick or mute, nor a timed mode with a
	// duration; nothing was written.
	warnSettingsUnknownMode
	// warnSettingsSaveFailed means the repository returned an error; Err holds it.
	warnSettingsSaveFailed
	// warnSettingsBadDuration means a timed mode (tban, tmute) came with a duration outside
	// punishMinSecs to punishMaxSecs; nothing was written.
	warnSettingsBadDuration
)

// warnSettingsResult is the verdict of an apply function. Key is the locale key the command
// replies with for that outcome. Stored is the row read back after a successful write (nil
// otherwise, or when the read-back failed), so a caller shows what is stored and never what it
// assumes.
type warnSettingsResult struct {
	Outcome warnSettingsOutcome
	Key     string
	Stored  *models.WarnSettings
	Err     error
}

// warnSettingsWriter holds the two repository writes of the warn settings, so a test can make
// one fail. Each write drops its own cache key. Mode writes the action and its duration in seconds
// (0 for a plain action) together.
type warnSettingsWriter struct {
	Limit func(chatID int64, limit int) error
	Mode  func(ctx context.Context, chatID int64, mode string, secs int) error
}

// warnSettingsWrites is the write seam of the warn settings; production uses the repository.
var warnSettingsWrites = warnSettingsWriter{Limit: warns.SetWarnLimit, Mode: warns.SetWarnModeContext}

// warnReadBack reads the chat's warn settings after a write. A failed read-back is logged and
// returns nil; the write already happened.
func warnReadBack(ctx context.Context, chatID int64) *models.WarnSettings {
	stored, err := warns.LoadWarnSettingsContext(ctx, chatID)
	if err != nil {
		log.Warnf("[Warns] read-back after write failed for chat %d: %v", chatID, err)
		return nil
	}
	return stored
}

// applyWarnLimit validates and stores the warn limit: 1..100, anything else writes nothing.
// The repository write invalidates its own cache key.
func applyWarnLimit(ctx context.Context, chatID int64, limit int) warnSettingsResult {
	if limit < 1 || limit > 100 {
		return warnSettingsResult{Outcome: warnSettingsOutOfRange, Key: "warns_limit_range_error"}
	}
	if err := warnSettingsWrites.Limit(chatID, limit); err != nil {
		return warnSettingsResult{Outcome: warnSettingsSaveFailed, Key: "common_settings_save_failed", Err: err}
	}
	return warnSettingsResult{Key: "warns_limit_updated", Stored: warnReadBack(ctx, chatID)}
}

// applyWarnMode validates and stores the action at the warn limit and its duration in seconds,
// exactly as spelled; the caller lower-cases user text first (D-09):
//   - a plain mode (ban, kick, mute) stores seconds 0, clearing any duration, whatever secs says;
//   - a timed mode (tban, tmute) with seconds 0 is warnSettingsUnknownMode, the pinned reply of a
//     timed word alone;
//   - a timed mode with seconds outside punishMinSecs to punishMaxSecs is warnSettingsBadDuration;
//   - a valid timed mode stores the mode and the seconds in one write.
//
// Nothing is written on a refusal. The repository write invalidates its own cache key. It judges no
// person's rights.
func applyWarnMode(ctx context.Context, chatID int64, mode string, secs int) warnSettingsResult {
	switch {
	case slices.Contains(warnSettingsModes, mode):
		secs = 0
	case !slices.Contains(warnTimedModes, mode), secs == 0:
		return warnSettingsResult{Outcome: warnSettingsUnknownMode, Key: "warns_mode_unknown"}
	case !punishDurationValid(secs):
		return warnSettingsResult{Outcome: warnSettingsBadDuration, Key: "warns_mode_bad_duration"}
	}
	if err := warnSettingsWrites.Mode(ctx, chatID, mode, secs); err != nil {
		return warnSettingsResult{Outcome: warnSettingsSaveFailed, Key: "common_settings_save_failed", Err: err}
	}
	return warnSettingsResult{Key: "warns_mode_updated_" + mode, Stored: warnReadBack(ctx, chatID)}
}

// warnModeArgs reads the arguments of /setwarnmode: the lower-cased first word and the seconds to
// hand applyWarnMode. A timed word followed by a second word gets that word's duration, or -1 when it
// is not a valid duration (applyWarnMode answers bad duration); a timed word alone gets 0, which keeps
// the pinned unknown reply; a plain word gets 0 whatever follows it (/setwarnmode ban 1d stores ban).
func warnModeArgs(args []string) (string, int) {
	mode := strings.ToLower(args[0])
	if !slices.Contains(warnTimedModes, mode) || len(args) < 2 {
		return mode, 0
	}
	secs, ok := parsePunishDuration(args[1])
	if !ok {
		return mode, -1
	}
	return mode, secs
}

func (moduleStruct) setWarnMode(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	connectedChat := chat_status.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	args := ctx.Args()[1:]
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	if !chat_status.RequireBotAdmin(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}

	var replyText string

	if len(args) > 0 {
		mode, secs := warnModeArgs(args)
		result := applyWarnMode(tracing.UpdateContext(ctx), chat.Id, mode, secs)
		switch result.Outcome {
		case warnSettingsSaveFailed:
			log.Errorf("[Warns] SetWarnMode failed for chat %d: %v", chat.Id, result.Err)
			errText, _ := tr.GetString(result.Key)
			_, _ = msg.Reply(b, errText, formatting.Shtml())
			return ext.EndGroups
		case warnSettingsUnknownMode:
			temp, _ := tr.GetString(result.Key)
			replyText = fmt.Sprintf(temp, args[0])
		case warnSettingsBadDuration:
			replyText, _ = tr.GetString(result.Key)
		default:
			if slices.Contains(warnTimedModes, mode) {
				replyText, _ = tr.GetString(result.Key, i18n.TranslationParams{"duration": punishDurationLabel(tr, secs)})
			} else {
				replyText, _ = tr.GetString(result.Key)
			}
		}
	} else {
		replyText, _ = tr.GetString("warns_specify_action")
	}

	_, err := msg.Reply(b, replyText, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

// warnApplyResult is what applyWarn did, so a caller can say it without the warn module's reply.
type warnApplyResult struct {
	// Count is the member's stored warns after this one, Limit and Mode are the group's.
	Count, Limit int
	Mode         string
	// LimitReached is true when Count reached Limit. LimitApplied is true only when the group's
	// limit action took effect and the warns were reset; LimitErr is the action's error, in which
	// case the warn stays counted and nothing is reset.
	LimitReached, LimitApplied bool
	LimitErr                   error
	// Reasons are the stored reasons including this warn's, taken before any reset.
	Reasons []string
	// Recorded is true once the warn is stored, so a returned error with Recorded set is the
	// reset's and one without it is the record's.
	Recorded bool
}

// applyWarn records one warn for userID under warnPunishLock, runs afterRecord (when not nil)
// inside the lock right after the warn is stored, and at the group's warn limit runs the limit
// action (kick, the permanent mute or ban) and resets the member's warns only after the action
// took effect. It never sends a message: the caller words what happened. An error is the warn
// that could not be recorded or the reset that failed; a refused limit action is in LimitErr.
func applyWarn(ctx context.Context, b *gotgbot.Bot, chat *gotgbot.Chat, userID int64, reason string, afterRecord func()) (warnApplyResult, error) {
	var res warnApplyResult
	mu := warnPunishLock(chat.Id, userID)
	mu.Lock()
	defer mu.Unlock()
	// A settings read that fails stores no warn and punishes nobody (fail closed): the
	// caller answers common_settings_save_failed.
	settings, err := warns.GetWarnSettingErrContext(ctx, chat.Id)
	if err != nil {
		return res, err
	}
	numWarns, reasons, err := warns.WarnUser(userID, chat.Id, reason)
	if err != nil {
		return res, err
	}
	res = warnApplyResult{Count: numWarns, Limit: settings.WarnLimit, Mode: settings.WarnMode, Reasons: reasons, Recorded: true}
	if afterRecord != nil {
		afterRecord()
	}
	if numWarns < settings.WarnLimit {
		return res, nil
	}
	res.LimitReached = true
	punished, actionErr := applyWarnLimitAction(ctx, b, chat, userID, settings.WarnMode)
	if actionErr != nil {
		res.LimitErr = actionErr
		return res, nil
	}
	if punished {
		if _, resetErr := warns.ResetUserWarns(userID, chat.Id); resetErr != nil {
			return res, resetErr
		}
		res.LimitApplied = true
	}
	return res, nil
}

// applyWarnLimitAction runs the group's limit action for mode. punished is false for a mode the
// warn module does not know, which applies nothing.
func applyWarnLimitAction(ctx context.Context, b *gotgbot.Bot, chat *gotgbot.Chat, userID int64, mode string) (punished bool, err error) {
	switch mode {
	case "kick":
		err = kickMemberContext(ctx, b, chat.Id, userID)
		if err != nil {
			log.Errorf("[warn] warnlimit: kick (%d) - %s", userID, err)
			return false, err
		}
	case "mute":
		_, err = chat.RestrictMember(b, userID,
			MutedPermissions,
			nil,
		)
		if err != nil {
			log.Errorf("[warn] warnlimit: mute (%d) - %s", userID, err)
			return false, err
		}
	case "ban":
		_, err = chat.BanMember(b, userID, nil)
		if err != nil {
			log.Errorf("[warn] warnlimit: ban (%d) - %s", userID, err)
			return false, err
		}
	default:
		log.Warnf("[Warns] Unknown warn mode: %s", mode)
		return false, nil
	}
	return true, nil
}

// warnSource says who started a warn, so the deletion rules tell a moderator's /warn from an
// automatic one (the blacklist's warn action): each is its own kind (D-13).
type warnSource int

const (
	// warnSourceManual is a moderator's /warn, /dwarn or /swarn.
	warnSourceManual warnSource = iota
	// warnSourceAuto is a warn an automatic rule issued, such as the blacklist's warn action.
	warnSourceAuto
)

// deletionKind is the bot kind the warn message of this source is queued under.
func (s warnSource) deletionKind() string {
	if s == warnSourceAuto {
		return deletion.KindBotAuto
	}
	return deletion.KindBotManual
}

// warnThisUser warns userID in the update's chat and posts the warning (or, at the limit, the
// limit message). The warning goes through the event message engine (postEvent): a group's stored warn
// message replaces the default, keeping the bot's Remove warn row last (D-05), Silent posts nothing,
// and with nothing stored the default is today's request byte for byte. The limit message is the warn
// limit event when the limit action took effect; a stored mode the code applies nothing for keeps
// today's reply (warnPostLimit). source picks the deletion kind,
// the actor ({admin}: the issuer of /warn, /dwarn and /swarn, "an admin" for an automatic warn) and
// whether an automatic rule raised it.
func (moduleStruct) warnThisUser(b *gotgbot.Bot, ctx *ext.Context, userId int64, reason, warnType string, source warnSource) (err error) {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	if chat_status.IsUserAdmin(b, chat.Id, userId) {
		text, _ := tr.GetString("warns_admin_warning_error")
		_, err = msg.Reply(b, text, nil)
		return err
	}

	chatMember, err := chat.GetMember(b, userId, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	u := chatMember.MergeChatMember().User
	res, err := applyWarn(tracing.UpdateContext(ctx), b, chat, userId, reason, func() {
		switch warnType {
		case "dwarn":
			if reply := extraction.ReplyTarget(msg); reply != nil {
				if _, deleteErr := reply.Delete(b, nil); deleteErr != nil {
					log.Errorf("[Warns] Failed to delete message: %v", deleteErr)
				}
			}
		case "swarn":
			_ = helpers.DeleteMessageWithErrorHandling(b, chat.Id, msg.MessageId)
		}
	})
	if err != nil {
		if res.Recorded {
			// The warn is stored and the limit action took effect, but the reset failed.
			return err
		}
		text, _ := tr.GetString("common_settings_save_failed")
		_, sendErr := msg.Reply(b, text, formatting.Shtml())
		if sendErr != nil {
			return sendErr
		}
		return ext.EndGroups
	}
	if res.LimitErr != nil {
		return res.LimitErr
	}

	switch {
	case res.LimitApplied:
		err = postEvent(warnSite(b, ctx, tr, source), warnLimitEventReply(tr, msg.MessageId, u, reason, res))
	case res.LimitReached:
		// A stored mode the code applies nothing for (tban, tmute, empty) is not an event: today's reply.
		err = warnPostLimit(b, chat, msg, tr, u, res, source)
	default:
		err = postEvent(warnSite(b, ctx, tr, source), warnEventReply(b, chat, msg.MessageId, tr, u, reason, res))
	}
	if err != nil {
		log.Errorf("[warn] sendMessage (%d) - %s", userId, err)
		return err
	}
	return ext.EndGroups
}

// warnSite is the warn's eventSite: a reply to the triggering message (the command, or the member's
// own message for an automatic warn) in the group's language, queued under the source's kind. An
// automatic warn has no acting admin ({admin} reads "an admin"); a moderator's names its issuer, or
// "an admin" for an anonymous admin. The default is sent with b.SendMessage, today's call.
func warnSite(b *gotgbot.Bot, ctx *ext.Context, tr *i18n.Translator, source warnSource) eventSite {
	chat := ctx.EffectiveChat
	site := eventSite{
		Bot:       b,
		Chat:      chat,
		Tr:        tr,
		ReplyTo:   ctx.EffectiveMessage.MessageId,
		Kind:      source.deletionKind(),
		Automatic: source == warnSourceAuto,
		SendDefault: func(text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
			return b.SendMessage(chat.Id, text, opts)
		},
	}
	if source != warnSourceAuto {
		site.Actor = eventActor{User: ctx.EffectiveUser, Anonymous: actorIsAnonymous(ctx)}
	}
	return site
}

// warnEventReply is the warn event of one warning: the default built by warnDefaultReply with the
// bot's rows (Remove warn, and the Rules button while the group's rules are set) and the reply to
// the triggering message, the values a custom text fills, and the Remove warn row alone as the row a
// custom message keeps, after its own buttons (D-05, D-15).
func warnEventReply(b *gotgbot.Bot, chat *gotgbot.Chat, replyTo int64, tr *i18n.Translator, u gotgbot.User, reason string, res warnApplyResult) eventReply {
	removeWarnText, _ := tr.GetString("warns_remove_button")
	removeWarn := gotgbot.InlineKeyboardButton{
		Text:         removeWarnText,
		CallbackData: encodeCallbackData("rmWarn", map[string]string{"u": fmt.Sprint(u.Id)}),
	}
	row := []gotgbot.InlineKeyboardButton{removeWarn}
	// The Rules button shows whenever the rules are set (a text, a media or buttons),
	// the same test /rules and a welcome's {rules} button use.
	if rules.GetChatRulesInfo(chat.Id).HasContent() {
		rulesButtonText, _ := tr.GetString("common_rules_button_emoji")
		row = append(row, gotgbot.InlineKeyboardButton{
			Text: rulesButtonText,
			Url:  fmt.Sprintf("t.me/%s?start=rules_%d", b.Username, chat.Id),
		})
	}
	in := eventDefaultInput{Target: u, Reason: reason, Count: res.Count, Limit: res.Limit, Rows: [][]gotgbot.InlineKeyboardButton{row}}
	text, opts := warnDefaultReply(tr, in)
	opts.ReplyParameters = &gotgbot.ReplyParameters{MessageId: replyTo, AllowSendingWithoutReply: true}
	return eventReply{
		Event:        eventmsg.EventWarn,
		Target:       u,
		Values:       eventValues{Reason: reason, Count: res.Count, Limit: res.Limit, Action: eventActionWord(tr, "warn")},
		Default:      text,
		Opts:         opts,
		KeepRows:     [][]gotgbot.InlineKeyboardButton{{removeWarn}},
		KeepRowsLast: true,
	}
}

// warnLimitEventReply is the warn limit event of a limit action that took effect: the default built by
// warnLimitDefaultReply (the limit action's text with the count, the limit and the stored reasons, an
// empty keyboard) with the reply to the triggering message, and the values a custom text fills: the
// reason of the warn that reached the limit, the count, the limit and the translated word of the limit
// action. KeepRows stays nil: the default has no bot row, so a custom message has none either (D-15).
func warnLimitEventReply(tr *i18n.Translator, replyTo int64, u gotgbot.User, reason string, res warnApplyResult) eventReply {
	in := eventDefaultInput{Target: u, Count: res.Count, Limit: res.Limit, Action: res.Mode, Reasons: res.Reasons}
	text, opts := warnLimitDefaultReply(tr, in)
	opts.ReplyParameters = &gotgbot.ReplyParameters{MessageId: replyTo, AllowSendingWithoutReply: true}
	return eventReply{
		Event:   eventmsg.EventWarnLimit,
		Target:  u,
		Values:  eventValues{Reason: reason, Count: res.Count, Limit: res.Limit, Action: eventActionWord(tr, res.Mode)},
		Default: text,
		Opts:    opts,
	}
}

// warnPostLimit posts the warn limit message exactly as before custom messages: the limit action's
// text (none for a mode the code applies nothing for) and the stored reasons, as a reply to the
// triggering message with an empty keyboard, queued under the source's kind.
func warnPostLimit(b *gotgbot.Bot, chat *gotgbot.Chat, msg *gotgbot.Message, tr *i18n.Translator, u gotgbot.User, res warnApplyResult, source warnSource) error {
	var reply string
	limitKeys := map[string]string{
		"kick": "warns_limit_kicked",
		"mute": "warns_limit_muted",
		"ban":  "warns_limit_banned",
	}
	if key, ok := limitKeys[res.Mode]; ok {
		temp, _ := tr.GetString(key)
		reply = fmt.Sprintf(temp, res.Count, res.Limit, formatting.MentionHtml(u.Id, u.FirstName))
	}
	var sb strings.Builder
	for _, warnReason := range res.Reasons {
		fmt.Fprintf(&sb, "\n - %s", html.EscapeString(warnReason))
	}
	reply += sb.String()
	sent, err := b.SendMessage(chat.Id, reply,
		&gotgbot.SendMessageOpts{
			ParseMode: formatting.HTML,
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
				IsDisabled: true,
			},
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId:                msg.MessageId,
				AllowSendingWithoutReply: true,
			},
			ReplyMarkup: &gotgbot.InlineKeyboardMarkup{},
		},
	)
	if err != nil {
		return err
	}
	scheduleBotMessage(sent, source.deletionKind())
	return nil
}

func (m moduleStruct) dispatchWarn(b *gotgbot.Bot, ctx *ext.Context, warnType string) error {
	mc, err := buildModerationCtx(&warnsModule, b, ctx)
	if err != nil {
		return ext.EndGroups
	}
	if !standardModGates(mc) {
		return ext.EndGroups
	}

	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	tr := mc.Tr

	userId, reason := extraction.ExtractUserAndText(b, ctx)
	if userId == -1 {
		return ext.EndGroups
	} else if chat_status.IsChannelId(userId) {
		text, _ := tr.GetString("common_anonymous_user_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("common_no_user_specified")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if !chat_status.IsUserInChat(b, chat, userId) {
		return ext.EndGroups
	}
	if warnType == "dwarn" && replyTargetMismatch(msg, userId) {
		_ = refuseReplyMismatch(mc)
		return ext.EndGroups
	}

	return m.warnThisUser(b, ctx, userId, reason, warnType, warnSourceManual)
}

func (m moduleStruct) warnUser(b *gotgbot.Bot, ctx *ext.Context) error {
	return m.dispatchWarn(b, ctx, "warn")
}

func (m moduleStruct) sWarnUser(b *gotgbot.Bot, ctx *ext.Context) error {
	return m.dispatchWarn(b, ctx, "swarn")
}

func (m moduleStruct) dWarnUser(b *gotgbot.Bot, ctx *ext.Context) error {
	return m.dispatchWarn(b, ctx, "dwarn")
}

func (moduleStruct) warnings(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	if !chat_status.RequireGroup(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_group_only_error", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}

	warnrc := warns.GetWarnSettingContext(tracing.UpdateContext(ctx), chat.Id)
	temp, _ := tr.GetString("warns_settings_display")
	text := fmt.Sprintf(temp, warnrc.WarnLimit, warnModeShown(warnrc.WarnMode, warnrc.WarnModeSecs))
	_, err := msg.Reply(b, text, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

// warnModeShown is the warn mode as /warnings shows it: an effective timed mode as the mode and the
// short duration ("tmute 2h"), anything else as stored (a timed mode with no duration stays the bare
// word, as before Phase 26).
func warnModeShown(mode string, secs int) string {
	if slices.Contains(warnTimedModes, mode) && autoActionEffective(mode, secs) {
		return mode + " " + punishDurationShort(secs)
	}
	return mode
}

func (moduleStruct) warns(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	userId := extraction.ExtractUser(b, ctx)
	if userId == -1 {
		if ctx.EffectiveUser == nil {
			text, _ := tr.GetString("common_anonymous_user_error")
			_, err := msg.Reply(b, text, nil)
			if err != nil {
				log.Error(err)
				return err
			}
			return ext.EndGroups
		}
		userId = ctx.EffectiveUser.Id
	} else if chat_status.IsChannelId(userId) {
		text, _ := tr.GetString("common_anonymous_user_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("common_no_user_specified")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	numWarns, reasons := warns.GetWarnsContext(tracing.UpdateContext(ctx), userId, chat.Id)
	text := ""

	if numWarns != 0 {
		warnrc := warns.GetWarnSettingContext(tracing.UpdateContext(ctx), chat.Id)
		if len(reasons) > 0 {
			temp, _ := tr.GetString("warns_user_warnings_list")
			text = fmt.Sprintf(temp, numWarns, warnrc.WarnLimit)
			var sb strings.Builder
			for _, reason := range reasons {
				fmt.Fprintf(&sb, "\n - %s", reason)
			}
			text += sb.String()
			msgs := formatting.SplitMessage(text)
			for _, msgText := range msgs {
				_, err := msg.Reply(b, msgText, nil)
				if err != nil {
					log.Error(err)
					return err
				}
			}
		} else {
			temp, _ := tr.GetString("warns_user_warnings_no_reasons")
			_, err := msg.Reply(b, fmt.Sprintf(temp, numWarns, warnrc.WarnLimit), nil)
			if err != nil {
				log.Error(err)
				return err
			}
		}
	} else {
		text, _ := tr.GetString("warns_user_no_warnings")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
	}
	return ext.EndGroups
}

func (moduleStruct) rmWarnButton(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	chat := ctx.EffectiveChat
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		return ext.EndGroups
	}

	userMatch := ""
	if decoded, ok := decodeCallbackData(query.Data, "rmWarn"); ok {
		userMatch, _ = decoded.Field("u")
	}
	if userMatch == "" {
		log.Warnf("[Warns] Invalid callback data format: %s", query.Data)
		return answerInvalidCallback(b, ctx, query)
	}
	userId, parseErr := strconv.ParseInt(userMatch, 10, 64)
	if parseErr != nil {
		log.Errorf("[Warns] Failed to parse user ID from callback: %v", parseErr)
		return answerInvalidCallback(b, ctx, query)
	}
	var replyText string

	removed, removeErr := warns.RemoveWarn(userId, chat.Id)
	if removeErr != nil {
		replyText, _ = tr.GetString("error_generic")
	} else if removed {
		temp, _ := tr.GetString("warns_removed_by")
		replyText = fmt.Sprintf(temp, formatting.MentionHtml(user.Id, user.FirstName))
	} else {
		replyText, _ = tr.GetString("warns_no_warns_to_remove")
	}

	if query.Message == nil {
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: replyText})
		return ext.EndGroups
	}

	_, _, err := query.Message.EditText(b, &gotgbot.EditMessageTextOpts{Text: replyText, ParseMode: formatting.HTML})
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = query.Answer(b, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (moduleStruct) setWarnLimit(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	connectedChat := chat_status.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	args := ctx.Args()[1:]
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	if !chat_status.RequireBotAdmin(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}

	var replyText string

	if len(args) == 0 {
		replyText, _ = tr.GetString("warns_limit_set_help")
	} else {
		num, err := strconv.Atoi(args[0])
		if err != nil {
			temp, _ := tr.GetString("warns_invalid_number")
			replyText = fmt.Sprintf(temp, html.EscapeString(args[0]))
		} else {
			result := applyWarnLimit(tracing.UpdateContext(ctx), chat.Id, num)
			switch result.Outcome {
			case warnSettingsSaveFailed:
				log.Errorf("[Warns] SetWarnLimit failed for chat %d: %v", chat.Id, result.Err)
				errText, _ := tr.GetString(result.Key)
				_, _ = msg.Reply(b, errText, formatting.Shtml())
				return ext.EndGroups
			case warnSettingsOutOfRange:
				replyText, _ = tr.GetString(result.Key)
			default:
				temp, _ := tr.GetString(result.Key)
				replyText = fmt.Sprintf(temp, num)
			}
		}
	}

	_, err := msg.Reply(b, replyText, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (moduleStruct) resetWarns(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	if !chat_status.RequireGroup(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_group_only_error", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}

	userId := extraction.ExtractUser(b, ctx)
	if userId == -1 {
		return ext.EndGroups
	} else if chat_status.IsChannelId(userId) {
		text, _ := tr.GetString("common_anonymous_user_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("common_no_user_specified")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	removed, resetErr := warns.ResetUserWarns(userId, chat.Id)
	var text string
	if resetErr != nil {
		text, _ = tr.GetString("error_generic")
	} else if removed {
		text, _ = tr.GetString("warns_reset_success")
	} else {
		text, _ = tr.GetString("warns_no_warns_to_remove")
	}
	_, err := msg.Reply(b, text, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (moduleStruct) resetAllWarns(b *gotgbot.Bot, ctx *ext.Context) error {
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	yesText, _ := tr.GetString("common_yes")
	noText, _ := tr.GetString("common_no")

	if !chat_status.RequireGroup(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_group_only_error", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.RequireUserOwner(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_owner_cmd_error", "chat_status_owner_button_error", chat_status.WithReply())
		return ext.EndGroups
	}

	warnrc := warns.GetAllChatWarns(chat.Id)
	if warnrc == 0 {
		text, _ := tr.GetString("warns_no_users_warned")
		_, err := msg.Reply(b, text, formatting.Shtml())
		return err
	}

	if chat_status.RequireUserOwner(b, ctx, chat, user.Id) {
		text, _ := tr.GetString("warns_reset_all_confirm")
		_, err := msg.Reply(b, text,
			&gotgbot.SendMessageOpts{
				ReplyMarkup: gotgbot.InlineKeyboardMarkup{
					InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
						{
							{
								Text:         yesText,
								CallbackData: encodeCallbackData("rmAllChatWarns", map[string]string{"a": "yes"}),
							},
							{
								Text:         noText,
								CallbackData: encodeCallbackData("rmAllChatWarns", map[string]string{"a": "no"}),
							},
						},
					},
				},
			},
		)
		if err != nil {
			log.Error(err)
			return err
		}
	}

	return ext.EndGroups
}

func (moduleStruct) warnsButtonHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	user := query.From
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	if !chat_status.RequireUserOwner(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_owner_cmd_error", "chat_status_owner_button_error", chat_status.WithReply())
		return ext.EndGroups
	}

	response := ""
	if decoded, ok := decodeCallbackData(query.Data, "rmAllChatWarns"); ok {
		response, _ = decoded.Field("a")
	}
	if response == "" {
		log.Warnf("[Warns] Invalid callback data format: %s", query.Data)
		return answerInvalidCallback(b, ctx, query)
	}
	var helpText string

	var replyText string

	chat := ctx.EffectiveChat
	switch response {
	case "yes":
		if chat == nil {
			helpText, _ = tr.GetString("error_generic")
			replyText = helpText
			break
		}
		if err := warns.ResetAllChatWarns(chat.Id); err == nil {
			helpText, _ = tr.GetString("warns_reset_all_success")
			replyText, _ = tr.GetString("warns_reset_all_final")
		} else {
			helpText, _ = tr.GetString("error_generic")
			replyText = helpText
		}
	case "no":
		helpText, _ = tr.GetString("warns_reset_all_cancelled")
		replyText = helpText
	default:
		return answerInvalidCallback(b, ctx, query)
	}

	if query.Message == nil {
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: helpText})
		return ext.EndGroups
	}

	_, _, err := query.Message.EditText(b, &gotgbot.EditMessageTextOpts{Text: replyText})
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = query.Answer(b,
		&gotgbot.AnswerCallbackQueryOpts{
			Text: helpText,
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (moduleStruct) removeWarn(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	if !chat_status.RequireGroup(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_group_only_error", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}

	userId := extraction.ExtractUser(b, ctx)
	if userId == -1 {
		return ext.EndGroups
	} else if chat_status.IsChannelId(userId) {
		text, _ := tr.GetString("common_anonymous_user_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("common_no_user_specified")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	var replyText string
	removed, removeErr := warns.RemoveWarn(userId, chat.Id)
	if removeErr != nil {
		replyText, _ = tr.GetString("error_generic")
	} else if removed {
		temp, _ := tr.GetString("warns_removed_by")
		replyText = fmt.Sprintf(temp, formatting.MentionHtml(user.Id, user.FirstName))
	} else {
		replyText, _ = tr.GetString("warns_no_warns_to_remove")
	}

	_, err := msg.Reply(b, replyText, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

var (
	warnDesc        = helpers.CommandDescriptor{Name: "warn"}
	swarnDesc       = helpers.CommandDescriptor{Name: "swarn"}
	dwarnDesc       = helpers.CommandDescriptor{Name: "dwarn"}
	resetWarnsDesc  = helpers.CommandDescriptor{Name: "resetwarns", Aliases: []string{"resetwarn"}}
	removeWarnDesc  = helpers.CommandDescriptor{Name: "rmwarn", Aliases: []string{"unwarn"}}
	warnsStatusDesc = helpers.CommandDescriptor{Name: "warns", Disableable: true}
)

func adminBotUserChecks(cmd string) []helpers.CheckFunc {
	return []helpers.CheckFunc{
		helpers.CheckDisabled(cmd),
		helpers.RequireGroup(),
		helpers.RequireBotAdmin(),
		helpers.RequireUserAdmin(),
	}
}

func initWarnDescs() {
	warnDesc.RequiredChecks = restrictChecks("warn")
	swarnDesc.RequiredChecks = restrictChecks("swarn")
	dwarnDesc.RequiredChecks = restrictChecks("dwarn")
	resetWarnsDesc.RequiredChecks = adminBotUserChecks("resetwarns")
	removeWarnDesc.RequiredChecks = adminBotUserChecks("rmwarn")
	warnsStatusDesc.RequiredChecks = []helpers.CheckFunc{helpers.CheckDisabled("warns")}
}

func LoadWarns(dispatcher *ext.Dispatcher) {
	SetModuleEnabled(warnsModule.moduleName, true)
	initWarnDescs()

	helpers.WrapCommand(dispatcher, warnDesc, pipelineHandler(warnsModule.warnUser))
	helpers.WrapCommand(dispatcher, swarnDesc, pipelineHandler(warnsModule.sWarnUser))
	helpers.WrapCommand(dispatcher, dwarnDesc, pipelineHandler(warnsModule.dWarnUser))
	helpers.WrapCommand(dispatcher, resetWarnsDesc, pipelineHandler(warnsModule.resetWarns))
	helpers.WrapCommand(dispatcher, removeWarnDesc, pipelineHandler(warnsModule.removeWarn))
	helpers.WrapCommand(dispatcher, warnsStatusDesc, pipelineHandler(warnsModule.warns))
	for _, cfg := range []struct {
		name    string
		handler func(*gotgbot.Bot, *ext.Context) error
	}{
		{"setwarnlimit", warnsModule.setWarnLimit},
		{"setwarnmode", warnsModule.setWarnMode},
		{"resetallwarns", warnsModule.resetAllWarns},
	} {
		desc := helpers.CommandDescriptor{
			Name:           cfg.name,
			RequiredChecks: []helpers.CheckFunc{helpers.CheckDisabled(cfg.name)},
		}
		helpers.WrapCommand(dispatcher, desc, pipelineHandler(cfg.handler))
	}
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("rmAllChatWarns"), warnsModule.warnsButtonHandler))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "warnings",
		RequiredChecks: []helpers.CheckFunc{
			helpers.CheckDisabled("warnings"),
			helpers.RequireGroup(),
			helpers.RequireBotAdmin(),
			helpers.RequireUserAdmin(),
		},
	}, pipelineHandler(warnsModule.warnings))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("rmWarn"), warnsModule.rmWarnButton))
}

func init() {
	RegisterLegacyModule("Warns", 200, LoadWarns)
	initWarnDescs()
	RegisterAnonymousAdminHandler("warn", anonPipelineHandler(warnDesc, warnsModule.warnUser))
	RegisterAnonymousAdminHandler("swarn", anonPipelineHandler(swarnDesc, warnsModule.sWarnUser))
	RegisterAnonymousAdminHandler("dwarn", anonPipelineHandler(dwarnDesc, warnsModule.dWarnUser))
}
