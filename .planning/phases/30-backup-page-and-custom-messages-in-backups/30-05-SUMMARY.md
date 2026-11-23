---
phase: 30-backup-page-and-custom-messages-in-backups
plan: 05
subsystem: verification-and-docs
tags: [backup, settings, safeguards, privacy, logs, e2e, dispatcher, agents-md, immutability, msg-10, bkp-01, bkp-02, bkp-03]

requires:
  - phase: 30-backup-page-and-custom-messages-in-backups
    provides: Wave 1 immutable golden TestLegacyPhase30Baseline (Plan 30-01, commit f73400f4)
  - phase: 30-backup-page-and-custom-messages-in-backups
    provides: Transaction-safe eventmsg.ReplaceAllValidatedTx and BackupModuleMessages = "messages" in fuku/db/backup (Plan 30-02, commit 0ac1d0e4)
  - phase: 30-backup-page-and-custom-messages-in-backups
    provides: Shared backup apply helpers and cross-replica Redis one-time backup tokens fuku:settings:backup:{import,reset}:<token> (Plan 30-03, commit 0cb8ba24)
  - phase: 30-backup-page-and-custom-messages-in-backups
    provides: Private-only Backup settings page ba (Order 83, actions bv/be/bi/bl/bj/br/bu), 8-locale UI, 30-page Main/Staff grid updates, and TestSettingsBackupPage (Plan 30-04, commit d8880d38)
provides:
  - Phase 30 safeguard suite in fuku/modules/settings_phase30_test.go (TestPhase30Codes, TestPhase30ReadsCreateNothing, TestPhase30StaffPrivacyAndLogs) verified under -race with zero localhost TCP sockets
  - Real-dispatcher end-to-end proof in fuku/modules/phase30_e2e_test.go (TestPhase30EndToEnd) driving LoadSettings, LoadBackup, LoadBans, and LoadRules over in-process miniredis and backupRoundTripFunc
  - Updated AGENTS.md documenting zero new goroutines/handler groups/shutdown drains, BackupModuleMessages, eventmsg.ReplaceAllValidatedTx, Redis keys fuku:settings:backup:{import,reset}:<token>, page ba (Order 83, 30 pages total), Phase 30 test suites, and provisional decisions P30-01..P30-04
  - Full local phase gate verification (translations, docs, go vet, CGO_ENABLED=0 build, test-shard-check, DB/i18n/module -race suites, AST caller guards, and strict golden + go.mod/go.sum immutability)
affects: [phase-30-verification, milestone-v1.2]

tech-stack:
  added: []
  patterns:
    - "In-process real-dispatcher E2E proof (phase28InProcessMiniredis + copyTitleBotClient + messagesE2EClient + backupRoundTripFunc) covering Export, Reset, live /ban fallback, Import with editor validation & skipped reporting, live /ban custom group+PM delivery, replay refusal, and old-export backward compatibility"
    - "Full DebugLevel logrus sweep across live Export (be), Reset (br/bu), Import (bi -> document -> bj), and Staff Group refusals asserting zero leakage of group titles, Staff Group titles, owner names, custom messages, file IDs, URLs, rules, or Redis tokens"

key-files:
  created:
    - fuku/modules/settings_phase30_test.go
    - fuku/modules/phase30_e2e_test.go
  modified:
    - AGENTS.md

key-decisions:
  - "Checked settingsLockRows alongside settingsCoreActions and all 30 settingsPages in TestPhase30Codes to permanently prevent Backup action codes (bv/be/bi/bl/bj/br/bu) from colliding with lock-row codes such as bt."
  - "Ordered settingsHubSeedGroup before ChangeGroupLanguage and env.setAdminsAllowed in TestPhase30EndToEnd because settingsHubSeedGroup runs settingsCleanup, ensuring live settingsGate admission for the allowed admin and deterministic English group replies."
  - "Recorded provisional owner decisions P30-01..P30-04 and all Phase 30 invariants directly alongside Phase 29 across AGENTS.md Startup/shutdown, Settings, Redis keys, Database/backup, and Test families sections."

