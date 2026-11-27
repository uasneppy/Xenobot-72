package modules

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/db/backup"
	"github.com/uasneppy/Xenobot-72/xenobot/db/lang"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/chat_status"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/error_handling"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/formatting"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/helpers"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/ratelimit"
)

var backupModule = moduleStruct{
	moduleName: "Backup",
}

type pendingImportState struct {
	backup    *backup.BackupFormat
	modules   []string
	token     string
	expiresAt time.Time
}

type pendingResetState struct {
	modules   []string
	token     string
	expiresAt time.Time
}

type backupExportOutcome int

const (
	backupExportSent backupExportOutcome = iota
	backupExportRateLimited
	backupExportNoModules
	backupExportStartBot
	backupExportSendFailed
	backupExportFailed
)

type backupExportResult struct {
	Outcome     backupExportOutcome
	Cooldown    time.Duration
	ModuleCount int
	Backup      *backup.BackupFormat
}

type settingsBackupImportRecord struct {
	UserID  int64                `json:"u"`
	ChatID  int64                `json:"c"`
	Modules []string             `json:"m"`
	Backup  *backup.BackupFormat `json:"b"`
}

type settingsBackupResetRecord struct {
	UserID  int64    `json:"u"`
	ChatID  int64    `json:"c"`
	Modules []string `json:"m"`
}

var (
	pendingMu                        sync.Mutex
	pendingImports                   = make(map[int64]pendingImportState)
	pendingResets                    = make(map[int64]pendingResetState)
	errNoValidModule                 = errors.New("no valid modules in arguments")
	errSettingsBackupPayloadTooLarge = errors.New("backup settings: pending import payload exceeds size limit")

	backupDownloadBaseURL     = "https://api.telegram.org/file/bot"
	backupDownloadHTTPClient  = &http.Client{}
	backupExportChatData      = backup.ExportChatData
	settingsBackupRedisClient = settingsCopyDefaultRedis
)

const (
	maxBackupFileSize = 10 * 1024 * 1024
	pendingBackupTTL  = 10 * time.Minute
	// Short action values leave room for an int64 chat ID and nonce under
	// Telegram's 64-byte callback-data limit.
	backupActionConfirmImport = "ci"
	backupActionCancelImport  = "xi"
	backupActionConfirmReset  = "cr"
	backupActionCancelReset   = "xr"

	// Cross-replica Redis key prefixes for private Backup settings page one-time confirm cards
	// (P30-01), kept outside xenobot:cache: so cache clears do not wipe in-flight confirmations.
	settingsBackupImportCardKeyPrefix = "xenobot:settings:backup:import:"
	settingsBackupResetCardKeyPrefix  = "xenobot:settings:backup:reset:"
	// Cap serialized pending import payload in Redis at 2 MiB to bound Redis memory (T-30-05).
	maxSettingsBackupRedisBytes = 2 * 1024 * 1024
)

func newPendingToken() (string, error) {
	var value [8]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate pending backup token: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}

func storePendingImport(chatID int64, bkp *backup.BackupFormat, modules []string) (string, error) {
	token, err := newPendingToken()
	if err != nil {
		return "", err
	}

	pendingMu.Lock()
	defer pendingMu.Unlock()
	pendingImports[chatID] = pendingImportState{
		backup:    bkp,
		modules:   modules,
		token:     token,
		expiresAt: time.Now().Add(pendingBackupTTL),
	}
	return token, nil
}

func consumePendingImport(chatID int64, token string) (*backup.BackupFormat, []string, bool) {
	pendingMu.Lock()
	defer pendingMu.Unlock()
	pending, ok := pendingImports[chatID]
	if !ok {
		return nil, nil, false
	}
	if !time.Now().Before(pending.expiresAt) {
		delete(pendingImports, chatID)
		return nil, nil, false
	}
	if pending.token != token {
		return nil, nil, false
	}
	delete(pendingImports, chatID)
	return pending.backup, pending.modules, true
}

func discardPendingImport(chatID int64, token string) bool {
	_, _, ok := consumePendingImport(chatID, token)
	return ok
}

