---
phase: 33-smarter-static-reactions-highlights-archive-and-backup-parity
plan: 08
subsystem: modules
tags: [backup, settings-ui, docs, makefile, agents-md, verification]

# Dependency graph
requires:
  - phase: 33-smarter-static-reactions-highlights-archive-and-backup-parity
    provides: "Smarter static reactions & cat stickers (33-04), Highlights Archive runtime (33-05), BackupModulePresence & ImportReport.ArchiveChannelSkipped (33-06), and Presence UI & ph sub-page (33-07)"
provides:
  - "formatBackupImportSuccess ArchiveChannelSkipped hint rendering in xenobot/modules/backup.go"
  - "21-module backup mask (0x1fffff) test & comment parity across backup_test, settings_backup, settings_backup_test, settings_phase30_test, and phase30_e2e_test"
  - "Makefile test-postgres-integrity Phase 33 coverage, synced docs/src/content/docs/commands/presence/index.md, and Phase 33 AGENTS.md documentation"
  - "Full Phase 33 verification gate green across build, translations, docs, lint, and -race suites with copy_settings.go and reactions_legacy_request_test.go untouched"
affects: [phase-34, phase-35, verify-work]

# Actuals (#2632)
actuals:
  tokens: 11647
  tasks: 3
  commits: 2
plan_head_before: e9a7095e3e414fb64bdeda80544e549a3fc86f2c
plan_head_after: 9b17f2f419adc4e410362f698d6d38d4874c0404

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Stateless 21-bit lowercase hex backup module mask (m=0..1fffff) derived dynamically from backup.AllExportableModules() with ArchiveChannelSkipped import hint"

key-files:
  created:
    - docs/src/content/docs/commands/presence/index.md
  modified:
    - xenobot/modules/backup.go
    - xenobot/modules/backup_test.go
    - xenobot/modules/settings_backup.go
    - xenobot/modules/settings_backup_test.go
    - xenobot/modules/settings_phase30_test.go
    - xenobot/modules/phase30_e2e_test.go
    - Makefile
    - AGENTS.md

key-decisions:
  - "Updated hardcoded 20-module / fffff / 100000 mask assertions in settings_backup_test.go, settings_phase30_test.go, and phase30_e2e_test.go to 21-module dynamic/0x1fffff equivalents (and 200000 out-of-range test) so Backup UI and /export parity stay green"

patterns-established:
  - "Use settingsBackupFormatMask(settingsBackupAllMask) and len(backup.AllExportableModules()) in Backup parity tests so adding exportable modules avoids brittle mask-literal drift"

requirements-completed:
  - COMM-01
  - COMM-02
  - COMM-03
  - COMM-04
  - COMM-05

coverage:
  - id: D1
    description: "formatBackupImportSuccess appends backup_import_archive_skipped when report.ArchiveChannelSkipped is true, and Settings Backup page ba handles the 21-bit module mask (0x1fffff) including backup_module_presence across all 8 locales."
    requirement: COMM-05
    verification:
      - kind: unit
        ref: "xenobot/modules/backup_test.go#TestSharedBackupApplyHelpers"
        status: pass
      - kind: unit
        ref: "xenobot/modules/settings_backup_test.go#TestSettingsBackupPage"
        status: pass
      - kind: integration
        ref: "xenobot/modules/phase30_e2e_test.go#TestPhase30EndToEnd"
        status: pass
    human_judgment: false
  - id: D2
    description: "xenobot/modules/copy_settings.go and xenobot/modules/reactions_legacy_request_test.go remain 100% untouched."
    requirement: COMM-01
    verification:
      - kind: other
        ref: "git diff -- xenobot/modules/copy_settings.go xenobot/modules/reactions_legacy_request_test.go"
        status: pass
      - kind: unit
        ref: "xenobot/modules/reactions_legacy_request_test.go#TestLegacyReactionRequests"
        status: pass
      - kind: unit
        ref: "xenobot/modules/settings_copy_test.go#TestSettingsCopy"
        status: pass
    human_judgment: false
  - id: D3
    description: "Makefile PG integrity list, generated Presence docs (make check-docs), AGENTS.md Phase 33 invariants, CGO_ENABLED=0 build, 8-locale translations, golangci-lint, and targeted -race suites across db/*, i18n, utils/helpers, and modules all pass."
    requirement: COMM-04
    verification:
      - kind: other
        ref: "CGO_ENABLED=0 go build ./... && make check-translations && make check-docs && GOTOOLCHAIN=go1.26.0 make lint"
        status: pass
      - kind: integration
        ref: "xenobot/modules/presence_archive_test.go#TestPresence"
        status: pass
      - kind: integration
        ref: "xenobot/modules/reactions_test.go#TestStaticReactions"
        status: pass
    human_judgment: false

