---
phase: 26-timed-punishments-escalation-and-warn-expiry
fixed_at: 2026-11-24T14:55:00Z
review_path: .planning/phases/26-timed-punishments-escalation-and-warn-expiry/26-REVIEW.md
fix_scope: critical_warning
iteration: 1
findings_in_scope: 6
fixed: 5
skipped: 1
status: partial
---

# Phase 26: Code Review Fix Report

**Fixed at:** 2026-11-24T14:55:00Z
**Source review:** `.planning/phases/26-timed-punishments-escalation-and-warn-expiry/26-REVIEW.md`
**Fix scope:** `critical_warning` (`WR-01`..`WR-06`; Info items `IN-01`, `IN-02` out of scope)
**Iteration:** 1

**Summary:**
- Findings in scope: 6
- Fixed: 5
- Skipped: 1

## Verification

Verification ran in the **main checkout** (`workflow.use_worktrees: false`).

- `go vet -tags testtools ./...` — PASS
- `CGO_ENABLED=0 go build ./...` — PASS
- `make check-translations` — PASS (1068 keys across 8 locales)
- `make check-docs` — PASS (no drift)
- `make test-shard-check` — PASS
- `TZ=UTC go test -tags testtools -race -count=1 ./fuku/db/escalation ./fuku/db/blacklists ./fuku/db/backup` — PASS
- `TZ=UTC go test -tags testtools -race -count=1 ./fuku/db/warns` — all non-miniredis tests pass; only `TestSetWarnModeDuration/every_write_drops_the_warn_settings_key` fails in-sandbox because localhost TCP socket dials (`127.0.0.1:...`) are blocked by the sandbox
- `TZ=UTC go test -tags testtools -race -count=1 -run '^(TestLegacyWarnSettingsRequests|TestLegacyBlacklistRequests|TestLegacyTimedPunishmentRequests|TestEscalation|TestPunishTimed|TestEventEngineCallers|TestPMCopyCallers)' ./fuku/modules` — PASS
- `TZ=UTC go test -tags testtools -race -count=1 -run '^(TestEscalation|TestTimedPunishmentLogsCarryIDsOnly|TestPhase26Cost)$' ./fuku/modules` — PASS

## Fixed Issues

### WR-01: TOCTOU race in escalation ladder edits can create step gaps or an enabled zero-step ladder

**Files modified:** `fuku/db/escalation/repository.go`, `fuku/modules/escalation_settings.go`, `fuku/modules/escalation_settings_test.go`, `fuku/modules/timed_punishment_safety_test.go`
**Commit:** `58e9c2f8`
**Applied fix:** Added transactional, `SELECT ... FOR UPDATE` row-locked repository methods `SetEnabledChecked`, `SetStepChecked`, and `RemoveLastStepChecked` (with sentinel errors `ErrStaleStep` and `ErrLastStepEnabled` and cache invalidation after commit) in `fuku/db/escalation/repository.go`. Wired `applyEscalationEnabled`, `applyEscalationStep`, and `applyEscalationRemoveStep` in `fuku/modules/escalation_settings.go` to use them so concurrent ladder edits serialize atomically and cannot create step gaps or an enabled zero-step ladder. Updated the test write seams in `fuku/modules/escalation_settings_test.go` and `fuku/modules/timed_punishment_safety_test.go`.

### WR-02: `blacklistWatcher` increments `chat_offences` via `escalate` without checking `CanBotRestrict`

**Files modified:** `fuku/modules/blacklists.go`
**Commit:** `5bbdf254`
**Applied fix:** Gated `escalate` and `blacklistPunish` in `blacklistWatcher` on `chat_status.CanBotRestrict(b, ctx, chat)` (returning `ext.ContinueGroups` when the bot lacks restrict permissions), matching `antiflood.go` and `staff_forwardblock_watch.go`.

### WR-04: Nil `db.DB` dereference in `*Context` repository wrappers despite nil-safe `*Tx` helpers

**Files modified:** `fuku/db/blacklists/repository.go`, `fuku/db/warns/repository.go`
**Commit:** `f6857be2`
**Applied fix:** Guarded `db.DB == nil` returning `errors.New("database not initialized")` before calling `db.DB.WithContext(ctx)` in `SetBlacklistActionTimedContext`, `SetWarnExpiryContext`, and `SetWarnModeContext`.

### WR-05: `PunishMaxSecs` (366 days) has no upper-bound clock-skew margin against Telegram's permanent-restriction rule

**Files modified:** `fuku/modules/punish_timed.go`, `fuku/modules/punish_timed_test.go`
**Commit:** `5110fd16`
**Applied fix:** Added `punishMaxMargin = 60 * time.Second` in `fuku/modules/punish_timed.go` and clamped outgoing `until_date` in `applyTimedPunishment` to at most `now.Unix() + int64(punishMaxSecs) - int64(punishMaxMargin/time.Second)` so minor clock skew cannot push a 366-day timed restriction past Telegram's 366-day limit into a permanent one, while keeping `models.PunishMaxSecs` and DB `CHECK` constraints intact. Added a unit test in `fuku/modules/punish_timed_test.go`.

### WR-06: `RemoveWarnAt` returns `gorm.ErrRecordNotFound` when the chat has no row in `chats`

**Files modified:** `fuku/db/warns/repository.go`
**Commit:** `3266d03f`
**Applied fix:** Treated `gorm.ErrRecordNotFound` returned by `lockWarnChat(tx, chatId)` inside `RemoveWarnAt` as a clean no-op (`return nil`, yielding `(false, nil)`), matching `ResetUserWarnsAt` so `/rmwarn` on an unseeded chat replies with `warns_no_warns_to_remove` instead of `error_generic`.

## Skipped Issues

### WR-03: Unescaped user input in HTML-mode error replies in `/setwarnmode` and `/setfloodmode`

**File:** `fuku/modules/warns.go:216-217` (and `fuku/modules/antiflood.go:761-762`)
**Reason:** Partially already fixed on HEAD and remaining site pinned by frozen legacy golden test:
1. `fuku/modules/antiflood.go:763` was already escaped with `html.EscapeString(args[0])` in Phase 23 commit `023d20b4`.
2. `fuku/modules/warns.go:217` is intentionally left unescaped because frozen MSG-11 golden test `fuku/modules/warns_legacy_request_test.go:159` (`mode_unknown_unescaped` / `goldenWarnModeAmpersand`) explicitly pins the raw `a&b<c` output and must not be modified (`TestLegacyWarnSettingsRequests` verified passing).
**Original issue:** On an unknown mode, `setWarnMode` (`warns.go:217`) and `setFloodMode` (`antiflood.go:762`) interpolate raw `args[0]` into a reply sent with `formatting.Shtml()` without `html.EscapeString`.

---

_Fixed: 2026-11-24T14:55:00Z_
_Fixer: the agent (gsd-code-fixer)_
_Iteration: 1_
