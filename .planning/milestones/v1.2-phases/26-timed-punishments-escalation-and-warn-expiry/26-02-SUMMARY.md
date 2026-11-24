---
phase: 26-timed-punishments-escalation-and-warn-expiry
plan: 02
subsystem: warns
status: complete
tags: [timed-punishments, warns, migration, event-messages, private-copies, member-requirements]
requires:
  - 26-01 (the two goldens, guarded unchanged)
provides:
  - migration 20261122120000 (warns_settings.warn_mode_secs, antiflood_settings.action_secs, blacklists.action_secs)
  - models.PunishMinSecs, models.PunishMaxSecs, models.PunishDurationValid
  - WarnSettings.WarnModeSecs, AntifloodSettings.ActionSecs, BlacklistSettings.ActionSecs
  - warns.SetWarnModeContext (SetWarnMode kept as the seconds-0 wrapper)
  - punish_timed.go (punishMinSecs, punishMaxSecs, punishMinLead, punishNow, errPunishTimeRanOut, autoPunishment, autoActionEffective, punishDurationValid, parsePunishDuration, punishDurationLabel, punishDurationShort, punishUntil, punishUntilTooClose, applyTimedPunishment)
  - applyWarnMode(ctx, chatID, mode, secs) with warnSettingsBadDuration; warnModeArgs; warnModeShown
  - warnApplyResult.Secs and .Until; applyWarnLimitAction with seconds (applyWarnLimitUntimed keeps the untimed branches)
  - eventDefaultInput.Secs; warnLimitTimedKeys; eventActionWords tmute and tban
  - eventPMSpec.UntilLine (warnlimit, flood, blacklist, fwdblock); eventmsg_pm_until
  - memberReqLimitMode (tmute to mute, tban to ban); a tban limit is memberReqRemoved
affects: [26-03, 26-04, 26-05, 26-06, 26-09]
tech-stack:
  added: []
  patterns:
    - "one helper sends every timed automatic punishment (applyTimedPunishment), with the end computed right before the request and refused locally under 35 s"
    - "a timed mode stored with seconds 0 is inert (autoActionEffective), so no stored setting starts acting by itself"
    - "timed default texts are separate maps read only when Secs is set (warnLimitTimedKeys), so every untimed default stays byte for byte"
key-files:
  created:
    - migrations/20261122120000_add_timed_punishment_durations.sql
    - fuku/db/models/punish.go
    - fuku/db/warns/timed_mode_test.go
    - fuku/db/migrations/timed_punishment_migration_test.go
    - fuku/modules/punish_timed.go
    - fuku/modules/punish_timed_test.go
    - fuku/modules/warns_timed_test.go
  modified:
    - fuku/db/models/warns.go
    - fuku/db/models/antiflood.go
    - fuku/db/models/blacklists.go
    - fuku/db/warns/repository.go
    - fuku/modules/warns.go
    - fuku/modules/settings_warns.go
    - fuku/modules/event_message.go
    - fuku/modules/event_message_auto.go
    - fuku/modules/event_message_pm.go
    - fuku/modules/memberreq_notice.go
    - fuku/modules/memberreq_enforce.go
    - fuku/i18n/eventmsg_locale_test.go
    - fuku/modules/event_message_auto_test.go
    - fuku/modules/settings_warns_test.go
    - fuku/modules/warns_settings_apply_test.go
    - fuku/db/migrations/warn_mode_check_test.go
    - fuku/db/migrations/blacklist_action_check_test.go
    - locales/en.yml, es.yml, fr.yml, hi.yml, id.yml, pt.yml, ru.yml, uk.yml
    - Makefile
    - AGENTS.md
