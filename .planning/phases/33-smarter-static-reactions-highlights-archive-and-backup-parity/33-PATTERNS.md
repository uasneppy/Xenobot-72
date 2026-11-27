# Phase 33: Smarter Static Reactions, Highlights Archive and Backup Parity - Pattern Map

**Mapped:** 2026-11-27
**Requirements:** `COMM-01`, `COMM-02`, `COMM-03`, `COMM-04`, `COMM-05`
**Files analyzed:** 24 new/modified files (plus locale files, docs, and test updates)
**Analogs found:** 24 / 24 (all git-tracked sources verified via `git ls-files`)

---

## File Classification

| New/Modified File | Req | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|-----|------|-----------|----------------|---------------|
| `migrations/20261129120000_add_static_reaction_options.sql` | COMM-01 | migration | schema-DDL | `migrations/20261128120000_add_chat_presence_ai_react_mode.sql` | exact |
| `migrations/20261129130000_add_presence_stickers_and_archive.sql` | COMM-02,03,04 | migration | schema-DDL | `migrations/20261127120000_add_chat_presence.sql` + `20261128120000_add_chat_presence_ai_react_mode.sql` | exact |
| `xenobot/db/models/reactions.go` | COMM-01,05 | model | CRUD | `xenobot/db/models/reactions.go` + `xenobot/db/models/chat_presence.go` | exact |
| `xenobot/db/models/chat_presence.go` | COMM-02,03,05 | model | CRUD | `xenobot/db/models/chat_presence.go` | exact |
| `xenobot/db/models/chat_presence_archived.go` *(new)* | COMM-04 | model | CRUD (append-only dedup) | `xenobot/db/models/greeted_member.go` + `xenobot/db/models/chat_presence.go` | exact |
| `xenobot/db/reactions/repository.go` | COMM-01 | service/repo | CRUD + cache | `xenobot/db/reactions/repository.go` + `xenobot/db/presence/repository.go` | exact |
| `xenobot/db/presence/repository.go` | COMM-02,03,04 | service/repo | CRUD + skipLocal cache | `xenobot/db/presence/repository.go` | exact |
| `xenobot/db/cache/local.go` | COMM-01 | config | cache-policy | `xenobot/db/cache/local.go` | exact |
| `xenobot/modules/reactions.go` | COMM-01,02 | watcher + commands | event-driven + request-response | `xenobot/modules/reactions.go` + `xenobot/modules/presence.go` + `xenobot/modules/presence_react_api.go` | exact |
| `xenobot/modules/settings_reactions.go` | COMM-01 | UI controller | request-response | `xenobot/modules/settings_reactions.go` + `xenobot/modules/settings_advthresholds.go` | exact |
| `xenobot/modules/presence.go` | COMM-02,03,04 | module + apply + keys | request-response + event-driven | `xenobot/modules/presence.go` | exact |
| `xenobot/modules/presence_reactions.go` | COMM-04 | event router | event-driven (Group 0 `NewReaction`) | `xenobot/modules/presence_reactions.go` | exact |
| `xenobot/modules/presence_archive.go` *(new)* | COMM-03,04 | service + command + watcher | event-driven + 2-step proof | `xenobot/modules/logchannels.go` + `xenobot/modules/presence_reactions.go` + `xenobot/utils/chat_status/chat_status.go` | exact |
| `xenobot/modules/settings_presence.go` | COMM-02,03 | UI controller + sub-page `ph` | request-response | `xenobot/modules/settings_presence.go` + `xenobot/modules/settings_advthresholds.go` | exact |
| `xenobot/db/backup/types.go` | COMM-05 | model/DTO | transform | `xenobot/db/backup/types.go` | exact |
| `xenobot/db/backup/backup.go` | COMM-05 | service/repo | batch/transactional | `xenobot/db/backup/backup.go` (`importReactions`, `importLogChannels`, `exportMessagesData`) | exact |
| `xenobot/modules/backup.go` | COMM-05 | formatter/command | request-response | `xenobot/modules/backup.go` (`formatBackupImportSuccess`) | exact |
| `xenobot/modules/settings_backup.go` | COMM-05 | UI controller | request-response | `xenobot/modules/settings_backup.go` | exact |
| `xenobot/modules/copy_settings.go` | COMM-05 | **UNTOUCHED** | — | `xenobot/modules/copy_settings.go` (must remain untouched) | exact |
| `locales/{en,uk,ru,es,fr,hi,id,pt}.yml` | COMM-01..05 | config/i18n | static | `locales/en.yml` (existing `settings_pr_*`, `backup_module_*`, `reactions_*`, `logs_*`) | exact |
| `xenobot/i18n/settings_locale_test.go` | COMM-01..05 | test | validation | `xenobot/i18n/settings_locale_test.go` | exact |
| `xenobot/db/migrations/chat_presence_archive_migration_test.go` *(new)* | COMM-01..04 | test | migration verify | `xenobot/db/migrations/chat_presence_ai_react_migration_test.go` | exact |
| `xenobot/db/backup/presence_backup_test.go` *(new)* | COMM-05 | test | DB roundtrip + security | `xenobot/db/backup/import_validation_test.go` + `chatconv_backup_test.go` + `event_messages_backup_test.go` | exact |
| `xenobot/modules/presence_archive_test.go` *(new)* | COMM-03,04 | test | dispatcher + miniredis + DB | `xenobot/modules/logchannels_flow_test.go` + `xenobot/modules/presence_reactions_test.go` | exact |

---

## Pattern Assignments

### 1. Database Migrations (`COMM-01`, `COMM-02`, `COMM-03`, `COMM-04`)

**Target files:**
- `migrations/20261129120000_add_static_reaction_options.sql`
- `migrations/20261129130000_add_presence_stickers_and_archive.sql`

**Latest existing migration (must sort strictly after):** `migrations/20261128120000_add_chat_presence_ai_react_mode.sql` (lines 1-13):
```sql
ALTER TABLE chat_presence
    ADD COLUMN IF NOT EXISTS ai_react_mode VARCHAR(16) NOT NULL DEFAULT 'off'
    CONSTRAINT chk_chat_presence_ai_react_mode CHECK (ai_react_mode IN ('off', 'low', 'medium'));
```

