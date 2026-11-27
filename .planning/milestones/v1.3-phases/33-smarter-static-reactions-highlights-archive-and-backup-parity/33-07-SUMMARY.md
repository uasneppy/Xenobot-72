---
phase: 33-smarter-static-reactions-highlights-archive-and-backup-parity
plan: 07
subsystem: modules
tags: [telegram, settings-ui, presence, cat-stickers, highlights-archive, i18n]

# Dependency graph
requires:
  - phase: 33-smarter-static-reactions-highlights-archive-and-backup-parity
    provides: Phase 33 locale keys and rune ceilings (33-03) and applyPresence* writers for Cat Stickers & Highlights Archive (33-05)
provides:
  - Cat Stickers toggle (cs) on Presence settings page pr and updated settingsPresenceMainState counting
  - Highlights Archive sub-page (ph) and unlink action (pu) registered as settingsPageAction entries on pr (keeping len(settingsPages) == 31 and settingsMainOwnerRows == 17)
  - Highlights Archive controls (ae, ah, ap, ad) with unlinked target guard, PrivateOnly / Staff Group fail-closed defense, and live settingsRightChangeInfo enforcement
  - 8-locale screen fit tests, flow tests, and Phase 33 registry/step-1 refusal tests (settings_phase33_test.go)
affects:
  - 33-08 (Verification & AGENTS.md / docs sync)

# Actuals (#2632)
actuals:
  tokens: 21937
  tasks: 2
  commits: 2
plan_head_before: d111b980cc56a0abdd5acd5c149d0b242a6f691c
plan_head_after: 5013f0c42c4d482a3af48962f1c5eed76ce88ca1

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Sub-page ph and unlink pu registered as PrivateOnly settingsPageAction entries on settingsPagePresence (pr) so len(settingsPages) == 31 and settingsMainOwnerRows == 17 stay unchanged"
    - "Partial presence.Settings normalization in settings_presence.go renderers (ArchiveThreshold==0 -> 5, ArchiveEmojiMode==\"\" -> positive, ArchiveDelivery==\"\" -> copy) paired with clamp(2..50) for stepper targets"

key-files:
  created:
    - xenobot/modules/settings_phase33_test.go
  modified:
    - xenobot/modules/settings_presence.go
    - xenobot/modules/settings_presence_test.go

key-decisions:
  - "Refreshed ph in place when tapping the middle value button of the ah stepper (a=ph) while reusing settingsStepperRow for step clamping and bound markers"
  - "Checked action-code collision for ph/pu against settingsCoreActions and other pages' Actions (excluding settingsLockRows control codes such as lock photo ph, which live under t= rather than a=)"

patterns-established:
  - "All archive write controls (ae, ah, ap, ad) and pu enforce PrivateOnly, fail-closed Staff Group check, settingsRightChangeInfo, and redraw renderSettingsPresenceArchive in place"

requirements-completed:
  - COMM-02
  - COMM-03

coverage:
  - id: D1
    description: "Settings page pr exposes the Cat Stickers toggle (cs) and Highlights Archive button (ph) in 7 rows (<= 8) with <= 64-byte callbacks while preserving len(settingsPages) == 31 and settingsMainOwnerRows == 17."
    requirement: COMM-02
    verification:
      - kind: unit
        ref: "xenobot/modules/settings_presence_test.go#TestSettingsPresenceCallbackBytes"
        status: pass
      - kind: unit
        ref: "xenobot/modules/settings_phase33_test.go#TestPhase33Codes"
        status: pass
    human_judgment: false
  - id: D2
    description: "Highlights Archive sub-page ph configures enable/disable (refusing unlinked chats with settings_ph_need_target alert), threshold (2..50), emoji filter (positive/all), delivery mode (copy/forward), and unlink (pu) across all 8 locales with PrivateOnly, Staff Group, and settingsRightChangeInfo guards."
    requirement: COMM-03
    verification:
      - kind: unit
        ref: "xenobot/modules/settings_presence_test.go#TestSettingsPresencePageFitsEveryLocale"
        status: pass
      - kind: unit
        ref: "xenobot/modules/settings_presence_test.go#TestSettingsPresenceFlows"
        status: pass
    human_judgment: false

# Metrics
duration: 6 min
completed: 2026-11-27
status: complete
---

# Phase 33 Plan 07: Presence Settings UI & Highlights Archive Sub-Page Summary

**Extended Presence settings page `pr` (`xenobot/modules/settings_presence.go`) with the Cat Stickers toggle (`cs`) and the Highlights Archive sub-page (`ph`/`pu` + `ae`, `ah`, `ap`, `ad`) as `settingsPageAction` entries on `pr`, keeping `len(settingsPages) == 31`, `settingsMainOwnerRows == 17`, `<= 8` rows, and `<= 64`-byte callbacks across all 8 locales.**

