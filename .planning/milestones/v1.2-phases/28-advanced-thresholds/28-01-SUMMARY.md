---
phase: 28-advanced-thresholds
plan: 01
subsystem: testing
tags: [golden-tests, msg-11, raid-detection, member-requirements, lockdown, captcha, miniredis, race]

requires:
  - phase: 27-manual-command-tuning
    provides: Baseline commit 654c8311 prior to Phase 28 production edits
provides:
  - Standalone MSG-11 characterization golden test suite pinning all 12 default threshold behaviors in TestLegacyPhase28DefaultThresholds
  - Sandbox-safe in-process miniredis helper phase28InProcessMiniredis using net.Pipe() and mr.Server().ServeConn with zero localhost TCP dials
affects: [28-02, 28-03, 28-04, 28-05, 28-06]

tech-stack:
  added: []
  patterns:
    - "In-process miniredis net.Pipe() Dialer (mr.Server().ServeConn) for sandboxed Redis Lua execution under -race"
    - "MSG-11 standalone characterization golden commit ahead of all production changes"

key-files:
  created:
    - fuku/modules/advthresholds_phase28_legacy_request_test.go
  modified: []

key-decisions:
  - "Used in-process miniredis net.Pipe() Dialer (phase28InProcessMiniredis) so every Redis Lua script executes without localhost TCP dials in both sandboxed environments and CI."
  - "Scoped UTC NowFunc and real en.yml locale override inside the captcha subtest with GetCaptchaAttemptIncludingExpired so chk_captcha_expires_at and attempt lookup pass deterministically in non-UTC local timezones without modifying production or existing test harness files."

patterns-established:
  - "phase28InProcessMiniredis(t): reusable in-process Redis Lua test helper with LIFO cleanup order (restore cache state -> close client -> close miniredis)."

requirements-completed: [TUNE-07]

status: complete
plan_head_before: 98b4992bce1db13de3d9cc862116d1d9660cfd57
plan_head_after: 48d5c240b8aa869e2f0ab958c1902afd24957772
commits: 1

actuals:
  tokens: 6566
  tasks: 2
  commits: 1

duration: 9min
completed: 2026-11-23
---

# Phase 28 Plan 01: Phase 28 Default-Threshold Characterization Goldens Summary

**Standalone MSG-11 characterization golden suite (`TestLegacyPhase28DefaultThresholds`) pinning all 12 unchanged default threshold behaviors across 8 subtests over an in-process `net.Pipe()` miniredis dialer, committed alone before any Phase 28 production edit**

## Performance

- **Duration:** 9 min
- **Started:** 2026-11-23T08:40:13Z
- **Completed:** 2026-11-23T08:49:48Z
- **Tasks:** 2
- **Files modified:** 1 (created `fuku/modules/advthresholds_phase28_legacy_request_test.go`; 0 production files touched)

## Accomplishments

