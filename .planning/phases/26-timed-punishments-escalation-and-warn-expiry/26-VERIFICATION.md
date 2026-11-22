---
phase: 26-timed-punishments-escalation-and-warn-expiry
verified: 2026-11-22T15:19:38Z
status: human_needed
score: 57/62 must-haves verified
covered_files:
  - .planning/phases/26-timed-punishments-escalation-and-warn-expiry/26-01-PLAN.md
  - .planning/phases/26-timed-punishments-escalation-and-warn-expiry/26-01-SUMMARY.md
  - .planning/phases/26-timed-punishments-escalation-and-warn-expiry/26-02-PLAN.md
  - .planning/phases/26-timed-punishments-escalation-and-warn-expiry/26-02-SUMMARY.md
  - .planning/phases/26-timed-punishments-escalation-and-warn-expiry/26-03-PLAN.md
  - .planning/phases/26-timed-punishments-escalation-and-warn-expiry/26-03-SUMMARY.md
  - .planning/phases/26-timed-punishments-escalation-and-warn-expiry/26-04-PLAN.md
  - .planning/phases/26-timed-punishments-escalation-and-warn-expiry/26-04-SUMMARY.md
  - .planning/phases/26-timed-punishments-escalation-and-warn-expiry/26-05-PLAN.md
  - .planning/phases/26-timed-punishments-escalation-and-warn-expiry/26-05-SUMMARY.md
  - .planning/phases/26-timed-punishments-escalation-and-warn-expiry/26-06-PLAN.md
  - .planning/phases/26-timed-punishments-escalation-and-warn-expiry/26-06-SUMMARY.md
  - .planning/phases/26-timed-punishments-escalation-and-warn-expiry/26-07-PLAN.md
  - .planning/phases/26-timed-punishments-escalation-and-warn-expiry/26-07-SUMMARY.md
  - .planning/phases/26-timed-punishments-escalation-and-warn-expiry/26-08-PLAN.md
  - .planning/phases/26-timed-punishments-escalation-and-warn-expiry/26-08-SUMMARY.md
  - .planning/phases/26-timed-punishments-escalation-and-warn-expiry/26-09-PLAN.md
  - .planning/phases/26-timed-punishments-escalation-and-warn-expiry/26-09-SUMMARY.md
  - .planning/phases/26-timed-punishments-escalation-and-warn-expiry/26-10-PLAN.md
  - .planning/phases/26-timed-punishments-escalation-and-warn-expiry/26-10-SUMMARY.md
  - .planning/phases/26-timed-punishments-escalation-and-warn-expiry/26-11-PLAN.md
  - .planning/phases/26-timed-punishments-escalation-and-warn-expiry/26-11-SUMMARY.md
  - .planning/phases/26-timed-punishments-escalation-and-warn-expiry/26-12-PLAN.md
  - .planning/phases/26-timed-punishments-escalation-and-warn-expiry/26-12-SUMMARY.md
  - AGENTS.md
  - Makefile
  - fuku/db/antiflood/optimized.go
  - fuku/db/antiflood/repository.go
  - fuku/db/antiflood/timed_action_test.go
  - fuku/db/backup/backup.go
  - fuku/db/backup/escalation_backup_test.go
  - fuku/db/backup/punish_export_legacy_test.go
  - fuku/db/backup/punish_import_test.go
  - fuku/db/backup/roundtrip_test.go
  - fuku/db/backup/testmain_test.go
  - fuku/db/blacklists/repository.go
  - fuku/db/blacklists/timed_action_test.go
  - fuku/db/cache/local.go
  - fuku/db/cache/local_test.go
  - fuku/db/escalation/repository.go
  - fuku/db/escalation/repository_test.go
  - fuku/db/escalation/testmain_test.go
  - fuku/db/migrations/blacklist_action_check_test.go
  - fuku/db/migrations/escalation_migration_test.go
  - fuku/db/migrations/timed_punishment_migration_test.go
  - fuku/db/migrations/warn_expiry_migration_test.go
  - fuku/db/migrations/warn_mode_check_test.go
  - fuku/db/models/antiflood.go
  - fuku/db/models/blacklists.go
  - fuku/db/models/escalation.go
  - fuku/db/models/punish.go
  - fuku/db/models/warns.go
  - fuku/db/models/warns_counted_test.go
  - fuku/db/warns/expiry_test.go
  - fuku/db/warns/repository.go
  - fuku/db/warns/timed_mode_test.go
  - fuku/i18n/eventmsg_locale_test.go
  - fuku/i18n/settings_locale_test.go
  - fuku/modules/antiflood.go
  - fuku/modules/antiflood_apply_test.go
  - fuku/modules/antiflood_timed_test.go
  - fuku/modules/blacklists.go
  - fuku/modules/blacklists_apply_test.go
  - fuku/modules/blacklists_timed_test.go
  - fuku/modules/escalation.go
  - fuku/modules/escalation_settings.go
  - fuku/modules/escalation_settings_test.go
  - fuku/modules/escalation_test.go
  - fuku/modules/event_message.go
  - fuku/modules/event_message_auto.go
  - fuku/modules/event_message_auto_test.go
  - fuku/modules/event_message_pm.go
  - fuku/modules/event_message_pm_safety_test.go
  - fuku/modules/forwardblock_notice.go
  - fuku/modules/memberreq_enforce.go
  - fuku/modules/memberreq_notice.go
  - fuku/modules/punish_timed.go
  - fuku/modules/punish_timed_test.go
  - fuku/modules/settings_antiflood.go
  - fuku/modules/settings_antiflood_test.go
  - fuku/modules/settings_antiflood_timed_test.go
  - fuku/modules/settings_blacklist.go
  - fuku/modules/settings_blacklist_test.go
  - fuku/modules/settings_blacklist_timed_test.go
  - fuku/modules/settings_duration.go
  - fuku/modules/settings_duration_test.go
  - fuku/modules/settings_escalation.go
  - fuku/modules/settings_escalation_screens_test.go
  - fuku/modules/settings_escalation_test.go
  - fuku/modules/settings_list.go
  - fuku/modules/settings_list_door_test.go
  - fuku/modules/settings_list_privacy_test.go
  - fuku/modules/settings_main_test.go
  - fuku/modules/settings_messages.go
  - fuku/modules/settings_messages_auto_test.go
  - fuku/modules/settings_phase18_test.go
  - fuku/modules/settings_phase19_test.go
  - fuku/modules/settings_phase26_test.go
  - fuku/modules/settings_staff_privacy_test.go
  - fuku/modules/settings_warns.go
  - fuku/modules/settings_warns_expiry_test.go
  - fuku/modules/settings_warns_screens_test.go
  - fuku/modules/settings_warns_test.go
  - fuku/modules/settings_warns_timed_test.go
  - fuku/modules/staff_forwardblock_watch.go
  - fuku/modules/test_harness_test.go
  - fuku/modules/timed_punishment_legacy_request_test.go
  - fuku/modules/timed_punishment_safety_test.go
  - fuku/modules/timed_punishments_e2e_test.go
  - fuku/modules/warns.go
  - fuku/modules/warns_expiry_test.go
  - fuku/modules/warns_settings_apply_test.go
  - fuku/modules/warns_timed_test.go
  - locales/en.yml
  - locales/es.yml
  - locales/fr.yml
  - locales/hi.yml
  - locales/id.yml
  - locales/pt.yml
  - locales/ru.yml
  - locales/uk.yml
  - migrations/20261122120000_add_timed_punishment_durations.sql
  - migrations/20261122130000_add_warn_expiry.sql
  - migrations/20261122140000_add_escalation_ladder.sql
