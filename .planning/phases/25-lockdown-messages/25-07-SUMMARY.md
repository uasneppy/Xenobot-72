---
phase: 25-lockdown-messages
plan: 07
subsystem: lockdown-messages
status: complete
tags: [lockdown, event-messages, settings-menu, safety, privacy, logs, tests]
requires:
  - 25-03 (the Lockdown family on the Messages page)
  - 25-04 (the automatic notice site)
  - 25-05 (the lift notice from every door, its helpers)
  - 25-06 (the surfaces toolkit and its sentinels)
provides:
  - settingsMsgLockdownForged and the Phase 25 entries of settingsMsgValidPresses
  - TestSettingsAuthorityMessagesLockdown, TestSettingsDriftMessagesLockdown
  - settingsPrivacyPhase25Run (in TestSettingsStaffPrivacy)
  - TestLockdownMessageLogsCarryIDsOnly, TestLockdownMessagePrivacy
  - the toolkit lockdownSafeRename, lockdownSafeStore, lockdownSafeBreak, lockdownSafeWant, lockdownSafeAlertLift,
    lockdownSafeLinkStaff, lockdownSafeLogChannel, lockdownSafeWantPrivate and lockdownSafeEveryPlaceholder
affects: [25-08]
tech-stack:
  added: []
  patterns:
    - "one environment drives two lockdown sites in their natural order (a start, then its lift), so a log or privacy sweep covers five sites in three environments"
key-files:
  created:
    - fuku/modules/event_message_lockdown_safety_test.go
  modified:
    - fuku/modules/settings_messages_safety_test.go
    - fuku/modules/settings_messages_lockdown_test.go
    - fuku/modules/settings_staff_privacy_test.go
    - AGENTS.md
decisions:
  - "The log test pairs the sites (the manual start with the lift by command, the join surge with the alert's lift, the group menu's lift alone), each pair once with the read failing and once with the custom photos refused: six environments for ten failures, under the plan's budget."
  - "A group menu's lift after a read error logs the engine's generic 'the default was sent' line though nothing is sent; the log test accepts it (it carries the chat ID and the event only), as plan 25-05 asked."
  - "The log channel has no stored title of its own (logchannels.Set's name is the group's chats row), so the privacy test records the channel's chats row under a sentinel title and searches for that title and the channel's ID."
  - "The privacy test identifies the custom messages by their placeholder separators and checks the {chatname} field by position, so the automatic notice (whose person placeholders name the bot) and the lifts (which name the lifter) are both checked the same way."
  - "AGENTS.md lines went into the commit of the tests they describe (Task 1's page tests in the Task 1 commit, the site tests in the Task 2 commit)."
metrics:
  duration: ~35 min
  completed: 2026-11-22
actuals:
  tokens: 8000
  tasks: 2
  commits: 2
plan_head_before: 1305f045b0bb1df6e404081a7e0eec5f7fe72b56
plan_head_after: a1d84ff0a563f34d2b08dca16f1c716887e0b2ad
---

# Phase 25 Plan 07: The safety of the lockdown messages Summary

The Messages page's and the event engine's safety rules now cover the Lockdown family and its five sites. Every forged press of a
lockdown event, every private copy form included, is refused in step 1 with no member lookup. The Lockdown list and the manual start's
screen create nothing and cost one read. An administrator without `can_change_info` can view and preview but change nothing, and a typist
demoted between Set and the typed message stores nothing. A text stored through the repository is what the page draws, and a page write is
what the next `/lockdown` posts. No log line of the five sites carries content, and no lockdown message names the Staff Group or the log
channel, also when the lift is pressed in the Staff Group. No production defect was found, so no production file changed.

## Tasks

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | the Messages page's safety rules over the Lockdown family | 266dddb | settings_messages_safety_test.go, settings_messages_lockdown_test.go, settings_staff_privacy_test.go, AGENTS.md |
| 2 | the lockdown sites log IDs only and never name the Staff Group | a1d84ff | event_message_lockdown_safety_test.go (new), AGENTS.md |

## What the tests hold

