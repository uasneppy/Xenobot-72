---
phase: 22-pm-copies
verified: 2026-11-21T19:05:00Z
status: human_needed
score: 12/12 must-haves verified
covered_files: [".planning/phases/22-pm-copies/22-01-PLAN.md", ".planning/phases/22-pm-copies/22-01-SUMMARY.md", ".planning/phases/22-pm-copies/22-02-PLAN.md", ".planning/phases/22-pm-copies/22-02-SUMMARY.md", ".planning/phases/22-pm-copies/22-03-PLAN.md", ".planning/phases/22-pm-copies/22-03-SUMMARY.md", ".planning/phases/22-pm-copies/22-04-PLAN.md", ".planning/phases/22-pm-copies/22-04-SUMMARY.md", ".planning/phases/22-pm-copies/22-05-PLAN.md", ".planning/phases/22-pm-copies/22-05-SUMMARY.md", ".planning/phases/22-pm-copies/22-06-PLAN.md", ".planning/phases/22-pm-copies/22-06-SUMMARY.md", ".planning/phases/22-pm-copies/22-07-PLAN.md", ".planning/phases/22-pm-copies/22-07-SUMMARY.md", ".planning/phases/22-pm-copies/22-08-PLAN.md", ".planning/phases/22-pm-copies/22-08-SUMMARY.md", ".planning/phases/22-pm-copies/22-09-PLAN.md", ".planning/phases/22-pm-copies/22-09-SUMMARY.md", "AGENTS.md", "fuku/db/eventmsg/events.go", "fuku/db/eventmsg/repository.go", "fuku/db/models/event_message.go", "fuku/i18n/eventmsg_locale_test.go", "fuku/modules/event_message.go", "fuku/modules/event_message_apply.go", "fuku/modules/event_message_pm.go", "fuku/modules/event_message_pm_details.go", "fuku/modules/event_message_pm_details_test.go", "fuku/modules/event_message_pm_failures_test.go", "fuku/modules/event_message_pm_log.go", "fuku/modules/event_message_pm_log_test.go", "fuku/modules/event_message_pm_safety_test.go", "fuku/modules/event_message_pm_test.go", "fuku/modules/moderation.go", "fuku/modules/mute.go", "fuku/modules/pm_copies_e2e_test.go", "fuku/modules/pm_copy_callers_test.go", "fuku/modules/settings_messages.go", "fuku/modules/settings_messages_pm_options_test.go", "fuku/modules/settings_messages_pm_preview_test.go", "fuku/modules/settings_messages_pm_test.go", "migrations/20261120120000_add_event_message_options.sql"]
covered_digest: "v4:sha256:e0a5f894e62f3ca3038cc21256db58c14df6d99215d148827ea9365a6c1b61b5"
behavior_unverified: 0
overrides_applied: 0
advisory:
  - finding: "Stray old phase number: the frontmatter of nine plans and five summaries says `phase: 21-pm-copies`, and the CONTEXT footer says `*Phase: 21-pm-copies*`"
    category: other
    reason: "Phases were renumbered on 2026-11-21; PM copies is Phase 22. Files: 22-01..22-09-PLAN.md line 2, 22-01..22-05-SUMMARY.md line 2, 22-CONTEXT.md:339. Docs only, no code effect; tooling that keys on the frontmatter phase would file these under Phase 21."
    evidence_status: "grep, reproducible"
  - finding: "STATE.md is stale: it still says plan 22-06 is paused with a `.continue-here.md` that no longer exists, and its Current focus line names Phase 21 as next to discuss"
    category: other
    reason: ".planning/STATE.md:23 and :28. All nine plans are complete (22-09-SUMMARY.md); the orchestrator should update STATE when it closes the phase."
    evidence_status: "file read, reproducible"
  - finding: "ROADMAP.md Phase 22 still reads `**Plans**: TBD` and its checkbox is open; REQUIREMENTS.md traceability keeps PMSG-01..07 as Pending"
    category: other
    reason: ".planning/ROADMAP.md:60 and :157, .planning/REQUIREMENTS.md:41-47 and :110-116. The Phase 21 entry lists its plans; Phase 22's list was never written in. This is normal bookkeeping for phase close, not a code gap."
    evidence_status: "file read, reproducible"
