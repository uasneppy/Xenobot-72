---
phase: 26-timed-punishments-escalation-and-warn-expiry
plan: 06
subsystem: warns, backup
status: complete
tags: [warn-expiry, warns, backup, migration, timed-punishments]
requires:
  - 26-02 (warn_mode_secs, antiflood_settings.action_secs, models.PunishDurationValid, punishNow, punishDurationLabel)
  - 26-05 (blacklists.action_secs, the timed blacklist action)
provides:
  - migration 20261122130000_add_warn_expiry.sql (warns_settings.warn_expiry_days with chk_warns_settings_expiry, warns_users.warn_times)
  - models.WarnExpiryMaxDays, WarnSettings.ExpiryDays, Warns.WarnTimes, (*Warns).NormalisedTimes, (*Warns).Counted
  - warns.WarnUserAt, RemoveWarnAt, ResetUserWarnsAt, GetWarnsAt, SetWarnExpiryContext, ErrWarnExpiryOutOfRange; the old names wrap time.Now()
  - the warn commands and the warn limit counting through the dated functions on punishNow; warnsExpiryNote
  - locale keys warns_expiry_note and warns_settings_expiry (8 files); warns_help_msg (uk, en) mentions expiry
  - backup exportedWarns, sanitizeImportedWarnSettings, sanitizeImportedAntiflood, sanitizeImportedBlacklist, sanitizeImportedWarnTimes
