---
phase: 25-lockdown-messages
plan: 06
subsystem: lockdown-messages
status: complete
tags: [lockdown, event-messages, evt-10, pm-copies, behavioural-guard]
requires:
  - 25-02 (TestEventEngineCallers, the structural half of EVT-10)
  - 25-04 (the automatic notice site)
  - 25-05 (the lift notice from every door, its helpers)
provides:
  - TestLockdownMessagesLeaveOtherSurfacesAlone (5 subtests)
  - TestLockdownEventsSendNoPrivateCopy (5 subtests)
  - the toolkit lockdownStoreEverySurfaceMessage, lockdownWantNoSentinel, lockdownWantNoPrivateSend, lockdownSurfaceCalls,
    lockdownCallsTo, lockdownWantNoPMCopyTrace and the sentinels lockdownSurfaceSentinel and lockdownSurfaceLabel
  - TestEventPMSpecs holding the five lockdown keys spec-less
affects: [25-07, 25-08]
tech-stack:
  added: []
  patterns:
    - "a sentinel search over every recorded request, each parameter rendered with lockdownParamText, so a keyboard or caption leak is found too"
    - "every case proves it reached its surface before it checks for the sentinel"
key-files:
  created:
    - fuku/modules/lockdown_message_surfaces_test.go
  modified:
    - fuku/modules/event_message_pm_test.go
    - AGENTS.md
decisions:
  - "The private copy subtests store a group message too (the sentinel), so each proves its site posted by finding the stored event message in the group, then checks that nothing went to a positive chat_id or from_chat_id, that no keyboard was edited (the Details button's path) and that no request carries pmcopy_ or #PM_COPY."
  - "The no-spec subtest has a control: a ban with its private copy row on makes eventPMWouldOffer true on the same site and messages, so the five false answers for the lockdown keys are not vacuous."
  - "The lockdown flow checks the group differently from the Staff Group and the log channel: in the group exactly two sends carry the sentinel (the start reply, then the lift notice, each the stored text alone or followed by its notes), and every other group request (the tally, bans, deletes, edits) carries none."
metrics:
  duration: ~40 min
  completed: 2026-11-22
actuals:
  tokens: 6200
  tasks: 2
  commits: 2
plan_head_before: 65cb6742337c9c41924e48d6b84b69230aa1c972
plan_head_after: b0669ccd56ffdda1eb85c64ab053d69ad6f02f92
---

# Phase 25 Plan 06: The behavioural half of EVT-10 Summary

With every lockdown message stored (a sentinel text and a sentinel URL button, for the locked group, its Staff Group and its
log channel, plus a switched-on private copy row with both options on), the running bot was driven through every surface that
is not a lockdown event: the alert's two copies, a reminder, a "Ban N recent joiners" run with its card report and its
`#LOCKDOWN_BAN` post, the alert's Lift card, the alert's close at the lift, the lift tally, the cannot-lock warning, a staff
`/mute` card and summary, a captcha challenge and a log post. None carries a sentinel. In the group the sentinel appears in
exactly two messages, the start reply and the lift notice, and nothing goes to a private chat. No lockdown event has a private
copy spec, and a manual start, an automatic start and a lift by `/unlockdown` and by the alert's button send nothing in private,
edit no keyboard and post no `#PM_COPY` line.

## Tasks

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | every non-event surface stays as it is with every lockdown message stored | 6c82418 | lockdown_message_surfaces_test.go (new) |
| 2 | no lockdown event ever sends a private copy | b0669cc | lockdown_message_surfaces_test.go, event_message_pm_test.go, AGENTS.md |

## What was built

- `TestLockdownMessagesLeaveOtherSurfacesAlone`:
  - `lockdown flow`: one `banEnv` plus a log channel (`setStaffLogChannel`). A joiner before the lock, `/lockdown raid wave`, a
    raider joining. One cycle posts the alert to both destinations. `ageAlert` makes a reminder due, and one cycle posts it to
    both. `lastAlert`, `confirmBan` and `cyclesUntilReported` deliver the card report (`lockdown_ban_result`) and one
    `#LOCKDOWN_BAN` post. The alert's `lk` and the card's `lc` lift it, and `cycleUntilIdle` closes both alert copies (each
    `closed_at` set, and the Staff Group copy's last edit carries the lifted footer). The tally is posted in the group. Then the
    sentinel search runs over every recorded request.
  - `cannot lock`: `warnEnv` with the bot not allowed to restrict, then `raidFire` of a join surge. There is no lockdown, one
    warning in each destination and nothing in the group.
  - `staff action`: `newStaffActionEnv(t, 2)` and `/mute 4242 flooding`, with both groups muted and a summary.
  - `captcha challenge`: `SendCaptcha` on a `newBotSite` with the captcha on.
  - `log post`: `actionlog.Admin` with exactly one request to the channel.
- `TestLockdownEventsSendNoPrivateCopy`, one `lockdownNoCopyEnv` per subtest. The environment has every lockdown message stored
  with private copies on, a log channel, the bot as a full administrator, a fast private copy pacer and the jobs awaited:
  - `no spec`
  - `manual start` (`/lockdown`)
  - `automatic start` (a flood `raidFire`)
  - `lift by command` (`/unlockdown`)
  - `lift by button` (`askToLiftLockdown` and Confirm)
- `TestEventPMSpecs`: an added loop holds `eventPMSpecFor` false for the five lockdown keys. The count of eighteen is unchanged.
- AGENTS.md:
  - The PM copies bullet now says the five lockdown events have no spec and never will.
  - Both tests are listed in the Phase 25 test families as the behavioural half of EVT-10, beside `TestEventEngineCallers`.

## Deviations from Plan

None in behaviour. Notes:

- `lockdownStoreEverySurfaceMessage(t, chatIDs...)` also turns both private copy options on for every chat, as Task 2 asks.
  Task 1's cases gain those options too, which only widens what they cover.
- The toolkit has three small helpers beyond the plan's four: `lockdownSurfaceCalls` (every request, read once the raid posts and
  the private copy jobs are done), `lockdownCallsTo` and `lockdownWantNoPMCopyTrace`. The plan's helper names are kept.
