---
phase: 20-conversations
verified: 2026-11-22T19:14:00Z
status: passed
score: 8/8 must-haves verified
covered_files:
  - .planning/phases/20-conversations/20-01-PLAN.md
  - .planning/phases/20-conversations/20-01-SUMMARY.md
  - .planning/phases/20-conversations/20-02-SUMMARY.md
  - .planning/phases/20-conversations/20-03-SUMMARY.md
  - .planning/phases/20-conversations/20-04-SUMMARY.md
  - .planning/phases/20-conversations/20-05-SUMMARY.md
  - .planning/phases/20-conversations/20-06-SUMMARY.md
  - .planning/phases/20-conversations/20-07-SUMMARY.md
  - .planning/phases/20-conversations/20-08-SUMMARY.md
  - AGENTS.md
  - Makefile
  - docs/src/content/docs/api-reference/environment.md
  - docs/src/content/docs/architecture/handler-groups.md
  - docs/src/content/docs/commands/chat/index.md
  - fuku/config/config.go
  - fuku/config/config_pure_test.go
  - fuku/db/backup/chatconv_backup_test.go
  - fuku/db/backup/testmain_test.go
  - fuku/db/cache/local.go
  - fuku/db/cache/local_test.go
  - fuku/db/chatconv/repository.go
  - fuku/db/chatconv/repository_test.go
  - fuku/db/chatconv/testmain_test.go
  - fuku/db/migrations/chat_conversation_migration_test.go
  - fuku/db/models/chat_conversation.go
  - fuku/i18n/settings_locale_test.go
  - fuku/modules/chat.go
  - fuku/modules/chat_e2e_test.go
  - fuku/modules/chat_env_test.go
  - fuku/modules/chat_group_test.go
  - fuku/modules/chat_history.go
  - fuku/modules/chat_logs_test.go
  - fuku/modules/chat_prompt.go
  - fuku/modules/chat_provider_test.go
  - fuku/modules/chat_run.go
  - fuku/modules/chat_send.go
  - fuku/modules/chat_settings.go
  - fuku/modules/chat_settings_test.go
  - fuku/modules/chat_watch.go
  - fuku/modules/chat_watch_test.go
  - fuku/modules/devs.go
  - fuku/modules/help_system.go
  - fuku/modules/settings_chat.go
  - fuku/modules/settings_chat_test.go
  - fuku/modules/settings_fwdblock_test.go
  - fuku/modules/settings_main_test.go
  - fuku/modules/settings_phase18_test.go
  - fuku/modules/settings_phase19_test.go
  - fuku/modules/settings_staff_privacy_test.go
  - fuku/modules/test_harness_test.go
  - fuku/utils/chatprompt/chatprompt.go
  - fuku/utils/chatprompt/chatprompt_test.go
  - fuku/utils/chatprompt/leaf_test.go
  - fuku/utils/claude/client.go
  - fuku/utils/claude/converse.go
  - fuku/utils/claude/converse_test.go
  - fuku/utils/claude/names.go
  - fuku/utils/gemini/client.go
  - fuku/utils/gemini/errors.go
  - fuku/utils/gemini/gemini_test.go
  - fuku/utils/gemini/leaf_test.go
  - fuku/utils/gemini/request.go
  - fuku/utils/gemini/response.go
  - fuku/utils/xenopersona/archive.md
  - fuku/utils/xenopersona/leaf_test.go
  - fuku/utils/xenopersona/persona.md
  - fuku/utils/xenopersona/xenopersona.go
  - fuku/utils/xenopersona/xenopersona_test.go
  - locales/en.yml
  - locales/es.yml
  - locales/fr.yml
  - locales/hi.yml
  - locales/id.yml
  - locales/pt.yml
  - locales/ru.yml
  - locales/uk.yml
  - main.go
  - migrations/20261119120000_add_chat_conversations.sql
  - sample.env
  - scripts/chat-spike/main.go
  - scripts/chat-spike/main_test.go
  - scripts/chat-spike/persona-check.txt
  - scripts/chat-spike/persona_check_test.go
covered_digest: "v4:sha256:5ea96619110f3284108383ea077daed2430a415a82f1f53c2ab54a02883e56a2"
behavior_unverified: 0
overrides_applied: 0
---

# Phase 20: Conversations Verification Report

