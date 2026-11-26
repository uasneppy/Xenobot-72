---
phase: 31-presence-foundation-settings-page-and-zero-ai-social-reactions
verified: 2026-11-26T23:25:35Z
status: passed
score: 5/5 must-haves verified
covered_files:
  - .planning/phases/31-presence-foundation-settings-page-and-zero-ai-social-reactions/31-01-PLAN.md
  - .planning/phases/31-presence-foundation-settings-page-and-zero-ai-social-reactions/31-01-SUMMARY.md
  - .planning/phases/31-presence-foundation-settings-page-and-zero-ai-social-reactions/31-02-PLAN.md
  - .planning/phases/31-presence-foundation-settings-page-and-zero-ai-social-reactions/31-02-SUMMARY.md
  - .planning/phases/31-presence-foundation-settings-page-and-zero-ai-social-reactions/31-03-PLAN.md
  - .planning/phases/31-presence-foundation-settings-page-and-zero-ai-social-reactions/31-03-SUMMARY.md
  - .planning/phases/31-presence-foundation-settings-page-and-zero-ai-social-reactions/31-04-PLAN.md
  - .planning/phases/31-presence-foundation-settings-page-and-zero-ai-social-reactions/31-04-SUMMARY.md
  - .planning/phases/31-presence-foundation-settings-page-and-zero-ai-social-reactions/31-05-PLAN.md
  - .planning/phases/31-presence-foundation-settings-page-and-zero-ai-social-reactions/31-05-SUMMARY.md
  - AGENTS.md
  - locales/en.yml
  - locales/es.yml
  - locales/fr.yml
  - locales/hi.yml
  - locales/id.yml
  - locales/pt.yml
  - locales/ru.yml
  - locales/uk.yml
  - migrations/20261127120000_add_chat_presence.sql
  - xenobot/config/config.go
  - xenobot/config/config_pure_test.go
  - xenobot/db/cache/local.go
  - xenobot/db/cache/local_test.go
  - xenobot/db/migrations/chat_presence_migration_test.go
  - xenobot/db/models/chat_presence.go
  - xenobot/db/presence/repository.go
  - xenobot/db/presence/repository_test.go
  - xenobot/db/presence/testmain_test.go
  - xenobot/i18n/settings_locale_test.go
  - xenobot/modules/presence.go
  - xenobot/modules/presence_react_api.go
  - xenobot/modules/presence_react_api_test.go
  - xenobot/modules/presence_reactions.go
  - xenobot/modules/presence_reactions_test.go
  - xenobot/modules/presence_test.go
  - xenobot/modules/settings_backup_test.go
  - xenobot/modules/settings_chat.go
  - xenobot/modules/settings_chat_test.go
  - xenobot/modules/settings_main_test.go
  - xenobot/modules/settings_phase18_test.go
  - xenobot/modules/settings_phase19_test.go
  - xenobot/modules/settings_phase29_test.go
  - xenobot/modules/settings_phase30_test.go
  - xenobot/modules/settings_presence.go
  - xenobot/modules/settings_presence_test.go
  - xenobot/modules/test_harness_test.go
covered_digest: "v4:sha256:a60cfa5284a60cc516912015044bf6c22ce28362a57c5e9eaf83fae1636c597e"
behavior_unverified: 0
overrides_applied: 0
---

# Phase 31: Presence Foundation, Settings Page and Zero-AI Social Reactions Verification Report