**Table creation analog:** `migrations/20261127120000_add_chat_presence.sql` (lines 12-20):
```sql
CREATE TABLE IF NOT EXISTS chat_presence (
    chat_id BIGINT PRIMARY KEY,
    consensus_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    consensus_threshold SMALLINT NOT NULL DEFAULT 3,
    retract_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    updated_by BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT chk_chat_presence_consensus_threshold CHECK (consensus_threshold BETWEEN 2 AND 20)
);
```

**Rules from `AGENTS.md` (lines 3655-3666) & `chat_presence_ai_react_migration_test.go` (lines 43-64):**
- Append-only; filename timestamp strictly greater than `20261128120000`.
- Single transaction: NO `BEGIN`, `COMMIT`, `ROLLBACK`, `CONCURRENTLY`, `NOT VALID`, or foreign keys (`REFERENCES`).
- Named `CHECK` constraints MUST be inline on `ADD COLUMN IF NOT EXISTS` (never top-level `ALTER TABLE ... ADD CONSTRAINT`, because both migration appliers swallow `duplicate_object` only on inline or standalone additions consistently when `ADD COLUMN IF NOT EXISTS` skips the clause on re-run).
- `reactions` new columns (`COMM-01`): `word_boundary BOOLEAN NOT NULL DEFAULT FALSE`, `match_caption BOOLEAN NOT NULL DEFAULT FALSE`, `chance_pct SMALLINT NOT NULL DEFAULT 100 CONSTRAINT chk_reactions_chance_pct CHECK (chance_pct BETWEEN 1 AND 100)`, `cooldown_secs INTEGER NOT NULL DEFAULT 0 CONSTRAINT chk_reactions_cooldown_secs CHECK (cooldown_secs BETWEEN 0 AND 86400)`.
- `chat_presence` new columns (`COMM-02`, `COMM-03`):
  - `cat_sticker_enabled BOOLEAN NOT NULL DEFAULT FALSE`
  - `archive_enabled BOOLEAN NOT NULL DEFAULT FALSE`
  - `archive_chat_id BIGINT NOT NULL DEFAULT 0`
  - `archive_thread_id BIGINT NOT NULL DEFAULT 0`
  - `archive_threshold SMALLINT NOT NULL DEFAULT 5 CONSTRAINT chk_chat_presence_archive_threshold CHECK (archive_threshold BETWEEN 2 AND 50)`
  - `archive_positive_only BOOLEAN NOT NULL DEFAULT TRUE` (or string enum `'positive'` / `'all'` — check exact CHECK constraint; if boolean, `NOT NULL DEFAULT TRUE` and NO `default:` GORM tag!)
  - `archive_delivery VARCHAR(16) NOT NULL DEFAULT 'copy' CONSTRAINT chk_chat_presence_archive_delivery CHECK (archive_delivery IN ('copy', 'forward'))`
- Durable dedup table (`COMM-04`):
  ```sql
  CREATE TABLE IF NOT EXISTS chat_presence_archived (
      chat_id BIGINT NOT NULL,
      message_id BIGINT NOT NULL,
      archived_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
      CONSTRAINT pk_chat_presence_archived PRIMARY KEY (chat_id, message_id)
  );
  ```

---

### 2. GORM Models (`xenobot/db/models/`)

#### `xenobot/db/models/reactions.go` (model, CRUD)
**Current file** (lines 5-16):
```go
type Reactions struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatID    int64     `gorm:"column:chat_id;not null;uniqueIndex:idx_reactions_chat_keyword" json:"chat_id,omitempty"`
	Keyword   string    `gorm:"column:keyword;not null;uniqueIndex:idx_reactions_chat_keyword" json:"keyword,omitempty"`
	Emoji     string    `gorm:"column:emoji;not null" json:"emoji,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}
