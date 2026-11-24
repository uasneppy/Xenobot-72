//go:build testtools

package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/stretchr/testify/require"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/backup"
	dbcache "github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/chats"
	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/db/lang"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/db/rules"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
	"github.com/uasneppy/fuku_robot/fuku/utils/cache"
	"github.com/uasneppy/fuku_robot/fuku/utils/ratelimit"
)

// TestPhase30EndToEnd drives the complete Phase 30 user journey (MSG-10 custom messages in
// backups and BKP-01..03 private Backup page: Export, Reset, and Import) through a real
// ext.Dispatcher backed by in-process miniredis (phase28InProcessMiniredis, zero localhost TCP
// dials), fake Telegram HTTP transport, and backupRoundTripFunc.
func TestPhase30EndToEnd(t *testing.T) {
	ctx := context.Background()
	fixedNow := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	withFrozenPunishNow(t, fixedNow)
	withFastPMCopyPacer(t)
	t.Cleanup(pmCopyJobs.wait)

	env := newPhase28SettingsEnv(t)
	withRealLocales(t)

	LoadBackup(env.dispatcher)
	LoadBans(env.dispatcher)
	LoadRules(env.dispatcher)

	titleClient := &copyTitleBotClient{lockdownFake: env.fake, titles: map[int64]string{}}
	e2eClient := newMessagesE2EClient(titleClient)
	env.bot.BotClient = e2eClient
	env.fake.sendDate = func() int64 { return fixedNow.Unix() }

	chat := env.chat
	chat.Title = "Facility <Alpha> & Co"
	env.chat.Title = chat.Title
	titleClient.setTitle(chat.Id, chat.Title)

	cleanupBackupPageChat(t, chat.Id)
	cleanupCopyPageChat(t, chat.Id)
	settingsMsgCleanup(t, chat.Id)
	eventPMCleanup(t, chat.Id)

	owner := env.newOwner(chat)
	allowedAdmin := gotgbot.User{Id: uniqueLinkOwnerID(), FirstName: "AllowedAdmin"}

	// Seed hub roster first (settingsHubSeedGroup runs settingsCleanup, which resets chat_settings_access and chats).
	settingsHubSeedGroup(t, chat, chat.Title, settingsHubCreator(owner))

	if err := chats.EnsureChatInDb(chat.Id, chat.Title); err != nil {
		t.Fatalf("EnsureChatInDb: %v", err)
	}
	if err := lang.ChangeGroupLanguage(chat.Id, "en"); err != nil {
		t.Fatalf("ChangeGroupLanguage(chat, en): %v", err)
	}
	t.Cleanup(func() {
		db.DB.Where("chat_id = ?", chat.Id).Delete(&models.Chat{})
		db.DB.Where("chat_id = ?", chat.Id).Delete(&models.ChatSettingsAccess{})
		db.DB.Where("chat_id = ?", chat.Id).Delete(&models.ApprovedUsers{})
		dbcache.DeleteCache(dbcache.CacheKey("chat_lang", chat.Id))
		dbcache.DeleteCache(dbcache.CacheKey("chat_settings_access", chat.Id))
		dbcache.DeleteCache(dbcache.CacheKey("approvals", chat.Id))
		dbcache.ResetLocalForTest()
	})

	for _, uid := range []int64{owner.Id, allowedAdmin.Id} {
		if err := lang.ChangeUserLanguage(uid, "en"); err != nil {
			t.Fatalf("ChangeUserLanguage(%d, en): %v", uid, err)
		}
		uid := uid
		t.Cleanup(func() {
			db.DB.Where("user_id = ?", uid).Delete(&models.User{})
			dbcache.DeleteCache(dbcache.CacheKey("user_lang", uid))
		})
	}

	env.fake.setBotRole(chat.Id, staffRoleAdmin)
	env.fake.setMemberRaw(chat.Id, owner.Id, copyCreatorJSON(owner.Id))
	env.fake.setCreator(chat.Id, owner.Id)
	env.fake.setMemberRaw(chat.Id, allowedAdmin.Id, copyAdminJSON(allowedAdmin.Id, true, true))
	env.fake.listAdministrator(chat.Id, allowedAdmin.Id)
	env.setAdminsAllowed(chat.Id, true)

	enTr := i18n.MustNewTranslator("en")
	chatTr := i18n.MustNewTranslator(lang.GetGroupLanguage(ctx, chat.Id))

	newTarget := func(first string) gotgbot.User {
		t.Helper()
		u := gotgbot.User{Id: uniqueLinkOwnerID(), FirstName: first}
		e2eClient.register(u)
		env.fake.setProfile(u.Id, u.FirstName, "", "")
		env.fake.setMember(chat.Id, u.Id, staffFakeMember{Status: gotgbot.ChatMemberStatusMember})
		memberReqCleanup(t, nil, []int64{u.Id})
		return u
	}

	sendGroupCommand := func(from gotgbot.User, text string, replyTo *gotgbot.User) {
		t.Helper()
		msg := gotgbot.Message{
			MessageId: env.updateID(),
			Chat:      chat,
			From:      &from,
			Text:      text,
		}
		if replyTo != nil {
			msg.ReplyToMessage = &gotgbot.Message{
				MessageId: env.updateID(),
				Chat:      chat,
				From:      replyTo,
				Text:      "hello",
			}
		}
		if err := env.dispatcher.ProcessUpdate(env.bot, &gotgbot.Update{
			UpdateId: env.updateID(),
			Message:  &msg,
		}, nil); err != nil {
			t.Fatalf("dispatch %q in %d: %v", text, chat.Id, err)
		}
	}

	// =========================================================================
	// Part 1: Configure Custom Messages & Rules on chat
	// =========================================================================
	const (
		customBanGroup = "CustomFacilityBan: {mention} ({reason})"
		customBanPM    = "CustomPMBan: {chatname} — {reason}"
		appealURL      = "https://example.org/appeal"
		initialRules   = "Facility regulation #1"
	)
	if _, err := eventmsg.ReplaceAllValidated(ctx, chat.Id, []eventmsg.Message{
		{
			Event:   eventmsg.EventBan,
			Dest:    eventmsg.DestGroup,
			Text:    customBanGroup,
			Buttons: []models.Button{{Name: "Appeal", Url: appealURL}},
		},
		{
			Event: eventmsg.EventBan,
			Dest:  eventmsg.DestPM,
			Text:  customBanPM,
		},
	}, eventmsg.Options{PlainTone: true}, owner.Id); err != nil {
		t.Fatalf("seed eventmsg: %v", err)
	}
	require.NoError(t, rules.SetChatRules(chat.Id, initialRules))

	// =========================================================================
	// Part 2: Export via Backup Page (be) & /export Parity (BKP-01, MSG-10)
	// =========================================================================
	adminPrivMsg := settingsPrivateOnlyMessage(t, env, allowedAdmin)
	pressAdmin := func(kv ...string) {
		t.Helper()
		env.pressPrivate(allowedAdmin, adminPrivMsg, env.privateFields(chat.Id, kv...))
	}

	pressAdmin("a", settingsActOpen, "p", settingsPageBackup)
	pressAdmin("a", settingsActBackupExport, "m", "fffff")

	docs := env.fake.callsFor("sendDocument")
	if len(docs) != 1 {
		ans, alert := env.lastAnswer()
		t.Fatalf("Part 2 page export sendDocument calls = %d, want 1 (edits=%d lastAnswer=%q alert=%v)", len(docs), len(env.edits(allowedAdmin.Id, adminPrivMsg)), ans, alert)
	}
	if gotChat := staffParamInt(docs[0].Params, "chat_id"); gotChat != allowedAdmin.Id {
		t.Fatalf("Part 2 page export chat_id = %d, want admin private chat %d (never group %d)", gotChat, allowedAdmin.Id, chat.Id)
	}
	pageBkp, _ := backupDecodeSentDocument(t, docs[0])
	if len(pageBkp.Modules) != 20 {
		t.Errorf("Part 2 exported module count = %d, want 20 (%v)", len(pageBkp.Modules), pageBkp.Modules)
	}
	if !slices.Contains(pageBkp.Modules, backup.BackupModuleMessages) || !slices.Contains(pageBkp.Modules, backup.BackupModuleRules) {
		t.Fatalf("Part 2 exported modules missing messages/rules: %v", pageBkp.Modules)
	}
	pageMsgRaw, err := json.Marshal(pageBkp.Data[backup.BackupModuleMessages])
	if err != nil {
		t.Fatalf("marshal page messages data: %v", err)
	}
	for _, want := range []string{customBanGroup, customBanPM, appealURL, `"plain_tone":true`} {
		if !strings.Contains(string(pageMsgRaw), want) {
			t.Errorf("Part 2 exported messages JSON missing %q:\n%s", want, pageMsgRaw)
		}
	}
	pageRulesRaw, _ := json.Marshal(pageBkp.Data[backup.BackupModuleRules])
	if !strings.Contains(string(pageRulesRaw), initialRules) {
		t.Errorf("Part 2 exported rules JSON missing %q:\n%s", initialRules, pageRulesRaw)
	}

	// Release export cooldown and run /export in chat through the real dispatcher -> exact parity.
	ratelimit.GetBackupRateLimiter().ReleaseExport(chat.Id)
	sendGroupCommand(allowedAdmin, "/export", nil)

	docs = env.fake.callsFor("sendDocument")
	if len(docs) != 2 {
		t.Fatalf("Part 2 /export sendDocument calls = %d, want 2", len(docs))
	}
	if gotChat := staffParamInt(docs[1].Params, "chat_id"); gotChat != allowedAdmin.Id {
		t.Fatalf("Part 2 /export chat_id = %d, want admin private chat %d", gotChat, allowedAdmin.Id)
	}
	cmdBkp, _ := backupDecodeSentDocument(t, docs[1])
	if !slices.Equal(pageBkp.Modules, cmdBkp.Modules) {
		t.Errorf("Part 2 module list parity mismatch:\npage=%v\ncmd =%v", pageBkp.Modules, cmdBkp.Modules)
	}
	for _, mod := range []string{backup.BackupModuleMessages, backup.BackupModuleRules} {
		pJSON, _ := json.Marshal(pageBkp.Data[mod])
		cJSON, _ := json.Marshal(cmdBkp.Data[mod])
		if string(pJSON) != string(cJSON) {
			t.Errorf("Part 2 parity mismatch for %s:\npage=%s\ncmd =%s", mod, pJSON, cJSON)
		}
	}

	// =========================================================================
	// Part 3: Reset via Backup Page (br -> bu) & Live Moderation Proof (BKP-03, P30-02)
	// =========================================================================
	wantOwnerErr, _ := enTr.GetString("chat_status_owner_button_error")
	wantExpired, _ := enTr.GetString("settings_card_expired")

	// Non-owner admin presses br -> refused with chat_status_owner_button_error; nothing changes.
	pressAdmin("a", settingsActBackupResetAsk, "m", "fffff")
	if ans, alert := copyLastAnswer(t, env); !alert || ans != wantOwnerErr {
		t.Errorf("Part 3 non-owner br answer = (%q, alert=%v), want (%q, true)", ans, alert, wantOwnerErr)
	}
	if gotRules := rules.GetChatRulesInfo(chat.Id).Rules; gotRules != initialRules {
		t.Fatalf("Part 3 non-owner br altered rules: %q", gotRules)
	}

	// Live owner opens private /settings -> ba -> presses br (m=fffff) -> two-tap confirm card -> bu.
	ownerPrivMsg := settingsPrivateOnlyMessage(t, env, owner)
	pressOwner := func(kv ...string) {
		t.Helper()
		env.pressPrivate(owner, ownerPrivMsg, env.privateFields(chat.Id, kv...))
	}

	pressOwner("a", settingsActOpen, "p", settingsPageBackup)
	pressOwner("a", settingsActBackupResetAsk, "m", "fffff")
	resetCardText, resetCardKB := env.lastEdit(owner.Id, ownerPrivMsg)
	if !strings.Contains(resetCardText, "• messages") || !strings.Contains(resetCardText, "• rules") {
		t.Errorf("Part 3 reset confirm card missing module list:\n%s", resetCardText)
	}
	resetYesBtn, ok := settingsAdvFindButton(t, resetCardKB, map[string]string{"a": settingsActBackupResetYes})
	if !ok {
		t.Fatal("Part 3 reset confirm card missing bu Yes button")
	}

	env.pressRaw(settingsPrivateChat(owner), owner, ownerPrivMsg, resetYesBtn.CallbackData)

	// Verify custom messages, options, and rules are cleared in both DB and cache.
	freshAfterReset, err := eventmsg.GetFresh(ctx, chat.Id)
	if err != nil {
		t.Fatalf("Part 3 eventmsg.GetFresh: %v", err)
	}
	cachedAfterReset, err := eventmsg.Get(ctx, chat.Id)
	if err != nil {
		t.Fatalf("Part 3 eventmsg.Get: %v", err)
	}
	for label, msgs := range map[string]*eventmsg.Messages{"fresh": freshAfterReset, "cached": cachedAfterReset} {
		if _, has := msgs.Lookup(eventmsg.EventBan, eventmsg.DestGroup); has {
			t.Errorf("Part 3 %s still has EventBan|group after reset", label)
		}
		if _, has := msgs.Lookup(eventmsg.EventBan, eventmsg.DestPM); has {
			t.Errorf("Part 3 %s still has EventBan|pm after reset", label)
		}
		if msgs.Options.PlainTone || msgs.Options.PMDetails || msgs.Options.PMLog {
			t.Errorf("Part 3 %s options not cleared after reset: %+v", label, msgs.Options)
		}
	}
	if gotRules := rules.GetChatRulesInfo(chat.Id).Rules; gotRules != "" {
		t.Errorf("Part 3 rules after reset = %q, want empty", gotRules)
	}

	// Live /ban spam through the real dispatcher falls back to standard default reply (not custom,
	// not plain tone) and delivers 0 PM copies.
	target1 := newTarget("ResetTarget")
	groupSendsBefore := len(callsToChat(env.fake.staffBotClient, "sendMessage", chat.Id))
	t1PMBefore := len(callsToChat(env.fake.staffBotClient, "sendMessage", target1.Id))

	sendGroupCommand(allowedAdmin, "/ban spam", &target1)
	waitEventPM(t)

	groupSends := callsToChat(env.fake.staffBotClient, "sendMessage", chat.Id)
	if len(groupSends) != groupSendsBefore+1 {
		t.Fatalf("Part 3 /ban group sendMessage delta = %d, want 1", len(groupSends)-groupSendsBefore)
	}
	gotDefaultBan := fmt.Sprint(groupSends[len(groupSends)-1].Params["text"])
	wantDefaultBan, _ := banDefaultReply(chatTr, eventDefaultInput{Target: target1, Reason: "spam"})
	if gotDefaultBan != wantDefaultBan {
		t.Errorf("Part 3 /ban after reset = %q, want standard default %q", gotDefaultBan, wantDefaultBan)
	}
	if gotPMs := len(callsToChat(env.fake.staffBotClient, "sendMessage", target1.Id)) - t1PMBefore; gotPMs != 0 {
		t.Errorf("Part 3 /ban after reset sent %d PM copies, want 0", gotPMs)
	}

	// Replaying bu answers settings_card_expired alert and changes nothing.
	env.pressRaw(settingsPrivateChat(owner), owner, ownerPrivMsg, resetYesBtn.CallbackData)
	if ans, alert := copyLastAnswer(t, env); !alert || ans != wantExpired {
		t.Errorf("Part 3 replayed bu answer = (%q, alert=%v), want (%q, true)", ans, alert, wantExpired)
	}

	// =========================================================================
	// Part 4: Import via Backup Page (bi -> private .json document -> bj)
	//         with Editor Checks & Live Moderation Proof (BKP-02, MSG-10)
	// =========================================================================
	msgMap, ok := pageBkp.Data[backup.BackupModuleMessages].(map[string]any)
	if !ok {
		t.Fatalf("Part 4 exported messages payload = %T, want map[string]any", pageBkp.Data[backup.BackupModuleMessages])
	}
	rawMsgRows, _ := msgMap["messages"].([]any)
	rawMsgRows = append(rawMsgRows,
		map[string]any{
			"event": eventmsg.EventKick,
			"dest":  eventmsg.DestGroup,
			"text":  "Bad kick button",
			"buttons": []any{
				map[string]any{"name": "Evil", "url": "javascript:evil()"},
			},
		},
		map[string]any{
			"event": "future_event_key",
			"dest":  eventmsg.DestGroup,
			"text":  "Future event text",
		},
	)
	msgMap["messages"] = rawMsgRows
	importPayload, err := pageBkp.ToJSON()
	if err != nil {
		t.Fatalf("Part 4 marshal modified backup: %v", err)
	}

	backupSetDownloadPayload(t, map[string][]byte{
		"backups/e2e_import.json": importPayload,
	})
	env.fake.responses["getFile"] = json.RawMessage(`{"file_id":"e2e-import-doc","file_path":"backups/e2e_import.json"}`)

	// Non-owner admin pressing bi -> refused with chat_status_owner_button_error.
	pressAdmin("a", settingsActBackupImport)
	if ans, alert := copyLastAnswer(t, env); !alert || ans != wantOwnerErr {
		t.Errorf("Part 4 non-owner bi answer = (%q, alert=%v), want (%q, true)", ans, alert, wantOwnerErr)
	}
	if env.redis.Exists(settingsInputKey(allowedAdmin.Id)) {
		t.Error("Part 4 non-owner bi wrote Redis wait")
	}

	// Live owner presses bi -> waiting prompt appears.
	pressOwner("a", settingsActBackupImport)
	waitPromptText, _ := env.lastEdit(owner.Id, ownerPrivMsg)
	wantWaitText, _ := enTr.GetString("settings_bkp_import_wait")
	if !strings.Contains(waitPromptText, wantWaitText) {
		t.Errorf("Part 4 bi waiting prompt missing %q:\n%s", wantWaitText, waitPromptText)
	}

	// Owner sends plain text first -> refused with settings_bkp_import_need_file, wait kept alive!
	sentBefore := len(env.fake.sentTo(owner.Id))
	sendPrivateMessage(env, owner, gotgbot.Message{Text: "not a json file"})
	plainReplies := env.fake.sentTo(owner.Id)[sentBefore:]
	wantNeedFile, _ := enTr.GetString("settings_bkp_import_need_file")
	if len(plainReplies) != 1 || !strings.Contains(sentText(plainReplies[0]), wantNeedFile) {
		t.Errorf("Part 4 plain text reply = %+v, want %q", plainReplies, wantNeedFile)
	}
	if !env.redis.Exists(settingsInputKey(owner.Id)) {
		t.Fatal("Part 4 plain text refusal dropped Redis wait")
	}

	// Owner sends .json document in private chat -> door downloads/parses, writes Redis import token,
	// closes waiting prompt, and sends two-tap Import confirm card.
	sentBefore = len(env.fake.sentTo(owner.Id))
	sendPrivateMessage(env, owner, gotgbot.Message{
		Document: &gotgbot.Document{FileId: "e2e-import-doc", FileName: "e2e_import.json"},
	})
	if env.redis.Exists(settingsInputKey(owner.Id)) {
		t.Error("Part 4 valid document upload left wait in Redis")
	}
	closedText, _ := env.lastEdit(owner.Id, ownerPrivMsg)
	wantClosed, _ := enTr.GetString("settings_edit_prompt_closed")
	if !strings.Contains(closedText, wantClosed) {
		t.Errorf("Part 4 waiting prompt not closed after upload:\n%s", closedText)
	}
	cardMsgs := env.fake.sentTo(owner.Id)[sentBefore:]
	if len(cardMsgs) != 1 {
		t.Fatalf("Part 4 expected 1 import confirm card, got %d", len(cardMsgs))
	}
	importCardKB, ok := cardMsgs[0].Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
	if !ok {
		t.Fatal("Part 4 import confirm card missing InlineKeyboardMarkup")
	}
	importYesBtn, ok := settingsAdvFindButton(t, importCardKB.InlineKeyboard, map[string]string{"a": settingsActBackupImportYes})
	if !ok {
		t.Fatal("Part 4 import confirm card missing bj Yes button")
	}
	importToken := settingsButtonFields(t, importYesBtn)["k"]
	if !env.redis.Exists(settingsBackupImportTokenKey(importToken)) {
		t.Fatalf("Part 4 Redis key %q missing", settingsBackupImportTokenKey(importToken))
	}

	// Live owner presses Yes (bj) -> 20 modules imported, 2 invalid entries skipped!
	importCardMsgID := env.updateID()
	env.pressRaw(settingsPrivateChat(owner), owner, importCardMsgID, importYesBtn.CallbackData)
	doneImportText, _ := env.lastEdit(owner.Id, importCardMsgID)
	wantSkipped2, _ := enTr.GetString("backup_import_skipped_entries", i18n.TranslationParams{"count": 2})
	if !strings.Contains(doneImportText, "20") || !strings.Contains(doneImportText, wantSkipped2) {
		t.Errorf("Part 4 post-import notice missing 20 modules / %q:\n%s", wantSkipped2, doneImportText)
	}
	restoredMsgs, err := eventmsg.GetFresh(ctx, chat.Id)
	if err != nil {
		t.Fatalf("Part 4 eventmsg.GetFresh: %v", err)
	}
	if _, hasKick := restoredMsgs.Lookup(eventmsg.EventKick, eventmsg.DestGroup); hasKick {
		t.Error("Part 4 invalid EventKick javascript: row should have been skipped")
	}
	if _, hasFuture := restoredMsgs.Lookup("future_event_key", eventmsg.DestGroup); hasFuture {
		t.Error("Part 4 unknown future_event_key row should have been skipped")
	}
	if !restoredMsgs.Options.PlainTone {
		t.Errorf("Part 4 PlainTone = false, want true")
	}
	if gotRules := rules.GetChatRulesInfo(chat.Id).Rules; gotRules != initialRules {
		t.Errorf("Part 4 restored rules = %q, want %q", gotRules, initialRules)
	}

	// Live /ban restored through the real dispatcher -> sends restored custom group ban message
	// with Appeal URL button AND restored custom PM copy!
	target2 := newTarget("RestoredTarget")
	groupSendsBefore = len(callsToChat(env.fake.staffBotClient, "sendMessage", chat.Id))
	t2PMBefore := len(callsToChat(env.fake.staffBotClient, "sendMessage", target2.Id))

	sendGroupCommand(allowedAdmin, "/ban restored", &target2)
	waitEventPM(t)

	groupSends = callsToChat(env.fake.staffBotClient, "sendMessage", chat.Id)
	if len(groupSends) != groupSendsBefore+1 {
		t.Fatalf("Part 4 /ban restored group sendMessage delta = %d, want 1", len(groupSends)-groupSendsBefore)
	}
	restoredBanCall := groupSends[len(groupSends)-1]
	restoredBanText := fmt.Sprint(restoredBanCall.Params["text"])
	if !strings.HasPrefix(restoredBanText, "CustomFacilityBan:") || !strings.Contains(restoredBanText, "(restored)") {
		t.Errorf("Part 4 /ban restored group text = %q, want custom template with (restored)", restoredBanText)
	}
	restoredBanKB := appealCallKeyboard(t, restoredBanCall)
	foundAppeal := false
	for _, btn := range settingsAllButtons(restoredBanKB) {
		if btn.Url == appealURL {
			foundAppeal = true
			break
		}
	}
	if !foundAppeal {
		t.Errorf("Part 4 /ban restored keyboard missing %q button: %+v", appealURL, restoredBanKB)
	}

	t2PMSends := callsToChat(env.fake.staffBotClient, "sendMessage", target2.Id)
	if len(t2PMSends) != t2PMBefore+1 {
		t.Fatalf("Part 4 /ban restored PM delta = %d, want 1", len(t2PMSends)-t2PMBefore)
	}
	restoredPMText := fmt.Sprint(t2PMSends[len(t2PMSends)-1].Params["text"])
	wantPM := "CustomPMBan: Facility &lt;Alpha&gt; &amp; Co — restored"
	if restoredPMText != wantPM {
		t.Errorf("Part 4 /ban restored PM text = %q, want %q", restoredPMText, wantPM)
	}

	// Replaying bj answers settings_card_expired alert and changes nothing.
	env.pressRaw(settingsPrivateChat(owner), owner, importCardMsgID, importYesBtn.CallbackData)
	if ans, alert := copyLastAnswer(t, env); !alert || ans != wantExpired {
		t.Errorf("Part 4 replayed bj answer = (%q, alert=%v), want (%q, true)", ans, alert, wantExpired)
	}

	// =========================================================================
	// Part 5: Old Export Backward Compatibility E2E (MSG-10)
	// =========================================================================
	// Release import cooldown for chat so the old-export import runs immediately.
	importCooldownKey := "backup:import:" + strconv.FormatInt(chat.Id, 10)
	if m := cache.GetMarshal(); m != nil {
		_ = m.Delete(ctx, importCooldownKey)
	}
	if rc := cache.GetRedisClient(); rc != nil {
		_ = rc.Del(ctx, importCooldownKey).Err()
	}

	const legacyRules = "Legacy regulation #2"
	oldExportJSON := []byte(fmt.Sprintf(`{
		"version": "1.0",
		"exported_at": "2026-11-23T12:00:00Z",
		"chat_id": %d,
		"chat_name": "Old Facility",
		"exported_by": %d,
		"bot_name": "Xenobot-72",
		"modules": ["rules"],
		"data": {
			"rules": {"settings": {"rules": %q}}
		}
	}`, chat.Id, owner.Id, legacyRules))

	backupSetDownloadPayload(t, map[string][]byte{
		"backups/e2e_old.json": oldExportJSON,
	})
	env.fake.responses["getFile"] = json.RawMessage(`{"file_id":"e2e-old-doc","file_path":"backups/e2e_old.json"}`)

	pressOwner("a", settingsActBackupImport)
	sentBefore = len(env.fake.sentTo(owner.Id))
	sendPrivateMessage(env, owner, gotgbot.Message{
		Document: &gotgbot.Document{FileId: "e2e-old-doc", FileName: "old_export.json"},
	})
	oldCards := env.fake.sentTo(owner.Id)[sentBefore:]
	if len(oldCards) != 1 {
		t.Fatalf("Part 5 old export confirm cards = %d, want 1", len(oldCards))
	}
	oldCardKB := oldCards[0].Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
	oldYesBtn, ok := settingsAdvFindButton(t, oldCardKB.InlineKeyboard, map[string]string{"a": settingsActBackupImportYes})
	if !ok {
		t.Fatal("Part 5 old export confirm card missing bj Yes button")
	}
	oldCardMsgID := env.updateID()
	env.pressRaw(settingsPrivateChat(owner), owner, oldCardMsgID, oldYesBtn.CallbackData)

	if gotRules := rules.GetChatRulesInfo(chat.Id).Rules; gotRules != legacyRules {
		t.Errorf("Part 5 rules after old-export import = %q, want %q", gotRules, legacyRules)
	}
	// Verify restored custom EventBan group & PM messages and PlainTone=true remain untouched!
	afterOldMsgs, err := eventmsg.GetFresh(ctx, chat.Id)
	if err != nil {
		t.Fatalf("Part 5 eventmsg.GetFresh: %v", err)
	}
	if row, has := afterOldMsgs.Lookup(eventmsg.EventBan, eventmsg.DestGroup); !has || row.Text != customBanGroup {
		t.Errorf("Part 5 old export clobbered EventBan|group: (%+v, ok=%v)", row, has)
	}
	if row, has := afterOldMsgs.Lookup(eventmsg.EventBan, eventmsg.DestPM); !has || row.Text != customBanPM {
		t.Errorf("Part 5 old export clobbered EventBan|pm: (%+v, ok=%v)", row, has)
	}
	if !afterOldMsgs.Options.PlainTone {
		t.Error("Part 5 old export cleared PlainTone, want true preserved")
	}
}
