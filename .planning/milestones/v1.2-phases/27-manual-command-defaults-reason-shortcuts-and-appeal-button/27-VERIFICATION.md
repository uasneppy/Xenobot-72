---
phase: 27-manual-command-defaults-reason-shortcuts-and-appeal-button
verified: 2026-11-23T03:54:10Z
status: human_needed
score: 29/30 must-haves verified
covered_files:
  - .planning/phases/27-manual-command-defaults-reason-shortcuts-and-appeal-button/27-01-PLAN.md
  - .planning/phases/27-manual-command-defaults-reason-shortcuts-and-appeal-button/27-01-SUMMARY.md
  - .planning/phases/27-manual-command-defaults-reason-shortcuts-and-appeal-button/27-02-PLAN.md
  - .planning/phases/27-manual-command-defaults-reason-shortcuts-and-appeal-button/27-02-SUMMARY.md
  - .planning/phases/27-manual-command-defaults-reason-shortcuts-and-appeal-button/27-03-PLAN.md
  - .planning/phases/27-manual-command-defaults-reason-shortcuts-and-appeal-button/27-03-SUMMARY.md
  - .planning/phases/27-manual-command-defaults-reason-shortcuts-and-appeal-button/27-04-PLAN.md
  - .planning/phases/27-manual-command-defaults-reason-shortcuts-and-appeal-button/27-04-SUMMARY.md
  - .planning/phases/27-manual-command-defaults-reason-shortcuts-and-appeal-button/27-05-PLAN.md
  - .planning/phases/27-manual-command-defaults-reason-shortcuts-and-appeal-button/27-05-SUMMARY.md
  - .planning/phases/27-manual-command-defaults-reason-shortcuts-and-appeal-button/27-06-PLAN.md
  - .planning/phases/27-manual-command-defaults-reason-shortcuts-and-appeal-button/27-06-SUMMARY.md
  - .planning/phases/27-manual-command-defaults-reason-shortcuts-and-appeal-button/27-07-PLAN.md
  - .planning/phases/27-manual-command-defaults-reason-shortcuts-and-appeal-button/27-07-SUMMARY.md
  - AGENTS.md
  - Makefile
  - fuku/db/backup/modtuning_backup_test.go
  - fuku/db/backup/testmain_test.go
  - fuku/db/cache/local.go
  - fuku/db/cache/local_test.go
  - fuku/db/migrations/manual_command_tuning_postgres_test.go
  - fuku/db/models/modtuning.go
  - fuku/db/modtuning/repository.go
  - fuku/db/modtuning/repository_test.go
  - fuku/db/modtuning/testmain_test.go
  - fuku/i18n/eventmsg_locale_test.go
  - fuku/i18n/settings_locale_test.go
  - fuku/modules/antiflood.go
  - fuku/modules/bans.go
  - fuku/modules/event_message.go
  - fuku/modules/event_message_auto.go
  - fuku/modules/event_message_pm.go
  - fuku/modules/moderation.go
  - fuku/modules/moderation_phase27_legacy_request_test.go
  - fuku/modules/modtuning_appeal_test.go
  - fuku/modules/modtuning_commands_test.go
  - fuku/modules/modtuning_e2e_test.go
  - fuku/modules/modtuning_reasons_test.go
  - fuku/modules/modtuning_settings.go
  - fuku/modules/mute.go
  - fuku/modules/settings_fwdblock_test.go
  - fuku/modules/settings_input.go
  - fuku/modules/settings_input_door.go
  - fuku/modules/settings_list_input.go
  - fuku/modules/settings_list_locale_test.go
  - fuku/modules/settings_main_test.go
  - fuku/modules/settings_messages.go
  - fuku/modules/settings_moddefaults.go
  - fuku/modules/settings_moddefaults_test.go
  - fuku/modules/settings_phase19_test.go
  - fuku/modules/settings_phase27_test.go
  - fuku/modules/settings_reasons.go
  - fuku/modules/settings_reasons_test.go
  - fuku/modules/test_harness_test.go
  - fuku/modules/warns.go
  - locales/en.yml
  - locales/es.yml
  - locales/fr.yml
  - locales/hi.yml
  - locales/id.yml
  - locales/pt.yml
  - locales/ru.yml
  - locales/uk.yml
  - migrations/20261122150000_add_manual_command_tuning.sql
covered_digest: "v4:sha256:96f77a144f3b56c4f51e3aacce22455500d5f7f5901baa6130e622f4ecd58d6d"
behavior_unverified: 1
overrides_applied: 0
behavior_unverified_items:
  - truth: "Migration 20261122150000 creates chat_manual_defaults and chat_reason_shortcuts idempotently on PostgreSQL with inline named CHECK and UNIQUE constraints refusing out-of-range durations, >2048-char appeal_url, blank word/reason, >500-char reason, and duplicates with SQLSTATE 23514/23505."
    test: "FUKU_TEST_DATABASE=true DATABASE_URL=... make test-postgres-integrity (specifically TestManualCommandTuningMigrationDefaults and TestManualDefaultsAndShortcutsCRUD)"
    expected: "Migration applies cleanly twice in one transaction, default row reads all zeros, invalid values fail with SQLSTATE 23514/23505 naming the inline constraints, and boundary values (60, 31622400, 2048, 500) succeed."
    why_human: "No PostgreSQL instance is configured locally (DATABASE_URL unset); TestManualCommandTuningMigrationDefaults skipped. Structural DDL checks (TestManualCommandTuningMigrationStatements) and SQLite repository CRUD/bounds tests (TestManualDefaultsAndShortcutsCRUD) passed, but PostgreSQL constraint execution was not exercised."
