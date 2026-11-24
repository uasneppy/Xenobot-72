---
phase: 25-lockdown-messages
plan: 08
subsystem: lockdown-messages
status: complete
tags: [lockdown, event-messages, end-to-end, evt-06, evt-07, evt-08, evt-10, agents-md]
requires:
  - 25-01 (the goldens)
  - 25-02 to 25-05 (the five lockdown sites and the Lockdown family on the Messages page)
  - 25-06 and 25-07 (the surfaces, privacy and log toolkits)
provides:
  - TestLockdownMessagesEndToEnd (10 subtests)
  - the toolkit lockdownMessagesE2E, newLockdownMessagesE2E, openLockdownEvent, setLockdownPart, setLockdownText,
    silence, lockCommand, wantLockdownReply, onlySend, wantAutoNotice, wantGoldenLast, wantNoPrivateSend
  - the final Phase 25 pass over AGENTS.md
affects: [phase verification]
requirements-completed: [EVT-06, EVT-07, EVT-08, EVT-10]
tech-stack:
  added: []
  patterns:
    - "every lockdown message is set on the page (presses of the drawn buttons, typed private messages), never through the repository"
    - "a golden is compared with the command's real message ID put in place of the recorded one, so the page's update IDs do not matter"
key-files:
  created:
    - fuku/modules/lockdown_messages_e2e_test.go
  modified:
    - AGENTS.md
decisions:
  - "The AI burst is counted through raidAIRecordVerdict (five verdicts from three accounts), as the plan's assumption says; the join surge and the flood go through the dispatcher."
  - "The four lift doors share one environment with a new /lockdown before each lift (a manual start is not quiet); each automatic trigger has its own environment."
  - "The private menu's lift is pressed by the owner (the live creator), so the four lifts name two different lifters and the {admin} of each is checked by name."
  - "The golden comparisons replace the recorded command message ID (101, 102) with the real one (lockdownE2EGolden), so the goldens stay unedited and the comparison is byte for byte otherwise."
metrics:
  duration: ~45 min
  completed: 2026-11-22
actuals:
  tokens: 6600
  tasks: 2
  commits: 2
plan_head_before: 0bb81b337f90c39015c367e8f0537a2158d69998
plan_head_after: e264e1cba6a20039c1ec2c0a31572162bb2fa74c
---

# Phase 25 Plan 08: The lockdown messages end to end Summary

One end-to-end test drives the whole phase through the real dispatcher, with only Telegram faked. The owner sets each lockdown
message from the group's private Messages page, by pressing the buttons the bot drew and typing in private. A manual `/lockdown`,
fifteen real joins, thirty real items from new members, an AI burst counted through the verdict counter, and lifts by
`/unlockdown`, by the alert's button in the Staff Group, by a private menu and by a group menu each post that group's custom message,
with its values and, for a start, the technical notes after it, always after the lock or the lift took effect. With nothing stored
the requests equal the goldens, a refused photo falls back to the golden default once, Silent posts nothing, the alert and the
raid-scale paths carry no custom text, and a Silent press on one replica is honoured by the other. AGENTS.md was read sentence by
sentence against the code: two sentences were corrected, and every identifier named in a Phase 25 sentence exists. No production
defect was found, so no production file changed.

