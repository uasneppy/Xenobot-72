# Project Research Summary

**Project:** Xenobot-72 — Milestone v1.3 Interactive Presence
**Domain:** Telegram Group-Management Bot — Interactive AI & Zero-AI Social Presence (Go 1.26, `gotgbot/v2`, PostgreSQL/GORM, Redis)
**Researched:** 2026-11-26
**Confidence:** HIGH

## Executive Summary

Milestone v1.3 transforms Xenobot-72 from an on-demand moderation and `@mention` assistant into a contextually aware, quietly interactive group presence across 9 complementary features (contextual AI emoji reactions with vision, Emoji Consensus, `👎` retraction, smarter static reactions, cat-sticker micro-triggers, Crowd-Picked Highlights / "Facility Archive", ambient room context in conversations, frictionless catch-up + silent `✍` PM briefs, vocative addressing, `/referendum` polls + tie-breakers, and playful `/inspect`). The decisive finding from codebase inspection, `gotgbot/v2` source verification, and live empirical Spikes 001–003 is that **zero new external Go dependencies and zero new shutdown drains are required**. Every required Bot API method, update handler, Redis primitive, vision media downloader, and LLM leaf client is already pinned and battle-tested in the repository.

The recommended architecture layers Interactive Presence onto the existing `-10..14` message pipeline and `StopChat` lifecycle with strict separation of concerns: **Owner-only AI consent** stays untouched in `chat_conversations` (page `p`, permanently excluded from Backup/Copy Settings), while all **admin-configurable zero-AI controls and dormant AI sub-toggles** live in a new `chat_presence` table (`xenobot/db/presence/`, `skipLocal` cache) managed through a new private Settings page `pr` (`OwnerOnly: false`, paired with `y` on Main so the 17-row layout stays unchanged). All asynchronous AI work joins a dedicated `chatPresenceQueue` worker lane under `StopChat` (`chatJobs` + `chatContext()`), keeping the 70 s webhook shutdown budget and `stop_grace_period: 75s` unchanged and preventing slow background reactions from starving human `/ask` replies.

The primary risks are silent Telegram/library edge cases and community-annoyance regressions rather than technical unknowns: `gotgbot/v2` omits `"reaction"` on the wire when `Reaction: nil` (breaking `👎` retraction unless `[]gotgbot.ReactionType{}` is passed), `MessageReactionUpdated` and `Poll` updates omit message bodies/authors/`chat_id`s, `gemini-4-hydrogen` and Featherless reasoning models fail 40–60% below `MaxTokens = 512`, Go RE2 `\b` silently breaks on Ukrainian/Russian Cyrillic, and neither `chat_watch.go`, `chat_log.go`, nor `reactions.go` currently guards against Staff Groups or active Lockdowns. Every one of these risks has a verified, low-cost mitigation baked into the 3-phase build order below.

---

## Key Findings

### Recommended Stack

Zero `go.mod` additions or version bumps are needed (see [STACK.md](STACK.md)). The real stack work is enabling `"message_reaction"` in `config.AppConfig.AllowedUpdates` (reusing already-enabled `"poll"`), handling verified `gotgbot/v2` wire quirks, and reusing Xenobot-72's existing infrastructure cleanly.

