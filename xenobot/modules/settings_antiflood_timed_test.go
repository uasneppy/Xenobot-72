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
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/antiflood"
	dbcache "github.com/uasneppy/Xenobot-72/xenobot/db/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
)

// settingsFloodTimedLine is the marker form of the page's action line for an effective timed action: the timed word
// with the action's option word and the duration's unit key and amount.
func settingsFloodTimedLine(optKey, unitKey string, n int) string {
	return staffMarker("settings_antiflood_action") + " " + staffMarker("settings_opt_timed") + " " +
		staffMarker(optKey) + " " + staffMarker(unitKey) + " " + strconv.Itoa(n)
}

// settingsFloodTimedRowOf returns the timed row of the Antiflood page's keyboard after checking the page's widths.
func settingsFloodTimedRowOf(t *testing.T, keyboard [][]gotgbot.InlineKeyboardButton) []gotgbot.InlineKeyboardButton {
	t.Helper()
	if widths := settingsWarnsWidths(keyboard); fmt.Sprint(widths) != "[5 3 2 1 2]" {
		t.Fatalf("Antiflood page keyboard widths = %v, want [5 3 2 1 2]", widths)
	}
	return keyboard[2]
}

// settingsFloodPreset returns the button of the duration screen that presses secs.
func settingsFloodPreset(t *testing.T, keyboard [][]gotgbot.InlineKeyboardButton, secs int) gotgbot.InlineKeyboardButton {
	t.Helper()
	for _, button := range settingsAllButtons(keyboard) {
		if fields := settingsButtonFields(t, button); fields["a"] == settingsActFloodTimed && fields["v"] == strconv.Itoa(secs) {
			return button
		}
	}
	t.Fatalf("the duration screen has no %d s preset: %+v", secs, keyboard)
	return gotgbot.InlineKeyboardButton{}
}

// settingsFloodSeedTimed writes an antiflood row with a limit, an action and its seconds directly and drops the
// cached copy.
func settingsFloodSeedTimed(t *testing.T, chatID int64, limit int, action string, secs int) {
	t.Helper()
	if err := db.DB.Create(&models.AntifloodSettings{ChatId: chatID, Limit: limit, Action: action, ActionSecs: secs}).Error; err != nil {
		t.Fatalf("seed antiflood_settings of %d: %v", chatID, err)
	}
	dbcache.DeleteCache(dbcache.CacheKey("antiflood", chatID))
	dbcache.ResetLocalForTest()
}

// settingsFloodOpenPage opens the Antiflood page of chat in a group menu on message msgID and returns what it drew.
func settingsFloodOpenPage(t *testing.T, env *settingsEnv, chat gotgbot.Chat, msgID int64) (string, [][]gotgbot.InlineKeyboardButton) {
	t.Helper()
	settingsAntifloodPress(t, env, chat, msgID, "a", "op", "p", "f")
	return env.lastEdit(chat.Id, msgID)
}

