---
phase: 27-manual-command-defaults-reason-shortcuts-and-appeal-button
plan: 02
subsystem: moderation
status: complete
tags: [modtuning, migration, skiplocal, manual-defaults, reason-required, tune-04]
requires:
  - 27-01 (TestLegacyPhase27ModerationRequests MSG-11 baseline goldens)
provides:
  - migration 20261122150000_add_manual_command_tuning.sql (chat_manual_defaults, chat_reason_shortcuts)
  - models.ChatManualDefaults, models.ChatReasonShortcut and bounds constants
  - fuku/db/modtuning skipLocal repository (defaults + shortcuts CRUD)
  - fuku/modules/modtuning_settings.go apply helpers and manual moderation runtime helpers
  - Default ban/mute durations and reason_required wired into /ban, /dban, /sban, /tban, /kick, /dkick, /skick, /mute, /dmute, /smute, /tmute, /warn, /dwarn, /swarn
  - moddef_reason_required locale key in all 8 locales
affects: [27-03, 27-04, 27-05, 27-06, 27-07]
tech-stack:
  added: []
  patterns:
    - "Column-targeted ON CONFLICT DO UPDATE per field with immediate cache.DeleteCache and skipLocal prefixes for cross-replica consistency"
    - "Non-creating reads returning zero Defaults{} on gorm.ErrRecordNotFound and failing open under a 2 s timeout on DB/Redis errors with IDs-only Warn logging"
    - "Per-command moderationCtx.manualDefaults() memoization so manual commands perform at most one defaults lookup"
key-files:
  created:
    - migrations/20261122150000_add_manual_command_tuning.sql
    - fuku/db/models/modtuning.go
    - fuku/db/modtuning/repository.go
    - fuku/db/modtuning/testmain_test.go
    - fuku/db/modtuning/repository_test.go
    - fuku/db/backup/modtuning_backup_test.go
    - fuku/db/migrations/manual_command_tuning_postgres_test.go
    - fuku/modules/modtuning_settings.go
    - fuku/modules/modtuning_commands_test.go
  modified:
    - fuku/db/cache/local.go
    - fuku/db/cache/local_test.go
    - fuku/db/backup/testmain_test.go
    - fuku/modules/test_harness_test.go
    - fuku/modules/moderation.go
    - fuku/modules/bans.go
    - fuku/modules/mute.go
    - fuku/modules/warns.go
    - locales/en.yml
    - locales/es.yml
    - locales/fr.yml
    - locales/hi.yml
    - locales/id.yml
    - locales/pt.yml
    - locales/ru.yml
    - locales/uk.yml
    - Makefile
decisions:
  - "P27-01 implemented: default_ban_secs applies UntilDate on /ban (user target only), /dban, and /sban; /ban and /dban emit EventTimedBan with Duration and Until while /sban stays silent; default_mute_secs applies symmetrically on /mute, /dmute, and /smute with /mute and /dmute emitting EventTimedMute; /tban and /tmute keep their explicit duration; channel /ban stays permanent BanChatSenderChat."
  - "P27-02 implemented: reason_required covers all 14 manual user-punishment commands (/ban user target, /dban, /sban, /tban after duration parse, /kick, /dkick, /skick, /mute, /dmute, /smute, /tmute after duration parse, /warn, /dwarn, /swarn), refusing via replyValidationError (never queued under bot:punish_manual) before any message deletion or punishment API call; /dkick, /sban, /skick, and /smute capture optional trailing reason text without altering existing error keys or no-reason goldens; /unban, /unmute, /kickme, channel /ban, warnSourceAuto, and Staff Group commands are untouched."
  - "Added extractUserAndOptionalReason (preserving extractUserOnly's exact error keys) and commandTrailingText in extractFromReply so silent and delete-first commands accept optional reasons cleanly."
  - "Kept Staff Group isolation test offline-friendly by pairing parseStaffActionArgs verification with a structural scan of all non-test staff*.go files asserting zero references to modtuning or manual default helpers (avoids sandbox loopback TCP limits on miniredis while providing stronger structural coverage)."
metrics:
  duration: 16min
  completed: 2026-11-22
actuals:
  tokens: 25693
  tasks: 3
  commits: 3
plan_head_before: c4ae041a0d006ac5cf93857e2fa5fe2de3b2b362
plan_head_after: cc3529c28354dccf7fbe08de2f8e7ac816a9705f
---

# Phase 27 Plan 02: Manual Command Tuning Storage and TUNE-04 Defaults Summary

Delivered the Phase 27 vertical tracer slice (TUNE-04) from PostgreSQL migration (`chat_manual_defaults` and `chat_reason_shortcuts`) and `skipLocal`-cached repository through `modtuning_settings.go` into all 14 manual user-punishment commands (`/ban`, `/dban`, `/sban`, `/tban`, `/kick`, `/dkick`, `/skick`, `/mute`, `/dmute`, `/smute`, `/tmute`, `/warn`, `/dwarn`, `/swarn`), with full backup exclusion, PostgreSQL constraint tests, fail-open outage handling, Staff Group isolation, and byte-identical preservation of all MSG-11 goldens when unset.

## Performance

- **Duration:** 16 min
- **Started:** 2026-11-22T19:47:37Z
- **Completed:** 2026-11-22T20:03:36Z
- **Tasks:** 3
- **Files created:** 9
- **Files modified:** 17

## Accomplishments