human_verification:
  - test: "Run PostgreSQL integrity tests (FUKU_TEST_DATABASE=true DATABASE_URL=... make test-postgres-integrity) covering TestManualCommandTuningMigrationDefaults and TestManualDefaultsAndShortcutsCRUD."
    expected: "Migration 20261122150000 runs idempotently twice on PostgreSQL, enforces all inline named CHECK (SQLSTATE 23514) and UNIQUE (SQLSTATE 23505) constraints, and column-targeted upserts + shortcut CRUD pass under -race."
    why_human: "No PostgreSQL server is available in this environment (DATABASE_URL unset), so TestManualCommandTuningMigrationDefaults skipped."
  - test: "Run CI static checks (make lint and gosec) on this tree."
    expected: "Both golangci-lint and gosec exit 0 with zero new findings on Phase 27 files."
    why_human: "Neither golangci-lint nor gosec is installed in this local environment."
  - test: "Perform live Telegram UAT in a real supergroup + bot private chat: open private /settings -> Manual defaults (q) and Reason shortcuts (rs) (and confirm group /settings shows only the open-in-private note and Staff Group Main hides both); set a default /ban and /mute duration, toggle Reason required on, set an Appeal link, add a shortcut (word -> full text), run /ban without reason (refused), /ban @user <word> (default duration + expanded reason in group, PM copy, and log channel + clickable Appeal button on group and PM copy), /warn @user <word>, /tban @user 2h other (explicit duration + verbatim word), delete the shortcut behind the two-tap card, and clear defaults."
    expected: "Every screen renders cleanly in Telegram, buttons and deep-link/URL actions work, punishments apply the configured durations and appeal button, and clearing defaults restores permanent untimed behavior."
    why_human: "Visual rendering, live Telegram API acceptance of keyboards/URLs, and end-to-end UX cannot be verified offline."
  - test: "Confirm or request changes to provisional owner decisions P27-01 through P27-05 documented in AGENTS.md."
    expected: "Owner signs off on: P27-01 (default ban/mute duration applies to /ban, /dban, /sban and /mute, /dmute, /smute; /ban & /dban emit EventTimedBan and /mute & /dmute emit EventTimedMute; channel /ban stays permanent; /tban & /tmute keep explicit duration), P27-02 (reason_required covers all 14 manual user-punishment commands including d*/s*/t* variants; channel /ban, /unban, /unmute, /kickme, warnSourceAuto, and Staff Group untouched), P27-03 (single-word lower-cased <=32 rune shortcuts, 1..500 rune plain text, cap 50, no overwrite on duplicate, exact single-word manual match only), P27-04 (appeal button also attaches to automatic ban/tban outcomes — EventReqBan, EventWarnLimit, EventFlood, EventBlacklist, EventForwardBlock, EventCaptcha — plus their PM copies; never channel/silent/lockdown/Staff Group), and P27-05 (pages q and rs are PrivateOnly, hidden in Staff Groups, require live settingsRightRestrict on writes, and stay outside backup/export/import/reset until Phase 30)."
    why_human: "Discuss phase was skipped via workflow.skip_discuss=true, so owner-noticeable scope choices require explicit owner confirmation."
audit_acknowledged:
  milestone: v1.2
  at: 2026-11-24
  status: human_needed
---

# Phase 27: Manual Command Defaults, Reason Shortcuts and Appeal Button Verification Report

**Phase Goal:** As the owner, I can set how my admins' manual punishments behave by default, save short words for common reasons, and give banned people a way to appeal.
**Verified:** 2026-11-23T03:54:10Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### ROADMAP Success Criteria Rollup

| # | Roadmap Success Criterion | Status | Evidence |
| - | ------------------------- | ------ | -------- |
| SC-1 | With a default duration set, `/mute` and `/ban` given without a duration apply it. An explicit duration is unchanged. With no default, they behave as today. | ✓ VERIFIED | `manualBanMember` (`bans.go:250,363,404`), `manualMuteMember` (`mute.go:203,218,258`), `banReplyWithButton`/`muteReplyWithButton` emitting `EventTimedBan`/`EventTimedMute`; verified by `TestManualCommandDefaults`, `TestManualDefaultsEndToEnd`, `TestLegacyPhase27ModerationRequests`, `TestLegacyModerationRequests`. |
| SC-2 | With "reason required" on, a punishment with no reason is refused with a short reply and nothing happens. With a reason, it runs. It is off by default. | ✓ VERIFIED | `manualApplyReasonRequired` in `moderation.go:334`, `bans.go:115`, `mute.go:160`, `warns.go:614` via `replyValidationError` (`moddef_reason_required`); verified by `TestManualCommandDefaults` (all 14 commands + exemptions) and `TestManualDefaultsEndToEnd`. |
| SC-3 | The owner adds a reason shortcut (a word and its full text) on a page, browses the list and deletes an entry behind a two-tap card. `/ban @user spam` then shows the full text as the reason in the group message, the PM copy, the warn reason and the log post. A word that is not a shortcut is used as typed. | ✓ VERIFIED | Private list page `rs` (`settings_reasons.go`) + plain-text `settingsListTwoStep` (`settings_list_input.go`) + `expandManualReason` (`moderation.go:337`, `bans.go:118`, `mute.go:163`, `warns.go:617`); verified by `TestSettingsReasons`, `TestReasonShortcutValidationAndApply`, `TestReasonShortcutExpansion`, `TestManualDefaultsEndToEnd`. |
| SC-4 | With an appeal link set, a ban message (default or custom) and its PM copy carry the button. With none set, they carry none. The link passes the editor's link check. | ✓ VERIFIED | `validateAppealURL` using `content.ValidButtonURL`, `postEventSent`/`sendDefaultPost`/`composeEventMessage` (`event_message.go`), `composeEventPM`/`runEventPM` (`event_message_pm.go`), page `q` (`settings_moddefaults.go`); verified by `TestAppealURLValidationAndApply`, `TestAppealButtonOnBanMessages`, `TestSettingsModDefaults`, `TestManualDefaultsEndToEnd`. |

