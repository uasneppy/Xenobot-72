---
phase: 35-interactive-group-toys
auditor: gsd-security-auditor
date: 2026-11-27
verdict: SECURED
asvs_level: 1
block_on: high
threats_total: 13
threats_closed: 13
threats_open: 0
---

# Phase 35 Security Audit

**Phase:** 35 — Interactive Group Toys
**Verdict:** `SECURED`
**Threats Closed:** 13/13
**Threats Open (blocking >= high):** 0
**ASVS Level:** 1

## Threat Verification

| Threat ID | Plan | Category | Severity | Disposition | Status | Evidence |
|-----------|------|----------|----------|-------------|--------|----------|
| T-35-01 | 35-01 | Denial of Service / Integrity | medium | mitigate | CLOSED | `migrations/20261201120000_add_presence_toys.sql:8-12` (two idempotent `ALTER TABLE chat_presence ADD COLUMN IF NOT EXISTS ... BOOLEAN NOT NULL DEFAULT FALSE` statements); `xenobot/db/models/chat_presence.go:31-32` (zero GORM `default:` tags on booleans); `xenobot/db/presence/repository.go:70-71,78-97,213-214,365-376,443-444,461-462,500-501`; `xenobot/db/migrations/chat_presence_toys_migration_test.go:45-136` |
| T-35-02 | 35-01 | Elevation of Privilege | high | mitigate | CLOSED | `xenobot/db/backup/types.go:125-149,252-253` (`AllExportableModules` excludes `chat_conversations`; `PresencePortableBackup` round-trips `referendum_enabled` and `inspect_enabled`); `xenobot/db/backup/backup.go:1449-1450,1482-1485`; `xenobot/db/backup/chatconv_backup_test.go:18-57`; `xenobot/modules/copy_settings.go` untouched |
| T-35-03 | 35-02 | Tampering / Injection | medium | mitigate | CLOSED | `xenobot/utils/chatprompt/toys.go:54-109,179-262,308-365` (byte-identical static blocks separated from `Clean`-sanitized dynamic & user sections with explicit untrusted-data framing) |
| T-35-04 | 35-02 | Tampering / Denial of Service | high | mitigate | CLOSED | `xenobot/utils/chatprompt/toys.go:113-177,269-306,384-434` (`ParseReferendumJSON` single JSON object `<= 300`-rune question & `2..5` deduplicated `<= 100`-rune options; `ParseInspectJSON` strips `U+FE0F` and strictly intersects `supportedReactionEmoji`, rejecting `😂`/`🐱`/`😹`/`😻`/`😼` lookalikes) |
| T-35-05 | 35-03 | Denial of Service / Integrity | medium | mitigate | CLOSED | `xenobot/i18n/settings_locale_test.go:1008-1011,1325`; `xenobot/i18n/chat_toys_locale_test.go:27-43,50-156`; `xenobot/i18n/chat_brief_locale_test.go:164-176` (`make check-translations` 1071 keys across all 8 locales, `./xenobot/i18n` `-race` passed) |
| T-35-06 | 35-04 | Elevation of Privilege | high | mitigate | CLOSED | `xenobot/modules/settings_presence.go:182,239-246,1049-1099,1131-1149` (`PrivateOnly: true`, `settingsRightChangeInfo`, `settingsPresenceGroupRefused`, fail-closed `settingsPresenceStaffGate`, `settings_pr_ai_need_owner` when `!convUnlocked`, `p.cannotOpen()` on DB errors); `xenobot/modules/settings_presence_test.go:1150-1324` |
| T-35-07 | 35-04 | Denial of Service / Tampering | low | mitigate | CLOSED | `xenobot/modules/settings_presence.go:489-560,961-1008` (`pr` 8 rows, `pc` 6 rows); `xenobot/modules/settings_presence_test.go:105-138,174-254`; `xenobot/modules/settings_phase33_test.go:29-92` (`len(settingsPages) == 31`, `settingsMainOwnerRows == 17`) |
| T-35-08 | 35-05 | Denial of Service / Tampering | high | mitigate | CLOSED | `xenobot/modules/chat.go:53-57,86`; `xenobot/modules/chat_polls.go:197-244,391-433,446-468,515-541` (`presenceChatTypeAllowed`, real non-bot sender, `!presenceIsStaffGroup`, `!presenceIsLockedDown`, `chatToyGateOf`, per-chat/user cooldowns, breaker check, `"toy:b"` budget, `chatSanitizeText` + `chatCutRunes`) |
| T-35-09 | 35-05 | Tampering / Repudiation | medium | mitigate | CLOSED | `xenobot/modules/presence_reactions.go:100`; `xenobot/modules/presence.go:95-102`; `xenobot/modules/chat_polls.go:546-650` (`handlers.NewPoll` under `StopChat`, `IsClosed && TotalVoterCount > 0 && len(tied) >= 2`, `!StaffGroup && !Lockdown`, atomic `SetNX` `presencePollTieKey`) |
| T-35-10 | 35-06 | Denial of Service / Elevation of Privilege | high | mitigate | CLOSED | `xenobot/modules/chat.go:62-66,87`; `xenobot/modules/chat_inspect.go:66-105,179-228,251-273`; `xenobot/modules/chat_react.go:418-436`; `xenobot/modules/chat_media.go:26-30` (`StopChat` `chatToysEnqueue`, `!StaffGroup && !Lockdown`, `chatToyGateOf`, `"toy:b"` budget, `chat_media.go` size caps + Featherless `text_only` fallback) |
| T-35-11 | 35-06 | Tampering / Integrity | medium | mitigate | CLOSED | `xenobot/modules/chat_inspect.go:125-131,235-248,286-317`; `xenobot/modules/presence_react_api.go:208-241`; `xenobot/modules/presence_reactions.go:230-263` (`AllowSendingWithoutReply: false`, `presenceSetReaction(..., "inspect")` 24h `bot_react` + author/admin `👎` retraction) |
| T-35-12 | 35-07 | Integrity / Repudiation | medium | mitigate | CLOSED | `Makefile:76` (`TestChatPresenceToysMigrationDefaults`); `docs/src/content/docs/commands/chat/index.md`; `xenobot/modules/copy_settings.go` untouched (`make check-docs`, `make lint`, `make test-shard-check` passed) |
| T-35-SC | 35-01..35-07 | Tampering | high | accept | CLOSED | Zero new external Go packages added (`go.mod` and `go.sum` untouched since `85d697cd`; `TestChatpromptPackageIsLeaf` green); documented in Accepted Risks Log |

## Accepted Risks Log

| Threat ID | Severity | Rationale | Scope |
|-----------|----------|-----------|-------|
| T-35-SC | high | Phase 35 introduces zero new Go module dependencies (`go.mod` and `go.sum` have no commits or uncommitted diffs across Phase 35); only Go stdlib and existing repo dependencies are used, and `xenobot/utils/chatprompt/toys.go` preserves the stdlib-only leaf invariant. | Plans `35-01`..`35-07` |

## Unregistered Flags

None. All 7 plan summaries (`35-01-SUMMARY.md`..`35-07-SUMMARY.md`) reported no unregistered threat flags, and code inspection found no unmapped attack surface.
