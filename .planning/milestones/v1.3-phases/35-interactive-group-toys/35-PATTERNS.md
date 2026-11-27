# Phase 35: Interactive Group Toys - Pattern Map

**Mapped:** 2026-11-27
**Files analyzed:** 17
**Analogs found:** 15 / 17 (2 new command/worker files synthesize multiple exact codebase analogs)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `migrations/20261201120000_add_presence_toys.sql` | migration | DDL / schema evolution | `migrations/20261130120000_add_presence_room_and_catchup.sql` | exact |
| `xenobot/db/migrations/chat_presence_toys_migration_test.go` | test | DDL / idempotency check | `xenobot/db/migrations/chat_presence_room_migration_test.go` | exact |
| `xenobot/db/models/chat_presence.go` | model | CRUD | `xenobot/db/models/chat_presence.go` (self) | exact |
| `xenobot/db/presence/repository.go` | service | CRUD + skipLocal cache | `xenobot/db/presence/repository.go` (self) | exact |
| `xenobot/db/presence/repository_test.go` | test | CRUD + concurrency | `xenobot/db/presence/repository_test.go` (self) | exact |
| `xenobot/db/backup/types.go` | model | transform / serialization | `xenobot/db/backup/types.go` (self) | exact |
| `xenobot/db/backup/backup.go` | service | CRUD / export-import | `xenobot/db/backup/backup.go` (self) | exact |
| `xenobot/db/backup/presence_backup_test.go` | test | CRUD / roundtrip | `xenobot/db/backup/presence_backup_test.go` (self) | exact |
| `xenobot/utils/chatprompt/toys.go` | utility | transform (leaf prompt + JSON) | `xenobot/utils/chatprompt/reaction.go` | exact |
| `xenobot/utils/chatprompt/toys_test.go` | test | pure unit test | `xenobot/utils/chatprompt/reaction_test.go` + `leaf_test.go` | exact |
| `xenobot/modules/presence.go` | middleware / service | CRUD + Redis key builders + safety gates | `xenobot/modules/presence.go` (self) | exact |
| `xenobot/modules/settings_presence.go` | controller | request-response (settings UI `pc`) | `xenobot/modules/settings_presence.go` (self) | exact |
| `xenobot/modules/chat.go` | route | request-response (command descriptors + docs) | `xenobot/modules/chat.go` (self) | exact |
| `xenobot/modules/chat_run.go` | worker pool | event-driven (`StopChat` + `chatReserveWith`) | `xenobot/modules/chat_run.go` (self) | exact |
| `xenobot/modules/chat_polls.go` | controller / worker | request-response + event-driven (`SendPoll` + `NewPoll`) | `xenobot/modules/chat_brief.go` + `chat_react.go` + `presence_reactions.go` | role-match (multi-analog) |
| `xenobot/modules/chat_inspect.go` | controller / worker | request-response + vision + reaction | `xenobot/modules/chat_react.go` + `chat_brief.go` + `chat_media.go` + `presence_react_api.go` | role-match (multi-analog) |
| `xenobot/i18n/settings_locale_test.go` + `locales/*.yml` + `Makefile` + `docs/` + `AGENTS.md` | config / test / docs | validation / CI gates | `xenobot/i18n/settings_locale_test.go` + `Makefile` + `scripts/generate_docs/parsers.go` | exact |

---

## Verified `gotgbot/v2` Poll & Reply Surfaces (Direct `go doc -src` Verification)

Verified against pinned `github.com/PaulSonOfLars/gotgbot/v2` (`go.mod`):

- **`Bot.SendPollWithContext` signature:**
  ```go
  func (bot *Bot) SendPollWithContext(ctx context.Context, chatId int64, question string, options []InputPollOption, opts *SendPollOpts) (*Message, error)
  ```
- **`InputPollOption` struct:**
  ```go
  type InputPollOption struct {
      Text          string               `json:"text"`
      TextParseMode string               `json:"text_parse_mode,omitempty"`
      TextEntities  []MessageEntity      `json:"text_entities,omitempty"`
      Media         InputPollOptionMedia `json:"media,omitempty"`
  }
  ```
- **`SendPollOpts` relevant fields:**
  - `MessageThreadId int64`
  - `IsAnonymous *bool` (pointer; Telegram defaults to `true` when omitted)
  - `Type string` (`"regular"` or `"quiz"`)
  - `AllowsMultipleAnswers bool`
  - `AllowsRevoting *bool`
  - `AllowAddingOptions bool` (not supported for anonymous polls and quizzes)
  - `OpenPeriod int64` (5–2,628,000 seconds; mutually exclusive with `CloseDate`)
  - `DisableNotification bool`
  - `ReplyParameters *ReplyParameters` (`MessageId`, `AllowSendingWithoutReply`, `PollOptionId`)
