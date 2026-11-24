---
phase: 24-captcha-ai-spam-and-member-requirement-notices
plan: 02
subsystem: event-messages
status: complete
tags: [event-engine, captcha, memberreq, aispam, locales]
requires:
  - 24-01 (TestLegacyScreeningNoticeRequests and its goldens)
provides:
  - the captcha event site (postCaptchaFailureNotice) through the timer and the web check
  - eventSite.SendCustom, Schedule, Admit; eventReply.Required, TextOnly; eventDefaultInput.Rules; postEventSent
  - eventmsg.EventCaptcha, EventAISpam, EventReqWarn, EventReqKick, EventReqBan, EventReqMute; OffByDefault, DefaultSilentFor
  - captchaFailureDefaultReply, memberReqNoticeDefault, memberReqNoticeNote, memberReqNoticeNoteReserve, aispamNoticeDefaultReply
  - captchaEventReason, aispamEventReason, the delete action word
affects: [24-03, 24-04, 24-05, 24-06, 24-07, 24-08]
tech-stack:
  added: []
  patterns:
    - "an optional site hook whose zero value is the earlier behaviour (SendCustom, Schedule, Admit), so every earlier golden runs unchanged"
    - "a site with its own deletion sets Schedule; the captcha notice keeps its in-process 30 s delete and never enters the queue"
    - "an event can be off by default through DefaultSilentFor, used by Resolve and the part writers' insert default"
key-files:
  created:
    - fuku/modules/event_message_screening_test.go
  modified:
    - fuku/db/eventmsg/events.go
    - fuku/db/eventmsg/repository.go
    - fuku/db/eventmsg/repository_test.go
    - fuku/modules/event_message.go
    - fuku/modules/event_message_auto.go
    - fuku/modules/event_message_auto_test.go
    - fuku/modules/captcha.go
    - fuku/modules/memberreq_notice.go
    - fuku/modules/pm_copy_callers_test.go
    - fuku/i18n/eventmsg_locale_test.go
    - locales/{en,es,fr,hi,id,pt,ru,uk}.yml
    - AGENTS.md
decisions:
  - "The Admit gate is also asked on the read-error path of an event that is not off by default, right before its default is sent: the gate means 'a message is about to be posted', and no current site has both an Admit and an on-by-default event."
  - "TextOnly drops the stored media before the content test, so a row whose only content is a file reads as no content and sends the default through the site's own send."
  - "aispamNoticeDefaultReply mentions the member's full name with formatting.MentionHtml (as the flood and blacklist defaults do); the plan left the mention form open."
  - "The Ukrainian and Russian aispam_notice_deleted texts are impersonal (Видалено ..., Удалено ...), so the bot's voice carries no grammatical gender."
  - "aispamEventReason has no caller in plan 24-04 (the page table uses the key name), so its nolint marker names plan 24-06, the AI site."
metrics:
  duration: ~25 min
  completed: 2026-11-21
actuals:
  tokens: 23200
  tasks: 3
  commits: 3
plan_head_before: bd2c02a9c55c81b03b2e2c653ebab97e96e25491
plan_head_after: b23b4735125eac9c937fd063e007cfca877d57a2
---

# Phase 24 Plan 02: Captcha tracer, engine hooks and screening builders Summary

A failed captcha now posts the group's custom message through the event engine, by timeout and by a failed web check alike,
with today's 30 second self-delete and no queue row; the engine gained the optional hooks the requirement and AI spam sites need
(`SendCustom`, `Schedule`, `Admit`, `Required`, `TextOnly`, `Rules`, `postEventSent`), the five other screening keys exist with
the AI spam notice off by default, and the captcha, requirement and AI spam default builders reproduce today's requests byte for
byte against the 24-01 goldens.

