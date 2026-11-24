---
phase: 28-advanced-thresholds
verified: 2026-11-23T13:07:51Z
status: passed
score: 24/25 must-haves verified
covered_files:
  - .planning/phases/28-advanced-thresholds/28-01-PLAN.md
  - .planning/phases/28-advanced-thresholds/28-01-SUMMARY.md
  - .planning/phases/28-advanced-thresholds/28-02-PLAN.md
  - .planning/phases/28-advanced-thresholds/28-02-SUMMARY.md
  - .planning/phases/28-advanced-thresholds/28-03-PLAN.md
  - .planning/phases/28-advanced-thresholds/28-03-SUMMARY.md
  - .planning/phases/28-advanced-thresholds/28-04-PLAN.md
  - .planning/phases/28-advanced-thresholds/28-04-SUMMARY.md
  - .planning/phases/28-advanced-thresholds/28-05-PLAN.md
  - .planning/phases/28-advanced-thresholds/28-05-SUMMARY.md
  - .planning/phases/28-advanced-thresholds/28-06-PLAN.md
  - .planning/phases/28-advanced-thresholds/28-06-SUMMARY.md
  - AGENTS.md
  - Makefile
  - fuku/db/advthresholds/repository.go
  - fuku/db/advthresholds/repository_test.go
  - fuku/db/advthresholds/testmain_test.go
  - fuku/db/backup/advthresholds_backup_test.go
  - fuku/db/backup/testmain_test.go
  - fuku/db/cache/local.go
  - fuku/db/cache/local_test.go
  - fuku/db/migrations/advanced_thresholds_postgres_test.go
  - fuku/db/models/advthresholds.go
  - fuku/i18n/settings_locale_test.go
  - fuku/modules/advthresholds.go
  - fuku/modules/advthresholds_ai_mreq_test.go
  - fuku/modules/advthresholds_e2e_test.go
  - fuku/modules/advthresholds_lockdown_captcha_test.go
  - fuku/modules/advthresholds_phase28_legacy_request_test.go
  - fuku/modules/advthresholds_raid_test.go
  - fuku/modules/captcha.go
  - fuku/modules/lockdown_alert.go
  - fuku/modules/memberreq_ai.go
  - fuku/modules/memberreq_limits.go
  - fuku/modules/memberreq_watch.go
  - fuku/modules/raid_ai.go
  - fuku/modules/raid_ai_count.go
  - fuku/modules/raid_ai_image.go
  - fuku/modules/raid_counters.go
  - fuku/modules/raid_flood.go
  - fuku/modules/raid_join.go
  - fuku/modules/settings_advthresholds.go
  - fuku/modules/settings_advthresholds_test.go
  - fuku/modules/settings_captcha.go
  - fuku/modules/settings_fwdblock_test.go
  - fuku/modules/settings_main_test.go
  - fuku/modules/settings_phase19_test.go
  - fuku/modules/settings_phase28_test.go
  - fuku/modules/test_harness_test.go
  - locales/en.yml
  - locales/es.yml
  - locales/fr.yml
  - locales/hi.yml
  - locales/id.yml
  - locales/pt.yml
  - locales/ru.yml
  - locales/uk.yml
  - migrations/20261123120000_add_advanced_thresholds.sql
covered_digest: "v4:sha256:a1fdd899fb9f6e51c4c850f67a2edd757361d29713d9899e6f205799d70a3cb5"
behavior_unverified: 1
overrides_applied: 0
behavior_unverified_items:
  - truth: "Migration 20261123120000_add_advanced_thresholds.sql idempotently creates chat_advanced_thresholds on PostgreSQL with all 12 integer columns defaulting to 0 and 12 inline named CHECK constraints rejecting out-of-range values with SQLSTATE 23514."
    test: "FUKU_TEST_DATABASE=true DATABASE_URL=... make test-postgres-integrity (specifically TestAdvancedThresholdsMigrationDefaults and TestAdvancedThresholdsCRUD)"
    expected: "Migration applies cleanly twice in one transaction on PostgreSQL, default row reads all zeros, invalid values below min or above max fail with SQLSTATE 23514 naming each chk_chat_adv_* constraint, and boundary values succeed."
    why_human: "No PostgreSQL instance is configured locally (DATABASE_URL unset); TestAdvancedThresholdsMigrationDefaults skipped. Structural DDL checks (TestAdvancedThresholdsMigrationStatements) and SQLite CRUD/bounds checks (TestAdvancedThresholdsCRUD) passed, but PostgreSQL constraint execution was not exercised."
