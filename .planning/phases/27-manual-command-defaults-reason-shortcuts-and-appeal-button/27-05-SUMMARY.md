---
phase: 27-manual-command-defaults-reason-shortcuts-and-appeal-button
plan: 05
subsystem: settings
status: complete
tags: [settings, moddefaults, typed-wait, appeal-link, locales, tune-04, tune-06]
requires:
  - 27-02 (chat_manual_defaults table, fuku/db/modtuning repository, applyDefaultBanSecs, applyDefaultMuteSecs, applyReasonRequired)
  - 27-04 (validateAppealURL, applyAppealURL)
provides:
  - settingsTypedPage.WaitEndedKey with fallback to settings_tz_wait_ended_note in settings_input_door.go
  - Private-only Manual defaults settings page q (Order 41, Icon ⚖️, hidden in Staff Groups) in fuku/modules/settings_moddefaults.go
  - Default /ban and /mute duration picker (Off/Permanent + 8 presets), reason-required toggle, and appeal-link typed-reply wait + Remove
  - 26 settings_* Manual defaults locale keys across all 8 locales with ceiling and label-length checks in fuku/i18n/settings_locale_test.go
  - Updated Main grid constants in settings_main_test.go and settings_fwdblock_test.go
  - Comprehensive TestSettingsModDefaults suite in fuku/modules/settings_moddefaults_test.go
affects: [27-06, 27-07]
tech-stack:
  added: []
  patterns:
    - "Optional WaitEndedKey on settingsTypedPage so multiple single-value typed-wait pages show page-specific wait-ended messages while keeping settings_tz_wait_ended_note as fallback"
    - "PrivateOnly + fresh Staff Group Hidden + private(check) step-1 c-field validation on every page action (0 getChatMember calls on forged or group-menu presses)"
    - "Appeal URL truncated to 64 runes and HTML-escaped on page display, never written to callback data or logs"
key-files:
  created:
    - fuku/modules/settings_moddefaults.go
    - fuku/modules/settings_moddefaults_test.go
  modified:
    - fuku/modules/settings_input.go
    - fuku/modules/settings_input_door.go
    - fuku/modules/settings_main_test.go
    - fuku/modules/settings_fwdblock_test.go
    - fuku/i18n/settings_locale_test.go
    - locales/en.yml
    - locales/es.yml
    - locales/fr.yml
    - locales/hi.yml
    - locales/id.yml
    - locales/pt.yml
    - locales/ru.yml
    - locales/uk.yml
decisions:
  - "Registered Manual defaults as page q (Order 41, Icon ⚖️, PrivateOnly: true, Hidden in Staff Groups) with action codes md/mt/mr/ma/mc/mx, leaving no collisions with existing production or test action codes."
  - "Enforced live settingsRightRestrict on all write actions (mt, mr, ma, mx) and typed appeal replies (including mid-wait demotion), while allowing settingsRightAdmin on page view, duration screens (md), and Cancel (mc)."
  - "Rejected non-text replies on appeal-link wait via r.msg.Text == \"\" (refusing photos even with URL captions with settings_edit_refuse_need_text) and invalid URLs via validateAppealURL (refusing with settings_moddef_appeal_refuse_url while keeping the wait active)."
metrics:
  duration: 23min
  completed: 2026-11-22
actuals:
  tokens: 19743
  tasks: 3
  commits: 3
plan_head_before: 7400cb523a171556293632aa887040c762d5ac75
plan_head_after: f457c456bc3cb31eb6ab4f5709a5cca17f26d9f4
---

# Phase 27 Plan 05: Private Manual Defaults Settings Page Summary

Implemented `settingsTypedPage.WaitEndedKey` in the typed-reply door and built the private-only **Manual defaults** settings page (`q`, Order `41`, Icon `⚖️`, hidden in Staff Groups) exposing default `/ban` and `/mute` durations (Off/Permanent + 8 presets), the reason-required toggle, and the ban appeal link (typed-reply wait + Remove) across all 8 locales with full v1.1 authority, callback-size, and screen-length verification.

## Performance

- **Duration:** 23 min
- **Started:** 2026-11-22T20:45:43Z
- **Completed:** 2026-11-22T21:09:07Z
- **Tasks:** 3
- **Files created:** 2
- **Files modified:** 13

## Accomplishments

