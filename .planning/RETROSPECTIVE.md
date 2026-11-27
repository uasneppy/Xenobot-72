# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.3 — Interactive Presence

**Shipped:** 2026-11-27
**Phases:** 5 (Phases 31–35) | **Plans:** 34 | **Tasks:** 91

### What Was Built
- **Presence Foundation & Zero-AI Social Reactions (Phase 31):** Subscribed `"message_reaction"`, introduced `chat_presence` (`skipLocal`), enforced shared Staff Group/lockdown safety gates, built private `Presence` Settings page `pr`, shipped `AvailableReactions`-validated reaction API (`IsBig: false`, 24 h `bot_react`), Emoji Consensus (≥ 3 distinct reactors), and `👎` Retraction (`[]gotgbot.ReactionType{}`).
- **Contextual AI Emoji Reactions (Phase 32):** Owner×Admin runtime double-gating, dedicated `chatPresenceQueue` under `StopChat`, cross-replica Redis gap/cooldown/roll/`react:b` gate, 3-provider JSON picker (`MaxTokens = 512`, all 73 standard emojis ∩ `AvailableReactions`, non-tripping error shedding), and `chat_media.go` vision with Featherless `text_only` fallback.
- **Smarter Static Reactions, Highlights Archive & Backup Parity (Phase 33):** Unicode rune-boundary matching (UK/RU/EN safe), captions, chance %, cooldowns, group `8 -> 13` `ctx.Data` AI handoff, zero-AI cat-sticker micro-triggers, Crowd-Picked Highlights (`/setarchive`/`/unsetarchive` + durable PostgreSQL dedup), and `BackupModulePresence` / `BackupModuleReactions` parity (excluding `chat_conversations`, keeping `copy_settings.go` untouched).
- **Room-Aware Conversations & Frictionless Catch-Up (Phase 34):** Timestamped `chat_log` (`At: msg.Date`) with topic isolation, deduplicated Ambient Room Context, opt-in vocative addressing, reply-free `/unrollshort`/`/unrolllong <N|me|2h> [pm]`, and silent `✍` react-to-brief in PM.
- **Interactive Group Toys (Phase 35):** AI-drafted native polls (`/referendum`, `toy:b` budget) with `handlers.NewPoll` deadpan tie-breakers, plus playful vision `/inspect` verdicts paired with `bot_react`-tracked reaction emojis.

### What Worked
- **Empirical AI spikes before roadmap planning:** Live Spikes 001–003 caught Gemini 4 `thinkingLevel: "low"` truncations below `256` tokens, Claude Haiku scratchpad prose leaks and `😂`/`🐱`/`😹` lookalike picks, and Featherless untagged reasoning cutoffs at `384` tokens — preventing every one of those defects from entering production code.
- **File-disjoint wave parallelization:** Waves across Phases 33–35 executed 3–4 independent plans simultaneously with zero merge conflicts and fast `-race` test turnaround.
- **Shared `StopChat` & zero new dependencies:** Routing reaction, brief, poll, and `/inspect` lanes through `chatJobs`/`chatDraining`/`chatContext()` preserved the 70 s webhook shutdown budget and required zero `go.mod` changes.

### What Was Inefficient
- **Spikes 001–003 untracked JSON exports:** Local `results-live.json` and `results-claude.json` outputs were left untracked in `.planning/spikes/` after spike evaluation (harmless, can be cleaned or ignored).
- **Multi-plan summary `requirements-completed` tracking:** In parallel wave runs, downstream milestone updates (`REQUIREMENTS.md` / `ROADMAP.md`) occasionally waited for the final wave cleanup plan instead of updating atomically.

### Patterns Established
- Dedicated `chat_presence` (`skipLocal`) + `BackupModulePresence` boundary separating Owner AI consent (`chat_conversations`, non-backupable) from admin-configurable interactivity toggles.
- `gotgbot/v2` wire-level `[]gotgbot.ReactionType{}` clearing verified via real `httptest` server assertions.
- Pure Go Unicode rune-boundary matching (`unicode.IsLetter`/`unicode.IsDigit`) across UK/RU/EN without RE2 `\b`.

### Key Lessons
1. Always verify Telegram Bot API / `gotgbot/v2` wire serialization directly (`Reaction: nil` omits `"reaction"`, `Update.Poll` omits `ChatId`, `ForwardMessageOpts` omits `ReplyMarkup`) before designing handlers.
2. Keep `xenobot/utils/chatprompt` strictly stdlib-only (`TestChatpromptPackageIsLeaf`) so pure prompt builders and JSON parsers run in millisecond unit tests without bloating `xenobot/modules` CI runtime.

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Sessions | Phases | Key Change |
|-----------|----------|--------|------------|
| v1.3 | 1 | 5 (31–35) | Added live pre-milestone multi-provider AI spikes (`001`–`003`) + file-disjoint wave parallelization + per-phase `VERIFICATION` + `SECURITY` (`SECURED`) |

### Cumulative Quality

| Milestone | Plans | Verified Phases | Security Open Threats | Zero-Dep Additions |
|-----------|-------|-----------------|----------------------|--------------------|
| v1.3 | 34 | 5/5 (`passed`) | 0 (81/81 closed) | 0 (`go.mod` untouched) |

### Top Lessons (Verified Across Milestones)

1. Pre-implementation live AI spikes eliminate thinking-token truncation and format drift across Claude, Gemini, and Featherless before code is written.
2. Enforcing `skipLocal` cache bypass, column-targeted GORM writes without boolean `default:` tags, and append-only SQL migrations prevents cross-replica stale-cache and zero-value overwrite bugs.
