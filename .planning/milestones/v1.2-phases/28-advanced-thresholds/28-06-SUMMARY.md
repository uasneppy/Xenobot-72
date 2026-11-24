---
phase: 28-advanced-thresholds
plan: 06
subsystem: verification-and-docs
tags: [settings, safeguards, privacy, e2e, dispatcher, agents-md, verification, thresholds]

requires:
  - phase: 28-advanced-thresholds
    provides: chat_advanced_thresholds skipLocal repository, Field/Area specs, and raid/join/flood/AI burst wiring (28-02)
  - phase: 28-advanced-thresholds
    provides: AI spam bars and member requirements cap/pause runtime wiring (28-03)
  - phase: 28-advanced-thresholds
    provides: Lockdown reminder interval and Option A extended captcha timeout runtime wiring plus newPhase28LockdownEnv helper (28-04)
  - phase: 28-advanced-thresholds
    provides: Private Advanced settings page av (aa/ao/as/ar/ax), 8-locale copy, Main/Staff-Group grid integration, and newPhase28SettingsEnv helper (28-05)
provides:
  - Phase 28 safeguard suite (TestPhase28Codes, TestPhase28ReadsCreateNothing, TestPhase28StaffPrivacyAndLogs) in fuku/modules/settings_phase28_test.go
  - End-to-end real-dispatcher proof (TestAdvancedThresholdsEndToEnd) in fuku/modules/advthresholds_e2e_test.go driving private UI edits into live flood lockdown, lockdown reminder, extended captcha timeout, and default restoration
  - Complete Phase 28 documentation in AGENTS.md (zero shutdown drain, chat_advanced_thresholds schema/skipLocal/backup exclusion, page av & runtime wiring, test families, goldens, drift guard, and provisional decisions P28-01..P28-04)
  - Full local Phase 28 verification gate pass (translations, docs, vet, build, test-shard-check, DB/cache/backup/i18n/module suites, golden immutability, go.mod/go.sum unchanged)
affects: [phase-28-verification]

tech-stack:
  added: []
  patterns:
    - "In-process miniredis safeguard & E2E verification (newPhase28SettingsEnv + phase28InProcessMiniredis) executing real dispatcher callbacks, Redis Lua scripts, skipLocal cache invalidations, worker cycles, and captcha challenges with zero localhost TCP dials"
    - "IDs-only logrus capture sweep over both normal settings edits/resets and forced DB read failures proving zero Staff Group, group title, or user name leaks (T-28-16)"

key-files:
  created:
    - fuku/modules/settings_phase28_test.go
    - fuku/modules/advthresholds_e2e_test.go
  modified:
    - AGENTS.md

key-decisions:
  - "Used real locales (withRealLocales + uk translator) in TestAdvancedThresholdsEndToEnd so flood lockdown alerts, reminder headers, Main state lines, and %d-formatted captcha challenge timeouts ('<b>30' vs '<b>2') are verified end-to-end as real user-facing text."
  - "Documented all four provisional owner decisions (P28-01..P28-04), zero new shutdown drain, chat_advanced_thresholds + skipLocal, page av (Order 81, PrivateOnly, hidden in Staff Groups), and Phase 28 test/golden/drift-guard families in AGENTS.md."

patterns-established:
  - "TestAdvancedThresholdsEndToEnd: single-chat real-dispatcher E2E flow covering UI edit -> live flood trigger -> reminder worker -> captcha joiner -> UI reset -> restored defaults."

requirements-completed: [TUNE-07]

status: complete
plan_head_before: 748aa03981c3f93f5ecb3d628b4d68e7e779c954
plan_head_after: 8cc83a26ed7a39f6ed21e0ad12ca825264a425c3
commits: 3

actuals:
  tokens: 10103
  tasks: 3
  commits: 3

duration: 17min
completed: 2026-11-23
---

# Phase 28 Plan 06: Safeguard Suite, End-to-End Dispatcher Proof, AGENTS.md & Phase Gates Summary

