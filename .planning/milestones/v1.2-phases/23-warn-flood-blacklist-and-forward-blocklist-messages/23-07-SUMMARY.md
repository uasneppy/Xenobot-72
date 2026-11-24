---
phase: 23-warn-flood-blacklist-and-forward-blocklist-messages
plan: 07
subsystem: event-messages
status: complete
tags: [event-messages, warns, antiflood, blacklists, forward-blocklist, e2e]
requires:
  - phase: 23-01
    provides: TestLegacyPunishmentNoticeRequests and the goldenNotice* constants
  - phase: 23-03
    provides: the Automatic family on the Messages page (codes wn wl fl bl fb)
  - phase: 23-04
    provides: the five Automatic send sites through postEvent
  - phase: 23-05
    provides: the private copy specs and the cap for rule-raised warns
  - phase: 23-06
    provides: the forward blocklist's Details and log sentinel check (TestEventMessageAutoPrivacy)
provides:
  - TestAutoMessagesEndToEnd and its toolkit autoMessagesE2E
  - the Phase 23 AGENTS.md sentences checked against the code
affects: [24]
tech-stack:
  added: []
  patterns:
    - "an end-to-end test of the Automatic family is Phase 22's pmCopiesE2E with LoadWarns and LoadBlacklists added and a 900 s bot:punish_auto timer beside the 600 s manual one, so a queue row proves its kind"
key-files:
  created:
    - fuku/modules/auto_messages_e2e_test.go
  modified:
    - AGENTS.md
requirements-completed: [EVT-02, EVT-03]
decisions:
  - "Antiflood, the warn limit and the blacklist word are set through their repositories (the toolkit's existing way); every message part and switch is set only on the page with real presses and typed private messages."
  - "The forward blocklist subtest runs on newFwdWatchEnv with the group text and the private copy stored through the repository (that environment is not a settings environment); it checks a delivered private copy, since the Details record and log line were already checked against sentinels by TestEventMessageAutoPrivacy (D-31)."
  - "autoMember gives the member the goldens' shape (first name, last name Lee, no username) so the nothing-stored requests can be compared with goldenNoticeWarnReason, goldenNoticeBlacklistMute and goldenNoticeFloodMute through messagesE2EGolden."
metrics:
  duration: ~75 min (of which 38 min 27 s the full package run)
  completed: 2026-11-21
actuals:
  tokens: 6800
  tasks: 2
  commits: 2
plan_head_before: f7180abc415667dd02edb0d57c96fc9697643524
plan_head_after: 5a7687bb1339b1bf032c2e3956992467ce6f1821
---

# Phase 23 Plan 07: The automatic messages end to end Summary

`TestAutoMessagesEndToEnd` drives the whole phase through the real dispatcher with only Telegram faked: the owner sets the
warn, warn limit, flood and blacklist messages and their private copies on the private Messages page with real presses and
typed messages, an administrator's `/warn` and members' messages run through the Warns, Antiflood and Blacklists modules, and a
real forward reaches the forward blocklist watcher of a linked group. All nine subtests pass on the first run; no production
defect was found.

## Tasks

| Task | Commit | Files |
|------|--------|-------|
| 1. The phase end to end through the real dispatcher | 18e3ae4 | fuku/modules/auto_messages_e2e_test.go (new), AGENTS.md (the test in the Phase 23 test families) |
| 2. The phase-wide gates, the full package run and the final AGENTS.md pass | 5a7687b | AGENTS.md |

## Requirements and success criteria

| Item | Proved by |
|------|-----------|
| EVT-02 (warn and warn limit have their own messages; the warn keeps Remove warn) | `TestAutoMessagesEndToEnd/owner_sets_a_warn_message` (custom warn with `{count}`/`{limit}`, custom row then Remove warn, the tap removes the warn), `/blacklist_warn_and_warn_limit` (custom warn from the blacklist, then ban and the custom warn limit with the ban word and 2/2); also `TestEventMessageWarn`, `TestEventMessageWarnLimit` |
| EVT-03 (flood, blacklist and forward blocklist actions have their own messages) | `TestAutoMessagesEndToEnd/flood`, `/blacklist_mute`, `/forward_blocklist`; also `TestEventMessageFlood`, `TestEventMessageBlacklist`, `TestEventMessageForwardBlock` |
| SC1: the Automatic family, each event with group message, private copy and Silent | `TestAutoMessagesEndToEnd` (opens wn, wl, fl, bl and the private copy screens of wn and bl through drawn buttons; Silent on wn), `TestSettingsMessagesAutoFamily`, `TestSettingsMessagesAutoPM` |
| SC2: custom warn on `/warn` and the blacklist's warn, Remove warn kept, `{count}` `{limit}`; custom warn limit with `{action}` | `TestAutoMessagesEndToEnd/owner_sets_a_warn_message`, `/blacklist_warn_and_warn_limit` |
| SC3: flood, blacklist, forward blocklist custom messages with `{action}` and `{reason}`; the forward blocklist names neither source nor Staff Group | `TestAutoMessagesEndToEnd/flood`, `/blacklist_mute`, `/forward_blocklist` (group text and delivered private copy, every placeholder, `fwdForbiddenInText`); `TestEventMessageAutoPrivacy` (Details record, `#PM_COPY` line, logs) |
| SC4: nothing stored is today's text byte for byte; refused falls back; same timer; punishment first | `TestAutoMessagesEndToEnd/nothing_stored` (three goldens, each queued under its kind), `/refused` (mute, refused photo, golden default once), the punishment-before-send checks of every subtest; `TestLegacyPunishmentNoticeRequests` |
| SC5: private copies follow Phase 22 incl. the per-group cap; every punishment happens | `TestAutoMessagesEndToEnd/pm_copies_and_the_cap` (four offenders muted with a notice each, exactly two copies at cap 2, `/warn` copy uncapped), `/silent` (Silent group message, copy still sent); `TestEventPMAutoEvents`, `TestEventPMAutoCap` |

