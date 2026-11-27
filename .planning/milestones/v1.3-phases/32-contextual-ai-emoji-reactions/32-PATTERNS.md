# Phase 32: Contextual AI Emoji Reactions - Pattern Map

**Mapped:** 2026-11-27
**Files analyzed:** 17 (4 new, 13 modified — counting the 8 `locales/*.yml` files as one locale group)
**Analogs found:** 17 / 17

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `migrations/20261128120000_add_chat_presence_ai_react_mode.sql` | migration | CRUD | `migrations/20261125120000_add_chat_conversation_log.sql` | exact |
| `xenobot/db/models/chat_presence.go` | model | CRUD | `xenobot/db/models/chat_conversation.go` | exact |
| `xenobot/db/presence/repository.go` | service | CRUD | `xenobot/db/chatconv/repository.go` | exact |
| `xenobot/db/presence/repository_test.go` | test | CRUD | `xenobot/db/presence/repository_test.go` | exact |
| `xenobot/utils/chatprompt/reaction.go` | utility | transform | `xenobot/utils/chatprompt/chatprompt.go` | exact |
| `xenobot/utils/chatprompt/reaction_test.go` | test | transform | `xenobot/utils/chatprompt/leaf_test.go` | exact |
| `xenobot/modules/presence.go` | service | event-driven | `xenobot/modules/presence.go` | exact |
| `xenobot/modules/presence_react_api.go` | service | request-response | `xenobot/modules/presence_react_api.go` | exact |
| `xenobot/modules/chat_log.go` | service | CRUD | `xenobot/modules/chat_log.go` | exact |
| `xenobot/modules/chat_react.go` | service | event-driven | `xenobot/modules/chat_brief.go` + `xenobot/modules/presence_reactions.go` | exact |
| `xenobot/modules/chat_run.go` | service | event-driven | `xenobot/modules/chat_run.go` | exact |
| `xenobot/modules/chat_watch.go` | middleware | event-driven | `xenobot/modules/chat_watch.go` | exact |
| `xenobot/modules/chat_react_test.go` | test | event-driven | `xenobot/modules/chat_watch_test.go` + `xenobot/modules/presence_reactions_test.go` | exact |
| `xenobot/modules/settings_presence.go` | controller | request-response | `xenobot/modules/settings_presence.go` + `xenobot/modules/settings_chat.go` | exact |
| `xenobot/modules/settings_presence_test.go` | test | request-response | `xenobot/modules/settings_presence_test.go` | exact |
| `locales/{en,uk,ru,es,fr,hi,id,pt}.yml` | config | transform | `locales/en.yml` (lines 2583-2594) | exact |
| `xenobot/i18n/settings_locale_test.go` | test | transform | `xenobot/i18n/settings_locale_test.go` (lines 932-965) | exact |

*(Also update `AGENTS.md` in the same commit with Phase 32 runtime, worker lane, Redis key, and UI invariants.)*

---

## Pattern Assignments

### `migrations/20261128120000_add_chat_presence_ai_react_mode.sql` (migration, CRUD)

**Analog:** `migrations/20261125120000_add_chat_conversation_log.sql` (and `migrations/20261127120000_add_chat_presence.sql`)

**Core pattern** (`migrations/20261125120000_add_chat_conversation_log.sql` lines 11-20):
- Append-only file strictly after `20261127120000_add_chat_presence.sql`.
- Put the named `CHECK` constraint **inline** on `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` so PostgreSQL skips the whole clause idempotently when the column already exists (both migration appliers swallow `duplicate_object` on a bare `ADD CONSTRAINT`).
```sql
ALTER TABLE chat_conversations ADD COLUMN IF NOT EXISTS log_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE chat_conversations ADD COLUMN IF NOT EXISTS judge_to SMALLINT NOT NULL DEFAULT 0 CONSTRAINT chk_chat_conversations_judge_to CHECK (judge_to BETWEEN 0 AND 2);
```
**Apply for Phase 32:**
```sql
ALTER TABLE chat_presence
    ADD COLUMN IF NOT EXISTS ai_react_mode VARCHAR(16) NOT NULL DEFAULT 'off'
    CONSTRAINT chk_chat_presence_ai_react_mode CHECK (ai_react_mode IN ('off', 'low', 'medium'));
```

---

### `xenobot/db/models/chat_presence.go` (model, CRUD)

**Analog:** `xenobot/db/models/chat_conversation.go` (lines 11-20) and `xenobot/db/models/chat_presence.go` (lines 11-23)

**String enum + CHECK tag pattern** (`xenobot/db/models/chat_conversation.go` lines 14):
```go
Provider   string    `gorm:"column:provider;not null;size:16;check:chk_chat_conversations_provider,provider IN ('anthropic', 'google', 'featherless')" json:"provider"`
```
**Existing struct to extend** (`xenobot/db/models/chat_presence.go` lines 11-18):
```go
type ChatPresence struct {
	ChatID             int64     `gorm:"column:chat_id;primaryKey;autoIncrement:false" json:"chat_id"`
	ConsensusEnabled   bool      `gorm:"column:consensus_enabled;not null" json:"consensus_enabled"`
	ConsensusThreshold int16     `gorm:"column:consensus_threshold;not null;check:chk_chat_presence_consensus_threshold,consensus_threshold BETWEEN 2 AND 20" json:"consensus_threshold"`
	RetractEnabled     bool      `gorm:"column:retract_enabled;not null" json:"retract_enabled"`
	UpdatedBy          int64     `gorm:"column:updated_by;not null" json:"updated_by"`
	UpdatedAt          time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}
```
**Apply for Phase 32:** Add `AIReactMode string` with `` `gorm:"column:ai_react_mode;not null;size:16;check:chk_chat_presence_ai_react_mode,ai_react_mode IN ('off', 'low', 'medium')" json:"ai_react_mode"` ``. Keep zero `default:` tags on booleans.

---

### `xenobot/db/presence/repository.go` (service, CRUD)

