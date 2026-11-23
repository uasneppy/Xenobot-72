//go:build testtools

package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/backup"
	dbcache "github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/chats"
	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/db/filters"
	"github.com/uasneppy/fuku_robot/fuku/db/greetings"
	"github.com/uasneppy/fuku_robot/fuku/db/lang"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/db/notes"
	"github.com/uasneppy/fuku_robot/fuku/db/rules"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
	"github.com/uasneppy/fuku_robot/fuku/utils/cache"
	"github.com/uasneppy/fuku_robot/fuku/utils/ratelimit"
)

// Phase 30 characterization goldens recorded on unchanged baseline commit 1da5af6f
// (production code baseline 25a4dca7) for MSG-11, MSG-10, BKP-01, BKP-02, BKP-03.
// Frozen before any Phase 30 production change; never edit, weaken, or re-record.
const (
	goldenP30ExportRateLimitedPrefix = "⏳ Wait <b>"
	goldenP30ExportRateLimitedSuffix = "</b> before exporting again."
	goldenP30ExportNoModules         = "❌ No exportable modules found for this chat."
	goldenP30ExportStartBot          = "📬 The backup is sent to you in a private chat. Start a private chat with me first, then send /export again."

	goldenP30ImportOwnerOnly   = "❌ Access denied. Only the group owner can import settings."
	goldenP30ImportNoReply     = "❌ Reply to a backup file with <code>/import</code>."
	goldenP30ImportInvalidFile = "❌ Invalid backup file format. Send a valid JSON backup file."
	goldenP30ImportConfirmText = "<b>⚠️ Confirm import</b>\n\nYou are about to import <b>3 modules</b>:\n• rules\n• filters\n• logchannels\n\nThis overwrites the existing settings. Proceed?"
	goldenP30ImportConfirmBtn  = "✅ Confirm Import"
	goldenP30ImportCancelBtn   = "❌ Cancel"
	goldenP30ImportSuccessText = "<b>✅ Import complete.</b>\n\n<b>Modules imported:</b> 3\n\n<b>Modules:</b>\n• rules\n• filters\n• logchannels\n\n⚠️ <b>1</b> filters, blacklist words or notes were left out because they are not valid here.\n⚠️ The log channel settings were not imported: this group has no log channel. Link one with /setlog, then import again."
	goldenP30ImportExpiredText = "❌ Import request expired. Start again with <code>/import</code>."
	goldenP30ImportCancelText  = "❌ Import cancelled."

	goldenP30ResetOwnerOnly   = "Access denied. Only the group owner can do this."
	goldenP30ResetConfirmText = "<b>⚠️ Confirm reset</b>\n\nYou are about to reset <b>2 modules</b>:\n• notes\n• rules\n\n<b>This cannot be undone.</b> Proceed?"
	goldenP30ResetConfirmBtn  = "⚠️ Yes, Reset All"
	goldenP30ResetCancelBtn   = "❌ Cancel"
	goldenP30ResetSuccessText = "<b>✅ Reset complete.</b>\n\n<b>Modules reset:</b> 2\n\n<b>Modules:</b>\n• notes\n• rules"
	goldenP30ResetExpiredText = "❌ Reset request expired. Start again with <code>/reset</code>."
	goldenP30ResetCancelText  = "❌ Reset cancelled."
)

func phase30NewEnglishChat(t *testing.T, title string) gotgbot.Chat {
	t.Helper()
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: title}
	require.NoError(t, chats.EnsureChatInDb(chat.Id, chat.Title))
	require.NoError(t, lang.ChangeGroupLanguage(chat.Id, "en"))
	phase30CleanupChat(t, chat.Id)
	return chat
}

func phase30CleanupChat(t *testing.T, chatID int64) {
	t.Helper()
	t.Cleanup(func() {
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
		_ = filters.RemoveAllFilters(chatID)
		if db.DB != nil {
			db.DB.Where("chat_id = ?", chatID).Delete(&models.RulesSettings{})
			db.DB.Where("chat_id = ?", chatID).Delete(&models.GreetingSettings{})
			db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatEventMessage{})
			db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatEventMessageOptions{})
			db.DB.Where("chat_id = ?", chatID).Delete(&db.Chat{})
		}
		dbcache.DeleteCache(eventmsg.CacheKey(chatID))
		dbcache.DeleteCache(fmt.Sprintf("rules_%d", chatID))
		dbcache.DeleteCache(fmt.Sprintf("greetings_%d", chatID))
		dbcache.DeleteCache(fmt.Sprintf("group_lang_%d", chatID))
	})
}

