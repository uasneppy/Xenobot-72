---
phase: 26-timed-punishments-escalation-and-warn-expiry
plan: 04
subsystem: antiflood
status: complete
tags: [timed-punishments, antiflood, settings-menu, event-messages, private-copies]
requires:
  - 26-02 (action_secs column, punish_timed.go core, eventPMSpec.UntilLine for the flood, eventActionWords tmute and tban)
  - 26-03 (renderSettingsDuration, settingsDurationPreset, settingsTimedWord)
provides:
  - antiflood.SetFloodActionContext(ctx, chatID, mode, secs); SetFloodModeContext and SetFloodMode as its seconds-0 wrappers
  - GetAntifloodSettingsContext reads action_secs
  - applyFloodMode(ctx, chatID, mode, secs) with floodBadDuration; floodTimedModes; floodModeArgs; floodShownAction
  - floodPunish and floodPunishTimed (the one place the flood's action is decided); floodUnmuteRows
  - floodNotice(b, chat, msg, tr, sender, p autoPunishment, limit, count, rows)
  - floodTimedKeys and the timed branch of floodDefaultReply
  - the Antiflood page's timed row and its page actions fd and ft
  - settingsTimedDurationScreen (shared by the Warnings and Antiflood pages)
  - locale keys antiflood_setfloodmode_bad_duration, antiflood_checkflood_timed_mute, antiflood_checkflood_timed_ban; antiflood_help_msg explains the timed form
affects: [26-05, 26-06, 26-09]
tech-stack:
  added: []
  patterns:
    - "an automatic rule's action switch lives in one helper (floodPunish) whose untimed branches are the old switch moved verbatim and whose timed branch calls applyTimedPunishment"
    - "the restrict-right check of an automatic rule is autoActionEffective, so warn and an inert timed action keep their old path"
    - "a page's timed row draws its duration screen through settingsTimedDurationScreen with its own action code and page key"
key-files:
  created:
    - fuku/db/antiflood/timed_action_test.go
    - fuku/modules/antiflood_timed_test.go
    - fuku/modules/settings_antiflood_timed_test.go
  modified:
    - fuku/db/antiflood/optimized.go
    - fuku/db/antiflood/repository.go
    - fuku/modules/antiflood.go
    - fuku/modules/event_message_auto.go
    - fuku/modules/settings_antiflood.go
    - fuku/modules/settings_duration.go
    - fuku/modules/settings_warns.go
    - fuku/modules/antiflood_apply_test.go
    - fuku/modules/settings_antiflood_test.go
    - locales/en.yml, es.yml, fr.yml, hi.yml, id.yml, pt.yml, ru.yml, uk.yml
    - AGENTS.md
decisions:
  - "floodPunish dispatches an effective timed action to floodPunishTimed first; the mute, kick and ban branches are the old switch moved unchanged (the same requests, rows and sender-chat handling); any other action applies nothing and posts the inert notice."
  - "A sender chat under a timed action gets nothing and no notice (D-13): no request, no sendMessage, no queue row."
  - "An end date refused locally (errPunishTimeRanOut) punishes nobody and posts today's inert notice (the punishment is returned with seconds 0, so the default reads the empty word); the helper's hourly-gated Warn names the chat."
  - "floodNotice treats a punishment as timed only when its Until is set (it was applied), and only then fills {duration} and {until} and hands Secs to floodDefaultReply."
  - "/flood shows an effective timed action as 'muted for 30m' or 'banned for 1d' (floodShownAction, the untranslated word plus punishDurationShort); warn and an inert timed action keep the empty word the golden pins."
  - "/setfloodmode follows /setwarnmode's grammar (floodModeArgs): the first word lower-cased, a timed word alone keeps the pinned unknown reply, a timed word with an invalid second word is the bad-duration reply, a plain word ignores what follows."
  - "settingsFloodSettable(action, secs) is autoActionEffective; settingsFloodActionWord is unchanged because the Warnings page shares it, and the page's own word goes through settingsFloodPageActionWord."
  - "The two duration-screen openers shared 25 identical lines (golangci-lint dupl); both now call settingsTimedDurationScreen in settings_duration.go."
metrics:
  duration: ~75 min
  completed: 2026-11-22
actuals:
  tokens: 19800
  tasks: 3
  commits: 4
plan_head_before: 6556eb9ed5541a8f437f94f1ae7ee32689555f4a
plan_head_after: 72dca4c
---

# Phase 26 Plan 04: The timed flood action Summary

`/setfloodmode tmute 30m` (or `tban 1d`), or the Antiflood page's Timed mute or Timed ban row and a preset, stores the flood's
action with its duration in `antiflood_settings.action_secs` through one apply function. A member who floods is then muted with
one `restrictChatMember` (`MutedPermissions`, `until_date` the punishment time plus the duration) or banned with one
`banChatMember` with that end date; the notice names the duration and keeps the Unmute row of a mute, a custom flood text fills
`{duration}` and `{until}`, and the flood's private copy adds its Until line. A sender chat under a timed action gets nothing, a
timed action stored with no duration stays inert, and every untimed flood and every pinned command reply is byte for byte as
before. `checkFlood`'s cyclomatic complexity went from 31 to 21.

## Tasks

| Task | Name | Commit |
|------|------|--------|
| 1 | the antiflood repository, applyFloodMode with seconds and the command forms | 310d59f |
| 2 | the flood's timed punishment and notice | 06811d6 |
| 3 | Timed mute and Timed ban on the Antiflood page | bea6028 |
| - | one helper for both timed rows' duration screens (lint: dupl) | 72dca4c |

## What later plans use

- **Repository**: `antiflood.SetFloodActionContext(ctx, chat, mode, secs)` writes only `action` and `action_secs` (a missing row
  gets the model's limit 5, D-27), is a no-op when both match the cached settings, and drops `antiflood:<chat>`.
- **Apply**: `applyFloodMode(ctx, chatID, mode, secs)`; outcomes `floodApplied`, `floodUnknownMode` (also a timed action with 0),
  `floodBadDuration`, `floodSaveFailed`. `floodTimedModes` is `tban`, `tmute`.
- **Flood path** (plan 26-09's ladder goes here): `floodPunish(b, chat, tr, user, p autoPunishment) (rows, applied, notice, err)`;
  `checkFlood` builds `autoPunishment{Action: flood.Action, Secs: flood.ActionSecs}` and checks the restrict right with
  `autoActionEffective`. `floodNotice` takes the punishment as applied and the limit.
- **Page**: `fd` (fields `k`, `settingsRightAdmin`) and `ft` (fields `k`, `v`, `settingsRightAdminFloodBot`); the timed row is
  `settingsFloodTimedRow(k, timed)`, the duration screen `settingsTimedDurationScreen(v, page, titleKey, act, k, mode,
  storedAction, storedSecs)`. A Blacklist action screen can reuse that helper; its press carries the owner field `c` plus `k` and
  `v`, so its 30-day preset must be measured (the Warnings and Antiflood ones are 47 bytes private, 43 group).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - lint] The two duration-screen openers were duplicates**
- **Found during:** the `--new-from-rev=origin/main` lint gate after Task 3.
- **Issue:** `settingsFloodOpenDuration` repeated 25 lines of 26-03's `settingsWarnsOpenDuration` (dupl).
- **Fix:** `settingsTimedDurationScreen` in `settings_duration.go` marks the stored preset and builds the presses and Back; both
  openers call it. No behaviour changed (the Warnings and Antiflood families pass unedited).
- **Files modified:** `fuku/modules/settings_duration.go`, `settings_warns.go`, `settings_antiflood.go`
- **Commit:** 72dca4c

### Plan inconsistencies resolved

**2. The parity test compares the timed mute at 10 minutes, not 30**
- The plan's `TestSettingsParityFloodTimed` presses `ft` with `v` 1800 against `/setfloodmode tmute 30m`, but 1800 is not a preset,
  so that press is (correctly) an invalid request in step 1 (`TestSettingsAntifloodTimedForged` holds `ft v 1800` refused). The
  test compares `v` 600 with `/setfloodmode tmute 10m`, and `tban` 86400 with `/setfloodmode tban 1d` as planned. The same
  inconsistency was met in 26-03.

**3. The plan's assertion-deletion grep prints 4, not 0**
- `git diff b7662c7 -- fuku/modules/settings_antiflood_test.go | grep -cE '^-\s*(if |t\.(Fatal|Error))'` counts 4 lines: the two
  `if len(keyboard) != 4 {` checks and their `t.Fatalf` messages, each replaced right after by the same check at 5 rows (the
  timed row). No assertion was deleted. See the list below.

**4. The flood's timed punishment with an end date refused locally**
- The plan does not say what the flood does when `applyTimedPunishment` returns `errPunishTimeRanOut` (only possible for a
  duration under 35 s, which no stored duration can be). It follows the warn limit's rule: nobody is punished and today's inert
  notice is posted (no row, no duration). Documented in `floodPunishTimed` and AGENTS.md.

**5. No `fuku/i18n/settings_locale_test.go` change**
- The plan lists the file, but the page adds no `settings_` key and no new label: the timed cells reuse `settings_opt_tmute` and
  `settings_opt_tban`, which 26-03's label group already covers, and the page's bad-duration notice reuses the command's
  `antiflood_setfloodmode_bad_duration`, which no limits table covers (the `antiflood_` keys have none).

**6. AGENTS.md sentences went into the commit whose change they describe**
- The event messages flood site sentences, the 30 s client bound and the callers list went into Task 2's commit (06811d6); the
  page, Permissions, Data, first-write and Testing sentences into Task 3's (bea6028). Task 1's commit invalidated no sentence.

### Additions beyond the plan

- `TestSetFloodActionDuration` also checks a seconds-only change writes and that a default mute write on a chat with no row
  creates nothing; `TestApplyFloodModeTimed` checks a plain action ignores the seconds it is handed (kick with 3600 stores 0).
- `TestSettingsAntifloodTimed` also checks the stored preset is marked on its own screen and not on the other action's, that a
  plain choice clears the seconds, and the two longest timed states in 8 languages; `TestSettingsAntifloodTimedForged` has
  eleven cases.
- `TestSettingsAntifloodCallbackBytes` gained the timed states and the duration screen (largest button 47 private, 43 group);
  `TestSettingsAntifloodPageFitsEveryLocale` gained two timed states and the bad-duration notice.

## Earlier tests whose expected values or code changed

- `settings_antiflood_test.go`:
  - `settingsAntifloodStepper`: `len(keyboard) != 4` became `!= 5`, and its message names the timed row;
  - `TestSettingsAntifloodPage` (limit 7, action ban): `len(keyboard) != 4` became `!= 5` (condition and message), the toggle is
    read from `keyboard[3]` (was 2) and the navigation row from `keyboard[4]` (was 3);
  - `TestSettingsAntifloodCallbackBytes` and `TestSettingsAntifloodPageFitsEveryLocale`: additions only (above).
- `antiflood_apply_test.go`: every `applyFloodMode` call passes seconds 0; no assertion changed.

No golden changed: `git diff --exit-code` exits 0 against 92f29f1 (`timed_punishment_legacy_request_test.go`,
`punish_export_legacy_test.go`), 36d2070 (`lockdown_message_legacy_request_test.go`), e40a7be
(`screening_notice_legacy_request_test.go`), 9d4554d and b7662c7 (`punishment_notice_legacy_request_test.go`), 86baf1a
(`moderation_legacy_request_test.go`) and b7662c7 (`warns_legacy_request_test.go`, `go.mod`, `go.sum`). There is no Phase 18
golden of the flood commands; `/setfloodmode` and `/flood` are pinned by the 26-01 golden.

## Verification

- Task 1 verify: `TestSetFloodActionDuration`, `TestSetFloodModeOnFreshChatStoredLimit`, `TestSetFloodMsgDelOnFreshChatStoredLimit`
  ok (1.1 s); `TestApplyFloodModeTimed`, `TestSetFloodModeTimedCommand`, `TestApplyFloodMode`, `TestAntifloodCommandsUseApply`,
  `TestAntifloodCommandsUpdateSettingsAndDisplay`, `TestLegacyTimedPunishmentRequests` ok (3.5 s).
- Task 2 verify: the eleven names plus `TestEventPMAutoEvents`, `TestAntiflood.*`, `TestAutoMessagesEndToEnd`, `TestPMCopyCallers`
  ok (6.8 s); gocyclo on `antiflood.go`: only `checkFlood` at 21 (was 31); `floodPunish` is not reported.
- Task 3 verify: the eighteen names ok (9.6 s); `TestSettingsLocaleKeys`, `TestSettingsLabelLengths` ok.
- Acceptance: `action_secs` in `optimized.go` 1; `func SetFloodMode(chatID int64, mode string) error` 1; the `applyFloodMode`
  signature 1; `floodPunish(` 2; `applyTimedPunishment(` 1; `applyFloodMode(ctx, p.group.Id` 2; the deletion grep, deviation 3.
- Touched families with `-race` (`TestLegacy|TestAntiflood|TestApplyFlood|TestSetFloodMode|TestFloodTimed|TestUpdateFlood|
  TestSettingsAntiflood|TestSettingsParityAntiflood|TestSettingsDriftAntiflood|TestSettingsAuthorityParityAntiflood|
  TestSettingsParityFloodTimed|TestSettingsDriftFloodTimed|TestSettingsAuthorityParityFloodTimed|TestSettingsDuration|
  TestSettingsWarns|TestSettingsMain|TestSettingsStaffPrivacy|TestSettingsControlActionAllowList|TestSettingsGateAllDoors|
  TestSettingsPhase18|TestSettingsPhase19|TestEventMessage|TestEventPM|TestEventActionWords|TestAutoEventDefaults|
  TestScreeningEventDefaults|TestAutoMessagesEndToEnd|TestPMCopyCallers|TestSettingsMessages|TestWarnLimitTimed|TestPunishTimed`):
  ok, 60.8 s (63.3 s for the command); `go test -race ./fuku/db/antiflood ./fuku/i18n` ok, 1.1 s and 53.1 s. After the refactor
  commit the Warnings, Antiflood and duration families were run again with `-race`: ok, 6.2 s.
- `make check-translations` exit 0 (0.4 s); `make check-docs` exit 0, no drift (1.5 s; the help text is not in a generated page);
  `make test-shard-check` exit 0 (25.7 s); `CGO_ENABLED=0 go build ./...` ok (6.9 s); `gofmt -l` prints none of the touched files.
- golangci-lint v2.13.1: `run --new-from-rev=origin/main ./...` 0 issues (44 s; 2 dupl before 72dca4c); with
  `--build-tags=notesttools` 0 issues (22 s); `run --new=false --max-issues-per-linter=0 --max-same-issues=0 ./fuku/modules/...
  ./fuku/db/antiflood/... ./fuku/i18n/...` 110 issues at HEAD and 110 at 6556eb9 (a scratch worktree), identical once positions
  are stripped except `checkFlood`'s gocyclo, 31 at the base and 21 at HEAD; no other finding in a touched file.
- gosec (`/root/go/bin/gosec -quiet ./...`, 11.6 s): 19 issues, the baseline set; none in a touched file.
- No PostgreSQL guarantee was added (no migration, CHECK or conditional update; `TestSetFloodActionDuration` rests on none), so
  `make test-postgres-integrity` was not run and the Makefile list is unchanged.
- **Full `go test -tags testtools -race -count=1 ./fuku/modules`: ok, 372.2 s for the package (6 min 15 s for the command), no FAIL, SKIP or panic.** 26-03 measured
  373.6 s, so the change is inside runner noise.
- Test time this plan adds to `fuku/modules`, measured together with `-race`: about 2.3 s (`TestSetFloodModeTimedCommand` 1.45 s,
  almost all of it the real locales' first load, which another real-locale test in the same process has already paid;
  `TestSettingsAntifloodTimed` 0.54 s, `TestFloodTimed` 0.10 s, the other five families 0.25 s together), within the 3 s budget.
  The package went from 373.6 s (26-03) to 372.2 s.

## Notes for plan 26-05

- `floodPunish` is the pattern for the blacklist: move the watcher's action switch into one helper with the untimed branches
  verbatim, an effective timed branch through `applyTimedPunishment`, and nothing for a sender chat under a timed action; check
  the restrict right with `autoActionEffective`.
- The page's timed row: `settingsTimedDurationScreen(v, page, titleKey, act, k, mode, storedAction, storedSecs)` draws the
  duration screen; the Blacklist's press carries the owner field `c`, so measure its 30-day preset (Antiflood and Warnings are 47
  bytes private with `k` and `v` only).
- `settingsFloodActionWord` is still shared with the Warnings page; the Antiflood page's own word is `settingsFloodPageActionWord`.
  The blacklist action screen has its own words (`settings_bl_*`), so use `settingsTimedWord` there for an effective timed action.
- `eventPMSpec.UntilLine` is set for `blacklist` and `fwdblock`; fill `eventValues.Duration` and `.Until` only from an applied
  punishment's end date, as `floodNotice` does (`p.Until > 0`).
- The flood's `errPunishTimeRanOut` path posts the inert notice; decide the blacklist's the same way unless the plan says otherwise.
- Main's Antiflood line shows the limit only, so `settingsMainWorstState` was not changed; a Blacklist Main line shows counts.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register. T-26-07: the end is computed inside `applyTimedPunishment` right before the request and refused
locally under 35 s (`TestFloodTimed` checks `until_date` now plus the duration). T-26-08: `ft` and `fd` are validated in step 1
(`settingsFloodTimedFields`, `settingsFloodDurationFields`), held by `TestSettingsAntifloodTimedForged`. T-26-09: `floodPunishTimed`
sends nothing to a sender chat (`sender_chat_tban`). T-26-SC: no dependency added.

## Self-Check: PASSED

- Files: fuku/db/antiflood/timed_action_test.go, fuku/modules/antiflood_timed_test.go and
  fuku/modules/settings_antiflood_timed_test.go exist.
- Commits: 310d59f, 06811d6, bea6028 and 72dca4c are ancestors of HEAD.
