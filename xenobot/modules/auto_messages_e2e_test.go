//go:build testtools

package modules

import (
	"context"
	"fmt"
	"html"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/Xenobot-72/xenobot/db/antiflood"
	"github.com/uasneppy/Xenobot-72/xenobot/db/deletion"
	"github.com/uasneppy/Xenobot-72/xenobot/db/eventmsg"
	"github.com/uasneppy/Xenobot-72/xenobot/db/warns"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/formatting"
)

// The automatic messages end to end (plan 23-07; EVT-02, EVT-03, the five success criteria of Phase 23). Everything is
// real but Telegram: the owner works the Automatic family of the private Messages page through the deep link and the
// buttons the bot drew, every part is a typed private message, an administrator's /warn and a member's messages in the
// group run through the real dispatcher with the Warns, Antiflood and Blacklists modules, and a real forward of a listed
// source reaches the forward blocklist watcher of a linked group. Only the Bot API is the hand-written fake, wrapped by
// Phase 21's messagesE2EClient. The forward blocklist's Details record and log line are checked against the sentinels by
// TestEventMessageAutoPrivacy (D-31) and not again here.

// autoMessagesE2EAutoDelay is the Bot messages timer of the automatic punishment kind every subtest stores, apart from
// the manual kind's 600 s, so a queue row proves its kind.
const autoMessagesE2EAutoDelay = 900

// autoMessagesE2E is Phase 22's pmCopiesE2E (a fast private copy pacer, every private copy job awaited) with the Warns
// and Blacklists modules on its dispatcher (the settings environment already loads Antiflood), a 900 s timer for the
// automatic punishment kind, and the blacklist and antiflood state of the chat removed when the test ends.
type autoMessagesE2E struct {
	*pmCopiesE2E
}

// newAutoMessagesE2E builds one environment for one subtest; real installs the real locale files, for a subtest that
// compares a request with the goldens recorded in Ukrainian.
func newAutoMessagesE2E(t *testing.T, real bool) *autoMessagesE2E {
	t.Helper()
	e := newPMCopiesE2E(t, real)
	LoadWarns(e.dispatcher)
	LoadBlacklists(e.dispatcher)
	settingsListCleanup(t, e.chat.Id)
	resetAntifloodState(t)
	t.Cleanup(func() { resetAntifloodState(t) })
	if err := deletion.SetRule(context.Background(), e.chat.Id, deletion.KindBotAuto, autoMessagesE2EAutoDelay); err != nil {
		t.Fatalf("SetRule error = %v", err)
	}
	return &autoMessagesE2E{pmCopiesE2E: e}
}

// autoMember makes a plain member of the group named first and Lee, with no username (the goldens' member), a fresh ID,
// a member record in the group and a private chat getChat answers.
func (e *autoMessagesE2E) autoMember(first string) gotgbot.User {
	e.t.Helper()
	user := gotgbot.User{Id: uniqueLinkOwnerID(), FirstName: first, LastName: "Lee"}
	e.client.register(user)
	e.fake.setProfile(user.Id, first, "Lee", "")
	e.fake.setMember(e.chat.Id, user.Id, staffFakeMember{Status: gotgbot.ChatMemberStatusMember})
	memberReqCleanup(e.t, nil, []int64{user.Id})
	return user
}

// openAuto walks Main, Messages, Automatic actions and the event's cell of user's private menu, each a press of the drawn
// button.
func (e *autoMessagesE2E) openAuto(user gotgbot.User, code string) *messagesE2EMenu {
	e.t.Helper()
	m := e.openHub(user)
	m.tap("a", settingsActMsgFamily, "t", "a")
	m.tap("a", settingsActMsgEvent, settingsMsgEventField, code)
	return m
}

// openAutoPM walks on to the event's private copy screen.
func (e *autoMessagesE2E) openAutoPM(user gotgbot.User, code string) *messagesE2EMenu {
	e.t.Helper()
	m := e.openAuto(user, code)
	m.tap("a", settingsActMsgEvent, settingsMsgDestField, settingsMsgDestPMCode)
	return m
}

