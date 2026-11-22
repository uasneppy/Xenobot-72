---
phase: 25-lockdown-messages
plan: 02
subsystem: event-messages
status: complete
tags: [lockdown, event-messages, engine, guard, evt-06, evt-10]
requires:
  - 25-01 (TestLegacyLockdownMessageRequests and its goldenLockdown* constants and helpers)
provides:
  - the five lockdown event keys (lockdown, lockjoin, lockflood, lockai, locklift)
  - the {percent} placeholder; eventDefaultInput.Window, Percent, Lockdown
  - fuku/modules/event_message_lockdown.go (lockdownFacts, the notes, the three default builders, lockdownEventPerson, lockdownEventSite, postLockdownStartEvent)
  - TestEventMessageLockdownStart, TestLockdownEventDefaults, TestLockdownNoteFitsReserve
  - TestEventEngineCallers and TestEventEngineCallersGuardFails (EVT-10, structural)
affects: [25-03, 25-04, 25-05, 25-06, 25-08]
tech-stack:
  added: []
  patterns:
    - "a lockdown file reaches the engine only through a postLockdown*Event adapter in a non-lockdown file"
    - "a lockdown eventSite whose Schedule does nothing, so no lockdown message enters the deletion queue"
    - "a default builder proven by sending its text and options through the site's own default call on a recording client"
key-files:
  created:
    - fuku/modules/event_message_lockdown.go
    - fuku/modules/event_message_lockdown_test.go
    - fuku/modules/event_engine_callers_test.go
  modified:
    - fuku/db/eventmsg/events.go
    - fuku/db/eventmsg/repository_test.go
    - fuku/modules/event_message.go
    - fuku/modules/lockdown.go
    - fuku/modules/pm_copy_callers_test.go
    - AGENTS.md
decisions:
  - "The start default and the appended notes share lockdownStartNoteLines; the start default keeps today's strings.Join + TrimSpace (no empty-line filter) so it is byte-identical even for a missing key."
  - "lockdownLiftDefaultReply calls lockdownLiftedText as planned (lockdown.go is not touched in Task 2); a test holds that the lift default equals its first line plus lockdownLiftNotes, so the two cannot drift."
  - "The guard's never-reach walk follows every same-package function a body names (called or used as a value), not only calls: strictly more than the plan asked."
  - "SendCaptcha reaches the engine through the attempt's timeout (scheduleCaptchaTimeout -> ... -> handleCaptchaTimeout -> postCaptchaFailureNotice), which is the Phase 24 captcha event, not the challenge; the guard gets eventEngineRootStops with that one stop and its reason."
metrics:
  duration: ~75 min
  completed: 2026-11-22
actuals:
  tokens: 14900
  tasks: 3
  commits: 3
plan_head_before: beb88e3177e4a384a272b8f9fd58d78031cab51c
plan_head_after: c4b5ea7f1d194480b44d2c6bbd0af1c72ac2b414
---

# Phase 25 Plan 02: The manual lockdown's message, the lockdown builders and the engine caller guard Summary

`/lockdown` now posts a group's stored lockdown message through the event message engine from the adapter file
`event_message_lockdown.go` (custom text with `{admin}`, `{chatname}`, `{reason}` and the person placeholders, then the
technical notes; never queued; Silent and the refusal fallback work), while a group with nothing stored gets the plan 25-01
golden byte for byte; the other four lockdown keys, `{percent}` and the automatic and lift default builders exist and are
proven against the goldens; and `TestEventEngineCallers` pins every caller of the engine and proves no alert, reminder,
raid-scale, staff, log or captcha challenge root can reach it.

## What later plans use

