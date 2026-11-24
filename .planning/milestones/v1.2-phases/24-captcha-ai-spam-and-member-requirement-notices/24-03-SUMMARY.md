---
phase: 24-captcha-ai-spam-and-member-requirement-notices
plan: 03
subsystem: event-messages
status: complete
tags: [pm-copies, captcha, memberreq, aispam, locales]
requires:
  - 24-02 (the screening event keys, the captcha site through the engine)
provides:
  - private copy specs for captcha, aispam, mreqwarn, mreqkick, mreqban and mreqmute, all Automatic
  - the six default private texts eventmsg_pm_captcha, _aispam, _mreqwarn, _mreqkick, _mreqban, _mreqmute in 8 languages
  - TestEventPMCaptcha and its toolkit newEventPMCaptchaSite, eventPMCaptchaFail, eventPMCaptchaKicked
affects: [24-04, 24-05, 24-06, 24-07, 24-08]
tech-stack:
  added: []
  patterns:
    - "a private copy spec may exist before its Messages page cell: the length sweep fills the end for every event instead of reading the page's timed flag"
key-files:
  created:
    - fuku/modules/event_message_pm_screening_test.go
  modified:
    - fuku/modules/event_message_pm.go
    - fuku/modules/event_message_pm_test.go
    - fuku/modules/event_message_pm_safety_test.go
    - fuku/i18n/eventmsg_locale_test.go
    - fuku/i18n/memberreq_locale_test.go
    - locales/{en,es,fr,hi,id,pt,ru,uk}.yml
    - AGENTS.md
decisions:
  - "eventmsg_pm_mreqwarn also carries Action: {action} (the plan's draft had none): at the warn limit {action} names the limit action, so without it a member kicked or banned at the limit would read only that they were warned."
  - "The requirement texts say the member does not meet the group's requirements, not that their profile does not: a member who broke only the channel rule must never be told to fix a profile (the M12 rule behind memberReqReworkedKeys)."
  - "The English mreqmute text joins its sentences with a colon to stay at 140 code points, the ceiling every default private text has."
  - "The Ukrainian and Russian AI spam texts are impersonal (видалено, удалено), as 24-02 chose for aispam_notice_deleted; the other languages speak as the bot (I removed)."
metrics:
  duration: ~35 min
  completed: 2026-11-21
actuals:
  tokens: 10300
  tasks: 3
  commits: 3
plan_head_before: ad3544ceb26eef9b57c1e4e076069c644aafc422
plan_head_after: f6e76f1ede838dbd8bacb384232004732e3115cb
---

# Phase 24 Plan 03: Private copies of the screening events Summary

The failed captcha, the AI spam notice and the requirement warn, kick, ban and mute now have private copy specs, all automatic, so
the per-group cap of automatic copies bounds them; each has a default private text in 8 languages that names the group, puts the
action after a label and leaves the reason to the reason line (for a requirement event, the broken rules). Through the real captcha
site, a member who fails the captcha is kicked, the group notice is posted, and only then does the member receive the copy; a burst
in one minute sends at most the capped number while every member is kicked, and without Redis no copy is sent.

## Tasks

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | the six default private texts in 8 languages | ea2bbcc | 8 locales, eventmsg_locale_test.go, memberreq_locale_test.go |
| 2 | the six specs, all automatic | 8dc5063 | event_message_pm.go, event_message_pm_test.go, event_message_pm_safety_test.go, AGENTS.md |
| 3 | the captcha's private copy through its site and the cap | f6e76f1 | event_message_pm_screening_test.go (new), AGENTS.md |

## What later plans use