**Phase Goal:** Group admins can configure Interactive Presence from the bot's private chat while groups gain safe, reaction-aware zero-AI presence (Emoji Consensus and `👎` Retraction) without adding any shutdown drain or touching Staff Groups or locked-down chats.
**Verified:** 2026-11-26T23:25:35Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
| --- | --- | --- | --- |
| 1 | In the bot's private `/settings` menu, a group admin opens the new `Presence` page (`pr`, also linked from Conversations page `p`, with page `p` <= 8 rows and Main at 17 rows), views and changes zero-AI presence settings in all 8 locales, and sees the page hidden in Staff Groups and refused with "open in private" in group menus. | ✓ VERIFIED | `xenobot/modules/settings_presence.go` registers `pr` (`Order: 91`, `Icon: ✨`, `OwnerOnly: false`, `PrivateOnly: true`, `PrivateNoteKey: settings_pr_group_note`, `Hidden: settingsPresenceHidden`, controls `c`/`ct`/`r` gated on `settingsRightChangeInfo`, `!p.private`, and fail-closed `settingsPresenceStaffGate`). `xenobot/modules/settings_chat.go` keeps `OwnerOnly: true` and adds one `settings_btn_chat_presence` (`a=op&p=pr`) row. All 11 keys exist in 8/8 locales (`make check-translations`, `xenobot/i18n` `-race` PASS). `TestSettingsPresenceCallbackBytes`, `TestSettingsPresencePageFitsEveryLocale`, `TestSettingsPresenceFlows`, `TestSettingsChatPageFitsEveryLocale`, `TestSettingsChatLog/presence_shortcut_row_opens_page_pr_in_private`, and `TestSettingsMainPhase18` (`y\|pr`, `settingsMainOwnerRows = 17`) all PASS under `-race`. |
| 2 | When three or more distinct members react to a group message with the same non-hostile standard emoji (with Emoji Consensus enabled), the bot mirrors that emoji at most once per message after respecting the chat cooldown, never double-counting one member toggling reactions or a Premium user stacking emojis. | ✓ VERIFIED | `xenobot/modules/presence_reactions.go` (`presenceHandleConsensus`) excludes `mru.User == nil`, skips 10 hostile emojis (`presenceConsensusHostile`), updates per-emoji distinct-member sets via atomic Redis Lua `presenceConsensusUpdateScript` (`SADD`/`SREM`/`EXPIRE`/`SCARD`), checks `presenceIsLockedDown(chatID)` lazily at threshold crossing before claiming, claims `cons:cd` + `cons:done:<msgID>` atomically in one `{<chat>}`-tagged Lua script (`presenceConsensusClaimScript`), resolves best-effort author via `presenceLogAuthorOf`, and mirrors once via `presenceSetReaction(..., "cons")`. All 10 subtests of `TestPresenceConsensus` and `TestPresenceDiffReactions` PASS under `-race`. |
| 3 | When the message's author or a group admin reacts `👎` to a message the bot reacted to, the bot silently clears its reaction (sending explicit `[]gotgbot.ReactionType{}`) and extends that user's cooldown with zero group messages posted; a non-author regular member reacting `👎` changes nothing. | ✓ VERIFIED | `xenobot/modules/presence_reactions.go` (`presenceHandleRetract`) reads Redis `bot_react` before any Telegram call, authorizes `presenceIsAnonGroupAdmin(mru)` (`User == nil && ActorChat.Id == Chat.Id`), message author (`rec.AuthorID`), or `presenceIsAdmin`, rejects non-author regular members and external channels, atomically deletes `bot_react` (`presenceClaimBotReact`), calls `presenceClearReaction` passing explicit non-nil `[]gotgbot.ReactionType{}`, extends 24h user cooldown (`presenceExtendUserCooldown`) when `mru.User != nil && ctx.Err() == nil`, and sends zero group messages. `TestPresenceClearReactionWireJSON` (proving `"reaction":"[]"` over real HTTP multipart wire vs `Reaction: nil` omission control), `TestPresenceBotReact`, and all 9 subtests of `TestPresenceRetract` PASS under `-race`. |
| 4 | Every outgoing bot reaction is normalized (`U+FE0F`-stripped), sent with `IsBig: false`, restricted to standard emojis allowed by the chat's cached `AvailableReactions`, and tracked with 24-hour `bot_react` metadata; reaction processing runs under `StopChat` with zero new shutdown drains. | ✓ VERIFIED | `xenobot/modules/presence_react_api.go` (`presenceSetReaction`) normalizes via `normaliseReactionEmoji`, validates against `supportedReactionEmoji` and 20m cached `AvailableReactions` (`xenobot:cache:avail_react:<chat>`, evicted on `REACTION_INVALID`, fail-closed on `getChat` error), paces via isolated `presenceReactionPacer` with `IsBig: false` (never touching `chatBreakers`), and stores 24h `bot_react` (`{e, a, s}`) under `xenobot:presence:{<chat>}:bot_react:<msg>`. `LoadPresence` registers group-0 `handlers.NewReaction(reaction.All, presenceModule.handleReaction)` joining `chatJobs`/`chatDraining`/`chatContext()`. `git diff` confirms `main.go`, `go.mod`, and `go.sum` are untouched (0 new shutdown drains, 0 new external deps). `TestPresenceReactAPI`, `TestPresenceAvailReactionsCache`, `TestPresenceBotReact`, and `TestShutdownEndsPresenceReaction` PASS under `-race`. |
| 5 | Private chats, Staff Groups, groups under active lockdown, bots, anonymous channels, and slash commands never trigger ambient interactive presence paths. | ✓ VERIFIED | `xenobot/modules/presence.go` (`presenceMessageAllowed`) rejects nil/private/channel chats, slash commands (`raidFloodIsSlashText`), anonymous channels/admins and service IDs (`raidFloodSenderCounts`), bots (`IsBot`), and Staff Groups (`presenceIsStaffGroup`); `presenceIsLockedDown` fails closed (`true`) on active rows or DB errors. `presence_reactions.go` enforces chat type, bot-reactor, Staff Group, anonymous-actor exclusion on Consensus, and lazy fail-closed lockdown checks before claiming. `TestPresenceSafetyGate`, `TestPresenceRetract` (Staff Group & external channel subtests), and `TestPresenceConsensus` (Staff Group, anonymous admin/channel, lazy fail-closed lockdown subtests) all PASS under `-race`. |

