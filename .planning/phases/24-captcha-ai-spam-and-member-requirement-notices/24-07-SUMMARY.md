---
phase: 24-captcha-ai-spam-and-member-requirement-notices
plan: 07
subsystem: event-messages-safety
status: complete
tags: [event-messages, settings-menu, safety, privacy, logs, tests]
requires:
  - 24-02 (the captcha site, the engine hooks, OffByDefault)
  - 24-03 (the six private copy specs)
  - 24-04 (the Messages page rows, the Member requirements family, NoMedia, NoSilent, the Notice switch)
  - 24-05 (the requirement site through the engine, its paced retry loop)
  - 24-06 (the AI spam site, aispamNoticeEnv)
provides:
  - settingsMsgScreeningForged and the Phase 24 entries of settingsMsgValidPresses
  - TestSettingsAuthorityMessagesScreening
  - settingsPrivacyPhase24Run (in TestSettingsStaffPrivacy)
  - TestEventMessageScreeningLogsCarryIDsOnly, TestEventMessageScreeningPrivacy
  - TestEventMessageMemberReqMuteWaitsOutRateLimit (the gap 24-05 left)
  - screenSafeLinkStaff, screenSafeRequests, screenSafeWantClean (toolkit in event_message_screening_safety_test.go)
affects: [24-08]
tech-stack:
  added: []
  patterns:
    - "a privacy sweep that exempts one pre-existing operator log line must also require that line to carry the sentinel, so the sweep proves the sentinel was in flight"
key-files:
  created:
    - fuku/modules/event_message_screening_safety_test.go
  modified:
    - fuku/modules/settings_messages_safety_test.go
    - fuku/modules/settings_messages_screening_test.go
    - fuku/modules/settings_staff_privacy_test.go
    - fuku/modules/event_message_pm_safety_test.go
    - AGENTS.md
decisions:
  - "/aispam's own `[AISpam] verdict` Info line keeps the category, model and probability by design (the plan's trust boundary keeps the verdict's details in the logs); the privacy test exempts that one line and requires it to carry the sentinel category."
  - "The requirement privacy case searches U+202E and U+200F (the controls the member's name carries), not every bidirectional control: the default requirement mention wraps the name in its own isolate (U+2068 ... U+2069) by design."
  - "The 24-05 gap is closed as a separate test in the safety file (TestEventMessageMemberReqMuteWaitsOutRateLimit) rather than a subtest of event_message_mreq_test.go, which is not in this plan's files."
  - "The page catalog settingsMsgScreens already drew every Phase 24 screen (the ai screen with its notice on and off, the requirement screens empty and full) since 24-04, so settings_messages_screens_test.go is unchanged."
metrics:
  duration: ~22 min
  completed: 2026-11-22
actuals:
  tokens: 9200
  tasks: 2
  commits: 3
plan_head_before: aa916892ae3dc3e3b2f99b3b2bf8078c0fd07c56
plan_head_after: 567c50e0012a3f852d683f0e47394340d85e238c
---

# Phase 24 Plan 07: The safety of the screening events Summary

The Messages page's and the event engine's safety rules now cover the six screening events and their three sites. Every forged press of
the new events and of the Member requirements family is refused in step 1 with no member lookup, an administrator without
`can_change_info` can view and preview every new screen but change nothing, the new screens create nothing and cost one read, no log line
of the captcha, requirement or AI spam sites carries content, and no group message, private copy, Details record or log line names the
Staff Group or the AI verdict, or carries a bidirectional control from a member's name. No production defect was found, so no production
file changed.

## Tasks

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | the page's safety over the new events and the Member requirements family | c4675d8 | settings_messages_safety_test.go, settings_messages_screening_test.go, settings_staff_privacy_test.go, AGENTS.md |
| 2 | the sites' logs, the privacy of every surface and the private texts' fit | de1eac4 | event_message_screening_safety_test.go (new), event_message_pm_safety_test.go, AGENTS.md |
| 2 (lint) | write the bidirectional controls as escape sequences (staticcheck ST1018) | 567c50e | event_message_screening_safety_test.go |