func storePendingReset(chatID int64, modules []string) (string, error) {
	token, err := newPendingToken()
	if err != nil {
		return "", err
	}

	pendingMu.Lock()
	defer pendingMu.Unlock()
	pendingResets[chatID] = pendingResetState{
		modules:   modules,
		token:     token,
		expiresAt: time.Now().Add(pendingBackupTTL),
	}
	return token, nil
}

func consumePendingReset(chatID int64, token string) ([]string, bool) {
	pendingMu.Lock()
	defer pendingMu.Unlock()
	pending, ok := pendingResets[chatID]
	if !ok {
		return nil, false
	}
	if !time.Now().Before(pending.expiresAt) {
		delete(pendingResets, chatID)
		return nil, false
	}
	if pending.token != token {
		return nil, false
	}
	delete(pendingResets, chatID)
	return pending.modules, true
}

func discardPendingReset(chatID int64, token string) bool {
	_, ok := consumePendingReset(chatID, token)
	return ok
}

func cleanupExpiredPending() {
	pendingMu.Lock()
	defer pendingMu.Unlock()
	now := time.Now()
	for k, v := range pendingImports {
		if !now.Before(v.expiresAt) {
			delete(pendingImports, k)
		}
	}
	for k, v := range pendingResets {
		if !now.Before(v.expiresAt) {
			delete(pendingResets, k)
		}
	}
}

func startPendingCleanupTicker() {
	go func() {
		defer error_handling.RecoverFromPanic("pendingBackupCleanup", "backup")
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			func() {
				defer error_handling.RecoverFromPanic("pendingBackupCleanupTick", "backup")
				cleanupExpiredPending()
			}()
		}
	}()
}

func settingsBackupImportTokenKey(token string) string {
	return settingsBackupImportCardKeyPrefix + token
}

func settingsBackupResetTokenKey(token string) string {
	return settingsBackupResetCardKeyPrefix + token
}

func settingsBackupPutToken(ctx context.Context, keyFn func(string) string, rec any) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	client := settingsBackupRedisClient()
	if client == nil {
		return "", errSettingsCopyNoRedis
	}
	payload, err := json.Marshal(rec)
	if err != nil {
		return "", err
	}
	if len(payload) > maxSettingsBackupRedisBytes {
		return "", errSettingsBackupPayloadTooLarge
	}
	token, err := settingsCopyNewToken()
	if err != nil {
		return "", err
	}
	ttl := settingsCardLifetime
	if ttl <= 0 {
		ttl = time.Millisecond
	}
	putCtx, cancel := context.WithTimeout(ctx, settingsReadTimeout)
	defer cancel()
	stored, err := client.SetNX(putCtx, keyFn(token), payload, ttl).Result()
	if err != nil {
		return "", err
	}
	if !stored {
		return "", errSettingsCopyTokenTaken
	}
	return token, nil
}

func settingsBackupPeekRawToken(ctx context.Context, keyFn func(string) string, token string) ([]byte, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	client := settingsBackupRedisClient()
	if client == nil {
		return nil, false, errSettingsCopyNoRedis
	}
	if !settingsCopyTokenValid(token) {
		return nil, false, nil
	}
	getCtx, cancel := context.WithTimeout(ctx, settingsReadTimeout)
	defer cancel()
	raw, err := client.Get(getCtx, keyFn(token)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return raw, true, nil
}

func settingsBackupConsumeRawToken(ctx context.Context, keyFn func(string) string, token string) ([]byte, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	client := settingsBackupRedisClient()
	if client == nil {
		return nil, false, errSettingsCopyNoRedis
	}
	if !settingsCopyTokenValid(token) {
		return nil, false, nil
	}
	consumeCtx, cancel := context.WithTimeout(ctx, settingsReadTimeout)
	defer cancel()
	raw, err := settingsCopyConsumeScript.Run(consumeCtx, client, []string{keyFn(token)}).Text()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return []byte(raw), true, nil
}

func unmarshalSettingsBackupImportRecord(raw []byte) (settingsBackupImportRecord, error) {
	var rec settingsBackupImportRecord
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&rec); err != nil {
		return settingsBackupImportRecord{}, err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return settingsBackupImportRecord{}, errors.New("backup settings: trailing data in pending import record")
	}
	return rec, nil
}

