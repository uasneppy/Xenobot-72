---
phase: 25-lockdown-messages
plan: 03
subsystem: settings-messages
status: complete
tags: [lockdown, settings, messages-page, editor, locales, evt-06, evt-07, evt-08]
requires:
  - 25-02 (the five lockdown event keys, {percent}, the three default builders, the notes and lockdownNoteReserve)
provides:
  - the Lockdown family (l 🚨) on the Messages page with ld 🔒, lj 👥, lf 💬, la 🧠 and lu 🔓
  - settingsMsgSample.Window, Percent, Trigger and NoReason, filled by settingsMsgPreviewReply
  - lockdownStartAppended and lockdownLiftAppended (what a Full preview of a custom text shows after it)
  - 14 new settings_ locale keys and the reworded settings_msg_intro in 8 languages
  - TestSettingsMessagesLockdownFamily
  - settingsMsgFamilyPM and settingsMsgWantFamilyPM in the page's screen tests
affects: [25-04, 25-05, 25-06, 25-07, 25-08]
tech-stack:
  added: []
  patterns:
    - "the page's private copy expectations are per family (settingsMsgFamilyPM): required for m, a and r, forbidden for l"
    - "a lockdown event's preview reason is lockdownTriggerSentence with the sample numbers, or nothing for the lift"
key-files:
  created:
    - fuku/modules/settings_messages_lockdown_test.go
  modified:
    - fuku/modules/settings_messages.go
    - fuku/modules/event_message_lockdown.go
    - fuku/modules/settings_messages_screens_test.go
    - fuku/modules/settings_messages_test.go
    - fuku/modules/settings_messages_auto_test.go
    - fuku/modules/settings_messages_pm_options_test.go
    - fuku/modules/settings_messages_screening_test.go
    - fuku/i18n/settings_locale_test.go
    - locales/{en,es,fr,hi,id,pt,ru,uk}.yml
    - AGENTS.md
decisions:
  - "The lift event's Appended is lockdownLiftAppended (lockdownLiftNotes of in.Lockdown). The plan named only lockdownStartAppended and said the lift shows no note; a preview's lift has zero facts, so lockdownLiftNotes returns \"\" and the preview shows no note, while lockdownLiftNotes and lockdownLiftNoteLines get the production caller the binding 25-02 note requires to drop their unused markers, and the page states what the 25-05 site will append."
  - "A lockdown event's private copy screens (the definition every event still registers) are not drawn by settingsMsgScreens: no press reaches them, because settingsMsgDefByFields refuses d=p without a spec. The lockdown event screen with PMOn set is still drawn and must have no private copy row."
  - "The typed-and-silent subtest works the page and /lockdown as the group's live creator (newOwner), not env.admin: listEditAdmin's administrator record has can_change_info but no can_restrict_members, and /lockdown needs the restrict right. The plan's 'by the env's admin' is satisfied in spirit (a live admin of the group who has both rights)."
  - "The page intro's limits row went from {330, 400} to {360, 440} (English 355, longest other language Indonesian 434), inside the plan's ceiling of {380, 460}."
metrics:
  duration: ~55 min
  completed: 2026-11-22
actuals:
  tokens: 15300
  tasks: 3
  commits: 3
plan_head_before: b3defed2fcb46ed681f92fe6ae862c8c113239c1
plan_head_after: 1c8c212cd15ba72c1ba282c5809c431dc484dac5
---

# Phase 25 Plan 03: The Lockdown family on the Messages page Summary

The Messages page now has a fourth section, Lockdown (🚨), with five events: by an admin (`ld`), join surge (`lj`), flood (`lf`), AI
spam burst (`la`) and lifted (`lu`). Each opens the editor with the text, media and buttons parts and a Silent switch, and none has
a private copy line or row. Every lockdown text holds back 400 units for the technical notes, so the prompt says `Up to 3696`, or
624 while a media item is stored. Full previews fill the trigger samples: 15 joins in 60 s, 30 messages in 30 s with 80 %, and 5
verdicts in 120 s, each with its trigger sentence as `{reason}`. A start event's preview of a custom text adds the approved-muted
note after it. The lift preview has no reason and no note. A text typed on the page reaches the next `/lockdown` at once through
the site plan 25-02 added.

