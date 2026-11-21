---
phase: 24-captcha-ai-spam-and-member-requirement-notices
plan: 01
subsystem: testing
status: complete
tags: [golden, characterization, captcha, memberreq, aispam, deletion]
requires: []
provides:
  - TestLegacyScreeningNoticeRequests (14 subtests)
  - the goldenScreen* constants (13)
  - newScreenGoldenSite, newScreenGoldenMreqEnv, screenGoldenReqMember, screenGoldenTokens, screenGoldenCaptcha, screenGoldenWantAction
affects: [24-02, 24-05, 24-06, 24-08]
tech-stack:
  added: []
  patterns:
    - "a golden recorded by running the test once with empty constants and pasting each %q `got` value, then committed before any production change"
    - "both bot punishment kinds carry a rule (600 s auto, 900 s manual), so a row of the wrong kind, or a row where none is wanted, fails"
    - "IDs that vary per run are replaced by tokens; the hold ID only where the callback data names it (h=<hold>), because a small number would match inside the text"
key-files:
  created:
    - fuku/modules/screening_notice_legacy_request_test.go
  modified:
    - AGENTS.md
decisions:
  - "The AGENTS.md Testing bullet naming the golden and its never-re-record rule went into the golden's own commit (orchestrator instruction, as in Phase 23), so the commit holds two files instead of the plan's one; it still precedes every send-site change and holds no production file."
  - "The requirement pause case also compares the first member's kick notice with goldenScreenReqKick (with that member's own <user> and <hold>), a free cross-check that the shared sender sends the same notice before the cap."
  - "The requirement mute case also pins the restrict's until_date to the hold's marker (MuteUntil)."
metrics:
  duration: ~20 min
  completed: 2026-11-21
actuals:
  tokens: 5400
  tasks: 2
  commits: 1
plan_head_before: 397a48d74e37fe82d8424079bfef9f5fd2bf7aa6
plan_head_after: e40a7be05c1bdab0ff580a7b80c0ef0e941ee35b
---

# Phase 24 Plan 01: Screening notice goldens Summary

`TestLegacyScreeningNoticeRequests` pins, on the code as it is before Phase 24, the exact `sendMessage` request, the action call and
the deletion queue state of the captcha failure notice, the member requirement notices (warn, warn limit at kick, ban and mute, kick,
ban and mute with its "I fixed it" button), the requirement pause notice and aispam's breaker notice, and that an AI delete verdict
deletes the message and posts nothing in the group, in Ukrainian with the real locales. It was committed (e40a7be) before any send
site changes.

## What later plans use

| Kind | Name |
|------|------|
| Test | `TestLegacyScreeningNoticeRequests` in `fuku/modules/screening_notice_legacy_request_test.go`, subtests `captcha_kick captcha_ban captcha_mute captcha_kick_held req_warn req_warnlimit_kick req_warnlimit_ban req_warnlimit_mute req_kick req_ban req_mute req_paused aispam_delete_silent aispam_breaker` |
| Constants | `goldenScreenCaptchaKick`, `goldenScreenCaptchaBan`, `goldenScreenCaptchaMute`, `goldenScreenCaptchaKickHeld`, `goldenScreenReqWarn`, `goldenScreenReqWarnLimitKick`, `goldenScreenReqWarnLimitBan`, `goldenScreenReqWarnLimitMute`, `goldenScreenReqKick`, `goldenScreenReqBan`, `goldenScreenReqMute`, `goldenScreenReqPaused`, `goldenScreenAISpamBreaker` (the request JSON with `<chat>`, and for the requirement notices `<user>` and `<hold>`) |
| Site | `newScreenGoldenSite(t) *botSite`: `newNoticeGoldenSite`'s shape (fresh supergroup, `noticeGoldenClient` answering member 42 `Ann & <Bo>`, real locales unless `withModerationGoldenLocales` installed them), a 600 s `bot:punish_auto` and a 900 s `bot:punish_manual` rule, `memberReqCleanup(chat, 42)`, and the chat's `stored_messages` and `ai_spam_settings` rows and cache key removed |
| Requirement env | `newScreenGoldenMreqEnv(t) *mreqSchedEnv`: `newMemberReqEnvWith(RealLocale)`, the deletion clock fixed at a whole second as `newMreqSchedEnv` does, the same two rules |
| Helpers | `screenGoldenReqMember(env)` (first name `Ann & <Bo>`, no last name, username `ann_lee`, fresh ID), `screenGoldenTokens(json, chatID, userID, holdID)` (0 skips a token), `screenGoldenCaptcha(t, s, action, held)` (an attempt of 42, two held texts with `held`, then `handleCaptchaTimeout(..., 77, action)` which must claim and apply), `screenGoldenWantAction(t, callsFor, method, userID)` (exactly one call of `method` about the user and none of `restrictChatMember`, `banChatMember`, `unbanChatMember` otherwise; returns the call) |