human_verification:
  - test: "Run PostgreSQL integrity tests (FUKU_TEST_DATABASE=true DATABASE_URL=... make test-postgres-integrity) covering TestAdvancedThresholdsMigrationDefaults and TestAdvancedThresholdsCRUD."
    expected: "Migration 20261123120000_add_advanced_thresholds.sql runs idempotently twice on PostgreSQL, enforces all 12 inline named CHECK constraints (SQLSTATE 23514), and column-targeted upserts + area/field/all resets pass under -race."
    why_human: "No PostgreSQL server is available in this environment (DATABASE_URL unset), so TestAdvancedThresholdsMigrationDefaults skipped."
  - test: "Run CI static checks (make lint and gosec) on this tree."
    expected: "Both golangci-lint and gosec exit 0 with zero new findings on Phase 28 files."
    why_human: "Neither golangci-lint nor gosec is installed in this local environment."
  - test: "Perform live Telegram UAT in a real supergroup + bot private chat: open private /settings -> Advanced (av) (confirm group /settings shows Private only and opening av shows settings_adv_group_note, and Staff Group Main hides av); inspect all 6 areas and verify each threshold shows current, default, and allowed range; set a custom flood window (e.g. 120 s), lockdown reminder interval (e.g. 15 min), and extended captcha timeout (e.g. 30 min); verify /captcha status and Captcha settings page display 30 min while /captchatime 11 is still refused; trigger a joiner captcha and confirm the 30-minute challenge text and expiry; reset one item and one area and confirm defaults are restored on the next event."
    expected: "Every screen, button label, and ✅ active indicator renders cleanly in Telegram, extended captcha timeout applies to the next joiner, custom thresholds act immediately across replicas, and resets cleanly restore today's defaults."
    why_human: "Visual rendering, live Telegram API keyboard acceptance, multi-replica Redis timing, and end-to-end UX cannot be verified offline."
  - test: "Confirm or request changes to provisional owner decisions P28-01 through P28-04 documented in AGENTS.md."
    expected: "Owner signs off on: P28-01 (0 = unset/default, curated safe bounds for all 12 thresholds, selecting a threshold's default preset normalizes storage to 0), P28-02 (Option A extended captcha timeout: chat_advanced_thresholds.captcha_timeout_mins 0 or 11..60 overrides sendCaptcha and /captcha + settings_captcha display while leaving captcha_settings.timeout 1..10 CHECK, /captchatime, the 1..10 stepper, and backup export untouched), P28-03 (page av is PrivateOnly, hidden in Staff Groups, live settingsRightAdmin + owner gate for views, live settingsRightRestrict for writes/resets, excluded from backup/export/import/reset until Phase 30), and P28-04 (transient advThresholdsLookup errors log one IDs-only Warn line and fall back to compiled defaults)."
    why_human: "Discuss phase was skipped via workflow.skip_discuss=true, so owner-noticeable defaults and authority boundaries require explicit owner confirmation."
---

# Phase 28: Advanced Thresholds Verification Report

**Phase Goal:** As the owner, I can tune the detection and screening limits that are fixed in code today, per group, inside safe bounds.
**Verified:** 2026-11-23T13:07:51Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### ROADMAP Success Criteria Rollup

| # | Roadmap Success Criterion | Status | Evidence |
| - | ------------------------- | ------ | -------- |
| SC-1 | A private Advanced page lists the areas (raid windows, new-member share and age, AI spam bars, member requirement cap and pause, lockdown reminder interval, captcha timeout). Each value shows its current setting, its default and its allowed range. | ✓ VERIFIED | `fuku/modules/settings_advthresholds.go` registers page `av` (`Order: 81`, `Icon: "🎛"`, `PrivateOnly: true`, `Hidden: settingsAdvHidden`) with 6 areas (`settingsAdvAreas`) and 12 items (`settingsAdvItems`); `renderSettingsAdvArea` (`settings_adv_item_line`) and `renderSettingsAdvItem` (`settings_adv_item_detail`) render `value`, `default`, and `range` for every item; verified by `TestSettingsAdvancedThresholds/hub_area_and_item_display` and `worst_case_callbacks_and_8_locales`. |
| SC-2 | A value can only be set inside its bounds. An out-of-range, stale or forged press changes nothing. Reset to default puts today's constant back, for one value or one area. | ✓ VERIFIED | Dual bounds enforcement: step-1 `private(...)` + `settingsAdvPresetValue` (`settings_advthresholds.go:213-266,299-312`), Go bounds validation in `applyAdvancedThreshold` (`advthresholds.go:161-177`) and `advthresholds.SetField` (`repository.go:374-385`), and 12 inline SQL `CHECK` constraints (`20261123120000_add_advanced_thresholds.sql:32-43`). Per-item (`ar`) and per-area (`ax`) resets zero columns via `resetAdvancedThreshold`/`resetAdvancedArea`. Verified by `TestSettingsAdvancedThresholds` (`set_preset_default_normalization_and_resets`, `forged_and_out_of_range_presses_refused_in_step1`), `TestPhase28Codes`, `TestAdvancedThresholdsCRUD`, and `TestAdvancedThresholdsEndToEnd`. |
| SC-3 | A changed value acts at the next relevant event on every replica. For example, a new flood window changes what counts as a flood, a new reminder interval changes when the next lockdown reminder posts, and a captcha timeout above 10 minutes applies to the next joiner. | ✓ VERIFIED | `chat_advanced_thresholds:` registered in `fuku/db/cache/local.go:62,125` `skipLocal`; every write calls `cache.DeleteCache(CacheKey(row.ChatId))` after commit (`repository.go:416,471`); all 12 runtime sites wired through `advThresholdsFor` (`raid_counters.go`, `raid_join.go`, `raid_flood.go`, `raid_ai.go`, `raid_ai_count.go`, `raid_ai_image.go`, `memberreq_ai.go`, `memberreq_watch.go`, `memberreq_limits.go`, `lockdown_alert.go`, `captcha.go`, `settings_captcha.go`). Verified by `TestSkipLocalChatAdvancedThresholdsKey`, `TestAdvancedThresholdsRaidWiring`, `TestAdvancedThresholdsAIAndMemberReq`, `TestAdvancedThresholdsLockdownAndCaptcha`, and `TestAdvancedThresholdsEndToEnd`. |
| SC-4 | A group that changes nothing behaves exactly as today. | ✓ VERIFIED | Missing row / `0` columns fall back to compiled constants; `TestAdvancedThresholdDefaultsMatchConstants` pins all 12 defaults against compiled module constants; `TestLegacyPhase28DefaultThresholds` (committed alone in `48d5c240` before any Phase 28 production edit and untouched since) passes 8/8 subtests under `-race`. |

### Observable Truths

