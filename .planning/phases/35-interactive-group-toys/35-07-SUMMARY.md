---
phase: 35-interactive-group-toys
plan: 07
subsystem: docs
tags: [docs, makefile, agents, lint, race-tests, toys]

# Dependency graph
requires:
  - phase: 35-interactive-group-toys
    provides: "chat_presence toys migration & repo (35-01), leaf chatprompt/toys.go (35-02), 8-locale keys & guards (35-03), Settings pc rf/in & toy Redis keys (35-04), /referendum & tie-breaker (35-05), /inspect & bot_react (35-06)"
provides:
  - "Makefile test-postgres-integrity filter updated with TestChatPresenceToysMigrationDefaults"
  - "Generated docs/src/content/docs/commands/chat/index.md synced with /referendum and /inspect"
  - "AGENTS.md updated with Phase 35 operational, shutdown, handler, Redis, Settings, and Backup invariants"
  - "Full Phase 35 build, translation, docs, lint, shard-check, and -race verification gate passed with copy_settings.go untouched"
affects: [v1.3-milestone-completion, phase-35-verification]

# Actuals (#2632)
actuals:
  tokens: 4457
  tasks: 3
  commits: 2
  plan_head_before: 390c92954c546d81eb48485ca353ecfd34c3e499
  plan_head_after: 153bd7c96d1d59ff3cd9c02a5dcb7e0de5dfd885

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Zero new shutdown drains, zero new message handler groups, and copy_settings.go untouched across Phase 35"

key-files:
  created: []
  modified:
    - Makefile
    - docs/src/content/docs/commands/chat/index.md
    - AGENTS.md

key-decisions:
  - "None - plan executed exactly as written"

patterns-established:
  - "Phase 35 CI integrity, generated docs, lint, shard-check, and -race verification gates all pass cleanly"

requirements-completed: [TOYS-01, TOYS-02, TOYS-03]

coverage:
  - id: D1
    description: "Makefile test-postgres-integrity includes TestChatPresenceToysMigrationDefaults and generated docs sync cleanly with make check-docs"
    requirement: TOYS-01
    verification:
      - kind: integration
        ref: "grep -q TestChatPresenceToysMigrationDefaults Makefile && make check-docs"
        status: pass
    human_judgment: false
  - id: D2
    description: "AGENTS.md documents all Phase 35 invariants (StopChat toy lane, group-0 NewPoll, chatprompt/toys.go, Redis poll/tie keys, Settings pc 6 rows, BackupModulePresence parity, copy_settings.go untouched)"
    requirement: TOYS-02
    verification:
      - kind: unit
        ref: "grep -q 'Phase 35' AGENTS.md && grep -q ReferendumMaxTokens AGENTS.md"
        status: pass
    human_judgment: false
  - id: D3
    description: "Full Phase 35 build, 8-locale translation check, docs drift check, golangci-lint v2, test-shard-check, and -race tests across db, utils, i18n, and modules pass with copy_settings.go untouched"
    requirement: TOYS-03
    verification:
      - kind: integration
        ref: "CGO_ENABLED=0 go build ./... && make check-translations && make check-docs && GOTOOLCHAIN=go1.26.0 make lint && make test-shard-check && go test -tags testtools -race -count=1 ./xenobot/db/migrations ./xenobot/db/presence ./xenobot/db/backup ./xenobot/utils/chatprompt ./xenobot/i18n && go test -tags testtools -race -count=1 ./xenobot/modules -run 'TestReferendum|TestPollTieBreaker|TestShutdownEndsChatToys|TestInspect|TestShutdownEndsInspect|TestPresence|TestSettingsPresence|TestPhase33Codes|TestSettingsMain|TestBackup|TestSettingsCopy'"
        status: pass
    human_judgment: false

# Metrics
duration: 3 min
completed: 2026-11-27T13:20:48Z
status: complete
---

# Phase 35 Plan 07: Docs, AGENTS.md & Final Verification Summary

**Synced generated command docs with `/referendum` and `/inspect`, added `TestChatPresenceToysMigrationDefaults` to `Makefile`, documented all Phase 35 invariants in `AGENTS.md`, and passed the full build, translation, docs, lint, shard-check, and `-race` test gate with `copy_settings.go` untouched.**

## Performance

