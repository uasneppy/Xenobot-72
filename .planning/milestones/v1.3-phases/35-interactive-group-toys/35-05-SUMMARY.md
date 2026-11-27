---
phase: 35-interactive-group-toys
plan: 05
subsystem: modules
tags: [go, chat, presence, polls, referendum, tie-breaker, stopchat, redis]

# Dependency graph
requires:
  - phase: 35-interactive-group-toys
    provides: "chatprompt/toys.go prompt builders and strict JSON parsers (35-02), referendum_* locales and literal guards (35-03), presencePollKey/presencePollTieKey/toy cooldown keys and Settings rf toggle (35-04)"
provides:
  - "Shared StopChat-joined interactive toy worker lane (chatToyQueue, chatToysEnsureWorkers, chatToysEnqueue, chatToysStatsSection) and dedicated 'toy:b' hourly budget"
  - "/referendum command (referendumDesc, referendumCommand, chatToyGateOf, chatReferendumProcess) sending native Telegram polls via SendPollWithContext and persisting 25h metadata at xenobot:presence:poll:<poll_id>"
  - "Group-0 handlers.NewPoll(poll.All, chatModule.handlePollUpdate) Facility Tie-Breaker with SETNX once-only claim and deadpan fallback"
affects: [35-06-inspect, 35-07-docs-and-verification]

# Actuals (#2632)
actuals:
  tokens: 13756
  tasks: 3
  commits: 3
  plan_head_before: 9a64b42d2ee0cee552fdda46973fe4c78fe79807
  plan_head_after: c802b18ec4b422818736c806755c3711ef2d760d

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Shared StopChat toy worker lane (chatToyTask on chatToyQueue, 2 workers, cap 16) joining chatJobs/chatDraining/chatContext() with zero new shutdown drains"
    - "Shared Owner x Admin x Audience x Staff/Lockdown gate helper chatToyGateOf reusable by /inspect"
    - "Rune truncation via chatCutRunes after chatSanitizeText so @\\u200b mention neutralization is preserved"

key-files:
  created:
    - xenobot/modules/chat_polls.go
    - xenobot/modules/chat_polls_test.go
  modified:
    - xenobot/modules/chat_run.go
    - xenobot/modules/chat.go
    - xenobot/modules/presence_reactions.go

key-decisions:
  - "Checked chatAudienceAllows before Owner/Admin enabled toggles in chatToyGateOf so ineligible members in AudienceAdminsApproved groups receive silence rather than off-hints."
  - "Added chatCutRunes instead of re-running chatprompt.Clean after chatSanitizeText so @\\u200b zero-width-space mention neutralization is never stripped before SendPollWithContext."

patterns-established:
  - "chatToyGateOf and chatToysEnqueue provide the shared gate and worker lane for Wave 3 interactive group toys (/referendum and /inspect)."

requirements-completed: [TOYS-01, TOYS-02]

coverage:
  - id: D1
    description: "Shared StopChat toy worker lane, 'toy:b' hourly budget, and chatStatsSection toy counters wired into chat_run.go, chat.go, and presence_reactions.go"
    requirement: TOYS-01
    verification:
      - kind: unit
        ref: "xenobot/modules/chat_polls_test.go#TestShutdownEndsChatToys"
        status: pass
    human_judgment: false
  - id: D2
    description: "/referendum double-gating, Audience, Staff/Lockdown safety, same-thread topic isolation, sanitized SendPollWithContext wire options, and 25h Redis poll metadata"
    requirement: TOYS-01
    verification:
      - kind: unit
        ref: "xenobot/modules/chat_polls_test.go#TestReferendumDoubleGateAndSafety"
        status: pass
      - kind: unit
        ref: "xenobot/modules/chat_polls_test.go#TestReferendumPollFlowAndTopicIsolation"
        status: pass
    human_judgment: false
  - id: D3
    description: "Group-0 handlers.NewPoll Facility Tie-Breaker ignoring unknown/open/0-vote/non-tied polls, claiming ties once-only via SETNX, respecting Staff/Lockdown/deleted polls, and falling back when AI fails"
    requirement: TOYS-02
    verification:
      - kind: unit
        ref: "xenobot/modules/chat_polls_test.go#TestPollTieBreaker"
        status: pass
      - kind: unit
        ref: "xenobot/modules/chat_polls_test.go#TestShutdownEndsChatToys"
        status: pass
    human_judgment: false