// TestSettingsAntifloodTimed is the page's timed row end to end (D-21): Timed ban opens the duration screen, a
// preset stores the timed action and returns to the page with its timed word and mark, a timed action with no
// duration keeps the read-only note, and the first timed press on a chat with no row says antiflood is now on.
func TestSettingsAntifloodTimed(t *testing.T) {
	t.Run("Timed ban stores tban 1 day", func(t *testing.T) {
		env := newSettingsEnv(t)
		env.send(env.admin, "/setflood 7")
		msgID := settingsAntifloodOpen(t, env)

		_, keyboard := env.lastEdit(env.chat.Id, msgID)
		timed := settingsFloodTimedRowOf(t, keyboard)
		if !containsMarker(timed[0].Text, "settings_opt_tmute") || !containsMarker(timed[1].Text, "settings_opt_tban") {
			t.Fatalf("timed row = %q | %q, want Timed mute and Timed ban", timed[0].Text, timed[1].Text)
		}
		if marked := settingsWarnsMarked(timed); len(marked) != 0 {
			t.Errorf("timed row marks %v for the default Mute, want nothing", marked)
		}
		if fields := settingsButtonFields(t, timed[0]); fields["a"] != "fd" || fields["k"] != "m" || fields["u"] == "" {
			t.Errorf("Timed mute fields = %v, want a=fd k=m and the opener", fields)
		}
		if fields := settingsButtonFields(t, timed[1]); fields["a"] != "fd" || fields["k"] != "b" {
			t.Errorf("Timed ban fields = %v, want a=fd k=b", fields)
		}

		env.pressRaw(env.chat, env.admin, msgID, timed[1].CallbackData)
		text, screen := env.lastEdit(env.chat.Id, msgID)
		if !strings.Contains(text, staffMarker("settings_duration_intro_tban")) || !strings.Contains(text, staffMarker("settings_page_antiflood")) {
			t.Fatalf("Timed ban drew %q, want the duration screen with Timed ban's intro", text)
		}
		if widths := settingsWarnsWidths(screen); fmt.Sprint(widths) != "[2 2 2 2 2]" {
			t.Errorf("duration screen widths = %v, want [2 2 2 2 2]", widths)
		}
		back := screen[len(screen)-1][0]
		if fields := settingsButtonFields(t, back); fields["a"] != "op" || fields["p"] != "f" {
			t.Errorf("Back fields = %v, want the Antiflood page", fields)
		}
		if row, _ := settingsAntifloodRow(t, env.chat.Id); row.Action != "mute" || row.ActionSecs != 0 {
			t.Errorf("opening the duration screen changed the row to %+v", row)
		}

		answers := env.answerCount()
		env.pressRaw(env.chat, env.admin, msgID, settingsFloodPreset(t, screen, 86400).CallbackData)
		if got := env.answerCount() - answers; got != 1 {
			t.Errorf("the preset press answered %d times, want once", got)
		}
		if row, ok := settingsAntifloodRow(t, env.chat.Id); !ok || row.Action != "tban" || row.ActionSecs != 86400 || row.Limit != 7 {
			t.Fatalf("row = %+v (row %v), want tban 86400 at limit 7", row, ok)
		}
		text, keyboard = env.lastEdit(env.chat.Id, msgID)
		if want := settingsFloodTimedLine("settings_opt_tban", "staff_act_duration_days", 1); !strings.Contains(text, want) {
			t.Errorf("the page redrew %q, want the action line %q", text, want)
		}
		if strings.Contains(text, staffMarker("settings_antiflood_action_note")) || strings.Contains(text, "⚠️") {
			t.Errorf("the page redrew %q with the read-only note or a notice", text)
		}
		if marked := settingsWarnsMarked(keyboard[1]); len(marked) != 0 {
			t.Errorf("the choice row marks %v, want nothing", marked)
		}
		if marked := settingsWarnsMarked(settingsFloodTimedRowOf(t, keyboard)); len(marked) != 1 || !containsMarker(marked[0], "settings_opt_tban") {
			t.Errorf("the timed row marks %v, want Timed ban alone", marked)
		}

		// Timed ban's screen marks the stored preset; Timed mute's marks nothing.
		tmuteData, tbanData := keyboard[2][0].CallbackData, keyboard[2][1].CallbackData
		env.pressRaw(env.chat, env.admin, msgID, tbanData)
		_, screen = env.lastEdit(env.chat.Id, msgID)
		if marked := settingsWarnsMarked(settingsAllButtons(screen)); len(marked) != 1 ||
			!strings.Contains(marked[0], staffMarker("staff_act_duration_days")+" 1") {
			t.Errorf("Timed ban's screen marks %v, want the 1 day preset", marked)
		}
		env.pressRaw(env.chat, env.admin, msgID, tmuteData)
		_, screen = env.lastEdit(env.chat.Id, msgID)
		if marked := settingsWarnsMarked(settingsAllButtons(screen)); len(marked) != 0 {
			t.Errorf("Timed mute's screen marks %v while a timed ban is stored, want nothing", marked)
		}

		// A plain choice replaces the timed action and clears its seconds.
		settingsAntifloodPress(t, env, env.chat, msgID, "a", "st", "p", "f", "t", "m", "v", "kick")
		if row, _ := settingsAntifloodRow(t, env.chat.Id); row.Action != "kick" || row.ActionSecs != 0 {
			t.Errorf("row = %+v after Kick, want kick and 0", row)
		}
		_, keyboard = env.lastEdit(env.chat.Id, msgID)
		if marked := settingsWarnsMarked(settingsFloodTimedRowOf(t, keyboard)); len(marked) != 0 {
			t.Errorf("the timed row marks %v after Kick, want nothing", marked)
		}
	})

	t.Run("a timed action with no duration keeps the note", func(t *testing.T) {
		env := newSettingsEnv(t)
		settingsFloodSeedTimed(t, env.chat.Id, 5, "tmute", 0)
		msgID := settingsAntifloodOpen(t, env)
		text, keyboard := env.lastEdit(env.chat.Id, msgID)
		if !strings.Contains(text, staffMarker("settings_antiflood_action")+" "+staffMarker("settings_opt_tmute")) ||
			!strings.Contains(text, staffMarker("settings_antiflood_action_note")) {
			t.Errorf("page %q, want the tmute word and the read-only note", text)
		}
		if marked := settingsWarnsMarked(settingsAllButtons(keyboard)); len(marked) != 0 {
			t.Errorf("an inert tmute marks %v, want nothing", marked)
		}
	})

	t.Run("the first timed press on a chat with no row says antiflood is now on", func(t *testing.T) {
		env := newSettingsEnv(t)
		msgID := settingsAntifloodOpen(t, env)
		settingsAntifloodPress(t, env, env.chat, msgID, "a", "ft", "k", "m", "v", "600")
		if row, ok := settingsAntifloodRow(t, env.chat.Id); !ok || row.Limit != 5 || row.Action != "tmute" || row.ActionSecs != 600 {
			t.Fatalf("row = %+v (row %v), want tmute 600 at the default limit 5", row, ok)
		}
		text, _ := env.lastEdit(env.chat.Id, msgID)
		if want := settingsNoticeWarnIcon + staffMarker("settings_antiflood_now_on") + " 5"; !strings.Contains(text, want) {
			t.Errorf("the redraw %q lacks the notice %q", text, want)
		}
	})

	t.Run("every timed state fits every language", func(t *testing.T) {
		withRealLocales(t)
		for _, code := range []string{"uk", "en", "es", "fr", "hi", "id", "pt", "ru"} {
			tr := i18n.MustNewTranslator(code)
			notice := settingsNoticeWarn(settingsText(tr, "antiflood_setfloodmode_bad_duration"))
			view := settingsView{Tr: tr, OwnerKey: "u", OwnerValue: "9999999999", GroupID: -1001234567890,
				GroupTitle: strings.Repeat("&", 64), OpenerName: "N"}
			for _, s := range []models.AntifloodSettings{
				{Limit: 100, Action: "tmute", ActionSecs: punishMaxSecs, DeleteAntifloodMessage: true},
				{Limit: 100, Action: "tban", ActionSecs: 5400},
			} {
				label := fmt.Sprintf("%s %s %d", code, s.Action, s.ActionSecs)
				screen := renderSettingsAntiflood(view, &s, notice)
				if units := panelUTF16Len(screen.Text); units > 3800 || units == 0 {
					t.Errorf("%s is %d UTF-16 units, want 1..3800", label, units)
				}
				if widths := settingsWarnsWidths(screen.Keyboard.InlineKeyboard); fmt.Sprint(widths) != "[5 3 2 1 2]" {
					t.Errorf("%s keyboard widths = %v, want [5 3 2 1 2]", label, widths)
				}
				for _, button := range screen.Keyboard.InlineKeyboard[2] {
					if runes := len([]rune(button.Text)); runes > 20 {
						t.Errorf("%s timed cell %q has %d code points, want at most 20", label, button.Text, runes)
					}
				}
				if strings.ContainsAny(screen.Text, "{}") || strings.Contains(screen.Text, staffMarker("settings_antiflood_action_note")) {
					t.Errorf("%s keeps a placeholder or the note: %q", label, screen.Text)
				}
				settingsAntifloodCheckBytes(t, label, screen)
			}
		}
	})
}

