# Requirements: Xenobot-72

**Defined:** 2026-11-26
**Milestone:** v1.3 Interactive Presence
**Core Value:** My trusted staff can protect every one of my communities from one place. We act on a bad actor across all groups at once, and the bot never lets anyone act in a group where they aren't an admin.

## v1.3 Requirements

Requirements for Milestone v1.3 Interactive Presence. Each maps to roadmap phases starting at Phase 31.

### Reaction Foundation & Settings

- [x] **PRES-01**: Bot subscribes to `"message_reaction"` updates in `AllowedUpdates` and processes reaction and poll events safely across replicas under `StopChat` with zero new shutdown drains
- [x] **PRES-02**: Every ambient interactive path enforces a shared safety gate skipping private chats, Staff Groups (`staff.GetStaffGroup`), active lockdowns (`lockdown.GetActiveFresh`), bots, anonymous channels, and slash commands
- [x] **PRES-03**: Group admin can configure v1.3 interactive settings from a private-only `Presence` Settings page (`pr`, hidden in Staff Groups, 8 locales), linked from Conversations page `p` (keeping page `p` <= 8 rows and Main grid at 17 rows)
- [ ] **PRES-04**: AI-backed interactive features are double-gated at runtime (Owner-enabled `chat_conversations` with 2-tap data notice AND admin per-feature toggles); zero-AI features are admin-configurable independently
- [x] **PRES-05**: Bot normalizes (`U+FE0F`-stripped) and validates every outgoing reaction against `supportedReactionEmoji` intersected with cached per-chat `AvailableReactions` (20 min Redis TTL), sends `IsBig: false`, and stores 24 h `bot_react` metadata (`emoji`, `author_id`, `source`)

### Contextual AI & Social Reactions

- [ ] **REACT-01**: Group admin can enable Contextual Random AI Emoji Reactions (`Off / Low / Medium`) gated before any AI call by cross-replica Redis message-gap (15–30 msgs), per-chat cooldown, per-user cooldown, probability roll, and dedicated hourly budget (`react:b`)
- [ ] **REACT-02**: Contextual AI reaction picker uses prompt-instructed JSON (`{"emoji":"..."}` / `{"emoji":"NONE"}`) with `MaxTokens = 512` across Claude, Gemini, and Featherless, offers all 73 standard emojis (intersected with `AvailableReactions`), warns against non-Telegram lookalikes (`😂`, `🐱`/`😹`/`😻`/`😼`), defaults to `NONE` on mundane/drama/tragedies, and silently sheds parse/network/400 errors without tripping shared conversation breakers
- [ ] **REACT-03**: Contextual AI reactions inspect recent same-thread context (`chat_log` when enabled, plus `ReplyTarget`/`Quote`) and include photos and sticker thumbnails on vision-capable providers via `chat_media.go`, degrading cleanly to text/caption/sticker-emoji on text-only Featherless models
- [x] **REACT-04**: Zero-AI Emoji Consensus lets the bot occasionally mirror a non-hostile standard emoji when >= 3 distinct members react with it (tracked via Redis Lua `SADD`/`SREM` diffs, with chat cooldown and once-only latch)
- [x] **REACT-05**: Zero-AI `👎` Retraction silently clears the bot's reaction (sending explicit `[]gotgbot.ReactionType{}`) and extends that user's cooldown when the message author or a group admin reacts `👎`

### Smarter Static Reactions & Highlights Archive

- [ ] **COMM-01**: Static reaction rules (`reactions.go`, group `8`) support optional pure Unicode word-boundary matching (UK/RU/EN safe, not ASCII-only `\b`), caption matching, trigger chance %, per-rule/chat cooldowns, Staff/lockdown guards, and a `ctx.Data` marker so group `13` AI reactions never overwrite static reactions
- [ ] **COMM-02**: Zero-AI cat-sticker micro-triggers detect cat glyphs on `msg.Sticker.Emoji` and occasionally react with an allowed `👀` or `❤` (behind cooldown + `ctx.Data` handoff) with zero AI calls
- [ ] **COMM-03**: Group admin can link or unlink a Highlights Archive destination channel or forum topic via 2-step `/setarchive` / `/unsetarchive` proof (refusing Staff Groups as source or destination) and configure the distinct-member reaction threshold, emoji filter (`Positive only` / `All standard`), and delivery mode (`CopyMessage` with `t.me/c/` jump button default, or `ForwardMessage`)
- [ ] **COMM-04**: When a message hits the Highlights threshold of distinct member reactions (excluding author self-reactions and hostile emojis), the bot archives it once-only (backed by durable PostgreSQL `(chat_id, message_id)` deduplication) into the linked destination
- [ ] **COMM-05**: Backup & Restore adds `BackupModulePresence` (`"presence"`) and extends `BackupModuleReactions` (normalizing legacy `chance == 0 -> 100`), strips `archive_chat_id`/`archive_thread_id` on import (`ArchiveChannelSkipped`), keeps `chat_conversations` permanently excluded from Backup, and leaves Copy Settings (`copy_settings.go`) untouched

### Room-Aware Conversations & Catch-Up