| # | Truth | Status | Evidence |
| - | ----- | ------ | -------- |
| 1 | (28-01) Characterization golden records on unchanged code at `654c8311` exact default behaviors for all 12 thresholds across 8 subtests (`TestLegacyPhase28DefaultThresholds`, MSG-11). | ✓ VERIFIED | `fuku/modules/advthresholds_phase28_legacy_request_test.go` passes 8/8 subtests under `-tags testtools -race -count=1` in 0.07 s. |
| 2 | (28-01) All golden subtests run in-process without localhost TCP dials (`phase28InProcessMiniredis` via `net.Pipe()` + `mr.Server().ServeConn`) under `-race`. | ✓ VERIFIED | Executed inside the sandbox (where localhost TCP connect is denied) and passed 8/8 subtests cleanly under `-race`. |
| 3 | (28-01) Golden file committed alone in `48d5c240` before any Phase 28 production edit and untouched since; pre-existing goldens and `go.mod`/`go.sum` untouched. | ✓ VERIFIED | `git log --oneline 654c8311..HEAD -- fuku/modules/advthresholds_phase28_legacy_request_test.go` prints 1 commit (`48d5c240`, 1 file changed); `git log 654c8311..48d5c240 -- fuku migrations ':!*_test.go'` is empty; `git diff 654c8311..HEAD -- go.mod go.sum` and pre-existing `*_legacy_request_test.go` are empty. |
| 4 | (28-02) Migration `20261123120000_add_advanced_thresholds.sql` idempotently creates `chat_advanced_thresholds` (`chat_id` PK, no FK on `chats`, 12 integer columns defaulting to `0`, `updated_by`, `updated_at`) with 12 inline named SQL CHECK constraints matching every Go bound. | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | DDL inspected (`migrations/20261123120000_add_advanced_thresholds.sql:15-44`), `TestAdvancedThresholdsMigrationStatements` passes (idempotent statement, 12 inline named `chk_chat_adv_*` CHECKs, no FKs), and SQLite bounds/CRUD pass (`TestAdvancedThresholdsCRUD`), but PostgreSQL execution (`TestAdvancedThresholdsMigrationDefaults`) skipped locally (`DATABASE_URL` unset) — see Human Verification. |
| 5 | (28-02) `chat_advanced_thresholds:` is listed in `fuku/db/cache/local.go` `skipLocal`; `Get`/`GetFresh` return zero values and create 0 rows when absent; column-targeted upserts validate bounds in Go before writing and drop cache via `cache.DeleteCache`; the table is excluded from and untouched by backup export/import/clear. | ✓ VERIFIED | `local.go:62,125`, `repository.go:325-472`; verified by `TestSkipLocalChatAdvancedThresholdsKey`, `TestAdvancedThresholdsCRUD` (4/4 subtests), and `TestAdvThresholdsExcludedFromBackup`. |
| 6 | (28-02) Drift guard test (`TestAdvancedThresholdDefaultsMatchConstants`) compares all 12 defaults in `fuku/db/models/advthresholds.go` / `fuku/db/advthresholds` against real compiled code constants and fails if any pair drifts. | ✓ VERIFIED | `advthresholds_raid_test.go:41-94` checks all 12 `models.Default*` constants, `advthresholds.AllSpecs()` invariants, and zero-`Thresholds` resolved helpers; passes under `-race`. |
| 7 | (28-02) All 5 `raidRecentWindow` sites (`recordRaidJoins`, `recordRaidFloodItem`, `raidSnapshotFlooders`, `raidAICheckCandidate`, `raidSnapshotAISpammers`), join surge window, flood window/share/account-floor, and AI burst window read cached `advthresholds.Thresholds` with derived TTLs (`joinedTTL`, `floodTTL`, `aiTTL`), 0 extra DB round trips on warm hot-path reads, and fail-to-default on transient read errors (P28-04). | ✓ VERIFIED | Direct code audit confirms zero production callers bypass `*With`/`*For` helpers (`raid_counters.go:100-124`, `raid_join.go:187-199`, `raid_flood.go:253-360,409-415`, `raid_ai.go:142-147`, `raid_ai_count.go:93-107,196-205,253-260`); verified by `TestAdvancedThresholdsRaidWiring` (5/5 subtests). |
| 8 | (28-03) `raidAIProcessImage` compares `result.probability >= advRaidAIImageBar(advThresholdsFor(ctx, job.chat.Id))` on fresh and cached probabilities in `fuku:raid:aiv:*`. | ✓ VERIFIED | `raid_ai_image.go:194-195`; verified by `TestAdvancedThresholdsAIAndMemberReq/raid_ai_image_bar_cached_and_fresh`. |
| 9 | (28-03) `memberReqAIJudge` returns raw `Probability` (`HasProbability: true`, `Fresh`) from the shared cross-chat singleflight (`n:<key>`) without baking a chat-specific verdict, and `memberReqAIRule` applies each caller's own `advMemberReqAIBar` via `memberReqAIVerdictOfBar`. | ✓ VERIFIED | `memberreq_ai.go:386-429,506-554`; verified by `TestAdvancedThresholdsAIAndMemberReq/memberreq_ai_bar_cross_chat_singleflight` (1 provider call, Chat A breach at 0.70, Chat B pass at 0.95) and all existing `TestMemberReqAI*` tests. |
| 10 | (28-03) `memberReqLookupCachedWith` reads `advMemberReqAIBar` only when `rules.AI` is true (0 threshold reads when AI rule is off) and calls `aiRead.satisfiedBar(aiBar)` -> `memberReqAIWatchSatisfiedBar`. | ✓ VERIFIED | `memberreq_watch.go:253-261,318-323`; verified by `TestAdvancedThresholdsAIAndMemberReq/memberreq_ai_watcher_fast_path_respects_lowered_bar`. |
| 11 | (28-03) `memberReqCapAllow` and `memberReqPauseChatWith` use `advMemberReqCapPerMin` and `advMemberReqPause` (preserving package-var seams `memberReqCapPerMinute` and `memberReqPause` when stored value is 0), setting Redis `SetNX` TTL and formatting `mreq_notice_paused` with effective values. | ✓ VERIFIED | `memberreq_limits.go:93-131`, `advthresholds.go:124-138`; verified by `TestAdvancedThresholdsAIAndMemberReq` (`memberreq_cap_and_pause_custom`, `memberreq_cap_and_pause_preserves_package_var_seams_when_unset`). |
| 12 | (28-04) Both `lockdownReminderSeq` sites in `fuku/modules/lockdown_alert.go` (`lockdownPlanReminders` and `lockdownPostAlertSlot`) use `lockdownReminderIntervalFor` + `lockdownReminderSeqFor` (preserving `lockdownReminderEvery` seam when 0). | ✓ VERIFIED | `lockdown_alert.go:375-396,438-439,789-790`; verified by `TestAdvancedThresholdsLockdownAndCaptcha` (3 reminder subtests) and `TestLockdownReminders`. |
| 13 | (28-04) `effectiveCaptchaTimeout` implements Option A (P28-02): overrides `timeout` in `sendCaptcha` (`ExpiresAt`, Turnstile challenge TTL, timeout timer, challenge text) and displays effective timeout in `/captcha` status and `settings_captcha` body while leaving `captcha_settings.timeout` (1..10 CHECK), `/captchatime`, the 1..10 stepper, and backup export untouched. | ✓ VERIFIED | `captcha.go:496-503,826-828,1300-1353,1393,1429`, `settings_captcha.go:85,149-178,211,313`; verified by `TestAdvancedThresholdsLockdownAndCaptcha` (`captcha_timeout_extended_30m_and_reset`, `captcha_command_still_rejects_above_10`), `TestSettingsCaptcha*`, and `TestCaptchaSubcommandsRejectInvalidValues`. |
| 14 | (28-04) EVT-10 holds: neither lockdown reminders nor captcha challenges are routed through the event message engine or PM copies. | ✓ VERIFIED | `TestEventEngineCallers`, `TestPMCopyCallers`, and `TestMemberDeletionExcludedMessages` all pass under `-race`. |
| 15 | (28-05) Private-only Advanced settings page (`av`, Order `81`, Icon `🎛`, `settings_page_advanced`, `PrivateOnly: true`, `Hidden` in Staff Groups) appears in Main between `m` (80) and `d` (85); in a group menu Main shows `Private only` with 0 DB reads and opening `av` shows `settings_adv_group_note`. | ✓ VERIFIED | `settings_advthresholds.go:216-225,325-346`; verified by `TestSettingsAdvancedThresholds` (`hub_area_and_item_display`, `live_authority_and_owner_gate`, `staff_group_hides_advanced_page`). |
| 16 | (28-05) Advanced hub (`av`) lists all 6 areas with Default / `{n} custom` status; each area (`aa`) lists every threshold with current, default, and allowed range plus Reset area (`ax`); each item (`ao`) shows description, current/default/range, curated presets (active prefixed `✅ `), and Reset to default (`ar`). | ✓ VERIFIED | `settings_advthresholds.go:68-204,396-513`; verified by `TestSettingsAdvancedThresholds` (`hub_area_and_item_display`, `set_preset_default_normalization_and_resets`). |
| 17 | (28-05) Every action code (`aa`, `ao`, `as`, `ar`, `ax`) validates `fields["c"] != ""` and allow-listed area/item/canonical-preset values in step 1 (0 `getChatMember` calls and 0 DB reads on forged/out-of-range/group-menu presses); views require live `settingsRightAdmin` + owner gate, writes (`as`, `ar`, `ax`) require live `settingsRightRestrict` (P28-03). | ✓ VERIFIED | `settings_advthresholds.go:213-266,299-312`; verified by `TestSettingsAdvancedThresholds` (`live_authority_and_owner_gate`, `forged_and_out_of_range_presses_refused_in_step1`). |
| 18 | (28-05) Worst-case callback payloads (`c=-1009999999999`) fit strictly under Telegram's 64-byte cap; all 56 new locale keys exist in all 8 locales passing `TestSettingsLocaleKeys`, `TestSettingsLabelLengths`, `TestLocaleStyle`, `TestLocalePlaceholdersMatchOtherLanguages`, and `make check-translations`. | ✓ VERIFIED | `fuku/i18n` test suite, `make check-translations` (1068/1068 keys across 8 locales), and `TestSettingsAdvancedThresholds/worst_case_callbacks_and_8_locales` all pass. |
| 19 | (28-05) `settings_main_test.go`, `settings_fwdblock_test.go`, and `settings_phase19_test.go` are updated for page `av` (`settingsMainOwnerRows = 16`, `staffOwnerRows = settingsMainOwnerRows - 3`) and all Main / Staff Group / Phase-19 / Phase-27 grid & code tests pass. | ✓ VERIFIED | `TestSettingsMainGrid`, `TestSettingsMainPhase18`, `TestSettingsMainPhase19`, `TestSettingsFwdBlockMainShape`, `TestSettingsPhase19Codes`, and `TestPhase27Codes` all pass under `-race`. |
| 20 | (28-06) All 5 Phase 28 action codes (`aa`, `ao`, `as`, `ar`, `ax`) map to `av`, are no core codes, keep all 28 registered pages intact, and refuse extra keys and group-menu presses (`u`) in step 1 with 0 `getChatMember` calls. | ✓ VERIFIED | `TestPhase28Codes` passes cleanly under `-race`. |
| 21 | (28-06) Opening Main, `av`, all 6 area screens, and all 12 item screens on an unconfigured chat (private and group) creates 0 rows in `chat_advanced_thresholds` and `chats`. | ✓ VERIFIED | `TestPhase28ReadsCreateNothing` passes cleanly under `-race`. |
| 22 | (28-06) Staff Group privacy and IDs-only log sweeps prove no Phase 28 screen, refusal, or log line ever names a linked Staff Group or user/group title, and `av` never appears in a Staff Group's Main. | ✓ VERIFIED | `TestPhase28StaffPrivacyAndLogs` and `TestSettingsStaffPrivacy` pass cleanly under `-race`. |
| 23 | (28-06) End-to-end dispatcher proof drives private `/settings` callbacks into live flood lockdown (`TriggerWindow == 120`), lockdown reminder at 16 min (`lrm=15`), extended captcha timeout (`ctm=30` -> `ExpiresAt` ~30 m + `"30"` challenge), and area/item resets restoring default behavior. | ✓ VERIFIED | `TestAdvancedThresholdsEndToEnd` passes cleanly under `-race` in 0.91 s (zero localhost TCP dials). |
| 24 | (28-06) `AGENTS.md` documents `chat_advanced_thresholds`, `chat_advanced_thresholds:` in `skipLocal`, zero new shutdown drain, page `av` (`aa`/`ao`/`as`/`ar`/`ax`), `TestLegacyPhase28DefaultThresholds`, `TestAdvancedThresholdDefaultsMatchConstants`, and provisional owner decisions `P28-01..P28-04`. | ✓ VERIFIED | Verified directly in `AGENTS.md:154-156,1307-1327,3305-3315,7023-7056`. |
| 25 | (28-06) Local phase gates (`make check-translations`, `make check-docs`, `go vet -tags testtools ./...`, `CGO_ENABLED=0 go build ./...`, `make test-shard-check`), `go.mod`/`go.sum` immutability, and golden immutability pass. | ✓ VERIFIED | All five commands exited `0`; `go.mod`/`go.sum` unchanged; `advthresholds_phase28_legacy_request_test.go` has 1 commit (`48d5c240`). (CI-only `make lint` and `gosec` are listed in Human Verification.) |

