---
phase: 28-advanced-thresholds
plan: 02
subsystem: database
tags: [postgres, gorm, redis, skipLocal, raid-detection, lua, thresholds, drift-guard]

requires:
  - phase: 28-advanced-thresholds
    provides: MSG-11 characterization golden suite TestLegacyPhase28DefaultThresholds and phase28InProcessMiniredis helper (28-01)
provides:
  - Idempotent chat_advanced_thresholds SQL migration with 12 inline named CHECK constraints and no FK on chats
  - ChatAdvancedThresholds GORM model and exported Min/Max/Default constants for all 12 Phase 28 thresholds
  - skipLocal-cached fuku/db/advthresholds repository (non-creating Get/GetFresh, column-targeted SetField, ResetField, ResetArea, ResetAll)
  - Go service/apply/lookup layer in fuku/modules/advthresholds.go with P28-01 default normalization and P28-04 fail-to-default reads
  - End-to-end raid detection wiring across join surge, flood, AI burst, derived TTLs/trim window, and all 5 raidRecentWindow sites
  - 12-constant drift guard (TestAdvancedThresholdDefaultsMatchConstants) and raid wiring/TTL/cache tests (TestAdvancedThresholdsRaidWiring)
affects: [28-03, 28-04, 28-05, 28-06]

tech-stack:
  added: []
  patterns:
    - "Zero-means-default column storage in chat_advanced_thresholds with Go default normalization (P28-01) and dual Go + SQL CHECK enforcement (T-28-02)"
    - "Cross-replica skipLocal caching (chat_advanced_thresholds:) with column-targeted upserts and post-commit DeleteCache (T-28-03)"
    - "Derived Redis TTL/trim invariants (trimWindow=max(joinWindow,recentWindow), joinedTTL=max(raidJoinedTTL,trimWindow+1h), floodTTL=max(raidFloodTTL,floodWindow*2), aiTTL=max(raidAITTL,aiWindow*2)) (T-28-04)"
    - "Fail-to-default bounded threshold lookup on transient DB/cache faults (P28-04, T-28-05)"

key-files:
  created:
    - migrations/20261123120000_add_advanced_thresholds.sql
    - fuku/db/models/advthresholds.go
    - fuku/db/advthresholds/repository.go
    - fuku/db/advthresholds/testmain_test.go
    - fuku/db/advthresholds/repository_test.go
    - fuku/db/backup/advthresholds_backup_test.go
    - fuku/db/migrations/advanced_thresholds_postgres_test.go
    - fuku/modules/advthresholds.go
    - fuku/modules/advthresholds_raid_test.go
  modified:
    - fuku/db/cache/local.go
    - fuku/db/cache/local_test.go
    - fuku/db/backup/testmain_test.go
    - fuku/modules/test_harness_test.go
    - fuku/modules/raid_counters.go
    - fuku/modules/raid_join.go
    - fuku/modules/raid_flood.go
    - fuku/modules/raid_ai.go
    - fuku/modules/raid_ai_count.go
    - Makefile

key-decisions:
  - "P28-01: Storing 0 in any chat_advanced_thresholds column means unset/default; applyAdvancedThreshold normalizes a submitted value equal to an item's compiled default to 0 so the item cleanly returns to Default state."
  - "P28-04: advThresholdsFor bounds lookups to 2 s, logs an IDs-only warning on read error, and falls back to zero Thresholds{} (compiled defaults) so transient DB/cache faults never disable raid detection."
  - "Derived Redis TTL/trim helpers preserve byte-identical defaults (25h joined, 2m flood, 5m AI) while preventing premature trimming/expiry whenever windows or recent-age hours are raised."

patterns-established:
  - "advThresholdsFor(ctx, chatID) + adv*() resolved helpers: shared hot-path lookup seam for Plans 28-03, 28-04, and 28-05."
  - "TestAdvancedThresholdDefaultsMatchConstants: compile-and-test drift guard pinning all 12 model/repository defaults against compiled module constants."

requirements-completed: [TUNE-07]

status: complete
plan_head_before: fae3df4fe6f9594e4424098632b55fc7279853d2
plan_head_after: f96d4b74633af10be59008232ac92cbc4555ecf8
commits: 3

actuals:
  tokens: 24023
  tasks: 3
  commits: 3

duration: 10min
completed: 2026-11-23
---

# Phase 28 Plan 02: Advanced Thresholds Foundation & Raid Tracer Slice Summary

