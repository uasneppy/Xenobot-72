---
phase: 25-lockdown-messages
verified: 2026-11-22T06:25:00Z
status: human_needed
score: 12/12 must-haves verified
covered_files: [".planning/phases/25-lockdown-messages/25-01-PLAN.md", ".planning/phases/25-lockdown-messages/25-01-SUMMARY.md", ".planning/phases/25-lockdown-messages/25-02-PLAN.md", ".planning/phases/25-lockdown-messages/25-02-SUMMARY.md", ".planning/phases/25-lockdown-messages/25-03-PLAN.md", ".planning/phases/25-lockdown-messages/25-03-SUMMARY.md", ".planning/phases/25-lockdown-messages/25-04-PLAN.md", ".planning/phases/25-lockdown-messages/25-04-SUMMARY.md", ".planning/phases/25-lockdown-messages/25-05-PLAN.md", ".planning/phases/25-lockdown-messages/25-05-SUMMARY.md", ".planning/phases/25-lockdown-messages/25-06-PLAN.md", ".planning/phases/25-lockdown-messages/25-06-SUMMARY.md", ".planning/phases/25-lockdown-messages/25-07-PLAN.md", ".planning/phases/25-lockdown-messages/25-07-SUMMARY.md", ".planning/phases/25-lockdown-messages/25-08-PLAN.md", ".planning/phases/25-lockdown-messages/25-08-SUMMARY.md", "AGENTS.md", "fuku/db/eventmsg/events.go", "fuku/modules/event_engine_callers_test.go", "fuku/modules/event_message.go", "fuku/modules/event_message_lockdown.go", "fuku/modules/event_message_lockdown_auto_test.go", "fuku/modules/event_message_lockdown_lift_test.go", "fuku/modules/event_message_lockdown_safety_test.go", "fuku/modules/event_message_lockdown_test.go", "fuku/modules/lockdown.go", "fuku/modules/lockdown_alert_callbacks.go", "fuku/modules/lockdown_message_legacy_request_test.go", "fuku/modules/lockdown_message_surfaces_test.go", "fuku/modules/lockdown_messages_e2e_test.go", "fuku/modules/pm_copy_callers_test.go", "fuku/modules/raid_trigger.go", "fuku/modules/settings_lockdown.go", "fuku/modules/settings_messages.go", "fuku/modules/settings_messages_lockdown_test.go", "locales/en.yml", "locales/uk.yml"]
covered_digest: "v4:sha256:89ae446a8c1e78767a534d64ef58f969c6c0f028f919b6c9e444c8a13a5a0bb7"
behavior_unverified: 0
overrides_applied: 0
advisory:
  - finding: "The engine's generic Warn line says 'the default was sent' after a group-menu lift whose read failed or whose custom message was refused, although that site's default is empty and nothing was sent"
    category: other
    reason: "postLockdownLiftEvent(..., withDefault=false) (fuku/modules/event_message_lockdown.go:411-438, called from settings_lockdown.go:370) passes a SendDefault that returns (nil, nil). postEventSent logs '[EventMessage] the messages of chat %d could not be read for %s, the default was sent' (fuku/modules/event_message.go:392) and postEventGroup logs '... was refused, the default was sent' (event_message.go:467). The line carries IDs and the event only, the behaviour is correct (nothing is posted beside the redraw, TestEventMessageLockdownLift 'group menu quiet', TestLockdownMessageLogsCarryIDsOnly 'group menu' subtests), and the comment of postLockdownLiftEvent (event_message_lockdown.go:402-404) and AGENTS.md (the lift paragraph of the Event messages bullet) both state it. Accepted follow-up from 25-07: a wording fix needs the site to tell the engine it has no default."
    evidence_status: "code read, reproducible; behaviour proven correct by tests"
  - finding: "AGENTS.md's TestPMCopyCallers sentence lists the postEvent callers without aispam_notice.go, which the test's allow-list has since Phase 24"
    category: other
    reason: "AGENTS.md:4037-4043 names event_message.go, warns.go, antiflood.go, blacklists.go, forwardblock_notice.go, event_message_auto.go and (Phase 25) event_message_lockdown.go; fuku/modules/pm_copy_callers_test.go also allows aispam_notice.go (postAISpamNotice). Pre-existing Phase 24 omission, not introduced here; the Phase 25 addition to the sentence is accurate."
    evidence_status: "file read"
  - finding: "Bookkeeping not yet closed: ROADMAP.md Phase 25 checkbox and its eight plan lines are still `[ ]`, REQUIREMENTS.md keeps EVT-06, EVT-07, EVT-08 and EVT-10 Pending"
    category: other
    reason: ".planning/ROADMAP.md:63 and the Phase 25 plan list, .planning/REQUIREMENTS.md:33-37 and :105-109. Normal phase-close work for the orchestrator (this verifier must not edit them), not a code gap."
    evidence_status: "file read"