**Score:** 24/25 truths verified (1 present, behavior-unverified: PostgreSQL migration constraint execution)

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `fuku/modules/advthresholds_phase28_legacy_request_test.go` | `TestLegacyPhase28DefaultThresholds`, `phase28InProcessMiniredis` | ✓ VERIFIED | Exists (679 lines, 8 subtests), committed alone in `48d5c240`, untouched since, passes under `-race` with zero TCP dials. |
| `migrations/20261123120000_add_advanced_thresholds.sql` | `chat_advanced_thresholds` table with 12 inline named CHECKs | ✓ VERIFIED | Exists (44 lines, 1 idempotent `CREATE TABLE IF NOT EXISTS`, 12 `chk_chat_adv_*` CHECKs, no FKs); PostgreSQL execution deferred to CI (`behavior_unverified`). |
| `fuku/db/models/advthresholds.go` | `ChatAdvancedThresholds`, exported `Default`/`Min`/`Max` constants | ✓ VERIFIED | Exists (84 lines), SQLite check tags match all 12 SQL CHECKs, drift-checked by `TestAdvancedThresholdDefaultsMatchConstants`. |
| `fuku/db/advthresholds/repository.go` | `Area`/`Field`/`FieldSpec`, `Thresholds`, `skipLocal`-cached `Get`, non-creating `GetFresh`, `SetField`/`ResetField`/`ResetArea`/`ResetAll` | ✓ VERIFIED | Exists (473 lines), Go bounds validated before DB write, column-targeted upserts, `cache.DeleteCache` on every write. |
| `fuku/db/cache/local.go` | `chatAdvancedThresholdsPrefix` in `skipLocal` | ✓ VERIFIED | Exists (`local.go:62,125`), verified by `TestSkipLocalChatAdvancedThresholdsKey`. |
| `fuku/modules/advthresholds.go` | `advThresholdsLookup`, `advThresholdsFor`, 12 resolved helpers, `applyAdvancedThreshold`, `resetAdvancedThreshold`, `resetAdvancedArea` | ✓ VERIFIED | Exists (201 lines), bounded 2 s timeout, fail-to-default with IDs-only Warn log (P28-04), default normalization to `0` (P28-01). |
| `fuku/modules/raid_counters.go`, `raid_join.go`, `raid_flood.go`, `raid_ai.go`, `raid_ai_count.go` | Per-group raid windows, flood share/floor, derived TTLs, all 5 `raidRecentWindow` sites | ✓ VERIFIED | Wired across all 5 `raidRecentWindow` sites + join/flood/AI windows with derived TTLs; zero prod callers bypass `*With`/`*For`. |
| `fuku/modules/raid_ai_image.go`, `memberreq_ai.go`, `memberreq_watch.go`, `memberreq_limits.go` | Per-group AI bars (singleflight-safe & watcher-safe) and memberreq cap/pause preserving seams | ✓ VERIFIED | All 4 files wired and verified by `TestAdvancedThresholdsAIAndMemberReq` and `TestMemberReqAI*`. |
| `fuku/modules/lockdown_alert.go`, `captcha.go`, `settings_captcha.go` | Both reminder sequence sites + Option A extended captcha timeout | ✓ VERIFIED | Wired across `lockdownPlanReminders`, `lockdownPostAlertSlot`, `SendCaptcha`, `/captcha`, and `settings_captcha`; `captcha_settings.timeout` 1..10 contract untouched. |
| `fuku/modules/settings_advthresholds.go` | Private Advanced settings page `av` (`aa`/`ao`/`as`/`ar`/`ax`), 6 areas / 12 items | ✓ VERIFIED | Exists (610 lines), `PrivateOnly: true`, `Hidden: settingsAdvHidden`, step-1 `private(...)` validation, `settingsRightRestrict` on writes. |
| `locales/{en,es,fr,hi,id,pt,ru,uk}.yml` & `fuku/i18n/settings_locale_test.go` | `settings_page_advanced` and all `settings_adv_*` keys in 8 languages | ✓ VERIFIED | All 8 locales complete; `make check-translations` and all 4 `fuku/i18n` test suites pass. |
| `fuku/modules/settings_advthresholds_test.go`, `settings_phase28_test.go`, `advthresholds_e2e_test.go`, `advthresholds_raid_test.go`, `advthresholds_ai_mreq_test.go`, `advthresholds_lockdown_captcha_test.go` | Full unit, UI, security, privacy, drift-guard, and E2E dispatcher test coverage | ✓ VERIFIED | All suites exist, are substantive, and pass under `-race` with zero localhost TCP dials. |
| `AGENTS.md` & `Makefile` | Rulebook entries (`P28-01..P28-04`, zero drain, schema, `skipLocal`, page `av`) and `test-postgres-integrity` wiring | ✓ VERIFIED | `AGENTS.md` updated cleanly; `Makefile` includes `TestAdvancedThresholdsMigrationDefaults`, `TestAdvancedThresholdsCRUD`, and `./fuku/db/advthresholds`. |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | -- | --- | ------ | ------- |
| `fuku/db/advthresholds/repository.go` | `fuku/db/cache/local.go` `skipLocal` | `CacheKey(chatID)` (`chat_advanced_thresholds:<id>`) matching `chatAdvancedThresholdsPrefix` | ✓ WIRED | Confirmed in `repository.go:24,319-321` and `local.go:62,125`; proven by `TestSkipLocalChatAdvancedThresholdsKey`. |
| `fuku/modules/raid_flood.go` (`raidFloodCountUpdate`, `raidSnapshotFlooders`) | `fuku/modules/advthresholds.go` | `advThresholdsFor` after `settings.FloodEnabled` & exemptions -> `recordRaidFloodItemWith` & `raidFloodCountsFor` | ✓ WIRED | Confirmed in `raid_flood.go:344-360,409-415`; warm hot path makes 0 DB queries (`TestAdvancedThresholdsRaidWiring/flood_window_share_floor_recent_and_ttl`). |
| `fuku/modules/raid_counters.go`, `raid_join.go`, `raid_ai.go`, `raid_ai_count.go` | `fuku/modules/advthresholds.go` | `advThresholdsFor` -> `advRaidJoinWindow`, `advRaidAIWindow`, `advRaidRecentWindow` across all 5 recent-window sites | ✓ WIRED | Direct grep confirms zero production callers still pass fixed `raidRecentWindow` or fixed raid windows. |
| `fuku/modules/raid_ai_image.go` | `fuku/modules/advthresholds.go` | `advRaidAIImageBar(advThresholdsFor(ctx, job.chat.Id))` | ✓ WIRED | `raid_ai_image.go:194-195`; tested on both fresh and cached probabilities. |
| `fuku/modules/memberreq_ai.go` (`memberReqAIRule` ← `memberReqAIJudge`) | `fuku/modules/advthresholds.go` | Raw `Probability` returned from `"n:"+plan.key` singleflight; per-caller `memberReqAIVerdictOfBar(answer.Probability, bar)` | ✓ WIRED | `memberreq_ai.go:393-423,523-554`; cross-chat isolation proven by `TestAdvancedThresholdsAIAndMemberReq/memberreq_ai_bar_cross_chat_singleflight`. |
| `fuku/modules/memberreq_watch.go` (`memberReqLookupCachedWith`) | `fuku/modules/advthresholds.go` | `advMemberReqAIBar` gated on `rules.AI`, passed to `aiRead.satisfiedBar(aiBar)` | ✓ WIRED | `memberreq_watch.go:318-323`; proven 0 lookups when `rules.AI == false`. |
| `fuku/modules/memberreq_limits.go` | `fuku/modules/advthresholds.go` | `advMemberReqCapPerMin` & `advMemberReqPause` -> `memberReqPauseChatWith` | ✓ WIRED | `memberreq_limits.go:93-131`; preserves package-var seams when 0. |
| `fuku/modules/lockdown_alert.go` (`lockdownPlanReminders`, `lockdownPostAlertSlot`) | `fuku/modules/advthresholds.go` | `lockdownReminderIntervalFor` + `lockdownReminderSeqFor` at both sites | ✓ WIRED | `lockdown_alert.go:394,438-439,789-790`. |
| `fuku/modules/captcha.go` & `settings_captcha.go` | `fuku/modules/advthresholds.go` | `effectiveCaptchaTimeout` -> `CreateCaptchaAttemptPreMessageIfEnabled`, `captcha_web_welcome`, `captchaChallengeText`, `/captcha`, `renderSettingsCaptchaWithTimeout` | ✓ WIRED | `captcha.go:496,826-828,1300-1353`, `settings_captcha.go:85,211,313`. |
| `fuku/modules/settings_advthresholds.go` | `fuku/modules/advthresholds.go` & `fuku/db/advthresholds/repository.go` | `settingsAdvRead` (`advthresholds.GetFresh`), `applyAdvancedThreshold`, `resetAdvancedThreshold`, `resetAdvancedArea` | ✓ WIRED | `settings_advthresholds.go:207,321,582,595,608`; proven by `TestSettingsAdvancedThresholds` and `TestAdvancedThresholdsEndToEnd`. |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| -------- | ------------- | ------ | ------------------ | ------ |
| `fuku/modules/settings_advthresholds.go` | `th` (`advthresholds.Thresholds`) on hub, area, item, Main | `settingsAdvReadNow` -> `advthresholds.GetFresh` (PostgreSQL `chat_advanced_thresholds`) | Yes (real DB query, zero struct on `ErrRecordNotFound`) | ✓ FLOWING |
| `fuku/modules/advthresholds.go` | `th` on raid, AI, memberreq, lockdown, captcha hot paths | `advThresholdsLookup` -> `advthresholds.Get` (`skipLocal` Redis cache backed by `GetFresh`) | Yes (real cache/DB query, fail-to-default on error) | ✓ FLOWING |
| `fuku/modules/raid_counters.go` / `raid_flood.go` / `raid_ai_count.go` | `joinWindow`, `floodWindow`, `aiWindow`, `recentWindow`, `joinedTTL`, `floodTTL`, `aiTTL` | `advThresholdsFor` -> Lua script `ARGV` | Yes (real per-chat thresholds driving live Redis ZSET windows & PEXPIRE TTLs) | ✓ FLOWING |
| `fuku/modules/captcha.go` & `settings_captcha.go` | `timeout` / `effectiveTimeout` | `effectiveCaptchaTimeout` (`advCaptchaTimeout(advThresholdsFor(...), settings.Timeout)`) | Yes (drives `captcha_attempts.ExpiresAt`, Turnstile TTL, timer, challenge text, and status display) | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Local phase gates (translations, docs, vet, build, shards) | `make check-translations && make check-docs && go vet -tags testtools ./... && CGO_ENABLED=0 go build ./... && make test-shard-check` | 1068/1068 keys across 8 locales, 0 doc drift, vet/build/shard-check exit `0` | ✓ PASS |
| Repository CRUD, `skipLocal`, backup exclusion, migration statements, i18n | `go test -tags testtools -race -count=1 ./fuku/db/advthresholds` + cache/backup/migrations/i18n targeted tests | All PASS (`TestAdvancedThresholdsMigrationDefaults` cleanly skipped: `DATABASE_URL` unset) | ✓ PASS |
| Phase 28 module tests + MSG-11 golden + EVT-10 guards (in-process, zero TCP dials) | `TZ=UTC go test -tags testtools -race -count=1 -run '^(TestLegacyPhase28DefaultThresholds\|TestAdvancedThresholdDefaultsMatchConstants\|TestAdvancedThresholdsRaidWiring\|TestAdvancedThresholdsAIAndMemberReq\|TestAdvancedThresholdsLockdownAndCaptcha\|TestSettingsAdvancedThresholds\|TestPhase28Codes\|TestPhase28ReadsCreateNothing\|TestPhase28StaffPrivacyAndLogs\|TestAdvancedThresholdsEndToEnd\|TestEventEngineCallers\|TestPMCopyCallers)$' ./fuku/modules` | 12/12 top-level suites PASS (4.06 s, 0 failures, 0 skips) | ✓ PASS |
| Pre-existing grid, captcha, lockdown, staff-privacy, memberreq AI & deletion regression suites | `TZ=UTC go test -tags testtools -race -count=1 -run '^(TestSettingsMainGrid\|TestSettingsMainPhase18\|TestSettingsMainPhase19\|TestSettingsFwdBlockMainShape\|TestSettingsPhase19Codes\|TestPhase27Codes\|TestSettingsStaffPrivacy\|TestSettingsCaptcha\|TestCaptchaSubcommandsRejectInvalidValues\|TestLockdownReminders\|TestMemberDeletionExcludedMessages)$' ./fuku/modules` and `-run '^TestMemberReqAI' ./fuku/modules` | All PASS under `-race` (12.66 s + 23.50 s) | ✓ PASS |
| MSG-11 golden & dependency immutability | `git log --oneline 654c8311..HEAD -- fuku/modules/advthresholds_phase28_legacy_request_test.go` and `git diff 654c8311..HEAD -- go.mod go.sum` | Exactly 1 commit (`48d5c240`, 1 file, before any Phase 28 prod commit); `go.mod`/`go.sum` empty diff | ✓ PASS |
| PostgreSQL migration constraint execution | `make test-postgres-integrity` (`TestAdvancedThresholdsMigrationDefaults`) | Skipped locally (`DATABASE_URL` unset) | ? SKIP |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ----------- | ----------- | ------ | -------- |
| `TUNE-07` | 28-01..28-06 | An Advanced page exposes per-group thresholds that are code constants today, each inside safe bounds and with a reset to default: the raid windows, the new-member share and age, the AI spam bars, the member requirement cap and pause, the lockdown reminder interval and a captcha timeout above 10 minutes. | ✓ SATISFIED (code/tests verified; PostgreSQL constraint run, CI lint/gosec, live Telegram UAT & P28-01..04 sign-off pending human verification) | All 12 thresholds wired end-to-end across storage, `skipLocal` cache, runtime watchers/workers, and private Advanced page `av` in 8 locales; marked Complete in `.planning/REQUIREMENTS.md:57,123` with zero orphaned Phase 28 requirements. |
| `MSG-11` (cross-cutting) | 28-01..28-06 | Every existing reply and notice / default threshold behavior is unchanged for a group with no customization, proved by characterization tests recorded before production changes. | ✓ SATISFIED | `TestLegacyPhase28DefaultThresholds` committed alone in `48d5c240` ahead of all production edits, untouched since, passes 8/8 subtests under `-race`. |
| `EVT-10` (cross-cutting) | 28-04..28-06 | Staff Group actions, lockdown alerts/reminders, raid-scale removals, log channel posts, and captcha challenges stay untouched and outside the event engine / PM copies; no Staff Group defaults or inheritance. | ✓ SATISFIED | `TestEventEngineCallers`, `TestPMCopyCallers`, `TestMemberDeletionExcludedMessages`, `TestPhase28StaffPrivacyAndLogs`, and `TestSettingsStaffPrivacy` all pass under `-race`; `av` hidden in Staff Groups. |

