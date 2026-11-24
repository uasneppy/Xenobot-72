//go:build testtools

package modules

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/fuku_robot/fuku/db"
	dbcache "github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/db/modtuning"
)

// settingsPhase27Code is one action code Phase 27 added: its page and a valid private-menu press of it.
// Every Phase 27 action belongs to a private-only page (q or rs) and requires the owner field c.
type settingsPhase27Code struct {
	code, page string
	valid      []string
}

func settingsPhase27Codes(minute string) []settingsPhase27Code {
	return []settingsPhase27Code{
		{settingsActModDefDur, settingsPageModDefaults, []string{"k", settingsModDefKindBan}},
		{settingsActModDefSetDur, settingsPageModDefaults, []string{"k", settingsModDefKindBan, "v", "3600"}},
		{settingsActModDefReason, settingsPageModDefaults, []string{"v", "1"}},
		{settingsActModDefAppealPrompt, settingsPageModDefaults, nil},
		{settingsActModDefAppealCancel, settingsPageModDefaults, []string{"t", settingsModDefAppealPart}},
		{settingsActModDefAppealRemove, settingsPageModDefaults, nil},
		{"sq", settingsPageReasons, []string{"v", "1"}},
		{"sx", settingsPageReasons, []string{"r", "5", "v", "1"}},
		{"sy", settingsPageReasons, []string{"r", "5", "e", minute}},
		{"sm", settingsPageReasons, nil},
		{"sc", settingsPageReasons, []string{"t", "w"}},
	}
}

