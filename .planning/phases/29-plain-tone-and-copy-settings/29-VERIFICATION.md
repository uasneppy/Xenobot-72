---
phase: 29-plain-tone-and-copy-settings
verified: 2026-11-23T15:58:00Z
status: passed
score: 21/22 must-haves verified
covered_files:
  - .planning/phases/29-plain-tone-and-copy-settings/29-01-PLAN.md
  - .planning/phases/29-plain-tone-and-copy-settings/29-01-SUMMARY.md
  - .planning/phases/29-plain-tone-and-copy-settings/29-02-PLAN.md
  - .planning/phases/29-plain-tone-and-copy-settings/29-02-SUMMARY.md
  - .planning/phases/29-plain-tone-and-copy-settings/29-03-PLAN.md
  - .planning/phases/29-plain-tone-and-copy-settings/29-03-SUMMARY.md
  - .planning/phases/29-plain-tone-and-copy-settings/29-04-PLAN.md
  - .planning/phases/29-plain-tone-and-copy-settings/29-04-SUMMARY.md
  - .planning/phases/29-plain-tone-and-copy-settings/29-05-PLAN.md
  - .planning/phases/29-plain-tone-and-copy-settings/29-05-SUMMARY.md
  - .planning/phases/29-plain-tone-and-copy-settings/29-06-PLAN.md
  - .planning/phases/29-plain-tone-and-copy-settings/29-06-SUMMARY.md
  - AGENTS.md
  - Makefile
  - fuku/db/advthresholds/repository.go
  - fuku/db/advthresholds/repository_test.go
  - fuku/db/antiflood/repository.go
  - fuku/db/antiflood/repository_test.go
  - fuku/db/backup/event_messages_backup_test.go
  - fuku/db/escalation/repository.go
  - fuku/db/escalation/repository_test.go
  - fuku/db/eventmsg/events.go
  - fuku/db/eventmsg/repository.go
  - fuku/db/eventmsg/repository_test.go
  - fuku/db/eventmsg/validate_write.go
  - fuku/db/eventmsg/validate_write_test.go
  - fuku/db/migrations/event_message_plain_tone_migration_test.go
  - fuku/db/models/event_message.go
  - fuku/db/modtuning/repository.go
  - fuku/db/modtuning/repository_test.go
  - fuku/i18n/eventmsg_locale_test.go
  - fuku/i18n/settings_locale_test.go
  - fuku/modules/aispam_notice.go
  - fuku/modules/antiflood.go
  - fuku/modules/bans.go
  - fuku/modules/copy_settings.go
  - fuku/modules/copy_settings_test.go
  - fuku/modules/event_message.go
  - fuku/modules/event_message_apply.go
  - fuku/modules/event_message_auto.go
  - fuku/modules/event_message_lockdown.go
  - fuku/modules/event_message_plain.go
  - fuku/modules/event_message_plain_test.go
  - fuku/modules/event_message_pm.go
  - fuku/modules/event_message_pm_details_test.go
  - fuku/modules/event_message_pm_failures_test.go
  - fuku/modules/event_message_pm_log_test.go
  - fuku/modules/event_message_pm_safety_test.go
  - fuku/modules/event_message_pm_test.go
  - fuku/modules/memberreq_notice.go
  - fuku/modules/modtuning_appeal_test.go
  - fuku/modules/mute.go
  - fuku/modules/phase29_e2e_test.go
  - fuku/modules/phase29_legacy_request_test.go
  - fuku/modules/settings_copy.go
  - fuku/modules/settings_copy_test.go
  - fuku/modules/settings_edit.go
  - fuku/modules/settings_main_test.go
  - fuku/modules/settings_messages.go
  - fuku/modules/settings_messages_auto_test.go
  - fuku/modules/settings_messages_lockdown_test.go
  - fuku/modules/settings_messages_pm_options_test.go
  - fuku/modules/settings_messages_pm_preview_test.go
  - fuku/modules/settings_messages_safety_test.go
  - fuku/modules/settings_messages_screening_test.go
  - fuku/modules/settings_messages_screens_test.go
  - fuku/modules/settings_messages_test.go
  - fuku/modules/settings_phase19_test.go
  - fuku/modules/settings_phase29_test.go
  - fuku/modules/settings_staff_privacy_test.go
  - fuku/modules/warns.go
  - locales/en.yml
  - locales/es.yml
  - locales/fr.yml
  - locales/hi.yml
  - locales/id.yml
  - locales/pt.yml
  - locales/ru.yml
  - locales/uk.yml
  - migrations/20261123130000_add_event_message_plain_tone.sql
covered_digest: "v4:sha256:e3a788cce1343e9ab02b4d9275b29e4b508941207b477bdc147c54ef53a7ffaf"
behavior_unverified: 1
overrides_applied: 0
behavior_unverified_items:
  - truth: "Migration 20261123130000_add_event_message_plain_tone.sql idempotently adds plain_tone BOOLEAN NOT NULL DEFAULT FALSE to chat_event_message_options on PostgreSQL with no transaction control, verified by statement and PostgreSQL integrity tests."
    test: "FUKU_TEST_DATABASE=true DATABASE_URL=... make test-postgres-integrity (specifically TestEventMessagePlainToneMigrationDefaults)"
    expected: "Migration applies cleanly twice inside one transaction on PostgreSQL, and a row inserted with only chat_id reads pm_details = FALSE, pm_log = FALSE, and plain_tone = FALSE."
    why_human: "No PostgreSQL instance is configured locally (DATABASE_URL unset); TestEventMessagePlainToneMigrationDefaults skipped. Structural DDL checks (TestEventMessagePlainToneMigrationStatements) and SQLite repository/replace checks (TestPlainToneOption, TestEventMsgValidateAndReplaceAll) passed, but PostgreSQL DDL execution was not exercised."
