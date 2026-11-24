---
phase: 23-warn-flood-blacklist-and-forward-blocklist-messages
plan: 05
subsystem: event-messages
status: complete
tags: [event-messages, pm-copies, automatic-cap, i18n]
requires:
  - phase: 23-03
    provides: the Automatic family on the Messages page (both editor definitions per event)
  - phase: 23-04
    provides: every Automatic send site through postEvent with eventSite.Automatic set
provides:
  - private copy specs for warn, warnlimit (manual) and flood, blacklist, fwdblock (automatic)
  - eventPMOffer.ByRule, so a rule-raised reply is capped whatever its spec
  - eventPMDefaultText filling count, limit and the escaped action word
  - five default private texts in 8 languages
affects: [23-06, 23-07]
tech-stack:
  added: []
  patterns:
    - "a reply's cap flag is spec.Automatic || offer.ByRule, set only in offerEventPM; ByRule comes from the site, never from a caller"
    - "an Automatic private copy test runs on newEventPMAutoSite: newNoticeGoldenSite plus miniredis, the fast pacer and eventPMTestClient"
key-files:
  created:
    - fuku/modules/event_message_pm_auto_test.go
  modified:
    - fuku/modules/event_message_pm.go
    - fuku/modules/event_message.go
    - fuku/modules/event_message_pm_test.go
    - fuku/modules/event_message_pm_safety_test.go
    - fuku/modules/settings_messages_auto_test.go
    - fuku/modules/settings_messages_screens_test.go
    - fuku/i18n/eventmsg_locale_test.go
    - locales/{en,es,fr,hi,id,pt,ru,uk}.yml
    - AGENTS.md
decisions:
  - "The flood's default private text names the group's flood protection rather than 'the bot' (en: 'The flood protection of <b>{group}</b> acted on your messages. Action: {action}.'); the reason line then gives the flood reason."
  - "eventPMDefaultText escapes {action} (html.EscapeString of the translated word), as eventPlaceholderValues does for the group message."
  - "TestEventPMSpecs now pins each spec's exact key (eventmsg_pm_<event>) instead of only a non-empty key: a strengthening."
  - "TestEventPMTextsFitEveryLocale fills count 100, limit 100 and the language's longest action word, so the five new texts are measured at their worst case."
  - "The Messages page's screen sweep (settingsMsgEveryPMOn, settingsMsgScreens, TestSettingsMessagesCallbackBytes) requires a private copy spec for every event again, as before plan 23-03."
  - "TestEventPMAutoCap uses the blacklist (one watcher call per offender, no flood counters) with the cap at 2 and a stopped minute: about 1.4 s."
metrics:
  duration: ~45 min
  completed: 2026-11-21
actuals:
  tokens: 13650
  tasks: 3
  commits: 3
plan_head_before: 3aaf0e25987f8e985457d8023fa3355d541c3f37
plan_head_after: 2bf1bac70f5842eb20d414f9b37aeed68d47ee57
---

# Phase 23 Plan 05: Private copies of the Automatic family Summary

Each of the five Phase 23 events now has a private copy. The warn and the warn limit have manual specs, and the flood,
blacklist and forward blocklist have automatic specs. A warn that an automatic rule raised counts against the per-group cap
through `eventPMOffer.ByRule`; `/warn` never does. The default private texts name the group, the count, the limit and the
action where they apply, in all 8 languages.

## Tasks

| Task | Name | Commit |
|------|------|--------|
| 1 | the five default private texts in 8 languages | d32a3fc |
| 2 | the specs, the per-reply cap flag and the default text's values | 2b253d6 |
| 3 | the private copies through the real sites, the cap and the page | 2bf1bac |

## What later plans use

| Kind | Name | Notes |
|------|------|-------|
| Specs | `warn` (`eventmsg_pm_warn`), `warnlimit` (`eventmsg_pm_warnlimit`) `Automatic: false`; `flood`, `blacklist`, `fwdblock` (`eventmsg_pm_<event>`) `Automatic: true` | twelve specs in all, registered in `event_message_pm.go`'s `init` |
| Engine | `eventPMOffer.ByRule` | set by `offerReplyPM` from `site.Automatic`; `offerEventPM` sets the unexported `automatic` to `spec.Automatic \|\| o.ByRule` |
| Engine | `eventPMDefaultText` params `count`, `limit` (empty when 0), `action` (escaped `values.Action`) | the manual texts carry none of the three |
| Locale | `eventmsg_pm_warn`, `_warnlimit`, `_flood`, `_blacklist`, `_fwdblock` | rows `{140, 200}` in `eventMsgLocaleLimits` |
| Test toolkit | `newEventPMAutoSite(t, kind)`, `eventPMAutoDefault`, `eventPMAutoWant`, `eventPMAutoNone`, `eventPMAutoNoRedis`, `eventPMAutoOffender`, `eventPMAutoTitle`/`eventPMAutoGroup` | `event_message_pm_auto_test.go` |

