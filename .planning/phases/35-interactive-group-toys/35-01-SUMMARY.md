---
phase: 35-interactive-group-toys
plan: 01
subsystem: database
tags: [postgres, gorm, migrations, msgpack, skiplocal, backup]

# Dependency graph
requires:
  - phase: 34-room-aware-conversations-and-frictionless-catchup
    provides: chat_presence schema, models.ChatPresence, presence repository, and BackupModulePresence
provides:
  - Append-only SQL migration adding referendum_enabled and inspect_enabled to chat_presence
  - GORM model fields ReferendumEnabled and InspectEnabled without default: tags
  - skipLocal-cached presence.Settings (msgpack rf/in), SetReferendumEnabled, SetInspectEnabled, and ReplacePortableTx parity
  - BackupModulePresence JSON export/import parity for referendum_enabled and inspect_enabled
affects: [35-04-settings-and-referendum, 35-05-inspect-and-docs]

actuals:
  tokens: 4451
  tasks: 3
  commits: 3

plan_head_before: be156f27507049c66ec984b32eb96cb38147c465
plan_head_after: f19a36ca2ad108d52d3526d85d1d559de6077581

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Idempotent ALTER TABLE chat_presence ADD COLUMN IF NOT EXISTS BOOLEAN NOT NULL DEFAULT FALSE without explicit BEGIN/COMMIT
    - Zero GORM default: tags on boolean model columns with column-targeted upserts dropping chat_presence:<chat>
    - Short msgpack tags (rf, in) for rolling-deploy cache safety

key-files:
  created:
    - migrations/20261201120000_add_presence_toys.sql
    - xenobot/db/migrations/chat_presence_toys_migration_test.go
  modified:
    - xenobot/db/models/chat_presence.go
    - xenobot/db/presence/repository.go
    - xenobot/db/presence/repository_test.go
    - xenobot/db/backup/types.go
    - xenobot/db/backup/backup.go
    - xenobot/db/backup/presence_backup_test.go
    - xenobot/db/backup/roundtrip_test.go

key-decisions:
  - "Added referendum_enabled and inspect_enabled strictly on admin-configurable chat_presence (leaving Owner-only chat_conversations and copy_settings.go untouched)."

patterns-established:
  - "Toy toggles follow the Phase 34 column-targeted upsert + skipLocal invalidation + BackupModulePresence portable field pattern."

requirements-completed: [TOYS-01, TOYS-03]

coverage:
  - id: D1
    description: "Idempotent SQL migration 20261201120000_add_presence_toys.sql and ChatPresence GORM boolean fields without default: tags"
    requirement: TOYS-01
    verification:
      - kind: unit
        ref: "xenobot/db/migrations/chat_presence_toys_migration_test.go#TestChatPresenceToysMigrationStatements"
        status: pass
      - kind: unit
        ref: "CGO_ENABLED=0 go build ./xenobot/db/models"
        status: pass
    human_judgment: false
  - id: D2
    description: "presence.Settings msgpack rf/in fields, Defaults, GetFresh, SetReferendumEnabled, SetInspectEnabled, and ReplacePortableTx"
    requirement: TOYS-01
    verification:
      - kind: unit
        ref: "xenobot/db/presence/repository_test.go#TestChatPresenceSettings"
        status: pass
    human_judgment: false
  - id: D3
    description: "BackupModulePresence export/import roundtrip for referendum_enabled and inspect_enabled with legacy JSON defaults and chat_conversations exclusion"
    requirement: TOYS-03
    verification:
      - kind: unit
        ref: "xenobot/db/backup/presence_backup_test.go#TestBackupPresenceArchiveStripped"
        status: pass
      - kind: unit
        ref: "xenobot/db/backup/roundtrip_test.go#TestAllModulesRoundTripEveryMeaningfulField"
        status: pass
      - kind: unit
        ref: "xenobot/db/backup/chatconv_backup_test.go#TestChatConversationsStayOutOfBackup"
        status: pass
    human_judgment: false

# Metrics
duration: 3min
completed: 2026-11-27
status: complete
---

# Phase 35 Plan 01: Presence Toys Schema, Repository, and Backup Parity Summary

**Added `referendum_enabled` and `inspect_enabled` on `chat_presence` across SQL migration, GORM model, `skipLocal`-cached repository writers, and `BackupModulePresence` export/import.**

