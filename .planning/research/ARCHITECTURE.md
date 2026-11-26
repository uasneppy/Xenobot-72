# Architecture Research — Milestone v1.3 Interactive Presence

**Domain:** Telegram Group-Management Bot — Interactive Presence, Reaction Pipeline & Room-Aware AI Extension (Go 1.26 / gotgbot v2 / PostgreSQL GORM / Redis)
**Researched:** 2026-11-26
**Confidence:** HIGH (all integration points, signatures, handler groups, cache keys, settings constraints, and backup invariants verified directly against the Xenobot-72 codebase and installed `gotgbot/v2` module)

---

## Standard Architecture

### System Overview

Milestone v1.3 layers an **Interactive Presence** subsystem onto Xenobot-72's existing handler pipeline, `StopChat` worker runtime, `settings|v1` stateless menu, and PostgreSQL/Redis boundaries without adding a single new shutdown drain or altering the 70 s webhook shutdown budget.

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                       Telegram Updates (Webhook / Polling)                   │
│  AllowedUpdates += "message_reaction"   ("poll" already enabled in config)   │
└──────────────┬──────────────────────────┬────────────────────────┬───────────┘
               │ Message / Edit           │ MessageReactionUpdated │ Poll
               ▼                          ▼                        ▼
┌──────────────────────────────┐  ┌───────────────────────┐  ┌─────────────────┐
│   Message Handler Pipeline   │  │  Group 0: NewReaction │  │ Group 0: NewPoll│
│                              │  │  presence_reactions.go│  │ chat_polls.go   │
│  -10..-1  Mod/Lock/Log/Users │  ├───────────────────────┤  ├─────────────────┤
│    0      Commands:          │  │ 1. 👎 Retraction (0AI)│  │ Closed & tied   │
│           /ask /unroll*      │  │ 2. Consensus (0AI)    │  │ bot-owned poll  │
│           /judge /referendum │  │ 3. Highlights (0AI)   │  │ → tie-breaker   │
│           /inspect /setarch* │  │ 4. ✍ PM Brief (AI)    │  └────────┬────────┘
│    3..7   AI Spam/Flood/BL   │  └───────────┬───────────┘           │
│    8      reactions.go       │              │                       │
│           + Smarter Static   │              │                       │
│           + Cat Stickers     │              │                       │
│           → ctx.Data mark    │              │                       │
│   13      chat_watch.go      │              │                       │
│           + Vocative prefix  │              │                       │
│           + AI React Gate    │              │                       │
│   14      chat_log.go (+At)  │              │                       │
└──────────────┬───────────────┘              │                       │
               │                              │                       │
               ▼                              ▼                       ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│            Unified Conversation & Presence Runtime (StopChat Drain)          │
│                                                                              │
│  ┌─────────────────────┐  ┌─────────────────────┐  ┌──────────────────────┐  │
│  │ chatQueue (3 wkrs)  │  │ chatBriefQueue (1w) │  │ chatPresenceQueue(2w)│  │
│  │ • @mention / reply  │  │ • /unrollshort/long │  │ • Contextual AI React│  │
│  │ • Vocative address  │  │ • /judge            │  │ • /inspect           │  │
│  │ • Ambient chat_log  │  │ • ✍ PM Brief        │  │ • /referendum        │  │
│  └──────────┬──────────┘  └──────────┬──────────┘  │ • Poll Tie-Breaker   │  │
│             └────────────────────────┴─────────────┴──────────┬───────────┘  │
│                                                               │              │
│   Shared: chatJobs tracker · chatDraining · chatContext() · chatBreakers     │
│           chatProviderOf() · chatDownloadMedia() · chatPacer · chatReserveWith│
└───────────────────────────────────────┬──────────────────────────────────────┘
                                        │
        ┌───────────────────────────────┴───────────────────────────────┐
        ▼                                                               ▼
