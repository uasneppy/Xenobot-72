//go:build testtools

package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
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
	"github.com/uasneppy/fuku_robot/fuku/utils/ratelimit"
)

type backupRoundTripFunc func(*http.Request) (*http.Response, error)

func (f backupRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestBackupModuleStructure(t *testing.T) {
	t.Run("backupModule has correct name", func(t *testing.T) {
		assert.Equal(t, "Backup", backupModule.moduleName)
	})
}

func TestLoadBackupDoesNotRegisterDeadHelpButtons(t *testing.T) {
	registry := DefaultHelpRegistry()
	previousButtons, hadButtons := registry.helpableKb[backupModule.moduleName]
	previousEnabled, hadEnabled := registry.AbleMap[backupModule.moduleName]
	delete(registry.helpableKb, backupModule.moduleName)
	delete(registry.AbleMap, backupModule.moduleName)
	t.Cleanup(func() {
		if hadButtons {
			registry.helpableKb[backupModule.moduleName] = previousButtons
		} else {
			delete(registry.helpableKb, backupModule.moduleName)
		}
		if hadEnabled {
			registry.AbleMap[backupModule.moduleName] = previousEnabled
		} else {
			delete(registry.AbleMap, backupModule.moduleName)
		}
	})

	LoadBackup(ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1}))

	assert.True(t, registry.AbleMap[backupModule.moduleName])
	_, ok := registry.helpableKb[backupModule.moduleName]
	assert.False(t, ok)
}

func TestBuildModuleList(t *testing.T) {
	t.Run("buildModuleList returns empty for empty slice", func(t *testing.T) {
		result := buildModuleList([]string{})
		assert.Equal(t, "", result)
	})

	t.Run("buildModuleList formats correctly", func(t *testing.T) {
		result := buildModuleList([]string{"notes", "filters", "rules"})
		assert.Contains(t, result, "notes")
		assert.Contains(t, result, "filters")
		assert.Contains(t, result, "rules")
		assert.Contains(t, result, "•")
	})

	t.Run("buildModuleList with single module", func(t *testing.T) {
		result := buildModuleList([]string{"notes"})
		assert.Equal(t, "• notes", result)
	})
}

func testTranslator(t *testing.T) *i18n.Translator {
	yaml := `
backup_export_success: "Chat: {chat}, Modules: {modules}, Time: {time}, List: {list}"
backup_import_file_too_large: "File is too large"
backup_import_invalid_file: "Invalid backup file"
backup_import_download_failed: "Download failed"
backup_import_rate_limited: "Wait {time}"
backup_import_expired: "Import expired"
backup_reset_expired: "Reset expired"
common_callback_invalid_request: "Invalid callback"
button_confirm_import: "Confirm Import"
button_cancel_import: "Cancel Import"
button_confirm_reset: "Confirm Reset"
button_cancel_reset: "Cancel Reset"
`
	tr, err := i18n.NewTestTranslator(yaml)
	require.NoError(t, err)
	return tr
}

func TestParseImportModules(t *testing.T) {
	t.Parallel()

	backupData := map[string]interface{}{
		"notes":   map[string]interface{}{"a": 1},
		"filters": map[string]interface{}{"b": 2},
		"rules":   map[string]interface{}{"c": 3},
	}

	t.Run("empty text returns empty slice", func(t *testing.T) {
		got, err := parseImportModules("", backupData)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("no args returns empty slice", func(t *testing.T) {
		got, err := parseImportModules("/import", backupData)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("valid modules only", func(t *testing.T) {
		got, err := parseImportModules("/import notes filters", backupData)
		require.NoError(t, err)
		assert.Equal(t, []string{"notes", "filters"}, got)
	})

	t.Run("invalid modules skipped", func(t *testing.T) {
		got, err := parseImportModules("/import notes invalid rules", backupData)
		require.NoError(t, err)
		assert.Equal(t, []string{"notes", "rules"}, got)
	})

	t.Run("case insensitive", func(t *testing.T) {
		got, err := parseImportModules("/import NOTES", backupData)
		require.NoError(t, err)
		assert.Equal(t, []string{"notes"}, got)
	})

	t.Run("deduplicates valid module args while preserving first occurrence", func(t *testing.T) {
		got, err := parseImportModules("/import notes filters NOTES rules filters", backupData)
		require.NoError(t, err)
		assert.Equal(t, []string{"notes", "filters", "rules"}, got)
	})

	t.Run("all invalid returns error", func(t *testing.T) {
		got, err := parseImportModules("/import foo bar", backupData)
		require.ErrorIs(t, err, errNoValidModule)
		assert.Nil(t, got)
	})
}

func TestParseModuleArgs(t *testing.T) {
	t.Parallel()

	valid := func(module string) bool {
		return module == "notes" || module == "filters"
	}

	got, err := parseModuleArgs([]string{"NOTES", "invalid", "filters", "notes", ""}, valid)
	require.NoError(t, err)
	assert.Equal(t, []string{"notes", "filters"}, got)

	_, err = parseModuleArgs([]string{"invalid", ""}, valid)
	require.ErrorIs(t, err, errNoValidModule)
}

func TestDownloadBackupFileRejectsInvalidDocumentBeforeNetwork(t *testing.T) {
	t.Parallel()

	tr := testTranslator(t)

	t.Run("non-json file", func(t *testing.T) {
		data, msg := downloadBackupFile(nil, &gotgbot.Document{FileName: "backup.txt"}, tr)
		assert.Nil(t, data)
		assert.Equal(t, "Invalid backup file", msg)
	})

	t.Run("file larger than ten megabytes", func(t *testing.T) {
		data, msg := downloadBackupFile(nil, &gotgbot.Document{
			FileName: "backup.json",
			FileSize: 10*1024*1024 + 1,
		}, tr)
		assert.Nil(t, data)
		assert.Equal(t, "File is too large", msg)
	})
}

func TestDownloadBackupFileReportsGotgbotGetFileFailure(t *testing.T) {
	tr := testTranslator(t)
	client := newModuleBotClient()
	client.errors["getFile"] = fmt.Errorf("telegram getFile failed")
	bot := newModuleTestBot(client)

	data, msg := downloadBackupFile(bot, &gotgbot.Document{
		FileName: "backup.json",
		FileId:   "backup-file-id",
	}, tr)

	assert.Nil(t, data)
	assert.NotEmpty(t, msg)
	assert.Len(t, client.callsFor("getFile"), 1)
}

func TestDownloadBackupFileDownloadsGetFilePath(t *testing.T) {
	tr := testTranslator(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/file/bot999:test/backups/chat.json", r.URL.Path)
		_, _ = w.Write([]byte(`{"version":"1.0.0"}`))
	}))
	t.Cleanup(server.Close)

	oldBaseURL := backupDownloadBaseURL
	oldHTTPClient := backupDownloadHTTPClient
	backupDownloadBaseURL = server.URL + "/file/bot"
	backupDownloadHTTPClient = server.Client()
	t.Cleanup(func() {
		backupDownloadBaseURL = oldBaseURL
		backupDownloadHTTPClient = oldHTTPClient
	})

	client := newModuleBotClient()
	client.responses["getFile"] = json.RawMessage(
		`{"file_id":"backup-file-id","file_path":"backups/chat.json"}`,
	)
	bot := newModuleTestBot(client)

	data, msg := downloadBackupFile(bot, &gotgbot.Document{
		FileName: "backup.json",
		FileId:   "backup-file-id",
	}, tr)

	assert.Equal(t, `{"version":"1.0.0"}`, string(data))
	assert.Empty(t, msg)
	assert.Len(t, client.callsFor("getFile"), 1)
}

func TestDownloadBackupFileLimitsResponseBody(t *testing.T) {
	tr := testTranslator(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("x"), maxBackupFileSize+1))
	}))
	t.Cleanup(server.Close)

	oldBaseURL := backupDownloadBaseURL
	oldHTTPClient := backupDownloadHTTPClient
	backupDownloadBaseURL = server.URL + "/file/bot"
	backupDownloadHTTPClient = server.Client()
	t.Cleanup(func() {
		backupDownloadBaseURL = oldBaseURL
		backupDownloadHTTPClient = oldHTTPClient
	})

	client := newModuleBotClient()
	client.responses["getFile"] = json.RawMessage(
		`{"file_id":"backup-file-id","file_path":"backups/chat.json"}`,
	)
	bot := newModuleTestBot(client)

	data, msg := downloadBackupFile(bot, &gotgbot.Document{
		FileName: "backup.json",
		FileId:   "backup-file-id",
	}, tr)

	assert.Nil(t, data)
	assert.Equal(t, "File is too large", msg)
}