patterns-established:
  - "Phase 30 safeguard & E2E verification pattern running 100% in-process under -race with zero localhost TCP listeners."

requirements-completed: [MSG-10, BKP-01, BKP-02, BKP-03]

status: complete
plan_head_before: 96a43a824eec4c10e052c5aff29ebb45bfc47312
plan_head_after: 1731d8df3cb1f110e35faa7add6ea114245449f6
commits: 3

actuals:
  tokens: 14811
  tasks: 3
  commits: 3

duration: 29min
completed: 2026-11-23
---

# Phase 30 Plan 05: Safeguards, Real-Dispatcher E2E Proof, AGENTS.md Update, and Local Phase Gates Summary

**Added Phase 30 safeguard suite (`TestPhase30Codes`, `TestPhase30ReadsCreateNothing`, `TestPhase30StaffPrivacyAndLogs`), real-dispatcher end-to-end proof (`TestPhase30EndToEnd`) covering Backup Export/Reset/Import and custom messages in backups with live `/ban` delivery, updated `AGENTS.md` with all Phase 30 invariants and provisional decisions `P30-01..P30-04`, and verified all local phase gates plus strict golden and dependency immutability**

## Performance

- **Duration:** 29 min
- **Started:** 2026-11-23T21:30:37Z
- **Completed:** 2026-11-23T21:59:35Z
- **Tasks:** 3
- **Files modified:** 3 (2 created, 1 modified)

## Accomplishments

- **Task 1 (`1da07ac9`):** Created `fuku/modules/settings_phase30_test.go` implementing:
  - `TestPhase30Codes`: verifies all 30 registered settings pages, private-only page `ba` (`Order: 83`, `Icon: "💾"`, 7 actions `bv`/`be`/`bi`/`bl`/`bj`/`br`/`bu`), `settingsTypedPages[settingsPageBackup]` wiring, zero collisions against `settingsCoreActions`, `settingsLockRows`, and every other page, and step-1 refusal of extra fields or group-menu presses (`u` instead of `c`) with `common_callback_invalid_request` alert and 0 `getChatMember` calls.
  - `TestPhase30ReadsCreateNothing`: opens Main (group & private), opens `ba` (group note & private page), and toggles `bv` (`m=1`, `m=0`, `m=fffff`) on an unconfigured chat, proving 0 rows created across all 10 tables in scope.
  - `TestPhase30StaffPrivacyAndLogs`: captures live `logrus` output at `DebugLevel` across `ba`/`bv`, Export (`be`), Reset (`br` -> `bu`), Import (`bi` -> private `.json` document -> `bj`), and Staff Group Main + refusals, proving Staff Group hides `ba`, never leaks Staff Group title or ID in any screen/caption/card/notice, and logs contain 0 sentinel titles, names, message texts, file IDs, button URLs, rules, document file IDs, or Redis confirmation tokens (`k`).
- **Task 2 (`2a939ab9`):** Created `fuku/modules/phase30_e2e_test.go` implementing `TestPhase30EndToEnd` driving `LoadSettings`, `LoadBackup`, `LoadBans`, and `LoadRules` through a real `ext.Dispatcher` over `phase28InProcessMiniredis(t)` and `backupRoundTripFunc`:
  - Custom `EventBan` group + PM messages (`CustomFacilityBan`, `CustomPMBan`, Appeal URL button), `PlainTone: true`, and `rules` exported via Backup page `be` (`m=fffff`) to the admin's private chat with exact 20-module parity against `/export`.
  - Non-owner refusal on `br`, live owner two-tap Reset (`br` -> `bu`) clearing custom messages, options, and rules in DB & cache (`P30-02`), live `/ban spam` falling back to standard default reply with 0 PM copies, and replay refusal (`settings_card_expired`).
  - Non-owner refusal on `bi`, live owner private `.json` upload with plain-text refusal (keeping wait alive) and 2 injected invalid custom message entries (`javascript:evil()` button on `EventKick` + unknown `"future_event_key"`), two-tap Import (`bj`) reporting 20 modules imported and 2 invalid entries skipped, live `/ban restored` posting restored custom group message + Appeal button AND delivering restored custom PM copy, and replay refusal.
  - Old pre-Phase-30 export without `"messages"` imported cleanly via `bi`/`bj`, updating `rules` while preserving restored custom `EventBan` group/PM messages and `PlainTone: true`.
