---
phase: 34-room-aware-conversations-and-frictionless-catch-up
plan: 05
subsystem: modules
tags: [conversations, ambient-context, vocative, deduplication, double-gating, redis]

requires:
  - phase: 34-room-aware-conversations-and-frictionless-catch-up
    provides: "presence.Settings AmbientContext/VocativeEnabled (34-01), chatprompt.UserWithAmbient & ParseVocativePrefix (34-02), and thread-isolated chatLogRecentThread (34-04)"
provides:
  - "chatReadHistoryWithIDs preserving chatHistoryEntry.MessageID set for ambient deduplication"
  - "Double-gated, topic-isolated, deduplicated chatAmbientContextTurns wired into chatProcessJob -> chatBuildRequest -> chatprompt.UserWithAmbient"
  - "Opt-in vocative name-addressing branch in group 13 (chatWatch) mutually exclusive with chatReactWatch"
  - "End-to-end tests TestChatAmbientContext and TestChatVocativeAddressing in chat_watch_test.go"
affects:
  - 34-07 (Settings UI exposes AmbientContext and VocativeEnabled toggles read by chat_run.go and chat_watch.go)

actuals:
  tokens: 5783
  tasks: 3
  commits: 6
  plan_head_before: 096879048883b605cc8309371103798ec6cb3bc4
  plan_head_after: 3872f3146155ca42203c3cee65e44efb054b74a9

tech-stack:
  added: []
  patterns:
    - "Owner×Admin double-gating on ambient context: !presenceIsStaffGroup && conv.Enabled && conv.LogEnabled && pres.AmbientContext"
    - "Strict ambient deduplication against chatReadHistoryWithIDs, job.quotedMessageID, and job.messageID"
    - "Group 13 vocative addressing mutual exclusion: early return after vocative handling even when chatAccept rejects"

key-files:
  created: []
  modified:
    - xenobot/modules/chat_history.go
    - xenobot/modules/chat_prompt.go
    - xenobot/modules/chat_run.go
    - xenobot/modules/chat_watch.go
    - xenobot/modules/chat_watch_test.go

key-decisions:
  - "Set job.quotedMessageID in chatJobOf whenever extraction.ReplyTarget(msg) != nil (including replies to the bot) so quoted turns never duplicate in ambient room context"
  - "Returned early in chatWatch whenever pres.VocativeEnabled is true and ParseVocativePrefix matches (even if chatAccept rejects on Audience or conv.Enabled) so vocative messages never fall through to chatReactWatch"

patterns-established:
  - "chatAmbientContextTurns degrades silently to nil on any lookup or Redis error so ambient context never breaks Conversations replies"

requirements-completed:
  - CHAT-02
  - CHAT-03

coverage:
  - id: D1
    description: "chatReadHistoryWithIDs, chatAmbientContextTurns (Owner×Admin double-gated, non-Staff Group, topic-isolated, 8-turn cap, deduplicated), and chatBuildRequest -> chatprompt.UserWithAmbient wiring"
    requirement: CHAT-02
    verification:
      - kind: unit
        ref: "xenobot/modules/chat_watch_test.go#TestChatAmbientContext"
        status: pass
      - kind: unit
        ref: "xenobot/modules/chat_watch_test.go#TestChatWatcherAnswers"
        status: pass
    human_judgment: false
  - id: D2
    description: "Opt-in vocative name-addressing in group 13 (chat_watch.go) with ForwardOrigin/StaffGroup/Audience guards and mutual exclusion against chatReactWatch"
    requirement: CHAT-03
    verification:
      - kind: unit
        ref: "xenobot/modules/chat_watch_test.go#TestChatVocativeAddressing"
        status: pass
      - kind: unit
        ref: "xenobot/modules/chat_watch_test.go#TestChatWatcherIgnores"
        status: pass
    human_judgment: false

duration: 4 min
completed: 2026-11-27
status: complete
---

# Phase 34 Plan 05: Ambient Room Context & Vocative Name-Addressing Summary

**Wired Owner×Admin double-gated, topic-isolated, deduplicated ambient room context (`CHAT-02`) and opt-in vocative name-addressing mutually exclusive with AI reactions (`CHAT-03`) into Conversations**

## Performance

- **Duration:** 4 min
- **Started:** 2026-11-27T07:37:39Z
- **Completed:** 2026-11-27T07:41:25Z
- **Tasks:** 3
- **Files modified:** 5