## Earlier tests whose expected sets changed (no assertion weakened)

- `event_message_pm_test.go`, `TestEventPMSpecs`: the table is twelve specs (the seven manual events plus `warn` and
  `warnlimit`, not automatic; `flood`, `blacklist` and `fwdblock`, automatic), each with exactly the key
  `eventmsg_pm_<event>` (stronger than the old non-empty check). "warn has no spec" became "an unknown event (`warning`)
  has no spec", because `warn` now has one. The three panic cases are unchanged.
- Same file, `TestEventPMDefaultTexts`: one more subtest, `warn limit` (3 of 3, the ban word, the reason line); the ten cases
  are unchanged.
- `event_message_pm_safety_test.go`, `TestEventPMTextsFitEveryLocale`: the measured values gain count 100, limit 100 and the
  language's longest action word, so the new texts are measured at their worst case. The checks are unchanged.
- `settings_messages_screens_test.go`: `settingsMsgEveryPMOn`, `settingsMsgScreens`' event screen widths and
  `TestSettingsMessagesCallbackBytes`' private copy row count no longer branch on whether an event has a spec. Each now fails on
  an event without one and expects the private copy row on every event screen, which is what they did before plan 23-03.

## Verification (all with `-tags testtools -race -count=1`)

- Task 1: `make check-translations` all present; `./fuku/i18n` `TestEventMsgLocaleKeys|TestLocaleStyle|TestLocalePlaceholdersMatchOtherLanguages` ok (7.4 s with build). Acceptance greps: 5 keys in each of the 8 files; 5 rows in the limits test.
- Task 2: RED first (`TestEventPMSpecs` and the warn-limit subtest failed with no copy sent). Then the verify set
  (`TestEventPMSpecs|TestEventPMDefaultTexts|TestEventPMComposition|TestEventPMLimits|TestEventPMEndToEnd|TestEventPMTextsFitEveryLocale|TestPMCopyCallers|TestLegacyModerationRequests|TestLegacyPunishmentNoticeRequests`) passed in 11.4 s.
  Acceptance greps: 5 registrations, `ByRule` in both files, 5 placeholder names.
- Task 3: `TestEventPMAutoEvents` (7 subtests), `TestEventPMAutoCap`, `TestSettingsMessagesAutoPM`, `TestSettingsMessagesAutoFamily`,
  `TestEventPMLimits`, `TestEventPMPacerIndependent`, `TestSettingsMessagesCallbackBytes` and `TestSettingsMessagesPageFitsEveryLocale`
  all PASS (18.8 s). Two mutations both failed the new tests: `ByRule: false` failed `automatic warn without redis`, and a
  non-automatic blacklist spec failed `TestEventPMAutoCap` with 5 copies.
- Touched families (`TestEventPM.*`, `TestEventMessage.*`, `TestSettingsMessages.*`, `TestSettingsDriftMessages.*`,
  `TestSettingsAuthorityMessages.*`, `TestMessagesEndToEnd`, `TestLegacy.*`, `TestPMCopy.*`, `TestAutoEvent.*`, `TestEventActionWords`,
  `TestSettingsStaffPrivacy`, `TestBotAuto.*`, `TestBotManual.*`, `TestWarn.*`, `TestApplyWarn`, `TestRenderEventText`,
  `TestMemberDeletionExcludedMessages`, `TestCommandRepliesStayInsideTheHandler`, `TestForwardBlockWatcher.*`): ok, 161 s (2 min 44 s
  with build), 110 top-level PASS, no FAIL or SKIP. The `./fuku/i18n` eventmsg and style tests: ok, 5.4 s.
- Goldens: `git diff --exit-code 9d4554d -- fuku/modules/punishment_notice_legacy_request_test.go` and
  `git diff --exit-code 7ecc5fd~1 -- fuku/modules/moderation_legacy_request_test.go` both exit 0.
