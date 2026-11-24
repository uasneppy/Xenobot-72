---
phase: 27-manual-command-defaults-reason-shortcuts-and-appeal-button
plan: 06
subsystem: settings
status: complete
tags: [settings, list-engine, reason-shortcuts, two-step-input, locales, tune-05]
requires:
  - 27-03 (normaliseReasonShortcutWord, checkReasonShortcutWord, checkReasonShortcutText, applyReasonShortcutAdd, applyReasonShortcutRemoveByID)
  - 27-05 (Manual defaults settings page q and updated Main grid baseline)
provides:
  - Plain-text Step 2 hook on settingsListTwoStep (CheckPlain, ApplyPlain) in fuku/modules/settings_list_input.go with static parts {rs, w, 1, r} and {rs, r, 2}
  - Private-only Reason shortcuts list page rs (Order 45, Icon 🏷, hidden in Staff Groups, Right settingsRightRestrict, action codes sg/sx/sy/sa/sc) in fuku/modules/settings_reasons.go
  - 19 settings_* Reason shortcuts locale keys across all 8 locales with ceilings and label-length guards in fuku/i18n/settings_locale_test.go and fuku/modules/settings_list_locale_test.go
  - Updated Main grid shape (15 owner rows, 12 full pairs) in fuku/modules/settings_main_test.go and Phase 19 page/code/part tables in fuku/modules/settings_phase19_test.go
  - Comprehensive TestSettingsReasons suite in fuku/modules/settings_reasons_test.go
affects: [27-07]
tech-stack:
  added: []
  patterns:
    - "Optional plain-text Step 2 mode (CheckPlain + ApplyPlain) on settingsListTwoStep sharing the exact StepMsgID redelivery guard, fresh listStepCheck before claim, atomic claim/finish/release, and sendSavedList flow while leaving media/button pages (Filters, Notes) untouched"
    - "settingsReasonsEntryOf populates settingsListEntry{ID, Text: row.Word, Detail: row.Reason} so the list engine renders escaped 'word → reason' lines and never places user text into callback data or buttons"
    - "PrivateOnly + fresh Staff Group Hidden + settingsRightRestrict on Add/Ask/Yes and both typed steps"
key-files:
  created:
    - fuku/modules/settings_reasons.go
    - fuku/modules/settings_reasons_test.go
  modified:
    - fuku/modules/settings_list_input.go
    - fuku/modules/settings_main_test.go
    - fuku/modules/settings_phase19_test.go
    - fuku/modules/settings_list_locale_test.go
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
  - "Added CheckPlain and ApplyPlain to settingsListTwoStep and skipped appending settings_entries_wait_content_limits in renderSettingsListPrompt only when ApplyPlain != nil, keeping Filters and Notes byte-identical."
  - "Registered page rs (Order 45, Icon 🏷) with action codes sg/sx/sy/sa/sc and parts w->r, pairing rs|b on Main so the owner grid has 12 full pairs while remaining 15 rows total and Staff Group Main remains unchanged."
  - "Updated settings_phase19_test.go registered pages list (adding q and rs), settingsPhase19ListPages (adding settingsPageReasons), and settingsPhase19Codes (adding sg/sx/sy/sa/sc) so Phase 19 structural code/part guards cover Reason shortcuts too."
metrics:
  duration: 22min
  completed: 2026-11-23
actuals:
  tokens: 20607
  tasks: 3
  commits: 3
plan_head_before: e4a46e9b8fcbbe59cecf6da9328c0d0d0a5a5fb6
plan_head_after: f6ff79b78d3341f501b33eda91e31aa3f5749fdb
---

# Phase 27 Plan 06: Private Reason Shortcuts List Page (`rs`) Summary

Extended `settingsListTwoStep` with an optional plain-text Step 2 mode (`CheckPlain` + `ApplyPlain`) and built the private-only **Reason shortcuts** list page (`rs`, Order `45`, Icon `🏷`, hidden in Staff Groups, `settingsRightRestrict`, action codes `sg`/`sx`/`sy`/`sa`/`sc`, parts `w`→`r`) on the v1.1 list engine across all 8 languages, with full browse/paging/escaping, two-step add, redelivery guard, two-tap delete, foreign-ID refusal, live authority, 64-byte callback, and 8-language screen-length test coverage.

## Performance

- **Duration:** 22 min (includes initial sandbox loopback diagnostic)
- **Started:** 2026-11-23T03:02:22Z
- **Completed:** 2026-11-23T03:23:58Z
- **Tasks:** 3
- **Files created:** 2
- **Files modified:** 13

## Accomplishments