- **`Poll` and `PollOption` structs:**
  - `Poll` has `Id string`, `Question string`, `Options []PollOption`, `TotalVoterCount int64`, `IsClosed bool`, `IsAnonymous bool`, `Type string`, `OpenPeriod int64`, `CloseDate int64`. **Crucial:** `Poll` carries **no** `ChatId`, `MessageId`, or `MessageThreadId`.
  - `PollOption` has `PersistentId string`, `Text string`, `VoterCount int64`.
- **Update subscription & handler filter:**
  - `xenobot/config/config.go` lines 546–557 already include `"poll"` in `cfg.AllowedUpdates`.
  - Filter package: `github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/poll` (`poll.All(_ *gotgbot.Poll) bool`, `poll.Regular`).
  - Handler constructor: `handlers.NewPoll(poll.All, responseFunc)`.

---

## Pattern Assignments

### 1. `migrations/20261201120000_add_presence_toys.sql` (migration, DDL)

**Analog:** `migrations/20261130120000_add_presence_room_and_catchup.sql` (lines 1–16)

**Idempotent column pattern** (`migrations/20261130120000_add_presence_room_and_catchup.sql`, lines 1–16):
```sql
-- Per-group Interactive Group Toys settings on chat_presence (TOYS-01, TOYS-03).
--
-- Stored in admin-configurable chat_presence (strictly isolated from Owner-only
-- chat_conversations per PRES-04). Read through the skipLocal cache key chat_presence:<chat>;
-- every write drops that key. Idempotent ADD COLUMN IF NOT EXISTS statements keep rolling
-- upgrades safe without table locks or explicit transactions.

ALTER TABLE chat_presence
    ADD COLUMN IF NOT EXISTS referendum_enabled BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE chat_presence
    ADD COLUMN IF NOT EXISTS inspect_enabled BOOLEAN NOT NULL DEFAULT FALSE;
```
**Constraints:** Must sort strictly after `20261130120000_add_presence_room_and_catchup.sql`, use no explicit `BEGIN`/`COMMIT`, no `CONCURRENTLY`, no `NOT VALID`, no `ADD CONSTRAINT`, and no foreign keys.

---

### 2. `xenobot/db/migrations/chat_presence_toys_migration_test.go` (test, DDL verification)

**Analog:** `xenobot/db/migrations/chat_presence_room_migration_test.go` (lines 14–138)

**Structure to copy** (`chat_presence_room_migration_test.go`, lines 14–138):
- Statement reader helper (`chatPresenceRoomStatements`, lines 19–38) using `MigrationRunner{}` + `cleanSupabaseSQL` + `isTransactionControlStatement`.
- Pure unit test (`TestChatPresenceRoomMigrationStatements`, lines 45–80): checks statement count (`len(run) == 2`), `ALTER TABLE chat_presence` prefix, exact `ADD COLUMN IF NOT EXISTS ... BOOLEAN NOT NULL DEFAULT FALSE` clauses, zero banned SQL keywords (`CONCURRENTLY`, `NOT VALID`, `BEGIN`, `COMMIT`, `ROLLBACK`, `ADD CONSTRAINT`, `REFERENCES`), and filename sort order `> "20261130120000_add_presence_room_and_catchup.sql"`.
- PostgreSQL integration test (`TestChatPresenceToysMigrationDefaults`, lines 85–138): gated by `XENOBOT_TEST_DATABASE=true` + `DATABASE_URL`, runs inside rolled-back transaction twice (idempotency pass 1 & 2), inserts `(chat_id)` only and verifies both booleans default to `false`. Add the test name to `Makefile` `test-postgres-integrity`.

---

### 3. `xenobot/db/models/chat_presence.go` (model, CRUD)

**Analog:** `xenobot/db/models/chat_presence.go` (lines 5–37)

**Boolean column tag rule** (`chat_presence.go`, lines 12–32):
```go
// All booleans carry NO GORM default: tag so a Go false is always written explicitly on
// Create and Save.
type ChatPresence struct {
	...
	AmbientContext     bool      `gorm:"column:ambient_context;not null" json:"ambient_context"`
	VocativeEnabled    bool      `gorm:"column:vocative_enabled;not null" json:"vocative_enabled"`
	PMBriefReact       bool      `gorm:"column:pm_brief_react;not null" json:"pm_brief_react"`
	ReferendumEnabled  bool      `gorm:"column:referendum_enabled;not null" json:"referendum_enabled"`
	InspectEnabled     bool      `gorm:"column:inspect_enabled;not null" json:"inspect_enabled"`
	UpdatedBy          int64     `gorm:"column:updated_by;not null" json:"updated_by"`
	UpdatedAt          time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}
```
**Critical:** Zero `default:` GORM tags on `ReferendumEnabled` and `InspectEnabled`.

---

### 4. `xenobot/db/presence/repository.go` (service, CRUD + skipLocal cache)

**Analog:** `xenobot/db/presence/repository.go` (lines 55–93, 194–210, 337–356, 386–444, 458–509)

