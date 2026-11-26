# Stack Research — Milestone v1.3 Interactive Presence

**Domain:** Telegram Group-Management Bot — Interactive Presence (Go 1.26, gotgbot v2, PostgreSQL/GORM, Redis)
**Researched:** 2026-11-26
**Confidence:** HIGH

## Executive Verdict

**Zero new external Go dependencies are required for Milestone v1.3.** Every Telegram Bot API method, update type, handler filter, Redis primitive, PostgreSQL migration pattern, Unicode boundary helper, and multimodal AI leaf client needed for all 9 v1.3 features is already present in `go.mod` and verified against the pinned module versions via `go doc`.

The real stack work for v1.3 is **enabling existing surfaces cleanly and respecting verified API/library edge cases**:
1. Adding `"message_reaction"` to `config.AppConfig.AllowedUpdates` (while reusing `"poll"`, which is already enabled).
2. Navigating verified `gotgbot/v2` serialization/unmarshaling quirks (`SetMessageReaction` nil vs empty slice, `ChatFullInfo.AvailableReactions` nil collapse, `Update.Poll` lacking `ChatId`/`MessageId`, `ForwardMessageOpts` lacking `ReplyMarkup`).
3. Using Go stdlib `unicode` + `unicode/utf8` for Cyrillic-safe word boundaries and vocative parsing (never Go RE2 `\b`).
4. Partitioning PostgreSQL schema cleanly between Owner-unlocked AI settings (`chat_conversations`, permanently excluded from Backup/Copy Settings) and Admin-configurable zero-AI settings (`chat_interactivity_settings` + extended `reactions`), with every new Redis-backed cache prefix registered in `skipLocal`.

---

## Recommended Stack (New v1.3 Additions & Surfaces)

### Core Technologies (Pinned in `go.mod` — No Version Bumps Needed)

| Technology | Version (Verified in `go.mod`) | New v1.3 Purpose | Why Recommended |
|------------|--------------------------------|------------------|-----------------|
| **Go stdlib** | `1.26.0` | `encoding/json` (reaction picker / `/referendum` structured JSON parsing), `math/rand/v2` (chance rolls & sampling), `unicode` + `unicode/utf8` (Cyrillic-safe word boundaries & vocative prefixes) | Zero dependencies, lock-free auto-seeded ChaCha8 PRNG (`math/rand/v2`, already used in `staff_sweeper.go`), ReDoS-free Unicode word boundaries across all 8 UI languages (`uk`, `en`, `ru`, `hi`, etc.). |
| **gotgbot/v2** | `v2.0.0-rc.36.0.20260919140833-240296efadb4` | `SetMessageReactionWithContext`, `GetChatWithContext` (`ChatFullInfo.AvailableReactions`), `handlers.NewReaction` (`MessageReactionUpdated`), `SendPollWithContext` + `handlers.NewPoll`, `ForwardMessageWithContext` / `CopyMessageWithContext`, `Sticker.Emoji`, `Message.Quote` | Already pinned to a recent Bot API 9.x build containing all reaction, poll (`InputPollOption`, `PollOption.PersistentId`, `ReplyParameters.PollOptionId`), and forum-topic (`MessageThreadId`) APIs. |
| **go-redis/v9** + **miniredis/v2** | `v9.22.0` / `v2.39.0` | Cross-replica reaction throttles, bot-reacted message registry (with author ID), per-emoji reactor sets (`SADD`/`SREM`/`SCARD`), `AvailableReactions` cache, poll metadata mapping, extended `chat_log` ZSET (`At int64`), atomic dedup latches (`SetNX`) | Matches existing `xenobot:chat:{<chat>}:*` hash-tag layout outside `xenobot:cache:` (survives `CLEAR_CACHE_ON_STARTUP`); all chosen primitives work identically on production Redis and `miniredis/v2` in `make test`. |
| **GORM + PostgreSQL / SQLite** | `gorm v1.31.2`, `postgres v1.6.3`, `sqlite v1.6.0` | Append-only SQL migrations (`> 20261126120000_allow_featherless_chat_provider.sql`) extending `chat_conversations` and `reactions`, plus new `chat_interactivity_settings` table | Follows established idempotent `ADD COLUMN IF NOT EXISTS` with inline `CHECK` constraints, `skipLocal` cache invalidation, and dual PostgreSQL/SQLite test parity. |
| **Existing Leaf AI Clients** (`xenobot/utils/{claude,gemini,featherless}`) | In-repo (stdlib-only) | Contextual AI emoji reaction picker (with vision via `chat_media.go`), ambient room context, `/unrollshort` extensions + `✍` PM brief, `/referendum` poll drafting, `/inspect` | Validated empirically across Spikes 001–003 with **zero client code changes** needed when using prompt-instructed JSON + `MaxTokens: 512`. |

