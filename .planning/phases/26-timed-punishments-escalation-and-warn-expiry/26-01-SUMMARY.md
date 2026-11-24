---
phase: 26-timed-punishments-escalation-and-warn-expiry
plan: 01
subsystem: testing
status: complete
tags: [golden, characterization, antiflood, warns, blacklists, backup]
requires: []
provides:
  - TestLegacyTimedPunishmentRequests (35 subtests)
  - the goldenTimed* constants (33)
  - timedGoldenRun, timedGoldenRow, timedGoldenTokens, timedGoldenCleanup, timedGoldenCommand, timedGoldenButton, timedGoldenFailFloodWrite and the seed helpers
  - TestLegacyPunishmentExport and goldenExportWarns, goldenExportAntiflood, goldenExportBlacklists
affects: [26-02, 26-03, 26-04, 26-05, 26-06, 26-07, 26-08, 26-09, 26-10, 26-11, 26-12]
tech-stack:
  added: []
  patterns:
    - "a golden recorded by running the test once with empty constants and pasting each %q `got` value, then committed alone before any production change"
    - "each case pins only the stored fields the plan names (state), so a read's incidental row creation is not frozen"
key-files:
  created:
    - fuku/modules/timed_punishment_legacy_request_test.go
    - fuku/db/backup/punish_export_legacy_test.go
  modified: []
decisions:
  - "Every command case runs in newNoticeGoldenSite (the Phase 23 send-site environment) through the handler directly, so the command cases and the inert send cases share one runner, one cleanup and the real locales."
  - "The stored state is pinned field by field (antiflood, warns_settings, blacklist, warns of member 42), and only where the plan names it: the /setfloodmode cases, /setwarnmode tmute, the /blacklistaction cases, /rmwarn, the button, /resetwarns and the inert warn limit. /warns, /warnings and /flood pin their reply only, because today's reads create rows (a zero warns row, a default warns_settings row) that D-35 plans to stop creating."
  - "The export golden inserts the two blacklist words with their action, in word order, because SQLite reads them through the (chat, word) index: word order and insert order then agree on any database."
  - "No AGENTS.md edit: the plan leaves the Testing sentence that names these goldens to 26-02, together with the first change they guard."
metrics:
  duration: ~55 min
  completed: 2026-11-22
actuals:
  tokens: 9500
  tasks: 2
  commits: 1
plan_head_before: 386efe212b26c5ff102296bb8a60bea0b5726bc5
plan_head_after: 92f29f19ba65235276ab541d50bfefacf607ad11
---

# Phase 26 Plan 01: Timed punishment goldens Summary

`TestLegacyTimedPunishmentRequests` pins, on the code as it is before Phase 26, the exact request of `/setfloodmode` (no argument, mute,
`BAN`, kick, `xyz`, `tmute` alone, `tban` alone, a failed save, a non-admin), `/flood` (off, mute, ban, kick, a stored tmute),
`/setwarnmode tmute`, `/warnings` (default, ban, a stored tmute), `/blacklistaction` (`tmute` alone, `tban` alone, the current mode with a
stored tmute), `/warns` (none, three reasons with markup, two warns with no reasons), `/rmwarn` and the Remove warn button (removed,
nothing to remove), `/resetwarns` (reset, nothing to reset), and what the warn limit, the flood, the blacklist and a member requirement
warn do at a timed setting with no duration, in Ukrainian with the real locales. `TestLegacyPunishmentExport` pins the backup export JSON
of the warns, antiflood and blacklists modules for today's rows. Both were committed alone, in one commit (92f29f1), before any
production change of the phase.

## What later plans use

