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
	dbcache "github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/db/warns"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
)

// settingsWarnsTimedLine is the marker form of the page's action line for an effective timed mode: the timed word
// with the mode's option word and the duration's unit key and amount.
func settingsWarnsTimedLine(optKey, unitKey string, n int) string {
	return staffMarker("settings_warns_action") + " " + staffMarker("settings_opt_timed") + " " +
		staffMarker(optKey) + " " + staffMarker(unitKey) + " " + strconv.Itoa(n)
}

// settingsWarnsTimedRow returns the timed row of the Warnings page's keyboard after checking the page's widths.
func settingsWarnsTimedRow(t *testing.T, keyboard [][]gotgbot.InlineKeyboardButton) []gotgbot.InlineKeyboardButton {
	t.Helper()
	if widths := settingsWarnsWidths(keyboard); fmt.Sprint(widths) != "[5 3 2 1 2]" {
		t.Fatalf("Warnings page keyboard widths = %v, want [5 3 2 1 2]", widths)
	}
	return keyboard[2]
}

// settingsWarnsMarked lists the labels of the marked buttons of a row.
func settingsWarnsMarked(row []gotgbot.InlineKeyboardButton) []string {
	var marked []string
	for _, button := range row {
		if strings.HasPrefix(button.Text, settingsSelectedPrefix) {
			marked = append(marked, button.Text)
		}
	}
	return marked
}

// settingsWarnsPreset returns the button of the duration screen that presses secs.
func settingsWarnsPreset(t *testing.T, keyboard [][]gotgbot.InlineKeyboardButton, secs int) gotgbot.InlineKeyboardButton {
	t.Helper()
	for _, button := range settingsAllButtons(keyboard) {
		if fields := settingsButtonFields(t, button); fields["a"] == settingsActWarnsTimed && fields["v"] == strconv.Itoa(secs) {
			return button
		}
	}
	t.Fatalf("the duration screen has no %d s preset: %+v", secs, keyboard)
	return gotgbot.InlineKeyboardButton{}
}

// settingsWarnsCachedSecs is the duration the command's cached read returns for chatID.
func settingsWarnsCachedSecs(chatID int64) int {
	return warns.GetWarnSettingContext(context.Background(), chatID).WarnModeSecs
}