## Tasks

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | the phase end to end through the real dispatcher | af8941b | lockdown_messages_e2e_test.go (new), AGENTS.md (the test's paragraph in the Phase 25 families) |
| 2 | the phase-wide gates, the full package run and the final AGENTS.md pass | e264e1c | AGENTS.md |

## What the end-to-end test holds

`TestLockdownMessagesEndToEnd`, 3.1 s with `-race` when run alone (1.4 s inside the Task 1 verify run, where the real locales were
already loaded), against the plan's budget of about 5 s:

| Subtest | Time | What it shows |
|---------|------|---------------|
| `owner sets the manual start` | 0.53 s | A typed text, photo and link button stored as typed. The hub reads Custom for `lockdown`. `/lockdown raid & co` makes `setChatPermissions` before one `sendPhoto` that replies to the command. Its caption is `<b>Ad&lt;b&gt;min</b> locked Lock Chat: raid &amp; co`, then a line break and the approved-muted note, and it carries the button. No queue row. The next cycle's alert in the Staff Group and the log channel carries none of it |
| `join surge` | 0.12 s | Fourteen self joins do nothing. The fifteenth locks, then one new message: `<b>15</b> joined in 60 s: ` with the escaped trigger sentence and the note. The tipping joiner is still recorded pending |
| `flood` | 0.15 s | Thirty items from four members who joined two hours ago: `<b>30</b> in 30 s, 100%` and the note, after the lock |
| `ai burst` | 0.09 s | Five verdicts from three accounts: `<b>5</b> spam in 120 s: ` with the trigger sentence and the note |
| `lift doors` | 0.16 s | Four lockdowns in one environment. `/unlockdown` replies to the command. The alert's Lift and Confirm in the Staff Group, a private menu's Lift Yes by the owner and a group menu's Lift Yes each post a new message. Each lift posts its custom text naming its lifter, once. The group menu is also redrawn with `lockdown_lifted`. No queue row |
| `nothing stored` | 1.40 s | Real locales. `/lockdown` and `/unlockdown` equal `goldenLockdownStart` and `goldenLockdownLiftCommand` with the command's message ID put in. Fifteen self joins in a second environment equal `goldenLockdownAutoJoinSurge`. No queue row (the time is the first real-locale load) |
| `refused` | 0.13 s | Real locales. A custom photo refused with a 400 comes after the lock, and the golden default follows it once |
| `silent` | 0.12 s | The start and the lift silenced from the page. `/lockdown` locks with no reply and the alert is posted. `/unlockdown` lifts with no reply |
| `untouched surfaces` | 0.31 s | All five texts set from the page. The alert, a reminder made due, a "Ban N recent joiners" run with its card report and `#LOCKDOWN_BAN` post, the alert's lift and the close of both copies carry none of them. In the group only the start reply and the lift notice carry one. Nothing goes to a private chat after the page |
| `two replicas` | 0.12 s | Replica B posts the custom start text. The owner silences it through replica A. B's next `/lockdown` locks and posts nothing |

Mutation checks (each reverted, `event_message_lockdown.go` restored byte for byte):

- `Count: trig.Count + 1` in `postLockdownAutoEvent` fails `join surge`, `flood` and `ai burst`.
- Building the lift reply under the `lockflood` event fails `lift doors`, `silent` and `untouched surfaces`.

## AGENTS.md pass

Every backticked identifier on the lines Phase 25 added or changed (`git diff d8c98c6 -- AGENTS.md`, 347 identifier-like tokens) was
looked for as a whole word in `fuku`, `main.go`, `migrations`, `locales` and `scripts`. Every one exists. The only tokens not found
as a word are the test file names (they exist as files) and the `goldenLockdown*` pattern. The facts were then read against the code:

- The five events and their order in `eventmsg.Events`.
- The Lockdown family's codes, icons, samples, `NoReason`, `Appended` and `ReservedText`.
- The five adapter callers and the never-reach roots and stops of `TestEventEngineCallers`.
- The wake before the post in `lockdownAnnounceLift`, and the group menu's wake followed by `postLockdownLiftEvent(..., false)`.
- `raidPostNotice`'s one call, made inside `raidPostAsync` only for `lockdownStartStarted`.
- The do-nothing `Schedule`.
- The notes and the 400-unit reserve.
- The send bounds. A custom send is bounded only by the bot's HTTP client timeout, `constants.LongTimeout`, 30 s. No sentence claims a 10 s bound for it.
- The subtest names of every Phase 25 test.

Two sentences were corrected:

- The manual start adapter's signature read `(…, row, botMember)`. The code's parameter is `bot`, so it now reads `(…, row, bot)` with a gloss.
- The scheduling bullet said only the `/lockdown` reply is never queued. It now says no lockdown message is ever queued: the reply, the automatic notice or the lift notice.

The end-to-end test and its toolkit were added to the Phase 25 test families in the Task 1 commit.

## Deviations from Plan

None in behaviour. Notes:

- The plan's `nothing stored` subtest is written as planned: one environment for `/lockdown` and `/unlockdown`, and a second for
  the join surge. The goldens' recorded message IDs (101, 102) are replaced by the command's real ID. The settings page uses update
  IDs, so a fixed 101 would be brittle. The goldens themselves are untouched.
- The plan's planner assumption listed `askToLiftLockdown`-style presses. The alert's lift uses 25-07's `lockdownSafeAlertLift`
  instead: it is the first alert in the lift-doors environment, which that helper requires.
- The toolkit adds a few helpers beyond the plan's named four (`setLockdownText`, `silence`, `lockCommand`, `onlySend`,
  `wantAutoNotice`, `wantGoldenLast`). The plan's names are kept.
- Not every check was re-run after the AGENTS.md wording commit, because that commit changes no code. The verify commands and the
  full package run were made on the final tree.

## Production fixes

None. The end-to-end test found no defect.

## Edits to earlier tests

None. Only the new test file was added; no earlier test, golden or helper changed.

The goldens are unchanged. `git diff --exit-code` exits 0 for each:

- `36d2070`: lockdown. `git log` shows exactly one commit for that file, its own.
- `e40a7be`: screening
- `9d4554d`: punishment
- `86baf1a`: moderation

`git diff --exit-code d8c98c6` also exits 0 for:

