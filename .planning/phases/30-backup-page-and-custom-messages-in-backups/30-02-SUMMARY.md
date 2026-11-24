---
phase: 30-backup-page-and-custom-messages-in-backups
plan: 02
subsystem: database
tags: [backup, eventmsg, custom-messages, msg-10, transactions, validation, i18n, race]

requires:
  - phase: 30-backup-page-and-custom-messages-in-backups
    provides: Standalone MSG-11 characterization golden suite TestLegacyPhase30Baseline (Plan 30-01, commit f73400f4)
  - phase: 29-plain-tone-and-copy-settings
    provides: eventmsg.ValidateMessage and eventmsg.ReplaceAllValidated shared editor validation rules
provides:
  - Transaction-safe eventmsg.ReplaceAllValidatedTx(tx, chatID, rows, opts, by) for caller-owned GORM transactions
  - BackupModuleMessages = "messages" and MessagesBackup in fuku/db/backup (20 exportable modules) with exportMessagesData, importMessages, and clearMessages
  - backup_module_messages locale key across all 8 languages (en, es, fr, hi, id, pt, ru, uk) while keeping backup_import_skipped_entries byte-identical for TestLegacyPhase30Baseline
  - Comprehensive MSG-10 test coverage in fuku/db/backup/event_messages_backup_test.go, roundtrip_test.go, backup_test.go, and fuku/db/eventmsg/validate_write_test.go
affects: [30-03, 30-04, 30-05]

tech-stack:
  added: []
  patterns:
    - "Caller-owned transaction writer: eventmsg.ReplaceAllValidatedTx executes inside backup.withBackupTx without nested transactions, returning CacheKey(chatID) for post-commit invalidation"
    - "Backup module validation parity: importMessages routes every candidate row through eventmsg.ValidateMessage via ReplaceAllValidatedTx and adds len(writeReport.Skipped) to ImportReport.Skipped"
    - "Old-export backward compatibility: exports omitting \"messages\" leave chat_event_messages and chat_event_message_options untouched; exports with \"messages\": {} clear to defaults"

key-files:
  created: []
  modified:
    - fuku/db/eventmsg/validate_write.go
    - fuku/db/eventmsg/validate_write_test.go
    - fuku/db/backup/types.go
    - fuku/db/backup/backup.go
    - fuku/db/backup/backup_test.go
    - fuku/db/backup/event_messages_backup_test.go
    - fuku/db/backup/roundtrip_test.go
    - locales/en.yml
    - locales/es.yml
    - locales/fr.yml
    - locales/hi.yml
    - locales/id.yml
    - locales/pt.yml
    - locales/ru.yml
    - locales/uk.yml

key-decisions:
  - "Extracted eventmsg.ReplaceAllValidatedTx(tx, chatID, rows, opts, by) without internal cache invalidation so backup.withBackupTx keeps multi-module rollback atomicity and invalidates eventmsg.CacheKey(chatID) only after commit (T-30-03)."
  - "Added BackupModuleMessages = \"messages\" as the 20th exportable backup module covering chat_event_messages and chat_event_message_options, cleared by ClearChatData / /reset (P30-02), while leaving tuning and staff tables excluded (P30-03)."
  - "Kept backup_import_skipped_entries byte-identical across all 8 locales so Wave 1 golden TestLegacyPhase30Baseline continues to pass without modification."

patterns-established:
  - "eventmsg.ReplaceAllValidatedTx: shared transaction-participating validated replacement used by both eventmsg.ReplaceAllValidated and fuku/db/backup import/clear."

requirements-completed: [MSG-10]

status: complete
plan_head_before: 5b78277960767b93a004661e2676168e1e383d76
plan_head_after: 0ac1d0e44e4fea93ee38977293f159d511e2f6ba
commits: 3

actuals:
  tokens: 11844
  tasks: 3
  commits: 3

duration: 20min
completed: 2026-11-23
---

# Phase 30 Plan 02: Transaction-Safe Event Message Writer and Custom Messages Backup Module Summary

