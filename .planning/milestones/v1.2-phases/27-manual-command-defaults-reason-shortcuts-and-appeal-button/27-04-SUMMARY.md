---
phase: 27-manual-command-defaults-reason-shortcuts-and-appeal-button
plan: 04
subsystem: moderation
status: complete
tags: [modtuning, appeal-button, event-messages, pm-copy, locales, tune-06]
requires:
  - 27-02 (chat_manual_defaults.appeal_url column, fuku/db/modtuning skipLocal repository, modtuning_settings.go)
provides:
  - Appeal URL validation, apply, and defensive read helpers (validateAppealURL, applyAppealURL, manualAppealURLFor) in fuku/modules/modtuning_settings.go
  - eventmsg_btn_appeal localized button label across all 8 locales with {20, 28} ceiling in fuku/i18n/eventmsg_locale_test.go
  - Appeal button attachment on default and custom ban group messages, PM copies, and refused-custom PM fallbacks inside fuku/modules/event_message.go and fuku/modules/event_message_pm.go
  - Automatic ban Appeal flags on EventWarnLimit, EventFlood, EventBlacklist, EventForwardBlock, and EventCaptcha (plus EventReqBan)
  - Messages page ReservedButtons updates (EventBan 2, EventTimedBan 1, EventReqBan 1; PM editors 1) and Full preview appeal button support in fuku/modules/settings_messages.go
  - Comprehensive validation, ban/PM/Details/100-button-cap/preview/non-ban/isolation tests in fuku/modules/modtuning_appeal_test.go
affects: [27-05, 27-07]
tech-stack:
  added: []
  patterns:
    - "Single lookup of manualAppealURLFor in postEventSent gated by eventSupportsAppeal(r) so non-ban events perform zero appeal lookups and unset groups pass r.Opts untouched for MSG-11 byte-identity"
    - "Defensive re-validation of stored appeal URL on read via validateAppealURL (http/https + non-empty host + no whitespace + <= 2048 runes)"
    - "100-button Telegram keyboard cap enforced in eventAppendAppealRow (pmCopyButtonCount(rows)+len(appealRow) <= settingsEditMaxButtons) alongside static ReservedButtons on ban message editors"
key-files:
  created:
    - fuku/modules/modtuning_appeal_test.go
  modified:
    - fuku/modules/modtuning_settings.go
    - fuku/modules/event_message.go
    - fuku/modules/event_message_pm.go
    - fuku/modules/warns.go
    - fuku/modules/antiflood.go
    - fuku/modules/event_message_auto.go
    - fuku/modules/settings_messages.go
    - fuku/i18n/eventmsg_locale_test.go
    - locales/en.yml
    - locales/es.yml
    - locales/fr.yml
    - locales/hi.yml
    - locales/id.yml
    - locales/pt.yml
    - locales/ru.yml
    - locales/uk.yml
decisions:
  - "P27-04 implemented: eventSupportsAppeal recognizes EventBan, EventTimedBan, EventReqBan, and automatic events with Appeal=true (EventWarnLimit, EventFlood, EventBlacklist, EventForwardBlock, EventCaptcha when applied action is ban or effective tban), while rejecting negative target IDs (channel bans) and leaving silent bans, non-ban events, lockdowns, and Staff Group actions isolated."
  - "validateAppealURL accepts empty string to clear the appeal link, rejects whitespace-only non-empty strings, trims surrounding whitespace on non-empty URLs, and enforces <= 2048 runes, no internal whitespace, and content.ValidButtonURL on both write and read (T-27-09)."
  - "Wired Appeal flag for both EventBlacklist and EventForwardBlock inside shared listNoticeReply (fuku/modules/event_message_auto.go), leaving blacklists.go, forwardblock_notice.go, and memberreq_notice.go (which already emits EventReqBan directly) untouched."
  - "Tested pmCopyAddDetails coexistence end-to-end with a socket-free in-memory go-redis ProcessHook so the test runs reliably inside sandboxed environments without binding localhost TCP sockets."
metrics:
  duration: 30min
  completed: 2026-11-22
