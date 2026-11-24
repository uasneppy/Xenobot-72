---
phase: 21-message-engine-messages-page-and-manual-punishments
fixed_at: 2026-11-24T10:27:52Z
review_path: .planning/phases/21-message-engine-messages-page-and-manual-punishments/21-REVIEW.md
fix_scope: critical_warning
iteration: 1
findings_in_scope: 2
fixed: 2
skipped: 0
status: all_fixed
---

# Phase 21: Code Review Fix Report

**Fixed at:** 2026-11-24T10:27:52Z
**Source review:** `.planning/phases/21-message-engine-messages-page-and-manual-punishments/21-REVIEW.md`
**Iteration:** 1

**Summary:**
- Findings in scope (`critical_warning`): 2 (`WR-01`, `WR-02`; info findings `IN-01`, `IN-02` out of scope)
- Fixed: 2
- Skipped: 0

## Verification Environment

- **Location:** main checkout (`workflow.use_worktrees=false` in `.planning/config.json`)
- **Checks run:**
  - `go vet -tags testtools ./...` — PASS
  - `CGO_ENABLED=0 go build ./...` — PASS
  - `make check-translations` — PASS
  - `make check-docs` — PASS
  - `make test-shard-check` — PASS
  - `TZ=UTC go test -tags testtools -race -count=1 -v -run '^(TestUnrestrictButtonEditKeepsTextSafe|TestCallbackHandlersRejectMissingCallbackMessage|TestSettingsEditCheckTextParity|TestSettingsEditTextLimits|TestSettingsEditFilledLength|TestSettingsEditEventPlaceholders|TestSettingsEditTextButtonLines|TestSettingsEditMediaKinds|TestSettingsEditMediaTextLong|TestSettingsEditButtonRefusals|TestSettingsEditCheckMalformedEntities|TestEventEngineCallers|TestPMCopyCallers|TestEventMsgValidateWriteMatchesSettingsPage)$' ./fuku/modules` — PASS

## Fixed Issues

### WR-01: Unban/Unmute button edit fails on custom media ban and mute messages

**Files modified:** `fuku/modules/bans.go`, `fuku/modules/unrestrict_button_html_test.go`
**Commit:** `b4b876d9`
**Applied fix:** Extracted `editUnrestrictMessage` in `fuku/modules/bans.go` to branch on the callback message type: text messages (or nil `EffectiveMessage`) use `EditText` with `OriginalHTML()`, captionless media (`Sticker` or `VideoNote`) clear the inline keyboard with `EditReplyMarkup`, and caption-capable media (photo, video, animation, document, audio, voice) use `EditCaption` with `OriginalCaptionHTML()`. Added race-tested cases in `TestUnrestrictButtonEditKeepsTextSafe` covering captioned media, uncaptioned media, and stickers.

### WR-02: Event placeholders are not modeled in `settingsEditFilledLongest`, allowing blank-when-empty templates and undercounting `{admin}`/`{until}`

**Files modified:** `fuku/modules/settings_edit_check.go`, `fuku/modules/settings_edit_check_test.go`
**Commit:** `328c296a`
**Applied fix:** Added `settingsEditEventMinimalReplacer` and `settingsEditEventWorstReplacer` in `fuku/modules/settings_edit_check.go` and applied them in `settingsEditFilledLongest` when `in.Def.Kind == settingsEditEvent` before calling `formatting.FillPlaceholders`. On the minimal pass, optional event tokens (`{reason}`, `{duration}`, `{until}`, `{action}`, `{count}`, `{limit}`, `{window}`, `{percent}`) substitute to `""` and `{admin}` substitutes to `"A"` so blank-when-empty event templates are refused with `settings_edit_refuse_need_text`. On the worst-case pass, `{admin}` expands to 129 UTF-16 units (`64 + space + 64`) and `{until}` to 16 units (`02.01.2006 15:04`). Added `TestSettingsEditEventPlaceholders` in `fuku/modules/settings_edit_check_test.go`.

---

_Fixed: 2026-11-24T10:27:52Z_
_Fixer: the agent (gsd-code-fixer)_
_Iteration: 1_
