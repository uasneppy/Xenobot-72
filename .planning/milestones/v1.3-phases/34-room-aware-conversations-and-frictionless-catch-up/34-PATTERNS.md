# Phase 34: Room-Aware Conversations and Frictionless Catch-Up - Pattern Map

**Mapped:** 2026-11-27
**Files analyzed:** 20 (new & modified across migrations, db, utils, modules, locales, tests, docs)
**Analogs found:** 20 / 20

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `migrations/20261130120000_add_presence_room_and_catchup.sql` | migration | schema-DDL | `migrations/20261129130000_add_presence_stickers_and_archive.sql` | exact |
| `xenobot/db/models/chat_presence.go` | model | CRUD | `xenobot/db/models/chat_presence.go` | exact |
| `xenobot/db/presence/repository.go` | repository | CRUD (cached `skipLocal`) | `xenobot/db/presence/repository.go` | exact |
| `xenobot/db/presence/repository_test.go` | test | CRUD + cache | `xenobot/db/presence/repository_test.go` | exact |
| `xenobot/db/backup/types.go` | model/config | transform (JSON DTO) | `xenobot/db/backup/types.go` | exact |
| `xenobot/db/backup/backup.go` | service | CRUD + transform | `xenobot/db/backup/backup.go` | exact |
| `xenobot/db/backup/presence_backup_test.go` | test | CRUD + transform | `xenobot/db/backup/presence_backup_test.go` | exact |
| `xenobot/utils/chatprompt/ambient.go` (or in `chatprompt.go`) | utility (leaf) | transform | `xenobot/utils/chatprompt/chatprompt.go` & `xenobot/utils/chatprompt/reaction.go` | exact |
| `xenobot/utils/chatprompt/ambient_test.go` (or `chatprompt_test.go`) | test | transform | `xenobot/utils/chatprompt/reaction_test.go` & `xenobot/utils/chatprompt/leaf_test.go` | exact |
| `xenobot/modules/chat_log.go` | watcher/store | event-driven + Redis ZSET | `xenobot/modules/chat_log.go` | exact |
| `xenobot/modules/chat_log_test.go` | test | Redis ZSET | `xenobot/modules/chat_log_test.go` | exact |
| `xenobot/modules/chat_history.go` | store | Redis list | `xenobot/modules/chat_history.go` | exact |
| `xenobot/modules/chat_prompt.go` | service | transform | `xenobot/modules/chat_prompt.go` | exact |
| `xenobot/modules/chat_run.go` | worker/pool | async queue | `xenobot/modules/chat_run.go` | exact |
| `xenobot/modules/chat_watch.go` | watcher (group 13) | event-driven | `xenobot/modules/chat_watch.go` | exact |
| `xenobot/modules/chat_watch_test.go` | test | event-driven + fake TG | `xenobot/modules/chat_watch_test.go` | exact |
| `xenobot/modules/chat_brief.go` | command + worker | request-response + async queue | `xenobot/modules/chat_brief.go` | exact |
| `xenobot/modules/chat_brief_test.go` | test | request-response + fake TG | `xenobot/modules/chat_brief_test.go` | exact |
| `xenobot/modules/presence.go` | service/apply | CRUD | `xenobot/modules/presence.go` | exact |
| `xenobot/modules/presence_reactions.go` | router (group 0) | event-driven (`NewReaction`) | `xenobot/modules/presence_reactions.go` | exact |
| `xenobot/modules/presence_reactions_test.go` | test | event-driven + httptest | `xenobot/modules/presence_reactions_test.go` | exact |
| `xenobot/modules/settings_presence.go` | settings UI (`pr`) | request-response (callback) | `xenobot/modules/settings_presence.go` | exact |
| `xenobot/modules/settings_presence_test.go` | test | settings UI + fake TG | `xenobot/modules/settings_presence_test.go` | exact |
| `xenobot/modules/settings_phase33_test.go` (or `settings_phase34_test.go`) | test | settings callback invariant | `xenobot/modules/settings_phase33_test.go` | exact |
| `locales/*.yml` & `xenobot/i18n/settings_locale_test.go` | config/test | static validation | `locales/en.yml` & `xenobot/i18n/settings_locale_test.go` | exact |

---

## Pattern Assignments

### 1. Schema Migration (`migrations/20261130120000_add_presence_room_and_catchup.sql`)

**Analog:** `migrations/20261129130000_add_presence_stickers_and_archive.sql` (lines 1–19) and `migrations/20261128120000_add_chat_presence_ai_react_mode.sql` (lines 1–13)

