---
phase: 35-interactive-group-toys
plan: 03
subsystem: i18n
tags: [i18n, locales, settings, toys, referendum, inspect, tests]

# Dependency graph
requires:
  - phase: 34-room-aware-conversations-and-frictionless-catch-up
    provides: Settings pc sub-page keys (settings_pc_*) and chat_help_msg structure
provides:
  - Phase 35 Settings pc toy keys (settings_pc_referendum_line, settings_pc_inspect_line, settings_btn_pc_referendum, settings_btn_pc_inspect) and updated intro/hint across all 8 locales
  - 15 referendum_* and inspect_* locale keys across all 8 locales (en, uk, ru, es, fr, hi, id, pt)
  - Updated chat_help_msg naming /referendum and /inspect under Commands for everyone in all 8 locales
  - Ceiling, button-width, HTML-balance, placeholder-parity, and literal-scan i18n guards (settings_locale_test.go, chat_toys_locale_test.go)
affects:
  - 35-04 (Settings pc UI wiring)
  - 35-05 (chat_polls.go and chat_inspect.go handlers)

# Actuals (#2632)
actuals:
  tokens: 21149
  tasks: 2
  commits: 2
plan_head_before: 8cbfcad37a31222e81f51ac2a8d44b3fc85a2982
plan_head_after: 8e24b897b4361d74d9a9e53cecd306c0902fe054

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "chatToysLocaleLimits + TestChatToysLocaleKeys + TestChatToysKeyLiteralsExist mirror chat_brief_locale_test.go"
    - "Settings pc toggle buttons enforced via phase18Toggles (<= 40 rendered runes with ✅ On / ❌ Off)"

key-files:
  created:
    - xenobot/i18n/chat_toys_locale_test.go
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
  - "Targeted the actual 8 shipped repo locales (uk, en, es, fr, hi, id, pt, ru per staffLocales / TestLocalesAreTheEight) rather than de/it listed in plan frontmatter"
  - "Named the usage hint key referendum_need_context (matching Task 1 action spec) and skipped unused referendum_usage key"

patterns-established:
  - "TestChatToysKeyLiteralsExist checks all keys in chatToysLocaleLimits immediately and scans ../modules/*chat*.go dynamically so Wave 1 passes before Wave 3 handlers land"

requirements-completed:
  - TOYS-01
  - TOYS-02
  - TOYS-03

coverage:
  - id: D1
    description: "All 8 locale files contain Settings pc toy toggles, updated settings_pc_intro/hint, 15 referendum_*/inspect_* strings, and updated chat_help_msg naming /referendum and /inspect"
    requirement: TOYS-01
    verification:
      - kind: unit
        ref: "make check-translations"
        status: pass
      - kind: unit
        ref: "go test -tags testtools -race -count=1 ./xenobot/i18n/...#TestLocaleStyle"
        status: pass
      - kind: unit
        ref: "go test -tags testtools -race -count=1 ./xenobot/i18n/...#TestChatHelpFitsOneMessage"
        status: pass
    human_judgment: false
  - id: D2
    description: "Settings pc button-width and ceiling guards updated in settings_locale_test.go (settingsLocaleLimits and phase18Toggles)"
    requirement: TOYS-02
    verification:
      - kind: unit
        ref: "go test -tags testtools -race -count=1 ./xenobot/i18n/...#TestSettingsLocaleKeys"
        status: pass
      - kind: unit
        ref: "go test -tags testtools -race -count=1 ./xenobot/i18n/...#TestSettingsLabelLengths"
        status: pass
    human_judgment: false
  - id: D3
    description: "chat_toys_locale_test.go enforces rune ceilings, allowed HTML tag balance, zero printf verbs, placeholder parity, literal usage, and chat_help_msg toy coverage across all 8 locales"
    requirement: TOYS-03
    verification:
      - kind: unit
        ref: "go test -tags testtools -race -count=1 ./xenobot/i18n/...#TestChatToysLocaleKeys"
        status: pass
      - kind: unit
        ref: "go test -tags testtools -race -count=1 ./xenobot/i18n/...#TestChatToysKeyLiteralsExist"
        status: pass
      - kind: unit
        ref: "go test -tags testtools -race -count=1 ./xenobot/i18n/...#TestChatHelpIncludesToys"
        status: pass
    human_judgment: false

# Metrics
duration: 4 min
completed: 2026-11-27
status: complete
---

# Phase 35 Plan 03: Locale Strings & i18n Guards Summary

