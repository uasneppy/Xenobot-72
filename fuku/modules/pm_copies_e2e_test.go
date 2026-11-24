//go:build testtools

package modules

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/db/lang"
	"github.com/uasneppy/fuku_robot/fuku/db/logchannels"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/db/timezone"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
	"github.com/uasneppy/fuku_robot/fuku/utils/cache"
)

// Private copies end to end (plan 22-09; PMSG-01 to PMSG-07). Everything is real but Telegram: the owner works the
// private Messages page through the deep link and the buttons the bot drew, every part is a typed private message, an
// administrator's /ban, /tmute and their silent variants run through the real dispatcher with the Bans and Mutes
// modules, the punished person opens their Details through the real /start deep link, a Staff Group fan-out runs on
// the staff environment and a second replica handles a /ban. Only the Bot API is the hand-written fake, wrapped by
// Phase 21's messagesE2EClient, which answers a registered member's private chat and can refuse every send to a
// member who never started the bot.

// pmCopiesE2EBanText is what the owner types as the ban's private text; its first word is made bold.
const pmCopiesE2EBanText = "{first}, you were banned from {chatname}: {reason}"

// pmCopiesE2EBanStored is the stored form of pmCopiesE2EBanText.
const pmCopiesE2EBanStored = "<b>{first}</b>, you were banned from {chatname}: {reason}"

// pmCopiesE2EBlocked is Telegram's answer to a send to a person who never started the bot.
var pmCopiesE2EBlocked = &gotgbot.TelegramError{Method: "sendMessage", Code: 403, Description: "Forbidden: bot can't initiate conversation with a user"}

// pmCopiesE2E is Phase 21's messagesE2E with a fast private copy pacer and the private copy keys removed when the
// test ends; every private copy job is awaited before the cleanups run.
type pmCopiesE2E struct {
	*messagesE2E
}

// newPMCopiesE2E builds one environment for one subtest; real installs the real locale files.
func newPMCopiesE2E(t *testing.T, real bool) *pmCopiesE2E {
	t.Helper()
	withFastPMCopyPacer(t)
	e := newMessagesE2E(t, real)
	eventPMCleanup(t, e.chat.Id)
	t.Cleanup(pmCopyJobs.wait)
	return &pmCopiesE2E{messagesE2E: e}
}

// allow lets every send to chatID through again (the person has started the bot).
func (c *messagesE2EClient) allow(chatID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.refused, chatID)
}

// openPM walks to the event's private copy screen of user's private menu, each step a press of the drawn button.
func (e *pmCopiesE2E) openPM(user gotgbot.User, code string) *messagesE2EMenu {
	e.t.Helper()
	m := e.openEvent(user, code)
	m.tap("a", settingsActMsgEvent, settingsMsgDestField, settingsMsgDestPMCode)
	return m
}

// openHub opens the Messages hub of user's private menu.
func (e *pmCopiesE2E) openHub(user gotgbot.User) *messagesE2EMenu {
	e.t.Helper()
	m := e.openMain(user)
	m.tap("a", settingsActOpen, "p", settingsPageMessages)
	return m
}

// switchPM turns the event's private copy on or off as the owner, with the private copy screen's drawn switch.
func (e *pmCopiesE2E) switchPM(code string, on bool) {
	e.t.Helper()
	v := "0"
	if on {
		v = "1"
	}
	e.openPM(e.owner, code).tap("a", settingsActMsgPM, "v", v)
}

// tapOption sets one of the hub's private copy options (code d Details, l the log line) as the owner.
func (e *pmCopiesE2E) tapOption(code string, on bool) {
	e.t.Helper()
	v := "0"
	if on {
		v = "1"
	}
	e.openHub(e.owner).tap("a", settingsActMsgPMOption, "t", code, "v", v)
}

// setPMPart presses Set of part on the event's private copy screen as the owner and sends msg as the typed reply.
func (e *pmCopiesE2E) setPMPart(code, part string, msg gotgbot.Message) {
	e.t.Helper()
	m := e.openPM(e.owner, code)
	m.tap("a", settingsActMsgSet, "t", part, settingsMsgDestField, settingsMsgDestPMCode)
	if !e.redis.Exists(settingsInputKey(e.owner.Id)) {
		e.t.Fatalf("Set of the private %s/%s left no wait", code, part)
	}
	sendPrivateMessage(e.settingsEnv, e.owner, msg)
	if e.redis.Exists(settingsInputKey(e.owner.Id)) {
		e.t.Fatalf("the typed private %s of %s did not finish the wait", part, code)
	}
}