**PostgreSQL `chat_advanced_thresholds` schema, `skipLocal`-cached repository, Go apply/lookup service with 12-constant drift guard, and end-to-end raid detection wiring across all raid windows, new-member share/floor, derived TTLs, and all 5 `raidRecentWindow` sites**

## Performance

- **Duration:** 10 min
- **Started:** 2026-11-23T08:51:24Z
- **Completed:** 2026-11-23T09:01:35Z
- **Tasks:** 3
- **Files modified:** 19 (9 created, 10 modified)

## Accomplishments

- Added idempotent migration `migrations/20261123120000_add_advanced_thresholds.sql` creating `chat_advanced_thresholds` (`chat_id` PK, no FK on `chats`, all 12 integer columns defaulting to `0`, `updated_by`, `updated_at`, and 12 inline named `chk_chat_adv_*` CHECK constraints) plus GORM model & exported `Min`/`Max`/`Default` constants in `fuku/db/models/advthresholds.go`.
- Built `fuku/db/advthresholds/repository.go` with `Area`/`Field` enums, `FieldSpec` registry (`SpecFor`, `FieldsInArea`, `ValidArea`, `AllAreas`, `AllSpecs`), `Thresholds` helpers, non-creating `Get`/`GetFresh`, Go-validated column-targeted `SetField`/`ResetField`/`ResetArea`, `ResetAll`, post-commit `cache.DeleteCache`, and `chatAdvancedThresholdsPrefix` registered in `fuku/db/cache/local.go` `skipLocal`.
- Built `fuku/modules/advthresholds.go` (`advThresholdsLookup`, bounded fail-to-default `advThresholdsFor`, all 12 resolved getters preserving package-var test seams, and `applyAdvancedThreshold` / `resetAdvancedThreshold` / `resetAdvancedArea` with default normalization and IDs-only logging).
- Wired all raid detection hot paths and all 5 `raidRecentWindow` sites (`raid_counters.go`, `raid_join.go`, `raid_flood.go`, `raid_ai.go`, `raid_ai_count.go`) with derived TTLs (`joinedTTL`, `floodTTL`, `aiTTL`) and `trimWindow = max(joinWindow, recentWindow)`, reading cached `advThresholdsFor` only after enabled/exemption guards so warm hot-path messages make 0 extra DB round trips.
- Added full test coverage: `TestSkipLocalChatAdvancedThresholdsKey`, `TestAdvancedThresholdsCRUD`, `TestAdvThresholdsExcludedFromBackup`, `TestAdvancedThresholdsMigrationStatements`, `TestAdvancedThresholdsMigrationDefaults` (wired into `Makefile` `test-postgres-integrity`), `TestAdvancedThresholdDefaultsMatchConstants` (12-constant drift guard), and `TestAdvancedThresholdsRaidWiring` (5/5 subtests passing under `-race` with zero localhost TCP dials), while keeping `TestLegacyPhase28DefaultThresholds` (MSG-11) untouched and green.

## Task Commits

1. **Task 1 (tracer): migration, model, skipLocal repository, apply/lookup service, and raid detection wiring across all 5 recent-window sites** - `29922fae` (`feat(28-02): add advanced thresholds schema, skipLocal repo, and raid wiring`)
2. **Task 2: repository CRUD, backup exclusion, and PostgreSQL migration integrity tests** - `5f58b224` (`test(28-02): add advthresholds CRUD, backup exclusion, and migration tests`)
3. **Task 3: drift guard and raid threshold runtime & TTL tests** - `f96d4b74` (`test(28-02): add 12-constant drift guard and raid threshold wiring tests`)

## Files Created/Modified