func unmarshalSettingsBackupResetRecord(raw []byte) (settingsBackupResetRecord, error) {
	var rec settingsBackupResetRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return settingsBackupResetRecord{}, err
	}
	return rec, nil
}

func settingsBackupReadRecord[T any](
	ctx context.Context,
	readFn func(context.Context, func(string) string, string) ([]byte, bool, error),
	keyFn func(string) string,
	decodeFn func([]byte) (T, error),
	token string,
) (T, bool, error) {
	var zero T
	raw, found, err := readFn(ctx, keyFn, token)
	if err != nil || !found {
		return zero, found, err
	}
	rec, err := decodeFn(raw)
	if err != nil {
		return zero, false, err
	}
	return rec, true, nil
}

func settingsBackupPutImportToken(ctx context.Context, rec settingsBackupImportRecord) (string, error) {
	return settingsBackupPutToken(ctx, settingsBackupImportTokenKey, rec)
}

func settingsBackupPeekImportToken(ctx context.Context, token string) (settingsBackupImportRecord, bool, error) {
	return settingsBackupReadRecord(ctx, settingsBackupPeekRawToken, settingsBackupImportTokenKey, unmarshalSettingsBackupImportRecord, token)
}

func settingsBackupConsumeImportToken(ctx context.Context, token string) (settingsBackupImportRecord, bool, error) {
	return settingsBackupReadRecord(ctx, settingsBackupConsumeRawToken, settingsBackupImportTokenKey, unmarshalSettingsBackupImportRecord, token)
}

func settingsBackupPutResetToken(ctx context.Context, rec settingsBackupResetRecord) (string, error) {
	return settingsBackupPutToken(ctx, settingsBackupResetTokenKey, rec)
}

func settingsBackupPeekResetToken(ctx context.Context, token string) (settingsBackupResetRecord, bool, error) {
	return settingsBackupReadRecord(ctx, settingsBackupPeekRawToken, settingsBackupResetTokenKey, unmarshalSettingsBackupResetRecord, token)
}

func settingsBackupConsumeResetToken(ctx context.Context, token string) (settingsBackupResetRecord, bool, error) {
	return settingsBackupReadRecord(ctx, settingsBackupConsumeRawToken, settingsBackupResetTokenKey, unmarshalSettingsBackupResetRecord, token)
}

// applyBackupExport runs the shared export flow used by both /export and the private Backup
// settings page: rate limit acquisition, chat export, JSON serialization, and private-chat
// document delivery to userID (never chatID).
func applyBackupExport(b *gotgbot.Bot, tr *i18n.Translator, chatID int64, chatTitle string, userID int64, modules []string) backupExportResult {
	limiter := ratelimit.GetBackupRateLimiter()
	if allowed, cooldown := limiter.AcquireExport(chatID); !allowed {
		return backupExportResult{
			Outcome:  backupExportRateLimited,
			Cooldown: cooldown,
		}
	}

	bkp, err := backupExportChatData(chatID, chatTitle, userID, modules)
	if err != nil {
		log.Errorf("[Backup] Export failed for chat %d: %v", chatID, err)
		return backupExportResult{Outcome: backupExportFailed}
	}

	if bkp == nil || len(bkp.Data) == 0 {
		return backupExportResult{Outcome: backupExportNoModules}
	}

	jsonData, err := bkp.ToJSON()
	if err != nil {
		log.Errorf("[Backup] Failed to marshal backup: %v", err)
		return backupExportResult{Outcome: backupExportFailed}
	}

	fileName := fmt.Sprintf("xenobot72_backup_%d_%s.json", chatID, time.Now().Format("20060102_150405"))
	caption := buildExportCaption(tr, bkp)

	// The backup holds every note, filter, custom message and setting of the group, so it goes
	// to the admin's private chat and never into the group.
	_, err = b.SendDocument(
		userID,
		gotgbot.InputFileByReader(fileName, bytes.NewReader(jsonData)),
		&gotgbot.SendDocumentOpts{
			Caption:   caption,
			ParseMode: "HTML",
		},
	)
	if settingsEditIsForbidden(err) {
		limiter.ReleaseExport(chatID)
		return backupExportResult{Outcome: backupExportStartBot}
	}
	if err != nil {
		log.Errorf("[Backup] Failed to send document: %v", err)
		return backupExportResult{
			Outcome:     backupExportSendFailed,
			ModuleCount: len(bkp.Data),
			Backup:      bkp,
		}
	}

	log.Infof("[Backup] Chat %d exported %d modules", chatID, len(bkp.Data))
	return backupExportResult{
		Outcome:     backupExportSent,
		ModuleCount: len(bkp.Data),
		Backup:      bkp,
	}
}

