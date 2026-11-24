---
phase: 22-pm-copies
reviewed: 2026-11-24T09:48:00Z
depth: standard
files_reviewed: 13
files_reviewed_list:
  - migrations/20261120120000_add_event_message_options.sql
  - fuku/db/models/event_message.go
  - fuku/db/eventmsg/events.go
  - fuku/db/eventmsg/repository.go
  - fuku/modules/event_message.go
  - fuku/modules/event_message_apply.go
  - fuku/modules/event_message_pm.go
  - fuku/modules/event_message_pm_details.go
  - fuku/modules/event_message_pm_log.go
  - fuku/modules/moderation.go
  - fuku/modules/mute.go
  - fuku/modules/settings_edit.go
  - fuku/modules/settings_messages.go
findings:
  critical: 0
  warning: 4
  info: 3
  total: 7
  necessity: 1
status: issues_found
---

# Phase 22: Code Review Report

**Reviewed:** 2026-11-24T09:48:00Z
**Depth:** standard
**Files Reviewed:** 13
**Status:** issues_found

## Summary

Reviewed all 13 Phase 22 (PM Copies) source and migration files against live `HEAD`, focusing on the private copy delivery pipeline (`event_message_pm.go`), Details button and deep-link authorization (`event_message_pm_details.go`), PM failure log channel posting (`event_message_pm_log.go`), options schema and repository (`20261120120000_add_event_message_options.sql`, `event_message.go`, `repository.go`), and Settings UI controls (`settings_messages.go`, `settings_edit.go`).

Overall, the core security boundaries are solid: `pmCopyDetailsDeepLink` enforces strict token grammar (`pmcopy_` + 22 base64url chars from 128-bit `crypto/rand`), binds access solely to `rec.UserID == user.Id`, and returns a uniform neutral reply across all error/mismatch cases; `event_messages:<chat>` is listed in `skipLocal`; options upserts are column-targeted; and automatic PM copies fail closed on Redis errors.

Four warnings and three info items were identified:
- `pmCopySend` discards the per-call timeout context created by `pmCopyPaced`, leaving Telegram HTTP calls unbounded by `pmCopyCallTimeout` and `pmCopyJobTimeout`.
- Member requirements PM copies allow media in `eventmsg.NoMedia` and `composeEventPM`, but `settingsMsgPMEditDef` hides the Media part in Settings.
- `pmCopyDetailsDeepLink` gives no reply at all if sending a stored custom copy (captured after a 403) fails validation on Telegram.
- `pmCopyAddDetails` edits reply markup asynchronously using a pre-send snapshot of `GroupRows`, which can resurrect already-resolved inline action buttons (Unban/Unmute/Remove warn).

## Narrative Findings (AI reviewer)

## Warnings

### WR-01: `pmCopySend` discards the bounded timeout context from `pmCopyPaced`