| Kind | Name | Notes |
|------|------|-------|
| Specs | `captcha`, `aispam`, `mreqwarn`, `mreqkick`, `mreqban`, `mreqmute` → `eventmsg_pm_<event>`, `Automatic: true` | eighteen specs in all; registered after the Phase 23 ones |
| Locale keys | `eventmsg_pm_captcha` (`{group}`, `{action}`), `eventmsg_pm_aispam` (`{group}`, `{action}`), `eventmsg_pm_mreqwarn` (`{group}`, `{count}`, `{limit}`, `{action}`), `eventmsg_pm_mreqkick`, `eventmsg_pm_mreqban`, `eventmsg_pm_mreqmute` (`{group}` only) | rows `{140, 200}` in `eventMsgLocaleLimits`; `eventmsg_pm_mreqmute` in `memberReqQuotingKeys` |
| Toolkit | `newEventPMCaptchaSite(t)`, `eventPMCaptchaFail(t, s, member)`, `eventPMCaptchaKicked(t, s, members...)` | `event_message_pm_screening_test.go`; the site stores the title `Q&A <Group>` in the chats row and switches the captcha copy on |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `TestEventPMTextsFitEveryLocale` failed once the six specs existed**
- **Found during:** Task 2
- **Issue:** the sweep asked the Messages page whether each spec's event is timed (`eventPMTimed`) and failed with "no family of the
  Messages page lists the aispam event": the six events join the page in plan 24-04, after their specs.
- **Fix:** the sweep fills the end (`Until`) for every event. A text without `{until}` ignores it and a timed text is at its longest
  with it, so each event is measured at its worst case, which is equal to or longer than before; `eventPMTimed` (used nowhere else)
  was removed. The sweep also gained the delete word in its search for the longest action word.
- **Files modified:** `fuku/modules/event_message_pm_safety_test.go`
- **Commit:** 8dc5063

Text choices that differ from the plan's English drafts are listed under `decisions` above (the warn's action, "requirements" in
place of "profile", the colon in the mute text).

### Earlier tests whose expected sets changed (no assertion weakened)

1. `fuku/modules/event_message_pm_test.go` `TestEventPMSpecs`: the automatic list gains the six screening events (eighteen specs);
   the three panic cases are unchanged.
2. `fuku/modules/event_message_pm_test.go` `TestEventPMDefaultTexts`: a new subtest `requirement warn` (one language, one case).
3. `fuku/modules/event_message_pm_safety_test.go` `TestEventPMTextsFitEveryLocale`: the end is filled for every event, not only
   the page's timed ones (a larger or equal measurement), and the action words swept gain `delete`. Lost with `eventPMTimed`: its
   incidental check that every spec's event is drawn by a Messages page family (see the notes for 24-04).