func (m moduleStruct) exportHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)

	if user == nil {
		return ext.EndGroups
	}

	if !chat_status.RequireGroup(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_group_only_error", "", chat_status.WithReply())
		return ext.EndGroups
	}

	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}

	if !chat_status.RequireBotAdmin(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	var modules []string
	if msg.Text != "" {
		args := strings.Fields(msg.Text)
		if len(args) > 1 {
			var parseErr error
			modules, parseErr = parseModuleArgs(args[1:], backup.IsValidModule)
			if parseErr != nil {
				text, _ := tr.GetString("backup_export_no_modules")
				_, _ = msg.Reply(b, text, formatting.Shtml())
				return ext.EndGroups
			}
		}
	}

	res := applyBackupExport(b, tr, chat.Id, chat.Title, user.Id, modules)
	switch res.Outcome {
	case backupExportRateLimited:
		cooldownStr := ratelimit.FormatCooldown(res.Cooldown)
		text, _ := tr.GetString("backup_export_rate_limited", i18n.TranslationParams{
			"time": cooldownStr,
		})
		_, _ = msg.Reply(b, text, formatting.Shtml())
	case backupExportNoModules:
		text, _ := tr.GetString("backup_export_no_modules")
		_, _ = msg.Reply(b, text, formatting.Shtml())
	case backupExportFailed:
		text, _ := tr.GetString("backup_export_failed")
		_, _ = msg.Reply(b, text, formatting.Shtml())
	case backupExportStartBot:
		text, _ := tr.GetString("backup_export_start_bot")
		_, _ = msg.Reply(b, text, formatting.Shtml())
	case backupExportSendFailed:
		text, _ := tr.GetString("backup_export_success_text", i18n.TranslationParams{
			"modules": fmt.Sprintf("%d", res.ModuleCount),
		})
		_, _ = msg.Reply(b, text, formatting.Shtml())
	case backupExportSent:
	}

	return ext.EndGroups
}

func validateImportRequest(b *gotgbot.Bot, ctx *ext.Context) (*gotgbot.Message, *gotgbot.Chat, *gotgbot.User, *i18n.Translator, bool) {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)

	if user == nil {
		return nil, nil, nil, nil, false
	}

	if !chat_status.RequireGroup(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_group_only_error", "", chat_status.WithReply())
		return nil, nil, nil, nil, false
	}

	if !chat_status.RequireBotAdmin(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		return nil, nil, nil, nil, false
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	if !chat_status.RequireUserOwner(b, ctx, nil, user.Id) {
		text, _ := tr.GetString("backup_import_creator_only")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return nil, nil, nil, nil, false
	}

	return msg, chat, user, tr, true
}

func downloadBackupFile(b *gotgbot.Bot, doc *gotgbot.Document, tr *i18n.Translator) ([]byte, string) {
	if doc == nil || !strings.HasSuffix(strings.ToLower(doc.FileName), ".json") {
		text, _ := tr.GetString("backup_import_invalid_file")
		return nil, text
	}

	if doc.FileSize > maxBackupFileSize {
		text, _ := tr.GetString("backup_import_file_too_large")
		return nil, text
	}

	fileData, err := downloadTelegramFile(b, doc.FileId)
	if err != nil {
		log.Errorf("[Backup] Failed to download file: %v", err)
		if errors.Is(err, errTelegramFileTooLarge) {
			text, _ := tr.GetString("backup_import_file_too_large")
			return nil, text
		}
		text, _ := tr.GetString("backup_import_download_failed")
		return nil, text
	}

	return fileData, ""
}