// settingsFloodCached is the action and seconds the command's cached read returns for chatID, and whether its cache
// key is in Redis.
func settingsFloodCached(env *settingsEnv, chatID int64) (action string, secs int, keyed bool) {
	for _, key := range env.redis.Keys() {
		if strings.Contains(key, "antiflood") && strings.Contains(key, fmt.Sprint(chatID)) {
			keyed = true
		}
	}
	stored := antiflood.GetFloodContext(context.Background(), chatID)
	return stored.Action, stored.ActionSecs, keyed
}

// TestSettingsParityFloodTimed holds the page's timed press and /setfloodmode to the same stored row and cache.
func TestSettingsParityFloodTimed(t *testing.T) {
	const menuMsgID = 6767
	for _, tc := range []struct {
		name, command, k, mode string
		secs                   int
	}{
		// 30 minutes is no preset (an ft press with v 1800 is refused in step 1), so the timed mute is compared at
		// the 10 minute preset.
		{"tmute 10m", "/setfloodmode tmute 10m", "m", "tmute", 600},
		{"tban 1d", "/setfloodmode tban 1d", "b", "tban", 86400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := newSettingsEnv(t)
			commandChat, menuChat := env.chat, env.addChat()
			settingsAntifloodSeed(t, commandChat.Id, 5, "kick")
			settingsAntifloodSeed(t, menuChat.Id, 5, "kick")

			env.sendIn(commandChat, env.admin, tc.command)
			settingsAntifloodPress(t, env, menuChat, menuMsgID, "a", "ft", "k", tc.k, "v", strconv.Itoa(tc.secs))

			commandRow, _ := settingsAntifloodRow(t, commandChat.Id)
			menuRow, _ := settingsAntifloodRow(t, menuChat.Id)
			if commandRow.Action != tc.mode || commandRow.ActionSecs != tc.secs || commandRow.Limit != 5 {
				t.Errorf("%s stored %+v, want %s %d and limit 5", tc.command, commandRow, tc.mode, tc.secs)
			}
			if menuRow.Action != commandRow.Action || menuRow.ActionSecs != commandRow.ActionSecs ||
				menuRow.Limit != commandRow.Limit || menuRow.DeleteAntifloodMessage != commandRow.DeleteAntifloodMessage {
				t.Errorf("the page stored %+v, the command %+v, want the same row", menuRow, commandRow)
			}
			ca, cs, ck := settingsFloodCached(env, commandChat.Id)
			ma, ms, mk := settingsFloodCached(env, menuChat.Id)
			if ca != ma || cs != ms || ck != mk {
				t.Errorf("cache state differs: command (%q, %d, key %v), page (%q, %d, key %v)", ca, cs, ck, ma, ms, mk)
			}
		})
	}
}

