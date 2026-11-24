# Phase 30: Backup Page and Custom Messages in Backups - Pattern Map

**Mapped:** 2026-11-23
**Files analyzed:** 18
**Analogs found:** 18 / 18

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `fuku/modules/backup_phase30_legacy_request_test.go` | test (golden) | request-response | `fuku/modules/phase29_legacy_request_test.go`, `fuku/modules/backup_export_private_test.go` | exact |
| `fuku/db/eventmsg/validate_write.go` | service / repository | CRUD / validation | `fuku/db/eventmsg/validate_write.go` | exact (self) |
| `fuku/db/backup/types.go` | model / schema | transform | `fuku/db/backup/types.go` | exact (self) |
| `fuku/db/backup/backup.go` | repository | CRUD / batch | `fuku/db/backup/backup.go` | exact (self) |
| `fuku/db/backup/event_messages_backup_test.go` | test | CRUD / validation | `fuku/db/backup/event_messages_backup_test.go`, `fuku/db/backup/greeted_backup_test.go` | exact |
| `fuku/db/backup/roundtrip_test.go` | test | CRUD / batch | `fuku/db/backup/roundtrip_test.go` | exact (self) |
| `fuku/modules/backup.go` | controller / service | request-response / file-I/O | `fuku/modules/backup.go` | exact (self) |
| `fuku/modules/settings_backup.go` | controller / UI | request-response / file-I/O | `fuku/modules/settings_copy.go`, `fuku/modules/settings_moddefaults.go` | exact |
| `fuku/modules/settings_input_door.go` | middleware / router | event-driven | `fuku/modules/settings_input_door.go`, `fuku/modules/settings_input.go` | exact (self) |
| `fuku/modules/settings_backup_test.go` | test | request-response | `fuku/modules/settings_copy_test.go`, `fuku/modules/backup_test.go` | exact |
| `fuku/modules/settings_phase30_test.go` | test (safeguards) | request-response | `fuku/modules/settings_phase29_test.go` | exact |
| `fuku/modules/phase30_e2e_test.go` | test (E2E) | request-response / file-I/O | `fuku/modules/phase29_e2e_test.go` | exact |
| `fuku/modules/settings_main_test.go` | test | transform | `fuku/modules/settings_main_test.go` | exact (self) |
| `fuku/modules/settings_fwdblock_test.go` | test | transform | `fuku/modules/settings_fwdblock_test.go` | exact (self) |
| `fuku/modules/settings_phase18_test.go` | test | transform | `fuku/modules/settings_phase18_test.go` | exact (self) |
| `fuku/modules/settings_phase19_test.go` | test | transform | `fuku/modules/settings_phase19_test.go` | exact (self) |
| `fuku/modules/settings_phase29_test.go` | test | transform | `fuku/modules/settings_phase29_test.go` | exact (self) |
| `fuku/i18n/settings_locale_test.go` & `locales/*.yml` | config / i18n | transform | `fuku/i18n/settings_locale_test.go`, `locales/en.yml` | exact (self) |

---

## Pattern Assignments

### 1. `fuku/db/eventmsg/validate_write.go` & `fuku/db/backup/types.go` & `fuku/db/backup/backup.go` (MSG-10: Custom Messages in Backups)

**Analogs:**
- `fuku/db/eventmsg/validate_write.go` (lines 149-234)
- `fuku/db/backup/types.go` (lines 96-220)
- `fuku/db/backup/backup.go` (lines 21-188, 532-591, 769-791, 1057-1103)

