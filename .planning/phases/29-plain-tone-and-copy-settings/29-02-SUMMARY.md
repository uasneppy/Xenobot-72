---
phase: 29-plain-tone-and-copy-settings
plan: 02
subsystem: event-messages
tags: [plain-tone, tune-08, migration, eventmsg, settings-messages, i18n, tracer]

requires:
  - phase: 29-plain-tone-and-copy-settings
    plan: 01
    provides: Frozen MSG-11 characterization goldens in TestLegacyPhase29Baseline
provides:
  - Idempotent migration adding plain_tone BOOLEAN NOT NULL DEFAULT FALSE to chat_event_message_options with statement and PostgreSQL integrity tests
  - ChatEventMessageOptions.PlainTone, eventmsg.Options.PlainTone, column-targeted SetPlainTone, and applyEventPlainTone
  - Pure event_message_plain.go builders outside eventEngineEntries plus lazy eventReply.PlainDefault and eventPMSpec.PlainKey hooks in postEventSent, composeEventPM, eventPMDefaultText, and EventBan
  - Messages hub Plain tone toggle (ht, can_change_info, private-only, row widths [2 2 1 1 1 2]), Main state indicator (settings_state_msg_plain), Full preview PlainTone wiring, and 8-locale UI + tracer ban keys
affects: [29-03, 29-04, 29-05, 29-06]

tech-stack:
  added: []
  patterns:
    - "Lazy PlainDefault func() string on eventReply evaluated in postEventSent only after eventMessagesLookup succeeds with msgs.Options.PlainTone == true (zero overhead or behavior change when Plain tone is off)"
    - "Pure plain-tone builders isolated in fuku/modules/event_message_plain.go with zero eventEngineEntries references so TestEventEngineCallers and TestPMCopyCallers require no allow-list changes"
    - "Column-targeted upsertOptions for plain_tone dropping event_messages:<chat> (already in skipLocal)"

key-files:
  created:
    - migrations/20261123130000_add_event_message_plain_tone.sql
    - fuku/modules/event_message_plain.go
    - fuku/db/migrations/event_message_plain_tone_migration_test.go
  modified:
    - fuku/db/models/event_message.go
    - fuku/db/eventmsg/repository.go
    - fuku/db/eventmsg/repository_test.go
    - fuku/db/backup/event_messages_backup_test.go
    - Makefile
    - fuku/modules/event_message.go
    - fuku/modules/event_message_pm.go
    - fuku/modules/event_message_apply.go
    - fuku/modules/bans.go
    - fuku/modules/settings_edit.go
    - fuku/modules/settings_messages.go
    - fuku/modules/settings_messages_test.go
    - fuku/modules/settings_messages_auto_test.go
    - fuku/modules/settings_messages_lockdown_test.go
    - fuku/modules/settings_messages_pm_options_test.go
    - fuku/modules/settings_messages_screening_test.go
    - fuku/modules/settings_messages_screens_test.go
    - fuku/modules/settings_messages_safety_test.go
    - fuku/modules/event_message_pm_safety_test.go
    - fuku/modules/event_message_pm_test.go
    - fuku/modules/modtuning_appeal_test.go
    - fuku/modules/settings_messages_pm_preview_test.go
    - fuku/modules/settings_staff_privacy_test.go
    - fuku/i18n/eventmsg_locale_test.go
    - fuku/i18n/settings_locale_test.go
    - locales/en.yml
    - locales/es.yml
    - locales/fr.yml
    - locales/hi.yml
    - locales/id.yml
    - locales/pt.yml
    - locales/ru.yml
    - locales/uk.yml

key-decisions:
  - "Evaluated r.PlainDefault in postEventSent after site.Admit and right before postEventGroup so unadmitted or tone-off replies never call PlainDefault, custom text overrides Plain tone, custom media/buttons without custom text use the plain default text, and Telegram custom-message refusals fall back to the plain default."
  - "Added PlainTone bool to settingsEditState so settingsMsgStateFor threads msgs.Options.PlainTone cleanly into both group and PM Full previews without an extra DB read."
  - "Extracted settingsMsgRedrawHubApplied to share hub redraw/error handling cleanly between settingsMsgRunPMOption (ho) and settingsMsgRunPlainTone (ht)."