// TestSettingsWarnsTimed is the page's timed row end to end (D-20): Timed mute opens the duration screen, a preset
// stores the timed mode and returns to the page with its timed word and mark, Main reads the same word, a press of
// the stored preset changes nothing, and a plain choice replaces it.
func TestSettingsWarnsTimed(t *testing.T) {
	env := newSettingsEnv(t)
	settingsWarnsCleanup(t, env.chat.Id)
	msgID := env.openMenu(env.admin)
	_, mainKeyboard := env.lastEdit(env.chat.Id, msgID)
	page, ok := settingsPageButton(t, mainKeyboard, settingsPageWarns)
	if !ok {
		t.Fatal("Main has no Warnings button")
	}
	env.pressRaw(env.chat, env.admin, msgID, page.CallbackData)

	_, keyboard := env.lastEdit(env.chat.Id, msgID)
	timed := settingsWarnsTimedRow(t, keyboard)
	if !containsMarker(timed[0].Text, "settings_opt_tmute") || !containsMarker(timed[1].Text, "settings_opt_tban") {
		t.Fatalf("timed row = %q | %q, want Timed mute and Timed ban", timed[0].Text, timed[1].Text)
	}
	if marked := settingsWarnsMarked(timed); len(marked) != 0 {
		t.Errorf("timed row marks %v for the default Mute, want nothing", marked)
	}
	if fields := settingsButtonFields(t, timed[0]); fields["a"] != "nd" || fields["k"] != "m" || fields["u"] == "" {
		t.Errorf("Timed mute fields = %v, want a=nd k=m and the opener", fields)
	}
	if fields := settingsButtonFields(t, timed[1]); fields["a"] != "nd" || fields["k"] != "b" {
		t.Errorf("Timed ban fields = %v, want a=nd k=b", fields)
	}

	env.pressRaw(env.chat, env.admin, msgID, timed[0].CallbackData)
	text, keyboard := env.lastEdit(env.chat.Id, msgID)
	if !strings.Contains(text, staffMarker("settings_duration_intro_tmute")) || !strings.Contains(text, staffMarker("settings_page_warns")) {
		t.Fatalf("Timed mute drew %q, want the duration screen with Timed mute's intro", text)
	}
	if widths := settingsWarnsWidths(keyboard); fmt.Sprint(widths) != "[2 2 2 2 2]" {
		t.Errorf("duration screen widths = %v, want [2 2 2 2 2]", widths)
	}
	if marked := settingsWarnsMarked(settingsAllButtons(keyboard)); len(marked) != 0 {
		t.Errorf("the duration screen marks %v while Mute is stored, want nothing", marked)
	}
	if _, has := settingsWarnsRow(t, env.chat.Id); has {
		t.Error("opening the duration screen wrote a row")
	}

	answers := env.answerCount()
	hour := settingsWarnsPreset(t, keyboard, 3600)
	env.pressRaw(env.chat, env.admin, msgID, hour.CallbackData)
	if got := env.answerCount() - answers; got != 1 {
		t.Errorf("the preset press answered %d times, want once", got)
	}
	if row, has := settingsWarnsRow(t, env.chat.Id); !has || row.WarnMode != "tmute" || row.WarnModeSecs != 3600 || row.WarnLimit != 3 {
		t.Fatalf("row = %+v (row %v), want tmute 3600 and limit 3", row, has)
	}
	text, keyboard = env.lastEdit(env.chat.Id, msgID)
	if want := settingsWarnsTimedLine("settings_opt_tmute", "staff_act_duration_hours", 1); !strings.Contains(text, want) {
		t.Errorf("the page redrew %q, want the action line %q", text, want)
	}
	if strings.Contains(text, staffMarker("settings_warns_action_inert")) || strings.Contains(text, "⚠️") {
		t.Errorf("the page redrew %q with the inert line or a notice", text)
	}
	if marked := settingsWarnsMarked(keyboard[1]); len(marked) != 0 {
		t.Errorf("the choice row marks %v, want nothing", marked)
	}
	if marked := settingsWarnsMarked(settingsWarnsTimedRow(t, keyboard)); len(marked) != 1 || !containsMarker(marked[0], "settings_opt_tmute") {
		t.Errorf("the timed row marks %v, want Timed mute alone", marked)
	}

	// The duration screen of the stored mode marks the stored preset; the other mode's marks nothing.
	tmuteData, tbanData := keyboard[2][0].CallbackData, keyboard[2][1].CallbackData
	env.pressRaw(env.chat, env.admin, msgID, tmuteData)
	_, screen := env.lastEdit(env.chat.Id, msgID)
	if marked := settingsWarnsMarked(settingsAllButtons(screen)); len(marked) != 1 ||
		!strings.Contains(marked[0], staffMarker("staff_act_duration_hours")+" 1") {
		t.Errorf("Timed mute's screen marks %v, want the 1 hour preset", marked)
	}
	env.pressRaw(env.chat, env.admin, msgID, tbanData)
	_, screen = env.lastEdit(env.chat.Id, msgID)
	if marked := settingsWarnsMarked(settingsAllButtons(screen)); len(marked) != 0 {
		t.Errorf("Timed ban's screen marks %v while a timed mute is stored, want nothing", marked)
	}

	// A press of the stored preset writes the same row again and shows no notice.
	env.pressRaw(env.chat, env.admin, msgID, tmuteData)
	_, screen = env.lastEdit(env.chat.Id, msgID)
	env.pressRaw(env.chat, env.admin, msgID, settingsWarnsPreset(t, screen, 3600).CallbackData)
	text, _ = env.lastEdit(env.chat.Id, msgID)
	if row, _ := settingsWarnsRow(t, env.chat.Id); row.WarnMode != "tmute" || row.WarnModeSecs != 3600 {
		t.Errorf("row = %+v after the stored preset, want tmute 3600", row)
	}
	if strings.Contains(text, "⚠️") {
		t.Errorf("the stored preset redrew %q with a notice, want none", text)
	}

	settingsWarnsPress(t, env, env.chat, msgID, "a", "bk")
	mainText, _ := env.lastEdit(env.chat.Id, msgID)
	line, found := settingsMainStateLine(mainText, "settings_page_warns")
	if want := staffMarker("settings_opt_timed") + " " + staffMarker("settings_opt_tmute") + " " +
		staffMarker("staff_act_duration_hours") + " 1"; !found || !strings.Contains(line, want) {
		t.Errorf("Main status line = %q (found %v), want the timed word %q", line, found, want)
	}

	settingsWarnsPress(t, env, env.chat, msgID, "a", "st", "p", "n", "t", "m", "v", "ban")
	if row, _ := settingsWarnsRow(t, env.chat.Id); row.WarnMode != "ban" || row.WarnModeSecs != 0 {
		t.Errorf("row = %+v after Ban, want ban and 0", row)
	}
	_, keyboard = env.lastEdit(env.chat.Id, msgID)
	if marked := settingsWarnsMarked(settingsWarnsTimedRow(t, keyboard)); len(marked) != 0 {
		t.Errorf("the timed row marks %v after Ban, want nothing", marked)
	}

	t.Run("the real words", func(t *testing.T) {
		withRealLocales(t)
		screen := renderSettingsWarns(settingsWarnsViews(i18n.MustNewTranslator("en"), "G")[1].view,
			&models.WarnSettings{WarnLimit: 3, WarnMode: "tmute", WarnModeSecs: 3600}, "")
		if !strings.Contains(screen.Text, "<b>Timed mute, 1 hour(s)</b>") {
			t.Errorf("the page %q does not read Timed mute, 1 hour(s)", screen.Text)
		}
	})

	t.Run("every timed state fits every language", func(t *testing.T) {
		withRealLocales(t)
		title := strings.Repeat("&", 64)
		for _, code := range []string{"uk", "en", "es", "fr", "hi", "id", "pt", "ru"} {
			tr := i18n.MustNewTranslator(code)
			notice := settingsNoticeWarn(fmt.Sprintf(settingsText(tr, "warns_mode_unknown"), strings.Repeat("a", settingsFloodModeMaxLen)))
			for _, variant := range settingsWarnsViews(tr, title) {
				for _, s := range []models.WarnSettings{
					{WarnLimit: 100, WarnMode: "tmute", WarnModeSecs: punishMaxSecs},
					{WarnLimit: 100, WarnMode: "tban", WarnModeSecs: 5400},
				} {
					label := fmt.Sprintf("%s %s %s %d", code, variant.name, s.WarnMode, s.WarnModeSecs)
					screen := renderSettingsWarns(variant.view, &s, notice)
					if units := panelUTF16Len(screen.Text); units > 3800 || units == 0 {
						t.Errorf("%s is %d UTF-16 units, want 1..3800", label, units)
					}
					if widths := settingsWarnsWidths(screen.Keyboard.InlineKeyboard); fmt.Sprint(widths) != "[5 3 2 1 2]" {
						t.Errorf("%s keyboard widths = %v, want [5 3 2 1 2]", label, widths)
					}
					for _, button := range settingsAllButtons(screen.Keyboard.InlineKeyboard) {
						if n := len(button.CallbackData); n < 1 || n > 64 {
							t.Errorf("%s button %q has %d bytes", label, button.Text, n)
						}
					}
					for _, button := range screen.Keyboard.InlineKeyboard[2] {
						if runes := len([]rune(button.Text)); runes > 20 {
							t.Errorf("%s timed cell %q has %d code points, want at most 20", label, button.Text, runes)
						}
					}
					if strings.ContainsAny(screen.Text, "{}") {
						t.Errorf("%s keeps an unfilled placeholder: %q", label, screen.Text)
					}
				}
			}
		}
	})
}