func TestDownloadBackupFileReportsHTTPStatusFailure(t *testing.T) {
	tr := testTranslator(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "upstream failed", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	oldBaseURL := backupDownloadBaseURL
	oldHTTPClient := backupDownloadHTTPClient
	backupDownloadBaseURL = server.URL + "/file/bot"
	backupDownloadHTTPClient = server.Client()
	t.Cleanup(func() {
		backupDownloadBaseURL = oldBaseURL
		backupDownloadHTTPClient = oldHTTPClient
	})

	client := newModuleBotClient()
	client.responses["getFile"] = json.RawMessage(
		`{"file_id":"backup-file-id","file_path":"backups/chat.json"}`,
	)
	bot := newModuleTestBot(client)

	data, msg := downloadBackupFile(bot, &gotgbot.Document{
		FileName: "backup.json",
		FileId:   "backup-file-id",
	}, tr)

	assert.Nil(t, data)
	assert.Equal(t, "Download failed", msg)
	assert.Len(t, client.callsFor("getFile"), 1)
}

func TestImportHandlerStoresDownloadedBackupForConfirmation(t *testing.T) {
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	backup := backup.NewBackupFormat(chat.Id, chat.Title, owner.Id, []string{"rules", "notes"})
	backup.Data["rules"] = map[string]interface{}{
		"settings": map[string]interface{}{"rules": "imported rules"},
	}
	backup.Data["notes"] = map[string]interface{}{
		"notes": []interface{}{
			map[string]interface{}{"keyword": "welcome", "reply": "hello"},
		},
	}
	backupData, err := backup.ToJSON()
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/file/bot999:test/backups/chat.json", r.URL.Path)
		_, _ = w.Write(backupData)
	}))
	t.Cleanup(server.Close)

	oldBaseURL := backupDownloadBaseURL
	oldHTTPClient := backupDownloadHTTPClient
	backupDownloadBaseURL = server.URL + "/file/bot"
	backupDownloadHTTPClient = server.Client()
	t.Cleanup(func() {
		backupDownloadBaseURL = oldBaseURL
		backupDownloadHTTPClient = oldHTTPClient
		dropPendingImport(chat.Id)
	})

	client := newModuleBotClient()
	client.responses["getFile"] = json.RawMessage(
		`{"file_id":"backup-file-id","file_path":"backups/chat.json"}`,
	)
	bot := newModuleTestBot(client)
	ctx := newModuleMessageContext(bot, chat, owner, "/import rules invalid rules")
	ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 333,
		Date:      1,
		Chat:      chat,
		Document: &gotgbot.Document{
			FileId:   "backup-file-id",
			FileName: "backup.json",
		},
	}

	err = backupModule.importHandler(bot, ctx)

	require.Equal(t, ext.EndGroups, err)
	gotBackup, gotModules, ok := peekPendingImport(chat.Id)
	require.True(t, ok)
	assert.Equal(t, []string{"rules"}, gotModules)
	assert.Equal(t, backup.Version, gotBackup.Version)
	assert.Contains(t, gotBackup.Data, "notes")
	assert.Len(t, client.callsFor("getFile"), 1)
	assert.Len(t, client.callsFor("sendMessage"), 1)
}

func TestImportHandlerClearsPendingWhenConfirmationFails(t *testing.T) {
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	bkp := backup.NewBackupFormat(chat.Id, chat.Title, owner.Id, []string{"rules"})
	bkp.Data["rules"] = map[string]interface{}{"settings": map[string]interface{}{"rules": "test"}}
	backupData, err := bkp.ToJSON()
	require.NoError(t, err)

	oldBaseURL := backupDownloadBaseURL
	oldHTTPClient := backupDownloadHTTPClient
	backupDownloadBaseURL = "https://example.invalid/file/bot"
	backupDownloadHTTPClient = &http.Client{Transport: backupRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(backupData)),
			Header:     make(http.Header),
		}, nil
	})}
	t.Cleanup(func() {
		backupDownloadBaseURL = oldBaseURL
		backupDownloadHTTPClient = oldHTTPClient
		dropPendingImport(chat.Id)
	})

	client := newModuleBotClient()
	client.responses["getFile"] = json.RawMessage(
		`{"file_id":"backup-file-id","file_path":"backups/chat.json"}`,
	)
	client.errors["sendMessage"] = errors.New("confirmation send failed")
	bot := newModuleTestBot(client)
	ctx := newModuleMessageContext(bot, chat, owner, "/import")
	ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 333,
		Date:      1,
		Chat:      chat,
		Document: &gotgbot.Document{
			FileId:   "backup-file-id",
			FileName: "backup.json",
		},
	}

	err = backupModule.importHandler(bot, ctx)

	require.Equal(t, ext.EndGroups, err)
	_, _, ok := peekPendingImport(chat.Id)
	assert.False(t, ok)
	assert.Len(t, client.callsFor("sendMessage"), 1)
}

