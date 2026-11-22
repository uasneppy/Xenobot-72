---
phase: 24-captcha-ai-spam-and-member-requirement-notices
plan: 04
subsystem: settings-messages
status: complete
tags: [settings, messages-page, captcha, aispam, memberreq, editor, locales]
requires:
  - 24-02 (the screening default builders, memberReqNoticeNote and its reserve, OffByDefault and Resolve)
  - 24-03 (the six private copy specs)
provides:
  - the failed captcha (cf 🧩) and AI spam notice (ai 🧠) rows of the Automatic family
  - the Member requirements family (r 🛂) with rw ❗, rk 👢, rb ⛔ and rm 🔇, NoMedia and ReservedText 600
  - settingsEditPageDef.ReservedText, held back by the text check, the caption check and the prompt's "Up to" number
  - the AI spam notice's Notice switch (off by default) and the Off state on the hub, the family list and Main
  - NoSilent (the requirement mute cannot be silenced), Appended (the requirement note after a custom text in a preview), Sample.Rules
  - memberReqNoticeAppended, memberReqNoticeWarnOf and memberReqRuleBits in memberreq_notice.go
  - 19 new settings_ locale keys and 3 reworded ones in 8 languages
  - TestSettingsMessagesScreeningFamily and TestSettingsEditReservedText
affects: [24-05, 24-06, 24-07, 24-08]
tech-stack:
  added: []
  patterns:
    - "an off-by-default event reads Off through Resolve and draws a Notice switch that writes the same Silent column with the opposite meaning"
    - "an editor definition reserves room for what the bot appends after a custom text (ReservedText), zero for every earlier definition"
    - "expected screen widths of the Messages page tests are computed from the definition's parts, NoSilent and the spec, not hand-written per event"
key-files:
  created:
    - fuku/modules/settings_messages_screening_test.go
  modified:
    - fuku/modules/settings_messages.go
    - fuku/modules/settings_edit.go
    - fuku/modules/settings_edit_check.go
    - fuku/modules/settings_edit_actions.go
    - fuku/modules/event_message_auto.go
    - fuku/modules/memberreq_notice.go
    - fuku/modules/settings_messages_test.go
    - fuku/modules/settings_messages_pm_options_test.go
    - fuku/modules/settings_messages_screens_test.go
    - fuku/modules/settings_messages_auto_test.go
    - fuku/modules/event_message_pm_safety_test.go
    - fuku/i18n/settings_locale_test.go
    - fuku/i18n/memberreq_locale_test.go
    - locales/{en,es,fr,hi,id,pt,ru,uk}.yml
    - AGENTS.md
decisions:
  - "The family's name is the settings page's own word (Requirements, Вимоги, Требования, ...), not 'Member requirements': '🛂 Member requirements' is 21 code points, over the 20 of a cell (D-42 and the plan's own label check). The hub heading and the family list read the same word."
  - "The requirement event names are the plain punishment words (Warning, Kick, Ban, Mute) the other families use; the family heading gives the context. The #PM_COPY log line therefore names a requirement mute 'Mute' like a manual mute (see notes)."
  - "A NoSilent event ignores a stored Silent in its state (the hub, the list, Main), as the send site's Required does, so a hand-written silent row is never drawn as Silent."
  - "settingsMsgRunSilent refuses a NoSilent event too (beside settingsMsgSilentFields), so a press cannot silence the mute even if one check is bypassed."
  - "The no-caption note on an event screen is drawn only for a definition that has a media part, so a NoMedia screen never mentions a media file a hand-written row might carry."
  - "The intro reword drops 'in the group's language' from the 'With nothing set' sentence in every language to keep the English within its 330 limit; the Automatic intro shortens its last sentence ('A blocked source is never named') for the same reason. No limits row was raised."
  - "The Automatic text prompt's literal token table gains {until} and a token table for the requirement prompt was added (stronger checks)."
metrics:
  duration: ~70 min
  completed: 2026-11-22
actuals:
  tokens: 45700
  tasks: 3
  commits: 4
plan_head_before: a77a827d5f529cfdad5227b813381cac0ce2ec72
plan_head_after: bafded769ec315edf984ff9ac2b8a57ded241621
---

# Phase 24 Plan 04: The screening events on the Messages page Summary

The Messages page now lists the failed captcha and the AI spam notice in its Automatic family and a new Member requirements family
(Requirements) with the requirement warn, kick, ban and mute. Each opens the v1.1 editor with its group message, its private copy and
its switch; the AI spam notice is off until the owner turns it on with its own Notice switch (the hub reads Off and Main does not count
it meanwhile); the requirement events take text and buttons only and their texts are held to 3496 characters so the bot's requirements
line and remedies always fit; the requirement mute has no Silent switch and a forged press naming it is an invalid request; a Full
preview of a requirement event shows the custom text followed by the requirements line, never the I fixed it button.