| Kind | Name |
|------|------|
| Test | `TestLegacyTimedPunishmentRequests` in `fuku/modules/timed_punishment_legacy_request_test.go`, subtests `floodmode_no_argument floodmode_mute floodmode_ban_upper floodmode_kick floodmode_unknown floodmode_tmute_alone floodmode_tban_alone floodmode_save_failed floodmode_not_admin flood_off flood_mute flood_ban flood_kick flood_stored_tmute warnmode_tmute_alone warnings_default warnings_ban warnings_stored_tmute blaction_tmute_alone blaction_tban_alone blaction_current_tmute warns_none warns_three_reasons warns_no_reasons rmwarn_removed rmwarn_nothing rmwarn_button_removed rmwarn_button_nothing resetwarns_reset resetwarns_nothing warnlimit_inert_tmute flood_inert_tmute blacklist_inert_tban blacklist_inert_tmute mreq_warnlimit_inert_tmute` |
| Constants | `goldenTimedFloodMode{NoArgument,Mute,Ban,Kick,Unknown,Tmute,Tban,SaveFailed,NotAdmin}`, `goldenTimedFlood{Off,Mute,Ban,Kick,StoredTmute}`, `goldenTimedWarnModeTmute`, `goldenTimedWarnings{Default,Ban,StoredTmute}`, `goldenTimedBlAction{Tmute,Tban,CurrentTmute}`, `goldenTimedWarns{None,ThreeReasons,NoReasons}`, `goldenTimedRmWarn{Removed,Nothing,ButtonRemoved,ButtonNothing}`, `goldenTimedResetWarns{Reset,Nothing}`, `goldenTimedWarnLimitInertTmute`, `goldenTimedFloodInertTmute`, `goldenTimedReqWarnLimitInertTmute` (request JSON with `<chat>`, `<admin>`; the requirement one with `<chat>`, `<user>`, `<hold>` through `screenGoldenTokens`) |
| Toolkit | `timedGoldenRun(t, case)` (one `newNoticeGoldenSite` per case, `timedGoldenCleanup`, `resetAntifloodState` before and after, then the punishment, deletion, request, state and queue checks), `timedGoldenRow(t, chatID)` (`antiflood=L/A/D; warns_settings=L/M; blacklist=[w:a]; warns=N[reasons]`, `none` for a missing row, read straight from the tables), `timedGoldenTokens(raw, ids)` (longest decimal first), `timedGoldenCommand`, `timedGoldenButton`, `timedGoldenFailFloodWrite`, `timedGoldenChatRow`, `timedGoldenWarnsOf`, `timedGoldenFloodRow`, `timedGoldenWarnSettings`, `timedGoldenWord` |
| Test | `TestLegacyPunishmentExport` in `fuku/db/backup/punish_export_legacy_test.go`, constants `goldenExportWarns`, `goldenExportAntiflood`, `goldenExportBlacklists` (`<chat>`, `<user>`, `<time>`), helper `punishExportTokens` |

Every case also checks that no punishment call is made but its own, and that the chat's deletion queue is empty except for the two inert
sends (one `bot:punish_manual` and one `bot:punish_auto` row at 600 s for message 9001).

## What the goldens record (byte quirks the changes must keep)

- **`/setfloodmode`**: HTML, link previews off, `reply_parameters` with `allow_sending_without_reply`. `tmute` and `tban` alone answer
  `antiflood_setfloodmode_unknown_type` naming the word, exactly as `xyz` does. `/setfloodmode mute` on a chat with no row answers the
  success text and **writes nothing** (`SetFloodModeContext` compares with the cached default `mute`); the first write of `ban` or `kick`
  stores limit 5 (D-27). `BAN` is lower-cased in the reply. A failed insert answers `common_settings_save_failed` and leaves no row. A
  non-admin gets `Доступ заборонено...` from `IsUserConnected`.
- **`/flood`**: the English word `muted`, `banned` or `kicked`; a stored `tmute` shows an empty word (`отримає дію: .`).
- **`/setwarnmode tmute`**: `warns_mode_unknown` naming `tmute`; no row written.
- **`/warnings`**: the raw stored mode in `<code>` (`tmute` for a stored tmute).
- **`/blacklistaction`**: `tmute` and `tban` alone answer `choose_correct_option` and leave the words' action; with no argument a stored
  `tmute` shows as `<b>tmute</b>`.
