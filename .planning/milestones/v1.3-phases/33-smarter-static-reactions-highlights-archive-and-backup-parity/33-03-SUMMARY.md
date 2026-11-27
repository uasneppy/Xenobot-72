---
phase: 33-smarter-static-reactions-highlights-archive-and-backup-parity
plan: 03
subsystem: i18n
tags: [locales, i18n, translations, yaml, settings-ui, presence, reactions, backup]

# Dependency graph
requires:
  - phase: 31-interactive-presence-reactions
    provides: Presence settings page pr keys and baseline locale structure
  - phase: 32-contextual-ai-reactions
    provides: Presence AI reaction settings keys and segmented button checks
provides:
  - All 47 Phase 33 translation keys across all 8 locales (en, uk, ru, es, fr, hi, id, pt)
  - settingsLocaleLimits ceilings, settingsAlertKeys entry, label length assertions, and private {id} exemption in xenobot/i18n/settings_locale_test.go
affects: [33-04, 33-05, 33-07, 33-08]

# Actuals (#2632)
actuals:
  tokens: 12825
  tasks: 2
  commits: 3
plan_head_before: 0367ed88087fb3ce54ec5067d2632845ed4bffed
plan_head_after: f1039625eab76b630f9a9d3965c66811b949c988

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Private-menu {id} placeholder exemption for settings_ph_target_linked alongside settings_staff_linked_to in TestSettingsReusedKeyPlaceholders"
    - "Phase 33 button length ceilings enforced in TestSettingsLabelLengths (<= 20 half-width archive choices, <= 40 full-width toggles/labels, <= 14 plain-text help name, <= 24 jump button)"

key-files:
  created: []
  modified:
    - locales/en.yml
    - locales/uk.yml
    - locales/ru.yml
    - locales/es.yml
    - locales/fr.yml
    - locales/hi.yml
    - locales/id.yml
    - locales/pt.yml
    - xenobot/i18n/settings_locale_test.go

key-decisions:
  - "Exempted settings_ph_target_linked in TestSettingsReusedKeyPlaceholders alongside settings_staff_linked_to because sub-page ph lives under PrivateOnly page pr and never renders in a group chat"
  - "Added explicit length assertions in TestSettingsLabelLengths for archive choice cells, help_module_name_presence, archive_jump_button, and backup_module_presence"

patterns-established:
  - "Phase 33 locale keys follow docs/src/content/docs/contributing/locale-style.md for uk/en and preserve exact {placeholder} sets across all 8 languages"

requirements-completed:
  - COMM-01
  - COMM-02
  - COMM-03
  - COMM-04
  - COMM-05

coverage:
  - id: D1
    description: "All 8 locale files define every Phase 33 key for Reactions options, Cat Stickers, Highlights Archive commands/UI, Presence help, and Backup Presence with matching placeholders"
    requirement: COMM-01
    verification:
      - kind: unit
        ref: "go test -tags testtools -race -count=1 ./xenobot/i18n/..."
        status: pass
      - kind: other
        ref: "make check-translations"
        status: pass
    human_judgment: false
  - id: D2
    description: "xenobot/i18n/settings_locale_test.go enforces settingsLocaleLimits ceilings, plain-text alert checks, and button rune limits across all 8 languages"
    requirement: COMM-03
    verification:
      - kind: unit
        ref: "go test -tags testtools -race -count=1 ./xenobot/i18n/..."
        status: pass
    human_judgment: false

# Metrics
duration: 5 min
completed: 2026-11-27
status: complete
---

# Phase 33 Plan 03: Phase 33 Locales & Ceilings Summary

**Added all 47 Phase 33 translation keys (Reactions Options, Cat Stickers, Highlights Archive commands/UI, Presence help, and Backup Presence) across all 8 locales with enforced rune ceilings and plain-text alert guards.**

## Performance

- **Duration:** 5 min
- **Started:** 2026-11-27T06:48:49Z
- **Completed:** 2026-11-27T06:53:54Z
- **Tasks:** 2
- **Files modified:** 9