affects:
  - 26-07 (the Warnings page's expiry screen writes through SetWarnExpiryContext)
tech-stack:
  added: []
  patterns:
    - "one pure counting rule in the model ((*Warns).Counted) that every warn reader, writer and the export share"
    - "dated repository functions (…At) with the old names kept as wrappers on time.Now()"
key-files:
  created:
    - migrations/20261122130000_add_warn_expiry.sql
    - fuku/db/migrations/warn_expiry_migration_test.go
    - fuku/db/models/warns_counted_test.go
    - fuku/db/warns/expiry_test.go
    - fuku/modules/warns_expiry_test.go
    - fuku/db/backup/punish_import_test.go
  modified:
    - fuku/db/models/warns.go
    - fuku/db/warns/repository.go
    - fuku/modules/warns.go
    - fuku/db/backup/backup.go
    - fuku/db/backup/roundtrip_test.go
    - locales/en.yml, es.yml, fr.yml, hi.yml, id.yml, pt.yml, ru.yml, uk.yml
    - Makefile
    - AGENTS.md
key-decisions:
  - "The warns export carries no warn times while the group's expiry is off (every row as stored); with expiry on it carries the counted warns, and the times only when the row stores one per warn. Times exported with expiry off would have changed the plan 26-01 export golden, whose rows now store times."
  - "warns_expiry_note and warns_settings_expiry take {duration} (punishDurationLabel of days*86400), not {days}, so the unit is declined by the existing staff_act_duration_* forms."
  - "An import keeps a duration only for a timed action (tmute, tban) with a valid duration, so a plain action keeps none; warns whose times are dropped are dated at the import (updated_at now)."
  - "ResetUserWarnsAt takes no chat lock (a missing chat answers false with no error, as ResetUserWarns did) and drops its cache keys after the commit."
  - "On the write path a row with no update time is dated now, so the far-future padding of a previous build's cached value is never stored."
metrics:
  duration: ~35 min
  completed: 2026-11-22
estimate:
  tokens: 160000
  tasks: 3
actuals:
  tokens: 29200
  tasks: 3
  commits: 3
plan_head_before: 82535381d1f0dd66d173b2233f4d22a0caf68ccb
plan_head_after: 3cf33a6f1c784afbfc4345c72585d76458132dbc
---

# Phase 26 Plan 06: Warn expiry storage and counting, and the backup Summary

Each warn now carries its unix time (`warns_users.warn_times`) and a group can store a warn expiry in days
(`warns_settings.warn_expiry_days`, 0 to 365, 0 off). One pure rule in the model, `(*models.Warns).Counted(expiryDays, now)`,
decides which warns count. `/warns`, `/rmwarn`, the Remove warn button, `/resetwarns`, the warn limit, the AI spam warn count and
the backup export all use it. Old rows are dated at their last change, so no old warn expires early. A cached value of the
previous build counts every warn. With expiry off every reply and the export stay byte for byte. A backup now carries the durations,
the expiry and the counted warns with their times, and an import stores 0 for any out-of-range value without refusing the file.

## Tasks

| Task | Name | Commit |
|------|------|--------|
| 1 | the warn time, the expiry setting and one counting rule in the repository (with the export rule that keeps the 26-01 golden) | 6c1db52 |
| 2 | the commands and the warn limit count what counts | 0270dce |
| 3 | the backup carries the counted warns, their times, the durations and the expiry | 3cf33a6 |

## What was built

- **Migration** `20261122130000_add_warn_expiry.sql`: two `ADD COLUMN IF NOT EXISTS` statements, the expiry with its
  inline named CHECK, the times as `JSONB NOT NULL DEFAULT '[]'`. There is no backfill. Both appliers run it unchanged.
- **Model**: `NormalisedTimes` pads missing times at the front with the earlier of `UpdatedAt` and the first stored time. It
  cuts extra times from the end and raises a decreasing time to the one before it. A zero `UpdatedAt` with no stored time pads
  with the far future. `Counted` returns every warn and the stored reasons with expiry off. With expiry on it returns the
  suffix newer than `now - days*24h`, with the last `min(count, len(reasons))` reasons. Both are pure.
- **Repository**: `WarnUserAt`, `RemoveWarnAt` and `ResetUserWarnsAt` run inside one transaction with `WarnUser`'s locks.
  `WarnUserAt` and `RemoveWarnAt` drop expired warns while expiry is on, then append or remove. `ResetUserWarnsAt` answers
  removed only when a warn counted. `GetWarnsAt` counts the cached row with the expiry from `LoadWarnSettingsContext`, and
  counts with 0 when that read fails. The `warns` cache value gains `Times` and `UpdatedAt`. `SetWarnExpiryContext` is a
  column-targeted upsert that refuses values out of range before any write and drops `warn_settings:<chat>`.
- **Commands**: `applyWarn` uses `WarnUserAt` and `ResetUserWarnsAt` with `punishNow()`, so the limit compares the counted number.
  `/warns` uses `GetWarnsAt`, and `/rmwarn` and the button use `RemoveWarnAt`. `/resetwarns` uses `ResetUserWarnsAt`. While expiry
  is on, `/warns` ends with the plain line `warns_expiry_note` (`warnsExpiryNote`) and `/warnings` adds `warns_settings_expiry`.
- **Backup**: `exportedWarns` builds the export described in decision 1. Four sanitisers cover the import: warn settings,
  antiflood (used by both the antiflood and the admin module), each blacklist word, and each member's warn times.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] The warns export leaves out warn times while expiry is off**
- **Found during:** Task 1, `TestLegacyPunishmentExport` failed as soon as `WarnUser` became a wrapper of `WarnUserAt`.
- **Issue:** the plan assumed the 26-01 golden's row has no times. It seeds through `warns.WarnUser`, which now stores one time
  per warn, so the plan's rule ("times exported whenever the row stores one per warn") would have added `"warn_times"` to the
  golden's export. The golden must never change.
- **Fix:** with expiry off, every row is exported as stored and without times. Times matter only when expiry is on, and an importer
  dates warns without times at the import. With expiry on the plan's rule holds. `TestWarnsExportCountsWhatCounts` checks
  "no time travels with expiry off" instead of "times when complete". The export change moved from Task 3 into Task 1, so every
  commit keeps the golden green.
- **Files modified:** fuku/db/backup/backup.go, fuku/db/backup/punish_import_test.go. **Commit:** 6c1db52 (rule), 3cf33a6 (test).

**2. [Rule 1 - Bug] The Task 1 export counted after clearing the stored times**
- **Found during:** Task 3, while writing `TestWarnsExportCountsWhatCounts`.
- **Issue:** `exportedWarns` cleared `WarnTimes` before calling `Counted`. With expiry on, every warn was then dated at
  `updated_at` and counted, so no expired warn was left out of the export. It shipped in 6c1db52. No caller depends on it yet.
- **Fix:** count from the stored times, then set the exported times. **Commit:** 3cf33a6.

**3. [Discretion] `{duration}` instead of `{days}` in the two new locale keys**
- The plan named `{days}`. Both keys take `{duration}` = `punishDurationLabel(tr, days*86400)`, so plurals and units come from the
  existing `staff_act_duration_*` forms ("1 тиж.", "30 day(s)") rather than "1 днів".

**4. [Rule 2 - Missing critical] The admin module's antiflood settings are sanitised too**
- `importAdmin` also restores `antiflood_settings`, so it calls `sanitizeImportedAntiflood`. Without that a tampered admin block
  would bypass the check the plan put on `importAntiflood`.

**5. [Repo rule] AGENTS.md updated in each task's commit, not all in Task 3**
- The plan put every AGENTS.md change in Task 3. The repository rule puts each change in the commit that makes it, so:
  - Task 1 carries the Data bullet, the export sentence and the PostgreSQL names.
  - Task 2 carries the Subsystem traps bullet.
  - Task 3 carries the import sanitising and the Testing families.

### Plan acceptance check that cannot hold as written

- `git diff --exit-code b7662c7 -- fuku/modules/timed_punishment_legacy_request_test.go` is non-zero because the file did not
  exist at b7662c7 (it was created at 92f29f1). The binding guard, `git diff --exit-code 92f29f1 -- <both 26-01 files>`, exits 0,
  and the file has one commit.

## Earlier tests whose expected values changed

- `TestAllModulesRoundTripEveryMeaningfulField` (fuku/db/backup/roundtrip_test.go): the plan required these seed and check
  additions, and no existing assertion was weakened or removed.
  - New seeds: the antiflood `ActionSecs` 1800, the blacklist word's `ActionSecs` 86400, the warn settings' `WarnModeSecs` 7200
    and `ExpiryDays` 30, and a member's `WarnTimes` (2 days and 1 hour old).
  - Five new assertions on those values after the round trip.

No golden was edited or re-recorded. Each of these `git diff --exit-code` guards exits 0:
- 92f29f1: both 26-01 files.
- 36d2070: lockdown.
- e40a7be: screening.
- 9d4554d: punishment notices.
- 86baf1a: `moderation_legacy_request_test.go`.
- b7662c7: the warns, punishment notice and blacklists goldens.

## Verification

- Task verifies:
  - `TestWarnsCounted` ok.
  - `TestWarnExpiryWrites`, `TestSetWarnModeDuration`, `TestWarnUserCreatesMissingParentRows` and
    `TestConcurrentWarnAndRemovePreserveCount` ok.
  - `TestWarnExpiryMigrationStatements` ok. `TestWarnExpiryMigrationDefaults` skipped locally without PostgreSQL, which is
    allowed.
  - The eight `fuku/modules` names ok (5.6 s). `TestLocaleStyle`, `TestLocalePlaceholdersMatchOtherLanguages` and
    `TestHelpPagesKeepTheirCommands` ok.
  - The seven `fuku/db/backup` names ok.
- Acceptance checks:
  - The latest migration is `20261122130000_add_warn_expiry.sql`, with `ADD COLUMN IF NOT EXISTS` twice.
  - The three old signatures are kept (count 3), and the Makefile names each new test once.
  - Counts in `warns.go`: `WarnUserAt(` 1, `RemoveWarnAt(` 2, `ResetUserWarnsAt(` 2.
  - Counts in `backup.go`: `Counted(` 1, `PunishDurationValid(` 3.
  - Both keys are in all 8 locale files.
- **PostgreSQL, run locally.** A throwaway PostgreSQL 16 cluster and Redis ran in the scratchpad with the CI environment
  variables, and both were stopped afterwards.
  - `TestRepositoryMigrationChain` built the chain with the new migration.
  - `make test-postgres-integrity` passed in all 20 packages: 67 PASS, no FAIL or SKIP, 49 s.
  - The run included these tests on PostgreSQL:
    - `TestWarnExpiryMigrationDefaults`: the second run, the zero defaults, `[]`, exactly one check by `conkey`, 366 refused with
      SQLSTATE 23514 naming `chk_warns_settings_expiry`, and 365 accepted.
    - `TestWarnExpiryWrites`: eight concurrent warns keep eight counts.
    - `TestAllModulesRoundTripEveryMeaningfulField`.
  - The harness reset `/tmp/claude-0` to 0700 during the first attempt, and the cluster stopped. The second run kept the directory
    traversable for the length of the run and restored it to 0700 afterwards.
- Touched families with `-race` (`TestLegacy.*|TestWarn.*|TestApplyWarn.*|TestSetWarn.*|TestRmWarn.*|TestResetWarn.*|
  TestMemberReqWarn.*|TestEventMessageWarn.*|TestAutoEventDefaults|TestSettingsWarns.*|TestSettingsParityWarn.*|
  TestSettingsDriftWarns.*|TestSettingsAuthorityParityWarns.*|TestAISpam.*|TestAutoMessagesEndToEnd|TestEventPMAuto.*|
  TestPunishTimed.*|TestConnectedAdminCommandsInPrivateChat|TestMemberReqScreensFitEveryLocale`): ok, 22.8 s (69 s with the
  build).
- `go test -tags testtools -race -count=1 ./fuku/db/... ./fuku/i18n`: ok, 81 s.
- Make targets and build:
  - `make check-translations` exit 0 (0.4 s).
  - `make generate-docs` changed nothing, and `make check-docs` exit 0 (1.4 s).
  - `make test-shard-check` exit 0 (28 s).
  - `CGO_ENABLED=0 go build ./...` ok (10.7 s).
- golangci-lint v2.13.1:
  - `run --new-from-rev=origin/main ./...`: 0 issues (58 s).
  - The same with `--build-tags=notesttools`: 0 issues (25 s).
  - `run --new=false` over `./fuku/modules/... ./fuku/db/warns/... ./fuku/db/models/... ./fuku/db/backup/... ./fuku/db/migrations/...`:
    117 findings at HEAD and 117 at 8253538 (a scratch worktree), identical once positions are stripped. `clearModuleData` stays
    at 21 and `checkWarns` is still the baseline unused finding.
- gosec (`/root/go/bin/gosec -quiet ./...`, 22.8 s): 19 issues, the baseline set. The one in a touched file is the existing
  G115 at `warnPunishLock` (`warns.go:42`).
- **Full `go test -tags testtools -race -count=1 -v ./fuku/modules`: ok, 404.8 s for the package (408 s for the command), no FAIL,
  SKIP or panic.**
  - 26-05 measured 372.2 s. This plan's test, `TestWarnExpiry`, took 0.22 s in that run (1.5 s alone, almost all of it the real
    locales' first load). Most of the difference is machine variance and the `-v` output, not this plan.
- Test time this plan adds to `fuku/modules`: about 0.2 s in the full run, within the 3 s budget. The `fuku/db` tests run in
  `test-rest`.

## Notes for plan 26-07

- `applyWarnExpiry(ctx, chatID, days)` should write through `warns.SetWarnExpiryContext`. It already refuses values outside
  0..365 (`warns.ErrWarnExpiryOutOfRange`, nothing written) and drops `warn_settings:<chat>`. A change of expiry needs no
  `warns:` key drop, because the warns cache holds the raw row and counts at read time.
- The page's reader `settingsWarnsRead` (`warns.LoadWarnSettingsContext`) already returns `ExpiryDays`.
- `/warnings` shows the expiry as `punishDurationLabel(tr, days*86400)`. Because the label picks the largest unit that divides
  evenly, 7 and 14 days read as weeks (`1 тиж.`, `2 week(s)`). Use the same form on the page so the two agree.
- `warns_help_msg` (uk, en) already says expiry is set on the Warnings page of `/settings`. Keep that true.
- Raising the expiry never brings back a warn already dropped. Plan 26-07's page text should not promise otherwise.
- A Main line or state that shows the expiry needs `settingsMainWorstState` extended, as 26-03 did for the timed word.
- The commands read `punishNow()`. A page test that moves the clock should do it through `mreqSwap(t, &punishNow, ...)`, as
  `warnExpirySite` does.

## Known Stubs

None. Every new identifier has a production caller in this plan, except `SetWarnExpiryContext`: it is called only by tests until
plan 26-07's Warnings page arrives, as the plan intends. golangci-lint's `unused` does not flag it, because it is exported.

## Threat Flags

None beyond the plan's register:
- T-26-13: tampered durations, expiry and times are stored as 0 or dropped (`TestImportSanitizesPunishmentSettings`).
- T-26-14: old warns are dated at their last change, and a previous build's value never expires a warn (`TestWarnsCounted`).
- T-26-15: the transaction's row locks keep the counts under concurrency (`TestWarnExpiryWrites` on PostgreSQL).
- T-26-SC: no dependency was added.

## Self-Check: PASSED

- Files: the six created files exist.
- Commits: 6c1db52, 0270dce and 3cf33a6 are ancestors of HEAD.