## What the tests hold

| Test | What it proves | Time (-race) |
|------|----------------|--------------|
| `TestSettingsMessagesForged` (extended) | 7 new valid presses (`hl&t=r`, `he&k=cf`, `he&k=rm&d=p`, `hq&k=ai&v=0`, `hs&k=rw&t=x`, `hp&k=rb&v=1`, `hf&k=cf`) each pass step 1 with one lookup; their generated forged forms plus the 9 of `settingsMsgScreeningForged` are refused in step 1 with no lookup, edit, row or wait | 0.25 s (whole test) |
| `TestSettingsMessagesReadsCreateNothing`, `TestSettingsMessagesCost` (extended) | the Member requirements list, the `rm` screen and its private copy screen and the `ai` screen create no row or key and cost the pipeline's calls and one read | 0.15 s, 0.14 s |
| `TestSettingsAuthorityMessagesScreening` (new) | a demoted typist stores nothing; without `can_change_info` the screens and Full preview open while 22 presses (Set both forms, Silent or Notice, the private copy switch of six events, two Removes) are the `settings_no_change_info` alert with no edit, wait or row change | 0.05 s |
| `settingsPrivacyPhase24Run` (new, in `TestSettingsStaffPrivacy`) | an administrator who is not the creator drives the new screens, the Notice press, a typed `rw` text and a `cf` Full preview; nothing names the Staff Group | 0.05 s |
| `TestEventMessageScreeningLogsCarryIDsOnly` (new) | six cases (read error and refused at each site); the AI read error posts nothing; no line carries content | 0.19 s |
| `TestEventMessageScreeningPrivacy` (new) | six cases (custom and defaults at each site), each group linked to a sentinel Staff Group; no request, Details record or log line leaks | 0.25 s |
| `TestEventMessageMemberReqMuteWaitsOutRateLimit` (new) | a custom mute notice waits out four 429s, sent five times with the custom text, the "I fixed it" row kept, nothing lifted | 0.08 s |
| `TestEventPMTextsFitEveryLocale` (extended) | each requirement event's default private text with all eight rule phrases fits 4096 units in 8 languages | 0.21 s (whole test) |

The new test time is about 0.9 s against the plan's 5 s.

Mutation checks (each reverted): removing `!ev.NoSilent` from `settingsMsgSilentFields` fails `TestSettingsMessagesForged` (`hq of rm with
v=1` reaches the lookup); giving `hq` the right `settingsRightAdmin` fails `TestSettingsAuthorityMessagesScreening`; skipping
`memberReqStripBidi` in `memberReqEventTarget` fails `TestEventMessageScreeningPrivacy` on the group notice and the private copy.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Raw format characters in a test literal**
- **Found during:** the lint gate after Task 2
- **Issue:** the member name and the leak list of the requirement privacy case held U+202E and U+200F as raw characters, which staticcheck
  reports (ST1018) and a reader cannot see.
- **Fix:** written as the escape sequences `\u202E` and `\u200F`; the test is unchanged in meaning.
- **Files modified:** fuku/modules/event_message_screening_safety_test.go
- **Commit:** 567c50e

### Additions beyond the plan

- `TestEventMessageMemberReqMuteWaitsOutRateLimit` closes the gap left by 24-05 (no test made a custom requirement mute notice wait out
  `ratelimit.ErrRateLimited`), as the orchestrator asked. It fits the plan's safeguards (D-19: a mute notice is never dropped by pacing).
- The privacy test's AI case exempts the pre-existing `[AISpam] verdict` line (see Decisions) instead of failing on it: that line logs the
  category, model and probability by design, and the plan's trust boundary says the verdict's details stay in the logs.

### Earlier tests whose expected values changed