- **Duration:** 3 min
- **Started:** 2026-11-27T13:17:57Z
- **Completed:** 2026-11-27T13:20:48Z
- **Tasks:** 3
- **Files modified:** 3

## Accomplishments
- Updated `Makefile` (`test-postgres-integrity`) to include `TestChatPresenceToysMigrationDefaults`, ran `make generate-docs` to sync `/referendum` and `/inspect` into `docs/src/content/docs/commands/chat/index.md`, and verified `make check-docs` reports zero drift.
- Updated `AGENTS.md` across Startup/shutdown, Handlers, Chat help, and Presence sections documenting `migrations/20261201120000_add_presence_toys.sql`, `chatprompt/toys.go` (`ReferendumMaxTokens = 768`, `InspectMaxTokens = 512`, `TieBreakerMaxTokens = 512`), `chatToyQueue` + group-`0` `handlers.NewPoll(poll.All, chatModule.handlePollUpdate)` under `StopChat` (keeping `70 s` worst-case shutdown and `stop_grace_period: 75s` unchanged), `"toy:b"` budgets, `xenobot:presence:poll:<poll_id>` & `xenobot:presence:poll:tie:<poll_id>`, `/inspect` vision + `bot_react` `"inspect"` / `👎` retraction, Settings `pc` 6 rows / `pr` 8 rows / 31 pages / 17 Main rows, `BackupModulePresence` parity, and `copy_settings.go` untouched.
- Verified `git diff -- xenobot/modules/copy_settings.go` is empty (last touched in `85d697cd`) and passed every Phase 35 verification gate:
  - `CGO_ENABLED=0 go build ./...` (exit 0)
  - `make check-translations` (1071 keys across all 8 locales, 0 missing)
  - `make check-docs` (zero drift across 168 commands / 35 modules)
  - `GOTOOLCHAIN=go1.26.0 make lint` (0 issues)
  - `make test-shard-check` (exit 0)
  - `go test -tags testtools -race -count=1 ./xenobot/db/migrations ./xenobot/db/presence ./xenobot/db/backup ./xenobot/utils/chatprompt ./xenobot/i18n` (`ok`: 1.562s, 1.375s, 1.640s, 1.231s, 43.257s)
  - `go test -tags testtools -race -count=1 ./xenobot/modules -run 'TestReferendum|TestPollTieBreaker|TestShutdownEndsChatToys|TestInspect|TestShutdownEndsInspect|TestPresence|TestSettingsPresence|TestPhase33Codes|TestSettingsMain|TestBackup|TestSettingsCopy'` (`ok`: 4.641s)

## Task Commits

Each task was committed atomically:

1. **Task 1: Update Makefile test-postgres-integrity and sync generated docs** - `cc0f016a` (docs)
2. **Task 2: Update AGENTS.md with Phase 35 architectural and operational invariants** - `153bd7c9` (docs)
3. **Task 3: Run full Phase 35 build, translation, docs, lint, shard-check, and -race verification gate** - verification-only gate (no files modified)

## Files Created/Modified
- `Makefile` - Added `TestChatPresenceToysMigrationDefaults` to `test-postgres-integrity`.
- `docs/src/content/docs/commands/chat/index.md` - Synced generated documentation for `/referendum` and `/inspect`.
- `AGENTS.md` - Documented Phase 35 invariants across Startup/shutdown, Handlers, Chat, and Presence sections.

## Decisions Made
None - plan executed exactly as written.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Phase 35 (Interactive Group Toys) is complete (`7/7` plans complete; `TOYS-01`, `TOYS-02`, `TOYS-03` implemented and verified).
- Ready for Phase 35 verification (`/gsd-verify-work 35`) and v1.3 milestone completion.

## Self-Check: PASSED
- Verified `Makefile`, `docs/src/content/docs/commands/chat/index.md`, `AGENTS.md`, and `.planning/phases/35-interactive-group-toys/35-07-SUMMARY.md` exist on disk.
- Verified task commits `cc0f016a` and `153bd7c9` are ancestors of `HEAD` (`commits: 2`).
- Verified `copy_settings.go` remained untouched and all build, translation, docs, lint, shard-check, and `-race` test gates passed cleanly.

---
*Phase: 35-interactive-group-toys*
*Completed: 2026-11-27*
