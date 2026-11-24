---
phase: 24-captcha-ai-spam-and-member-requirement-notices
plan: 06
subsystem: aispam-notice
status: complete
tags: [event-messages, aispam, notices, rate-limit, pm-copies, locales]
requires:
  - 24-02 (postEvent, Admit, OffByDefault, aispamNoticeDefaultReply, aispamEventReason)
  - 24-03 (the aispam private copy spec)
  - 24-04 (the Messages page's Notice switch for the aispam row)
  - 24-05 (the paced-site pattern, the TestPMCopyCallers allow-list shape)
provides:
  - postAISpamNotice, the AI spam site of the event engine, called last in processAISpam's delete branch
  - aispamNoticeAllow and aispamNoticeScript (per-member 5 minute latch, per-group 3 a minute, fail closed)
  - aispamNoticePacer (fuku:aispam:notice:pace:*, 200 ms, 1 retry, MaxWait 2 s)
  - aispamTask.user and aispamTask.threadID
  - TestAISpamNotice
affects: [24-07, 24-08]
tech-stack:
  added: []
  patterns:
    - "an off-by-default event whose Admit is a fail-closed Lua cap: the engine asks it only when the notice or its private copy is on, so a group that never turned it on spends no Redis command"
key-files:
  created:
    - fuku/modules/aispam_notice.go
    - fuku/modules/aispam_notice_test.go
  modified:
    - fuku/modules/aispam.go
    - fuku/modules/event_message_auto.go
    - fuku/modules/pm_copy_callers_test.go
    - locales/{en,es,fr,hi,id,pt,ru,uk}.yml
    - docs/src/content/docs/commands/aispam/index.md
    - AGENTS.md
decisions:
  - "The mirror and the burst count still run after a failed delete, as before; only the notice is gated on the delete returning no error."
  - "The default notice's options get MessageThreadId from the task; SendCustom inherits the topic through site.ThreadID (postEventGroup sets msg.ThreadID)."
  - "A send error is logged at Debug with the chat and user IDs and pmCopyErrCode's Telegram code, never the error text."
  - "The help names the group owner as the one who turns the notice on (always true; an admin the owner gate admits with can_change_info can too)."
metrics:
  duration: ~50 min
  completed: 2026-11-22
actuals:
  tokens: 6500
  tasks: 2
  commits: 2
plan_head_before: 8346db6de757e1a37f41488add61af6abeb23d5b
plan_head_after: 6f23fffe353c4d20151cd667c8ba99e0add3f40e
---

# Phase 24 Plan 06: The AI spam notice Summary

`/aispam` can now post a notice after it deletes a spam message, but only in a group that turned the AI spam notice on from
the Messages page. The notice names the member (or is the group's own text), goes into the deleted message's topic and is never
a reply. It is queued under `bot:punish_auto`. A group gets at most one notice per member every 5 minutes and three a minute,
through one Lua script that fails closed. Both sends go through the notice's own fleet pacer on the pool's context. A group that
never turned the notice on behaves exactly as before and spends no Redis command on it. Every spam message is deleted, whatever
happens to the notice.

## Tasks

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | the AI spam notice, capped, paced and off by default | aacd051 | aispam_notice.go (new), aispam_notice_test.go (new), aispam.go, event_message_auto.go, pm_copy_callers_test.go, AGENTS.md |
| 2 | the /aispam help, its doc page and the AGENTS.md lines | 6f23fff | locales/*.yml (8), docs/.../commands/aispam/index.md, AGENTS.md |

## What was built

- `aispam_notice.go`: the limits `aispamNoticePerMinute` (3), `aispamNoticeUserWindow` (5 min), `aispamNoticeNow`,
  `aispamNoticeRedisClient` (`raidRedis`), the keys `fuku:aispam:notice:{<chat>}:u:<user>` and `:m:<minute>`, `aispamNoticeScript`
  (SET NX EX 300 on the member key, else 0; INCR plus EXPIRE 120 on the minute key, 1 while at most 3), `aispamNoticeAllow`
  (bounded by `raidRedisTimeout`, false on no client, an error or a refusal), the pacer (`aispamNoticePacerOptions`,
  `aispamNoticePacer`, `aispamNoticePaced` with a 10 s per-call bound) and `postAISpamNotice(task)`.
- The site: the task's chat, the group's translator, no actor, no reply, `ThreadID` the topic, `bot:punish_auto`, `Automatic`,
  `Admit` the caps, `SendDefault` around `helpers.SendMessageWithErrorHandling` and `SendCustom` around `media.SendGreetingMessage`,
  both inside `aispamNoticePaced(aispamContext(), ...)`. The reply is event `aispam`, the member as the target, `{reason}`
  `aispamEventReason(tr)`, `{action}` the delete word, and the default from `aispamNoticeDefaultReply` with the topic added. `Required`
  is false.
- `aispam.go`: `aispamTask.user` and `aispamTask.threadID` (only for a topic message) are filled in `checkAISpam`.
  `processAISpam` keeps the delete error and calls `postAISpamNotice` last, after the mirror and `raidAIRecordVerdict`, only when
  the delete returned no error. Nothing else changed: the keep and failure branches, `aispamNotifyBreaker` and the TypeSafe request
  are as before.
- `aispamEventReason` lost its `//nolint:unused` marker. `TestPMCopyCallers` lists `aispam_notice.go` for `postEvent`, with its
  reason.
- `aispam_help_msg` in all 8 languages: the filter only deletes, unless the group owner turns on the AI spam notice on the
  Messages page of `/settings` in the bot's private chat. The notice is then limited to one per member every 5 minutes and 3 a
  minute. The `/aispam` doc page gained "The AI Spam Notice" section.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] The `/aispam` doc page is frozen, so it was edited by hand**
- **Found during:** Task 2
- **Issue:** `docs/src/content/docs/commands/aispam/index.md` carries `<!-- MANUALLY MAINTAINED: do not regenerate -->`, so
  `make generate-docs` does not touch it and there was no regenerated page to commit. Its "What Happens To A Flagged Message"
  section still said "no notice to the sender".
- **Fix:** I edited the page by hand. The "no notice" clause is gone, and a new section, "The AI Spam Notice", gives the same
  facts as the help. `make check-docs` reports no drift.
- **Commit:** 6f23fff

**2. [Rule 1 - Test design] The `shutdown` subtest cancels the pool at the delete, not inside the stand-in**
- **Found during:** Task 1
- **Issue:** The plan's stand-in cancels the pool's context before it answers. TypeSafe's request derives from that context, so
  the check would be cut off as shed, with no delete. That contradicts the expected "deleted, no notice", and whether the answer
  arrives before the transport notices the cancel is a race.
- **Fix:** A thin client wrapper (`aispamNoticeCancelClient`) calls `aispamCancel()` right after `deleteMessage`. This is the case
  the subtest is meant to prove: a shutdown that lands between the deletion and the notice. The subtest also checks that no cap
  key was written. `restoreAISpamPool` restores the pool.
- **Commit:** aacd051

**3. [Plan ordering] The AGENTS.md lines for the notice went into task 1's commit**
- The plan put every AGENTS.md change in task 2. The executor rules require AGENTS.md changes in the same commit as the code they
  describe, so the Data, DrainAISpamChecks, Scheduling, Event messages and test-family lines are in aacd051. Task 2 adds only the
  help and doc-page note.

**4. [Scope] `event_message_auto.go` was edited although the plan's file list leaves it out**
- This is the marker removal that 24-04 and 24-05 required. The function body is unchanged.

**5. [Test text] The `custom` subtest's stored text is `{fullname} ({mention}) {action}: {reason}`**
- The Welcome placeholder `{mention}` renders a member's `@username` when they have one, and the golden member has one. I added
  `{fullname}` so the test also checks the escaping of `Ann & <Bo> Lee`. The assertion is exact.

### Earlier tests whose expected values changed

None. `pm_copy_callers_test.go` gained one allow-list entry (`postEvent`: `aispam_notice.go`, with its reason), as the plan
requires. No assertion changed.

No golden changed: `git diff --exit-code e40a7be -- fuku/modules/screening_notice_legacy_request_test.go`,
`git diff --exit-code 9d4554d -- fuku/modules/punishment_notice_legacy_request_test.go` and
`git diff --exit-code 8346db6 -- fuku/modules/moderation_legacy_request_test.go` all exit 0. The last change to the moderation
golden is 2dcb0c7. `git diff --exit-code 86baf1a -- go.mod go.sum main.go migrations` exits 0.

## Verification (with durations)

- Task 1 verify list (`TestAISpamNotice`, the five `TestCheckAISpam*` and breaker tests, `TestShutdownDrainsAISpamChecks`,
  `TestLegacyScreeningNoticeRequests`, `TestBotAutoPunishmentNotices`, `TestRaidAITextRequestIsMinimal`, `TestPMCopyCallers`,
  `TestCommandRepliesStayInsideTheHandler`): PASS (5.9 s package, 8.4 s wall). No SKIP.
- `go test -run '^TestAISpamNotice$' -v` prints 7 `--- PASS: TestAISpamNotice/` lines. The test takes about 1.5 s, against a
  3 s budget.
- Mutation checks: an `Admit` that always allows fails `caps`, `quiet failures` and `private copy`. Dropping the delete-error
  guard and the pool-context check fails `quiet failures` and `shutdown`.
- Task 2: `make check-translations` all present (0.3 s); `make check-docs` no drift (1.5 s); `TestHelpPagesKeepTheirCommands`,
  `TestLocaleStyle`, `TestLocalePlaceholdersMatchOtherLanguages` PASS (17.1 s); the whole `./fuku/i18n` package PASS (53.3 s);
  `TestAISpamIsReachableFromHelp` and the `Help` tests of `fuku/modules` PASS. The `aispam_help_msg` line in `en.yml` no longer
  contains `no notice` (count 0).
- Touched families with `-race` (`TestAISpam.*`, `TestCheckAISpam.*`, `TestEventMessage.*`, `TestEventPM.*`, `TestPMCopy.*`,
  `TestLegacy.*`, `TestSettingsMessages.*`, `TestRaidAI.*`, `TestShutdownDrainsAISpamChecks`, `TestScreeningEventDefaults`,
  `TestBotAutoPunishmentNotices` and the three end-to-end tests): PASS (56.2 s package, 58.6 s wall).
- `make test-shard-check` clean (25.0 s); `CGO_ENABLED=0 go build ./...` ok (6.8 s).
- golangci-lint v2.13.1: `--new-from-rev=origin/main ./...` reports 0 issues (3.2 s), and the same with
  `--build-tags=notesttools` reports 0 issues (22.4 s). `--new=false` over `./fuku/modules/...` with no issue cap gives 110
  baseline findings, none of them in `aispam.go`, `aispam_notice.go`, `aispam_notice_test.go`, `event_message_auto.go` or
  `pm_copy_callers_test.go`.
- Full `go test -tags testtools -race -count=1 ./fuku/modules`: ok, 366.5 s package (6 m 9 s wall).

## Notes for plans 24-07 and 24-08

- The AI notice site lives in `aispam_notice.go`. A privacy test (T-24-22) can drive it the way `TestAISpamNotice` does
  (`newAISpamNoticeEnv`, `spam`, `turnOn`, `notices`). Set a sentinel category and model in the stand-in's answer, then check that
  none reaches the notice, its private copy or a log line. The site logs only chat and user IDs and the Telegram code.
- The env swaps `raidSettingsLookup` to turn off the AI burst trigger. Without that swap, seven delete verdicts from three or more
  accounts in 120 s start a lockdown, and its public notice would count as a group send. An end-to-end test with several spammers
  needs the same swap, or must tell notices apart by their text.
- The per-member latch is a real Redis TTL (miniredis time). Only the minute is clocked (`aispamNoticeNow`). To prove that a
  member gets a second notice after 5 minutes, use `miniredis.FastForward`.
- 24-08's end-to-end test should turn the notice on through the Messages page's Notice toggle (24-04), not the repository, and
  confirm that the next deletion posts.
- The open gap left by 24-05 still stands: no test makes a custom requirement mute notice wait out `ErrRateLimited`. It is not
  related to this plan.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register:
- T-24-20: `caps` and `quiet failures`.
- T-24-21: the notice is the last step, only after a successful delete, and the `TestCheckAISpam*` and raid AI tests pass
  unchanged.
- T-24-22: the default text and the values carry only the member and fixed translated words.
- T-24-23: `shutdown`, and `TestShutdownDrainsAISpamChecks` passes unchanged.
- T-24-24: the event is off by default (`off by default`).
- T-24-32: the site is in `aispam_notice.go`, listed in `TestPMCopyCallers`. No raid file names the engine.

## Self-Check: PASSED

- Files: `fuku/modules/aispam_notice.go` and `fuku/modules/aispam_notice_test.go` exist.
- Commits: aacd051 and 6f23fff are ancestors of HEAD. `git rev-list --count 8346db6..HEAD` before this summary is 2.
