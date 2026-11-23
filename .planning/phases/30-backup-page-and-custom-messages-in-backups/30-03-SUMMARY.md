---
phase: 30-backup-page-and-custom-messages-in-backups
plan: 03
subsystem: backup
tags: [backup, export, import, reset, redis, lua, tokens, race, bkp-01, bkp-02, bkp-03]

requires:
  - phase: 30-backup-page-and-custom-messages-in-backups
    provides: Custom messages backup module BackupModuleMessages (Plan 30-02, commit 0ac1d0e4) and Wave 1 golden TestLegacyPhase30Baseline (Plan 30-01, commit f73400f4)
  - phase: 29-plain-tone-and-copy-settings
    provides: settingsCopyNewToken, settingsCopyTokenValid, settingsCopyDefaultRedis, and settingsCopyConsumeScript Lua GET+DEL pattern in fuku/modules/settings_copy.go
provides:
  - Shared apply helpers in fuku/modules/backup.go (applyBackupExport, downloadAndParseBackupFile, applyBackupImport, formatBackupImportSuccess, applyBackupReset, formatBackupResetSuccess) called by /export, /import, /reset and ready for the private Backup settings page
  - Cross-replica Redis pending confirmation token store (fuku:settings:backup:import: and fuku:settings:backup:reset:) with 16-char base64url 96-bit tokens, SET NX, settingsCardLifetime TTL, 2 MiB serialized payload cap (maxSettingsBackupRedisBytes, P30-01), json.Number int64 precision preservation, peek-before-consume binding, atomic Lua GET+DEL consumption, and fail-closed nil-Redis handling
  - Unit tests TestSharedBackupApplyHelpers and TestSettingsBackupRedisTokens in fuku/modules/backup_test.go using backupRoundTripFunc and in-process phase28InProcessMiniredis with zero localhost TCP sockets
affects: [30-04, 30-05]

tech-stack:
  added: []
  patterns:
    - "Shared command/page apply seam: /export, /import, and /reset delegate core execution to applyBackupExport, downloadAndParseBackupFile, applyBackupImport/formatBackupImportSuccess, and applyBackupReset/formatBackupResetSuccess while keeping their exact command replies, permission checks, rate limits, and confirm cards"
    - "Cross-replica Redis one-time backup tokens (P30-01): 96-bit crypto/rand base64url token stored via SET NX under fuku:settings:backup:{import,reset}:<token> with settingsCardLifetime TTL, 2 MiB serialized payload cap, peek-and-bind verification before atomic Lua GET+DEL via settingsCopyConsumeScript, and json.Decoder.UseNumber() on import record decode so 64-bit integer IDs in BackupFormat.Data keep exact precision"

key-files:
  created: []
  modified:
    - fuku/modules/backup.go
    - fuku/modules/backup_test.go

key-decisions:
  - "Preserved 64-bit integer precision across Redis pending-import round-trips by decoding settingsBackupImportRecord with json.NewDecoder(...).UseNumber() (matching backup.BackupFormatFromJSON) and rejecting trailing JSON data."
  - "Added backupExportChatData = backup.ExportChatData test seam alongside settingsBackupRedisClient = settingsCopyDefaultRedis so TestSharedBackupApplyHelpers covers backupExportNoModules and backupExportFailed deterministically without altering production behavior."
  - "Bounded Backup Redis token operations with settingsReadTimeout and validated token format via settingsCopyTokenValid after the fail-closed nil-Redis check so malformed tokens incur zero Redis round-trips while unavailable Redis always returns errSettingsCopyNoRedis."
  - "Left BKP-01, BKP-02, and BKP-03 unchecked in REQUIREMENTS.md until Plan 30-04 delivers the private Backup settings page UI that wires onto these shared helpers."

patterns-established:
  - "Shared backup apply seams (applyBackupExport, downloadAndParseBackupFile, applyBackupImport, formatBackupImportSuccess, applyBackupReset, formatBackupResetSuccess) ensure 100% behavioral parity between /export, /import, /reset commands and the Plan 30-04 Backup settings page."
  - "Cross-replica Redis pending import/reset tokens (settingsBackupPut/Peek/Consume{Import,Reset}Token) decouple two-tap Backup page confirmations from single-replica in-memory maps."

requirements-completed: []

