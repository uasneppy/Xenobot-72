---
phase: 29-plain-tone-and-copy-settings
plan: 01
subsystem: testing
tags: [golden-tests, msg-11, plain-tone, copy-settings, event-messages, pm-copy, miniredis, race]

requires:
  - phase: 28-advanced-thresholds
    provides: Baseline commit ca060519 prior to Phase 29 production edits
provides:
  - Standalone MSG-11 characterization golden test suite pinning all 23 tone-off group defaults, all 18 tone-off PM defaults, unconfigured postEventSent behavior, and uncopied-group isolation in TestLegacyPhase29Baseline
affects: [29-02, 29-03, 29-04, 29-05, 29-06]

tech-stack:
  added: []
  patterns:
    - "Signature-decoupled PM default golden capture via postEventSent + in-process miniredis (zero direct calls to composeEventPM or eventPMDefaultText)"
    - "MSG-11 standalone characterization golden commit ahead of all Phase 29 production changes"

key-files:
  created:
    - fuku/modules/phase29_legacy_request_test.go
  modified: []

key-decisions:
  - "Drove tone_off_all_18_pm_defaults through postEventSent + waitEventPM over phase28InProcessMiniredis and a fake moduleBotClient so the golden stays completely decoupled from internal PM composer signatures modified in Plan 29-02 and requires zero localhost TCP dials."
  - "Pinned deterministic UTC timestamps via punishNow and chatTimezoneLookup test hooks (restored in t.Cleanup) so all 23 group defaults and all 18 PM defaults are 100% deterministic across host timezones."

patterns-established:
  - "phase29WithFixedUTCClock(t, now): deterministic UTC clock helper swapping punishNow and chatTimezoneLookup with t.Cleanup restoration."

requirements-completed: [TUNE-08, TUNE-09]

status: complete
plan_head_before: 90a286c8593ac9690588ac35a6c2de559d297905
plan_head_after: b4e8da4898c43f9e7e27d0b966b2257c7463ce0e
commits: 1

actuals:
  tokens: 5424
  tasks: 2
  commits: 1

duration: 6min
completed: 2026-11-23
---

# Phase 29 Plan 01: Phase 29 Tone-Off and Uncopied-Group Characterization Goldens Summary

**Standalone MSG-11 characterization golden suite (`TestLegacyPhase29Baseline`) freezing all 23 tone-off group defaults, all 18 tone-off PM defaults, unconfigured `postEventSent` delivery, and uncopied-group isolation on unchanged commit `ca060519`, committed alone before any Phase 29 production edit**

## Performance

- **Duration:** 6 min
- **Started:** 2026-11-23T13:36:21Z
- **Completed:** 2026-11-23T13:42:00Z
- **Tasks:** 2
- **Files modified:** 1 (created `fuku/modules/phase29_legacy_request_test.go`; 0 production files touched)

## Accomplishments

- Created `fuku/modules/phase29_legacy_request_test.go` (`//go:build testtools`) with `TestLegacyPhase29Baseline` containing 4 subtests recorded on unchanged commit `ca060519`:
  1. `tone_off_all_23_group_defaults` — renders all 23 milestone v1.2 group events across Manual, Automatic, Member requirements, and Lockdown families in English with deterministic UTC clock and sample inputs, asserting byte-for-byte equality against frozen `goldenP29GroupDefaults`.
  2. `tone_off_all_18_pm_defaults` — enables only the PM switch for all 18 events in `eventPMSpecs` (with 0 `chat_event_message_options` rows), drives delivery via `postEventSent` + `waitEventPM` over `phase28InProcessMiniredis` (zero localhost TCP dials, zero direct references to internal PM composers), and asserts byte-for-byte equality against frozen `goldenP29PMDefaults`.
  3. `tone_off_post_event_sends_unchanged_default` — exercises `postEventSent` for `EventBan` on an unconfigured chat, asserts the sent text matches `goldenP29GroupDefaults[eventmsg.EventBan]` byte for byte, and asserts 0 rows are created in `chat_event_messages` or `chat_event_message_options`.
  4. `uncopied_group_isolation_baseline` — seeds `srcChat` across all copy-scope tables (event messages, options, warn tuning, antiflood, blacklist, escalation, manual defaults, reason shortcuts, advanced thresholds), leaves `dstChat` unconfigured except one `approvals` row and one `"keepword"` blacklist entry, reads `srcChat` via fresh getters, and asserts `dstChat` has 0 rows across all unconfigured tables (`antiflood.GetFloodContext(ctx, dstChat).Limit == 0`, no Staff Group link) while retaining `"keepword"` and its approval untouched.
- Verified `gofmt`, `go vet -tags testtools ./fuku/modules`, `CGO_ENABLED=0 go build ./...`, `go test -tags testtools -race -count=1 -run '^TestLegacyPhase29Baseline$' -v ./fuku/modules` (4/4 subtests passing in ~0.90 s with 0 skips), zero coupling to internal PM helper signatures, zero production edits since `ca060519`, and unchanged `go.mod`/`go.sum`.
- Committed `fuku/modules/phase29_legacy_request_test.go` alone in `b4e8da48`.

## Task Commits

1. **Task 1 & Task 2: Record and commit Phase 29 tone-off and uncopied-group characterization goldens alone** - `b4e8da48` (`test(29-01): pin tone-off defaults and uncopied-group baseline before Phase 29`)

## Files Created/Modified

- `fuku/modules/phase29_legacy_request_test.go` — Standalone MSG-11 characterization golden test suite (`TestLegacyPhase29Baseline`) pinning all 23 tone-off group defaults, all 18 tone-off PM defaults, unconfigured `postEventSent` delivery, and uncopied-group isolation.

## Decisions Made

- Drove `tone_off_all_18_pm_defaults` through `postEventSent` + `waitEventPM` over `phase28InProcessMiniredis` and `newModuleBotClient()` rather than calling internal PM composers directly, ensuring zero signature coupling to Plan 29-02's `plain bool` parameter additions while keeping wall time under 1 s and avoiding localhost TCP dials.
- Fixed deterministic UTC formatting via `punishNow` and `chatTimezoneLookup` test hooks (restored in `t.Cleanup`) so timed PM defaults (`23.11.2026 12:00 (UTC)`) are timezone-independent.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- MSG-11 golden baseline is locked in commit `b4e8da48` with zero production changes since `ca060519`.
- Ready for Wave 2 (Plan 29-02: `plain_tone` migration/model/repo, pure plain group/PM defaults, and 8-locale plain tone strings).

## Self-Check: PASSED

- `fuku/modules/phase29_legacy_request_test.go`: FOUND
- Commit `b4e8da48`: FOUND (1 file changed, 0 deletions, 0 production files modified since `ca060519`)
- `TestLegacyPhase29Baseline`: 4/4 subtests PASSED under `-race` with 0 skips
