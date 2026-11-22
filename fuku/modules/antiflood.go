package modules

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/antiflood"
	"github.com/uasneppy/fuku_robot/fuku/db/deletion"
	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/db/lang"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
	"github.com/uasneppy/fuku_robot/fuku/utils/cache"
	"github.com/uasneppy/fuku_robot/fuku/utils/chat_status"
	"github.com/uasneppy/fuku_robot/fuku/utils/error_handling"
	"github.com/uasneppy/fuku_robot/fuku/utils/formatting"
	"github.com/uasneppy/fuku_robot/fuku/utils/helpers"
	"github.com/uasneppy/fuku_robot/fuku/utils/tracing"
)

const (
	maxConcurrentAdminChecks  = 50
	maxConcurrentMsgDeletions = 5
)

type floodKey struct {
	chatId int64
	userId int64
}

type antifloodStruct struct {
	moduleStruct
	syncHelperMap       sync.Map
	adminCheckSemaphore chan struct{}
}

type floodControl struct {
	userId       int64
	messageCount int
	messageIDs   []int64
	lastActivity int64
}

var _normalAntifloodModule = moduleStruct{
	moduleName:   "Antiflood",
	handlerGroup: 4,
}

var antifloodModule = antifloodStruct{
	moduleStruct:        _normalAntifloodModule,
	syncHelperMap:       sync.Map{},
	adminCheckSemaphore: make(chan struct{}, maxConcurrentAdminChecks),
}

func init() {
	RegisterLegacyModule("Antiflood", 150, LoadAntiflood)
	go func() {
		defer error_handling.RecoverFromPanic("cleanupLoop", "antiflood")
		antifloodModule.cleanupLoop(context.Background())
	}()
}

func (a *antifloodStruct) cleanupOnce(now int64) {
	a.syncHelperMap.Range(func(key, value any) bool {
		// States are *floodControl so CompareAndDelete compares pointers:
		// values hold slices and are not comparable.
		floodData, ok := value.(*floodControl)
		if !ok || floodData == nil || now-floodData.lastActivity <= 600 {
			return true
		}
		// Delete only if still stale: a concurrent updateFlood CAS-stores a fresh
		// value and CompareAndDelete fails, keeping the new burst intact.
		a.syncHelperMap.CompareAndDelete(key, value)
		return true
	})
}

func (a *antifloodStruct) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			func() {
				defer error_handling.RecoverFromPanic("cleanupOnce", "antiflood")
				a.cleanupOnce(time.Now().Unix())
			}()
		case <-ctx.Done():
			log.Info("Antiflood cleanup goroutine shutting down gracefully")
			return
		}
	}
}

func cachedAdminStatus(chatId, userId int64) (known bool, isAdmin bool) {
	ok, cached := cache.GetAdminCacheList(chatId)
	if !ok || !cached.Cached || cached.Negative || len(cached.UserInfo) == 0 {
		return false, false
	}
	if cached.UserMap != nil {
		_, isAdmin = cached.UserMap[userId]
		return true, isAdmin
	}
	for i := range cached.UserInfo {
		if cached.UserInfo[i].User.Id == userId {
			return true, true
		}
	}
	return true, false
}

func (a *antifloodStruct) userIsFloodExempt(b *gotgbot.Bot, chatId, userId int64) bool {
	if known, isAdmin := cachedAdminStatus(chatId, userId); known {
		return isAdmin
	}
	return a.adminCheckWithTimeout(b, chatId, userId)
}

func (a *antifloodStruct) adminCheckWithTimeout(b *gotgbot.Bot, chatId, userId int64) bool {
	select {
	case a.adminCheckSemaphore <- struct{}{}:
	default:
		log.WithFields(log.Fields{
			"chatId": chatId,
			"userId": userId,
		}).Warn("Admin check semaphore full - assuming admin to prevent false positives")
		return true
	}

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result := make(chan bool, 1)
	go func() {
		// Release the slot when the real work finishes, not when the caller
		// times out: the old defer released early, unbounding live lookups.
		defer func() { <-a.adminCheckSemaphore }()
		defer error_handling.RecoverFromPanic("adminCheck", "antiflood")
		isAdmin := chat_status.IsUserAdmin(b, chatId, userId)
		select {
		case result <- isAdmin:
		case <-ctxTimeout.Done():
		}
	}()

	select {
	case isAdmin := <-result:
		return isAdmin
	case <-ctxTimeout.Done():
		log.WithFields(log.Fields{
			"chatId": chatId,
			"userId": userId,
		}).Warn("Admin check timed out, skipping flood check to prevent false positives")
		return true
	}
}

