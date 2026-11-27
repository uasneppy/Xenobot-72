---
phase: 34-room-aware-conversations-and-frictionless-catch-up
plan: 03
subsystem: i18n
tags: [locales, i18n, settings, brief, help, yaml]

# Dependency graph
requires: []
provides:
  - "8-locale strings for Presence Room & Catch-Up sub-page (`pc`), `settings_pr_room_line`, `settings_btn_pr_room`, and Owner-lock alerts (`settings_pc_need_owner_conv_log`, `settings_pc_need_owner_log`)"
  - "8-locale brief strings (`brief_empty`, `brief_header_short_pm`, `brief_header_long_pm`, `brief_note_pm`, `brief_pm_blocked`) and updated `brief_usage_short`, `brief_usage_long`, `chat_help_msg`, `presence_help_msg`"
  - "Rune ceiling, plain-text alert, button label length, and brief limit registrations in `xenobot/i18n/settings_locale_test.go` and `xenobot/i18n/chat_brief_locale_test.go`"
affects:
  - "34-06 (Reply-free /unrollshort and /unrolllong + PM brief delivery)"
  - "34-07 (Settings Presence Room & Catch-Up sub-page pc)"
  - "34-08 (Verification & docs generation)"

# Actuals (#2632)
actuals:
  tokens: 29140
  tasks: 2
  commits: 2
plan_head_before: b6076ba077c0923806fc444a795e06cabf3caf23
plan_head_after: 1d3e9e914a786bf86a82e935d5004866d417bcd0

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Use `{state}` placeholder on `settings_pr_room_line` and `settings_btn_pc_*` matching sibling Presence toggle/status lines"
    - "Keep `brief_usage_short`/`brief_usage_long` free of `&lt;`/`&gt;` entities (`<code>/unrollshort [N|me|2h] [pm]</code>`) to satisfy `TestChatBriefLocaleKeys`'s strict `<>&` check while keeping `<code>` markup valid"

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
    - xenobot/i18n/chat_brief_locale_test.go

key-decisions:
  - "Used `{state}` in `settings_pr_room_line` (`Room & catch-up: {state}`) so `renderSettingsPresence` can format either an On/Off word or active count summary cleanly."
  - "Avoided `&lt;`/`&gt;` inside `brief_usage_short` and `brief_usage_long` (`[N|me|2h] [pm]`) because `TestChatBriefLocaleKeys` forbids `&` after stripping `<b>`/`<i>`/`<code>` tags, while using `&lt;N|me|2h&gt;` in `chat_help_msg` per `locale-style.md`."
  - "Registered `settings_btn_pr_room` in `fullWidthLabels` and `settings_btn_pc_{ambient,vocative,pmbrief}` in `phase18Toggles` inside `TestSettingsLabelLengths` to enforce the `<= 40`-rune rendered button cap across all 8 locales."

patterns-established:
  - "All Phase 34 `settings_pc_*` and `brief_*_pm` keys enforce identical placeholders and tight `{en, other}` rune ceilings across all 8 locales."

requirements-completed:
  - CHAT-02
  - CHAT-03
  - CHAT-04
  - CHAT-05

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "All 8 locale files define Phase 34 Settings Room & Catch-Up (`pr`/`pc`), Owner-lock alert, and reply-free/PM brief keys with matching placeholders and style-guide compliance."
    requirement: CHAT-02
    verification:
      - kind: unit
        ref: "xenobot/i18n/locale_style_test.go#TestLocaleStyle"
        status: pass
      - kind: unit
        ref: "xenobot/i18n/locale_rewrite_guard_test.go#TestLocalePlaceholdersMatchOtherLanguages"
        status: pass
      - kind: other
        ref: "make check-translations"
        status: pass
    human_judgment: false
  - id: D2
    description: "`settings_locale_test.go` enforces ceilings, plain-text <=200-rune alerts, and <=40-rune button widths for all Phase 34 `settings_pr_room_*` and `settings_pc_*` keys."
    requirement: CHAT-03
    verification:
      - kind: unit
        ref: "xenobot/i18n/settings_locale_test.go#TestSettingsLocaleKeys"
        status: pass
      - kind: unit
        ref: "xenobot/i18n/settings_locale_test.go#TestSettingsLabelLengths"
        status: pass
    human_judgment: false
  - id: D3
    description: "`brief_usage_short`, `brief_usage_long`, `chat_help_msg`, `presence_help_msg`, and new `brief_*` PM/empty keys pass `chatBriefLocaleLimits`, `TestChatHelpFitsOneMessage`, and `TestHelpPagesKeepTheirCommands`."
    requirement: CHAT-04
    verification:
      - kind: unit
        ref: "xenobot/i18n/chat_brief_locale_test.go#TestChatBriefLocaleKeys"
        status: pass
      - kind: unit
        ref: "xenobot/i18n/chat_brief_locale_test.go#TestChatHelpFitsOneMessage"
        status: pass
      - kind: unit
        ref: "xenobot/i18n/locale_rewrite_guard_test.go#TestHelpPagesKeepTheirCommands"
        status: pass
    human_judgment: false
  - id: D4
    description: "`brief_note_pm`, `brief_pm_blocked`, `brief_header_short_pm`, `brief_header_long_pm`, and `settings_pc_pmbrief_line`/`settings_btn_pc_pmbrief`/`settings_pc_need_owner_log` ship with 8-locale parity and registered ceilings for silent `✍` PM briefs."
    requirement: CHAT-05
    verification:
      - kind: unit
        ref: "xenobot/i18n/chat_brief_locale_test.go#TestChatBriefLocaleKeys"
        status: pass
      - kind: unit
        ref: "xenobot/i18n/settings_locale_test.go#TestSettingsLocaleKeys"
        status: pass
    human_judgment: false