covered_digest: "v4:sha256:2fd8d742c8ddf4029b798fd0df5674d5d0791c38e0cd3a55b569fa73a59d7cbb"
behavior_unverified: 4
overrides_applied: 0
behavior_unverified_items:
  - truth: "Migration 20261122120000 runs twice on PostgreSQL and its checks refuse 31622401 with SQLSTATE 23514."
    test: "FUKU_TEST_DATABASE=true DATABASE_URL=... go test -tags testtools -count=1 -run '^TestTimedPunishmentMigrationDefaults$' ./fuku/db/migrations"
    expected: "The three duration columns apply twice, existing rows read 0, and 31622401 fails with SQLSTATE 23514 naming the check."
    why_human: "No PostgreSQL here. The test skips. The SQL text is present; the constraint was not executed."
  - truth: "Migration 20261122130000 runs twice on PostgreSQL, refuses 366 with SQLSTATE 23514, and concurrent warns of one member keep one count each."
    test: "Run TestWarnExpiryMigrationDefaults against PostgreSQL, and TestConcurrentWarnAndRemovePreserveCount under make test-postgres-integrity."
    expected: "Second apply is a no-op, 366 is SQLSTATE 23514, and concurrent warns do not lose a count."
    why_human: "TestWarnExpiryMigrationDefaults skipped. The column SQL is present; the PostgreSQL run was not."
  - truth: "Migration 20261122140000 runs twice on PostgreSQL and its checks refuse a bad step action, seconds over 31622400, forget_days 0 or 366, and a negative offence count with SQLSTATE 23514."
    test: "Run TestEscalationMigrationDefaults against PostgreSQL."
    expected: "Both tables apply twice and each named check refuses the out-of-range value with SQLSTATE 23514."
    why_human: "TestEscalationMigrationDefaults skipped. SQLite check coverage in TestEscalationSettings is not that run."
  - truth: "Sixteen concurrent CountOffence calls for one member at one instant leave one row with count 1."
    test: "On PostgreSQL, go test -tags testtools -count=1 -run '^TestOffenceCountConcurrent$' ./fuku/db/escalation"
    expected: "The sixteen-workers subtest runs, every result is 1, and one chat_offences row exists."
    why_human: "Off PostgreSQL the test returns before t.Run. A green line is not the subtest. T-26-34 forbids marking it passed."
human_verification:
  - test: "With PostgreSQL and DATABASE_URL set, run make test-postgres-integrity, including TestTimedPunishmentMigrationDefaults, TestWarnExpiryMigrationDefaults, TestEscalationMigrationDefaults, and TestOffenceCountConcurrent."
    expected: "Migrations apply twice, out-of-range checks fail with SQLSTATE 23514, and sixteen concurrent offences count as one."
    why_human: "No PostgreSQL in this environment (T-26-34). The three *MigrationDefaults tests skipped. TestOffenceCountConcurrent passed with no subtest."
  - test: "Run make lint and gosec on this tree."
    expected: "Both exit 0. No new G101 or phase-26 finding."
    why_human: "golangci-lint and gosec are not installed. Plan 26-12's gate truth must not be marked passed."
  - test: "TZ=UTC go test -tags testtools -count=1 ./fuku/modules, and compare failures with commit 78648d16."
    expected: "Phase 26 tests stay green. Remaining failures are the pre-existing captcha expiry set (chk_captcha_expires_at) and TestChatClockReadsTheGroup, TestApplyTimezone, TestSettingsTimezoneTyped."
    why_human: "The full package is not green here. TestLegacyScreeningNoticeRequests captcha_kick, captcha_ban, captcha_mute, and captcha_kick_held failed in this pass on CreateCaptchaAttemptPreMessage / chk_captcha_expires_at. That is the known pre-phase captcha set, not a timed-punishment golden drift, and it is not proof the suite passed."
---

# Phase 26: Timed Punishments, Escalation and Warn Expiry Verification Report

**Phase Goal:** As the owner, I can make automatic punishments timed, escalate repeat offenders step by step, and let old warns expire.
**Verified:** 2026-11-22T15:19:38Z
**Status:** human_needed
**Re-verification:** No — initial verification

There is no `26-SECURITY.md`. None was written.

The 26-12 close-out (commit `a286b62d`, documenting `d6793499`) did not re-run `TestTimedPunishmentsEndToEnd`. This pass did. Eight subtests passed. That does not make `make lint`, gosec, `make test-postgres-integrity`, or the full `fuku/modules` package green.

## Goal Achievement

### Observable Truths

Roadmap success criteria are the contract. Plan 26-12 truths 1–4 restate them and are not scored twice.