- **Task 1 (`3663fa75`):** Extended `settingsListTwoStep` in `fuku/modules/settings_list_input.go` with `CheckPlain` and `ApplyPlain`, skipped `settings_entries_wait_content_limits` on plain-text Step 2 prompts, added the plain-text branch in `listStepTwo` sharing the exact `StepMsgID` redelivery guard, fresh `listStepCheck`, and atomic `claimWait`/`finishWait`/`releaseWait`/`sendSavedList` flow, and registered static parts `{rs, w, 1, r}` and `{rs, r, 2}`; added 19 `settings_*` Reason shortcuts locale keys across all 8 `locales/*.yml` files with ceiling and label-length guards in `fuku/i18n/settings_locale_test.go`.
- **Task 2 (`55567f79`):** Implemented `fuku/modules/settings_reasons.go` registering page `rs` (`settingsPageReasons`, Order `45`, Icon `🏷`, `PrivateOnly: true`, `Hidden: settingsReasonsHidden`, `Right: settingsRightRestrict`, `CountMax: 50`, codes `sg`/`sx`/`sy`/`sa`/`sc`), routing all reads and writes through `modtuning.*Fresh`, `applyReasonShortcutAdd`, and `applyReasonShortcutRemoveByID`; updated Main grid shape and title keys in `fuku/modules/settings_main_test.go`, Phase 19 page/code/part tables in `fuku/modules/settings_phase19_test.go`, and `settingsListExtraKeys` in `fuku/modules/settings_list_locale_test.go`.
- **Task 3 (`f6ff79b7`):** Added `TestSettingsReasons` in `fuku/modules/settings_reasons_test.go` with 7 top-level subtests (plus 8 per-language subtests) proving empty state (0 DB rows on open), browsing/pagination/HTML escaping, Main state in private/group (0 DB reads in group), two-step plain-text add without `settings_entries_wait_content_limits`, step-1 & step-2 refusals, `StepMsgID` redelivery guard, raced duplicate non-overwrite, cap-50 `AddGate`, Cancel & `/cancel`, expired wait markers for both parts (`w` and `r`), two-tap delete + cache invalidation + foreign-chat ID refusal + stale/expired Yes, live `settingsRightRestrict` & mid-wait demotion & owner gate & group-menu/forged step-1 refusal (0 `getChatMember` calls) & Staff Group hiding, worst-case 17-digit ID `<= 64`-byte callbacks (`Omitted == 0`), and `1..3800` UTF-16 screen lengths across all 8 locales.

## Task Commits

1. **Task 1: plain-text Step 2 hook and 8-language Reason shortcuts locales** — `3663fa75` (`feat(27-06): extend settingsListTwoStep for plain-text Step 2 and add Reason shortcuts locales`)
2. **Task 2: Reason shortcuts list page (`rs`) and Main/Phase 19/list-locale guard updates** — `55567f79` (`feat(27-06): implement Reason shortcuts list page (rs) and update Main/Phase 19 guards`)
3. **Task 3: comprehensive `TestSettingsReasons` suite** — `f6ff79b7` (`test(27-06): add comprehensive TestSettingsReasons suite for Reason shortcuts page`)

## Verification Results

- `go test -tags testtools -count=1 -run '^(TestSettingsLocaleKeys|TestSettingsLabelLengths|TestLocaleStyle|TestLocalePlaceholdersMatchOtherLanguages)$' ./fuku/i18n` — PASS (0.495s).
- `go test -tags testtools -race -count=1 -run '^TestSettingsReasons$' -v ./fuku/modules` — PASS (7/7 top-level subtests + 8/8 language subtests, 0 skips, 1.19s).
- `go test -tags testtools -race -count=1 -run '^(TestSettingsReasons|TestSettingsListKeysExist|TestSettingsMain|TestSettingsFwdBlockMainShape|TestSettingsPhase19|TestSettingsFilters|TestSettingsNotes|TestSettingsModDefaults|TestLegacyPhase27ModerationRequests|TestLegacyModerationRequests)$' ./fuku/modules` — PASS (3.029s).
- `make check-translations && CGO_ENABLED=0 go build ./...` — PASS (1068 keys present across all 8 locales; full binary build clean).
- `gofmt -l fuku/modules/settings_list_input.go fuku/modules/settings_reasons.go fuku/modules/settings_reasons_test.go` — clean (empty).
- `git diff --exit-code e4a46e9b8fcbbe59cecf6da9328c0d0d0a5a5fb6 -- fuku/modules/moderation_phase27_legacy_request_test.go fuku/modules/moderation_legacy_request_test.go go.mod go.sum` — PASS (MSG-11 goldens and Go dependencies untouched).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Updated `TestSettingsPhase19Codes` registered pages and list-code tables in `fuku/modules/settings_phase19_test.go`**
- **Found during:** Task 2 verification
- **Issue:** `TestSettingsPhase19Codes` checks the exact sorted set of all registered page keys (`want` was missing both `"q"` from 27-05 and `"rs"` from 27-06) and verifies all list-engine pages/codes.
- **Fix:** Added `"q"` and `"rs"` to `want` in `TestSettingsPhase19Codes`, added `settingsPageReasons` to `settingsPhase19ListPages`, and added `sg`/`sx`/`sy`/`sa`/`sc` to `settingsPhase19Codes` (matching `settingsPageReactions`).
- **Files modified:** `fuku/modules/settings_phase19_test.go`
- **Commit:** `55567f79`

## Known Stubs

None.

## Next Phase Readiness

- TUNE-05 private Reason shortcuts list page (`rs`) is complete and verified end-to-end across all 8 languages.
- Ready for Plan 27-07 (phase integration, documentation, and verification).

## Self-Check: PASSED

- Created/modified files verified on disk: `fuku/modules/settings_list_input.go`, `fuku/modules/settings_reasons.go`, `fuku/modules/settings_reasons_test.go`, `fuku/modules/settings_main_test.go`, `fuku/modules/settings_phase19_test.go`, `fuku/modules/settings_list_locale_test.go`, `fuku/i18n/settings_locale_test.go`, and all 8 `locales/*.yml` files — ALL FOUND.
- Task commits verified as ancestors of HEAD: `3663fa75`, `55567f79`, `f6ff79b7` — ALL FOUND.