func (a *antifloodStruct) updateFlood(chatId, userId, msgId int64) (shouldPunish bool, floodCrc floodControl, floodSettings *db.AntifloodSettings) {
	floodSettings = antiflood.GetFlood(chatId)
	shouldPunish, floodCrc = a.updateFloodWithSettings(chatId, userId, msgId, floodSettings)
	return
}

// updateFloodWithSettings is updateFlood with the settings row already in hand,
// so callers that loaded it (checkFlood) do not read the cache a second time.
func (a *antifloodStruct) updateFloodWithSettings(chatId, userId, msgId int64, floodSettings *db.AntifloodSettings) (shouldPunish bool, floodCrc floodControl) {
	if floodSettings.Limit != 0 {
		currentTime := time.Now().Unix()
		key := floodKey{chatId: chatId, userId: userId}

		// Lock-free RMW on *floodControl (pointers are comparable for CAS; the
		// old floodMu protocol let the cleaner mint a second mutex → lost counts).
		for {
			var cur floodControl
			old, loaded := a.syncHelperMap.Load(key)
			if loaded && old != nil {
				// Keep loaded set for an idle pointer. It stays in the map until the
				// 10-minute cleaner, so treating it as absent makes LoadOrStore fail
				// and this loop never returns. CompareAndSwap replaces it instead.
				if prev, ok := old.(*floodControl); ok && prev != nil && currentTime-prev.lastActivity <= 60 {
					cur = *prev
				}
			}
			if cur.userId == 0 {
				cur.userId = userId
				cur.messageCount = 0
				cur.messageIDs = make([]int64, 0, floodSettings.Limit+5)
			} else {
				next := make([]int64, len(cur.messageIDs), cap(cur.messageIDs))
				copy(next, cur.messageIDs)
				cur.messageIDs = next
			}
			cur.messageCount++
			cur.lastActivity = currentTime
			cur.messageIDs = append(cur.messageIDs, msgId)
			if len(cur.messageIDs) > floodSettings.Limit+5 {
				cur.messageIDs = cur.messageIDs[len(cur.messageIDs)-(floodSettings.Limit+5):]
			}
			next := cur
			punish := false
			if cur.messageCount > floodSettings.Limit {
				// Reset the stored counter but hand the burst to the caller: it
				// bulk-deletes the tracked IDs (the old reset returned an empty
				// list, deleting nothing).
				next = floodControl{lastActivity: currentTime, messageIDs: make([]int64, 0)}
				punish = true
			}
			stored := &floodControl{
				userId:       next.userId,
				messageCount: next.messageCount,
				messageIDs:   next.messageIDs,
				lastActivity: next.lastActivity,
			}
			var swapped bool
			if !loaded {
				_, swapped = a.syncHelperMap.LoadOrStore(key, stored)
				swapped = !swapped
			} else {
				swapped = a.syncHelperMap.CompareAndSwap(key, old, stored)
			}
			if swapped {
				shouldPunish = punish
				floodCrc = cur
				break
			}
		}
	}

	return
}