**Phase 28 cross-cutting safeguard suite (`TestPhase28Codes`, `TestPhase28ReadsCreateNothing`, `TestPhase28StaffPrivacyAndLogs`), real-dispatcher end-to-end proof (`TestAdvancedThresholdsEndToEnd`) across UI edits/live events/resets, complete `AGENTS.md` rulebook update (`P28-01..P28-04`), and green local phase gates**

## Performance

- **Duration:** 17 min
- **Started:** 2026-11-23T12:42:05Z
- **Completed:** 2026-11-23T12:59:21Z
- **Tasks:** 3
- **Files modified:** 3 (2 created, 1 modified)

## Accomplishments

- Created `fuku/modules/settings_phase28_test.go` with `TestPhase28Codes` (all 5 `av` codes `aa`/`ao`/`as`/`ar`/`ax`, all 28 registered pages, step-1 refusal of extra keys and group-menu presses with 0 `getChatMember` calls), `TestPhase28ReadsCreateNothing` (opening Main, `av`, all 6 areas, and all 12 items on an unconfigured chat creates 0 rows in `chat_advanced_thresholds` and `chats`), and `TestPhase28StaffPrivacyAndLogs` (zero Staff Group or title/user leaks across rendered screens, answers, buttons, and captured logrus output including forced DB read errors, plus `av` hidden in Staff Group private and group Main).
- Created `fuku/modules/advthresholds_e2e_test.go` with `TestAdvancedThresholdsEndToEnd` backed by `phase28InProcessMiniredis` (zero localhost TCP dials) driving the real dispatcher across:
  1. Flood window UI edit (`av` → `aa rw` → `ao flw` → `as flw=120`) causing 6 messages from 3 recent joiners spanning 50 s to trigger an automatic flood lockdown (`TriggerKind == flood`, `TriggerWindow == 120`).
  2. Lockdown reminder UI edit (`as lrm=15`) causing an alert aged 16 min to post reminder `seq = 1` to both the Staff Group and the log channel.
  3. Extended captcha timeout UI edit (`as ctm=30`) giving the next joiner `ExpiresAt` ~30 min out and challenge text containing `30`.
  4. Area reset (`ax rw`) and item resets (`ar lrm`, `ar ctm`) returning `chat_advanced_thresholds` to `0` (`Default` on Main) and restoring default 30 s flood window (50 s span no longer fires), 60 min reminder interval (16 min plans 0 reminders), and 2 min default captcha timeout.
- Updated `AGENTS.md` with Phase 28's zero new shutdown drain note, `chat_advanced_thresholds` schema/`skipLocal`/backup exclusion, Advanced settings page `av` & runtime wiring invariants (5 `raidRecentWindow` sites, `memberreq_ai.go` cross-chat singleflight fix, Option A `captcha_timeout_mins`), Phase 28 test/golden/drift-guard families, and provisional owner decisions `P28-01..P28-04`.
- Verified all local phase gates (`make check-translations`, `make check-docs`, `go vet -tags testtools ./...`, `CGO_ENABLED=0 go build ./...`, `make test-shard-check`), all Phase 28 DB/cache/backup/i18n/module/EVT-10/golden suites under `-race`, Wave 1 golden immutability (`1` commit `48d5c240` since `654c8311`), and `go.mod`/`go.sum` untouched.

## Task Commits

1. **Task 1: add Phase 28 codes, non-creating reads, and Staff Group privacy / log sweep tests** - `5e2da306` (`test(28-06): add Phase 28 codes, non-creating reads, and Staff Group privacy / log sweep tests`)
2. **Task 2: add end-to-end dispatcher proof driving private UI edits into live raid, reminder, and captcha behavior** - `93b8d263` (`test(28-06): add end-to-end dispatcher proof for Phase 28 advanced thresholds`)
3. **Task 3: update AGENTS.md and run all Phase 28 verification & immutability gates** - `8cc83a26` (`docs(28-06): document Phase 28 advanced thresholds, skipLocal, page av, goldens, and P28-01..P28-04 in AGENTS.md`)

## Files Created/Modified

