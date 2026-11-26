# Phase 31: Presence Foundation, Settings Page and Zero-AI Social Reactions - Pattern Map

**Mapped:** 2026-11-26
**Files analyzed:** 26 (14 new, 12 modified)
**Analogs found:** 26 / 26

> **Repository path corrections (verified against git-tracked source):**
> - SQL migrations live at repo root `migrations/*.sql` (NOT `xenobot/db/migrations/` — `xenobot/db/migrations/` holds the Go runner and migration tests).
> - Locale YAML files live at repo root `locales/{en,es,fr,hi,id,pt,ru,uk}.yml` (NOT `xenobot/locales/`).
> - The local cache bypass lives in `xenobot/db/cache/local.go` (NOT `xenobot/cache/local.go`).
> - The conversations model is singular: `xenobot/db/models/chat_conversation.go` (NOT `chat_conversations.go`).
> - Settings Main/page-grid tests live in `xenobot/modules/settings_main_test.go` (there is no `settings_pages_test.go`); Staff Group Main line-count assertions also live in `xenobot/modules/settings_backup_test.go` and `xenobot/modules/settings_phase18_test.go`.

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `xenobot/config/config.go` *(MOD)* | config | startup-config | `xenobot/config/config.go` (lines 546-561) | exact |
| `migrations/20261127120000_add_chat_presence.sql` *(NEW)* | migration | schema-DDL | `migrations/20261119120000_add_chat_conversations.sql` | exact |
| `xenobot/db/migrations/chat_presence_migration_test.go` *(NEW)* | test | schema-validation | `xenobot/db/migrations/chat_conversation_migration_test.go` | exact |
| `xenobot/db/models/chat_presence.go` *(NEW)* | model | CRUD | `xenobot/db/models/chat_conversation.go` | exact |
| `xenobot/db/cache/local.go` *(MOD)* | middleware/cache | cache-bypass | `xenobot/db/cache/local.go` (lines 52-63, 113-126) | exact |
| `xenobot/db/cache/local_test.go` *(MOD)* | test | cache-bypass | `xenobot/db/cache/local_test.go` (lines 355-364) | exact |
| `xenobot/db/presence/repository.go` *(NEW)* | service/repository | CRUD + Redis cache | `xenobot/db/chatconv/repository.go` | exact |
| `xenobot/db/presence/testmain_test.go` *(NEW)* | test-harness | DB fixture | `xenobot/db/chatconv/testmain_test.go` | exact |
| `xenobot/db/presence/repository_test.go` *(NEW)* | test | CRUD + cache | `xenobot/db/chatconv/repository_test.go` | exact |
| `xenobot/modules/test_harness_test.go` *(MOD)* | test-harness | DB AutoMigrate | `xenobot/modules/test_harness_test.go` (lines 378-395) | exact |
| `xenobot/modules/presence.go` *(NEW)* | module/guard | event-driven | `xenobot/modules/chat.go` + `xenobot/modules/chat_watch.go` + `xenobot/modules/chat_settings.go` | role-match |
| `xenobot/modules/presence_react_api.go` *(NEW)* | service/adapter | request-response + Redis | `xenobot/modules/reactions.go` (lines 32-40, 433-450) + `xenobot/modules/chat_send.go` (lines 24-45, 167-177) + `xenobot/modules/chat_history.go` | role-match |
| `xenobot/modules/presence_reactions.go` *(NEW)* | controller/handler | event-driven + Redis Lua | `xenobot/modules/chat_log.go` (lines 57-94) + `xenobot/modules/chat_run.go` (lines 72-109, 172-210, 377-390) + `xenobot/modules/chat_brief.go` (lines 297-338) | role-match |
| `xenobot/modules/settings_presence.go` *(NEW)* | controller/UI | request-response (callback) | `xenobot/modules/settings_advthresholds.go` (lines 210-268) + `xenobot/modules/settings_deletion.go` (lines 113-125, 282-310) + `xenobot/modules/settings_chat.go` | exact |
| `xenobot/modules/settings_chat.go` *(MOD)* | controller/UI | request-response (callback) | `xenobot/modules/settings_chat.go` (lines 180-222) | exact |
| `xenobot/modules/settings_main_test.go` *(MOD)* | test | UI layout contract | `xenobot/modules/settings_main_test.go` (lines 35-42, 571-578) | exact |
| `xenobot/modules/settings_chat_test.go` *(MOD)* | test | UI layout + flow | `xenobot/modules/settings_chat_test.go` (lines 525-581) | exact |
| `xenobot/modules/settings_backup_test.go` *(MOD)* | test | Staff Group Main lines | `xenobot/modules/settings_backup_test.go` (lines 247-270) | exact |
| `xenobot/modules/settings_phase18_test.go` *(MOD)* | test | Staff Group Main lines | `xenobot/modules/settings_phase18_test.go` (lines 90-96) | exact |
| `xenobot/modules/settings_presence_test.go` *(NEW)* | test | UI + Staff/Group/Role gates | `xenobot/modules/settings_chat_test.go` + `xenobot/modules/settings_backup_test.go` | exact |
| `xenobot/modules/presence_test.go` *(NEW)* | test | safety gate + StopChat drain | `xenobot/modules/chat_env_test.go` | exact |
| `xenobot/modules/presence_react_api_test.go` *(NEW)* | test | httptest wire JSON + cache | `xenobot/modules/chat_env_test.go` + `xenobot/modules/reactions.go` | role-match |
| `xenobot/modules/presence_reactions_test.go` *(NEW)* | test | Lua set-diff + Consensus + `👎` | `xenobot/modules/chat_env_test.go` | exact |
| `locales/{en,es,fr,hi,id,pt,ru,uk}.yml` *(MOD)* | config/i18n | static locale map | `locales/en.yml` (lines 2372-2395) + `docs/src/content/docs/contributing/locale-style.md` | exact |
| `xenobot/i18n/settings_locale_test.go` *(MOD)* | test | i18n ceilings + grid labels | `xenobot/i18n/settings_locale_test.go` (lines 909-931, 1125-1170) | exact |
| `AGENTS.md` *(MOD)* | docs/contract | repo invariants | `AGENTS.md` (Startup/shutdown, Handlers, Data sections) | exact |

---

## Pattern Assignments

### 1. `xenobot/config/config.go` *(MOD)* — Config (`PRES-01`)

