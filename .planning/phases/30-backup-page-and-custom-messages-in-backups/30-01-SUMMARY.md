---
phase: 30-backup-page-and-custom-messages-in-backups
plan: 01
subsystem: testing
tags: [golden-tests, msg-11, backup, export, import, reset, event-messages, backward-compat, race]

requires:
  - phase: 29-plain-tone-and-copy-settings
    provides: Baseline commit 1da5af6f (production code baseline 25a4dca7) prior to Phase 30 production edits
provides:
  - Standalone MSG-11 characterization golden test suite pinning /export, /import, and /reset command replies, permission/rate-limit refusals, confirm/cancel cards, old-export backward compatibility, and unconfigured-chat default /ban output in TestLegacyPhase30Baseline
affects: [30-02, 30-03, 30-04, 30-05]

tech-stack:
  added: []
  patterns:
    - "In-memory backupRoundTripFunc transport on backupDownloadHTTPClient (restored in t.Cleanup) with zero localhost TCP sockets"
    - "Explicit module arguments (/export notes rules, /reset notes rules) so adding BackupModuleMessages in Plan 30-02 never breaks the Wave 1 golden"
    - "MSG-11 standalone characterization golden commit ahead of all Phase 30 production changes"

key-files:
  created:
    - fuku/modules/phase30_legacy_request_test.go
  modified: []

key-decisions:
  - "Used explicit module arguments (/export notes rules, /reset notes rules) rather than asserting len(backup.AllExportableModules()) == 19 so adding BackupModuleMessages = \"messages\" in Plan 30-02 preserves 100% golden compatibility without editing phase30_legacy_request_test.go."
  - "Intercepted backup file downloads via in-memory backupRoundTripFunc on backupDownloadHTTPClient (restored in t.Cleanup) and pinned UTC timestamp formatting via phase29WithFixedUTCClock so all 5 subtests run in < 1 s with zero localhost TCP dials and full timezone determinism."

patterns-established:
  - "phase30NewEnglishChat / phase30CleanupChat / phase30NewBackupClient: isolated per-subtest English group fixture with full DB, cache, pending-map, and backup rate-limiter cleanup."

requirements-completed: []

status: complete
plan_head_before: 25f6b4a2bddf208754cd53bb67ce1822f739ce20
plan_head_after: f73400f4aed13d9b0cca990d77c9d0dea6d20a5b
commits: 1

actuals:
  tokens: 6395
  tasks: 2
  commits: 1

duration: 6min
completed: 2026-11-23
---

# Phase 30 Plan 01: Phase 30 Backup Commands and Old-Export Characterization Goldens Summary

**Standalone MSG-11 characterization golden suite (`TestLegacyPhase30Baseline`) freezing `/export`, `/import`, and `/reset` replies, owner/admin gates, cooldown refusals, confirm/cancel flows, old-export backward compatibility without `"messages"`, and unconfigured-chat default `/ban` output on unchanged baseline `1da5af6f`, committed alone before any Phase 30 production edit**

## Performance

- **Duration:** 6 min
- **Started:** 2026-11-23T20:27:11Z
- **Completed:** 2026-11-23T20:33:09Z
- **Tasks:** 2
- **Files modified:** 1 (created `fuku/modules/phase30_legacy_request_test.go`; 0 production files touched)

## Accomplishments

