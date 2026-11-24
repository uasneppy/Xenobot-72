---
phase: 24-captcha-ai-spam-and-member-requirement-notices
plan: 05
subsystem: memberreq-notices
status: complete
tags: [event-messages, memberreq, notices, i-fixed-it, pm-copies]
requires:
  - 24-02 (postEventSent, the SendCustom/Schedule hooks, Required, TextOnly, memberReqNoticeDefault, memberReqNoticeNote)
  - 24-03 (the four requirement private copy specs)
  - 24-04 (memberReqNoticeAppended, the Requirements family on the Messages page)
provides:
  - memberReqPostEventNotice, the requirement site of the event engine, called by memberReqAnnounce
  - memberReqNoticeSend, the one paced retry loop of the custom notice, the default and the pause notice
  - memberReqEventOf and memberReqEventTarget
  - TestEventMessageMemberReq and TestEventMessageMemberReqMute
affects: [24-06, 24-07, 24-08]
tech-stack:
  added: []
  patterns:
    - "a site whose own send has a retry loop wraps both SendDefault and SendCustom in it, so a custom message gets the same pacing guarantees as the default"
key-files:
  created:
    - fuku/modules/event_message_mreq_test.go
  modified:
    - fuku/modules/memberreq_notice.go
    - fuku/modules/memberreq_enforce.go
    - fuku/modules/pm_copy_callers_test.go
    - AGENTS.md
decisions:
  - "KeepRows is set explicitly to the I fixed it rows with KeepRowsLast, although eventBotRows(Opts) would yield the same rows; the site, not the default's options, decides what a custom mute notice keeps."
  - "The mute budget (memberReqMuteNoticeBudget) applies per send: the custom message and the default each get their own 5 minute bound for rate-limit waits, as each notice did before."
  - "SendCustom ignores the per-call context (media.SendGreetingMessage takes none); each try is still paced and bounded by the HTTP client's timeout. Accepted."
  - "A refused custom message goes through the loop's one retry before the engine falls back to the default, so a refusal costs at most four requests (custom, retry, default, retry), as the plan accepted."
metrics:
  duration: ~45 min
  completed: 2026-11-22
actuals:
  tokens: 10900
  tasks: 2
  commits: 2
plan_head_before: 15bcc76dcd61c2014e912eaf2458b29ef60fbbb5
plan_head_after: 374e0da83da5c23316bfa205de8e95cfdfbae540
---

# Phase 24 Plan 05: The requirement notices through the event engine Summary

The member requirement warn, kick, ban and mute notices now go through the event engine. A group's custom text, filled with the
member, the broken rules, the action word and a warn's numbers, is posted as a new message after the punishment and is followed by a
blank line, the requirements line and the default's remedies. A custom mute notice always ends with the "I fixed it" row, cannot be
silenced, never carries a stored media file, is queued at the 24 hour cap and is the hold's stored notice, so the tap frees the member,
edits it and reschedules it from the tap. With nothing stored every requirement notice and the pause notice send today's bytes.

## Tasks

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | the warn, kick and ban notices through the engine | 080e3ad | memberreq_notice.go, memberreq_enforce.go, event_message_mreq_test.go (new), pm_copy_callers_test.go, AGENTS.md |
| 2 | the mute notice keeps "I fixed it" | 374e0da | event_message_mreq_test.go, AGENTS.md |

## What was built

- `memberReqNoticeSend(ctx, withButton, send)`: the loop `memberReqPostNotice` had (a mute waits out `ErrRateLimited` within
  `memberReqMuteNoticeBudget`, every other failure is tried once more, a buttonless notice is not retried after `ErrRateLimited`), with no
  scheduling. `memberReqPostNotice` is now its wrapper and is used only by the pause notice (`memberreq_limits.go`).
- `memberReqPostEventNotice(ctx, b, c, tr, user, hold, warn) (int64, error)`: the site of the planner assumptions (no actor, no reply, no
  topic, `bot:punish_auto`, `Automatic`, both sends in the loop, `Schedule` = `memberReqScheduleNotice`) and the reply (event from
  `memberReqEventOf`, target from `memberReqEventTarget`, `{reason}` = `memberReqRulesText`, `{action}` the punishment's or the applied limit
  action's word, `{count}`/`{limit}` for a warn, `Note` = `memberReqNoticeAppended(action)(tr, in)`, `TextOnly` always, `Required` and
  `KeepRows`+`KeepRowsLast` for the mute). It returns the posted message's ID, or 0 for a Silent notice.
- `memberReqAnnounce` calls it and stores a non-zero ID; a Silent notice stores none; an error keeps today's handling (a mute goes to
  `memberReqUndeliveredMute`, anything else is logged by IDs).
- `memberreq_fixed.go` is unchanged (`git diff --exit-code 86baf1a -- fuku/modules/memberreq_fixed.go` exits 0).

