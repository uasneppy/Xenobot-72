---
phase: 34-room-aware-conversations-and-frictionless-catch-up
plan: 04
subsystem: modules
tags: [chat-log, redis, zset, timestamps, forum-topics, miniredis]

# Dependency graph
requires: []
provides:
  - "chatLogEntry.At (`json:\"at,omitempty\"`) populated from `msg.Date` in `chatLogEntryOf`"
  - "Original `At` preservation across edits in `chatLogEdit` via `chatLogEntryAt`"
  - "Thread-isolated Redis log query helpers: `chatLogEntryAt`, `chatLogRecent`, `chatLogSinceUser`, `chatLogSinceTime`"
affects:
  - "34-05 (Ambient room context in conversations)"
  - "34-06 (Reply-free /unrollshort and /unrolllong catch-up)"
  - "34-07 (✍ PM brief reaction trigger)"

# Actuals (#2632)
actuals:
  tokens: 10486
  tasks: 2
  commits: 4
plan_head_before: 013971a9156ddb8b1bae5fac271213a8fe0c5b90
plan_head_after: 1777bce3fd0c7bf89ab4c17272df38af91f389df

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Read-before-write timestamp preservation in chatLogEdit using ZRangeArgs ByScore"
    - "Strict entry.Thread == threadID isolation and legacy At <= 0 exclusion in time-window scans"

key-files:
  created: []
  modified:
    - xenobot/modules/chat_log.go
    - xenobot/modules/chat_log_test.go

key-decisions:
  - "Updated existing TestChatLogWatcher compact-field assertion (4 -> 5 fields, expecting At: msg.Date) under Rule 1 when CHAT-01 added the at JSON field."
  - "Scanned the full <=1000-entry ring in chatLogSinceTime without early break so mixed legacy At==0 or slightly out-of-order timestamps never truncate valid results."

patterns-established:
  - "Thread-isolated Redis ring readers bind ctx with chatLogReadTimeout and enforce entry.Thread == threadID + optional beforeMsgID > 0 upper bound."

requirements-completed:
  - CHAT-01

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "chatLogEntry stores At int64 (json:\"at,omitempty\") from msg.Date and preserves original At on edits via chatLogEntryAt."
    requirement: CHAT-01
    verification:
      - kind: unit
        ref: "xenobot/modules/chat_log_test.go#TestChatLogThreadAndTimeQueries/timestamp_recording_and_edit_preservation"
        status: pass
      - kind: unit
        ref: "xenobot/modules/chat_log_test.go#TestChatLogWatcher"
        status: pass
    human_judgment: false
  - id: D2
    description: "chatLogEntryAt, chatLogRecent, chatLogSinceUser, and chatLogSinceTime enforce strict entry.Thread == threadID isolation, respect beforeMsgID, and skip legacy At <= 0 entries in time-window scans."
    requirement: CHAT-01
    verification:
      - kind: unit
        ref: "xenobot/modules/chat_log_test.go#TestChatLogThreadAndTimeQueries/legacy_At==0_entries"
        status: pass
      - kind: unit
        ref: "xenobot/modules/chat_log_test.go#TestChatLogThreadAndTimeQueries/strict_thread_isolation,_beforeMsgID,_and_oldest-first_ordering"
        status: pass
    human_judgment: false

# Metrics
duration: 4min
completed: 2026-11-27
status: complete
---

# Phase 34 Plan 04: Timestamped & Thread-Isolated Chat Log Summary

**Added `msg.Date` creation timestamps (`at`), edit-time timestamp preservation, and strict topic-isolated Redis query helpers (`chatLogEntryAt`, `chatLogRecent`, `chatLogSinceUser`, `chatLogSinceTime`) to `xenobot/modules/chat_log.go`.**

## Performance