patterns-established:
  - "Plain tone builder pattern: pure func(tr *i18n.Translator, in eventDefaultInput) string in fuku/modules/event_message_plain.go reusing eventPlainReasonLine (eventmsg_pm_reason) with zero references to eventEngineEntries."
  - "PM plain-tone locale guard: TestEventMsgLocaleKeys enforces <b>{group}</b> on every eventmsg_plain_pm_* key alongside eventmsg_pm_*."

requirements-completed: [TUNE-08]

status: complete
plan_head_before: 50ce516de5cd86b8760fc8ec10f5c0e5fb3165bb
plan_head_after: 40659d85595af7b61936f5b00dd2e43045b5fb53
commits: 3

actuals:
  tokens: 25713
  tasks: 3
  commits: 3

duration: 23min
completed: 2026-11-23
---

# Phase 29 Plan 02: Plain Tone Vertical Tracer Slice Summary

**End-to-end Plain tone (TUNE-08) vertical tracer from idempotent `plain_tone` migration and column-targeted `eventmsg.SetPlainTone` through lazy `PlainDefault`/`PlainKey` engine hooks, 8-locale Messages hub toggle (`ht`), and `/ban` group, PM, and Full preview delivery**

## Performance

- **Duration:** 23 min
- **Started:** 2026-11-23T13:43:31Z
- **Completed:** 2026-11-23T14:06:46Z
- **Tasks:** 3
- **Files modified:** 36 (3 created, 33 modified; `fuku/modules/phase29_legacy_request_test.go`, `go.mod`, and `go.sum` untouched)

## Accomplishments

