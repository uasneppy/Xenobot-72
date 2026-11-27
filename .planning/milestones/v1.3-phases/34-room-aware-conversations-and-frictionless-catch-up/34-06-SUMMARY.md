---
phase: 34-room-aware-conversations-and-frictionless-catch-up
plan: 06
subsystem: modules
tags: [chat-brief, presence-reactions, pm-brief, forum-topics, stopchat, redis]

# Dependency graph
requires:
  - phase: 34-01
    provides: presence.Settings.PMBriefReact persistence
  - phase: 34-03
    provides: 8-locale brief_empty, brief_header_*_pm, brief_note_pm, brief_pm_blocked, and brief_usage_* strings
  - phase: 34-04
    provides: chatLogEntryAt, chatLogRecent, chatLogSinceUser, chatLogSinceTime thread-isolated Redis queries
provides:
  - Reply-free /unrollshort and /unrolllong argument parser (chatBriefParseArgs) and count/me/duration/pm modes in chat_brief.go
  - Shared chatDeliverPrivate(..., silent) path with PM group-title headers and silentPM group-noise suppression
  - Silent ✍ PM brief trigger (presenceHandlePMBrief) in presence_reactions.go under StopChat with zero new shutdown drains
affects:
  - 34-08 (Phase 34 verification & docs check)

# Actuals (#2632)
actuals:
  tokens: 27864
  tasks: 3
  commits: 11
  plan_head_before: ceca608329318ad1b57633f59b3fa684f3e27a47
  plan_head_after: 41ea47ae6bab322bf56febecef16b2790bb35dae

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "chatBriefParseArgs bounds reply-free counts (5..200) and durations (1m..7d via extraction.ParseDurationToken) while keeping /judge strictly reply-only"
    - "silentPM audit suppresses all group-facing sends (typing, breaker, budget, empty/too_old, refusal, malfunction, pm_note, pm_blocked) across success and 403"
    - "presenceHandlePMBrief enforces Owner LogEnabled + provider, lazy lockdown fail-closed check, Audience, Redis SetNX cooldown, and chatLogEntryAt before queueing under StopChat"

key-files:
  created: []
  modified:
    - xenobot/modules/chat_brief.go
    - xenobot/modules/chat_brief_test.go
    - xenobot/modules/presence_reactions.go
    - xenobot/modules/presence_reactions_test.go

key-decisions:
  - "Kept chatJudgeDeliverPrivate as a thin wrapper calling chatDeliverPrivate(..., false) and invoked it for chatBriefJudge so /judge behavior stays unchanged and unused-symbol lint stays clean."
  - "Placed per-user ✍ Redis SetNX cooldown before chatLogEntryAt and chatBriefEnqueue in presenceHandlePMBrief so rapid reaction tapping on any message cannot spam Redis scans or brief:b budgets."

patterns-established:
  - "Reply-free /unroll* queries pass job.threadID into chatLogRecent/chatLogSinceUser/chatLogSinceTime and answer brief_empty when nothing matches in the invoking topic."
  - "✍ PM brief reactions share StopChat (chatJobs, chatDraining, chatContext()) with zero new shutdown drains."

requirements-completed:
  - CHAT-04
  - CHAT-05

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "Reply-free /unrollshort and /unrolllong support count (5..200), me, and duration (1m..7d via extraction.ParseDurationToken) with optional trailing pm, strict topic isolation, and brief_empty while keeping /judge reply-only."
    requirement: CHAT-04
    verification:
      - kind: unit
        ref: "xenobot/modules/chat_brief_test.go#TestChatBriefReplyFree"
        status: pass
      - kind: unit
        ref: "xenobot/modules/chat_brief_test.go#TestChatBriefShort"
        status: pass
      - kind: unit
        ref: "xenobot/modules/chat_brief_judge_test.go#TestChatJudge"
        status: pass
    human_judgment: false
  - id: D2
    description: "Reacting ✍ triggers presenceHandlePMBrief (Owner LogEnabled + provider, lazy lockdown, Audience, SetNX user cooldown, chatLogEntryAt) and delivers a silent PM brief under StopChat with zero group messages on success or 403."
    requirement: CHAT-05
    verification:
      - kind: unit
        ref: "xenobot/modules/presence_reactions_test.go#TestPresencePMBriefReaction"
        status: pass
      - kind: unit
        ref: "xenobot/modules/presence_reactions_test.go#TestShutdownEndsPresenceReaction"
        status: pass
      - kind: unit
        ref: "xenobot/modules/chat_brief_safety_test.go#TestShutdownEndsChatBriefs"
        status: pass
    human_judgment: false

# Metrics
duration: 6 min
completed: 2026-11-27
status: complete
---

# Phase 34 Plan 06: Reply-Free Briefs & Silent ✍ PM Brief Summary