human_verification:
  - test: "Run PostgreSQL integrity tests (FUKU_TEST_DATABASE=true DATABASE_URL=... make test-postgres-integrity) covering TestEventMessagePlainToneMigrationDefaults."
    expected: "Migration 20261123130000_add_event_message_plain_tone.sql runs idempotently twice on PostgreSQL and a row inserted with only chat_id defaults plain_tone to FALSE."
    why_human: "No PostgreSQL server is available in this environment (DATABASE_URL unset), so TestEventMessagePlainToneMigrationDefaults skipped."
  - test: "Run CI static checks (make lint and gosec) on this tree."
    expected: "Both golangci-lint and gosec exit 0 with zero new findings on Phase 29 files."
    why_human: "Neither golangci-lint nor gosec is installed in this local environment."
  - test: "Perform live Telegram UAT in two real supergroups + bot private chat: (1) open private /settings -> Messages (h) on Source Group, toggle Plain tone ON (ht), verify Main shows 'Plain tone' when no custom messages exist, run /ban and /kick (with PM copy ON) and confirm neutral emoji-free default messages in group and PM, then set a custom /ban text and confirm custom text overrides Plain tone while /kick stays neutral; (2) open private /settings -> Copy settings (cp) on Source Group, select Target Group (ck), confirm the card names both group titles, all 6 overwritten areas, excluded items, and one-time non-linking notice, tap Yes (cy), and verify Target Group mirrors all 6 areas while preserving Target Group's antiflood-off state, blacklist word list, warn_limit, approvals, and Staff Group links, and that replaying an old card is refused."
    expected: "Every screen, button label, and confirmation card renders cleanly in Telegram, Plain tone replaces uncustomized defaults across group and PM while leaving custom texts untouched, and Copy Settings mirrors all 6 areas once with opaque refusals and replay protection."
    why_human: "Visual rendering, live Telegram API keyboard acceptance, multi-replica Redis timing, and end-to-end UX cannot be verified offline."
  - test: "Confirm or request changes to provisional owner decisions P29-01 through P29-04 documented in AGENTS.md."
    expected: "Owner signs off on: P29-01 (full mirror on copy — empty or default source entries clear the target's custom message, switch, option, escalation ladder, manual defaults, reason shortcuts, or advanced thresholds across all 6 copied areas), P29-02 (live dual-group authority at both ck and cy — non-creator target admins admitted by chat_settings_access must hold BOTH can_change_info AND can_restrict_members in the target group, with uniform opaque settings_pick_unavailable refusal), P29-03 (blacklist action/duration copied to the target's existing blacklist words only without copying or deleting words), and P29-04 (warns and antiflood tuning copied while preserving target warn_limit and inserting flood_limit = 0 via antiflood.SetFloodActionPreserveOffContext when target has no row so antiflood is never accidentally enabled)."
    why_human: "Discuss phase was skipped via workflow.skip_discuss=true, so owner-noticeable scope and authority choices require explicit owner confirmation."
---

# Phase 29: Plain Tone and Copy Settings Verification Report

**Phase Goal:** As the owner, I can make the bot's default texts neutral, and I can copy one group's messages and tuning to another group I own.
**Verified:** 2026-11-23T15:58:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### ROADMAP Success Criteria Rollup

| # | Roadmap Success Criterion | Status | Evidence |
| - | ------------------------- | ------ | -------- |
| SC-1 | The Messages page has a plain-tone switch, off by default. With it on, the bot uses neutral default texts for this milestone's events in the group's language (all 8 languages). Custom messages are untouched. | ✓ VERIFIED | `fuku/modules/settings_messages.go:381-383,1171-1181` registers `settingsActMsgPlainTone = "ht"` (`Right: settingsRightChangeInfo`, private-only `c`, `v` in `0\|1`, hub row widths `[2 2 1 1 1 2]`); `fuku/modules/event_message_plain.go` defines pure builders for all 23 group events and `fuku/modules/event_message_pm.go:79-105` registers `PlainKey` for all 18 PM copy specs across all 8 locales (`en`, `es`, `fr`, `hi`, `id`, `pt`, `ru`, `uk`). `postEventSent` (`event_message.go:460-464`) swaps `r.Default` only when `msgs.Options.PlainTone && r.PlainDefault != nil`, so custom text (`row.Text != ""`) overrides Plain tone. Verified by `TestPlainToneEveryEventEveryLocale`, `TestPlainToneRuntime` (6/6 subtests), `TestSettingsMessagesPMOptions`, and `TestPhase29EndToEnd`. |
| SC-2 | From a group's settings, the owner picks another group and sees a card that names exactly what will be overwritten. After Yes, the target group has the same custom messages and tuning values. | ✓ VERIFIED | `fuku/modules/settings_copy.go:109-134,509-594` registers private-only page `cp` (`Order: 82`, `Icon: "📋"`) with paginated target picker (`cg`), target selection (`ck`) drawing `renderSettingsCopyConfirm` (`settings_copy_confirm_head`, `settings_copy_confirm_areas` naming all 6 overwritten areas, `settings_copy_confirm_keep` naming excluded items and one-time non-linking note), and confirmed `cy` invoking `applyCopySettings` (`copy_settings.go:179-247`) across all 9 tables in scope (`chat_event_messages`, `chat_event_message_options`, `warns_settings`, `antiflood_settings`, `blacklists`, `chat_escalation`, `chat_manual_defaults`, `chat_reason_shortcuts`, `chat_advanced_thresholds`). Verified by `TestCopySettingsMirrorAndExclusions`, `TestSettingsCopyPage/confirm_card_names_all_areas_and_executes_copy`, and `TestPhase29EndToEnd`. |
| SC-3 | The person must be allowed to change settings in both groups, checked live. Otherwise the copy is refused, and the refusal does not reveal anything about the other group. | ✓ VERIFIED | `settingsCopyVerifySource` and `settingsCopyVerifyTarget` (`settings_copy.go:323-390`) run on BOTH `ck` (ask) and `cy` (Yes) after `verify()` checks the source group: live `lockdownLiveMember`, `settingsGate`, P29-02 dual rights (`CanChangeInfo && CanRestrictMembers`) for non-creator target admins, live `GetChatWithContext` (`group`/`supergroup` only), and fresh `staff.GetStaffGroupFresh` (failing closed on DB error). Every target refusal maps to `copyTargetUnavailable` -> uniform opaque alert `settings_pick_unavailable`. Verified across all 16 `ck`/`cy` refusal subtests in `TestSettingsCopyPage/live_dual_group_auth_and_uniform_opaque_refusal` and `TestPhase29StaffPrivacyAndLogs`. |
| SC-4 | Copying happens once. Nothing links the groups afterwards, the source is unchanged, and a stale or replayed card changes nothing. | ✓ VERIFIED | `settingsCopyPutToken` stores a 96-bit 16-char base64url token under `fuku:settings:copy:<token>` (`SET NX`, 5m TTL `settingsCardLifetime`) bound to `{"u","s","t"}`; `settingsCopyRunYes` peeks to verify user/source binding without burning the owner's token on cross-user/source presses, re-verifies both groups live, and atomically consumes the token via Lua `GET+DEL` (`settingsCopyConsumeScript`, `settings_copy.go:75-80,233-252,444-486`) before running one-shot `applyCopySettings`. Verified by `TestSettingsCopyPage/replay_expired_wrong_user_wrong_source_and_no_redis`, `TestCopySettingsMirrorAndExclusions`, and `TestPhase29EndToEnd`. |