┌─────────────────────────────────────────┐  ┌─────────────────────────────────┐
│    PostgreSQL (Durable Per-Group Config)│  │  Redis (Cross-Replica Ephemeral)│
│                                         │  │                                 │
│  • chat_conversations (Owner-only AI)   │  │  skipLocal DB cache:            │
│    - stays OUT of Backup & Copy         │  │   chat_conv:<chat>              │
│  • chat_presence (NEW — Admin-managed)  │  │   chat_presence:<chat>  (NEW)   │
│    - AI sub-toggles (inert if !conv.En) │  │                                 │
│    - Zero-AI toggles & static tuning    │  │  Runtime (outside xenobot:cache)│
│    - Highlights archive target + rules  │  │   xenobot:chat:{<c>}:log (+At)  │
│    - BackupModulePresence ("presence")  │  │   xenobot:chat:{<c>}:react:*    │
│  • reactions (existing keyword rules)   │  │   xenobot:chat:{<c>}:avail_react│
│                                         │  │   xenobot:chat:poll:<poll_id>   │
│                                         │  │   xenobot:presence:{<c>}:*      │
└─────────────────────────────────────────┘  └─────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Implementation Anchor |
|-----------|----------------|-----------------------|
| **Config & Update Subscription** | Subscribe both webhook (`httpserver/server.go:235`) and polling (`main.go:355`) to `"message_reaction"` updates (`"poll"` is already subscribed). | Modified: `xenobot/config/config.go` |
| **Presence Persistence (`chat_presence`)** | Store admin-configurable zero-AI settings and per-feature AI sub-toggles separately from owner-only AI consent (`chat_conversations`). Column-targeted upserts + `skipLocal` cache. | New: `migrations/*_add_chat_presence.sql`, `xenobot/db/models/chat_presence.go`, `xenobot/db/presence/repository.go`; Modified: `xenobot/db/cache/local.go` |
| **Reaction Update Router** | Single `handlers.NewReaction(reaction.All, ...)` entry at group `0` dispatching `👎` retraction, Emoji Consensus, Highlights Archive aggregation, and `✍` PM brief triggers. | New: `xenobot/modules/presence_reactions.go` |
| **Shared Reaction Helper** | Normalise emojis (`U+FE0F` stripped), intersect with cached `ChatFullInfo.AvailableReactions` ∩ `supportedReactionEmoji`, send paced `SetMessageReaction(IsBig: false)`, and record `(emoji, author_id, src)` in Redis `bot_react` for retraction. | New: `xenobot/modules/presence_react_api.go` |
| **Contextual Random AI Reactions** | Cheap local + Redis Lua anti-annoyance gate called from group `13` when message is neither addressed nor static-reacted; worker lane fetches `chat_log` tail + `chat_media.go` vision bytes, calls provider (`json`, `MaxTokens: 512`), validates output, stamps reaction. | New: `xenobot/modules/chat_react.go`, `xenobot/modules/chat_react_prompt.go`; Modified: `xenobot/modules/chat_watch.go`, `xenobot/modules/chat_run.go` |
| **Smarter Static Reactions & Cat Stickers** | Enhance group `8` (`reactions.go`) with word boundaries, caption matching, chance %, cooldowns, cat-sticker `msg.Sticker.Emoji` micro-triggers, Staff Group guard, and `ctx.Data` handoff marker. | Modified: `xenobot/modules/reactions.go` |
| **Crowd-Picked Highlights ("Facility Archive")** | Aggregate distinct member reactors per message in Redis sets, enforce once-only archive latch, `ForwardMessage`/`CopyMessage` + `t.me/c/` jump button to admin-verified target channel/topic (`/setarchive` / `/unsetarchive`). | New: `xenobot/modules/presence_archive.go` |
| **Room-Aware Conversations & Catch-Up** | Add `At int64` to `chatLogEntry`, add tail/user/time log readers, inject ambient thread context into `chatBuildRequest` (deduplicated against `chatHistory`), add vocative addressing, extend `/unrollshort`/`/unrolllong` with `<N\|me\|2h> [pm]`. | Modified: `xenobot/modules/chat_log.go`, `xenobot/modules/chat_run.go`, `xenobot/modules/chat_watch.go`, `xenobot/modules/chat_brief.go`, `xenobot/modules/chat_prompt.go`, `xenobot/utils/chatprompt/chatprompt.go` |
| **Group Toys (`/referendum`, Tie-Breaker, `/inspect`)** | AI-drafted native `SendPoll` + Redis poll registry + `handlers.NewPoll` tie-breaker on tied closed polls; member-accessible `/inspect` returning verdict + reaction emoji. | New: `xenobot/modules/chat_polls.go`, `xenobot/modules/chat_inspect.go`; Modified: `xenobot/modules/chat.go` |
| **Settings UI (Page `p` + Page `pr`)** | Keep page `p` (`OwnerOnly: true`) at `<= 8` rows by adding 1 shortcut row to new page `pr` (`OwnerOnly: false`, `PrivateOnly: true`, hidden in Staff Groups) with 4 sub-pages (`pa`, `pt`, `ps`, `ph`). | New: `xenobot/modules/settings_presence.go`; Modified: `xenobot/modules/settings_chat.go`, `xenobot/modules/settings_main_test.go` |
| **Backup Parity (`presence` module)** | Export/import/reset portable `chat_presence` fields under `BackupModulePresence = "presence"`, stripping destination `archive_chat_id`/`archive_thread_id` (matching `importLogChannels`), while keeping `chat_conversations` strictly excluded. | Modified: `xenobot/db/backup/types.go`, `xenobot/db/backup/backup.go` |

---

## Recommended Project Structure

Explicit map of **new** vs **modified** files aligned with Xenobot-72's existing package layout:

```
migrations/
└── 20261127120000_add_chat_presence.sql        # NEW: chat_presence table (single transaction, CHECK constraints)

xenobot/
├── config/
│   └── config.go                               # MODIFIED: add "message_reaction" to AllowedUpdates
├── db/
│   ├── cache/
│   │   └── local.go                            # MODIFIED: add chatPresencePrefix to skipLocal()
│   ├── models/
│   │   └── chat_presence.go                    # NEW: ChatPresence GORM model
│   ├── presence/
│   │   ├── repository.go                       # NEW: Get/GetFresh/column-targeted upserts/ReplacePortableTx
│   │   └── repository_test.go                  # NEW: constraint, cache invalidation, concurrency tests
│   └── backup/
│       ├── types.go                            # MODIFIED: BackupModulePresence = "presence", PresenceBackup struct
│       ├── backup.go                           # MODIFIED: export/import/clear presence (strip archive target IDs)
│       └── presence_backup_test.go             # NEW: roundtrip, sanitization, archive-strip, chatconv-exclusion tests
├── utils/
│   └── chatprompt/
│       └── chatprompt.go                       # MODIFIED: add UserWithAmbient() framing untrusted ambient room lines
└── modules/
    ├── chat.go                                 # MODIFIED: register /referendum, /inspect, NewPoll; AltHelpOptions
    ├── chat_watch.go                           # MODIFIED: vocative prefix check + ambient AI reaction hook in group 13
    ├── chat_log.go                             # MODIFIED: add At int64 to chatLogEntry + chatLogRecent/SinceUser/SinceTime
    ├── chat_run.go                             # MODIFIED: launch chatPresenceQueue in chatEnsureWorkers; pass ambient context; /stats
    ├── chat_prompt.go                          # MODIFIED: wire ambient room lines into chatBuildRequest
    ├── chat_brief.go                           # MODIFIED: /unrollshort <N|me|2h> [pm] + silent PM delivery for ✍ trigger
    ├── chat_react.go                           # NEW: contextual AI reaction gate, Lua throttle, worker job, budget
    ├── chat_react_prompt.go                    # NEW: compact reaction-picker prompt (73 emojis, JSON, MaxTokens=512, parser)
    ├── chat_polls.go                           # NEW: /referendum command, SendPoll, Redis poll registry, NewPoll tie-breaker
    ├── chat_inspect.go                         # NEW: /inspect command, JSON verdict+emoji prompt, media + reaction stamp
    ├── reactions.go                            # MODIFIED: word boundary, captions, chance %, cooldowns, cat-sticker triggers, ctx.Data mark
    ├── presence.go                             # NEW: LoadPresence (NewReaction, /setarchive, /unsetarchive) + shared guards
    ├── presence_react_api.go                   # NEW: AvailableReactions cache, bot_react tracker, paced safe reaction sender
    ├── presence_reactions.go                   # NEW: MessageReactionUpdated router (👎 retract, consensus, highlights, ✍ brief)
    ├── presence_archive.go                     # NEW: Highlights Archive reactor sets, latch, Forward/Copy + jump button, /setarchive
    ├── settings_chat.go                        # MODIFIED: add 1 shortcut row ("✨ Presence →" op p=pr) within 8-row cap
    ├── settings_presence.go                    # NEW: page "pr" hub + sub-pages (pa, pt, ps, ph) + apply helpers
    └── settings_main_test.go                   # MODIFIED: update settingsMainShape* constants (y|pr pair, same 17 rows!)
```

### Structure Rationale

