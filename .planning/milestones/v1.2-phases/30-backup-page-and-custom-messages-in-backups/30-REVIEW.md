---
phase: 30-backup-page-and-custom-messages-in-backups
reviewed: 2026-11-24T08:50:00Z
depth: standard
files_reviewed: 29
files_reviewed_list:
  - AGENTS.md
  - fuku/db/backup/backup.go
  - fuku/db/backup/backup_test.go
  - fuku/db/backup/event_messages_backup_test.go
  - fuku/db/backup/roundtrip_test.go
  - fuku/db/backup/types.go
  - fuku/db/eventmsg/validate_write.go
  - fuku/db/eventmsg/validate_write_test.go
  - fuku/i18n/settings_locale_test.go
  - fuku/modules/backup.go
  - fuku/modules/backup_test.go
  - fuku/modules/phase30_e2e_test.go
  - fuku/modules/phase30_legacy_request_test.go
  - fuku/modules/settings_backup.go
  - fuku/modules/settings_backup_test.go
  - fuku/modules/settings_fwdblock_test.go
  - fuku/modules/settings_main_test.go
  - fuku/modules/settings_phase18_test.go
  - fuku/modules/settings_phase19_test.go
  - fuku/modules/settings_phase29_test.go
  - fuku/modules/settings_phase30_test.go
  - locales/en.yml
  - locales/es.yml
  - locales/fr.yml
  - locales/hi.yml
  - locales/id.yml
  - locales/pt.yml
  - locales/ru.yml
  - locales/uk.yml
findings:
  critical: 0
  warning: 3
  info: 1
  total: 4
  necessity: 0
status: issues_found
---

# Phase 30: Code Review Report

**Reviewed:** 2026-11-24T08:50:00Z
**Depth:** standard
**Files Reviewed:** 29
**Status:** issues_found

## Summary

Reviewed all 29 files in the Phase 30 scope (`25a4dca7..HEAD`), including full reads of `fuku/db/eventmsg/validate_write.go`, `fuku/db/backup/types.go`, `fuku/db/backup/backup.go`, `fuku/modules/backup.go`, and `fuku/modules/settings_backup.go`, plus the 8 locale files, `AGENTS.md`, and the Phase 30 test suites.

The core transaction-safe event message replacement (`eventmsg.ReplaceAllValidatedTx`), `BackupModuleMessages` export/import/reset wiring, cross-replica Redis one-time token store (96-bit `SET NX` tokens, `json.Decoder.UseNumber()`, peek-before-consume binding, atomic Lua `GET+DEL`, fail-closed nil-Redis handling), Staff Group hiding/privacy, and test cleanup hygiene are solid. No critical security or data-loss defects were found.

Three warnings were identified around untrusted backup JSON handling and user-facing size-limit messaging:
1. `parseImportModules` validates caption/argument module names only against `bkp.Data` keys rather than `backup.IsValidModule`, allowing extra keys in `bkp.Data` to pass into the HTML confirmation card unescaped and to fail only after burning the import cooldown.
2. The private Backup import wait prompt and size-limit error tell users the maximum file size is 10 MB, whereas `settingsBackupPutImportToken` rejects serialized payloads larger than 2 MiB (`maxSettingsBackupRedisBytes`).
3. `BackupFormat.Validate()` does not reject or deduplicate repeated entries in `bkp.Modules`, which can cause a crafted JSON file to exceed Telegram's 4096-character limit on the confirmation card or run duplicate module imports inside one transaction.

## Narrative Findings (AI reviewer)

## Warnings

### WR-01: `parseImportModules` accepts unknown `bkp.Data` keys without checking `backup.IsValidModule`

**File:** `fuku/modules/backup.go:599-610`
**Issue:** `BackupFormat.Validate()` (`fuku/db/backup/types.go:64-71`) only validates entries listed in `b.Modules`; arbitrary extra keys in `b.Data` are permitted. `parseImportModules` (called by `settingsBackupImportTyped` at `fuku/modules/settings_backup.go:318` and `importHandler` at `fuku/modules/backup.go:655`) checks only `_, ok := backupData[module]` without calling `backup.IsValidModule(module)`. If a backup JSON contains an extra key in `data` (such as `<b>evil</b>`, `<broken`, or `"unknown"`) and the caller names it in the document caption or `/import` arguments:
1. The raw key is interpolated unescaped into the HTML confirmation card via `buildModuleList(modules)` (`fuku/modules/backup.go:989-994`, rendered with `ParseMode: "HTML"`), allowing HTML formatting injection or causing Telegram `400 Bad Request: can't parse entities` after `settingsBackupImportTyped` has already closed the wait prompt.
2. For non-HTML unknown keys, the confirmation card is shown and tapping Confirm consumes the one-time Redis token and burns the chat's `AcquireImport` rate-limit cooldown before `importChatData` fails with `unknown module`.