human_verification:
  - test: "Live Telegram, manual start: set a custom 'By an admin' lockdown message with {admin}, {chatname}, {reason} and a URL button; remove the bot's delete right; run `/lockdown raid & co` in a forum topic"
    expected: "The group is locked first, then the custom text appears as a reply to the command in that topic, with the button, followed on the next line by 'Approved users are muted too' and the no-delete note; the Staff Group alert and the log channel copy are unchanged and carry none of the custom text; the message is never deleted automatically"
    why_human: "The fake proves the request bytes and the order; how a real client renders a reply in a topic with a media caption and keyboard is client behaviour"
  - test: "Live Telegram, automatic start: set custom Join surge, Flood and AI burst messages with {count}, {window}, {percent} and {reason}; trigger a real join surge in a test supergroup (and, if practical, a flood)"
    expected: "The lockdown starts first, then one new message (no reply, no topic) with the trigger's numbers, the trigger sentence as {reason} and the technical notes after it; the alert in the Staff Group is unchanged"
    why_human: "Real join and message bursts, Redis counters and the async raidPostAsync post are faked or scripted in tests"
  - test: "Live Telegram, lift from every door: with a custom Lifted message stored, lift once by `/unlockdown`, once by the alert's Lift button in the Staff Group, once from a private settings menu and once from a settings menu opened in the group; then remove the custom message and lift from a group menu again"
    expected: "Each lift restores the permissions first; /unlockdown answers with the custom text as a reply, the three button lifts post it as a new message in the locked group (never in the Staff Group), naming the lifter; the manual-change and unbanning lines follow when they apply; with nothing stored the group-menu lift posts nothing beside the redrawn menu"
    why_human: "Live callbacks and the real lift ordering against Telegram"
  - test: "Live Telegram, refusal: store a custom Lifted (or By an admin) message whose photo file ID belongs to another bot, and run the command"
    expected: "The action happens, Telegram refuses the photo, and today's default text is posted once"
    why_human: "The refusal is scripted in tests; the real Bot API error code for a foreign file ID is not provable offline"
  - test: "Live Telegram, Silent: switch Silent on for By an admin and Lifted, then /lockdown and /unlockdown"
    expected: "The group is locked and lifted, the alert and the tally behave as before, and neither command answers in the group"
    why_human: "Confirms the owner accepts a silent command reply in a live group (P10)"
  - test: "Owner reviews the provisional decisions P1 to P15 of 25-CONTEXT.md ('Provisional Decisions for Owner Confirmation')"
    expected: "The owner confirms or asks to change each: P1 keys lockdown/lockjoin/lockflood/lockai/locklift, codes ld lj lf la lu, family 'Lockdown' 🚨 last, icons 🔒 👥 💬 🧠 🔓; P2 only a /lockdown that started is an event (refusals and 'already active' unchanged); P3 {percent} joins every event's placeholders (empty except the flood trigger); P4 {window} a bare number of seconds, {count} the trigger's count; P5 {reason} = the /lockdown reason or trigger sentence, empty for a lift; P6 person placeholders name the recorded starter/lifter, or the bot for an automatic start and an anonymous admin (the default text still names the tapper); P7 notes after one line break, lift lines after a custom lift text; P8 400-unit reserve ('Up to 3696', 624 with media); P9 one media item allowed; P10 every lockdown event can be silenced, so /lockdown and /unlockdown can answer nothing; P11 a group-menu lift posts a stored custom message; P12 lockdown messages never auto-deleted; P13 preview samples; P14 the settings intro reworded in 8 languages; P15 a button lift's {chatname} from the stored chat_title"
    why_human: "These are planner defaults taken without asking the owner (the run was authorized autonomous); each is reversible"
  - test: "Run `make test-postgres-integrity` against a PostgreSQL database (DATABASE_URL set, FUKU_TEST_DATABASE=true)"
    expected: "Passes unchanged. Phase 25 adds no migration, index or CHECK (D-40), but the five new event key strings are written to chat_event_messages.event for the first time"
    why_human: "No PostgreSQL in this environment; the SQLite halves pass (fuku/db/eventmsg ok)"