4. `fuku/i18n/eventmsg_locale_test.go`: six rows in `eventMsgLocaleLimits` (the test requires a row per `eventmsg_` key).
5. `fuku/i18n/memberreq_locale_test.go`: `memberReqQuotingKeys` gains `eventmsg_pm_mreqmute` (stronger: the text must quote the
   button's words verbatim in every language).

## Verification (with durations)

- Task 1: `make check-translations` all present; `TestEventMsgLocaleKeys|TestMemberReqLocaleKeys|TestLocaleStyle|TestLocalePlaceholdersMatchOtherLanguages`
  4 PASS (7.4 s package, 9.2 s wall). Acceptance greps: 6 in each of the 8 locale files; 6 rows; 1 quoting entry.
- Task 2: the plan's nine tests plus `TestEventPMPrivacy` ok (4.6 s package, 45 s wall with the build). Acceptance greps: 6 specs,
  19 `registerEventPM(` lines.
- Task 3: `TestEventPMCaptcha|TestEventMessageCaptcha|TestEventPMLimits|TestEventPMPacerIndependent` ok (3.3 s package);
  `--- PASS: TestEventPMCaptcha/` 3.
- Goldens: `git diff --exit-code e40a7be -- screening_notice_legacy_request_test.go`, `9d4554d -- punishment_notice_legacy_request_test.go`
  and `e40a7be -- moderation_legacy_request_test.go` exit 0; `git diff --exit-code 86baf1a -- go.mod go.sum main.go migrations` exits 0.
- Touched families with `-race` (178 top-level tests: `TestCaptcha|TestHandleCaptcha|TestEventMessage|TestEventPM|TestPMCopy|TestLegacy|TestMemberReqNotice|TestMemberReqFixed|TestAISpam|TestScreening|TestEventActionWords|TestAutoEvent|TestMemberDeletionExcluded|TestSettingsMessages|TestAutoMessages|TestSettingsStaffPrivacy|TestMessagesEndToEnd|TestPMCopiesEndToEnd`):
  ok, 71.9 s (74 s wall). `./fuku/i18n` full with `-race`: ok, 49.5 s.
- `make check-docs`: no drift (1.3 s). `make test-shard-check`: no diff (23.6 s). `CGO_ENABLED=0 go build ./...`: ok (6.6 s).
- golangci-lint v2.13.1: `run --new-from-rev=origin/main ./...` 0 issues (47.9 s); with `--build-tags=notesttools` 0 issues (20.0 s);
  `run --new=false ./fuku/modules/... ./fuku/i18n/... ./fuku/db/eventmsg/...` (and `notesttools` on `./fuku/modules/...`) filtered to
  the touched files: 0 findings; only the four `captcha.go` baseline findings remain (untouched file).
- Full `go test -tags testtools -race -count=1 ./fuku/modules`: ok, 331.4 s (5 m 34 s wall).
- New tests' time (budget about 3 s): `TestEventPMCaptcha` 0.10 s (delivered 0.03 s, blocked 0.02 s, cap 0.04 s); the
  `requirement warn` subtest of `TestEventPMDefaultTexts` is a pure render (well under 0.01 s).

## Notes for plan 24-04 (and later)

- The six specs exist before their page cells. With `eventPMTimed` gone, nothing checks that every spec's event is drawn by a
  Messages page family; 24-04 may add that check once the Automatic and Requirements families list the six events (it would have
  failed between 24-03 and 24-04 by construction).
- `pmCopyLogText` names an event by `eventPMEventName`, which falls back to the escaped event key until 24-04 lists the six events in
  a family; after 24-04 the `#PM_COPY` line names them by their page names.
- The captcha site (`postCaptchaFailureNotice`) sets `eventSite.Automatic`, so its copies are capped through `eventPMOffer.ByRule` as
  well as through the spec (a mutation of the captcha spec to `Automatic: false` leaves `TestEventPMCaptcha` passing for that reason;
  `TestEventPMSpecs` is what pins the spec's flag). The requirement and AI spam sites should set `Automatic` too, as the captcha does.
- The captcha copy's `{group}` is the chats row's title (`chats.GetChatSettings`); a test of a copy must store a title with
  `chats.EnsureChatInDb` and drop the `chat` cache key when it ends (`newEventPMCaptchaSite` does both).
- `eventmsg_pm_mreqwarn` fills `{count}`, `{limit}` and `{action}`; the 24-05 site must pass `Count` (the member's warns), `Limit`
  (the group's limit) and `Action` (the warn word, or the limit action's word when the limit action took effect) in `eventValues`,
  and `Reason` as `memberReqRulesText` so the reason line lists the broken rules.
- No new caller of `postEvent`, `postEventSent`, `offerEventPM` or `registerEventPM` was added, so `TestPMCopyCallers` is unchanged;
  24-05 (`memberreq_notice.go`) and 24-06 (`aispam_notice.go`) add their allow-list entries with reasons.
- `Admit` is untouched here: the captcha site has none. For 24-06, `Admit` is asked once, after the read, only when the group message
  or the private copy would be sent, so a refusal also sends no private copy.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register. T-24-07: all six specs are automatic and `TestEventPMCaptcha` proves the cap and the fail-closed path
while every member is kicked. T-24-08: the six texts name only `<b>{group}</b>` (`TestEventMsgLocaleKeys`), and `TestEventPMPrivacy`
sweeps every spec's default and custom copy for the Staff Group and another group in 8 languages (it ran with the six new specs).
T-24-09: `composeEventPM` carries no bot row; the mute text only tells the member where the button is. T-24-SC: no dependency.

## Self-Check: PASSED

- Files: `fuku/modules/event_message_pm_screening_test.go` exists; every modified file is in the three commits.
- Commits: ea2bbcc, 8dc5063 and f6e76f1 are ancestors of HEAD; `git rev-list --count ad3544c..f6e76f1` is 3. A `docs(24): pause work
  during plan 24-03` commit (2b1fc52, `.planning/HANDOFF.json` and `.continue-here.md`) was made on the branch by the orchestrating
  session while the gates ran; it is not this plan's and is not counted.