**Patterns to extend:**
1. **`Settings` struct & `Defaults()`** (lines 55–93): add short msgpack-tagged booleans so cached Phase 31–34 entries decode cleanly to `false` during rolling deploys:
   ```go
   ReferendumEnabled bool `msgpack:"rf"`
   InspectEnabled    bool `msgpack:"in"`
   ```
   (Confirm no msgpack tag collisions: existing tags are `ce`, `ct`, `re`, `ar`, `cs`, `ae`, `ac`, `at`, `ah`, `am`, `ad`, `ax`, `ve`, `pb`.)
2. **`GetFresh` mapping** (lines 194–209): populate `ReferendumEnabled: row.ReferendumEnabled` and `InspectEnabled: row.InspectEnabled`.
3. **Column-targeted writers** (copy `SetAmbientContext`, lines 337–356):
   ```go
   // SetReferendumEnabled writes referendum_enabled with one column-targeted upsert and drops the cache key.
   func SetReferendumEnabled(ctx context.Context, chatID int64, enabled bool, by int64) error {
   	row := defaultRow(chatID, by)
   	row.ReferendumEnabled = enabled
   	return upsertColumns(ctx, row, []string{"referendum_enabled"})
   }
   ```
   Same for `SetInspectEnabled` (`[]string{"inspect_enabled"}`). Both automatically drop `CacheKey(chatID)` (`chat_presence:<chat>`, already registered in `skipLocal` at `xenobot/db/cache/local.go:63`).
4. **`ReplacePortableTx` & `defaultRow`** (lines 407–444, 458–479): add `ReferendumEnabled` and `InspectEnabled` to `row` construction and `"referendum_enabled"`, `"inspect_enabled"` to `portableCols`.

---

### 5. `xenobot/db/backup/types.go`, `xenobot/db/backup/backup.go`, `xenobot/db/backup/presence_backup_test.go` (service, Backup parity)

**Analogs:**
- `xenobot/db/backup/types.go` lines 236–254 (`PresencePortableBackup`)
- `xenobot/db/backup/backup.go` lines 1427–1496 (`exportPresenceData`, `importPresence`, `clearPresence`)
- `xenobot/db/backup/presence_backup_test.go` lines 47–80 (`TestBackupPresenceArchiveStripped`)

**Code excerpts to extend:**
In `types.go` (lines 239–252), add:
```go
	ReferendumEnabled  bool   `json:"referendum_enabled"`
	InspectEnabled     bool   `json:"inspect_enabled"`
```
In `backup.go` `exportPresenceData` (lines 1435–1450) and `importPresence` (lines 1465–1481), wire `ReferendumEnabled` and `InspectEnabled` into `PresencePortableBackup` and `presence.Settings`. Legacy backups without those keys JSON-unmarshal cleanly to `false`. `chat_conversations` stays strictly excluded (`TestChatConversationsStayOutOfBackup`), and `xenobot/modules/copy_settings.go` stays untouched.

---

### 6. `xenobot/utils/chatprompt/toys.go` & `toys_test.go` (utility, leaf prompt + JSON parser)

**Analog:** `xenobot/utils/chatprompt/reaction.go` (lines 11–241) + `xenobot/utils/chatprompt/leaf_test.go` (lines 14–44)

**Leaf constraint (`leaf_test.go`, lines 33–39):** `chatprompt` MUST import ONLY Go stdlib and `github.com/uasneppy/Xenobot-72/xenobot/utils/xenopersona`. Keep all prompt builders, JSON parsers, and Unicode/rune length sanitizers here using stdlib (`encoding/json`, `bytes`, `io`, `slices`, `strings`, `unicode`, `unicode/utf8`) so `TestChatpromptPackageIsLeaf` stays green.

