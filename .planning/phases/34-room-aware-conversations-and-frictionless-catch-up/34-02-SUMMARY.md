---
phase: 34-room-aware-conversations-and-frictionless-catch-up
plan: 02
subsystem: chatprompt
tags: [go, chatprompt, unicode, vocative, ambient-context, leaf-package]

requires:
  - phase: 33-smarter-static-reactions-highlights-archive-cat-stickers
    provides: "xenobot/utils/chatprompt leaf package with UserWithFiles, line, Clean, and TestChatpromptPackageIsLeaf"
provides:
  - "chatprompt.UserWithAmbient prompt builder with sanitized ambient room chatter framing"
  - "chatprompt.UserWithFiles delegation to UserWithAmbient(nil, ...)"
  - "chatprompt.ParseVocativePrefix deterministic O(N) Unicode vocative parser across EN/UK/RU and custom botFirstName"
affects:
  - 34-05
  - 34-06

actuals:
  tokens: 4050
  tasks: 2
  commits: 2
plan_head_before: b81bedadec509cfb90a5e3ba3d9dc056a8bc4ac9
plan_head_after: ca757c9c01f709110746adb2c3db78b00e2f4609

tech-stack:
  added: []
  patterns:
    - "Leaf prompt package uses only Go stdlib (cmp, slices, strings, unicode, unicode/utf8) and xenobot/utils/xenopersona"
    - "Rune-by-rune unicode.SimpleFold prefix matching with delimiter boundary checks (zero regexp)"

key-files:
  created:
    - xenobot/utils/chatprompt/ambient.go
    - xenobot/utils/chatprompt/ambient_test.go
  modified:
    - xenobot/utils/chatprompt/chatprompt.go

key-decisions:
  - "Matched vocative prefixes rune-by-rune with unicode.SimpleFold and rejected invalid UTF-8 bytes so hot-path checks remain O(N) with zero allocations and no RE2 \\b bugs on Cyrillic"
  - "Rejected trailing word continuations on canonical BotName (e.g. Xenobot-72s) rather than falling back to the shorter Xenobot prefix across the hyphen"

patterns-established:
  - "Ambient same-thread turns render via line(t, MaxQuotedRunes) under [Recent room chatter in this topic, oldest first] before [Recent conversation, oldest first]"

requirements-completed:
  - CHAT-02
  - CHAT-03

coverage:
  - id: D1
    description: "UserWithAmbient frames ambient same-thread room turns under [Recent room chatter in this topic, oldest first] using line(t, MaxQuotedRunes), and UserWithFiles delegates to UserWithAmbient(nil, ...)"
    requirement: "CHAT-02"
    verification:
      - kind: unit
        ref: "xenobot/utils/chatprompt/ambient_test.go#TestUserWithAmbient"
        status: pass
      - kind: unit
        ref: "xenobot/utils/chatprompt/chatprompt_test.go#TestUserWithFiles"
        status: pass
    human_judgment: false
  - id: D2
    description: "ParseVocativePrefix matches leading Xenobot name variants across EN/UK/RU and custom botFirstName case-insensitively with Unicode rune boundaries and delimiter stripping"
    requirement: "CHAT-03"
    verification:
      - kind: unit
        ref: "xenobot/utils/chatprompt/ambient_test.go#TestParseVocativePrefix"
        status: pass
    human_judgment: false
  - id: D3
    description: "xenobot/utils/chatprompt remains a pure leaf package importing only Go stdlib and xenobot/utils/xenopersona"
    verification:
      - kind: unit
        ref: "xenobot/utils/chatprompt/leaf_test.go#TestChatpromptPackageIsLeaf"
        status: pass
    human_judgment: false

duration: 2min
completed: 2026-11-27
status: complete
---

# Phase 34 Plan 02: Ambient Prompt Framing & Unicode Vocative Parser Summary

**Pure leaf `UserWithAmbient` prompt builder with `UserWithFiles` delegation (`CHAT-02`) and zero-regexp Unicode `ParseVocativePrefix` parser across EN/UK/RU (`CHAT-03`)**

## Performance

- **Duration:** 2 min
- **Started:** 2026-11-27T07:28:32Z
- **Completed:** 2026-11-27T07:30:11Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Added `UserWithAmbient(ambient, history []Turn, quoted *Turn, message Turn, files []File) string` in `xenobot/utils/chatprompt/ambient.go`, placing sanitized ambient same-thread chatter under `[Recent room chatter in this topic, oldest first]` via `line(t, MaxQuotedRunes)` before `[Recent conversation, oldest first]`.
- Updated `UserWithFiles` in `xenobot/utils/chatprompt/chatprompt.go` to delegate directly to `UserWithAmbient(nil, history, quoted, message, files)` with byte-identical output.
- Implemented `ParseVocativePrefix(text, botFirstName string) (stripped string, matched bool)` using longest-first candidate sorting, `unicode.SimpleFold` rune matching, invalid UTF-8 rejection, and strict post-prefix delimiter/whitespace stripping.
- Added fast millisecond unit tests in `xenobot/utils/chatprompt/ambient_test.go` covering section ordering, injection flattening, `Xenobot-72 (member)` impersonation defense, EN/UK/RU vocatives, custom `botFirstName`, mid-word/mid-sentence non-matches, and invalid UTF-8 inputs.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add UserWithAmbient in xenobot/utils/chatprompt/ambient.go and delegate UserWithFiles** - `80573bc2` (feat)
2. **Task 2: Add pure Unicode ParseVocativePrefix and comprehensive unit tests** - `ca757c9c` (feat)

## Files Created/Modified
- `xenobot/utils/chatprompt/ambient.go` - `UserWithAmbient` prompt builder and pure Unicode `ParseVocativePrefix` parser
- `xenobot/utils/chatprompt/chatprompt.go` - `UserWithFiles` delegated to `UserWithAmbient(nil, ...)`
- `xenobot/utils/chatprompt/ambient_test.go` - Unit tests for `UserWithAmbient` and `ParseVocativePrefix`

## Decisions Made
- Used rune-by-rune `unicode.SimpleFold` matching rather than `strings.ToLower` + `strings.HasPrefix` to avoid heap allocations on the group `13` hot path and safely reject invalid UTF-8 sequences.
- When `BotName` (`Xenobot-72`) matches a prefix but fails the delimiter check (e.g. `"Xenobot-72s"`), `ParseVocativePrefix` immediately returns `("", false)` instead of retrying shorter `"Xenobot"` across the hyphen.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- `chatprompt.UserWithAmbient` and `chatprompt.ParseVocativePrefix` are ready for Wave 2 plans (`34-05` / `34-06`) in `xenobot/modules/`.

## Self-Check: PASSED
- Files verified: `xenobot/utils/chatprompt/ambient.go`, `xenobot/utils/chatprompt/chatprompt.go`, `xenobot/utils/chatprompt/ambient_test.go`
- Commits verified: `80573bc2`, `ca757c9c`
- Verification run: `go test -tags testtools -race -count=1 -v ./xenobot/utils/chatprompt` (all tests including `TestChatpromptPackageIsLeaf`, `TestUserWithAmbient`, `TestParseVocativePrefix` passed) and `GOTOOLCHAIN=go1.26.0 golangci-lint run --build-tags testtools ./xenobot/utils/chatprompt/...` (0 issues).

---
*Phase: 34-room-aware-conversations-and-frictionless-catch-up*
*Completed: 2026-11-27*
