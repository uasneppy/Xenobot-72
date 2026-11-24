---
phase: 23-warn-flood-blacklist-and-forward-blocklist-messages
fixed_at: 2026-11-24T11:08:35Z
review_path: /Users/denyspetrenko/Xenobot-72/.planning/phases/23-warn-flood-blacklist-and-forward-blocklist-messages/23-REVIEW.md
fix_scope: critical_warning
iteration: 1
findings_in_scope: 4
fixed: 4
skipped: 0
status: all_fixed
---

# Phase 23: Code Review Fix Report

**Fixed at:** 2026-11-24T11:08:35Z
**Source review:** `.planning/phases/23-warn-flood-blacklist-and-forward-blocklist-messages/23-REVIEW.md`
**Fix scope:** `critical_warning` (excludes Info `IN-01`, `IN-02`, `IN-03` and necessity `WR-05`)
**Iteration:** 1

**Summary:**
- Findings in scope: 4
- Fixed: 4
- Skipped: 0

## Fixed Issues

### WR-01: `rmWarnButton` fails on custom media warn messages and leaves the button active without answering the callback

**Files modified:** `fuku/modules/warns.go`, `fuku/modules/warns_command_test.go`
**Commit:** `0212f5cf`
**Applied fix:** Extracted `editRmWarnMessage` in `fuku/modules/warns.go` following the `editUnrestrictMessage` pattern from `fuku/modules/bans.go`: text messages use `EditText`, captionless media (stickers and video notes) clear their inline keyboard via `EditReplyMarkup`, and caption-capable media (photo, video, animation, document, audio, voice) update their caption via `EditCaption`. Updated `rmWarnButton` to always answer the callback query even if the message edit fails, while still returning `editErr`. Added unit tests in `fuku/modules/warns_command_test.go` covering captioned media, captionless stickers, and callback query answering on edit failure.

### WR-02: `warnSite` omits `ThreadID`, misrouting warn notices to the General topic in forum supergroups after deletion

**Files modified:** `fuku/modules/warns.go`, `fuku/modules/warns_command_test.go`
**Commit:** `936dfa92`
**Applied fix:** Set `site.ThreadID = ctx.EffectiveMessage.MessageThreadId` in `warnSite` when `ctx.EffectiveMessage.IsTopicMessage` is true, propagated `site.ThreadID` onto `reply.Opts.MessageThreadId` for both `warnEventReply` and `warnLimitEventReply` in `warnThisUser`, and set `MessageThreadId` on `warnPostLimit`'s `SendMessageOpts` for topic messages. Non-topic messages keep `MessageThreadId == 0`, preserving non-forum requests byte for byte. Added `TestWarnTopicThreadIDPropagated` in `fuku/modules/warns_command_test.go`.

### WR-03: `warnPostLimit` sends an empty message text when an inert warn-limit mode is reached with no reasons

**Files modified:** `fuku/modules/warns.go`, `fuku/modules/warns_command_test.go`
**Commit:** `d2709075`
**Applied fix:** Added an early `if strings.TrimSpace(reply) == "" { return nil }` guard in `warnPostLimit` before calling `b.SendMessage`, preventing Telegram `400 Bad Request: message text is empty` errors when an inert warn-limit mode is reached with no stored reasons. Added `TestWarnPostLimitSkipsEmptyReply` in `fuku/modules/warns_command_test.go`.

### WR-04: Unescaped HTML interpolation in untimed `blacklistNoticeDefaultReply` and command error replies

**Files modified:** `fuku/modules/event_message_auto.go`, `fuku/modules/antiflood.go`, `fuku/modules/blacklists.go`, `fuku/modules/event_message_auto_test.go`
**Commit:** `023d20b4`
**Applied fix:**
- In `fuku/modules/event_message_auto.go`, escaped Telegram HTML text markup characters (`<`, `>`, `&`) on the untimed branch of `blacklistNoticeDefaultReply` via `telegramHTMLTextEscaper` (preserving single quotes so frozen `punishment_notice_legacy_request_test.go` goldens remain byte-identical). Added `TestBlacklistNoticeDefaultReplyEscapesHTML` in `fuku/modules/event_message_auto_test.go`.
- In `fuku/modules/antiflood.go`, wrapped `args[0]` in `html.EscapeString` on the unknown flood mode reply.
- In `fuku/modules/blacklists.go`, wrapped each entry in `result.TooLong` and `result.Already` with `html.EscapeString`, matching `result.TooBroad`.
- Note on `fuku/modules/warns.go:217`: intentionally left untouched because `fuku/modules/warns_legacy_request_test.go:159` (`mode_unknown_unescaped`) explicitly pins the unescaped `a&b<c` output in `goldenWarnModeAmpersand` as a frozen legacy golden contract.

## Out-of-Scope Findings

The following findings in `23-REVIEW.md` were excluded by `fix_scope: critical_warning` and remain untouched:
- `WR-05` (`Category: necessity`): unused `limitKeys` map in `warnPostLimit`.
- `IN-01`, `IN-02`, `IN-03` (`Info` severity).

## Verification

**Environment:** Main checkout (`workflow.use_worktrees: false`), branch `claude/sharp-goodall-f23ang`, `TZ=UTC`.

All checks passed:
- `go vet -tags testtools ./...` — pass
- `CGO_ENABLED=0 go build ./...` — pass
- `make check-translations` — pass (1068 keys across all 8 locales)
- `make check-docs` — pass (no drift)
- `make test-shard-check` — pass
- `TZ=UTC go test -tags testtools -race -count=1 -run '^(TestEventEngineCallers|TestPMCopyCallers|TestLegacyPhase29Baseline|TestLegacyPhase30Baseline|TestRmWarnButton|TestWarnTopicThreadIDPropagated|TestWarnPostLimitSkipsEmptyReply|TestBlacklistNoticeDefaultReplyEscapesHTML|TestAutoEventDefaults)$' ./fuku/modules` — pass (`3.337s`)
- `TZ=UTC go test -tags testtools -race -count=1 -run 'TestAutoEventDefaults|TestBlacklistNoticeDefaultReplyEscapesHTML|TestLegacyBlacklistRequests|TestLegacyTimedPunishmentRequests|TestLegacyPunishmentNoticeRequests|TestLegacyWarnSettingsRequests' ./fuku/modules` — pass (`74.823s`)
- `TZ=UTC go test -tags testtools -race -count=1 -run 'TestRmWarnButton|TestWarnCallbackHandlersPropagateGotgbotRequestErrors' ./fuku/modules` — pass (`1.882s`)

Note: Pre-existing `TestEventMessageForwardBlock/forward_block_through_the_watcher` attempts to dial `127.0.0.1` miniredis, which is blocked by the sandbox TCP restriction (`connect: operation not permitted`).

---

_Fixed: 2026-11-24T11:08:35Z_
_Fixer: the agent (gsd-code-fixer)_
_Iteration: 1_
