package modules

import (
	"context"
	"strconv"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/db/deletion"
	"github.com/uasneppy/Xenobot-72/xenobot/db/eventmsg"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/helpers"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/media"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/ratelimit"
)

// The AI spam notice (Phase 24, EVT-05) is the AI filter's only group message about a deletion. It is
// off by default (eventmsg.OffByDefault: a group with no row reads Silent, so the engine asks nothing,
// spends no Redis command and posts nothing), and a group's owner turns it on from the Messages page.
// When it is on, a deletion posts at most one notice per member every aispamNoticeUserWindow and at
// most aispamNoticePerMinute per group in one fixed minute, through one atomic script that fails closed
// (no Redis or a Redis error posts nothing). Both sends go through the notice's own fleet pacer, whose
// waits derive from the /aispam pool's context, so the shutdown drain sheds them. The notice is a new
// message in the deleted message's topic, never a reply (the message is gone), queued under
// bot:punish_auto. Nothing about it can delay or skip a deletion: it is the last step of a delete verdict
// and runs only after the delete succeeded.

var (
	// aispamNoticePerMinute is the most AI spam notices one group posts in one fixed minute. A variable only
	// so a test can change it; production code never reassigns it.
	aispamNoticePerMinute = 3
	// aispamNoticeNow is the clock of the per-group minute. A variable only so a test can stop it.
	aispamNoticeNow = time.Now
	// aispamNoticeRedisClient is the Redis client of the notice's caps, nil without Redis. A variable only so
	// a test can take it away.
	aispamNoticeRedisClient = raidRedis
)

const (
	// aispamNoticeUserWindow is how long a member's notice keeps their next ones quiet.
	aispamNoticeUserWindow = 5 * time.Minute
	// aispamNoticeMinuteTTL is how long a minute's counter lives: two windows, so an idle group's counters
	// expire on their own.
	aispamNoticeMinuteTTL = 120 * time.Second
	// aispamNoticeCallTimeout bounds one Telegram request of a notice.
	aispamNoticeCallTimeout = 10 * time.Second
)

// aispamNoticeUserKey is the per-member latch of one group's AI spam notice:
// xenobot:aispam:notice:{<chat>}:u:<user>, outside the cache prefix, the chat ID a hash tag.
func aispamNoticeUserKey(chatID, userID int64) string {
	return "xenobot:aispam:notice:{" + strconv.FormatInt(chatID, 10) + "}:u:" + strconv.FormatInt(userID, 10)
}

// aispamNoticeMinuteKey is the counter of one group's AI spam notices in one fixed minute (the Unix time
// divided by 60): xenobot:aispam:notice:{<chat>}:m:<minute>, sharing the member key's hash tag.
func aispamNoticeMinuteKey(chatID, minute int64) string {
	return "xenobot:aispam:notice:{" + strconv.FormatInt(chatID, 10) + "}:m:" + strconv.FormatInt(minute, 10)
}

// aispamNoticeScript sets the member latch (KEYS[1]) with NX for ARGV[1] seconds and answers 0 when it was
// already set. Otherwise it counts the notice in the minute (KEYS[2], expiring after ARGV[3] seconds) and
// answers 1 while the count is at most ARGV[2], else 0. The member latch is spent even when the minute is
// full, so the member's next spam inside the window stays quiet too (the quiet direction).
var aispamNoticeScript = redis.NewScript(`
if not redis.call('SET', KEYS[1], '1', 'NX', 'EX', ARGV[1]) then
	return 0
end
local count = redis.call('INCR', KEYS[2])
redis.call('EXPIRE', KEYS[2], ARGV[3])
if count <= tonumber(ARGV[2]) then
	return 1
end
return 0
`)

// aispamNoticeAllow runs aispamNoticeScript for the member in the group's current minute, bounded by
// raidRedisTimeout, and reports whether the notice may be posted. It fails closed: no Redis client, a
// Redis error or a refusal posts nothing.
func aispamNoticeAllow(ctx context.Context, chatID, userID int64) bool {
	client := aispamNoticeRedisClient()
	if client == nil {
		return false
	}
	capCtx, cancel := context.WithTimeout(ctx, raidRedisTimeout)
	defer cancel()
	allowed, err := aispamNoticeScript.Run(capCtx, client,
		[]string{aispamNoticeUserKey(chatID, userID), aispamNoticeMinuteKey(chatID, aispamNoticeNow().Unix()/60)},
		int64(aispamNoticeUserWindow/time.Second), aispamNoticePerMinute, int64(aispamNoticeMinuteTTL/time.Second)).Int64()
	if err != nil {
		log.WithFields(log.Fields{"chat_id": chatID, "user_id": userID}).Debug("[AISpam] the notice's cap could not be read, no notice is posted")
		return false
	}
	return allowed == 1
}