decisions:
  - "applyWarnLimitAction tests the timed case first (an autoPunishment that is Timed and effective) and otherwise calls applyWarnLimitUntimed, the old switch moved verbatim, so the untimed branches and the inert default branch are byte for byte what they were."
  - "warnLimitDefaultReply reads a new map (warnLimitTimedKeys) only when in.Secs is set; warnLimitKeys is untouched, so an inert tmute or tban can never reach a timed or untimed text by accident (the Messages page preview included)."
  - "An end date refused locally (errPunishTimeRanOut) makes the warn limit punish nobody: punished false with no error, so the warn stays counted, nothing is reset, and warnThisUser posts today's inert limit reply. The helper's Warn is gated hourly per chat (punishTimeRanOutWarn, a memberDeleteWarnGate); the warn module's own Warn names the chat and user IDs."
  - "applyTimedPunishment validates its action itself (tmute or tban, else an error and nothing sent) and computes the end from punishNow() with no pacer between it and the request; its comment and the AGENTS.md sentence state the 30 s HTTP client bound and no shorter one."
  - "The eventmsg_pm_until line is placed right after the default private text and before the reason line (\"after the action text\", D-17)."
  - "/setwarnmode TBAN 1d is accepted (the first word is lower-cased, as for the plain modes); a plain word ignores what follows (ban 1d stores ban), and a timed word followed by anything that is not a valid duration is -1 for applyWarnMode, the bad-duration reply."
metrics:
  duration: ~45 min
  completed: 2026-11-22
actuals:
  tokens: 37000
  tasks: 3
  commits: 6
plan_head_before: 517c5f5520224362f39f5404ff56e55c6b0c869f
plan_head_after: 339be204eed52fe25fb14afd7e2ae07032494f56
---

# Phase 26 Plan 02: The timed warn limit end to end Summary

`/setwarnmode tmute 2h` (or `tban 1d`) stores the warn limit's action with its duration in the new
`warns_settings.warn_mode_secs`; when a member reaches the limit the bot sends one `restrictChatMember` with
`MutedPermissions` (or one `banChatMember`) whose `until_date` is computed right before the request, resets the warns
and posts the warn limit message naming the duration (default or custom, `{duration}` and `{until}` filled), its private
copy adds an Until line, and a member requirement warn follows the timed limit. A timed mode stored with no duration, and
every group that sets nothing, behaves exactly as before: every golden passes unchanged.

## Tasks

| Task | Name | Commit |
|------|------|--------|
| 1 | the duration columns, the warn repository write, and their PostgreSQL tests | c60bbcc |
| 2 | the timed-punishment core, applyWarnMode with seconds and the /setwarnmode forms | 7b73edc |
| 3 | the timed warn limit end to end: the request, the message, the private copy and the requirement path | 5310a2b |
| - | lint: punishMinSecs given a production reader | f52ddfb |
| - | the two PostgreSQL check tests count only the column's own checks | 26e2587 |
| - | the migration's header no longer spells the statement form (acceptance count) | 339be20 |

## What later plans use

- **Storage**: the three duration columns (`chk_warns_settings_mode_secs`, `chk_antiflood_action_secs`,
  `chk_blacklists_action_secs`, 0 to 31622400) and the model fields, JSON `omitempty`, no `default:` tag. Plans 26-04 and
  26-05 write `antiflood_settings.action_secs` and `blacklists.action_secs`; nothing writes them yet.
- **Bounds**: `models.PunishMinSecs` (60), `models.PunishMaxSecs` (31622400), `models.PunishDurationValid` (plan 26-06's
  import check); `punishDurationValid` is held equal to it by `TestPunishTimedCore`.
- **Core** (`fuku/modules/punish_timed.go`): `autoActionEffective(action, secs)`, `parsePunishDuration(word)`,
  `punishDurationLabel(tr, secs)`, `punishDurationShort(secs)`, `punishUntil(now, secs)`, `punishUntilTooClose(until)`,
  `applyTimedPunishment(b, chatID, userID, action, secs) (until int64, err error)` (returns `errPunishTimeRanOut` with nothing
  sent), `autoPunishment{Action, Secs, Until}` with `Timed()`, the clock seam `punishNow`.
- **Warn mode**: `applyWarnMode(ctx, chatID, mode, secs)`; the settings page (`settings_warns.go`) passes 0 until plan 26-03.
  The seam is `warnSettingsWrites.Mode func(ctx, chatID, mode, secs) error`.
