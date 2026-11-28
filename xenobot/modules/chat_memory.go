package modules

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/db/chatconv"
	"github.com/uasneppy/Xenobot-72/xenobot/db/membermem"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/chatprompt"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/error_handling"
)

// Per-member memories (Phase 36). With the group owner's Memories switch on (and conversations on), the
// conversation feature keeps up to 12 short notes about a member, in that group only: after a reply to the
// member's own addressed message, a small writer lane asks the group's provider to update the member's
// notes from that message alone, and the next conversation turn for the same member gets those notes in
// its user text. The notes never reach a brief, a /judge verdict, a reaction or a toy, and are deleted by
// the member's /forgetme and by the owner turning the switch (or conversations) off.
//
// The writer is its own lane, joined to StopChat like the others (chatJobs, chatDraining, chatContext):
// a note cut off at shutdown is simply not written, so there is nothing to resume and no drain is added.
// It makes no Telegram call of any kind, so it can never reply to a command a wrapper has deleted
// (TestCommandRepliesStayInsideTheHandler), and a failure of it is never a provider failure of the group.

const (
	// chatMemoryQueueCapacity is how many writer jobs wait at most; a full queue sheds.
	chatMemoryQueueCapacity = 16
	// chatMemoryWorkerCount is how many notebooks are updated at once on a replica.
	chatMemoryWorkerCount = 1
	// chatMemoryJobTimeout bounds one writer job: one provider call with its retry.
	chatMemoryJobTimeout = 50 * time.Second
	// chatMemoryMinRunes is the shortest message worth a writer call: a short line holds no stable fact.
	chatMemoryMinRunes = 25
	// chatMemoryReadTimeout bounds the read of a member's notes before a reply.
	chatMemoryReadTimeout = 2 * time.Second
	// chatMemoryBudgetFamily is the budget family of the writer (xenobot:chat:mem:b:...).
	chatMemoryBudgetFamily = "mem:b"
)

var (
	// chatMemoryCooldown is the least time between two writer calls for one member in one group. A
	// variable only so a test can shorten it.
	chatMemoryCooldown = 15 * time.Minute
	// chatMemoryUserHourlyBudget, chatMemoryChatHourlyBudget and chatMemoryGlobalHourlyBudget are the
	// writer calls one member may cost in one group, one group may cost and the whole bot may spend in one
	// UTC hour. Package variables only so a test can shrink them.
	chatMemoryUserHourlyBudget   int64 = 4
	chatMemoryChatHourlyBudget   int64 = 20
	chatMemoryGlobalHourlyBudget int64 = 200

	chatMemoryQueue chan chatMemoryTask
	// chatMemoryProcess runs one writer job; a variable only so a test can install a gated one, and so a
	// goroutine reaches the job through a variable and not a function that could reply.
	chatMemoryProcess = chatMemoryProcessTask
	// chatMemorySettingsFresh reads the group's switch from the database right before a write, so a switch
	// turned off a moment ago is honoured on every replica. A variable only so a test can swap it.
	chatMemorySettingsFresh = chatconv.GetFresh

	chatMemoryWrittenCount   atomic.Int64
	chatMemoryUnchangedCount atomic.Int64
	chatMemoryFailedCount    atomic.Int64
	chatMemoryShedCount      atomic.Int64
	chatMemoryCappedCount    atomic.Int64
)

// chatMemoryTask is one writer job: the member and group by ID, the provider to ask, the member's own
// message and the bot's reply. It holds no name and no username, and no Telegram client.
type chatMemoryTask struct {
	chatID, userID int64
	provider       string
	message, reply string
}

// chatMemoryStatsSection is the memory part of the developer /stats: numbers only.
func chatMemoryStatsSection() string {
	return fmt.Sprintf(
		"\n    <b>Memories:</b> written %d, unchanged %d, failed %d, shed %d, capped %d",
		chatMemoryWrittenCount.Load(), chatMemoryUnchangedCount.Load(), chatMemoryFailedCount.Load(),
		chatMemoryShedCount.Load(), chatMemoryCappedCount.Load(),
	)
}

