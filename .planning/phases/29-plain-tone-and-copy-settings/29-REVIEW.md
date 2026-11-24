---
phase: 29-plain-tone-and-copy-settings
reviewed: 2026-11-24T09:11:00Z
depth: standard
files_reviewed: 68
files_reviewed_list:
  - AGENTS.md
  - Makefile
  - fuku/db/advthresholds/repository.go
  - fuku/db/advthresholds/repository_test.go
  - fuku/db/antiflood/repository.go
  - fuku/db/antiflood/repository_test.go
  - fuku/db/backup/event_messages_backup_test.go
  - fuku/db/escalation/repository.go
  - fuku/db/escalation/repository_test.go
  - fuku/db/eventmsg/events.go
  - fuku/db/eventmsg/repository.go
  - fuku/db/eventmsg/repository_test.go
  - fuku/db/eventmsg/validate_write.go
  - fuku/db/eventmsg/validate_write_test.go
  - fuku/db/migrations/event_message_plain_tone_migration_test.go
  - fuku/db/models/event_message.go
  - fuku/db/modtuning/repository.go
  - fuku/db/modtuning/repository_test.go
  - fuku/i18n/eventmsg_locale_test.go
  - fuku/i18n/settings_locale_test.go
  - fuku/modules/aispam_notice.go
  - fuku/modules/antiflood.go
  - fuku/modules/bans.go
  - fuku/modules/copy_settings.go
  - fuku/modules/copy_settings_test.go
  - fuku/modules/event_message.go
  - fuku/modules/event_message_apply.go
  - fuku/modules/event_message_auto.go
  - fuku/modules/event_message_lockdown.go
  - fuku/modules/event_message_plain.go
  - fuku/modules/event_message_plain_test.go
  - fuku/modules/event_message_pm.go
  - fuku/modules/event_message_pm_details_test.go
  - fuku/modules/event_message_pm_failures_test.go
  - fuku/modules/event_message_pm_log_test.go
  - fuku/modules/event_message_pm_safety_test.go
  - fuku/modules/event_message_pm_test.go
  - fuku/modules/memberreq_notice.go
  - fuku/modules/modtuning_appeal_test.go
  - fuku/modules/mute.go
  - fuku/modules/phase29_e2e_test.go
  - fuku/modules/phase29_legacy_request_test.go
  - fuku/modules/settings_copy.go
  - fuku/modules/settings_copy_test.go
  - fuku/modules/settings_edit.go
  - fuku/modules/settings_main_test.go
  - fuku/modules/settings_messages.go
  - fuku/modules/settings_messages_auto_test.go
  - fuku/modules/settings_messages_lockdown_test.go
  - fuku/modules/settings_messages_pm_options_test.go
  - fuku/modules/settings_messages_pm_preview_test.go
  - fuku/modules/settings_messages_safety_test.go
  - fuku/modules/settings_messages_screening_test.go
  - fuku/modules/settings_messages_screens_test.go
  - fuku/modules/settings_messages_test.go
  - fuku/modules/settings_phase19_test.go
  - fuku/modules/settings_phase29_test.go
  - fuku/modules/settings_staff_privacy_test.go
  - fuku/modules/warns.go
  - locales/en.yml
  - locales/es.yml
  - locales/fr.yml
  - locales/hi.yml
  - locales/id.yml
  - locales/pt.yml
  - locales/ru.yml
  - locales/uk.yml
  - migrations/20261123130000_add_event_message_plain_tone.sql
findings:
  critical: 0
  warning: 3
  info: 2
  total: 5
  necessity: 0
status: issues_found
---

# Phase 29: Code Review Report

**Reviewed:** 2026-11-24T09:11:00Z
**Depth:** standard
**Files Reviewed:** 68
**Status:** issues_found

## Summary

Reviewed the Phase 29 Plain tone (`TUNE-08`) and Copy Settings (`TUNE-09`) implementation against current HEAD: the idempotent `plain_tone` migration, `eventmsg` repository and shared `ReplaceAllValidated` / `ReplaceAllValidatedTx` writer, tuning repository replace helpers (`antiflood`, `escalation`, `modtuning`, `advthresholds`), pure plain-tone builders and all 23 group / 18 PM send sites, Messages hub toggle (`ht`), and private Copy Settings page (`cp`, `cg`/`ck`/`cy`) with live dual-group verification and atomic Lua Redis token consumption.

Security fundamentals are solid: Copy Settings enforces step-1 private-only canonical field validation, live dual-group checks at both `ck` and `cy`, uniform opaque `settings_pick_unavailable` target refusals, P29-02 non-creator dual-right checks, peek-before-consume binding so cross-user/cross-source callbacks cannot burn a valid card, HTML escaping of titles/reasons/mentions, and IDs-only log hygiene. Plain tone preserves tone-off defaults byte-for-byte, respects custom text precedence, and enforces non-empty `PlainKey` on all 18 PM specs at init.

No Critical (Blocker) defects were found. Three Warnings (escalation ladder gap round-trip bug in `ReplaceLadder`, non-atomic multi-table copy writes in `applyCopySettings`, and unbounded context-less per-group Staff Group queries in `settingsCopyLoadCandidates`) and two Info items should be addressed.

## Narrative Findings (AI reviewer)

## Warnings

