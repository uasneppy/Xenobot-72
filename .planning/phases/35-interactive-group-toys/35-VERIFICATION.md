---
phase: 35-interactive-group-toys
verified: 2026-11-27T13:24:48Z
status: passed
score: 3/3 must-haves verified
behavior_unverified: 0
overrides_applied: 0
covered_files:
  - .planning/phases/35-interactive-group-toys/35-01-PLAN.md
  - .planning/phases/35-interactive-group-toys/35-01-SUMMARY.md
  - .planning/phases/35-interactive-group-toys/35-02-PLAN.md
  - .planning/phases/35-interactive-group-toys/35-02-SUMMARY.md
  - .planning/phases/35-interactive-group-toys/35-03-PLAN.md
  - .planning/phases/35-interactive-group-toys/35-03-SUMMARY.md
  - .planning/phases/35-interactive-group-toys/35-04-PLAN.md
  - .planning/phases/35-interactive-group-toys/35-04-SUMMARY.md
  - .planning/phases/35-interactive-group-toys/35-05-PLAN.md
  - .planning/phases/35-interactive-group-toys/35-05-SUMMARY.md
  - .planning/phases/35-interactive-group-toys/35-06-PLAN.md
  - .planning/phases/35-interactive-group-toys/35-06-SUMMARY.md
  - .planning/phases/35-interactive-group-toys/35-07-PLAN.md
  - .planning/phases/35-interactive-group-toys/35-07-SUMMARY.md
  - AGENTS.md
  - Makefile
  - docs/src/content/docs/commands/chat/index.md
  - locales/en.yml
  - locales/es.yml
  - locales/fr.yml
  - locales/hi.yml
  - locales/id.yml
  - locales/pt.yml
  - locales/ru.yml
  - locales/uk.yml
  - migrations/20261201120000_add_presence_toys.sql
  - xenobot/db/backup/backup.go
  - xenobot/db/backup/presence_backup_test.go
  - xenobot/db/backup/roundtrip_test.go
  - xenobot/db/backup/types.go
  - xenobot/db/migrations/chat_presence_toys_migration_test.go
  - xenobot/db/models/chat_presence.go
  - xenobot/db/presence/repository.go
  - xenobot/db/presence/repository_test.go
  - xenobot/i18n/chat_toys_locale_test.go
  - xenobot/i18n/settings_locale_test.go
  - xenobot/modules/chat.go
  - xenobot/modules/chat_inspect.go
  - xenobot/modules/chat_inspect_test.go
  - xenobot/modules/chat_polls.go
  - xenobot/modules/chat_polls_test.go
  - xenobot/modules/chat_run.go
  - xenobot/modules/presence.go
  - xenobot/modules/presence_reactions.go
  - xenobot/modules/presence_test.go
  - xenobot/modules/settings_phase33_test.go
  - xenobot/modules/settings_presence.go
  - xenobot/modules/settings_presence_test.go
  - xenobot/utils/chatprompt/toys.go
  - xenobot/utils/chatprompt/toys_test.go
covered_digest: "v4:sha256:874a531020a517b455ef5071173a788ce6a94ad6223ae0247d71244ceba37a08"
---

# Phase 35: Interactive Group Toys Verification Report

