---
phase: 29-plain-tone-and-copy-settings
plan: 04
subsystem: copy-settings
tags: [copy-settings, tune-09, eventmsg, validate-write, msg-10, antiflood, escalation, modtuning, advthresholds]

requires:
  - phase: 29-plain-tone-and-copy-settings
    plan: 03
    provides: Complete Plain tone coverage across all 23 group and 18 PM event defaults plus plain_tone storage
provides:
  - Shared validated event-message writer (HasPM, NoMedia, NoSilent, ReservedButtonsFor, ReservedTextFor, ValidateMessage, WriteReport, SkippedRow, ReplaceAllValidated) in fuku/db/eventmsg with zero import of fuku/modules (reusable by Phase 30 MSG-10)
  - Atomic validated repository replace helpers: antiflood.GetFloodFreshContext, antiflood.SetFloodActionPreserveOffContext (explicit flood_limit=0 on insert), escalation.ReplaceLadder, modtuning.ReplaceDefaults, modtuning.ReplaceShortcuts, and advthresholds.ReplaceAll
  - Copy Settings core engine (applyCopySettings and fresh non-creating snapshot reader) in fuku/modules/copy_settings.go mirroring all 6 areas across 9 tables with strict exclusion boundaries
  - Drift guard (TestEventMsgValidateWriteMatchesSettingsPage) and full-mirror/clearing/exclusion/malformed-row tests (TestCopySettingsMirrorAndExclusions) under -race
affects: [29-05, 29-06, 30]

tech-stack:
  added: []
  patterns:
    - "Shared validated event-message write contract in package eventmsg (ValidateMessage + transactional ReplaceAllValidated + cache.DeleteCache) decoupled from fuku/modules so both Copy Settings and Phase 30 backup import reuse one source of truth"
    - "Explicit raw SQL INSERT ... VALUES (?, 0, ...) ON CONFLICT (chat_id) DO UPDATE SET action, action_secs, updated_at in antiflood.SetFloodActionPreserveOffContext to bypass GORM's default:5 tag on flood_limit and keep unconfigured targets off (P29-04)"
    - "Fresh non-creating snapshot read (readCopySettingsSnapshot) followed by explicit 9-table allow-list writes and IDs-only structured logging in applyCopySettings"

key-files:
  created:
    - fuku/db/eventmsg/validate_write.go
    - fuku/db/eventmsg/validate_write_test.go
    - fuku/modules/copy_settings.go
    - fuku/modules/copy_settings_test.go
  modified:
    - fuku/db/eventmsg/events.go
    - fuku/db/antiflood/repository.go
    - fuku/db/antiflood/repository_test.go
    - fuku/db/escalation/repository.go
    - fuku/db/escalation/repository_test.go
    - fuku/db/modtuning/repository.go
    - fuku/db/modtuning/repository_test.go
    - fuku/db/advthresholds/repository.go
    - fuku/db/advthresholds/repository_test.go

key-decisions:
  - "Checked raw rune count against base limit (4096/1024) and UTF-16 visible length against effective limit (limit - ReservedTextFor) in eventmsg.ValidateMessage, matching settings_edit_check.go so valid HTML-formatted messages saved by the editor are never falsely rejected during Copy Settings or backup import."
  - "Used explicit SQL INSERT with flood_limit = 0 and ON CONFLICT (chat_id) DO UPDATE of action, action_secs, updated_at in antiflood.SetFloodActionPreserveOffContext so GORM's default:5 tag can never silently enable antiflood on an unconfigured target."
  - "Read warns_settings directly via loadCopyWarnSettingsFresh in copy_settings.go (non-creating, uncached) and sorted all eventmsg.Messages.Rows map keys before calling eventmsg.ReplaceAllValidated so even tampered/unknown raw DB rows on source are deterministically validated and reported in MessagesSkipped."

patterns-established:
  - "Drift guard pattern: TestEventMsgValidateWriteMatchesSettingsPage enforces 1-to-1 parity between eventmsg metadata/constants and settingsMsgFamilies/eventPMSpecs/settingsEditPageDef on every test run."
  - "Atomic full-mirror repository replace pattern: validate before touching DB, delete row when input equals defaults (empty mirror clears target), upsert all columns on non-default input, and drop cache after commit."

requirements-completed: [TUNE-09]

status: complete
plan_head_before: d51429ba152f1bf11ba8cf8175834d4ccabaccb9
plan_head_after: 9a4025a3a6dab17fc619925cabee9973d279faca
commits: 3

actuals:
  tokens: 20429
  tasks: 3
  commits: 3

duration: 11min
completed: 2026-11-23
---

# Phase 29 Plan 04: Copy Settings Core Engine & Shared Validated Message Writer Summary

