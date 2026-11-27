---
phase: 34-room-aware-conversations-and-frictionless-catch-up
plan: 07
subsystem: settings-ui
tags: [presence, settings, telegram-ui, owner-admin-gate, i18n, tests]

requires:
  - phase: 34-01
    provides: presence.Settings AmbientContext/VocativeEnabled/PMBriefReact and column-targeted Set* writers
  - phase: 34-03
    provides: 8-locale settings_pr_room_*, settings_pc_*, settings_btn_pc_*, and Owner-lock alert strings
provides:
  - applyPresenceAmbientContext, applyPresenceVocativeEnabled, and applyPresencePMBriefReact in xenobot/modules/presence.go
  - Private Settings page pr Room & Catch-Up sub-page button (a=pc, keeping pr at 8 rows <= 8, 31 pages, 17 Main rows)
  - Sub-page pc (renderSettingsPresenceRoomCatchup) with Owner×Admin double-gated controls ax, ve, pb and admin disable when locked
  - 8-locale UI fit, 64-byte callback, Owner×Admin gate, Staff Group fail-closed, and Phase 33/34 code tests
affects:
  - 34-08 (Phase 34 verification & docs generation)

actuals:
  tokens: 11953
  tasks: 3
  commits: 3
  plan_head_before: ceca608329318ad1b57633f59b3fa684f3e27a47
  plan_head_after: 62813e3c1d0e8eb2eb4a65d2534d3b2eb2a6f20a

tech-stack:
  added: []
  patterns:
    - "Shared settingsPresenceSetRoomToggle helper enforcing Owner×Admin gates on enable (v=1) while allowing authorized admins to disable (v=0) when locked"
    - "Sub-page pc (4 rows) keeps Presence page pr at exactly 8 rows (<= 8 invariant) without adding top-level pages or Main rows"

key-files:
  created: []
  modified:
    - xenobot/modules/presence.go
    - xenobot/modules/presence_test.go
    - xenobot/modules/settings_presence.go
    - xenobot/modules/settings_presence_test.go
    - xenobot/modules/settings_phase33_test.go

key-decisions:
  - "Rendered stored boolean values on pc toggles and pr summary so an admin can still tap v=0 to disable an active toggle after the Owner turns off Conversations or Message log."
  - "Added settingsPresenceChatLocks(chatID) returning both convUnlocked and logUnlocked in a single chatconv.GetFresh read so MainState and sub-page pc avoid duplicate DB lookups."

patterns-established:
  - "Presence sub-pages (ph, pc) reuse page pr with PrivateOnly action codes, settingsRightAdmin entry, and settingsRightChangeInfo mutators."

requirements-completed:
  - CHAT-02
  - CHAT-03
  - CHAT-05

coverage:
  - id: D1
    description: "presenceWrites fields and applyPresenceAmbientContext, applyPresenceVocativeEnabled, applyPresencePMBriefReact helpers with read-back and save-failure handling"
    requirement: CHAT-02
    verification:
      - kind: unit
        ref: "xenobot/modules/presence_test.go#TestPresenceApply"
        status: pass
      - kind: unit
        ref: "xenobot/modules/presence_test.go#TestPresenceSafetyGate"
        status: pass
    human_judgment: false
  - id: D2
    description: "Settings page pr (8 rows <= 8, 31 pages, 17 Main rows) and sub-page pc with Owner×Admin double-gating on ax/ve/pb and admin disable while locked"
    requirement: CHAT-03
    verification:
      - kind: unit
        ref: "xenobot/modules/settings_presence_test.go#TestSettingsPresenceRoomCatchup"
        status: pass
      - kind: unit
        ref: "xenobot/modules/settings_phase33_test.go#TestPhase33Codes"
        status: pass
    human_judgment: false
  - id: D3
    description: "8-locale UI fit (<= 8 rows, <= 3800 UTF-16 units, <= 64-byte callbacks, 20/40-rune buttons) and MainState count parity across ax/ve/pb"
    requirement: CHAT-05
    verification:
      - kind: unit
        ref: "xenobot/modules/settings_presence_test.go#TestSettingsPresenceCallbackBytes"
        status: pass
      - kind: unit
        ref: "xenobot/modules/settings_presence_test.go#TestSettingsPresencePageFitsEveryLocale"
        status: pass
      - kind: unit
        ref: "xenobot/modules/settings_presence_test.go#TestSettingsPresenceFlows"
        status: pass
      - kind: other
        ref: "GOTOOLCHAIN=go1.26.0 golangci-lint run ./xenobot/modules"
        status: pass
    human_judgment: false

duration: 4 min
completed: 2026-11-27
status: complete
---

# Phase 34 Plan 07: Presence Apply Helpers & Room & Catch-Up Settings Sub-Page Summary

**Added `applyPresenceAmbientContext`, `applyPresenceVocativeEnabled`, and `applyPresencePMBriefReact` to `presence.go` and exposed `ax`, `ve`, and `pb` on a 4-row `Room & Catch-Up` sub-page (`a=pc`) under Presence (`pr`, 8 rows), with Owner×Admin double-gating, 8-locale UI fit, and fail-closed Staff Group protection.**

