---
phase: 35-interactive-group-toys
plan: 06
subsystem: modules
tags: [go, chat, presence, inspect, vision, featherless, bot-react, stopchat]

# Dependency graph
requires:
  - phase: 35-interactive-group-toys
    provides: "InspectStatic/Dynamic/User and ParseInspectJSON (35-02), inspect_* locales (35-03), presenceToyUserCooldownKey and Settings in toggle (35-04), shared StopChat toy lane chatToysEnqueue, chatToyGateOf, and toy:b budget (35-05)"
provides:
  - "/inspect command registration (inspectDesc, DefaultHelpRegistry().AltHelpOptions['Chat'] with referendum and inspect)"
  - "/inspect worker execution with reply-target extraction, photo/sticker/image-doc vision, Featherless 400/422 text_only fallback, AllowSendingWithoutReply=false verdict reply, and presenceSetReaction(..., 'inspect') bot_react tracking"
  - "Comprehensive -race test suite in xenobot/modules/chat_inspect_test.go"
affects: [35-07-docs-and-verification]

# Actuals (#2632)
actuals:
  tokens: 10857
  tasks: 3
  commits: 3
  plan_head_before: a187439a365cb845463f28a8f16178395be08919
  plan_head_after: db3de065f510e8283f5de12706e67e8e2b622091

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Adapter methods asChat() and asReactJob() on chatInspectJob reuse chat_react.go vision (chatReactDownloadVision, chatReactCallProvider) and chat_send.go gated replies without modifying chat_react.go"
    - "Text verdict replies to targetMsgID with AllowSendingWithoutReply: false and stamps validated U+FE0F-stripped emoji via presenceSetReaction(..., targetAuthorID, 'inspect') only after reply succeeds"

key-files:
  created:
    - xenobot/modules/chat_inspect.go
    - xenobot/modules/chat_inspect_test.go
  modified:
    - xenobot/modules/chat.go

key-decisions:
  - "Checked extraction.ReplyTarget(msg) before chatToyGateOf in inspectCommand so missing-reply /inspect calls return inspect_usage with zero Redis or AI calls"
  - "Used chatInspectJob.asReactJob() adapter to reuse chatReactDownloadVision and chatReactCallProvider directly from chat_react.go without duplicating Featherless text_only fallback logic"
  - "Excluded channel SenderChat posts from targetAuthorID (0) while recording reply.From.Id for user messages so 👎 retraction remains author/admin-authorized"

patterns-established:
  - "/inspect composes chatToyGateOf, chatToysEnqueue, chatReactDownloadVision, chatReactCallProvider, and presenceSetReaction(..., 'inspect') under StopChat"

requirements-completed: [TOYS-03]

coverage:
  - id: D1
    description: "Register inspectDesc with helpers.WrapCommand on chatModule and include both referendum and inspect in DefaultHelpRegistry().AltHelpOptions['Chat']"
    requirement: TOYS-03
    verification:
      - kind: unit
        ref: "xenobot/modules/chat_inspect_test.go#TestInspectGatesAndUsage"
        status: pass
    human_judgment: false
  - id: D2
    description: "/inspect command gate, reply-target extraction, vision + Featherless text_only fallback, AllowSendingWithoutReply=false HTML verdict reply, and presenceSetReaction('inspect') bot_react stamping with 👎 retraction"
    requirement: TOYS-03
    verification:
      - kind: unit
        ref: "xenobot/modules/chat_inspect_test.go#TestInspectGatesAndUsage"
        status: pass
      - kind: unit
        ref: "xenobot/modules/chat_inspect_test.go#TestInspectVisionAndRetraction"
        status: pass
    human_judgment: false
  - id: D3
    description: "/inspect joins StopChat via chatToysEnqueue/chatContext() so SIGTERM aborts in-flight calls without tripping chatBreakers and sheds late commands"
    requirement: TOYS-03
    verification:
      - kind: unit
        ref: "xenobot/modules/chat_inspect_test.go#TestShutdownEndsInspect"
        status: pass
    human_judgment: false

# Metrics
duration: 5 min
completed: 2026-11-27T13:17:07Z
status: complete
---

# Phase 35 Plan 06: /inspect Command, Vision & Retraction Summary

**Implemented `/inspect` (`TOYS-03`) on the shared `StopChat` toy lane with reply-target extraction, photo/sticker/image-doc vision + Featherless `text_only` fallback, `AllowSendingWithoutReply: false` HTML Facility Inspection verdicts, and `presenceSetReaction(..., "inspect")` `bot_react` stamping supporting `👎` retraction.**

## Performance

- **Duration:** 5 min
- **Started:** 2026-11-27T13:11:57Z
- **Completed:** 2026-11-27T13:17:07Z
- **Tasks:** 3
- **Files modified:** 3

