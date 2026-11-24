---
phase: 25-lockdown-messages
plan: 01
subsystem: testing
status: complete
tags: [golden, characterization, lockdown, raid, settings, deletion]
requires: []
provides:
  - TestLegacyLockdownMessageRequests (12 subtests)
  - the goldenLockdown* constants (12)
  - newLockdownGoldenEnv, newLockdownGoldenSettingsEnv, lockdownGoldenTokens, lockdownGoldenRequest, lockdownGoldenWant, lockdownGoldenWantSends, lockdownGoldenNoQueue, lockdownGoldenNoPrivate, lockdownGoldenSettingsButton, lockdownGoldenWantLifted
affects: [25-02, 25-04, 25-05, 25-08]
tech-stack:
  added: []
  patterns:
    - "a golden recorded by running the test once with empty constants and pasting each %q `got` value, then committed alone before any production change"
    - "every deletion rule of the group at its shortest delay (deletionShortestRules), so a lockdown message that entered the queue would leave a row"
    - "chat and user IDs replaced by tokens, longest decimal first, so an ID inside a longer one is never matched there"
key-files:
  created:
    - fuku/modules/lockdown_message_legacy_request_test.go
  modified: []
decisions:
  - "12 subtests, one per case of the plan's behavior list; the plan's acceptance count of 13 does not match its own list (12 cases, cannot_lock pins two constants in one case), so no case was invented to reach it."
  - "The anonymous tapper is a live administrator with the restrict right who is listed in getChatAdministrators (listAdministrator), as the plan asks, not newAnonLockdownEnv's creator."
  - "The menu cases find the Lift (a=ls) and Yes (a=ly) buttons by their decoded settings data, because the real locale files carry no markers."
  - "No AGENTS.md edit: the plan leaves the Testing sentence to 25-02, together with the first change it guards."
metrics:
  duration: ~35 min
  completed: 2026-11-22
actuals:
  tokens: 5800
  tasks: 2
  commits: 1
plan_head_before: 5ce9dcded59c4b7071309a9d34e400e2b9072e98
plan_head_after: 36d20708319bfdf156a7ee5bd8950ed146bc477b
---

# Phase 25 Plan 01: Lockdown message goldens Summary

