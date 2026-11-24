---
phase: "30"
slug: "backup-page-and-custom-messages-in-backups"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-11-23"
---

# Phase 30 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> Mode: Plan-authored threat register (`register_authored_at_plan_time: true`; `T-30-01` through `T-30-12` + `T-30-SC` verified against implementation and tests).

---

## Trust Boundaries

| Boundary | Plan(s) | Description |
|----------|---------|-------------|
| Golden baseline boundary (`phase30_legacy_request_test.go`) | 30-01 | Test-only characterization golden committed alone in Wave 1 (`f73400f4`) ahead of all Phase 30 production changes; no production input crosses a new boundary. |
| Untrusted backup JSON file (`"messages"` module payload) -> `backup.ImportChatDataReport` -> `eventmsg.ReplaceAllValidatedTx` -> PostgreSQL `chat_event_messages` & `chat_event_message_options` | 30-02 | User-supplied backup JSON could carry malicious button URLs (`javascript:`, `tg://`), oversized texts/captions breaking member-requirement or lockdown reserves, disallowed media types, unknown event keys, or non-object JSON shapes. |
| Uploaded `.json` backup document -> `downloadAndParseBackupFile` -> `settingsBackupPutImportToken` -> Redis `fuku:settings:backup:import:<token>` | 30-03 | Parsed backup payload is temporarily stored in Redis between private file upload and two-tap Yes confirmation across replicas (2 MiB cap, 5m TTL). |
| Confirm callback (`k=<token>`) -> `settingsBackupPeekImportToken` / `settingsBackupConsumeImportToken` / `settingsBackupConsumeResetToken` | 30-03 | Concurrent, cross-user, or replayed Yes presses across replicas could attempt double import, double reset, or token invalidation. |
| Private callback (`settings\|v1\|a=bv/be/bi/bl/bj/bq/bu&c=...`) -> `settings_backup.go` | 30-04 | Untrusted callback data specifies canonical 20-bit module bitmasks (`m`) or one-time confirmation tokens (`k`) for export, import, or reset. |
| Private message during active import wait -> `settings_input_door.go` -> `settingsBackupImportTyped` -> `downloadAndParseBackupFile` | 30-04 | Untrusted private message/document uploaded by a user during an active `bi` import wait. |
| Logs & monitoring sink <- Phase 30 runtime (`settings_backup.go`, `backup.go`, `fuku/db/backup/backup.go`) | 30-05 | Application logs must never receive group titles, Staff Group titles/IDs, usernames, custom message content, file IDs, button URLs, or one-time Redis backup confirmation tokens. |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-30-01 | Tampering | Golden assertions in `phase30_legacy_request_test.go` | medium | mitigate | `fuku/modules/phase30_legacy_request_test.go:123-571` defines `TestLegacyPhase30Baseline` (5 subtests with explicit module lists, no hardcoded 19-module length). `git log --oneline 1da5af6f..HEAD -- fuku/modules/phase30_legacy_request_test.go` has exactly 1 commit (`f73400f4`, 1 file changed), `git log --oneline 1da5af6f..f73400f4 -- fuku migrations locales ':!*_test.go'` is empty, zero pre-existing `*_legacy_request_test.go` files modified since `1da5af6f`, and `TestLegacyPhase30Baseline` passes under `-race`. | closed |
| T-30-02 | Tampering / Injection | Untrusted custom messages in imported backup JSON (`fuku/db/backup/backup.go` `importMessages`) | high | mitigate | `fuku/db/backup/backup.go:610-611,1389-1420` routes `BackupModuleMessages` through `decodeModuleData` and `eventmsg.ReplaceAllValidatedTx(tx, chatID, rows, opts, 0)` (ignoring JSON `chat_id`), adds `len(writeReport.Skipped)` to `report.Skipped`, and returns `eventmsg.CacheKey(chatID)` for post-commit invalidation. `fuku/db/eventmsg/validate_write.go:65-142` (`ValidateMessage`) enforces `ValidEvent`, `ValidDest`, `HasPM`, `allowedMediaType`, `NoMedia`, `content.ValidButtonURL` (line 94), `MaxButtons`/`MaxRowButtons` with `{rules}` and `ReservedButtonsFor`, `MaxTextLength`/`MaxCaptionLength` minus `ReservedTextFor`, and `NoSilent`. Proven under `-race` by `TestEventMessagesImportValidationAndSkipped`, `TestEventMsgValidateAndReplaceAll`, `TestSharedBackupApplyHelpers`, `TestSettingsBackupPage`, and `TestPhase30EndToEnd`. | closed |
| T-30-03 | Denial of Service / Data Corruption | Nested database transaction inside `withBackupTx` breaking rollback atomicity / SQLite lock | high | mitigate | `fuku/db/eventmsg/validate_write.go:149-227` (`ReplaceAllValidatedTx`) rejects `tx == nil` (`errNoDatabase`), runs directly on caller's `tx *gorm.DB` with no nested transaction and no premature cache invalidation; `ReplaceAllValidated` (lines 234-254) wraps it in a single transaction; `fuku/db/backup/backup.go:69-84,1414,1423` calls `ReplaceAllValidatedTx(tx, ...)` inside `withBackupTx` and invalidates caches only after commit (line 82). Proven under `-race` by `TestReplaceAllValidatedTx`, `TestEventMessagesInBackup`, and `TestImportChatDataRollsBackEarlierModules`. | closed |
| T-30-04 | Information Disclosure / Scope Creep | Tuning or Staff Group tables accidentally included in backups | medium | mitigate | `fuku/db/backup/types.go:96-142` adds only `BackupModuleMessages = "messages"` (20 modules total); `fuku/db/backup/backup.go:537-553,1389-1427` touches only `ChatEventMessage` and `ChatEventMessageOptions`. `TestEscalationStaysOutOfBackup`, `TestModTuningExcludedFromBackup`, `TestAdvThresholdsExcludedFromBackup`, `TestChatConversationsStayOutOfBackup`, and `TestEventMessagesInBackup` all pass under `-race`. | closed |
| T-30-05 | Denial of Service | Redis memory exhaustion via oversized pending import payload (`settingsBackupPutImportToken`) | medium | mitigate | `fuku/modules/backup.go:98,564` and `fuku/modules/federations_io.go:223-229` cap downloaded files at `maxBackupFileSize` (10 MiB via `io.LimitReader`); `fuku/modules/backup.go:112,240-273,342-344` enforces `len(payload) <= maxSettingsBackupRedisBytes` (2 MiB, returning `errSettingsBackupPayloadTooLarge` before `SetNX`) and TTL `settingsCardLifetime` (5m), failing closed (`errSettingsCopyNoRedis`) when Redis is nil; `fuku/modules/settings_backup.go:341-350` surfaces `backup_import_file_too_large` / `settings_bkp_unavailable`. Proven under `-race` by `TestSettingsBackupRedisTokens/import_token_rejects_oversized_payload_and_nil_redis`. | closed |
| T-30-06 | Replay / Race Condition | Concurrent or replayed Yes presses on Import or Reset confirm cards across replicas | high | mitigate | `fuku/modules/backup.go:109-110,255,265,298-319` generates 96-bit `crypto/rand` 16-char base64url tokens (`settingsCopyNewToken`), writes under `fuku:settings:backup:{import,reset}:` via `SET NX` with `settingsCardLifetime` TTL, and consumes atomically via Lua `GET+DEL` (`settingsCopyConsumeScript`). Proven with 8 concurrent goroutines + replay checks under `-race` in `TestSettingsBackupRedisTokens`, `TestSettingsBackupPage`, and `TestPhase30EndToEnd`. | closed |
| T-30-07 | Elevation of Privilege | Non-owner admin attempting Import (`bi`/`bj`) or Reset (`bq`/`bu`), or demotion between ask and Yes | high | mitigate | `fuku/modules/settings_backup.go:70-102` enforces `settingsRightOwnerBotAdmin` on `bi`, `bj`, `bq`, `bu`, and `settingsTypedPages[settingsPageBackup]`, and `settingsRightAdminBotAdmin` on `be`. Every callback runs live `lockdownLiveMember`, `settingsGate`, and `checkRight` (`fuku/modules/settings_menu.go`); every typed upload runs `r.authorize(typed.Right)` with live `lockdownLiveMember`, `settingsGate`, `IsBotAdmin`, and `ChatMemberStatusCreator` (`fuku/modules/settings_input_door.go:226-232,289-349`) before `settingsBackupImportTyped`. Proven under `-race` by `TestSettingsBackupPage` and `TestPhase30EndToEnd`. | closed |
| T-30-08 | Information Disclosure | Backup file posted in group chat or Backup page accessible in group menu or Staff Group | high | mitigate | `fuku/modules/settings_backup.go:55-95` sets `PrivateOnly: true`, `PrivateNoteKey: "settings_bkp_group_note"`, `Hidden: settingsBackupHidden` (fresh `staff.GetStaffGroupFresh`), and wraps all 7 actions in `private(...)` (refused in step 1 on group menus with 0 `getChatMember` calls); `fuku/modules/settings_menu.go:600-613,695-706` blocks Staff Groups and renders the private-only note with 0 DB reads; `settingsBackupVerifyNonStaff` and `settingsBackupImportTyped` fail closed on fresh Staff Group checks; `applyBackupExport` (`fuku/modules/backup.go:431-438`) always delivers via `b.SendDocument(userID, ...)`. Proven under `-race` by `TestSettingsBackupPage`, `TestPhase30StaffPrivacyAndLogs`, `TestPhase30EndToEnd`, and `TestLegacyPhase30Baseline`. | closed |
| T-30-09 | Replay / Tampering | Cross-user token theft, replayed Yes (`bj`/`bu`), or forged bitmask `m` | high | mitigate | Step-1 validation (`fuku/modules/settings_menu.go:355-378,436`; `fuku/modules/settings_backup.go:113-131`; `fuku/modules/settings_copy.go:153-169`) rejects non-canonical hex `m`, non-16-char base64url `k`, extra keys, and group fields with 0 `getChatMember` calls. `settingsBackupRunImportYes` and `settingsBackupRunResetYes` peek (`GET`) and verify `rec.UserID == p.query.From.Id && rec.ChatID == p.group.Id` before atomic Lua `GET+DEL` consumption (and re-check after consume), preventing cross-user presses from invalidating the owner's card. Proven under `-race` by `TestSettingsBackupPage` and `TestPhase30Codes`. | closed |
| T-30-10 | Denial of Service | Rate-limit bypass between `/export`, `/import`, `/reset` and the Backup page | medium | mitigate | `be` -> `applyBackupExport`, `bj`, and `bu` acquire cooldowns from the exact same `ratelimit.GetBackupRateLimiter()` singleton (`AcquireExport`/`AcquireImport`/`AcquireReset`) used by `/export`, `/import`, and `/reset`. Proven under `-race` by `TestSettingsBackupPage`, `TestSharedBackupApplyHelpers`, and `TestPhase30EndToEnd`. | closed |
| T-30-11 | Information Disclosure | Log leakage of group titles, Staff Group titles, usernames, custom message texts, file IDs, button URLs, or backup confirmation tokens | medium | mitigate | Every `log.*` call in `fuku/modules/settings_backup.go` and `fuku/modules/backup.go` logs numeric chat/user IDs and module counts only. `TestPhase30StaffPrivacyAndLogs` captures live `logrus` output at `DebugLevel` across `ba`/`bv`, Export (`be`), Reset (`bq`/`bu`), Import (`bi`/doc/`bj`), and Staff Group refusals and asserts 0 leaks across all 10 sentinels (including both 16-char Redis tokens) in logs and UI. Passes under `-race`. | closed |
| T-30-12 | Integrity / Regression | Unnoticed drift in Phase 30 goldens, shard lists, docs, translations, or AST caller guards | medium | mitigate | Verified directly: `make check-translations` (1068 keys across 8 locales), `make check-docs` (0 drift), `make test-shard-check` (0 diff), `go vet -tags testtools ./...`, `CGO_ENABLED=0 go build ./...`, `./fuku/db/eventmsg`, `./fuku/db/backup`, `./fuku/i18n`, all Phase 30 & AST guard tests (`TestEventEngineCallers`, `TestPMCopyCallers`, `TestNoRawNewCommand`, `TestCommandRepliesStayInsideTheHandler`, `TestEventMsgValidateWriteMatchesSettingsPage`, `TestMemberDeletionExcludedMessages/no_file_of_the_never-delete_families_calls_a_deletion_producer`), and git immutability invariants (`phase30_legacy_request_test.go` = 1 commit `f73400f4`, pre-existing `*_legacy_request_test.go` = 0 diffs, `go.mod`/`go.sum` = 0 diffs) all exit `0`. | closed |
| T-30-SC | Tampering | Go module dependencies (`go.mod` / `go.sum`) | low | accept | `git diff --exit-code 1da5af6f -- go.mod go.sum` exited `0`; zero third-party Go module dependencies were added or changed in Phase 30. | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-30-01 | T-30-SC | No third-party Go module dependency was added, upgraded, or removed in Phase 30 (`git diff --exit-code 1da5af6f -- go.mod go.sum` exited 0). | Phase 30 security audit | 2026-11-23 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-11-23 | 13 | 13 | 0 | gsd-security-auditor (plan-authored register, ASVS L1) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-11-23
