---
phase: 30-backup-page-and-custom-messages-in-backups
plan: 04
subsystem: settings-backup
tags: [backup, settings, private-only, bitmask, redis, lua, typed-reply, i18n, msg-10, bkp-01, bkp-02, bkp-03]

requires:
  - phase: 30-backup-page-and-custom-messages-in-backups
    provides: Shared backup apply helpers (applyBackupExport, downloadAndParseBackupFile, applyBackupImport, formatBackupImportSuccess, applyBackupReset, formatBackupResetSuccess) and cross-replica Redis token helpers in fuku/modules/backup.go (Plan 30-03, commit 0cb8ba24)
  - phase: 29-plain-tone-and-copy-settings
    provides: Private-only page pattern, step-1 private(...) validators, settingsCopyDefaultStaffLookup, settingsCopyYesValid, and Lua GET+DEL token consumption in fuku/modules/settings_copy.go
  - phase: 27-manual-command-defaults-reason-shortcuts-and-appeal-button
    provides: settingsTypedPages single-value wait door pattern with WaitEndedKey in fuku/modules/settings_moddefaults.go and fuku/modules/settings_input_door.go
provides:
  - Private-only Backup settings page ba (Order 83, Icon 💾, Hidden in Staff Groups, static zero-I/O MainState) with 7 private-gated action codes (bv, be, bi, bl, bj, br, bu) in fuku/modules/settings_backup.go
  - Stateless 20-module canonical lowercase hex bitmask selector (m=0..fffff) covering all 20 backup.AllExportableModules() entries including custom messages
  - Private .json document wait registered on settingsTypedPages[settingsPageBackup] (part i, settingsRightOwnerBotAdmin) leading into a cross-replica two-tap Import confirmation card
  - 8-locale Backup page UI strings (18 settings_* keys + 5 missing backup_module_* keys + tightened verbose backup_module_* labels <= 17 runes) verified by fuku/i18n and make check-translations
  - Updated Main, Staff Group, and Phase 18/19/29 grid and page-registry tests for 30 registered settings pages
  - Comprehensive 8-subtest TestSettingsBackupPage suite in fuku/modules/settings_backup_test.go running in-process under -race with zero localhost TCP listeners
affects: [30-05]

tech-stack:
  added: []
  patterns:
    - "Stateless 20-bit canonical lowercase hex bitmask (m=0..fffff) for Backup module selection with zero DB/Redis state per toggle"
    - "Private document upload via settingsTypedPages[settingsPageBackup] (part i, settingsRightOwnerBotAdmin) keeping wait alive on invalid/non-document inputs and handing off to a one-time Redis confirm card (bj)"
    - "Peek-and-bind verification (UserID + ChatID) before atomic Lua GET+DEL consumption on Import Yes (bj) and Reset Yes (bu), sharing AcquireExport/AcquireImport/AcquireReset with /export, /import, and /reset"

key-files:
  created:
    - fuku/modules/settings_backup.go
    - fuku/modules/settings_backup_test.go
  modified:
    - locales/en.yml
    - locales/es.yml
    - locales/fr.yml
    - locales/hi.yml
    - locales/id.yml
    - locales/pt.yml
    - locales/ru.yml
    - locales/uk.yml
    - fuku/i18n/settings_locale_test.go
    - fuku/modules/settings_main_test.go
    - fuku/modules/settings_fwdblock_test.go
    - fuku/modules/settings_phase18_test.go
    - fuku/modules/settings_phase19_test.go
    - fuku/modules/settings_phase29_test.go

key-decisions:
  - "Tightened verbose pre-existing backup_module_* translations in es, fr, id, pt, ru, and uk alongside adding the 5 missing module keys so every 20-module cell renders with ✅/⬜ within the 20-rune 2-column button ceiling in TestSettingsLabelLengths across all 8 locales."
  - "Formatted backup_import_failed and backup_reset_failed page notices via settingsBackupFormatFailNotice to replace %s with common_settings_save_failed in real locales while keeping marker-locale strings intact and never leaking raw internal error strings."
  - "Updated settings_phase18_test.go (Staff Group state-line count len(settingsMainTitleKeys)-7 and 30-page registry list) alongside settings_main_test.go, settings_fwdblock_test.go, settings_phase19_test.go, and settings_phase29_test.go so all phase page-registry guards agree on the 30 registered pages."
  - "Added in-process runtime assertions in TestSettingsBackupPage/open_private_and_group_note_and_staff_hidden covering settingsMainShapeGroup, settingsMainShapePrivate, settingsMainOwnerRows (17), Staff Group rows (settingsMainOwnerRows-4 = 13), and Staff Group state lines (len(settingsMainTitleKeys)-7 = 22) over phase28InProcessMiniredis."

