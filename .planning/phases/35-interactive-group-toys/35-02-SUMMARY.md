---
phase: 35-interactive-group-toys
plan: 02
subsystem: chatprompt
tags: [go, chatprompt, llm-prompts, json-parser, referendum, inspect, tie-breaker]

# Dependency graph
requires: []
provides:
  - "Leaf prompt builders ReferendumStatic/Dynamic/User, InspectStatic/Dynamic/User, and TieBreakerStatic/Dynamic/User in xenobot/utils/chatprompt/toys.go"
  - "Strict bounded parsers ParseReferendumJSON, ParseInspectJSON, and ParseTieBreakerText with U+FE0F normalization and lookalike emoji rejection"
  - "Token floor constants ReferendumMaxTokens=768, InspectMaxTokens=512, TieBreakerMaxTokens=512"
affects: [35-05, 35-06]

# Actuals
actuals:
  tokens: 7881
  tasks: 2
  commits: 2
  plan_head_before: be156f27507049c66ec984b32eb96cb38147c465
  plan_head_after: 17ad4bed

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Stdlib-only leaf package prompts/parsers with byte-identical static prompts and Clean-sanitized dynamic/user framing"
    - "Single-object JSON decoding with io.EOF trailing-token rejection and U+FE0F-normalized palette intersection"

key-files:
  created:
    - xenobot/utils/chatprompt/toys.go
    - xenobot/utils/chatprompt/toys_test.go
  modified: []

key-decisions:
  - "ParseInspectJSON clears invalid or lookalike emojis (Emoji == \"\", EmojiValid == false) while returning ok == true for a valid verdict so /inspect can still deliver text verdicts in chats with custom-only reactions"
  - "cleanInspectVerdict preserves up to 3 non-empty Clean-sanitized lines bounded by InspectVerdictMaxRunes (600)"

patterns-established:
  - "Reused stripReactionCodeFence, stripReactionVS16, normalizeReactionPalette, and containsNormalizedEmoji within package chatprompt"

requirements-completed: [TOYS-01, TOYS-02, TOYS-03]

coverage:
  - id: D1
    description: "Leaf prompt builders and strict JSON parsers for /referendum, tied-poll tie-breakers, and /inspect in xenobot/utils/chatprompt/toys.go"
    requirement: "TOYS-01"
    verification:
      - kind: unit
        ref: "xenobot/utils/chatprompt/toys_test.go#TestToyTokenFloors"
        status: pass
      - kind: unit
        ref: "xenobot/utils/chatprompt/toys_test.go#TestReferendumPrompts"
        status: pass
      - kind: unit
        ref: "xenobot/utils/chatprompt/toys_test.go#TestParseReferendumJSON"
        status: pass
      - kind: unit
        ref: "xenobot/utils/chatprompt/leaf_test.go#TestChatpromptPackageIsLeaf"
        status: pass
    human_judgment: false
  - id: D2
    description: "Tie-breaker leaf prompt builders and bounded ruling sanitizer"
    requirement: "TOYS-02"
    verification:
      - kind: unit
        ref: "xenobot/utils/chatprompt/toys_test.go#TestTieBreakerPrompts"
        status: pass
    human_judgment: false
  - id: D3
    description: "/inspect leaf prompt builders and strict JSON parser with U+FE0F strip and lookalike rejection"
    requirement: "TOYS-03"
    verification:
      - kind: unit
        ref: "xenobot/utils/chatprompt/toys_test.go#TestInspectPromptsAndParser"
        status: pass
    human_judgment: false

# Metrics
duration: 2 min
completed: 2026-11-27T12:54:51Z
status: complete
---

# Phase 35 Plan 02: Leaf Group Toy Prompts & Parsers Summary

**Stdlib-only leaf prompt builders and strict Telegram-bounded JSON parsers for `/referendum` (`ReferendumMaxTokens=768`), `/inspect` (`InspectMaxTokens=512` with `U+FE0F` strip and lookalike rejection), and tied-poll Facility Tie-Breakers**

## Performance

- **Duration:** 2 min
- **Started:** 2026-11-27T12:52:37Z
- **Completed:** 2026-11-27T12:54:51Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Implemented `xenobot/utils/chatprompt/toys.go` importing only Go standard library packages so `TestChatpromptPackageIsLeaf` stays green.
- Added `/referendum` prompt builders (`ReferendumStatic`, `ReferendumDynamic`, `ReferendumUser`) and `ParseReferendumJSON` enforcing single-object JSON without trailing tokens, `<= 300`-rune question, `2..5` case-insensitively deduplicated `<= 100`-rune options, and `ReferendumMaxTokens = 768`.
- Added `/inspect` prompt builders (`InspectStatic`, `InspectDynamic`, `InspectUser`) and `ParseInspectJSON` preserving up to 3 Clean-sanitized lines (`<= 600` runes), stripping `U+FE0F`, rejecting lookalike emojis (`😂`, `🐱`, `😹`, `😻`, `😼`), and intersecting strictly with the normalized chat-allowed palette (`InspectMaxTokens = 512`).
- Added Facility Tie-Breaker prompt builders (`TieBreakerStatic`, `TieBreakerDynamic`, `TieBreakerUser`) and `ParseTieBreakerText` (`TieBreakerMaxTokens = 512`, `TieBreakerMaxRunes = 400`).
- Added comprehensive `-race` unit tests in `xenobot/utils/chatprompt/toys_test.go` covering token floors, prompt sanitization, injection framing, multibyte Cyrillic truncation, deduplication, and parser fail-closed edge cases; `golangci-lint` reported 0 issues.

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement leaf prompt builders and strict JSON parsers in xenobot/utils/chatprompt/toys.go** - `6d973e8d` (feat)
2. **Task 2: Add comprehensive pure unit tests in xenobot/utils/chatprompt/toys_test.go** - `17ad4bed` (test)

## Files Created/Modified
- `xenobot/utils/chatprompt/toys.go` - Leaf prompt builders, token constants, and strict JSON/text parsers for `/referendum`, `/inspect`, and tie-breakers.
- `xenobot/utils/chatprompt/toys_test.go` - Pure unit tests for token floors, prompt sanitization, JSON parsing, rune caps, deduplication, and lookalike emoji rejection.

## Decisions Made
- `ParseInspectJSON` keeps a valid `Verdict` (`ok == true`) and clears `Emoji` (`""`, `EmojiValid: false`) when the chat's allowed reaction palette is empty or the model picks a disallowed/lookalike emoji, allowing `/inspect` to deliver text verdicts even in custom-only reaction chats.
- `cleanInspectVerdict` cleans each line with `Clean`, drops empty lines, caps at 3 lines, and bounds total length to `InspectVerdictMaxRunes` (600) to guarantee plain-text 2–3 line verdicts free of control/bidi characters.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- `xenobot/utils/chatprompt/toys.go` is ready for Wave 2 plans `35-05` (`chat_polls.go`) and `35-06` (`chat_inspect.go`).

## Self-Check: PASSED

---
*Phase: 35-interactive-group-toys*
*Completed: 2026-11-27*