## Tasks

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | the Lockdown family's strings in 8 languages | c8ecb66 | 8 locales, `fuku/i18n/settings_locale_test.go` |
| 2 | the rows, the Lockdown family, the samples and the page's shapes | 45f7bb3 | `settings_messages.go`, `event_message_lockdown.go`, five page tests, AGENTS.md |
| 3 | prove the Lockdown family through the real menu | 1c8c212 | `settings_messages_lockdown_test.go` (new), AGENTS.md |

## What later plans use

| Kind | Name | Notes |
|------|------|-------|
| Events | `settingsMsgLockdownEvents`: `ld` 🔒 `lockdownStartDefaultReply`; `lj` 👥, `lf` 💬, `la` 🧠 `lockdownAutoDefaultReply(models.LockdownTrigger*)`; `lu` 🔓 `lockdownLiftDefaultReply` | starts: `Appended: lockdownStartAppended`; lift: `Appended: lockdownLiftAppended` |
| Family | `{Code: "l", Icon: "🚨", ..., ReservedText: lockdownNoteReserve}`, the fourth row of `settingsMsgFamilies` | wait pages `hldg hljg hlfg hlag hlug` |
| Samples | `settingsMsgSample.Window`, `Percent`, `Trigger`, `NoReason` | `Window` goes to `values.Window` as `strconv.Itoa` and to `in.Window`; `Percent` to both |
| Appended | `lockdownStartAppended(tr, in)` = `lockdownStartNotes(tr, in.Lockdown)`; `lockdownLiftAppended(tr, in)` = `lockdownLiftNotes(tr, in.Lockdown)` | `event_message_lockdown.go` |
| Test helpers | `settingsMsgFamilyPM`, `settingsMsgWantFamilyPM` (screens test), `settingsMsgWantLockdownStates`, `settingsMsgSilentToggleOf` (lockdown test) | a new family must add its row to `settingsMsgFamilyPM` and `settingsMsgFamilyWidths` |
| Locale keys | `settings_msg_fam_lockdown`, `_fam_lockdown_intro`, `_wait_text_lockdown`, `_hint_lockdown`, `_ev_lock{down,join,flood,ai,lift}`, `_desc_lock{down,join,flood,ai,lift}`; reworded `settings_msg_intro` | limits rows, the six cells' label group, the prompt's literal tokens and `{max}` |

All seven `//nolint:unused // first production caller arrives in plan 25-03` markers are gone (`lockdownNoteReserve`,
`lockdownNoticeOpts`, `lockdownAutoIntroKeys`, `lockdownAutoDefaultReply`, `lockdownLiftNoteLines`, `lockdownLiftNotes`,
`lockdownLiftDefaultReply`). `golangci-lint run --tests=false --enable-only=unused ./fuku/modules/...` reports 0 issues.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `lockdownLiftNotes` needed a production caller**
- **Found during:** Task 2
- **Issue:** the 25-02 note says to remove the markers from `lockdownLiftNoteLines` and `lockdownLiftNotes`. The plan's table has the
  lift preview show no note and gives the lift event no `Appended`. Then neither identifier has a production caller and `unused`
  would report both.
- **Fix:** I added `lockdownLiftAppended` (`lockdownLiftNotes` of `in.Lockdown`) as the lift event's `Appended`. A preview's lift has
  zero facts, so the note is empty and the preview shows none, as D-32 requires. The page now names what the plan 25-05 site will
  append after a custom lift text.
- **Files modified:** `fuku/modules/event_message_lockdown.go`, `fuku/modules/settings_messages.go`
- **Commit:** 45f7bb3

**2. [Discretion] The typed-and-silent subtest runs as the live creator**
- `listEditAdmin` gives `env.admin` `can_change_info` without `can_restrict_members`, and `/lockdown` refuses without the restrict
  right. The subtest therefore uses `env.newOwner(env.chat)` for the page and for `/lockdown`.

