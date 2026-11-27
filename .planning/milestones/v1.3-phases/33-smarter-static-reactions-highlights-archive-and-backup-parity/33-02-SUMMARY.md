---
phase: 33-smarter-static-reactions-highlights-archive-and-backup-parity
plan: 02
subsystem: database
tags: [postgres, gorm, migrations, presence, redis, msgpack, backup]

# Dependency graph
requires:
  - phase: 32-contextual-ai-emoji-reactions
    provides: chat_presence table, models.ChatPresence, skipLocal cache key chat_presence:<chat>, and xenobot/db/presence column-targeted upsert pattern
provides:
  - Idempotent SQL migration 20261129130000_add_presence_stickers_and_archive.sql adding cat_sticker_enabled, archive_* columns with inline CHECK constraints, and chat_presence_archived composite PK table
  - models.ChatPresence extended with CatStickerEnabled and Archive* fields (zero boolean default tags) and new models.ChatPresenceArchived
  - xenobot/db/presence column-targeted setters, rolling-deploy Settings normalization, TryClaimArchived durable deduplication, and ReplacePortableTx/ClearTx backup transaction helpers
affects:
  - 33-03 (Cat-Sticker Micro-Triggers in xenobot/modules/reactions.go & settings_presence.go)
  - 33-04 (Highlights Archive service, commands, and settings sub-page ph)
  - 33-05 (BackupModulePresence export/import/clear parity in xenobot/db/backup)

# Actuals (#2632)
actuals:
  tokens: 13522
  tasks: 3
  commits: 5
  plan_head_before: 8baa638fb8dc7b20b82856d19ad65f7ca89f12fb
  plan_head_after: adcaa512a430249ae4cd278f7779af6e665747d9

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Inline CHECK constraints on ADD COLUMN IF NOT EXISTS for idempotent PostgreSQL re-runs"
    - "Composite primary key (chat_id, message_id) + ON CONFLICT DO NOTHING claim returning RowsAffected == 1 for durable multi-replica deduplication"
    - "Rolling-deploy msgpack normalization in normalizeSettings() mapping zero/empty Phase 33 fields to safe defaults"
    - "Backup transactional helper ReplacePortableTx preserving target chat's archive_chat_id/archive_thread_id and flagging archiveSkipped"

key-files:
  created:
    - migrations/20261129130000_add_presence_stickers_and_archive.sql
    - xenobot/db/migrations/chat_presence_archive_migration_test.go
    - xenobot/db/models/chat_presence_archived.go
  modified:
    - xenobot/db/models/chat_presence.go
    - xenobot/db/presence/repository.go
    - xenobot/db/presence/repository_test.go
    - xenobot/db/presence/testmain_test.go

key-decisions:
  - "Checked ArchiveChatID != 0 inside a DB transaction in SetArchiveEnabled(true) before upserting archive_enabled=true so concurrent ClearArchiveTarget calls cannot race into an enabled-without-target state"
  - "Normalized ConsensusThreshold alongside AIReactMode and Phase 33 archive fields in shared normalizeSettings() used by Get(), GetFresh(), and ReplacePortableTx()"

patterns-established:
  - "upsertColumnsTx / upsertColumns shared helper for single- and multi-column upserts with post-commit skipLocal cache invalidation"
  - "TryClaimArchived durable claim pattern matching greeted_members without foreign keys"

requirements-completed: [COMM-02, COMM-03, COMM-04]

coverage:
  - id: D1
    description: "Append-only SQL migration 20261129130000_add_presence_stickers_and_archive.sql sorts after 20261129120000_add_static_reaction_options.sql, adds cat_sticker_enabled and Highlights Archive columns with inline CHECK constraints, and creates chat_presence_archived (chat_id, message_id) PK."
    requirement: COMM-02
    verification:
      - kind: unit
        ref: "xenobot/db/migrations/chat_presence_archive_migration_test.go#TestChatPresenceArchiveMigrationStatements"
        status: pass
    human_judgment: false
  - id: D2
    description: "models.ChatPresence carries zero GORM default: tags on boolean fields and models.ChatPresenceArchived enforces composite primary key (chat_id, message_id)."
    requirement: COMM-04
    verification:
      - kind: unit
        ref: "CGO_ENABLED=0 go build ./xenobot/db/..."
        status: pass
    human_judgment: false
  - id: D3
    description: "xenobot/db/presence exposes CatStickerEnabled and Highlights Archive column-targeted setters, SetArchiveTarget/ClearArchiveTarget, TryClaimArchived, ReplacePortableTx/ClearTx, and rolling-deploy normalization for pre-Phase-33 cached payloads."
    requirement: COMM-03
    verification:
      - kind: unit
        ref: "xenobot/db/presence/repository_test.go#TestChatPresenceSettings"
        status: pass
    human_judgment: false

# Metrics
duration: 3min
completed: 2026-11-27
status: complete
---

# Phase 33 Plan 02: Presence Stickers, Highlights Archive Schema & Repository Summary

**Append-only SQL migration, GORM models without boolean default tags, durable `(chat_id, message_id)` Highlights Archive deduplication (`TryClaimArchived`), rolling-deploy cache normalization, and transactional backup helpers (`ReplacePortableTx`/`ClearTx`) in `xenobot/db/presence`.**

## Performance