## Accomplishments
- Registered `inspectDesc` (`Disableable: true`, `RequireGroup`) via `helpers.WrapCommand` on `chatModule` and updated `DefaultHelpRegistry().AltHelpOptions["Chat"]` to list both `"referendum"` and `"inspect"` in `xenobot/modules/chat.go`.
- Implemented `xenobot/modules/chat_inspect.go` enforcing group chat, real non-bot user, `!presenceIsStaffGroup`, `!presenceIsLockedDown`, mandatory `extraction.ReplyTarget` (`inspect_usage`), Owner×Admin/`Audience` gating (`chatToyGateOf`), per-user cooldown (`presenceToyUserCooldownKey`), dedicated `"toy:b"` hourly budget (`chatReserveWith`), photo/sticker/image-doc vision (`chatReactDownloadVision` + `chatReactCallProvider` with Featherless `400/422` `text_only` fallback), `InspectMaxTokens` (`512`) `ParseInspectJSON` validation, sanitized HTML reply (`AllowSendingWithoutReply: false`, `MessageThreadId` preserved), and `presenceSetReaction(..., targetAuthorID, "inspect")` `bot_react` stamping.
- Created `xenobot/modules/chat_inspect_test.go` covering all gates, `"toy:b"` budget isolation, photo/sticker vision, Featherless `text_only` retry/cache, `U+FE0F` normalization, lookalike/disallowed emoji handling, deleted target silence, end-to-end `👎` retraction, and `StopChat` cancellation under `-race`; `GOTOOLCHAIN=go1.26.0 golangci-lint run ./xenobot/modules/...` passed with 0 issues.

## Task Commits

Each task was committed atomically:

1. **Task 1: Register inspectDesc and update AltHelpOptions["Chat"] in xenobot/modules/chat.go** - `379dfc57` (feat)
2. **Task 2: Implement /inspect command, vision fallback, verdict reply, and bot_react stamping in chat_inspect.go** - `028fcb38` (feat)
3. **Task 3: Add comprehensive tests in xenobot/modules/chat_inspect_test.go** - `db3de065` (test)

## Files Created/Modified
- `xenobot/modules/chat.go` - Added `inspectDesc`, registered `/inspect` in `LoadChat`, and added `"inspect"` alongside `"referendum"` in `DefaultHelpRegistry().AltHelpOptions["Chat"]`.
- `xenobot/modules/chat_inspect.go` - Implemented `inspectCommand`, `chatInspectJobFromReply`, `chatInspectProcess`, `chatInspectReserve`, `chatInspectReplyRefusal`, and `chatInspectSendVerdict`.
- `xenobot/modules/chat_inspect_test.go` - Added `TestInspectGatesAndUsage`, `TestInspectVisionAndRetraction`, and `TestShutdownEndsInspect`.

## Decisions Made
- Evaluated `extraction.ReplyTarget(msg) == nil` before `chatToyGateOf` so replying without a target returns `inspect_usage` with zero Redis/DB/AI I/O.
- Added `chatInspectJob.asReactJob()` to reuse `chatReactDownloadVision` and `chatReactCallProvider` directly from `xenobot/modules/chat_react.go` without duplicating Featherless `text_only` fallback logic.
- Recorded `targetAuthorID = 0` when `reply.SenderChat != nil` so channel posts do not grant `bot_react` ownership to `Channel_Bot` while user messages record `reply.From.Id` for `👎` retraction.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- Initial test run failed on `TestInspectVisionAndRetraction` because test groups default to the `uk` locale (`<b>🔍 Огляд об'єкта</b>`) while the assertion checked the English literal `"Facility Inspection"`; updated the assertion to compare against `chatCanned(t, "inspect_header")`.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- `/inspect` (`TOYS-03`) is complete and verified alongside `/referendum` (`TOYS-01`) and Facility Tie-Breaker (`TOYS-02`), ready for Plan `35-07` (`docs` & final verification).
- `STATE.md`, `ROADMAP.md`, and `REQUIREMENTS.md` were left untouched for the central orchestrator update.

## Self-Check: PASSED
- Verified `xenobot/modules/chat.go`, `xenobot/modules/chat_inspect.go`, and `xenobot/modules/chat_inspect_test.go` exist on disk.
- Verified commits `379dfc57`, `028fcb38`, and `db3de065` are ancestors of `HEAD`.
- Verified `go test -tags testtools -race -count=1 ./xenobot/modules -run 'TestInspect|TestShutdownEndsInspect'` (`ok 2.745s`), `TestInspect|TestShutdownEndsInspect|TestReferendum|TestPollTieBreaker|TestShutdownEndsChatToys` (`ok 3.158s`), and `GOTOOLCHAIN=go1.26.0 golangci-lint run ./xenobot/modules/...` (`0 issues`) all passed after the final edit.

---
*Phase: 35-interactive-group-toys*
*Completed: 2026-11-27*
