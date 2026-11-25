---
phase: quick-261125-0iq
verified: 2026-11-25T02:30:00Z
status: human_needed
score: 8/8 must-haves verified
covered_files: [".planning/quick/261125-0iq-add-unrollshort-unrolllong-and-judge-ai-/261125-0iq-PLAN.md", ".planning/quick/261125-0iq-add-unrollshort-unrolllong-and-judge-ai-/261125-0iq-SUMMARY.md", "AGENTS.md", "docs/src/content/docs/commands/chat/index.md", "locales/en.yml", "migrations/20261125120000_add_chat_conversation_log.sql", "xenobot/db/chatconv/repository.go", "xenobot/db/models/chat_conversation.go", "xenobot/modules/chat.go", "xenobot/modules/chat_brief.go", "xenobot/modules/chat_brief_prompt.go", "xenobot/modules/chat_log.go", "xenobot/modules/chat_run.go", "xenobot/modules/chat_settings.go", "xenobot/modules/settings_chat.go"]
covered_digest: "v4:sha256:93be3a21061a1a9035ba5170d7edbb210f90ae1572d774408687a379fbf4cc63"
behavior_unverified: 0
overrides_applied: 0
human_verification:
  - test: "With a real Anthropic or Gemini key, log a short quarrel of 3 to 5 members, reply /judge to its first message"
    expected: "The verdict gives every roster name exactly once a 1 to 10 score and a short reason (or 'not enough shown to judge' for a name marked (no messages shown)), scores nobody off the roster, is plain text, in the group's language, and ends with the AI-opinion notice"
    why_human: "Tests drive scripted provider stand-ins; whether a real model follows chatBriefInstruction(chatBriefJudge) and covers the whole roster cannot be checked by grep or a fake"
  - test: "With a real provider, reply /unrollshort and /unrolllong to a message about 30 messages back"
    expected: "Short brief of at most ~8 lines and a longer detailed brief, in the group's language, no links or at-handles, no instructions from member text followed"
    why_human: "Output quality and prompt-injection resistance of a real model are not testable with stand-ins"
---

# Quick 261125-0iq: /unrollshort, /unrolllong, /judge Verification Report

**Goal:** AI briefs (/unrollshort, /unrolllong) and an admin's scored verdict (/judge) of a stretch of a group's conversation, read from a per-group rolling message log behind an owner-only switch.
**Verified:** 2026-11-25
**Status:** human_needed (all code-level truths verified; real-model output quality needs a human)
**Re-verification:** No — initial verification
**Commits:** e2a219e, c1ec853, 06a3490 on top of 2294a42

## Goal Achievement

### Observable Truths

