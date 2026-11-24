---
phase: 23-warn-flood-blacklist-and-forward-blocklist-messages
plan: 06
subsystem: event-messages
status: complete
tags: [event-messages, settings-menu, safety, privacy, logs]
requires:
  - phase: 23-03
    provides: the Automatic family on the Messages page (codes wn wl fl bl fb)
  - phase: 23-04
    provides: the five Automatic send sites through postEvent
  - phase: 23-05
    provides: private copy specs for all twelve events, so every Automatic event has a d=p form and an hp switch
provides:
  - forged, valid, reads and cost coverage of the Automatic family on the Messages page, both forms, hp included
  - TestSettingsAuthorityMessagesAuto
  - settingsPrivacyPhase23Run in TestSettingsStaffPrivacy (with the shared settingsPrivacyMsgRunClean and settingsPrivacyMsgModerator)
  - TestEventMessageAutoLogsCarryIDsOnly and TestEventMessageAutoPrivacy
affects: [23-07]
tech-stack:
  added: []
  patterns:
    - "a Messages page run's privacy check is settingsPrivacyMsgRunClean (edits, sends to the presser and answers, searched for the Staff Group), shared by the Phase 22 and Phase 23 runs"
    - "a privacy check of the forward blocklist searches only send* and edit* requests: getChatMember of the Staff Group's owner is a read the watcher's live authority step must make"
key-files:
  created:
    - fuku/modules/event_message_auto_safety_test.go
  modified:
    - fuku/modules/settings_messages_safety_test.go
    - fuku/modules/settings_messages_auto_test.go
    - fuku/modules/settings_staff_privacy_test.go
    - AGENTS.md
decisions:
  - "The Automatic valid presses spread the five codes over every action code in both forms (one press per code and form, hp on all five), so the generated forged forms (group menu, u in a private menu, extra field, unknown event, unknown part, v=2, d=x and d=g) cover every new code without a cross product; settingsMsgAutoForged adds the planner's forms the generator does not make (hp v=x, an unknown part t=z, an extra key x)."
  - "TestSettingsAuthorityMessagesAuto is one environment: the demoted typist first (listed with can_change_info, Set on fb, then listed without it), then every view and every refusal by the same administrator without can_change_info; the refusals are Set in both forms, Silent and hp on all five events plus Remove of a stored flood text and a stored blacklist private text, and every row is compared whole (text, Silent, updated_at) before and after."
  - "The Phase 22 privacy run's tail and moderator setup became settingsPrivacyMsgRunClean and settingsPrivacyMsgModerator, unchanged in what they check, so the Phase 23 run adds only its presses."
  - "TestEventMessageAutoLogsCarryIDsOnly also covers the warn limit (five sites, the must-have says five) and forbids the fixed flood and forward blocklist reasons besides the planner's sentinels."
  - "TestEventMessageAutoPrivacy runs on newFwdWatchEnv (the plan's key link) rather than newEventPMAutoSite: the point is the real watcher with a linked Staff Group; the Phase 22 toolkit (withFastPMCopyPacer, eventPMCleanup, eventPMDetailsOn, eventPMLogOn, eventPMDetailsTokens, eventPMDetailsRecord) supplies the private copy side, and t.Cleanup(pmCopyJobs.wait) is registered after the pacer swap."
  - "TestEventPMTextsFitEveryLocale needed no edit: plan 23-05 already fills count 100, limit 100, the language's longest action word and eventPMWorstReason for every spec."
metrics:
  duration: ~50 min
  completed: 2026-11-21
actuals:
  tokens: 8500
  tasks: 2
  commits: 2
plan_head_before: 5027ae61e3d48329b6dac7bffabb2d584eef4b86
plan_head_after: 27bf3774408a291a23c7264edebf00615230816d
---

# Phase 23 Plan 06: Automatic family safeguards Summary

The Messages page's forged-press, authority, reads, cost and privacy tests now cover the Automatic family (`wn wl fl bl fb`,
group message and private copy, `hp` included), and two new tests prove that the five sends log IDs only and that a real
forward of a blocked source never names the source or the Staff Group in the group notice, the Details edit and record, the
`#PM_COPY` line or any log line.

## Tasks

| Task | Commit | Files |
|------|--------|-------|
| 1. The page's safety over the Automatic family | a7de444 | settings_messages_safety_test.go, settings_messages_auto_test.go, settings_staff_privacy_test.go, AGENTS.md |
| 2. The sends' logs and the forward blocklist's privacy | 27bf377 | event_message_auto_safety_test.go (new), AGENTS.md |

## What each test proves

- `TestSettingsMessagesForged`: 20 more valid presses (the Automatic list, every action code on the five events in both forms, `hp` on
  all five) and their generated forged forms plus `settingsMsgAutoForged` are each one invalid-request alert with no `getChatMember`, no
  `getChat` and no edit; each valid press makes exactly one lookup of the presser.
- `TestSettingsMessagesReadsCreateNothing` opens the Automatic list, the forward blocklist's event screen and its private copy screen in
  the private menu (the run still ends on a private copy screen); the group menu replays every new forged press; no event message,
  options, chats or rules row and no `event_messages` key is created.
- `TestSettingsMessagesCost`: the three new draws are each one read and the pipeline's four calls, also with Redis down.
- `TestSettingsAuthorityMessagesAuto` (new): see the decisions above.
- `TestSettingsStaffPrivacy` / `settingsPrivacyPhase23Run` (new): an administrator who is not the creator goes through the Automatic list,
  the forward blocklist's private copy and event screens, Set and a typed text, Silent and Full preview; the forward blocklist row holds the
  text and Silent on; no edit, sent message or answer names the Staff Group.
