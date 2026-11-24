---
phase: 27-manual-command-defaults-reason-shortcuts-and-appeal-button
plan: 03
subsystem: moderation
status: complete
tags: [modtuning, reason-shortcuts, expansion, html-escaping, tune-05]
requires:
  - 27-02 (chat_reason_shortcuts table, fuku/db/modtuning skipLocal repository, manual command tuning pipeline)
provides:
  - Reason shortcut normalization, validation, and apply helpers (normaliseReasonShortcutWord, checkReasonShortcutWord, checkReasonShortcutText, applyReasonShortcutAdd, applyReasonShortcutRemoveByID) in fuku/modules/modtuning_settings.go
  - Runtime single-word shortcut expansion (manualShortcutsLookup, expandManualReason) with zero-I/O empty/multi-word fast-path and fail-open IDs-only logging
  - Shortcut expansion wired into all 14 manual user-punishment commands (/ban, /dban, /sban, /tban, /kick, /dkick, /skick, /mute, /dmute, /smute, /tmute, /warn, /dwarn, /swarn)
  - Comprehensive validation, expansion, HTML escaping, PM copy, warn/warn-limit, log channel, fail-open, and isolation tests in fuku/modules/modtuning_reasons_test.go
affects: [27-06, 27-07]
tech-stack:
  added: []
  patterns:
    - "Zero-I/O fast-path in expandManualReason: empty or multi-word reasons return immediately without touching DB or cache"
    - "Raw plain-text storage in chat_reason_shortcuts with HTML escaping at every render site (group default reply, custom {reason}, PM copy, warn/warn-limit reply, actionlog.Admin) while /warns prints plain text"
    - "IDs-only Warn logging in all shortcut apply and lookup error paths (never logging word, reason text, or raw error strings that could echo user text)"
key-files:
  created:
    - fuku/modules/modtuning_reasons_test.go
  modified:
    - fuku/modules/modtuning_settings.go
    - fuku/modules/moderation.go
    - fuku/modules/bans.go
    - fuku/modules/mute.go
    - fuku/modules/warns.go
decisions:
  - "Checked fresh existence before the 50-entry cap in applyReasonShortcutAdd (matching filters.go checkFilterKeyword) and treated ON CONFLICT DO NOTHING races (added=false) as reasonShortcutExists so duplicates never overwrite."
  - "Logged chatID/userID/shortcutID only without echoing raw DB error strings in modtuning_settings.go so constraint or driver error details can never leak shortcut words or reason text into logs (T-27-08)."
  - "Wired expandManualReason in moderation.go (for the 9 non-temporal reasonChecked user-punishment commands), bans.go (moderationTban after extractTemporalTarget + manualApplyReasonRequired), mute.go (moderationTmute after extractTemporalTarget + manualApplyReasonRequired), and warns.go (dispatchWarn before warnThisUser, keeping warnSourceAuto and Staff Group commands completely isolated)."
metrics:
  duration: 6min
  completed: 2026-11-22
actuals:
  tokens: 9755
  tasks: 2
  commits: 2
plan_head_before: fa95390e28fd6609d7c53899d9054fc113de46a7
plan_head_after: a3e1e3ed2782815503da163960b3edd7982f9625
---

# Phase 27 Plan 03: Reason Shortcuts Validation, Apply Layer, and Manual Expansion Summary

Implemented TUNE-05 reason-shortcut validation/apply helpers and wired single-word, case-insensitive shortcut expansion with empty/multi-word zero-I/O fast-path into all 14 manual user-punishment commands (`/ban`, `/dban`, `/sban`, `/tban`, `/kick`, `/dkick`, `/skick`, `/mute`, `/dmute`, `/smute`, `/tmute`, `/warn`, `/dwarn`, `/swarn`), verified end-to-end across group replies, custom `{reason}` messages, PM copies, warn storage, `/warns`, warn-limit replies, and log channel posts with HTML escaping, fail-open DB handling, and complete isolation from automatic warns and Staff Group commands.

## Performance

- **Duration:** 6 min
- **Started:** 2026-11-22T20:05:26Z
- **Completed:** 2026-11-22T20:11:55Z
- **Tasks:** 2
- **Files created:** 1
- **Files modified:** 5

## Accomplishments

