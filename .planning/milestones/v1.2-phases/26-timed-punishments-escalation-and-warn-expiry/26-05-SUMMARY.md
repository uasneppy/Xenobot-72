---
phase: 26-timed-punishments-escalation-and-warn-expiry
plan: 05
subsystem: blacklists
status: complete
tags: [timed-punishments, blacklists, settings-menu, event-messages, private-copies]
requires:
  - 26-02 (blacklists.action_secs column, punish_timed.go core, eventPMSpec.UntilLine for the blacklist, eventActionWords tmute and tban)
  - 26-03 (renderSettingsDuration, settingsDurationPreset, settingsTimedWord)
  - 26-04 (floodPunish pattern, settingsTimedDurationScreen, settingsFloodTimedMode and the m/b cell values)
provides:
  - blacklists.SetBlacklistActionTimedContext(ctx, chat, action, secs) and AddBlacklistTimedContext(ctx, chat, word, action, secs); SetBlacklistAction, SetBlacklistActionContext, AddBlacklist and AddBlacklistContext as their seconds-0 wrappers
  - ListBlacklistEntriesFresh reads action_secs
  - blacklistWrites.Add and .SetAction take the seconds
  - applyBlacklistAction(ctx, chatID, action, secs) with blacklistActionBadDuration; blacklistTimedActions; blacklistActionArgs; blacklistShownAction
  - blacklistSharedAction returns (action, secs)
  - blacklistPunish (the one place the blacklist's punishment is decided); blacklistNotice(b, ctx, user, p autoPunishment, reason)
  - listNoticeReply(tr, event, target, p autoPunishment, reason); blacklistNoticeTimedKeys and the timed branch of blacklistNoticeDefaultReply
  - the Blacklist action screen's timed row and its page actions bd and bz; settingsListEntry.Secs
  - settingsTimedDurationScreenBack (settingsTimedDurationScreen with the caller's Back)
  - locale keys blacklists_set_bl_action_bad_duration, blacklists_bl_watcher_tmuted_user, blacklists_bl_watcher_tbanned_user; blacklists_help_msg explains the timed form
affects: [26-06, 26-09, 26-12]
tech-stack:
  added: []
  patterns:
    - "an automatic rule's action switch lives in one helper (blacklistPunish) whose untimed branches are the old switch moved verbatim and whose timed branch calls applyTimedPunishment"
    - "a list page's uniformity compares the action and the seconds; settingsListEntry.Secs is read by one page only"
    - "a duration screen whose Back is not the page goes through settingsTimedDurationScreenBack, the shared core"
key-files:
  created:
    - fuku/db/blacklists/timed_action_test.go
    - fuku/modules/blacklists_timed_test.go
    - fuku/modules/settings_blacklist_timed_test.go
  modified:
    - fuku/db/blacklists/repository.go
    - fuku/modules/blacklists.go
    - fuku/modules/event_message_auto.go
    - fuku/modules/forwardblock_notice.go
    - fuku/modules/settings_blacklist.go
    - fuku/modules/settings_list.go
    - fuku/modules/settings_duration.go
    - fuku/modules/blacklists_apply_test.go
    - fuku/modules/settings_blacklist_test.go
    - fuku/modules/settings_list_door_test.go
    - fuku/modules/settings_list_privacy_test.go
    - fuku/modules/settings_phase19_test.go
    - locales/en.yml, es.yml, fr.yml, hi.yml, id.yml, pt.yml, ru.yml, uk.yml
    - AGENTS.md
decisions:
  - "applyBlacklistAction: a value of blacklistActionValues stores seconds 0 whatever it is handed; tban or tmute with 0 is blacklistActionUnknown (the pinned choose_correct_option); out of 1m..366d (and -1 for a word that is no duration) is blacklistActionBadDuration; anything else is unknown. blacklistActionValues is unchanged, so TestForwardBlockVocabulary holds."
  - "/blacklistaction reads its arguments through blacklistActionArgs: one word as before; exactly two words whose first (lower-cased) is tmute or tban are the timed form; any other two or more words keep the pinned choose_correct_option."
  - "blacklistSharedAction gives a new word the shared plain action with seconds 0 or the shared effective timed action with its seconds; mixed actions or seconds, a timed action with no duration and an unknown stored value give warn and 0."
  - "blacklistWatcher keeps its warn branch verbatim, returns ContinueGroups for an action autoActionEffective refuses (none, delete, a timed action with no duration: delete and post nothing, as before), and hands the rest to blacklistPunish."
  - "The watcher adds no restrict-right check: it never had one (IsBotAdmin is its gate), and adding one would change the untimed paths. autoActionEffective is the inert gate, the plan's reading of the 26-04 note."
  - "A sender chat under a timed action gets no request and no notice (!user.IsUser(), like the flood); the untimed branches keep their IsAnonymousChannel tests byte for byte."
  - "An end date refused locally (errPunishTimeRanOut) punishes nobody and posts nothing: the blacklist's inert path (the message is already deleted), the flood's rule applied to the blacklist."
  - "listNoticeReply treats a punishment as timed only when its Until is set (applied), and only then fills {duration} and {until} and hands Secs to the default; the timed default escapes the reason, the untimed default keeps splicing it raw."
  - "The Blacklist duration screen's Back is the action screen ({a: bo}), as the plan says; settingsTimedDurationScreen's Back is the page, so its body became settingsTimedDurationScreenBack and both older callers are unchanged."
  - "bz writes again on a press of the stored value with no notice, like bs (not the flood's no-write shortcut)."
  - "bd and bz reuse settingsFloodTimedMode, settingsFloodDurationFields, settingsFloodTimedFields and the m/b cell values, behind settingsListPrivateField."
metrics:
  duration: ~70 min
  completed: 2026-11-22
actuals:
  tokens: 21900
  tasks: 3
  commits: 3
plan_head_before: ad6014143ef20d4e17ebfc1b5c64aabe6a241577
plan_head_after: 597d5ec
---

# Phase 26 Plan 05: The timed blacklist action Summary

`/blacklistaction tmute 2h` (or `tban 1w`), or the Blacklist action screen's Timed mute or Timed ban row and a preset, stores
the action with its duration in `blacklists.action_secs` on every word through one apply function, and a word added later takes
the shared action and seconds. A member whose message matches a word has it deleted and is muted with one `restrictChatMember`
(`MutedPermissions`, `until_date` the punishment time plus the duration) or banned with one `banChatMember` with that end date;
the default notice names the duration and escapes the reason, a custom blacklist text fills `{duration}` and `{until}`, and the
private copy adds its Until line. A sender chat under a timed action gets nothing, a timed action stored with no duration stays
inert, the forward blocklist's notice is byte-identical, and every pinned reply holds. `blacklistWatcher`'s cyclomatic complexity
went from 30 to 26. With plans 26-02 to 26-04, the timed actions of TUNE-01 are implemented.

## Tasks

| Task | Name | Commit |
|------|------|--------|
| 1 | the blacklist repository, applyBlacklistAction with seconds, the shared action and the command form | 1727f3e |
| 2 | the watcher's timed punishment and the shared notice | 6301406 |
| 3 | Timed mute and Timed ban on the Blacklist action screen | 597d5ec |

## What later plans use

- **Repository**: `blacklists.SetBlacklistActionTimedContext(ctx, chat, action, secs)` is one `UPDATE` of `action` and
  `action_secs` on every word (rows matched returned, `blacklist:<chat>` dropped); `AddBlacklistTimedContext(ctx, chat, word,
  action, secs)` stores one word. The four older writes are their seconds-0 wrappers (the goldens call `AddBlacklist` and
  `SetBlacklistAction`).
- **Apply**: `applyBlacklistAction(ctx, chatID, action, secs)`; outcomes `blacklistActionApplied`, `blacklistActionNoWords`,
  `blacklistActionUnknown` (also a timed action with 0), `blacklistActionSaveFailed`, `blacklistActionBadDuration`.
  `blacklistTimedActions` is `tban`, `tmute`. The seams are `blacklistWrites.Add(ctx, chat, word, action, secs)` and
  `.SetAction(ctx, chat, action, secs)`.
- **Watcher** (plan 26-09's ladder goes here): `blacklistPunish(b, ctx, user, p autoPunishment) (applied, notice, err)`;
  `blacklistWatcher` builds `autoPunishment{Action: matched.Action, Secs: matched.ActionSecs}` and gates with
  `autoActionEffective`. `blacklistNotice(b, ctx, user, p, reason)` takes the punishment as applied.
- **Shared notice**: `listNoticeReply(tr, event, target, p autoPunishment, reason)`; `forwardBlockNotice` passes
  `autoPunishment{Action: action}` (signature unchanged; plan 26-09 widens it). `blacklistNoticeTimedKeys` already serves a
  forward blocklist ladder step: pass an applied `autoPunishment` with `Until` set and the timed text and values follow.
- **Page**: `bd` (fields `c`, `k`; `settingsRightAdmin`) and `bz` (fields `c`, `k`, `v`; `settingsRightRestrictBotRestrict`);
  `settingsBlacklistTimedRow`, `settingsBlacklistSelected`, `settingsBlacklistUniformAction` (action, secs, ok),
  `settingsBlacklistEntryWord`, `settingsBlacklistActs`, `settingsBlacklistRedrawSet`. The duration screen is
  `settingsTimedDurationScreenBack(v, titleKey, act, k, mode, storedAction, storedSecs, back)`; its 30-day preset is 47 bytes in a
  private menu (`bd` 37).

## Deviations from Plan

### Plan inconsistencies resolved

**1. The duration screen's Back goes through a sibling of settingsTimedDurationScreen**
- The 26-04 note says to draw the Blacklist duration screen with `settingsTimedDurationScreen(v, page, ...)`, whose Back is
  `{a: op, p: page}` (the list page), while the plan says Back `{"a": "bo"}` (the action screen). The plan's Back was kept:
  `settingsTimedDurationScreen` became a wrapper of `settingsTimedDurationScreenBack`, which takes the Back fields; the Warnings
  and Antiflood pages call the wrapper unchanged (their families pass unedited).

**2. No restrict-right check was added to the watcher**
- The note "check the restrict right with autoActionEffective" describes the flood, whose watcher already checked
  `CanBotRestrict`. `blacklistWatcher` never checked the restrict right (only `IsBotAdmin`); adding a check would change the
  untimed paths the goldens pin. `autoActionEffective` is used as the plan says, as the inert gate.

**3. No `fuku/i18n/settings_locale_test.go` change**
- The plan lists the file, but the page adds no `settings_` key and no new label: the timed cells reuse `settings_opt_tmute` and
  `settings_opt_tban` (26-03's label group covers them) and the notices reuse `settings_bl_action_no_words` and
  `common_settings_save_failed`. The three new keys are `blacklists_` keys, which no limits table covers.

**4. Two more test files follow the seam signatures**
- `settings_list_door_test.go` and `settings_list_privacy_test.go` stub `blacklistWrites.Add` and `.SetAction` too, so their stubs
  gained the seconds argument (no assertion changed). The plan named only `settings_blacklist_test.go` and
  `blacklists_apply_test.go`.

**5. AGENTS.md sentences went into the commit whose change they describe**
- Task 1's commit carries the Data bullet of the new writes, the apply-function paragraph and the A19-3 extension; Task 2's the
  watcher, `blacklistPunish`, the shared notice and the forward blocklist sentences; Task 3's the Blacklist page bullet, the
  codes, the widths, the Permissions sentence and the Testing families.

### Additions beyond the plan

- `ListBlacklistEntriesFresh` reads `action_secs`, so the page sees the seconds; `settingsBlacklistRows` passes them to
  `applyBlacklistAdd`, so a word typed on the page takes the shared timed action too.
- `TestApplyBlacklistActionTimed` also checks a plain action handed seconds stores 0 and `delete` with seconds is unknown;
  `TestBlacklistActionTimedCommand` checks three bad durations; `TestSettingsBlacklistTimed` checks the duration screen's Back, the
  stored preset marked on its own action only and a plain choice clearing the seconds; `TestSettingsBlacklistTimedForged` has
  fourteen cases.

## Earlier tests whose expected values or code changed

- `settings_blacklist_test.go`:
  - `TestSettingsBlacklistAction` (uniform screen): rows `{3,2,2}` became `{3,2,2,2}` (condition and message), and Back is read
    at button index 7 (was 5);
  - `TestSettingsListBlacklistPageFitsEveryLocale`: the mixed action screen's widths `[3 2 2]` became `[3 2 2 2]`; additions: the
    mixed timed and uniform timed screens and the duration screen;
  - `TestSettingsListBlacklistCallbackBytes`: additions only (the timed action screen, both duration screens, `bd` 37, `bz` 47);
  - the `blacklistWrites.SetAction` stub gained `int`.
- `settings_phase19_test.go`: `settingsPhase19Codes` lists `bd` and `bz` (the registry now has 31 list codes).
- `blacklists_apply_test.go`: every `applyBlacklistAction` call passes seconds 0 and the seam stubs gained the seconds; no
  assertion changed.
- `settings_list_door_test.go`, `settings_list_privacy_test.go`: the seam stubs gained the seconds; no assertion changed.

No golden changed: `git diff --exit-code` exits 0 against 92f29f1 (`timed_punishment_legacy_request_test.go`,
`punish_export_legacy_test.go`), 36d2070 (`lockdown_message_legacy_request_test.go`), e40a7be
(`screening_notice_legacy_request_test.go`), 9d4554d (`punishment_notice_legacy_request_test.go`), 86baf1a
(`moderation_legacy_request_test.go`) and b7662c7 (`warns_legacy_request_test.go`, `punishment_notice_legacy_request_test.go`,
`blacklists_legacy_request_test.go`). `settings_fwdblock.go` is unchanged against b7662c7.

## Verification

- Task 1 verify: `TestSetBlacklistActionDuration` ok (`./fuku/db/blacklists`, 1.0 s); `TestApplyBlacklistActionTimed`,
  `TestBlacklistActionTimedCommand`, `TestApplyBlacklist`, `TestApplyBlacklistTooBroad`, `TestApplyBlacklistInheritsAction`,
  `TestLegacyBlacklistRequests`, `TestLegacyTimedPunishmentRequests`, `TestForwardBlockVocabulary` ok (3.1 s).
- Task 2 verify: the ten names (`TestBlacklistTimed`, `TestLegacyPunishmentNoticeRequests`, `TestLegacyTimedPunishmentRequests`,
  `TestEventMessageBlacklist`, `TestEventMessageForwardBlock`, `TestForwardBlockWatcherPunishments`, `TestAutoEventDefaults`,
  `TestEventMessageAutoLogsCarryIDsOnly`, `TestEventEngineCallers`, `TestPMCopyCallers`) ok (5.3 s, each listed by `-v`); the
  `TestEventMessage.*|TestEventPM.*|TestAutoMessagesEndToEnd|TestForwardBlock.*|TestSettingsMessages.*` families ok (24.4 s).
- Task 3 verify: the eighteen names ok (8.4 s); `TestSettingsLocaleKeys`, `TestSettingsLabelLengths` ok.
- Acceptance: the `applyBlacklistAction` signature 1; `func SetBlacklistAction(chatId int64, action string) error` 1;
  `func AddBlacklist(chatId int64, trigger string) error` 1; the unchanged vocabulary 1; `blacklistPunish(` 2;
  `applyTimedPunishment(` 1; `applyBlacklistAction(ctx, p.group.Id` 2; `settings_fwdblock.go` unchanged against b7662c7.
- Touched families with `-race` (`TestLegacy.*|TestBlacklist.*|TestApplyBlacklist.*|TestSettingsBlacklist.*|
  TestSettingsParityBlacklist.*|TestSettingsDriftBlacklist.*|TestSettingsAuthorityParityBlacklist.*|TestSettingsList.*|
  TestSettingsFwdBlock.*|TestSettingsPhase19.*|TestSettingsGateAllDoors|TestSettingsStaffPrivacy|TestSettingsDuration.*|
  TestSettingsWarns.*|TestSettingsAntiflood.*|TestEventMessage.*|TestEventPM.*|TestAutoEventDefaults|TestAutoMessagesEndToEnd|
  TestPMCopyCallers|TestForwardBlock.*|TestSettingsMessages.*|TestEventEngineCallers|TestEventActionWords|TestScreeningEventDefaults|
  TestSettingsMain.*|TestSettingsControlActionAllowList|TestFloodTimed|TestWarnLimitTimed|TestPunishTimed.*|TestSettingsPhase18.*`
  over `./fuku/modules ./fuku/db/blacklists`): ok, 65.8 s and 1.1 s (68.1 s for the command). `go test -race ./fuku/db/blacklists
  ./fuku/i18n`: ok, 1.1 s and 53.4 s.
- `make check-translations` exit 0 (0.4 s); `make check-docs` exit 0, no drift (1.4 s; the help text is not in a generated page);
  `make test-shard-check` exit 0 (2.4 s); `CGO_ENABLED=0 go build ./...` ok (6.8 s); `gofmt -l` prints none of the touched files.
- golangci-lint v2.13.1: `run --new-from-rev=origin/main ./...` 0 issues (43 s); with `--build-tags=notesttools` 0 issues
  (22 s); `run --new=false --max-issues-per-linter=0 --max-same-issues=0 ./fuku/modules/... ./fuku/db/blacklists/...` at HEAD and
  at ad60141 (a scratch worktree) have the same findings once positions are stripped, except `blacklistWatcher`'s gocyclo, 30 at the
  base and 26 at HEAD, and the baseline `dupl` of `rmAllBlacklists`, which moved from line 564 to 628; `addBlacklist` stays at 23
  and `blacklistPunish` and `setBlacklistAction` are not reported.
- gosec (`/root/go/bin/gosec -quiet ./...`, 17.3 s): 19 issues, the baseline set; the one in a touched file is the existing
  `int64(row.ID)` G115 of `settingsBlacklistEntryOf` (line 72 at the base, 78 now).
- No PostgreSQL guarantee was added (no migration, CHECK or conditional update; `TestSetBlacklistActionDuration` rests on none), so
  `make test-postgres-integrity` was not run and the Makefile list is unchanged.
- **Full `go test -tags testtools -race -count=1 ./fuku/modules`: ok, 372.2 s for the package (6 min 15 s for the command), no
  FAIL, SKIP or panic.** 26-04 measured 372.2 s.
- Test time this plan adds to `fuku/modules`, measured together with `-race`: about 2.2 s (`TestBlacklistActionTimedCommand` 1.42 s,
  almost all of it the real locales' first load, which another real-locale test in the same process has already paid;
  `TestSettingsBlacklistTimed` 0.41 s, `TestSettingsAuthorityParityBlacklistTimed` 0.17 s, `TestBlacklistTimed` 0.10 s, the other
  four families 0.12 s together), within the 3 s budget.

## Notes for plan 26-06

- The backup import's check of `blacklists.action_secs` should use `models.PunishDurationValid` (the same bounds as
  `applyBlacklistAction`); a word imported with a timed action and 0 is inert, as everywhere else.
- `blacklistShownAction(action, secs)` is the command wording of a timed blacklist action (`tmute 2h`), like `warnModeShown`.
- Plan 26-09: put the ladder's dispatch in `blacklistPunish` (the untimed branches are verbatim; the timed branch is the only
  `applyTimedPunishment` call), and widen `forwardBlockNotice` to take an `autoPunishment` with `Until` set so the shared timed
  texts and `{duration}`/`{until}` follow.

## Known Stubs

None. Every new identifier has a production caller in this plan.

## Threat Flags

None beyond the plan's register. T-26-10: the end is computed inside `applyTimedPunishment` right before the request and refused
locally under 35 s (`TestBlacklistTimed` checks `until_date` now plus the duration). T-26-11: `bz` and `bd` are validated in step
1 (`settingsListPrivateField` plus `settingsFloodTimedFields` or `settingsFloodDurationFields`), held by
`TestSettingsBlacklistTimedForged`, and a mutation dropping the preset check and the ninth right made 21 assertions fail. T-26-12:
the timed default escapes `{reason}` (`TestBlacklistTimed` `tmute`, a stored reason holding `<b>`). T-26-SC: no dependency added.

## Self-Check: PASSED

- Files: fuku/db/blacklists/timed_action_test.go, fuku/modules/blacklists_timed_test.go and
  fuku/modules/settings_blacklist_timed_test.go exist.
- Commits: 1727f3e, 6301406 and 597d5ec are ancestors of HEAD.