**Transaction-safe shared validation writer (`fuku/db/eventmsg/validate_write.go`):**
- `fuku/db/backup/backup.go` runs multi-module imports and resets inside `withBackupTx(func(tx *gorm.DB) ([]string, error))` (`fuku/db/backup/backup.go:66-81`). Calling `db.DB.Transaction(...)` inside `withBackupTx` would open a second connection on SQLite (deadlocking or breaking rollback atomicity tested by `TestImportChatDataRollsBackEarlierModules` in `fuku/db/backup/roundtrip_test.go:351-377`).
- Extract the transactional body of `ReplaceAllValidated` into `ReplaceAllValidatedTx(tx *gorm.DB, chatID int64, rows []Message, opts Options, by int64) (WriteReport, error)` in `fuku/db/eventmsg/validate_write.go`:
```go
// ReplaceAllValidatedTx validates all candidate rows, deduplicates valid rows by (event, dest)
// (last valid wins), and replaces chatID's chat_event_messages and chat_event_message_options
// inside caller-supplied transaction tx. The caller is responsible for invalidating
// CacheKey(chatID) after tx commits.
func ReplaceAllValidatedTx(tx *gorm.DB, chatID int64, rows []Message, opts Options, by int64) (WriteReport, error) {
	if tx == nil {
		return WriteReport{}, fukuerrors.Wrap(errNoDatabase, "replace event messages")
	}
	// ... existing ValidateMessage loop, dedup, toWrite filter, and tx Delete/Create on
	// models.ChatEventMessage and models.ChatEventMessageOptions ...
}
```
- Notice how `ValidateMessage` (`validate_write.go:65-142`) already enforces:
  - `ValidEvent(msg.Event)` (`"unknown_event"` -> skipped and counted in `WriteReport.Skipped`)
  - `ValidDest(msg.Dest)` (`"unknown_dest"` -> skipped and counted)
  - `msg.Dest == DestPM && !HasPM(msg.Event)` (`"pm_not_supported"` -> skipped and counted)
  - `allowedMediaType` & `NoMedia` (`"invalid_media"`, `"media_not_allowed"` -> skipped and counted)
  - `content.ValidButtonURL` & non-blank button names (`"invalid_button"`, `"invalid_button_url"` -> skipped and counted)
  - `MaxButtons` (100 including `{rules}` token and `ReservedButtonsFor`) and `MaxRowButtons` (8) (`"buttons_limit"` -> skipped and counted)
  - `MaxTextLength` (4096) / `MaxCaptionLength` (1024) minus `ReservedTextFor` (600 for member requirements, 400 for lockdown) (`"text_too_long"` -> skipped and counted)
  - `NoSilent` normalization on `mreqmute|group`.
- Note on duplicate `(event, dest)` rows in a hand-crafted backup JSON: if a backup JSON lists two valid rows for the same `(event, dest)`, check whether duplicate keys should increment `report.Skipped` (like `keepValidEntries` in `fuku/db/backup/backup.go:1333-1346`) or if `ReplaceAllValidatedTx` should count overwritten duplicates in `WriteReport.Skipped` when `seen[key]` is already present. Wait: in `TestEventMsgValidateAndReplaceAll` (`fuku/db/eventmsg/validate_write_test.go:240-260`), the existing test passes two `EventBan|DestGroup` rows (`"first ban"` and `"final ban"`) and asserts `len(rep.Skipped) == 2` (only the 2 invalid rows). Keeping `ReplaceAllValidated` / `ReplaceAllValidatedTx`'s existing dedup behavior preserves `TestEventMsgValidateAndReplaceAll` without breaking Phase 29 tests!

**Backup module constant and schema (`fuku/db/backup/types.go`):**
- Add `BackupModuleMessages = "messages"` to the `BackupModule*` constants (`types.go:96-117`) and `AllExportableModules()` (`types.go:118-140`, bringing total exportable modules from 19 to 20).
- Define `MessagesBackup`:
```go
type MessagesBackup struct {
	Messages []models.ChatEventMessage       `json:"messages,omitempty"`
	Options  *models.ChatEventMessageOptions `json:"options,omitempty"`
}
```

**Export, Import, and Clear in `fuku/db/backup/backup.go`:**
- In `ExportModuleData` (`backup.go:21-64`):
```go
	case BackupModuleMessages:
		return exportMessagesData(chatID)
```
  where `exportMessagesData(chatID int64) (*MessagesBackup, error)` reads `models.ChatEventMessage` rows ordered by `event ASC, dest ASC` and `findChatSetting[models.ChatEventMessageOptions](chatID)`.
