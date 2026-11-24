---
phase: 24-captcha-ai-spam-and-member-requirement-notices
verified: 2026-11-22T02:50:00Z
status: human_needed
score: 12/12 must-haves verified
covered_files: [".planning/phases/24-captcha-ai-spam-and-member-requirement-notices/24-01-PLAN.md", ".planning/phases/24-captcha-ai-spam-and-member-requirement-notices/24-01-SUMMARY.md", ".planning/phases/24-captcha-ai-spam-and-member-requirement-notices/24-02-PLAN.md", ".planning/phases/24-captcha-ai-spam-and-member-requirement-notices/24-02-SUMMARY.md", ".planning/phases/24-captcha-ai-spam-and-member-requirement-notices/24-03-PLAN.md", ".planning/phases/24-captcha-ai-spam-and-member-requirement-notices/24-03-SUMMARY.md", ".planning/phases/24-captcha-ai-spam-and-member-requirement-notices/24-04-PLAN.md", ".planning/phases/24-captcha-ai-spam-and-member-requirement-notices/24-04-SUMMARY.md", ".planning/phases/24-captcha-ai-spam-and-member-requirement-notices/24-05-PLAN.md", ".planning/phases/24-captcha-ai-spam-and-member-requirement-notices/24-05-SUMMARY.md", ".planning/phases/24-captcha-ai-spam-and-member-requirement-notices/24-06-PLAN.md", ".planning/phases/24-captcha-ai-spam-and-member-requirement-notices/24-06-SUMMARY.md", ".planning/phases/24-captcha-ai-spam-and-member-requirement-notices/24-07-PLAN.md", ".planning/phases/24-captcha-ai-spam-and-member-requirement-notices/24-07-SUMMARY.md", ".planning/phases/24-captcha-ai-spam-and-member-requirement-notices/24-08-PLAN.md", ".planning/phases/24-captcha-ai-spam-and-member-requirement-notices/24-08-SUMMARY.md", "AGENTS.md", "fuku/db/eventmsg/events.go", "fuku/db/eventmsg/repository.go", "fuku/modules/aispam.go", "fuku/modules/aispam_notice.go", "fuku/modules/aispam_notice_test.go", "fuku/modules/captcha.go", "fuku/modules/event_message.go", "fuku/modules/event_message_auto.go", "fuku/modules/event_message_mreq_test.go", "fuku/modules/event_message_pm.go", "fuku/modules/event_message_pm_screening_test.go", "fuku/modules/event_message_screening_safety_test.go", "fuku/modules/event_message_screening_test.go", "fuku/modules/memberreq_enforce.go", "fuku/modules/memberreq_notice.go", "fuku/modules/pm_copy_callers_test.go", "fuku/modules/screening_messages_e2e_test.go", "fuku/modules/screening_notice_legacy_request_test.go", "fuku/modules/settings_edit.go", "fuku/modules/settings_edit_actions.go", "fuku/modules/settings_edit_check.go", "fuku/modules/settings_messages.go", "fuku/modules/settings_messages_screening_test.go"]
covered_digest: "v4:sha256:f18aa2e48b1186be6ba02b640967064e51610ccb8ff29b8a25d33c5f0adc9b00"
behavior_unverified: 0
overrides_applied: 0
advisory:
  - finding: "The per-send 10 s bound of the AI spam notice and of a custom requirement notice is created but never reaches the HTTP request"
    category: other
    reason: "aispamNoticePaced (fuku/modules/aispam_notice.go:116-122) and memberReqPaced wrap each call in a 10 s context, but the AI notice's two sends (aispam_notice.go:155 helpers.SendMessageWithErrorHandling, :164 media.SendGreetingMessage) and the requirement site's custom send (memberreq_notice.go:390-391 media.SendGreetingMessage) ignore the context they are given, so an in-flight request is bounded only by the bot's HTTP client (constants.LongTimeout, 30 s, main.go:110). AGENTS.md:2841 ('each send bounded by 10 s and derived from aispamContext()') and the comments at aispam_notice.go:116 and memberreq_notice.go:266-268 therefore overstate the bound. The pacer wait does derive from the context, so a shutdown still sheds a waiting notice, and DrainAISpamChecks gives up after its own 9 s, so the 70 s worst case and correctness are unchanged. The same pattern already exists in Phase 22's private copy job (event_message_pm.go:368-371), because media.SendGreetingMessage takes no context. Fix: send the AI notice's default with SendMessageWithContext(callCtx, ...) (it has no golden), or correct the two sentences."
    evidence_status: "code read, reproducible; no failing test (behaviour only differs on a request that hangs 10-30 s)"
  - finding: "A refused custom requirement notice costs one extra Telegram request before the default is sent"
    category: other
    reason: "memberReqNoticeSend (memberreq_notice.go:275-303) retries any non-rate-limit failure once, so a 400 on the custom message is sent twice before postEventGroup's refusal branch sends the default. Harmless (the default follows, as TestEventMessageMemberReq 'refused' proves) and consistent with today's retry rule for every requirement notice."
    evidence_status: "code read"
  - finding: "Bookkeeping not yet closed: ROADMAP.md Phase 24 checkbox and its eight plan lines are still `[ ]`, REQUIREMENTS.md keeps EVT-04, EVT-05 and EVT-09 Pending, STATE.md still says `Phase: 23 of 30`"
    category: other
    reason: ".planning/ROADMAP.md:62 and :203-210, .planning/REQUIREMENTS.md:31-36 and :103-108, .planning/STATE.md:27. Normal phase-close work for the orchestrator (this verifier must not edit them), not a code gap."
    evidence_status: "file read, reproducible"