**Extracted transaction-safe `eventmsg.ReplaceAllValidatedTx`, wired `BackupModuleMessages = "messages"` (`chat_event_messages` + `chat_event_message_options`) into `fuku/db/backup` export, validated import, and reset with `backup_module_messages` across all 8 locales, and verified full MSG-10 round-trip, editor validation, skipped counting, old-export compatibility, rollback atomicity, and tuning/staff exclusion**

## Performance

- **Duration:** 20 min
- **Started:** 2026-11-23T20:35:51Z
- **Completed:** 2026-11-23T20:55:57Z
- **Tasks:** 3
- **Files modified:** 15

## Accomplishments

- **Task 1 (`4a12da3e`):** Extracted `eventmsg.ReplaceAllValidatedTx(tx *gorm.DB, chatID int64, rows []Message, opts Options, by int64) (WriteReport, error)` in `fuku/db/eventmsg/validate_write.go`, delegated `ReplaceAllValidated` to it (invalidating `CacheKey(chatID)` only after transaction commit), and added `TestReplaceAllValidatedTx` in `fuku/db/eventmsg/validate_write_test.go` proving nil-tx error handling, outer-transaction rollback atomicity, and caller-owned cache invalidation.
- **Task 2 (`aa82fdd5`):** Added `BackupModuleMessages = "messages"` and `MessagesBackup` to `fuku/db/backup/types.go` and `AllExportableModules()` (20 modules total), wired `exportMessagesData`, `importMessages` (decoding via `decodeModuleData`, routing rows through `eventmsg.ReplaceAllValidatedTx`, accumulating `len(writeReport.Skipped)` into `report.Skipped`, and returning `eventmsg.CacheKey(chatID)` for post-commit invalidation), and `clearMessages` into `fuku/db/backup/backup.go`, and added `backup_module_messages` across all 8 locale files (`en`, `es`, `fr`, `hi`, `id`, `pt`, `ru`, `uk`) while keeping `backup_import_skipped_entries` unchanged for Wave 1 golden compatibility.
- **Task 3 (`0ac1d0e4`):** Extended `cleanupBackupChat` and `TestModuleValidation` in `fuku/db/backup/backup_test.go`, added custom group/PM messages and options seeding/assertions (plus stale-row replacement) to `TestAllModulesRoundTripEveryMeaningfulField` in `fuku/db/backup/roundtrip_test.go`, and replaced the pre-Phase-30 exclusion guard in `fuku/db/backup/event_messages_backup_test.go` with `TestEventMessagesInBackup` (export, JSON round-trip, cached + fresh post-import checks, multi-module rollback atomicity `T-30-03`, full & single-module reset with cache invalidation `P30-02`, and tuning/staff exclusion `P30-03`), `TestEventMessagesImportValidationAndSkipped` (1 valid + 7 editor-invalid rows -> `report.Skipped == 7`, `T-30-02`), and `TestOldExportWithoutMessagesAndOmittedFields` (legacy export preservation, `"messages": {}` clearing to defaults, and table-alias rejection).

## Task Commits

Each task was committed atomically:

1. **Task 1: Extract eventmsg.ReplaceAllValidatedTx and add unit tests** - `4a12da3e` (`refactor(30-02): extract transaction-safe eventmsg.ReplaceAllValidatedTx`)
2. **Task 2: Wire BackupModuleMessages into fuku/db/backup and add backup_module_messages across all 8 locales** - `aa82fdd5` (`feat(30-02): add BackupModuleMessages to fuku/db/backup and 8 locales`)
3. **Task 3: Update event_messages_backup_test.go, roundtrip_test.go, and backup_test.go for MSG-10** - `0ac1d0e4` (`test(30-02): cover MSG-10 custom messages backup round-trip, validation, and reset`)

## Files Created/Modified

