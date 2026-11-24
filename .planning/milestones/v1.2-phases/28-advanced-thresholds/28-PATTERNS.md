# Phase 28: Advanced Thresholds - Pattern Map

**Mapped:** 2026-11-23
**Files analyzed:** 24 new/modified files (1 migration, 1 model, 1 DB package, 1 cache gate, 1 backup test, 12 module files, 8 locale files + locale/main/guard tests)
**Analogs found:** 24 / 24 (all verified git-tracked source files)

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `migrations/20261123120000_add_advanced_thresholds.sql` | migration | CRUD | `migrations/20261122150000_add_manual_command_tuning.sql` | exact |
| `fuku/db/models/advthresholds.go` | model | CRUD | `fuku/db/models/modtuning.go` | exact |
| `fuku/db/advthresholds/repository.go` | repository | CRUD (cached, `skipLocal`) | `fuku/db/modtuning/repository.go` | exact |
| `fuku/db/advthresholds/repository_test.go` | test | CRUD + PG integrity | `fuku/db/modtuning/repository_test.go` | exact |
| `fuku/db/cache/local.go` | cache-gate | request-response | `fuku/db/cache/local.go` (`chatManualDefaultsPrefix`) | exact |
| `fuku/db/cache/local_test.go` | test | unit | `fuku/db/cache/local_test.go` (lines 335-351) | exact |
| `fuku/db/backup/testmain_test.go` | test-harness | schema | `fuku/db/backup/testmain_test.go` (lines 87-88) | exact |
| `fuku/db/backup/advthresholds_backup_test.go` | test | CRUD (exclusion) | `fuku/db/backup/modtuning_backup_test.go` | exact |
| `fuku/modules/test_harness_test.go` | test-harness | schema | `fuku/modules/test_harness_test.go` (lines 390-392) | exact |
| `fuku/modules/advthresholds.go` | service | read/apply/defaults | `fuku/modules/modtuning.go` / `fuku/modules/escalation_settings.go` | exact |
| `fuku/modules/raid_counters.go` | watcher-helper | event-driven (Redis Lua) | `fuku/modules/raid_counters.go` | in-place |
| `fuku/modules/raid_join.go` | watcher | event-driven (Redis Lua) | `fuku/modules/raid_join.go` | in-place |
| `fuku/modules/raid_flood.go` | watcher | event-driven (Redis Lua) | `fuku/modules/raid_flood.go` | in-place |
| `fuku/modules/raid_ai.go` | watcher | event-driven (Redis Lua) | `fuku/modules/raid_ai.go` | in-place |
| `fuku/modules/raid_ai_count.go` | worker-helper | event-driven (Redis Lua) | `fuku/modules/raid_ai_count.go` | in-place |
| `fuku/modules/raid_ai_image.go` | worker | async-eval (Redis cache) | `fuku/modules/raid_ai_image.go` | in-place |
| `fuku/modules/raid_trigger.go` | service | event-driven (snapshots) | `fuku/modules/raid_trigger.go` | in-place |
| `fuku/modules/memberreq_ai.go` | worker | async-eval (singleflight) | `fuku/modules/memberreq_ai.go` | in-place |
| `fuku/modules/memberreq_watch.go` | watcher | event-driven (pipeline) | `fuku/modules/memberreq_watch.go` | in-place |
| `fuku/modules/memberreq_limits.go` | service | event-driven (Redis Lua) | `fuku/modules/memberreq_limits.go` | in-place |
| `fuku/modules/lockdown_alert.go` | worker | periodic-poll (DB) | `fuku/modules/lockdown_alert.go` | in-place |
| `fuku/modules/captcha.go` | handler | event-driven (joiner) | `fuku/modules/captcha.go` | in-place |
| `fuku/modules/settings_advthresholds.go` | UI controller | request-response (PrivateOnly) | `fuku/modules/settings_moddefaults.go` + `fuku/modules/settings_deletion.go` | exact |
| `fuku/modules/settings_phase28_test.go` | test | safety/codes/privacy | `fuku/modules/settings_phase27_test.go` | exact |
| `fuku/modules/settings_main_test.go` + `settings_fwdblock_test.go` | test | Main grid shape | in-place updates | in-place |
| `locales/{en,uk,ru,es,fr,pt,id,hi}.yml` + `fuku/i18n/settings_locale_test.go` | i18n + test | static lookup | `fuku/i18n/settings_locale_test.go` (lines 771-805, 1026-1054) | exact |

---

## The 12 Tunable Thresholds: Today's Constants, Storage, Bounds & Drift Guard

All 12 values sit in **6 logical areas** matching ROADMAP Phase 28 Success Criterion 1. Storing `0` in any column means **"unset / use today's constant"** — so a missing row or a reset column behaves identically to today AND preserves existing test seams that swap package variables (`memberReqCapPerMinute`, `memberReqPause`, `lockdownReminderEvery`).