### Observable Truths

| # | Truth | Status | Evidence |
| - | ----- | ------ | -------- |
| 1 | (29-01) `TestLegacyPhase29Baseline` records on unchanged commit `ca060519` the exact tone-off default outputs for all 23 milestone v1.2 group events and all 18 PM copy specs in English, verifying that a group with no `chat_event_message_options` row reads default options and sends unchanged default texts byte for byte (MSG-11, TUNE-08). | ✓ VERIFIED | `fuku/modules/phase29_legacy_request_test.go:31-76,104-280` pins all 23 `goldenP29GroupDefaults` and 18 `goldenP29PMDefaults` plus unconfigured `postEventSent`; passes under `-tags testtools -race -count=1`. |
| 2 | (29-01) The golden also pins the uncopied-group baseline: reading a configured source group's messages and tuning creates 0 rows in a target group and leaves an unconfigured target's `antiflood_settings` (`Limit == 0`), `warns_settings`, `blacklists`, `chat_escalation`, `chat_manual_defaults`, `chat_reason_shortcuts`, `chat_advanced_thresholds`, `approvals`, and Staff Group links completely untouched (MSG-11, TUNE-09). | ✓ VERIFIED | `fuku/modules/phase29_legacy_request_test.go:282-459` (`uncopied_group_isolation_baseline`) passes under `-race`. |
| 3 | (29-01) The golden file is committed alone in one `test(29-01)` commit (`b4e8da48`) before any commit of Phase 29 touches production code, and no later plan edits, weakens, or re-records it or any pre-existing golden file. | ✓ VERIFIED | `git log --oneline ca060519..HEAD -- fuku/modules/phase29_legacy_request_test.go` shows exactly 1 commit (`b4e8da48`); `git show --stat b4e8da48` shows 1 file changed (459 insertions); `git log ca060519..b4e8da48 -- fuku migrations locales ':!*_test.go'` is empty; `git diff --name-only ca060519..HEAD -- 'fuku/**/*_legacy_request_test.go'` lists only `fuku/modules/phase29_legacy_request_test.go`. |
| 4 | (29-02) Migration `20261123130000_add_event_message_plain_tone.sql` idempotently adds `plain_tone BOOLEAN NOT NULL DEFAULT FALSE` to `chat_event_message_options` with no transaction control, verified by statement and PostgreSQL integrity tests. | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | DDL inspected (`migrations/20261123130000_add_event_message_plain_tone.sql:11-12`), `TestEventMessagePlainToneMigrationStatements` passes offline, and `TestEventMessagePlainToneMigrationDefaults` is wired in `Makefile:77`, but PostgreSQL execution skipped locally (`DATABASE_URL` unset) — see Human Verification. |
| 5 | (29-02) `eventmsg.Options` carries `PlainTone bool`, `eventmsg.GetFresh` reads it, `eventmsg.SetPlainTone` writes only `plain_tone` (+ `updated_by`, `updated_at`) without clobbering `pm_details` or `pm_log`, drops `event_messages:<chat>` (already in `skipLocal`), and `TestEventMessagesStayOutOfBackup` proves `plain_tone` stays out of backups until Phase 30. | ✓ VERIFIED | `fuku/db/models/event_message.go:45`, `fuku/db/eventmsg/repository.go:49,145,226-233`; verified by `TestPlainToneOption` (`fuku/db/eventmsg/repository_test.go`) and `TestEventMessagesStayOutOfBackup` (`fuku/db/backup/event_messages_backup_test.go`). |
| 6 | (29-02) `eventReply` carries lazy `PlainDefault func() string` (never called when `msgs.Options.PlainTone` is `false` or on read error), `eventPMSpec` carries `PlainKey string`, and `event_message_plain.go` defines pure builders referencing ZERO `eventEngineEntries` so `TestEventEngineCallers` and `TestPMCopyCallers` stay untouched. | ✓ VERIFIED | `event_message.go:114,460-464`, `event_message_pm.go:42,396-445`, `event_message_plain.go:1-308`; `TestEventEngineCallers`, `TestPMCopyCallers`, and `TestPlainToneRuntime/plain_tone_off_matches_default` all pass under `-race`. |
| 7 | (29-02) The Messages hub exposes the Plain tone switch (`ht`, `settingsRightChangeInfo`, private-only `c`, absolute `v` in `0\|1`) with hub row widths `[2 2 1 1 1 2]`, and turning Plain tone on makes `/ban` group reply, ban PM copy, and Messages ban preview emit neutral plain-tone text end-to-end while custom ban text overrides Plain tone and tone off stays byte-identical to `TestLegacyPhase29Baseline`. | ✓ VERIFIED | `settings_messages.go:381-383,605-607,716-718,822-831,1015-1019,1171-1181`; verified across all 7 `TestSettingsMessages*` suites and `TestPhase29EndToEnd`. |
| 8 | (29-03) Every one of the 23 milestone v1.2 group events and every one of the 18 PM copy specs has a neutral, emoji-free, factual plain-tone default text in all 8 languages (`en`, `es`, `fr`, `hi`, `id`, `pt`, `ru`, `uk`). | ✓ VERIFIED | All 8 locale files define 53 `eventmsg_plain_*` keys (35 group templates + 18 PM templates); verified by `TestEventMsgLocaleKeys`, `TestLocaleStyle`, `TestLocalePlaceholdersMatchOtherLanguages`, and `make check-translations` (1069/1069 keys). |
| 9 | (29-03) `registerEventPM` panics at init if `spec.PlainKey == ""` so no PM-capable event can ever ship without a plain-tone default, and `TestEventMsgLocaleKeys` verifies ceilings, balanced `<b>`-only markup, placeholder equality across all 8 languages, and `<b>{group}</b>` in every `eventmsg_plain_pm_*` key. | ✓ VERIFIED | `event_message_pm.go:61-63`, `fuku/i18n/eventmsg_locale_test.go`; verified by `TestEventPMRegister` and `TestEventMsgLocaleKeys`. |
| 10 | (29-03) Every event send site and every Messages preview entry in `settingsMsgFamilies` populates `PlainDefault` via `event_message_plain.go`, preserving all appended technical notes (`mutes_unmute_lockdown_note`, `memberReqNoticeRemedies`, `lockdownStartNoteLines`, `lockdownLiftNoteLines`) and all bot buttons (Unban, Unmute, Remove warn, Rules, 'I fixed it', Appeal) while keeping `event_message_plain.go` free of any `eventEngineEntries` reference. | ✓ VERIFIED | Wired across `bans.go`, `mute.go`, `warns.go`, `event_message_auto.go`, `antiflood.go`, `aispam_notice.go`, `memberreq_notice.go`, `event_message_lockdown.go`, and all 23 entries in `settings_messages.go:150-284`. |
| 11 | (29-03) `TestPlainToneEveryEventEveryLocale` renders all 23 group events and all 18 PM events in BOTH tones across ALL 8 languages, asserting non-empty HTML-safe output, zero unexpanded `{placeholder}` tokens, identical filled values/placeholders between tones, and Telegram UTF-16 length limits (`<= 4096`, and `<= 520` for single-rule requirement notices). | ✓ VERIFIED | `fuku/modules/event_message_plain_test.go:46-466` passes under `-race`. |
| 12 | (29-03) `TestPlainToneRuntime` proves custom text overrides Plain tone, custom media/buttons with empty text compose with the plain default text, Telegram refusal of a custom message falls back to the plain default, forward blocklist plain messages never reveal source/Staff Group, and tone off remains byte-identical to `TestLegacyPhase29Baseline`. | ✓ VERIFIED | `fuku/modules/event_message_plain_test.go:468-742` passes all 6 subtests under `-race`. |
| 13 | (29-04) `fuku/db/eventmsg/validate_write.go` provides one shared validated message write function (`ValidateMessage` + `ReplaceAllValidated` returning `WriteReport{Written, Skipped}`) in package `eventmsg` (zero `fuku/modules` imports, reusable by Phase 30 `MSG-10`), enforcing `ValidEvent`/`ValidDest`/`HasPM`, media kind & `NoMedia`, `content.ValidButtonURL`, 100-button & 8-per-row caps with event button reserves, 4096/1024 text/caption rune & UTF-16 limits with event text reserves (`600`/`400`), `NoSilent` normalization, single-transaction replace, and `cache.DeleteCache`. | ✓ VERIFIED | `fuku/db/eventmsg/events.go:164-237`, `fuku/db/eventmsg/validate_write.go:25-234`; verified by `TestEventMsgValidateAndReplaceAll` (`fuku/db/eventmsg/validate_write_test.go`). |
| 14 | (29-04) `TestEventMsgValidateWriteMatchesSettingsPage` proves `eventmsg.ReservedButtonsFor`, `eventmsg.ReservedTextFor`, `eventmsg.HasPM`, `eventmsg.NoMedia`, and `eventmsg.NoSilent` stay 1-to-1 with `settingsMsgFamilies` and `eventPMSpecs` so the editor, Copy Settings, and Phase 30 backup import can never drift. | ✓ VERIFIED | `fuku/modules/copy_settings_test.go:65-148` passes under `-race`. |
| 15 | (29-04) `antiflood.SetFloodActionPreserveOffContext` writes `action` and `action_secs` without triggering GORM's `default:5` tag on `flood_limit`, so copying to a target group with no `antiflood_settings` row leaves `Limit == 0` (antiflood OFF) instead of silently turning antiflood on (P29-04). | ✓ VERIFIED | `fuku/db/antiflood/repository.go:122-139` uses explicit raw SQL `INSERT ... VALUES (?, 0, ...) ON CONFLICT (chat_id) DO UPDATE`; verified by `TestSetFloodActionPreserveOffAndGetFresh`, `TestCopySettingsMirrorAndExclusions`, and `TestPhase29EndToEnd`. |
| 16 | (29-04) `applyCopySettings` in `fuku/modules/copy_settings.go` mirrors all 6 named areas across 9 tables per P29-01/P29-03/P29-04, drops every target cache key, leaves the source completely unchanged, creates no ongoing link, and NEVER touches excluded tables (Staff Group links, approvals, `warns_users`, `chat_offences`, lockdowns, blacklist words, greetings, rules, notes, filters, locks, `chat_settings_access`). | ✓ VERIFIED | `fuku/modules/copy_settings.go:87-247`; verified by `TestCopySettingsMirrorAndExclusions` (3/3 subtests) and `TestPhase29EndToEnd`. |
| 17 | (29-05) Private-only Copy Settings page `cp` (Order `82`, Icon `📋`, `PrivateOnly: true`, hidden in Staff Groups via fresh `staff.GetStaffGroupFresh`, static zero-I/O `MainState`) is registered with action codes `cg` (page), `ck` (pick target), and `cy` (confirmed Yes), all requiring private owner field `c` and refusing forged or group-menu presses in step 1 with 0 `getChatMember` calls; Main/Phase-19 grid tests updated. | ✓ VERIFIED | `fuku/modules/settings_copy.go:105-169,255-263`; verified by `TestSettingsCopyPage`, `TestPhase29Codes`, `TestSettingsMainGrid`, `TestSettingsMainPhase18`, `TestSettingsMainPhase19`, `TestSettingsFwdBlockMainShape`, and `TestSettingsPhase19Codes`. |
| 18 | (29-05) Both source and target groups are verified live at ask (`ck`) AND again at Yes (`cy`) (`lockdownLiveMember`, `settingsGate`, live `GetChatWithContext` group/supergroup, fresh non-Staff-Group check failing closed on DB error, and P29-02 `can_change_info && can_restrict_members` for non-creator target admins); every target refusal returns uniform opaque `settings_pick_unavailable`. | ✓ VERIFIED | `fuku/modules/settings_copy.go:323-399,419-427,462-470`; verified across all 16 subtests of `TestSettingsCopyPage/live_dual_group_auth_and_uniform_opaque_refusal`. |
| 19 | (29-05) The two-tap confirm card drawn by `ck` binds `(userID, sourceChatID, targetChatID)` to a 16-char base64url one-time Redis token under `fuku:settings:copy:<token>` (TTL 5m, `SET NX`, failing closed when Redis is unavailable), names both escaped group titles, all 6 overwritten areas, excluded items, and one-time non-linking note; `cy` atomically consumes the token via Lua `GET+DEL`; all 8 locales pass 64-byte callback and UTF-16 screen length checks. | ✓ VERIFIED | `fuku/modules/settings_copy.go:75-80,175-252,428-506,563-594`; verified by `TestSettingsCopyPage` (`confirm_card_names_all_areas_and_executes_copy`, `replay_expired_wrong_user_wrong_source_and_no_redis`, `worst_case_64_byte_callbacks`, `all_8_languages_screen_lengths`). |
| 20 | (29-06) `TestPhase29Codes`, `TestPhase29ReadsCreateNothing`, and `TestPhase29StaffPrivacyAndLogs` verify all 29 registered pages, zero action-code collisions, step-1 refusal with 0 `getChatMember` calls, 0 rows created across all 10 tables when opening Main/`h`/`cp`/`cg`, Staff Group privacy, and zero leakage of group titles, user names, message texts, file IDs, button/appeal URLs, shortcut reasons, or copy tokens in logs. | ✓ VERIFIED | `fuku/modules/settings_phase29_test.go:44-475` passes all 3 safeguard suites under `-race`. |
| 21 | (29-06) `TestPhase29EndToEnd` drives the real `ext.Dispatcher` end-to-end with in-process `miniredis` and fake Telegram HTTP across Plain tone (`ht` -> live `/ban` group & PM plain copy -> custom text precedence -> `/kick` plain tone) and Copy Settings (`cp` -> `ck` -> `cy` -> live target `/kick sp` and `/ban sp` with copied Plain tone, custom ban text, reason required, expanded shortcut, appeal button, preserved `antiflood_settings.Limit == 0`, preserved target blacklist words/excluded tables, non-linking, and `cy` replay refusal). | ✓ VERIFIED | `fuku/modules/phase29_e2e_test.go:32-475` passes under `-race`. |
| 22 | (29-06) `AGENTS.md` documents the Phase 29 migration, `plain_tone` column, `eventmsg.ReplaceAllValidated` (shared with Phase 30), `antiflood.SetFloodActionPreserveOffContext`, Redis key family `fuku:settings:copy:<token>`, Messages toggle `ht`, page `cp` (Order 82, codes `cg`/`ck`/`cy`), zero new shutdown drains / handler groups, provisional decisions `P29-01..P29-04`, and Phase 29 test suites; all local phase gates pass and Wave 1 + pre-existing goldens + `go.mod`/`go.sum` are strictly immutable. | ✓ VERIFIED | `AGENTS.md:7108-7138` (plus schema/Redis/page sections) verified; all 5 static/build/shard gates and all test suites exited `0`; `b4e8da48` is the single commit on `phase29_legacy_request_test.go`. |

