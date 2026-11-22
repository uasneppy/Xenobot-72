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

	"github.com/uasneppy/fuku_robot/fuku/db"
	dbcache "github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/db/warns"
)

// settingsWarnsExpiryLineOf is the marker form of the expiry line for a stored expiry: Off, or the unit key and
// amount punishDurationLabel picks.
func settingsWarnsExpiryLineOf(days int) string {
	line := staffMarker("settings_warns_expiry_line") + " "
	switch {
	case days == 0:
		return line + staffMarker("settings_state_off_plain")
	case days%7 == 0:
		return line + staffMarker("staff_act_duration_weeks") + " " + strconv.Itoa(days/7)
	}
	return line + staffMarker("staff_act_duration_days") + " " + strconv.Itoa(days)
}

// settingsWarnsExpiryPresetButton returns the expiry screen's button that presses days.
func settingsWarnsExpiryPresetButton(t *testing.T, keyboard [][]gotgbot.InlineKeyboardButton, days int) gotgbot.InlineKeyboardButton {
	t.Helper()
	for _, button := range settingsAllButtons(keyboard) {
		if fields := settingsButtonFields(t, button); fields["a"] == settingsActWarnsExpiry && fields["v"] == strconv.Itoa(days) {
			return button
		}
	}
	t.Fatalf("the expiry screen has no %d day preset: %+v", days, keyboard)
	return gotgbot.InlineKeyboardButton{}
}

// settingsWarnsExpiryMember is a fresh member whose warns, users row and cache keys in chatID are deleted when the
// test ends.
func settingsWarnsExpiryMember(t *testing.T, chatID int64) gotgbot.User {
	t.Helper()
	member := gotgbot.User{Id: uniqueLinkOwnerID() + 47, FirstName: "Warned"}
	t.Cleanup(func() {
		db.DB.Where("chat_id = ?", chatID).Delete(&models.Warns{})
		db.DB.Where("user_id IN ?", []int64{member.Id}).Delete(&models.User{})
		dbcache.DeleteCache(dbcache.CacheKey("warns", member.Id, chatID))
		dbcache.DeleteCache(dbcache.CacheKey("user", member.Id))
		dbcache.ResetLocalForTest()
	})
	return member
}

// settingsWarnsExpiryWarnAt stores one warn of member in chatID dated at when, as /warn at that time would.
func settingsWarnsExpiryWarnAt(t *testing.T, chatID int64, member gotgbot.User, reason string, when time.Time) {
	t.Helper()
	if _, _, err := warns.WarnUserAt(context.Background(), member.Id, chatID, reason, when); err != nil {
		t.Fatalf("WarnUserAt(%q) error = %v", reason, err)
	}
}

// settingsWarnsExpiryWarns runs /warns as the env's administrator in reply to member, through the command's own
// handler, and returns its reply.
func settingsWarnsExpiryWarns(t *testing.T, env *settingsEnv, chat gotgbot.Chat, member gotgbot.User) string {
	t.Helper()
	msg := &gotgbot.Message{
		MessageId: 101, Date: 1, Chat: chat, From: &env.admin, Text: "/warns",
		ReplyToMessage: &gotgbot.Message{MessageId: 100, Date: 1, Chat: chat, From: &member, Text: "hi"},
	}
	before := len(env.fake.sentTo(chat.Id))
	_ = warnsModule.warns(env.bot, ext.NewContext(env.bot, &gotgbot.Update{UpdateId: 1, Message: msg}, nil))
	sent := env.fake.sentTo(chat.Id)
	if len(sent) != before+1 {
		t.Fatalf("/warns sent %d messages, want 1", len(sent)-before)
	}
	return fmt.Sprint(sent[len(sent)-1].Params["text"])
}

