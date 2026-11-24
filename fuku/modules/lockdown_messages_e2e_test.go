//go:build testtools

package modules

import (
	"fmt"
	"html"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
)

// The lockdown messages end to end (plan 25-08; EVT-06, EVT-07, EVT-08, EVT-10, the five success criteria of Phase 25).
// Everything is real but Telegram: the owner works the Lockdown family of the private Messages page through the deep
// link and the buttons the bot drew, every part is a typed private message, an administrator's /lockdown and
// /unlockdown run through the real dispatcher, fifteen chat_member joins trip the join surge, thirty items from new
// members trip the flood, five spam verdicts counted through the verdict counter trip the AI burst, and the lifts come
// from the command, the alert's button in the Staff Group, a private menu and a menu opened in the group. Only the Bot
// API is the hand-written fake (wrapped by Phase 21's messagesE2EClient, so a custom photo is answered as a message of
// the group). The sites' logs and privacy are swept by TestLockdownMessageLogsCarryIDsOnly and
// TestLockdownMessagePrivacy (plan 25-07) and every non-event surface by TestLockdownMessagesLeaveOtherSurfacesAlone
// (plan 25-06), not again here beyond the untouched surfaces case.

// lockdownMessagesE2E is Phase 21's messagesE2E (the settings environment with the Lockdown module, a live creator who
// works the private menu and the media-answering client) with the group linked to a Staff Group (the alert's
// destination, its burst rows removed when the test ends), a log channel with every category on, the chat's raid
// settings removed, every deletion rule of the chat at its shortest delay (so a lockdown message that entered the
// queue would leave a row) and the bot an administrator who can restrict, delete and invite.
type lockdownMessagesE2E struct {
	*messagesE2E
	staffChat  gotgbot.Chat
	logChannel int64
}

// newLockdownMessagesE2E builds one environment for one subtest; real installs the real locale files, for a subtest
// that compares a request with the goldens recorded in Ukrainian.
func newLockdownMessagesE2E(t *testing.T, real bool) *lockdownMessagesE2E {
	t.Helper()
	e := newMessagesE2E(t, real)
	raidCleanup(t, e.chat.Id)
	eventMsgCleanup(t, e.chat.Id)
	deletionShortestRules(t, e.chat.Id)
	e.fake.setBotMember(e.chat.Id, lockdownBotAdminJSON(true, true, true))
	staffChat := lockdownBurstEnv(e.lockdownEnv)
	logChannel := uniqueModuleChatID()
	setStaffLogChannel(t, e.chat.Id, logChannel)
	return &lockdownMessagesE2E{messagesE2E: e, staffChat: staffChat, logChannel: logChannel}
}

// openLockdownEvent walks Main, Messages, Lockdown and the event's cell of user's private menu, each a press of the
// drawn button.
func (e *lockdownMessagesE2E) openLockdownEvent(user gotgbot.User, code string) *messagesE2EMenu {
	e.t.Helper()
	m := e.openMain(user)
	m.tap("a", settingsActOpen, "p", settingsPageMessages)
	m.tap("a", settingsActMsgFamily, "t", "l")
	m.tap("a", settingsActMsgEvent, settingsMsgEventField, code)
	return m
}

// setLockdownPart presses Set of part on the lockdown event's screen as the owner and sends msg as the typed reply.
func (e *lockdownMessagesE2E) setLockdownPart(code, part string, msg gotgbot.Message) {
	e.t.Helper()
	e.openLockdownEvent(e.owner, code).tap("a", settingsActMsgSet, "t", part)
	if !e.redis.Exists(settingsInputKey(e.owner.Id)) {
		e.t.Fatalf("Set of %s/%s left no wait", code, part)
	}
	sendPrivateMessage(e.settingsEnv, e.owner, msg)
	if e.redis.Exists(settingsInputKey(e.owner.Id)) {
		e.t.Fatalf("the typed %s of %s did not finish the wait", part, code)
	}
}

