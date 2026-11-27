---
phase: 33-smarter-static-reactions-highlights-archive-and-backup-parity
plan: 06
subsystem: database
tags: [postgres, gorm, redis, backup, presence, reactions, security]

# Dependency graph
requires:
  - phase: 33-smarter-static-reactions-highlights-archive-and-backup-parity
    provides: "reactions Rule/validators (33-01) and presence.ReplacePortableTx/ClearTx/CacheKey (33-02)"
provides:
  - "BackupModulePresence = \"presence\" (21st exportable module) and PresencePortableBackup omitting archive_chat_id/archive_thread_id"
  - "ImportReport.ArchiveChannelSkipped, exportPresenceData, importPresence, and clearPresence in xenobot/db/backup"
  - "importReactions ChancePct/CooldownSecs normalization and dual-key cache invalidation (reactions + reaction_rules)"
  - "Security, portability, round-trip, and legacy normalization tests in xenobot/db/backup"
affects: [33-05, backup-ui, settings-backup]

# Actuals (#2632)
actuals:
  tokens: 6991
  tasks: 2
  commits: 4
plan_head_before: 872f4c79b1afb0b6410ba116d622d3177e4fbf9b
plan_head_after: 1d86834e6552ef92a54999316e1ba6eba2f7c4da

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "DTO-level field omission in PresencePortableBackup combined with presence.ReplacePortableTx target preservation and ArchiveChannelSkipped flagging"
    - "Dual-key post-commit cache invalidation for reactions:<chat> and reaction_rules:<chat> on import and clear"

key-files:
  created:
    - xenobot/db/backup/presence_backup_test.go
  modified:
    - xenobot/db/backup/types.go
    - xenobot/db/backup/backup.go
    - xenobot/db/backup/testmain_test.go
    - xenobot/db/backup/backup_test.go
    - xenobot/db/backup/roundtrip_test.go

key-decisions:
  - "Reused reactions.ValidChancePct/DefaultChancePct/ValidCooldownSecs in importReactions instead of duplicating range constants"
  - "Kept PresencePortableBackup JSON fields without omitempty so false booleans (e.g. retract_enabled=false) serialize explicitly and match models.ChatPresence"

patterns-established:
  - "Portable backup DTOs omit chat-bound Telegram target IDs at struct level so tampered JSON fields are ignored during decode before hitting repository TX helpers"

requirements-completed: [COMM-05]

coverage:
  - id: D1
    description: "BackupModulePresence (\"presence\") exports, imports, and clears portable chat_presence settings while stripping archive_chat_id/archive_thread_id and setting ImportReport.ArchiveChannelSkipped when unlinked."
    requirement: "COMM-05"
    verification:
      - kind: unit
        ref: "xenobot/db/backup/presence_backup_test.go#TestBackupPresenceArchiveStripped"
        status: pass
      - kind: unit
        ref: "xenobot/db/backup/roundtrip_test.go#TestAllModulesRoundTripEveryMeaningfulField"
        status: pass
    human_judgment: false
  - id: D2
    description: "importReactions normalizes legacy ChancePct == 0 to 100, clamps out-of-range ChancePct/CooldownSecs, round-trips Phase 33 reaction options, and invalidates both reactions and reaction_rules cache keys while keeping chat_conversations excluded."
    requirement: "COMM-05"
    verification:
      - kind: unit
        ref: "xenobot/db/backup/presence_backup_test.go#TestBackupReactionsLegacyNormalization"
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

# Phase 33 Plan 06: Backup & Restore Parity for Presence & Smarter Reactions Summary

**21st exportable backup module `BackupModulePresence` (`"presence"`) with archive target stripping (`ArchiveChannelSkipped`), `reactions` Phase 33 option round-trip and legacy `ChancePct == 0 -> 100` normalization, and dual-key cache invalidation**

## Performance

- **Duration:** 3 min
- **Started:** 2026-11-27T06:54:48Z
- **Completed:** 2026-11-27T06:58:00Z
- **Tasks:** 2
- **Files modified:** 6 (1 created, 5 modified)