// setAutoPart presses Set of part on the event's screen as the owner and sends msg as the typed reply.
func (e *autoMessagesE2E) setAutoPart(code, part string, msg gotgbot.Message) {
	e.t.Helper()
	e.openAuto(e.owner, code).tap("a", settingsActMsgSet, "t", part)
	if !e.redis.Exists(settingsInputKey(e.owner.Id)) {
		e.t.Fatalf("Set of %s/%s left no wait", code, part)
	}
	sendPrivateMessage(e.settingsEnv, e.owner, msg)
	if e.redis.Exists(settingsInputKey(e.owner.Id)) {
		e.t.Fatalf("the typed %s of %s did not finish the wait", part, code)
	}
}

// setAutoText sets the event's text from a typed message whose first word is bold.
func (e *autoMessagesE2E) setAutoText(code, text string) {
	e.t.Helper()
	first := strings.IndexByte(text, ' ')
	if first < 0 {
		first = len(text)
	}
	e.setAutoPart(code, settingsEditPartText, gotgbot.Message{
		Text: text, Entities: []gotgbot.MessageEntity{{Type: "bold", Offset: 0, Length: int64(first)}},
	})
}

// setAutoButton sets the event's buttons part to one link button from the page.
func (e *autoMessagesE2E) setAutoButton(code string) {
	e.t.Helper()
	e.setAutoPart(code, settingsEditPartButtons, gotgbot.Message{Text: "[Rules](buttonurl://https://example.com/rules)"})
}

// switchAutoPM turns the event's private copy on with the private copy screen's drawn switch, as the owner.
func (e *autoMessagesE2E) switchAutoPM(code, event string) {
	e.t.Helper()
	e.openAutoPM(e.owner, code).tap("a", settingsActMsgPM, "v", "1")
	if row, _ := settingsMsgStoredPM(e.t, e.chat.Id, event); row.Silent {
		e.t.Fatalf("after the switch the %s private copy is still off", event)
	}
}

// memberSays runs text as member's message in the group through the real dispatcher and returns its message ID.
func (e *autoMessagesE2E) memberSays(member gotgbot.User, text string) int64 {
	e.t.Helper()
	return e.sendIn(e.chat, member, text)
}

// lastGroup is the newest message the bot sent to the group.
func (e *autoMessagesE2E) lastGroup() staffSentMessage {
	e.t.Helper()
	sent := e.fake.sentTo(e.chat.Id)
	if len(sent) == 0 {
		e.t.Fatal("the bot sent nothing to the group")
	}
	return sent[len(sent)-1]
}

// wantQueuedAs fails unless message msgID of the group is queued under kind at its date plus delay seconds.
func (e *autoMessagesE2E) wantQueuedAs(msgID int64, kind string, delay int, what string) {
	e.t.Helper()
	row, ok := e.queued(msgID)
	if !ok {
		e.t.Fatalf("%s (message %d) is not queued for deletion", what, msgID)
	}
	if row.Kind != kind || row.DeleteAt.Sub(row.SentAt) != time.Duration(delay)*time.Second {
		e.t.Errorf("%s is queued as %s at sent + %s, want %s at + %ds", what, row.Kind, row.DeleteAt.Sub(row.SentAt), kind, delay)
	}
}

// wantPunishedFirst fails unless punishment comes before send in the methods the fake was asked for after skip (D-03).
func (e *autoMessagesE2E) wantPunishedFirst(skip int, punishment, send string) {
	e.t.Helper()
	methods := methodsSince(e.settingsEnv, skip)
	if p, s := eventMsgIndex(methods, punishment), eventMsgIndex(methods, send); p < 0 || s < p {
		e.t.Fatalf("methods = %v, want %s before %s", methods, punishment, send)
	}
}

// deleted reports whether the bot deleted message msgID of the group.
func (e *autoMessagesE2E) deleted(msgID int64) bool {
	for _, call := range callsToChat(e.fake.staffBotClient, "deleteMessage", e.chat.Id) {
		if staffParamInt(call.Params, "message_id") == msgID {
			return true
		}
	}
	return false
}

