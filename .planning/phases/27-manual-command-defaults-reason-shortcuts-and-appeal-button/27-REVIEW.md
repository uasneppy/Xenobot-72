---
phase: 27-manual-command-defaults-reason-shortcuts-and-appeal-button
reviewed: 2026-11-24T09:54:00Z
depth: standard
files_reviewed: 19
files_reviewed_list:
  - migrations/20261122150000_add_manual_command_tuning.sql
  - fuku/db/cache/local.go
  - fuku/db/models/modtuning.go
  - fuku/db/modtuning/repository.go
  - fuku/modules/modtuning_settings.go
  - fuku/modules/settings_moddefaults.go
  - fuku/modules/settings_reasons.go
  - fuku/modules/settings_input.go
  - fuku/modules/settings_input_door.go
  - fuku/modules/settings_list_input.go
  - fuku/modules/moderation.go
  - fuku/modules/bans.go
  - fuku/modules/mute.go
  - fuku/modules/warns.go
  - fuku/modules/antiflood.go
  - fuku/modules/event_message.go
  - fuku/modules/event_message_auto.go
  - fuku/modules/event_message_pm.go
  - fuku/modules/settings_messages.go
findings:
  critical: 0
  warning: 4
  info: 2
  total: 6
  necessity: 0
status: issues_found
---

# Phase 27: Code Review Report

**Reviewed:** 2026-11-24T09:54:00Z
**Depth:** standard
**Files Reviewed:** 19
**Status:** issues_found

## Summary

Reviewed all 19 files covering Phase 27 (Manual Command Defaults, Reason Shortcuts, and Appeal Button) at standard depth: the SQL migration and GORM models, `modtuning` repository and `skipLocal` cache registration, manual defaults and reason shortcut apply/expansion logic, command pipeline integration (`/ban`, `/dban`, `/sban`, `/tban`, `/kick`, `/dkick`, `/skick`, `/mute`, `/dmute`, `/smute`, `/tmute`, `/warn`, `/dwarn`, `/swarn`), appeal button attachment across group replies, private copies, and previews, and the private settings pages `q` (Manual defaults) and `rs` (Reason shortcuts).

Overall the implementation is well-structured, respects cache-coherency (`skipLocal`), avoids logging user text, enforces Telegram's 100-button cap, and guards private-only settings callbacks cleanly. Four warnings and two info items were found: (1) a fail-unsafe fallback to permanent ban/mute when `manualDefaultUntil` rejects a positive default duration, (2) empty-hostname/unroutable appeal URLs accepted by `validateAppealURL` combined with no retry-without-appeal fallback in `sendDefaultPost` and `runEventPM`, (3) a TOCTOU race on the 50-shortcut cap that can break downstream `ReplaceShortcutsTx` callers, and (4) a post-save read failure in `settingsModDefAppealTyped` that renders fabricated zero defaults.

## Narrative Findings (AI reviewer)

## Warnings

### WR-01: `manualBanMember` and `manualMuteMember` silently escalate to permanent punishment when `manualDefaultUntil` returns false