**3. [Rule 2 - stronger check] AGENTS.md updated with Task 2**
- The plan put every AGENTS.md change in Task 3. Task 2 already changes what several bullets describe (the hub rows, the families,
  the editor's `ReservedText`, the 25-03 marker sentence), and the repo rule is "same commit as the change". So those bullets went
  into 45f7bb3, and the Testing family went into 1c8c212.

### Earlier tests whose expected values changed (no assertion weakened)

1. `settings_messages_test.go` `TestSettingsMessagesOpens`:
   - hub widths changed from `2,1,1,1,2` to `2,2,1,1,2`;
   - the hub must now also carry `settings_msg_fam_lockdown` (added);
   - the comment was updated.
2. `settings_messages_auto_test.go` `TestSettingsMessagesAutoFamily`: hub widths changed from `2,1,1,1,2` to `2,2,1,1,2`.
3. `settings_messages_pm_options_test.go` `TestSettingsMessagesPMOptions`: the hub and redrawn hub widths changed from `2,1,1,1,2` to
   `2,2,1,1,2`.
4. `settings_messages_screening_test.go` `TestSettingsMessagesScreeningFamily`:
   - hub widths changed from `2,1,1,1,2` to `2,2,1,1,2`;
   - the Member requirements cell was alone on row 1. It is now first on row 1, and the test checks its exact position.
5. `settings_messages_screens_test.go`:
   - the hub widths are `[2 2 1 1 2]`;
   - `settingsMsgFamilyWidths` gains `"l": "[2 2 1 2]"`;
   - `settingsMsgEveryPMOn`, `settingsMsgScreens` and `TestSettingsMessagesCallbackBytes` go through the new
     `settingsMsgWantFamilyPM`. It still fails on a missing spec in `m`, `a` and `r`, and now also fails on any spec in `l` and on a
     family the table does not list.
   - A lockdown event screen expects no private copy row and exactly 0 `d` buttons. Every other event screen still expects exactly 1.
   - A lockdown event's unreachable private copy screens, prompts and refusals are not drawn.
6. `fuku/i18n/settings_locale_test.go`:
   - new limits rows, a label group and literal and `{max}` token rows only;
   - `settings_msg_intro` limits changed from `{330, 400}` to `{360, 440}`;
   - `TestSettingsReusedKeyPlaceholders` admits the lockdown prompt's literal `{id}` alongside the other three prompts.

No golden changed. `git diff --exit-code` exits 0 for each:
- `36d2070 -- lockdown_message_legacy_request_test.go`
- `e40a7be -- screening_notice_legacy_request_test.go`
- `9d4554d -- punishment_notice_legacy_request_test.go`
- `86baf1a -- moderation_legacy_request_test.go`

The lockdown golden still has only its own commit. `go.mod`, `go.sum`, `main.go` and `migrations` are unchanged since d8c98c6.

## Verification (with durations)

**Task verifies**

| Check | Result |
|-------|--------|
| Task 1: `make check-translations`, then `go test -tags testtools -race -count=1 ./fuku/i18n` | all present; ok, 50.4 s |
| Task 2: the plan's family run (`TestSettingsMessages.*`, drift, authority, `TestSettingsEditResolvedDefs`, `ReservedButtons`, `ReservedText`, `Check.*`, `Typed.*`, `TestSettingsMain.*`, `TestSettingsStaffPrivacy`, `TestLockdownEventDefaults`, `TestEventPMTextsFitEveryLocale`, `TestEventPMSpecs`) | ok, 35.1 s package (72 s wall) |
| Task 3: `TestSettingsMessagesLockdownFamily`, `ScreeningFamily`, `AutoFamily`, `Opens`, `Preview`, `TestEventMessageLockdownStart` with `-v` | ok, 3.4 s; no FAIL, SKIP or "no tests to run" |

**`TestSettingsMessagesLockdownFamily` on its own: 1.68 s**

| Subtest | Time |
|---------|------|
| hub and list | 0.36 s |
| screens | 0.03 s |
| typed and silent | 0.04 s |
| previews | 1.25 s (the first real-locale load) |

When the locales are already loaded, it takes 0.13 s. The plan's budget was about 3 s.

A mutation check confirmed the test bites: with `lockdownStartAppended` returning "", the `previews` subtest fails. I then restored
the file.

**Acceptance greps**
- 1 family row; 5 event codes;
- 0 markers left;
- `[2 2 1 1 2]` present and `[2 1 1 1 2]` absent in the screens test;
- `"l": "[2 2 1 2]"` once;
- 14 keys in each of the 8 locale files;
- `"settings_msg_wait_text_lockdown"` twice in the locale test;
- `TestSettingsMessagesLockdownFamily` in AGENTS.md;
- 4 subtests PASS.

**Touched families with `-race`.** The pattern was:

`TestSettingsMessages.*|TestSettingsDriftMessages.*|TestSettingsAuthorityMessages.*|TestSettingsStaffPrivacy|TestSettingsMain.*|TestSettingsEdit.*|TestEventMessage.*|TestEventPM.*|TestPMCopy.*|TestPMCopies.*|TestLegacy.*|TestEventEngine.*|TestLockdownEventDefaults|TestLockdownNoteFitsReserve|TestAutoEventDefaults|TestScreeningEventDefaults|TestMessagesEndToEnd|TestAutoMessagesEndToEnd|TestSettingsGateAllDoors`

Result: ok, 69.6 s for the package (71.9 s wall), 199 top-level PASS, no FAIL, SKIP or panic.

**Other gates**

| Gate | Result |
|------|--------|
| `make check-translations` | all present (0.3 s) |
| `make check-docs` | no drift (1.4 s) |
| `make test-shard-check` | exit 0 (22 s) |
| `CGO_ENABLED=0 go build ./...` | ok (2.0 s) |

**golangci-lint v2.13.1**

| Run | Result |
|-----|--------|
| `run --new-from-rev=origin/main ./...` | 0 issues (38 s) |
| the same with `--build-tags=notesttools` | 0 issues (21 s) |
| `run --new=false ./fuku/modules/... ./fuku/db/eventmsg/... ./fuku/i18n/...` | 110 baseline findings, the same count as 25-02; none in a touched file |
| `run --tests=false --enable-only=unused ./fuku/modules/...` | 0 issues |

**Full `go test -tags testtools -race -count=1 ./fuku/modules`:** ok, 354.4 s for the package (357 s wall), no FAIL, SKIP or panic. 25-02 measured 357.6 s, so the plan adds no measurable time.

## Known Stubs

None.

## Threat Flags

None.

| Threat | Status |
|--------|--------|
| T-25-06 | Unchanged code: `EditRight` is `settingsRightChangeInfo`, judged on every press. Plan 25-07 adds the authority test. |
| T-25-07 | Covered by the `screens` subtest: a forged `he d=p` on `ld` and `hp` on `lu` get the invalid request alert, and no row is written. |
| T-25-08 | No new string names the Staff Group, a log channel or an AI provider. |
| T-25-SC | No dependency was added. |

## Notes for plan 25-04

- **Page side.** The page side of the automatic notice is done. Defaults are `lockdownAutoDefaultReply(kind)`. The preview samples are:
  - `lj`: 15 joins in 60 s;
  - `lf`: 30 messages in 30 s with 80 %;
  - `la`: 5 verdicts in 120 s.

  Each sample's `{reason}` is `lockdownTriggerSentence(tr, kind, count, window, percent)`. Build the site's `eventValues` the same way:
  - `Reason` = the trigger sentence;
  - `Count`;
  - `Window` as `strconv.Itoa(window)`;
  - `Percent` (0 renders nothing);
  - `Note` = `lockdownStartNotes(tr, lockdownStartFacts(res.BotMember))`, the same notes `lockdownStartAppended` shows.
- **Silent.** A Silent press on `lj`, `lf` or `la` already stores Silent on the group row. The 25-04 site has to honour it: post
  nothing, with the lockdown standing.
- **Default send.** The automatic notice's default send must keep `raidSend`'s request byte for byte (golden `auto_*`). The cannot-lock
  warning shares `raidSend` and must stay untouched.
- **Engine callers.** `raidPostNotice` will name `postLockdownAutoEvent`. Add the adapter and the caller to `eventEngineCallers` and
  `eventLockdownAdapterCallers` with reasons. Check that no never-reach root (`raidWarnCannotLock`, `runLockdownCycle`) reaches it.
- **Owner for site tests.** `TestSettingsMessagesLockdownFamily/typed_and_silent` shows the pattern for driving a lockdown command
  from the settings environment: use `env.newOwner(env.chat)` and `env.fake.setBotMember(...lockdownBotAdminJSON(true, true, true))`.
  The admin from `listEditAdmin` cannot lock.
- **Lift.** For plan 25-05: the lift event's `Appended` is `lockdownLiftAppended`. The site's `Note` should be
  `lockdownLiftNotes(tr, facts)` so the preview and the group message stay one construction.

## Self-Check: PASSED

- Files: `fuku/modules/settings_messages_lockdown_test.go` and this SUMMARY exist.
- Commits: c8ecb66, 45f7bb3 and 1c8c212 are ancestors of HEAD.
- The ledger: `git rev-list --count b3defed..1c8c212` is 3.