// startDeepLink runs /start <payload> in user's private chat through the real dispatcher.
func (e *pmCopiesE2E) startDeepLink(user gotgbot.User, payload string) {
	e.t.Helper()
	e.sendPrivate(user, "/start "+payload)
}

// privateSends is every send of env's bot to user's private chat, after every private copy job ended.
func (e *pmCopiesE2E) privateSends(env *settingsEnv, user gotgbot.User) []moduleBotCall {
	e.t.Helper()
	waitEventPM(e.t)
	return settingsEditNewSends(env, user.Id, map[string]int{}, false)
}

// detailsEdits is every editMessageReplyMarkup of the group, after every private copy job ended.
func (e *pmCopiesE2E) detailsEdits() []moduleBotCall {
	e.t.Helper()
	waitEventPM(e.t)
	return callsToChat(e.fake.staffBotClient, "editMessageReplyMarkup", e.chat.Id)
}

// logLines is every #PM_COPY post the bot sent to channelID.
func (e *pmCopiesE2E) logLines(channelID int64) []string {
	e.t.Helper()
	waitEventPM(e.t)
	var lines []string
	for _, call := range callsToChat(e.fake.staffBotClient, "sendMessage", channelID) {
		if text := fmt.Sprint(call.Params["text"]); strings.Contains(text, pmCopyLogTag) {
			lines = append(lines, text)
		}
	}
	return lines
}

// linkLogChannel makes a fresh channel the group's log channel, removed when the test ends.
func (e *pmCopiesE2E) linkLogChannel() int64 {
	e.t.Helper()
	channelID := uniqueModuleChatID()
	if err := logchannels.Set(e.chat.Id, e.chat.Title, channelID); err != nil {
		e.t.Fatalf("logchannels.Set error = %v", err)
	}
	e.t.Cleanup(func() { db.DB.Where("chat_id = ?", e.chat.Id).Delete(&models.LogChannel{}) })
	return channelID
}

// pmCopiesE2EOne fails unless calls is exactly one send of method and returns its decoded parameters.
func pmCopiesE2EOne(t *testing.T, calls []moduleBotCall, method, what string) map[string]any {
	t.Helper()
	if len(calls) != 1 || calls[0].Method != method {
		t.Fatalf("%s: got %+v, want one %s", what, calls, method)
	}
	return rulesCallParams(t, calls[0])
}