```
**Analog for booleans + checks:** `xenobot/db/models/chat_presence.go` (lines 9-19):
- **CRITICAL:** NO GORM `default:` tag on any `bool` (`WordBoundary`, `MatchCaption`) so Go `false` is always written explicitly on `Create`/`Save`!
- Add `omitempty` on new JSON tags (`word_boundary,omitempty`, `match_caption,omitempty`, `chance_pct,omitempty`, `cooldown_secs,omitempty`) so old v1.0-v1.2 backup JSON without these keys decodes to zero values (`ChancePct == 0` normalized to `100` on read/import!).

#### `xenobot/db/models/chat_presence.go` (model, CRUD)
**Analog:** self (`xenobot/db/models/chat_presence.go`, lines 11-24):
- Follow the exact same pattern: no `default:` tag on `CatStickerEnabled`, `ArchiveEnabled`, `ArchivePositiveOnly` (if bool).
- Inline `check:` GORM tags matching SQL `CHECK` names so SQLite tests enforce the same constraints as PostgreSQL (`escalation.go` / `chat_presence.go` pattern).

#### `xenobot/db/models/chat_presence_archived.go` (model, append-only dedup)
**Analog:** `xenobot/db/models/chat_presence.go` + `xenobot/db/models/greeted_member.go`:
- Composite primary key: `ChatID int64` (`gorm:"column:chat_id;primaryKey;autoIncrement:false"`), `MessageID int64` (`gorm:"column:message_id;primaryKey;autoIncrement:false"`), `ArchivedAt time.Time` (`gorm:"column:archived_at"`).
- Table name: `"chat_presence_archived"`.

---

### 3. Repositories & Caching (`xenobot/db/reactions/`, `xenobot/db/presence/`, `xenobot/db/cache/local.go`)

#### `xenobot/db/reactions/repository.go` (service/repo, CRUD + cache)
**Existing signatures inspected** (`xenobot/db/reactions/repository.go`, lines 16-164):
- `reactionsCacheKey(chatID int64) string` -> `cache.CacheKey("reactions", chatID)`
- `GetReactions(chatID int64) map[string]string` / `GetReactionsContext(ctx context.Context, chatID int64) map[string]string`
- `AddReaction(chatID int64, keyword, emoji string) error`
- `RemoveReaction(chatID int64, keyword string) error`
- `ResetReactions(chatID int64) error`
- `AddReactionContext(ctx context.Context, chatID int64, keyword, emoji string, overwrite bool) (added bool, err error)`
- `ListReactionEntriesFresh(ctx context.Context, chatID int64) ([]models.Reactions, error)` (line 115: currently `.Select("id", "keyword", "emoji")` — MUST widen `.Select(...)` to include the new columns!)
- `GetReactionEntryFresh(ctx context.Context, chatID, id int64) (*models.Reactions, error)`
- `CountReactionsFresh(ctx context.Context, chatID int64) (int64, error)`
- `ReactionExistsFresh(ctx context.Context, chatID int64, keyword string) (bool, error)`
- `RemoveReactionByID(ctx context.Context, chatID, id int64) (*models.Reactions, error)`

**Analog for typed cached rule struct + column-targeted upserts:** `xenobot/db/presence/repository.go` (lines 40-176):
```go
type Rule struct {
	ID           uint   `msgpack:"id"`
	Keyword      string `msgpack:"k"`
	Emoji        string `msgpack:"e"`
	WordBoundary bool   `msgpack:"wb"`
	MatchCaption bool   `msgpack:"mc"`
	ChancePct    int    `msgpack:"ch"`
	CooldownSecs int    `msgpack:"cd"`
}
```
- **Rolling-deploy cache safety trap:** Existing `GetReactionsContext` stores `map[string]string` under `reactions:<chatID>`. Keep `GetReactionsContext` intact (or store rich `[]Rule` under a distinct cache key e.g. `cache.CacheKey("reaction_rules", chatID)`), and drop **both** `reactionsCacheKey(chatID)` and `reactionRulesCacheKey(chatID)` on every write (`AddReaction`, `AddReactionContext`, `RemoveReaction`, `RemoveReactionByID`, `ResetReactions`, and new option writers `SetReactionOptionsByID` / `AddReactionRuleContext`).
- **Normalize legacy `ChancePct == 0 -> 100`** in `normalizeRule` on every read from DB/cache so pre-Phase-33 rows or zeroed entries always fire at 100%.
- **Skip-local cache:** Add `reactionRulesPrefix = CacheKey("reaction_rules") + ":"` (and/or `reactionsPrefix`) to `skipLocal` in `xenobot/db/cache/local.go` (lines 62-135) if per-rule option edits in private settings must take effect immediately across replicas.

#### `xenobot/db/presence/repository.go` (service/repo, CRUD + skipLocal)
**Existing signatures inspected** (`xenobot/db/presence/repository.go`, lines 39-176):
- `Settings` struct (lines 40-45) with compact `msgpack` tags:
  ```go
  type Settings struct {
  	ConsensusEnabled   bool   `msgpack:"ce"`
  	ConsensusThreshold int    `msgpack:"ct"`
  	RetractEnabled     bool   `msgpack:"re"`
  	AIReactMode        string `msgpack:"ar"`
  	// Add Phase 33 fields with new msgpack tags (rolling-deploy safe: missing tags decode as zero/defaults):
  	// CatStickerEnabled bool `msgpack:"cs"`
  	// ArchiveEnabled bool `msgpack:"ae"`
  	// ArchiveChatID int64 `msgpack:"ac"`
  	// ArchiveThreadID int64 `msgpack:"at"`
  	// ArchiveThreshold int `msgpack:"ah"`
  	// ArchivePositiveOnly bool `msgpack:"ap"` (careful: if default is true, store ArchiveEmojiMode string `msgpack:"am"` ('positive'/'all') or normalize "" -> 'positive' so old cached entries without the field default to 'positive'!)
  	// ArchiveDelivery string `msgpack:"ad"` ('copy'/'forward', normalize "" -> 'copy'!)
  }
  ```
  - **CRITICAL rolling-deploy detail:** Just like `normalizeAIReactMode` (lines 68-73) maps `"" -> "off"` for pre-Phase-32 cached `Settings`, using string enum fields `ArchiveEmojiMode` (`"" -> "positive"`) and `ArchiveDelivery` (`"" -> "copy"`), and clamping `ArchiveThreshold == 0 -> DefaultArchiveThreshold (5)`, ensures old Phase 32 Redis cache entries lacking Phase 33 fields decode safely to the exact Phase 33 defaults without needing a cache flush!
- Column-targeted upserts (lines 114-176):
  - Copy `upsert(ctx, row, column)` pattern (`clause.OnConflict{Columns: []clause.Column{{Name: "chat_id"}}, DoUpdates: clause.AssignmentColumns([]string{column, "updated_by", "updated_at"})}` + `cache.DeleteCache(CacheKey(row.ChatID))`).
  - Add `SetArchiveTarget(ctx context.Context, chatID, targetChatID, targetThreadID, by int64) error` (multi-column upsert updating `archive_chat_id`, `archive_thread_id`, `archive_enabled` = `true` or preserving, `updated_by`, `updated_at` and dropping `CacheKey(chatID)`).
  - Add `ClearArchiveTarget(ctx context.Context, chatID, by int64) error` (sets `archive_chat_id = 0`, `archive_thread_id = 0`, `archive_enabled = false`, drops cache).
  - Add durable Highlights dedup methods (or `TryClaimArchived(ctx context.Context, chatID, msgID int64) (bool, error)` using `clause.OnConflict{Columns: []clause.Column{{Name: "chat_id"}, {Name: "message_id"}}, DoNothing: true}` returning `result.RowsAffected == 1` — identical to `greeted_member` claim pattern!).
  - Add `ReplacePortableTx(tx *gorm.DB, chatID int64, s Settings, by int64) error` and `ClearTx(tx *gorm.DB, chatID int64) error` for `xenobot/db/backup/backup.go` (analog: `advthresholds.ReplaceAllTx` / `eventmsg.ReplaceAllValidatedTx`), remembering **never** to drop cache inside the uncommitted `tx` — return `CacheKey(chatID)` from the backup importer so `withBackupTx` invalidates after commit!

---

### 4. Smarter Static Reactions & Cat-Sticker Micro-Triggers (`COMM-01`, `COMM-02`)

#### `xenobot/modules/reactions.go` (watcher + commands, event-driven + request-response)
**Existing code inspected** (`xenobot/modules/reactions.go`, lines 27-522):
- `reactionsModule = moduleStruct{moduleName: "Reactions", handlerGroup: 8}`
- `supportedReactionEmoji` (lines 32-40): 73 standard Telegram emojis.
- `normaliseReactionEmoji(raw string) string` (lines 434-436): `strings.ReplaceAll(strings.TrimSpace(raw), "\ufe0f", "")`.
- `checkReactions(b *gotgbot.Bot, ctx *ext.Context) error` (lines 321-372): currently checks `msg.Text == ""`, reads `reactions.GetReactionsContext`, uses `strings.Contains(lowerText, keyword)`, and calls `b.SetMessageReaction` directly without `presenceReactionPacer`, `AvailableReactions`, `bot_react`, Staff/Lockdown guards, captions, chance, cooldowns, or `ctx.Data[presenceStaticReactedKey]`.

**Analogs to combine in `reactions.go`:**
1. **Shared safety gate (from `xenobot/modules/presence.go`, lines 99-134):**
   - Run `userID, ok := presenceMessageAllowed(b, ctx)` FIRST (rejects private chats, channels, slash commands, bots, `sender_chat`/anon channels, and Staff Groups via `skipLocal` `staff.GetStaffGroup`).
   - Run `presenceIsLockedDown(chat.Id)` **lazily** ONLY after a keyword rule or cat-sticker matches and passes cooldown/chance rolls (Anti-Pattern 2 from `ARCHITECTURE.md` / `presence.go:83-85`: never hit PostgreSQL on every ordinary message!).
2. **Pure Unicode word-boundary matcher (NO `regexp` `\b`!):**
   - Analog for Unicode rune classification: `xenobot/utils/chatprompt/chatprompt.go` lines 262-264:
     ```go
     r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
     ```
   - Implement `matchStaticReactionKeyword(text, keyword string, wordBoundary bool) bool`:
     - Lowercase both `text` and `keyword` (`strings.ToLower`).
     - If `!wordBoundary`, `strings.Contains(lowerText, lowerKeyword)`.
     - If `wordBoundary`, scan all occurrences of `lowerKeyword` in `lowerText`: decode rune immediately before match start (`utf8.DecodeLastRuneInString(lowerText[:idx])`) and immediately after match end (`utf8.DecodeRuneInString(lowerText[idx+len(lowerKeyword):])`); match succeeds iff neither boundary rune satisfies `unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'`. Works identically for Ukrainian (`і`, `ї`, `є`, `ґ`), Russian, English, and multi-word phrases.
