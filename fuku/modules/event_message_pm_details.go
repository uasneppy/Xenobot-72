package modules

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/lang"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
	"github.com/uasneppy/fuku_robot/fuku/utils/formatting"
	"github.com/uasneppy/fuku_robot/fuku/utils/media"
)

// The Details button of a private copy (Phase 22, PMSG-04, D-19, D-20). When a private copy job could not
// deliver its copy and the group's Details option is on, the job stores the copy, exactly as it tried to
// send it, in a short-lived Redis record behind a random token, and adds one URL button to the group
// message that opens the bot's private chat with /start pmcopy_<token>. The deep link shows the record to
// the punished person only: the payload is attacker-controlled and grants nothing by itself, the only
// authority is that the record's user ID equals the ID of the person who opened it. Every other case (a
// malformed payload, an unknown or expired token, a record that does not parse, a Redis error, another
// person) gets one neutral reply that names no group, event or person. No log line carries the token, the
// copy, a name or a file ID.

const (
	// pmCopyDetailsPrefix is the deep link prefix of a Details record.
	pmCopyDetailsPrefix = "pmcopy_"
	// pmCopyDetailsTTL is how long a Details record lives: longer than the 24 hour cap of the Bot messages
	// timer, so the record outlives every group message that can carry its button.
	pmCopyDetailsTTL = 48 * time.Hour
	// pmCopyTokenBytes is the random part of a token: 128 bits from crypto/rand.
	pmCopyTokenBytes = 16
	// pmCopyTokenLen is the length of a token in unpadded base64url.
	pmCopyTokenLen = 22
	// pmCopyMaxButtons is Telegram's limit of buttons in one inline keyboard.
	pmCopyMaxButtons = 100
	// pmCopyResolvedTTL bounds how long a resolved group action message is remembered so a delayed
	// pmCopyAddDetails edit cannot overwrite the cleared keyboard and resurrect an action button.
	pmCopyResolvedTTL = 2 * time.Minute
	// pmCopyResolvedMax caps the in-process resolved-action map.
	pmCopyResolvedMax = 4096
)

var (
	// errPMCopyActionResolved signals that the group message's interactive action button was already
	// resolved before pmCopyAddDetails reached Telegram.
	errPMCopyActionResolved = errors.New("private copy details: group action already resolved")
	pmCopyResolvedMu        sync.Mutex
	pmCopyResolvedActions   = map[[2]int64]time.Time{}
)

func pmCopyResolvedKey(chatID, msgID int64) string {
	return "fuku:pmcopy:{" + strconv.FormatInt(chatID, 10) + "}:res:" + strconv.FormatInt(msgID, 10)
}

func pmCopyHasInteractiveRows(rows [][]gotgbot.InlineKeyboardButton) bool {
	for _, row := range rows {
		for _, btn := range row {
			if btn.CallbackData != "" {
				return true
			}
		}
	}
	return false
}

// pmCopyMarkGroupActionResolved records that an interactive action button on (chatID, msgID) has been
// resolved and its keyboard cleared, so a racing pmCopyAddDetails edit will not restore stale rows.
func pmCopyMarkGroupActionResolved(chatID, msgID int64) {
	if chatID == 0 || msgID == 0 {
		return
	}
	now := pmCopyNow()
	key := [2]int64{chatID, msgID}
	pmCopyResolvedMu.Lock()
	if len(pmCopyResolvedActions) >= pmCopyResolvedMax {
		for k, exp := range pmCopyResolvedActions {
			if !now.Before(exp) {
				delete(pmCopyResolvedActions, k)
			}
		}
		if len(pmCopyResolvedActions) >= pmCopyResolvedMax {
			for k := range pmCopyResolvedActions {
				delete(pmCopyResolvedActions, k)
				break
			}
		}
	}
	pmCopyResolvedActions[key] = now.Add(pmCopyResolvedTTL)
	pmCopyResolvedMu.Unlock()

	if client := pmCopyRedisClient(); client != nil {
		setCtx, cancel := context.WithTimeout(context.Background(), raidRedisTimeout)
		defer cancel()
		_ = client.Set(setCtx, pmCopyResolvedKey(chatID, msgID), "1", pmCopyResolvedTTL).Err()
	}
}