- `git diff --exit-code 1a382c3 -- go.mod go.sum main.go migrations` exits 0.
- `make check-translations` all present; `make check-docs` reports no drift; `make test-shard-check` rc 0 (22 s); `CGO_ENABLED=0 go build ./...` ok
  (the four together 9.5 s, without the shard check).
- golangci-lint v2.13.1 (`CGO_ENABLED=0`): `run --new-from-rev=origin/main ./...` 0 issues (39 s); with `--build-tags=notesttools`
  0 issues (19 s); `run --new=false ./fuku/modules/... ./fuku/i18n/...` filtered to the touched files prints nothing (only baseline
  findings in other files).
- gofmt is clean on every touched Go file. The full `fuku/modules` package was not run; plan 23-07 runs it.

## Deviations from Plan

1. [Rule 2 - correctness] The plan did not list `event_message_pm_safety_test.go`. Its locale sweep now ranges over the new
   specs, but with no count, limit or action the sweep would measure the five texts empty. It now fills worst-case values.
   Commit 2b253d6.
2. [Binding note from 23-03/23-04] `settings_messages_screens_test.go` was also not in the plan's file list. Its three spec-conditional
   checks were made strict again, as the 23-04 notes asked. Commit 2bf1bac.
3. [Choice] `TestEventPMAutoEvents` has seven subtests, not five. The extra two split out cases from the plan's behaviour list:
   `automatic warn without redis`, and `flood by a sender chat`, which the 23-04 notes asked to assert with `pmCopySkipTarget`.
   The `warn` subtest takes the cap's Redis client away before `/warn`, so one site proves both that the copy is sent and that it is
   never capped.
4. [Choice] The `blacklist` subtest punishes with kick and the `forward block` subtest with ban, so the copies show two more action
   words than mute alone. The flood subtest keeps mute.
5. [Choice] The flood text's wording differs from the plan's English draft; see decisions.

## Notes for plan 23-06

- All twelve events have a private copy spec. Every Automatic event screen draws the private copy row (`a=he&k=<code>&d=p`), and
  `settingsMsgDefByFields` resolves `d=p` for `wn wl fl bl fb`. The forged-press and authority tests can now cover the private
  copy forms (`he`, `hs`, `hr`, `hc`, `hv`, `hf` with `d=p`, and `hp`) of the new `k` values.
- `hp` (the private copy switch) works for the five new events with no code change. `settingsMsgPMFields` accepts them because
  each has a spec.
- The hub's private copies block and the ` · 📩` mark now cover the Automatic events, and Main counts a switched-on Automatic
  private copy as changed (`settingsMsgCountChanged`).
- The private copy previews (`settingsMsgPMPreview`) of the Automatic events fill the page's samples through
  `settingsMsgPreviewReply`. With no stored text the default private text gets the sample count, limit and action word, because
  `eventPMDefaultText` now reads `values.Count`, `.Limit` and `.Action`.
- `newEventPMAutoSite` is the site for any further private copy test of these events. It waits for the jobs in `t.Cleanup`,
  and a swap of `pmCopyNow`, `pmCopyAutoCapPerMinute` or `pmCopyRedisClient` needs one more `t.Cleanup(pmCopyJobs.wait)` after it
  (`eventPMAutoNoRedis` does this).
- The Details record and the log line of an Automatic copy that is not delivered use the same paths as Phase 22; this plan did
  not test them. D-31 asks plan 23-07 to check the forward blocklist's Details record and log line against the sentinels. The
  sentinel setup in `TestEventPMAutoEvents/forward block` can be reused for that.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register:
- T-23-13: `TestEventPMAutoCap` bounds a wave of offenders to the cap while every offender is still punished.
- T-23-14: the `automatic warn without redis` subtest and the `/warn` at the end of `TestEventPMAutoCap` cover it.
- T-23-15: the `forward block` subtest checks the sentinels on the private copy request.
- T-23-05: `TestPMCopyCallers` passes, and `registerEventPM` is still used only in `event_message_pm.go`.
- T-23-SC: `go.mod` and `go.sum` are unchanged.

## Self-Check: PASSED

- Files: `fuku/modules/event_message_pm_auto_test.go` exists, and `TestEventPMAutoCap` and `TestSettingsMessagesAutoPM` are defined.
- Commits: d32a3fc, 2b253d6 and 2bf1bac are ancestors of HEAD.