actuals:
  tokens: 14423
  tasks: 3
  commits: 3
plan_head_before: 7f76543dae665e8c7a1e107e85a43cd2e80a9855
plan_head_after: 3d159a70a41ad094ab17626d774dcec9105302e1
---

# Phase 27 Plan 04: Appeal URL Validation, Ban Message Appeal Button, and Preview Integration Summary

Implemented TUNE-06 end-to-end: `validateAppealURL`, `applyAppealURL`, and `manualAppealURLFor` in `fuku/modules/modtuning_settings.go`, `eventmsg_btn_appeal` across all 8 locales, appeal URL button attachment on default and custom ban group messages, PM copies, refused-custom PM fallbacks, Details-edited group messages, automatic ban outcomes, and Messages page Full previews/reserved button counts, while keeping unset groups byte-identical (MSG-11) and EVT-10 caller guards green.

## Performance

- **Duration:** 30 min
- **Started:** 2026-11-22T20:13:26Z
- **Completed:** 2026-11-22T20:43:53Z
- **Tasks:** 3
- **Files created:** 1
- **Files modified:** 16

## Accomplishments

- **Task 1 (`44ac320f`):** Added `SetAppealURL` to `modTuningWrites` and implemented `validateAppealURL`, `applyAppealURL` (IDs-only logging, never logging URL or raw error text), and `manualAppealURLFor` (defensive re-validation on read) in `fuku/modules/modtuning_settings.go`; added `eventmsg_btn_appeal` across all 8 locales (`en`, `uk`, `es`, `fr`, `hi`, `id`, `pt`, `ru`) and registered `"eventmsg_btn_appeal": {20, 28}` in `fuku/i18n/eventmsg_locale_test.go`.
- **Task 2 (`2db58d29`):** Added `eventReply.Appeal`, `eventReply.AppealURL`, `eventSupportsAppeal`, `eventAppealRow`, and `eventAppendAppealRow` in `fuku/modules/event_message.go`; populated `r.AppealURL` once in `postEventSent` only when `eventSupportsAppeal(r)` holds (including before `eventMessagesLookup` error fallback); attached the appeal row in `sendDefaultPost` (passing `r.Opts` untouched when unset and returning updated `Rows` for `pmCopyAddDetails`), `composeEventMessage`, `composeEventPM`, and `runEventPM` refused-custom PM fallback (`eventPMOffer.AppealRow`); set `Appeal` on automatic ban/tban outcomes in `warns.go`, `antiflood.go`, and `event_message_auto.go` (`listNoticeReply` covering blacklist + forward blocklist, plus `postCaptchaFailureNotice`); bumped `ReservedButtons` in `fuku/modules/settings_messages.go` (`EventBan`: 2, `EventTimedBan`: 1, `EventReqBan`: 1, and their PM editors: 1) and wired `manualAppealURLFor` into `settingsMsgPreview` and `settingsMsgPMPreview` Full previews.
- **Task 3 (`3d159a70`):** Added `fuku/modules/modtuning_appeal_test.go` with `TestAppealURLValidationAndApply` (4 subtests) and `TestAppealButtonOnBanMessages` (10 subtests) verifying validation/clearing/rejections/IDs-only logs, unset byte-identity, default `/ban` & `/tban` & `default_ban_secs` & lookup-failure fallback, custom button ordering, PM default/custom/refused fallback, Details coexistence with in-memory Redis hook, automatic bans vs non-bans and channel bans, non-ban manual commands and zero-lookup guard, 100-button keyboard cap (group & PM), Messages previews & `ReservedButtons`, and Staff Group/lockdown isolation.

## Task Commits

1. **Task 1: add applyAppealURL and eventmsg_btn_appeal across all 8 locales** — `44ac320f` (`feat(27-04): add applyAppealURL validation and eventmsg_btn_appeal across locales`)
2. **Task 2: attach appeal button in event_message.go, event_message_pm.go, automatic ban sites, and Messages previews** — `2db58d29` (`feat(27-04): attach appeal button to ban messages, PM copies, and previews`)
3. **Task 3: comprehensive appeal URL and appeal button tests** — `3d159a70` (`test(27-04): add appeal URL validation and appeal button ban-message tests`)