// TestSettingsWarnsExpiry is the expiry control end to end (TUNE-03, D-38): the page reads Off, the button opens the
// expiry screen with Off marked, 7 days is stored and read back as 1 week, /warns then counts only the warn of the
// last 7 days, and Off makes every warn count again.
func TestSettingsWarnsExpiry(t *testing.T) {
	const menuMsgID = 6767
	env := newSettingsEnv(t)
	settingsWarnsCleanup(t, env.chat.Id)
	now := time.Unix(1_900_000_000, 0)
	mreqSwap(t, &punishNow, func() time.Time { return now })
	member := settingsWarnsExpiryMember(t, env.chat.Id)
	settingsWarnsExpiryWarnAt(t, env.chat.Id, member, "old", now.Add(-8*24*time.Hour))
	settingsWarnsExpiryWarnAt(t, env.chat.Id, member, "new", now)
	settingsWarnsCleanup(t, env.chat.Id) // the warns created a default settings row: start from none

	text, keyboard := settingsWarnsOpenPage(env, env.chat, menuMsgID)
	if !strings.Contains(text, settingsWarnsExpiryLineOf(0)) {
		t.Errorf("the page %q does not read the expiry Off", text)
	}
	if widths := settingsWarnsWidths(keyboard); fmt.Sprint(widths) != "[5 3 2 1 2]" {
		t.Fatalf("Warnings page widths = %v, want [5 3 2 1 2]", widths)
	}
	expiry := keyboard[3][0]
	if !containsMarker(expiry.Text, "settings_btn_warns_expiry") {
		t.Errorf("expiry button = %q, want settings_btn_warns_expiry", expiry.Text)
	}
	if !containsMarker(expiry.Text, "settings_state_off_plain") {
		t.Errorf("expiry button = %q, want it to read Off", expiry.Text)
	}
	if fields := settingsButtonFields(t, expiry); fields["a"] != "ne" || len(fields) != 2 {
		t.Errorf("expiry button fields = %v, want a=ne and the opener", fields)
	}

	env.pressRaw(env.chat, env.admin, menuMsgID, expiry.CallbackData)
	text, keyboard = env.lastEdit(env.chat.Id, menuMsgID)
	for _, want := range []string{
		staffMarker("settings_page_warns"), staffMarker("settings_warns_expiry_intro"),
		staffMarker("settings_warns_expiry_hint"), settingsWarnsExpiryLineOf(0),
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the expiry screen %q does not contain %q", text, want)
		}
	}
	if widths := settingsWarnsWidths(keyboard); fmt.Sprint(widths) != "[2 2 2 1 2]" {
		t.Errorf("expiry screen widths = %v, want [2 2 2 1 2]", widths)
	}
	if marked := settingsWarnsMarked(settingsAllButtons(keyboard)); len(marked) != 1 || !containsMarker(marked[0], "settings_state_off_plain") {
		t.Errorf("the expiry screen marks %v, want Off alone", marked)
	}
	if back := keyboard[4][0]; settingsButtonFields(t, back)["a"] != settingsActOpen || settingsButtonFields(t, back)["p"] != "n" {
		t.Errorf("Back fields = %v, want the Warnings page", settingsButtonFields(t, back))
	}
	if _, has := settingsWarnsRow(t, env.chat.Id); has {
		t.Error("opening the page and the expiry screen wrote a row")
	}

	if reply := settingsWarnsExpiryWarns(t, env, env.chat, member); !strings.Contains(reply, "- old") || !strings.Contains(reply, "- new") {
		t.Fatalf("/warns with expiry off = %q, want both warns", reply)
	}

	answers := env.answerCount()
	env.pressRaw(env.chat, env.admin, menuMsgID, settingsWarnsExpiryPresetButton(t, keyboard, 7).CallbackData)
	if got := env.answerCount() - answers; got != 1 {
		t.Errorf("the 7 day press answered %d times, want once", got)
	}
	if row, has := settingsWarnsRow(t, env.chat.Id); !has || row.ExpiryDays != 7 || row.WarnLimit != 3 || row.WarnMode != "mute" {
		t.Fatalf("row = %+v (row %v), want expiry 7 with limit 3 and mute", row, has)
	}
	text, keyboard = env.lastEdit(env.chat.Id, menuMsgID)
	if !strings.Contains(text, settingsWarnsExpiryLineOf(7)) || strings.Contains(text, "⚠️") {
		t.Errorf("the page redrew %q, want the expiry line reading 1 week and no notice", text)
	}
	if label := keyboard[3][0].Text; !strings.Contains(label, staffMarker("staff_act_duration_weeks")+" 1") {
		t.Errorf("expiry button = %q, want it to read 1 week", label)
	}
	if reply := settingsWarnsExpiryWarns(t, env, env.chat, member); strings.Contains(reply, "- old") || !strings.Contains(reply, "- new") {
		t.Errorf("/warns with expiry 7 = %q, want the new warn alone", reply)
	}

	env.pressRaw(env.chat, env.admin, menuMsgID, keyboard[3][0].CallbackData)
	_, keyboard = env.lastEdit(env.chat.Id, menuMsgID)
	if marked := settingsWarnsMarked(settingsAllButtons(keyboard)); len(marked) != 1 ||
		!strings.Contains(marked[0], staffMarker("staff_act_duration_weeks")+" 1") {
		t.Errorf("the expiry screen marks %v, want the 1 week preset", marked)
	}
	answers = env.answerCount()
	env.pressRaw(env.chat, env.admin, menuMsgID, settingsWarnsExpiryPresetButton(t, keyboard, 0).CallbackData)
	if got := env.answerCount() - answers; got != 1 {
		t.Errorf("the Off press answered %d times, want once", got)
	}
	if row, _ := settingsWarnsRow(t, env.chat.Id); row.ExpiryDays != 0 {
		t.Errorf("row expiry = %d after Off, want 0", row.ExpiryDays)
	}
	if text, _ := env.lastEdit(env.chat.Id, menuMsgID); !strings.Contains(text, settingsWarnsExpiryLineOf(0)) {
		t.Errorf("the page redrew %q, want the expiry Off", text)
	}
	if reply := settingsWarnsExpiryWarns(t, env, env.chat, member); !strings.Contains(reply, "- old") || !strings.Contains(reply, "- new") {
		t.Errorf("/warns after Off = %q, want both warns again", reply)
	}
}

