---
phase: 28-advanced-thresholds
reviewed: 2026-11-24T09:53:00Z
depth: standard
files_reviewed: 18
files_reviewed_list:
  - migrations/20261123120000_add_advanced_thresholds.sql
  - fuku/db/cache/local.go
  - fuku/db/models/advthresholds.go
  - fuku/db/advthresholds/repository.go
  - fuku/modules/advthresholds.go
  - fuku/modules/settings_advthresholds.go
  - fuku/modules/settings_captcha.go
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
findings:
  critical: 0
  warning: 5
  info: 2
  total: 7
  necessity: 0
status: issues_found
---

# Phase 28: Code Review Report

**Reviewed:** 2026-11-24T09:53:00Z
**Depth:** standard
**Files Reviewed:** 18
**Status:** issues_found

## Summary

Reviewed all 18 Phase 28 (Advanced Thresholds) files on HEAD, covering the PostgreSQL migration, `skipLocal` cache configuration, GORM model and repository, module helpers, Private-only `/settings` Advanced thresholds UI, raid Lua scripts and derived TTLs, member-requirements AI bar / cap / pause wiring, lockdown reminder interval wiring, and Option A extended captcha timeout.

The core schema, SQL/Go bound parity, column-targeted upserts, `skipLocal` invalidation, callback authorization/validation, and raid Redis Lua scripts with derived TTLs are solid. Five warnings were found in cross-feature interactions: (1) extended captcha timeouts (11–60 min) outlive Turnstile's hardcoded 11-minute `InitDataMaxAge`, (2) lengthening `lockdown_reminder_mins` during an active lockdown causes sequence-number collisions that suppress reminders for hours, (3) `/captchatime` and the Captcha settings stepper silently update `captcha_settings.timeout` while an extended override keeps shadowing it, (4) the account-global AI name baseline key (`nm:<user>`) lets a permissive chat's bar bypass a stricter chat's bar, and (5) coalesced singleflight followers double-count `breached`/`passed` stats in `memberReqAIRule`. Two minor info items were noted for dead seam code and all-zero row creation on reset.

## Narrative Findings (AI reviewer)

## Warnings

### WR-01: Extended captcha timeout (11–60 min) exceeds Turnstile `InitDataMaxAge` (11 min)

**File:** `fuku/modules/captcha.go:1300-1337` (interacting with `fuku/utils/captchaweb/initdata.go:16-19`)
**Issue:** `SendCaptcha` applies `effectiveCaptchaTimeout` (up to 60 minutes via `chat_advanced_thresholds.captcha_timeout_mins`) when creating `preAttempt.ExpiresAt` and issuing the Turnstile web challenge, and tells the joiner they have up to 60 minutes to verify. However, `captchaweb.InitDataMaxAge` (`fuku/utils/captchaweb/initdata.go:19`) is still hardcoded to `11 * time.Minute` (explicitly sized for the old 10-minute max plus 1-minute grace). A member who opens the Mini App on join and completes the widget after 11 minutes—well within a 15–60 minute timeout—is rejected with `ErrInvalidInitData`.
**Fix:** Raise `captchaweb.InitDataMaxAge` to cover the maximum extended captcha timeout (e.g., `time.Duration(models.MaxCaptchaTimeoutMins+1) * time.Minute`, or `61 * time.Minute`) and update its comment/tests accordingly.

### WR-02: Lengthening `lockdown_reminder_mins` mid-lockdown suppresses reminders for hours due to `seq` collision

**File:** `fuku/modules/lockdown_alert.go:375-384`, `438-450`
**Issue:** `lockdownReminderSeqFor` computes `seq = int(elapsed / every)` from `alert_started_at`, and `lockdownPlanReminders` skips planning whenever `lockdown.ReminderPlanned(ld.ID, seq)` is true. That bucket formula assumed `every` was a fixed 1-hour constant. If a lockdown runs at `lockdown_reminder_mins = 15` for 2 hours (`seq` 1..8 planned and sent) and the owner then changes the interval to `60` minutes, `elapsed / 60m` drops back to `seq = 2` (already planned), and `seq` 3..8 are also already planned—suppressing all reminders for 7 hours until `seq = 9` at hour 9.
**Fix:** Base the next reminder due check on the elapsed time since the most recent reminder row's `created_at` (falling back to `alert_started_at` when no reminder exists yet) and allocate `maxSeq + 1` once `now.Sub(lastAt) >= interval`, rather than deriving `seq` by dividing total elapsed time by a mutable interval.