human_verification:
  - test: "Live Telegram, failed captcha: set a custom Failed captcha message with {mention}, {action}, {count} and {reason}; let a joiner time out on the math captcha after sending a message, and let another fail a Turnstile check"
    expected: "Each time the failure action happens first, then the custom text appears as a new message (no reply, no topic) with the action word and the held count, and deletes itself after about 30 seconds; the captcha challenge message itself is unchanged; the verify page shows the same failed answer as before"
    why_human: "The timer, the web verify route and Cloudflare are faked in tests; the real 30 s self-delete, the Turnstile page and a live client's rendering are not provable offline"
  - test: "Live Telegram, requirement mute: set a custom Mute (Member requirements) text with a URL button; a member who breaks a rule (no last name) posts"
    expected: "The member is muted, then the custom text, a blank line and the requirements line (plus the photo remedy when that rule broke), with the custom button first and 'I fixed it' below it; after fixing the name the member taps 'I fixed it', is unmuted and the notice is edited to the fixed text with no keyboard; the screen has no Silent switch and no media part"
    why_human: "The fake proves the request bytes and the callback; how a real client draws the keyboard and whether the live callback lifts the live restriction is client and server behaviour"
  - test: "Live Telegram, requirement warn and warn limit: a custom Warning (Member requirements) text with {reason}, {count}, {limit} and {action}; let a member reach the warn limit through the requirement"
    expected: "Each warn posts the custom text followed by the requirements line; at the limit the limit action happens first and {action} names it"
    why_human: "Live ordering of the warn module's limit action and the notice"
  - test: "Live Telegram, AI spam notice: with /aispam on and the notice off, post spam; then turn the AI spam event's Notice switch on in the Messages page and post spam from one member three times and from four members within a minute (a forum topic for one of them)"
    expected: "With the notice off every spam message is deleted and nothing is posted. With it on every spam message is still deleted, the first one per member posts a notice in that message's topic, a member's later spam within five minutes posts nothing, and no more than three notices appear in the group in one minute; the notice never names the AI provider or a probability"
    why_human: "TypeSafe and Telegram are faked; the real verdict latency, the topic placement and the caps under a real wave need a live group with Redis"
  - test: "Live Telegram, refusal: store a custom Failed captcha message whose photo file ID belongs to another bot, and fail a captcha"
    expected: "The failure action happens, Telegram refuses the photo, and today's default captcha notice is posted once and deletes itself after 30 s"
    why_human: "The refusal is scripted in tests; the real Bot API error code for a foreign file ID is not provable offline"
  - test: "Live Telegram, private copies: turn on the private copy of Failed captcha and of Mute (Member requirements) for a member who started the bot"
    expected: "The member receives the default private text naming the group, the action and, for the requirement, the broken rules on the reason line; the requirement mute copy tells them to tap 'I fixed it' under the group's notice and carries no button"
    why_human: "Live private chat delivery"
  - test: "Owner reviews the provisional decisions P1 to P16 of 24-CONTEXT.md ('Provisional Decisions for Owner Confirmation'), plus the shared plain event names"
    expected: "The owner confirms or asks to change each: P1 codes cf ai rw rk rb rm, keys, family 'Member requirements' 🛂 and icons 🧩 🧠 ❗ 👢 ⛔ 🔇, and the requirement events' plain names Warning, Kick, Ban, Mute, which they share with other families (also in the #PM_COPY log line, 24-04-SUMMARY); P2 the requirement warn also covers the warn-limit notices; P3 the pause and AI breaker notices stay non-events; P4 no media part on requirement events; P5 requirements line after a blank line, 'I fixed it' below custom buttons; P6 the requirement mute cannot be silenced; P7 requirement {reason} = the broken rules, {until} empty for the mute; P8 captcha {action}/{count}/{until}/{reason} values, {duration} empty; P9 the captcha message keeps its 30 s self-delete; P10 AI notice caps 1 per member per 5 min, 3 per group per minute, fail closed, a refusal also sheds the private copy; P11 AI notice in the topic, never a reply, automatic punishments timer; P12 the AI notice names the member; P13 all six events have an automatic (capped) private copy, off by default; P14 a 600-unit reserve ('Up to 3496'); P15 the preview samples; P16 captcha {chatname} from the stored chats row"
    why_human: "These are planner defaults taken without asking the owner (the run was authorized autonomous); each is reversible as a table row, a constant or one site's field"
  - test: "Run `make test-postgres-integrity` against a PostgreSQL database (DATABASE_URL set, FUKU_TEST_DATABASE=true)"
    expected: "Passes unchanged. Phase 24 adds no migration, index or CHECK (D-41), but the six new event key strings are written to chat_event_messages.event for the first time and the aispam group row is inserted with silent=true by the part writers"
    why_human: "No PostgreSQL in this environment; 24-08 did not run it either. The SQLite halves pass (fuku/db/eventmsg ok)"