- Created `fuku/modules/advthresholds_phase28_legacy_request_test.go` (`//go:build testtools`) with `phase28InProcessMiniredis(t)` connecting a real `*redis.Client` to `*miniredis.Miniredis` via `net.Pipe()` + `mr.Server().ServeConn` (zero localhost TCP dials) and restoring `cache.SetCacheState` in `t.Cleanup`.
- Recorded and verified all 8 subtests in `TestLegacyPhase28DefaultThresholds` on unchanged code at baseline `654c8311`:
  1. `raid_join_window_60s_and_recent_24h` — 60 s surge window (`TriggerWindow == 60`), 24 h joined-set retention/trimming, `24h < TTL <= 25h`.
  2. `raid_flood_window_30s_share_60_floor_3_recent_24h` — 30 s flood window (`TriggerWindow == 30`), 60% new-member share (`TriggerPercent >= 60`), 3-account floor, 24 h newness age, `TTL <= 2m`, and `raidSnapshotFlooders` 24 h filtering.
  3. `raid_ai_window_120s_and_recent_24h` — 120 s AI burst window (`TriggerWindow == 120`), 24 h newness check in `raidAICheckCandidate`, `TTL <= 5m`, and `raidSnapshotAISpammers` 24 h filtering.
  4. `raid_ai_image_bar_085` — `raidAIImageBar == 0.85` (cached `0.85` increments `raidAISpam`, `0.84` increments `raidAIKept`) via `raidAIProcessImage` with outbound HTTP blocked.
  5. `memberreq_ai_bar_090` — `memberReqAIVerdictOf`, `memberReqAIDecide`, and `memberReqAIWatchSatisfied` boundary at `0.90` breach vs `0.89` pass.
  6. `memberreq_cap_10_and_pause_10m` — 10 distinct users allowed per minute, 11th refused, pause latch TTL `10m`, and `mreq_notice_paused` carrying `n=10`, `minutes=10`.
  7. `lockdown_reminder_every_1h` — `lockdownReminderSeq` returns `0` at `59m`, `1` at `60m`/`61m`, and `2` at `120m`.
  8. `captcha_timeout_default_2m_and_configured_5m` — `SendCaptcha` creates `captcha_attempts` with `ExpiresAt` matching default 2 min and configured 5 min and renders the matching timeout in the challenge text.
- Verified `gofmt`, `go vet -tags testtools ./fuku/modules`, `CGO_ENABLED=0 go build ./...`, `go test -tags testtools -race -count=1 -run '^TestLegacyPhase28DefaultThresholds$' -v ./fuku/modules` (8/8 subtests passing in ~0.39 s), zero production edits since `654c8311`, and unchanged `go.mod`/`go.sum`.
- Committed the golden test file alone in `48d5c240`.

## Task Commits

1. **Task 1 & Task 2: Record and commit Phase 28 default-threshold characterization goldens alone** - `48d5c240` (`test(28-01): pin default threshold behaviors before Phase 28`)

## Files Created/Modified

- `fuku/modules/advthresholds_phase28_legacy_request_test.go` — Standalone MSG-11 characterization golden test suite (`TestLegacyPhase28DefaultThresholds`) and `phase28InProcessMiniredis` helper.

## Decisions Made

- Used `raidAIProcessImage` (the actual symbol in `fuku/modules/raid_ai_image.go`) to exercise cached `0.85` / `0.84` probabilities directly without network calls, matching the plan's intent.
- Scoped `db.DB.Config.NowFunc = func() time.Time { return time.Now().UTC() }`, real `locales/en.yml` via `i18n.OverrideManagerForTest`, and `captcha.GetCaptchaAttemptIncludingExpired` inside the captcha subtest so the golden test passes reliably in non-UTC local timezones as well as UTC CI without touching any production or existing test file.

## Deviations from Plan

None - plan executed exactly as written (minor test-local symbol/timezone accommodations stayed strictly inside the new test file with zero production or existing test file edits).

## Issues Encountered

- Running on a local `+02:00` machine initially tripped SQLite's lexicographical `chk_captcha_expires_at` (`expires_at > created_at`) because GORM's default `NowFunc` writes local timestamps while `captcha_attempts.expires_at` is UTC. Resolved cleanly inside the new test's captcha subtest by scoping `db.DB.Config.NowFunc` to UTC with `t.Cleanup` restoration and loading `locales/en.yml` via `i18n.OverrideManagerForTest`.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- MSG-11 golden baseline is locked in commit `48d5c240` with zero production changes since `654c8311`.
- Ready for Wave 2 (Plan 28-02: migration, model, repository, `skipLocal`, backup exclusion, and drift guard).

## Self-Check: PASSED

- `fuku/modules/advthresholds_phase28_legacy_request_test.go`: FOUND
- Commit `48d5c240`: FOUND (1 file changed, 0 deletions, 0 production files modified since `654c8311`)
- `TestLegacyPhase28DefaultThresholds`: 8/8 subtests PASSED under `-race` with 0 skips
