//go:build testtools

package modules

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/advthresholds"
	dbcache "github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
)

// settingsPhase28Code is one action code Phase 28 added: its page and a valid private-menu press of it.
// Every Phase 28 action belongs to the private-only page av and requires the owner field c.
type settingsPhase28Code struct {
	code, page string
	valid      []string
}

func settingsPhase28Codes() []settingsPhase28Code {
	return []settingsPhase28Code{
		{settingsActAdvArea, settingsPageAdvanced, []string{"k", string(advthresholds.AreaRaidWindows)}},
		{settingsActAdvItem, settingsPageAdvanced, []string{"t", string(advthresholds.FieldRaidFloodWindow)}},
		{settingsActAdvSet, settingsPageAdvanced, []string{"t", string(advthresholds.FieldRaidFloodWindow), "v", "120"}},
		{settingsActAdvReset, settingsPageAdvanced, []string{"t", string(advthresholds.FieldRaidFloodWindow)}},
		{settingsActAdvResetArea, settingsPageAdvanced, []string{"k", string(advthresholds.AreaRaidWindows)}},
	}
}

// TestPhase28Codes verifies every Phase 28 action code (aa, ao, as, ar, ax on page av) maps to av,
// is no core code, leaves all 28 registered pages intact, and refuses both an extra key and a
// group-menu press (u instead of c) in step 1 with 0 getChatMember calls.
func TestPhase28Codes(t *testing.T) {
	codes := settingsPhase28Codes()

	pg, ok := settingsPageByKey(settingsPageAdvanced)
	if !ok {
		t.Fatalf("page %q is not registered", settingsPageAdvanced)
	}
	if !pg.PrivateOnly {
		t.Errorf("page %q PrivateOnly = false, want true", settingsPageAdvanced)
	}
	if len(pg.Actions) != len(codes) {
		t.Errorf("page %q registers %d action codes, test lists %d", settingsPageAdvanced, len(pg.Actions), len(codes))
	}

	for _, row := range codes {
		entry, ok := settingsActions[row.code]
		if !ok || entry.pageKey != row.page {
			t.Errorf("code %q maps to %+v (registered %v), want page %q", row.code, entry, ok, row.page)
		}
		if _, inPage := pg.Actions[row.code]; !inPage {
			t.Errorf("code %q missing from page %q Actions", row.code, row.page)
		}
		if slices.Contains(settingsCoreActions, row.code) {
			t.Errorf("code %q is a core code", row.code)
		}
	}

	allPages := []string{
		"s", "v", "l", "c", "f", "q", "n", "j", "k", "rs",
		"b", "i", "w", "g", "r", "e", "x", "h", "m", "av",
		"d", "t", "u", "z", "o", "y", "p", "a",
	}
	if len(allPages) != 28 {
		t.Fatalf("expected 28 registered pages in test list, got %d", len(allPages))
	}
	for _, key := range allPages {
		if _, ok := settingsPageByKey(key); !ok {
			t.Errorf("page %q is not registered", key)
		}
	}

	env := newPhase28SettingsEnv(t)
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

	for _, row := range codes {
		kv := append([]string{"a", row.code}, row.valid...)
		withExtra := append(slices.Clone(kv), "x", "1")
		refused(row.code+" with an extra key", func() {
			env.pressPrivate(owner, privateMsg, env.privateFields(env.chat.Id, withExtra...))
		})
		refused(row.code+" from a group menu", func() {
			env.press(owner, groupMsg, env.groupFields(owner, kv...))
		})
	}
}