func chatMemoryEnsureWorkers() {
	chatMemoryQueue = make(chan chatMemoryTask, chatMemoryQueueCapacity)
	for range chatMemoryWorkerCount {
		go chatMemoryWorker()
	}
}

func chatMemoryWorker() {
	defer error_handling.RecoverFromPanic("chatMemoryWorker", "Chat")
	for task := range chatMemoryQueue {
		func() {
			defer chatJobs.finish()
			defer error_handling.RecoverFromPanic("chatMemoryProcess", "Chat")
			if chatContext().Err() != nil {
				chatMemoryShedCount.Add(1)
				return
			}
			chatMemoryProcess(task)
		}()
	}
}

// chatMemoryEnqueue hands a writer job to the lane, shedding it when the process is draining or the
// queue is full. A shed job loses one update of a notebook and nothing else.
func chatMemoryEnqueue(task chatMemoryTask) bool {
	chatEnsureWorkers()
	chatJobs.start()
	if chatDraining.Load() {
		chatJobs.finish()
		chatMemoryShedCount.Add(1)
		return false
	}
	select {
	case chatMemoryQueue <- task:
		return true
	default:
		chatJobs.finish()
		chatMemoryShedCount.Add(1)
		log.WithFields(log.Fields{"chat_id": task.chatID, "user_id": task.userID}).Debug("[Chat] memory queue full, update shed")
		return false
	}
}

// chatMemoryCooldownKey is one member's writer cooldown in one group.
func chatMemoryCooldownKey(chatID, userID int64) string {
	return chatKeyPrefix + "{" + strconv.FormatInt(chatID, 10) + "}:mem:cd:" + strconv.FormatInt(userID, 10)
}

// chatMemoriesOn reports whether the group's owner has turned memories on for a group whose
// conversations are on and that is not a Staff Group. An unreadable setting is off.
func chatMemoriesOn(ctx context.Context, chatID int64) bool {
	conv, err := chatSettingsLookup(ctx, chatID)
	if err != nil || !conv.Enabled || !conv.MemoriesEnabled {
		return false
	}
	return !presenceIsStaffGroup(chatID)
}

// chatMemoryNotesFor reads the speaker's notes for a reply: fresh, this group only, none on any error.
func chatMemoryNotesFor(ctx context.Context, chatID, userID int64) []string {
	readCtx, cancel := context.WithTimeout(ctx, chatMemoryReadTimeout)
	defer cancel()
	notes, err := membermem.Get(readCtx, chatID, userID)
	if err != nil {
		log.WithFields(log.Fields{"chat_id": chatID, "user_id": userID}).Debug("[Chat] the member notes could not be read")
		return nil
	}
	return notes
}

// chatMemoryOfferHook is how chatProcessJob offers a finished exchange to the writer lane. It is set in
// init because chatProcessJob is the value of chatProcess, which the lane's own enqueue path (through
// chatEnsureWorkers and chatWorker) reads: a direct call would be an initialization cycle.
var chatMemoryOfferHook func(job chatJob, reply string)

func init() { chatMemoryOfferHook = chatMemoryOffer }

// chatMemoryOffer queues the update of the speaker's notebook after a reply was sent and recorded. A
// message with no text of its own, or one too short to hold a stable fact, is not offered.
func chatMemoryOffer(job chatJob, reply string) {
	if !job.hasText || utf8.RuneCountInString(strings.TrimSpace(job.turn.Text)) < chatMemoryMinRunes {
		return
	}
	chatMemoryEnqueue(chatMemoryTask{
		chatID: job.chatID, userID: job.userID, provider: job.provider, message: job.turn.Text, reply: reply,
	})
}

var (
	chatMemoryDigits = regexp.MustCompile(`\d`)
)

// chatMemoryAcceptNotes is the module's last filter over the writer's list, after the strict parse:
// a note that holds a link, an @handle, an address of an account or a long number (a phone number, an
// ID, a card) is dropped, whatever the model made of the instruction not to keep them. It never edits a
// note: a note that needs editing is not kept.
func chatMemoryAcceptNotes(notes []string) []string {
	kept := make([]string, 0, len(notes))
	for _, note := range notes {
		if chatSanitizeText(note) != note || strings.ContainsAny(note, "@<>") {
			continue
		}
		if len(chatMemoryDigits.FindAllString(note, -1)) >= 7 {
			continue
		}
		kept = append(kept, note)
	}
	return membermem.Normalize(kept)
}