// setLockdownText sets the event's text from a typed message whose first word is bold.
func (e *lockdownMessagesE2E) setLockdownText(code, text string) {
	e.t.Helper()
	first := strings.IndexByte(text, ' ')
	if first < 0 {
		first = len(text)
	}
	e.setLockdownPart(code, settingsEditPartText, gotgbot.Message{
		Text: text, Entities: []gotgbot.MessageEntity{{Type: "bold", Offset: 0, Length: int64(first)}},
	})
}

// silence presses the Silent toggle of the lockdown event's screen as the owner.
func (e *lockdownMessagesE2E) silence(code, event string) {
	e.t.Helper()
	e.openLockdownEvent(e.owner, code).tap("a", settingsActMsgSilent, settingsMsgEventField, code, "v", "1")
	if silent, by, ok := settingsMsgSilentStored(e.t, e.chat.Id, event); !ok || !silent || by != e.owner.Id {
		e.t.Fatalf("after the Silent press the %s row reads Silent %v by %d (row %v), want Silent by the owner", event, silent, by, ok)
	}
}

// lockCommand runs text as a message of from in the group through env's real dispatcher (the base one or a replica's)
// and returns the command's message ID.
func (e *lockdownMessagesE2E) lockCommand(env *settingsEnv, from gotgbot.User, text string) int64 {
	e.t.Helper()
	id := env.updateID()
	msg := &gotgbot.Message{MessageId: id, Date: 1, Chat: e.chat, From: &from, Text: text}
	if err := env.dispatcher.ProcessUpdate(env.bot, &gotgbot.Update{UpdateId: id, Message: msg}, nil); err != nil {
		e.t.Fatalf("ProcessUpdate(%d) error = %v", id, err)
	}
	return id
}

// wantLockdownReply fails unless call is one sendMessage whose text is want and which replies to message replyTo (or
// is a new message with no reply and no topic when replyTo is 0).
func (e *lockdownMessagesE2E) wantLockdownReply(call moduleBotCall, want string, replyTo int64) {
	e.t.Helper()
	params := rulesCallParams(e.t, call)
	if call.Method != "sendMessage" || fmt.Sprint(params["text"]) != want {
		e.t.Errorf("the group got %s %q, want the message %q", call.Method, params["text"], want)
	}
	reply, hasReply := params["reply_parameters"].(map[string]any)
	switch {
	case replyTo == 0 && (hasReply || params["message_thread_id"] != nil):
		e.t.Errorf("the message carries reply_parameters %v or a topic, want a new message", params["reply_parameters"])
	case replyTo != 0 && (!hasReply || reply["message_id"] != float64(replyTo)):
		e.t.Errorf("reply_parameters = %v, want a reply to the command %d", params["reply_parameters"], replyTo)
	}
}

// onlySend fails unless the group received exactly one message since mark, and returns it.
func (e *lockdownMessagesE2E) onlySend(mark map[string]int, what string) moduleBotCall {
	e.t.Helper()
	sends := e.groupSends(e.settingsEnv, mark)
	if len(sends) != 1 {
		e.t.Fatalf("%s: the group got %+v, want exactly one message", what, sends)
	}
	return sends[0]
}

// wantNoPrivateSend fails for every request the bot made to a private chat (a positive chat ID) after the first mark
// requests: no send, edit or copy, whatever its method. The requests before the mark are the owner's private menu.
func (e *lockdownMessagesE2E) wantNoPrivateSend(mark int, what string) {
	e.t.Helper()
	calls := lockdownSurfaceCalls(e.fake.moduleBotClient)
	for _, call := range calls[mark:] {
		for _, key := range []string{"chat_id", "from_chat_id"} {
			if _, ok := call.Params[key]; ok && staffParamInt(call.Params, key) > 0 {
				e.t.Errorf("%s: %s went to the private chat %v, want nothing in private", what, call.Method, call.Params[key])
			}
		}
	}
}