**Score:** 21/22 truths verified (1 present, behavior-unverified: PostgreSQL migration default execution)

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `fuku/modules/phase29_legacy_request_test.go` | `TestLegacyPhase29Baseline` pinning all 23 tone-off group defaults, 18 PM defaults, and uncopied-group isolation | ✓ VERIFIED | Exists (459 lines, 4 subtests), committed alone in `b4e8da48` before any Phase 29 production commit, untouched since, passes under `-race`. |
| `migrations/20261123130000_add_event_message_plain_tone.sql` | Idempotent `ALTER TABLE chat_event_message_options ADD COLUMN IF NOT EXISTS plain_tone BOOLEAN NOT NULL DEFAULT FALSE` | ✓ VERIFIED | Exists (12 lines, 1 statement, no transaction control); verified offline by `TestEventMessagePlainToneMigrationStatements`; PostgreSQL execution deferred to CI (`behavior_unverified`). |
| `fuku/db/models/event_message.go` & `fuku/db/eventmsg/repository.go` | `PlainTone` on `ChatEventMessageOptions`/`Options`, `GetFresh`, column-targeted `SetPlainTone` dropping `event_messages:<chat>` | ✓ VERIFIED | Exists and wired; verified by `TestPlainToneOption` and `TestEventMessagesStayOutOfBackup`. |
| `fuku/modules/event_message_plain.go` | All 23 pure group plain-tone builders + `eventPlainRender` + `eventPlainReasonLine` with zero `eventEngineEntries` references | ✓ VERIFIED | Exists (308 lines); `TestEventEngineCallers` and `TestPMCopyCallers` pass with zero allow-list modifications. |
| `fuku/modules/event_message.go`, `event_message_pm.go`, `event_message_apply.go` | Lazy `eventReply.PlainDefault`, required `eventPMSpec.PlainKey`, `plain bool` composer parameter, `applyEventPlainTone` | ✓ VERIFIED | All 18 PM specs carry `PlainKey` (init panic on empty); `postEventSent` evaluates `PlainDefault` only when `msgs.Options.PlainTone` is true. |
| `fuku/modules/{bans,mute,warns,event_message_auto,antiflood,aispam_notice,memberreq_notice,event_message_lockdown,settings_messages}.go` | Lazy `PlainDefault` closures at every send site, all 23 `settingsMsgFamilies` entries, and Messages hub toggle `ht` | ✓ VERIFIED | All 23 events wired at both live send sites and Messages previews; hub row widths `[2 2 1 1 1 2]`. |
| `fuku/modules/event_message_plain_test.go` | `TestPlainToneEveryEventEveryLocale` and `TestPlainToneRuntime` | ✓ VERIFIED | Exists (742 lines), renders all 23 group + 18 PM events across both tones and 8 locales, plus 6 runtime precedence/fallback/privacy subtests. |
| `fuku/db/eventmsg/events.go`, `validate_write.go`, `validate_write_test.go` | Shared `HasPM`, `NoMedia`, `NoSilent`, `ReservedButtonsFor`, `ReservedTextFor`, `ValidateMessage`, `ReplaceAllValidated` | ✓ VERIFIED | Exists (234 lines in `validate_write.go`), zero `fuku/modules` imports (ready for Phase 30 `MSG-10`), drift-guarded by `TestEventMsgValidateWriteMatchesSettingsPage`. |
| `fuku/db/{antiflood,escalation,modtuning,advthresholds}/repository.go` | `GetFloodFreshContext`, `SetFloodActionPreserveOffContext`, `ReplaceLadder`, `ReplaceDefaults`, `ReplaceShortcuts`, `ReplaceAll` | ✓ VERIFIED | All 4 repositories provide validated atomic replace helpers + cache invalidation; `SetFloodActionPreserveOffContext` inserts `flood_limit = 0` on missing target rows. |
| `fuku/modules/copy_settings.go` & `copy_settings_test.go` | `applyCopySettings` mirroring all 6 areas across 9 tables + `TestEventMsgValidateWriteMatchesSettingsPage` & `TestCopySettingsMirrorAndExclusions` | ✓ VERIFIED | Exists (247 lines + 632 lines of tests), passes 3/3 mirror/clear/skip subtests and drift guard under `-race`. |
| `fuku/modules/settings_copy.go` & `settings_copy_test.go` | Private Copy Settings page `cp` (Order `82`, `cg`/`ck`/`cy`), live dual-group auth, uniform opaque `settings_pick_unavailable`, Redis Lua `GET+DEL` token | ✓ VERIFIED | Exists (594 lines + 885 lines of tests), passes 6/6 subtests (including 16 dual-group auth/opacity cases and 8-locale screen lengths) under `-race`. |
| `locales/{en,es,fr,hi,id,pt,ru,uk}.yml` & `fuku/i18n/{eventmsg,settings}_locale_test.go` | All 53 `eventmsg_plain_*` keys + 4 Plain tone UI keys + 12 Copy Settings UI keys across all 8 languages | ✓ VERIFIED | 1069/1069 keys across all 8 locales; all i18n ceiling, label-length, placeholder, and style tests pass. |
| `fuku/modules/settings_phase29_test.go`, `phase29_e2e_test.go`, `AGENTS.md` | `TestPhase29Codes`, `TestPhase29ReadsCreateNothing`, `TestPhase29StaffPrivacyAndLogs`, `TestPhase29EndToEnd`, and rulebook updates (`P29-01..P29-04`) | ✓ VERIFIED | Exists (475 + 475 lines), passes under `-race` with zero localhost TCP dials; `AGENTS.md` updated completely. |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | -- | --- | ------ | ------- |
| `fuku/modules/settings_messages.go` (`settingsActMsgPlainTone` / `ht`) | `fuku/modules/event_message_apply.go` -> `fuku/db/eventmsg/repository.go` | `settingsMsgRunPlainTone` -> `applyEventPlainTone` -> `eventmsg.SetPlainTone` -> `cache.DeleteCache("event_messages:<chat>")` | ✓ WIRED | Confirmed in `settings_messages.go:381-383,822-831`, `event_message_apply.go:21,29,179-182`, `repository.go:226-233`. |
| `fuku/modules/event_message.go` (`postEventSent`) & `event_message_pm.go` (`offerReplyPM`) | `eventReply.PlainDefault` (`event_message_plain.go`) & `eventPMSpec.PlainKey` | `if msgs.Options.PlainTone && r.PlainDefault != nil` swap before `postEventGroup`; `msgs.Options.PlainTone` passed to `composeEventPM`/`eventPMDefaultText` | ✓ WIRED | Confirmed in `event_message.go:460-466` and `event_message_pm.go:396-445`; all 23 send sites and 23 preview entries set `PlainDefault`, all 18 PM specs set `PlainKey`. |
| `fuku/modules/settings_copy.go` (`ck` / `cy`) | `fuku/modules/settings_menu.go`, `settings_gate.go`, `fuku/db/staff` | `settingsCopyVerifySource` + `settingsCopyVerifyTarget` (`lockdownLiveMember`, `settingsGate`, P29-02 `CanChangeInfo && CanRestrictMembers`, `GetChatWithContext`, `staff.GetStaffGroupFresh`) | ✓ WIRED | Confirmed in `settings_copy.go:323-399,419-427,462-470`; runs on both `ck` and `cy`; all target refusals return `settings_pick_unavailable`. |
| `fuku/modules/settings_copy.go` (`cy`) | Redis `fuku:settings:copy:<token>` + `fuku/modules/copy_settings.go` (`applyCopySettings`) | `settingsCopyPeekToken` bind check -> live dual-group re-verify -> Lua `settingsCopyConsumeScript` (`GET+DEL`) -> `applyCopySettings` | ✓ WIRED | Confirmed in `settings_copy.go:75-80,212-252,444-506`. |
| `fuku/modules/copy_settings.go` (`applyCopySettings`) | `fuku/db/eventmsg/validate_write.go` + `warns` + `antiflood` + `blacklists` + `escalation` + `modtuning` + `advthresholds` | `readCopySettingsSnapshot` (fresh non-creating reads) -> `eventmsg.ReplaceAllValidated`, `warns.SetWarnModeContext`/`SetWarnExpiryContext`, `antiflood.SetFloodActionPreserveOffContext`, `blacklists.SetBlacklistActionTimedContext`, `escalation.ReplaceLadder`, `modtuning.ReplaceDefaults`/`ReplaceShortcuts`, `advthresholds.ReplaceAll` | ✓ WIRED | Confirmed in `copy_settings.go:87-247`; drops every target cache key after commit. |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| -------- | ------------- | ------ | ------------------ | ------ |
| `fuku/modules/settings_messages.go` | `msgs.Options.PlainTone` on hub, Main state, and `settingsEditState.PlainTone` in previews | `settingsMsgReadNow` -> `eventmsg.GetFresh` (`chat_event_message_options.plain_tone`) | Yes (real DB query, `false` when row absent) | ✓ FLOWING |
| `fuku/modules/event_message.go` & `event_message_pm.go` | `msgs.Options.PlainTone` on live moderation/screening/lockdown events | `eventMessagesLookup` -> `eventmsg.Get` (`skipLocal` Redis cache backed by `GetFresh`) | Yes (drives live `r.PlainDefault()` evaluation and `spec.PlainKey` selection) | ✓ FLOWING |
| `fuku/modules/settings_copy.go` | Picker candidate groups (`[]adminindex.ListedGroup`) and confirm card token (`settingsCopyTokenRecord`) | `settingsCopyLoadCandidates` (`adminindex` + `staff.GetStaffGroupFresh`) and Redis `fuku:settings:copy:<token>` (`SET NX` / Lua `GET+DEL`) | Yes (real roster query and atomic Redis token lifecycle) | ✓ FLOWING |
| `fuku/modules/copy_settings.go` | `copySettingsSnapshot` (all 6 areas) written to `dstChatID` | Fresh DB reads on `srcChatID` -> validated transactional/upsert writes on `dstChatID` across 9 tables | Yes (real cross-group DB mirror verified by live dispatcher execution on `dstChat`) | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Static, doc, vet, build, and shard gates | `make check-translations && make check-docs && go vet -tags testtools ./... && CGO_ENABLED=0 go build ./... && make test-shard-check` | 1069/1069 translation keys across 8 locales, 0 doc drift, vet/build/shard-check exit `0` | ✓ PASS |
| MSG-11 golden immutability & dependency immutability | `git log --oneline ca060519..HEAD -- fuku/modules/phase29_legacy_request_test.go`, `git diff --exit-code ca060519..HEAD -- go.mod go.sum`, `git diff --name-only ca060519..HEAD -- 'fuku/**/*_legacy_request_test.go'` | Exactly 1 commit (`b4e8da48`, 1 file changed, 0 production commits before it); `go.mod`/`go.sum` and all pre-existing goldens untouched | ✓ PASS |
| Phase 29 DB repositories, migration statements, backup exclusion, and i18n guards | `TZ=UTC go test -tags testtools -race -count=1 ./fuku/db/eventmsg ./fuku/db/antiflood ./fuku/db/escalation ./fuku/db/modtuning ./fuku/db/advthresholds` + `migrations` + `backup` + `i18n` | All PASS (`TestEventMessagePlainToneMigrationDefaults` cleanly skipped: `DATABASE_URL` unset) | ✓ PASS |
| Phase 29 core, golden, plain-tone, copy-settings, safeguards, E2E, and EVT-10 caller guards | `TZ=UTC go test -tags testtools -race -count=1 -run '^(TestLegacyPhase29Baseline\|TestPlainToneEveryEventEveryLocale\|TestPlainToneRuntime\|TestEventMsgValidateWriteMatchesSettingsPage\|TestCopySettingsMirrorAndExclusions\|TestSettingsCopyPage\|TestPhase29Codes\|TestPhase29ReadsCreateNothing\|TestPhase29StaffPrivacyAndLogs\|TestPhase29EndToEnd\|TestEventEngineCallers\|TestPMCopyCallers\|TestMemberDeletionExcludedMessages)$' -v ./fuku/modules` | 13/13 top-level suites PASS under `-race` (4.30 s, 0 failures, 0 skips) | ✓ PASS |
| Messages hub, Main grid, Phase 18/19/28 codes, and Staff Group privacy regressions | `TZ=UTC go test -tags testtools -race -count=1 -run '^(TestSettingsMessages\|TestSettingsMessagesAuto\|TestSettingsMessagesLockdown\|TestSettingsMessagesPMOptions\|TestSettingsMessagesScreening\|TestSettingsMessagesScreens\|TestSettingsMessagesSafety\|TestSettingsMainGrid\|TestSettingsMainPhase18\|TestSettingsMainPhase19\|TestSettingsMainPageFitsEveryLocale\|TestSettingsFwdBlockMainShape\|TestSettingsPhase19Codes\|TestPhase28Codes\|TestSettingsStaffPrivacy)$' ./fuku/modules` | All 15 regression suites PASS under `-race` (12.14 s) | ✓ PASS |
| PostgreSQL migration default execution | `make test-postgres-integrity` (`TestEventMessagePlainToneMigrationDefaults`) | Skipped locally (`DATABASE_URL` unset) | ? SKIP |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ----------- | ----------- | ------ | -------- |
| `TUNE-08` | 29-01, 29-02, 29-03, 29-06 | A plain-tone switch per group replaces the bot's themed default texts with neutral ones for the events of this milestone, in all 8 languages. | ✓ SATISFIED (code/tests verified; PostgreSQL migration run, CI lint/gosec, live Telegram UAT & P29-01..04 sign-off pending human verification) | `plain_tone` migration/model/repo, Messages hub toggle `ht`, pure plain builders for all 23 group events and 18 PM specs across all 8 languages, custom message precedence, and E2E proof complete; marked Complete in `.planning/REQUIREMENTS.md:58,124`. |
| `TUNE-09` | 29-01, 29-04, 29-05, 29-06 | Copy settings: from a group's settings the owner copies the custom messages and tuning values of another group they own, after a confirmation that says what will be overwritten. The person must be allowed to change settings in both groups, checked live. | ✓ SATISFIED (code/tests verified; CI lint/gosec, live Telegram UAT & P29-01..04 sign-off pending human verification) | Shared `eventmsg.ReplaceAllValidated`, safe tuning replace helpers (`SetFloodActionPreserveOffContext`, `ReplaceLadder`, `ReplaceDefaults`, `ReplaceShortcuts`, `ReplaceAll`), `applyCopySettings` across all 6 areas / 9 tables, private page `cp` (`cg`/`ck`/`cy`) with live dual-group verification at ask and Yes, uniform opaque `settings_pick_unavailable`, and pair-bound Lua-consumed Redis confirm cards complete; marked Complete in `.planning/REQUIREMENTS.md:59,125`. |
| `MSG-11` (cross-cutting) | 29-01..29-06 | Every existing reply and notice is byte-identical for a group with no custom message (and Plain tone off / uncopied), proved by characterization tests recorded before the send sites are changed. | ✓ SATISFIED | `TestLegacyPhase29Baseline` committed alone in `b4e8da48` on unchanged `ca060519` before any Phase 29 production edit, untouched since, passes 4/4 subtests under `-race`. |
| `EVT-10` (cross-cutting) | 29-02..29-06 | Staff Group actions, lockdown alerts, raid-scale removals, log channel posts and captcha challenges are not events: their texts and behaviour are unchanged; no Staff Group defaults or inheritance. | ✓ SATISFIED | `TestEventEngineCallers`, `TestPMCopyCallers`, `TestMemberDeletionExcludedMessages`, `TestPhase29StaffPrivacyAndLogs`, and `TestSettingsStaffPrivacy` all pass under `-race`; page `cp` is hidden in Staff Groups and refuses Staff Group sources/targets opaquely. |