**Phase Goal:** Conversations (shipped in v2.26.0) — creator-only, private-chat-configured AI conversations in groups via mentions, replies, `/ask` and `/xeno`, with Anthropic and Gemini providers, static xenopersona archive, output filtering, rate limits, circuit breakers, and zero private-chat conversing (per `20-CONTEXT.md` and `.planning/research/xenobot-chat/PLAN.md`).
**Verified:** 2026-11-22T19:14:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
| --- | --- | --- | --- |
| 1 | `fuku/utils/xenopersona` is a stdlib-only leaf embedding `persona.md` and `archive.md` (leading source comment stripped), returning identical bytes on every `StaticSystemText()` call within 6,000–12,000 tokens, with exact canon numbers, Contradictions and Not on record sections, no placeholders/dates/old project names (`Fuku`, `Alita`, `DivideProjects`) and no web addresses (`20-01-PLAN.md`, PLAN.md 7.1–7.3). | ✓ VERIFIED | Inspected `fuku/utils/xenopersona/xenopersona.go`, `persona.md`, `archive.md`, `xenopersona_test.go`. All 7 tests in `./fuku/utils/xenopersona` pass (`~11,528` tokens: persona 1,887, archive 9,707; leaf check passes). Note: owner tuned `persona.md` directly in commit `b450df50` after UAT; all tests remain green. |
| 2 | Anthropic (`fuku/utils/claude/converse.go`) and Google Gemini (`fuku/utils/gemini`) clients implement conversation requests with static cached prefix first, dynamic context second, `effort`/`thinkingLevel` low, no tools/sampling/prefill, 40 s timeout, single retry ≤ 2 s, 1 MiB response cap, no redirect following, header-only auth keys, and key/URL-scrubbed errors (`20-02`, PLAN.md 6.2–6.4). | ✓ VERIFIED | Inspected `fuku/utils/claude/converse.go` and `fuku/utils/gemini/{client,request,response,errors}.go`. `ServerFallback` excludes Haiku. All tests in `./fuku/utils/claude` and `./fuku/utils/gemini` pass (including leaf tests, request shape, stop/finish reasons, refusals, retry, redirect refusal, and key redaction). |
| 3 | `fuku/utils/chatprompt` is a leaf builder shared by the bot and `scripts/chat-spike`, flattening history/quoted/message turns into untrusted user sections, sanitizing control/bidi chars, defusing `Xenobot-72` member names, stripping bot mentions, and omitting user IDs/usernames/chat IDs; `scripts/chat-spike` refuses non-HTTPS remote endpoints and stays in sync with `20-PERSONA-CHECK.md` (`20-03`, PLAN.md 7.4–7.5). | ✓ VERIFIED | Inspected `fuku/utils/chatprompt/chatprompt.go` and `scripts/chat-spike/main.go`. All tests in `./fuku/utils/chatprompt` (6 tests, including `TestChatpromptPackageIsLeaf`) and `./scripts/chat-spike` (`TestRunAnthropic`, `TestRunGoogle`, `TestRunRefusals`, `TestRunAllFailed`, `TestPersonaCheckFilesAgree`) pass. |
| 4 | Per-group storage (`migrations/20261119120000_add_chat_conversations.sql`, `fuku/db/models/chat_conversation.go`, `fuku/db/chatconv/repository.go`) has inline `CHECK` constraints, no `default:` tag on bool, `chat_conv:` in `skipLocal` (`fuku/db/cache/local.go`), column-targeted upserts calling `cache.DeleteCache`, backup exclusion, and apply functions (`fuku/modules/chat_settings.go`) that wipe Redis transcript and own-message IDs on Off (`20-04`, PLAN.md 4.2). | ✓ VERIFIED | Inspected migration SQL, model, repository, `local.go`, and `chat_settings.go`. Behavioral tests `TestChatConversationSettings` (`./fuku/db/chatconv`), `TestApplyChatSettings` (`./fuku/modules`, `-race`), and `TestChatConversationsStayOutOfBackup` (`./fuku/db/backup`) all pass. |
| 5 | Server config (`fuku/config/config.go`) defaults `ENABLE_CHAT=false`, `CHAT_ANTHROPIC_MODEL=claude-haiku-5-5`, `CHAT_GEMINI_MODEL=gemini-4-hydrogen`, registers `GEMINI_API_KEY` in `logredact`; settings page `p` (`fuku/modules/settings_chat.go`) is `OwnerOnly: true`, `PrivateOnly: true`, enforces live creator check (`settingsRightOwner`) on all controls (`e`, `m`, `w`) and confirm action (`py`), uses a 2-tap data-notice confirm card for On, one-tap Off, and refuses group-menu presses and unconfigured providers (`20-05`, PLAN.md 4.1, 4.3, 6.1). | ✓ VERIFIED | Inspected `fuku/config/config.go` and `fuku/modules/settings_chat.go`. `TestChatConfig` (`./fuku/config`), `TestSettingsChatOwnerFlow`, `TestSettingsChatMainAndGroupMenu`, `TestSettingsChatAuthority`, `TestSettingsChatNotConfigured`, `TestSettingsChatCallbackBytes`, `TestSettingsChatPageFitsEveryLocale`, and `TestSettingsStaffPrivacy` all pass. |
| 6 | Runtime (`chat.go`, `chat_watch.go`, `chat_run.go`, `chat_history.go`, `chat_prompt.go`, `chat_send.go`) registers module `Chat` (priority 245) and `chatWatcher` alone at handler group 13 (always `ext.ContinueGroups`, panic recovery, zero I/O on unaddressed/private messages); never converses in private chats; ignores bots/channels/anonymous admins/moderation replies; enforces audience, 64-queue/3-worker pool, 5 s cooldown, 12/80/800 hourly Redis budgets, per-provider breakers, output filtering (`[link removed]`, `@\u200b`, 1200-rune cap, plain text, `AllowSendingWithoutReply: false`), isolated `chatPacer`, gated canned replies, IDs-only logs, `/stats` counters, and `StopChat` drain in `main.go` (`20-06`, PLAN.md 3.1–3.5, 5.1–5.6). | ✓ VERIFIED | Inspected all runtime files, `main.go:267-271`, and `fuku/modules/devs.go:379`. Behavioral tests `TestChatWatcherGroupReachedByMention`, `TestChatWatcherAnswers`, `TestChatWatcherIgnores`, `TestChatWatcherAudience`, `TestChatGuards`, `TestChatFilterReply`, `TestChatDeletedMessageGetsNoReply`, `TestShutdownEndsChat`, `TestChatLogsCarryIDsOnly`, and `TestChatProvidersRequest` all pass. |
| 7 | `/ask` and `/xeno` are registered via `helpers.WrapCommand` as disableable (`/xeno` checks both `xeno` and `ask`), refuse private chats (`chat_private_refused`), show `chat_off_hint` when off and `chat_ask_usage` when bare; `/help` includes `Chat` with icon `🗨`; all locale keys exist in all 8 locales and pass style checks; docs (`commands/chat/`, `environment.md`, `handler-groups.md`, `sample.env`, `AGENTS.md`) have zero drift (`20-07`, PLAN.md 3.1, 3.5, 8). | ✓ VERIFIED | Inspected `fuku/modules/chat.go`, `help_system.go:155`, `locales/en.yml`, `locales/uk.yml`, `docs/`, `sample.env`, and `AGENTS.md`. `TestChatAskCommand`, `go test -tags testtools -race ./fuku/i18n/...`, `make check-translations`, `make check-docs`, and `make test-shard-check` all pass. |
| 8 | End-to-end flow (owner private enablement -> mention -> Anthropic adapter reply -> follow-up reply carrying history), live owner UAT, and retroactive STRIDE security audit all pass (`20-08`, PLAN.md 10, 13). | ✓ VERIFIED | `TestChatEndToEnd` passes against real dispatcher and `httptest` provider; `20-UAT.md` is `status: complete` with 8/8 passed and 0 issues; `20-SECURITY.md` is `status: verified` with 18/18 threats closed (`threats_open: 0`). |

