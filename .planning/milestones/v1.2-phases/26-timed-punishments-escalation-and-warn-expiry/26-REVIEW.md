---
phase: 26-timed-punishments-escalation-and-warn-expiry
reviewed: 2026-11-24T09:54:41Z
depth: standard
files_reviewed: 25
files_reviewed_list:
  - migrations/20261122120000_add_timed_punishment_durations.sql
  - migrations/20261122130000_add_warn_expiry.sql
  - migrations/20261122140000_add_escalation_ladder.sql
  - fuku/db/models/punish.go
  - fuku/db/models/antiflood.go
  - fuku/db/models/blacklists.go
  - fuku/db/models/warns.go
  - fuku/db/models/escalation.go
  - fuku/db/antiflood/optimized.go
  - fuku/db/antiflood/repository.go
  - fuku/db/blacklists/repository.go
  - fuku/db/warns/repository.go
  - fuku/db/escalation/repository.go
  - fuku/modules/punish_timed.go
  - fuku/modules/escalation.go
  - fuku/modules/escalation_settings.go
  - fuku/modules/settings_escalation.go
  - fuku/modules/settings_duration.go
  - fuku/modules/settings_warns.go
  - fuku/modules/settings_antiflood.go
  - fuku/modules/settings_blacklist.go
  - fuku/modules/warns.go
  - fuku/modules/antiflood.go
  - fuku/modules/blacklists.go
  - fuku/modules/staff_forwardblock_watch.go
findings:
  critical: 0
  warning: 6
  info: 2
  total: 8
  necessity: 0
status: issues_found
---

# Phase 26: Code Review Report

**Reviewed:** 2026-11-24T09:54:41Z
**Depth:** standard
**Files Reviewed:** 25
**Status:** issues_found

## Summary

Reviewed all 25 Phase 26 files covering timed punishment duration storage and enforcement (`punish_timed.go`, `warns.go`, `antiflood.go`, `blacklists.go`, `settings_duration.go`, `settings_warns.go`, `settings_antiflood.go`, `settings_blacklist.go`), warn expiry storage, counting, and backup parity (`models/warns.go`, `db/warns/repository.go`, `migrations/20261122130000_add_warn_expiry.sql`), and the escalation ladder (`models/escalation.go`, `db/escalation/repository.go`, `escalation.go`, `escalation_settings.go`, `settings_escalation.go`, `staff_forwardblock_watch.go`).

Overall the schema migrations, GORM models, warn-expiry counting (`NormalisedTimes` / `Counted`), atomic offence counting (`CountOffence`), cache invalidation, and private-only escalation UI validation are well-structured and defensive. No Critical/Blocker defects were found. Six Warnings and two Info items were identified, centered on a TOCTOU race in concurrent escalation ladder edits, a missing `CanBotRestrict` check before `escalate` in `blacklistWatcher`, unescaped HTML arguments in `/setwarnmode` and `/setfloodmode`, nil `db.DB` dereferences in `*Context` repository wrappers, zero upper-bound clock-skew margin at `PunishMaxSecs` (366 days), and `RemoveWarnAt` returning `gorm.ErrRecordNotFound` when a chat is absent from `chats`. No necessity findings were identified.

## Narrative Findings (AI reviewer)

## Warnings

### WR-01: TOCTOU race in escalation ladder edits can create step gaps or an enabled zero-step ladder

**File:** `fuku/modules/escalation_settings.go:109-171`
**Issue:** `applyEscalationEnabled`, `applyEscalationStep`, and `applyEscalationRemoveStep` each perform a non-transactional read-check-write (`escalationBefore` → invariant check → `escalationWrites.SetStep` / `SetEnabled`). Two admins editing the same group's Escalation page concurrently can violate both ladder invariants:
1. **Step gap / phantom step resurrection:** Admin A removes the last step `N` (`applyEscalationRemoveStep` sees `len(before.Steps) == N`, writes `stepN = Step{}`) while Admin B adds step `N+1` (`applyEscalationStep` sees `len(before.Steps) == N`, writes `step(N+1)`). Because `ladderOf` (`fuku/db/escalation/repository.go:131-140`) stops at the first empty step, `step(N+1)` is silently hidden until an admin later adds step `N`, at which point the orphaned `step(N+1)` resurrects.
2. **Enabled ladder with zero steps:** When 1 step exists and `enabled == false`, Admin A turns the ladder on (`SetEnabled(..., true)`) while Admin B removes step 1 (`before.Enabled == false`, clears `step1`), leaving `enabled = true` with zero steps despite `escalationLastStepRefused` and `escalationExampleSteps`.
**Fix:** Perform the read, invariant check, and write inside a single database transaction with a `SELECT ... FOR UPDATE` lock on `chat_escalation` (e.g., repository methods `SetEnabledChecked`, `SetStepChecked`, `RemoveLastStepChecked` that drop `CacheKey(chatID)` after commit).