**Analog:** `xenobot/db/chatconv/repository.go` (lines 18-121) and `xenobot/db/presence/repository.go` (lines 18-140)

**String enum constants + validator + column-targeted upsert** (`xenobot/db/chatconv/repository.go` lines 18-23, 45-65, 113-121):
```go
const (
	ProviderAnthropic   = "anthropic"
	ProviderGoogle      = "google"
	ProviderFeatherless = "featherless"
)

func ValidProvider(p string) bool {
	return p == ProviderAnthropic || p == ProviderGoogle || p == ProviderFeatherless
}

func SetProvider(ctx context.Context, chatID int64, provider string, by int64) error {
	if !ValidProvider(provider) {
		return ErrInvalidValue
	}
	row := defaultRow(chatID, by)
	row.Provider = provider
	return upsert(ctx, row, "provider")
}
```
**Existing msgpack `Settings`, `GetFresh`, `defaultRow`, and `upsert`** (`xenobot/db/presence/repository.go` lines 34-49, 82-87, 113-140):
```go
type Settings struct {
	ConsensusEnabled   bool `msgpack:"ce"`
	ConsensusThreshold int  `msgpack:"ct"`
	RetractEnabled     bool `msgpack:"re"`
}
```
**Apply for Phase 32:**
1. Export `AIReactModeOff = "off"`, `AIReactModeLow = "low"`, `AIReactModeMedium = "medium"` and `ValidAIReactMode(m string) bool`.
2. Add `AIReactMode string \`msgpack:"ar"\`` to `Settings`, default `AIReactModeOff` in `Defaults()`, populate in `GetFresh` and `defaultRow`.
3. **Rolling-deploy cache safety:** A `chat_presence:<chat>` msgpack value cached by a Phase 31 replica has no `"ar"` field and decodes `AIReactMode == ""`. Normalize `""` → `AIReactModeOff` in `Get()` (after `GetFromCacheOrLoad`) and/or treat `!ValidAIReactMode(mode)` as `AIReactModeOff` at readers!
4. Add `SetAIReactMode(ctx context.Context, chatID int64, mode string, by int64) error` calling `upsert(ctx, row, "ai_react_mode")`. Cache key `chat_presence:<chat>` is already in `skipLocal` (`xenobot/db/cache/local.go`) and dropped by `upsert` on line 138.

---

### `xenobot/db/presence/repository_test.go` (test, CRUD)

**Analog:** `xenobot/db/presence/repository_test.go` (lines 39-198)

**Test pattern:** Single `TestChatPresenceSettings` with `newChat(t)`, `utilsCache.SetupTestMemoryMarshaler(t)`, testing:
- Missing row returns `Defaults()` (`AIReactMode: AIReactModeOff`) with `rowCount == 0`.
- `SetAIReactMode` rejects invalid values (`""`, `"high"`, `"OFF"`) with `ErrInvalidValue` and zero DB inserts.
- `SetAIReactMode` column-targeted upsert preserves `ConsensusEnabled`, `ConsensusThreshold`, `RetractEnabled`, and invalidates `CacheKey(chatID)`.
- Raw `db.DB.Create(&models.ChatPresence{..., AIReactMode: "high"})` is rejected by `chk_chat_presence_ai_react_mode`.

---

### `xenobot/utils/chatprompt/reaction.go` (utility, transform)

**Analog:** `xenobot/utils/chatprompt/chatprompt.go` (lines 1-17, 103-186, 206-227) + Spike 001/002 validated prompt & JSON parser (`.planning/spikes/001-reaction-picker-tokens-and-format/main.go` lines 121-270, `.planning/spikes/002-reaction-abstention-and-persona/main.go` lines 36-92)

**Leaf-package constraint** (`xenobot/utils/chatprompt/leaf_test.go` lines 14-39):
```go
// TestChatpromptPackageIsLeaf keeps the package free of xenobot/config and xenobot/db,
// whose init functions connect to PostgreSQL before main: it may import the
// standard library and the xenopersona leaf only.
```
Any new file in `xenobot/utils/chatprompt/` MUST import only the Go standard library (`bytes`, `encoding/json`, `slices`, `sort`, `strings`, `unicode`) — zero `gotgbot`, zero `xenobot/config`, zero `xenobot/db`.

