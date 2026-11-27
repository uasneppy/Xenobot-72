---
phase: 32-contextual-ai-emoji-reactions
plan: 01
subsystem: database
tags: [postgres, gorm, migration, msgpack, cache, presence, ai-reactions]
status: complete

requires:
  - phase: 31-presence-foundation-settings-page-and-zero-ai-social-reactions
    provides: chat_presence table, models.ChatPresence, skipLocal chat_presence:<chat> cache key, xenobot/db/presence repository
provides:
  - Append-only idempotent SQL migration adding chat_presence.ai_react_mode with inline chk_chat_presence_ai_react_mode CHECK constraint
  - models.ChatPresence.AIReactMode GORM column with zero default tags on booleans
  - xenobot/db/presence AIReactModeOff/Low/Medium constants, ValidAIReactMode, SetAIReactMode column-targeted upsert, and rolling-deploy cache normalization
affects:
  - 32-03 (runtime contextual AI emoji reaction watcher reading presence.Settings.AIReactMode)
  - 32-04 (Settings page pr AI Reactions segmented selector writing via presence.SetAIReactMode)

tech-stack:
  added: []
  patterns:
    - Inline named CHECK constraint on ALTER TABLE ... ADD COLUMN IF NOT EXISTS for idempotent PostgreSQL re-runs
    - Rolling-deploy msgpack schema evolution normalizing missing/empty AIReactMode ("") to AIReactModeOff ("off") on read in Get() and GetFresh()
    - Column-targeted OnConflict upsert with immediate skipLocal cache.DeleteCache invalidation

key-files:
  created:
    - migrations/20261128120000_add_chat_presence_ai_react_mode.sql
    - xenobot/db/migrations/chat_presence_ai_react_migration_test.go
  modified:
    - xenobot/db/models/chat_presence.go
    - xenobot/db/presence/repository.go
    - xenobot/db/presence/repository_test.go

key-decisions:
  - "Kept ai_react_mode strictly on chat_presence (isolated from Owner-only chat_conversations) with inline chk_chat_presence_ai_react_mode CHECK constraint and zero GORM default: tags on boolean columns."
  - "Normalized empty/unknown cached AIReactMode values to AIReactModeOff in both Get() and GetFresh() so Phase 31 rolling-deploy msgpack entries without tag \"ar\" never expose an invalid empty mode."

patterns-established:
  - "Rolling-deploy presence cache normalization: normalizeAIReactMode applied immediately after cache.GetFromCacheOrLoad in presence.Get and on DB scan in presence.GetFresh."

requirements-completed:
  - PRES-04
  - REACT-01

duration: 4 min
completed: 2026-11-27

commits: 3
plan_head_before: b4380a0b8af9747d5324526635a643f60f1bc17a
plan_head_after: edc7eba42feb7220cbe91da91c0149afcf04661a

actuals:
  tokens: 5017
  tasks: 3
  commits: 3
---

# Phase 32 Plan 01: AI React Mode Persistence & Rolling-Deploy Cache Safety Summary

**Append-only `chat_presence.ai_react_mode` SQL migration with inline CHECK constraint, GORM model column, column-targeted `SetAIReactMode` upsert, and Phase 31 rolling-deploy msgpack normalization in `xenobot/db/presence`**

## Performance

- **Duration:** 4 min (236 s)
- **Started:** 2026-11-26T23:40:50Z
- **Completed:** 2026-11-26T23:44:46Z
- **Tasks:** 3
- **Files modified:** 5 (2 created, 3 modified)

## Accomplishments