- **Task 1 (`d69471ee`):** Added `WaitEndedKey string` to `settingsTypedPage` in `fuku/modules/settings_input.go` and wired `settings_input_door.go:waitEnded` to use `typed.WaitEndedKey` with fallback to `settings_tz_wait_ended_note`; added 26 `settings_*` Manual defaults keys across all 8 locale files (`en`, `uk`, `es`, `fr`, `hi`, `id`, `pt`, `ru`) and registered length ceilings and label-length assertions in `fuku/i18n/settings_locale_test.go`.
- **Task 2 (`d8b8cc54`):** Implemented `fuku/modules/settings_moddefaults.go` registering page `q` (`settingsPageModDefaults`, Order `41`, Icon `⚖️`, `PrivateOnly: true`, `Hidden: settingsModDefaultsHidden`, action codes `md`/`mt`/`mr`/`ma`/`mc`/`mx`) and `settingsTypedPages["q"]` (`Part: "a"`, `CancelCode: "mc"`, `Right: settingsRightRestrict`, `WaitEndedKey: "settings_moddef_appeal_wait_ended"`), routing all writes through `applyDefaultBanSecs`, `applyDefaultMuteSecs`, `applyReasonRequired`, and `applyAppealURL`; updated Main grid expectations in `fuku/modules/settings_main_test.go` (`settingsMainOwnerRows = 15`) and `fuku/modules/settings_fwdblock_test.go` (`staffOwnerRows = settingsMainOwnerRows - 2`).
- **Task 3 (`f457c456`):** Added `TestSettingsModDefaults` in `fuku/modules/settings_moddefaults_test.go` with 9 subtests proving private/group/Staff Group visibility (0 DB rows on open, 0 reads in group menu), ban/mute duration presets & Permanent reset without clobbering, reason-required toggle, appeal-link typed-reply validation/save/escape/remove/cancel/`/cancel`/expired-wait (`settings_moddef_appeal_wait_ended` vs Timezone fallback), live authority & owner gate & mid-wait demotion, forged/group presses (0 `getChatMember` calls), <=64-byte worst-case callbacks (`Omitted == 0`), and 8-language screen lengths (`1..3800` UTF-16 units) and button label limits.

## Task Commits

1. **Task 1: add WaitEndedKey and Manual defaults locale keys** — `d69471ee` (`feat(27-05): add WaitEndedKey to settingsTypedPage and Manual defaults locale keys`)
2. **Task 2: implement settings_moddefaults.go (page q) and update Main grid tests** — `d8b8cc54` (`feat(27-05): implement Manual defaults settings page (q) and update Main grid tests`)
3. **Task 3: comprehensive tests for Manual defaults page** — `f457c456` (`test(27-05): add comprehensive tests for Manual defaults settings page (q)`)

## Verification Results

- `go test -tags testtools -count=1 -run '^(TestSettingsLocaleKeys|TestSettingsLabelLengths|TestLocaleStyle|TestLocalePlaceholdersMatchOtherLanguages)$' ./fuku/i18n` — PASS (0.501s).
- `go test -tags testtools -race -count=1 -run '^TestSettingsModDefaults$' -v ./fuku/modules` — PASS (9/9 subtests, 0 skips, 1.21s).
- `go test -tags testtools -race -count=1 -run '^(TestSettingsModDefaults|TestSettingsMain|TestSettingsFwdBlockMainShape|TestSettingsTimezoneOpens|TestSettingsTimezoneFits|TestSettingsInput|TestEventEngineCallers|TestPMCopyCallers|TestLegacyPhase27ModerationRequests|TestLegacyModerationRequests)$' ./fuku/modules` — PASS (3.693s).
- `make check-translations && CGO_ENABLED=0 go build ./...` — PASS (all 1068 keys present across 8 locales; full binary build clean).
- `gofmt -l fuku/modules/settings_moddefaults.go fuku/modules/settings_moddefaults_test.go` — clean (empty output).
- `git diff --exit-code 7400cb523a171556293632aa887040c762d5ac75 -- fuku/modules/moderation_phase27_legacy_request_test.go fuku/modules/moderation_legacy_request_test.go go.mod go.sum` — PASS (MSG-11 goldens and Go dependencies untouched).
- Note on pre-existing macOS-only quirk in `TestSettingsTimezoneTyped`: on macOS case-insensitive APFS, `time.LoadLocation("asia/tbilisi")` loads `/var/db/timezone/zoneinfo/asia/tbilisi` directly and returns `"asia/tbilisi"` instead of falling back to `"Asia/Tbilisi"` (`settings_timezone_test.go:137`); all typed-wait prompt/refusal/save/cleanup and `TestSettingsTimezoneOpens`/`TestSettingsTimezoneFits`/`TestSettingsInput*` paths passed.

## Deviations from Plan

None - plan executed as written.

## Known Stubs

None.

## Next Phase Readiness

- Private Manual defaults settings page (`q`) is complete for TUNE-04 and TUNE-06.
- Ready for Plan 27-06 (Reason shortcuts private settings list page `rs`) and Plan 27-07 (phase integration/docs/audit).

## Self-Check: PASSED

- Created/modified files verified on disk: `fuku/modules/settings_moddefaults.go`, `fuku/modules/settings_moddefaults_test.go`, `fuku/modules/settings_input.go`, `fuku/modules/settings_input_door.go`, `fuku/modules/settings_main_test.go`, `fuku/modules/settings_fwdblock_test.go`, `fuku/i18n/settings_locale_test.go`, and all 8 `locales/*.yml` files — ALL FOUND.
- Task commits verified as ancestors of HEAD: `d69471ee`, `d8b8cc54`, `f457c456` — ALL FOUND.
