//go:build testtools

package modules

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
)

// settingsPhase26Code is one action code Phase 26 added: its page, whether it exists in the private menu only (it
// needs the owner field c) and a valid press of it.
type settingsPhase26Code struct {
	code, page string
	private    bool
	valid      []string
}

var settingsPhase26Codes = []settingsPhase26Code{
	{settingsActWarnsDuration, settingsPageWarns, false, []string{"k", "m"}},
	{settingsActWarnsTimed, settingsPageWarns, false, []string{"k", "m", "v", "3600"}},
	{settingsActWarnsExpiryOpen, settingsPageWarns, false, nil},
	{settingsActWarnsExpiry, settingsPageWarns, false, []string{"v", "7"}},
	{settingsActFloodDuration, settingsPageAntiflood, false, []string{"k", "m"}},
	{settingsActFloodTimed, settingsPageAntiflood, false, []string{"k", "m", "v", "3600"}},
	{settingsActBlacklistDuration, "b", true, []string{"k", "m"}},
	{settingsActBlacklistTimed, "b", true, []string{"k", "m", "v", "3600"}},
	{settingsActEscToggle, settingsPageEscalation, true, []string{"v", "1"}},
	{settingsActEscForget, settingsPageEscalation, true, []string{"v", "7"}},
	{settingsActEscStep, settingsPageEscalation, true, []string{"t", "1"}},
	{settingsActEscSet, settingsPageEscalation, true, []string{"t", "1", "v", "ban"}},
	{settingsActEscDuration, settingsPageEscalation, true, []string{"t", "1", "k", "m"}},
	{settingsActEscTimed, settingsPageEscalation, true, []string{"t", "1", "k", "m", "v", "3600"}},
	{settingsActEscRemove, settingsPageEscalation, true, []string{"t", "1"}},
}

// TestPhase26Codes: every code of the phase belongs to exactly the page that declares it and is no core code, a
// valid press with one extra key is refused in step 1 with no getChatMember, and a private-only code pressed from a
// group menu (u in place of c) is refused in step 1 too.
func TestPhase26Codes(t *testing.T) {
	for _, row := range settingsPhase26Codes {
		entry, ok := settingsActions[row.code]
		if !ok || entry.pageKey != row.page {
			t.Errorf("code %q maps to %+v (registered %v), want page %q", row.code, entry, ok, row.page)
		}
		if slices.Contains(settingsCoreActions, row.code) {
			t.Errorf("code %q is a core code", row.code)
		}
	}
	if _, ok := settingsPageByKey(settingsPageEscalation); !ok {
		t.Error("the Escalation page is not registered")
	}
	for _, key := range []string{"s", "v", "l", "c", "f", "n", "k", "b", "i", "w", "g", "r", "e", "x", "h", "m", "d", "t", "u", "z", "o", "y", "p", "a"} {
		if _, ok := settingsPageByKey(key); !ok {
			t.Errorf("page %q is no longer registered", key)
		}
	}

	env := newSettingsEnv(t)
	owner := env.newOwner(env.chat)
	privateMsg := settingsPrivateOnlyMessage(t, env, owner)
	groupMsg := env.openMenu(owner)
	lookups := func() int { return len(callsToChat(env.fake.staffBotClient, "getChatMember", env.chat.Id)) }
	refused := func(name string, press func()) {
		t.Helper()
		before := lookups()
		press()
		if answer, alert := env.lastAnswer(); !alert || !strings.Contains(answer, staffMarker("common_callback_invalid_request")) {
			t.Errorf("%s answered %q (alert %v), want the invalid-request alert", name, answer, alert)
		}
		if got := lookups(); got != before {
			t.Errorf("%s made %d getChatMember calls, want 0 (refused in step 1)", name, got-before)
		}
	}
	for _, row := range settingsPhase26Codes {
		kv := append([]string{"a", row.code}, row.valid...)
		withExtra := append(slices.Clone(kv), "x", "1")
		if row.private {
			refused(row.code+" with an extra key", func() { env.pressPrivate(owner, privateMsg, env.privateFields(env.chat.Id, withExtra...)) })
			refused(row.code+" from a group menu", func() { env.press(owner, groupMsg, env.groupFields(owner, kv...)) })
			continue
		}
		refused(row.code+" with an extra key", func() { env.press(owner, groupMsg, env.groupFields(owner, withExtra...)) })
	}
}

