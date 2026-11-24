---
phase: 29-plain-tone-and-copy-settings
plan: 03
subsystem: event-messages
tags: [plain-tone, tune-08, i18n, eventmsg, pm-copy, settings-messages, testing]

requires:
  - phase: 29-plain-tone-and-copy-settings
    plan: 02
    provides: Plain tone storage/options, lazy PlainDefault/PlainKey engine hooks, Messages hub toggle, and /ban tracer slice
provides:
  - Neutral, emoji-free, third-person plain-tone locale keys for all 23 group events (35 templates) and all 18 PM copy specs across all 8 languages (en, es, fr, hi, id, pt, ru, uk)
  - Complete eventMsgLocaleLimits rows for every eventmsg_plain_* and eventmsg_plain_pm_* key
  - All 23 pure plain-tone group builders in fuku/modules/event_message_plain.go (zero eventEngineEntries references)
  - registerEventPM init panic guard requiring non-empty PlainKey on all 18 PM specs
  - Lazy PlainDefault wiring across all send sites (bans, mute, warns, auto, antiflood, aispam, memberreq, lockdown) and all 23 settingsMsgFamilies previews
  - Exhaustive 2-tone x 8-language x all-events test (TestPlainToneEveryEventEveryLocale) and 6-case runtime precedence/fallback/privacy test (TestPlainToneRuntime)
affects: [29-04, 29-05, 29-06]

tech-stack:
  added: []
  patterns:
    - "eventPlainRender splices already-escaped HTML user/admin mentions via lockdownSplice after i18n interpolation so untrusted display names containing placeholder tokens can never be re-interpolated"
    - "registerEventPM enforces non-empty spec.PlainKey at init so no PM-capable event can ship without a plain-tone default"
    - "Member requirement and lockdown plain builders reuse memberReqNoticeRemedies, lockdownStartNoteLines, and lockdownLiftNoteLines verbatim so remedies and technical notes stay identical between tones"

key-files:
  created:
    - fuku/modules/event_message_plain_test.go
  modified:
    - locales/en.yml
    - locales/es.yml
    - locales/fr.yml
    - locales/hi.yml
    - locales/id.yml
    - locales/pt.yml
    - locales/ru.yml
    - locales/uk.yml
    - fuku/i18n/eventmsg_locale_test.go
    - fuku/modules/event_message_plain.go
    - fuku/modules/event_message_pm.go
    - fuku/modules/bans.go
    - fuku/modules/mute.go
    - fuku/modules/warns.go
    - fuku/modules/event_message_auto.go
    - fuku/modules/antiflood.go
    - fuku/modules/aispam_notice.go
    - fuku/modules/memberreq_notice.go
    - fuku/modules/event_message_lockdown.go
    - fuku/modules/settings_messages.go
    - fuku/modules/event_message_pm_test.go
    - fuku/modules/event_message_pm_failures_test.go
    - fuku/modules/event_message_pm_details_test.go
    - fuku/modules/event_message_pm_log_test.go

key-decisions:
  - "Used Action: {action} phrasing in plain-tone flood, blacklist/fwdblock, and captcha templates so eventActionWord participles remain grammatical across all 8 languages including Hindi, Ukrainian, and Russian."
  - "Spliced user/admin HTML mentions post-interpolation via eventPlainRender (using lockdownNameToken + lockdownSplice) to prevent both HTML injection (T-29-05) and secondary placeholder expansion from untrusted names."
  - "Added PlainDefault closures in fuku/modules/warns.go (where warnEventReply and warnLimitEventReply live) alongside bans, mute, auto, antiflood, aispam, memberreq, and lockdown so every production eventReply constructor is wired."

patterns-established:
  - "Complete 23-event pure plain builder set in fuku/modules/event_message_plain.go with zero references to eventEngineEntries."
  - "Cross-tone x cross-locale verification pattern in TestPlainToneEveryEventEveryLocale asserting escaping, UTF-16 ceilings, zero unexpanded placeholders, and tone-off parity."

requirements-completed: [TUNE-08]

status: complete
plan_head_before: 599a7570b3848f7b02c209cc87a7f23a3522bb5f
plan_head_after: 14df5f1e754a339657b789f09992f6c997200f6b
commits: 3

actuals:
  tokens: 35128
  tasks: 3
  commits: 3

duration: 22min
completed: 2026-11-23
---

# Phase 29 Plan 03: Plain Tone Full Rollout Across All Events and Locales Summary

**Complete TUNE-08 plain-tone rollout covering all 23 milestone v1.2 group events and all 18 PM copy specs across all 8 languages, with `registerEventPM` init enforcement and exhaustive 2-tone x 8-language and runtime verification**

## Performance

- **Duration:** 22 min
- **Started:** 2026-11-23T14:08:38Z
- **Completed:** 2026-11-23T14:30:30Z
- **Tasks:** 3
- **Files modified:** 25 (1 created, 24 modified; `fuku/modules/phase29_legacy_request_test.go`, `go.mod`, and `go.sum` untouched)

## Accomplishments

