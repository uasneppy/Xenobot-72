---
phase: 32-contextual-ai-emoji-reactions
plan: 02
subsystem: ai-prompts
tags: [chatprompt, leaf-package, reactions, json-parser, unicode, vs16, prompt-injection]
status: complete

requires:
  - phase: 20-conversations
    provides: xenobot/utils/chatprompt leaf package (Clean, Name, line, Turn, MaxMessageRunes, MaxQuotedRunes, MaxTitleRunes, TestChatpromptPackageIsLeaf)
provides:
  - Compact stdlib-only reaction prompt builders (ReactionStatic, ReactionDynamic, ReactionUser, ReactionMaxTokens = 512)
  - Strict U+FE0F-normalizing JSON reaction parser (ParseReactionJSON, ReactionParseStatus, ReactionParseEmoji, ReactionParseNone, ReactionParseError)
  - Pure millisecond unit test suite covering prompt construction, sanitization, U+FE0F normalization, restricted palettes, lookalikes, and malformed JSON rejections
affects:
  - 32-03 (runtime contextual AI reaction worker calling ReactionStatic, ReactionDynamic, ReactionUser, ReactionMaxTokens, and ParseReactionJSON)

tech-stack:
  added: []
  patterns:
    - Stdlib-only leaf prompt builder in xenobot/utils/chatprompt verified by TestChatpromptPackageIsLeaf
    - Fail-closed ReactionParseStatus zero value (ReactionParseError at iota 0) with strict single-object JSON decoding and trailing-token rejection
    - Palette-aware lookalike warnings suggesting only valid chat-allowed alternate emojis (🤣, ❤, 👀, 🥰)

key-files:
  created:
    - xenobot/utils/chatprompt/reaction.go
    - xenobot/utils/chatprompt/reaction_test.go
  modified: []

key-decisions:
  - "Placed ReactionParseError at iota 0 so any uninitialized ReactionParseStatus fails closed."
  - "Filtered suggested lookalike replacements (🤣, ❤, 👀, 🥰) and the JSON example emoji in ReactionDynamic against the chat's normalized palette so restricted-palette chats never nudge models toward chat-disallowed emojis."
  - "Used json.NewDecoder with a second Decode(&trailing) == io.EOF assertion and strict outer-fence matching in ParseReactionJSON to reject trailing garbage, concatenated JSON objects, nested fences, and prose wrapping."

patterns-established:
  - "Reaction prompt wire contract: compact static persona + 6 red-line/abstention rules (< 3000 bytes, excluding the 39 KB comic archive), dynamic per-chat U+FE0F-stripped palette + lookalike warning + JSON contract, and sanitized user sections via Clean/Name/line."
  - "Strict reaction parsing: strip single outer markdown fence, require single JSON object with non-null non-empty string \"emoji\" key, handle case-insensitive NONE, strip U+FE0F on both parsed value and allowlist, and require exact membership."

requirements-completed:
  - REACT-02
  - REACT-03

duration: 3 min
completed: 2026-11-27

commits: 2
plan_head_before: 7901bc5727264dec03dcb71c2dfb09872655754c
plan_head_after: a97d6102caa4c676885a82acfa9a5bc78a49e6d9

actuals:
  tokens: 4931
  tasks: 2
  commits: 2
---

# Phase 32 Plan 02: Contextual AI Reaction Prompt Builder & Strict JSON Parser Summary

**Stdlib-only leaf reaction prompt builder (`ReactionMaxTokens = 512`, compact `< 3000`-byte `ReactionStatic`, palette-aware `ReactionDynamic`, sanitized `ReactionUser`) and strict `U+FE0F`-normalizing `ParseReactionJSON` parser in `xenobot/utils/chatprompt`**

## Performance

- **Duration:** 3 min (166 s)
- **Started:** 2026-11-26T23:46:18Z
- **Completed:** 2026-11-26T23:49:04Z
- **Tasks:** 2
- **Files modified:** 2 (2 created, 0 modified)

## Accomplishments

