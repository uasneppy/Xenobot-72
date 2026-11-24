# Phase 27: Manual Command Defaults, Reason Shortcuts and Appeal Button - Pattern Map

**Mapped:** 2026-11-22
**Files analyzed:** 23 (new & modified across DB, cache, manual commands, event messages, settings pages, backup, locales, goldens & safeguards)
**Analogs found:** 23 / 23

---

## Executive Inventory: Free Codes, Namespaces, Migration Format & Guard Traps

### 1. Settings Page Registry — Used vs Free Keys, Orders & Icons

25 of 26 single-letter page keys (`a`..`z`) are already registered in production `init()`. **Only `q` is free as a 1-letter key.** Two-letter page keys (e.g. `md`, `rs`, or `q` + `rs`) are fully supported by `settingsPage.Key string` and fit well under the 64-byte callback cap (`settings_list.go` Yes button does not even carry `p`; Open button `settings|v1|a=op&c=-1001234567890&p=rs` is 36 bytes). Note: Phases 28 (Advanced), 29 (Copy settings), and 30 (Backup) will also need keys.

| Order | Key | Icon | TitleKey | PrivateOnly | Hidden in Staff | File |
|------:|-----|------|----------|-------------|-----------------|------|
| 10 | `s` | 👥 | `settings_page_staff` | no | no | `fuku/modules/settings_staff.go` |
| 11 | `v` | 🚷 | `settings_page_fwdblock` | yes | outside Staff | `fuku/modules/settings_fwdblock.go` |
| 20 | `l` | 🔒 | `settings_page_lockdown` | no | no | `fuku/modules/settings_lockdown.go` |
| 30 | `c` | 🧩 | `settings_page_captcha` | no | no | `fuku/modules/settings_captcha.go` |
| 40 | `f` | 🌊 | `settings_page_antiflood` | no | no | `fuku/modules/settings_antiflood.go` |
| 42 | `n` | 🚩 | `settings_page_warns` | no | no | `fuku/modules/settings_warns.go` |
| 43 | `j` | 🪜 | `settings_page_esc` | yes | yes | `fuku/modules/settings_escalation.go` |
| 44 | `k` | 🔐 | `settings_page_locks` | no | no | `fuku/modules/settings_locks.go` |
| 46 | `b` | 🔤 | `settings_page_blacklist` | yes | no | `fuku/modules/settings_blacklist.go` |
| 47 | `i` | 🔁 | `settings_page_filters` | yes | no | `fuku/modules/settings_filters.go` |
| 50 | `w` | 🎉 | `settings_page_welcome` | no | no | `fuku/modules/settings_edit.go` |
| 60 | `g` | 👋 | `settings_page_goodbye` | no | no | `fuku/modules/settings_edit.go` |
| 70 | `r` | 📜 | `settings_page_rules` | no | no | `fuku/modules/settings_edit.go` |
| 72 | `e` | 📝 | `settings_page_notes` | yes | no | `fuku/modules/settings_notes.go` |
| 73 | `x` | 👍 | `settings_page_reactions` | yes | no | `fuku/modules/settings_reactions.go` |
| 74 | `h` | 📨 | `settings_page_msgs` | yes | yes | `fuku/modules/settings_messages.go` |
| 80 | `m` | 🛂 | `settings_page_mreq` | no | no | `fuku/modules/settings_mreq.go` |
| 85 | `d` | 🧹 | `settings_page_del` | no | yes | `fuku/modules/settings_deletion.go` |
| 86 | `t` | 🔘 | `settings_page_switches` | no | no | `fuku/modules/settings_switches.go` |
| 87 | `u` | 🌐 | `settings_page_lang` | no | no | `fuku/modules/settings_language.go` |
| 87 | `z` | 🕒 | `settings_page_tz` | no | no | `fuku/modules/settings_timezone.go` |
| 88 | `o` | 🧾 | `settings_page_log` | no | no | `fuku/modules/settings_logchannel.go` |
| 89 | `y` | 🎭 | `settings_page_anon` | no | no | `fuku/modules/settings_anonadmin.go` |
| 89 | `p` | 🗨 | `settings_page_chat` | yes (OwnerOnly) | yes | `fuku/modules/settings_chat.go` |
| 90 | `a` | 🔑 | `settings_page_access` | no (OwnerOnly) | no | `fuku/modules/settings_access.go` |

**Suggested Phase 27 placement (zero collisions with existing pages or test helpers):**
- **Manual defaults / Moderation page:** Key `"q"` (only free 1-letter key, or `"md"`), Order `41` or `45` (adjacent to Warns `42` / Escalation `43` / Locks `44`), Icon `"⚖️"` or `"🔨"` (unused on Main; `🔨` is only used inside Messages hub for Manual family). `PrivateOnly: true`, `Hidden: <StaffGroup>`.
- **Reason shortcuts list page:** Key `"rs"` (2-letter key; avoid `"qt"` used by `settings_edit_resolved_test.go:200`), Order `45` or `75` (next to Manual defaults or Messages `74`), Icon `"🏷"` or `"💬"` (unused on Main). `PrivateOnly: true`, `Hidden: <StaffGroup>`.

### 2. Action Codes (`a` field in `settings|v1`) — Used vs Free

**Core action codes (reserved in `settings_pages.go:430-433`):** `oh`, `op`, `bk`, `cl`, `tg`, `st`, `no`, `pk`

**Production page action codes currently registered (81 codes):**
`ay`, `bc`, `bd`, `bn`, `bo`, `bp`, `bs`, `bx`, `by`, `bz`, `dl`, `dr`, `dy`, `ea`, `ec`, `eg`, `ex`, `ey`, `fd`, `ft`, `gc`, `gf`, `gr`, `gs`, `gv`, `hc`, `he`, `hf`, `hl`, `ho`, `hp`, `hq`, `hr`, `hs`, `hv`, `ia`, `ic`, `ig`, `ix`, `iy`, `ja`, `jd`, `je`, `jf`, `jr`, `js`, `jt`, `kg`, `ls`, `ly`, `mo`, `my`, `nd`, `ne`, `nt`, `nx`, `py`, `rc`, `rf`, `rr`, `rs`, `rv`, `va`, `vc`, `ve`, `vg`, `vs`, `vx`, `vy`, `wc`, `wf`, `wr`, `ws`, `wv`, `xa`, `xc`, `xg`, `xx`, `xy`, `zk`, `zo`

**Test-only action codes to avoid (registered temporarily inside tests — avoid to prevent collision panics):**
- `settings_edit_resolved_test.go:194`: `qs`, `qr`, `qc`, `qv`, `qf` (with test page key `qt`)
- `settings_list_env_test.go:250-254`: `zg`, `zx`, `zy`, `za`, `zc`