- **`/warns`**: no parse mode, no link preview options, `reply_parameters` with the message ID only; the reasons are spliced unescaped
  (`\n - <b>bold</b> & co`).
- **`/rmwarn`, `/resetwarns`**: HTML replies with the usual reply options; the Remove warn button edits message 102 with
  `reply_markup: {}`. Removing the last warn by the button leaves the row with `num_warns` 0 (`warns=0[]`); `/resetwarns` deletes it.
- **Warn limit at a stored `tmute`**: the reply is `\n - reason` (the empty limit text, then the reasons) with `reply_markup: {}`,
  replying to the command; no punishment, the warn is kept, queued under `bot:punish_manual`.
- **Flood at a stored `tmute`**: the message is deleted, no punishment, the notice `Виявлено флуд. <mention> — дію виконано: .` with
  `reply_markup: {}` and no reply, queued under `bot:punish_auto`.
- **Blacklist at a stored `tban` or `tmute`**: the message is deleted, nothing is sent and nothing is queued.
- **Requirement warn at a limit of 1 stored as `tmute`**: the plain warn notice (`Попередження 1 з 1.`), no punishment, one warn kept,
  queued under `bot:punish_auto` at 600 s.
- **Export**: `warn_settings` keys `_id`, `warn_limit`, `warn_mode`, `created_at`, `updated_at`; each warns row `user_id`, `chat_id`,
  `num_warns`, `warns` (the reasons, JSON-escaped), the times; antiflood `chat_id`, `limit`, `action`, `delete_antiflood_message`, the
  times; blacklists `blacklist_mode` (the first entry's action) and the entries with `reason` `Blacklisted word: '%s'`. No duration,
  expiry or warn-times key appears, so the new fields must be `omitempty`.

## Tasks

| Task | Name | Commit |
|------|------|--------|
| 1 | record the commands, the warn displays, the inert timed sends and the exports | 92f29f1 |
| 2 | commit the two goldens alone and record the gate baselines | 92f29f1 |

## Verification

- Task 1's verify: `TestLegacyTimedPunishmentRequests` ok, 35 subtests PASS, no FAIL, SKIP or "no tests to run"; the test takes 1.58 s
  alone (the first subtest loads the locale files), 0.45 s after another real-locale golden, 44 s for the command with the build.
  `TestLegacyPunishmentExport` ok, 0.02 s (4 s with the build); `-count=2` passes.
- A mutation check (a wrong state field and a changed reply word) fails both checks; the file was restored before the commit.
- Task 2's verify: all seven `fuku/modules` names and both `fuku/db/backup` names ok (3.7 s and 1.1 s, 6 s and 2 s with the build); the
  commit subject starts with `test(26-01):`.
- Acceptance: `grep -c 'func TestLegacyTimedPunishmentRequests('` 1; `grep -c 'func TestLegacyPunishmentExport('` 1; 35 PASS lines
  (at least 31); 33 `goldenTimed*` constants (at least 25); 35 `<chat>` (at least 20); the commit holds the two files; no production
  commit since b7662c7.
- `git diff --exit-code b7662c7 -- fuku/modules/antiflood.go fuku/modules/warns.go fuku/modules/blacklists.go fuku/db/warns fuku/db/antiflood fuku/db/blacklists fuku/db/backup/backup.go fuku/db/models` exits 0; the same for `go.mod go.sum`.
- The earlier goldens are untouched: `git diff --exit-code` against 36d2070 (lockdown), e40a7be (screening), 9d4554d (punishment notices)
  and 86baf1a (moderation) exits 0 for each.
- Touched families, `-race`: `^(TestLegacy|TestAntiflood|TestApplyFlood|TestSetFlood|TestWarn|TestRmWarn|TestApplyWarn|TestBlacklist|TestApplyBlacklist|TestMemberReqWarn|TestEventMessageWarn|TestEventMessageFlood|TestEventMessageBlacklist)` ok, 7.5 s (10 s with the build); `./fuku/db/backup ./fuku/db/warns ./fuku/db/antiflood ./fuku/db/blacklists` ok, 4 s.
- `CGO_ENABLED=0 go build ./...` ok (3 s); `make test-shard-check` exit 0 (23 s); `gofmt -l` prints nothing.
- golangci-lint v2.13.1: `run --new-from-rev=origin/main ./...` 0 issues (2 s, cached); the same with `--build-tags=notesttools`
  0 issues (21 s); `run --new=false ./fuku/modules/... ./fuku/db/backup/...` 116 baseline findings (gocyclo 32, staticcheck 15,
  unused 4, the rest other linters), none in the two new files (the config's `build-tags: ["testtools"]` lints them), 42 s.
- The full `go test -tags testtools -race -count=1 ./fuku/modules`: ok, 368.7 s for the package (6 min 11 s for the command), no FAIL,
  SKIP or panic.

## Baselines (for every later plan of the phase)

- **gosec** (`$(go env GOPATH)/bin/gosec -quiet ./...`, version `dev`): 19 issues in 18 files, 17 `#nosec`; `./fuku/...` gives the same
  19. Per file: `fuku/modules/staff_action_summary.go` 2 (G101); 1 each in `fuku/db/chatconv/repository.go` (G115),
  `fuku/modules/deletion_worker.go` (G118), `fuku/modules/lockdown_alert.go` (G101), `fuku/modules/settings_blacklist.go` (G115),
  `fuku/modules/settings_filters.go` (G115), `fuku/modules/settings_lockdown.go` (G115), `fuku/modules/settings_notes.go` (G115),
  `fuku/modules/settings_reactions.go` (G115), `fuku/modules/settings_staff.go` (G115), `fuku/modules/staff_link.go` (G101),
  `fuku/modules/staff_log.go` (G101), `fuku/modules/staff_recheck.go` (G101), `fuku/modules/staff_sweeper.go` (G404),
  `fuku/modules/staff_unlink.go` (G101), **`fuku/modules/warns.go:40` (G115, `warnPunishLock`'s int64 to uint64)**,
  `fuku/utils/captchaweb/limiter.go` (G101), `fuku/utils/captchaweb/siteverify.go` (G117). 17 s.
- **golangci-lint** `--new-from-rev=origin/main ./...`: 0 issues, also with `--build-tags=notesttools`. `--new=false` over
  `./fuku/modules/... ./fuku/db/backup/...`: 116.
- **Test time**: `fuku/modules` package 368.7 s; this plan adds about 1.6 s there (0.5 s once the locales are loaded) and 0.02 s in
  `fuku/db/backup` (D-45 budget: about 4 s).

## Deviations from Plan

**1. [Plan assumption] One environment type for every case**
- The plan assumed the flood commands would run on `newModuleBotClient`, `newModuleTestBot` and `newModuleMessageContext`. Every case,
  the commands included, runs in `newNoticeGoldenSite` through the handler directly (the same handlers the plan names), so one runner
  checks requests, punishments, deletions, state and queue for all of them. The not-admin case sends from member 42 in that site.

**2. [Rule 2 - test robustness] The stored state is pinned per field**
- `timedGoldenRow` returns every field, but each case compares only the fields the plan pins. `/warns` creates a zero `warns_users` row
  for a member with no warns and a default `warns_settings` row when the chat is known, and `/warnings` creates nothing for a chat the
  chats table does not know; freezing those would block D-35 (`GetWarnsAt` through `LoadWarnSettingsContext`, which creates no row),
  and the plan pins only the replies of `/warns`, `/warnings` and `/flood`.

**3. [Rule 3 - fixture] The chats row is seeded for the warns cases with no warn**
- `RemoveWarn` locks the chats row and answers `error_generic` without it, so `rmwarn_nothing`, `rmwarn_button_nothing`,
  `resetwarns_nothing`, `warns_none` and `warns_no_reasons` first run `chats.EnsureChatInDb`, as every group the bot has seen has one.

**4. [Rule 3 - fixture] The export's blacklist words are inserted directly, in word order**
- SQLite returned the words in (chat, word) index order, not insert order, so the words are created with action `kick` and the
  `/addblacklist` reason in word order (`scam`, `spam`) and never updated; the export then lists them the same way whichever order the
  database reads them in.

**5. [Plan inconsistency, harmless] The verify of Task 1 asks for at least 31 PASS lines; there are 35, one per case of the behaviour
list.**

## Not verified

- The goldens run on SQLite only (as every `fuku/modules` test does); the export golden is written to be order-stable on PostgreSQL but was
  not run there.

## Notes for plan 26-02

- Compare with these constants by name; never edit either file (guard: `git diff --exit-code 92f29f1 -- fuku/modules/timed_punishment_legacy_request_test.go fuku/db/backup/punish_export_legacy_test.go`).
- The goldens call only: `antifloodModule.setFloodMode`, `antifloodModule.flood`, `warnsModule.setWarnMode`, `.warnings`, `.warns`,
  `.removeWarn`, `.resetWarns`, `.rmWarnButton`, `.warnUser`, `blacklistsModule.setBlacklistAction`, the Phase 23 drivers
  `noticeGoldenWarn`, `noticeGoldenFlood`, `noticeGoldenBlacklist`, the screening environment (`newScreenGoldenMreqEnv`,
  `screenGoldenReqMember`, `mreqEnforce`, `mreqNotices`, `screenGoldenWantAction`, `screenGoldenTokens`), the two-argument repository
  writes `warns.SetWarnLimit`, `warns.SetWarnMode`, `warns.WarnUser`, `antiflood.SetFlood`, `antiflood.SetFloodMode`,
  `antiflood.SetFloodMsgDel`, `blacklists.AddBlacklist`, `blacklists.SetBlacklistAction`, `chats.EnsureChatInDb`, the outcome
  `memberReqWarned`, the model fields `AntifloodSettings.{Limit,Action,DeleteAntifloodMessage}`, `WarnSettings.{WarnLimit,WarnMode}`,
  `BlacklistSettings.{Word,Action,Reason}`, `Warns.{UserId,ChatId,NumWarns,Reasons}` and `backup.ExportModuleData`. Keep those, or the
  goldens stop compiling.
- `floodmode_mute` pins that `/setfloodmode mute` on a chat with no row writes no row; a new `applyFloodMode(ctx, chat, mode, secs)` that
  always upserts would break it. Keep the same-value short-circuit for an untimed mode.
- `floodmode_ban_upper` and `floodmode_kick` pin `antiflood=5/<mode>/false` (the D-27 first write); a write that also stores
  `action_secs` 0 is invisible to the golden.
- `flood_stored_tmute` and `flood_inert_tmute` pin the empty word for a stored `tmute` with no duration (`дію: .`): D-08's inert rule must
  keep it.
- `fuku/modules/warns.go` already has one gosec finding (G115 at line 40, `warnPunishLock`); "no finding in a file it touched" must be
  read as "no new finding" there.
- The AGENTS.md Testing sentence that names `TestLegacyTimedPunishmentRequests` and `TestLegacyPunishmentExport` is 26-02's, in the same
  commit as the first change they guard.

## Known Stubs

None.

## Threat Flags

None. T-26-01 is mitigated: both goldens are committed alone in 92f29f1 before any production change; later plans run them unchanged and
check `git diff --exit-code 92f29f1` on both files. T-26-SC: `go.mod` and `go.sum` unchanged.

## Self-Check: PASSED

- Files: `fuku/modules/timed_punishment_legacy_request_test.go` and `fuku/db/backup/punish_export_legacy_test.go` exist.
- Commits: 92f29f1 is an ancestor of HEAD.
