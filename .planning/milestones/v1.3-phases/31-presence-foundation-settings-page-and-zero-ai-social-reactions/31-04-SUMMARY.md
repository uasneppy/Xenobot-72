---
phase: 31-presence-foundation-settings-page-and-zero-ai-social-reactions
plan: 04
subsystem: api
tags: [presence, reactions, thumbs-down-retraction, emoji-consensus, redis-lua, stopchat]

# Dependency graph
requires:
  - phase: 31-presence-foundation-settings-page-and-zero-ai-social-reactions
    provides: Shared PRES-02 safety gate, operational xenobot:presence:{<chat>}:* keys, and safe reaction API (31-02)
provides:
  - LoadPresence module registration at handler group 0 without SetModuleEnabled
  - StopChat-tracked MessageReactionUpdated router joining chatJobs, chatDraining, and chatContext() with zero new shutdown drains
  - REACT-05 thumbs-down retraction with bot_react pre-check, author/admin/anonymous-group-admin authorization, atomic DEL claim, silent reaction=[] clear, and 24h user cooldown extension
  - REACT-04 zero-AI Emoji Consensus with atomic Cluster-safe Redis Lua distinct-member sets, hostile denylist, lazy fail-closed lockdown check, atomic dual cooldown+once-only latch claim, and best-effort chat_log author lookup
affects:
  - 31-05-PLAN.md
  - phase-32-contextual-ai-emoji-reactions
  - phase-33-smarter-static-reactions-highlights-and-backup-parity

actuals:
  tokens: 10551
  tasks: 2
  commits: 2
  plan_head_before: 1258b06d483d87ba4177f73857326b5439d1d9d6
  plan_head_after: d42b05556ebce724c9a28db3192043a03a879040

tech-stack:
  added: []
  patterns:
    - "Group-0 handlers.NewReaction(reaction.All, ...) registered via RegisterLegacyModule without SetModuleEnabled so /help stays unchanged"
    - "StopChat lifecycle integration via chatJobs.start()/finish(), chatDraining.Load(), and 3s timeout derived from chatContext() with zero changes to main.go"
    - "Atomic Cluster-safe Lua SADD/SREM/SCARD sets and dual EXISTS+SET cooldown/latch claim under {<chat>} hash tags"
    - "Best-effort chat_log score lookup (presenceLogAuthorOf) feeding AuthorID into bot_react so authors can retract Consensus reactions with zero getChatMember calls"

key-files:
  created:
    - xenobot/modules/presence_reactions.go
    - xenobot/modules/presence_reactions_test.go
  modified: []

key-decisions:
  - "Processed removed standard emojis before added emojis in presenceHandleConsensus so user emoji swaps update SREM even if an added emoji immediately crosses threshold"
  - "Checked presenceIsLockedDown lazily at threshold crossing BEFORE presenceConsensusClaim so active lockdowns or DB outages fail closed without burning cons:cd or cons:done latches"
  - "Guarded presenceExtendUserCooldown on ctx.Err() == nil after presenceClearReaction so a job cancelled by StopChat never writes post-shutdown cooldown keys"

patterns-established:
  - "MessageReactionUpdated handlers check bot_react in Redis before any getChatMember/setMessageReaction call, preventing quota exhaustion storms"
  - "Blocking httptest shutdown tests release handler channels via defer before httptest.Server.Close() runs in LIFO t.Cleanup"

requirements-completed: [PRES-01, PRES-02, REACT-04, REACT-05]

coverage:
  - id: D1
    description: "LoadPresence registers handlers.NewReaction at group 0 without adding a /help entry and joins StopChat (chatJobs + chatDraining + chatContext()) with zero new shutdown drains"
    requirement: PRES-01
    verification:
      - kind: integration
        ref: "xenobot/modules/presence_reactions_test.go#TestShutdownEndsPresenceReaction"
        status: pass
      - kind: unit
        ref: "xenobot/modules/help_test.go#TestHelpModuleNamesExist"
        status: pass
    human_judgment: false
  - id: D2
    description: "Reaction router enforces Staff Group, chat type, bot-reactor, anonymous-actor, and lazy fail-closed active lockdown safety gates"
    requirement: PRES-02
    verification:
      - kind: integration
        ref: "xenobot/modules/presence_reactions_test.go#TestPresenceRetract"
        status: pass
      - kind: integration
        ref: "xenobot/modules/presence_reactions_test.go#TestPresenceConsensus"
        status: pass
    human_judgment: false
  - id: D3
    description: "Zero-AI Emoji Consensus counts distinct non-anonymous real members per non-hostile standard emoji via atomic Redis Lua sets, defeats toggle-farming and Premium stacking, claims chat cooldown and per-message latch atomically, and resolves best-effort chat_log author"
    requirement: REACT-04
    verification:
      - kind: unit
        ref: "xenobot/modules/presence_reactions_test.go#TestPresenceDiffReactions"
        status: pass
      - kind: integration
        ref: "xenobot/modules/presence_reactions_test.go#TestPresenceConsensus"
        status: pass
    human_judgment: false
  - id: D4
    description: "Thumbs-down retraction checks bot_react before any Telegram call, authorizes message author, group admin, or anonymous group admin (ActorChat.Id == Chat.Id), clears with wire reaction=[], extends user cooldown, and posts zero group messages"
    requirement: REACT-05
    verification:
      - kind: integration
        ref: "xenobot/modules/presence_reactions_test.go#TestPresenceRetract"
        status: pass
    human_judgment: false