human_verification:
  - test: "On live Telegram, turn on a ban's private copy, then /ban a member who has never started the bot (and, separately, one who blocked it)"
    expected: "Telegram answers 403 for both, the job classifies it as blocked (Debug log only), the ban and the group reply are unchanged, and with the options on one Details button and one #PM_COPY line appear"
    why_human: "The 403 is simulated by the test client (messagesE2EClient.refuse); the real Bot API's code and wording for a never-started user, a blocked bot and a deleted account cannot be proved by a fake"
  - test: "Tap the Details button under the group message as the punished member who has never started the bot, then as another member"
    expected: "The t.me/<bot>?start=pmcopy_<token> URL opens the bot's private chat; after pressing Start the member sees their copy; the other member sees only 'There is nothing to show here.'"
    why_human: "The Telegram client's handling of a start-payload URL for a user with no chat yet (the Start button and the payload delivery) is client behaviour no test reaches"
  - test: "Owner reviews the default decisions: the private copy is in the group's language (D-12), unban and unmute have copies (D-06), the copy is still sent when the group message is Silent (D-11), the Details record lives 48 h (D-19), and automatic events are capped at 10 copies per group per minute (D-17)"
    expected: "The owner confirms each, or asks for a change (each is a constant or one table row: eventPMSpecs, pmCopyDetailsTTL, pmCopyAutoCapPerMinute, offerReplyPM's Silent rule)"
    why_human: "These are the planner's provisional defaults (22-CONTEXT 'Behaviour an Owner Would Notice'), not owner-settled decisions"
  - test: "Run `make test-postgres-integrity` against a PostgreSQL database"
    expected: "TestEventMessageOptionsMigrationDefaults and the pm subtests of TestEventMessagesUpsert pass (migration runs twice, both options default false, the inline primary key refuses a second row with SQLSTATE 23505, concurrent pm part writers leave silent true)"
    why_human: "No PostgreSQL is available in this environment (DATABASE_URL unset), and 22-09 did not run it either; the SQLite halves pass"
---

# Phase 22: PM Copies Verification Report