**Score:** 8/8 truths verified (0 present, behavior-unverified)

### Required Artifacts

*(Note: `gsd-tools query verify.artifacts` returned exit 66 `NO_INPUT` because `20-01-PLAN.md` declares `must_haves.truths` only. All deliverables across Plans 20-01..20-08 were verified manually at Levels 1–4.)*

| Artifact | Expected | Status | Details |
| --- | --- | --- | --- |
| `fuku/utils/xenopersona/{xenopersona.go,persona.md,archive.md}` | Stdlib leaf embedding static persona + archive | ✓ VERIFIED | Exists, substantive (~11,528 tokens), wired into `chatprompt.Static()`, leaf + content tests pass. |
| `fuku/utils/claude/converse.go` | Anthropic conversation client | ✓ VERIFIED | Exists (236 lines), substantive, wired into `chatAnthropicProvider`, `httptest` tests pass. |
| `fuku/utils/gemini/{client,request,response,errors}.go` | Gemini generateContent leaf client | ✓ VERIFIED | Exists, substantive, wired into `chatGeminiProvider` and `scripts/chat-spike`, leaf + `httptest` tests pass. |
| `fuku/utils/chatprompt/chatprompt.go` | Leaf prompt builder shared by bot and spike | ✓ VERIFIED | Exists (207 lines), substantive, wired into `chat_prompt.go` and `scripts/chat-spike/main.go`, tests pass. |
| `scripts/chat-spike/{main.go,persona-check.txt}` | Owner persona check CLI + 43 questions | ✓ VERIFIED | Exists (321 lines), substantive, wired to `chatprompt`/`claude`/`gemini`, synced with `20-PERSONA-CHECK.md`. |
| `migrations/20261119120000_add_chat_conversations.sql` | Idempotent `chat_conversations` DDL | ✓ VERIFIED | Exists (23 lines), inline `CHECK` constraints, no FK. |
| `fuku/db/models/chat_conversation.go` & `fuku/db/chatconv/repository.go` | GORM model + cached repo (`skipLocal`) | ✓ VERIFIED | Exists, no bool `default:`, `chat_conv:` in `fuku/db/cache/local.go:53`, `cache.DeleteCache` on writes, excluded from backup. |
| `fuku/modules/chat_settings.go` & `fuku/modules/settings_chat.go` | Apply functions + owner-only private page `p` | ✓ VERIFIED | Exists, wired via `registerSettingsPage` (`OwnerOnly: true`, `PrivateOnly: true`), live `settingsRightOwner`, wipes Redis on Off. |
| `fuku/modules/{chat,chat_watch,chat_run,chat_history,chat_prompt,chat_send}.go` | Chat module, group 13 watcher, `/ask` & `/xeno`, worker pool, guards, filter | ✓ VERIFIED | Exists, registered at priority 245, watcher at group 13, `StopChat` wired in `main.go:269`, stats wired in `devs.go:379`. |
| `fuku/config/config.go`, `locales/*.yml`, `docs/src/content/docs/commands/chat/index.md` | Config defaults, `logredact`, 8 locales, generated docs | ✓ VERIFIED | `ENABLE_CHAT` default false, `GEMINI_API_KEY` redacted, all 8 locales complete, zero docs drift. |