// TestSettingsParityWarnTimed holds the page's timed press and /setwarnmode to the same stored row and cache.
func TestSettingsParityWarnTimed(t *testing.T) {
	const menuMsgID = 6363
	for _, tc := range []struct {
		name, command, k, mode string
		secs                   int
	}{
		{"tmute 6h", "/setwarnmode tmute 6h", "m", "tmute", 21600},
		{"tban 1d", "/setwarnmode tban 1d", "b", "tban", 86400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := newSettingsEnv(t)
			commandChat, menuChat := env.chat, env.addChat()
			settingsWarnsCleanup(t, commandChat.Id, menuChat.Id)
			settingsWarnsSeed(t, commandChat.Id, 5, "kick")
			settingsWarnsSeed(t, menuChat.Id, 5, "kick")

			settingsWarnsCommand(env, commandChat, tc.command)
			settingsWarnsPress(t, env, menuChat, menuMsgID, "a", "nt", "k", tc.k, "v", strconv.Itoa(tc.secs))

			commandRow, _ := settingsWarnsRow(t, commandChat.Id)
			menuRow, _ := settingsWarnsRow(t, menuChat.Id)
			if commandRow.WarnMode != tc.mode || commandRow.WarnModeSecs != tc.secs || commandRow.WarnLimit != 5 {
				t.Errorf("%s stored %+v, want %s %d and limit 5", tc.command, commandRow, tc.mode, tc.secs)
			}
			if menuRow.WarnMode != commandRow.WarnMode || menuRow.WarnModeSecs != commandRow.WarnModeSecs ||
				menuRow.WarnLimit != commandRow.WarnLimit {
				t.Errorf("the page stored %+v, the command %+v, want the same row", menuRow, commandRow)
			}
			cl, cm, ck := settingsWarnsCached(env, commandChat.Id)
			ml, mm, mk := settingsWarnsCached(env, menuChat.Id)
			cs, ms := settingsWarnsCachedSecs(commandChat.Id), settingsWarnsCachedSecs(menuChat.Id)
			if cl != ml || cm != mm || ck != mk || cs != ms {
				t.Errorf("cache state differs: command (%d, %q, %d, key %v), page (%d, %q, %d, key %v)", cl, cm, cs, ck, ml, mm, ms, mk)
			}
		})
	}
}

