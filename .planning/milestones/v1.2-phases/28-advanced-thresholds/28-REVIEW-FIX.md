---
phase: 28-advanced-thresholds
fixed_at: 2026-11-24T15:51:58Z
review_path: /Users/denyspetrenko/Xenobot-72/.planning/phases/28-advanced-thresholds/28-REVIEW.md
fix_scope: critical_warning
iteration: 1
findings_in_scope: 5
fixed: 5
skipped: 0
status: all_fixed
---

# Phase 28: Code Review Fix Report

**Fixed at:** 2026-11-24T15:51:58Z
**Source review:** `/Users/denyspetrenko/Xenobot-72/.planning/phases/28-advanced-thresholds/28-REVIEW.md`
**Fix scope:** `critical_warning`
**Iteration:** 1

**Summary:**
- Findings in scope: 5 (`WR-01`..`WR-05`; `IN-01` and `IN-02` out of scope)
- Fixed: 5
- Skipped: 0

## Fixed Issues

### WR-01: Extended captcha timeout (11–60 min) exceeds Turnstile `InitDataMaxAge` (11 min)

**Files modified:** `fuku/utils/captchaweb/initdata.go`, `fuku/utils/captchaweb/initdata_test.go`
**Commit:** `a5a21311`
**Applied fix:** Raised `captchaweb.InitDataMaxAge` from `11 * time.Minute` to `61 * time.Minute` (covering the 60-minute `chat_advanced_thresholds.captcha_timeout_mins` maximum plus 1-minute grace) and updated the comment and `TestInitDataRejects` boundary/acceptance cases accordingly.

### WR-02: Lengthening `lockdown_reminder_mins` mid-lockdown suppresses reminders for hours due to `seq` collision

**Files modified:** `fuku/db/lockdown/alerts.go`, `fuku/modules/lockdown_alert.go`, `fuku/modules/advthresholds_lockdown_captcha_test.go`
**Commit:** `844d5951`
**Applied fix:** Added `lockdown.LatestReminder(lockdownID)` returning the highest reminder `seq` and its `created_at`, plus `lockdownNextReminderSeq` and `lockdownReminderSuperseded` in `fuku/modules/lockdown_alert.go`. Normal fixed-interval bucket progression and outage skip-ahead are preserved, while mid-lockdown interval lengthening (`elapsed/every <= lastSeq`) allocates `lastSeq + 1` once `now.Sub(lastAt) >= interval`, and stale-pending supersession checks both the due bucket and `lastSeq`. Added regression subtest `lockdown_reminder_lengthened_mid_lockdown_does_not_suppress`.

### WR-03: `/captchatime` and Captcha settings stepper silently edit `captcha_settings.timeout` while shadowed by an extended override

**Files modified:** `fuku/modules/captcha.go`, `fuku/modules/settings_captcha.go`, `fuku/modules/advthresholds_lockdown_captcha_test.go`
**Commit:** `aaa7258e`
**Applied fix:** Updated `applyCaptchaTimeout` to reset `chat_advanced_thresholds.captcha_timeout_mins` back to `0` when an extended override is active so any explicit 1–10 minute change via `/captchatime` or the Captcha stepper takes effect immediately. Updated `renderSettingsCaptchaWithTimeout` to display `effectiveTimeout` on the stepper center label and `settingsCaptchaSetTime` to avoid treating a boundary click as a no-op while an extended override is active. Added regression subtest `captcha_timeout_command_clears_extended_override`.

### WR-04: Account-global AI name baseline (`nm:<user>`) allows a permissive chat's bar to bypass a stricter chat's bar

**Files modified:** `fuku/modules/memberreq_ai.go`, `fuku/modules/memberreq_watch.go`, `AGENTS.md`, `fuku/modules/advthresholds_ai_mreq_test.go`, `fuku/modules/memberreq_ai_baseline_test.go`, `fuku/modules/memberreq_ai_baselinettl_test.go`, `fuku/modules/memberreq_ai_e2e_test.go`, `fuku/modules/memberreq_ai_fake_test.go`, `fuku/modules/memberreq_ai_guards_test.go`, `fuku/modules/memberreq_ai_latch_test.go`, `fuku/modules/memberreq_ai_pausedwatch_test.go`, `fuku/modules/memberreq_ai_redisloss_test.go`, `fuku/modules/memberreq_ai_sec_test.go`, `fuku/modules/memberreq_ai_watch_test.go`, `fuku/modules/memberreq_fixed_ai_test.go`
**Commit:** `52195435`
**Applied fix:** Scoped `memberReqAIBaselineKey(chatID, userID)` to `fuku:memberreq:nm:<chat>:<user>` across `memberReqAIReadState`, `memberReqAIRecordPass`, `memberReqAIBaseline`, and `memberReqAIWatchQueue`, updated `AGENTS.md` and existing member-requirements AI test helpers/callers, and added regression subtest `memberreq_ai_baseline_isolated_per_chat` verifying a permissive chat's pass never exempts the account in a stricter chat.

### WR-05: Coalesced singleflight followers over-count `breached`/`passed` stats in `memberReqAIRule`

**Files modified:** `fuku/modules/memberreq_ai.go`, `fuku/modules/advthresholds_ai_mreq_test.go`
**Commit:** `9d2540b4`
**Applied fix:** Added an `atomic.Bool` `leader` flag in `memberReqAIRule` and extracted `memberReqAIRecordFlightStats` so only the singleflight leader increments `memberReqAIStats.breached`/`passed` on fresh provider calls (`breached + passed == checked`) while coalesced followers are counted under `memberReqAIStats.cached`. Extended `memberreq_ai_bar_cross_chat_singleflight` to assert `checked=1, breached=1, passed=0, cached=1`.

## Verification

**Environment:** Main checkout (`/Users/denyspetrenko/Xenobot-72`, `workflow.use_worktrees: false`), `TZ=UTC`.

All checks passed:
- `go vet -tags testtools ./...`
- `CGO_ENABLED=0 go build ./...`
- `make check-translations` (1068 keys present across all 8 locales)
- `make check-docs` (no drift)
- `make test-shard-check` (shard list matches `go test -list`)
- `TZ=UTC go test -tags testtools -race -count=1 ./fuku/db/advthresholds ./fuku/db/lockdown`
- `TZ=UTC go test -tags testtools -race -count=1 -run '^TestInitData' ./fuku/utils/captchaweb` (other `captchaweb` tests require localhost TCP binds/dials blocked by the sandbox)
- `TZ=UTC go test -tags testtools -race -count=1 -run '^(TestLockdownReminders|TestLockdownRemindersReachEveryLockdown|TestPhase28Lockdown|TestLegacyPhase28DefaultThresholds)' ./fuku/modules`
- `TZ=UTC go test -tags testtools -race -count=1 -run '^(TestLegacyPhase28DefaultThresholds|TestAdvancedThresholds|TestSettingsAdvancedThresholds|TestPhase28|TestEventEngineCallers|TestPMCopyCallers|TestLegacyPhase29Baseline|TestLegacyPhase30Baseline)' ./fuku/modules`

---

_Fixed: 2026-11-24T15:51:58Z_
_Fixer: the agent (gsd-code-fixer)_
_Iteration: 1_