- **Engine**: `eventDefaultInput.Secs`; `eventActionWords` maps `tmute` and `tban` to the mute and ban keys;
  `eventPMSpec.UntilLine` is already set for `flood`, `blacklist` and `fwdblock`, so 26-04 and 26-05 only need to fill
  `eventValues.Until` (and `Duration`) for their timed actions.
- **Locale keys** (8 files): `warns_mode_updated_tmute`, `warns_mode_updated_tban`, `warns_mode_bad_duration`,
  `warns_limit_tmuted`, `warns_limit_tbanned`, `eventmsg_pm_until`; `warns_help_msg` explains the timed form in every language.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] The PostgreSQL check tests counted the new duration checks**
- **Found during:** the local PostgreSQL integrity run after Task 3.
- **Issue:** `TestBlacklistActionValuesOnPostgres` and `TestWarnModeCheckOnPostgres` selected the checks whose definition text
  contains `action` or `warn_mode`, so `chk_blacklists_action_secs` and `chk_warns_settings_mode_secs` were counted and both
  failed (`make test-postgres-integrity` would have failed in CI).
- **Fix:** both select the checks whose `conkey` includes the column itself; the assertion (exactly `chk_blacklist_action`,
  exactly `chk_warn_mode`) is unchanged. AGENTS.md's two sentences say so.
- **Files modified:** `fuku/db/migrations/blacklist_action_check_test.go`, `fuku/db/migrations/warn_mode_check_test.go`, AGENTS.md
- **Commit:** 26e2587

**2. [Rule 3 - lint] `punishMinSecs` had no production reader**
- `punishDurationValid` now reads `punishMinSecs` and `punishMaxSecs` (the aliases of the model bounds) instead of calling
  `models.PunishDurationValid`; a test holds the two equal at every boundary. Commit f52ddfb.

**3. [Process] AGENTS.md sentences invalidated by Task 2 were updated in Task 3's commit**
- Task 2 (7b73edc) changed `/setwarnmode` and `applyWarnMode`, which made two AGENTS.md sentences ("accepts exactly ban, kick
  and mute") stale; they were rewritten in 5310a2b, together with the plan's Task 3 sentences, rather than in 7b73edc. No
  commit was amended.

**4. [Plan acceptance] The migration's header spelled "ADD COLUMN IF NOT EXISTS"**
- The acceptance count of that phrase read 4 (three statements and the comment). The comment now says "the IF NOT EXISTS
  column form"; the SQL is unchanged and the file is new on this branch (applied only by test databases built from the
  chain). Commit 339be20.

**5. [Plan detail] `TestWarnLimitTimed` `too_close` calls the helper with a 34 s duration**
- With a fixed clock the end and the too-close check read one time, so the subtest passes a duration that `punishUntil`
  accepts (at least 30 s) but that is under the 35 s lead; it also checks a non-timed action is refused with nothing sent.
  The Warn names the chat (and the action), no user.

**6. [Plan detail] The custom text subtest has no literal ampersand**
- A stored event text is the owner's HTML and is not escaped, so `tmute_custom` checks that the filled values are escaped
  (the action word and the duration label go through `html.EscapeString`) rather than a stored `&`.

### Earlier tests whose expected values or code changed

- `TestEventActionWords` (`event_message_auto_test.go`): `tban` and `tmute` left the "no word" list (the plan adds them to
  `eventActionWords`, D-16) and are now asserted to equal the mute and ban words, non-empty; `Tmute` joined the no-word list.
- `TestApplyWarnSettings` (`warns_settings_apply_test.go`) and the Warnings page's failed-save subtest
  (`settings_warns_test.go`): the calls pass seconds 0 and the seam stubs take the new signature; no assertion changed.
- `TestEventMsgLocaleKeys` tables (`fuku/i18n/eventmsg_locale_test.go`): a row for `eventmsg_pm_until` ({40, 60}) and the
  key added to `eventMsgNotDefaultText` (it does not name the group).