## Tasks

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 0 | resume: delete the pause handoff | bea9814 | .planning/HANDOFF.json, the phase's .continue-here.md |
| 1 | the strings in 8 languages | 02c21af | 8 locales, settings_locale_test.go, memberreq_locale_test.go |
| 2 | the rows, the family, the reserve, the Notice switch, the page's shapes | 2c2aea6 | settings_messages.go, settings_edit*.go, event_message_auto.go, memberreq_notice.go, five page tests, event_message_pm_safety_test.go, AGENTS.md |
| 3 | the proof through the real menu | bafded7 | settings_messages_screening_test.go (new), AGENTS.md |

## What later plans use

| Kind | Name | Notes |
|------|------|-------|
| Page rows | `cf` 🧩 `captchaFailureDefaultReply` sample kick, 2 removed, `eventmsg_reason_captcha`; `ai` 🧠 `aispamNoticeDefaultReply` sample delete, `eventmsg_reason_aispam` | appended to `settingsMsgAutoEvents` |
| Family | `r` 🛂 `settings_msg_fam_mreq`, `NoMedia`, `ReservedText: memberReqNoticeNoteReserve`, events `rw rk rb rm` | `rm` has `ReservedButtons: 1` and `NoSilent` |
| Builders | `memberReqNoticeAppended(action)` (the note with the default's template key), `memberReqNoticeWarnOf(action, in)`, `memberReqRuleBits(codes...)` | `memberreq_notice.go` |
| Editor | `settingsEditPageDef.ReservedText` | 0 for every earlier definition and for every private copy definition |
| Locale keys | the 19 of the plan's table, plus the rewording of `settings_msg_intro`, `settings_msg_fam_auto_intro`, `settings_msg_wait_text_auto` (now lists `{until}`) | rows in `settingsLocaleLimits`, label groups, literal tokens, quoting check |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] The family name would not fit its cell**
- **Found during:** Task 1
- **Issue:** the plan's English `Member requirements` makes the cell `🛂 Member requirements` 21 code points, over the 20 the plan
  itself requires.
- **Fix:** the family is named with the settings page's own word in each language (`settings_page_mreq`: Requirements, Requisitos,
  Exigences, आवश्यकताएँ, Persyaratan, Requisitos, Требования, Вимоги).