## Performance

- **Duration:** 6 min
- **Started:** 2026-11-27T07:04:20Z
- **Completed:** 2026-11-27T07:10:20Z
- **Tasks:** 2
- **Files modified:** 3 (1 created, 2 modified)

## Accomplishments
- Added `settingsPresenceControlCatSticker` (`cs`), `settingsPresenceControlArchiveEnabled` (`ae`), `settingsPresenceControlArchiveThreshold` (`ah`), `settingsPresenceControlArchiveEmoji` (`ap`), `settingsPresenceControlArchiveDelivery` (`ad`), and actions `settingsActPresenceHighlights` (`ph`, `settingsRightAdmin`) and `settingsActPresenceArchiveUnlink` (`pu`, `settingsRightChangeInfo`) on `settingsPagePresence`.
- Updated `settingsPresenceMainState` and `renderSettingsPresence` (`pr`, 7 rows) and implemented `renderSettingsPresenceArchive` (`ph`, 5–6 rows) with partial `presence.Settings` normalization, `settings_ph_need_target` alert on unlinked enable, `settings_ph_unlinked` notice on unlink, and fail-closed `PrivateOnly` + Staff Group guards.
- Updated `xenobot/modules/settings_presence_test.go` (7-row `pr` check, `ph` callback bytes, 8-locale screen fit for both `pr` and `ph`, Cat Stickers & Highlights Archive interactive flows/rights/refusals) and created `xenobot/modules/settings_phase33_test.go` (`TestPhase33Codes`).

## Task Commits

Each task was committed atomically:

1. **Task 1: Add Cat Stickers control (cs) and Highlights Archive sub-page (ph / pu + ae, ah, ap, ad) to settings_presence.go** - `bf0b216a` (feat)
2. **Task 2: Update settings_presence_test and add settings_phase33_test** - `5013f0c4` (test)

## Files Created/Modified
- `xenobot/modules/settings_presence.go` - Cat Stickers toggle `cs`, Highlights Archive sub-page `ph`/`pu`, archive controls `ae`/`ah`/`ap`/`ad`, renderer `renderSettingsPresenceArchive`, and updated `settingsPresenceMainState`
- `xenobot/modules/settings_presence_test.go` - Updated `pr` row count (`5 -> 7`), added `ph` callback byte & 8-locale fit checks, and added Cat Stickers + Highlights Archive flow/guard subtests
- `xenobot/modules/settings_phase33_test.go` - Added `TestPhase33Codes` verifying `len(settingsPages) == 31`, `settingsMainOwnerRows == 17`, `pr` controls/actions, zero action collisions, and step-1 refusal of unknown fields and group-menu presses (`u` instead of `c`) with 0 `getChatMember` calls

## Decisions Made
- Reused `settingsStepperRow` for `ah` (`2..50`) and rewired the center value button to `{"a": settingsActPresenceHighlights}` so tapping the current value refreshes `ph` in place instead of jumping back to `pr`.
- Validated `ph`/`pu` action-code uniqueness against `settingsCoreActions` and all other pages' `Actions` maps without colliding on `settingsLockRows` control codes (`t=ph` is a control code on `k`, not an `a=` action code).

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- Initial `TestPhase33Codes` draft copied `settings_phase30_test.go`'s `lockCodes` check, which flagged `ph` against `settingsLockRows`'s photo lock control code (`t=ph`); aligned `TestPhase33Codes` to check action-code uniqueness against `settingsCoreActions` and other `settingsPages` `Actions` (matching `settings_phase18_test.go` and the plan specification).
- Note during broader test run: `TestPhase30EndToEnd` (from Phase 30/33-06 parity) failed on `"presence"` module parity in `settings_backup.go` (outside `33-07` scope). All `33-07` target tests passed cleanly under `-race`.

## User Setup Required
None - no external service configuration required.

## Verification
- `go test -tags testtools -race -count=1 ./xenobot/modules -run 'TestSettingsPresence|TestPhase33Codes|TestPhase30Codes|TestSettingsMain|TestSettingsChat|TestSettingsFwdBlockMainShape|TestPhase18'` — passed (`3.905s`)
- `GOTOOLCHAIN=go1.26.0 golangci-lint run --build-tags testtools ./xenobot/modules/...` — `0 issues`
- `make check-translations` — passed (1069 keys across all 8 locales)

## Next Phase Readiness
- Presence settings UI (`pr`, `cs`, `ph`, `pu`, `ae`, `ah`, `ap`, `ad`) is complete and verified; ready for `33-08`.

---
*Phase: 33-smarter-static-reactions-highlights-archive-and-backup-parity*
*Completed: 2026-11-27*

## Self-Check: PASSED
