---
phase: 23-warn-flood-blacklist-and-forward-blocklist-messages
verified: 2026-11-21T23:40:00Z
status: human_needed
score: 10/10 must-haves verified
covered_files: [".planning/phases/23-warn-flood-blacklist-and-forward-blocklist-messages/23-01-PLAN.md", ".planning/phases/23-warn-flood-blacklist-and-forward-blocklist-messages/23-01-SUMMARY.md", ".planning/phases/23-warn-flood-blacklist-and-forward-blocklist-messages/23-02-PLAN.md", ".planning/phases/23-warn-flood-blacklist-and-forward-blocklist-messages/23-02-SUMMARY.md", ".planning/phases/23-warn-flood-blacklist-and-forward-blocklist-messages/23-03-PLAN.md", ".planning/phases/23-warn-flood-blacklist-and-forward-blocklist-messages/23-03-SUMMARY.md", ".planning/phases/23-warn-flood-blacklist-and-forward-blocklist-messages/23-04-PLAN.md", ".planning/phases/23-warn-flood-blacklist-and-forward-blocklist-messages/23-04-SUMMARY.md", ".planning/phases/23-warn-flood-blacklist-and-forward-blocklist-messages/23-05-PLAN.md", ".planning/phases/23-warn-flood-blacklist-and-forward-blocklist-messages/23-05-SUMMARY.md", ".planning/phases/23-warn-flood-blacklist-and-forward-blocklist-messages/23-06-PLAN.md", ".planning/phases/23-warn-flood-blacklist-and-forward-blocklist-messages/23-06-SUMMARY.md", ".planning/phases/23-warn-flood-blacklist-and-forward-blocklist-messages/23-07-PLAN.md", ".planning/phases/23-warn-flood-blacklist-and-forward-blocklist-messages/23-07-SUMMARY.md", "AGENTS.md", "fuku/db/eventmsg/events.go", "fuku/i18n/eventmsg_locale_test.go", "fuku/i18n/settings_locale_test.go", "fuku/modules/antiflood.go", "fuku/modules/auto_messages_e2e_test.go", "fuku/modules/blacklists.go", "fuku/modules/event_message.go", "fuku/modules/event_message_auto.go", "fuku/modules/event_message_auto_safety_test.go", "fuku/modules/event_message_auto_sites_test.go", "fuku/modules/event_message_auto_test.go", "fuku/modules/event_message_pm.go", "fuku/modules/event_message_pm_auto_test.go", "fuku/modules/forwardblock_notice.go", "fuku/modules/pm_copy_callers_test.go", "fuku/modules/punishment_notice_legacy_request_test.go", "fuku/modules/settings_messages.go", "fuku/modules/settings_messages_auto_test.go", "fuku/modules/staff_forwardblock_watch.go", "fuku/modules/warns.go"]
covered_digest: "v4:sha256:50ec684de154cacc52ce52b87fd5772de24cf3a9e54439bf90abff3058c49942"
behavior_unverified: 0
overrides_applied: 0
advisory:
  - finding: "Test-time risk: the full fuku/modules run took 38 min 27 s (2307 s, 23-07-SUMMARY.md Gates table) against make test's 40-minute per-package limit"
    category: other
    reason: "1 min 33 s of headroom on the machine that ran it; AGENTS.md already says CI runners measured the same package from 19 to 30 min per shard. Phase 23 added about 25 s. CI shards fuku/modules four ways, but a local `make test` runs it as one package and can now time out on a slower or loaded machine. The slowest tests are older (TestMemberReqLookupLogsCarryIDsOnly 80 s, TestSettingsEditCheckRules 75 s, TestSettingsStaffPrivacy 38 s); trimming one of them before Phase 24 adds tests is the cheapest fix."
    evidence_status: "recorded wall time in 23-07-SUMMARY.md; not re-run here (per instruction)"
  - finding: "The refusal fallback (MSG-04) is exercised directly for the warn (TestEventMessageWarn 'a refused custom warn sends the default once') and the flood (TestEventMessageFlood 'refused', TestAutoMessagesEndToEnd/refused), not for the warn limit, blacklist or forward blocklist sites"
    category: other
    reason: "All five sites post through the same postEventGroup (fuku/modules/event_message.go:346-375), whose refusal branch calls the site's own SendDefault once; the three untested sites differ only in their SendDefault closure (b.SendMessage, the call the golden pins). Low risk; a one-case subtest per site would close it."
    evidence_status: "code read, reproducible"
  - finding: "Bookkeeping not yet closed: ROADMAP.md Phase 23 checkbox and its seven plan lines are still `[ ]`, REQUIREMENTS.md keeps EVT-02 and EVT-03 Pending, and STATE.md still says `Phase: 22 of 30` and `Next: plan Phase 23`"
    category: other
    reason: ".planning/ROADMAP.md:61 and :178-184, .planning/REQUIREMENTS.md:29-30 and :101-102, .planning/STATE.md:23-26. Normal phase-close work for the orchestrator, not a code gap."
    evidence_status: "file read, reproducible"
