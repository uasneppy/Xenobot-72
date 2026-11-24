---
phase: 29-plain-tone-and-copy-settings
plan: 06
subsystem: phase29-safeguards-e2e-docs
tags: [plain-tone, copy-settings, tune-08, tune-09, safeguards, e2e, staff-privacy, log-hygiene, agents-md]

requires:
  - phase: 29-plain-tone-and-copy-settings
    plan: 05
    provides: Private Copy Settings page cp (Order 82, actions cg/ck/cy), live dual-group verification, and 8-locale UI
provides:
  - Phase 29 safeguard tests (TestPhase29Codes, TestPhase29ReadsCreateNothing, TestPhase29StaffPrivacyAndLogs) in fuku/modules/settings_phase29_test.go
  - Real-dispatcher end-to-end proof (TestPhase29EndToEnd) for Plain tone (TUNE-08) and Copy Settings (TUNE-09) in fuku/modules/phase29_e2e_test.go
  - Updated AGENTS.md rulebook documenting Phase 29 schema, eventmsg.ReplaceAllValidated, antiflood.SetFloodActionPreserveOffContext, fuku:settings:copy:<token>, page cp, toggle ht, zero new drains, test suites, and provisional decisions P29-01..P29-04
affects: [30]

tech-stack:
  added: []
  patterns:
    - "Phase-wide action-code, non-creating-read (across all 10 tables in scope), Staff Group privacy, and IDs-only logrus capture sweep backed by in-process miniredis (zero localhost TCP dials)"
    - "Real ext.Dispatcher E2E test driving both Plain tone (ht toggle, live /ban group & PM plain copy, custom text precedence) and Copy Settings (cp/ck/cy, 6-area mirror, P29-01..P29-04 invariants, excluded tables, non-linking, and one-time cy replay protection)"

key-files:
  created:
    - fuku/modules/settings_phase29_test.go
    - fuku/modules/phase29_e2e_test.go
  modified:
    - AGENTS.md

key-decisions:
  - "Backed TestPhase29Codes, TestPhase29ReadsCreateNothing, TestPhase29StaffPrivacyAndLogs, and TestPhase29EndToEnd with phase28InProcessMiniredis so the entire Phase 29 safeguard and E2E suite executes under -race in < 2s with zero localhost TCP socket dials."
  - "Verified captured logrus output across Plain tone toggles, plain renders, and Copy Settings ask/refusal/confirm/copy against 10 distinctive sentinels (source/target/Staff Group titles, owner name, custom message text, media file_id, button URL, appeal URL, reason shortcut, and the 16-char one-time Redis copy token)."
  - "Documented provisional owner decisions P29-01..P29-04, shared validated writer eventmsg.ReplaceAllValidated (for reuse by Phase 30 MSG-10), antiflood.SetFloodActionPreserveOffContext, and fuku:settings:copy:<token> in AGENTS.md."

patterns-established:
  - "Cross-group feature log-hygiene sweep pattern: assert both positive completion log presence ([CopySettings] completed copy) and zero occurrences of any sentinel PII, content, URL, or one-time Redis token."

requirements-completed: [TUNE-08, TUNE-09]

status: complete
plan_head_before: 59ac85fef4029e0e3ef2d13d6cccc13867d84265
plan_head_after: 92fc756b10f4a1d1b5ce9951d90a511182392ef5
commits: 3

actuals:
  tokens: 14660
  tasks: 3
  commits: 3

duration: 24min
completed: 2026-11-23
---

# Phase 29 Plan 06: Phase 29 Safeguards, Real-Dispatcher E2E Proof & Rulebook Update Summary

**Phase 29 safeguard suite (`TestPhase29Codes`, `TestPhase29ReadsCreateNothing`, `TestPhase29StaffPrivacyAndLogs`), real-dispatcher E2E proof (`TestPhase29EndToEnd`) covering Plain tone (`TUNE-08`) and Copy Settings (`TUNE-09`), and `AGENTS.md` rulebook documentation of `P29-01..P29-04` and shared Phase 29/30 primitives**

## Performance

