---
phase: 23-warn-flood-blacklist-and-forward-blocklist-messages
plan: 02
subsystem: event-messages
status: complete
tags: [event-messages, warns, engine, i18n, golden]
requires:
  - 23-01 (TestLegacyPunishmentNoticeRequests and its toolkit)
provides:
  - eventSite, postEvent, postEventGroup, the site-taking offerReplyPM and sendDefaultPost, (c *moderationCtx) site()
  - eventReply.KeepRows and KeepRowsLast; eventValues.Action, Count, Limit, Window; eventDefaultInput.Count, Limit, Action, Reasons, Rows
  - the events warn, warnlimit, flood, blacklist, fwdblock (eventmsg.EventWarn ... EventForwardBlock)
  - warnDefaultReply, warnLimitDefaultReply, floodDefaultReply, blacklistNoticeDefaultReply, eventActionWord, floodEventReason
  - locale keys eventmsg_action_warn, _mute, _kick, _ban, eventmsg_reason_flood (all 8 languages)
  - TestEventMessageWarn, TestEventActionWords, TestAutoEventDefaults
affects: [23-03, 23-04, 23-05, 23-06, 23-07]
tech-stack:
  added: []
  patterns:
    - "a send site describes itself as an eventSite and posts through postEvent; its SendDefault is exactly its call before custom messages"
    - "a default builder is proven by sending its text and options through SendMessage to chat 0 on an in-memory recorder and comparing the request with the golden byte for byte"
key-files:
  created:
    - fuku/modules/event_message_auto.go
    - fuku/modules/event_message_auto_test.go
  modified:
    - fuku/modules/event_message.go
    - fuku/modules/warns.go
    - fuku/db/eventmsg/events.go
    - fuku/db/eventmsg/repository_test.go
    - fuku/modules/pm_copy_callers_test.go
    - fuku/i18n/eventmsg_locale_test.go
    - locales/{en,es,fr,hi,id,pt,ru,uk}.yml
    - AGENTS.md
decisions:
  - "TestAutoEventDefaults sends each builder's output through a real gotgbot SendMessage on newModuleBotClient (in memory, no database) and compares the whole request with the golden, instead of comparing only the text and the reply_markup field: stronger (every option byte) and no hand-written serialisation."
  - "warnThisUser was split into warnSite, warnEventReply and warnPostLimit to keep its complexity low (13 by count; golangci-lint's gocyclo at 20 reports nothing). warnPostLimit is today's limit send, unchanged."
  - "The warn's custom message keeps the Remove warn row alone (KeepRows) after the custom and {rules} buttons; the default's Rules button is not kept (D-15, P5)."
  - "Ukrainian action words follow the existing warn-limit and blacklist texts and the glossary: отримує попередження, отримує мут, вилучено, забанено; the flood reason is забагато повідомлень поспіль."
  - "warnLimitDefaultReply renders an inert mode as the reasons alone and floodDefaultReply an inert action with an empty word, so they reproduce the inert goldens too (18 golden cases, the plan asked for at least 13)."
metrics:
  duration: ~75 min
  completed: 2026-11-21
actuals:
  tokens: 11200
  tasks: 3
  commits: 3
plan_head_before: c7d18bce11d364616fe7e22df2f51fc5e8549e7f
plan_head_after: 43fc03338f941c4375f524f0decc7a63cfbe6d83
---

# Phase 23 Plan 02: Generic event engine, the warn message and the Automatic family's builders Summary

The event message engine is now generic (`eventSite` plus `postEvent`), the moderation reply step is a thin wrapper
over it with byte-identical requests, `/warn` and the blacklist's and forward blocklist's warns post a group's custom
warn message with the bot's Remove warn row last, and the five automatic event keys, the four translated action words,
the flood reason and the four default builders exist, each builder proven byte for byte against the 23-01 goldens.

## Tasks

| Task | Name | Commit |
|------|------|--------|
| 1 | the warn message end to end through a generic engine | 7ecc5fd |
| 2 | the action words, the flood reason and the engine guard | 317943c |
| 3 | the remaining default builders, proven against the goldens | 43fc033 |

## What later plans use