## Verification Results

- `go test -tags testtools -count=1 -run '^(TestEventMsgLocaleKeys|TestLocaleStyle|TestLocalePlaceholdersMatchOtherLanguages)$' ./fuku/i18n` — PASS.
- `go test -tags testtools -race -count=1 -run '^(TestAppealURLValidationAndApply|TestAppealButtonOnBanMessages|TestEventEngineCallers|TestPMCopyCallers|TestLegacyModerationRequests|TestLegacyPhase27ModerationRequests|TestLegacyPunishmentNoticeRequests|TestSettingsEditReservedButtons|TestManualCommandDefaults|TestReasonShortcutValidationAndApply|TestReasonShortcutExpansion)$' ./fuku/modules` — PASS (10/10 subtests of `TestAppealButtonOnBanMessages`, 4/4 subtests of `TestAppealURLValidationAndApply`, all 17 subtests of `TestLegacyModerationRequests`, all 17 subtests of `TestLegacyPhase27ModerationRequests`, all 22 subtests of `TestLegacyPunishmentNoticeRequests`, all 5 subtests of `TestSettingsEditReservedButtons`, and both EVT-10 caller guards passed with 0 failures and 0 skips).
- `make check-translations && CGO_ENABLED=0 go build ./...` — PASS (all 1068 keys present across 8 locales; full binary build green).
- `gofmt -l` across all created/modified Go files — clean.
- `git diff --exit-code 7f76543dae665e8c7a1e107e85a43cd2e80a9855 -- fuku/modules/moderation_phase27_legacy_request_test.go fuku/modules/moderation_legacy_request_test.go fuku/modules/punishment_notice_legacy_request_test.go fuku/modules/screening_notice_legacy_request_test.go go.mod go.sum` — PASS (all MSG-11 golden suites and Go dependencies untouched).
- Note on `TestLegacyScreeningNoticeRequests`: when executed inside the sandbox, its `req_*` / `aispam_*` subtests attempt to dial `127.0.0.1` TCP sockets (`miniredis` / `httptest.NewServer`), which the sandbox blocks (`connect: operation not permitted`), and its `captcha_*` subtests hit a pre-existing SQLite `chk_captcha_expires_at` failure during fixture setup before reaching `postCaptchaFailureNotice`. Unchanged screening behavior and `EventReqBan`/`EventCaptcha` appeal attachment are verified directly in `TestAppealButtonOnBanMessages/automatic_bans_carry_appeal_and_non_bans_do_not` and `TestEventEngineCallers`/`TestPMCopyCallers`.

## Deviations from Plan

None - plan executed as written. Because `blacklistNotice` (`blacklists.go`) and `forwardBlockNotice` (`forwardblock_notice.go`) both delegate to `listNoticeReply` in `event_message_auto.go`, and `memberReqPostEventNotice` (`memberreq_notice.go`) already emits `EventReqBan` directly, wiring `Appeal` inside `listNoticeReply` covered both blacklist and forward blocklist without redundant edits to `blacklists.go`, `forwardblock_notice.go`, or `memberreq_notice.go`.

## Known Stubs

None.

## Next Phase Readiness

- TUNE-06 appeal URL validation, apply function (`applyAppealURL`), button attachment on ban messages and PM copies, and Messages page preview/reserve integration are complete and ready for Plan 27-05 (Manual defaults & Appeal link private settings page).

## Self-Check: PASSED

- Created/modified files verified on disk: `fuku/modules/modtuning_appeal_test.go`, `fuku/modules/modtuning_settings.go`, `fuku/modules/event_message.go`, `fuku/modules/event_message_pm.go`, `fuku/modules/warns.go`, `fuku/modules/antiflood.go`, `fuku/modules/event_message_auto.go`, `fuku/modules/settings_messages.go`, `fuku/i18n/eventmsg_locale_test.go`, and all 8 `locales/*.yml` files — ALL FOUND.
- Task commits verified as ancestors of HEAD: `44ac320f`, `2db58d29`, `3d159a70` — ALL FOUND.