// TestSettingsDriftWarnExpiry holds the page to an expiry stored outside it and the warn limit to the expiry the
// page stored. There is no command, so this and the authority test stand in for a parity test (D-38).
func TestSettingsDriftWarnExpiry(t *testing.T) {
	const menuMsgID = 6868

	t.Run("a repository write is what the page and the screen draw", func(t *testing.T) {
		env := newSettingsEnv(t)
		chat := env.addChat()
		settingsWarnsCleanup(t, chat.Id)
		if err := warns.SetWarnExpiryContext(context.Background(), chat.Id, 30); err != nil {
			t.Fatalf("SetWarnExpiryContext(30) error = %v", err)
		}
		text, keyboard := settingsWarnsOpenPage(env, chat, menuMsgID)
		if !strings.Contains(text, settingsWarnsExpiryLineOf(30)) {
			t.Errorf("the page shows %q, want the expiry 30 days", text)
		}
		env.pressRaw(chat, env.admin, menuMsgID, keyboard[3][0].CallbackData)
		_, screen := env.lastEdit(chat.Id, menuMsgID)
		if marked := settingsWarnsMarked(settingsAllButtons(screen)); len(marked) != 1 ||
			!strings.Contains(marked[0], staffMarker("staff_act_duration_days")+" 30") {
			t.Errorf("the expiry screen marks %v, want the 30 day preset", marked)
		}
	})

	t.Run("an expiry that is no preset is drawn and marks nothing", func(t *testing.T) {
		env := newSettingsEnv(t)
		chat := env.addChat()
		settingsWarnsCleanup(t, chat.Id)
		if err := warns.SetWarnExpiryContext(context.Background(), chat.Id, 5); err != nil {
			t.Fatalf("SetWarnExpiryContext(5) error = %v", err)
		}
		settingsWarnsPress(t, env, chat, menuMsgID, "a", "ne")
		text, screen := env.lastEdit(chat.Id, menuMsgID)
		if !strings.Contains(text, settingsWarnsExpiryLineOf(5)) {
			t.Errorf("the expiry screen %q does not draw 5 days", text)
		}
		if marked := settingsWarnsMarked(settingsAllButtons(screen)); len(marked) != 0 {
			t.Errorf("the expiry screen marks %v for 5 days, want nothing", marked)
		}
	})

	t.Run("the page's expiry is what the warn limit counts", func(t *testing.T) {
		env := newSettingsEnv(t)
		chat := env.addChat()
		settingsWarnsCleanup(t, chat.Id)
		now := time.Unix(1_900_000_000, 0)
		mreqSwap(t, &punishNow, func() time.Time { return now })
		member := settingsWarnsExpiryMember(t, chat.Id)
		if err := warns.SetWarnLimit(chat.Id, 2); err != nil {
			t.Fatalf("SetWarnLimit() error = %v", err)
		}
		settingsWarnsExpiryWarnAt(t, chat.Id, member, "two days ago", now.Add(-2*24*time.Hour))
		settingsWarnsPress(t, env, chat, menuMsgID, "a", "nx", "v", "1")
		if row, _ := settingsWarnsRow(t, chat.Id); row.ExpiryDays != 1 || row.WarnLimit != 2 {
			t.Fatalf("row = %+v, want expiry 1 and limit 2", row)
		}

		res, err := applyWarn(context.Background(), env.bot, &chat, member.Id, "spam", nil)
		if err != nil || res.LimitReached || res.Count != 1 {
			t.Fatalf("applyWarn() = %+v, %v, want 1 counted warn and the limit not reached", res, err)
		}
		for _, call := range env.fake.callsFor("restrictChatMember") {
			if staffParamInt(call.Params, "user_id") == member.Id {
				t.Errorf("the member was restricted although the expired warn does not count: %+v", call.Params)
			}
		}
	})

	t.Run("opening the page and the screen on a chat with no row writes nothing", func(t *testing.T) {
		env := newSettingsEnv(t)
		chat := env.addChat()
		settingsWarnsCleanup(t, chat.Id)
		settingsWarnsOpenPage(env, chat, menuMsgID)
		settingsWarnsPress(t, env, chat, menuMsgID, "a", "ne")
		if text, _ := env.lastEdit(chat.Id, menuMsgID); !strings.Contains(text, staffMarker("settings_warns_expiry_intro")) {
			t.Fatalf("ne drew %q, want the expiry screen", text)
		}
		if _, has := settingsWarnsRow(t, chat.Id); has {
			t.Error("opening the page and the expiry screen wrote a row")
		}
	})
}