// TestSettingsDriftFloodTimed holds the page to what the command stored, a duration that is no preset included,
// and the flood to what the page stored.
func TestSettingsDriftFloodTimed(t *testing.T) {
	const menuMsgID = 6868

	t.Run("a command's duration that is no preset is drawn and marks no preset", func(t *testing.T) {
		env := newSettingsEnv(t)
		chat := env.addChat()
		env.sendIn(chat, env.admin, "/setfloodmode tmute 90m")

		text, keyboard := settingsFloodOpenPage(t, env, chat, menuMsgID)
		if want := settingsFloodTimedLine("settings_opt_tmute", "staff_act_duration_minutes", 90); !strings.Contains(text, want) {
			t.Errorf("the page shows %q, want %q", text, want)
		}
		if marked := settingsWarnsMarked(settingsFloodTimedRowOf(t, keyboard)); len(marked) != 1 || !containsMarker(marked[0], "settings_opt_tmute") {
			t.Errorf("the timed row marks %v, want Timed mute", marked)
		}
		env.pressRaw(chat, env.admin, menuMsgID, keyboard[2][0].CallbackData)
		_, screen := env.lastEdit(chat.Id, menuMsgID)
		if marked := settingsWarnsMarked(settingsAllButtons(screen)); len(marked) != 0 {
			t.Errorf("the duration screen marks %v for 90 minutes, want no preset", marked)
		}
	})

	t.Run("a member who floods is muted for the page's duration", func(t *testing.T) {
		env := newSettingsEnv(t)
		chat := env.addChat()
		now := time.Unix(1_900_000_000, 0)
		mreqSwap(t, &punishNow, func() time.Time { return now })
		resetAntifloodState(t)
		t.Cleanup(func() { resetAntifloodState(t) })
		member := gotgbot.User{Id: uniqueLinkOwnerID() + 44, FirstName: "Flooder"}
		t.Cleanup(func() {
			db.DB.Where("user_id IN ?", []int64{member.Id}).Delete(&models.User{})
			dbcache.DeleteCache(dbcache.CacheKey("user", member.Id))
			dbcache.ResetLocalForTest()
		})
		env.fake.setMember(chat.Id, member.Id, staffFakeMember{Status: gotgbot.ChatMemberStatusMember})
		if err := antiflood.SetFlood(chat.Id, 1); err != nil {
			t.Fatalf("SetFlood() error = %v", err)
		}
		settingsAntifloodPress(t, env, chat, menuMsgID, "a", "ft", "k", "m", "v", "21600")
		if row, _ := settingsAntifloodRow(t, chat.Id); row.Action != "tmute" || row.ActionSecs != 21600 {
			t.Fatalf("row = %+v, want tmute 21600", row)
		}

		for _, text := range []string{"one", "two"} {
			if err := antifloodModule.checkFlood(env.bot, newModuleMessageContext(env.bot, chat, member, text)); err != ext.ContinueGroups {
				t.Fatalf("checkFlood(%q) error = %v, want ContinueGroups", text, err)
			}
		}
		var restricts []moduleBotCall
		for _, call := range env.fake.callsFor("restrictChatMember") {
			if staffParamInt(call.Params, "user_id") == member.Id {
				restricts = append(restricts, call)
			}
		}
		if len(restricts) != 1 {
			t.Fatalf("restrictChatMember calls about the member = %d, want 1", len(restricts))
		}
		if got, want := fmt.Sprint(restricts[0].Params["until_date"]), strconv.FormatInt(now.Unix()+21600, 10); got != want {
			t.Errorf("until_date = %s, want %s", got, want)
		}
	})
}