**Phase Goal:** Permitted group members can spark in-character participation through AI-drafted native polls with automatic deadpan tie-breakers (`/referendum`) and vision-capable "Facility Inspection" verdicts with matching emoji reactions (`/inspect`).
**Verified:** 2026-11-27T13:24:48Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| #   | Truth | Status | Evidence |
| --- | --- | --- | --- |
| 1   | When `/referendum` is enabled (and Owner Conversations is on), a permitted member (following `Audience`) runs `/referendum` from recent same-thread context or a replied debate and gets a validated, sanitized native Telegram poll (`SendPoll`) bounded by the dedicated `toy:b` hourly budget. | ✓ VERIFIED | `xenobot/modules/chat.go` registers `referendumDesc` via `helpers.WrapCommand` and adds `"referendum"` to `DefaultHelpRegistry().AltHelpOptions["Chat"]`. `xenobot/modules/chat_polls.go` (`referendumCommand`, `chatToyGateOf`, `chatReferendumProcess`) enforces `presenceChatTypeAllowed`, real non-bot user, `!presenceIsStaffGroup`, `!presenceIsLockedDown`, `chatAudienceAllows`, `conv.Enabled && pres.ReferendumEnabled`, and `chatProviderConfigured`; pulls context from `extraction.ReplyTarget`, same-thread `chatLogRecent`, and topic argument; executes on the `StopChat`-joined `chatToyQueue` (`chatToysEnqueue`, `chatContext()`) behind per-chat/per-user cooldowns and dedicated `"toy:b"` hourly budgets (`chatReserveWith`); builds and parses via leaf `xenobot/utils/chatprompt/toys.go` (`ReferendumMaxTokens = 768`, `ParseReferendumJSON` enforcing `<= 300`-rune question and `2..5` deduplicated `<= 100`-rune options); preserves `@\u200b` neutralization via `chatSanitizeText` + `chatCutRunes`; sends `SendPollWithContext` (`OpenPeriod: 900`, `DisableNotification: true`, `AllowAddingOptions: false`, `MessageThreadId: job.threadID`); and persists 25 h metadata at `xenobot:presence:poll:<poll_id>`. Verified by `TestReferendumDoubleGateAndSafety`, `TestReferendumPollFlowAndTopicIsolation`, `TestShutdownEndsChatToys`, `TestParseReferendumJSON`, `TestSettingsPresenceRoomCatchup`, and `TestBackupPresenceArchiveStripped` under `-race`. |
| 2   | When a bot-created `/referendum` poll closes in a tie with at least one vote (`TotalVoterCount > 0`), the bot claims it once-only across replicas and posts one deadpan Facility Tie-Breaker verdict in the original chat and forum topic; zero-vote or non-tied polls post nothing. | ✓ VERIFIED | `xenobot/config/config.go` includes `"poll"` in `AllowedUpdates`; `xenobot/modules/presence_reactions.go` (`LoadPresence`) registers `handlers.NewPoll(poll.All, chatModule.handlePollUpdate)` at group `0`. `xenobot/modules/chat_polls.go` (`handlePollUpdate` -> `chatHandlePollUpdate`) joins `StopChat` (`chatJobs.start()`/`finish()`, `chatDraining.Load()`, `chatContext()`), gates on `p.IsClosed && p.TotalVoterCount > 0 && len(tiedOptions) >= 2` (`chatPollTiedOptions`), resolves `{chat_id, message_id, thread_id, lang, question}` from `presencePollKey(pollID)`, enforces `!presenceIsStaffGroup && !presenceIsLockedDown`, claims once-only across replicas via `client.SetNX(..., presencePollTieKey(pollID), "1", chatPollTieClaimTTL)` (`chatPollClaimTie`), generates a `TieBreakerMaxTokens = 512` verdict (with localized `referendum_tiebreaker_fallback` on AI failure/refusal), and replies in HTML to `meta.MessageID` in `meta.ChatID` with `MessageThreadId: meta.ThreadID` and `AllowSendingWithoutReply: false`. Verified by `TestPollTieBreaker` (concurrent `SETNX` once-only tie-breaker, topic preservation, unknown/open/0-vote/non-tied ignores, Staff/lockdown blocks, AI failure fallback, deleted poll silence) and `TestShutdownEndsChatToys` under `-race`. |
| 3   | When `/inspect` is enabled (and Owner Conversations is on), a permitted member replying `/inspect` to a text message, photo, or sticker receives a crisp 2–3 line deadpan "Facility Inspection" verdict (with vision via `chat_media.go` when available) plus a validated matching reaction emoji recorded in `bot_react` so `👎` retraction clears it. | ✓ VERIFIED | `xenobot/modules/chat.go` registers `inspectDesc` via `helpers.WrapCommand` and adds `"inspect"` to `DefaultHelpRegistry().AltHelpOptions["Chat"]`. `xenobot/modules/chat_inspect.go` (`inspectCommand`, `chatInspectJobFromReply`, `chatInspectProcess`) enforces group chat, non-bot user, `!presenceIsStaffGroup`, `!presenceIsLockedDown`, mandatory reply target (`inspect_usage`), `chatToyGateOf(..., chatToyInspect)`, per-user cooldown (`presenceToyUserCooldownKey`), and `"toy:b"` budget on the `StopChat` toy lane (`chatToysEnqueue`); reuses `chatExtractReactionMediaRef`/`chatExtractMediaRef`, `chatReactDownloadVision` (`chat_media.go`), and `chatReactCallProvider` (with Featherless `400/422` `text_only` retry/cache without tripping `chatBreakers`); enforces `InspectMaxTokens = 512`, `ParseInspectJSON` (`cleanInspectVerdict` `<= 3` lines / `<= 600` runes, `U+FE0F` strip, lookalike `😂`/`🐱`/`😹`/`😻`/`😼` rejection, normalized `AvailableReactions` palette check); replies with `AllowSendingWithoutReply: false` and `MessageThreadId` preserved; and records `presenceSetReaction(ctx, job.bot, job.chatID, job.targetMsgID, job.targetAuthorID, parsed.Emoji, "inspect")` (setting `targetAuthorID = 0` for `SenderChat` posts). Verified by `TestInspectGatesAndUsage`, `TestInspectVisionAndRetraction` (including end-to-end `👎` retraction clearing with `[]`), and `TestShutdownEndsInspect` under `-race`. |