- In `importListModule` (`backup.go:575-591`):
```go
	case BackupModuleMessages:
		keys, err = importMessages(tx, chatID, data, report)
```
  where `importMessages(tx *gorm.DB, chatID int64, payload interface{}, report *ImportReport) ([]string, error)`:
  - Calls `decodeModuleData(payload, BackupModuleMessages, &data)` (so a non-map payload fails `TestImportModuleDataRejectsMalformedPayloadForEveryModule` in `fuku/db/backup/backup_test.go:215-225` with `"invalid messages data format"`).
  - Converts `data.Messages` to `[]eventmsg.Message` and `data.Options` to `eventmsg.Options`.
  - Calls `writeReport, err := eventmsg.ReplaceAllValidatedTx(tx, chatID, rows, opts, 0)`.
  - Increments `report.Skipped += len(writeReport.Skipped)`.
  - Returns `[]string{eventmsg.CacheKey(chatID)}, nil`.
- In `clearModuleData` (`backup.go:1057-1103`):
```go
	case BackupModuleMessages:
		return clearMessages(tx, chatID)
```
  where `clearMessages(tx *gorm.DB, chatID int64) ([]string, error)` deletes `models.ChatEventMessage` and `models.ChatEventMessageOptions` for `chatID` on `tx` (or calls `eventmsg.ReplaceAllValidatedTx(tx, chatID, nil, eventmsg.Options{}, 0)`) and returns `[]string{eventmsg.CacheKey(chatID)}, nil`.
- **Old exports without `"messages"` (`TestOldGreetingsExportDecodesToZeros` pattern):**
  - When importing an old backup whose `Modules` list does not include `"messages"`, `importChatData` (`backup.go:145-168`) iterates only over `modules` (`backup.Modules` when `len(modules) == 0`), so old exports without `"messages"` import cleanly without touching or failing on `chat_event_messages` / `chat_event_message_options`.
  - When importing a `"messages"` module block whose `options` or `messages` field is omitted (`{}`), `decodeModuleData` decodes `Messages: nil, Options: nil`, which clears existing custom messages and options to defaults (`PMDetails: false, PMLog: false, PlainTone: false`).

**Updating `fuku/db/backup/event_messages_backup_test.go` and `fuku/db/backup/roundtrip_test.go`:**
- In `fuku/db/backup/cleanupBackupChat` (`backup_test.go:293-364`), add cleanup for `&models.ChatEventMessage{}` and `&models.ChatEventMessageOptions{}` (and `dbcache.DeleteCache(eventmsg.CacheKey(chatID))`).
- Update `fuku/db/backup/event_messages_backup_test.go` (which previously asserted `TestEventMessagesStayOutOfBackup` for Phases 21–29) to test:
  1. Custom messages (`chat_event_messages`, both `group` and `pm` destinations, text, media, buttons, silent) and options (`chat_event_message_options`: `pm_details`, `pm_log`, `plain_tone`) ARE exported under `BackupModuleMessages` (`"messages"`), round-trip through `ToJSON` -> `BackupFormatFromJSON` -> `ImportChatDataReport`, and are cleared by `ClearChatData(srcChat, nil)` and `ClearChatData(srcChat, []string{BackupModuleMessages})` with `eventmsg.CacheKey(chatID)` invalidated.
  2. `ImportChatDataReport` applies `eventmsg.ValidateMessage` checks: invalid button URLs (`javascript:...`), over-limit text/captions (including member-requirement 3496 and lockdown 3696/624 reserves), disallowed media on member requirements, unsupported PM on lockdown events, and unknown future event keys are skipped and counted in `report.Skipped` while valid rows in the same payload are written.
  3. An old export without `"messages"` in `Modules` / `Data` imports cleanly without error and leaves existing custom messages untouched (or an export with `"messages": {}` decodes omitted fields to zeros/defaults, matching `TestOldGreetingsExportDecodesToZeros`).
  4. Tuning and staff tables (`chat_escalation`, `chat_offences`, `chat_manual_defaults`, `chat_reason_shortcuts`, `chat_advanced_thresholds`, `chat_conversations`) remain excluded from `AllExportableModules()`, export, import, and reset (`P30-03`, alongside `escalation_backup_test.go`, `modtuning_backup_test.go`, `advthresholds_backup_test.go`, `chatconv_backup_test.go`).
