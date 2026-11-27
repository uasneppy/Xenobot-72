---
phase: 35-interactive-group-toys
plan: 04
subsystem: ui
tags: [presence, settings, redis, owner-admin-gating, i18n, toys]

# Dependency graph
requires:
  - phase: 35-01
    provides: presence.SetReferendumEnabled, presence.SetInspectEnabled, and chat_presence schema/repo
  - phase: 35-03
    provides: 8-locale settings_pc_referendum_line, settings_pc_inspect_line, settings_btn_pc_referendum, and settings_btn_pc_inspect keys
provides:
  - Operational Redis toy key builders outside xenobot:cache: (presencePollKey, presencePollTieKey, presenceToyChatCooldownKey, presenceToyUserCooldownKey)
  - Presence apply-layer writers applyPresenceReferendumEnabled and applyPresenceInspectEnabled via presenceWrites
  - Private Settings sub-page pc toggles rf (ReferendumEnabled) and in (InspectEnabled) with Owner×Admin double-gating via settingsPresenceNeedConv
affects: [35-05-referendum-and-tie-breaker, 35-06-inspect, 35-07-docs-and-verification]

actuals:
  tokens: 8461
  tasks: 3
  commits: 3

plan_head_before: 32b5085d7878b0695b19ba7f4f1eaea1f1045ea3
plan_head_after: 51f71722d251ad7cbf5a775b84b0fe94dc680a04

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Operational toy Redis keys under xenobot:presence: (poll:<id>, poll:tie:<id>, {<chat>}:toy:cd, {<chat>}:toy:cd:u:<user>)"
    - "Sub-page pc expanded to 6 rows (<= 8) while keeping pr at 8 rows, len(settingsPages) == 31, and settingsMainOwnerRows == 17"
    - "rf and in enforce settingsPresenceNeedConv (settings_pr_ai_need_owner) on enable while permitting admin disable when locked"

key-files:
  created: []
  modified:
    - xenobot/modules/presence.go
    - xenobot/modules/presence_test.go
    - xenobot/modules/settings_presence.go
    - xenobot/modules/settings_presence_test.go
    - xenobot/modules/settings_phase33_test.go

key-decisions:
  - "Placed rf and in directly on sub-page pc (bringing pc from 4 to 6 rows) with settingsPresenceNeedConv gating rather than adding a new settings page or action code."

patterns-established:
  - "Toy settings toggles follow settingsPresenceSetRoomToggle(p, value, settingsPresenceNeedConv, ...) so Owner×Admin gating and fail-closed DB error handling stay uniform."

requirements-completed: [TOYS-01, TOYS-03]

coverage:
  - id: D1
    description: "Operational Redis toy key builders (presencePollKey, presencePollTieKey, presenceToyChatCooldownKey, presenceToyUserCooldownKey) and presenceWrites/apply helpers for ReferendumEnabled and InspectEnabled"
    requirement: TOYS-01
    verification:
      - kind: unit
        ref: "xenobot/modules/presence_test.go#TestPresenceKeys"
        status: pass
      - kind: unit
        ref: "xenobot/modules/presence_test.go#TestPresenceApply"
        status: pass
      - kind: unit
        ref: "xenobot/modules/presence_test.go#TestPresenceSafetyGate"
        status: pass
    human_judgment: false
  - id: D2
    description: "Sub-page pc exposes rf and in (6 rows <= 8, pr 8 rows, 31 pages, 17 Main rows) with settingsPresenceNeedConv Owner×Admin double-gating and accurate Main/pr counts"
    requirement: TOYS-03
    verification:
      - kind: unit
        ref: "xenobot/modules/settings_presence_test.go#TestSettingsPresenceRoomCatchup"
        status: pass
      - kind: unit
        ref: "xenobot/modules/settings_phase33_test.go#TestPhase33Codes"
        status: pass
      - kind: unit
        ref: "xenobot/modules/settings_main_test.go#TestSettingsMain"
        status: pass
    human_judgment: false
  - id: D3
    description: "All 8 locales render 6-row pc within <= 8 rows, <= 3800 UTF-16 units, <= 64-byte callbacks, and Staff Groups stay hidden and fail closed"
    requirement: TOYS-03
    verification:
      - kind: unit
        ref: "xenobot/modules/settings_presence_test.go#TestSettingsPresenceCallbackBytes"
        status: pass
      - kind: unit
        ref: "xenobot/modules/settings_presence_test.go#TestSettingsPresencePageFitsEveryLocale"
        status: pass
      - kind: unit
        ref: "GOTOOLCHAIN=go1.26.0 golangci-lint run ./xenobot/modules/..."
        status: pass
    human_judgment: false

# Metrics
duration: 4 min
completed: 2026-11-27
status: complete
---

# Phase 35 Plan 04: Presence Apply Layer, Toy Redis Keys, and Settings UI Summary