### Observable Truths

| # | Truth | Status | Evidence |
| - | ----- | ------ | -------- |
| 1 | (27-01) Characterization golden records on unchanged code at `b7393953` exact `sendMessage`, punishment, deletion, and stored warn requests across 17 manual moderation paths (`TestLegacyPhase27ModerationRequests`, MSG-11). | ✓ VERIFIED | `fuku/modules/moderation_phase27_legacy_request_test.go` passes 17/17 subtests under `-tags testtools -race -count=1`. |
| 2 | (27-01) Golden file committed alone in `71dbf865` before any Phase 27 production edit; neither it nor pre-existing golden files (`moderation_legacy_request_test.go`, `punishment_notice_legacy_request_test.go`, etc.) were modified afterward. | ✓ VERIFIED | `git log --oneline b7393953..HEAD -- fuku/modules/moderation_phase27_legacy_request_test.go` shows 1 commit (`71dbf865`, 1 file); `git diff b7393953..HEAD` on pre-existing goldens and `go.mod`/`go.sum` is empty. |
| 3 | (27-02) Migration `20261122150000_add_manual_command_tuning.sql` creates `chat_manual_defaults` and `chat_reason_shortcuts` idempotently with inline named CHECK/UNIQUE constraints (duration 0 or 60..31622400, appeal_url <= 2048, non-empty word/reason, reason <= 500). | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | Migration SQL and `TestManualCommandTuningMigrationStatements` pass (3 idempotent statements, inline named CHECKs/UNIQUE, no FKs), and SQLite bounds/CRUD pass (`TestManualDefaultsAndShortcutsCRUD`), but PostgreSQL execution (`TestManualCommandTuningMigrationDefaults`) skipped locally (`DATABASE_URL` unset) — see Human Verification. |
| 4 | (27-02) Both cache key prefixes (`chat_manual_defaults:`, `chat_reason_shortcuts:`) are in `fuku/db/cache/local.go` `skipLocal`; reads create no rows; column-targeted upserts drop cache via `cache.DeleteCache`; neither table is in backup export/import/clear. | ✓ VERIFIED | `local.go:59-60,117-118`, `repository.go:160-167,291,314`; verified by `TestSkipLocalChatManualDefaultsKey`, `TestSkipLocalChatReasonShortcutsKey`, `TestManualDefaultsAndShortcutsCRUD`, `TestModTuningExcludedFromBackup`. |
| 5 | (27-02) With `default_ban_secs` set, `/ban` (user target), `/dban`, and `/sban` apply `UntilDate` (guarded by `punishUntilTooClose`), `/ban` and `/dban` emit `EventTimedBan` with `Duration` and `Until`; channel `/ban` stays permanent; `/tban` keeps explicit duration; unset matches goldens (TUNE-04, P27-01, MSG-11). | ✓ VERIFIED | `modtuning_settings.go:201-240`, `bans.go:194-210,250,363,404`; verified by `TestManualCommandDefaults/default_ban_duration`, `TestManualDefaultsEndToEnd`, `TestLegacyPhase27ModerationRequests`, `TestLegacyModerationRequests`. |
| 6 | (27-02) With `default_mute_secs` set, `/mute`, `/dmute`, and `/smute` apply `UntilDate`, `/mute` and `/dmute` emit `EventTimedMute` with `Duration` and `Until`; `/tmute` keeps explicit duration; unset matches goldens (TUNE-04, P27-01, MSG-11). | ✓ VERIFIED | `modtuning_settings.go:244-258`, `mute.go:72-86,203,218,258`; verified by `TestManualCommandDefaults/default_mute_duration`, `TestLegacyPhase27ModerationRequests`, `TestLegacyModerationRequests`. |
| 7 | (27-02) With `reason_required` on, all 14 manual user-punishment commands without a reason (checked after duration extraction on `/tban`/`/tmute`) are refused with `moddef_reason_required` via `replyValidationError` with zero punishment/deletion/PM/log; channel `/ban`, `/unban`, `/unmute`, `/kickme`, `warnSourceAuto`, and Staff Group untouched (TUNE-04, P27-02, EVT-10). | ✓ VERIFIED | `moderation.go:333-336`, `bans.go:115`, `mute.go:160`, `warns.go:614`; verified by `TestManualCommandDefaults` (`reason_required_*`, `fail_open_*`, `staff_group_isolation`) and `TestManualDefaultsEndToEnd`. |
| 8 | (27-03) Shortcut word normalization/validation trims and lower-cases, enforces single word, rejects empty/leading `/`/`> 32` runes; reason text requires 1..500 trimmed runes; cap 50; duplicates refused without overwriting (TUNE-05, P27-03). | ✓ VERIFIED | `modtuning_settings.go:262-334`; verified by `TestReasonShortcutValidationAndApply` (4/4 subtests). |
| 9 | (27-03) Single-word case-insensitive reason shortcuts expand across all 14 manual user-punishment commands before group reply, PM copy, warn storage, and log channel post (TUNE-05). | ✓ VERIFIED | `moderation.go:337`, `bans.go:118`, `mute.go:163`, `warns.go:617`; verified by `TestReasonShortcutExpansion` (7/7 subtests) and `TestManualDefaultsEndToEnd`. |
| 10 | (27-03) `/tban` and `/tmute` expand after duration extraction; unknown words and multi-word reasons remain verbatim (empty/multi-word fast-path does zero DB/cache lookup) (TUNE-05). | ✓ VERIFIED | `modtuning_settings.go:353-357`, `bans.go:118`, `mute.go:163`; verified by `TestReasonShortcutExpansion/tban_and_tmute_preserve_duration_and_expand` and `unknown_word_and_multiword_verbatim_with_zero_lookup_on_multiword`. |
| 11 | (27-03) Stored shortcut text containing HTML characters (`<`, `>`, `&`, `"`) is stored raw and escaped wherever rendered in HTML (default replies, custom `{reason}`, PM copies, warn-limit, log channel) while `/warns` prints plain text (TUNE-05). | ✓ VERIFIED | Verified by `TestReasonShortcutExpansion/ban_default_custom_pm_and_log_html_escaped`, `warn_dwarn_swarn_expand_store_pm_and_limit_escape`, and `TestManualDefaultsEndToEnd`. |
| 12 | (27-03) Automatic warns (`warnSourceAuto`), automatic rules, and Staff Group commands never expand reason shortcuts (EVT-10). | ✓ VERIFIED | `expandManualReason` is called in `warns.go:dispatchWarn:617` (never `warnThisUser`) and zero `staff*.go` files reference `modtuning`; verified by `TestReasonShortcutExpansion/auto_warn_and_staff_group_never_expand`. |
| 13 | (27-04) `applyAppealURL` accepts empty string (clears) or `http`/`https` URL passing `content.ValidButtonURL` <= 2048 runes, rejecting unsafe/malformed/whitespace-only/over-length URLs without writing (TUNE-06). | ✓ VERIFIED | `modtuning_settings.go:137-169`; verified by `TestAppealURLValidationAndApply` (4/4 subtests). |
| 14 | (27-04) When an appeal link is set, ban messages (default or custom `EventBan`, `EventTimedBan`, `EventReqBan`, plus automatic ban/tban outcomes of `EventWarnLimit`, `EventFlood`, `EventBlacklist`, `EventForwardBlock`, `EventCaptcha`) and their PM copies (including refused-custom PM fallback) carry `eventmsg_btn_appeal` as a single-button final row (TUNE-06, P27-04). | ✓ VERIFIED | `event_message.go:128-168,371,438-440,542-567`, `event_message_pm.go:351-355,407-412`, `warns.go:530`, `antiflood.go:497`, `event_message_auto.go:298,372`; verified by `TestAppealButtonOnBanMessages` (10/10 subtests) and `TestManualDefaultsEndToEnd`. |
| 15 | (27-04) When no appeal link is set, `r.Opts` passes untouched to `site.SendDefault`, non-ban events make zero appeal lookups, and existing goldens stay byte-identical (MSG-11). | ✓ VERIFIED | `event_message.go:438,543-556`; verified by `TestAppealButtonOnBanMessages`, `TestLegacyModerationRequests` (17/17), `TestLegacyPhase27ModerationRequests` (17/17), `TestLegacyPunishmentNoticeRequests` (22/22). |
| 16 | (27-04) 100-button keyboard limit omits appeal button when full; `pmCopyAddDetails` preserves appeal button in `GroupRows`; Messages Full previews (group & PM) show appeal button; `EventBan` (2), `EventTimedBan` (1), `EventReqBan` (1), and PM defs (1) reserve button room (TUNE-06). | ✓ VERIFIED | `event_message.go:159-168,566`, `settings_messages.go:150,154,237,469-471,1012-1014,1071-1073`; verified by `TestAppealButtonOnBanMessages` and `TestSettingsEditReservedButtons` (5/5). |
| 17 | (27-04) Channel bans, silent bans, non-ban events, lockdown joiner bans, Ban N recent joiners, and Staff Group actions never carry an appeal button; `TestEventEngineCallers` and `TestPMCopyCallers` pass (EVT-10). | ✓ VERIFIED | `eventSupportsAppeal` rejects `r.Target.Id < 0`; `TestEventEngineCallers`, `TestPMCopyCallers`, and `TestAppealButtonOnBanMessages/staff_group_and_raid_isolation` all pass. |
| 18 | (27-05) `settingsTypedPage.WaitEndedKey` is read by `settings_input_door.go:waitEnded` with fallback to `settings_tz_wait_ended_note`. | ✓ VERIFIED | `settings_input.go:302`, `settings_input_door.go:200-204`, `settings_moddefaults.go:107`; verified by `TestSettingsModDefaults/appeal_link_cancel_command_and_expired_wait` and `TestSettingsTimezoneOpens`. |
| 19 | (27-05) Private-only Manual defaults page (`q`, Order 41, Icon `⚖️`, hidden in Staff Groups) lets owner/allowed admin configure default `/ban` & `/mute` durations, Reason required switch, and Appeal link (typed wait + Remove) via `apply*` helpers (TUNE-04, TUNE-06). | ✓ VERIFIED | `settings_moddefaults.go:62-108,324-345,434-470`; verified by `TestSettingsModDefaults` (9/9 subtests). |
| 20 | (27-05) Every page `q` press and typed reply is judged live (`settingsRightRestrict` on writes, `settingsRightAdmin` on views/Cancel, owner gate); forged/extra/group-menu presses refused in step 1 with 0 `getChatMember` calls; `<= 64`-byte callbacks (`Omitted == 0`); `1..3800` UTF-16 lengths in all 8 languages. | ✓ VERIFIED | `settings_moddefaults.go:59-100`; verified by `TestSettingsModDefaults` (`live_authority_and_owner_gate`, `forged_and_invalid_presses`, `worst_case_callback_bytes`, `all_8_languages_screen_lengths_and_labels`). |
| 21 | (27-05) All Manual defaults `settings_*` keys exist in all 8 locales, pass `TestSettingsLocaleKeys`, `TestSettingsLabelLengths`, `TestLocaleStyle`, `make check-translations`; Main grid tests updated. | ✓ VERIFIED | `fuku/i18n` locale suite, `make check-translations`, `TestSettingsMainGrid`, `TestSettingsMainPageFitsEveryLocale`, `TestSettingsFwdBlockMainShape` all pass. |
| 22 | (27-06) `settingsListTwoStep` plain-text Step 2 mode (`CheckPlain`, `ApplyPlain`) preserves `StepMsgID` redelivery guard, fresh re-check, atomic claim/finish/release, and `sendSavedList` while leaving Filters and Notes unchanged. | ✓ VERIFIED | `settings_list_input.go:115,346-352,471-512`; verified by `TestSettingsReasons`, `TestSettingsFilters*`, `TestSettingsNotes*`, `TestSettingsInput*`. |
| 23 | (27-06) Private-only Reason shortcuts list page (`rs`, Order 45, Icon `🏷`, hidden in Staff Groups, `settingsRightRestrict`, codes `sg`/`sx`/`sy`/`sa`/`sc`, parts `w`→`r`) supports browsing paged escaped `word → full text` lines, two-step plain-text add, and two-tap delete card (TUNE-05). | ✓ VERIFIED | `settings_reasons.go:151-195`; verified by `TestSettingsReasons` (7/7 subtests) and `TestManualDefaultsEndToEnd`. |
| 24 | (27-06) Reason shortcuts list safeguards hold: live authority + owner gate, foreign-chat row ID refused as gone, duplicate refused without overwrite, cap 50, `<= 64`-byte worst-case 17-digit callbacks (`Omitted == 0`), `1..3800` UTF-16 lengths in all 8 languages. | ✓ VERIFIED | Verified by `TestSettingsReasons` (all 7 subtests + 8 per-language subtests). |
| 25 | (27-06) All Reason shortcuts `settings_*` keys exist in all 8 locales, pass `TestSettingsLocaleKeys`, `TestSettingsLabelLengths`, `TestSettingsListKeysExist`, `TestLocaleStyle`, `make check-translations`; Main grid & Phase 19 guards updated. | ✓ VERIFIED | `fuku/i18n` suite, `TestSettingsListKeysExist`, `TestSettingsPhase19Codes`, `TestSettingsPhase19EndToEnd`, `TestSettingsMainPhase19`, `make check-translations` all pass. |
| 26 | (27-07) All 11 Phase 27 action codes (`md`/`mt`/`mr`/`ma`/`mc`/`mx` → `q`; `sg`/`sx`/`sy`/`sa`/`sc` → `rs`) map to their page, are no core codes, refuse extra keys and group-menu presses in step 1 with 0 `getChatMember` calls, and keep all 27 registered pages intact. | ✓ VERIFIED | `TestPhase27Codes` passes cleanly under `-race`. |
| 27 | (27-07) Opening Main, page `q`, its duration screens, and page `rs` on an unconfigured chat creates 0 rows in `chat_manual_defaults`, `chat_reason_shortcuts`, and `chats`. | ✓ VERIFIED | `TestPhase27ReadsCreateNothing` passes cleanly under `-race`. |
| 28 | (27-07) Staff Group privacy and IDs-only log sweeps prove zero Staff Group, shortcut word/reason, or appeal URL leakage into UI or logrus output. | ✓ VERIFIED | `TestPhase27StaffPrivacyAndLogs` and `TestSettingsStaffPrivacy` pass cleanly under `-race`. |
| 29 | (27-07) End-to-end dispatcher proof exercises private `/settings` configuration (`q` & `rs`) -> `/ban` refusal -> `/ban spam` default duration + escaped expanded reason in group, PM copy, log channel + Appeal button -> `/warn spam` -> `/tban 2h other` -> two-tap delete + clear defaults restoring permanent untimed ban. | ✓ VERIFIED | `TestManualDefaultsEndToEnd` passes cleanly under `-race`. |
| 30 | (27-07) `AGENTS.md` documents new tables, `skipLocal` prefixes, pages `q` & `rs`, action codes, zero new shutdown drain, golden file, and provisional decisions `P27-01..P27-05`; local phase gates (`make check-translations`, `make check-docs`, `go vet -tags testtools ./...`, `CGO_ENABLED=0 go build ./...`, `make test-shard-check`, golden & `go.mod` immutability) pass. | ✓ VERIFIED | All local commands exited 0; `AGENTS.md` lines 6962-6988 verified directly. (CI-only `make lint` and `gosec` are listed in Human Verification.) |