## Deviations from Plan

None - plan executed exactly as written.

### Earlier tests whose expected values changed

None. `pm_copy_callers_test.go` gained one allow-list entry (`postEventSent`: `memberreq_notice.go`, with its reason), which the plan
requires; no assertion changed.

No golden changed: `git diff --exit-code e40a7be -- fuku/modules/screening_notice_legacy_request_test.go`,
`git diff --exit-code 9d4554d -- fuku/modules/punishment_notice_legacy_request_test.go` and
`git diff --exit-code 15bcc76 -- fuku/modules/moderation_legacy_request_test.go` all exit 0.

## Verification (with durations)

- Task 1 verify list (`TestEventMessageMemberReq`, `TestLegacyScreeningNoticeRequests`, the notice, cap, warn recovery, replicas,
  `TestPMCopyCallers`, `TestCommandRepliesStayInsideTheHandler`): PASS (7.6 s package, 10.3 s wall); 5 subtests of
  `TestEventMessageMemberReq` pass.
- Task 2 verify list (both new tests, `TestMemberReqFixed*`, mute notice at cap, retry, undeliverable, the marker check, refused punishment,
  the golden): PASS (18.4 s package, 20.7 s wall); 4 subtests of `TestEventMessageMemberReqMute` pass.
- New tests' wall time: `TestEventMessageMemberReq` 1.7 s (1.5 s of it the first real-locale environment), `TestEventMessageMemberReqMute`
  0.2 s; about 1.9 s against the plan's 4 s.
- Mutation checks: dropping `Required` or `TextOnly` fails `silent and media are ignored`; scheduling with the plain timer instead of
  `memberReqScheduleNotice` fails both custom mute subtests (24 hour cap); dropping the `Note` fails the warn, warn limit and mute
  subtests. (Dropping `KeepRows` alone is harmless by construction: the default's options carry the same rows.)
- Touched families with `-race` (`TestMemberReqNotice|Enforce|Fixed|Cap|Mute|Limits|Punish|Undelivered|Warn|Screens.*`,
  `TestEventMessage.*`, `TestEventPM.*`, `TestPMCopy.*`, `TestLegacy.*`, `TestSettingsMessages.*`, `TestScreeningEventDefaults` and the
  three end-to-end tests): PASS (56.4 s package, 59.2 s wall).
- `make check-translations` all present (0.4 s); `make check-docs` no drift (1.4 s); `make test-shard-check` clean (23.7 s);
  `CGO_ENABLED=0 go build ./...` ok (8.9 s); `git diff --exit-code 86baf1a -- go.mod go.sum main.go migrations` exits 0.
- golangci-lint v2.13.1: `--new-from-rev=origin/main ./...` 0 issues; the same with `--build-tags=notesttools` 0 issues (22.8 s);
  `--new=false` over `./fuku/modules/...` with no issue cap: no finding in `memberreq_notice.go`, `memberreq_enforce.go`,
  `event_message_mreq_test.go` or `pm_copy_callers_test.go` (only baseline findings elsewhere).
- Full `go test -tags testtools -race -count=1 ./fuku/modules`: ok, 359.2 s package (6 m 2 s wall).

## Notes for plan 24-06 (the AI spam site)

- `TestPMCopyCallers`: `aispam_notice.go` needs its own allow-list entry for `postEvent` or `postEventSent`, with its reason.
- The requirement site is the pattern for a site with its own pacer: wrap both `SendDefault` and `SendCustom` in the pacer (here
  `memberReqNoticeSend`). `media.SendGreetingMessage` takes no context, so the custom send cannot honour a per-call deadline; the pacer
  still spaces it.
- `aispamEventReason` keeps its `//nolint:unused` marker until 24-06 calls it.
- The AI notice is off by default: with no row `Resolve` reads Silent, so the group step posts nothing and `Admit` is not asked
  (`eventGroupWouldPost`). The site must set `Admit` (the per-member and per-minute caps, fail closed) and leave `Required` false.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register. T-24-15: `KeepRows`/`KeepRowsLast`, `Required` and `TextOnly` are set by the site
(`custom keeps the button`, `silent and media are ignored`). T-24-16: `memberReqEventTarget` strips bidirectional controls and caps the
names (24-07's privacy tests use such a name). T-24-17: the custom send goes through the same loop as the default; only a refusal falls
back, and only both failing lifts the mute (`undelivered`). T-24-18: `refused` proves the golden default follows a refusal. T-24-19:
`memberreq_fixed.go` unchanged and `TestMemberReqFixedReschedulesOnlyTheNotice` passes unchanged.

## Self-Check: PASSED

- Files: `fuku/modules/event_message_mreq_test.go` exists; every modified file is in the two commits.
- Commits: 080e3ad and 374e0da are ancestors of HEAD; `git rev-list --count 15bcc76..HEAD` before this summary is 2.