func TestBuildExportCaption(t *testing.T) {
	t.Parallel()

	tr := testTranslator(t)
	backup := backup.NewBackupFormat(12345, "Test Chat", 67890, []string{"notes", "filters"})
	backup.Data["notes"] = map[string]interface{}{"test": "data"}
	backup.Data["filters"] = map[string]interface{}{"test": "data"}
	backup.ExportedAt = backup.ExportedAt.UTC()

	caption := buildExportCaption(tr, backup)
	assert.Contains(t, caption, "Test Chat")
	assert.Contains(t, caption, "2")
	assert.Contains(t, caption, formatChatTime(backup.ExportedAt, chatClockOf(backup.ChatID), "2006-01-02 15:04:05"))
	assert.Contains(t, caption, "notes")
	assert.Contains(t, caption, "filters")
}

func TestBuildImportKeyboard(t *testing.T) {
	t.Parallel()

	tr := testTranslator(t)
	chatID := int64(-9223372036854775807 - 1)
	token := "0123456789abcdef"
	keyboard := buildImportKeyboard(tr, chatID, token)

	require.Len(t, keyboard.InlineKeyboard, 1)
	require.Len(t, keyboard.InlineKeyboard[0], 2)

	confirmBtn := keyboard.InlineKeyboard[0][0]
	assert.Equal(t, "Confirm Import", confirmBtn.Text)
	assert.NotEmpty(t, confirmBtn.CallbackData)

	cancelBtn := keyboard.InlineKeyboard[0][1]
	assert.Equal(t, "Cancel Import", cancelBtn.Text)
	assert.NotEmpty(t, cancelBtn.CallbackData)

	decodedConfirm, ok := decodeCallbackData(confirmBtn.CallbackData, "backup")
	require.True(t, ok)
	action, _ := decodedConfirm.Field("a")
	assert.Equal(t, "ci", action)
	chatIDStr, _ := decodedConfirm.Field("c")
	assert.Equal(t, "-9223372036854775808", chatIDStr)
	gotToken, _ := decodedConfirm.Field("t")
	assert.Equal(t, token, gotToken)
	assert.LessOrEqual(t, len(confirmBtn.CallbackData), 64)

	decodedCancel, ok := decodeCallbackData(cancelBtn.CallbackData, "backup")
	require.True(t, ok)
	action, _ = decodedCancel.Field("a")
	assert.Equal(t, "xi", action)
	gotToken, _ = decodedCancel.Field("t")
	assert.Equal(t, token, gotToken)
	assert.LessOrEqual(t, len(cancelBtn.CallbackData), 64)
}

func TestBuildResetKeyboard(t *testing.T) {
	t.Parallel()

	tr := testTranslator(t)
	chatID := int64(-9223372036854775807 - 1)
	token := "0123456789abcdef"
	keyboard := buildResetKeyboard(tr, chatID, token)

	require.Len(t, keyboard.InlineKeyboard, 2)
	require.Len(t, keyboard.InlineKeyboard[0], 1)
	require.Len(t, keyboard.InlineKeyboard[1], 1)

	confirmBtn := keyboard.InlineKeyboard[0][0]
	assert.Equal(t, "Confirm Reset", confirmBtn.Text)
	assert.NotEmpty(t, confirmBtn.CallbackData)

	cancelBtn := keyboard.InlineKeyboard[1][0]
	assert.Equal(t, "Cancel Reset", cancelBtn.Text)
	assert.NotEmpty(t, cancelBtn.CallbackData)

	decodedConfirm, ok := decodeCallbackData(confirmBtn.CallbackData, "backup")
	require.True(t, ok)
	action, _ := decodedConfirm.Field("a")
	assert.Equal(t, "cr", action)
	chatIDStr, _ := decodedConfirm.Field("c")
	assert.Equal(t, "-9223372036854775808", chatIDStr)
	gotToken, _ := decodedConfirm.Field("t")
	assert.Equal(t, token, gotToken)
	assert.LessOrEqual(t, len(confirmBtn.CallbackData), 64)

	decodedCancel, ok := decodeCallbackData(cancelBtn.CallbackData, "backup")
	require.True(t, ok)
	action, _ = decodedCancel.Field("a")
	assert.Equal(t, "xr", action)
	gotToken, _ = decodedCancel.Field("t")
	assert.Equal(t, token, gotToken)
	assert.LessOrEqual(t, len(cancelBtn.CallbackData), 64)
}

// peekPendingImport / peekPendingReset / dropPendingImport / dropPendingReset
// are white-box accessors over the in-memory pending backup state, used by
// these tests to assert handler side effects; production handlers manage this
// state exclusively through store/consume/discard/cleanup paths.
func peekPendingImport(chatID int64) (*backup.BackupFormat, []string, bool) {
	pendingMu.Lock()
	defer pendingMu.Unlock()
	pending, ok := pendingImports[chatID]
	if !ok || !time.Now().Before(pending.expiresAt) {
		delete(pendingImports, chatID)
		return nil, nil, false
	}
	return pending.backup, pending.modules, true
}

func dropPendingImport(chatID int64) {
	pendingMu.Lock()
	defer pendingMu.Unlock()
	delete(pendingImports, chatID)
}

func peekPendingReset(chatID int64) ([]string, bool) {
	pendingMu.Lock()
	defer pendingMu.Unlock()
	pending, ok := pendingResets[chatID]
	if !ok || !time.Now().Before(pending.expiresAt) {
		delete(pendingResets, chatID)
		return nil, false
	}
	return pending.modules, true
}

func dropPendingReset(chatID int64) {
	pendingMu.Lock()
	defer pendingMu.Unlock()
	delete(pendingResets, chatID)
}

func TestPendingImportsMaps(t *testing.T) {
	t.Run("pending imports maps exist", func(t *testing.T) {
		assert.NotNil(t, pendingImports)
		assert.NotNil(t, pendingResets)
	})
}

func TestPendingImportRejectsStaleTokenAndConsumesOnce(t *testing.T) {
	chatID := uniqueModuleChatID()
	t.Cleanup(func() { dropPendingImport(chatID) })

	oldToken, err := storePendingImport(chatID, &backup.BackupFormat{}, []string{"rules"})
	require.NoError(t, err)
	current := &backup.BackupFormat{}
	currentToken, err := storePendingImport(chatID, current, []string{"notes"})
	require.NoError(t, err)
	require.NotEqual(t, oldToken, currentToken)

	_, _, ok := consumePendingImport(chatID, oldToken)
	assert.False(t, ok)

	var consumed atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, modules, ok := consumePendingImport(chatID, currentToken)
			if ok {
				assert.Same(t, current, got)
				assert.Equal(t, []string{"notes"}, modules)
				consumed.Add(1)
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, int32(1), consumed.Load())

	expiredToken, err := storePendingImport(chatID, current, []string{"notes"})
	require.NoError(t, err)
	pendingMu.Lock()
	expired := pendingImports[chatID]
	expired.expiresAt = time.Now().Add(-time.Second)
	pendingImports[chatID] = expired
	pendingMu.Unlock()
	_, _, ok = consumePendingImport(chatID, expiredToken)
	assert.False(t, ok)
}

