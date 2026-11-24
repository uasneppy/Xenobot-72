---
phase: 24-captcha-ai-spam-and-member-requirement-notices
reviewed: 2026-11-24T09:46:39Z
depth: standard
files_reviewed: 14
files_reviewed_list:
  - fuku/db/eventmsg/events.go
  - fuku/db/eventmsg/repository.go
  - fuku/modules/aispam.go
  - fuku/modules/aispam_notice.go
  - fuku/modules/captcha.go
  - fuku/modules/event_message.go
  - fuku/modules/event_message_auto.go
  - fuku/modules/event_message_pm.go
  - fuku/modules/memberreq_enforce.go
  - fuku/modules/memberreq_notice.go
  - fuku/modules/settings_edit.go
  - fuku/modules/settings_edit_actions.go
  - fuku/modules/settings_edit_check.go
  - fuku/modules/settings_messages.go
findings:
  critical: 0
  warning: 4
  info: 2
  total: 6
  necessity: 0
status: issues_found
---

# Phase 24: Code Review Report

**Reviewed:** 2026-11-24T09:46:39Z
**Depth:** standard
**Files Reviewed:** 14
**Status:** issues_found

## Summary

Reviewed all 14 source files in scope for Phase 24 (Captcha, AI Spam, and Member Requirement Notices) against HEAD at `standard` depth.

Overall, the core architecture is solid:
- **AI spam privacy & rate limiting:** `aispam_notice.go` and `event_message_auto.go` never expose AI model names, provider names, probabilities, or spam categories to group notices or private copies; the atomic Lua script in `aispam_notice.go` uses `{<chat>}` hash tags for Redis Cluster compatibility and fails closed when Redis is unavailable or errors.
- **Off-by-default & non-silenceable invariants:** `EventAISpam` stays off by default even when custom text is saved (`DefaultSilentFor`), and `EventReqMute` cannot be silenced in either the settings UI (`NoSilent`) or the send path (`Required: true`), preserving the "I fixed it" recovery button.
- **Captcha failure notices:** `handleCaptchaTimeout` routes all timeout and failed-solve paths through `postCaptchaFailureNotice`, keeping the 30-second in-process self-deletion schedule and avoiding the deletion queue.

Four warnings and two info items were found:
1. Context timeouts and shutdown cancellation are silently discarded in `aispam_notice.go` (`SendDefault` and `SendCustom`) and `memberreq_notice.go` (`SendCustom`).
2. `processAISpam` relies on `helpers.DeleteMessageWithErrorHandling` returning `nil` to gate `postAISpamNotice`, which swallows `"message can't be deleted"` and posts a false deletion notice when the bot lacks delete rights.
3. `eventmsg.NoMedia` allows media on member-requirement PM copies (`DestPM`) and `composeEventPM` sends it, but `settingsMsgPMEditDef` hides the Media part in the Messages UI, leaving imported/copied media invisible and uneditable while still shrinking the text limit to 1024.
4. `memberReqPostEventNotice` does not set `reply.Appeal` when a member-requirement warn reaches the warn limit and applies a `ban`/`tban`, omitting the group's Appeal button.

No Critical (BLOCKER) security vulnerabilities, data-loss bugs, or necessity findings were identified.

## Narrative Findings (AI reviewer)

## Warnings

### WR-01: Paced notice send closures discard bounded `callCtx`, defeating per-call timeout and shutdown cancellation (WARNING)

