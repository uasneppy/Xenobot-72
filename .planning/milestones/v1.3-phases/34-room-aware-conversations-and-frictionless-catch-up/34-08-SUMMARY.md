---
phase: 34-room-aware-conversations-and-frictionless-catch-up
plan: 08
subsystem: docs-and-verification
tags: [makefile, generated-docs, agents-md, lint, race-tests, verification]

# Dependency graph
requires:
  - phase: 34-room-aware-conversations-and-frictionless-catch-up
    provides: "Ambient context & vocative addressing (34-05), reply-free /unroll* & silent ✍ PM brief (34-06), and Room & Catch-Up Settings UI (34-07)"
provides:
  - "Makefile test-postgres-integrity registered with TestChatPresenceRoomMigrationDefaults"
  - "Synced docs/src/content/docs/commands/{chat,presence}/index.md with zero make check-docs drift"
  - "Updated AGENTS.md documenting all Phase 34 invariants (StopChat 70 s unchanged, chatLogEntry.At, Ambient Room Context, Vocative Addressing, reply-free /unroll*, silent ✍ PM brief, Settings pc, BackupModulePresence parity, copy_settings.go untouched)"
  - "Full Phase 34 build, translation, docs, lint, shard-check, and -race test verification across CHAT-01..05"
affects:
  - 35-interactive-group-toys

# Actuals (#2632)
actuals:
  tokens: 7455
  tasks: 3
  commits: 2
  plan_head_before: 1977317fca95e527f32976089370be8bcd1e4f56
  plan_head_after: f3d35ab6be6eee9055fb1f4d6b54936a3796812b

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Generated command docs stay byte-synced via make generate-docs + make check-docs"
    - "PostgreSQL migration integrity regex in Makefile tracks every new phase migration test"

key-files:
  created: []
  modified:
    - Makefile
    - AGENTS.md
    - docs/src/content/docs/commands/chat/index.md
    - docs/src/content/docs/commands/presence/index.md

key-decisions:
  - "Documented xenobot:presence:pmbrief:cd:{<chat>}:<user> (matching presencePMBriefCooldownKey in presence_reactions.go) and exact 70 s StopChat reuse in AGENTS.md"

patterns-established:
  - "Protected xenobot/modules/copy_settings.go verified completely untouched across Phase 34 (last commit 85d697cd)"

requirements-completed:
  - CHAT-01
  - CHAT-02
  - CHAT-03
  - CHAT-04
  - CHAT-05

coverage:
  - id: D1
    description: "Makefile test-postgres-integrity includes TestChatPresenceRoomMigrationDefaults and make generate-docs / make check-docs pass with zero drift"
    requirement: CHAT-01
    verification:
      - kind: other
        ref: "make check-docs"
        status: pass
      - kind: integration
        ref: "go test -tags testtools -race -count=1 ./xenobot/db/migrations ./xenobot/db/presence ./xenobot/db/backup"
        status: pass
    human_judgment: false
  - id: D2
    description: "AGENTS.md documents all Phase 34 invariants (StopChat 70 s unchanged, chatLogEntry.At + thread-isolated readers, Ambient Room Context, Vocative Addressing, reply-free /unroll*, silent ✍ PM brief, Settings pc, BackupModulePresence parity, copy_settings.go untouched)"
    requirement: CHAT-02
    verification:
      - kind: other
        ref: "grep -q 'Phase 34' AGENTS.md && grep -q 'ParseVocativePrefix' AGENTS.md"
        status: pass
    human_judgment: false
  - id: D3
    description: "Full Phase 34 build, translations, docs, golangci-lint (Go 1.26), shard-check, copy_settings.go immutability, and targeted -race test suites pass across db, utils, i18n, and modules"
    requirement: CHAT-05
    verification:
      - kind: other
        ref: "CGO_ENABLED=0 go build ./... && make check-translations && make check-docs && GOTOOLCHAIN=go1.26.0 make lint && make test-shard-check"
        status: pass
      - kind: unit
        ref: "go test -tags testtools -race -count=1 ./xenobot/db/migrations ./xenobot/db/presence ./xenobot/db/backup ./xenobot/utils/chatprompt ./xenobot/utils/extraction ./xenobot/i18n"
        status: pass
      - kind: unit
        ref: "go test -tags testtools -race -count=1 ./xenobot/modules -run 'TestChatLog|TestChatAmbientContext|TestChatVocativeAddressing|TestChatWatcher|TestChatReact|TestChatBrief|TestChatJudge|TestPresence|TestShutdownEndsPresenceReaction|TestShutdownEndsChat|TestShutdownEndsChatBriefs|TestSettingsPresence|TestPhase33Codes|TestSettingsMain|TestBackup|TestSettingsCopy'"
        status: pass
    human_judgment: false

