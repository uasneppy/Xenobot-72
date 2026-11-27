---
phase: 31-presence-foundation-settings-page-and-zero-ai-social-reactions
plan: 03
subsystem: ui
tags: [i18n, locales, presence, settings, translations, style-guide]

# Dependency graph
requires:
  - phase: 31-presence-foundation-settings-page-and-zero-ai-social-reactions
    provides: chat_presence schema and Settings model (31-01)
provides:
  - Complete 8-locale YAML translations (en, uk, es, fr, hi, id, pt, ru) for all 11 Presence Settings page (pr) and Conversations shortcut keys
  - Width ceiling entries in settingsLocaleLimits and 20-rune Main grid / 40-rune toggle & shortcut guards in TestSettingsLabelLengths
affects:
  - 31-05-PLAN.md

actuals:
  tokens: 4319
  tasks: 3
  commits: 3
  plan_head_before: 5ef9943ac751bf3448fd461b8544cdc4c1f786bb
  plan_head_after: 817af85447beeda43d6c6afe04646e873ed50371

tech-stack:
  added: []
  patterns:
    - "8-locale settings_pr_* and settings_btn_pr_* copy with exact {n}/{state}/{count} placeholder parity and HTML-only markup"
    - "Composite label length enforcement for Main grid cell (✨ + settings_page_presence <= 20 runes) and substituted toggle/shortcut labels (<= 40 runes)"

key-files:
  created: []
  modified:
    - locales/en.yml
    - locales/uk.yml
    - locales/es.yml
    - locales/fr.yml
    - locales/hi.yml
    - locales/id.yml
    - locales/pt.yml
    - locales/ru.yml
    - xenobot/i18n/settings_locale_test.go

key-decisions:
  - "Matched settings_state_presence_on to existing per-locale compact counter patterns (e.g. uk 'увімк.: {n}', ru '{n} вкл') for consistent Main status block rendering"
  - "Registered settings_btn_pr_consensus and settings_btn_pr_retract in TestSettingsLabelLengths phase18Toggles and settings_btn_chat_presence in fullWidthLabels in addition to the Main grid cell check so rendered 40-rune button limits are CI-enforced across all 8 locales"

patterns-established:
  - "Presence status lines follow the plain-text Conversations style ({state}/{count}) without inline HTML tags while staying under tight settingsLocaleLimits ceilings"

requirements-completed: [PRES-03]

coverage:
  - id: D1
    description: "All 8 locale files (en, uk, es, fr, hi, id, pt, ru) define all 11 Presence Settings page and Conversations shortcut keys with identical placeholders and zero drift"
    requirement: PRES-03
    verification:
      - kind: other
        ref: "make check-translations"
        status: pass
      - kind: unit
        ref: "xenobot/i18n/settings_locale_test.go#TestSettingsLocaleKeys"
        status: pass
      - kind: unit
        ref: "xenobot/i18n/staff_locale_test.go#TestLocalePlaceholdersMatchOtherLanguages"
        status: pass
    human_judgment: false
  - id: D2
    description: "English and Ukrainian Presence copy obeys docs/src/content/docs/contributing/locale-style.md and passes TestLocaleStyle"
    requirement: PRES-03
    verification:
      - kind: unit
        ref: "xenobot/i18n/locale_style_test.go#TestLocaleStyle"
        status: pass
    human_judgment: false
  - id: D3
    description: "Main grid button ('✨ ' + settings_page_presence <= 20 runes), substituted toggle labels (<= 40 runes), and Conversations shortcut button (<= 40 runes) pass settingsLocaleLimits and TestSettingsLabelLengths across all 8 locales"
    requirement: PRES-03
    verification:
      - kind: unit
        ref: "xenobot/i18n/settings_locale_test.go#TestSettingsLabelLengths"
        status: pass
    human_judgment: false

duration: 4 min
completed: 2026-11-26
status: complete
---

# Phase 31 Plan 03: Presence Settings 8-Locale Translations & Length Guards Summary