audit_acknowledged:
  milestone: v1.2
  at: 2026-11-24
  status: human_needed
---

# Phase 24: Captcha, AI Spam and Member Requirement Notices Verification Report

**Phase Goal:** As the owner, I can customize the message for a failed captcha and the notices for member requirement breaches, and I can turn on a rate-limited notice for AI spam deletions.
**Verified:** 2026-11-22T02:50:00Z
**Status:** human_needed
**Re-verification:** No, initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC1 / EVT-04: a member who fails the captcha by timeout or by a failed web check gets the custom message in the group and, if on, by PM | ✓ VERIFIED | `handleCaptchaTimeout` (`captcha.go:1563`) now calls only `postCaptchaFailureNotice` (`event_message_auto.go:282-300`) after `executeCaptchaFailureAction` and its settle; the timer, cleanup tick, recovery and the web `Fail`/`Expire` (`captcha_web.go`, untouched) all reach it. The site is `Automatic`, so the private copy (spec `eventmsg_pm_captcha`, `event_message_pm.go:91`) is capped. `TestEventMessageCaptcha` (timer custom, web fail custom), `TestEventPMCaptcha`, e2e `owner_sets_the_captcha_message`, `web_check` (replay posts nothing), `pm_copies` (cap 2, three joiners, two copies) pass |
| 2 | SC1: the captcha challenge message is unchanged | ✓ VERIFIED | The only `captcha.go` change is in `handleCaptchaTimeout` after the challenge delete (diff 86baf1a..HEAD: the name read and the notice send); `SendCaptcha` and the challenge are untouched. `TestCaptchaWeb*` and e2e `owner_sets_the_captcha_message` (the math challenge carries its own text) pass |
| 3 | SC2 / EVT-09: a requirement notice (warn, kick, ban or mute) takes a custom text; the list of broken rules still follows it | ✓ VERIFIED | `memberReqAnnounce` (`memberreq_enforce.go:562`) posts through `memberReqPostEventNotice` (`memberreq_notice.go:350-418`): event by action (`memberReqEventOf`), `Note` = `memberReqNoticeAppended` (blank line, `eventmsg_mreq_rules`, remedies), `TextOnly`. `TestEventMessageMemberReq` (warn custom, warn limit custom, kick silent, refused, private copy) and e2e `requirement_warn` pass |
| 4 | SC2: a mute still carries "I fixed it", which works as before (24 h cap, reschedule from the tap) | ✓ VERIFIED | `Required` for the mute (`memberreq_notice.go:404`, a stored Silent is ignored, `eventGroupMessage` `event_message.go:399-405`), `KeepRows` + `KeepRowsLast` (`:407`), `Schedule` = `memberReqScheduleNotice(sent, withButton)` (24 h cap), the posted message ID stored by `SetHoldNotice` so `memberReqFixedEdit`'s reschedule matches. Page: no Silent row, `hq rm` refused (`settingsMsgSilentFields`, `settingsMsgRunSilent`). `TestEventMessageMemberReqMute` (custom keeps the button, silent and media ignored, undelivered), e2e `requirement_mute` (tap 20 min later unmutes, edits, moves the row to the tap + 900 s) pass. My mutation `Required: false` failed `TestEventMessageMemberReqMute` |
| 5 | SC3 / EVT-05: the AI spam notice is off by default, and a deletion then posts nothing | ✓ VERIFIED | `eventmsg.OffByDefault` / `DefaultSilentFor` (`events.go:130-142`), used by `Resolve` and the part writers' insert (`repository.go:78, 257`); the engine reads `Resolve` and posts nothing on a read error for an off-by-default event (`event_message.go:374-380`). `TestAISpamNotice/off_by_default` (deleted, nothing posted, no `fuku:aispam:notice:*` key), `TestEventMessageEngineHooks` (read error, no row), e2e `ai_notice`. My mutation `OffByDefault` → false failed both |
| 6 | SC3: with it on, at most one notice per person per few minutes and a capped number per group per minute; a wave posts at most the cap and every spam message is still deleted | ✓ VERIFIED | `processAISpam` deletes first and calls `postAISpamNotice` only when the delete succeeded (`aispam.go:336-355`); `aispamNoticeScript` (`SET NX EX 300` per member, `INCR`/`EXPIRE 120` per minute, ≤ 3) through `eventSite.Admit`, fail closed (`aispam_notice.go:70-102`), its own pacer. `TestAISpamNotice/caps`, `quiet_failures`, `shutdown`; e2e `ai_notice` (third spam inside 5 min quiet, three other members → two more notices, all deleted). 24-08's always-allow `Admit` mutation failed the e2e test |
| 7 | SC4: a group with nothing stored gets today's captcha and requirement notices byte for byte (and the pause notice, the AI breaker notice, the silent AI delete) | ✓ VERIFIED | `TestLegacyScreeningNoticeRequests` passes; `git diff --exit-code e40a7be -- fuku/modules/screening_notice_legacy_request_test.go` exits 0 and the file has one commit (its own). `TestScreeningEventDefaults` compares the builders with the goldens; e2e `nothing_stored` (real locales) matches `goldenScreenCaptchaKick`, `goldenScreenReqKick`, `goldenScreenReqMute`. The pause notice still goes through `memberReqPostNotice`, `aispamNotifyBreaker` is untouched |
| 8 | SC4: a refused custom message falls back to the default, and the deletion timers apply as before | ✓ VERIFIED | `postEventGroup` refusal branch (`event_message.go:451-458` (the `eventMessageRefused` branch)) sends `site.SendDefault` once; captcha `Schedule` is the in-process 30 s self-delete (`event_message_auto.go:263-275`, never queued), requirement `memberReqScheduleNotice` (`bot:punish_auto`, mute at the cap), AI notice `Kind: bot:punish_auto`. Refusal tests: captcha (`TestEventMessageCaptcha/refused`, e2e `refused`), requirement (`TestEventMessageMemberReq/refused`, safety `requirement refused`), AI (`ai spam refused`). My mutation replacing the captcha `Schedule` failed `TestEventMessageCaptcha`. `TestMemberDeletionExcludedMessages` passes |
| 9 | Phase 21-23 sends unchanged after the engine hooks | ✓ VERIFIED | `git diff --exit-code 9d4554d -- .../punishment_notice_legacy_request_test.go` and `86baf1a -- .../moderation_legacy_request_test.go` exit 0; both goldens pass; every hook's zero value is the old behaviour (`event_message.go:137-159`) |
| 10 | Privacy: no Staff Group name, AI verdict detail or stored text in a log line or message (beyond the existing `[AISpam] verdict` operator line) | ✓ VERIFIED | New log lines carry chat ID, user ID, event and code only (`aispam_notice.go:94, 178-182`, `event_message.go:377, 380, 386`); the AI texts name no provider, model or probability in any of the 8 locales (`aispam_notice_deleted`, `eventmsg_reason_aispam`, `eventmsg_pm_aispam`). `TestEventMessageScreeningLogsCarryIDsOnly`, `TestEventMessageScreeningPrivacy`, `TestSettingsStaffPrivacy` pass |
| 11 | No staff, lockdown or raid file reaches the engine; no new table, migration, handler group, drain or dependency | ✓ VERIFIED | `TestPMCopyCallers` (allow-lists with `postEventSent` and the three new callers) passes; `git diff --stat 86baf1a HEAD -- go.mod go.sum main.go migrations` is empty; `TestCommandRepliesStayInsideTheHandler` passes |
| 12 | Locale keys present in 8 languages; AGENTS.md matches what Phase 24 built | ✓ VERIFIED | `make check-translations` all present; `TestEventMsgLocaleKeys`, `TestMemberReqLocaleKeys`, `TestSettingsLocaleKeys`, `TestSettingsLabelLengths`, `TestLocaleStyle`, `TestLocalePlaceholdersMatchOtherLanguages` pass. AGENTS.md read against code (the `fuku:aispam:notice:*` keys and script, `DrainAISpamChecks` note, engine hooks, Scheduling bullet, `OffByDefault`/`DefaultSilentFor`, requirement site, page shapes); one overstated sub-clause ("each send bounded by 10 s", AGENTS.md:2841) is recorded as an advisory |