// TestSettingsDriftWarnTimed holds the page to what the command stored, a duration that is no preset included,
// and the warn limit to what the page stored.
func TestSettingsDriftWarnTimed(t *testing.T) {
	const menuMsgID = 6464

	t.Run("a command's duration that is no preset is drawn and marks no preset", func(t *testing.T) {
		env := newSettingsEnv(t)
		chat := env.addChat()
		settingsWarnsCleanup(t, chat.Id)
		settingsWarnsCommand(env, chat, "/setwarnmode tmute 90m")

		text, keyboard := settingsWarnsOpenPage(env, chat, menuMsgID)
		if want := settingsWarnsTimedLine("settings_opt_tmute", "staff_act_duration_minutes", 90); !strings.Contains(text, want) {
			t.Errorf("the page shows %q, want %q", text, want)
		}
		if marked := settingsWarnsMarked(settingsWarnsTimedRow(t, keyboard)); len(marked) != 1 || !containsMarker(marked[0], "settings_opt_tmute") {
			t.Errorf("the timed row marks %v, want Timed mute", marked)
		}
		env.pressRaw(chat, env.admin, menuMsgID, keyboard[2][0].CallbackData)
		_, screen := env.lastEdit(chat.Id, menuMsgID)
		if marked := settingsWarnsMarked(settingsAllButtons(screen)); len(marked) != 0 {
			t.Errorf("the duration screen marks %v for 90 minutes, want no preset", marked)
		}
	})

	t.Run("a member who reaches the limit is banned for the page's duration", func(t *testing.T) {
		env := newSettingsEnv(t)
		chat := env.addChat()
		settingsWarnsCleanup(t, chat.Id)
		now := time.Unix(1_900_000_000, 0)
		mreqSwap(t, &punishNow, func() time.Time { return now })
		member := uniqueLinkOwnerID() + 43
		t.Cleanup(func() {
			db.DB.Where("chat_id = ?", chat.Id).Delete(&models.Warns{})
			dbcache.ResetLocalForTest()
		})
		if err := warns.SetWarnLimit(chat.Id, 1); err != nil {
			t.Fatalf("SetWarnLimit() error = %v", err)
		}
		settingsWarnsPress(t, env, chat, menuMsgID, "a", "nt", "k", "b", "v", "21600")
		if row, _ := settingsWarnsRow(t, chat.Id); row.WarnMode != "tban" || row.WarnModeSecs != 21600 {
			t.Fatalf("row = %+v, want tban 21600", row)
		}

		res, err := applyWarn(context.Background(), env.bot, &chat, member, "spam", nil)
		if err != nil || !res.LimitApplied {
			t.Fatalf("applyWarn() = %+v, %v, want the limit applied", res, err)
		}
		var bans []moduleBotCall
		for _, call := range env.fake.callsFor("banChatMember") {
			if staffParamInt(call.Params, "user_id") == member {
				bans = append(bans, call)
			}
		}
		if len(bans) != 1 {
			t.Fatalf("banChatMember calls about the member = %d, want 1", len(bans))
		}
		if got, want := fmt.Sprint(bans[0].Params["until_date"]), strconv.FormatInt(now.Unix()+21600, 10); got != want {
			t.Errorf("until_date = %s, want %s", got, want)
		}
	})
}