**File:** `fuku/modules/modtuning_settings.go:226-258`
**Issue:** `manualDefaultUntil` is explicitly documented to guard against invalid durations and `punishUntilTooClose` "so Telegram never reads a too-close expiry as permanent." However, when `defs.DefaultBanSecs > 0` (or `defs.DefaultMuteSecs > 0`) and `manualDefaultUntil` returns `(_, false)`, both `manualBanMember` (lines 228–238) and `manualMuteMember` (lines 246–256) fall straight through to `c.Chat.BanMember(c.Bot, t.userID, nil)` / `c.Chat.RestrictMember(c.Bot, t.userID, MutedPermissions, nil)` — executing a **permanent** ban or mute instead of refusing or failing safe like `applyTimedPunishment` (`punish_timed.go:175-180`). If a clock jump, GC pause between `punishUntil` and `punishUntilTooClose`, or corrupted cached duration triggers `ok == false`, a group with a configured temporary default ban/mute will silently permanently ban/mute the user.
**Fix:** When `defs.DefaultBanSecs > 0` (or `defs.DefaultMuteSecs > 0`) and `manualDefaultUntil` returns `false`, log a warning naming only the chat ID and return an error (or refuse without executing a permanent punishment) rather than falling through to the permanent call:
```go
func manualBanMember(c *moderationCtx, t *target) error {
	defs := c.manualDefaults()
	if defs.DefaultBanSecs > 0 {
		until, ok := manualDefaultUntil(defs.DefaultBanSecs)
		if !ok {
			log.Warnf("[ModTuning] default ban expiry of chat %d could not be computed; aborting ban", c.Chat.Id)
			return errPunishTimeRanOut
		}
		if _, err := c.Chat.BanMember(c.Bot, t.userID, &gotgbot.BanChatMemberOpts{UntilDate: until}); err != nil {
			return err
		}
		t.untilDate = until
		t.timeVal = punishDurationLabel(c.Tr, defs.DefaultBanSecs)
		return nil
	}
	_, err := c.Chat.BanMember(c.Bot, t.userID, nil)
	return err
}
```
Apply the same pattern to `manualMuteMember`.

### WR-02: `validateAppealURL` accepts empty-hostname URLs and ban reply paths have no fallback when Telegram rejects the appeal button

**File:** `fuku/modules/modtuning_settings.go:137-155` (also `fuku/modules/event_message.go:548-568`, `fuku/modules/event_message_pm.go:351-362`)
**Issue:** `validateAppealURL` delegates URL structure checks to `content.ValidButtonURL`, which only checks `(u.Scheme == "http" || u.Scheme == "https") && u.Host != ""`. Because Go's `net/url.Parse` sets `u.Host = ":80"` for `"https://:80"` (with `u.Hostname() == ""`), and accepts single-label/local hosts like `"http://localhost/appeal"` or `"https://appeal"`, an admin can save a URL that passes `validateAppealURL` but is rejected by Telegram Bot API with `400 Bad Request: BUTTON_URL_INVALID`. When that happens:
1. In `postEventGroup` (`event_message.go:527-532`), a refused custom message falls back to `sendDefaultPost(site, r)`, which still attaches the same `r.AppealURL` and fails with 400.
2. In `sendDefaultPost` (`event_message.go:548-565`), there is no retry without the appeal row on `eventMessageRefused(err)`, so **all ban replies in the group fail to send**.
3. In `runEventPM` (`event_message_pm.go:352-362`), the custom-message fallback also re-attaches `o.AppealRow` (lines 356–360) and fails with 400, while non-custom PM copies do not retry without `o.AppealRow` at all.
**Fix:**
1. Tighten `validateAppealURL` to parse the URL and require a non-empty `u.Hostname()` (and consider rejecting obvious non-public/dotless hosts if desired):
```go
u, err := url.Parse(trimmed)
if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
	return "", false
}
```
2. In `sendDefaultPost`, if `len(appealRow) > 0` was attached and `site.SendDefault` returns an error matching `eventMessageRefused(err)`, retry once with the original `r.Opts` (without the appeal row) and log a chat-ID-only warning. Similarly, in `runEventPM`, when `pmCopySend` returns `pmCopyRefused` and the message carried `o.AppealRow`, fall back to sending the default text without the appeal row so a broken appeal link can never suppress ban notifications.

### WR-03: TOCTOU race on `ReasonShortcutsCapMax` (50) can exceed the cap and break `ReplaceShortcutsTx` callers