**Score:** 12/12 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `fuku/modules/screening_notice_legacy_request_test.go` | goldens of the captcha, requirement, pause, AI breaker notices and the silent AI delete, committed alone first | ✓ VERIFIED | one commit (e40a7be), unchanged since; passes |
| `fuku/db/eventmsg/events.go`, `repository.go` | six event keys, `OffByDefault`, `DefaultSilentFor`, `Resolve` and insert default | ✓ VERIFIED | no migration; `fuku/db/eventmsg` tests pass |
| `fuku/modules/event_message.go` | `SendCustom`, `Schedule`, `Admit`, `Required`, `TextOnly`, `postEventSent` | ✓ VERIFIED | used by the three sites |
| `fuku/modules/event_message_auto.go` | captcha site, captcha and AI default builders, words | ✓ VERIFIED | `postCaptchaFailureNotice` called from `captcha.go:1563` |
| `fuku/modules/memberreq_notice.go` | requirement site, default builder, appended note, 600-unit reserve | ✓ VERIFIED | called from `memberReqAnnounce` |
| `fuku/modules/aispam_notice.go` | AI notice site, caps, pacer | ✓ VERIFIED | called from `processAISpam` after a successful delete |
| `fuku/modules/event_message_pm.go` | six automatic specs | ✓ VERIFIED | |
| `fuku/modules/settings_messages.go`, `settings_edit*.go` | captcha and AI in Automatic, Member requirements family, Notice switch, `NoSilent`, `NoMedia`, `ReservedText` | ✓ VERIFIED | `TestSettingsMessagesScreeningFamily`, `TestSettingsEditReservedText`, `TestSettingsAuthorityMessagesScreening` |
| `fuku/modules/screening_messages_e2e_test.go` | end-to-end proof | ✓ VERIFIED | 8 subtests through the real dispatcher and real page presses |
| `locales/*.yml` | new keys in 8 languages | ✓ VERIFIED | |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `handleCaptchaTimeout` | `postEvent` | `postCaptchaFailureNotice` | ✓ WIRED | captcha.go:1563 → event_message_auto.go:300 |
| web `Fail` / `Expire` | `handleCaptchaTimeout` | `settleCaptchaAttempt` / `expireCaptchaAttempt` | ✓ WIRED | unchanged path; e2e `web_check` |
| `memberReqAnnounce` | `postEventSent` | `memberReqPostEventNotice` | ✓ WIRED | memberreq_enforce.go:562 → memberreq_notice.go:409 |
| posted notice ID | hold `notice_msg_id` | `SetHoldNotice` | ✓ WIRED | "I fixed it" reschedule matches the posted message |
| `processAISpam` delete verdict | `postAISpamNotice` → `postEvent` | `deleteErr == nil` | ✓ WIRED | aispam.go:353-355 |
| `postEventSent` | `site.Admit` | only when the group message or private copy would be sent | ✓ WIRED | event_message.go:383 |
| Messages page | stored rows read by the send path | `eventmsg.Get` (skipLocal key) | ✓ WIRED | e2e presses change the next send |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| captcha custom message | target names, action, held count, mute end | live `getChatMember`, captcha settings, `stored_messages` | Yes | ✓ FLOWING |
| requirement custom message | broken rules mask, warn count and limit, limit action | the hold, `applyWarn` result | Yes | ✓ FLOWING |
| AI notice | member, topic | the aispam task (`task.user`, `task.threadID` from the message) | Yes | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Phase 24 families, the three goldens, neighbouring families (145 top-level tests) | `go test -tags testtools -race -count=1 -run '^(TestScreeningMessagesEndToEnd\|TestLegacy...\|TestAISpamNotice\|TestEventMessage.*\|TestEventPM.*\|TestSettingsMessages.*\|TestMemberReqFixed.*\|TestMemberReqNotice.*\|TestCaptchaWeb.*\|TestSettingsStaffPrivacy\|TestPMCopyCallers\|...)$' -v ./fuku/modules` | `ok 74.9s`, 145 PASS, 0 SKIP, 0 FAIL; all 8 `TestScreeningMessagesEndToEnd` subtests pass | ✓ PASS |
| Locale tests | `go test ... -run '^(TestEventMsgLocaleKeys\|TestMemberReqLocaleKeys\|TestSettingsLocale.*\|TestSettingsLabelLengths\|TestLocaleStyle\|TestLocalePlaceholdersMatchOtherLanguages\|TestHelpPagesKeepTheirCommands\|TestRawPlaceholderKeysRender)$' ./fuku/i18n` | `ok 23.5s` | ✓ PASS |
| eventmsg repository | `go test -tags testtools -race -count=1 ./fuku/db/eventmsg` | `ok 1.1s` | ✓ PASS |
| Goldens unchanged | `git diff --exit-code e40a7be / 9d4554d / 86baf1a -- <golden>` | exit 0, 0, 0 | ✓ PASS |
| Translations, docs, shards, build | `make check-translations`, `make check-docs`, `make test-shard-check`, `CGO_ENABLED=0 go build ./...` | all present; no drift; clean; ok | ✓ PASS |
| Mutation: requirement mute not required | `Required: false` → `TestEventMessageMemberReqMute` | FAIL (0 messages posted) | ✓ caught |
| Mutation: AI notice on by default | `OffByDefault` → false → `TestAISpamNotice`, e2e | FAIL (off_by_default, ai_notice) | ✓ caught |
| Mutation: captcha notice queued instead of self-deleting | `Schedule` → `Kind: bot:punish_auto` → `TestEventMessageCaptcha` | FAIL (timer custom, nothing stored, refused) | ✓ caught |
| Full fuku/modules | `go test -tags testtools -race -count=1 -timeout 45m ./fuku/modules` | `ok 338.5s` (5 min 41 s wall), exit 0 | ✓ PASS |
| Lint on new code | `CGO_ENABLED=0 golangci-lint run --new-from-rev=origin/main ./...` | `0 issues.` | ✓ PASS |