patterns-established:
  - "Private-only Backup page ba (Order 83, actions bv/be/bi/bl/bj/br/bu) shares applyBackupExport, downloadAndParseBackupFile, applyBackupImport, applyBackupReset, and BackupRateLimiter with /export, /import, and /reset."
  - "All 20 backup_module_* keys are guarded in TestSettingsLabelLengths across all 8 locales."

requirements-completed: [MSG-10, BKP-01, BKP-02, BKP-03]

status: complete
plan_head_before: 4a67c76372f10cb9aa763cf4606132f1bae2f190
plan_head_after: d8880d38428d157efb5bf11f3925230c5534649d
commits: 3

actuals:
  tokens: 30298
  tasks: 3
  commits: 3

duration: 19min
completed: 2026-11-23
---

# Phase 30 Plan 04: Private Backup Settings Page (ba), 8-Locale UI, and Tests Summary

**Implemented private-only Backup settings page `ba` (Order `83`, Icon `💾`, actions `bv`/`be`/`bi`/`bl`/`bj`/`br`/`bu`) with stateless 20-module bitmask selection, private `.json` document wait via `settingsTypedPages[settingsPageBackup]`, cross-replica Redis two-tap Import & Reset confirmation cards, 8-locale UI coverage, 30-page Main/Staff/Phase grid updates, and 8-subtest `TestSettingsBackupPage` suite verified under `-race`**

## Performance

- **Duration:** 19 min
- **Started:** 2026-11-23T21:10:09Z
- **Completed:** 2026-11-23T21:28:40Z
- **Tasks:** 3
- **Files modified:** 16 (2 created, 14 modified)

## Accomplishments

- **Task 1 (`0db9f55a`):** Added the 5 missing `backup_module_*` labels (`antiraid`, `approvals`, `federations`, `logchannels`, `reactions`) and tightened overly long non-English `backup_module_*` labels so all 20 module cells fit 2-column buttons (`<= 20` runes with `✅ `/`⬜ `) in every language; added all 18 `settings_page_backup`, `settings_state_backup`, `settings_bkp_*`, and `settings_btn_bkp_*` keys across `en`, `es`, `fr`, `hi`, `id`, `pt`, `ru`, and `uk`; registered all new limits, alerts, page cell, action buttons, and 20 module label checks in `fuku/i18n/settings_locale_test.go`.
- **Task 2 (`a6be0aa5`):** Implemented `fuku/modules/settings_backup.go` registering private-only page `settingsPageBackup = "ba"` at `Order: 83` (`Icon: "💾"`, `Hidden: settingsBackupHidden`, static zero-I/O `MainState`), step-1 `private(...)` validators on all 7 actions (`bv`, `be`, `bi`, `bl`, `bj`, `br`, `bu`), stateless 20-bit canonical lowercase hex bitmask encoding/parsing (`0`..`fffff`), private `.json` document wait routing on `settingsTypedPages[settingsPageBackup]`, peek-and-bind verification before atomic Lua `GET+DEL` consumption on `bj` and `bu`, shared apply helpers and `BackupRateLimiter` parity with `/export`, `/import`, and `/reset`, and IDs-only logging; updated `settings_main_test.go` (`settingsMainOwnerRows = 17`, `ba|d` row, `"settings_page_backup"`), `settings_fwdblock_test.go` (`staffOwnerRows = settingsMainOwnerRows - 4`), `settings_phase18_test.go` (Staff Group line formula `len(settingsMainTitleKeys)-7` and 30-page registry list), `settings_phase19_test.go`, and `settings_phase29_test.go`.
- **Task 3 (`d8880d38`):** Added `fuku/modules/settings_backup_test.go` with all 8 subtests of `TestSettingsBackupPage` backed by `phase28InProcessMiniredis(t)` and `backupRoundTripFunc` (zero localhost TCP listeners): private/group-note/Staff-Group hiding + live Main/Staff grid shape checks, 20-module bitmask toggling + `m=0` refusal, Export rights/rate-limit/private delivery/command parity, Import owner gate/private document wait/refusals/cancel/two-tap card/skipped & log-channel report/old export compatibility/command parity, Reset owner gate/two-tap card/rate limit/command parity, replay/TTL expiry/wrong-user/wrong-chat/demoted-owner/nil-Redis fail-closed checks, worst-case `math.MinInt64` 64-byte callback checks, and all-8-language screen length checks (0 omitted buttons, `1..3800` UTF-16 units).