## Performance

- **Duration:** 4 min
- **Started:** 2026-11-27T07:38:07Z
- **Completed:** 2026-11-27T07:42:21Z
- **Tasks:** 3
- **Files modified:** 5

## Accomplishments
- Extended `presenceWrites` and added `applyPresenceAmbientContext`, `applyPresenceVocativeEnabled`, and `applyPresencePMBriefReact` in `xenobot/modules/presence.go`, with persistence, read-back, and save-failure unit coverage in `xenobot/modules/presence_test.go`.
- Added controls `ax` (`AmbientContext`), `ve` (`VocativeEnabled`), and `pb` (`PMBriefReact`) plus action `pc` (`settingsActPresenceRoom`) on page `pr` in `xenobot/modules/settings_presence.go`, keeping `len(settingsPages) == 31`, `settingsMainOwnerRows == 17`, and `pr` at 8 rows (`<= 8`).
- Implemented `settingsPresenceChatLocks` / `settingsPresenceChatLocksForView`, updated `settingsPresenceMainState` to count unlocked room & catch-up features, and built `renderSettingsPresenceRoomCatchup` with `settingsPresenceSetRoomToggle` enforcing Owner×Admin double-gating on enable (`ax`: `Enabled && LogEnabled`, `ve`: `Enabled`, `pb`: `LogEnabled`, with configured provider) while allowing authorized admins to disable (`v=0`) even when locked.
- Updated `xenobot/modules/settings_presence_test.go` and `xenobot/modules/settings_phase33_test.go` to verify 8-row `pr`, 4-row `pc`, `<= 64`-byte callbacks, `<= 3800` UTF-16 units across all 8 locales, Owner×Admin double-gating, `CanChangeInfo` enforcement, group-menu refusal, and Staff Group fail-closed protection under `-race`.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add Phase 34 apply helpers to presence.go and presence_test.go** - `09687904` (feat)
2. **Task 2: Add Room & Catch-Up sub-page pc under Settings page pr with Owner×Admin double-gating** - `79b21675` (feat)
3. **Task 3: Update settings_presence_test.go and settings_phase33_test.go for 8-row pr, sub-page pc, and Owner×Admin gates** - `62813e3c` (test)

## Files Created/Modified
- `xenobot/modules/presence.go` - Added `AmbientContext`, `VocativeEnabled`, `PMBriefReact` writers to `presenceWrites` and `applyPresence*` helpers.
- `xenobot/modules/presence_test.go` - Added write/read-back and failure tests for all three Phase 34 `applyPresence*` helpers.
- `xenobot/modules/settings_presence.go` - Added `ax`/`ve`/`pb` controls, `pc` sub-page action, `settingsPresenceChatLocks`, `renderSettingsPresence` row 7 (`a=pc`), `renderSettingsPresenceRoomCatchup`, and `settingsPresenceSetRoomToggle`.
- `xenobot/modules/settings_presence_test.go` - Updated `pr` row assertions (7 -> 8), added 8-locale & callback-byte coverage for `pc`, and added `TestSettingsPresenceRoomCatchup`.
- `xenobot/modules/settings_phase33_test.go` - Updated `pr` action (3: `ph`, `pu`, `pc`) and control (12) assertions and step-1 validation checks.

## Decisions Made
- Rendered stored boolean values on `pc` toggles and the `pr` summary line so an admin can still press `v=0` to disable an active toggle after the Owner turns off Conversations or Message log, while `settingsPresenceMainState` counts only unlocked active features.
- Added `settingsPresenceChatLocks` returning `(convUnlocked, logUnlocked, err)` from one `chatconv.GetFresh` lookup so `settingsPresenceMainState` and `pc` rendering avoid redundant DB reads.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- Running Task 2's `<verify>` command prior to Task 3 failed only on the three legacy count assertions in `settings_presence_test.go` and `settings_phase33_test.go` (`rows = 8, want 7`, `3 action codes, want 2`), which Task 3 updated as planned; after Task 3, all tests passed under `-race` and `golangci-lint` reported 0 issues.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Presence apply layer and Settings `pr` / `pc` UI for `ambient_context`, `vocative_enabled`, and `pm_brief_react` are ready for Phase 34 Plan 08 verification and docs generation.

## Self-Check: PASSED
- Verified all 5 modified files and `34-07-SUMMARY.md` exist on disk.
- Verified task commits `09687904`, `79b21675`, and `62813e3c` exist in git history (`git merge-base --is-ancestor`).
- Verified `go test -tags testtools -race -count=1 ./xenobot/modules -run 'TestPresenceApply|TestPresenceSafetyGate|TestSettingsPresence|TestPhase33Codes|TestSettingsMain|TestSettingsCallback'` and `GOTOOLCHAIN=go1.26.0 golangci-lint run ./xenobot/modules` pass with 0 failures and 0 lint issues.

---
*Phase: 34-room-aware-conversations-and-frictionless-catch-up*
*Completed: 2026-11-27*