audit_acknowledged:
  milestone: v1.2
  at: 2026-11-24
  status: human_needed
---

# Phase 25: Lockdown Messages Verification Report

**Phase Goal:** As the owner, I can customize what the bot posts in a group when a lockdown starts (by hand or by one of the three triggers) and when it is lifted. The technical notes, the alerts and every raid-scale path stay as they are.
**Verified:** 2026-11-22T06:25:00Z
**Status:** human_needed
**Re-verification:** No, initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC1 / EVT-06: a manual `/lockdown` posts the custom message and the technical notes that apply follow it | ✓ VERIFIED | `lockdown` (`lockdown.go:385-386`) calls `postLockdownStartEvent` only after `startLockdownRow` answered `lockdownStartStarted` (every refusal returns earlier). The adapter (`event_message_lockdown.go:226-260`) sets `Note: lockdownStartNotes(tr, facts)` from the live `res.BotMember`; the default and the notes share `lockdownStartNoteLines`. `TestEventMessageLockdownStart` (custom, custom notes, silent, refused, anonymous), e2e `owner sets the manual start` (setChatPermissions before the sendPhoto reply, caption ends with the approved-muted note). My mutation removing the `Note` failed `TestEventMessageLockdownStart/{custom,custom_notes,anonymous}` and two e2e subtests |
| 2 | SC2 / EVT-07: an automatic lockdown posts its trigger's message with `{count}`, `{window}` and `{percent}`, and the same notes follow | ✓ VERIFIED | `raidFire` runs `raidPostNotice` inside `raidPostAsync` only for `lockdownStartStarted` (`raid_trigger.go:316`); `raidPostNotice` makes one call, `postLockdownAutoEvent` (`event_message_lockdown.go:286-330`): event by kind (`lockdownAutoEvents`), values `Reason` = trigger sentence, `Count`, `Window` (`strconv.Itoa`), `Percent`; `{percent}` added to `eventTokenNames` and `eventPlaceholderValues` (`event_message.go:172, 199-201`). `TestEventMessageLockdownAuto` (join surge, flood notes, ai burst notes, real surge, silent, refused, drain), e2e `join surge`, `flood`, `ai burst`. 25-08's `Count + 1` mutation failed all three e2e trigger subtests |
| 3 | SC3 / EVT-08: lifting posts the custom lifted message from `/unlockdown`, the alert's Lift button and the settings menu (private and group) | ✓ VERIFIED | `/unlockdown` → `postLockdownLiftReplyEvent` only for `liftApplyDone` (`lockdown.go:412-420`); alert Lift Confirm and private menu → `lockdownAnnounceLift` (wake, then `postLockdownLiftEvent(..., true)`, `lockdown_alert_callbacks.go:373-376`); group menu → `wakeLockdownWorker` + `postLockdownLiftEvent(..., false)` (`settings_lockdown.go:366-371`). Chat built from the lockdown row (`event_message_lockdown.go:414`), never the button's chat. `TestEventMessageLockdownLift` (command, command notes, refusal unchanged, silent, alert button, alert refused, private menu, group menu custom, group menu quiet, tally after silent), e2e `lift doors` (four doors, each posts once, naming its lifter) |
| 4 | D-22: a group-menu lift with nothing stored posts nothing (byte-identical to today); a stored custom message is posted | ✓ VERIFIED | Default send returns `(nil, nil)` when `!withDefault` (`event_message_lockdown.go:417-423`). Golden `TestLegacyLockdownMessageRequests/lift_menu_group`, `TestEventMessageLockdownLift/group_menu_quiet` and `group_menu_custom`. My mutation `false` → `true` at `settings_lockdown.go:370` failed both `lift_menu_group` and `group_menu_quiet` |
| 5 | SC4 / EVT-10 structural: alerts, reminders, raid-scale removals and reports, the tally, the cannot-lock warning, Staff Group surfaces, captcha challenge and the owner log dump never reach the engine | ✓ VERIFIED | `TestEventEngineCallers` (`event_engine_callers_test.go`): exact caller map, `eventLockdownAdapterCallers` has exactly five entries (`lockdown.go: moduleStruct.lockdown`, `lockdown.go: moduleStruct.unlockdown`, `raid_trigger.go: raidPostNotice`, `lockdown_alert_callbacks.go: lockdownAnnounceLift`, `settings_lockdown.go: settingsLockdownLiftYes`), 20 never-reach roots that must exist, one sanctioned stop (`SendCaptcha` → `handleCaptchaTimeout`). `TestEventEngineCallersGuardFails` proves the walk can fail. `TestPMCopyCallers` still forbids engine entries in `staff*`, `lockdown*`, `raid*` files; `event_message_lockdown.go` joined the `postEvent` allow-list with a reason. My mutation calling `postLockdownAutoEvent` from `raidWarnCannotLock` failed `TestEventEngineCallers` on all three checks (caller map, adapter map, reachability) |
| 6 | SC4 / EVT-10 behavioural: those surfaces send exactly what they send today with every lockdown message stored | ✓ VERIFIED | `TestLockdownMessagesLeaveOtherSurfacesAlone` (lockdown flow: alert, reminder, sync, close, Ban run, card report, `#LOCKDOWN_BAN`, tally; cannot lock; staff action; captcha challenge; log post) and e2e `untouched surfaces`; golden `cannot_lock` (Staff Group and log channel copies) unchanged |
| 7 | SC4 / D-04: no lockdown event sends a private copy | ✓ VERIFIED | No `registerEventPM` for the five keys; `offerReplyPM` returns without a spec. `TestEventPMSpecs` (eighteen specs, five lockdown keys spec-less), `TestLockdownEventsSendNoPrivateCopy` (no spec, manual start, automatic start, lift by command, lift by button with a pm row switched on), page refuses `d=p`/`hp` for lockdown codes (`TestSettingsMessagesLockdownFamily`). My mutation registering a spec for `locklift` failed `TestEventPMSpecs` and `TestLockdownEventsSendNoPrivateCopy/{no_spec,lift_by_command,lift_by_button}` (a real private send) |
| 8 | SC5 / MSG-11: a group with nothing stored gets today's lockdown and lift texts byte for byte | ✓ VERIFIED | `TestLegacyLockdownMessageRequests` passes; `git diff --exit-code 36d2070 -- fuku/modules/lockdown_message_legacy_request_test.go` exits 0 and the file has one commit (its own). `TestLockdownEventDefaults` compares the builders with the goldens; e2e `nothing stored` (real locales) equals `goldenLockdownStart`, `goldenLockdownLiftCommand`, `goldenLockdownAutoJoinSurge`. Lift default re-caps an already-capped name (`lockdownCapRunes` is idempotent) |
| 9 | SC5 / MSG-04: a refused custom message falls back to the default once; the lock, snapshot, worker and alert are never delayed | ✓ VERIFIED | `postEventGroup`'s refusal branch sends `site.SendDefault` once; every adapter is called after the action (start after `startLockdownRow`, automatic inside the async post, lift after `liftLockdownRow`), and `lockdownAnnounceLift` now wakes the worker before reading/posting (D-23, code read). Tests: start `refused`, auto `refused`, lift `alert refused`, e2e `refused` (lock before the refused photo, golden default after), e2e `join surge` (tipping joiner still recorded) |
| 10 | D-08 / MSG-09: no lockdown message is queued for deletion, custom or default | ✓ VERIFIED | `lockdownEventSite` sets `Schedule: func(*gotgbot.Message) {}` (`event_message_lockdown.go:210`, in `lockdownEventSite`). Goldens assert no queue row with every bot kind's rule at its shortest delay. My mutation replacing the no-op `Schedule` with `Kind: bot:punish_auto` failed `TestEventMessageLockdownAuto` and `TestEventMessageLockdownLift` subtests |
| 11 | Privacy and logs: no lockdown message names the Staff Group or log channel; logs carry IDs only | ✓ VERIFIED | Button lift's chat from the row; adapter log lines keep today's text (chat ID and Telegram error). `TestLockdownMessagePrivacy`, `TestLockdownMessageLogsCarryIDsOnly` (six subtests, read error and refused photos per site), `TestSettingsStaffPrivacy` (`a private Phase 25 run ...`) pass |
| 12 | Messages page Lockdown family, locale keys in 8 languages, AGENTS.md accurate; no 10 s claim for a custom send | ✓ VERIFIED | Family `l` 🚨 with five events (`settings_messages.go:251-299`), `ReservedText: lockdownNoteReserve` (400), `TestSettingsMessagesLockdownFamily` (hub and list, screens, typed and silent, previews), `TestSettingsAuthorityMessagesLockdown`, `TestSettingsDriftMessagesLockdown`, `TestLockdownNoteFitsReserve`. 14 new keys present once in each of the 8 locale files; `make check-translations` clean. The only "10 s" mentions in the touched production files (`event_message_lockdown.go:282, 406`) describe the default's `lockdownCallTimeout`; every custom-send sentence in code and AGENTS.md says "bounded only by the bot's HTTP client timeout (30 s)". AGENTS.md Phase 25 sentences read against the code (events, adapters and signatures, five adapter callers, roots, wake order, group-menu lift, no queue, no spec, page shapes): accurate; one pre-existing Phase 24 omission recorded as advisory |