// downloadAndParseBackupFile downloads, parses, validates, and checks format version compatibility
// for a backup JSON document. Shared by /import and the private Backup settings page.
func downloadAndParseBackupFile(b *gotgbot.Bot, doc *gotgbot.Document, tr *i18n.Translator) (*backup.BackupFormat, string) {
	fileData, errText := downloadBackupFile(b, doc, tr)
	if fileData == nil {
		return nil, errText
	}

	bkp, err := backup.BackupFormatFromJSON(fileData)
	if err != nil {
		log.Errorf("[Backup] Failed to parse backup: %v", err)
		text, _ := tr.GetString("backup_import_invalid_file")
		return nil, text
	}

	if err := bkp.Validate(); err != nil {
		log.Errorf("[Backup] Invalid backup: %v", err)
		text, _ := tr.GetString("backup_import_invalid_file")
		return nil, text
	}

	if !bkp.IsCompatibleVersion() {
		text, _ := tr.GetString("backup_import_version_mismatch")
		return nil, text
	}

	return bkp, ""
}

func parseImportModules(text string, backupData map[string]interface{}) ([]string, error) {
	if text != "" {
		args := strings.Fields(text)
		if len(args) > 1 {
			return parseModuleArgs(args[1:], func(module string) bool {
				if !backup.IsValidModule(module) {
					return false
				}
				_, ok := backupData[module]
				return ok
			})
		}
	}
	return nil, nil
}

func parseModuleArgs(args []string, valid func(string) bool) ([]string, error) {
	if len(args) == 0 {
		return nil, nil
	}
	modules := make([]string, 0, len(args))
	seen := make(map[string]struct{}, len(args))
	for _, arg := range args {
		module := strings.ToLower(arg)
		if module == "" || !valid(module) {
			continue
		}
		if _, ok := seen[module]; ok {
			continue
		}
		seen[module] = struct{}{}
		modules = append(modules, module)
	}
	if len(modules) == 0 {
		return nil, errNoValidModule
	}
	return modules, nil
}