**Phase 35 Settings `pc` toy controls, `/referendum`, tie-breaker, `/inspect`, and `chat_help_msg` translations across all 8 repository locales with strict rune-ceiling, button-width, HTML-balance, and placeholder-parity tests**

## Performance

- **Duration:** 4 min
- **Started:** 2026-11-27T12:52:45Z
- **Completed:** 2026-11-27T12:57:07Z
- **Tasks:** 2
- **Files modified:** 10

## Accomplishments
- Added `settings_pc_referendum_line`, `settings_pc_inspect_line`, `settings_btn_pc_referendum`, `settings_btn_pc_inspect`, and updated `settings_pc_intro` and `settings_pc_hint` across all 8 locales (`en`, `uk`, `ru`, `es`, `fr`, `hi`, `id`, `pt`).
- Added 8 `referendum_*` keys (command errors, `referendum_need_context`, tie-breaker header, and deadpan fallback) and 7 `inspect_*` keys (usage, errors, and inspection header) across all 8 locales, plus `/referendum` and `/inspect` bullets in `chat_help_msg`.
- Registered the new Settings `pc` keys in `settingsLocaleLimits` and `phase18Toggles` inside `xenobot/i18n/settings_locale_test.go` and created `xenobot/i18n/chat_toys_locale_test.go`.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add Phase 35 Settings pc, /referendum, tie-breaker, /inspect, and chat_help_msg translations across all 8 locales** - `08c805dc` (feat)
2. **Task 2: Register Phase 35 ceilings in settings_locale_test.go and add chat_toys_locale_test.go** - `8e24b897` (test)

## Files Created/Modified
- `locales/en.yml`, `locales/uk.yml`, `locales/ru.yml`, `locales/es.yml`, `locales/fr.yml`, `locales/hi.yml`, `locales/id.yml`, `locales/pt.yml` - Phase 35 Settings `pc`, `referendum_*`, `inspect_*`, and `chat_help_msg` translations.
- `xenobot/i18n/settings_locale_test.go` - Added `settings_pc_referendum_line`, `settings_pc_inspect_line`, `settings_btn_pc_referendum`, `settings_btn_pc_inspect` to `settingsLocaleLimits` and the two button keys to `phase18Toggles`.
- `xenobot/i18n/chat_toys_locale_test.go` - Added `TestChatToysLocaleKeys`, `TestChatToysKeyLiteralsExist`, and `TestChatHelpIncludesToys`.

## Decisions Made
- Updated the actual 8 shipped locales in `locales/` (`uk`, `en`, `es`, `fr`, `hi`, `id`, `pt`, `ru`, enforced by `TestLocalesAreTheEight`) rather than `de`/`it` referenced in the plan header.
- Kept `referendum_need_context` as the usage hint key per Task 1's specification (no extra `referendum_usage` key).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Updated actual repository locale set (`hi.yml`, `id.yml`) instead of non-existent `de.yml` and `it.yml`**
- **Found during:** Task 1 (locale file inspection)
- **Issue:** `35-03-PLAN.md` frontmatter and Task 1 `<files>` listed `locales/de.yml` and `locales/it.yml`, which do not exist in the repository; the 8 locales enforced by `TestLocalesAreTheEight` and `staffLocales` are `uk`, `en`, `es`, `fr`, `hi`, `id`, `pt`, and `ru`.
- **Fix:** Added all Phase 35 translations to `locales/hi.yml` and `locales/id.yml` alongside `en.yml`, `uk.yml`, `ru.yml`, `es.yml`, `fr.yml`, and `pt.yml`.
- **Files modified:** `locales/hi.yml`, `locales/id.yml`
- **Verification:** `make check-translations` and `go test -tags testtools -race -count=1 ./xenobot/i18n/...` passed across all 8 locales.
- **Committed in:** `08c805dc` (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Essential so all 8 actual repository locales stay in parity and pass `make check-translations` and `xenobot/i18n`.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All 8 locales and `xenobot/i18n` guards are ready for Wave 2/3 Go implementations (`settings_presence.go`, `chat_polls.go`, `chat_inspect.go`).
- `make check-translations` and `go test -tags testtools -race -count=1 ./xenobot/i18n/...` both pass under `-race`.

## Self-Check: PASSED
- Verified all 10 created/modified files exist on disk.
- Verified commits `08c805dc` and `8e24b897` are ancestors of `HEAD`.

---
*Phase: 35-interactive-group-toys*
*Completed: 2026-11-27*