- Created `fuku/modules/phase30_legacy_request_test.go` (`//go:build testtools`) with `TestLegacyPhase30Baseline` containing 5 flat subtests recorded on unchanged baseline `1da5af6f` (production code baseline `25a4dca7`):
  1. `export_command_private_delivery_refusals_and_rate_limit` — seeds `notes` and `rules`, invokes `/export notes rules`, verifies 1 `sendDocument` to `owner.Id` (never `chat.Id`), decodes the exported JSON via `backup.BackupFormatFromJSON`, pins the exact HTML caption including `(UTC)`, verifies immediate cooldown refusal (`⏳ Wait <b>...</b> before exporting again.`), releases cooldown to pin unknown-module refusal (`❌ No exportable modules found for this chat.`), and simulates 403 Forbidden on `sendDocument` to pin the private-start reply and cooldown release on retry.
  2. `import_command_owner_gate_confirm_card_skipped_report_and_cancel` — pins non-owner admin refusal, no-reply refusal, non-`.json` document refusal with 0 `getFile` calls, valid `.json` download via in-memory `backupRoundTripFunc` (zero localhost TCP dials) with exact 3-module confirmation card (`rules`, `filters`, `logchannels`) and `ci`/`xi` buttons, confirmed execution pinning exact success + skipped-entry (`javascript:alert(1)` button URL) + missing-log-channel hints and DB persistence, expired replay refusal, and Cancel (`xi`) callback answer/edit preserving existing rules.
  3. `reset_command_owner_gate_confirm_card_and_execution` — pins non-owner admin owner-gate refusal, owner `/reset notes rules` confirmation card and `cr`/`xr` buttons, confirmed execution pinning exact 2-module reset reply and DB clearance of both notes and rules, expired replay refusal, and Cancel (`xr`) callback answer/edit preserving seeded data.
  4. `old_export_without_messages_imports_cleanly_and_preserves_custom_messages` — seeds a target chat with a custom `ban` group event message and options (`PMDetails: true, PlainTone: true`), imports a pre-Phase-30 JSON backup containing only `["rules", "greetings"]` via `backup.ImportChatDataReport(dstChat, oldBkp, nil)`, and verifies clean import with `Skipped == 0` while leaving `chat_event_messages` and `chat_event_message_options` untouched.
  5. `unconfigured_chat_default_ban_reply_unchanged` — verifies `banDefaultReply(tr, phase29FixedDeterministicInput())` and unconfigured-chat `postEventSent` match `goldenP29GroupDefaults[eventmsg.EventBan]` byte for byte.
- Verified `gofmt`, `go vet -tags testtools ./fuku/modules`, `CGO_ENABLED=0 go build ./...`, `TZ=UTC go test -tags testtools -race -count=1 -run '^TestLegacyPhase30Baseline$' -v ./fuku/modules` (5/5 subtests passing in 0.87 s with 0 skips), zero production edits since `1da5af6f`, and unchanged `go.mod`/`go.sum`.
- Committed `fuku/modules/phase30_legacy_request_test.go` alone in `f73400f4`.

## Task Commits

1. **Task 1 & Task 2: Record and commit Phase 30 backup commands and old-export characterization goldens alone** - `f73400f4` (`test(30-01): pin backup commands and old-export baseline before Phase 30`)

## Files Created/Modified

- `fuku/modules/phase30_legacy_request_test.go` — Standalone MSG-11 characterization golden test suite (`TestLegacyPhase30Baseline`) pinning `/export`, `/import`, `/reset` command behavior, refusals, confirm/cancel cards, old-export backward compatibility, and unconfigured-chat default `/ban` output.

## Decisions Made

- Used explicit module lists (`/export notes rules`, `/reset notes rules`, and explicit `modules` in backup JSON) instead of asserting `len(backup.AllExportableModules()) == 19`, so Plan 30-02's addition of `BackupModuleMessages = "messages"` keeps `TestLegacyPhase30Baseline` 100% valid and untouched.
- Used `backupRoundTripFunc` on `backupDownloadHTTPClient` (restored in `t.Cleanup`) rather than `httptest.NewServer` so the golden suite opens zero localhost TCP listeners, and pinned UTC formatting via `phase29WithFixedUTCClock` so export caption timestamps (`... (UTC)`) are deterministic across host timezones.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- MSG-11 golden baseline is locked in standalone commit `f73400f4` with zero production changes since `1da5af6f`.
- Ready for Wave 2 (Plan 30-02: `eventmsg.ReplaceAllValidatedTx`, `BackupModuleMessages = "messages"` export/import/reset in `fuku/db/backup`, and backup test updates).

## Self-Check: PASSED

- `fuku/modules/phase30_legacy_request_test.go`: FOUND
- Commit `f73400f4`: FOUND (1 file changed, 0 deletions, 0 production files modified since `1da5af6f`)
- `TestLegacyPhase30Baseline`: 5/5 subtests PASSED under `-race` with 0 skips
- `CGO_ENABLED=0 go build ./...`: PASSED
- `git diff --exit-code 1da5af6f -- go.mod go.sum`: PASSED
