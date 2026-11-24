---
phase: 27-manual-command-defaults-reason-shortcuts-and-appeal-button
fixed_at: 2026-11-24T15:17:00Z
review_path: .planning/phases/27-manual-command-defaults-reason-shortcuts-and-appeal-button/27-REVIEW.md
fix_scope: critical_warning
iteration: 1
findings_in_scope: 4
fixed: 4
skipped: 0
status: all_fixed
---

# Phase 27: Code Review Fix Report

**Fixed at:** 2026-11-24T15:17:00Z
**Source review:** `.planning/phases/27-manual-command-defaults-reason-shortcuts-and-appeal-button/27-REVIEW.md`
**Fix scope:** `critical_warning` (`WR-01`..`WR-04`; `IN-01` and `IN-02` out of scope)
**Iteration:** 1

**Summary:**
- Findings in scope: 4
- Fixed: 4
- Skipped: 0

## Fixed Issues

### WR-01: `manualBanMember` and `manualMuteMember` silently escalate to permanent punishment when `manualDefaultUntil` returns false

**Files modified:** `fuku/modules/modtuning_settings.go`, `fuku/modules/modtuning_commands_test.go`
**Commit:** `ae508855`
**Status:** fixed
**Applied fix:** When `defs.DefaultBanSecs > 0` or `defs.DefaultMuteSecs > 0` and `manualDefaultUntil` returns `false`, log a chat-ID-only warning and return `errPunishTimeRanOut` instead of falling through to an untimed permanent ban/mute call. Added a unit test (`TestManualCommandDefaults/invalid_positive_default_fails_safe`) confirming zero Telegram API calls are made on an invalid positive default duration.

### WR-02: `validateAppealURL` accepts empty-hostname URLs and ban reply paths have no fallback when Telegram rejects the appeal button

**Files modified:** `fuku/modules/modtuning_settings.go`, `fuku/modules/event_message.go`, `fuku/modules/event_message_pm.go`, `fuku/modules/modtuning_appeal_test.go`
**Commit:** `24dd746b`
**Status:** fixed
**Applied fix:** Tightened `validateAppealURL` with `net/url.Parse` to require `http`/`https` and a non-empty `u.Hostname()`, rejecting host-less port URLs such as `https://:80`. Added a retry-without-appeal-row fallback on `eventMessageRefused` in `sendDefaultPost` (logging chat ID only) and on `pmCopyRefused` in `runEventPM` for both default and custom PM copies (preserving `tried.Text` when `o.Fallback` is empty on non-custom PMs). Added tests covering empty-hostname rejection and group/PM refusal fallbacks.

### WR-03: TOCTOU race on `ReasonShortcutsCapMax` (50) can exceed the cap and break `ReplaceShortcutsTx` callers

**Files modified:** `fuku/db/modtuning/repository.go`, `fuku/db/modtuning/repository_test.go`, `fuku/modules/modtuning_settings.go`, `fuku/modules/modtuning_reasons_test.go`
**Commit:** `5c2038ab`
**Status:** fixed
**Applied fix:** Added `modtuning.ErrShortcutCapReached` (preserving the `(bool, error)` signature of `modtuning.AddShortcut`) and moved duplicate + `ReasonShortcutsCapMax` enforcement inside a transaction in `AddShortcut`, serialized per chat via `pg_advisory_xact_lock` on Postgres (and an in-process mutex for non-Postgres SQLite test drivers). Mapped `ErrShortcutCapReached` to `reasonShortcutCap` in `applyReasonShortcutAdd`. Added concurrent cap-race tests in both `fuku/db/modtuning` and `fuku/modules`.

### WR-04: `settingsModDefAppealTyped` renders fabricated zero defaults on post-save read failure

**Files modified:** `fuku/modules/settings_moddefaults.go`, `fuku/modules/settings_moddefaults_test.go`
**Commit:** `39dad824`
**Status:** fixed
**Applied fix:** On post-save `settingsModDefaultsReadNow` failure in `settingsModDefAppealTyped`, log a warning, send only the saved confirmation notice without rendering fabricated zero defaults, and close the prompt (matching `settings_input_door.go`). Added a socket-free `phase28InProcessMiniredis`-backed test (`TestSettingsModDefaults/appeal_link_post_save_read_error_sends_notice_only`).

## Verification

**Environment:** Main checkout (`workflow.use_worktrees` is `false` in `.planning/config.json`).

All checks passed:
- `go vet -tags testtools ./...`
- `CGO_ENABLED=0 go build ./...`
- `make check-translations` (1068 keys across all 8 locales)
- `make check-docs` (no drift)
- `make test-shard-check`
- `TZ=UTC go test -tags testtools -race -count=1 ./fuku/db/modtuning`
- `TZ=UTC go test -tags testtools -race -count=1 -run '^(TestLegacyPhase27ModerationRequests|TestManualCommandDefaults|TestAppealURLValidationAndApply|TestAppealButtonOnBanMessages|TestReasonShortcutValidationAndApply|TestReasonShortcutExpansion|TestEventEngineCallers|TestPMCopyCallers|TestLegacyPhase29Baseline|TestLegacyPhase30Baseline)$' ./fuku/modules`
- `TZ=UTC go test -tags testtools -race -count=1 -run '^(TestPhase27Codes|TestSettingsModDefaults/appeal_link_post_save_read_error_sends_notice_only)$' ./fuku/modules`

*(Note: legacy TCP-bound `miniredis` subtests in `TestSettingsModDefaults`/`TestSettingsReasons` that dial `127.0.0.1` are blocked by the sandbox's localhost socket restriction; the new WR-04 regression test uses `phase28InProcessMiniredis` and passed under `-race`.)*

---

_Fixed: 2026-11-24T15:17:00Z_
_Fixer: the agent (gsd-code-fixer)_
_Iteration: 1_