- **Duration:** 3 min
- **Started:** 2026-11-27T06:48:49Z
- **Completed:** 2026-11-27T06:51:44Z
- **Tasks:** 3
- **Files modified:** 7 (3 created, 4 modified)

## Accomplishments
- Added idempotent migration `migrations/20261129130000_add_presence_stickers_and_archive.sql` sorting strictly after `20261129120000_add_static_reaction_options.sql`, extending `chat_presence` with 7 columns (`cat_sticker_enabled`, `archive_enabled`, `archive_chat_id`, `archive_thread_id`, `archive_threshold`, `archive_emoji_mode`, `archive_delivery`) with inline named `CHECK` constraints and creating `chat_presence_archived` with `pk_chat_presence_archived (chat_id, message_id)`.
- Updated `models.ChatPresence` (keeping zero `default:` tags on all boolean columns) and added `models.ChatPresenceArchived` with composite primary key `(chat_id, message_id)`.
- Extended `xenobot/db/presence` with Phase 33 constants, validators (`ValidArchiveThreshold`, `ValidArchiveEmojiMode`, `ValidArchiveDelivery`), `Settings` msgpack fields + `normalizeSettings` for rolling-deploy safety, column-targeted setters (`SetCatStickerEnabled`, `SetArchiveEnabled`, `SetArchiveTarget`, `ClearArchiveTarget`, `SetArchiveThreshold`, `SetArchiveEmojiMode`, `SetArchiveDelivery`), `TryClaimArchived`, and backup transaction helpers (`ReplacePortableTx`, `ClearTx`).

## Task Commits

Each task was committed atomically on `main` (Note: `git rev-list --count plan_head_before..plan_head_after` is `5` because Wave 1 peer `33-01` committed 2 interleaved commits `01dfae9f` and `d7839418` in parallel):

1. **Task 1: Add append-only presence stickers/archive SQL migration, migration test, and GORM models** - `78b9a15d` (feat)
2. **Task 2: Extend xenobot/db/presence repository with CatSticker, Archive CRUD, TryClaimArchived, and backup tx helpers** - `18b8d6e2` (feat)
3. **Task 3: Update testmain and add repository tests for CatSticker, Archive, TryClaimArchived, and rolling-deploy normalization** - `adcaa512` (test)

## Files Created/Modified
- `migrations/20261129130000_add_presence_stickers_and_archive.sql` - Idempotent DDL for `chat_presence` Phase 33 columns and `chat_presence_archived` table
- `xenobot/db/migrations/chat_presence_archive_migration_test.go` - Statement shape test (`TestChatPresenceArchiveMigrationStatements`) and PostgreSQL opt-in defaults/CHECK/PK test (`TestChatPresenceArchiveMigrationDefaults`)
- `xenobot/db/models/chat_presence.go` - Extended `ChatPresence` GORM model with CatSticker and Archive fields
- `xenobot/db/models/chat_presence_archived.go` - Composite-PK deduplication GORM model for `chat_presence_archived`
- `xenobot/db/presence/repository.go` - Phase 33 constants, validators, normalizers, column-targeted setters, `TryClaimArchived`, and `ReplacePortableTx`/`ClearTx`
- `xenobot/db/presence/testmain_test.go` - Registered `&models.ChatPresenceArchived{}` in `testdb.Run`
- `xenobot/db/presence/repository_test.go` - Full coverage of new defaults, setters, unlinked enable refusal, SQL CHECK constraints, rolling-deploy cache normalization, `TryClaimArchived`, and `ReplacePortableTx`/`ClearTx`

## Decisions Made
- Wrapped `SetArchiveEnabled(ctx, chatID, true, by)` in a DB transaction reading `archive_chat_id` before upserting `archive_enabled=true` so concurrent `ClearArchiveTarget` calls cannot leave archiving enabled without a destination.
- Placed all portable field normalization (including `ConsensusThreshold` and Phase 33 archive defaults) into a single `normalizeSettings` helper shared by `Get`, `GetFresh`, and `ReplacePortableTx`.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Verification
- `go test -tags testtools -race -count=1 ./xenobot/db/presence ./xenobot/db/migrations -run 'TestChatPresence'` — passed (`xenobot/db/presence` 1.385s, `xenobot/db/migrations` 1.234s)
- `go test -tags testtools -race -count=1 ./xenobot/db/migrations ./xenobot/db/presence` — passed (`xenobot/db/migrations` 1.427s, `xenobot/db/presence` 1.454s; `TestChatPresenceArchiveMigrationDefaults` skipped without `XENOBOT_TEST_DATABASE=true`, SQLite tests exercised CHECK constraints and PK deduplication)
- `CGO_ENABLED=0 go build ./xenobot/db/...` — passed
- `GOTOOLCHAIN=go1.26.0 golangci-lint run ./xenobot/db/models/... ./xenobot/db/presence/... ./xenobot/db/migrations/...` — 0 issues

## Self-Check: PASSED

## Next Phase Readiness
- `xenobot/db/presence` and `models.ChatPresenceArchived` are ready for Wave 2 (`33-03` Cat-Sticker Micro-Triggers, `33-04` Highlights Archive service/UI, `33-05` BackupModulePresence integration).

---
*Phase: 33-smarter-static-reactions-highlights-archive-and-backup-parity*
*Completed: 2026-11-27*
