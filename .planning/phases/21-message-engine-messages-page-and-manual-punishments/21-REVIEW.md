---
phase: 21-message-engine-messages-page-and-manual-punishments
reviewed: 2026-11-24T09:46:00Z
depth: standard
files_reviewed: 19
files_reviewed_list:
  - migrations/20261120110000_add_chat_event_messages.sql
  - fuku/db/cache/local.go
  - fuku/db/models/event_message.go
  - fuku/db/eventmsg/events.go
  - fuku/db/eventmsg/repository.go
  - fuku/modules/event_message.go
  - fuku/modules/event_message_apply.go
  - fuku/modules/settings_messages.go
  - fuku/modules/settings_edit.go
  - fuku/modules/settings_edit_actions.go
  - fuku/modules/settings_edit_check.go
  - fuku/modules/settings_edit_preview.go
  - fuku/modules/settings_input_door.go
  - fuku/modules/settings_menu.go
  - fuku/modules/settings_pages.go
  - fuku/modules/bans.go
  - fuku/modules/mute.go
  - fuku/modules/moderation.go
  - fuku/modules/bot_updates.go
findings:
  critical: 0
  warning: 2
  info: 2
  total: 4
  necessity: 0
status: issues_found
---

# Phase 21: Code Review Report

**Reviewed:** 2026-11-24T09:46:00Z
**Depth:** standard
**Files Reviewed:** 19
**Status:** issues_found

## Summary

Reviewed all 19 Phase 21 files covering the `chat_event_messages` schema, cache `skipLocal` wiring, `fuku/db/eventmsg` repository and event metadata, the event message engine (`event_message.go`, `event_message_apply.go`), the Messages settings page and hooked editor integration (`settings_messages.go`, `settings_edit*.go`, `settings_input_door.go`, `settings_menu.go`, `settings_pages.go`), anonymous-admin actor marking (`bot_updates.go`), and the manual punishment send sites (`bans.go`, `mute.go`, `moderation.go`).

Overall architecture is solid: column-targeted upserts prevent concurrent part-writer clobbering, `event_messages:<chat>` is excluded from process-local LRU caching, sentinel-based placeholder replacement prevents cross-substitution and HTML injection, and post-punishment send failures fall back cleanly to default replies without leaking stored content to logs. Two functional warnings were found: pressing the preserved Unban/Unmute button on a custom media ban or mute message fails when editing the message because `unrestrictButtonHandler` calls `EditText`, and `settingsEditFilledLongest` does not account for event placeholders when checking for blank-after-fill or worst-case length.

## Narrative Findings (AI reviewer)

## Warnings

### WR-01: Unban/Unmute button edit fails on custom media ban and mute messages

**File:** `fuku/modules/bans.go:980-1003`
**Issue:** Phase 21 allows groups to attach media (photo, video, animation, document, audio, voice, or sticker) to `EventBan` and `EventMute` replies while preserving the bot's `Unban` / `Unmute` inline keyboard row (`composeEventMessage` in `fuku/modules/event_message.go:360-375`). When an admin clicks that button on a media message, `unrestrictButtonHandler` calls `ctx.EffectiveMessage.OriginalHTML()` (which reads `msg.Text` and returns `""` for captioned media) and then unconditionally calls `msg.EditText(...)`. Telegram's `editMessageText` rejects media messages with `400 Bad Request: there is no text in the message to edit`, leaving the `Unban`/`Unmute` button live on the message, failing to show who unbanned/unmuted the user, logging an error, and returning `editErr`.
**Fix:** Branch on whether the callback message has text, caption-capable media, or captionless media (sticker):
```go
var editErr error
if ctx.EffectiveMessage != nil && ctx.EffectiveMessage.Text != "" {
	updatedText := "<s>" + ctx.EffectiveMessage.OriginalHTML() + "</s>\n\n"
	_, _, editErr = msg.EditText(b, &gotgbot.EditMessageTextOpts{
		Text:      updatedText + helpText,
		ParseMode: formatting.HTML,
	})
} else if ctx.EffectiveMessage != nil && ctx.EffectiveMessage.Sticker != nil {
	// Stickers cannot carry a caption; clear the inline keyboard so the button is not left live.
	_, _, editErr = msg.EditReplyMarkup(b, &gotgbot.EditMessageReplyMarkupOpts{
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{},
	})
} else {
	updatedCaption := ""
	if ctx.EffectiveMessage != nil {
		updatedCaption = ctx.EffectiveMessage.OriginalCaptionHTML()
	}
	if updatedCaption != "" {
		updatedCaption = "<s>" + updatedCaption + "</s>\n\n"
	}
	_, _, editErr = msg.EditCaption(b, &gotgbot.EditMessageCaptionOpts{
		Caption:   updatedCaption + helpText,
		ParseMode: formatting.HTML,
	})
}
```