**File:** `fuku/modules/aispam_notice.go:151-168` (also `fuku/modules/memberreq_notice.go:402-406`)
**Issue:** `aispamNoticePaced` (`fuku/modules/aispam_notice.go:117-123`) derives `bounded, cancel := context.WithTimeout(callCtx, aispamNoticeCallTimeout)` from `poolCtx` and passes `bounded` to its `call` callback so that each Telegram request is bounded by 10 s and aborted on shutdown drain (`DrainAISpamChecks`, 9 s timeout). However, both `SendDefault` (line 153) and `SendCustom` (line 162) ignore the `context.Context` parameter (`func(context.Context) error`) and invoke `helpers.SendMessageWithErrorHandling` / `media.SendGreetingMessage`, neither of which accepts a context. As a result, `aispamNoticeCallTimeout` and `poolCtx` cancellation never bound or abort an in-flight Telegram HTTP call — only the pacer wait before the call starts. The same context drop occurs in `fuku/modules/memberreq_notice.go:402-406`, where `SendCustom` discards `callCtx` from `memberReqNoticeSend` (whereas `SendDefault` at lines 398-400 properly calls `b.SendMessageWithContext(callCtx, ...)`).
**Fix:** Pass `callCtx` into context-aware Telegram send methods so the configured call timeout and shutdown context actually cancel in-flight HTTP requests:
```go
// In aispam_notice.go (and add a context-aware variant for media.SendGreetingMessageContext):
SendDefault: func(text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
	var sent *gotgbot.Message
	err := aispamNoticePaced(poolCtx, func(callCtx context.Context) error {
		var sendErr error
		sent, sendErr = task.bot.SendMessageWithContext(callCtx, task.chatID, text, opts)
		return sendErr
	})
	return sent, err
},
```
(Apply the same `callCtx` propagation to `SendCustom` in both `aispam_notice.go:162` and `memberreq_notice.go:403` via a `media.SendGreetingMessageContext(callCtx, ...)` wrapper.)

### WR-02: `DeleteMessageWithErrorHandling` swallows `"message can't be deleted"`, causing `postAISpamNotice` to announce a deletion that did not occur (WARNING)

**File:** `fuku/modules/aispam.go:336-355`
**Issue:** `processAISpam` gates `postAISpamNotice(task)` on `if deleteErr == nil` (line 353), with the stated contract that the notice runs "only after the message was deleted" (line 352). However, `helpers.DeleteMessageWithErrorHandling` (`fuku/utils/helpers/telegram_helpers.go:17-25`) catches both `"message to delete not found"` and `"message can't be deleted"` and returns `nil`. If the bot lacks delete permissions in the group (or the message cannot be deleted by Telegram), `deleteErr` is `nil`, so `postAISpamNotice` still posts a notice claiming the message was removed as spam even though the spam message remains visible in the chat.
**Fix:** Call `task.bot.DeleteMessage(task.chatID, task.messageID, nil)` directly (or use a helper returning `(deleted bool, err error)` that only treats actual deletion success as `deleted = true`) and gate `postAISpamNotice(task)` on `deleted`:
```go
deleted, deleteErr := task.bot.DeleteMessage(task.chatID, task.messageID, nil)
if deleteErr != nil && !strings.Contains(deleteErr.Error(), "message to delete not found") {
	log.WithFields(log.Fields{
		"chat_id":    task.chatID,
		"message_id": task.messageID,
		"error":      deleteErr,
	}).Warn("[AISpam] failed to delete a message the model flagged as spam")
}
// ...
if deleted && deleteErr == nil {
	postAISpamNotice(task)
}
```

### WR-03: `NoMedia` rule disagrees between `eventmsg.NoMedia` / `composeEventPM` and the Messages PM editor UI (WARNING)

**File:** `fuku/modules/settings_messages.go:470-484` (cross-referenced with `fuku/db/eventmsg/events.go:178-188` and `fuku/modules/event_message_pm.go:408-411`)
**Issue:** `eventmsg.NoMedia(event, dest)` (`fuku/db/eventmsg/events.go:178-188`) returns `true` only for `DestGroup` of `EventReqWarn`, `EventReqKick`, `EventReqBan`, and `EventReqMute`, and returns `false` for `DestPM` (because private copies do not append the requirements line). Accordingly, `eventmsg.ValidateMessage` accepts media on member-requirement PM copies (e.g., via Copy Settings or backup import), and `composeEventPM` (`fuku/modules/event_message_pm.go:408-411`) sends that media. However, `settingsMsgPMEditDef` (`fuku/modules/settings_messages.go:473`) assigns `def.Parts = settingsMsgPartsOf(fam)`, which omits `settingsEditMediaPart` because `fam.NoMedia` is `true`. As a result:
- Owners cannot set, view, preview, or remove media on member-requirement PM copies in the Messages UI.
- If a member-requirement PM copy already has stored media (from import/copy), it is invisible in the UI yet still causes `settingsEditPromptMax` (`settings_edit_actions.go:45-46`) and `settingsEditCheckGreetingText` (`settings_edit_check.go:144-146`) to enforce the 1024-character caption limit instead of 4096.
**Fix:** Align both layers to one rule. Since private copies do not append the requirements line (and `settingsMsgPMEditDef` already sets `ReservedText = 0`), give PM definitions all three parts regardless of `fam.NoMedia`:
```go
func settingsMsgPMEditDef(fam settingsMsgFamily, ev settingsMsgEvent) settingsEditPageDef {
	def := settingsMsgEditCodes
	def.Key, def.Kind, def.Icon, def.TitleKey = settingsMsgPMWaitPage(ev), settingsEditEvent, settingsMsgPMIcon, "settings_msg_pm_title"
	def.Parts = []settingsEditPart{settingsMsgTextPart(fam), settingsEditMediaPart, settingsEditButtonsPart}
	// ...
```
(Alternatively, if member-requirement PM copies are intentionally text-only too, remove the `if dest != DestGroup` guard in `eventmsg.NoMedia` and strip media in `composeEventPM` when `r.TextOnly` is set.)