# Metrics
duration: 5 min
completed: 2026-11-27
status: complete
---

# Phase 33 Plan 08: Backup UI Parity, Docs Sync & Verification Gate Summary

**Wired `ArchiveChannelSkipped` and the 21-module (`0x1fffff`) backup mask across `backup.go`, `settings_backup.go`, and Phase 30/Backup test suites, synced Presence command docs and `AGENTS.md`, and passed the complete Phase 33 build, translation, docs, lint, and `-race` verification gate with `copy_settings.go` and `reactions_legacy_request_test.go` untouched.**

## Performance

- **Duration:** 5 min
- **Started:** 2026-11-27T07:11:20Z
- **Completed:** 2026-11-27T07:16:44Z
- **Tasks:** 3
- **Files modified:** 9 (1 created, 8 modified)

## Accomplishments
- Updated `formatBackupImportSuccess` in `xenobot/modules/backup.go` to append `backup_import_archive_skipped` when `report.ArchiveChannelSkipped` is true, updated `xenobot/modules/settings_backup.go` 21-bit (`m=0..1fffff`) comments, extended `xenobot/modules/backup_test.go` (including `ChatPresence`/`ChatPresenceArchived` cleanup), and verified all 21 modules + `ArchiveChannelSkipped` across all 8 locales in `xenobot/modules/settings_backup_test.go`.
- Added `TestStaticReactionOptionsMigrationDefaults`, `TestChatPresenceArchiveMigrationDefaults`, and `TestBackupPresenceArchiveStripped` (plus `./xenobot/db/reactions` and `./xenobot/db/presence`) to `Makefile` `test-postgres-integrity`, ran `make generate-docs` to create `docs/src/content/docs/commands/presence/index.md` (`make check-docs` zero drift), and updated `AGENTS.md` with Phase 33 invariants.
- Confirmed `xenobot/modules/copy_settings.go` and `xenobot/modules/reactions_legacy_request_test.go` have zero Phase-33 diffs and passed the full Phase 33 verification gate (`CGO_ENABLED=0 go build ./...`, `make check-translations`, `make check-docs`, `GOTOOLCHAIN=go1.26.0 make lint`, `make test-shard-check`, and `-race` tests across `db/migrations`, `db/reactions`, `db/presence`, `db/cache`, `db/backup`, `i18n`, `utils/helpers`, and `modules`).

## Task Commits

Each task was committed atomically:

1. **Task 1: Wire ArchiveChannelSkipped in backup.go and 21-module mask coverage in settings_backup.go** - `25e6844b` (feat)
2. **Task 2: Update Makefile PG integrity list, sync generated docs, and update AGENTS.md** - `9b17f2f4` (docs)
3. **Task 3: Run full Phase 33 verification gate and verify protected files untouched** - verification-only gate (no code changes)

## Files Created/Modified
- `xenobot/modules/backup.go` - Appends `backup_import_archive_skipped` hint when `report.ArchiveChannelSkipped` is true.
- `xenobot/modules/backup_test.go` - Cleans up `models.ChatPresence`, `models.ChatPresenceArchived`, and `presence.CacheKey`, and tests `ArchiveChannelSkipped` formatting.
- `xenobot/modules/settings_backup.go` - Updated 21-bit (`m=0..1fffff`) / 21-module documentation comments.
- `xenobot/modules/settings_backup_test.go` - Updated 21-module (`1fffff`, `200000` invalid mask) assertions and 8-locale `ArchiveChannelSkipped` screen-fit checks.
- `xenobot/modules/settings_phase30_test.go` - Updated hardcoded `"fffff"` all-mask calls to `settingsBackupFormatMask(settingsBackupAllMask)`.
- `xenobot/modules/phase30_e2e_test.go` - Updated hardcoded `"fffff"` and `20` module counts to `settingsBackupFormatMask(settingsBackupAllMask)` and `len(backup.AllExportableModules())`.
- `Makefile` - Extended `test-postgres-integrity` with Phase 33 migration/backup tests and `reactions`/`presence` DB packages.
- `AGENTS.md` - Documented Phase 33 invariants (zero new shutdown drains, `reaction_rules:` `skipLocal`, group `8` safety gate/Unicode boundaries/cat stickers/`ctx.Data` handoff, group `0`+`11` `/setarchive`/`/unsetarchive` & `chat_presence_archived`, `BackupModulePresence` 21st module, `copy_settings.go` untouched).
- `docs/src/content/docs/commands/presence/index.md` - Generated Presence commands documentation (`/setarchive`, `/unsetarchive`).