### Supporting In-Repo Packages to Reuse (Do Not Duplicate)

| In-Repo Package / Symbol | Purpose in v1.3 | When to Use |
|--------------------------|-----------------|-------------|
| `xenobot/modules/chat_media.go` (`chatExtractMediaRef`, `chatDownloadMedia`, `chatMediaLabel`) | Photo and static/animated/video sticker thumbnail download with 5 MB cap, no redirects, and MIME sniffing | Contextual AI emoji reactions and `/inspect` on vision-capable providers (Anthropic, Google, vision Featherless models). |
| `xenobot/modules/chat_run.go` (`chatJobs`, `chatContext`, `chatReserveWith`, `chatBreaker`, `StopChat`) | Worker lifecycle, graceful shutdown, per-feature hourly budgets, and provider circuit breakers | Every new v1.3 AI worker/job MUST join `StopChat` (`chatJobs` + `chatContext`) — zero new shutdown drains, preserving the 70 s webhook worst case and `stop_grace_period: 75s`. |
| `xenobot/modules/chat_brief.go` (`chatJudgeDeliverPrivate`, `chatBriefQueue`) | Private-message delivery with 403 handling + HTML splitting | `/unrollshort ... pm` and silent `✍` react-to-brief in PM. |
| `xenobot/utils/extraction` (`ReplyTarget`, `ParseDurationToken`) | Replied-message resolution and `<digits><m\|h\|d\|w>` duration parsing | Reaction reply context (`ReplyTarget`) and `/unrollshort <duration>` (`ParseDurationToken` matches `/tban`/`/tmute` grammar). |
| `xenobot/utils/ratelimit` (`TelegramPacer`) | Fleet-wide Redis-backed Telegram pacing with 429 `retry_after` backoff | Use existing `chatPacer` for `/referendum`, `/inspect`, `✍` briefs; use a dedicated `reactionPacer` (`MaxWait: 2s`, `MaxRetries: 1`) for best-effort reactions and highlights so bursts never stall replies. |
| `xenobot/modules/reactions.go` (`supportedReactionEmoji`, `normaliseReactionEmoji`) | Canonical 73 standard Telegram reaction emojis with `U+FE0F` stripping | Validation boundary for every AI reaction, Emoji Consensus trigger, static reaction, and `AvailableReactions` intersection. |
| `xenobot/db/staff` (`IsStaffGroup`) & `xenobot/db/lockdown` (`GetActive`) | Explicit ambient-feature safety gates | Neither `chat_watch.go`, `chat_log.go`, nor `reactions.go` currently checks these; every v1.3 ambient feature MUST check both before acting. |

---

## Verified `gotgbot/v2` API Surfaces & Quirks

All signatures and behaviors below were verified directly against `github.com/PaulSonOfLars/gotgbot/v2 v2.0.0-rc.36.0.20260919140833-240296efadb4` via `/opt/homebrew/bin/go doc -src`.

### 1. Update Subscriptions (`xenobot/config/config.go`)

- **Add `"message_reaction"`** to `cfg.AllowedUpdates` (`config.go:546-561`). Both webhook registration (`xenobot/utils/httpserver/server.go:235`) and polling startup (`main.go:355`) already pass `config.AppConfig.AllowedUpdates` verbatim.
- **`"poll"` is already subscribed** (`config.go:556`) — no config change required for the `/referendum` tie-breaker.
- **Telegram Bot API requirement** (verified on `gotgbot.Update.MessageReaction`): the bot MUST be an administrator in the chat to receive `message_reaction` updates, and Telegram **never** sends `message_reaction` updates for reactions set by bots.

### 2. Observing Reactions (`handlers.NewReaction`, `MessageReactionUpdated`)

```go
// Package github.com/PaulSonOfLars/gotgbot/v2/ext/handlers
handlers.NewReaction(reaction.All, responseFunc)
```

- **`ext.Context` population** (verified in `ext.NewContext`):
  - `ctx.EffectiveMessage` is **`nil`** (Telegram sends no message body in `MessageReactionUpdated` and has no `getMessage` API).
  - `ctx.EffectiveChat` = `&update.MessageReaction.Chat`
  - `ctx.EffectiveUser` = `update.MessageReaction.User` (**`nil`** when an anonymous admin/channel reacts via `ActorChat`)
  - `ctx.EffectiveSender` = `update.MessageReaction.GetSender()` (`Sender{User: u.User, Chat: u.ActorChat, ChatId: u.Chat.Id}`)