- Extend `TestAllModulesRoundTripEveryMeaningfulField` in `fuku/db/backup/roundtrip_test.go` to seed and assert `BackupModuleMessages` (`models.ChatEventMessage` and `models.ChatEventMessageOptions`).

---

### 2. `fuku/modules/backup.go` (Shared Apply Functions for `/export`, `/import`, `/reset` and the Backup Page)

**Analog:** `fuku/modules/backup.go` (lines 187-775)

**Core pattern — extract shared apply functions so commands and settings page share one implementation:**
- **Export seam (`applyBackupExport`):**
  - Extract from `exportHandler` (`backup.go:227-288`):
    - Rate-limit check: `limiter := ratelimit.GetBackupRateLimiter(); allowed, cooldown := limiter.AcquireExport(chatID)`
    - Data export: `bkp, err := backup.ExportChatData(chatID, chatTitle, userID, modules)`
    - Empty check: `len(bkp.Data) == 0`
    - JSON marshal: `jsonData, err := bkp.ToJSON()`
    - Filename & caption: `fileName := fmt.Sprintf("xenobot72_backup_%d_%s.json", chatID, time.Now().Format("20060102_150405"))`, `caption := buildExportCaption(tr, bkp)`
    - Private document delivery: `b.SendDocument(userID, gotgbot.InputFileByReader(fileName, bytes.NewReader(jsonData)), &gotgbot.SendDocumentOpts{Caption: caption, ParseMode: "HTML"})`
    - 403 handling: `if settingsEditIsForbidden(err) { limiter.ReleaseExport(chatID); ... }`
  - `exportHandler` calls `applyBackupExport` and keeps its exact command replies (`backup_export_rate_limited`, `backup_export_no_modules`, `backup_export_failed`, `backup_export_start_bot`, `backup_export_success_text`).
  - The Backup page's Export action (`be`) calls `applyBackupExport` and shows the appropriate page notice/toast.
- **Import file download & parse seam (`downloadAndParseBackupFile`):**
  - Extract from `importHandler` (`backup.go:396-421`):
    - `fileData, errText := downloadBackupFile(b, doc, tr)` (checks `.json` extension case-insensitively, `doc.FileSize <= maxBackupFileSize`, downloads via `downloadTelegramFile(b, doc.FileId)` with `maxBackupFileSize` body cap).
    - `bkp, err := backup.BackupFormatFromJSON(fileData)` -> on error returns `backup_import_invalid_file`.
    - `if err := bkp.Validate(); err != nil` -> on error returns `backup_import_invalid_file`.
    - `if !bkp.IsCompatibleVersion()` -> returns `backup_import_version_mismatch`.
- **Import execution & report formatting seam (`applyBackupImport` / `formatBackupImportSuccess`):**
  - Extract from `handleConfirmImport` (`backup.go:612-637`):
    - Calls `report, err := backup.ImportChatDataReport(chatID, bkp, modules)`
    - Formats `backup_import_success` + optional `backup_import_skipped_entries` (`report.Skipped > 0`) + optional `backup_import_log_skipped` (`report.LogChannelSkipped`).
- **Reset execution seam (`applyBackupReset`):**
  - Extract from `handleConfirmReset` (`backup.go:681-693`):
    - Calls `backup.ClearChatData(chatID, modules)`
    - Formats `backup_reset_success`.

---

### 3. `fuku/modules/settings_backup.go` & `fuku/modules/settings_input_door.go` (Private-Only Backup Settings Page: BKP-01, BKP-02, BKP-03)

