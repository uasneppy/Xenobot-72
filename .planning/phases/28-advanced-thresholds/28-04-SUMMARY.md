---
phase: 28-advanced-thresholds
plan: 04
subsystem: moderation
tags: [lockdown, reminders, captcha, thresholds, option-a, evt-10, settings]

requires:
  - phase: 28-advanced-thresholds
    provides: MSG-11 characterization golden suite TestLegacyPhase28DefaultThresholds and phase28InProcessMiniredis helper (28-01)
  - phase: 28-advanced-thresholds
    provides: chat_advanced_thresholds repository, skipLocal cache, and advThresholdsFor / advLockdownReminderEvery / advCaptchaTimeout helpers (28-02)
provides:
  - Consistent per-group lockdown reminder interval lookup (lockdownReminderIntervalFor + lockdownReminderSeqFor) at both planner (lockdownPlanReminders) and delivery/supersession (lockdownPostAlertSlot) sites
  - Option A extended captcha timeout (effectiveCaptchaTimeout) wired into sendCaptcha (ExpiresAt, Turnstile TTL, challenge text), /captcha status, and settings_captcha body display while keeping captcha_settings.timeout (1..10), /captchatime, stepper, and backups untouched
  - End-to-end tests in TestAdvancedThresholdsLockdownAndCaptcha (5 subtests, zero localhost TCP dials)
affects: [28-05, 28-06]

tech-stack:
  added: []
  patterns:
    - "Shared lockdownReminderIntervalFor + lockdownReminderSeqFor helper across both planner and delivery sites so reminder scheduling and stale-reminder supersession never skew (T-28-09)"
    - "Option A extended captcha timeout override (P28-02, T-28-10): chat_advanced_thresholds.captcha_timeout_mins (0 or 11..60) overrides sendCaptcha and status displays while preserving captcha_settings.timeout (1..10 SQL CHECK), /captchatime, stepper, and backup format"
    - "Package-var seam preservation for lockdownReminderEvery when stored lockdown_reminder_mins == 0"

key-files:
  created:
    - fuku/modules/advthresholds_lockdown_captcha_test.go
  modified:
    - fuku/modules/lockdown_alert.go
    - fuku/modules/captcha.go
    - fuku/modules/settings_captcha.go

key-decisions:
  - "P28-02: Option A extended captcha timeout overrides sendCaptcha (ExpiresAt, Turnstile challenge TTL, challenge text), /captcha status, and settings_captcha body via effectiveCaptchaTimeout while leaving captcha_settings.timeout (1..10 SQL CHECK), /captchatime, the Captcha page 1..10 stepper, and backup export completely untouched."
  - "Kept lockdownReminderSeq and renderSettingsCaptcha as thin default-delegating wrappers around lockdownReminderSeqFor and renderSettingsCaptchaWithTimeout so all existing unit tests compile and pass unchanged."
  - "Built newPhase28LockdownEnv / newPhase28ReminderScene in advthresholds_lockdown_captcha_test.go using phase28InProcessMiniredis so all 5 subtests run under -race with zero localhost TCP dials."

patterns-established:
  - "lockdownReminderSeqFor + lockdownReminderIntervalFor: single interval resolution seam shared across lockdown reminder planning and delivery supersession."
  - "effectiveCaptchaTimeout(ctx, chatID, baseTimeout): single Option A timeout resolution helper shared by sendCaptcha, /captcha status, and settings_captcha."

requirements-completed: [TUNE-07]

status: complete
plan_head_before: b6e24c5bc033f0da2a053876f3efc2c0d5b5ba5e
plan_head_after: 325e5b6155d9699ac1ecc6ded318189f0d7068f4
commits: 3

actuals:
  tokens: 5842
  tasks: 3
  commits: 3

duration: 29min
completed: 2026-11-23
---

# Phase 28 Plan 04: Lockdown Reminder Interval & Extended Captcha Timeout Summary

**Per-group lockdown reminder interval wired consistently across planner and delivery supersession (preserving the `lockdownReminderEvery` test seam) and Option A extended captcha timeout (`11..60` min) wired into `sendCaptcha`, `/captcha` status, and `settings_captcha` body while keeping `1..10` captcha contracts and EVT-10 intact**