| Test | What it proves | Time (-race) |
|------|----------------|--------------|
| `TestSettingsMessagesForged` (extended) | 9 new valid presses (`hl&t=l`, `he&k=ld`, `hs&k=lj&t=x`, `hr&k=lf&t=m`, `hc&k=la&t=b`, `hv&k=lu&t=x`, `hf&k=ld`, `hq&k=lj&v=1`, `hq&k=lu&v=0`) each pass step 1 with one lookup. Their generated forged forms, plus the 9 of `settingsMsgLockdownForged`, are refused in step 1 with no lookup, edit, row or wait. The 9: the list from a group menu, `he`/`hs`/`hv`/`hf` with `d=p`, `hp` on `lu` and `ld`, `he&k=lu&d=x`, `hq&k=lf&v=2` | 0.28 s (whole test) |
| `TestSettingsMessagesReadsCreateNothing`, `TestSettingsMessagesCost` (extended) | the Lockdown list and the `ld` screen create no row, options row or key, and cost the pipeline's calls and one read | 0.17 s, 0.14 s |
| `TestSettingsAuthorityMessagesLockdown` (new, one environment) | a typist demoted between Set on `lu` and the typed text stores nothing. Without `can_change_info` the list, the `ld` screen and its Full preview open, while Set of `lj`, Remove of a stored `lf` text and Silent of `la` are the `settings_no_change_info` alert with no edit, wait or row change. An administrator the owner gate refuses gets `settings_gate_owner_only`. The live creator's typed `lj` text is stored by them | 0.05 s |
| `TestSettingsDriftMessagesLockdown` (new, one environment) | a stored `lockdown` text is drawn with its length and counted Custom on the hub. A typed `ld` text and a Silent press on `la` are what the cached `eventMessagesLookup` returns at once. The next `/lockdown raid & co` replies with the typed text and the approved-muted note. Opening every lockdown screen of another chat writes nothing | 0.08 s |
| `settingsPrivacyPhase25Run` (new, in `TestSettingsStaffPrivacy`) | an administrator who is not the creator drives the list, the `lu` screen, a typed `lu` text, Silent and a Full preview in a group linked to a Staff Group. Nothing names it | 0.44 s alone (first locale load) |
| `TestLockdownMessageLogsCarryIDsOnly` (new, 6 subtests) | three site pairs (manual start then lift by command; join surge then the alert's lift in the Staff Group; group menu lift), each once with the read failing and once with the custom photos refused. Each failure leaves the engine's Warn naming the chat ID and the event. The group menu's lift sends nothing, and its read-error line is accepted. No line carries the stored text or caption, the file ID, the button, the reason, the trigger sentence, the admin's name or the group's title | 0.70 s |
| `TestLockdownMessagePrivacy` (new, 2 subtests) | a Staff Group and a log channel titled with sentinels, custom texts with every placeholder. A join surge then the alert's Lift pressed in the Staff Group, and `/lockdown` then a group menu's lift: each custom message has the locked group's title as `{chatname}`, and no request to the locked group names the Staff Group's or the log channel's title or ID | 0.16 s |

The new test time is about 1.3 s against the plan's 4 s.

Mutation checks (each reverted):
- Giving `hq` the right `settingsRightAdmin` fails `TestSettingsAuthorityMessagesLockdown`. The Silent press on `la` edited the menu and wrote a row.
- Logging `row.Text` in the engine's refused line fails `TestLockdownMessageLogsCarryIDsOnly` in the three refused subtests.
- Building the button lift's chat with the Staff Group's title in place of `ld.ChatTitle` fails `TestLockdownMessagePrivacy` in both subtests. Both checks catch it: the leak search and the `{chatname}` field.

## Deviations from Plan

None in behaviour. Notes:

- The log test runs five sites in three environments per mode, six in all, by pairing each start with its lift, rather than one
  environment per site. Every site's line is still required by name (`lockdownSafeWant`).
- The log channel has no stored title of its own: `logchannels.Set`'s name argument is the group's chats row. The privacy test therefore
  records a chats row for the channel ID under a sentinel title (`lockdownSafeLogChannel`), so the bot has a title it could leak. The row
  is removed when the test ends.
- AGENTS.md was updated in both task commits, each time with the lines for that commit's tests.

## Production fixes

None. The tests found no defect.

## Edits to earlier tests

Every edit only adds entries. No expected value changed and no assertion was weakened:
- `settings_messages_safety_test.go`:
  - 9 entries in `settingsMsgValidPresses`;
  - the new `settingsMsgLockdownForged`, appended in `settingsMsgForgedPresses`;
  - two opens and one check in the private branch of `TestSettingsMessagesReadsCreateNothing`;
  - two draws in `TestSettingsMessagesCost`.
- `settings_staff_privacy_test.go`: one subtest call in `TestSettingsStaffPrivacy` and the new `settingsPrivacyPhase25Run`.
- `settings_messages_lockdown_test.go`: the two new tests are appended. `TestSettingsMessagesLockdownFamily` is unchanged.

The goldens are unchanged. `git diff --exit-code` exits 0 for each:
- `36d2070`: lockdown
- `e40a7be`: screening
- `9d4554d`: punishment
- `86baf1a`: moderation

`git diff --exit-code d8c98c6 -- go.mod go.sum main.go migrations` also exits 0.

## Verification (with durations)

- Task 1 verify command (9 tests): all PASS, no SKIP. 37.3 s for the package, 80 s wall.
- Task 2 verify command (5 tests): all PASS, no SKIP. 4.0 s for the package, 44 s wall.
- Acceptance greps:
  - `settingsMsgLockdownForged` in the safety file: 2.
  - `settingsMsgEventField, "(ld|lj|lf|la|lu)"` in the safety file: 16.
  - The two test functions in `settings_messages_lockdown_test.go`: 2.
  - `settingsPrivacyPhase25Run(` in the privacy file: 2.
  - The two test functions in the new file: 2.
  - `TestLockdownMessagePrivacy` and `TestSettingsAuthorityMessagesLockdown` in AGENTS.md: 1 each.
- Touched families with `-race`: ok, 82.2 s for the package, 85 s wall. The families: `TestSettingsMessages.*`, the authority and drift
  families, `TestSettingsStaffPrivacy`, `TestLockdown.*`, `TestEventMessage.*`, `TestEventPM.*`, `TestPMCopy.*`, `TestLegacy.*`,
  `TestEventEngine.*`, `TestRaid.*`, `TestSettingsLockdown.*` and `TestUnlockdown.*`.
- Checks: `make check-translations` all present (0.3 s), `make check-docs` no drift (1.3 s), `make test-shard-check` exit 0 (23.7 s),
  `CGO_ENABLED=0 go build ./...` ok (2.2 s).
- golangci-lint v2.13.1:
  - `run --new-from-rev=origin/main ./...`: 0 issues (39 s).
  - The same with `--build-tags=notesttools`: 0 issues (21 s).
  - `--new=false ./fuku/modules/...` with no issue cap: 110 baseline issues, none in a touched file.
- gosec v2.29.0 `-quiet ./fuku/modules/...`: 16 baseline issues, none in a touched file. No non-test file changed.
- Full `go test -tags testtools -race -count=1 ./fuku/modules`: ok, 379.0 s for the package (6 m 22 s wall), no FAIL or SKIP line. Plan 25-06
  measured 374.5 s, so the difference is within runner noise.

## Notes for plan 25-08 (the closing plan)

- Reusable toolkit in `event_message_lockdown_safety_test.go`:
  - `lockdownSafeRename(env)`: sentinel group title and admin name.
  - `lockdownSafeLinkStaff(t, env)`: a Staff Group with a sentinel title, linked, cleaned up.
  - `lockdownSafeLogChannel(t, env)`: a log channel with a sentinel chats-row title.
  - `lockdownSafeAlertLift(t, env, staffChat)`: the worker cycle, the alert's Lift and the card's Confirm in the Staff Group.
  - `lockdownSafeEveryPlaceholder` and `lockdownSafeWantPrivate`.
- A lift starts the triggers' quiet period, so an end-to-end run that wants an automatic start and a lift in one environment must start
  first (as `TestLockdownMessagePrivacy` does).
- The engine's misleading "the default was sent" line after a read error in a group menu's lift is still there. It carries IDs only. A
  wording fix would need the group menu's site to say it has no default; that is out of this plan's scope and left as a possible follow-up.
- `eventLockdownAdapterCallers` stays at five, and no non-test file changed in this plan.
- Phase 25's new tests so far add about 3.5 s in all (25-02 to 25-07) against the 28 s budget.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register:
- T-25-17: `settingsMsgLockdownForged` and the new valid presses.
- T-25-18: `TestLockdownMessageLogsCarryIDsOnly`.
- T-25-19: `TestLockdownMessagePrivacy` and `settingsPrivacyPhase25Run`.
- T-25-20: `TestSettingsAuthorityMessagesLockdown`.
- T-25-SC: no dependency added.

## Self-Check: PASSED

- Files: `fuku/modules/event_message_lockdown_safety_test.go` and this SUMMARY exist.
- Commits 266dddb and a1d84ff are ancestors of HEAD.
- The ledger: `git rev-list --count 1305f04..a1d84ff` is 2.