**Score:** 12/12 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `fuku/modules/lockdown_message_legacy_request_test.go` | goldens of start, automatic notices, cannot-lock warning, lift from every door, committed alone first | ✓ VERIFIED | one commit (36d2070), unchanged; passes |
| `fuku/db/eventmsg/events.go` | five lockdown event keys appended | ✓ VERIFIED | no migration; `fuku/db/eventmsg` passes |
| `fuku/modules/event_message.go` | `Percent` value and `{percent}` token; `Window`, `Percent`, `Lockdown` default inputs | ✓ VERIFIED | no hook change; Phase 21-24 goldens pass |
| `fuku/modules/event_message_lockdown.go` | four adapters, default builders, notes, reserve | ✓ VERIFIED | every identifier has a production caller; no `nolint` |
| `fuku/modules/lockdown.go`, `raid_trigger.go`, `lockdown_alert_callbacks.go`, `settings_lockdown.go` | sites call the adapters | ✓ VERIFIED | five call sites, matched exactly by the guard |
| `fuku/modules/settings_messages.go` | Lockdown family | ✓ VERIFIED | page tests pass |
| `fuku/modules/event_engine_callers_test.go` | structural EVT-10 guard | ✓ VERIFIED | passes; fails under mutation |
| `fuku/modules/lockdown_message_surfaces_test.go` | behavioural EVT-10 and no-PM guards | ✓ VERIFIED | passes; fails under mutation |
| `fuku/modules/lockdown_messages_e2e_test.go` | end-to-end proof | ✓ VERIFIED | 10 subtests through the real dispatcher |
| `locales/*.yml` | 14 new keys in 8 languages, intro reworded | ✓ VERIFIED | |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `/lockdown` (started) | `postEvent` | `postLockdownStartEvent` | ✓ WIRED | lockdown.go:385 |
| `raidFire` → `raidPostAsync` | `postEvent` | `raidPostNotice` → `postLockdownAutoEvent` | ✓ WIRED | raid_trigger.go:316, :374 |
| `/unlockdown` (liftApplyDone) | `postEvent` | `postLockdownLiftReplyEvent` | ✓ WIRED | lockdown.go:419, tally after |
| alert Lift Confirm / private menu Lift Yes | `postEvent` | `lockdownAnnounceLift` → `postLockdownLiftEvent(true)` | ✓ WIRED | lockdown_alert_callbacks.go:374-375 |
| group menu Lift Yes | `postEvent` | `postLockdownLiftEvent(false)` | ✓ WIRED | settings_lockdown.go:370 |
| Messages page | stored rows read by the send path | `eventmsg.Get` (skipLocal key) | ✓ WIRED | e2e presses change the next send; `two replicas` |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| manual start | starter, reason, bot rights | recorded name, `row.Reason`, `res.BotMember` | Yes | ✓ FLOWING |
| automatic notice | count, window, percent, trigger sentence, bot rights | `raidTrigger`, `res.BotMember` | Yes | ✓ FLOWING |
| lift notice | lifter, manual change, unbanning, title | `lockdownLiftApply`/`lockdownLiftResult`, row's `ChatTitle` | Yes | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Phase 25 families, the four goldens and neighbouring families (234 top-level tests: `TestLockdown.*`, `TestRaid.*`, `TestEventMessage.*`, `TestSettingsMessages.*`, `TestSettingsLockdown.*`, `TestUnlockdown.*`, `TestEventEngineCallers*`, `TestPMCopyCallers`, `TestEventPMSpecs`, `TestSettingsStaffPrivacy`, the five end-to-end tests, `TestCommandRepliesStayInsideTheHandler`, `TestDeletionExcludedMessagesNeverScheduled`) | `go test -tags testtools -race -count=1 -v -run '^(...)$' ./fuku/modules` | `ok 85.8s`, 234 PASS, 0 FAIL, 0 SKIP; 10 `TestLockdownMessagesEndToEnd/` subtests pass | ✓ PASS |
| eventmsg and i18n | `go test -tags testtools -race -count=1 ./fuku/db/eventmsg ./fuku/i18n` | `ok 1.1s`, `ok 52.4s` | ✓ PASS |
| Goldens unchanged | `git diff --exit-code 36d2070 / e40a7be / 9d4554d / 86baf1a -- <golden>` | exit 0, 0, 0, 0 | ✓ PASS |
| Translations, docs, shards, build | `make check-translations`, `make check-docs`, `make test-shard-check`, `CGO_ENABLED=0 go build ./...` | all present; no drift; exit 0; ok | ✓ PASS |
| Lint on new code | `CGO_ENABLED=0 golangci-lint run --new-from-rev=origin/main ./...` | `0 issues.` | ✓ PASS |
| Unchanged files | `git diff --stat d8c98c6 HEAD -- go.mod go.sum main.go migrations` | empty | ✓ PASS |
| Mutation: group-menu lift with default on | `settings_lockdown.go:370` `false` → `true` | FAIL `lift_menu_group`, `group_menu_quiet` | ✓ caught |
| Mutation: no notes after a custom start | drop `Note` in `postLockdownStartEvent` | FAIL 3 start subtests, 2 e2e subtests | ✓ caught |
| Mutation: lockdown messages queued | no-op `Schedule` → `Kind: bot:punish_auto` | FAIL auto and lift subtests | ✓ caught |
| Mutation: cannot-lock warning reaches engine | call `postLockdownAutoEvent` in `raidWarnCannotLock` | FAIL `TestEventEngineCallers` (3 errors) | ✓ caught |
| Mutation: lockdown event with a PM spec | `registerEventPM(EventLockLift, ...)` | FAIL `TestEventPMSpecs`, `TestLockdownEventsSendNoPrivateCopy` (behavioural) | ✓ caught |
| Full fuku/modules | `go test -tags testtools -race -count=1 -timeout 45m ./fuku/modules` | `ok 353.9s`, exit 0, no FAIL or SKIP line | ✓ PASS |

