---
phase: 33-smarter-static-reactions-highlights-archive-and-backup-parity
plan: 01
subsystem: database
tags: [postgres, gorm, redis, cache, migrations, reactions]

# Dependency graph
requires:
  - phase: 32-contextual-ai-emoji-reactions
    provides: "Latest baseline migration 20261128120000_add_chat_presence_ai_react_mode.sql and skipLocal chat_presence pattern"
provides:
  - "Idempotent SQL migration 20261129120000_add_static_reaction_options.sql adding word_boundary, match_caption, chance_pct, and cooldown_secs with inline CHECK constraints"
  - "Extended models.Reactions with WordBoundary, MatchCaption, ChancePct, and CooldownSecs (zero boolean default: tags, omitempty JSON tags)"
  - "xenobot/db/reactions Rule DTO, GetReactionRulesContext, SetReactionOptionsByID, widened ListReactionEntriesFresh, and dual-key cache invalidation"
  - "skipLocal registration for reaction_rules:<chat> in xenobot/db/cache/local.go"
affects: [33-03, 33-05, reactions-watcher, settings-reactions, backup]

# Actuals (#2632)
actuals:
  tokens: 7446
  tasks: 3
  commits: 5
plan_head_before: 391d71d4d3696a03a9bf28ae73913b2261ba8ee3
plan_head_after: d78394188e5eef86a74d0eaf74be59a263863135

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Dual-key reaction cache invalidation: reactions:<chat> (legacy map) and reaction_rules:<chat> (skipLocal typed []Rule) both dropped on every mutation"
    - "Rolling-deploy Rule normalization: normalizeRule maps ChancePct <= 0 or > 100 to 100 and clamps CooldownSecs outside 0..86400 to 0 after cache load"
    - "Zero boolean GORM default: tags on WordBoundary/MatchCaption so false writes explicitly"

key-files:
  created:
    - migrations/20261129120000_add_static_reaction_options.sql
    - xenobot/db/migrations/static_reaction_options_migration_test.go
  modified:
    - xenobot/db/models/reactions.go
    - xenobot/db/reactions/repository.go
    - xenobot/db/reactions/repository_test.go
    - xenobot/db/cache/local.go
    - xenobot/db/cache/local_test.go

key-decisions:
  - "Kept GetReactionsContext on reactions:<chat> unchanged and added distinct skipLocal key reaction_rules:<chat> for []Rule so rolling deploys never mix cached types"
  - "Added //go:build testtools build tag to xenobot/db/reactions/repository_test.go to use utilsCache.SetupTestMemoryMarshaler consistent with other db repository test suites"

patterns-established:
  - "Per-rule option updates strictly scoped by WHERE id = ? AND chat_id = ? with pre-DB bounds validation returning ErrInvalidOption"

requirements-completed: [COMM-01]

coverage:
  - id: D1
    description: "Idempotent append-only SQL migration adding word_boundary, match_caption, chance_pct, and cooldown_secs with inline CHECK constraints"
    requirement: "COMM-01"
    verification:
      - kind: unit
        ref: "xenobot/db/migrations/static_reaction_options_migration_test.go#TestStaticReactionOptionsMigrationStatements"
        status: pass
    human_judgment: false
  - id: D2
    description: "Extended models.Reactions and xenobot/db/reactions Rule cache, GetReactionRulesContext normalization, SetReactionOptionsByID, and skipLocal registration"
    requirement: "COMM-01"
    verification:
      - kind: unit
        ref: "xenobot/db/reactions/repository_test.go#TestReactionRuleOptionsAndCache"
        status: pass
      - kind: unit
        ref: "xenobot/db/cache/local_test.go#TestSkipLocalReactionRulesKey"
        status: pass
    human_judgment: false

# Metrics
duration: 3min
completed: 2026-11-27
status: complete
---

# Phase 33 Plan 01: Static Reaction Options Persistence Summary

**Append-only migration, `models.Reactions` option columns, typed `Rule` cache with `ChancePct == 0 -> 100` normalization, chat-scoped `SetReactionOptionsByID`, and `reaction_rules:` `skipLocal` registration for `COMM-01`**

## Performance

- **Duration:** 3 min
- **Started:** 2026-11-27T06:48:47Z
- **Completed:** 2026-11-27T06:51:36Z
- **Tasks:** 3
- **Files modified:** 7

