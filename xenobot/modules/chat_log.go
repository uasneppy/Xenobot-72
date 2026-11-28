package modules

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/utils/chatprompt"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/error_handling"
)

// The message log: while the owner's Message log switch is on, the bot keeps the newest 1000
// messages of real members of the group in one Redis sorted set (xenobot:chat:{<chat>}:log), so
// /unrollshort, /unrolllong and /judge can brief a stretch of the conversation; the Bot API has no
// method that returns history. The score is the Telegram message ID (IDs grow within a chat, and a
// reply's target ID is the start of the stretch), the member a compact JSON entry. The log is Redis
// only, outside the xenobot:cache: prefix, written by two Lua scripts and deleted when the switch is
// turned off. Deleted messages are not tracked and stay in the ring.

const (
	// chatLogGroup is the handler group of the log watcher, right after the conversation watcher
	// (13) and the only handler of its group. It ends no update.
	chatLogGroup = 14
	// chatLogTTL is the idle expiry of the ring, renewed by every write.
	chatLogTTL = 7 * 24 * time.Hour
	// chatLogTextRunes is how much of a message's text an entry keeps.
	chatLogTextRunes = 400
	// chatLogReadTimeout bounds the read of a stretch (up to a ring's worth of entries).
	chatLogReadTimeout = 2 * time.Second
	// chatLogRecentScanLimit is the least the reverse scan of chatLogRecentThread reads. The scan
	// grows with the number of entries asked for (chatLogRecentScanPerEntry each), because in a forum
	// the topic's own messages are a small part of the newest ones.
	chatLogRecentScanLimit = 50
	// chatLogRecentScanPerEntry is how many newest ring entries are read per entry asked for.
	chatLogRecentScanPerEntry = 8
)

// chatLogRecentMaxAge is how old a message may be and still count as recent room chatter. A quiet
// chat's week-old messages are not "recent", and a model told they are would answer off a stale
// room. An entry with no date (written before dates were kept) is kept. A variable only for tests.
var chatLogRecentMaxAge = 3 * time.Hour

// chatLogMax is how many messages the ring keeps; a variable only so a test can shrink it.
var chatLogMax = 1000

// chatLogEntry is one logged message, stored as compact JSON. It holds the sender's user ID for
// telling two people of one name apart; the ID is never sent to a provider.
type chatLogEntry struct {
	MessageID int64  `json:"m"`
	UserID    int64  `json:"u"`
	Name      string `json:"n"`
	Text      string `json:"t,omitempty"`
	// Media is the English tag of an attached item, such as [photo].
	Media string `json:"k,omitempty"`
	// Thread is the forum topic's thread ID, 0 outside a topic.
	Thread int64 `json:"th,omitempty"`
	Edited bool  `json:"e,omitempty"`
	// At is the original Telegram message Unix timestamp (msg.Date), preserved across edits.
	At int64 `json:"at,omitempty"`
}

// chatLogReplaceLua is the body of both scripts: it replaces any member at the message's own score, adds the entry, keeps the
// newest ARGV[3] and renews the expiry, in one round trip, so a redelivered update replaces the
// entry instead of duplicating it. The edit script (chatLogEditScript) is the same but writes
// nothing when no member sits at the score, so an edit of a message that was never logged, or that already left the ring,
// never inserts it.
const chatLogReplaceLua = `
local old = redis.call('ZRANGEBYSCORE', KEYS[1], ARGV[1], ARGV[1])
%s
for i = 1, #old do redis.call('ZREM', KEYS[1], old[i]) end
redis.call('ZADD', KEYS[1], ARGV[1], ARGV[2])
local n = redis.call('ZCARD', KEYS[1])
local keep = tonumber(ARGV[3])
if n > keep then redis.call('ZREMRANGEBYRANK', KEYS[1], 0, n - keep - 1) end
redis.call('EXPIRE', KEYS[1], ARGV[4])
return 1
`

var (
	chatLogInsertScript = redis.NewScript(chatLogReplace(""))
	chatLogEditScript   = redis.NewScript(chatLogReplace("if #old == 0 then return 0 end"))
)

func chatLogReplace(guard string) string {
	return fmt.Sprintf(chatLogReplaceLua, guard)
}