| # | Truth | Status | Evidence |
| --- | --- | --- | --- |
| 1 | Owner sets a timed ban or timed mute, with a duration, for warn limit, blacklist, or flood, from the page and by command. The member is restricted until the end date, the message shows `{duration}` and `{until}`, and the restriction ends at that date. | ✓ VERIFIED | `TestTimedPunishmentsEndToEnd` `warn_limit_page`, `flood_command`, `blacklist_page`, `pm_until` passed. `applyTimedPunishment` sends `restrictChatMember` or `banChatMember` with `UntilDate` from `punishNow()` immediately before the call. Pages call `applyWarnMode`, `applyFloodMode`, and `applyBlacklistAction`. |
| 2 | With the ladder on, a member's first, second, and later automatic offences get the owner's steps, in order, on whichever replica sees them. With the ladder off (the default), each setting's own punishment applies. | ✓ VERIFIED | E2E `ladder` passed, including a second bot on the same database, forget-after-31-days back to step 1, and ladder-off plain mute with no `chat_offences` row. `TestEscalation` `steps_in_order`, `off`, `second_bot_instance` passed. No row reads off (`TestEscalationSettings`). `enabled` defaults false in the migration. |
| 3 | With warn expiry of N days, a warn older than N days does not count, and `/warns` shows what counts. Off (the default) changes nothing. | ✓ VERIFIED | E2E `warn_expiry` passed (7 days hides the 8-day warns, shows the note, limit uses counted warns, Off shows every warn). `TestWarnExpiry` `display`, `limit`, `off` passed. Column default is 0. |
| 4 | A group that sets none of this behaves as today, and every argument the three commands accept today answers as before. | ✓ VERIFIED | E2E `nothing_set` passed. `TestLegacyTimedPunishmentRequests` (33 subtests) and `TestLegacyPunishmentExport` passed. Both golden files have only commit `92f29f19`, test-only, before any production commit of this phase. `go.mod` / `go.sum` are unchanged since `92f29f19^`. |
| 5 | 26-01: goldens pin `/setfloodmode`, `/flood`, bare timed words, failed save, non-admin, and the stored row. | ✓ VERIFIED | Those subtests passed inside `TestLegacyTimedPunishmentRequests`. |
| 6 | 26-01: goldens pin `/warns`, `/rmwarn`, the Remove button, and `/resetwarns`. | ✓ VERIFIED | `warns_*`, `rmwarn_*`, `resetwarns_*` subtests passed. |
| 7 | 26-01: goldens pin inert timed warn limit, flood, blacklist, and member-requirement warn. | ✓ VERIFIED | `warnlimit_inert_tmute`, `flood_inert_tmute`, `blacklist_inert_*`, `mreq_warnlimit_inert_tmute` passed. |
| 8 | 26-01: export golden of warns, antiflood, and blacklists with no duration, expiry, or warn times. | ✓ VERIFIED | `TestLegacyPunishmentExport` passed. |
| 9 | 26-01: both golden files are one commit, before production changes, and were not re-recorded. | ✓ VERIFIED | `git log` for both files is only `92f29f19` (`test(26-01)`), two files, +626 lines. |
| 10 | 26-02: `/setwarnmode tmute 2h` stores `tmute` and 7200; the limit sends `restrictChatMember` with that end; `tban` sends `banChatMember`. | ✓ VERIFIED | `TestWarnLimitTimed` `tmute_default` and `tban_default` passed. `applyWarn` calls `WarnUserAt` then `applyTimedPunishment`. |
| 11 | 26-02: bare `tban` stays unknown, `tmute 0m` and `tmute 400d` write nothing, `ban 1d` stores plain ban. | ✓ VERIFIED | `TestApplyWarnModeTimed` and `TestSetWarnModeTimedCommand` were in the passing `fuku/modules` run. `warnModeArgs` maps a bare timed word to seconds 0 and a bad duration to -1. |
| 12 | 26-02: custom warn-limit text fills `{duration}`, `{until}`, and the mute or ban word; the private copy adds Until; untimed events stay byte-identical. | ✓ VERIFIED | `TestWarnLimitTimed` `tmute_custom`, `pm_until/timed`, `pm_until/untimed` passed. `warnLimitEventReply` calls `warnLimitDefaultReply`. |
| 13 | 26-02: a member-requirement warn that reaches a timed limit posts the mute or ban notice, and a timed ban counts as a removal. | ✓ VERIFIED | `TestWarnLimitTimed` `mreq_timed_limit/tmute` and `tban` passed. |
| 14 | 26-02: a stored timed mode with no duration stays inert, so the pinned legacy suites still pass. | ✓ VERIFIED | `TestWarnLimitTimed/inert_no_duration` and the 26-01 inert subtests passed. `TestLegacyPunishmentNoticeRequests`, `TestLegacyWarnSettingsRequests`, and `TestLegacyBlacklistRequests` passed in the follow-up run. `TestLegacyScreeningNoticeRequests` captcha subtests failed on `chk_captcha_expires_at` (pre-existing; see Human Verification). That failure is not a timed-punishment request change. `autoActionEffective` returns false for `tmute`/`tban` unless the seconds are in range. |
| 15 | 26-02: migration `20261122120000` runs twice on PostgreSQL and refuses 31622401 with SQLSTATE 23514. | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | SQL adds `warn_mode_secs`, `antiflood_settings.action_secs`, and `blacklists.action_secs`, each `CHECK (... BETWEEN 0 AND 31622400)`. `TestTimedPunishmentMigrationDefaults` skipped: no `FUKU_TEST_DATABASE`. |
| 16 | 26-03: Warnings page Timed mute + 1 h stores the same row as `/setwarnmode tmute 1h`, and the next limit mute lasts an hour. | ✓ VERIFIED | `TestSettingsWarnsTimed` and `TestSettingsParityWarnTimed` passed in the settings run. `settingsWarnsSetTimed` calls `applyWarnMode`. |
| 17 | 26-03: the duration screen offers eight presets, marks the stored one, and shows a non-preset (90m) with nothing marked. | ✓ VERIFIED | `renderSettingsDuration` plus `settingsTimedDurationScreen`. `TestSettingsWarnsTimed` passed. |
| 18 | 26-03: an `nt` press needs the live warn-mode right; a member, a gated admin, and a forged duration, mode, or extra field write nothing. | ✓ VERIFIED | `TestSettingsAuthorityParityWarnTimed` and `TestSettingsWarnsTimedForged` passed. |
| 19 | 26-03: a Messages-page preview fills `{duration}` in the group language and `{until}` one hour ahead. | ✓ VERIFIED | `TestSettings` messages-auto coverage is in the passing settings run (`settings_messages_auto_test.go` is in the phase diff). Event values are filled in `event_message_auto.go`. |
| 20 | 26-03: Warnings and duration screens fit 3800 UTF-16 units and 64-byte buttons in 8 languages. | ✓ VERIFIED | `settings_warns_screens_test.go` is in the phase diff and the settings package run passed. |
| 21 | 26-04: `/setfloodmode tmute 30m` or the Antiflood page mutes with `until_date` = now + duration; a timed ban has no Unmute row. | ✓ VERIFIED | `TestFloodTimed` `tmute` and `tban` passed. `checkFlood` calls `escalate` then `floodPunish`; timed branch calls `applyTimedPunishment`. Page `ft` calls `applyFloodMode`. |
| 22 | 26-04: `tmute` alone is still the unknown reply; `tmute 0m` writes nothing; `ban x` stores ban with 0 seconds; `/flood` shows the timed word. | ✓ VERIFIED | `TestSetFloodModeTimedCommand` and `TestApplyFloodModeTimed` passed. Bare `tmute` is in the 26-01 golden, which passed. |
| 23 | 26-04: a flooding sender chat under a timed action gets nothing; a stored `tmute` with no duration stays the inert notice. | ✓ VERIFIED | `TestFloodTimed` `sender_chat_tban` and `inert_no_duration` passed. `floodPunishTimed` returns no notice when `!user.IsUser()`. |
| 24 | 26-04: custom flood text fills `{duration}`, `{until}`, and the action word; the private copy adds Until. | ✓ VERIFIED | `TestFloodTimed` `custom_text` and `pm_until` passed. E2E `pm_until` passed. |
| 25 | 26-04: page and command leave the same antiflood row; the press needs the flood right; the first timed write shows the on-notice; screens fit. | ✓ VERIFIED | `settings_antiflood_timed_test.go` was in the passing settings run. `settings_antiflood.go` calls `applyFloodMode` for both plain and timed presses. |
| 26 | 26-04: `checkFlood` complexity is no higher than 31 at `b7662c7`, and the legacy antiflood tests still pass. | ✓ VERIFIED | AST complexity (same counter that reads 31 on `b7662c7`) is 21 now. `TestLegacyTimedPunishmentRequests` passed. |
| 27 | 26-05: `/blacklistaction tmute 2h` or the action screen stores `tmute` / 7200 on every word and mutes until now+7200, notice naming duration and escaped reason. | ✓ VERIFIED | `TestBlacklistTimed/tmute` passed. `blacklistWatcher` calls `escalate` then `blacklistPunish` then `applyTimedPunishment`. Page `bz` calls `applyBlacklistAction`. |
| 28 | 26-05: bare `tmute`/`tban` and `ban mute` still answer `choose_correct_option`; `tmute 0m` writes nothing; no-argument shows `tmute 2h`. | ✓ VERIFIED | `TestBlacklistActionTimedCommand` passed. Bare words are in the 26-01 golden, which passed. |
| 29 | 26-05: a word added after a timed action takes that action and seconds; a mixed list gives a new word warn and 0. | ✓ VERIFIED | `TestApplyBlacklistActionTimed` passed. `SetBlacklistActionTimedContext` exists and the seconds-0 wrappers remain. |
| 30 | 26-05: a sender chat under a timed action gets no request; a durationless `tban` deletes and posts nothing; forward-block plain mute/kick/ban notices stay byte-identical. | ✓ VERIFIED | `TestBlacklistTimed` `sender_chat_tmute` and `inert_no_duration` passed. `TestForwardBlockVocabulary` passed. `forwardBlockMute/Kick/Ban/Warn` complexities are unchanged from `b7662c7` (2, 2, 2, 4). |
| 31 | 26-05: the action screen draws one timed action only when every word shares action and seconds; the press needs the live right; screens fit. | ✓ VERIFIED | `settings_blacklist_timed_test.go` was in the passing settings run. |
| 32 | 26-05: `blacklistWatcher` complexity ≤ 30 and `addBlacklist` ≤ 23 at `b7662c7`; legacy blacklist and forward-block tests pass. | ✓ VERIFIED | Now 26 and 23. `TestLegacyBlacklistRequests` and `TestForwardBlockVocabulary` passed. |
| 33 | 26-06: expiry of 7 days drops an 8-day warn from `/warns` and from the limit count. | ✓ VERIFIED | `TestWarnExpiry` `display` and `limit` passed. E2E `warn_expiry` passed. `applyWarn` uses `WarnUserAt(..., punishNow())`. |
| 34 | 26-06: `/rmwarn` and the button remove the newest counted warn; `/resetwarns` says removed only when one counted; AI spam uses the counted number. | ✓ VERIFIED | `TestWarnExpiry` `rmwarn`, `button`, `resetwarns`, `aispam_count` passed. |
| 35 | 26-06: with expiry off, warn commands, the limit, and the export match today. | ✓ VERIFIED | `TestWarnExpiry/off` and `TestLegacyPunishmentExport` passed. |
| 36 | 26-06: a pre-phase warn is dated at the row's last change, and a previous-build cache value counts every warn. | ✓ VERIFIED | `(*Warns).Counted` / `NormalisedTimes` exist. `TestWarnsCounted` was in the passing `fuku/db/models` run. Migration comment and `warn_times` default `'[]'` match that rule. |
| 37 | 26-06: migration `20261122130000` runs twice on PostgreSQL, refuses 366 with SQLSTATE 23514, and concurrent warns keep one count each. | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | SQL adds `warn_expiry_days` (`chk_warns_settings_expiry`, 0–365) and `warn_times jsonb default '[]'`. `TestWarnExpiryMigrationDefaults` skipped. |
| 38 | 26-06: export carries counted warns and their times; import keeps valid times only; out-of-range expiry imports as 0; the 26-01 export golden still passes. | ✓ VERIFIED | `exportWarnsData` calls `Counted`. `TestImportSanitizesPunishmentSettings` and `TestWarnsExportCountsWhatCounts` were in the passing `fuku/db/backup` run. `TestLegacyPunishmentExport` passed. |
| 39 | 26-07: the owner picks 7 days on the Warnings expiry screen; `/warns` and the limit then use 7 days; Off counts every warn. | ✓ VERIFIED | E2E presses `settingsActWarnsExpiry` `v=7` then `v=0`. `TestSettingsWarnsExpiry` and `TestSettingsDriftWarnExpiry` passed. `nx` calls `applyWarnExpiry`, whose seam is `SetWarnExpiryContext`. |
| 40 | 26-07: the expiry press needs the live warn right; a member, a gated admin, and a forged value write nothing. | ✓ VERIFIED | `TestSettingsAuthorityWarnExpiry` and `TestSettingsWarnsExpiryForged` passed. |
| 41 | 26-07: the page draws a repository-stored 30; opening the page or the expiry screen creates no row. | ✓ VERIFIED | `TestSettingsDriftWarnExpiry` passed (it calls `SetWarnExpiryContext(30)`). |
| 42 | 26-07: Warnings rows and the expiry screen fit 64-byte buttons and 3800 UTF-16 units in 8 languages. | ✓ VERIFIED | `settings_warns_screens_test.go` is in the passing settings run. |
| 43 | 26-08: migration `20261122140000` runs twice on PostgreSQL and its checks refuse a bad step, seconds over 31622400, forget_days outside 1–365, and a negative count with SQLSTATE 23514. | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | SQL creates `chat_escalation` and `chat_offences` with those named checks. `TestEscalationMigrationDefaults` skipped. |
| 44 | 26-08: `CountOffence` returns 1, then the same count inside 60 s, then +1, then 1 after the forget period; sixteen concurrent calls on PostgreSQL leave count 1. | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | `TestOffenceCountRules` passed on this database (count, window, forget). The sixteen-worker subtest did not run: `TestOffenceCountConcurrent` returns when the driver is not `postgres`, and this pass printed `PASS` with no subtest. Not a PostgreSQL pass (T-26-34). |
| 45 | 26-08: `Get` is cached under `chat_escalation:<chat>` and listed in `skipLocal`; a missing row is off / no steps / 30 days and creates nothing; writes are column upserts that drop the key. | ✓ VERIFIED | `TestEscalationSettings` passed, including the no-row case and two writers. `TestSkipLocalChatEscalationKey` and `TestSkipLocalMatchesFreshnessCriticalKeys` passed. `skipLocal` prefixes `chatEscalationPrefix`. |
| 46 | 26-08: neither ladder table is in backup, export, import, or reset. | ✓ VERIFIED | `TestEscalationStaysOutOfBackup` was in the passing `fuku/db/backup` run. |
| 47 | 26-09: a ladder of tmute 1 h, tmute 1 d, ban applies those steps to flood, blacklist, and forward blocklist, on any replica, and the notice names the applied action. | ✓ VERIFIED | `TestEscalation` `steps_in_order`, `flood_steps`, `second_bot_instance`, `forward_album` passed. E2E `ladder` passed. Call sites: `antiflood.go:347`, `blacklists.go:813`, `staff_forwardblock_watch.go:330`. |
| 48 | 26-09: offences within 60 s share a step and count once; after the forget period the next offence is step 1. | ✓ VERIFIED | `TestEscalation` `same_offence` and `forget` passed. E2E advances 61 s between offences and 31 days before the restart. `countOffenceSQL` encodes the window and the forget period. |
| 49 | 26-09: ladder off, no step, a sender chat, a warn, and delete-only are not counted; goldens stay unchanged. | ✓ VERIFIED | `TestEscalation` `off`, `sender_chat`, `not_offences` passed. E2E `nothing_set` and ladder-off passed. `escalate` returns `p` when the action is ineffective, `userID <= 0`, or the ladder is off or empty. |
| 50 | 26-09: an unreadable ladder keeps the rule's action; a failed or 2 s count applies step 1; one gated Warn line carries chat and user IDs only. | ✓ VERIFIED | `TestEscalation` `unreadable_ladder` and `count_failed` passed. `escalationErrClass` logs `timeout`, `canceled`, or `error`, not the error text. `escalationCountTimeout` is 2 s. |
| 51 | 26-09: no goroutine, Redis key, or shutdown drain; `checkFlood` and `blacklistWatcher` stay at or under the `b7662c7` caps; forward-block names and signatures stay; `TestEventEngineCallers` and `TestPMCopyCallers` need no new allow-list entry. | ✓ VERIFIED | `escalation.go` starts no goroutine. Complexities: `checkFlood` 21 (was 31), `blacklistWatcher` 26 (was 30). Forward-block function complexities unchanged except `forwardBlockEnforce` 8 → 9, which is the `escalate` call. Both caller tests passed. |
| 52 | 26-10: private `/settings` shows Escalation after Warnings, Off by default; turning it on stores the example ladder and Main reads 3 steps. | ✓ VERIFIED | `TestSettingsEscalationOpens` passed. E2E opens `settingsPageEscalation` and `GetFresh` sees 3 steps. `settingsPageEscalation = "j"`. |
| 53 | 26-10: the owner sets a step (ban, kick, mute, or timed with a preset), adds up to five, removes the last, and picks 1/7/30/90 day memory; the next offence follows the stored ladder. | ✓ VERIFIED | `TestSettingsEscalationSteps` passed. Page writes go through `applyEscalationStep`, `applyEscalationRemoveStep`, `applyEscalationForget`. |
| 54 | 26-10: removing the only step while on is refused; a stale press writes nothing; turning the ladder off clears offence counts. | ✓ VERIFIED | Covered by `TestSettingsEscalationSteps` / drift tests in the passing run. `applyEscalationEnabled` calls `ClearOffences` when turning off. E2E confirms zero rows after off. |
| 55 | 26-10: view needs the live admin check and owner gate; writes need `settingsRightRestrict` live; forged and group-menu presses write nothing. | ✓ VERIFIED | `TestSettingsAuthorityEscalation` and `TestSettingsEscalationForged` passed. |
| 56 | 26-10: the page is private only, hidden in a Staff Group, opening it creates no row, buttons ≤ 64 bytes (worst 51), screens fit 8 languages, and the Main grid keeps the owner rows. | ✓ VERIFIED | `TestSettingsEscalationReadsCreateNothing`, `TestSettingsEscalationCallbackBytes`, and `TestSettingsEscalationPageFitsEveryLocale` passed. E2E `staff_group` finds no Escalation line. |
| 57 | 26-11: logs of timed punishment, the ladder, expiry, and apply functions carry no reason, word, name, stored text, or message ID; failures name the chat. | ✓ VERIFIED | `TestTimedPunishmentLogsCarryIDsOnly` passed. `escalate` Warn lines use chat and user IDs plus `escalationErrClass`. |
| 58 | 26-11: new default texts and the private Until line fit Telegram limits in 8 languages, with the name escaped and no raw placeholder. | ✓ VERIFIED | `TestTimedNoticeTextsFitEveryLocale` passed. |
| 59 | 26-11: every Phase 26 action code belongs to one page, is not a core code, accepts only its fields, and a forged or group-menu press is refused in step 1; opening a screen creates no settings row. | ✓ VERIFIED | `TestPhase26Codes` and `TestPhase26ReadsCreateNothing` passed. |
| 60 | 26-11: no new screen names the Staff Group, its title, or its ID, in 8 languages, including a non-creator admin. | ✓ VERIFIED | `settingsPrivacyPhase26Screens` is wired inside `TestSettingsStaffPrivacy` (`settings_staff_privacy_test.go`). |
| 61 | 26-11: ladder off writes no offence row and makes no count statement; ladder on makes exactly one; drawing Escalation makes no Telegram call of its own. | ✓ VERIFIED | `TestPhase26Cost` passed. E2E ladder-off asserts zero rows. |
| 62 | 26-12: the full `fuku/modules` run, `make lint`, `make check-translations`, `make check-docs`, `make test-shard-check`, and gosec pass, and the full-package time is recorded. | ? UNCERTAIN | Not marked passed. `make lint`, gosec, and `make test-postgres-integrity` were not run (tools and PostgreSQL absent). The full `fuku/modules` package is red on this machine: this pass reproduced `TestLegacyScreeningNoticeRequests` captcha failures (`chk_captcha_expires_at`). The pause record says that set, plus the three timezone tests, already fails on `78648d16`. `go.mod` and `go.sum` are unchanged. Phase 26's own named tests passed (see spot-checks). |