## Tasks

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | the failed captcha's message end to end, through the timer and the web check | 8038f21 | events.go, repository_test.go, event_message.go, event_message_auto.go, captcha.go, event_message_screening_test.go, pm_copy_callers_test.go, AGENTS.md |
| 2 | the remaining keys, the off-by-default rule, the site hooks and the words | cff641f | events.go, repository.go, repository_test.go, event_message.go, event_message_auto.go, event_message_auto_test.go, event_message_screening_test.go, eventmsg_locale_test.go, 8 locales, AGENTS.md |
| 3 | the requirement and AI spam default builders and the requirement note's reserve | b23b473 | memberreq_notice.go, event_message_auto.go, event_message_screening_test.go, AGENTS.md |

## What later plans use

| Kind | Name | Notes |
|------|------|-------|
| Engine | `eventSite.SendCustom`, `Schedule`, `Admit`; `eventReply.Required`, `TextOnly`; `eventDefaultInput.Rules`; `postEventSent(site, r) (eventGroupPost, error)` | nil hooks are today's behaviour; `Admit` is asked once, after the read, only when the group message or the private copy would be sent (`eventGroupWouldPost`, `eventPMWouldOffer`), with `context.Background()` (the site bounds its own call) |
| Engine | `eventGroupMessage(r, msgs)` | the group step's row: `Resolve`, media dropped for `TextOnly`, silent unless `Required` |
| Read rule | a read error of an `OffByDefault` event posts nothing (one Warn); other events send the default after `Admit` | |
| Keys | `eventmsg.EventCaptcha` `captcha`, `EventAISpam` `aispam`, `EventReqWarn` `mreqwarn`, `EventReqKick` `mreqkick`, `EventReqBan` `mreqban`, `EventReqMute` `mreqmute` | appended to `Events` in that order |
| Off by default | `eventmsg.OffByDefault(event)` (aispam only), `eventmsg.DefaultSilentFor(event, dest)` | `Resolve` and `upsert` use it; `SetText` on a fresh aispam group key stores `silent` true |
| Captcha site | `postCaptchaFailureNotice(bot, chatID, tr, target, action, stored) error`, `captchaFailureDefaultReply`, `captchaFailureNoticeLife` (30 s variable), `captchaFailureNoticeDelete(bot, chatID)` | `captcha.go` calls only `postCaptchaFailureNotice` |
| Builders | `memberReqNoticeDefault(action) eventDefaultReply`, `memberReqNoticeNote(tr, broken, key) string`, `memberReqNoticeNoteReserve` (600), `aispamNoticeDefaultReply` | each carries `//nolint:unused // first production caller arrives in plan 24-04` |
| Words | `eventActionWord(tr, "delete")`, `captchaEventReason(tr)`, `aispamEventReason(tr)` | `aispamEventReason` carries the marker naming plan 24-06 |
| Locale keys | `eventmsg_action_delete`, `eventmsg_reason_captcha`, `eventmsg_reason_aispam`, `eventmsg_mreq_rules` (`{rules}`), `aispam_notice_deleted` (`{name}`) | all 8 files; the four `eventmsg_` keys have rows in `eventMsgLocaleLimits` |
| Tests | `TestEventMessageCaptcha`, `TestEventMessageEngineHooks`, `TestScreeningEventDefaults`, `TestMemberReqNoticeNoteFitsReserve` | `event_message_screening_test.go`; toolkit `eventMsgCaptchaSite`, `eventMsgKickedFirst`, `eventMsgHookSite`, `eventMsgHookReply`, `eventMsgHookTexts`, `screenDefaultWant`, `screenNoticeInput` |

## Deviations from Plan

None in production behaviour. Small choices the plan left open are recorded under `decisions` above.

### Earlier tests whose expected sets changed (no assertion weakened)

1. `fuku/db/eventmsg/repository_test.go` `TestEventMessages/the event and destination tables`: the expected `Events` list gains
   `captcha` (Task 1) and `aispam mreqwarn mreqkick mreqban mreqmute` (Task 2); the refused-words list gains `Captcha`, `captchas`,
   `AISpam`, `mreq`, `mreqwarns`, `mreq_ban` (stronger). `TestEventMessagesPM` gains a new subtest `an event off by default`.