**Clean free action-code families for Phase 27:**
- For Manual Defaults page (`q`): `qd` (open duration screen for mute/ban), `qt` (set preset duration or clear/none), `qr`? **NO, avoid `qr` (`settings_edit_resolved_test.go`)!** Use `q`-codes outside `{qc, qf, qr, qs, qv}`, e.g. `qd` (open duration), `qn` (set duration / none), `qq` (toggle reason-required), `qa` (prompt appeal link), `qk` (cancel appeal wait), `qx` (remove appeal link) — OR `m`-prefix codes outside `{mo, my}`, e.g. `md` (duration screen), `mt` (set duration), `mr` (toggle reason-required), `ma` (set appeal link), `mc` (cancel appeal wait), `mx` (remove appeal link).
- For Reason Shortcuts list page (`rs` or `k`-free prefix `c` / `s` / `u` — none of `cg, cx, cy, ca, cc` or `ug, ux, uy, ua, uc` or `sg, sx, sy, sa, sc` are used!):
  - Completely unused 5-code list family: e.g. `sg`, `sx`, `sy`, `sa`, `sc` (Pager, Ask, Yes, Add, Cancel) or `cg`, `cx`, `cy`, `ca`, `cc` or `ug`, `ux`, `uy`, `ua`, `uc`. Zero overlap with any prod or test code!

### 3. Latest Migrations & Naming Convention

Latest files in `migrations/`:
- `20261120110000_add_chat_event_messages.sql` (Phase 21)
- `20261120120000_add_event_message_options.sql` (Phase 22)
- `20261122120000_add_timed_punishment_durations.sql` (Phase 26)
- `20261122130000_add_warn_expiry.sql` (Phase 26)
- `20261122140000_add_escalation_ladder.sql` (Phase 26)

Next migration timestamp must sort strictly after `20261122140000` (e.g. `20261122150000_add_manual_moderation_defaults.sql` and/or `20261122160000_add_reason_shortcuts.sql`, or one combined migration `20261122150000_add_manual_command_tuning.sql`).
All DDL must be idempotent (`CREATE TABLE IF NOT EXISTS ...`), carry inline `CONSTRAINT` clauses (both Go runner and `scripts/migrate_psql.sh` run it in one transaction), have no foreign key on `chats`, and stay out of backup/export/import/reset.

### 4. Critical Codebase Traps Discovered During Mapping