## Performance

- **Duration:** 29 min
- **Started:** 2026-11-23T11:48:00Z
- **Completed:** 2026-11-23T12:17:12Z
- **Tasks:** 3
- **Files modified:** 4 (1 created, 3 modified)

## Accomplishments

- Added `lockdownReminderSeqFor` (guarding `every <= 0`) and `lockdownReminderIntervalFor` (`advLockdownReminderEvery(advThresholdsFor(ctx, chatID))`) in `fuku/modules/lockdown_alert.go`, wired both `lockdownPlanReminders` and `lockdownPostAlertSlot` (delivery supersession) to the same helper, and kept `lockdownReminderSeq` as a thin wrapper preserving the `lockdownReminderEvery` package-var test seam when unset (`0`).
- Implemented Option A extended captcha timeout (P28-02) via `effectiveCaptchaTimeout(ctx, chatID, baseTimeout)` in `fuku/modules/captcha.go`, wired into `sendCaptcha` (driving `captcha_attempts.ExpiresAt`, Turnstile challenge TTL, timeout timer, and challenge message text) and `/captcha` status (`captcha_settings_timeout`), while leaving `applyCaptchaTimeout`, `fuku/db/captcha/repository.go`, `fuku/db/models/captcha.go`, and `captcha_settings` SQL CHECK constraints untouched.
- Added `renderSettingsCaptchaWithTimeout` in `fuku/modules/settings_captcha.go` so `settingsCaptchaOpen`, `settingsCaptchaRedraw`, and `settingsCaptchaSetTime` display the effective timeout in `settings_captcha_time` while keeping the `1..10` stepper bound to `s.Timeout` and creating 0 DB rows on unconfigured reads.
- Added `TestAdvancedThresholdsLockdownAndCaptcha` in `fuku/modules/advthresholds_lockdown_captcha_test.go` with all 5 subtests (`lockdown_reminder_shorter_interval_15m`, `lockdown_reminder_longer_interval_120m`, `lockdown_reminder_preserves_package_var_seam_when_unset`, `captcha_timeout_extended_30m_and_reset`, `captcha_command_still_rejects_above_10`) running under `-race` via `phase28InProcessMiniredis` with zero localhost TCP dials.

## Task Commits

1. **Task 1: wire per-group lockdown reminder interval at both planner and delivery sites** - `4ed5be8d` (`feat(28-04): wire per-group lockdown reminder interval at planner and delivery sites`)
2. **Task 2: wire Option A extended captcha timeout into sendCaptcha and status displays** - `982986d2` (`feat(28-04): wire Option A extended captcha timeout into sendCaptcha and status displays`)
3. **Task 3: tests for custom lockdown reminder interval and extended captcha timeout** - `325e5b61` (`test(28-04): add lockdown reminder interval and extended captcha timeout tests`)

## Files Created/Modified

- `fuku/modules/lockdown_alert.go` — Added `lockdownReminderSeqFor` and `lockdownReminderIntervalFor`; wired both `lockdownPlanReminders` and `lockdownPostAlertSlot` while preserving `lockdownReminderSeq` wrapper and EVT-10 boundary.
- `fuku/modules/captcha.go` — Added `effectiveCaptchaTimeout`; wired effective timeout into `sendCaptcha` (`CreateCaptchaAttemptPreMessageIfEnabled`, `captcha_web_welcome`, `captchaChallengeText`) and `/captcha` status while keeping `applyCaptchaTimeout` (`1..10`) untouched.
- `fuku/modules/settings_captcha.go` — Added `renderSettingsCaptchaWithTimeout` displaying effective timeout in the Captcha settings body (`settingsCaptchaOpen`, `settingsCaptchaRedraw`, `settingsCaptchaSetTime`) while keeping the `1..10` stepper bound to `s.Timeout`.
- `fuku/modules/advthresholds_lockdown_captcha_test.go` — Created `TestAdvancedThresholdsLockdownAndCaptcha` covering 15 m / 120 m reminder intervals, unset seam preservation, 30 m extended captcha timeout + `/captcha` status + reset back to 3 m without clobbering `captcha_settings.timeout`, and `/captchatime` rejection above 10.