Every case asserts its action call by method and `user_id` and the absence of the other two, the number of notices with each one
equal to its golden, and the queue: no row for the captcha notice and the AI delete, one `bot:punish_auto` row due 600 s after the
clock for the requirement warn, warn-limit, kick and ban notices, the pause notice (and the kick notice before it) and the breaker
notice, and one due 24 hours after the clock for the requirement mute notice. The captcha cases also pin the `deleteMessage` of the
challenge (77); the requirement warn cases the stored warns (1 after a plain warn, 0 after the limit action); the mute the restrict's
`until_date` equal to the hold's `MuteUntil`; the AI case one `deleteMessage` of message 501 and no `sendMessage` at all.

## What the goldens record (byte quirks the default builders must keep)

- **Captcha failure notice** (`captcha_timeout_failure_{kicked,banned,muted}`, or `captcha_timeout_with_messages` with held messages):
  sent through `helpers.SendMessageWithErrorHandling` with `&gotgbot.SendMessageOpts{ParseMode: formatting.HTML}` only, so the
  request has `chat_id`, `parse_mode` and `text` and nothing else: no `link_preview_options`, no `reply_markup` (not even `{}`), no
  `reply_parameters`, no thread. The text starts with `❌ ` and the mention is `MentionHtml(42, first name)` from the live
  `getChatMember` (`Ann &amp; &lt;Bo&gt;`; the code falls back to `User` when the lookup fails or the first name is empty). The held
  variant reads `...: капчу не пройдено, результат — вигнання.\n\n📝 Повідомлень, надісланих до перевірки, видалено: 2.` (the action
  word `captcha_action_kicked` and the count spliced by `%s`/`%d`). No queue row is written under either bot kind's rule.
- **Captcha action calls**: kick is one member-removing `unbanChatMember` (no `banChatMember`), ban one `banChatMember`, mute one
  `restrictChatMember` (with no `until_date`; the 24 hour unmute lives in `captcha_muted_users`).
- **Requirement notices** (`memberReqPostNotice`): HTML, `link_preview_options` `{"is_disabled":true}`, no `reply_parameters`, and no
  `reply_markup` field at all when there is no button (an empty `gotgbot.InlineKeyboardMarkup{}` is not set on the opts; unlike the
  Phase 23 warn limit, which serialises `{}`). The mention is `memberReqMention`: the link text is the escaped name wrapped in U+2068
  ... U+2069 (`⁨Ann &amp; &lt;Bo&gt;⁩` inside `<a href="tg://user?id=<user>">`). The rule phrase of the last-name rule is
  `немає прізвища`; the last-name rule has no remedy key, so no remedy sentence follows. Warn: `❗ <mention>, ви порушуєте вимоги групи:
  немає прізвища. Попередження 1 з 3.` Warn limit: `... порушує вимоги групи: ... Це 1 з 1 попереджень, тому учасника вигнано.`
  (`забанено.`, and for mute `тому учасник отримує мут. Адмін може зняти мут.`). Kick: `👢 Учасника <mention> вигнано: ...`. Ban: `⛔
  Учасника <mention> забанено: ...`. Mute: `🔇 <mention>, вам видано мут: ... Виправте вказане і натисніть «Виправлено».` with one row
  of one button `✅ Виправлено`, callback data `mreq|v1|a=f&h=<hold>&u=<user>` (the `&` appears as `&` in the JSON).
- **Requirement action calls**: kick is one `unbanChatMember` (member-removing), ban one `banChatMember`, mute one
  `restrictChatMember` with `until_date` = the hold's `MuteUntil`; the warn-limit actions likewise (kick unban, ban ban, mute restrict).