**Analogs:**
- `fuku/modules/settings_copy.go` (lines 25-594) — private-only page registration, `Hidden` in Staff Groups, static `MainState`, Redis one-time token with Lua `GET+DEL` (`settingsCopyConsumeScript`), confirm card rendering.
- `fuku/modules/settings_moddefaults.go` (lines 56-109, 359-470) — `settingsTypedPages` registration for a private wait, prompt rendering, Cancel action, door `Typed` callback.
- `fuku/modules/settings_input.go` (lines 281-307) & `fuku/modules/settings_input_door.go` (lines 199-233, 289-349, 529-545) — typed-reply wait store and door routing.

**Page Registration & Action Codes (`fuku/modules/settings_backup.go`):**
- Page key: `settingsPageBackup = "bk"` (note: `"bk"` is free in `settingsPages`; `registerSettingsPage` checks `settingsPages[pg.Key]` for page keys and `settingsCoreActions` / `settingsActions` for action codes) or `settingsPageBackup = "ba"`. Using `"bk"` or `"ba"` at `Order: 83` (right after `cp` Copy Settings at `Order: 82` and before `d` Deletion at `Order: 85`), `Icon: "💾"`, `TitleKey: "settings_page_backup"`, `PrivateOnly: true`, `PrivateNoteKey: "settings_bkp_group_note"`, `Hidden: settingsBackupHidden` (hides in Staff Groups via `staff.GetStaffGroupFresh`).
- Action codes (all verified free across `settingsCoreActions` and `settingsActions`):
  - `settingsActBackupToggle = "bt"` (`Fields: []string{"m"}`, `Right: settingsRightAdmin`, `Validate: private(settingsBackupMaskValid)`): redraws the Backup page with module bitmask `m` (1..5 lowercase hex digits `"0"`..`"fffff"`, where bit `i` corresponds to `backup.AllExportableModules()[i]`). Enables stateless per-module toggling, "Select all" (`m=fffff`), and "Clear" (`m=0`) without Redis or DB I/O.
  - `settingsActBackupExport = "be"` (`Fields: []string{"m"}`, `Right: settingsRightAdminBotAdmin`, `Validate: private(settingsBackupNonZeroMaskValid)`): decodes `m` into the selected module slice, calls `applyBackupExport`, sends the `.json` document to `p.query.From.Id` in private chat, and redraws the Backup page with a success notice (or rate-limit / failure notice).
  - `settingsActBackupImport = "bi"` (`Right: settingsRightOwnerBotAdmin`, `Validate: private(settingsBackupNoExtraFields)`): starts a private wait (`settingsInputSet` with `Page: settingsPageBackup, Part: settingsBackupImportPart ("i")`) and edits the menu message into the waiting prompt with a Cancel button (`a=bl&t=i`).
  - `settingsActBackupImportCancel = "bl"` (`Fields: []string{"t"}`, `Right: settingsRightAdmin`, `Validate: private(func(f map[string]string) bool { return f["t"] == settingsBackupImportPart })`): cancels the active import file wait via `settingsInputCancel` and redraws the Backup page.
  - `settingsActBackupImportYes = "bj"` (`Fields: []string{"k"}`, `Right: settingsRightOwnerBotAdmin`, `Validate: private(settingsBackupTokenFieldValid)`): peeks and verifies the one-time Redis import token (`fuku:settings:backup:import:<token>`), checks `AcquireImport(p.group.Id)` rate limit, atomically consumes the token via Lua `GET+DEL`, runs `applyBackupImport`, and redraws the Backup page with the import report (including `Skipped` and `LogChannelSkipped`).
  - `settingsActBackupResetAsk = "br"` (`Fields: []string{"m"}`, `Right: settingsRightOwnerBotAdmin`, `Validate: private(settingsBackupNonZeroMaskValid)`): decodes `m` into the selected module slice, stores a one-time Redis reset token (`fuku:settings:backup:reset:<token>`, TTL `settingsCardLifetime = 5m`, binding `{UserID, ChatID, Modules}`), and draws the two-tap Reset confirmation card (`a=bu&k=<token>` for Yes, `a=no&p=<pageKey>` for No).
  - `settingsActBackupResetYes = "bu"` (`Fields: []string{"k"}`, `Right: settingsRightOwnerBotAdmin`, `Validate: private(settingsBackupTokenFieldValid)`): peeks and verifies the one-time Redis reset token (`fuku:settings:backup:reset:<token>`), checks `AcquireReset(p.group.Id)` rate limit, atomically consumes the token via Lua `GET+DEL`, runs `applyBackupReset`, and redraws the Backup page with the reset summary.