- **8-Locale Plain-Tone Templates & i18n Guards (`065edac6`):**
  - Added the remaining 34 `eventmsg_plain_*` group keys (covering Manual, Automatic, Member requirements, and Lockdown families, including timed and nameless variants) and 17 `eventmsg_plain_pm_*` PM keys across all 8 locales (`en`, `es`, `fr`, `hi`, `id`, `pt`, `ru`, `uk`) — bringing totals to 53 `eventmsg_plain_*` keys (35 group + 18 PM) per locale.
  - Registered length ceilings for every plain key in `eventMsgLocaleLimits` (`fuku/i18n/eventmsg_locale_test.go`). Verified all 8 locales pass `TestEventMsgLocaleKeys`, `TestLocaleStyle`, `TestLocalePlaceholdersMatchOtherLanguages`, the full `./fuku/i18n` suite, and `make check-translations`.
- **All 23 Pure Builders, PM Spec Guard, Send-Site & Preview Wiring (`a2d9e884`):**
  - Implemented all 23 pure plain-tone group default builders in `fuku/modules/event_message_plain.go` (`banPlainDefaultReply`, `tbanPlainDefaultReply`, `kickPlainDefaultReply`, `mutePlainDefaultReply`, `tmutePlainDefaultReply`, `unbanPlainDefaultReply`, `unmutePlainDefaultReply`, `warnPlainDefaultReply`, `warnLimitPlainDefaultReply`, `floodPlainDefaultReply`, `blacklistPlainDefaultReply`, `captchaFailurePlainDefaultReply`, `aispamPlainDefaultReply`, `memberReqPlainNoticeDefault`, `lockdownStartPlainDefaultReply`, `lockdownAutoPlainDefaultReply`, `lockdownLiftPlainDefaultReply`) with zero `eventEngineEntries` references and safe HTML escaping + post-interpolation mention splicing (`eventPlainRender`).
  - Wired `PlainKey` on all 18 `registerEventPM` calls in `fuku/modules/event_message_pm.go` and added the `if spec.PlainKey == ""` init panic guard.
  - Wired lazy `PlainDefault` closures at every `eventReply` send site across `bans.go`, `mute.go` (preserving `mutes_unmute_lockdown_note`), `warns.go`, `event_message_auto.go`, `antiflood.go`, `aispam_notice.go`, `memberreq_notice.go`, and `event_message_lockdown.go`, and populated `PlainDefault` on all 23 entries in `settingsMsgFamilies` (`settings_messages.go`).
- **Exhaustive 2-Tone x 8-Language & Runtime Precedence Verification (`14df5f1e`):**
  - Added `TestPlainToneEveryEventEveryLocale` in `fuku/modules/event_message_plain_test.go` verifying all 23 group events (+ 14 timed/limit/nameless variants) and all 18 PM specs across both tones and all 8 languages for non-empty output, UTF-16 ceilings (`<= 4096`, single-rule `mreq* <= 520`), zero unexpanded placeholders, HTML escaping of `<script>&` (T-29-05), zero themed emojis in plain tone, dynamic value parity between tones, and distinct neutral English wording.
  - Added `TestPlainToneRuntime` with all 6 subtests (`plain_tone_off_matches_default`, `plain_tone_on_replaces_default_and_preserves_buttons_and_timers`, `custom_text_overrides_plain_tone`, `custom_media_without_text_uses_plain_default`, `refused_custom_message_falls_back_to_plain_default`, `fwdblock_and_aispam_plain_privacy`) using `phase28InProcessMiniredis` (total runtime 0.05s under `-race`).
  - Updated 4 existing PM unit test files (`event_message_pm_test.go`, `event_message_pm_failures_test.go`, `event_message_pm_details_test.go`, `event_message_pm_log_test.go`) to pass `PlainKey` to `registerEventPM` / `withTestEventPM` and assert that empty `PlainKey` panics.

## Task Commits

1. **Task 1: add all remaining neutral plain-tone group and PM locale keys across all 8 languages** - `065edac6` (`feat(29-03): add plain-tone group and PM locale keys across all 8 languages`)
2. **Task 2: implement all 23 pure builders in event_message_plain.go and wire every send site, PM spec, and preview** - `a2d9e884` (`feat(29-03): implement all 23 plain-tone builders and wire all send sites, PM specs, and previews`)
3. **Task 3: add exhaustive 2-tone x 8-language x all-events test and runtime precedence test** - `14df5f1e` (`test(29-03): add 2-tone x 8-locale x all-events and plain-tone runtime tests`)

## Files Created/Modified