### Key Link Verification

*(Note: `gsd-tools query verify.key-links` returned exit 66 `NO_INPUT` because `20-01-PLAN.md` declares no `must_haves.key_links`. Critical links were verified directly in source and tests.)*

| From | To | Via | Status | Details |
| --- | --- | --- | --- | --- |
| `fuku/modules/settings_chat.go` | `fuku/modules/chat_settings.go` -> `fuku/db/chatconv/repository.go` | `applyChatEnabled`/`applyChatProvider`/`applyChatAudience` -> `SetEnabled`/`SetProvider`/`SetAudience` + `cache.DeleteCache` | ✓ WIRED | Verified in `settings_chat.go:242,284,308,328`, `chat_settings.go:139-172`, `repository.go:89-136`, and `TestSettingsChatOwnerFlow`. |
| `fuku/modules/chat_settings.go` (Off) | Redis `fuku:chat:{chat}:h`, `:mine` | `chatWipeMemory(chatID)` -> `client.Del(...)` | ✓ WIRED | Verified in `chat_settings.go:144-146,186-196` and `TestApplyChatSettings`. |
| `fuku/modules/chat.go` (`/ask`, `/xeno`) & `chat_watch.go` (group 13) | `fuku/modules/chat_run.go` | `chatAccept` -> `chatEnqueue` -> `chatWorker` -> `chatProcessJob` | ✓ WIRED | Verified in `chat.go:60-75`, `chat_watch.go:77-81`, `chat_run.go:166-324`, `TestChatWatcherAnswers`, `TestChatAskCommand`. |
| `fuku/modules/chat_run.go` | `fuku/modules/chat_prompt.go` -> `claude.Client.Converse` / `gemini.Client.Generate` | `chatBuildRequest` (`chatprompt`) -> `chatProviderOf(job.provider).converse` | ✓ WIRED | Verified in `chat_run.go:266-268`, `chat_prompt.go:69-105,186-196`, `TestChatProvidersRequest`, `TestChatEndToEnd`. |
| `fuku/modules/chat_run.go` | `fuku/modules/chat_send.go` & `chat_history.go` | `chatFilterReply` -> `chatSendReply` (`chatPacer`, `AllowSendingWithoutReply: false`) -> `chatRecordExchange` | ✓ WIRED | Verified in `chat_run.go:299-322`, `chat_send.go:64-105`, `chat_history.go:71-95`, `TestChatEndToEnd`. |
| `main.go` | `fuku/modules/chat_run.go` | `shutdownManager.RegisterHandler` -> `modules.StopChat()` | ✓ WIRED | Verified in `main.go:267-271` and `TestShutdownEndsChat`. |
| `fuku/modules/devs.go` (`/stats`) | `fuku/modules/chat_run.go` | `chatStatsSection()` | ✓ WIRED | Verified in `fuku/modules/devs.go:379` and `chat_run.go:132-137`. |
| `fuku/db/cache/local.go` | `fuku/db/chatconv/repository.go` | `chatConvPrefix = CacheKey("chat_conv") + ":"` in `skipLocal` | ✓ WIRED | Verified in `fuku/db/cache/local.go:53,90-92`. |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| --- | --- | --- | --- | --- |
| `fuku/modules/settings_chat.go` | `stored` (`chatconv.Settings`) | `settingsChatReadSettings` -> `chatconv.GetFresh` -> PostgreSQL `chat_conversations` | Yes | ✓ FLOWING |
| `fuku/modules/chat_watch.go` | `settings` (`chatconv.Settings`) | `chatSettingsLookup` -> `chatconv.Get` (Redis-cached, `skipLocal`) -> PostgreSQL | Yes | ✓ FLOWING |
| `fuku/modules/chat_run.go` | `history` (`[]chatprompt.Turn`) | `chatReadHistory` -> Redis `LRange` on `fuku:chat:{chat}:h` | Yes | ✓ FLOWING |
| `fuku/modules/chat_run.go` | `answer.Text` | `chatProviderOf(job.provider).converse` -> live Anthropic/Gemini HTTPS API -> `chatFilterReply` -> Telegram `SendMessageWithContext` | Yes | ✓ FLOWING |
| `fuku/modules/devs.go` | `/stats` Conversations line | `chatStatsSection()` -> atomic counters (`chatAnsweredCount`, `chatRefusedCount`, `chatFailedCount`, `chatShedCount`, `chatCappedCount`) | Yes | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| --- | --- | --- | --- |
| Leaf packages, DB repo, and spike tool tests | `go test -tags testtools -v ./fuku/utils/xenopersona ./fuku/utils/claude ./fuku/utils/gemini ./fuku/utils/chatprompt ./fuku/db/chatconv ./scripts/chat-spike` | All 6 packages PASS (0 failures) | ✓ PASS |
| Phase 20 runtime, settings page, E2E, guards, logs, shutdown, privacy tests | `go test -tags testtools -v -count=1 -run '^(TestChatEndToEnd\|TestChatWatcher\|TestChatLogs\|TestChatProviders\|TestChatGuards\|TestChatFilterReply\|TestShutdownEndsChat\|TestChatDeletedMessageGetsNoReply\|TestChatAskCommand\|TestChatSettings\|TestSettingsChat\|TestChatConversations\|TestSettingsStaffPrivacy\|TestSettingsMain)' ./fuku/modules` | All tests PASS (1.700s) | ✓ PASS |
| Apply functions & Redis wipe (`-race`) + i18n/locale style tests (`-race`) | `go test -tags testtools -race -count=1 -v -run '^(TestApplyChatSettings)$' ./fuku/modules && go test -tags testtools -race -count=1 ./fuku/i18n/...` | Both PASS (`modules` 1.801s, `i18n` 28.504s) | ✓ PASS |
| Config defaults/redaction + backup exclusion | `go test -tags testtools -v -count=1 -run 'TestChat\|TestGemini\|TestEnableChat\|TestChatConfig' ./fuku/config ./fuku/db/backup` | `TestChatConfig` PASS, `TestChatConversationsStayOutOfBackup` PASS | ✓ PASS |
| Translations, docs drift, and test-shard alignment | `make check-translations && make check-docs && make test-shard-check` | All 8 locales complete (1066 keys), 0 docs drift, shard diff empty | ✓ PASS |