**Cross-Replica Pending Import & Reset State (`P30-01`):**
- Copy the exact Redis Lua one-time token pattern from `fuku/modules/settings_copy.go` (lines 43-47, 75-80, 171-252):
```go
const (
	settingsBackupImportTokenPrefix = "fuku:settings:backup:import:"
	settingsBackupResetTokenPrefix  = "fuku:settings:backup:reset:"
	// Cap serialized pending import payload in Redis at 2 MiB (well above typical backup files while bounding Redis memory).
	maxSettingsBackupRedisBytes = 2 * 1024 * 1024
)

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
```
- Before consuming at Yes (`bj` / `bu`):
  1. Peek the token (`GET`). If missing (`redis.Nil`), answer alert `settings_card_expired` and redraw the Backup page with `settingsNoticeWarn(expired)`.
  2. Verify `rec.UserID == p.query.From.Id && rec.ChatID == p.group.Id` (if mismatched, answer `common_callback_invalid_request` WITHOUT deleting the real owner's token!).
  3. Verify fresh non-Staff-Group status on `p.group.Id` (failing closed on DB error).
  4. Note that `p.verify()` and `p.runPageAction()` already verified live `lockdownLiveMember`, `settingsGate`, live `GetChatWithContext`, and `settingsRightOwnerBotAdmin` (bot is admin + presser is live group creator)!
  5. Check `limiter.AcquireImport(p.group.Id)` (for `bj`) or `limiter.AcquireReset(p.group.Id)` (for `bu`). Wait: consume the one-time token via Lua `GET+DEL` (`settingsCopyConsumeScript`) first or check rate limit first?
     - If we consume the token via Lua `GET+DEL` first, a concurrent replay of the same Yes button is rejected as `settings_card_expired` and never double-spends the rate limiter or double-runs the import/reset; if the rate limiter then refuses (`!allowed`), the card is closed and the page redraws with the rate-limit warning notice (`backup_import_rate_limited` / `backup_reset_rate_limited`).

**Private Document Routing in Typed-Reply Door (`fuku/modules/settings_input_door.go` & `fuku/modules/settings_backup.go`):**
- Register in `init()` of `fuku/modules/settings_backup.go`:
```go
	settingsTypedPages[settingsPageBackup] = settingsTypedPage{
		Part:         settingsBackupImportPart,
		CancelCode:   settingsActBackupImportCancel,
		Right:        settingsRightOwnerBotAdmin,
		Typed:        settingsBackupImportTyped,
		WaitEndedKey: "settings_bkp_import_wait_ended",
	}
```
- In `settings_input_door.go`:
  - `settingsInputCandidate(msg)` (`settings_input_door.go:63-67`) already accepts private messages with `msg.Document != nil`.
  - When `r.wait.Page == settingsPageBackup && r.wait.Part == settingsBackupImportPart`, `r.run()` (`settings_input_door.go:226-233`) runs `r.authorize(settingsRightOwnerBotAdmin)` (live `lockdownLiveMember`, `settingsGate`, `IsBotAdmin` + live `botCheck` fallback, live creator check `settingsRightRefusalFor` -> `"chat_status_owner_button_error"`, live `GetChatWithContext`), then `r.refuseSlash()`, and then invokes `settingsBackupImportTyped(r)`.
  - In `settingsBackupImportTyped(r *settingsInputReply)`:
    1. Check fresh non-Staff-Group (`if settingsBackupHidden(&r.group) { r.end("settings_edit_refuse_access_lost"); return }`).
    2. Check `if r.msg.Document == nil { r.refuse(r.text("settings_bkp_import_need_file")); return }` — keeps the wait alive and replies with the refusal + Cancel button (`settingsInputNoteRefusal`).
    3. Call `bkp, errText := downloadAndParseBackupFile(r.b, r.msg.Document, r.tr)`. If `bkp == nil`, call `r.refuse(errText)` — keeps the wait alive so the owner can send a valid `.json` file!
    4. Determine `importModules`: if `r.msg.Caption != ""` has module args, parse them via `parseImportModules("import "+r.msg.Caption, bkp.Data)`, or use `bkp.Modules`.
    5. Claim the wait: `claim, claimed := r.claimWait(); if !claimed { return }`.
    6. Store the one-time Redis import token (`settingsBackupPutImportToken(ctx, settingsBackupImportRecord{UserID: r.user.Id, ChatID: r.wait.GroupID, Modules: importModules, Backup: bkp})`). If Redis storage fails (or exceeds `maxSettingsBackupRedisBytes`), release the wait (`r.releaseWait(claim)`) and `r.refuse(...)`.
    7. Finish the wait (`r.finishWait(claim)`), close the waiting prompt (`r.closePrompt()`), and send the two-tap Import confirmation card (`renderSettingsBackupImportConfirm(r.view(), bkp, importModules, token)`) as a new message in the private chat (`r.send(screen.Text, &screen.Keyboard, false)`).

---

### 4. Grid & Page Registry Test Updates Across Existing Test Files

Adding the 30th registered settings page (at `Order: 83`, right after `cp` at `Order: 82` and before `d` at `Order: 85`, with `Hidden: settingsBackupHidden` so it is hidden in Staff Groups) requires updating the exact page-list and grid-shape constants in these test files:

1. **`fuku/modules/settings_main_test.go`** (lines 35-42, 574-578):
   - Currently 29 registered pages (28 visible in a normal group, 26 in the 2-column grid + 2 owner-only full-width rows `p` and `a` + 1 nav row = 16 rows).
   - Adding the Backup page (e.g. key `"ba"` or `"bk"`, `Order: 83`) makes **27** grid cells in a normal group (`14` grid rows, with `y` Anonymous admins alone on the 14th grid row) + `2` owner-only rows (`p`, `a`) + `1` nav row = **17** rows (`settingsMainOwnerRows = 17`):
   ```go
   settingsMainShapeGroup   = "s|l / c|f / q|n / j|k / rs|b / i|w / g|r / e|x / h|m / av|cp / ba|d / t|u / z|o / y / p / a / close"
   settingsMainShapePrivate = "s|l / c|f / q|n / j|k / rs|b / i|w / g|r / e|x / h|m / av|cp / ba|d / t|u / z|o / y / p / a / groups|close"
   settingsMainOwnerRows    = 17
   ```
   - Add `"settings_page_backup"` to `settingsMainTitleKeys` (`settings_main_test.go:574-578`) right after `"settings_page_copy"` and before `"settings_page_del"`.

2. **`fuku/modules/settings_fwdblock_test.go`** (lines 348-354):
   - Because the Backup page is hidden in a Staff Group (`Hidden: settingsBackupHidden`), a Staff Group's Main keyboard is unchanged in shape, while `staffOwnerRows` relative to `settingsMainOwnerRows` becomes:
   ```go
   staffOwnerRows = settingsMainOwnerRows - 4
   ```

3. **`fuku/modules/settings_phase19_test.go`** (line 96):
   - Update `want` in `TestSettingsPhase19Codes` to include the Backup page key (`"ba"` or `"bk"`).

4. **`fuku/modules/settings_phase29_test.go`** (lines 45-55):
   - Update `allPages` in `TestPhase29Codes` (which asserts `len(settingsPages) == len(allPages)`) to include the new Backup page key (30 pages total).

5. **`fuku/modules/settings_phase18_test.go`** (lines 93-95, 171):
   - Check `TestSettingsPhase18InStaffGroup` (line 93) and `TestSettingsPhase18Codes` (line 171) so their page count and `want` slice match all 30 registered pages (in a Staff Group, 8 pages from `settingsMainTitleKeys` are hidden — `d, h, j, q, rs, av, cp` + Backup — and 1 Staff-Group-only page `v` is shown, so a Staff Group has `len(settingsMainTitleKeys) - 7` state lines).

6. **`fuku/i18n/settings_locale_test.go`** (lines 900, 905, 1137, 1230):
   - Add every new `settings_page_backup` / `settings_state_backup` / `settings_bkp_*` / `settings_btn_bkp_*` locale key and its `(maxEN, maxOther)` rune limit to `settingsLocaleLimits`.
   - Add any new alert keys to `settingsAlertKeys` (plain text, no HTML tags, `<= 200` runes in all 8 languages).
   - Add `{"💾", "settings_page_backup"}` to `pages` in `TestSettingsLabelLengths` (`<= 20` runes with icon + space in all 8 languages).
   - Add any two-column Backup page buttons to `editorButtons` (`<= 20` runes) or full-width buttons to `fullWidthLabels` (`<= 40` runes).

---

## Shared Patterns

### 1. Step-1 Private-Menu Validation (Zero I/O on Forged/Group Callbacks)
**Source:** `fuku/modules/settings_copy.go` (lines 106-133)
**Apply to:** `fuku/modules/settings_backup.go`
```go
	private := func(check func(map[string]string) bool) func(map[string]string) bool {
		return func(fields map[string]string) bool { return fields["c"] != "" && check(fields) }
	}
```
Every action registered by `settings_backup.go` must wrap its `Validate` function in `private(...)` so a group-menu callback (`u` instead of `c`) or any callback with extra/malformed keys fails in step 1 of `settingsPress.verify()` with `common_callback_invalid_request` and `0` `getChatMember` calls.

### 2. Atomic One-Time Redis Token Consumption via Lua `GET+DEL`
**Source:** `fuku/modules/settings_copy.go` (lines 75-80, 232-252)
**Apply to:** `fuku/modules/settings_backup.go` (Import Confirm Yes & Reset Confirm Yes)
```go
	settingsCopyConsumeScript = redis.NewScript(`
local v = redis.call("GET", KEYS[1])
if not v then return false end
redis.call("DEL", KEYS[1])
return v
`)
```
Always peek (`GET`) first to verify `rec.UserID == p.query.From.Id && rec.ChatID == p.group.Id` before running the atomic `GET+DEL` script, so a forged press from another user or chat never deletes the legitimate owner's pending confirmation token.

### 3. IDs-Only Logging & Staff Group Privacy
**Source:** `fuku/modules/settings_phase29_test.go` (lines 224-475)
**Apply to:** `fuku/modules/settings_backup.go`, `fuku/modules/backup.go`, `fuku/modules/settings_phase30_test.go`
- Never log chat titles, user names, backup JSON contents, custom message texts, file IDs, button URLs, or Redis confirmation tokens (`k`). Log only numeric `chatID`, `userID`, module counts, and error classes.
- Never show the Backup page in a Staff Group (`Hidden: settingsBackupHidden` using `staff.GetStaffGroupFresh`), and verify non-Staff-Group status fresh before executing Export, Import, or Reset.

### 4. Immutable MSG-11 Goldens First
**Source:** `fuku/modules/phase29_legacy_request_test.go`, `fuku/db/backup/punish_export_legacy_test.go`
**Apply to:** Plan 30-01 (`fuku/modules/backup_phase30_legacy_request_test.go`)
- Record characterization goldens for `/export`, `/import`, and `/reset` (plus pre-Phase-30 event message replies) on the unmodified codebase in Wave 1 before any production file is edited, and verify `git diff --exit-code` on all golden test files in every subsequent plan.

---

## No Analog Found

All Phase 30 files have direct, exact analogs in the repository.

---

## Metadata

**Analog search scope:** `fuku/db/backup/`, `fuku/db/eventmsg/`, `fuku/modules/`, `fuku/utils/ratelimit/`, `fuku/i18n/`, `locales/`
**Files scanned:** 42
**Pattern extraction date:** 2026-11-23
