---
phase: 29-plain-tone-and-copy-settings
fixed_at: 2026-11-24T09:31:28Z
review_path: /Users/denyspetrenko/Xenobot-72/.planning/phases/29-plain-tone-and-copy-settings/29-REVIEW.md
fix_scope: all
iteration: 1
findings_in_scope: 5
fixed: 5
skipped: 0
status: all_fixed
---

# Phase 29: Code Review Fix Report

**Fixed at:** 2026-11-24T09:31:28Z
**Source review:** `.planning/phases/29-plain-tone-and-copy-settings/29-REVIEW.md`
**Iteration:** 1
**Verification environment:** main checkout (`workflow.use_worktrees: false`)

**Summary:**
- Findings in scope: 5
- Fixed: 5
- Skipped: 0

## Fixed Issues

### WR-01: `escalation.ReplaceLadder` accepts interior empty `Step{}` gaps that are silently dropped on read

**Files modified:** `fuku/db/escalation/repository.go`, `fuku/db/escalation/repository_test.go`
**Commit:** `b9a5bd45`
**Applied fix:** Added `seenEmpty` tracking in `ReplaceLadder` validation to return `ErrInvalidValue` whenever a non-empty step follows an empty `Step{}`, preventing silent truncation on read. Added `"leading empty step"` and `"interior empty step"` negative test cases in `TestReplaceLadder`.

### WR-02: `applyCopySettings` performs 9 separate DB writes without an enclosing transaction, risking partial target state on mid-way failure

**Files modified:** `fuku/db/warns/repository.go`, `fuku/db/antiflood/repository.go`, `fuku/db/blacklists/repository.go`, `fuku/db/escalation/repository.go`, `fuku/db/modtuning/repository.go`, `fuku/db/advthresholds/repository.go`, `fuku/modules/copy_settings.go`, `fuku/modules/copy_settings_test.go`
**Commit:** `ea2f443a`
**Applied fix:** Added `*Tx(tx *gorm.DB, ...)` variants (`warns.SetWarnModeTx`, `warns.SetWarnExpiryTx`, `antiflood.SetFloodActionPreserveOffTx`, `blacklists.SetBlacklistActionTimedTx`, `escalation.ReplaceLadderTx`, `modtuning.ReplaceDefaultsTx`, `modtuning.ReplaceShortcutsTx`, `advthresholds.ReplaceAllTx`) matching `eventmsg.ReplaceAllValidatedTx`. Wrapped all 9 target table writes in `applyCopySettings` inside a single `db.DB.WithContext(ctx).Transaction(...)` via `writeCopySettingsTx` with no nested transactions, and moved all 8 target cache invalidations to `dropCopySettingsTargetCaches` after commit. Added `atomic_rollback_on_late_step_failure` subtest in `TestCopySettingsMirrorAndExclusions`.

### WR-03: `settingsCopyLoadCandidates` runs unbounded per-group Staff Group DB queries outside `readCtx`

**Files modified:** `fuku/db/staff/repository.go`, `fuku/modules/settings_copy.go`
**Commit:** `94c8b2e7`
**Applied fix:** Added `staff.StaffGroupChatIDsFresh(ctx, chatIDs)` to fetch matching Staff Group IDs in a single `WHERE chat_id IN ?` query bounded by `ctx`. Updated `settingsCopyLoadCandidates` to filter candidates via `settingsCopyStaffBatchLookup(readCtx, candidateIDs)` while preserving `settingsCopyStaffLookup` and `settingsCopyDefaultStaffLookup` for single-chat checks and `settings_backup.go`.

### IN-01: Misleading log message in `eventMsgOptionsReadBack` for `applyEventPlainTone`

**Files modified:** `fuku/modules/event_message_apply.go`
**Commit:** `46865276`
**Applied fix:** Updated `eventMsgOptionsReadBack` log templates and doc comment from `"private copy %s option"` / `"private copy options"` to `"event message %s option"` / `"event message options"`.

### IN-02: `eventPlainRender` mutates caller-provided `params` map in place

**Files modified:** `fuku/modules/event_message_plain.go`
**Commit:** `1053414b`
**Applied fix:** Cloned non-nil `params` with `maps.Clone(params)` in `eventPlainRender` before inserting `mentionKey`.

## Verification

Ran in main checkout (`workflow.use_worktrees: false`, `TZ=UTC`):
- `make check-translations` — passed (1068 keys across all 8 locales)
- `make check-docs` — passed (no drift)
- `go vet -tags testtools ./...` — passed
- `CGO_ENABLED=0 go build ./...` — passed
- `make test-shard-check` — passed
- `TZ=UTC go test -tags testtools -race -count=1 ./fuku/db/eventmsg ./fuku/db/escalation ./fuku/db/modtuning ./fuku/db/advthresholds ./fuku/db/backup ./fuku/db/staff ./fuku/db/blacklists ./fuku/i18n` — all passed
- `TZ=UTC go test -tags testtools -race -count=1 -run '^(TestGetFloodFresh|TestSetFloodActionPreserveOff|TestAntiflood)' ./fuku/db/antiflood` — passed (pre-existing `TestSetFloodActionDuration/every_write_drops_the_antiflood_key` requires loopback TCP `127.0.0.1` which is blocked by the sandbox)
- `TZ=UTC go test -tags testtools -race -count=1 -run '^(TestLegacyPhase29Baseline|TestLegacyPhase30Baseline|TestPlainToneEveryEventEveryLocale|TestPlainToneRuntime|TestEventMsgValidateWriteMatchesSettingsPage|TestCopySettingsMirrorAndExclusions|TestSettingsCopyPage|TestPhase29Codes|TestPhase29ReadsCreateNothing|TestPhase29StaffPrivacyAndLogs|TestPhase29EndToEnd|TestSettingsBackupPage|TestPhase30EndToEnd|TestEventEngineCallers|TestPMCopyCallers|TestSettingsDeletionMemberCodesUnique)$' ./fuku/modules` — all passed

---

_Fixed: 2026-11-24T09:31:28Z_
_Fixer: the agent (gsd-code-fixer)_
_Iteration: 1_