**Score:** 57/62 truths verified (4 present, behavior-unverified; 1 uncertain)

### Required Artifacts

Every `must_haves.artifacts` path exists, contains the declared symbol, and is referenced from production or from a test that calls production. No stub returns.

| Artifact | Expected | Status | Details |
| --- | --- | --- | --- |
| `fuku/modules/punish_timed.go` | `applyTimedPunishment` | ✓ VERIFIED | 192 lines. Sends restrict or ban with `UntilDate`. Wired from warns, flood, blacklist, and forward block. |
| `fuku/modules/timed_punishments_e2e_test.go` | `TestTimedPunishmentsEndToEnd` | ✓ VERIFIED | 8 subtests. Re-run this pass: PASS 0.11s. |
| `fuku/modules/escalation.go` | `escalate` | ✓ VERIFIED | Wired from the three automatic sites. |
| `fuku/db/escalation/repository.go` | `CountOffence` | ✓ VERIFIED | One `INSERT ... ON CONFLICT ... RETURNING`. |
| `migrations/20261122120000_add_timed_punishment_durations.sql` | three duration checks | ✓ VERIFIED | File exists. PostgreSQL execution unverified (truth 15). |
| `migrations/20261122130000_add_warn_expiry.sql` | expiry check and `warn_times` | ✓ VERIFIED | File exists. PostgreSQL execution unverified (truth 37). |
| `migrations/20261122140000_add_escalation_ladder.sql` | both tables | ✓ VERIFIED | File exists. PostgreSQL execution unverified (truth 43). |
| `fuku/modules/settings_duration.go` | `renderSettingsDuration` | ✓ VERIFIED | Called by `settingsTimedDurationScreen` (warns, flood, blacklist) and directly by the Escalation page. |
| `fuku/modules/settings_warns.go` | `nt`, `nx` | ✓ VERIFIED | `nt` → `applyWarnMode`; `nx` → `applyWarnExpiry`. |
| `fuku/modules/settings_antiflood.go` | `ft` | ✓ VERIFIED | Calls `applyFloodMode`. |
| `fuku/modules/settings_blacklist.go` | `bz` | ✓ VERIFIED | Calls `applyBlacklistAction`. |
| `fuku/modules/settings_escalation.go` | page `j` | ✓ VERIFIED | Writes go through `applyEscalation*`. |
| `fuku/modules/timed_punishment_legacy_request_test.go` | `TestLegacyTimedPunishmentRequests` | ✓ VERIFIED | Only commit `92f29f19`. Passed. |
| `fuku/db/backup/punish_export_legacy_test.go` | `TestLegacyPunishmentExport` | ✓ VERIFIED | Same commit. Passed. |
| Remaining plan artifacts (`warns` repository, models `Counted`, import test, safety tests, migration tests, offence tests) | symbols named in each plan | ✓ VERIFIED | Each `contains` pattern is present. Line counts are well above stubs. |