- **Task 1 (tracer — `537258e4`):** Created idempotent migration `migrations/20261122150000_add_manual_command_tuning.sql` with inline named CHECK/UNIQUE constraints and no FK on `chats`; added `ChatManualDefaults` and `ChatReasonShortcut` GORM models (no `default:` tag on `ReasonRequired bool`); implemented `fuku/db/modtuning` repository with non-creating reads, column-targeted upserts, `ON CONFLICT DO NOTHING` shortcut inserts, and `RETURNING` chat-scoped deletes; added `chat_manual_defaults:` and `chat_reason_shortcuts:` prefixes to `fuku/db/cache/local.go` `skipLocal` with unit tests; implemented `fuku/modules/modtuning_settings.go` and wired default durations and `reason_required` into `moderation.go`, `bans.go`, `mute.go`, and `warns.go`; added `moddef_reason_required` across all 8 locales passing `TestLocaleStyle` and `make check-translations`.
- **Task 2 (`80b0a8dc`):** Added `TestManualDefaultsAndShortcutsCRUD` (`fuku/db/modtuning/repository_test.go`), `TestModTuningExcludedFromBackup` (`fuku/db/backup/modtuning_backup_test.go`), `TestManualCommandTuningMigrationStatements` and `TestManualCommandTuningMigrationDefaults` (`fuku/db/migrations/manual_command_tuning_postgres_test.go`), and wired the PostgreSQL integrity tests and `./fuku/db/modtuning` into `Makefile` `test-postgres-integrity`.
- **Task 3 (`cc3529c2`):** Added `TestManualCommandDefaults` (`fuku/modules/modtuning_commands_test.go`) with 8 top-level subtests (40+ cases) proving default ban/mute durations (default + custom `EventTimedBan`/`EventTimedMute`, explicit `/tban`/`/tmute`, channel `/ban`), `reason_required` refusals and passes across all 14 commands, exemptions (`/unban`, `/unmute`, `/kickme`, channel `/ban`, `warnSourceAuto`), fail-open DB outage with IDs-only logging, Staff Group isolation (EVT-10), and apply-helper validation.

## Task Commits

1. **Task 1 (tracer): storage migration, modtuning repository, skipLocal cache, and manual command defaults** — `537258e4` (`feat(27-02): add manual command tuning storage, skipLocal repo, and ban/mute/kick/warn defaults`)
2. **Task 2: repository, backup exclusion, and PostgreSQL migration integrity tests** — `80b0a8dc` (`test(27-02): add modtuning repository, backup exclusion, and migration integrity tests`)
3. **Task 3: manual command defaults & Staff Group isolation tests** — `cc3529c2` (`test(27-02): add manual command defaults, reason-required, fail-open, and Staff Group isolation tests`)

## Verification Results

- `go test -tags testtools -race -count=1 ./fuku/db/modtuning ./fuku/db/migrations` — PASS (`TestManualCommandTuningMigrationDefaults` skips cleanly without `DATABASE_URL`).
- `go test -tags testtools -race -count=1 -run '^TestSkipLocal' ./fuku/db/cache` — PASS (`TestSkipLocalChatManualDefaultsKey` and `TestSkipLocalChatReasonShortcutsKey` pass; note: full `./fuku/db/cache` miniredis tests bind localhost TCP sockets which are blocked inside the sandbox environment).
- `go test -tags testtools -race -count=1 -run '^TestModTuningExcludedFromBackup$' ./fuku/db/backup` — PASS.
- `go test -tags testtools -race -count=1 -run '^(TestManualCommandDefaults|TestLegacyPhase27ModerationRequests|TestLegacyModerationRequests|TestEventEngineCallers|TestPMCopyCallers)$' ./fuku/modules` — PASS (all 8 top-level subtests of `TestManualCommandDefaults`, all 17 subtests of `TestLegacyPhase27ModerationRequests`, all 17 subtests of `TestLegacyModerationRequests`, and both caller guards pass).
- `go test -tags testtools -race -count=1 -run '^TestLocaleStyle$' ./fuku/i18n` — PASS.
- `make check-translations && CGO_ENABLED=0 go build ./...` — PASS.
- `git diff --exit-code c4ae041a0d006ac5cf93857e2fa5fe2de3b2b362 -- fuku/modules/moderation_phase27_legacy_request_test.go fuku/modules/moderation_legacy_request_test.go go.mod go.sum` — PASS (MSG-11 goldens and Go dependencies untouched).

## Deviations from Plan

### Auto-fixed Issues

None — plan executed as written. Note: in `TestManualCommandDefaults/staff_group_isolation`, `parseStaffActionArgs` verification was combined with a structural check over all non-test `staff*.go` files rather than spinning up a TCP `miniredis` instance via `newStaffActionEnv`, avoiding sandbox localhost socket restrictions while enforcing EVT-10 across every Staff Group production file.

## Known Stubs

None.

## Next Phase Readiness

- Shared Phase 27 storage (`chat_manual_defaults`, `chat_reason_shortcuts`), `skipLocal` cache prefixes, and `fuku/db/modtuning` repository are ready for Plan 27-03 (reason shortcuts expansion), Plan 27-04 (appeal button on ban messages & PM copies), and Plans 27-05/27-06 (private settings pages).

## Self-Check: PASSED

- Created files verified on disk: `migrations/20261122150000_add_manual_command_tuning.sql`, `fuku/db/models/modtuning.go`, `fuku/db/modtuning/repository.go`, `fuku/db/modtuning/testmain_test.go`, `fuku/db/modtuning/repository_test.go`, `fuku/db/backup/modtuning_backup_test.go`, `fuku/db/migrations/manual_command_tuning_postgres_test.go`, `fuku/modules/modtuning_settings.go`, `fuku/modules/modtuning_commands_test.go` — ALL FOUND.
- Task commits verified as ancestors of HEAD: `537258e4`, `80b0a8dc`, `cc3529c2` — ALL FOUND.