*(Note: Running an unfiltered `-run 'TestChat'` in `./fuku/modules` on macOS also matches Phase 18's unrelated `TestChatClockReadsTheGroup` in `chat_clock_test.go`, which fails on macOS's case-insensitive `/usr/share/zoneinfo` filesystem; `chat_clock.go` is Phase 18's timezone module and outside Phase 20.)*

### Probe Execution

No `scripts/*/tests/probe-*.sh` probes are declared for Phase 20 (live provider checks use `scripts/chat-spike` and were completed by the owner in `20-UAT.md` Test #8).

### Requirements Coverage

Phase 20 uses `.planning/research/xenobot-chat/PLAN.md` and `20-01-PLAN.md` (`requirements: [PLAN.md 7.1, 7.2, 7.3]`) rather than `REQUIREMENTS.md` IDs (which map Phases 21–30). No orphaned `REQUIREMENTS.md` entries map to Phase 20.

| Requirement | Source Plan | Description | Status | Evidence |
| --- | --- | --- | --- | --- |
| `PLAN.md 7.1` | `20-01-PLAN.md` | Leaf `fuku/utils/xenopersona` with embedded `persona.md` + `archive.md`, stable bytes, 6k–12k tokens, no placeholders/old names | ✓ SATISFIED | `xenopersona.go`, all 7 tests in `./fuku/utils/xenopersona` PASS |
| `PLAN.md 7.2` | `20-01-PLAN.md` | `persona.md` prose covering identity, voice, format, Ukrainian rules/glossary, two question kinds, limits, untrusted input | ✓ SATISFIED | `persona.md` verified + owner refinement `b450df50` + UAT Test #8 PASS |
| `PLAN.md 7.3` | `20-01-PLAN.md` | `archive.md` distilled from `LORE.md` with exact numbers, Contradictions, Not on record, ten canon rules, exclusions | ✓ SATISFIED | `archive.md`, `TestStaticSystemTextKeepsTheCanonRules`, `TestArchiveWritesNoAddress` PASS |
| `20-CONTEXT.md` Hard Rule 1 | `20-06`, `20-07` | Bot never converses in private chats | ✓ SATISFIED | `chat_watch.go:64`, `chat.go:49-55`, `TestChatWatcherIgnores`, `TestChatAskCommand`, UAT Test #5 PASS |
| `20-CONTEXT.md` Hard Rule 2 | `20-05`, `20-08` | Only group's live owner can enable conversations, from private chat | ✓ SATISFIED | `settings_chat.go:50-51,55-76,184-190`, `TestSettingsChatAuthority`, `TestSettingsChatMainAndGroupMenu`, UAT Tests #2–3 PASS |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| --- | --- | --- | --- | --- |
| *(none)* | — | Scanned all Phase 20 implementation files for `TBD`, `FIXME`, `XXX`, `TODO`, `HACK`, `PLACEHOLDER`, and empty stubs | — | 0 anti-patterns or unresolved debt markers found |

Informational note:
- Commit `b450df50` (`Update persona.md`, authored by the owner) tuned `fuku/utils/xenopersona/persona.md` after initial UAT (adding Ukrainian transliteration rules and tightening wording). All automated `xenopersona` tests pass and owner UAT (`20-UAT.md`) is 8/8 passed.

### Human Verification Required

None. Owner live UAT is already complete (`20-UAT.md`: `status: complete`, 8/8 passed, 0 issues) and retroactive STRIDE ASVS L1 security verification is already complete (`20-SECURITY.md`: `status: verified`, 18/18 threats closed, `threats_open: 0`).

### Gaps Summary

No gaps found. All 8 Phase 20 must-haves are implemented, wired, behaviorally tested, UAT-verified, and security-audited.

---

_Verified: 2026-11-22T19:14:00Z_
_Verifier: the agent (gsd-verifier)_
