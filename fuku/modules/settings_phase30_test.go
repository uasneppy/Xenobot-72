//go:build testtools

package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/backup"
	dbcache "github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/chats"
	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/db/rules"
)

// settingsPhase30Code is one action code Phase 30 added on private-only page ba, paired with a
// valid set of extra fields for private-menu callbacks. Every Phase 30 action requires the
// private owner field c.
type settingsPhase30Code struct {
	code, page string
	valid      []string
}

func settingsPhase30Codes() []settingsPhase30Code {
	return []settingsPhase30Code{
		{settingsActBackupToggle, settingsPageBackup, []string{"m", "fffff"}},
		{settingsActBackupExport, settingsPageBackup, []string{"m", "fffff"}},
		{settingsActBackupImport, settingsPageBackup, nil},
		{settingsActBackupImportCancel, settingsPageBackup, []string{"t", settingsBackupImportPart}},
		{settingsActBackupImportYes, settingsPageBackup, []string{"k", "0123456789abcdef"}},
		{settingsActBackupResetAsk, settingsPageBackup, []string{"m", "fffff"}},
		{settingsActBackupResetYes, settingsPageBackup, []string{"k", "0123456789abcdef"}},
	}
}

// TestPhase30Codes verifies all 30 registered settings pages, action codes bv/be/bi/bl/bj/br/bu
// on private-only page ba, zero collisions with core actions, lock rows, or other pages, and
// step-1 refusal of extra unknown fields or group-menu presses (u instead of c) with 0
// getChatMember calls.
func TestPhase30Codes(t *testing.T) {
	allPages := []string{
		"s", "v", "l", "c", "f", "q", "n", "j", "k", "rs",
		"b", "i", "w", "g", "r", "e", "x", "h", "m", "av",
		"cp", "ba", "d", "t", "u", "z", "o", "y", "p", "a",
	}
	if len(allPages) != 30 {
		t.Fatalf("expected 30 registered pages in test list, got %d", len(allPages))
	}
	if len(settingsPages) != len(allPages) {
		t.Errorf("registered settingsPages count = %d, want %d", len(settingsPages), len(allPages))
	}
	seenPages := map[string]bool{}
	for _, key := range allPages {
		if seenPages[key] {
			t.Errorf("duplicate page key %q in test list", key)
		}
		seenPages[key] = true
		if _, ok := settingsPageByKey(key); !ok {
			t.Errorf("page %q is not registered", key)
		}
	}

	baPage, ok := settingsPageByKey(settingsPageBackup)
	if !ok {
		t.Fatalf("page %q is not registered", settingsPageBackup)
	}
	if !baPage.PrivateOnly {
		t.Errorf("page %q PrivateOnly = false, want true", settingsPageBackup)
	}
	if baPage.Order != 83 {
		t.Errorf("page %q Order = %d, want 83", settingsPageBackup, baPage.Order)
	}
	if baPage.Icon != "💾" {
		t.Errorf("page %q Icon = %q, want 💾", settingsPageBackup, baPage.Icon)
	}
	codes := settingsPhase30Codes()
	if len(baPage.Actions) != len(codes) {
		t.Errorf("page %q registers %d action codes, want %d", settingsPageBackup, len(baPage.Actions), len(codes))
	}

	typedCfg, ok := settingsTypedPages[settingsPageBackup]
	if !ok {
		t.Fatalf("settingsTypedPages[%q] is not registered", settingsPageBackup)
	}
	if typedCfg.Part != settingsBackupImportPart || typedCfg.CancelCode != settingsActBackupImportCancel || typedCfg.Right != settingsRightOwnerBotAdmin {
		t.Errorf("settingsTypedPages[%q] = %+v, want Part=%q CancelCode=%q Right=%v",
			settingsPageBackup, typedCfg, settingsBackupImportPart, settingsActBackupImportCancel, settingsRightOwnerBotAdmin)
	}

	exportableCount := len(backup.AllExportableModules())
	wantAllMask := computeSettingsBackupAllMask(exportableCount)
	if settingsBackupAllMask != wantAllMask {
		t.Errorf("settingsBackupAllMask = %#x, want %#x for %d exportable modules", settingsBackupAllMask, wantAllMask, exportableCount)
	}
	if wantHex := len(strconv.FormatUint(uint64(wantAllMask), 16)); settingsBackupMaskMaxHex != wantHex {
		t.Errorf("settingsBackupMaskMaxHex = %d, want %d", settingsBackupMaskMaxHex, wantHex)
	}

	lockCodes := make(map[string]bool, len(settingsLockRows))
	for _, row := range settingsLockRows {
		lockCodes[row.Code] = true
	}

	for _, row := range codes {
		entry, ok := settingsActions[row.code]
		if !ok || entry.pageKey != row.page {
			t.Errorf("code %q maps to %+v (registered %v), want page %q", row.code, entry, ok, row.page)
		}
		if _, inPage := baPage.Actions[row.code]; !inPage {
			t.Errorf("code %q missing from page %q Actions", row.code, row.page)
		}
		if slices.Contains(settingsCoreActions, row.code) {
			t.Errorf("code %q collides with settingsCoreActions", row.code)
		}
		if lockCodes[row.code] {
			t.Errorf("code %q collides with settingsLockRows", row.code)
		}
		for otherKey, otherPage := range settingsPages {
			if otherKey == row.page {
				continue
			}
			if _, dup := otherPage.Actions[row.code]; dup {
				t.Errorf("code %q collides with page %q", row.code, otherKey)
			}
		}
	}

	env := newPhase28SettingsEnv(t)
	owner := env.newOwner(env.chat)
	privateMsg := settingsPrivateOnlyMessage(t, env, owner)
	groupMsg := env.openMenu(owner)
	lookups := func() int { return len(env.fake.callsFor("getChatMember")) }
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

// TestPhase30ReadsCreateNothing proves opening Main (group and private), opening Backup (ba) in
// group and private menus, and toggling module selection via bv (m=1, m=0, m=fffff) on an
// unconfigured chat creates 0 rows across all 10 tables in scope.
func TestPhase30ReadsCreateNothing(t *testing.T) {
	tables := []struct {
		name  string
		model any
	}{
		{"chat_event_messages", &models.ChatEventMessage{}},
		{"chat_event_message_options", &models.ChatEventMessageOptions{}},
		{"warns_settings", &models.WarnSettings{}},
		{"antiflood_settings", &models.AntifloodSettings{}},
		{"blacklists", &models.BlacklistSettings{}},
		{"chat_escalation", &models.ChatEscalation{}},
		{"chat_manual_defaults", &models.ChatManualDefaults{}},
		{"chat_reason_shortcuts", &models.ChatReasonShortcut{}},
		{"chat_advanced_thresholds", &models.ChatAdvancedThresholds{}},
		{"chat_settings_access", &models.ChatSettingsAccess{}},
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
	cleanupCopyPageChat(t, env.chat.Id)
	cleanupBackupPageChat(t, env.chat.Id)
	db.DB.Where("chat_id = ?", env.chat.Id).Delete(&models.ChatSettingsAccess{})
	dbcache.DeleteCache(dbcache.CacheKey("chat_settings_access", env.chat.Id))
	t.Cleanup(func() {
		db.DB.Where("chat_id = ?", env.chat.Id).Delete(&models.ChatSettingsAccess{})
		db.DB.Where("chat_id = ?", env.chat.Id).Delete(&models.Chat{})
		dbcache.DeleteCache(dbcache.CacheKey("chat_settings_access", env.chat.Id))
		dbcache.ResetLocalForTest()
	})

	owner := env.newOwner(env.chat)
	privateMsg := settingsPrivateOnlyMessage(t, env, owner)
	groupMsg := env.openMenu(owner)

	private := func(kv ...string) { env.pressPrivate(owner, privateMsg, env.privateFields(env.chat.Id, kv...)) }
	group := func(kv ...string) { env.press(owner, groupMsg, env.groupFields(owner, kv...)) }

	// Group menu: Main -> open ba (private-only note) -> Back.
	group("a", settingsActOpen, "p", settingsPageBackup)
	group("a", settingsActBack)

	// Private menu: Main -> open ba -> toggle bv (m=1, m=0, m=fffff) -> Back.
	private("a", settingsActBack)
	private("a", settingsActOpen, "p", settingsPageBackup)
	private("a", settingsActBackupToggle, "m", "1")
	private("a", settingsActBackupToggle, "m", "0")
	private("a", settingsActBackupToggle, "m", "fffff")
	private("a", settingsActBack)

	if got := rows(env.chat.Id); len(got) != 0 {
		t.Errorf("opening Backup screens and toggling bv on unconfigured chat created rows %v, want none", got)
	}
}

// TestPhase30StaffPrivacyAndLogs proves the Backup page never appears in a Staff Group's Main
// menu, never leaks a Staff Group title or ID in any screen, document caption, confirm card, or
// notice, and that a full log capture across Export (be), Reset (br -> bu), Import (bi ->
// document -> bj), and Staff Group refusals contains ZERO group titles, Staff Group titles,
// user names, custom message texts, file IDs, button URLs, rules text, or Redis confirmation
// tokens (k).
func TestPhase30StaffPrivacyAndLogs(t *testing.T) {
	const (
		sentinelGroupTitle = "SECRET-GROUP-TITLE-P30"
		sentinelStaffTitle = "SECRET-STAFF-HQ-P30"
		sentinelOwnerName  = "SECRET-OWNER-P30"
		sentinelMsgText    = "SECRET-MSG-TEXT-P30"
		sentinelFileID     = "SECRET-FILE-ID-P30"
		sentinelBtnURL     = "https://secret-p30.example.org/btn"
		sentinelRules      = "SECRET-RULES-P30"
		sentinelDocFileID  = "SECRET-BACKUP-DOC-ID-P30"
	)

	ctx := context.Background()
	env := newPhase28SettingsEnv(t)
	titleClient := &copyTitleBotClient{lockdownFake: env.fake, titles: map[int64]string{}}
	env.bot.BotClient = titleClient

	group := env.chat
	group.Title = sentinelGroupTitle
	env.chat.Title = sentinelGroupTitle
	cleanupCopyPageChat(t, group.Id)
	cleanupBackupPageChat(t, group.Id)

	staffChat := settingsStaffTestOwnedStaffGroup(env, sentinelStaffTitle)
	t.Cleanup(func() {
		cleanupCopyPageChat(t, staffChat.Id)
		cleanupBackupPageChat(t, staffChat.Id)
	})
	linkLockdownGroupTo(env.lockdownEnv, staffChat)

	owner := gotgbot.User{Id: uniqueLinkOwnerID(), FirstName: sentinelOwnerName}
	t.Cleanup(func() {
		db.DB.Where("user_id = ?", owner.Id).Delete(&models.User{})
		dbcache.DeleteCache(dbcache.CacheKey("user", owner.Id))
		dbcache.ResetLocalForTest()
	})
	for _, cid := range []int64{group.Id, staffChat.Id} {
		env.fake.setMemberRaw(cid, owner.Id, copyCreatorJSON(owner.Id))
		env.fake.setCreator(cid, owner.Id)
	}
	titleClient.setTitle(group.Id, sentinelGroupTitle)
	titleClient.setTitle(staffChat.Id, sentinelStaffTitle)
	settingsHubSeedGroup(t, group, sentinelGroupTitle, settingsHubCreator(owner))
	settingsHubSeedGroup(t, staffChat, sentinelStaffTitle, settingsHubCreator(owner))

	if err := chats.EnsureChatInDb(group.Id, sentinelGroupTitle); err != nil {
		t.Fatalf("EnsureChatInDb: %v", err)
	}
	if _, err := eventmsg.ReplaceAllValidated(ctx, group.Id, []eventmsg.Message{
		{
			Event:     eventmsg.EventBan,
			Dest:      eventmsg.DestGroup,
			Text:      sentinelMsgText,
			FileID:    sentinelFileID,
			MediaType: db.PHOTO,
			Buttons:   []models.Button{{Name: "Btn", Url: sentinelBtnURL}},
		},
	}, eventmsg.Options{PMDetails: true, PlainTone: true}, owner.Id); err != nil {
		t.Fatalf("seed eventmsg: %v", err)
	}
	require.NoError(t, rules.SetChatRules(group.Id, sentinelRules))

	// Capture every logrus line at DebugLevel across the entire Phase 30 flow.
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

	msgID := settingsPrivateOnlyMessage(t, env, owner)
	press := func(kv ...string) { env.pressPrivate(owner, msgID, env.privateFields(group.Id, kv...)) }

	// 1. Open ba and toggle modules (bv).
	press("a", settingsActOpen, "p", settingsPageBackup)
	press("a", settingsActBackupToggle, "m", "7ffff")
	press("a", settingsActBackupToggle, "m", "fffff")

	// 2. Export (be) all 20 modules containing sentinels -> capture exported backup JSON.
	press("a", settingsActBackupExport, "m", "fffff")
	docs := env.fake.callsFor("sendDocument")
	if len(docs) != 1 {
		t.Fatalf("sendDocument calls = %d, want 1", len(docs))
	}
	exportedBkp, _ := backupDecodeSentDocument(t, docs[0])
	exportedBytes, err := exportedBkp.ToJSON()
	if err != nil {
		t.Fatalf("marshal exported backup: %v", err)
	}
	for _, mustPresent := range []string{sentinelMsgText, sentinelFileID, sentinelBtnURL, sentinelRules} {
		if !strings.Contains(string(exportedBytes), mustPresent) {
			t.Fatalf("exported JSON missing sentinel %q", mustPresent)
		}
	}

	// 3. Reset (br -> capture 16-char reset token -> bu).
	press("a", settingsActBackupResetAsk, "m", "fffff")
	_, resetCardKB := env.lastEdit(owner.Id, msgID)
	resetYesBtn, ok := settingsAdvFindButton(t, resetCardKB, map[string]string{"a": settingsActBackupResetYes})
	if !ok {
		t.Fatal("reset confirm card missing bu Yes button")
	}
	resetToken := settingsButtonFields(t, resetYesBtn)["k"]
	if !settingsCopyTokenValid(resetToken) {
		t.Fatalf("unexpected reset token %q", resetToken)
	}
	env.pressRaw(settingsPrivateChat(owner), owner, msgID, resetYesBtn.CallbackData)

	afterResetMsgs, err := eventmsg.GetFresh(ctx, group.Id)
	if err != nil {
		t.Fatalf("eventmsg.GetFresh after reset: %v", err)
	}
	if _, hasBan := afterResetMsgs.Lookup(eventmsg.EventBan, eventmsg.DestGroup); hasBan || afterResetMsgs.Options.PlainTone {
		t.Fatalf("reset did not clear custom messages/options: %+v", afterResetMsgs)
	}
	if gotRules := rules.GetChatRulesInfo(group.Id).Rules; gotRules != "" {
		t.Fatalf("reset did not clear rules: %q", gotRules)
	}

	// 4. Import (bi -> send .json document with sentinel file_id -> capture 16-char import token -> bj).
	backupSetDownloadPayload(t, map[string][]byte{
		"backups/p30_privacy.json": exportedBytes,
	})
	env.fake.responses["getFile"] = json.RawMessage(fmt.Sprintf(`{"file_id":%q,"file_path":"backups/p30_privacy.json"}`, sentinelDocFileID))

	press("a", settingsActBackupImport)
	sentBefore := len(env.fake.sentTo(owner.Id))
	sendPrivateMessage(env, owner, gotgbot.Message{
		Document: &gotgbot.Document{FileId: sentinelDocFileID, FileName: "p30_privacy.json"},
	})
	replies := env.fake.sentTo(owner.Id)[sentBefore:]
	if len(replies) != 1 {
		t.Fatalf("expected 1 import confirm card after upload, got %d", len(replies))
	}
	importCardKB, ok := replies[0].Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
	if !ok {
		t.Fatal("import confirm card missing InlineKeyboardMarkup")
	}
	importYesBtn, ok := settingsAdvFindButton(t, importCardKB.InlineKeyboard, map[string]string{"a": settingsActBackupImportYes})
	if !ok {
		t.Fatal("import confirm card missing bj Yes button")
	}
	importToken := settingsButtonFields(t, importYesBtn)["k"]
	if !settingsCopyTokenValid(importToken) {
		t.Fatalf("unexpected import token %q", importToken)
	}
	importCardMsgID := env.updateID()
	env.pressRaw(settingsPrivateChat(owner), owner, importCardMsgID, importYesBtn.CallbackData)

	restoredMsgs, err := eventmsg.GetFresh(ctx, group.Id)
	if err != nil {
		t.Fatalf("eventmsg.GetFresh after import: %v", err)
	}
	if row := restoredMsgs.Resolve(eventmsg.EventBan, eventmsg.DestGroup); row.Text != sentinelMsgText || row.FileID != sentinelFileID || !restoredMsgs.Options.PlainTone {
		t.Fatalf("import did not restore sentinel custom message: row=%+v opts=%+v", row, restoredMsgs.Options)
	}
	if gotRules := rules.GetChatRulesInfo(group.Id).Rules; gotRules != sentinelRules {
		t.Fatalf("import did not restore sentinel rules: %q", gotRules)
	}

	// Verify UI output (screens, exported document caption, confirm cards, notices, answers)
	// across steps 1-4 NEVER leaks the linked Staff Group's title or numeric ID.
	var seen []string
	for _, edit := range env.edits(owner.Id, msgID) {
		seen = append(seen, fmt.Sprint(edit.Params["text"]))
		for _, button := range settingsAllButtons(staffKeyboardOf(edit.Params["reply_markup"])) {
			seen = append(seen, button.Text, button.CallbackData, button.Url)
		}
	}
	for _, edit := range env.edits(owner.Id, importCardMsgID) {
		seen = append(seen, fmt.Sprint(edit.Params["text"]))
		for _, button := range settingsAllButtons(staffKeyboardOf(edit.Params["reply_markup"])) {
			seen = append(seen, button.Text, button.CallbackData, button.Url)
		}
	}
	for _, sent := range env.fake.sentTo(owner.Id) {
		seen = append(seen, fmt.Sprint(sent.Params["text"]))
		for _, button := range settingsAllButtons(staffKeyboardOf(sent.Params["reply_markup"])) {
			seen = append(seen, button.Text, button.CallbackData, button.Url)
		}
	}
	for _, sent := range env.fake.sentTo(group.Id) {
		seen = append(seen, fmt.Sprint(sent.Params["text"]))
		for _, button := range settingsAllButtons(staffKeyboardOf(sent.Params["reply_markup"])) {
			seen = append(seen, button.Text, button.CallbackData, button.Url)
		}
	}
	for _, doc := range env.fake.callsFor("sendDocument") {
		seen = append(seen, fmt.Sprint(doc.Params["caption"]))
	}
	for _, answer := range env.fake.callsFor("answerCallbackQuery") {
		seen = append(seen, fmt.Sprint(answer.Params["text"]))
	}
	everything := strings.Join(seen, "\n")

	staffDigits := strings.TrimPrefix(strconv.FormatInt(staffChat.Id, 10), "-")
	for _, leak := range []string{
		sentinelStaffTitle,
		html.EscapeString(sentinelStaffTitle),
		staffDigits,
	} {
		if strings.Contains(everything, leak) {
			t.Errorf("Phase 30 Backup UI output leaked Staff Group %q", leak)
		}
	}

	// 5. Staff Group hides ba from Main in both private and group /settings, and refuses op/be/bi/br.
	beforeStaffPriv := len(env.fake.sentTo(owner.Id))
	env.sendPrivate(owner, fmt.Sprintf("/start settings_%d", staffChat.Id))
	staffPrivSent := env.fake.sentTo(owner.Id)
	if len(staffPrivSent) <= beforeStaffPriv {
		t.Fatal("Staff Group private /start sent no message")
	}
	staffPrivateMain := staffPrivSent[len(staffPrivSent)-1]
	checkStaffMain := func(where, text string, kb [][]gotgbot.InlineKeyboardButton) {
		t.Helper()
		if containsMarker(text, "settings_page_backup") {
			t.Errorf("%s Staff Group Main text unexpectedly contains settings_page_backup:\n%s", where, text)
		}
		for _, btn := range settingsAllButtons(kb) {
			if !strings.HasPrefix(btn.CallbackData, settingsCallbackNamespace+"|") {
				continue
			}
			fields := settingsButtonFields(t, btn)
			if fields["a"] == settingsActOpen && fields["p"] == settingsPageBackup {
				t.Errorf("%s Staff Group Main unexpectedly has button for page ba: %+v", where, btn)
			}
		}
	}
	checkStaffMain("private", fmt.Sprint(staffPrivateMain.Params["text"]), staffKeyboardOf(staffPrivateMain.Params["reply_markup"]))

	env.fake.setBotRole(staffChat.Id, staffRoleAdmin)
	beforeStaffSent := len(env.fake.sentTo(staffChat.Id))
	staffSettingsMsg := gotgbot.Message{
		MessageId: env.updateID(),
		Chat:      staffChat,
		From:      &owner,
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
		t.Fatal("Staff Group /settings sent no message")
	}
	lastStaff := staffSent[len(staffSent)-1]
	checkStaffMain("group", fmt.Sprint(lastStaff.Params["text"]), staffKeyboardOf(lastStaff.Params["reply_markup"]))

	staffPrivMsg := env.updateID()
	for _, fields := range []map[string]string{
		env.privateFields(staffChat.Id, "a", settingsActOpen, "p", settingsPageBackup),
		env.privateFields(staffChat.Id, "a", settingsActBackupExport, "m", "fffff"),
		env.privateFields(staffChat.Id, "a", settingsActBackupImport),
		env.privateFields(staffChat.Id, "a", settingsActBackupResetAsk, "m", "fffff"),
	} {
		env.pressPrivate(owner, staffPrivMsg, fields)
		ans, alert := copyLastAnswer(t, env)
		if !alert || !strings.Contains(ans, staffMarker("common_callback_invalid_request")) {
			t.Errorf("Staff Group %v answer = (%q, alert=%v), want common_callback_invalid_request alert", fields, ans, alert)
		}
	}

	// 6. Verify captured logrus output was active across Export, Reset, and Import and contains
	//    ZERO sentinels or Redis confirmation tokens.
	logs := logBuf.String()
	for _, expectedLog := range []string{
		fmt.Sprintf("[Backup] Chat %d exported ", group.Id),
		fmt.Sprintf("[Backup] Chat %d reset ", group.Id),
		fmt.Sprintf("[Backup] Chat %d imported ", group.Id),
	} {
		if !strings.Contains(logs, expectedLog) {
			t.Errorf("expected log line %q to prove log capture is active; logs:\n%s", expectedLog, logs)
		}
	}
	for _, secret := range []string{
		sentinelStaffTitle,
		sentinelGroupTitle,
		sentinelOwnerName,
		sentinelMsgText,
		sentinelFileID,
		sentinelBtnURL,
		sentinelRules,
		sentinelDocFileID,
		importToken,
		resetToken,
	} {
		if strings.Contains(logs, secret) {
			t.Errorf("captured logrus output leaked %q:\n%s", secret, logs)
		}
	}
}