- [ ] **CHAT-01**: `chatLogEntry` stores backward-compatible timestamps (`At: msg.Date`, `json:"at,omitempty"`) and provides strict same-thread query helpers (`Recent`, `SinceUser`, `SinceTime`) without corrupting timestamps on message edits
- [ ] **CHAT-02**: When Ambient Room Context is enabled (with `LogEnabled` on), Conversations injects the last 6–10 same-thread `chat_log` entries into `chatBuildRequest`, deduplicated against `chatReadHistory` and the triggering message
- [ ] **CHAT-03**: When Vocative Name-Addressing is enabled, members can start a non-forwarded message with Xenobot's name (`"Xenobot, ..."` / `"Кзенобот, ..."`, pure Unicode prefix check) to converse without `@mention` or Reply, mutually exclusive with ambient AI reactions
- [ ] **CHAT-04**: Members can run `/unrollshort` and `/unrolllong` without replying using count (`50`), self-anchor (`me`), or duration (`2h`), plus optional `pm` delivery
- [ ] **CHAT-05**: When `✍` PM Briefs are enabled (with `LogEnabled` on), reacting `✍` to a logged message silently delivers a private `/unrollshort` from that message to the reactor's PM, posting zero group messages on success or `403`

### Interactive Group Toys

- [ ] **TOYS-01**: When `/referendum` is enabled, permitted members (following Conversations `Audience`) can trigger an AI-drafted native Telegram poll (`SendPoll`) from recent same-thread context or a replied debate, with strict JSON validation, text sanitization, dedicated hourly budget (`toy:b`), and Redis poll metadata (`poll_id -> {chat, msg, thread, lang}`)
- [ ] **TOYS-02**: When a bot-created `/referendum` poll closes in a tie with `TotalVoterCount > 0`, a `handlers.NewPoll` listener claims it once-only and posts a deadpan Facility Tie-Breaker verdict in the original chat/topic
- [ ] **TOYS-03**: When `/inspect` is enabled, permitted members (following Conversations `Audience`) can reply to a text, photo, or sticker with `/inspect` for a crisp 2–3 line deadpan "Facility Inspection" verdict (vision-enabled via `chat_media.go`) plus a validated matching reaction emoji recorded in `bot_react`

## Future Requirements

Deferred to v1.4+. Tracked but not in current roadmap.

### Presence Enhancements

- **PRES-F01**: Group-specific custom persona snippets and inside jokes
- **PRES-F02**: Highlights Archive multi-photo album (media-group) bundling
- **PRES-F03**: Per-emoji custom allow-list editor for Highlights Archive (beyond `Positive only / All standard`)

## Out of Scope

Explicitly excluded from v1.3 to preserve chat signal-to-noise, privacy, and architecture invariants.

| Feature | Reason |
|---------|--------|
| Open-mic 60–90s follow-up timer after bot replies | Hijacks human-to-human replies in active groups; replaced by opt-in vocative name-addressing (`CHAT-03`) |
| Sending every group message to an AI model | Violates `PROJECT.md` cost/latency rule; contextual AI reactions stay strictly behind message-gap, cooldown, probability, and hourly budget gates |
| `🌐` translate-by-reaction | Non-standard emoji (unusable by non-Premium users) and `MessageReactionUpdated` carries no message text |
| End-of-day scheduled Shift Report | Hits `chatLogMax = 1000` in busy chats and adds a new background scheduler + shutdown hook |
| Dynamic AI welcome messages | Adds join latency, risks raid-burst AI spend, and competes with v1.1 custom welcome templates |
| Exporting/copying `chat_conversations` or Highlights `archive_chat_id` | Preserves Owner 2-tap AI data consent (`TestChatConversationsStayOutOfBackup`) and live-proof channel security (`importLogChannels` invariant) |
| Bundling `chat_presence` into Phase 29 Copy Settings | Keeps Copy Settings focused on moderation messages & punishment tuning; admins migrate presence via Backup (`/export`/`/import`) |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| PRES-01 | Phase 31 | Complete |
| PRES-02 | Phase 31 | Complete |
| PRES-03 | Phase 31 | Complete |
| PRES-04 | Phase 32 | Pending |
| PRES-05 | Phase 31 | Complete |
| REACT-01 | Phase 32 | Pending |
| REACT-02 | Phase 32 | Pending |
| REACT-03 | Phase 32 | Pending |
| REACT-04 | Phase 31 | Complete |
| REACT-05 | Phase 31 | Complete |
| COMM-01 | Phase 33 | Pending |
| COMM-02 | Phase 33 | Pending |
| COMM-03 | Phase 33 | Pending |
| COMM-04 | Phase 33 | Pending |
| COMM-05 | Phase 33 | Pending |
| CHAT-01 | Phase 34 | Pending |
| CHAT-02 | Phase 34 | Pending |
| CHAT-03 | Phase 34 | Pending |
| CHAT-04 | Phase 34 | Pending |
| CHAT-05 | Phase 34 | Pending |
| TOYS-01 | Phase 35 | Pending |
| TOYS-02 | Phase 35 | Pending |
| TOYS-03 | Phase 35 | Pending |

**Coverage:**
- v1.3 requirements: 23 total
- Mapped to phases: 23
- Unmapped: 0

---
*Requirements defined: 2026-11-26*
*Last updated: 2026-11-26 after v1.3 roadmap creation*