// chatLogWrite runs a replace script for one entry. It reports whether the entry was written.
func chatLogWrite(client *redis.Client, script *redis.Script, chatID int64, entry chatLogEntry) (bool, error) {
	raw, err := json.Marshal(entry)
	if err != nil {
		return false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), chatRedisTimeout)
	defer cancel()
	written, err := script.Run(ctx, client, []string{chatLogKey(chatID)},
		strconv.FormatInt(entry.MessageID, 10), string(raw), chatLogMax, int(chatLogTTL.Seconds())).Int()
	return written == 1, err
}

// chatLogRecord adds a message to the ring, replacing an entry of the same message ID.
func chatLogRecord(client *redis.Client, chatID int64, entry chatLogEntry) error {
	_, err := chatLogWrite(client, chatLogInsertScript, chatID, entry)
	return err
}

// chatLogEdit replaces the entry of an edited message and marks it edited; an ID that is not in
// the ring is left out. It preserves the original At timestamp when an existing entry carries one.
func chatLogEdit(client *redis.Client, chatID int64, entry chatLogEntry) (bool, error) {
	if existing, ok, err := chatLogEntryAt(context.Background(), client, chatID, entry.MessageID); err != nil {
		return false, err
	} else if ok && existing.At > 0 {
		entry.At = existing.At
	}
	entry.Edited = true
	return chatLogWrite(client, chatLogEditScript, chatID, entry)
}

// chatLogEntryAt reads the single logged entry at score messageID from chatID's log ring.
// Returns (zero, false, nil) when client is nil, messageID <= 0, absent, or malformed.
func chatLogEntryAt(ctx context.Context, client *redis.Client, chatID, messageID int64) (chatLogEntry, bool, error) {
	if client == nil || messageID <= 0 {
		return chatLogEntry{}, false, nil
	}
	rCtx, cancel := context.WithTimeout(ctx, chatLogReadTimeout)
	defer cancel()
	id := strconv.FormatInt(messageID, 10)
	raw, err := client.ZRangeArgs(rCtx, redis.ZRangeArgs{
		Key:     chatLogKey(chatID),
		Start:   id,
		Stop:    id,
		ByScore: true,
	}).Result()
	if err != nil || len(raw) == 0 {
		return chatLogEntry{}, false, err
	}
	var entry chatLogEntry
	if json.Unmarshal([]byte(raw[0]), &entry) != nil {
		return chatLogEntry{}, false, nil
	}
	return entry, true, nil
}

// chatLogHas reports whether the message is in the ring. The members are JSON, so the score is
// asked for with ZCOUNT. A Redis error is returned.
func chatLogHas(ctx context.Context, client *redis.Client, chatID, messageID int64) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, chatRedisTimeout)
	defer cancel()
	id := strconv.FormatInt(messageID, 10)
	n, err := client.ZCount(ctx, chatLogKey(chatID), id, id).Result()
	return n > 0, err
}

// chatLogSince returns the logged messages from fromID on, oldest first. An entry that does not
// parse is left out.
func chatLogSince(ctx context.Context, client *redis.Client, chatID, fromID int64) ([]chatLogEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, chatLogReadTimeout)
	defer cancel()
	raw, err := client.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key: chatLogKey(chatID), Start: strconv.FormatInt(fromID, 10), Stop: "+inf", ByScore: true,
	}).Result()
	if err != nil {
		return nil, err
	}
	entries := make([]chatLogEntry, 0, len(raw))
	for _, item := range raw {
		var entry chatLogEntry
		if json.Unmarshal([]byte(item), &entry) != nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func chatLogZRevRange(ctx context.Context, client *redis.Client, chatID, start, stop int64) ([]string, error) {
	return client.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:   chatLogKey(chatID),
		Start: start,
		Stop:  stop,
		Rev:   true,
	}).Result()
}

