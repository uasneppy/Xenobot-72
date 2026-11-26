---
phase: 31-presence-foundation-settings-page-and-zero-ai-social-reactions
plan: 02
subsystem: api
tags: [presence, safety-gate, redis, telegram-reactions, available-reactions, pacer, httptest]

# Dependency graph
requires:
  - phase: 31-presence-foundation-settings-page-and-zero-ai-social-reactions
    provides: xenobot/db/presence repository, models.ChatPresence, and chat_presence skipLocal registration (31-01)
provides:
  - Shared Interactive Presence safety gate (chat type, slash, sender_chat, bots, Staff Groups, lazy fail-closed lockdowns, anonymous group admin check)
  - Operational Redis key namespace xenobot:presence:{<chat>}:* strictly outside xenobot:cache:
  - Typed applyPresence* settings writers with bounds validation and read-back
  - Safe Telegram reaction API with U+FE0F normalization, 20m AvailableReactions cache, dedicated presenceReactionPacer, 24h bot_react metadata, and explicit []gotgbot.ReactionType{} wire clear
affects:
  - 31-04-PLAN.md
  - 31-05-PLAN.md
  - phase-32-contextual-ai-emoji-reactions
  - phase-33-smarter-static-reactions-highlights-and-backup-parity
  - phase-34-room-aware-conversations-and-catch-up
  - phase-35-interactive-group-toys

actuals:
  tokens: 12504
  tasks: 2
  commits: 4
  plan_head_before: 7490b359a03d49e10848c454d56755b952abbc5c
  plan_head_after: cca3502f0da8f0b9cad665e59c8e5aa2198fa341

tech-stack:
  added: []
  patterns:
    - "Shared PRES-02 safety gate separating hot-path cached Staff Group / sender checks from lazy fail-closed lockdown.GetActiveFresh calls"
    - "Operational xenobot:presence:{<chat>}:* Redis keys outside xenobot:cache: with Redis Cluster hash tags and hex-encoded emoji suffixes"
    - "Dedicated presenceReactionPacer isolated from chatPacer and chatBreakers with 20m AvailableReactions cache and REACTION_INVALID eviction"
    - "Explicit non-nil []gotgbot.ReactionType{} for clearing reactions verified over real HTTP via httptest"

key-files:
  created:
    - xenobot/modules/presence.go
    - xenobot/modules/presence_test.go
    - xenobot/modules/presence_react_api.go
    - xenobot/modules/presence_react_api_test.go
  modified: []

key-decisions:
  - "Decoded real gotgbot/v2 multipart/form-data wire payloads in httptest (plus a Reaction: nil negative control) to prove presenceClearReaction transmits reaction=[] while nil omits it"
  - "Delegated presenceRedisClient to chatRedisClient so existing withChatRedis test helpers work uniformly across Chat and Presence suites"
  - "Hex-encoded UTF-8 emojis in presenceConsensusUsersKey to avoid Redis key delimiter or whitespace ambiguities"

patterns-established:
  - "presenceMessageAllowed enforces cheap message/sender/Staff checks while intentionally leaving presenceIsLockedDown lazy right before firing"
  - "presenceSetReaction / presenceClearReaction log only chat_id and chatSendErrClass(err) at Debug and never call chatRecordFailure"

requirements-completed: [PRES-02, PRES-05]

coverage:
  - id: D1
    description: "Shared PRES-02 safety gate rejects private chats, channels, slash commands, bots, anonymous channels/admins, Staff Groups, and active lockdowns (fail-closed on DB errors), while recognizing anonymous group admins (User==nil && ActorChat.Id==Chat.Id) for retraction only"
    requirement: PRES-02
    verification:
      - kind: unit
        ref: "xenobot/modules/presence_test.go#TestPresenceSafetyGate"
        status: pass
    human_judgment: false
  - id: D2
    description: "Operational xenobot:presence:{<chat>}:* Redis keys live strictly outside xenobot:cache: with Cluster hash tags, and applyPresence* writers enforce 2..20 bounds and read-back semantics"
    requirement: PRES-02
    verification:
      - kind: unit
        ref: "xenobot/modules/presence_test.go#TestPresenceKeys"
        status: pass
      - kind: integration
        ref: "xenobot/modules/presence_test.go#TestPresenceApply"
        status: pass
    human_judgment: false
  - id: D3
    description: "presenceClearReaction serializes explicit non-nil []gotgbot.ReactionType{} as reaction=[] on the wire over real HTTP (with Reaction: nil negative control proving omission)"
    requirement: PRES-05
    verification:
      - kind: integration
        ref: "xenobot/modules/presence_react_api_test.go#TestPresenceClearReactionWireJSON"
        status: pass
    human_judgment: false
  - id: D4
    description: "presenceSetReaction normalizes U+FE0F, enforces supportedReactionEmoji intersected with 20m cached AvailableReactions (evicted on REACTION_INVALID), sends IsBig: false via presenceReactionPacer, writes 24h bot_react metadata outside xenobot:cache:, and never poisons chatBreakers"
    requirement: PRES-05
    verification:
      - kind: integration
        ref: "xenobot/modules/presence_react_api_test.go#TestPresenceReactAPI"
        status: pass
      - kind: integration
        ref: "xenobot/modules/presence_react_api_test.go#TestPresenceAvailReactionsCache"
        status: pass
      - kind: integration
        ref: "xenobot/modules/presence_react_api_test.go#TestPresenceBotReact"
        status: pass
      - kind: other
        ref: "CGO_ENABLED=0 go build ./xenobot/modules"
        status: pass
    human_judgment: false

duration: 8 min
completed: 2026-11-26
status: complete
---

