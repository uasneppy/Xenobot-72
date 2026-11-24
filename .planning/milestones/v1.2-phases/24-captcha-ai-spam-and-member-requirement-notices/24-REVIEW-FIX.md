---
phase: 24-captcha-ai-spam-and-member-requirement-notices
fixed_at: 2026-11-24T14:22:28Z
review_path: .planning/phases/24-captcha-ai-spam-and-member-requirement-notices/24-REVIEW.md
fix_scope: critical_warning
iteration: 1
findings_in_scope: 4
fixed: 3
skipped: 1
status: partial
---

# Phase 24: Code Review Fix Report

**Fixed at:** 2026-11-24T14:22:28Z
**Source review:** `.planning/phases/24-captcha-ai-spam-and-member-requirement-notices/24-REVIEW.md`
**Fix scope:** `critical_warning`
**Iteration:** 1

**Summary:**
- Findings in scope: 4 (`WR-01`, `WR-02`, `WR-03`, `WR-04`; `IN-01` and `IN-02` are out of `critical_warning` scope)
- Fixed in this pass: 3 (`WR-01`, `WR-02`, `WR-04`)
- Skipped (already resolved on HEAD): 1 (`WR-03`, previously fixed by Phase 22 commit `f9f52f43`)

## Fixed Issues

### WR-01: Paced notice send closures discard bounded `callCtx`, defeating per-call timeout and shutdown cancellation

**Files modified:** `fuku/utils/helpers/telegram_helpers.go`, `fuku/utils/helpers/telegram_helpers_test.go`, `fuku/modules/aispam_notice.go`, `fuku/modules/memberreq_notice.go`
**Commit:** `fc452fed`
**Applied fix:** Added `helpers.SendMessageWithErrorHandlingContext` (delegating `SendMessageWithErrorHandling` to it with `context.Background()`) and propagated `callCtx` into `helpers.SendMessageWithErrorHandlingContext` and `media.SendGreetingMessageWithContext` in `aispam_notice.go` (`SendDefault` and `SendCustom`) and into `media.SendGreetingMessageWithContext` in `memberreq_notice.go` (`SendCustom`). Added `TestSendMessageWithErrorHandlingContextPropagatesContext`.

### WR-02: `DeleteMessageWithErrorHandling` swallows `"message can't be deleted"`, causing `postAISpamNotice` to announce a deletion that did not occur

**Files modified:** `fuku/modules/aispam.go`, `fuku/modules/aispam_notice_test.go`
**Commit:** `7bd43299`
**Applied fix:** Replaced `helpers.DeleteMessageWithErrorHandling` in `processAISpam` with a direct `task.bot.DeleteMessage(task.chatID, task.messageID, nil)` call, logging warnings for delete errors other than `"message to delete not found"`, and gating `postAISpamNotice(task)` on `deleted && deleteErr == nil`. Added in-process regression test `TestAISpamNoticeSkippedWhenMessageCannotBeDeleted`.

### WR-04: Warn-limit ban in `memberReqPostEventNotice` omits the group's Appeal button

**Files modified:** `fuku/modules/memberreq_notice.go`, `fuku/modules/event_message_mreq_test.go`
**Commit:** `ffa60b52`
**Applied fix:** Set `reply.Appeal = true` in `memberReqPostEventNotice` when a member-requirement warn reaches the warn limit and applies a `ban` or `tban` (`memberReqLimitMode(warn.Mode) == "ban"`), so both the group notice and private copy attach the group's configured Appeal button. Added in-process regression test `TestMemberReqWarnLimitBanAppeal` covering `ban`, `tban`, and `kick`.

## Skipped Issues

### WR-03: `NoMedia` rule disagrees between `eventmsg.NoMedia` / `composeEventPM` and the Messages PM editor UI

**File:** `fuku/modules/settings_messages.go:472-486`
**Reason:** Already resolved on HEAD by Phase 22 commit `f9f52f43` (`fix(22): WR-02 include media part in settingsMsgPMEditDef to match eventmsg.NoMedia and composeEventPM`), which already sets `def.Parts = []settingsEditPart{settingsMsgTextPart(fam), settingsEditMediaPart, settingsEditButtonsPart}` in `settingsMsgPMEditDef`. No further edit required.
**Original issue:** `eventmsg.NoMedia` allows media on member-requirement PM copies (`DestPM`) and `composeEventPM` sends it, while `settingsMsgPMEditDef` previously omitted `settingsEditMediaPart`.

## Verification

**Environment:** Main checkout (`workflow.use_worktrees: false`), `TZ=UTC`, sandboxed (no localhost TCP binds; new Redis-backed tests use `phase28InProcessMiniredis(t)`).

- `TZ=UTC go vet -tags testtools ./...` — passed
- `CGO_ENABLED=0 go build ./...` — passed
- `make check-translations` — passed (1068 keys, all 8 locales complete)
- `make check-docs` — passed (no drift)
- `make test-shard-check` — passed
- `TZ=UTC go test -tags testtools -race -count=1 -timeout 60s -run '^TestSendMessageWithErrorHandling' ./fuku/utils/helpers` — passed
- `TZ=UTC go test -tags testtools -race -count=1 -timeout 60s -run '^TestSendGreetingMessageWithContextPropagatesContext$' ./fuku/utils/media` — passed
- `TZ=UTC go test -tags testtools -race -count=1 -timeout 60s -run '^(TestEventEngineCallers|TestPMCopyCallers|TestEventMsgValidateWriteMatchesSettingsPage|TestLegacyPhase29Baseline|TestLegacyPhase30Baseline|TestAISpamNoticeSkippedWhenMessageCannotBeDeleted|TestMemberReqWarnLimitBanAppeal)$' ./fuku/modules` — passed

---

_Fixed: 2026-11-24T14:22:28Z_
_Fixer: the agent (gsd-code-fixer)_
_Iteration: 1_