3. **Caption matching (`COMM-01`):**
   - Read text via `text := msg.Text; if text == "" { text = msg.Caption }` (analog: `chatMessageText(msg)` in `chat_watch.go` / `chat_react.go:269`).
4. **Trigger chance % & per-rule / per-chat cooldowns (`COMM-01`):**
   - Chance roll: analog `chatReactRoll(chancePct)` in `xenobot/modules/chat_react.go` lines 84-86 (`chancePct >= 100 || (chancePct > 0 && rand.IntN(100) < chancePct)`).
   - Operational Redis cooldowns (strictly under `xenobot:presence:{<chat>}:*` outside `xenobot:cache:`, analog `xenobot/modules/presence.go:37-77` + `chat_react.go:128-158`):
     - Per-chat static cooldown key: `presenceChatTag(chatID) + "static:cd"` (e.g. short anti-spam burst gate, or per-rule key `presenceChatTag(chatID) + "static:r:" + strconv.FormatUint(uint64(ruleID), 10)`).
     - Use Redis `SET ... NX EX <secs>` (or Lua) only when a rule matches + passes chance roll + passes lazy `presenceIsLockedDown`.
5. **Cat-sticker micro-triggers (`COMM-02`):**
   - Trigger in `checkReactions` (group `8`) when NO static keyword rule reacted AND `msg.Sticker != nil`:
     - Read `pres, err := presenceSettingsGet(ctx, chat.Id)` (`skipLocal` Redis read). Check `pres.CatStickerEnabled`.
     - Check `msg.Sticker.Emoji` against cat glyphs (e.g. `🐱`, `😸`, `😹`, `😻`, `😼`, `😽`, `🙀`, `😿`, `😾`, `🐈`, `🐈‍⬛`).
     - Gate behind a probability roll + Redis cooldown (`presenceChatTag(chatID) + "cat:cd"`, outside `xenobot:cache:`) + lazy `presenceIsLockedDown(chat.Id)`.
     - Intersect `[]string{"👀", "❤"}` with `presenceAllowedPalette(ctx, b, chat.Id)` (reusing `presence_react_api.go:93`), pick one allowed emoji, and react via `presenceSetReaction(..., userID, emoji, "cat")` (zero AI calls!).
6. **Reaction API & Group 13 `ctx.Data` handoff (`COMM-01`, `COMM-02`):**
   - Replace raw `b.SetMessageReaction` in `reactions.go:356` with `presenceSetReaction(runCtx, b, chat.Id, msg.MessageId, userID, emoji, "static")` (`presence_react_api.go:208-241`), which automatically:
     - Normalizes `U+FE0F` and intersects with cached `AvailableReactions` (`presenceAllowedEmoji`),
     - Paces via `presenceReactionPacer` with `IsBig: false`,
     - Records 24h `bot_react` metadata so `👎` retraction (`REACT-05`) works on static and cat-sticker reactions too!
   - When `presenceSetReaction` returns `true` (for either a static rule or cat sticker), set:
     ```go
     if ctx.Data == nil {
         ctx.Data = make(map[string]any)
     }
     ctx.Data[presenceStaticReactedKey] = true
     ```
     (`presenceStaticReactedKey` is already defined in `xenobot/modules/presence.go:29` and already respected by `chatReactWatch` in `xenobot/modules/chat_react.go:258-262`!)

#### `xenobot/modules/settings_reactions.go` (UI controller, request-response)
**Existing code inspected** (`xenobot/modules/settings_reactions.go`, lines 23-168):
- Registered via `registerSettingsList(settingsReactionsDef())` (`p = "x"`, `Order: 73`, `Right: settingsRightChangeInfo`, `PagerCode: "xg"`, `AskCode: "xx"`, `YesCode: "xy"`, `AddCode: "xa"`, `CancelCode: "xc"`).
- **Important pattern check:** `settings_list.go` does NOT natively bind item-click actions for per-row detail screens, and `settings_phase19_test.go:119` checks exact `settingsReactionsDef()` actions.
- **Analogs for adding per-rule options UI on a `settingsListDef` page:**
  - `xenobot/modules/settings_blacklists.go` / `xenobot/modules/settings_fwdblock.go` use `settingsListDef.AddRowExtra` + `settingsListDef.Actions` (custom `settingsPageAction` entries on the list page) + `settingsListDef.Redraw`!
  - Or `settingsReactionsTyped` can accept optional flags (while preserving `"keyword emoji"` 2-token compatibility pinned by `settings_reactions_test.go:130-162`!) AND an edit/options sub-screen via `settingsListDef.Actions` (e.g., `xr` open rule options / `xo` toggle/set option on rule `r=<id>`)!
  - **CRITICAL test constraint:** `settings_reactions_test.go:97` asserts `settingsListRowWidths(keyboard) == []int{3, 1, 2}` on `x` when `AddRowExtra` is nil! If you add an `AddRowExtra` button or per-row edit buttons on `x`, update `settings_reactions_test.go` AND `settings_phase19_test.go:72-76, 119` (exact action code table for `x`) AND `settings_locale_test.go`!