### WR-03: `/captchatime` and Captcha settings stepper silently edit `captcha_settings.timeout` while shadowed by an extended override

**File:** `fuku/modules/captcha.go:772-788`, `834-852`; `fuku/modules/settings_captcha.go:158-179`, `295-314`
**Issue:** When `chat_advanced_thresholds.captcha_timeout_mins` is set (11–60 min), `effectiveCaptchaTimeout` always returns the override and ignores `captcha_settings.timeout`. Yet `/captchatime 5` writes `captcha_settings.timeout = 5` and replies `"Captcha timeout set to 5 minutes"` while the effective timeout remains unchanged (e.g. 30 min). Likewise, on the `/settings` Captcha page, the body shows `effectiveTimeout` (30 min) while the stepper button displays and steps `s.Timeout` (5 min -> 6 min) with no indication that the extended override is taking precedence.
**Fix:** Either clear `chat_advanced_thresholds.captcha_timeout_mins` (reset to `0`) when an admin explicitly sets a 1–10 minute timeout via `/captchatime` or the Captcha stepper, or include a clear notice in the reply and Captcha page whenever an extended timeout override is active.

### WR-04: Account-global AI name baseline (`nm:<user>`) allows a permissive chat's bar to bypass a stricter chat's bar

**File:** `fuku/modules/memberreq_ai.go:230-232`, `360-366`, `449-452`; `fuku/modules/memberreq_watch.go:253-261`
**Issue:** Phase 28 introduces per-chat `member_req_ai_bar_pct` (50%–99%) and refactors the singleflight to return raw probabilities so chats do not leak thresholds (T-28-06). However, the accepted-name baseline key `fuku:memberreq:nm:<user>` has no `chatID`. If an existing member of Chat B (strict bar 60%, no join marker) renames to a spam name scoring `0.80` and passes in Chat A (permissive bar 95%), Chat A writes `nm:<user> = <hash>`. When the user next messages in Chat B, both `memberReqAIWatchSatisfiedBar` (line 260) and `memberReqAIDecideBar` (line 361) see `!marker && baseline == hash` and immediately pass the user, bypassing Chat B's 60% bar.
**Fix:** Scope the baseline key per chat (`fuku:memberreq:nm:<chat>:<user>`) so a pass accepted under one group's threshold never exempts the account in a stricter group.

### WR-05: Coalesced singleflight followers over-count `breached`/`passed` stats in `memberReqAIRule`

**File:** `fuku/modules/memberreq_ai.go:404-415`
**Issue:** `memberReqAIJudge` increments `memberReqAIStats.checked` once and returns `memberReqAIAnswer{Probability: ..., HasProbability: true, Fresh: true}`. `memberReqFlightValue` returns that same `answer` (with `Fresh: true`) to the leader and every concurrent follower coalesced on `"n:"+plan.key`. In `memberReqAIRule` lines 409–415, every coalesced caller checks `if answer.Fresh` and increments `breached` or `passed`, causing `breached + passed` to exceed `checked` under concurrency while followers are not counted as `cached`.
**Fix:** Track whether the current caller was the flight leader (as `raid_ai_image.go:168-180` does with a `leader` flag) and only increment `breached`/`passed` for the leader, counting followers under `memberReqAIStats.cached`.

## Info

### IN-01: Unused `ResetAll` field on `advThresholdsWrites`

**File:** `fuku/modules/advthresholds.go:33`, `38`
**Issue:** `advThresholdsWrites.ResetAll` is initialized to `advthresholds.ResetAll` but is never referenced anywhere in `fuku/modules/`.
**Fix:** Remove the unused `ResetAll` field from `advThresholdsWrites`.

### IN-02: Resetting a field or area on a chat with no row inserts an all-zero row

**File:** `fuku/db/advthresholds/repository.go:374-406`
**Issue:** Unlike `ResetAll` and `ReplaceAllTx` (which delete the row when `CustomizedCount() == 0`), calling `ResetField`, `ResetArea`, or `SetField(..., 0, ...)` on a chat with no existing row performs an `ON CONFLICT` upsert that inserts an all-zero row into `chat_advanced_thresholds`.
**Fix:** When writing `0` in `ResetField`/`ResetArea` (or `SetField` with `value == 0`), use an `UPDATE` statement so a chat with no existing row does not get an unnecessary all-zero row inserted.

---

_Reviewed: 2026-11-24T09:53:00Z_
_Reviewer: the agent (gsd-code-reviewer)_
_Depth: standard_