**Score:** 3/3 truths verified (0 present, behavior-unverified)

### Required Artifacts & Key Links

| Artifact / Link | Status | Verification Details |
| --- | --- | --- |
| `migrations/20261201120000_add_presence_toys.sql`, `xenobot/db/migrations/chat_presence_toys_migration_test.go`, `xenobot/db/models/chat_presence.go` | ✓ VERIFIED | Append-only idempotent SQL migration strictly after `20261130120000_add_presence_room_and_catchup.sql`; `models.ChatPresence` adds `ReferendumEnabled` and `InspectEnabled` with zero GORM `default:` tags. |
| `xenobot/db/presence/repository.go`, `xenobot/db/backup/{types,backup}.go` | ✓ VERIFIED | `Settings` (`rf`, `in`), `SetReferendumEnabled`, `SetInspectEnabled` with `CacheKey(chatID)` invalidation, `ReplacePortableTx` parity, and `BackupModulePresence` export/import parity (keeping `chat_conversations` excluded and `copy_settings.go` untouched). |
| `xenobot/utils/chatprompt/toys.go`, `xenobot/utils/chatprompt/toys_test.go` | ✓ VERIFIED | Stdlib-only leaf prompt builders (`ReferendumMaxTokens = 768`, `InspectMaxTokens = 512`, `TieBreakerMaxTokens = 512`) and strict JSON/text parsers; `TestChatpromptPackageIsLeaf` passes. |
| `locales/{en,uk,es,fr,hi,id,pt,ru}.yml`, `xenobot/i18n/{settings_locale_test,chat_toys_locale_test}.go` | ✓ VERIFIED | All 1071 keys present across all 8 locales (`make check-translations` pass; `xenobot/i18n` `-race` tests pass including `TestChatToysLocaleKeys`, `TestChatToysKeyLiteralsExist`, `TestChatHelpIncludesToys`, `TestSettingsLocaleKeys`, `TestSettingsLabelLengths`). |
| `xenobot/modules/{presence,settings_presence}.go`, `xenobot/modules/{settings_presence,settings_phase33}_test.go` | ✓ VERIFIED | Operational toy keys outside `xenobot:cache:`, `applyPresenceReferendumEnabled`/`applyPresenceInspectEnabled`, and sub-page `pc` (`rf`/`in`, 6 rows `<= 8`, `pr` 8 rows, `31` pages, `17` Main rows) with `settingsPresenceNeedConv` Owner×Admin double-gating and fail-closed Staff Group refusal. |
| `xenobot/modules/{chat,chat_run,chat_polls,chat_inspect,presence_reactions}.go` | ✓ VERIFIED | `/referendum` and `/inspect` registered via `helpers.WrapCommand`, `chatToysEnsureWorkers` and `chatToysStatsSection` wired in `chat_run.go`, group-0 `handlers.NewPoll(poll.All, chatModule.handlePollUpdate)` registered in `LoadPresence`, all joining `StopChat` with zero new shutdown drains (`main.go` untouched, `70 s` worst-case shutdown and `stop_grace_period: 75s` unchanged). |
| `Makefile`, `docs/src/content/docs/commands/chat/index.md`, `AGENTS.md` | ✓ VERIFIED | `TestChatPresenceToysMigrationDefaults` registered in `Makefile`; `make check-docs` passes with zero drift; `AGENTS.md` documents all Phase 35 invariants. |

