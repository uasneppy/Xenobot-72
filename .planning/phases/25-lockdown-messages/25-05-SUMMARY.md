---
phase: 25-lockdown-messages
plan: 05
subsystem: lockdown-messages
status: complete
tags: [lockdown, event-messages, lift, evt-08, evt-10]
requires:
  - 25-02 (lockdownLiftDefaultReply, lockdownLiftNotes, lockdownEventSite, lockdownEventPerson, the engine guard)
  - 25-03 (the Lockdown family on the Messages page, so lu can be set and silenced; lockdownLiftAppended)
  - 25-04 (the paced site pattern)
provides:
  - postLockdownLiftReplyEvent (/unlockdown's reply site) and postLockdownLiftEvent (the button lifts' site, the default switchable off)
  - lockdownLiftReply (the one construction of the lift's eventReply for both sites)
  - lockdownLiftCustomWait (30 s, the pacer wait of a custom button lift notice)
  - lockdownPostLiftNotice now returns (*gotgbot.Message, error) and is the button lift's default send
  - TestEventMessageLockdownLift
  - the five final adapter callers in eventLockdownAdapterCallers
affects: [25-06, 25-07, 25-08]
tech-stack:
  added: []
  patterns:
    - "a site whose default must not post (a group menu's lift) passes a SendDefault that returns (nil, nil); the engine's schedule of nil is a no-op"
    - "the lift's chat for a button lift is built from the lockdown row (ID, supergroup, stored title), never from the pressed message"
key-files:
  created:
    - fuku/modules/event_message_lockdown_lift_test.go
  modified:
    - fuku/modules/event_message_lockdown.go
    - fuku/modules/lockdown.go
    - fuku/modules/lockdown_alert_callbacks.go
    - fuku/modules/settings_lockdown.go
    - fuku/modules/event_engine_callers_test.go
    - AGENTS.md
decisions:
  - "Tasks 1 and 2 went into one feat commit, as Task 2 asks (AGENTS.md in the same commit as the code it describes)."
  - "lockdownPostLiftNotice stays as the button lift's default send and returns (*gotgbot.Message, error) without logging; postLockdownLiftEvent logs today's `could not be posted` line, so TestLockdownAnnounceLift passes unedited."
  - "A custom button lift notice's pacer wait derives from its own lockdownLiftCustomWait (30 s) context, the 25-04 pattern; the default keeps context.Background() (the pacer's MaxWait) and lockdownCallTimeout. No sentence claims a 10 s bound for a custom send."
  - "Both lift sites build their eventReply through lockdownLiftReply, so the default, the person placeholders and the appended facts are one construction (the 25-03 note: Note is lockdownLiftNotes of the facts)."
  - "No eventEngineRootStops entry was needed: no never-reach root (the alert's other buttons, runLockdownCycle, lockdownPostInlineTally) reaches lockdownAnnounceLift or an adapter."
metrics:
  duration: ~45 min
  completed: 2026-11-22
actuals:
  tokens: 11300
  tasks: 2
  commits: 1
plan_head_before: 05060a8917abd36cd645591ab6f6e1ffd47c07f4
plan_head_after: 04b210d3a099cf4fe6bf8ec410cd218cabc59c00
---

# Phase 25 Plan 05: The lift message from every door Summary

A recorded lift now posts the group's own `locklift` message from every door: `/unlockdown` as a reply to the command, the
alert's Lift Confirm and a private settings menu as a new message in the locked group, and a menu opened in the group as a new
message only when one is stored. `{admin}` and the person placeholders name the lifter (the bot for an anonymous admin's
`/unlockdown`), `{chatname}` is the locked group's title, and the manual-change and unbanning lines follow a custom text. A group
with nothing stored gets the plan 25-01 goldens byte for byte, and the group menu still posts nothing beside its redraw.

## Tasks

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | The lift notice through the lockdown adapter from every door | 04b210d | event_message_lockdown.go, lockdown.go, lockdown_alert_callbacks.go, settings_lockdown.go, event_message_lockdown_lift_test.go, event_engine_callers_test.go |
| 2 | AGENTS.md lines for the lift notice | 04b210d | AGENTS.md |

## What was built

- `event_message_lockdown.go`:
  - `lockdownLiftReply(b, tr, actorID, actorName, res, named)`: the `locklift` reply. `Target` is `lockdownEventPerson` (the lifter as
    recorded, or the bot when not named), no `Values` (D-12), `Default`/`Opts` from `lockdownLiftDefaultReply` with the lifter and
    `{ManualChange, Finished, Unbanning}`, `Note` `lockdownLiftNotes` of the same facts.
  - `postLockdownLiftReplyEvent(b, ctx, msg, tr, actor, res lockdownLiftApply)`: the command's chat, a reply to the command in its
    topic, `Actor` the lifter with `Anonymous` from `actorIsAnonymous(ctx)`, `SendDefault` today's `msg.Reply` (empty text sends
    nothing, as `replyLockdown`), a send error logged as `[Lockdown] reply: %v`.
  - `postLockdownLiftEvent(b, ld, actorID, actorName, res, withDefault)`: the chat from the row (`ld.ChatID`, supergroup,
    `ld.ChatTitle`), `staffChatTranslator(ld.ChatID)`, no reply or topic, not `Automatic`; `SendDefault` is
    `lockdownPostLiftNotice` with `withDefault` and `(nil, nil)` without it; `SendCustom` is one `lockdownPaced` unit under its own
    `lockdownLiftCustomWait` context around `media.SendGreetingMessage`; a send error is logged as
    `[Lockdown] lift notice for chat %d could not be posted: %v`. The comments state the bound exactly as D-19 does.