## Decisions Made
- Updated `settings_phase30_test.go` and `phase30_e2e_test.go` to use `settingsBackupFormatMask(settingsBackupAllMask)` and `len(backup.AllExportableModules())` rather than hardcoded `"fffff"` and `20` so Backup UI and `/export` parity tests remain accurate for all 21 exportable modules.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated hardcoded 20-module / `fffff` literals in Phase 30 Backup parity tests**
- **Found during:** Task 1 (`go test -tags testtools -race -count=1 ./xenobot/modules -run 'TestBackup|TestSettingsBackup|TestPhase30|TestSettingsCopy'`)
- **Issue:** `xenobot/modules/settings_backup_test.go` treated `"100000"` as an out-of-range mask and asserted `"fffff"` / `20` modules; `xenobot/modules/phase30_e2e_test.go` and `xenobot/modules/settings_phase30_test.go` also used hardcoded `"fffff"` and `20`, causing `TestPhase30EndToEnd` to mismatch between page export (`m=fffff` -> 20 modules) and `/export` (21 modules).
- **Fix:** Updated `"100000"` -> `"200000"` as the invalid mask in `settings_backup_test.go` and switched all-mask test references to `"1fffff"` / `settingsBackupFormatMask(settingsBackupAllMask)` and `len(backup.AllExportableModules())`.
- **Files modified:** `xenobot/modules/settings_backup_test.go`, `xenobot/modules/settings_phase30_test.go`, `xenobot/modules/phase30_e2e_test.go`
- **Verification:** `TestSettingsBackupPage`, `TestPhase30Codes`, `TestPhase30ReadsCreateNothing`, `TestPhase30StaffPrivacyAndLogs`, and `TestPhase30EndToEnd` all passed under `-race`.
- **Committed in:** `25e6844b`

---

**Total deviations:** 1 auto-fixed (Rule 1 - Bug)
**Impact on plan:** Necessary to bring Phase 30 / Backup parity tests in sync with the 21st exportable module added in `33-06`.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Verification
- `git diff -- xenobot/modules/copy_settings.go xenobot/modules/reactions_legacy_request_test.go` — empty (0 changes; last commit `85d697cd`).
- `CGO_ENABLED=0 go build ./...` — PASS.
- `make check-translations` — PASS (1070 keys across all 8 locales).
- `make check-docs` — PASS (zero drift).
- `GOTOOLCHAIN=go1.26.0 make lint` — PASS (0 issues); `GOTOOLCHAIN=go1.26.0 golangci-lint run --build-tags testtools ./xenobot/modules/... ./xenobot/db/...` — PASS (0 issues).
- `make test-shard-check` — PASS.
- `go test -tags testtools -race -count=1 ./xenobot/db/migrations ./xenobot/db/reactions ./xenobot/db/presence ./xenobot/db/cache ./xenobot/db/backup ./xenobot/i18n` — PASS.
- `go test -tags testtools -race -count=1 ./xenobot/utils/helpers -run 'TestNoRawNewCommand'` — PASS.
- `go test -tags testtools -race -count=1 ./xenobot/modules -run 'TestLegacyReactionRequests|TestStaticReactions|TestCatSticker|TestPresence|TestShutdownEndsPresenceReaction|TestSettingsPresence|TestSettingsReactions|TestSettingsListReactionsFits|TestSettingsParityReactionAdd|TestSettingsAuthorityParityReactions|TestSettingsBackup|TestBackup|TestSettingsCopy|TestSettingsMain|TestPhase30Codes|TestPhase30EndToEnd|TestPhase33Codes|TestSettingsPhase19Codes|TestHelpModuleNamesExist'` — PASS (`5.782s`).

## Next Phase Readiness
- Phase 33 (`COMM-01`..`COMM-05`) is complete and verified across all 8 plans; ready for `/gsd-verify-work 33` or Phase 34 planning.

## Self-Check: PASSED

- All created/modified files (`xenobot/modules/backup.go`, `xenobot/modules/backup_test.go`, `xenobot/modules/settings_backup.go`, `xenobot/modules/settings_backup_test.go`, `xenobot/modules/settings_phase30_test.go`, `xenobot/modules/phase30_e2e_test.go`, `Makefile`, `AGENTS.md`, `docs/src/content/docs/commands/presence/index.md`) verified present on disk.
- Both task commits (`25e6844b`, `9b17f2f4`) verified as ancestors of `HEAD`.

---
*Phase: 33-smarter-static-reactions-highlights-archive-and-backup-parity*
*Completed: 2026-11-27*