- **Duration:** 24 min
- **Started:** 2026-11-23T15:25:00Z
- **Completed:** 2026-11-23T15:48:30Z
- **Tasks:** 3
- **Files modified:** 3 (2 created, 1 modified; `fuku/modules/phase29_legacy_request_test.go`, pre-existing `*_legacy_request_test.go` goldens, `go.mod`, and `go.sum` strictly untouched)

## Accomplishments

- **Phase 29 Safeguard Tests (`59a4070f`):**
  - Created `fuku/modules/settings_phase29_test.go` with `TestPhase29Codes`, `TestPhase29ReadsCreateNothing`, and `TestPhase29StaffPrivacyAndLogs`.
  - `TestPhase29Codes` verifies all 29 registered settings pages, action code `ht` on `h` and `cg`/`ck`/`cy` on private-only page `cp`, zero collisions with `settingsCoreActions` or other pages, and step-1 refusal of extra unknown fields or group-menu presses (`u` instead of `c`) with 0 `getChatMember` calls.
  - `TestPhase29ReadsCreateNothing` proves opening Main, Messages hub (`h`), and Copy Settings (`cp` and `cg`) in both group and private menus on an unconfigured group creates 0 rows across all 10 tables in scope (`chat_event_messages`, `chat_event_message_options`, `warns_settings`, `antiflood_settings`, `blacklists`, `chat_escalation`, `chat_manual_defaults`, `chat_reason_shortcuts`, `chat_advanced_thresholds`, and `chat_settings_access`).
  - `TestPhase29StaffPrivacyAndLogs` proves Staff Group hiding of `cp`, uniform opaque target refusal (`settings_pick_unavailable`) when picking a Staff Group or inaccessible group, zero Staff Group title/ID leakage in any UI screen or plain-tone render, and zero leakage of group titles, user names, message texts, file IDs, button URLs, appeal URLs, reason shortcuts, or the 16-char Redis copy token across captured debug-level `logrus` output.
- **Real-Dispatcher End-to-End Proof (`f12a0bd6`):**
  - Created `fuku/modules/phase29_e2e_test.go` (`TestPhase29EndToEnd`) driving a real `ext.Dispatcher` with `LoadSettings`, `LoadBans`, `LoadMutes`, `LoadWarns`, fake Telegram HTTP transport, and in-process `miniredis` (`phase28InProcessMiniredis`).
  - **Part 1 (Plain tone E2E, `TUNE-08`):** Toggles Plain tone ON via `ht` (`v=1`) on the private Messages hub (`h`), enables PM copy for `EventBan`, runs `/ban` in the source group to prove neutral plain-tone group reply (preserving the Unban button row) and neutral plain-tone PM copy, then sets a custom `EventBan` group text and proves custom text precedence on `/ban` while `/kick` continues posting neutral plain tone.
  - **Part 2 (Copy Settings E2E, `TUNE-09`):** Configures all 6 areas on `srcChat` and pre-existing state on `dstChat` (no `antiflood_settings` row so `Limit == 0`, own blacklist word `"dstword"`, plus excluded `approved_users`, `warns_users`, and `staff_group_links` rows), drives `cp` -> `ck` (confirm card naming escaped titles and all 6 areas) -> `cy` (copy completion), and proves live target behavior on `dstChat` (`/kick sp` plain tone + expanded shortcut, `/ban` refused by copied `ReasonRequired`, `/ban sp` posting copied custom ban template + expanded shortcut + appeal button + plain PM copy, `antiflood_settings.Limit == 0` preserved, only `"dstword"` kept with copied `"tban", 3600`, excluded tables untouched, subsequent target edits not affecting source, and replayed `cy` returning `settings_card_expired` without modifying target).
