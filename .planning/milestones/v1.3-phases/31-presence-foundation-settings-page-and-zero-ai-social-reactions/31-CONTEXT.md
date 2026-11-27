# Phase 31: Presence Foundation, Settings Page and Zero-AI Social Reactions - Context

**Gathered:** 2026-11-26
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

Group admins can configure Interactive Presence from the bot's private chat while groups gain safe, reaction-aware zero-AI presence (Emoji Consensus and `👎` Retraction) without adding any shutdown drain or touching Staff Groups or locked-down chats.

Requirements: `PRES-01`, `PRES-02`, `PRES-03`, `PRES-05`, `REACT-04`, `REACT-05`.

</domain>

<decisions>
## Implementation Decisions

### Roadmap & Research Constraints (Locked from v1.3 Exploration & Research)
- Enable `"message_reaction"` in `config.AppConfig.AllowedUpdates` (rolling deploy note: all v1.2 replicas must stop before final `getWebhookInfo` verification).
- Store admin-configurable Interactive Presence settings in a dedicated `chat_presence` table (`xenobot/db/presence/`, `models.ChatPresence`, `skipLocal` cache key `chat_presence:<chat>`), keeping `chat_conversations` strictly Owner-only and permanently excluded from Backup/Copy Settings. Boolean columns omit GORM `default:` tags; migrations are append-only strictly after `20261126120000_allow_featherless_chat_provider.sql`.
- Register private-only Settings page `pr` (`OwnerOnly: false`, `PrivateOnly: true`, hidden in Staff Groups, `settingsRightChangeInfo`), paired with `y` on Main (keeping `settingsMainOwnerRows = 17` unchanged) and add one shortcut row on Conversations page `p` (keeping page `p` <= 8 rows). Provide full 8-locale translations following `locale-style.md`.
- Clear bot reactions on `👎` retraction by passing explicit non-nil `[]gotgbot.ReactionType{}` (`gotgbot/v2` omits `"reaction"` on the wire when `nil`; verify wire JSON in `httptest`).
- Anonymous group admin (`ActorChat`) policy: allow `mru.User == nil && mru.ActorChat != nil && mru.ActorChat.Id == mru.Chat.Id` (anonymous admin acting as the group itself) to trigger `👎` retraction, while excluding anonymous/channel actors (`mru.User == nil`) from Consensus distinct-member counting and user-scoped flows.
- Operational Redis keys (cooldowns, 24h `bot_react` metadata storing `{emoji, author_id, source}`, distinct-reactor sets) live outside `xenobot:cache:` so `CLEAR_CACHE_ON_STARTUP` never wipes them; `AvailableReactions` (20 min TTL) and `chat_presence` (`skipLocal`) live in cache.
- Shared Staff/Lockdown safety gate skips private chats, Staff Groups (`staff.GetStaffGroup`), active lockdowns (`lockdown.GetActiveFresh` lazily after cheap/Redis checks), bots, anonymous channels, and slash commands.
- Reaction processing joins `StopChat` (`chatJobs` + `chatContext()`) with zero new shutdown drains (preserving the 70 s webhook worst-case shutdown sum and `stop_grace_period: 75s`).

### the agent's Discretion
All remaining implementation choices (helper naming, sub-page button layout within 64-byte `callbackcodec` limits, exact Redis key suffixes, and unit/integration test structure) are at the agent's discretion following `AGENTS.md`.

</decisions>

<code_context>
## Existing Code Insights

See `.planning/research/SUMMARY.md`, `.planning/research/ARCHITECTURE.md`, `.planning/research/STACK.md`, and `.planning/research/PITFALLS.md`.

</code_context>

<specifics>
## Specific Ideas

Refer to `.planning/ROADMAP.md` Phase 31 success criteria and `.planning/REQUIREMENTS.md` (`PRES-01`, `PRES-02`, `PRES-03`, `PRES-05`, `REACT-04`, `REACT-05`).

</specifics>

<deferred>
## Deferred Ideas

- Phase 32: Contextual AI Emoji Reactions (`PRES-04`, `REACT-01..03`).
- Phase 33: Smarter Static Reactions, Cat Stickers, Highlights Archive & Backup Parity (`COMM-01..05`).
- Phase 34: Room-Aware Conversations & Frictionless Catch-Up (`CHAT-01..05`).
- Phase 35: Interactive Group Toys (`TOYS-01..03`).

</deferred>
