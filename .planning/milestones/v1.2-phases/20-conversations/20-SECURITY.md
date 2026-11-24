---
phase: "20"
slug: "conversations"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-11-22"
---

# Phase 20 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> Mode: Retroactive-STRIDE (`register_authored_at_plan_time: false`; register built from Phase 20 implementation files and verified against code and tests).

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| TB-1: Telegram Updates -> Bot Handlers | Untrusted group/private messages, mentions, captions, replies, and `/ask` / `/xeno` commands entering `fuku/modules/chat.go` and `chat_watch.go` | Message text/caption, entities, reply target, `Chat.Type`, `Chat.Title`, `Chat.Id`, `From.Id`, `From.FirstName`, `From.IsBot`, `SenderChat` |
| TB-2: Settings Callbacks -> Persistence | Unauthenticated Telegram callback queries entering `fuku/modules/settings_menu.go` and `settings_chat.go`, verified against live Telegram API before touching PostgreSQL (`chat_conversations`) and Redis (`chat_conv:<chat>`, `fuku:chat:{chat}:*`) | Callback fields (`a`, `c`, `p`, `t`, `v`, `e`), live `getChatMember` / `getChat` responses, enabled/provider/audience settings, creator `user_id` |
| TB-3: Bot Runtime -> External LLM Providers | Outbound HTTPS requests from `fuku/modules/chat_prompt.go` through `fuku/utils/claude/converse.go` and `fuku/utils/gemini/client.go` to Anthropic and Google Gemini | Static persona/archive (`xenopersona`), dynamic chat metadata (title, language, date, bot username), untrusted member first names and message/quote/transcript text, API keys in auth headers (`x-api-key`, `x-goog-api-key`) |
| TB-4: LLM Provider Responses -> Telegram & Redis | Untrusted model output parsed in `claude/converse.go` and `gemini/response.go`, filtered in `chat_send.go`, posted to Telegram group chats, and appended to Redis rolling transcript (`chat_history.go`) | JSON response bodies, generated text, stop/finish reasons, usage metadata, filtered plain-text replies, Telegram message IDs |
| TB-5: Operator Environment / CLI -> Runtime & Logs | Environment variables loaded in `fuku/config/config.go` (`ENABLE_CHAT`, `ANTHROPIC_API_KEY`, `GEMINI_API_KEY`, `CHAT_ANTHROPIC_MODEL`, `CHAT_GEMINI_MODEL`), `scripts/chat-spike/main.go` CLI flags, and structured `logrus` output | API keys, model IDs, feature flags, spike `-endpoint` / `-questions`, diagnostic logs and `/stats` counters |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-20-01 | Spoofing | `fuku/modules/chat.go`, `chat_watch.go` | high | mitigate | Private-chat/non-group refusal: `chat_watch.go:64` drops non-`group`/`supergroup`; `chat.go:49-55` replies `chat_private_refused` in private chats (`chat_watch_test.go:103-107, 405-409`). | closed |
| T-20-02 | Spoofing | `fuku/modules/chat.go`, `chat_watch.go`, `chat_history.go` | medium | mitigate | Bot/channel/anonymous-admin filtering and reply-loop prevention: `raidFloodSenderCounts` + `IsBot` checks at `chat_watch.go:70-73`, `chat.go:56-59`; replies only trigger on message IDs in `fuku:chat:{chat}:mine` (`chat_watch.go:130-138, 225-230`). | closed |
| T-20-03 | Spoofing | `fuku/utils/chatprompt/chatprompt.go` | medium | mitigate | Prompt speaker impersonation & header forgery: `Clean()` (`chatprompt.go:140-161`) strips invalid UTF-8, control chars, and Unicode `Cf` bidi/format chars; `Name()` (`chatprompt.go:126-135`) appends `" (member)"` if first name contains `"xenobot-72"`; `line()` renders single-line turns (`chatprompt_test.go:10-75`). | closed |
| T-20-04 | Tampering | `fuku/utils/claude/converse.go`, `fuku/utils/gemini/request.go`, `fuku/utils/xenopersona/persona.md`, `fuku/modules/chat_run.go` | high | mitigate | Prompt injection causing moderation/tool actions: neither Anthropic (`converse.go:115-122`) nor Gemini (`request.go:52-57`) exposes tools or function calling; runtime only sends filtered plain text (`chat_run.go:299-305`) with zero moderation code paths (`converse_test.go:106-110`, `gemini_test.go:126-130`). | closed |
| T-20-05 | Tampering | `fuku/modules/chat_send.go`, `fuku/utils/xenopersona/xenopersona_test.go` | high | mitigate | Malicious LLM output (links, `@mention` pings, markup, oversized replies): `chatFilterReply` (`chat_send.go:55-83`) replaces link tokens with `[link removed]`, inserts `\u200b` after `@`, truncates to 1200 runes; `chatSendReply` (`chat_send.go:90-105`) omits `ParseMode` and sets `LinkPreviewOptions.IsDisabled: true` (`chat_watch_test.go:36-38, 281-299`). | closed |
| T-20-06 | Tampering | `migrations/20261119120000_add_chat_conversations.sql`, `fuku/db/chatconv/repository.go`, `fuku/db/cache/local.go`, `fuku/config/config.go` | high | mitigate | Settings/DB/model tampering & backup-import activation bypass: SQL `CHECK` constraints, repo allowlist + parameterized GORM upserts (`repository.go:50-57, 96-135`), `chat_conv:` in `skipLocal` + `DeleteCache`, excluded from backup (`chatconv_backup_test.go`), strict `ValidModel` + `url.PathEscape` (`config.go:659-676`, `gemini/client.go:55-61, 131`). | closed |
| T-20-07 | Tampering | `fuku/utils/claude/converse.go`, `fuku/utils/gemini/client.go`, `scripts/chat-spike/main.go` | high | mitigate | HTTP redirect credential exfiltration & SSRF: `CheckRedirect` returns `http.ErrUseLastResponse` in `converse.go:69-74`, `gemini/client.go:87-92`, and `scripts/chat-spike/main.go:101-107`; production defaults to official HTTPS hosts; `chat-spike` `validEndpoint` restricts `-endpoint` to `https` or loopback without userinfo. | closed |
| T-20-08 | Tampering | `fuku/modules/chat_watch.go`, `chat_send.go`, `chat_run.go` | medium | mitigate | Answering messages deleted by synchronous moderation: watcher at handler group `13` (`chat_watch.go:32`), `AllowSendingWithoutReply: false` (`chat_send.go:96-99`), silent shed on refusal (`chat_run.go:306-310`, `TestChatDeletedMessageGetsNoReply`). Async `aispam` race accepted below (AR-20-01). | closed |
| T-20-09 | Repudiation | `fuku/db/chatconv/repository.go`, `fuku/modules/settings_chat.go`, `chat_run.go` | low | mitigate | Audit trail & observability: `updated_by` (creator `user_id`) and `updated_at` written on every upsert (`repository.go:118, 130`); atomic counters in `/stats` and structured warning logs on failures/breaker pauses (`chat_run.go:122-137, 329-343`). | closed |
| T-20-10 | Information Disclosure | `fuku/config/config.go`, `fuku/utils/claude/*`, `fuku/utils/gemini/*`, `fuku/modules/chat_prompt.go`, `scripts/chat-spike/main.go` | critical | mitigate | API key leakage: keys sent strictly in headers (`x-api-key`/`x-goog-api-key`), registered in `logredact` (`config.go:680-692`), invalid model logs rune count only, `*url.Error` stripped and scrubbed to `[REDACTED]` (`gemini/errors.go:39-48, 95-131`, `claude/errors.go:44-109`, `chat_logs_test.go:17-78`). | closed |
| T-20-11 | Information Disclosure | `fuku/modules/chat_run.go`, `chat_prompt.go`, `chat_send.go`, `chat_history.go`, `chat_settings.go`, `fuku/utils/chatprompt/chatprompt.go` | high | mitigate | PII & conversation leakage: logs emit IDs and error classes only (`chat_logs_test.go:17-78`); provider payloads omit user IDs, member usernames, and chat IDs (`chat_provider_test.go:76-87`); Redis caps at 12 lines/2h and 200 IDs/24h (`chat_history.go:24-33`) and clears immediately on Off (`chat_settings.go:144-146`). | closed |
| T-20-12 | Information Disclosure | `fuku/modules/settings_chat.go`, `settings_pages.go`, `settings_menu.go` | medium | mitigate | Group-menu settings exposure: `OwnerOnly: true`, `PrivateOnly: true` (`settings_chat.go:50-51`); non-owners never see the row; group menus show generic "In private" without DB read and refuse all group presses (`settings_chat_test.go:156-201`). | closed |
| T-20-13 | Denial of Service | `fuku/modules/chat_run.go`, `chat_send.go`, `chat_watch.go`, `fuku/utils/claude/converse.go`, `fuku/utils/gemini/client.go` | high | mitigate | Billing/worker/rate-limit exhaustion: bounded pool (64 queue, 3 workers, silent shed), 5s per-user cooldown, 3-tier hourly Redis budgets (`12/user`, `80/chat`, `800/global`, fail-closed), per-provider circuit breakers, isolated `chatPacer`, gated canned replies, 1MiB response cap, 40s timeout, max 2s single retry, `StopChat()` drain, zero I/O on unaddressed messages, fail-closed without Redis. | closed |
| T-20-14 | Elevation of Privilege | `fuku/modules/settings_chat.go`, `settings_menu.go` | critical | mitigate | Non-owner/group-menu activation bypass: `OwnerOnly: true`, `PrivateOnly: true`, all controls (`e`, `m`, `w`) and confirm action (`py`) require `settingsRightOwner` checked via live `getChatMember` (`ChatMemberStatusCreator`, never cached) on every press; confirm card validates expiry, `p.private`, and provider config (`settings_chat_test.go:176-354`). | closed |
| T-20-15 | Elevation of Privilege | `fuku/modules/chat.go`, `chat_watch.go` | high | mitigate | Audience restriction bypass: both watcher and `/ask`/`/xeno` funnel through `chatAccept` -> `chatAudienceAllows` (`chat_watch.go:170-183`), failing closed on settings, admin, or approvals lookup errors (`chat_watch_test.go:161-184`). | closed |
| T-20-16 | Elevation of Privilege | `fuku/config/config.go`, `fuku/modules/chat_settings.go`, `chat_watch.go`, `chat_run.go`, `settings_chat.go` | high | mitigate | Kill-switch (`ENABLE_CHAT`) bypass: `EnableChat` defaults to `false` (`config.go:512`), enforced via `chatProviderConfigured` at runtime and settings activation/provider writes while permitting Off (`config_pure_test.go:564-574`, `chat_watch_test.go:268-277`, `settings_chat_test.go:284-354`). | closed |
| T-20-17 | Elevation of Privilege | `fuku/modules/chat.go` | low | mitigate | Disabled-command bypass via `/xeno`: `/xeno` checks both `CheckDisabled("xeno")` and `CheckDisabled("ask")` (`chat.go:31-35`, `chat_watch_test.go:411-420`). | closed |
| T-20-18 | Information Disclosure | `fuku/modules/settings_chat.go`, `chat_prompt.go` | medium | transfer | Third-party LLM processing of addressed group messages (Anthropic/Gemini): transferred to operator & group creator via default-off `ENABLE_CHAT`, creator-only private opt-in, and mandatory 2-tap confirm card displaying `settings_chat_data_notice` naming the selected provider (`settings_chat.go:109-111, 174-179, 256-257`). | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| TR-20-01 | T-20-18 | Addressed member messages, quoted messages, up to 12 recent turns, first names, and group title/language are sent to Anthropic or Google Gemini only after the operator sets `ENABLE_CHAT=true` and the group creator confirms the data notice card in private chat; no user IDs, member usernames, or chat IDs leave the bot. | Owner (phase design / `20-SPIKE.md`) | 2026-11-22 |
| AR-20-01 | T-20-08 (residual) | `aispam` judges asynchronously, so a spam message mentioning the bot can still receive a reply if `aispam` deletes it only after the reply is posted. Synchronous deletion modules (blacklist, locks, antiflood) are mitigated via handler group 13 and `AllowSendingWithoutReply: false` (`20-06-SUMMARY.md`, `20-SPIKE.md`). | Owner (`20-06-SUMMARY.md` / `20-SPIKE.md`) | 2026-11-22 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-11-22 | 18 | 18 | 0 | gsd-security-auditor (retroactive-STRIDE, ASVS L1) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-11-22
