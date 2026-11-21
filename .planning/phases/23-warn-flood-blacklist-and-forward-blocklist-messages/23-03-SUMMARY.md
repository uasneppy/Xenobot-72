---
phase: 23-warn-flood-blacklist-and-forward-blocklist-messages
plan: 03
subsystem: settings-messages-page
status: complete
tags: [settings, messages-page, event-messages, i18n]
requires:
  - 23-02 (the five event keys, the default builders, eventActionWord)
provides:
  - settingsMsgSample and the field settingsMsgEvent.Sample
  - settingsMsgAutoEvents and the Automatic family (code a, 🤖) in settingsMsgFamilies
  - the sample-filled settingsMsgPreviewReply
  - 14 new settings_msg_ keys and 3 reworded ones in all 8 languages
  - TestSettingsMessagesAutoFamily
affects: [23-04, 23-05, 23-06, 23-07]
tech-stack:
  added: []
  patterns:
    - "an expected shape that depends on a table (family list widths, the private copy row of an event screen) is written per table row in the test (settingsMsgFamilyWidths, eventPMSpecFor), never derived from the renderer"
key-files:
  created:
    - fuku/modules/settings_messages_auto_test.go
  modified:
    - fuku/modules/settings_messages.go
    - fuku/modules/event_message_auto.go
    - fuku/modules/settings_messages_test.go
    - fuku/modules/settings_messages_pm_options_test.go
    - fuku/modules/settings_messages_screens_test.go
    - fuku/i18n/settings_locale_test.go
    - locales/{en,es,fr,hi,id,pt,ru,uk}.yml
    - AGENTS.md
decisions:
  - "Event names follow each language's settings page words: en Warning, Warning limit, Flood, Blacklist, Blocked source (the page is Blocked sources); uk Попередження, Ліміт попереджень, Флуд, Чорний список, Заборонене джерело."
  - "The family name had to fit 18 code points: es Acciones del bot, fr Actions du bot, ru Действия бота (the literal translations are 20 or more); en, hi, id, pt and uk translate Automatic actions."
  - "The Automatic text prompt also says that for flood {count} and {limit} are messages (D-11)."
  - "TestSettingsEditLocaleTokens and the {id} exemption of the locale test cover settings_msg_wait_text_auto like the manual prompt, so its literal tokens stay byte-identical in every language."
  - "The AGENTS.md Messages page bullet changed in the Task 2 commit (with the behaviour it describes, per the repo rule); the test name went in with the test in Task 3."
metrics:
  duration: ~60 min
  completed: 2026-11-21
actuals:
  tokens: 25300
  tasks: 3
  commits: 3
plan_head_before: ae439a9fb692e05c397e1f1de7dbaa17c5570191
plan_head_after: 3b19873252bb9efd95df5bda45d1e2f7d69d65c4
---

# Phase 23 Plan 03: The Automatic family on the Messages page Summary

The Messages page now has a second family, Automatic actions (🤖), with warn, warn limit, flood, blacklist and forward
blocklist as table rows. Each event gets both editor definitions from the existing loop, so Set, Remove, the typed reply,
Silent and the previews work with no engine change. Previews fill each event's sample values and never carry a bot row.
The strings exist in all 8 languages.

## Tasks

| Task | Name | Commit |
|------|------|--------|
| 1 | the Automatic family's strings in 8 languages | d4ebcb3 |
| 2 | the family on the page, its previews and the page's shapes | ec85f55 |
| 3 | prove the Automatic family through the real menu | 3b19873 |

## What later plans use

| Kind | Name | Notes |
|------|------|-------|
| Table | `settingsMsgSample{Count, Limit int; Action, ReasonKey string}`, `settingsMsgEvent.Sample` | zero for a manual event, so its previews are unchanged |
| Table | `settingsMsgAutoEvents` | warn `wn` 🚩 (reserved 1, 1 of 3, warn), warn limit `wl` ⛔ (3 of 3, mute), flood `fl` 🌊 (reserved 1, 6 of 5, mute, `eventmsg_reason_flood`), blacklist `bl` 🔤 (mute), forward blocklist `fb` 🚷 (mute, `staff_fwdblock_reason`) |
| Family | `{Code: "a", Icon: "🤖", NameKey: "settings_msg_fam_auto", ...}` | second entry of `settingsMsgFamilies` |
| Preview | `settingsMsgPreviewReply` | for a non-zero sample: `Values.Count/Limit`, `Values.Action = eventActionWord(groupTr, Sample.Action)`, reason from `ReasonKey`; default input with the same count and limit, `Action` = the mode word, `Reasons` = [reason], no `Rows` |
| Test toolkit | `settingsMsgFamilyWidths` (map by family code), `settingsMsgWantAutoStates` | in `settings_messages_screens_test.go` and `settings_messages_auto_test.go` |

