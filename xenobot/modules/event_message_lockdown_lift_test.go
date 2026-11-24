//go:build testtools

package modules

import (
	"fmt"
	"html"
	"strings"
	"testing"
	"time"

	"github.com/uasneppy/Xenobot-72/xenobot/db/eventmsg"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/formatting"
)

// lockdownLiftSettingsEnv is a settings environment for a lift from the menu: every deletion rule of its
// chat at the shortest delay and its stored event messages removed when the test ends.
func lockdownLiftSettingsEnv(t *testing.T, golden bool) *settingsEnv {
	t.Helper()
	var env *settingsEnv
	if golden {
		env = newLockdownGoldenSettingsEnv(t)
	} else {
		env = newSettingsEnv(t)
		deletionShortestRules(t, env.chat.Id)
	}
	eventMsgCleanup(t, env.chat.Id)
	return env
}

// lockdownLiftWantLifted fails unless the lockdown row was lifted by liftedBy and the group's permissions
// are the stored pre-lockdown set again.
func lockdownLiftWantLifted(t *testing.T, env *lockdownEnv, row *models.ChatLockdown, liftedBy int64) {
	t.Helper()
	lifted := env.freshRow(row.ID)
	if lifted.State != models.LockdownStateLifting && lifted.State != models.LockdownStateLifted {
		t.Fatalf("lockdown state = %q, want lifting or lifted", lifted.State)
	}
	if lifted.LiftedBy == nil || *lifted.LiftedBy != liftedBy {
		t.Fatalf("lifted by %v, want %d", lifted.LiftedBy, liftedBy)
	}
	if got := env.fake.chatPermsRaw(env.chat.Id); got != lockdownTestPrePermissions {
		t.Fatalf("permissions after the lift = %q, want the stored pre-lockdown set", got)
	}
}

// lockdownLiftGroupSends is how many sendMessage requests went to chatID, refused ones included.
func lockdownLiftGroupSends(env *lockdownEnv, chatID int64) int {
	n := 0
	for _, call := range env.fake.callsFor("sendMessage") {
		if staffParamInt(call.Params, "chat_id") == chatID {
			n++
		}
	}
	return n
}

// lockdownLiftLastText is the text and the serialised request of the last message chatID received.
func lockdownLiftLastText(t *testing.T, env *lockdownEnv, chatID int64) (text, raw string) {
	t.Helper()
	sent := env.fake.sentTo(chatID)
	if len(sent) == 0 {
		t.Fatalf("chat %d received no message", chatID)
	}
	return fmt.Sprint(sent[len(sent)-1].Params["text"]), lockdownGoldenRequest(t, env, chatID, len(sent)-1)
}

// lockdownLiftWantNewMessage fails when raw is a reply or carries a topic.
func lockdownLiftWantNewMessage(t *testing.T, raw string) {
	t.Helper()
	if strings.Contains(raw, "reply_parameters") || strings.Contains(raw, "message_thread_id") {
		t.Fatalf("request = %s, want a new message with no reply and no topic", raw)
	}
}

// lockdownLiftEditsBeforeNotice is how many edits of message msgID in chatID were made before the last
// sendMessage to group.
func lockdownLiftEditsBeforeNotice(env *lockdownEnv, chatID, msgID, group int64) int {
	client := env.fake.moduleBotClient
	client.mu.Lock()
	defer client.mu.Unlock()
	last := -1
	for i, call := range client.calls {
		if call.Method == "sendMessage" && staffParamInt(call.Params, "chat_id") == group {
			last = i
		}
	}
	edits := 0
	for _, call := range client.calls[:last+1] {
		if call.Method == "editMessageText" && staffParamInt(call.Params, "chat_id") == chatID &&
			staffParamInt(call.Params, "message_id") == msgID {
			edits++
		}
	}
	return edits
}

