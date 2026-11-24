---
phase: 23-warn-flood-blacklist-and-forward-blocklist-messages
plan: 04
subsystem: event-messages
status: complete
tags: [event-messages, warns, antiflood, blacklists, forward-blocklist]
requires:
  - 23-01 (TestLegacyPunishmentNoticeRequests and its toolkit)
  - 23-02 (eventSite, postEvent, the default builders, eventActionWord, floodEventReason)
  - 23-03 (the Automatic family on the Messages page)
provides:
  - warnLimitEventReply and the warnlimit branch of warnThisUser
  - floodNotice (antiflood.go)
  - blacklistNotice (blacklists.go)
  - forwardBlockNotice(b, ctx, userID, action) through postEvent
  - eventSenderTarget, listNoticeSite, listNoticeReply (event_message_auto.go)
  - TestEventMessageWarnLimit, TestEventMessageFlood, TestEventMessageBlacklist, TestEventMessageForwardBlock
affects: [23-05, 23-06, 23-07]
tech-stack:
  added: []
  patterns:
    - "an automatic notice's site is built in its own helper (floodNotice, blacklistNotice, forwardBlockNotice), so the watcher keeps its complexity and only calls the helper after the punishment"
    - "an inert mode is not an event: the site still builds its eventReply but sends it through sendDefaultPost (flood) or keeps its old function (warnPostLimit)"
key-files:
  created:
    - fuku/modules/event_message_auto_sites_test.go
  modified:
    - fuku/modules/warns.go
    - fuku/modules/antiflood.go
    - fuku/modules/blacklists.go
    - fuku/modules/forwardblock_notice.go
    - fuku/modules/staff_forwardblock_watch.go
    - fuku/modules/event_message_auto.go
    - fuku/modules/pm_copy_callers_test.go
    - AGENTS.md
decisions:
  - "The flood notice moved into floodNotice; checkFlood keeps its exemptions, deletes and punishment switch and lost the fmode variable (floodDefaultReply derives the English word from the action). gocyclo stays at 31."
  - "blacklistWatcher's three inline notice blocks became one post-switch punishment-error check and one blacklistNotice call; the warn case returns ext.ContinueGroups itself and none plus the inert actions are one default case. gocyclo fell from 35 to 30."
  - "The blacklist and the forward blocklist share listNoticeSite and listNoticeReply in event_message_auto.go, but each file calls postEvent itself, so TestPMCopyCallers sees each caller."
  - "eventSenderTarget names a sender as Sender.Id and Sender.Name do (a sender chat's ID and title, a user's names and username), so the default and custom notices name the same sender; forwardBlockNotice sets the target's ID to the userID it is given, as the default's mention did."
  - "The flood's inert actions still build the eventReply and send it through sendDefaultPost (the same SendMessageWithErrorHandling call and the same queue row); the warn limit's inert mode keeps warnPostLimit untouched."
metrics:
  duration: ~55 min
  completed: 2026-11-21
actuals:
  tokens: 12600
  tasks: 3
  commits: 3
plan_head_before: 7d594861fdc93d211cba50e4cd99f519d17e30dd
plan_head_after: 5fb7d3847a4effcaa62e129fc8f6c16dd1e03587
---

# Phase 23 Plan 04: The warn limit, flood, blacklist and forward blocklist sends through the engine Summary

The warn limit (when its action took effect), the flood notice (mute, kick, ban), the blacklist's mute, kick and ban notices and the
forward blocklist's notices now post through `postEvent`. A group's stored message replaces the default with `{action}`, `{reason}` and,
where they apply, `{count}` and `{limit}` filled. The flood keeps its Unmute or Unban row first. The forward blocklist message never names
the source or the Staff Group. With nothing stored, every send is the golden request byte for byte, and the inert modes keep today's send.

## Tasks

| Task | Name | Commit |
|------|------|--------|
| 1 | the warn limit's message through the engine | 82cddbc |
| 2 | the flood notice through the engine | fcdb533 |
| 3 | the blacklist and forward blocklist notices through the engine | 5fb7d38 |

## What later plans use