## Earlier tests whose expected sets changed (no assertion weakened)

- `settings_messages_test.go`, `TestSettingsMessagesOpens`: the hub's widths `1, 1, 1, 2` become `2, 1, 1, 2` (two family
  cells), and the hub must also show the `settings_msg_fam_auto` heading (one more required key).
- `settings_messages_pm_options_test.go`, `TestSettingsMessagesPMOptions`: the two hub width checks `1, 1, 1, 2` become `2, 1, 1, 2`.
- `settings_messages_screens_test.go`, `settingsMsgScreens`: the hub entry is `[2 1 1 2]`; a family list's widths come from the
  explicit map `settingsMsgFamilyWidths` (`m` `[2 2 2 1 2]`, `a` `[2 2 1 2]`, an unknown family fails the test); an event screen
  is `[1 1 1 1 1 1 2]` / `[2 2 2 2 2 1 1 2]` when its event has a private copy spec and `[1 1 1 1 1 2]` / `[2 2 2 2 2 1 2]`
  otherwise (the private copy screen keeps its shape).
- Same file, `settingsMsgEveryPMOn`: writes the pm row and checks the mark only for events that have a private copy spec
  (`settingsMsgPMOn` is false without one by design). Every manual event still has both.
- Same file, `TestSettingsMessagesCallbackBytes`: every event screen must carry exactly one `d` button when its event has a
  spec and none when it has not (it was "exactly one" for all events, which were all manual). The per-code size table is
  unchanged: the automatic codes are as long as the manual ones.
- `fuku/i18n/settings_locale_test.go`: the `settings_msg_intro` ceiling rises from `{285, 336}` to `{330, 400}`, because the
  plan rewords the text to cover automatic actions (en 316, longest id 390). The two preview headers got shorter and keep
  their rows.

## Verification (all with `-tags testtools`)

- Task 1: `make check-translations` all present; `go test -race -run '^(TestSettings.*|TestLocaleStyle|TestLocalePlaceholdersMatchOtherLanguages)$' ./fuku/i18n` ok (13 s with build). 14 keys in each of the 8 files; 18 matches in the limits test.
- Task 2: `go test -race -run '^(TestSettingsMessages.*|TestSettingsDriftMessages.*|TestSettingsAuthorityMessages.*|TestSettingsEditResolvedDefs|TestSettingsEditReservedButtons|TestSettingsMain.*|TestSettingsStaffPrivacy|TestEventPMTextsFitEveryLocale|TestAutoEventDefaults)$' ./fuku/modules`: the first run failed only `TestSettingsMessagesCallbackBytes` (the expected-set edit above), then passed.
- Task 3: `TestSettingsMessagesAutoFamily -v`: 4 subtests PASS (3.6 s with build). A mutation that skips the sample fill made the `preview` subtest fail, so the test proves the samples.
- Families with `-race` (`TestSettingsMessages.*`, `TestSettingsDriftMessages.*`, `TestSettingsAuthorityMessages.*`, `TestSettingsEditResolvedDefs`, `TestSettingsEditReservedButtons`, `TestSettingsMain.*`, `TestSettingsStaffPrivacy`, `TestEventPMTextsFitEveryLocale`, `TestAutoEventDefaults`, `TestMessagesEndToEnd`, `TestEventPM.*`, `TestEventMessage.*`, `TestLegacyPunishmentNoticeRequests`, `TestLegacyModerationRequests`, `TestPMCopyCallers`): ok, 131 s (134 s with build), 64 top-level PASS, no FAIL or SKIP.
- Goldens unchanged: `git diff --exit-code 9d4554d -- fuku/modules/punishment_notice_legacy_request_test.go` and `git diff --exit-code 7ecc5fd~1 -- fuku/modules/moderation_legacy_request_test.go` exit 0; no `*legacy*` test file touched.
- `git diff --exit-code 1a382c3 -- go.mod go.sum main.go migrations` exits 0.
- `make check-translations` (0.3 s), `make check-docs` no drift (1.3 s), `make test-shard-check` rc 0 (24 s), `CGO_ENABLED=0 go build ./...` ok (3 s).
- golangci-lint v2.13.1 (`CGO_ENABLED=0`): `run --new-from-rev=origin/main ./...` 0 issues (44 s); with `--build-tags=notesttools` 0 issues (22 s); `run --new=false ./fuku/modules/... ./fuku/i18n/...` has no finding in any file this plan touched (only the baseline findings in other files).
- gofmt clean on every touched Go file. The full `fuku/modules` package was not run (plan 23-07 runs it).