**Score:** 29/30 truths verified (1 present, behavior-unverified: PostgreSQL migration constraint execution)

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `fuku/modules/moderation_phase27_legacy_request_test.go` | `TestLegacyPhase27ModerationRequests`, `goldenP27*`, `phase27AssertNoUntilDate` | ✓ VERIFIED | Exists, substantive (334 lines, 17 subtests), committed alone in `71dbf865`, untouched since, passes `-race`. |
| `migrations/20261122150000_add_manual_command_tuning.sql` | `chat_manual_defaults` and `chat_reason_shortcuts` tables with inline named CHECKs | ✓ VERIFIED | Exists, 44 lines, 3 idempotent statements, inline named constraints, no FKs; PostgreSQL execution deferred to CI (`behavior_unverified`). |
| `fuku/db/models/modtuning.go` | `ChatManualDefaults`, `ChatReasonShortcut`, bounds constants | ✓ VERIFIED | Exists, 49 lines, no `default:` tag on `ReasonRequired`, SQLite check tags match migration. |
| `fuku/db/modtuning/repository.go` | `skipLocal` cached reads, non-creating `Get*Fresh`, column-targeted upserts, shortcut CRUD | ✓ VERIFIED | Exists, 316 lines, every write calls `cache.DeleteCache`, `AddShortcut` uses `ON CONFLICT DO NOTHING`, `RemoveShortcutByID` scopes by `chat_id`. |
| `fuku/modules/modtuning_settings.go` | `apply*`, `validateAppealURL`, `manualDefaultsFor`, `manualBanMember`, `manualMuteMember`, `expandManualReason` | ✓ VERIFIED | Exists, 369 lines, wired into all manual punishment handlers, `event_message.go`, `settings_moddefaults.go`, `settings_reasons.go`, IDs-only logs. |
| `fuku/modules/modtuning_commands_test.go` | `TestManualCommandDefaults` | ✓ VERIFIED | Exists, 8 top-level subtests passing `-race`. |
| `fuku/modules/modtuning_reasons_test.go` | `TestReasonShortcutValidationAndApply`, `TestReasonShortcutExpansion` | ✓ VERIFIED | Exists, 4 + 7 subtests passing `-race`. |
| `fuku/modules/event_message.go` | `eventReply.Appeal`, `eventSupportsAppeal`, `eventAppendAppealRow`, `sendDefaultPost`/`composeEventMessage` appeal attachment | ✓ VERIFIED | Exists, wired, passes all golden and appeal tests. |
| `fuku/modules/event_message_pm.go` | `eventPMOffer.AppealRow`, `composeEventPM`, `runEventPM` fallback appeal attachment | ✓ VERIFIED | Exists, wired, verified in `TestAppealButtonOnBanMessages` and `TestManualDefaultsEndToEnd`. |
| `fuku/modules/modtuning_appeal_test.go` | `TestAppealURLValidationAndApply`, `TestAppealButtonOnBanMessages` | ✓ VERIFIED | Exists, 4 + 10 subtests passing `-race`. |
| `fuku/modules/settings_input.go` | `settingsTypedPage.WaitEndedKey` | ✓ VERIFIED | Exists (`settings_input.go:302`), wired to `settings_input_door.go:200`. |
| `fuku/modules/settings_moddefaults.go` | Private Manual defaults page (`q`, Order 41, `⚖️`, codes `md`/`mt`/`mr`/`ma`/`mc`/`mx`) | ✓ VERIFIED | Exists, 470 lines, registered in `init()`, wired to `modtuning_settings.go` apply helpers. |
| `fuku/modules/settings_moddefaults_test.go` | `TestSettingsModDefaults` | ✓ VERIFIED | Exists, 9 subtests passing `-race`. |
| `fuku/modules/settings_list_input.go` | Plain-text Step 2 hook (`CheckPlain`, `ApplyPlain`) and static parts `{rs, w, 1, r}`, `{rs, r, 2}` | ✓ VERIFIED | Exists, Filters & Notes untouched (`ApplyPlain == nil`). |
| `fuku/modules/settings_reasons.go` | Private Reason shortcuts list page (`rs`, Order 45, `🏷`, codes `sg`/`sx`/`sy`/`sa`/`sc`) | ✓ VERIFIED | Exists, 195 lines, registered via `registerSettingsList`. |
| `fuku/modules/settings_reasons_test.go` | `TestSettingsReasons` | ✓ VERIFIED | Exists, 7 top-level + 8 language subtests passing `-race`. |
| `fuku/modules/settings_phase27_test.go` | `TestPhase27Codes`, `TestPhase27ReadsCreateNothing`, `TestPhase27StaffPrivacyAndLogs` | ✓ VERIFIED | Exists, 405 lines, 3/3 passing `-race`. |
| `fuku/modules/modtuning_e2e_test.go` | `TestManualDefaultsEndToEnd` | ✓ VERIFIED | Exists, 392 lines, passing `-race`. |
| `AGENTS.md` | Updated rulebook for Phase 27 tables, `skipLocal`, pages `q`/`rs`, codes, goldens, `P27-01..P27-05` | ✓ VERIFIED | Exists, updated accurately. |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | -- | --- | ------ | ------- |
| `fuku/modules/moderation_phase27_legacy_request_test.go:91` | `fuku/modules/moderation_legacy_request_test.go` | `newModerationGoldenSite(t)` & `legacyRequestJSON` | ✓ WIRED | Verified in source & passing test. |
| `fuku/db/modtuning/repository.go:25-26,54-61` | `fuku/db/cache/local.go:59-60,117-118` | `chatManualDefaultsPrefix` & `chatReasonShortcutsPrefix` in `skipLocal` | ✓ WIRED | Verified by `TestSkipLocalChatManualDefaultsKey` & `TestSkipLocalChatReasonShortcutsKey`. |
| `fuku/modules/moderation.go:334,337` | `fuku/modules/modtuning_settings.go` | `manualApplyReasonRequired` & `expandManualReason` | ✓ WIRED | Runs before `cmd.execute` for all `reasonChecked` non-channel commands. |
| `fuku/modules/bans.go:115,118,250,363,404` | `fuku/modules/modtuning_settings.go` | `manualApplyReasonRequired`, `expandManualReason`, `manualBanMember` | ✓ WIRED | Covers `/ban`, `/dban`, `/sban`, `/tban`. |
| `fuku/modules/mute.go:160,163,203,218,258` | `fuku/modules/modtuning_settings.go` | `manualApplyReasonRequired`, `expandManualReason`, `manualMuteMember` | ✓ WIRED | Covers `/mute`, `/dmute`, `/smute`, `/tmute`. |
| `fuku/modules/warns.go:614,617` | `fuku/modules/modtuning_settings.go` | `manualApplyReasonRequired` & `expandManualReason` in `dispatchWarn` only | ✓ WIRED | Never called inside `warnThisUser` (`warnSourceAuto` isolated). |
| `fuku/modules/event_message.go:438-440` | `fuku/modules/modtuning_settings.go` | `manualAppealURLFor(site.Chat.Id)` gated by `eventSupportsAppeal(r)` | ✓ WIRED | Populated once before `eventMessagesLookup`; non-ban events make 0 lookups. |
| `fuku/modules/settings_messages.go:1013,1072` | `fuku/modules/event_message.go` | `manualAppealURLFor(p.group.Id)` in `settingsMsgPreview` & `settingsMsgPMPreview` | ✓ WIRED | Full previews include stored appeal button. |
| `fuku/modules/settings_moddefaults.go:325,327,336,344,451` | `fuku/modules/modtuning_settings.go` | `applyDefaultBanSecs`, `applyDefaultMuteSecs`, `applyReasonRequired`, `applyAppealURL` | ✓ WIRED | All page `q` writes route through apply helpers. |
| `fuku/modules/settings_input_door.go:200` | `fuku/modules/settings_input.go:302` | `typed.WaitEndedKey` with `settings_tz_wait_ended_note` fallback | ✓ WIRED | Page `q` sets `settings_moddef_appeal_wait_ended`. |
| `fuku/modules/settings_reasons.go:120,134,186` | `fuku/modules/settings_list_input.go:472,508` & `modtuning_settings.go` | `settingsListTwoStep.ApplyPlain` -> `applyReasonShortcutAdd`; `Remove` -> `applyReasonShortcutRemoveByID` | ✓ WIRED | Plain-text two-step add & two-tap delete wired cleanly. |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| -------- | ------------- | ------ | ------------------ | ------ |
| `fuku/modules/settings_moddefaults.go` | `defs` (`modtuning.Defaults`) | `modtuning.GetDefaultsFresh` -> `chat_manual_defaults` | Yes | ✓ FLOWING |
| `fuku/modules/settings_reasons.go` | `entries` (`[]settingsListEntry`) | `modtuning.ListShortcutsFresh` -> `chat_reason_shortcuts` | Yes | ✓ FLOWING |
| `fuku/modules/moderation.go` / `bans.go` / `mute.go` / `warns.go` | `c.manualDefaults()`, `tgt.reason` | `modtuning.GetDefaults` / `modtuning.GetShortcuts` (`skipLocal` cached DB read) | Yes | ✓ FLOWING |
| `fuku/modules/event_message.go` / `event_message_pm.go` | `r.AppealURL`, `o.AppealRow` | `manualAppealURLFor` -> `modtuning.GetDefaults` -> `validateAppealURL` | Yes | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Translations & docs & vet & build & shard check | `make check-translations && make check-docs && go vet -tags testtools ./... && CGO_ENABLED=0 go build ./... && make test-shard-check` | All 1068 keys present across 8 locales; 0 docs drift; vet/build/shard-check exit 0 | ✓ PASS |
| DB, cache `skipLocal`, backup exclusion, i18n | `go test -tags testtools -race -count=1 ./fuku/db/modtuning ./fuku/db/cache` + `TestModTuningExcludedFromBackup` + `fuku/i18n` locale suite | All passed, 0 failures, 0 skips | ✓ PASS |
| Migration statements (offline) & PG integrity | `go test -tags testtools -race -count=1 -run '^TestManualCommandTuning' -v ./fuku/db/migrations` | `TestManualCommandTuningMigrationStatements` PASS; `TestManualCommandTuningMigrationDefaults` SKIP (no `DATABASE_URL`) | ? SKIP (PG) |
| Focused Phase 27 module tests, goldens, EVT-10 caller guards | `go test -tags testtools -race -count=1 -v -run '^(TestLegacyPhase27ModerationRequests\|TestLegacyModerationRequests\|TestManualCommandDefaults\|TestReasonShortcutValidationAndApply\|TestReasonShortcutExpansion\|TestAppealURLValidationAndApply\|TestAppealButtonOnBanMessages\|TestSettingsModDefaults\|TestSettingsReasons\|TestPhase27Codes\|TestPhase27ReadsCreateNothing\|TestPhase27StaffPrivacyAndLogs\|TestManualDefaultsEndToEnd\|TestEventEngineCallers\|TestPMCopyCallers\|TestSettingsListKeysExist\|TestSettingsFwdBlockMainShape\|TestSettingsEditReservedButtons\|TestLegacyPunishmentNoticeRequests\|TestSettingsTimezoneOpens\|TestSettingsTimezoneFits)$' ./fuku/modules` | All passed (4.43s), 0 failures, 0 skips | ✓ PASS |
| Main, Phase 19, Filters, Notes, Input, StaffPrivacy regressions | `go test -tags testtools -race -count=1 -v -run '^(TestSettingsMain\|TestSettingsPhase19\|TestSettingsFilters\|TestSettingsNotes\|TestSettingsInput\|TestSettingsStaffPrivacy)' ./fuku/modules` | All passed (12.35s), 0 failures, 0 skips | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
| ----------- | -------------- | ----------- | ------ | -------- |
| **TUNE-04** | 27-01, 27-02, 27-05, 27-07 | Manual commands have per-group defaults: a default duration for `/mute` and `/ban` given without one, and a "reason required" switch that refuses a punishment with no reason. | ✓ SATISFIED | `modtuning_settings.go`, `moderation.go`, `bans.go`, `mute.go`, `warns.go`, `settings_moddefaults.go`; `TestManualCommandDefaults`, `TestSettingsModDefaults`, `TestManualDefaultsEndToEnd`. |
| **TUNE-05** | 27-01, 27-03, 27-06, 27-07 | Reason shortcuts: the owner saves short words that expand to a full reason text when used as the reason of a manual punishment. | ✓ SATISFIED | `modtuning_settings.go`, `settings_list_input.go`, `settings_reasons.go`; `TestReasonShortcutValidationAndApply`, `TestReasonShortcutExpansion`, `TestSettingsReasons`, `TestManualDefaultsEndToEnd`. |
| **TUNE-06** | 27-01, 27-04, 27-05, 27-07 | An appeal button: the owner sets one link or contact per group, and it is added to ban messages and their PM copies when set. | ✓ SATISFIED | `modtuning_settings.go`, `event_message.go`, `event_message_pm.go`, `settings_messages.go`, `settings_moddefaults.go`; `TestAppealURLValidationAndApply`, `TestAppealButtonOnBanMessages`, `TestSettingsModDefaults`, `TestManualDefaultsEndToEnd`. |
| **MSG-11** (cross-cutting) | 27-01..27-07 | Every existing reply and notice is byte-identical for a group with no custom setting, proved by immutable characterization tests recorded before production changes. | ✓ SATISFIED | `moderation_phase27_legacy_request_test.go` committed alone in `71dbf865` and untouched; `moderation_legacy_request_test.go` untouched; both golden suites + `TestLegacyPunishmentNoticeRequests` pass under `-race`. |
| **EVT-10** (cross-cutting) | 27-02..27-07 | Staff Group actions, lockdown alerts, raid-scale removals, log channel posts, and captcha challenges are unchanged. | ✓ SATISFIED | `TestEventEngineCallers`, `TestPMCopyCallers`, `TestPhase27StaffPrivacyAndLogs`, `TestSettingsStaffPrivacy` pass; zero `modtuning` references in `fuku/modules/staff*.go`. |