func TestPendingResetRejectsStaleAndExpiredTokens(t *testing.T) {
	chatID := uniqueModuleChatID()
	t.Cleanup(func() { dropPendingReset(chatID) })

	oldToken, err := storePendingReset(chatID, []string{"rules"})
	require.NoError(t, err)
	currentToken, err := storePendingReset(chatID, []string{"notes"})
	require.NoError(t, err)
	require.NotEqual(t, oldToken, currentToken)

	assert.False(t, discardPendingReset(chatID, oldToken))
	modules, ok := peekPendingReset(chatID)
	require.True(t, ok)
	assert.Equal(t, []string{"notes"}, modules)

	pendingMu.Lock()
	pending := pendingResets[chatID]
	pending.expiresAt = time.Now().Add(-time.Second)
	pendingResets[chatID] = pending
	pendingMu.Unlock()

	_, ok = consumePendingReset(chatID, currentToken)
	assert.False(t, ok)
	_, ok = peekPendingReset(chatID)
	assert.False(t, ok)
}

func TestImportHandlerRequiresReplyDocument(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleMessageContext(bot, chat, owner, "/import")

	err := backupModule.importHandler(bot, ctx)
	assert.Equal(t, ext.EndGroups, err)
	assert.Len(t, client.callsFor("sendMessage"), 1)
}

func TestImportHandlerRejectsInvalidReplyDocument(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleMessageContext(bot, chat, owner, "/import")
	ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 333,
		Date:      1,
		Chat:      chat,
		Document: &gotgbot.Document{
			FileId:   "not-json",
			FileName: "backup.txt",
		},
	}

	err := backupModule.importHandler(bot, ctx)
	assert.Equal(t, ext.EndGroups, err)
	assert.Len(t, client.callsFor("sendMessage"), 1)
	assert.Empty(t, client.callsFor("getFile"))
}

func TestExportHandlerSendsRequestedModuleBackupDocument(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	require.NoError(t, chats.EnsureChatInDb(chat.Id, chat.Title))
	require.NoError(t, notes.AddNote(chat.Id, "welcome", "hello", "", nil, db.TEXT, false, false, false, true, false, false))

	ctx := newModuleMessageContext(bot, chat, owner, "/export notes invalid notes")
	err := backupModule.exportHandler(bot, ctx)
	assert.Equal(t, ext.EndGroups, err)
	assert.Len(t, client.callsFor("sendDocument"), 1)
	assert.Empty(t, client.callsFor("sendMessage"))
}

func TestExportHandlerFallsBackToTextWhenDocumentSendFails(t *testing.T) {
	client := newModuleBotClient()
	client.errors["sendDocument"] = fmt.Errorf("document upload failed")
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	require.NoError(t, chats.EnsureChatInDb(chat.Id, chat.Title))
	require.NoError(t, notes.AddNote(chat.Id, "fallback", "hello", "", nil, db.TEXT, false, false, false, true, false, false))

	ctx := newModuleMessageContext(bot, chat, owner, "/export notes")
	err := backupModule.exportHandler(bot, ctx)
	assert.Equal(t, ext.EndGroups, err)
	assert.Len(t, client.callsFor("sendDocument"), 1)
	assert.Len(t, client.callsFor("sendMessage"), 1)
}

func TestValidateImportRequestRejectsNonOwner(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	ctx := newModuleMessageContext(bot, chat, member, "/import")

	msg, gotChat, user, tr, ok := validateImportRequest(bot, ctx)
	assert.False(t, ok)
	assert.Nil(t, msg)
	assert.Nil(t, gotChat)
	assert.Nil(t, user)
	assert.Nil(t, tr)
	assert.NotEmpty(t, client.callsFor("sendMessage"))
}

func TestResetHandlerStoresPendingModulesAndRepliesWithConfirmation(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleMessageContext(bot, chat, owner, "/reset rules notes invalid rules")
	t.Cleanup(func() {
		dropPendingReset(chat.Id)
	})

	err := backupModule.resetHandler(bot, ctx)
	assert.Equal(t, ext.EndGroups, err)
	gotModules, ok := peekPendingReset(chat.Id)
	assert.True(t, ok)
	assert.Equal(t, []string{"rules", "notes"}, gotModules)
	assert.Len(t, client.callsFor("sendMessage"), 1)
}

func TestResetHandlerClearsPendingWhenConfirmationFails(t *testing.T) {
	client := newModuleBotClient()
	client.errors["sendMessage"] = errors.New("confirmation send failed")
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleMessageContext(bot, chat, owner, "/reset rules")
	t.Cleanup(func() { dropPendingReset(chat.Id) })

	err := backupModule.resetHandler(bot, ctx)

	require.Equal(t, ext.EndGroups, err)
	_, ok := peekPendingReset(chat.Id)
	assert.False(t, ok)
	assert.Len(t, client.callsFor("sendMessage"), 1)
}

func TestBackupCallbackHandlerConfirmsPendingImport(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	require.NoError(t, chats.EnsureChatInDb(chat.Id, chat.Title))

	backup := backup.NewBackupFormat(chat.Id, chat.Title, owner.Id, []string{"rules"})
	backup.Data["rules"] = map[string]interface{}{
		"settings": map[string]interface{}{
			"chat_id":   chat.Id,
			"rules":     "imported rules",
			"rules_btn": "Read rules",
			"private":   true,
		},
	}
	token, err := storePendingImport(chat.Id, backup, []string{"rules"})
	require.NoError(t, err)
	t.Cleanup(func() {
		dropPendingImport(chat.Id)
	})

	callback := encodeCallbackData(
		"backup",
		map[string]string{"a": backupActionConfirmImport, "c": strconv.FormatInt(chat.Id, 10), "t": token},
	)
	ctx := newModuleCallbackContext(bot, chat, owner, callback)
	err = backupModule.backupCallbackHandler(bot, ctx)
	assert.Equal(t, ext.EndGroups, err)
	_, _, ok := peekPendingImport(chat.Id)
	assert.False(t, ok)
	assert.Equal(t, "imported rules", rules.GetChatRulesInfo(chat.Id).Rules)
	assert.Len(t, client.callsFor("sendMessage"), 1)
}

func TestBackupCallbackHandlerConfirmsPendingReset(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	require.NoError(t, chats.EnsureChatInDb(chat.Id, chat.Title))
	rules.SetChatRules(chat.Id, "rules before reset")
	token, err := storePendingReset(chat.Id, []string{"rules"})
	require.NoError(t, err)
	t.Cleanup(func() {
		dropPendingReset(chat.Id)
	})

	callback := encodeCallbackData(
		"backup",
		map[string]string{"a": backupActionConfirmReset, "c": strconv.FormatInt(chat.Id, 10), "t": token},
	)
	ctx := newModuleCallbackContext(bot, chat, owner, callback)
	err = backupModule.backupCallbackHandler(bot, ctx)
	assert.Equal(t, ext.EndGroups, err)
	_, ok := peekPendingReset(chat.Id)
	assert.False(t, ok)
	assert.Empty(t, rules.GetChatRulesInfo(chat.Id).Rules)
	assert.Len(t, client.callsFor("sendMessage"), 1)
}

