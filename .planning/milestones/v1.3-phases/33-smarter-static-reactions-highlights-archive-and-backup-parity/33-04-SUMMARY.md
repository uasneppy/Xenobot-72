---
phase: 33-smarter-static-reactions-highlights-archive-and-backup-parity
plan: 04
subsystem: modules
tags: [reactions, unicode, redis, presence, settings, cat-stickers]

# Dependency graph
requires:
  - phase: 33-01
    provides: "Rule DTO, GetReactionRulesContext, GetReactionEntryFresh, SetReactionOptionsByID, and skipLocal reaction_rules:<chat>"
  - phase: 33-02
    provides: "CatStickerEnabled field on presence.Settings"
  - phase: 33-03
    provides: "Localized settings_rct_* / settings_btn_rct_* keys and ceilings across all 8 locales"
provides:
  - "Unicode word-boundary static reaction matcher (matchStaticReactionKeyword), caption matching, ChancePct roll, and Redis cooldowns outside xenobot:cache:"
  - "Zero-AI cat-sticker micro-triggers (👀/❤ intersected with AvailableReactions) behind Redis cooldown and lazy lockdown check"
  - "presenceSetReaction + ctx.Data[presenceStaticReactedKey] handoff preventing group 13 AI overwrites"
  - "Settings page x per-rule options UI (AddRowExtra + xo/xr/xs actions)"
affects: [reactions, presence, settings, group-13-ai-reactions]

# Actuals (#2632)
actuals:
  tokens: 29392
  tasks: 3
  commits: 10
plan_head_before: 872f4c79b1afb0b6410ba116d622d3177e4fbf9b
plan_head_after: 56cab4c5bd9c1557345331edd1c19f00027da5c2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Pure Go linear Unicode word-boundary scan (strings.Index + utf8.Decode* + unicode.IsLetter/IsDigit/_) with zero RE2 \\b"
    - "Lazy presenceIsLockedDown check immediately before claiming Redis cooldowns and calling presenceSetReaction"
    - "Group 8 -> Group 13 single-reaction handoff via ctx.Data[presenceStaticReactedKey] = true"

key-files:
  created:
    - xenobot/modules/reactions_smart_test.go
  modified:
    - xenobot/modules/reactions.go
    - xenobot/modules/settings_reactions.go
    - xenobot/modules/settings_reactions_test.go
    - xenobot/modules/settings_phase19_test.go

key-decisions:
  - "Claim per-rule Redis cooldown before per-chat anti-burst cooldown so a rule already on cooldown never consumes the chat burst window"
  - "Render the Options button via settingsReactionsAddRow unconditionally so xo is reachable and shows settings_rct_opts_empty cleanly if all rules are deleted"

patterns-established:
  - "Group 8 static and cat-sticker reactions share presenceMessageAllowed + lazy presenceIsLockedDown + presenceSetReaction"

requirements-completed: [COMM-01, COMM-02]

coverage:
  - id: D1
    description: "Unicode word-boundary, caption matching, trigger chance %, Redis cooldowns, Staff/lockdown gates, and presenceSetReaction handoff in reactions.go"
    requirement: "COMM-01"
    verification:
      - kind: unit
        ref: "xenobot/modules/reactions_smart_test.go#TestStaticReactionsUnicodeAndOptions"
        status: pass
      - kind: unit
        ref: "xenobot/modules/reactions_smart_test.go#TestStaticAndCatReactionsHandoffToGroup13"
        status: pass
      - kind: unit
        ref: "xenobot/modules/reactions_legacy_request_test.go#TestLegacyReactionRequests"
        status: pass
    human_judgment: false
  - id: D2
    description: "Zero-AI cat-sticker micro-triggers reacting with allowed 👀 or ❤ behind Redis cooldown and lazy lockdown check"
    requirement: "COMM-02"
    verification:
      - kind: unit
        ref: "xenobot/modules/reactions_smart_test.go#TestCatStickerMicroTriggers"
        status: pass
      - kind: unit
        ref: "xenobot/modules/reactions_smart_test.go#TestStaticAndCatReactionsHandoffToGroup13"
        status: pass
    human_judgment: false
  - id: D3
    description: "Settings page x per-rule options UI (xo/xr/xs) with private-only enforcement, can_change_info write gate, <= 64-byte callbacks, and 8-locale screen fit"
    requirement: "COMM-01"
    verification:
      - kind: unit
        ref: "xenobot/modules/settings_reactions_test.go#TestSettingsReactionsOptions"
        status: pass
      - kind: unit
        ref: "xenobot/modules/settings_reactions_test.go#TestSettingsListReactionsFits"
        status: pass
      - kind: unit
        ref: "xenobot/modules/settings_phase19_test.go#TestSettingsPhase19Codes"
        status: pass
    human_judgment: false

# Metrics
duration: 9min
completed: 2026-11-27
status: complete
---

# Phase 33 Plan 04: Smarter Static Reactions, Cat-Sticker Micro-Triggers, and Page x Options Summary

**Unicode word-boundary, caption, chance, and Redis-cooled static reactions with zero-AI cat-sticker micro-triggers (`COMM-01`, `COMM-02`), group `8 -> 13` handoff via `presenceStaticReactedKey`, and Settings page `x` per-rule options UI (`xo`/`xr`/`xs`)**

## Performance

- **Duration:** 9 min
- **Started:** 2026-11-27T06:54:41Z
- **Completed:** 2026-11-27T07:03:04Z
- **Tasks:** 3
- **Files modified:** 5