**Implemented reply-free `/unrollshort` and `/unrolllong` catch-up modes (`<N|me|duration> [pm]`) in `chat_brief.go` and wired silent `✍` react-to-brief PM delivery from `presence_reactions.go` under `StopChat` with zero new shutdown drains.**

## Performance

- **Duration:** 6 min
- **Started:** 2026-11-27T07:37:41Z
- **Completed:** 2026-11-27T07:44:07Z
- **Tasks:** 3
- **Files modified:** 4

## Accomplishments
- Added `chatBriefMode` (`reply`, `count`, `me`, `time`), `chatBriefParseArgs` (clamping count to `5..200`, duration to `60s..7d` via `extraction.ParseDurationToken`, and optional trailing `pm`), thread-isolated stretch loading (`chatLogRecent`, `chatLogSinceUser`, `chatLogSinceTime`), PM group-title headers (`brief_header_short_pm`/`brief_header_long_pm`), and `chatDeliverPrivate(..., silent)` in `xenobot/modules/chat_brief.go`.
- Audited `chatBriefProcessJob` and `chatBriefFinish` so `job.silentPM == true` suppresses every group-facing send (`chatSendTyping`, `chatSayGated`, `chatBriefSay`, `brief_note_pm`, `brief_pm_blocked`) on both success and `403`, while keeping `/judge` strictly reply-only.
- Added `presenceHandlePMBrief` (`presencePMBriefCooldown = 60s`, `presencePMBriefCooldownKey`) in `xenobot/modules/presence_reactions.go`, triggered on normalized `"✍"` reactions in group `0` under `StopChat` and gated on `PMBriefReact`, `LogEnabled` + configured provider, `!presenceIsStaffGroup`, lazy `presenceIsLockedDown`, `chatAudienceAllows`, Redis `SetNX` user cooldown, and `chatLogEntryAt`.
- Added comprehensive `-race` tests `TestChatBriefReplyFree` and `TestPresencePMBriefReaction` covering argument parsing, topic isolation, `raidNow()` time-window queries, PM headers, `403` silence, double-gating, safety gates, cooldowns, and `StopChat` draining.

## Task Commits

Each task was committed atomically (`plan_head_before..plan_head_after` measures 11 commits on `main` due to 8 interleaved commits from parallel Wave 2 peers `34-05` and `34-07`; `34-06` authored the 3 commits below):

1. **Task 1: Add reply-free /unroll* argument parsing, mode dispatch, and silentPM delivery in chat_brief.go** - `334ec704` (feat)
2. **Task 2: Wire silent ✍ PM brief trigger in presence_reactions.go under StopChat** - `1b1e7b0d` (feat)
3. **Task 3: Add tests in chat_brief_test.go and presence_reactions_test.go for reply-free briefs and silent ✍ PM briefs** - `41ea47ae` (test)

## Files Created/Modified
- `xenobot/modules/chat_brief.go` - Reply-free `/unroll*` parser/dispatch, PM brief headers, `chatDeliverPrivate(..., silent)`, and `silentPM` suppression.
- `xenobot/modules/presence_reactions.go` - `presenceHandlePMBrief` and per-user `✍` Redis cooldown under `StopChat`.
- `xenobot/modules/chat_brief_test.go` - `TestChatBriefReplyFree` unit & e2e coverage.
- `xenobot/modules/presence_reactions_test.go` - `TestPresencePMBriefReaction` coverage including `403` silence, gates, cooldowns, and `StopChat` drain.

## Decisions Made
- Kept `chatJudgeDeliverPrivate` calling `chatDeliverPrivate(ctx, job, client, messages, false)` and routed `chatBriefJudge` through it in `chatBriefFinish` so `/judge` remains unchanged and `golangci-lint` reports zero unused-symbol warnings.

## Deviations from Plan
None - plan executed as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `CHAT-04` and `CHAT-05` implementation and tests are complete and verified under `-race` (`go test -tags testtools -race -count=1 ./xenobot/modules -run 'TestChatBrief|TestChatJudge|TestPresence|TestShutdownEndsPresenceReaction|TestShutdownEndsChatBriefs'`), `golangci-lint` (`0 issues`), and `CGO_ENABLED=0 go build ./...`.
- Ready for Phase 34 verification (`34-08`).

---
*Phase: 34-room-aware-conversations-and-frictionless-catch-up*
*Completed: 2026-11-27*

## Self-Check: PASSED
- `xenobot/modules/chat_brief.go`: FOUND
- `xenobot/modules/chat_brief_test.go`: FOUND
- `xenobot/modules/presence_reactions.go`: FOUND
- `xenobot/modules/presence_reactions_test.go`: FOUND
- `334ec704`, `1b1e7b0d`, `41ea47ae`: FOUND in git history
