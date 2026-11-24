---
phase: 28-advanced-thresholds
plan: 05
subsystem: settings-ui
tags: [settings, ui, i18n, locales, authorization, callbacks, thresholds]

requires:
  - phase: 28-advanced-thresholds
    provides: chat_advanced_thresholds skipLocal repository, Field/Area specs, and applyAdvancedThreshold / resetAdvancedThreshold / resetAdvancedArea (28-02)
  - phase: 28-advanced-thresholds
    provides: AI spam bars and member requirements cap/pause runtime wiring (28-03)
  - phase: 28-advanced-thresholds
    provides: Lockdown reminder interval and Option A extended captcha timeout runtime wiring plus newPhase28LockdownEnv helper (28-04)
provides:
  - Private-only Advanced settings page av (Order 81, Icon 🎛, Hidden in Staff Groups) with hub, 6 area screens, 12 item screens, curated presets, item & area resets, and strict step-1 validation
  - Complete 8-language locale support (56 new keys in en/uk/ru/es/fr/pt/id/hi) with length caps and grid label guards in fuku/i18n/settings_locale_test.go
  - Updated Main, Staff Group, and Phase 19 grid tests (settingsMainOwnerRows = 16, staffOwnerRows = settingsMainOwnerRows - 3)
  - Comprehensive TestSettingsAdvancedThresholds suite (6 subtests, zero localhost TCP dials) covering display, presets/default normalization/resets, live settingsRightRestrict & owner gate, step-1 forged/out-of-range/group-menu rejection, 64-byte worst-case callbacks & 8 real locales, and Staff Group hiding
affects: [28-06]

tech-stack:
  added: []
  patterns:
    - "Table-driven private-only settings page (settingsAdvAreas + settingsAdvItems) with step-1 private(...) validation allow-listing exact area/item codes and canonical curated presets before any Telegram or DB I/O (T-28-13)"
    - "Live settingsRightAdmin + owner gate for views (av, aa, ao) and live settingsRightRestrict for all writes/resets (as, ar, ax) (P28-03, T-28-12)"
    - "Staff Group hiding via fresh staff.GetStaffGroupFresh seam (settingsAdvStaffLookup) and PrivateOnly group-menu short-circuit with 0 DB reads (T-28-14)"

key-files:
  created:
    - fuku/modules/settings_advthresholds.go
    - fuku/modules/settings_advthresholds_test.go
  modified:
    - fuku/i18n/settings_locale_test.go
    - fuku/modules/settings_main_test.go
    - fuku/modules/settings_fwdblock_test.go
    - fuku/modules/settings_phase19_test.go
    - locales/en.yml
    - locales/es.yml
    - locales/fr.yml
    - locales/hi.yml
    - locales/id.yml
    - locales/pt.yml
    - locales/ru.yml
    - locales/uk.yml

key-decisions:
  - "P28-03: Viewing av, aa, and ao requires live settingsRightAdmin + the settings owner gate (private chat only, hidden in Staff Groups); writing or resetting (as, ar, ax) requires live settingsRightRestrict (group creator or can_restrict_members)."
  - "Formatted AI spam bar thresholds (rib, mrb) as 0.XX decimals (0.50–0.99, defaults 0.85 and 0.90) via settings_adv_unit_bar and displayed unset captcha_timeout_mins as settings_adv_captcha_default ('Captcha page (1–10 min)') with range 11 min–60 min."
  - "Built newPhase28SettingsEnv on top of newPhase28LockdownEnv (phase28InProcessMiniredis) so TestSettingsAdvancedThresholds runs under -race with zero localhost TCP dials."

patterns-established:
  - "newPhase28SettingsEnv: in-process miniredis settings environment for fast, sandbox-safe settings page tests."

requirements-completed: [TUNE-07]

status: complete
plan_head_before: 30b3ce0456592dd9f4d0b005f8a7d4f9d4482829
plan_head_after: 21a06b35584b61e60e9d0c4ea5af381e8603fc85
commits: 3

actuals:
  tokens: 25207
  tasks: 3
  commits: 3

duration: 21min
completed: 2026-11-23
---

