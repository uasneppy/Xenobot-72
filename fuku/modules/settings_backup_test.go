//go:build testtools

package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/backup"
	dbcache "github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/chats"
	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/db/notes"
	"github.com/uasneppy/fuku_robot/fuku/db/rules"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
	"github.com/uasneppy/fuku_robot/fuku/utils/cache"
	"github.com/uasneppy/fuku_robot/fuku/utils/ratelimit"
)

func cleanupBackupPageChat(t *testing.T, chatID int64) {
	t.Helper()
	clean := func() {
		dropPendingImport(chatID)
		dropPendingReset(chatID)
		ratelimit.GetBackupRateLimiter().ReleaseExport(chatID)
		if m := cache.GetMarshal(); m != nil {
			_ = m.Delete(context.Background(), "backup:import:"+strconv.FormatInt(chatID, 10))
			_ = m.Delete(context.Background(), "backup:reset:"+strconv.FormatInt(chatID, 10))
		}
		if rc := cache.GetRedisClient(); rc != nil {
			_ = rc.Del(context.Background(),
				"backup:export:"+strconv.FormatInt(chatID, 10),
				"backup:import:"+strconv.FormatInt(chatID, 10),
				"backup:reset:"+strconv.FormatInt(chatID, 10),
			).Err()
		}
		_ = notes.RemoveAllNotes(chatID)
		if db.DB != nil {
			db.DB.Where("chat_id = ?", chatID).Delete(&models.RulesSettings{})
			db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatEventMessage{})
			db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatEventMessageOptions{})
			db.DB.Where("chat_id = ?", chatID).Delete(&models.StaffGroup{})
			db.DB.Where("chat_id = ?", chatID).Delete(&db.Chat{})
		}
		dbcache.DeleteCache(eventmsg.CacheKey(chatID))
		dbcache.DeleteCache(fmt.Sprintf("rules_%d", chatID))
		dbcache.DeleteCache(fmt.Sprintf("notes_list_%d", chatID))
		dbcache.DeleteCache(fmt.Sprintf("group_lang_%d", chatID))
	}
	clean()
	t.Cleanup(clean)
}

func backupModuleBit(t *testing.T, module string) uint32 {
	t.Helper()
	idx := slices.Index(settingsBackupModules(), module)
	if idx < 0 {
		t.Fatalf("unknown backup module %q", module)
	}
	return uint32(1) << uint(idx)
}

func backupSetDownloadPayload(t *testing.T, files map[string][]byte) {
	t.Helper()
	oldBase := backupDownloadBaseURL
	oldClient := backupDownloadHTTPClient
	backupDownloadBaseURL = "https://example.invalid/file/bot"
	backupDownloadHTTPClient = &http.Client{
		Transport: backupRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			for pathSuffix, payload := range files {
				if strings.HasSuffix(req.URL.Path, "/"+pathSuffix) {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewReader(payload)),
						Header:     make(http.Header),
					}, nil
				}
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader("not found")),
				Header:     make(http.Header),
			}, nil
		}),
	}
	t.Cleanup(func() {
		backupDownloadBaseURL = oldBase
		backupDownloadHTTPClient = oldClient
	})
}

func backupDecodeSentDocument(t *testing.T, call moduleBotCall) (*backup.BackupFormat, string) {
	t.Helper()
	reader, ok := call.Params["document"].(*gotgbot.FileReader)
	if !ok {
		t.Fatalf("sendDocument document param = %T, want *gotgbot.FileReader", call.Params["document"])
	}
	raw, err := io.ReadAll(reader.Data)
	if err != nil {
		t.Fatalf("read exported document: %v", err)
	}
	parsed, err := backup.BackupFormatFromJSON(raw)
	if err != nil {
		t.Fatalf("BackupFormatFromJSON: %v", err)
	}
	return parsed, reader.Name
}

func backupSeedSampleData(t *testing.T, chat gotgbot.Chat, actorID int64) {
	t.Helper()
	ctx := context.Background()
	if err := chats.EnsureChatInDb(chat.Id, chat.Title); err != nil {
		t.Fatalf("EnsureChatInDb(%d): %v", chat.Id, err)
	}
	if _, err := eventmsg.ReplaceAllValidated(ctx, chat.Id, []eventmsg.Message{
		{Event: eventmsg.EventBan, Dest: eventmsg.DestGroup, Text: "Custom group ban for {mention}", Silent: true},
		{Event: eventmsg.EventBan, Dest: eventmsg.DestPM, Text: "Custom PM ban from {chatname}"},
	}, eventmsg.Options{PMDetails: true, PlainTone: true}, actorID); err != nil {
		t.Fatalf("seed eventmsg(%d): %v", chat.Id, err)
	}
	require.NoError(t, rules.SetChatRules(chat.Id, "Group rules line"))
	if err := notes.AddNote(chat.Id, "faq", "Read the FAQ", "", nil, db.TEXT, false, false, false, true, false, false); err != nil {
		t.Fatalf("seed note(%d): %v", chat.Id, err)
	}
}