duration: 16 min
completed: 2026-11-26
status: complete
---

# Phase 31 Plan 04: Presence Reaction Router, Thumbs-Down Retraction, and Emoji Consensus Summary

**Implemented the group-0 `MessageReactionUpdated` router integrated into `StopChat` with zero new shutdown drains, delivering `👎` Retraction (`REACT-05`) and Redis Lua Emoji Consensus (`REACT-04`) with full Staff/Lockdown/ActorChat safety gates and `-race`-verified integration tests.**

## Performance

- **Duration:** 16 min
- **Started:** 2026-11-26T22:54:39Z
- **Completed:** 2026-11-26T23:10:21Z
- **Tasks:** 2
- **Files modified:** 2 (2 created)

## Accomplishments
- Created `xenobot/modules/presence_reactions.go` registering `LoadPresence` (`RegisterLegacyModule("Presence", 252, LoadPresence)` without `SetModuleEnabled`) at handler group `0` via `handlers.NewReaction(reaction.All, presenceModule.handleReaction)` and joining `StopChat` (`chatJobs.start()`/`finish()`, `chatDraining.Load()`, and 3 s timeout derived from `chatContext()`) with zero changes to `main.go`.
- Implemented `presenceDiffReactions` (stripping `U+FE0F` and filtering custom/paid/unsupported reactions), `presenceHandleRetract` (Redis `bot_react` pre-check before any Telegram call, author/anonymous-group-admin/live-admin authorization, atomic `DEL` claim, silent `[]gotgbot.ReactionType{}` clear, 24 h cooldown extension, zero group messages), and `presenceHandleConsensus` (hostile-emoji denylist, atomic Lua `SADD`/`SREM`/`SCARD` distinct-member sets, lazy fail-closed `presenceIsLockedDown` at threshold crossing, atomic dual `cons:cd` + `cons:done:<msgID>` Lua claim under `{<chat>}` hash tags, best-effort `chat_log` author lookup, and `presenceSetReaction(..., "cons")`).
- Created `xenobot/modules/presence_reactions_test.go` covering `TestPresenceDiffReactions`, `TestPresenceRetract` (9 subtests), `TestPresenceConsensus` (10 subtests), and `TestShutdownEndsPresenceReaction`, all passing under `-race` alongside `TestHelpModuleNamesExist` and `golangci-lint` with zero warnings.

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement LoadPresence, StopChat reaction router, 👎 Retraction, and Redis Lua Emoji Consensus** - `ae56d185` (feat)
2. **Task 2: Add fast, race-safe integration and StopChat shutdown tests for 👎 Retraction and Emoji Consensus** - `d42b0555` (test)

## Files Created/Modified
- `xenobot/modules/presence_reactions.go` - `LoadPresence` group-0 registration, `StopChat`-tracked reaction router, `presenceDiffReactions`, `presenceLogAuthorOf`, `👎` Retraction (`REACT-05`), and Redis Lua Emoji Consensus (`REACT-04`).
- `xenobot/modules/presence_reactions_test.go` - Race-safe unit, miniredis + `httptest` wire integration, and `StopChat` shutdown tests (`TestPresenceDiffReactions`, `TestPresenceRetract`, `TestPresenceConsensus`, `TestShutdownEndsPresenceReaction`).

## Decisions Made
- Processed `removed` emojis before `added` emojis in `presenceHandleConsensus` so a member switching emojis updates `SREM` before any threshold-triggered early return.
- Kept `presenceIsLockedDown(chatID)` strictly lazy at threshold crossing and before `presenceConsensusClaim` so locked-down chats or DB outages never burn per-chat cooldowns or per-message latches.
- Guarded `presenceExtendUserCooldown` on `ctx.Err() == nil` so a retraction cancelled mid-flight by `StopChat` does not leave post-shutdown cooldown keys.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- Initial `TestShutdownEndsPresenceReaction` registered `t.Cleanup(func() { close(testDone) })` before `newWireTestBot(t, ...)` (which registers `t.Cleanup(srv.Close)`), causing LIFO cleanup to block in `httptest.Server.Close()` waiting for the active request handler after all assertions had passed. Switched to `defer close(testDone)` so the blocking server handler releases before `srv.Close()` runs; the full test suite then completed in ~2.5 s under `-race`.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Plan `31-05` (Settings page `pr`, Conversations page `p` shortcut, and `AGENTS.md` updates) can build directly on `LoadPresence`, `presence_reactions.go`, and `presence.go`.
- Zero blockers or stubs.

---
*Phase: 31-presence-foundation-settings-page-and-zero-ai-social-reactions*
*Completed: 2026-11-26*

## Self-Check: PASSED
