---
phase: 34-room-aware-conversations-and-frictionless-catch-up
plan: 01
subsystem: database
tags: [postgres, gorm, msgpack, redis, migrations, backup, presence]

requires:
  - phase: 33-static-reactions-highlights-archive-stickers-backup
    provides: chat_presence schema, skipLocal repository pattern, BackupModulePresence DTO and ReplacePortableTx
provides:
  - Idempotent SQL migration 20261130120000_add_presence_room_and_catchup.sql adding ambient_context, vocative_enabled, pm_brief_react
  - models.ChatPresence extended with AmbientContext, VocativeEnabled, PMBriefReact (no GORM default tags)
  - xenobot/db/presence Settings msgpack tags (ax, ve, pb), column-targeted setters, and ReplacePortableTx parity
  - BackupModulePresence export/import parity with legacy JSON false defaults and chat_conversations exclusion
affects:
  - 34-03 (uses presence.Settings AmbientContext/VocativeEnabled/PMBriefReact and setters)
  - 34-05 (uses presence.Settings and setters in Settings UI)

actuals:
  tokens: 6568
  tasks: 3
  commits: 3
  plan_head_before: b81bedadec509cfb90a5e3ba3d9dc056a8bc4ac9
  plan_head_after: dabb7f73

tech-stack:
  added: []
  patterns:
    - "No GORM default: tags on ChatPresence boolean columns so explicit Go false always persists"
    - "Column-targeted upserts with skipLocal cache invalidation for independent presence toggles"
    - "Short msgpack field tags (ax, ve, pb) backward-compatible with pre-Phase-34 cached payloads"

key-files:
  created:
    - migrations/20261130120000_add_presence_room_and_catchup.sql
    - xenobot/db/migrations/chat_presence_room_migration_test.go
  modified:
    - xenobot/db/models/chat_presence.go
    - xenobot/db/presence/repository.go
    - xenobot/db/presence/repository_test.go
    - xenobot/db/backup/types.go
    - xenobot/db/backup/backup.go
    - xenobot/db/backup/presence_backup_test.go
    - xenobot/db/backup/roundtrip_test.go

key-decisions:
  - "None - followed plan as specified"

patterns-established:
  - "Phase 34 room & catch-up booleans follow Phase 31/33 chat_presence column-targeted upsert and BackupModulePresence parity"

requirements-completed:
  - CHAT-02
  - CHAT-03
  - CHAT-05

coverage:
  - id: D1
    description: "Append-only idempotent SQL migration 20261130120000_add_presence_room_and_catchup.sql and models.ChatPresence columns without GORM default tags"
    requirement: CHAT-02
    verification:
      - kind: unit
        ref: "xenobot/db/migrations/chat_presence_room_migration_test.go#TestChatPresenceRoomMigrationStatements"
        status: pass
      - kind: unit
        ref: "CGO_ENABLED=0 go build ./xenobot/db/models"
        status: pass
    human_judgment: false
  - id: D2
    description: "xenobot/db/presence Settings (ax/ve/pb), SetAmbientContext, SetVocativeEnabled, SetPMBriefReact, and ReplacePortableTx parity"
    requirement: CHAT-03
    verification:
      - kind: unit
        ref: "xenobot/db/presence/repository_test.go#TestChatPresenceSettings"
        status: pass
    human_judgment: false
  - id: D3
    description: "BackupModulePresence DTO, export/import wiring, legacy JSON false defaults, and full backup roundtrip parity"
    requirement: CHAT-05
    verification:
      - kind: unit
        ref: "xenobot/db/backup/presence_backup_test.go#TestBackupPresenceArchiveStripped"
        status: pass
      - kind: unit
        ref: "xenobot/db/backup/roundtrip_test.go#TestAllModulesRoundTripEveryMeaningfulField"
        status: pass
      - kind: unit
        ref: "xenobot/db/backup/roundtrip_test.go#TestChatConversationsStayOutOfBackup"
        status: pass
    human_judgment: false

duration: 3 min
completed: 2026-11-27
status: complete
---