**Analog:** `xenobot/config/config.go` (lines 546-561)

**Core pattern** (lines 546-561):
```go
	cfg.AllowedUpdates = []string{
		"message",
		"edited_message",
		"channel_post",
		"edited_channel_post",
		"inline_query",
		"chosen_inline_result",
		"callback_query",
		"shipping_query",
		"pre_checkout_query",
		"poll",
		"poll_answer",
		"my_chat_member",
		"chat_member",
		"chat_join_request",
		"message_reaction", // Phase 31 (PRES-01) — never add "message_reaction_count"
	}
```

- Both webhook (`xenobot/utils/httpserver/server.go:235`) and polling (`main.go:355`) pass `config.AppConfig.AllowedUpdates` verbatim.
- `"poll"` is already subscribed (`config.go:556`).
- Do **not** add `"message_reaction_count"` (channel-only, batched, no user IDs).

---

### 2. `migrations/20261127120000_add_chat_presence.sql` *(NEW)* — Migration (`PRES-03`)

**Analog:** `migrations/20261119120000_add_chat_conversations.sql` (lines 1-23)

**Core DDL pattern** (lines 14-23):
```sql
CREATE TABLE IF NOT EXISTS chat_conversations (
    chat_id BIGINT PRIMARY KEY,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    provider VARCHAR(16) NOT NULL DEFAULT 'anthropic',
    audience SMALLINT NOT NULL DEFAULT 0,
    updated_by BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT chk_chat_conversations_provider CHECK (provider IN ('anthropic', 'google')),
    CONSTRAINT chk_chat_conversations_audience CHECK (audience BETWEEN 0 AND 1)
);
```

**Mandatory SQL migration rules (`AGENTS.md` lines 3609-3621 + `chat_conversation_migration_test.go` lines 39-66):**
- Filename MUST sort strictly after `20261126120000_allow_featherless_chat_provider.sql` (e.g., `migrations/20261127120000_add_chat_presence.sql`). Never edit any previously applied file (SHA-256 checksum aborts startup).
- Single transaction (no `BEGIN`, `COMMIT`, `ROLLBACK`, `CONCURRENTLY`).
- One idempotent `CREATE TABLE IF NOT EXISTS chat_presence (...)` statement.
- All `CHECK` constraints MUST be **inline** inside `CREATE TABLE` with explicit `CONSTRAINT chk_chat_presence_<col> CHECK (...)` names (never bare top-level `ADD CONSTRAINT` or `NOT VALID` on a new table, because both appliers swallow `duplicate_object`).
- **No foreign key (`REFERENCES`)** on `chats` (matches `chat_conversations`, `chat_advanced_thresholds`, `staff_groups`).
- Boolean defaults live in SQL (`DEFAULT FALSE` / `DEFAULT TRUE` for `retract_enabled`), while GORM struct tags omit `default:`.

---

### 3. `xenobot/db/migrations/chat_presence_migration_test.go` *(NEW)* — Migration Test

**Analog:** `xenobot/db/migrations/chat_conversation_migration_test.go` (lines 1-134)

**Exact test structure to copy:**
1. Helper `chatPresenceStatements(t *testing.T) []string` reading `filepath.Join("..", "..", "..", "migrations", chatPresenceMigration)` and filtering through `(&MigrationRunner{}).splitSQLStatements(cleanSupabaseSQL(string(content)))`, skipping blank/`isTransactionControlStatement`, returning `stripSQLComments(stmt)`.
2. `TestChatPresenceMigrationStatements(t *testing.T)` (runs locally without DB):
   - Asserts `len(run) == 1` and `strings.HasPrefix(run[0], "CREATE TABLE IF NOT EXISTS chat_presence ")`.
   - Asserts every named inline `CONSTRAINT chk_chat_presence_*` exists.
   - Asserts banned tokens absent: `"CONCURRENTLY"`, `"NOT VALID"`, `"BEGIN"`, `"COMMIT"`, `"ROLLBACK"`, `"ADD CONSTRAINT"`, `"REFERENCES"`.
   - Asserts `chatPresenceMigration > "20261126120000_allow_featherless_chat_provider.sql"`.
3. `TestChatPresenceMigrationDefaults(t *testing.T)` (PostgreSQL-only; one of the ONLY tests allowed to call `t.Skip` when `XENOBOT_TEST_DATABASE != "true"` or `DATABASE_URL == ""`):
   - Opens Postgres, checks `database.Migrator().HasTable("chat_presence")`, runs inside a rolled-back `tx := database.Begin()`.
   - Re-executes every statement to prove idempotency.
   - Inserts `INSERT INTO chat_presence (chat_id) VALUES (?)` and verifies SQL defaults (notably `retract_enabled = true`, `consensus_enabled = false`, `consensus_threshold = 3`, etc.).
   - Uses `SAVEPOINT refused` / `ROLLBACK TO SAVEPOINT refused` to verify each `CHECK` constraint returns `SQLSTATE 23514` naming `chk_chat_presence_*`.

---

### 4. `xenobot/db/models/chat_presence.go` *(NEW)* — GORM Model (`PRES-03`)

**Analog:** `xenobot/db/models/chat_conversation.go` (lines 1-25)

**Exact struct & tag pattern** (lines 11-25):
```go
package models

import "time"

// ChatPresence is one chat's admin-configurable Interactive Presence settings.
// Booleans carry NO gorm default: tag so a Go false is always persisted explicitly on Create/Save.
type ChatPresence struct {
	ChatID             int64     `gorm:"column:chat_id;primaryKey;autoIncrement:false" json:"chat_id"`
	ConsensusEnabled   bool      `gorm:"column:consensus_enabled;not null" json:"consensus_enabled"`
	ConsensusThreshold int16     `gorm:"column:consensus_threshold;not null;check:chk_chat_presence_consensus_threshold,consensus_threshold BETWEEN 2 AND 20" json:"consensus_threshold"`
	RetractEnabled     bool      `gorm:"column:retract_enabled;not null" json:"retract_enabled"`
	// ... any Phase 32-35 columns if pre-allocated or added later via append-only migrations ...
	UpdatedBy          int64     `gorm:"column:updated_by;not null" json:"updated_by"`
	UpdatedAt          time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// TableName returns the migration's table name.
func (ChatPresence) TableName() string {
	return "chat_presence"
}
```