- **`xenobot/db/presence/` separate from `xenobot/db/chatconv/`:** `chatconv` enforces the Phase 20 invariant (`TestChatConversationsStayOutOfBackup`) that AI consent and provider selection are Owner-only and never touched by Backup/Import/Reset. Putting admin-configurable interactive settings in `chat_presence` keeps both invariants clean with zero special-casing inside `chatconv`.
- **Presence worker lane inside `xenobot/modules/chat_run.go`:** Follows the exact pattern `chat_brief.go` established — separate bounded channel and worker goroutines started by `chatEnsureWorkers()`, counted in `chatJobs`, gated by `chatDraining`, and deriving every network/API call from `chatContext()`. Zero new shutdown drains, zero starvation of `@mention`/`/ask` replies.
- **`presence*.go` vs `chat_*.go` split in `xenobot/modules/`:** Zero-AI reaction/archive features (`presence*.go`) remain completely decoupled from AI providers and continue working even when `ENABLE_CHAT=false`. AI-backed interactive features (`chat_react.go`, `chat_polls.go`, `chat_inspect.go`) sit alongside `chat_run.go` and reuse its provider adapters, breakers, media downloader, and `StopChat` lifecycle.

---

## Architectural Patterns

### Pattern 1: Handler Group Placement & `ctx.Data` Handoff

**What:** Preserve the exact `-10..14` message handler ordering without inserting new message groups. Use `ctx.Data` (already initialized per update by `tracing.TracingProcessor` at `xenobot/utils/tracing/processor.go:57-58`) for zero-I/O coordination between group `8` and group `13`. Register non-message handlers (`handlers.NewReaction`, `handlers.NewPoll`) at group `0` — gotgbot dispatches by update type, so they never contend with message handlers, and `CommandDeletionProcessor` (`xenobot/modules/command_deletion.go:59-60`) returns immediately when `ctx.Message == nil`.

**Placement table:**

| Handler / Feature | Group | Update Type | Ends Update? | Why This Group |
|-------------------|-------|-------------|--------------|----------------|
| `/referendum`, `/inspect`, `/setarchive`, `/unsetarchive`, extended `/unrollshort`/`/unrolllong` | `0` | Message (command) | `ext.EndGroups` | Standard `helpers.WrapCommand` commands (`setarchive` uses `.SetAllowChannel(true)` like `/setlog` at `logchannels.go:404`). |
| `handlers.NewReaction(reaction.All, ...)` | `0` | `MessageReactionUpdated` | `ext.EndGroups` | Non-message update; single router avoids redundant Redis/DB reads across 👎 retraction, consensus, highlights, and ✍ brief. |
| `handlers.NewPoll(poll.All, ...)` | `0` | `Poll` | `ext.EndGroups` | Non-message update; checks `poll.IsClosed` + Redis `xenobot:chat:poll:<id>` registry for bot-created tied polls. |
| Smarter static reactions + Cat-sticker micro-triggers | `8` | Message (`reactions.go`) | `ext.ContinueGroups` | Runs after moderation deletes (`aispam` 3, `antiflood` 4, `locks` 5–6, `blacklists` 7). Sets `ctx.Data[presenceStaticReactedKey] = true` on reaction so group `13` skips AI reaction. |
| Vocative name-addressing + Contextual AI reaction gate | `13` | Message (`chat_watch.go`) | `ext.ContinueGroups` | Mutually exclusive branch in one watcher: if addressed (mention / reply-to-bot / vocative prefix) → enqueue `chatJob`; else if `!ctx.Data[presenceStaticReactedKey]` → evaluate ambient AI reaction gate. |
| `chatLogWatcher` (`chat_log.go`) | `14` | Message + EditedMessage | `ext.ContinueGroups` | Unchanged position; records `At int64` from `msg.Date`. Async workers in group `13` filter `entry.MessageID < job.messageID` so group `13` vs `14` timing is race-free. |

**Example (`ctx.Data` handoff and group 13 mutual exclusion):**
```go
const presenceStaticReactedKey = "presence.static_reacted"

// In reactions.go (group 8), after SetMessageReaction succeeds:
if ctx.Data == nil {
    ctx.Data = make(map[string]any)
}
ctx.Data[presenceStaticReactedKey] = true

// In chat_watch.go (group 13):
if addressed, vocative := chatAddressedOrVocative(b, msg); addressed {
    // Verify chatconv.Enabled (+ presence.VocativeEnabled if vocative), then chatEnqueue(...)
    return
}
if reacted, _ := ctx.Data[presenceStaticReactedKey].(bool); !reacted {
    chatReactWatch(b, ctx) // cheap local + Redis gate; enqueues chatReactJob only on rare roll win
}
```

### Pattern 2: Zero New Shutdown Drains via `StopChat` Multi-Lane Pool

**What:** Reuse `StopChat()` (`xenobot/modules/chat_run.go:380-390`) identically to how `chat_brief.go` already joins it (`chat_run.go:153`, `chat_brief.go:321-338`). Launch a third lane (`chatPresenceQueue`, capacity `32`, `2` workers) inside `chatEnsureWorkers()` for contextual AI reactions, `/inspect`, `/referendum`, and AI poll tie-breakers.

**When to use:** Every asynchronous AI or multi-step Telegram task in v1.3.

**Trade-offs:**
- **Pros:** Zero new handlers in `main.go`; webhook worst-case shutdown stays **70 s** (`stop_grace_period: 75s` unchanged); ambient reactions cannot head-of-line block `@mention` replies (`chatQueue`) or `/unrollshort`/`/judge` (`chatBriefQueue`).
- **Cons:** All three lanes share the single 2 s `chatStopWait` timer on shutdown; cut-off ambient reactions or toys are simply lost (best effort — nothing needs DB resumption, matching Conversations and Briefs).
- **Zero-AI reaction handlers (`👎` retraction, Emoji Consensus, Highlights Archive):** Execute synchronously inside the `handlers.NewReaction` update handler under a short bounded timeout (`3 * time.Second`) derived from `tracing.UpdateContext(ctx)` (and `chatPaced`). Because `httpServer.Stop()` / `updater.Stop()` already drain in-flight updates (10 s) before `StopChat()`, zero extra drain is needed.

### Pattern 3: Two-Tier Authority Model (Owner-Unlock + Admin-Toggle)

**What:** Separate **AI enablement & privacy consent** (Owner-only, `chat_conversations` on page `p`) from **interactive feature configuration** (Admin-configurable, `chat_presence` on page `pr`).

