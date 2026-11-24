---
phase: 26-timed-punishments-escalation-and-warn-expiry
plan: 09
subsystem: modules, escalation
status: complete
tags: [escalation, flood, blacklist, forward-blocklist]
requires:
  - 26-08
provides:
  - escalate, escalationCountTimeout, escalationLadderLookup, escalationCount, escalationWarnGates (fuku/modules/escalation.go)
  - forwardBlockPunishmentOf, forwardBlockPunishTimed; forwardBlockNotice takes an autoPunishment
requirements: [TUNE-02]
---

# Plan 26-09: the ladder acts at the flood, the blacklist and the forward blocklist

## What changed
- `escalate` resolves the punishment: unchanged for anything not effective, for a sender chat, a service account or the bot,
  and when the ladder is off, empty or unreadable; step `min(count, len)` otherwise; step 1 when the count fails or passes 2 s.
  It checks `autoActionEffective` itself, so each call site is one added statement and `checkFlood` and `blacklistWatcher` keep
  their cyclomatic complexity (no branch added).
- `checkFlood` and `blacklistWatcher` call it right before `floodPunish` / `blacklistPunish`.
- `forwardBlockEnforce` calls it inside the punishment latch (an album counts once) and dispatches to `forwardBlockMute`,
  `Kick`, `Ban` (names and signatures unchanged) or the new `forwardBlockPunishTimed`.
- `TestEscalation` (11 subtests, about 1 s): steps in order, flood, same offence, forget, off, unreadable ladder, failed count,
  sender chat, warn and delete-only, second bot instance, forward blocklist album and later steps.
- AGENTS.md: Escalation ladder bullet, forward blocklist amendment, shutdown sentence, test family.

## Verification
Run with `-race`: `TestEscalation`, the Legacy punishment goldens, `TestFloodTimed`, `TestBlacklistTimed`, the event message
flood, blacklist and forward block families, `TestForwardBlockWatcher*`, `TestEventEngineCallers*`, `TestPMCopyCallers`,
`TestEventPM*`, `TestAutoMessagesEndToEnd`, `TestSettingsMessages*`; `CGO_ENABLED=0 go build ./...`; `make check-docs`.

Failing, and failing the same way without this change (checked for `TestMemberDeletionExcludedMessages` on a stash): the
captcha tests (`TestEventPMCaptcha`, `TestEventMessageCaptcha`, `TestEventMessageScreening*`, `TestLegacyScreeningNoticeRequests`,
`TestMemberDeletionExcludedMessages/a_captcha_challenge`) all stop on `CHECK constraint failed: chk_captcha_expires_at`.
This looks like a fixed test clock against today's real date; it is outside this plan and needs its own look.

Not run (no golangci-lint, gosec or PostgreSQL here): the lint and gosec gates, the complexity numbers from the linter (the
change adds no branch to either function), and `TestOffenceCountConcurrent` on PostgreSQL.