**Orphaned requirements check:** Zero orphaned requirements mapped to Phase 27 in `.planning/REQUIREMENTS.md` (only `TUNE-04`, `TUNE-05`, `TUNE-06` map to Phase 27, and all three are claimed and satisfied).

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| — | — | Zero `TBD`/`FIXME`/`XXX` debt markers and zero Phase 27 `TODO`/`HACK`/`PLACEHOLDER` markers or stubs found across all 49 changed files (`git diff b7393953..HEAD`). Three pre-existing `# TODO: Translate` comments in `locales/{fr,pt,ru}.yml` are outside Phase 27 diffs. | ℹ️ Info | None. |

### Human Verification Required

1. **PostgreSQL Integrity Suite (`make test-postgres-integrity`)**
   - **Test:** With PostgreSQL running and `FUKU_TEST_DATABASE=true DATABASE_URL=...`, execute `make test-postgres-integrity` (covering `TestManualCommandTuningMigrationDefaults` and `TestManualDefaultsAndShortcutsCRUD`).
   - **Expected:** Migration `20261122150000_add_manual_command_tuning.sql` applies twice idempotently, default columns read zero, out-of-range CHECK and duplicate UNIQUE violations fail with SQLSTATE `23514` / `23505` naming the inline constraints, and repository CRUD passes under `-race`.
   - **Why human:** No PostgreSQL service is configured in this local environment (`DATABASE_URL` unset), so `TestManualCommandTuningMigrationDefaults` skipped.