**Critical SQLite + PostgreSQL rules:**
- NEVER put `default:true` or `default:false` on GORM boolean tags (`gorm:"column:...;not null"` only), or GORM treats Go `false` as zero-value and substitutes the tag default on `Create`!
- `check:chk_chat_presence_<col>,<expr>` on GORM tags keeps SQLite `AutoMigrate` tests enforcing the exact same bounds as the PostgreSQL SQL migration.
- Add `&models.ChatPresence{}` to `AutoMigrate` in `xenobot/modules/test_harness_test.go` (lines 380-394) and `xenobot/db/presence/testmain_test.go` (via `testdb.Run(m, &models.ChatPresence{})`).

---

### 5. `xenobot/db/cache/local.go` & `xenobot/db/cache/local_test.go` *(MOD)* — Cache Bypass (`PRES-03`)

**Analog:** `xenobot/db/cache/local.go` (lines 52-63, 110-126) & `xenobot/db/cache/local_test.go` (lines 355-364)

**Exact prefix registration** (`local.go` lines 62-63, 113-126):
```go
	chatAdvancedThresholdsPrefix = CacheKey("chat_advanced_thresholds") + ":"
	chatPresencePrefix           = CacheKey("chat_presence") + ":"
```
```go
		strings.HasPrefix(key, chatReasonShortcutsPrefix) ||
		strings.HasPrefix(key, chatAdvancedThresholdsPrefix) ||
		strings.HasPrefix(key, chatPresencePrefix)
```
- The trailing `":"` is mandatory so a key like `"chat_presencex:1"` does NOT match.
- `CacheKey("chat_presence", chatID)` produces `xenobot:cache:chat_presence:<chatID>` (via `xenobot/db/cache/keys.go:11-30`).

**Exact unit test** (`local_test.go` lines 355-364):
```go
func TestSkipLocalChatPresenceKey(t *testing.T) {
	if !skipLocal(CacheKey("chat_presence", int64(-100123))) {
		t.Error("the presence key is served from the local layer")
	}
	if skipLocal(CacheKey("chat_presencex", int64(1))) {
		t.Error("a key that only starts with chat_presence is excluded from the local layer")
	}
}
```

---

### 6. `xenobot/db/presence/repository.go`, `testmain_test.go`, `repository_test.go` *(NEW)* — Repository (`PRES-03`)

**Analog:** `xenobot/db/chatconv/repository.go` (lines 1-173), `testmain_test.go` (lines 1-15), `repository_test.go` (lines 1-211)

**Imports pattern** (`chatconv/repository.go` lines 4-16):
```go
import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	xenoerrors "github.com/uasneppy/Xenobot-72/xenobot/utils/errors"
)
```

**Exact function signatures & conventions to mirror:**
- `type Settings struct` with short `msgpack` tags (e.g., `ce`, `ct`, `re`) — zero-value safe for rolling deploys; **note on `RetractEnabled` (default `true`)**: store `RetractEnabled bool` (or store `RetractDisabled` in msgpack / set `RetractEnabled: true` in `Defaults()`) and ensure `defaultRow` populates ALL non-zero defaults (`RetractEnabled: true`, `ConsensusThreshold: 3`) before every column-targeted upsert!
- `func Defaults() Settings`
- `func CacheKey(chatID int64) string { return cache.CacheKey("chat_presence", chatID) }`
- `func Get(ctx context.Context, chatID int64) (Settings, error)` — wraps `cache.GetFromCacheOrLoad(ctx, CacheKey(chatID), cacheTTL, func(ctx context.Context) (Settings, error) { return GetFresh(ctx, chatID) })`
- `func GetFresh(ctx context.Context, chatID int64) (Settings, error)` — returns `Defaults(), nil` on `errors.Is(err, gorm.ErrRecordNotFound)` without creating any row; wraps `errNoDatabase` if `db.DB == nil`.
- Column-targeted upsert writers (`SetConsensusEnabled`, `SetConsensusThreshold`, `SetRetractEnabled`, etc.) validating inputs with `ErrInvalidValue` before any DB write, then calling shared `upsert(ctx, row, column)`:
```go
func upsert(ctx context.Context, row *models.ChatPresence, column string) error {
	if db.DB == nil {
		return xenoerrors.Wrap(errNoDatabase, "write chat presence setting")
	}
	err := db.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.AssignmentColumns([]string{column, "updated_by", "updated_at"}),
	}).Create(row).Error
	if err != nil {
		return xenoerrors.Wrapf(err, "write the presence %s of chat %d", column, row.ChatID)
	}
	cache.DeleteCache(CacheKey(row.ChatID))
	return nil
}
```
- **Concurrency safety:** Column-targeted `OnConflict.DoUpdates` ensures two admins pressing two buttons concurrently never clobber each other's column, and `cache.DeleteCache(CacheKey(row.ChatID))` runs immediately after every write.
- **TestMain (`testmain_test.go`):** `//go:build testtools` + `os.Exit(testdb.Run(m, &models.ChatPresence{}))`.
- **Test coverage (`repository_test.go`):** `//go:build testtools`, `newChat(t)` with `-time.Now().UnixNano()` and `t.Cleanup` deleting DB row + calling `cache.DeleteCache`, asserting: (1) missing row returns `Defaults()` without inserting a row, (2) each column write updates only its column + drops cache + writes explicit Go `false` (crucial for `RetractEnabled: false`!), (3) invalid values return `ErrInvalidValue` and insert zero rows, (4) DB `CHECK` constraints reject out-of-range raw `db.DB.Create(...)`.

---

### 7. `xenobot/modules/presence.go` *(NEW)* — Module Registration, Shared Safety Gate & Apply Helpers (`PRES-01`, `PRES-02`)

**Analogs:** `xenobot/modules/chat.go` (lines 22, 54-77, 145-148), `xenobot/modules/chat_watch.go` (lines 51-82), `xenobot/modules/chat_settings.go` (lines 74-158)