- `lockdown.go` `unlockdown`: `liftApplyDone` goes through `postLockdownLiftReplyEvent` then `lockdownPostInlineTally`; every other
  outcome keeps `replyLockdown(b, msg, lockdownLiftApplyText(...))`.
- `lockdown_alert_callbacks.go`: `lockdownAnnounceLift` wakes the worker first (D-23) and then calls
  `postLockdownLiftEvent(..., true)`; `lockdownPostLiftNotice` returns `(*gotgbot.Message, error)` and no longer logs. Nothing else in
  the file changed; `lockdownAlertCallback` stays at gocyclo 21.
- `settings_lockdown.go` `settingsLockdownLiftYes`: the group branch is `wakeLockdownWorker()` then
  `postLockdownLiftEvent(..., false)`. The private branch is unchanged (`lockdownAnnounceLift`). Doc comment updated.
- `event_engine_callers_test.go`: five `eventEngineCallers` entries (the two adapters, `moduleStruct.unlockdown`,
  `lockdownAnnounceLift`, `settingsLockdownLiftYes`) and three `eventLockdownAdapterCallers` entries, each with its reason, for
  exactly five adapter callers.
- AGENTS.md: the Lockdown page bullet (the group menu's Yes posts a stored custom lifted message), the `chat_lockdowns` bullet (the
  lift notice of every door, the wake first), a lift paragraph in the Event messages bullet (both sites, the facts, the default
  switched off for a group menu and its generic refusal line, the bound), the guard's five adapter callers, and
  `TestEventMessageLockdownLift` in the Phase 25 test families.

## Deviations from Plan

None in behaviour. Two notes:

- **Concurrent commit on the branch.** `05060a8 fix(raid): rename the cannot-lock warning's trigger marker ...` landed on the branch
  after this run started from a clean `a790aa6` and before the 25-05 commit. It was not made by this run. The plan ledger
  (`plan_head_before`) is `05060a8`, so `commits: 1` measures this plan's commit only.
- **`group menu quiet`** runs its two cases (Silent, refused) in two environments inside one subtest, not as nested subtests, so the
  plan's acceptance count of exactly 10 `--- PASS: TestEventMessageLockdownLift/` lines holds.

## Earlier tests whose expected values changed

None. The only edit to an earlier test file is the eight map entries (and one comment line) in `event_engine_callers_test.go`,
which the plan requires. No assertion changed. `git diff --exit-code d8c98c6 -- lockdown_apply_test.go settings_lockdown_lift_test.go
lockdown_alert_lift_test.go` exits 0.

## Mutation checks (done, then reverted)

- Wake after the notice in `lockdownAnnounceLift`: `alert button` fails ("not woken before the lift notice").
- `withDefault` true in the group menu: `group menu quiet` fails (the default sent after the refusal) and the golden `lift_menu_group`
  fails (3 messages, want 2).