### Probe Execution

Not applicable: the phase declares no probe scripts.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| EVT-04 | 24-01..03, 24-04, 24-07, 24-08 | A failed captcha has its own message, for the timeout and a failed web check alike | ✓ SATISFIED | Truths 1, 2 |
| EVT-05 | 24-02..04, 24-06..08 | AI spam notice, off by default, capped per person and per group | ✓ SATISFIED | Truths 5, 6 |
| EVT-09 | 24-02..05, 24-07, 24-08 | Requirement notices take a custom text, keep the broken rules list and the mute's "I fixed it" | ✓ SATISFIED | Truths 3, 4 |

No orphaned requirements: REQUIREMENTS.md maps only EVT-04, EVT-05 and EVT-09 to Phase 24.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (production files of the phase) | | no TBD/FIXME/XXX/TODO/HACK added, no `nolint` added, no `first production caller arrives in plan 24-` note left | ℹ️ Info | none |
| `fuku/modules/aispam_notice.go` | 116-122, 155, 164 | bounded context created and ignored by the send | ⚠️ Warning (advisory) | request bounded by 30 s, not 10 s; doc overstates |
| `fuku/modules/memberreq_notice.go` | 390-391 | custom send ignores the per-try context | ⚠️ Warning (advisory) | same; pre-existing pattern in `event_message_pm.go:368` |