// TestSettingsAuthorityParityWarnTimed holds the timed press to what /setwarnmode refuses, with the owner gate's
// documented carve-out, and the duration screen to the live admin check alone.
func TestSettingsAuthorityParityWarnTimed(t *testing.T) {
	const menuMsgID = 6565
	press := []string{"a", "nt", "k", "m", "v", "21600"}
	const command = "/setwarnmode tmute 6h"
	wrote := func(row models.WarnSettings) bool { return row.WarnMode == "tmute" && row.WarnModeSecs == 21600 }

	t.Run("a live administrator writes by both doors", func(t *testing.T) {
		env := newSettingsEnv(t)
		commandChat, menuChat := env.addChat(), env.addChat()
		settingsWarnsCleanup(t, commandChat.Id, menuChat.Id)
		settingsWarnsCommand(env, commandChat, command)
		settingsWarnsPress(t, env, menuChat, menuMsgID, press...)
		for _, id := range []int64{commandChat.Id, menuChat.Id} {
			if row, has := settingsWarnsRow(t, id); !has || !wrote(row) {
				t.Errorf("chat %d row = %+v (row %v), want tmute 21600", id, row, has)
			}
		}
	})

	t.Run("a bot that is not an admin is refused like the command, and the duration screen still opens", func(t *testing.T) {
		env := newSettingsEnv(t)
		commandChat, menuChat := env.addChat(), env.addChat()
		settingsWarnsCleanup(t, commandChat.Id, menuChat.Id)
		env.fake.setBotRole(commandChat.Id, staffRoleMember)
		env.fake.setBotRole(menuChat.Id, staffRoleMember)
		refusal := staffMarker("connections_is_user_connected_bot_not_admin")

		settingsWarnsCommand(env, commandChat, command)
		refusedByCommand := false
		for _, sent := range env.fake.sentTo(commandChat.Id) {
			if strings.Contains(fmt.Sprint(sent.Params["text"]), refusal) {
				refusedByCommand = true
			}
		}
		if !refusedByCommand {
			t.Fatalf("%s was not refused for a bot that is not an admin", command)
		}

		// nd needs only the live admin check.
		settingsWarnsPress(t, env, menuChat, menuMsgID, "a", "nd", "k", "m")
		if text, _ := env.lastEdit(menuChat.Id, menuMsgID); !strings.Contains(text, staffMarker("settings_duration_intro_tmute")) {
			t.Fatalf("nd drew %q, want the duration screen", text)
		}
		answers := env.answerCount()
		settingsWarnsPress(t, env, menuChat, menuMsgID, press...)
		text, _ := env.lastEdit(menuChat.Id, menuMsgID)
		if !strings.Contains(text, refusal) || !strings.Contains(text, staffMarker("settings_warns_limit")) {
			t.Errorf("the press drew %q, want the Warnings page with the notice %s", text, refusal)
		}
		if got := env.answerCount() - answers; got != 1 {
			t.Errorf("the press answered %d times, want once", got)
		}
		for _, id := range []int64{commandChat.Id, menuChat.Id} {
			if _, has := settingsWarnsRow(t, id); has {
				t.Errorf("a refused press or command wrote a row for chat %d", id)
			}
		}
	})

	t.Run("a plain member is refused by both", func(t *testing.T) {
		env := newSettingsEnv(t)
		chat := env.addChat()
		settingsWarnsCleanup(t, chat.Id)
		member := gotgbot.User{Id: uniqueLinkOwnerID() + 42, FirstName: "Plain"}
		t.Cleanup(func() {
			db.DB.Where("user_id IN ?", []int64{member.Id}).Delete(&models.User{})
			dbcache.DeleteCache(dbcache.CacheKey("user", member.Id))
			dbcache.ResetLocalForTest()
		})
		env.fake.setMember(chat.Id, member.Id, staffFakeMember{Status: gotgbot.ChatMemberStatusMember})
		_ = warnsModule.setWarnMode(env.bot, newModuleMessageContext(env.bot, chat, member, command))

		for _, fields := range [][]string{press, {"a", "nd", "k", "m"}} {
			env.pressPrivate(member, 778, env.privateFields(chat.Id, fields...))
			answer, alert := env.lastAnswer()
			if !alert || !strings.Contains(answer, staffMarker("settings_not_admin")) {
				t.Errorf("%v: answer = %q (alert %v), want the alert settings_not_admin", fields, answer, alert)
			}
		}
		if _, has := settingsWarnsRow(t, chat.Id); has {
			t.Error("a plain member's press or command wrote a row")
		}
	})

	t.Run("an administrator the owner gate refuses is refused by the menu only (the documented carve-out)", func(t *testing.T) {
		resetSettingsOwnerMemo(t)
		env := newSettingsEnv(t)
		commandChat, menuChat := env.addChat(), env.addChat()
		settingsWarnsCleanup(t, commandChat.Id, menuChat.Id)
		for _, chat := range []gotgbot.Chat{commandChat, menuChat} {
			env.newOwner(chat)
			env.setAdminsAllowed(chat.Id, false)
		}

		settingsWarnsPress(t, env, menuChat, menuMsgID, press...)
		if answer, alert := env.lastAnswer(); !alert || !strings.Contains(answer, staffMarker("settings_gate_owner_only")) {
			t.Errorf("answer = %q (alert %v), want the alert settings_gate_owner_only", answer, alert)
		}
		if _, has := settingsWarnsRow(t, menuChat.Id); has {
			t.Error("the gated press wrote a row")
		}
		settingsWarnsCommand(env, commandChat, command)
		if row, has := settingsWarnsRow(t, commandChat.Id); !has || !wrote(row) {
			t.Errorf("%s row = %+v (row %v), want the command to write", command, row, has)
		}
	})
}