func (m moduleStruct) importHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	msg, chat, _, tr, ok := validateImportRequest(b, ctx)
	if !ok {
		return ext.EndGroups
	}

	if msg.ReplyToMessage == nil || msg.ReplyToMessage.Document == nil {
		text, _ := tr.GetString("backup_import_no_reply")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	doc := msg.ReplyToMessage.Document

	bkp, errText := downloadAndParseBackupFile(b, doc, tr)
	if bkp == nil {
		_, _ = msg.Reply(b, errText, formatting.Shtml())
		return ext.EndGroups
	}

	importModules, parseErr := parseImportModules(msg.Text, bkp.Data)
	if parseErr != nil {
		text, _ := tr.GetString("backup_import_invalid_file")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	if len(importModules) == 0 {
		importModules = bkp.Modules
	}

	limiter := ratelimit.GetBackupRateLimiter()
	if allowed, cooldown := limiter.AcquireImport(chat.Id); !allowed {
		text, _ := tr.GetString("backup_import_rate_limited", i18n.TranslationParams{
			"time": ratelimit.FormatCooldown(cooldown),
		})
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	token, err := storePendingImport(chat.Id, bkp, importModules)
	if err != nil {
		log.Errorf("[Backup] Failed to store pending import: %v", err)
		text, _ := tr.GetString("backup_import_failed", i18n.TranslationParams{"error": err.Error()})
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	confirmText, _ := tr.GetString("backup_import_confirm", i18n.TranslationParams{
		"modules": fmt.Sprintf("%d", len(importModules)),
		"list":    buildModuleList(importModules),
	})

	keyboard := buildImportKeyboard(tr, chat.Id, token)

	_, err = msg.Reply(b, confirmText, &gotgbot.SendMessageOpts{
		ParseMode:   "HTML",
		ReplyMarkup: keyboard,
	})
	if err != nil {
		discardPendingImport(chat.Id, token)
		log.Errorf("[Backup] Failed to send confirmation: %v", err)
	}

	return ext.EndGroups
}

func (m moduleStruct) resetHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)

	if user == nil {
		return ext.EndGroups
	}

	if !chat_status.RequireGroup(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_group_only_error", "", chat_status.WithReply())
		return ext.EndGroups
	}

	if !chat_status.RequireBotAdmin(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	if !chat_status.RequireUserOwner(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_owner_cmd_error", "chat_status_owner_button_error", chat_status.WithReply())
		return ext.EndGroups
	}

	var resetModules []string
	if msg.Text != "" {
		args := strings.Fields(msg.Text)
		if len(args) > 1 {
			var parseErr error
			resetModules, parseErr = parseModuleArgs(args[1:], backup.IsValidModule)
			if parseErr != nil {
				text, _ := tr.GetString("backup_export_no_modules")
				_, _ = msg.Reply(b, text, formatting.Shtml())
				return ext.EndGroups
			}
		}
	}

	if len(resetModules) == 0 {
		resetModules = backup.AllExportableModules()
	}

	limiter := ratelimit.GetBackupRateLimiter()
	if allowed, cooldown := limiter.AcquireReset(chat.Id); !allowed {
		text, _ := tr.GetString("backup_reset_rate_limited", i18n.TranslationParams{
			"time": ratelimit.FormatCooldown(cooldown),
		})
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	token, err := storePendingReset(chat.Id, resetModules)
	if err != nil {
		log.Errorf("[Backup] Failed to store pending reset: %v", err)
		text, _ := tr.GetString("backup_reset_failed", i18n.TranslationParams{"error": err.Error()})
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	confirmText, _ := tr.GetString("backup_reset_confirm", i18n.TranslationParams{
		"modules": fmt.Sprintf("%d", len(resetModules)),
		"list":    buildModuleList(resetModules),
	})

	keyboard := buildResetKeyboard(tr, chat.Id, token)

	_, err = msg.Reply(b, confirmText, &gotgbot.SendMessageOpts{
		ParseMode:   "HTML",
		ReplyMarkup: keyboard,
	})
	if err != nil {
		discardPendingReset(chat.Id, token)
		log.Errorf("[Backup] Failed to send confirmation: %v", err)
	}

	return ext.EndGroups
}

func (m moduleStruct) backupCallbackHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	user := query.From
	chat := ctx.EffectiveChat
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	if chat == nil {
		return answerInvalidCallback(b, ctx, query)
	}

	if !chat_status.RequireUserOwner(b, ctx, nil, user.Id) {
		text, _ := tr.GetString("backup_import_creator_only")
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text:      text,
			ShowAlert: true,
		})
		return ext.EndGroups
	}

	decoded, ok := decodeCallbackData(query.Data, "backup")
	if !ok {
		return answerInvalidCallback(b, ctx, query)
	}

	action, _ := decoded.Field("a")
	chatIDStr, _ := decoded.Field("c")
	token, _ := decoded.Field("t")
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	validAction := action == backupActionConfirmImport ||
		action == backupActionCancelImport ||
		action == backupActionConfirmReset ||
		action == backupActionCancelReset
	if err != nil || chatID != chat.Id || token == "" || !validAction {
		return answerInvalidCallback(b, ctx, query)
	}

	switch action {
	case backupActionConfirmImport:
		return m.handleConfirmImport(b, ctx, tr, chat, token)
	case backupActionCancelImport:
		return m.handleCancelImport(b, ctx, tr, query, token)
	case backupActionConfirmReset:
		return m.handleConfirmReset(b, ctx, tr, chat, token)
	case backupActionCancelReset:
		return m.handleCancelReset(b, ctx, tr, query, token)
	}
	return ext.EndGroups
}

// applyBackupImport executes a validated backup import for chatID and logs numeric IDs only.
// Shared by /import confirmation and the private Backup settings page.
func applyBackupImport(chatID int64, bkp *backup.BackupFormat, modules []string) (backup.ImportReport, error) {
	report, err := backup.ImportChatDataReport(chatID, bkp, modules)
	if err != nil {
		log.Errorf("[Backup] Import failed for chat %d: %v", chatID, err)
		return backup.ImportReport{}, err
	}
	count := len(modules)
	if count == 0 && bkp != nil {
		count = len(bkp.Modules)
	}
	log.Infof("[Backup] Chat %d imported %d modules", chatID, count)
	return report, nil
}

// formatBackupImportSuccess formats the import completion summary and skipped/log-channel/archive-channel hints.
func formatBackupImportSuccess(tr *i18n.Translator, modules []string, report backup.ImportReport) string {
	text, _ := tr.GetString("backup_import_success", i18n.TranslationParams{
		"modules": fmt.Sprintf("%d", len(modules)),
		"list":    buildModuleList(modules),
	})
	var hints []string
	if report.Skipped > 0 {
		skipped, _ := tr.GetString("backup_import_skipped_entries", i18n.TranslationParams{"count": report.Skipped})
		hints = append(hints, skipped)
	}
	if report.LogChannelSkipped {
		logSkipped, _ := tr.GetString("backup_import_log_skipped")
		hints = append(hints, logSkipped)
	}
	if report.ArchiveChannelSkipped {
		archiveSkipped, _ := tr.GetString("backup_import_archive_skipped")
		hints = append(hints, archiveSkipped)
	}
	if len(hints) > 0 {
		text = strings.TrimRight(text, "\n") + "\n\n" + strings.Join(hints, "\n")
	}
	return text
}

func (m moduleStruct) handleConfirmImport(b *gotgbot.Bot, ctx *ext.Context, tr *i18n.Translator, chat *gotgbot.Chat, token string) error {
	bkp, modules, ok := consumePendingImport(chat.Id, token)
	if !ok {
		text, _ := tr.GetString("backup_import_expired")
		_, err := b.SendMessage(chat.Id, text, formatting.Shtml())
		if err != nil {
			log.Errorf("[Backup] Failed to send message: %v", err)
		}
		return ext.EndGroups
	}

	report, err := applyBackupImport(chat.Id, bkp, modules)
	if err != nil {
		text, _ := tr.GetString("backup_import_failed", i18n.TranslationParams{
			"error": err.Error(),
		})
		_, _ = b.SendMessage(chat.Id, text, formatting.Shtml())
		return ext.EndGroups
	}

	text := formatBackupImportSuccess(tr, modules, report)
	_, err = b.SendMessage(chat.Id, text, formatting.Shtml())
	if err != nil {
		log.Errorf("[Backup] Failed to send success message: %v", err)
	}

	return ext.EndGroups
}

func (m moduleStruct) handleCancelPending(b *gotgbot.Bot, ctx *ext.Context, tr *i18n.Translator, query *gotgbot.CallbackQuery, token string, discard func(int64, string) bool, cancelKey, expiredKey string) error {
	chat := ctx.EffectiveChat

	if !discard(chat.Id, token) {
		text, _ := tr.GetString(expiredKey)
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return ext.EndGroups
	}

	text, _ := tr.GetString(cancelKey)
	_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
		Text: text,
	})

	msg := ctx.EffectiveMessage
	if msg != nil {
		_, _, _ = msg.EditText(b, &gotgbot.EditMessageTextOpts{Text: text, ParseMode: "HTML"})
	}

	return ext.EndGroups
}