None. Every edit to an earlier test only adds entries (`settingsMsgValidPresses`, the forged list, the opened screens, the cost draws, the
fit sweep's requirement reason) or a new subtest call; no expected value was changed and no assertion weakened. The goldens are unchanged:
`git diff --exit-code e40a7be -- fuku/modules/screening_notice_legacy_request_test.go`,
`git diff --exit-code 9d4554d -- fuku/modules/punishment_notice_legacy_request_test.go` and
`git diff --exit-code 86baf1a -- fuku/modules/moderation_legacy_request_test.go` all exit 0.

## Verification (with durations)

- Task 1 verify (8 tests) PASS: 32.5 s package, 70 s wall.
- Task 2 verify (7 tests plus the rate-limit test) PASS: 4.2 s package.
- Touched families with -race (`TestSettingsMessages.*`, the authority and drift families, `TestSettingsStaffPrivacy`,
  `TestSettingsEditReservedText`, `TestEventMessage.*`, `TestEventPM.*`, `TestPMCopy.*`, `TestAISpamNotice`, `TestMemberReqMuteNoticeRetry`,
  `TestMemberReqNotice.*`, `TestScreeningEventDefaults` and the three `TestLegacy*` goldens) PASS: 39.5 s package, 42 s wall.
- `make check-translations` all present (0.3 s); `make check-docs` no drift (1.2 s); `make test-shard-check` clean (21 s);
  `CGO_ENABLED=0 go build ./...` ok (2 s); `git diff --exit-code 86baf1a -- go.mod go.sum main.go migrations` exit 0.
- golangci-lint v2.13.1: `run --new-from-rev=origin/main ./...` 0 issues (37 s, after 567c50e); the same with `--build-tags=notesttools`
  0 issues (19 s); `run --new=false ./fuku/modules/... ./fuku/i18n/... ./fuku/db/eventmsg/...` with no issue cap: 110 baseline issues, none in
  a file this plan touched (the filter `settings_messages|event_message|settings_staff_privacy|aispam_notice|memberreq_notice` matches 0).
- Full `go test -tags testtools -race -count=1 ./fuku/modules`: ok, 340.5 s package (5 m 43 s wall), no skip.

## Notes for plan 24-08 (the end-to-end plan)

- `screenSafeLinkStaff(t, groupChatID)` links any group to a Staff Group titled with a sentinel and cleans both rows; `screenSafeRequests`
  dumps every send and edit of a `*moduleBotClient` as JSON (pass `env.fake.moduleBotClient` for the settings-based environments,
  `s.client` for a bot site) and `screenSafeWantClean` searches without regard to case.
- An end-to-end AI spam run sees `/aispam`'s `[AISpam] verdict` Info line carrying the category, model and probability. That is the
  operator's record, not part of the notice. A log sweep there must exempt it as `TestEventMessageScreeningPrivacy` does.
- The requirement privacy case uses a member with no username, so `{mention}` is the name. A member with a username gets `@username`
  and would not exercise the stripping.
- The captcha notice still deletes itself through `time.AfterFunc` after `captchaFailureNoticeLife` (30 s). Tests that do not shorten it
  leave a timer that fires after they end, as `TestEventPMCaptcha` does. An end-to-end test that waits for the delete should shorten it.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register:
- T-24-25: `settingsMsgScreeningForged`, the new valid presses, and `TestSettingsAuthorityMessagesScreening`.
- T-24-26: `TestEventMessageScreeningLogsCarryIDsOnly`.
- T-24-27: `TestEventMessageScreeningPrivacy`.
- T-24-28: the requirement case of `TestEventMessageScreeningPrivacy`, proven by mutation.
- T-24-SC: no dependency.

## Self-Check: PASSED

- Files: `fuku/modules/event_message_screening_safety_test.go` exists, and every modified file is in the three commits.
- Commits: c4675d8, de1eac4 and 567c50e are ancestors of HEAD. `git rev-list --count aa91689..HEAD` before this summary is 3.
