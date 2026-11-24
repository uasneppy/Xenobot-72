---
phase: "29"
slug: "plain-tone-and-copy-settings"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-11-23"
---

# Phase 29 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> Mode: Plan-authored threat register (`register_authored_at_plan_time: true`; `T-29-01` through `T-29-17` + `T-29-SC` verified against implementation and tests).

---

## Trust Boundaries

| Boundary | Plan(s) | Description |
|----------|---------|-------------|
| Golden baseline boundary (`phase29_legacy_request_test.go`) | 29-01 | Test-only characterization golden committed alone in Wave 1 (`b4e8da48`) before any Phase 29 production edit; pins all 23 group defaults, all 18 PM defaults, unconfigured `postEventSent`, and uncopied-group isolation against `ca060519`. |
| Private callback (`settings\|v1\|a=ht&c=...&v=...`) -> Messages hub handler -> `chat_event_message_options` | 29-02 | Untrusted callback data toggles a group's Plain tone setting (`plain_tone`). |
| Replica A `SetPlainTone` -> Replica B punishment hot path | 29-02 | Plain tone changes must apply at the next event across all replicas without stale local cache or clobbering `pm_details`/`pm_log`. |
| Untrusted user display names / reasons / group titles -> plain-tone HTML templates -> Telegram `parse_mode=HTML` | 29-03 | User-supplied strings are interpolated into all 23 group and 18 PM plain-tone templates across 8 locales. |
| Source DB rows (possibly legacy or tampered) -> `eventmsg.ReplaceAllValidated` / `applyCopySettings` -> Target group tables | 29-04 | Copy Settings transfers messages, button URLs, appeal URLs, and tuning integers across groups. |
| Copy Settings write scope -> Excluded group tables (`staff_group_links`, `approvals`, `warns_users`, `chat_offences`, `blacklists` words, `chat_settings_access`, content tables) | 29-04 | Scope creep could leak or overwrite access control, approvals, member offences, or Staff Group links. |
| Private callback (`settings\|v1\|a=ck/cy&c=...&t=.../k=...`) -> `settings_copy.go` -> Redis token + Telegram `getChatMember`/`getChat` + `applyCopySettings` | 29-05 | Untrusted callback data specifies target chat ID or confirm token for cross-group writes. |
| Target-group probe (`ck` with arbitrary negative chat ID `t`) -> Telegram API -> Callback alert returned to caller | 29-05 | An attacker could probe arbitrary chat IDs to learn whether the bot is in a chat, whether it is a Staff Group, or what its settings access mode is. |
| Logs & monitoring sink <- Phase 29 runtime (`settings_copy.go`, `copy_settings.go`, `event_message_apply.go`, `event_message_plain.go`) | 29-06 | Application logs must never receive user/group PII, message content, file IDs, URLs, or one-time Redis tokens. |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-29-01 | Tampering | Golden assertions in `phase29_legacy_request_test.go` (weakening or re-recording to hide tone-off default regression) | medium | mitigate | `fuku/modules/phase29_legacy_request_test.go:31-76` pins `goldenP29GroupDefaults` (23 events) & `goldenP29PMDefaults` (18 PM specs); `git log --oneline ca060519..HEAD -- fuku/modules/phase29_legacy_request_test.go` shows exactly 1 commit (`b4e8da48`), decoupled from internal PM composer signatures (`composeEventPM`/`eventPMDefaultText` not called directly). | closed |
| T-29-02 | Elevation of Privilege | `settingsActMsgPlainTone` (`ht`) in `fuku/modules/settings_messages.go` | high | mitigate | `fuku/modules/settings_messages.go:381-383,596-599,822-831` registers `ht` with `Fields: []string{"v"}`, `Right: settingsRightChangeInfo`, `settingsMsgPlainToneFields` requiring private `c` (`settingsListPrivateField`) and `v` in `{"0","1"}` (`settingsEditSwitchValue`) in step 1 (0 `getChatMember` calls on forged/group presses); verified in `fuku/modules/settings_messages_safety_test.go` and `fuku/modules/settings_phase29_test.go:34,90-138` (`TestPhase29Codes`). | closed |
| T-29-03 | Tampering / Integrity | Cross-replica staleness or column clobbering on `chat_event_message_options` | medium | mitigate | `fuku/db/eventmsg/repository.go:226-253` (`SetPlainTone` + `upsertOptions`) updates only `plain_tone`, `updated_by`, `updated_at` on `chat_id` conflict (preserving `pm_details` and `pm_log`) and calls `cache.DeleteCache(CacheKey(chatID))`; `event_messages:` prefix is listed in `skipLocal` (`fuku/db/cache/local.go:56,121`); verified in `fuku/db/eventmsg/repository_test.go`. | closed |
| T-29-04 | Tampering | AST boundary bypass (`event_message_plain.go` reaching event engine or Staff Group paths) | medium | mitigate | `fuku/modules/event_message_plain.go:1-308` contains pure string builders only with zero references to any of the 10 `eventEngineEntries`; verified by `TestEventEngineCallers` and `TestPMCopyCallers` passing with zero allow-list modifications. | closed |
| T-29-05 | Tampering / Injection | HTML injection via unescaped `{user}`, `{reason}`, `{admin}`, or `{group}` in plain-tone builders (`event_message_plain.go`, `event_message_pm.go`) | high | mitigate | `fuku/modules/event_message_plain.go:22-34,43,53-62,137,248,258,301` escapes user/admin mentions via `formatting.MentionHtml`, `memberReqMention`, `lockdownMention`, and splices them post-interpolation via `eventPlainRender`/`lockdownSplice`, plus escapes reasons/times via `html.EscapeString`; `fuku/modules/event_message_pm.go:430-454` escapes `{group}` via `staffDisplayTitle` and `{action}`/`{reason}` via `html.EscapeString`; verified across all 23 group + 18 PM events x 8 locales with `<script>&"1` payloads in `fuku/modules/event_message_plain_test.go:81-165` (`TestPlainToneEveryEventEveryLocale`). | closed |
| T-29-06 | Information Disclosure | Plain-tone `fwdblock` or `aispam` messages leaking blocked source, Staff Group, or AI provider/model/score | medium | mitigate | `fuku/modules/event_message_plain.go:164-179` (`blacklistPlainDefaultReply` receives only `staff_fwdblock_reason` for `fwdblock`) and `fuku/modules/event_message_plain.go:208-212` (`aispamPlainDefaultReply` interpolates only `{user}`); verified by `TestPlainToneRuntime/fwdblock_and_aispam_plain_privacy` in `fuku/modules/event_message_plain_test.go`. | closed |
| T-29-07 | Tampering / Injection | Malicious button URL (`javascript:`, `tg:`), oversized text/caption, disallowed media on `mreq*`, or invalid appeal URL copied to target | high | mitigate | `fuku/db/eventmsg/validate_write.go:64-142` (`ValidateMessage` + `ReplaceAllValidated`) enforces `ValidEvent`, `ValidDest`, `HasPM`, `allowedMediaType`, `NoMedia`, `content.ValidButtonURL` (line 94), 100-button & 8-per-row caps with `ReservedButtonsFor`, 4096/1024 limits with `ReservedTextFor`, and `NoSilent`; `fuku/modules/copy_settings.go:143-149` re-validates `AppealURL` via `validateAppealURL`; verified in `fuku/db/eventmsg/validate_write_test.go` (`TestEventMsgValidateAndReplaceAll`) and `fuku/modules/copy_settings_test.go` (`invalid_source_message_row_skipped_by_validated_writer`). | closed |
| T-29-08 | Elevation of Privilege / Tampering | Copy Settings modifying `staff_group_links`, `approvals`, `chat_settings_access`, `warns_users`, `chat_offences`, or blacklist word lists | high | mitigate | `fuku/modules/copy_settings.go:179-247` (`applyCopySettings`) writes strictly to the 9 allow-listed tables across 6 areas; line 214 uses `blacklists.SetBlacklistActionTimedContext` to update `action`/`action_secs` on existing target words without copying/adding/deleting words; verified in `fuku/modules/copy_settings_test.go:150-350` (`TestCopySettingsMirrorAndExclusions/full_mirror_populated_source`) and `fuku/modules/phase29_e2e_test.go:421-454`. | closed |
| T-29-09 | Integrity | `antiflood.SetFloodActionContext` `default:5` silently enabling antiflood on an unconfigured target group | high | mitigate | `fuku/db/antiflood/repository.go:122-139` (`SetFloodActionPreserveOffContext`) executes raw SQL `INSERT INTO antiflood_settings (chat_id, flood_limit, action, action_secs, ...) VALUES (?, 0, ?, ?, ...)` with `ON CONFLICT (chat_id) DO UPDATE SET action = EXCLUDED.action, action_secs = EXCLUDED.action_secs, updated_at = EXCLUDED.updated_at`, explicitly bypassing GORM's `default:5` tag; verified in `fuku/db/antiflood/repository_test.go`, `fuku/modules/copy_settings_test.go`, and `fuku/modules/phase29_e2e_test.go:422-425`. | closed |
| T-29-10 | Integrity | Drift between `eventmsg.ValidateMessage` reserves/rules and `settings_messages.go` editor rules | medium | mitigate | `fuku/modules/copy_settings_test.go:65-148` (`TestEventMsgValidateWriteMatchesSettingsPage`) compares `MaxTextLength`, `MaxCaptionLength`, `MaxButtons`, `MaxRowButtons`, `MreqTextReserve`, `LockdownTextReserve`, `HasPM`, `NoMedia`, `NoSilent`, `ReservedButtonsFor`, and `ReservedTextFor` 1-to-1 against `settingsMsgFamilies`, `eventPMSpecs`, and `settingsEditPageByKey`. | closed |
| T-29-11 | Information Disclosure | Target group probing via forged `ck` callbacks with arbitrary `t` IDs (`settings_copy.go`) | high | mitigate | `fuku/modules/settings_copy.go:342-399` (`settingsCopyVerifyTarget` + `settingsCopyAnswerVerdict`) maps all 6 target refusal causes (bot not in chat 4xx, non-admin status, `settingsGateOwnerOnly`, missing target write rights, non-group chat type, and fresh Staff Group row) to `copyTargetUnavailable` -> uniform opaque `settings_pick_unavailable` alert; verified in `fuku/modules/settings_copy_test.go` (`live_dual_group_auth_and_uniform_opaque_refusal`) and `fuku/modules/settings_phase29_test.go:326-342`. | closed |
| T-29-12 | Elevation of Privilege | Admin demoted or rights revoked between `ck` (ask) and `cy` (Yes), or non-creator admin missing `can_change_info`/`can_restrict_members` in target | high | mitigate | `fuku/modules/settings_copy.go:323-390,419-427,462-470` executes `settingsCopyVerifySource` and `settingsCopyVerifyTarget` (including line 364: `member.Status != gotgbot.ChatMemberStatusCreator && (!member.CanChangeInfo || !member.CanRestrictMembers)`) on BOTH `ck` and `cy` before token consumption and before `applyCopySettings`; verified in `fuku/modules/settings_copy_test.go`. | closed |
| T-29-13 | Replay / Tampering | Replayed, stale, concurrent, or cross-user `cy` Yes presses | high | mitigate | `fuku/modules/settings_copy.go:44-58,75-80,175-252,444-486` generates a 96-bit `crypto/rand` 16-char base64url token under `fuku:settings:copy:<token>` (`SET NX`, TTL `settingsCardLifetime` = 5m), checks `rec.UserID == p.query.From.Id && rec.SourceChatID == p.group.Id` via `settingsCopyPeekToken` before deleting (so cross-user/source presses cannot invalidate a valid owner's card), and atomically consumes the token via Lua `GET+DEL` (`settingsCopyConsumeScript`); verified in `fuku/modules/settings_copy_test.go` (`replay_expired_wrong_user_wrong_source_and_no_redis`) and `fuku/modules/phase29_e2e_test.go:465-474`. | closed |
| T-29-14 | Tampering / Injection | Malicious HTML in source or target group title rendered on confirm card or copied notice | medium | mitigate | `fuku/modules/settings_copy.go:493,564-565` escapes source and target group titles via `html.EscapeString(settingsCopyTitleOrID(...))` before interpolating into `settings_copy_confirm_head`, `settings_notice_copied`, and `settings_notice_copied_skipped`; verified in `fuku/modules/settings_copy_test.go` and `fuku/modules/phase29_e2e_test.go:341-365`. | closed |
| T-29-15 | Denial of Service | Unbounded callback payloads (> 64 bytes) or Redis unavailable during card creation | medium | mitigate | `fuku/modules/settings_copy.go:106-169,184-187,433-437` enforces step-1 validation (`private(...)`, `settingsCopyPageValid`, `settingsCopyPickValid`, `settingsCopyYesValid`) with 0 Telegram calls on invalid input and fails closed with `settings_copy_unavailable` when Redis is nil/unavailable; `fuku/modules/settings_copy_test.go` (`worst_case_64_byte_callbacks`) verifies all callbacks `<= 64` bytes at `math.MinInt64`. | closed |
| T-29-16 | Information Disclosure | Log leakage of group titles, usernames, custom message texts, file IDs, button/appeal URLs, or copy tokens (`settings_copy.go`, `copy_settings.go`, `event_message_apply.go`) | medium | mitigate | `fuku/modules/copy_settings.go:186-240`, `fuku/modules/settings_copy.go:313-484`, and `fuku/modules/event_message_apply.go:142-145` log only numeric chat/user IDs, step names, and counts; `fuku/modules/settings_phase29_test.go:224-428` (`TestPhase29StaffPrivacyAndLogs`) captures debug-level `logrus` output across all flows with 10 sentinels (group/Staff titles, owner name, message text, file ID, button URL, appeal URL, shortcut reason, and 16-char copy token) and asserts 0 leaks. | closed |
| T-29-17 | Integrity / Regression | Unnoticed drift in Phase 29 goldens, shard lists, docs, translations, or AST caller guards | medium | mitigate | All local gates (`make check-translations`, `make check-docs`, `make test-shard-check`, `go vet -tags testtools ./...`, `CGO_ENABLED=0 go build ./...`), AST caller guards (`TestEventEngineCallers`, `TestPMCopyCallers`), and git immutability checks on `phase29_legacy_request_test.go` (1 commit `b4e8da48`), pre-existing `*_legacy_request_test.go` files (0 diffs), and `go.mod`/`go.sum` (0 diffs) pass. | closed |
| T-29-SC | Tampering | Go module dependencies (`go.mod` / `go.sum`) | low | accept | `git diff --exit-code ca060519 -- go.mod go.sum` exits `0`; zero new third-party dependencies added in Phase 29. | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-29-01 | T-29-SC | No third-party Go module dependency was added or upgraded in Phase 29 (`git diff --exit-code ca060519 -- go.mod go.sum` exited 0). | Phase 29 security audit | 2026-11-23 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-11-23 | 18 | 18 | 0 | gsd-security-auditor (plan-authored register, ASVS L1) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-11-23
