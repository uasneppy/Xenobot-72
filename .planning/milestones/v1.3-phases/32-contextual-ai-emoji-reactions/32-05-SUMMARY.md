---
phase: 32-contextual-ai-emoji-reactions
plan: 05
subsystem: modules
tags: [chat-react, presence, reactions, redis-lua, vision, featherless, stop-chat]
status: complete

requires:
  - phase: 32-contextual-ai-emoji-reactions
    plan: 01
    provides: presence.AIReactModeOff/Low/Medium, presence.ValidAIReactMode, chat_presence.ai_react_mode
  - phase: 32-contextual-ai-emoji-reactions
    plan: 02
    provides: chatprompt.ReactionStatic, ReactionDynamic, ReactionUser, ReactionMaxTokens (512), ParseReactionJSON
  - phase: 32-contextual-ai-emoji-reactions
    plan: 04
    provides: presenceStaticReactedKey, presenceAIReactGapKey, presenceAIReactChatCooldownKey, presenceAllowedPalette, chatLogRecentThread
provides:
  - Dedicated bounded chatPresenceQueue worker lane (2 workers, cap 32) sharing StopChat (chatJobs, chatDraining, chatContext) with zero new shutdown drains
  - Mutually exclusive group 13 unaddressed handoff (chatReactWatch) preserving zero operational Redis I/O for default/off groups
  - Atomic same-slot Redis Lua throttle (gap, chat cooldown, shared user cooldown preserving 24h 👎 retraction TTL, math/rand/v2 roll) and dedicated "react:b" hourly budget
  - Full 3-provider contextual reaction worker (chatReactProcessJob) with lazy fail-closed lockdown, read-only breaker checks, same-thread chat_log + ReplyTarget/Quote, photo/sticker vision with 24h Featherless 400/422 text_only fallback, MaxTokens=512 JSON parsing, and non-tripping error shedding
  - Developer /stats AI reactions counters (chatReactStatsSection) and fast race-tested unit/integration suite
affects:
  - 32-06 (Settings page pr AI Reactions controls and AGENTS.md documentation)
  - 33 (Smarter Static Reactions setting ctx.Data[presenceStaticReactedKey] in group 8)

tech-stack:
  added: []
  patterns:
    - Dedicated 2-worker/32-cap chatPresenceQueue lane launched inside chatEnsureWorkers() under StopChat with zero main.go edits
    - Atomic same-slot {<chatID>} Redis Lua gate checking message gap, chat cooldown, shared user cooldown (preserving 24h retraction TTL), and pre-rolled math/rand/v2 probability win
    - Read-only chatBreaker(provider).isPaused inspection with strictly non-tripping silent error shedding at Debug level
    - 24h xenobot:cache:featherless_text_only:<model> degradation and single text-only retry on non-auth Featherless 400/422 vision rejections

key-files:
  created:
    - xenobot/modules/chat_react.go
    - xenobot/modules/chat_react_test.go
  modified:
    - xenobot/modules/chat_run.go
    - xenobot/modules/chat_watch.go

key-decisions:
  - "Checked presenceSettingsGet before chatSettingsLookup and presenceRedisClient in chatReactWatch so default/off groups perform zero operational Redis I/O and keep TestChatWatcherIgnores 100% intact."
  - "Guarded Redis Lua SET EX calls on cooldown > 0 and preserved KEYS[3] on EXISTS == 1 so an existing 24h 👎 retraction cooldown is honored and never shortened by AI reaction cooldowns."
  - "Kept chatBreakers strictly read-only in chatReactProcessJob (isPaused only; zero calls to chatRecordFailure, countFailure, pauseProvider, or clear) so background reaction faults never disrupt human Conversations."

patterns-established:
  - "chatPresenceQueue joins StopChat via chatJobs.start()/finish(), chatDraining, and chatContext() without adding shutdown drains or altering the 70 s worst-case budget."
  - "Background AI reactions use dedicated react:b hourly budgets and xenobot:cache:featherless_text_only:<model> 24h capability caching."

requirements-completed:
  - PRES-04
  - REACT-01
  - REACT-02
  - REACT-03

