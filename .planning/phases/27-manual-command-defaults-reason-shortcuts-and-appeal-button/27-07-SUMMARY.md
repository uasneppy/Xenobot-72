---
phase: 27-manual-command-defaults-reason-shortcuts-and-appeal-button
plan: 07
subsystem: verification
status: complete
tags: [safeguards, e2e, privacy, logs, docs, phase-gates, tune-04, tune-05, tune-06]
requires:
  - 27-01 (immutable TestLegacyPhase27ModerationRequests golden)
  - 27-02 (chat_manual_defaults table, default durations, reason_required)
  - 27-03 (chat_reason_shortcuts table, reason shortcut expansion)
  - 27-04 (appeal URL validation and ban/PM appeal button)
  - 27-05 (Manual defaults settings page q)
  - 27-06 (Reason shortcuts list page rs)
provides:
  - Phase 27 safeguard tests (TestPhase27Codes, TestPhase27ReadsCreateNothing, TestPhase27StaffPrivacyAndLogs) in fuku/modules/settings_phase27_test.go
  - End-to-end dispatcher proof (TestManualDefaultsEndToEnd) in fuku/modules/modtuning_e2e_test.go
  - Updated AGENTS.md documenting Phase 27 zero shutdown drain, tables, skipLocal prefixes, pages q & rs, action codes, immutable golden, and provisional decisions P27-01..P27-05
  - Green local Phase 27 verification gate suite (translations, docs, vet, CGO_ENABLED=0 build, test-shard-check, golden & go.mod immutability, all targeted DB/cache/migration/backup/i18n/module suites)
affects: []
tech-stack:
  added: []
  patterns:
    - "Cross-layer privacy & IDs-only logrus sweep (TestPhase27StaffPrivacyAndLogs) combining Staff Group title/ID sentinels, shortcut word/reason sentinels, and appeal URL sentinels across private settings, group /ban, and Staff Group Main"
    - "Full dispatcher E2E proof (TestManualDefaultsEndToEnd) over newPMCopiesE2E(t, true) + LoadWarns + logchannels.Set verifying configure -> enforce -> clear -> restored permanent default ban behavior"
key-files:
  created:
    - fuku/modules/settings_phase27_test.go
    - fuku/modules/modtuning_e2e_test.go
  modified:
    - AGENTS.md
decisions:
  - "Documented provisional owner decisions P27-01..P27-05 in AGENTS.md alongside Phase 27 storage, skipLocal prefixes, settings pages q & rs, action codes, zero shutdown drain, and immutable golden TestLegacyPhase27ModerationRequests."
  - "Recorded PostgreSQL-only integrity tests (make test-postgres-integrity), golangci-lint (make lint), and gosec as CI-required checks since those tools/services are not installed locally."
metrics:
  duration: 22min
  completed: 2026-11-23
actuals:
  tokens: 11616
  tasks: 3
  commits: 3
plan_head_before: 0f03bb24dc8a48b72b48626f9cf09c271e8b9f1c
plan_head_after: 5fbf4d85058afccb9534ebce4264a04b6f07c301
---

# Phase 27 Plan 07: Phase 27 Safeguards, End-to-End Proof, AGENTS.md, and Gate Verification Summary

Completed Phase 27 with cross-cutting safeguard tests (`TestPhase27Codes`, `TestPhase27ReadsCreateNothing`, `TestPhase27StaffPrivacyAndLogs`), the full dispatcher end-to-end proof (`TestManualDefaultsEndToEnd`), rulebook updates in `AGENTS.md` covering all new tables, `skipLocal` prefixes, pages `q` & `rs`, action codes, zero shutdown drain, immutable golden, and provisional decisions `P27-01..P27-05`, and a clean pass of every local Phase 27 verification gate.

## Performance

- **Duration:** 22 min (includes initial sandbox loopback timeout before enabling loopback for `miniredis`)
- **Started:** 2026-11-23T03:25:28Z
- **Completed:** 2026-11-23T03:47:22Z
- **Tasks:** 3
- **Files created:** 2
- **Files modified:** 1