2. **CI Static Analysis (`make lint` & `gosec`)**
   - **Test:** Run `make lint` and `gosec ./...` in CI or an environment with `golangci-lint` and `gosec` installed.
   - **Expected:** Exit 0 with zero new lint or security findings on Phase 27 code.
   - **Why human:** `golangci-lint` and `gosec` are not installed locally.

3. **Live Telegram End-to-End Check**
   - **Test:** In a real supergroup and the bot's private chat, open `/settings` → **Manual defaults** (`q`) and **Reason shortcuts** (`rs`) (confirming group `/settings` shows only the open-in-private note and Staff Group Main hides both); configure default `/ban` and `/mute` durations, enable Reason required, set an Appeal URL, and add a reason shortcut (`spam` → full text); run `/ban` without a reason (refused), `/ban @user spam` (applies default duration, expands reason in group reply, PM copy, and log channel, attaches clickable Appeal button on group reply and PM copy), `/warn @user spam`, `/tban @user 2h other` (explicit duration + verbatim word); remove the shortcut behind the two-tap card and clear all defaults.
   - **Expected:** Clean visual layout, valid Telegram keyboards and clickable Appeal URL buttons, expected durations/refusals/expansions, and restored permanent untimed defaults after clearing.
   - **Why human:** Live Telegram API rendering, button interaction, and UX feel cannot be tested offline.