### Probe Execution

Not applicable: the phase declares no probe scripts.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| EVT-06 | 25-01, 25-02, 25-03, 25-07, 25-08 | Manual lockdown message with the technical notes appended | ✓ SATISFIED | Truth 1 |
| EVT-07 | 25-01, 25-02, 25-03, 25-04, 25-07, 25-08 | Per-trigger automatic message with `{count}`, `{window}`, `{percent}`, notes appended | ✓ SATISFIED | Truth 2 |
| EVT-08 | 25-01, 25-03, 25-05, 25-07, 25-08 | Lift message whoever lifted it | ✓ SATISFIED | Truths 3, 4 |
| EVT-10 | 25-02, 25-04, 25-05, 25-06, 25-08 | Staff Group, alerts, raid-scale removals, log posts, captcha challenges are not events | ✓ SATISFIED | Truths 5, 6, 7 |

No orphaned requirements: REQUIREMENTS.md maps only EVT-06, EVT-07, EVT-08 and EVT-10 to Phase 25.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (production files of the phase) | | no TBD/FIXME/XXX/TODO/HACK or `nolint` added; no `first production caller arrives in plan 25-` note left | ℹ️ Info | none |
| `fuku/modules/event_message.go` | 392, 467 | generic "the default was sent" line also fires for the group-menu lift, whose default is empty | ⚠️ Warning (advisory) | log wording only; IDs only; documented |