// wantNoneCarry fails for every request of calls that carries any of the words.
func lockdownE2EWantNoneCarry(t *testing.T, calls []moduleBotCall, what string, words ...string) {
	t.Helper()
	for _, call := range calls {
		request := lockdownSurfaceRequest(call)
		for _, word := range words {
			if strings.Contains(request, word) {
				t.Errorf("%s: %s carries %q: %s", what, call.Method, word, request)
			}
		}
	}
}

// lockdownE2EGolden puts the command's message ID in place of the one the golden was recorded with.
func lockdownE2EGolden(golden string, recorded, msgID int64) string {
	return strings.Replace(golden, `"message_id":`+strconv.FormatInt(recorded, 10)+`,`,
		`"message_id":`+strconv.FormatInt(msgID, 10)+`,`, 1)
}

// wantGoldenLast fails unless the last message the group received, with its IDs replaced by tokens, is golden with the
// command's message ID put in for recorded.
func (e *lockdownMessagesE2E) wantGoldenLast(what, golden string, recorded, msgID int64) {
	e.t.Helper()
	sent := e.fake.sentTo(e.chat.Id)
	ids := map[int64]string{e.chat.Id: "<chat>", e.admin.Id: "<admin>"}
	lockdownGoldenWant(e.t, what, lockdownGoldenRequest(e.t, e.lockdownEnv, e.chat.Id, len(sent)-1), ids,
		lockdownE2EGolden(golden, recorded, msgID))
}

// wantAutoNotice fails unless the trigger kind started the env's lockdown with count and window, and the group got the
// one notice want, a new message after the lock; it returns the lockdown row.
func (e *lockdownMessagesE2E) wantAutoNotice(mark map[string]int, calls int, kind string, count, window int, want string) *models.ChatLockdown {
	e.t.Helper()
	row := lockdownEventWantStarted(e.t, e.lockdownEnv, e.bot.Id)
	if row.TriggerKind != kind || row.TriggerCount != count || row.TriggerWindow != window {
		e.t.Fatalf("lockdown = %s %d in %d s, want %s %d in %d s", row.TriggerKind, row.TriggerCount, row.TriggerWindow, kind, count, window)
	}
	methods := methodsSince(e.settingsEnv, calls)
	if lock, send := eventMsgIndex(methods, "setChatPermissions"), eventMsgIndex(methods, "sendMessage"); lock < 0 || send < lock {
		e.t.Fatalf("methods = %v, want the lock before the notice", methods)
	}
	e.wantLockdownReply(e.onlySend(mark, "the automatic notice"), want, 0)
	lockdownGoldenNoQueue(e.t, e.chat.Id)
	return row
}

// approvedNote is the line break and the one technical note a custom start text ends with while the bot holds every
// right, in the marker locale.
var lockdownE2EApprovedNote = "\n" + staffMarker("lockdown_note_approved_muted")

