---
phase: 25-lockdown-messages
reviewed: 2026-11-24T09:54:55Z
depth: standard
files_reviewed: 8
files_reviewed_list:
  - fuku/db/eventmsg/events.go
  - fuku/modules/event_message.go
  - fuku/modules/event_message_lockdown.go
  - fuku/modules/lockdown.go
  - fuku/modules/lockdown_alert_callbacks.go
  - fuku/modules/raid_trigger.go
  - fuku/modules/settings_lockdown.go
  - fuku/modules/settings_messages.go
findings:
  critical: 0
  warning: 2
  info: 2
  total: 4
  necessity: 1
status: issues_found
---

# Phase 25: Code Review Report

**Reviewed:** 2026-11-24T09:54:55Z
**Depth:** standard
**Files Reviewed:** 8
**Status:** issues_found

## Summary

Reviewed the eight Phase 25 (Lockdown Messages) source files on `HEAD` at `standard` depth, covering event definitions and reserves (`fuku/db/eventmsg/events.go`), the event message engine (`fuku/modules/event_message.go`), lockdown event adapters and technical notes (`fuku/modules/event_message_lockdown.go`), manual and automatic lockdown start/lift paths (`fuku/modules/lockdown.go`, `fuku/modules/raid_trigger.go`, `fuku/modules/lockdown_alert_callbacks.go`, `fuku/modules/settings_lockdown.go`), and the Messages settings UI (`fuku/modules/settings_messages.go`).

Core Phase 25 invariants hold up well:
- **EVT-10 boundaries:** Only the five public lockdown notices (`lockdown`, `lockjoin`, `lockflood`, `lockai`, `locklift`) enter the event engine, and only after the lockdown or lift has taken effect. Alerts, reminders, raid-scale removals, lift tallies, cannot-lock warnings, refusals, and Staff Group surfaces remain outside the engine, and none of the five events supports private copies (`eventmsg.HasPM` is `false` and no PM spec exists).
- **Non-blocking & shutdown drain:** `raidPostTracker` and `StopRaidPosts` in `fuku/modules/raid_trigger.go` are race-free, recover panics before decrementing, and bound shutdown waits cleanly. Both `lockdownLiftConfirm` and `settingsLockdownLiftYes` update the confirmation card before waiting on the pacer to post the lift notice.
- **No deletion scheduling:** `lockdownEventSite` installs a no-op `Schedule` hook so lockdown notices are never queued for auto-deletion.

No critical defects or security vulnerabilities were found. Two warnings (one behavioral edge case with sticker media dropping technical notes, and one necessity finding for redundant state guards in `lockdown_alert_callbacks.go`) and two minor info items are reported below.

## Narrative Findings (AI reviewer)

## Warnings

### WR-01: Sticker media on lockdown events silently drops custom text and mandatory technical notes

**Classification:** WARNING
**File:** `fuku/modules/settings_messages.go:304-308` (also `fuku/db/eventmsg/events.go:178-188`, `fuku/modules/event_message_lockdown.go:257, 329, 358`)
**Issue:** Unlike the Member requirements family (`NoMedia: true`), the Lockdown family allows media attachments (`NoMedia: false`), and both the Messages editor (`fuku/modules/settings_edit_check.go:280`) and `eventmsg.ValidateMessage` (`fuku/db/eventmsg/validate_write.go:55`) accept `db.STICKER`. At send time, `composeEventMessage` (`fuku/modules/event_message.go:349-359`) appends `r.Note` (`lockdownStartNotes` — approved-users-muted and missing delete/invite rights — or `lockdownLiftNotes` — manual permission changes and joiners being unbanned) to `msg.Text`, then passes `msg` to `media.SendGreetingMessage`. For `db.STICKER`, `media.Send` calls `sendSticker` (`fuku/utils/media/sender.go:89-90, 127-141`), which invokes `b.SendSticker` and ignores `content.Text` entirely. As a result, attaching a sticker to any lockdown event silently drops both the custom text and all mandatory technical notes (including missing-rights warnings and unbanning notices), undermining the `lockdownNoteReserve` / `r.Note` guarantee.
**Fix:** Either reject captionless media (`db.STICKER`) for events with mandatory appended notes (`ReservedText > 0`) in `settings_edit_check.go` and `eventmsg.ValidateMessage`, or fall back to sending the text/notes when `row.MediaType == db.STICKER` on lockdown events.

### WR-02: Impossible-state guard — redundant `ld.State != models.LockdownStateActive` checks in alert callback handlers

**Classification:** WARNING
**File:** `fuku/modules/lockdown_alert_callbacks.go:262, 329, 398, 481, 613, 696`
**Category:** necessity
**Issue:** `lockdownAlertCallback` (`fuku/modules/lockdown_alert_callbacks.go:93-95`) already checks `if ld.State != models.LockdownStateActive` and returns `m.lockdownStalePress(b, query, tr, slot, ld)` before dispatching to the action switch (`lines 97-112`). All six handlers (`lockdownLiftAsk`, `lockdownLiftConfirm`, `lockdownBanAsk`, `lockdownBanConfirm`, `lockdownRevokeAsk`, `lockdownRevokeConfirm`) are unexported and called solely from that switch, and none of them re-reads or mutates `ld` before repeating `if ld.State != models.LockdownStateActive` at lines 262, 329, 398, 481, 613, and 696. The guarded state cannot occur at any of those six branches (concurrent lifts after the initial `GetFresh` are instead caught by `liftLockdownRow`'s conditional `BeginLift` returning `lockdownLiftLost` at line 347).
**Fix:** Remove the six redundant `if ld.State != models.LockdownStateActive` checks and document on the handlers that `lockdownAlertCallback` guarantees `ld.State == models.LockdownStateActive` prior to dispatch.

## Info

### IN-01: Duplicate `lockdownNoteReserve` literal instead of referencing `eventmsg.LockdownTextReserve`

**Classification:** INFO
**File:** `fuku/modules/event_message_lockdown.go:48`
**Issue:** `const lockdownNoteReserve = 400` duplicates `eventmsg.LockdownTextReserve = 400` (`fuku/db/eventmsg/validate_write.go:31`) even though `fuku/modules/event_message_lockdown.go` already imports `fuku/db/eventmsg`. Keeping two independent literals requires a separate drift test (`copy_settings_test.go:83`).
**Fix:** Define `const lockdownNoteReserve = eventmsg.LockdownTextReserve` (or use `eventmsg.LockdownTextReserve` directly in `fuku/modules/settings_messages.go:307`).

### IN-02: `lockdownStartDefaultReply` uses `in.Target.FirstName` directly instead of full name like `lockdownLiftDefaultReply`

**Classification:** INFO
**File:** `fuku/modules/event_message_lockdown.go:172`
**Issue:** `lockdownLiftDefaultReply` (`fuku/modules/event_message_lockdown.go:151`) normalizes the actor name via `lockdownCapRunes(staffFullName(&in.Target), lockdownNameMaxRunes)`, so both live calls (where `FirstName` already holds the capped full name) and Messages previews (where `in.Target` is `p.query.From` with separate `FirstName` and `LastName`) render consistently. `lockdownStartDefaultReply` (line 172) passes `in.Target.FirstName` directly, dropping `LastName` during a manual lockdown default preview.
**Fix:** Use `lockdownCapRunes(staffFullName(&in.Target), lockdownNameMaxRunes)` in `lockdownStartDefaultReply` to match `lockdownLiftDefaultReply`.

---

_Reviewed: 2026-11-24T09:54:55Z_
_Reviewer: the agent (gsd-code-reviewer)_
_Depth: standard_