coverage:
  - deliverable: "Dedicated chatPresenceQueue worker lane under StopChat and group 13 mutually exclusive unaddressed handoff with Owner×Admin double-gating"
    human_judgment: false
    verification:
      - kind: test
        ref: "xenobot/modules/chat_react_test.go#TestChatReactDoubleGateAndSafety"
        status: pass
      - kind: test
        ref: "xenobot/modules/chat_react_test.go#TestShutdownEndsChatReact"
        status: pass
      - kind: test
        ref: "xenobot/modules/chat_watch_test.go#TestChatWatcherIgnores"
        status: pass
  - deliverable: "Cross-replica same-slot Redis Lua throttle (gap, chat/user cooldowns, 24h retraction TTL preservation, math/rand/v2 roll) and dedicated react:b hourly budget"
    human_judgment: false
    verification:
      - kind: test
        ref: "xenobot/modules/chat_react_test.go#TestChatReactRedisLuaGateAndBudget"
        status: pass
  - deliverable: "Same-thread chat_log context, ReplyTarget/Quote forwarding, photo/sticker vision via chat_media.go, Featherless 400/422 24h text_only fallback, and custom-only palette skip"
    human_judgment: false
    verification:
      - kind: test
        ref: "xenobot/modules/chat_react_test.go#TestChatReactContextVisionAndFeatherlessFallback"
        status: pass
  - deliverable: "3-provider Claude/Gemini/Featherless MaxTokens=512 JSON picker, is_big=false + 24h bot_react (ai), /stats counters, and non-tripping shedding"
    human_judgment: false
    verification:
      - kind: test
        ref: "xenobot/modules/chat_react_test.go#TestChatReactProvidersAndNonTrippingShedding"
        status: pass

duration: 7 min
completed: 2026-11-27

commits: 3
plan_head_before: a1c78963d23deca53561773d4a0b2f3a8d14fe03
plan_head_after: 0874c396007ab8a21a8cc251c0a634f63edc5ab4

actuals:
  tokens: 13009
  tasks: 3
  commits: 3
---

# Phase 32 Plan 05: Contextual AI Emoji Reactions Runtime Summary

**Dedicated 2-worker/32-cap `chatPresenceQueue` lane under `StopChat`, group 13 mutually exclusive `chatReactWatch` with Owner×Admin double-gating and same-slot Redis Lua throttling, 3-provider `MaxTokens = 512` vision/context reaction worker with 24h Featherless `text_only` fallback, and non-tripping error shedding**

## Performance

- **Duration:** 7 min (441 s)
- **Started:** 2026-11-27T00:00:59Z
- **Completed:** 2026-11-27T00:08:20Z
- **Tasks:** 3
- **Files modified:** 4 (2 created, 2 modified)

## Accomplishments

- Wired `chatPresenceEnsureWorkers()` (2 workers, cap 32) into `chatEnsureWorkers()` and appended `chatReactStatsSection()` into `chatStatsSection()` in `xenobot/modules/chat_run.go`, sharing `chatJobs`, `chatDraining`, and `chatContext()` under `StopChat` with zero new shutdown drains and zero `main.go` edits.
- Wired mutually exclusive unaddressed handoff (`chatReactWatch(b, ctx)`) inside `chatWatch` in `xenobot/modules/chat_watch.go`, keeping `chatWatcher` returning `ext.ContinueGroups` on every path so group 14 `chatLogWatcher` always runs.
- Implemented `xenobot/modules/chat_react.go` with cheapest-first pre-AI gating (`chatAnyProviderConfigured`, `ctx.Data[presenceStaticReactedKey]`, `presenceMessageAllowed`, scorable text/photo/sticker check, `presenceSettingsGet` default-off zero-Redis-I/O fast path, `PRES-04` Owner×Admin double gate), atomic same-slot `{<chatID>}` Redis Lua throttle (`chatReactGateScript` covering message gap, chat cooldown, shared per-user cooldown preserving 24h `👎` retraction TTL, and `math/rand/v2` probability roll), and dedicated `"react:b"` hourly budget (`6`/`20`/`200`).
- Implemented `chatReactProcessJob` enforcing lazy fail-closed `presenceIsLockedDown`, read-only `chatBreaker(provider).isPaused`, `presenceAllowedPalette` pre-check (skipping downloads and LLM calls for custom-only chats), same-thread `chatLogRecentThread` (up to 5 entries when `LogEnabled`) plus always-included `ReplyTarget` and `msg.Quote`, photo/sticker vision downloads via `chat_media.go`, 24h `xenobot:cache:featherless_text_only:<model>` caching + single text-only retry on non-auth Featherless `400`/`422`, `MaxTokens = 512` `ParseReactionJSON`, `presenceSetReaction(..., "ai")`, and strictly non-tripping silent error shedding.
- Added fast (`< 0.1 s` test runtime), race-safe test coverage in `xenobot/modules/chat_react_test.go` (`TestChatReactDoubleGateAndSafety`, `TestChatReactRedisLuaGateAndBudget`, `TestChatReactContextVisionAndFeatherlessFallback`, `TestChatReactProvidersAndNonTrippingShedding`, `TestShutdownEndsChatReact`).

