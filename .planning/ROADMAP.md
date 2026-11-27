# Roadmap: Xenobot-72

## Milestones

- ✅ **v1.0 Staff Group, Raid Protection, Web Captcha & Settings Menu** - Phases 1-9 (shipped 2026-11-02, archived in `milestones/v1.0-ROADMAP.md`)
- ✅ **v1.1 PM Settings Hub & Member Screening** - Phases 10-19 (shipped 2026-11-13, archived in `milestones/v1.1-ROADMAP.md`; the owner's live Telegram checks are deferred, see `STATE.md` Deferred Items)
- ✅ **v1.2 Conversations, Custom Messages and Tuning** - Phases 20-30 (shipped 2026-11-24, archived in `milestones/v1.2-ROADMAP.md`; live Telegram checks for Phases 21-27 deferred, see `STATE.md` Deferred Items)
- 🚧 **v1.3 Interactive Presence** - Phases 31-35 (in progress)

## Phases

<details>
<summary>✅ v1.0 Staff Group, Raid Protection, Web Captcha & Settings Menu (Phases 1-9) - SHIPPED 2026-11-02</summary>

Full phase details, success criteria and plan lists: [milestones/v1.0-ROADMAP.md](milestones/v1.0-ROADMAP.md). Requirements: [milestones/v1.0-REQUIREMENTS.md](milestones/v1.0-REQUIREMENTS.md).

- [x] **Phase 1: Staff Group Links** - 10/10 plans (completed 2026-10-05)
- [x] **Phase 2: Staff Actions Across Groups** - 10/10 plans (completed 2026-10-05)
- [x] **Phase 3: Staff Audit and Undo** - 12/12 plans (completed 2026-10-06)
- [x] **Phase 4: Manual Lockdown** - 8/8 plans (completed 2026-10-08)
- [x] **Phase 5: Lockdown Alerts and Response** - 14/14 plans (completed 2026-10-18)
- [x] **Phase 6: Automatic Raid Detection** - 13/13 plans (completed 2026-10-29)
- [x] **Phase 7: AI-Assisted Raid Detection** - 14/14 plans (completed 2026-10-29)
- [x] **Phase 8: Turnstile Web Captcha** - 11/11 plans (completed 2026-10-30)
- [x] **Phase 9: Settings Menu** - 14/14 plans (completed 2026-10-30)

</details>

<details>
<summary>✅ v1.1 PM Settings Hub & Member Screening (Phases 10-19) - SHIPPED 2026-11-13</summary>

Full phase details, success criteria and plan lists: [milestones/v1.1-ROADMAP.md](milestones/v1.1-ROADMAP.md). Requirements: [milestones/v1.1-REQUIREMENTS.md](milestones/v1.1-REQUIREMENTS.md). Audit: [milestones/v1.1-MILESTONE-AUDIT.md](milestones/v1.1-MILESTONE-AUDIT.md) (`tech_debt`, no blocker). Phase folders, with each phase's `VERIFICATION`, `SECURITY` and `SPIKE` files: `milestones/v1.1-phases/`.

Every phase is code-verified and security-audited with no open threat; each verification reads `human_needed` because the owner's live Telegram checks were deferred on purpose (the dates below are the verification dates).

- [x] **Phase 10: PM Settings Hub and Owner Gate** - 11/11 plans (verified 2026-11-04)
- [x] **Phase 11: Message Editor** - 14/14 plans (verified 2026-11-04)
- [x] **Phase 12: Rules Media, Media Under Text and Greet Once** - 16/16 plans (verified 2026-11-05)
- [x] **Phase 13: Member Requirements** - 14/14 plans (verified 2026-11-08)
- [x] **Phase 14: Profile Photo and Channel Rules** - 12/12 plans (verified 2026-11-08)
- [x] **Phase 15: AI Spam-Name Check** - 11/11 plans (verified 2026-11-10)
- [x] **Phase 16: Scheduled Deletion of Service and Bot Messages** - 9/9 plans (verified 2026-11-12)
- [x] **Phase 17: Command, Edit and Self-Destruct Deletion** - 12/12 plans (verified 2026-11-12)
- [x] **Phase 18: Remaining Settings Pages** - 15/15 plans (verified 2026-11-13)
- [x] **Phase 19: Blacklist, Filter and Note Pages** - 12/12 plans (verified 2026-11-13)

</details>

<details>
<summary>✅ v1.2 Conversations, Custom Messages and Tuning (Phases 20-30) - SHIPPED 2026-11-24</summary>

Full phase details, success criteria and plan lists: [milestones/v1.2-ROADMAP.md](milestones/v1.2-ROADMAP.md). Requirements: [milestones/v1.2-REQUIREMENTS.md](milestones/v1.2-REQUIREMENTS.md). Phase folders, with each phase's `VERIFICATION`, `SECURITY`, `REVIEW` and `REVIEW-FIX` files: `milestones/v1.2-phases/`.

Every phase is implemented, security-audited with no open threat, code-reviewed with all actionable findings fixed, and merged to `main` via PR #23 (14/14 CI checks green). Live Telegram checks for Phases 21-27 were deferred on purpose (`human_needed`, acknowledged in `STATE.md` Deferred Items).

- [x] **Phase 20: Conversations** - 8/8 plans (shipped in v2.26.0)
- [x] **Phase 21: Message Engine, Messages Page and Manual Punishments** - 11/11 plans (verified, live checks deferred)
- [x] **Phase 22: PM Copies** - 9/9 plans (verified, live checks deferred)
- [x] **Phase 23: Warn, Flood, Blacklist and Forward Blocklist Messages** - 7/7 plans (verified, live checks deferred)
- [x] **Phase 24: Captcha, AI Spam and Member Requirement Notices** - 8/8 plans (verified, live checks deferred)
- [x] **Phase 25: Lockdown Messages** - 8/8 plans (verified, live checks deferred)
- [x] **Phase 26: Timed Punishments, Escalation and Warn Expiry** - 12/12 plans (verified, live checks deferred)
- [x] **Phase 27: Manual Command Defaults, Reason Shortcuts and Appeal Button** - 7/7 plans (verified, live checks deferred)
- [x] **Phase 28: Advanced Thresholds** - 6/6 plans (completed 2026-11-23)
- [x] **Phase 29: Plain Tone and Copy Settings** - 6/6 plans (completed 2026-11-23)
- [x] **Phase 30: Backup Page and Custom Messages in Backups** - 5/5 plans (completed 2026-11-24)

</details>

### 🚧 v1.3 Interactive Presence (In Progress)

**Milestone Goal:** Make Xenobot-72 feel alive, witty, and context-aware in groups through restrained emoji reactions, room-aware replies, and opt-in community toys—without ever becoming annoying, noisy, or out of character.

**Phase Numbering:**
- Integer phases (31–35): Planned milestone work, continuing from v1.2's Phase 30.
- Decimal phases (31.1, 31.2): Urgent insertions (marked with INSERTED).

- [x] **Phase 31: Presence Foundation, Settings Page and Zero-AI Social Reactions** - `"message_reaction"` subscription, `chat_presence` storage, shared Staff/lockdown safety gate, private `Presence` Settings page (`pr`), validated `AvailableReactions`-safe reaction API + `bot_react` tracking, zero-AI Emoji Consensus, and `👎` Retraction (completed 2026-11-27)
- [x] **Phase 32: Contextual AI Emoji Reactions** - Owner×Admin runtime double-gating, `chatPresenceQueue` worker lane under `StopChat`, cross-replica Redis throttle/budget gate, 3-provider JSON picker (`MaxTokens = 512`) over all 73 standard emojis ∩ `AvailableReactions`, vision support via `chat_media.go` with text-only Featherless fallback, and non-tripping error shedding (completed 2026-11-27)
- [x] **Phase 33: Smarter Static Reactions, Highlights Archive and Backup Parity** - Unicode word-boundary/caption/chance/cooldown static reactions with `ctx.Data` AI handoff, zero-AI cat-sticker micro-triggers, Crowd-Picked Highlights ("Facility Archive") with `/setarchive` proof and durable PostgreSQL dedup, and `BackupModulePresence` / `BackupModuleReactions` parity (completed 2026-11-27)
- [x] **Phase 34: Room-Aware Conversations and Frictionless Catch-Up** - Timestamped `chat_log` (`At: msg.Date`) with thread-isolated query helpers, Ambient Room Context injected into Conversations, opt-in vocative name-addressing, reply-free `/unrollshort`/`/unrolllong <N|me|2h> [pm]`, and silent `✍` react-to-brief in PM (completed 2026-11-27)
- [ ] **Phase 35: Interactive Group Toys** - AI-drafted native polls (`/referendum`) with dedicated `toy:b` budget and `handlers.NewPoll` deadpan tie-breaker, plus playful `/inspect` vision verdicts paired with validated reaction emojis

### Cross-Cutting Constraints

- **Zero new external Go dependencies and zero new shutdown drains.** Every asynchronous interactive job joins `StopChat` (`chatJobs` + `chatContext()`), keeping the 70 s webhook worst-case shutdown sum and `stop_grace_period: 75s` unchanged.
- **No new message handler groups.** Message hooks reuse existing groups (`8` static reactions/cat stickers, `13` conversation/vocative/AI-reaction watcher, `14` message log); `handlers.NewReaction` and `handlers.NewPoll` sit at group `0` (non-message updates).
- **Owner×Admin consent boundary.** `chat_conversations` stays strictly Owner-only (2-tap data notice) and permanently excluded from Backup/Copy Settings; admin toggles live in `chat_presence` (`skipLocal`), and every AI interactive feature is double-gated at runtime (`conv.Enabled && featureToggle`).
- **Silent degradation and Staff/Lockdown safety.** Ambient and reaction paths skip private chats, Staff Groups, active lockdowns, bots, anonymous channels, and slash commands, and never post error spam into groups.
- **Repo and CI discipline (`AGENTS.md`).** Append-only idempotent SQL migrations strictly after `20261126120000_allow_featherless_chat_provider.sql`, no GORM `default:` tags on booleans, cache invalidation on writes, 8-locale parity following `locale-style.md`, 64-byte callback caps, and light millisecond unit tests so `xenobot/modules` stays inside its CI budget.

## Phase Details

### Phase 31: Presence Foundation, Settings Page and Zero-AI Social Reactions
**Goal**: Group admins can configure Interactive Presence from the bot's private chat while groups gain safe, reaction-aware zero-AI presence (Emoji Consensus and `👎` Retraction) without adding any shutdown drain or touching Staff Groups or locked-down chats.
**Depends on**: Phase 30 (v1.2 settings framework, Conversations page `p`, and `StopChat` runtime)
**Requirements**: PRES-01, PRES-02, PRES-03, PRES-05, REACT-04, REACT-05
**Planning notes**:
- Enable `"message_reaction"` in `config.AppConfig.AllowedUpdates` (rolling deploy note: all v1.2 replicas must stop before final `getWebhookInfo` verification).
- Clear reactions by passing explicit non-nil `[]gotgbot.ReactionType{}` (`gotgbot/v2` omits `"reaction"` when `nil`; assert wire JSON in `httptest`).
- Settle `ActorChat` policy for anonymous group admins (`mru.User == nil && mru.ActorChat.Id == mru.Chat.Id`) on `👎` retraction while keeping anonymous/channel actors excluded from Consensus and user-scoped flows.
- Operational Redis keys (cooldowns, `bot_react`, distinct-reactor sets) must live outside `xenobot:cache:` so `CLEAR_CACHE_ON_STARTUP` never wipes them; `AvailableReactions` (20 min TTL) and `chat_presence` (`skipLocal`) stay in cache.
- Pair `pr` with `y` on Main (keeping `settingsMainOwnerRows = 17`) and add one shortcut row on page `p` (keeping page `p` <= 8 rows).
**Success Criteria** (what must be TRUE):
  1. In the bot's private `/settings` menu, a group admin opens the new `Presence` page (`pr`, also linked from Conversations page `p`, with page `p` <= 8 rows and Main at 17 rows), views and changes zero-AI presence settings in all 8 locales, and sees the page hidden in Staff Groups and refused with "open in private" in group menus.
  2. When three or more distinct members react to a group message with the same non-hostile standard emoji (with Emoji Consensus enabled), the bot mirrors that emoji at most once per message after respecting the chat cooldown, never double-counting one member toggling reactions or a Premium user stacking emojis.
  3. When the message's author or a group admin reacts `👎` to a message the bot reacted to, the bot silently clears its reaction (sending explicit `[]gotgbot.ReactionType{}`) and extends that user's cooldown with zero group messages posted; a non-author regular member reacting `👎` changes nothing.
  4. Every outgoing bot reaction is normalized (`U+FE0F`-stripped), sent with `IsBig: false`, restricted to standard emojis allowed by the chat's cached `AvailableReactions`, and tracked with 24-hour `bot_react` metadata; reaction processing runs under `StopChat` with zero new shutdown drains.
  5. Private chats, Staff Groups, groups under active lockdown, bots, anonymous channels, and slash commands never trigger ambient interactive presence paths.
**Plans:** 5/5 plans complete

Plans:
- [x] 31-01-PLAN.md — AllowedUpdates `"message_reaction"`, `chat_presence` migration/model/`skipLocal` cache & repository
- [x] 31-02-PLAN.md — Shared Staff/Lockdown/actor safety gate, apply layer, safe reaction API, 20m `AvailableReactions` cache & 24h `bot_react` tracking
- [x] 31-03-PLAN.md — 8-locale translations and `settings_locale_test.go` ceilings/20-rune grid cap for Presence page `pr`
- [x] 31-04-PLAN.md — Group 0 `LoadPresence` reaction router under `StopChat`, `👎` Retraction & Redis Lua Emoji Consensus
- [x] 31-05-PLAN.md — Private Settings page `pr`, Conversations page `p` shortcut, UI/Main/Staff regression tests & `AGENTS.md` update
**UI hint**: yes

### Phase 32: Contextual AI Emoji Reactions
**Goal**: Once a group owner enables Conversations, admins can turn on restrained, context- and vision-aware AI emoji reactions that feel dry and observant without spamming chats, starving human replies, or tripping shared conversation breakers.
**Depends on**: Phase 31
**Requirements**: PRES-04, REACT-01, REACT-02, REACT-03
**Planning notes**:
- Add a bounded `chatPresenceQueue` lane inside `chatEnsureWorkers()` counted in `chatJobs` and derived from `chatContext()` so background reactions never starve the 3-worker `chatQueue` for `/ask` and shutdown still needs zero new drains.
- Enforce `MaxTokens = 512` (hard floor empirically validated in Spikes 001–003 for Gemini 4 thinking tokens and Featherless reasoning prose) and explicit prompt warnings against non-Telegram lookalikes (`😂`, `🐱`/`😹`/`😻`/`😼`).
- Background reaction parse/network/`400`/`422`/empty errors must shed silently and never trip `chatBreakers[provider]`; cache Featherless `text_only` vision fallback on `400`/`422`.
**Success Criteria** (what must be TRUE):
  1. A group admin can toggle Contextual AI Reactions (`Off / Low / Medium`) on the Presence page, yet no AI reaction call ever fires unless the group owner has also enabled Conversations (with its 2-tap data notice); disabling either switch immediately stops AI reactions while zero-AI features keep working independently.
  2. Ordinary group traffic is filtered before any AI call by cross-replica Redis message-gap (15–30 messages), per-chat cooldown, per-user cooldown, probability roll, and a dedicated hourly reaction budget (`react:b`).
  3. On eligible messages, the AI picker across Claude, Gemini, and Featherless uses prompt-instructed JSON (`MaxTokens = 512`) over all 73 standard emojis intersected with `AvailableReactions`, rejects non-Telegram lookalikes, defaults to `NONE` on mundane chatter, drama, or tragedies, and silently sheds parse, network, or `400` errors without tripping shared conversation breakers.
  4. Eligible replies, quoted messages, photos, and stickers pass recent same-thread `chat_log` context (when enabled) and visual thumbnails on vision-capable providers via `chat_media.go`, degrading cleanly to text, caption, or sticker emoji on text-only Featherless models.
**Plans:** 6/6 plans complete

Plans:
- [x] 32-01-PLAN.md — `chat_presence.ai_react_mode` append-only migration, model, rolling-deploy normalization & repository
- [x] 32-02-PLAN.md — Leaf `chatprompt/reaction.go` prompt builder (`MaxTokens = 512`), lookalike warnings & strict `U+FE0F` JSON parser
- [x] 32-03-PLAN.md — 8-locale translations and `settings_locale_test.go` ceilings/alert/3-button caps for Presence AI reactions
- [x] 32-04-PLAN.md — Shared `presence.go` AI keys & apply helper, `presenceAllowedPalette`, and bounded thread-isolated `chatLogRecentThread`
- [x] 32-05-PLAN.md — `chatPresenceQueue` under `StopChat`, group 13 Lua gate, context/vision + Featherless `text_only` fallback & non-tripping shedding
- [x] 32-06-PLAN.md — Private Settings page `pr` `[Off | Low | Medium]` Owner×Admin double-gating, tests, `AGENTS.md` update & final gate
**UI hint**: yes

### Phase 33: Smarter Static Reactions, Highlights Archive and Backup Parity
**Goal**: Groups get sharper, spam-safe static reactions, playful zero-AI cat-sticker micro-triggers, a crowd-picked Facility Archive for standout messages, and safe Backup/Restore portability for presence and reaction rules.
**Depends on**: Phase 32
**Requirements**: COMM-01, COMM-02, COMM-03, COMM-04, COMM-05
**Planning notes**:
- Use pure Go Unicode rune boundaries (`unicode.IsLetter`/`unicode.IsDigit`) for word-boundary matching—never Go RE2 `\b`, which is ASCII-only and silently breaks Ukrainian/Russian Cyrillic.
- Set `ctx.Data[presenceStaticReactedKey]` in group `8` when a static rule or cat-sticker trigger reacts so group `13` AI reactions never overwrite it.
- Model `/setarchive` / `/unsetarchive` 2-step proof on `/setlog` (rejecting Staff Groups as source or destination); default Highlights delivery to `CopyMessage` with supergroup-guarded inline `t.me/c/` jump button (with `ForwardMessage` as the alternative mode on sub-page `ph`).
- Back Highlights deduplication with a durable PostgreSQL `(chat_id, message_id)` unique table (`chat_presence_archived`) so Redis restarts or viral posts never re-archive.
**Success Criteria** (what must be TRUE):
  1. Static reaction rules support optional pure Unicode word-boundary matching (Ukrainian, Russian, and English safe), media caption matching, trigger chance %, per-rule/chat cooldowns, and Staff/lockdown guards; whenever a static rule or cat-sticker trigger reacts, group `13` AI reactions never overwrite that reaction.
  2. When cat-sticker micro-triggers are enabled, sending a sticker carrying a cat glyph on `msg.Sticker.Emoji` occasionally receives an allowed `👀` or `❤` reaction behind cooldown with zero AI calls.
  3. A group admin can link or unlink a Highlights Archive channel or forum topic via 2-step `/setarchive` / `/unsetarchive` proof (with Staff Groups refused as both source and destination) and configure the distinct-member threshold, emoji filter (`Positive only` / `All standard`), and delivery mode (`CopyMessage` with `t.me/c/` jump button default, or `ForwardMessage`) from the Presence page.
  4. When a group message reaches the configured threshold of distinct non-author member reactions (excluding hostile emojis), the bot archives it into the linked destination exactly once across restarts and replicas.
  5. `/export` and `/import` include `BackupModulePresence` (`"presence"`) and extended `BackupModuleReactions` (normalizing legacy `chance == 0` to `100`), strip `archive_chat_id`/`archive_thread_id` on import (`ArchiveChannelSkipped`), keep `chat_conversations` permanently excluded from Backup, and leave Copy Settings untouched.
**Plans:** 8/8 plans complete

Plans:
- [x] 33-01-PLAN.md — Static reactions migration (`word_boundary`, `match_caption`, `chance_pct`, `cooldown_secs`), `models.Reactions`, `Rule` repo & `reaction_rules:` `skipLocal` cache
- [x] 33-02-PLAN.md — Presence cat-sticker & Highlights Archive migration, `chat_presence_archived` dedup table, rolling-deploy normalization & repo
- [x] 33-03-PLAN.md — 8-locale translations, `settings_locale_test.go` ceilings & `Presence` help labels for Phase 33
- [x] 33-04-PLAN.md — Smarter static reactions (`reactions.go` group `8` Unicode boundary/caption/chance/cooldown + `ctx.Data` AI handoff), zero-AI cat-sticker micro-triggers & Settings page `x` rule options
- [x] 33-05-PLAN.md — Highlights Archive runtime (`/setarchive`/`/unsetarchive` 2-step proof, Group `0` distinct-reactor tracking, durable DB dedup & `CopyMessage`/`ForwardMessage` delivery)
- [x] 33-06-PLAN.md — Backup & Restore DB parity (`BackupModulePresence`, `BackupModuleReactions` legacy normalization, `ArchiveChannelSkipped`, `chat_conversations` exclusion)
- [x] 33-07-PLAN.md — Private Settings page `pr` Cat-Sticker toggle & `ph` Highlights Archive sub-page UI (`len(settingsPages) == 31`, Main 17 rows)
- [x] 33-08-PLAN.md — Backup module/Settings 21-module wiring, `make generate-docs`, `Makefile`, `AGENTS.md` update & final Phase 33 verification gate
**UI hint**: yes

### Phase 34: Room-Aware Conversations and Frictionless Catch-Up
**Goal**: Conversations replies understand what the room was just talking about, members can address Xenobot naturally by name when opted in, and catching up on missed threads works via reply-free `/unroll*` arguments or a silent `✍` reaction straight to PM.
**Depends on**: Phase 32
**Requirements**: CHAT-01, CHAT-02, CHAT-03, CHAT-04, CHAT-05
**Planning notes**:
- Populate `chatLogEntry.At` from `msg.Date` (never `time.Now().Unix()`) and preserve original `At` on edits; treat `At == 0` legacy Redis entries gracefully.
- Filter every `chat_log` query (`Recent`, `SinceUser`, `SinceTime`) strictly by `entry.Thread == threadID` so forum topics never bleed into each other, and deduplicate ambient entries against `chatReadHistory` and the triggering message.
- Extract a silent PM delivery path (`silent: true`) for `✍` PM briefs instead of calling `chatJudgeDeliverPrivate`, guaranteeing zero group replies on both success and `403`.
**Success Criteria** (what must be TRUE):
  1. Logged group messages store backward-compatible original timestamps (`At: msg.Date`) that are not corrupted by message edits, and `Recent`, `SinceUser`, and `SinceTime` query helpers return only entries from the same forum thread.
  2. When Ambient Room Context and Message Log are enabled, Conversations replies incorporate the last 6–10 same-thread `chat_log` entries without duplicating turns already in `chatReadHistory` or repeating the triggering message.
  3. When Vocative Name-Addressing is enabled, a member starting a non-forwarded message with Xenobot's name (`"Xenobot, ..."` / `"Кзенобот, ..."`) triggers a Conversations reply without `@mention` or Reply, and that message does not also trigger an ambient AI reaction.
  4. Members can run `/unrollshort` and `/unrolllong` without replying by passing a count (`50`), self-anchor (`me`), or duration (`2h`), plus optional `pm` delivery.
  5. When `✍` PM Briefs and Message Log are enabled, reacting `✍` to a logged message delivers a private `/unrollshort` starting from that message to the reactor's PM and posts zero group messages whether PM delivery succeeds or fails with `403`.
**Plans:** 8/8 plans complete

Plans:
- [x] 34-01-PLAN.md — `chat_presence` room & catch-up migration (`ambient_context`, `vocative_enabled`, `pm_brief_react`), model, `skipLocal` repo & `BackupModulePresence` parity
- [x] 34-02-PLAN.md — Leaf `chatprompt.UserWithAmbient` prompt builder & pure Unicode `ParseVocativePrefix` parser (`TestChatpromptPackageIsLeaf`)
- [x] 34-03-PLAN.md — 8-locale translations & `xenobot/i18n` ceilings/alert/help validation for Room & Catch-Up (`pc`) and reply-free/PM briefs
- [x] 34-04-PLAN.md — Timestamped `chat_log` (`At: msg.Date`), edit timestamp preservation, legacy `At == 0` handling & strict topic-isolated query helpers
- [x] 34-05-PLAN.md — Ambient Room Context (`chat_run.go`, `chatReadHistoryWithIDs` dedup) & opt-in vocative name-addressing (`chat_watch.go`, mutually exclusive with `chatReactWatch`)
- [x] 34-06-PLAN.md — Reply-free `/unrollshort` & `/unrolllong <N|me|duration> [pm]` (`chat_brief.go`) & silent `✍` PM brief (`presence_reactions.go` → `chatBriefQueue` under `StopChat`)
- [x] 34-07-PLAN.md — `presence.go` apply layer & private Settings sub-page `pc` under `pr` (`8` rows, `31` pages, `17` Main rows) with Owner×Admin double-gating
- [x] 34-08-PLAN.md — `Makefile` PG integrity list, `make generate-docs` / `make check-docs`, `AGENTS.md` update & full Phase 34 verification gate
**UI hint**: yes

### Phase 35: Interactive Group Toys
**Goal**: Permitted group members can spark in-character participation through AI-drafted native polls with automatic deadpan tie-breakers (`/referendum`) and vision-capable "Facility Inspection" verdicts with matching emoji reactions (`/inspect`).
**Depends on**: Phase 34
**Requirements**: TOYS-01, TOYS-02, TOYS-03
**Planning notes**:
- Targeted prompt validation during planning: validate multi-field JSON output (`question` + 2–5 bounded `options` for `/referendum`; `verdict` + validated `emoji` for `/inspect`) across Claude, Gemini, and Featherless at `MaxTokens >= 512` (consider `768` for localized Ukrainian/Russian `/referendum` options so thinking/reasoning tokens never truncate options).
- Store `xenobot:chat:poll:<poll_id> -> {chat, msg, thread, lang}` with TTL strictly longer than `OpenPeriod` so closed-poll `handlers.NewPoll` updates (which omit `chat_id`) always resolve back to the right chat and forum topic.
- Guard tie-breaker delivery with `IsClosed && TotalVoterCount > 0 && topTie >= 2` plus an atomic cross-replica Redis claim so 0-vote polls or redelivered updates never post.
**Success Criteria** (what must be TRUE):
  1. When `/referendum` is enabled (and Owner Conversations is on), a permitted member (following `Audience`) runs `/referendum` from recent same-thread context or a replied debate and gets a validated, sanitized native Telegram poll (`SendPoll`) bounded by the dedicated `toy:b` hourly budget.
  2. When a bot-created `/referendum` poll closes in a tie with at least one vote (`TotalVoterCount > 0`), the bot claims it once-only across replicas and posts one deadpan Facility Tie-Breaker verdict in the original chat and forum topic; zero-vote or non-tied polls post nothing.
  3. When `/inspect` is enabled (and Owner Conversations is on), a permitted member replying `/inspect` to a text message, photo, or sticker receives a crisp 2–3 line deadpan "Facility Inspection" verdict (with vision via `chat_media.go` when available) plus a validated matching reaction emoji recorded in `bot_react` so `👎` retraction clears it.
**Plans:** 7 plans

Plans:
- [ ] 35-01-PLAN.md — `chat_presence` toys migration (`referendum_enabled`, `inspect_enabled`), model, `skipLocal` repo & `BackupModulePresence` parity
- [ ] 35-02-PLAN.md — Leaf `chatprompt/toys.go` prompt builders (`ReferendumMaxTokens = 768`, `InspectMaxTokens = 512`), lookalike warnings & strict JSON parsers
- [ ] 35-03-PLAN.md — 8-locale translations, `chat_help_msg` update & `xenobot/i18n` ceilings/button-width/tag validation (`settings_locale_test.go`, `chat_toys_locale_test.go`)
- [ ] 35-04-PLAN.md — `presence.go` apply layer & toy Redis keys + Private Settings sub-page `pc` (`rf`/`in`, 6 rows `<= 8`, `pr` 8 rows, `31` pages, `17` Main rows) with Owner×Admin gating
- [ ] 35-05-PLAN.md — Shared `StopChat` toy lane & `"toy:b"` budget, `/referendum` (`SendPollWithContext` + `xenobot:presence:poll:<id>`) & group `0` `handlers.NewPoll` tie-breaker (`TOYS-01`, `TOYS-02`)
- [ ] 35-06-PLAN.md — `/inspect` reply command (`chat_media.go` vision + Featherless `text_only` fallback, `AllowSendingWithoutReply: false` verdict + `presenceSetReaction(..., "inspect")` 24h `bot_react` / `👎` retraction) (`TOYS-03`)
- [ ] 35-07-PLAN.md — `Makefile` PG integrity list, `make generate-docs` / `make check-docs`, `AGENTS.md` update & full Phase 35 verification gate
**UI hint**: yes

## Progress

**Execution Order:**
Phases execute in numeric order: 31 → 32 → 33 → 34 → 35

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 31. Presence Foundation, Settings Page and Zero-AI Social Reactions | v1.3 | 5/5 | Complete    | 2026-11-27 |
| 32. Contextual AI Emoji Reactions | v1.3 | 6/6 | Complete    | 2026-11-27 |
| 33. Smarter Static Reactions, Highlights Archive and Backup Parity | v1.3 | 8/8 | Complete    | 2026-11-27 |
| 34. Room-Aware Conversations and Frictionless Catch-Up | v1.3 | 8/8 | Complete    | 2026-11-27 |
| 35. Interactive Group Toys | v1.3 | 0/7 | Planned | - |