**Added `presence.go` operational toy Redis key builders and `applyPresenceReferendumEnabled`/`applyPresenceInspectEnabled` writers, and exposed `rf` and `in` on private Settings sub-page `pc` (`6` rows `<= 8`, `pr` `8` rows, `31` pages, `17` Main rows) with Owner×Admin double-gating across all 8 locales.**

## Performance

- **Duration:** 4 min
- **Started:** 2026-11-27T12:57:47Z
- **Completed:** 2026-11-27T13:02:11Z
- **Tasks:** 3
- **Files modified:** 5

## Accomplishments
- Added `presencePollKey` (`xenobot:presence:poll:<id>`), `presencePollTieKey` (`xenobot:presence:poll:tie:<id>`), `presenceToyChatCooldownKey` (`xenobot:presence:{<chat>}:toy:cd`), and `presenceToyUserCooldownKey` (`xenobot:presence:{<chat>}:toy:cd:u:<user>`) in `xenobot/modules/presence.go` strictly outside `xenobot:cache:`, alongside `presenceWrites.ReferendumEnabled`/`InspectEnabled` and `applyPresenceReferendumEnabled`/`applyPresenceInspectEnabled`.
- Registered `settingsPresenceControlReferendum` (`rf`) and `settingsPresenceControlInspect` (`in`) on `settingsPagePresence` (`pr`) and expanded `renderSettingsPresenceRoomCatchup` (`pc`) from 4 to 6 rows (`<= 8`) while keeping `pr` at 8 rows, `len(settingsPages) == 31`, and `settingsMainOwnerRows == 17`.
- Enforced Owner×Admin double-gating via `settingsPresenceNeedConv` (`settings_pr_ai_need_owner` alert when `!convUnlocked`, fail-closed `settings_cannot_open` on DB read error, admin `v=0` disable allowed when locked, `settingsRightChangeInfo`, and Staff Group refusal) and updated `settingsPresenceMainState` and `renderSettingsPresence` `roomActive` counts.
- Extended `xenobot/modules/presence_test.go`, `xenobot/modules/settings_presence_test.go`, and `xenobot/modules/settings_phase33_test.go` covering 6-row `pc`, `<= 64`-byte callbacks, 8-locale fit, Owner×Admin gating, Staff Groups, and `TestPhase33Codes`, all passing under `-race` with 0 `golangci-lint` issues.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add toy apply helpers and Redis operational key builders to presence.go and presence_test.go** - `9479d466` (feat)
2. **Task 2: Add referendum_enabled (rf) and inspect_enabled (in) to sub-page pc with settingsPresenceNeedConv gating** - `d65accf6` (feat)
3. **Task 3: Update settings_presence_test.go and settings_phase33_test.go for 6-row pc and rf/in double-gating** - `51f71722` (test)

## Files Created/Modified
- `xenobot/modules/presence.go` - Added 4 operational toy Redis key builders, `presenceWrites` fields, and `applyPresenceReferendumEnabled` / `applyPresenceInspectEnabled`.
- `xenobot/modules/presence_test.go` - Extended `TestPresenceKeys` and `TestPresenceApply` for toy keys, persistence, and error handling.
- `xenobot/modules/settings_presence.go` - Added `rf`/`in` controls on `pr`, 6-row `pc` rendering, `settingsPresenceNeedConv` setters, and Main/`pr` count updates.
- `xenobot/modules/settings_presence_test.go` - Updated `TestSettingsPresenceCallbackBytes`, `TestSettingsPresencePageFitsEveryLocale`, and `TestSettingsPresenceRoomCatchup` for 6-row `pc`, 8-locale fit, Owner×Admin gating, and Staff Group refusal.
- `xenobot/modules/settings_phase33_test.go` - Verified `rf` and `in` registration in `prPage.Controls` alongside 31 pages and 17 Main rows.

## Decisions Made
None - followed plan as specified.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `presence.go` Redis operational keys, apply-layer writers, and Settings `pc` toggles (`rf` and `in`) are ready for Wave 3 runtime handlers (`35-05` `/referendum` + tie-breaker and `35-06` `/inspect`).
- `STATE.md` and `ROADMAP.md` were left untouched for central orchestrator update after Wave 2.

## Self-Check: PASSED
- Verified all 5 modified files exist on disk.
- Verified commits `9479d466`, `d65accf6`, and `51f71722` are ancestors of `HEAD`.
- Verified `go test -tags testtools -race -count=1 ./xenobot/modules -run 'TestPresenceKeys|TestPresenceApply|TestPresenceSafetyGate|TestSettingsPresence|TestPhase33Codes|TestSettingsMain|TestSettingsCallback'` and `GOTOOLCHAIN=go1.26.0 golangci-lint run ./xenobot/modules/...` passed after the final edit.

---
*Phase: 35-interactive-group-toys*
*Completed: 2026-11-27*