**Module registration pattern** (`chat.go` lines 54-77, 145-148; `reactions.go` lines 374-376):
```go
var presenceModule = moduleStruct{moduleName: "Presence"}

func LoadPresence(dispatcher *ext.Dispatcher) {
	// Non-message update at group 0: gotgbot dispatches by update type, so it never contends
	// with message handlers and CommandDeletionProcessor returns immediately when ctx.Message == nil.
	dispatcher.AddHandler(handlers.NewReaction(reaction.All, presenceModule.handleReaction))
	log.Info("[Modules] Presence module loaded")
}

func init() {
	RegisterLegacyModule("Presence", 252, LoadPresence)
}
```
> **Note on `SetModuleEnabled`:** Do NOT call `SetModuleEnabled("Presence", true)` unless you also add `help_module_name_presence` and a help page in all 8 locales — `TestHelpModuleNamesExist` loads every module and fails on any enabled help module missing a `help_module_name_<module>` key (`AGENTS.md` line 6064). Phase 31 has no user-facing `/commands` yet, so registering `LoadPresence` without `SetModuleEnabled` (like `staff_watchers.go` / `admin_index_feed.go` / `member_deletion.go`) avoids adding an empty `/help` button.

**Shared safety gate pattern (`PRES-02`)** — combining verified existing signatures:
- Chat type check: `chat != nil && (chat.Type == "group" || chat.Type == "supergroup")` (skips private chats and channels).
- Staff Group lookup (cached `skipLocal` Redis): `staff.GetStaffGroup(chatID) != nil` (`xenobot/db/staff/repository.go:108`: `func GetStaffGroup(chatID int64) *models.StaffGroup`). Wrap in a package var `var presenceStaffLookup = staff.GetStaffGroup` (or `staff.GetStaffGroupFresh` for fail-closed reads) so tests can inject errors/seams!
- Active Lockdown lookup (direct Postgres — **MUST be called LAZILY** only after cheap local checks and Redis gates pass!): `lockdown.GetActiveFresh(chatID)` (`xenobot/db/lockdown/repository.go:76`: `func GetActiveFresh(chatID int64) (*models.ChatLockdown, error)`). Fail closed: if `err != nil || ld != nil`, skip! Wrap in `var presenceLockdownLookup = lockdown.GetActiveFresh`.
- Message-path sender & slash guards (reusable across Phases 31-35, modeled on `chat_watch.go:61-73`):
  - `raidFloodIsSlashText(msg)` — skips slash-prefixed commands.
  - `raidFloodSenderCounts(b, ctx)` — rejects `sender_chat` (anonymous channels/admins) and returns real `userID`; also check `ctx.EffectiveSender.User.IsBot`.
- Reaction-path actor guards (`MessageReactionUpdated`):
  - `mru := ctx.MessageReaction; if mru == nil` → skip.
  - `mru.User != nil && mru.User.IsBot` → skip completely.
  - Anonymous group admin helper:
    ```go
    func presenceIsAnonGroupAdmin(mru *gotgbot.MessageReactionUpdated) bool {
    	return mru != nil && mru.User == nil && mru.ActorChat != nil && mru.ActorChat.Id == mru.Chat.Id
    }
    ```
  - Anonymous/channel actors (`mru.User == nil`) are **excluded** from Consensus distinct-member counting and user-scoped flows, while `presenceIsAnonGroupAdmin(mru)` IS allowed to trigger `👎` retraction!

**Redis key namespace & client seam** (modeled on `chat_settings.go:74-111`):
```go
// presenceKeyPrefix sits outside xenobot:cache: so CLEAR_CACHE_ON_STARTUP never wipes cooldowns,
// 24h bot_react metadata, or distinct-reactor sets.
const presenceKeyPrefix = "xenobot:presence:"

var presenceRedisClient = func() *redis.Client {
	return chatRedisClient()
}
```
(Or share `chatRedisClient` directly so existing `withChatRedis(t)` test helper automatically swaps Redis for both `chat_*` and `presence_*` tests! Sharing `chatRedisClient` keeps test setup uniform and clean.)

**Apply helpers pattern** (modeled on `chat_settings.go:118-158`):
- Package var struct `presenceWrites` wrapping `presence.SetConsensusEnabled`, `presence.SetConsensusThreshold`, `presence.SetRetractEnabled` + `var presenceReadFresh = presence.GetFresh`.
- Typed `presenceApplyOutcome` (`presenceApplied`, `presenceInvalidValue`, `presenceSaveFailed`) and `presenceApplyResult{Outcome, Settings *presence.Settings, Err}`.
- Settings page calls `applyPresence*` and renders from the read-back row (never writes directly to `presence.*`).

---

### 8. `xenobot/modules/presence_react_api.go` *(NEW)* — Safe Reaction API, `AvailableReactions` Cache & `bot_react` (`PRES-05`)

**Analogs:**
- `xenobot/modules/reactions.go` (lines 32-40 `supportedReactionEmoji`, lines 433-436 `normaliseReactionEmoji`)
- `xenobot/modules/chat_send.go` (lines 24-45 `chatPacer` / `chatPaced`, lines 167-177 `chatSendErrClass`)
- `xenobot/modules/chat_history.go` (lines 20-33, 35-45 compact JSON Redis structs + `chatRedisTimeout`)
- `xenobot/db/cache/loader.go` (`cache.GetFromCacheOrLoad`, `cache.CacheKey`, `cache.DeleteCache`)

**Exact code patterns to reuse/copy:**
1. **Emoji normalization & canonical allowlist** (`reactions.go:32-40, 434-436`):
   - Reuse package-private `normaliseReactionEmoji(raw string) string` (trims + strips every `\ufe0f`) and `supportedReactionEmoji` (the 73 standard Telegram reaction emojis).
   - Extracting standard emojis from `[]gotgbot.ReactionType`:
     ```go
     func presenceStandardEmojis(list []gotgbot.ReactionType) []string {
     	out := make([]string, 0, len(list))
     	for _, rt := range list {
     		merged := rt.MergeReactionType()
     		if merged.Type != "emoji" || merged.Emoji == "" {
     			continue
     		}
     		emoji := normaliseReactionEmoji(merged.Emoji)
     		if emoji != "" && slices.Contains(supportedReactionEmoji, emoji) {
     			out = append(out, emoji)
     		}
     	}
     	return out
     }
     ```
