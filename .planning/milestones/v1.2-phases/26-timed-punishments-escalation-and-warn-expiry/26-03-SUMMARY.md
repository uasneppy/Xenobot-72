---
phase: 26-timed-punishments-escalation-and-warn-expiry
plan: 03
subsystem: settings
status: complete
tags: [timed-punishments, settings-menu, warns, event-messages]
requires:
  - 26-02 (applyWarnMode with seconds, punishDurationLabel, autoActionEffective, LoadWarnSettingsContext with WarnModeSecs)
provides:
  - settings_duration.go (settingsDurationPresets, settingsDurationPreset, renderSettingsDuration, settingsTimedWord)
  - the Warnings page's timed row and its page actions nd and nt
  - settingsWarnsActionWord(tr, mode, secs)
  - settingsMsgSample.Secs and the Messages page's {duration} and {until} previews
  - locale keys settings_opt_timed, settings_duration_intro_tmute, settings_duration_intro_tban, settings_duration_hint
affects: [26-04, 26-05, 26-10]
tech-stack:
  added: []
  patterns:
    - "one shared, pure duration screen whose presses are built by the caller (press func) and whose Back is the caller's fields"
    - "a preset press is validated in step 1 by the canonical decimal of a preset, so no forged duration reaches a lookup"
    - "a preview sample value that only a custom text reads is set after the default input is built, so the default preview stays byte for byte"
key-files:
  created:
    - fuku/modules/settings_duration.go
    - fuku/modules/settings_duration_test.go
    - fuku/modules/settings_warns_timed_test.go
  modified:
    - fuku/modules/settings_warns.go
    - fuku/modules/settings_messages.go
    - fuku/modules/settings_warns_test.go
    - fuku/modules/settings_warns_screens_test.go
    - fuku/modules/settings_main_test.go
    - fuku/modules/settings_messages_auto_test.go
    - fuku/i18n/settings_locale_test.go
    - locales/en.yml, es.yml, fr.yml, hi.yml, id.yml, pt.yml, ru.yml, uk.yml
    - AGENTS.md
decisions:
  - "The timed row's cells press nd with k m or b; nd draws the shared duration screen with the stored preset marked only while that timed mode is stored and effective; a preset presses nt, which stores through applyWarnMode(ctx, p.group.Id, mode, secs) and redraws the Warnings page."
  - "nd has right settingsRightAdmin (it writes nothing); nt has settingsRightAdminFloodBot, the right of /setwarnmode."
  - "The page's inert line shows only while autoActionEffective(mode, secs) is false; an effective timed mode marks its timed cell and the choice row marks nothing."
  - "The default private copy preview of the four events with a sample duration drops the sample end, so it shows no Until line for the sample's untimed mute (Rule 1, below)."
  - "The AGENTS.md Warnings page, duration screen, Permissions and Testing sentences went into Task 2's commit, the commit whose change they describe (D-48); the Messages sentences into Task 3's."
metrics:
  duration: ~70 min
  completed: 2026-11-22
actuals:
  tokens: 17300
  tasks: 3
  commits: 3
plan_head_before: 80e05a1f54515e6adfab5ba6bccafe3b07ba39b3
plan_head_after: 7791af9de4d99f67dc8172c941880a1c5cd5267c
---

# Phase 26 Plan 03: The timed warn limit on its page, and the shared duration screen Summary

The Warnings page has a `[Timed mute | Timed ban]` row: a cell opens the shared duration screen (eight presets from 10 minutes
to 30 days, two to a row, the stored preset marked), a preset stores the timed mode through `applyWarnMode`, the function
`/setwarnmode` calls, and the page comes back reading "Timed mute, 1 hour(s)" with that cell marked; Main reads the same word.
The Messages page's previews of a custom warn limit, flood, blacklist or forward blocklist text fill `{duration}` in the group's
language and `{until}` one hour ahead, and the Automatic family's text prompt explains both. Every golden passes unchanged.

