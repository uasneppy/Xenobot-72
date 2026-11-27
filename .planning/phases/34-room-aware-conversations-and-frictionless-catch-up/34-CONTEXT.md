# Phase 34: Room-Aware Conversations and Frictionless Catch-Up - Context

**Gathered:** 2026-11-27
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

Conversations replies understand what the room was just talking about, members can address Xenobot naturally by name when opted in, and catching up on missed threads works via reply-free `/unroll*` arguments or a silent `✍` reaction straight to PM.

Requirements: `CHAT-01`, `CHAT-02`, `CHAT-03`, `CHAT-04`, `CHAT-05`.

</domain>

<decisions>
## Implementation Decisions

### Roadmap & Research Constraints (Locked from v1.3 Exploration & Research)
- **`chatLogEntry` timestamps & thread-isolated helpers (`CHAT-01`):**
  - Add backward-compatible `At int64` (`json:"at,omitempty"`) to `chatLogEntry` populated from `msg.Date` (never `time.Now().Unix()`). Preserve original `At` on edits (never overwrite with `msg.EditDate`). Treat `At == 0` legacy Redis entries gracefully.
  - Filter every `chat_log` query (`chatLogRecentThread`, `chatLogSinceUser`, `chatLogSinceTime`) strictly by `entry.Thread == threadID` so forum topics never bleed into each other.
- **Ambient Room Context in Conversations (`CHAT-02`):**
  - Add `ambient_context` boolean toggle on `chat_presence` (append-only SQL migration strictly after `20261129130000_add_presence_stickers_and_archive.sql`, zero GORM `default:` tags on booleans, `BackupModulePresence` parity in `xenobot/db/backup/`!).
  - Double-gated at runtime: requires Owner-enabled `chat_conversations` + `LogEnabled` AND admin-enabled `ambient_context`. Inject the last 6–10 same-thread `chat_log` entries into `chatBuildRequest` (leaf helper in `xenobot/utils/chatprompt/` — keep `TestChatpromptPackageIsLeaf` green!), deduplicated against `chatReadHistory` and the triggering `job.messageID`.
- **Opt-In Vocative Name-Addressing (`CHAT-03`):**
  - Add `vocative_enabled` boolean toggle on `chat_presence` (with `BackupModulePresence` parity), double-gated with Owner `chat_conversations` + `Audience` permissions (`Everyone` vs `Admins & Approved`).
  - Pure start-of-message Unicode prefix parser (`"Xenobot, ..."` / `"Xenobot: ..."` / `"Кзенобот, ..."` / matching bot's localized/canonical name prefixes) skipping forwarded messages (`msg.ForwardOrigin != nil`). Mutually exclusive with `chatReactWatch` in group `13` (`chat_watch.go`) so vocative-addressed messages go to Conversations and never also trigger ambient AI reactions.
- **Reply-Free `/unrollshort` & `/unrolllong` (`CHAT-04`):**
  - Support running `/unrollshort` and `/unrolllong` without replying by passing a count (e.g., `/unrollshort 50`, bounded), self-anchor (`/unrollshort me`, using `chatLogSinceUser`), or duration (`/unrollshort 2h`, using existing `extraction.ParseDurationToken` + `chatLogSinceTime`), plus optional trailing `pm` delivery token. Preserve existing reply-to-message behavior unchanged.
- **Silent `✍` React-to-Brief in PM (`CHAT-05`):**
  - Add `pm_brief_react` boolean toggle on `chat_presence` (with `BackupModulePresence` parity), double-gated with Owner `chat_conversations` (`LogEnabled == true`).
  - Route `✍` reactions from `presence_reactions.go` (group `0` `NewReaction`, skipping `mru.User == nil` / bots / Staff Groups / locked-down chats, respecting `Audience`) to `chatBriefQueue` under `StopChat` (zero new shutdown drains).
  - Extract a silent PM delivery path (`silent: true`) instead of calling `chatJudgeDeliverPrivate`, guaranteeing **zero group messages posted** whether PM delivery succeeds or fails with `403` (user never `/start`ed the bot). Rate-limit/cooldown per user so tapping `✍` cannot spam brief budgets.
- **Settings UI & Repo Rules (`AGENTS.md`):**
  - Expose `ambient_context`, `vocative_enabled`, and `pm_brief_react` on private Settings page `pr` (or a clean sub-page under `pr` keeping `pr` compact <= 8 rows and Main at 17 rows, reusing Phase 32's Owner×Admin check `settingsPresenceConvUnlocked` with `settings_pr_ai_need_owner`), full 8-locale parity, `make generate-docs` / `make check-docs` if command docs change, `AGENTS.md` updates, and `GOTOOLCHAIN=go1.26.0 make lint`.

### the agent's Discretion
Exact vocative alias list/delimiters, max count/duration caps for reply-free `/unroll*`, `✍` per-user cooldown duration, sub-page vs direct rows on `pr` (within the 8-row limit), and `BackupModulePresence` field names are at the agent's discretion following `AGENTS.md` and `.planning/research/`.

</decisions>

<code_context>
## Existing Code Insights

See Phase 31–33 SUMMARY files, `xenobot/modules/chat_brief.go`, `chat_log.go`, `chat_watch.go`, `chat_prompt.go`, `xenobot/utils/chatprompt/`, `presence_reactions.go`, `settings_presence.go`, `xenobot/db/presence/`, `xenobot/db/backup/`.

</code_context>

<specifics>
## Specific Ideas

Refer to `.planning/ROADMAP.md` Phase 34 success criteria and `.planning/REQUIREMENTS.md` (`CHAT-01`..`CHAT-05`).

</specifics>

<deferred>
## Deferred Ideas

- Phase 35: Interactive Group Toys (`TOYS-01..03`).

</deferred>