1. **`moderation.go` / `bans.go` / `mute.go` execution order for `/tban` and `/tmute`:** `extractTemporalTarget` currently runs **inside `execute`** (`bans.go:110`, `mute.go:114`), AFTER `extract` and `validate`. Therefore `t.reason` still contains `"2h spam"` during `extract` and `validate`, and only becomes `"spam"` after `extractTemporalTarget` runs! Reason-required refusal and reason-shortcut expansion for `/tban`/`/tmute` MUST run **after** duration extraction and **before** `BanMember`/`RestrictMember`.
2. **`extractUserOnly` and `extractFromReply` discard reason text:** `/sban` (`bans.go:341`), `/smute` (`mute.go:168`), `/skick` (`bans.go:967`) use `extractUserOnly`; `/dkick` (`bans.go:88`) uses `extractFromReply` which returns `target{userID: uid}` without reading any text after `/dkick`. Record provisional owner decisions explicitly for: (a) which commands `reason_required` covers (e.g., `/ban`, `/dban`, `/tban`, `/mute`, `/dmute`, `/tmute`, `/kick`, `/dkick`, `/warn`, `/dwarn` — and whether `/sban`, `/smute`, `/skick`, `/swarn` are covered or exempt, and whether channel `/ban` is covered), and (b) whether `/sban`, `/dban`, `/smute`, `/dmute` also inherit the default ban/mute duration.
3. **`settings_input_door.go:199-200` hardcodes `"settings_tz_wait_ended_note"` for ALL `settingsTypedPages`!** Adding a second `settingsTypedPages` entry (for typing the appeal link) requires adding a `WaitEndedKey string` field to `settingsTypedPage` (`settings_input.go:297-301`) with fallback to `"settings_tz_wait_ended_note"` when empty, so an expired appeal-link wait does not tell the user their timezone wait ended.
4. **`settings_list_input.go:113-117, 460-472` assumes `TwoStep` step 2 is media/buttons via `content.ExtractTypedNoteOrFilter`:** Filters and Notes accept media/buttons and show `settings_entries_wait_content_limits` (4096 text / 1024 caption). Reason Shortcuts step 2 is **plain text only**. Extending `settingsListTwoStep` with a plain-text path (e.g. `PlainStepTwo` or `ExtractPlain` + `StepTwoLimitsKey`) cleanly reuses the entire atomic `settingsInputAdvance`, `StepMsgID` redelivery guard (`r.msg.MessageId <= r.wait.StepMsgID`), fresh re-check, claim/finish/release, and `sendSavedList`!
5. **`settings_main_test.go` hardcodes Main grid shape and title keys:** `settingsMainShapeGroup`, `settingsMainShapePrivate`, `settingsMainOwnerRows` (`settings_main_test.go:36-38`) and `settingsMainTitleKeys` (`settings_main_test.go:573-578`) assert the exact Main grid. Registering new settings pages requires updating those constants in the same commit.
6. **`fuku/modules/test_harness_test.go:388` and `fuku/db/backup/testmain_test.go:85` auto-migrate every model for SQLite test runs:** Every new GORM model MUST be added to both lists and to its own package's `testmain_test.go` (`testdb.Run(m, ...)`).
7. **EVT-10 structural guards (`TestEventEngineCallers`, `TestPMCopyCallers`):** Do NOT add calls to `postEvent`, `replyEvent`, `composeEventMessage`, `composeEventPM`, or `offerEventPM` in any new file outside the allow-lists without updating `event_engine_callers_test.go:140-177` and `pm_copy_callers_test.go:27-53`. Keep appeal-button attachment inside `event_message.go` / `event_message_pm.go` (engine's own files, exempt from caller allow-list) and `settings_messages.go` previews!
8. **MSG-11 golden preservation:** `TestLegacyModerationRequests` (`moderation_legacy_request_test.go:162-212`) pins exact JSON of `/ban`, `/dban`, `/tban`, `/kick`, `/dkick`, `/mute`, `/dmute`, `/tmute`, `/unban`, `/unmute`, `/sban`, `/skick`, `/smute` with nothing stored. When no default duration, no reason-required, no shortcuts, and no appeal link are stored, every single request MUST remain byte-identical!

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `migrations/20261122150000_add_manual_command_tuning.sql` (new) | migration | DDL | `migrations/20261122140000_add_escalation_ladder.sql` + `migrations/20261113130000_add_staff_forward_blocks.sql` | exact |
| `fuku/db/models/modtuning.go` (or `manual_defaults.go`) (new) | model | CRUD | `fuku/db/models/escalation.go` + `fuku/db/models/punish.go` | exact |
| `fuku/db/modtuning/repository.go` (new) | repository | CRUD + skipLocal cache | `fuku/db/escalation/repository.go` + `fuku/db/reactions/repository.go` | exact |
| `fuku/db/cache/local.go` (modified) | cache config | prefix match | `fuku/db/cache/local.go` (`chatEscalationPrefix`) | exact |
| `fuku/db/cache/local_test.go` (modified) | test | unit | `fuku/db/cache/local_test.go` (`TestSkipLocalChatEscalationKey`) | exact |
| `fuku/db/backup/modtuning_backup_test.go` (new) + `testmain_test.go` (modified) | test | backup exclusion | `fuku/db/backup/escalation_backup_test.go` | exact |
| `fuku/modules/modtuning_settings.go` (new) | service/apply | CRUD + validation | `fuku/modules/escalation_settings.go` + `fuku/modules/event_message_apply.go` | exact |
| `fuku/modules/bans.go` (modified) | controller/handler | request-response | `fuku/modules/bans.go` + `fuku/modules/punish_timed.go` | exact |
| `fuku/modules/mute.go` (modified) | controller/handler | request-response | `fuku/modules/mute.go` + `fuku/modules/punish_timed.go` | exact |
| `fuku/modules/warns.go` (modified) | controller/handler | request-response | `fuku/modules/warns.go` (`dispatchWarn`) | exact |
| `fuku/modules/moderation.go` (modified) | pipeline/utility | transform + guard | `fuku/modules/moderation.go` (`moderationCommand.run`) | exact |
| `fuku/modules/event_message.go` (modified) | renderer/service | transform + send | `fuku/modules/event_message.go` (`composeEventMessage`, `sendDefaultPost`) | exact |
| `fuku/modules/event_message_pm.go` (modified) | renderer/service | async send | `fuku/modules/event_message_pm.go` (`composeEventPM`, `runEventPM`) | exact |
| `fuku/modules/settings_messages.go` (modified) | settings page | preview + button reserve | `fuku/modules/settings_messages.go` (`settingsMsgManualEvents`, `settingsMsgPreview`, `settingsMsgPMPreview`) | exact |
| `fuku/modules/settings_input.go` & `settings_input_door.go` (modified) | middleware/door | event-driven (Redis wait) | `fuku/modules/settings_input.go` (`settingsTypedPage`) & `settings_input_door.go` (`waitEnded`) | exact |
| `fuku/modules/settings_list_input.go` (modified) | framework/engine | two-step typed input | `fuku/modules/settings_list_input.go` (`settingsListTwoStep`, `listStepTwo`) | exact |
| `fuku/modules/settings_moddefaults.go` (new) | settings page | request-response + typed wait | `fuku/modules/settings_escalation.go` + `fuku/modules/settings_timezone.go` + `fuku/modules/settings_duration.go` | exact |
| `fuku/modules/settings_reasons.go` (new) | settings list page | CRUD + two-step typed add | `fuku/modules/settings_filters.go` + `fuku/modules/settings_reactions.go` | exact |
| `fuku/modules/moderation_phase27_legacy_request_test.go` (new — Wave 1 golden!) | test | characterization golden | `fuku/modules/moderation_legacy_request_test.go` | exact |
| `fuku/modules/settings_main_test.go` & `test_harness_test.go` (modified) | test | harness + Main grid | `fuku/modules/settings_main_test.go` | exact |
| `fuku/modules/settings_list_locale_test.go` (modified) | test | locale guard | `fuku/modules/settings_list_locale_test.go` | exact |
| `fuku/modules/settings_phase27_test.go` & `*_screens_test.go` (new) | test | safeguards (bytes, 8-lang, privacy, reads) | `fuku/modules/settings_phase26_test.go` + `fuku/modules/settings_escalation_screens_test.go` + `fuku/modules/settings_staff_privacy_test.go` | exact |
| `locales/{uk,en,es,fr,hi,id,pt,ru}.yml` (modified) | locale | i18n | `locales/uk.yml`, `locales/en.yml` + `fuku/i18n/locale_style_test.go` | exact |

---

## Pattern Assignments

### 1. Migration & GORM Models (`migrations/20261122150000_*.sql`, `fuku/db/models/*.go`)

**Analogs:**
- Per-chat settings row: `migrations/20261122140000_add_escalation_ladder.sql` (lines 21-44) & `fuku/db/models/escalation.go` (lines 10-31)
- Per-chat list table (for reason shortcuts): `migrations/20261113130000_add_staff_forward_blocks.sql` (lines 22-31)

**SQL pattern (`migrations/20261122140000_add_escalation_ladder.sql:21-44`):**
```sql
CREATE TABLE IF NOT EXISTS chat_escalation (
    chat_id BIGINT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    step1_action VARCHAR(8) NOT NULL DEFAULT '',
    step1_secs INTEGER NOT NULL DEFAULT 0,
    ...
    updated_by BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT pk_chat_escalation PRIMARY KEY (chat_id),
    CONSTRAINT chk_chat_escalation_step1 CHECK (step1_action IN ('', 'mute', 'tmute', 'kick', 'tban', 'ban') AND step1_secs BETWEEN 0 AND 31622400)
);
```
- **Bounds for default durations:** reuse `models.PunishMinSecs` (`60`) and `models.PunishMaxSecs` (`31622400` = `366 * 24 * 60 * 60`) from `fuku/db/models/punish.go:8-19`:
  - `0` means "no default (permanent, today's behavior)"; non-zero must be `BETWEEN 60 AND 31622400` (CHECK `default_mute_secs = 0 OR default_mute_secs BETWEEN 60 AND 31622400`).
  - `reason_required BOOLEAN NOT NULL DEFAULT FALSE` (GORM struct field carries NO `default:` tag so Go `false` is always written!).
  - `appeal_url TEXT NOT NULL DEFAULT ''` (or `VARCHAR(2048)` — validated in Go by `content.ValidButtonURL`, cleared to `''`).

**List table SQL pattern (`migrations/20261113130000_add_staff_forward_blocks.sql:22-31`):**
```sql
CREATE TABLE IF NOT EXISTS staff_forward_blocks (
    id BIGSERIAL PRIMARY KEY,
    staff_chat_id BIGINT NOT NULL,
    source_chat_id BIGINT NOT NULL,
    source_title TEXT NOT NULL DEFAULT '',
    created_by BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT uk_staff_forward_blocks_source UNIQUE (staff_chat_id, source_chat_id),
    CONSTRAINT chk_staff_forward_blocks_source CHECK (source_chat_id < 0)
);
```
- For reason shortcuts: `id BIGSERIAL PRIMARY KEY`, `chat_id BIGINT NOT NULL`, `word VARCHAR(100) NOT NULL`, `reason TEXT NOT NULL`, `created_by BIGINT NOT NULL DEFAULT 0`, `created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()`, `CONSTRAINT uk_chat_reason_shortcuts_word UNIQUE (chat_id, word)`.
- **GORM model rule (`fuku/db/models/escalation.go:8-26`):** Never put `default:` tags on `bool` columns so GORM writes Go `false`; include `check:` tags on constrained columns so SQLite test runs enforce the same checks; implement `TableName() string`.

---

### 2. Repository & Cache (`fuku/db/<pkg>/repository.go`, `fuku/db/cache/local.go`)

**Analogs:**
- Settings row + column-targeted upsert + `skipLocal`: `fuku/db/escalation/repository.go` (lines 92-128, 228-247)
- List CRUD for v1.1 list engine (`List*Fresh`, `Get*Fresh`, `Count*Fresh`, `Remove*ByID`, `Add*` with `ON CONFLICT DO NOTHING`): `fuku/db/reactions/repository.go` (lines 90-164)

**Key patterns to copy:**
1. **Cache key & `skipLocal` (`fuku/db/cache/local.go:44-110`):**
   Every key read on a manual punishment or ban reply MUST be in `skipLocal` with a trailing colon so changes take effect on the very next command on every replica:
   ```go
   // fuku/db/cache/local.go:57, 108-109
   chatEscalationPrefix = CacheKey("chat_escalation") + ":"
   // inside skipLocal(key string) bool:
       strings.HasPrefix(key, chatEscalationPrefix)
   ```
   Add `TestSkipLocal<Name>Key` in `fuku/db/cache/local_test.go` (copy lines 321-330: assert `skipLocal(CacheKey(prefix, int64(-100123))) == true` and `skipLocal(CacheKey(prefix+"x", int64(1))) == false`).
2. **Column-targeted upsert (`fuku/db/escalation/repository.go:234-247`):**
   ```go
   err := db.DB.WithContext(ctx).Clauses(clause.OnConflict{
       Columns:   []clause.Column{{Name: "chat_id"}},
       DoUpdates: clause.AssignmentColumns(append(columns, "updated_by", "updated_at")),
   }).Create(row).Error
   if err != nil {
       return fukuerrors.Wrapf(err, "write ... of chat %d", row.ChatId)
   }
   cache.DeleteCache(CacheKey(row.ChatId))
   ```
   Two admins editing different columns (e.g. default mute duration vs appeal URL) never clobber each other!
3. **Non-creating reads (`fuku/db/escalation/repository.go:99-128`):**
   `Get(ctx, chatID)` uses `cache.GetFromCacheOrLoad(ctx, CacheKey(chatID), cache.DefaultCacheTTL, func(ctx) { return GetFresh(ctx, chatID) })`. `GetFresh` returns default zero-struct (no error, creates NO row) on `gorm.ErrRecordNotFound`, and returns wrapped errors on DB failure (never silently defaults on DB error so callers can fail-safe / log).
4. **List engine repository methods (`fuku/db/reactions/repository.go:90-164`):**
   - `GetShortcuts(ctx, chatID) (map[string]string, error)` — cached in `skipLocal` (or loaded together with settings under one `skipLocal` key if desired, OR its own `skipLocal` key dropped on add/remove).
   - `ListEntriesFresh(ctx, chatID)` — `Select("id", "word", "reason").Where("chat_id = ?", chatID).Order("id").Find(&rows)`
   - `GetEntryFresh(ctx, chatID, id)` — `Where("id = ? AND chat_id = ?", id, chatID).Take(&row)` (`gorm.ErrRecordNotFound` -> `nil, nil`, so foreign-chat IDs read as absent!)
   - `CountFresh(ctx, chatID)` — `Model(...).Where("chat_id = ?", chatID).Count(&n)`
   - `ExistsFresh(ctx, chatID, word)` — `Model(...).Where("chat_id = ? AND word = ?", chatID, word).Count(&n)`
   - `AddShortcut(ctx, chatID, word, reason, by)` — `clause.OnConflict{Columns: [chat_id, word], DoNothing: true}`, check `result.RowsAffected == 1`, drop cache only when added (no overwrite on race!).
   - `RemoveByID(ctx, chatID, id)` — `Clauses(clause.Returning{}).Where("id = ? AND chat_id = ?", id, chatID).Delete(&rows)`, drop cache only when `len(rows) == 1`.
5. **Backup exclusion test (`fuku/db/backup/escalation_backup_test.go:18-74`):**
   Add `fuku/db/backup/<name>_backup_test.go` proving `ExportChatData`, `ImportChatData`, and `ClearChatData` neither include nor touch the new tables, and register the new models in `fuku/db/backup/testmain_test.go` and `fuku/modules/test_harness_test.go`.

---

### 3. Manual Moderation Commands: Default Duration, Reason Required, Reason Shortcut Expansion

**Files to modify:**
- `fuku/modules/moderation.go` (lines 124-139, 144-225, 245-323, 471-480)
- `fuku/modules/bans.go` (lines 84-260, 278-290, 337-391, 963-979)
- `fuku/modules/mute.go` (lines 61-78, 107-217)
- `fuku/modules/warns.go` (lines 572-627)

**Existing exact structure & line references:**

1. **Moderation pipeline (`fuku/modules/moderation.go:245-323`):**
   ```go
   type target struct {
       userID    int64
       reason    string
       timeVal   string // used by time-based commands (e.g., tban)
       isChannel bool   // true if original target was a channel ID
       untilDate int64  // the Telegram until_date a time-based command computed; 0 for a permanent one
   }
   ```
   Pipeline order in `moderationCommand.run` (`moderation.go:275-323`):
   `buildModerationCtx` -> `gates` -> `extract` -> `validate` -> `execute` -> `reply` -> `actionlog.Admin(mc.Bot, mc.Chat, mc.User, cmd.logAction, tgt.userID, tgt.reason)`.

2. **How `/ban`, `/dban`, `/sban`, `/tban` currently work (`fuku/modules/bans.go`):**
   - `moderationBan` (lines 197-260): calls `extraction.ExtractUserAndText` directly (allowing channel IDs). For `t.isChannel`: calls `BanChatSenderChat` (no `UntilDate` in Telegram API) and `replySuccess` (NOT `banReplyWithButton`). For user targets: calls `c.Chat.BanMember(c.Bot, t.userID, nil)` and `banReplyWithButton` (`EventBan`).
   - `moderationDban` (lines 356-391): `extractFromArgs` -> validates reply & mismatch & `banTargetValidation` -> deletes replied message -> `c.Chat.BanMember(c.Bot, t.userID, nil)` -> `banReplyWithButton`.
   - `moderationSban` (lines 337-354): `extractUserOnly` (no reason extracted today!) -> `banTargetValidation` -> `c.Chat.BanMember(c.Bot, t.userID, nil)` -> deletes command message -> `reply: nil`.
   - `moderationTban` (lines 103-141): `extractFromArgs` -> `banTargetValidation` -> inside `execute`: `_time := extractTemporalTarget(c, t)` -> `c.Chat.BanMember(c.Bot, t.userID, &gotgbot.BanChatMemberOpts{UntilDate: _time})` -> `reply` sends `EventTimedBan` (`tbanDefaultReply`, `Values: {Reason, Duration: t.timeVal, Until: eventUntil(t.untilDate)}`).

3. **How `/mute`, `/dmute`, `/smute`, `/tmute` currently work (`fuku/modules/mute.go`):**
   - `moderationMute` (lines 149-162): `extractFromArgs` -> `muteTargetValidation` -> `c.Chat.RestrictMember(c.Bot, t.userID, MutedPermissions, nil)` -> `muteReplyWithButton` (`EventMute`).
   - `moderationDmute` (lines 182-217): `extractFromArgs` -> validates reply & mismatch & `muteTargetValidation` -> deletes replied message -> `RestrictMember(..., nil)` -> `muteReplyWithButton`.
   - `moderationSmute` (lines 164-180): `extractUserOnly` -> `muteTargetValidation` -> `RestrictMember(..., nil)` -> deletes command message.
   - `moderationTmute` (lines 107-147): `extractFromArgs` -> `muteTargetValidation` -> inside `execute`: `_time := extractTemporalTarget(c, t)` -> `RestrictMember(..., &gotgbot.RestrictChatMemberOpts{UntilDate: _time})` -> `reply` sends `EventTimedMute` (`tmuteDefaultReply`).

4. **How `/kick`, `/dkick`, `/skick` currently work (`fuku/modules/bans.go`):**
   - `moderationKick` (lines 278-290): `extractFromArgs` -> `kickTargetValidation` -> `kickMember` -> `kickReply` (`EventKick`).
   - `moderationDkick` (lines 84-101): `extractFromReply` (note: does NOT extract reason text today!) -> `kickTargetValidation` -> deletes reply -> `kickMember` -> `kickReply`.
   - `moderationSkick` (lines 963-979): `extractUserOnly` -> `kickTargetValidation` -> `kickMember` -> deletes command -> `reply: nil`.

5. **How `/warn`, `/dwarn`, `/swarn` currently work (`fuku/modules/warns.go:572-627`):**
   - `dispatchWarn(b, ctx, warnType)` builds `mc`, runs `standardModGates(mc)`, calls `userId, reason := extraction.ExtractUserAndText(b, ctx)`, validates target is in chat and `dwarn` reply match, then calls `m.warnThisUser(b, ctx, userId, reason, warnType, warnSourceManual)`.
   - **Important:** `warnThisUser` is ALSO called by automatic rules (`blacklists.go`, `staff_forwardblock_watch.go`) with `warnSourceAuto`! Therefore reason-required and reason-shortcut expansion MUST live in `dispatchWarn` (manual commands only, NEVER inside `warnThisUser` for `warnSourceAuto`!).

6. **How timed punishment computation works (`fuku/modules/punish_timed.go:150-192`):**
   ```go
   // punish_timed.go:152-160
   func punishUntil(now time.Time, secs int) (int64, bool) {
       return extraction.TemporaryUntilDate(now.Unix(), int64(secs))
   }
   func punishUntilTooClose(until int64) bool {
       return until-punishNow().Unix() < int64(punishMinLead/time.Second)
   }
   ```
   - `punishDurationValid(secs)` checks `60 <= secs <= 31622400`.
   - `punishDurationLabel(tr, secs)` formats in group language (`"1 hour(s)"`, `"2 год"`, etc.).
   - `punishDurationShort(secs)` formats short grammar (`"1h"`, `"30m"`, `"7d"`).
   - For default ban/mute duration on `/ban`, `/mute` (and `/dban`, `/dmute`, `/sban`, `/smute` per provisional decision):
     - If no explicit duration was typed and `defaultSecs > 0` (and `punishDurationValid(defaultSecs)`):
       - Compute `until, ok := punishUntil(punishNow(), defaultSecs)` and check `!punishUntilTooClose(until)`.
       - Pass `&gotgbot.BanChatMemberOpts{UntilDate: until}` / `&gotgbot.RestrictChatMemberOpts{UntilDate: until}`.
       - Populate `t.untilDate = until` and `t.timeVal = punishDurationLabel(c.Tr, defaultSecs)` (or `extraction`-style wording), and send the timed event (`EventTimedBan` / `EventTimedMute`) or `EventBan` / `EventMute` with duration+until (planner picks and documents; `EventTimedBan`/`EventTimedMute` naturally shows `{duration}` and `{until}` in both default and custom texts and PM copies, and `EventTimedBan` also gets the appeal button!).
     - If an admin passes a duration token to `/ban @user 2h spam` or `/mute @user 1h spam`? **Check carefully:** Today `/ban @user 2h spam` treats `"2h spam"` as the reason (permanent ban), while `/tban @user 2h spam` parses `2h` as duration! SC1 states: *"With a default duration set, `/mute` and `/ban` given without a duration apply it. An explicit duration is unchanged. With no default, they behave as today."*
       - Note BOTH paths:
         1. `/tban @user 2h spam` and `/tmute @user 2h spam` (explicit duration) stay unchanged.
         2. `/tban @user [reason]` and `/tmute @user [reason]` (if called without a duration when default is set, or `/ban @user 2h spam` if `/ban` optionally accepts a leading duration token when default is set? Be careful: do NOT break existing `/ban @user 2h spam` when default is NOT set — pin this in Wave 1 goldens!).
         3. Cleanest, zero-regression rule:
            - When default duration is 0 (unset): `/ban`, `/mute`, `/tban`, `/tmute` behave 100% identically to today (goldens pass byte for byte).
            - When default duration is set (> 0): `/ban` and `/mute` (no duration argument) apply the group's default duration; `/tban` and `/tmute` with an explicit duration apply the explicit duration (and if `/ban`/`/mute` optionally checks a leading `extraction.ParseDurationToken` OR `/tban`/`/tmute` falls back to default when no duration is given, document clearly as a provisional decision!).

7. **Reason-required refusal & Reason-shortcut expansion:**
   - **Order of operations on a manual punishment:**
     1. Permission gates (`standardModGates` / `deleteModGates`)
     2. Target extraction (`ExtractUserAndText`)
     3. Target validation (`validateTarget`, reply mismatch, in-chat check) — refuse bad targets BEFORE complaining about a missing reason (or check reason right after target validation, BEFORE `execute` and BEFORE deleting any replied message in `/dban`/`/dmute`/`/dkick`/`/dwarn`!).
     4. For `/tban`/`/tmute`: parse duration first (`extractTemporalTarget`) so `t.reason` is the reason WITHOUT the duration token!
     5. **Reason-required check:** if `settings.ReasonRequired && strings.TrimSpace(reason) == ""` -> reply with short refusal via `replyValidationError` (`c.Msg.Reply(c.Bot, text, formatting.Shtml())`, NOT `replySuccess` so it is NOT queued under `bot:punish_manual`!) and return `ext.EndGroups` so NOTHING happens (no ban/mute/kick/warn, no message delete, no PM copy, no log post).
     6. **Reason-shortcut expansion:** if `reason != ""`, look up normalized word (e.g., `strings.ToLower(strings.TrimSpace(reason))`, matching exact single-word shortcut key). If found in chat's shortcuts map, replace `reason` with the stored full text. If not found (or multi-word not matching a shortcut), keep `reason` as typed.
        - **Escaping guarantee:** Store raw plain text in DB. Every downstream consumer ALREADY escapes it:
          - `moderationReasonLine` (`moderation.go:46`): `formatting.HtmlEscape(reason)`
          - `eventPlaceholderValues` (`event_message.go:191`): `html.EscapeString(v.Reason)`
          - `eventPMDefaultText` (`event_message_pm.go:436`): `html.EscapeString(values.Reason)`
          - `warnDefaultReply` (`event_message_auto.go:36`): `html.EscapeString(in.Reason)`
          - `warnLimitDefaultReply` (`event_message_auto.go:80`): `html.EscapeString(reason)`
          - `actionlog.Admin` (`actionlog.go:60`): `html.EscapeString(reason)`
          - `/warns` (`warns.go:727`): sends with `opts = nil` (plain text, no parse mode).
   - **Fail-open vs fail-closed on DB read error for manual tuning:** Follow `event_message.go:390-400` ("the punishment comes first" / never break moderation on a DB hiccup): if reading manual tuning fails, log one `Warn` with chat ID only (never user text) and proceed with defaults (no default duration, reason not required, no shortcut expansion).
   - **Staff Group isolation (EVT-10):** `staff_action_parse.go` and `staff_action*.go` are completely separate from `bans.go`, `mute.go`, `warns.go`, `moderation.go` and NEVER read manual command defaults, reason shortcuts, or appeal links. Add a test proving Staff Group `/ban` / `/mute` / `/tban` / `/tmute` ignore per-group defaults, reason-required, shortcuts, and appeal links.

---

### 4. Appeal Button on Ban Messages & PM Copies (`event_message.go`, `event_message_pm.go`, `settings_messages.go`)

**Analogs & exact integration points:**
- URL check: `content.ValidButtonURL(raw string) bool` (`fuku/utils/content/content.go:270-273`):
  ```go
  func ValidButtonURL(raw string) bool {
      u, err := url.Parse(raw)
      return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
  }
  ```
- Group message button assembly (`fuku/modules/event_message.go:309-324`):
  ```go
  kept := r.KeepRows
  if kept == nil {
      kept = eventBotRows(r.Opts)
  }
  custom := keyboard.BuildKeyboard(buttons)
  var rows [][]gotgbot.InlineKeyboardButton
  if r.KeepRowsLast {
      rows = append(append(rows, custom...), kept...)
  } else {
      rows = append(append(rows, kept...), custom...)
  }
  if len(rows) > 0 {
      msg.Keyboard = &gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows}
  }
  ```
- Default group message send (`fuku/modules/event_message.go:486-493`):
  ```go
  func sendDefaultPost(site eventSite, r eventReply) (eventGroupPost, error) {
      sent, err := site.SendDefault(r.Default, r.Opts)
      if err != nil {
          return eventGroupPost{}, err
      }
      site.schedule(sent)
      return eventGroupPost{Msg: sent, Rows: eventBotRows(r.Opts)}, nil
  }
  ```
- PM copy assembly (`fuku/modules/event_message_pm.go:385-405`):
  ```go
  if rows := keyboard.BuildKeyboard(buttons); len(rows) > 0 {
      msg.Keyboard = &gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows}
  }
  ```
- PM copy Details button cap check (`fuku/modules/event_message_pm_details.go:146-152, 174-177`):
  ```go
  func pmCopyButtonCount(rows [][]gotgbot.InlineKeyboardButton) int
  // checks: pmCopyButtonCount(o.GroupRows)+1 > pmCopyMaxButtons (100)
  ```

**Exact requirements & edge cases for Appeal Button (TUNE-06):**
1. **Which events carry the appeal button:**
   - Definitely `eventmsg.EventBan` (`/ban`, `/dban`) and `eventmsg.EventTimedBan` (`/tban`, and `/ban` when default duration is active).
   - Check/document provisional decision on automatic ban events: `eventmsg.EventReqBan` and automatic ban outcomes of `EventWarnLimit`, `EventFlood`, `EventBlacklist`, `EventForwardBlock`, `EventCaptcha` (when `Action == "ban" || Action == "tban"`). Simplest clean hook: a boolean field on `eventReply` (e.g. `Appeal bool`) or helper `eventIsBan(event, action)` checked inside `event_message.go` + `event_message_pm.go`!
   - NEVER on channel bans (`t.isChannel` doesn't go through `replyEvent`), NEVER on Staff Group actions, NEVER on lockdown joiner bans or "Ban N recent joiners" (EVT-10).
2. **Position & Telegram 100-button limit:**
   - Added as a single-button row `[]gotgbot.InlineKeyboardButton{{Text: label, Url: appealURL}}` **after** any custom buttons (and for `EventBan`, after Unban + custom buttons).
   - Counts toward Telegram's 100-button limit (`settingsEditMaxButtons = 100`):
     - At send time: only append if `pmCopyButtonCount(rows)+1 <= settingsEditMaxButtons` (never push a keyboard over 100 buttons and trigger a 400!).
     - In `settings_messages.go`: bump `ReservedButtons` on `EventBan` (from `1` to `2`: Unban + potential Appeal) and `EventTimedBan` (from `0` to `1`: potential Appeal), and on their PM definitions (`1`: potential Appeal), OR check dynamically — reserving statically in `settings_messages.go` guarantees an owner can never save 100 custom buttons on a ban message and later be surprised when setting an appeal link! (Verify `TestSettingsEditReservedButtons` — it uses its own synthetic `event` struct, not `settingsMsgManualEvents`, so bumping `ReservedButtons` won't break it.)
3. **Default ban message with appeal link set vs unset:**
   - When NO appeal link is set (`appealURL == ""`): `r.Opts` is passed UNTOUCHED to `site.SendDefault`, preserving byte-identical JSON for `TestLegacyModerationRequests`!
   - When an appeal link IS set (`appealURL != ""` and `content.ValidButtonURL(appealURL)`):
     - In `sendDefaultPost`: shallow-copy `r.Opts` (never mutate caller's pointer), clone `eventBotRows(r.Opts)`, append the appeal row (if `pmCopyButtonCount(rows)+1 <= settingsEditMaxButtons`), set `opts.ReplyMarkup = gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows}`, and return those rows in `eventGroupPost.Rows` so `pmCopyAddDetails` preserves the appeal button if it later appends the Details button!
     - In `composeEventMessage`: append the appeal row after `custom` (respecting 100-button limit).
     - In `composeEventPM`: append the appeal row after `keyboard.BuildKeyboard(buttons)` (respecting 100-button limit).
     - In `runEventPM` fallback (`event_message_pm.go:349`): if a custom PM copy is refused and falls back to `o.Fallback`, attach the appeal row to the fallback text message too (if appeal URL set) so a refused custom PM copy still carries the appeal button!
     - In `settingsMsgPreview` & `settingsMsgPMPreview` (`settings_messages.go:987-1072`): pass the group's stored appeal link into preview composition so Full preview shows the appeal button when set!
4. **No extra DB read per ban if stored cleanly:** Either store `appeal_url` in `chat_event_message_options` (already read on every `postEventSent` inside `eventmsg.Get` — ZERO extra DB/Redis round trips!) OR in the new `chat_manual_defaults` table (cached in `skipLocal`, read only for ban events). Storing `appeal_url` in the manual-defaults table (or alongside `chat_event_message_options`) both keep `skipLocal` consistency; check backup tests (`event_messages_backup_test.go` vs new `modtuning_backup_test.go`) when choosing!

---

### 5. Private Settings Page: Manual Command Defaults + Appeal Link (`fuku/modules/settings_moddefaults.go`)

**Analogs:**
- Private-only + Hidden in Staff Group + apply functions + duration screen: `fuku/modules/settings_escalation.go` (lines 15-171, 403-435) & `fuku/modules/escalation_settings.go` (lines 18-187)
- Shared duration presets screen: `fuku/modules/settings_duration.go` (lines 16-110)
- Single-value typed input via `settingsTypedPages` (for Appeal Link): `fuku/modules/settings_timezone.go` (lines 89-95, 228-332)

**Imports & registration pattern (`fuku/modules/settings_escalation.go:47-106`):**
```go
private := func(check func(map[string]string) bool) func(map[string]string) bool {
    return func(fields map[string]string) bool { return fields["c"] != "" && check(fields) }
}
registerSettingsPage(settingsPage{
    Key:            settingsPageModDefaults, // e.g. "q"
    Order:          41, // or 45
    Icon:           "⚖️",
    TitleKey:       "settings_page_moddefaults",
    MainState:      settingsModDefaultsMainState,
    Open:           settingsModDefaultsOpen,
    Hidden:         settingsModDefaultsHidden, // fresh staff.GetStaffGroupFresh
    PrivateOnly:    true,
    PrivateNoteKey: "settings_moddef_group_note",
    Actions:        map[string]settingsPageAction{ ... },
})
```
**Right to use:**
- Viewing / opening sub-screens (duration screen, Cancel): `settingsRightAdmin` (with live admin + owner gate already enforced by `settingsPress.verify`).
- Changing default mute/ban duration, reason-required toggle, or appeal link: `settingsRightRestrict` (matches Escalation page for punishment tuning; OR `settingsRightChangeInfo` for appeal link — using `settingsRightRestrict` across the Manual Defaults page or matching `settingsRightChangeInfo` for appeal link / shortcuts should be consistent and documented in the plan and `AGENTS.md`).

**Duration picker pattern (`fuku/modules/settings_duration.go:54-85`):**
- Reuse `settingsDurationPresets` (`600, 3600, 21600, 43200, 86400, 259200, 604800, 2592000` — 10m, 1h, 6h, 12h, 1d, 3d, 7d, 30d) + an "Off / Permanent" (`v="0"`) choice button!
- Note `renderSettingsDuration` (`settings_duration.go:54`) draws only the 8 presets and mentions "set by command" in `settings_duration_hint` — for default `/mute` and `/ban` durations (which have NO command, only the private page, plus an "Off / Permanent" choice), either add a small helper or render a clean screen reusing `settingsDurationPresets`, `settingsDurationPreset`, and `punishDurationLabel` with an explicit Off/Permanent button (`v="0"`).

**Appeal link typed-reply pattern (`fuku/modules/settings_timezone.go:89-95, 249-332`):**
```go
// Register in init():
settingsTypedPages[settingsPageModDefaults] = settingsTypedPage{
    Part:         settingsModDefAppealPart, // e.g. "a"
    CancelCode:   settingsActModDefAppealCancel,
    Right:        settingsRightRestrict, // or settingsRightChangeInfo
    Typed:        settingsModDefAppealTyped,
    WaitEndedKey: "settings_moddef_appeal_wait_ended", // NEW field on settingsTypedPage!
}
```
And in `fuku/modules/settings_input_door.go:199-201`:
```go
} else if typed, ok := settingsTypedPages[page]; ok && typed.Part == part {
    key := typed.WaitEndedKey
    if key == "" {
        key = "settings_tz_wait_ended_note"
    }
    r.plain(r.text(key))
```
Inside `settingsModDefAppealTyped(r *settingsInputReply)`:
1. Require plain text (`r.msg.Text != ""`), trim whitespace (`strings.TrimSpace(r.msg.Text)`).
2. Validate URL with `content.ValidButtonURL(raw)` (`http` or `https`, non-empty host) and sensible max length (e.g. `<= 2048` bytes / runes) — on refusal, call `r.refuse(r.text(...))` (keeps wait!).
3. `claim, claimed := r.claimWait()`, write via `applyAppealURL(...)`, on save failure `r.releaseWait(claim)` + `r.refuse(...)`, on success `r.finishWait(claim)`, log chat/user IDs ONLY (NEVER log the URL!), send updated page as a new message, `r.closePrompt()`.
4. Provide a "Remove appeal link" button on the page when a link is currently set (clears to `""`, no confirm card needed or simple button like Editor Remove).
5. Display the stored appeal link safely on the page: either show a safe status ("Set" / masked host or `html.EscapeString` truncated) and NEVER put user text in callback data or log lines!

---

### 6. Private Settings List Page: Reason Shortcuts (`fuku/modules/settings_reasons.go`, `fuku/modules/settings_list_input.go`)

**Analogs:**
- Two-step list table: `fuku/modules/settings_filters.go` (lines 14-188)
- `Detail` display (`1. <code>word</code> → full text`): `fuku/modules/settings_reactions.go` (lines 42-45) & `fuku/modules/settings_list.go` (lines 59-61, 344-353)
- Two-step wait flow & static part table: `fuku/modules/settings_list_input.go` (lines 38-46, 104-132, 341-504)

**Exact list-engine hooks (`fuku/modules/settings_filters.go:144-188`):**
```go
func settingsReasonsDef() settingsListDef {
    return settingsListDef{
        Key:          settingsPageReasons, // e.g. "rs"
        Order:        45,
        Icon:         "🏷",
        TitleKey:     "settings_page_reasons",
        PagerCode:    "sg",
        AskCode:      "sx",
        YesCode:      "sy",
        Right:        settingsRightRestrict, // or settingsRightChangeInfo
        CountMax:     reasonShortcutsCapMax, // e.g. 50 or 100
        Hidden:       settingsReasonsHidden, // hidden in Staff Group!
        Remove:       settingsReasonsRemove,
        DeleteAskKey: "settings_rsn_delete_ask",
        RemovedKey:   "settings_rsn_removed",
        Read:         ..., // sets Entry{ID, Text: row.Word, Detail: row.Reason}
        Entry:        ...,
        Count:        ...,
        EntryParam:   "word",
        IntroKey:     "settings_rsn_intro",
        CountKey:     "settings_rsn_count",
        EmptyKey:     "settings_rsn_empty",
        AddCode:      "sa",
        CancelCode:   "sc",
        AddKey:       "settings_btn_rsn_add",
        Prompt:       settingsReasonsPrompt,
        AddGate:      settingsReasonsAddGate,
        TwoStep:      &settingsListTwoStep{ ... },
    }
}
```

**Static part table addition (`fuku/modules/settings_list_input.go:38-46`):**
```go
{Page: settingsPageReasons, Code: "w", Step: 1, Next: "r", WordKey: "settings_entries_part_reason_word"},
{Page: settingsPageReasons, Code: "r", Step: 2, WordKey: "settings_entries_part_reason_text"},
```
(Remember `settings_list_locale_test.go` automatically checks every `part.WordKey` in `settingsListParts` across all 8 languages!)

**Extending `settingsListTwoStep` for plain-text Step 2 (`fuku/modules/settings_list_input.go:104-132, 341-348, 460-504`):**
Today `renderSettingsListPrompt` unconditionally appends `settings_entries_wait_content_limits` (text/caption limits) on step 2, and `listStepTwo` unconditionally calls `content.ExtractTypedNoteOrFilter`.
Clean minimal extension (zero change to Filters or Notes):
- Add optional `ApplyPlain func(ctx context.Context, chatID int64, norm, text string, tr *i18n.Translator) settingsListApplyResult` and `CheckPlain func(text string, tr *i18n.Translator) string` (or `PlainText bool` + `PlainMaxRunes int`) on `settingsListTwoStep`:
  - In `renderSettingsListPrompt`: if `def.TwoStep.ApplyPlain == nil`, append `settings_entries_wait_content_limits` as today; otherwise let `def.Prompt(v, part)` provide the step-2 prompt & length rule.
  - In `listStepTwo`: if `ts.ApplyPlain != nil`:
    1. Keep the `if r.msg.MessageId <= r.wait.StepMsgID { return }` redelivery guard!
    2. Require `r.msg.Text != ""` (else `r.refuse(r.text("settings_edit_refuse_need_text"))`), trim whitespace, validate non-blank & length (e.g. `<= 200` or `<= 500` runes, matching safe Telegram reply budgets) via `ts.CheckPlain` (on refusal `r.refuse(...)`, wait kept!).
    3. Re-run `r.listStepCheck(ts, norm)` against fresh DB read (catches concurrent duplicate/cap before claim!).
    4. Claim wait -> call `ts.ApplyPlain(...)` -> finish/release/sendSavedList/closePrompt identically to Filters/Notes!

**Word normalization rules for Step 1:**
- Single word only (no whitespace), lower-cased (`strings.ToLower(strings.TrimSpace(text))`), bounded length (e.g. 1..32 runes, `<= listKeywordMaxRunes`), no leading `/`, no duplicate (refused, never overwritten, matching Filters/Notes).
- Note `settingsListLine` (`settings_list.go:344-353`) automatically renders `1. <code>word</code> → escaped_full_text_truncated_32` when `e.Detail = row.Reason` is populated!

---

## Shared Patterns

### 1. Golden / Characterization Tests First (MSG-11)
**Source:** `fuku/modules/moderation_legacy_request_test.go` (lines 21-212)
**Apply to:** Wave 1 commit (ALONE before any production edits)
- Pin exact `sendMessage` / `banChatMember` / `restrictChatMember` / `unbanChatMember` / `deleteMessage` requests for:
  - `/ban`, `/dban`, `/sban`, `/tban` (including `/tban` missing duration refusal `extraction_no_time_specified` and invalid duration refusal `extraction_invalid_time_type`)
  - `/mute`, `/dmute`, `/smute`, `/tmute` (including `/tmute` missing/invalid duration refusals)
  - `/kick`, `/dkick`, `/skick`
  - `/warn`, `/dwarn`, `/swarn` (missing reason allowed today -> warn succeeds with no reason line)
- Never edit or weaken existing `TestLegacyModerationRequests` or `TestLegacyWarnSettingsRequests`.

### 2. IDs-Only Logging & Privacy (No User Text / URLs / Staff Group Leaks)
**Source:** `fuku/modules/event_message_lockdown_safety_test.go` (lines 103-178) & `fuku/modules/settings_staff_privacy_test.go` (lines 1963-2036)
**Apply to:** All new repository, apply, command, and settings page files
- Log lines carry `chatID`, `userID`, action/event name, and error class ONLY — never a shortcut word, shortcut full reason, typed reason, or appeal URL.
- Staff Group privacy: new pages are `PrivateOnly: true` and `Hidden` in Staff Groups; add a Phase 27 run in `settings_staff_privacy_test.go` (or `settings_phase27_test.go`) proving no output names a linked Staff Group.

### 3. Settings Safeguards (64-Byte Callbacks, 8-Language Fit, Step-1 Validation, Non-Creating Reads)
**Source:** `fuku/modules/settings_phase26_test.go` (lines 25-168) & `fuku/modules/settings_escalation_screens_test.go` (lines 46-113)
**Apply to:** Both new settings pages
- `TestPhase27Codes`: every action code belongs to its page, no core code, extra key refused in step 1 with 0 `getChatMember` calls, group menu (`u` instead of `c`) refused in step 1 with 0 `getChatMember` calls.
- `TestPhase27ReadsCreateNothing`: opening Main and every new screen on an unconfigured chat creates 0 rows in the new tables and 0 `chats` rows.
- Worst-case callback byte test (`<= 64` bytes, `Omitted == 0`, chat ID `-1001234567890`, 17-digit list row ID).
- 8-language screen length test (`1..3800` UTF-16 units with 64-rune titles; button labels `<= 40` full-width / `<= 20` multi-column).
- Table-driven locale key test (update `settingsListExtraKeys()` in `fuku/modules/settings_list_locale_test.go` + test any table-driven keys on the Manual Defaults page).

### 4. Locale Style & Translations
**Source:** `fuku/i18n/locale_style_test.go`, `docs/src/content/docs/contributing/locale-style.md`
**Apply to:** All 8 locale files (`locales/{uk,en,es,fr,hi,id,pt,ru}.yml`)
- Every new key in all 8 files; `uk` and `en` follow the locale style guide (`TestLocaleStyle`).
- `make check-translations`, `make check-docs`, `make test` (or sharded), and `make lint` must pass.
- Update `AGENTS.md` in the same commit as any new table, `skipLocal` key, settings page, callback action code, or rule change.

---

## No Analog Found

None — every component of Phase 27 has an exact, tracked analog in the existing codebase.

---

## Metadata

**Analog search scope:** `fuku/modules/`, `fuku/db/`, `fuku/utils/`, `migrations/`, `AGENTS.md`
**Files scanned:** 32 tracked source & test files (all verified via `git ls-files`)
**Pattern extraction date:** 2026-11-22
