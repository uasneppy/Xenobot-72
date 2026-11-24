---
phase: 24-captcha-ai-spam-and-member-requirement-notices
plan: 08
subsystem: screening-messages-e2e
status: complete
tags: [event-messages, captcha, turnstile, memberreq, aispam, pm-copies, e2e, tests]
requires:
  - 24-01 (the screening goldens)
  - 24-02 (the captcha site and the engine hooks)
  - 24-03 (the six private copy specs)
  - 24-04 (the Messages page rows, the Member requirements family, the Notice switch)
  - 24-05 (the requirement site)
  - 24-06 (the AI spam site and its caps)
  - 24-07 (the safety tests; their notes for 24-08)
provides:
  - TestScreeningMessagesEndToEnd and its toolkit screeningMessagesE2E
affects: [phase 24 verification]
requirements-completed: [EVT-04, EVT-05, EVT-09]
tech-stack:
  added: []
  patterns:
    - "an end-to-end test that needs the web captcha builds its runtime over the settings environment's own bot and Redis, so the owner's page presses and the verify route meet in one group"
key-files:
  created:
    - fuku/modules/screening_messages_e2e_test.go
  modified:
    - AGENTS.md
decisions:
  - "The captcha timeout is driven by moving the attempt's created_at and expires_at back together (chk_captcha_expires_at requires expires_at > created_at) and then calling expireCaptchaAttempt, the timer's own path."
  - "captchaFailureNoticeLife is 20 ms in every subtest (set in the constructor), so no in-process self-delete timer outlives a subtest."
  - "The AI notice's 'two more notices' is checked with the minute clock stopped (aispamNoticeNow); the per-member window is a real miniredis TTL, so the third spam inside five minutes needs no FastForward."
  - "The web check adds a held message, so the custom text reads (1): a held count of 0 renders as nothing, as AGENTS.md documents."
  - "The captcha private copy cap runs in a stopped pmCopyNow minute that differs from the real minute in which the requirement mute's copy was counted, so the two never share a counter."
metrics:
  duration: ~45 min
  completed: 2026-11-22
actuals:
  tokens: 9600
  tasks: 2
  commits: 2
plan_head_before: 22255ef253a258b58603f7ae39c1d6412a81d368
plan_head_after: 0e0823db9a4503408b1fb3d2b92cc0e2f810ca1c
---

# Phase 24 Plan 08: The screening messages end to end Summary

One end-to-end test now drives the whole of Phase 24 through the real dispatcher. Only the Bot API, Cloudflare's siteverify and TypeSafe are faked.
The owner sets the failed captcha's text, the requirement mute's text and button, the requirement warn's text and the AI spam notice by pressing the
buttons the bot drew in the private Messages page, typing each part in private. A real `chat_member` join meets the captcha and times out, a Turnstile
check fails on the verify route, members without a last name break the requirement and tap "I fixed it", and spam is judged and deleted. The test
found no production defect, so no production file changed.

## Tasks

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | the phase end to end through the real dispatcher | cfb80c9 | fuku/modules/screening_messages_e2e_test.go (new) |
| 2 | the phase-wide gates, the full package run and the AGENTS.md pass | 0e0823d | AGENTS.md |

## What the test holds