- No `Note` on the lift reply: `command notes` fails.

## Verification (with durations)

- The plan's Task 1 verify command (24 tests by name, `-race`, `-v`): all PASS, no SKIP, 7.5 s (10.1 s wall).
  `TestEventMessageLockdownLift` alone: 2.2 s (10 subtests; `alert refused` 1.4 s is the first real-locale load), 0.55 s inside the
  combined run.
- Acceptance greps: `postLockdownLiftReplyEvent(` in lockdown.go 1; `postLockdownLiftEvent(` in lockdown_alert_callbacks.go 1 and in
  settings_lockdown.go 1; no engine entry in any non-test `lockdown*.go`, `raid*.go` or `settings_lockdown.go` (n=0);
  `postLockdownLiftEvent` in AGENTS.md 3; `TestEventMessageLockdownLift` in AGENTS.md 1.
- Goldens unchanged (`git diff --exit-code` exits 0): `36d2070` lockdown, `e40a7be` screening, `9d4554d` punishment, `86baf1a`
  moderation. `git diff --exit-code d8c98c6 -- go.mod go.sum main.go migrations`: no diff.
- Touched families with `-race` (`TestLockdown|TestRaid|TestEventMessage|TestEventPM|TestPMCopy|TestLegacy|TestEventEngine|
  TestShutdown|TestStopRaidPosts|TestSettingsLockdown|TestSettingsParityLockdown|TestUnlockdown|TestApplyLockdown|
  TestSettingsMessagesLockdown|TestCommandRepliesStayInsideTheHandler|TestMemberDeletionExcludedMessages`): ok, 50.4 s.
- `make check-translations`: all present (0.3 s). `make check-docs`: no drift (1.3 s). `make test-shard-check`: exit 0 (23.6 s).
- `CGO_ENABLED=0 go build ./...`: ok (2.1 s).
- golangci-lint v2.13.1: `run --new-from-rev=origin/main ./...` 0 issues (43 s); with `--build-tags=notesttools` 0 issues (22 s);
  `--new=false ./fuku/modules/...` filtered to the touched files: only the two baseline gocyclo lines, `(moduleStruct).lockdown` 21
  and `(moduleStruct).lockdownAlertCallback` 21 (both at or under their ceilings).
- Full `go test -tags testtools -race -count=1 ./fuku/modules`: ok, 357.6 s (361 s wall), no FAIL or SKIP line.

## Known Stubs

None.

## Threat Flags

None. The plan's threat model covers the new surface: T-25-12 (the button lift's chat is built from the row, never the pressed
message), T-25-13 (the wake comes first, proven by `alert button`; the tally follows, proven by `tally after silent`).

## Notes for plan 25-06

- The lift doors for the behavioural guard: `/unlockdown` (reply), the alert's Lift Confirm and a private menu's Lift Yes (both
  through `lockdownAnnounceLift`), and a group menu's Lift Yes (`postLockdownLiftEvent` with the default off). None sends a private
  copy: `locklift` has no PM spec, so `offerReplyPM` returns at once.
- `lockdownAnnounceLift` now wakes the worker before posting. A behavioural test that holds the lift notice with `gateSends` sees the
  wake already in `lockdownWake`; drain it with `lockdownApplyDrainWake()` before the press.
- The group menu's refusal path logs the engine's generic "the default was sent" line though nothing is sent (the default is empty
  by design); plan 25-07's log test should accept it (IDs and event only).
- `event_message_lockdown_lift_test.go` has small helpers a later test can reuse: `lockdownLiftSettingsEnv(t, golden)`,
  `lockdownLiftWantLifted`, `lockdownLiftGroupSends` (refused requests included), `lockdownLiftSilence`,
  `lockdownLiftGroupMenuYes`, `lockdownLiftEditsBeforeNotice`.
- `eventLockdownAdapterCallers` is final at five entries; 25-06 should add no adapter caller.

## Self-Check: PASSED

- Files: `fuku/modules/event_message_lockdown_lift_test.go` and this SUMMARY exist.
- Commit 04b210d is an ancestor of HEAD.
- The ledger: `git rev-list --count 05060a8..04b210d` is 1.