**Idempotent column pattern** (`20261129130000_add_presence_stickers_and_archive.sql` lines 9–14):
```sql
ALTER TABLE chat_presence
    ADD COLUMN IF NOT EXISTS cat_sticker_enabled BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE chat_presence
    ADD COLUMN IF NOT EXISTS archive_enabled BOOLEAN NOT NULL DEFAULT FALSE;
```
Add `ambient_context`, `vocative_enabled`, and `pm_brief_react` as `BOOLEAN NOT NULL DEFAULT FALSE` using the exact same `ALTER TABLE chat_presence ADD COLUMN IF NOT EXISTS ...` statements, timestamped strictly after `20261129130000`.

---

### 2. Presence Model & Repository (`xenobot/db/models/chat_presence.go`, `xenobot/db/presence/repository.go`, `repository_test.go`)

**Analog:** `xenobot/db/models/chat_presence.go` (lines 11–28) & `xenobot/db/presence/repository.go` (lines 55–86, 175–241, 355–439)

**No GORM `default:` tag on booleans** (`xenobot/db/models/chat_presence.go` lines 11–20):
```go
// All booleans carry NO GORM default: tag so a Go false is always written explicitly on
// Create and Save.
type ChatPresence struct {
	ChatID             int64     `gorm:"column:chat_id;primaryKey;autoIncrement:false" json:"chat_id"`
	ConsensusEnabled   bool      `gorm:"column:consensus_enabled;not null" json:"consensus_enabled"`
...
	CatStickerEnabled  bool      `gorm:"column:cat_sticker_enabled;not null" json:"cat_sticker_enabled"`
```
Note: `ChatPresence` uses `gorm:"column:...;not null"` without `default:` on all boolean columns so Go `false` is never silently overridden by SQL defaults on GORM `Create`.

**Settings struct & `msgpack` tags** (`xenobot/db/presence/repository.go` lines 55–86):
```go
type Settings struct {
	ConsensusEnabled   bool   `msgpack:"ce"`
	ConsensusThreshold int    `msgpack:"ct"`
	RetractEnabled     bool   `msgpack:"re"`
	AIReactMode        string `msgpack:"ar"`
	CatStickerEnabled  bool   `msgpack:"cs"`
	ArchiveEnabled     bool   `msgpack:"ae"`
	ArchiveChatID      int64  `msgpack:"ac"`
	ArchiveThreadID    int64  `msgpack:"at"`
	ArchiveThreshold   int    `msgpack:"ah"`
	ArchiveEmojiMode   string `msgpack:"am"`
	ArchiveDelivery    string `msgpack:"ad"`
}
```
Add three distinct short `msgpack` tags (e.g., `AmbientContext bool` `msgpack:"ax"`, `VocativeEnabled bool` `msgpack:"ve"`, `PMBriefReact bool` `msgpack:"pb"`). Cached values written before Phase 34 lack those keys and cleanly decode to `false` (`Defaults()`).

**Column-targeted upsert & `skipLocal` cache drop** (`xenobot/db/presence/repository.go` lines 236–241, 443–469):
```go
// SetCatStickerEnabled writes cat_sticker_enabled with one column-targeted upsert and drops the cache key.
func SetCatStickerEnabled(ctx context.Context, chatID int64, enabled bool, by int64) error {
	row := defaultRow(chatID, by)
	row.CatStickerEnabled = enabled
	return upsertColumns(ctx, row, []string{"cat_sticker_enabled"})
}
```
Note: `CacheKey(chatID)` uses `cache.CacheKey("chat_presence", chatID)` (`repository.go:158-160`), which is already registered in `skipLocal` (`xenobot/db/cache/local.go:63`).

**Backup transaction parity** (`xenobot/db/presence/repository.go` lines 376–406):
Add the 3 new fields to `row := &models.ChatPresence{...}`, `portableCols := []string{...}`, `defaultRow(...)`, and `GetFresh(...)` (`repository.go:187-199`).

---

### 3. Backup Parity (`xenobot/db/backup/types.go`, `xenobot/db/backup/backup.go`, `presence_backup_test.go`)

**Analog:** `xenobot/db/backup/types.go` (lines 236–251) & `xenobot/db/backup/backup.go` (lines 1427–1483)