human_verification:
  - test: "Live Telegram: in a test group set a custom Warning message with {count}/{limit}/{reason} and one URL button, then /warn a member, and let the blacklist's warn action warn another"
    expected: "Each warn posts the custom text as a reply to the triggering message with the custom button first and the bot's Remove warn button last; tapping Remove warn removes the warn; the message is deleted on the Bot messages timer (manual kind for /warn, automatic kind for the blacklist warn)"
    why_human: "The fake client proves the request bytes; how a real Telegram client draws the keyboard order and whether the callback still works on the live message is client behaviour"
  - test: "Live Telegram: set the warn limit to 2 with action Ban and a custom Warning limit message using {action}; warn a member twice"
    expected: "The member is banned and their warns reset, then the custom limit message names the ban word in the group's language"
    why_human: "Real ban latency and the order of the ban and the message on a live chat are not provable with a fake"
  - test: "Live Telegram: custom Flood, Blacklist and Blocked source messages, each with {action} and {reason}; trigger a flood (mute), a blacklisted word (mute) and a forward from a blocked channel (mute) in a group linked to a Staff Group"
    expected: "Each punishment happens first; each custom message appears (the flood one in the member's topic with the Unmute button first); the Blocked source message shows neither the channel's title nor ID nor the Staff Group's title; the private copies (if turned on) arrive and name neither either"
    why_human: "Live message rendering, topic placement and the Unmute callback on a real forum supergroup"
  - test: "Live Telegram: store a custom flood message whose photo file ID belongs to another bot"
    expected: "The bot mutes, Telegram refuses the photo, and today's default flood notice is posted once"
    why_human: "The refusal is scripted in the tests; the real Bot API error code for a foreign file ID is not provable offline"
  - test: "Owner reviews the provisional decisions P1 to P10 of 23-CONTEXT.md (`Provisional Decisions for Owner Confirmation`)"
    expected: "The owner confirms or asks to change: P1 /swarn posts the custom warn and inert modes keep today's text; P2 {admin} reads 'an admin' for automatic actions; P3 the blacklist {reason} is the untranslated stored reason naming the word; P4 the warn limit {reason} is the last warn's reason and the flood gets a new translated reason; P5 the flood admin row above custom buttons, Remove warn below them, a custom warn drops the default Rules button; P6 custom messages go exactly where the default goes; P7 family name, icons, order, codes wn wl fl bl fb; P8 rule-raised warns count against the private copy cap, /warn never; P9 default private texts name count, limit and action; P10 the preview sample values"
    why_human: "These are planner defaults taken without asking the owner (the run was authorized autonomous); each is reversible as a table row, a constant or one site's field"
  - test: "Run `make test-postgres-integrity` against a PostgreSQL database (DATABASE_URL set, FUKU_TEST_DATABASE=true)"
    expected: "Passes unchanged; Phase 23 adds no migration, index or CHECK (D-36), but the event key strings `warnlimit` and `fwdblock` are written to chat_event_messages.event for the first time, and the Phase 21/22 PostgreSQL tests (TestEventMessagesUpsert, the migration defaults) should still pass"
    why_human: "No PostgreSQL in this environment; 23-07 did not run it either. The SQLite halves pass (fuku/db/eventmsg ok)"
