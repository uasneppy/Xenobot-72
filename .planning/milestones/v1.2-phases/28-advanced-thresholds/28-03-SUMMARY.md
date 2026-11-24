---
phase: 28-advanced-thresholds
plan: 03
subsystem: moderation
tags: [raid-ai, memberreq, singleflight, watcher, thresholds, redis, claude]

requires:
  - phase: 28-advanced-thresholds
    provides: MSG-11 characterization golden suite TestLegacyPhase28DefaultThresholds and phase28InProcessMiniredis helper (28-01)
  - phase: 28-advanced-thresholds
    provides: chat_advanced_thresholds repository, skipLocal cache, and advThresholdsFor / adv*() helpers (28-02)
provides:
  - Per-chat raid AI image spam bar comparison on fresh and cached probabilities in raidAIProcessImage
  - Cross-chat singleflight-safe raw-probability memberReqAIAnswer with per-caller advMemberReqAIBar evaluation via memberReqAIVerdictOfBar
  - Watcher fast-path bar comparison (memberReqAIWatchSatisfiedBar / satisfiedBar) gated on rules.AI
  - Per-chat member requirement cap and pause duration wiring in memberReqCapAllow / memberReqPauseChatWith preserving package-var test seams when unset
  - End-to-end tests in TestAdvancedThresholdsAIAndMemberReq (5 subtests, in-process miniredis and RoundTripper)
affects: [28-05, 28-06]

tech-stack:
  added: []
  patterns:
    - "Raw-probability cross-chat singleflight return (memberReqAIAnswer{Probability, HasProbability, Fresh}) with per-caller chat bar evaluation (T-28-06)"
    - "Watcher fast-path threshold reads strictly gated on rules.AI (zero lookups when AI rule is off or ignored) (T-28-07)"
    - "Package-var seam preservation for memberReqCapPerMinute and memberReqPause when DB threshold is 0 (unset)"

key-files:
  created:
    - fuku/modules/advthresholds_ai_mreq_test.go
  modified:
    - fuku/modules/raid_ai_image.go
    - fuku/modules/memberreq_ai.go
    - fuku/modules/memberreq_watch.go
    - fuku/modules/memberreq_limits.go

key-decisions:
  - "Kept Verdict memberReqVerdict (zero value memberReqUnknown) alongside Probability/HasProbability/Fresh on memberReqAIAnswer so existing guard and Redis-down unit tests inspecting refusal answers compile and pass unchanged."
  - "Passed cache.Context (rather than the 250ms Redis pipeline timeout context) into advThresholdsFor inside memberReqLookupCachedWith so cold threshold reads retain their full 2s bounded timeout."
  - "Guarded memberReqAIRule with mode.Names == nil and !memberReqAIConfigured() before calling advThresholdsFor so inert/unconfigured paths incur zero threshold reads."

patterns-established:
  - "memberReqAIVerdictOfBar / memberReqAIDecideBar / memberReqAIPrepareBar / memberReqAIWatchSatisfiedBar: bar-parameterized helpers with default-bar wrappers keeping existing unit tests intact."

requirements-completed: [TUNE-07]

status: complete
plan_head_before: 99e7cf928f7d7beea0b61bb0b46de42184a6aefb
plan_head_after: 07239e3141e8887952a047f5b7c456c97cad20a3
commits: 3

actuals:
  tokens: 9153
  tasks: 3
  commits: 3

duration: 9min
completed: 2026-11-23
---

# Phase 28 Plan 03: AI Spam Bars & Member Requirement Brake Wiring Summary

**Per-chat raid AI image spam bar, cross-chat singleflight-isolated member requirement AI bar, `rules.AI`-gated watcher fast-path comparison, and per-chat member requirement cap/pause wiring preserving all package-variable test seams**

## Performance

- **Duration:** 9 min
- **Started:** 2026-11-23T09:11:52Z
- **Completed:** 2026-11-23T09:20:50Z
- **Tasks:** 3
- **Files modified:** 5 (1 created, 4 modified)

## Accomplishments