# Phase 31 Plan 02: Shared Presence Safety Gate and Safe Reaction API Summary

**Delivered the shared Interactive Presence safety gate (Staff Groups, lazy fail-closed lockdowns, actor/chat filters), operational `xenobot:presence:{<chat>}:*` Redis namespace, typed `applyPresence*` layer, and wire-verified Telegram reaction API with 20m `AvailableReactions` caching and breaker isolation.**

## Performance

- **Duration:** 8 min
- **Started:** 2026-11-26T21:50:21Z
- **Completed:** 2026-11-26T21:57:56Z
- **Tasks:** 2
- **Files modified:** 4 (4 created)

## Accomplishments
- Implemented the shared `PRES-02` safety gate (`presenceChatTypeAllowed`, `presenceIsStaffGroup`, lazy fail-closed `presenceIsLockedDown`, `presenceMessageAllowed`, and `presenceIsAnonGroupAdmin`), operational `xenobot:presence:{<chat>}:*` key builders strictly outside `xenobot:cache:`, and typed `applyPresence*` settings writers in `xenobot/modules/presence.go`.
- Implemented the `PRES-05` safe reaction API in `xenobot/modules/presence_react_api.go` with `U+FE0F` normalization reusing `normaliseReactionEmoji`/`supportedReactionEmoji`, 20-minute `AvailableReactions` caching under `xenobot:cache:avail_react:<chat>`, dedicated `presenceReactionPacer` isolated from `chatPacer`, 24-hour `bot_react` metadata (`{e, a, s}`) with atomic `DEL` claiming and 24-hour user cooldown extension, silent 400 degradation (evicting `avail_react` on `REACTION_INVALID` without ever touching `chatBreakers`), and explicit `[]gotgbot.ReactionType{}` clearing.
- Verified real `gotgbot/v2` HTTP wire serialization over `httptest.NewServer` (confirming `presenceClearReaction` transmits `reaction=[]` while `Reaction: nil` omits `"reaction"`, and `presenceSetReaction` strips `U+FE0F` with `IsBig: false`).

## Task Commits

Each task was committed atomically via TDD (RED → GREEN):

1. **Task 1: Implement shared safety gate, operational Redis key helpers, and presence apply layer (tracer, TDD)**
   - RED: `2529e321` (test) — compiling stubs + failing assertions across `TestPresenceSafetyGate`, `TestPresenceKeys`, and `TestPresenceApply`
   - GREEN: `f0a24f4e` (feat) — full safety gate, operational key builders, and `applyPresence*` writers; tracer gate verified end-to-end
2. **Task 2: Implement safe reaction API, AvailableReactions 20m cache, bot_react tracker, and wire-JSON tests (TDD)**
   - RED: `6f07087a` (test) — compiling stubs + failing `httptest` wire, cache, 400 isolation, and `bot_react` tests
   - GREEN: `cca3502f` (feat) — full safe reaction API implementation and multipart wire decoder in `presence_react_api_test.go`

## Files Created/Modified
- `xenobot/modules/presence.go` - Shared PRES-02 safety gate, `xenobot:presence:{<chat>}:*` key builders, `presenceRedisClient` seam, and typed `applyPresence*` settings writers.
- `xenobot/modules/presence_test.go` - `-race`-safe unit and SQLite tests for chat/sender/Staff/Lockdown/anon-admin guards, Redis key isolation/hash tags, and `applyPresence*` outcomes.
- `xenobot/modules/presence_react_api.go` - Dedicated `presenceReactionPacer`, `presenceStandardEmojis`, 20m `AvailableReactions` cache, `bot_react` read/write/claim + 24h cooldown helpers, `presenceSetReaction`, and `presenceClearReaction`.
- `xenobot/modules/presence_react_api_test.go` - Real `httptest.NewServer` wire tests (including `reaction=[]` clear vs `Reaction: nil` omission control), `AvailableReactions` cache matrix, `REACTION_INVALID`/`MESSAGE_ID_INVALID` breaker isolation, and concurrent `presenceClaimBotReact` race test.

## Decisions Made
- Inspected real `gotgbot/v2` `BaseBotClient` wire encoding during `httptest` verification (`multipart/form-data` with JSON-encoded `reaction` field `[]`) and decoded it directly in `decodeTelegramWireParams`, adding a `Reaction: nil` negative control to prove `[]gotgbot.ReactionType{}` is required to keep `"reaction"` on the wire.
- Delegated `presenceRedisClient` to `chatRedisClient` so existing `withChatRedis(t)` test fixtures work seamlessly without duplicating Redis test seams.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- Initial Task 2 test helper passed `srv.Client()` (`*http.Client`) instead of `*srv.Client()` (`http.Client`) into `gotgbot.BaseBotClient`, and assumed raw JSON POST body before `httptest` revealed `gotgbot/v2` encodes `setMessageReaction` as `multipart/form-data` with JSON-serialized `reaction` field; updated `newWireTestBot` and added `decodeTelegramWireParams` supporting both `multipart/form-data` and JSON.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Plan `31-04` (reaction router, `👎` Retraction, Emoji Consensus) and Plan `31-05` (Settings page `pr`) can build directly on `presence.go` (`presenceMessageAllowed`, `presenceIsLockedDown`, `presenceIsAnonGroupAdmin`, `presence*Key`, `applyPresence*`) and `presence_react_api.go` (`presenceStandardEmojis`, `presenceSetReaction`, `presenceClearReaction`, `presenceReadBotReact`, `presenceClaimBotReact`, `presenceExtendUserCooldown`).
- Zero blockers.

---
*Phase: 31-presence-foundation-settings-page-and-zero-ai-social-reactions*
*Completed: 2026-11-26*

## Self-Check: PASSED