- **Reaction diffing**: `MessageReactionUpdated` carries `OldReaction []gotgbot.ReactionType` and `NewReaction []gotgbot.ReactionType`. Extract standard emojis via `rt.MergeReactionType()` checking `Type == "emoji"` (or type switch on `gotgbot.ReactionTypeEmoji`), normalizing each with `normaliseReactionEmoji` (strip `U+FE0F`). Ignore `ReactionTypeCustomEmoji`, `ReactionTypePaid`, and `ReactionTypeUnknown`.
- **Anonymous admin handling**:
  - When `u.User == nil && u.ActorChat != nil && u.ActorChat.Id == u.Chat.Id`, the reactor is an **anonymous admin of the group** — allow `👎` retraction, skip user-scoped PM actions (`✍` brief), and skip non-member counting in Consensus/Highlights.
  - When `u.User != nil && u.User.IsBot`, ignore completely.

### 3. Setting & Clearing Reactions (`SetMessageReactionWithContext`)

```go
func (bot *Bot) SetMessageReactionWithContext(
    ctx context.Context,
    chatId int64,
    messageId int64,
    opts *SetMessageReactionOpts,
) (bool, error)
```

- **Setting one reaction**: `&gotgbot.SetMessageReactionOpts{Reaction: []gotgbot.ReactionType{gotgbot.ReactionTypeEmoji{Emoji: emoji}}, IsBig: false}`. Bots (non-premium) can set at most 1 reaction per message.
- **CRITICAL gotgbot serialization quirk (Clearing / `👎` Retraction)**:
  - Verified in `SetMessageReactionWithContext` source: `addIfValueNotZero(v, "reaction", opts.Reaction, opts.Reaction == nil)`.
  - Passing `Reaction: nil` **omits** the `"reaction"` key from the JSON payload. Pass an **explicit empty non-nil slice** `Reaction: []gotgbot.ReactionType{}` so gotgbot always serializes `"reaction": []`, which unconditionally removes the bot's reaction.
- **Error classification**: Telegram `400` errors (`REACTION_INVALID`, `MESSAGE_ID_INVALID`, `REACTIONS_TOO_MANY`, `CHAT_ADMIN_REQUIRED`) are expected on deleted messages or restricted chats — swallow silently and **never** record them against AI provider circuit breakers (`chatBreaker`).

### 4. Per-Chat Allowed Reactions (`GetChatWithContext` & `ChatFullInfo.AvailableReactions`)

```go
info, err := b.GetChatWithContext(ctx, chatID, nil)
// info.AvailableReactions []gotgbot.ReactionType
// info.MaxReactionCount   int64
// info.HasProtectedContent bool
```

- **CRITICAL gotgbot unmarshaling quirk** (verified in `unmarshalReactionTypeArray` source):
  - When Telegram omits `available_reactions` (meaning **all 73 standard emoji reactions are allowed**), `len(d) == 0` returns `nil, nil`.
  - When Telegram returns `"available_reactions": []`, the loop runs 0 times on `var vs []ReactionType` and **also returns `nil, nil`**.
  - When a chat allows **only custom emojis**, `info.AvailableReactions` is non-empty (`len > 0`), containing only `ReactionTypeCustomEmoji` items — intersecting with `supportedReactionEmoji` yields `0` standard emojis.
- **Handling rule**:
  - `len(info.AvailableReactions) == 0`: treat as all 73 standard emojis allowed (and if `SetMessageReaction` later returns `400 REACTION_INVALID`, silently drop as `NONE` without tripping AI breakers).
  - `len(info.AvailableReactions) > 0`: intersect its `ReactionTypeEmoji` entries (normalized via `normaliseReactionEmoji`) with `supportedReactionEmoji`. If the intersection is empty, cache an explicit `"none"` sentinel in Redis (15–30 min TTL) so the bot skips AI reaction calls completely for that chat.

### 5. Crowd-Picked Highlights (`ForwardMessageWithContext` vs `CopyMessageWithContext`)

| Method | Signature Highlights | Forum Topics | Inline Jump Button (`t.me/c/`) | Author Attribution |
|--------|---------------------|--------------|-------------------------------|--------------------|
| `ForwardMessageWithContext` | `(ctx, destChatID, fromChatID, msgID, *ForwardMessageOpts)` | `MessageThreadId` supported | **NO** — `ForwardMessageOpts` has no `ReplyMarkup` field | Preserved ("Forwarded from ...") |
| `CopyMessageWithContext` | `(ctx, destChatID, fromChatID, msgID, *CopyMessageOpts)` | `MessageThreadId` supported | **YES** — `CopyMessageOpts.ReplyMarkup` attaches an inline `URL` button in one API call | Omitted (respects hidden forwards) |

- **Jump URL builder**: Because `MessageReactionUpdated` carries `Chat` + `MessageId` (not `*gotgbot.Message`), build the jump URL with the same logic as `gotgbot.Message.GetLink()`:
  - Public supergroup (`chat.Username != ""`): `https://t.me/<username>/<msgID>`
  - Private supergroup (`strings.HasPrefix(idStr, "-100")`): `https://t.me/c/<strippedID>/<msgID>`
  - Basic `group` chat (`chat.Type == "group"`): omit jump button (no `t.me/c/` link exists).