# Phase 28 Plan 05: Private Advanced Settings Page (`av`) & 8-Locale Copy Summary

**Private-only Advanced settings page (`av`, Order `81`, Icon `🎛`, hidden in Staff Groups) exposing all 12 tunable thresholds across 6 areas with current/default/range display, curated safe presets, per-item & per-area resets, live `settingsRightRestrict` authorization, step-1 forged-callback defense, and full 8-language localization**

## Performance

- **Duration:** 21 min
- **Started:** 2026-11-23T12:19:02Z
- **Completed:** 2026-11-23T12:40:12Z
- **Tasks:** 3
- **Files modified:** 14 (2 created, 12 modified)

## Accomplishments

- Added all 56 `settings_page_advanced` / `settings_state_adv_n` / `settings_adv_*` / `settings_btn_adv_*` keys across all 8 locale files (`en`, `uk`, `ru`, `es`, `fr`, `pt`, `id`, `hi`) with identical placeholder sets, zero Staff Group references, full `locale-style.md` compliance for `en`/`uk`, and registered length caps plus `{"🎛", "settings_page_advanced"}` in `fuku/i18n/settings_locale_test.go`.
- Implemented `fuku/modules/settings_advthresholds.go` registering page `av` (`Order: 81`, `Icon: "🎛"`, `TitleKey: "settings_page_advanced"`, `PrivateOnly: true`, `PrivateNoteKey: "settings_adv_group_note"`, `Hidden: settingsAdvHidden`) and all 5 action codes (`aa`, `ao`, `as`, `ar`, `ax`), every validator wrapped in `private(...)` requiring `fields["c"] != ""` and exact area/item/canonical-preset allow-lists before any Telegram or DB I/O.
- Updated `fuku/modules/settings_main_test.go` (`settingsMainShapeGroup`, `settingsMainShapePrivate`, `settingsMainOwnerRows = 16`, `settingsMainTitleKeys`), `fuku/modules/settings_fwdblock_test.go` (`staffOwnerRows = settingsMainOwnerRows - 3`), and `fuku/modules/settings_phase19_test.go` (`"av"` in registered pages).
- Added `TestSettingsAdvancedThresholds` in `fuku/modules/settings_advthresholds_test.go` with all 6 subtests (`hub_area_and_item_display`, `set_preset_default_normalization_and_resets`, `live_authority_and_owner_gate`, `forged_and_out_of_range_presses_refused_in_step1`, `worst_case_callbacks_and_8_locales`, `staff_group_hides_advanced_page`), running under `-race` in 1.15s with zero localhost TCP dials.

## Task Commits

1. **Task 1: add all 8 locale files' Advanced settings keys and update locale length tests** - `847033aa` (`feat(28-05): add Advanced settings locale keys across all 8 languages`)
2. **Task 2: implement private Advanced settings page (av) and update Main/Staff-Group/Phase-19 grid tests** - `8b854206` (`feat(28-05): implement private Advanced settings page av and update grid tests`)
3. **Task 3: add comprehensive Advanced settings page UI, authority, forged-callback, 64-byte, and 8-language tests** - `21a06b35` (`test(28-05): add Advanced settings page UI, authority, security, 64-byte, and 8-locale tests`)

## Files Created/Modified

- `fuku/modules/settings_advthresholds.go` — Table-driven Advanced settings page `av`, hub/area/item renderers, curated presets, item & area resets, and strict step-1 validation.
- `fuku/modules/settings_advthresholds_test.go` — `TestSettingsAdvancedThresholds` (6 subtests, zero localhost TCP dials) covering UI, presets, default normalization, resets, live authority & owner gate, step-1 forged/out-of-range/group-menu rejection, 64-byte worst-case callbacks & 8 locales, and Staff Group hiding.
- `fuku/i18n/settings_locale_test.go` — Length caps for all 56 new keys and `{"🎛", "settings_page_advanced"}` in `TestSettingsLabelLengths`.
- `locales/{en,uk,ru,es,fr,pt,id,hi}.yml` — Complete 8-language translations for the Advanced settings page.
- `fuku/modules/settings_main_test.go`, `fuku/modules/settings_fwdblock_test.go`, `fuku/modules/settings_phase19_test.go` — Updated Main grid shapes, row counts (`16`, `settingsMainOwnerRows - 3`), title keys, and registered page list for `av`.