**Core technologies & verified surfaces:**
- **Go `1.26.0` stdlib (`encoding/json`, `math/rand/v2`, `unicode`/`unicode/utf8`):** Structured JSON parsing for reaction/poll/inspect pickers, lock-free ChaCha8 probability rolls, and ReDoS-free Unicode word-boundary & vocative scanning across all 8 UI languages (never Go RE2 `\b`, which is ASCII-only and breaks Ukrainian/Russian Cyrillic).
- **`gotgbot/v2` (`v2.0.0-rc.36`):** Provides `SetMessageReactionWithContext`, `GetChatWithContext` (`ChatFullInfo.AvailableReactions`), `handlers.NewReaction`, `SendPollWithContext` + `handlers.NewPoll`, `ForwardMessageWithContext` / `CopyMessageWithContext`, `Sticker.Emoji`, and `Message.Quote`. Verified wire quirks: clear reactions with explicit non-nil `[]gotgbot.ReactionType{}` (never `nil`), treat `len(AvailableReactions) == 0` as all 73 standard emojis allowed, guard `MessageReactionUpdated.User == nil` (`ActorChat`), and note `ForwardMessageOpts` has no `ReplyMarkup` field (only `CopyMessageOpts` attaches an inline `t.me/c/` jump button in one call).
- **`go-redis/v9` (`v9.22.0`) + `miniredis/v2` (`v2.39.0`):** Cluster-safe `{<chat>}` Lua scripts for message-gap/cooldown throttling and `SADD`/`SREM`/`SCARD` distinct-reactor sets, `bot_react` metadata (storing `author_id` for `👎` retraction), poll ID→chat/message/thread mappings, and extended `chat_log` (`At int64` from `msg.Date`). All operational runtime keys live outside `xenobot:cache:` so `CLEAR_CACHE_ON_STARTUP` never wipes active polls, cooldowns, or reaction tracking.
- **GORM + PostgreSQL/SQLite (`gorm v1.31.2`):** Append-only idempotent SQL migrations strictly after `20261126120000_allow_featherless_chat_provider.sql`, adding `chat_presence` (plus `chat_presence_archived` dedup table) and backward-compatible columns on `reactions`; boolean fields omit GORM `default:` tags so explicit `false` always persists.
- **Existing Leaf AI Clients (`claude`, `gemini`, `featherless`) + `chat_media.go`:** Empirically validated across Spikes 001–003 with zero client changes required when using prompt-instructed JSON (`{"emoji":"..."}` / `{"emoji":"NONE"}`), **`MaxTokens = 512`** (hard floor for `gemini-4-hydrogen` thinking tokens and Featherless reasoning prose), and strict `supportedReactionEmoji ∩ AvailableReactions` post-filtering (`U+FE0F` stripped).

### Expected Features

All 9 scoped features decompose cleanly into foundational anti-annoyance table stakes and high-character differentiators (see [FEATURES.md](FEATURES.md)):

**Must have (table stakes):**
- **`"message_reaction"` update foundation & shared `AvailableReactions` cache** — shared substrate for `👎` retraction, Emoji Consensus, Highlights Archive, and `✍` PM brief.
- **Strict anti-annoyance & safety gate** — hard skip for private chats, Staff Groups (`staff.IsStaffGroup`), active lockdowns (`lockdown.GetActive`), bots, anonymous channels, slash commands, and static-reacted messages (`ctx.Data` marker), plus cross-replica Redis message-gap + chat/user cooldown + probability roll + hourly budget (`IsBig: false`).
- **Two-tier permission boundary & private Settings UI (`pr`, 8 locales)** — Owner unlocks AI/Conversations on `OwnerOnly` page `p` (2-tap data notice); admins toggle zero-AI features independently and AI interactive features once Owner has unlocked AI.
- **`👎` Retraction (Zero-AI)** — message author or group admin silently clears Xenobot's reaction and extends their user cooldown (zero public group noise).
- **Smarter static reactions (`reactions.go`)** — Unicode word-boundary option, caption matching, trigger chance %, per-rule/chat cooldowns, Staff/lockdown guard, and `ctx.Data` handoff to prevent AI overwrite.
- **Frictionless catch-up (`/unrollshort` / `/unrolllong <N|me|2h> [pm]`)** — timestamped `chat_log` (`At: msg.Date`) with thread isolation and PM delivery.
- **100% silent degradation on ambient/reaction paths** — zero group error spam on provider errors, `REACTION_INVALID`, deleted messages, or `403` PM blocks.

**Should have (competitive differentiators):**
- **Contextual Random AI Emoji Reactions (with Vision)** — dry, observant persona reacting across all 73 standard emojis ∩ `AvailableReactions` (including photos/stickers via `chat_media.go`), defaulting to `NONE` on mundane chatter, drama, and war/tragedy red lines.
- **Emoji Consensus (Zero-AI)** — joins organic ≥ 3 distinct-member standard emoji moments (excluding negative/pile-on emojis) at zero AI cost.
- **Cat-Sticker Micro-Triggers (Zero-AI)** — matches free `msg.Sticker.Emoji` cat glyphs to stamp occasional `👀`/`❤`, tying into Xenobot-72's canonical persona.
- **Crowd-Picked Highlights ("Facility Archive", Zero-AI)** — distinct-member reaction threshold (excluding author self-reactions and hostile emojis) archiving once-only into a live-proof linked channel or forum topic (`/setarchive`).
- **Ambient Room Context + Opt-In Vocative Addressing** — injects 6–10 deduplicated same-thread `chat_log` entries into `chatBuildRequest` and lets members optionally start messages with `"Xenobot, ..."` / `"Кзенобот, ..."` (avoiding noisy open-mic timers).
- **Silent `✍` React-to-Brief in PM** — tapping `✍` on any logged message sends a private summary with zero group clutter (even on `403`).
- **`/referendum` AI-Drafted Native Polls + Tied-Poll Tie-Breaker** — drafts crisp native polls from recent thread debates and automatically posts one deadpan tie-breaker on closed tied polls (`TotalVoterCount > 0`).
- **Playful `/inspect`** — member-accessible sibling to `/judge` delivering a 2–3 line "Facility Inspection" + matching validated reaction emoji.