- **Protected content**: If the source group has `HasProtectedContent == true`, Telegram returns `400` on both `ForwardMessage` and `CopyMessage`. Treat `400` as permanent non-retryable and latch the message so it does not retry on further reactions.

### 6. Native Polls & Tied-Poll Tie-Breaker (`SendPollWithContext` & `handlers.NewPoll`)

```go
// Send native poll
msg, err := b.SendPollWithContext(ctx, chatID, question, options, &gotgbot.SendPollOpts{
    MessageThreadId: threadID,
    IsAnonymous:     &isAnon, // *bool; Telegram defaults to true when omitted
    Type:            "regular",
    OpenPeriod:      openPeriodSecs, // 5..2628000 s (optional auto-close)
    ReplyParameters: &gotgbot.ReplyParameters{MessageId: replyMsgID, AllowSendingWithoutReply: true},
})

// Observe poll state updates (already in AllowedUpdates)
handlers.NewPoll(poll.All, pollHandler)
```

- **Hard Telegram API limits** (verified in `SendPoll` / `InputPollOption` docs):
  - `question`: 1–300 UTF-16/rune characters (truncate/validate AI output strictly before calling Telegram).
  - `options`: 2–12 `gotgbot.InputPollOption{Text}` entries (recommend 2–5 for `/referendum`), each 1–100 characters.
  - `OpenPeriod`: 5 to 2,628,000 seconds (~30 days); mutually exclusive with `CloseDate`.
- **CRITICAL Bot API constraint on `Update.Poll`**:
  - `gotgbot.Poll` contains `Id`, `Question`, `Options []PollOption` (`PersistentId`, `Text`, `VoterCount`), `TotalVoterCount`, and `IsClosed` — it has **NO `ChatId`, `MessageId`, or `MessageThreadId`**, and `ext.NewContext` leaves `EffectiveChat`, `EffectiveMessage`, and `EffectiveUser` all `nil`!
  - Therefore, `/referendum` MUST store `msg.Poll.Id -> {chat_id, msg_id, thread_id, lang}` in Redis immediately upon `SendPollWithContext` success.
  - When `handlers.NewPoll` receives `p.IsClosed == true && p.TotalVoterCount > 0` and 2+ options share the maximum `VoterCount`, claim a one-shot Redis latch and post the deadpan 1-line tie-breaker replying to `msg_id` (optionally setting `ReplyParameters.PollOptionId = winner.PersistentId` when non-empty).

---

## Redis Data Structures & Key Layout

All v1.3 transient state uses `xenobot:chat:{<chat>}:*` (or `xenobot:rx:{<chat>}:*`) **outside** the `xenobot:cache:` prefix so `CLEAR_CACHE_ON_STARTUP` preserves active cooldowns, logs, bot-reaction tracking, and pending polls, while chat-ID hash tags `{<chat>}` keep multi-key Lua scripts Cluster-safe.