## Gates

| Gate | Result | Wall time |
|------|--------|-----------|
| Task 1 verify (`TestAutoMessagesEndToEnd`, `TestMessagesEndToEnd`, `TestPMCopiesEndToEnd`, both legacy goldens, -race -v) | PASS, 9/9 subtests, no skip | 26 s |
| make check-translations | PASS | 0.3 s |
| make check-docs | PASS (no drift) | 1.3 s |
| make test-shard-check | PASS | 20.8 s |
| CGO_ENABLED=0 go build ./... | PASS | 2.0 s |
| golangci-lint v2.13.1 `--new-from-rev=origin/main ./...` | 0 issues | 37 s |
| same with `--build-tags=notesttools` | 0 issues | 19 s |
| `--new=false ./fuku/modules/... ./fuku/db/eventmsg/...`, Phase 23 files | baseline only: `dupl` blacklists.go/approvals.go; `gocyclo` `checkFlood` 31 (unchanged), `addBlacklist` 23 (unchanged), `blacklistWatcher` 30 (down from 35) | 2 s |
| `go test -tags testtools -race -count=1 ./fuku/db/... ./fuku/i18n/... ./fuku/utils/...` | PASS (67 packages) | 70 s |
| Full `go test -tags testtools -race -count=1 -timeout 45m ./fuku/modules` | PASS, 2095 top-level tests, no fail, no skip | 2307 s (38 min 27 s) |
| go.mod, go.sum, main.go, migrations vs 1a382c3 | unchanged (D-23) | |
| `first production caller arrives in plan 23-` notes left | 0 | |
| Goldens | `git diff --exit-code 9d4554d -- punishment_notice_legacy_request_test.go` exits 0; `moderation_legacy_request_test.go` unchanged since 1a382c3 | |

## Full package time

38 min 27 s against the phase-start baseline of about 37.7 min (+45 s), so over the 38-minute mark. The Phase 23 tests add
about 25 s in all; the slowest are `TestAutoMessagesEndToEnd` 5.7 s, `TestSettingsMessagesAutoFamily` 2.6 s,
`TestLegacyPunishmentNoticeRequests` 1.6 s, `TestEventMessageForwardBlock` 1.6 s, `TestEventMessageAutoLogsCarryIDsOnly` 1.5 s,
`TestEventPMAutoCap` 1.5 s, `TestEventMessageWarn` 1.4 s. The rest of the delta is run-to-run noise. The package's slowest tests
are older (`TestMemberReqLookupLogsCarryIDsOnly` 80 s, `TestSettingsEditCheckRules` 75 s, `TestSettingsStaffPrivacy` 38 s) and
are the better follow-up target than any Phase 23 test.

## Mutation check

Turning the custom warn's `KeepRowsLast` off in `warns.go` (temporarily, not committed) failed `owner sets a warn message` on
the keyboard order; setting `KeepRows` nil did not, because nil falls back to the default's rows, which are the same Remove
warn row (correct behaviour, not a gap).

## Deviations from Plan

- No production file changed: the end-to-end test found no defect, so `event_message.go`, `event_message_pm.go`,
  `settings_messages.go`, `warns.go`, `antiflood.go`, `blacklists.go` and `forwardblock_notice.go` are untouched.
- The AGENTS.md sentence naming the new test went into the test's own commit (repo rule: AGENTS.md changes travel with the
  change they describe); the Task 2 commit holds the AGENTS.md corrections only, as `docs(23-07)` rather than the plan's
  single `test(23-07)` commit.
- Acceptance check "each golden has exactly one commit": `punishment_notice_legacy_request_test.go` has one;
  `moderation_legacy_request_test.go` has two, the second being the earlier comment-only phase renumber (2dcb0c7, "Phase 20"
  to "Phase 21" in a comment). Its requests are untouched and it is unchanged since 1a382c3.
- The flood is configured with the burst deletion on in the `flood` subtest (so every message of the burst is checked as
  deleted) and off in `nothing stored` and `refused` (the golden deletes the triggering message only).

## AGENTS.md pass

Every Phase 23 sentence was read against the code (event list and order in `events.go`, `eventSite` fields, `postEvent`,
`postEventGroup`, `offerReplyPM`, the builders in `event_message_auto.go`, the five sites, the specs, `o.automatic = spec.Automatic
|| o.ByRule`, the cap of 10 and the pool of 16, the `postEvent` allow-list of `TestPMCopyCallers`, the family table, codes,
row shapes and the auto prompt's placeholders). Two corrections: the four default builders now have production callers (the
sentence still said "until" plans 23-03 to 23-05), and the event list sentence now says Phase 23 added its five events with no
migration and Phases 24 and 25 add theirs the same way.

## Not verified

- `make test-postgres-integrity` (left to the phase verifier; this phase adds no PostgreSQL guarantee, D-36).
- The full `make test` coverage threshold (only the modules package and the db/i18n/utils packages were run).

## Known Stubs

None.

## Threat Flags

None. T-23-19: `forward blocklist` subtest (and `TestEventMessageAutoPrivacy`). T-23-20: `pm copies and the cap`. T-23-21:
`owner sets a warn message` taps the kept Remove warn button. T-23-SC: go.mod and go.sum unchanged since 1a382c3.

## Self-Check: PASSED

- FOUND: fuku/modules/auto_messages_e2e_test.go
- FOUND: 18e3ae4, 5a7687b (ancestors of HEAD)
