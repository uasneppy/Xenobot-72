---
phase: 31-presence-foundation-settings-page-and-zero-ai-social-reactions
plan: 01
subsystem: database
tags: [config, allowed-updates, postgres, gorm, sqlite, redis, skip-local, presence]

# Dependency graph
requires:
  - phase: 30-backup-page-and-custom-messages-in-backups
    provides: Baseline schema ending at 20261126120000_allow_featherless_chat_provider.sql and skipLocal cache pattern
provides:
  - Telegram "message_reaction" subscription in config.AppConfig.AllowedUpdates (excluding "message_reaction_count")
  - Append-only SQL migration 20261127120000_add_chat_presence.sql with inline CHECK (2..20) and SQL defaults
  - models.ChatPresence GORM model with zero default: tags on booleans and SQLite AutoMigrate registration
  - skipLocal bypass for chat_presence:<chat> keys
  - xenobot/db/presence repository with column-targeted upserts, bounds validation, and immediate cache invalidation
affects:
  - 31-02-PLAN.md
  - 31-03-PLAN.md
  - 31-04-PLAN.md
  - 31-05-PLAN.md
  - phase-32-contextual-ai-emoji-reactions

actuals:
  tokens: 6588
  tasks: 3
  commits: 4
  plan_head_before: 89433a0eb9fadf0d817101149f183b5bfaf3b905
  plan_head_after: e0422bd1ea22aa40a378de0e212c4cad94be11b7

tech-stack:
  added: []
  patterns:
    - "Dedicated admin-configurable chat_presence table isolated from Owner-only chat_conversations"
    - "Omit GORM default: tags on boolean model fields while keeping SQL DEFAULT TRUE/FALSE in migration DDL"
    - "Column-targeted clause.OnConflict upserts populated via defaultRow() followed by immediate cache.DeleteCache on skipLocal keys"

key-files:
  created:
    - migrations/20261127120000_add_chat_presence.sql
    - xenobot/db/migrations/chat_presence_migration_test.go
    - xenobot/db/models/chat_presence.go
    - xenobot/db/presence/repository.go
    - xenobot/db/presence/testmain_test.go
    - xenobot/db/presence/repository_test.go
  modified:
    - xenobot/config/config.go
    - xenobot/config/config_pure_test.go
    - xenobot/db/cache/local.go
    - xenobot/db/cache/local_test.go
    - xenobot/modules/test_harness_test.go

key-decisions:
  - "Subscribing \"message_reaction\" (and excluding \"message_reaction_count\") in config.LoadConfig covers both webhook and polling paths with zero extra plumbing"
  - "chat_presence booleans carry no GORM default: tags so Go false writes (especially RetractEnabled=false on new and existing rows) persist explicitly"
  - "Activated utilsCache.SetupTestMemoryMarshaler(t) in xenobot/db/presence cache invalidation test to verify stale cached Get entries are evicted on every Set* write"

patterns-established:
  - "chat_presence:<chat> skipLocal prefix with trailing colon prevents sibling prefix collisions"
  - "Dual-layer threshold validation: Go ValidConsensusThreshold (2..20) returning ErrInvalidValue before DB writes plus inline SQL/GORM CHECK chk_chat_presence_consensus_threshold"

requirements-completed: [PRES-01, PRES-03]

coverage:
  - id: D1
    description: "config.AppConfig.AllowedUpdates subscribes to \"message_reaction\" and excludes \"message_reaction_count\""
    requirement: PRES-01
    verification:
      - kind: unit
        ref: "xenobot/config/config_pure_test.go#TestLoadConfig"
        status: pass
    human_judgment: false
  - id: D2
    description: "skipLocal bypasses chat_presence:<chat> without bypassing sibling prefixes"
    requirement: PRES-03
    verification:
      - kind: unit
        ref: "xenobot/db/cache/local_test.go#TestSkipLocalChatPresenceKey"
        status: pass
    human_judgment: false
  - id: D3
    description: "Append-only SQL migration 20261127120000_add_chat_presence.sql and models.ChatPresence define chat_presence with inline CHECK (2..20) and no GORM default: tags on booleans"
    requirement: PRES-03
    verification:
      - kind: unit
        ref: "xenobot/db/migrations/chat_presence_migration_test.go#TestChatPresenceMigrationStatements"
        status: pass
      - kind: other
        ref: "CGO_ENABLED=0 go build ./..."
        status: pass
    human_judgment: false
  - id: D4
    description: "xenobot/db/presence repository implements defaults-without-insert reads, column-targeted upserts preserving untouched columns and Go false values, 2..20 validation, immediate cache invalidation, and SQLite harness parity"
    requirement: PRES-03
    verification:
      - kind: integration
        ref: "xenobot/db/presence/repository_test.go#TestChatPresenceSettings"
        status: pass
    human_judgment: false