## Accomplishments
- Upgraded `checkReactions` in `xenobot/modules/reactions.go` (group `8`) to run `presenceMessageAllowed` first, match optional pure Go Unicode word boundaries (`isStaticWordRune` + `utf8.Decode*`), match captions when `MatchCaption` is enabled, gate via `ChancePct` and Redis cooldown keys under `xenobot:presence:{<chat>}:*` (outside `xenobot:cache:`), verify `presenceIsLockedDown` lazily, send via `presenceSetReaction`, and set `ctx.Data[presenceStaticReactedKey] = true` so group `13` AI reactions never overwrite static reactions.
- Added zero-AI cat-sticker micro-triggers (`tryCatStickerReaction`) reacting with allowed `👀` or `❤` (intersected with `presenceAllowedPalette`) when `CatStickerEnabled` is true and no static rule reacted, behind a Redis cooldown and lazy lockdown check.
- Extended `reactionWrites` (`Rules`, `Entry`, `SetOptionsByID`) and added `applyReactionSetOptions` with pre-DB bounds validation, while leaving `/addreaction`, `/removereaction`, `/reactions`, `/resetreactions`, and `reactions_legacy_request_test.go` untouched.
- Added `AddRowExtra` (`settingsReactionsAddRow`) and `xo`/`xr`/`xs` rule option picker, view, and setter actions on Settings page `x` (`xenobot/modules/settings_reactions.go`) with private-only (`c != ""`), strict `k`/`v` validation, `settingsRightChangeInfo` on `xs`, and `<= 64`-byte callbacks.
- Added `xenobot/modules/reactions_smart_test.go` and updated `xenobot/modules/settings_reactions_test.go` and `xenobot/modules/settings_phase19_test.go`.

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement Unicode boundary/caption/chance/cooldown static reactions and cat-sticker micro-triggers in reactions.go** - `1d86834e` (feat)
2. **Task 2: Add per-rule static reaction options UI to settings_reactions.go** - `419f08f4` (feat)
3. **Task 3: Add smart static reaction & cat-sticker tests and update settings_reactions_test / settings_phase19_test** - `56cab4c5` (test)

_Note: `plan_head_before..plan_head_after` counts 10 commits on `main` because Wave 2 peers `33-05` and `33-06` committed concurrently on disjoint files._

## Files Created/Modified
- `xenobot/modules/reactions.go` - Unicode boundary matcher, caption/chance/cooldown gating, cat-sticker micro-triggers, `presenceSetReaction` + `presenceStaticReactedKey` handoff, and `applyReactionSetOptions`.
- `xenobot/modules/settings_reactions.go` - `AddRowExtra` (`settingsReactionsAddRow`) and `xo`/`xr`/`xs` rule options UI on page `x`.
- `xenobot/modules/reactions_smart_test.go` - Unit and dispatcher tests for Unicode boundaries, captions, chance/cooldowns, Staff/lockdown gates, cat stickers, and group `8 -> 13` handoff.
- `xenobot/modules/settings_reactions_test.go` - Updated page `x` row widths (`[]int{3, 2, 2}`), extended `TestSettingsListReactionsFits` for `xo`/`xr` in all 8 locales, and added `TestSettingsReactionsOptions`.
- `xenobot/modules/settings_phase19_test.go` - Added `xo`, `xr`, `xs` to `settingsPhase19Codes`.

## Decisions Made
- Claim `staticRuleCooldownKey` before `staticChatCooldownKey` in `tryStaticReactionRules` so a cooled-down rule never consumes the chat-wide anti-burst window for subsequent rules.
- Render `settings_btn_rct_opts` unconditionally in `settingsReactionsAddRow` so the `xo` picker is consistently positioned next to Add and exercises `settings_rct_opts_empty` cleanly if the rule list becomes empty.

## Deviations from Plan

None - plan executed exactly as written.

## Verification
- `go test -tags testtools -race -count=1 ./xenobot/modules -run 'TestLegacyReactionRequests|TestStaticReactions|TestCatSticker|TestStaticAndCatReactions|TestSettingsReactions|TestSettingsListReactionsFits|TestSettingsParityReactionAdd|TestSettingsAuthorityParityReactions|TestSettingsPhase19Codes'` — PASS (`3.585s`)
- `go test -tags testtools -race -count=1 ./xenobot/modules -run 'TestReactionCommands|TestReactionsHelp|TestCheckReactions|TestReactionCap'` — PASS (`2.761s`)
- `GOTOOLCHAIN=go1.26.0 golangci-lint run ./xenobot/modules` — PASS (`0 issues`)
- `CGO_ENABLED=0 go build ./xenobot/modules` — PASS
- `git diff --name-only xenobot/modules/reactions_legacy_request_test.go` — empty (untouched)

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Smarter static reactions, zero-AI cat-sticker micro-triggers, and page `x` rule options are complete and verified under `-race` alongside `33-05` and `33-06`.

## Self-Check: PASSED

- All 5 created/modified files verified on disk (`reactions.go`, `reactions_smart_test.go`, `settings_reactions.go`, `settings_reactions_test.go`, `settings_phase19_test.go`).
- All 3 task commits (`1d86834e`, `419f08f4`, `56cab4c5`) verified as ancestors of `HEAD`.

---
*Phase: 33-smarter-static-reactions-highlights-archive-and-backup-parity*
*Completed: 2026-11-27*