**Score:** 5/5 truths verified (0 present, behavior-unverified)

### Required Artifacts

All 18 PLAN `must_haves.artifacts` passed `gsd-tools query verify.artifacts` (`all_passed: true`) and manual Level 1–4 inspection:

| Artifact | Expected | Status | Details |
| --- | --- | --- | --- |
| `xenobot/config/config.go` | `"message_reaction"` in `AllowedUpdates` (no `"message_reaction_count"`) | ✓ VERIFIED | Line 561 adds `"message_reaction"`; wired to both polling (`main.go:355`) and webhook (`xenobot/utils/httpserver/server.go:235`). |
| `migrations/20261127120000_add_chat_presence.sql` | Append-only idempotent DDL strictly after `20261126120000` with `CHECK (2..20)` | ✓ VERIFIED | Sorted last in `migrations/`; `CREATE TABLE IF NOT EXISTS chat_presence` with defaults and inline `chk_chat_presence_consensus_threshold`. |
| `xenobot/db/models/chat_presence.go` | `models.ChatPresence` without GORM `default:` tags on booleans | ✓ VERIFIED | `ConsensusEnabled` and `RetractEnabled` have `not null` only; matching `check:` constraint on `ConsensusThreshold`; registered in `test_harness_test.go`. |
| `xenobot/db/cache/local.go` | `skipLocal` bypass for `chat_presence:<chat>` | ✓ VERIFIED | `chatPresencePrefix = CacheKey("chat_presence") + ":"` checked in `skipLocal()` (`TestSkipLocalChatPresenceKey` PASS). |
| `xenobot/db/presence/repository.go` | CRUD with column-targeted upserts, `2..20` validation, cache invalidation | ✓ VERIFIED | `Get`, `GetFresh`, `SetConsensusEnabled`, `SetConsensusThreshold`, `SetRetractEnabled`, `Defaults`, `ValidConsensusThreshold`, `CacheKey`; `TestChatPresenceSettings` PASS. |
| `xenobot/modules/presence.go` | Shared `PRES-02` safety gate, `xenobot:presence:{<chat>}:*` keys, `applyPresence*` | ✓ VERIFIED | Operational keys sit outside `xenobot:cache:`; `TestPresenceSafetyGate`, `TestPresenceKeys`, `TestPresenceApply` PASS. |
| `xenobot/modules/presence_react_api.go` | Safe reaction API, 20m `AvailableReactions` cache, 24h `bot_react`, wire clear | ✓ VERIFIED | Dedicated `presenceReactionPacer`, `U+FE0F` strip, `IsBig: false`, explicit `[]gotgbot.ReactionType{}` clear; zero `chatBreakers` writes. |
| `xenobot/modules/presence_test.go` | Safety gate, Redis key isolation, and `applyPresence*` tests | ✓ VERIFIED | All subtests PASS under `-race`. |
| `xenobot/modules/presence_react_api_test.go` | `httptest` wire JSON clear, cache matrix, 400 breaker isolation, `bot_react` race tests | ✓ VERIFIED | All subtests PASS under `-race`. |
| `locales/en.yml` (+ 7 other locales) | Complete 8-locale translations for all 11 Presence & shortcut keys | ✓ VERIFIED | All 11 keys present in 8/8 locales (`en`, `uk`, `es`, `fr`, `hi`, `id`, `pt`, `ru`); placeholder & style checks PASS. |
| `xenobot/i18n/settings_locale_test.go` | `settingsLocaleLimits` ceilings & 20-rune/40-rune `TestSettingsLabelLengths` guards | ✓ VERIFIED | All 11 keys registered; `{"✨", "settings_page_presence"}` in Main grid check; `xenobot/i18n` suite PASS. |
| `xenobot/modules/presence_reactions.go` | Group-0 `LoadPresence` under `StopChat`, `👎` Retraction, Lua Emoji Consensus | ✓ VERIFIED | `RegisterLegacyModule("Presence", 252, LoadPresence)` without `SetModuleEnabled` (`/help` unchanged); `TestHelpModuleNamesExist` PASS. |
| `xenobot/modules/presence_reactions_test.go` | Reaction diff, Retraction, Consensus, and `TestShutdownEndsPresenceReaction` | ✓ VERIFIED | All subtests PASS under `-race` (~0.56s total). |
| `xenobot/modules/settings_presence.go` | Private-only Settings page `pr`, hidden in Staff Groups, fail-closed controls | ✓ VERIFIED | 4-row pure renderer, canonical base-10 `2..20` threshold validation, idempotent read-back setters. |
| `xenobot/modules/settings_chat.go` | Single `settings_btn_chat_presence` shortcut row on page `p` | ✓ VERIFIED | Owner-only preserved; worst-case 3-provider render fits `<= 8` rows in all 8 locales. |
| `xenobot/modules/settings_presence_test.go` | Callback byte cap, 8-locale fit, owner/admin/group/Staff Group flows | ✓ VERIFIED | All subtests PASS under `-race`. |
| `AGENTS.md` | Updated repository invariants for Phase 31 | ✓ VERIFIED | Documents `StopChat` reaction tracking (70s worst case unchanged), group-0 `NewReaction`, `AllowedUpdates` rolling-deploy note, `chat_presence`, and page `pr`. |