- `TestBlacklistActionValuesOnPostgres`, `TestWarnModeCheckOnPostgres`: the query, not the expected values (deviation 1).

No golden changed: `git diff --exit-code` exits 0 against 92f29f1 (`timed_punishment_legacy_request_test.go`,
`punish_export_legacy_test.go`), 36d2070 (lockdown), e40a7be (screening), 9d4554d (punishment notices), 86baf1a (moderation)
and b7662c7 (`warns_legacy_request_test.go`, `go.mod`, `go.sum`).

## Verification

- Task 1 verify: `TestTimedPunishmentMigrationStatements` PASS, `TestTimedPunishmentMigrationDefaults` SKIP without
  PostgreSQL (the allowed skip); `TestSetWarnModeDuration`, `TestWarnUserCreatesMissingParentRows`,
  `TestConcurrentWarnAndRemovePreserveCount` PASS.
- Task 2 verify: the seven `fuku/modules` names ok (3.2 s, 44 s with the build); `TestLocaleStyle`,
  `TestLocalePlaceholdersMatchOtherLanguages`, `TestHelpPagesKeepTheirCommands` ok.
- Task 3 verify: the thirteen `fuku/modules` names ok (5.2 s); `TestEventMsgLocaleKeys`, `TestLocaleStyle`,
  `TestLocalePlaceholdersMatchOtherLanguages` ok.
- Acceptance: `ADD COLUMN IF NOT EXISTS` 3, `^(BEGIN|COMMIT)` 0; the latest migration is
  `20261122120000_add_timed_punishment_durations.sql`; the Makefile names `TestTimedPunishmentMigrationDefaults` and
  `TestSetWarnModeDuration` once each; the `SetWarnMode` and `applyWarnMode` signatures as specified; `warns_mode_bad_duration`
  in 8 files; no G101 word in `punish_timed.go`; `UntilLine: true` 4; `applyTimedPunishment(` in `warns.go` and AGENTS.md, whose
  sentence names the 30 s HTTP client bound and no shorter one; the 26-01 golden file has one commit.
- **PostgreSQL, run locally** (a throwaway PostgreSQL 16 cluster and Redis in the scratchpad, the CI environment variables):
  `TestRepositoryMigrationChain` built the chain with the new migration; then `make test-postgres-integrity` ok in all 20
  packages, 65 PASS, no FAIL or SKIP, 46 s, including `TestTimedPunishmentMigrationDefaults` (the second run, the zero defaults,
  the three named checks, 31622401 refused with SQLSTATE 23514 and 31622400 accepted) and `TestSetWarnModeDuration` on
  PostgreSQL. That run is what found deviation 1.
- Touched families with `-race`:
  `^(TestLegacy|TestWarn|TestApplyWarn|TestSetWarn|TestPunishTimed|TestRmWarn|TestMemberReqWarn|TestMemberReqNotice|TestEventMessageWarn|TestEventMessageFlood|TestEventMessageBlacklist|TestEventMessageForwardBlock|TestEventPM|TestAutoEventDefaults|TestEventActionWords|TestScreeningEventDefaults|TestSettingsWarns|TestSettingsParityWarn|TestSettingsDriftWarns|TestSettingsAuthorityParityWarns|TestSettingsMessages|TestAutoMessagesEndToEnd|TestPMCopiesEndToEnd|TestMemberReqScreensFitEveryLocale)`
  ok, 25.3 s (27.5 s with a warm build); `go test -tags testtools -race -count=1 ./fuku/db/... ./fuku/i18n` ok, 75 s.
- `make check-translations` exit 0; `make check-docs` exit 0 (1.4 s; `make generate-docs` changed nothing, the warns page does
  not carry the help text); `make test-shard-check` exit 0 (23.5 s); `CGO_ENABLED=0 go build ./...` ok (9 s); `gofmt -l` on the
  touched files prints nothing.