- **Task 1 (`7ee9f123`):** Added `reasonWordOutcome`, `reasonShortcutAddOutcome`, `reasonShortcutRemoveOutcome`, `manualShortcutsLookup`, `reasonShortcutWrites`, `normaliseReasonShortcutWord`, `checkReasonShortcutWord`, `checkReasonShortcutText`, `applyReasonShortcutAdd`, `applyReasonShortcutRemoveByID`, and `expandManualReason` in `fuku/modules/modtuning_settings.go`; wired `expandManualReason` into `fuku/modules/moderation.go`, `fuku/modules/bans.go` (`moderationTban`), `fuku/modules/mute.go` (`moderationTmute`), and `fuku/modules/warns.go` (`dispatchWarn`, never `warnThisUser`).
- **Task 2 (`a3e1e3ed`):** Added `fuku/modules/modtuning_reasons_test.go` with `TestReasonShortcutValidationAndApply` (word/text validation, duplicate refusal without overwrite, 50-shortcut cap, foreign-chat remove refusal, raced add, error paths, and IDs-only log capture) and `TestReasonShortcutExpansion` (7 top-level subtests proving HTML-escaped expansion across group replies, custom `{reason}`, PM copies, log channel posts, `/tban`/`/tmute` duration preservation, silent commands, `/warn`/`/dwarn`/`/swarn` + `/warns` plain text + warn-limit escaping, unknown/multi-word verbatim + zero-lookup fast-path, fail-open DB read error, and `warnSourceAuto` + Staff Group isolation).

## Task Commits

1. **Task 1: reason-shortcut validation/apply helpers and manual command expansion** — `7ee9f123` (`feat(27-03): add reason shortcut apply helpers and manual command expansion`)
2. **Task 2: comprehensive validation, expansion, escaping, and isolation tests** — `a3e1e3ed` (`test(27-03): add reason shortcut validation, expansion, escaping, and isolation tests`)

## Verification Results

- `go test -tags testtools -race -count=1 -run '^(TestReasonShortcutValidationAndApply|TestReasonShortcutExpansion|TestManualCommandDefaults|TestLegacyPhase27ModerationRequests|TestLegacyModerationRequests|TestEventEngineCallers|TestPMCopyCallers)$' -v ./fuku/modules` — PASS (7 top-level subtests of `TestReasonShortcutExpansion`, `TestReasonShortcutValidationAndApply`, `TestManualCommandDefaults`, both MSG-11 golden suites, and both EVT-10 caller guards all pass with 0 failures and 0 skips).
- `CGO_ENABLED=0 go build ./...` — PASS.
- `grep -c 'func expandManualReason(' fuku/modules/modtuning_settings.go` — `1`.
- `grep -c 'expandManualReason(' fuku/modules/warns.go` — `1` (inside `dispatchWarn`).
- `gofmt -l fuku/modules/modtuning_settings.go fuku/modules/moderation.go fuku/modules/bans.go fuku/modules/mute.go fuku/modules/warns.go fuku/modules/modtuning_reasons_test.go` — clean (empty).
- `git diff --exit-code fa95390e28fd6609d7c53899d9054fc113de46a7 -- fuku/modules/moderation_phase27_legacy_request_test.go fuku/modules/moderation_legacy_request_test.go go.mod go.sum` — PASS (MSG-11 goldens and Go dependencies untouched).

## Deviations from Plan

None - plan executed exactly as written. Note: in `TestReasonShortcutExpansion/warn_dwarn_swarn_expand_store_pm_and_limit_escape`, because `warns.go` does not post to `actionlog.Admin` (only `moderationCommand.run` does), the warn subtest verifies warn reply, warn PM copy, stored raw warn text via `warns.GetWarns`, plain-text `/warns` output, and HTML-escaped warn-limit output, while log channel expansion is verified across `/ban`, `/dban`, `/sban`, `/tban`, `/kick`, `/dkick`, `/skick`, `/mute`, `/dmute`, `/smute`, and `/tmute`.

## Known Stubs

None.

## Next Phase Readiness

- TUNE-05 validation, apply helpers (`normaliseReasonShortcutWord`, `checkReasonShortcutWord`, `checkReasonShortcutText`, `applyReasonShortcutAdd`, `applyReasonShortcutRemoveByID`), and runtime manual expansion are ready for Plan 27-06 (private Reason shortcuts list page `rs`) and Plan 27-04 (appeal button).

## Self-Check: PASSED

- Created/modified files verified on disk: `fuku/modules/modtuning_settings.go`, `fuku/modules/moderation.go`, `fuku/modules/bans.go`, `fuku/modules/mute.go`, `fuku/modules/warns.go`, `fuku/modules/modtuning_reasons_test.go` — ALL FOUND.
- Task commits verified as ancestors of HEAD: `7ee9f123`, `a3e1e3ed` — ALL FOUND.