`TestLegacyLockdownMessageRequests` pins, on the code as it is before Phase 25, the exact `sendMessage` request of the `/lockdown`
success reply (all rights; a reason with the delete and invite rights missing; after an anonymous admin's proof tap), the automatic
lockdown's public notice (join surge; flood without the delete right; AI burst without the invite right), the cannot-lock warning's
Staff Group and log channel copies, and the lift notice from `/unlockdown` (finished; with a manual change and two joiners being
unbanned), from the alert's Lift Confirm and from a private settings menu, in Ukrainian with the real locales. A lift from a menu
opened in the group is pinned as sending no message beyond the menu's own edits. Every case checks that no deletion queue row exists
for the group with every rule at its shortest delay, and every case but the private menu's that no request went to a private chat.
It was committed alone (36d2070) before any send site changes.

## What later plans use

| Kind | Name |
|------|------|
| Test | `TestLegacyLockdownMessageRequests` in `fuku/modules/lockdown_message_legacy_request_test.go`, subtests `start start_reason_no_rights start_anonymous auto_join_surge auto_flood_no_delete auto_ai_no_invite cannot_lock lift_command lift_command_unbanning lift_alert lift_menu_private lift_menu_group` |
| Constants | `goldenLockdownStart`, `goldenLockdownStartReasonNoRights`, `goldenLockdownStartAnonymous`, `goldenLockdownAutoJoinSurge`, `goldenLockdownAutoFloodNoDelete`, `goldenLockdownAutoAINoInvite`, `goldenLockdownCannotLockStaff`, `goldenLockdownCannotLockLog`, `goldenLockdownLiftCommand`, `goldenLockdownLiftCommandUnbanning`, `goldenLockdownLiftAlert`, `goldenLockdownLiftMenuPrivate` (the request JSON with `<chat>`, `<admin>`, `<tapper>`, `<staff>`, `<log>`) |
| Envs | `newLockdownGoldenEnv(t) *lockdownEnv` (`newRealLocaleLockdownEnv`, `raidCleanup`, `deletionShortestRules` of the chat); `newLockdownGoldenSettingsEnv(t) *settingsEnv` (`newSettingsEnvWith(RealLocale)`, the same two) |
| Helpers | `lockdownGoldenTokens(raw, map[int64]string)`, `lockdownGoldenRequest(t, env, chatID, n)` (the n-th sendMessage, from 0, through `legacyRequestJSON`), `lockdownGoldenWant(t, what, got, ids, want)`, `lockdownGoldenWantSends(t, env, chatID, n)`, `lockdownGoldenNoQueue(t, chatID)`, `lockdownGoldenNoPrivate(t, env)` (no recorded request with a positive `chat_id`), `lockdownGoldenSettingsButton(t, keyboard, act)`, `lockdownGoldenWantLifted(t, env, row)`, and the constant `lockdownGoldenChangedPerms` |

The start cases also check the lockdown is confirmed (and, after the proof, started by the tapper); the automatic cases that it is
confirmed with the trigger's kind and the group got exactly one message; `cannot_lock` that no lockdown exists, the group got nothing
and the Staff Group and the log channel one message each; the lift cases that the row was lifted (the unbanning case also that
`manual_change` was recorded) and the group got exactly two messages (the `/lockdown` reply, then the lift notice); `lift_menu_group`
that the group had exactly two messages before the Yes (the `/lockdown` reply and the `/settings` choice message that became the menu),
none after, and that the menu was edited.

## What the goldens record (byte quirks the default builders must keep)

- **`/lockdown` reply** (`replyLockdown`, `msg.Reply` with `formatting.Shtml()`): HTML, `link_preview_options` `{"is_disabled":true}`,
  `reply_parameters` `{"message_id":<command>,"allow_sending_without_reply":true}`, no keyboard, no thread. Lines joined by `\n`:
  `lockdown_started`, `lockdown_note_approved_muted`, `lockdown_started_by` with the mention (`<a href="tg://user?id=<admin>">Ad&lt;b&gt;min</a>`),
  then `Причина: raid &amp; &lt;b&gt;spam&lt;/b&gt;` when a reason was given, then `lockdown_note_no_delete`, then
  `lockdown_note_no_invite`, each only when the bot lacks that right. The order is fixed: started, approved-muted, started-by, reason,
  no-delete, no-invite.
- **After the anonymous proof**: the same reply, replying to the cached command message (101), naming the tapper (`Tap&lt;per`).
- **Automatic notice** (`raidPostNotice` through `raidSend`): HTML, link previews off, no `reply_parameters`, no thread, no keyboard.
  Lines: the trigger's intro (`lockdown_auto_started_join_surge` with `60` and `15`, `_flood` with `30`, `30`, `80%`, `_ai_burst` with
  `120` and `5`), `lockdown_note_approved_muted`, then the no-delete and no-invite notes as for the command.
- **Cannot-lock warning** (`raidWarnCannotLock`): the Staff Group copy is `⚠️ Виявлено рейд у <b>Lock Chat</b> (Наплив вступів: 15 за
  60 с.), але ...` with the same options as the notice; the log channel copy is the same text after the header
  `<b>Lock Chat</b> (<code><chat></code>)\n`.
- **Lift notice from `/unlockdown`** (`replyLockdown`): HTML, link previews off, `reply_parameters` to the `/unlockdown` message (102).
  Lines: `lockdown_lifted` with the lifter's mention, then `lockdown_lifted_manual_change` when the live permissions differed from the
  locked set, then `lockdown_lifted_unbanning` with the count (2) while joiners are left to unban.
- **Lift notice from the alert's Lift Confirm and from a private menu's Yes** (`lockdownAnnounceLift`, `lockdownPostLiftNotice`): the
  same text, HTML, link previews off, **no** `reply_parameters`.
- **Lift from a group menu's Yes**: no `sendMessage` to the group; the menu is redrawn with the lift words as its notice.
- No case writes a `scheduled_deletions` row for the group, although every bot kind has a 60 s rule.

## Tasks

| Task | Name | Commit |
|------|------|--------|
| 1 | record the lockdown start, automatic, cannot-lock and lift texts with their queue and private-chat state | 36d2070 |
| 2 | commit the goldens alone, before any send site changes | 36d2070 |

## Verification

- `go test -tags testtools -race -count=1 -run '^TestLegacyLockdownMessageRequests$' -v ./fuku/modules`: exit 0, 12 subtests PASS,
  no FAIL, SKIP or "no tests to run"; the test takes 1.53 s (the first subtest 1.11 s, which includes loading the locale files; the
  others 0.03 to 0.06 s), the command 41 s with the build. `-count=2` also passes.