| Feature | Key Pattern | Redis Type | TTL | Purpose & Atomicity Notes |
|---------|-------------|------------|-----|---------------------------|
| **Message log (extended)** | `xenobot:chat:{<chat>}:log` | `ZSET` (existing) | 7 d (sliding) | Add `At int64 \`json:"at,omitempty"\`` (`msg.Date`) to `chatLogEntry` for `/unrollshort <duration>`. Backward-compatible with pre-v1.3 entries (`At == 0`). |
| **Chat `AvailableReactions` cache** | `xenobot:chat:{<chat>}:avail_rx` | `STRING` | 20 min | Stores `"*"` (all 73 allowed), `"-"` (0 standard emojis allowed), or JSON array of allowed standard emojis. Avoids repeated `getChat` calls. |
| **AI reaction message-gap counter** | `xenobot:chat:{<chat>}:rx:gap` | `STRING` (int) | 24 h | `INCR` per eligible group message; atomic Lua script checks `gap >= minGap` and resets on pass. |
| **AI reaction cooldowns** | `xenobot:chat:{<chat>}:rx:cd` / `...:rx:ucd:<user>` | `STRING` (`"1"`) | Preset (e.g., 3–10 m chat; 10–30 m user) | `SetNX` claim. `👎` retraction overwrites `rx:ucd:<user>` with extended cooldown (e.g., 2–6 h). |
| **AI reaction hourly budget** | `xenobot:chat:rx:b:{u\|c\|g}:...` | `STRING` (int) | 2 h | Reuses existing `chatReserveWith(client, chatID, userID, "rx:b", ...)` without inventing a second budget engine. |
| **Bot-reacted message registry** | `xenobot:chat:{<chat>}:rx:bot:<msgID>` | `STRING` (JSON) | 24 h | Stores `{"a": <authorUserID>, "e": "<emoji>"}`. **Essential for `👎` retraction**: `MessageReactionUpdated` does NOT include the original message author, so recording `authorUserID` at reaction time lets `👎` verify "author or admin" even when `LogEnabled` is off. |
| **Emoji Consensus per-emoji sets** | `xenobot:chat:{<chat>}:rx:u:<msgID>:<emoji>` | `SET` (userIDs) | 48 h | On `MessageReactionUpdated`: `SADD` for `New \ Old`, `SREM` for `Old \ New`, renew TTL; check `SCARD >= threshold`. One-shot latch `...:rx:cons:<msgID>` (`SetNX`, 24 h) + chat cooldown prevent double-joins across replicas. |
| **Highlights distinct-reactor set** | `xenobot:chat:{<chat>}:hl:u:<msgID>` | `SET` (userIDs) | 48 h | Idempotent update from `MessageReactionUpdated`: if `len(NewReaction) > 0` → `SADD userID`; if `len(NewReaction) == 0` → `SREM userID`. One-shot latch `...:hl:sent:<msgID>` (`SetNX`, 7 d) guarantees at-most-once archiving. |
| **Static reaction cooldowns** | `xenobot:rx:{<chat>}:cd` / `...:rcd:<ruleID>` | `STRING` (`"1"`) | Per-rule/chat secs | `SetNX` cross-replica cooldowns for smarter static reactions and cat-sticker micro-triggers. |
| **Static→AI same-update handoff** | `ctx.Data["xenobot_static_reacted"] = true` | In-memory (`ext.Context.Data`) | Single update | Zero Redis I/O: group `8` (`reactions.go`) marks `ctx.Data`, group `13+` (contextual AI reactions) skips messages already reacted to statically. |
| **`✍` PM brief dedup** | `xenobot:chat:{<chat>}:brief:rx:<msgID>:<userID>` | `STRING` (`"1"`) | 5 min | `SetNX` prevents toggling `✍` on/off from firing duplicate briefs; shares `chatBriefCooldownKey` and `brief:b` hourly budget. |
| **`/referendum` poll registry & tie-breaker latch** | `xenobot:chat:poll:<pollID>` & `xenobot:chat:poll:tb:<pollID>` | `STRING` (JSON / `"1"`) | 7 d / 24 h | Maps `poll.Id` → `{chat_id, msg_id, thread_id, lang}` (since `Update.Poll` omits chat/message IDs); `tb` latch guarantees exactly one tie-breaker post across replicas. |

---

## PostgreSQL Schema Changes & Migration Strategy

All new migrations MUST be append-only timestamped SQL files sorting strictly after `20261126120000_allow_featherless_chat_provider.sql`, single-transaction, idempotent (`CREATE TABLE IF NOT EXISTS` / `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` with inline `CHECK` constraints, never bare `ADD CONSTRAINT` without drop), and paired with dual SQLite/PostgreSQL GORM models and migration unit tests (`xenobot/db/migrations/*_test.go`).

### 1. Extend `chat_conversations` (Owner-Unlocked AI + Admin-Toggleable AI Features)

Keep AI feature toggles on `chat_conversations` (read through the existing `chat_conv:<chat>` cache key, already in `skipLocal`, with msgpack-tagged fields on `chatconv.Settings` so older cached entries decode safely to zero/off):

- `ai_reactions_mode SMALLINT NOT NULL DEFAULT 0` (`CHECK (ai_reactions_mode BETWEEN 0 AND 2)`: `0=Off, 1=Low, 2=Medium`)
- `ambient_context BOOLEAN NOT NULL DEFAULT FALSE`
- `vocative_enabled BOOLEAN NOT NULL DEFAULT FALSE`
- `referendum_enabled BOOLEAN NOT NULL DEFAULT FALSE`
- `inspect_enabled BOOLEAN NOT NULL DEFAULT FALSE`
- `pm_brief_react_enabled BOOLEAN NOT NULL DEFAULT FALSE`

**Why on `chat_conversations`?**
- `TestChatConversationsStayOutOfBackup` (`xenobot/db/backup/chatconv_backup_test.go`) enforces that `chat_conversations` is **never** exported, imported, reset, or copied by Backup/Copy Settings — preventing an imported backup from ever activating AI data sharing without the group Owner's explicit 2-tap data notice.
- Permission split stays clean in the Settings menu (`settings_chat.go` page `p` is at 7/8 rows; link row 8 to a new private sub-page): Owner-only controls (`enabled`, `provider`, `audience`, `log_enabled`, `judge_to`) stay `settingsRightOwner`; once Owner enables AI (`enabled` / `log_enabled`), group admins (`settingsRightChangeInfo` or `settingsRightAdmin`) can toggle the interactive AI sub-controls.