// aispamNoticePacerOptions are the options of the AI spam notice's pacer: its own Redis keys, so a wave of
// notices never spends another pacer's slots. A slot more than MaxWait away fails at once with no request,
// so a notice never holds an aispam worker long. It is a variable so a test can build a fast pacer from it.
var aispamNoticePacerOptions = ratelimit.TelegramPacerOptions{
	NextKey:  "xenobot:aispam:notice:pace:next",
	BlockKey: "xenobot:aispam:notice:pace:block",
	Interval: 200 * time.Millisecond,
	// MaxRetries 1 is the smallest the pacer accepts (a value of 0 or less becomes 3).
	MaxRetries: 1,
	MaxWait:    2 * time.Second,
}

// aispamNoticePacer paces every send of an AI spam notice across the fleet. Nothing else uses its keys. It
// is a variable so a test can install a fast one.
var aispamNoticePacer = ratelimit.NewTelegramPacer(aispamNoticePacerOptions)

// aispamNoticePaced runs one send of a notice under aispamNoticePacer, bounded by aispamNoticeCallTimeout.
func aispamNoticePaced(ctx context.Context, call func(context.Context) error) error {
	return aispamNoticePacer.Do(ctx, func(callCtx context.Context) error {
		bounded, cancel := context.WithTimeout(callCtx, aispamNoticeCallTimeout)
		defer cancel()
		return call(bounded)
	})
}

// postAISpamNotice posts the AI spam notice of a deletion through the event engine, after the message was
// deleted. The site has no actor and no reply, the deleted message's topic, bot:punish_auto, Automatic (so
// the private copy is capped too) and an admission gate of the two caps; both sends go through the notice's
// pacer on the pool's context. It returns at once when the pool was cancelled by the shutdown, and a send
// error is logged at Debug by chat and user IDs and the Telegram code only.
func postAISpamNotice(task aispamTask) {
	poolCtx := aispamContext()
	if poolCtx.Err() != nil {
		return
	}
	tr := i18n.MustNewTranslator(task.language)
	text, opts := aispamNoticeDefaultReply(tr, eventDefaultInput{Target: task.user})
	if task.threadID != 0 {
		opts.MessageThreadId = task.threadID
	}
	chat := task.chat
	site := eventSite{
		Bot:       task.bot,
		Chat:      &chat,
		Tr:        tr,
		ThreadID:  task.threadID,
		Kind:      deletion.KindBotAuto,
		Automatic: true,
		Admit: func(ctx context.Context) bool {
			return aispamNoticeAllow(ctx, task.chatID, task.userID)
		},
		SendDefault: func(text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
			var sent *gotgbot.Message
			err := aispamNoticePaced(poolCtx, func(callCtx context.Context) error {
				var sendErr error
				sent, sendErr = helpers.SendMessageWithErrorHandlingContext(callCtx, task.bot, task.chatID, text, opts)
				return sendErr
			})
			return sent, err
		},
		SendCustom: func(msg media.GreetingMessage) (*gotgbot.Message, error) {
			var sent *gotgbot.Message
			err := aispamNoticePaced(poolCtx, func(callCtx context.Context) error {
				var sendErr error
				sent, sendErr = media.SendGreetingMessageWithContext(callCtx, task.bot, task.chatID, msg)
				return sendErr
			})
			return sent, err
		},
	}
	reply := eventReply{
		Event:        eventmsg.EventAISpam,
		Target:       task.user,
		Values:       eventValues{Reason: aispamEventReason(tr), Action: eventActionWord(tr, "delete")},
		Default:      text,
		PlainDefault: func() string { return aispamPlainDefaultReply(tr, eventDefaultInput{Target: task.user}) },
		Opts:         opts,
	}
	if err := postEvent(site, reply); err != nil {
		log.WithFields(log.Fields{
			"chat_id": task.chatID,
			"user_id": task.userID,
			"code":    pmCopyErrCode(err),
		}).Debug("[AISpam] the notice was not posted")
	}
}