---

### 5. Crowd-Picked Highlights Archive (`COMM-03`, `COMM-04`)

#### `xenobot/modules/presence_archive.go` (new file: `/setarchive`, `/unsetarchive`, reactor tracking, durable dedup, Forward/Copy + jump button)

**Pattern A — 2-Step Forward Proof (`/setarchive`, `/unsetarchive`)**
**Closest Analog:** `xenobot/modules/logchannels.go` (lines 37-170, 341-415):
- Pending marker key (lines 37-43):
  ```go
  func setarchivePendingKey(chatID, messageID int64) string {
      return fmt.Sprintf("xenobot:setarchive:%d:%d", chatID, messageID)
  }
  ```
  Reuse `storePendingMarker(key, value)` / `consumePendingMarker(key)` (`logchannels.go:48-93`, uses `cache.GetRedisClient().GetDel` outside `xenobot:cache:`, 1h TTL).
- Command registration (`logchannels.go:404-405`):
  ```go
  dispatcher.AddHandler(helpers.NewBotCommand("setarchive", presenceModule.setArchive).SetAllowChannel(true))
  dispatcher.AddHandler(helpers.NewBotCommand("unsetarchive", presenceModule.unsetArchive))
  ```
  **CRITICAL (`TestNoRawNewCommand`):** Never call `handlers.NewCommand`; always use `helpers.NewBotCommand` (or `helpers.WrapCommand`).