### WR-01: `escalation.ReplaceLadder` accepts interior empty `Step{}` gaps that are silently dropped on read

**File:** `fuku/db/escalation/repository.go:241-272`
**Issue:** `ReplaceLadder` skips zero `Step{}` values during validation (`if s == (Step{}) { continue }`) and then writes `ladder.Steps[n-1]` verbatim into `step1_action`..`step5_action`. PostgreSQL's `chk_chat_escalation_step*` constraints permit `''` in any slot, so an input such as `Ladder{Enabled: true, Steps: []Step{{}, {Action: ActionBan}}}` is accepted and persisted with `step1_action = ""` and `step2_action = "ban"`. When read back, `ladderOf` (`fuku/db/escalation/repository.go:133-138`) stops at the first empty `Action` (`if s.Action == "" { break }`), silently truncating `step2` and returning an empty `Steps` slice. By contrast, `SetEnabledWithSteps` (`repository.go:186-190`) rejects empty steps via `!ValidStep(s)`.
**Fix:** Reject interior empty steps (any non-empty step after an empty step) before touching the database, returning `ErrInvalidValue`:
```go
	hasSteps := false
	seenEmpty := false
	for _, s := range ladder.Steps {
		if s == (Step{}) {
			seenEmpty = true
			continue
		}
		if seenEmpty || !ValidStep(s) {
			return ErrInvalidValue
		}
		hasSteps = true
	}
```

### WR-02: `applyCopySettings` performs 9 separate DB writes without an enclosing transaction, risking partial target state on mid-way failure

**File:** `fuku/modules/copy_settings.go:179-247`
**Issue:** `applyCopySettings` executes 9 independent repository writes sequentially (`eventmsg.ReplaceAllValidated`, `warns.SetWarnModeContext`, `warns.SetWarnExpiryContext`, `antiflood.SetFloodActionPreserveOffContext`, `blacklists.SetBlacklistActionTimedContext`, `escalation.ReplaceLadder`, `modtuning.ReplaceDefaults`, `modtuning.ReplaceShortcuts`, and `advthresholds.ReplaceAll`). If any write after step 1 fails (e.g., `settingsCopyWriteTimeout` expires, a transient DB disconnect occurs, or a later repository rejects a malformed source row), all earlier writes are already committed with their caches invalidated while later tables retain the target's prior configuration. Because `settingsCopyRunYes` (`fuku/modules/settings_copy.go:458-479`) consumes the one-time token before calling `applyCopySettings` and only shows `common_settings_save_failed`, the target group is left in a silently half-copied state.
**Fix:** Add `*Tx` variants (matching `eventmsg.ReplaceAllValidatedTx`) for the remaining tuning writes so `applyCopySettings` runs all 9 table updates inside one `db.DB.WithContext(ctx).Transaction(...)` and drops all target cache keys only after the transaction commits.

### WR-03: `settingsCopyLoadCandidates` runs unbounded per-group Staff Group DB queries outside `readCtx`

**File:** `fuku/modules/settings_copy.go:262-280`
**Issue:** `settingsCopyLoadCandidates` derives `readCtx` with `settingsReadTimeout` for `settingsCopyHubLookup`, then calls `settingsCopyStaffLookup(g.ChatID)` inside a loop over every group returned by `adminindex` (line 274). `settingsCopyDefaultStaffLookup` calls `staff.GetStaffGroupFresh(chatID)` (`fuku/db/staff/repository.go:209`), which takes no `context.Context` and issues one SQL query per group. For a user managing many groups or during DB latency spikes, the loop is not bounded by `readCtx` and can block the callback handler well beyond `settingsReadTimeout`.
**Fix:** Add a context-aware batch lookup in `fuku/db/staff` (e.g., `StaffGroupChatIDsFresh(ctx, chatIDs)`) that fetches matching Staff Group IDs in a single `WHERE chat_id IN ?` query bounded by `readCtx`, and use that set to filter `groups` in `settingsCopyLoadCandidates`.

## Info

### IN-01: Misleading log message in `eventMsgOptionsReadBack` for `applyEventPlainTone`

**File:** `fuku/modules/event_message_apply.go:139-151,179-182`
**Issue:** `applyEventPlainTone` delegates to `eventMsgOptionsReadBack(ctx, chatID, "plain tone", writeErr)`, whose log templates hardcode `"private copy %s option"` and `"private copy options"`. A write or read-back failure logs `"storing the private copy plain tone option failed"`, even though Plain tone applies to both group replies and private copies.
**Fix:** Change the format strings in `eventMsgOptionsReadBack` to `"event message %s option"` and `"event message options"`.

### IN-02: `eventPlainRender` mutates caller-provided `params` map in place

**File:** `fuku/modules/event_message_plain.go:22-28`
**Issue:** When `mentionKey != ""` and `params != nil`, `eventPlainRender` assigns `params[mentionKey] = lockdownNameToken` directly into the caller's map. All current call sites pass fresh literals, so there is no current defect, but reusing a `TranslationParams` map across calls would silently overwrite keys.
**Fix:** Clone `params` (`maps.Clone(params)`) when non-nil before inserting `mentionKey`.

---

_Reviewed: 2026-11-24T09:11:00Z_
_Reviewer: the agent (gsd-code-reviewer)_
_Depth: standard_
