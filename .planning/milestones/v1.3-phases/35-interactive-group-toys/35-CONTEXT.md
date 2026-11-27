# Phase 35: Interactive Group Toys - Context

**Gathered:** 2026-11-27
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

Permitted group members can spark in-character participation through AI-drafted native polls with automatic deadpan tie-breakers (`/referendum`) and vision-capable "Facility Inspection" verdicts with matching emoji reactions (`/inspect`).

Requirements: `TOYS-01`, `TOYS-02`, `TOYS-03`.

</domain>

<decisions>
## Implementation Decisions

### Roadmap & Research Constraints (Locked from v1.3 Exploration & Research)
- **Owner×Admin double-gating & Backup parity (`TOYS-01`, `TOYS-03`):**
  - Add `referendum_enabled` and `inspect_enabled` boolean toggles on `chat_presence` (append-only SQL migration strictly after `20261130120000_add_presence_room_and_catchup.sql`, zero GORM `default:` tags on booleans, `BackupModulePresence` parity in `xenobot/db/backup/`).
  - Double-gated at runtime: requires Owner-enabled `chat_conversations` (with configured provider) AND admin-enabled toggle (`referendum_enabled` / `inspect_enabled`), plus Conversations `Audience` (`Everyone` vs `Admins & Approved`), and explicit `presenceChatTypeAllowed` + `!presenceIsStaffGroup` + `!presenceIsLockedDown` (fail-closed).
- **Private Settings UI (`pr` / `pc`):**
  - Expose `referendum_enabled` and `inspect_enabled` on the Room & Catch-Up sub-page `pc` (currently 4 rows out of 8, leaving 4 free rows!) or a dedicated compact layout under `pr` — keeping `pr` at `<= 8` rows, `len(settingsPages) == 31`, and `settingsMainOwnerRows == 17`, with `settingsPresenceConvUnlocked` (`settings_pr_ai_need_owner`) on enable and full 8-locale parity.
- **`/referendum` AI-drafted native polls (`TOYS-01`):**
  - Command registered with `helpers.NewBotCommand("referendum", ...)` (and `SetModuleHelpFunc` / `DefaultHelpRegistry` doc parity).
  - Pulls context from replied debate target (`extraction.ReplyTarget`) or recent same-thread `chat_log` (`chatLogRecent`, bounded, `entry.Thread == threadID`).
  - Leaf prompt & strict JSON parser in `xenobot/utils/chatprompt/` (stdlib + `xenopersona` only, `TestChatpromptPackageIsLeaf` green): returns `question` + `options` (2–5 bounded options, sanitized via `chatprompt.Clean` / Telegram `SendPoll` length limits: question <= 300 runes, each option <= 100 runes, deduplicated non-empty options). Use `MaxTokens >= 512` (recommend `768` so localized Ukrainian/Russian poll options never truncate behind Gemini 4 / Featherless thinking tokens).
  - Runs on `chatPresenceQueue` (or dedicated toy helper under `StopChat`: `chatJobs.start()`/`finish()`, `chatDraining.Load()`, `chatContext()` — zero new shutdown drains, 70 s worst case unchanged!), gated by dedicated hourly budget (`"toy:b"`) via `chatReserveWith` and per-chat cooldown.
  - Sends native Telegram poll via `SendPollWithContext` (`MessageThreadId: threadID`, `DisableNotification: true`, sensible `OpenPeriod` e.g. 300–3600s or configurable/default open period so `Poll` updates close cleanly, `AllowAddingOptions: false` or true). Stores `xenobot:presence:poll:<poll_id>` (outside `xenobot:cache:`, TTL > `OpenPeriod` e.g. 25h) with `{chat_id, msg_id, thread_id, lang}` because Telegram `Update.Poll` omits `ChatId`/`MessageId`/`MessageThreadId`!
- **Tied-Poll Deadpan Tie-Breaker (`TOYS-02`):**
  - Register `handlers.NewPoll` listener at group `0` (`LoadPresence` in `presence_reactions.go` or `chat_polls.go` called from `LoadPresence`).
  - Trigger only when `poll.IsClosed && poll.TotalVoterCount > 0` and top vote count has `topTie >= 2` (2+ options tied for first place).
  - Resolve `{chat_id, msg_id, thread_id, lang}` from Redis `xenobot:presence:poll:<poll_id>` (ignore unknown/external polls), verify `!presenceIsStaffGroup` & `!presenceIsLockedDown`, claim once-only across replicas (`SETNX` or Lua/atomic claim key `xenobot:presence:poll:tie:<poll_id>`), and post one short deadpan "Facility Tie-Breaker" verdict in the original chat & `threadID` (joining `StopChat`, zero new shutdown drains).
- **Playful `/inspect` (`TOYS-03`):**
  - Member-accessible reply command registered via `helpers.NewBotCommand("inspect", ...)` (replying to text, caption, photo, or sticker; if no reply, crisp usage hint).
  - Enforces Owner `chat_conversations` + `inspect_enabled` + `Audience` + `!StaffGroup` + `!Lockdown` + `"toy:b"` hourly budget + per-user/chat cooldown, running under `StopChat`.
  - Uses `chat_media.go` vision pipeline for photos/stickers (with Featherless `text_only` fallback) + leaf prompt & strict JSON parser (`verdict` + `emoji`, `MaxTokens >= 512`), explicit non-Telegram emoji warning (`😂`, `🐱`/`😹`/`😻`/`😼`), strict `supportedReactionEmoji` ∩ `AvailableReactions` validation with `U+FE0F` strip.
  - Sends the 2–3 line deadpan "Facility Inspection" verdict as a reply (respecting `threadID` and `AllowSendingWithoutReply: false`) AND stamps the validated matching reaction emoji on the target message via `presenceSetReaction(..., authorID, "inspect")` so 24h `bot_react` tracking and `👎` retraction work automatically!
- **Repo Rules (`AGENTS.md`):**
  - Update `Makefile` (`test-postgres-integrity`), `make generate-docs` / `make check-docs`, `AGENTS.md`, `locales/*.yml`, `make check-translations`, `GOTOOLCHAIN=go1.26.0 make lint`, and keep `copy_settings.go` untouched.

### the agent's Discretion
Exact prompt wording, `OpenPeriod` default for `/referendum` polls, exact `"toy:b"` hourly caps, whether `referendum_enabled` & `inspect_enabled` sit directly on sub-page `pc` (which has 4 spare rows) or another arrangement keeping `pr <= 8` rows, and exact JSON field schemas are at the agent's discretion following `AGENTS.md` and `.planning/research/`.

</decisions>

<code_context>
## Existing Code Insights

See Phase 31–34 SUMMARY files, `.planning/research/SUMMARY.md`, `STACK.md`, `ARCHITECTURE.md`, `PITFALLS.md`, `xenobot/modules/chat_brief.go` (for `/judge` audience & command patterns), `chat_react.go` (for `chatPresenceQueue` & `chat_media.go` vision), `presence_react_api.go`, `presence_reactions.go`, `settings_presence.go`.

</code_context>

<specifics>
## Specific Ideas

Refer to `.planning/ROADMAP.md` Phase 35 success criteria and `.planning/REQUIREMENTS.md` (`TOYS-01`..`TOYS-03`).

</specifics>

<deferred>
## Deferred Ideas

- Future (v1.4+): Group-specific custom persona snippets, Highlights album bundling, per-emoji allow-list editor.

</deferred>