| Subtest | Success criterion | What it proves |
|---------|-------------------|----------------|
| `owner sets the captcha message` | 1 (EVT-04) | typed text stored, the hub reads Custom; the join's math challenge carries its own text; the held message is deleted; the timeout kicks first, then posts `<b>{mention}</b> {action} ({count})` with the kick word and 1, no reply, no queue row; the notice is deleted after its life |
| `web check` | 1 (EVT-04) | a Turnstile join, a held message, a failed verify answered `failed`/`kick` as before, the custom text posted once; the same verify again replays the answer and posts nothing |
| `requirement mute` | 2 (EVT-09) | no media part and no Silent switch on the page; muted first, then the custom text, a blank line and the requirements line, with the link button then "I fixed it" and a 24 hour `bot:punish_auto` row; after a last name is added the tap 20 minutes later unmutes, edits the notice to the fix text with no keyboard and moves its row to the tap plus 900 s |
| `requirement warn` | 2 (EVT-09) | a custom text with `{reason}` followed by the requirements line, one warn stored, a 900 s row |
| `ai notice` | 3 (EVT-05) | nothing stored: deleted, nothing posted, no `fuku:aispam:notice:*` key; the drawn Notice switch turns it on; the next spam is deleted then noticed, the third inside five minutes is deleted quietly, three other members' spam is all deleted with two more notices (three in the stopped minute) |
| `nothing stored` | 4 | real locales: a captcha kick, a requirement kick and a requirement mute equal `goldenScreenCaptchaKick`, `goldenScreenReqKick` and `goldenScreenReqMute` with the case's IDs put in, each with its queue state |
| `refused` | 4 | real locales: a custom captcha photo refused with a 400 comes after the kick and is followed by the golden default once |
| `pm copies` | 1, 2 | the requirement mute's private copy, switched on from its private copy screen, sends the default copy with the broken rule on the reason line; the captcha's copy on with the cap at 2: three joiners fail, all kicked and noticed, exactly two copies |

Time: the test takes 1.1 s to 1.3 s inside a family run and 2.8 s alone (the first real-locale load), against the plan's budget of about 6 s.
Four runs with `-count=4` took 7.3 s together with no failure.

Mutation checks (each reverted, `git status` clean afterwards):
- an `Admit` that always allows fails `ai notice` (the third spam and a fourth notice post);
- dropping the requirement site's `KeepRows`/`KeepRowsLast` fails `requirement mute` ("I fixed it" moves above the custom button);
- a private copy cap that always refuses fails `pm copies`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] chk_captcha_expires_at refused moving only the expiry**
- **Found during:** Task 1
- **Issue:** the plan drives the timeout "on an attempt whose expiry was moved into the past"; updating `expires_at` alone violates
  `expires_at > created_at`.
- **Fix:** the helper moves `created_at` a minute further back in the same update. Test-only.
- **Commit:** cfb80c9

### Plan differences, recorded

- The plan's behaviour list names a 600 s row for the requirement warn. The environment is Phase 23's toolkit, whose automatic timer is
  900 s, so the warn's row is checked at 900 s. The delay is not part of the comparison with a golden.
- The plan names a `spam(member)` helper and a `breakRule(member, rule, action)` helper. `breakRule` takes only the action, because every
  subtest uses the last-name rule.
- AGENTS.md's Testing entry for the test went into the `docs(24-08)` commit the plan names (0e0823d), not the test's own commit (cfb80c9).
  The rule that AGENTS.md changes share the commit they describe was broken because I committed the test before writing the entry, and
  commits are never amended.
- The AGENTS.md pass found no sentence that the code contradicts. I checked all 333 identifiers in the Phase 24 sentences against the
  repository (all exist; the only misses are the prefixes `goldenScreen*`, `settings_msg_ev_*` and `settings_msg_desc_*`). I also checked
  the caps, the pacer options, the 600-unit reserve, the off-by-default rule, the requirement site's hooks and the 24 hour cap against
  `aispam_notice.go`, `memberreq_notice.go` and `event_message_auto.go`.

### Earlier tests whose expected values changed

None. No earlier test was edited. The goldens are unchanged:
- `git diff --exit-code e40a7be -- fuku/modules/screening_notice_legacy_request_test.go` exits 0, and that file has one commit (its own).
- `git diff --exit-code 9d4554d -- fuku/modules/punishment_notice_legacy_request_test.go` exits 0.
- `git diff --exit-code 86baf1a -- fuku/modules/moderation_legacy_request_test.go fuku/modules/punishment_notice_legacy_request_test.go` exits 0.

## Verification (with durations)