**Artifacts:** 41/41 present and substantive. PostgreSQL-only tests are present and skip; they are not stubs.

### Key Link Verification

| From | To | Via | Status | Details |
| --- | --- | --- | --- | --- |
| `timed_punishment_legacy_request_test.go` | `legacyRequestJSON` | one serialiser | ✓ WIRED | Called for the golden cases. |
| same | `newNoticeGoldenSite` | Phase 23 site | ✓ WIRED | Called by name. |
| `warns.go` `applyWarnLimitAction` | `applyTimedPunishment` | end date before the request | ✓ WIRED | `warns.go:314`. |
| `warns.go` `setWarnMode` | `applyWarnMode` | command and page | ✓ WIRED | `warns.go:208`. Page `nt` also calls it (`settings_warns.go:470`). |
| `warnLimitEventReply` | `warnLimitDefaultReply` | `{duration}` and `{until}` | ✓ WIRED | `warns.go:518`. |
| `settings_warns.go` `nd` | `renderSettingsDuration` | shared duration screen | ✓ WIRED | Via `settingsWarnsOpenDuration` → `settingsTimedDurationScreen` → `renderSettingsDuration` (`settings_duration.go:108`). Not a direct call; the screen is the shared one. |
| `checkFlood` | `floodPunish` | one helper | ✓ WIRED | `antiflood.go:347-348`, after `escalate`. |
| Antiflood `ft` | `applyFloodMode` | page and command | ✓ WIRED | `settings_antiflood.go:502`. |
| `blacklistWatcher` | `blacklistPunish` | one helper | ✓ WIRED | `blacklists.go:813-814`, after `escalate`. |
| `listNoticeReply` | `blacklistNoticeDefaultReply` | shared notice | ✓ WIRED | `event_message_auto.go:291`. |
| `applyWarn` | `WarnUserAt` | counted limit | ✓ WIRED | `warns.go:277`. |
| `exportWarnsData` | `Counted` | one counting rule | ✓ WIRED | `backup.go:503`. |
| `nx` | `applyWarnExpiry` | only expiry write | ✓ WIRED | `settings_warns.go:503`. |
| `applyWarnExpiry` | `SetWarnExpiryContext` | cache drop | ✓ WIRED | Seam `warnSettingsWrites.Expiry` is `SetWarnExpiryContext` (`warns.go:98`). |
| `CountOffence` | `chat_offences` | one upsert | ✓ WIRED | `countOffenceSQL` `ON CONFLICT (chat_id, user_id)`. |
| `skipLocal` | `chatEscalationPrefix` | every replica | ✓ WIRED | `local.go` prefix check. Test passed. |
| `checkFlood`, `blacklistWatcher`, `forwardBlockEnforce` | `escalate` | before punish | ✓ WIRED | Three call sites above. |
| `escalate` | `escalationCount` | 2 s context | ✓ WIRED | `escalation.go:63`. Default is `CountOffence`. |
| `settings_escalation.go` | `applyEscalation*` | one apply per control | ✓ WIRED | Toggle, forget, step, remove. |
| `escalation_settings.go` | `escalationWrites` / `GetFresh` | repository seam | ✓ WIRED | `escalationRead = escalation.GetFresh`. |
| staff privacy test | new screens | sentinel title and ID | ✓ WIRED | `settingsPrivacyPhase26Screens` in `settings_staff_privacy_test.go`. |
| e2e test | real dispatcher | settings env | ✓ WIRED | `newTimedE2E` → `newAutoMessagesE2E` → `newPMCopiesE2E` → `newMessagesE2E` → `newSettingsEnvWith`. The plan's literal pattern is one frame down, not missing. |