**Portable DTO & JSON tags** (`xenobot/db/backup/types.go` lines 239–249):
```go
type PresencePortableBackup struct {
	ConsensusEnabled   bool   `json:"consensus_enabled"`
	ConsensusThreshold int    `json:"consensus_threshold"`
	RetractEnabled     bool   `json:"retract_enabled"`
	AIReactMode        string `json:"ai_react_mode"`
	CatStickerEnabled  bool   `json:"cat_sticker_enabled"`
	ArchiveEnabled     bool   `json:"archive_enabled"`
	ArchiveThreshold   int    `json:"archive_threshold"`
	ArchiveEmojiMode   string `json:"archive_emoji_mode"`
	ArchiveDelivery    string `json:"archive_delivery"`
}
```
Add `AmbientContext`, `VocativeEnabled`, and `PMBriefReact` with `json:"ambient_context"`, `json:"vocative_enabled"`, `json:"pm_brief_react"` (`omitempty` is NOT used on booleans here so `false` round-trips explicitly, while older JSON backups without these fields unmarshal cleanly to `false`).

**Export / Import wiring** (`xenobot/db/backup/backup.go` lines 1435–1478):
Wire the 3 fields in both `exportPresenceData` and `importPresence`. Keep `chat_conversations` permanently excluded from backup (`TestChatConversationsStayOutOfBackup`).

**Test analog** (`xenobot/db/backup/presence_backup_test.go` lines 47–92):
Extend `TestBackupPresenceArchiveStripped` to assert export/import roundtrip of `AmbientContext`, `VocativeEnabled`, and `PMBriefReact`, plus default `false` on legacy JSON payloads missing those keys.

---

### 4. Leaf Prompt & Vocative Helpers (`xenobot/utils/chatprompt/`)

**Analog:** `xenobot/utils/chatprompt/chatprompt.go` (lines 101–150, 243–273) & `xenobot/utils/chatprompt/reaction.go` (lines 108–137)

**Leaf package invariant** (`xenobot/utils/chatprompt/leaf_test.go` lines 17–44):
```go
// TestChatpromptPackageIsLeaf keeps the package free of xenobot/config and xenobot/db,
// whose init functions connect to PostgreSQL before main: it may import the
// standard library and the xenopersona leaf only.
```
Any pure helper placed in `xenobot/utils/chatprompt/` may ONLY import standard library packages (`strings`, `unicode`, `unicode/utf8`, etc.) and `github.com/uasneppy/Xenobot-72/xenobot/utils/xenopersona`.

**Ambient context section framing** (`xenobot/utils/chatprompt/chatprompt.go` lines 103–150 & `reaction.go` lines 112–137):
```go
func UserWithFiles(history []Turn, quoted *Turn, message Turn, files []File) string {
	var b strings.Builder
	if len(history) > 0 {
		b.WriteString("[Recent conversation, oldest first]\n")
		for _, t := range history {
			b.WriteString(line(t, MaxQuotedRunes))
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
```
Add `UserWithAmbient(ambient, history []Turn, quoted *Turn, message Turn, files []File) string` (having `UserWithFiles` delegate to `UserWithAmbient(nil, history, quoted, message, files)` so existing callers stay unchanged). Place the ambient block *before* `[Recent conversation, oldest first]` under a clear untrusted-data label such as `[Recent room chatter in this topic, oldest first]` using `line(t, MaxQuotedRunes)` so every line is sanitized (`Clean` + `Name` + `"name: text"` single-line rendering prevents prompt/section forging).

**Pure Unicode vocative prefix parser** (new function in `xenobot/utils/chatprompt/`):
Model on `StripMention` (`chatprompt.go:247-273`) using pure rune/case-folding prefix matching (no Go RE2 `\b`, which fails on Cyrillic!).
- Accept leading name variants case-insensitively (e.g., `"Xenobot-72"`, `"Xenobot"`, `"Кзенобот"`, `"Ксенобот"`, plus optional bot `FirstName` if distinct/non-empty), matching longest prefix first so `"Xenobot-72"` beats `"Xenobot"`.
- Require immediate delimiter after the name: end-of-string OR punctuation/whitespace rune (`unicode.IsSpace(r)` or `','`, `':'`, `'!'`, `'?'`, `'.'`, `'-'`, `'–'`, `'—'`). Never match mid-word (e.g. `"Xenobots"` or `"Xenobot2"`) or mid-sentence (`"Who is Xenobot?"`).
- Strip the matched name + trailing delimiter/whitespace and return `(stripped string, matched bool)`. Note: if the caller sends `"Xenobot,"` with nothing after, `chatJobOf` handles empty text via `chatMediaLabel` or fallback greeting handling (verify against `chatJobOf` at `chat_watch.go:197-215`).

---

### 5. Timestamped & Thread-Isolated Message Log (`xenobot/modules/chat_log.go`, `chat_log_test.go`)

**Analog:** `xenobot/modules/chat_log.go` (lines 48–58, 105–177, 221–235)