- **Task 3 (`1731d8df`):** Updated `AGENTS.md` across Startup/shutdown, Settings pages, Redis keys, Database/backup, and Phase 30 test families sections with all Phase 30 invariants (`BackupModuleMessages = "messages"`, `eventmsg.ReplaceAllValidatedTx`, `fuku:settings:backup:import:<token>` & `fuku:settings:backup:reset:<token>`, page `ba` at Order `83`, 30 registered pages, zero new goroutines/handler groups/shutdown drains, and provisional decisions `P30-01..P30-04`), and verified every local phase and immutability gate.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add Phase 30 codes, non-creating reads, and Staff privacy/log tests** - `1da07ac9` (`test(30-05): add Phase 30 codes, non-creating reads, and Staff privacy/log tests`)
2. **Task 2: Add real-dispatcher E2E proof for Backup page and custom messages in backups** - `2a939ab9` (`test(30-05): add real-dispatcher E2E proof for Backup page and custom messages in backups`)
3. **Task 3: Update AGENTS.md and run all Phase 30 verification & immutability gates** - `1731d8df` (`docs(30-05): document Phase 30 Backup page, custom messages in backups, and P30-01..P30-04`)

## Files Created/Modified

- `fuku/modules/settings_phase30_test.go` — Safeguard suite (`TestPhase30Codes`, `TestPhase30ReadsCreateNothing`, `TestPhase30StaffPrivacyAndLogs`) running in-process under `-race`.
- `fuku/modules/phase30_e2e_test.go` — Real-dispatcher E2E proof (`TestPhase30EndToEnd`) for Backup page Export/Reset/Import and custom messages in backups.
- `AGENTS.md` — Documented Phase 30 zero-drain posture, Backup page `ba` (Order `83`, 30 pages), Redis backup confirmation tokens, `BackupModuleMessages` + `eventmsg.ReplaceAllValidatedTx`, Phase 30 test suites, and provisional decisions `P30-01..P30-04`.

## Decisions Made

- Explicitly guarded against lock-row code collisions (`settingsLockRows`, e.g., `bt`) inside `TestPhase30Codes` alongside `settingsCoreActions` and all 30 registered settings pages.
- Sequenced `settingsHubSeedGroup` ahead of `ChangeGroupLanguage` and `env.setAdminsAllowed` in `TestPhase30EndToEnd` so hub roster cleanup does not reset `chat_settings_access` or group language prior to allowed-admin Export and live `/ban` checks.

## Deviations from Plan

None - plan executed as written.

## Verification Results