| Kind | Name | Notes |
|------|------|-------|
| Site | `warnThisUser`'s `switch`: `res.LimitApplied` uses `postEvent(warnSite(...), warnLimitEventReply(tr, msg.MessageId, u, reason, res))`, `res.LimitReached` alone uses `warnPostLimit` (unchanged) | the warn limit's site is `warnSite`, so `Automatic` follows the warn's source |
| Site | `floodNotice(b, chat, msg, tr, sender, flood, count, rows)` | `ThreadID` and the default's `MessageThreadId` are the message's topic, `bot:punish_auto`, `Automatic` true, default through `helpers.SendMessageWithErrorHandling` |
| Site | `blacklistNotice(b, ctx, user, action, reason)` | `reason` is `fmt.Sprintf(reason, i)`, escaped only by the engine |
| Site | `forwardBlockNotice(b, ctx, userID, action)` | `action` is `"mute"`, `"kick"` or `"ban"`; reason `forwardBlockReason(tr)` |
| Helpers | `eventSenderTarget(sender)`, `listNoticeSite(b, chat, tr)`, `listNoticeReply(tr, event, target, action, reason)` | `event_message_auto.go` |
| Test toolkit | `eventMsgAutoWarnSettings`, `eventMsgAutoSilent`, `eventMsgAutoPunishedFirst`, `eventMsgAutoCustom`, `eventMsgAutoGolden`, `eventMsgAutoFlood` (flood driver with a topic), `eventMsgAutoRow`, `eventMsgAutoDeleted`, the texts `eventMsgAutoWarnLimitText`, `eventMsgAutoFloodText`, `eventMsgAutoListText`, `eventMsgAutoEveryPlaceholder` | `event_message_auto_sites_test.go` |

## Earlier tests whose expected sets changed (no assertion weakened)

- `fuku/modules/pm_copy_callers_test.go`, `TestPMCopyCallers`: the `postEvent` allow-list gains `antiflood.go` (Task 2), `blacklists.go` and
  `forwardblock_notice.go` (Task 3), each with its reason. The check itself, the forbidden names and prefixes are unchanged. This edit is the
  one the plan requires; no other earlier test changed.

## Verification (all with `-tags testtools`)

- Task 1 verify (`TestEventMessageWarnLimit|TestEventMessageWarn|TestLegacyPunishmentNoticeRequests|TestWarnThisUserSource|TestApplyWarn`, `-race`):
  ok, 5.5 s (47 s with build); `TestEventMessageWarnLimit -v` 4 subtests PASS.
- Task 2 verify (`TestEventMessageFlood|TestLegacyPunishmentNoticeRequests|TestBotAutoPunishmentNotices|TestPMCopyCallers|TestAutoEventDefaults|TestAntiflood.*|TestApplyFlood.*|TestUpdateFloodWithSettingsReplacesIdleCounter`, `-race`):
  ok, 16 s (56 s with build); `TestEventMessageFlood -v` 4 subtests PASS.
- Task 3 verify (`TestEventMessageBlacklist|TestEventMessageForwardBlock|TestLegacyPunishmentNoticeRequests|TestBotAutoPunishmentNotices|TestWarnThisUserSource|TestPMCopyCallers|TestMemberDeletionExcludedMessages|TestForwardBlockWatcher.*|TestForwardBlockSourceOf|TestCommandRepliesStayInsideTheHandler`, `-race`):
  the first run failed only the watcher subtest's expectation (the staff environment's marker locale, fixed in the test before the commit),
  then `TestEventMessageBlacklist` 4 and `TestEventMessageForwardBlock` 2 subtests PASS (4 s).
- Families with `-race` (`TestEventMessage`, `TestEventPM`, `TestEventActionWords`, `TestAutoEvent`, `TestWarn`, `TestApplyWarn`,
  `TestSetWarn`, `TestAntiflood`, `TestApplyFlood`, `TestUpdateFlood`, `TestFlood`, `TestBlacklist`, `TestApplyBlacklist`, `TestForwardBlock`,
  `TestApplyForwardBlock`, `TestSettingsMessages`, `TestSettingsDriftMessages`, `TestSettingsAuthorityMessages`, `TestMessagesEndToEnd`,
  `TestPMCopy`, `TestLegacy`, `TestMemberDeletionExcludedMessages`, `TestBotAuto`, `TestBotManual`, `TestCommandRepliesStayInsideTheHandler`,
  `TestRenderEventText`): ok, 136 s (2 min 19 s with build), 149 top-level PASS, no FAIL or SKIP.
- Goldens unchanged: `git diff --exit-code 9d4554d -- fuku/modules/punishment_notice_legacy_request_test.go` and
  `git diff --exit-code 7ecc5fd~1 -- fuku/modules/moderation_legacy_request_test.go` exit 0.
- `git diff --exit-code 1a382c3 -- go.mod go.sum main.go migrations` exits 0.
- Acceptance greps: `eventmsg.EventWarnLimit` in warns.go 1; `postEvent(` in antiflood.go, blacklists.go, forwardblock_notice.go 1 each;
  `eventmsg.EventFlood` in antiflood.go 1; the 23-04 nolint marker 0; `postEvent|eventSite|eventReply` in staff_forwardblock_watch.go 0; its
  diff against 1a382c3 is 6 changed lines (the three calls); the allow-list names 6 matches (at least 5).