**Entry struct & `msg.Date` timestamp** (`chat_log.go` lines 48–58, 221–235):
```go
type chatLogEntry struct {
	MessageID int64  `json:"m"`
	UserID    int64  `json:"u"`
	Name      string `json:"n"`
	Text      string `json:"t,omitempty"`
	Media     string `json:"k,omitempty"`
	Thread    int64  `json:"th,omitempty"`
	Edited    bool   `json:"e,omitempty"`
	At        int64  `json:"at,omitempty"`
}
```
In `chatLogEntryOf(msg *gotgbot.Message, userID int64)` (`chat_log.go:221-235`), set `At: msg.Date` (NEVER `time.Now().Unix()`).

**Preserving original `At` on edits** (`chat_log.go` lines 107–110):
Currently:
```go
func chatLogEdit(client *redis.Client, chatID int64, entry chatLogEntry) (bool, error) {
	entry.Edited = true
	return chatLogWrite(client, chatLogEditScript, chatID, entry)
}
```
In Telegram Bot API, `EditedMessage.Date` remains the original creation Unix timestamp while `EditDate` is the edit time. For defense-in-depth (and tests calling `chatLogEdit` with `At: 0` or a different timestamp), read the existing entry at score `entry.MessageID` (`ZRangeArgs` with `Start`/`Stop` = `entry.MessageID`, `ByScore: true`) before `chatLogWrite` and preserve `existing.At` if `existing.At > 0` (falling back to `entry.At` if `existing.At == 0`).

**Existing thread-isolated recent reader** (`chat_log.go` lines 148–177):
```go
func chatLogRecentThread(ctx context.Context, client *redis.Client, chatID, threadID, beforeMsgID int64, limit int) ([]chatLogEntry, error)
```
Note: `chatLogRecentThread` caps its reverse scan at `chatLogRecentScanLimit` (`50`), which is ideal for `CHAT-02` (ambient context of 6–10 entries) and Phase 32 AI reactions (`5` entries).
For `CHAT-04` (`/unrollshort 50..200`, `me`, `2h`), add thread-isolated helpers in `chat_log.go` following the exact same signature and `rCtx, cancel := context.WithTimeout(ctx, chatLogReadTimeout)` + `entry.Thread != threadID` guard:
- `chatLogRecent(ctx, client, chatID, threadID, beforeMsgID int64, limit int) ([]chatLogEntry, error)`: reverse scan over the ring (`ZRevRange` `0 .. -1` or bounded by `chatLogMax`) filtering `entry.Thread == threadID && (beforeMsgID <= 0 || entry.MessageID < beforeMsgID)`, returning up to `limit` entries ordered oldest-first.
- `chatLogSinceUser(ctx, client, chatID, threadID, userID, beforeMsgID int64) ([]chatLogEntry, error)`: reverse scan finding the newest entry where `entry.Thread == threadID && entry.UserID == userID && (beforeMsgID <= 0 || entry.MessageID < beforeMsgID)`, then collecting all same-thread entries from that message ID onward (oldest-first).
- `chatLogSinceTime(ctx, client, chatID, threadID, cutoffUnix, beforeMsgID int64) ([]chatLogEntry, error)`: reverse scan keeping entries where `entry.Thread == threadID && entry.At >= cutoffUnix && (beforeMsgID <= 0 || entry.MessageID < beforeMsgID)`; treat legacy `entry.At == 0` gracefully (skip `entry.At == 0` so pre-Phase-34 entries are never falsely pulled into a recent time window, and note that entries with `0 < entry.At < cutoffUnix` stop or are excluded — scan safely across out-of-order/mixed `At == 0` entries or break once `entry.At > 0 && entry.At < cutoffUnix`).
- `chatLogEntryAt(ctx, client, chatID, messageID int64) (chatLogEntry, bool, error)` (or `chatLogEntryOfID`): reads the single entry at score `messageID` (`ZRangeArgs` `ByScore: true`) so `✍` PM brief (`CHAT-05`) resolves `entry.Thread` and presence in one Redis read!

---

### 6. Ambient Room Context & Deduplication (`xenobot/modules/chat_history.go`, `chat_prompt.go`, `chat_run.go`)

**Analog:** `xenobot/modules/chat_history.go` (lines 49–66), `xenobot/modules/chat_prompt.go` (lines 248–259), `xenobot/modules/chat_run.go` (lines 285–301), and `xenobot/modules/chat_react.go` (lines 387–416)