## Accomplishments

- **Task 1 (`c709b37d`):** Created `fuku/modules/settings_phase27_test.go` (`//go:build testtools`) with:
  - `TestPhase27Codes`: proves all 11 Phase 27 action codes (`md`, `mt`, `mr`, `ma`, `mc`, `mx` -> `q`; `sg`, `sx`, `sy`, `sa`, `sc` -> `rs`) map to their page, are no core code, match the exact registered action set on `q`/`rs`, leave all 27 registered pages intact, and refuse both extra keys and group-menu presses (`u` instead of `c`) in step 1 with 0 `getChatMember` calls.
  - `TestPhase27ReadsCreateNothing`: proves opening Main, page `q`, its ban/mute duration screens, and page `rs` (private and group menus) on an unconfigured chat creates 0 rows in `chat_manual_defaults`, `chat_reason_shortcuts`, and `chats`.
  - `TestPhase27StaffPrivacyAndLogs`: links a group to a Staff Group with sentinel title `"SecretStaffSentinel"`, exercises pages `q` and `rs` with sentinel shortcut `"secretword"` / `"Secret full <reason>"` and appeal URL `"https://secret-appeal.example.test/form"`, executes `/ban secretword`, and verifies zero Staff Group leakage into UI output, zero shortcut/appeal/Staff Group leakage into captured logrus lines, and pages `q`/`rs` never appearing in a Staff Group's private or group Main.
- **Task 2 (`19f460a6`):** Created `fuku/modules/modtuning_e2e_test.go` (`//go:build testtools`) with `TestManualDefaultsEndToEnd`, driving the real dispatcher through the complete 8-step lifecycle:
  1. Owner configures default `/ban` duration (`3600` s), Reason required (`1`), and Appeal link (`https://example.com/appeal`) on page `q` in private `/settings`.
  2. Owner adds `spam` → `"Repeated <promo> & links"` via two-step typed reply on page `rs` and confirms HTML-escaped rendering (`<code>spam</code> → Repeated &lt;promo&gt; &amp; links`).
  3. Admin runs `/ban` without a reason: refused with `moddef_reason_required`, 0 `banChatMember` calls, 0 PM copies, 0 log posts.
  4. Admin runs `/ban spam`: `banChatMember` receives `until_date = fixedNow + 3600`, group reply uses `EventTimedBan` with escaped reason + Appeal URL button, PM copy delivers escaped reason + Appeal URL button, and log channel receives escaped reason.
  5. Admin runs `/warn spam`: warn reply and stored warn record carry the expanded reason.
  6. Admin runs `/tban 2h other`: applies explicit `7200` s (not `3600`), preserves unknown word `"other"` verbatim, and carries Appeal URL button on group reply and PM copy.
  7. Owner deletes `spam` behind the two-tap card (`sx` → `sy`), resets default `/ban` duration to Permanent (`0`), turns Reason required off (`0`), and removes Appeal link (`mx`).
  8. Admin runs `/ban` without a reason: permanent `banChatMember` (no `until_date`), default `EventBan` reply with Unban button (`unrestrict|v1|a=unban&u=...`) only and no Appeal button on group reply or PM copy.
- **Task 3 (`5fbf4d85`):** Updated `AGENTS.md` with scoped edits documenting Phase 27's zero shutdown drain (`stop_grace_period: 75s` unchanged), `chat_manual_defaults` & `chat_reason_shortcuts` (migration `20261122150000_add_manual_command_tuning.sql`, `skipLocal` prefixes `chat_manual_defaults:` & `chat_reason_shortcuts:`, backup exclusion), pages `q` & `rs` and all 11 action codes + `WaitEndedKey` + plain-text `settingsListTwoStep`, immutable golden `TestLegacyPhase27ModerationRequests`, Phase 27 test families, and provisional owner decisions `P27-01..P27-05`.

## Task Commits

