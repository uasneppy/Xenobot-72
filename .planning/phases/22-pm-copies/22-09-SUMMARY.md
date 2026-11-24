---
phase: 22-pm-copies
plan: 09
subsystem: event-messages-pm-copies
status: complete
tags: [pm-copies, e2e, phase-gates, PMSG-01, PMSG-02, PMSG-03, PMSG-04, PMSG-05, PMSG-06, PMSG-07]
requires: [22-01, 22-02, 22-03, 22-04, 22-05, 22-06, 22-07, 22-08]
requirements-completed: [PMSG-01, PMSG-02, PMSG-03, PMSG-04, PMSG-05, PMSG-06, PMSG-07]
provides:
  - TestPMCopiesEndToEnd (11 subtests) and its toolkit pmCopiesE2E on Phase 21's messagesE2E
  - messagesE2EClient.allow (a refused member's sends go through again once they started the bot)
  - the final Phase 22 AGENTS.md pass (Testing, PM copies, the pmcopy_ deep link, Permissions)
affects: [22]
tech-stack:
  added: []
  patterns:
    - "the Details flow is driven through the real dispatcher: the token is read from the group message's edit, the member's real /start pmcopy_<token> shows the copy, another member's gets the neutral reply"
    - "a member who never started the bot is a send refusal in the client wrapper (no request reaches the fake), lifted when the member starts the bot"
key-files:
  created:
    - fuku/modules/pm_copies_e2e_test.go
  modified:
    - AGENTS.md
decisions:
  - "No production change: every subtest passed against the code of plans 22-01 to 22-08, and four mutations of production code (the Details owner check removed, the log line off, the refused-custom fallback off, the pm switch ignored) each failed the subtest that guards it."
  - "owner sets a private copy runs /ban spam <b> instead of the plan's /ban spam & co, so the group reply is compared byte for byte with goldenModerationBanReason (real locales) and the copy's {reason} proves the escaping on the same reason."
  - "default text sets the group's language to Spanish and its zone to Asia/Tokyo, so 'in the group's language' and 'in the group's zone' are both observed (a Ukrainian default would equal the bot's default language)."
  - "group silent bans a member who started the bot (the copy is sent with no group message) and one who blocked it (no edit and no Details record, because there is no group message to edit)."
  - "previews expects the title Test Chat: a private menu knows its group from getChat, which the shared fake answers with that title; the send path uses the update's chat (Lock Chat). Not a defect: in production both are the group's real title."
metrics:
  duration: ~95 min
  completed: 2026-11-21
actuals:
  tokens: 7500
  tasks: 2
  commits: 2
plan_head_before: 692c978adcda4ae8e2a6ba159d7e3ff89a55e8fc
plan_head_after: 46e018171e92cfa2984529c548bb0fced6de86db
---

# Phase 22 Plan 09: Private copies end to end Summary

The phase's five success criteria are now observed through the running bot: the owner types a private copy's text and switches
it on from the private Messages page, and an administrator's `/ban` replying to a member bans, posts today's group reply byte for
byte and then sends the member one private message with every placeholder filled and escaped and no Unban row; the private copy's
Full preview reaches the owner alone; a timed mute with nothing stored sends the default private text in the group's language with
the end in the group's zone; a member who never started the bot changes nothing but gets, with the options on, a Details button
under the group reply that only they can open through `/start pmcopy_<token>` and one `#PM_COPY` line in the log channel; a refused
custom copy falls back to the default once; a silent group event still sends the copy, and the silent variants send none; every
change needs `can_change_info` on every press and typed message; a Staff Group fan-out sends the target nothing; a switch turned
off on one replica stops the next copy on another; and without Redis the switch, the option and a manual copy work while no
Details button is added. No subtest found a defect, so no production code changed.

## Tasks

| Task | Name | Commit |
|------|------|--------|
| 1 | `TestPMCopiesEndToEnd` and the `pmCopiesE2E` toolkit (AGENTS.md Testing sentence in the same commit) | 037123b |
| 2 | Phase gates and the final AGENTS.md pass (PM copies, deep link and Permissions bullets name the test) | 46e0181 |

## Requirements

| Requirement | Proved by |
|-------------|-----------|
| PMSG-01 (each event's private copy with its own text, media, buttons and switch, off by default) | `TestPMCopiesEndToEnd/owner_sets_a_private_copy` (typed text stored with the copy still off, the switch from the page, the hub's one private copy mark, the member's copy filled and escaped after the group reply), `.../refused_custom` (a typed photo), `.../default_text`, `.../two_replicas`; `TestSettingsMessagesPMScreen`, `TestSettingsMessagesPMSwitch`, `TestSettingsMessagesPMTyped`, `TestEventMessagesPM` |
| PMSG-02 (best effort: a person who never started or blocked the bot changes nothing) | `.../not_started` (the 403 member is banned once and the group reply posted as usual), `.../group_silent`, `.../no_redis`; `TestEventPMFailures` (a 403 of either wording with one request and the golden group reply) |
| PMSG-03 (its own fleet-wide pacing, never before or instead of the punishment) | `.../owner_sets_a_private_copy` (the copy after the ban and after the group reply), `.../refused_custom`; `TestEventPMPacerIndependent` (a block on `fuku:pmcopy:pace:block` leaves the ban and the golden reply immediate; other pacers' blocks leave the copy alone), `TestEventPMCost` (the handler returns before any copy request) |
| PMSG-04 (Details button on an undelivered copy; only the punished person gets the content) | `.../not_started` (the edit of the group reply keeps the Unban row and adds Details, the member's real `/start pmcopy_<token>` shows the stored copy, another member's gets only `eventmsg_pm_details_none`), `.../group_silent` and `.../no_redis` (no button when there is no group message or no Redis); `TestEventPMDetails`, `TestPMCopyDetailsDeepLink`, `TestSettingsMessagesPMOptions` |
| PMSG-05 (no copy for joiner bans, Ban N recent joiners or Staff Group actions; a cap for automatic events) | `.../staff_group_action` (a staff `/ban` fan-out to a linked group whose ban copy reads on bans and sends the target nothing), `.../silent_variants`; `TestPMCopyCallers` (no `staff`, `lockdown` or `raid` file names the copy engine), `TestEventPMLimits` (the automatic cap per chat and minute, fail closed without Redis) |
| PMSG-06 (never the Staff Group or another group; the admin only through `{admin}`) | `TestEventPMPrivacy` (every default and custom copy, the Details record, the log line and every request of a blocked `/ban` in 8 languages name neither group; an anonymous admin's `{admin}` is never the tapper), `settingsPrivacyPhase22Screens`, `settingsPrivacyPhase22Run`; `.../previews` (the private copy preview names the presser as member and admin, never a group but its own) |
| PMSG-07 (with the log option on, one line in the log channel for an undelivered copy) | `.../not_started` (exactly one `#PM_COPY` line naming the member's mention and ID); `TestEventPMLogLine`, `TestSettingsDriftMessagesPMOptions`, `TestEventPMLogsCarryIDsOnly` |

## Phase gates

| Gate | Result | Duration |
|------|--------|----------|
| `go test -tags testtools -race -count=1 -run '^(TestPMCopiesEndToEnd\|TestMessagesEndToEnd\|TestLegacyModerationRequests)$' -v ./fuku/modules` | PASS, 11 subtests of `TestPMCopiesEndToEnd` (7.4 s), no SKIP | 20 s |
| `make check-translations` | all present | 1 s |
| `make check-docs` | no drift | 1 s |
| `make test-shard-check` | lists equal | 23 s |
| `CGO_ENABLED=0 go build ./...` | ok | 2 s |
| golangci-lint v2.13.1 `run --new-from-rev=origin/main ./...` | 0 issues | 23 s |
| the same with `--build-tags=notesttools` | 0 issues | 23 s |
| `run --new=false ./fuku/...`, filtered to the 40 Go files Phase 22's own commits touched | nothing in a Phase 22 file but four `dupl` findings between `bans.go` and `mute.go`, which the same run at the phase base `d0fae0e` reports identically (pre-existing, out of scope) | 2 s (cached) |
| `go test -tags testtools -race -count=1 ./fuku/db/... ./fuku/i18n/... ./fuku/utils/...` | ok, 67 packages | 84 s |
| `go test -tags testtools -race -count=1 -timeout 45m ./fuku/modules` (the whole package, once, in the background beside the other gates) | ok, exit 0 | 2262 s (37.7 min) |
| `git diff --exit-code 692c978 -- go.mod go.sum main.go` (the commit before this plan) | no change | - |
| `ls migrations \| grep -c '^2026112012'` | 1 | - |
| `grep -rn 'first production caller arrives in plan 21-' fuku` | none | - |
| `grep -c 'TestPMCopiesEndToEnd' AGENTS.md` | 4 | - |

## Mutation checks (all reverted, tree clean afterwards)

| Mutation | Failed subtest |
|----------|----------------|
| the Details deep link without the record's user check (`rec.UserID != user.Id`) | `not started` |
| `runEventPM` never posts the log line | `not started` |
| `runEventPM` without the refused-custom fallback | `refused custom` |
| `offerReplyPM` ignores the pm row's switch | `two replicas` |

## Deviations from Plan

### Auto-fixed Issues

None: no defect found, no production file changed.

### Other deviations

**1. [Behavior] `/ban spam <b>` in place of `/ban spam & co`**
- The default group reply is compared with `goldenModerationBanReason`, which was recorded with that reason; the copy's
  `{reason}` (`spam &lt;b&gt;`) proves the escaping on it.

**2. [Behavior] the default text in Spanish, with a Tokyo end**
- The plan says "in the group's language with the end in the group's zone"; the group is set to `es` and `Asia/Tokyo` so neither
  could be the bot's defaults by chance.

**3. [Behavior] `group silent` with two members**
- One started (the copy is sent with no group message), one blocked with Details on (no edit, no record): the plan's "no edit"
  only means something for a copy that was not delivered.

**4. [Acceptance] the golden has two commits**
- `git log -- fuku/modules/moderation_legacy_request_test.go` lists 31e7e18 (its own) and 2dcb0c7, the milestone renumbering,
  which changed one comment ("before Phase 20" to "before Phase 21") and no golden literal. The file is unchanged since, and this
  plan did not touch it.

**5. [Acceptance] the `9940d97` diff**
- Compared against the commit before this plan (`692c978`) as the brief asked: `go.mod`, `go.sum` and `main.go` are unchanged.

**6. [Commits] two commits**
- The planned subject `test(22-09): prove private copies end to end` carries the test and its AGENTS.md Testing sentence; the final
  AGENTS.md pass is its own `docs(22-09)` commit.

No `TestLegacy*Requests` golden and no `TestSettingsDeletionPhase16ScreensUnchanged` digest was touched; no earlier test was edited.

## AGENTS.md final pass

Every Phase 22 sentence (Data: `chat_event_messages`, `chat_event_message_options`, the three `fuku:pmcopy:*` keys; Handlers: the
`pmcopy_` deep link; the Messages page's private copy, previews, switch and options; Permissions; Startup and shutdown's no-drain
sentence; the PM copies trap; Testing) was read against `event_message_pm.go`, `event_message_pm_details.go`,
`event_message_pm_log.go`, `event_message.go`, `event_message_apply.go` and `settings_messages.go`: the job's order, the
classifier, the sizes and row shapes, the option table and struct fields, the migrations and every one of the 523 backticked
identifiers of those bullets exist and match. Nothing contradicted the code; the PM copies trap, the deep link bullet and the
Permissions sentence now name `TestPMCopiesEndToEnd`, and the Phase 22 test families bullet describes it and its toolkit.

## Known Stubs

None.

## Threat Flags

None: T-22-35 (`not started`: the real `/start` for the person and for someone else), T-22-36 (`staff group action`,
`TestPMCopyCallers`), T-22-37 (`rights`), T-22-38 (`two replicas`) and T-21-SC (no change to `go.mod` or `go.sum`) are mitigated as
planned.

## Not verified

- No PostgreSQL run (`make test-postgres-integrity`): nothing here rests on a constraint; the phase's PostgreSQL guarantees
  (`TestEventMessagesUpsert`, `TestEventMessageOptionsMigrationDefaults`) are already in its list and were not run here.
- `make test` with coverage and its CI shards, and `make lint` itself, were not run; the plain whole-package run and the
  CI-equivalent golangci-lint runs above were. The whole `fuku/modules` run took 2262 s while it shared the machine with the other
  gates, under the 45 min timeout given but close to `make test`'s 40 min per package; a CI shard runs a quarter of it.
- `skipLocal` for `event_messages:` is not proved by `two replicas` (both replicas share one process and its local layer); what it
  proves is that every write drops the shared value.
- The live Telegram behaviours (a 403 for a person who never started the bot, the Details URL opening the bot) rest on the fake.

## Self-Check: PASSED

- FOUND: fuku/modules/pm_copies_e2e_test.go
- FOUND commits: 037123b, 46e0181, both ancestors of HEAD