- `fuku/modules/settings_phase28_test.go` — `TestPhase28Codes`, `TestPhase28ReadsCreateNothing`, and `TestPhase28StaffPrivacyAndLogs` (in-process miniredis, zero localhost TCP dials).
- `fuku/modules/advthresholds_e2e_test.go` — `TestAdvancedThresholdsEndToEnd` driving real dispatcher callbacks, flood watchers, lockdown reminder worker, captcha joiner flow, and resets (in-process miniredis, zero localhost TCP dials).
- `AGENTS.md` — Phase 28 zero-drain note, `chat_advanced_thresholds` + `skipLocal` + backup exclusion, page `av` (`aa`/`ao`/`as`/`ar`/`ax`) + runtime wiring, test/golden/drift-guard registry, and provisional decisions `P28-01..P28-04`.

## Decisions Made

- Used real locales (`withRealLocales(t)` + `uk` translator) in `TestAdvancedThresholdsEndToEnd` so `%d`-formatted captcha challenge timeouts (`<b>30` vs `<b>2`), lockdown reminder headers, and Main state lines are exercised with real production strings rather than synthetic markers.
- Recorded all four provisional owner decisions (`P28-01..P28-04`) concisely in `AGENTS.md` alongside Phase 27's `P27-01..P27-05`.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- Pre-existing `TestSettingsStaffPrivacy` and `TestMemberDeletionExcludedMessages` use localhost TCP miniredis (requiring loopback outside the sandbox), and `TestMemberDeletionExcludedMessages/a_captcha_challenge` requires `TZ=UTC` on non-UTC hosts because SQLite's `chk_captcha_expires_at` (`expires_at > created_at`) compares `created_at` (GORM local time) lexicographically against `expires_at` (UTC). Under `TZ=UTC` with loopback enabled, all 13 module suites (including `TestMemberDeletionExcludedMessages`) and `TestSettingsStaffPrivacy` pass cleanly under `-race`. Every new Phase 28 test (`settings_phase28_test.go`, `advthresholds_e2e_test.go`) uses `phase28InProcessMiniredis` and UTC `NowFunc` so they pass inside the sandbox regardless of host TZ with zero localhost TCP dials.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 28 implementation, safeguards, end-to-end dispatcher proof, documentation, and local gates are complete and green.
- Ready for Phase 28 verification (`/gsd-verify-work` / phase verifier).

## Self-Check: PASSED

- Created/modified files verified on disk:
  - `fuku/modules/settings_phase28_test.go` — FOUND
  - `fuku/modules/advthresholds_e2e_test.go` — FOUND
  - `AGENTS.md` — FOUND
- Task commits verified as ancestors of HEAD with 0 accidental file deletions: `5e2da306`, `93b8d263`, `8cc83a26`.
- Acceptance criteria verified:
  - `TestPhase28Codes`, `TestPhase28ReadsCreateNothing`, `TestPhase28StaffPrivacyAndLogs` PASS count = `3`, `TestSettingsStaffPrivacy` PASS under `-race`.
  - `TestAdvancedThresholdsEndToEnd` PASS count = `1` under `-race` (in-process, zero localhost TCP dials).
  - `grep -c 'chat_advanced_thresholds' AGENTS.md` → `8` (>= 2); `grep -c 'P28-04' AGENTS.md` → `2` (>= 1).
  - Wave 1 golden `fuku/modules/advthresholds_phase28_legacy_request_test.go` commits since `654c8311` → `1` (`48d5c240`).
  - `git diff --exit-code 654c8311 -- go.mod go.sum` → exit `0` (untouched).
  - `make check-translations && make check-docs && go vet -tags testtools ./... && CGO_ENABLED=0 go build ./... && make test-shard-check` → all exit `0`.
  - `go test -tags testtools -race -count=1 ./fuku/db/advthresholds ./fuku/db/migrations`, `TestSkipLocal` (`./fuku/db/cache`), `TestAdvThresholdsExcludedFromBackup` (`./fuku/db/backup`), `TestSettingsLocaleKeys|TestSettingsLabelLengths|TestLocaleStyle|TestLocalePlaceholdersMatchOtherLanguages` (`./fuku/i18n`), and all 13 Phase 28 + EVT-10 + golden suites in `./fuku/modules` → all PASS under `-race`.