func pmCopyGroupActionResolved(ctx context.Context, chatID, msgID int64) bool {
	if chatID == 0 || msgID == 0 {
		return false
	}
	now := pmCopyNow()
	key := [2]int64{chatID, msgID}
	pmCopyResolvedMu.Lock()
	if exp, ok := pmCopyResolvedActions[key]; ok {
		if now.Before(exp) {
			pmCopyResolvedMu.Unlock()
			return true
		}
		delete(pmCopyResolvedActions, key)
	}
	pmCopyResolvedMu.Unlock()

	if client := pmCopyRedisClient(); client != nil {
		checkCtx, cancel := context.WithTimeout(ctx, raidRedisTimeout)
		defer cancel()
		if n, err := client.Exists(checkCtx, pmCopyResolvedKey(chatID, msgID)).Result(); err == nil && n > 0 {
			return true
		}
	}
	return false
}

// pmCopyBotUsername is the bot username the Details URL is built with. A variable only so a test can take it
// away; the default is the cached username every deep link button uses.
var pmCopyBotUsername = getBotUsername

// pmCopyDetailsKey is the Redis key of a Details record: fuku:pmcopy:det:<token>, outside the cache prefix.
func pmCopyDetailsKey(token string) string {
	return "fuku:pmcopy:det:" + token
}

// pmCopyDetailsRecord is one undelivered private copy: the punished user (u, the only person the record is
// ever shown to), the group (c), the event (e), the copy as the job tried to send it (t the text, f the
// file ID, y the media type, b the URL button rows), and for an unvalidated custom copy its default fallback
// text (fb) and appeal button row (ar) in case Telegram refuses the custom payload when opened. It never
// carries the Staff Group or another group.
type pmCopyDetailsRecord struct {
	UserID    int64                            `json:"u"`
	ChatID    int64                            `json:"c"`
	Event     string                           `json:"e"`
	Text      string                           `json:"t"`
	FileID    string                           `json:"f,omitempty"`
	MediaType int                              `json:"y"`
	Buttons   [][]gotgbot.InlineKeyboardButton `json:"b,omitempty"`
	Fallback  string                           `json:"fb,omitempty"`
	AppealRow []gotgbot.InlineKeyboardButton   `json:"ar,omitempty"`
}

// errPMCopyNoRedis is returned when a Details record cannot be stored or read because there is no Redis.
var errPMCopyNoRedis = errors.New("private copy details: no Redis")

// errPMCopyTokenTaken is returned when a fresh token already names a record (never expected with 128 random bits).
var errPMCopyTokenTaken = errors.New("private copy details: token already in use")