## Task Commits

Each task was committed atomically:

1. **Task 1: Add Backup page locale keys across all 8 languages and update settings_locale_test.go** - `0db9f55a` (`feat(30-04): add Backup settings page locale keys across all 8 languages`)
2. **Task 2: Implement private Backup settings page (ba) in settings_backup.go and update grid tests** - `a6be0aa5` (`feat(30-04): implement private Backup settings page ba and update grid tests`)
3. **Task 3: Add comprehensive TestSettingsBackupPage suite** - `d8880d38` (`test(30-04): add comprehensive TestSettingsBackupPage suite`)

## Files Created/Modified

- `locales/{en,es,fr,hi,id,pt,ru,uk}.yml` — Added 18 Backup settings page keys and 5 missing `backup_module_*` keys, and tightened verbose non-English `backup_module_*` labels to `<= 17` runes.
- `fuku/i18n/settings_locale_test.go` — Registered all 18 Backup page keys in `settingsLocaleLimits`, 2 alerts in `settingsAlertKeys`, `{"💾", "settings_page_backup"}` in `pages`, 7 buttons in `editorButtons`, and all 20 `backup_module_*` button width checks in `TestSettingsLabelLengths`.
- `fuku/modules/settings_backup.go` — Implemented private-only Backup page `ba` (`Order: 83`), 7 action handlers (`bv`, `be`, `bi`, `bl`, `bj`, `br`, `bu`), `settingsTypedPages[settingsPageBackup]` private `.json` document wait handler, and pure renderers.
- `fuku/modules/settings_main_test.go` — Updated `settingsMainShapeGroup`, `settingsMainShapePrivate`, `settingsMainOwnerRows = 17`, and `settingsMainTitleKeys` for `ba` at Order `83`.
- `fuku/modules/settings_fwdblock_test.go` — Updated `staffOwnerRows = settingsMainOwnerRows - 4`.
- `fuku/modules/settings_phase18_test.go` — Updated Staff Group state-line expectation to `len(settingsMainTitleKeys)-7` and 30-page registry list in `TestSettingsPhase18Codes`.
- `fuku/modules/settings_phase19_test.go` — Added `"ba"` to `TestSettingsPhase19Codes` registered pages list.
- `fuku/modules/settings_phase29_test.go` — Added `"ba"` to `TestPhase29Codes` 30-page registry list.
- `fuku/modules/settings_backup_test.go` — Added 8-subtest `TestSettingsBackupPage` suite using `phase28InProcessMiniredis(t)` and `backupRoundTripFunc`.

## Decisions Made

- Kept all 20 `backup_module_*` button labels concise (`<= 17` runes) across all 8 languages and enforced that ceiling directly in `TestSettingsLabelLengths`.
- Used `settingsBackupFormatFailNotice` so `backup_import_failed` and `backup_reset_failed` notices cleanly substitute `%s` with `common_settings_save_failed` on real locales without leaking raw internal DB errors or breaking marker-locale assertions.
- Synchronized `settings_phase18_test.go` alongside `settings_main_test.go`, `settings_fwdblock_test.go`, `settings_phase19_test.go`, and `settings_phase29_test.go` so all 30 registered settings pages match across every phase code guard.

## Deviations from Plan

None - plan executed as written.

## Verification Results