1. **Task 1: Phase 27 safeguard tests** — `c709b37d` (`test(27-07): add Phase 27 codes, non-creating reads, and Staff privacy/log sweep tests`)
2. **Task 2: End-to-end dispatcher proof** — `19f460a6` (`test(27-07): add end-to-end dispatcher proof TestManualDefaultsEndToEnd`)
3. **Task 3: AGENTS.md update** — `5fbf4d85` (`docs(27-07): document Phase 27 tables, skipLocal keys, pages q & rs, goldens, and P27-01..P27-05 in AGENTS.md`)

## Verification Results

All local Phase 27 gates passed with zero failures or skips:

- `make check-translations` — PASS (1068 keys present across all 8 locales).
- `make check-docs` — PASS (no drift detected).
- `go vet -tags testtools ./...` — PASS.
- `CGO_ENABLED=0 go build ./...` — PASS.
- `make test-shard-check` — PASS (0 diff across shards).
- `go test -tags testtools -race -count=1 ./fuku/db/modtuning ./fuku/db/cache ./fuku/db/migrations` — PASS (`modtuning` 1.401s, `cache` 2.516s, `migrations` 1.518s).
- `go test -tags testtools -race -count=1 -run '^TestModTuningExcludedFromBackup$' ./fuku/db/backup` — PASS (1.458s).
- `go test -tags testtools -count=1 -run '^(TestSettingsLocaleKeys|TestSettingsLabelLengths|TestEventMsgLocaleKeys|TestLocaleStyle|TestLocalePlaceholdersMatchOtherLanguages)$' ./fuku/i18n` — PASS (0.554s).
- `go test -tags testtools -race -count=1 -run '^(TestPhase27Codes|TestPhase27ReadsCreateNothing|TestPhase27StaffPrivacyAndLogs|TestSettingsStaffPrivacy)$' -v ./fuku/modules` — PASS (11.890s, 3/3 `TestPhase27*` + full `TestSettingsStaffPrivacy`).
- `go test -tags testtools -race -count=1 -run '^TestManualDefaultsEndToEnd$' -v ./fuku/modules` — PASS (0.87s).
- `go test -tags testtools -race -count=1 -run '^(TestLegacyPhase27ModerationRequests|TestLegacyModerationRequests|TestManualCommandDefaults|TestReasonShortcutValidationAndApply|TestReasonShortcutExpansion|TestAppealURLValidationAndApply|TestAppealButtonOnBanMessages|TestSettingsModDefaults|TestSettingsReasons|TestPhase27Codes|TestPhase27ReadsCreateNothing|TestPhase27StaffPrivacyAndLogs|TestManualDefaultsEndToEnd|TestEventEngineCallers|TestPMCopyCallers|TestSettingsListKeysExist|TestSettingsMain|TestSettingsFwdBlockMainShape|TestSettingsPhase19)$' ./fuku/modules` — PASS (4.245s).
- Golden & dependency immutability:
  - `git log --oneline b7393953..HEAD -- fuku/modules/moderation_phase27_legacy_request_test.go` — exactly 1 commit (`71dbf865`).
  - `git diff --exit-code b7393953 -- fuku/modules/moderation_legacy_request_test.go go.mod go.sum` — exit `0` (untouched).
- `AGENTS.md` grep checks: `chat_manual_defaults` (`3`), `chat_reason_shortcuts` (`4`).
- **CI-required checks (not installed locally):** `make test-postgres-integrity` (PostgreSQL-only integrity tests), `make lint` (`golangci-lint`), and `gosec`.

## Deviations from Plan

None - plan executed exactly as written.

## Known Stubs

None.

## Next Phase Readiness

- Phase 27 (`27-01` through `27-07`) is complete and ready for phase verification (`/gsd-verify-work` / phase verifier).

## Self-Check: PASSED

- Created/modified files verified on disk: `fuku/modules/settings_phase27_test.go`, `fuku/modules/modtuning_e2e_test.go`, `AGENTS.md` — ALL FOUND.
- Task commits verified as ancestors of HEAD: `c709b37d`, `19f460a6`, `5fbf4d85` — ALL FOUND.