**Wiring:** 22/22 connections verified.

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| --- | --- | --- | --- | --- |
| Timed notice | `{duration}`, `{until}` | `punishDurationLabel` and the `until` returned by `applyTimedPunishment` (the same value sent as `until_date`) | Yes | ✓ FLOWING |
| Warnings / Antiflood / Blacklist pages | stored action and seconds | `apply*` read-back and repository `Get` | Yes | ✓ FLOWING |
| `/warns` and the limit | counted warns | `WarnUserAt` / `(*Warns).Counted` over stored `warn_times` | Yes | ✓ FLOWING |
| Ladder step | offence count | `CountOffence` `RETURNING offences` | Yes | ✓ FLOWING |
| Escalation page | ladder | `escalation.GetFresh` after `escalationWrites` | Yes | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| --- | --- | --- | --- |
| Owner flows, ladder, expiry, nothing-set | `go test -tags testtools -count=1 -run '^TestTimedPunishmentsEndToEnd$' ./fuku/modules` | 8/8 PASS (0.11s), including `warn_limit_page`, `flood_command`, `blacklist_page`, `ladder`, `warn_expiry`, `pm_until`, `staff_group`, `nothing_set` | ✓ PASS |
| Legacy command and inert goldens | `-run '^TestLegacyTimedPunishmentRequests$'` | 33 subtests PASS | ✓ PASS |
| Warn limit, flood, blacklist, expiry, ladder units | `-run '^(TestWarnLimitTimed\|TestFloodTimed\|TestBlacklistTimed\|TestWarnExpiry\|TestEscalation)$'` | all PASS | ✓ PASS |
| Settings, authority, forged, phase-26 safety, non-captcha legacy suites | second `fuku/modules` `-run` (list in the command log) | `ok` 0.846s | ✓ PASS |
| Export golden, offence rules, ladder settings | `fuku/db/backup`, `fuku/db/escalation` | `TestLegacyPunishmentExport` PASS; `TestOffenceCountRules` PASS; `TestEscalationSettings` PASS | ✓ PASS |
| PostgreSQL migration defaults | `TestTimedPunishmentMigrationDefaults`, `TestWarnExpiryMigrationDefaults`, `TestEscalationMigrationDefaults` | `--- SKIP` (no `FUKU_TEST_DATABASE`) | ? SKIP |
| Sixteen concurrent offences | `TestOffenceCountConcurrent` | `--- PASS` with no subtest, because `db.DB.Name() != "postgres"` returns first | ? SKIP |
| `skipLocal` for `chat_escalation` | `-run '^(TestSkipLocalChatEscalationKey\|TestSkipLocalMatchesFreshnessCriticalKeys)$' ./fuku/db/cache` | PASS | ✓ PASS |
| Captcha goldens named by plan 26-02 | `TestLegacyScreeningNoticeRequests` | FAIL `captcha_kick`, `captcha_ban`, `captcha_mute`, `captcha_kick_held`: `chk_captcha_expires_at` | ✗ FAIL (pre-existing; not a phase gap) |

