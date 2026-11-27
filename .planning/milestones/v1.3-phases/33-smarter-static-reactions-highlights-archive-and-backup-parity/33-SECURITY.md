---
phase: 33-smarter-static-reactions-highlights-archive-and-backup-parity
auditor: gsd-security-auditor
date: 2026-11-27
verdict: SECURED
asvs_level: 1
block_on: high
threats_total: 23
threats_closed: 23
threats_open: 0
---

# Phase 33 Security Audit

**Phase:** 33 — Smarter Static Reactions, Highlights Archive, and Backup Parity
**Verdict:** `SECURED`
**Threats Closed:** 23/23
**Threats Open (blocking >= high):** 0
**ASVS Level:** 1

## Threat Verification

| Threat ID | Plan | Category | Severity | Disposition | Status | Evidence |
|-----------|------|----------|----------|-------------|--------|----------|
| T-33-01 | 33-01 | Tampering | medium | mitigate | CLOSED | `xenobot/db/reactions/repository.go:19-30,43-51,254-257` (`ValidChancePct` `1..100`, `ValidCooldownSecs` `0..86400`, `ErrInvalidOption` before DB write); `xenobot/modules/reactions.go:800-803`; `migrations/20261129120000_add_static_reaction_options.sql:16-22` (`chk_reactions_chance_pct`, `chk_reactions_cooldown_secs`); `xenobot/db/models/reactions.go:15-16` |
| T-33-02 | 33-01 | Repudiation / Integrity | medium | mitigate | CLOSED | `xenobot/db/cache/local.go:64,136` (`reactionRulesPrefix` in `skipLocal`); `xenobot/db/reactions/repository.go:75-86,165,175,184,213,274,304` (`invalidateReactionCaches` drops both `reactions:<chat>` and `reaction_rules:<chat>`) |
| T-33-03 | 33-01 | Elevation of Privilege | high | mitigate | CLOSED | `xenobot/db/reactions/repository.go:238-249,258-276` (`SetReactionOptionsByID` scopes `UPDATE` with `WHERE id = ? AND chat_id = ?`, returns `nil, nil` on `RowsAffected == 0`, reads back via `GetReactionEntryFresh`) |
| T-33-04 | 33-02 | Tampering | medium | mitigate | CLOSED | `xenobot/db/presence/repository.go:32-44,98-111,298-325` (`ValidArchiveThreshold` `2..50`, `ValidArchiveEmojiMode` `positive`/`all`, `ValidArchiveDelivery` `copy`/`forward`, `ErrInvalidValue` before DB write); `xenobot/modules/presence.go:304-339`; `migrations/20261129130000_add_presence_stickers_and_archive.sql:21-31`; `xenobot/db/models/chat_presence.go:23-25` |
| T-33-05 | 33-02 | Repudiation / Integrity | high | mitigate | CLOSED | `migrations/20261129130000_add_presence_stickers_and_archive.sql:33-38` (`pk_chat_presence_archived` on `(chat_id, message_id)`); `xenobot/db/models/chat_presence_archived.go:11-15`; `xenobot/db/presence/repository.go:329-348` (`TryClaimArchived` `ON CONFLICT (chat_id, message_id) DO NOTHING`, `RowsAffected == 1`) |
| T-33-06 | 33-02 | Integrity | high | mitigate | CLOSED | `xenobot/db/presence/repository.go:148-155,164-200,355-407` (`normalizeSettings` in `Get`/`GetFresh`; `ReplacePortableTx` preserves target's `ArchiveChatID`/`ArchiveThreadID` and updates only `portableCols`); `xenobot/db/cache/local.go:63,135` (`chatPresencePrefix` in `skipLocal`) |
| T-33-07 | 33-03 | Denial of Service / Tampering | low | mitigate | CLOSED | `locales/{en,uk,ru,es,fr,hi,id,pt}.yml` (`make check-translations` passed: 1070 keys across all 8 locales); `xenobot/i18n/settings_locale_test.go:956-989,1025,1624-1640` (`settingsLocaleLimits`, `settingsAlertKeys` including `settings_ph_need_target`, `<= 14` rune `help_module_name_presence`, `<= 24` rune `archive_jump_button`) |
| T-33-08 | 33-04 | Denial of Service / Tampering | medium | mitigate | CLOSED | `xenobot/modules/reactions.go:382-424` (`isStaticWordRune` + `matchStaticReactionKeyword` using linear `strings.Index`, `utf8.DecodeLastRuneInString`, `utf8.DecodeRuneInString`, `unicode.IsLetter`/`IsDigit`; zero `regexp` imports) |
| T-33-09 | 33-04 | Information Disclosure / Integrity | high | mitigate | CLOSED | `xenobot/modules/reactions.go:583-586,513-515,549-551` (`presenceMessageAllowed` first; lazy `presenceIsLockedDown` before reacting); `xenobot/modules/presence.go:114-147` (fails closed on DB error) |
| T-33-10 | 33-04 | Integrity | low | mitigate | CLOSED | `xenobot/modules/reactions.go:426-434,522-525,569-572` (`setPresenceStaticReacted` sets `ctx.Data[presenceStaticReactedKey] = true` on static/cat success); `xenobot/modules/chat_react.go:258-262` (`chatReactWatch` skips AI reaction when `presenceStaticReactedKey` is true) |
| T-33-11 | 33-04 | Elevation of Privilege | high | mitigate | CLOSED | `xenobot/modules/settings_reactions.go:153-173,372-379,445-472` (`settingsListPrivateField` `c != ""`, `settingsRightAdmin` on `xo`/`xr`, `settingsRightChangeInfo` on `xs`, `settingsReactionsSetOptValidate`); `xenobot/db/reactions/repository.go:238-276` (`WHERE id = ? AND chat_id = ?`) |
| T-33-12 | 33-05 | Elevation of Privilege / Information Disclosure | high | mitigate | CLOSED | `xenobot/modules/presence_archive.go:123,143,148,156-160,186,194-215,252,256,319` (`RequireUserAdmin`/`CanUserChangeInfo`, `presenceArchiveBotCanPost`, and `presenceIsStaffGroup` checks execute before `consumePendingMarker`; Staff Groups refused as source and destination on link and before delivery) |
| T-33-13 | 33-05 | Integrity / Abuse | medium | mitigate | CLOSED | `xenobot/modules/presence_archive.go:287-305,312,327-366` (excludes `mru.User == nil`, `IsBot`, message author via `presenceLogAuthorOf` + `presenceReadBotReact`, and `presenceConsensusHostile`; `SADD`/`SREM` distinct user IDs via `presenceConsensusUpdateScript`) |
| T-33-14 | 33-05 | Integrity | high | mitigate | CLOSED | `xenobot/modules/presence_archive.go:376-399` (fast Redis latch `presenceArchiveDoneKey`, lazy fail-closed `presenceIsLockedDown`, atomic PostgreSQL `presenceArchiveClaim` / `TryClaimArchived`) |
| T-33-15 | 33-05 | Tampering / Broken Link | low | mitigate | CLOSED | `xenobot/modules/presence_archive.go:403-414,436-454` (`presenceArchiveJumpURL` restricts links to `Username != ""` or `chat.Type == "supergroup" && strings.HasPrefix(..., "-100")`, omitting URL markup on legacy `"group"` chats) |
| T-33-16 | 33-06 | Elevation of Privilege / Tampering | high | mitigate | CLOSED | `xenobot/db/backup/types.go:239-249` (`PresencePortableBackup` omits `archive_chat_id`/`archive_thread_id`); `xenobot/db/backup/backup.go:1427-1483` (`importPresence` zeroes target IDs and sets `report.ArchiveChannelSkipped`); `xenobot/db/presence/repository.go:355-407` |
| T-33-17 | 33-06 | Elevation of Privilege | high | mitigate | CLOSED | `xenobot/db/backup/types.go:101-149` (21 exportable modules; no `chat_conversations`); `xenobot/db/backup/backup.go:26-48,551-614,1092-1114`; `xenobot/db/backup/chatconv_backup_test.go:18-57` |
| T-33-18 | 33-06 | Integrity | medium | mitigate | CLOSED | `xenobot/db/backup/backup.go:76,898-923` (`importReactions` resets `ID = 0`, normalizes zero/invalid `ChancePct` to `100`, clamps `CooldownSecs` to `0..86400`, returns both `reactions` and `reaction_rules` cache keys for post-commit invalidation) |
| T-33-19 | 33-07 | Denial of Service / UI Regression | low | mitigate | CLOSED | `xenobot/modules/settings_phase33_test.go:28-52,115-117` (`len(settingsPages) == 31`, `settingsMainOwnerRows == 17`); `xenobot/modules/settings_presence.go:418-563`; `xenobot/modules/settings_presence_test.go:70-175` |
| T-33-20 | 33-07 | Elevation of Privilege | high | mitigate | CLOSED | `xenobot/modules/settings_presence.go:163-224,592,691-700,730,757-862` (`PrivateOnly: true`, `Hidden: settingsPresenceHidden`, `settingsPresenceGroupRefused`, fail-closed `settingsPresenceStaffGate`, `settingsRightAdmin` on `ph`, `settingsRightChangeInfo` on `cs`/`ae`/`ah`/`ap`/`ad`/`pu`, unlinked check alerting `settings_ph_need_target`) |
| T-33-21 | 33-08 | Repudiation / Integrity | low | mitigate | CLOSED | `xenobot/modules/backup.go:867-870` (`formatBackupImportSuccess` appends `backup_import_archive_skipped` when `report.ArchiveChannelSkipped` is true) |
| T-33-22 | 33-08 | Elevation of Privilege | high | mitigate | CLOSED | `xenobot/modules/copy_settings.go` untouched since pre-Phase-31 commit `85d697cd` with zero `presence`/`chat_conv` references; `xenobot/db/backup/chatconv_backup_test.go` intact |
| T-33-SC | 33-01..33-08 | Tampering | high | accept | CLOSED | Zero new external Go packages added (`go.mod` and `go.sum` untouched since `85d697cd`); documented in Accepted Risks Log |

## Accepted Risks Log

| Threat ID | Severity | Rationale | Scope |
|-----------|----------|-----------|-------|
| T-33-SC | high | Phase 33 introduces zero new Go module dependencies (`go.mod` and `go.sum` have no commits or uncommitted diffs across Phase 33); only Go stdlib and existing repo dependencies are used. | Plans `33-01`..`33-08` |

## Unregistered Flags

None. All 8 plan summaries (`33-01-SUMMARY.md`..`33-08-SUMMARY.md`) reported no unregistered threat flags, and code inspection found no unmapped attack surface.