# Metrics
duration: 4 min
completed: 2026-11-27
status: complete
---

# Phase 34 Plan 08: Docs Sync, Makefile Integrity, AGENTS.md & Verification Gate Summary

**Registered `TestChatPresenceRoomMigrationDefaults` in `Makefile`, synced generated docs (`chat` and `presence`), documented all Phase 34 invariants in `AGENTS.md`, confirmed `copy_settings.go` untouched, and passed the full build, translation, docs, Go 1.26 lint, shard-check, and `-race` test gate (`CHAT-01..05`).**

## Performance

- **Duration:** 4 min
- **Started:** 2026-11-27T07:45:07Z
- **Completed:** 2026-11-27T07:48:53Z
- **Tasks:** 3
- **Files modified:** 4

## Accomplishments
- Added `TestChatPresenceRoomMigrationDefaults` to `Makefile`'s `test-postgres-integrity` `-run` regex and ran `make generate-docs`, updating `docs/src/content/docs/commands/chat/index.md` and `docs/src/content/docs/commands/presence/index.md` with zero `make check-docs` drift.
- Updated `AGENTS.md` across Startup/shutdown, Handlers, Chat/Briefs, Presence, and Redis sections to accurately capture Phase 34 (`StopChat` 70 s worst case unchanged, `chatLogEntry.At` + thread-isolated readers, `CHAT-02` double-gated ambient context + dedup, `CHAT-03` pure Unicode `ParseVocativePrefix` + group 13 mutual exclusion, `CHAT-04` reply-free `/unrollshort`/`/unrolllong`, `CHAT-05` silent `✍` PM brief, Settings sub-page `pc` on 8-row `pr`, `BackupModulePresence` parity, and `copy_settings.go` untouched).
- Verified `xenobot/modules/copy_settings.go` has zero Phase-34 diffs (last modified in `85d697cd`) and passed `CGO_ENABLED=0 go build ./...`, `make check-translations` (1070 keys across 8 locales), `make check-docs`, `GOTOOLCHAIN=go1.26.0 make lint` (0 issues), `make test-shard-check`, and `-race` tests across `./xenobot/db/migrations`, `./xenobot/db/presence`, `./xenobot/db/backup`, `./xenobot/utils/chatprompt`, `./xenobot/utils/extraction`, `./xenobot/i18n`, and targeted `./xenobot/modules` suites.

## Task Commits

Each task was committed atomically (`Task 3` was verification-only with no workspace edits after `Task 2`):

1. **Task 1: Update Makefile test-postgres-integrity and sync generated docs** - `2374b6fc` (docs)
2. **Task 2: Update AGENTS.md with Phase 34 architectural and operational invariants** - `f3d35ab6` (docs)
3. **Task 3: Run full Phase 34 build, translation, docs, lint, shard-check, and -race verification gate** - verification-only (no additional files modified)

## Files Created/Modified
- `Makefile` - Added `TestChatPresenceRoomMigrationDefaults` to `test-postgres-integrity`.
- `docs/src/content/docs/commands/chat/index.md` - Regenerated Chat command documentation with reply-free `/unrollshort` & `/unrolllong` syntax.
- `docs/src/content/docs/commands/presence/index.md` - Regenerated Presence documentation.
- `AGENTS.md` - Documented all Phase 34 architectural, shutdown, handler, Redis, Settings, and backup invariants.

## Decisions Made
None - plan executed as specified (noting exact Redis key spelling `xenobot:presence:pmbrief:cd:{<chat>}:<user>` matching `presencePMBriefCooldownKey` in `presence_reactions.go`).

## Deviations from Plan
None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 34 (`CHAT-01..05`) implementation, generated documentation, `AGENTS.md`, and automated verification gates are complete and ready for phase verification (`/gsd-verify-work 34`) and Phase 35 (`TOYS-01..03`).

---
*Phase: 34-room-aware-conversations-and-frictionless-catch-up*
*Completed: 2026-11-27*
