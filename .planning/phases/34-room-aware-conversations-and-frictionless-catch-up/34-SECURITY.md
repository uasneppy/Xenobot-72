---
phase: 34-room-aware-conversations-and-frictionless-catch-up
auditor: gsd-security-auditor
date: 2026-11-27
verdict: SECURED
asvs_level: 1
block_on: high
threats_total: 17
threats_closed: 17
threats_open: 0
---

# Phase 34 Security Audit

**Phase:** 34 — Room-Aware Conversations and Frictionless Catch-Up
**Verdict:** `SECURED`
**Threats Closed:** 17/17
**Threats Open (blocking >= high):** 0
**ASVS Level:** 1

## Threat Verification

| Threat ID | Plan | Category | Severity | Disposition | Status | Evidence |
|-----------|------|----------|----------|-------------|--------|----------|
| T-34-01 | 34-01 | Tampering / Integrity | medium | mitigate | CLOSED | `migrations/20261130120000_add_presence_room_and_catchup.sql:9-16` (idempotent `ADD COLUMN IF NOT EXISTS ... BOOLEAN NOT NULL DEFAULT FALSE`); `xenobot/db/models/chat_presence.go:27-29` (`AmbientContext`, `VocativeEnabled`, `PMBriefReact` carry zero GORM `default:` tags); `xenobot/db/migrations/chat_presence_room_migration_test.go:45-138` |
| T-34-02 | 34-01 | Elevation of Privilege | high | mitigate | CLOSED | `xenobot/db/backup/types.go:101-149,239-252` (`AllExportableModules` excludes `chat_conversations`; `PresencePortableBackup` omits `archive_chat_id`/`archive_thread_id` and round-trips `ambient_context`/`vocative_enabled`/`pm_brief_react`); `xenobot/db/backup/backup.go:1427-1489` (`importPresence` zeroes archive target IDs and delegates to `ReplacePortableTx`); `xenobot/db/presence/repository.go:386-444` (`ReplacePortableTx` preserves target `existingArchiveChatID`/`existingArchiveThreadID`); `xenobot/db/backup/chatconv_backup_test.go:18-57` |
| T-34-03 | 34-02 | Tampering / Prompt Injection | medium | mitigate | CLOSED | `xenobot/utils/chatprompt/ambient.go:13-22,60-66` (`UserWithAmbient` renders every ambient turn via `line(t, MaxQuotedRunes)` before `[Recent conversation, oldest first]` and closing untrusted-data reminder); `xenobot/utils/chatprompt/chatprompt.go:103-105,132-182` (`UserWithFiles` delegates to `UserWithAmbient(nil, ...)`; `line` uses `Clean` and `Name` to strip newlines/controls/bidi and suffix `(member)` on `Xenobot-72` impersonators) |
| T-34-04 | 34-02 | Denial of Service (ReDoS) / Tampering | low | mitigate | CLOSED | `xenobot/utils/chatprompt/ambient.go:3-9,74-171` (stdlib-only imports `cmp`, `slices`, `strings`, `unicode`, `unicode/utf8`; zero `regexp`; deterministic O(N) `matchFoldPrefix`/`runeEqualFold`/`isVocativeDelimiter` prefix check); `xenobot/utils/chatprompt/leaf_test.go:17-44` (`TestChatpromptPackageIsLeaf`) |
| T-34-05 | 34-03 | Tampering / Denial of Service | low | mitigate | CLOSED | `xenobot/i18n/settings_locale_test.go:991-1006,1043-1044` (`settingsKeyCeilings` and `settingsAlertKeys` for `settings_pc_need_owner_conv_log` and `settings_pc_need_owner_log`, plain text `<= 200` runes); `xenobot/i18n/chat_brief_locale_test.go:41-45` (`chatBriefLocaleLimits` for `brief_empty`, `brief_header_short_pm`, `brief_header_long_pm`, `brief_note_pm`, `brief_pm_blocked`); `locales/{en,es,fr,hi,id,pt,ru,uk}.yml` (`make check-translations` passed: 1070 keys across all 8 locales; `./xenobot/i18n` `-race` passed) |
| T-34-06 | 34-04 | Tampering / Integrity | low | mitigate | CLOSED | `xenobot/modules/chat_log.go:59,109-117,302-304,362` (`chatLogEntry.At` populated from `msg.Date` in `chatLogEntryOf`; `chatLogEdit` reads `chatLogEntryAt` and preserves `existing.At > 0`; `chatLogSinceTime` skips `entry.At <= 0` legacy entries) |
| T-34-07 | 34-04 | Information Disclosure | medium | mitigate | CLOSED | `xenobot/modules/chat_log.go:196,229,264,299` (`entry.Thread != threadID` enforced in `chatLogRecentThread`, `chatLogRecent`, `chatLogSinceUser`, and `chatLogSinceTime`) |
| T-34-08 | 34-05 | Elevation of Privilege / Privacy | high | mitigate | CLOSED | `xenobot/modules/chat_run.go:368-380` (`chatAmbientContextTurns` enforces `!presenceIsStaffGroup(job.chatID)`, `conv.Enabled && conv.LogEnabled`, and `pres.AmbientContext`, and queries `chatLogRecentThread` scoped by `job.threadID`) |
| T-34-09 | 34-05 | Tampering / Context Bloat | low | mitigate | CLOSED | `xenobot/modules/chat_history.go:56-80` (`chatReadHistoryWithIDs` returns `MessageID > 0` set); `xenobot/modules/chat_watch.go:243-244` (`job.quotedMessageID = reply.MessageId`); `xenobot/modules/chat_run.go:290-291,390-392` (filters out `job.messageID`, `job.quotedMessageID`, and `historyIDs[entry.MessageID]`); `xenobot/modules/chat_prompt.go:256` (`chatprompt.UserWithAmbient`) |
| T-34-10 | 34-05 | Spoofing / Elevation of Privilege | medium | mitigate | CLOSED | `xenobot/modules/chat_watch.go:82-103` (requires `msg.ForwardOrigin == nil`, `chatprompt.ParseVocativePrefix`, `!presenceIsStaffGroup(chat.Id)`, `pres.VocativeEnabled`, and `chatAccept` [`conv.Enabled` + `Audience`], returning early at line `99` before `chatReactWatch` at line `103`) |
| T-34-11 | 34-06 | Denial of Service / Info Disclosure | medium | mitigate | CLOSED | `xenobot/modules/chat_brief.go:49-54,192-223,246-259,497-504` (`chatBriefParseArgs` clamps count `5..200` and duration `60s..7d` via `extraction.ParseDurationToken`; `/judge` remains reply-only at `246-251`; `job.threadID` passed to `chatLogRecent`, `chatLogSinceUser`, `chatLogSinceTime`) |
| T-34-12 | 34-06 | Elevation of Privilege / DoS | high | mitigate | CLOSED | `xenobot/modules/presence_reactions.go:195,202,219-221,359-407` (`presenceHandlePMBrief` enforces `mru.User != nil && !mru.User.IsBot`, `!presenceIsStaffGroup`, `conv.LogEnabled && chatProviderConfigured(conv.Provider)`, lazy fail-closed `presenceIsLockedDown`, `chatAudienceAllows`, Redis `SetNX` on `presencePMBriefCooldownKey`, and `chatLogEntryAt` before `chatBriefEnqueue`) |
| T-34-13 | 34-06 | Information Disclosure / Spam | medium | mitigate | CLOSED | `xenobot/modules/chat_brief.go:472-492,518-524,539-541,555-557,567-569,582,676-704` (`job.silentPM` skips `chatSayGated`, `chatSendTyping`, `chatBriefSay`, and `chatDeliverPrivate(..., silent)` suppresses `brief_note_pm`, `brief_pm_blocked` on `403`, and `brief_malfunction`) |
| T-34-14 | 34-07 | Elevation of Privilege | high | mitigate | CLOSED | `xenobot/modules/settings_presence.go:178,223-234,299-309,365-387,1017-1097` (`PrivateOnly: true`, `settingsRightChangeInfo`, `settingsPresenceGroupRefused`, fail-closed `settingsPresenceStaffGate`, `settingsPresenceChatLocks` + `settingsPresenceSetRoomToggle` checking `convUnlocked`/`logUnlocked` on enable and calling `p.cannotOpen()` on DB read failure) |
| T-34-15 | 34-07 | Denial of Service / Tampering | low | mitigate | CLOSED | `xenobot/modules/settings_presence.go:471-540,937-976` (`renderSettingsPresence` keeps `pr` at 8 rows; `renderSettingsPresenceRoomCatchup` renders `pc` in 4 rows); `xenobot/modules/settings_presence_test.go:70-132,168-246` (verifies `<= 8` rows, `<= 64`-byte callbacks, `<= 3800` UTF-16 units, `Omitted == 0` across all 8 locales); `xenobot/modules/settings_phase33_test.go:29-90` (`len(settingsPages) == 31`, `settingsMainOwnerRows == 17`) |
| T-34-16 | 34-08 | Integrity / Repudiation | medium | mitigate | CLOSED | `Makefile:76` (`TestChatPresenceRoomMigrationDefaults` in `test-postgres-integrity`); `xenobot/modules/copy_settings.go` (empty `git diff`, zero `presence`/`chatconv` references); `docs/src/content/docs/commands/chat/index.md:14,30` & `docs/src/content/docs/commands/presence/index.md:17` (`make check-docs` passed with zero drift) |
| T-34-SC | 34-01..34-08 | Tampering | high | accept | CLOSED | Zero new external Go packages added (`go.mod` and `go.sum` untouched since `85d697cd`); documented in Accepted Risks Log |

## Accepted Risks Log

| Threat ID | Severity | Rationale | Scope |
|-----------|----------|-----------|-------|
| T-34-SC | high | Phase 34 introduces zero new Go module dependencies (`go.mod` and `go.sum` have no commits or uncommitted diffs across Phase 34); only Go stdlib and existing repo dependencies are used. | Plans `34-01`..`34-08` |

## Unregistered Flags

None. All 8 plan summaries (`34-01-SUMMARY.md`..`34-08-SUMMARY.md`) reported no unregistered threat flags, and code inspection found no unmapped attack surface.
