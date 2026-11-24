---
phase: 22-pm-copies
plan: 08
subsystem: event-messages-pm-copies
status: complete
tags: [pm-copies, safeguards, privacy, settings-menu, PMSG-04, PMSG-05, PMSG-06]
requires: [22-07]
provides:
  - settingsMsgScreens draws the hub with every private copy on, the event screen whose private copy line reads On and the private copy screen with its switch on
  - settingsMsgCheckDest and settingsMsgEditorCodes (the field d held to p, on the private copy row and the private copy screens' editor buttons only)
  - settingsMsgValidPresses keyed by name, with hp, ho and the d=p forms of he hs hr hc hv hf; settingsMsgForgedPresses adds an unknown option, v=2 for hp and ho, d=x and d=g
  - TestEventPMTextsFitEveryLocale, TestEventPMCost, TestEventPMPrivacy (event_message_pm_safety_test.go)
  - eventPMRequestList, eventPMRefuseLogClient, the worst-case sentinels eventPMWorstTitle, eventPMWorstReason, eventPMWorstName
  - settingsPrivacyPhase22Screens and settingsPrivacyPhase22Run in TestSettingsStaffPrivacy
  - TestEventPMLogsCarryIDsOnly cases for the Details record and refused edit, the deep link and a refused log post
affects: [22-09]
tech-stack:
  added: []
  patterns:
    - "a cost test compares a site's request list (method and chat, the group written <chat>) with the golden site's, before and after a held job is released"
    - "a log sweep forbids every Details token found in Redis after the run, so a token can never reach a line"
key-files:
  created:
    - fuku/modules/event_message_pm_safety_test.go
  modified:
    - fuku/modules/settings_messages_screens_test.go
    - fuku/modules/settings_messages_safety_test.go
    - fuku/modules/event_message_pm_failures_test.go
    - fuku/modules/settings_staff_privacy_test.go
    - AGENTS.md
decisions:
  - "No production change: every safeguard passed against the code of plans 22-01 to 22-07, and every size and row shape D-26 fixed was exact."
  - "The plan's settingsPrivacyPhase21Screens and settingsPrivacyPhase21Run were named before the renumbering; that name already belongs to the Messages page sweep (now Phase 21), so the new parts are settingsPrivacyPhase22Screens and settingsPrivacyPhase22Run."
  - "settingsPrivacyPhase22Screens renders the texts a private copy job and the Details deep link send (defaults with and without the lockdown note, the log line of both reasons, the Details strings, the preview header); the private copy screens and the hub's block are screens of settingsMsgScreens, so the existing Phase 21 sweep searches them, and drawing them twice would only add time."
  - "AGENTS.md already had one 'Phase 22 test families' bullet (from plans 22-01 to 22-07); the safeguards were added to it, so the acceptance count stays 1."
  - "A Details record that cannot be read (a Redis error) logs the opener's user ID, not a chat ID: the chat is in the record it could not read. The log test asserts the user ID there."
metrics:
  duration: ~55 min
  completed: 2026-11-21
actuals:
  tokens: 9650
  tasks: 3
  commits: 4
plan_head_before: a953a883e294d20c257005927bf130e44dac689c
plan_head_after: 000a3c8
---

# Phase 22 Plan 08: Private copy safeguards Summary

The private copies are now held to every settings and privacy rule by measurement: every Messages screen of the phase (the
private copy screens, the event screen's private copy row, the hub's options block and the event lines' private copy mark) fits
the 64-byte and 3800-unit limits in 8 languages with the row shapes D-26 fixed, the field `d` is only ever `p` and only where
it belongs, every forged press of the ten codes is refused before any lookup, no draw or private copy preview creates a row, an
options row or a cache key, every default private text, the Details strings and the log line fit Telegram's limits with
markup-heavy worst cases, a `/ban` with the copy on makes no request of the copy's before its handler returns, no copy, Details
view, log line or request names the Staff Group or another group, and no log line carries content or a Details token. No
safeguard found a defect, so no production code changed.

## Tasks

| Task | Name | Commit |
|------|------|--------|
| 1 | screens and texts measured in 8 languages: `settingsMsgScreens` PM-on states, the `d` check in `TestSettingsMessagesCallbackBytes`, `TestEventPMTextsFitEveryLocale` | d070454 |
| 2 | forged presses (hp, ho, d=p forms), reads and cost of the private copy screen and its Full preview, `TestEventPMCost`, `TestEventPMLogsCarryIDsOnly` extended | 7ef5f3d |
| - | lint fix asked by the coordinator: De Morgan's law in `settingsMsgCheckDest` (staticcheck QF1001 in CI) | 938ca7c |
| 3 | `TestEventPMPrivacy`, `settingsPrivacyPhase22Screens` and `settingsPrivacyPhase22Run`, AGENTS.md | 000a3c8 |

## Measured limits

| What | Measured |
|------|----------|
| Bytes at chat ID `-1001234567890` | unchanged from the plans: `hl` 37, `he` 42 (with `d`), `hs hr hc hv` 46 (with `d`), `hf` 42 (with `d`), `hq` 42, `hp` 42, `ho` 41, `op` 37, Back to Main and Close 33; Main cell 37 private, 33 group; group note buttons 29. D-26's numbers were exact. |
| Field `d` | only `p`; on the event screen's private copy row (`he`, exactly one per event screen) and on every editor button (`hs hr hc hv hf`) of a private copy screen, prompt and refusal; no Phase 21 button carries it |
| Rows | hub `[1 1 1 2]` (also with every private copy on), event screen `[2 2 2 2 2 1 1 2]` full (also with the private copy line On), private copy screen `[1 1 1 1 1 2]` empty, `[2 2 2 2 2 1 2]` full with the switch off and on |
| Private texts | every default private text with a 64-rune `<&Ж>` title, a 200-character `&<b>` reason and an end, with and without the lockdown note, 1..4096 UTF-16 units; the log line of every outcome at most 1000 with a 64-rune `<&Ы>` name appearing once, escaped, inside the mention; Details label at most 20 code points, neutral reply at most 120; no `{`, `%!` or empty `<b></b>` |
| Cost | `/ban` with the copy off: exactly the golden site's request list; with the copy on and held: the same list when the handler returns, one job in flight, one request to the member after release |

## What plan 22-09 uses

| Kind | Name |
|------|------|
| Screens | `settingsMsgEveryPMOn(t)` (every event custom with its private copy on), the PM-on entries of `settingsMsgScreens` |
| Forged presses | `settingsMsgValidPresses` (keyed by name; the code is `kv[1]`), `settingsMsgForgedPresses()` |
| Cost | `eventPMRequestList(s)` (method and chat, the group written `<chat>`) |
| Logs | `eventPMRefuseLogClient{BotClient, channel, err}` (refuses `#PM_COPY` posts to a channel after recording them) |
| Privacy | `eventPMPrivacyCustom` (every Welcome and event placeholder), the sentinels `eventPMPrivacyStaffTitle`, `eventPMPrivacyOtherTitle`, `eventPMPrivacyTapper` |

## Verification

- `go test -tags testtools -race -count=1 -run '^(TestSettingsMessagesCallbackBytes|TestSettingsMessagesPageFitsEveryLocale|TestEventPMTextsFitEveryLocale)$' ./fuku/modules`: ok (9.6 s).
- `go test -tags testtools -race -count=1 -run '^(TestSettingsMessagesForged|TestSettingsMessagesReadsCreateNothing|TestSettingsMessagesCost|TestEventPMCost|TestEventPMLogsCarryIDsOnly)$' -v ./fuku/modules`: every test PASS, no SKIP (5.7 s); the twelve subtests of `TestEventPMLogsCarryIDsOnly` all PASS.
- `go test -tags testtools -race -count=1 -run '^TestEventPMPrivacy$' -v ./fuku/modules`: PASS (2.7 s).
- `go test -tags testtools -race -count=1 -run '^TestSettingsStaffPrivacy$/(pure|a_private_Phase_22)' -v ./fuku/modules`: PASS; pure renders 34.6 s (was about 28 s before Phase 22's screens and texts), the Phase 22 run 0.45 s.
- `go test -tags testtools -race -count=1 -timeout 30m -run '^(TestSettingsStaffPrivacy|TestEventPMPrivacy|TestSettingsMessages.*|TestSettingsDriftMessages.*|TestSettingsAuthorityMessages.*|TestEventPM.*|TestPMCopy.*|TestLegacyModerationRequests|TestCommandRepliesStayInsideTheHandler)$' ./fuku/modules`: ok (99.5 s), no FAIL, no SKIP.
- `CGO_ENABLED=0 go build ./...` ok; `gofmt -l` empty on the five touched Go files; `make check-docs`: no drift.
- golangci-lint: `run --new-from-rev=origin/main ./...` 0 issues; `--build-tags=notesttools` 0 issues; `run --new=false ./fuku/modules/...` reports nothing in the five touched files.
- Mutation proofs (each reverted, `git diff` empty afterwards):
  - `settingsMsgPMFields` without the `v` check: `TestSettingsMessagesForged` fails (`hp with v=2 made 1 getChatMember calls` and `1 getChat calls`).
  - the private copy definition's `Fields` without `d`: `TestSettingsMessagesCallbackBytes` fails (469 "has no d" errors).
  - the token appended to the deep link's Redis-error Debug line: `TestEventPMLogsCarryIDsOnly` fails (`a log line carries "<token>"`).
  - the linked Staff Group's chat ID appended to `pmCopyLogText`: `TestEventPMPrivacy` fails in every language and event.
- Acceptance greps: `renderSettingsMsgPM(` 3 and `2 2 2 2 2 1 1 2` 3 in the screens test; `func TestEventPMTextsFitEveryLocale(`, `func TestEventPMCost(`, `func TestEventPMPrivacy(` once each; `settingsActMsgPM\b\|settingsActMsgPMOption` 4 in the safety test; `Phase 22 test families` once in AGENTS.md; `settingsPrivacyPhase22Screens` 3 in the privacy test (see the naming decision).

## Deviations from Plan

### Auto-fixed Issues

None in production code.

### Other deviations

**1. [Naming] Phase 22 names for the privacy parts**
- The plan (written before the renumbering) names `settingsPrivacyPhase21Screens` and `settingsPrivacyPhase21Run`; the former already exists for the Messages page (Phase 21). The new parts are `settingsPrivacyPhase22Screens` and `settingsPrivacyPhase22Run`, and the plan's grep for `settingsPrivacyPhase21Screens` holds on the existing function (3).

**2. [Lint] a fourth commit**
- CI's staticcheck flagged QF1001 (De Morgan) in `settingsMsgCheckDest` of commit 7ef5f3d's tree (introduced in d070454). The local `--new-from-rev=origin/main` run missed it because the repository's `new: true` policy reports only issues the run considers new. Fixed in its own commit 938ca7c, as the coordinator asked; a `--new=false` run now reports nothing in the touched files.

**3. [Acceptance] `git diff --exit-code 9940d97 -- go.mod go.sum main.go` exits 1**
- `main.go` differs from 9940d97 by the merge of main (Conversations, v2.26.0, commit 16fbe95), before this plan. This plan changed none of the three files (`git diff a953a88 HEAD -- go.mod go.sum main.go` is empty).

**4. [Test] the Details deep link's Redis-error line names the user, not the chat**
- The plan's behavior says failure lines name the chat ID; the line of a record that cannot be read has no chat to name (the chat is in the record). The test asserts the opener's user ID for that line and the chat for the refused view.

**5. [Commits] one commit per task**
- The plan named one commit; each task is its own commit and the last carries the planned subject with AGENTS.md beside it.

### Expected-set edits of earlier tests (widened, no other change)

- `settingsMsgScreens`: three more states (the hub with every private copy on, the event screen with its private copy On, the private copy screen with the switch on).
- `settingsMsgValidPresses`: keyed by name instead of code (the code is the second item), eight more presses; `settingsMsgForgedPresses` switches on that code.
- `TestSettingsMessagesReadsCreateNothing`: the private branch opens every private copy screen and the ban's private copy Full preview, and also counts `chat_event_message_options`; the last-edit check now expects the private copy screen.
- `TestSettingsMessagesCost`: one more draw (a private copy screen).

No `TestLegacy*Requests` golden and no `TestSettingsDeletionPhase16ScreensUnchanged` digest was touched; `TestLegacyModerationRequests` passes unedited.

## Not verified

- The whole `fuku/modules` package, `make test`, `make test-shard-check` and `make lint` itself were not run (targeted runs only; the CI-equivalent golangci-lint runs above were). `TestSettingsStaffPrivacy`'s pure renders grew by about 6 s with the Phase 22 additions measured together with the earlier Phase 22 screens; the shard time was not measured.
- No PostgreSQL run: nothing here rests on a constraint.
- `make check-translations` was not run: no locale file changed.
- A default private text sent as a caption (a custom copy with media and no stored text) can pass 1024 UTF-16 units with an extreme title and reason; Telegram then refuses it and the job sends the default text alone (D-15). This is the designed fallback, not checked as a limit.

## Notes for plan 22-09 (end-to-end and phase gates)

- `TestSettingsStaffPrivacy`'s pure renders now take about 35 s; 22-09 should add no language sweep.
- `TestEventPMCost` already proves the handler never waits for a copy against the golden site; the end-to-end test need not repeat a held-job case.
- The phase gate should run golangci-lint with `--new=false` on the files the phase touched, because `--new-from-rev` missed a staticcheck finding here.
- The `9940d97` acceptance diffs of the phase's plans now fail on `main.go` because of the main merge; compare against the commit before each plan instead.

## Known Stubs

None.

## Threat Flags

None: T-22-31 (`TestSettingsMessagesForged`), T-22-32 (`TestSettingsMessagesCallbackBytes`, `TestSettingsMessagesPageFitsEveryLocale`),
T-22-33 (`TestEventPMPrivacy`, `settingsPrivacyPhase22Screens`, `settingsPrivacyPhase22Run`, and the Phase 21 sweep over the PM screens) and
T-22-34 (`TestEventPMLogsCarryIDsOnly`) are mitigated as planned.

## Self-Check: PASSED

- FOUND: fuku/modules/event_message_pm_safety_test.go
- FOUND commits: d070454, 7ef5f3d, 938ca7c, 000a3c8, all ancestors of HEAD