duration: 4 min
completed: 2026-11-26
status: complete
---

# Phase 31 Plan 01: Presence Foundation Config and Persistence Summary

**Subscribed Telegram `message_reaction` updates, added the append-only `chat_presence` PostgreSQL schema and GORM model, bypassed process-local caching for `chat_presence:<chat>`, and delivered the column-targeted `xenobot/db/presence` repository with SQLite parity.**

## Performance

- **Duration:** 4 min
- **Started:** 2026-11-26T21:43:39Z
- **Completed:** 2026-11-26T21:48:06Z
- **Tasks:** 3
- **Files modified:** 11 (6 created, 5 modified)

## Accomplishments
- Added `"message_reaction"` (excluding `"message_reaction_count"`) to `config.AppConfig.AllowedUpdates` so both webhook and polling deployments receive per-user reaction updates (`PRES-01`).
- Registered `chat_presence:<chat>` in `xenobot/db/cache/local.go` `skipLocal()` with trailing-colon boundary protection so Interactive Presence setting changes propagate across replicas on the next event (`PRES-03`).
- Created append-only migration `migrations/20261127120000_add_chat_presence.sql` (strictly after `20261126120000_allow_featherless_chat_provider.sql`), its statement/PostgreSQL defaults test suite, and `models.ChatPresence` with inline `CHECK (consensus_threshold BETWEEN 2 AND 20)` and zero GORM `default:` tags on booleans.
- Implemented `xenobot/db/presence` (`Defaults`, `ValidConsensusThreshold`, `CacheKey`, `Get`, `GetFresh`, `SetConsensusEnabled`, `SetConsensusThreshold`, `SetRetractEnabled`) via TDD (RED → GREEN) and registered `&models.ChatPresence{}` in `xenobot/modules/test_harness_test.go`.

## Task Commits

Each task was committed atomically:

1. **Task 1: Subscribe "message_reaction" in AllowedUpdates and register chat_presence in skipLocal (tracer)** - `e78196d1` (feat)
2. **Task 2: Add append-only chat_presence SQL migration, migration test, and GORM model** - `141d3529` (feat)
3. **Task 3: Implement xenobot/db/presence repository, tests, and SQLite harness registration (TDD)**
   - RED: `3c0f6b67` (test) — compiling stubs + failing assertions across all 4 subtests
   - GREEN: `e0422bd1` (feat) — full repository implementation, active memory cache marshaler check, and SQLite `AutoMigrate` registration

## Files Created/Modified
- `xenobot/config/config.go` - Appended `"message_reaction"` to `cfg.AllowedUpdates`.
- `xenobot/config/config_pure_test.go` - Asserted `"message_reaction"` present and `"message_reaction_count"` absent in `TestLoadConfig`.
- `xenobot/db/cache/local.go` - Added `chatPresencePrefix` to `skipLocal()`.
- `xenobot/db/cache/local_test.go` - Added `TestSkipLocalChatPresenceKey`.
- `migrations/20261127120000_add_chat_presence.sql` - Idempotent `CREATE TABLE IF NOT EXISTS chat_presence` with inline `chk_chat_presence_consensus_threshold`.
- `xenobot/db/migrations/chat_presence_migration_test.go` - Offline SQL statement/sort test and PostgreSQL defaults/constraint test.
- `xenobot/db/models/chat_presence.go` - `ChatPresence` GORM model with no boolean `default:` tags and matching SQLite `CHECK` tag.
- `xenobot/db/presence/repository.go` - Cached reads, fresh reads, threshold validation, column-targeted upserts, and immediate `cache.DeleteCache` invalidation.
- `xenobot/db/presence/testmain_test.go` - SQLite `testdb.Run` harness for `models.ChatPresence`.
- `xenobot/db/presence/repository_test.go` - Unit/integration tests for defaults, explicit Go `false`, column isolation, bounds rejection, cache eviction, and DB `CHECK` constraint.
- `xenobot/modules/test_harness_test.go` - Registered `&models.ChatPresence{}` in `AutoMigrate`.

## Decisions Made
- Followed plan specifications directly; activated `utilsCache.SetupTestMemoryMarshaler(t)` inside `TestChatPresenceSettings` cache invalidation subtest so `Get` genuinely serves from cache and proves `cache.DeleteCache` evicts stale entries on every `Set*` write.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Ready for `31-02-PLAN.md` (Presence safety gate & `AvailableReactions`-safe reaction API) and `31-04-PLAN.md` (Settings page `pr`), both of which depend on `xenobot/db/presence`, `models.ChatPresence`, and `"message_reaction"` in `AllowedUpdates`.
- Zero blockers.

---
*Phase: 31-presence-foundation-settings-page-and-zero-ai-social-reactions*
*Completed: 2026-11-26*

## Self-Check: PASSED