- Task 1 verify: 8 tests (`TestScreeningMessagesEndToEnd`, the three earlier end-to-end tests, the three goldens and
  `TestCaptchaWebTracerJoinVerifyUnmute`) passed in 8.0 s package time and 47 s wall time. There were 8 `--- PASS: TestScreeningMessagesEndToEnd/` lines
  and no SKIP.
- Touched families with `-race` (`TestScreeningMessagesEndToEnd`, the end-to-end tests, `TestLegacy.*`, `TestEventMessage.*`,
  `TestEventPM.*`, `TestPMCopy.*`, `TestAISpamNotice`, `TestCaptchaWeb.*`, `TestMemberReqFixed.*`, `TestMemberReqNotice.*`,
  `TestSettingsMessages.*`, `TestScreeningEventDefaults`): 132 top-level tests passed in 45.2 s package time and 48 s wall time.
- `make check-translations`: all present (0.3 s). `make check-docs`: no drift (2 s). `make test-shard-check`: clean (24 s).
  `CGO_ENABLED=0 go build ./...`: ok (2 s).
- `go test -tags testtools -race -count=1 ./fuku/db/eventmsg ./fuku/db/cache`: ok (4 s). The `./fuku/i18n` list (`TestEventMsgLocaleKeys`,
  `TestMemberReqLocaleKeys`, `TestSettings.*`, `TestLocaleStyle`, `TestLocalePlaceholdersMatchOtherLanguages`,
  `TestHelpPagesKeepTheirCommands`): ok (27.8 s).
- golangci-lint v2.13.1:
  - `run --new-from-rev=origin/main ./...`: 0 issues.
  - The same with `--build-tags=notesttools`: 0 issues (23 s).
  - `run --new=false ./fuku/modules/... ./fuku/db/eventmsg/... ./fuku/i18n/...` with no issue cap: 110 baseline issues, the same count as 24-07.
    The plan's filter matches only the four `captcha.go` findings of 86baf1a (`captchaVerifyCallback` 28, `captchaRefreshCallback` 40,
    `handlePendingCaptchaMessage` 24, QF1003). It matches nothing in `handleCaptchaTimeout`, `processAISpam`, `memberReqAnnounce` or the new
    test file.
- `git diff --exit-code 86baf1a -- go.mod go.sum main.go migrations` exits 0. No dated `first production caller arrives in plan 24-` note
  is left (count 0). AGENTS.md names `TestScreeningMessagesEndToEnd` and `TestLegacyScreeningNoticeRequests`.
- Full `go test -tags testtools -race -count=1 -timeout 45m ./fuku/modules`: ok, 353.5 s package time (356 s wall time). Lint and the shard check
  overlapped its first minute. That is well inside the 39 minute line.

Phase 24's new tests against the phase's 30 s budget, by the summaries' measurements (`-race`): 24-02 `TestEventMessageCaptcha` and the hooks
(under 1 s), 24-03 `TestEventPMCaptcha` 0.1 s, 24-04 `TestSettingsMessagesScreeningFamily` about 2 s, 24-05 the requirement site tests under
1 s, 24-06 `TestAISpamNotice` about 1.5 s, 24-07 about 0.9 s, 24-08 about 1.3 s (2.8 s cold). Together that is well under 10 s.

## Notes for the phase verifier

- `make test-postgres-integrity` is left to the verifier. This phase adds no PostgreSQL guarantee (D-41).
- `TestScreeningMessagesEndToEnd` covers all four success criteria. The goldens are byte-identical to their own commits.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register:
- T-24-29: `requirement mute` taps the kept button and is unmuted.
- T-24-30: `ai notice` checks the per-member and per-minute caps, and every message is deleted.
- T-24-31: `owner sets the captcha message`, `web check` and `TestCaptchaWebTracerJoinVerifyUnmute` cover the captcha.
- T-24-SC: no dependency change.

## Self-Check: PASSED

- Files: `fuku/modules/screening_messages_e2e_test.go` exists, and AGENTS.md carries the entry.
- Commits: cfb80c9 and 0e0823d are ancestors of HEAD. `git rev-list --count 22255ef..HEAD` before this summary is 2.