- the screening, punishment and moderation goldens;
- `go.mod go.sum main.go migrations`.

## Verification (with durations)

- **Task 1 verify command** (11 tests, `-race -v`): all PASS, no SKIP, no "no tests to run". 9.5 s for the package, 12 s wall.
  `--- PASS: TestLockdownMessagesEndToEnd/` appears 10 times.
- **Task 2 verify chain:**

  | Check | Result | Time |
  |-------|--------|------|
  | `make check-translations` | all present | 1 s |
  | `make check-docs` | no drift | 1 s |
  | `make test-shard-check` | exit 0 | 23 s |
  | `CGO_ENABLED=0 go build ./...` | ok | 3 s |

- **golangci-lint v2.13.1:**
  - `run --new-from-rev=origin/main ./...`: 0 issues (39 s).
  - The same with `--build-tags=notesttools`: 0 issues (21 s).
  - `--new=false` on `./fuku/modules/... ./fuku/db/eventmsg/... ./fuku/i18n/...` with no issue cap: 110 baseline issues, the same count as plan 25-07. None is in the new file. The plan's production-file grep prints only the two baseline `gocyclo` findings: `(moduleStruct).lockdown` at 21 and `(moduleStruct).lockdownAlertCallback` at 21.
- **gosec** (`-quiet ./fuku/modules/...`): 16 baseline issues, the same as plan 25-07. No non-test file changed. The installed binary reports its version as `dev`.
- **Unit tests:** `go test -tags testtools -race -count=1 ./fuku/db/eventmsg ./fuku/i18n` passed (1.1 s and 50.0 s).
- **Touched families with `-race`** passed: 85.3 s for the package, 88 s wall. The families:
  - `TestLockdown.*`, `TestLegacy.*`, `TestEventMessageLockdown.*`, `TestEventEngine.*`
  - `TestPMCopy.*`, `TestEventPM.*`, `TestRaid.*`
  - `TestSettingsMessages.*`, `TestSettingsAuthorityMessages.*`, `TestSettingsDriftMessages.*`, `TestSettingsStaffPrivacy`, `TestSettingsLockdown.*`
  - `TestUnlockdown.*`
  - the four other end-to-end tests
- **Acceptance greps:**
  - `func TestLockdownMessagesEndToEnd(`: 1.
  - In AGENTS.md: `TestLockdownMessagesEndToEnd` 1, `TestLegacyLockdownMessageRequests` 1, `TestEventEngineCallers` 5.
  - `first production caller arrives in plan 25-` in `fuku`: 0.
  - `bounded by 10 s` or `10 s bound` in `event_message_lockdown.go`: 0.
- **Full `go test -tags testtools -race -count=1 -timeout 45m ./fuku/modules`:** ok, 361.9 s for the package (364 s wall), no FAIL or
  SKIP line. Plan 25-07 measured 379.0 s, so the difference is runner noise, and the run is well under the 10-minute mark.

## Phase 25 test time against the 28 s budget (D-39)

| Plan | New tests | Time (`-race`), as each plan measured it |
|------|-----------|----------------|
| 25-01 | `TestLegacyLockdownMessageRequests` | 1.5 s |
| 25-02 | `TestEventMessageLockdownStart`, `TestLockdownEventDefaults`, `TestLockdownNoteFitsReserve`, `TestEventEngineCallers` (and its guard self-check) | 1.1 s warm (1.8 s with the first locale load) |
| 25-03 | `TestSettingsMessagesLockdownFamily` | 1.7 s |
| 25-04 | `TestEventMessageLockdownAuto` | 1.75 s |
| 25-05 | `TestEventMessageLockdownLift` | 2.2 s alone (0.55 s with the locales warm) |
| 25-06 | `TestLockdownMessagesLeaveOtherSurfacesAlone`, `TestLockdownEventsSendNoPrivateCopy` | 0.9 s |
| 25-07 | the authority, drift, privacy and log tests | 1.3 s |
| 25-08 | `TestLockdownMessagesEndToEnd` | 3.1 s alone (1.4 s with the locales warm) |

That is about 14.3 s in all with each test's own cold time, well inside the 28 s budget. Each cold time includes one
real-locale load, which only the first such test of a package run pays.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register:

- T-25-21: `owner sets the manual start`, `join surge` and `refused` show the lock before any message.
- T-25-22: `untouched surfaces`.
- T-25-23: the AGENTS.md pass and the acceptance grep.
- T-25-SC: no dependency added.

## Self-Check: PASSED

- Files: `fuku/modules/lockdown_messages_e2e_test.go` and this SUMMARY exist.
- Commits af8941b and e264e1c are ancestors of HEAD.
- The ledger: `git rev-list --count 0bb81b3..e264e1c` is 2.