- `make check-translations` — PASSED (1068 keys present across all 8 locales)
- `make check-docs` — PASSED (no drift detected)
- `go vet -tags testtools ./...` — PASSED
- `CGO_ENABLED=0 go build ./...` — PASSED
- `make test-shard-check` — PASSED (all test shards match `go test -tags testtools -list`)
- `TZ=UTC go test -tags testtools -race -count=1 ./fuku/db/eventmsg ./fuku/db/backup` — PASSED (0 failures, 0 skips)
- `TZ=UTC go test -tags testtools -race -count=1 ./fuku/i18n` — PASSED (full package, 0 failures, 0 skips)
- `TZ=UTC go test -tags testtools -race -count=1 -run '^(TestPhase30Codes|TestPhase30ReadsCreateNothing|TestPhase30StaffPrivacyAndLogs)$' -v ./fuku/modules` — PASSED (3/3 in 0.34s)
- `TZ=UTC go test -tags testtools -race -count=1 -run '^TestSettingsStaffPrivacy/pure_renders_in_every_language$' -v ./fuku/modules` — PASSED (all 8 languages)
- `TZ=UTC go test -tags testtools -race -count=1 -run '^(TestPhase30EndToEnd|TestLegacyPhase30Baseline)$' -v ./fuku/modules` — PASSED (`TestPhase30EndToEnd` in 1.18s, `TestLegacyPhase30Baseline` in 0.02s)
- `TZ=UTC go test -tags testtools -race -count=1 -run '^(TestLegacyPhase30Baseline|TestLegacyPhase29Baseline|TestSharedBackupApplyHelpers|TestSettingsBackupRedisTokens|TestSettingsBackupPage|TestExportGoesToThePrivateChat|TestSettingsMainPageFitsEveryLocale|TestSettingsPhase18Codes|TestSettingsPhase19Codes|TestPhase29Codes|TestPhase30Codes|TestPhase30ReadsCreateNothing|TestPhase30StaffPrivacyAndLogs|TestPhase30EndToEnd|TestEventEngineCallers|TestPMCopyCallers|TestNoRawNewCommand|TestCommandRepliesStayInsideTheHandler|TestEventMsgValidateWriteMatchesSettingsPage|TestSettingsCopyPage|TestPhase29EndToEnd)$' ./fuku/modules` — PASSED (5.005s, 0 failures, 0 skips)
- `TZ=UTC go test -tags testtools -race -count=1 -run '^TestSettingsMainGrid$/^(the_owner|the_same|nine_grid|fifteen_grid|an_allowed|an_odd|a_page|a_notice|no_visible)' ./fuku/modules` — PASSED (10/10 pure subtests)
- `TZ=UTC go test -tags testtools -race -count=1 -run '^TestMemberDeletionExcludedMessages$/^no_file_of_the_never-delete_families_calls_a_deletion_producer$' -v ./fuku/modules` — PASSED (AST guard)
- `AGENTS.md` checks: `fuku:settings:backup:import:` (3), `fuku:settings:backup:reset:` (3), `P30-01` (2), `P30-04` (2)
- Golden & dependency immutability:
  - `git log --format=%h 1da5af6f..HEAD -- fuku/modules/phase30_legacy_request_test.go` -> `1` commit (`f73400f4`)
  - `git diff --exit-code 1da5af6f -- go.mod go.sum` -> exit `0` (unchanged)
  - `git diff --name-only 1da5af6f -- 'fuku/**/*_legacy_request_test.go'` -> `0` pre-existing goldens modified
- **Sandbox note on pre-existing TCP-miniredis tests:** Pre-existing tests that bind/dial loopback TCP `127.0.0.1` via `miniredis.Run()` (`TestSettingsStaffPrivacy` non-pure subtests, `TestSettingsMainGrid/end_to_end`, `TestSettingsMainPhase18`, `TestSettingsMainPhase19`, `TestSettingsFwdBlockMainShape`, `TestSettingsPhase18InStaffGroup`, `TestMemberDeletionExcludedMessages` TCP subtests) hit `dial tcp 127.0.0.1:...: connect: operation not permitted` inside this sandbox. Every new Phase 30 suite (`settings_phase30_test.go`, `phase30_e2e_test.go`, `settings_backup_test.go`, `backup_test.go`), every pure/AST subtest (`TestSettingsStaffPrivacy/pure_renders_in_every_language`, `TestSettingsMainGrid` pure subtests, `TestMemberDeletionExcludedMessages/no_file_of_the_never-delete_families_calls_a_deletion_producer`), and all live Main/Staff grid shape assertions inside `TestSettingsBackupPage` passed cleanly in-process under `-race`.

## Issues Encountered

- Initial run of `TestPhase30EndToEnd` failed at Part 2 with `settings_gate_owner_only` because `settingsHubSeedGroup` ran after `env.setAdminsAllowed(chat.Id, true)` and invoked `settingsCleanup`, wiping `chat_settings_access`. Moving `settingsHubSeedGroup` ahead of `ChangeGroupLanguage` and `env.setAdminsAllowed` resolved it cleanly on the next run.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- All 5 plans of Phase 30 (30-01 through 30-05) are complete and verified under `-race`.
- Ready for Phase 30 verification (`/gsd-verify-work` / phase verifier) and milestone v1.2 closure.

## Self-Check: PASSED