// TestSettingsAuthorityParityFloodTimed holds the timed press to what /setfloodmode refuses, with the owner gate's
// documented carve-out, and the duration screen to the live admin check alone.
func TestSettingsAuthorityParityFloodTimed(t *testing.T) {
	const menuMsgID = 6969
	press := []string{"a", "ft", "k", "m", "v", "21600"}
	const command = "/setfloodmode tmute 6h"
	wrote := func(row models.AntifloodSettings) bool { return row.Action == "tmute" && row.ActionSecs == 21600 }

	t.Run("a live administrator writes by both doors", func(t *testing.T) {
		env := newSettingsEnv(t)
		commandChat, menuChat := env.addChat(), env.addChat()
		env.sendIn(commandChat, env.admin, command)
		settingsAntifloodPress(t, env, menuChat, menuMsgID, press...)
		for _, id := range []int64{commandChat.Id, menuChat.Id} {
			if row, has := settingsAntifloodRow(t, id); !has || !wrote(row) {
				t.Errorf("chat %d row = %+v (row %v), want tmute 21600", id, row, has)
			}
		}
	})

	t.Run("a bot that is not an admin is refused like the command, and the duration screen still opens", func(t *testing.T) {
		env := newSettingsEnv(t)
		commandChat, menuChat := env.addChat(), env.addChat()
		env.fake.setBotRole(commandChat.Id, staffRoleMember)
		env.fake.setBotRole(menuChat.Id, staffRoleMember)
		refusal := staffMarker("connections_is_user_connected_bot_not_admin")

		env.sendIn(commandChat, env.admin, command)
		if !settingsAntifloodSentHas(env, commandChat.Id, refusal) {
			t.Fatalf("%s was not refused for a bot that is not an admin", command)
		}

		settingsAntifloodPress(t, env, menuChat, menuMsgID, "a", "fd", "k", "m")
		if text, _ := env.lastEdit(menuChat.Id, menuMsgID); !strings.Contains(text, staffMarker("settings_duration_intro_tmute")) {
			t.Fatalf("fd drew %q, want the duration screen", text)
		}
		answers := env.answerCount()
		settingsAntifloodPress(t, env, menuChat, menuMsgID, press...)
		text, _ := env.lastEdit(menuChat.Id, menuMsgID)
		if !strings.Contains(text, refusal) || !strings.Contains(text, staffMarker("settings_antiflood_action")) {
			t.Errorf("the press drew %q, want the Antiflood page with the notice %s", text, refusal)
		}
		if got := env.answerCount() - answers; got != 1 {
			t.Errorf("the press answered %d times, want once", got)
		}
		for _, id := range []int64{commandChat.Id, menuChat.Id} {
			if _, has := settingsAntifloodRow(t, id); has {
				t.Errorf("a refused press or command wrote a row for chat %d", id)
			}
		}
	})

	t.Run("a plain member is refused by both", func(t *testing.T) {
		env := newSettingsEnv(t)
		chat := env.addChat()
		member := gotgbot.User{Id: uniqueLinkOwnerID() + 45, FirstName: "Plain"}
		t.Cleanup(func() {
			db.DB.Where("user_id IN ?", []int64{member.Id}).Delete(&models.User{})
			dbcache.DeleteCache(dbcache.CacheKey("user", member.Id))
			dbcache.ResetLocalForTest()
		})
		env.fake.setMember(chat.Id, member.Id, staffFakeMember{Status: gotgbot.ChatMemberStatusMember})
		env.sendIn(chat, member, command)

		for _, fields := range [][]string{press, {"a", "fd", "k", "m"}} {
			env.pressPrivate(member, 779, env.privateFields(chat.Id, fields...))
			answer, alert := env.lastAnswer()
			if !alert || !strings.Contains(answer, staffMarker("settings_not_admin")) {
				t.Errorf("%v: answer = %q (alert %v), want the alert settings_not_admin", fields, answer, alert)
			}
		}
		if _, has := settingsAntifloodRow(t, chat.Id); has {
			t.Error("a plain member's press or command wrote a row")
		}
	})

	t.Run("an administrator the owner gate refuses is refused by the menu only (the documented carve-out)", func(t *testing.T) {
		resetSettingsOwnerMemo(t)
		env := newSettingsEnv(t)
		commandChat, menuChat := env.addChat(), env.addChat()
		for _, chat := range []gotgbot.Chat{commandChat, menuChat} {
			env.newOwner(chat)
			env.setAdminsAllowed(chat.Id, false)
		}

		settingsAntifloodPress(t, env, menuChat, menuMsgID, press...)
		if answer, alert := env.lastAnswer(); !alert || !strings.Contains(answer, staffMarker("settings_gate_owner_only")) {
			t.Errorf("answer = %q (alert %v), want the alert settings_gate_owner_only", answer, alert)
		}
		if _, has := settingsAntifloodRow(t, menuChat.Id); has {
			t.Error("the gated press wrote a row")
		}
		env.sendIn(commandChat, env.admin, command)
		if row, has := settingsAntifloodRow(t, commandChat.Id); !has || !wrote(row) {
			t.Errorf("%s row = %+v (row %v), want the command to write", command, row, has)
		}
	})
}