### Key Link Verification

All 14 PLAN `must_haves.key_links` passed `gsd-tools query verify.key-links` (`all_verified: true`) and direct code inspection:

| From | To | Via | Status | Details |
| --- | --- | --- | --- | --- |
| `xenobot/config/config.go` | `main.go` & `xenobot/utils/httpserver/server.go` | `config.AppConfig.AllowedUpdates` | ✓ WIRED | Passed to both `GetUpdatesOpts.AllowedUpdates` (polling) and `SetWebhookOpts.AllowedUpdates` (webhook). |
| `xenobot/db/presence/repository.go` | `xenobot/db/cache/local.go` | `cache.CacheKey("chat_presence", chatID)` -> `skipLocal` | ✓ WIRED | Exact prefix match `chatPresencePrefix = CacheKey("chat_presence") + ":"`. |
| `xenobot/db/presence/repository.go` | `xenobot/db/models/chat_presence.go` | `clause.OnConflict` upsert on `models.ChatPresence` | ✓ WIRED | Column-targeted updates + immediate `cache.DeleteCache`. |
| `xenobot/modules/test_harness_test.go` | `xenobot/db/models/chat_presence.go` | SQLite `AutoMigrate(&models.ChatPresence{})` | ✓ WIRED | Verified by all SQLite module integration tests. |
| `xenobot/modules/presence.go` | `xenobot/db/presence/repository.go` | `presenceWrites` & `presenceReadFresh` | ✓ WIRED | Typed `applyPresence*` writers with read-back. |
| `xenobot/modules/presence.go` | `xenobot/db/staff/repository.go` | `presenceStaffLookup = staff.GetStaffGroup` | ✓ WIRED | Used in `presenceIsStaffGroup` on both message and reaction gates. |
| `xenobot/modules/presence.go` | `xenobot/db/lockdown/repository.go` | `presenceLockdownLookup = lockdown.GetActiveFresh` | ✓ WIRED | Invoked lazily at Consensus threshold crossing, failing closed on error. |
| `xenobot/modules/presence_react_api.go` | `xenobot/modules/reactions.go` | `normaliseReactionEmoji` & `supportedReactionEmoji` | ✓ WIRED | Reused directly for emoji normalization and allow-list validation. |
| `xenobot/i18n/settings_locale_test.go` | `locales/*.yml` | `settingsLocaleLimits` & `TestSettingsLabelLengths` | ✓ WIRED | All 11 keys guarded across all 8 locales. |
| `xenobot/modules/presence_reactions.go` | `xenobot/modules/chat_run.go` | `chatJobs.start()`/`finish()`, `chatDraining.Load()`, `chatContext()` | ✓ WIRED | Verified live cancellation in `TestShutdownEndsPresenceReaction`. |
| `xenobot/modules/presence_reactions.go` | `xenobot/modules/presence_react_api.go` | `presenceSetReaction`, `presenceClearReaction`, `presenceReadBotReact`, `presenceClaimBotReact`, `presenceExtendUserCooldown` | ✓ WIRED | End-to-end miniredis + `httptest` wire tests pass. |
| `xenobot/modules/presence_reactions.go` | `xenobot/modules/presence.go` | `presenceChatTypeAllowed`, `presenceIsStaffGroup`, `presenceIsLockedDown`, `presenceIsAnonGroupAdmin` | ✓ WIRED | All safety gates exercised in `TestPresenceRetract` and `TestPresenceConsensus`. |
| `xenobot/modules/settings_presence.go` | `xenobot/modules/presence.go` | `applyPresenceConsensusEnabled`, `applyPresenceConsensusThreshold`, `applyPresenceRetractEnabled` | ✓ WIRED | Verified end-to-end in `TestSettingsPresenceFlows`. |
| `xenobot/modules/settings_chat.go` | `xenobot/modules/settings_presence.go` | `settings_btn_chat_presence` callback (`a=op&p=pr`) | ✓ WIRED | Verified in `TestSettingsChatLog/presence_shortcut_row_opens_page_pr_in_private`. |
| `xenobot/modules/settings_presence.go` | `xenobot/db/staff/repository.go` | `settingsPresenceStaffLookup = staff.GetStaffGroupFresh` | ✓ WIRED | Used in `Hidden`, `MainState`, `Open`, and `settingsPresenceStaffGate`. |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| --- | --- | --- | --- | --- |
| `xenobot/modules/settings_presence.go` | `stored presence.Settings` | `settingsPresenceReadSettings` -> `presence.GetFresh` (PostgreSQL `chat_presence`, defaulting to `Defaults()` when no row exists) | Yes (real GORM query; writes persist via `applyPresence*` and read back fresh) | ✓ FLOWING |
| `xenobot/modules/presence_reactions.go` | `settings presence.Settings` | `presenceSettingsGet` -> `presence.Get` (`skipLocal` Redis cache + PostgreSQL `chat_presence`) | Yes | ✓ FLOWING |
| `xenobot/modules/presence_reactions.go` | `rec presenceBotReact`, `count int64` | Operational Redis keys `xenobot:presence:{<chat>}:bot_react:<msg>` and `cons:u:<msg>:<hex>` via `presenceRedisClient()` | Yes (real Redis GET/DEL and Lua `SADD`/`SREM`/`SCARD`) | ✓ FLOWING |
| `xenobot/modules/presence_react_api.go` | `avail presenceAvailReactions` | `cache.GetFromCacheOrLoad` -> `b.GetChatWithContext` (`xenobot:cache:avail_react:<chat>`, 20m TTL) | Yes (real Telegram API call under `presenceReactionPacer`) | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| --- | --- | --- | --- |
| Full project compile (static) | `CGO_ENABLED=0 go build ./...` | Exit 0 | ✓ PASS |
| 8-locale translation parity | `make check-translations` | 1069 keys present in all 8 locales | ✓ PASS |
| Generated docs drift check | `make check-docs` | No drift detected (34 modules, 164 commands unchanged) | ✓ PASS |
| Static analysis / linter | `GOTOOLCHAIN=go1.26.0 make lint` | `0 issues.` | ✓ PASS |
| Core packages (`config`, `cache`, `migrations`, `presence`, `i18n`) | `go test -tags testtools -race -count=1 ./xenobot/config ./xenobot/db/cache ./xenobot/db/migrations ./xenobot/db/presence ./xenobot/i18n` | All 5 packages `ok` under `-race` | ✓ PASS |
| Presence, Settings (`pr`/`p`/Main/Backup/Phase 18-30), Help, CommandRepliesGuard, Backup Exclusion | `go test -tags testtools -race -count=1` targeted `-run` across `./xenobot/modules` & `./xenobot/db/backup` | All tests `PASS` under `-race` (0 failures, 0 races, 0 skips) | ✓ PASS |
| Git commits integrity | `gsd-tools query verify.commits` (21 Phase 31 commits) | `all_valid: true` (21/21) | ✓ PASS |

