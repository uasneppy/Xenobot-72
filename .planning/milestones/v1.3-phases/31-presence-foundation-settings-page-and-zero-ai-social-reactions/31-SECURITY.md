---
phase: 31-presence-foundation-settings-page-and-zero-ai-social-reactions
auditor: gsd-security-auditor
date: 2026-11-26
verdict: SECURED
asvs_level: 1
block_on: high
threats_total: 13
threats_closed: 13
threats_open: 0
---

# Phase 31 Security Audit

**Phase:** 31 — Presence Foundation, Settings Page, and Zero-AI Social Reactions
**Verdict:** `SECURED`
**Threats Closed:** 13/13
**Threats Open (blocking >= high):** 0
**ASVS Level:** 1 (with L2 boundary verification)

## Threat Verification

| Threat ID | Plan | Category | Severity | Disposition | Status | Evidence |
|-----------|------|----------|----------|-------------|--------|----------|
| T-31-01 | 31-01 | Tampering | medium | mitigate | CLOSED | `xenobot/db/presence/repository.go:22-23,52-54,97-104` (`ValidConsensusThreshold` `2..20` returns `ErrInvalidValue` before DB write); `migrations/20261127120000_add_chat_presence.sql:19` (`chk_chat_presence_consensus_threshold`); `xenobot/db/models/chat_presence.go:14`; `xenobot/db/presence/repository_test.go:124-196` |
| T-31-02 | 31-01 | Information Disclosure / Tampering | high | mitigate | CLOSED | `xenobot/db/cache/local.go:63,131` (`chatPresencePrefix` in `skipLocal()`); `xenobot/db/presence/repository.go:57-59,138` (`cache.DeleteCache` on every `Set*` write); `TestSkipLocalChatPresenceKey` |
| T-31-03 | 31-01 | Tampering | medium | mitigate | CLOSED | `xenobot/db/models/chat_presence.go:13,15` (no GORM `default:` tags on booleans); `xenobot/db/presence/repository_test.go:82-122` |
| T-31-04 | 31-02 | Information Disclosure / Elevation of Privilege | high | mitigate | CLOSED | `xenobot/modules/presence.go:65,68,73-124` (chat type, slash, `sender_chat`/anonymous channel, bot, `staff.GetStaffGroup`, lazy fail-closed `lockdown.GetActiveFresh`, `ActorChat.Id == Chat.Id` for retraction only); `xenobot/modules/presence_reactions.go:178-190,199,221,296`; `TestPresenceSafetyGate` |
| T-31-05 | 31-02 | Tampering / Denial of Service | high | mitigate | CLOSED | `xenobot/modules/presence_react_api.go:20,31-37,92-122,194-247` (explicit `[]gotgbot.ReactionType{}` clear, `U+FE0F` normalization, 20m `AvailableReactions` cache + `REACTION_INVALID` eviction, dedicated `presenceReactionPacer` `MaxWait: 2s`, zero `chatRecordFailure`/`chatBreakers` calls); `TestPresenceClearReactionWireJSON`, `TestPresenceReactAPI` |
| T-31-06 | 31-02 | Tampering / Repudiation | medium | mitigate | CLOSED | `xenobot/modules/presence.go:24,32-60` (`xenobot:presence:{<chat>}:*` outside `xenobot:cache:`); all `log.*` calls across `presence*.go` and `settings_presence.go` log only IDs and error classes, never message text or user names |
| T-31-07 | 31-03 | Tampering / Denial of Service | low | mitigate | CLOSED | All 11 Presence keys across 8 locales (`locales/*.yml`); `xenobot/i18n/settings_locale_test.go:934-944,1184,1247,1260`; `make check-translations` and `./xenobot/i18n` tests passed |
| T-31-08 | 31-04 | Elevation of Privilege / Denial of Service | high | mitigate | CLOSED | `xenobot/modules/presence_reactions.go:208-241` (`bot_react` pre-check before any Telegram call, strict author/anon-admin/admin authorization, atomic `DEL` claim, silent clear, zero group messages); `TestPresenceRetract` |
| T-31-09 | 31-04 | Tampering / Abuse | medium | mitigate | CLOSED | `xenobot/modules/presence_reactions.go:48-86,199,247-336` (Lua `SADD`/`SREM`/`EXPIRE`/`SCARD` distinct-user sets, `mru.User != nil`, hostile denylist, lazy fail-closed lockdown check before claim, atomic dual `cons:cd`+`cons:done` claim with `{<chat>}` hash tags); `TestPresenceConsensus` |
| T-31-10 | 31-04 | Denial of Service | medium | mitigate | CLOSED | `xenobot/modules/presence_reactions.go:24,155-171` (`chatJobs.start()`/`finish()`, `chatDraining.Load()`, 3s timeout from `chatContext()`, zero `main.go` changes); `TestShutdownEndsPresenceReaction` |
| T-31-11 | 31-05 | Elevation of Privilege / Information Disclosure | high | mitigate | CLOSED | `xenobot/modules/settings_presence.go:35,48-75,95-178,237-295` (`PrivateOnly: true`, `settingsRightChangeInfo`, `!p.private` refusal, `Hidden` + fail-closed `settingsPresenceStaffGate` via `staff.GetStaffGroupFresh` returning generic `common_callback_invalid_request` without naming Staff Group, canonical `2..20` validator); `TestSettingsPresenceFlows` |
| T-31-12 | 31-05 | Tampering | medium | mitigate | CLOSED | `xenobot/modules/settings_chat.go:80-81` (`OwnerOnly: true`, `PrivateOnly: true`); `xenobot/modules/settings_presence.go:26-30` (`c`/`ct`/`r`); `TestChatConversationsStayOutOfBackup`, `TestSettingsPresenceCallbackBytes`, `TestSettingsPresencePageFitsEveryLocale`, `TestSettingsChatCallbackBytes`, `TestSettingsChatPageFitsEveryLocale` |
| T-31-SC | 31-01..31-05 | Tampering | high | accept | CLOSED | Zero new external Go or JS dependencies (`git diff --stat 89433a0eb9fadf0d817101149f183b5bfaf3b905..HEAD -- go.mod go.sum docs/package.json` empty); documented in Accepted Risks Log |