_Note on automated `verify.artifacts` / `verify.key-links`:_ Plans `35-01`, `35-02`, `35-04`, `35-05`, and `35-06` passed both `verify.artifacts` (`all_passed: true`) and `verify.key-links` (`all_verified: true`). In `35-03-PLAN.md`, `verify.key-links` flagged `referendum_usage` as missing because the executed implementation uses `referendum_need_context` (per Plan 03 Task 1 spec and `35-03-SUMMARY.md`; enforced by `TestChatToysLocaleKeys` and `TestChatToysKeyLiteralsExist`). In `35-07-PLAN.md`, `verify.artifacts` flagged `docs/src/content/docs/` as a directory path rather than the generated file `docs/src/content/docs/commands/chat/index.md`, which exists and passes `make check-docs` with zero drift.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| --- | --- | --- | --- | --- |
| `TOYS-01` | 35-01, 35-02, 35-03, 35-04, 35-05, 35-07 | Permitted members can trigger `/referendum` for an AI-drafted native poll (`SendPoll`) from recent same-thread context or a replied debate, with strict JSON validation, sanitization, `"toy:b"` hourly budget, and Redis poll metadata | ✓ SATISFIED | `migrations/20261201120000_add_presence_toys.sql`, `xenobot/utils/chatprompt/toys.go`, `xenobot/modules/{chat,chat_run,chat_polls,presence,settings_presence}.go`; `TestReferendumDoubleGateAndSafety`, `TestReferendumPollFlowAndTopicIsolation`, `TestParseReferendumJSON`, `TestSettingsPresenceRoomCatchup`. |
| `TOYS-02` | 35-02, 35-03, 35-05, 35-07 | Closed tied `/referendum` poll with `TotalVoterCount > 0` claimed once-only across replicas by `handlers.NewPoll` and answered with a deadpan Facility Tie-Breaker verdict in the original chat/topic | ✓ SATISFIED | `xenobot/config/config.go` (`"poll"` in `AllowedUpdates`), `xenobot/modules/presence_reactions.go` (`handlers.NewPoll`), `xenobot/modules/chat_polls.go` (`handlePollUpdate`, `chatPollClaimTie`, `chatPollSendTieBreaker`); `TestPollTieBreaker`, `TestShutdownEndsChatToys`. |
| `TOYS-03` | 35-01, 35-02, 35-03, 35-04, 35-06, 35-07 | Permitted members replying `/inspect` to a text, photo, or sticker receive a 2–3 line deadpan "Facility Inspection" verdict (vision-enabled via `chat_media.go`) plus a validated reaction emoji recorded in `bot_react` | ✓ SATISFIED | `xenobot/utils/chatprompt/toys.go`, `xenobot/modules/{chat,chat_inspect,presence,settings_presence}.go`; `TestInspectGatesAndUsage`, `TestInspectVisionAndRetraction`, `TestShutdownEndsInspect`. |