func TestLockdownMessagesEndToEnd(t *testing.T) {
	t.Run("owner sets the manual start", func(t *testing.T) {
		e := newLockdownMessagesE2E(t, false)
		e.setLockdownText("ld", "{admin} locked {chatname}: {reason}")
		e.setLockdownPart("ld", settingsEditPartMedia, settingsMediaItemID("photo", "lock-photo"))
		e.setLockdownPart("ld", settingsEditPartButtons, gotgbot.Message{Text: "[Rules](buttonurl://https://example.com/rules)"})
		row, ok := settingsMsgStored(t, e.chat.Id, eventmsg.EventLockdown)
		if !ok || row.Text != "<b>{admin}</b> locked {chatname}: {reason}" || row.FileID != "lock-photo" ||
			row.MediaType != db.PHOTO || len(row.Buttons) != 1 || row.Buttons[0].Url != "https://example.com/rules" {
			t.Fatalf("the stored lockdown message is %+v, want the typed text, the photo and the Rules button", row)
		}

		// The hub's manual start line reads Custom, every other lockdown event Default.
		m := e.openMain(e.owner)
		m.tap("a", settingsActOpen, "p", settingsPageMessages)
		settingsMsgWantLockdownStates(t, m.text, map[string]string{eventmsg.EventLockdown: "settings_msg_state_custom"})

		mark, calls := settingsEditMarkSends(e.settingsEnv, e.chat.Id), len(eventMsgMethods(e.fake.moduleBotClient))
		id := e.lockCommand(e.settingsEnv, e.admin, "/lockdown raid & co")
		lockdownEventWantStarted(t, e.lockdownEnv, e.admin.Id)
		methods := methodsSince(e.settingsEnv, calls)
		if lock, photo := eventMsgIndex(methods, "setChatPermissions"), eventMsgIndex(methods, "sendPhoto"); lock < 0 || photo < lock {
			t.Fatalf("methods = %v, want the group locked before the custom sendPhoto", methods)
		}
		sends := e.groupSends(e.settingsEnv, mark)
		if len(sends) != 1 || sends[0].Method != "sendPhoto" {
			t.Fatalf("the group got %+v, want one sendPhoto", sends)
		}
		params := rulesCallParams(t, sends[0])
		caption := "<b>Ad&lt;b&gt;min</b> locked " + html.EscapeString(e.chat.Title) + ": raid &amp; co" + lockdownE2EApprovedNote
		if params["caption"] != caption || params["photo"] != "lock-photo" || params["parse_mode"] != "HTML" {
			t.Errorf("sendPhoto caption %q photo %v parse mode %v, want %q, lock-photo and HTML", params["caption"], params["photo"], params["parse_mode"], caption)
		}
		if reply, _ := params["reply_parameters"].(map[string]any); reply["message_id"] != float64(id) {
			t.Errorf("reply_parameters = %v, want the command %d", params["reply_parameters"], id)
		}
		if rows := settingsMsgSentKeyboard(t, params); len(rows) != 1 || len(rows[0]) != 1 ||
			rows[0][0].Text != "Rules" || rows[0][0].Url != "https://example.com/rules" {
			t.Errorf("the keyboard = %+v, want the Rules button alone", rows)
		}
		lockdownGoldenNoQueue(t, e.chat.Id)

		// The alert reaches the Staff Group and the log channel with none of the custom message.
		e.cycle()
		if staffGot, logGot := len(e.fake.sentTo(e.staffChat.Id)), len(e.fake.sentTo(e.logChannel)); staffGot != 1 || logGot != 1 {
			t.Fatalf("the alert reached the Staff Group %d and the log channel %d times, want once each", staffGot, logGot)
		}
		all := lockdownSurfaceCalls(e.fake.moduleBotClient)
		for _, chatID := range []int64{e.staffChat.Id, e.logChannel} {
			lockdownE2EWantNoneCarry(t, lockdownCallsTo(all, chatID), "the alert", "lock-photo", "example.com/rules", " locked ")
		}
	})

	t.Run("join surge", func(t *testing.T) {
		e := newLockdownMessagesE2E(t, false)
		e.setLockdownText("lj", "{count} joined in {window} s: {reason}")

		mark, calls := settingsEditMarkSends(e.settingsEnv, e.chat.Id), len(eventMsgMethods(e.fake.moduleBotClient))
		e.selfJoins(14)
		if row := e.activeLockdown(); row != nil {
			t.Fatalf("a lockdown exists after 14 joins: %+v", row)
		}
		fifteenth := e.newJoiner("Fifteenth")
		e.join(fifteenth, fifteenth, "")
		raidPosts.wait()

		sentence := lockdownTriggerSentence(staffChatTranslator(e.chat.Id), models.LockdownTriggerJoinSurge, 15, 60, 0)
		want := "<b>15</b> joined in 60 s: " + html.EscapeString(sentence) + lockdownE2EApprovedNote
		e.wantAutoNotice(mark, calls, models.LockdownTriggerJoinSurge, 15, 60, want)
		if joiner := e.joinerRow(fifteenth.Id); joiner.State != models.JoinerStatePending {
			t.Errorf("the joiner who tipped the threshold is %q, want pending (recorded by the guard as before)", joiner.State)
		}
	})

	t.Run("flood", func(t *testing.T) {
		e := newLockdownMessagesE2E(t, false)
		e.setLockdownText("lf", "{count} in {window} s, {percent}%")

		recent := e.newMembers(4, 2*time.Hour)
		mark, calls := settingsEditMarkSends(e.settingsEnv, e.chat.Id), len(eventMsgMethods(e.fake.moduleBotClient))
		e.sendItems(recent, 30)
		raidPosts.wait()

		row := e.wantAutoNotice(mark, calls, models.LockdownTriggerFlood, 30, 30, "<b>30</b> in 30 s, 100%"+lockdownE2EApprovedNote)
		if row.TriggerPercent != 100 {
			t.Errorf("the flood's percent = %d, want 100", row.TriggerPercent)
		}
	})

	t.Run("ai burst", func(t *testing.T) {
		e := newLockdownMessagesE2E(t, false)
		e.setLockdownText("la", "{count} spam in {window} s: {reason}")

		accounts := e.newMembers(3, time.Minute)
		mark, calls := settingsEditMarkSends(e.settingsEnv, e.chat.Id), len(eventMsgMethods(e.fake.moduleBotClient))
		for _, user := range []gotgbot.User{accounts[0], accounts[0], accounts[1], accounts[1], accounts[2]} {
			raidAIRecordVerdict(e.bot, e.chat, user.Id, fmt.Sprintf("m:%d", e.updateID()))
		}
		raidAIFires.wait()
		raidPosts.wait()

		sentence := lockdownTriggerSentence(staffChatTranslator(e.chat.Id), models.LockdownTriggerAIBurst, 5, 120, 0)
		want := "<b>5</b> spam in 120 s: " + html.EscapeString(sentence) + lockdownE2EApprovedNote
		e.wantAutoNotice(mark, calls, models.LockdownTriggerAIBurst, 5, 120, want)
	})

	t.Run("lift doors", func(t *testing.T) {
		e := newLockdownMessagesE2E(t, false)
		e.setLockdownText("lu", "{admin} opened {chatname}")
		opened := func(name string) string {
			return "<b>" + html.EscapeString(name) + "</b> opened " + html.EscapeString(e.chat.Title)
		}

		// /unlockdown: the custom text as a reply to the command.
		row := e.lockAsAdmin("raid one")
		mark := settingsEditMarkSends(e.settingsEnv, e.chat.Id)
		id := e.lockCommand(e.settingsEnv, e.admin, "/unlockdown")
		lockdownLiftWantLifted(t, e.lockdownEnv, row, e.admin.Id)
		e.wantLockdownReply(e.onlySend(mark, "/unlockdown"), opened(e.admin.FirstName), id)

		// The alert's Lift and its Confirm, pressed in the Staff Group: a new message naming the presser.
		row = e.lockAsAdmin("raid two")
		mark = settingsEditMarkSends(e.settingsEnv, e.chat.Id)
		lockdownSafeAlertLift(t, e.lockdownEnv, e.staffChat)
		lockdownLiftWantLifted(t, e.lockdownEnv, row, e.admin.Id)
		e.wantLockdownReply(e.onlySend(mark, "the alert's lift"), opened(e.admin.FirstName), 0)

		// A private menu's Lift Yes, by the owner.
		row = e.lockAsAdmin("raid three")
		m := e.openMain(e.owner)
		m.tap("a", settingsActOpen, "p", settingsPageLockdown)
		m.tap("a", "ls")
		mark = settingsEditMarkSends(e.settingsEnv, e.chat.Id)
		m.tap("a", "ly")
		lockdownLiftWantLifted(t, e.lockdownEnv, row, e.owner.Id)
		e.wantLockdownReply(e.onlySend(mark, "the private menu's lift"), opened(e.owner.FirstName), 0)

		// A group menu's Lift Yes: the menu is redrawn as before and the custom text follows.
		row = e.lockAsAdmin("raid four")
		msgID, before := lockdownLiftGroupMenuYes(t, e.settingsEnv)
		lockdownLiftWantLifted(t, e.lockdownEnv, row, e.admin.Id)
		if text, _ := e.lastEdit(e.chat.Id, msgID); !strings.Contains(text, staffMarker("lockdown_lifted")) {
			t.Errorf("the group menu after the Yes = %q, want the lift result as its notice", text)
		}
		sent := e.fake.sentTo(e.chat.Id)
		if len(sent) != before+1 {
			t.Fatalf("the group received %d messages after the group menu's Yes, want the one notice", len(sent)-before)
		}
		e.wantLockdownReply(moduleBotCall{Method: "sendMessage", Params: sent[before].Params}, opened(e.admin.FirstName), 0)
		lockdownGoldenNoQueue(t, e.chat.Id)
	})

	t.Run("nothing stored", func(t *testing.T) {
		e := newLockdownMessagesE2E(t, true)
		id := e.lockCommand(e.settingsEnv, e.admin, "/lockdown")
		lockdownEventWantStarted(t, e.lockdownEnv, e.admin.Id)
		e.wantGoldenLast("the /lockdown reply", goldenLockdownStart, 101, id)
		row := e.activeLockdown()
		id = e.lockCommand(e.settingsEnv, e.admin, "/unlockdown")
		lockdownLiftWantLifted(t, e.lockdownEnv, row, e.admin.Id)
		e.wantGoldenLast("the /unlockdown reply", goldenLockdownLiftCommand, 102, id)
		lockdownGoldenWantSends(t, e.lockdownEnv, e.chat.Id, 2)

		// A lift starts the triggers' quiet period, so the join surge has an environment of its own.
		s := newLockdownMessagesE2E(t, true)
		s.selfJoins(15)
		raidPosts.wait()
		lockdownEventWantStarted(t, s.lockdownEnv, s.bot.Id)
		lockdownGoldenWantSends(t, s.lockdownEnv, s.chat.Id, 1)
		lockdownGoldenWant(t, "the join surge notice", lockdownGoldenRequest(t, s.lockdownEnv, s.chat.Id, 0),
			map[int64]string{s.chat.Id: "<chat>"}, goldenLockdownAutoJoinSurge)
		lockdownGoldenNoQueue(t, e.chat.Id)
		lockdownGoldenNoQueue(t, s.chat.Id)
	})

	t.Run("refused", func(t *testing.T) {
		e := newLockdownMessagesE2E(t, true)
		e.setLockdownPart("ld", settingsEditPartText, gotgbot.Message{Text: "Locked {chatname}"})
		e.setLockdownPart("ld", settingsEditPartMedia, settingsMediaItemID("photo", "lock-photo"))
		e.fake.script("sendPhoto", e.chat.Id, staffFakeError(400, "Bad Request: wrong file identifier/HTTP URL specified"))

		mark, calls := settingsEditMarkSends(e.settingsEnv, e.chat.Id), len(eventMsgMethods(e.fake.moduleBotClient))
		id := e.lockCommand(e.settingsEnv, e.admin, "/lockdown")
		lockdownEventWantStarted(t, e.lockdownEnv, e.admin.Id)
		methods := methodsSince(e.settingsEnv, calls)
		lock, photo, text := eventMsgIndex(methods, "setChatPermissions"), eventMsgIndex(methods, "sendPhoto"), eventMsgIndex(methods, "sendMessage")
		if lock < 0 || photo < lock || text < photo {
			t.Fatalf("methods = %v, want the lock, then the refused photo, then the default", methods)
		}
		if sends := e.groupSends(e.settingsEnv, mark); len(sends) != 2 {
			t.Fatalf("the group got %+v, want the refused photo and the default", sends)
		}
		e.wantGoldenLast("the default after the refusal", goldenLockdownStart, 101, id)
		lockdownGoldenNoQueue(t, e.chat.Id)
	})

	t.Run("silent", func(t *testing.T) {
		e := newLockdownMessagesE2E(t, false)
		e.silence("ld", eventmsg.EventLockdown)
		e.silence("lu", eventmsg.EventLockLift)

		mark := settingsEditMarkSends(e.settingsEnv, e.chat.Id)
		e.lockCommand(e.settingsEnv, e.admin, "/lockdown raid")
		row := lockdownEventWantStarted(t, e.lockdownEnv, e.admin.Id)
		if sends := e.groupSends(e.settingsEnv, mark); len(sends) != 0 {
			t.Errorf("a silent /lockdown posted %+v, want nothing", sends)
		}
		e.cycle()
		if got := len(e.fake.sentTo(e.staffChat.Id)); got != 1 {
			t.Errorf("%d messages reached the Staff Group, want the alert", got)
		}

		mark = settingsEditMarkSends(e.settingsEnv, e.chat.Id)
		e.lockCommand(e.settingsEnv, e.admin, "/unlockdown")
		lockdownLiftWantLifted(t, e.lockdownEnv, row, e.admin.Id)
		if sends := e.groupSends(e.settingsEnv, mark); len(sends) != 0 {
			t.Errorf("a silent /unlockdown posted %+v, want nothing", sends)
		}
	})

	t.Run("untouched surfaces", func(t *testing.T) {
		e := newLockdownMessagesE2E(t, false)
		const sentinel = "ZZE2ELOCK"
		for _, part := range []struct{ code, text string }{
			{"ld", sentinel + "START {reason}"}, {"lj", sentinel + "JOIN {count}"}, {"lf", sentinel + "FLOOD {count}"},
			{"la", sentinel + "AI {count}"}, {"lu", sentinel + "LIFT {admin}"},
		} {
			e.setLockdownPart(part.code, settingsEditPartText, gotgbot.Message{Text: part.text})
		}
		mark := len(lockdownSurfaceCalls(e.fake.moduleBotClient))

		before := e.newJoiner("Before")
		e.join(before, before, "")
		row := e.lockAsAdmin("raid wave")
		time.Sleep(3 * time.Millisecond)
		raider := e.newJoiner("Raider")
		e.join(raider, raider, "")

		// The alert, then a reminder made due.
		e.cycle()
		if staffGot, logGot := len(e.fake.sentTo(e.staffChat.Id)), len(e.fake.sentTo(e.logChannel)); staffGot != 1 || logGot != 1 {
			t.Fatalf("the alert reached the Staff Group %d and the log channel %d times, want once each", staffGot, logGot)
		}
		ageAlert(t, row.ID, 61*time.Minute)
		e.cycle()
		if rows := e.reminderRows(); len(rows) != 2 {
			t.Fatalf("reminder rows = %+v, want one per destination", rows)
		}

		// "Ban N recent joiners": the card's report and the #LOCKDOWN_BAN post.
		a := lastAlert(t, e.lockdownEnv, e.staffChat)
		card := e.confirmBan(a, e.admin)
		e.cyclesUntilReported(8)
		if text := e.lastCardText(e.staffChat.Id, card.msg.MessageID); !strings.Contains(text, staffMarker("lockdown_ban_result")) {
			t.Fatalf("the Ban card = %q, want the run's report", text)
		}
		if got := len(e.logBanPosts(e.logChannel)); got != 1 {
			t.Fatalf("%d #LOCKDOWN_BAN posts in the log channel, want 1", got)
		}

		// The alert's Lift and its Confirm, then the cycles that close the copies.
		pressLockdownButton(e.lockdownEnv, e.admin, e.staffChat, a.alert.MessageID, lockdownButtonData(t, a.alert.Params["reply_markup"], "lk"))
		posted := e.fake.sentTo(e.staffChat.Id)
		liftCard := posted[len(posted)-1]
		pressLockdownButton(e.lockdownEnv, e.admin, e.staffChat, liftCard.MessageID, lockdownButtonData(t, liftCard.Params["reply_markup"], "lc"))
		e.cycleUntilIdle()
		lockdownLiftWantLifted(t, e.lockdownEnv, row, e.admin.Id)
		closed := 0
		for _, alertRow := range e.alertRows() {
			if alertRow.Kind == models.AlertKindAlert && alertRow.ClosedAt != nil {
				closed++
			}
		}
		if closed != 2 {
			t.Fatalf("%d alert copies closed at the lift, want both", closed)
		}

		calls := lockdownSurfaceCalls(e.fake.moduleBotClient)[mark:]
		lockdownE2EWantNoneCarry(t, lockdownCallsTo(calls, e.staffChat.Id), "the Staff Group", sentinel)
		lockdownE2EWantNoneCarry(t, lockdownCallsTo(calls, e.logChannel), "the log channel", sentinel)
		var carried []string
		var others []moduleBotCall
		for _, call := range lockdownCallsTo(calls, e.chat.Id) {
			if text := fmt.Sprint(call.Params["text"]); strings.HasPrefix(call.Method, "send") && strings.Contains(text, sentinel) {
				carried = append(carried, text)
				continue
			}
			others = append(others, call)
		}
		if len(carried) != 2 || !strings.HasPrefix(carried[0], sentinel+"START raid wave") || !strings.HasPrefix(carried[1], sentinel+"LIFT ") {
			t.Fatalf("the group's messages carrying a custom text = %q, want the start reply and the lift notice", carried)
		}
		lockdownE2EWantNoneCarry(t, others, "the group's other requests (the tally, the bans and the edits)", sentinel)
		e.wantNoPrivateSend(mark, "the lockdown flow")
	})

	t.Run("two replicas", func(t *testing.T) {
		e := newLockdownMessagesE2E(t, false)
		e.setLockdownPart("ld", settingsEditPartText, gotgbot.Message{Text: "Replica start {reason}"})

		replica := e.otherReplica()
		replica.fake.setBotMember(e.chat.Id, lockdownBotAdminJSON(true, true, true))
		replica.fake.sendDate = func() int64 { return time.Now().Unix() }

		// Replica B posts the custom start reply and holds the message in the shared cache.
		mark := settingsEditMarkSends(replica, e.chat.Id)
		e.lockCommand(replica, e.admin, "/lockdown raid")
		want := "Replica start raid" + lockdownE2EApprovedNote
		if sends := e.groupSends(replica, mark); len(sends) != 1 || fmt.Sprint(sends[0].Params["text"]) != want {
			t.Fatalf("replica B posted %+v, want %q", sends, want)
		}
		e.lockCommand(replica, e.admin, "/unlockdown")
		if row := e.activeLockdown(); row != nil {
			t.Fatalf("replica B's /unlockdown left the lockdown %+v active", row)
		}

		// The owner silences the manual start through replica A; B's next /lockdown locks and posts nothing.
		e.silence("ld", eventmsg.EventLockdown)
		locks := len(callsToChat(replica.fake.staffBotClient, "setChatPermissions", e.chat.Id))
		mark = settingsEditMarkSends(replica, e.chat.Id)
		e.lockCommand(replica, e.admin, "/lockdown raid again")
		if row := e.activeLockdown(); row == nil || row.LockedAt == nil {
			t.Fatalf("replica B's second /lockdown left no confirmed lockdown (%+v)", row)
		}
		if got := len(callsToChat(replica.fake.staffBotClient, "setChatPermissions", e.chat.Id)); got <= locks {
			t.Errorf("replica B's second /lockdown did not lock the group")
		}
		if sends := e.groupSends(replica, mark); len(sends) != 0 {
			t.Errorf("replica B posted %+v after the Silent press on A, want nothing", sends)
		}
	})
}
