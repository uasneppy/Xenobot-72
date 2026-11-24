---
phase: 23-warn-flood-blacklist-and-forward-blocklist-messages
reviewed: 2026-11-24T09:46:00Z
depth: standard
files_reviewed: 10
files_reviewed_list:
  - fuku/db/eventmsg/events.go
  - fuku/modules/antiflood.go
  - fuku/modules/blacklists.go
  - fuku/modules/event_message.go
  - fuku/modules/event_message_auto.go
  - fuku/modules/event_message_pm.go
  - fuku/modules/forwardblock_notice.go
  - fuku/modules/settings_messages.go
  - fuku/modules/staff_forwardblock_watch.go
  - fuku/modules/warns.go
findings:
  critical: 0
  warning: 5
  info: 3
  total: 8
  necessity: 2
status: issues_found
---

# Phase 23: Code Review Report

**Reviewed:** 2026-11-24T09:46:00Z
**Depth:** standard
**Files Reviewed:** 10
**Status:** issues_found

## Summary

Reviewed all 10 source files covering Phase 23 (Warn, Flood, Blacklist, and Forward Blocklist Messages) at standard depth against current `HEAD`.

Overall, the Phase 23 architecture is well-structured: Staff Group privacy is cleanly preserved (`forwardblock_notice.go` keeps blocked source and Staff Group identifiers out of scope, `staff_forwardblock_watch.go` logs only chat/user IDs and error classes, and `settings_messages.go` hides the Messages hub in Staff Groups), event/destination validation in `fuku/db/eventmsg/events.go` matches the Messages hub and reserved-button caps, and private-copy pacing/capping in `event_message_pm.go` properly isolates automatic rules via `ByRule` and `Automatic`.

No Critical security or data-loss vulnerabilities were found. However, five Warnings (one necessity) and three Info items (one necessity) were identified, most notably:
- `rmWarnButton` in `warns.go` calls `EditText`, which fails on custom media warn messages and leaves the `Remove warn` button active without answering the callback query.
- `warnSite` in `warns.go` omits `ThreadID`, causing warn and warn-limit notices to land in the General topic in forum supergroups when the triggering message was deleted (`/swarn`, blacklist warn, forward-blocklist warn).
- `warnPostLimit` can attempt to send an empty message when an inert warn-limit mode is reached with no stored reasons, and contains an unreachable `limitKeys` branch.
- Unescaped HTML interpolation in untimed `blacklistNoticeDefaultReply` and several command replies can break Telegram HTML parsing.

## Narrative Findings (AI reviewer)

## Warnings

### WR-01: `rmWarnButton` fails on custom media warn messages and leaves the button active without answering the callback

**File:** `fuku/modules/warns.go:821-831`
**Issue:** `EventWarn` is registered in the Automatic family (`fuku/modules/settings_messages.go:189-193, 295-298`) with `NoMedia: false`, so groups can attach media (photo, video, animation, document, audio, voice, sticker, or video note) to custom warn messages while `warnEventReply` (`warns.go:503`) keeps the `Remove warn` (`rmWarn`) button. When an admin taps `Remove warn` on a custom media warn message, `rmWarnButton` removes the warn from the database (`warns.go:806`) and then calls `query.Message.EditText(...)` (`warns.go:821`). Telegram rejects `editMessageText` on media messages with `400 Bad Request: there is no text in the message to edit`. Because `rmWarnButton` returns `err` immediately at line 824 before calling `query.Answer(b, nil)` at line 827, the admin's client spins until timeout and the `Remove warn` button remains attached—allowing accidental repeated taps to remove additional warns.
**Fix:**
```go
	var editErr error
	if m, ok := query.Message.(*gotgbot.Message); ok && m.Text == "" {
		if m.Caption != "" || m.Photo != nil || m.Video != nil || m.Animation != nil || m.Document != nil || m.Audio != nil || m.Voice != nil {
			_, _, editErr = query.Message.EditCaption(b, &gotgbot.EditMessageCaptionOpts{Caption: replyText, ParseMode: formatting.HTML})
		} else {
			// Captionless media (sticker, video_note): clear the button and surface the result in the callback toast.
			_, _, editErr = query.Message.EditReplyMarkup(b, &gotgbot.EditMessageReplyMarkupOpts{ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
		}
	} else {
		_, _, editErr = query.Message.EditText(b, &gotgbot.EditMessageTextOpts{Text: replyText, ParseMode: formatting.HTML})
	}
	if editErr != nil {
		log.Error(editErr)
	}
	// Always answer the callback after the warn removal took effect.
	if _, err := query.Answer(b, nil); err != nil {
		log.Error(err)
		return err
	}
	if editErr != nil {
		return editErr
	}
```
_(Note: `unrestrictButtonHandler` in `fuku/modules/bans.go:990` has the same `EditText` limitation on custom media `EventFlood` messages, though it does answer the callback query.)_

