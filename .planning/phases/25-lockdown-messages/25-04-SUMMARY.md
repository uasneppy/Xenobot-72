---
phase: 25-lockdown-messages
plan: 04
subsystem: lockdown-messages
status: complete
tags: [lockdown, raid, event-messages, automatic-notice, evt-07, evt-10]
requires:
  - 25-02 (lockdownAutoDefaultReply, lockdownStartNotes, lockdownEventSite, lockdownEventPerson, {percent}, the engine guard)
  - 25-03 (the Lockdown family on the Messages page, so lj, lf and la can be set and silenced)
provides:
  - postLockdownAutoEvent (the automatic lockdown notice's site, in event_message_lockdown.go)
  - raidPostNotice reduced to the one adapter call
  - TestEventMessageLockdownAuto
  - the guard entries event_message_lockdown.go postLockdownAutoEvent and raid_trigger.go raidPostNotice
affects: [25-05, 25-07, 25-08]
tech-stack:
  added: []
  patterns:
    - "a paced lockdown site sets SendCustom to one lockdownPaced unit under its own raidPostTimeout around media.SendGreetingMessage"
    - "the default send ignores the builder's options and makes today's call (raidSend), so the golden request is unchanged"
key-files:
  created:
    - fuku/modules/event_message_lockdown_auto_test.go
  modified:
    - fuku/modules/event_message_lockdown.go
    - fuku/modules/raid_trigger.go
    - fuku/modules/event_engine_callers_test.go
    - AGENTS.md
decisions:
  - "Tasks 1 and 2 went into one feat commit, as Task 2 asks (AGENTS.md in the same commit as the code it describes)."
  - "An unknown trigger kind returns before any read through the adapter's own kind-to-event map (lockdownAutoEvents), so it posts nothing and reads nothing, as raidPostNotice's default branch did."
  - "No eventEngineRootStops entry was needed: raidWarnCannotLock and runLockdownCycle do not reach raidPostNotice or the adapter."
metrics:
  duration: ~35 min
  completed: 2026-11-22
actuals:
  tokens: 6400
  tasks: 2
  commits: 1
plan_head_before: 838061bfcb65e6b7ac413fef0e26bcd9d1caa33b
plan_head_after: 3111440b9311c26869b7322acfeabaa0513be220
---

# Phase 25 Plan 04: The automatic lockdown notices through the engine Summary

When a join surge, a flood or an AI burst locks a group, the group now gets its own stored message for that trigger
(`lockjoin`, `lockflood`, `lockai`) with `{count}`, `{window}`, the flood's `{percent}` and the trigger sentence as `{reason}`
filled and escaped, `{admin}` reading "an admin" and the person placeholders naming the bot, followed after one line break
by the technical notes. The send stays inside the post `raidPostAsync` counts and `StopRaidPosts` drains. A group with nothing
stored gets the plan 25-01 goldens byte for byte, and the cannot-lock warning is untouched.

## Tasks

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | The automatic notice through the lockdown adapter, inside the same tracked post | 3111440 | event_message_lockdown.go, raid_trigger.go, event_message_lockdown_auto_test.go, event_engine_callers_test.go |
| 2 | AGENTS.md lines for the automatic notice and the drain | 3111440 | AGENTS.md |

## What was built

- `postLockdownAutoEvent(b, chat, trig, res)` in `event_message_lockdown.go`:
  - It maps the kind to the event through `lockdownAutoEvents`. Any other kind returns at once.
  - The site: the trigger's chat, `staffChatTranslator`, no actor, no reply or topic, `Automatic`, and the do-nothing
    `Schedule` of `lockdownEventSite`.
  - The default: `SendDefault` calls `raidSend(b, chat.Id, text)` and ignores the builder's options, so the request is
    today's.
  - A custom message: `SendCustom` runs one `lockdownPaced` unit under its own `raidPostTimeout` context around
    `media.SendGreetingMessage`.
  - The reply:
    - `Target`: the bot (`lockdownEventPerson(b, b.Id, "", false)`).
    - `Values`: `Reason` is `lockdownTriggerSentence`, plus `Count`, `Window` (`strconv.Itoa`) and `Percent`.
    - `Default` and `Opts`: from `lockdownAutoDefaultReply(kind)`, with the trigger's numbers and `lockdownStartFacts(res.BotMember)`.
    - `Note`: `lockdownStartNotes` of those facts.
  - A send error is logged with today's `[RaidDetection] ... was not delivered` line.
  - The comment states the bound exactly as D-19 does. The default keeps `raidSend`'s 10 s call timeout. The custom request
    has no context, so once in flight only the bot's HTTP client timeout (30 s) bounds it. `StopRaidPosts` gives up after
    `raidPostStopWait`.
- `raidPostNotice` keeps its signature. Its doc comment now says the message may be the group's own. Its body is the one
  adapter call. `raidFire`, `raidPostAsync`, `StopRaidPosts`, `raidSend` and `raidWarnCannotLock` did not change.
  `raid_trigger.go` names no engine entry.
- `eventEngineCallers` gains `event_message_lockdown.go: postLockdownAutoEvent` and `raid_trigger.go: raidPostNotice`.
  `eventLockdownAdapterCallers` gains `raid_trigger.go: raidPostNotice`. Each entry has its reason.
- AGENTS.md changes:
  - the `StopRaidPosts` bullet;
  - the automatic announcement bullet under Data;
  - a new paragraph in the Event messages bullet about the automatic site;
  - the guard's adapter caller list;
  - `TestEventMessageLockdownAuto` in the Phase 25 test families.

## Deviations from Plan

None. The plan was executed as written.

## Earlier tests whose expected values changed

None. The only edit to an earlier test file is the two map entries in `event_engine_callers_test.go`, which the plan
requires. No assertion changed and no golden was touched.

## Verification (with durations)

- The plan's Task 1 verify command (14 tests by name, `-race`): all PASS in 7.2 s; `TestEventMessageLockdownAuto` 1.75 s.
- `TestEventMessageLockdownAuto -v`: 7 `--- PASS` subtests. When run alone, `refused` takes 1.43 s (the first real-locale load)
  and the other six about 0.6 s together.
- Acceptance greps:
  - `func postLockdownAutoEvent(` in `event_message_lockdown.go`: 1.
  - `postLockdownAutoEvent(` in `raid_trigger.go`: 1.
  - `lockdown_auto_started_` in `raid_trigger.go`: 0.
  - `lockdown_auto_cant_lock_` in `raid_trigger.go`: 3.
  - No engine entry in any non-test `lockdown*.go` or `raid*.go`: count 0.
  - `postLockdownAutoEvent` in AGENTS.md: 4 lines, one of them naming the HTTP client bound and none claiming a 10 s bound.
  - `TestEventMessageLockdownAuto` in AGENTS.md: 2.
- `git diff --exit-code d8c98c6 -- go.mod go.sum main.go migrations`: no diff, so no drain or handler was registered.
- Goldens unchanged (`git diff --exit-code` exits 0 for each):
  - `36d2070 -- lockdown_message_legacy_request_test.go`
  - `e40a7be -- screening_notice_legacy_request_test.go`
  - `9d4554d -- punishment_notice_legacy_request_test.go`
  - `86baf1a -- moderation_legacy_request_test.go`
- Touched families with `-race`: `^(TestLockdown|TestRaid|TestEventMessage|TestEventPM|TestPMCopy|TestLegacy|TestEventEngine|TestShutdown|TestStopRaidPosts|TestSettingsMessagesLockdown|TestCommandRepliesStayInsideTheHandler|TestMemberDeletionExcludedMessages)` ok in 48.5 s.
- `make check-translations`: all present (0.3 s).
- `make check-docs`: no drift (1.3 s).
- `make test-shard-check`: exit 0 (21.9 s).
- `CGO_ENABLED=0 go build ./...`: ok (1.8 s).
- golangci-lint v2.13.1:
  - `run --new-from-rev=origin/main ./...`: 0 issues (38 s).
  - The same with `--build-tags=notesttools`: 0 issues (20 s).
  - `--new=false` on `./fuku/modules/...`: no finding in any touched file, so gocyclo did not rise. `raidPostNotice` lost its
    switch, and the new function has none.
- Full `go test -tags testtools -race -count=1 ./fuku/modules`: ok, 356.1 s (359 s wall), no FAIL or SKIP line.

## Known Stubs

None.

## Threat Flags

None. The plan's threat model already covers the new surface:
- T-25-09 is covered by the `drain`, `silent` and `refused` subtests.
- T-25-10 is covered by the `join surge` subtest: `{admin}` reads "an admin" and `{mention}` names the bot.
- T-25-11: `raidWarnCannotLock` is untouched and is a never-reach root.

## Notes for plan 25-05

- Pattern for a paced site: set `site.SendCustom` after `lockdownEventSite(...)` to one `lockdownPaced` unit under its own
  timeout around `media.SendGreetingMessage`. Let `SendDefault` make today's exact call and ignore the builder's options.
  `lockdownPostLiftNotice`'s options must stay byte for byte (golden `lift_alert`, `lift_menu_private`).
- `eventLockdownAdapterCallers` now has `lockdown.go: moduleStruct.lockdown` and `raid_trigger.go: raidPostNotice`. 25-05 adds
  `moduleStruct.unlockdown`, `lockdownAnnounceLift` and `settingsLockdownLiftYes`, plus the two adapters in
  `eventEngineCallers`, for exactly five adapter callers.
- `lockdownAnnounceLift` is reached today from `lockdown_alert_callbacks.go:360` (the Lift Confirm) and `settings_lockdown.go:364`.
  Before adding the adapter, re-check that none of the alert's other buttons (`lockdownLiftAsk`, `lockdownBanAsk`/`Confirm`,
  `lockdownRevokeAsk`/`Confirm`, `lockdownCardCancel`) and neither `runLockdownCycle` nor `lockdownPostInlineTally` names it,
  even through a shared helper. The walk follows bare names and function values. If a root legitimately reaches it, add an
  `eventEngineRootStops` entry with a reason; never drop the root.
- `TestEventMessageLockdownAuto`'s `refused` subtest is the slow one (1.4 s when the test runs alone), because it is the
  first real-locale load of the run. The other six take about 0.6 s together.

## Self-Check: PASSED

- Files: `fuku/modules/event_message_lockdown_auto_test.go` and `fuku/modules/event_message_lockdown.go` exist.
- Commit 3111440 is an ancestor of HEAD.
- The ledger: `git rev-list --count 838061b..3111440` is 1.
