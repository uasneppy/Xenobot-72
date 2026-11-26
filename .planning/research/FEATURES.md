# Feature Research: Xenobot-72 v1.3 Interactive Presence

**Domain:** Interactive AI & community-presence features for an active Telegram group-management bot (Go / gotgbot v2 / PostgreSQL / Redis)
**Researched:** 2026-11-26
**Confidence:** HIGH (grounded in Xenobot-72 codebase inspection, validated Spikes 001–003 across Claude Haiku / Gemini 4 / Featherless, and cross-checked against Telegram Bot API constraints and community bot UX patterns)

---

## Feature Landscape

### Table Stakes (Users Expect These)

Features required so interactive presence feels native, safe, and non-annoying in active Telegram groups. Missing any of these makes the bot feel spammy, broken, or intrusive.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **`"message_reaction"` update foundation** | Shared event substrate for Emoji Consensus, `👎` retraction, Highlights Archive, and `✍` PM brief; Telegram only delivers individual user reaction diffs when `"message_reaction"` is explicitly in `AllowedUpdates` | LOW | Add `"message_reaction"` to `AllowedUpdates` in `xenobot/config/config.go` and wire a single `handlers.NewReaction` dispatcher entry. Remember: `MessageReactionUpdated` carries no message text/body (no `getMessage` in Bot API), so downstream features must use Redis aggregation, `ForwardMessage`/`CopyMessage`, or `chat_log.go`. |
| **Strict anti-annoyance & safety gate** | Active groups quickly mute or kick bots that react every 2 messages, chime in during moderator arguments, or react to tragedy/war news | MEDIUM | Hard local pre-filter before any AI call: skip private chats, Staff Groups (`staff.IsStaffGroup`), active lockdowns (`lockdown.GetActive`), bots, anonymous channels, slash commands, and messages already reacted to by static `reactions.go` (`ctx.Data` marker). Cross-replica Redis throttle: message-gap counter (15–30 msgs), per-chat cooldown, per-user cooldown, random probability roll, dedicated hourly budget (`chatReserveWith`), frequency presets (`Off / Low / Medium`). Keep `IsBig: false`. |
| **Per-chat `AvailableReactions` intersection** | Telegram rejects disabled or non-standard emojis with `400 REACTION_INVALID`; groups often curate their reaction palette | LOW | Cache `ChatFullInfo.AvailableReactions` in Redis (15–30 min TTL; omitted field = all 73 standard emojis). Always normalize by stripping `U+FE0F` and intersect against `supportedReactionEmoji` ∩ `AvailableReactions`. Never trip AI circuit breakers on Telegram `400` (`REACTION_INVALID` or deleted message). |
| **`👎` Retraction (Zero-AI)** | Members need a silent, frictionless "read the room / back off" signal when a bot reaction misses the mark | LOW | Track bot-reacted `(chatID, msgID)` in Redis (24h TTL). When the message author or a group admin adds `👎`, silently clear the bot's reaction (`Reaction: nil`) and extend that user's reaction cooldown (e.g., 2–6 hours). Never reply publicly in the chat. |
| **Owner-unlock + Admin-toggle permission boundary** | AI features incur API cost and send group messages to an external provider (data-disclosure notice required), whereas zero-AI community toys should not require owner intervention | MEDIUM | **Locked-in decision:** Owner unlocks AI/Conversations per group on `OwnerOnly` page `p` (2-tap data disclosure card); once unlocked, admins (when Access allows) can toggle individual AI interactive features on/off. Zero-AI features (Consensus, `👎` retraction, smarter static reactions, cat-sticker triggers, Highlights) are admin-configurable independently. |
| **Private Settings UI & 8-locale parity** | Xenobot-72 v1.1/v1.2 established that all group configuration lives in private-chat inline menus across all 8 locales (`uk`, `en`, `es`, `fr`, `hi`, `id`, `pt`, `ru`) with 64-byte callback budgets | MEDIUM | Conversations page `p` is already at 7/8 rows (`settings_chat_test.go:567`). Add a dedicated private-only `Interactivity` page (non-OwnerOnly, hidden in Staff Groups) with sub-screens so every screen stays ≤ 8 rows, ≤ 3800 UTF-16 chars, and ≤ 40-rune button labels across all 8 locales. |
| **Smarter static reactions (`reactions.go`)** | Legacy substring matching (`strings.Contains`) triggers false positives inside unrelated words (e.g. `"cat"` in `"category"`), ignores photo captions, and fires on 100% of matches with zero cooldown | MEDIUM | Add optional word-boundary matching (Unicode-aware `\b` / non-letter boundaries so Cyrillic/Ukrainian works properly), caption support, trigger chance % (1–100%), per-rule/per-chat cooldowns, Staff Group/lockdown skip guards, and `ctx.Data` handoff marker in group `8` so AI reactions never overwrite static ones. |
| **Frictionless catch-up (`/unrollshort <N\|me\|2h> [pm]`)** | Having to scroll up 120 messages just to reply to a message before typing `/unrollshort` breaks the core catch-up UX | MEDIUM | Add backward-compatible `At int64` (`json:"at,omitempty"`) to `chatLogEntry` in `chat_log.go`. Support `/unrollshort <N>` (e.g. `10`–`200`), `/unrollshort me` (since requester's last logged message in the same thread), `/unrollshort <duration>` (e.g. `30m`, `2h`, capped at `24h`), and trailing `pm` flag reusing `chatJudgeDeliverPrivate`. Preserve legacy reply-to-message behavior unchanged. |
| **Graceful degradation & silent failure** | Ambient features must never spam error messages into group chat when Redis hiccups, a provider rate-limits, a user hasn't `/start`ed the bot, or a target message is deleted | LOW | Ambient/reaction paths fail silently (increment `/stats` counters only, zero group replies). Explicit commands (`/unrollshort`, `/referendum`, `/inspect`) keep existing gated malfunction/cooldown notices. All new AI work reuses `StopChat` (`chatJobs` + `chatContext`) — zero new shutdown drains. |

---

### Differentiators (Competitive Advantage)

Features that give Xenobot-72 genuine, memorable character ("alive, witty, context-aware") while staying strictly in persona and far quieter than typical noisy AI bots.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Contextual Random AI Emoji Reactions (with Vision)** | Instead of robotic keyword matching, Xenobot-72 reacts like a dry, observant regular — including to memes, photos, and stickers — while staying silent (`NONE`) on mundane chatter, drama, and tragedies | HIGH | Validated in Spikes 001–003: use JSON wire format (`{"emoji":"..."}` / `{"emoji":"NONE"}`) with **`MaxTokens = 512`** (prevents Gemini 4 `thinkingLevel: "low"` and Featherless Qwen3 `<think>` truncation). Offer all 73 standard emojis ∩ `AvailableReactions` (including spicy ones `🤡`, `💩`, `🤮`, `🖕`, `🤬` only when earned). Prompt must explicitly warn against non-Telegram emojis (`😂`, `🐱`, `😹`, `😻`, `😼`). Context: last 3–5 same-thread `chat_log` entries + free `ReplyTarget`/`Quote` + photo/sticker thumbnail via `chat_media.go` on vision providers (degrades cleanly to caption/sticker-emoji on text-only Featherless). |
| **Emoji Consensus (Zero-AI)** | Joins organic community moments ("everyone is dropping 🔥 or 💀/🤣 on this banger") at zero AI cost | LOW | Aggregate `(chatID, msgID, emoji) -> set of distinct non-bot userIDs` in Redis with TTL (e.g. 6–24h). When ≥ 3 distinct members use the same standard emoji (intersected with `AvailableReactions`, excluding `👎` and opt-out negative/pile-on emojis by default), roll a probability check (e.g. 35–50%) behind a per-chat cooldown and add that exact emoji. Never overrides an existing bot reaction on that message. |
| **Cat-Sticker Micro-Triggers (Zero-AI)** | Deeply in-character Easter egg tied to Xenobot-72's canonical cat-collection persona, running on Telegram's free `msg.Sticker.Emoji` metadata with zero vision/AI spend | LOW | Match `msg.Sticker != nil` where `msg.Sticker.Emoji` contains a cat glyph (`🐱🐈😺😸😹😻😼😽🙀😿😾`). Behind a probability roll (e.g. 20–35%) and per-chat/per-user cooldown, stamp `👀` or `❤` (intersected with `AvailableReactions`). Skips Staff Groups, lockdowns, and messages already reacted to. |
| **Crowd-Picked Highlights ("Facility Archive")** | Turns organic group reactions into a curated best-of archive channel or forum topic without admins manually forwarding quotes | HIGH | Zero-AI Redis `SADD`/`SREM` aggregation of distinct non-bot reactors per `(chatID, msgID)` (excluding the message author's self-reactions and negative reactions like `👎`/`💩`/`🤮`/`🖕`/`🤬` by default). On crossing the configured threshold (default 5, configurable 3–25), atomically claim a one-time `SETNX` posted marker in Redis + persist mapping in DB so concurrent reactions across replicas never double-post. `ForwardMessage` (or `CopyMessage` fallback when author hides forward link) + `t.me/c/<internal_id>/<msg_id>` jump button into a linked channel or forum topic. Linking requires live admin proof (like `/setlog`). |
| **Ambient Room Context in Conversations** | When a member tags `@Xenobot` with `"what do you think?"` or `"agree?"`, the bot actually knows what the group was just discussing instead of asking for context | MEDIUM | When `LogEnabled` is on and Ambient Context toggle is enabled, inject the last 6–10 same-thread entries from `chat_log.go` into `chatBuildRequest` (`chat_run.go`) inside a clearly fenced `<room_context>` block separate from `chatReadHistory` direct turns, instructing the model to treat it as background context only. |
| **Silent `✍` React-to-Brief in PM** | Zero-clutter catch-up: a member scrolls back to where they left off, taps `✍` on that message, and silently receives a private `/unrollshort` summary in PM without polluting the group with a `/unrollshort` command | MEDIUM | Triggered from `handlers.NewReaction` when a non-bot member adds `✍` to a message present in `chat_log.go` (and `LogEnabled` + AI unlocked + feature enabled). Reuses the brief queue, cooldown, and `chatJudgeDeliverPrivate` without posting *any* group reply on success or `403` failure (if the user hasn't `/start`ed the bot, silently shed so the group never gets spammed with "start me in PM" errors!). |
| **Opt-In Vocative Name-Addressing** | Natural conversational trigger (`"Xenobot, explain this"` / `"Кзенобот, ..."`), far less intrusive than an open-mic follow-up timer | LOW | Opt-in toggle (off by default). Strict start-of-message vocative regex: optional greeting/filler excluded; must begin with `Xenobot` / `Кзенобот` (case-insensitive) followed immediately by `,`, `:`, `!`, `?`, or whitespace + non-empty text (never matches mid-sentence mentions like `"I think Xenobot broke"`). Passes through the exact same `chatAccept` gate, audience check, and rate limits as `@mention`. |
| **AI-Drafted Native Polls (`/referendum`) + Tied-Poll Tie-Breaker** | Turns messy group debates into crisp, witty 1-tap Telegram native polls and resolves deadlocks automatically in character | HIGH | Reads recent same-thread `chat_log` (or replied message + surrounding stretch), prompts AI for structured JSON (`question` ≤ 300 chars, 2–5 `options` ≤ 100 chars, optional tongue-in-cheek last option), validates lengths, and posts via `SendPoll` (regular, non-anonymous by default or configurable, bounded `open_period` e.g. 1h–24h or manual close). Stores `poll_id -> (chatID, msgID, threadID)` in Redis (TTL = `open_period` + 1h, max 7d). Reuses existing `"poll"` update in `AllowedUpdates`: when a tracked bot poll transitions to `IsClosed == true` and has a tie for 1st place with ≥ 2 total votes, posts one deadpan 1-line tie-breaker verdict replying to the poll. |
| **Playful `/inspect` (Member Sibling to `/judge`)** | Low-stakes community toy: members reply to a meme, desk setup, hot take, or meal photo with `/inspect` and get a punchy 2–3 line "Facility Inspection" rating plus a matching reaction emoji | MEDIUM | Member-accessible (governed by Conversations audience or dedicated admin toggle, disableable via `helpers.WrapCommand`). Supports text + attached photo/sticker via `chat_media.go` on vision providers. Structured JSON output returns both the short inspection verdict (2–3 lines, strict char cap) and one standard reaction emoji (validated against `supportedReactionEmoji` ∩ `AvailableReactions`) stamped onto the target message. Strict per-user cooldown (e.g. 60s) and dedicated hourly cap so groups don't turn into `/inspect` spam. |

---

### Anti-Features (Commonly Requested, Often Problematic)

Features that look fun on paper but reliably ruin active Telegram communities or violate Xenobot-72's architectural invariants.

| Anti-Feature | Why Requested | Why Problematic | Alternative |
|--------------|---------------|-----------------|-------------|
| **Open-mic / follow-up timer ("keep replying for 2 minutes without tagging")** | Feels like a human joining a chat thread | Catastrophically noisy in active groups: bot hijacks side conversations between humans, burns token budgets rapidly, and makes members feel spied on | **Opt-in Vocative Name-Addressing** (`"Xenobot, ..."`) + **Ambient Room Context** (bot reads the room silently and only speaks when explicitly addressed) |
| **Reacting to every message or evaluating every message with AI** | "More reactions = smarter bot" | Massive API cost, 429 rate limits, and extreme user annoyance; violates `PROJECT.md` Out of Scope ("Sending every message to an AI model") | **Strict local pre-gate** (15–30 msg gap + cooldowns + probability roll + `NONE` default) so AI is only called on a tiny sampled fraction of notable messages |
| **Public error messages on reaction-triggered actions (`✍` PM brief, Consensus, AI reactions)** | "Tell the user why their `✍` didn't work" | If 10 users tap `✍` or `👎` without starting the bot, posting `"Please start me in PM first"` turns a silent reaction into group spam | **100% silent degradation** on all reaction-triggered paths; users who want visible feedback use explicit `/unrollshort pm` |
| **Self-reacting / negative-emoji farming into Highlights Archive** | "Count every reaction emoji equally" | Users boost their own jokes, or trolls brigade a bad take with 5× `💩`/`👎` to immortalize drama in the Highlights channel | **Exclude message author's own reactions** and **exclude negative/hostile emojis** (`👎`, `💩`, `🤮`, `🖕`, `🤬`, `😡`) from Highlights threshold by default |
| **Big reaction animations (`IsBig: true`)** | "Makes bot reactions pop visually" | Full-screen confetti/explosion animations triggered randomly by a bot feel obnoxious and distracting | Always pass **`IsBig: false`** on every automated reaction |
| **Auto-importing or copying Highlights destination chat IDs or AI enablement across groups** | "One-click clone everything to my new group" | Security & privacy violation: Group B could leak/post into Group A's archive channel without admin proof, or silently enable paid AI + third-party data transfer without Owner's 2-tap disclosure | Mirror `importLogChannels` (never import destination chat/topic ID) and `TestChatConversationsStayOutOfBackup` (never auto-enable `chat_conversations.Enabled` or `LogEnabled` on import/copy) |
| **Auto-deleting Highlights posts when users un-react or syncing live reaction counts via continuous edits** | "Keep starboard count badge updated live like Discord" | Every extra reaction edit burns Telegram's 20 msg/min group/channel edit budget, causes 429 storms on viral messages, and fights Telegram's forwarded-message immutability (forwarded messages cannot be edited!) | **One-time immutable archive post** ("Facility Archive") with optional jump button; no continuous counter-editing loop |
| **Allowing interactive presence in Staff Groups or during active Lockdowns** | "Why not have fun everywhere?" | Staff Groups are sensitive moderation war rooms (and never send messages to AI for reactions/toys); during a raid lockdown, every Telegram API call must be reserved for defense | Explicitly guard **all** v1.3 watchers and commands with `staff.IsStaffGroup` and `lockdown.GetActive` checks (matching `PROJECT.md` guardrails) |
| **Custom/Premium emoji reactions from the bot** | "Let the bot use our group's custom stickers/emojis" | Telegram Bot API forbids bots from setting paid reactions and restricts custom emojis unless already present or explicitly allowed; breaks portability across groups | Restrict strictly to Telegram's **73 standard emojis** (`supportedReactionEmoji` ∩ `ChatFullInfo.AvailableReactions`) |

---

## Anti-Annoyance Mechanics & UX Flows

### 1. Per-Feature Community Etiquette & Throttling Defaults

| Feature | Default State | Anti-Annoyance Guardrails | Silent Failure Rule |
|---------|---------------|---------------------------|---------------------|
| **Contextual AI Reactions** | `Off` (presets: `Off` / `Low` / `Medium`) | `Low`: gap ≥ 25 msgs, chat CD 10 min, user CD 30 min, 25% roll, ≤ 6/hr/chat. `Medium`: gap ≥ 15 msgs, chat CD 5 min, user CD 15 min, 40% roll, ≤ 12/hr/chat. Always `NONE` on mundane/drama/tragedy. `IsBig: false`. | 100% silent on any AI/parse/Telegram error or budget hit. |
| **Emoji Consensus** | `Off` (admin toggle) | Threshold ≥ 3 distinct non-bot members with identical standard emoji within 30 min; 40% join probability; 10 min chat cooldown; skips negative emojis (`👎`, `💩`, `🤮`, `🖕`, `🤬`, `😡`) and messages bot already reacted to. | 100% silent. |
| **`👎` Retraction** | `On` when any bot auto-reaction is used (admin toggleable) | Triggers only for message author or live group admin on messages in the 24h bot-reacted Redis set; clears bot reaction (`Reaction: nil`) and sets 3-hour per-user auto-reaction cooldown. | 100% silent (no "sorry!" reply). |
| **Smarter Static Reactions** | Existing rules unchanged; new options default backward-compatibly | Per-rule or chat defaults: word-boundary (`off` for legacy rules, `on` recommended for new), match captions (`on`), chance % (`100%` legacy, configurable `10–100%`), cooldown (`0s` legacy, configurable `0s / 30s / 2m / 5m / 15m`). Marks `ctx.Data` in group `8`. | 100% silent. |
| **Cat-Sticker Micro-Triggers** | `Off` (admin toggle) | Only `msg.Sticker != nil` with cat `Sticker.Emoji`; 30% roll; 10 min chat cooldown / 20 min user cooldown; stamps `👀` or `❤` ∩ `AvailableReactions`. | 100% silent. |
| **Highlights Archive** | `Off` (requires linked destination) | Default threshold `5` distinct non-bot members (configurable `3–25`); ignores message author's self-reactions and negative emojis by default; 24h aggregation window; one-time `SETNX` + DB dedup; max 1 archive post per 30s per chat; skips Staff Groups & lockdowns. | 100% silent in main chat; logs error class only. |
| **Ambient Room Context** | `On` once Owner enables both Conversations + Message Log (admin toggleable) | Injects last 8 (6–10) same-thread `chat_log` entries inside fenced ambient context block; excludes the current message itself; obeys existing `chat_log` 400-rune truncation. | Silently omits ambient block if Redis is unavailable. |
| **`/unrollshort` / `/unrolllong` Catch-Up** | Available whenever `LogEnabled` is on | `<N>` clamped `5..200`; `<duration>` clamped `5m..24h`; `me` scans same thread back to requester's last logged message (if none found, replies with clear localized hint, spends no AI budget). Optional trailing `pm` delivers to PM via `chatJudgeDeliverPrivate` with `judge_pm_blocked` fallback. | Explicit command errors localized; `pm` success leaves a brief confirmation or delivers cleanly. |
| **`✍` React-to-Brief in PM** | `On` when `LogEnabled` + AI unlocked (admin toggleable) | Triggers on `✍` reaction added by a real non-bot member to a message ID present in `chat_log`; shares brief cooldown (20s) and user/chat/global hourly brief budgets; sends brief privately with group title + jump link. | **100% silent in group** even on `403` (user never started bot) or budget/cooldown hit. |
| **Vocative Name-Addressing** | `Off` (admin toggle, requires AI unlocked) | Start-of-message only: `^(?i)(xenobot|кзенобот)([,:!?\s]+.+)$` (min 2 chars after name); ignores slash commands, bots, Staff Groups, lockdowns; shares `chatAccept` audience & cooldown. | Silent if gated/cooldown (exact same as `@mention`). |
| **`/referendum` + Tie-Breaker** | `Off` (admin toggle, requires AI unlocked; command disableable) | Member or admin command (configurable or follows Conversations audience); strict cooldown (5 min per chat, 10 min per user) and dedicated hourly cap (e.g. 4/hr/chat) so groups can't spam polls; default `open_period` 1h (or manual close); tie-breaker fires at most once per closed bot poll with ≥ 2 total votes. | Explicit command notices for cooldown/missing context; tie-breaker fails silently if poll message was deleted. |
| **`/inspect`** | `Off` (admin toggle, requires AI unlocked; command disableable) | Reply-required (message, caption, photo, or sticker); 60s per-user cooldown, 15s per-chat cooldown, dedicated hourly cap (e.g. 15/hr/chat); outputs 2–3 lines + 1 validated reaction emoji on target message. | Explicit usage/cooldown reply on command; reaction stamp fails silently if `REACTION_INVALID`. |

---

### 2. Settings Menu UX & Callback Architecture

Xenobot-72's settings framework (`settings_pages.go`, `settings_menu.go`, `settings_chat.go`) enforces three strict invariants that shape v1.3's menu layout:
1. **`settingsPage.OwnerOnly`** hides a page completely from non-owner admins on Main and refuses `openPage`. Page `p` (Conversations) MUST remain `OwnerOnly: true, PrivateOnly: true` because it guards AI enablement (2-tap data disclosure card), provider selection, and `LogEnabled`.
2. **8-row keyboard cap** (`settings_chat_test.go:567`): Page `p` currently uses up to **7 rows** (toggle + 2 provider rows + audience + message log + judge + nav), leaving **exactly 1 row** available.
3. **64-byte callback budget** & **40-rune button label cap** across all 8 locales (`uk`, `en`, `es`, `fr`, `hi`, `id`, `pt`, `ru`).

#### Recommended Settings Structure

- **Keep Page `p` (`🗨 Conversations`, `OwnerOnly: true, PrivateOnly: true`, Order 89) intact**, adding at most **1 row** (`✨ Interactivity →`) as a direct jump to the new Interactivity page for the owner (still ≤ 8 rows).
- **Add Page `in` (`✨ Interactivity`, `OwnerOnly: false, PrivateOnly: true, Hidden: isStaffGroup`, Order 75 — right beside Reactions `73` and Messages `74`)**:
  - Accessible in private chat to the Owner AND to group admins when Access (`a`) allows admins (`settingsRightChangeInfo` or `settingsRightAdmin`).
  - Uses compact sub-screens (just like Messages `h` and Member Requirements `m`) so every screen stays **≤ 7–8 rows**:
    1. **Hub (`in`)**: Shows high-level status of AI lock state (`🔓 AI unlocked` / `🔒 Owner must enable Conversations first`) plus 3–4 category buttons:
       - `🎭 AI Presence` (sub-screen `ina`) — Contextual AI Reactions (`Off / Low / Medium`), Ambient Room Context (`On/Off`), Vocative Addressing (`On/Off`), `✍` PM Brief (`On/Off`), `/referendum` (`On/Off`), `/inspect` (`On/Off`). If Owner has NOT enabled Conversations, buttons display a locked state or alert `settings_interactivity_ai_locked` and refuse writes.
       - `🤝 Reaction Social` (sub-screen `ins`) — Emoji Consensus (`On/Off` + threshold stepper `3..10`), `👎` Retraction (`On/Off`), Cat-Sticker Triggers (`On/Off`). Always editable by admins regardless of AI status.
       - `⭐ Facility Archive` (sub-screen `inh`) — Shows link status (`Linked` / `Not linked`, never revealing channel title/ID in group menus; private-only page shows linked status and unlink two-tap card), threshold stepper (`3..25`), emoji filter (`Positive only` / `All standard`), and instructions to link via `/setarchive` (mirroring `/setlog` channel-proof flow).
       - `👍 Static Reactions →` (jumps to existing page `x`, or configure static reaction defaults on `inr`: default word-boundary, captions, chance %, cooldown).
- **Enhance Page `x` (`👍 Reactions`, existing list engine `settings_reactions.go`)**:
  - Keep existing `keyword emoji` 2-token typed add 100% backward-compatible, while allowing optional trailing flags in `/addreaction` (e.g. `/addreaction cat 👀 --word --chance=50 --cd=60`) and/or a `⚙ Defaults` button on page `x` (or under `in`) for chat-wide static defaults (word boundary, match captions, chance %, cooldown) so the list engine's 8-per-page UI stays clean and uncluttered.

#### New Commands & Linking Flow

All new commands register via `helpers.WrapCommand` with `Disableable: true`, `CheckDisabled`, and `RequireGroup`:
- `/referendum [topic]` — Draft & post a native AI poll from recent thread context or replied debate.
- `/inspect` (reply to message/photo/sticker) — Playful 2–3 line Facility Inspection + matching reaction emoji.
- `/setarchive` / `/unsetarchive` — Admin-only channel/forum-topic linking for Crowd-Picked Highlights, mirroring `/setlog` (forward a message from the target channel or run inside the target forum topic with a live bot-admin + user-admin proof check so no one can hijack someone else's channel).
- Extended `/unrollshort <N|me|2h> [pm]` and `/unrolllong <N|me|2h> [pm]` — Backwards-compatible with reply-to-message invocation.
- Extended `/addreaction` — Backwards-compatible with `/addreaction <keyword> <emoji>`, adding optional modifiers (`word`, `chance`, `cd`).

---

### 3. Backup & Copy Settings Parity Audit (Phase 29 / Phase 30)

Directly answers `.planning/research/questions.md` #1 based on auditing `xenobot/db/backup/backup.go`, `xenobot/db/backup/chatconv_backup_test.go`, `xenobot/db/backup/types.go`, and `xenobot/modules/copy_settings.go`:

| v1.3 Setting / Data | Storage | `/export` & `/import` (Phase 30) | Reset (`ClearChatData`) | Copy Settings (Phase 29 `copy_settings.go`) | Security / Rationale |
|---------------------|---------|----------------------------------|-------------------------|---------------------------------------------|----------------------|
| **Owner AI Enablement** (`chat_conversations`: `Enabled`, `Provider`, `Audience`, `LogEnabled`, `JudgeTo`) | Existing `chat_conversations` | **EXCLUDE** (unchanged) | **KEEP UNTOUCHED** (unchanged) | **EXCLUDE** | Preserves `TestChatConversationsStayOutOfBackup` invariant: an import or copy must NEVER bypass the Owner's 2-tap data disclosure confirmation or silently incur AI API billing. |
| **Smarter Static Reaction fields** (`word_boundary`, `match_caption`, `chance_pct`, `cooldown_secs`) | Extended `reactions` table (or `reactions` + chat defaults) | **INCLUDE** inside existing `BackupModuleReactions` (`"reactions"`) | **CLEAR** with `reactions` | **EXCLUDE** (Copy Settings explicitly excludes content lists like filters, notes, blacklist words, reactions) | Old v1.0–v1.2 JSON backups missing the new fields decode cleanly to legacy defaults (`word_boundary=false`, `chance_pct=100`, `cooldown_secs=0`). |
| **Zero-AI Presence settings** (Emoji Consensus on/off & threshold, `👎` retraction on/off, cat-sticker triggers on/off, static reaction chat defaults) | New `chat_interactivity` table | **INCLUDE** in new `BackupModuleInteractivity` (`"interactivity"`) | **CLEAR** when `"interactivity"` is reset | **OPTIONAL / RECOMMEND EXCLUDE from v1.2's 6 moderation areas** (or defer to a separate copy category) | Copy Settings (`settings_copy.go`) is documented and tested as a mirror of the **6 moderation & event-message tuning areas** (`messages`, `warns`, `antiflood`, `blacklists`, `escalation`, `moddefaults/thresholds`). Keeping Copy Settings focused avoids surprising admins who copy moderation rules and suddenly inherit fun toys. |
| **Admin AI Feature Toggles** (AI reactions frequency, Ambient context, Vocative addressing, `✍` PM brief, `/referendum`, `/inspect`) | New `chat_interactivity` table | **INCLUDE as dormant preferences** in `BackupModuleInteractivity` (`"interactivity"`) | **CLEAR** to `Off` when `"interactivity"` is reset | **EXCLUDE** | Importing preferences into a group whose Owner has NOT unlocked AI stores the toggle preferences safely while keeping every AI feature **100% inert** at runtime until the target Owner confirms the 2-tap data notice on page `p`. Validate all enum bounds on import (editor-equivalent validation). |
| **Highlights Archive target** (`archive_chat_id`, `archive_thread_id`) | New `chat_highlights_settings` (or columns on `chat_interactivity`) | **NEVER IMPORT TARGET ID** (mirror `importLogChannels`!) | **CLEAR** link + settings on reset | **EXCLUDE** | Identical security rule to `xenobot/db/backup/backup.go:1285`: a backup names the source group's channel; linking requires live admin proof (`/setarchive`). On import, apply threshold & emoji-filter flags **only** if target already has a linked archive channel; otherwise skip and set `report.ArchiveChannelSkipped = true`. |
| **Runtime ephemeral state** (Redis reaction counts, bot-reacted 24h IDs, cooldowns, active poll map, `chat_log` ring) | Redis only (outside `xenobot:cache:`) | **EXCLUDE** | **N/A** (ephemeral TTL) | **EXCLUDE** | Ephemeral operational state never belongs in backups. |

---

## Feature Dependencies

```
[AllowedUpdates: "message_reaction" + handlers.NewReaction]
    ├──required by──> [👎 Retraction (Zero-AI)]
    ├──required by──> [Emoji Consensus (Zero-AI)]
    ├──required by──> [Crowd-Picked Highlights / Facility Archive]
    └──required by──> [Silent ✍ React-to-Brief in PM]
                          └──requires──> [Existing chat_log.go (LogEnabled)]
                                             └──requires──> [Existing Owner AI Unlock (page p)]

[Per-Chat AvailableReactions Redis Cache + supportedReactionEmoji]
    ├──required by──> [Contextual Random AI Emoji Reactions]
    │                     ├──requires──> [Existing chat_media.go (Vision)]
    │                     ├──requires──> [Existing StopChat pool + chatReserveWith]
    │                     ├──requires──> [Existing Owner AI Unlock (page p)]
    │                     ├──enhanced by──> [Existing chat_log.go (3-5 thread entries)]
    │                     └──de-conflicted with──> [Static reactions.go (ctx.Data marker in group 8)]
    ├──required by──> [Emoji Consensus (Zero-AI)]
    ├──required by──> [Cat-Sticker Micro-Triggers (Zero-AI)]
    └──required by──> [Playful /inspect (matching reaction emoji)]

[chatLogEntry.At int64 Timestamp Addition]
    └──required by──> [Frictionless /unrollshort <N|me|2h> [pm]]
                          └──reuses──> [Existing chatJudgeDeliverPrivate]

[Existing chat_log.go (LogEnabled)]
    ├──required by──> [Ambient Room Context in Conversations (chat_run.go)]
    └──enhances──> [/referendum AI Polls]
                       └──required by──> [Tied-Poll Tie-Breaker (existing "poll" update)]

[New Interactivity Settings Page (page in) + chat_interactivity DB table]
    ├──governs──> [All v1.3 Admin Toggles (Zero-AI + AI-locked)]
    └──integrates with──> [BackupModuleInteractivity (Phase 30 backup parity)]
```

### Dependency Notes

- **`"message_reaction"` foundation must ship first (Phase A):** Four independent features (`👎` retraction, Emoji Consensus, Highlights Archive, `✍` PM brief) all consume `MessageReactionUpdated`. Building one clean reaction router + shared `AvailableReactions` cache avoids duplicate handlers and conflicting Redis schemas.
- **Static `reactions.go` (group `8`) coordinates with AI Reactions (group `13+`):** Because Telegram limits bots to **1 reaction per message**, static reactions (group `8`) must stamp a `ctx.Data` key when they react so the later contextual AI reaction watcher (group `13+`) and cat-sticker watcher immediately skip that message without wasting a roll or overwriting the static reaction.
- **Zero shutdown-budget growth:** Every new AI path (Contextual AI Reactions, `✍` PM brief, `/referendum`, `/inspect`, Tie-breaker if AI-assisted) MUST run inside the existing `chatJobs` / `chatBriefQueue` pools under `chatContext()`, keeping `StopChat` (2s) and the 70s webhook worst case (`stop_grace_period: 75s`) completely unchanged.
- **Explicit Staff Group & Lockdown guards:** Unlike moderation watchers in earlier groups, neither `chat_watch.go`, `chat_log.go`, nor `reactions.go` currently checks `staff.IsStaffGroup` or `lockdown.GetActive`. Every v1.3 ambient watcher and interactive command must check both explicitly before acting.
- **Owner Unlock vs Admin Toggle:** `chat_interactivity` stores admin toggles separately from `chat_conversations`, so `TestChatConversationsStayOutOfBackup` stays green and page `p` retains its strict `OwnerOnly: true` boundary while admins get full access to page `in`.

---

## MVP Definition

Because v1.3 is a **subsequent milestone** with all 9 features already scoped and approved across 3 phases (`v1.3-interactive-presence.md`), the rollout phases act as clean vertical slices:

### Launch Slice 1 — Phase A: Reaction & Social Foundation
- [ ] `"message_reaction"` in `AllowedUpdates` + `handlers.NewReaction` router + `AvailableReactions` Redis cache (15–30 min TTL)
- [ ] `chat_interactivity` schema + private Settings page `in` (Owner-unlock check + Admin toggles) + Staff Group / Lockdown safety guards
- [ ] Contextual Random AI Emoji Reactions (text, captions, vision via `chat_media.go`, `json` + `MaxTokens = 512`, all 73 emojis ∩ `AvailableReactions`, strict anti-annoyance throttle, `Off / Low / Medium`)
- [ ] Emoji Consensus (Zero-AI, ≥ 3 distinct members)
- [ ] `👎` Retraction (Zero-AI, author/admin silent retract + extended cooldown)

### Launch Slice 2 — Phase B: Community & Zero-AI Polish
- [ ] Smarter Static Reactions (`reactions.go`: word-boundary, captions, chance %, cooldowns, `ctx.Data` handoff marker, backward-compatible `BackupModuleReactions`)
- [ ] Zero-AI Cat-Sticker Micro-Triggers (`msg.Sticker.Emoji` → occasional `👀`/`❤`)
- [ ] Crowd-Picked Highlights / "Facility Archive" (Redis distinct-member aggregation, `/setarchive` live-proof linking, `ForwardMessage`/`CopyMessage` + `t.me/c/` jump button, idempotent dedup, `BackupModuleInteractivity` with `importLogChannels`-style target protection)

### Launch Slice 3 — Phase C: Room-Aware Conversations & Group Toys
- [ ] Ambient Room Context in Conversations (last 6–10 same-thread `chat_log` entries in `chatBuildRequest`)
- [ ] Frictionless Catch-Up (`At int64` in `chatLogEntry`, `/unrollshort` & `/unrolllong` `<N|me|2h> [pm]`) + Silent `✍` React-to-Brief in PM
- [ ] Opt-in Vocative Name-Addressing (`"Xenobot, ..."` / `"Кзенобот, ..."`)
- [ ] `/referendum` AI-drafted native polls + deadpan tied-poll tie-breaker (`"poll"` updates)
- [ ] Playful `/inspect` (member sibling to `/judge` + matching reaction emoji)

### Future Consideration (v1.4+)
- **Group-specific custom persona snippets / inside jokes** — defer until v1.3 ambient room context proves how well live context covers group culture without prompt bloat.
- **Highlights media-group (album) bundling** — defer unless active groups frequently highlight multi-photo albums; single-message forward/copy covers 95% of highlights cleanly without album-buffer complexity.
- **Per-reaction custom emoji allow-list editor for Highlights** — v1.3's `Positive only / All standard` binary switch covers community needs cleanly without complex emoji-picker UIs inside Telegram's 64-byte callback budget.

---

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority | Target Phase |
|---------|------------|---------------------|----------|--------------|
| `"message_reaction"` foundation + `AvailableReactions` cache | HIGH | LOW | P1 | Phase A |
| Anti-annoyance throttle + Staff/Lockdown safety guards | HIGH | MEDIUM | P1 | Phase A |
| Owner-unlock + Admin-toggle Settings page (`in`) | HIGH | MEDIUM | P1 | Phase A |
| Contextual Random AI Emoji Reactions (with Vision) | HIGH | HIGH | P1 | Phase A |
| `👎` Retraction (Zero-AI) | HIGH | LOW | P1 | Phase A |
| Emoji Consensus (Zero-AI) | MEDIUM | LOW | P1 | Phase A |
| Smarter Static Reactions + `ctx.Data` handoff | MEDIUM | MEDIUM | P1 | Phase B |
| Cat-Sticker Micro-Triggers (Zero-AI) | MEDIUM | LOW | P2 | Phase B |
| Crowd-Picked Highlights ("Facility Archive") | HIGH | HIGH | P1 | Phase B |
| Backup Parity (`BackupModuleInteractivity` + `Reactions` schema) | MEDIUM | MEDIUM | P1 | Phase B |
| Ambient Room Context in Conversations | HIGH | MEDIUM | P1 | Phase C |
| Frictionless `/unrollshort <N\|me\|2h> [pm]` + `✍` PM Brief | HIGH | MEDIUM | P1 | Phase C |
| Opt-in Vocative Name-Addressing | MEDIUM | LOW | P2 | Phase C |
| `/referendum` AI Polls + Tied-Poll Tie-Breaker | MEDIUM | HIGH | P2 | Phase C |
| Playful `/inspect` | MEDIUM | MEDIUM | P2 | Phase C |

**Priority key:**
- P1: Core milestone value or foundational guardrail
- P2: High-character differentiator included in v1.3 scope

---

## Competitor Feature Analysis

| Feature | GroupHelp / MissRose | Combot / Telegram Starboard Bots | Discord AI & Starboard Bots | Xenobot-72 v1.3 Approach |
|---------|----------------------|----------------------------------|-----------------------------|--------------------------|
| **Auto-reactions** | Simple static keyword → emoji triggers (substring, 100% fire rate, easily noisy) | Rare or static triggers only | Random/keyword emoji reactions, often spammy without gap gating | **Hybrid two-tier:** smarter static rules (word-boundary, captions, chance %, cooldowns) + sampled contextual AI reactions with vision, `NONE` abstention, and `👎` retraction |
| **Starboard / Highlights** | Not built into Rose/GroupHelp | Standalone starboard bots often suffer from self-react farming, 429 edit storms, or missing forum-topic support | Starboard bots edit star counts continuously (viable on Discord, rate-limited on Telegram) | **Zero-AI Facility Archive:** Redis distinct-member set, excludes author & negative emojis, one-time immutable forward/copy + jump link, `/setarchive` live-proof linking |
| **Catch-up / Summaries** | Not available | Basic stats/analytics, no AI briefs | Slash-command summaries that spam public channel | **Frictionless `/unrollshort <N\|me\|2h> [pm]` + silent `✍` react-to-brief in PM** (zero group clutter) |
| **AI Chat Awareness** | None | None | Often open-mic or stateless single-turn Q&A | **Silent ambient room context** (`chat_log` last 6–10 same-thread entries) + opt-in vocative addressing (`"Xenobot, ..."`) |
| **Polls & Community Toys** | Manual `/poll` utilities only | XP/leveling leaderboards (often noisy) | Fun commands with little group context | **Context-aware `/referendum` + automatic tied-poll tie-breaker** and **playful `/inspect`** tied to dry Facility persona |
| **Multi-group Permissions & Backups** | Rose `/export` & `/import` (flat dump) | Web dashboard per group | Per-server Discord permissions | **Owner-unlock AI + Admin-toggle interactivity** in private Telegram menus (8 locales), with strict security boundaries in Backup & Copy Settings |

---

## Sources

- **Xenobot-72 Codebase (HIGH confidence):**
  - `.planning/PROJECT.md` & `.planning/notes/v1.3-interactive-presence.md` (locked-in v1.3 decisions & architecture guardrails)
  - `.planning/spikes/MANIFEST.md` (Spikes 001–003 empirically validated 2026-11-26 across `claude-haiku-5-5`, `gemini-4-hydrogen`, and Featherless `inclusionAI/Ling-3.2-air`)
  - `xenobot/modules/settings_chat.go`, `settings_chat_test.go`, `settings_pages.go`, `settings_menu.go`, `settings_reactions.go`, `settings_logchannel.go` (settings framework invariants, `OwnerOnly` gate, 8-row test cap, 64-byte callback budget)
  - `xenobot/modules/reactions.go`, `chat.go`, `chat_watch.go`, `chat_run.go`, `chat_log.go`, `chat_brief.go` (existing handler groups, `supportedReactionEmoji`, `StopChat`, `chatJudgeDeliverPrivate`)
  - `xenobot/modules/copy_settings.go`, `xenobot/db/backup/backup.go`, `xenobot/db/backup/types.go`, `xenobot/db/backup/chatconv_backup_test.go` (Backup & Copy Settings security invariants)
  - `xenobot/config/config.go` (`AllowedUpdates` list, 8 enabled locales)
- **Telegram Bot API & Community Bot UX (MEDIUM confidence — cross-checked web documentation):**
  - Telegram Bot API `setMessageReaction` (1 reaction max per bot, no paid reactions, no service messages, `is_big`), `ChatFullInfo.available_reactions`, and `sendPoll` (1–300 char question, 2–10 options ≤ 100 chars, `open_period`/`close_date`, 1 msg/s & ~20 msg/min group pacing limits)
  - Community starboard & reaction-bot anti-spam patterns (distinct-member aggregation, negative-emoji & self-reaction exclusion, silent failures in main chat, idempotent de-duplication)

---
*Feature research for: Xenobot-72 Milestone v1.3 Interactive Presence*
*Researched: 2026-11-26*