# Phase 34 Plan 01: Presence Room & Catch-Up Persistence Summary

**Extended `chat_presence` schema, GORM model, `skipLocal`-cached repository setters, and `BackupModulePresence` export/import parity for `ambient_context`, `vocative_enabled`, and `pm_brief_react`**

## Performance

- **Duration:** 3 min
- **Started:** 2026-11-27T07:28:33Z
- **Completed:** 2026-11-27T07:31:30Z
- **Tasks:** 3
- **Files modified:** 9 (2 created, 7 modified)

## Accomplishments
- Added append-only idempotent SQL migration `migrations/20261130120000_add_presence_room_and_catchup.sql` (sorting strictly after `20261129130000_add_presence_stickers_and_archive.sql`) and `models.ChatPresence` boolean fields (`AmbientContext`, `VocativeEnabled`, `PMBriefReact`) with zero GORM `default:` tags.
- Extended `xenobot/db/presence` `Settings` (`ax`, `ve`, `pb` msgpack tags), `Defaults()`, `GetFresh`, `defaultRow`, `ReplacePortableTx`, and added column-targeted setters (`SetAmbientContext`, `SetVocativeEnabled`, `SetPMBriefReact`) with `CacheKey(chatID)` invalidation and pre-Phase-34 cache compatibility.
- Extended `PresencePortableBackup`, `exportPresenceData`, and `importPresence` in `xenobot/db/backup` so the three toggles round-trip cleanly and legacy JSON payloads default missing keys to `false` while keeping `chat_conversations` excluded.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add append-only SQL migration, migration test, and ChatPresence model columns** - `27c6ff12` (feat)
2. **Task 2: Extend xenobot/db/presence repository with setters, cache tags, and ReplacePortableTx parity** - `013971a9` (feat)
3. **Task 3: Extend BackupModulePresence DTO, export/import wiring, and backup roundtrip tests** - `dabb7f73` (feat)

## Files Created/Modified
- `migrations/20261130120000_add_presence_room_and_catchup.sql` - Idempotent `ADD COLUMN IF NOT EXISTS` DDL for `ambient_context`, `vocative_enabled`, `pm_brief_react`.
- `xenobot/db/migrations/chat_presence_room_migration_test.go` - Statement validation and PostgreSQL opt-in default checks.
- `xenobot/db/models/chat_presence.go` - Extended `ChatPresence` model without GORM `default:` tags on new booleans.
- `xenobot/db/presence/repository.go` - Added `ax`/`ve`/`pb` fields, column-targeted setters, and `ReplacePortableTx` parity.
- `xenobot/db/presence/repository_test.go` - Added tests for setters, cache invalidation, pre-Phase-34 msgpack decoding, and `ReplacePortableTx`.
- `xenobot/db/backup/types.go` - Added `ambient_context`, `vocative_enabled`, `pm_brief_react` JSON fields to `PresencePortableBackup`.
- `xenobot/db/backup/backup.go` - Wired fields in `exportPresenceData` and `importPresence`.
- `xenobot/db/backup/presence_backup_test.go` - Verified export/import roundtrip and legacy JSON default-to-false behavior.
- `xenobot/db/backup/roundtrip_test.go` - Added assertions for the three toggles in `TestAllModulesRoundTripEveryMeaningfulField`.

## Decisions Made
None - followed plan as specified.

## Deviations from Plan
None - plan executed exactly as written.

## Issues Encountered
- `TestChatPresenceRoomMigrationDefaults` skipped in local run because `XENOBOT_TEST_DATABASE=true` / `DATABASE_URL` PostgreSQL environment is not active locally (all SQLite `testtools` repository/backup tests and migration statement checks ran and passed under `-race`, and `golangci-lint` reported 0 issues).

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `chat_presence` persistence, `xenobot/db/presence` setters, and `BackupModulePresence` parity are ready for Wave 2 plans (`34-03`, `34-05`).

## Self-Check: PASSED

---
*Phase: 34-room-aware-conversations-and-frictionless-catch-up*
*Completed: 2026-11-27*