## Accepted Risks Log

| Threat ID | Severity | Rationale | Scope |
|-----------|----------|-----------|-------|
| T-31-SC | high | Phase 31 introduces zero new Go modules or JS packages (`go.mod`, `go.sum`, and `docs/package.json` are untouched across all Phase 31 commits); only stdlib and previously pinned repo dependencies are used. | Plans `31-01`..`31-05` |

## Unregistered Flags

None. All 5 plan summaries (`31-01-SUMMARY.md`..`31-05-SUMMARY.md`) reported no unregistered threat flags, and code inspection found no unmapped attack surface.

## Independent Verification Commands Run

- `make check-translations` — PASS (1069 keys, all 8 locales complete)
- `go test -tags testtools -race -count=1 ./xenobot/config ./xenobot/db/cache ./xenobot/db/migrations -run 'TestLoadConfig|TestSkipLocalChatPresenceKey|TestChatPresenceMigrationStatements'` — PASS
- `go test -tags testtools -race -count=1 ./xenobot/db/presence ./xenobot/i18n` — PASS
- `go test -tags testtools -race -count=1 ./xenobot/db/backup -run 'TestChatConversationsStayOutOfBackup'` — PASS
- `go test -tags testtools -race -count=1 ./xenobot/modules -run 'TestPresence|TestShutdownEndsPresenceReaction|TestSettingsPresence|TestSettingsChatCallbackBytes|TestSettingsChatPageFitsEveryLocale'` — PASS
- `git diff --stat 89433a0eb9fadf0d817101149f183b5bfaf3b905..HEAD -- go.mod go.sum main.go docs/package.json` — empty diff