func TestSettingsBackupPage(t *testing.T) {
	t.Run("open_private_and_group_note_and_staff_hidden", func(t *testing.T) {
		env := newPhase28SettingsEnv(t)
		owner := env.newOwner(env.chat)
		cleanupBackupPageChat(t, env.chat.Id)

		var exportCalls atomic.Int32
		origExport := backupExportChatData
		backupExportChatData = func(chatID int64, chatName string, exportedBy int64, modules []string) (*backup.BackupFormat, error) {
			exportCalls.Add(1)
			return origExport(chatID, chatName, exportedBy, modules)
		}
		t.Cleanup(func() { backupExportChatData = origExport })

		// 1. Group menu Main has exact 17-row shape with ba at Order 83 and Private-only state.
		groupMsg := env.openMenu(owner)
		groupMainText, groupMainKB := env.lastEdit(env.chat.Id, groupMsg)
		if got := settingsMainShape(t, groupMainKB); got != settingsMainShapeGroup {
			t.Errorf("group Main shape = %q, want %q", got, settingsMainShapeGroup)
		}
		if len(groupMainKB) != settingsMainOwnerRows {
			t.Errorf("group Main rows = %d, want %d", len(groupMainKB), settingsMainOwnerRows)
		}
		if line, ok := settingsMainStateLine(groupMainText, "settings_page_backup"); !ok || !strings.Contains(line, staffMarker("settings_state_private_only")) {
			t.Errorf("group Main backup line = %q (ok=%v), want settings_state_private_only", line, ok)
		}

		// Opening ba in a group menu shows settings_bkp_group_note and makes 0 DB reads.
		env.press(owner, groupMsg, env.groupFields(owner, "a", settingsActOpen, "p", settingsPageBackup))
		noteText, _ := env.lastEdit(env.chat.Id, groupMsg)
		if !strings.Contains(noteText, staffMarker("settings_bkp_group_note")) {
			t.Errorf("group ba screen = %q, want settings_bkp_group_note", noteText)
		}
		if exportCalls.Load() != 0 {
			t.Errorf("group menu called export %d times, want 0", exportCalls.Load())
		}

		// Step-1 refusals: group-menu actions (u instead of c) and malformed fields -> 0 getChatMember calls.
		beforeMembers := len(env.fake.callsFor("getChatMember"))
		for _, bad := range []struct {
			chat   gotgbot.Chat
			fields map[string]string
		}{
			{env.chat, env.groupFields(owner, "a", settingsActBackupToggle, "m", "fffff")},
			{env.chat, env.groupFields(owner, "a", settingsActBackupExport, "m", "fffff")},
			{env.chat, env.groupFields(owner, "a", settingsActBackupImport)},
			{env.chat, env.groupFields(owner, "a", settingsActBackupImportCancel, "t", settingsBackupImportPart)},
			{env.chat, env.groupFields(owner, "a", settingsActBackupImportYes, "k", "AbCdEfGhIj0123_-")},
			{env.chat, env.groupFields(owner, "a", settingsActBackupResetAsk, "m", "fffff")},
			{env.chat, env.groupFields(owner, "a", settingsActBackupResetYes, "k", "AbCdEfGhIj0123_-")},
			{settingsPrivateChat(owner), env.privateFields(env.chat.Id, "a", settingsActBackupToggle, "m", "FFFFF")},
			{settingsPrivateChat(owner), env.privateFields(env.chat.Id, "a", settingsActBackupToggle, "m", "00")},
			{settingsPrivateChat(owner), env.privateFields(env.chat.Id, "a", settingsActBackupToggle, "m", "100000")},
			{settingsPrivateChat(owner), env.privateFields(env.chat.Id, "a", settingsActBackupExport, "m", "")},
			{settingsPrivateChat(owner), env.privateFields(env.chat.Id, "a", settingsActBackupImportCancel, "t", "x")},
			{settingsPrivateChat(owner), env.privateFields(env.chat.Id, "a", settingsActBackupImportYes, "k", "short")},
			{settingsPrivateChat(owner), env.privateFields(env.chat.Id, "a", settingsActBackupResetYes, "k", "bad!token1234567")},
		} {
			data := encodeCallbackData(settingsCallbackNamespace, bad.fields)
			env.pressRaw(bad.chat, owner, groupMsg, data)
			ans, alert := copyLastAnswer(t, env)
			if !alert || !strings.Contains(ans, staffMarker("common_callback_invalid_request")) {
				t.Errorf("forged press %v answer = (%q, alert=%v), want common_callback_invalid_request alert", bad.fields, ans, alert)
			}
		}
		if got := len(env.fake.callsFor("getChatMember")) - beforeMembers; got != 0 {
			t.Errorf("step-1 forged presses made %d getChatMember calls, want 0", got)
		}

		// 2. Private menu Main has exact 17-row shape, static MainState, and opens ba with all 20 modules checked.
		privMsg := env.updateID()
		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActBack))
		privMainText, privMainKB := env.lastEdit(owner.Id, privMsg)
		if got := settingsMainShape(t, privMainKB); got != settingsMainShapePrivate {
			t.Errorf("private Main shape = %q, want %q", got, settingsMainShapePrivate)
		}
		if len(privMainKB) != settingsMainOwnerRows {
			t.Errorf("private Main rows = %d, want %d", len(privMainKB), settingsMainOwnerRows)
		}
		if line, ok := settingsMainStateLine(privMainText, "settings_page_backup"); !ok || !strings.Contains(line, staffMarker("settings_state_backup")) {
			t.Errorf("private Main backup line = %q (ok=%v), want settings_state_backup", line, ok)
		}

		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActOpen, "p", settingsPageBackup))
		baText, baKB := env.lastEdit(owner.Id, privMsg)
		if !strings.Contains(baText, staffMarker("settings_bkp_intro")) || !strings.Contains(baText, staffMarker("settings_bkp_selected")) {
			t.Errorf("private ba screen missing intro/selected markers:\n%s", baText)
		}
		checkedBtns := 0
		for _, btn := range settingsAllButtons(baKB) {
			f := settingsButtonFields(t, btn)
			if f["a"] == settingsActBackupToggle && strings.HasPrefix(btn.Text, "✅ ") {
				checkedBtns++
			}
		}
		if checkedBtns != len(settingsBackupModules()) {
			t.Errorf("checked module buttons = %d, want %d", checkedBtns, len(settingsBackupModules()))
		}
		if _, ok := settingsAdvFindButton(t, baKB, map[string]string{"a": settingsActBackupExport, "m": "fffff"}); !ok {
			t.Error("Export button missing m=fffff")
		}

		// 3. Staff Group hides ba from Main (staffOwnerRows = settingsMainOwnerRows - 4, state lines = len(settingsMainTitleKeys) - 7)
		//    and refuses op, bv, be, bi, bl, bj, br, bu with common_callback_invalid_request.
		staffChat := env.addChat()
		cleanupBackupPageChat(t, staffChat.Id)
		copySeedStaffGroup(t, staffChat.Id, owner.Id)
		env.fake.setMemberRaw(staffChat.Id, owner.Id, copyCreatorJSON(owner.Id))
		env.fake.setCreator(staffChat.Id, owner.Id)

		staffPrivMsg := env.updateID()
		env.pressPrivate(owner, staffPrivMsg, env.privateFields(staffChat.Id, "a", settingsActBack))
		staffMainText, staffMainKB := env.lastEdit(owner.Id, staffPrivMsg)
		if _, hasLine := settingsMainStateLine(staffMainText, "settings_page_backup"); hasLine {
			t.Error("Staff Group Main still shows settings_page_backup line")
		}
		if _, hasBtn := settingsPageButton(t, staffMainKB, settingsPageBackup); hasBtn {
			t.Error("Staff Group Main still has ba button")
		}
		if got := len(staffMainKB); got != settingsMainOwnerRows-4 {
			t.Errorf("Staff Group Main rows = %d, want %d", got, settingsMainOwnerRows-4)
		}
		if got := len(settingsMainLines(staffMainText)); got != len(settingsMainTitleKeys)-7 {
			t.Errorf("Staff Group Main state lines = %d, want %d", got, len(settingsMainTitleKeys)-7)
		}

		for _, fields := range []map[string]string{
			env.privateFields(staffChat.Id, "a", settingsActOpen, "p", settingsPageBackup),
			env.privateFields(staffChat.Id, "a", settingsActBackupToggle, "m", "fffff"),
			env.privateFields(staffChat.Id, "a", settingsActBackupExport, "m", "fffff"),
			env.privateFields(staffChat.Id, "a", settingsActBackupImport),
			env.privateFields(staffChat.Id, "a", settingsActBackupImportCancel, "t", settingsBackupImportPart),
			env.privateFields(staffChat.Id, "a", settingsActBackupImportYes, "k", "AbCdEfGhIj0123_-"),
			env.privateFields(staffChat.Id, "a", settingsActBackupResetAsk, "m", "fffff"),
			env.privateFields(staffChat.Id, "a", settingsActBackupResetYes, "k", "AbCdEfGhIj0123_-"),
		} {
			env.pressPrivate(owner, staffPrivMsg, fields)
			ans, alert := copyLastAnswer(t, env)
			if !alert || !strings.Contains(ans, staffMarker("common_callback_invalid_request")) {
				t.Errorf("Staff Group %v answer = (%q, alert=%v), want common_callback_invalid_request alert", fields, ans, alert)
			}
		}
	})

	t.Run("module_bitmask_toggling_select_all_and_clear", func(t *testing.T) {
		env := newPhase28SettingsEnv(t)
		owner := env.newOwner(env.chat)
		cleanupBackupPageChat(t, env.chat.Id)

		privMsg := env.updateID()
		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActOpen, "p", settingsPageBackup))
		_, kb := env.lastEdit(owner.Id, privMsg)

		// Toggle messages (bit 19) off -> mask becomes 7ffff.
		msgBit := backupModuleBit(t, backup.BackupModuleMessages)
		wantAfterMsgOff := settingsBackupFormatMask(settingsBackupAllMask ^ msgBit)
		msgToggleBtn, ok := settingsAdvFindButton(t, kb, map[string]string{"a": settingsActBackupToggle, "m": wantAfterMsgOff})
		if !ok {
			t.Fatalf("missing toggle button for messages mask %s", wantAfterMsgOff)
		}
		env.pressRaw(settingsPrivateChat(owner), owner, privMsg, msgToggleBtn.CallbackData)
		_, kb = env.lastEdit(owner.Id, privMsg)
		if _, ok := settingsAdvFindButton(t, kb, map[string]string{"a": settingsActBackupExport, "m": wantAfterMsgOff}); !ok {
			t.Fatalf("Export button did not update to mask %s", wantAfterMsgOff)
		}

		// Toggle rules (bit 15) off too.
		rulesBit := backupModuleBit(t, backup.BackupModuleRules)
		wantAfterBothOff := settingsBackupFormatMask((settingsBackupAllMask ^ msgBit) ^ rulesBit)
		rulesToggleBtn, ok := settingsAdvFindButton(t, kb, map[string]string{"a": settingsActBackupToggle, "m": wantAfterBothOff})
		if !ok {
			t.Fatalf("missing toggle button for rules mask %s", wantAfterBothOff)
		}
		env.pressRaw(settingsPrivateChat(owner), owner, privMsg, rulesToggleBtn.CallbackData)
		_, kb = env.lastEdit(owner.Id, privMsg)
		if _, ok := settingsAdvFindButton(t, kb, map[string]string{"a": settingsActBackupResetAsk, "m": wantAfterBothOff}); !ok {
			t.Fatalf("Reset button did not update to mask %s", wantAfterBothOff)
		}

		// Press Clear (m=0): all 20 buttons unchecked.
		clearBtn, ok := settingsAdvFindButton(t, kb, map[string]string{"a": settingsActBackupToggle, "m": "0"})
		if !ok {
			t.Fatal("missing Clear (m=0) button")
		}
		env.pressRaw(settingsPrivateChat(owner), owner, privMsg, clearBtn.CallbackData)
		_, kb = env.lastEdit(owner.Id, privMsg)
		unchecked := 0
		for _, btn := range settingsAllButtons(kb) {
			f := settingsButtonFields(t, btn)
			if f["a"] == settingsActBackupToggle && strings.HasPrefix(btn.Text, "⬜ ") && f["m"] != "0" {
				unchecked++
			}
		}
		if unchecked != len(settingsBackupModules()) {
			t.Errorf("unchecked module buttons after Clear = %d, want %d", unchecked, len(settingsBackupModules()))
		}

		// Pressing Export (be m=0) or Reset (br m=0) answers settings_bkp_pick_one and edits nothing.
		editsBefore := len(env.edits(owner.Id, privMsg))
		docsBefore := len(env.fake.callsFor("sendDocument"))
		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActBackupExport, "m", "0"))
		ans, alert := copyLastAnswer(t, env)
		if !alert || !strings.Contains(ans, staffMarker("settings_bkp_pick_one")) {
			t.Errorf("be m=0 answer = (%q, alert=%v), want settings_bkp_pick_one", ans, alert)
		}
		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActBackupResetAsk, "m", "0"))
		ans, alert = copyLastAnswer(t, env)
		if !alert || !strings.Contains(ans, staffMarker("settings_bkp_pick_one")) {
			t.Errorf("br m=0 answer = (%q, alert=%v), want settings_bkp_pick_one", ans, alert)
		}
		if len(env.edits(owner.Id, privMsg)) != editsBefore {
			t.Error("be/br with m=0 unexpectedly edited message")
		}
		if len(env.fake.callsFor("sendDocument")) != docsBefore {
			t.Error("be with m=0 unexpectedly sent document")
		}
		for _, key := range env.redis.Keys() {
			if strings.HasPrefix(key, settingsBackupResetCardKeyPrefix) {
				t.Errorf("br with m=0 unexpectedly wrote Redis key %q", key)
			}
		}

		// Press Select all (m=fffff): all 20 modules selected again.
		allBtn, ok := settingsAdvFindButton(t, kb, map[string]string{"a": settingsActBackupToggle, "m": "fffff"})
		if !ok {
			t.Fatal("missing Select all (m=fffff) button")
		}
		env.pressRaw(settingsPrivateChat(owner), owner, privMsg, allBtn.CallbackData)
		_, kb = env.lastEdit(owner.Id, privMsg)
		if _, ok := settingsAdvFindButton(t, kb, map[string]string{"a": settingsActBackupExport, "m": "fffff"}); !ok {
			t.Error("Select all did not restore m=fffff on Export button")
		}
	})

	t.Run("export_rights_rate_limit_private_delivery_and_command_parity", func(t *testing.T) {
		env := newPhase28SettingsEnv(t)
		owner := env.newOwner(env.chat)
		cleanupBackupPageChat(t, env.chat.Id)
		backupSeedSampleData(t, env.chat, owner.Id)

		// Allowed non-creator admin presses be (m=fffff) -> succeeds, private delivery only, includes messages.
		admin := gotgbot.User{Id: owner.Id + 101, FirstName: "AllowedAdmin"}
		env.fake.setMemberRaw(env.chat.Id, admin.Id, copyAdminJSON(admin.Id, true, true))
		env.fake.listAdministrator(env.chat.Id, admin.Id)
		env.setAdminsAllowed(env.chat.Id, true)

		privMsg := env.updateID()
		env.pressPrivate(admin, privMsg, env.privateFields(env.chat.Id, "a", settingsActBackupExport, "m", "fffff"))
		afterText, _ := env.lastEdit(admin.Id, privMsg)
		if !strings.Contains(afterText, settingsNoticeOKIcon) || !strings.Contains(afterText, staffMarker("backup_export_success_text")) {
			t.Errorf("export redraw missing OK success notice:\n%s", afterText)
		}

		docs := env.fake.callsFor("sendDocument")
		if len(docs) != 1 {
			t.Fatalf("sendDocument calls = %d, want 1", len(docs))
		}
		if gotChat := staffParamInt(docs[0].Params, "chat_id"); gotChat != admin.Id {
			t.Errorf("sendDocument chat_id = %d, want presser private chat %d (never group %d)", gotChat, admin.Id, env.chat.Id)
		}
		if fmt.Sprint(docs[0].Params["parse_mode"]) != "HTML" {
			t.Errorf("sendDocument parse_mode = %v, want HTML", docs[0].Params["parse_mode"])
		}
		exportedPage, fileName := backupDecodeSentDocument(t, docs[0])
		if !strings.HasPrefix(fileName, fmt.Sprintf("xenobot72_backup_%d_", env.chat.Id)) || !strings.HasSuffix(fileName, ".json") {
			t.Errorf("unexpected export filename %q", fileName)
		}
		if !slices.Contains(exportedPage.Modules, backup.BackupModuleMessages) {
			t.Errorf("page export missing messages module: %v", exportedPage.Modules)
		}
		if _, ok := exportedPage.Data[backup.BackupModuleMessages]; !ok {
			t.Error("page export data missing messages payload")
		}

		// Immediate second export hits AcquireExport rate limit and sends 0 additional documents.
		env.pressPrivate(admin, privMsg, env.privateFields(env.chat.Id, "a", settingsActBackupExport, "m", "fffff"))
		rateText, _ := env.lastEdit(admin.Id, privMsg)
		if !strings.Contains(rateText, settingsNoticeWarnIcon) || !strings.Contains(rateText, staffMarker("backup_export_rate_limited")) {
			t.Errorf("rate-limited export redraw missing warning notice:\n%s", rateText)
		}
		if len(env.fake.callsFor("sendDocument")) != 1 {
			t.Errorf("rate-limited export sent extra document (total=%d)", len(env.fake.callsFor("sendDocument")))
		}

		// Parity with /export on subset (messages, rules, notes) + owner export check.
		ratelimit.GetBackupRateLimiter().ReleaseExport(env.chat.Id)
		subsetMask := backupModuleBit(t, backup.BackupModuleMessages) | backupModuleBit(t, backup.BackupModuleRules) | backupModuleBit(t, backup.BackupModuleNotes)
		ownerPrivMsg := env.updateID()
		env.pressPrivate(owner, ownerPrivMsg, env.privateFields(env.chat.Id, "a", settingsActBackupExport, "m", settingsBackupFormatMask(subsetMask)))
		docs = env.fake.callsFor("sendDocument")
		if len(docs) != 2 {
			t.Fatalf("owner subset export sendDocument calls = %d, want 2", len(docs))
		}
		if gotChat := staffParamInt(docs[1].Params, "chat_id"); gotChat != owner.Id {
			t.Errorf("owner export went to chat %d, want %d", gotChat, owner.Id)
		}
		pageSubset, _ := backupDecodeSentDocument(t, docs[1])

		ratelimit.GetBackupRateLimiter().ReleaseExport(env.chat.Id)
		cmdClient := phase30NewBackupClient(admin)
		cmdBot := newModuleTestBot(cmdClient)
		if err := backupModule.exportHandler(cmdBot, newModuleMessageContext(cmdBot, env.chat, admin, "/export notes rules messages")); err != ext.EndGroups {
			t.Fatalf("/export returned %v, want ext.EndGroups", err)
		}
		cmdDocs := cmdClient.callsFor("sendDocument")
		if len(cmdDocs) != 1 {
			t.Fatalf("/export sendDocument calls = %d, want 1", len(cmdDocs))
		}
		cmdSubset, _ := backupDecodeSentDocument(t, cmdDocs[0])

		for _, mod := range []string{backup.BackupModuleNotes, backup.BackupModuleRules, backup.BackupModuleMessages} {
			pageJSON, _ := json.Marshal(pageSubset.Data[mod])
			cmdJSON, _ := json.Marshal(cmdSubset.Data[mod])
			if string(pageJSON) != string(cmdJSON) {
				t.Errorf("export parity mismatch for module %s:\npage=%s\ncmd =%s", mod, pageJSON, cmdJSON)
			}
		}

		// When the bot is NOT an admin, be is refused with chat_status_bot_not_admin and sends 0 documents.
		noAdminChat := env.addChat()
		cleanupBackupPageChat(t, noAdminChat.Id)
		env.fake.setMemberRaw(noAdminChat.Id, owner.Id, copyCreatorJSON(owner.Id))
		env.fake.setCreator(noAdminChat.Id, owner.Id)
		env.fake.smu.Lock()
		env.fake.botRole[noAdminChat.Id] = staffRoleMember
		env.fake.smu.Unlock()
		env.fake.setBotMember(noAdminChat.Id, fmt.Sprintf(`{"status":"member","user":{"id":%d,"is_bot":true,"first_name":"Fuku"}}`, staffTestBotID))

		docsBefore := len(env.fake.callsFor("sendDocument"))
		noAdminMsg := env.updateID()
		env.pressPrivate(owner, noAdminMsg, env.privateFields(noAdminChat.Id, "a", settingsActBackupExport, "m", "fffff"))
		refusedText, _ := env.lastEdit(owner.Id, noAdminMsg)
		if !strings.Contains(refusedText, staffMarker("chat_status_bot_not_admin")) {
			t.Errorf("bot-not-admin export redraw = %q, want chat_status_bot_not_admin", refusedText)
		}
		if len(env.fake.callsFor("sendDocument")) != docsBefore {
			t.Error("bot-not-admin export sent document, want 0")
		}
	})

	t.Run("import_owner_right_private_document_wait_two_tap_card_and_command_parity", func(t *testing.T) {
		env := newPhase28SettingsEnv(t)
		owner := env.newOwner(env.chat)
		cleanupBackupPageChat(t, env.chat.Id)

		// 1. Non-owner admin pressing bi is refused with chat_status_owner_button_error and creates no wait.
		admin := gotgbot.User{Id: owner.Id + 201, FirstName: "ModAdmin"}
		env.fake.setMemberRaw(env.chat.Id, admin.Id, copyAdminJSON(admin.Id, true, true))
		env.fake.listAdministrator(env.chat.Id, admin.Id)
		env.setAdminsAllowed(env.chat.Id, true)

		adminPrivMsg := env.updateID()
		env.pressPrivate(admin, adminPrivMsg, env.privateFields(env.chat.Id, "a", settingsActBackupImport))
		ans, alert := copyLastAnswer(t, env)
		if !alert || !strings.Contains(ans, staffMarker("chat_status_owner_button_error")) {
			t.Errorf("non-owner bi answer = (%q, alert=%v), want chat_status_owner_button_error alert", ans, alert)
		}
		if env.redis.Exists(settingsInputKey(admin.Id)) {
			t.Error("non-owner bi unexpectedly wrote Redis wait")
		}

		// 2. Owner presses bi -> prompt appears with Cancel button (bl) and Redis wait exists.
		privMsg := env.updateID()
		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActBackupImport))
		promptText, promptKB := env.lastEdit(owner.Id, privMsg)
		for _, want := range []string{
			staffMarker("settings_bkp_import_wait"),
			staffMarker("settings_bkp_import_wait_rules"),
			staffMarker("settings_edit_wait_footer"),
		} {
			if !strings.Contains(promptText, want) {
				t.Errorf("import prompt missing %q:\n%s", want, promptText)
			}
		}
		cancelBtn, ok := settingsAdvFindButton(t, promptKB, map[string]string{
			"a": settingsActBackupImportCancel,
			"t": settingsBackupImportPart,
		})
		if !ok {
			t.Fatal("import prompt missing Cancel button (bl t=i)")
		}
		if !env.redis.Exists(settingsInputKey(owner.Id)) {
			t.Fatal("bi did not write Redis wait")
		}

		// Verify Cancel (bl) cancels wait cleanly, then re-open bi.
		env.pressRaw(settingsPrivateChat(owner), owner, privMsg, cancelBtn.CallbackData)
		cancelText, _ := env.lastEdit(owner.Id, privMsg)
		if !strings.Contains(cancelText, staffMarker("settings_edit_cancelled")) {
			t.Errorf("bl cancel screen missing settings_edit_cancelled:\n%s", cancelText)
		}
		if env.redis.Exists(settingsInputKey(owner.Id)) {
			t.Error("bl did not clear Redis wait")
		}
		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActBackupImport))

		// Prepare backup payloads for invalid JSON and valid JSON with messages + rules + logchannels.
		validBkp := backup.NewBackupFormat(env.chat.Id, env.chat.Title, owner.Id, []string{
			backup.BackupModuleMessages,
			backup.BackupModuleRules,
			backup.BackupModuleLogChannels,
		})
		validBkp.Data[backup.BackupModuleMessages] = map[string]any{
			"messages": []any{
				map[string]any{
					"event": eventmsg.EventBan,
					"dest":  eventmsg.DestGroup,
					"text":  "Imported custom ban for {mention}",
				},
				map[string]any{
					"event": eventmsg.EventKick,
					"dest":  eventmsg.DestGroup,
					"text":  "Bad button kick",
					"buttons": []any{
						map[string]any{"name": "Bad", "url": "javascript:alert(1)"},
					},
				},
			},
			"options": map[string]any{
				"pm_details": true,
				"plain_tone": true,
			},
		}
		validBkp.Data[backup.BackupModuleRules] = map[string]any{
			"settings": map[string]any{
				"rules": "Imported rules via page",
			},
		}
		validBkp.Data[backup.BackupModuleLogChannels] = map[string]any{
			"settings": map[string]any{
				"cat_admin": false,
			},
		}
		validBytes, err := validBkp.ToJSON()
		if err != nil {
			t.Fatalf("marshal validBkp: %v", err)
		}

		oversizedBkp := backup.NewBackupFormat(env.chat.Id, env.chat.Title, owner.Id, []string{
			backup.BackupModuleRules,
		})
		oversizedBkp.Data[backup.BackupModuleRules] = map[string]any{
			"rules": strings.Repeat("x", maxSettingsBackupRedisBytes+64),
		}
		oversizedBytes, err := oversizedBkp.ToJSON()
		if err != nil {
			t.Fatalf("marshal oversizedBkp: %v", err)
		}

		backupSetDownloadPayload(t, map[string][]byte{
			"backups/corrupt.json":   []byte(`{not valid json`),
			"backups/oversized.json": oversizedBytes,
			"backups/valid.json":     validBytes,
		})

		// 3a. Plain text message during wait -> refused with settings_bkp_import_need_file + Cancel, wait stays alive!
		sentBefore := len(env.fake.sentTo(owner.Id))
		sendPrivateMessage(env, owner, gotgbot.Message{Text: "not a file"})
		replies := env.fake.sentTo(owner.Id)[sentBefore:]
		if len(replies) != 1 || !strings.Contains(sentText(replies[0]), staffMarker("settings_bkp_import_need_file")) {
			t.Errorf("plain text reply = %+v, want settings_bkp_import_need_file", replies)
		}
		if !env.redis.Exists(settingsInputKey(owner.Id)) {
			t.Fatal("plain text refusal unexpectedly cleared wait")
		}

		// 3b. .txt document during wait -> refused with backup_import_invalid_file, wait stays alive!
		sentBefore = len(env.fake.sentTo(owner.Id))
		sendPrivateMessage(env, owner, gotgbot.Message{
			Document: &gotgbot.Document{FileId: "txt-doc", FileName: "backup.txt"},
		})
		replies = env.fake.sentTo(owner.Id)[sentBefore:]
		if len(replies) != 1 || !strings.Contains(sentText(replies[0]), staffMarker("backup_import_invalid_file")) {
			t.Errorf(".txt doc reply = %+v, want backup_import_invalid_file", replies)
		}
		if !env.redis.Exists(settingsInputKey(owner.Id)) {
			t.Fatal(".txt refusal unexpectedly cleared wait")
		}

		// 3c. Corrupted .json document during wait -> refused with backup_import_invalid_file, wait stays alive!
		env.fake.responses["getFile"] = json.RawMessage(`{"file_id":"bad-json","file_path":"backups/corrupt.json"}`)
		sentBefore = len(env.fake.sentTo(owner.Id))
		sendPrivateMessage(env, owner, gotgbot.Message{
			Document: &gotgbot.Document{FileId: "bad-json", FileName: "corrupt.json"},
		})
		replies = env.fake.sentTo(owner.Id)[sentBefore:]
		if len(replies) != 1 || !strings.Contains(sentText(replies[0]), staffMarker("backup_import_invalid_file")) {
			t.Errorf("corrupt json reply = %+v, want backup_import_invalid_file", replies)
		}
		if !env.redis.Exists(settingsInputKey(owner.Id)) {
			t.Fatal("corrupt json refusal unexpectedly cleared wait")
		}

		// 3d. Valid .json document exceeding maxSettingsBackupRedisBytes (2 MiB) -> refused with
		// settings_bkp_import_file_too_large, wait stays alive!
		env.fake.responses["getFile"] = json.RawMessage(`{"file_id":"big-json","file_path":"backups/oversized.json"}`)
		sentBefore = len(env.fake.sentTo(owner.Id))
		sendPrivateMessage(env, owner, gotgbot.Message{
			Document: &gotgbot.Document{FileId: "big-json", FileName: "oversized.json"},
		})
		replies = env.fake.sentTo(owner.Id)[sentBefore:]
		if len(replies) != 1 || !strings.Contains(sentText(replies[0]), staffMarker("settings_bkp_import_file_too_large")) {
			t.Errorf("oversized json reply = %+v, want settings_bkp_import_file_too_large", replies)
		}
		if !env.redis.Exists(settingsInputKey(owner.Id)) {
			t.Fatal("oversized json refusal unexpectedly cleared wait")
		}

		// 4. Valid .json document -> stores fuku:settings:backup:import:<token>, clears wait, closes prompt, sends two-tap confirm card!
		env.fake.responses["getFile"] = json.RawMessage(`{"file_id":"ok-json","file_path":"backups/valid.json"}`)
		sentBefore = len(env.fake.sentTo(owner.Id))
		sendPrivateMessage(env, owner, gotgbot.Message{
			Document: &gotgbot.Document{FileId: "ok-json", FileName: "valid.json"},
		})
		if env.redis.Exists(settingsInputKey(owner.Id)) {
			t.Error("valid upload left wait in Redis")
		}
		closedPromptText, _ := env.lastEdit(owner.Id, privMsg)
		if !strings.Contains(closedPromptText, staffMarker("settings_edit_prompt_closed")) {
			t.Errorf("prompt was not closed after valid upload: %q", closedPromptText)
		}
		replies = env.fake.sentTo(owner.Id)[sentBefore:]
		if len(replies) != 1 {
			t.Fatalf("expected 1 confirm card message after upload, got %d", len(replies))
		}
		cardText := sentText(replies[0])
		for _, want := range []string{
			staffMarker("backup_import_confirm"),
			"• messages",
			"• rules",
			"• logchannels",
		} {
			if !strings.Contains(cardText, want) {
				t.Errorf("import confirm card missing %q:\n%s", want, cardText)
			}
		}
		cardKB, ok := replies[0].Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
		if !ok {
			t.Fatalf("confirm card missing InlineKeyboardMarkup")
		}
		yesBtn, ok := settingsAdvFindButton(t, cardKB.InlineKeyboard, map[string]string{"a": settingsActBackupImportYes})
		if !ok {
			t.Fatal("import confirm card missing Yes (bj) button")
		}
		if _, ok := settingsAdvFindButton(t, cardKB.InlineKeyboard, map[string]string{"a": settingsActCancel, "p": settingsPageBackup}); !ok {
			t.Fatal("import confirm card missing Cancel (no p=ba) button")
		}
		token := settingsButtonFields(t, yesBtn)["k"]
		redisKey := settingsBackupImportTokenKey(token)
		if !env.redis.Exists(redisKey) {
			t.Fatalf("Redis key %q missing after upload", redisKey)
		}
		if ttl := env.redis.TTL(redisKey); ttl <= 0 || ttl > settingsCardLifetime {
			t.Errorf("import Redis key TTL = %v, want (0, %v]", ttl, settingsCardLifetime)
		}

		// 5. Pressing Yes (bj) as live owner consumes token, runs applyBackupImport, reports skipped & log-channel hints.
		cardMsgID := env.updateID()
		env.pressRaw(settingsPrivateChat(owner), owner, cardMsgID, yesBtn.CallbackData)
		if env.redis.Exists(redisKey) {
			t.Errorf("Redis key %q still exists after bj", redisKey)
		}
		doneText, _ := env.lastEdit(owner.Id, cardMsgID)
		for _, want := range []string{
			settingsNoticeOKIcon,
			staffMarker("backup_import_success"),
			staffMarker("backup_import_skipped_entries"),
			staffMarker("backup_import_log_skipped"),
		} {
			if !strings.Contains(doneText, want) {
				t.Errorf("post-import redraw missing %q:\n%s", want, doneText)
			}
		}

		gotMsgs, err := eventmsg.GetFresh(context.Background(), env.chat.Id)
		if err != nil {
			t.Fatalf("eventmsg.GetFresh: %v", err)
		}
		banRow, hasBan := gotMsgs.Lookup(eventmsg.EventBan, eventmsg.DestGroup)
		if !hasBan || banRow.Text != "Imported custom ban for {mention}" {
			t.Errorf("imported ban message = (%+v, ok=%v)", banRow, hasBan)
		}
		if _, hasKick := gotMsgs.Lookup(eventmsg.EventKick, eventmsg.DestGroup); hasKick {
			t.Error("invalid javascript: button kick message should have been skipped")
		}
		if !gotMsgs.Options.PMDetails || !gotMsgs.Options.PlainTone {
			t.Errorf("imported options = %+v, want PMDetails=true PlainTone=true", gotMsgs.Options)
		}
		if gotRules := rules.GetChatRulesInfo(env.chat.Id).Rules; gotRules != "Imported rules via page" {
			t.Errorf("imported rules = %q, want %q", gotRules, "Imported rules via page")
		}

		// 6. Command parity: /import + ci on a second chat leaves identical DB state.
		cmdChat := env.addChat()
		cleanupBackupPageChat(t, cmdChat.Id)
		if err := chats.EnsureChatInDb(cmdChat.Id, cmdChat.Title); err != nil {
			t.Fatalf("EnsureChatInDb(cmdChat): %v", err)
		}
		cmdClient := phase30NewBackupClient(admin)
		cmdClient.responses["getFile"] = json.RawMessage(`{"file_id":"ok-json","file_path":"backups/valid.json"}`)
		cmdBot := newModuleTestBot(cmdClient)
		cmdOwner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
		importCtx := newModuleMessageContext(cmdBot, cmdChat, cmdOwner, "/import")
		importCtx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
			MessageId: 501,
			Date:      1,
			Chat:      cmdChat,
			Document:  &gotgbot.Document{FileId: "ok-json", FileName: "valid.json"},
		}
		if err := backupModule.importHandler(cmdBot, importCtx); err != ext.EndGroups {
			t.Fatalf("/import returned %v", err)
		}
		cmdMsgs := cmdClient.callsFor("sendMessage")
		if len(cmdMsgs) != 1 {
			t.Fatalf("/import confirm messages = %d, want 1", len(cmdMsgs))
		}
		cmdConfirmKB := cmdMsgs[0].Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
		if err := backupModule.backupCallbackHandler(cmdBot, newModuleCallbackContext(cmdBot, cmdChat, cmdOwner, cmdConfirmKB.InlineKeyboard[0][0].CallbackData)); err != ext.EndGroups {
			t.Fatalf("/import confirm callback returned %v", err)
		}
		cmdGotMsgs, err := eventmsg.GetFresh(context.Background(), cmdChat.Id)
		if err != nil {
			t.Fatalf("cmd eventmsg.GetFresh: %v", err)
		}
		cmdBan, hasCmdBan := cmdGotMsgs.Lookup(eventmsg.EventBan, eventmsg.DestGroup)
		if !hasCmdBan || cmdBan.Text != banRow.Text || cmdGotMsgs.Options != gotMsgs.Options {
			t.Errorf("page vs /import eventmsg parity mismatch: page=(%q,%+v) cmd=(%q,%+v)", banRow.Text, gotMsgs.Options, cmdBan.Text, cmdGotMsgs.Options)
		}
		if rules.GetChatRulesInfo(cmdChat.Id).Rules != rules.GetChatRulesInfo(env.chat.Id).Rules {
			t.Error("page vs /import rules parity mismatch")
		}

		// 7. Old export without "messages" via bi + document + bj succeeds and preserves existing custom messages.
		oldChat := env.addChat()
		cleanupBackupPageChat(t, oldChat.Id)
		env.fake.setMemberRaw(oldChat.Id, owner.Id, copyCreatorJSON(owner.Id))
		env.fake.setCreator(oldChat.Id, owner.Id)
		backupSeedSampleData(t, oldChat, owner.Id)

		oldExportJSON := []byte(`{
			"version": "1.0",
			"exported_at": "2026-11-23T12:00:00Z",
			"chat_id": -100300099,
			"chat_name": "Old Chat",
			"exported_by": 1,
			"bot_name": "Xenobot-72",
			"modules": ["rules"],
			"data": {
				"rules": {"settings": {"rules": "Legacy rules only"}}
			}
		}`)
		backupSetDownloadPayload(t, map[string][]byte{
			"backups/old.json": oldExportJSON,
		})
		env.fake.responses["getFile"] = json.RawMessage(`{"file_id":"old-json","file_path":"backups/old.json"}`)

		oldPrivMsg := env.updateID()
		env.pressPrivate(owner, oldPrivMsg, env.privateFields(oldChat.Id, "a", settingsActBackupImport))
		sentBefore = len(env.fake.sentTo(owner.Id))
		sendPrivateMessage(env, owner, gotgbot.Message{
			Document: &gotgbot.Document{FileId: "old-json", FileName: "old.json"},
		})
		oldReplies := env.fake.sentTo(owner.Id)[sentBefore:]
		if len(oldReplies) != 1 {
			t.Fatalf("old export confirm messages = %d, want 1", len(oldReplies))
		}
		oldCardKB := oldReplies[0].Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
		oldYes, ok := settingsAdvFindButton(t, oldCardKB.InlineKeyboard, map[string]string{"a": settingsActBackupImportYes})
		if !ok {
			t.Fatal("old export confirm card missing Yes button")
		}
		oldCardMsg := env.updateID()
		env.pressRaw(settingsPrivateChat(owner), owner, oldCardMsg, oldYes.CallbackData)

		if got := rules.GetChatRulesInfo(oldChat.Id).Rules; got != "Legacy rules only" {
			t.Errorf("old export rules = %q, want Legacy rules only", got)
		}
		oldMsgs, err := eventmsg.GetFresh(context.Background(), oldChat.Id)
		if err != nil {
			t.Fatalf("old export eventmsg.GetFresh: %v", err)
		}
		if preserved, ok := oldMsgs.Lookup(eventmsg.EventBan, eventmsg.DestGroup); !ok || preserved.Text != "Custom group ban for {mention}" {
			t.Errorf("old export clobbered custom ban message: %+v (ok=%v)", preserved, ok)
		}
	})

	t.Run("reset_owner_right_two_tap_card_rate_limit_and_command_parity", func(t *testing.T) {
		env := newPhase28SettingsEnv(t)
		owner := env.newOwner(env.chat)
		cleanupBackupPageChat(t, env.chat.Id)
		backupSeedSampleData(t, env.chat, owner.Id)

		// 1. Non-owner admin pressing br is refused with chat_status_owner_button_error and writes no Redis token.
		admin := gotgbot.User{Id: owner.Id + 301, FirstName: "ResetMod"}
		env.fake.setMemberRaw(env.chat.Id, admin.Id, copyAdminJSON(admin.Id, true, true))
		env.fake.listAdministrator(env.chat.Id, admin.Id)
		env.setAdminsAllowed(env.chat.Id, true)

		mask := backupModuleBit(t, backup.BackupModuleMessages) | backupModuleBit(t, backup.BackupModuleRules)
		maskStr := settingsBackupFormatMask(mask)

		adminMsg := env.updateID()
		env.pressPrivate(admin, adminMsg, env.privateFields(env.chat.Id, "a", settingsActBackupResetAsk, "m", maskStr))
		ans, alert := copyLastAnswer(t, env)
		if !alert || !strings.Contains(ans, staffMarker("chat_status_owner_button_error")) {
			t.Errorf("non-owner br answer = (%q, alert=%v), want chat_status_owner_button_error", ans, alert)
		}

		// 2. Owner presses br with messages+rules mask -> stores Redis reset token & draws two-tap Reset card.
		privMsg := env.updateID()
		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActBackupResetAsk, "m", maskStr))
		cardText, cardKB := env.lastEdit(owner.Id, privMsg)
		for _, want := range []string{
			staffMarker("backup_reset_confirm"),
			"• rules",
			"• messages",
		} {
			if !strings.Contains(cardText, want) {
				t.Errorf("reset confirm card missing %q:\n%s", want, cardText)
			}
		}
		yesBtn, ok := settingsAdvFindButton(t, cardKB, map[string]string{"a": settingsActBackupResetYes})
		if !ok {
			t.Fatal("reset confirm card missing Yes (bu) button")
		}
		if _, ok := settingsAdvFindButton(t, cardKB, map[string]string{"a": settingsActCancel, "p": settingsPageBackup}); !ok {
			t.Fatal("reset confirm card missing Cancel (no p=ba) button")
		}
		token := settingsButtonFields(t, yesBtn)["k"]
		redisKey := settingsBackupResetTokenKey(token)
		if !env.redis.Exists(redisKey) {
			t.Fatalf("Redis reset key %q missing after br", redisKey)
		}

		// 3. Pressing Yes (bu) as live owner clears messages + options + rules and keeps notes intact.
		env.pressRaw(settingsPrivateChat(owner), owner, privMsg, yesBtn.CallbackData)
		if env.redis.Exists(redisKey) {
			t.Errorf("Redis reset key %q still exists after bu", redisKey)
		}
		doneText, _ := env.lastEdit(owner.Id, privMsg)
		if !strings.Contains(doneText, settingsNoticeOKIcon) || !strings.Contains(doneText, staffMarker("backup_reset_success")) {
			t.Errorf("post-reset redraw missing OK success notice:\n%s", doneText)
		}
		gotMsgs, err := eventmsg.GetFresh(context.Background(), env.chat.Id)
		if err != nil {
			t.Fatalf("eventmsg.GetFresh: %v", err)
		}
		if _, hasBan := gotMsgs.Lookup(eventmsg.EventBan, eventmsg.DestGroup); hasBan {
			t.Error("reset did not clear custom ban message")
		}
		if gotMsgs.Options.PlainTone || gotMsgs.Options.PMDetails {
			t.Errorf("reset did not clear event message options: %+v", gotMsgs.Options)
		}
		if gotRules := rules.GetChatRulesInfo(env.chat.Id).Rules; gotRules != "" {
			t.Errorf("reset did not clear rules: %q", gotRules)
		}
		if len(notes.GetNotesList(env.chat.Id, true)) == 0 {
			t.Error("reset cleared unselected notes module")
		}

		// 4. Second reset within cooldown is refused by AcquireReset with backup_reset_rate_limited.
		require.NoError(t, rules.SetChatRules(env.chat.Id, "Should survive rate-limited reset"))
		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActBackupResetAsk, "m", maskStr))
		_, cardKB2 := env.lastEdit(owner.Id, privMsg)
		yesBtn2, ok := settingsAdvFindButton(t, cardKB2, map[string]string{"a": settingsActBackupResetYes})
		if !ok {
			t.Fatal("second reset ask missing Yes button")
		}
		env.pressRaw(settingsPrivateChat(owner), owner, privMsg, yesBtn2.CallbackData)
		rateText, _ := env.lastEdit(owner.Id, privMsg)
		if !strings.Contains(rateText, settingsNoticeWarnIcon) || !strings.Contains(rateText, staffMarker("backup_reset_rate_limited")) {
			t.Errorf("rate-limited reset redraw missing warning notice:\n%s", rateText)
		}
		if got := rules.GetChatRulesInfo(env.chat.Id).Rules; got != "Should survive rate-limited reset" {
			t.Errorf("rate-limited reset modified rules: %q", got)
		}

		// 5. Command parity: /reset messages rules on an identically seeded chat leaves identical DB state.
		cmdChat := env.addChat()
		cleanupBackupPageChat(t, cmdChat.Id)
		backupSeedSampleData(t, cmdChat, owner.Id)
		cmdClient := phase30NewBackupClient(admin)
		cmdBot := newModuleTestBot(cmdClient)
		cmdOwner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
		if err := backupModule.resetHandler(cmdBot, newModuleMessageContext(cmdBot, cmdChat, cmdOwner, "/reset messages rules")); err != ext.EndGroups {
			t.Fatalf("/reset returned %v", err)
		}
		cmdMsgs := cmdClient.callsFor("sendMessage")
		if len(cmdMsgs) != 1 {
			t.Fatalf("/reset confirm messages = %d, want 1", len(cmdMsgs))
		}
		cmdConfirmKB := cmdMsgs[0].Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
		if err := backupModule.backupCallbackHandler(cmdBot, newModuleCallbackContext(cmdBot, cmdChat, cmdOwner, cmdConfirmKB.InlineKeyboard[0][0].CallbackData)); err != ext.EndGroups {
			t.Fatalf("/reset confirm callback returned %v", err)
		}
		cmdGotMsgs, err := eventmsg.GetFresh(context.Background(), cmdChat.Id)
		if err != nil {
			t.Fatalf("cmd eventmsg.GetFresh: %v", err)
		}
		if _, hasBan := cmdGotMsgs.Lookup(eventmsg.EventBan, eventmsg.DestGroup); hasBan {
			t.Error("/reset did not clear custom ban message")
		}
		if cmdGotMsgs.Options.PlainTone || cmdGotMsgs.Options.PMDetails {
			t.Errorf("/reset did not clear options: %+v", cmdGotMsgs.Options)
		}
		if rules.GetChatRulesInfo(cmdChat.Id).Rules != "" {
			t.Error("/reset did not clear rules")
		}
		if len(notes.GetNotesList(cmdChat.Id, true)) == 0 {
			t.Error("/reset cleared unselected notes")
		}
	})

	t.Run("replay_expired_wrong_user_demoted_owner_and_no_redis", func(t *testing.T) {
		env := newPhase28SettingsEnv(t)
		owner := env.newOwner(env.chat)
		otherChat := env.addChat()
		cleanupBackupPageChat(t, env.chat.Id)
		cleanupBackupPageChat(t, otherChat.Id)
		backupSeedSampleData(t, env.chat, owner.Id)

		env.fake.setMemberRaw(otherChat.Id, owner.Id, copyCreatorJSON(owner.Id))
		env.fake.setCreator(otherChat.Id, owner.Id)

		otherOwner := gotgbot.User{Id: owner.Id + 401, FirstName: "OtherCreator"}
		env.fake.setMemberRaw(env.chat.Id, otherOwner.Id, copyCreatorJSON(otherOwner.Id))

		sampleBkp := backup.NewBackupFormat(env.chat.Id, env.chat.Title, owner.Id, []string{backup.BackupModuleRules})
		sampleBkp.Data[backup.BackupModuleRules] = map[string]any{
			"settings": map[string]any{"rules": "Token-imported rules"},
		}
		impToken, err := settingsBackupPutImportToken(context.Background(), settingsBackupImportRecord{
			UserID:  owner.Id,
			ChatID:  env.chat.Id,
			Modules: []string{backup.BackupModuleRules},
			Backup:  sampleBkp,
		})
		if err != nil {
			t.Fatalf("put import token: %v", err)
		}
		rstToken, err := settingsBackupPutResetToken(context.Background(), settingsBackupResetRecord{
			UserID:  owner.Id,
			ChatID:  env.chat.Id,
			Modules: []string{backup.BackupModuleRules},
		})
		if err != nil {
			t.Fatalf("put reset token: %v", err)
		}

		privMsg := env.updateID()

		// 1. Wrong user & wrong chat on bj and bu -> common_callback_invalid_request and owner's tokens stay intact!
		for _, bad := range []struct {
			who    gotgbot.User
			chatID int64
			action string
			token  string
		}{
			{otherOwner, env.chat.Id, settingsActBackupImportYes, impToken},
			{owner, otherChat.Id, settingsActBackupImportYes, impToken},
			{otherOwner, env.chat.Id, settingsActBackupResetYes, rstToken},
			{owner, otherChat.Id, settingsActBackupResetYes, rstToken},
		} {
			env.pressPrivate(bad.who, privMsg, env.privateFields(bad.chatID, "a", bad.action, "k", bad.token))
			ans, alert := copyLastAnswer(t, env)
			if !alert || !strings.Contains(ans, staffMarker("common_callback_invalid_request")) {
				t.Errorf("wrong bind (%s) answer = (%q, alert=%v), want common_callback_invalid_request", bad.action, ans, alert)
			}
		}
		if !env.redis.Exists(settingsBackupImportTokenKey(impToken)) {
			t.Fatal("wrong-user/chat press invalidated owner's import token")
		}
		if !env.redis.Exists(settingsBackupResetTokenKey(rstToken)) {
			t.Fatal("wrong-user/chat press invalidated owner's reset token")
		}

		// 2. Demoted owner (plain member -> settings_not_admin; non-owner admin -> chat_status_owner_button_error) refused, tokens intact.
		env.fake.setMemberRaw(env.chat.Id, owner.Id, copyMemberJSON(owner.Id))
		for _, tc := range []struct{ act, tok string }{
			{settingsActBackupImportYes, impToken},
			{settingsActBackupResetYes, rstToken},
		} {
			env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", tc.act, "k", tc.tok))
			ans, alert := copyLastAnswer(t, env)
			if !alert || !strings.Contains(ans, staffMarker("settings_not_admin")) {
				t.Errorf("demoted member %s answer = (%q, alert=%v), want settings_not_admin", tc.act, ans, alert)
			}
		}
		env.fake.setMemberRaw(env.chat.Id, owner.Id, copyAdminJSON(owner.Id, true, true))
		for _, tc := range []struct{ act, tok string }{
			{settingsActBackupImportYes, impToken},
			{settingsActBackupResetYes, rstToken},
		} {
			env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", tc.act, "k", tc.tok))
			ans, alert := copyLastAnswer(t, env)
			if !alert || !strings.Contains(ans, staffMarker("chat_status_owner_button_error")) {
				t.Errorf("demoted admin %s answer = (%q, alert=%v), want chat_status_owner_button_error", tc.act, ans, alert)
			}
		}
		if got := rules.GetChatRulesInfo(env.chat.Id).Rules; got != "Group rules line" {
			t.Errorf("demoted owner presses modified rules: %q", got)
		}

		// Restore owner and confirm real owner's tokens STILL succeed once, then replay returns settings_card_expired!
		env.fake.setMemberRaw(env.chat.Id, owner.Id, copyCreatorJSON(owner.Id))
		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActBackupImportYes, "k", impToken))
		if got := rules.GetChatRulesInfo(env.chat.Id).Rules; got != "Token-imported rules" {
			t.Fatalf("real owner bj did not apply import: rules = %q", got)
		}
		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActBackupImportYes, "k", impToken))
		ans, alert := copyLastAnswer(t, env)
		if !alert || !strings.Contains(ans, staffMarker("settings_card_expired")) {
			t.Errorf("bj replay answer = (%q, alert=%v), want settings_card_expired", ans, alert)
		}

		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActBackupResetYes, "k", rstToken))
		if got := rules.GetChatRulesInfo(env.chat.Id).Rules; got != "" {
			t.Fatalf("real owner bu did not apply reset: rules = %q", got)
		}
		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActBackupResetYes, "k", rstToken))
		ans, alert = copyLastAnswer(t, env)
		if !alert || !strings.Contains(ans, staffMarker("settings_card_expired")) {
			t.Errorf("bu replay answer = (%q, alert=%v), want settings_card_expired", ans, alert)
		}

		// 3. TTL expiry via miniredis FastForward -> both bj and bu answer settings_card_expired and change nothing.
		require.NoError(t, rules.SetChatRules(env.chat.Id, "Survive expiry"))
		expImpToken, err := settingsBackupPutImportToken(context.Background(), settingsBackupImportRecord{
			UserID:  owner.Id,
			ChatID:  env.chat.Id,
			Modules: []string{backup.BackupModuleRules},
			Backup:  sampleBkp,
		})
		if err != nil {
			t.Fatalf("put expiring import token: %v", err)
		}
		expRstToken, err := settingsBackupPutResetToken(context.Background(), settingsBackupResetRecord{
			UserID:  owner.Id,
			ChatID:  env.chat.Id,
			Modules: []string{backup.BackupModuleRules},
		})
		if err != nil {
			t.Fatalf("put expiring reset token: %v", err)
		}
		env.redis.FastForward(settingsCardLifetime + time.Second)
		for _, tc := range []struct{ act, tok string }{
			{settingsActBackupImportYes, expImpToken},
			{settingsActBackupResetYes, expRstToken},
		} {
			env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", tc.act, "k", tc.tok))
			ans, alert := copyLastAnswer(t, env)
			if !alert || !strings.Contains(ans, staffMarker("settings_card_expired")) {
				t.Errorf("expired %s answer = (%q, alert=%v), want settings_card_expired", tc.act, ans, alert)
			}
		}
		if got := rules.GetChatRulesInfo(env.chat.Id).Rules; got != "Survive expiry" {
			t.Errorf("expired tokens modified rules: %q", got)
		}

		// 4. Redis unavailable (nil) -> bi answers settings_edit_unavailable and br answers settings_bkp_unavailable.
		prevMarshal, prevMgr, prevClient := cache.GetCacheState()
		cache.SetCacheState(nil, nil, nil)
		origBkpRedis := settingsBackupRedisClient
		settingsBackupRedisClient = func() *redis.Client { return nil }
		t.Cleanup(func() {
			settingsBackupRedisClient = origBkpRedis
			cache.SetCacheState(prevMarshal, prevMgr, prevClient)
		})

		editsBefore := len(env.edits(owner.Id, privMsg))
		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActBackupImport))
		ans, alert = copyLastAnswer(t, env)
		if !alert || !strings.Contains(ans, staffMarker("settings_edit_unavailable")) {
			t.Errorf("no-redis bi answer = (%q, alert=%v), want settings_edit_unavailable", ans, alert)
		}
		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActBackupResetAsk, "m", "fffff"))
		ans, alert = copyLastAnswer(t, env)
		if !alert || !strings.Contains(ans, staffMarker("settings_bkp_unavailable")) {
			t.Errorf("no-redis br answer = (%q, alert=%v), want settings_bkp_unavailable", ans, alert)
		}
		if len(env.edits(owner.Id, privMsg)) != editsBefore {
			t.Error("no-redis bi/br edited message, want no prompt/card drawn")
		}
	})

	t.Run("worst_case_64_byte_callbacks", func(t *testing.T) {
		minChat := strconv.FormatInt(math.MinInt64, 10)
		cases := []struct {
			name   string
			fields map[string]string
		}{
			{"op_ba", map[string]string{"a": settingsActOpen, "c": minChat, "p": settingsPageBackup}},
			{"bv_fffff", map[string]string{"a": settingsActBackupToggle, "c": minChat, "m": "fffff"}},
			{"be_fffff", map[string]string{"a": settingsActBackupExport, "c": minChat, "m": "fffff"}},
			{"bi", map[string]string{"a": settingsActBackupImport, "c": minChat}},
			{"bl_i", map[string]string{"a": settingsActBackupImportCancel, "c": minChat, "t": settingsBackupImportPart}},
			{"bj_token16", map[string]string{"a": settingsActBackupImportYes, "c": minChat, "k": "AbCdEfGhIj0123_-"}},
			{"br_fffff", map[string]string{"a": settingsActBackupResetAsk, "c": minChat, "m": "fffff"}},
			{"bu_token16", map[string]string{"a": settingsActBackupResetYes, "c": minChat, "k": "AbCdEfGhIj0123_-"}},
			{"no_ba", map[string]string{"a": settingsActCancel, "c": minChat, "p": settingsPageBackup}},
		}
		for _, tc := range cases {
			data := encodeCallbackData(settingsCallbackNamespace, tc.fields)
			if data == "" {
				t.Fatalf("%s failed to encode", tc.name)
			}
			if len(data) > 64 {
				t.Errorf("%s callback %q is %d bytes, exceeds Telegram 64-byte cap", tc.name, data, len(data))
			}
			decoded, ok := decodeCallbackData(data, settingsCallbackNamespace)
			if !ok {
				t.Errorf("%s callback %q failed to decode", tc.name, data)
				continue
			}
			if !settingsCheckFields(decoded.Fields["a"], decoded.Fields) {
				t.Errorf("%s callback %q rejected by settingsCheckFields", tc.name, data)
			}
		}
	})

	t.Run("all_8_languages_screen_lengths", func(t *testing.T) {
		withRealLocales(t)
		titles := map[string]string{
			"ampersands": strings.Repeat("&", 64),
			"script":     strings.Repeat("<script>", 8),
			"astral":     strings.Repeat("😀", 64),
			"multibyte":  strings.Repeat("Ж", 64),
		}
		allMods := settingsBackupModules()
		until := time.Date(2026, 11, 23, 12, 5, 0, 0, time.UTC)

		for _, langCode := range []string{"en", "es", "fr", "hi", "id", "pt", "ru", "uk"} {
			tr := i18n.MustNewTranslator(langCode)
			for titleKind, title := range titles {
				view := settingsView{
					Tr:         tr,
					Private:    true,
					OwnerKey:   "c",
					OwnerValue: strconv.FormatInt(math.MinInt64, 10),
					GroupID:    math.MinInt64,
					GroupTitle: title,
					OpenerName: title,
				}
				importNotice := settingsNoticeOK(formatBackupImportSuccess(tr, allMods, backup.ImportReport{
					Skipped:           7,
					LogChannelSkipped: true,
				}))
				resetNotice := settingsNoticeOK(formatBackupResetSuccess(tr, allMods))

				screens := []struct {
					name   string
					screen settingsScreen
				}{
					{"main_all", renderSettingsBackup(view, settingsBackupAllMask, "")},
					{"main_none", renderSettingsBackup(view, 0, "")},
					{"import_wait", renderSettingsBackupImportPrompt(view, until, false)},
					{"import_wait_replaced", renderSettingsBackupImportPrompt(view, until, true)},
					{"import_confirm_20", renderSettingsBackupImportConfirm(view, allMods, "AbCdEfGhIj0123_-")},
					{"reset_confirm_20", renderSettingsBackupResetConfirm(view, allMods, "AbCdEfGhIj0123_-")},
					{"post_import_notice", renderSettingsBackup(view, settingsBackupAllMask, importNotice)},
					{"post_reset_notice", renderSettingsBackup(view, settingsBackupAllMask, resetNotice)},
				}
				for _, sc := range screens {
					if sc.screen.Omitted != 0 {
						t.Errorf("%s/%s/%s omitted %d buttons", langCode, titleKind, sc.name, sc.screen.Omitted)
					}
					if n := panelUTF16Len(sc.screen.Text); n == 0 || n > 3800 {
						t.Errorf("%s/%s/%s UTF-16 len = %d, want 1..3800", langCode, titleKind, sc.name, n)
					}
				}
			}
		}
	})
}
