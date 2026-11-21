---
phase: 23-warn-flood-blacklist-and-forward-blocklist-messages
plan: 01
subsystem: testing
status: complete
tags: [golden, characterization, warns, antiflood, blacklists, forward-blocklist, deletion]
requires: []
provides:
  - TestLegacyPunishmentNoticeRequests (22 subtests)
  - the goldenNotice* constants (22)
  - noticeGoldenTarget, noticeGoldenChannel, noticeGoldenClient, newNoticeGoldenSite
  - the drivers noticeGoldenWarn, noticeGoldenFlood, noticeGoldenBlacklist, noticeGoldenForwardBlock
affects: [23-02, 23-03, 23-04, 23-05, 23-06, 23-07]
tech-stack:
  added: []
  patterns:
    - "a golden recorded by running the test once with empty constants and pasting each %q `got` value, then committed before any production change"
    - "the site stores a rule of the case's kind (600 s) and of the other bot punishment kind (900 s), so wantRow proves the kind as well as the delay"
key-files:
  created:
    - fuku/modules/punishment_notice_legacy_request_test.go
  modified:
    - AGENTS.md
decisions:
  - "The AGENTS.md Testing bullet naming the golden and its never-re-record rule went into the golden's own commit (orchestrator instruction), so the commit holds two files instead of the plan's one; it still precedes every send-site change."
  - "newNoticeGoldenSite also stores a 900 s rule of the other bot punishment kind, so a row queued under the wrong kind fails wantRow instead of passing as no row."
  - "The getChatMember call count of the warn cases is not pinned (the plan's table mentioned one): the formal check list pins the sendMessage request, the punishment calls and the queue row, and a call count would bind later refactors without guarding any message."
  - "noticeGoldenTarget is a gotgbot.User (it is the message sender and the getChatMember record); the getChat answer is built from its fields."
metrics:
  duration: ~25 min
  completed: 2026-11-21
actuals:
  tokens: 7100
  tasks: 2
  commits: 1
plan_head_before: edad4d67800639323d9051a357f9f06c8bb26132
plan_head_after: 9d4554de4688f55fa9777b72e9f42f1d285e7b47
---

# Phase 23 Plan 01: Punishment notice goldens Summary

`TestLegacyPunishmentNoticeRequests` pins, on the code as it is before Phase 23, the exact `sendMessage` request, the punishment
call and the deletion queue row of all 22 sends this phase will change (the warn, the warn limit, the flood notice, the blacklist
notice and the forward blocklist notice, inert modes included), in Ukrainian with the real locales, and it was committed before any
send site changes.

## What later plans use

| Kind | Name |
|------|------|
| Test | `TestLegacyPunishmentNoticeRequests` in `fuku/modules/punishment_notice_legacy_request_test.go`, subtests `warn_reason warn_rules dwarn swarn warn_blacklist warn_forward_block warnlimit_mute warnlimit_kick warnlimit_ban warnlimit_inert flood_mute flood_kick flood_ban flood_ban_channel flood_inert blacklist_mute blacklist_kick blacklist_ban blacklist_ban_channel forward_block_mute forward_block_kick forward_block_ban` |
| Constants | `goldenNoticeWarnReason`, `goldenNoticeWarnRules`, `goldenNoticeDwarn`, `goldenNoticeSwarn`, `goldenNoticeWarnBlacklist`, `goldenNoticeWarnForwardBlock`, `goldenNoticeWarnLimitMute`, `goldenNoticeWarnLimitKick`, `goldenNoticeWarnLimitBan`, `goldenNoticeWarnLimitInert`, `goldenNoticeFloodMute`, `goldenNoticeFloodKick`, `goldenNoticeFloodBan`, `goldenNoticeFloodBanChannel`, `goldenNoticeFloodInert`, `goldenNoticeBlacklistMute`, `goldenNoticeBlacklistKick`, `goldenNoticeBlacklistBan`, `goldenNoticeBlacklistBanChannel`, `goldenNoticeForwardBlockMute`, `goldenNoticeForwardBlockKick`, `goldenNoticeForwardBlockBan` (the request JSON with the chat ID as `<chat>`) |
| Fixtures | `noticeGoldenTarget` (`gotgbot.User{Id: 42, FirstName: "Ann & <Bo>", LastName: "Lee", Username: "ann_lee"}`), `noticeGoldenChannel` (`gotgbot.Chat{Id: -1001234567890, Type: "channel", Title: "Spam Channel"}`) |
| Client | `noticeGoldenClient{*moduleBotClient}`: records every request in the embedded client first, then answers `getChatMember` of 42 with the target as a plain member and `getChat` of 42 with the target as a private chat |
| Site | `newNoticeGoldenSite(t, kind) *botSite`: fresh supergroup, real locales unless `withModerationGoldenLocales` already installed them, 600 s rule of `kind`, 900 s rule of the other bot punishment kind, `botSiteWarnsCleanup`, `memberReqCleanup(chat, 42)` |
| Drivers | `noticeGoldenWarn(t, s, call, texts...)` (each command as `s.commandCtx(text, true, false)`, must end with `ext.EndGroups`), `noticeGoldenFlood(t, s, action, channel)` (limit 1, messages 101 and 102 through `checkFlood`, each must return `ext.ContinueGroups`), `noticeGoldenBlacklist(t, s, action, channel, want)` (the word `spam`, message 101 `this has spam inside`), `noticeGoldenForwardBlock(t, s, punish)` (the punishment function on message 101), and `noticeGoldenMessage(s, id, text, channel)` (a target message dated by the fixed clock, with `SenderChat` set before `ext.NewContext` so `EffectiveSender` is the channel) |