func (m moduleStruct) handleCancelImport(b *gotgbot.Bot, ctx *ext.Context, tr *i18n.Translator, query *gotgbot.CallbackQuery, token string) error {
	return m.handleCancelPending(b, ctx, tr, query, token, discardPendingImport, "backup_import_cancelled", "backup_import_expired")
}

// applyBackupReset clears the selected modules (or all exportable modules when empty) for chatID.
// Shared by /reset confirmation and the private Backup settings page.
func applyBackupReset(chatID int64, modules []string) error {
	if err := backup.ClearChatData(chatID, modules); err != nil {
		log.Errorf("[Backup] Reset failed for chat %d: %v", chatID, err)
		return err
	}
	count := len(modules)
	if count == 0 {
		count = len(backup.AllExportableModules())
	}
	log.Infof("[Backup] Chat %d reset %d modules", chatID, count)
	return nil
}

// formatBackupResetSuccess formats the reset completion summary.
func formatBackupResetSuccess(tr *i18n.Translator, modules []string) string {
	text, _ := tr.GetString("backup_reset_success", i18n.TranslationParams{
		"modules": fmt.Sprintf("%d", len(modules)),
		"list":    buildModuleList(modules),
	})
	return text
}

func (m moduleStruct) handleConfirmReset(b *gotgbot.Bot, ctx *ext.Context, tr *i18n.Translator, chat *gotgbot.Chat, token string) error {
	modules, ok := consumePendingReset(chat.Id, token)
	if !ok || len(modules) == 0 {
		text, _ := tr.GetString("backup_reset_expired")
		_, _ = b.SendMessage(chat.Id, text, formatting.Shtml())
		return ext.EndGroups
	}

	if err := applyBackupReset(chat.Id, modules); err != nil {
		text, _ := tr.GetString("backup_reset_failed", i18n.TranslationParams{
			"error": err.Error(),
		})
		_, _ = b.SendMessage(chat.Id, text, formatting.Shtml())
		return ext.EndGroups
	}

	text := formatBackupResetSuccess(tr, modules)
	_, err := b.SendMessage(chat.Id, text, formatting.Shtml())
	if err != nil {
		log.Errorf("[Backup] Failed to send success message: %v", err)
	}

	return ext.EndGroups
}

