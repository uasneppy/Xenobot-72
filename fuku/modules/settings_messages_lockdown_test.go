//go:build testtools

package modules

import (
	"fmt"
	"html"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
)

// settingsMsgWantLockdownStates fails the test unless every lockdown event has its line in text with its icon and the
// expected state word (events not in want read Default).
func settingsMsgWantLockdownStates(t *testing.T, text string, want map[string]string) {
	t.Helper()
	for _, ev := range settingsMsgLockdownEvents {
		word := "settings_state_default"
		if w, ok := want[ev.Key]; ok {
			word = w
		}
		line, ok := settingsMainStateLine(text, ev.NameKey)
		if !ok {
			t.Errorf("no line for %s in %q", ev.Key, text)
			continue
		}
		if !strings.Contains(line, ev.Icon) || !strings.Contains(line, staffMarker(word)) {
			t.Errorf("%s line = %q, want the icon %s and %s", ev.Key, line, ev.Icon, word)
		}
	}
}

// TestSettingsMessagesLockdownFamily drives the Lockdown family (Phase 25, plan 25-03) through the real private menu: its
// section on the hub and its list, the five event screens (three parts, Silent, no private copy row, the reserve of the
// technical notes), a forged private copy press, a typed text and a Silent press that the next /lockdown reads at once,
// and the previews filled with the trigger samples and the notes.
func TestSettingsMessagesLockdownFamily(t *testing.T) {
	t.Run("hub and list", func(t *testing.T) {
		env := newSettingsMsgEnv(t)
		m, text, keyboard := settingsMsgOpenPrivate(env, env.admin)
		heading := "🚨 <b>" + staffMarker("settings_msg_fam_lockdown") + "</b>"
		mreq := strings.Index(text, "🛂 <b>"+staffMarker("settings_msg_fam_mreq")+"</b>")
		lockdown := strings.Index(text, heading)
		pm := strings.Index(text, settingsMsgPMIcon+" <b>"+staffMarker("settings_msg_pm_title")+"</b>")
		if mreq < 0 || lockdown < mreq || pm < lockdown {
			t.Errorf("the hub %q does not draw the bold 🚨 Lockdown section after the Member requirements one and before the private copies", text)
		}
		settingsMsgWantLockdownStates(t, text, nil)
		settingsMsgWantWidths(t, "the hub", keyboard, 2, 2, 1, 1, 2)

		cell, ok := settingsMsgButtonWith(t, keyboard, "a", settingsActMsgFamily, "t", "l")
		if !ok || !strings.HasPrefix(cell.Text, "🚨 ") || !containsMarker(cell.Text, "settings_msg_fam_lockdown") ||
			len(keyboard[1]) != 2 || keyboard[1][1].CallbackData != cell.CallbackData {
			t.Fatalf("the hub has no 🚨 Lockdown cell second on its row: %+v", keyboard)
		}
		list, listKeyboard := m.pressButton(cell)
		if !containsMarker(list, "settings_msg_fam_lockdown_intro") || !containsMarker(list, "settings_msg_list_hint") {
			t.Errorf("the Lockdown list %q lacks its intro or hint", list)
		}
		settingsMsgWantLockdownStates(t, list, nil)
		settingsMsgWantWidths(t, "the Lockdown list", listKeyboard, 2, 2, 1, 2)
		for _, ev := range settingsMsgLockdownEvents {
			button, ok := settingsMsgButtonWith(t, listKeyboard, "a", settingsActMsgEvent, settingsMsgEventField, ev.Code)
			if !ok || !strings.HasPrefix(button.Text, ev.Icon+" ") || !containsMarker(button.Text, ev.NameKey) {
				t.Errorf("the Lockdown list has no %s %s cell: %+v", ev.Icon, ev.Code, listKeyboard)
			}
		}
		if back := settingsButtonFields(t, listKeyboard[3][0]); back["a"] != settingsActOpen || back["p"] != settingsPageMessages {
			t.Errorf("the Lockdown list's Back carries %v, want the hub", back)
		}
	})

	t.Run("screens", func(t *testing.T) {
		env := newSettingsMsgEnv(t)
		listEditAdmin(env.settingsEnv, env.chat, env.admin, true)
		m := settingsMsgPrivateMenu(env, env.chat, env.admin)

		for _, ev := range settingsMsgLockdownEvents {
			text, keyboard := m.press("a", settingsActMsgEvent, settingsMsgEventField, ev.Code)
			settingsMsgWantWidths(t, "the empty "+ev.Code+" screen", keyboard, 1, 1, 1, 1, 1, 2)
			for _, part := range []string{settingsEditPartText, settingsEditPartMedia, settingsEditPartButtons} {
				settingsMsgPartButton(t, keyboard, settingsActMsgSet, part)
			}
			if _, ok := settingsMsgButtonWith(t, keyboard, "a", settingsActMsgSilent, settingsMsgEventField, ev.Code); !ok {
				t.Errorf("the %s screen has no Silent switch: %+v", ev.Code, keyboard)
			}
			if _, ok := settingsMsgButtonWith(t, keyboard, "a", settingsActMsgEvent, settingsMsgDestField, settingsMsgDestPMCode); ok {
				t.Errorf("the %s screen has a private copy row: %+v", ev.Code, keyboard)
			}
			for _, key := range []string{ev.NameKey, ev.DescKey, "settings_msg_hint_lockdown", "settings_msg_line_silent", "settings_edit_line_media"} {
				if !containsMarker(text, key) {
					t.Errorf("the %s screen %q lacks %s", ev.Code, text, key)
				}
			}
			if containsMarker(text, "settings_msg_line_pm") {
				t.Errorf("the %s screen %q carries the private copy line", ev.Code, text)
			}
		}

		// Set text on the join surge draws the family's prompt with 4096 less the notes' reserve; Cancel ends the wait.
		_, keyboard := m.press("a", settingsActMsgEvent, settingsMsgEventField, "lj")
		prompt, promptKeyboard := m.pressButton(settingsMsgPartButton(t, keyboard, settingsActMsgSet, settingsEditPartText))
		if wait := settingsEditWaitOf(env.settingsEnv, env.admin); wait["p"] != "hljg" || wait["t"] != settingsEditPartText {
			t.Errorf("Set text on the join surge wrote the wait %v, want page hljg and part x", wait)
		}
		const want = settingsEditTextMax - lockdownNoteReserve
		if want != 3696 || !containsMarker(prompt, "settings_msg_wait_text_lockdown") || !settingsEditPromptHas(prompt, want) {
			t.Errorf("the lockdown text prompt %q lacks the family's prompt or the limit %d", prompt, want)
		}
		m.pressButton(promptKeyboard[0][0])
		settingsEditWaitGone(t, env.settingsEnv, env.admin)
		def, ok := settingsEditPageByKey("hljg")
		if !ok {
			t.Fatal("no editor definition for the join surge")
		}
		withMedia := settingsEditState{Kind: settingsEditEvent, TextIsDefault: true, FileID: "f", MediaType: db.PHOTO}
		if got := settingsEditPromptMax(def, withMedia); got != 624 {
			t.Errorf("with a media item stored the join surge's prompt shows %d, want 624", got)
		}

		// A forged private copy press on a lockdown event is an invalid request and writes nothing.
		for _, forged := range [][]string{
			{"a", settingsActMsgEvent, settingsMsgEventField, "ld", settingsMsgDestField, settingsMsgDestPMCode},
			{"a", settingsActMsgPM, settingsMsgEventField, "lu", "v", "1"},
		} {
			env.pressPrivate(env.admin, m.msgID, env.privateFields(env.chat.Id, forged...))
			if answer, alert := env.lastAnswer(); !alert || !containsMarker(answer, "common_callback_invalid_request") {
				t.Errorf("the press %v answered %q (alert %v), want the invalid request alert", forged, answer, alert)
			}
		}
		if n := settingsMsgRows(t, env.chat.Id); n != 0 {
			t.Errorf("the forged presses left %d event message rows, want none", n)
		}
	})

	t.Run("typed and silent", func(t *testing.T) {
		env := newSettingsMsgEnv(t)
		// The owner is the live creator: can_change_info for the page, and the restrict right /lockdown asks for.
		owner := env.newOwner(env.chat)
		env.fake.setBotMember(env.chat.Id, lockdownBotAdminJSON(true, true, true))

		settingsMsgSetWait(t, env, owner, "ld", settingsEditPartText)
		sendPrivateMessage(env.settingsEnv, owner, gotgbot.Message{Text: "Locked down: {reason}"})
		if row, ok := settingsMsgStored(t, env.chat.Id, eventmsg.EventLockdown); !ok || row.Text != "Locked down: {reason}" {
			t.Fatalf("the lockdown row is %+v (row %v), want the typed text", row, ok)
		}

		m := settingsMsgPrivateMenu(env, env.chat, owner)
		_, keyboard := m.press("a", settingsActMsgEvent, settingsMsgEventField, "lu")
		m.pressButton(settingsMsgSilentToggleOf(t, keyboard, "lu"))
		if silent, by, ok := settingsMsgSilentStored(t, env.chat.Id, eventmsg.EventLockLift); !ok || !silent || by != owner.Id {
			t.Errorf("after the press the locklift row reads Silent %v by %d (row %v), want Silent by %d", silent, by, ok, owner.Id)
		}
		_, hub, _ := settingsMsgOpenPrivate(env, owner)
		settingsMsgWantLockdownStates(t, hub, map[string]string{
			eventmsg.EventLockdown: "settings_msg_state_custom", eventmsg.EventLockLift: "settings_msg_state_silent",
		})

		// The next /lockdown replies with the page's text, filled, then the approved-muted note.
		env.send(owner, "/lockdown raid & co")
		lockdownEventWantStarted(t, env.lockdownEnv, owner.Id)
		sent := env.fake.sentTo(env.chat.Id)
		if len(sent) != 1 {
			t.Fatalf("the group received %d messages after /lockdown, want the one reply", len(sent))
		}
		if got, want := fmt.Sprint(sent[0].Params["text"]), "Locked down: raid &amp; co\n"+staffMarker("lockdown_note_approved_muted"); got != want {
			t.Errorf("the /lockdown reply = %q, want %q", got, want)
		}
	})

	t.Run("previews", func(t *testing.T) {
		env := &settingsMsgEnv{settingsEnv: newSettingsEnvWith(t, settingsEnvOptions{RealLocale: true})}
		settingsMsgCleanup(t, env.chat.Id)
		listEditAdmin(env.settingsEnv, env.chat, env.admin, true)
		settingsEditSetLanguages(t, env.admin.Id, "en", env.chat.Id, "en")
		admin := env.admin
		tr := i18n.MustNewTranslator("en")
		groupMark := settingsEditMarkSends(env.settingsEnv, env.chat.Id)
		fullPreview := func(code string) string {
			t.Helper()
			mark := settingsEditMarkSends(env.settingsEnv, admin.Id)
			env.pressPrivate(admin, env.updateID(), env.privateFields(env.chat.Id, "a", settingsActMsgFull, settingsMsgEventField, code))
			sent := settingsEditNewSends(env.settingsEnv, admin.Id, mark, false)
			if len(sent) != 2 || sent[1].Method != "sendMessage" {
				t.Fatalf("Full preview of %s sent %+v, want the header and one sendMessage", code, sent)
			}
			params := rulesCallParams(t, sent[1])
			if keyboard := settingsMsgSentKeyboard(t, params); len(keyboard) != 0 {
				t.Errorf("Full preview of %s carries %+v, want no keyboard", code, keyboard)
			}
			return fmt.Sprint(params["text"])
		}

		// A typed flood text, then its Full preview: the sample numbers, the flood's trigger sentence as {reason}, then the
		// approved-muted note (the sample bot holds both rights).
		settingsMsgSetWait(t, env, admin, "lf", settingsEditPartText)
		sendPrivateMessage(env.settingsEnv, admin, gotgbot.Message{Text: "{count} {window} {percent}: {reason}"})
		sentence := lockdownTriggerSentence(tr, models.LockdownTriggerFlood, 30, 30, 80)
		approved, _ := tr.GetString("lockdown_note_approved_muted")
		if sentence == "" || approved == "" {
			t.Fatal("the flood's trigger sentence or the approved-muted note is empty in English")
		}
		if got, want := fullPreview("lf"), "30 30 80: "+html.EscapeString(sentence)+"\n"+approved; got != want {
			t.Errorf("Full preview of the flood =\n %q\nwant\n %q", got, want)
		}

		// The lift with nothing stored: the default naming the presser, with no note.
		want, _ := lockdownLiftDefaultReply(tr, eventDefaultInput{Target: admin})
		got := fullPreview("lu")
		if got != want || !strings.Contains(got, fmt.Sprintf("tg://user?id=%d", admin.Id)) {
			t.Errorf("Full preview of the default lift = %q, want %q naming the presser", got, want)
		}
		manual, _ := tr.GetString("lockdown_lifted_manual_change")
		if strings.Contains(got, approved) || strings.Contains(got, manual) || strings.Contains(got, "\n") {
			t.Errorf("Full preview of the default lift %q carries a note", got)
		}

		if calls := settingsEditNewSends(env.settingsEnv, env.chat.Id, groupMark, false); len(calls) != 0 {
			t.Errorf("the previews sent %d messages into the group, want none: %+v", len(calls), calls)
		}
	})
}