Every case asserts: exactly its own punishment call (by method and `user_id` or `sender_chat_id`) and none of the other three
(`restrictChatMember`, `banChatMember`, `unbanChatMember`, `banChatSenderChat`), the delete it makes before the notice where one
applies, the target's stored warns for the warn cases, the number of `sendMessage` requests with the last one equal to its golden,
and exactly one queue row for message 9001 of the case's kind due 600 s after the fixed clock.

## What the goldens record (facts for the default builders)

- **Warn** (`warns_user_warning` + `warns_warning_reason`): reply to the triggering message (`message_id` 101, the command or the
  member's own message) with `allow_sending_without_reply`, link previews off, HTML; keyboard `[❌ Зняти попередження]`
  (`rmWarn|v1|u=42`), plus the Rules URL button `t.me/FukuTestBot?start=rules_<chat>` in the same row when the rules are set. The
  mention uses the first name from `getChatMember` (`Ann &amp; &lt;Bo&gt;`); the reason is escaped (`spam &amp; &lt;b&gt;`,
  the blacklist's `Blacklisted word: &#39;spam&#39;`). No reason means no reason line.
- **Warn limit**: same options, `reply_markup` `{}` (the empty keyboard), text `warns_limit_{muted,kicked,banned}` then one
  `\n - <reason>` per stored reason (`first`, `second` for the mute case at 2/2). The warns are reset (0 stored). The inert `tban`
  text is the reasons list alone (`"\n - reason"`), no punishment call, and the warn stays stored (1).
- **Flood** (`antiflood_checkflood_perform_action`): no reply parameters and no link preview options, HTML, the full name
  (`Ann &amp; &lt;Bo&gt; Lee`, or the channel's title), the untranslated word `muted`/`kicked`/`banned`, `reply_markup` with the
  Unmute row for a mute, the Unban row (`u=-1001234567890`) for a banned sender chat and `{}` otherwise. The inert `warn` sends
  `"дію виконано: ."` with an empty word and no punishment. `message_thread_id` is absent (0) in these cases.
- **Blacklist and forward blocklist** (`blacklists_bl_watcher_{muted,kicked,banned}_user`, `formatting.Shtml()`): link previews off
  and `reply_parameters` `{"allow_sending_without_reply":true}` with no `message_id`, no keyboard, the full name. The blacklist's
  reason is spliced unescaped (`'spam'`); the forward blocklist's is `staff_fwdblock_reason` (`пересилання із заблокованого
  джерела`), naming neither the source nor the Staff Group.
- **Kinds**: `/warn`, `/dwarn`, `/swarn` and the warn limit reached by `/warn` are `bot:punish_manual`; the blacklist's and the
  forward blocklist's warns, flood, blacklist and forward blocklist are `bot:punish_auto`.

## Tasks

| Task | Name | Commit |
|------|------|--------|
| 1 | record the five send sites' requests, punishment calls and deletion rows | 9d4554d |
| 2 | commit the goldens alone, before any send site changes | 9d4554d |

## Verification

- `go test -tags testtools -race -count=1 -run '^TestLegacyPunishmentNoticeRequests$' -v ./fuku/modules`, twice: exit 0, 22 subtests
  PASS, no FAIL, SKIP or "no tests to run"; the test takes 1.45 to 1.57 s, the command about 5 s each (41 s for the first, cold build).
- `go test -tags testtools -race -count=1 -run '^(TestLegacyPunishmentNoticeRequests|TestWarnThisUserSource|TestBotAutoPunishmentNotices|TestLegacyModerationRequests)$' ./fuku/modules`: ok, 15 s.
- `grep -cE '^\s+goldenNotice[A-Za-z]+\s+=' ...` prints 22; `grep -c '<chat>' ...` prints 24.
- `git diff --exit-code 1a382c3 -- fuku/modules/warns.go fuku/modules/antiflood.go fuku/modules/blacklists.go fuku/modules/forwardblock_notice.go fuku/modules/staff_forwardblock_watch.go fuku/modules/event_message.go go.mod go.sum` exits 0.
- `git log -1 --format=%s` of the golden starts with `test(23-01):`.
- `CGO_ENABLED=0 go build ./...` ok (3 s); `make check-translations` all present (under 1 s); `gofmt -l` empty.
- golangci-lint v2.13.1 (`CGO_ENABLED=0`): `run --new-from-rev=origin/main ./...` 0 issues (37 s); the same with
  `--build-tags=notesttools` 0 issues (19 s); `run --new=false ./fuku/modules/...` reports no finding in the new file.
- The full `fuku/modules` package was not run (plan 23-07 runs it).

## Deviations from Plan

**1. [Instruction] AGENTS.md Testing bullet in the golden's commit**
- The plan kept the golden commit to one file and left the AGENTS.md sentence to plan 23-02; the orchestrator's instruction put the
  Testing sentence naming the golden and its never-re-record rule in the same commit. The commit (9d4554d) therefore holds the golden
  and AGENTS.md (a new "Phase 23 test families" bullet), and the plan's "only the golden file" check counts 2 files. No production
  file is in it. Plan 23-02 extends that bullet instead of adding it.

**2. [Choice] The other kind's rule is stored too**
- `newNoticeGoldenSite` stores a 900 s rule of the other bot punishment kind besides the plan's 600 s rule of the case's kind, so
  the queue check proves the kind (a row of the wrong kind would exist and fail), as `TestWarnThisUserSource`'s blacklist case does.

**3. [Choice] No getChatMember count**
- The plan's case table mentioned "one getChatMember of 42" for `warn_reason`; the formal assertions (sendMessage, punishment calls,
  queue row) are pinned and the call count is not, because it guards no message and would bind later refactors of the warn path.

## Not verified

- Nothing beyond the goldens: no production code changed.

## Notes for plan 23-02

- Compare each default builder with these constants by name; the builders must reproduce the options exactly: the warn's
  `ReplyMarkup: &keyboard` with an empty keyboard serialises as `"reply_markup":{}` at the limit (keep a non-nil empty markup), the
  flood's `gotgbot.InlineKeyboardMarkup{InlineKeyboard: nil}` serialises as `{}` too, and `formatting.Shtml()` carries a
  `reply_parameters` object with only `allow_sending_without_reply` (so "no reply" for the blacklist and forward blocklist is that
  object, not an absent field).
- The warn's mention is `MentionHtml(id, first name)` from the live `getChatMember` record; the other three use the sender's full
  name (`ctx.EffectiveSender.Name()`), which is a channel's title for a sender chat.
- The blacklist's reason is spliced unescaped into its notice and escaped in its warn reason: keep both as they are.
- The inert warn limit (`tban`) text starts with a newline and carries no mode line; the inert flood (`warn`) text has an empty word.
- Both sends of `warnlimit_mute` are answered with message 9001, so one queue row is expected there.
- The test calls only `warnsModule.warnUser`, `dWarnUser`, `sWarnUser`, `antifloodModule.checkFlood`,
  `blacklistsModule.blacklistWatcher`, `forwardBlockWarn`, `forwardBlockMute`, `forwardBlockKick` and `forwardBlockBan`; keep those
  signatures, or the golden stops compiling.
- AGENTS.md Testing already has the "Phase 23 test families" bullet with the golden; extend it rather than add another.

## Known Stubs

None.

## Threat Flags

None. T-23-01 is mitigated: the golden is committed before any send-site change; later plans run it unchanged and check
`git diff --exit-code 9d4554d -- fuku/modules/punishment_notice_legacy_request_test.go`. T-23-SC: `go.mod` and `go.sum` unchanged.

## Self-Check: PASSED

- Files: `fuku/modules/punishment_notice_legacy_request_test.go` exists; `AGENTS.md` carries the Phase 23 Testing bullet.
- Commits: 9d4554d is an ancestor of HEAD.