func (m moduleStruct) handleCancelReset(b *gotgbot.Bot, ctx *ext.Context, tr *i18n.Translator, query *gotgbot.CallbackQuery, token string) error {
	return m.handleCancelPending(b, ctx, tr, query, token, discardPendingReset, "backup_reset_cancelled", "backup_reset_expired")
}

func buildExportCaption(tr *i18n.Translator, backup *backup.BackupFormat) string {
	modulesList := buildModuleList(backup.Modules)
	caption, _ := tr.GetString("backup_export_success", i18n.TranslationParams{
		"modules": fmt.Sprintf("%d", len(backup.Data)),
		"list":    modulesList,
		"chat":    html.EscapeString(backup.ChatName),
		"time":    formatChatTime(backup.ExportedAt, chatClockOf(backup.ChatID), "2006-01-02 15:04:05"),
	})
	return caption
}

func buildModuleList(modules []string) string {
	if len(modules) == 0 {
		return ""
	}
	return "• " + strings.Join(modules, "\n• ")
}

func buildImportKeyboard(tr *i18n.Translator, chatID int64, token string) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text: trS(tr, "button_confirm_import"),
					CallbackData: encodeCallbackData("backup", map[string]string{
						"a": backupActionConfirmImport,
						"c": fmt.Sprintf("%d", chatID),
						"t": token,
					}),
				},
				{
					Text: trS(tr, "button_cancel_import"),
					CallbackData: encodeCallbackData("backup", map[string]string{
						"a": backupActionCancelImport,
						"c": fmt.Sprintf("%d", chatID),
						"t": token,
					}),
				},
			},
		},
	}
}

func buildResetKeyboard(tr *i18n.Translator, chatID int64, token string) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text: trS(tr, "button_confirm_reset"),
					CallbackData: encodeCallbackData("backup", map[string]string{
						"a": backupActionConfirmReset,
						"c": fmt.Sprintf("%d", chatID),
						"t": token,
					}),
				},
			},
			{
				{
					Text: trS(tr, "button_cancel_reset"),
					CallbackData: encodeCallbackData("backup", map[string]string{
						"a": backupActionCancelReset,
						"c": fmt.Sprintf("%d", chatID),
						"t": token,
					}),
				},
			},
		},
	}
}

func LoadBackup(dispatcher *ext.Dispatcher) {
	SetModuleEnabled(backupModule.moduleName, true)

	dispatcher.AddHandler(helpers.NewBotCommand("export", backupModule.exportHandler))
	dispatcher.AddHandler(helpers.NewBotCommand("import", backupModule.importHandler))
	dispatcher.AddHandler(helpers.NewBotCommand("reset", backupModule.resetHandler))

	dispatcher.AddHandler(handlers.NewCallback(
		callbackquery.Prefix("backup"),
		backupModule.backupCallbackHandler,
	))

	log.Info("[Backup] Module loaded successfully")
}

func init() {
	RegisterLegacyModule("Backup", 270, LoadBackup)
	startPendingCleanupTicker()
}