### WR-02: `blacklistWatcher` increments `chat_offences` via `escalate` without checking `CanBotRestrict`

**File:** `fuku/modules/blacklists.go:807-818`
**Issue:** `checkFlood` (`fuku/modules/antiflood.go:290-297`) and `forwardBlockEnforce` (`fuku/modules/staff_forwardblock_watch.go:316-320`) both verify `chat_status.CanBotRestrict` before calling `escalate`. `blacklistWatcher` only checks `chat_status.IsBotAdmin` (line 782) and calls `escalate` unconditionally at line 813 before `blacklistPunish`. When the bot is an administrator without `CanRestrictMembers` in a group with escalation enabled and an effective blacklist action (`mute`, `kick`, `ban`, `tmute`, `tban`), every matched message increments the member's offence count in `chat_offences` even though `blacklistPunish` will fail with a Telegram permission error.
**Fix:** Gate `escalate` and `blacklistPunish` on `chat_status.CanBotRestrict(b, ctx, chat)`:
```go
	if !chat_status.CanBotRestrict(b, ctx, chat) {
		return ext.ContinueGroups
	}
	p = escalate(tracing.UpdateContext(ctx), b, chat.Id, user.Id(), "blacklist", p)
```

### WR-03: Unescaped user input in HTML-mode error replies in `/setwarnmode` and `/setfloodmode`

**File:** `fuku/modules/warns.go:216-217` (and `fuku/modules/antiflood.go:761-762`)
**Issue:** On an unknown mode, `setWarnMode` (`warns.go:217`: `replyText = fmt.Sprintf(temp, args[0])`) and `setFloodMode` (`antiflood.go:762`: `fmt.Sprintf(temp, args[0])`) interpolate raw `args[0]` into a reply sent with `formatting.Shtml()` (HTML parse mode) without `html.EscapeString`. The settings-menu counterparts (`settings_warns.go:339`, `settings_antiflood.go:317`) and `/setwarnlimit` (`warns.go:868`) all escape user input. An argument containing `<`, `>`, or `&` (e.g., `/setwarnmode <mute>`) causes Telegram to reject the reply with HTTP 400 (`can't parse entities`).
**Fix:** Escape `args[0]` with `html.EscapeString(args[0])` in both `warns.go:217` and `antiflood.go:762`.

### WR-04: Nil `db.DB` dereference in `*Context` repository wrappers despite nil-safe `*Tx` helpers

**File:** `fuku/db/blacklists/repository.go:116-117` (and `fuku/db/warns/repository.go:412-416`, `fuku/db/warns/repository.go:465-466`)
**Issue:** `SetBlacklistActionTimedTx`, `SetWarnExpiryTx`, and `SetWarnModeTx` each check `if tx == nil` and return `errors.New("database not initialized")`. However, their `*Context` wrappers (`SetBlacklistActionTimedContext`, `SetWarnExpiryContext`, `SetWarnModeContext`) unconditionally call `db.DB.WithContext(ctx)` without checking `db.DB == nil`, causing a nil-pointer panic if invoked when `db.DB` is nil. By contrast, `antiflood.SetFloodActionPreserveOffContext` (`fuku/db/antiflood/repository.go:145`) and `escalation.ReplaceLadder` (`fuku/db/escalation/repository.go:281`) guard `db.DB == nil` before calling `WithContext`.
**Fix:** Guard `db.DB == nil` before calling `db.DB.WithContext(ctx)` in all three wrappers:
```go
	if db.DB == nil {
		return 0, errors.New("database not initialized")
	}
```

