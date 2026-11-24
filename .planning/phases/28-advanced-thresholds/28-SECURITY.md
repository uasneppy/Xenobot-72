---
phase: "28"
slug: "advanced-thresholds"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-11-23"
---

# Phase 28 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> Mode: Plan-authored threat register (`register_authored_at_plan_time: true`; `T-28-01` through `T-28-16` + `T-28-SC` verified against implementation and tests).

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| DB / Repository -> Redis Lua scripts | Per-group numeric thresholds flow from PostgreSQL/Redis cache into Lua ARGV windows, caps, and derived TTLs (`raid_counters.go`, `raid_flood.go`, `raid_ai_count.go`) | Integer bounds-validated windows, shares, floors, caps, TTLs |
| Replica A write -> Replica B hot-path read | `chat_advanced_thresholds:<chat>` writes on one replica must take effect on every replica on the next event (`skipLocal` + post-commit `DeleteCache`) | Cached `Thresholds` struct |
| Shared cross-chat singleflight (`n:<key>`) -> per-chat enforcement | Concurrent chats evaluating the same canonical user name share one Anthropic call; each caller's own `advMemberReqAIBar` decides its verdict | Raw `float64` probability |
| Cached verdict (`fuku:memberreq:nv:*`, `fuku:raid:aiv:*`) -> watcher / worker | Cached raw probabilities are shared across chats and re-evaluated against the requesting chat's current bar | Raw `float64` probability |
| `chat_advanced_thresholds` -> lockdown worker | Stored `lockdown_reminder_mins` governs both `lockdownPlanReminders` and `lockdownPostAlertSlot` supersession | `15..1440` min interval |
| `chat_advanced_thresholds` -> `sendCaptcha` / `captcha_attempts` | Option A extended timeout override (`11..60` min) governs `ExpiresAt`, Turnstile TTL, and challenge text without altering `captcha_settings.timeout` (1..10) or backups | Extended timeout minutes |
| Telegram callback query -> `settings_advthresholds.go` | Untrusted `set|v1|...` callbacks cross into settings dispatch (`aa`, `ao`, `as`, `ar`, `ax`), Step-1 validation, live `getChatMember` authorization, and DB writes | Callback fields, live admin/creator status |
| Private menu vs Group menu / Staff Group | Page `av` is private-only (`PrivateOnly: true`, `Hidden: settingsAdvHidden`); must never run from a group menu, appear in a Staff Group, pollute DB on reads, or leak Staff Group titles into UI or logs | Settings screens, toast answers, structured logs |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-28-01 | Tampering | `fuku/modules/advthresholds_phase28_legacy_request_test.go` | medium | mitigate | Committed alone in `48d5c240` before any production edits; exact 1 commit since `654c8311`; `TestLegacyPhase28DefaultThresholds` (8 subtests) passes untouched under `-race`. | closed |
| T-28-02 | Denial of Service / Tampering | `chat_advanced_thresholds` column bounds | high | mitigate | Dual enforcement: Go bounds checks in `fuku/db/advthresholds/repository.go:374-381` and `fuku/modules/advthresholds.go:161-168`, plus 12 inline named SQL CHECK constraints in `migrations/20261123120000_add_advanced_thresholds.sql:32-43` and `fuku/db/models/advthresholds.go:65-76`. | closed |
| T-28-03 | Tampering / Integrity | Cross-replica cache staleness on `chat_advanced_thresholds:<chat>` | medium | mitigate | `chatAdvancedThresholdsPrefix` registered in `fuku/db/cache/local.go:62,125` (`skipLocal`); post-commit `DeleteCache` on every write (`fuku/db/advthresholds/repository.go:416,471`); `TestSkipLocalChatAdvancedThresholdsKey`. | closed |
| T-28-04 | Denial of Service | Redis TTL / trim mismatch on raised raid windows | medium | mitigate | Derived TTLs (`trimWindow = max(joinWindow, recentWindow)`, `joinedTTL = max(raidJoinedTTL, trimWindow+1h)`, `floodTTL = max(raidFloodTTL, floodWindow*2)`, `aiTTL = max(raidAITTL, aiWindow*2)`) and all 5 `raidRecentWindow` sites wired (`TestAdvancedThresholdsRaidWiring`). | closed |
| T-28-05 | Denial of Service | Transient DB read error on raid hot path | medium | mitigate | `advThresholdsFor` (`fuku/modules/advthresholds.go:21,44-56`) bounds lookup to `2s`, logs IDs-only Warn, and fails to compiled defaults (`fail_to_default_on_lookup_error`). | closed |
| T-28-06 | Tampering / Elevation | `memberReqAIJudge` / `memberReqAIRule` shared `"n:<key>"` singleflight | high | mitigate | `memberReqAIJudge` returns raw `memberReqAIAnswer{Probability, HasProbability: true}`; `memberReqAIRule` applies each caller's own `advMemberReqAIBar` via `memberReqAIVerdictOfBar` (`memberreq_ai_bar_cross_chat_singleflight`). | closed |
| T-28-07 | Security Bypass | `memberReqWatch` fast-path vs lowered `member_req_ai_bar_pct` | medium | mitigate | `memberReqLookupCachedWith` (`fuku/modules/memberreq_watch.go:318-324`) resolves `advMemberReqAIBar` when `rules.AI` is true and passes it to `aiRead.satisfiedBar(aiBar)` (`memberreq_ai_watcher_fast_path_respects_lowered_bar`). | closed |
| T-28-08 | Denial of Service | `memberReqCapAllow` / `memberReqPauseChatWith` invalid cap/pause | medium | mitigate | Go + SQL CHECK bounds (`cap 3..100`, `pause 1..60m`), fail-to-default via `advThresholdsFor`, package-var fallback on `0` (`TestAdvancedThresholdsAIAndMemberReq`). | closed |
| T-28-09 | Denial of Service / Integrity | `lockdownPlanReminders` vs `lockdownPostAlertSlot` interval skew / divide-by-zero | high | mitigate | `lockdownReminderSeqFor` returns `0` when `every <= 0`; both planner (`lockdown_alert.go:438-439`) and delivery (`lockdown_alert.go:789-790`) call `lockdownReminderIntervalFor(ctx, ld.ChatID)` + `lockdownReminderSeqFor` (`15..1440` bounds). | closed |
| T-28-10 | Tampering / Denial of Service | `captcha_timeout_mins` bounds & preserving `captcha_settings.timeout` 1..10 / backups | medium | mitigate | Option A stores `0` or `11..60` in `chat_advanced_thresholds`; `effectiveCaptchaTimeout` overlays without altering `captcha_settings.timeout` 1..10 CHECK, `/captchatime`, or backups (`TestAdvThresholdsExcludedFromBackup`, `TestAdvancedThresholdsLockdownAndCaptcha`). | closed |
| T-28-11 | Tampering | EVT-10 boundary: lockdown reminders / captcha challenges outside event engine & PM copies | high | mitigate | `lockdownRenderReminder` and `sendCaptcha` remain outside `postEvent`/PM copies; verified by `TestEventEngineCallers` and `TestPMCopyCallers`. | closed |
| T-28-12 | Elevation of Privilege | `settings_advthresholds.go` write/reset authorization (`as`, `ar`, `ax`) | high | mitigate | Views (`aa`, `ao`) require live `settingsRightAdmin` + owner gate; all writes/resets (`as`, `ar`, `ax`) require live `settingsRightRestrict` (`TestSettingsAdvancedThresholds/live_authority_and_owner_gate`). | closed |
| T-28-13 | Tampering | Forged, out-of-range, unknown-field, or group-menu (`u`) callbacks targeting `aa`/`ao`/`as`/`ar`/`ax` | high | mitigate | Step-1 `private(...)` validators allow-list exact area/item codes and canonical curated presets, rejecting extra keys with 0 `getChatMember` calls and 0 DB reads (`TestPhase28Codes`, `forged_and_out_of_range_presses_refused_in_step1`). | closed |
| T-28-14 | Information Disclosure | Staff Group visibility or leakage of Staff Group identity on `av` | medium | mitigate | `Hidden: settingsAdvHidden` hides `av` in Staff Groups; `PrivateOnly: true` short-circuits group-menu reads; 0 `{staff}`/`{group}`/`{title}`/`{chat}` placeholders in `settings_adv_*` locales (`staff_group_hides_advanced_page`, `TestPhase28StaffPrivacyAndLogs`). | closed |
| T-28-15 | Denial of Service / Storage Pollution | Non-creating read invariant on `av`/`aa`/`ao`/Main | medium | mitigate | `GetFresh` returns `Thresholds{}, nil` on `gorm.ErrRecordNotFound` without inserting rows (`TestPhase28ReadsCreateNothing`, `TestAdvancedThresholdsCRUD`). | closed |
| T-28-16 | Information Disclosure | Staff Group title or sensitive content in Phase 28 UI or logs | medium | mitigate | Phase 28 logs emit IDs and field/area codes only; verified by `TestPhase28StaffPrivacyAndLogs` with Staff Group sentinel, group title, and moderator name across UI and captured logrus output. | closed |
| T-28-SC | Tampering | Go module dependencies (`go.mod` / `go.sum`) | low | accept | No third-party Go module dependency added or changed in Phase 28 (`git diff --exit-code 654c8311 -- go.mod go.sum` = 0). | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-28-01 | T-28-SC | No third-party Go module dependency was added or upgraded in Phase 28 (`git diff --exit-code 654c8311 -- go.mod go.sum` exited 0). | Phase 28 security audit | 2026-11-23 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-11-23 | 17 | 17 | 0 | gsd-security-auditor (plan-authored register, ASVS L1) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-11-23