- Step 1 & Step 2 (`logchannels.go:114-142, 349-397`):
  - **In a channel (`chat.Type == "channel"`):** Refuse if `presenceIsStaffGroup(chat.Id)` (channels aren't staff groups, but keep guard consistent); store `setarchivePendingKey(chat.Id, msg.MessageId)` and reply asking admin to forward that post into the source group.
  - **In a supergroup/group (`setarchive`):**
    1. If `helpers.IsAutomaticForward(msg)` -> return `false` (`logchannels.go:357-359`).
    2. Check `presenceIsStaffGroup(chat.Id)` -> **refuse Staff Group as source**!
    3. If `msg.ForwardOrigin != nil`: extract origin `(originChatID, originMsgID, ok)` — support `"channel"` (like `channelForwardOrigin` at `logchannels.go:98-107`) AND if the destination is a supergroup/forum topic, note `channelForwardOrigin` only matches `origin.Chat.Type == "channel"`; for forwarding from another supergroup or same-group topic check `origin.Chat != nil && (origin.Chat.Type == "channel" || origin.Chat.Type == "supergroup")`, and **refuse if `presenceIsStaffGroup(originChatID)` (refuse Staff Group as destination!)**.
    4. Also support same-group forum topic binding if an admin runs `/setarchive` directly inside a topic (check `requireUser`, `RequireUserAdmin` / `CanUserChangeInfo`, `!presenceIsStaffGroup(chat.Id)`, `msg.IsTopicMessage`, `msg.MessageThreadId`, and live bot post permission).
    5. Verify live admin (`RequireUser` + `RequireUserAdmin`) BEFORE `consumePendingMarker` so a regular member forwarding the post never burns the token (`logchannels.go:364-374`).
    6. Also attach a group-11 (or group-8/11) forward capture watcher OR handle forward inside `/setarchive` command (notice `/setlog` uses both: `/setlog` command handles forwarded `/setlog` text, and `captureSetLogForward` at group 11 handles forwards without command text; reusing that exact pattern or handling in `presence_archive.go` at group 11 after `logchannels` ensures both work!).
  - **`/unsetarchive`:** Analog `unsetLog` (`logchannels.go:144-170`) — require group, require admin (`RequireUserAdmin` / `CanUserChangeInfo`), refuse Staff Group, call `applyPresenceClearArchiveTarget`.

**Pattern B — Distinct-Member Reactor Tracking & Durable Once-Only Dedup (`COMM-04`)**
**Closest Analog:** `xenobot/modules/presence_reactions.go` (lines 48-75, 199-304):
- Called from `presenceHandleReaction` in `xenobot/modules/presence_reactions.go` (lines 173-202) right after `presenceHandleConsensus`:
  - Skip private chats, nil `mru.User` (Highlights counts distinct real non-anonymous members `mru.User != nil`, excluding `mru.User.IsBot`), and Staff Groups (`presenceIsStaffGroup(mru.Chat.Id)`).
  - Read `settings.ArchiveEnabled && settings.ArchiveChatID != 0`.
  - **Exclude hostile emojis:** Reuse `presenceConsensusHostile` (`presence_reactions.go:48-59`).
  - **Emoji filter (`Positive only` vs `All standard`):** Filter `added` / `removed` against either `presencePositiveEmojis` (positive subset of `supportedReactionEmoji`) or all non-hostile `supportedReactionEmoji`.
  - **Exclude author self-reactions (`COMM-04`):** Resolve `authorID := presenceLogAuthorOf(ctx, client, chatID, msgID)` (or also check `bot_react` if cached!). If `authorID != 0 && mru.User.Id == authorID`, ignore self-reaction (do not `SADD`).
  - **Set-diff tracking (Pitfall 3 & Architecture Pattern 4):**
    - Track qualifying reactors in one per-message Redis SET `presenceChatTag(chatID) + "arch:u:" + strconv.FormatInt(msgID, 10)` (48h-72h TTL, outside `xenobot:cache:`).
    - When `len(qualifyingAdded) > 0` or `len(qualifyingRemoved) > 0`, compute whether the user currently holds *any* qualifying reaction on `msgID`: wait — `MessageReactionUpdated.NewReaction` gives the user's **complete** current reaction list on `msgID`! So checking `len(qualifyingFromList(mru.NewReaction)) > 0` tells us directly whether to `SADD` (`"add"`) or `SREM` (`"rem"`) `userID` via `presenceConsensusUpdateScript` (`presence_reactions.go:63-74`)! Zero Premium multi-emoji inflation, zero toggle farming!
  - **Lazy lockdown & durable PostgreSQL dedup (`COMM-04`):**
    1. When `count >= int64(threshold)`:
    2. Fast Redis latch check `EXISTS arch:done:<msgID>` (avoids hitting PostgreSQL on subsequent reactions after archiving!).
    3. Lazy `presenceIsLockedDown(chatID)` (fail-closed).
    4. Durable PostgreSQL claim: `presence.TryClaimArchived(ctx, chatID, msgID)` (`INSERT INTO chat_presence_archived (chat_id, message_id) VALUES (?, ?) ON CONFLICT (chat_id, message_id) DO NOTHING`, returning `RowsAffected == 1`). If `false`, set `arch:done:<msgID>` and return!
    5. Verify `!presenceIsStaffGroup(settings.ArchiveChatID)` (defense-in-depth).

**Pattern C — Delivery (`CopyMessage` + Jump Button Default, or `ForwardMessage`) (`COMM-03`, `COMM-04`)**
**Closest Analog for `t.me/c/` link:** `xenobot/utils/chat_status/chat_status.go` (lines 640-659):
```go
func GetMessageLinkFromMessageId(chat *gotgbot.Chat, messageId int64) (messageLink string)
```
- **Supergroup guard (Pitfall 8 / `33-CONTEXT.md`):** Only build `t.me/c/` jump button when `chat.Type == "supergroup"` (and `strings.HasPrefix(strconv.FormatInt(chat.Id, 10), "-100")`), or `chat.Username != ""` (`https://t.me/<user>/<msgID>`). Include `?thread=<threadID>` if topic message if desired, and skip the URL button on legacy `"group"` chats where `t.me/c/` links don't work!
- **`CopyMessage` (default):** Call `b.CopyMessageWithContext(ctx, settings.ArchiveChatID, chatID, msgID, &gotgbot.CopyMessageOpts{MessageThreadId: settings.ArchiveThreadID, ReplyMarkup: jumpMarkup})` via `presencePaced` (bounded timeout derived from `chatContext()`).
- **`ForwardMessage`:** Call `b.ForwardMessageWithContext(ctx, settings.ArchiveChatID, chatID, msgID, &gotgbot.ForwardMessageOpts{MessageThreadId: settings.ArchiveThreadID})` via `presencePaced`.
- Handle Telegram `400`/`403` (deleted message, `HasProtectedContent`, revoked target rights) silently at Debug without tripping `chatBreakers` (and never leave the Redis latch un-marked if Postgres already claimed once-only, or handle `CopyMessage` fallback cleanly).

---

### 6. Presence Settings Page `pr` & Highlights Sub-Page `ph` (`COMM-02`, `COMM-03`)

#### `xenobot/modules/settings_presence.go` (UI controller + sub-page `ph`, request-response)
**Existing code inspected** (`xenobot/modules/settings_presence.go`, lines 24-439):
- `settingsPagePresence = "pr"`, `Order: 91`, `Icon: "✨"`, `OwnerOnly: false`, `PrivateOnly: true`, `PrivateNoteKey: "settings_pr_group_note"`, `Hidden: settingsPresenceHidden`.
- Existing controls (`Controls` map): `c` (consensus), `ct` (threshold), `r` (retract), `ai` (AI react mode).
- **Sub-page pattern analog:** `xenobot/modules/settings_advthresholds.go` (lines 226-267, 396-420) and `xenobot/modules/settings_messages.go` (lines 36-50):
  - **CRITICAL:** Implement `ph` (Highlights Archive sub-page) as a **`settingsPageAction` on `settingsPagePresence`** (e.g., `settingsActPresenceHighlights = "ph"` in `settingsPagePresence.Actions`), **NOT** as a separate `registerSettingsPage(...)` entry!
  - Why: `settings_menu.go:575` dispatches `a=op&p=...` only for top-level Main grid pages, and `settings_phase30_test.go:58` asserts exact `len(settingsPages) == 31`. Using `a=ph` on `pr` (with Back button `a=op&p=pr` and Close button `a=cl`) adds zero cells to Main, keeps `settingsMainOwnerRows = 17` and `len(settingsPages) = 31`, and inherits `pr`'s `PrivateOnly` + `settingsPresenceStaffGate`!
  - Validate every `Actions` entry on `pr` with `private(...)` (`fields["c"] != ""`) and `Right: settingsRightChangeInfo` (read-only view action `ph` can use `Right: settingsRightAdmin`, write actions on `ph` use `Right: settingsRightChangeInfo`).
  - **Callback cap check:** All button `CallbackData` strings MUST be `<= 64` bytes with worst-case negative 64-bit `c=-1009999999999999` (`settings_presence_test.go:86-90`).
  - **Cat-sticker toggle on `pr`:** Add `settingsPresenceControlCatSticker = "cs"` (`tg` action, `settingsRightChangeInfo`) to `Controls`, adding 1 row on `pr` (`5 -> 6` rows total with Highlights button `6 -> 7` rows total, staying `<= 8` rows!).
  - **`settingsPresenceMainState`:** Update active counter (`settings_presence.go:225-240`) to also count `stored.CatStickerEnabled` and `stored.ArchiveEnabled && stored.ArchiveChatID != 0` so Main status accurately reflects active Presence features.

---

### 7. Backup & Restore Parity (`COMM-05`)

#### `xenobot/db/backup/types.go` (model/DTO, transform)
**Existing code inspected** (`xenobot/db/backup/types.go`, lines 101-232):
- Add `BackupModulePresence = "presence"` right after `BackupModuleMessages = "messages"` (line 121) and append it to `AllExportableModules()` (line 145) — bringing total modules from `20` to `21`.
- Add `PresenceBackup`:
  ```go
  type PresenceBackup struct {
      Settings *PresencePortableBackup `json:"settings,omitempty"`
  }
  ```
  (Or `SettingBackup[PresencePortableBackup]`, **explicitly omitting** `archive_chat_id` and `archive_thread_id` from the JSON struct so they are never even serialized on export — matching `GreetingsBackup` omitting `LastMsgId`!)
- `ReactionsBackup` (line 219): already `[]models.Reactions`, so new `omitempty` fields on `models.Reactions` (`word_boundary`, `match_caption`, `chance_pct`, `cooldown_secs`) travel automatically while old v1.0-v1.2 exports without them decode cleanly!

#### `xenobot/db/backup/backup.go` (service/repo, batch/transactional)
**Existing code inspected** (`xenobot/db/backup/backup.go`, lines 24-45, 89-92, 453-456, 520-607, 891-906, 1075-1102, 1228-1230, 1285-1317):
- **`moduleExporters` (line 24) & `moduleClearers` (line 1075) & `importListModule` (line 589):**
  - Register `BackupModulePresence` in `moduleExporters` -> `exporter(exportPresenceData)`, `importListModule` -> `importPresence(tx, chatID, data, report)`, and `moduleClearers` -> `clearPresence`.
- **`ImportReport` (lines 89-92):**
  ```go
  type ImportReport struct {
      Skipped               int
      LogChannelSkipped     bool
      ArchiveChannelSkipped bool
  }
  ```
- **Extending `importReactions` (lines 891-906):**
  - Normalize legacy `ChancePct == 0` to `100` (and clamp out-of-range `ChancePct < 0 || ChancePct > 100` to `100`, `CooldownSecs < 0 || CooldownSecs > 86400` to `0`), normalize `Emoji` with `U+FE0F` strip if needed (check existing `TestReactionsRoundTrip` / `roundtrip_test.go` expectations), and invalidate both `cacheKey("reactions", chatID)` and `cacheKey("reaction_rules", chatID)`.
- **`exportPresenceData` / `importPresence` / `clearPresence` (mirroring `importLogChannels` at lines 1289-1317 & `greetings.dropGreetingMessageIDs`):**
  - **Export:** Read `models.ChatPresence` for `chatID`; if none, return `&PresenceBackup{}` (nil `Settings`). Map to portable DTO, never copying `ArchiveChatID` or `ArchiveThreadID`.
  - **Import:**
    - If `data.Settings == nil`: leave target's `chat_presence` alone (or reset? Wait — compare `importLogChannels` lines 1294-1296 `if data.Settings == nil { return nil, nil }` vs `replaceChatSetting`; for `BackupModulePresence` with `Settings != nil`, preserve the target's existing `archive_chat_id` / `archive_thread_id` if already linked!).
    - **Target stripping & `ArchiveChannelSkipped`:** Never overwrite `archive_chat_id` or `archive_thread_id` from backup JSON (even if a tampered JSON includes them!). Read target's existing `chat_presence` row inside `tx`:
      - If `data.Settings.ArchiveEnabled` was `true` in the backup (or archive config is present) AND target has `current.ArchiveChatID == 0`, force `archive_enabled = false` and set `report.ArchiveChannelSkipped = true`!
      - If target already has `current.ArchiveChatID != 0`, keep target's `ArchiveChatID` & `ArchiveThreadID` and apply the imported `ArchiveEnabled`!
    - Clamp/sanitize all imported fields (`ConsensusThreshold` `2..20` else `3`, `AIReactMode` valid else `'off'`, `ArchiveThreshold` `2..50` else `5`, `ArchiveEmojiMode` valid else `'positive'`, `ArchiveDelivery` valid else `'copy'`).
    - Return `[]string{cacheKey("chat_presence", chatID)}` so `withBackupTx` drops `skipLocal` cache after commit.
  - **Clear (`clearPresence`):** Delete `chat_presence` row (or reset to defaults), unlink archive target, and return `[]string{cacheKey("chat_presence", chatID)}`.