| Capability | Server Switch | Owner Unlock (`chat_conversations`, page `p`) | Admin Config (`chat_presence`, page `pr` / commands) | Effective Runtime Condition |
|------------|---------------|-----------------------------------------------|------------------------------------------------------|-----------------------------|
| **Contextual AI Reactions** | `ENABLE_CHAT` + key | `conv.Enabled == true` | `ai_react_mode` (`Off`/`Low`/`Medium`) | `conv.Enabled && mode > 0 && !staff && !lockdown` |
| **Ambient Room Context** | `ENABLE_CHAT` + key | `conv.Enabled && conv.LogEnabled` | `ambient_context == true` | `conv.Enabled && conv.LogEnabled && ambient_context` |
| **Vocative Addressing** | `ENABLE_CHAT` + key | `conv.Enabled == true` | `vocative_enabled == true` | `conv.Enabled && vocative_enabled && !staff` |
| **Silent `✍` PM Brief** | `ENABLE_CHAT` + key | `conv.LogEnabled == true` | `pm_brief_react == true` | `conv.LogEnabled && pm_brief_react && !staff` |
| **`/referendum` + Tie-Breaker** | `ENABLE_CHAT` + key | `conv.Enabled == true` | `referendum_enabled == true` | `conv.Enabled && referendum_enabled && !staff` |
| **`/inspect`** | `ENABLE_CHAT` + key | `conv.Enabled == true` | `inspect_enabled == true` | `conv.Enabled && inspect_enabled && !staff` |
| **Emoji Consensus** | *(none — 0 AI)* | *(not required)* | `consensus_enabled`, `consensus_threshold` | `consensus_enabled && !staff && !lockdown` |
| **`👎` Retraction** | *(none — 0 AI)* | *(not required)* | `retract_enabled` (default `true`) | `retract_enabled && bot_reacted` |
| **Smarter Static + Cat Stickers** | *(none — 0 AI)* | *(not required)* | `cat_sticker_enabled`, `static_*` | `rule_match && !staff && !lockdown` |
| **Highlights Archive** | *(none — 0 AI)* | *(not required)* | `/setarchive` + `archive_enabled`, threshold, emoji | `archive_enabled && archive_chat_id != 0 && !staff && !lockdown` |

**Why this fits `settings_menu.go` cleanly:**
- Page `p` (`settingsPageChat`, `OwnerOnly: true`, `PrivateOnly: true`) stays strictly Owner-only (`settings_menu.go:595` refuses non-creators before `pg.Open` runs) and adds just **1 row** (`✨ Presence →`, `a=op&p=pr`), reaching **8 rows** max — respecting `TestSettingsChatPageFitsEveryLocale`'s `rows <= 8` assertion (`settings_chat_test.go:567`).
- Page `pr` (`settingsPagePresence`, `Order: 91`, `OwnerOnly: false`, `PrivateOnly: true`, `Hidden: settingsPresenceHidden` [hides in Staff Groups via `staff.GetStaffGroupFresh`]) uses `Right: settingsRightChangeInfo` (matching `settings_reactions.go:141`).
  - **Main-grid bonus:** `settingsMainShapePrivate` (`settings_main_test.go:37`) currently has 27 non-owner grid pages leaving `y` (Anonymous admins) alone on a half-row (`... / z|o / y / p / a / ...`). Adding `pr` pairs `y|pr` into the empty right slot, keeping `settingsMainOwnerRows = 17` **unchanged**!
  - **Menu access gate:** Respects v1.1 `settingsGate` (`settings_gate.go:89`): Owner always has access; regular admins get access when Owner enables "Admins can change settings" (or when no live owner exists), while in-group admin commands (`/addreaction`, `/setarchive`, `/unsetarchive`) stay open to live admins with `CanUserChangeInfo`.
  - **AI sub-page locking:** Inside `pr`'s AI sub-pages (`pa`, `pt`), enabling an AI toggle checks `chatconv.GetFresh`; if `!conv.Enabled` (or `!conv.LogEnabled` for `ambient_context` / `pm_brief_react`), the write is refused with an alert telling the admin that the group Owner must enable Conversations / Message log first. Disabling (`v=0`) is always allowed for admins.

### Pattern 4: Bridging Telegram Bot API Metadata Gaps with Short-Lived Redis Records

**What:** Both `MessageReactionUpdated` and `Poll` updates omit key context that Telegram's Bot API provides no `getMessage` method to recover. Store tiny, bounded, TTL-scoped metadata records in Redis at the moment the bot acts:

1. **`bot_react` record (`xenobot:presence:{<chat>}:bot_react:<msg>`, TTL 24h):**
   - Written whenever the bot calls `SetMessageReaction` (from AI reactions, static reactions, cat stickers, `/inspect`, or Emoji Consensus).
   - Stores `{"e":"👀","a":<authorUserID>,"s":"ai"}`.
   - Solves `👎` Retraction: when a `MessageReactionUpdated` arrives with `👎`, the bot immediately knows (a) it actually reacted to `msgID` and (b) whether `mru.User.Id == authorUserID` without needing `getMessage`. (For Emoji Consensus where `mru` has no author, look up `chat_log` at score `msgID`; if unavailable, `a = 0` so group admins can still retract.)
2. **Bot poll registry (`xenobot:chat:poll:<pollID>`, TTL 7d):**
   - `gotgbot.Poll` updates carry ONLY `Poll.Id` — no `chat_id`, no `message_id`, no `thread_id`.
   - Written when `/referendum` sends a poll (`sent.Poll.Id` → `{"c":chatID,"m":msgID,"th":threadID,"q":question,"p":provider}`).
   - When `handlers.NewPoll` sees `poll.IsClosed`, one Redis `GET` identifies bot-owned polls and supplies the exact reply target for the tie-breaker.