- golangci-lint v2.13.1: `run --new-from-rev=origin/main ./...` 0 issues (50 s cold, then 0 after f52ddfb); the same with
  `--build-tags=notesttools` 0 issues (22 s); `run --new=false ./fuku/modules/... ./fuku/db/warns/... ./fuku/db/models/...
  ./fuku/db/migrations/...` 122 issues at HEAD and 122 at 517c5f5 (a worktree), the same findings with the same messages
  (complexities included) once positions are stripped; in the touched files only the baseline `checkWarns` unused.
- gosec (`$(go env GOPATH)/bin/gosec -quiet ./...`, 17.8 s): 19 issues, the same 18 files and counts as the 26-01 baseline
  (`warns.go` keeps its one G115 at `warnPunishLock`); no new finding.
- **Full `go test -tags testtools -race -count=1 ./fuku/modules`: ok, 374.4 s for the package (6 min 17 s for the command), no
  FAIL, SKIP or panic.**
- Test time this plan adds to `fuku/modules`: about 1.6 s alone (`TestPunishTimedCore` 1.3 s, almost all of it the real
  locales' first load, which another real-locale test in the same process has already paid; `TestWarnLimitTimed` 0.27 s,
  `TestSetWarnModeTimedCommand` 0.08 s, `TestApplyWarnModeTimed` 0.01 s), within the 3 s budget. The package went from 368.7 s
  (26-01) to 374.4 s, inside runner noise.

## Not verified

- The timed restrict and ban against the real Bot API (the fake records the request; Telegram's 30 s permanence rule is held
  by construction through `punishMinLead`).

## Notes for plan 26-03

- The page calls `applyWarnMode(ctx, p.group.Id, value, 0)` (`settings_warns.go`); the timed press passes the preset's seconds.
  `applyWarnMode` already refuses a timed mode with 0 (unknown) and a duration outside the bounds (bad duration), and a plain
  mode stores 0, so a page Mute, Ban or Kick press clears a stored duration.
- `warnModeShown(mode, secs)` and `punishDurationShort`/`punishDurationLabel` are the wordings to reuse; the page's inert line
  (`settings_warns_action_inert`) must now be drawn only for a timed mode with no duration (`autoActionEffective` false), not for
  every stored `tmute`/`tban`.
- AGENTS.md's Warnings page bullet now says the page passes seconds 0 "until plan 26-03": that sentence is 26-03's to rewrite.
- `warnSettingsWrites.Mode` is `func(ctx, chatID, mode, secs) error`; stub it with that signature.
- Every write still drops `warn_settings:<chat>`; the page's reader `warns.LoadWarnSettingsContext` already returns
  `WarnModeSecs`.
- The two PostgreSQL check tests now select by `conkey`; any new check test on a column that has a `_secs` sibling should do
  the same.
- Plans 26-04 and 26-05: `UntilLine` is already set for `flood`, `blacklist` and `fwdblock`, and `eventActionWords` already maps
  `tmute`/`tban`; fill `eventValues.Duration` and `.Until` from `applyTimedPunishment`'s returned end, and keep the flood's
  inert check on `floodWords` (an inert stored `tmute` still builds an action word in `floodNotice`'s values, unused because the
  inert path goes through `sendDefaultPost`).

## Known Stubs

None. Every new identifier has a production caller in this plan (`autoPunishment` and `applyTimedPunishment` in
`applyWarnLimitAction`, `punishDurationShort` in `/warnings`, `punishDurationLabel` in the replies and the event values).

## Threat Flags

None beyond the plan's register. T-26-02: the bounds in `applyWarnMode` and the CHECKs, the end computed right before the
request and refused under 35 s (`TestPunishTimedCore`, `too_close`). T-26-03: a timed mode with seconds 0 is inert
(`inert_no_duration`, the `warnlimit_inert_tmute` golden). T-26-04: the new Warn lines carry chat and user IDs and the action
only. T-26-SC: `go.mod` and `go.sum` unchanged.

## Self-Check: PASSED

- Files: every file under key-files exists.
- Commits: c60bbcc, 7b73edc, 5310a2b, f52ddfb, 26e2587 and 339be20 are ancestors of HEAD.