func TestPMCopiesEndToEnd(t *testing.T) {
	t.Run("owner sets a private copy", func(t *testing.T) {
		e := newPMCopiesE2E(t, true)
		member := e.member("Ann & <Bo>")

		e.setPMPart("bn", settingsEditPartText, gotgbot.Message{
			Text: pmCopiesE2EBanText, Entities: []gotgbot.MessageEntity{{Type: "bold", Offset: 0, Length: 7}},
		})
		if row, _ := settingsMsgStoredPM(t, e.chat.Id, eventmsg.EventBan); row.Text != pmCopiesE2EBanStored || !row.Silent {
			t.Fatalf("after the typed text the pm row is %+v, want the stored text with the copy still off", row)
		}
		e.switchPM("bn", true)
		if row, _ := settingsMsgStoredPM(t, e.chat.Id, eventmsg.EventBan); row.Silent || row.UpdatedBy != e.owner.Id {
			t.Fatalf("after the switch the pm row is %+v, want the copy on, written by the owner", row)
		}
		if hub := e.openHub(e.owner); strings.Count(hub.text, settingsMsgPMSuffix) != 1 {
			t.Errorf("the hub carries the private copy mark %d times, want once (the ban line):\n%s", strings.Count(hub.text, settingsMsgPMSuffix), hub.text)
		}

		mark := settingsEditMarkSends(e.settingsEnv, e.chat.Id)
		id := e.command(e.settingsEnv, e.admin, "/ban spam <b>", member)
		if got := e.punished(e.settingsEnv, "banChatMember", member); got != 1 {
			t.Fatalf("/ban banned %d times, want once", got)
		}
		sends := e.groupSends(e.settingsEnv, mark)
		if len(sends) != 1 {
			t.Fatalf("the group got %+v, want the one default reply", sends)
		}
		e.wantGolden(sends[0], goldenModerationBanReason, member, id)

		params := pmCopiesE2EOne(t, e.privateSends(e.settingsEnv, member), "sendMessage", "the member's private chat")
		const want = "<b>Ann &amp; &lt;Bo&gt;</b>, you were banned from Lock Chat: spam &lt;b&gt;"
		if params["text"] != want || params["parse_mode"] != "HTML" {
			t.Errorf("the private copy is %q (%v), want %q in HTML", params["text"], params["parse_mode"], want)
		}
		if rows := settingsMsgSentKeyboard(t, params); len(rows) != 0 {
			t.Errorf("the private copy carries the keyboard %+v, want none (never the Unban row)", rows)
		}
		if _, ok := params["reply_parameters"]; ok {
			t.Errorf("the private copy is a reply")
		}
		group, private := e.fake.sentTo(e.chat.Id), e.fake.sentTo(member.Id)
		if len(group) == 0 || group[len(group)-1].MessageID > private[0].MessageID {
			t.Errorf("the private copy was not sent after the group reply")
		}
	})

	t.Run("previews", func(t *testing.T) {
		e := newPMCopiesE2E(t, false)
		member := e.member("Mia")
		e.setPMPart("bn", settingsEditPartText, gotgbot.Message{
			Text: pmCopiesE2EBanText, Entities: []gotgbot.MessageEntity{{Type: "bold", Offset: 0, Length: 7}},
		})
		e.switchPM("bn", true)
		group := settingsEditMarkSends(e.settingsEnv, e.chat.Id)
		owner := settingsEditMarkSends(e.settingsEnv, e.owner.Id)

		m := e.openPM(e.owner, "bn")
		m.tap("a", settingsActMsgFull, settingsMsgEventField, "bn", settingsMsgDestField, settingsMsgDestPMCode)
		seen := settingsEditNewSends(e.settingsEnv, e.owner.Id, owner, false)
		if len(seen) < 2 || !containsMarker(fmt.Sprint(seen[len(seen)-2].Params["text"]), "settings_msg_preview_header_pm") {
			t.Fatalf("Full preview sent %+v to the owner, want the private copy header and the content", seen)
		}
		params := rulesCallParams(t, seen[len(seen)-1])
		// A private menu knows the group from its getChat, which the fake answers with the title "Test Chat".
		want := "<b>Owner</b>, you were banned from Test Chat: " + staffMarker("settings_msg_sample_reason")
		if params["text"] != want {
			t.Errorf("the preview reads %q, want %q", params["text"], want)
		}
		if rows := settingsMsgSentKeyboard(t, params); len(rows) != 0 {
			t.Errorf("the preview carries %+v, want no keyboard", rows)
		}
		if sends := e.groupSends(e.settingsEnv, group); len(sends) != 0 {
			t.Errorf("Full preview sent %d messages to the group", len(sends))
		}
		if sends := e.privateSends(e.settingsEnv, member); len(sends) != 0 {
			t.Errorf("Full preview sent %d messages to the member", len(sends))
		}
	})

	t.Run("default text", func(t *testing.T) {
		e := newPMCopiesE2E(t, true)
		member := e.member("Tim")
		if err := lang.ChangeGroupLanguage(e.chat.Id, "es"); err != nil {
			t.Fatalf("ChangeGroupLanguage error = %v", err)
		}
		if err := timezone.Set(context.Background(), e.chat.Id, "Asia/Tokyo", e.owner.Id); err != nil {
			t.Fatalf("timezone.Set error = %v", err)
		}
		e.switchPM("tm", true)

		e.command(e.settingsEnv, e.admin, "/tmute 30m", member)
		restricts := callsToChat(e.fake.staffBotClient, "restrictChatMember", e.chat.Id)
		if len(restricts) != 1 {
			t.Fatalf("restrictChatMember calls = %d, want 1", len(restricts))
		}
		tokyo, err := time.LoadLocation("Asia/Tokyo")
		if err != nil {
			t.Fatalf("load Asia/Tokyo: %v", err)
		}
		until := time.Unix(staffParamInt(restricts[0].Params, "until_date"), 0)
		params := i18n.TranslationParams{"group": "Lock Chat", "until": formatChatTime(until, tokyo, "02.01.2006 15:04")}
		want, _ := i18n.MustNewTranslator("es").GetString("eventmsg_pm_tmute", params)
		ukrainian, _ := i18n.MustNewTranslator("uk").GetString("eventmsg_pm_tmute", params)
		if want == "" || want == ukrainian || !strings.Contains(want, "(UTC+9)") {
			t.Fatalf("the expected Spanish default %q is not a Spanish text with the Tokyo end", want)
		}
		sent := pmCopiesE2EOne(t, e.privateSends(e.settingsEnv, member), "sendMessage", "the member's private chat")
		if sent["text"] != want {
			t.Errorf("the default private copy is %q, want %q", sent["text"], want)
		}
	})

	t.Run("not started", func(t *testing.T) {
		e := newPMCopiesE2E(t, false)
		channelID := e.linkLogChannel()
		member, other := e.member("Nora"), e.member("Otto")
		e.client.refuse(member.Id, pmCopiesE2EBlocked)
		e.switchPM("bn", true)
		e.tapOption("d", true)
		e.tapOption("l", true)
		if row, ok := settingsMsgStoredOptions(t, e.chat.Id); !ok || !row.PMDetails || !row.PMLog {
			t.Fatalf("after the hub presses the options row is %+v (row %v), want Details and Log on", row, ok)
		}

		mark := settingsEditMarkSends(e.settingsEnv, e.chat.Id)
		e.command(e.settingsEnv, e.admin, "/ban spam", member)
		if got := e.punished(e.settingsEnv, "banChatMember", member); got != 1 {
			t.Fatalf("/ban banned %d times, want once", got)
		}
		sends := e.groupSends(e.settingsEnv, mark)
		if len(sends) != 1 || sends[0].Method != "sendMessage" {
			t.Fatalf("the group got %+v, want the one default reply", sends)
		}
		group := e.fake.sentTo(e.chat.Id)
		groupMsg := group[len(group)-1].MessageID

		edits := e.detailsEdits()
		if len(edits) != 1 || staffParamInt(edits[0].Params, "message_id") != groupMsg {
			t.Fatalf("editMessageReplyMarkup of the group = %+v, want one of the reply %d", edits, groupMsg)
		}
		edit := rulesCallParams(t, edits[0])
		markup, _ := edit["reply_markup"].(map[string]any)
		rows, _ := markup["inline_keyboard"].([]any)
		token := eventPMDetailsTokenOf(t, rows)
		if keyboard := settingsMsgSentKeyboard(t, edit); len(keyboard) != 2 ||
			keyboard[0][0].CallbackData != encodeCallbackData("unrestrict", map[string]string{"a": "unban", "u": strconv.FormatInt(member.Id, 10)}) {
			t.Errorf("the edited keyboard = %+v, want the Unban row, then Details", keyboard)
		}
		rec, _ := eventPMDetailsRecord(t, token)
		if rec.UserID != member.Id || rec.ChatID != e.chat.Id || !containsMarker(rec.Text, "eventmsg_pm_ban") {
			t.Fatalf("the Details record is %+v, want the member's default ban copy", rec)
		}
		if sends := e.privateSends(e.settingsEnv, member); len(sends) != 0 {
			t.Fatalf("the refused copy reached the member: %+v", sends)
		}

		lines := e.logLines(channelID)
		if len(lines) != 1 || !strings.Contains(lines[0], fmt.Sprintf("tg://user?id=%d", member.Id)) ||
			!strings.Contains(lines[0], fmt.Sprintf("<code>%d</code>", member.Id)) {
			t.Fatalf("the log channel got %q, want one #PM_COPY line naming the member", lines)
		}

		// The member starts the bot and opens Details: they see their copy.
		e.client.allow(member.Id)
		e.startDeepLink(member, pmCopyDetailsPrefix+token)
		shown := pmCopiesE2EOne(t, e.privateSends(e.settingsEnv, member), "sendMessage", "the member's Details")
		if shown["text"] != rec.Text {
			t.Errorf("Details showed %q, want the stored copy %q", shown["text"], rec.Text)
		}

		// Anyone else gets the neutral reply only.
		e.startDeepLink(other, pmCopyDetailsPrefix+token)
		reply := pmCopiesE2EOne(t, e.privateSends(e.settingsEnv, other), "sendMessage", "another member's Details")
		if text := fmt.Sprint(reply["text"]); text != staffMarker("eventmsg_pm_details_none") {
			t.Errorf("another member's Details answered %q, want the neutral reply alone", text)
		}
	})

	t.Run("refused custom", func(t *testing.T) {
		e := newPMCopiesE2E(t, false)
		member := e.member("Rex")
		e.setPMPart("bn", settingsEditPartText, gotgbot.Message{Text: "Gone {first}"})
		e.setPMPart("bn", settingsEditPartMedia, settingsMediaItemID("photo", "pm-ban-photo"))
		e.switchPM("bn", true)
		e.fake.script("sendPhoto", member.Id, staffFakeError(400, "Bad Request: wrong file identifier/HTTP URL specified"))

		e.command(e.settingsEnv, e.admin, "/ban spam", member)
		sends := e.privateSends(e.settingsEnv, member)
		var photos, texts []moduleBotCall
		for _, call := range sends {
			switch call.Method {
			case "sendPhoto":
				photos = append(photos, call)
			case "sendMessage":
				texts = append(texts, call)
			}
		}
		if len(photos) != 1 || len(sends) != 2 {
			t.Fatalf("the member's private chat got %+v, want the refused photo and one default text", sends)
		}
		params := pmCopiesE2EOne(t, texts, "sendMessage", "the fallback")
		if text := fmt.Sprint(params["text"]); !containsMarker(text, "eventmsg_pm_ban") || strings.Contains(text, "Gone") {
			t.Errorf("the fallback reads %q, want the default private ban text", text)
		}
		if rows := settingsMsgSentKeyboard(t, params); len(rows) != 0 {
			t.Errorf("the fallback carries %+v, want text alone", rows)
		}
	})

	t.Run("group silent", func(t *testing.T) {
		e := newPMCopiesE2E(t, false)
		started, blocked := e.member("Sam"), e.member("Bea")
		e.client.refuse(blocked.Id, pmCopiesE2EBlocked)
		e.openEvent(e.owner, "bn").tap("a", settingsActMsgSilent, settingsMsgEventField, "bn", "v", "1")
		e.switchPM("bn", true)
		e.tapOption("d", true)

		mark := settingsEditMarkSends(e.settingsEnv, e.chat.Id)
		e.command(e.settingsEnv, e.admin, "/ban spam", started)
		e.command(e.settingsEnv, e.admin, "/ban spam", blocked)
		if e.punished(e.settingsEnv, "banChatMember", started) != 1 || e.punished(e.settingsEnv, "banChatMember", blocked) != 1 {
			t.Fatalf("the silent /ban did not ban both members")
		}
		if sends := e.groupSends(e.settingsEnv, mark); len(sends) != 0 {
			t.Errorf("a silent /ban posted %+v in the group", sends)
		}
		pmCopiesE2EOne(t, e.privateSends(e.settingsEnv, started), "sendMessage", "the started member's private chat")
		if edits := e.detailsEdits(); len(edits) != 0 {
			t.Errorf("with no group message the job made %d edits", len(edits))
		}
		if tokens := eventPMDetailsTokens(t); len(tokens) != 0 {
			t.Errorf("with no group message the job stored %d Details records", len(tokens))
		}
	})

	t.Run("silent variants", func(t *testing.T) {
		e := newPMCopiesE2E(t, false)
		sbanned, skicked := e.member("Sal"), e.member("Ski")
		e.switchPM("bn", true)
		e.switchPM("kk", true)

		e.command(e.settingsEnv, e.admin, "/sban", sbanned)
		e.command(e.settingsEnv, e.admin, "/skick", skicked)
		if e.punished(e.settingsEnv, "banChatMember", sbanned) != 1 || e.punished(e.settingsEnv, "unbanChatMember", skicked) != 1 {
			t.Fatalf("/sban or /skick did not act")
		}
		for _, user := range []gotgbot.User{sbanned, skicked} {
			if sends := e.privateSends(e.settingsEnv, user); len(sends) != 0 {
				t.Errorf("a silent variant sent %+v to member %d", sends, user.Id)
			}
		}
	})

	t.Run("rights", func(t *testing.T) {
		e := newPMCopiesE2E(t, false)
		listEditAdmin(e.settingsEnv, e.chat, e.admin, false)
		settingsMsgSeedPM(t, e.chat.Id, eventmsg.EventBan, "Seeded", false)
		before, _ := settingsMsgStoredPM(t, e.chat.Id, eventmsg.EventBan)

		// Opening the private copy screen and previewing need only the admin check and the owner gate.
		m := e.openPM(e.admin, "bn")
		if !m.has("a", settingsActMsgPM) {
			t.Fatalf("the private copy screen did not open for an administrator without can_change_info:\n%s", m.text)
		}
		mark := settingsEditMarkSends(e.settingsEnv, e.admin.Id)
		m.tap("a", settingsActMsgFull, settingsMsgEventField, "bn", settingsMsgDestField, settingsMsgDestPMCode)
		if answer, alert := e.lastAnswer(); alert || answer != "" {
			t.Errorf("Full preview by an administrator without can_change_info answered %q (alert %v)", answer, alert)
		}
		if seen := settingsEditNewSends(e.settingsEnv, e.admin.Id, mark, true); len(seen) != 1 {
			t.Errorf("Full preview sent %d messages after the header, want 1", len(seen))
		}

		refused := func(m *messagesE2EMenu, kv ...string) {
			t.Helper()
			edits := len(e.edits(e.admin.Id, m.msgID))
			m.tap(kv...)
			if answer, alert := e.lastAnswer(); !alert || !containsMarker(answer, "settings_no_change_info") {
				t.Errorf("%v answered %q (alert %v), want the alert settings_no_change_info", kv, answer, alert)
			}
			if got := len(e.edits(e.admin.Id, m.msgID)); got != edits {
				t.Errorf("the refused %v edited the screen", kv)
			}
		}
		refused(m, "a", settingsActMsgSet, "t", settingsEditPartText, settingsMsgDestField, settingsMsgDestPMCode)
		refused(m, "a", settingsActMsgRemove, "t", settingsEditPartText, settingsMsgDestField, settingsMsgDestPMCode)
		refused(m, "a", settingsActMsgPM, "v", "1")
		hub := e.openHub(e.admin)
		refused(hub, "a", settingsActMsgPMOption, "t", "d", "v", "1")
		refused(hub, "a", settingsActMsgPMOption, "t", "l", "v", "1")
		if e.redis.Exists(settingsInputKey(e.admin.Id)) {
			t.Errorf("a refused Set left a wait")
		}
		if row, ok := settingsMsgStoredOptions(t, e.chat.Id); ok {
			t.Errorf("the refused option presses wrote the options row %+v", row)
		}

		// A wait whose sender is demoted before the typed message stores nothing.
		listEditAdmin(e.settingsEnv, e.chat, e.admin, true)
		m = e.openPM(e.admin, "bn")
		m.tap("a", settingsActMsgSet, "t", settingsEditPartText, settingsMsgDestField, settingsMsgDestPMCode)
		listEditAdmin(e.settingsEnv, e.chat, e.admin, false)
		sent := settingsEditSentCount(e.settingsEnv, e.admin)
		sendPrivateMessage(e.settingsEnv, e.admin, gotgbot.Message{Text: "Typed after the demotion"})
		settingsEditWantReply(t, e.settingsEnv, e.admin, sent, "settings_no_change_info")
		settingsEditWaitGone(t, e.settingsEnv, e.admin)
		if after, _ := settingsMsgStoredPM(t, e.chat.Id, eventmsg.EventBan); after.Text != before.Text || !after.Silent || !after.UpdatedAt.Equal(before.UpdatedAt) {
			t.Errorf("the pm row changed to %+v, want it as seeded", after)
		}
	})

	t.Run("staff group action", func(t *testing.T) {
		withFastPMCopyPacer(t)
		env := newStaffActionEnv(t, 1)
		group := env.groups[0]
		eventPMCleanup(t, group)
		t.Cleanup(pmCopyJobs.wait)
		storeEventPM(t, group, eventmsg.EventBan, "You were banned", true)
		msgs, err := eventMessagesLookup(context.Background(), group)
		if err != nil || msgs.Resolve(eventmsg.EventBan, eventmsg.DestPM).Silent {
			t.Fatalf("the linked group's ban copy does not read on (%v)", err)
		}
		env.fake.setMember(group, staffTestTarget, staffFakeMember{Status: gotgbot.ChatMemberStatusMember})

		summary, _ := env.runStaffCommand(fmt.Sprintf("/ban %d spam", staffTestTarget))
		wantDoneLine(t, summary, "Group A")
		if writes := env.callsTo("banChatMember", group); len(writes) != 1 || staffParamInt(writes[0].Params, "user_id") != staffTestTarget {
			t.Fatalf("banChatMember calls in the linked group = %+v, want one for the target", writes)
		}
		waitEventPM(t)
		for _, method := range settingsEditSendMethods {
			if calls := callsToChat(env.fake.staffBotClient, method, staffTestTarget); len(calls) != 0 {
				t.Errorf("the staff /ban sent %d %s to the target's private chat", len(calls), method)
			}
		}
	})

	t.Run("two replicas", func(t *testing.T) {
		e := newPMCopiesE2E(t, false)
		first, second := e.member("Una"), e.member("Duo")
		e.switchPM("bn", true)

		replica := e.otherReplica()
		LoadBans(replica.dispatcher)
		replica.fake.setBotRole(e.chat.Id, staffRoleAdmin)
		replica.fake.sendDate = func() int64 { return time.Now().Unix() }
		replicaClient := newMessagesE2EClient(replica.bot.BotClient)
		replica.bot.BotClient = replicaClient
		for _, user := range []gotgbot.User{first, second} {
			replicaClient.register(user)
		}

		// Replica B sends the copy and holds the switch in the shared cache.
		e.command(replica, e.admin, "/ban spam", first)
		pmCopiesE2EOne(t, e.privateSends(replica, first), "sendMessage", "the first member's private chat")

		// The owner switches the copy off through replica A; B's next /ban sends none.
		e.switchPM("bn", false)
		e.command(replica, e.admin, "/ban spam", second)
		if e.punished(replica, "banChatMember", second) != 1 {
			t.Fatalf("replica B did not ban after the switch")
		}
		if sends := e.privateSends(replica, second); len(sends) != 0 {
			t.Errorf("replica B sent %+v after the copy was switched off on A", sends)
		}
	})

	t.Run("no redis", func(t *testing.T) {
		e := newPMCopiesE2E(t, false)
		started, blocked := e.member("Una"), e.member("Duo")
		e.client.refuse(blocked.Id, pmCopiesE2EBlocked)

		restore := cache.DisableRedisForTest()
		defer restore()

		// The switch and the option work without Redis.
		e.switchPM("bn", true)
		e.tapOption("d", true)
		if row, ok := settingsMsgStoredOptions(t, e.chat.Id); !ok || !row.PMDetails {
			t.Fatalf("the Details option without Redis is %+v (row %v), want on", row, ok)
		}

		// A manual copy is still sent.
		e.command(e.settingsEnv, e.admin, "/ban spam", started)
		pmCopiesE2EOne(t, e.privateSends(e.settingsEnv, started), "sendMessage", "the started member's private chat")

		// A refused copy gets no Details button: no record can exist without Redis.
		mark := settingsEditMarkSends(e.settingsEnv, e.chat.Id)
		e.command(e.settingsEnv, e.admin, "/ban spam", blocked)
		if sends := e.groupSends(e.settingsEnv, mark); len(sends) != 1 {
			t.Fatalf("the group got %+v, want the default reply", sends)
		}
		if edits := e.detailsEdits(); len(edits) != 0 {
			t.Errorf("without Redis the job added a Details button: %+v", edits)
		}
		restore()
		if tokens := eventPMDetailsTokens(t); len(tokens) != 0 {
			t.Errorf("without Redis the job stored %d Details records", len(tokens))
		}
	})
}