// chatLogRecentThread performs a bounded reverse scan of the newest max(chatLogRecentScanLimit,
// limit*chatLogRecentScanPerEntry) entries in chatID's log ring, strictly filtering to
// entry.Thread == threadID && entry.MessageID < beforeMsgID so forum topics never bleed and the
// triggering message is never duplicated, and dropping entries older than chatLogRecentMaxAge. It
// returns up to limit matching entries ordered oldest-first, or (nil, nil) when none match.
func chatLogRecentThread(ctx context.Context, client *redis.Client, chatID, threadID, beforeMsgID int64, limit int) ([]chatLogEntry, error) {
	if client == nil || limit <= 0 {
		return nil, nil
	}
	rCtx, cancel := context.WithTimeout(ctx, chatLogReadTimeout)
	defer cancel()
	scan := max(chatLogRecentScanLimit, limit*chatLogRecentScanPerEntry)
	raw, err := chatLogZRevRange(rCtx, client, chatID, 0, int64(scan)-1)
	if err != nil {
		return nil, err
	}
	oldest := int64(0)
	if chatLogRecentMaxAge > 0 {
		oldest = time.Now().Add(-chatLogRecentMaxAge).Unix()
	}
	entries := make([]chatLogEntry, 0, min(limit, len(raw)))
	for _, item := range raw {
		var entry chatLogEntry
		if json.Unmarshal([]byte(item), &entry) != nil {
			continue
		}
		if entry.Thread != threadID || entry.MessageID >= beforeMsgID {
			continue
		}
		if entry.At > 0 && entry.At < oldest {
			continue
		}
		entries = append(entries, entry)
		if len(entries) == limit {
			break
		}
	}
	if len(entries) == 0 {
		return nil, nil
	}
	slices.Reverse(entries)
	return entries, nil
}

// chatLogRecent scans chatID's log ring from newest to oldest, returning up to limit entries with
// entry.Thread == threadID (and entry.MessageID < beforeMsgID when beforeMsgID > 0), ordered oldest-first.
func chatLogRecent(ctx context.Context, client *redis.Client, chatID, threadID, beforeMsgID int64, limit int) ([]chatLogEntry, error) {
	if client == nil || limit <= 0 {
		return nil, nil
	}
	rCtx, cancel := context.WithTimeout(ctx, chatLogReadTimeout)
	defer cancel()
	raw, err := chatLogZRevRange(rCtx, client, chatID, 0, -1)
	if err != nil {
		return nil, err
	}
	entries := make([]chatLogEntry, 0, min(limit, len(raw)))
	for _, item := range raw {
		var entry chatLogEntry
		if json.Unmarshal([]byte(item), &entry) != nil {
			continue
		}
		if entry.Thread != threadID || (beforeMsgID > 0 && entry.MessageID >= beforeMsgID) {
			continue
		}
		entries = append(entries, entry)
		if len(entries) == limit {
			break
		}
	}
	if len(entries) == 0 {
		return nil, nil
	}
	slices.Reverse(entries)
	return entries, nil
}

// chatLogSinceUser scans chatID's log ring from newest to oldest for entries in threadID
// (strictly below beforeMsgID when beforeMsgID > 0) back to and including the newest entry
// from userID. Returns (nil, nil) if userID has no matching entry in that thread.
func chatLogSinceUser(ctx context.Context, client *redis.Client, chatID, threadID, userID, beforeMsgID int64) ([]chatLogEntry, error) {
	if client == nil || userID <= 0 {
		return nil, nil
	}
	rCtx, cancel := context.WithTimeout(ctx, chatLogReadTimeout)
	defer cancel()
	raw, err := chatLogZRevRange(rCtx, client, chatID, 0, -1)
	if err != nil {
		return nil, err
	}
	var entries []chatLogEntry
	found := false
	for _, item := range raw {
		var entry chatLogEntry
		if json.Unmarshal([]byte(item), &entry) != nil {
			continue
		}
		if entry.Thread != threadID || (beforeMsgID > 0 && entry.MessageID >= beforeMsgID) {
			continue
		}
		entries = append(entries, entry)
		if entry.UserID == userID {
			found = true
			break
		}
	}
	if !found {
		return nil, nil
	}
	slices.Reverse(entries)
	return entries, nil
}

// chatLogSinceTime scans chatID's log ring from newest to oldest, returning entries in threadID
// (strictly below beforeMsgID when beforeMsgID > 0) with entry.At >= cutoffUnix, ordered
// oldest-first. Legacy entries with entry.At <= 0 are skipped so they never match a time window.
func chatLogSinceTime(ctx context.Context, client *redis.Client, chatID, threadID, cutoffUnix, beforeMsgID int64) ([]chatLogEntry, error) {
	if client == nil {
		return nil, nil
	}
	rCtx, cancel := context.WithTimeout(ctx, chatLogReadTimeout)
	defer cancel()
	raw, err := chatLogZRevRange(rCtx, client, chatID, 0, -1)
	if err != nil {
		return nil, err
	}
	var entries []chatLogEntry
	for _, item := range raw {
		var entry chatLogEntry
		if json.Unmarshal([]byte(item), &entry) != nil {
			continue
		}
		if entry.Thread != threadID || (beforeMsgID > 0 && entry.MessageID >= beforeMsgID) {
			continue
		}
		if entry.At <= 0 || entry.At < cutoffUnix {
			continue
		}
		entries = append(entries, entry)
	}
	if len(entries) == 0 {
		return nil, nil
	}
	slices.Reverse(entries)
	return entries, nil
}

