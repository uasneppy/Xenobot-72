//go:build testtools

package modules

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
	"github.com/uasneppy/fuku_robot/fuku/utils/formatting"
)

// settingsMsgWantAutoStates fails the test unless every event of the Automatic family has its line in text with its
// icon and the expected state word (events not in want read Default, or Off for an event that is off by default: the
// AI spam notice, Phase 24).
func settingsMsgWantAutoStates(t *testing.T, text string, want map[string]string) {
	t.Helper()
	for _, ev := range settingsMsgAutoEvents {
		word := "settings_state_default"
		if eventmsg.OffByDefault(ev.Key) {
			word = "common_status_off"
		}
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

// TestSettingsMessagesAutoFamily drives the Automatic family (Phase 23, plan 23-03) through the real private menu: its
// section on the hub and its list, a typed text stored on the warn, the flood's Silent switch and the previews filled
// with each event's sample values and no bot row.
func TestSettingsMessagesAutoFamily(t *testing.T) {
	t.Run("hub and list", func(t *testing.T) {
		env := newSettingsMsgEnv(t)
		m, text, keyboard := settingsMsgOpenPrivate(env, env.admin)
		if !strings.Contains(text, "🤖 <b>"+staffMarker("settings_msg_fam_auto")+"</b>") {
			t.Errorf("the hub %q has no bold 🤖 Automatic actions heading", text)
		}
		settingsMsgWantAutoStates(t, text, nil)
		settingsMsgWantWidths(t, "the hub", keyboard, 2, 2, 1, 1, 2)

		cell, ok := settingsMsgButtonWith(t, keyboard, "a", settingsActMsgFamily, "t", "a")
		if !ok || !strings.HasPrefix(cell.Text, "🤖 ") || !containsMarker(cell.Text, "settings_msg_fam_auto") {
			t.Fatalf("the hub has no 🤖 Automatic actions cell: %+v", keyboard)
		}
		if manual, ok := settingsMsgButtonWith(t, keyboard, "a", settingsActMsgFamily, "t", "m"); !ok || len(keyboard[0]) != 2 || keyboard[0][0].CallbackData != manual.CallbackData {
			t.Errorf("the hub's first row is not the Admin actions cell then the Automatic actions cell: %+v", keyboard[0])
		}
		text, keyboard = m.pressButton(cell)
		if !containsMarker(text, "settings_msg_fam_auto_intro") || !containsMarker(text, "settings_msg_list_hint") {
			t.Errorf("the Automatic list %q lacks its intro or hint", text)
		}
		settingsMsgWantAutoStates(t, text, nil)
		settingsMsgWantWidths(t, "the Automatic list", keyboard, 2, 2, 2, 1, 2)
		if back := settingsButtonFields(t, keyboard[4][0]); back["a"] != settingsActOpen || back["p"] != settingsPageMessages {
			t.Errorf("the Automatic list's Back carries %v, want the hub", back)
		}

		for _, ev := range settingsMsgAutoEvents {
			button, ok := settingsMsgButtonWith(t, keyboard, "a", settingsActMsgEvent, settingsMsgEventField, ev.Code)
			if !ok || !strings.HasPrefix(button.Text, ev.Icon+" ") || !containsMarker(button.Text, ev.NameKey) {
				t.Errorf("the Automatic list has no %s %s cell: %+v", ev.Icon, ev.Code, keyboard)
				continue
			}
			screen, screenKeyboard := m.pressButton(button)
			// An event that is off by default (the AI spam notice, Phase 24) draws the Notice line in place of the Silent
			// line.
			switchLine, otherLine := "settings_msg_line_silent", "settings_msg_line_notice"
			if eventmsg.OffByDefault(ev.Key) {
				switchLine, otherLine = otherLine, switchLine
			}
			for _, key := range []string{ev.NameKey, ev.DescKey, "settings_msg_hint_auto", "settings_edit_line_text", switchLine} {
				if !containsMarker(screen, key) {
					t.Errorf("the %s screen %q lacks %s", ev.Code, screen, key)
				}
			}
			if containsMarker(screen, otherLine) {
				t.Errorf("the %s screen %q carries %s", ev.Code, screen, otherLine)
			}
			if containsMarker(screen, "settings_msg_hint_manual") {
				t.Errorf("the %s screen %q carries the manual family's hint", ev.Code, screen)
			}
			for _, part := range []string{settingsEditPartText, settingsEditPartMedia, settingsEditPartButtons} {
				set := settingsMsgPartButton(t, screenKeyboard, settingsActMsgSet, part)
				if fields := settingsButtonFields(t, set); fields[settingsMsgEventField] != ev.Code {
					t.Errorf("the %s screen's Set of %s carries %v, want k=%s", ev.Code, part, fields, ev.Code)
				}
			}
			if _, ok := settingsMsgButtonWith(t, screenKeyboard, "a", settingsActMsgSilent, settingsMsgEventField, ev.Code); !ok {
				t.Errorf("the %s screen has no Silent toggle: %+v", ev.Code, screenKeyboard)
			}
			if back, ok := settingsMsgButtonWith(t, screenKeyboard, "a", settingsActMsgFamily, "t", "a"); !ok || back.CallbackData == "" {
				t.Errorf("the %s screen has no Back to the Automatic list: %+v", ev.Code, screenKeyboard)
			}
		}
	})

	t.Run("typed text", func(t *testing.T) {
		env := newSettingsMsgEnv(t)
		listEditAdmin(env.settingsEnv, env.chat, env.admin, true)
		user := env.admin
		settingsMsgSetWait(t, env, user, "wn", settingsEditPartText)
		before := settingsEditSentCount(env.settingsEnv, user)
		sendPrivateMessage(env.settingsEnv, user, gotgbot.Message{
			Text: "{mention} {count}/{limit}", Entities: []gotgbot.MessageEntity{{Type: "bold", Offset: 0, Length: 9}},
		})
		const want = "<b>{mention}</b> {count}/{limit}"
		if row, ok := settingsMsgStored(t, env.chat.Id, eventmsg.EventWarn); !ok || row.Text != want || row.UpdatedBy != user.Id || row.Silent {
			t.Errorf("the warn row is %+v (row %v), want the text %q written by %d and Silent off", row, ok, want, user.Id)
		}
		settingsEditWantReply(t, env.settingsEnv, user, before, "settings_edit_saved_text", "settings_msg_ev_warn")
		settingsEditWaitGone(t, env.settingsEnv, user)
		_, hub, _ := settingsMsgOpenPrivate(env, user)
		settingsMsgWantAutoStates(t, hub, map[string]string{eventmsg.EventWarn: "settings_msg_state_custom"})
	})

	t.Run("silent", func(t *testing.T) {
		env := newSettingsMsgEnv(t)
		listEditAdmin(env.settingsEnv, env.chat, env.admin, true)
		m := settingsMsgPrivateMenu(env, env.chat, env.admin)
		_, keyboard := m.press("a", settingsActMsgEvent, settingsMsgEventField, "fl")
		toggle, fields := settingsMsgSilentToggle(t, keyboard)
		if fields[settingsMsgEventField] != "fl" || fields["v"] != "1" {
			t.Fatalf("the flood's Silent toggle carries %v, want k=fl and v=1", fields)
		}
		m.pressButton(toggle)
		if silent, by, ok := settingsMsgSilentStored(t, env.chat.Id, eventmsg.EventFlood); !ok || !silent || by != env.admin.Id {
			t.Errorf("the flood row reads Silent %v by %d (row %v), want Silent on by %d", silent, by, ok, env.admin.Id)
		}
		if n := settingsMsgRows(t, env.chat.Id); n != 1 {
			t.Errorf("the chat has %d event message rows after the press, want the flood's one", n)
		}
		_, hub, _ := settingsMsgOpenPrivate(env, env.admin)
		settingsMsgWantAutoStates(t, hub, map[string]string{eventmsg.EventFlood: "settings_msg_state_silent"})
	})

	t.Run("preview", func(t *testing.T) {
		env := &settingsMsgEnv{settingsEnv: newSettingsEnvWith(t, settingsEnvOptions{RealLocale: true})}
		settingsMsgCleanup(t, env.chat.Id)
		listEditAdmin(env.settingsEnv, env.chat, env.admin, false)
		settingsEditSetLanguages(t, env.admin.Id, "en", env.chat.Id, "en")
		admin := env.admin
		tr := i18n.MustNewTranslator("en")
		text := func(key string) string {
			value, _ := tr.GetString(key)
			return value
		}
		groupMark := settingsEditMarkSends(env.settingsEnv, env.chat.Id)
		fullPreview := func(code string) map[string]any {
			t.Helper()
			mark := settingsEditMarkSends(env.settingsEnv, admin.Id)
			env.pressPrivate(admin, env.updateID(), env.privateFields(env.chat.Id, "a", settingsActMsgFull, settingsMsgEventField, code))
			sent := settingsEditNewSends(env.settingsEnv, admin.Id, mark, false)
			if len(sent) != 2 {
				t.Fatalf("Full preview of %s sent %d messages, want the header and the message: %+v", code, len(sent), sent)
			}
			if header := fmt.Sprint(rulesCallParams(t, sent[0])["text"]); header != text("settings_msg_preview_header") {
				t.Errorf("the header of the %s preview = %q, want %q", code, header, text("settings_msg_preview_header"))
			}
			if sent[1].Method != "sendMessage" {
				t.Errorf("Full preview of %s was sent by %s, want sendMessage", code, sent[1].Method)
			}
			return rulesCallParams(t, sent[1])
		}

		// The warn limit's default: the limit text with the presser's mention and 3/3, then the sample reason, no keyboard.
		params := fullPreview("wl")
		mention := formatting.MentionHtml(admin.Id, admin.FirstName)
		want := fmt.Sprintf(text("warns_limit_muted"), 3, 3, mention) + "\n - " + text("settings_msg_sample_reason")
		if got := fmt.Sprint(params["text"]); got != want {
			t.Errorf("Full preview of the default warn limit = %q, want %q", got, want)
		}
		if keyboard := settingsMsgSentKeyboard(t, params); len(keyboard) != 0 {
			t.Errorf("Full preview of the warn limit carries %+v, want no keyboard", keyboard)
		}

		// A stored flood text: the translated mute word, 6/5 and the flood reason, with no bot row.
		settingsMsgSeed(t, env.chat.Id, eventmsg.EventFlood, "{action} {count}/{limit}: {reason}", false)
		params = fullPreview("fl")
		want = text("eventmsg_action_mute") + " 6/5: " + text("eventmsg_reason_flood")
		if got := fmt.Sprint(params["text"]); got != want {
			t.Errorf("Full preview of the custom flood = %q, want %q", got, want)
		}
		if keyboard := settingsMsgSentKeyboard(t, params); len(keyboard) != 0 {
			t.Errorf("Full preview of the flood carries %+v, want no bot row", keyboard)
		}

		if calls := settingsEditNewSends(env.settingsEnv, env.chat.Id, groupMark, false); len(calls) != 0 {
			t.Errorf("the previews sent %d messages into the group, want none: %+v", len(calls), calls)
		}
	})
}

// TestSettingsMessagesAutoPM proves the private copies of the Automatic family on the real private menu (plan 23-05):
// every event screen offers its private copy row, full width above Back and Close, with the private copy line reading
// Off, and the warn's private copy screen opens with its switch Off and writes nothing.
func TestSettingsMessagesAutoPM(t *testing.T) {
	env := newSettingsMsgEnv(t)
	listEditAdmin(env.settingsEnv, env.chat, env.admin, true)
	m := settingsMsgPrivateMenu(env, env.chat, env.admin)
	group := strconv.FormatInt(env.chat.Id, 10)

	for _, ev := range settingsMsgAutoEvents {
		text, keyboard := m.press("a", settingsActMsgEvent, settingsMsgEventField, ev.Code)
		if !strings.Contains(text, settingsMsgPMLine(false)) {
			t.Errorf("the %s screen %q lacks the private copy line reading Off", ev.Code, text)
		}
		want := encodeCallbackData(settingsCallbackNamespace, map[string]string{
			"a": settingsActMsgEvent, "c": group, settingsMsgDestField: settingsMsgDestPMCode, settingsMsgEventField: ev.Code,
		})
		if row := keyboard[len(keyboard)-2]; len(row) != 1 || row[0].CallbackData != want || !containsMarker(row[0].Text, "settings_btn_msg_pm_open") {
			t.Errorf("the %s screen has no full-width private copy row above Back and Close: %+v", ev.Code, keyboard)
		}
		settingsMsgWantWidths(t, "the empty "+ev.Code+" screen", keyboard, 1, 1, 1, 1, 1, 1, 2)
	}

	text, keyboard := settingsMsgOpenPM(t, m, "wn")
	for _, key := range []string{"settings_msg_pm_title", "settings_msg_ev_warn", "settings_msg_pm_intro", "settings_msg_hint_auto", "settings_msg_hint_pm"} {
		if !containsMarker(text, key) {
			t.Errorf("the warn's private copy screen %q lacks %s", text, key)
		}
	}
	if !strings.Contains(text, settingsMsgPMLine(false)) {
		t.Errorf("the warn's private copy screen %q lacks its line reading Off", text)
	}
	settingsMsgWantWidths(t, "the warn's private copy screen", keyboard, 1, 1, 1, 1, 1, 2)
	toggle, fields := settingsMsgPMSwitch(t, keyboard)
	if toggle.Text != staffMarker("settings_btn_msg_pm")+" ❌" || fields[settingsMsgEventField] != "wn" || fields["v"] != "1" {
		t.Errorf("the warn's private copy switch is %q with %v, want it reading Off with k=wn and v=1", toggle.Text, fields)
	}
	if _, ok := settingsMsgStoredPM(t, env.chat.Id, eventmsg.EventWarn); ok {
		t.Error("opening the warn's private copy screen wrote a pm row")
	}
}

// settingsMsgAllRows is every event message row of chatID, keyed by event and destination, for comparing what a run
// left with what it found.
func settingsMsgAllRows(t *testing.T, chatID int64) map[string]models.ChatEventMessage {
	t.Helper()
	var rows []models.ChatEventMessage
	if err := db.DB.Where("chat_id = ?", chatID).Find(&rows).Error; err != nil {
		t.Fatalf("read the event messages of chat %d: %v", chatID, err)
	}
	out := make(map[string]models.ChatEventMessage, len(rows))
	for _, row := range rows {
		out[row.Event+"|"+row.Dest] = row
	}
	return out
}

// TestSettingsAuthorityMessagesAuto holds the rights of the Automatic family (Phase 23, D-26, MSG-08) in one environment:
// a typist demoted between Set and the typed message stores nothing, and an administrator without can_change_info opens
// the Automatic list, an event screen, a private copy screen and Full preview, while every Set, Remove, Silent and private
// copy switch of the five events, in the group message's and the private copy's form, is the settings_no_change_info
// alert with no edit, no wait and no row written.
func TestSettingsAuthorityMessagesAuto(t *testing.T) {
	env := newSettingsMsgEnv(t)
	admin := env.admin

	// A wait on the forward blocklist's text, then the sender loses can_change_info before typing.
	listEditAdmin(env.settingsEnv, env.chat, admin, true)
	settingsMsgSetWait(t, env, admin, "fb", settingsEditPartText)
	listEditAdmin(env.settingsEnv, env.chat, admin, false)
	before := settingsEditSentCount(env.settingsEnv, admin)
	sendPrivateMessage(env.settingsEnv, admin, gotgbot.Message{Text: "Typed after the demotion {mention}"})
	settingsEditWantReply(t, env.settingsEnv, admin, before, "settings_no_change_info", "settings_edit_wait_stopped")
	settingsEditWaitGone(t, env.settingsEnv, admin)
	if row, ok := settingsMsgStored(t, env.chat.Id, eventmsg.EventForwardBlock); ok {
		t.Errorf("the typed text of a demoted typist was stored: %+v", row)
	}

	// Viewing needs no can_change_info.
	m := settingsMsgPrivateMenu(env, env.chat, admin)
	if text, _ := m.press("a", settingsActMsgFamily, "t", "a"); !containsMarker(text, "settings_msg_fam_auto_intro") {
		t.Errorf("the Automatic list did not open for an administrator without can_change_info: %q", text)
	}
	if text, _ := m.press("a", settingsActMsgEvent, settingsMsgEventField, "wn"); !containsMarker(text, "settings_msg_ev_warn") {
		t.Errorf("the warn's screen did not open for an administrator without can_change_info: %q", text)
	}
	if text, _ := settingsMsgOpenPM(t, m, "fb"); !containsMarker(text, "settings_msg_pm_intro") {
		t.Errorf("the forward blocklist's private copy screen did not open: %q", text)
	}
	sent := len(env.fake.sentTo(admin.Id))
	m.press("a", settingsActMsgFull, settingsMsgEventField, "wn")
	if got := len(env.fake.sentTo(admin.Id)) - sent; got < 2 {
		t.Errorf("the warn's Full preview sent %d messages to an administrator without can_change_info, want the header and the message", got)
	}

	// Every change is refused: the stored flood text and blacklist private text give Remove something to remove.
	settingsMsgSeed(t, env.chat.Id, eventmsg.EventFlood, "Stored flood text", false)
	settingsMsgSeedPM(t, env.chat.Id, eventmsg.EventBlacklist, "Stored blacklist copy", false)
	rows := settingsMsgAllRows(t, env.chat.Id)
	pm := []string{settingsMsgDestField, settingsMsgDestPMCode}
	presses := [][]string{
		{"a", settingsActMsgRemove, "t", settingsEditPartText, settingsMsgEventField, "fl"},
		append([]string{"a", settingsActMsgRemove, "t", settingsEditPartText, settingsMsgEventField, "bl"}, pm...),
	}
	for _, ev := range settingsMsgAutoEvents {
		presses = append(presses,
			[]string{"a", settingsActMsgSet, "t", settingsEditPartText, settingsMsgEventField, ev.Code},
			append([]string{"a", settingsActMsgSet, "t", settingsEditPartText, settingsMsgEventField, ev.Code}, pm...),
			[]string{"a", settingsActMsgSilent, settingsMsgEventField, ev.Code, "v", "1"},
			[]string{"a", settingsActMsgPM, settingsMsgEventField, ev.Code, "v", "1"},
		)
	}
	for _, kv := range presses {
		msgID := env.updateID()
		env.pressPrivate(admin, msgID, env.privateFields(env.chat.Id, kv...))
		if answer, alert := env.lastAnswer(); !alert || !containsMarker(answer, "settings_no_change_info") {
			t.Errorf("%v answered %q (alert %v), want the settings_no_change_info alert", kv, answer, alert)
		}
		if got := len(env.edits(admin.Id, msgID)); got != 0 {
			t.Errorf("the refused %v made %d edits, want none", kv, got)
		}
		if env.redis.Exists(settingsInputKey(admin.Id)) {
			t.Errorf("the refused %v left a wait", kv)
		}
	}
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
}