2. **`AvailableReactions` cache (20 min TTL, inside `xenobot:cache:`)**:
   - Store under `cache.CacheKey("avail_react", chatID)` (so it lives in cache with `20 * time.Minute` TTL as locked in `31-CONTEXT.md`, and does NOT need `skipLocal` — or uses `cache.GetFromCacheOrLoad` with `20 * time.Minute` TTL).
   - Cache value struct (msgpack-serializable):
     ```go
     type presenceAvailReactions struct {
     	All     bool     `msgpack:"a"` // true when ChatFullInfo.AvailableReactions is empty/nil
     	Allowed []string `msgpack:"e"` // normalized intersection with supportedReactionEmoji (may be empty if All==false -> custom-only chat)
     }
     ```
   - **Verified `gotgbot/v2` quirk (`STACK.md` §4):**
     - `len(info.AvailableReactions) == 0` → `All: true` (all 73 standard emojis allowed; if `SetMessageReaction` later returns `400 REACTION_INVALID`, silently ignore and drop cache key `cache.DeleteCache(availKey)`).
     - `len(info.AvailableReactions) > 0` → intersect `ReactionTypeEmoji` entries (after `normaliseReactionEmoji`) with `supportedReactionEmoji`; if empty, `All: false, Allowed: nil` (chat allows only custom emojis — skip all reactions!).
   - Call `b.GetChatWithContext(callCtx, chatID, nil)` inside a paced call.
3. **Dedicated best-effort reaction pacer** (modeled on `chat_send.go:27-45`):
   ```go
   var presenceReactionPacer = ratelimit.NewTelegramPacer(ratelimit.TelegramPacerOptions{
   	NextKey:    "xenobot:presence:pace:next",
   	BlockKey:   "xenobot:presence:pace:block",
   	Interval:   100 * time.Millisecond,
   	MaxRetries: 1,
   	MaxWait:    2 * time.Second,
   })
   ```
   - Never uses `chatPacer` (so reaction bursts cannot delay `/ask` replies) and never calls `chatRecordFailure` (Telegram `400 REACTION_INVALID` / `MESSAGE_ID_INVALID` are logged at Debug via `chatSendErrClass(err)` and swallowed silently).
4. **Setting a reaction (`IsBig: false`) + 24h `bot_react` metadata**:
   - Validate `emoji := normaliseReactionEmoji(raw)` is in `supportedReactionEmoji` AND allowed by `AvailableReactions`.
   - Call `b.SetMessageReactionWithContext(callCtx, chatID, messageID, &gotgbot.SetMessageReactionOpts{Reaction: []gotgbot.ReactionType{gotgbot.ReactionTypeEmoji{Emoji: emoji}}, IsBig: false})`.
   - On success, write 24h metadata outside `xenobot:cache:`:
     - Key: `presenceKeyPrefix + "{" + strconv.FormatInt(chatID, 10) + "}:bot_react:" + strconv.FormatInt(messageID, 10)`
     - Payload (compact JSON):
       ```go
       type presenceBotReact struct {
       	Emoji    string `json:"e"`
       	AuthorID int64  `json:"a"`
       	Source   string `json:"s"` // e.g., "cons", "static", "ai", "inspect"
       }
       ```
     - TTL: `24 * time.Hour`.
5. **Clearing a reaction (`👎` Retraction — CRITICAL `gotgbot/v2` wire rule)**:
   ```go
   _, err := b.SetMessageReactionWithContext(callCtx, chatID, messageID, &gotgbot.SetMessageReactionOpts{
   	Reaction: []gotgbot.ReactionType{}, // MUST be non-nil empty slice! nil omits "reaction" on the wire!
   })
   ```
   - Assert exact wire JSON `"reaction":[]` in an `httptest` test!

---

### 9. `xenobot/modules/presence_reactions.go` *(NEW)* — Reaction Router, `👎` Retraction & Emoji Consensus (`PRES-01`, `REACT-04`, `REACT-05`)

**Analogs:**
- `xenobot/modules/chat_run.go` (lines 72-109 `chatJobs`/`chatDraining`/`chatContext()`, lines 172-210 `chatEnqueue`/`chatClaimCooldown`, lines 377-390 `StopChat`)
- `xenobot/modules/chat_brief.go` (lines 297-338 joining `chatJobs` + `chatDraining` + `chatContext()` without a new drain)
- `xenobot/modules/chat_log.go` (lines 57-94 `redis.NewScript` + `chatRedisTimeout`)
- `xenobot/utils/chat_status/chat_status.go:223` (`chat_status.IsUserAdmin(b, chatID, userId)`)

**Joining `StopChat` with ZERO new shutdown drains (`PRES-01`):**
- On every `handleReaction(b *gotgbot.Bot, ctx *ext.Context) error`:
  1. `defer error_handling.RecoverFromPanic("handleReaction", "Presence")`
  2. Guard `chatDraining.Load()` and bracket work with `chatJobs.start()` / `defer chatJobs.finish()`:
     ```go
     chatJobs.start()
     if chatDraining.Load() {
     	chatJobs.finish()
     	return ext.EndGroups
     }
     defer chatJobs.finish()
     runCtx, cancel := context.WithTimeout(chatContext(), presenceReactionTimeout) // e.g. 3s
     defer cancel()
     ```
  3. Because every Redis/Telegram call derives from `runCtx` (child of `chatContext()`) and is counted in `chatJobs`, `StopChat()` immediately cancels in-flight reaction calls and waits inside its existing `chatStopWait` (2 s). Zero new shutdown handlers in `main.go`, 70 s webhook worst case and `stop_grace_period: 75s` unchanged!

**Reaction diffing & Branch A (`👎` Retraction, `REACT-05`):**
- Compute `oldSet`, `newSet`, `added = newSet \ oldSet`, `removed = oldSet \ newSet` over normalized standard emojis (`presenceStandardEmojis`).
- Cheap-first order on `"👎"` in `added`:
  1. Check `settings.RetractEnabled` (skip if off).
  2. Check Redis `GET bot_react:<msgID>` FIRST — if absent or Redis unavailable, return immediately (**zero Telegram API calls!**).
  3. Authorization check:
     - If `presenceIsAnonGroupAdmin(mru)` (`mru.User == nil && mru.ActorChat != nil && mru.ActorChat.Id == mru.Chat.Id`) → **authorized** (zero `getChatMember` call!).
     - Else if `mru.User == nil` (other channel/anonymous actor) → **not authorized**.
     - Else if `rec.AuthorID != 0 && mru.User.Id == rec.AuthorID` → **authorized** (zero `getChatMember` call!).
     - Else → check `presenceIsAdmin(b, chatID, mru.User.Id)` (package var defaulting to `chat_status.IsUserAdmin`).
     - If not authorized → do nothing!
  4. Atomically claim/delete `bot_react:<msgID>` (`client.Del(ctx, key).Int64()` returning `> 0`, or `GetDel`) so concurrent `👎` taps across replicas don't double-call Telegram.
  5. Clear bot reaction via `presenceClearReaction(runCtx, b, chatID, msgID)` (sending explicit `[]gotgbot.ReactionType{}`).
  6. If `mru.User != nil`, extend that user's per-user cooldown key outside `xenobot:cache:` (e.g., `presenceUserCooldownKey(chatID, mru.User.Id)` → `SET "1" EX <extendedCooldown>`, e.g. 24h) so subsequent presence/AI reactions leave them alone.
  7. Post **zero** group messages.