func phase30NewBackupClient(nonOwnerAdmin gotgbot.User) *moduleBotClient {
	client := newModuleBotClient()
	client.responses["getChatMember"] = json.RawMessage(fmt.Sprintf(
		`{"status":"administrator","user":{"id":%d,"is_bot":false,"first_name":%q},"can_change_info":true,"can_delete_messages":true,"can_restrict_members":true}`,
		nonOwnerAdmin.Id,
		nonOwnerAdmin.FirstName,
	))
	client.responses["getChatAdministrators"] = json.RawMessage(fmt.Sprintf(
		`[{"status":"creator","user":{"id":777000,"is_bot":false,"first_name":"Telegram"}},{"status":"administrator","user":{"id":%d,"is_bot":false,"first_name":%q},"can_change_info":true,"can_delete_messages":true,"can_restrict_members":true},{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Fuku"},"can_pin_messages":true,"can_delete_messages":true,"can_restrict_members":true,"can_promote_members":true,"can_change_info":true,"can_invite_users":true,"can_manage_chat":true}]`,
		nonOwnerAdmin.Id,
		nonOwnerAdmin.FirstName,
	))
	return client
}

func TestLegacyPhase30Baseline(t *testing.T) {
	withRealLocales(t)
	tr := i18n.MustNewTranslator("en")
	fixedTime := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	phase29WithFixedUTCClock(t, fixedTime)

	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	nonOwnerAdmin := gotgbot.User{Id: 50301, FirstName: "Moderator"}

	t.Run("export_command_private_delivery_refusals_and_rate_limit", func(t *testing.T) {
		chat := phase30NewEnglishChat(t, "Phase 30 Backup Group")
		require.NoError(t, notes.AddNote(chat.Id, "welcome", "hello", "", nil, db.TEXT, false, false, false, true, false, false))
		rules.SetChatRules(chat.Id, "Be kind")

		client := phase30NewBackupClient(nonOwnerAdmin)
		bot := newModuleTestBot(client)

		err := backupModule.exportHandler(bot, newModuleMessageContext(bot, chat, owner, "/export notes rules"))
		require.Equal(t, ext.EndGroups, err)

		docs := client.callsFor("sendDocument")
		require.Len(t, docs, 1)
		assert.Empty(t, client.callsFor("sendMessage"))
		assert.Equal(t, owner.Id, staffParamInt(docs[0].Params, "chat_id"), "backup document must go to admin private chat, never the group")
		assert.Equal(t, "HTML", fmt.Sprint(docs[0].Params["parse_mode"]))

		reader, ok := docs[0].Params["document"].(*gotgbot.FileReader)
		require.True(t, ok, "expected *gotgbot.FileReader document payload")
		assert.True(t, strings.HasPrefix(reader.Name, fmt.Sprintf("xenobot72_backup_%d_", chat.Id)))
		assert.True(t, strings.HasSuffix(reader.Name, ".json"))

		rawJSON, readErr := io.ReadAll(reader.Data)
		require.NoError(t, readErr)
		exported, parseErr := backup.BackupFormatFromJSON(rawJSON)
		require.NoError(t, parseErr)
		assert.Equal(t, []string{"notes", "rules"}, exported.Modules)
		assert.Equal(t, chat.Id, exported.ChatID)
		assert.Equal(t, chat.Title, exported.ChatName)
		assert.Equal(t, owner.Id, exported.ExportedBy)
		assert.Len(t, exported.Data, 2)
		assert.Contains(t, exported.Data, "notes")
		assert.Contains(t, exported.Data, "rules")

		wantCaption := fmt.Sprintf(
			"<b>✅ Export complete.</b>\n\n<b>Chat:</b> %s\n<b>Modules exported:</b> 2\n<b>Exported at:</b> %s (UTC)\n\n<b>Modules:</b>\n• notes\n• rules",
			chat.Title,
			exported.ExportedAt.UTC().Format("2006-01-02 15:04:05"),
		)
		assert.Equal(t, wantCaption, fmt.Sprint(docs[0].Params["caption"]))

		// Immediate second export hits cooldown: 0 new documents and exact English cooldown wrapper.
		err = backupModule.exportHandler(bot, newModuleMessageContext(bot, chat, owner, "/export notes"))
		require.Equal(t, ext.EndGroups, err)
		assert.Len(t, client.callsFor("sendDocument"), 1)
		msgs := client.callsFor("sendMessage")
		require.Len(t, msgs, 1)
		assert.Equal(t, chat.Id, staffParamInt(msgs[0].Params, "chat_id"))
		rateText := fmt.Sprint(msgs[0].Params["text"])
		assert.True(t, strings.HasPrefix(rateText, goldenP30ExportRateLimitedPrefix), "rate-limit reply = %q", rateText)
		assert.True(t, strings.HasSuffix(rateText, goldenP30ExportRateLimitedSuffix), "rate-limit reply = %q", rateText)
		innerCooldown := strings.TrimSuffix(strings.TrimPrefix(rateText, goldenP30ExportRateLimitedPrefix), goldenP30ExportRateLimitedSuffix)
		assert.NotEmpty(t, innerCooldown)
		assert.Contains(t, innerCooldown, "minute")

		// Release cooldown and test unknown module refusal.
		ratelimit.GetBackupRateLimiter().ReleaseExport(chat.Id)
		err = backupModule.exportHandler(bot, newModuleMessageContext(bot, chat, owner, "/export unknownmod"))
		require.Equal(t, ext.EndGroups, err)
		assert.Len(t, client.callsFor("sendDocument"), 1)
		msgs = client.callsFor("sendMessage")
		require.Len(t, msgs, 2)
		assert.Equal(t, goldenP30ExportNoModules, fmt.Sprint(msgs[1].Params["text"]))

		// Simulate 403 Forbidden on sendDocument: exact start-bot reply and cooldown released for retry.
		client.errors["sendDocument"] = &gotgbot.TelegramError{
			Code:        403,
			Description: "Forbidden: bot can't initiate conversation with a user",
		}
		err = backupModule.exportHandler(bot, newModuleMessageContext(bot, chat, owner, "/export notes"))
		require.Equal(t, ext.EndGroups, err)
		for _, doc := range client.callsFor("sendDocument") {
			assert.NotEqual(t, chat.Id, staffParamInt(doc.Params, "chat_id"))
		}
		msgs = client.callsFor("sendMessage")
		require.Len(t, msgs, 3)
		assert.Equal(t, chat.Id, staffParamInt(msgs[2].Params, "chat_id"))
		assert.Equal(t, goldenP30ExportStartBot, fmt.Sprint(msgs[2].Params["text"]))

		delete(client.errors, "sendDocument")
		err = backupModule.exportHandler(bot, newModuleMessageContext(bot, chat, owner, "/export notes"))
		require.Equal(t, ext.EndGroups, err)
		assert.Len(t, client.callsFor("sendDocument"), 3)
		assert.Len(t, client.callsFor("sendMessage"), 3)
	})

	t.Run("import_command_owner_gate_confirm_card_skipped_report_and_cancel", func(t *testing.T) {
		chat := phase30NewEnglishChat(t, "Phase 30 Import Group")
		client := phase30NewBackupClient(nonOwnerAdmin)
		bot := newModuleTestBot(client)

		// 1. Non-owner admin is refused.
		err := backupModule.importHandler(bot, newModuleMessageContext(bot, chat, nonOwnerAdmin, "/import"))
		require.Equal(t, ext.EndGroups, err)
		msgs := client.callsFor("sendMessage")
		require.Len(t, msgs, 1)
		assert.Equal(t, goldenP30ImportOwnerOnly, fmt.Sprint(msgs[0].Params["text"]))

		// 2. Owner without replying to a document gets exact no-reply refusal.
		err = backupModule.importHandler(bot, newModuleMessageContext(bot, chat, owner, "/import"))
		require.Equal(t, ext.EndGroups, err)
		msgs = client.callsFor("sendMessage")
		require.Len(t, msgs, 2)
		assert.Equal(t, goldenP30ImportNoReply, fmt.Sprint(msgs[1].Params["text"]))

		// 3. Owner replying to a non-.json file gets invalid-file refusal with 0 getFile calls.
		txtCtx := newModuleMessageContext(bot, chat, owner, "/import")
		txtCtx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
			MessageId: 201,
			Date:      1,
			Chat:      chat,
			Document: &gotgbot.Document{
				FileId:   "txt-file-id",
				FileName: "backup.txt",
			},
		}
		err = backupModule.importHandler(bot, txtCtx)
		require.Equal(t, ext.EndGroups, err)
		assert.Empty(t, client.callsFor("getFile"))
		msgs = client.callsFor("sendMessage")
		require.Len(t, msgs, 3)
		assert.Equal(t, goldenP30ImportInvalidFile, fmt.Sprint(msgs[2].Params["text"]))

		// 4. Valid backup JSON containing rules, filters (1 valid + 1 invalid button URL), and logchannels.
		bkp := backup.NewBackupFormat(chat.Id, chat.Title, owner.Id, []string{"rules", "filters", "logchannels"})
		bkp.Data["rules"] = map[string]interface{}{
			"settings": map[string]interface{}{
				"rules": "Imported rules",
			},
		}
		bkp.Data["filters"] = map[string]interface{}{
			"filters": []interface{}{
				map[string]interface{}{
					"keyword":      "hi",
					"filter_reply": "hello",
					"msgtype":      db.TEXT,
				},
				map[string]interface{}{
					"keyword":      "badbtn",
					"filter_reply": "unsafe",
					"msgtype":      db.TEXT,
					"filter_buttons": []interface{}{
						map[string]interface{}{
							"name": "Bad",
							"url":  "javascript:alert(1)",
						},
					},
				},
			},
		}
		bkp.Data["logchannels"] = map[string]interface{}{
			"settings": map[string]interface{}{
				"cat_admin": false,
			},
		}
		backupBytes, marshalErr := bkp.ToJSON()
		require.NoError(t, marshalErr)

		oldBaseURL := backupDownloadBaseURL
		oldHTTPClient := backupDownloadHTTPClient
		backupDownloadBaseURL = "https://example.invalid/file/bot"
		backupDownloadHTTPClient = &http.Client{
			Transport: backupRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				assert.Equal(t, "/file/bot999:test/backups/p30.json", req.URL.Path)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader(backupBytes)),
					Header:     make(http.Header),
				}, nil
			}),
		}
		t.Cleanup(func() {
			backupDownloadBaseURL = oldBaseURL
			backupDownloadHTTPClient = oldHTTPClient
		})

		client.responses["getFile"] = json.RawMessage(`{"file_id":"json-file-id","file_path":"backups/p30.json"}`)
		jsonCtx := newModuleMessageContext(bot, chat, owner, "/import")
		jsonCtx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
			MessageId: 202,
			Date:      1,
			Chat:      chat,
			Document: &gotgbot.Document{
				FileId:   "json-file-id",
				FileName: "backup.json",
			},
		}
		err = backupModule.importHandler(bot, jsonCtx)
		require.Equal(t, ext.EndGroups, err)
		assert.Len(t, client.callsFor("getFile"), 1)

		msgs = client.callsFor("sendMessage")
		require.Len(t, msgs, 4)
		assert.Equal(t, goldenP30ImportConfirmText, fmt.Sprint(msgs[3].Params["text"]))

		kb, ok := msgs[3].Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
		require.True(t, ok, "expected InlineKeyboardMarkup on import confirm card")
		require.Len(t, kb.InlineKeyboard, 1)
		require.Len(t, kb.InlineKeyboard[0], 2)
		confirmBtn := kb.InlineKeyboard[0][0]
		cancelBtn := kb.InlineKeyboard[0][1]
		assert.Equal(t, goldenP30ImportConfirmBtn, confirmBtn.Text)
		assert.Equal(t, goldenP30ImportCancelBtn, cancelBtn.Text)

		decConfirm, ok := decodeCallbackData(confirmBtn.CallbackData, "backup")
		require.True(t, ok)
		confirmAction, _ := decConfirm.Field("a")
		assert.Equal(t, backupActionConfirmImport, confirmAction)

		decCancel, ok := decodeCallbackData(cancelBtn.CallbackData, "backup")
		require.True(t, ok)
		cancelAction, _ := decCancel.Field("a")
		assert.Equal(t, backupActionCancelImport, cancelAction)

		// 5. Owner presses Confirm (ci): exact success + skipped + log-channel hints, DB updated.
		err = backupModule.backupCallbackHandler(bot, newModuleCallbackContext(bot, chat, owner, confirmBtn.CallbackData))
		require.Equal(t, ext.EndGroups, err)
		msgs = client.callsFor("sendMessage")
		require.Len(t, msgs, 5)
		assert.Equal(t, goldenP30ImportSuccessText, fmt.Sprint(msgs[4].Params["text"]))
		assert.Equal(t, "Imported rules", rules.GetChatRulesInfo(chat.Id).Rules)
		assert.True(t, filters.DoesFilterExists(chat.Id, "hi"))
		assert.False(t, filters.DoesFilterExists(chat.Id, "badbtn"))

		// 6. Replaying the same Confirm token sends exact expired text.
		err = backupModule.backupCallbackHandler(bot, newModuleCallbackContext(bot, chat, owner, confirmBtn.CallbackData))
		require.Equal(t, ext.EndGroups, err)
		msgs = client.callsFor("sendMessage")
		require.Len(t, msgs, 6)
		assert.Equal(t, goldenP30ImportExpiredText, fmt.Sprint(msgs[5].Params["text"]))

		// 7. Cancel (xi) on a fresh chat answers and edits with exact cancelled text, leaving rules untouched.
		cancelChat := phase30NewEnglishChat(t, "Phase 30 Import Cancel Group")
		rules.SetChatRules(cancelChat.Id, "Keep existing rules")
		cancelClient := phase30NewBackupClient(nonOwnerAdmin)
		cancelClient.responses["getFile"] = json.RawMessage(`{"file_id":"json-file-id","file_path":"backups/p30.json"}`)
		cancelBot := newModuleTestBot(cancelClient)

		cancelImportCtx := newModuleMessageContext(cancelBot, cancelChat, owner, "/import")
		cancelImportCtx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
			MessageId: 203,
			Date:      1,
			Chat:      cancelChat,
			Document: &gotgbot.Document{
				FileId:   "json-file-id",
				FileName: "backup.json",
			},
		}
		err = backupModule.importHandler(cancelBot, cancelImportCtx)
		require.Equal(t, ext.EndGroups, err)
		cancelMsgs := cancelClient.callsFor("sendMessage")
		require.Len(t, cancelMsgs, 1)
		cancelKB, ok := cancelMsgs[0].Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
		require.True(t, ok)
		cancelData := cancelKB.InlineKeyboard[0][1].CallbackData

		err = backupModule.backupCallbackHandler(cancelBot, newModuleCallbackContext(cancelBot, cancelChat, owner, cancelData))
		require.Equal(t, ext.EndGroups, err)
		answers := cancelClient.callsFor("answerCallbackQuery")
		require.Len(t, answers, 1)
		assert.Equal(t, goldenP30ImportCancelText, fmt.Sprint(answers[0].Params["text"]))
		edits := cancelClient.callsFor("editMessageText")
		require.Len(t, edits, 1)
		assert.Equal(t, goldenP30ImportCancelText, fmt.Sprint(edits[0].Params["text"]))
		assert.Equal(t, "Keep existing rules", rules.GetChatRulesInfo(cancelChat.Id).Rules)
	})

	t.Run("reset_command_owner_gate_confirm_card_and_execution", func(t *testing.T) {
		chat := phase30NewEnglishChat(t, "Phase 30 Reset Group")
		require.NoError(t, notes.AddNote(chat.Id, "welcome", "hello", "", nil, db.TEXT, false, false, false, true, false, false))
		rules.SetChatRules(chat.Id, "Resettable rules")

		client := phase30NewBackupClient(nonOwnerAdmin)
		bot := newModuleTestBot(client)

		// 1. Non-owner admin is refused by owner gate.
		err := backupModule.resetHandler(bot, newModuleMessageContext(bot, chat, nonOwnerAdmin, "/reset notes rules"))
		require.Equal(t, ext.EndGroups, err)
		msgs := client.callsFor("sendMessage")
		require.Len(t, msgs, 1)
		assert.Equal(t, goldenP30ResetOwnerOnly, fmt.Sprint(msgs[0].Params["text"]))

		// 2. Owner gets exact confirmation card and two buttons (cr / xr).
		err = backupModule.resetHandler(bot, newModuleMessageContext(bot, chat, owner, "/reset notes rules"))
		require.Equal(t, ext.EndGroups, err)
		msgs = client.callsFor("sendMessage")
		require.Len(t, msgs, 2)
		assert.Equal(t, goldenP30ResetConfirmText, fmt.Sprint(msgs[1].Params["text"]))

		kb, ok := msgs[1].Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
		require.True(t, ok, "expected InlineKeyboardMarkup on reset confirm card")
		require.Len(t, kb.InlineKeyboard, 2)
		require.Len(t, kb.InlineKeyboard[0], 1)
		require.Len(t, kb.InlineKeyboard[1], 1)
		confirmBtn := kb.InlineKeyboard[0][0]
		cancelBtn := kb.InlineKeyboard[1][0]
		assert.Equal(t, goldenP30ResetConfirmBtn, confirmBtn.Text)
		assert.Equal(t, goldenP30ResetCancelBtn, cancelBtn.Text)

		decConfirm, ok := decodeCallbackData(confirmBtn.CallbackData, "backup")
		require.True(t, ok)
		confirmAction, _ := decConfirm.Field("a")
		assert.Equal(t, backupActionConfirmReset, confirmAction)

		decCancel, ok := decodeCallbackData(cancelBtn.CallbackData, "backup")
		require.True(t, ok)
		cancelAction, _ := decCancel.Field("a")
		assert.Equal(t, backupActionCancelReset, cancelAction)

		// 3. Owner presses Confirm (cr): exact success reply and DB rows cleared.
		err = backupModule.backupCallbackHandler(bot, newModuleCallbackContext(bot, chat, owner, confirmBtn.CallbackData))
		require.Equal(t, ext.EndGroups, err)
		msgs = client.callsFor("sendMessage")
		require.Len(t, msgs, 3)
		assert.Equal(t, goldenP30ResetSuccessText, fmt.Sprint(msgs[2].Params["text"]))
		assert.Empty(t, notes.GetNotesList(chat.Id, true))
		assert.Empty(t, rules.GetChatRulesInfo(chat.Id).Rules)

		// 4. Replaying Confirm (cr) sends exact expired text.
		err = backupModule.backupCallbackHandler(bot, newModuleCallbackContext(bot, chat, owner, confirmBtn.CallbackData))
		require.Equal(t, ext.EndGroups, err)
		msgs = client.callsFor("sendMessage")
		require.Len(t, msgs, 4)
		assert.Equal(t, goldenP30ResetExpiredText, fmt.Sprint(msgs[3].Params["text"]))

		// 5. Cancel (xr) on a fresh chat answers and edits with exact cancelled text, keeping DB intact.
		cancelChat := phase30NewEnglishChat(t, "Phase 30 Reset Cancel Group")
		require.NoError(t, notes.AddNote(cancelChat.Id, "stay", "keep me", "", nil, db.TEXT, false, false, false, true, false, false))
		rules.SetChatRules(cancelChat.Id, "Stay rules")
		cancelClient := phase30NewBackupClient(nonOwnerAdmin)
		cancelBot := newModuleTestBot(cancelClient)

		err = backupModule.resetHandler(cancelBot, newModuleMessageContext(cancelBot, cancelChat, owner, "/reset notes rules"))
		require.Equal(t, ext.EndGroups, err)
		cancelMsgs := cancelClient.callsFor("sendMessage")
		require.Len(t, cancelMsgs, 1)
		cancelKB, ok := cancelMsgs[0].Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
		require.True(t, ok)
		cancelData := cancelKB.InlineKeyboard[1][0].CallbackData

		err = backupModule.backupCallbackHandler(cancelBot, newModuleCallbackContext(cancelBot, cancelChat, owner, cancelData))
		require.Equal(t, ext.EndGroups, err)
		answers := cancelClient.callsFor("answerCallbackQuery")
		require.Len(t, answers, 1)
		assert.Equal(t, goldenP30ResetCancelText, fmt.Sprint(answers[0].Params["text"]))
		edits := cancelClient.callsFor("editMessageText")
		require.Len(t, edits, 1)
		assert.Equal(t, goldenP30ResetCancelText, fmt.Sprint(edits[0].Params["text"]))
		assert.Equal(t, "Stay rules", rules.GetChatRulesInfo(cancelChat.Id).Rules)
		assert.NotEmpty(t, notes.GetNotesList(cancelChat.Id, true))
	})

	t.Run("old_export_without_messages_imports_cleanly_and_preserves_custom_messages", func(t *testing.T) {
		dstChat := phase30NewEnglishChat(t, "Old Export Target").Id
		bg := context.Background()

		require.NoError(t, eventmsg.SetText(bg, dstChat, eventmsg.EventBan, eventmsg.DestGroup, "Keep custom ban", 1))
		require.NoError(t, eventmsg.SetPMDetails(bg, dstChat, true, 1))
		require.NoError(t, eventmsg.SetPlainTone(bg, dstChat, true, 1))

		oldExportJSON := []byte(`{
			"version": "1.0",
			"exported_at": "2026-11-23T12:00:00Z",
			"chat_id": -100300001,
			"chat_name": "Legacy Source Group",
			"exported_by": 777000,
			"bot_name": "Xenobot-72",
			"modules": ["rules", "greetings"],
			"data": {
				"rules": {
					"settings": {
						"rules": "Pre-Phase-30 rules",
						"private": true
					}
				},
				"greetings": {
					"settings": {
						"welcome_settings": {
							"welcome_enabled": true,
							"welcome_text": "Pre-Phase-30 welcome",
							"welcome_type": 1
						}
					}
				}
			}
		}`)

		oldBkp, err := backup.BackupFormatFromJSON(oldExportJSON)
		require.NoError(t, err)
		require.NoError(t, oldBkp.Validate())
		require.NotContains(t, oldBkp.Modules, "messages")
		require.NotContains(t, oldBkp.Data, "messages")

		report, err := backup.ImportChatDataReport(dstChat, oldBkp, nil)
		require.NoError(t, err)
		assert.Equal(t, 0, report.Skipped)
		assert.False(t, report.LogChannelSkipped)

		assert.Equal(t, "Pre-Phase-30 rules", rules.GetChatRulesInfo(dstChat).Rules)
		greet := greetings.GetGreetingSettings(dstChat)
		require.NotNil(t, greet)
		require.NotNil(t, greet.WelcomeSettings)
		assert.Equal(t, "Pre-Phase-30 welcome", greet.WelcomeSettings.WelcomeText)

		// Pre-Phase-30 export without "messages" must not touch chat_event_messages or chat_event_message_options.
		gotMsgs, err := eventmsg.GetFresh(bg, dstChat)
		require.NoError(t, err)
		banMsg, ok := gotMsgs.Lookup(eventmsg.EventBan, eventmsg.DestGroup)
		require.True(t, ok, "custom ban message must survive import of old backup without messages module")
		assert.Equal(t, "Keep custom ban", banMsg.Text)
		assert.True(t, gotMsgs.Options.PMDetails)
		assert.True(t, gotMsgs.Options.PlainTone)
	})

	t.Run("unconfigured_chat_default_ban_reply_unchanged", func(t *testing.T) {
		chat := phase30NewEnglishChat(t, "Unconfigured Chat")
		got, opts := banDefaultReply(tr, phase29FixedDeterministicInput())
		assert.Equal(t, goldenP29GroupDefaults[eventmsg.EventBan], got)
		require.NotNil(t, opts)

		var sentText string
		site := eventSite{
			Bot:  newModuleTestBot(newModuleBotClient()),
			Chat: &chat,
			Tr:   tr,
			SendDefault: func(text string, _ *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
				sentText = text
				return &gotgbot.Message{MessageId: 404}, nil
			},
		}
		_, err := postEventSent(site, eventReply{
			Event:   eventmsg.EventBan,
			Target:  phase29FixedDeterministicInput().Target,
			Values:  eventValues{Reason: "spam"},
			Default: got,
		})
		require.NoError(t, err)
		assert.Equal(t, goldenP29GroupDefaults[eventmsg.EventBan], sentText)
	})
}