### WR-04: Warn-limit ban in `memberReqPostEventNotice` omits the group's Appeal button (WARNING)

**File:** `fuku/modules/memberreq_notice.go:376-388,409-419`
**Issue:** Every other automatic ban path (`antiflood.go:498`, `event_message_auto.go:299`, `event_message_auto.go:373`, `warns.go:532`) sets `Appeal: true` on `eventReply` when the applied action is `ban` or `tban`, so `eventSupportsAppeal` (`fuku/modules/event_message.go:129-141`) attaches the group's Appeal URL button to both the group notice and the private copy. In `memberReqPostEventNotice`, when `hold.Action == models.RequirementActionWarn` reaches the warn limit and applies a `ban` or `tban` (`warn != nil && warn.LimitReached && warn.LimitApplied` and `memberReqLimitMode(warn.Mode) == "ban"`), `reply.Event` is `EventReqWarn` (not `EventReqBan`) and `reply.Appeal` is never set. Banned members therefore do not receive the group's configured Appeal button on either the group notice or the private copy.
**Fix:** Set `Appeal` on `reply` in `memberReqPostEventNotice` when a warn-limit ban took effect:
```go
	appeal := hold.Action == models.RequirementActionWarn &&
		warn != nil && warn.LimitReached && warn.LimitApplied &&
		memberReqLimitMode(warn.Mode) == "ban"
	reply := eventReply{
		Event:        memberReqEventOf(hold.Action),
		Target:       memberReqEventTarget(user),
		Values:       values,
		Default:      text,
		PlainDefault: func() string { return memberReqPlainNoticeDefault(hold.Action)(tr, in) },
		Opts:         opts,
		Note:         memberReqNoticeAppended(hold.Action)(tr, in),
		TextOnly:     true,
		Required:     hold.Action == models.RequirementActionMute,
		Appeal:       appeal,
	}
```

## Info

### IN-01: Duplicate `memberReqNoticeNoteReserve` constant instead of referencing `eventmsg.MreqTextReserve` (INFO)

**File:** `fuku/modules/memberreq_notice.go:222`
**Issue:** `const memberReqNoticeNoteReserve = 600` duplicates `eventmsg.MreqTextReserve = 600` (`fuku/db/eventmsg/validate_write.go:30`), even though `memberreq_notice.go` already imports `fuku/db/eventmsg`.
**Fix:** Alias `const memberReqNoticeNoteReserve = eventmsg.MreqTextReserve` (or reference `eventmsg.MreqTextReserve` directly in `settings_messages.go:302`) so there is a single source of truth.

### IN-02: Group Full preview does not strip stored media for `NoMedia` events (INFO)

**File:** `fuku/modules/settings_messages.go:1058-1062`
**Issue:** On live send, `eventGroupMessage` (`fuku/modules/event_message.go:476-478`) clears `FileID` and `MediaType` when `r.TextOnly` is true (member requirement group notices). In `settingsMsgPreview`, `row` copies `st.FileID` and `st.MediaType` directly into `composeEventMessage` without clearing them for `eventmsg.NoMedia(ev.Key, eventmsg.DestGroup)`, so if a legacy or manually written row ever had media stored, Full preview would not match live send behavior.
**Fix:** Clear `row.FileID` and `row.MediaType` in `settingsMsgPreview` when `eventmsg.NoMedia(ev.Key, eventmsg.DestGroup)` is true.

---

_Reviewed: 2026-11-24T09:46:39Z_
_Reviewer: the agent (gsd-code-reviewer)_
_Depth: standard_