## Decisions Made

- Implemented P28-02 (Option A) strictly as an override in `chat_advanced_thresholds.captcha_timeout_mins` so existing `1..10` SQL CHECK constraints, `captcha_settings.timeout` repository/command validation, Captcha settings stepper, and backup export/import remain completely unchanged.
- In `advthresholds_lockdown_captcha_test.go`, constructed `newPhase28LockdownEnv` / `newPhase28ReminderScene` backed by `phase28InProcessMiniredis` instead of `newLockdownEnv`'s TCP-dialing `withMiniredisServer` so the test suite executes with zero localhost TCP dials in sandboxed environments and CI.

## Deviations from Plan

None - plan executed as written. (Note: in Task 1, the delivery/supersession function in `fuku/modules/lockdown_alert.go` is named `lockdownPostAlertSlot` rather than `lockdownDeliverAlert`; in Task 3, the existing refusal outcome constant in `fuku/modules/captcha.go` is `captchaInvalid` rather than `captchaApplyInvalid`.)

## Issues Encountered

- Running pre-existing SQLite captcha tests (`TestSettingsCaptchaToggle`, `TestMemberDeletionExcludedMessages/a_captcha_challenge`) on a non-UTC host (`+02:00`) fails SQLite's lexical `chk_captcha_expires_at` comparison unless `TZ=UTC` is set (pre-existing behavior outside Plan 28-04 scope; `TestAdvancedThresholdsLockdownAndCaptcha` sets `db.DB.Config.NowFunc` to UTC so it passes in any host timezone). With `TZ=UTC` and loopback enabled for pre-existing `withMiniredisServer` tests, the entire verification suite passes cleanly in 4.8s.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- All 12 Phase 28 runtime thresholds across all 6 areas (raid windows, new-member share & age, AI spam bars, memberreq cap/pause, lockdown reminder interval, extended captcha timeout) are now wired and verified under `-race`.
- Ready for Plan 28-05 (Private Advanced Settings page `av`, action codes, 8-locale copy, and safety/privacy/locale tests).

## Self-Check: PASSED

- All created/modified files verified present on disk (`fuku/modules/lockdown_alert.go`, `fuku/modules/captcha.go`, `fuku/modules/settings_captcha.go`, `fuku/modules/advthresholds_lockdown_captcha_test.go`).
- All 3 task commits (`4ed5be8d`, `982986d2`, `325e5b61`) verified as ancestors of HEAD with 0 accidental file deletions.
- `fuku/modules/advthresholds_phase28_legacy_request_test.go`, `fuku/db/captcha/repository.go`, `fuku/db/models/captcha.go`, `go.mod`, and `go.sum` verified untouched.
- Acceptance greps verified:
  - `grep -c 'func lockdownReminderSeqFor(' fuku/modules/lockdown_alert.go` → `1`
  - `grep -c 'lockdownReminderIntervalFor(' fuku/modules/lockdown_alert.go` → `3`
  - `grep -c 'func effectiveCaptchaTimeout(' fuku/modules/captcha.go` → `1`
  - `grep -c 'effectiveCaptchaTimeout(' fuku/modules/captcha.go` → `3`
  - `grep -c 'effectiveCaptchaTimeout(' fuku/modules/settings_captcha.go` → `3`
  - `TestAdvancedThresholdsLockdownAndCaptcha` subtest PASS count → `5`
- Full verification suite passed under `-race` with zero failures/skips:
  - `TestAdvancedThresholdsLockdownAndCaptcha` (5/5, 0.39s, zero localhost TCP dials) — PASS
  - `TestLockdownReminders` (6/6) — PASS
  - `TestCaptchaSubcommandsRejectInvalidValues` (5/5) — PASS
  - `TestSettingsCaptcha*` (Page, Toggle, Turnstile, CallbackBytes, PageFitsEveryLocale, ReadFailure) — PASS
  - `TestLegacyPhase28DefaultThresholds` (8/8) — PASS
  - `TestEventEngineCallers` — PASS
  - `TestPMCopyCallers` — PASS
  - `TestMemberDeletionExcludedMessages` (6/6) — PASS
  - `CGO_ENABLED=0 go build ./...` — PASS