### 2. New Table `chat_interactivity_settings` (Zero-AI Features — Admin-Configurable Independently)

Create a dedicated table for zero-AI community features so they work even when `ENABLE_CHAT=false` or Owner never unlocked AI:

- `chat_id BIGINT PRIMARY KEY` (no FK on `chats`, matching `chat_conversations` and `chat_advanced_thresholds`)
- `consensus_enabled BOOLEAN NOT NULL DEFAULT FALSE`
- `consensus_threshold SMALLINT NOT NULL DEFAULT 3 CHECK (consensus_threshold BETWEEN 2 AND 20)`
- `retract_enabled BOOLEAN NOT NULL DEFAULT TRUE`
- `cat_stickers_enabled BOOLEAN NOT NULL DEFAULT FALSE`
- `highlights_enabled BOOLEAN NOT NULL DEFAULT FALSE`
- `highlights_target_chat_id BIGINT NOT NULL DEFAULT 0`
- `highlights_target_thread_id BIGINT NOT NULL DEFAULT 0`
- `highlights_threshold SMALLINT NOT NULL DEFAULT 5 CHECK (highlights_threshold BETWEEN 2 AND 100)`
- `highlights_mode SMALLINT NOT NULL DEFAULT 0 CHECK (highlights_mode BETWEEN 0 AND 1)` (`0=Forward, 1=CopyWithJumpButton`)
- `updated_by BIGINT NOT NULL DEFAULT 0`, `updated_at TIMESTAMPTZ DEFAULT NOW()`

**Cache & Backup/Copy parity**:
- Register `chatInteractivityPrefix = CacheKey("chat_interactivity") + ":"` in `skipLocal` (`xenobot/db/cache/local.go`) so admin toggles act immediately across replicas.
- If included in Backup (`/export`, `/import`) or Copy Settings: copy/import the safe boolean & threshold knobs, **and explicitly strip/zero `highlights_target_chat_id` and `highlights_target_thread_id` (and force `highlights_enabled = false`) on cross-chat import/copy** — mirroring how `log_channels` and `staff_groups` prevent redirecting one group's output into another community's channel.

### 3. Extend `reactions` Table (Smarter Static Reactions)

Add backward-compatible columns to `reactions`:
- `word_boundary BOOLEAN NOT NULL DEFAULT FALSE`
- `match_caption BOOLEAN NOT NULL DEFAULT FALSE`
- `chance SMALLINT NOT NULL DEFAULT 100 CHECK (chance BETWEEN 1 AND 100)`
- `cooldown_secs INTEGER NOT NULL DEFAULT 0 CHECK (cooldown_secs BETWEEN 0 AND 86400)`

**Backup compatibility**: `BackupModuleReactions` serializes `[]models.Reactions`. On `importReactions`, normalize `Chance == 0` (from pre-v1.3 exports) to `100` and clamp/validate `CooldownSecs` so existing v1.0–v1.2 backup files import cleanly with zero breaking changes.

---

## AI Runtime & Prompt-Stack Constraints (Empirically Validated in Spikes 001–003)

| Constraint | Verified Setting | Why (Empirical Evidence from Live Spikes) |
|------------|------------------|-------------------------------------------|
| **Output token floor (`MaxTokens`)** | **`512`** (never 32–128) | `gemini-4-hydrogen` cannot disable thinking (`thinkingLevel: "low"` consumes ~85–87 thinking tokens) and Featherless models (`Ling-3.2-air` / Qwen3) emit reasoning prose. `128` failed 40–60% with `ErrNoReply`; `384` still truncated on Featherless; **`512`** achieved 0% truncation across Claude Haiku 5.5, Gemini 4, and Featherless. |
| **Structured wire format** | Prompt-instructed JSON (`{"emoji":"..."}` / `{"emoji":"NONE"}`) parsed with `encoding/json` | Eliminated Claude Haiku plain-text scratchpad leakage and dropped Featherless `256`-token failures from 40% to 0%. Works identically across all 3 existing leaf clients without adding provider-specific `response_format` schema plumbing. |
| **Non-Telegram emoji guard** | Prompt warning + strict `supportedReactionEmoji ∩ AvailableReactions` post-filter (`U+FE0F` stripped) | Claude repeatedly emitted `😂`/`😹`/`😻` and Featherless echoed `😼` on cat/funny probes — all invalid Telegram reactions (`REACTION_INVALID`). Always map invalid output to `NONE` silently without tripping breakers. |
| **Vision attachment path** | Reuse `chatExtractMediaRef` + `chatDownloadMedia` (`chat_media.go`) | Claude Haiku averaged +79 image input tokens (~0.70 s); Gemini 4 averaged +1,056 tokens (~2.34 s, 6/6 accuracy); Featherless accepted `image_url` (+61 tok) with occasional `no choices` — always fall back to text/caption/`msg.Sticker.Emoji` and shed parse/provider errors silently as `NONE`. |