status: complete
plan_head_before: 2a7202d5de1e7cf03f8eea65e4490644402ad11a
plan_head_after: 0cb8ba241e148b691cdee67ba9c1ac6acbef6d77
commits: 2

actuals:
  tokens: 11006
  tasks: 2
  commits: 2

duration: 9min
completed: 2026-11-23
---

# Phase 30 Plan 03: Shared Backup Apply Helpers and Cross-Replica Redis Token Store Summary

**Extracted shared `applyBackupExport`, `downloadAndParseBackupFile`, `applyBackupImport`/`formatBackupImportSuccess`, and `applyBackupReset`/`formatBackupResetSuccess` helpers in `fuku/modules/backup.go` with zero behavioral change to `/export`, `/import`, and `/reset`, added cross-replica Redis pending Import/Reset one-time token storage (`fuku:settings:backup:import:` and `fuku:settings:backup:reset:`) with 2 MiB size cap and atomic Lua `GET+DEL` consumption (`P30-01`), and verified both seams under `-race` with zero localhost TCP listeners**

## Performance

- **Duration:** 9 min
- **Started:** 2026-11-23T20:58:50Z
- **Completed:** 2026-11-23T21:08:07Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- **Task 1 (`dd4c5f25`):** Extracted `applyBackupExport`, `downloadAndParseBackupFile`, `applyBackupImport`, `formatBackupImportSuccess`, `applyBackupReset`, and `formatBackupResetSuccess` in `fuku/modules/backup.go`, delegated `exportHandler`, `importHandler`, `handleConfirmImport`, and `handleConfirmReset` to them while preserving byte-identical replies, permission checks, rate limits, and confirm cards (`TestLegacyPhase30Baseline`), and implemented cross-replica Redis token helpers (`settingsBackupPutImportToken`, `settingsBackupPeekImportToken`, `settingsBackupConsumeImportToken`, `settingsBackupPutResetToken`, `settingsBackupPeekResetToken`, `settingsBackupConsumeResetToken`) using `settingsCopyNewToken`, `settingsCopyTokenValid`, `SET NX`, `settingsCardLifetime` TTL, 2 MiB `maxSettingsBackupRedisBytes` cap (`errSettingsBackupPayloadTooLarge`, T-30-05), `json.Decoder.UseNumber()`, atomic Lua `GET+DEL` via `settingsCopyConsumeScript` (T-30-06), and fail-closed `errSettingsCopyNoRedis` when Redis is unavailable.
- **Task 2 (`0cb8ba24`):** Added `TestSharedBackupApplyHelpers` (covering `applyBackupExport` delivery to `userID`, rate-limit refusal, 403 cooldown release, send failure, empty modules, and DB failure; `downloadAndParseBackupFile` via `backupRoundTripFunc` for valid JSON, non-JSON, nil doc, corrupted JSON, and incompatible version; `applyBackupImport` + `formatBackupImportSuccess` with custom `"messages"` skipping an invalid `javascript:` button; and `applyBackupReset` + `formatBackupResetSuccess` clearing `"messages"` and `"rules"` while preserving unselected `"notes"`) and `TestSettingsBackupRedisTokens` using `phase28InProcessMiniredis(t)` (covering import/reset token put, peek-and-bind without deletion, 8-goroutine concurrent atomic Lua `GET+DEL` consumption and replay refusal, `json.Number` 64-bit precision, TTL expiry via `FastForward`, 2 MiB payload cap rejection with 0 keys written, and fail-closed nil-Redis handling across all 6 token operations).

## Task Commits

Each task was committed atomically:

1. **Task 1: Extract shared backup apply helpers and add cross-replica Redis pending token store** - `dd4c5f25` (`feat(30-03): extract shared backup apply helpers and Redis pending token store`)
2. **Task 2: Add unit tests for shared apply helpers and cross-replica Redis tokens** - `0cb8ba24` (`test(30-03): cover shared backup apply helpers and cross-replica Redis tokens`)

## Files Created/Modified

- `fuku/modules/backup.go` — Extracted shared export/download-parse/import/reset apply and formatting helpers, routed `/export`, `/import`, and `/reset` through them, and added cross-replica Redis pending import/reset token helpers (`fuku:settings:backup:import:`, `fuku:settings:backup:reset:`, 2 MiB cap, atomic Lua `GET+DEL`).
- `fuku/modules/backup_test.go` — Added `TestSharedBackupApplyHelpers` and `TestSettingsBackupRedisTokens` using `backupRoundTripFunc` and `phase28InProcessMiniredis(t)` with full DB/cache/rate-limiter cleanup.