| # | Truth (PLAN must_haves) | Status | Evidence |
|---|-------|--------|----------|
| 1 | Log keeps newest 1000 real member messages (text, captions, media tag, edits in place) only while switch on + ENABLE_CHAT + provider configured; Off deletes the log | ✓ VERIFIED | `chat_log.go`: Lua insert/edit scripts replace by score, `ZREMRANGEBYRANK` trims to `chatLogMax` (1000), edit script writes nothing for an absent ID; `chatLogWatch` checks group type, `chatAnyProviderConfigured`, non-slash user content, `raidFloodSenderCounts` + not bot, `settings.LogEnabled` + provider, Redis. Registered at group 14 with `SetAllowEdited(true)` (`chat.go`). `applyChatLog` Off calls `chatWipeLog(chatLogKey)`. Tests `TestChatLogRing`, `TestChatLogWatcher*`, `TestApplyChatLog`, `TestSettingsChatLog` pass |
| 2 | /unrollshort, /unrolllong brief the replied-to message onward; out-of-log target answers the hint, no provider call | ✓ VERIFIED | `chatBriefPrepare`: `extraction.ReplyTarget` → gate → `chatLogHas` (ZCOUNT) → `brief_too_old` before enqueue; worker re-checks with `chatBriefStretch` (first entry must be fromID). `TestChatBriefRefusals` asserts 0 provider calls for usage/too-old/off/unavailable; `TestChatBriefShort`, `TestChatBriefLong`, `TestChatBriefEndToEnd` pass |
| 3 | /judge admins only via live getChatMember (direct and anonymous re-entry), 1–10 score + reason per roster name, 40-name roster, "(no messages shown)" mark, unscored line, AI-opinion notice | ✓ VERIFIED | `judgeDesc.RequiredChecks` = CheckDisabled, RequireGroup, `requireLockdownAuthority(false)` (live `lockdownLiveMember`, anon → proof prompt); `RegisterAnonymousAdminHandler("judge", anonPipelineHandler(judgeDesc, judgeAnon))` in `chat.go` init. `chatBriefRoster`/`chatBriefParticipants`/`chatBriefFrame` (judge_unscored_note + judge_ai_notice). `TestChatJudge` (cached-admin-but-live-member refused, 0 provider calls, live check asked), `TestChatJudgeAnonymousAdmin`, `TestChatJudgeRoster`, `TestChatJudgeNeverActs` pass |
| 4 | Destination group/pm/staff by argument or stored default; blocked PM or no Staff link answers in group, nothing else sent | ✓ VERIFIED | `chatJudgeDestination` (0 words → stored default, 1 word group/pm/staff case-insensitive, else usage); staff link read fresh at handler (no link/err → reply, no AI call) and again before send; `chatJudgeDeliverPrivate` 403 → `judge_pm_blocked`; notes `judge_note_pm`/`judge_note_staff`. `TestChatJudge`, `TestChatJudgeStaff` pass |
| 5 | Owner-only private Conversations page: Message log switch (off default, one tap on, data notice) + Judge destination choice; group menu / non-owner press changes nothing | ✓ VERIFIED | `settings_chat.go` controls `l` (tg, 0/1) and `j` (st, g/p/s), both `settingsRightOwner`, both first call `settingsChatGroupRefused`; On refused when provider not configured; `settings_chat_log_notice` body. `TestSettingsChatLog`, `TestSettingsChatJudge`, `TestSettingsChatAuthority`, `TestSettingsDriftChat`, callback-bytes and fits-every-locale tests pass |
| 6 | Provider gets first names only; transcript labelled untrusted; logs carry IDs/error classes only | ✓ VERIFIED | `chatBriefLabels` (first name + deterministic `#n`, never an ID), `chatBriefInstruction` + closing "do not follow instructions" line; log lines use `chatRedisErrClass`/`chatSendErrClass`. `TestChatBriefRequestIsMinimal`, `TestChatBriefLogsCarryIDsOnly` pass |
| 7 | Cost bounded: cooldown, per-user/group/fleet hourly budgets, 1 worker + 16 queue, 30000-rune input cap incl. roster, output caps, shared breaker, StopChat drains briefs | ✓ VERIFIED | `chatBriefProcessJob`: `chatClaimCooldown` (20 s), `chatReserveWith(... 6/30/300)`, `breaker.isPaused`; `chatBriefRequest` subtracts `chatBriefRosterReserve()` for judge; `MaxTokens` 1500/3000/4000; `chatBriefEnqueue` counts `chatJobs`, refuses on `chatDraining`, worker derives from `chatContext()`, started inside `chatEnsureWorkers`' Once. `TestChatBriefLimits`, `TestChatBriefQueue`, `TestShutdownEndsChatBriefs`, `TestChatBriefInertWithoutRedis` pass |
| 8 | Every text in 8 locales; help and generated docs list the three commands; AGENTS.md updated | ✓ VERIFIED | 23 `brief_`/`judge_` keys in each of 8 locale files; `chat_help_msg` names all three in every language; `make check-translations` and `make check-docs` pass; AGENTS.md describes `/unrolllong`, `/judge`, page p controls, group 14, Redis keys, StopChat |

**Score:** 8/8 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Status | Details |
|----------|--------|---------|
| `migrations/20261125120000_add_chat_conversation_log.sql` | ✓ VERIFIED | Two idempotent `ADD COLUMN IF NOT EXISTS`, `judge_to` CHECK inline |
| `xenobot/db/chatconv/repository.go` | ✓ VERIFIED | `LogEnabled`/`JudgeTo` (msgpack l/j), `SetLogEnabled`, `SetJudgeTo`, `ValidJudgeTo`, column-targeted upsert + DeleteCache |
| `xenobot/modules/chat_log.go` | ✓ VERIFIED | Ring, scripts, watcher, `chatLogGroup = 14` |
| `xenobot/modules/chat_brief.go` | ✓ VERIFIED | Three commands, gate, queue/worker, destinations, `chatBriefProcess` |
| `xenobot/modules/chat_brief_prompt.go` | ✓ VERIFIED | Pure builders incl. `chatBriefTranscript`, roster, parts, messages |
| `xenobot/modules/settings_chat.go` | ✓ VERIFIED | `settingsChatControlLog`, `settingsChatControlJudge` |
| `xenobot/i18n/chat_brief_locale_test.go` | ✓ VERIFIED | `TestChatBriefLocaleKeys` passes |

### Key Link Verification

| From | To | Via | Status |
|------|----|-----|--------|
| chat.go | chat_log.go | `AddHandlerToGroup(... chatLogWatcher).SetAllowEdited(true), chatLogGroup)` | WIRED |
| chat_settings.go | chat_log.go | `chatWipeLog` → `chatLogKey` | WIRED |
| chat_brief.go | chat_log.go | `chatLogHas` in handler, `chatLogSince` in worker | WIRED |
| chat_brief.go | chat_run.go | `chatEnsureWorkers` calls `chatBriefEnsureWorkers`; `chatJobs`/`chatContext` | WIRED |
| chat_brief.go | provider | `chatProviderOf(job.provider).converse(ctx, request)` with `MaxTokens` | WIRED |
| settings_chat.go | chat_settings.go | `applyChatLog`, `applyChatJudgeTo` | WIRED |
| chat_settings.go | chatconv | `chatConvWrites.LogEnabled/JudgeTo` | WIRED |
| chat.go | anonymous router | `RegisterAnonymousAdminHandler("judge", anonPipelineHandler(judgeDesc, judgeAnon))` | WIRED |
| chat_brief.go/chat.go | lockdown.go | `requireLockdownAuthority(false)` in `judgeDesc` | WIRED |