func TestBackupCallbackHandlerIgnoresWrongChatConfirmation(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	token, err := storePendingReset(chat.Id, []string{"rules"})
	require.NoError(t, err)
	t.Cleanup(func() {
		dropPendingReset(chat.Id)
	})

	callback := encodeCallbackData(
		"backup",
		map[string]string{"a": backupActionConfirmReset, "c": strconv.FormatInt(chat.Id+1, 10), "t": token},
	)
	ctx := newModuleCallbackContext(bot, chat, owner, callback)
	err = backupModule.backupCallbackHandler(bot, ctx)
	assert.Equal(t, ext.EndGroups, err)
	_, ok := peekPendingReset(chat.Id)
	assert.True(t, ok)
	assert.Empty(t, client.callsFor("sendMessage"))
	assert.Len(t, client.callsFor("answerCallbackQuery"), 1)
}

func TestBackupCallbackHandlerRejectsInvalidCallbackData(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	for _, data := range []string{
		"not-a-backup-callback",
		encodeCallbackData("backup", map[string]string{
			"a": "crafted",
			"c": strconv.FormatInt(chat.Id, 10),
			"t": "token",
		}),
		encodeCallbackData("backup", map[string]string{
			"a": backupActionConfirmReset,
			"c": "not-a-chat",
			"t": "token",
		}),
		encodeCallbackData("backup", map[string]string{
			"a": backupActionConfirmReset,
			"c": strconv.FormatInt(chat.Id, 10),
		}),
	} {
		ctx := newModuleCallbackContext(bot, chat, owner, data)
		assert.Equal(t, ext.EndGroups, backupModule.backupCallbackHandler(bot, ctx))
	}
	assert.Len(t, client.callsFor("answerCallbackQuery"), 4)
}

func TestBackupConfirmHandlersReportExpiredState(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := &gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	tr := testTranslator(t)
	ctx := &ext.Context{}
	t.Cleanup(func() {
		dropPendingImport(chat.Id)
		dropPendingReset(chat.Id)
	})

	err := backupModule.handleConfirmImport(bot, ctx, tr, chat, "")
	assert.Equal(t, ext.EndGroups, err)
	assert.Len(t, client.callsFor("sendMessage"), 1)

	err = backupModule.handleConfirmReset(bot, ctx, tr, chat, "")
	assert.Equal(t, ext.EndGroups, err)
	assert.Len(t, client.callsFor("sendMessage"), 2)
}

func TestBackupCallbackCancelImportAndResetCleanup(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	importToken, err := storePendingImport(chat.Id, backup.NewBackupFormat(chat.Id, chat.Title, owner.Id, []string{"rules"}), []string{"rules"})
	require.NoError(t, err)
	cancelImport := encodeCallbackData(
		"backup",
		map[string]string{"a": backupActionCancelImport, "c": strconv.FormatInt(chat.Id, 10), "t": importToken},
	)
	importCtx := newModuleCallbackContext(bot, chat, owner, cancelImport)
	err = backupModule.handleCancelImport(bot, importCtx, testTranslator(t), importCtx.CallbackQuery, importToken)
	assert.Equal(t, ext.EndGroups, err)
	_, _, ok := peekPendingImport(chat.Id)
	assert.False(t, ok)

	resetToken, err := storePendingReset(chat.Id, []string{"rules"})
	require.NoError(t, err)
	cancelReset := encodeCallbackData(
		"backup",
		map[string]string{"a": backupActionCancelReset, "c": strconv.FormatInt(chat.Id, 10), "t": resetToken},
	)
	resetCtx := newModuleCallbackContext(bot, chat, owner, cancelReset)
	err = backupModule.handleCancelReset(bot, resetCtx, testTranslator(t), resetCtx.CallbackQuery, resetToken)
	assert.Equal(t, ext.EndGroups, err)
	_, ok = peekPendingReset(chat.Id)
	assert.False(t, ok)

	assert.Len(t, client.callsFor("answerCallbackQuery"), 2)
}

func cleanupSharedBackupTestChat(t *testing.T, chatID int64) {
	t.Helper()
	t.Cleanup(func() {
		dropPendingImport(chatID)
		dropPendingReset(chatID)
		ratelimit.GetBackupRateLimiter().ReleaseExport(chatID)
		_ = notes.RemoveAllNotes(chatID)
		if db.DB != nil {
			db.DB.Where("chat_id = ?", chatID).Delete(&models.RulesSettings{})
			db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatEventMessage{})
			db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatEventMessageOptions{})
			db.DB.Where("chat_id = ?", chatID).Delete(&db.Chat{})
		}
		dbcache.DeleteCache(eventmsg.CacheKey(chatID))
		dbcache.DeleteCache(fmt.Sprintf("rules_%d", chatID))
	})
}