`make test` was not re-run as a full suite. The user recorded exit 2 (`fuku/modules`, 103 top-level failures, mostly captcha expiry plus the three timezone tests) and that the same set fails on `78648d16`. This pass does not treat that as a green suite and does not treat it as a Phase 26 regression.

### Probe Execution

No `scripts/*/tests/probe-*.sh` is declared by this phase. Step 7c: no probes to run.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| --- | --- | --- | --- | --- |
| TUNE-01 | 26-01, 26-02, 26-03, 26-04, 26-05, 26-06, 26-11, 26-12 | Timed ban or mute with an owner-chosen duration, from the page and the command, for warn limit, blacklist, and antiflood. | ✓ SATISFIED | Truths 1, 10, 16, 21, 27. E2E page and command paths passed. `REQUIREMENTS.md` still shows the checkbox pending; this report does not edit it. |
| TUNE-02 | 26-01, 26-08, 26-09, 26-10, 26-11, 26-12 | Per-group ladder, off by default; first, second, and later automatic offences. | ✓ SATISFIED | Truths 2, 47–56. Concurrent PostgreSQL proof is still human (truth 44). The owner-visible ladder is proven on the shared test database, including a second replica. |
| TUNE-03 | 26-01, 26-06, 26-07, 26-11, 26-12 | Warns older than N days stop counting. Off by default. | ✓ SATISFIED | Truths 3, 33, 39. E2E and `TestWarnExpiry` passed. |