// flood configures antiflood through its repository: limit, the action and whether the burst is deleted.
func (e *autoMessagesE2E) flood(limit int, action string, deleteBurst bool) {
	e.t.Helper()
	if err := antiflood.SetFlood(e.chat.Id, limit); err != nil {
		e.t.Fatalf("SetFlood error = %v", err)
	}
	if err := antiflood.SetFloodMode(e.chat.Id, action); err != nil {
		e.t.Fatalf("SetFloodMode error = %v", err)
	}
	if err := antiflood.SetFloodMsgDel(e.chat.Id, deleteBurst); err != nil {
		e.t.Fatalf("SetFloodMsgDel error = %v", err)
	}
}

// warnLimit stores the group's warn limit and limit action through the repository.
func (e *autoMessagesE2E) warnLimit(limit int, mode string) {
	e.t.Helper()
	if err := warns.SetWarnLimit(e.chat.Id, limit); err != nil {
		e.t.Fatalf("SetWarnLimit error = %v", err)
	}
	if err := warns.SetWarnMode(e.chat.Id, mode); err != nil {
		e.t.Fatalf("SetWarnMode error = %v", err)
	}
}

// warnsOf is the member's stored warn count.
func (e *autoMessagesE2E) warnsOf(member gotgbot.User) int {
	e.t.Helper()
	n, _ := warns.GetWarns(member.Id, e.chat.Id)
	return n
}

// autoMessagesE2EUnmute is the callback data of the flood's Unmute row for member.
func autoMessagesE2EUnmute(member gotgbot.User) string {
	return encodeCallbackData("unrestrict", map[string]string{"a": "unmute", "u": strconv.FormatInt(member.Id, 10)})
}

// autoMessagesE2ERemoveWarn is the callback data of the warn's Remove warn row for member.
func autoMessagesE2ERemoveWarn(member gotgbot.User) string {
	return encodeCallbackData("rmWarn", map[string]string{"u": strconv.FormatInt(member.Id, 10)})
}

// autoMessagesE2EBlacklistReason is the blacklist's reason for the word spam as a custom message shows it (escaped).
const autoMessagesE2EBlacklistReason = "Blacklisted word: &#39;spam&#39;"

