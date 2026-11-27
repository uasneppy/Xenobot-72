---
phase: 32-contextual-ai-emoji-reactions
plan: 04
subsystem: modules
tags: [presence, reactions, redis, chat-log, palette, thread-isolation]
status: complete

requires:
  - phase: 31-presence-foundation-settings-page-and-zero-ai-social-reactions
    provides: presence.go operational Redis keys, presence_react_api.go avail_react 20m cache, presenceWrites/applyPresence* helpers
  - phase: 32-contextual-ai-emoji-reactions
    plan: 01
    provides: presence.AIReactModeOff/Low/Medium, presence.ValidAIReactMode, presence.SetAIReactMode
provides:
  - presenceStaticReactedKey ctx.Data handoff constant (group 8 -> group 13)
  - Same-slot operational Redis keys presenceAIReactGapKey and presenceAIReactChatCooldownKey under xenobot:presence:{<chat>}:*
  - presenceWrites.AIReactMode and applyPresenceAIReactMode with pre-DB validation and read-back
  - presenceAllowedPalette reusing the 20m avail_react cache with cloned slices and fail-closed custom-only/error handling
  - chatLogRecentThread bounded 50-entry reverse scan with strict thread and beforeMsgID isolation
affects:
  - 32-05 (Contextual AI Emoji Reactions runtime worker and Redis Lua gate)
  - 32-06 (Settings page pr AI Reactions segmented selector using applyPresenceAIReactMode)

tech-stack:
  added: []
  patterns:
    - Same-slot Redis Cluster hash tag {<chat>} across ai:gap, ai:cd:chat, and cd:u:<user> outside xenobot:cache:
    - Cloned palette slices returned from 20m cached AvailableReactions with fail-closed (nil, false) on custom-only chats and getChat errors
    - Bounded ZRevRange scan (50 entries) under chatLogReadTimeout with strict entry.Thread == threadID && entry.MessageID < beforeMsgID filtering

key-files:
  created: []
  modified:
    - xenobot/modules/presence.go
    - xenobot/modules/presence_test.go
    - xenobot/modules/presence_react_api.go
    - xenobot/modules/presence_react_api_test.go
    - xenobot/modules/chat_log.go
    - xenobot/modules/chat_log_test.go

key-decisions:
  - "Placed presenceAIReactGapKey (:ai:gap) and presenceAIReactChatCooldownKey (:ai:cd:chat) under presenceChatTag(chatID) outside xenobot:cache: so CLEAR_CACHE_ON_STARTUP never wipes operational state and one Lua script can atomically coordinate them with presenceUserCooldownKey (:cd:u:<user>)."
  - "Cloned slices returned by presenceAllowedPalette and returned (nil, false) for custom-only chats or getChat failures so callers skip AI calls without mutating cached slices or touching chatBreakers."
  - "Bounded chatLogRecentThread to ZRevRange(0, 49) under chatLogReadTimeout and filtered strictly on entry.Thread == threadID && entry.MessageID < beforeMsgID before reversing to oldest-first, preventing forum topic bleed and triggering-message duplication."

patterns-established:
  - "presenceAllowedPalette is the single cached AvailableReactions intersection entrypoint; presenceAllowedEmoji delegates to it."
  - "chatLogRecentThread is the bounded, topic-isolated context reader for background reaction workers."

requirements-completed:
  - PRES-04
  - REACT-01
  - REACT-02
  - REACT-03

duration: 5 min
completed: 2026-11-26

commits: 3
plan_head_before: e6b03564277ebbce5be3010b72e74df3a7eb5938
plan_head_after: 41a7ecc61cde1da427cff739aeeaaa86647ec989

actuals:
  tokens: 5619
  tasks: 3
  commits: 3
---

# Phase 32 Plan 04: Shared Module Substrate for Contextual AI Reactions Summary

**Same-slot AI reaction Redis keys and `applyPresenceAIReactMode` in `presence.go`, cloned fail-closed `presenceAllowedPalette` in `presence_react_api.go`, and bounded forum-topic-isolated `chatLogRecentThread` in `chat_log.go`**

## Performance

- **Duration:** 5 min (319 s)
- **Started:** 2026-11-26T23:54:02Z
- **Completed:** 2026-11-26T23:59:21Z
- **Tasks:** 3
- **Files modified:** 6

## Accomplishments

- Extended `xenobot/modules/presence.go` with `presenceStaticReactedKey` (`"presence.static_reacted"`), same-slot operational Redis key helpers `presenceAIReactGapKey` (`xenobot:presence:{<chat>}:ai:gap`) and `presenceAIReactChatCooldownKey` (`xenobot:presence:{<chat>}:ai:cd:chat`), `presenceWrites.AIReactMode`, and `applyPresenceAIReactMode` (pre-DB `presence.ValidAIReactMode` validation, chat-ID-only warning on failure, fresh read-back on success).
- Extracted `presenceAllowedPalette(ctx, b, chatID)` in `xenobot/modules/presence_react_api.go` reusing the 20-minute `avail_react:<chat>` cache, cloning returned slices, returning all 73 `supportedReactionEmoji` when `avail.All`, returning `(nil, false)` for custom-only chats and `getChat` errors without touching `chatBreakers`, and delegating `presenceAllowedEmoji` to it with zero Phase 31 regression.
- Added `chatLogRecentScanLimit = 50` and `chatLogRecentThread(ctx, client, chatID, threadID, beforeMsgID, limit)` in `xenobot/modules/chat_log.go` performing a bounded `ZRevRange(0, 49)` under `chatLogReadTimeout`, skipping malformed JSON, enforcing strict `entry.Thread == threadID && entry.MessageID < beforeMsgID` isolation, capping at `limit`, and reversing to oldest-first.
- Added race-tested unit suites covering `presence_test.go`, `TestPresenceAllowedPalette`, and `TestChatLogRecentThread`.