## Tasks

| Task | Name | Commit |
|------|------|--------|
| 1 | the shared duration screen | 09a155b |
| 2 | Timed mute and Timed ban on the Warnings page, with parity, drift and authority | d85fd63 |
| 3 | the Messages page previews and prompt for {duration} and {until} | 7791af9 |

## What later plans use

- `renderSettingsDuration(v, titleKey, action, selected, press, backFields, notice)`: pure; `press(secs)` returns the preset's
  callback fields (the caller's action code and fields), `backFields` the caller's Back. `selected` 0 or a value that is no preset
  marks nothing. Rows `[2 2 2 2 2]`. The largest button is the 30-day preset: 47 bytes private, 43 group with the Warnings fields
  (`a=nt&k=m&v=2592000`); a caller with a longer field set (the Blacklist's `c` plus `k` plus `v`) must check its own worst case.
- `settingsDurationPreset(v)` is the step-1 check of every preset press; `settingsDurationPresets` the table.
- `settingsTimedWord(tr, action, secs)`: ok only for an effective `tmute` or `tban`; callers keep their own word otherwise.
- Locale keys `settings_opt_timed` (`{action}, {duration}`), `settings_duration_intro_tmute`, `_tban` and `settings_duration_hint`
  in all 8 files, with limit rows; the timed cells and the duration cells have a group in `TestSettingsLabelLengths`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] The default private copy preview would have gained an Until line**
- **Found during:** Task 3.
- **Issue:** `settingsMsgPreviewReply` is also the input of the private copy preview (`settingsMsgPMPreview`), and
  `eventPMDefaultText` appends `eventmsg_pm_until` whenever the spec has `UntilLine` and `{until}` is set. With `Sample.Secs` the
  default private copy preview of the warn limit, flood, blacklist and forward blocklist would have shown an Until line for the
  sample's untimed mute, so the default preview would no longer have been what the site sends.
- **Fix:** `settingsMsgPMPreview` clears the sample duration and end when no private text is stored (`st.Text == ""`), for an
  event with `Sample.Secs` that is not `Timed`. A custom private text still fills both. The `timed sample` subtest checks the
  flood's default private copy preview carries no end (a mutation that removes the clear fails it).
- **Files modified:** `fuku/modules/settings_messages.go`, `fuku/modules/settings_messages_auto_test.go`
- **Commit:** 7791af9

### Plan inconsistencies resolved

**2. The parity and authority tests use 6 hours, not 2 hours**
- The plan's `TestSettingsParityWarnTimed` and `TestSettingsAuthorityParityWarnTimed` press `nt` with `v` 7200 against
  `/setwarnmode tmute 2h`, but 7200 is not one of the plan's own presets (`600, 3600, 21600, 43200, 86400, 259200, 604800, 2592000`),
  so the press is (correctly) an invalid request in step 1. The tests use `v` 21600 against `/setwarnmode tmute 6h`; `tban` 86400
  against `/setwarnmode tban 1d` is as planned.

**3. The plan's assertion-deletion grep prints 10, not 0**
- `git diff b7662c7 -- settings_warns_test.go settings_warns_screens_test.go | grep -cE '^-\s*(if |t\.(Fatal|Error))'` counts 10
  lines. Each is a line whose only change is a width literal (the plan's "take the new row as a set edit"), with its replacement
  line added right after it: no assertion was deleted. The edits:
  - `settingsWarnsStepper`: `len(keyboard) != 3 ... len(keyboard[2]) != 2` became `!= 4 ... len(keyboard[2]) != 2 || len(keyboard[3]) != 2`,
    and its message "rows of 5, 3 and 2" became "rows of 5, 3, 2 and 2" (stricter: one more row checked);
  - `TestSettingsWarnsOpens` group menu: the navigation row check reads `keyboard[3]` instead of `keyboard[2]` (index 2 is now the
    timed row, which would have passed the old check trivially);
  - `TestSettingsWarnsOpens` private menu and `TestSettingsWarnsInertAction`: `len(keyboard) != 3` became `!= 4`;
  - `TestSettingsWarnsCallbackBytes` and `TestSettingsWarnsPageFitsEveryLocale`: `"[5 3 2]"` became `"[5 3 2 2]"` (condition and message).

### Additions beyond the plan

- `TestSettingsWarnsTimedForged` has eleven cases (the plan's seven plus `nt v 0600`, `nt` with no `v`, `nd` with no `k` and `nd` with an
  extra key). `TestSettingsWarnsTimed` also checks Timed ban's screen marks nothing while a timed mute is stored, the real English
  words ("Timed mute, 1 hour(s)") and the two longest timed states in 8 languages.
- `TestSettingsDurationScreen` asserts the measured largest button (47 private, 43 group).

## Earlier tests whose expected values or code changed

- `settings_warns_test.go` and `settings_warns_screens_test.go`: the width edits listed in deviation 3, nothing else.
- `settings_main_test.go` (`settingsMainWorstState`): the call takes the new `secs` argument (0 for the stored modes) and the
  candidates gain the timed word at limit 100 for every preset, `punishMaxSecs` and 5400.
- `fuku/i18n/settings_locale_test.go`: four limit rows, the timed and duration cell group, and `<code>{duration}</code>` in
  `settingsMsgAutoLiteralTokens` (the `{until}` comment extended).
- `settings_messages_auto_test.go`: a new `timed sample` subtest only.

No golden changed: `git diff --exit-code` exits 0 against 92f29f1 (`timed_punishment_legacy_request_test.go`,
`punish_export_legacy_test.go`), 36d2070 (`lockdown_message_legacy_request_test.go`), e40a7be (`screening_notice_legacy_request_test.go`),
9d4554d (`punishment_notice_legacy_request_test.go`), 86baf1a (`moderation_legacy_request_test.go`) and b7662c7
(`warns_legacy_request_test.go`, `go.mod`, `go.sum`).

## Verification

- Task 1 verify: `TestSettingsDurationScreen` ok (0.12 s); `TestSettingsLocaleKeys`, `TestSettingsLabelLengths`, `TestLocaleStyle`,
  `TestLocalePlaceholdersMatchOtherLanguages` ok.
- Task 2 verify, with `-race`: all 19 names PASS (9.7 s for the package); the five new families 0.39 s together.
- Task 3 verify, with `-race`: `TestSettingsMessagesAutoFamily` (with `timed sample`), `TestSettingsMessagesPreview`,
  `TestSettingsMessagesPageFitsEveryLocale`, `TestSettingsMessagesAutoPM` ok (14.0 s); `TestSettingsEditLocaleTokens`,
  `TestSettingsLocaleKeys`, `TestLocaleStyle`, `TestLocalePlaceholdersMatchOtherLanguages` ok.
- Acceptance: `func renderSettingsDuration(` 1; `settings_duration_hint` in 8 files; `"nd"|"nt"` 2; `applyWarnMode(ctx, p.group.Id` 2;
  `Secs: 3600` 4; `{duration}` in `settings_msg_wait_text_auto` in 8 files; the deletion grep, see deviation 3.
- Touched families with `-race` (`TestSettingsDuration|TestSettingsWarns|TestSettingsParityWarn|TestSettingsDriftWarn|
  TestSettingsAuthorityParityWarn|TestSettingsMessages|TestSettingsDriftMessages|TestSettingsAuthorityMessages|TestSettingsMain|
  TestSettingsStaffPrivacy|TestEventMessage|TestEventPM|TestEventActionWords|TestAutoEventDefaults|TestScreeningEventDefaults|
  TestLegacy|TestSettingsControlActionAllowList|TestSettingsGateAllDoors|TestSettingsPhase18|TestWarnLimitTimed|
  TestApplyWarnModeTimed|TestSetWarnModeTimedCommand|TestAutoMessagesEndToEnd|TestMessagesEndToEnd|TestPMCopiesEndToEnd`): ok,
  59.8 s; `go test -race ./fuku/i18n` ok, 54.9 s.
- `make check-translations` exit 0 (0.4 s); `make check-docs` exit 0, no drift (1.5 s); `make test-shard-check` exit 0 (2.5 s);
  `CGO_ENABLED=0 go build ./...` ok (2.5 s); `gofmt -l` prints none of the touched files.
- golangci-lint v2.13.1: `run --new-from-rev=origin/main ./...` 0 issues (47 s); with `--build-tags=notesttools` 0 issues (26 s);
  `run --new=false --max-issues-per-linter=0 --max-same-issues=0 ./fuku/modules/... ./fuku/i18n/...` 110 issues at HEAD and 110 at
  80e05a1 (a scratch worktree), identical once line numbers are stripped; none in a touched file.
- gosec (`/root/go/bin/gosec -quiet ./...`, 17.7 s): 19 issues, the baseline set (`warns.go` keeps its one G115); none in a touched file.
- No PostgreSQL guarantee was added (no migration, CHECK or conditional update), so `make test-postgres-integrity` was not run.
- **Full `go test -tags testtools -race -count=1 ./fuku/modules`: ok, 373.6 s for the package (6 min 17 s for the command), no
  FAIL, SKIP or panic.** 26-02 measured 374.4 s, so the change is inside runner noise.
- Test time this plan adds to `fuku/modules`: about 0.6 s (`TestSettingsDurationScreen` 0.12 s, the five Warnings families
  0.39 s with `-race`, `timed sample` 0.04 s), within its 3 s budget.

## Notes for plan 26-04

- Reuse `renderSettingsDuration` with the Antiflood page's own `press` (`{"a": "ft", "k": k, "v": secs}`) and Back
  `{"a": "op", "p": "f"}`; mark the stored preset only while the stored action is that timed mode and effective
  (`autoActionEffective`), as `settingsWarnsOpenDuration` does.
- Validate `ft` in step 1 with `settingsDurationPreset` and a `k` check; the forged-press test pattern is
  `TestSettingsWarnsTimedForged` (`settingsMenuWantRefused(..., "common_callback_invalid_request", true)`).
- Use `settingsTimedWord` for the Antiflood action word of an effective timed action and keep `settingsFloodActionWord` otherwise;
  `settingsFloodActionWord` is shared with the Warnings page, so do not change what it returns for `tmute` and `tban` (the inert
  Warnings states read it).
- A Blacklist preset press carries the owner field `c` plus `k` and `v`; its 30-day button must be measured (the Warnings one is 47
  bytes private).
- The Messages page already fills `{duration}` and `{until}` for the flood, blacklist and forward blocklist previews (`Sample.Secs`
  3600); the default private copy preview of those events drops the sample end (deviation 1), so a later change to that preview
  must keep it.
- `settingsMainWorstState` now includes the Warnings timed word; add the Antiflood timed word the same way if Main's Antiflood line
  shows it.

## Known Stubs

None. `renderSettingsDuration`, `settingsDurationPreset` and `settingsTimedWord` have production callers in `settings_warns.go`.

## Threat Flags

None beyond the plan's register. T-26-05: `nt` and `nd` are validated in step 1 (`settingsWarnsTimedFields`,
`settingsWarnsDurationFields`), held by `TestSettingsWarnsTimedForged` (no lookup, nothing written). T-26-06: `nt` declares
`settingsRightAdminFloodBot`, judged on the press's live member record after the owner gate, held by
`TestSettingsAuthorityParityWarnTimed`. T-26-SC: no dependency added.

## Self-Check: PASSED

- Files: settings_duration.go, settings_duration_test.go and settings_warns_timed_test.go exist.
- Commits: 09a155b, d85fd63 and 7791af9 are ancestors of HEAD.