### Anti-Patterns Found

None. Zero `TBD`/`FIXME`/`XXX` debt markers, zero `TODO`/`HACK`/`PLACEHOLDER` comments, zero stub handlers, zero production callers bypassing per-chat threshold lookups, and zero Staff Group or user/group title leaks in UI or logs.

### Human Verification Required

### 1. PostgreSQL Integrity Tests

**Test:** Run `FUKU_TEST_DATABASE=true DATABASE_URL=... make test-postgres-integrity` (covering `TestAdvancedThresholdsMigrationDefaults` and `TestAdvancedThresholdsCRUD`).
**Expected:** Migration `20261123120000_add_advanced_thresholds.sql` runs idempotently twice on PostgreSQL, enforces all 12 inline named `chk_chat_adv_*` CHECK constraints with SQLSTATE `23514`, and column-targeted upserts + field/area/all resets pass under `-race`.
**Why human:** No PostgreSQL server is available in this environment (`DATABASE_URL` unset), so `TestAdvancedThresholdsMigrationDefaults` skipped.

### 2. CI Static Checks (`make lint` and `gosec`)

**Test:** Run `make lint` and `gosec` on this tree in CI or a host with both tools installed.
**Expected:** Both `golangci-lint` and `gosec` exit `0` with zero new findings on Phase 28 files.
**Why human:** Neither `golangci-lint` nor `gosec` is installed in this local environment.