// TestPhase28ReadsCreateNothing verifies opening Main, av, all 6 area screens (rw, nm, ab, mr, lr, ct),
// and all 12 item screens on an unconfigured chat (private and group menus) creates 0 rows in
// chat_advanced_thresholds and chats.
func TestPhase28ReadsCreateNothing(t *testing.T) {
	tables := []struct {
		name  string
		model any
	}{
		{"chat_advanced_thresholds", &models.ChatAdvancedThresholds{}},
		{"chats", &models.Chat{}},
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

	env := newPhase28SettingsEnv(t)
	t.Cleanup(func() {
		db.DB.Where("chat_id = ?", env.chat.Id).Delete(&models.Chat{})
	})

	owner := env.newOwner(env.chat)
	privateMsg := settingsPrivateOnlyMessage(t, env, owner)
	groupMsg := env.openMenu(owner)

	private := func(kv ...string) { env.pressPrivate(owner, privateMsg, env.privateFields(env.chat.Id, kv...)) }
	group := func(kv ...string) { env.press(owner, groupMsg, env.groupFields(owner, kv...)) }

	group("a", settingsActOpen, "p", settingsPageAdvanced) // private-only group note
	group("a", settingsActBack)

	private("a", settingsActOpen, "p", settingsPageAdvanced)
	for _, area := range settingsAdvAreas {
		private("a", settingsActAdvArea, "k", string(area.Area))
		for _, item := range settingsAdvItemsForArea(area.Area) {
			private("a", settingsActAdvItem, "t", string(item.Field))
		}
		private("a", settingsActOpen, "p", settingsPageAdvanced)
	}
	private("a", settingsActBack)

	if got := rows(env.chat.Id); len(got) != 0 {
		t.Errorf("opening Phase 28 screens created rows %v, want none", got)
	}
}

type phase28LogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *phase28LogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *phase28LogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// TestPhase28StaffPrivacyAndLogs links a group to a Staff Group with sentinel title
// "SecretStaffSentinel28", exercises opening av, setting thresholds, resetting an item,
// resetting an area, and forcing a DB read error; asserts (a) no rendered screen/answer/button
// ever contains "SecretStaffSentinel28", (b) no captured logrus line contains "SecretStaffSentinel28"
// or any group/user title, and (c) av never appears in a Staff Group's Main (group or private).
func TestPhase28StaffPrivacyAndLogs(t *testing.T) {
	const (
		sentinelStaffTitle = "SecretStaffSentinel28"
		sentinelGroupTitle = "PublicGroupSentinel28"
		sentinelModName    = "ModeratorSentinel28"
	)

	env := newPhase28SettingsEnv(t)
	env.chat.Title = sentinelGroupTitle

	staffChat := settingsStaffTestOwnedStaffGroup(env, sentinelStaffTitle)
	linkLockdownGroupTo(env.lockdownEnv, staffChat)

	moderator := gotgbot.User{Id: uniqueLinkOwnerID(), FirstName: sentinelModName}
	t.Cleanup(func() {
		db.DB.Where("user_id = ?", moderator.Id).Delete(&models.User{})
		dbcache.DeleteCache(dbcache.CacheKey("user", moderator.Id))
		dbcache.ResetLocalForTest()
	})
	env.fake.setMemberRaw(env.chat.Id, moderator.Id, settingsPrivacyPhase19AdminJSON(moderator.Id))
	env.fake.listAdministrator(env.chat.Id, moderator.Id)

	// Capture every logrus line across the entire run.
	logger := log.StandardLogger()
	prevOut := logger.Out
	prevLevel := logger.GetLevel()
	logBuf := &phase28LogBuffer{}
	logger.SetOutput(logBuf)
	logger.SetLevel(log.DebugLevel)
	t.Cleanup(func() {
		logger.SetOutput(prevOut)
		logger.SetLevel(prevLevel)
	})

	msgID := settingsPrivateOnlyMessage(t, env, moderator)
	press := func(kv ...string) { env.pressPrivate(moderator, msgID, env.privateFields(env.chat.Id, kv...)) }

	// Exercise page av: open hub, area, item, set thresholds, reset item, reset area, return to Main.
	press("a", settingsActOpen, "p", settingsPageAdvanced)
	press("a", settingsActAdvArea, "k", string(advthresholds.AreaRaidWindows))
	press("a", settingsActAdvItem, "t", string(advthresholds.FieldRaidFloodWindow))
	press("a", settingsActAdvSet, "t", string(advthresholds.FieldRaidFloodWindow), "v", "120")
	press("a", settingsActAdvReset, "t", string(advthresholds.FieldRaidFloodWindow))
	press("a", settingsActAdvSet, "t", string(advthresholds.FieldRaidJoinWindow), "v", "15")
	press("a", settingsActAdvSet, "t", string(advthresholds.FieldRaidFloodWindow), "v", "120")
	press("a", settingsActAdvResetArea, "k", string(advthresholds.AreaRaidWindows))
	press("a", settingsActOpen, "p", settingsPageAdvanced)
	press("a", settingsActBack)

	// Force a DB read error on av / aa / ao so error logging paths are exercised too.
	origRead := settingsAdvRead
	settingsAdvRead = func(_ context.Context, _ int64) (advthresholds.Thresholds, error) {
		return advthresholds.Thresholds{}, errors.New("simulated advthresholds read error")
	}
	t.Cleanup(func() { settingsAdvRead = origRead })
	press("a", settingsActOpen, "p", settingsPageAdvanced)
	press("a", settingsActAdvArea, "k", string(advthresholds.AreaRaidWindows))
	press("a", settingsActAdvItem, "t", string(advthresholds.FieldRaidFloodWindow))
	settingsAdvRead = origRead

	// (a) Collect everything shown to the non-creator moderator and in the group: must never name the Staff Group.
	var seen []string
	for _, edit := range env.edits(moderator.Id, msgID) {
		seen = append(seen, fmt.Sprint(edit.Params["text"]))
		for _, button := range settingsAllButtons(staffKeyboardOf(edit.Params["reply_markup"])) {
			seen = append(seen, button.Text, button.CallbackData, button.Url)
		}
	}
	for _, sent := range env.fake.sentTo(moderator.Id) {
		seen = append(seen, fmt.Sprint(sent.Params["text"]))
		for _, button := range settingsAllButtons(staffKeyboardOf(sent.Params["reply_markup"])) {
			seen = append(seen, button.Text, button.CallbackData, button.Url)
		}
	}
	for _, sent := range env.fake.sentTo(env.chat.Id) {
		seen = append(seen, fmt.Sprint(sent.Params["text"]))
		for _, button := range settingsAllButtons(staffKeyboardOf(sent.Params["reply_markup"])) {
			seen = append(seen, button.Text, button.CallbackData, button.Url)
		}
	}
	for _, answer := range env.fake.callsFor("answerCallbackQuery") {
		seen = append(seen, fmt.Sprint(answer.Params["text"]))
	}
	everything := strings.Join(seen, "\n")

	for _, want := range []string{
		"settings_adv_intro",
		"settings_adv_area_intro_rw",
		"settings_adv_desc_flw",
		"settings_adv_saved",
		"settings_adv_reset_item_done",
		"settings_adv_reset_area_done",
	} {
		if !containsMarker(everything, want) {
			t.Errorf("privacy run never produced %s", want)
		}
	}
	staffDigits := strings.TrimPrefix(strconv.FormatInt(staffChat.Id, 10), "-")
	for _, leak := range []string{
		sentinelStaffTitle,
		html.EscapeString(sentinelStaffTitle),
		staffDigits,
	} {
		if strings.Contains(everything, leak) {
			t.Errorf("Phase 28 run leaked Staff Group %q into UI output", leak)
		}
	}

	// (b) Captured logrus output must never contain the Staff Group title, group title, or user name.
	logs := logBuf.String()
	if !strings.Contains(logs, "simulated advthresholds read error") {
		t.Errorf("expected forced read error to be logged so log capture is proven active; logs:\n%s", logs)
	}
	for _, secret := range []string{
		sentinelStaffTitle,
		sentinelGroupTitle,
		sentinelModName,
	} {
		if strings.Contains(logs, secret) {
			t.Errorf("captured logrus output leaked %q:\n%s", secret, logs)
		}
	}

	// (c) Page av must never appear in a Staff Group's Main menu (private or group).
	staffOwner := env.newOwner(staffChat)
	env.sendPrivate(staffOwner, fmt.Sprintf("/start settings_%d", staffChat.Id))
	staffPrivateMain := settingsPrivateOnlySent(t, env, staffOwner)
	checkStaffMain := func(where, text string, kb [][]gotgbot.InlineKeyboardButton) {
		t.Helper()
		if containsMarker(text, "settings_page_advanced") {
			t.Errorf("%s Staff Group Main text unexpectedly contains settings_page_advanced:\n%s", where, text)
		}
		for _, btn := range settingsAllButtons(kb) {
			if !strings.HasPrefix(btn.CallbackData, settingsCallbackNamespace+"|") {
				continue
			}
			fields := settingsButtonFields(t, btn)
			if fields["a"] == settingsActOpen && fields["p"] == settingsPageAdvanced {
				t.Errorf("%s Staff Group Main unexpectedly has button for page av: %+v", where, btn)
			}
		}
	}
	checkStaffMain("private", fmt.Sprint(staffPrivateMain.Params["text"]), staffKeyboardOf(staffPrivateMain.Params["reply_markup"]))

	env.fake.setBotRole(staffChat.Id, staffRoleAdmin)
	beforeStaffSent := len(env.fake.sentTo(staffChat.Id))
	staffSettingsMsg := gotgbot.Message{
		MessageId: env.updateID(),
		Chat:      staffChat,
		From:      &staffOwner,
		Text:      "/settings",
	}
	if err := env.dispatcher.ProcessUpdate(env.bot, &gotgbot.Update{
		UpdateId: env.updateID(),
		Message:  &staffSettingsMsg,
	}, nil); err != nil {
		t.Fatalf("Staff Group /settings error: %v", err)
	}
	staffSent := env.fake.sentTo(staffChat.Id)
	if len(staffSent) <= beforeStaffSent {
		t.Fatalf("Staff Group /settings sent no message")
	}
	lastStaff := staffSent[len(staffSent)-1]
	checkStaffMain("group", fmt.Sprint(lastStaff.Params["text"]), staffKeyboardOf(lastStaff.Params["reply_markup"]))
}