## Performance

- **Duration:** 3 min
- **Started:** 2026-11-27T12:52:40Z
- **Completed:** 2026-11-27T12:55:01Z
- **Tasks:** 3
- **Files modified:** 9

## Accomplishments
- Created `migrations/20261201120000_add_presence_toys.sql` (strictly after `20261130120000_add_presence_room_and_catchup.sql`) with two idempotent `ALTER TABLE chat_presence ADD COLUMN IF NOT EXISTS ... BOOLEAN NOT NULL DEFAULT FALSE` statements and matching statement/PostgreSQL default tests.
- Extended `models.ChatPresence` (`xenobot/db/models/chat_presence.go`) with `ReferendumEnabled` and `InspectEnabled` carrying zero GORM `default:` tags so explicit `false` always persists.
- Extended `xenobot/db/presence` with `ReferendumEnabled` (`msgpack:"rf"`), `InspectEnabled` (`msgpack:"in"`), `Defaults()`, `GetFresh`, `SetReferendumEnabled`, `SetInspectEnabled`, `ReplacePortableTx`, and `defaultRow`, verified under `-race` including rolling-deploy legacy cache normalization.
- Extended `BackupModulePresence` (`PresencePortableBackup`, `exportPresenceData`, `importPresence`) and roundtrip/legacy backup tests while keeping `chat_conversations` excluded and `xenobot/modules/copy_settings.go` untouched.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add append-only SQL migration, migration tests, and ChatPresence model columns** - `d3d5a390` (feat)
2. **Task 2: Extend xenobot/db/presence repository with setters, msgpack rf/in tags, and ReplacePortableTx parity** - `674d527f` (feat)
3. **Task 3: Extend BackupModulePresence DTO, export/import wiring, and backup/roundtrip tests** - `f19a36ca` (feat)

## Files Created/Modified
- `migrations/20261201120000_add_presence_toys.sql` - Idempotent DDL adding `referendum_enabled` and `inspect_enabled` to `chat_presence`
- `xenobot/db/migrations/chat_presence_toys_migration_test.go` - Statement validation unit test and `XENOBOT_TEST_DATABASE`-gated PostgreSQL idempotency/default test
- `xenobot/db/models/chat_presence.go` - Added `ReferendumEnabled` and `InspectEnabled` boolean columns with no `default:` tags
- `xenobot/db/presence/repository.go` - Added `rf`/`in` msgpack fields, `Defaults`, `GetFresh`, `SetReferendumEnabled`, `SetInspectEnabled`, `ReplacePortableTx`, and `defaultRow` parity
- `xenobot/db/presence/repository_test.go` - Added explicit `false`/cache-invalidation, rolling-deploy cache normalization, and `ReplacePortableTx` tests
- `xenobot/db/backup/types.go` - Added `referendum_enabled` and `inspect_enabled` to `PresencePortableBackup`
- `xenobot/db/backup/backup.go` - Wired `ReferendumEnabled` and `InspectEnabled` into `exportPresenceData` and `importPresence`
- `xenobot/db/backup/presence_backup_test.go` - Verified export/import roundtrip and legacy JSON fallback to `false`
- `xenobot/db/backup/roundtrip_test.go` - Asserted `ReferendumEnabled` and `InspectEnabled` across `TestAllModulesRoundTripEveryMeaningfulField`

## Decisions Made
None - followed plan as specified.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- Note on test environment: `TestChatPresenceToysMigrationDefaults` in `xenobot/db/migrations` skipped cleanly because `XENOBOT_TEST_DATABASE=true` and `DATABASE_URL` were not set in the local unit environment; `TestChatPresenceToysMigrationStatements`, `xenobot/db/presence`, and `xenobot/db/backup` all ran and passed under `-race`, and `golangci-lint` reported 0 issues across all four touched packages.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Data layer (`chat_presence` migration, GORM model, `skipLocal` repository, and `BackupModulePresence` parity) is ready for Wave 2 plans (`35-04` and `35-05`).
- `STATE.md` and `ROADMAP.md` were left untouched per parallel Wave 1 execution instructions.

## Self-Check: PASSED

---
*Phase: 35-interactive-group-toys*
*Completed: 2026-11-27*