- **Files modified:** locales/*.yml
- **Commit:** 02c21af

**2. [Rule 1 - Bug] The no-caption note on a NoMedia screen**
- **Found during:** Task 2
- **Issue:** `settingsMsgPartLines` drew the sticker/video-note note whenever the state carried such a media, even on a definition with
  no media part, which would mention a media file the requirement site never sends (TextOnly).
- **Fix:** the note is drawn only when the definition has a media part. Every earlier definition has one, so nothing earlier changes.
- **Files modified:** fuku/modules/settings_messages.go
- **Commit:** 2c2aea6

**3. [Plan note restored] The page check that every private copy spec's event is listed by a family**
- **Found during:** Task 2 (binding note of 24-03)
- **Fix:** `TestEventPMTextsFitEveryLocale` again fails when a spec's event is listed by no Messages family (a direct check, in place
  of the incidental one `eventPMTimed` gave before 24-03).
- **Files modified:** fuku/modules/event_message_pm_safety_test.go
- **Commit:** 2c2aea6

### Earlier tests whose expected values changed (no assertion weakened)

1. `settings_messages_test.go` `TestSettingsMessagesOpens`: hub widths `[2 1 1 2]` → `[2 1 1 1 2]`, Back read from row 5 (was 4), and
   the hub must also carry `settings_msg_fam_mreq` (added).
2. `settings_messages_pm_options_test.go` `TestSettingsMessagesPMOptions`: the hub and redrawn hub widths `[2 1 1 2]` → `[2 1 1 1 2]`.
3. `settings_messages_screens_test.go` `settingsMsgScreens`: the hub widths `[2 1 1 1 2]`; `settingsMsgFamilyWidths` `a` → `[2 2 2 1 2]` and
   new `r` `[2 2 2]`; the event and private copy screen widths are computed by the new `settingsMsgScreenWidths` from the definition's
   parts, `NoSilent` and the spec, which yields exactly the old literals for the three-part families.
4. `settings_messages_auto_test.go` `TestSettingsMessagesAutoFamily`: `settingsMsgWantAutoStates` reads Off (`common_status_off`) for an
   off-by-default event; hub widths `[2 1 1 1 2]`; the Automatic list `[2 2 2 1 2]` with Back on row 5; the screen loop expects
   `settings_msg_line_notice` on the AI spam screen in place of `settings_msg_line_silent` and now also fails when the other line is
   present (added).
5. `fuku/i18n/settings_locale_test.go`: new limits rows, label groups and token entries only; `TestSettingsReusedKeyPlaceholders` admits
   the requirement prompt's literal `{id}` beside the manual and Automatic prompts'.
6. `fuku/i18n/memberreq_locale_test.go`: `memberReqQuotingKeys` gains the three texts that name the I fixed it button (stronger).

No golden changed: `git diff --exit-code e40a7be -- fuku/modules/screening_notice_legacy_request_test.go`,
`git diff --exit-code 9d4554d -- fuku/modules/punishment_notice_legacy_request_test.go` and
`git diff --exit-code HEAD -- fuku/modules/moderation_legacy_request_test.go` all exit 0.

## Verification (with durations)

- Task 1: `make check-translations` all present; `TestSettings.*|TestMemberReqLocaleKeys|TestLocaleStyle|TestLocalePlaceholdersMatchOtherLanguages`
  on `./fuku/i18n` PASS (13.2 s package, 14.5 s wall); whole `./fuku/i18n` PASS (49.6 s). Acceptance: 19 keys in each of the 8 files;
  12 matching rows (at least 7 required).
- Task 2: the plan's family run (`TestSettingsMessages.*`, drift, authority, editor, Main, `TestSettingsStaffPrivacy`,
  `TestEventPMTextsFitEveryLocale`, `TestScreeningEventDefaults`, `TestAutoEventDefaults`) PASS (36.9 s package, 76 s wall). The event
  message, PM copy, Legacy, end-to-end and editor families (`TestEventMessage.*|TestEventPM.*|TestPMCopy.*|TestLegacy.*|TestMemberReqNotice.*|TestMessagesEndToEnd|TestPMCopiesEndToEnd|TestAutoMessagesEndToEnd|TestSettingsEdit.*|TestSettingsGateAllDoors`)
  PASS (35.0 s package, 75 s wall). Acceptance greps: 1 family row, 6 event codes, `ReservedText` in all three editor files, no 24-04
  marker left, `[2 1 1 1 2]` present and `[2 1 1 2]` absent in the screens test.
- Task 3: `TestSettingsMessagesScreeningFamily` 4 subtests PASS (1.90 s: hub and lists 0.42 s, requirement screens 0.02 s, ai notice
  0.05 s, previews 1.41 s, most of it the real locales) and `TestSettingsEditReservedText` PASS (0.02 s); about 1.9 s of new test time
  against the plan's 3 s. Mutation check: removing the reserve from the check, or the note from the preview, fails the new tests.
- `make check-docs` no drift (1.3 s); `make test-shard-check` clean (23 s); `CGO_ENABLED=0 go build ./...` ok (6.9 s);
  `git diff --exit-code 86baf1a -- go.mod go.sum main.go migrations` exit 0.
- golangci-lint v2.13.1: `--new-from-rev=origin/main ./...` 0 issues (50 s); the same with `--build-tags=notesttools` 0 issues (24 s);
  `--new=false ./fuku/modules/... ./fuku/i18n/... ./fuku/db/eventmsg/...` with no issue cap: no finding in any touched file.
- Full `go test -tags testtools -race -count=1 ./fuku/modules`: ok, 346.9 s package (5 m 49 s wall).

## Notes for plan 24-05 (the requirement site)

- The site should build its reply with `TextOnly: true` for all four requirement events (the page offers no media part, but a row written
  by hand could carry a file), `Required: true` for the mute, `KeepRows` the I fixed it rows with `KeepRowsLast`, and `Note` from
  `memberReqNoticeAppended(action)(tr, in)` (the same function the page's preview uses, so the preview and the group message cannot
  differ); `in.Rules` the broken mask, `in.Count`/`in.Limit`/`in.Action` as the default builder reads them. Set `Automatic` as the captcha
  site does.
- The page reads a NoSilent event's stored Silent as not silent, matching `Required`; the page never writes Silent for `mreqmute`.
- The editor holds a requirement text to 3496 units, but texts stored before or by other means are not; the site's refusal fallback to the
  default still covers a text Telegram refuses.
- `memberReqRuleBits` turns rule codes into a mask if a test needs sample rules.
- The `#PM_COPY` log line now names the six events by their page names; the requirement events share the plain words Warning, Kick, Ban,
  Mute with other families (P1 is provisional, the owner may prefer distinct names).

## Notes for plan 24-06 (the AI spam site)

- `aispamEventReason` keeps its `//nolint:unused // first production caller arrives in plan 24-06` marker; the page uses the key name, not
  the function.
- The Notice switch writes the aispam group row's Silent through `applyEventSilent` (false is on); the site must read `Resolve`, which with no
  row reads Silent (off).

## Known Stubs

None.

## Threat Flags

None. T-24-10: the new `k` values go through the unchanged `settingsMsgDefByFields` and rights (plan 24-07 extends the forged-press and
authority tests). T-24-11: `NoSilent`, `settingsMsgSilentFields` and `settingsMsgRunSilent` refuse silencing the mute; the
`requirement screens` subtest asserts the refusal. T-24-12: `ReservedText` in the check and the prompt; `TestSettingsEditReservedText`.
T-24-13: the preview reply has no options and no `KeepRows`; the `previews` subtest asserts no keyboard. T-24-14: the new strings name no
group; `TestSettingsStaffPrivacy` renders every screen of the three families. T-24-SC: go.mod and go.sum unchanged.

## Self-Check: PASSED

- Files: `fuku/modules/settings_messages_screening_test.go` exists; every modified file is in the four commits.
- Commits: bea9814, 02c21af, 2c2aea6 and bafded7 are ancestors of HEAD; `git rev-list --count a77a827..HEAD` before this summary is 4.