| Area (`k`) | Code (`t`) | Column in `chat_advanced_thresholds` | Today's Constant (Source & Line) | Default | Proposed Safe Bounds | Unit / Encoding |
|------------|------------|--------------------------------------|----------------------------------|---------|----------------------|-----------------|
| **1. Raid windows** (`rw`) | `jsw` | `raid_join_window_secs` | `raidJoinWindow = 60 * time.Second` (`raid_counters.go:31`) | `60` s | `15..600` s (15 s – 10 min) | integer seconds (`0` = default) |
| | `flw` | `raid_flood_window_secs` | `raidFloodWindow = 30 * time.Second` (`raid_flood.go:58`) | `30` s | `10..300` s (10 s – 5 min) | integer seconds (`0` = default) |
| | `aiw` | `raid_ai_window_secs` | `raidAIWindow = 120 * time.Second` (`raid_ai_count.go:35`) | `120` s | `30..900` s (30 s – 15 min) | integer seconds (`0` = default) |
| **2. New-member share & age** (`nm`) | `fnp` | `raid_flood_new_percent` | `raidFloodNewPercent = 60` (`raid_flood.go:61`) | `60` % | `20..100` % | integer percent (`0` = default) |
| | `fma` | `raid_flood_min_accounts` | `raidFloodMinAccounts = 3` (`raid_flood.go:63`) | `3` accts | `2..20` accounts | integer count (`0` = default) |
| | `rna` | `raid_recent_hours` | `raidRecentWindow = 24 * time.Hour` (`raid_counters.go:34`) | `24` h | `1..168` h (1 h – 7 days) | integer hours (`0` = default) |
| **3. AI spam bars** (`ab`) | `rib` | `raid_ai_image_bar_pct` | `raidAIImageBar = 0.85` (`raid_ai_image.go:35`) | `85` (`0.85`) | `50..99` (`0.50..0.99`) | integer percent 50..99 (`float64(v)/100.0`, `0` = default) — avoids floating-point DB/JSON/callback drift! |
| | `mrb` | `member_req_ai_bar_pct` | `memberReqAIBar = 0.9` (`memberreq_ai.go:73`) | `90` (`0.90`) | `50..99` (`0.50..0.99`) | integer percent 50..99 (`float64(v)/100.0`, `0` = default) |
| **4. Member req cap & pause** (`mr`) | `mrc` | `member_req_cap_per_min` | `memberReqCapPerMinute = 10` (`memberreq_limits.go:28`) | `10` /min | `3..100` /min | integer count (`0` = default) |
| | `mrp` | `member_req_pause_mins` | `memberReqPause = 10 * time.Minute` (`memberreq_limits.go:31`) | `10` min | `1..60` min | integer minutes (`0` = default) |
| **5. Lockdown reminder** (`lr`) | `lrm` | `lockdown_reminder_mins` | `lockdownReminderEvery = time.Hour` (`lockdown_alert.go:78`) | `60` min | `15..1440` min (15 min – 24 h) | integer minutes (`0` = default) |
| **6. Captcha timeout** (`ct`) | `ctm` | `captcha_timeout_mins` | `captcha_settings.timeout` (1..10 min, default 2 min, `fuku/db/captcha/repository.go:41`) | `0` (uses Captcha page setting, 2 min default) | `0` (default) or `11..60` min (or `1..60` min; see Section 5 below!) | integer minutes (`0` = use Captcha page setting) |

**Drift guard requirement (from ROADMAP):**
Every default in `fuku/db/advthresholds` / `fuku/modules/advthresholds.go` MUST be compared against the real code constant in a unit test (`TestAdvancedThresholdDefaultsMatchConstants`) that fails if any constant or default ever drifts:
- `DefaultRaidJoinWindowSecs == int(raidJoinWindow / time.Second)` (60)
- `DefaultRaidFloodWindowSecs == int(raidFloodWindow / time.Second)` (30)
- `DefaultRaidAIWindowSecs == int(raidAIWindow / time.Second)` (120)
- `DefaultRaidFloodNewPercent == raidFloodNewPercent` (60)
- `DefaultRaidFloodMinAccounts == raidFloodMinAccounts` (3)
- `DefaultRaidRecentHours == int(raidRecentWindow / time.Hour)` (24)
- `DefaultRaidAIImageBarPct == int(math.Round(raidAIImageBar * 100))` (85)
- `DefaultMemberReqAIBarPct == int(math.Round(memberReqAIBar * 100))` (90)
- `DefaultMemberReqCapPerMin == memberReqCapPerMinute` (10)
- `DefaultMemberReqPauseMins == int(memberReqPause / time.Minute)` (10)
- `DefaultLockdownReminderMins == int(lockdownReminderEvery / time.Minute)` (60)
- `DefaultCaptchaTimeoutMins == 0` (falls back to `captcha_settings.timeout`, whose default is 2 min in `captcha.GetCaptchaSettingsContext`).

---

## Detailed Pattern Assignments & Traps by Area

### 1. Raid Windows, New-Member Share, Account Floor & Newness Age

#### Exact Existing Sites & Constants
1. **`fuku/modules/raid_counters.go`**
   - Lines 30-36: `raidJoinWindow = 60 * time.Second`, `raidRecentWindow = 24 * time.Hour`, `raidJoinedTTL = 25 * time.Hour`
   - Lines 83-94: `raidJoinScript` — **ALREADY parameterized via ARGV!**
     ```lua
     -- KEYS[1] = joined set. ARGV: 1 now (ms), 2 surge window (ms), 3 recent window (ms),
     -- 4 cap, 5 TTL (ms), 6.. user IDs.
     local now = tonumber(ARGV[1])
     for i = 6, #ARGV do
       redis.call('ZADD', KEYS[1], now, ARGV[i])
     end
     redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', now - tonumber(ARGV[3]))
     local size = redis.call('ZCARD', KEYS[1])
     local cap = tonumber(ARGV[4])
     if size > cap then redis.call('ZREMRANGEBYRANK', KEYS[1], 0, size - cap - 1) end
     redis.call('PEXPIRE', KEYS[1], ARGV[5])
     return redis.call('ZCOUNT', KEYS[1], '(' .. (now - tonumber(ARGV[2])), '+inf')
     ```
   - Lines 100-121: `recordRaidJoins(chatID int64, userIDs []int64)` passes `raidJoinWindow`, `raidRecentWindow`, `raidJoinedTTL`.
   - **TRAP 1A (TTL & Trim Consistency):**
     - `trimWindow := max(joinWindow, recentWindow)` (never trim entries still inside `joinWindow` even if `recentWindow` is lowered to 1h and `joinWindow` is 10m; currently `1h > 10m` anyway, `max()` guarantees the invariant!).
     - `joinedTTL := max(raidJoinedTTL, trimWindow + time.Hour)` — for default `recentWindow=24h`, returns `25h` (byte-identical to `raidJoinedTTL`). For `recentWindow=168h` (7 days), returns `169h` so the joined set never expires while members are still considered "new"!