- **Migration, Model, and Repository (`3bddfcec`, `40659d85`):**
  - Added idempotent migration `migrations/20261123130000_add_event_message_plain_tone.sql` (`ALTER TABLE chat_event_message_options ADD COLUMN IF NOT EXISTS plain_tone BOOLEAN NOT NULL DEFAULT FALSE;`) with no transaction control or banned clauses, verified by offline `TestEventMessagePlainToneMigrationStatements` and PG-only `TestEventMessagePlainToneMigrationDefaults` (added to `Makefile`'s `test-postgres-integrity`).
  - Extended `models.ChatEventMessageOptions` (no `default:` GORM tag) and `eventmsg.Options` with `PlainTone bool`, populated it in `eventmsg.GetFresh`, added column-targeted `eventmsg.SetPlainTone` dropping `event_messages:<chat>` (in `skipLocal`), and added `applyEventPlainTone` on `eventMsgWrites`.
  - Extended `fuku/db/eventmsg/repository_test.go` for concurrent 3-column non-clobbering writes, Go-false persistence, and cache invalidation, and `fuku/db/backup/event_messages_backup_test.go` proving `"plain_tone"` stays excluded from backups and survives resets until Phase 30.
- **Engine, PM Composer, Pure Plain Builders, and Messages Hub (`3bddfcec`):**
  - Created `fuku/modules/event_message_plain.go` with pure `eventPlainReasonLine` and `banPlainDefaultReply` referencing zero `eventEngineEntries` (`TestEventEngineCallers` and `TestPMCopyCallers` pass with zero allow-list edits).
  - Added lazy `PlainDefault func() string` on `eventReply` (swapping `r.Default` in `postEventSent` only when `msgs.Options.PlainTone && r.PlainDefault != nil`), `PlainKey string` on `eventPMSpec` (`eventmsg_plain_pm_ban` for `EventBan`), `plain bool` parameter on `composeEventPM` and `eventPMDefaultText`, and `PlainDefault` on the untimed `/ban` send site in `bans.go`.
  - Registered `settingsActMsgPlainTone = "ht"` (`Fields: []string{"v"}`, `Right: settingsRightChangeInfo`, private-only via `settingsListPrivateField` + `settingsEditSwitchValue`, 37-byte callback for 14-char chat IDs), added the Plain tone line/hint/button to `renderSettingsMsgHub` (updating hub row widths to `[2 2 1 1 1 2]`), wired `settings_state_msg_plain` into `settingsMsgMainState`, and threaded `PlainTone` through `settingsEditState` into group and PM Full previews.
- **8-Locale UI & Tracer Keys (`871736a4`):**
  - Added `settings_state_msg_plain`, `settings_msg_line_plain_tone`, `settings_btn_msg_plain_tone`, `settings_msg_hint_plain_tone`, `eventmsg_plain_ban`, and `eventmsg_plain_pm_ban` across all 8 locales (`en`, `es`, `fr`, `hi`, `id`, `pt`, `ru`, `uk`).
  - Registered ceilings, icon toggle length check, placeholder checks, and `eventmsg_plain_pm_*` `<b>{group}</b>` enforcement in `fuku/i18n/settings_locale_test.go` and `fuku/i18n/eventmsg_locale_test.go`. All 5 `./fuku/i18n` suites and `make check-translations` pass.
- **Hub Widths & End-to-End Tracer Verification (`40659d85`):**
  - Updated hub row widths (`[2 2 1 1 2]` -> `[2 2 1 1 1 2]`) across all 6 Messages hub test files, registered `settingsActMsgPlainTone` in `settingsMsgValidPresses`, `settingsMsgForgedPresses`, and `TestSettingsMessagesCallbackBytes`, and added end-to-end tests in `TestSettingsMessagesPMOptions` proving `ht` authority/forged rejection, Plain tone `/ban` group + PM + Full preview output, custom text override, and tone-off restoration matching `TestLegacyPhase29Baseline` byte-for-byte.

## Task Commits

1. **Task 1: tracer — migration, model, SetPlainTone repository, engine/PM/ban hooks, and Messages hub Plain tone switch** - `3bddfcec` (`feat(29-02): wire plain_tone storage, engine/PM/ban tracer hooks, and Messages hub toggle`)
2. **Task 2: add Messages hub Plain tone UI keys and tracer ban plain-tone keys across all 8 locales** - `871736a4` (`feat(29-02): add Plain tone hub UI and tracer ban keys across all 8 locales`)
3. **Task 3: migration, repository, backup exclusion, hub-width updates, and end-to-end tracer verification** - `40659d85` (`test(29-02): verify plain_tone migration, repo, backup exclusion, hub widths, and /ban tracer`)

## Files Created/Modified

- `migrations/20261123130000_add_event_message_plain_tone.sql` — Idempotent migration adding `plain_tone BOOLEAN NOT NULL DEFAULT FALSE` to `chat_event_message_options`.
- `fuku/db/models/event_message.go`, `fuku/db/eventmsg/repository.go` — `PlainTone` field on `ChatEventMessageOptions`/`Options`, `GetFresh` read, and column-targeted `SetPlainTone` with cache invalidation.
- `fuku/modules/event_message_plain.go` — Pure plain-tone builders (`eventPlainReasonLine`, `banPlainDefaultReply`) with zero `eventEngineEntries` references.
- `fuku/modules/event_message.go`, `fuku/modules/event_message_pm.go`, `fuku/modules/event_message_apply.go`, `fuku/modules/bans.go` — Lazy `PlainDefault` / `PlainKey` wiring, `plain bool` composer parameter, `applyEventPlainTone`, and untimed `/ban` tracer hook.
- `fuku/modules/settings_edit.go`, `fuku/modules/settings_messages.go` — `settingsEditState.PlainTone`, `settingsActMsgPlainTone` (`ht`), hub layout `[2 2 1 1 1 2]`, `settingsMsgMainState` plain indicator, and preview wiring.
- `locales/{en,es,fr,hi,id,pt,ru,uk}.yml`, `fuku/i18n/{settings,eventmsg}_locale_test.go` — 6 Plain tone UI and tracer ban keys across all 8 locales and their guard tests.
- `fuku/db/migrations/event_message_plain_tone_migration_test.go`, `Makefile`, `fuku/db/eventmsg/repository_test.go`, `fuku/db/backup/event_messages_backup_test.go` — Migration statement/PG tests, repository isolation/cache tests, and backup exclusion update.
- `fuku/modules/settings_messages{,_auto,_lockdown,_pm_options,_screening,_screens,_safety}_test.go` — Hub width updates, `ht` forged/authority/callback-byte coverage, and `/ban` end-to-end tracer subtest.
- `fuku/modules/{event_message_pm_safety,event_message_pm,modtuning_appeal,settings_messages_pm_preview,settings_staff_privacy}_test.go` — Updated internal `composeEventPM`/`eventPMDefaultText` unit test call sites to pass `plain=false`.

## Decisions Made

- Placed the `msgs.Options.PlainTone && r.PlainDefault != nil` swap after `site.Admit` and immediately before `postEventGroup` in `postEventSent` so unadmitted messages and tone-off messages never invoke `PlainDefault`.
- Threaded `PlainTone` through `settingsEditState` populated in `settingsMsgStateFor` so both `settingsMsgPreview` and `settingsMsgPMPreview` honor Plain tone from the already-loaded fresh read.
- Omitted unused `settings_msg_plain_title` (not referenced by Task 1's hub renderer or Task 2's 4-key behavior spec) to keep the Messages hub compact and avoid dead locale keys.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Updated 5 existing unit test callers of `composeEventPM` and `eventPMDefaultText`**
- **Found during:** Task 1
- **Issue:** Adding the required `plain bool` parameter to `composeEventPM` and `eventPMDefaultText` broke compilation of 5 existing unit test files in `./fuku/modules` that invoke those internal helpers directly (`event_message_pm_safety_test.go`, `event_message_pm_test.go`, `modtuning_appeal_test.go`, `settings_messages_pm_preview_test.go`, `settings_staff_privacy_test.go`). None of these are MSG-11 `*_legacy_request_test.go` golden files.
- **Fix:** Passed `false` as the `plain` argument at each existing tone-off call site so `./fuku/modules` compiles and preserves existing test assertions.
- **Files modified:** `fuku/modules/event_message_pm_safety_test.go`, `fuku/modules/event_message_pm_test.go`, `fuku/modules/modtuning_appeal_test.go`, `fuku/modules/settings_messages_pm_preview_test.go`, `fuku/modules/settings_staff_privacy_test.go`
- **Commit:** `3bddfcec`

## Issues Encountered

- Running pre-existing `TestMemberDeletionExcludedMessages` and `TestSettingsMessages*` tests inside the default sandbox fails on localhost TCP miniredis dials (`connect: operation not permitted`) and non-UTC host `TZ` triggers SQLite's pre-existing `chk_captcha_expires_at` comparison in `TestMemberDeletionExcludedMessages/a_captcha_challenge` (same pre-existing behavior noted in Phase 28 summaries). Running with `TZ=UTC` and loopback enabled passes every test cleanly under `-race`, and the new Plain tone tracer subtest uses `phase28InProcessMiniredis` (zero localhost TCP dials).

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- The complete Plain tone vertical tracer is proven end-to-end under `-race`, `TestLegacyPhase29Baseline` passes untouched, and `TestEventEngineCallers`/`TestPMCopyCallers` remain green with zero allow-list modifications.
- Ready for Plan 29-03 to expand `event_message_plain.go`, the remaining 22 group send sites, the remaining 17 PM specs (plus non-empty `PlainKey` init enforcement), and all 8-locale plain-tone strings.

## Self-Check: PASSED

- Created files exist: `migrations/20261123130000_add_event_message_plain_tone.sql`, `fuku/modules/event_message_plain.go`, `fuku/db/migrations/event_message_plain_tone_migration_test.go`
- Commits exist and are ancestors of `HEAD`: `3bddfcec`, `871736a4`, `40659d85` (`commits: 3` matching `git rev-list --count 50ce516de5cd86b8760fc8ec10f5c0e5fb3165bb..40659d85595af7b61936f5b00dd2e43045b5fb53`)
- Untouched invariants verified: `fuku/modules/phase29_legacy_request_test.go`, `go.mod`, `go.sum` have zero diffs
- All verification suites passed under `-race`: `./fuku/db/eventmsg`, `./fuku/db/migrations`, `TestEventMessagesStayOutOfBackup`, `./fuku/i18n` (5 suites) + `make check-translations`, `TestLegacyPhase29Baseline`, `TestEventEngineCallers`, `TestPMCopyCallers`, `TestMemberDeletionExcludedMessages`, `TestSettingsMessages*`, `TestSettingsAuthorityMessages*`, `TestSettingsDriftMessages*`, `CGO_ENABLED=0 go build ./...`, and `go vet -tags testtools`