func (m *moduleStruct) checkFlood(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender
	if user == nil {
		return ext.ContinueGroups
	}
	// Linked-channel auto-forwards are the group's own channel posts, and their
	// sender ID is a channel, which restrictChatMember/banChatMember reject.
	if user.IsAnonymousAdmin() || user.IsLinkedChannel() {
		return ext.ContinueGroups
	}
	msg := ctx.EffectiveMessage
	if msg.MediaGroupId != "" {
		return ext.ContinueGroups
	}

	chatId := chat.Id
	// Cheapest discriminator first: an unconfigured chat (Limit == 0) pays only
	// this one cached read, not the translator/admin/approval lookups below.
	flood := antiflood.GetFlood(chatId)
	if flood.Limit == 0 {
		return ext.ContinueGroups
	}

	userId := user.Id()

	if antifloodModule.userIsFloodExempt(b, chatId, userId) {
		return ext.ContinueGroups
	}

	if chat_status.IsApprovedForUpdate(b, ctx, chatId, userId) {
		return ext.ContinueGroups
	}

	flooded, floodCrc := antifloodModule.updateFloodWithSettings(chatId, userId, msg.MessageId, flood)
	if !flooded {
		return ext.ContinueGroups
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	p := autoPunishment{Action: flood.Action, Secs: flood.ActionSecs}

	// Every action that punishes (mute, kick, ban, and a timed mute or ban with a duration) needs the
	// right to restrict; warn and a timed action stored with no duration apply nothing and need none.
	if autoActionEffective(p.Action, p.Secs) {
		if !chat_status.CanBotRestrict(b, ctx, chat) {
			log.WithFields(log.Fields{
				"chatId": chatId,
			}).Debug("Antiflood action skipped: bot lacks restrict permissions")
			return ext.ContinueGroups
		}
	}

	if flood.DeleteAntifloodMessage {
		var firstError error
		var errorMu sync.Mutex

		recordError := func(err error, msgId int64) {
			if err != nil {
				log.Errorf("Failed to delete flood message %d: %v", msgId, err)
				errorMu.Lock()
				if firstError == nil {
					firstError = err
				}
				errorMu.Unlock()
			}
		}

		if len(floodCrc.messageIDs) <= 3 {
			for _, i := range floodCrc.messageIDs {
				err := helpers.DeleteMessageWithErrorHandling(b, chatId, i)
				recordError(err, i)
			}
		} else {
			sem := make(chan struct{}, maxConcurrentMsgDeletions)
			var wg sync.WaitGroup

			for _, msgId := range floodCrc.messageIDs {
				wg.Add(1)
				sem <- struct{}{}

				go func(messageId int64) {
					defer wg.Done()
					defer func() { <-sem }()
					defer error_handling.RecoverFromPanic("floodMsgDelete", "antiflood")

					err := helpers.DeleteMessageWithErrorHandling(b, chatId, messageId)
					recordError(err, messageId)
				}(msgId)
			}

			wg.Wait()
		}

		if firstError != nil {
			log.Warnf("[Antiflood] Some flood messages could not be deleted (may be too old): %v", firstError)
		}
	} else {
		_ = helpers.DeleteMessageWithErrorHandling(b, chatId, msg.MessageId)
	}

	rows, applied, notice, err := floodPunish(b, chat, tr, user, p)
	if err != nil {
		return err
	}
	if !notice {
		return ext.ContinueGroups
	}
	if err := floodNotice(b, chat, msg, tr, user, applied, flood.Limit, floodCrc.messageCount, rows); err != nil {
		log.Error(err)
		return err
	}
	return ext.ContinueGroups
}

// floodUnmuteRows is the admins' Unmute row of a flood mute, plain or timed.
func floodUnmuteRows(tr *i18n.Translator, userID int64) [][]gotgbot.InlineKeyboardButton {
	return [][]gotgbot.InlineKeyboardButton{
		{
			{
				Text:         trS(tr, "button_unmute_admins"),
				CallbackData: encodeCallbackData("unrestrict", map[string]string{"a": "unmute", "u": fmt.Sprint(userID)}),
			},
		},
	}
}

// floodPunish applies p to the flooding sender, the one place the flood's action is decided (plan
// 26-09 adds the escalation ladder here). It returns the bot's rows of the notice, the punishment as
// applied (Until set for a timed one) and whether a notice is due; an error is a punishment Telegram
// refused, logged here, after which nothing is posted.
//   - mute, kick and ban send today's requests with today's rows: a mute or a kick of a sender chat
//     sends nothing and posts nothing, a ban of a sender chat bans the chat and carries the Unban row;
//   - an effective timed mute or ban (a duration of 1m to 366d) goes through applyTimedPunishment, a
//     timed mute keeping the Unmute row and a timed ban carrying none, and a sender chat gets nothing
//     and no notice (D-13: a sender chat has no timed form, and banSenderChat would ban it for good);
//   - anything else (warn, a timed action stored with no duration) applies nothing and posts today's
//     inert notice.
func floodPunish(b *gotgbot.Bot, chat *gotgbot.Chat, tr *i18n.Translator, user *gotgbot.Sender, p autoPunishment) (
	rows [][]gotgbot.InlineKeyboardButton, applied autoPunishment, notice bool, err error) {
	if p.Timed() && autoActionEffective(p.Action, p.Secs) {
		return floodPunishTimed(b, chat, tr, user, p)
	}
	userID := user.Id()
	switch p.Action {
	case "mute":
		if !user.IsUser() {
			return nil, p, false, nil
		}
		rows = floodUnmuteRows(tr, userID)
		if _, err = chat.RestrictMember(b, userID, MutedPermissions, nil); err != nil {
			log.Errorf(" checkFlood: %d (%d) - %v", chat.Id, userID, err)
			return nil, p, false, err
		}
	case "kick":
		if !user.IsUser() {
			return nil, p, false, nil
		}
		if err = kickMember(b, chat.Id, userID); err != nil {
			log.Errorf(" checkFlood: %d (%d) - %v", chat.Id, userID, err)
			return nil, p, false, err
		}
	case "ban":
		if user.IsUser() {
			_, err = chat.BanMember(b, userID, nil)
		} else {
			rows = [][]gotgbot.InlineKeyboardButton{
				{
					{
						Text:         trS(tr, "antiflood_button_unban_admins"),
						CallbackData: encodeCallbackData("unrestrict", map[string]string{"a": "unban", "u": fmt.Sprint(userID)}),
					},
				},
			}
			_, err = chat.BanSenderChat(b, userID, nil)
		}
		if err != nil {
			log.Errorf(" checkFlood: %d (%d) - %v", chat.Id, userID, err)
			return nil, p, false, err
		}
	}
	return rows, p, true, nil
}

// floodPunishTimed applies an effective timed mute or ban of the flooding member through
// applyTimedPunishment, whose end date it records on the punishment. A sender chat gets nothing and no
// notice. An end date refused locally (errPunishTimeRanOut, logged by the helper) punishes nobody: the
// notice is today's inert one, with no row and no duration.
func floodPunishTimed(b *gotgbot.Bot, chat *gotgbot.Chat, tr *i18n.Translator, user *gotgbot.Sender, p autoPunishment) (
	rows [][]gotgbot.InlineKeyboardButton, applied autoPunishment, notice bool, err error) {
	if !user.IsUser() {
		return nil, p, false, nil
	}
	userID := user.Id()
	p.Until, err = applyTimedPunishment(b, chat.Id, userID, p.Action, p.Secs)
	switch {
	case errors.Is(err, errPunishTimeRanOut):
		return nil, autoPunishment{Action: p.Action}, true, nil
	case err != nil:
		log.Errorf(" checkFlood: %d (%d) - %v", chat.Id, userID, err)
		return nil, p, false, err
	}
	if p.Action == "tmute" {
		rows = floodUnmuteRows(tr, userID)
	}
	return rows, p, true, nil
}

// floodNotice posts the flood notice after the punishment (D-03). A mute, a kick, a ban and an applied
// timed mute or ban are the flood event (postEvent): a group's stored flood message replaces the default,
// keeping the bot's Unmute or Unban row first (the rows of the default's options, D-15), with {action} the
// translated word, {reason} the fixed flood reason, {count} the messages of the burst and {limit} the
// flood limit; a timed punishment also fills {duration} (the duration in the group's language) and
// {until} (the end date its request was sent with, in the chat's zone, D-16). A stored inert action (warn,
// a timed action with no duration) applies nothing and is not an event: its notice is today's, sent as
// is. The default is today's request byte for byte for every untimed action: no reply, the message's
// topic, sent through helpers.SendMessageWithErrorHandling (which answers no message for a chat the bot
// cannot post in) and queued as an automatic punishment message (D-11, D-17); a timed punishment's default
// names the duration.
func floodNotice(b *gotgbot.Bot, chat *gotgbot.Chat, msg *gotgbot.Message, tr *i18n.Translator, sender *gotgbot.Sender,
	p autoPunishment, limit, count int, rows [][]gotgbot.InlineKeyboardButton) error {
	target := eventSenderTarget(sender)
	timed := p.Timed() && p.Until > 0
	in := eventDefaultInput{Target: target, Action: p.Action, Rows: rows}
	if timed {
		in.Secs = p.Secs
	}
	text, opts := floodDefaultReply(tr, in)
	opts.MessageThreadId = msg.MessageThreadId
	site := eventSite{
		Bot:       b,
		Chat:      chat,
		Tr:        tr,
		ThreadID:  msg.MessageThreadId,
		Kind:      deletion.KindBotAuto,
		Automatic: true,
		SendDefault: func(text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
			return helpers.SendMessageWithErrorHandling(b, chat.Id, text, opts)
		},
	}
	values := eventValues{Reason: floodEventReason(tr), Count: count, Limit: limit, Action: eventActionWord(tr, p.Action)}
	if timed {
		values.Duration, values.Until = punishDurationLabel(tr, p.Secs), time.Unix(p.Until, 0)
	}
	r := eventReply{
		Event:   eventmsg.EventFlood,
		Target:  target,
		Values:  values,
		Default: text,
		Opts:    opts,
	}
	if _, acts := floodWords[p.Action]; !acts && !timed {
		_, err := sendDefaultPost(site, r)
		return err
	}
	return postEvent(site, r)
}

// floodOffWords are the arguments /setflood reads as "turn antiflood off".
var floodOffWords = []string{"off", "no", "false", "0"}

// floodModes are the plain actions a command or the settings page may write, with seconds 0.
// warn stays valid in the database check but is never written (D-20).
var floodModes = []string{"ban", "kick", "mute"}

// floodTimedModes are the timed actions (Phase 26, D-09): written only with a duration of
// punishMinSecs to punishMaxSecs, and inert when stored with none.
var floodTimedModes = []string{"tban", "tmute"}

// floodApplyOutcome is what an apply function did with its input.
type floodApplyOutcome int

const (
	// floodApplied is the zero value: the value was valid and stored.
	floodApplied floodApplyOutcome = iota
	// floodDisabled means a limit of 0 was stored, which turns antiflood off.
	floodDisabled
	// floodOutOfRange means the limit was neither 0 nor 3..100; nothing was written.
	floodOutOfRange
	// floodUnknownMode means the action was not ban, kick or mute, or a timed action came with
	// no duration; nothing was written.
	floodUnknownMode
	// floodSaveFailed means the repository returned an error; Err holds it.
	floodSaveFailed
	// floodBadDuration means a timed action (tban, tmute) came with a duration outside
	// punishMinSecs to punishMaxSecs; nothing was written.
	floodBadDuration
)

// floodApplyResult is the verdict of an apply function. Stored is the row read back from the
// database after a successful write (nil otherwise, or when the read-back failed), so a caller
// shows what is stored and never what it assumes.
type floodApplyResult struct {
	Outcome floodApplyOutcome
	Stored  *models.AntifloodSettings
	Err     error
}

// floodReadBack reads the chat's antiflood row uncached after a write. A failed read-back is
// logged and returns nil; the write already happened.
func floodReadBack(ctx context.Context, chatID int64) *models.AntifloodSettings {
	stored, err := antiflood.GetAntifloodSettingsContext(ctx, chatID)
	if err != nil {
		log.Warnf("[Antiflood] read-back after write failed for chat %d: %v", chatID, err)
		return nil
	}
	return stored
}

// floodSaved turns the result of a repository write into a floodApplyResult.
func floodSaved(ctx context.Context, chatID int64, ok floodApplyOutcome, err error) floodApplyResult {
	if err != nil {
		return floodApplyResult{Outcome: floodSaveFailed, Err: err}
	}
	return floodApplyResult{Outcome: ok, Stored: floodReadBack(ctx, chatID)}
}

// applyFloodLimit validates and stores the antiflood limit: 0 turns antiflood off, 3..100 sets
// the limit, anything else writes nothing. The repository write invalidates its own cache key.
func applyFloodLimit(ctx context.Context, chatID int64, limit int) floodApplyResult {
	switch {
	case limit == 0:
		return floodSaved(ctx, chatID, floodDisabled, antiflood.SetFloodContext(ctx, chatID, 0))
	case limit >= 3 && limit <= 100:
		return floodSaved(ctx, chatID, floodApplied, antiflood.SetFloodContext(ctx, chatID, limit))
	default:
		return floodApplyResult{Outcome: floodOutOfRange}
	}
}

// applyFloodMode validates and stores the antiflood action and its duration in seconds, exactly
// as spelled; the caller lower-cases user text first (D-09, the rules of applyWarnMode):
//   - a plain action (ban, kick, mute) stores seconds 0, clearing any duration, whatever secs says;
//   - a timed action (tban, tmute) with seconds 0 is floodUnknownMode, the pinned reply of a timed
//     word alone;
//   - a timed action with seconds outside punishMinSecs to punishMaxSecs is floodBadDuration;
//   - a valid timed action stores the action and the seconds in one write.
//
// Nothing is written on a refusal. The repository write invalidates its own cache key. It judges
// no person's rights.
func applyFloodMode(ctx context.Context, chatID int64, mode string, secs int) floodApplyResult {
	switch {
	case slices.Contains(floodModes, mode):
		secs = 0
	case !slices.Contains(floodTimedModes, mode), secs == 0:
		return floodApplyResult{Outcome: floodUnknownMode}
	case !punishDurationValid(secs):
		return floodApplyResult{Outcome: floodBadDuration}
	}
	return floodSaved(ctx, chatID, floodApplied, antiflood.SetFloodActionContext(ctx, chatID, mode, secs))
}

// applyFloodDelete stores whether messages that caused a flood are deleted.
func applyFloodDelete(ctx context.Context, chatID int64, on bool) floodApplyResult {
	return floodSaved(ctx, chatID, floodApplied, antiflood.SetFloodMsgDelContext(ctx, chatID, on))
}

func (m *moduleStruct) setFlood(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	connectedChat := chat_status.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	args := ctx.Args()[1:]

	var replyText string

	if len(args) == 0 {
		replyText, _ = tr.GetString(strings.ToLower(m.moduleName) + "_errors_expected_args")
	} else {
		limit, parsed := 0, true
		if !slices.Contains(floodOffWords, strings.ToLower(args[0])) {
			num, err := strconv.Atoi(args[0])
			if err != nil {
				parsed = false
				replyText, _ = tr.GetString(strings.ToLower(m.moduleName) + "_errors_invalid_int")
			} else if num == 0 {
				// "00" or "-0" is not an off word; the command has always called it out of range.
				num = -1
			}
			limit = num
		}
		if parsed {
			result := applyFloodLimit(tracing.UpdateContext(ctx), chat.Id, limit)
			switch result.Outcome {
			case floodSaveFailed:
				log.Errorf("[Antiflood] SetFlood failed for chat %d: %v", chat.Id, result.Err)
				errText, _ := tr.GetString("common_settings_save_failed")
				_, _ = msg.Reply(b, errText, formatting.Shtml())
				return ext.EndGroups
			case floodDisabled:
				replyText, _ = tr.GetString(strings.ToLower(m.moduleName) + "_setflood_disabled")
			case floodApplied:
				temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_setflood_success")
				replyText = fmt.Sprintf(temp, limit)
			default:
				replyText, _ = tr.GetString(strings.ToLower(m.moduleName) + "_errors_set_in_limit")
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

func (m *moduleStruct) flood(b *gotgbot.Bot, ctx *ext.Context) error {
	var text string
	msg := ctx.EffectiveMessage

	if chat_status.CheckDisabledCmd(b, msg, "flood") {
		return ext.EndGroups
	}
	connectedChat := chat_status.IsUserConnected(b, ctx, false, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	flood := antiflood.GetFloodContext(tracing.UpdateContext(ctx), chat.Id)
	if flood.Limit == 0 {
		text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_flood_disabled")
	} else {
		temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_flood_show_settings")
		text = fmt.Sprintf(temp, flood.Limit, floodShownAction(flood.Action, flood.ActionSecs))
	}
	_, err := msg.Reply(b, text, formatting.Shtml())
	if err != nil {
		return err
	}
	return ext.EndGroups
}

// floodShownAction is the untranslated word /flood shows for a stored action: muted, banned or kicked
// for a plain action, the mute or ban word followed by " for " and the short duration ("muted for
// 30m") for an effective timed action, and nothing for anything else (warn, or a timed action stored
// with no duration, as before Phase 26).
func floodShownAction(action string, secs int) string {
	switch {
	case action == "tmute" && autoActionEffective(action, secs):
		return "muted for " + punishDurationShort(secs)
	case action == "tban" && autoActionEffective(action, secs):
		return "banned for " + punishDurationShort(secs)
	}
	return floodWords[action]
}

// floodModeArgs reads the arguments of /setfloodmode: the lower-cased first word and the seconds to
// hand applyFloodMode. A timed word followed by a second word gets that word's duration, or -1 when
// it is not a valid duration (applyFloodMode answers bad duration); a timed word alone gets 0, which
// keeps the pinned unknown reply; a plain word gets 0 whatever follows it (/setfloodmode ban x stores
// ban).
func floodModeArgs(args []string) (string, int) {
	mode := strings.ToLower(args[0])
	if !slices.Contains(floodTimedModes, mode) || len(args) < 2 {
		return mode, 0
	}
	secs, ok := parsePunishDuration(args[1])
	if !ok {
		return mode, -1
	}
	return mode, secs
}

func (m *moduleStruct) setFloodMode(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	connectedChat := chat_status.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	args := ctx.Args()[1:]

	if len(args) > 0 {
		selectedMode, secs := floodModeArgs(args)
		result := applyFloodMode(tracing.UpdateContext(ctx), chat.Id, selectedMode, secs)
		switch result.Outcome {
		case floodSaveFailed:
			log.Errorf("[Antiflood] SetFloodMode failed for chat %d: %v", chat.Id, result.Err)
			errText, _ := tr.GetString("common_settings_save_failed")
			_, _ = msg.Reply(b, errText, formatting.Shtml())
			return ext.EndGroups
		case floodApplied:
			shown := selectedMode
			if slices.Contains(floodTimedModes, selectedMode) {
				shown += " " + punishDurationShort(secs)
			}
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_setfloodmode_success")
			_, err := msg.Reply(b, fmt.Sprintf(temp, shown), formatting.Shtml())
			if err != nil {
				log.Error(err)
			}
			return ext.EndGroups
		case floodBadDuration:
			text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_setfloodmode_bad_duration")
			_, err := msg.Reply(b, text, formatting.Shtml())
			if err != nil {
				return err
			}
		default:
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_setfloodmode_unknown_type")
			_, err := msg.Reply(b, fmt.Sprintf(temp, args[0]), formatting.Shtml())
			if err != nil {
				return err
			}
		}
	} else {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_setfloodmode_specify_action")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			return err
		}
	}
	return ext.EndGroups
}

func (m *moduleStruct) setFloodDeleter(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	connectedChat := chat_status.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	args := ctx.Args()[1:]
	var text string

	if len(args) > 0 {
		selectedMode := strings.ToLower(args[0])
		switch selectedMode {
		case "on", "yes", "off", "no":
			on := selectedMode == "on" || selectedMode == "yes"
			if result := applyFloodDelete(tracing.UpdateContext(ctx), chat.Id, on); result.Outcome == floodSaveFailed {
				log.Errorf("[Antiflood] SetFloodMsgDel failed for chat %d: %v", chat.Id, result.Err)
				errText, _ := tr.GetString("common_settings_save_failed")
				_, _ = msg.Reply(b, errText, formatting.Shtml())
				return ext.EndGroups
			}
			if on {
				text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_flood_deleter_enabled")
			} else {
				text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_flood_deleter_disabled")
			}
		default:
			text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_flood_deleter_invalid_option")
		}
	} else {
		currSet := antiflood.GetFloodContext(tracing.UpdateContext(ctx), chat.Id).DeleteAntifloodMessage
		if currSet {
			text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_flood_deleter_already_enabled")
		} else {
			text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_flood_deleter_already_disabled")
		}
	}
	_, err := msg.Reply(b, text, formatting.Shtml())
	if err != nil {
		return err
	}

	return ext.EndGroups
}

func LoadAntiflood(dispatcher *ext.Dispatcher) {
	SetModuleEnabled(antifloodModule.moduleName, true)

	dispatcher.AddHandler(helpers.NewBotCommand("setflood", antifloodModule.setFlood))
	dispatcher.AddHandler(helpers.NewBotCommand("setfloodmode", antifloodModule.setFloodMode))
	dispatcher.AddHandler(helpers.NewBotCommand("delflood", antifloodModule.setFloodDeleter))
	dispatcher.AddHandler(helpers.NewBotCommand("clearflood", antifloodModule.setFloodDeleter))
	dispatcher.AddHandler(helpers.NewBotCommand("flood", antifloodModule.flood))
	helpers.AddCmdToDisableable("flood")
	dispatcher.AddHandlerToGroup(handlers.NewMessage(message.All, antifloodModule.checkFlood), antifloodModule.handlerGroup)
}