No requirement ID in any plan frontmatter is missing from `REQUIREMENTS.md`. No Phase 26 ID in `REQUIREMENTS.md` is absent from the plans. No orphaned IDs.

### Decision Coverage

All trackable CONTEXT.md decisions are honored by shipped artifacts. 49/49 honored, 0 not honored. Non-blocking.

### Test Quality Audit

| Test File | Linked Req | Active | Skipped | Circular | Assertion Level | Verdict |
| --- | --- | --- | --- | --- | --- | --- |
| `timed_punishments_e2e_test.go` | TUNE-01, TUNE-02, TUNE-03 | 8 subtests, all run | 0 | No. Goldens were recorded in `92f29f19` before production edits and were not regenerated. | Behavioral (`until_date`, notice text, second replica) | PASS |
| `warns_timed_test.go`, `antiflood_timed_test.go`, `blacklists_timed_test.go`, `warns_expiry_test.go`, `escalation_test.go` | TUNE-01–03 | Run this pass | 0 | No | Behavioral | PASS |
| `timed_punishment_migration_test.go`, `warn_expiry_migration_test.go`, `escalation_migration_test.go` | storage for TUNE-01–03 | Statement tests not in this run's filter; Defaults exist | 3 `*MigrationDefaults` skip without PostgreSQL | No | Would be value-level on PostgreSQL | Environment skip, not a disabled test |
| `fuku/db/escalation/repository_test.go` `TestOffenceCountConcurrent` | TUNE-02 concurrency | Subtest not registered off PostgreSQL | Returns before `t.Run` (not `t.Skip`) | No | None on this machine | Warning: a green result is not the sixteen-worker proof |

**Disabled tests on requirements:** 0 always-disabled. The PostgreSQL tests skip only when the database is absent, and they are not the only tests of TUNE-01–03.
**Circular patterns detected:** 0. The 26-01 goldens have a single pre-production commit.
**Insufficient assertions:** 0 on the tests that ran. E2E compares `until_date` and rendered duration text.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| --- | --- | --- | --- | --- |
| `fuku/db/escalation/repository_test.go` | 392 | `if db.DB.Name() != "postgres" { return }` | ⚠️ Warning | `go test` prints PASS with no subtest. Do not read it as T-26-34 evidence. |
| `fuku/modules/screening_notice_legacy_request_test.go` | 197 | `chk_captcha_expires_at` on captcha goldens | ℹ️ Info | Pre-existing captcha expiry failure, reproduced this pass. Not introduced by the timed-punishment diff. Not a Phase 26 gap. |
| Phase 26 production Go | — | `TBD` / `FIXME` / `XXX` | none | No debt markers in the phase Go files scanned. |

No hollow handlers, empty punishment returns on the timed path, or placeholder notices. `go.mod` and `go.sum` have no diff since `92f29f19^`.

### Human Verification Required

### 1. PostgreSQL integrity (T-26-34)

**Test:** Run `make test-postgres-integrity` with `FUKU_TEST_DATABASE=true` and `DATABASE_URL` set.
**Expected:** `TestTimedPunishmentMigrationDefaults`, `TestWarnExpiryMigrationDefaults`, and `TestEscalationMigrationDefaults` apply twice and refuse out-of-range values with SQLSTATE 23514. `TestOffenceCountConcurrent` runs its sixteen-worker subtest and leaves one row with count 1.
**Why human:** No PostgreSQL here. The three Defaults tests skipped. The concurrent test returned before its subtest.

### 2. Lint and gosec

**Test:** `make lint` and gosec.
**Expected:** Exit 0.
**Why human:** Neither binary is installed. Plan 26-12 must not be marked passed on this gate.

### 3. Full `fuku/modules` package

**Test:** `TZ=UTC go test -tags testtools -count=1 ./fuku/modules`, compared with `78648d16`.
**Expected:** Phase 26 tests stay green. Any remaining failures are the pre-existing captcha expiry set and `TestChatClockReadsTheGroup`, `TestApplyTimezone`, `TestSettingsTimezoneTyped`.
**Why human:** This pass saw `TestLegacyScreeningNoticeRequests` captcha subtests fail on `chk_captcha_expires_at`. The recorded full-package failure predates Phase 26. That is not a pass, and it is not a Phase 26 implementation gap.

### Deferred Items

None. Phases 27–30 cover manual-command defaults, thresholds, tone and copy, and the backup page. They do not take the PostgreSQL checks, the concurrent offence count, lint, or gosec.

### Gaps Summary

No implementation gap blocks the phase goal. The owner can set a timed mute or ban from the page and the command, the ladder steps repeat offences (including a second replica) and is off by default, and warn expiry drops old warns from `/warns` and the limit. A group that sets none of this still matches the 26-01 goldens.

What is not verified, and must not be marked passed:

- The three `*MigrationDefaults` tests and `TestOffenceCountConcurrent` have not run against PostgreSQL (T-26-34).
- `make lint` and gosec have not run.
- The full `fuku/modules` package is red here for the pre-existing captcha expiry constraint and, per the pause record, the three timezone tests. The 26-12 close-out did not re-run the e2e test; this verification did, and that test passed.

---

_Verified: 2026-11-22T15:19:38Z_
_Verifier: Claude (gsd-verifier)_