## Deviations from Plan

**1. [Expected set] `settings_msg_intro` ceiling raised**
- The plan rewords the intro to cover automatic actions; the new text is 316 code points in English, over the old 285. The
  row became `{330, 400}`. No other ceiling changed.

**2. [Expected set] `TestSettingsMessagesCallbackBytes` and `settingsMsgEveryPMOn`**
- Not listed in the plan, but both assumed every event has a private copy spec. They now follow `eventPMSpecFor`. Once 23-05
  registers the specs, every event screen has the row again and the checks are as strict as before.

**3. [Repo rule] AGENTS.md split across Task 2 and Task 3**
- The plan put every AGENTS.md change in Task 3. The repo rule says AGENTS.md changes go in the same commit as the change they
  describe, so the Messages page bullet (the family, codes, icons, `Sample`, previews, `[2 1 1 2]`, `[2 2 1 2]`, reserved buttons,
  spec-dependent screen widths) went in with the code in Task 2, and the test family entry with the test in Task 3.

**4. [Addition] The auto prompt's literal tokens are guarded**
- `TestSettingsEditLocaleTokens` holds `settings_msg_wait_text_auto`'s placeholders byte-identical in every language and
  requires `{max}`. The `{id}` exemption of the placeholder test now covers it too, as it does for the manual prompt.

No Rule 1-4 deviations. No architectural change.

## Notes for plan 23-04

- `floodEventReason` still carries `//nolint:unused // first production caller arrives in plan 23-04`; the page reads the flood
  reason through `Sample.ReasonKey` (`eventmsg_reason_flood`), not through `floodEventReason`, so the 23-04 send site stays
  its first caller. Remove the marker there.
- The three builders lost their 23-03 markers: the page table uses `warnLimitDefaultReply`, `floodDefaultReply` and
  `blacklistNoticeDefaultReply`.
- The page uses the input conventions a send site must match: `Action` = the mode word, `Reasons` = the stored reasons,
  `Count`/`Limit` as ints, `Rows` = the site's bot rows (the page passes none). The custom `{action}` is
  `eventActionWord(groupTr, mode)`.
- `ReservedButtons` is 1 for the warn (Remove warn) and the flood (its admin row), 0 for the other three. A flood site that
  keeps its admin row through `eventBotRows(r.Opts)` matches what the editor counts.
- Until 23-05 registers the private copy specs, the automatic event screens have no private copy row and the hub's
  `settingsMsgEveryPMOn` skips them. Both come back by themselves once the specs exist, and 23-05 can also add the private
  copy subtests to `TestSettingsMessagesAutoFamily`.
- Plan 23-06 extends the forged-press and authority tests to the new `k` values (T-23-06). `settingsMsgDefByFields` and the
  actions' rights already cover them unchanged.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register. T-23-06: no new code, field or right; the existing gates apply to the new rows. T-23-07: the
`preview` subtest asserts no keyboard on both previews. T-23-08: `TestSettingsStaffPrivacy` passes over `settingsMsgScreens`,
which now draws both families.

## Self-Check: PASSED

- Files: `fuku/modules/settings_messages_auto_test.go` exists.
- Commits: d4ebcb3, ec85f55 and 3b19873 are ancestors of HEAD.