audit_acknowledged:
  milestone: v1.2
  at: 2026-11-24
  status: human_needed
---

# Phase 23: Warn, Flood, Blacklist and Forward Blocklist Messages Verification Report

**Phase Goal:** As the owner, I can customize what the bot says when it warns someone, when a member reaches the warn limit, and when flood, a blacklisted word or a blocked forward gets punished, in the group and by PM.
**Verified:** 2026-11-21T23:40:00Z
**Status:** human_needed
**Re-verification:** No, initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC1: The Messages page has an Automatic family with warn, warn limit, flood action, blacklist action and forward blocklist action; each opens the editor with a group message, a PM copy and a Silent switch | ✓ VERIFIED | `settingsMsgAutoEvents` and family `a` 🤖 in `settings_messages.go:151-192`; the page's `init` registers both editor definitions for every event of every family; `registerEventPM` for all five in `event_message_pm.go:82-86`, so every event screen carries the private copy row. `TestSettingsMessagesAutoFamily`, `TestSettingsMessagesAutoPM`, `TestAutoMessagesEndToEnd` (opens wn, wl, fl, bl and private copy screens through drawn buttons; Silent on wn) pass |
| 2 | SC2a / EVT-02: A custom warn message is posted on `/warn` and on the blacklist's warn action, keeps its Remove warn button, fills `{count}` and `{limit}` | ✓ VERIFIED | `warnThisUser` below-limit branch posts through `postEvent(warnSite, warnEventReply)` (`warns.go:342-343`); `warnEventReply` sets `KeepRows` = the Remove warn row with `KeepRowsLast: true` (`warns.go:404-405`), `composeEventMessage` appends kept rows after custom and `{rules}` buttons (`event_message.go:258-262`); `Count`, `Limit` filled (`warns.go:400`). The blacklist's warn action and `forwardBlockWarn` call `warnThisUser(..., warnSourceAuto)`. `TestEventMessageWarn`, `TestAutoMessagesEndToEnd/owner_sets_a_warn_message` (taps the kept Remove warn and the warn goes), `/blacklist_warn_and_warn_limit` pass; 23-07 mutation of `KeepRowsLast` failed the e2e test |
| 3 | SC2b / EVT-02: When a member reaches the limit, the custom warn-limit message is posted with `{action}` naming what was done | ✓ VERIFIED | `case res.LimitApplied: postEvent(warnSite, warnLimitEventReply)` (`warns.go:337-338`); `LimitApplied` set only after the limit action and the reset (`warns.go:223-226`); `{action}` = `eventActionWord(tr, res.Mode)` (`warns.go:422`). Inert modes keep `warnPostLimit` (`warns.go:339-341`, 430). `TestEventMessageWarnLimit`, e2e `blacklist_warn_and_warn_limit` (ban word, 2/2) pass |
| 4 | SC3 / EVT-03: A flood, blacklist or forward blocklist punishment posts its custom message with `{action}` and `{reason}` | ✓ VERIFIED | `floodNotice` (`antiflood.go:413-445`): `{reason}` = `floodEventReason`, `{count}` the burst, `{limit}` the flood limit, `{action}` the word, Unmute/Unban row kept first; `blacklistNotice` (`blacklists.go:780-784`) and `forwardBlockNotice` (`forwardblock_notice.go:24-31`) through `listNoticeReply`/`listNoticeSite` (`event_message_auto.go:176-198`). Each is called after the punishment returned without error. `TestEventMessageFlood`, `TestEventMessageBlacklist`, `TestEventMessageForwardBlock`, e2e `flood`, `blacklist_mute`, `forward_blocklist` pass |
| 5 | SC3: The forward blocklist message names neither the blocked source nor the Staff Group | ✓ VERIFIED | `forwardBlockNotice` takes only the bot, the update, the member ID and the action word; target = the sender (`eventSenderTarget`), reason = fixed `staff_fwdblock_reason`, chat = the linked group; no source or Staff Group value is in scope. Its warn path uses the same fixed reason (`staff_forwardblock_watch.go:350-356`). `TestEventMessageAutoPrivacy` (real watcher, sentinel source and Staff Group titles: group notice, Details edit and record, `#PM_COPY` line, logs) and e2e `forward_blocklist` (`fwdForbiddenInText` on the group text and the delivered private copy, every placeholder filled) pass |
| 6 | SC4a: A group with nothing stored gets today's text for all five, byte for byte | ✓ VERIFIED | `TestLegacyPunishmentNoticeRequests` (22 subtests) passes; the file is unchanged since 9d4554d (`git diff 9d4554d HEAD` empty, one commit in its log); `TestAutoEventDefaults` compares every builder with the goldens; e2e `nothing_stored` checks the warn, blacklist and flood goldens through the real dispatcher. `TestLegacyModerationRequests` (Phase 21) still passes after the engine went generic |
| 7 | SC4b: A refused custom message falls back to the default; the message is deleted on the same timer as the default; the punishment is never delayed | ✓ VERIFIED | `postEventGroup` refusal branch sends `site.SendDefault` once (`event_message.go:360-365`); custom and default both queued under `site.Kind` (`event_message.go:367`, `:382`); kinds unchanged (warn by source, flood/blacklist/fwdblock `bot:punish_auto`). Every site calls `postEvent` after the punishment. Tested: refused warn, refused flood (e2e `refused`: mute, refused photo, golden default once, queued automatic), e2e `wantPunishedFirst`, `wantQueuedAs` per kind. See advisory on the three sites without their own refusal case |
| 8 | SC5: PM copies follow Phase 22's rules, including the per-group cap; a flood of offenders produces at most the capped number of copies a minute and every punishment still happens | ✓ VERIFIED | Specs: warn and warnlimit manual, flood/blacklist/fwdblock automatic (`event_message_pm.go:82-86`); `o.automatic = spec.Automatic \|\| o.ByRule` (`event_message_pm.go:301`), `ByRule: site.Automatic` (`event_message.go:418`). `TestEventPMAutoCap`, `TestEventPMAutoEvents` and e2e `pm_copies_and_the_cap` (four offenders each muted once with a notice each, exactly two copies at cap 2, an uncapped `/warn` copy) and `silent` (Silent group message, copy still sent) pass |
| 9 | Plan truth: no staff, lockdown or raid file reaches the engine; no new table, migration, key, handler group, drain or dependency | ✓ VERIFIED | `TestPMCopyCallers` (allow-list for `postEvent`: event_message.go, warns.go, antiflood.go, blacklists.go, forwardblock_notice.go) passes; `TestMemberDeletionExcludedMessages` and `TestCommandRepliesStayInsideTheHandler` pass; `git diff --stat 1a382c3 HEAD -- go.mod go.sum main.go migrations` is empty |
| 10 | Plan truth: AGENTS.md's Phase 23 sentences match the code | ✓ VERIFIED | Read against code: `eventSite` field list, `postEvent` / `postEventGroup` / `sendDefaultPost` / `offerReplyPM` behaviour, `KeepRows` / `KeepRowsLast`, `warnSite` actor and kind rules, `LimitApplied` gating and `warnPostLimit` for inert modes, `floodNotice` (no reply, topic, `SendMessageWithErrorHandling`, admin row first, inert actions through `sendDefaultPost`), `listNoticeSite` / `listNoticeReply`, the specs and `spec.Automatic \|\| o.ByRule`, the `postEvent` allow-list, the event order in `events.go`. No mismatch found |