---

## Installation

```bash
# Zero new dependencies — no `go get` required.
# Verify existing pinned modules and build:
CGO_ENABLED=0 /opt/homebrew/bin/go build ./...
```

---

## Alternatives Considered

| Category | Recommended | Alternative | When to Use Alternative |
|----------|-------------|-------------|-------------------------|
| **Word-boundary & vocative matching** | Go stdlib `unicode.IsLetter`/`IsDigit` + `utf8.DecodeRuneInString` | Go `regexp` (`\b`) or third-party `dlclark/regexp2` | Never use Go `regexp` `\b` for Ukrainian/Russian text (ASCII-only). Only consider `regexp2` if full PCRE lookarounds were ever needed (unnecessary here). |
| **Highlights delivery** | Configurable `ForwardMessage` (default) OR `CopyMessage` with `InlineKeyboardMarkup` jump button | `ForwardMessage` + second reply message carrying jump button | Two-message approach adds visual clutter and doubles Telegram API calls per highlight; offering a `highlights_mode` toggle (`forward` vs `copy_with_link`) cleanly exposes Telegram's real API tradeoff. |
| **Reaction picker structured output** | Prompt-instructed JSON + `MaxTokens: 512` on existing leaf clients | Provider-native `response_format` / `tool_choice` / `responseSchema` | Only needed if complex multi-field nested schemas fail to parse; Spikes 001–003 proved prompt-instructed JSON is 100% reliable at `MaxTokens: 512` for simple schemas (`{"emoji":"..."}`, `/referendum` question+options). |
| **Reaction Telegram pacing** | Dedicated `reactionPacer` (`MaxWait: 2s`, `MaxRetries: 1`) for fire-and-forget reactions; existing `chatPacer` for replies/polls/briefs | Sharing `chatPacer` for everything | Sharing `chatPacer` is fine for low traffic, but a dedicated `reactionPacer` ensures a burst of Emoji Consensus / static reactions never delays an interactive `/ask` or `/unrollshort` reply. |
| **AI interactive feature storage** | Columns on `chat_conversations` (excluded from Backup/Copy) + separate `chat_interactivity_settings` table for zero-AI features | One combined table for all v1.3 features | A single table would either drag Owner-gated AI settings into Backup/Copy Settings (violating `TestChatConversationsStayOutOfBackup`) or keep zero-AI settings out of Backup unnecessarily. |

---

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| **Go `regexp` `\b` word boundaries** | RE2 defines `\w` strictly as ASCII `[0-9A-Za-z_]`; `\bкіт\b` or `\bКзенобот\b` **silently fails to match Cyrillic** in Ukrainian/Russian groups. | Rune boundary helper using `unicode.IsLetter(r) \|\| unicode.IsDigit(r)` with `utf8.DecodeLastRuneInString` / `utf8.DecodeRuneInString`. |
| **`"message_reaction_count"` in `AllowedUpdates`** | Fires only for anonymous channel reactions, batched with up to several minutes delay, and provides no user IDs. | `"message_reaction"` (`MessageReactionUpdated`) aggregated per user ID in Redis Sets. |
| **`Bot.DeleteMessageReaction` for `👎` retraction** | Removes *another user's* reaction and requires `can_delete_messages` admin right. | `Bot.SetMessageReactionWithContext` with `Reaction: []gotgbot.ReactionType{}` (clears the bot's own reaction with no admin right required). |
| **`SetMessageReactionOpts{Reaction: nil}` for retraction** | `gotgbot/v2` omits `"reaction"` from JSON when `opts.Reaction == nil`. | Explicit empty non-nil slice: `Reaction: []gotgbot.ReactionType{}`. |
| **`MaxTokens` < `512` for AI reaction picker / `/referendum`** | `gemini-4-hydrogen` (`thinkingLevel: "low"`) and Featherless reasoning prose exhaust small budgets and return `ErrNoReply` (40–60% failure at `128`). | `MaxTokens: 512` with JSON output format (validated in Spike 001). |
| **New external AI SDKs or NLP/emoji libraries** | Bloats binary, breaks stdlib-only leaf client contract (tested by `leaf_test.go`), and duplicates working code. | Existing `xenobot/utils/{claude,gemini,featherless}` + `supportedReactionEmoji` + Go stdlib. |
| **New shutdown drain registrations in `main.go`** | Webhook-mode worst-case shutdown budget is already 70 s against the 60 s enforced budget (`stop_grace_period: 75s`). | Reuse `StopChat` (`chatJobs` + `chatContext`) for all v1.3 async AI workers; zero new drains. |
| **`math/rand` (v1)** | Uses legacy global mutex-locked source; Go 1.26 codebase already standardized on `math/rand/v2`. | `math/rand/v2` (`rand.Float64()`, `rand.IntN()`). |
| **Putting `chat_conversations` in Backup / Copy Settings** | Violates `TestChatConversationsStayOutOfBackup` and could silently activate paid AI + third-party data disclosure without Owner consent. | Keep `chat_conversations` excluded; only back up/copy safe zero-AI fields (with target chat/topic IDs zeroed on cross-chat import/copy). |