- `make check-translations` all present (0.4 s), `make check-docs` no drift (1.3 s), `make test-shard-check` rc 0 (28 s),
  `CGO_ENABLED=0 go build ./...` ok (3 s).
- golangci-lint v2.13.1 (`CGO_ENABLED=0`): `run --new-from-rev=origin/main ./...` 0 issues (3 s, warm cache); with `--build-tags=notesttools`
  0 issues (27 s); `run --new=false ./fuku/modules/...` filtered to the touched files (41 s) prints exactly the four baseline findings: the
  `dupl` of `blacklists.go` with `approvals.go`, `gocyclo` of `checkFlood` at 31 (unchanged), `addBlacklist` at 23 (unchanged) and
  `blacklistWatcher` at 30 (was 35).
- gofmt clean on every touched Go file. No locale key was added. The full `fuku/modules` package was not run (plan 23-07 runs it).

## Deviations from Plan

**1. [Choice] Shared site and reply for the two list notices**
- The plan described the forward blocklist's site as "the same as the blacklist's". Both now come from `listNoticeSite` and `listNoticeReply`
  in `event_message_auto.go`, and each file still calls `postEvent` itself (the guard counts each caller).

**2. [Choice] `eventSenderTarget`**
- A small helper turns the punished sender into the custom message's member for the flood, the blacklist and the forward blocklist: a user's
  ID, names and username, or a sender chat's ID and title (the planner's `{Id, FirstName: title}`).

**3. [Addition] The forward blocklist sentinel check uses a real Staff Group and a real forward origin**
- `forward block custom` creates a Staff Group with a sentinel title linked to the site's group and gives the message a `ForwardOrigin` of a
  channel with a sentinel title and ID, then checks the whole request names none of them. The watcher subtest also runs
  `fwdForbiddenInText` over the custom notice.

**4. [Repo rule] AGENTS.md in each task's commit**
- The Event messages bullet, the PM copies allow-list sentence and the Phase 23 test families bullet changed in each task's commit with the
  behaviour they describe; the forward blocklist watcher bullet changed in Task 3.

No Rule 1-4 deviations. No architectural change.

## Notes for plan 23-05

- Every site passes `Automatic` on its `eventSite`: `warnSite` (warn and warn limit) sets it for `warnSourceAuto`; `floodNotice` and
  `listNoticeSite` set it to true. 23-05's `ByRule` can read `site.Automatic` in `offerReplyPM` with no site change.
- `postEvent` already calls `offerReplyPM` for every new site, so registering the private copy specs of `warnlimit`, `flood`, `blacklist` and
  `fwdblock` is enough to start offering copies; until then `eventPMSpecFor` answers false and nothing is offered.
- The private copy needs the event values these sites already set: the warn limit's `Count`, `Limit`, `Action` and `Reason`; the flood's
  `Count`, `Limit`, `Action` and `Reason` (`floodEventReason`); the blacklist's and forward blocklist's `Action` and `Reason`.
- A copy's target is `r.Target`: for the flood and blacklist it is `eventSenderTarget` of the sender, so a sender chat's ID is negative and
  `pmCopySkipTarget` refuses it (no private copy to a channel), which is the intended behaviour; worth one assertion.
- The forward blocklist's copy must name neither the source nor the Staff Group: the site holds only the fixed reason and the member, and
  `TestEventMessageForwardBlock`'s sentinel setup (a linked Staff Group with a sentinel title and a forward origin with a sentinel source) can
  be reused for the private copy, the Details record and the log line.
- `TestPMCopyCallers`: the `postEvent` allow-list is complete for the five events (`event_message.go`, `warns.go`, `antiflood.go`,
  `blacklists.go`, `forwardblock_notice.go`); 23-05 adds no caller unless it adds a site.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register. T-23-09: `TestEventMessageForwardBlock` checks a stored text with every placeholder against a sentinel source
ID and title and a sentinel Staff Group title and ID. T-23-10: every custom subtest asserts the punishment call precedes the notice. T-23-11:
the blacklist subtests assert the escaped quote. T-23-12: the flood `refused` subtest sends the golden default once. T-23-05: no staff file
names `postEvent`, and `staff_forwardblock_watch.go` changes only the three arguments.

## Self-Check: PASSED

- Files: `fuku/modules/event_message_auto_sites_test.go` exists.
- Commits: 82cddbc, fcdb533 and 5fb7d38 are ancestors of HEAD.