- `for f in locales/{en,es,fr,hi,id,pt,ru,uk}.yml; do grep -c '^settings_page_backup:' "$f"; done | grep -c '^1$'` -> `8`
- `for f in locales/{en,es,fr,hi,id,pt,ru,uk}.yml; do grep -c '^settings_bkp_import_wait:' "$f"; done | grep -c '^1$'` -> `8`
- `grep -c 'settingsPageBackup = "ba"' fuku/modules/settings_backup.go` -> `1`
- `grep -c 'settingsTypedPages\[settingsPageBackup\]' fuku/modules/settings_backup.go` -> `1`
- `TZ=UTC go test -tags testtools -race -count=1 ./fuku/i18n` — PASSED (full package, 0 failures, 0 skips)
- `make check-translations` — PASSED (1068 keys present across all 8 locales)
- `TZ=UTC go test -tags testtools -race -count=1 -run '^TestSettingsBackupPage$' -v ./fuku/modules 2>&1 | grep -c '^    --- PASS: TestSettingsBackupPage/'` -> `8`
- `TZ=UTC go test -tags testtools -race -count=1 -run '^(TestSettingsBackupPage|TestSettingsCopyPage|TestSettingsMainPageFitsEveryLocale|TestSettingsPhase18Codes|TestSettingsPhase19Codes|TestPhase28Codes|TestPhase29Codes|TestSharedBackupApplyHelpers|TestSettingsBackupRedisTokens|TestExportGoesToThePrivateChat|TestLegacyPhase30Baseline|TestEventEngineCallers|TestPMCopyCallers|TestNoRawNewCommand|TestCommandRepliesStayInsideTheHandler)$' ./fuku/modules` — PASSED (0 failures, 0 skips)
- `TZ=UTC go test -tags testtools -race -count=1 -run '^TestSettingsMainGrid$/^(the_owner|the_same|nine_grid|fifteen_grid|an_allowed|an_odd|a_page|a_notice|no_visible)' ./fuku/modules` — PASSED (10/10 pure subtests)
- `go vet -tags testtools ./fuku/i18n ./fuku/modules` — PASSED
- `CGO_ENABLED=0 go build ./...` — PASSED
- `git diff --exit-code 1da5af6f -- go.mod go.sum` — PASSED
- `git diff --exit-code f73400f4 -- fuku/modules/phase30_legacy_request_test.go` — PASSED
- **Sandbox note on pre-existing TCP-miniredis tests:** Pre-existing regression tests that dial loopback `127.0.0.1` via `miniredis.Run()` (`TestSettingsMainGrid/end_to_end`, `TestSettingsMainPhase18`, `TestSettingsMainPhase19`, `TestSettingsFwdBlockMainShape`, `TestSettingsPhase18InStaffGroup`, `TestPhase26Codes`, `TestPhase27Codes`) cannot bind/dial loopback TCP inside this sandbox (`operation not permitted`). All updated grid/row/state-line constants (`settingsMainShapeGroup`, `settingsMainShapePrivate`, `settingsMainOwnerRows = 17`, Staff Group `settingsMainOwnerRows - 4 = 13`, and Staff Group `len(settingsMainTitleKeys) - 7 = 22`) were verified live in-process inside `TestSettingsBackupPage/open_private_and_group_note_and_staff_hidden` under `-race` with `phase28InProcessMiniredis`, and all code/registry suites (`TestSettingsPhase18Codes`, `TestSettingsPhase19Codes`, `TestPhase28Codes`, `TestPhase29Codes`, `TestSettingsMainPageFitsEveryLocale`) passed under `-race`.

## Issues Encountered

- Initial run of `TestSettingsBackupPage` surfaced two test-fixture typos in `fuku/modules/settings_backup_test.go` (calling `/export` with a user not listed in `phase30NewBackupClient` and encoding `buttons` as a JSON string instead of a `models.ButtonArray` slice). Both were fixed in `settings_backup_test.go` before committing Task 3.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Private Backup settings page `ba`, 8-locale strings, grid updates, and `TestSettingsBackupPage` are complete and verified under `-race`.
- Ready for Wave 5 (Plan 30-05: Phase 30 safeguard suite `settings_phase30_test.go`, E2E suite `phase30_e2e_test.go`, and `AGENTS.md` documentation update).

## Self-Check: PASSED

- Created files `fuku/modules/settings_backup.go`, `fuku/modules/settings_backup_test.go`, and `30-04-SUMMARY.md`: FOUND
- Task commits `0db9f55a`, `a6be0aa5`, and `d8880d38`: FOUND and ancestors of HEAD (`commits: 3`)
- Zero stubs, zero skipped tests, zero unmitigated threat flags
- All acceptance criteria, i18n/translation checks, `-race` unit/golden/caller/page tests, `go vet`, and `CGO_ENABLED=0 go build ./...`: PASSED
