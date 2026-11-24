---
phase: 26-timed-punishments-escalation-and-warn-expiry
plan: 07
subsystem: settings, warns
status: complete
tags: [warn-expiry, settings, warnings-page, timed-punishments]
requires:
  - 26-03 (the Warnings page's timed row, nd and nt, settingsWarnsRedraw)
  - 26-06 (warns.SetWarnExpiryContext, WarnSettings.ExpiryDays, the counting rule, punishNow, punishDurationLabel)
provides:
  - applyWarnExpiry, warnSettingsBadExpiry, warnSettingsWrites.Expiry (fuku/modules/warns.go)
  - the Warnings page's expiry line and full-width expiry button, the ne and nx page actions, renderSettingsWarnsExpiry,
    settingsWarnsExpiryPresets, settingsWarnsExpiryLabel (fuku/modules/settings_warns.go)
  - locale keys settings_warns_expiry_line, settings_btn_warns_expiry, settings_warns_expiry_intro, settings_warns_expiry_hint (8 files)
affects:
  - 26-12 (proves TUNE-03 end to end through this page)
tech-stack:
  added: []
  patterns:
    - "a setting with no command: one apply function beside its siblings, drift and authority tests in place of a parity test"
key-files:
  created:
    - fuku/modules/settings_warns_expiry_test.go
  modified:
    - fuku/modules/warns.go
    - fuku/modules/settings_warns.go
    - fuku/modules/settings_warns_test.go
    - fuku/modules/settings_warns_timed_test.go
    - fuku/modules/settings_warns_screens_test.go
    - fuku/i18n/settings_locale_test.go
    - locales/en.yml, es.yml, fr.yml, hi.yml, id.yml, pt.yml, ru.yml, uk.yml
    - AGENTS.md
key-decisions:
  - "The expiry button is a full-width row, so its label is held to the repository's full-width ceiling of 40 code points (with its longest state), not 20; the expiry screen's preset cells are half-width and held to 20. A 20-point button with a word is impossible in French and Spanish once the state is '2 semaine(s)' or '2 semana(s)'."
  - "The button's template carries a word (en '🕒 Warn expiry: {state}') rather than the bare '🕒 {state}', because 🕒 is also the Timezone page's icon and a bare 'Off' button says nothing."
  - "The expiry screen draws the expiry line with the stored value in its body (after the intro), which is how a stored value that is no preset is shown."
  - "warnSettingsBadExpiry maps to common_callback_invalid_request: it is unreachable from a valid press (nx's Validate accepts presets only), so no new locale key."
  - "No Main state change: Main keeps 'Limit {n}, {action}', so settingsMainWorstState needs no extension."
metrics:
  duration: ~45 min
  completed: 2026-11-22
estimate:
  tokens: 100000
  tasks: 2
actuals:
  tokens: 15800
  tasks: 2
  commits: 2
plan_head_before: 131d55b38de3478a0d03afbcfd6d7f66ea8e0a30
plan_head_after: ccb4aaf
---

# Phase 26 Plan 07: Warn expiry on the Warnings page Summary

The owner can now set warn expiry from the Warnings page. The page shows an expiry line and a full-width button,
`🕒 Warn expiry: Off` or the stored value. The button (`ne`) opens an expiry screen with the presets Off, 1 d, 3 d, 1 w,
2 w, 30 d and 90 d in rows `[2 2 2 1 2]`. A preset (`nx`) stores through the new `applyWarnExpiry`, which writes through
`warns.SetWarnExpiryContext`, and redraws the page. `/warns` and the warn limit follow at once. Values use
`punishDurationLabel`, the same form as `/warnings`.

## Tasks

| Task | Name | Commit |
|------|------|--------|
| 1 | applyWarnExpiry and the expiry control on the Warnings page (with the AGENTS.md page and permissions text) | 118bee0 |
| 2 | the Warnings screens fit (callback bytes and 8 languages), and the AGENTS.md test families | ccb4aaf |

## What was built

- **Apply** (`warns.go`):
  - `applyWarnExpiry(ctx, chatID, days)` is the expiry's only write path. It sits beside `applyWarnMode`.
  - A value outside 0 to `models.WarnExpiryMaxDays` returns `warnSettingsBadExpiry` and writes nothing.
  - A failed write returns `warnSettingsSaveFailed` (`common_settings_save_failed`). The page logs it by chat ID.
  - On success it returns `Stored` from `warnReadBack`.
  - The seam is `warnSettingsWrites.Expiry`, default `warns.SetWarnExpiryContext`. That function drops `warn_settings:<chat>`.
- **Page** (`settings_warns.go`):
  - The body gets the expiry line after the action lines (and after the inert line when there is one).
  - The keyboard gets the expiry button on its own row before Back and Close.
  - `ne` takes no field and needs `settingsRightAdmin`. It draws the pure `renderSettingsWarnsExpiry`.
  - `nx` takes field `v`. `settingsWarnsExpiryPreset` validates it: the canonical decimal of a preset, so `007`, `2`, `366`,
    `-1` and empty are refused in step 1.
  - `nx` needs `settingsRightAdminFloodBot`. It calls `applyWarnExpiry` and redraws through `settingsWarnsRedraw`.
  - The screen marks the stored preset. A stored value that is no preset shows on the line and marks nothing.
  - Back goes to `a=op&p=n`.
  - The hint says three things: an expired warn stops counting at once, it is dropped at the member's next warn change, and
    raising the expiry later does not bring it back.
- **Limits:**
  - The page's largest button stays 48 bytes in a private menu and 44 in a group menu.
  - The expiry screen's largest is the 90-day preset (`a=nx&c=<14-char chat>&v=90`): 38 bytes private, 34 in a group menu.

## Deviations from Plan

### Discretion / auto-adjusted

**1. The expiry button's ceiling is 40 code points (full-width), not 20**
- The plan's label group asked for the button with its state to be at most 20. The button is alone on its row, and the
  repository's full-width buttons use the 40 ceiling (`accessToggle`).
- With a word in the label, 20 is impossible in French ("🕒 Expiration : 2 semaine(s)" is 28) and Spanish.
- `TestSettingsLabelLengths` holds the button at 40 with each state (Off, 90 days, 2 weeks) and every preset cell at 20
  with the check mark. `TestSettingsWarnsPageFitsEveryLocale` holds half-width cells at 20 and the single-button row at 40.

**2. The button's template has a word**
- The plan sketched `🕒 {state}`. The template is "🕒 Warn expiry: {state}" (and the equivalent in each language) for the
  two reasons in the key decisions.

**3. Task 1 carries the width edits of the screens tests, and AGENTS.md is split across both commits**
- The new row breaks `[5 3 2 2]` in `settings_warns_screens_test.go`. Task 1 changes those two literals so its commit stays
  green. Task 2 adds the new cases.
- Under the repository rule, the Warnings page bullet, the Permissions sentence and the Subsystem traps sentence go in Task 1.
  The Testing families go in Task 2.

**4. `/warns` is checked through its own handler with a hand-built reply message**
- The settings environment does not load the Warns module. `settingsWarnsExpiryWarns` therefore calls `warnsModule.warns`
  directly, with the administrator replying to the member, and reads the reply.

### Plan acceptance check that cannot hold as written

- The check is `git diff b7662c7 -- settings_warns_screens_test.go | grep -cE '^-\s*(if |t\.(Fatal|Error))'`. It prints 4,
  not 0.
- All four removed lines are the width literal (`[5 3 2]` at b7662c7, already changed to `[5 3 2 2]` by 26-03). Each is
  replaced by the same assertion with `[5 3 2 1 2]`. No assertion was removed or weakened.

## Earlier tests whose expected values changed (set edits for the new row, as the plan requires)

- `fuku/modules/settings_warns_test.go`:
  - `settingsWarnsStepper`'s layout check went from rows 5, 3, 2, 2 to 5, 3, 2, 1, 2.
  - In `TestSettingsWarnsOpens`, the navigation row index went from 3 to 4 and the private page's `len(keyboard)` from 4 to 5.
  - In `TestSettingsWarnsInertAction`, `len(keyboard)` went from 4 to 5.
- `fuku/modules/settings_warns_timed_test.go`: `[5 3 2 2]` became `[5 3 2 1 2]` in four places (`settingsWarnsTimedRow`
  and the "every timed state fits" subtest of `TestSettingsWarnsTimed`).
- `fuku/modules/settings_warns_screens_test.go`: `[5 3 2 2]` became `[5 3 2 1 2]` in four places
  (`TestSettingsWarnsCallbackBytes`, `TestSettingsWarnsPageFitsEveryLocale`). New cases were added. The largest-button
  value of 48/44 is unchanged.

No golden was edited or re-recorded. Every guard exits 0:
- `git diff --exit-code 92f29f1` (both 26-01 files)
- `36d2070` (lockdown)
- `e40a7be` (screening)
- `9d4554d` (punishment notices)
- `86baf1a` (`moderation_legacy_request_test.go`)
- `b7662c7` (the warns, punishment notice and blacklists goldens)

## Verification

- Task 1 verify (11 `fuku/modules` names with `-race`): ok, 8.2 s. Task 1 `fuku/i18n` names: ok.
- Task 2 verify (`TestSettingsWarnsCallbackBytes`, `TestSettingsWarnsPageFitsEveryLocale`, `TestSettingsMainPageFitsEveryLocale`,
  `TestSettingsStaffPrivacy`): ok, 25 s.
- Touched families with `-race`: ok, 38 s.
  - Names: `TestSettingsWarns.*`, `TestSettingsParityWarn.*`, `TestSettingsDriftWarn.*`, `TestSettingsAuthorityWarn.*`,
    `TestSettingsAuthorityParityWarn.*`, `TestSettingsGateAllDoors`, `TestWarnExpiry`, `TestLegacy.*`, `TestApplyWarn.*`,
    `TestSettingsPhase18.*`, `TestSettingsMain.*`, `TestSettingsStaffPrivacy`, `TestSettingsControlActionAllowList`,
    `TestSettingsDuration.*`, `TestSettingsAntiflood.*`.
  - `go test -race ./fuku/i18n`: ok, 57 s.
- Make targets and build:
  - `make check-translations`: exit 0 (0.4 s).
  - `make check-docs`: exit 0 (1.5 s).
  - `make test-shard-check`: exit 0 (28 s).
  - `CGO_ENABLED=0 go build ./...`: ok.
- golangci-lint v2.13.1:
  - `run --new-from-rev=origin/main ./...`: 0 issues (50 s).
  - The same with `--build-tags=notesttools`: 0 issues (24 s).
  - `run --new=false ./fuku/modules/... ./fuku/i18n/...`: 110 findings at HEAD and 110 at 131d55b (scratch worktree),
    identical once positions are stripped. There are none in the touched files.
- gosec (`/root/go/bin/gosec -quiet ./...`, 17.6 s): 19 issues, the baseline. The one in a touched file is the existing G115
  at `warns.go:42`.
- **Full `go test -tags testtools -race -count=1 ./fuku/modules` (without `-v`): ok, 394.7 s for the package (6 m 38 s for the
  command), with no FAIL, SKIP or panic.**
- Test time this plan adds to `fuku/modules`: about 1.0 s, within the 2 s budget.
  - The four new tests take 0.63 s.
  - `TestSettingsWarnsPageFitsEveryLocale` grew 0.37 s (1.59 s to 1.96 s).
  - `TestSettingsWarnsCallbackBytes` changes by a negligible amount.

## Known Stubs

None. Every new identifier has a production caller. `applyWarnExpiry` is called by `settingsWarnsSetExpiry`, and
`SetWarnExpiryContext` now has its production caller through the seam.

## Threat Flags

None beyond the plan's register:
- T-26-16: a forged `nx` is refused in step 1 (`TestSettingsWarnsExpiryForged`). The apply function's bounds and the CHECK
  stand behind it.
- T-26-17: `settingsRightAdminFloodBot` is judged on the live member record, with the bot check and the owner gate
  (`TestSettingsAuthorityWarnExpiry`).
- T-26-SC: no dependency added.

## Notes for plan 26-08 and later

- The Warnings page is now `[5 3 2 1 2]`. A later row there (the escalation ladder lives on its own page per D-31) shifts the
  navigation row index again. `settingsWarnsStepper` and `TestSettingsWarnsOpens` read rows by index.
- Codes used on page `n`: `st` controls `l` and `m`, and page actions `nd`, `nt`, `ne`, `nx`.
- `settingsWarnsExpiryLabel(tr, days)` is the expiry word for any later screen or Main line. If Main ever shows it, extend
  `settingsMainWorstState`.
- A full-width button on a settings page is held to 40 code points. Half-width cells are held to 20.

## Self-Check: PASSED

- Files: `fuku/modules/settings_warns_expiry_test.go` exists. `applyWarnExpiry(` is in `warns.go` (signature count 1) and in
  `settings_warns.go` (count 1). `"ne"` and `"nx"` are in `settings_warns.go`.
- Commits: 118bee0 and ccb4aaf are ancestors of HEAD.