# Metrics
duration: 8 min
completed: 2026-11-27T13:11:18Z
status: complete
---

# Phase 35 Plan 05: /referendum & Tied-Poll Tie-Breaker Summary

**Implemented `/referendum` AI-drafted native polls (`TOYS-01`), the shared `StopChat` toy worker lane with `"toy:b"` hourly budgets, and the group-`0` `handlers.NewPoll` once-only Facility Tie-Breaker (`TOYS-02`).**

## Performance

- **Duration:** 8 min
- **Started:** 2026-11-27T13:03:06Z
- **Completed:** 2026-11-27T13:11:18Z
- **Tasks:** 3
- **Files modified:** 5

## Accomplishments
- Wired `chatToysEnsureWorkers()` and `chatToysStatsSection()` into `xenobot/modules/chat_run.go`, registered `referendumDesc` with `helpers.WrapCommand` and `DefaultHelpRegistry().AltHelpOptions["Chat"]` in `xenobot/modules/chat.go`, and registered group-`0` `handlers.NewPoll(poll.All, chatModule.handlePollUpdate)` inside `LoadPresence` in `xenobot/modules/presence_reactions.go`.
- Implemented `xenobot/modules/chat_polls.go` with the `StopChat`-joined toy worker pool (`chatToyQueue`, `chatJobs`, `chatDraining`, `chatContext()`), `"toy:b"` hourly budget reservation via `chatReserveWith`, `chatToyGateOf` double-gating, same-thread `chatLogRecent` + `ReplyTarget` context extraction, `SendPollWithContext` (`OpenPeriod: 900`, `DisableNotification: true`, `AllowAddingOptions: false`), 25h `xenobot:presence:poll:<poll_id>` metadata storage outside `xenobot:cache:`, and `handlePollUpdate` with `SETNX` `xenobot:presence:poll:tie:<poll_id>` once-only claiming and localized deadpan fallback.
- Created `xenobot/modules/chat_polls_test.go` covering double-gating, `AudienceAdminsApproved`, Staff/Lockdown safety, missing-context hint, same-thread topic isolation, `"toy:b"` budget and per-chat cooldown, `SendPoll` wire parameters, 25h poll metadata TTL, concurrent `SETNX` tie-breaker claiming, ignored/deleted polls, AI failure fallback, and `StopChat` abort/shedding under `-race`.

## Task Commits

Each task was committed atomically:

1. **Task 1: Wire StopChat toy worker lane and stats into chat_run.go, chat.go, and presence_reactions.go** - `66c11d7a` (feat)
2. **Task 2: Implement /referendum, shared toy lane/budget, and handlers.NewPoll tie-breaker in chat_polls.go** - `988c43ca` (feat)
3. **Task 3: Add comprehensive tests in xenobot/modules/chat_polls_test.go** - `c802b18e` (test)

## Files Created/Modified
- `xenobot/modules/chat_run.go` - Joined `chatToysEnsureWorkers()` into `chatEnsureWorkers()` and appended `chatToysStatsSection()` to `chatStatsSection()`.
- `xenobot/modules/chat.go` - Added `referendumDesc` (`Disableable: true`, `RequireGroup`), registered `/referendum` via `helpers.WrapCommand`, and added `"referendum"` to `AltHelpOptions["Chat"]`.
- `xenobot/modules/presence_reactions.go` - Registered `handlers.NewPoll(poll.All, chatModule.handlePollUpdate)` in `LoadPresence` at group `0`.
- `xenobot/modules/chat_polls.go` - Implemented the shared `StopChat` toy worker lane, `"toy:b"` budget, `chatToyGateOf`, `/referendum` command/worker, `SendPollWithContext`, Redis poll metadata, and `handlePollUpdate` tie-breaker.
- `xenobot/modules/chat_polls_test.go` - Added `TestReferendumDoubleGateAndSafety`, `TestReferendumPollFlowAndTopicIsolation`, `TestPollTieBreaker`, and `TestShutdownEndsChatToys`.