2. **`fuku/modules/raid_join.go`**
   - Lines 183-199: `raidRecordAndFire` calls `recordRaidJoins(chat.Id, userIDs)` and on trigger constructs:
     ```go
     raidFire(b, chat, raidTrigger{
         Kind:   models.LockdownTriggerJoinSurge,
         Count:  count,
         Window: int(joinWindow / time.Second), // line 196: was raidJoinWindow
     })
     ```
   - `raidTrigger.Window` flows into `ChatLockdown.TriggerWindow` (`raid_trigger.go:299`), `lockdownTriggerSentence` (`raid_trigger.go:349`, `{window}`), `lockdownAlertTriggerLine` (`lockdown_alert.go:609`), and Phase 25 custom lockdown messages (`{window}`). No locale string changes needed — all 8 languages already say `in {window} s` (`locales/en.yml:1671`).

3. **`fuku/modules/raid_flood.go`**
   - Lines 56-65: `raidFloodWindow = 30 * time.Second`, `raidFloodNewPercent = 60`, `raidFloodMinAccounts = 3`, `raidFloodTTL = 2 * time.Minute`.
   - Lines 100-119: `raidFloodScript` — **ALREADY parameterized via ARGV!**
     - `ARGV[2]` = flood window (ms)
     - `ARGV[3]` = recent window (ms)
     - `ARGV[7]` = TTL (ms)
   - Lines 207-216: `raidFloodCounts(items, newItems, accounts, threshold int)` compares:
     ```go
     percent = newItems * 100 / items
     fire = items >= threshold &&
         newItems*100 >= newPercent*items && // line 213: was raidFloodNewPercent
         accounts >= minAccounts             // line 214: was raidFloodMinAccounts
     ```
   - Lines 242-267: `recordRaidFloodItem` passes `floodWindow`, `recentWindow`, `floodTTL`.
   - **TRAP 1B (Flood TTL vs Existing Test):**
     - `fuku/modules/raid_flood_test.go:615` asserts `ttl > 0 && ttl <= raidFloodTTL` (`2 * time.Minute`) for default chats!
     - Use `floodTTL := max(raidFloodTTL, floodWindow*2)`. For default `30s`, `max(2m, 1m) = 2m` (exact match with existing test!). For `300s` (5 min), `floodTTL = 10m` (always outlives window).
   - Lines 294-344: `raidFloodCountUpdate` — **HOT PATH (runs on every supergroup message!)**:
     - Already checks pure Go guards first (supergroup, sender shape, item key), then Redis, then `raidSettingsLookup(settingsCtx, chat.Id)` (Redis `skipLocal`), returning immediately if `!settings.FloodEnabled`, then approved/admin exemptions.
     - Read `advThresholdsLookup(settingsCtx, chat.Id)` ONLY after `settings.FloodEnabled` and exemptions pass!
     - Because `chat_advanced_thresholds:<chat>` is cached in Redis (`DefaultCacheTTL = 30m`, listed in `skipLocal`), warm reads cost **0 DB round trips per message** (1 Redis GET, identical to `raid_detection:<chat>`).
     - On read error: log debug and **fall back to defaults** (or treat unreadable gracefully — wait: falling back to defaults vs counting nothing; note `raidSettingsLookup` error counts nothing, whereas `advThresholdsLookup` returning defaults on transient error keeps today's raid protection alive rather than disabling flood protection; document this clearly and test!).
   - Line 339: `raidTrigger{Kind: models.LockdownTriggerFlood, Count: items, Window: int(floodWindow / time.Second), Percent: percent}`.
   - Lines 378-396: `raidSnapshotFlooders(chatID int64)` passes `recentWindow.Milliseconds()` to `raidFloodersScript` (`ARGV[2]`). **MUST pass the chat's `recentWindow`, NOT hardcoded `raidRecentWindow`!**

4. **`fuku/modules/raid_ai.go`**
   - Lines 116-159: `raidAICandidateScript` & `raidAICheckCandidate` pass `ARGV[2] = recentWindow.Milliseconds()` (line 146).
   - Called from `raidAIWatchText` (line 245) and `raidAIWatchImage` (line 304) ONLY after `settings.AIEnabled` is true.
   - **TRAP 1C (All 5 `raidRecentWindow` Sites!):**
     `raidRecentWindow` is used in **5 distinct places** — ALL 5 must read the chat's `recentWindow`:
     1. `raid_counters.go:110` (`recordRaidJoins` → `raidJoinScript` trims joined set)
     2. `raid_flood.go:254` (`recordRaidFloodItem` → `raidFloodScript` checks `joinedAt > now - recentWindow`)
     3. `raid_flood.go:392` (`raidSnapshotFlooders` → `raidFloodersScript` keeps recent joiners for alert)
     4. `raid_ai.go:146` (`raidAICheckCandidate` → `raidAICandidateScript` checks if sender is new for AI text & image checks)
     5. `raid_ai_count.go:255` (`raidSnapshotAISpammers` → `raidAISpammersScript` keeps recent joiners for alert)

5. **`fuku/modules/raid_ai_count.go`**
   - Lines 33-40: `raidAIWindow = 120 * time.Second`, `raidAIMinAccounts = 3` (fixed), `raidAITTL = 5 * time.Minute`.
   - Lines 68-82: `raidAIVerdictScript` — **ALREADY parameterized via ARGV!** (`ARGV[2]` = window ms, `ARGV[6]` = TTL ms).
   - **TRAP 1D (AI TTL Consistency):**
     - `aiTTL := max(raidAITTL, aiWindow*2)`. For default `120s`, `max(5m, 4m) = 5m` (exact match with `raidAITTL`). For `900s` (15 min), `aiTTL = 30m`.
   - Line 201: `raidTrigger{Kind: models.LockdownTriggerAIBurst, Count: items, Window: int(aiWindow / time.Second)}`.
   - Line 255: `raidSnapshotAISpammers` passes `recentWindow.Milliseconds()` to `raidAISpammersScript`.

---

### 2. AI Spam Bars (`raidAIImageBar` 0.85 and `memberReqAIBar` 0.90)

#### 2A. Raid Image Spam Bar — `fuku/modules/raid_ai_image.go`
- Line 35: `const raidAIImageBar = 0.85`
- Lines 268-294: `raidAICachedVerdict` / `raidAIStoreVerdict` (`fuku:raid:aiv:<file_unique_id>`, TTL 1h):
  - **GOOD NEWS:** Cache stores the **RAW `float64` probability** (`strconv.FormatFloat(probability, 'f', -1, 64)`), NOT a boolean!
  - Singleflight (`raidAIImageFlight`, line 171) is keyed per-chat (`chatID + ":" + uid`) and returns `raidAIImageResult{probability: float64}`.
- Line 194:
  ```go
  bar := advThresholdsFor(ctx, job.chat.Id).RaidAIImageBar()
  spam := result.probability >= bar
  ```
  Changing a group's bar acts immediately on the very next image, including images whose probability is already cached in `fuku:raid:aiv:*`!

#### 2B. Member Requirement AI Spam-Name Bar — `fuku/modules/memberreq_ai.go` & `fuku/modules/memberreq_watch.go`
- Line 73 (`memberreq_ai.go`): `const memberReqAIBar = 0.9`
- Line 512 (`memberreq_ai.go`): Cache `fuku:memberreq:nv:<verdictKey>` stores the **RAW `float64` probability**, NOT a boolean!
- **CRITICAL TRAP 2A — Cross-Chat Singleflight Verdict Leak (`memberreq_ai.go:385-392, 483-520`):**
  ```go
  // TODAY (BUG IF BAR IS PER-CHAT!):
  answer, _ := memberReqFlightValue(ctx, "n:"+plan.key, func(flightCtx context.Context) memberReqAIAnswer {
      return memberReqAIJudge(flightCtx, chatID, userID, plan)
  })
  verdict = answer.Verdict
  ```
  Because `"n:"+plan.key` has **no `chatID`**, two concurrent chats judging the same name share ONE flight! Today `memberReqAIJudge` returns `memberReqAIAnswer{Verdict: memberReqAIVerdictOf(probability)}` computed with one bar. If Chat A (bar 0.70) is the leader and Chat B (bar 0.95) is the follower for a name with probability 0.80, Chat B would falsely receive `memberReqBreach`!
  - **REQUIRED FIX:** Have `memberReqAIAnswer` carry raw probability (`Probability float64`, `HasProbability bool`, `Sticky bool`) from `memberReqAIJudge` (without baking a chat-specific verdict inside the shared flight), then apply `memberReqAIVerdictOfBar(answer.Probability, bar)` in `memberReqAIRule` for each caller's own `chatID`!
  - Also update `memberReqAIPrepare` / `memberReqAIDecide` (line 354) to pass `bar` to `memberReqAIVerdictOfBar(probability, bar)`.
- **CRITICAL TRAP 2B — Watcher Fast-Path Comparison (`memberreq_watch.go:212-251`):**
  ```go
  // memberreq_watch.go:243-251
  func memberReqAIWatchSatisfied(verdictValue, baseline string, marker bool, hash string) bool {
      if marker {
          return false
      }
      if probability, ok := memberReqAIParseProbability(verdictValue); ok && probability < memberReqAIBar {
          return true
      }
      return baseline != "" && baseline == hash
  }
  ```
  If Chat A lowers its bar to `0.70` and the cached probability is `0.80`, the watcher MUST compare `probability < bar` (`0.80 < 0.70` → false → starts job!) instead of `probability < 0.9`!
  - Read `advThresholdsFor(ctx, chatID).MemberReqAIBar()` in `memberReqLookupCachedWith` ONLY when `rules.AI` is true (zero extra reads when the AI rule is off!).
  - Pass `bar` into `aiRead.satisfied(bar)` → `memberReqAIWatchSatisfiedBar(verdict, baseline, marker, r.hash, bar)` (keep `memberReqAIWatchSatisfied` delegating with `memberReqAIBar` so existing unit tests compile cleanly!).

---

### 3. Member Requirement Cap (10/min) and Pause Duration (10 min)

#### Exact Existing Sites (`fuku/modules/memberreq_limits.go`)
- Lines 25-32:
  ```go
  var (
      memberReqCapPerMinute = 10
      memberReqPause = 10 * time.Minute
  )
  ```
- Line 36: `const memberReqCapCounterTTL = 120 * time.Second` (fixed 2× one-minute bucket; unchanged regardless of cap or pause).
- Lines 59-66: `memberReqCapScript` — counts distinct users in the current Unix minute (`KEYS[1] = capm`, `KEYS[2] = cap`), returns total count. Does NOT hardcode the cap!
- Lines 75-127: `memberReqCapAllow` compares `count <= int64(cap)` (line 93), and `memberReqPauseChat` calls `client.SetNX(latchCtx, memberReqPausedKey(chatID), "1", pause)` (line 110) and formats `mreq_notice_paused` with `"n": strconv.Itoa(cap)` and `"minutes": strconv.Itoa(int(pause / time.Minute))` (lines 121-122).
- **TRAP 3A — Package Var Test Seams:**
  Multiple existing tests swap `memberReqCapPerMinute` and `memberReqPause` via `mreqSwap(t, &memberReqCapPerMinute, 1)` (`memberreq_limits_test.go`, `memberreq_notice_schedule_test.go`, `screening_notice_legacy_request_test.go`).
  - **REQUIRED PATTERN:** When the stored threshold is `0` (unset / default), return the package variable `memberReqCapPerMinute` / `memberReqPause` (NOT hardcoded literals `10` / `10*time.Minute`)! When non-zero, return the group's stored value. All existing tests remain 100% green!
- **Pause notice (`mreq_notice_paused`):** All 8 locales already parameterize `{n}` and `{minutes}` (verified in `locales/*.yml`). No locale changes needed for the notice itself.

---

### 4. Lockdown Reminder Interval (`lockdownReminderEvery`, 1 hour)

#### Exact Existing Sites (`fuku/modules/lockdown_alert.go`)
- Line 78: `var lockdownReminderEvery = time.Hour`
- Lines 375-384:
  ```go
  func lockdownReminderSeq(ld *models.ChatLockdown, now time.Time) int {
      if ld.AlertStartedAt == nil || lockdownReminderEvery <= 0 {
          return 0
      }
      elapsed := now.Sub(*ld.AlertStartedAt)
      if elapsed < 0 {
          return 0
      }
      return int(elapsed / lockdownReminderEvery)
  }
  ```
- **BOTH call sites of `lockdownReminderSeq`:**
  1. `lockdown_alert.go:426` (`lockdownPlanReminders`): decides whether a reminder `seq >= 1` is due for `ld` (`ld.ChatID`).
  2. `lockdown_alert.go:776` (`lockdownDeliverAlert`): checks `if slot.Seq < lockdownReminderSeqFor(ld, time.Now(), interval)` to supersede late/stale reminders.
- **TRAP 4A — Package Var Test Seam:**
  `lockdown_reminder_test.go` modifies `lockdownReminderEvery` directly in tests. When `LockdownReminderMins == 0` (default), `lockdownReminderIntervalFor(ctx, ld.ChatID)` MUST return `lockdownReminderEvery` (NOT a hardcoded `time.Hour`)!
- **TRAP 4B — Consistent Both Sites:**
  Both `lockdownPlanReminders` (line 426) AND `lockdownDeliverAlert` (line 776) MUST use the exact same `lockdownReminderIntervalFor(ctx, ld.ChatID)` helper so a reminder planned with a 15-min interval is not falsely marked superseded against 60 min (or vice versa).
- **EVT-10 Verified:** `lockdownRenderReminder` (`lockdown_alert.go:684-715`) uses `lockdown_reminder_title` and `lockdown_reminder_counts` — neither hardcodes "1 hour", and neither touches the event message engine or PM copies.

---

### 5. Captcha Timeout Above Today's 10-Minute Cap

#### Complete Audit of Existing Captcha Timeout Sites & Constraints
1. **SQL CHECK constraints (TWO of them on `captcha_settings.timeout`!):**
   - `migrations/20250807103246_add_captcha_tables.sql:7` — unnamed inline `CHECK (timeout > 0 AND timeout <= 10)` (`captcha_settings_timeout_check`)
   - `migrations/20260412000000_add_missing_check_constraints.sql:16-18` — named `chk_captcha_timeout_range CHECK (timeout BETWEEN 1 AND 10) NOT VALID`
2. **GORM model:** `fuku/db/models/captcha.go:10` — `check:chk_captcha_timeout_range,timeout BETWEEN 1 AND 10`
3. **DB constraint test:** `fuku/db/constraints_test.go:87-105` (`TestCaptchaSettingsConstraint_TimeoutRangeIntegration`) — explicitly asserts `timeout = 11` **fails**!
4. **Repository:** `fuku/db/captcha/repository.go:97-116` — `SetCaptchaTimeout` rejects `timeout < 1 || timeout > 10` (`ErrInvalidTimeout`).
5. **Cache:** `captcha_settings:<chat>` is **NOT** in `skipLocal` (`fuku/db/cache/local.go`).
6. **Command & apply:** `fuku/modules/captcha.go:825-828` (`applyCaptchaTimeout`) rejects `minutes < 1 || minutes > 10`, and `fuku/modules/captcha_command_test.go:686` explicitly asserts `/captchatime 11` is refused!
7. **Settings page:** `fuku/modules/settings_captcha.go:33-34` (`settingsCaptchaMinTime = 1`, `settingsCaptchaMaxTime = 10`, stepper `±1`).
8. **Attempt creation & timers (ALREADY support arbitrary minutes > 10!):**
   - `fuku/db/captcha/repository.go:175`: `ExpiresAt: time.Now().UTC().Add(time.Duration(timeout) * time.Minute).Truncate(time.Microsecond)` — NO 10-min cap.
   - `fuku/modules/captcha.go:1291, 1320, 1342` (`sendCaptcha`): uses `timeout` for `CreateCaptchaAttemptPreMessageIfEnabled`, `captcha_web_welcome` `"number"`, and `captchaChallengeText` `"number"`.
   - `fuku/modules/captcha.go:1417` (`scheduleCaptchaTimeout`): `delay := time.Until(attempt.ExpiresAt)` — NO 10-min cap.
   - `fuku/modules/captcha.go:2114` (refresh handler): `remainingMinutes := int(time.Until(attempt.ExpiresAt).Minutes())` — NO 10-min cap.
   - `fuku/utils/captchaweb/challenge.go:129` (`Store.Issue`): `ttl := time.Until(rec.ExpiresAt) + ChallengeGrace` — NO 10-min cap.
   - `fuku/db/captcha/repository.go:248` (`HasPendingCaptchaAttemptsContext`): queries `expires_at > NOW()`, invalidated on creation — works for any timeout.
   - `fuku/db/backup/backup.go:335, 388, 721-733`: exports/imports `captcha_settings` directly.

#### Recommended Architecture: Option A — Override in `chat_advanced_thresholds.captcha_timeout_mins`
Store `captcha_timeout_mins INTEGER NOT NULL DEFAULT 0` in `chat_advanced_thresholds` (CHECK `captcha_timeout_mins = 0 OR captcha_timeout_mins BETWEEN 11 AND 60`, or `BETWEEN 1 AND 60` — `0` = unset / defer to `captcha_settings.timeout`).
- In `fuku/modules/captcha.go`:
  - Add `effectiveCaptchaTimeout(ctx context.Context, chatID int64, baseTimeout int) int`: if `advThresholdsFor(ctx, chatID).CaptchaTimeoutMins > 0`, return it; else return `baseTimeout`.
  - Call `effectiveCaptchaTimeout` in `sendCaptcha` (`captcha.go:1291, 1320, 1342`) so the next joiner immediately gets the extended timeout for `ExpiresAt`, challenge message text, Turnstile challenge TTL, and timeout timer!
  - Optionally also display the effective timeout in `/captcha` status (`captcha.go:501`) and `settings_captcha.go` (line 153) when overridden, while leaving `settings_captcha` stepper untouched for 1..10.
- **Why Option A beats Option B (widening `captcha_settings.timeout`):**
  1. **Zero breaking changes** to `TestCaptchaSettingsConstraint_TimeoutRangeIntegration`, `TestCaptchaSubcommandsRejectInvalidValues` (`/captchatime 11`), `settings_captcha.go` tests, or `/export` backup files (tuning stays out of backups per ROADMAP!).
  2. **Instant cross-replica effect** because `chat_advanced_thresholds:<chat>` is in `skipLocal` (no need to alter `captcha_settings` caching policy!).
  3. **Clean Reset to default**: resetting Captcha timeout on the Advanced page sets `captcha_timeout_mins = 0`, restoring the group's Captcha page setting without clobbering whatever 1..10 value the admin set via `/captchatime`!
  4. **No dual-constraint migration hazard** on `captcha_settings` (unlike `20261030120000_allow_turnstile_captcha_mode.sql`).

*(If Option B were ever chosen instead, the migration MUST use the exact DO-block loop from `migrations/20261030120000_allow_turnstile_captcha_mode.sql:29-49` to drop both `chk_captcha_timeout_range` AND the unnamed `captcha_settings_timeout_check`, plus update `models/captcha.go:10`, `constraints_test.go:97`, `captcha_command_test.go:686`, `db/captcha/repository.go:98`, and add `captcha_settings:` to `skipLocal`.)*

---

### 6. Storage, Migration, Cache (`skipLocal`), Backup Exclusion & Test Harness

#### Next Migration Timestamp
- Latest migration: `migrations/20261122150000_add_manual_command_tuning.sql`
- **Next migration:** `migrations/20261123120000_add_advanced_thresholds.sql`
- Pattern to copy from `migrations/20261122150000_add_manual_command_tuning.sql` (lines 18-30):
  - Single table `chat_advanced_thresholds`, `PRIMARY KEY (chat_id)`, NO foreign key on `chats` (so reads/writes never depend on `chats` rows, matching `chat_manual_defaults` and `chat_escalation`), idempotent `CREATE TABLE IF NOT EXISTS`, inline named `CHECK` constraints matching every Go bound (`col = 0 OR col BETWEEN min AND max`), `updated_by BIGINT NOT NULL DEFAULT 0`, `updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()`.

#### GORM Model (`fuku/db/models/advthresholds.go`)
- Copy `fuku/db/models/modtuning.go` (lines 16-33):
  - Struct `ChatAdvancedThresholds` with `TableName() string { return "chat_advanced_thresholds" }`.
  - Mirror every SQL `CHECK` constraint in GORM `check:` tags so SQLite tests enforce the same bounds as PostgreSQL.
  - Export bound constants (`RaidJoinWindowMinSecs`, `RaidJoinWindowMaxSecs`, etc.) or keep them in `fuku/db/advthresholds` as the single source of truth.

#### Repository (`fuku/db/advthresholds/repository.go`)
- Copy `fuku/db/modtuning/repository.go` (lines 24-140):
  - `cacheModule = "chat_advanced_thresholds"`, `CacheKey(chatID)` returns `cache.CacheKey(cacheModule, chatID)`.
  - `Get(ctx, chatID)` → `cache.GetFromCacheOrLoad(ctx, CacheKey(chatID), cache.DefaultCacheTTL, ...)` calling `GetFresh`. Missing row returns zero `Thresholds{}, nil` and **creates no row** (`TestPhase28ReadsCreateNothing`!).
  - Column-targeted upsert (`clause.OnConflict{Columns: []clause.Column{{Name: "chat_id"}}, DoUpdates: clause.AssignmentColumns([]string{col, "updated_by", "updated_at"})}`) so two admins tuning two fields never clobber each other!
  - Every setter validates bounds in Go BEFORE any DB write (returns `ErrInvalidValue`) and calls `cache.DeleteCache(CacheKey(chatID))` AFTER commit!
  - Support `ResetField(ctx, chatID, field, by)` (sets column to `0`), `ResetArea(ctx, chatID, area, by)` (sets all columns in area to `0` in one upsert), and `ResetAll(ctx, chatID)` (deletes row + `DeleteCache`).

#### `skipLocal` Registration (`fuku/db/cache/local.go` + `local_test.go`)
- Analog: `fuku/db/cache/local.go` lines 59-60, 117-118:
  ```go
  chatAdvancedThresholdsPrefix = CacheKey("chat_advanced_thresholds") + ":"
  // in skipLocal(key):
  strings.HasPrefix(key, chatAdvancedThresholdsPrefix)
  ```
- Test in `fuku/db/cache/local_test.go` (after line 351): assert `skipLocal(CacheKey("chat_advanced_thresholds", int64(-100123))) == true` and `skipLocal(CacheKey("chat_advanced_thresholdsx", int64(1))) == false`.

#### Backup Exclusion & Test Harnesses
- `fuku/db/backup/testmain_test.go`: add `&models.ChatAdvancedThresholds{},` after line 88 (`&models.ChatReasonShortcut{},`).
- `fuku/db/backup/advthresholds_backup_test.go`: copy `fuku/db/backup/modtuning_backup_test.go` (export, import, clear all leave `chat_advanced_thresholds` untouched).
- `fuku/modules/test_harness_test.go`: add `&models.ChatAdvancedThresholds{},` after line 391 (`&models.ChatReasonShortcut{},`).

---

### 7. Private Advanced Settings Page (`fuku/modules/settings_advthresholds.go`)

#### Free Page Key, Order Slot, Icon & Action Codes (All Verified Free!)
All 26 single-letter keys `a`..`z` are taken (Phase 27 took `q` and `rs`). Verified by scanning all `fuku/modules/settings*.go`:
- **Page Key:** `settingsPageAdvanced = "av"` — **FREE**
- **Order:** `81` (free, sits right after Member requirements `80` and before Deletion `85`; no tie with any other page!)
- **Icon:** `"🎛"` (or `"🎚"` — verify 1-2 rune display width in `TestSettingsLabelLengths`, `gridMax = 20`!)
- **TitleKey:** `"settings_page_advanced"` (short in all 8 locales so `icon + " " + title <= 20` runes!)
- **Visibility:** `PrivateOnly: true`, `PrivateNoteKey: "settings_adv_group_note"`, `Hidden: settingsAdvHidden` (hides page in Staff Groups via fresh `staff.GetStaffGroupFresh`, matching `settings_moddefaults.go:134-137` and `settings_escalation.go:42`!).
- **Rights:**
  - Viewing Main, Hub, Area screens, Threshold screens: `settingsRightAdmin` (plus the settings menu's owner gate `OwnerGate` on private `/settings`!).
  - Setting or resetting any threshold / area: `settingsRightRestrict` (matches `settings_lockdown.go`, `settings_mreq.go`, `settings_escalation.go`, `settings_moddefaults.go` — live `creator` or `can_restrict_members`!).
- **Free 2-letter action codes (verified zero collisions across all 166 2-letter literals in `settings*.go`):**
  - `settingsActAdvArea = "aa"` — open area screen (`Fields: []string{"k"}`, `Right: settingsRightAdmin`)
  - `settingsActAdvItem = "ao"` — open single threshold screen (`Fields: []string{"t"}`, `Right: settingsRightAdmin`)
  - `settingsActAdvSet  = "as"` — set threshold to preset value (`Fields: []string{"t", "v"}`, `Right: settingsRightRestrict`)
  - `settingsActAdvReset = "ar"` — reset one threshold to default (`Fields: []string{"t"}`, `Right: settingsRightRestrict`)
  - `settingsActAdvResetArea = "ax"` — reset one entire area to defaults (`Fields: []string{"k"}`, `Right: settingsRightRestrict`)
- **Step-1 strict validation (copy `settings_moddefaults.go:59-61`):**
  - Wrap every `Validate` in `private(check)` requiring `fields["c"] != ""` so any group-menu press (`u`), extra key, unknown area/threshold code, or out-of-range value is refused in step 1 with **0 `getChatMember` calls** and **0 DB reads**!
- **64-byte callback budget (worst case `c=-1009999999999`):**
  - `set|v1|a=as&c=-1009999999999&t=lrm&v=1440` = **41 bytes** (well under 64-byte Telegram limit).
  - `set|v1|a=ax&c=-1009999999999&k=rw` = **33 bytes**.

#### UI Structure (Table-Driven — Copy `settings_deletion.go` + `settings_moddefaults.go`)
- **Hub screen (`av`):** Lists the 6 areas with how many values in each area are customized vs Default (e.g., `Default` or `N custom`), plus one button per area (2-column grid = 3 rows) and Back / Close.
- **Area screen (`aa`, `k=<area>`):** Lists each threshold in that area showing its **current value, default, and allowed range** (Success Criterion 1!), one button per threshold to edit it, a **"Reset area to default"** button (`ax`) when any value in the area is non-default (or always visible; idempotent), plus Back to Advanced & Close.
- **Threshold screen (`ao`, `t=<item>`):** Shows the threshold's description, **current setting, default value, and allowed range**, curated in-bounds **preset choice buttons** (2–3 per row, selected prefixed `✅ `, canonical values only — every preset validated in step 1 AND in Go apply/repo bounds!), a **"Reset to default"** button (`ar`), and Back to Area & Close.
  - Using curated preset buttons inside the safe bounds (like `settings_deletion.go` and `settings_duration.go`) guarantees zero typed-input parsing ambiguity, zero out-of-range taps, and minimal test surface, while still validating arbitrary integer bounds in Go (`applyAdvancedThreshold` + DB `CHECK`) so forged/hand-built values outside bounds are always rejected!
  - *(Optional alternative: if typed numeric input is also desired alongside presets, `settingsTypedPages[settingsPageAdvanced]` can be wired using `settings_moddefaults.go:102-108` with cancel code `"ac"`? Wait: `"ac"` is taken by `settings_locks.go` — use `"az"` which is FREE!)*

#### Main Grid & Guard Tests That MUST Be Updated
1. **`fuku/modules/settings_main_test.go`:**
   - Lines 36-38:
     - `settingsMainShapeGroup`: insert `av` between `m` (80) and `d` (85):
       `"s|l / c|f / q|n / j|k / rs|b / i|w / g|r / e|x / h|m / av|d / t|u / z|o / y / p / a / close"`
     - `settingsMainShapePrivate`:
       `"s|l / c|f / q|n / j|k / rs|b / i|w / g|r / e|x / h|m / av|d / t|u / z|o / y / p / a / groups|close"`
     - `settingsMainOwnerRows = 16` (was `15`, because 25 grid cells = 13 grid rows + `p` + `a` + nav = 16 rows!).
   - Lines 573-578 (`settingsMainTitleKeys`): insert `"settings_page_advanced"` between `"settings_page_mreq"` and `"settings_page_del"`.
2. **`fuku/modules/settings_fwdblock_test.go`:**
   - Line 349: `staffOwnerRows = settingsMainOwnerRows - 3` (was `- 2`! Because `av` is hidden in Staff Groups, a Staff Group still has 13 rows while a normal group now has 16 rows!).
3. **`fuku/modules/settings_phase28_test.go` (copy `settings_phase27_test.go`):**
   - `TestPhase28Codes`: verifies all `av` action codes (`aa`, `ao`, `as`, `ar`, `ax`), verifies all 28 pages are registered, and asserts extra keys and group-menu presses fail in step 1 with 0 `getChatMember` calls.
   - `TestPhase28ReadsCreateNothing`: opening Main, `av`, all 6 area screens, and all 12 threshold screens creates 0 rows in `chat_advanced_thresholds` and `chats`.
   - `TestPhase28StaffPrivacyAndLogs`: linked group with sentinel Staff Group title, exercises setting & resetting thresholds, verifies zero Staff Group leaks in UI/logs and verifies `av` never appears in a Staff Group's Main menu (group or private).
4. **`fuku/i18n/settings_locale_test.go`:**
   - Add every new `settings_adv_*` / `settings_page_advanced` key to the length-cap map (after line 805).
   - Add `{"🎛", "settings_page_advanced"}` to `TestSettingsLabelLengths` (lines 1026-1054).
   - Remember `TestLocaleStyle` (`fuku/i18n/locale_style_test.go`) enforces `docs/src/content/docs/contributing/locale-style.md` on `en.yml` and `uk.yml`, and `TestLocalePlaceholders` enforces identical placeholder sets across all 8 locales!

---

## Shared Patterns

### MSG-11 Goldens First (Wave 1, Committed Alone)
Before touching ANY production file whose constant moves, Plan 28-01 must record characterization tests pinning today's default behavior for an unconfigured group:
1. Raid join surge triggers at 60 s window and reports `{window} = 60` (`lockdown_trigger_join_surge`), does not trigger when joins fall outside 60 s.
2. Raid flood triggers at 30 s window, 60% new-member share, 3 new accounts, 24 h newness age, and reports `{window} = 30`, `{percent} >= 60` (`lockdown_trigger_flood`).
3. Raid AI burst triggers at 120 s window and reports `{window} = 120` (`lockdown_trigger_ai_burst`); raid image check treats `0.85` as spam and `0.84` as kept.
4. Member requirement AI rule treats `0.90` as breach and `0.89` as pass; member requirement brake pauses at >10 members/min for 10 min and posts `mreq_notice_paused` with `n=10`, `minutes=10`.
5. Lockdown reminder posts first reminder at 1 hour (`lockdownReminderEvery`).
6. Captcha joiner challenge uses default `captcha_settings.timeout` (2 min, or admin's 1..10 min setting) for `ExpiresAt` and challenge text.

### Zero New Shutdown Drains
Phase 28 adds **zero** goroutines and **zero** shutdown drains (all threshold reads are synchronous cached lookups under bounded 2 s contexts inside existing handlers/workers). The 70 s webhook worst-case drain figure and `stop_grace_period: 75s` in `AGENTS.md` stay unchanged.

### EVT-10 Structural Guard
No threshold setting, page, or helper may route Staff Group actions, lockdown alerts/reminders, raid-scale removals, log channel posts, or captcha challenges through the event message engine or PM copies. Changing `lockdownReminderEvery` only changes *when* the existing reminder planner schedules `AlertKindReminder`, never its text or destinations.

---

## No Analog Found

All 24 files have exact or in-place codebase analogs. No greenfield/unmatched files.

## Metadata

**Analog search scope:** `fuku/modules/`, `fuku/db/`, `fuku/i18n/`, `fuku/utils/captchaweb/`, `migrations/`, `locales/`
**Files scanned:** 42
**Pattern extraction date:** 2026-11-23