func TestSharedBackupApplyHelpers(t *testing.T) {
	withRealLocales(t)
	tr := i18n.MustNewTranslator("en")
	bg := context.Background()

	t.Run("applyBackupExport", func(t *testing.T) {
		chatID := uniqueModuleChatID()
		userID := int64(503001)
		require.NoError(t, chats.EnsureChatInDb(chatID, "Shared Export Group"))
		cleanupSharedBackupTestChat(t, chatID)
		require.NoError(t, notes.AddNote(chatID, "welcome", "hello", "", nil, db.TEXT, false, false, false, true, false, false))
		require.NoError(t, eventmsg.SetText(bg, chatID, eventmsg.EventBan, eventmsg.DestGroup, "Shared export custom ban", userID))

		client := newModuleBotClient()
		bot := newModuleTestBot(client)

		// 1. Success delivery goes to userID (never chatID) and carries custom messages.
		res := applyBackupExport(bot, tr, chatID, "Shared Export Group", userID, []string{"notes", backup.BackupModuleMessages})
		require.Equal(t, backupExportSent, res.Outcome)
		assert.Equal(t, 2, res.ModuleCount)
		require.NotNil(t, res.Backup)

		docs := client.callsFor("sendDocument")
		require.Len(t, docs, 1)
		assert.Equal(t, userID, staffParamInt(docs[0].Params, "chat_id"))
		assert.NotEqual(t, chatID, staffParamInt(docs[0].Params, "chat_id"))
		assert.Equal(t, "HTML", fmt.Sprint(docs[0].Params["parse_mode"]))

		reader, ok := docs[0].Params["document"].(*gotgbot.FileReader)
		require.True(t, ok)
		rawJSON, err := io.ReadAll(reader.Data)
		require.NoError(t, err)
		parsed, err := backup.BackupFormatFromJSON(rawJSON)
		require.NoError(t, err)
		assert.Equal(t, []string{"notes", backup.BackupModuleMessages}, parsed.Modules)
		assert.Contains(t, string(rawJSON), "Shared export custom ban")

		// 2. Immediate second export hits rate limit with 0 additional document sends.
		limited := applyBackupExport(bot, tr, chatID, "Shared Export Group", userID, []string{"notes"})
		assert.Equal(t, backupExportRateLimited, limited.Outcome)
		assert.Greater(t, limited.Cooldown, time.Duration(0))
		assert.Len(t, client.callsFor("sendDocument"), 1)

		// 3. 403 Forbidden returns backupExportStartBot and releases the cooldown for immediate retry.
		ratelimit.GetBackupRateLimiter().ReleaseExport(chatID)
		client.errors["sendDocument"] = &gotgbot.TelegramError{
			Code:        403,
			Description: "Forbidden: bot can't initiate conversation with a user",
		}
		startBotRes := applyBackupExport(bot, tr, chatID, "Shared Export Group", userID, []string{"notes"})
		assert.Equal(t, backupExportStartBot, startBotRes.Outcome)

		delete(client.errors, "sendDocument")
		retryRes := applyBackupExport(bot, tr, chatID, "Shared Export Group", userID, []string{"notes"})
		assert.Equal(t, backupExportSent, retryRes.Outcome)
		assert.Equal(t, 1, retryRes.ModuleCount)

		// 4. Generic sendDocument error returns backupExportSendFailed with module count preserved.
		ratelimit.GetBackupRateLimiter().ReleaseExport(chatID)
		client.errors["sendDocument"] = errors.New("upload timeout")
		sendErrRes := applyBackupExport(bot, tr, chatID, "Shared Export Group", userID, []string{"notes"})
		assert.Equal(t, backupExportSendFailed, sendErrRes.Outcome)
		assert.Equal(t, 1, sendErrRes.ModuleCount)
		assert.NotNil(t, sendErrRes.Backup)
		delete(client.errors, "sendDocument")

		// 5. Empty exported module data returns backupExportNoModules.
		origExport := backupExportChatData
		t.Cleanup(func() { backupExportChatData = origExport })

		ratelimit.GetBackupRateLimiter().ReleaseExport(chatID)
		backupExportChatData = func(cID int64, title string, by int64, mods []string) (*backup.BackupFormat, error) {
			return backup.NewBackupFormat(cID, title, by, mods), nil
		}
		emptyRes := applyBackupExport(bot, tr, chatID, "Shared Export Group", userID, []string{"notes"})
		assert.Equal(t, backupExportNoModules, emptyRes.Outcome)

		// 6. Export DB error returns backupExportFailed.
		ratelimit.GetBackupRateLimiter().ReleaseExport(chatID)
		backupExportChatData = func(int64, string, int64, []string) (*backup.BackupFormat, error) {
			return nil, errors.New("simulated export failure")
		}
		failedRes := applyBackupExport(bot, tr, chatID, "Shared Export Group", userID, []string{"notes"})
		assert.Equal(t, backupExportFailed, failedRes.Outcome)
	})

	t.Run("downloadAndParseBackupFile", func(t *testing.T) {
		validBkp := backup.NewBackupFormat(-100300010, "Download Group", 777000, []string{"rules", backup.BackupModuleMessages})
		validBkp.Data["rules"] = map[string]interface{}{
			"settings": map[string]interface{}{"rules": "Be nice"},
		}
		validBkp.Data[backup.BackupModuleMessages] = backup.MessagesBackup{
			Messages: []models.ChatEventMessage{
				{Event: eventmsg.EventBan, Dest: eventmsg.DestGroup, Text: "Custom ban"},
			},
		}
		validBytes, err := validBkp.ToJSON()
		require.NoError(t, err)

		incompatBkp := backup.NewBackupFormat(-100300010, "Download Group", 777000, []string{"rules"})
		incompatBkp.Version = "9.9"
		incompatBkp.Data["rules"] = map[string]interface{}{
			"settings": map[string]interface{}{"rules": "Future version"},
		}
		incompatBytes, err := incompatBkp.ToJSON()
		require.NoError(t, err)

		var payload []byte
		oldBaseURL := backupDownloadBaseURL
		oldHTTPClient := backupDownloadHTTPClient
		backupDownloadBaseURL = "https://example.invalid/file/bot"
		backupDownloadHTTPClient = &http.Client{
			Transport: backupRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				assert.Equal(t, "/file/bot999:test/backups/shared.json", req.URL.Path)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader(payload)),
					Header:     make(http.Header),
				}, nil
			}),
		}
		t.Cleanup(func() {
			backupDownloadBaseURL = oldBaseURL
			backupDownloadHTTPClient = oldHTTPClient
		})

		client := newModuleBotClient()
		client.responses["getFile"] = json.RawMessage(`{"file_id":"shared-json-id","file_path":"backups/shared.json"}`)
		bot := newModuleTestBot(client)

		wantInvalid, _ := tr.GetString("backup_import_invalid_file")
		wantMismatch, _ := tr.GetString("backup_import_version_mismatch")

		// Non-JSON extension rejected before getFile.
		got, errText := downloadAndParseBackupFile(bot, &gotgbot.Document{FileId: "txt-id", FileName: "backup.txt"}, tr)
		assert.Nil(t, got)
		assert.Equal(t, wantInvalid, errText)
		assert.Empty(t, client.callsFor("getFile"))

		// Nil document rejected before getFile.
		got, errText = downloadAndParseBackupFile(bot, nil, tr)
		assert.Nil(t, got)
		assert.Equal(t, wantInvalid, errText)
		assert.Empty(t, client.callsFor("getFile"))

		// Valid JSON backup succeeds.
		payload = validBytes
		got, errText = downloadAndParseBackupFile(bot, &gotgbot.Document{FileId: "shared-json-id", FileName: "backup.json"}, tr)
		require.NotNil(t, got)
		assert.Empty(t, errText)
		assert.Equal(t, []string{"rules", backup.BackupModuleMessages}, got.Modules)

		// Corrupted JSON rejected with backup_import_invalid_file.
		payload = []byte(`{not-valid-json`)
		got, errText = downloadAndParseBackupFile(bot, &gotgbot.Document{FileId: "shared-json-id", FileName: "backup.json"}, tr)
		assert.Nil(t, got)
		assert.Equal(t, wantInvalid, errText)

		// Incompatible version rejected with backup_import_version_mismatch.
		payload = incompatBytes
		got, errText = downloadAndParseBackupFile(bot, &gotgbot.Document{FileId: "shared-json-id", FileName: "backup.json"}, tr)
		assert.Nil(t, got)
		assert.Equal(t, wantMismatch, errText)
	})

	t.Run("applyBackupImport_and_formatBackupImportSuccess", func(t *testing.T) {
		chatID := uniqueModuleChatID()
		require.NoError(t, chats.EnsureChatInDb(chatID, "Shared Import Group"))
		cleanupSharedBackupTestChat(t, chatID)

		src := backup.NewBackupFormat(chatID, "Shared Import Group", 777000, []string{"rules", backup.BackupModuleMessages})
		src.Data["rules"] = map[string]interface{}{
			"settings": map[string]interface{}{"rules": "Imported via shared helper"},
		}
		src.Data[backup.BackupModuleMessages] = backup.MessagesBackup{
			Messages: []models.ChatEventMessage{
				{
					Event:   eventmsg.EventBan,
					Dest:    eventmsg.DestGroup,
					Text:    "Valid shared ban",
					Buttons: models.ButtonArray{{Name: "Help", Url: "https://example.org/help"}},
				},
				{
					Event:   eventmsg.EventWarn,
					Dest:    eventmsg.DestGroup,
					Text:    "Unsafe warn button",
					Buttons: models.ButtonArray{{Name: "XSS", Url: "javascript:alert(1)"}},
				},
			},
			Options: &models.ChatEventMessageOptions{
				PMDetails: true,
				PlainTone: true,
			},
		}
		raw, err := src.ToJSON()
		require.NoError(t, err)
		decoded, err := backup.BackupFormatFromJSON(raw)
		require.NoError(t, err)

		modules := []string{"rules", backup.BackupModuleMessages}
		report, err := applyBackupImport(chatID, decoded, modules)
		require.NoError(t, err)
		assert.Equal(t, 1, report.Skipped)
		assert.False(t, report.LogChannelSkipped)

		assert.Equal(t, "Imported via shared helper", rules.GetChatRulesInfo(chatID).Rules)
		gotMsgs, err := eventmsg.GetFresh(bg, chatID)
		require.NoError(t, err)
		banMsg, ok := gotMsgs.Lookup(eventmsg.EventBan, eventmsg.DestGroup)
		require.True(t, ok)
		assert.Equal(t, "Valid shared ban", banMsg.Text)
		_, warnExists := gotMsgs.Lookup(eventmsg.EventWarn, eventmsg.DestGroup)
		assert.False(t, warnExists, "invalid javascript: button message must be skipped")
		assert.True(t, gotMsgs.Options.PMDetails)
		assert.True(t, gotMsgs.Options.PlainTone)

		summary := formatBackupImportSuccess(tr, modules, report)
		wantSkippedHint, _ := tr.GetString("backup_import_skipped_entries", i18n.TranslationParams{"count": 1})
		wantLogHint, _ := tr.GetString("backup_import_log_skipped")
		assert.Contains(t, summary, "• rules")
		assert.Contains(t, summary, "• messages")
		assert.Contains(t, summary, wantSkippedHint)
		assert.NotContains(t, summary, wantLogHint)

		withLogHint := formatBackupImportSuccess(tr, modules, backup.ImportReport{Skipped: 1, LogChannelSkipped: true})
		assert.Contains(t, withLogHint, wantSkippedHint)
		assert.Contains(t, withLogHint, wantLogHint)
	})

	t.Run("applyBackupReset_and_formatBackupResetSuccess", func(t *testing.T) {
		chatID := uniqueModuleChatID()
		require.NoError(t, chats.EnsureChatInDb(chatID, "Shared Reset Group"))
		cleanupSharedBackupTestChat(t, chatID)

		rules.SetChatRules(chatID, "Rules to clear")
		require.NoError(t, eventmsg.SetText(bg, chatID, eventmsg.EventBan, eventmsg.DestGroup, "Custom ban to clear", 777000))
		require.NoError(t, eventmsg.SetPlainTone(bg, chatID, true, 777000))
		require.NoError(t, notes.AddNote(chatID, "keep", "untouched note", "", nil, db.TEXT, false, false, false, true, false, false))

		resetMods := []string{backup.BackupModuleMessages, "rules"}
		require.NoError(t, applyBackupReset(chatID, resetMods))

		assert.Empty(t, rules.GetChatRulesInfo(chatID).Rules)
		gotMsgs, err := eventmsg.GetFresh(bg, chatID)
		require.NoError(t, err)
		_, banExists := gotMsgs.Lookup(eventmsg.EventBan, eventmsg.DestGroup)
		assert.False(t, banExists)
		assert.False(t, gotMsgs.Options.PlainTone)
		assert.NotEmpty(t, notes.GetNotesList(chatID, true), "unselected modules must survive selective reset")

		summary := formatBackupResetSuccess(tr, resetMods)
		wantSummary, _ := tr.GetString("backup_reset_success", i18n.TranslationParams{
			"modules": "2",
			"list":    buildModuleList(resetMods),
		})
		assert.Equal(t, wantSummary, summary)
	})
}