### Behavioral Spot-Checks & Automated Gate Results

| Check | Command | Result | Status |
| --- | --- | --- | --- |
| Full Go build | `CGO_ENABLED=0 go build ./...` | Exit 0 | ✓ PASS |
| Translations parity (8 locales) | `make check-translations` | 1071 keys verified across all 8 locales (0 missing) | ✓ PASS |
| Generated docs drift | `make check-docs` | Exit 0 — zero drift across 168 commands / 35 modules | ✓ PASS |
| Linter (default + `testtools`) | `GOTOOLCHAIN=go1.26.0 make lint && GOTOOLCHAIN=go1.26.0 golangci-lint run --build-tags testtools ./xenobot/modules/... ./xenobot/db/... ./xenobot/utils/chatprompt/... ./xenobot/i18n/...` | 0 issues | ✓ PASS |
| Shard parity check | `make test-shard-check` | Exit 0 | ✓ PASS |
| DB, utils, i18n & config `-race` suites | `go test -tags testtools -race -count=1 ./xenobot/db/migrations ./xenobot/db/presence ./xenobot/db/backup ./xenobot/utils/chatprompt ./xenobot/i18n ./xenobot/config` | Passed (`db/migrations` 1.442s, `db/presence` 1.255s, `db/backup` 1.531s, `chatprompt` 1.103s, `i18n` 43.086s, `config` 1.237s) | ✓ PASS |
| Command registry guard | `go test -tags testtools -race -count=1 ./xenobot/utils/helpers -run 'TestNoRawNewCommand'` | Passed (1.988s) | ✓ PASS |
| Modules Phase 35 & regression `-race` suites | `go test -tags testtools -race -count=1 -v ./xenobot/modules -run 'TestReferendum\|TestPollTieBreaker\|TestShutdownEndsChatToys\|TestInspect\|TestShutdownEndsInspect\|TestPresence\|TestSettingsPresence\|TestPhase33Codes\|TestSettingsMain\|TestBackup\|TestSettingsCopy'` | Passed (4.837s) | ✓ PASS |

### Anti-Patterns Found

- Zero debt markers (`TBD`, `FIXME`, `XXX`, `TODO`, `HACK`, `PLACEHOLDER`) across all Phase 35 Go, SQL, and doc files.
- Zero GORM `default:` tags on `ReferendumEnabled` and `InspectEnabled` in `xenobot/db/models/chat_presence.go`.
- Zero changes to `xenobot/modules/copy_settings.go` or `main.go` (`git diff be156f27507049c66ec984b32eb96cb38147c465..HEAD -- xenobot/modules/copy_settings.go main.go` is empty; `StopChat` 70 s webhook worst-case shutdown and `stop_grace_period: 75s` unchanged).

### Non-Blocking Notes (Deferred Live-Telegram Group Checks)

Optional live-Telegram group checks remain deferred to milestone-end per project policy (`ROADMAP.md` / `STATE.md`):
1. Live Telegram group/forum topic exercise of `/referendum` AI-drafted native poll (`SendPoll` in topic, `OpenPeriod`, `DisableNotification`, `"toy:b"` budget) and closed tied-poll `Update.Poll` Facility Tie-Breaker delivery.
2. Live Telegram group exercise of `/inspect` on text, photo, and animated/static sticker targets with vision verdict + `bot_react` emoji stamping and author/admin `👎` retraction.
3. Live private-chat Settings sub-page `pc` walkthrough (`rf` and `in` toggles, Owner×Admin `settings_pr_ai_need_owner` gate, and Backup `/export`/`/import` roundtrip).
4. Live PostgreSQL integration run (`make test-postgres-integrity` with `XENOBOT_TEST_DATABASE=true`) for `TestChatPresenceToysMigrationDefaults`.

---

_Verified: 2026-11-27T13:24:48Z_
_Verifier: the agent (gsd-verifier)_