## Accomplishments
- Added `migrations/20261129120000_add_static_reaction_options.sql` sorting strictly after `20261128120000_add_chat_presence_ai_react_mode.sql`, idempotently adding `word_boundary`, `match_caption`, `chance_pct` (`chk_reactions_chance_pct`), and `cooldown_secs` (`chk_reactions_cooldown_secs`).
- Extended `models.Reactions` with `WordBoundary`, `MatchCaption` (no GORM `default:` tag), `ChancePct` (`default:100`), and `CooldownSecs` (`default:0`) using `omitempty` JSON tags for legacy backup compatibility.
- Added `Rule`, `ValidChancePct`, `ValidCooldownSecs`, `ErrInvalidOption`, `GetReactionRulesContext` (with post-cache `ChancePct == 0 -> 100` normalization), `SetReactionOptionsByID` (bounds-checked and `id = ? AND chat_id = ?` scoped), widened `ListReactionEntriesFresh`, dual-key invalidation (`reactions:<chat>` + `reaction_rules:<chat>`), and `skipLocal` protection for `reaction_rules:`.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add append-only static reaction options migration, migration test, and GORM model fields** - `8baa638f` (feat)
2. **Task 2: Extend xenobot/db/reactions repository with Rule cache, options writer, and skipLocal registration** - `01dfae9f` (feat)
3. **Task 3: Add repository and skipLocal tests for static reaction options and legacy normalization** - `d7839418` (test)

_Note: `plan_head_before..plan_head_after` counts 5 commits on `main` because Wave 1 peer `33-02` committed twice (`78b9a15d`, `18b8d6e2`) concurrently on disjoint files._

## Files Created/Modified
- `migrations/20261129120000_add_static_reaction_options.sql` - Idempotent DDL adding static reaction option columns and inline CHECK constraints.
- `xenobot/db/migrations/static_reaction_options_migration_test.go` - Statement-shape test and opt-in PostgreSQL default/CHECK verification test.
- `xenobot/db/models/reactions.go` - Extended `Reactions` GORM model with four new columns and `omitempty` JSON tags.
- `xenobot/db/reactions/repository.go` - Added `Rule`, bounds validators, `GetReactionRulesContext`, `SetReactionOptionsByID`, widened `ListReactionEntriesFresh`, and dual-key cache invalidation.
- `xenobot/db/reactions/repository_test.go` - Added `TestReactionRuleOptionsAndCache` covering defaults, option updates, boolean resets, cross-chat IDOR refusal, bounds validation, cache eviction, and legacy `ChancePct == 0 -> 100` normalization.
- `xenobot/db/cache/local.go` - Registered `reactionRulesPrefix` (`reaction_rules:`) in `skipLocal`.
- `xenobot/db/cache/local_test.go` - Added `TestSkipLocalReactionRulesKey`.

## Decisions Made
- Added `//go:build testtools` to `xenobot/db/reactions/repository_test.go` so `TestReactionRuleOptionsAndCache` can use `utilsCache.SetupTestMemoryMarshaler(t)` matching all other `xenobot/db/*` repository test suites.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added `//go:build testtools` build tag to `xenobot/db/reactions/repository_test.go`**
- **Found during:** Task 3 (Adding `TestReactionRuleOptionsAndCache`)
- **Issue:** `utilsCache.SetupTestMemoryMarshaler` is guarded by `//go:build testtools`, while `xenobot/db/reactions/repository_test.go` previously lacked the build tag.
- **Fix:** Added `//go:build testtools` to the top of `xenobot/db/reactions/repository_test.go` consistent with `xenobot/db/presence/repository_test.go` and other db repositories.
- **Files modified:** `xenobot/db/reactions/repository_test.go`
- **Verification:** `go test -tags testtools -race -count=1 ./xenobot/db/migrations ./xenobot/db/reactions ./xenobot/db/cache` and `golangci-lint` passed with 0 issues.
- **Committed in:** `d7839418` (Task 3 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Necessary for compilation with `SetupTestMemoryMarshaler` under `-tags testtools`; no scope creep.

## Verification
- `go test -tags testtools -race -count=1 ./xenobot/db/reactions ./xenobot/db/cache ./xenobot/db/migrations -run 'TestReactions|TestReaction|TestSkipLocalReactionRulesKey|TestStaticReactionOptionsMigration'` — PASS
- `go test -tags testtools -race -count=1 ./xenobot/db/migrations ./xenobot/db/reactions ./xenobot/db/cache` — PASS (`TestStaticReactionOptionsMigrationDefaults` skipped when `XENOBOT_TEST_DATABASE` is unset, expected opt-in PG test)
- `CGO_ENABLED=0 go build ./xenobot/db/...` — PASS
- `GOTOOLCHAIN=go1.26.0 golangci-lint run ./xenobot/db/models ./xenobot/db/reactions ./xenobot/db/cache ./xenobot/db/migrations` — PASS (0 issues)

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `Rule`, `GetReactionRulesContext`, `SetReactionOptionsByID`, `ValidChancePct`, `ValidCooldownSecs`, and `reaction_rules:` `skipLocal` are ready for Wave 2 (`33-03`: runtime static reaction matching & settings UI, and `33-05`: backup/restore parity).

## Self-Check: PASSED

- All 7 created/modified files verified on disk.
- All 3 task commits (`8baa638f`, `01dfae9f`, `d7839418`) verified as ancestors of `HEAD`.

---
*Phase: 33-smarter-static-reactions-highlights-archive-and-backup-parity*
*Completed: 2026-11-27*