// settingsMsgSilentToggleOf returns the Silent toggle of the event code's screen, failing the test when there is none.
func settingsMsgSilentToggleOf(t *testing.T, keyboard [][]gotgbot.InlineKeyboardButton, code string) gotgbot.InlineKeyboardButton {
	t.Helper()
	button, ok := settingsMsgButtonWith(t, keyboard, "a", settingsActMsgSilent, settingsMsgEventField, code)
	if !ok {
		t.Fatalf("the %s screen has no Silent toggle: %+v", code, keyboard)
	}
	return button
}

// TestSettingsAuthorityMessagesLockdown holds MSG-08 for the Lockdown family (Phase 25, D-31): it has no command, so this test
// and TestSettingsDriftMessagesLockdown stand in for a parity test. A typist demoted between Set on the lift and the typed
// message stores nothing; without can_change_info an administrator opens the list, the manual start's screen and its Full
// preview, while Set of the join surge's text, Remove of a stored flood text and Silent of the AI burst are each the
// settings_no_change_info alert with nothing edited, no wait and every row unchanged; an administrator the owner gate
// refuses is refused at the press; the live creator's Set and typed text store the row.
func TestSettingsAuthorityMessagesLockdown(t *testing.T) {
	resetSettingsOwnerMemo(t)
	env := newSettingsMsgEnv(t)
	admin := env.admin

	listEditAdmin(env.settingsEnv, env.chat, admin, true)
	settingsMsgSetWait(t, env, admin, "lu", settingsEditPartText)
	listEditAdmin(env.settingsEnv, env.chat, admin, false)
	before := settingsEditSentCount(env.settingsEnv, admin)
	sendPrivateMessage(env.settingsEnv, admin, gotgbot.Message{Text: "Typed after the demotion {admin}"})
	settingsEditWantReply(t, env.settingsEnv, admin, before, "settings_no_change_info", "settings_edit_wait_stopped")
	settingsEditWaitGone(t, env.settingsEnv, admin)
	if row, ok := settingsMsgStored(t, env.chat.Id, eventmsg.EventLockLift); ok {
		t.Errorf("the typed text of a demoted typist was stored: %+v", row)
	}

	// Viewing and previewing need no can_change_info.
	m := settingsMsgPrivateMenu(env, env.chat, admin)
	for _, view := range []struct {
		kv  []string
		key string
	}{
		{[]string{"a", settingsActMsgFamily, "t", "l"}, "settings_msg_fam_lockdown_intro"},
		{[]string{"a", settingsActMsgEvent, settingsMsgEventField, "ld"}, "settings_msg_ev_lockdown"},
	} {
		if text, _ := m.press(view.kv...); !containsMarker(text, view.key) {
			t.Errorf("%v did not open for an administrator without can_change_info: %q", view.kv, text)
		}
	}
	sent := len(env.fake.sentTo(admin.Id))
	m.press("a", settingsActMsgFull, settingsMsgEventField, "ld")
	if got := len(env.fake.sentTo(admin.Id)) - sent; got < 2 {
		t.Errorf("the manual start's Full preview sent %d messages to an administrator without can_change_info, want the header and the message", got)
	}

	// Every change is refused: the stored flood text gives Remove something to remove.
	settingsMsgSeed(t, env.chat.Id, eventmsg.EventLockFlood, "Stored flood text", false)
	rows := settingsMsgAllRows(t, env.chat.Id)
	refused := func(user gotgbot.User, key string, kv ...string) {
		t.Helper()
		msgID := env.updateID()
		env.pressPrivate(user, msgID, env.privateFields(env.chat.Id, kv...))
		if answer, alert := env.lastAnswer(); !alert || !containsMarker(answer, key) {
			t.Errorf("%v answered %q (alert %v), want the %s alert", kv, answer, alert, key)
		}
		if got := len(env.edits(user.Id, msgID)); got != 0 {
			t.Errorf("the refused %v made %d edits, want none", kv, got)
		}
		if env.redis.Exists(settingsInputKey(user.Id)) {
			t.Errorf("the refused %v left a wait", kv)
		}
	}
	for _, kv := range [][]string{
		{"a", settingsActMsgSet, "t", settingsEditPartText, settingsMsgEventField, "lj"},
		{"a", settingsActMsgRemove, "t", settingsEditPartText, settingsMsgEventField, "lf"},
		{"a", settingsActMsgSilent, settingsMsgEventField, "la", "v", "1"},
	} {
		refused(admin, "settings_no_change_info", kv...)
	}

	// With can_change_info back, an administrator the owner gate refuses is refused at the press.
	owner := env.newOwner(env.chat)
	listEditAdmin(env.settingsEnv, env.chat, admin, true)
	env.setAdminsAllowed(env.chat.Id, false)
	refused(admin, "settings_gate_owner_only", "a", settingsActMsgSet, "t", settingsEditPartText, settingsMsgEventField, "lj")

	after := settingsMsgAllRows(t, env.chat.Id)
	if len(after) != len(rows) {
		t.Errorf("the refused presses changed the rows from %d to %d", len(rows), len(after))
	}
	for key, row := range rows {
		got, ok := after[key]
		if !ok || got.Text != row.Text || got.Silent != row.Silent || !got.UpdatedAt.Equal(row.UpdatedAt) {
			t.Errorf("the refused presses changed the %s row from %+v to %+v", key, row, got)
		}
	}

	// The live creator sets and types the join surge's text.
	settingsMsgSetWait(t, env, owner, "lj", settingsEditPartText)
	sendPrivateMessage(env.settingsEnv, owner, gotgbot.Message{Text: "Raid of {count}"})
	if row, ok := settingsMsgStored(t, env.chat.Id, eventmsg.EventLockJoinSurge); !ok || row.Text != "Raid of {count}" || row.UpdatedBy != owner.Id {
		t.Errorf("the creator's typed text left %+v (row %v), want the text stored by %d", row, ok, owner.Id)
	}
}