### WR-02: Event placeholders are not modeled in `settingsEditFilledLongest`, allowing blank-when-empty templates and undercounting `{admin}`/`{until}`

**File:** `fuku/modules/settings_edit_check.go:201-214`
**Issue:** `settingsEditCheckGreetingText` validates `settingsEditEvent` texts using `settingsEditFilledLongest`, which calls `formatting.FillPlaceholders` (Welcome placeholders only). Event placeholders (`{reason}`, `{duration}`, `{until}`, `{admin}`, `{action}`, `{count}`, `{limit}`, `{window}`, `{percent}`) are left as literal `{name}` strings during validation:
1. A text-only custom event message consisting solely of optional event placeholders (such as `{reason}` or `{duration}`) passes `minimalBlank` because `"{reason}"` is non-empty. At runtime, when a punishment has no reason (or on `EventLockLift` preview where `NoReason: true`), `renderEventText` substitutes `""`, producing an empty text that Telegram rejects with 400 (`message text is empty`), causing live replies to fall back to the default and `Preview text` to fail with `settings_edit_preview_failed`.
2. Worst-case length validation measures `{admin}` as 7 UTF-16 units instead of a worst-case 129-unit admin full name (64 + space + 64) and `{until}` as 7 units instead of 16 (`02.01.2006 15:04`), allowing texts near the 4096/1024-unit limit to pass validation and fail at send time.
**Fix:** For `in.Def.Kind == settingsEditEvent`, substitute event placeholders before calling `formatting.FillPlaceholders` in `settingsEditFilledLongest`: use `""` for optional event tokens (`{reason}`, `{duration}`, `{until}`, `{action}`, `{count}`, `{limit}`, `{window}`, `{percent}`) on the `minimal` pass so templates that can render empty are refused with `settings_edit_refuse_need_text`, and use worst-case sample widths for `{admin}` (129 chars) and `{until}` (16 chars) on the worst-case pass.

## Info

### IN-01: Redundant map copy in `settingsMsgEditActions`

**File:** `fuku/modules/settings_messages.go:343-351`
**Issue:** `settingsMsgEditActions` calls `settingsEditResolvedActions(...)`, which already returns a map containing exactly the five action codes (`hs`, `hr`, `hc`, `hv`, `hf`), and then copies those same five entries into a second map.
**Fix:** Return `settingsEditResolvedActions(settingsMsgEditCodes, []string{settingsMsgEventField, settingsMsgDestField}, settingsMsgDefByFields)` directly.

### IN-02: Stale doc comment on `settingsMsgPMEditDef` contradicts `ReservedButtons` assignment

**File:** `fuku/modules/settings_messages.go:467-477`
**Issue:** The doc comment for `settingsMsgPMEditDef` states "no reserved buttons", while lines 475-477 directly below set `def.ReservedButtons = 1` for `EventBan`, `EventTimedBan`, and `EventReqBan` (to reserve room for the Appeal button).
**Fix:** Update the comment to note that ban events reserve 1 button for the Appeal link while other events reserve none.

---

_Reviewed: 2026-11-24T09:46:00Z_
_Reviewer: the agent (gsd-code-reviewer)_
_Depth: standard_
