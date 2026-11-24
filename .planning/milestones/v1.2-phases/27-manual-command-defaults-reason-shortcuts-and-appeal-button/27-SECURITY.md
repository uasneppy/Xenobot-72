---
phase: "27"
slug: "manual-command-defaults-reason-shortcuts-and-appeal-button"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-11-23"
---

# Phase 27 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> Mode: Plan-authored threat register (`register_authored_at_plan_time: true`; `T-27-01` through `T-27-19` + `T-27-SC` verified against implementation and tests).

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Group command text -> moderation handlers | Untrusted admin/member command arguments reach duration and reason parsing (`moderation.go`, `bans.go`, `mute.go`, `warns.go`) | Target user/reply, duration tokens, reason text, shortcut words |
| DB/Redis tuning read -> moderation execution | Cached `chat_manual_defaults` (`skipLocal`) controls whether a manual punishment proceeds or becomes timed (`modtuning_settings.go`) | `default_ban_secs`, `default_mute_secs`, `reason_required`, `appeal_url` |
| Stored shortcut full text -> HTML Telegram messages | Owner-authored shortcut text in `chat_reason_shortcuts` is interpolated into HTML-formatted group replies, custom `{reason}` messages, PM copies, warn-limit messages, and log channel posts | Raw stored shortcut text, HTML-escaped at every render surface |
| Shortcut word / text / appeal URL -> structured logs | Owner-defined shortcut text and appeal URLs must not leak into application logs (`modtuning_settings.go`, `settings_moddefaults.go`, `settings_reasons.go`) | Chat/user/row IDs only (no secrets, URLs, or shortcut text) |
| Owner-typed appeal URL -> Telegram inline keyboard `Url` button | Untrusted URL string placed on public group and PM buttons sent to banned users (`event_message.go`, `event_message_pm.go`) | Validated `http`/`https` URL (`<= 2048` runes, non-empty host) |
| Combined inline keyboards -> Telegram 100-button limit | Custom buttons + bot buttons + appeal button + Details button must stay within Telegram's 100-button ceiling (`event_message.go`, `event_message_pm.go`, `settings_messages.go`) | Inline keyboard button rows |
| Telegram callback query -> `settings_moddefaults.go` & `settings_reasons.go` | Forged callback data (bad `k`, arbitrary `v`, foreign-chat row ID `r`, extra keys, group-menu `u` on private-only codes) crosses into settings dispatch | Callback fields, live `getChatMember` / `getChat` checks |
| Private messages -> `settingsTypedPages["q"]` & `settings_list_input.go` (`ApplyPlain`) | Untrusted private messages (redeliveries, media-only, oversized text, HTML markup, mid-wait demotion) enter typed-wait flows | Private message text, wait nonces, live authority checks |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-27-01 | Tampering | `fuku/modules/moderation_phase27_legacy_request_test.go` | medium | mitigate | Exactly 1 commit (`71dbf865`, committed alone before production changes); `moderation_legacy_request_test.go` has 0 diff vs `b7393953`; both golden suites pass on HEAD. | closed |
| T-27-02 | Denial of Service | `manualDefaultsFor` on DB/Redis outage during moderation | high | mitigate | `modtuning_settings.go:59,187-196` bounds reads with `manualDefaultsReadTimeout = 2s`, logs one IDs-only Warn, and fails open returning `modtuning.Defaults{}` (`TestManualCommandDefaults`). | closed |
| T-27-03 | Tampering | Out-of-range duration (`< 60` or `> 31622400`) in `chat_manual_defaults` | medium | mitigate | Enforced in SQL CHECKs (`chk_chat_manual_defaults_ban`/`mute`), GORM model + `modtuning.ValidDefaultDuration`, `applyDefaultBanSecs`/`applyDefaultMuteSecs` (`punishDurationValid`), and runtime `manualDefaultUntil` + `punishUntilTooClose`. | closed |
| T-27-04 | Repudiation / Integrity | Concurrent edits clobbering sibling defaults or stale local cache across replicas | medium | mitigate | Column-targeted `OnConflict` upserts + `cache.DeleteCache` (`fuku/db/modtuning/repository.go:111-169,291,314`); `chat_manual_defaults:` and `chat_reason_shortcuts:` in `skipLocal` (`fuku/db/cache/local.go:59-60,117-118`). | closed |
| T-27-05 | Elevation of Privilege / Scope Leak | Staff Group or automatic rules (`warnSourceAuto`) inheriting per-group defaults | medium | mitigate | `manualApplyReasonRequired` and `manualBanMember`/`manualMuteMember` run only in manual paths (`moderation.go`, `bans.go`, `mute.go`, `warns.go:dispatchWarn`); zero references in `staff*.go` or `lockdown*.go`. | closed |
| T-27-06 | Tampering / Injection | `expandManualReason` output rendered into HTML Telegram messages | high | mitigate | Stored raw plain text and HTML-escaped at every render site (`moderationReasonLine`, `eventPlaceholderValues`, `eventPMDefaultText`, `warnDefaultReply`, `warnLimitDefaultReply`, `warnPostLimit`, `actionlog.Admin`, `settingsListLabel`); verified with `<promo> &` in `TestReasonShortcutExpansion`. | closed |
| T-27-07 | Denial of Service | Unbounded shortcut count or huge shortcut text | medium | mitigate | Enforced via `ReasonShortcutsCapMax=50`, `ReasonShortcutWordMaxRunes=32`, `ReasonShortcutTextMaxRunes=500` in SQL CHECKs, repo guards, Go validation + fresh cap check; zero-I/O empty/multi-word fast-path in `expandManualReason`. | closed |
| T-27-08 | Information Disclosure | Shortcut word/reason written to application logs | low | mitigate | All `modtuning_settings.go` log lines emit `chatID`, `userID`, and `id` only; verified by logrus capture in `TestReasonShortcutValidationAndApply`. | closed |
| T-27-09 | Tampering / Injection | `applyAppealURL` & `eventAppealRow` accepting unsafe URI schemes | high | mitigate | `validateAppealURL` enforces trimmed non-blank `<= 2048` runes, zero whitespace, and `content.ValidButtonURL` (strict `http`/`https` + non-empty host) on write AND defensively on read in `manualAppealURLFor` and `eventAppealRow` (`TestAppealURLValidationAndApply`). | closed |
| T-27-10 | Denial of Service | Keyboard exceeding Telegram's 100-button cap | medium | mitigate | `eventAppendAppealRow` checks `pmCopyButtonCount(rows)+len(appealRow) > settingsEditMaxButtons` (100) before appending across group, custom, and PM sends; static `ReservedButtons` updated in `settings_messages.go` (`TestAppealButtonOnBanMessages`). | closed |
| T-27-11 | Information Disclosure | Appeal URL leaked in logs or Staff Group / raid-scale bans | low | mitigate | Logs emit IDs only; `eventSupportsAppeal` excludes negative target IDs (channel bans) and non-ban events; zero appeal calls in `staff*.go` or `lockdown*.go` (`TestEventEngineCallers`, `TestPMCopyCallers`). | closed |
| T-27-12 | Elevation of Privilege | `settings_moddefaults.go` write actions (`mt`, `mr`, `ma`, `mx`) and typed appeal reply | high | mitigate | All write actions and `settingsTypedPages["q"]` declare `Right: settingsRightRestrict`, evaluated live via `r.authorize` (`getChatMember` + owner gate + right check) in `settings_input_door.go` (`TestSettingsModDefaults`). | closed |
| T-27-13 | Tampering | Forged callback fields on page `q` | medium | mitigate | Every action wraps `Validate` with `private(...)` requiring `fields["c"] != ""` and strict field allow-lists (`k` in `b|m`, `v` in preset/0, `settingsEditSwitchValue`, `t == "a"`) (`TestPhase27Codes`, `TestSettingsModDefaults`). | closed |
| T-27-14 | Information Disclosure / Cross-Group Leak | Staff Group displaying Manual defaults or group-menu leaking appeal URL | medium | mitigate | Page `q` declares `PrivateOnly: true`, `PrivateNoteKey: "settings_moddef_group_note"`, `Hidden: settingsModDefaultsHidden` (`staff.GetStaffGroupFresh`); appeal URL truncated and HTML-escaped on render, never in callbacks or logs. | closed |
| T-27-15 | Elevation of Privilege / Cross-Tenant IDOR | `settingsReasonsEntry` / `settingsReasonsRemove` with foreign-chat row ID `r` | high | mitigate | `modtuning.GetShortcutFresh` and `modtuning.RemoveShortcutByID` scope SQL strictly with `WHERE id = ? AND chat_id = ?` (`TestManualDefaultsAndShortcutsCRUD`, `TestSettingsReasons`). | closed |
| T-27-16 | Elevation of Privilege | Unprivileged admin or demoted user adding/deleting shortcuts mid-wait | high | mitigate | `settingsReasonsDef()` sets `Right: settingsRightRestrict`, applied to `sx`/`sy`/`sa` and re-authorized live via `r.authorize(def.Right)` on both Step 1 and Step 2 before any write (`TestSettingsReasons`). | closed |
| T-27-17 | Tampering / Race Condition | Duplicate word overwrite, cap bypass, or step-1 redelivery accepted as step-2 | medium | mitigate | `listStepTwo` drops `r.msg.MessageId <= r.wait.StepMsgID`, re-runs fresh `r.listStepCheck` before `claimWait()`, and saves via `UNIQUE (chat_id, word)` + `ON CONFLICT DO NOTHING` (`RowsAffected == 1`). | closed |
| T-27-18 | Information Disclosure | Staff Group titles, shortcut texts, and appeal URLs leaking into UI or logs | medium | mitigate | Verified by `TestPhase27StaffPrivacyAndLogs` exercising pages `q` and `rs` and `/ban` with unique sentinels across UI, buttons, answers, and captured logrus output. | closed |
| T-27-19 | Tampering / Regression | Golden modification, unguarded action codes, or docs/translation/shard drift | medium | mitigate | `moderation_phase27_legacy_request_test.go` has 1 commit (`71dbf865`), `moderation_legacy_request_test.go` and `go.mod`/`go.sum` untouched, `TestPhase27Codes` guards all 11 codes, and `check-translations`, `check-docs`, `test-shard-check` pass. | closed |
| T-27-SC | Tampering | Go module dependencies (`go.mod` / `go.sum`) | low | accept | No third-party dependency added or upgraded in Phase 27 (`git diff --stat b7393953 -- go.mod go.sum` is empty). | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-27-01 | T-27-SC | No Go module dependencies were added or upgraded in Phase 27 (`go.mod` and `go.sum` have zero diff since `b7393953`). | Phase 27 security audit | 2026-11-23 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-11-23 | 20 | 20 | 0 | gsd-security-auditor (plan-authored register, ASVS L1) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-11-23