### Anti-Patterns Found

None. Zero `TBD`/`FIXME`/`XXX` debt markers, zero new `TODO`/`HACK`/`PLACEHOLDER` comments in Phase 29 code, zero stub implementations, zero `eventEngineEntries` references in `event_message_plain.go` or `copy_settings.go`, and zero leakage of group titles, user names, message texts, file IDs, URLs, or Redis copy tokens into logs.

### Human Verification Required

### 1. PostgreSQL Integrity Tests

**Test:** Run `FUKU_TEST_DATABASE=true DATABASE_URL=... make test-postgres-integrity` (specifically `TestEventMessagePlainToneMigrationDefaults`).
**Expected:** Migration `20261123130000_add_event_message_plain_tone.sql` runs idempotently twice inside one transaction on PostgreSQL, and inserting a row into `chat_event_message_options` with only `chat_id` defaults `pm_details`, `pm_log`, and `plain_tone` to `FALSE`.
**Why human:** No PostgreSQL server is available in this local environment (`DATABASE_URL` unset), so `TestEventMessagePlainToneMigrationDefaults` skipped.

### 2. CI Static Checks (`make lint` and `gosec`)

**Test:** Run `make lint` and `gosec` on this tree in CI or a host with both tools installed.
**Expected:** Both `golangci-lint` and `gosec` exit `0` with zero new findings on Phase 29 files.
**Why human:** Neither `golangci-lint` nor `gosec` is installed in this local environment.