### Probe Execution

Phase 31 declares no `scripts/*/tests/probe-*.sh` scripts; automated verification is performed via Go `-race` unit/integration tests, `httptest` wire checks, `make check-translations`, `make check-docs`, and `make lint` (all PASS above).

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| --- | --- | --- | --- | --- |
| `PRES-01` | `31-01`, `31-04`, `31-05` | Subscribe `"message_reaction"` in `AllowedUpdates` and process reaction events safely across replicas under `StopChat` with zero new shutdown drains (`"poll"` already subscribed; `handlers.NewPoll` listener deferred to Phase 35 `TOYS-02`). | ✓ SATISFIED | `xenobot/config/config.go:561`, `TestLoadConfig`, `LoadPresence` at group 0 in `presence_reactions.go`, `TestShutdownEndsPresenceReaction`, `main.go` unchanged, `AGENTS.md` updated. |
| `PRES-02` | `31-02`, `31-04`, `31-05` | Shared safety gate skipping private chats, Staff Groups (`staff.GetStaffGroup`), active lockdowns (`lockdown.GetActiveFresh`), bots, anonymous channels, and slash commands. | ✓ SATISFIED | `xenobot/modules/presence.go`, `presence_reactions.go`, `settings_presence.go`; `TestPresenceSafetyGate`, `TestPresenceRetract`, `TestPresenceConsensus`, `TestSettingsPresenceFlows`. |
| `PRES-03` | `31-01`, `31-03`, `31-05` | Private-only `Presence` Settings page (`pr`, hidden in Staff Groups, 8 locales) backed by `chat_presence` (`skipLocal`), linked from Conversations page `p` (`p <= 8` rows, Main `17` rows). | ✓ SATISFIED | `migrations/20261127120000_add_chat_presence.sql`, `xenobot/db/presence`, `xenobot/db/cache/local.go`, 8 locale files + `settings_locale_test.go`, `settings_presence.go`, `settings_chat.go`; all UI/Main/Staff/DB tests PASS. |
| `PRES-05` | `31-02` | Normalize (`U+FE0F`-stripped) and validate every outgoing reaction against `supportedReactionEmoji` intersected with 20m cached `AvailableReactions`, send `IsBig: false`, store 24h `bot_react` metadata outside cache. | ✓ SATISFIED | `xenobot/modules/presence_react_api.go`; `TestPresenceReactAPI`, `TestPresenceAvailReactionsCache`, `TestPresenceBotReact`, `TestPresenceClearReactionWireJSON`. |
| `REACT-04` | `31-04` | Zero-AI Emoji Consensus mirrors a non-hostile standard emoji when >= N distinct members react (Redis Lua `SADD`/`SREM` diffs, chat cooldown + once-only latch). | ✓ SATISFIED | `xenobot/modules/presence_reactions.go` (`presenceHandleConsensus`, `presenceConsensusUpdateScript`, `presenceConsensusClaimScript`); `TestPresenceDiffReactions`, `TestPresenceConsensus` (10 subtests). |
| `REACT-05` | `31-04` | Zero-AI `👎` Retraction silently clears bot reaction (`[]gotgbot.ReactionType{}`) and extends user cooldown when author, group admin, or anonymous group admin reacts `👎`. | ✓ SATISFIED | `xenobot/modules/presence_reactions.go` (`presenceHandleRetract`), `presence_react_api.go` (`presenceClearReaction`); `TestPresenceClearReactionWireJSON`, `TestPresenceRetract` (9 subtests). |