- **Rulebook Update & Phase Gates (`92fc756b`):**
  - Updated `AGENTS.md` across Startup/shutdown, Settings pages, Redis keys, Database tables, and Phase 29 test families to document migration `20261123130000_add_event_message_plain_tone.sql`, `eventmsg.ReplaceAllValidated`, `antiflood.SetFloodActionPreserveOffContext`, `fuku:settings:copy:<token>`, toggle `ht`, page `cp` (Order `82`, `cg`/`ck`/`cy`), zero new shutdown drains, and provisional owner decisions `P29-01..P29-04`.
  - Passed `make check-translations`, `make check-docs`, `go vet -tags testtools ./...`, `CGO_ENABLED=0 go build ./...`, `make test-shard-check`, all Phase 29 DB + i18n + module `-race` test suites, and immutability checks on `phase29_legacy_request_test.go`, pre-existing goldens, and `go.mod`/`go.sum`.

## Task Commits

1. **Task 1: add Phase 29 codes, non-creating reads, and Staff Group privacy / IDs-only log sweep tests** - `59a4070f` (`test(29-06): add Phase 29 codes, non-creating reads, and Staff Group privacy / IDs-only log sweep tests`)
2. **Task 2: add real-dispatcher E2E proof for Plain tone and Copy Settings** - `f12a0bd6` (`test(29-06): add real-dispatcher E2E proof for Plain tone and Copy Settings`)
3. **Task 3: update AGENTS.md and run all Phase 29 verification & immutability gates** - `92fc756b` (`docs(29-06): document Phase 29 Plain tone, Copy Settings, shared validated writer, and P29-01..P29-04 in AGENTS.md`)

## Files Created/Modified

- `fuku/modules/settings_phase29_test.go` — `TestPhase29Codes`, `TestPhase29ReadsCreateNothing`, and `TestPhase29StaffPrivacyAndLogs`.
- `fuku/modules/phase29_e2e_test.go` — `TestPhase29EndToEnd` real-dispatcher E2E test for Plain tone (`TUNE-08`) and Copy Settings (`TUNE-09`).
- `AGENTS.md` — Phase 29 architecture, schema, Redis key, settings page, test suite, and `P29-01..P29-04` documentation.

## Decisions Made

- Used canonical short chat ID `t=-100123` in `TestPhase29Codes` step-1 extra-field test (`a=ck&c=<chat>&t=-100123&x=1`) so the encoded callback stays under Telegram's 64-byte cap and reaches the step-1 unknown-field validator.
- Used Ukrainian group translator (`ukTr`, matching the default uncached group language in `newPhase28SettingsEnv`) alongside English private menu translator (`enTr`) in `TestPhase29EndToEnd` to verify both group moderation replies and private settings cards cleanly with zero TCP dials.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- Running inside the macOS sandbox blocks localhost TCP socket dials (`miniredis.RunT` in older pre-Phase-28 tests); all Phase 29 tests in `settings_phase29_test.go`, `phase29_e2e_test.go`, `settings_copy_test.go`, `copy_settings_test.go`, and `event_message_plain_test.go` use in-process `phase28InProcessMiniredis` or memory marshalers and pass under `-race` with zero TCP dials.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Phase 29 (`TUNE-08` Plain tone and `TUNE-09` Copy Settings) is complete, verified end-to-end under `-race`, and documented in `AGENTS.md`.
- `eventmsg.ReplaceAllValidated` (`fuku/db/eventmsg/validate_write.go`) is tested, drift-guarded against `settings_messages.go`, and ready for Phase 30 (`MSG-10` backup export/import/reset integration).

## Self-Check: PASSED

- Created/modified files exist: `fuku/modules/settings_phase29_test.go`, `fuku/modules/phase29_e2e_test.go`, `AGENTS.md`
- Commits exist and are ancestors of `HEAD`: `59a4070f`, `f12a0bd6`, `92fc756b` (`commits: 3` matching `git rev-list --count 59ac85fef4029e0e3ef2d13d6cccc13867d84265..92fc756b10f4a1d1b5ce9951d90a511182392ef5`)
- Immutability verified: `fuku/modules/phase29_legacy_request_test.go` has exactly 1 commit since `ca060519`, pre-existing `*_legacy_request_test.go` files have 0 diffs, and `go.mod`/`go.sum` have 0 diffs
- All local gates passed: `make check-translations`, `make check-docs`, `go vet -tags testtools ./...`, `CGO_ENABLED=0 go build ./...`, `make test-shard-check`, DB suites, i18n suites, and Phase 29 module suites under `-race`