**Shared validated `eventmsg.ReplaceAllValidated` writer (zero `fuku/modules` dependency for Phase 30 MSG-10 reuse), four safe atomic tuning replace helpers eliminating the antiflood `default:5` trap, and `applyCopySettings` mirroring all 6 areas across 9 tables with editor drift guard and strict exclusion verification under `-race`**

## Performance

- **Duration:** 11 min
- **Started:** 2026-11-23T14:33:04Z
- **Completed:** 2026-11-23T14:43:59Z
- **Tasks:** 3
- **Files modified:** 13 (4 created, 9 modified; `fuku/modules/phase29_legacy_request_test.go`, `go.mod`, and `go.sum` untouched)

## Accomplishments

- **Shared Validated Event-Message Writer (`edfa1861`):**
  - Exported `HasPM`, `NoMedia`, `NoSilent`, `ReservedButtonsFor`, and `ReservedTextFor` in `fuku/db/eventmsg/events.go`.
  - Created `fuku/db/eventmsg/validate_write.go` exporting `MaxTextLength`, `MaxCaptionLength`, `MaxButtons`, `MaxRowButtons`, `MreqTextReserve`, `LockdownTextReserve`, `SkippedRow`, `WriteReport`, `ValidateMessage`, and `ReplaceAllValidated` with zero imports from `fuku/modules` so Phase 30's `fuku/db/backup` can call it directly without import cycles.
  - Enforced `ValidEvent`, `ValidDest`, `HasPM`, paired `FileID`/`MediaType` & allowed media kinds, `NoMedia`, `content.ValidButtonURL`, non-empty button labels, 100-button & 8-per-row caps with `{rules}` bonus and `ReservedButtonsFor`, 4096/1024 raw rune and UTF-16 visible length limits with `ReservedTextFor`, `NoSilent` normalization, single-transaction replace (omitting empty default rows and deleting the options row when all three options are false), and post-commit `cache.DeleteCache`.
  - Added `fuku/db/eventmsg/validate_write_test.go` (`TestEventMsgValidateAndReplaceAll`) covering every rejection case, boundary accepts, `NoSilent` normalization, transactional replacement, empty clearing, and cache invalidation.
- **Safe Tuning Repository Replace Helpers (`e0c18e42`):**
  - Added `antiflood.GetFloodFreshContext` (non-creating fresh read) and `antiflood.SetFloodActionPreserveOffContext` using explicit SQL `INSERT ... VALUES (?, 0, ...)` + `ON CONFLICT (chat_id) DO UPDATE` of `action`, `action_secs`, `updated_at` so an unconfigured target stays OFF (`Limit == 0`) while a configured target preserves `flood_limit` and `delete_antiflood_message` (P29-04, T-29-09).
  - Added `escalation.ReplaceLadder`, `modtuning.ReplaceDefaults`, `modtuning.ReplaceShortcuts`, and `advthresholds.ReplaceAll`, each validating inputs before touching DB, deleting rows when mirroring default/empty state, writing atomically, and dropping its cache key after commit.
  - Added focused unit tests to all four repository test suites (`fuku/db/{antiflood,escalation,modtuning,advthresholds}/repository_test.go`), all passing under `-race`.
- **Copy Settings Core & Verification (`9a4025a3`):**
  - Created `fuku/modules/copy_settings.go` with `copySettingsOutcome`, `copySettingsResult`, `readCopySettingsSnapshot`, and `applyCopySettings` mirroring all 6 areas across 9 tables (`chat_event_messages`, `chat_event_message_options`, `warns_settings`, `antiflood_settings`, `blacklists` action/secs on existing target rows only, `chat_escalation`, `chat_manual_defaults`, `chat_reason_shortcuts`, `chat_advanced_thresholds`) per P29-01/P29-03/P29-04, re-validating `appeal_url` via `validateAppealURL`, logging IDs/counts/step names only, and referencing zero `eventEngineEntries`.
  - Created `fuku/modules/copy_settings_test.go` with `TestEventMsgValidateWriteMatchesSettingsPage` (1-to-1 drift guard against `settingsMsgFamilies`, `eventPMSpecs`, and `settingsEditPageDef`) and `TestCopySettingsMirrorAndExclusions` (`full_mirror_populated_source`, `empty_source_clears_target_full_mirror`, `invalid_source_message_row_skipped_by_validated_writer`), proving full mirror, off-antiflood preservation, `warn_limit` preservation, blacklist word isolation, strict exclusion of all 9 excluded tables, source immutability / no ongoing link, and malformed-row rejection.

## Task Commits