## Accomplishments
- Added `chatReadHistoryWithIDs` in `xenobot/modules/chat_history.go` (preserving `chatReadHistory` for existing callers), extended `chatBuildRequest` in `xenobot/modules/chat_prompt.go` to pass `ambient` into `chatprompt.UserWithAmbient`, and implemented `chatAmbientContextTurns` (`chatAmbientContextLimit = 8`) in `xenobot/modules/chat_run.go` with strict `!presenceIsStaffGroup`, `conv.Enabled && conv.LogEnabled`, `pres.AmbientContext` double-gating, same-thread `chatLogRecentThread` reads, and deduplication against `historyIDs`, `job.quotedMessageID`, and `job.messageID`.
- Extended `chatJobOf` in `xenobot/modules/chat_watch.go` to always capture `job.quotedMessageID` from `extraction.ReplyTarget`, and added the opt-in vocative addressing branch in `chatWatch` (`msg.ForwardOrigin == nil`, `chatprompt.ParseVocativePrefix`, `!presenceIsStaffGroup`, `pres.VocativeEnabled`, `chatAccept`) returning early so vocatively addressed messages never double-fire `chatReactWatch`.
- Added `TestChatAmbientContext` and `TestChatVocativeAddressing` in `xenobot/modules/chat_watch_test.go` and verified all matching `TestChat*` suites under `-race`, `golangci-lint`, and `CGO_ENABLED=0 go build ./...`.

## Task Commits

Each task was committed atomically (`plan_head_before..plan_head_after` counts 6 commits due to 3 interleaved commits from parallel Wave 2 peers `34-06` and `34-07` on `main`; `34-05` authored the 3 commits below):

1. **Task 1: Add chatReadHistoryWithIDs, ambient room context lookup/dedup in chat_run.go, and chatBuildRequest wiring** - `ad5ba5ed` (feat)
2. **Task 2: Add opt-in vocative name-addressing branch in chat_watch.go and quotedMessageID tracking** - `c1c86a29` (feat)
3. **Task 3: Add tests in chat_watch_test.go for Ambient Room Context and Vocative Name-Addressing** - `3872f314` (test)

## Files Created/Modified
- `xenobot/modules/chat_history.go` - Added `chatReadHistoryWithIDs` returning oldest-first turns plus decoded `MessageID` set; `chatReadHistory` delegates to it.
- `xenobot/modules/chat_prompt.go` - Extended `chatBuildRequest` with `ambient []chatprompt.Turn` wired into `chatprompt.UserWithAmbient`.
- `xenobot/modules/chat_run.go` - Added `chatAmbientContextLimit = 8`, `chatJob.quotedMessageID`, `chatAmbientContextTurns`, and `chatProcessJob` wiring.
- `xenobot/modules/chat_watch.go` - Tracked `job.quotedMessageID` in `chatJobOf` and added opt-in vocative branch in `chatWatch` mutually exclusive with `chatReactWatch`.
- `xenobot/modules/chat_watch_test.go` - Added `TestChatAmbientContext` and `TestChatVocativeAddressing`.

## Decisions Made
- Captured `job.quotedMessageID = reply.MessageId` in `chatJobOf` before filtering `reply.From.Id != b.Id`, ensuring replies to either members or the bot itself never duplicate the quoted target in ambient room context.
- Returned early from `chatWatch` whenever `pres.VocativeEnabled` is true and `ParseVocativePrefix` matched (even if `chatAccept` returned non-open), keeping vocative addressing strictly mutually exclusive with `chatReactWatch`.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `CHAT-02` and `CHAT-03` runtime wiring and tests are complete; ready for Phase 34 verification once sibling Wave 2 plans (`34-06`, `34-07`) conclude.

## Self-Check: PASSED
- Files verified: `xenobot/modules/chat_history.go`, `xenobot/modules/chat_prompt.go`, `xenobot/modules/chat_run.go`, `xenobot/modules/chat_watch.go`, `xenobot/modules/chat_watch_test.go`
- Commits verified: `ad5ba5ed`, `c1c86a29`, `3872f314`
- Verification run: `go test -tags testtools -race -count=1 -v ./xenobot/modules -run 'TestChatAmbientContext|TestChatVocativeAddressing|TestChatWatcher|TestChatReact'` (PASS, 2.987s), `GOTOOLCHAIN=go1.26.0 golangci-lint run --build-tags testtools ./xenobot/modules/...` (0 issues), `CGO_ENABLED=0 go build ./...` (exit 0).

---
*Phase: 34-room-aware-conversations-and-frictionless-catch-up*
*Completed: 2026-11-27*