## Decisions Made

- Kept AI spam bar values formatted as `0.50`–`0.99` (`0.85` / `0.90` defaults) and unset extended captcha timeout displayed as `settings_adv_captcha_default` (`Captcha page (1–10 min)`) with range `11 min–60 min`, matching ROADMAP and Option A semantics.
- Reused `newPhase28LockdownEnv` (`phase28InProcessMiniredis`) inside `newPhase28SettingsEnv` so `TestSettingsAdvancedThresholds` runs fast under `-race` with zero localhost TCP dials even inside sandboxed environments.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- Pre-existing grid tests (`TestSettingsMainPhase19`, `TestSettingsPhase19Codes`, `TestPhase27Codes`) use `withMiniredisServer` (TCP loopback), which requires loopback access outside the sandbox; re-running with loopback enabled passed all grid and registry suites cleanly in 2.6s. The new `TestSettingsAdvancedThresholds` uses `phase28InProcessMiniredis` and passes inside the sandbox with zero localhost TCP dials in 1.15s.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Private Advanced settings page `av`, 8-locale copy, Main/Staff-Group grid integration, and UI/authority/security tests are complete and verified under `-race`.
- Ready for Plan 28-06 (final Phase 28 safety/privacy/zero-write guard suite `settings_phase28_test.go`, docs/AGENTS verification, and full-phase sign-off).

## Self-Check: PASSED

- All created and modified files verified present on disk (`fuku/modules/settings_advthresholds.go`, `fuku/modules/settings_advthresholds_test.go`, `fuku/i18n/settings_locale_test.go`, `fuku/modules/settings_main_test.go`, `fuku/modules/settings_fwdblock_test.go`, `fuku/modules/settings_phase19_test.go`, and all 8 `locales/*.yml`).
- All 3 task commits (`847033aa`, `8b854206`, `21a06b35`) verified as ancestors of HEAD with 0 accidental file deletions.
- `fuku/modules/advthresholds_phase28_legacy_request_test.go`, `go.mod`, and `go.sum` verified untouched.
- Acceptance greps verified:
  - `grep -c '^settings_page_advanced:' locales/*.yml | grep -c ':1$'` → `8`
  - `grep -c 'settings_page_advanced' fuku/i18n/settings_locale_test.go` → `2`
  - `grep -c 'settingsPageAdvanced' fuku/modules/settings_advthresholds.go` → `4`
  - `grep -cE 'settingsMainOwnerRows\s*=\s*16' fuku/modules/settings_main_test.go` → `1`
  - `grep -c 'settingsMainOwnerRows - 3' fuku/modules/settings_fwdblock_test.go` → `1`
  - `TestSettingsAdvancedThresholds` subtest PASS count → `6`
- Full verification suite passed:
  - `go test -tags testtools -count=1 -run '^(TestSettingsLocaleKeys|TestSettingsLabelLengths|TestLocaleStyle|TestLocalePlaceholdersMatchOtherLanguages)$' ./fuku/i18n` — PASS (0.51s)
  - `go test -tags testtools -race -count=1 -run '^TestSettingsAdvancedThresholds$' -v ./fuku/modules` — PASS (6/6, 1.15s, zero localhost TCP dials)
  - `go test -tags testtools -race -count=1 -run '^(TestSettingsAdvancedThresholds|TestSettingsMainGrid|TestSettingsMainPhase18|TestSettingsMainPhase19|TestSettingsFwdBlockMainShape|TestSettingsPhase19Codes|TestPhase27Codes|TestLegacyPhase28DefaultThresholds)$' ./fuku/modules` — PASS (3.18s)
  - `make check-translations` — PASS (all 1068 keys present across all 8 locales)
  - `CGO_ENABLED=0 go build ./...` — PASS
  - `go vet -tags testtools ./fuku/modules ./fuku/i18n` — PASS