- **Pause notice** (`mreq_notice_paused`, same sender and options as the requirement notices, no button): `⏸ Покарання за вимогами тут
  призупинено на 10 хв: понад 1 учасників порушили вимогу за хвилину. ...` (the `1` is the test's `memberReqCapPerMinute`).
- **AI spam breaker notice** (`aispam_breaker_notice`, `formatting.Shtml()`): HTML, link previews off and `reply_parameters`
  `{"allow_sending_without_reply":true}` with no `message_id`; no keyboard. Queued under `bot:punish_auto`.
- **AI delete verdict**: one `deleteMessage` of the flagged message and no `sendMessage` to anyone (with no dump chat configured), no
  queue row.

## Tasks

| Task | Name | Commit |
|------|------|--------|
| 1 | record the captcha, requirement and AI spam notices with their action calls and queue state | e40a7be |
| 2 | commit the goldens alone, before any send site changes | e40a7be |

## Verification

- `go test -tags testtools -race -count=1 -run '^TestLegacyScreeningNoticeRequests$' -v ./fuku/modules`, twice: exit 0, 14 subtests
  PASS, no FAIL, SKIP or "no tests to run"; the test takes 1.87 and 1.88 s, the command about 3 s each (42 s for the first, cold build).
- `go test -tags testtools -race -count=1 -run '^(TestLegacyScreeningNoticeRequests|TestLegacyPunishmentNoticeRequests|TestMemberReqNoticeScheduled|TestHandleCaptchaTimeoutAppliesKickMuteAndSkipsStaleAttempts|TestCheckAISpamDeletesFlaggedMessageAndMirrorsIt)$' ./fuku/modules`: ok, 4 s.
- `grep -c 'func TestLegacyScreeningNoticeRequests('` prints 1; `grep -cE '^\s+goldenScreen[A-Za-z]+\s+='` prints 13; `grep -c '<chat>'`
  prints 16.
- `git diff --exit-code 86baf1a -- fuku/modules/captcha.go fuku/modules/captcha_web.go fuku/modules/memberreq_notice.go fuku/modules/memberreq_enforce.go fuku/modules/memberreq_limits.go fuku/modules/aispam.go fuku/modules/event_message.go` exits 0; the same for `go.mod go.sum`.
- `git log -1 --format=%s` of the golden starts with `test(24-01):`.
- `CGO_ENABLED=0 go build ./...` ok; `make check-translations` all present (under 1 s); `gofmt -l` empty.
- golangci-lint v2.13.1 (`CGO_ENABLED=0`): `run --new-from-rev=origin/main ./...` 0 issues (42 s); the same with
  `--build-tags=notesttools` 0 issues (22 s); `run --new=false ./fuku/modules/...` (and again with `--build-tags=testtools`, 26 s)
  reports no finding in the new file.
- The full `fuku/modules` package was not run (plan 24-08 runs it).

## Deviations from Plan

**1. [Instruction] AGENTS.md Testing bullet in the golden's commit**
- The plan kept the golden commit to one file and left the AGENTS.md sentence to plan 24-02; the orchestrator's instruction put the
  Testing sentence naming the golden and its never-re-record rule in the same commit. The commit (e40a7be) therefore holds the golden
  and AGENTS.md (a new "Phase 24 test families" bullet after the Phase 23 one), and the plan's "only the golden file" check counts 2
  files. No production file is in it. Plan 24-02 extends that bullet instead of adding it.

**2. [Choice] The pause case cross-checks the kick notice**
- Besides the pause notice, `req_paused` compares the first member's kick notice with `goldenScreenReqKick` and requires both rows;
  it costs nothing and proves the shared sender sends the same notice up to the cap.

## Not verified

- Nothing beyond the goldens: no production code changed.

## Notes for plan 24-02

- Compare each default builder with these constants by name. The captcha default must keep the bare options (`ParseMode` HTML only):
  adding `LinkPreviewOptions`, a `ReplyMarkup` or a `ReplyParameters` changes the request. The requirement defaults must leave
  `ReplyMarkup` unset when there is no button (no `"reply_markup":{}`), keep link previews off and send no reply parameters.
- The captcha mention is the first name only (from `getChatMember`, `User` as the fallback), escaped by `MentionHtml`, with no
  isolates; the requirement mention is `memberReqMention`, which wraps the escaped name in U+2068 ... U+2069 inside the link.
- `captcha_timeout_with_messages` is filled with `fmt.Sprintf(template, mention, actionWord, count)` and the three plain templates with
  `fmt.Sprintf(template, mention)`; the action word comes from `captcha_action_{kicked,banned,muted}` (kick is the default branch).
- The requirement text is translated with the name token `<<mreq-name>>` and the mention is spliced in afterwards (`lockdownSplice`),
  so a name is never translated; the remedy sentences (none for the last-name rule) are appended after one space before the splice.
- The breaker notice uses `formatting.Shtml()` (a `reply_parameters` object with only `allow_sending_without_reply`); it is not an
  event and must stay byte-identical.
- The test calls only `handleCaptchaTimeout`, `memberReqEnforce` (through `mreqEnforce`), `aispamNotifyBreaker` and the aispam
  dispatcher path (`LoadAISpam`); keep those signatures, or the golden stops compiling.
- The captcha cases leave the in-process 30 s self-delete timer running after the test returns (as the existing captcha tests do);
  it only records a `deleteMessage` on the finished site's client.

## Known Stubs

None.

## Threat Flags

None. T-24-01 is mitigated: the golden is committed before any send-site change; later plans run it unchanged and check
`git diff --exit-code e40a7be -- fuku/modules/screening_notice_legacy_request_test.go`. T-24-SC: `go.mod` and `go.sum` unchanged.

## Self-Check: PASSED

- Files: `fuku/modules/screening_notice_legacy_request_test.go` exists; `AGENTS.md` carries the Phase 24 Testing bullet.
- Commits: e40a7be is an ancestor of HEAD.