**Imports & sanitization pattern** (`xenobot/utils/chatprompt/chatprompt.go` lines 11-17, 177-227):
- Reuse `Clean(s, max)`, `Name(s)`, `line(t, max)`, `MaxMessageRunes`, `MaxQuotedRunes`, `MaxTitleRunes` so members cannot forge `[...]` section headers or impersonate `Xenobot-72`.
- Keep `ReactionStatic()` compact (~450 tokens, byte-identical across chats; **never** embed `xenopersona.StaticSystemText()`'s 39 KB comic archive).
- Define `const ReactionMaxTokens = 512` (proven cross-provider floor from Spikes 001–003).
- Put per-chat title, group language, intersected `AvailableReactions` palette, explicit lookalike warning (`😂` and cat-face emojis `🐱`/`😹`/`😻`/`😼` are NOT valid Telegram reactions — use `🤣`, `❤`, `👀`, `🥰`), and strict JSON output contract (`{"emoji":"🔥"}` or `{"emoji":"NONE"}`) in `ReactionDynamic(...)`.
- Put optional `[Recent thread context, oldest first]`, optional `[Message being replied to]`, optional `[Quoted excerpt]` (for `msg.Quote.Text`), `[Latest message to score for a reaction]`, and untrusted-data closing reminder in `ReactionUser(...)`.

**Strict JSON parser pattern** (adapted from Spike 001 `ParseReaction`, `.planning/spikes/001-reaction-picker-tokens-and-format/main.go` lines 189-270):
- Trim whitespace and strip optional markdown ```` ```json ... ``` ```` fences (`stripCodeFence`).
- Unmarshal into `map[string]json.RawMessage`: require `"emoji"` key to exist (reject `{}` / non-object / prose).
- Reject `null` or `""`? Note `32-CONTEXT.md` line 34: *"Strict JSON parser requiring `"emoji"` key (rejecting `{}`/`null`/prose/multi-emoji/non-allowlisted emojis), normalizing `U+FE0F`."*
  - **Important distinction vs Spike 001:** In `32-CONTEXT.md`, `{"emoji":null}` or top-level `null` must be rejected (treated as parse error / shed to `NONE`), whereas `{"emoji":"NONE"}` (case-insensitive) is a valid `NONE`. Check `bytes.Equal(trimmedVal, []byte("null"))` → return parse error!
- Unmarshal `"emoji"` strictly as JSON string; if `strings.EqualFold(token, "NONE")` → `ReactionParseNone`.
- Normalize `norm := strings.ReplaceAll(strings.TrimSpace(token), "\ufe0f", "")` (matching `normaliseReactionEmoji` in `reactions.go:434-436`).
- Check exact membership in the normalized `allowed` palette (intersected `supportedReactionEmoji ∩ AvailableReactions`); reject anything outside `allowed` (automatically rejecting `😂`, `🐱`, `😹`, `😻`, `😼`, multi-emoji strings, and chat-disallowed standard emojis).

---

### `xenobot/utils/chatprompt/reaction_test.go` (test, transform)

**Analog:** `xenobot/utils/chatprompt/leaf_test.go` + `xenobot/utils/chatprompt/chatprompt_test.go`

**Core pattern:** Pure millisecond unit tests (zero DB, zero Redis, zero HTTP):
- Prompt assembly: `ReactionStatic()` stays compact (`< 3000` bytes, no comic archive), includes red-line/abstention/drama rules and lookalike warning (`😂`, `🐱`/`😹`/`😻`/`😼`), `ReactionDynamic` includes chat palette and JSON contract, `ReactionUser` includes recent log, reply target, `Quote` text, and strips control/newline chars via `Clean`/`Name`.
- `ParseReactionJSON`: all 7 `U+FE0F` standard emojis (`❤️`→`❤`, `❤️‍🔥`→`❤‍🔥`, `✍️`→`✍`, `🕊️`→`🕊`, `☃️`→`☃`, `🤷‍♂️`→`🤷‍♂`, `🤷‍♀️`→`🤷‍♀`), fenced ```` ```json\n{"emoji":"🔥"}\n``` ````, `{"emoji":"NONE"}`, restricted 5-emoji palette rejection, `{}` / `null` / `{"emoji":null}` / `{"emoji":""}` / prose / multi-emoji (`"🔥👏"`) / lookalikes (`😂`, `🐱`, `😹`, `😻`, `😼`) all rejected.
- `TestChatpromptPackageIsLeaf` automatically enforces zero forbidden imports.

---

### `xenobot/modules/presence.go` (service, event-driven)

**Analog:** `xenobot/modules/presence.go` (lines 21-60, 126-201)

**Operational Redis key pattern** (lines 24-60 — strictly outside `xenobot:cache:`, using `{<chatID>}` Redis Cluster hash tag):
```go
const presenceKeyPrefix = "xenobot:presence:"

func presenceChatTag(chatID int64) string {
	return presenceKeyPrefix + "{" + strconv.FormatInt(chatID, 10) + "}:"
}

func presenceBotReactKey(chatID, msgID int64) string {
	return presenceChatTag(chatID) + "bot_react:" + strconv.FormatInt(msgID, 10)
}

func presenceUserCooldownKey(chatID, userID int64) string {
	return presenceChatTag(chatID) + "cd:u:" + strconv.FormatInt(userID, 10)
}
```
**Add to `presence.go` for Phase 32:**
1. `const presenceStaticReactedKey = "presence.static_reacted"` (handoff marker respected in group 13 so Phase 33's group 8 drops in cleanly).
2. Redis key helpers under `presenceChatTag(chatID)` (same `{<chatID>}` slot so one Lua script can read/write all three atomically in Redis Cluster!):
   - `presenceAIReactGapKey(chatID)` → `xenobot:presence:{<chat>}:ai:gap`
   - `presenceAIReactChatCooldownKey(chatID)` → `xenobot:presence:{<chat>}:ai:cd:chat`
   - Reuse existing `presenceUserCooldownKey(chatID, userID)` → `xenobot:presence:{<chat>}:cd:u:<user>` so a 24h `👎` retraction cooldown automatically blocks AI reactions to that member!
3. Extend `presenceWrites` struct with `AIReactMode func(ctx context.Context, chatID int64, mode string, by int64) error` (default `presence.SetAIReactMode`) and add `applyPresenceAIReactMode(ctx, chatID, mode, by) presenceApplyResult` mirroring `applyPresenceConsensusThreshold` (lines 173-182).

---

### `xenobot/modules/presence_react_api.go` (service, request-response)

**Analog:** `xenobot/modules/presence_react_api.go` (lines 77-122, 190-227)

**Existing `AvailableReactions` cache loader** (lines 92-122):
```go
func presenceAllowedEmoji(ctx context.Context, b *gotgbot.Bot, chatID int64, emoji string) bool {
	if b == nil || emoji == "" {
		return false
	}
	avail, err := cache.GetFromCacheOrLoad(ctx, presenceAvailReactKey(chatID), presenceAvailReactTTL, func(loadCtx context.Context) (presenceAvailReactions, error) {
		var info *gotgbot.ChatFullInfo
		callErr := presencePaced(loadCtx, func(callCtx context.Context) error {
			var err error
			info, err = b.GetChatWithContext(callCtx, chatID, nil)
			return err
		})
		if callErr != nil {
			return presenceAvailReactions{}, callErr
		}
		if info == nil || len(info.AvailableReactions) == 0 {
			return presenceAvailReactions{All: true}, nil
		}
		return presenceAvailReactions{
			All:     false,
			Allowed: presenceStandardEmojis(info.AvailableReactions),
		}, nil
	})
	if err != nil {
		log.WithField("chat_id", chatID).Debugf("[Presence] getChat for available reactions failed: %s", chatSendErrClass(err))
		return false
	}
	if avail.All {
		return true
	}
	return slices.Contains(avail.Allowed, emoji)
}
```
**Apply for Phase 32:** Extract `presenceAllowedPalette(ctx context.Context, b *gotgbot.Bot, chatID int64) ([]string, bool)` using that exact `cache.GetFromCacheOrLoad` call:
- On error → return `nil, false` (fail closed).
- If `avail.All` → return `slices.Clone(supportedReactionEmoji), true`.
- Otherwise → return `slices.Clone(avail.Allowed), len(avail.Allowed) > 0` (a custom-only chat returns `nil, false`, skipping the AI call completely!).
- Implement `presenceAllowedEmoji` as a thin wrapper over `presenceAllowedPalette` (zero behavior change for Phase 31).
- Reuse `presenceSetReaction(ctx, b, chatID, msgID, authorID, emoji, "ai")` unchanged — it enforces `IsBig: false`, `AvailableReactions`, `presenceReactionPacer`, `REACTION_INVALID` cache eviction, 24h `bot_react` metadata (`"s":"ai"`), and never touches `chatBreakers`.

---

### `xenobot/modules/chat_log.go` (service, CRUD)

**Analog:** `xenobot/modules/chat_log.go` (lines 119-139)

**Existing read pattern** (lines 121-139):
```go
func chatLogSince(ctx context.Context, client *redis.Client, chatID, fromID int64) ([]chatLogEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, chatLogReadTimeout)
	defer cancel()
	raw, err := client.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key: chatLogKey(chatID), Start: strconv.FormatInt(fromID, 10), Stop: "+inf", ByScore: true,
	}).Result()
	if err != nil {
		return nil, err
	}
	entries := make([]chatLogEntry, 0, len(raw))
	for _, item := range raw {
		var entry chatLogEntry
		if json.Unmarshal([]byte(item), &entry) != nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}
```
**Apply for Phase 32:** Add `chatLogRecentThread(ctx context.Context, client *redis.Client, chatID, threadID, beforeMsgID int64, limit int) ([]chatLogEntry, error)`:
- Use bounded reverse scan (`ZRevRange(ctx, chatLogKey(chatID), 0, 49)` or `ZRangeArgs{ByScore: true, Rev: true, Start: ..., Stop: "-inf", Count: 50}`) instead of reading all 1000 entries (avoids Pitfall 9 / Performance Trap).
- Filter strictly `entry.Thread == threadID && entry.MessageID < beforeMsgID` (guarantees zero forum-topic cross-leak and deterministic exclusion of the triggering message regardless of group 13 vs group 14 timing).
- Collect up to `limit` (e.g., 5), then `slices.Reverse(entries)` to return oldest-first.

---

### `xenobot/modules/chat_react.go` (service, event-driven)

**Analogs:**
- Worker lane & stats: `xenobot/modules/chat_brief.go` (lines 37-82, 297-338) + `xenobot/modules/chat_run.go` (lines 227-250, 380-390)
- Redis Lua gate & safety guards: `xenobot/modules/presence_reactions.go` (lines 61-86, 296-304, 319-336)
- Media extraction & download: `xenobot/modules/chat_media.go` (lines 138-187, 191-211, 232-275)
- Featherless HTTP error inspection: `xenobot/utils/featherless/errors.go` (lines 24-78)

**1. Dedicated worker lane joining `StopChat`** (copy from `xenobot/modules/chat_brief.go` lines 297-338):
```go
const (
	chatPresenceQueueCapacity = 32
	chatPresenceWorkerCount   = 2
	chatReactJobTimeout       = 45 * time.Second
)

var (
	chatPresenceQueue chan chatReactJob
	chatReactProcess  = chatReactProcessJob
)

func chatPresenceEnsureWorkers() {
	chatPresenceQueue = make(chan chatReactJob, chatPresenceQueueCapacity)
	for range chatPresenceWorkerCount {
		go chatPresenceWorker()
	}
}

func chatPresenceWorker() {
	defer error_handling.RecoverFromPanic("chatPresenceWorker", "Chat")
	for job := range chatPresenceQueue {
		func() {
			defer chatJobs.finish()
			defer error_handling.RecoverFromPanic("chatReactProcess", "Chat")
			if chatContext().Err() != nil {
				chatReactShedCount.Add(1)
				return
			}
			chatReactProcess(job)
		}()
	}
}

func chatPresenceEnqueue(job chatReactJob) bool {
	chatEnsureWorkers()
	chatJobs.start()
	if chatDraining.Load() {
		chatJobs.finish()
		chatReactShedCount.Add(1)
		return false
	}
	select {
	case chatPresenceQueue <- job:
		return true
	default:
		chatJobs.finish()
		chatReactShedCount.Add(1)
		log.WithFields(log.Fields{"chat_id": job.chatID, "user_id": job.userID}).Debug("[Chat] presence queue full, reaction shed")
		return false
	}
}
```
Notice: zero new shutdown drains, zero `main.go` changes, zero starvation of `chatQueue` (3 workers) or `chatBriefQueue` (1 worker).

**2. Pre-AI cheap gate + cross-replica Redis Lua gate (`chatReactWatch`)**:
Order of checks (cheapest first, zero Postgres calls on hot path):
1. `if !chatAnyProviderConfigured()` → return.
2. `if reacted, _ := ctx.Data[presenceStaticReactedKey].(bool); reacted` → return.
3. `userID, ok := presenceMessageAllowed(b, ctx)` → skips non-group, slash text, bots, anonymous channels/admins, and Staff Groups (`presenceStaffLookup`).
4. Skip messages with no scorable content (no text, no caption, no photo, no sticker).
5. Cached `skipLocal` reads (bounded by `chatWatchTimeout`):
   - `conv, err := chatSettingsLookup(lookupCtx, chat.Id)` → require `err == nil && conv.Enabled && chatProviderConfigured(conv.Provider)`.
   - `pres, err := presenceSettingsGet(lookupCtx, chat.Id)` → require `err == nil && pres.AIReactMode != presence.AIReactModeOff` (and valid mode).
6. `client := presenceRedisClient(); if client == nil` → return.
7. Atomic Redis Lua gate (`chatReactGateScript`) on the 3 same-slot `{<chat>}` keys (`KEYS[1] = presenceAIReactGapKey`, `KEYS[2] = presenceAIReactChatCooldownKey`, `KEYS[3] = presenceUserCooldownKey`):
   - Always `local gap = redis.call('INCR', KEYS[1])` and `EXPIRE KEYS[1] gapTTL` (e.g. 24h).
   - If `gap < minGap` OR `EXISTS KEYS[2] == 1` OR `EXISTS KEYS[3] == 1` → return `0`.
   - Pass pre-rolled `win` (`0` or `1` from `math/rand/v2` seam `chatReactRoll(mode)`) as an `ARGV` (avoiding `math.random` inside Redis Lua, which is disallowed in deterministic Lua scripts!):
     - If `win == 1`: `SET KEYS[1] 0 EX gapTTL`, `SET KEYS[2] '1' EX chatCD`, `SET KEYS[3] '1' EX userCD`, return `1`.
     - If `win == 0`: keep `gap` at `>= minGap` (or leave counter intact) and return `0`.
   - Note on `👎` retraction interaction: Because `KEYS[3]` is `presenceUserCooldownKey(chatID, userID)` (set to 24h by `presenceExtendUserCooldown`), the Lua `EXISTS KEYS[3] == 1` check automatically honors the 24h retraction cooldown without overwriting its 24h TTL!
8. Capture lightweight `chatReactJob` (IDs, `threadID`, `chatTitle`, `provider`, `logEnabled`, `turn`, `quoted` from `extraction.ReplyTarget(msg)`, `quoteText` from `msg.Quote.Text`, and optional photo/sticker `mediaRef` via `chatExtractReactionMediaRef(msg)` — zero I/O in watcher) and call `chatPresenceEnqueue(job)`.

**3. Worker execution (`chatReactProcessJob`)**:
1. Context derived from `root := chatContext()` with `chatReactJobTimeout` (`45 * time.Second`).
2. Re-check `client := chatRedisClient()` and `chatProviderConfigured(job.provider)`.
3. Lazy fail-closed lockdown check: `if presenceIsLockedDown(job.chatID)` → shed silently.
4. Read-only breaker check: `if chatBreaker(job.provider).isPaused(job.chatID, raidNow())` → shed silently.
5. Dedicated hourly budget via `chatReserveWith` (`chat_run.go:229-250`):
   ```go
   switch chatReserveWith(client, job.chatID, job.userID, "react:b",
       chatReactUserHourlyBudget, chatReactChatHourlyBudget, chatReactGlobalHourlyBudget) {
   case chatBudgetCapped:
       chatReactCappedCount.Add(1)
       return
   case chatBudgetFault:
       chatReactShedCount.Add(1)
       return
   }
   ```
6. Resolve allowed palette: `palette, ok := presenceAllowedPalette(ctx, job.bot, job.chatID)` → if `!ok || len(palette) == 0`, shed silently before any media download or LLM call!
7. Context assembly (`REACT-03`):
   - If `job.logEnabled`: `chatLogRecentThread(ctx, client, job.chatID, job.threadID, job.messageID, 5)` (best effort; Redis error degrades to empty log).
   - Always pass `job.quoted` (from `extraction.ReplyTarget`) and `job.quoteText` (from `msg.Quote.Text`) even when `LogEnabled` is off.
8. Vision media download & Featherless `text_only` caching (`REACT-03`):
   - Only download photos and sticker thumbnails (`chatExtractReactionMediaRef`: `msg.Photo` via `raidAIPhotoRef` or `msg.Sticker` via `chatExtractStickerRef` — never PDFs or text files).
   - If `job.provider == chatconv.ProviderFeatherless` and Redis/cache has `featherlessTextOnly` cached for `config.AppConfig.ChatFeatherlessModel`: skip image download and send text/caption/`[sticker <emoji>]` only.
   - Otherwise call `chatDownloadMedia(ctx, job.bot, job.chatID, job.userID, job.provider, job.media)`.
9. Provider call (`REACT-02`):
   ```go
   req := chatRequest{
       Static:      chatprompt.ReactionStatic(),
       Dynamic:     chatprompt.ReactionDynamic(job.chatTitle, language, palette),
       User:        chatprompt.ReactionUser(recentTurns, job.quoted, job.quoteText, job.turn),
       MaxTokens:   chatprompt.ReactionMaxTokens, // 512
       Attachments: media.Attachments,
   }
   answer, err := chatProviderOf(job.provider).converse(ctx, req)
   ```
   - **Featherless vision 400/422 auto-fallback:** If `err != nil && job.provider == chatconv.ProviderFeatherless && len(req.Attachments) > 0`:
     ```go
     var fErr *featherless.HTTPError
     if errors.As(err, &fErr) && (fErr.Status == http.StatusBadRequest || fErr.Status == http.StatusUnprocessableEntity) && !fErr.KeyRejected() {
         chatMarkFeatherlessTextOnly(ctx, client, config.AppConfig.ChatFeatherlessModel) // 24h TTL
         req.Attachments = nil
         answer, err = chatProviderOf(job.provider).converse(ctx, req)
     }
     ```
     *(If the message had no text/caption and only a captionless `[photo]` after stripping attachments, either skip retry or retry with `[photo]` — Spike 003 confirmed text-only Featherless cleanly returns `NONE` on `[photo]` and uses `[sticker <emoji>]` + context on stickers.)*
   - **Non-tripping error shedding:** On ANY error (`err != nil`, `answer.Outcome == chatRefused`, or `ParseReactionJSON` failure), log only error class at `Debug`, increment `chatReactShedCount` (or `chatReactNoneCount`), and return — **NEVER call `chatRecordFailure` and NEVER touch `chatBreakers[provider]`**!
10. Parse & apply:
    - `emoji, status := chatprompt.ParseReactionJSON(answer.Text, palette)`
    - If `status == chatprompt.ReactionParseEmoji`: call `presenceSetReaction(ctx, job.bot, job.chatID, job.messageID, job.userID, emoji, "ai")`; increment `chatReactSentCount` on `true`.
    - Otherwise increment `chatReactNoneCount` / `chatReactShedCount`.

---

### `xenobot/modules/chat_run.go` (service, event-driven)

**Analog:** `xenobot/modules/chat_run.go` (lines 139-155)

**Exact wiring sites:**
```go
// lines 139-144:
func chatStatsSection() string {
	return fmt.Sprintf(
		"\n    <b>Conversations:</b> answered %d, refused %d, failed %d, shed %d, capped %d",
		chatAnsweredCount.Load(), chatRefusedCount.Load(), chatFailedCount.Load(), chatShedCount.Load(), chatCappedCount.Load(),
	) + chatBriefStatsSection() + chatReactStatsSection()
}

// lines 146-155:
func chatEnsureWorkers() {
	chatWorkersOnce.Do(func() {
		chatQueue = make(chan chatJob, chatQueueCapacity)
		chatContext()
		for range chatWorkerCount {
			go chatWorker()
		}
		chatBriefEnsureWorkers()
		chatPresenceEnsureWorkers()
	})
}
```

---

### `xenobot/modules/chat_watch.go` (middleware, event-driven)

**Analog:** `xenobot/modules/chat_watch.go` (lines 61-82)

**Exact hook site** (lines 74-82):
```go
	if addressed, _ := chatAddressed(b, msg); !addressed {
		chatReactWatch(b, ctx)
		return
	}
	gate, provider := chatAccept(b, chat.Id, userID)
	if gate != chatGateOpen {
		return
	}
	chatEnqueue(chatJobOf(b, msg, userID, provider))
```
**Why here:**
- Strict mutual exclusion with Conversations: an addressed message (`@mention` or reply to bot's conversation message) never enters `chatReactWatch`.
- `chatWatcher` still returns `ext.ContinueGroups` on every path (so group 14 `chatLogWatcher` always runs).
- Note: Keep `TestChatWatcherIgnores` green — `chatReactWatch` must do zero I/O when `chatAnyProviderConfigured()` is false or `presenceSettingsGet` returns default `AIReactModeOff` (no Redis gate call when `AIReactMode == "off"`!). Double-check `TestChatWatcherIgnores` in `chat_watch_test.go` to confirm whether it asserts zero `chatSettingsLookup` calls on unaddressed messages!
  - **CRITICAL CHECK for `chat_watch_test.go:75-120`:** Look at the doc comment on `TestChatWatcherIgnores` (line 75): *"holds every message that must get no reply and, for the unaddressed ones, no I/O at all."* Let's note explicitly for the planner: inspect the full body of `TestChatWatcherIgnores` — if `newChatEnv` enables Conversations and asserts zero Redis/DB calls on unaddressed messages, checking `presenceSettingsGet` (or gating on `presenceSettingsGet` / `chatSettingsLookup`) will be hit unless `TestChatWatcherIgnores` is inspected and kept compatible (e.g. `chatReactWatch` checks `presenceSettingsGet` first, which is a separate seam from `chatSettingsLookup`, or `newChatEnv` stubs `presenceSettingsGet` to return `AIReactModeOff` with zero DB/Redis calls).

---

### `xenobot/modules/chat_react_test.go` (test, event-driven)

**Analogs:**
- Pool shutdown & restore: `xenobot/modules/chat_watch_test.go` (lines 303-346 — `TestShutdownEndsChat`, `restoreChatPool`) and `xenobot/modules/presence_reactions_test.go` (lines 846-890 — `TestShutdownEndsPresenceReaction`)
- Wire bot & breaker cleanliness assertion: `xenobot/modules/presence_react_api_test.go` (lines 24-62 — `withFastPresencePacer`, `newWireTestBot`, `assertChatBreakersClean`)

**Required test seams & cleanup discipline** (per `AGENTS.md` & Pitfall 9):
- Never call `t.Parallel()` on tests that swap package globals (`chatSwap(t, &..., ...)` restores via `t.Cleanup`).
- Use `withChatRedis(t)` (miniredis), `withFastPresencePacer(t)`, `utilsCache.SetupTestMemoryMarshaler(t)`, `newWireTestBot(t, ...)`, and fake `chatProviderOf` — zero real HTTP, zero SQLite where pure seams suffice, keeping total file runtime `< 2 s` under `-race`.
- Explicitly assert `assertChatBreakersClean(t)` after parse errors, `ErrNoReply`, Featherless `no choices`, Featherless `400`/`422`, and Telegram `400 REACTION_INVALID`.
- Add `TestShutdownEndsChatReact` proving `StopChat()` cancels a hung reaction provider call within `chatStopWait`, sends nothing, and sheds subsequent messages, followed by `restoreChatPool(t)`.

---

### `xenobot/modules/settings_presence.go` (controller, request-response)

**Analogs:**
- Page `pr` structure, Staff fail-closed gate, group refusal, apply/redraw pattern: `xenobot/modules/settings_presence.go` (lines 22-295)
- 3-choice segmented button row (`st` action + `settingsSelectedPrefix`): `xenobot/modules/settings_chat.go` (lines 51-69, 229-244)

**3-choice control & button row pattern** (`xenobot/modules/settings_chat.go` lines 53-69, 231-244):
```go
var settingsPresenceAIReactCodes = map[string]string{
	"o": presence.AIReactModeOff,
	"l": presence.AIReactModeLow,
	"m": presence.AIReactModeMedium,
}

var settingsPresenceAIReactKeys = map[string]string{
	presence.AIReactModeOff:    "settings_btn_pr_ai_off",
	presence.AIReactModeLow:    "settings_btn_pr_ai_low",
	presence.AIReactModeMedium: "settings_btn_pr_ai_medium",
}

func settingsPresenceAIReactRow(k *settingsKeyboard, stored string) []gotgbot.InlineKeyboardButton {
	var row []gotgbot.InlineKeyboardButton
	for _, code := range []string{"o", "l", "m"} {
		mode := settingsPresenceAIReactCodes[code]
		label := settingsText(k.view.Tr, settingsPresenceAIReactKeys[mode])
		if mode == stored {
			label = settingsSelectedPrefix + label
		}
		if button, ok := k.button(label, map[string]string{
			"a": settingsActSet, "p": settingsPagePresence, "t": settingsPresenceControlAIReact, "v": code,
		}); ok {
			row = append(row, button)
		}
	}
	return row
}
```
**Owner×Admin double-gating on page `pr` (`PRES-04`):**
- Add test seam `var settingsPresenceChatRead = chatconv.GetFresh` (alongside `settingsPresenceStaffLookup` and `settingsPresenceRead`).
- Register control `settingsPresenceControlAIReact = "ai"` with `Right: settingsRightChangeInfo`, `Actions: []string{settingsActSet}`, `Validate: func(v string) bool { _, ok := settingsPresenceAIReactCodes[v]; return ok }`.
- In `settingsPresenceSetAIReact(p, value)`:
  1. Run `settingsPresenceGroupRefused(p) || settingsPresenceStaffGate(p)`.
  2. Look up `mode := settingsPresenceAIReactCodes[value]`.
  3. **If `mode != presence.AIReactModeOff`:** check `chatAnyProviderConfigured()` and read fresh `conv, err := settingsPresenceReadChat(p.group.Id)`:
     - On `err != nil` → `p.cannotOpen(); return nil` (fail closed).
     - If `!chatAnyProviderConfigured() || !conv.Enabled || !chatProviderConfigured(conv.Provider)` → refuse write and `p.answer(settingsText(p.tr, "settings_pr_ai_need_owner"), true); return nil` (plain-text alert, `<= 200` chars).
  4. **If `mode == presence.AIReactModeOff`:** ALWAYS allow disabling without requiring Conversations to be enabled!
  5. Delegate to `settingsPresenceSetStored(...)` → `applyPresenceAIReactMode(...)`.
- In `renderSettingsPresence(v, stored, convUnlocked, notice)`:
  - Show `settings_pr_ai_line` (`Contextual AI reactions: {mode}`) and, when `!convUnlocked`, an explicit lock note (`settings_pr_ai_locked_note` explaining the Owner must enable Conversations first).
  - Keyboard has **5 rows** (`<= 8` cap): Consensus toggle, Threshold stepper, Retract toggle, AI Reactions 3-button row `[Off | Low | Medium]`, `navRow(false)`.
- In `settingsPresenceMainState`: read `stored` and (when `stored.AIReactMode != presence.AIReactModeOff`) check `conv.Enabled && chatProviderConfigured(conv.Provider)` before counting AI reactions as active (or show active count accurately so turning off Conversations immediately reflects on Main).

---

### `xenobot/modules/settings_presence_test.go` (test, request-response)

**Analog:** `xenobot/modules/settings_presence_test.go` (lines 59-160)

**Exact test updates:**
- `TestSettingsPresenceCallbackBytes`: include `AIReactMode` values (`"off"`, `"low"`, `"medium"`) × `convUnlocked` (`false`/`true`), assert `Omitted == 0`, `rows == 5`, and every callback data `1..64` bytes.
- `TestSettingsPresencePageFitsEveryLocale`: test worst-case states across all 8 locales (`uk, en, es, fr, hi, id, pt, ru`), assert `rows <= 8`, UTF-16 `<= 3800`, every button `1..40` runes.
- `TestSettingsPresenceFlows`: add subtests proving:
  - Regular admin (with settings access) cannot enable `low`/`medium` while Owner has `chat_conversations.enabled == false` (receives `settings_pr_ai_need_owner` alert, zero DB writes).
  - Once Owner enables Conversations, admin can switch `off` → `low` → `medium` → `off`.
  - Disabling (`medium` → `off`) still succeeds even after Owner disables Conversations.
  - Zero-AI Consensus & Retract toggles continue working regardless of Conversations state.

---

### `locales/{en,uk,ru,es,fr,hi,id,pt}.yml` & `xenobot/i18n/settings_locale_test.go` (config & test, transform)

**Analogs:**
- `locales/en.yml` (lines 2583-2594)
- `xenobot/i18n/settings_locale_test.go` (lines 932-965)

**Existing limits & alert registry** (`xenobot/i18n/settings_locale_test.go` lines 934-965):
```go
	"settings_page_presence":     {14, 17},
	"settings_state_presence_on": {10, 16},
	"settings_pr_group_note":     {160, 220},
	"settings_pr_intro":          {200, 260},
	"settings_pr_consensus_line": {36, 48},
	"settings_pr_threshold_line": {48, 60},
	"settings_pr_retract_line":   {40, 52},
	"settings_pr_hint":           {170, 220},
	"settings_btn_pr_consensus":  {30, 34},
	"settings_btn_pr_retract":    {28, 34},
	"settings_btn_chat_presence": {24, 30},
```
**Apply for Phase 32:**
- Add every new `settings_pr_*` / `settings_btn_pr_*` key to all 8 locale files (`en, uk, ru, es, fr, hi, id, pt`) AND register exact `{enMax, otherMax}` bounds in `settingsLocaleLimits` (`settings_locale_test.go`), plus add `settings_pr_ai_need_owner` to `settingsAlertKeys` (plain text, no HTML tags, `<= 200` chars).
- Keep the 3 segmented button labels (`settings_btn_pr_ai_off`, `settings_btn_pr_ai_low`, `settings_btn_pr_ai_medium`) short (e.g. `<= 12` runes) so all 3 buttons fit comfortably on one row with `settingsSelectedPrefix` (`"✓ "`) on narrow phones.
- Run `make check-translations` and `go test -tags testtools -race ./xenobot/i18n/...`.

---

## Shared Patterns

### 1. Owner×Admin Double-Gating (`PRES-04`)
**Sources:** `xenobot/modules/chat_watch.go:100-121`, `xenobot/modules/presence.go:97-117`, `xenobot/modules/settings_presence.go:120-143`
**Apply to:** `chat_react.go`, `settings_presence.go`
- Runtime AI reaction gate requires ALL of: `chatAnyProviderConfigured()` + `conv.Enabled` + `chatProviderConfigured(conv.Provider)` + `pres.AIReactMode != presence.AIReactModeOff` + `presenceMessageAllowed` (group/supergroup, non-slash, non-bot, non-anonymous, non-Staff) + lazy fail-closed `!presenceIsLockedDown(chatID)`.
- Disabling EITHER `chat_conversations.enabled` (Owner) OR `chat_presence.ai_react_mode` (Admin) stops AI reactions immediately without affecting zero-AI Consensus or `👎` Retraction.

### 2. Zero New Shutdown Drains via `StopChat` Multi-Lane Pool
**Sources:** `xenobot/modules/chat_run.go:146-191, 380-390`, `xenobot/modules/chat_brief.go:297-338`, `xenobot/modules/presence_reactions.go:155-171`
**Apply to:** `chat_react.go`, `chat_run.go`
- Launch `chatPresenceEnsureWorkers()` inside `chatEnsureWorkers()` (`sync.Once`).
- Every enqueue runs `chatJobs.start()` before reading `chatDraining.Load()`, calls `chatJobs.finish()` on shed/completion, and derives every Redis/Telegram/provider timeout from `chatContext()`.
- Webhook worst-case shutdown remains **70 s** (`stop_grace_period: 75s` unchanged); zero edits to `main.go`.

### 3. Non-Tripping Error Shedding & Featherless Vision Degradation
**Sources:** `xenobot/modules/chat_run.go:114-126, 268-273`, `xenobot/modules/presence_react_api.go:212-218`, `xenobot/utils/featherless/errors.go:24-78`
**Apply to:** `chat_react.go`
- Read `chatBreaker(provider).isPaused(chatID, raidNow())` before calling the provider, so a paused/broken provider is skipped cleanly.
- **NEVER** call `chatRecordFailure` or `breaker.countFailure` / `breaker.pauseProvider` from background AI reactions — shed JSON parse errors, invalid emojis, `ErrNoReply`, Featherless `no choices`, vision `400`/`422`, transport/TLS hiccups, and Telegram `400`s silently as `NONE` with class-only `Debug` logs.
- On Featherless `*featherless.HTTPError` with status `400` or `422` (and `!KeyRejected()`) when images were sent, cache per-model `text_only` in Redis (e.g. `xenobot:cache:featherless_text_only:<model>`, 24h TTL) and retry once without images.

### 4. Operational vs Cache Redis Namespace Discipline
**Sources:** `xenobot/modules/presence.go:21-60`, `xenobot/modules/presence_react_api.go:77-80`, `xenobot/modules/chat_run.go:229-250`
**Apply to:** `presence.go`, `chat_react.go`, `presence_react_api.go`
- Operational state stays strictly outside `xenobot:cache:`:
  - `xenobot:presence:{<chat>}:ai:gap` (message-gap counter)
  - `xenobot:presence:{<chat>}:ai:cd:chat` (per-chat AI reaction cooldown)
  - `xenobot:presence:{<chat>}:cd:u:<user>` (shared per-user cooldown — honors 24h `👎` retraction cooldown!)
  - `xenobot:presence:{<chat>}:bot_react:<msg>` (24h retraction metadata, source `"ai"`)
  - `xenobot:chat:react:b:{u,c,g}:...` (via `chatReserveWith(..., "react:b", ...)`)
- Ephemeral lookup caches stay inside `xenobot:cache:` (safe to wipe on `CLEAR_CACHE_ON_STARTUP`):
  - `avail_react:<chat>` (20m TTL, already in `presence_react_api.go`)
  - `chat_presence:<chat>` (`skipLocal`, already in `xenobot/db/presence/repository.go`)
  - Featherless model `text_only` vision capability cache (24h TTL).

### 5. Lightweight Deterministic Testing (`AGENTS.md` CI Budget)
**Sources:** `xenobot/utils/chatprompt/leaf_test.go`, `xenobot/modules/presence_react_api_test.go:24-62`, `xenobot/modules/chat_watch_test.go:335-346`
**Apply to:** All Phase 32 test files
- Put all prompt-formatting, `U+FE0F`, lookalike, restricted-palette, and JSON-parser edge cases into pure millisecond tests in `xenobot/utils/chatprompt/reaction_test.go`.
- In `xenobot/modules`, swap seams via `chatSwap(t, ...)`, never use `t.Parallel()` on tests touching package globals, use `restoreChatPool(t)` after `StopChat()`, and avoid Cartesian DB/locale loops.

---

## No Analog Found

None — every Phase 32 component has a direct, git-tracked analog in Phase 20 (`chat_*`), Phase 31 (`presence*`, `settings_presence*`), or `xenobot/utils/chatprompt/` / `xenobot/utils/featherless/`.

*(Note on `chat_budget.go` / `chat_breaker.go`: these do not exist as standalone files; hourly budgets [`chatReserveWith`] and provider breakers [`chatBreakers`, `chatBreaker`, `chatRecordFailure`] live directly inside `xenobot/modules/chat_run.go`.)*

---

## Metadata

**Analog search scope:** `migrations/`, `xenobot/db/models/`, `xenobot/db/presence/`, `xenobot/db/chatconv/`, `xenobot/modules/`, `xenobot/utils/chatprompt/`, `xenobot/utils/featherless/`, `xenobot/i18n/`, `locales/`, `.planning/spikes/001..003/`
**Files scanned:** 26
**Tracked-source gate:** Verified via `git ls-files` for every analog file
**Pattern extraction date:** 2026-11-27