---

## Version Compatibility

| Component A | Compatible With | Verified Notes |
|-------------|-----------------|----------------|
| `gotgbot/v2 v2.0.0-rc.36.0.20260919140833-240296efadb4` | Go `1.26.0` | Includes `MessageReactionUpdated`, `ReactionType{Emoji,CustomEmoji,Paid,Unknown}`, `ChatFullInfo.{AvailableReactions,MaxReactionCount,HasProtectedContent}`, `SendPoll` (`InputPollOption`, `PollOption.PersistentId`, `ReplyParameters.PollOptionId`), `handlers.NewReaction`, `handlers.NewPoll`. |
| `go-redis/v9 v9.22.0` | `miniredis/v2 v2.39.0` | All required commands (`SADD`, `SREM`, `SCARD`, `ZADD`, `ZRANGEBYSCORE`, `ZREMRANGEBYRANK`, `SET NX EX`, `INCR`, `EXPIRE`, Lua `redis.NewScript`) are supported in `miniredis/v2` for race-tested CI runs (`make test`). |
| `gorm.io/gorm v1.31.2` | `postgres v1.6.3` & `sqlite v1.6.0` | New boolean columns on GORM structs MUST omit `default:` tags (following `models.ChatConversation` convention) so Go `false` is always persisted explicitly on `Create`/`Save`. |
| `chatLogEntry` (`At int64 \`json:"at,omitempty"\``) | Existing `xenobot:chat:{<chat>}:log` entries | `omitempty` ensures rolling deploys read pre-v1.3 entries cleanly (`At == 0`) without flushing Redis logs. |
| `chatconv.Settings` (msgpack) | Existing `chat_conv:<chat>` Redis cache entries | New msgpack-tagged fields with short keys (e.g., `rx`, `ac`, `vo`, `rf`, `in`, `pb`) decode to zero/off on cache entries written by v1.2 replicas during rolling deploys. |

---

## Sources

- **Pinned repository manifest**: `go.mod` (`go 1.26.0`, `gotgbot/v2 v2.0.0-rc.36.0.20260919140833-240296efadb4`, `go-redis/v9 v9.22.0`, `gorm v1.31.2`) — **HIGH confidence** (verified locally).
- **`gotgbot/v2` source & API docs**: `/opt/homebrew/bin/go doc -src` on `Bot.SetMessageReactionWithContext`, `Bot.DeleteMessageReaction`, `Bot.GetChatWithContext`, `ChatFullInfo.UnmarshalJSON`, `unmarshalReactionTypeArray`, `Bot.SendPollWithContext`, `Poll`, `PollOption`, `InputPollOption`, `Bot.ForwardMessageWithContext`, `Bot.CopyMessageWithContext`, `ReplyParameters`, `Sticker`, `Message.Quote`, `Message.GetLink`, `ext.NewContext`, `handlers.NewReaction`, `handlers.NewPoll` — **HIGH confidence** (verified directly against exact module source).
- **Xenobot-72 production codebase**: `xenobot/config/config.go`, `xenobot/modules/reactions.go`, `xenobot/modules/chat*.go`, `xenobot/modules/settings_chat.go`, `xenobot/modules/copy_settings.go`, `xenobot/db/chatconv/repository.go`, `xenobot/db/reactions/repository.go`, `xenobot/db/cache/local.go`, `xenobot/db/backup/{types.go,backup.go,chatconv_backup_test.go}`, `xenobot/utils/{claude,gemini,featherless,extraction,ratelimit,keyword_matcher}` — **HIGH confidence** (direct inspection).
- **Empirical AI spike results**: `.planning/spikes/MANIFEST.md` and `.planning/notes/v1.3-interactive-presence.md` (Spikes 001–003, all `VALIDATED` on 2026-11-26 across `claude-haiku-5-5`, `gemini-4-hydrogen`, and Featherless `inclusionAI/Ling-3.2-air`) — **HIGH confidence**.

---
*Stack research for: Xenobot-72 Milestone v1.3 Interactive Presence*
*Researched: 2026-11-26*