**Branch B (Zero-AI Emoji Consensus, `REACT-04`):**
- Skip when `mru.User == nil` (exclude anonymous admins & channels from distinct-member counting) or `!settings.ConsensusEnabled`.
- Maintain a hostile-emoji denylist (never mirror hostile/negative emojis such as `👎`, `🤬`, `🤮`, `💩`, `🤡`, `🖕`, `😡`, `💔`, `😢`, `😭`).
- Update distinct-reactor sets via an atomic Redis Lua `redis.NewScript` (modeled on `chat_log.go:62-94`) using `{<chatID>}` hash tags so Cluster slot routing is exact:
  - Keys: `xenobot:presence:{<chat>}:cons:u:<msgID>:<emoji_hex>` (48h TTL)
  - For each non-hostile emoji in `added`: `SADD key userID` + `EXPIRE key ttl` + check `SCARD key`.
  - For each emoji in `removed`: `SREM key userID` + `EXPIRE key ttl`.
  - Single-user toggle farming (`🔥` → remove → `🔥`) leaves `SCARD == 1`; Premium user stacking 3 different emojis adds 1 to each emoji's set, never 3 to one set.
- When `SCARD >= threshold` (default `3`, configurable `2..20` in `chat_presence`):
  1. Check `lockdown.GetActiveFresh(chatID)` **lazily here** (fail-closed if locked down or DB error).
  2. Atomically claim per-chat cooldown (`SETNX xenobot:presence:{<chat>}:cons:cd "1" EX <cooldown>`) AND per-message once-only latch (`SETNX xenobot:presence:{<chat>}:cons:done:<msgID> "1" EX 48h`) — better yet, claim both atomically in one small Lua script (so a lost chat cooldown never burns the message latch, and a lost message latch never burns the chat cooldown!).
  3. Resolve `authorID` best-effort (optional `chat_log` lookup at score `msgID` if Redis log exists, else `0` so admins can still retract with `👎`).
  4. Call `presenceSetReaction(runCtx, b, chatID, msgID, authorID, emoji, "cons")` (validating against `AvailableReactions`, `IsBig: false`, writing 24h `bot_react`).

---

### 10. `xenobot/modules/settings_presence.go` *(NEW)* — Settings Page `pr` (`PRES-03`)

**Analogs:**
- `xenobot/modules/settings_advthresholds.go` (lines 210-268: `PrivateOnly: true`, `PrivateNoteKey`, `Hidden`, private field check `fields["c"] != ""`)
- `xenobot/modules/settings_deletion.go` (lines 113-125, 282-310: `Hidden` hiding in Staff Groups + fail-closed `Open`)
- `xenobot/modules/settings_chat.go` (lines 24-150, 249-284, 388-407: `settingsChatGroupRefused`, `settingsChatSetStored`, read-back rendering)
- `xenobot/modules/settings_reactions.go` (line 141: `Right: settingsRightChangeInfo`)

**Exact registration pattern:**
```go
const (
	settingsPagePresence = "pr"
	// Control codes (t values) — keep 1-2 chars so callback data stays well under 64 bytes
	settingsPresenceControlConsensus  = "c"  // tg: 0 or 1
	settingsPresenceControlThreshold  = "ct" // st: integer 2..20 (or stepper/presets)
	settingsPresenceControlRetract    = "r"  // tg: 0 or 1
)

var settingsPresenceStaffLookup = staff.GetStaffGroupFresh
var settingsPresenceRead = presence.GetFresh

func init() {
	registerSettingsPage(settingsPage{
		Key:            settingsPagePresence,
		Order:          91, // Sorts right after y (Order 89, non-OwnerOnly) → pairs y|pr on Main!
		Icon:           "✨",
		TitleKey:       "settings_page_presence",
		OwnerOnly:      false,
		PrivateOnly:    true,
		PrivateNoteKey: "settings_pr_group_note",
		Hidden:         settingsPresenceHidden,
		MainState:      settingsPresenceMainState,
		Open:           settingsPresenceOpen,
		Controls: map[string]settingsControl{
			settingsPresenceControlConsensus: {
				Right: settingsRightChangeInfo, Set: settingsPresenceSetConsensus,
				Actions: []string{settingsActToggle}, Validate: settingsEditSwitchValue,
			},
			settingsPresenceControlThreshold: {
				Right: settingsRightChangeInfo, Set: settingsPresenceSetThreshold,
				Actions: []string{settingsActSet}, Validate: settingsPresenceValidThreshold,
			},
			settingsPresenceControlRetract: {
				Right: settingsRightChangeInfo, Set: settingsPresenceSetRetract,
				Actions: []string{settingsActToggle}, Validate: settingsEditSwitchValue,
			},
		},
	})
}
```

**Why `Order: 91` works cleanly (`settings_pages.go:458-470, 529-568`):**
- `settingsPagesInOrder()` sorts by `Order` then `Key`.
- `renderSettingsMain()` splits entries into `grid` (`!entry.OwnerOnly`) and `anchored` (`entry.OwnerOnly`).
- Currently `y` (Anonymous admins, `Order: 89`, `OwnerOnly: false`) is the 27th grid entry (alone on row 14).
- Adding `pr` (`Order: 91`, `OwnerOnly: false`) makes it the 28th grid entry, pairing `y|pr` into row 14!
- Anchored `OwnerOnly` rows remain `p` (row 15) and `a` (row 16), plus nav row (row 17) → **`settingsMainOwnerRows` stays `17`!**

**Mandatory security & error invariants:**
- `settingsPresenceHidden(group *gotgbot.Chat) bool`: returns `err == nil && row != nil` via `settingsPresenceStaffLookup(group.Id)` (matches `settingsDelHidden`, `settings_deletion.go:284-287`).
- `settingsPresenceOpen` and every control `Set` MUST:
  1. Refuse group menus before any read/write (`if !p.private { p.answer(settingsText(p.tr, "common_callback_invalid_request"), true); return nil }` — matches `settingsChatGroupRefused`).
  2. Fail closed on Staff Group check (if `err != nil || row != nil`, return error / `p.answer(..., true)` — never leak Staff Group identity).
  3. Redraw without DB write if the pressed value equals the stored value.