// pmCopyNewToken returns a fresh token: 16 bytes from crypto/rand in unpadded base64url, 22 characters. It is
// never derived from a user, chat or message ID.
func pmCopyNewToken() (string, error) {
	raw := make([]byte, pmCopyTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// pmCopyDetailsRecordOf is the record of the copy msg a job tried to send for o.
func pmCopyDetailsRecordOf(o eventPMOffer, msg media.GreetingMessage) pmCopyDetailsRecord {
	rec := pmCopyDetailsRecord{UserID: o.UserID, ChatID: o.Chat.Id, Event: o.Event, Text: msg.Text, FileID: msg.FileID, MediaType: msg.MsgType}
	if msg.Keyboard != nil {
		rec.Buttons = msg.Keyboard.InlineKeyboard
	}
	if o.Custom && o.Fallback != "" {
		rec.Fallback = o.Fallback
		if len(o.AppealRow) > 0 {
			rec.AppealRow = append([]gotgbot.InlineKeyboardButton(nil), o.AppealRow...)
		}
	}
	return rec
}

// pmCopyDetailsPut stores rec under a fresh token for pmCopyDetailsTTL (SET NX, bounded by raidRedisTimeout)
// and returns the token. With no Redis, or on any Redis error, it stores nothing and returns the error.
func pmCopyDetailsPut(ctx context.Context, rec pmCopyDetailsRecord) (string, error) {
	client := pmCopyRedisClient()
	if client == nil {
		return "", errPMCopyNoRedis
	}
	token, err := pmCopyNewToken()
	if err != nil {
		return "", err
	}
	value, err := json.Marshal(rec)
	if err != nil {
		return "", err
	}
	putCtx, cancel := context.WithTimeout(ctx, raidRedisTimeout)
	defer cancel()
	stored, err := client.SetNX(putCtx, pmCopyDetailsKey(token), value, pmCopyDetailsTTL).Result()
	if err != nil {
		return "", err
	}
	if !stored {
		return "", errPMCopyTokenTaken
	}
	return token, nil
}

// pmCopyDetailsGet reads the record of token (bounded by raidRedisTimeout). found is false for no record;
// no Redis, a Redis error and a record that does not parse are errors.
func pmCopyDetailsGet(ctx context.Context, token string) (pmCopyDetailsRecord, bool, error) {
	client := pmCopyRedisClient()
	if client == nil {
		return pmCopyDetailsRecord{}, false, errPMCopyNoRedis
	}
	return redisGetJSON[pmCopyDetailsRecord](ctx, client, pmCopyDetailsKey(token))
}

// pmCopyButtonCount is the number of buttons in rows.
func pmCopyButtonCount(rows [][]gotgbot.InlineKeyboardButton) int {
	n := 0
	for _, row := range rows {
		n += len(row)
	}
	return n
}

// pmCopyAddDetails stores the copy msg the job tried to send (the custom copy, or the default after a
// refused custom copy) and adds the Details button to the group message (D-19). runEventPM calls it only
// for a copy that was not delivered, with the group's Details option on and a group message posted. It adds
// nothing, with one Debug line naming the chat and the user, when there is no Redis (no record can exist),
// the bot has no username or one more button would pass Telegram's 100; a record that cannot be stored stops
// it the same way. The edit is one editMessageReplyMarkup of the group message through pmCopyPaced with a
// fresh copy of the rows the message was sent with (the bot's own Unban or Unmute row and the custom
// buttons, in their order) and one more row with the URL button. A failed edit (the message already deleted
// by its timer, a 429, a 5xx) is one Warn line with the chat ID and the Telegram code and changes nothing
// else; the record then expires on its own.
func pmCopyAddDetails(ctx context.Context, o eventPMOffer, msg media.GreetingMessage) {
	if pmCopyRedisClient() == nil {
		log.Debugf("[EventMessage] no Details button for user %d in chat %d: no Redis", o.UserID, o.Chat.Id)
		return
	}
	username := pmCopyBotUsername(o.Bot)
	if username == "" {
		log.Debugf("[EventMessage] no Details button for user %d in chat %d: the bot has no username", o.UserID, o.Chat.Id)
		return
	}
	if pmCopyButtonCount(o.GroupRows)+1 > pmCopyMaxButtons {
		log.Debugf("[EventMessage] no Details button for user %d in chat %d: the group message has no room for a button", o.UserID, o.Chat.Id)
		return
	}
	interactive := pmCopyHasInteractiveRows(o.GroupRows)
	if interactive && pmCopyGroupActionResolved(ctx, o.Chat.Id, o.GroupMsgID) {
		log.Debugf("[EventMessage] no Details button for user %d in chat %d: the group message action was already resolved", o.UserID, o.Chat.Id)
		return
	}
	token, err := pmCopyDetailsPut(ctx, pmCopyDetailsRecordOf(o, msg))
	if err != nil {
		log.Debugf("[EventMessage] no Details button for user %d in chat %d: the record could not be stored", o.UserID, o.Chat.Id)
		return
	}
	rows := make([][]gotgbot.InlineKeyboardButton, 0, len(o.GroupRows)+1)
	for _, row := range o.GroupRows {
		rows = append(rows, append([]gotgbot.InlineKeyboardButton(nil), row...))
	}
	rows = append(rows, []gotgbot.InlineKeyboardButton{{
		Text: o.DetailsLabel,
		Url:  "https://t.me/" + username + "?start=" + pmCopyDetailsPrefix + token,
	}})
	editErr := pmCopyPaced(ctx, func(callCtx context.Context) error {
		if interactive && pmCopyGroupActionResolved(callCtx, o.Chat.Id, o.GroupMsgID) {
			return errPMCopyActionResolved
		}
		_, _, err := o.Bot.EditMessageReplyMarkupWithContext(callCtx, &gotgbot.EditMessageReplyMarkupOpts{
			ChatId:      o.Chat.Id,
			MessageId:   o.GroupMsgID,
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows},
		})
		return err
	})
	if errors.Is(editErr, errPMCopyActionResolved) {
		log.Debugf("[EventMessage] skipped Details button edit for user %d in chat %d: the group message action was already resolved", o.UserID, o.Chat.Id)
		return
	}
	if editErr != nil {
		log.Warnf("[EventMessage] the Details button of chat %d could not be added (code %d)", o.Chat.Id, pmCopyErrCode(editErr))
	}
}

// parsePMCopyDetails parses a /start payload strictly: exactly pmCopyDetailsPrefix and 22 characters of
// A-Za-z0-9_-. It returns the token.
func parsePMCopyDetails(arg string) (string, bool) {
	token, ok := strings.CutPrefix(arg, pmCopyDetailsPrefix)
	if !ok || len(token) != pmCopyTokenLen {
		return "", false
	}
	for i := 0; i < len(token); i++ {
		c := token[i]
		if (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' && c != '-' {
			return "", false
		}
	}
	return token, true
}

// pmCopyDetailsDeepLink opens /start pmcopy_<token> (deep links run in private chats only). The payload is
// attacker-controlled and authorizes nothing: the stored copy is sent to the opener's private chat only when
// the record's user ID equals the opener's ID, and may be opened again until the record expires (a view
// deletes nothing). A malformed payload, an unknown or expired token, a record that does not parse, a Redis
// error and an opener the record does not name all get the one neutral reply eventmsg_pm_details_none in the
// opener's language, which names no group, event or person. The token is never logged.
func pmCopyDetailsDeepLink(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error {
	msg := ctx.EffectiveMessage
	neutral := func() error {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("eventmsg_pm_details_none")
		if _, err := msg.Reply(b, text, formatting.Shtml()); err != nil {
			log.Warnf("[EventMessage] the Details reply could not be sent (code %d)", pmCopyErrCode(err))
		}
		return ext.EndGroups
	}
	token, ok := parsePMCopyDetails(arg)
	if !ok || user == nil {
		return neutral()
	}
	readCtx, cancel := context.WithTimeout(context.Background(), pmCopyCallTimeout)
	defer cancel()
	rec, found, err := pmCopyDetailsGet(readCtx, token)
	if err != nil {
		log.Debugf("[EventMessage] a Details record opened by user %d could not be read", user.Id)
		return neutral()
	}
	if !found || rec.UserID != user.Id {
		return neutral()
	}
	copyMsg := media.GreetingMessage{Text: rec.Text, FileID: rec.FileID, MsgType: rec.MediaType}
	if copyMsg.FileID == "" || copyMsg.MsgType == 0 {
		copyMsg.FileID, copyMsg.MsgType = "", db.TEXT
	}
	if len(rec.Buttons) > 0 {
		copyMsg.Keyboard = &gotgbot.InlineKeyboardMarkup{InlineKeyboard: rec.Buttons}
	}
	if _, err := media.SendGreetingMessage(b, user.Id, copyMsg); err != nil {
		if eventMessageRefused(err) && rec.Fallback != "" {
			fallbackMsg := media.GreetingMessage{Text: rec.Fallback, MsgType: db.TEXT}
			if len(rec.AppealRow) > 0 {
				fallbackMsg.Keyboard = &gotgbot.InlineKeyboardMarkup{
					InlineKeyboard: [][]gotgbot.InlineKeyboardButton{append([]gotgbot.InlineKeyboardButton(nil), rec.AppealRow...)},
				}
			}
			if _, fbErr := media.SendGreetingMessage(b, user.Id, fallbackMsg); fbErr == nil {
				return ext.EndGroups
			}
		}
		log.Warnf("[EventMessage] the Details copy of user %d in chat %d could not be shown (code %d)", user.Id, rec.ChatID, pmCopyErrCode(err))
		return neutral()
	}
	return ext.EndGroups
}

func init() {
	RegisterDeepLinkHandler(pmCopyDetailsPrefix, pmCopyDetailsDeepLink)
}