**Added complete 8-locale UI copy for the private Presence Settings page (`pr`) and Conversations page (`p`) shortcut button, and locked every key into `settingsLocaleLimits` and `TestSettingsLabelLengths` (20-rune Main grid and 40-rune button caps).**

## Performance

- **Duration:** 4 min
- **Started:** 2026-11-26T21:59:24Z
- **Completed:** 2026-11-26T22:03:20Z
- **Tasks:** 3
- **Files modified:** 9

## Accomplishments
- Added all 11 Presence keys (`settings_page_presence`, `settings_state_presence_on`, `settings_pr_group_note`, `settings_pr_intro`, `settings_pr_consensus_line`, `settings_pr_threshold_line`, `settings_pr_retract_line`, `settings_pr_hint`, `settings_btn_pr_consensus`, `settings_btn_pr_retract`, `settings_btn_chat_presence`) across all 8 locale files (`en`, `uk`, `es`, `fr`, `hi`, `id`, `pt`, `ru`) with identical `{n}`, `{state}`, and `{count}` placeholders and compliant `locale-style.md` copy.
- Registered all 11 keys in `settingsLocaleLimits` in `xenobot/i18n/settings_locale_test.go` with tight English/other rune ceilings.
- Registered `{"✨", "settings_page_presence"}` in `TestSettingsLabelLengths` (enforcing `<= 20` runes across all 8 languages; max observed 13 runes in `ru`/`uk`) plus `settings_btn_pr_consensus`/`settings_btn_pr_retract` in `phase18Toggles` and `settings_btn_chat_presence` in `fullWidthLabels` (enforcing `<= 40` runes after `{state}` substitution).
- Verified `make check-translations` and `go test -tags testtools -race -count=1 ./xenobot/i18n/...` pass with zero errors.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add Presence page and Conversations shortcut keys to en, uk, es, and fr locales** - `633429fd` (feat)
2. **Task 2: Add matching Presence page and Conversations shortcut keys to hi, id, pt, and ru locales** - `45612586` (feat)
3. **Task 3: Register Presence keys in settingsLocaleLimits and TestSettingsLabelLengths** - `817af854` (test)

## Files Created/Modified
- `locales/en.yml`, `locales/uk.yml`, `locales/es.yml`, `locales/fr.yml` - Added 11 Presence Settings page and Conversations shortcut keys matching `locale-style.md` and placeholder parity.
- `locales/hi.yml`, `locales/id.yml`, `locales/pt.yml`, `locales/ru.yml` - Added matching 11 Presence keys with concise labels and identical placeholders.
- `xenobot/i18n/settings_locale_test.go` - Added all 11 keys to `settingsLocaleLimits`, added `{"✨", "settings_page_presence"}` to `TestSettingsLabelLengths` grid `pages`, and added the two toggle buttons and shortcut button to the 40-rune checks.

## Decisions Made
- Used existing per-locale compact state patterns for `settings_state_presence_on` (e.g., `uk`: `"увімк.: {n}"` matching `settings_state_del_on`/`settings_state_mreq_on`, `ru`: `"{n} вкл"` matching `settings_state_sw_on`) so the Main status block stays consistent across pages.
- Extended `TestSettingsLabelLengths` (`phase18Toggles` and `fullWidthLabels`) with `settings_btn_pr_consensus`, `settings_btn_pr_retract`, and `settings_btn_chat_presence` so substituted button lengths (`<= 40` runes even with Spanish `"❌ Desactivado"`) remain permanently guarded in CI.

## Deviations from Plan

None - plan executed exactly as written. (Minor note: the ceiling map in `xenobot/i18n/settings_locale_test.go` is named `settingsLocaleLimits` in the codebase rather than `settingsLocaleCeilings` as referenced in the plan prose.)

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Plan `31-05` (`settings_presence.go` and `settings_chat.go` shortcut row) has all 11 Presence keys verified and ready across all 8 locales with zero missing keys or length violations.
- Zero blockers.

---
*Phase: 31-presence-foundation-settings-page-and-zero-ai-social-reactions*
*Completed: 2026-11-26*

## Self-Check: PASSED