- **Exclusion invariant:** `chat_conversations` stays permanently excluded (`TestChatConversationsStayOutOfBackup`).

#### `xenobot/modules/backup.go` & `xenobot/modules/settings_backup.go`
- `xenobot/modules/backup.go:853-871` (`formatBackupImportSuccess`): add `if report.ArchiveChannelSkipped` hint appending `backup_import_archive_skipped` right after `if report.LogChannelSkipped`!
- `xenobot/modules/settings_backup.go:47`: `settingsBackupAllMask` automatically expands from 20 bits (`0xfffff`) to 21 bits (`0x1fffff`), `settingsBackupMaskMaxHex` stays `6`. Update stale `"20"` comments at lines 21, 192, 534. Add `"backup_module_presence"` to all 8 `locales/*.yml`.
- `xenobot/modules/copy_settings.go`: **MUST STAY UNTOUCHED** (`applyCopySettings` remains strictly scoped to the 6 moderation/tuning areas across 9 tables).

---

## Shared Patterns

### 1. Shared Safety Gate (`PRES-02`)
**Source:** `xenobot/modules/presence.go` (lines 88-134)
**Apply to:** `reactions.go` (group `8`), `presence_reactions.go` (group `0`), `presence_archive.go` (`/setarchive`, `/unsetarchive`, archive delivery)
- Hot path (`reactions.go`): `userID, ok := presenceMessageAllowed(b, ctx)` first (zero PostgreSQL calls; skips private chats, channels, slash commands, bots, `sender_chat`, and `skipLocal` Staff Groups).
- Lazy lockdown (`presenceIsLockedDown(chatID)`): call **only** right before acting (after keyword/sticker/threshold match and cooldown roll), failing closed on DB errors.
- Archive destination: explicitly check `!presenceIsStaffGroup(targetChatID)` in `/setarchive` AND right before delivery.

### 2. Group `8` -> Group `13` Single-Reaction Handoff
**Source:** `xenobot/modules/presence.go:29`, `xenobot/modules/chat_react.go:258-262`
**Apply to:** `xenobot/modules/reactions.go` (both static keyword rules and cat-sticker micro-triggers)
- Whenever `presenceSetReaction(...)` returns `true` in group `8`, set `ctx.Data[presenceStaticReactedKey] = true` (initializing `ctx.Data` if `nil`). Group `13` (`chatReactWatch`) already checks this key and skips AI reaction evaluation.