- `locales/{en,es,fr,hi,id,pt,ru,uk}.yml` — Added 34 remaining `eventmsg_plain_*` group keys and 17 remaining `eventmsg_plain_pm_*` PM keys per locale (51 keys x 8 languages).
- `fuku/i18n/eventmsg_locale_test.go` — Added `eventMsgLocaleLimits` ceilings for all 51 new plain keys.
- `fuku/modules/event_message_plain.go` — Implemented all 23 pure plain-tone group builders and `eventPlainRender` post-interpolation mention splicer.
- `fuku/modules/event_message_pm.go` — Set `PlainKey` on all 18 PM specs and enforced `spec.PlainKey != ""` panic guard in `registerEventPM`.
- `fuku/modules/{bans,mute,warns,event_message_auto,antiflood,aispam_notice,memberreq_notice,event_message_lockdown}.go` — Wired lazy `PlainDefault` closures on every production `eventReply` send site.
- `fuku/modules/settings_messages.go` — Wired `PlainDefault` on all 23 `settingsMsgEvent` entries across all 4 families.
- `fuku/modules/event_message_plain_test.go` — Created `TestPlainToneEveryEventEveryLocale` and `TestPlainToneRuntime`.
- `fuku/modules/{event_message_pm,event_message_pm_failures,event_message_pm_details,event_message_pm_log}_test.go` — Supplied `PlainKey` in existing `registerEventPM`/`withTestEventPM` test fixtures and added empty-`PlainKey` panic test.

## Decisions Made

- Used neutral `Action: {action}` phrasing for plain-tone flood, blacklist/fwdblock, and captcha templates so translated `eventActionWord` participles remain grammatical across all 8 languages without needing per-action duplicates.
- Used post-interpolation mention splicing (`eventPlainRender` with `lockdownNameToken` + `lockdownSplice`) across all plain builders so untrusted display names containing placeholder tokens or `<script>&` are always safely escaped and never re-interpolated.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Wired `PlainDefault` in `fuku/modules/warns.go`**
- **Found during:** Task 2
- **Issue:** Plan 29-03 listed `warnEventReply` and `warnLimitEventReply` under `event_message_auto.go`, whereas in the codebase both functions live in `fuku/modules/warns.go`. Leaving `warns.go` unwired would have left `EventWarn` and `EventWarnLimit` send sites without `PlainDefault`.
- **Fix:** Wired lazy `PlainDefault` closures (`warnPlainDefaultReply`, `warnLimitPlainDefaultReply`) directly in `warnEventReply` and `warnLimitEventReply` in `fuku/modules/warns.go`.
- **Files modified:** `fuku/modules/warns.go`
- **Commit:** `a2d9e884`

**2. [Rule 1 - Bug] Updated 4 existing PM unit test files for `registerEventPM` empty `PlainKey` panic guard**
- **Found during:** Task 3
- **Issue:** Adding the required `if spec.PlainKey == ""` panic guard in `registerEventPM` caused 4 existing PM unit test files (`event_message_pm_test.go`, `event_message_pm_failures_test.go`, `event_message_pm_details_test.go`, `event_message_pm_log_test.go`) that passed `eventPMSpec{DefaultKey: ...}` without `PlainKey` to panic. None of these are MSG-11 `*_legacy_request_test.go` golden files.
- **Fix:** Added `PlainKey: "eventmsg_plain_pm_ban"` to each existing test fixture and added an explicit empty-`PlainKey` panic assertion in `TestEventPMRegister`.
- **Files modified:** `fuku/modules/event_message_pm_test.go`, `fuku/modules/event_message_pm_failures_test.go`, `fuku/modules/event_message_pm_details_test.go`, `fuku/modules/event_message_pm_log_test.go`
- **Commit:** `14df5f1e`

## Issues Encountered

- Running pre-existing `TestMemberDeletionExcludedMessages` inside the restricted sandbox fails on localhost TCP miniredis dials (`connect: operation not permitted`, matching the environment note in `29-02-SUMMARY.md`). Running with `TZ=UTC` and loopback enabled passes all suites cleanly under `-race` in ~3.7s; the new `TestPlainToneRuntime` uses `phase28InProcessMiniredis` (no localhost TCP dials).

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- TUNE-08 is complete across all 23 group events and all 18 PM events in all 8 languages, with custom messages unaffected, `TestLegacyPhase29Baseline` byte-identical, and `TestEventEngineCallers`/`TestPMCopyCallers` green.
- Ready for Plan 29-04 (Copy Settings shared validated message writer & repository helpers).

## Self-Check: PASSED

- Created files exist: `fuku/modules/event_message_plain_test.go`
- Commits exist and are ancestors of `HEAD`: `065edac6`, `a2d9e884`, `14df5f1e` (`commits: 3` matching `git rev-list --count 599a7570b3848f7b02c209cc87a7f23a3522bb5f..14df5f1e754a339657b789f09992f6c997200f6b`)
- Untouched invariants verified: `fuku/modules/phase29_legacy_request_test.go`, `go.mod`, `go.sum` have zero diffs
- All verification suites passed under `-race`: `./fuku/i18n` + `make check-translations`, `TestPlainToneEveryEventEveryLocale`, `TestPlainToneRuntime` (6/6 subtests), `TestLegacyPhase29Baseline`, `TestEventEngineCallers`, `TestPMCopyCallers`, `TestMemberDeletionExcludedMessages`, `TestEventPM*`, `TestSettingsMessagesScreens`, `TestSettingsMessagesPMOptions`, `CGO_ENABLED=0 go build ./...`, and `go vet -tags testtools`