| Kind | Name |
|------|------|
| Event keys | `eventmsg.EventLockdown` (`lockdown`), `EventLockJoinSurge` (`lockjoin`), `EventLockFlood` (`lockflood`), `EventLockAIBurst` (`lockai`), `EventLockLift` (`locklift`), appended to `Events` after `mreqmute` |
| Engine | `eventValues.Percent` and `{percent}` (after `window`, 0 renders nothing); `eventDefaultInput.Window int`, `Percent int`, `Lockdown lockdownFacts` |
| Facts and notes | `lockdownFacts{NoDelete, NoInvite, ManualChange, Finished bool; Unbanning int64}`, `lockdownStartFacts(bot)`, `lockdownStartNoteLines`, `lockdownStartNotes`, `lockdownLiftNoteLines`, `lockdownLiftNotes`, `lockdownNoteReserve = 400` |
| Builders | `lockdownStartDefaultReply(tr, in)` (`formatting.Shtml()`), `lockdownAutoDefaultReply(kind) eventDefaultReply` (`lockdownNoticeOpts()`: HTML, previews off; unknown kind `"", nil`), `lockdownLiftDefaultReply(tr, in)` (`formatting.Shtml()`; a paced site must send the text with its own options, `lockdownPostLiftNotice`'s) |
| Site helpers | `lockdownEventPerson(b, userID, name, named)` (the bot's `b.User` when not named), `lockdownEventSite(b, chat, tr, actor, replyTo, threadID, automatic, sendDefault)` (no-op `Schedule`) |
| Manual start | `postLockdownStartEvent(b, ctx, msg, tr, actor, name, row, botMember)`, no return; logs `[Lockdown] reply: %v` |
| Guard | `eventEngineCallers`, `eventLockdownAdapterCallers`, `eventEngineNeverRoots`, `eventEngineRootStops`; helpers `eventEngineParseDir`, `eventEngineScan`, `(eventEngineScanResult).reaches(root, stops)`, `eventEngineWantExact` |
| Test helpers | `lockdownEventEnv(t, golden)`, `lockdownEventWantStarted`, `lockdownEventOnlySend`, `lockdownDefaultRequest(t, text, opts, replyTo)`, constants `lockdownDefaultChat/Admin/Tapper` and `lockdownDefaultIDs` |

## Tasks

| Task | Name | Commit |
|------|------|--------|
| 1 | the manual lockdown's message end to end, through /lockdown | 4bb1b39 |
| 2 | the other four keys, {percent} and the automatic and lift builders, proven against the goldens | f5341c1 |
| 3 | the structural EVT-10 guard over every caller of the engine | c4b5ea7 |

## Earlier tests whose expected values changed (expected-set edits only, nothing weakened)

- `fuku/db/eventmsg/repository_test.go`, subtest "the event and destination tables": the expected `Events` list gains the five
  lockdown keys; the refused words gain `lockdownjoin`, `Lockdown`, `lock`, `unlockdown`.
- `fuku/modules/pm_copy_callers_test.go`: the `postEvent` allow-list gains `event_message_lockdown.go` with its reason.

## Verification

- Task 1 verify (15 module tests + `TestEventMessages`): ok, 6.1 s + 1.1 s (cached build).
- Task 2 verify (13 module tests + `TestEventMessages`): ok, 4.7 s + 1.1 s.
- Task 3 verify (`TestEventEngineCallers|TestPMCopyCallers|TestCommandRepliesStayInsideTheHandler|TestRepliesGuardFindsAReplyingGoroutine`): ok, 2.8 s.
- New tests' own time: `TestEventMessageLockdownStart` 0.4 s warm (1.8 s when it is the first to load the real locales),
  `TestLockdownEventDefaults` 0.02 s warm, `TestLockdownNoteFitsReserve` 0.05 s, `TestEventEngineCallers` 0.6 s,
  `TestEventEngineCallersGuardFails` 0.00 s: about 1.1 s warm, inside the plan's 3 s budget.
- Mutation check of the guard: `_ = postEvent` added to `lockdownPostTally` made `TestEventEngineCallers` fail three ways (unlisted
  caller, `runLockdownCycle` and `lockdownPostInlineTally` reach the engine); reverted.
- Acceptance greps: `postLockdownStartEvent(` once in each of the adapter file and `lockdown.go`; no lockdown or raid production
  file names an engine entry; 5 `EventLock*`-style keys (`EventLockdown` plus four); `"percent"` in `event_message.go`; both builders
  and `lockdownNoteReserve = 400`; the six named roots appear in the guard; `TestEventEngineCallers` in AGENTS.md;
  `TestEventMessageLockdownStart` has 5 PASS subtests.
- Goldens untouched: `git diff --exit-code` against 36d2070 (lockdown), e40a7be (screening), 9d4554d (punishment notices),
  86baf1a (moderation) all exit 0; the lockdown golden still has only its own commit (36d2070). `go.mod go.sum main.go migrations`
  unchanged since d8c98c6.
- Families with `-race` (`TestLockdown|TestRaid|TestSettingsLockdown|TestLegacy|TestEventMessage|TestEventPM|TestPMCopy|TestPMCopies|
  TestSettingsMessages|TestSettingsDriftMessages|TestSettingsAuthorityMessages|TestEventEngine|TestAutoEventDefaults|
  TestScreeningEventDefaults|TestRenderEventText` over `./fuku/modules ./fuku/db/eventmsg`): ok, 58.3 s + 1.1 s (60.7 s wall).
- `make check-translations` ok (0.9 s, no key added); `make check-docs` no drift (1.5 s); `make test-shard-check` exit 0 (21 s);
  `CGO_ENABLED=0 go build ./...` ok.
- golangci-lint v2.13.1: `run --new-from-rev=origin/main ./...` 0 issues (39 s); the same with `--build-tags=notesttools` 0 issues
  (22 s); `run --new=false ./fuku/modules/... ./fuku/db/eventmsg/... ./fuku/i18n/...` on the touched files: only the two baseline
  gocyclo lines, `(moduleStruct).lockdown` now 21 (was 24) and `(moduleStruct).lockdownAlertCallback` 21 (unchanged). With
  `--tests=false --enable-only=unused` exactly the seven marked identifiers are reported when their markers are removed, so every
  `//nolint:unused // first production caller arrives in plan 25-03` is needed.
- Full `go test -tags testtools -race -count=1 ./fuku/modules`: ok, 357.6 s for the package (5 m 59.7 s wall), no FAIL, SKIP or panic.

## Deviations from Plan

**1. [Rule 1 - plan assumption wrong] `SendCaptcha` reaches the engine through the captcha timeout**
- **Found during:** Task 3
- **Issue:** the never-reach walk found `SendCaptcha -> scheduleCaptchaTimeout -> expireCaptchaAttempt -> expireCaptchaAttemptApplied ->
  settleCaptchaAttempt -> handleCaptchaTimeout -> postCaptchaFailureNotice -> postEvent`. The challenge schedules the attempt's timeout,
  and a timeout that fails the member posts the Phase 24 captcha event, which is sanctioned; the challenge message itself never goes
  through the engine.
- **Fix:** `eventEngineRootStops` (per root, the functions the walk does not enter, each with a reason, each required to exist): only
  `SendCaptcha` → `handleCaptchaTimeout`. Every other path from `SendCaptcha` is still walked. Documented in the test and AGENTS.md.
- **Commit:** c4b5ea7

**2. [Rule 2 - stronger guard] The walk follows function values too**
- The plan asked to follow calls; the walk follows every same-package function a body names (a call, a `go` statement, a function passed
  as a value) and every method named on the caller's own receiver. This only widens what is checked. A method whose receiver has no name
  (`func (moduleStruct) warnThisUser`) is keyed by its bare name, as `repliesGuardDeclKey` does; the bare-name following still reaches it.

**3. [Discretion] Key names in the caller map**
- The planner listed some callers as methods; in the tree `floodNotice`, `kickReply`, `banReplyWithButton`, `moderationTban`,
  `moderationUnban`, `muteReplyWithButton`, `moderationTmute`, `moderationUnmute` are plain functions and `warnThisUser` has an unnamed
  receiver, so their keys carry no type. Recorded from the tree as the plan asked.

**4. [Discretion] Extra assertions in `TestLockdownEventDefaults`**
- Besides the nine goldens it holds that the defaults end with the very notes a custom text gets (start and automatic) and that the lift
  default equals its first line plus `lockdownLiftNotes`, since `lockdownLiftedText` (in `lockdown.go`, untouched by Task 2) and
  `lockdownLiftNoteLines` are two constructions of the same lines.

## Known Stubs

None. The seven Task 2 identifiers marked `//nolint:unused` (`lockdownNoteReserve`, `lockdownNoticeOpts`, `lockdownAutoIntroKeys`,
`lockdownAutoDefaultReply`, `lockdownLiftNoteLines`, `lockdownLiftNotes`, `lockdownLiftDefaultReply`) are complete and tested; they
get their first production caller in plan 25-03 (the page), which removes the markers.

## Threat Flags

None. T-25-02 (escaping) and T-25-03 (anonymous admin) are covered by the `custom` and `anonymous` subtests; T-25-04 by
`TestEventEngineCallers` and `TestPMCopyCallers`; T-25-05 by `silent` and `refused` (the lockdown stands whatever the message does);
T-25-SC: no dependency added.

## Notes for plan 25-03

- Use the builders as the page's defaults: the manual event's `Default` is `lockdownStartDefaultReply`; the triggers'
  `lockdownAutoDefaultReply(models.LockdownTrigger*)` with `in.Count`, `in.Window`, `in.Percent` and `in.Lockdown`; the lift
  `lockdownLiftDefaultReply` with `in.Lockdown.ManualChange/Finished/Unbanning`. Remove all seven `//nolint:unused` markers once each
  has a production caller, and re-run `golangci-lint run --tests=false --enable-only=unused ./fuku/modules/...` to prove none is left.
- `lockdownNoteReserve` is the family's `ReservedText`; the Full preview's appended note for a start event is
  `lockdownStartNotes(tr, lockdownFacts{})` (both rights held: the approved-muted line alone); the lift has no note.
- `{window}` is a string value (`eventValues.Window`), so the samples pass `strconv.Itoa(60)`; `{percent}` is `eventValues.Percent`.
- No `eventmsg_` key is needed; no lockdown event may get a private copy spec (`TestEventPMSpecs` still counts eighteen).

## Notes for plans 25-04 and 25-05

- Add `postLockdownAutoEvent`, `postLockdownLiftReplyEvent` and `postLockdownLiftEvent` in `event_message_lockdown.go` and add each
  adapter and each new caller to `eventEngineCallers` and `eventLockdownAdapterCallers` with a reason; `TestEventEngineCallers` fails
  until both maps are exact.
- The never-reach walk is wide (it follows function values and bare names): check that `runLockdownCycle` and the other roots do not
  reach `lockdownAnnounceLift` or `raidPostNotice` once those call an adapter; if a root legitimately does, add a stop to
  `eventEngineRootStops` with its reason rather than removing the root.
- The paced sites must send the default with today's options (`raidSend`'s and `lockdownPostLiftNotice`'s, no reply parameters), not
  the builders' `formatting.Shtml()`, or the goldens break; `TestLockdownEventDefaults` already proves the lift text with the paced
  options.
- `lockdown.go` still has `replyLockdown` for `/unlockdown`'s reply and every refusal; the cannot-lock warning shares `raidSend` and must
  stay byte-identical (golden `cannot_lock`).

## Self-Check: PASSED

- Files: `fuku/modules/event_message_lockdown.go`, `fuku/modules/event_message_lockdown_test.go`,
  `fuku/modules/event_engine_callers_test.go` exist.
- Commits: 4bb1b39, f5341c1, c4b5ea7 are ancestors of HEAD.