// settingsPrivateOnlyMessage opens the private menu of env's group for user and returns its message ID.
func settingsPrivateOnlyMessage(t *testing.T, env *settingsEnv, user gotgbot.User) int64 {
	t.Helper()
	env.sendPrivate(user, fmt.Sprintf("/start settings_%d", env.chat.Id))
	return settingsPrivateOnlySent(t, env, user).MessageID
}

// TestPhase26ReadsCreateNothing: opening Main and every screen of the phase on a chat with no rows, in a private menu
// and, for the pages that exist there, a group menu, creates no row in the phase's tables and no chats row.
func TestPhase26ReadsCreateNothing(t *testing.T) {
	tables := []struct {
		name  string
		model any
	}{
		{"warns_settings", &models.WarnSettings{}},
		{"antiflood_settings", &models.AntifloodSettings{}},
		{"blacklists", &models.BlacklistSettings{}},
		{"chat_escalation", &models.ChatEscalation{}},
		{"chat_offences", &models.ChatOffence{}},
	}
	rows := func(chatID int64) map[string]int64 {
		out := map[string]int64{}
		for _, table := range tables {
			var n int64
			if err := db.DB.Model(table.model).Where("chat_id = ?", chatID).Count(&n).Error; err != nil {
				t.Fatalf("count %s: %v", table.name, err)
			}
			if n != 0 {
				out[table.name] = n
			}
		}
		return out
	}

	env := newSettingsEnv(t)
	t.Cleanup(func() {
		for _, table := range tables {
			db.DB.Where("chat_id = ?", env.chat.Id).Delete(table.model)
		}
	})
	owner := env.newOwner(env.chat)
	privateMsg := settingsPrivateOnlyMessage(t, env, owner)
	groupMsg := env.openMenu(owner)

	private := func(kv ...string) { env.pressPrivate(owner, privateMsg, env.privateFields(env.chat.Id, kv...)) }
	group := func(kv ...string) { env.press(owner, groupMsg, env.groupFields(owner, kv...)) }

	group("a", "op", "p", settingsPageWarns)
	group("a", settingsActWarnsDuration, "k", "m")
	group("a", settingsActWarnsExpiryOpen)
	group("a", "op", "p", settingsPageAntiflood)
	group("a", settingsActFloodDuration, "k", "b")
	group("a", "op", "p", settingsPageEscalation) // the group note
	private("a", "op", "p", settingsPageWarns)
	private("a", settingsActWarnsDuration, "k", "b")
	private("a", settingsActWarnsExpiryOpen)
	private("a", "op", "p", settingsPageAntiflood)
	private("a", settingsActFloodDuration, "k", "m")
	private("a", "op", "p", "b")
	private("a", settingsActBlacklistDuration, "k", "m")
	private("a", "op", "p", settingsPageEscalation)
	private("a", settingsActEscStep, "t", "1")
	private("a", settingsActEscDuration, "t", "1", "k", "b")
	private("a", "bk")

	if got := rows(env.chat.Id); len(got) != 0 {
		t.Errorf("opening the screens created rows %v", got)
	}
	var chats int64
	if err := db.DB.Model(&models.Chat{}).Where("chat_id = ?", env.chat.Id).Count(&chats).Error; err != nil {
		t.Fatal(err)
	}
	if chats != 0 {
		t.Errorf("opening the screens created %d chats rows", chats)
	}
}