### WR-02: `warnSite` omits `ThreadID`, misrouting warn notices to the General topic in forum supergroups after deletion

**File:** `fuku/modules/warns.go:454-471`
**Issue:** Unlike `moderationCtx.site()` (`fuku/modules/event_message.go:416-418`) and `floodNotice` (`fuku/modules/antiflood.go:475, 480`), `warnSite` leaves `site.ThreadID` as `0`, and `warnEventReply` / `warnLimitEventReply` (`warns.go:495, 520`) do not set `opts.MessageThreadId`. On `/swarn` (`warns.go:415`), blacklist `warn` (`fuku/modules/blacklists.go:791`), and forward-blocklist `warn` (`fuku/modules/staff_forwardblock_watch.go:304`), the triggering message is deleted before `postEvent` runs. Because `AllowSendingWithoutReply: true` is set and `ThreadID` is `0`, Telegram drops the missing reply target and posts both default and custom warn / warn-limit notices into the forum's General topic instead of the topic where the offence occurred.
**Fix:**
```go
	if ctx.EffectiveMessage.IsTopicMessage {
		site.ThreadID = ctx.EffectiveMessage.MessageThreadId
	}
```
Also propagate `MessageThreadId` onto `opts` in `warnEventReply` and `warnLimitEventReply` when `ctx.EffectiveMessage.IsTopicMessage` is true (non-topic messages keep `MessageThreadId == 0`, preserving non-forum requests byte for byte).

### WR-03: `warnPostLimit` sends an empty message text when an inert warn-limit mode is reached with no reasons

**File:** `fuku/modules/warns.go:539-573`
**Issue:** `warnPostLimit` is called from `warnThisUser` (`warns.go:437-439`) when `res.LimitReached` is true and `res.LimitApplied` is false (an inert warn mode such as `""`, `tmute`/`tban` with `Secs == 0`, or `errPunishTimeRanOut`). For all such modes, `reply` starts as `""`. If none of the accumulated warns had a reason (`len(res.Reasons) == 0`), `reply` remains `""` and `b.SendMessage(chat.Id, "", ...)` at line 555 fails with Telegram `400 Bad Request: message text is empty`.
**Fix:**
```go
	if strings.TrimSpace(reply) == "" {
		return nil
	}
```
Add this guard before `b.SendMessage` at line 555 (or provide a non-empty fallback message for inert limit hits without reasons).

### WR-04: Unescaped HTML interpolation in untimed `blacklistNoticeDefaultReply` and command error replies

**File:** `fuku/modules/event_message_auto.go:167`
**Issue:** On the timed branch (`event_message_auto.go:158`), `blacklistNoticeDefaultReply` escapes `in.Reason` with `html.EscapeString(in.Reason)`. On the untimed branch (`event_message_auto.go:167`), it splices `in.Reason` raw into a message sent with `formatting.Shtml()` (`ParseMode: HTML`). Because `blacklists.go:792` builds `filled := fmt.Sprintf(reason, i)` from the raw blacklisted word `i`, any blacklisted word containing `<`, `>`, or `&` (e.g., `a&b`, `<tag>`) causes Telegram to either reject the notice with `400 Bad Request: can't parse entities` or render injected HTML. The same unescaped interpolation into `formatting.Shtml()` occurs in `fuku/modules/antiflood.go:762` (`args[0]`), `fuku/modules/warns.go:217` (`args[0]`), and `fuku/modules/blacklists.go:371, 393` (`result.TooLong`, `result.Already`).
**Fix:** Wrap `in.Reason` in `html.EscapeString` at `fuku/modules/event_message_auto.go:167` (updating legacy golden fixtures if appropriate) and wrap user-supplied values in `html.EscapeString` at `antiflood.go:762`, `warns.go:217`, and `blacklists.go:371, 393`.