### WR-05: `PunishMaxSecs` (366 days) has no upper-bound clock-skew margin against Telegram's permanent-restriction rule

**File:** `fuku/modules/punish_timed.go:28,175-187` (and `fuku/db/models/punish.go:13`)
**Issue:** Telegram treats `until_date` either `< 30s` or `> 366 days` from Telegram's current server time as a **permanent** restriction. `applyTimedPunishment` enforces a 5-second safety lead on the lower bound (`punishMinLead = 35 * time.Second`) to absorb clock skew and request flight time, but `PunishMaxSecs` is set to exactly `366 * 24 * 60 * 60` (`31622400`) with zero upper-bound safety margin. If an admin configures `tmute 366d` or `tban 366d` and the bot's clock is even 1 second ahead of Telegram's server clock, `now.Unix() + 31622400` exceeds 366 days on Telegram's side and Telegram silently converts the timed punishment into a permanent mute/ban, contradicting the invariant in `punish_timed.go:27` ("A longer one is refused, never made permanent").
**Fix:** Subtract a small safety margin (e.g., 60 seconds) when `until - now.Unix()` is within the margin of `366 * 86400`, or cap `PunishMaxSecs` below 366 days (e.g., `365 * 24 * 60 * 60` or `366*86400 - 60`).

### WR-06: `RemoveWarnAt` returns `gorm.ErrRecordNotFound` when the chat has no row in `chats`

**File:** `fuku/db/warns/repository.go:249-256`
**Issue:** `RemoveWarnAt` starts its transaction with `lockWarnChat(tx, chatId)` (`SELECT id FROM chats WHERE chat_id = ? FOR UPDATE`). Unlike `WarnUserAt` (which calls `chats.EnsureChatInDb` first) and `ResetUserWarnsAt` (which returns `(false, nil)` when absent), `RemoveWarnAt` does not handle `gorm.ErrRecordNotFound` from `lockWarnChat`. If `/rmwarn` runs for a chat not yet present in `chats`, `RemoveWarnAt` returns `(false, gorm.ErrRecordNotFound)` and the handler replies with `error_generic` (`warns.go:1129-1130`) instead of `warns_no_warns_to_remove`.
**Fix:** Treat `gorm.ErrRecordNotFound` from `lockWarnChat` in `RemoveWarnAt` as a clean no-op:
```go
		if err := lockWarnChat(tx, chatId); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
```

## Info

### IN-01: `CountOffence` accepts `forget < SameOffenceWindow` where the 60s window overrides `forget`

**File:** `fuku/db/escalation/repository.go:331-357`
**Issue:** `CountOffence` validates `forget < time.Second` (line 348), but in `countOffenceSQL` the `WHEN ... < window` (60s) branch precedes `WHEN ... >= forgetSecs`. Any direct caller passing `1s <= forget < 60s` will have offences between `forget` and 60s deduplicated as the same offence instead of resetting to 1. Production traffic always passes `ForgetDays * 24h` (`>= 86400s`), so this only affects direct callers/tests.
**Fix:** Validate `forget < SameOffenceWindow` (or document the requirement), or check `>= forgetSecs` before `< window` when `forgetSecs <= window`.

### IN-02: Hand-rolled `slicesContainsString` duplicates stdlib `slices.Contains`

**File:** `fuku/modules/settings_escalation.go:108-115`
**Issue:** `slicesContainsString` duplicates `slices.Contains` from the Go standard library, which is already used across `fuku/modules/` (`warns.go`, `antiflood.go`, `blacklists.go`, `settings_warns.go`, `settings_antiflood.go`, `settings_blacklist.go`).
**Fix:** Import `"slices"`, replace `slicesContainsString(settingsEscUntimed, f["v"])` with `slices.Contains(settingsEscUntimed, f["v"])`, and remove `slicesContainsString`.

---

_Reviewed: 2026-11-24T09:54:41Z_
_Reviewer: the agent (gsd-code-reviewer)_
_Depth: standard_