2. `fuku/modules/event_message_auto_test.go` `TestEventActionWords`: `delete` moves from the inert list to the action table
   (`eventmsg_action_delete`), the inert list gains `Delete` and `deleted`, the distinct-word count follows the table, and the reason
   check covers `captchaEventReason` and `aispamEventReason` beside `floodEventReason`.
3. `fuku/modules/pm_copy_callers_test.go` `TestPMCopyCallers`: `event_message_auto.go` joins the `postEvent` allow-list (the captcha
   failure's notice, for `captcha.go`, which stays free of the engine), and `postEventSent` is tracked (allow-list: `event_message.go`,
   where `postEvent` wraps it) and forbidden in every staff, lockdown and raid file.

## Verification (with durations)

- Task 1 verify (16 tests incl. the three goldens and the web captcha tests): ok, 5.0 s; `TestEventMessages` (eventmsg): ok, 1.1 s.
- Task 2 verify: modules ok 4.6 s; `./fuku/db/eventmsg` ok 1.1 s; `TestEventMsgLocaleKeys|TestLocaleStyle|TestLocalePlaceholdersMatchOtherLanguages` ok;
  `make check-translations` all present (0.4 s).
- Task 3 verify: ok 3.8 s; `TestScreeningEventDefaults` 13 PASS subtests (11 goldens, the AI spam notice, the text-equality sweep).
- Acceptance greps: `func postEventSent(` 1; `postCaptchaFailureNotice(` in captcha.go 1; `postEvent|eventSite|eventReply|scheduleBotMessage|deletionSchedule`
  in captcha.go 0; `TestEventMessageCaptcha/` PASS 6; six `Event... = "` keys; `DefaultSilentFor(` in repository.go 4; the five locale
  keys 5 in each of 8 files; `msgs.Lookup(r.Event` 0; builder greps 2 and 2.
- Goldens: `git diff --exit-code e40a7be --` on `screening_notice_legacy_request_test.go`, `punishment_notice_legacy_request_test.go`
  and `moderation_legacy_request_test.go` exits 0 for each.
- `git diff --exit-code 86baf1a -- memberreq_enforce.go memberreq_limits.go aispam.go settings_messages.go` and `-- go.mod go.sum main.go migrations` exit 0.
- Touched families with `-race` (`TestCaptcha|TestHandleCaptcha|TestEventMessage|TestEventPM|TestPMCopy|TestLegacy|TestMemberReqNotice|TestMemberReqFixed|TestAISpam|TestCheckAISpam|TestScreening|TestEventActionWords|TestAutoEvent|TestMemberDeletionExcluded|TestSettingsMessages|TestAutoMessages`):
  ok, 181 top-level tests, 53.8 s (56 s wall). `./fuku/db/eventmsg ./fuku/i18n ./fuku/utils/captchaweb`: ok (55.6 s wall, i18n 54 s).
- `make check-docs`: no drift (1.3 s). `make test-shard-check`: no diff (24.5 s). `CGO_ENABLED=0 go build ./...`: ok (2.4 s).
- golangci-lint v2.13.1: `run --new-from-rev=origin/main ./...` 0 issues (42.9 s); with `--build-tags=notesttools` 0 issues (21.5 s);
  `run --new=false ./fuku/modules/... ./fuku/db/eventmsg/... ./fuku/i18n/...` (and with `notesttools`) filtered to the touched files:
  only the four `captcha.go` baseline findings (`gocyclo` of `captchaVerifyCallback`, `captchaRefreshCallback`, `handlePendingCaptchaMessage`,
  staticcheck QF1003), none of `handleCaptchaTimeout`.
- Full `go test -tags testtools -race -count=1 ./fuku/modules`: ok, 346.4 s (5 m 49 s wall).
- New tests' time (budget about 3 s): `TestEventMessageCaptcha` 0.6 to 1.9 s (the web subtest 0.4 s, the timer subtest 0.18 s),
  `TestEventMessageEngineHooks` 0.03 s, `TestScreeningEventDefaults` 0.14 s, `TestMemberReqNoticeNoteFitsReserve` 0.05 s.