## Decisions Made

- Used `json.NewDecoder(bytes.NewReader(raw))` with `UseNumber()` (plus trailing-data check) in `unmarshalSettingsBackupImportRecord` so 64-bit integer IDs inside `BackupFormat.Data` round-trip through Redis without `float64` precision loss (matching `backup.BackupFormatFromJSON`).
- Added a minimal production-default test seam `backupExportChatData = backup.ExportChatData` so `TestSharedBackupApplyHelpers` deterministically verifies `backupExportNoModules` and `backupExportFailed` outcomes without modifying production export behavior.
- Kept all `backup.go` log lines strictly numeric (`chatID` and module counts only; never chat titles, user names, backup contents, or confirmation tokens).
- Left `BKP-01`, `BKP-02`, and `BKP-03` pending in `.planning/REQUIREMENTS.md` until Plan 30-04 implements the private Backup settings page UI on top of these shared seams.

## Deviations from Plan

None - plan executed as written.

## Verification Results

- `grep -c 'func applyBackupExport(' fuku/modules/backup.go` -> `1`
- `grep -c 'func downloadAndParseBackupFile(' fuku/modules/backup.go` -> `1`
- `grep -c 'func applyBackupImport(' fuku/modules/backup.go` -> `1`
- `grep -c 'func applyBackupReset(' fuku/modules/backup.go` -> `1`
- `grep -c 'fuku:settings:backup:import:' fuku/modules/backup.go` -> `1`
- `grep -c 'fuku:settings:backup:reset:' fuku/modules/backup.go` -> `1`
- `grep -c 'func TestSharedBackupApplyHelpers(' fuku/modules/backup_test.go` -> `1`
- `grep -c 'func TestSettingsBackupRedisTokens(' fuku/modules/backup_test.go` -> `1`
- `TZ=UTC go test -tags testtools -race -count=1 -run '^(TestSharedBackupApplyHelpers|TestSettingsBackupRedisTokens|TestExportGoesToThePrivateChat|TestLegacyPhase30Baseline)$' -v ./fuku/modules` — PASSED (0 failures, 0 skips)
- `TZ=UTC go test -tags testtools -race -count=1 -run '^(TestBackup|TestLoadBackup|TestBuild|TestParse|TestDownload|TestImport|TestExport|TestValidateImport|TestReset|TestPending|TestSharedBackupApplyHelpers|TestSettingsBackupRedisTokens|TestLegacyPhase30Baseline|TestEventEngineCallers|TestPMCopyCallers|TestNoRawNewCommand|TestCommandRepliesStayInsideTheHandler)$' ./fuku/modules` — PASSED (0 failures, 0 skips)
- `go vet -tags testtools ./fuku/modules` — PASSED
- `CGO_ENABLED=0 go build ./...` — PASSED
- `git diff --exit-code 1da5af6f -- go.mod go.sum` — PASSED
- `git diff --exit-code f73400f4 -- fuku/modules/phase30_legacy_request_test.go` — PASSED

## Issues Encountered

- `settingsCopyConsumeScript.Run(...).Bytes()` is undefined on `*redis.Cmd` in `go-redis/v9`; switched to `.Text()` converted to `[]byte(raw)` (matching `fuku/modules/settings_copy.go`) before committing Task 1.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Shared backup apply helpers and cross-replica Redis pending import/reset token store are complete and verified under `-race`.
- Ready for Wave 4 (Plan 30-04: Backup settings page locales across 8 languages, private-only `ba` page in `fuku/modules/settings_backup.go`, typed-reply document wait hook, grid/registry test updates, and `fuku/modules/settings_backup_test.go`).

## Self-Check: PASSED

- Modified files `fuku/modules/backup.go`, `fuku/modules/backup_test.go`, and `30-03-SUMMARY.md`: FOUND
- Task commits `dd4c5f25` and `0cb8ba24`: FOUND and ancestors of HEAD (`commits: 2`)
- All acceptance criteria, `-race` unit/golden/caller tests, `go vet`, and `CGO_ENABLED=0 go build ./...`: PASSED