Orphaned requirements check: `REQUIREMENTS.md` traces exactly 6 requirements to Phase 31 (`PRES-01`, `PRES-02`, `PRES-03`, `PRES-05`, `REACT-04`, `REACT-05`); all 6 are claimed by plans `31-01`..`31-05` and verified above (0 orphaned).

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| --- | --- | --- | --- | --- |
| — | — | None in Phase 31 files (`TBD`/`FIXME`/`XXX`: 0 matches; `TODO`/`HACK`/`PLACEHOLDER`: 0 matches in Phase 31 code/keys; pre-existing `# TODO: Translate` comments on legacy `reactions_help_msg` in `fr`/`pt`/`ru` are outside Phase 31 scope) | — | None |

### Non-Blocking Notes: Optional Live-Telegram Sanity Checks (Deferred to Milestone-End)

Per `PROJECT.md` and Xenobot-72 milestone policy, live Telegram group checks are intentionally deferred to milestone-end/owner testing and do not block Phase 31 progression:
1. After deploying v1.3 and terminating all v1.2 replicas, verify `getWebhookInfo` lists `"message_reaction"` in `allowed_updates` (and not `"message_reaction_count"`).
2. In a real test group's private `/settings`, open `✨ Presence` (both from Main and via `✨ Presence →` on Conversations page `p`), toggle Emoji Consensus on, verify 3 real members reacting `🔥` mirrors once, and confirm author or admin reacting `👎` clears the bot's reaction silently.
3. Confirm `✨ Presence` does not appear in a Staff Group's `/settings` menu and that opening `pr` from a group menu displays the private-only notice.

### Gaps Summary

No gaps found. All 5 Phase 31 success criteria and all 6 mapped requirements (`PRES-01`, `PRES-02`, `PRES-03`, `PRES-05`, `REACT-04`, `REACT-05`) are implemented, wired end-to-end, free of regressions or anti-patterns, and verified under `-race`, build, translation, doc-drift, and lint gates.

---

_Verified: 2026-11-26T23:25:35Z_
_Verifier: the agent (gsd-verifier)_
