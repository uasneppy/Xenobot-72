---
phase: 30-backup-page-and-custom-messages-in-backups
fixed_at: 2026-11-24T09:02:20Z
review_path: /Users/denyspetrenko/Xenobot-72/.planning/phases/30-backup-page-and-custom-messages-in-backups/30-REVIEW.md
fix_scope: all
iteration: 1
findings_in_scope: 4
fixed: 4
skipped: 0
status: all_fixed
---

# Phase 30: Code Review Fix Report

**Fixed at:** 2026-11-24T09:02:20Z
**Source review:** `/Users/denyspetrenko/Xenobot-72/.planning/phases/30-backup-page-and-custom-messages-in-backups/30-REVIEW.md`
**Iteration:** 1
**Verification environment:** main checkout (`workflow.use_worktrees: false`)

**Summary:**
- Findings in scope: 4
- Fixed: 4
- Skipped: 0

## Fixed Issues

### WR-01: `parseImportModules` accepts unknown `bkp.Data` keys without checking `backup.IsValidModule`

**Files modified:** `fuku/modules/backup.go`, `fuku/modules/backup_test.go`
**Commit:** `87bb2d60`
**Status:** fixed
**Applied fix:** Updated `parseImportModules` in `fuku/modules/backup.go` to check `backup.IsValidModule(module)` before checking `backupData[module]`, preventing arbitrary or HTML-containing extra keys in `bkp.Data` from reaching the HTML confirmation card or consuming cooldowns. Added a regression subtest in `TestParseImportModules` (`fuku/modules/backup_test.go`) covering unknown and HTML-like extra keys in `backupData`.

### WR-02: Private Backup import advertises a 10 MB limit but rejects payloads over 2 MiB

**Files modified:** `locales/en.yml`, `locales/es.yml`, `locales/fr.yml`, `locales/hi.yml`, `locales/id.yml`, `locales/pt.yml`, `locales/ru.yml`, `locales/uk.yml`, `fuku/i18n/settings_locale_test.go`, `fuku/modules/settings_backup.go`, `fuku/modules/settings_backup_test.go`
**Commit:** `9236f785`
**Status:** fixed
**Applied fix:** Updated `settings_bkp_import_wait_rules` across all 8 locales to state the 2 MB limit (matching `maxSettingsBackupRedisBytes`), added dedicated `settings_bkp_import_file_too_large` across all 8 locales and `settingsLocaleLimits` in `fuku/i18n/settings_locale_test.go`, wired `errSettingsBackupPayloadTooLarge` in `fuku/modules/settings_backup.go` to `settings_bkp_import_file_too_large`, and added an oversized-payload refusal check in `TestSettingsBackupPage` (`fuku/modules/settings_backup_test.go`).

### WR-03: Duplicate entries in `BackupFormat.Modules` are not rejected or deduplicated

**Files modified:** `fuku/db/backup/types.go`, `fuku/db/backup/backup_test.go`
**Commit:** `baf37f40`
**Status:** fixed
**Applied fix:** Added a `seen` map check in `BackupFormat.Validate()` (`fuku/db/backup/types.go`) to return an error on duplicate entries in `b.Modules`, and added a `"duplicate modules"` validation test case in `TestBackupTypes` (`fuku/db/backup/backup_test.go`).

### IN-01: Hardcoded 20-bit mask constant is decoupled from `backup.AllExportableModules()`

**Files modified:** `fuku/modules/settings_backup.go`, `fuku/modules/settings_phase30_test.go`
**Commit:** `dc314982`
**Status:** fixed
**Applied fix:** Replaced the hardcoded `(1 << 20) - 1` mask and `len(raw) > 5` hex bound in `fuku/modules/settings_backup.go` with `settingsBackupAllMask` and `settingsBackupMaskMaxHex` derived via `computeSettingsBackupAllMask(len(backup.AllExportableModules()))`, and added explicit assertions for both in `TestPhase30Codes` (`fuku/modules/settings_phase30_test.go`).

## Verification

All verification commands ran in the **main checkout** (`workflow.use_worktrees: false`) after the final commit (`dc314982`):

- `make check-translations` — passed (1068 keys present across all 8 locales)
- `make check-docs` — passed (no drift)
- `go vet -tags testtools ./...` — passed
- `CGO_ENABLED=0 go build ./...` — passed
- `make test-shard-check` — passed
- `TZ=UTC go test -tags testtools -race -count=1 ./fuku/db/backup ./fuku/db/eventmsg ./fuku/i18n` — passed (`backup` 1.527s, `eventmsg` 1.408s, `i18n` 31.160s)
- `TZ=UTC go test -tags testtools -race -count=1 -run '^(TestParseImportModules|TestSettingsDeletionMemberCodesUnique|TestLegacyPhase30Baseline|TestSharedBackupApplyHelpers|TestSettingsBackupRedisTokens|TestSettingsBackupPage|TestPhase30Codes|TestPhase30ReadsCreateNothing|TestPhase30StaffPrivacyAndLogs|TestPhase30EndToEnd|TestEventMsgValidateWriteMatchesSettingsPage|TestEventEngineCallers|TestPMCopyCallers|TestSettingsPhase18Codes|TestSettingsPhase19Codes|TestPhase28Codes|TestPhase29Codes)$' ./fuku/modules` — passed (4.070s)

---

_Fixed: 2026-11-24T09:02:20Z_
_Fixer: the agent (gsd-code-fixer)_
_Iteration: 1_