1. **Task 1: implement shared validated message write function in fuku/db/eventmsg/validate_write.go** - `edfa1861` (`feat(29-04): add shared validated eventmsg writer ReplaceAllValidated`)
2. **Task 2: add safe replace helpers to antiflood, escalation, modtuning, and advthresholds repositories** - `e0c18e42` (`feat(29-04): add safe atomic replace helpers to tuning repositories`)
3. **Task 3: implement copy_settings.go core and drift/mirror/exclusion tests** - `9a4025a3` (`feat(29-04): implement Copy Settings core engine with drift and exclusion tests`)

## Files Created/Modified

- `fuku/db/eventmsg/events.go` — Exported `HasPM`, `NoMedia`, `NoSilent`, `ReservedButtonsFor`, and `ReservedTextFor`.
- `fuku/db/eventmsg/validate_write.go` — Shared `ValidateMessage`, `ReplaceAllValidated`, `WriteReport`, `SkippedRow`, and editor-aligned limit/reserve constants.
- `fuku/db/eventmsg/validate_write_test.go` — Comprehensive validation, `NoSilent`, transactional replacement, empty-clear, and cache drop tests.
- `fuku/db/antiflood/repository.go`, `fuku/db/antiflood/repository_test.go` — `GetFloodFreshContext`, `SetFloodActionPreserveOffContext`, and off-preservation/existing-preservation tests.
- `fuku/db/escalation/repository.go`, `fuku/db/escalation/repository_test.go` — `ReplaceLadder` and validation/default-delete/cache tests.
- `fuku/db/modtuning/repository.go`, `fuku/db/modtuning/repository_test.go` — `ReplaceDefaults`, `ReplaceShortcuts`, and validation/atomic-replace/clear/cache tests.
- `fuku/db/advthresholds/repository.go`, `fuku/db/advthresholds/repository_test.go` — `ReplaceAll` and bounds/zero-reset/cache tests.
- `fuku/modules/copy_settings.go` — `copySettingsOutcome`, `copySettingsResult`, `readCopySettingsSnapshot`, `loadCopyWarnSettingsFresh`, and `applyCopySettings`.
- `fuku/modules/copy_settings_test.go` — `TestEventMsgValidateWriteMatchesSettingsPage` and `TestCopySettingsMirrorAndExclusions`.

## Decisions Made

- Enforced raw rune count against the base Telegram limit (`4096`/`1024`) and UTF-16 visible length against `limit - ReservedTextFor` in `ValidateMessage` so HTML-formatted editor messages stay 100% compatible with Copy Settings and Phase 30 backup import while still refusing oversized raw HTML or visible text.
- Bypassed GORM's struct `default:5` tag via raw SQL upsert in `antiflood.SetFloodActionPreserveOffContext` to guarantee `flood_limit = 0` on brand-new target rows across both SQLite and PostgreSQL.
- Referenced `models.ReasonShortcutsCapMax` (the existing model constant) in `modtuning.ReplaceShortcuts` instead of introducing a duplicate constant.

## Deviations from Plan

None - plan executed exactly as written (minor constant-name alignment to existing `models.ReasonShortcutsCapMax` / `blacklists.AddBlacklist` / `models.StaffHealthOK` in the codebase).

## Issues Encountered

- Pre-existing `fuku/db/antiflood/timed_action_test.go` (`TestSetFloodActionDuration`) and `TestLegacyPhase29Baseline` open localhost TCP miniredis sockets, which require loopback network access outside the default sandbox; running with `TZ=UTC` and loopback enabled passed every suite cleanly under `-race`.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `eventmsg.ReplaceAllValidated`, all four repository replace helpers, and `applyCopySettings` are ready for Plan 29-05 (private Copy Settings page `cp`, one-time Redis confirm token, dual-group live verification, and 8-locale UI) and Phase 30 (`MSG-10` backup import).

## Self-Check: PASSED

- Created files exist: `fuku/db/eventmsg/validate_write.go`, `fuku/db/eventmsg/validate_write_test.go`, `fuku/modules/copy_settings.go`, `fuku/modules/copy_settings_test.go`
- Commits exist and are ancestors of `HEAD`: `edfa1861`, `e0c18e42`, `9a4025a3` (`commits: 3` matching `git rev-list --count d51429ba152f1bf11ba8cf8175834d4ccabaccb9..9a4025a3a6dab17fc619925cabee9973d279faca`)
- Untouched invariants verified: `fuku/modules/phase29_legacy_request_test.go`, `go.mod`, `go.sum` have zero diffs
- All verification suites passed under `-race`: `./fuku/db/eventmsg`, `./fuku/db/antiflood`, `./fuku/db/escalation`, `./fuku/db/modtuning`, `./fuku/db/advthresholds`, `TestEventMsgValidateWriteMatchesSettingsPage`, `TestCopySettingsMirrorAndExclusions` (3/3 subtests), `TestLegacyPhase29Baseline`, `TestEventEngineCallers`, `TestPMCopyCallers`, `CGO_ENABLED=0 go build ./...`, and `go vet -tags testtools`