| Kind | Name | Notes |
|------|------|-------|
| Engine | `eventSite{Bot, Chat, Tr, Actor, ReplyTo, ThreadID, Kind, Automatic, SendDefault}` | `ReplyTo` and `ThreadID` 0 mean none; `Actor` empty reads "an admin" |
| Engine | `postEvent(site, r)`, `postEventGroup(site, r, msgs)`, `sendDefaultPost(site, r)`, `offerReplyPM(site, r, msgs, post)` | `sendDefaultPost` = `site.SendDefault` then `scheduleBotMessage(sent, site.Kind)` |
| Engine | `(c *moderationCtx) site()`, `replyEvent` = `postEvent(c.site(), r)` | |
| Fields | `eventReply.KeepRows`, `.KeepRowsLast`; `eventValues.Action`, `.Count`, `.Limit`, `.Window`; `eventDefaultInput.Count`, `.Limit`, `.Action`, `.Reasons`, `.Rows` | `Count`/`Limit` 0 render empty |
| Builders | `warnDefaultReply`, `warnLimitDefaultReply`, `floodDefaultReply`, `blacklistNoticeDefaultReply` | `fuku/modules/event_message_auto.go` |
| Words | `eventActionWord(tr, action)`, `floodEventReason(tr)` | inert actions have no word |
| Warn site | `warnSite(b, ctx, tr, source)`, `warnEventReply(b, chat, replyTo, tr, u, reason, res)`, `warnPostLimit(b, chat, msg, tr, u, res, source)` | `fuku/modules/warns.go` |

## Earlier tests whose expected sets changed (no assertion weakened)

- `fuku/db/eventmsg/repository_test.go`, `TestEventMessages/the event and destination tables`: the expected `Events`
  list grows from the seven manual events to twelve (the five automatic keys appended), and the refused words replace
  `warn` (now valid) with `warning` and `Warn`; `""`, `Ban`, `ban `, `dban`, `sban` are still refused.
- Same file, `TestEventMessages/an unknown event or destination writes nothing`: the unknown event used is `warning`
  instead of `warn` (now a valid event); the assertion (ErrUnknownEvent, no row) is unchanged.
- `fuku/modules/pm_copy_callers_test.go`, `TestPMCopyCallers`: `postEvent` is tracked with an allow-list
  (`event_message.go`, `warns.go`, each with its reason) and added to the forbidden names of staff, lockdown and raid
  files; the existing allow-lists and checks are unchanged. A probe file `staff_zz_probe.go` naming `postEvent` made it
  fail with both messages (guard and allow-list), then was removed.
- `TestEventPMSpecs` was not touched: `warn` still has no private copy spec in this plan.

## Verification (all with `-tags testtools -race -count=1`)

- Task 1 verify (`TestEventMessageWarn|TestLegacyPunishmentNoticeRequests|TestLegacyModerationRequests|TestWarnThisUserSource|TestBotAutoPunishmentNotices|TestEventMessageBanEndToEnd|TestEventMessageCompose|TestEventMessageFallback|TestEventMessageSites|TestEventMessageAfterAnonymousProof|TestEventPMEndToEnd|TestPMCopyCallers`): ok, 22 s (61 s with build); `./fuku/db/eventmsg` `TestEventMessages`: ok, 1 s.
- `TestEventMessageWarn -v`: 4 subtests PASS.
- Task 2 verify: modules ok (6 s), i18n `TestEventMsgLocaleKeys|TestLocaleStyle|TestLocalePlaceholdersMatchOtherLanguages` ok (6 s), `make check-translations` all present; 54 s in all.
- Task 3 verify: `TestAutoEventDefaults -v` 18 subtests PASS, with the other three tests ok, 6 s (44 s with build).
- Families: `^(TestEventMessage|TestEventPM|TestLegacy|TestWarn|TestBotAuto|TestBotManual|TestPMCopy|TestAutoEvent|TestEventActionWords|TestSettingsMessages|TestSettingsDriftMessages|TestSettingsAuthorityMessages|TestMessagesEndToEnd|TestApplyWarn|TestRenderEventText|TestPMCopyCallers)`: ok, 99 s, 91 top-level PASS, no FAIL or SKIP; `./fuku/db/eventmsg` and the whole `./fuku/i18n` package: ok (1 s, 50 s).
- Goldens unchanged: `git diff --exit-code 9d4554d -- fuku/modules/punishment_notice_legacy_request_test.go` and `git diff --exit-code 7ecc5fd~1 -- fuku/modules/moderation_legacy_request_test.go` exit 0.
- `git diff --exit-code 1a382c3 -- go.mod go.sum main.go migrations fuku/modules/antiflood.go fuku/modules/blacklists.go fuku/modules/forwardblock_notice.go fuku/modules/staff_forwardblock_watch.go` exits 0.
- `make check-translations` (0.3 s), `make check-docs` no drift (1.3 s), `make test-shard-check` rc 0 (26 s), `CGO_ENABLED=0 go build ./...` ok (7 s).
- golangci-lint v2.13.1 (`CGO_ENABLED=0`): `run --new-from-rev=origin/main ./...` 0 issues (38 s); with `--build-tags=notesttools` 0 issues (20 s); `run --new=false ./fuku/modules/... ./fuku/db/eventmsg/... ./fuku/i18n/...` reports no finding in any touched file (the baseline `checkFlood`, `addBlacklist`, `blacklistWatcher` and `blacklists.go` dupl findings are untouched; the 4 `unused` findings are pre-existing in other files). `warnThisUser` complexity: 13 by count, under the 20 threshold.
- gofmt clean on every touched Go file. The full `fuku/modules` package was not run (plan 23-07 runs it).