## Accomplishments
- Added `help_module_name_presence`, `presence_help_msg`, 9 `archive_*` keys, `backup_module_presence`, `backup_import_archive_skipped`, 21 `settings_pr_*`/`settings_ph_*`/`settings_btn_ph_*` keys, and 13 `settings_btn_rct_*`/`settings_rct_*` keys to all 8 locale files (`en`, `uk`, `ru`, `es`, `fr`, `hi`, `id`, `pt`).
- Registered every new `settings_*` key in `settingsLocaleLimits`, added `settings_ph_need_target` to `settingsAlertKeys`, added `backup_module_presence`, `settings_btn_rct_opts`, `settings_btn_pr_cat`, `settings_btn_pr_archive`, `settings_btn_ph_enabled`, `settings_btn_ph_unlink`, `settings_btn_rct_wb`, `settings_btn_rct_cap`, `settings_btn_ph_filter_*`, `settings_btn_ph_del_*`, `help_module_name_presence`, and `archive_jump_button` length checks in `TestSettingsLabelLengths`.
- Verified `make check-translations` (1069 keys across all 8 locales) and `go test -tags testtools -race -count=1 ./xenobot/i18n/...` pass with zero failures.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add Phase 33 keys to en, uk, ru, and es locales and register settingsLocaleLimits** - `50591eef` (feat)
2. **Task 2: Add Phase 33 keys to fr, hi, id, and pt locales and run full i18n parity check** - `f1039625` (feat)

_(Note: Peer Wave 1 commit `68b61a8a` from `33-02` landed on `main` between plan start and Task 2 completion, so `git rev-list --count 0367ed88..f1039625` is 3 while this plan authored 2 task commits.)_

## Files Created/Modified
- `locales/en.yml` - English Phase 33 translations following `locale-style.md`
- `locales/uk.yml` - Ukrainian Phase 33 translations following `locale-style.md`
- `locales/ru.yml` - Russian Phase 33 translations
- `locales/es.yml` - Spanish Phase 33 translations
- `locales/fr.yml` - French Phase 33 translations
- `locales/hi.yml` - Hindi Phase 33 translations
- `locales/id.yml` - Indonesian Phase 33 translations
- `locales/pt.yml` - Portuguese Phase 33 translations
- `xenobot/i18n/settings_locale_test.go` - Added `settingsLocaleLimits`, `settingsAlertKeys`, `TestSettingsLabelLengths` checks, and `settings_ph_target_linked` private `{id}` exemption

## Decisions Made
- Exempted `settings_ph_target_linked` in `TestSettingsReusedKeyPlaceholders` alongside `settings_staff_linked_to` because the Highlights Archive sub-page `ph` is an action on the `PrivateOnly` Presence page `pr` and never renders in a group chat.
- Used `"{secs} s"` (and localized unit equivalents matching `settings_adv_unit_s`) for `settings_rct_cd_val` so placeholder parity and compact label length hold across all 8 locales.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Exempted private `{id}` placeholder in `settings_ph_target_linked`**
- **Found during:** Task 1 (`xenobot/i18n/settings_locale_test.go`)
- **Issue:** `TestSettingsReusedKeyPlaceholders` forbids `{id}` in any `settings_*` key except `settings_staff_linked_to`, which would reject `settings_ph_target_linked` (`Destination: <code>{id}</code>`).
- **Fix:** Updated `TestSettingsReusedKeyPlaceholders` to allow `settings_ph_target_linked` alongside `settings_staff_linked_to` since sub-page `ph` inherits `pr`'s `PrivateOnly` guard.
- **Files modified:** `xenobot/i18n/settings_locale_test.go`
- **Verification:** `go test -tags testtools -race -count=1 ./xenobot/i18n/...` passed across all 8 locales.
- **Committed in:** `50591eef` (part of Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 Rule 1 bug)
**Impact on plan:** Essential for `TestSettingsReusedKeyPlaceholders` to accept the private Highlights Archive destination key without weakening group privacy checks.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Wave 2 plans (`33-04`, `33-05`) and Wave 3 plans (`33-07`, `33-08`) can now reference every Phase 33 locale key with verified placeholder parity and button length compliance.

---
*Phase: 33-smarter-static-reactions-highlights-archive-and-backup-parity*
*Completed: 2026-11-27*

## Self-Check: PASSED