### 3. Live Telegram UAT

**Test:** In a real supergroup + bot private chat: open private `/settings` → Advanced (`av`) (confirm group `/settings` shows `Private only` and opening `av` shows `settings_adv_group_note`, and Staff Group Main hides `av`); inspect all 6 areas and verify each threshold shows current, default, and allowed range; set a custom flood window (`120 s`), lockdown reminder interval (`15 min`), and extended captcha timeout (`30 min`); verify `/captcha` status and the Captcha page display `30 min` while `/captchatime 11` is still refused; trigger a joiner captcha and verify the 30-minute challenge text and expiry; reset one item and one area and confirm defaults are restored on the next event.
**Expected:** Every screen, button label, and `✅ ` active marker renders cleanly in Telegram, extended captcha timeout applies to the next joiner, custom thresholds act immediately across replicas, and resets cleanly restore today's defaults.
**Why human:** Visual rendering, live Telegram API keyboard acceptance, multi-replica Redis timing, and end-to-end UX cannot be verified offline.

### 4. Owner Confirmation of Provisional Decisions `P28-01`..`P28-04`

**Test:** Review and confirm (or request adjustments to) provisional owner decisions `P28-01` through `P28-04` documented in `AGENTS.md`.
**Expected:** Owner signs off on:
- **P28-01:** Storing `0` means unset/default; safe bounds across all 12 thresholds (`raid_join_window_secs` `15..600`, `raid_flood_window_secs` `10..300`, `raid_ai_window_secs` `30..900`, `raid_flood_new_percent` `20..100`, `raid_flood_min_accounts` `2..20`, `raid_recent_hours` `1..168`, `raid_ai_image_bar_pct` `50..99`, `member_req_ai_bar_pct` `50..99`, `member_req_cap_per_min` `3..100`, `member_req_pause_mins` `1..60`, `lockdown_reminder_mins` `15..1440`, `captcha_timeout_mins` `0` or `11..60`); selecting a threshold's default preset normalizes storage to `0`.
- **P28-02:** Extended captcha timeout (>10 min) uses Option A override in `chat_advanced_thresholds.captcha_timeout_mins` (`11..60`), overriding `sendCaptcha` and `/captcha` + `settings_captcha` displays while keeping `captcha_settings.timeout` (`1..10` CHECK), `/captchatime`, the Captcha page 1..10 stepper, and `/export` untouched.
- **P28-03:** Page `av` is `PrivateOnly`, hidden in Staff Groups, requires live `settingsRightAdmin` + owner gate to view and live `settingsRightRestrict` to write/reset, and stays outside backup/export/import/reset until Phase 30.
- **P28-04:** On transient `advThresholdsLookup` errors, hot paths log one IDs-only Warn line and fall back to compiled defaults.
**Why human:** Discuss phase was skipped via `workflow.skip_discuss=true`, so owner-noticeable scope and authority choices require explicit owner confirmation.

### Gaps Summary

No code, test, wiring, or requirement gaps were found. All 24 locally verifiable must-haves, all 4 ROADMAP Success Criteria, `TUNE-07`, MSG-11 golden immutability (`48d5c240`), EVT-10 caller guards (`TestEventEngineCallers`, `TestPMCopyCallers`, `TestMemberDeletionExcludedMessages`), and all local phase gates pass cleanly under `-race`. Overall status is `human_needed` solely for the 1 PostgreSQL-only migration constraint check (`TestAdvancedThresholdsMigrationDefaults`, skipped locally without `DATABASE_URL`) plus CI `make lint`/`gosec`, live Telegram UAT, and owner confirmation of provisional decisions `P28-01`..`P28-04`.

---

_Verified: 2026-11-23T13:07:51Z_
_Verifier: the agent (gsd-verifier)_