// TestSettingsDriftMessagesLockdown holds that the page and the lockdown sites read one store (Phase 25, D-31): a lockdown
// text stored through the repository is drawn by the manual start's screen and counted Custom on the hub; a typed text and a
// Silent press on the page are what the send path's cached read returns at once, with no cache reset; the next /lockdown
// posts the typed text followed by the approved-muted note; and opening every lockdown screen writes nothing.
func TestSettingsDriftMessagesLockdown(t *testing.T) {
	env := newSettingsMsgEnv(t)
	// The owner is the live creator: can_change_info for the page, and the restrict right /lockdown asks for.
	owner := env.newOwner(env.chat)
	env.fake.setBotMember(env.chat.Id, lockdownBotAdminJSON(true, true, true))
	m := settingsMsgPrivateMenu(env, env.chat, owner)

	const stored = "Stored {reason}"
	settingsMsgSeed(t, env.chat.Id, eventmsg.EventLockdown, stored, false)
	text, _ := m.press("a", settingsActMsgEvent, settingsMsgEventField, "ld")
	if want := staffMarker("settings_edit_state_text_len") + " " + fmt.Sprint(len(stored)); !strings.Contains(text, want) {
		t.Errorf("the manual start's screen %q lacks the stored text's length", text)
	}
	hub, _ := m.press("a", settingsActOpen, "p", settingsPageMessages)
	settingsMsgWantLockdownStates(t, hub, map[string]string{eventmsg.EventLockdown: "settings_msg_state_custom"})

	// A typed text is what the send path reads at once, after it had cached the stored one.
	if got := settingsMsgSendPathText(t, env.chat.Id, eventmsg.EventLockdown); got != stored {
		t.Fatalf("the send path reads %q before the typed text, want the stored one", got)
	}
	settingsMsgSetWait(t, env, owner, "ld", settingsEditPartText)
	sendPrivateMessage(env.settingsEnv, owner, gotgbot.Message{Text: "Typed: {reason}"})
	if got := settingsMsgSendPathText(t, env.chat.Id, eventmsg.EventLockdown); got != "Typed: {reason}" {
		t.Errorf("after the typed save the send path reads %q, want the new text", got)
	}

	// A Silent press is what the send path reads at once.
	if settingsMsgSendPathSilent(t, env.chat.Id, eventmsg.EventLockAIBurst) {
		t.Fatal("the send path reads the AI burst's message as silent before any press")
	}
	m.press("a", settingsActMsgSilent, settingsMsgEventField, "la", "v", "1")
	if !settingsMsgSendPathSilent(t, env.chat.Id, eventmsg.EventLockAIBurst) {
		t.Error("after the page turned Silent on, the send path still reads the AI burst's message as not silent")
	}

	// The next /lockdown replies with the typed text, filled, then the approved-muted note.
	env.send(owner, "/lockdown raid & co")
	lockdownEventWantStarted(t, env.lockdownEnv, owner.Id)
	sent := env.fake.sentTo(env.chat.Id)
	if len(sent) != 1 {
		t.Fatalf("the group received %d messages after /lockdown, want the one reply", len(sent))
	}
	if got, want := fmt.Sprint(sent[0].Params["text"]), "Typed: raid &amp; co\n"+staffMarker("lockdown_note_approved_muted"); got != want {
		t.Errorf("the /lockdown reply = %q, want %q", got, want)
	}

	// Opening every lockdown screen of another chat writes nothing.
	other := env.addChat()
	o := settingsMsgPrivateMenu(env, other, env.admin)
	o.press("a", settingsActOpen, "p", settingsPageMessages)
	o.press("a", settingsActMsgFamily, "t", "l")
	for _, ev := range settingsMsgLockdownEvents {
		if text, _ := o.press("a", settingsActMsgEvent, settingsMsgEventField, ev.Code); !containsMarker(text, ev.NameKey) {
			t.Errorf("the %s screen of another chat did not open: %q", ev.Code, text)
		}
	}
	if n := settingsMsgRows(t, other.Id); n != 0 {
		t.Errorf("opening every lockdown screen wrote %d event message rows, want none", n)
	}
}