## Accomplishments
- Added `BackupModulePresence = "presence"` (21 modules total in `AllExportableModules()`), `PresencePortableBackup` (explicitly omitting `archive_chat_id` and `archive_thread_id`), and `PresenceBackup` in `xenobot/db/backup/types.go`.
- Added `ImportReport.ArchiveChannelSkipped`, `exportPresenceData`, `importPresence` (delegating to `presence.ReplacePortableTx`), and `clearPresence` (delegating to `presence.ClearTx`) in `xenobot/db/backup/backup.go`, returning `presence.CacheKey(chatID)` for post-commit invalidation.
- Extended `importReactions` to reset `ID = 0`, normalize `ChancePct <= 0 || > 100` to `100`, clamp `CooldownSecs` outside `0..86400` to `0`, and invalidate both `reactions:<chat>` and `reaction_rules:<chat>` on import and clear.
- Added `models.ChatPresence` and `models.ChatPresenceArchived` to `xenobot/db/backup/testmain_test.go`, updated `TestModuleValidation`/`cleanupBackupChat`/`TestAllModulesRoundTripEveryMeaningfulField`, and created `xenobot/db/backup/presence_backup_test.go`.

## Task Commits

Each task was committed atomically on `main`:

1. **Task 1: Add BackupModulePresence, ArchiveChannelSkipped, and reaction normalization** - `18a5164d` (feat)
2. **Task 2: Update backup_test/roundtrip_test and add presence_backup_test** - `1fde7cb6` (test)

_Note: `plan_head_before..plan_head_after` counts 4 commits because Wave 2 peers `33-05` (`007f91e1`) and `33-04` (`1d86834e`) committed concurrently on disjoint files._

## Files Created/Modified
- `xenobot/db/backup/types.go` - Added `BackupModulePresence`, updated `AllExportableModules()` to 21 modules, and defined `PresencePortableBackup` / `PresenceBackup`.
- `xenobot/db/backup/backup.go` - Added `ImportReport.ArchiveChannelSkipped`, `exportPresenceData`, `importPresence`, `clearPresence`, and `importReactions`/`clearReactions` normalization & dual-key invalidation.
- `xenobot/db/backup/testmain_test.go` - Registered `models.ChatPresence` and `models.ChatPresenceArchived` in SQLite test `AutoMigrate`.
- `xenobot/db/backup/backup_test.go` - Updated `TestModuleValidation` (20 -> 21 modules) and `cleanupBackupChat` for `ChatPresence`, `ChatPresenceArchived`, and `reaction_rules`/`chat_presence` cache keys.
- `xenobot/db/backup/roundtrip_test.go` - Extended `TestAllModulesRoundTripEveryMeaningfulField` for Phase 33 `Reactions` options and `BackupModulePresence` portability + destination archive preservation.
- `xenobot/db/backup/presence_backup_test.go` - Added `TestBackupPresenceArchiveStripped` and `TestBackupReactionsLegacyNormalization`.

## Decisions Made
- Used `reactions.ValidChancePct`, `reactions.DefaultChancePct`, and `reactions.ValidCooldownSecs` in `importReactions` rather than duplicating bounds constants.
- Left `copy_settings.go` untouched (`chat_conversations` permanently excluded).

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- Initial `golangci-lint` run flagged `S1039: unnecessary use of fmt.Sprintf` on `tamperedJSON` in `xenobot/db/backup/presence_backup_test.go`; replaced with a plain string literal and re-verified with 0 lint issues.

## User Setup Required
None - no external service configuration required.

## Verification
- `go test -tags testtools -race -count=1 ./xenobot/db/backup` — PASS (1.647s; all tests including `TestAllModulesRoundTripEveryMeaningfulField`, `TestChatConversationsStayOutOfBackup`, `TestBackupPresenceArchiveStripped`, `TestBackupReactionsLegacyNormalization`)
- `go test -tags testtools -race -count=1 -run '^TestChatConversationsStayOutOfBackup$' ./xenobot/db/backup` — PASS (1.369s)
- `CGO_ENABLED=0 go build ./xenobot/db/...` — PASS
- `GOTOOLCHAIN=go1.26.0 golangci-lint run ./xenobot/db/backup/...` — PASS (0 issues)

## Next Phase Readiness
- `BackupModulePresence` and `ImportReport.ArchiveChannelSkipped` are ready for `xenobot/modules/backup.go` / `settings_backup.go` (`33-05` UI wiring).

## Self-Check: PASSED

- All 6 created/modified files verified on disk.
- Both task commits (`18a5164d`, `1fde7cb6`) verified as ancestors of `HEAD`.

---
*Phase: 33-smarter-static-reactions-highlights-archive-and-backup-parity*
*Completed: 2026-11-27*