func TestAutoMessagesEndToEnd(t *testing.T) {
	t.Run("owner sets a warn message", func(t *testing.T) {
		e := newAutoMessagesE2E(t, false)
		member := e.autoMember("Ann & <Bo>")
		e.setAutoPart("wn", settingsEditPartText, gotgbot.Message{
			Text: "{mention}: {count}/{limit}, {reason}", Entities: []gotgbot.MessageEntity{{Type: "bold", Offset: 0, Length: 9}},
		})
		e.setAutoButton("wn")
		if row, _ := settingsMsgStored(t, e.chat.Id, eventmsg.EventWarn); row.Text != "<b>{mention}</b>: {count}/{limit}, {reason}" || len(row.Buttons) != 1 {
			t.Fatalf("the stored warn message is %+v, want the typed text and one button", row)
		}

		// The hub's warn line reads Custom.
		hub := e.openHub(e.owner)
		if line, ok := settingsMainStateLine(hub.text, "settings_msg_ev_warn"); !ok || !containsMarker(line, "settings_msg_state_custom") {
			t.Errorf("the hub's warn line = %q, want Custom", line)
		}

		mark := settingsEditMarkSends(e.settingsEnv, e.chat.Id)
		id := e.command(e.settingsEnv, e.admin, "/warn spam & co", member)
		if got := e.warnsOf(member); got != 1 {
			t.Fatalf("stored warns = %d, want 1", got)
		}
		params := pmCopiesE2EOne(t, e.groupSends(e.settingsEnv, mark), "sendMessage", "the group after /warn")
		want := "<b>" + formatting.MentionHtml(member.Id, member.FirstName) + "</b>: 1/3, spam &amp; co"
		if params["text"] != want {
			t.Errorf("the warn reads %q, want %q", params["text"], want)
		}
		if reply, _ := params["reply_parameters"].(map[string]any); reply["message_id"] != float64(id) {
			t.Errorf("reply_parameters = %v, want the command %d", params["reply_parameters"], id)
		}
		rows := settingsMsgSentKeyboard(t, params)
		if len(rows) != 2 || len(rows[0]) != 1 || len(rows[1]) != 1 || rows[0][0].Url != "https://example.com/rules" ||
			rows[1][0].CallbackData != autoMessagesE2ERemoveWarn(member) {
			t.Fatalf("the warn's keyboard = %+v, want the custom button, then the Remove warn row", rows)
		}
		warned := e.lastGroup()
		e.wantQueuedAs(warned.MessageID, deletion.KindBotManual, messagesE2EManualDelay, "the custom warn")

		// The kept Remove warn button still removes the warn.
		e.pressRaw(e.chat, e.admin, warned.MessageID, rows[1][0].CallbackData)
		if got := e.warnsOf(member); got != 0 {
			t.Errorf("after the Remove warn tap the member has %d warns, want 0", got)
		}
	})

	t.Run("blacklist warn and warn limit", func(t *testing.T) {
		e := newAutoMessagesE2E(t, false)
		member := e.autoMember("Ann & <Bo>")
		settingsListSeedBlacklistAction(t, e.chat.Id, "spam", "warn")
		e.warnLimit(2, "ban")
		e.setAutoText("wn", "Warned {first} {count}/{limit} by {admin}")
		e.setAutoText("wl", "Out {first} {action} at {count}/{limit}")
		tr := i18n.MustNewTranslator("uk")
		anon, _ := tr.GetString("eventmsg_admin_anon")

		// The first word: the custom warn, by "an admin", queued as an automatic punishment message.
		mark := settingsEditMarkSends(e.settingsEnv, e.chat.Id)
		first := e.memberSays(member, "buy spam now")
		if !e.deleted(first) {
			t.Errorf("the first message with the word was not deleted")
		}
		params := pmCopiesE2EOne(t, e.groupSends(e.settingsEnv, mark), "sendMessage", "the group after the first word")
		if want := "<b>Warned</b> Ann &amp; &lt;Bo&gt; 1/2 by " + anon; params["text"] != want {
			t.Errorf("the warn reads %q, want %q", params["text"], want)
		}
		if rows := settingsMsgSentKeyboard(t, params); len(rows) != 1 || rows[0][0].CallbackData != autoMessagesE2ERemoveWarn(member) {
			t.Errorf("the warn's keyboard = %+v, want the Remove warn row", rows)
		}
		e.wantQueuedAs(e.lastGroup().MessageID, deletion.KindBotAuto, autoMessagesE2EAutoDelay, "the automatic custom warn")

		// The second word reaches the limit: the member is banned, then the custom warn limit names the ban at 2/2.
		mark, calls := settingsEditMarkSends(e.settingsEnv, e.chat.Id), len(eventMsgMethods(e.fake.moduleBotClient))
		second := e.memberSays(member, "spam again")
		if !e.deleted(second) {
			t.Errorf("the second message with the word was not deleted")
		}
		if got := e.punished(e.settingsEnv, "banChatMember", member); got != 1 {
			t.Fatalf("the limit banned %d times, want once", got)
		}
		e.wantPunishedFirst(calls, "banChatMember", "sendMessage")
		params = pmCopiesE2EOne(t, e.groupSends(e.settingsEnv, mark), "sendMessage", "the group after the second word")
		if want := "<b>Out</b> Ann &amp; &lt;Bo&gt; " + eventActionWord(tr, "ban") + " at 2/2"; params["text"] != want {
			t.Errorf("the warn limit reads %q, want %q", params["text"], want)
		}
		e.wantQueuedAs(e.lastGroup().MessageID, deletion.KindBotAuto, autoMessagesE2EAutoDelay, "the custom warn limit")
		if got := e.warnsOf(member); got != 0 {
			t.Errorf("stored warns = %d after the limit action, want 0", got)
		}
	})

	t.Run("flood", func(t *testing.T) {
		e := newAutoMessagesE2E(t, false)
		member := e.autoMember("Ann & <Bo>")
		e.flood(3, "mute", true)
		e.setAutoText("fl", "Flood {first} {action} {count}/{limit}: {reason}")
		e.setAutoButton("fl")
		tr := i18n.MustNewTranslator("uk")

		mark, calls := settingsEditMarkSends(e.settingsEnv, e.chat.Id), len(eventMsgMethods(e.fake.moduleBotClient))
		var burst []int64
		for i := range 4 {
			burst = append(burst, e.memberSays(member, fmt.Sprintf("hello %d", i)))
		}
		for _, id := range burst {
			if !e.deleted(id) {
				t.Errorf("message %d of the burst was not deleted", id)
			}
		}
		if got := e.punished(e.settingsEnv, "restrictChatMember", member); got != 1 {
			t.Fatalf("the flood muted %d times, want once", got)
		}
		e.wantPunishedFirst(calls, "restrictChatMember", "sendMessage")
		params := pmCopiesE2EOne(t, e.groupSends(e.settingsEnv, mark), "sendMessage", "the group after the flood")
		want := "<b>Flood</b> Ann &amp; &lt;Bo&gt; " + eventActionWord(tr, "mute") + " 4/3: " + html.EscapeString(floodEventReason(tr))
		if params["text"] != want {
			t.Errorf("the flood notice reads %q, want %q", params["text"], want)
		}
		rows := settingsMsgSentKeyboard(t, params)
		if len(rows) != 2 || rows[0][0].CallbackData != autoMessagesE2EUnmute(member) || rows[1][0].Url != "https://example.com/rules" {
			t.Errorf("the flood keyboard = %+v, want the Unmute row first, then the custom button", rows)
		}
		e.wantQueuedAs(e.lastGroup().MessageID, deletion.KindBotAuto, autoMessagesE2EAutoDelay, "the custom flood notice")
	})

	t.Run("blacklist mute", func(t *testing.T) {
		e := newAutoMessagesE2E(t, false)
		member := e.autoMember("Ann & <Bo>")
		settingsListSeedBlacklistAction(t, e.chat.Id, "spam", "mute")
		e.setAutoText("bl", "Removed {action}: {reason}")
		tr := i18n.MustNewTranslator("uk")

		mark, calls := settingsEditMarkSends(e.settingsEnv, e.chat.Id), len(eventMsgMethods(e.fake.moduleBotClient))
		id := e.memberSays(member, "buy spam now")
		if !e.deleted(id) {
			t.Errorf("the message with the word was not deleted")
		}
		if got := e.punished(e.settingsEnv, "restrictChatMember", member); got != 1 {
			t.Fatalf("the blacklist muted %d times, want once", got)
		}
		e.wantPunishedFirst(calls, "restrictChatMember", "sendMessage")
		params := pmCopiesE2EOne(t, e.groupSends(e.settingsEnv, mark), "sendMessage", "the group after the word")
		if want := "<b>Removed</b> " + eventActionWord(tr, "mute") + ": " + autoMessagesE2EBlacklistReason; params["text"] != want {
			t.Errorf("the blacklist notice reads %q, want %q", params["text"], want)
		}
		e.wantQueuedAs(e.lastGroup().MessageID, deletion.KindBotAuto, autoMessagesE2EAutoDelay, "the custom blacklist notice")
	})

	t.Run("forward blocklist", func(t *testing.T) {
		env := newFwdWatchEnv(t, "mute")
		withFastPMCopyPacer(t)
		eventPMCleanup(t, env.group)
		t.Cleanup(pmCopyJobs.wait)
		eventMsgStore(t, env.group, eventmsg.EventForwardBlock, eventMsgAutoEveryPlaceholder)
		storeEventPM(t, env.group, eventmsg.EventForwardBlock, eventMsgAutoEveryPlaceholder, true)

		member := env.member()
		env.forward(member)
		waitEventPM(t)
		if calls := env.callsTo("restrictChatMember", env.group); len(calls) != 1 {
			t.Fatalf("restrictChatMember calls = %d, want the one mute", len(calls))
		}
		tr := i18n.MustNewTranslator("uk")
		anon, _ := tr.GetString("eventmsg_admin_anon")
		mention := formatting.MentionHtml(member.Id, member.FirstName)
		want := "Member||Member|" + mention + "|" + mention + "|" + strconv.FormatInt(member.Id, 10) + "|" +
			html.EscapeString(env.groupChat.Title) + "||" + anon + "|" + html.EscapeString(forwardBlockReason(tr)) + "|" +
			eventActionWord(tr, "mute") + "|||||"
		sent := env.fake.sentTo(env.group)
		if len(sent) != 1 || fmt.Sprint(sent[0].Params["text"]) != want {
			t.Fatalf("the linked group got %+v, want the one custom notice %q", sent, want)
		}
		fwdForbiddenInText(t, env, "the custom notice", fmt.Sprint(sent[0].Params["text"]))
		copies := env.callsTo("sendMessage", member.Id)
		if len(copies) != 1 {
			t.Fatalf("sends to the member's private chat = %d, want the one private copy", len(copies))
		}
		text := fmt.Sprint(copies[0].Params["text"])
		if text != want {
			t.Errorf("the private copy reads %q, want %q", text, want)
		}
		fwdForbiddenInText(t, env, "the private copy", text)
	})

	t.Run("nothing stored", func(t *testing.T) {
		e := newAutoMessagesE2E(t, true)
		warned, listed, flooder := e.autoMember("Ann & <Bo>"), e.autoMember("Ann & <Bo>"), e.autoMember("Ann & <Bo>")
		settingsListSeedBlacklistAction(t, e.chat.Id, "spam", "mute")
		e.flood(3, "mute", false)

		mark := settingsEditMarkSends(e.settingsEnv, e.chat.Id)
		id := e.command(e.settingsEnv, e.admin, "/warn spam & <b>", warned)
		sends := e.groupSends(e.settingsEnv, mark)
		if len(sends) != 1 {
			t.Fatalf("the group got %+v after /warn, want the one default", sends)
		}
		e.wantGolden(sends[0], goldenNoticeWarnReason, warned, id)
		e.wantQueuedAs(e.lastGroup().MessageID, deletion.KindBotManual, messagesE2EManualDelay, "the default warn")

		mark = settingsEditMarkSends(e.settingsEnv, e.chat.Id)
		e.memberSays(listed, "buy spam now")
		if sends = e.groupSends(e.settingsEnv, mark); len(sends) != 1 {
			t.Fatalf("the group got %+v after the word, want the one default", sends)
		}
		e.wantGolden(sends[0], goldenNoticeBlacklistMute, listed, 0)
		e.wantQueuedAs(e.lastGroup().MessageID, deletion.KindBotAuto, autoMessagesE2EAutoDelay, "the default blacklist notice")

		mark = settingsEditMarkSends(e.settingsEnv, e.chat.Id)
		for i := range 4 {
			e.memberSays(flooder, fmt.Sprintf("hello %d", i))
		}
		if sends = e.groupSends(e.settingsEnv, mark); len(sends) != 1 {
			t.Fatalf("the group got %+v after the flood, want the one default", sends)
		}
		e.wantGolden(sends[0], goldenNoticeFloodMute, flooder, 0)
		e.wantQueuedAs(e.lastGroup().MessageID, deletion.KindBotAuto, autoMessagesE2EAutoDelay, "the default flood notice")
	})

	t.Run("refused", func(t *testing.T) {
		e := newAutoMessagesE2E(t, true)
		member := e.autoMember("Ann & <Bo>")
		e.flood(3, "mute", false)
		e.setAutoText("fl", "Flood {first} {action}")
		e.setAutoPart("fl", settingsEditPartMedia, settingsMediaItemID("photo", "flood-photo"))
		e.fake.script("sendPhoto", e.chat.Id, staffFakeError(400, "Bad Request: wrong file identifier/HTTP URL specified"))

		mark, calls := settingsEditMarkSends(e.settingsEnv, e.chat.Id), len(eventMsgMethods(e.fake.moduleBotClient))
		for i := range 4 {
			e.memberSays(member, fmt.Sprintf("hello %d", i))
		}
		methods := methodsSince(e.settingsEnv, calls)
		mute, photo, text := eventMsgIndex(methods, "restrictChatMember"), eventMsgIndex(methods, "sendPhoto"), eventMsgIndex(methods, "sendMessage")
		if mute < 0 || photo < mute || text < photo {
			t.Fatalf("methods = %v, want the mute, then the refused photo, then the default", methods)
		}
		sends := e.groupSends(e.settingsEnv, mark)
		var defaults []moduleBotCall
		for _, call := range sends {
			if call.Method == "sendMessage" {
				defaults = append(defaults, call)
			}
		}
		if len(sends) != 2 || len(defaults) != 1 {
			t.Fatalf("the group got %+v, want the refused photo and one default", sends)
		}
		e.wantGolden(defaults[0], goldenNoticeFloodMute, member, 0)
		e.wantQueuedAs(e.lastGroup().MessageID, deletion.KindBotAuto, autoMessagesE2EAutoDelay, "the default after the refusal")
	})

	t.Run("silent", func(t *testing.T) {
		e := newAutoMessagesE2E(t, false)
		member := e.autoMember("Ann & <Bo>")
		e.openAuto(e.owner, "wn").tap("a", settingsActMsgSilent, settingsMsgEventField, "wn", "v", "1")
		if row, ok := settingsMsgStored(t, e.chat.Id, eventmsg.EventWarn); !ok || !row.Silent {
			t.Fatalf("after the Silent press the warn row is %+v, want Silent on", row)
		}
		e.switchAutoPM("wn", eventmsg.EventWarn)

		mark := settingsEditMarkSends(e.settingsEnv, e.chat.Id)
		e.command(e.settingsEnv, e.admin, "/warn spam", member)
		if got := e.warnsOf(member); got != 1 {
			t.Fatalf("stored warns = %d, want 1: a silent warn still warns", got)
		}
		if sends := e.groupSends(e.settingsEnv, mark); len(sends) != 0 {
			t.Errorf("the silent /warn posted %+v in the group, want nothing", sends)
		}
		params := pmCopiesE2EOne(t, e.privateSends(e.settingsEnv, member), "sendMessage", "the member's private chat")
		if text := fmt.Sprint(params["text"]); !containsMarker(text, "eventmsg_pm_warn") {
			t.Errorf("the private copy reads %q, want the default private warn text", text)
		}
	})

	t.Run("pm copies and the cap", func(t *testing.T) {
		e := newAutoMessagesE2E(t, false)
		settingsListSeedBlacklistAction(t, e.chat.Id, "spam", "mute")
		e.switchAutoPM("bl", eventmsg.EventBlacklist)
		e.switchAutoPM("wn", eventmsg.EventWarn)
		now := time.Date(2026, 11, 21, 12, 0, 5, 0, time.UTC)
		mreqSwap(t, &pmCopyNow, func() time.Time { return now })
		mreqSwap(t, &pmCopyAutoCapPerMinute, 2)
		t.Cleanup(pmCopyJobs.wait)

		var offenders []gotgbot.User
		mark := settingsEditMarkSends(e.settingsEnv, e.chat.Id)
		for range 4 {
			offender := e.autoMember("Offender")
			offenders = append(offenders, offender)
			e.memberSays(offender, "buy spam now")
		}
		waitEventPM(t)
		copies := 0
		for _, offender := range offenders {
			if got := e.punished(e.settingsEnv, "restrictChatMember", offender); got != 1 {
				t.Errorf("offender %d was muted %d times, want once: every punishment happens", offender.Id, got)
			}
			copies += len(e.privateSends(e.settingsEnv, offender))
		}
		if sends := e.groupSends(e.settingsEnv, mark); len(sends) != len(offenders) {
			t.Errorf("group notices = %d, want one per offender (%d)", len(sends), len(offenders))
		}
		if copies != 2 {
			t.Errorf("private copies in one minute = %d, want the cap of 2", copies)
		}

		// A /warn in the same minute is an admin's act and is never capped.
		member := e.autoMember("Ann & <Bo>")
		e.command(e.settingsEnv, e.admin, "/warn spam", member)
		if sends := e.privateSends(e.settingsEnv, member); len(sends) != 1 {
			t.Errorf("private copies of the /warn = %d, want 1", len(sends))
		}
	})
}