**Score:** 10/10 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `fuku/modules/punishment_notice_legacy_request_test.go` | goldens of all 22 sends, committed alone first | ✓ VERIFIED | one commit (9d4554d), unchanged since; passes |
| `fuku/modules/event_message.go` | generic `eventSite`, `postEvent`, `KeepRows` | ✓ VERIFIED | substantive, used by five sites |
| `fuku/modules/event_message_auto.go` | four default builders, action word, flood reason, list site and reply | ✓ VERIFIED | each called by its site and by `settingsMsgAutoEvents` |
| `fuku/db/eventmsg/events.go` | five event keys in `Events` | ✓ VERIFIED | `warn warnlimit flood blacklist fwdblock` appended, no migration |
| `fuku/modules/warns.go` | warn and warn limit through the engine | ✓ VERIFIED | `warnSite`, `warnEventReply`, `warnLimitEventReply` |
| `fuku/modules/antiflood.go` | flood notice through the engine | ✓ VERIFIED | `floodNotice`, called from `checkFlood` |
| `fuku/modules/blacklists.go` | blacklist notice through the engine | ✓ VERIFIED | `blacklistNotice` after the switch |
| `fuku/modules/forwardblock_notice.go` | forward blocklist notice through the engine, outside staff files | ✓ VERIFIED | takes an action word; called by `forwardBlockMute/Kick/Ban` |
| `fuku/modules/event_message_pm.go` | five specs, `ByRule` | ✓ VERIFIED | |
| `fuku/modules/settings_messages.go` | Automatic family, samples | ✓ VERIFIED | |
| `fuku/modules/auto_messages_e2e_test.go` | end-to-end proof | ✓ VERIFIED | 9 subtests, real dispatcher, real page presses |
| `locales/*.yml` | new keys in 8 languages | ✓ VERIFIED | `make check-translations` passes; `TestEventMsgLocaleKeys`, `TestSettingsLabelLengths`, `TestLocaleStyle`, `TestLocalePlaceholdersMatchOtherLanguages` pass |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `warnThisUser` | `postEvent` | `warnSite` + `warnEventReply` / `warnLimitEventReply` | ✓ WIRED | warns.go:338, :343 |
| blacklist warn action / `forwardBlockWarn` | `warnThisUser` | `warnSourceAuto` | ✓ WIRED | Automatic site, "an admin", auto kind, `ByRule` |
| `checkFlood` | `floodNotice` → `postEvent` | after the punishment switch | ✓ WIRED | antiflood.go:398, :444 |
| `blacklistWatcher` | `blacklistNotice` → `postEvent` | after the punishment, error checked first | ✓ WIRED | blacklists.go:758-770 |
| `forwardBlockMute/Kick/Ban` | `forwardBlockNotice` → `postEvent` | action word | ✓ WIRED | staff_forwardblock_watch.go:364, :372, :380 |
| `postEvent` | `offerReplyPM` → `offerEventPM` | `ByRule: site.Automatic` | ✓ WIRED | event_message.go:327, :418; event_message_pm.go:301 |
| Messages page | stored rows read by the send path | `eventmsg.Get` (skipLocal key) | ✓ WIRED | e2e presses on the page change the next send |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| warn custom message | `res.Count`, `res.Limit`, `reason` | `applyWarn` (stored warns, warn settings) | Yes | ✓ FLOWING |
| warn limit custom message | `res.Mode` → `{action}` | warn settings row | Yes | ✓ FLOWING |
| flood custom message | `floodCrc.messageCount`, `flood.Limit`, `flood.Action` | antiflood counters and settings | Yes | ✓ FLOWING |
| blacklist custom message | `fmt.Sprintf(reason, i)`, `matched.Action` | the matched blacklist row | Yes | ✓ FLOWING |
| fwdblock custom message | action word, fixed reason | forward blocklist rules | Yes (no source data by design) | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Phase 23 tests plus neighbours (29 names incl. both goldens, `TestPMCopyCallers`, `TestMemberDeletionExcludedMessages`, `TestMessagesEndToEnd`, `TestPMCopiesEndToEnd`) | `go test -tags testtools -race -count=1 -run '^(...)$' ./fuku/modules` | `ok 67.7s` | ✓ PASS |
| Locale tests | `go test ... -run '^(TestEventMsgLocaleKeys\|TestSettingsLabelLengths\|TestLocaleStyle\|TestLocalePlaceholdersMatchOtherLanguages\|TestSettingsLocale.*)$' ./fuku/i18n` | `ok 9.1s` | ✓ PASS |
| eventmsg repository | `go test -tags testtools -race -count=1 ./fuku/db/eventmsg` | `ok 1.1s` | ✓ PASS |
| Translations | `make check-translations` | all present | ✓ PASS |
| Lint on new code | `golangci-lint run --new-from-rev=origin/main ./...` | `0 issues.` | ✓ PASS |
| Golden unchanged | `git diff 9d4554d HEAD -- fuku/modules/punishment_notice_legacy_request_test.go` | empty; one commit | ✓ PASS |
| Full fuku/modules | not re-run (instruction); 23-07 recorded PASS, 2095 tests, 2307 s | n/a | ? SKIP |