- Task 2's verify (`TestLegacyLockdownMessageRequests|TestRaidAutoNotice|TestRaidCannotLockWarnsOncePerDay|TestLockdownCommandStarts|TestLockdownAnnounceLift|TestSettingsLockdownLift|TestSettingsLockdownLiftPrivate|TestLockdownAlertLift`, all eight names exist): ok, 4.1 s (42 s with the build).
- The lockdown and raid families, `-race -run '^(TestLockdown|TestRaid|TestSettingsLockdown|TestLegacyLockdown)'`: ok, 35.1 s.
- The full `go test -tags testtools -race -count=1 ./fuku/modules`: ok, 339.9 s for the package (342 s for the command), no FAIL, SKIP or panic.
- `grep -c 'func TestLegacyLockdownMessageRequests('` prints 1; `grep -cE '^\s+goldenLockdown[A-Za-z]+\s+='` prints 12;
  `grep -c '<chat>'` prints 21.
- `git diff --exit-code d8c98c6 -- fuku/modules/lockdown.go fuku/modules/raid_trigger.go fuku/modules/lockdown_alert_callbacks.go fuku/modules/settings_lockdown.go fuku/modules/event_message.go fuku/modules/settings_messages.go` exits 0; the same for `go.mod go.sum`; no commit since d8c98c6 touches those files.
- The earlier goldens are untouched: `git diff --exit-code` against e40a7be (screening), 9d4554d (punishment notices) and 86baf1a
  (moderation) exits 0 for each.
- The golden's commit holds one file and its subject starts with `test(25-01):`.
- `CGO_ENABLED=0 go build ./...` ok; `make test-shard-check` exit 0 (5 s).
- golangci-lint v2.13.1: `run --new-from-rev=origin/main ./...` 0 issues (38 s, after one fix, below); the same with
  `--build-tags=notesttools` 0 issues (23 s); `run --new=false ./fuku/modules/...` reports 110 baseline findings, none in the new file.

## Deviations from Plan

**1. [Plan inconsistency] 12 subtests, not 13**
- The acceptance criterion counts 13 PASS lines, but the plan's behavior list and its artifact table name 12 cases (12 constants, with
  `cannot_lock` pinning two of them and `lift_menu_group` none). The test has exactly the listed 12; no case was invented to reach 13.

**2. [Rule 1 - lint] Embedded field selector**
- staticcheck QF1008 flagged `env.fake.staffBotClient.moduleBotClient`; it reads `env.fake.moduleBotClient` before the commit.

## Not verified

- Nothing beyond the goldens: no production code changed.

## Notes for plan 25-02

- Compare each default with these constants by name, through `lockdownGoldenTokens` and the same ID map (`<chat>`, `<admin>`,
  `<tapper>`, `<staff>`, `<log>`).
- The command's reply and the lift reply of `/unlockdown` go through `replyLockdown` (`msg.Reply` with `formatting.Shtml()`), so the
  default must keep the `reply_parameters` with `allow_sending_without_reply` and the disabled link preview; the automatic notice
  (`raidSend`) and the button lifts (`lockdownPostLiftNotice`) send no reply parameters and must keep it that way.
- The technical notes are separate `\n`-joined lines after the intro, in the fixed order started, approved-muted, started-by, reason,
  no-delete, no-invite for the command, and intro, approved-muted, no-delete, no-invite for the automatic notice; the lift's
  manual-change and unbanning lines follow `lockdown_lifted` the same way. `lockdownJoinLines` drops empty lines.
- The anonymous case names the tapper, not "an admin": the `/lockdown` reply's `{admin}` behaviour for a proved anonymous admin is
  today's mention of the tapper.
- The cannot-lock warning is not a group message and must stay byte-identical (it shares `raidSend` with the automatic notice).
- The group menu's lift posts nothing; any change there must keep `lift_menu_group` passing (D-22's baseline).
- The test calls only the dispatcher, `raidFire`, `askToLiftLockdown`/`pressLockdownButton` and the settings presses; keep those entry
  points, or the golden stops compiling.
- The plan wants the AGENTS.md Testing sentence that names this golden added in 25-02, with the first change it guards.

## Known Stubs

None.

## Threat Flags

None. T-25-01 is mitigated: the golden is committed alone before any send-site change; later plans run it unchanged and check
`git diff --exit-code 36d2070 -- fuku/modules/lockdown_message_legacy_request_test.go`. T-25-SC: `go.mod` and `go.sum` unchanged.

## Self-Check: PASSED

- Files: `fuku/modules/lockdown_message_legacy_request_test.go` exists.
- Commits: 36d2070 is an ancestor of HEAD.