3. **Exact distinct-reactor tracking (`xenobot:presence:{<chat>}:reactors:<msg>:<emoji_hex>` & `reactors_all:<msg>`, TTL 48h):**
   - Because `MessageReactionUpdated` provides both `OldReaction` and `NewReaction` (the user's full before/after reaction list on that message):
     - Per-emoji sets: `SADD` for each standard emoji in `New \ Old`; `SREM` for each in `Old \ New`.
     - Any-emoji set (for Highlights when `archive_emoji == ""`): if `len(validNew) > 0` → `SADD reactors_all:<msg> userID`; else → `SREM reactors_all:<msg> userID`. Zero double-counting and zero false decrements when a Premium user holds 2 emojis and removes 1.

---

## Data Flow

### Key Data Flows

#### 1. Contextual Random AI Emoji Reaction Flow (Group 13 → Presence Worker Lane)
```
Group Message (group 8: reactions.go did NOT set ctx.Data[presenceStaticReactedKey])
  ↓
group 13: chatWatch() — NOT @mention, NOT reply-to-bot, NOT vocative
  ↓
chatReactWatch() [cheap synchronous gate, zero Postgres calls]:
  1. Skip non-group, slash text, bots, anonymous channels, empty content (no text/caption/photo/sticker)
  2. Read chatconv.Get (skipLocal Redis) & presence.Get (skipLocal Redis) → require EnableChat && conv.Enabled && ai_react_mode > 0
  3. Read staff.GetStaffGroup(chatID) (skipLocal Redis) → skip Staff Groups
  4. Atomic Redis Lua throttle (chatReactGateScript):
     INCR react:gap → require gap >= minGap (Low: 30, Med: 15)
     && !EXISTS react:cd:chat && !EXISTS react:cd:u:<user>
     → roll local rand probability (Low: 25%, Med: 45%)
     → on WIN: reset react:gap=0, SETNX react:cd:chat & react:cd:u:<user>
  ↓ (only on rare ~1-in-30..60 roll win!)
chatPresenceEnqueue(chatReactJob)  [counts in chatJobs, gated by chatDraining]
  ↓
chatPresenceWorker() [ctx derived from chatContext(), 45s timeout]:
  1. lockdown.GetActiveFresh(chatID) → abort silently if locked down
  2. chatBreaker(provider).isPaused(chatID) → abort silently
  3. chatReserveWith(client, chatID, userID, "react:b", uCap, cCap, gCap) → abort silently if capped
  4. Resolve allowed emojis = supportedReactionEmoji ∩ cached AvailableReactions (Redis 20m TTL)
  5. Gather context:
     • If conv.LogEnabled: chatLogRecent(chatID, threadID, 5) filtered to MessageID < job.messageID
     • Always free: ReplyTarget(msg) / msg.Quote captured in job
     • If photo/sticker & provider supports vision: chatDownloadMedia() (reusing chat_media.go!)
  6. Call chatProviderOf(provider).converse(ctx, req) with JSON prompt & MaxTokens = 512
     • Parse {"emoji":"..."} — on parse error / ErrNoReply / Featherless "no choices" → shed as NONE (DO NOT trip breaker!)
     • On real HTTP 401/403/429/5xx → chatRecordFailure()
  7. Normalise emoji (strip U+FE0F) & verify membership in allowed set; if "NONE" or invalid → return
  8. presenceSetReaction(ctx, bot, chatID, msgID, authorID, emoji, "ai"):
     • chatPaced → b.SetMessageReaction(IsBig: false) (400 deleted/invalid ignored silently, never trips breaker)
     • Record xenobot:presence:{<chat>}:bot_react:<msg> (TTL 24h)
```

#### 2. `MessageReactionUpdated` Event Flow (Group 0)
```
Telegram MessageReactionUpdated (mru)
  ↓
presenceReactionHandler (group 0, bounded 3s ctx from tracing.UpdateContext):
  • Ignore private chats, nil mru.User, mru.User.IsBot, staff.GetStaffGroup(chatID) != nil
  • Extract added/removed standard emojis (normalised, U+FE0F stripped)
  • Read presence.Get(ctx, chatID) (skipLocal Redis)
  ├─► [Branch A: 👎 Retraction] (zero-AI)
  │     If "👎" in added && presence.RetractEnabled:
  │       GET bot_react:<msg> → if found && (mru.User.Id == rec.Author || IsUserAdmin):
  │         SetMessageReaction(Reaction: nil) + DEL bot_react:<msg>
  │         SET react:cd:u:<user> TTL 24h (extended user cooldown)
  ├─► [Branch B: Emoji Consensus] (zero-AI)
  │     Update reactors:<msg>:<emoji> sets (SADD added / SREM removed, TTL 48h)
  │     If presence.ConsensusEnabled && SCARD >= consensus_threshold && !lockdown:
  │       If SETNX cons:cd (5m) && SETNX cons:done:<msg> (48h):
  │         Verify emoji ∈ AvailableReactions → presenceSetReaction(..., "cons")
  ├─► [Branch C: Highlights Archive] (zero-AI)
  │     If presence.ArchiveEnabled && presence.ArchiveChatID != 0 && !lockdown:
  │       Update reactors_all:<msg> (or specific emoji set)
  │       If SCARD >= archive_threshold && SETNX arch:done:<msg> (7d):
  │         ForwardMessage to (ArchiveChatID, ArchiveThreadID) + jump button
  │         (fallback: CopyMessage + caption/button if forward fails)
  └─► [Branch D: ✍ PM Brief] (AI-backed)
        If "✍" in added && presence.PMBriefReact && chatAnyProviderConfigured():
          Read chatconv.Get → require conv.LogEnabled && chatProviderConfigured(conv.Provider)
          Verify chatLogHas(chatID, mru.MessageId)
          Enqueue chatBriefJob{dest: JudgeToPrivate, silentPMBlocked: true, fromID: mru.MessageId, userID: mru.User.Id}
```

#### 3. Room-Aware Conversations & Catch-Up Flow
```
[Conversations Reply]
  chat_watch.go (group 13):
    • Matches @mention OR reply-to-bot-own-msg OR (pure vocative prefix regex && conv.Enabled && presence.VocativeEnabled && !staff)
    • Enqueues chatJob
  chat_run.go (chatQueue worker):
    • Reads chatReadHistory() (direct bot exchange, up to 12 lines)
    • If conv.LogEnabled && presence.AmbientContext:
        reads chatLogRecent(chatID, threadID, 8)
        drops entries with MessageID >= job.messageID or MessageID already in chatHistory
    • chatBuildRequest() → chatprompt.UserWithAmbient(ambient, history, quoted, turn, files)
    • Provider call → filter → reply → chatRecordExchange()

[Frictionless Catch-Up]
  /unrollshort [N | me | 2h] [pm]  (or as a reply + optional "pm")
    • Reply target present → existing chatLogSince(fromID)
    • Integer N (5..200) → chatLogRecent(chatID, threadID, N)
    • "me" → chatLogSinceUser(chatID, threadID, userID)
    • Duration (10m..24h) → chatLogSinceTime(chatID, threadID, now - dur) [uses new chatLogEntry.At]
    • Trailing "pm" → job.dest = chatconv.JudgeToPrivate (reuses chatJudgeDeliverPrivate)
```

---

## Backup & Copy Settings Audit

Verified directly against `xenobot/db/backup/backup.go`, `xenobot/db/backup/chatconv_backup_test.go`, `xenobot/modules/backup.go`, and `xenobot/modules/copy_settings.go`:

### 1. `/export`, `/import`, `/reset` (`xenobot/db/backup/`)

| Data Area | Backup Module | Export | Import Behavior | Reset Behavior | Rationale |
|-----------|---------------|--------|-----------------|----------------|-----------|
| `chat_conversations` (`enabled`, `provider`, `audience`, `log_enabled`, `judge_to`) | **EXCLUDED** | Never exported | Never written | Untouched | Preserves `TestChatConversationsStayOutOfBackup`: importing a backup must NEVER silently unlock AI processing or message logging without the target Owner's 2-tap data consent. |
| `reactions` (keyword → emoji rows) | `reactions` (existing) | Unchanged | Unchanged (keep normalizing emoji) | Deletes rows | Full backward compatibility with existing v1.0/v1.1 backup files. |
| `chat_presence` portable toggles & tuning (`ai_react_mode`, `ambient_context`, `vocative_enabled`, `pm_brief_react`, `referendum_enabled`, `inspect_enabled`, `consensus_*`, `retract_enabled`, `cat_sticker_enabled`, `static_*`, `archive_threshold`, `archive_emoji`) | **`presence` (NEW)** | Exported as `PresenceBackup` (omitting `archive_chat_id` & `archive_thread_id`) | Clamped/validated (e.g., `ai_react_mode ∈ 0..2`, `static_chance_pct ∈ 1..100`, `archive_emoji` in `""` ∪ `supportedReactionEmoji`). AI sub-toggles stay **inert** at runtime until Owner unlocks `chat_conversations`. | Resets row to safe defaults (`chat_presence:<chat>` dropped) | Portable admin configuration; safe even for AI sub-toggles because `chat_conversations.enabled` remains the hard runtime gate. |
| `chat_presence.archive_chat_id`, `archive_thread_id`, `archive_enabled` | **`presence` (NEW)** | Target IDs **stripped** (never exported, like `greetings.LastMsgId`) | **Never overwrites** target's `archive_chat_id`/`archive_thread_id` (mirrors `importLogChannels`, `backup.go:1285-1317`). Applies `archive_enabled` ONLY if target already has `archive_chat_id != 0`; otherwise forces `false` and flags `ArchiveSkipped` in `ImportReport`. | Unlinks target (`archive_chat_id = 0`, `archive_thread_id = 0`, `archive_enabled = false`) | Prevents a tampered or cross-group backup from exfiltrating group highlights into another group's channel without `/setarchive` proof! |

### 2. Copy Settings (`xenobot/modules/copy_settings.go`)

- **Recommendation: Do NOT add `chat_presence` or `chat_conversations` to `applyCopySettings`.**
- **Why (verified at `copy_settings.go:212-226`):** `applyCopySettings` is explicitly scoped as an atomic copy of **moderation messages & punishment tuning** (6 named areas across 9 tables) and deliberately excludes content/feature modules (`greetings`, `rules`, `notes`, `filters`, `locks`, `reactions`, `chat_settings_access`). Copying interactive presence toys or archive settings alongside punishment messages would violate user expectations and expand the blast radius of Copy Settings. Admins migrating presence settings between groups should use `/export presence` → `/import presence` (or the private Backup page `ba`).

---

## Integration Points

### External Services & Bot API Contracts

| Service / API | Integration Pattern | Verified Gotchas & Guardrails |
|---------------|---------------------|-------------------------------|
| **Telegram `MessageReactionUpdated`** | Add `"message_reaction"` to `config.AppConfig.AllowedUpdates`; register `handlers.NewReaction(reaction.All, ...)` at group `0`. | Update carries NO message body and NO message author. Bot must have admin status in supergroups/channels to receive all reaction updates (`message_reaction` is only delivered to admins in chats). |
| **Telegram `SetMessageReaction`** | Paced via `chatPaced`; always `IsBig: false`; emoji normalized via `normaliseReactionEmoji` (`U+FE0F` stripped) and intersected with `supportedReactionEmoji ∩ ChatFullInfo.AvailableReactions`. | `AvailableReactions == nil` means all 73 standard emojis allowed. Silent `400` (`REACTION_INVALID`, `MESSAGE_ID_INVALID`) must be ignored and **never** trip `chatBreakers`. |
| **Telegram `SendPoll` + `Poll` update** | `"poll"` is already in `AllowedUpdates` (`config.go:556`); register `handlers.NewPoll(poll.All, ...)` at group `0`. `SendPoll` caps: question 1–300 chars, 2–12 options (1–100 chars each), `OpenPeriod` 5–2,628,000 s. | `Poll` update carries `Poll.Id` only (no `chat_id` or `message_id`). Store `xenobot:chat:poll:<id>` in Redis on send. Use `SETNX` latch on tie-breaker so redelivered closed-poll updates never double-post. |
| **AI Providers (`claude`, `gemini`, `featherless`)** | Reuse `chatProviderOf(provider).converse(ctx, req)` (`chat_prompt.go:83`) and `chatDownloadMedia()` (`chat_media.go:232`). | **Empirical Spikes 001–003:** Reaction picker MUST use `json` wire format (`{"emoji":"..."}` / `{"emoji":"NONE"}`) with **`MaxTokens = 512`** (`gemini-4-hydrogen` spends ~85 thinking tokens at `thinkingLevel: "low"`; Featherless overflows prose below `512`). Prompt MUST explicitly warn that `😂` and cat faces (`🐱/😹/😻/😼`) are NOT valid Telegram reactions. Shed parse errors & Featherless `no choices` silently as `NONE` without tripping `chatBreakers`. |

### Internal Boundaries

| Boundary | Communication | Key Considerations |
|----------|---------------|--------------------|
| `reactions.go` (group `8`) → `chat_watch.go` (group `13`) | In-update `ctx.Data[presenceStaticReactedKey]` bool | Zero I/O; guarantees static keyword / cat-sticker reactions always take precedence over contextual AI reactions. |
| `chat_watch.go` (group `13`) ↔ `chat_log.go` (group `14`) | Redis sorted set `xenobot:chat:{<chat>}:log` | Workers filter `entry.MessageID < job.messageID` (reactions & ambient context) so group 13 enqueueing before group 14 writes is completely deterministic. |
| `chat_run.go` ↔ `chat_react.go` / `chat_polls.go` / `chat_inspect.go` | Shared `chatJobs`, `chatDraining`, `chatContext()`, `chatReserveWith()`, `chatPaced()`, `chatBreakers` | Dedicated `chatPresenceQueue` prevents ambient traffic from starving `@mention` replies; dedicated budget families (`react:b`, `toy:b`) keep hourly caps independent from `b` (conversations) and `brief:b` (briefs). |
| `settings_chat.go` (`p`) ↔ `settings_presence.go` (`pr`) | Stateless callback link (`a=op&p=pr&c=<chat>`) + fresh `chatconv.GetFresh` read | `p` stays `OwnerOnly: true` (<= 8 rows); `pr` is `OwnerOnly: false`, `PrivateOnly: true`, `Hidden` in Staff Groups, and refuses enabling AI toggles unless `chatconv` is unlocked by Owner. |
| `presence_archive.go` ↔ `logchannels.go` pattern | Redis 1h pending marker `xenobot:setarchive:<target>:<msg>` | Exact 2-step forward-proof model as `/setlog` (plus direct `/setarchive` inside a forum topic of the same group with live `CanChangeInfo`), preventing arbitrary-chat exfiltration. |

---

## Anti-Patterns to Avoid

### Anti-Pattern 1: Piling Admin Interactive Toggles into `chat_conversations`
**What people do:** Add 15 new columns (`ai_react_mode`, `consensus_enabled`, `archive_chat_id`, etc.) onto `chat_conversations` and expose them on page `p`.
**Why it's wrong:** Breaks page `p`'s `OwnerOnly: true` boundary (admins can't access page `p`), violates `TestSettingsChatPageFitsEveryLocale`'s 8-row cap, and breaks `TestChatConversationsStayOutOfBackup`'s clean exclusion invariant.
**Do this instead:** Create `chat_presence` (`xenobot/db/presence/`) + page `pr` (`OwnerOnly: false`, `PrivateOnly: true`, paired next to `y` on Main), linked from page `p`'s 8th row.

### Anti-Pattern 2: Calling `lockdown.GetActiveFresh` on Every Group Message
**What people do:** Check `lockdown.GetActiveFresh(chatID)` at the top of `chatReactWatch` or `reactions.go` on every incoming message.
**Why it's wrong:** `lockdown.GetActiveFresh` hits PostgreSQL directly (no cache). Running it on every message in busy groups adds massive unnecessary DB load.
**Do this instead:** Gate on cheap pure checks + cached `skipLocal` Redis reads (`chatconv.Get`, `presence.Get`, `staff.GetStaffGroup`) + the Redis Lua gap/cooldown/probability roll FIRST. Call `lockdown.GetActiveFresh` **only when a reaction/archive/consensus event is actually about to fire** (rare: ~1 in 30–60 messages or threshold crossings).

### Anti-Pattern 3: Tripping `chatBreakers` on Reaction Parse Errors or Telegram `400`s
**What people do:** Reuse `chatRecordFailure` blindly whenever the reaction picker returns an unparseable string, `ErrNoReply`, or `SetMessageReaction` returns `400 REACTION_INVALID` / `MESSAGE_ID_INVALID`.
**Why it's wrong:** `chatBreakers[provider]` is shared with Conversations and Briefs (`chat_run.go:114`). Five deleted messages or Featherless formatting glitches would pause the entire group's Conversations for 5 minutes.
**Do this instead:** Silently drop reaction JSON parse failures, invalid emoji outputs, `ErrNoReply` on reaction picks, and Telegram `400` errors as `NONE` (incrementing a dedicated `chatReactShedCount` metric for `/stats`). Feed `chatRecordFailure` only on genuine provider HTTP 401/403/429/5xx or transport errors.

### Anti-Pattern 4: Importing `archive_chat_id` or Accepting Raw Chat IDs for Highlights Archive
**What people do:** Allow admins to type `-1001234567890` into a settings prompt or import `archive_chat_id` verbatim from a backup JSON file.
**Why it's wrong:** Any admin could point their group's Highlights Archive at another private channel the bot administers, exfiltrating/spamming that channel without proving admin rights there.
**Do this instead:** Require the 2-step `/setarchive` forward proof (mirroring `/setlog` in `logchannels.go:109-142`) or same-group forum-topic `/setarchive`, and always strip `archive_chat_id`/`archive_thread_id` on backup export/import (mirroring `importLogChannels`).

---

## Scaling Considerations

| Scale | Architecture Adjustments |
|-------|--------------------------|
| **Current target (owner's own communities, multi-replica)** | Single PostgreSQL + Redis + 2-worker `chatPresenceQueue` per replica. Redis Lua throttle (`chatReactGateScript`) + `SETNX` latches keep all cross-replica reaction, consensus, archive, and tie-breaker behavior exact with zero DB writes on the hot message path. |
| **Busy groups (100+ msg/min during active chats)** | `chatReactWatch` pays at most 2 `skipLocal` Redis `GET`s + 1 Lua `EVALSHA` per message, and skips `lockdown.GetActiveFresh` unless the probability roll wins. `MessageReactionUpdated` updates touch Redis sets only when `consensus_enabled || archive_enabled || retract_enabled || pm_brief_react`. |
| **Burst / raid protection** | Dedicated `chatPresenceQueue` (cap 32, shed on full) + `chatReserveWith(..., "react:b", ...)` hourly caps + `lockdown.GetActiveFresh` silence guard ensure a spam burst can never exhaust AI budgets or starve `@mention` replies. |

---

## Suggested Phase Build Order

Dependencies flow strictly top-down across three vertical phases (matching `v1.3-interactive-presence.md` Phases A → B → C, mapped to Milestone v1.3 Phases **31 → 32 → 33**):

### Phase 31 (Phase A) — Reaction & Presence Foundation + Contextual AI Reactions
**Why first:** Establishes the `"message_reaction"` subscription, `chat_presence` schema & `skipLocal` cache, shared reaction API + `AvailableReactions` cache + `bot_react` tracker, `chatPresenceQueue` under `StopChat`, and the `pr` Settings page with Owner-unlock / Admin-toggle enforcement that Phases 32 and 33 plug into.

1. **Config & DB Foundation:** Add `"message_reaction"` to `AllowedUpdates` (`config.go`); add migration `add_chat_presence.sql`, `models.ChatPresence`, `xenobot/db/presence/` repository, and `chatPresencePrefix` in `skipLocal` (`local.go`).
2. **Reaction API & Update Router:** Build `presence_react_api.go` (`AvailableReactions` 20m Redis cache, `bot_react` 24h tracker, paced `SetMessageReaction(IsBig: false)`) and `presence_reactions.go` (`handlers.NewReaction(reaction.All, ...)` at group `0`).
3. **Zero-AI Social Reactions:** Implement `👎` Retraction (author/admin clears bot reaction + 24h user cooldown) and Emoji Consensus (distinct-reactor Redis sets, >= 3 matching reactions, chat cooldown + once-only latch).
4. **Contextual Random AI Emoji Reactions:** Launch `chatPresenceQueue` inside `chatEnsureWorkers()` (`StopChat` drain unchanged); build `chat_react_prompt.go` (73 standard emojis, `json` wire format, `MaxTokens = 512`, prompt warning on `😂`/cat faces) and `chat_react.go` (group `13` gate, Redis Lua gap/cooldown/roll throttle, `chat_log` tail + `chat_media.go` vision, breaker-safe error shedding, `/stats` counters).
5. **Settings Menu (`p` + `pr`):** Register `settingsPagePresence = "pr"` (`Order: 91`, `PrivateOnly: true`, `Hidden` in Staff Groups, paired with `y` on Main), add 1 shortcut row to page `p` (stays `<= 8` rows), wire Owner-unlock checks on AI sub-pages, update `settings_main_test.go`.

### Phase 32 (Phase B) — Community Polish, Highlights Archive & Backup Parity
**Depends on:** Phase 31 (`chat_presence`, `presence_reactions.go`, `presence_react_api.go`, page `pr`).

1. **Smarter Static Reactions & Cat Stickers (`reactions.go`):** Add word-boundary matching, caption support, chance %, Redis cooldowns, Staff Group/lockdown guards, zero-AI cat-sticker micro-triggers (`msg.Sticker.Emoji`), and `ctx.Data[presenceStaticReactedKey]` handoff to group `13`.
2. **Crowd-Picked Highlights ("Facility Archive"):** Build `presence_archive.go` (Redis distinct-reactor tracking inside `presence_reactions.go`, once-only 7d latch, `ForwardMessage`/`CopyMessage` + `t.me/c/` jump button, `/setarchive` & `/unsetarchive` 2-step proof modeled on `/setlog`), wire sub-page `ph` in `settings_presence.go`.
3. **Backup Parity:** Add `BackupModulePresence = "presence"` in `xenobot/db/backup/` (export/import/reset with clamping & archive-target stripping matching `importLogChannels`, plus regression tests proving `chat_conversations` remains excluded and `copy_settings.go` remains untouched).

### Phase 33 (Phase C) — Room-Aware Conversations, Catch-Up & Group Toys
**Depends on:** Phase 31 (`chat_presence`, `chatPresenceQueue`, `presence_reactions.go`, page `pr`).

1. **Log Timestamps & Query Helpers (`chat_log.go`):** Add backward-compatible `At int64` (`json:"at,omitempty"`) to `chatLogEntry` and add `chatLogRecent`, `chatLogSinceUser`, `chatLogSinceTime`.
2. **Ambient Room Context & Vocative Addressing:** Inject deduplicated last 6–10 same-thread `chat_log` entries into `chatBuildRequest` (`chat_run.go`, `chat_prompt.go`, `chatprompt.UserWithAmbient`); add opt-in pure start-of-message vocative prefix matching (`"Xenobot, ..."` / `"Кзенобот, ..."`) in `chat_watch.go`.
3. **Frictionless Catch-Up & Silent `✍` PM Brief:** Extend `/unrollshort` / `/unrolllong` for `<N|me|2h> [pm]` (`chat_brief.go`) and wire the `✍` reaction branch in `presence_reactions.go` to enqueue silent-403 private briefs.
4. **AI Polls (`/referendum`) + Tie-Breaker (`chat_polls.go`):** Draft native `SendPoll` from thread/reply context in `chatPresenceQueue`, store `xenobot:chat:poll:<id>` in Redis, wire `handlers.NewPoll(poll.All, ...)` at group `0` to post a deadpan once-only tie-breaker when a bot poll closes in a tie.
5. **Playful `/inspect` (`chat_inspect.go`):** Member-accessible sibling to `/judge` running in `chatPresenceQueue`, supporting text + `chat_media.go` vision, posting a 2–3 line inspection reply and stamping a validated matching reaction emoji onto the target message (recorded in `bot_react` for `👎` retraction).

---

## Sources

- **Codebase (HIGH confidence — directly read and verified):**
  - `xenobot/config/config.go` (`AllowedUpdates`, `EnableChat`, provider config)
  - `main.go` & `xenobot/utils/httpserver/server.go` (LIFO shutdown drains, webhook/polling `AllowedUpdates`)
  - `xenobot/modules/chat.go`, `chat_watch.go`, `chat_log.go`, `chat_run.go`, `chat_brief.go`, `chat_brief_prompt.go`, `chat_media.go`, `chat_prompt.go`, `chat_settings.go`, `chat_history.go`, `chat_send.go`
  - `xenobot/modules/reactions.go`, `settings_reactions.go`, `settings_chat.go`, `settings_chat_test.go`, `settings_menu.go`, `settings_pages.go`, `settings_gate.go`, `settings_main_test.go`, `logchannels.go`, `command_deletion.go`
  - `xenobot/modules/copy_settings.go`, `backup.go`, `xenobot/db/backup/backup.go`, `types.go`, `chatconv_backup_test.go`
  - `xenobot/db/chatconv/repository.go`, `xenobot/db/reactions/repository.go`, `xenobot/db/staff/repository.go`, `xenobot/db/lockdown/repository.go`, `xenobot/db/cache/local.go`, `xenobot/utils/chatprompt/chatprompt.go`, `xenobot/utils/tracing/processor.go`
- **Library Contracts (HIGH confidence — `go doc` against installed `go.mod` `gotgbot/v2`):**
  - `handlers.NewReaction`, `handlers.NewPoll`, `filters/reaction`, `filters/poll`, `gotgbot.MessageReactionUpdated`, `gotgbot.Poll`, `gotgbot.ChatFullInfo.AvailableReactions`, `gotgbot.Bot.SendPoll`
- **Project & Spike Decisions (HIGH confidence):**
  - `.planning/PROJECT.md`, `.planning/notes/v1.3-interactive-presence.md`, `.planning/spikes/MANIFEST.md` (Spikes 001–003 `VALIDATED` 2026-11-26), `.planning/research/questions.md`

---
*Architecture research for: Xenobot-72 Milestone v1.3 Interactive Presence*
*Researched: 2026-11-26*