// TestSettingsAntifloodTimedForged refuses every forged timed press in step 1, before any lookup, with nothing
// written.
func TestSettingsAntifloodTimedForged(t *testing.T) {
	const menuMsgID = 7070
	forged := []struct {
		name   string
		fields []string
	}{
		{"ft v 1800", []string{"a", "ft", "k", "m", "v", "1800"}},
		{"ft v 0", []string{"a", "ft", "k", "m", "v", "0"}},
		{"ft v 600x", []string{"a", "ft", "k", "m", "v", "600x"}},
		{"ft v 0600", []string{"a", "ft", "k", "b", "v", "0600"}},
		{"ft k x", []string{"a", "ft", "k", "x", "v", "3600"}},
		{"ft no k", []string{"a", "ft", "v", "3600"}},
		{"ft no v", []string{"a", "ft", "k", "m"}},
		{"ft an extra key", []string{"a", "ft", "k", "m", "v", "3600", "p", "f"}},
		{"fd k z", []string{"a", "fd", "k", "z"}},
		{"fd no k", []string{"a", "fd"}},
		{"fd an extra key", []string{"a", "fd", "k", "m", "v", "3600"}},
	}
	env := newSettingsEnv(t)
	for _, tc := range forged {
		t.Run(tc.name, func(t *testing.T) {
			data := encodeCallbackData(settingsCallbackNamespace, env.groupFields(env.admin, tc.fields...))
			if data == "" {
				t.Fatalf("forged press %v did not encode", tc.fields)
			}
			before := settingsMenuMarkOf(env)
			env.pressRaw(env.chat, env.admin, menuMsgID, data)
			settingsMenuWantRefused(t, env, before, "common_callback_invalid_request", true)
			if _, has := settingsAntifloodRow(t, env.chat.Id); has {
				t.Error("a forged press wrote a row")
			}
		})
	}
}