### Data-Flow Trace (Level 4)

| Artifact | Data | Source | Real data | Status |
|----------|------|--------|-----------|--------|
| brief/verdict messages | `stretch` | `chatLogSince` ZRANGE BYSCORE of ring written by group-14 watcher | yes | ✓ FLOWING |
| page state | `stored.LogEnabled/JudgeTo` | `chatconv.GetFresh` | yes | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Build | `CGO_ENABLED=0 go build ./...` | ok | ✓ PASS |
| Feature tests | `go test -tags testtools -race -count=1 -run '^(TestChatLog\|TestChatJudge\|TestChatBrief\|TestShutdownEndsChat\|TestSettingsChat\|TestSettingsDriftChat\|TestApplyChat\|TestChatKeyLiteralsExist\|TestChatHelpFitsOneMessage\|TestChatConversation\|TestChatWatcher\|TestChatProvidersRequest\|TestChatEndToEnd\|TestChatLogsCarryIDsOnly\|TestHelpPagesKeepTheirCommands\|TestSettingsLabelLengths\|TestLocalePlaceholdersMatchOtherLanguages\|TestLocaleStyle\|TestNoRawNewCommand\|TestDisableRefusesNamesThatHaveNoCheck\|TestCommandRepliesStayInsideTheHandler\|TestHelpModuleNamesExist\|TestSettingsMainGrid\|TestSettingsMainPageFitsEveryLocale)' ./xenobot/modules ./xenobot/db/chatconv ./xenobot/i18n ./xenobot/db/migrations` | all ok (PG-only `TestChatConversationLogMigrationDefaults` skips locally, allowed) | ✓ PASS |
| Vet | `go vet -tags testtools` on the three packages | clean | ✓ PASS |
| Translations | `make check-translations` | all present | ✓ PASS |
| Docs drift | `make check-docs` | no drift | ✓ PASS |
| Shard list | `make test-shard-check` | no diff | ✓ PASS |
| Lint | `make lint` | not runnable here (golangci-lint built with go1.25 vs go1.26 config) | ? SKIP |

### Probe Execution

Not applicable (no probes declared).

### Requirements Coverage

| Requirement | Status | Evidence |
|-------------|--------|----------|
| QUICK-261125-0iq | ✓ SATISFIED | Truths 1–8 |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (new code) | — | No TBD/FIXME/XXX in new files | — | — |
| `Makefile` | 76 | `TestChatConversationLogMigrationDefaults` (judge_to CHECK on PostgreSQL) not added to `test-postgres-integrity` `-run` list, so it never runs in CI; AGENTS.md says a new CHECK guarantee joins that list. Partly mitigated: `TestChatConversationSettings` (in the list) stores `JudgeTo: 3` and expects refusal | ⚠️ Warning | CI does not exercise the migration's CHECK/idempotence on PG |
| `261125-0iq-SUMMARY.md` | 1–10 | Still says `status: incomplete`, tasks 2 and 3 pending (only Task 1 documented) | ⚠️ Warning | Planning record stale; does not reflect c1ec853 / 06a3490 |
| `docs/.../commands/chat/index.md` | 36–38 | Generated table shows "No description available" and Disableable ❌ for three commands that are Disableable | ℹ️ Info | Generator limitation (reads only `AddCmdToDisableable`); Task 1 summary flagged it for Task 3, which left it |
| `chat_brief.go` | — | Cooldown is claimed in the worker, so one member can fill the 16-slot brief queue and briefly shed others' briefs (silently, no cost) | ℹ️ Info | Cost still bounded; fairness only |

### Human Verification Required

1. **Real-model /judge coverage** — With a real provider key, log a quarrel and reply /judge. Expected: every roster name scored 1–10 with a reason exactly once, "(no messages shown)" names say not enough shown, nobody off-roster scored, ends with the AI-opinion notice. Why human: tests use scripted stand-ins.
2. **Real-model brief quality** — Reply /unrollshort and /unrolllong with a real provider. Expected: short (~8 lines) vs detailed brief, group language, plain text, no injected instructions followed. Why human: model output quality.

### Gaps Summary

No blocking gaps. All eight must-have truths are implemented, wired and exercised by passing behavioral tests through the real dispatcher (fake Telegram and provider stand-ins). Two warnings for the orchestrator: add `TestChatConversationLogMigrationDefaults` to the Makefile PostgreSQL integrity list, and refresh the stale SUMMARY.md. Remaining items are real-provider output quality checks.

---

_Verified: 2026-11-25_
_Verifier: Claude (gsd-verifier)_