- `TestEventMessageAutoLogsCarryIDsOnly` (new): five sites x {messages unreadable, custom photo refused}; each logs the Warn naming the chat
  and the event, and no line carries the stored text or caption, file ID, button name or link, typed reason, matched word, member's names
  or the fixed reasons.
- `TestEventMessageAutoPrivacy` (new): custom and defaults; the member is muted once, the group notice, one Details edit, one Details record
  (the member's copy) and one `#PM_COPY` post exist, and the sentinel Staff Group title, the sentinel source title, the listed source's stored
  title and both chat IDs appear in no send or edit, the Details record or any log line.

Both new log and privacy tests were checked against a mutation: logging `r.Default` in the refused-message Warn failed the log test at all
five sites, and a stored private copy carrying the source sentinel failed the privacy test on the request and on the Details record.

## Deviations from Plan

None in production: no safeguard found a defect, so `event_message.go`, `event_message_pm.go` and `settings_messages.go` are unchanged.

- The forged and valid press tables follow the planner's list in substance; `hl t=a` from a group menu, `he k=zz`, `he k=fb d=x`, `hq k=wl
  v=2`, `hp k=fl v=x`, `hs k=wn t=z` and `hf k=bl` with an extra key are all present, generated or explicit; the valid set is wider (every
  code on the new events in both forms) to satisfy the orchestrator's coverage note.
- `TestEventPMTextsFitEveryLocale` was not edited (already complete since 23-05).
- `newEventPMAutoSite` was not used: the log test needs no private copy (it runs on `newNoticeGoldenSite` like the sites tests) and the
  privacy test runs on `newFwdWatchEnv` per the plan's key link.

## Earlier tests whose expected sets changed

- `settingsMsgValidPresses` gained 20 entries and `settingsMsgForgedPresses` starts from `settingsMsgAutoForged`: they feed
  `TestSettingsMessagesForged`, `TestSettingsMessagesReadsCreateNothing` and `TestSettingsMessagesCost`; no assertion changed.
- `TestSettingsMessagesReadsCreateNothing` and `TestSettingsMessagesCost` open three more screens; no assertion changed.
- `settingsPrivacyPhase22Run` was refactored onto `settingsPrivacyMsgModerator` and `settingsPrivacyMsgRunClean` with the same wants and
  leak list (the only change is the phase name in the error text).
- No golden changed: `git diff --exit-code 9d4554d -- fuku/modules/punishment_notice_legacy_request_test.go` and the same for
  `moderation_legacy_request_test.go` exit 0.

## Gates

| Gate | Result | Wall time |
|------|--------|-----------|
| Task 1 verify (7 tests, -race) | PASS, no skip | 53 s (StaffPrivacy 38.4 s) |
| Task 2 verify (6 tests, -race) | PASS, no skip | 11 s |
| Touched families (-race: TestSettingsMessages*, TestSettingsAuthorityMessages*, TestSettingsDriftMessages*, TestSettingsStaffPrivacy, TestEventMessage*, TestEventPM*, both legacy goldens, TestPMCopyCallers, TestPMCopyDetailsDeepLink, TestMessagesEndToEnd) | PASS | 2 m 4 s |
| make check-translations | PASS | 0.3 s |
| make check-docs | PASS | 1.1 s |
| make test-shard-check | PASS | 22 s |
| CGO_ENABLED=0 go build ./... | PASS | 2.2 s |
| golangci-lint v2.13.1 --new-from-rev=origin/main ./... | 0 issues | 37 s |
| same with --build-tags=notesttools | 0 issues | 19 s |
| --new=false ./fuku/modules/... | baseline findings only, none in a touched file | 1 s |
| go.mod, go.sum, main.go, migrations vs 1a382c3 | unchanged | |

## New test timings (-race)

| Test | Time |
|------|------|
| TestSettingsAuthorityMessagesAuto | 0.47 s |
| TestEventMessageAutoLogsCarryIDsOnly | 1.31 s |
| TestEventMessageAutoPrivacy | 0.81 s |
| TestSettingsStaffPrivacy / Phase 23 run | 0.39 s |
| TestSettingsMessagesForged (extended) | 0.53 s (0.45 s before) |
| TestSettingsMessagesReadsCreateNothing (extended) | 0.50 s (0.46 s before) |
| TestSettingsMessagesCost (extended) | 0.40 s (0.44 s before) |

About 3.1 s added to the package in all.

## Notes for plan 23-07

- D-31's check of the forward blocklist's Details record and log line is now done by `TestEventMessageAutoPrivacy` through the real watcher;
  the end-to-end test need not repeat it.
- No defect was found; the five sends, the page and the private copies need no production fix before the closing run.
- The full `fuku/modules` package was not run here; 23-07 records its time (this plan adds about 3 s).

## Known Stubs

None.

## Threat Flags

None. T-23-16: `TestEventMessageAutoLogsCarryIDsOnly`. T-23-17: the extended forged and valid press tables and
`TestSettingsAuthorityMessagesAuto`. T-23-18: `TestEventMessageAutoPrivacy`. T-23-SC: no dependency change.

## Self-Check: PASSED

- FOUND: fuku/modules/event_message_auto_safety_test.go
- FOUND: a7de444, 27bf377 (ancestors of HEAD)