**Phase Goal:** As the owner, I can have the bot also tell a punished person in private, and it never delays or changes the punishment.
**Verified:** 2026-11-21T19:05:00Z
**Status:** human_needed
**Re-verification:** No, initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC1 / PMSG-01: each punishment event has a private copy with its own text, media, buttons and on/off switch, off by default | ✓ VERIFIED | `eventmsg.DestPM`, `DefaultSilent("pm") == true`, `Resolve` reads no row as off, `upsert` inserts a new pm row with `silent = DefaultSilent(dest)` and never updates the switch on conflict (`fuku/db/eventmsg/repository.go:242-262`). Seven specs registered in `event_message_pm.go:67-77`. Page: `settingsMsgPMEditDef`, `hp` with `settingsRightChangeInfo` (`settings_messages.go:209-213`). Tests `TestEventMessagesPM`, `TestSettingsMessagesPMScreen/Switch/Typed`, e2e `owner sets a private copy` pass |
| 2 | SC1: with it on, a member who started the bot gets it after the punishment and the group message | ✓ VERIFIED | `replyEvent` runs `replyEventGroup` first, then `offerReplyPM` (`event_message.go:233-244`); the punishment ran earlier in the pipeline's `execute`. E2E subtest checks order and content (placeholders filled and escaped, no Unban row) |
| 3 | SC2 / PMSG-02: a member who never started or blocked the bot changes nothing else | ✓ VERIFIED | `pmCopyClassify` maps 403 to blocked by code only (`event_message_pm.go`, `pmCopyClassify`); the job is detached and its outcome never reaches the handler. `TestEventPMFailures` (403 of either wording: one request, golden group reply), e2e `not started` (banned once, one default group reply) |
| 4 | SC2 / PMSG-03: own fleet-wide pacing, never before or instead of the punishment | ✓ VERIFIED | `pmCopyPacerOptions` keys `fuku:pmcopy:pace:next|block`, 200 ms, 1 retry, 15 s; every job call goes through `pmCopyPaced`; job in a bounded pool (`startBelow(16)`, 60 s, background context, recover). `TestEventPMPacerIndependent` (a block on the pm keys leaves the ban and reply immediate; other pacers' blocks leave the copy alone), `TestEventPMCost` (handler returns before any copy request) pass |
| 5 | SC3 / PMSG-04: with Details on and the copy not delivered, the group message carries a Details button | ✓ VERIFIED | `runEventPM` calls `pmCopyAddDetails` only when not delivered, `Options.PMDetails` and `GroupMsgID != 0`; it stores `fuku:pmcopy:det:<token>` (crypto/rand, 22 chars, SET NX, 48 h) and edits the group message keeping its rows plus one URL row through the pm pacer (`event_message_pm_details.go`). `TestEventPMDetails`, e2e `not started` (edit keeps Unban row, adds Details) |
| 6 | SC3 / PMSG-04: only the punished person gets the content; anyone else none | ✓ VERIFIED | `pmCopyDetailsDeepLink` parses strictly, reads by token, sends only when `rec.UserID == user.Id`, else one neutral reply; deep links run in private chats only (`help.go:326-346`). `TestPMCopyDetailsDeepLink`, e2e: the member sees the stored copy, another member gets only `eventmsg_pm_details_none`; 22-09 mutation removing the user check failed `not started` |
| 7 | SC4 / PMSG-07: with the log option on, an undelivered copy leaves one line in the log channel | ✓ VERIFIED | `pmCopyPostLog` (admin category through `actionlog.Destination`, `#PM_COPY`, member mention and code ID, event name, reason class), called only for a not-delivered outcome with `Options.PMLog` (`event_message_pm_log.go`). `TestEventPMLogLine`, e2e `not started` (exactly one line) |
| 8 | SC5 / PMSG-05: no copy for a lockdown's joiner bans, Ban N recent joiners or a Staff Group action | ✓ VERIFIED | `offerEventPM` is called only from `event_message.go` and `replyEvent` only from `bans.go`/`mute.go` (grep); `TestPMCopyCallers` parses every non-test file and forbids `staff*`, `lockdown*`, `raid*` files from naming the engine. E2E `staff group action` (staff /ban fan-out to a linked group whose ban copy is on sends the target nothing) |
| 9 | SC5 / PMSG-05: a per-group cap bounds copies per minute for automatic events | ✓ VERIFIED | `pmCopyCapAllow` (INCR + EXPIRE 120 in one transaction, `fuku:pmcopy:{<chat>}:cap:<minute>`, limit 10, fail closed) is the job's first step for `automatic` offers. No production event is automatic yet (all seven manual specs are `Automatic: false`, by design D-17), so the cap is proved on a test-registered automatic event in `TestEventPMLimits` (per chat, per minute, fail closed without Redis or on a Redis error, manual uncapped) |
| 10 | SC5 / PMSG-06: a copy never names the Staff Group or another group; the admin only through `{admin}` | ✓ VERIFIED | The copy is built only from the event's own chat (`composeEventPM`, `eventPMDefaultText` with `{group}` = own escaped title); the default text never names the admin; `{admin}` uses `eventAdminName` (anonymous admin -> `eventmsg_admin_anon`). `TestEventPMPrivacy` (8 languages, copy, Details record, log line, every request; anonymous admin never the tapper), `TestSettingsStaffPrivacy` with `settingsPrivacyPhase22Screens`/`Run` pass |
| 11 | The two per-group options (Details, Log) sit on the Messages page, need `can_change_info` | ✓ VERIFIED | `settingsMsgPMOptions` (rows `d`, `l`), `ho` with `settingsRightChangeInfo` and strict `Validate`; one column-targeted upsert each (`SetPMDetails`, `SetPMLog`) dropping `event_messages:<chat>` (in `skipLocal`); migration `20261120120000` idempotent, inline PK. `TestSettingsMessagesPMOptions`, `TestSettingsDriftMessagesPMOptions`, `TestSettingsAuthorityMessagesPMOptions` pass |
| 12 | The group reply is unchanged for a group that turns nothing on | ✓ VERIFIED | `TestLegacyModerationRequests` passes; the golden file's history is 31e7e18 (its own) plus 2dcb0c7 (a comment from the renumbering); `TestEventPMCost` shows /ban with the copy off makes exactly the golden site's requests |

**Score:** 12/12 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `migrations/20261120120000_add_event_message_options.sql` | options table | ✓ VERIFIED | One `CREATE TABLE IF NOT EXISTS`, all columns NOT NULL with zero defaults, inline PK, no BEGIN/COMMIT |
| `fuku/db/eventmsg/events.go`, `repository.go` | pm destination, default, Resolve, options | ✓ VERIFIED | As described in truths 1 and 11; failed read of either table is returned, never a default |
| `fuku/modules/event_message_pm.go` | specs, pacer, pool, classifier, cap, job, composer | ✓ VERIFIED | Substantive (399 lines), wired from `offerReplyPM` |
| `fuku/modules/event_message_pm_details.go` | record, edit, deep link | ✓ VERIFIED | Registered through `RegisterDeepLinkHandler("pmcopy_", ...)` in `init` |
| `fuku/modules/event_message_pm_log.go` | log line | ✓ VERIFIED | Wired from `runEventPM` |
| `fuku/modules/event_message.go` | replyEvent split, offerReplyPM | ✓ VERIFIED | Group step unchanged, offer after it, failed read sends no copy |
| `fuku/modules/event_message_apply.go` | applyEventPM, applyEventPMDetails/Log, dest-aware part writers | ✓ VERIFIED | Through `eventMsgWrites` seam, fresh read-back |
| `fuku/modules/settings_messages.go` | private copy screen, `hp`, `ho`, `d` field | ✓ VERIFIED | `settingsMsgDefByFields` refuses any `d` but `p`, `d=p` on an event without spec, and a press without `c` |
| `fuku/modules/pm_copies_e2e_test.go` | end-to-end proof | ✓ VERIFIED | 11 subtests through the real dispatcher, all pass |
| `fuku/modules/pm_copy_callers_test.go` | PMSG-05 guard | ✓ VERIFIED | go/parser walk, allow-lists with reasons, stale entries fail |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `bans.go` / `mute.go` sites | `replyEvent` | `c.replyEvent(eventReply{...})` | ✓ WIRED | 7 call sites, all seven events; `/unmute` passes `PMNote` (`mute.go:279-284`) |
| `replyEvent` | `offerReplyPM` -> `offerEventPM` | after `replyEventGroup` | ✓ WIRED | `event_message.go:241-242`, `:312-349` |
| `runEventPM` | `pmCopyAddDetails`, `pmCopyPostLog` | not delivered + option | ✓ WIRED | independent of each other |
| group message edit URL | `pmCopyDetailsDeepLink` | `/start pmcopy_<token>` | ✓ WIRED | `help.go` private branch -> `HandleDeepLink` |
| hub toggles `ho` | `applyEventPMDetails` / `applyEventPMLog` | `settingsMsgPMOptions[].Apply` | ✓ WIRED | |
| send path read | options table | `eventmsg.GetFresh` -> `Messages.Options` | ✓ WIRED | one cached key, in `skipLocal` |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| private copy job | `o.Message` | `composeEventPM` from the pm row read by `eventMessagesLookup` (cached `eventmsg.Get`) | Yes | ✓ FLOWING |
| Details view | `rec` | Redis record written by the job | Yes | ✓ FLOWING |
| log line | `o.Options.PMLog`, channel | options table; `actionlog.Destination` | Yes | ✓ FLOWING |
| hub block / event screen line | `Options`, pm row | `eventmsg.GetFresh` via `settingsMsgRead` | Yes | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Phase 22 module tests, golden, guards and page safeguards | `go test -tags testtools -race -count=1 -run '^(TestPMCopiesEndToEnd\|TestEventPM.*\|TestPMCopyDetailsDeepLink\|TestPMCopyCallers\|TestSettingsMessagesPM.*\|TestSettingsDriftMessagesPM.*\|TestSettingsAuthorityMessagesPM.*\|TestLegacyModerationRequests\|TestCommandRepliesStayInsideTheHandler\|TestSettingsMessagesCallbackBytes\|TestSettingsMessagesPageFitsEveryLocale\|TestSettingsMessagesForged\|TestSettingsMessagesReadsCreateNothing\|TestSettingsStaffPrivacy)$' ./fuku/modules` | ok 86 s, 33 top-level tests PASS, 0 SKIP, 0 FAIL | ✓ PASS |
| Storage, locales, migration statements, backup exclusion | `go test ... -run '^(TestEventMessages\|TestEventMessagesUpsert\|TestEventMessagesPM\|TestEventMsgLocaleKeys\|TestLocaleStyle\|TestLocalePlaceholdersMatchOtherLanguages\|TestEventMessageOptionsMigrationStatements\|TestEventMessagesStayOutOfBackup\|TestSettingsLabelLengths)$' ./fuku/db/eventmsg ./fuku/i18n ./fuku/db/migrations ./fuku/db/backup` | ok x4 | ✓ PASS |
| Translations | `make check-translations` | all present | ✓ PASS |
| PostgreSQL integrity list includes the new guarantee | `grep TestEventMessageOptionsMigrationDefaults Makefile` | present in `test-postgres-integrity` | ✓ PASS (run itself: human item) |

### Probe Execution

No probes declared or conventional for this phase (`scripts/*/tests/probe-*.sh` not referenced). SKIPPED.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| PMSG-01 | 22-01, 22-02, 22-04, 22-05, 22-09 | own text, media, buttons and switch, off by default | ✓ SATISFIED | Truth 1, 2 |
| PMSG-02 | 22-03, 22-09 | best effort, nothing else changes | ✓ SATISFIED | Truth 3 |
| PMSG-03 | 22-02, 22-03, 22-08 | own fleet-wide pacing, never before or instead of the punishment | ✓ SATISFIED | Truth 4 |
| PMSG-04 | 22-06 | Details button, only the punished person gets the content | ✓ SATISFIED (live client behaviour: human) | Truths 5, 6 |
| PMSG-05 | 22-03 | never for raid-scale removals or staff actions; automatic cap | ✓ SATISFIED | Truths 8, 9 |
| PMSG-06 | 22-02, 22-08 | never names the Staff Group or another group; admin only through `{admin}` | ✓ SATISFIED | Truth 10 |
| PMSG-07 | 22-07 | one log line for an undelivered copy | ✓ SATISFIED | Truth 7 |

No orphaned requirement: REQUIREMENTS.md maps exactly PMSG-01..07 to Phase 22.

### AGENTS.md Claims

Every backticked identifier in the Phase 22 lines of AGENTS.md (Data: `chat_event_messages`, `chat_event_message_options`, the three `fuku:pmcopy:*` families; Handlers: the `pmcopy_` deep link; the Messages page's private copy, previews, switch and options; Permissions; Startup and shutdown's no-drain note; the PM copies trap; Testing) was grepped against `fuku/`, `locales/en.yml`, `migrations` and `Makefile`: none missing. The numeric claims checked against code match (pacer 200 ms / 1 retry / 15 s, 16 in flight, 60 s job, cap 10 per minute with EXPIRE 120, Details TTL 48 h, token 22 characters, `ho` 41 bytes, `hp` 42). The "Phase 21" references in AGENTS.md all mean the message engine, correctly; no stray "Phase 21" names the PM copies there.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (phase Go files) | - | TBD/FIXME/XXX/TODO, `nolint:unused` placeholders | none found | - |
| `.planning/phases/22-pm-copies/22-0{1..9}-PLAN.md`, `22-0{1..5}-SUMMARY.md` | 2 | `phase: 21-pm-copies` (old number) | ℹ️ Info | doc defect, see advisory |
| `.planning/phases/22-pm-copies/22-CONTEXT.md` | 339 | `*Phase: 21-pm-copies*` | ℹ️ Info | doc defect |
| `.planning/STATE.md` | 23, 28 | stale position (22-06 paused, missing `.continue-here.md`) | ℹ️ Info | bookkeeping |
| `.planning/ROADMAP.md` | 60, 157 | Phase 22 unchecked, `Plans: TBD` | ℹ️ Info | bookkeeping |

### Human Verification Required

### 1. Live 403 for a member who never started the bot (and one who blocked it)

**Test:** Turn on a ban's private copy and both options, then `/ban` a member who has never started the bot, and another who blocked it.
**Expected:** The ban and the group reply are as usual; no private message; one Details button under the group reply; one `#PM_COPY` line in the log channel.
**Why human:** The 403 is simulated by the test client; the real Bot API answer for these cases cannot be proved by a fake.

### 2. Details deep link opens the bot

**Test:** As the punished member (never started the bot), tap Details in the group; then as another member.
**Expected:** The bot's private chat opens; after Start the member sees their copy; the other member sees only the neutral line.
**Why human:** Telegram client behaviour of a `?start=` URL for a user with no chat yet.

### 3. Owner's default decisions

**Test:** Confirm or change: copy in the group's language (D-12); unban and unmute have copies (D-06); the copy is sent even when the group message is Silent (D-11); Details record 48 h (D-19); automatic cap 10 per group per minute (D-17).
**Expected:** Each confirmed, or a one-line change requested.
**Why human:** Planner's provisional defaults, not owner-settled.

### 4. PostgreSQL integrity run

**Test:** `make test-postgres-integrity` with `DATABASE_URL` and `FUKU_TEST_DATABASE=true`.
**Expected:** `TestEventMessageOptionsMigrationDefaults` and the pm subtests of `TestEventMessagesUpsert` pass.
**Why human:** No PostgreSQL here; not run by 22-09 either.

### Gaps Summary

No code gap. Every must-have truth from the five roadmap success criteria and PMSG-01..07 is present, substantive, wired and exercised by a passing behavioural test, most of them through the real dispatcher in `TestPMCopiesEndToEnd`. The only automatic-event behaviour (the per-group cap) has no production consumer yet by design (D-17) and is proved on a test-registered automatic event. What remains is outside the code: live Telegram checks, the owner's sign-off on five provisional defaults, a PostgreSQL run, and doc bookkeeping (stale `21-pm-copies` frontmatter from the renumbering, STATE.md and ROADMAP.md not yet advanced past plan 22-06).

---

_Verified: 2026-11-21T19:05:00Z_
_Verifier: Claude (gsd-verifier)_
