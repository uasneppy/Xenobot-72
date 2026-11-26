# Phase 32: Contextual AI Emoji Reactions - Context

**Gathered:** 2026-11-27
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

Once a group owner enables Conversations, admins can turn on restrained, context- and vision-aware AI emoji reactions that feel dry and observant without spamming chats, starving human replies, or tripping shared conversation breakers.

Requirements: `PRES-04`, `REACT-01`, `REACT-02`, `REACT-03`.

</domain>

<decisions>
## Implementation Decisions

### Roadmap, Research & Empirical Spike Constraints (Spikes 001–003 Validated)
- **Owner×Admin double-gating (`PRES-04`):** Add `ai_react_mode` (`"off"` / `"low"` / `"medium"`, default `"off"`) on `chat_presence` (append-only migration strictly after `20261127120000_add_chat_presence.sql`, inline named `CHECK` constraint, no GORM `default:` on booleans). Expose on private Settings page `pr` (`settingsRightChangeInfo`, 8 locales with `settings_locale_test.go` caps). At runtime, AI reactions ONLY fire when BOTH `config.AppConfig.EnableChat` + `chatconv.Enabled` (Owner-unlocked with 2-tap data notice) AND `chat_presence.AIReactMode != "off"` hold. Disabling either immediately stops AI reactions while zero-AI features remain independent.
- **Dedicated `chatPresenceQueue` under `StopChat`:** Launch a bounded background reaction worker lane inside `chatEnsureWorkers()` (`chat_run.go`, e.g. 2 workers, cap 32) counted in `chatJobs`, refused when `chatDraining.Load()`, and derived from `chatContext()`. Zero new shutdown drains; preserves the 70 s webhook shutdown budget and never starves the 3-worker `chatQueue` for human `/ask` replies.
- **Strict pre-AI local & cross-replica Redis gate (`REACT-01`):** Run inside group `13` (`chat_watch.go`) ONLY when the message is NOT addressed to Xenobot (mutually exclusive with Conversations) and passes `presenceMessageAllowed`. Check cheap gates first, then cross-replica Redis Lua gate (outside `xenobot:cache:`, under `xenobot:presence:{<chat>}:*`):
  - Message-gap counter (e.g., `low`: 30 msgs, `medium`: 15 msgs since last AI reaction)
  - Per-chat cooldown (e.g., `low`: 15 min, `medium`: 8 min)
  - Per-user cooldown (respecting any extended 24h `👎` retraction cooldown from `presenceUserCooldownKey`!)
  - Random probability roll (`math/rand/v2`, e.g., `low`: 12%, `medium`: 25%)
  - Lazy fail-closed `presenceIsLockedDown(chatID)` check
  - Dedicated hourly budget via `chatReserveWith` family `"react:b"`
- **Empirically validated 3-provider JSON picker (`REACT-02`):**
  - Wire format: prompt-instructed JSON (`{"emoji":"..."}` / `{"emoji":"NONE"}`) with **`MaxTokens = 512`** (proven floor across Claude Haiku 5.5, Gemini 4 `thinkingLevel: "low"`, and Featherless reasoning models).
  - Palette: all 73 standard Telegram emojis (`supportedReactionEmoji`) intersected with cached per-chat `AvailableReactions` (`U+FE0F`-stripped). Put static persona + rules in `Static` and per-chat intersected palette + JSON format rule in `Dynamic` (keep `Static` compact — do NOT include the 39 KB comic archive).
  - Explicitly warn in prompt that `😂` and cat-face emojis (`🐱`/`😹`/`😻`/`😼`) are NOT valid Telegram reactions (use `🤣`, `❤`, `👀`, `🥰`).
  - Mandatory `NONE` default on mundane logistics, unclear sarcasm, arguments/moderation drama, and war/tragedy/grief red lines (validated 15/15 red-line safety across all 3 providers).
  - Strict JSON parser requiring `"emoji"` key (rejecting `{}`/`null`/prose/multi-emoji/non-allowlisted emojis), normalizing `U+FE0F`.
  - **Non-tripping error shedding:** Check `chatBreaker(provider).isPaused(chatID)` before calling, but NEVER call `chatRecordFailure` / trip shared `chatBreakers[provider]` on reaction parse errors, `ErrNoReply`, Featherless `no choices`, vision `400`/`422`, or Telegram `400` errors — shed silently as `NONE`.
- **Context & Vision (`REACT-03`):**
  - Include recent same-thread context: last 3–5 same-thread entries from `chat_log` when `LogEnabled` is on (strictly `entry.Thread == threadID` and `entry.MessageID < msgID`), plus free `extraction.ReplyTarget(msg)` and `msg.Quote` even when `LogEnabled` is off.
  - Include photos and sticker thumbnails on vision-capable providers reusing `chat_media.go`; on Featherless, detect/cache `text_only` vision fallback (`400`/`422`, e.g., 24h TTL) and degrade cleanly to text/caption/`[sticker <emoji>]`.
  - Record sent reactions via Phase 31's `presenceSetReaction(..., authorID, "ai")` so `IsBig: false`, `AvailableReactions` validation, and `👎` retraction work automatically!

### the agent's Discretion
Exact prompt wording (grounded in Spikes 001–003 `compact_strict`), exact low/medium numeric defaults within the 15–30 msg gap range, `/stats` counter naming, and leaf prompt package layout (`xenobot/utils/chatprompt`) are at the agent's discretion following `AGENTS.md`.

</decisions>

<code_context>
## Existing Code Insights

See Phase 31 summaries (`31-01`..`31-05`), `.planning/research/SUMMARY.md`, `.planning/spikes/001-reaction-picker-tokens-and-format/README.md`, `002-reaction-abstention-and-persona/README.md`, and `003-reaction-vision-media/README.md`.

</code_context>

<specifics>
## Specific Ideas

Refer to `.planning/ROADMAP.md` Phase 32 success criteria and `.planning/REQUIREMENTS.md` (`PRES-04`, `REACT-01`, `REACT-02`, `REACT-03`).

</specifics>

<deferred>
## Deferred Ideas

- Phase 33: Smarter Static Reactions (`ctx.Data` handoff from group 8), Cat Stickers, Highlights Archive & Backup Parity (`COMM-01..05`). Note: Phase 32 can already respect `ctx.Data[presenceStaticReactedKey]` if present so Phase 33 drops in cleanly!
- Phase 34: Room-Aware Conversations & Frictionless Catch-Up (`CHAT-01..05`).
- Phase 35: Interactive Group Toys (`TOYS-01..03`).

</deferred>