- Implemented `xenobot/utils/chatprompt/reaction.go` importing Go stdlib only (`bytes`, `encoding/json`, `io`, `slices`, `strings`), passing `TestChatpromptPackageIsLeaf` with zero external or `xenopersona` dependencies.
- Exported `ReactionMaxTokens = 512`, `ReactionParseStatus` (`ReactionParseError` zero value, `ReactionParseEmoji`, `ReactionParseNone`), `ReactionStatic()`, `ReactionDynamic(chatTitle, language, palette)`, `ReactionUser(recent, quoted, quoteText, message)`, and `ParseReactionJSON(raw, allowed)`.
- Kept `ReactionStatic()` compact (`< 3000` bytes, byte-identical across chats, excluding the 39 KB comic archive) with Xenobot-72's dry persona and the 6 empirically validated abstention/red-line/drama/spicy/prompt-injection rules.
- Sanitized all untrusted prompt inputs (`chatTitle`, `language`, `recent`, `quoted`, `quoteText`, `message`) through `Clean`/`Name`/`line` (stripping newlines, control, and bidi chars; marking `Xenobot-72` impersonators as `(member)`; carrying zero user IDs, chat IDs, or usernames) and added explicit warnings against non-Telegram lookalikes (`😂`, `🐱`, `😹`, `😻`, `😼`).
- Added comprehensive pure millisecond unit tests in `xenobot/utils/chatprompt/reaction_test.go` covering prompt invariants, sanitization/caps, all 7 `U+FE0F` standard emojis, markdown code fences, case-insensitive `NONE`, restricted 5-emoji palettes, lookalikes, and malformed/trailing-garbage JSON rejections.

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement xenobot/utils/chatprompt/reaction.go prompt builder and strict JSON parser** - `79d2b12f` (feat)
2. **Task 2: Add comprehensive pure millisecond unit tests in xenobot/utils/chatprompt/reaction_test.go** - `a97d6102` (test)

## Files Created/Modified

- `xenobot/utils/chatprompt/reaction.go` - Compact reaction prompt builders (`ReactionStatic`, `ReactionDynamic`, `ReactionUser`, `ReactionMaxTokens`) and strict `U+FE0F`-normalizing `ParseReactionJSON` parser.
- `xenobot/utils/chatprompt/reaction_test.go` - Pure millisecond unit tests (`TestReactionPrompts`, `TestParseReactionJSON`) verifying prompt sanitization, compact length, lookalike warnings, `U+FE0F` normalization across all 7 standard emojis, restricted palettes, and malformed payload rejections.

## Decisions Made

- Placed `ReactionParseError` at `iota == 0` so an uninitialized `ReactionParseStatus` fails closed by default.
- Filtered suggested lookalike replacements (`🤣`, `❤`, `👀`, `🥰`) and the wire-format example emoji in `ReactionDynamic` against the chat's normalized `palette` so restricted-palette chats never nudge the model to output a disallowed emoji.
- Used `json.NewDecoder` + trailing `Decode(&trailing) == io.EOF` and strict outer-fence validation in `ParseReactionJSON` so trailing prose, concatenated objects, nested code fences, and prose-wrapped fences are all rejected cleanly.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## Verification

- `go test -tags testtools -race -count=1 ./xenobot/utils/chatprompt -run 'TestChatpromptPackageIsLeaf'` — PASS (0.00s).
- `go test -tags testtools -race -count=1 -v ./xenobot/utils/chatprompt/...` — PASS (`TestReactionPrompts` 0.00s, `TestParseReactionJSON` 48 subtests 0.01s, `TestChatpromptPackageIsLeaf` 0.00s, plus all existing `chatprompt` tests).
- `GOTOOLCHAIN=go1.26.0 golangci-lint run ./xenobot/utils/chatprompt/...` — 0 issues.
- `CGO_ENABLED=0 go build ./xenobot/utils/chatprompt/...` — PASS.

## Next Phase Readiness

- Ready for Plan `32-03` (runtime contextual AI emoji reaction worker lane in `xenobot/modules/chat_react.go`) to wire `chatprompt.ReactionStatic()`, `chatprompt.ReactionDynamic(...)`, `chatprompt.ReactionUser(...)`, `chatprompt.ReactionMaxTokens`, and `chatprompt.ParseReactionJSON(...)`.
- No stubs, no new external dependencies, no new shutdown drains, and no threat flags introduced.

## Self-Check: PASSED