// chatLogMediaTag is the English tag of the item a message carries, in square brackets, or "" for
// none. A GIF is checked before the document Telegram also sets for it.
func chatLogMediaTag(msg *gotgbot.Message) string {
	switch {
	case msg.Animation != nil:
		return "[GIF]"
	case msg.Sticker != nil:
		return "[sticker]"
	case msg.Photo != nil:
		return "[photo]"
	case msg.Video != nil:
		return "[video]"
	case msg.VideoNote != nil:
		return "[video note]"
	case msg.Voice != nil:
		return "[voice]"
	case msg.Audio != nil:
		return "[audio]"
	case msg.Document != nil:
		return "[file]"
	case msg.Poll != nil:
		return "[poll]"
	case msg.Contact != nil:
		return "[contact]"
	case msg.Location != nil, msg.Venue != nil:
		return "[location]"
	case msg.Dice != nil:
		return "[dice]"
	case msg.Story != nil:
		return "[story]"
	case msg.Checklist != nil:
		return "[checklist]"
	case msg.Game != nil:
		return "[game]"
	case msg.RichMessage != nil:
		return "[rich message]"
	}
	return ""
}

// chatLogEntryOf builds the entry of a message: the sender's cleaned first name, the cleaned and
// cut text or caption, and the media tag.
func chatLogEntryOf(msg *gotgbot.Message, userID int64) chatLogEntry {
	entry := chatLogEntry{
		MessageID: msg.MessageId,
		UserID:    userID,
		Text:      chatprompt.Clean(chatMessageText(msg), chatLogTextRunes),
		Media:     chatLogMediaTag(msg),
		At:        msg.Date,
	}
	if msg.From != nil {
		entry.Name = chatprompt.Clean(msg.From.FirstName, chatprompt.MaxNameRunes)
	}
	if msg.IsTopicMessage {
		entry.Thread = msg.MessageThreadId
	}
	return entry
}

// chatLogWatcher is the handler of group 14. Its result is a named value preset to
// ext.ContinueGroups, so no path and no recovered panic ends an update.
func (moduleStruct) chatLogWatcher(b *gotgbot.Bot, ctx *ext.Context) (result error) {
	result = ext.ContinueGroups
	defer error_handling.RecoverFromPanic("chatLogWatcher", "Chat")
	chatLogWatch(b, ctx)
	return result
}

// chatLogWatch logs the message, cheapest checks first with no I/O before the settings read: a
// group or supergroup; the server can run conversations at all (ENABLE_CHAT is the kill switch of
// the log); user content that is not a slash text; a real member; something to keep; the switch and
// the stored provider; Redis.
func chatLogWatch(b *gotgbot.Bot, ctx *ext.Context) {
	msg, chat := ctx.EffectiveMessage, ctx.EffectiveChat
	if msg == nil || chat == nil || (chat.Type != "group" && chat.Type != "supergroup") {
		return
	}
	if !chatAnyProviderConfigured() {
		return
	}
	if _, slash, ok := raidFloodItemKey(msg); !ok || slash {
		return
	}
	userID, ok := raidFloodSenderCounts(b, ctx)
	if !ok || ctx.EffectiveSender.User.IsBot {
		return
	}
	entry := chatLogEntryOf(msg, userID)
	if entry.Text == "" && entry.Media == "" {
		return
	}
	lookupCtx, cancel := context.WithTimeout(context.Background(), chatWatchTimeout)
	defer cancel()
	settings, err := chatSettingsLookup(lookupCtx, chat.Id)
	if err != nil || !settings.LogEnabled || !chatProviderConfigured(settings.Provider) {
		return
	}
	client := chatRedisClient()
	if client == nil {
		return
	}
	if ctx.EditedMessage != nil {
		_, err = chatLogEdit(client, chat.Id, entry)
	} else {
		err = chatLogRecord(client, chat.Id, entry)
	}
	if err != nil {
		log.WithField("chat_id", chat.Id).Debugf("[Chat] the message log could not be written: %s", chatRedisErrClass(err))
	}
}

// chatRedisErrClass names a Redis error by its class only, never its text.
func chatRedisErrClass(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "cancelled"
	case errors.Is(err, redis.Nil):
		return "no value"
	}
	return "redis error"
}