**Defer (v1.4+):**
- Group-specific custom persona snippets / inside jokes (validate live ambient room context first).
- Highlights multi-photo album (media-group) bundling (single-message forward/copy covers ~95% cleanly).
- Per-reaction custom emoji allow-list UI editor for Highlights (v1.3's `Positive only / All standard` binary switch fits the 64-byte callback budget cleanly).

### Architecture Approach

The unified architecture reconciles minor naming and storage variations across the 4 research tracks into **one canonical design** (see [ARCHITECTURE.md](ARCHITECTURE.md)):

**Reconciled Canonical Naming & Boundaries:**
- **DB Table & Package:** **`chat_presence`** (`xenobot/db/presence/`, `models.ChatPresence`, `skipLocal` cache prefix `chat_presence:<chat>`) stores all admin-configurable zero-AI settings AND dormant AI sub-toggles (`ai_react_mode`, `ambient_context`, `vocative_enabled`, `pm_brief_react`, `referendum_enabled`, `inspect_enabled`), plus a lightweight durable **`chat_presence_archived (chat_id, message_id)`** unique table so Highlights never re-archive across Redis restarts or >7d viral posts. `chat_conversations` stays 100% pure Owner-only AI consent (`TestChatConversationsStayOutOfBackup` untouched). Per-rule static reaction options (`word_boundary`, `match_caption`, `chance`, `cooldown_secs`) extend the existing `reactions` table directly.
- **Settings UI:** Keep `settingsPageChat` (`p`, `OwnerOnly: true`) intact and add 1 shortcut row (`✨ Presence →`, staying ≤ 8 rows). Register new top-level page **`pr`** (`Order: 91`, `OwnerOnly: false`, `PrivateOnly: true`, `Hidden` in Staff Groups, `settingsRightChangeInfo`) with compact sub-pages (`pa` AI Presence, `pt` Toys & Catch-Up, `ps` Social & Static, `ph` Facility Archive). Page `pr` fills the empty right slot next to `y` on Main, keeping `settingsMainOwnerRows = 17` unchanged.
- **Resolved Backup & Copy Settings Audit:**
  - **`chat_conversations`:** Permanently **EXCLUDED** from `/export`, `/import`, `/reset`, and Copy Settings (Owner 2-tap data consent can never be bypassed).
  - **`reactions` (`BackupModuleReactions`):** Backward-compatibly exports/imports the new per-rule columns; legacy v1.0–v1.2 JSON backups normalize `chance == 0 → 100` on import.
  - **`chat_presence` (`BackupModulePresence = "presence"`):** Exports/imports/resets portable zero-AI knobs AND dormant AI sub-toggles (clamped/validated via apply-layer bounds; AI sub-toggles remain 100% inert until target Owner unlocks `chat_conversations`), while **stripping and never importing** `archive_chat_id` / `archive_thread_id` (mirroring `importLogChannels` / `LogChannelSkipped` with an `ArchiveChannelSkipped` report flag; `archive_enabled` applies only if target already has a `/setarchive`-verified channel).
  - **Copy Settings (`copy_settings.go`):** **EXCLUDES** both `chat_conversations` and `chat_presence` (Copy Settings remains focused on the 6 moderation & event-message areas across 9 tables; admins migrate presence via `/export presence` → `/import presence` or page `ba`).

**Major components:**
1. **Reaction & Poll Update Routers (`presence_reactions.go`, `chat_polls.go` at group `0`):** Single `handlers.NewReaction` dispatcher serving `👎` retraction, Emoji Consensus, Highlights Archive, and `✍` PM brief; `handlers.NewPoll` listener resolving closed tied bot polls via Redis `xenobot:chat:poll:<id>`.
2. **Shared Safe Reaction API (`presence_react_api.go`):** `U+FE0F` normalization, `supportedReactionEmoji ∩ AvailableReactions` (cached 20 min under `xenobot:cache:avail_react:<chat>`), dedicated `reactionPacer` (`MaxWait: 2s`, `MaxRetries: 1`), `IsBig: false`, explicit `[]gotgbot.ReactionType{}` clearing, and 24h `bot_react` metadata (`{emoji, author_id, source}`).
3. **Unified `StopChat` Multi-Lane Runtime (`chat_run.go` + `chat_react.go` / `chat_polls.go` / `chat_inspect.go`):** Adds a bounded `chatPresenceQueue` (2 workers, cap 32) inside `chatEnsureWorkers()` counted in `chatJobs` and derived from `chatContext()`, plus independent hourly budgets (`react:b`, `toy:b`). Background reactions check `chatBreaker(provider).isPaused(chatID)` but **never trip** the shared conversation breaker on parse errors, `ErrNoReply`, Featherless `no choices`, vision `400/422`, or Telegram `400`s.
4. **Message Pipeline Extensions (`reactions.go` group `8`, `chat_watch.go` group `13`, `chat_log.go` group `14`):** Zero new message handler groups; `ctx.Data[presenceStaticReactedKey]` prevents group `13` AI reactions from overwriting group `8` static/cat-sticker reactions; workers filter `chat_log` by `entry.Thread == threadID` and `entry.MessageID < job.messageID` (deduplicated against `chatReadHistory`).

### Critical Pitfalls

Top 5 failure modes to prevent from day one (see [PITFALLS.md](PITFALLS.md) for all 9):

1. **Silent `message_reaction` & `gotgbot/v2` wire failures:** `Reaction: nil` omits `"reaction"` on the wire (must pass `[]gotgbot.ReactionType{}` + assert wire JSON in `httptest`); `MessageReactionUpdated.User` is `nil` for anonymous admins/channels (`ActorChat`); rolling deploys where an old v1.2 replica restarts last overwrite webhook `allowed_updates` and drop `"message_reaction"`.
2. **Staff Group confidentiality & active Lockdown leakage:** Neither `chat_watch.go`, `chat_log.go`, nor `reactions.go` currently checks `staff.IsStaffGroup` or `lockdown.GetActive`. Every v1.3 entry point MUST pass a shared cheap gate (checking cached `staff.GetStaffGroup` first, and checking uncached `lockdown.GetActiveFresh` lazily only when an event is about to fire), and `/setarchive` must refuse Staff Groups as both source and destination.
3. **Reaction toggle-farming, Premium multi-emoji inflation, & cross-replica races:** Plain Redis `INCR` lets one user toggle-farm Consensus/Highlights and lets Premium users triple-count. Always diff `OldReaction` vs `NewReaction` in Redis Lua `SADD`/`SREM` sets of distinct `userID`s, record `author_id` in `bot_react` at reaction time (checking `bot_react` before ever calling `getChatMember` on `👎`), and gate Highlights with a durable DB `(chat_id, message_id)` unique constraint.
4. **Empirical LLM failure modes & shared breaker poisoning:** Never set `MaxTokens < 512` (Gemini 4 thinking tokens and Featherless reasoning prose fail 40–60% at `128`). Explicitly warn in prompts against non-Telegram lookalikes (`😂`, `🐱/😹/😻/😼`), isolate background reactions off the 3-worker `chatQueue`, cache Featherless `text_only` vision fallback on `400/422`, and never let background reaction/parse/400 errors trip `chatBreakers[provider]`.
5. **Owner-consent bypass, Cyrillic `\b` failure, `chat_log` edit corruption, & silent `✍` PM spam:** Double-gate every AI feature at runtime (`conv.Enabled && featureToggle`); use pure Go Unicode rune boundaries (never RE2 `\b`); populate `chatLogEntry.At = msg.Date` (never `time.Now().Unix()`, which corrupts edits); and extract a silent PM delivery path (`silent: true`) for `✍` rather than calling `chatJudgeDeliverPrivate` (which posts public group notices on success and `403`).

---

## Implications for Roadmap

Based on strict dependency flow and vertical slice value, Milestone v1.3 maps directly onto **3 phases** continuing from v1.2's Phase 30:

### Phase 31 (Phase A): Reaction & Presence Foundation + Contextual AI Reactions
**Rationale:** Four features depend on `"message_reaction"`, `chat_presence`, the shared reaction API/`AvailableReactions` cache, `bot_react` author tracking, the `chatPresenceQueue` lane under `StopChat`, and the `pr` Settings page with Owner×Admin gating. Building this substrate first with the flagship AI Reactions + Zero-AI Consensus + `👎` Retraction delivers immediate user-visible value and locks down every security/shutdown invariant before Phases 32–33 plug in.
**Delivers:**
- `"message_reaction"` in `AllowedUpdates`, `chat_presence` migration + `xenobot/db/presence/` + `skipLocal` cache, shared Staff/Lockdown gate.
- `presence_react_api.go` (`AvailableReactions` 20m cache, `reactionPacer`, `[]gotgbot.ReactionType{}` clear, `bot_react` 24h tracker) and `presence_reactions.go` (`handlers.NewReaction` router at group `0`).
- Zero-AI `👎` Retraction (author/admin silent clear + extended user cooldown) and Emoji Consensus (Lua `SADD`/`SREM` distinct-member sets ≥ 3, cooldown + once-only latch).
- Contextual Random AI Emoji Reactions (`chatPresenceQueue` under `StopChat`, group `13` Lua gap/cooldown/roll gate, `json` + `MaxTokens = 512`, `compact_strict` prompt + 73-emoji ∩ `AvailableReactions` filter, `chat_media.go` vision with Featherless fallback, non-tripping error shedding, `/stats` counters).
- Private Settings page `pr` (paired with `y` on Main, 1 shortcut row on page `p` staying ≤ 8 rows, Owner×Admin lock enforcement, 8-locale parity).
**Addresses:** Reaction foundation, anti-annoyance & safety gate, `AvailableReactions` intersection, `👎` retraction, Owner-unlock + Admin-toggle Settings UI, Contextual AI Reactions with vision, Emoji Consensus.
**Avoids:** Pitfalls 1 (`Reaction: nil`, `ActorChat`, `AllowedUpdates`), 2 (Staff/Lockdown leakage), 3 (toggle-farming & `getChatMember` storms), 4 (thinking-token starvation, lookalike emojis, worker/breaker poisoning), 5 (`OwnerOnly` lockout & 8-row cap), 9 (pure fast unit tests for parsers/renders).

### Phase 32 (Phase B): Community Polish, Highlights Archive & Backup Parity
**Rationale:** Builds directly on Phase 31's `chat_presence`, `presence_reactions.go`, `presence_react_api.go`, and page `pr` to deliver all remaining zero-AI community features and establish full Backup/Import/Reset parity before adding AI room-awareness and toys.
**Delivers:**
- Smarter Static Reactions (`reactions.go` group `8`: pure Unicode word boundaries, caption matching, chance %, Redis cooldowns, Staff/Lockdown guards, `ctx.Data[presenceStaticReactedKey]` handoff to group `13`) + backward-compatible `reactions` migration & `BackupModuleReactions` normalization (`chance == 0 → 100`).
- Zero-AI Cat-Sticker Micro-Triggers (`msg.Sticker.Emoji` cat glyphs → occasional `👀`/`❤` ∩ `AvailableReactions`, sets `ctx.Data` handoff).
- Crowd-Picked Highlights ("Facility Archive"): `presence_archive.go`, durable `chat_presence_archived (chat_id, message_id)` dedup table + Redis distinct-reactor set (excluding author & negative emojis), `/setarchive` / `/unsetarchive` 2-step proof modeled on `/setlog` (rejecting Staff Groups), `ForwardMessage` / `CopyMessage` + supergroup-guarded `t.me/c/` jump button, sub-page `ph` on page `pr`.
- `BackupModulePresence = "presence"` in `xenobot/db/backup/` (clamping/validation, stripping `archive_chat_id`/`archive_thread_id` with `ArchiveChannelSkipped`, regression tests proving `chat_conversations` stays excluded and `copy_settings.go` stays untouched).
**Addresses:** Smarter static reactions, cat-sticker micro-triggers, Crowd-Picked Highlights, Backup/Copy parity.
**Avoids:** Pitfalls 2 (Highlights Staff Group leak), 3 (Premium inflation & re-archive loops), 5 (cross-chat archive target hijacking & v1.2 backup breakage), 6 (static vs AI reaction overwrite), 8 (RE2 `\b` Cyrillic bug & non-supergroup `t.me/c/` links).

### Phase 33 (Phase C): Room-Aware Conversations, Frictionless Catch-Up & Group Toys
**Rationale:** Builds on Phase 31's `chatPresenceQueue`, `presence_reactions.go`, and `chat_presence` toggles to make Xenobot-72's existing Conversations and Briefs genuinely room-aware and add the two interactive group toys (`/referendum` + `/inspect`). Keeping all `chat_log` schema extensions (`At: msg.Date`) and prompt additions together in Phase 33 ensures clean thread-isolation and deduplication testing in one pass.
**Delivers:**
- `chatLogEntry.At = msg.Date` (`json:"at,omitempty"`) + `chatLogRecent`, `chatLogSinceUser`, `chatLogSinceTime` helpers with strict `entry.Thread == threadID` filtering and graceful `At == 0` legacy handling.
- Ambient Room Context in Conversations (`chatprompt.UserWithAmbient`, 6–10 same-thread entries deduplicated against `chatReadHistory` and `job.messageID`) + Opt-in Vocative Name-Addressing (`"Xenobot, ..."` / `"Кзенобот, ..."` pure start-of-message Unicode parser skipping forwards).
- Frictionless Catch-Up (`/unrollshort` & `/unrolllong <N|me|2h> [pm]`) + Silent `✍` React-to-Brief in PM (`presence_reactions.go` → `chatBriefQueue` with extracted `chatDeliverPrivate(..., silent: true)` posting zero group messages on success or `403`).
- `/referendum` AI-drafted native polls (`SendPoll` with strict length/`chatSanitizeText` validation, Redis `xenobot:chat:poll:<id>` with TTL > `OpenPeriod`) + `handlers.NewPoll` deadpan once-only tie-breaker (`IsClosed && TotalVoterCount > 0 && topTie >= 2`).
- Playful `/inspect` (member-accessible reply command in `chatPresenceQueue`, text + `chat_media.go` vision, 2–3 line verdict + validated matching reaction emoji recorded in `bot_react` for `👎` retraction).
**Addresses:** Ambient room context, frictionless catch-up, silent `✍` PM brief, vocative addressing, `/referendum` + tie-breaker, `/inspect`.
**Avoids:** Pitfalls 4 (`MaxTokens >= 512` JSON parsing on toys), 6 (`At` edit corruption, forum topic bleed, `chatHistory` double-injection), 7 (`Poll` missing `chat_id`, 0-vote tie, duplicate close, invalid LLM poll options), 8 (vocative false positives & `✍` public PM-blocked spam).

### Phase Ordering Rationale

- **Substrate-first (Phase 31 → 32 & 33):** `"message_reaction"`, `chat_presence`, `presence_react_api.go`, `chatPresenceQueue`, and page `pr` are shared by Phases 32 and 33; shipping them in Phase 31 eliminates parallel schema/handler churn.
- **Zero-AI + Backup lock-in before AI expansion (Phase 32 → 33):** Locking down `reactions.go` `ctx.Data` handoff, durable Highlights deduplication, and `BackupModulePresence` security boundaries in Phase 32 means Phase 33's additional AI toggles inherit tested settings and backup plumbing automatically.
- **CI runtime discipline across all 3 phases:** Because `xenobot/modules` already takes 19–30 min on CI shards under `-race`, every phase keeps pure parsers/matchers/validators/renders in fast millisecond unit tests and avoids Cartesian DB/locale loops or `t.Parallel()` on global seams.

### Research Flags

Phases needing light targeted validation during planning:
- **Phase 33 (`/referendum` & `/inspect` prompts):** Multi-field JSON output (`question` + 2–5 bounded `options`; `verdict` + validated `emoji`) across Claude Haiku, Gemini 4, and Featherless should reuse Spike 001's `MaxTokens >= 512` (consider `768` for `/referendum`) and be sanity-tested during Phase 33 planning/execution so thinking/reasoning tokens do not truncate poll options or emit lookalike emojis.

Phases with standard / already-verified patterns (skip `--research-phase`):
- **Phase 31:** All `gotgbot/v2` wire signatures, Redis Lua patterns, `StopChat` multi-lane workers, `settings_chat` invariants, and 3-provider reaction picker economics were directly verified in-repo and empirically validated in Spikes 001–003.
- **Phase 32:** Directly mirrors established in-repo patterns (`/setlog`, `importLogChannels`, `BackupModuleReactions`, pure Go Unicode rune boundaries).

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| **Stack** | HIGH | Every signature, quirk, and version verified directly against pinned `go.mod` and `go doc -src` on `gotgbot/v2 v2.0.0-rc.36`; zero new external dependencies. |
| **Features** | HIGH | All 9 features locked in `.planning/notes/v1.3-interactive-presence.md`, empirically validated in Spikes 001–003, and audited against `settings_*`, `backup`, and `copy_settings` invariants. |
| **Architecture** | HIGH | Exact file paths, handler groups (`-10..14` + group `0` non-message), `skipLocal` cache keys, `StopChat` shutdown budget (70 s unchanged), and Main grid layout (`y|pr`, 17 rows unchanged) verified directly in source. |
| **Pitfalls** | HIGH | Grounded in wire-level `gotgbot/v2` code (`gen_methods.go:7497`), live multi-provider failure data from Spikes 001–003, and Xenobot-72's CI/shutdown constraints in `AGENTS.md`. |

**Overall confidence:** **HIGH**

### Gaps to Address During Planning

- **Highlights delivery default (`ForwardMessage` vs `CopyMessage`):** Because `gotgbot.ForwardMessageOpts` cannot carry an inline `ReplyMarkup` jump button, confirm in Phase 32 planning whether Highlights defaults to `CopyMessage` (with inline `t.me/c/` button), `ForwardMessage` (preserving author attribution, no button), or exposes a 2-option mode toggle (`forward` / `copy_with_link`) on sub-page `ph`.
- **Anonymous admin (`ActorChat`) reaction policy:** Confirm in Phase 31 planning whether `mru.User == nil && mru.ActorChat.Id == mru.Chat.Id` (anonymous group admin) is allowed to trigger `👎` retraction (recommended by STACK.md) while still skipped for user-scoped `✍` PM briefs and non-member Consensus/Highlights counting.
- **Multi-replica webhook rollout note:** Operational runbook / release notes for v1.3 must state that all v1.2 replicas must terminate before final `getWebhookInfo` verification so an older container cannot overwrite `allowed_updates` and drop `"message_reaction"`.
- **Multi-field JSON token budget for `/referendum` & `/inspect`:** Validate whether `MaxTokens = 512` is sufficient or `768` is safer for longer localized Ukrainian/Russian poll options on Featherless reasoning models during Phase 33.

---

## Sources

### Primary (HIGH confidence)
- **Xenobot-72 Codebase:** `go.mod`, `AGENTS.md`, `xenobot/config/config.go`, `main.go`, `xenobot/utils/httpserver/server.go`, `xenobot/modules/{chat*,reactions,settings_*,copy_settings,backup,logchannels,command_deletion}.go`, `xenobot/db/{chatconv,reactions,staff,lockdown,cache/local,backup/*}`, `xenobot/utils/{claude,gemini,featherless,extraction,ratelimit,chatprompt,tracing}`
- **Pinned `gotgbot/v2` (`v2.0.0-rc.36.0.20260919140833-240296efadb4`) Source (`go doc -src`):** `SetMessageReactionWithContext`, `DeleteMessageReaction`, `GetChatWithContext`, `ChatFullInfo.AvailableReactions`, `MessageReactionUpdated`, `SendPollWithContext`, `Poll`, `ForwardMessageWithContext`, `CopyMessageWithContext`, `handlers.NewReaction`, `handlers.NewPoll`, `ext.NewContext`
- **Empirical Spikes 001–003 (all `VALIDATED` 2026-11-26):** `.planning/spikes/MANIFEST.md` and `.planning/notes/v1.3-interactive-presence.md` (live tests across `claude-haiku-5-5`, `gemini-4-hydrogen`, and Featherless `inclusionAI/Ling-3.2-air`)
- **Synthesized Research Files:** [STACK.md](STACK.md), [FEATURES.md](FEATURES.md), [ARCHITECTURE.md](ARCHITECTURE.md), [PITFALLS.md](PITFALLS.md)

### Secondary (MEDIUM confidence)
- **Telegram Bot API & Community Bot UX patterns:** Cross-checked Bot API documentation (`allowed_updates`, bot admin requirement for `message_reaction`, 1-reaction bot cap, `sendPoll` bounds) and community starboard/reaction anti-spam conventions.

---
*Research completed: 2026-11-26*
*Ready for roadmap: yes*