## Task Commits

Each task was committed atomically:

1. **Task 1: Extend presence.go with AI reaction operational Redis keys, presenceStaticReactedKey, and applyPresenceAIReactMode** - `480200b5` (feat)
2. **Task 2: Extract presenceAllowedPalette in presence_react_api.go and add unit tests** - `cab8b341` (feat)
3. **Task 3: Add bounded thread-isolated chatLogRecentThread in chat_log.go and unit tests** - `41a7ecc6` (feat)

## Files Created/Modified

- `xenobot/modules/presence.go` - Added `presenceStaticReactedKey`, `presenceAIReactGapKey`, `presenceAIReactChatCooldownKey`, `presenceWrites.AIReactMode`, and `applyPresenceAIReactMode`.
- `xenobot/modules/presence_test.go` - Extended `TestPresenceKeys` and `TestPresenceApply` to cover the new AI reaction Redis keys, `presenceStaticReactedKey`, and `applyPresenceAIReactMode` (valid, invalid, save failure).
- `xenobot/modules/presence_react_api.go` - Extracted `presenceAllowedPalette` with slice cloning and fail-closed semantics; delegated `presenceAllowedEmoji` to it.
- `xenobot/modules/presence_react_api_test.go` - Added `TestPresenceAllowedPalette` (nil bot, all-73 + 20m cache hit + slice mutation isolation, restricted standard+custom normalization, custom-only skip, HTTP 500 fail-closed with clean `chatBreakers`).
- `xenobot/modules/chat_log.go` - Added `chatLogRecentScanLimit = 50` and `chatLogRecentThread` bounded reverse scan with strict thread and `beforeMsgID` filtering.
- `xenobot/modules/chat_log_test.go` - Added `TestChatLogRecentThread` (nil/non-positive guards, forum topic isolation across threads 0/10/20, `MessageID < beforeMsgID` filter, oldest-first cap, malformed JSON skip, 50-entry scan bound, Redis error propagation).

## Decisions Made

- Kept all new AI reaction operational keys under `presenceChatTag(chatID)` (`xenobot:presence:{<chat>}:*`) outside `xenobot:cache:` so they survive `CLEAR_CACHE_ON_STARTUP` and share the Redis Cluster hash slot with `presenceUserCooldownKey`.
- Returned cloned slices from `presenceAllowedPalette` so callers cannot mutate `supportedReactionEmoji` or cached `AvailableReactions` entries, and returned `(nil, false)` on custom-only chats so Plan `32-05` skips media download and LLM calls completely.
- Bounded `chatLogRecentThread` to 50 entries via `ZRevRange` instead of scanning the full 1000-entry ring, mitigating T-32-07 (cross-topic bleed) and T-32-08 (bounded scan DoS).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added unit test coverage in `xenobot/modules/presence_test.go` for Task 1 symbols**
- **Found during:** Task 1 verification (`golangci-lint run --build-tags=testtools ./xenobot/modules/...`)
- **Issue:** Because `presenceStaticReactedKey`, `presenceAIReactGapKey`, `presenceAIReactChatCooldownKey`, and `applyPresenceAIReactMode` are unexported substrate helpers consumed by upcoming Plans `32-05` and `32-06`, `golangci-lint`'s `unused` linter flagged all 4 as unused.
- **Fix:** Extended `TestPresenceKeys` and `TestPresenceApply` in `xenobot/modules/presence_test.go` to test the handoff constant, both operational Redis keys, and `applyPresenceAIReactMode` (valid write/read-back, invalid mode zero-write guard, and save failure).
- **Files modified:** `xenobot/modules/presence_test.go`
- **Verification:** `go test -tags testtools -race -count=1 ./xenobot/modules -run 'TestPresenceSafetyGate|TestPresenceKeys|TestPresenceApply'` PASS; `golangci-lint` reported 0 issues.
- **Commit:** `480200b5`

## Issues Encountered

None.

## Verification

- `CGO_ENABLED=0 go build ./...` — PASS.
- `go test -tags testtools -race -count=1 ./xenobot/modules -run 'TestPresenceAllowedPalette|TestPresenceSetReaction|TestPresenceClearReaction|TestChatLog|TestPresenceSafetyGate|TestPresenceKeys|TestPresenceApply|TestPresenceReactAPI|TestPresenceAvailReactionsCache|TestPresenceBotReact'` — PASS (4.720s).
- `GOTOOLCHAIN=go1.26.0 golangci-lint run --build-tags=testtools ./xenobot/modules/...` — 0 issues.

## Threat Model Coverage

- **T-32-07 (Information Disclosure):** Mitigated in `chatLogRecentThread` by strictly enforcing `entry.Thread == threadID && entry.MessageID < beforeMsgID` on every scanned entry, verified across threads `0`, `10`, and `20` in `TestChatLogRecentThread`.
- **T-32-08 (Denial of Service):** Mitigated by capping reverse scans at `chatLogRecentScanLimit = 50` under `chatLogReadTimeout`, reusing the 20m `avail_react` cache, cloning slices in `presenceAllowedPalette`, and failing closed `(nil, false)` on errors/custom-only chats without touching `chatBreakers`.
- **T-32-SC (Supply Chain):** Zero new external Go dependencies added.

## Next Phase Readiness

- Unblocks Plan `32-05` (Contextual AI Reactions runtime worker, Lua gate, vision/Featherless fallback) and Plan `32-06` (Settings page `pr` AI Reactions controls).
- No stubs, no new shutdown drains, no new handler groups, and no threat flags introduced.

## Self-Check: PASSED