### WR-05: Impossible-state guard — `limitKeys` map in `warnPostLimit` can never match

**File:** `fuku/modules/warns.go:541-549`
**Category:** necessity
**Issue:** `warnPostLimit` has only one caller in the repository (`fuku/modules/warns.go:439`), reached only when `res.LimitReached` is true, `res.LimitApplied` is false, and `res.LimitErr` is nil (`warns.go:430-439`). In `applyWarn` and `applyWarnLimitUntimed` (`warns.go:289-302, 331-358`), every mode in `limitKeys` (`"kick"`, `"mute"`, `"ban"`) always either returns a non-nil `actionErr` (setting `res.LimitErr`, which returns at line 431) or returns `punished = true` (setting `res.LimitApplied = true` or returning a reset error at line 421). Consequently, `limitKeys[res.Mode]` can never match when `warnPostLimit` executes.
**Fix:** Remove the `limitKeys` map and `if key, ok := limitKeys[res.Mode]; ok` block from `warnPostLimit` and document in its comment that it is only invoked for inert/unapplied limit modes.

## Info

### IN-01: `pmCopySend` ignores the timeout-bounded `callCtx` created by `pmCopyPaced`

**File:** `fuku/modules/event_message_pm.go:382-388`
**Issue:** `pmCopyPaced` (`event_message_pm.go:130-136`) derives a `pmCopyCallTimeout` (10s) context and passes it to its callback, but `pmCopySend` discards it (`func(context.Context) error`) because `media.SendGreetingMessage` takes no `context.Context`. As a result, `pmCopyCallTimeout` and `pmCopyJobTimeout` bound only the pacer wait, not an in-flight Telegram HTTP request (unlike `pmCopyAddDetails` and `pmCopyPostLog`, which call `*WithContext` endpoints).
**Fix:** Add a context-accepting `media.SendGreetingMessageWithContext` and pass `callCtx`, or document in `pmCopySend` that the HTTP request relies on the bot client's timeout.

### IN-02: Missing `nil` guard on `punish` returned by `forwardBlockPunishmentOf`

**File:** `fuku/modules/staff_forwardblock_watch.go:330-335`
**Issue:** Although `escalation.Get` currently validates ladder steps to `mute`/`tmute`/`kick`/`tban`/`ban`, `forwardBlockPunishmentOf(p)` (`staff_forwardblock_watch.go:351-358`) returns a map lookup (`forwardBlockPunishments[p.Action]`) that would yield `nil` if an unrecognized action were ever returned by `escalate`. Invoking `punish(b, ctx, userID)` at line 334 would panic (recovered by `forwardBlockWatcher`, bypassing the structured failure log).
**Fix:** Add `if punish == nil { return }` before `if err := punish(b, ctx, userID); err != nil`.

### IN-03: Already-defended-elsewhere — duplicate `RequireUserOwner` check in `resetAllWarns`

**File:** `fuku/modules/warns.go:984`
**Category:** necessity
**Issue:** `resetAllWarns` already enforces ownership and returns `ext.EndGroups` on failure at `fuku/modules/warns.go:972-975`. Wrapping the confirmation reply in `if chat_status.RequireUserOwner(b, ctx, chat, user.Id)` again at line 984 repeats the same owner check unnecessarily.
**Fix:** Remove the redundant `if chat_status.RequireUserOwner(b, ctx, chat, user.Id)` wrapper at line 984.

---

_Reviewed: 2026-11-24T09:46:00Z_
_Reviewer: the agent (gsd-code-reviewer)_
_Depth: standard_