- The plan's planner assumption asked for `setStaffLogChannel` with the admin category on. It turns every category on, the same
  helper the earlier lockdown tests use.

## Production fixes

None. The tests found no defect.

## Earlier tests whose expected values changed

None. The one edit to an earlier test is an added assertion loop in `TestEventPMSpecs`, which leaves its count of eighteen
unchanged. No earlier assertion was weakened.

## Mutation checks (done, then reverted)

- Registering a private copy spec for `locklift` and `lockdown` in `event_message_pm.go`: `no spec` fails on both keys, and `manual
  start`, `lift by command` and `lift by button` fail with a `sendMessage` to the actor's private chat.
- Appending the stored `lockdown` text to the reminder body (`lockdownRenderReminder`) and the stored `locklift` text to the tally
  (`lockdownPostTally`): `lockdown flow` fails on the Staff Group's and the log channel's reminder sends and edits, and on a third
  sentinel message in the group (the tally).

## Verification (with durations)

- Task 1 verify command (7 tests by name, `-race`, `-v`): all PASS, no SKIP, 4.0 s (45 s wall with the build).
- Task 2 verify command (5 tests by name, `-race`, `-v`): all PASS, no SKIP, 4.1 s (45 s wall).
  - `TestLockdownMessagesLeaveOtherSurfacesAlone`: 0.75 s (`lockdown flow` 0.54 s).
  - `TestLockdownEventsSendNoPrivateCopy`: 0.15 s.
  - Together about 0.9 s, under the plan's 4 s budget.
- Acceptance greps:
  - 5 `--- PASS: TestLockdownMessagesLeaveOtherSurfacesAlone/` lines and 5 `--- PASS: TestLockdownEventsSendNoPrivateCopy/` lines.
  - `LOCKDOWN_BAN` in the new file: 3. `ageAlert(` in the new file: 1.
  - `EventLock` in `event_message_pm_test.go`: 5.
  - `TestLockdownEventsSendNoPrivateCopy` in AGENTS.md: 2. `TestLockdownMessagesLeaveOtherSurfacesAlone` in AGENTS.md: 1.
  - `git diff --name-only 65cb674..HEAD -- fuku/modules ':!fuku/modules/*_test.go'`: empty.
- Goldens unchanged (`git diff --exit-code` exits 0 for each):
  - `36d2070`: lockdown
  - `e40a7be`: screening
  - `9d4554d`: punishment
  - `86baf1a`: moderation
- `git diff --exit-code d8c98c6 -- go.mod go.sum main.go migrations`: no diff.
- `make check-translations`, `make check-docs`, `make test-shard-check` (exit 0) and `CGO_ENABLED=0 go build ./...`: all ok, 28 s together.
- golangci-lint v2.13.1:
  - `run --new-from-rev=origin/main ./...`: 0 issues (26 s).
  - The same with `--build-tags=notesttools`: 0 issues (22 s).
  - `--new=false` on `./fuku/modules/...`, with and without `testtools`: no finding in either touched file.
- gosec v2.29.0 `-quiet ./fuku/modules/...`: 16 baseline issues, none in a touched file. No non-test file changed.
- Touched families with `-race` (`TestLockdown|TestRaid|TestEventMessage|TestEventPM|TestPMCopy|TestLegacy|TestEventEngine|
  TestDeletionExcluded|TestShutdown|TestStopRaidPosts|TestSettingsMessagesLockdown|TestCommandRepliesStayInsideTheHandler|
  TestMemberDeletionExcludedMessages`): ok, 51.5 s (54 s wall).
- Full `go test -tags testtools -race -count=1 ./fuku/modules`: ok, 374.5 s (377 s wall), no FAIL or SKIP line. The plan 25-05 run
  took 357.6 s; the two new tests add about 1 s, and the rest is runner noise.

## Known Stubs

None.

## Threat Flags

None. The tests cover T-25-15 and T-25-16, and they add no production surface.

## Notes for plan 25-07

- The toolkit can be reused by name:
  - `lockdownStoreEverySurfaceMessage(t, chatIDs...)` stores all five lockdown events (a sentinel text and button), the private copy
    rows with the copies switched on, and both options. Its cleanup is `eventPMCleanup`.
  - `lockdownSurfaceCalls(client)` returns every request, read after the raid posts and the private copy jobs.
  - `lockdownWantNoSentinel`, `lockdownWantNoPrivateSend`, `lockdownCallsTo` and `lockdownWantNoPMCopyTrace`.
  - `lockdownSurfaceSentinel` and `lockdownSurfaceLabel` are a ready text and label for a log sweep: a log line must never carry
    either.
- `lockdownNoCopyEnv(t)` is a lockdown environment with every lockdown message stored, private copies on, a log channel and the bot
  as a full administrator. It suits a log sweep over the start, automatic and lift sites.
- Test time: the phase's new tests so far add about 1 s to the 28 s budget for this plan (D-39).
- No adapter caller was added. `eventLockdownAdapterCallers` stays at five, and no non-test file changed in this plan.

## Self-Check: PASSED

- Files: `fuku/modules/lockdown_message_surfaces_test.go` and this SUMMARY exist.
- Commits 6c82418 and b0669cc are ancestors of HEAD.
- The ledger: `git rev-list --count 65cb674..b0669cc` is 2.