- The worst requirement note (all eight rules, the mute key with both remedies): uk 310, en 305, es 320, fr 346, hi 283, id 322,
  pt 316, ru 311 UTF-16 units, under the 600 reserve.

## Notes for plan 24-03 (and later)

- Private copy specs (24-03): register `captcha`, `aispam` and the four requirement events with `registerEventPM` (all `Automatic: true`,
  D-34). The captcha site's `Chat` is `{Id, Type: "supergroup", Title: chats.GetChatSettings(chatID).ChatName}`, so `{group}` in a
  default private text is the stored title (empty for a chat the bot has no row for; `staffDisplayTitle` handles that). The captcha
  site passes the full member (names and username from the live `getChatMember`, `{Id, FirstName: "User"}` on an error), so
  `pmCopySkipTarget` and the private send get the real user ID. `offerReplyPM` runs after the captcha's group step already today;
  it returns at once until a spec exists.
- The captcha reply's values: Reason `captchaEventReason(tr)`, Action `eventActionWord(tr, action)`, Count the held messages, Until
  `time.Now().Add(captchaFailureMuteSpan)` for a mute only. `eventPMDefaultText` already fills `{count}`, `{action}`, `{until}`.
- `TestPMCopyCallers`: a new file that calls `postEvent` or `postEventSent` needs an allow-list entry with its reason (24-05:
  `memberreq_notice.go`; 24-06: `aispam_notice.go`); `postEventSent` is already tracked and forbidden in staff, lockdown and raid files.
- `Admit` is consulted only when the group message or the private copy would be sent, so the AI spam notice off (no row, or a row
  written only by a part writer) spends no Redis command; a read error of `aispam` posts nothing.
- The nolint markers: plan 24-04 removes the four on `memberReqNoticeDefault`, `memberReqNoticeNote`, `memberReqNoticeNoteReserve`
  and `aispamNoticeDefaultReply`; plan 24-06 removes the one on `aispamEventReason`.
- `memberReqNoticeDefault` sets `ReplyMarkup` as a value `gotgbot.InlineKeyboardMarkup` only when `in.Rows` is non-empty, exactly as
  `memberReqPostNotice` does; the 24-05 site must keep sending the default through the requirement pacer and retry loop with these
  options, and set `KeepRows` to the "I fixed it" rows with `KeepRowsLast` and `Required` for the mute.
- The captcha cases leave the in-process self-delete timer running after a test returns (unchanged); `captchaFailureNoticeLife` is
  read when the notice is scheduled, so a test that shortens it must do so before the timeout runs.

## Known Stubs

None. The builders not yet called by a site are intentional (D-25: proven before a site switches) and carry dated nolint markers that
name the plan bringing their caller.

## Threat Flags

None beyond the plan's register. T-24-02: the three goldens run unchanged and their files are byte-identical to e40a7be. T-24-03: the
web verify answers come from the claim (`TestCaptchaWebFailedSolveAppliesAction` and the other web tests pass unchanged); the one added
read is the engine's bounded cached read. T-24-04: `TestEventMessageCaptcha` asserts no queue row with every bot kind at 60 s and
captcha.go names no producer. T-24-05: `DefaultSilentFor` in `upsert` stores Silent for a new aispam group row (eventmsg subtest).
T-24-06: the custom captcha text is filled with the escaped values (`<i>{mention}</i>` with `Ann & <Bo>`, golden name). T-24-SC:
go.mod and go.sum unchanged.

## Self-Check: PASSED

- Files: `fuku/modules/event_message_screening_test.go` exists; every modified file is in the three commits.
- Commits: 8038f21, cff641f and b23b473 are ancestors of HEAD; `git rev-list --count bd2c02a..HEAD` before this summary is 3.