### 3. Operational Redis vs Cache Prefix Boundary
**Source:** `xenobot/modules/presence.go:21-77`, `xenobot/modules/presence_react_api.go:19-24`, `xenobot/modules/logchannels.go:37-43`
**Apply to:** All Phase 33 Redis keys
- Operational keys (static cooldowns, cat-sticker cooldown, `/setarchive` pending marker `xenobot:setarchive:...`, Highlights reactor sets `xenobot:presence:{<chat>}:arch:u:<msg>`, Highlights latch `xenobot:presence:{<chat>}:arch:done:<msg>`) MUST live **outside** `xenobot:cache:` so `CLEAR_CACHE_ON_STARTUP` never wipes them.
- `chat_presence:<chat>` stays in `skipLocal` (`xenobot/db/cache/local.go:63`).

### 4. Zero New Shutdown Drains
**Source:** `xenobot/modules/presence_reactions.go:155-171`, `AGENTS.md`
**Apply to:** `reactions.go`, `presence_reactions.go`, `presence_archive.go`
- Keep `70 s` webhook worst-case shutdown and `stop_grace_period: 75s` unchanged; `handleReaction` already joins `StopChat` (`chatJobs` + `chatDraining` + `chatContext()`).

### 5. Locale Parity & Ceilings (`AGENTS.md`, `xenobot/i18n/settings_locale_test.go`)
**Source:** `xenobot/i18n/settings_locale_test.go:27-1878`, `locales/*.yml`
**Apply to:** All 8 locale files (`en`, `uk`, `ru`, `es`, `fr`, `hi`, `id`, `pt`)
- Every new `settings_*` key MUST be added to `settingsLocaleLimits` in `xenobot/i18n/settings_locale_test.go`.
- Every key used via `tr.GetString` / `settingsText` MUST exist in all 8 `locales/*.yml` (`make check-translations`).
- Half-width grid/choice buttons `<= 20` runes; full-width toggles `<= 40` runes; callback data `<= 64` bytes; screens `<= 3800` UTF-16 units.

---

## Existing Tests Requiring Updates (Exact Registries, Module Lists, Command Lists)

| Test / Check Location | Line(s) | Exact Assertion / Constraint | Required Update for Phase 33 |
|-----------------------|---------|------------------------------|------------------------------|
| `xenobot/db/backup/backup_test.go` | `198-204` | `TestModuleValidation`: `assert.Len(t, modules, 20)` | Update `20 -> 21`, assert `BackupModulePresence` present and `IsValidModule("presence")` true. |
| `xenobot/db/backup/backup_test.go` | `309-389` | `cleanupBackupChat(t, chatID)` | Delete `models.ChatPresence{}` + `models.ChatPresenceArchived{}` and invalidate `presence.CacheKey(chatID)`. |
| `xenobot/db/backup/roundtrip_test.go` | `20-380` | `TestAllModulesRoundTripEveryMeaningfulField` | Seed and assert round-trip for `BackupModulePresence` and new `models.Reactions` fields (`WordBoundary`, `MatchCaption`, `ChancePct`, `CooldownSecs`). |
| `xenobot/modules/settings_backup_test.go` | `1188-1205` | `ImportReport{Skipped: 7, LogChannelSkipped: true}` & `import_confirm_20` | Verify 21 modules render within UTF-16/100-button caps; add `ArchiveChannelSkipped: true` coverage. |
| `xenobot/modules/backup_test.go` | `1173` | `formatBackupImportSuccess` hint check | Add `ArchiveChannelSkipped: true` assertion. |
| `xenobot/modules/settings_presence_test.go` | `83-85` | `if got := len(screen.Keyboard.InlineKeyboard); got != 5` | Update expected row count on page `pr` (e.g. `5 -> 7` with Cat Stickers toggle + Highlights sub-page button). |
| `xenobot/modules/settings_phase30_test.go` | `53-63` | `len(allPages) != 31` / `len(settingsPages) == len(allPages)` | Keep `ph` as a `settingsPageAction` on `pr` (NOT a top-level `settingsPage`) so `31` stays untouched; add Phase 33 action code collision tests in `settings_phase33_test.go`. |
| `xenobot/modules/settings_phase19_test.go` | `72-76, 119-121` | `settingsPhase19Codes` exact action count on `settingsPageReactions` (`x`) | Update `settingsPhase19Codes` ONLY if new `Actions` are added to `settingsPageReactions` (`x`). |
| `xenobot/modules/settings_reactions_test.go` | `97` | `settingsListRowWidths(keyboard)` expects `[]int{3, 1, 2}` | Update ONLY if `AddRowExtra` or per-row buttons change `x`'s keyboard layout; keep 2-token `keyword emoji` tests green! |
| `xenobot/modules/reactions_legacy_request_test.go` | `96-160` | Golden wire requests for `/addreaction`, `/removereaction`, `/reactions`, `/resetreactions` | **DO NOT RE-RECORD** (`AGENTS.md:1985`); keep default `reactions` commands/output backward-compatible! |
| `xenobot/i18n/settings_locale_test.go` | `27+`, `1569-1575` | `settingsLocaleLimits` must cover 100% of `settings_*` keys | Add ceiling entry for every new `settings_pr_*`, `settings_ph_*`, `settings_btn_pr_*`, `settings_btn_ph_*`, `settings_rct_*` key. |
| `xenobot/utils/helpers/command_registry_test.go` | `125` | `TestNoRawNewCommand` | Ensure `/setarchive` and `/unsetarchive` use `helpers.NewBotCommand` (never `handlers.NewCommand`). |
| `Makefile` (`check-translations`, `check-docs`, `test-postgres-integrity`) | `74-84`, `121-145` | Missing locale keys, generated-docs drift, PG integrity test list | Run `make check-translations`, `make generate-docs` (`make check-docs`), and consider adding Phase 33 PG migration/backup tests to `test-postgres-integrity`. |

---

## Metadata

**Analog search scope:** `migrations/`, `xenobot/db/models/`, `xenobot/db/presence/`, `xenobot/db/reactions/`, `xenobot/db/backup/`, `xenobot/db/cache/`, `xenobot/db/migrations/`, `xenobot/modules/`, `xenobot/i18n/`, `xenobot/utils/chat_status/`, `xenobot/utils/chatprompt/`, `locales/`, `Makefile`
**Files scanned:** 26
**Pattern extraction date:** 2026-11-27
