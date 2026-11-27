# Phase 33: Smarter Static Reactions, Highlights Archive and Backup Parity - Context

**Gathered:** 2026-11-27
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

Groups get sharper, spam-safe static reactions, playful zero-AI cat-sticker micro-triggers, a crowd-picked Facility Archive for standout messages, and safe Backup/Restore portability for presence and reaction rules.

Requirements: `COMM-01`, `COMM-02`, `COMM-03`, `COMM-04`, `COMM-05`.

</domain>

<decisions>
## Implementation Decisions

### Roadmap and research constraints (locked)
- Static reactions (`reactions.go`, group `8`): optional pure Unicode word-boundary matching using `unicode.IsLetter`/`unicode.IsDigit` (never RE2 `\b`, which is ASCII-only and breaks Ukrainian/Russian), caption matching, trigger chance %, per-rule/chat cooldowns (Redis, outside `xenobot:cache:`), and Staff Group/lockdown guards via the Phase 31 `presenceMessageAllowed` / `presenceIsLockedDown` helpers. New per-rule columns on `reactions` with an append-only migration; legacy rows normalize `chance == 0` to `100`.
- When a static rule or cat-sticker trigger reacts, set `ctx.Data[presenceStaticReactedKey]` (defined in Phase 32) so group `13` AI reactions never overwrite it.
- Cat-sticker micro-triggers: match `msg.Sticker.Emoji` against cat glyphs, occasionally react with an allowed `👀` or `❤` (intersected with `AvailableReactions` via `presenceSetReaction`), behind cooldown, zero AI calls. Toggle lives on `chat_presence`.
- Highlights Archive: `/setarchive` / `/unsetarchive` use a 2-step proof modeled on `/setlog` and refuse Staff Groups as both source and destination. Configurable distinct-member threshold, emoji filter (`Positive only` / `All standard`), delivery mode (`CopyMessage` with supergroup-guarded `t.me/c/` jump button as default, or `ForwardMessage`). Distinct reactors tracked via the Phase 31 Redis `SADD`/`SREM` diff primitives (excluding the author and hostile emojis). Once-only archiving backed by a durable PostgreSQL `chat_presence_archived (chat_id, message_id)` unique table. Archive target IDs live on `chat_presence`; sub-page `ph` on Settings page `pr`.
- Backup: add `BackupModulePresence = "presence"` and extend `BackupModuleReactions`. On import strip `archive_chat_id`/`archive_thread_id` (report `ArchiveChannelSkipped`, mirroring `importLogChannels`). `chat_conversations` stays permanently excluded from backups; `copy_settings.go` stays untouched. Old v1.0–v1.2 backups import unchanged.
- Repo rules (AGENTS.md): append-only migrations strictly after the latest existing one, no GORM `default:` tags on booleans, `cache.DeleteCache` on writes, all 8 locales, 64-byte callback cap, light fast tests, no new shutdown drains, update AGENTS.md.

### the agent's Discretion
Exact column names, cooldown defaults, cat glyph list, callback codes and sub-page layout are at the agent's discretion following `AGENTS.md`, the Phase 31/32 summaries, and `.planning/research/`.

</decisions>

<code_context>
## Existing Code Insights

See Phase 31 and 32 SUMMARY files, `.planning/research/ARCHITECTURE.md`, `STACK.md` and `PITFALLS.md`.

</code_context>

<specifics>
## Specific Ideas

Refer to `.planning/ROADMAP.md` Phase 33 success criteria and `.planning/REQUIREMENTS.md` (`COMM-01`..`COMM-05`).

</specifics>

<deferred>
## Deferred Ideas

- Phase 34: Room-Aware Conversations and Frictionless Catch-Up (`CHAT-01..05`).
- Phase 35: Interactive Group Toys (`TOYS-01..03`).
- Future: Highlights album bundling and a per-emoji allow-list editor.

</deferred>