## Decisions Made
- Checked `chatAudienceAllows` before Owner/Admin toggle checks in `chatToyGateOf` so unauthorized members in `AudienceAdminsApproved` groups stay silent even when toggles are off.
- Introduced `chatCutRunes` to re-bound sanitized poll questions/options after `chatSanitizeText` rather than re-running `chatprompt.Clean`, because `chatprompt.Clean` strips `unicode.Cf` format characters including `@\u200b` mention neutralization.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Preserved `@\u200b` mention neutralization in `chatSanitizeReferendumDraft`**
- **Found during:** Task 3 (`TestReferendumPollFlowAndTopicIsolation`)
- **Issue:** Calling `chatprompt.Clean(chatSanitizeText(...), max)` stripped the `\u200b` zero-width space inserted by `chatSanitizeText` because `chatprompt.Clean` removes `unicode.Cf` characters, leaving `@bob` un-neutralized in the poll question.
- **Fix:** Added `chatCutRunes` to enforce the `300`/`100` rune limits after `chatSanitizeText` without stripping `\u200b` (since `ParseReferendumJSON` already ran `chatprompt.Clean` on the raw model output).
- **Files modified:** `xenobot/modules/chat_polls.go`
- **Verification:** `TestReferendumPollFlowAndTopicIsolation` passed under `-race`, verifying `@\u200bbob` and `[link removed]` on the `sendPoll` wire payload.
- **Committed in:** `c802b18e` (Task 3 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Essential for `T-35-08` `@` mention neutralization in native Telegram polls.

## Issues Encountered
- Initial `golangci-lint` run flagged `gocyclo` > 20 on `chatReferendumProcess` and `chatHandlePollUpdate`, unused `chatToyUserCooldown` before `chat_inspect.go` lands, and 3 unchecked `env.mr.Set` calls in `chat_polls_test.go`. Split the two functions into focused helpers, enforced `chatToyUserCooldown` alongside `chatToyChatCooldown`, and checked all `env.mr.Set` errors; final `GOTOOLCHAIN=go1.26.0 golangci-lint run ./xenobot/modules/...` passed with 0 issues.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Shared `chatToyQueue`, `chatToyGateOf`, `chatToyUserCooldown`, and `"toy:b"` budget constants are ready for Plan `35-06` (`xenobot/modules/chat_inspect.go`).
- `STATE.md`, `ROADMAP.md`, and `REQUIREMENTS.md` were left untouched for the central orchestrator update after Wave 3 completes.

## Self-Check: PASSED
- Verified all 5 created/modified files exist on disk (`xenobot/modules/chat_run.go`, `xenobot/modules/chat.go`, `xenobot/modules/presence_reactions.go`, `xenobot/modules/chat_polls.go`, `xenobot/modules/chat_polls_test.go`).
- Verified commits `66c11d7a`, `988c43ca`, and `c802b18e` are ancestors of `HEAD`.
- Verified `go test -tags testtools -race -count=1 ./xenobot/modules -run 'TestReferendum|TestPollTieBreaker|TestShutdownEndsChatToys'`, `go test -tags testtools -race -count=1 ./xenobot/i18n/... -run 'TestChatToys'`, and `GOTOOLCHAIN=go1.26.0 golangci-lint run ./xenobot/modules/...` all passed after the final edit.

---
*Phase: 35-interactive-group-toys*
*Completed: 2026-11-27*