### Probe Execution

Not applicable: the phase declares no probe scripts.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| EVT-02 | 23-02, 23-04, 23-07 | Warn and "warn limit reached" have their own messages; the warn message keeps its Remove warn button | ✓ SATISFIED | Truths 2, 3 |
| EVT-03 | 23-04, 23-07 | The antiflood action, the blacklist action and the forward blocklist action each have their own message | ✓ SATISFIED | Truths 4, 5 |

No orphaned requirements: REQUIREMENTS.md maps only EVT-02 and EVT-03 to Phase 23.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (production files of the phase) | | no TBD/FIXME/XXX/TODO/HACK, no leftover `nolint:unused` | ℹ️ Info | none |
| `fuku/modules/antiflood.go` | `checkFlood` | gocyclo 31 (baseline, unchanged) | ℹ️ Info | allowed by D-39 |
| `fuku/modules/blacklists.go` | `blacklistWatcher` | gocyclo 30 (down from 35) | ℹ️ Info | improved |

### Human Verification Required

1. **Custom warn on live Telegram.** Set a custom Warning with `{count}`, `{limit}`, `{reason}` and a URL button; `/warn` a member and let the blacklist's warn action warn another. Expected: the custom text replies to the triggering message, custom button first, Remove warn last; tapping Remove warn works; deleted on the Bot messages timer of its kind. Why human: client rendering and the live callback.
2. **Custom warn limit on live Telegram.** Limit 2, action Ban, custom Warning limit with `{action}`; warn twice. Expected: ban and reset first, then the custom message with the ban word. Why human: live ordering.
3. **Flood, blacklist and blocked source on live Telegram**, in a forum supergroup linked to a Staff Group, with private copies on. Expected: punishment first, custom message (flood in the topic with Unmute first), nothing naming the blocked channel or the Staff Group, private copies the same. Why human: live rendering and topics.
4. **Refusal on live Telegram.** A custom flood photo with a foreign file ID. Expected: mute, refused photo, today's default once. Why human: the real error code.
5. **Owner decisions P1 to P10** (23-CONTEXT.md, Provisional Decisions for Owner Confirmation). Confirm or change each.
6. **PostgreSQL run.** `make test-postgres-integrity` with a database. Expected: pass unchanged (no new guarantee, D-36).

### Gaps Summary

No gaps. Every roadmap success criterion and both requirements are implemented, wired to their send sites, and proven by passing tests that exercise the behaviour, not just the presence of symbols. The golden of today's replies was committed alone first and is unchanged, and Phase 21's golden still passes on the generic engine. The forward blocklist path has no value for the source or the Staff Group in scope, and a sentinel test confirms it. Three advisories remain: the full `fuku/modules` run is 38 min 27 s against `make test`'s 40-minute per-package limit, the refusal fallback has no case of its own at three of the five sites (they share one tested code path), and the roadmap, requirements and state bookkeeping are still open. The status is human_needed because of the live Telegram checks, the owner's ten provisional decisions and the PostgreSQL run.

---

_Verified: 2026-11-21T23:40:00Z_
_Verifier: Claude (gsd-verifier)_
