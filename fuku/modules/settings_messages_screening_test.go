//go:build testtools

package modules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
)

// settingsMsgReqEvent is the Member requirements family's event with the code, failing the test when there is none.
func settingsMsgReqEvent(t *testing.T, code string) (settingsMsgFamily, settingsMsgEvent) {
	t.Helper()
	fam, ev, ok := settingsMsgEventByCode(code)
	if !ok || fam.Code != "r" {
		t.Fatalf("no Member requirements event %q (family %q)", code, fam.Code)
	}
	return fam, ev
}

// settingsMsgWantReqStates fails the test unless every event of the Member requirements family has its line in text with
// its icon and the Default word.
func settingsMsgWantReqStates(t *testing.T, text string) {
	t.Helper()
	for _, ev := range settingsMsgReqEvents {
		line, ok := settingsMainStateLine(text, ev.NameKey)
		if !ok {
			t.Errorf("no line for %s in %q", ev.Key, text)
			continue
		}
		if !strings.Contains(line, ev.Icon) || !strings.Contains(line, staffMarker("settings_state_default")) {
			t.Errorf("%s line = %q, want the icon %s and Default", ev.Key, line, ev.Icon)
		}
	}
}

// TestSettingsMessagesScreeningFamily drives the Phase 24 events (plan 24-04) through the real private menu: the failed
// captcha and the AI spam notice in the Automatic family, the Member requirements family's four events, the AI spam
// notice's Notice switch (off by default) and the previews filled with the sample values.
func TestSettingsMessagesScreeningFamily(t *testing.T) {
	t.Run("hub and lists", func(t *testing.T) {
		env := newSettingsMsgEnv(t)
		m, text, keyboard := settingsMsgOpenPrivate(env, env.admin)
		if !strings.Contains(text, "🛂 <b>"+staffMarker("settings_msg_fam_mreq")+"</b>") {
			t.Errorf("the hub %q has no bold 🛂 Member requirements heading", text)
		}
		settingsMsgWantAutoStates(t, text, nil)
		settingsMsgWantReqStates(t, text)
		settingsMsgWantWidths(t, "the hub", keyboard, 2, 1, 1, 1, 2)

		cell, ok := settingsMsgButtonWith(t, keyboard, "a", settingsActMsgFamily, "t", "r")
		if !ok || !strings.HasPrefix(cell.Text, "🛂 ") || !containsMarker(cell.Text, "settings_msg_fam_mreq") || len(keyboard[1]) != 1 {
			t.Fatalf("the hub has no 🛂 Member requirements cell alone on its row: %+v", keyboard)
		}
		list, listKeyboard := m.pressButton(cell)
		if !containsMarker(list, "settings_msg_fam_mreq_intro") {
			t.Errorf("the Member requirements list %q lacks its intro", list)
		}
		settingsMsgWantReqStates(t, list)
		settingsMsgWantWidths(t, "the Member requirements list", listKeyboard, 2, 2, 2)
		for _, ev := range settingsMsgReqEvents {
			button, ok := settingsMsgButtonWith(t, listKeyboard, "a", settingsActMsgEvent, settingsMsgEventField, ev.Code)
			if !ok || !strings.HasPrefix(button.Text, ev.Icon+" ") || !containsMarker(button.Text, ev.NameKey) {
				t.Errorf("the Member requirements list has no %s %s cell: %+v", ev.Icon, ev.Code, listKeyboard)
			}
		}

		auto, ok := settingsMsgButtonWith(t, keyboard, "a", settingsActMsgFamily, "t", "a")
		if !ok {
			t.Fatalf("the hub has no Automatic actions cell: %+v", keyboard)
		}
		_, autoKeyboard := m.pressButton(auto)
		settingsMsgWantWidths(t, "the Automatic list", autoKeyboard, 2, 2, 2, 1, 2)
		for _, code := range []string{"cf", "ai"} {
			if _, ok := settingsMsgButtonWith(t, autoKeyboard, "a", settingsActMsgEvent, settingsMsgEventField, code); !ok {
				t.Errorf("the Automatic list has no %s cell: %+v", code, autoKeyboard)
			}
		}
	})

	t.Run("requirement screens", func(t *testing.T) {
		env := newSettingsMsgEnv(t)
		listEditAdmin(env.settingsEnv, env.chat, env.admin, true)
		m := settingsMsgPrivateMenu(env, env.chat, env.admin)

		// The mute: text and buttons only, no Silent switch, its private copy row and the family's hint.
		text, keyboard := m.press("a", settingsActMsgEvent, settingsMsgEventField, "rm")
		settingsMsgWantWidths(t, "the empty requirement mute screen", keyboard, 1, 1, 1, 1, 2)
		for _, part := range []string{settingsEditPartText, settingsEditPartButtons} {
			settingsMsgPartButton(t, keyboard, settingsActMsgSet, part)
		}
		if _, ok := settingsMsgButtonWith(t, keyboard, "a", settingsActMsgSet, "t", settingsEditPartMedia); ok {
			t.Errorf("the requirement mute screen offers a media part: %+v", keyboard)
		}
		if _, ok := settingsMsgButtonWith(t, keyboard, "a", settingsActMsgSilent); ok {
			t.Errorf("the requirement mute screen has a Silent switch: %+v", keyboard)
		}
		if _, ok := settingsMsgButtonWith(t, keyboard, "a", settingsActMsgEvent, settingsMsgDestField, settingsMsgDestPMCode); !ok {
			t.Errorf("the requirement mute screen has no private copy row: %+v", keyboard)
		}
		for _, key := range []string{"settings_msg_ev_mreqmute", "settings_msg_desc_mreqmute", "settings_msg_hint_mreq"} {
			if !containsMarker(text, key) {
				t.Errorf("the requirement mute screen %q lacks %s", text, key)
			}
		}
		for _, key := range []string{"settings_msg_line_silent", "settings_msg_hint_silent", "settings_edit_line_media"} {
			if containsMarker(text, key) {
				t.Errorf("the requirement mute screen %q carries %s", text, key)
			}
		}

		// Set text draws the family's prompt with the limit less the bot's reserve; Cancel ends the wait.
		prompt, promptKeyboard := m.pressButton(settingsMsgPartButton(t, keyboard, settingsActMsgSet, settingsEditPartText))
		if wait := settingsEditWaitOf(env.settingsEnv, env.admin); wait["p"] != "hrmg" || wait["t"] != settingsEditPartText {
			t.Errorf("Set text on the requirement mute wrote the wait %v, want page hrmg and part x", wait)
		}
		const want = settingsEditTextMax - memberReqNoticeNoteReserve
		if !containsMarker(prompt, "settings_msg_wait_text_mreq") || !settingsEditPromptHas(prompt, want) {
			t.Errorf("the requirement text prompt %q lacks the family's prompt or the limit %d", prompt, want)
		}
		m.pressButton(promptKeyboard[0][0])
		settingsEditWaitGone(t, env.settingsEnv, env.admin)

		// A forged hq naming the mute is an invalid request and writes nothing.
		env.pressPrivate(env.admin, m.msgID, env.privateFields(env.chat.Id, "a", settingsActMsgSilent, settingsMsgEventField, "rm", "v", "1"))
		if answer, alert := env.lastAnswer(); !alert || !containsMarker(answer, "common_callback_invalid_request") {
			t.Errorf("a Silent press naming the requirement mute answered %q (alert %v), want the invalid request alert", answer, alert)
		}
		if n := settingsMsgRows(t, env.chat.Id); n != 0 {
			t.Errorf("the forged press left %d event message rows, want none", n)
		}

		// The warn keeps its Silent switch.
		_, keyboard = m.press("a", settingsActMsgEvent, settingsMsgEventField, "rw")
		settingsMsgWantWidths(t, "the empty requirement warn screen", keyboard, 1, 1, 1, 1, 1, 2)
		if _, ok := settingsMsgButtonWith(t, keyboard, "a", settingsActMsgSilent, settingsMsgEventField, "rw"); !ok {
			t.Errorf("the requirement warn screen has no Silent switch: %+v", keyboard)
		}
	})

	t.Run("ai notice", func(t *testing.T) {
		env := newSettingsMsgEnv(t)
		listEditAdmin(env.settingsEnv, env.chat, env.admin, true)
		m := settingsMsgPrivateMenu(env, env.chat, env.admin)
		mainLine := func() string {
			t.Helper()
			text, _ := m.press("a", settingsActBack)
			line, ok := settingsMainStateLine(text, "settings_page_msgs")
			if !ok {
				t.Fatalf("Main has no Messages line: %q", text)
			}
			return line
		}
		if line := mainLine(); !containsMarker(line, "settings_state_default") {
			t.Errorf("Main's Messages line with the AI spam notice off = %q, want Default", line)
		}

		text, keyboard := m.press("a", settingsActMsgEvent, settingsMsgEventField, "ai")
		if want := staffMarker("settings_msg_line_notice") + " ❌ " + staffMarker("common_status_off"); !strings.Contains(text, want) {
			t.Errorf("the AI spam screen %q lacks the Notice line %q", text, want)
		}
		if !containsMarker(text, "settings_msg_hint_notice") || containsMarker(text, "settings_msg_line_silent") {
			t.Errorf("the AI spam screen %q does not draw the Notice hint in place of Silent", text)
		}
		toggle, ok := settingsMsgButtonWith(t, keyboard, "a", settingsActMsgSilent, settingsMsgEventField, "ai")
		if fields := settingsButtonFields(t, toggle); !ok || toggle.Text != staffMarker("settings_btn_msg_notice")+" ❌" || fields["v"] != "0" {
			t.Fatalf("the AI spam screen's Notice toggle is %q with %v, want it reading Off with v=0", toggle.Text, fields)
		}

		text, keyboard = m.pressButton(toggle)
		if silent, by, ok := settingsMsgSilentStored(t, env.chat.Id, eventmsg.EventAISpam); !ok || silent || by != env.admin.Id {
			t.Errorf("after the press the aispam row reads Silent %v by %d (row %v), want the notice on by %d", silent, by, ok, env.admin.Id)
		}
		if want := staffMarker("settings_msg_line_notice") + " ✅ " + staffMarker("common_status_on"); !strings.Contains(text, want) {
			t.Errorf("the redrawn AI spam screen %q lacks %q", text, want)
		}
		_, hub, _ := settingsMsgOpenPrivate(env, env.admin)
		settingsMsgWantAutoStates(t, hub, map[string]string{eventmsg.EventAISpam: "settings_state_default"})
		if line, want := mainLine(), staffMarker("settings_state_msg_n")+" 1"; !strings.Contains(line, want) {
			t.Errorf("Main's Messages line with the AI spam notice on = %q, want %q", line, want)
		}

		toggle, ok = settingsMsgButtonWith(t, keyboard, "a", settingsActMsgSilent, settingsMsgEventField, "ai")
		if fields := settingsButtonFields(t, toggle); !ok || fields["v"] != "1" {
			t.Fatalf("the redrawn Notice toggle carries %v, want v=1", fields)
		}
		m.pressButton(toggle)
		if silent, _, ok := settingsMsgSilentStored(t, env.chat.Id, eventmsg.EventAISpam); !ok || !silent {
			t.Errorf("after the second press the aispam row reads Silent %v (row %v), want the notice off", silent, ok)
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
		fullPreview := func(code string) map[string]any {
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
			return params
		}

		// A typed requirement mute text, then its Full preview: the text with the presser's mention and the sample rules,
		// then the requirements line naming both rules and the photo remedy.
		settingsMsgSetWait(t, env, admin, "rm", settingsEditPartText)
		sendPrivateMessage(env.settingsEnv, admin, gotgbot.Message{
			Text: "{mention}: {reason}", Entities: []gotgbot.MessageEntity{{Type: "bold", Offset: 0, Length: 9}},
		})
		if row, ok := settingsMsgStored(t, env.chat.Id, eventmsg.EventReqMute); !ok || row.Text != "<b>{mention}</b>: {reason}" {
			t.Fatalf("the requirement mute row is %+v (row %v), want the typed text", row, ok)
		}
		_, ev := settingsMsgReqEvent(t, "rm")
		rules := ev.Sample.Rules
		wantSuffix := "</b>: " + memberReqRulesText(tr, rules) + "\n" + memberReqNoticeNote(tr, rules, "mreq_notice_mute")
		got := fmt.Sprint(fullPreview("rm")["text"])
		if !strings.HasPrefix(got, fmt.Sprintf("<b><a href=\"tg://user?id=%d\">", admin.Id)) || !strings.HasSuffix(got, wantSuffix) {
			t.Errorf("Full preview of the requirement mute = %q, want the mention, then %q", got, wantSuffix)
		}
		photoRemedy, _ := tr.GetString("mreq_remedy_photo")
		if photoRemedy == "" || !strings.Contains(got, photoRemedy) || strings.Count(got, memberReqRulesText(tr, rules)) != 2 {
			t.Errorf("Full preview of the requirement mute %q lacks the photo remedy or the rules in the text and the requirements line", got)
		}

		// The failed captcha with nothing stored: the default with the kick word and 2 removed messages.
		want := buildCaptchaFailureMessage(tr, "kick", admin.Id, admin.FirstName, 2)
		if got := fmt.Sprint(fullPreview("cf")["text"]); got != want {
			t.Errorf("Full preview of the default failed captcha = %q, want %q", got, want)
		}

		if calls := settingsEditNewSends(env.settingsEnv, env.chat.Id, groupMark, false); len(calls) != 0 {
			t.Errorf("the previews sent %d messages into the group, want none: %+v", len(calls), calls)
		}
	})
}

// TestSettingsEditReservedText proves the editor's text reserve (plan 24-04, D-29) purely: a Member requirements event's
// group text is held to 4096 less memberReqNoticeNoteReserve, its prompt says so, and a definition with no reserve (the
// Automatic warn, and the requirement mute's private copy, which appends nothing) keeps the full limit.
func TestSettingsEditReservedText(t *testing.T) {
	withRealLocales(t)
	_, mute := settingsMsgReqEvent(t, "rm")
	reqDef, ok := settingsEditPageByKey(settingsMsgWaitPage(mute))
	if !ok {
		t.Fatal("no editor definition for the requirement mute")
	}
	pmDef, ok := settingsEditPageByKey(settingsMsgPMWaitPage(mute))
	if !ok {
		t.Fatal("no private copy definition for the requirement mute")
	}
	_, warn, ok := settingsMsgEventByCode("wn")
	if !ok {
		t.Fatal("no warn event")
	}
	warnDef, ok := settingsEditPageByKey(settingsMsgWaitPage(warn))
	if !ok {
		t.Fatal("no editor definition for the warn")
	}
	empty := settingsEditState{Kind: settingsEditEvent, TextIsDefault: true}
	check := func(def settingsEditPageDef, n int) settingsEditVerdict {
		return settingsEditCheck(settingsEditCheckInput{
			Def: def, Part: settingsEditPartText, Msg: settingsEditTypedMessage(strings.Repeat("a", n)), State: empty,
			Owner: gotgbot.User{Id: 1, FirstName: "Al"}, GroupTitle: "Group", Language: "en",
		})
	}

	limit := settingsEditTextMax - memberReqNoticeNoteReserve
	if limit != 3496 || reqDef.ReservedText != memberReqNoticeNoteReserve {
		t.Fatalf("the requirement mute reserves %d (limit %d), want %d (limit 3496)", reqDef.ReservedText, limit, memberReqNoticeNoteReserve)
	}
	if v := check(reqDef, limit); !v.OK {
		t.Errorf("a requirement text of %d units was refused: %+v", limit, v)
	}
	v := check(reqDef, limit+1)
	if v.OK || v.RefusalKey != "settings_edit_refuse_too_long_filled" {
		t.Fatalf("a requirement text of %d units = %+v, want the too-long refusal", limit+1, v)
	}
	if got, n := settingsEditParam(t, v, "max"), settingsEditParam(t, v, "n"); got != limit || n != limit+1 {
		t.Errorf("the refusal names %d of %d, want %d of %d", n, got, limit+1, limit)
	}
	if got := settingsEditPromptMax(reqDef, empty); got != limit {
		t.Errorf("the requirement prompt shows %d, want %d", got, limit)
	}

	for name, def := range map[string]settingsEditPageDef{"the warn": warnDef, "the requirement mute's private copy": pmDef} {
		if def.ReservedText != 0 {
			t.Errorf("%s reserves %d, want none", name, def.ReservedText)
		}
		if v := check(def, limit+1); !v.OK {
			t.Errorf("%s refused a text of %d units: %+v", name, limit+1, v)
		}
		if got := settingsEditPromptMax(def, empty); got != settingsEditTextMax {
			t.Errorf("%s's prompt shows %d, want %d", name, got, settingsEditTextMax)
		}
	}
}