// TestSettingsAuthorityWarnExpiry holds the expiry press to the right of the warn commands (an admin and the bot as
// an administrator, judged live) and the expiry screen to the live admin check alone.
func TestSettingsAuthorityWarnExpiry(t *testing.T) {
	const menuMsgID = 6969
	press := []string{"a", "nx", "v", "7"}
	open := []string{"a", "ne"}

	t.Run("a live administrator writes", func(t *testing.T) {
		env := newSettingsEnv(t)
		chat := env.addChat()
		settingsWarnsCleanup(t, chat.Id)
		settingsWarnsPress(t, env, chat, menuMsgID, press...)
		if row, has := settingsWarnsRow(t, chat.Id); !has || row.ExpiryDays != 7 {
			t.Errorf("row = %+v (row %v), want expiry 7", row, has)
		}
	})

	t.Run("a bot that is not an admin refuses the press, and the screen still opens", func(t *testing.T) {
		env := newSettingsEnv(t)
		chat := env.addChat()
		settingsWarnsCleanup(t, chat.Id)
		env.fake.setBotRole(chat.Id, staffRoleMember)
		settingsWarnsPress(t, env, chat, menuMsgID, open...)
		if text, _ := env.lastEdit(chat.Id, menuMsgID); !strings.Contains(text, staffMarker("settings_warns_expiry_intro")) {
			t.Fatalf("ne drew %q, want the expiry screen", text)
		}
		answers := env.answerCount()
		settingsWarnsPress(t, env, chat, menuMsgID, press...)
		text, _ := env.lastEdit(chat.Id, menuMsgID)
		if !strings.Contains(text, staffMarker("connections_is_user_connected_bot_not_admin")) ||
			!strings.Contains(text, staffMarker("settings_warns_limit")) {
			t.Errorf("the press drew %q, want the Warnings page with the bot-not-admin notice", text)
		}
		if got := env.answerCount() - answers; got != 1 {
			t.Errorf("the press answered %d times, want once", got)
		}
		if _, has := settingsWarnsRow(t, chat.Id); has {
			t.Error("a refused press wrote a row")
		}
	})

	t.Run("a plain member is refused", func(t *testing.T) {
		env := newSettingsEnv(t)
		chat := env.addChat()
		settingsWarnsCleanup(t, chat.Id)
		member := gotgbot.User{Id: uniqueLinkOwnerID() + 48, FirstName: "Plain"}
		t.Cleanup(func() {
			db.DB.Where("user_id IN ?", []int64{member.Id}).Delete(&models.User{})
			dbcache.DeleteCache(dbcache.CacheKey("user", member.Id))
			dbcache.ResetLocalForTest()
		})
		env.fake.setMember(chat.Id, member.Id, staffFakeMember{Status: gotgbot.ChatMemberStatusMember})
		for _, fields := range [][]string{press, open} {
			env.pressPrivate(member, 779, env.privateFields(chat.Id, fields...))
			answer, alert := env.lastAnswer()
			if !alert || !strings.Contains(answer, staffMarker("settings_not_admin")) {
				t.Errorf("%v: answer = %q (alert %v), want the alert settings_not_admin", fields, answer, alert)
			}
		}
		if _, has := settingsWarnsRow(t, chat.Id); has {
			t.Error("a plain member's press wrote a row")
		}
	})

	t.Run("an administrator the owner gate refuses is refused at step 4b", func(t *testing.T) {
		resetSettingsOwnerMemo(t)
		env := newSettingsEnv(t)
		chat := env.addChat()
		settingsWarnsCleanup(t, chat.Id)
		env.newOwner(chat)
		env.setAdminsAllowed(chat.Id, false)
		for _, fields := range [][]string{press, open} {
			settingsWarnsPress(t, env, chat, menuMsgID, fields...)
			if answer, alert := env.lastAnswer(); !alert || !strings.Contains(answer, staffMarker("settings_gate_owner_only")) {
				t.Errorf("%v: answer = %q (alert %v), want the alert settings_gate_owner_only", fields, answer, alert)
			}
		}
		if _, has := settingsWarnsRow(t, chat.Id); has {
			t.Error("the gated press wrote a row")
		}
	})
}

// TestSettingsWarnsExpiryForged refuses every forged expiry press in step 1, before any lookup, with nothing
// written.
func TestSettingsWarnsExpiryForged(t *testing.T) {
	const menuMsgID = 7070
	forged := []struct {
		name   string
		fields []string
	}{
		{"nx v 2", []string{"a", "nx", "v", "2"}},
		{"nx v 366", []string{"a", "nx", "v", "366"}},
		{"nx v 007", []string{"a", "nx", "v", "007"}},
		{"nx v -1", []string{"a", "nx", "v", "-1"}},
		{"nx v empty", []string{"a", "nx", "v", ""}},
		{"nx no v", []string{"a", "nx"}},
		{"nx an extra key", []string{"a", "nx", "v", "7", "k", "m"}},
		{"ne an extra key", []string{"a", "ne", "v", "7"}},
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