Earlier tests edited by the phase (`settings_messages_*_test.go`, `settings_staff_privacy_test.go`, `event_message_pm_test.go`, `pm_copy_callers_test.go`, `fuku/i18n/settings_locale_test.go`, `fuku/db/eventmsg/repository_test.go`) were read in the diff: they follow the new hub shape (`[2 2 1 1 2]`), add the Lockdown family's rows, limits and the five keys, and add assertions; none weakens one.

### Human Verification Required

1. **Manual start on live Telegram**: custom reply in a topic with a button, notes after it, lock first, alert unchanged, never deleted.
2. **Automatic start on live Telegram**: a real join surge locks first, then one new message with the numbers, trigger sentence and notes.
3. **Lift from every door on live Telegram**: `/unlockdown` reply, alert button, private menu and group menu each post the custom lifted message in the locked group naming the lifter; with nothing stored the group-menu lift posts nothing beside its redraw.
4. **Refusal on live Telegram**: a foreign photo file ID; the action, the refusal, today's default once.
5. **Silent on live Telegram**: `/lockdown` and `/unlockdown` with their events silenced answer nothing; the alert and the tally behave as before.
6. **Owner decisions P1 to P15** of 25-CONTEXT.md (keys, codes, icons and family position; only a successful `/lockdown` is an event; `{percent}` for every event; `{window}` as bare seconds; `{reason}` per event; person placeholders name the recorded person or the bot; note placement; the 400-unit reserve; media allowed; Silent on every lockdown event; group-menu custom lift; never auto-deleted; preview samples; the reworded settings intro; `{chatname}` from the stored title).
7. **PostgreSQL run.** `make test-postgres-integrity` with a database. Expected: pass unchanged (no new guarantee, D-40).