- `migrations/20261123120000_add_advanced_thresholds.sql` — Idempotent `chat_advanced_thresholds` table with 12 inline named CHECK constraints.
- `fuku/db/models/advthresholds.go` — `ChatAdvancedThresholds` model with SQLite-compatible CHECK tags and exported `Default`/`Min`/`Max` constants.
- `fuku/db/advthresholds/repository.go` — `Area`/`Field`/`FieldSpec`, `Thresholds`, `skipLocal`-cached `Get`/`GetFresh`, column-targeted `SetField`/`ResetField`/`ResetArea`, `ResetAll`.
- `fuku/db/advthresholds/testmain_test.go` — SQLite/Postgres test harness for `fuku/db/advthresholds`.
- `fuku/db/advthresholds/repository_test.go` — `TestAdvancedThresholdsCRUD` covering zero-row reads, all 12 min/max bounds, invalid rejections, column-targeted upserts, field/area/all resets, cache invalidation, and concurrent writes.
- `fuku/db/cache/local.go` & `fuku/db/cache/local_test.go` — `chatAdvancedThresholdsPrefix` in `skipLocal` and `TestSkipLocalChatAdvancedThresholdsKey`.
- `fuku/db/backup/testmain_test.go` & `fuku/db/backup/advthresholds_backup_test.go` — Model registration and `TestAdvThresholdsExcludedFromBackup`.
- `fuku/db/migrations/advanced_thresholds_postgres_test.go` — `TestAdvancedThresholdsMigrationStatements` and `TestAdvancedThresholdsMigrationDefaults`.
- `fuku/modules/test_harness_test.go` — AutoMigrate registration for `&models.ChatAdvancedThresholds{}`.
- `fuku/modules/advthresholds.go` — `advThresholdsLookup`, `advThresholdsFor`, 12 resolved getters, `applyAdvancedThreshold`, `resetAdvancedThreshold`, `resetAdvancedArea`.
- `fuku/modules/raid_counters.go`, `fuku/modules/raid_join.go`, `fuku/modules/raid_flood.go`, `fuku/modules/raid_ai.go`, `fuku/modules/raid_ai_count.go` — Per-group raid windows, flood share/floor, derived TTLs, and all 5 `raidRecentWindow` sites.
- `fuku/modules/advthresholds_raid_test.go` — `TestAdvancedThresholdDefaultsMatchConstants` and `TestAdvancedThresholdsRaidWiring`.
- `Makefile` — Added `TestAdvancedThresholdsMigrationDefaults`, `TestAdvancedThresholdsCRUD`, and `./fuku/db/advthresholds` to `test-postgres-integrity`.

## Decisions Made

- Kept `recordRaidFloodItem` and `recordRaidAIVerdict` delegating to `recordRaidFloodItemWith` / `recordRaidAIVerdictWith` with default constants while hot-path callers (`raidFloodCountUpdate`, `raidAIRecordVerdict`) read `advThresholdsFor` once per event after enabled/exemption gates, preserving backward compatibility with existing unit tests.
- Added `TestAdvancedThresholdsMigrationStatements` alongside `TestAdvancedThresholdsMigrationDefaults` (matching Phase 27's `manual_command_tuning_postgres_test.go`) so migration syntax, inline constraints, absence of FKs, and sort order are also verified without PostgreSQL.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Shared Phase 28 storage, `skipLocal` cache, service/apply layer, and raid tracer slice are complete and verified under `-race`.
- Ready for Plan 28-03 (AI spam bars & memberreq cap/pause wiring) and Plan 28-04 (lockdown reminder & captcha extended timeout wiring).

## Self-Check: PASSED

- All 9 created files verified present on disk (`migrations/20261123120000_add_advanced_thresholds.sql`, `fuku/db/models/advthresholds.go`, `fuku/db/advthresholds/repository.go`, `fuku/db/advthresholds/testmain_test.go`, `fuku/db/advthresholds/repository_test.go`, `fuku/db/backup/advthresholds_backup_test.go`, `fuku/db/migrations/advanced_thresholds_postgres_test.go`, `fuku/modules/advthresholds.go`, `fuku/modules/advthresholds_raid_test.go`).
- All 3 task commits (`29922fae`, `5f58b224`, `f96d4b74`) verified as ancestors of HEAD with 0 accidental file deletions.
- `fuku/modules/advthresholds_phase28_legacy_request_test.go`, `go.mod`, and `go.sum` verified untouched.
- Verification commands all passed under `-race`:
  - `go test -tags testtools -race -count=1 ./fuku/db/advthresholds` — PASS
  - `go test -tags testtools -race -count=1 -run '^TestSkipLocal' ./fuku/db/cache` — PASS
  - `go test -tags testtools -race -count=1 -run '^TestAdvThresholdsExcludedFromBackup$' ./fuku/db/backup` — PASS
  - `go test -tags testtools -race -count=1 -run '^(TestAdvancedThresholdsMigrationStatements|TestAdvancedThresholdsMigrationDefaults)$' ./fuku/db/migrations` — PASS (PG-only test skips cleanly when `DATABASE_URL` is unset)
  - `go test -tags testtools -race -count=1 -run '^(TestAdvancedThresholdDefaultsMatchConstants|TestAdvancedThresholdsRaidWiring|TestLegacyPhase28DefaultThresholds)$' ./fuku/modules` — PASS
  - `CGO_ENABLED=0 go build ./...` — PASS