// lockdownLiftSilence stores the group's locklift message as Silent.
func lockdownLiftSilence(t *testing.T, chatID int64) {
	t.Helper()
	if err := eventmsg.SetSilent(t.Context(), chatID, eventmsg.EventLockLift, eventmsg.DestGroup, true, 1); err != nil {
		t.Fatalf("SetSilent error = %v", err)
	}
}

// lockdownLiftGroupMenuYes opens the Lockdown page in a menu in the group, asks to lift and presses Yes,
// returning the menu message and how many messages the group had received before the Yes.
func lockdownLiftGroupMenuYes(t *testing.T, env *settingsEnv) (msgID int64, before int) {
	t.Helper()
	msgID = settingsLockdownTestOpen(t, env)
	_, keyboard := env.lastEdit(env.chat.Id, msgID)
	env.pressRaw(env.chat, env.admin, msgID, lockdownGoldenSettingsButton(t, keyboard, "ls").CallbackData)
	_, card := env.lastEdit(env.chat.Id, msgID)
	before = len(env.fake.sentTo(env.chat.Id))
	lockdownApplyDrainWake()
	env.pressRaw(env.chat, env.admin, msgID, lockdownGoldenSettingsButton(t, card, "ly").CallbackData)
	return msgID, before
}

func TestEventMessageLockdownLift(t *testing.T) {
	t.Run("command", func(t *testing.T) {
		env := lockdownEventEnv(t, false)
		row := env.lockAsAdmin("raid")
		eventMsgStore(t, env.chat.Id, eventmsg.EventLockLift, "<b>{admin}</b> opened {chatname} ({mention})")

		env.send(env.admin, "/unlockdown")

		lockdownLiftWantLifted(t, env, row, env.admin.Id)
		text, raw := lockdownLiftLastText(t, env, env.chat.Id)
		want := "<b>Ad&lt;b&gt;min</b> opened " + html.EscapeString(row.ChatTitle) + " (" +
			formatting.MentionHtml(env.admin.Id, "Ad<b>min") + ")"
		if text != want {
			t.Fatalf("custom lift reply\n got %q\nwant %q", text, want)
		}
		if replyTo := `"reply_parameters":{"message_id":102,"allow_sending_without_reply":true}`; !strings.Contains(raw, replyTo) {
			t.Fatalf("request = %s, want a reply to the command (%s)", raw, replyTo)
		}
		lockdownGoldenWantSends(t, env, env.chat.Id, 2)
		lockdownGoldenNoQueue(t, env.chat.Id)
		lockdownGoldenNoPrivate(t, env)
	})

	t.Run("command notes", func(t *testing.T) {
		env := lockdownEventEnv(t, false)
		row := env.lockAsAdmin("raid")
		seedLockdownJoiner(t, row.ID, env.chat.Id, uniqueLinkOwnerID(), models.JoinerStateBanned)
		seedLockdownJoiner(t, row.ID, env.chat.Id, uniqueLinkOwnerID(), models.JoinerStateBanned)
		env.fake.setChatPermsRaw(env.chat.Id, lockdownGoldenChangedPerms)
		eventMsgStore(t, env.chat.Id, eventmsg.EventLockLift, "opened by {admin}")

		env.send(env.admin, "/unlockdown")

		if lifted := env.freshRow(row.ID); lifted.LiftedBy == nil || !lifted.ManualChange {
			t.Fatalf("lockdown row = %+v, want a lift recorded with a manual change", lifted)
		}
		text, _ := lockdownLiftLastText(t, env, env.chat.Id)
		unbanning := lockdownText(staffChatTranslator(env.chat.Id), "lockdown_lifted_unbanning", map[string]any{"count": int64(2)})
		if !strings.Contains(unbanning, staffMarker("lockdown_lifted_unbanning")) || !strings.Contains(unbanning, "2") {
			t.Fatalf("setup: the unbanning line %q does not carry its key and the count", unbanning)
		}
		want := "opened by Ad&lt;b&gt;min\n" + staffMarker("lockdown_lifted_manual_change") + "\n" + unbanning
		if text != want {
			t.Fatalf("custom lift reply\n got %q\nwant %q", text, want)
		}
		lockdownGoldenNoQueue(t, env.chat.Id)
	})

	t.Run("command refusal unchanged", func(t *testing.T) {
		env := lockdownEventEnv(t, false)
		eventMsgStore(t, env.chat.Id, eventmsg.EventLockLift, "custom {admin}")

		env.send(env.admin, "/unlockdown")

		text, _ := lockdownEventOnlySend(t, env)
		if text != staffMarker("lockdown_not_active") {
			t.Fatalf("/unlockdown with no lockdown replied %q, want %q", text, staffMarker("lockdown_not_active"))
		}
	})

	t.Run("command silent", func(t *testing.T) {
		env := lockdownEventEnv(t, false)
		row := env.lockAsAdmin("raid")
		lockdownLiftSilence(t, env.chat.Id)

		env.send(env.admin, "/unlockdown")

		lockdownLiftWantLifted(t, env, row, env.admin.Id)
		// Only the /lockdown reply: the lift posts nothing.
		lockdownGoldenWantSends(t, env, env.chat.Id, 1)
		lockdownGoldenNoQueue(t, env.chat.Id)
	})

	t.Run("alert button", func(t *testing.T) {
		env := lockdownEventEnv(t, false)
		card := askToLiftLockdown(t, env)
		eventMsgStore(t, env.chat.Id, eventmsg.EventLockLift, "lifted by {admin} in {chatname}")
		gated := gateSends(env)
		lockdownApplyDrainWake()

		done := make(chan struct{})
		go func() {
			defer close(done)
			pressLockdownButton(env, env.admin, card.staffChat, card.msgID, card.confirm)
		}()
		t.Cleanup(func() {
			gated.open()
			<-done
		})
		select {
		case <-gated.entered:
		case <-time.After(10 * time.Second):
			t.Fatal("the custom lift notice never reached its send")
		}
		if !lockdownApplyDrainWake() {
			t.Fatal("the lockdown worker was not woken before the lift notice was sent (D-23)")
		}
		gated.open()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("the Lift Confirm did not return after the send went through")
		}

		lockdownLiftWantLifted(t, env, card.row, env.admin.Id)
		if edits := env.cardEdits(card.staffChat.Id, card.msgID); len(edits) == 0 ||
			!strings.Contains(edits[len(edits)-1], staffMarker("lockdown_alert_lift_done")) {
			t.Fatalf("card edits = %q, want the card edited to %s", edits, staffMarker("lockdown_alert_lift_done"))
		}
		text, raw := lockdownLiftLastText(t, env, env.chat.Id)
		if want := "lifted by Ad&lt;b&gt;min in " + html.EscapeString(card.row.ChatTitle); text != want {
			t.Fatalf("custom lift notice\n got %q\nwant %q", text, want)
		}
		lockdownLiftWantNewMessage(t, raw)
		lockdownGoldenWantSends(t, env, env.chat.Id, 2)
		lockdownGoldenNoQueue(t, env.chat.Id)
	})

	t.Run("alert refused", func(t *testing.T) {
		env := lockdownEventEnv(t, true)
		card := askToLiftLockdown(t, env)
		eventMsgStore(t, env.chat.Id, eventmsg.EventLockLift, "custom {admin}")
		attemptsBefore := lockdownLiftGroupSends(env, env.chat.Id)
		env.fake.script("sendMessage", env.chat.Id, staffFakeError(400, "Bad Request: the custom message was refused"))

		pressLockdownButton(env, env.admin, card.staffChat, card.msgID, card.confirm)

		lockdownGoldenWantLifted(t, env, card.row)
		if got := lockdownLiftGroupSends(env, env.chat.Id) - attemptsBefore; got != 2 {
			t.Fatalf("sendMessage requests to the group = %d, want the refused custom message and the default", got)
		}
		lockdownGoldenWantSends(t, env, env.chat.Id, 2)
		ids := map[int64]string{env.chat.Id: "<chat>", env.admin.Id: "<admin>", card.staffChat.Id: "<staff>"}
		lockdownGoldenWant(t, "the default after the refusal", lockdownGoldenRequest(t, env, env.chat.Id, 1), ids, goldenLockdownLiftAlert)
		lockdownGoldenNoQueue(t, env.chat.Id)
	})

	t.Run("private menu", func(t *testing.T) {
		env := lockdownLiftSettingsEnv(t, false)
		row := env.lockAsAdmin("raid")
		eventMsgStore(t, env.chat.Id, eventmsg.EventLockLift, "lifted by {admin} in {chatname}")
		env.sendPrivate(env.admin, fmt.Sprintf("/start settings_%d", env.chat.Id))
		privateMsgID := settingsPrivateOnlySent(t, env, env.admin).MessageID
		private := settingsPrivateChat(env.admin)
		env.pressPrivate(env.admin, privateMsgID, env.privateFields(env.chat.Id, "a", "op", "p", settingsPageLockdown))
		_, keyboard := env.lastEdit(env.admin.Id, privateMsgID)
		env.pressRaw(private, env.admin, privateMsgID, lockdownGoldenSettingsButton(t, keyboard, "ls").CallbackData)
		_, card := env.lastEdit(env.admin.Id, privateMsgID)
		editsBefore := len(env.edits(env.admin.Id, privateMsgID))

		env.pressRaw(private, env.admin, privateMsgID, lockdownGoldenSettingsButton(t, card, "ly").CallbackData)

		lockdownLiftWantLifted(t, env.lockdownEnv, row, env.admin.Id)
		lockdownGoldenWantSends(t, env.lockdownEnv, env.chat.Id, 2)
		text, raw := lockdownLiftLastText(t, env.lockdownEnv, env.chat.Id)
		want := "lifted by " + html.EscapeString(staffFullName(&env.admin)) + " in " + html.EscapeString(row.ChatTitle)
		if text != want {
			t.Fatalf("custom lift notice\n got %q\nwant %q", text, want)
		}
		lockdownLiftWantNewMessage(t, raw)
		// The page was redrawn before the notice went out (WR-04).
		if edits := lockdownLiftEditsBeforeNotice(env.lockdownEnv, env.admin.Id, privateMsgID, env.chat.Id); edits <= editsBefore {
			t.Fatalf("%d edits of the menu before the lift notice, want the redraw (more than %d) first", edits, editsBefore)
		}
		lockdownGoldenNoQueue(t, env.chat.Id)
	})

	t.Run("group menu custom", func(t *testing.T) {
		env := lockdownLiftSettingsEnv(t, false)
		row := env.lockAsAdmin("raid")
		eventMsgStore(t, env.chat.Id, eventmsg.EventLockLift, "opened by {admin}")

		msgID, before := lockdownLiftGroupMenuYes(t, env)

		lockdownLiftWantLifted(t, env.lockdownEnv, row, env.admin.Id)
		if finalText, _ := env.lastEdit(env.chat.Id, msgID); !strings.Contains(finalText, staffMarker("lockdown_lifted")) {
			t.Fatalf("redrawn page %q does not carry the lift result as a notice", finalText)
		}
		if !lockdownApplyDrainWake() {
			t.Fatal("the lockdown worker was not woken after a lift from a group menu")
		}
		lockdownGoldenWantSends(t, env.lockdownEnv, env.chat.Id, before+1)
		text, raw := lockdownLiftLastText(t, env.lockdownEnv, env.chat.Id)
		if want := "opened by " + html.EscapeString(staffFullName(&env.admin)); text != want {
			t.Fatalf("custom lift notice\n got %q\nwant %q", text, want)
		}
		lockdownLiftWantNewMessage(t, raw)
		lockdownGoldenNoQueue(t, env.chat.Id)
		lockdownGoldenNoPrivate(t, env.lockdownEnv)
	})

	t.Run("group menu quiet", func(t *testing.T) {
		// Silent: the menu is the only answer.
		silent := lockdownLiftSettingsEnv(t, false)
		row := silent.lockAsAdmin("raid")
		lockdownLiftSilence(t, silent.chat.Id)
		sendsBefore := lockdownLiftGroupSends(silent.lockdownEnv, silent.chat.Id)

		msgID, before := lockdownLiftGroupMenuYes(t, silent)

		lockdownLiftWantLifted(t, silent.lockdownEnv, row, silent.admin.Id)
		if finalText, _ := silent.lastEdit(silent.chat.Id, msgID); !strings.Contains(finalText, staffMarker("lockdown_lifted")) {
			t.Fatalf("redrawn page %q does not carry the lift result as a notice", finalText)
		}
		lockdownGoldenWantSends(t, silent.lockdownEnv, silent.chat.Id, before)
		if got := lockdownLiftGroupSends(silent.lockdownEnv, silent.chat.Id) - sendsBefore; got != 1 {
			t.Fatalf("sendMessage requests to the group = %d, want only the /settings choice message", got)
		}

		// A refused custom message: no default is sent from a group menu (D-22).
		refused := lockdownLiftSettingsEnv(t, false)
		row = refused.lockAsAdmin("raid")
		eventMsgStore(t, refused.chat.Id, eventmsg.EventLockLift, "custom {admin}")
		msgID = settingsLockdownTestOpen(t, refused)
		_, keyboard := refused.lastEdit(refused.chat.Id, msgID)
		refused.pressRaw(refused.chat, refused.admin, msgID, lockdownGoldenSettingsButton(t, keyboard, "ls").CallbackData)
		_, card := refused.lastEdit(refused.chat.Id, msgID)
		before = len(refused.fake.sentTo(refused.chat.Id))
		sendsBefore = lockdownLiftGroupSends(refused.lockdownEnv, refused.chat.Id)
		refused.fake.script("sendMessage", refused.chat.Id, staffFakeError(400, "Bad Request: the custom message was refused"))

		refused.pressRaw(refused.chat, refused.admin, msgID, lockdownGoldenSettingsButton(t, card, "ly").CallbackData)

		lockdownLiftWantLifted(t, refused.lockdownEnv, row, refused.admin.Id)
		if got := lockdownLiftGroupSends(refused.lockdownEnv, refused.chat.Id) - sendsBefore; got != 1 {
			t.Fatalf("sendMessage requests to the group = %d, want only the refused custom message", got)
		}
		lockdownGoldenWantSends(t, refused.lockdownEnv, refused.chat.Id, before)
		lockdownGoldenNoQueue(t, refused.chat.Id)
	})

	t.Run("tally after silent", func(t *testing.T) {
		env := lockdownEventEnv(t, false)
		row := env.lockAsAdmin("raid")
		raider := env.newJoiner("Raider")
		env.join(raider, raider, "https://t.me/+abc")
		env.fake.setMember(env.chat.Id, raider.Id, staffFakeMember{Status: "kicked"})
		env.cycle()
		if got := env.joinerRow(raider.Id); got.State != models.JoinerStateKept {
			t.Fatalf("setup: joiner is %q, want kept", got.State)
		}
		lockdownLiftSilence(t, env.chat.Id)

		env.send(env.admin, "/unlockdown")

		if lifted := env.freshRow(row.ID); lifted.State != models.LockdownStateLifted {
			t.Fatalf("lockdown after /unlockdown = %q, want lifted at once", lifted.State)
		}
		if got := env.repliesWith(staffMarker("lockdown_lifted")); len(got) != 0 {
			t.Fatalf("lift notices = %q, want none: the message is silent", got)
		}
		if got := env.messagesWith(staffMarker("lockdown_lift_tally_kept") + " 1"); got != 1 {
			t.Fatalf("tally messages naming the one kept ban = %d, want 1", got)
		}
		lockdownGoldenNoQueue(t, env.chat.Id)
	})
}