## Deviations from Plan

**1. [Choice] TestAutoEventDefaults compares the whole request**
- The plan compared the builder's text and keyboard with the golden's `text` and `reply_markup`. The test instead sends the
  builder's text and options through `SendMessage` to chat 0 on an in-memory recording client (`autoDefaultRequest`) and
  compares the whole request JSON with the golden (`<chat>` replaced by `0`), with the warn's and the warn limit's reply
  parameters added as their site adds them. It is stricter (every option, not two fields) and touches no database.

**2. [Choice] 18 golden cases instead of the listed 13**
- The inert warn limit (`tban`) and the inert flood (`warn`) goldens are covered too, so plans 23-04/05 can reuse the
  builders for the inert paths if they choose.

**3. [Choice] TestEventMessageWarn's reason**
- The custom subtest types `/warn spam & <b>` (not `spam & co`), so the escaping of both `&` and `<` is asserted
  (T-23-04) and the fallback subtest compares with `goldenNoticeWarnReason` exactly.

No Rule 1-4 deviations. No architectural change.

## Notes for plan 23-03

- The builders `warnLimitDefaultReply`, `floodDefaultReply` and `blacklistNoticeDefaultReply` carry
  `//nolint:unused // first production caller arrives in plan 23-03`; remove them when the Messages page's event table
  references them. `floodEventReason` carries the 23-04 note.
- A builder's `in.Rows` is the site's bot rows; for a preview (D-27) pass no rows: previews never carry a bot row. The
  flood's default splices the untranslated English word, the custom `{action}` the translated `eventActionWord`.
- `warnLimitDefaultReply` takes `Action` as the mode word (`kick`, `mute`, `ban`) and `Reasons` as the stored reasons;
  `blacklistNoticeDefaultReply` takes `Action` and the already filled `Reason` (spliced unescaped, as today).
- The warn's `ReservedButtons` is 1 (the Remove warn row, `KeepRows` with `KeepRowsLast`); the flood's is 1 (its admin row
  is kept first through `eventBotRows(r.Opts)` when the site leaves `KeepRows` nil).
- `eventmsg.Events` now has 12 entries; the Messages page's Automatic family uses codes `wn wl fl bl fb` (D-09).
- For plans 23-04/05: `postEvent` callers must be added to `TestPMCopyCallers`'s `postEvent` allow-list with a reason
  (`antiflood.go`, `blacklists.go`, `forwardblock_notice.go`); the limit branch of `warnThisUser` is `warnPostLimit`, the
  one call to switch; keep `warnsModule.warnUser`, `dWarnUser`, `sWarnUser`, `checkFlood`, `blacklistWatcher` and the four
  `forwardBlock*` signatures, which the golden calls. `eventSite.Automatic` is set by `warnSite` but not read yet (D-28's
  `ByRule` arrives with the private copy specs).

## Known Stubs

None. The three builders with dated nolint directives are complete and tested; only their production callers come later.

## Threat Flags

None beyond the plan's register: T-23-02 (goldens unchanged), T-23-03 (`KeepRows` set by the site, asserted last),
T-23-04 (`&` and `<` escaped in the custom warn), T-23-05 (`TestPMCopyCallers` forbids `postEvent` in staff, lockdown
and raid files, proven by a probe file), T-23-SC (`go.mod`, `go.sum` unchanged).

## Self-Check: PASSED

- Files: `fuku/modules/event_message_auto.go` and `fuku/modules/event_message_auto_test.go` exist.
- Commits: 7ecc5fd, 317943c and 43fc033 are ancestors of HEAD.