// TestPhase27Codes verifies every Phase 27 action code (md, mt, mr, ma, mk, mx on page q; sq, sx, sy, sm, sc
// on page rs) maps to its page, is no core code, leaves every prior page registered, and refuses both an
// extra key and a group-menu press (u instead of c) in step 1 with 0 getChatMember calls.
func TestPhase27Codes(t *testing.T) {
	codes := settingsPhase27Codes(settingsCardExpiry(time.Now()))

	registered := map[string]string{}
	for _, pageKey := range []string{settingsPageModDefaults, settingsPageReasons} {
		pg, ok := settingsPageByKey(pageKey)
		if !ok {
			t.Fatalf("page %q is not registered", pageKey)
		}
		if !pg.PrivateOnly {
			t.Errorf("page %q PrivateOnly = false, want true", pageKey)
		}
		for code := range pg.Actions {
			registered[code] = pageKey
		}
	}
	if len(registered) != len(codes) {
		t.Errorf("Phase 27 pages register %d action codes, test lists %d", len(registered), len(codes))
	}

	for _, row := range codes {
		entry, ok := settingsActions[row.code]
		if !ok || entry.pageKey != row.page {
			t.Errorf("code %q maps to %+v (registered %v), want page %q", row.code, entry, ok, row.page)
		}
		if registered[row.code] != row.page {
			t.Errorf("code %q registered under %q, want %q", row.code, registered[row.code], row.page)
		}
		if slices.Contains(settingsCoreActions, row.code) {
			t.Errorf("code %q is a core code", row.code)
		}
	}

	for _, key := range []string{
		"s", "v", "l", "c", "f", "q", "n", "j", "k", "rs",
		"b", "i", "w", "g", "r", "e", "x", "h", "m", "d",
		"t", "u", "z", "o", "y", "p", "a",
	} {
		if _, ok := settingsPageByKey(key); !ok {
			t.Errorf("page %q is not registered", key)
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

// TestPhase27ReadsCreateNothing verifies opening Main, page q, its ban and mute duration screens, and page
// rs on an unconfigured chat (in both private and group menus) creates 0 rows in chat_manual_defaults,
// chat_reason_shortcuts, and chats.
func TestPhase27ReadsCreateNothing(t *testing.T) {
	tables := []struct {
		name  string
		model any
	}{
		{"chat_manual_defaults", &models.ChatManualDefaults{}},
		{"chat_reason_shortcuts", &models.ChatReasonShortcut{}},
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

	env := newSettingsEnv(t)
	cleanupModTuningChat(t, env.chat.Id)
	t.Cleanup(func() {
		db.DB.Where("chat_id = ?", env.chat.Id).Delete(&models.Chat{})
	})

	owner := env.newOwner(env.chat)
	privateMsg := settingsPrivateOnlyMessage(t, env, owner)
	groupMsg := env.openMenu(owner)

	private := func(kv ...string) { env.pressPrivate(owner, privateMsg, env.privateFields(env.chat.Id, kv...)) }
	group := func(kv ...string) { env.press(owner, groupMsg, env.groupFields(owner, kv...)) }

	group("a", settingsActOpen, "p", settingsPageModDefaults) // private-only group note
	group("a", settingsActBack)
	group("a", settingsActOpen, "p", settingsPageReasons) // private-only group note
	group("a", settingsActBack)

	private("a", settingsActOpen, "p", settingsPageModDefaults)
	private("a", settingsActModDefDur, "k", settingsModDefKindBan)
	private("a", settingsActOpen, "p", settingsPageModDefaults)
	private("a", settingsActModDefDur, "k", settingsModDefKindMute)
	private("a", settingsActOpen, "p", settingsPageModDefaults)
	private("a", settingsActBack)
	private("a", settingsActOpen, "p", settingsPageReasons)
	private("a", settingsActBack)

	if got := rows(env.chat.Id); len(got) != 0 {
		t.Errorf("opening Phase 27 screens created rows %v, want none", got)
	}
}

type phase27LogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *phase27LogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *phase27LogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// TestPhase27StaffPrivacyAndLogs links a group to a Staff Group with sentinel title "SecretStaffSentinel",
// exercises pages q and rs with sentinel shortcut and appeal URL values, executes /ban secretword, and
// verifies (a) no screen/reply/button/answer leaks the Staff Group, (b) no logrus line leaks the shortcut
// word, full reason, appeal URL, or Staff Group title, and (c) pages q and rs never appear in a Staff
// Group's Main menu.
func TestPhase27StaffPrivacyAndLogs(t *testing.T) {
	const (
		sentinelStaffTitle = "SecretStaffSentinel"
		sentinelWord       = "secretword"
		sentinelReason     = "Secret full <reason>"
		sentinelAppealHost = "secret-appeal.example.test"
		sentinelAppealURL  = "https://secret-appeal.example.test/form"
	)

	setCachedBotUsername(t, "FukuTestBot")
	env := newSettingsListEnv(t)
	LoadBans(env.dispatcher)
	cleanupModTuningChat(t, env.chat.Id)
	eventMsgCleanup(t, env.chat.Id)

	staffChat := settingsStaffTestOwnedStaffGroup(env.settingsEnv, sentinelStaffTitle)
	linkLockdownGroupTo(env.lockdownEnv, staffChat)

	moderator := gotgbot.User{Id: uniqueLinkOwnerID(), FirstName: "Moderator"}
	target := gotgbot.User{Id: uniqueLinkOwnerID(), FirstName: "Target", LastName: "Member"}
	t.Cleanup(func() {
		db.DB.Where("user_id IN ?", []int64{moderator.Id, target.Id}).Delete(&models.User{})
		dbcache.DeleteCache(dbcache.CacheKey("user", moderator.Id))
		dbcache.DeleteCache(dbcache.CacheKey("user", target.Id))
		dbcache.ResetLocalForTest()
	})
	env.fake.setMemberRaw(env.chat.Id, moderator.Id, settingsPrivacyPhase19AdminJSON(moderator.Id))
	env.fake.listAdministrator(env.chat.Id, moderator.Id)
	env.fake.setMember(env.chat.Id, target.Id, staffFakeMember{Status: gotgbot.ChatMemberStatusMember})
	env.fake.setProfile(target.Id, target.FirstName, target.LastName, "")

	// Capture every logrus line across the entire run.
	logger := log.StandardLogger()
	prevOut := logger.Out
	prevLevel := logger.GetLevel()
	logBuf := &phase27LogBuffer{}
	logger.SetOutput(logBuf)
	logger.SetLevel(log.DebugLevel)
	t.Cleanup(func() {
		logger.SetOutput(prevOut)
		logger.SetLevel(prevLevel)
	})

	msgID := settingsPrivateOnlyMessage(t, env.settingsEnv, moderator)
	press := func(kv ...string) { env.pressPrivate(moderator, msgID, env.privateFields(env.chat.Id, kv...)) }

	// Exercise page q (Manual defaults): open, ban/mute duration pickers, reason toggle, appeal URL set & remove & set.
	press("a", settingsActOpen, "p", settingsPageModDefaults)
	press("a", settingsActModDefDur, "k", settingsModDefKindBan)
	press("a", settingsActModDefSetDur, "k", settingsModDefKindBan, "v", "3600")
	press("a", settingsActModDefDur, "k", settingsModDefKindMute)
	press("a", settingsActModDefSetDur, "k", settingsModDefKindMute, "v", "600")
	press("a", settingsActModDefReason, "v", "1")
	press("a", settingsActModDefAppealPrompt)
	env.sendPrivate(moderator, sentinelAppealURL)
	press("a", settingsActModDefAppealRemove)
	press("a", settingsActModDefAppealPrompt)
	env.sendPrivate(moderator, sentinelAppealURL)

	// Exercise page rs (Reason shortcuts): open, two-step add, two-tap delete, re-add for /ban.
	press("a", settingsActOpen, "p", settingsPageReasons)
	press("a", "sm")
	env.sendPrivate(moderator, sentinelWord)
	env.sendPrivate(moderator, sentinelReason)

	entries, err := modtuning.ListShortcutsFresh(context.Background(), env.chat.Id)
	if err != nil || len(entries) != 1 {
		t.Fatalf("ListShortcutsFresh after add = %+v (%v), want 1 shortcut", entries, err)
	}
	press("a", "sx", "r", strconv.FormatInt(entries[0].ID, 10), "v", "1")
	press("a", "sy", "r", strconv.FormatInt(entries[0].ID, 10), "e", settingsCardExpiry(time.Now()))
	press("a", "sm")
	env.sendPrivate(moderator, sentinelWord)
	env.sendPrivate(moderator, sentinelReason)
	press("a", settingsActBack)

	// Execute /ban secretword in the linked group as moderator replying to target.
	replied := &gotgbot.Message{
		MessageId: env.updateID(),
		Chat:      env.chat,
		From:      &target,
		Text:      "hello",
	}
	banMsg := gotgbot.Message{
		MessageId:      env.updateID(),
		Chat:           env.chat,
		From:           &moderator,
		Text:           "/ban " + sentinelWord,
		ReplyToMessage: replied,
	}
	if err := env.dispatcher.ProcessUpdate(env.bot, &gotgbot.Update{
		UpdateId: env.updateID(),
		Message:  &banMsg,
	}, nil); err != nil {
		t.Fatalf("/ban dispatch error: %v", err)
	}

	if bans := callsToChat(env.fake.staffBotClient, "banChatMember", env.chat.Id); len(bans) != 1 {
		t.Fatalf("banChatMember calls = %d, want 1", len(bans))
	}

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

	for _, want := range []string{"settings_moddef_intro", "settings_rsn_intro"} {
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
			t.Errorf("Phase 27 run leaked Staff Group %q into UI output", leak)
		}
	}

	// (b) Captured logrus output must never contain shortcut word, full reason, appeal URL, or Staff Group title.
	logs := logBuf.String()
	for _, secret := range []string{
		sentinelWord,
		"Secret full",
		sentinelReason,
		sentinelAppealHost,
		sentinelAppealURL,
		sentinelStaffTitle,
	} {
		if strings.Contains(logs, secret) {
			t.Errorf("captured logrus output leaked %q:\n%s", secret, logs)
		}
	}

	// (c) Pages q and rs must never appear in a Staff Group's Main menu (private or group).
	staffOwner := env.newOwner(staffChat)
	env.sendPrivate(staffOwner, fmt.Sprintf("/start settings_%d", staffChat.Id))
	staffPrivateMain := settingsPrivateOnlySent(t, env.settingsEnv, staffOwner)
	checkStaffMain := func(where, text string, kb [][]gotgbot.InlineKeyboardButton) {
		t.Helper()
		for _, forbidden := range []string{"settings_page_moddefaults", "settings_page_reasons"} {
			if containsMarker(text, forbidden) {
				t.Errorf("%s Staff Group Main text unexpectedly contains %s:\n%s", where, forbidden, text)
			}
		}
		for _, btn := range settingsAllButtons(kb) {
			if !strings.HasPrefix(btn.CallbackData, settingsCallbackNamespace+"|") {
				continue
			}
			fields := settingsButtonFields(t, btn)
			if fields["a"] == settingsActOpen && (fields["p"] == settingsPageModDefaults || fields["p"] == settingsPageReasons) {
				t.Errorf("%s Staff Group Main unexpectedly has button for page %q: %+v", where, fields["p"], btn)
			}
		}
	}
	checkStaffMain("private", fmt.Sprint(staffPrivateMain.Params["text"]), staffKeyboardOf(staffPrivateMain.Params["reply_markup"]))

	// Group /settings inside the Staff Group itself.
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