- All callback data MUST be `<= 64` bytes even with worst-case 14-digit `-1001234567890` chat ID (`screen.Omitted == 0`).
- Every button label MUST be `<= 40` code points (Main grid button `"✨ " + settings_page_presence` MUST be `<= 20` code points in all 8 locales — see `TestSettingsLabelLengths`!).

---

### 11. `xenobot/modules/settings_chat.go` *(MOD)* — Shortcut Row on Page `p` (`PRES-03`)

**Analog:** `xenobot/modules/settings_chat.go` (lines 180-222)

**Exact modification** (insert 1 row right before `k.navRow(false)` at line 220):
```go
	k.row(settingsChatJudgeRow(k, stored.JudgeTo)...)
	if btn, ok := k.button(settingsText(v.Tr, "settings_btn_chat_presence"), map[string]string{
		"a": settingsActOpen, "p": settingsPagePresence,
	}); ok {
		k.row(btn)
	}
	k.navRow(false)
```
**Row count verification (worst case: all 3 providers configured):**
1. Conversations toggle (`e`)
2. Providers 1-2 (`a`, `g`)
3. Provider 3 (`f`)
4. Audience (`e`, `a`)
5. Message log toggle (`l`)
6. Judge destination (`g`, `p`, `s`)
7. **NEW:** Presence shortcut (`a=op&p=pr`)
8. Navigation (`Back | Close`)
Total = **8 rows** — satisfies `TestSettingsChatPageFitsEveryLocale` (`rows <= 8`, `settings_chat_test.go:567`).
- Note: In a Staff Group, `openPage` refuses `a=op&p=pr` via `pg.Hidden` (`settings_menu.go:600-603`), or `renderSettingsChat` can keep the pure button since `settings_menu.go:600` always enforces `pg.Hidden` on press (and page `p` is pure with no DB reads).

---

### 12. Settings Test Updates *(MOD & NEW)*

Every existing test file that pins Main's shape or Staff Group state line counts MUST be updated cleanly:

| Test File | Exact Lines | Required Update |
|-----------|-------------|-----------------|
| `xenobot/modules/settings_main_test.go` | lines 36-38 | `settingsMainShapeGroup`: `... / z\|o / y\|pr / p / a / close`<br>`settingsMainShapePrivate`: `... / z\|o / y\|pr / p / a / groups\|close`<br>`settingsMainOwnerRows = 17` (**UNCHANGED!**) |
| `xenobot/modules/settings_main_test.go` | lines 573-578 | Add `"settings_page_presence"` to `settingsMainTitleKeys` right after `"settings_page_anon"` (before `"settings_page_chat"`, `"settings_page_access"`). |
| `xenobot/modules/settings_backup_test.go` | lines 247, 267-268 | `len(settingsMainTitleKeys)-7` → `len(settingsMainTitleKeys)-8` (Staff Group hides `pr` too, so 9 hidden pages + 1 Blocked sources `v` = `len - 8`; `staffOwnerRows = settingsMainOwnerRows - 4` stays unchanged!). |
| `xenobot/modules/settings_phase18_test.go` | lines 93-94 | `len(settingsMainTitleKeys)-7` → `len(settingsMainTitleKeys)-8` (update comment from 8 to 9 hidden pages). |
| `xenobot/modules/settings_fwdblock_test.go` | lines 349-353 | **NO CHANGE needed!** Because `pr` is hidden in Staff Groups and `settingsMainOwnerRows` stays `17`, Staff Group Main shape and `staffOwnerRows = settingsMainOwnerRows - 4` remain exact! |
| `xenobot/modules/settings_chat_test.go` | lines 525-581 | Verify `TestSettingsChatCallbackBytes` and `TestSettingsChatPageFitsEveryLocale` (`rows <= 8`, `UTF-16 <= 3800`, `Omitted == 0`, button `<= 40` runes) pass with the new shortcut row; add assertion that clicking `a=op&p=pr` opens page `pr`. |
| `xenobot/modules/settings_presence_test.go` *(NEW)* | — | Follow `settings_chat_test.go` + `settings_backup_test.go`: pure 8-locale render/length/64-byte callback test (`Omitted == 0`, `UTF-16 <= 3800`, `rows <= 8`), owner + allowed-admin + denied-admin flows, group menu private-only note (zero DB reads), Staff Group hidden from Main + callback refusal, idempotent press writes nothing, DB write failure shows warn notice. |

---

### 13. Locales (`locales/{en,es,fr,hi,id,pt,ru,uk}.yml`) & `xenobot/i18n/settings_locale_test.go` *(MOD)* (`PRES-03`)

**Analogs:** `locales/en.yml` (lines 2372-2395), `docs/src/content/docs/contributing/locale-style.md`, `xenobot/i18n/settings_locale_test.go` (lines 909-931, 1125-1170)