**Preserving MessageIDs from `chatReadHistory`** (`chat_history.go` lines 49–66):
Currently `chatReadHistory` discards `entry.MessageID`:
```go
	for i := len(raw) - 1; i >= 0; i-- {
		var entry chatHistoryEntry
		if json.Unmarshal([]byte(raw[i]), &entry) != nil {
			continue
		}
		turns = append(turns, chatprompt.Turn{Name: entry.Name, Text: entry.Text, Bot: entry.Role == "b"})
	}
```
Add `chatReadHistoryWithIDs(ctx, client, chatID) ([]chatprompt.Turn, map[int64]bool)` (having `chatReadHistory` call it so existing callers stay unchanged). Also store `quotedMessageID int64` on `chatJob` in `chatJobOf` (`chat_watch.go:222`) when `reply != nil`.

**Double-gated ambient lookup & deduplication** (`chat_run.go` lines 285–301 & `chat_react.go` lines 387–416):
In `chatProcessJob(job chatJob)` (`chat_run.go:286`):
1. Read `conv, err := chatSettingsLookup(ctx, job.chatID)` and `pres, err := presenceSettingsGet(ctx, job.chatID)` (both `skipLocal` Redis reads; fail silently with `nil` ambient turns on error).
2. Double-gate: proceed ONLY if `err == nil && conv.Enabled && conv.LogEnabled && pres.AmbientContext && !presenceIsStaffGroup(job.chatID)`.
3. Call `chatLogRecentThread(ctx, client, job.chatID, job.threadID, job.messageID, chatAmbientContextLimit)` (with `chatAmbientContextLimit` in 6–10, e.g. `8`).
4. Filter out any entry whose `entry.MessageID == job.messageID`, `entry.MessageID == job.quotedMessageID`, or `historyIDs[entry.MessageID]` is true!
5. Convert remaining entries into `[]chatprompt.Turn` using the exact text/media formatting pattern from `chatReactRecentTurns` (`chat_react.go:400-414`).
6. Pass `ambient` into `chatBuildRequest(ctx, job, job.bot.Username, ambient, history, media)` (`chat_prompt.go:248`), calling `chatprompt.UserWithAmbient(ambient, history, job.quoted, job.turn, media.Files)`.

---

### 7. Vocative Name-Addressing (`xenobot/modules/chat_watch.go`, `chat_watch_test.go`)

**Analog:** `xenobot/modules/chat_watch.go` (lines 61–83, 127–140, 197–239)