- `fuku/db/eventmsg/validate_write.go` — Exported `ReplaceAllValidatedTx` and delegated `ReplaceAllValidated` to it.
- `fuku/db/eventmsg/validate_write_test.go` — Added `TestReplaceAllValidatedTx` (nil-tx error, outer-tx rollback atomicity, caller-owned cache invalidation).
- `fuku/db/backup/types.go` — Added `BackupModuleMessages = "messages"`, `MessagesBackup`, and 20-module `AllExportableModules()`.
- `fuku/db/backup/backup.go` — Added `exportMessagesData`, `importMessages`, and `clearMessages`, wired into `ExportModuleData`, `importListModule`, and `clearModuleData`.
- `fuku/db/backup/backup_test.go` — Extended `TestModuleValidation` (20 modules, `"messages"`) and `cleanupBackupChat` (`ChatEventMessage`, `ChatEventMessageOptions`, `eventmsg.CacheKey`).
- `fuku/db/backup/roundtrip_test.go` — Seeded and verified custom group/PM event messages, options, and stale destination replacement in `TestAllModulesRoundTripEveryMeaningfulField`.
- `fuku/db/backup/event_messages_backup_test.go` — Replaced pre-Phase-30 exclusion test with `TestEventMessagesInBackup`, `TestEventMessagesImportValidationAndSkipped`, and `TestOldExportWithoutMessagesAndOmittedFields`.
- `locales/{en,es,fr,hi,id,pt,ru,uk}.yml` — Added `backup_module_messages` right after `backup_module_warns`.

## Decisions Made

- Kept `backup_import_skipped_entries` byte-identical across all 8 locales per Task 2 instructions so `TestLegacyPhase30Baseline` (recorded in Plan 30-01) passes untouched; the Go doc comment on `ImportReport` documents that `Skipped` now also includes invalid custom event messages.
- Preserved `gofmt`-canonical column alignment in `fuku/db/backup/types.go` (`BackupModuleMessages    = "messages"`).
- Added `//go:build testtools` to `fuku/db/backup/event_messages_backup_test.go` (matching `advthresholds_backup_test.go`, `modtuning_backup_test.go`, `logchannels_old_export_test.go`, `punish_export_legacy_test.go`, `punish_import_test.go`) so tests use `utilscache.SetupTestMemoryMarshaler(t)` to verify real cache invalidation of `eventmsg.CacheKey(chatID)` with zero localhost sockets.

## Deviations from Plan

None - plan executed as written (note: `must_haves` summary line mentioned updating `backup_import_skipped_entries`, while Task 2 `<behavior>` and `<action>` explicitly directed leaving `backup_import_skipped_entries` unchanged to preserve Wave 1 golden `TestLegacyPhase30Baseline`; followed Task 2's explicit directive).

## Verification Results

- `TZ=UTC go test -tags testtools -race -count=1 ./fuku/db/eventmsg ./fuku/db/backup ./fuku/db/adminindex ./fuku/db/staff ./fuku/i18n` — PASSED (0 failures, 0 skips)
- `TZ=UTC go test -tags testtools -race -count=1 -run '^(TestEventMsgValidateWriteMatchesSettingsPage|TestLegacyPhase30Baseline|TestLegacyPhase29Baseline|TestEventEngineCallers|TestPMCopyCallers)$' ./fuku/modules` — PASSED (0 failures, 0 skips)
- `go vet -tags testtools ./fuku/db/eventmsg ./fuku/db/backup` — PASSED
- `CGO_ENABLED=0 go build ./...` — PASSED
- `make check-translations` — PASSED (all 8 locales complete)
- `git diff --exit-code 1da5af6f -- go.mod go.sum` — PASSED
- `git diff --exit-code f73400f4 -- fuku/modules/phase30_legacy_request_test.go` — PASSED

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Requirement `MSG-10` is complete at the database and backup repository layer.
- Ready for Wave 3 (Plan 30-03: shared export/import/reset apply functions in `fuku/modules/backup.go` and the private-only Backup settings page in `fuku/modules/settings_backup.go`).

## Self-Check: PASSED

- All 15 modified files and `30-02-SUMMARY.md`: FOUND
- Task commits `4a12da3e`, `aa82fdd5`, `0ac1d0e4`: FOUND and ancestors of HEAD (`commits: 3`)
- All unit, round-trip, MSG-10, golden, translation, vet, and `CGO_ENABLED=0 go build ./...` checks: PASSED