**Mandatory 8-locale rules (`AGENTS.md` lines 6033-6060 + `locale-style.md`):**
1. Add every new key to ALL 8 files: `en.yml`, `es.yml`, `fr.yml`, `hi.yml`, `id.yml`, `pt.yml`, `ru.yml`, `uk.yml` (never `config.yml`).
2. **HTML only** (`<b>`, `<i>`, `<code>`, `<a>`), never Markdown (`*`, `` ` ``, `_`).
3. **Ukrainian & English style (`TestLocaleStyle`, empty allow-list):**
   - No leading/trailing whitespace, no double spaces, never 3 consecutive newlines.
   - Ukrainian quotes `«…»`; English quotes `"…"`; em dash ` — ` with spaces.
   - Glossary: Ukrainian `група`, `учасник`, `адмін`, `власник групи`, `приватний чат`; English `group`, `member`, `admin`, `group owner`, `private chat`.
   - Avoid the banned two-line reply shape (short opener ≤4 words on line 1 + fact on line 2).
4. **Placeholder parity (`TestLocalePlaceholdersMatchOtherLanguages`):** `{name}` placeholders MUST match identically across all 8 files. Prefer `{name}` over `%s`/`%d`.
5. **Width ceilings (`xenobot/i18n/settings_locale_test.go`):**
   - Register every new `settings_page_presence`, `settings_state_presence*`, `settings_pr_*`, `settings_btn_pr_*`, `settings_btn_chat_presence` key in `settingsLocaleCeilings` (lines 912-931).
   - Register `{"✨", "settings_page_presence"}` in `TestSettingsLabelLengths` `pages` slice (lines 1139-1170) — `"✨ " + title` MUST be `<= 20` runes in ALL 8 languages!
   - Register any alert keys (if any) in `settingsAlertKeys` (plain text, no HTML tags, `<= 200` chars).
6. Run `make check-translations` and `go test -tags testtools ./xenobot/i18n/...`.

---

## Shared Patterns

### 1. Shutdown Lifecycle (`StopChat`, Zero New Drains)
**Source:** `xenobot/modules/chat_run.go` (lines 72-109, 174-191, 380-390) & `xenobot/modules/chat_brief.go` (lines 321-338)
**Apply to:** `xenobot/modules/presence_reactions.go`, `xenobot/modules/presence_react_api.go`
```go
// Enter StopChat tracking before reading chatDraining (exact chatEnqueue / chatBriefEnqueue ordering):
chatJobs.start()
if chatDraining.Load() {
	chatJobs.finish()
	return ext.EndGroups
}
defer chatJobs.finish()
runCtx, cancel := context.WithTimeout(chatContext(), presenceReactionTimeout)
defer cancel()
```
- **Invariant:** `main.go` registers ZERO new shutdown handlers. Webhook worst-case shutdown stays 70 s (`stop_grace_period: 75s`). Tests calling `StopChat()` restore pool state via `restoreChatPool` (see `chat_env_test.go`).

### 2. Redis Key Separation (`xenobot:presence:` vs `xenobot:cache:`)
**Source:** `xenobot/modules/chat_settings.go` (lines 74-93), `xenobot/db/cache/keys.go` (lines 11-30), `xenobot/db/cache/local.go` (lines 44-126)
**Apply to:** `xenobot/db/presence/repository.go`, `xenobot/modules/presence_react_api.go`, `xenobot/modules/presence_reactions.go`
- **Inside `xenobot:cache:` (safe to wipe on `CLEAR_CACHE_ON_STARTUP`):**
  - `cache.CacheKey("chat_presence", chatID)` (`skipLocal` enabled in `local.go`)
  - `cache.CacheKey("avail_react", chatID)` (20 min TTL)
- **Outside `xenobot:cache:` (survives `CLEAR_CACHE_ON_STARTUP`, Cluster-safe `{<chatID>}` hash tag):**
  - `xenobot:presence:{<chat>}:bot_react:<msgID>` (JSON `{e, a, s}`, 24h TTL)
  - `xenobot:presence:{<chat>}:cons:u:<msgID>:<emoji_hex>` (Redis `SET` of distinct user IDs, 48h TTL)
  - `xenobot:presence:{<chat>}:cons:cd` (chat cooldown `STRING`, e.g. 5m TTL)
  - `xenobot:presence:{<chat>}:cons:done:<msgID>` (once-only latch `STRING`, 48h TTL)
  - `xenobot:presence:{<chat>}:cd:u:<userID>` (per-user cooldown extended on `👎` retraction, e.g. 24h TTL)

### 3. Staff Group & Active Lockdown Safety Gate (`PRES-02`)
**Source:** `xenobot/db/staff/repository.go:108,209`, `xenobot/db/lockdown/repository.go:76`, `xenobot/modules/chat_watch.go:61-73`
**Apply to:** `xenobot/modules/presence.go`, `xenobot/modules/presence_reactions.go`, `xenobot/modules/settings_presence.go`
- Always execute cheap/pure checks first (chat type, bot/channel/slash check), then cached `skipLocal` Redis reads (`presence.Get`, `staff.GetStaffGroup`), then Redis state checks (`bot_react`, Lua set-diff), and call `lockdown.GetActiveFresh(chatID)` **lazily only when an action is about to fire** (because it queries PostgreSQL directly). Fail closed on DB errors.

### 4. Error Handling & Panic Recovery
**Source:** `xenobot/modules/chat_watch.go:52-57`, `xenobot/modules/chat_send.go:167-177`, `AGENTS.md` (Go rules)
**Apply to:** All `xenobot/modules/presence*.go` handlers and helpers
- Every handler/goroutine starts with `defer error_handling.RecoverFromPanic("<fn>", "Presence")`.
- Never log user message text, names, or secrets — log `chat_id` / `user_id` and error class (`chatSendErrClass(err)`) only.
- Never trip `chatBreakers` on Telegram `400` (`REACTION_INVALID`, `MESSAGE_ID_INVALID`) or reaction-path errors.

### 5. Lightweight Race-Safe Testing (`AGENTS.md` Testing rules)
**Source:** `xenobot/modules/chat_env_test.go` (lines 97-153), `xenobot/modules/settings_chat_test.go` (lines 525-581)
**Apply to:** All Phase 31 `*_test.go` files
- Build tag `//go:build testtools` on DB-backed tests.
- Never call `t.Parallel()` in `xenobot/modules` tests that mutate package-level seams (`chatRedisClient`, `presenceReactionPacer`, `settingsPresenceRead`, etc.); always restore via `chatSwap` / `t.Cleanup`.
- Keep tests fast (milliseconds): use pure render tests for 8-locale matrices, and targeted single-environment integration tests with miniredis + `httptest` (for `SetMessageReaction` wire JSON `{"reaction":[]}` verification) + SQLite so `xenobot/modules` stays well within CI shard time budgets.
- Never use `t.Skip` outside the PostgreSQL-only migration defaults test.

---

## No Analog Found

All 26 files have exact or strong role-match analogs in Xenobot-72's git-tracked source. No unmapped files remain.

---

## Metadata

**Analog search scope:** `xenobot/config/`, `migrations/`, `xenobot/db/{models,cache,chatconv,migrations,staff,lockdown}/`, `xenobot/modules/{chat*,reactions*,settings*,test_harness_test.go,registry_test.go}`, `xenobot/utils/chat_status/`, `xenobot/i18n/`, `locales/`, `docs/src/content/docs/contributing/locale-style.md`
**Files scanned:** 38 (all verified git-tracked via `git ls-files`)
**Pattern extraction date:** 2026-11-26
