---
phase: 26-timed-punishments-escalation-and-warn-expiry
plan: 12
subsystem: testing
status: complete
tags: [e2e, timed-punishment, escalation, warn-expiry, gates]
requires:
  - 26-11
provides:
  - TestTimedPunishmentsEndToEnd (8 subtests through the real dispatcher)
affects: []
requirements: [TUNE-01, TUNE-02, TUNE-03]
tech-stack:
  added: []
  patterns:
    - "phase end-to-end proof presses buttons the bot drew; Telegram and the clock are the only fakes"
key-files:
  created:
    - fuku/modules/timed_punishments_e2e_test.go
  modified:
    - AGENTS.md
decisions:
  - "The full fuku/modules failures that remain under TZ=UTC are the three timezone tests that already fail on 78648d16. They are recorded, not treated as Phase 26 regressions."
  - "make lint, gosec and make test-postgres-integrity were not run. No golangci-lint, gosec or PostgreSQL was available. They stay human-needed and are not marked passed."
metrics:
  duration: recorded from the paused run
  completed: 2026-11-22
actuals:
  tokens: 5081
  tasks: 3
  commits: 1
plan_head_before: 5046db0a
plan_head_after: d6793499
---

# Plan 26-12: end-to-end proof and phase gates

**The owner's timed-punishment, ladder and warn-expiry flows run through the real dispatcher.** The repository gates that need lint, gosec or PostgreSQL were not run here.

Closed out from commit `d6793499` and the pause record. No second executor was dispatched.

## What landed

Task 1 is `d6793499` (`test(26-12): drive timed punishments, the ladder and warn expiry end to end`). `TestTimedPunishmentsEndToEnd` has eight subtests: `warn_limit_page`, `flood_command`, `blacklist_page`, `ladder`, `warn_expiry`, `pm_until`, `staff_group`, `nothing_set`. AGENTS.md records the test at about 3 s and the rule that `make test-shard-check` must run from a path with no space.

## Gates recorded on that run

Passed, as recorded in `.continue-here.md`:

- Each plan 26-01 golden file has one commit. Confirmed again for `timed_punishment_legacy_request_test.go` and `punish_export_legacy_test.go`.
- The Phase 21–25 golden files have no diff since `b7662c7`, and those tests passed.
- `TestLegacyPunishmentExport` passed.
- The migration and repository test names list (`TestTimedPunishmentMigrationDefaults`, `TestWarnExpiryMigrationDefaults`, `TestEscalationMigrationDefaults`, `TestSetWarnModeDuration`, `TestWarnExpiryWrites`, `TestEscalationSettings`, `TestOffenceCountConcurrent`).
- `make test-shard-check` passed from `/tmp/xeno72`.
- `make check-docs`, `make check-translations` and `go vet ./...` passed.
- The Phase 26 `fuku/db` packages passed with `-race`.

Not a clean full package:

- `go test -tags testtools -race -count=1 ./fuku/modules` took 247 s and failed 82 tests, the same set as on `78648d16` (before this session's Phase 26 work).
- The same package with `TZ=UTC` took 236 s. The only failures were `TestChatClockReadsTheGroup`, `TestApplyTimezone` and `TestSettingsTimezoneTyped`. All three also fail on `78648d16`.

Not run, and not marked passed:

- `make lint` and gosec. golangci-lint and gosec are not installed.
- `make test-postgres-integrity`. No `DATABASE_URL` and no PostgreSQL. `TestOffenceCountConcurrent` and the three `*MigrationDefaults` tests have not run against PostgreSQL. This is human-needed for the verifier (T-26-34).

## Deviations from Plan

The plan's acceptance line says the full `fuku/modules` run passes. It does not, on this machine, for the reason above. The three timezone failures predate Phase 26.

**Total deviations:** 0 production changes in this close-out.
**Impact on plan:** the end-to-end test is committed. The lint, gosec and PostgreSQL gates remain open.

## Issues Encountered

None in the end-to-end test itself. The close-out did not re-run it. The pause record is the source for the gate times and the failure names.

## User Setup Required

None for the test. A machine with PostgreSQL, golangci-lint and gosec still has to run `make test-postgres-integrity`, `make lint` and gosec before those gates can pass.

## Next Phase Readiness

Phase 26's plans all have summaries. Verification is the remaining gate, and it should keep the PostgreSQL, lint and gosec runs as human-needed.