**Fix:**
```go
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
```

### WR-02: Private Backup import advertises a 10 MB limit but rejects payloads over 2 MiB

**File:** `fuku/modules/settings_backup.go:343-345`
**Issue:** `settings_bkp_import_wait_rules` across all 8 locales (e.g. `locales/en.yml:2545`) instructs the owner to `"Send a <code>.json</code> document up to 10 MB."`, and `downloadBackupFile` (`fuku/modules/backup.go:98,551`) downloads files up to `maxBackupFileSize` (10 MiB). However, `settingsBackupPutToken` (`fuku/modules/backup.go:112,252-254`) rejects any serialized pending import record larger than `maxSettingsBackupRedisBytes` (2 MiB) with `errSettingsBackupPayloadTooLarge`, which `settingsBackupImportTyped` maps to `backup_import_file_too_large` (`"❌ Backup file too large. Maximum size is 10MB."` in `locales/en.yml:1237`). Any owner uploading a valid 2–10 MB backup via the private Backup page is told the file exceeds 10 MB despite being well under 10 MB.

**Fix:** Align the Backup settings page prompt and error text across all 8 locales with the actual 2 MiB pending-import limit (for example, state 2 MB in `settings_bkp_import_wait_rules` and use a dedicated `settings_bkp_import_file_too_large` key for `errSettingsBackupPayloadTooLarge` that states the 2 MB limit—or directs owners with 2–10 MB backups to `/import`).

### WR-03: Duplicate entries in `BackupFormat.Modules` are not rejected or deduplicated

**File:** `fuku/db/backup/types.go:64-71`
**Issue:** `BackupFormat.Validate()` checks that each element of `b.Modules` is valid and present in `b.Data`, but does not check for duplicates. When no caption/module filter is supplied, `settingsBackupImportTyped` (`fuku/modules/settings_backup.go:324-326`) uses `bkp.Modules` directly. A small crafted JSON file repeating a valid module name hundreds of times (e.g. `"rules"` repeated 500 times) passes `Validate()`, causes `renderSettingsBackupImportConfirm` (`buildModuleList(importModules)`) to exceed Telegram's 4096-character limit so `r.send(...)` fails after `r.finishWait(claim)` and `r.closePrompt()` have already closed the wait, or with fewer duplicates executes the same module import repeatedly in `importChatData` (`fuku/db/backup/backup.go:164`) and inflates `len(modules)` and `report.Skipped`.

**Fix:** Reject duplicate module entries in `BackupFormat.Validate()`:
```go
	seen := make(map[string]struct{}, len(b.Modules))
	for _, module := range b.Modules {
		if !IsValidModule(module) {
			return fmt.Errorf("unknown module: %s", module)
		}
		if _, dup := seen[module]; dup {
			return fmt.Errorf("duplicate module: %s", module)
		}
		seen[module] = struct{}{}
		if _, ok := b.Data[module]; !ok {
			return fmt.Errorf("missing data for module: %s", module)
		}
	}
```

## Info

### IN-01: Hardcoded 20-bit mask constant is decoupled from `backup.AllExportableModules()`

**File:** `fuku/modules/settings_backup.go:45`
**Issue:** `settingsBackupAllMask uint32 = (1 << 20) - 1` and the `len(raw) > 5` check in `settingsBackupParseMask` (`fuku/modules/settings_backup.go:114`) hardcode the 20-module count independently of `backup.AllExportableModules()`. If a 21st exportable module is added in the future, `settingsBackupFormatMask(mask ^ bit)` will mask off bit 20 with `& settingsBackupAllMask`, silently preventing that module from being selected on page `ba`.
**Fix:** Add an explicit assertion in `TestPhase30Codes` that `settingsBackupAllMask == (uint32(1)<<len(backup.AllExportableModules()))-1` (or compute the mask and max hex digit count from `len(backup.AllExportableModules())`).

---

_Reviewed: 2026-11-24T08:50:00Z_
_Reviewer: the agent (gsd-code-reviewer)_
_Depth: standard_