**Mutually exclusive watcher branch** (`chat_watch.go` lines 74–83):
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
For `CHAT-03`:
1. Keep the fast zero-I/O checks first (`chat.Type`, `raidFloodIsSlashText`, `raidFloodSenderCounts`, `IsBot`).
2. Check `chatAddressed(b, msg)` (@mention or reply-to-bot-own-msg) first — if true, proceed via `chatAccept` -> `chatEnqueue` (never calls `chatReactWatch`).
3. Else check pure vocative gate: `msg.ForwardOrigin == nil` AND `stripped, isVocative := chatprompt.ParseVocativePrefix(chatMessageText(msg), b.FirstName)`.
4. If `isVocative`:
   - Check `!presenceIsStaffGroup(chat.Id)` (skip Staff Groups!).
   - Read `pres, err := presenceSettingsGet(lookupCtx, chat.Id)` -> if `err == nil && pres.VocativeEnabled`:
     - Run `gate, provider := chatAccept(b, chat.Id, userID)`.
     - If `gate == chatGateOpen`: build `job := chatJobOf(b, msg, userID, provider)` (overriding `job.turn.Text` with stripped text or media label), `chatEnqueue(job)`, and **return** (never calls `chatReactWatch`!).
     - Even if `gate != chatGateOpen` (e.g., `Audience` rejected or `conv.Enabled == false`), when `pres.VocativeEnabled` is on and the member vocatively addressed Xenobot, **return** without calling `chatReactWatch`? Wait: if `conv.Enabled == false`, `chatReactWatch` would skip anyway (`!conv.Enabled`), and if `Audience` blocked it, returning prevents falling back to AI reactions on a rejected vocative address!
   - If `!pres.VocativeEnabled` (or read error): fall through to `chatReactWatch(b, ctx)` (the group didn't opt into vocative addressing, so ambient AI reactions remain unaffected).

---

### 8. Reply-Free `/unrollshort` & `/unrolllong` + Silent `✍` PM Brief (`xenobot/modules/chat_brief.go`, `presence_reactions.go`)

**Analog:** `xenobot/modules/chat_brief.go` (lines 85–211, 357–458, 529–547) & `xenobot/modules/presence_reactions.go` (lines 182–214, 259–316) & `xenobot/utils/extraction/extraction.go` (lines 76–118)

**Reply-free argument parsing** (`chat_brief.go` lines 161–201):
- Extract `arg := chatCommandArgument(chatMessageText(msg))` (helper in `chat.go:126`).
- Parse tokens (`strings.Fields(arg)`):
  - Check optional trailing `"pm"` (case-insensitive) -> `toPM = true`.
  - If `reply := extraction.ReplyTarget(msg); reply != nil`:
    - Disallow conflicting positional anchor tokens (`len(remaining) > 0` -> reply `usageKey`).
    - Keep existing reply flow: `chatLogHas(ctx, client, chat.Id, reply.MessageId)` -> `fromID = reply.MessageId`.
  - If `reply == nil`:
    - Require exactly 1 positional token in `remaining` (0 or >1 -> reply `usageKey`):
      - `"me"` (case-insensitive): `mode = chatBriefModeMe`.
      - All-ASCII digits `N`: parse positive integer bounded by `chatBriefMinCount..chatBriefMaxCount` (e.g. `5..200`, out of range -> reply `usageKey`): `mode = chatBriefModeCount, count = N`.
      - `spec, matched, err := extraction.ParseDurationToken(strings.ToLower(tok))`: if `matched && err == nil && spec.Seconds >= chatBriefMinDurationSecs && spec.Seconds <= chatBriefMaxDurationSecs` (e.g., `60` or `600` s to `24*3600` s): `mode = chatBriefModeTime, duration = time.Duration(spec.Seconds)*time.Second`. Otherwise reply `usageKey`.
- Note on `chatBriefPrepare` / `chatBriefProcessJob`:
  - Check `gate := chatBriefGateOf(chat.Id)` as today.
  - Check `!presenceIsStaffGroup(chat.Id)`? Note: `unrollShort` / `unrollLong` already work in groups with `LogEnabled`; check `CHAT-05` `✍` specifically enforces `!presenceIsStaffGroup` + `!presenceIsLockedDown` + `Audience`.
  - Set `job.dest = chatconv.JudgeToPrivate` when `toPM == true`.
  - In `chatBriefProcessJob` (`chat_brief.go:394-404`): branch on `job.mode` (reply `chatLogSince` + `chatBriefStretch`, count `chatLogRecent`, me `chatLogSinceUser`, time `chatLogSinceTime`). When empty, answer `brief_too_old` (or `brief_empty` — unless `job.silentPM == true`, in which case return silently with zero group messages!).

**Silent `✍` PM delivery (`CHAT-05`)** (`chat_brief.go` lines 361–458, 533–547):
Extract a shared helper from `chatJudgeDeliverPrivate`:
```go
func chatDeliverPrivate(ctx context.Context, job chatBriefJob, client *redis.Client, messages []string, silent bool) {
	err := chatBriefSendAll(ctx, job, job.userID, 0, false, messages)
	switch {
	case err == nil:
		chatBriefAnsweredCount.Add(1)
		if !silent {
			chatBriefSay(ctx, job, "judge_note_pm")
		}
	case settingsEditIsForbidden(err):
		chatBriefFailedCount.Add(1)
		if !silent {
			chatBriefSay(ctx, job, "judge_pm_blocked")
		}
	default:
		chatBriefFailedCount.Add(1)
		log.WithFields(log.Fields{"chat_id": job.chatID, "user_id": job.userID}).Warnf("[Chat] the brief/verdict could not be sent in private: %s", chatSendErrClass(err))
		if !silent {
			chatSayGated(ctx, job.bot, client, job.asChat(), chatBriefSaidMalfunction, "brief_malfunction", chatMalfunctionGate)
		}
	}
}
```
**CRITICAL `silentPM` audit of `chatBriefProcessJob` & `chatBriefFinish` (`chat_brief.go:361-458`):**
When `job.silentPM == true`, **every** group-facing send must be skipped:
- Line 379 (`breaker.isPaused`): skip `chatSayGated`.
- Line 386 (`chatBudgetCapped`): skip `chatSayGated`.
- Line 393 (`chatSendTyping`): skip typing indicator in the group!
- Line 402 (`stretch == nil`): skip `chatBriefSay(ctx, job, "brief_too_old")`.
- Line 417 (`provider error`): skip `chatSayGated`.
- Line 431 (`chatRefused`): skip `chatBriefSay(ctx, job, "brief_refused")`.
- Line 441 (`len(parts) == 0`): skip `chatSayGated`.
- Note on `chatBriefFrame` (`chat_brief.go:474-480`): for `chatBriefShort` delivered to PM (`job.dest == chatconv.JudgeToPrivate && job.kind != chatBriefJudge`), consider adding the group title context (like `judge_header_other` or a localized PM brief header) or keeping `brief_header_short` — check `locales/en.yml` `brief_header_short`.

**`✍` reaction trigger in `presence_reactions.go`** (lines 200–214):
```go
	if settings.PMBriefReact && mru.User != nil && slices.Contains(added, "✍") {
		presenceHandlePMBrief(runCtx, b, mru)
	}
```
Inside `presenceHandlePMBrief(ctx, b, mru)`:
1. `if !chatAnyProviderConfigured()` -> return.
2. `conv, err := chatSettingsLookup(ctx, mru.Chat.Id)` -> require `err == nil && conv.LogEnabled && chatProviderConfigured(conv.Provider)`.
3. `if presenceIsLockedDown(mru.Chat.Id)` -> return (lazy fail-closed check!).
4. `if !chatAudienceAllows(ctx, b, conv.Audience, mru.Chat.Id, mru.User.Id)` -> return (respect `Audience`!).
5. Check per-user `✍` cooldown/dedup in Redis (outside `xenobot:cache:`, e.g. `presenceChatTag(chatID) + "pmbrief:cd:" + strconv.FormatInt(userID, 10)` or per-`(chatID, userID, msgID)` + user cooldown) via `SetNX` BEFORE `chatBriefEnqueue` so tapping `✍` repeatedly cannot drain `brief:b` budgets.
6. Read the logged entry at `mru.MessageId` (`chatLogEntryAt` via `ZRangeArgs` `ByScore: true`) — if absent or error, return silently! Use `entry.Thread` as `job.threadID` (because `MessageReactionUpdated` has no `MessageThreadId`).
7. Call `chatBriefEnqueue(chatBriefJob{bot: b, chatID: mru.Chat.Id, chatTitle: mru.Chat.Title, userID: mru.User.Id, messageID: mru.MessageId, threadID: entry.Thread, fromID: mru.MessageId, provider: conv.Provider, kind: chatBriefShort, dest: chatconv.JudgeToPrivate, silentPM: true})`!
   - Note: `handleReaction` already runs inside `chatJobs` under `StopChat` (`presence_reactions.go:168-179`), and `chatBriefEnqueue` joins `chatJobs` / `chatContext()` under `StopChat`, so **zero new shutdown drains** are added.

---

### 9. Settings UI & Double-Gating (`xenobot/modules/presence.go`, `settings_presence.go`, `locales/*.yml`, `settings_locale_test.go`)

**Analog:** `xenobot/modules/presence.go` (lines 158–261), `xenobot/modules/settings_presence.go` (lines 25–50, 162–225, 276–296, 418–563, 655–689), `xenobot/modules/settings_phase33_test.go` (lines 11–143)

**Apply layer** (`xenobot/modules/presence.go` lines 158–261):
Add `AmbientContext`, `VocativeEnabled`, `PMBriefReact` fields to `presenceWrites` and corresponding `applyPresenceAmbientContext`, `applyPresenceVocativeEnabled`, `applyPresencePMBriefReact` functions calling `presenceReadBack`.

**Settings UI layout & 8-row cap** (`xenobot/modules/settings_presence.go` lines 446–467):
Page `pr` (`renderSettingsPresence`) currently renders **7 rows** (with `TestSettingsPresencePageFitsEveryLocale` enforcing `rows <= 8` and `TestSettingsPresenceCallbackBytes` currently asserting `7` rows):
- Row 1: Consensus toggle
- Row 2: Consensus threshold stepper
- Row 3: Retract toggle
- Row 4: AI React `[Off | Low | Medium]`
- Row 5: Cat sticker toggle
- Row 6: Highlights Archive sub-page button (`a=ph`)
- Row 7: Back / Close (`k.navRow(false)`)

Adding a single sub-page button on `pr` (e.g., `settingsActPresenceRoom = "pr2"` or `"pc"`, opening a "Room & Catch-Up" sub-page just like `settingsActPresenceHighlights = "ph"` opens Highlights Archive!):
- Keeps `pr` at **8 rows** (strictly within `<= 8`!),
- Keeps `len(settingsPages) == 31` and `settingsMainOwnerRows == 17` untouched,
- Gives the sub-page a clean layout: 3 toggle rows (`ambient_context`, `vocative_enabled`, `pm_brief_react`) + Back (to `pr`) / Close nav row = 4 rows, with room for Owner-lock status notes and descriptive hints in all 8 locales.

**Owner×Admin double-gating on controls** (`xenobot/modules/settings_presence.go` lines 276–296, 655–689):
```go
	if mode != presence.AIReactModeOff {
		unlocked, err := settingsPresenceConvUnlocked(p.group.Id)
		if err != nil {
			log.WithField("chat_id", p.group.Id).Warn("[Settings] conversation lookup failed when enabling presence AI reactions")
			p.cannotOpen()
			return nil
		}
		if !unlocked {
			p.answer(settingsText(p.tr, "settings_pr_ai_need_owner"), true)
			return nil
		}
	}
```
Follow the exact same pattern:
- Enabling `vocative_enabled` (`v=1`) requires `conv.Enabled && chatProviderConfigured(conv.Provider)` (else alert `settings_pr_ai_need_owner`).
- Enabling `ambient_context` (`v=1`) requires `conv.Enabled && conv.LogEnabled && chatProviderConfigured(conv.Provider)` (else alert `settings_pr_ai_need_owner` or a dedicated `<= 200`-rune alert key).
- Enabling `pm_brief_react` (`v=1`) requires `conv.LogEnabled && chatProviderConfigured(conv.Provider)` (else alert `settings_pr_log_need_owner` or `settings_pr_ai_need_owner`).
- Disabling (`v=0`) ALWAYS succeeds for authorized admins (`settingsRightChangeInfo`), even when Conversations / Message log is off.

**Locale & Docs checks** (`xenobot/i18n/settings_locale_test.go` lines 934–1027, 1613–1642):
- Add every new `settings_*` key to `settingsKeyCeilings` in `settings_locale_test.go` across all 8 locales (`uk`, `en`, `es`, `fr`, `hi`, `id`, `pt`, `ru`).
- Add any new alert keys to `settingsAlertKeys` (must be plain text, `<= 200` runes).
- Update `chat_help_msg` / `brief_usage_short` / `brief_usage_long` if `/unrollshort`/`/unrolllong` syntax docs change, and run `make check-translations` and `make generate-docs` / `make check-docs` (see `Makefile:121-145`).

---

## Shared Patterns

### Staff Group & Lockdown Safety Gate (`PRES-02`)
**Source:** `xenobot/modules/presence.go` lines 101–147
**Apply to:** Vocative addressing (`chat_watch.go`), `✍` PM brief (`presence_reactions.go`), `ambient_context` (`chat_run.go`)
- Hot message paths use `presenceMessageAllowed(b, ctx)` / `presenceIsStaffGroup(chatID)` (`staff.GetStaffGroup`, `skipLocal` Redis cache).
- Lazy Postgres check `presenceIsLockedDown(chatID)` (`lockdown.GetActiveFresh`, fail-closed on error) runs **only** right before acting (e.g. inside `presenceHandlePMBrief` after confirming `✍` + `settings.PMBriefReact`).

### `StopChat` Shutdown Lifecycle (Zero New Drains)
**Source:** `xenobot/modules/chat_run.go` lines 94–109, 381–391 & `xenobot/modules/presence_reactions.go` lines 164–180
**Apply to:** All Phase 34 async/reaction flows (`chatQueue`, `chatBriefQueue`, `handleReaction`)
- Every reaction handler (`handleReaction`) and queue enqueue (`chatEnqueue`, `chatBriefEnqueue`) joins `chatJobs`, checks `chatDraining.Load()`, and derives its context from `chatContext()`.
- `main.go` and the 70 s webhook shutdown sum (`stop_grace_period: 75s`) remain completely unchanged.

### Test Seams & `-race` Discipline (`AGENTS.md`, `xenobot/modules/chat_env_test.go`)
**Source:** `xenobot/modules/chat_env_test.go`, `xenobot/modules/presence_reactions_test.go`, `xenobot/modules/settings_presence_test.go`
**Apply to:** All new tests in `xenobot/modules/`
- Keep `xenobot/modules` unit tests fast (millisecond pure tests in `xenobot/utils/chatprompt/` and `chat_brief_prompt_test.go`, minimal DB/miniredis matrix tests in `modules`).
- Never call `t.Parallel()` in tests that mutate package-level test seams (`chatSettingsLookup`, `presenceSettingsGet`, `chatProviderOf`, `presenceStaffLookup`, `presenceLockdownLookup`, `chatLogMax`); always restore via `chatSwap` / `t.Cleanup`.

---

## Metadata

**Analog search scope:** `migrations/`, `xenobot/db/models/`, `xenobot/db/presence/`, `xenobot/db/backup/`, `xenobot/db/cache/`, `xenobot/utils/chatprompt/`, `xenobot/utils/extraction/`, `xenobot/modules/`, `xenobot/i18n/`, `locales/`, `Makefile`
**Files scanned:** 32 (all verified git-tracked via `git ls-files`)
**Pattern extraction date:** 2026-11-27