### 3. Live Telegram UAT

**Test:** In two real supergroups + the bot's private chat:
1. Open private `/settings` → Messages (`h`) for Source Group, toggle Plain tone ON (`ht`), confirm Main shows `Plain tone` when no custom messages are set, trigger `/ban` and `/kick` (with PM copy ON) and confirm neutral, emoji-free third-person default texts in both the group and the punished member's PM; set a custom `/ban` text and confirm custom text overrides Plain tone while `/kick` continues using Plain tone.
2. Open private `/settings` → Copy settings (`cp`) for Source Group (confirm group `/settings` shows `Private only` and opening `cp` in a group shows `settings_copy_group_note`, and Staff Group Main hides `cp`), pick Target Group (`ck`), verify the confirmation card names both group titles, all 6 overwritten areas, excluded items, and the one-time non-linking notice, tap Yes (`cy`), and verify Target Group mirrors all 6 areas while preserving Target Group's antiflood-off state (`Limit == 0`), existing blacklist word list, `warn_limit`, approvals, and Staff Group links, and that tapping an old/replayed card is refused with `settings_card_expired`.
**Expected:** Every screen, toggle, and two-tap confirmation card renders cleanly in Telegram, Plain tone replaces only uncustomized default messages across group and PM, and Copy Settings mirrors all 6 areas once with opaque target refusals and atomic replay protection.
**Why human:** Visual rendering, live Telegram API keyboard acceptance, multi-replica Redis timing, and end-to-end UX cannot be verified offline.