# Metrics
duration: 8min
completed: 2026-11-27
status: complete
---

# Phase 34 Plan 03: Phase 34 Locales & i18n Guard Tests Summary

**Added all Phase 34 Room & Catch-Up (`pc`), Owner-lock alert, reply-free `/unrollshort`/`/unrolllong`, and PM brief strings across all 8 locales with full ceiling, alert, style, and help-page test coverage.**

## Performance

- **Duration:** 8 min
- **Started:** 2026-11-27T07:28:29Z
- **Completed:** 2026-11-27T07:36:35Z
- **Tasks:** 2
- **Files modified:** 10

## Accomplishments
- Added `settings_pr_room_line`, `settings_btn_pr_room`, `settings_pc_*`, `settings_btn_pc_*`, `settings_pc_need_owner_conv_log`, and `settings_pc_need_owner_log` across all 8 locale files (`en`, `uk`, `es`, `fr`, `hi`, `id`, `pt`, `ru`).
- Added `brief_empty`, `brief_header_short_pm`, `brief_header_long_pm`, `brief_note_pm`, and `brief_pm_blocked`, and updated `brief_usage_short`, `brief_usage_long`, `chat_help_msg`, and `presence_help_msg` across all 8 locales to document reply-free `[N|me|2h] [pm]` arguments and Room & Catch-Up / `✍` PM brief settings.
- Registered all new Settings keys, plain-text alert keys, button label length checks, and brief keys in `xenobot/i18n/settings_locale_test.go` and `xenobot/i18n/chat_brief_locale_test.go`, passing `make check-translations` and `go test -tags testtools -race -count=1 ./xenobot/i18n/...`.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add Phase 34 Presence Room & Catch-Up and brief keys across all 8 locale files** - `33fa5bfe` (feat)
2. **Task 2: Register Phase 34 ceilings and alert keys in settings_locale_test.go and chat_brief_locale_test.go** - `1d3e9e91` (test)

## Files Created/Modified
- `locales/{en,uk,es,fr,hi,id,pt,ru}.yml` - Phase 34 `settings_pr_room_*`, `settings_pc_*`, `brief_*` keys and updated `brief_usage_*`, `chat_help_msg`, `presence_help_msg`.
- `xenobot/i18n/settings_locale_test.go` - Registered ceilings in `settingsLocaleLimits`, alerts in `settingsAlertKeys`, and button label length checks in `TestSettingsLabelLengths`.
- `xenobot/i18n/chat_brief_locale_test.go` - Registered `brief_empty`, `brief_header_short_pm`, `brief_header_long_pm`, `brief_note_pm`, and `brief_pm_blocked` in `chatBriefLocaleLimits`.

## Decisions Made
- Used `{state}` in `settings_pr_room_line` to match `settings_pr_consensus_line`, `settings_pr_retract_line`, `settings_pr_cat_line`, and `settings_pr_archive_line`.
- Used `[N|me|2h] [pm]` inside `<code>` in `brief_usage_short`/`brief_usage_long` so `TestChatBriefLocaleKeys` (which disallows `&` after stripping `<b>`/`<i>`/`<code>`) passes cleanly while `chat_help_msg` uses `&lt;N|me|2h&gt;` per `locale-style.md`.

## Deviations from Plan

None - plan executed as written (note: the ceiling map in `xenobot/i18n/settings_locale_test.go` is named `settingsLocaleLimits` in code rather than `settingsKeyCeilings`).

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Downstream plans `34-06` and `34-07` can consume all new `brief_*`, `settings_pr_room_*`, and `settings_pc_*` locale keys directly.

---
*Phase: 34-room-aware-conversations-and-frictionless-catch-up*
*Completed: 2026-11-27*

## Self-Check: PASSED
- All 10 modified files and `34-03-SUMMARY.md` verified on disk.
- Task commits `33fa5bfe` and `1d3e9e91` verified in git history.
- `make check-translations` and `go test -tags testtools -race -count=1 ./xenobot/i18n/...` passed after final edits.