// chatMemoryProcessTask updates one member's notebook: the cheap guards first (no provider call before
// all of them hold), then one provider call through the shared helper, then one write. Every refusal and
// every failure leaves the stored notes as they were; none of them is a provider failure of the group.
func chatMemoryProcessTask(task chatMemoryTask) {
	root := chatContext()
	ctx, cancel := context.WithTimeout(root, chatMemoryJobTimeout)
	defer cancel()
	fields := log.Fields{"chat_id": task.chatID, "user_id": task.userID}

	client := chatRedisClient()
	if client == nil || !chatProviderConfigured(task.provider) || !chatMemoriesOn(ctx, task.chatID) {
		chatMemoryShedCount.Add(1)
		return
	}
	if chatBreaker(task.provider).isPaused(task.chatID, raidNow()) {
		chatMemoryShedCount.Add(1)
		return
	}
	if !chatClaimCooldown(ctx, client, chatMemoryCooldownKey(task.chatID, task.userID), chatMemoryCooldown) {
		chatMemoryShedCount.Add(1)
		return
	}
	switch chatReserveWith(client, task.chatID, task.userID, chatMemoryBudgetFamily,
		chatMemoryUserHourlyBudget, chatMemoryChatHourlyBudget, chatMemoryGlobalHourlyBudget) {
	case chatBudgetCapped:
		chatMemoryCappedCount.Add(1)
		return
	case chatBudgetFault:
		chatMemoryShedCount.Add(1)
		return
	}

	existing, err := membermem.Get(ctx, task.chatID, task.userID)
	if err != nil {
		// Unknown notes are never overwritten: a write from a list the writer could not see would lose them.
		chatMemoryShedCount.Add(1)
		log.WithFields(fields).Debug("[Chat] the member notes could not be read for an update")
		return
	}
	date := time.Now().In(chatClock(ctx, task.chatID)).Format("2006-01-02")
	request := chatRequest{
		Static:    chatprompt.MemoryStatic(),
		Dynamic:   "Today's date: " + date + ".",
		User:      chatprompt.MemoryUser(existing, task.message, task.reply),
		MaxTokens: chatprompt.MemoryMaxTokens,
	}
	answer, err := chatConverse(ctx, root, chatCall{Lane: "memory", ChatID: task.chatID}, task.provider, request)
	if err != nil {
		if root.Err() != nil || ctx.Err() != nil {
			chatMemoryShedCount.Add(1)
			return
		}
		chatMemoryFailedCount.Add(1)
		log.WithFields(fields).Debugf("[Chat] the memory update failed: %s", chatErrorClass(err))
		return
	}
	if answer.Outcome == chatRefused {
		chatMemoryUnchangedCount.Add(1)
		return
	}
	parsed, ok := chatprompt.ParseMemoryJSON(answer.Text)
	if !ok {
		chatMemoryFailedCount.Add(1)
		log.WithFields(fields).Debug("[Chat] the memory update was not a notes object")
		return
	}
	notes := chatMemoryAcceptNotes(parsed)
	if slices.Equal(notes, existing) {
		chatMemoryUnchangedCount.Add(1)
		return
	}
	// The owner may have turned the feature off while the provider was thinking: read the switch from the
	// database now, and write nothing when it is off.
	if fresh, err := chatMemorySettingsFresh(ctx, task.chatID); err != nil || !fresh.Enabled || !fresh.MemoriesEnabled {
		chatMemoryShedCount.Add(1)
		return
	}
	if _, err := membermem.Replace(ctx, task.chatID, task.userID, notes); err != nil {
		chatMemoryFailedCount.Add(1)
		log.WithFields(fields).Warn("[Chat] the member notes could not be written")
		return
	}
	chatMemoryWrittenCount.Add(1)
}