// TestSettingsWarnsTimedForged refuses every forged timed press in step 1, before any lookup, with nothing written.
func TestSettingsWarnsTimedForged(t *testing.T) {
	const menuMsgID = 6666
	forged := []struct {
		name   string
		fields []string
	}{
		{"nt v 7201", []string{"a", "nt", "k", "m", "v", "7201"}},
		{"nt v 0", []string{"a", "nt", "k", "m", "v", "0"}},
		{"nt v 600x", []string{"a", "nt", "k", "m", "v", "600x"}},
		{"nt v 0600", []string{"a", "nt", "k", "b", "v", "0600"}},
		{"nt k x", []string{"a", "nt", "k", "x", "v", "3600"}},
		{"nt no k", []string{"a", "nt", "v", "3600"}},
		{"nt no v", []string{"a", "nt", "k", "m"}},
		{"nt an extra key", []string{"a", "nt", "k", "m", "v", "3600", "p", "n"}},
		{"nd k z", []string{"a", "nd", "k", "z"}},
		{"nd no k", []string{"a", "nd"}},
		{"nd an extra key", []string{"a", "nd", "k", "m", "v", "3600"}},
	}
	env := newSettingsEnv(t)
	settingsWarnsCleanup(t, env.chat.Id)
	for _, tc := range forged {
		t.Run(tc.name, func(t *testing.T) {
			data := encodeCallbackData(settingsCallbackNamespace, env.groupFields(env.admin, tc.fields...))
			if data == "" {
				t.Fatalf("forged press %v did not encode", tc.fields)
			}
			before := settingsMenuMarkOf(env)
			env.pressRaw(env.chat, env.admin, menuMsgID, data)
			settingsMenuWantRefused(t, env, before, "common_callback_invalid_request", true)
			if _, has := settingsWarnsRow(t, env.chat.Id); has {
				t.Error("a forged press wrote a row")
			}
		})
	}
}