func TestSettingsBackupRedisTokens(t *testing.T) {
	bg := context.Background()

	t.Run("import_token_put_peek_consume_and_replay", func(t *testing.T) {
		mr := phase28InProcessMiniredis(t)

		bkp := backup.NewBackupFormat(-1003000201, "Token Group", 777000, []string{"rules", backup.BackupModuleMessages})
		bkp.Data["rules"] = map[string]interface{}{
			"settings": map[string]interface{}{"rules": "Token rules"},
		}
		bkp.Data[backup.BackupModuleMessages] = backup.MessagesBackup{
			Messages: []models.ChatEventMessage{
				{
					Event:     eventmsg.EventBan,
					Dest:      eventmsg.DestGroup,
					Text:      "Redis pending custom ban",
					UpdatedBy: 9007199254740993, // > 2^53 to verify json.Number int64 precision
				},
			},
			Options: &models.ChatEventMessageOptions{PlainTone: true},
		}
		rawBkp, err := bkp.ToJSON()
		require.NoError(t, err)
		parsedBkp, err := backup.BackupFormatFromJSON(rawBkp)
		require.NoError(t, err)

		rec := settingsBackupImportRecord{
			UserID:  777000,
			ChatID:  -1003000201,
			Modules: []string{"rules", backup.BackupModuleMessages},
			Backup:  parsedBkp,
		}

		token, err := settingsBackupPutImportToken(bg, rec)
		require.NoError(t, err)
		assert.True(t, settingsCopyTokenValid(token), "token %q must be 16-char base64url", token)

		redisKey := settingsBackupImportTokenKey(token)
		require.True(t, mr.Exists(redisKey))
		ttl := mr.TTL(redisKey)
		assert.Greater(t, ttl, time.Duration(0))
		assert.LessOrEqual(t, ttl, settingsCardLifetime)

		// Peek returns exact binding and payload without deleting the key.
		peeked, found, err := settingsBackupPeekImportToken(bg, token)
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, rec.UserID, peeked.UserID)
		assert.Equal(t, rec.ChatID, peeked.ChatID)
		assert.Equal(t, rec.Modules, peeked.Modules)
		require.NotNil(t, peeked.Backup)
		assert.Contains(t, peeked.Backup.Data, "rules")
		assert.Contains(t, peeked.Backup.Data, backup.BackupModuleMessages)
		assert.True(t, mr.Exists(redisKey), "peek must not delete the pending token")

		// Wrong-user / wrong-chat binding check via peek leaves the real owner's token untouched.
		assert.NotEqual(t, int64(999999), peeked.UserID)
		assert.NotEqual(t, int64(-1009999999), peeked.ChatID)
		assert.True(t, mr.Exists(redisKey))

		// Malformed tokens fail closed with zero key mutation.
		for _, bad := range []string{"", "short", "bad/token!123456"} {
			_, badFound, badErr := settingsBackupPeekImportToken(bg, bad)
			assert.NoError(t, badErr)
			assert.False(t, badFound)
			_, badFound, badErr = settingsBackupConsumeImportToken(bg, bad)
			assert.NoError(t, badErr)
			assert.False(t, badFound)
		}
		assert.True(t, mr.Exists(redisKey))

		// Atomic Lua GET+DEL across 8 concurrent goroutines: exactly 1 succeeds (T-30-06).
		const workers = 8
		start := make(chan struct{})
		var consumed atomic.Int32
		var winner settingsBackupImportRecord
		var winnerMu sync.Mutex
		var wg sync.WaitGroup
		for range workers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				got, ok, consumeErr := settingsBackupConsumeImportToken(bg, token)
				assert.NoError(t, consumeErr)
				if ok {
					consumed.Add(1)
					winnerMu.Lock()
					winner = got
					winnerMu.Unlock()
				}
			}()
		}
		close(start)
		wg.Wait()

		assert.Equal(t, int32(1), consumed.Load())
		assert.Equal(t, rec.UserID, winner.UserID)
		assert.Equal(t, rec.ChatID, winner.ChatID)
		assert.Equal(t, rec.Modules, winner.Modules)
		require.NotNil(t, winner.Backup)
		msgMap, ok := winner.Backup.Data[backup.BackupModuleMessages].(map[string]interface{})
		require.True(t, ok)
		msgSlice, ok := msgMap["messages"].([]interface{})
		require.True(t, ok)
		require.Len(t, msgSlice, 1)
		firstMsg, ok := msgSlice[0].(map[string]interface{})
		require.True(t, ok)
		updatedByNum, ok := firstMsg["updated_by"].(json.Number)
		require.True(t, ok, "expected json.Number for 64-bit integer precision")
		assert.Equal(t, "9007199254740993", updatedByNum.String())

		// Replay after consume returns found == false.
		_, found, err = settingsBackupConsumeImportToken(bg, token)
		require.NoError(t, err)
		assert.False(t, found)
		_, found, err = settingsBackupPeekImportToken(bg, token)
		require.NoError(t, err)
		assert.False(t, found)
		assert.False(t, mr.Exists(redisKey))
	})

	t.Run("reset_token_put_peek_consume_and_expiry", func(t *testing.T) {
		mr := phase28InProcessMiniredis(t)

		rec := settingsBackupResetRecord{
			UserID:  777000,
			ChatID:  -1003000202,
			Modules: []string{backup.BackupModuleMessages, "rules"},
		}

		// 1. Put, peek (without deleting), concurrent atomic consume, and replay refusal.
		token, err := settingsBackupPutResetToken(bg, rec)
		require.NoError(t, err)
		assert.True(t, settingsCopyTokenValid(token))

		redisKey := settingsBackupResetTokenKey(token)
		require.True(t, mr.Exists(redisKey))
		ttl := mr.TTL(redisKey)
		assert.Greater(t, ttl, time.Duration(0))
		assert.LessOrEqual(t, ttl, settingsCardLifetime)

		peeked, found, err := settingsBackupPeekResetToken(bg, token)
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, rec, peeked)
		assert.True(t, mr.Exists(redisKey))

		const workers = 8
		start := make(chan struct{})
		var consumed atomic.Int32
		var wg sync.WaitGroup
		for range workers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				got, ok, consumeErr := settingsBackupConsumeResetToken(bg, token)
				assert.NoError(t, consumeErr)
				if ok {
					assert.Equal(t, rec, got)
					consumed.Add(1)
				}
			}()
		}
		close(start)
		wg.Wait()
		assert.Equal(t, int32(1), consumed.Load())

		_, found, err = settingsBackupConsumeResetToken(bg, token)
		require.NoError(t, err)
		assert.False(t, found)

		// 2. Expiry via FastForward(settingsCardLifetime + time.Second).
		expToken, err := settingsBackupPutResetToken(bg, rec)
		require.NoError(t, err)
		peeked, found, err = settingsBackupPeekResetToken(bg, expToken)
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, rec, peeked)

		mr.FastForward(settingsCardLifetime + time.Second)

		_, found, err = settingsBackupPeekResetToken(bg, expToken)
		require.NoError(t, err)
		assert.False(t, found)
		_, found, err = settingsBackupConsumeResetToken(bg, expToken)
		require.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("import_token_rejects_oversized_payload_and_nil_redis", func(t *testing.T) {
		mr := phase28InProcessMiniredis(t)

		// 1. Oversized serialized payload fails with errSettingsBackupPayloadTooLarge and writes 0 Redis keys (T-30-05).
		oversizedBkp := backup.NewBackupFormat(-1003000203, "Oversized Group", 777000, []string{"rules"})
		oversizedBkp.Data["rules"] = map[string]interface{}{
			"settings": map[string]interface{}{
				"rules": strings.Repeat("x", maxSettingsBackupRedisBytes+1024),
			},
		}
		token, err := settingsBackupPutImportToken(bg, settingsBackupImportRecord{
			UserID:  777000,
			ChatID:  -1003000203,
			Modules: []string{"rules"},
			Backup:  oversizedBkp,
		})
		require.ErrorIs(t, err, errSettingsBackupPayloadTooLarge)
		assert.Empty(t, token)
		assert.Empty(t, mr.Keys(), "oversized import payload must write zero keys to Redis")

		// 2. Nil Redis client fails closed with errSettingsCopyNoRedis across all 6 token operations.
		origRedis := settingsBackupRedisClient
		settingsBackupRedisClient = func() *redis.Client { return nil }
		t.Cleanup(func() { settingsBackupRedisClient = origRedis })

		validToken := "0123456789abcdef"

		tok, err := settingsBackupPutImportToken(bg, settingsBackupImportRecord{UserID: 1, ChatID: -1001, Modules: []string{"rules"}})
		assert.ErrorIs(t, err, errSettingsCopyNoRedis)
		assert.Empty(t, tok)

		_, found, err := settingsBackupPeekImportToken(bg, validToken)
		assert.ErrorIs(t, err, errSettingsCopyNoRedis)
		assert.False(t, found)

		_, found, err = settingsBackupConsumeImportToken(bg, validToken)
		assert.ErrorIs(t, err, errSettingsCopyNoRedis)
		assert.False(t, found)

		tok, err = settingsBackupPutResetToken(bg, settingsBackupResetRecord{UserID: 1, ChatID: -1001, Modules: []string{"rules"}})
		assert.ErrorIs(t, err, errSettingsCopyNoRedis)
		assert.Empty(t, tok)

		_, found, err = settingsBackupPeekResetToken(bg, validToken)
		assert.ErrorIs(t, err, errSettingsCopyNoRedis)
		assert.False(t, found)

		_, found, err = settingsBackupConsumeResetToken(bg, validToken)
		assert.ErrorIs(t, err, errSettingsCopyNoRedis)
		assert.False(t, found)
	})
}