**File:** `fuku/modules/modtuning_settings.go:309-333` (also `fuku/db/modtuning/repository.go:318-345`)
**Issue:** `applyReasonShortcutAdd` enforces the 50-shortcut per-chat cap by calling `CountFresh` (line 317) and then `Add` (`modtuning.AddShortcut`, line 325) in separate, unlocked DB round trips. Two concurrent admins adding different shortcut words when a chat has 49 shortcuts will both observe `count == 49 < 50` and both inserts will succeed, leaving 51 rows in `chat_reason_shortcuts`. Because `modtuning.ReplaceShortcutsTx` (`repository.go:348, 369`) strictly rejects `len(entries) > models.ReasonShortcutsCapMax`, any subsequent settings copy from that chat (`copy_settings.go:190`) will fail with `modtuning.ErrInvalidValue`.
**Fix:** Enforce the cap atomically inside `modtuning.AddShortcut` (e.g., within a transaction using a per-chat advisory lock `pg_advisory_xact_lock` on Postgres, or an atomic `INSERT INTO chat_reason_shortcuts (...) SELECT ... WHERE (SELECT COUNT(*) FROM chat_reason_shortcuts WHERE chat_id = ?) < ? ON CONFLICT (chat_id, word) DO NOTHING`), returning a distinct outcome when the cap is reached.

### WR-04: `settingsModDefAppealTyped` renders fabricated zero defaults on post-save read failure

**File:** `fuku/modules/settings_moddefaults.go:463-468`
**Issue:** After saving the appeal URL, `settingsModDefAppealTyped` calls `settingsModDefaultsReadNow(r.wait.GroupID)` to redraw page `q`. If that read fails, lines 464–466 substitute `defs = modtuning.Defaults{AppealURL: cleaned}` and still render the full Manual defaults screen. Any previously configured `DefaultBanSecs`, `DefaultMuteSecs`, or `ReasonRequired` values are displayed as `Permanent` / `Off`, misleading the admin into thinking those settings were wiped. By contrast, `settings_input_door.go:636-641` and `settings_list_input.go:300-305` send only the confirmation notice when the post-save reload fails.
**Fix:** On read error, log a warning and send only the saved confirmation notice (matching `settings_input_door.go:637-641`):
```go
notice := settingsNoticeOK(settingsText(r.tr, "settings_moddef_saved_appeal"))
defs, err := settingsModDefaultsReadNow(r.wait.GroupID)
if err != nil {
	log.Warnf("[Settings] manual defaults of chat %d could not be read after a typed save: %v", r.wait.GroupID, err)
	r.send(notice, nil, false)
	r.closePrompt()
	return
}
screen := renderSettingsModDefaults(r.view(), defs, notice)
r.send(screen.Text, &screen.Keyboard, false)
r.closePrompt()
```

## Info

### IN-01: `dispatchWarn` checks `reason_required` before admin-target validation

**File:** `fuku/modules/warns.go:616-621`
**Issue:** In `moderationCommand.run` (`fuku/modules/moderation.go:324-338`), target validation (admin protection, bot-self) runs before `manualApplyReasonRequired`. In `dispatchWarn` (`warns.go:616`), `manualApplyReasonRequired` runs before `warnThisUser` checks `IsUserAdmin(b, chat.Id, userId)` (line 393), so `/warn @admin` without a reason replies with `moddef_reason_required` instead of `warns_admin_warning_error`.
**Fix:** Check `IsUserAdmin(b, chat.Id, userId)` in `dispatchWarn` before `manualApplyReasonRequired`, or move target validation ahead of the reason check for consistency with `/ban`, `/kick`, and `/mute`.

### IN-02: Stale action code in doc comment (`mc` instead of `mk`)

**File:** `fuku/modules/settings_moddefaults.go:408`
**Issue:** The doc comment for `settingsModDefRunAppealCancel` states `serves mc`, whereas `settingsActModDefAppealCancel` (line 36) is `"mk"`.
**Fix:** Update the comment to `serves mk`.

---

_Reviewed: 2026-11-24T09:54:00Z_
_Reviewer: the agent (gsd-code-reviewer)_
_Depth: standard_