**Imports & token floor pattern** (`reaction.go`, lines 1–14):
```go
const (
	// ReferendumMaxTokens gives Claude, Gemini 4 thinking tokens, and Featherless reasoning prose
	// headroom so localized Ukrainian/Russian poll questions and 2-5 options never truncate.
	ReferendumMaxTokens = 768
	// InspectMaxTokens matches ReactionMaxTokens (512) for short 2-3 line verdict + emoji JSON.
	InspectMaxTokens = 512
)
```
**Prompt + strict JSON parsing pattern to copy from `reaction.go` (lines 45–106, 139–241):**
- **Static prompt** (byte-identical across chats for prompt caching): Xenobot-72 deadpan facility-management persona, prompt-injection defense (treat member chat text as untrusted data), and task contract.
- **Dynamic prompt** (`Clean(chatTitle, MaxTitleRunes)`, `Clean(language, 40)`):
  - For `/referendum`: group title, language, JSON schema `{"question":"...","options":["...","..."]}` (2–5 concise options, question <= 300 runes, each option <= 100 runes, in the group's language).
  - For `/inspect`: group title, language, normalized allowed emoji palette (`normalizeReactionPalette`), explicit non-Telegram lookalike warning copied from `reaction.go:93-96` (`😂` and `🐱, 😹, 😻, 😼` are NOT valid Telegram reactions), JSON schema `{"verdict":"...","emoji":"..."}` (2–3 crisp deadpan sentences/lines, no markdown code blocks required, single allowed emoji).
- **User prompt**:
  - For `/referendum`: optional `[Replied debate message]` (`line(*reply, MaxQuotedRunes)`) + optional `[Recent topic context, oldest first]` (`[]Turn`) + optional `[Member prompt]` (`Clean(topic, MaxMessageRunes)`) + untrusted-data reminder (`reaction.go:135`).
  - For `/inspect`: `[Message to inspect]` (`line(target, MaxMessageRunes)`) + optional `[Quoted excerpt]` + optional `[Media description]` + untrusted-data reminder.
- **Strict JSON parser pattern** (`reaction.go`, lines 159–241):
  - `stripReactionCodeFence(raw)` to strip a single optional markdown fence.
  - `json.NewDecoder(strings.NewReader(s))` + trailing `dec.Decode(&trailing) != io.EOF` check to reject trailing garbage.
  - For `/referendum` (`ParseReferendumJSON`): validate `question = Clean(rawQuestion, 300)` non-empty; iterate `options`, run `Clean(opt, 100)`, skip empty, deduplicate case-insensitively (`strings.ToLower`), cap at 5 options, require `len(options) >= 2`.
  - For `/inspect` (`ParseInspectJSON`): validate `verdict` non-empty (normalize line breaks via `CleanFileContent` or `Clean` and rune-bound e.g. `<= 600`), validate `emoji` via `stripReactionVS16(token)` + `containsNormalizedEmoji(allowed, norm)` (fail closed to error or allow caller to check whether reaction emoji is valid).

---

### 7. `xenobot/modules/presence.go` (middleware/service, Redis keys + safety gates + apply layer)

**Analog:** `xenobot/modules/presence.go` (lines 25–147, 158–212, 347–381)

**Patterns to extend:**
1. **Redis operational keys** (lines 25–90, strictly outside `xenobot:cache:` so `CLEAR_CACHE_ON_STARTUP` never wipes them):
   - Poll metadata key (`chat_polls.go` or `presence.go`): `presenceKeyPrefix + "poll:" + pollID` (or `chatKeyPrefix + "poll:" + pollID`, e.g., `xenobot:presence:poll:<poll_id>` matching `35-CONTEXT.md`, TTL `> OpenPeriod`, e.g. `25 * time.Hour` when `OpenPeriod` is 1h/24h, or `openPeriod + 24*time.Hour`).
   - Tie-breaker once-only claim key: `presenceKeyPrefix + "poll:tie:" + pollID` (`SETNX` with 24h TTL).
   - Per-chat / per-user toy cooldown keys using `{<chatID>}` hash tag: `presenceChatTag(chatID) + "toy:cd"`, `presenceChatTag(chatID) + "toy:cd:u:" + strconv.FormatInt(userID, 10)`.
2. **Apply layer** (lines 158–212, 347–381):
   - Add `ReferendumEnabled` and `InspectEnabled` to `presenceWrites` struct (`presence.SetReferendumEnabled`, `presence.SetInspectEnabled`).
   - Add `applyPresenceReferendumEnabled` and `applyPresenceInspectEnabled` copying `applyPresenceVocativeEnabled` (lines 357–363).

---

### 8. `xenobot/modules/settings_presence.go` (controller, Private Settings UI)

**Analog:** `xenobot/modules/settings_presence.go` (lines 45–58, 183–235, 389–437, 937–1097)

**Row & callback constraints (verified in `settings_presence.go` & `settings_phase33_test.go`):**
- Main grid: `len(settingsPages) == 31`, `settingsMainOwnerRows == 17` (`settings_main_test.go:36-38`).
- Page `pr` (`renderSettingsPresence`, lines 514–538) is already **8 rows** (5 controls + 2 sub-page buttons `ph`/`pc` + `navRow`).
- Sub-page `pc` (`renderSettingsPresenceRoomCatchup`, lines 954–975) currently has **4 rows**:
  - Row 1: `ambient_context` toggle (`t=ax`)
  - Row 2: `vocative_enabled` toggle (`t=ve`)
  - Row 3: `pm_brief_react` toggle (`t=pb`)
  - Row 4: `[Back | Close]`
  Adding `referendum_enabled` (`t=rf`) and `inspect_enabled` (`t=in`) directly onto `pc` brings `pc` to **6 rows** (still `<= 8` rows), keeps `pr` at **8 rows**, keeps `len(prPage.Actions) == 3` (`ph`, `pu`, `pc`), keeps `len(settingsPages) == 31`, and keeps `settingsMainOwnerRows == 17`!
- **Owner×Admin double-gating on `pc`** (`settingsPresenceSetRoomToggle`, lines 1017–1097):
  - Both `/referendum` and `/inspect` depend on Owner-enabled `chat_conversations` (not `LogEnabled` — `/referendum` falls back gracefully or uses `chat_log` when on, and `/inspect` inspects a replied message directly). Therefore both use `settingsPresenceNeedConv` (alerting `settings_pr_ai_need_owner` when `!convUnlocked`, just like `settingsPresenceSetVocative` at lines 1079–1087):
    ```go
    func settingsPresenceSetReferendum(p *settingsPress, value string) *settingsScreen {
    	return settingsPresenceSetRoomToggle(
    		p, value, settingsPresenceNeedConv,
    		func(s presence.Settings) bool { return s.ReferendumEnabled },
    		func(ctx context.Context, enabled bool) presenceApplyResult {
    			return applyPresenceReferendumEnabled(ctx, p.group.Id, enabled, p.query.From.Id)
    		},
    	)
    }
    ```
  - Update `settingsPresenceMainState` (lines 401–433) so `ReferendumEnabled && convUnlocked` and `InspectEnabled && convUnlocked` count toward Main's active feature count (`1..10 on`), and update `renderSettingsPresence` `roomActive` (lines 480–493) to include `ReferendumEnabled` and `InspectEnabled`.

---

### 9. `xenobot/modules/chat.go` & `xenobot/modules/chat_run.go` (command registration & pool stats)

**Analogs:** `xenobot/modules/chat.go` (lines 24–77), `xenobot/modules/chat_run.go` (lines 143–160, 232–255, 432–442), `xenobot/modules/presence_reactions.go` (lines 97–107)

**Command registration & `generate_docs` parser rule (verified in `scripts/generate_docs/parsers.go` lines 154, 270–310):**
- `35-CONTEXT.md` notes `helpers.NewBotCommand("referendum", ...)` and `helpers.NewBotCommand("inspect", ...)`.
- In `xenobot/modules/chat.go`, `/unrollshort` and `/unrolllong` use `helpers.WrapCommand` (lines 26–37, 66–67), which internally calls `helpers.NewBotCommand`, registers `AddCmdToDisableable` when `Disableable: true`, runs `BuildCommandContext` (building `c.Tr` and refusing anonymous users without identity), and runs `RequiredChecks`:
  ```go
  var referendumDesc = helpers.CommandDescriptor{
  	Name:           "referendum",
  	Disableable:    true,
  	RequiredChecks: []helpers.CheckFunc{helpers.CheckDisabled("referendum"), helpers.RequireGroup()},
  }
  ```
  - If using `helpers.WrapCommand(dispatcher, referendumDesc, chatModule.referendumCommand)`, `parsers.go` lines 270–310 automatically extracts both `referendumDesc` and `chatModule` -> `Chat` module docs (`docs/src/content/docs/commands/chat/index.md`) with `Disableable: true`.
  - If using `helpers.NewBotCommand("referendum", chatModule.referendumCommand)` directly, `parsers.go` line 154 also maps `chatModule` -> `Chat`, and `AddCmdToDisableable("referendum")` would be needed if disableable. Either way, `chatModule` MUST be the receiver variable so `generate_docs` assigns `/referendum` and `/inspect` to `Chat` (not `chat_polls` / `chat_inspect`), and `DefaultHelpRegistry().AltHelpOptions["Chat"]` (line 75) MUST include `"referendum"` and `"inspect"`.
- **Group `0` `NewPoll` registration** (`presence_reactions.go:98` / `chat.go`):
  ```go
  dispatcher.AddHandler(handlers.NewPoll(poll.All, chatModule.handlePollUpdate))
  ```
  Register in `LoadChat` (or `LoadPresence` in `presence_reactions.go` calling a `chat_polls.go` handler) — group `0` non-message update handler, zero new message handler groups.
- **Pool integration (`chat_run.go`, lines 143–160, 234–255):**
  - Append `+ chatToysStatsSection()` to `chatStatsSection()` (line 147) so `/stats` exposes toy counters (numbers only, no IDs/text).
  - Reuse `chatPresenceQueue` (launched in `chatEnsureWorkers` at line 158) or dispatch typed toy work via a small helper in `chat_polls.go`/`chat_inspect.go` that calls `chatEnsureWorkers()`, `chatJobs.start()`/`finish()`, checks `chatDraining.Load()`, and derives from `chatContext()`. Note: keeping `chatPresenceQueue` strictly for `chatReactJob` or widening `chatPresenceQueue` to a small task struct (or calling `chatToysEnqueue` with a `2`-worker or `chatPresenceQueue` unified item) must still obey `chatJobs.start()`/`finish()`, `chatDraining.Load()`, and `chatContext()` so `StopChat()` drains all in-flight toy jobs with **zero new shutdown drains**! (Using a single `func()` or tagged `chatPresenceItem` on `chatPresenceQueue` avoids spawning extra idle goroutines while sharing `chatPresenceWorkerCount = 2` and `StopChat()`.)

---

### 10. `xenobot/modules/chat_polls.go` (`/referendum` + `handlers.NewPoll` Tie-Breaker, `TOYS-01`, `TOYS-02`)

**Analogs:**
- Command gate & context loading: `xenobot/modules/chat_brief.go` (lines 148–168, 230–294, 454–546)
- Queue & worker lifecycle (`StopChat`, `chatReserveWith`, `chatBreaker`): `xenobot/modules/chat_react.go` (lines 184–201, 463–520) and `xenobot/modules/chat_run.go` (lines 234–255)
- Send pacing & sanitization: `xenobot/modules/chat_send.go` (lines 39–45, 63–67, 97–122)
- Non-message Group `0` handler under `StopChat`: `xenobot/modules/presence_reactions.go` (lines 169–185)

**Exact patterns to copy:**
1. **Command gate (`chat_brief.go:230-294` + Owner×Admin×Safety checks):**
   - Check group (`chat.Type == "group" || chat.Type == "supergroup"`), real non-bot user (`raidFloodSenderCounts(b, ctx)` + `!user.IsBot`), `!presenceIsStaffGroup(chat.Id)` (Staff Groups refused/ignored before any AI call), `!presenceIsLockedDown(chat.Id)` (or lazy check inside worker before sending/AI).
   - Double-gate (`chat_react.go:277-284` + `chat_watch.go:122-143`):
     - `chatAnyProviderConfigured()`
     - `conv, err := chatSettingsLookup(ctx, chat.Id)` -> require `conv.Enabled && chatProviderConfigured(conv.Provider)`
     - `pres, err := presenceSettingsGet(ctx, chat.Id)` -> require `pres.ReferendumEnabled`
     - `chatAudienceAllows(ctx, b, conv.Audience, chat.Id, userID)`
     - `chatRedisClient() != nil`
   - Context gathering (without `LogEnabled` being mandatory if the user replied to a message or provided a topic!):
     - Reply target: `extraction.ReplyTarget(msg)` -> extract text/caption/`chatMediaLabel`.
     - Recent same-thread log: if `conv.LogEnabled`, call `chatLogRecentThread(ctx, client, chat.Id, threadID, msg.MessageId, 8)` (strictly `entry.Thread == threadID`).
     - Optional command argument: `chatCommandArgument(chatMessageText(msg))`.
     - If neither reply target, nor argument, nor recent thread context exists, reply with crisp localized hint (e.g. `referendum_need_context`).
2. **Budget & cooldown (`chat_run.go:234-255` + `chat_react.go:477-490`):**
   - Claim per-chat cooldown (`presenceToyChatCooldownKey(chat.Id)`) via `chatClaimCooldown`.
   - Check `chatBreaker(job.provider).isPaused(job.chatID, raidNow())`.
   - Reserve dedicated hourly budget `"toy:b"` via `chatReserveWith(client, job.chatID, job.userID, "toy:b", chatToyUserHourlyBudget, chatToyChatHourlyBudget, chatToyGlobalHourlyBudget)` (isolated from `"b"`, `"brief:b"`, and `"react:b"`).
3. **LLM call & strict validation (`chat_react.go:504-518` + `chatprompt/toys.go`):**
   - `MaxTokens: chatprompt.ReferendumMaxTokens` (`>= 512`, recommend `768`).
   - On provider error: if `root.Err() != nil || errors.Is(err, context.Canceled)` -> shed silently; otherwise `chatRecordFailure` (or localized malfunction line on user-invoked `/referendum`, matching `chat_brief.go:537-542`, while parse errors answer malfunction without tripping breaker if malformed JSON).
   - Parse with `chatprompt.ParseReferendumJSON(answer.Text)` + `chatSanitizeText` on `question` and every `option` (re-check length `1..300` for question, `1..100` for each option, `2..5` deduplicated non-empty options).
4. **Native `SendPollWithContext` & Redis poll metadata (`TOYS-01`):**
   ```go
   isAnon := false // or configurable/true; note AllowAddingOptions: false works for both anonymous and non-anonymous polls
   openPeriod := int64(3600) // e.g. 1h (within 5..2628000s)
   var sent *gotgbot.Message
   err := chatPaced(ctx, func(callCtx context.Context) error {
   	var callErr error
   	sent, callErr = job.bot.SendPollWithContext(callCtx, job.chatID, parsed.Question, options, &gotgbot.SendPollOpts{
   		MessageThreadId:     job.threadID,
   		IsAnonymous:         &isAnon,
   		Type:                "regular",
   		OpenPeriod:          openPeriod,
   		AllowAddingOptions:  false,
   		DisableNotification: true,
   		ReplyParameters: &gotgbot.ReplyParameters{
   			MessageId:                job.messageID,
   			AllowSendingWithoutReply: true,
   		},
   	})
   	return callErr
   })
   ```
   Immediately on success (`sent != nil && sent.Poll != nil`), persist `xenobot:presence:poll:<sent.Poll.Id>` (outside `xenobot:cache:`, TTL `time.Duration(openPeriod)*time.Second + 24*time.Hour` e.g. `25*time.Hour`) storing JSON `{chat_id, message_id, thread_id, lang, question}`.
5. **Group `0` `handlers.NewPoll` tie-breaker (`TOYS-02`):**
   - Copy `handleReaction` `StopChat` join pattern from `presence_reactions.go` lines 169–185 (`chatJobs.start()` / `chatDraining.Load()` / `defer chatJobs.finish()`).
   - Guard: `p := ctx.Poll; if p == nil || !p.IsClosed || p.TotalVoterCount <= 0 { return ext.EndGroups }`.
   - Compute max `VoterCount` across `p.Options`; count how many options have `VoterCount == maxVotes` (`maxVotes > 0`). If `tiedCount < 2`, return immediately.
   - Lookup `xenobot:presence:poll:<p.Id>` in Redis; if missing or unreadable, ignore (external/unknown poll).
   - Verify `!presenceIsStaffGroup(meta.ChatID)` and `!presenceIsLockedDown(meta.ChatID)`.
   - Atomic once-only cross-replica claim: `client.SetNX(ctx, "xenobot:presence:poll:tie:"+p.Id, "1", 24*time.Hour)`. If `!won`, return.
   - Generate or format one deadpan "Facility Tie-Breaker" verdict (either AI-generated via `chatProviderOf` under `chatContext()` if `conv.Enabled && pres.ReferendumEnabled`, or crisp localized deadpan fallback when AI fails/disabled — **never** post on 0-vote polls) and send via `chatPaced` with `MessageThreadId: meta.ThreadID` and `ReplyParameters: &gotgbot.ReplyParameters{MessageId: meta.MessageID, AllowSendingWithoutReply: false}` (so a deleted poll stays silent).

---

### 11. `xenobot/modules/chat_inspect.go` (`/inspect`, `TOYS-03`)

**Analogs:**
- Command prep & reply checks: `xenobot/modules/chat_brief.go` (lines 230–294)
- Vision media extraction + Featherless `text_only` fallback: `xenobot/modules/chat_react.go` (lines 205–218, 380–436) + `xenobot/modules/chat_media.go` (lines 167–211, 232–275)
- Reaction validation & `bot_react` stamping: `xenobot/modules/presence_react_api.go` (lines 88–136, 208–241)
- Reply send (`AllowSendingWithoutReply: false`): `xenobot/modules/chat_send.go` (lines 69–112)

**Exact patterns to copy:**
1. **Command gate:**
   - Require group, real non-bot user (`raidFloodSenderCounts`), `!presenceIsStaffGroup(chat.Id)`, `!presenceIsLockedDown(chat.Id)`.
   - Require `reply := extraction.ReplyTarget(msg)` — if `reply == nil`, reply with localized usage hint (`inspect_usage`) and spend nothing.
   - Enforce Owner×Admin double-gate (`conv.Enabled && chatProviderConfigured(conv.Provider) && pres.InspectEnabled`) + `chatAudienceAllows(ctx, b, conv.Audience, chat.Id, userID)` + `chatRedisClient() != nil`.
   - Extract target content from `reply`: text/caption (`chatMessageText(reply)`), sticker/photo media (`chatExtractReactionMediaRef(reply)` — or `chatExtractMediaRef(reply)` if supporting image documents too, keeping photos & stickers primary per `TOYS-03`), and `chatMediaLabel(reply)` fallback so stickers without captions still have `[sticker 🐱]`.
2. **Worker execution under `StopChat` (`chatContext()`):**
   - Per-user/chat cooldown + `chatBreaker(job.provider).isPaused` + `chatReserveWith(..., "toy:b", ...)`.
   - Fetch chat's allowed reaction palette: `palette, ok := presenceAllowedPalette(ctx, job.bot, job.chatID)` (`presence_react_api.go:93-126`). Note: if `palette` is empty (custom-only reactions in chat), `/inspect` can still post its text verdict while skipping the `presenceSetReaction` stamp!
   - Download media with Featherless `text_only` degradation (`chat_react.go:418-436`):
     ```go
     media := chatReactDownloadVision(ctx, client, ...) // checks chatIsFeatherlessTextOnly
     answer, err := chatReactCallProvider(ctx, root, client, job.provider, req) // retries without Attachments & caches text_only on 400/422
     ```
   - `MaxTokens: chatprompt.InspectMaxTokens` (`512`).
   - Parse JSON via `chatprompt.ParseInspectJSON(answer.Text, palette)`:
     - Sanitize `verdict` with `chatFilterReply` / `chatSanitizeText` (2–3 crisp lines).
     - Validate `emoji` (`U+FE0F` stripped, `supportedReactionEmoji ∩ palette`, rejecting lookalikes `😂`, `🐱`/`😹`/`😻`/`😼`).
3. **Reply + `bot_react` stamp (`TOYS-03`):**
   - Send verdict as a reply to `reply.MessageId` (not the `/inspect` command message, or check `AllowSendingWithoutReply: false` on `reply.MessageId` so a deleted target stays silent — matching `chat_send.go:97-112` with `AllowSendingWithoutReply: false` and `MessageThreadId: job.threadID`).
   - Stamp the validated emoji onto `reply.MessageId` via `presenceSetReaction(ctx, job.bot, job.chatID, job.targetMsgID, job.targetAuthorID, emoji, "inspect")` (`presence_react_api.go:208-241`). Passing `job.targetAuthorID` (`reply.From.Id`, or `0` if nil) and source `"inspect"` automatically writes `xenobot:presence:{<chat>}:bot_react:<msgID>` (TTL 24h), so `👎` retraction in `presence_reactions.go:228-261` clears the `/inspect` reaction with zero extra code!

---

## Shared Patterns

### Owner×Admin Double-Gating & Safety Gate
**Source:** `xenobot/modules/presence.go` (lines 103–147) & `xenobot/modules/chat_react.go` (lines 254–285)
**Apply to:** `chat_polls.go`, `chat_inspect.go`, `settings_presence.go`
```go
if !presenceChatTypeAllowed(chat) || presenceIsStaffGroup(chat.Id) || presenceIsLockedDown(chat.Id) {
	return
}
pres, err := presenceSettingsGet(ctx, chat.Id)
if err != nil || !pres.ReferendumEnabled { // or pres.InspectEnabled
	return
}
conv, err := chatSettingsLookup(ctx, chat.Id)
if err != nil || !conv.Enabled || !chatProviderConfigured(conv.Provider) {
	return
}
if !chatAudienceAllows(ctx, b, conv.Audience, chat.Id, userID) {
	return
}
```

### Dedicated `"toy:b"` Hourly Budget & Cooldown
**Source:** `xenobot/modules/chat_run.go` (lines 209–255)
**Apply to:** `chat_polls.go`, `chat_inspect.go`
```go
switch chatReserveWith(client, job.chatID, job.userID, "toy:b",
	chatToyUserHourlyBudget, chatToyChatHourlyBudget, chatToyGlobalHourlyBudget) {
case chatBudgetCapped:
	...
case chatBudgetFault:
	...
}
```
Keys generated: `xenobot:chat:toy:b:u:<chat>:<user>:<hour>`, `xenobot:chat:toy:b:c:<chat>:<hour>`, `xenobot:chat:toy:b:g:<hour>` (all outside `xenobot:cache:`).

### `StopChat` Lifecycle Joining (Zero New Shutdown Drains)
**Source:** `xenobot/modules/chat_react.go` (lines 184–201) & `xenobot/modules/presence_reactions.go` (lines 169–185)
**Apply to:** `chat_polls.go`, `chat_inspect.go` (both async commands and `handlers.NewPoll`)
- Increment `chatJobs.start()` before reading `chatDraining.Load()`; if draining, call `chatJobs.finish()` and shed immediately.
- Derive every provider HTTP call and paced Telegram API call from `chatContext()`.
- Keep webhook worst-case shutdown at **70 s** and `stop_grace_period: 75s` unchanged (`main.go` untouched).

### Locale & Docs & CI Parity Gates
**Sources:** `xenobot/i18n/settings_locale_test.go` (lines 991–1045, 1312–1319), `scripts/generate_docs/parsers.go` (lines 154, 270–310), `Makefile` (lines 74–79)
**Apply to:** All Phase 35 `settings_*` keys, `/referendum` & `/inspect` help/locale keys, migration tests
- Every new `settings_*` key across all 8 `locales/*.yml` files MUST have an entry in `settingsLocaleLimits` (`settings_locale_test.go`).
- Toggle button keys placed in `phase18Toggles` (`settings_locale_test.go:1312-1319`) are verified against `<= 40` rendered runes with `✅ On` / `❌ Off`.
- Plain-text `answerCallbackQuery` alert keys belong in `settingsAlertKeys` (`<= 200` runes, no `<` or `>`).
- Run `make check-translations`, `make generate-docs` + `make check-docs`, `GOTOOLCHAIN=go1.26.0 make lint`, and update `Makefile` `test-postgres-integrity` and `AGENTS.md`.

---

## No Analog Found

All 17 new/modified files have direct tracked analogs in the Xenobot-72 codebase; `chat_polls.go` and `chat_inspect.go` compose verified patterns from `chat_brief.go`, `chat_react.go`, `chat_media.go`, `chat_send.go`, `presence_reactions.go`, and `presence_react_api.go`, paired with the verified `gotgbot/v2` `SendPollWithContext` / `handlers.NewPoll` contracts above.

---

## Metadata

**Analog search scope:** `migrations/`, `xenobot/db/{models,presence,backup,cache,migrations}/`, `xenobot/utils/{chatprompt,helpers}/`, `xenobot/modules/`, `xenobot/i18n/`, `xenobot/config/`, `scripts/generate_docs/`, `locales/`, `Makefile`
**Files scanned:** 24 tracked files + `gotgbot/v2` `go doc -src`
**Pattern extraction date:** 2026-11-27