### 4. Owner Confirmation of Provisional Decisions `P29-01`..`P29-04`

**Test:** Review and confirm (or request adjustments to) provisional owner decisions `P29-01` through `P29-04` documented in `AGENTS.md`.
**Expected:** Owner signs off on:
- **P29-01 (Full Mirror on Copy):** Copying overwrites the target group's event messages and moderation tuning in scope to match the source group completely; an empty or default source entry clears the target's custom message, switch, option, escalation ladder, manual default, reason shortcuts, or advanced thresholds across all 6 copied areas (`chat_event_messages`, `chat_event_message_options`, `warns_settings`/`antiflood_settings`/`blacklists` action tuning, `chat_escalation`, `chat_manual_defaults`/`chat_reason_shortcuts`, and `chat_advanced_thresholds`).
- **P29-02 (Authority in Both Groups):** Both source and target groups are verified live at ask (`ck`) and again at confirmed Yes (`cy`) via `lockdownLiveMember` + `settingsGate` + `GetChatWithContext` (`group`/`supergroup` only) + fresh non-Staff-Group check (failing closed on DB error). For non-creator admins admitted by `chat_settings_access`, target verification requires BOTH `can_change_info` AND `can_restrict_members` in the target group; every target-side refusal returns the uniform opaque alert `settings_pick_unavailable`.
- **P29-03 (Blacklist Action Without Copying Words):** Blacklist words themselves are never copied or deleted; only the source group's blacklist action and timed duration (`action`, `action_secs`, or `"warn", 0` when the source has no blacklist words) are applied to the target group's existing blacklist words (if any) via `blacklists.SetBlacklistActionTimedContext`.
- **P29-04 (Warns & Antiflood Tuning Scope):** Copy Settings copies `warns_settings` tuning (`warn_mode`, `warn_mode_secs`, `warn_expiry_days` — preserving the target's `warn_limit` or default 3 and never touching `warns_users`) and `antiflood_settings` tuning (`action`, `action_secs` via `antiflood.SetFloodActionPreserveOffContext` — preserving the target's `flood_limit` and `delete_antiflood_message`, and inserting `flood_limit = 0` when the target has no row so antiflood is never accidentally enabled).
**Why human:** Discuss phase was skipped via `workflow.skip_discuss=true`, so owner-noticeable scope and authority choices require explicit owner confirmation.

### Gaps Summary

No code, test, wiring, or requirement gaps were found. All 21 locally verifiable must-haves, all 4 ROADMAP Success Criteria, `TUNE-08`, `TUNE-09`, MSG-11 golden immutability (`b4e8da48`), EVT-10 caller guards (`TestEventEngineCallers`, `TestPMCopyCallers`, `TestMemberDeletionExcludedMessages`), and all local phase gates (`make check-translations`, `make check-docs`, `go vet -tags testtools ./...`, `CGO_ENABLED=0 go build ./...`, `make test-shard-check`) pass cleanly under `-race`. Overall status is `human_needed` solely for the 1 PostgreSQL-only migration default check (`TestEventMessagePlainToneMigrationDefaults`, skipped locally without `DATABASE_URL`) plus CI `make lint`/`gosec`, live Telegram UAT, and owner confirmation of provisional decisions `P29-01`..`P29-04`.

---

_Verified: 2026-11-23T15:58:00Z_
_Verifier: the agent (gsd-verifier)_