- Wired `advRaidAIImageBar(advThresholdsFor(ctx, job.chat.Id))` into `raidAIProcessImage` in `fuku/modules/raid_ai_image.go` so both fresh Anthropic probabilities and cached `fuku:raid:aiv:*` probabilities immediately apply each chat's custom image bar while preserving `const raidAIImageBar = 0.85`.
- Resolved the cross-chat singleflight verdict leak (T-28-06) in `fuku/modules/memberreq_ai.go`: `memberReqAIJudge` now returns raw `Probability` (`HasProbability: true`, `Fresh: true` on provider calls) without baking a chat-specific verdict, while `memberReqAIRule` evaluates each caller's own `advMemberReqAIBar(advThresholdsFor(ctx, chatID))` via `memberReqAIVerdictOfBar` / `memberReqAIPrepareBar` / `memberReqAIDecideBar`.
- Resolved the watcher fast-path trap (T-28-07) in `fuku/modules/memberreq_watch.go`: added `memberReqAIWatchSatisfiedBar` and `(memberReqAIWatchRead).satisfiedBar(bar)`, wired into `memberReqLookupCachedWith` with `advMemberReqAIBar` read strictly when `rules.AI` is true (0 lookups when AI rule is off or `ignoreAI` is set).
- Wired `advMemberReqCapPerMin` and `advMemberReqPause` into `memberReqCapAllow` and `memberReqPauseChatWith` in `fuku/modules/memberreq_limits.go`, applying the chat's effective cap and pause to Redis `SetNX` TTL, IDs-only warning logs, and `mreq_notice_paused` (`n` and `minutes`) while preserving package-variable test seams (`memberReqCapPerMinute`, `memberReqPause`) when unset (`0`).
- Added `TestAdvancedThresholdsAIAndMemberReq` in `fuku/modules/advthresholds_ai_mreq_test.go` with 5 subtests running under `-race` via `phase28InProcessMiniredis` and in-process `http.RoundTripper` transports (zero localhost TCP dials, zero `httptest.NewServer`).

## Task Commits

1. **Task 1: wire raidAIImageBar and fix memberReqAIBar across cross-chat singleflight and watcher fast-path** - `a679b1f1` (`feat(28-03): wire per-chat AI spam bars with cross-chat singleflight and watcher isolation`)
2. **Task 2: wire memberReqCapPerMinute and memberReqPause preserving package-var test seams** - `66a2ba55` (`feat(28-03): wire per-chat memberreq cap and pause with seam fallback`)
3. **Task 3: tests for raid AI bar, cross-chat singleflight isolation, watcher fast-path, and memberreq cap & pause** - `07239e31` (`test(28-03): add AI spam bar, cross-chat singleflight, watcher, and memberreq brake tests`)

## Files Created/Modified

- `fuku/modules/raid_ai_image.go` — Compare `result.probability >= advRaidAIImageBar(advThresholdsFor(ctx, job.chat.Id))` on fresh and cached image probabilities.
- `fuku/modules/memberreq_ai.go` — Added `memberReqAIVerdictOfBar`, `memberReqAIPrepareBar`, `memberReqAIDecideBar`, raw-probability `memberReqAIAnswer` (`Probability`, `HasProbability`, `Fresh`), and per-caller bar application in `memberReqAIRule`.
- `fuku/modules/memberreq_watch.go` — Added `memberReqAIWatchSatisfiedBar` and `satisfiedBar(bar)` and gated `advMemberReqAIBar` lookup on `rules.AI` inside `memberReqLookupCachedWith`.
- `fuku/modules/memberreq_limits.go` — Resolved `advMemberReqCapPerMin` and `advMemberReqPause` in `memberReqCapAllow` and added `memberReqPauseChatWith` using effective cap/pause for TTL, logs, and `mreq_notice_paused`.
- `fuku/modules/advthresholds_ai_mreq_test.go` — Created `TestAdvancedThresholdsAIAndMemberReq` covering all 5 required subtests under `-race`.

## Decisions Made

- Retained `Verdict memberReqVerdict` on `memberReqAIAnswer` (zero value `memberReqUnknown`) so existing guard and Redis-down tests (`memberreq_ai_guards_test.go`, `memberreq_ai_redisdown_test.go`) continue compiling and passing unchanged on refusal/error paths while probability answers are always evaluated per caller via `answer.HasProbability`.
- Passed `cache.Context` into `advThresholdsFor` inside `memberReqLookupCachedWith` so threshold lookups use their own 2 s bounded timeout rather than the 250 ms Redis pipeline context.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- All AI spam bars (`raid_ai_image_bar_pct`, `member_req_ai_bar_pct`) and member requirement brake thresholds (`member_req_cap_per_min`, `member_req_pause_mins`) are wired and verified under `-race`.
- Ready for Plans 28-04, 28-05, and 28-06.

## Self-Check: PASSED