The 25-08 SUMMARY records no separate list of live checks; the items above are derived from what the fakes cannot prove.

### Accepted follow-ups

- The misleading "the default was sent" log line after a group-menu lift read error (or refused custom message): IDs only, behaviour correct, documented in code and AGENTS.md (25-07 SUMMARY "Notes for plan 25-08").
- AGENTS.md's `TestPMCopyCallers` sentence omits `aispam_notice.go` (Phase 24).
- Roadmap, requirements and state bookkeeping for Phase 25 left to the orchestrator.

### Gaps Summary

No gaps. Each success criterion and the four requirements are implemented in production code, wired from the real send sites (`lockdown`, `raidPostNotice` inside `raidPostAsync`, `unlockdown`, `lockdownAnnounceLift`, `settingsLockdownLiftYes`) through the one adapter file into the generic engine, and proven by passing tests that exercise the behaviour; five mutations I made each turned a test red, among them both EVT-10 guards (structural and behavioural) and the group-menu default. The lockdown golden was committed alone first and is unchanged, and the Phase 21, 23 and 24 goldens still pass on the extended engine. `eventLockdownAdapterCallers` has exactly five entries. The status is human_needed because of the live Telegram checks, the owner's fifteen provisional decisions and the PostgreSQL run.

---

_Verified: 2026-11-22T06:25:00Z_
_Verifier: Claude (gsd-verifier)_