- **Duration:** 4 min
- **Started:** 2026-11-27T07:28:34Z
- **Completed:** 2026-11-27T07:32:15Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Added `At int64` (`json:"at,omitempty"`) to `chatLogEntry` and populated `At: msg.Date` in `chatLogEntryOf`.
- Updated `chatLogEdit` to read the existing score entry via `chatLogEntryAt` before writing so edits never overwrite or clear original creation timestamps.
- Implemented `chatLogEntryAt`, `chatLogRecent`, `chatLogSinceUser`, and `chatLogSinceTime` with `chatLogReadTimeout` bounds, `entry.Thread == threadID` isolation, `beforeMsgID` filtering, oldest-first ordering, and safe skipping of legacy `At <= 0` entries in time-window queries.
- Added comprehensive miniredis unit tests in `TestChatLogThreadAndTimeQueries` and verified all `TestChatLog*` tests under `-race` and `golangci-lint` (`0 issues`).

## Task Commits

Each task was committed atomically (`plan_head_before..plan_head_after` counts 4 commits due to 2 interleaved commits from parallel Wave 1 peer `34-01` on `main`; `34-04` authored the 2 commits below):

1. **Task 1: Add At: msg.Date to chatLogEntry, preserve At on edits, and add thread-isolated query helpers** - `56b8f084` (feat)
2. **Task 2: Add miniredis unit tests for timestamps, edit preservation, legacy At==0, and strict topic isolation** - `1777bce3` (test)

## Files Created/Modified
- `xenobot/modules/chat_log.go` - Added `At` field, `chatLogEdit` timestamp preservation, and `chatLogEntryAt`, `chatLogRecent`, `chatLogSinceUser`, `chatLogSinceTime` helpers.
- `xenobot/modules/chat_log_test.go` - Updated `TestChatLogWatcher` for `at` field expectations and added `TestChatLogThreadAndTimeQueries`.

## Decisions Made
- Updated `TestChatLogWatcher` in `xenobot/modules/chat_log_test.go` (which previously asserted `len(fields) == 4` and `At == 0`) so Task 1's tracer verification remained green once `At: msg.Date` was recorded.
- Kept `chatLogSinceTime` scanning across all entries in the <=1000-item ring rather than breaking early so mixed legacy `At == 0` entries or non-monotonic timestamps never omit valid entries inside the cutoff window.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated pre-Phase-34 field-count assertion in TestChatLogWatcher**
- **Found during:** Task 1 (`Add At: msg.Date to chatLogEntry, preserve At on edits, and add thread-isolated query helpers`)
- **Issue:** `TestChatLogWatcher` asserted `At == 0` and `len(fields) == 4` (`m, u, n, t`) on logged messages, failing once `chatLogEntryOf` populated `At: msg.Date` (`5` fields including `at`).
- **Fix:** Updated `TestChatLogWatcher` expectations in `xenobot/modules/chat_log_test.go` to check `At: msg.Date`, `len(fields) == 5`, and `got[0].At == text.Date` after edits.
- **Files modified:** `xenobot/modules/chat_log_test.go`
- **Verification:** `go test -tags testtools -race -count=1 ./xenobot/modules -run 'TestChatLog'` passed.
- **Committed in:** `56b8f084` (part of Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 Rule 1 test expectation update)
**Impact on plan:** Necessary for `TestChatLogWatcher` correctness after introducing `chatLogEntry.At`; zero scope creep.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `CHAT-01` timestamping and thread-isolated query primitives (`chatLogEntryAt`, `chatLogRecent`, `chatLogSinceUser`, `chatLogSinceTime`) are ready for Wave 2 consumers (`34-05`, `34-06`, `34-07`).
- Both `go test -tags testtools -race -count=1 ./xenobot/modules -run 'TestChatLog'` and `golangci-lint run ./xenobot/modules/...` passed after final code edits.

## Self-Check: PASSED
- `xenobot/modules/chat_log.go`: FOUND
- `xenobot/modules/chat_log_test.go`: FOUND
- `56b8f084`: FOUND
- `1777bce3`: FOUND

---
*Phase: 34-room-aware-conversations-and-frictionless-catch-up*
*Completed: 2026-11-27*