4. **Owner Confirmation of Provisional Decisions (`P27-01`..`P27-05`)**
   - **Test:** Review `P27-01`..`P27-05` in `AGENTS.md` (lines 6967-6987): default duration coverage on `/dban`/`/sban`/`/dmute`/`/smute` (`P27-01`), `reason_required` coverage across all 14 manual user-punishment commands (`P27-02`), shortcut word/text bounds and exact single-word matching (`P27-03`), appeal button inclusion on automatic ban/tban events (`P27-04`), and `PrivateOnly`/`settingsRightRestrict`/backup-exclusion policy (`P27-05`).
   - **Expected:** Owner confirms defaults or identifies any scope adjustment desired.
   - **Why human:** `workflow.skip_discuss=true` deferred owner sign-off on provisional decisions to verification.

### Gaps Summary

No code, test, wiring, translation, documentation, or requirement gaps were found. All 29 locally verifiable must-haves, all 4 ROADMAP success criteria, all requirements (`TUNE-04`, `TUNE-05`, `TUNE-06`), and cross-cutting guards (`MSG-11`, `EVT-10`) passed verification. Overall status is `human_needed` solely for PostgreSQL execution of `TestManualCommandTuningMigrationDefaults`, CI `make lint`/`gosec`, live Telegram UAT, and owner confirmation of provisional decisions `P27-01..P27-05`.

---

_Verified: 2026-11-23T03:54:10Z_
_Verifier: the agent (gsd-verifier)_