## Task Commits

Each task was committed atomically:

1. **Task 1 (tracer): Wire chatPresenceQueue under StopChat, group 13 unaddressed handoff, and pre-AI double-gate + Redis Lua throttle** - `ceda298f` (feat)
2. **Task 2: Complete chatReactProcessJob with context, vision, Featherless 400/422 text_only fallback, and non-tripping shedding** - `378c8f0b` (feat)
3. **Task 3: Add fast, race-safe unit & integration test suite in xenobot/modules/chat_react_test.go** - `0874c396` (test)

## Files Created/Modified

- `xenobot/modules/chat_react.go` - Dedicated `chatPresenceQueue` worker lane, `/stats` counters, pre-AI double gate, atomic same-slot Redis Lua throttle, context/vision assembly, Featherless `text_only` fallback, and non-tripping shedding.
- `xenobot/modules/chat_run.go` - Launches `chatPresenceEnsureWorkers()` in `chatEnsureWorkers()` and appends `chatReactStatsSection()` in `chatStatsSection()`.
- `xenobot/modules/chat_watch.go` - Hands off unaddressed group 13 messages exclusively to `chatReactWatch(b, ctx)`.
- `xenobot/modules/chat_react_test.go` - Race-safe unit and integration test suite covering double-gating, safety gates, Lua throttle, `"react:b"` budget, context/vision/Featherless fallback, all 3 real provider adapters, non-tripping shedding, and `StopChat` shutdown.

## Decisions Made

- Ordered `presenceSettingsGet` before `chatSettingsLookup` and `presenceRedisClient` in `chatReactWatch` so default/off groups perform zero operational Redis I/O, preserving `TestChatWatcherIgnores` without modifying existing tests.
- Checked `EXISTS KEYS[3] == 1` before any `SET` in `chatReactGateScript` so an existing 24h `👎` retraction cooldown on `presenceUserCooldownKey` blocks AI reactions without overwriting or shortening its TTL.
- Kept `chatBreakers` strictly read-only in `chat_react.go` (only `isPaused`, never calling `chatRecordFailure`, `countFailure`, `pauseProvider`, or `clear`) so background reaction errors cannot poison or alter Conversations breaker state.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## Verification

- `go test -tags testtools -race -count=1 -v ./xenobot/modules -run 'TestChatReact|TestShutdownEndsChat|TestChatWatcher|TestChatProvidersRequest'` — PASS (`2.926s`; all `TestChatReact*`, `TestShutdownEndsChatReact`, `TestShutdownEndsChat`, `TestShutdownEndsChatBriefs`, `TestChatWatcherIgnores`, `TestChatWatcherAnswers`, `TestChatWatcherAudience`, `TestChatWatcherGroupReachedByMention`, `TestChatProvidersRequest`).
- `CGO_ENABLED=0 go build ./...` — PASS.
- `GOTOOLCHAIN=go1.26.0 golangci-lint run --build-tags=testtools ./xenobot/modules/...` — `0 issues`.

## Threat Model Coverage

- **T-32-09 (DoS — Burst/Spend):** Mitigated via pre-AI atomic Redis Lua gap/cooldown/roll gate, dedicated `"react:b"` hourly budget (`6`/`20`/`200`), and bounded 2-worker/32-cap `chatPresenceQueue` isolated from `chatQueue`.
- **T-32-10 (DoS — Breaker Poisoning):** Mitigated by read-only `chatBreaker(provider).isPaused` inspection and zero breaker mutations across parse, lookalike, provider, vision, and Telegram 400 errors (verified by `assertChatBreakersClean`).
- **T-32-11 (Information Disclosure):** Mitigated by `presenceMessageAllowed` (rejecting private chats, Staff Groups, bots, channels, slash commands), lazy fail-closed `presenceIsLockedDown`, zero user/chat IDs or usernames on wire across all 3 providers, and class-only Debug logging.
- **T-32-12 (Elevation of Privilege — Consent Boundary):** Mitigated by requiring both server `EnableChat` + Owner `chat_conversations.enabled` (with configured provider) AND Admin `chat_presence.ai_react_mode != "off"`.
- **T-32-SC (Supply Chain):** Zero new external Go dependencies added.

## Next Phase Readiness

- Ready for Plan `32-06` (Settings page `pr` AI Reactions segmented selector `[Off | Low | Medium]` with Owner×Admin double-gating UI and `AGENTS.md` Phase 32 updates).
- No stubs, no new shutdown drains, no new handler groups, and no threat flags introduced.

## Self-Check: PASSED