**File:** `fuku/modules/event_message_pm.go:382-388`
**Issue:** `pmCopyPaced` (`event_message_pm.go:130-136`) derives `bounded, cancel := context.WithTimeout(callCtx, pmCopyCallTimeout)` (10s) inside `pmCopyJobTimeout` (60s) and passes it to its callback, specifically to bound each Telegram request. However, `pmCopySend` ignores the context parameter (`func(context.Context) error`) and calls `media.SendGreetingMessage(o.Bot, o.UserID, msg)`, which calls `gotgbot` without a context (`fuku/utils/media/sender.go:113, 363`). Unlike `pmCopyAddDetails` (`EditMessageReplyMarkupWithContext`) and `pmCopyPostLog` (`SendMessageWithContext`), a hanging Telegram send in `pmCopySend` will not cancel when `pmCopyCallTimeout` or `pmCopyJobTimeout` expires, pinning one of the 16 `pmCopyJobs` slots until the underlying HTTP client times out.
**Fix:** Add a context-aware `media.SendGreetingMessageWithContext(ctx, b, chatID, msg)` (routing to gotgbot's `*WithContext` send methods) and pass `callCtx` from `pmCopySend`:
```go
func pmCopySend(ctx context.Context, o eventPMOffer, msg media.GreetingMessage) (pmCopyOutcome, int) {
	err := pmCopyPaced(ctx, func(callCtx context.Context) error {
		_, sendErr := media.SendGreetingMessageWithContext(callCtx, o.Bot, o.UserID, msg)
		return sendErr
	})
	return pmCopyClassify(err), pmCopyErrCode(err)
}
```

### WR-02: Member requirements PM copies disagree on media support between Settings UI, DB validation, and send path

**File:** `fuku/modules/settings_messages.go:470-474`
**Issue:** `eventmsg.NoMedia(event, dest)` (`fuku/db/eventmsg/events.go:178-188`) returns `false` whenever `dest != DestGroup`, so DB validation, Copy Settings, and Backup Import allow storing media on Member Requirements PM copies (`mreqwarn`, `mreqkick`, `mreqban`, `mreqmute`), and `composeEventPM` (`fuku/modules/event_message_pm.go:408-411`) sends `row.FileID`/`row.MediaType` for them. However, `settingsMsgPMEditDef` sets `def.Parts = settingsMsgPartsOf(fam)`, which drops `settingsEditMediaPart` whenever `fam.NoMedia` is `true` (`settings_messages.go:301, 323-328`). As a result, admins cannot view, set, preview, or remove media on Member Requirements PM copies in Settings even when media is stored and actively sent to users.
**Fix:** Align all three layers. Since PM copies do not append the group requirements line/remedies, include `settingsEditMediaPart` on PM definitions in `settingsMsgPMEditDef`:
```go
def.Parts = []settingsEditPart{settingsMsgTextPart(fam), settingsEditMediaPart, settingsEditButtonsPart}
```
(Or, if Member Requirements PM copies are intentionally text-and-buttons only, update `eventmsg.NoMedia` and `composeEventPM` to reject/strip media for `DestPM` as well.)

### WR-03: `pmCopyDetailsDeepLink` returns silence when sending a stored custom copy fails

**File:** `fuku/modules/event_message_pm_details.go:236-246`
**Issue:** When a private copy fails with 403 (`pmCopyBlocked`), Telegram rejects the call before validating the message payload, so `runEventPM` (`fuku/modules/event_message_pm.go:351-374`) does not fall back to `o.Fallback` and stores the unvalidated custom `o.Message` into Redis. When the punished user later taps Details and opens `/start pmcopy_<token>`, Telegram validates `copyMsg` for the first time. If `media.SendGreetingMessage` fails (e.g., an expired/invalid `FileID` or rejected HTML/buttons returns 400), `pmCopyDetailsDeepLink` only logs a warning and returns `ext.EndGroups` — the user receives no fallback message and no neutral error reply.
**Fix:** Store `Fallback` (and any appeal button row) in `pmCopyDetailsRecord` when `o.Custom` is true, and on `eventMessageRefused(err)` send the fallback text once; if sending still fails, reply with `neutral()` so the user is not met with silence:
```go
if _, err := media.SendGreetingMessage(b, user.Id, copyMsg); err != nil {
	if eventMessageRefused(err) && rec.Fallback != "" {
		fallbackMsg := media.GreetingMessage{Text: rec.Fallback, MsgType: db.TEXT}
		if _, fbErr := media.SendGreetingMessage(b, user.Id, fallbackMsg); fbErr == nil {
			return ext.EndGroups
		}
	}
	log.Warnf("[EventMessage] the Details copy of user %d in chat %d could not be shown (code %d)", user.Id, rec.ChatID, pmCopyErrCode(err))
	return neutral()
}
```

### WR-04: `pmCopyAddDetails` can overwrite later keyboard updates and resurrect resolved action buttons

**File:** `fuku/modules/event_message_pm_details.go:169-184`
**Issue:** `pmCopyAddDetails` runs asynchronously after `pmCopySend` (which can wait up to `pmCopyPacerOptions.MaxWait` = 15s for a pacer slot or 429 retry) and waits again on `pmCopyPaced` before calling `EditMessageReplyMarkupWithContext` with a snapshot of `o.GroupRows` captured at send time. If an admin clicks an inline action button on the group message (such as Unban, Unmute, or Remove warn, which edits the message and clears its keyboard) before `pmCopyAddDetails` completes, `EditMessageReplyMarkupWithContext` blindly restores the stale `o.GroupRows` alongside the Details button, resurrecting an already-executed moderation button.
**Fix:** Avoid restoring stale interactive action rows after a delay — either bound how late `pmCopyAddDetails` may edit a message with interactive rows, or skip editing if the message's action callback has already been handled (e.g., track recently resolved message IDs or only append Details when the reply is still within a short window).

## Info

### IN-01: `pmCopyAddDetails` does not guard against an empty `o.DetailsLabel`

**File:** `fuku/modules/event_message_pm_details.go:150-176`
**Issue:** Unlike `eventAppealRow` (`fuku/modules/event_message.go:150-153`), which returns `nil` when its label is empty, `pmCopyAddDetails` does not check `o.DetailsLabel == ""` before writing a 48-hour Redis key (`pmCopyDetailsPut`) and calling `EditMessageReplyMarkupWithContext`. An empty label (e.g., missing translation or direct `offerEventPM` call without `DetailsLabel`) creates an orphaned Redis entry and makes a doomed Telegram API call (400 `BUTTON_TEXT_INVALID`).
**Fix:** Add an early `if o.DetailsLabel == ""` guard at the top of `pmCopyAddDetails` before calling `pmCopyDetailsPut`.

### IN-02: `pmCopyPostLog` calls `actionlog.Destination` without propagating `ctx`

**File:** `fuku/modules/event_message_pm_log.go:78-79`
**Issue:** `pmCopyPostLog` checks `ctx.Err() != nil` and passes `ctx` to `pmCopyPaced`, but calls `actionlog.Destination(&chat, logchannels.CategoryAdmin)` which invokes `logchannels.Get(chat.Id)` using `context.Background()` (`fuku/db/logchannels/repository.go:43-45`). A slow DB/cache lookup on a cache miss is therefore not bounded by `pmCopyJobTimeout`.
**Fix:** Add `actionlog.DestinationContext(ctx, &chat, category)` backed by `logchannels.GetContext` and call it from `pmCopyPostLog`.

### IN-03: Impossible-state guard — `settingsMsgAnyPM()` always evaluates to true

**File:** `fuku/modules/settings_messages.go:611-620`
**Category:** necessity
**Issue:** `settingsMsgAnyPM()` scans `settingsMsgFamilies` to decide whether to render the private copies options block on the Messages hub (`settings_messages.go:1182`). `eventPMSpecs` is statically populated at `init()` with 18 events (`fuku/modules/event_message_pm.go:76-106`) that are statically listed in `settingsMsgFamilies` (`fuku/modules/settings_messages.go:290-309`), and no test overrides `settingsMsgFamilies` to a PM-less set. The false branch is never reached in production or tests.
**Fix:** Remove `settingsMsgAnyPM()` and render the private copies options block unconditionally in `renderSettingsMsgHub`.

---

_Reviewed: 2026-11-24T09:48:00Z_
_Reviewer: the agent (gsd-code-reviewer)_
_Depth: standard_