Earlier tests edited by the phase were read: the changes follow the new page shapes (hub `[2 1 1 1 2]`, Automatic list `[2 2 2 1 2]`), add the delete word and the two new reasons to `TestEventActionWords`, add the screening events to `TestEventPMTextsFitEveryLocale` (with the all-rules reason), shorten the hang timeouts of three memberreq tests (300/400 ms, still longer than the 150 ms budget the tests prove) and cache the marker locale; none weakens an assertion.

### Human Verification Required

1. **Failed captcha on live Telegram** (timeout and Turnstile): failure action first, custom text as a new message, self-delete after ~30 s, challenge unchanged. Why human: real timer, Cloudflare and client.
2. **Requirement mute on live Telegram**: muted first, custom text, requirements line, custom button then "I fixed it"; tapping it after a fix unmutes and edits. Why human: live callback and client keyboard.
3. **Requirement warn and warn limit on live Telegram**: requirements line after each custom warn; limit action first, `{action}` names it. Why human: live ordering.
4. **AI spam notice on live Telegram**: off: deleted, nothing posted; on: notice in the topic, one per member per 5 min, at most 3 a minute, every spam deleted, no provider or probability named. Why human: real verdicts, topics and caps with Redis.
5. **Refusal on live Telegram**: a foreign photo file ID on the captcha message; action, refusal, today's default once. Why human: the real error code.
6. **Private copies on live Telegram**: captcha and requirement mute copies with the default texts. Why human: live delivery.
7. **Owner decisions P1 to P16** (24-CONTEXT.md) plus the shared plain event names (Warning, Kick, Ban, Mute, also in `#PM_COPY`). Confirm or change each.
8. **PostgreSQL run.** `make test-postgres-integrity` with a database. Expected: pass unchanged (no new guarantee, D-41).

### Gaps Summary

No gaps. Every roadmap success criterion and the three requirements are implemented in production code, wired from the real send sites (`handleCaptchaTimeout`, `memberReqAnnounce`, `processAISpam`) through the generic engine, and proven by passing tests that exercise the behaviour; three mutations I made (the mute's `Required`, `OffByDefault`, the captcha's self-delete schedule) each turned a test red. The screening golden was committed alone first and is unchanged, and the Phase 21 and 23 goldens still pass on the hooked engine. Advisories: the 10 s per-send bound of the AI notice and the custom requirement send is not applied to the HTTP request (30 s client timeout instead; AGENTS.md:2841 overstates it), a refused custom requirement notice costs one extra request, and the roadmap, requirements and state bookkeeping are still open. The status is human_needed because of the live Telegram checks, the owner's sixteen provisional decisions plus the shared event names, and the PostgreSQL run.

---

_Verified: 2026-11-22T02:50:00Z_
_Verifier: Claude (gsd-verifier)_