- Added append-only idempotent SQL migration `migrations/20261128120000_add_chat_presence_ai_react_mode.sql` sorting strictly after `20261127120000_add_chat_presence.sql`, adding `ai_react_mode VARCHAR(16) NOT NULL DEFAULT 'off'` with inline `CONSTRAINT chk_chat_presence_ai_react_mode CHECK (ai_react_mode IN ('off', 'low', 'medium'))`.
- Added migration verification tests (`TestChatPresenceAIReactMigrationStatements` and opt-in PostgreSQL `TestChatPresenceAIReactMigrationDefaults`) and extended `models.ChatPresence` with `AIReactMode` while keeping zero `default:` tags on boolean fields.
- Extended `xenobot/db/presence` with `AIReactModeOff`, `AIReactModeLow`, `AIReactModeMedium`, `ValidAIReactMode`, `normalizeAIReactMode`, `Settings.AIReactMode` (`msgpack:"ar"`), `Defaults()` returning `"off"`, rolling-deploy normalization in `Get()` and `GetFresh()`, and column-targeted `SetAIReactMode` with immediate `skipLocal` cache invalidation.
- Expanded `xenobot/db/presence/repository_test.go` to verify invalid mode rejection before DB writes, column-targeted upserts preserving existing columns, cache invalidation, rolling-deploy normalization of legacy Phase 31 msgpack payloads (missing `"ar"` tag) and empty cached strings, and `chk_chat_presence_ai_react_mode` enforcement.

## Task Commits

Each task was committed atomically:

1. **Task 1 (tracer): Add append-only ai_react_mode SQL migration, migration test, and GORM model column** - `d68fc10d` (feat)
2. **Task 2: Extend xenobot/db/presence repository with AIReactMode CRUD and rolling-deploy normalization** - `654e2715` (feat)
3. **Task 3: Add repository tests for AIReactMode validation, rolling-deploy normalization, cache invalidation, and DB CHECK** - `edc7eba4` (test)

## Files Created/Modified

- `migrations/20261128120000_add_chat_presence_ai_react_mode.sql` - Idempotent `ALTER TABLE chat_presence ADD COLUMN IF NOT EXISTS ai_react_mode` with inline `chk_chat_presence_ai_react_mode` CHECK constraint.
- `xenobot/db/migrations/chat_presence_ai_react_migration_test.go` - Statement shape validation and opt-in PostgreSQL default/CHECK test.
- `xenobot/db/models/chat_presence.go` - Added `AIReactMode` field and updated struct doc comment.
- `xenobot/db/presence/repository.go` - Exported `AIReactModeOff/Low/Medium`, `ValidAIReactMode`, `SetAIReactMode`, added `msgpack:"ar"` field and rolling-deploy normalization in `Get()`/`GetFresh()`.
- `xenobot/db/presence/repository_test.go` - Updated struct assertions and added comprehensive `ai_react_mode` validation, upsert, rolling-deploy cache normalization, and CHECK constraint subtests.

## Decisions Made

- Kept `ai_react_mode` strictly inside `chat_presence` (separate from Owner-only `chat_conversations`) to uphold `PRES-04` and keep backup boundaries untouched.
- Applied `normalizeAIReactMode` on both cached (`Get`) and uncached (`GetFresh`) read paths so any rolling-deploy Phase 31 cache payload missing msgpack tag `"ar"` safely reads as `"off"`.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## Verification

- `go test -tags testtools -race -count=1 ./xenobot/db/migrations ./xenobot/db/presence` — PASS (`TestChatPresenceSettings` 0.01s, `TestChatPresenceAIReactMigrationStatements` 0.00s, `TestChatPresenceMigrationStatements` 0.01s).
- `CGO_ENABLED=0 go build ./xenobot/db/...` and `CGO_ENABLED=0 go build ./...` — PASS.
- `GOTOOLCHAIN=go1.26.0 golangci-lint run --build-tags=testtools ./xenobot/db/migrations/... ./xenobot/db/models/... ./xenobot/db/presence/...` — 0 issues.
- `go test -tags testtools -race -count=1 ./xenobot/modules -run 'TestPresence|TestSettingsPresence'` — PASS (3.065s; zero regressions to Phase 31 tests).

## Next Phase Readiness

- Ready for Wave 2 plans (`32-03` runtime contextual AI emoji reactions and `32-04` Settings page `pr` AI Reactions controls) to consume `presence.AIReactModeOff/Low/Medium`, `presence.ValidAIReactMode`, `presence.Settings.AIReactMode`, and `presence.SetAIReactMode`.
- No stubs, no new external dependencies, no new shutdown drains, and no threat flags introduced.

## Self-Check: PASSED

