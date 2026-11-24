# Phase 29: Plain Tone and Copy Settings - Pattern Map

**Mapped:** 2026-11-23
**Files analyzed:** 24 (10 new, 14 modified)
**Analogs found:** 24 / 24

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `migrations/20261123130000_add_event_message_plain_tone.sql` | migration | CRUD | `migrations/20261120120000_add_event_message_options.sql` | exact |
| `fuku/db/models/event_message.go` (modify) | model | CRUD | self (`ChatEventMessageOptions` lines 33-51) | exact |
| `fuku/db/eventmsg/repository.go` (modify) | repository | CRUD | self (`SetPMDetails`, `SetPMLog`, `upsertOptions` lines 193-235) | exact |
| `fuku/db/eventmsg/validate_write.go` (new) | service/repository | transform + CRUD | `fuku/modules/settings_edit_check.go` + `fuku/utils/content/content.go` | exact |
| `fuku/db/eventmsg/validate_write_test.go` (new) | test | CRUD | `fuku/db/eventmsg/repository_test.go` | exact |
| `fuku/db/antiflood/repository.go` (modify) | repository | CRUD | self (`SetFloodActionContext` lines 77-96) | exact |
| `fuku/db/escalation/repository.go` (modify) | repository | CRUD | self (`SetEnabledWithSteps`, `upsert` lines 182-247) | exact |
| `fuku/db/modtuning/repository.go` (modify) | repository | CRUD | self (`upsertDefaults`, `AddShortcut`, `ListShortcutsFresh`) | exact |
| `fuku/db/advthresholds/repository.go` (modify) | repository | CRUD | self (`upsertColumns`, `ResetAll` lines 408-473) | exact |
| `fuku/modules/event_message_plain.go` (new) | service | transform | `fuku/modules/event_message_auto.go` + `event_message_lockdown.go` | exact |
| `fuku/modules/event_message.go` (modify) | service | request-response | self (`postEventSent`, `sendDefaultPost`, `offerReplyPM` lines 437-613) | exact |
| `fuku/modules/event_message_pm.go` (modify) | service | event-driven | self (`eventPMSpec`, `eventPMDefaultText`, `composeEventPM` lines 40-101, 391-451) | exact |
| `fuku/modules/event_message_apply.go` (modify) | service | CRUD | self (`applyEventPMDetails`, `applyEventPMLog` lines 126-169) | exact |
| `fuku/modules/settings_messages.go` (modify) | component/controller | request-response | self (`settingsMsgPMOptions`, `renderSettingsMsgHub` lines 553-595, 1111-1146) | exact |
| `fuku/modules/copy_settings.go` (new) | service | CRUD + transform | `fuku/modules/modtuning_settings.go` + `fuku/modules/advthresholds.go` | exact |
| `fuku/modules/settings_copy.go` (new) | component/controller | request-response | `fuku/modules/settings_advthresholds.go` + `fuku/modules/settings_hub.go` + `fuku/modules/event_message_pm_details.go` | exact |
| `fuku/modules/event_message_plain_test.go` (new) | test | transform | `fuku/modules/event_message_test.go` + `event_message_pm_test.go` | exact |
| `fuku/modules/copy_settings_test.go` (new) | test | CRUD | `fuku/modules/settings_advthresholds_test.go` + `settings_moddefaults_test.go` | exact |
| `fuku/modules/settings_copy_test.go` (new) | test | request-response | `fuku/modules/settings_advthresholds_test.go` + `settings_hub_test.go` | exact |
| `fuku/modules/settings_phase29_test.go` (new) | test | request-response | `fuku/modules/settings_phase28_test.go` | exact |
| `locales/{en,es,fr,hi,id,pt,ru,uk}.yml` (modify) | config/i18n | — | `fuku/i18n/locale_style_test.go` + `fuku/i18n/locale_rewrite_guard_test.go` | exact |
| `fuku/db/backup/event_messages_backup_test.go` (modify) | test | CRUD | self (lines 46-104) | exact |
| `fuku/modules/settings_main_test.go`, `settings_fwdblock_test.go`, `settings_phase19_test.go` (modify) | test | — | Phase 28 commit `8b854206` | exact |
| `fuku/modules/settings_messages*_test.go` (modify 7 files) | test | — | self (hub row width `[2 2 1 1 2]` -> `[2 2 1 1 1 2]` & `settingsMsgValidPresses`) | exact |

---

## Part 1: Plain Tone (TUNE-08) — Complete Inventory & Architecture

### 1.1 Migration & Storage (`chat_event_message_options`)

**Next migration timestamp** (latest is `migrations/20261123120000_add_advanced_thresholds.sql`):
- `migrations/20261123130000_add_event_message_plain_tone.sql`

**Analog:** `migrations/20261120120000_add_event_message_options.sql` (lines 20-27)
```sql
ALTER TABLE chat_event_message_options
    ADD COLUMN IF NOT EXISTS plain_tone BOOLEAN NOT NULL DEFAULT FALSE;
```

**Why `chat_event_message_options` is the exact right table:**
1. Already loaded in the SAME `eventmsg.Get` / `eventmsg.GetFresh` read (`fuku/db/eventmsg/repository.go:135-141`) into `Messages.Options`. Zero extra DB or Redis queries on punishment hot paths!
2. Already cached under `event_messages:<chat>`, which is ALREADY listed in `skipLocal` (`fuku/db/cache/local.go:55, 121`). Toggling Plain tone takes effect at the very next event on every replica!
3. Uses column-targeted upsert (`upsertOptions`, `fuku/db/eventmsg/repository.go:220-235`), so `SetPlainTone` never clobbers `pm_details` or `pm_log`, and vice versa.
4. `models.ChatEventMessageOptions` carries NO `default:` GORM tag on booleans (`fuku/db/models/event_message.go:38-46`), so Go `false` is always written.
5. An earlier replica's cached msgpack value without `PlainTone` decodes `PlainTone` as `false` (safe zero-value default during rolling deploy).

**Model addition (`fuku/db/models/event_message.go:40-46`):**
```go
type ChatEventMessageOptions struct {
	ChatID    int64     `gorm:"column:chat_id;primaryKey;autoIncrement:false" json:"chat_id"`
	PMDetails bool      `gorm:"column:pm_details;not null" json:"pm_details"`
	PMLog     bool      `gorm:"column:pm_log;not null" json:"pm_log"`
	PlainTone bool      `gorm:"column:plain_tone;not null" json:"plain_tone"`
	UpdatedBy int64     `gorm:"column:updated_by;not null" json:"updated_by"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}
```

**Repository additions (`fuku/db/eventmsg/repository.go`):**
- Add `PlainTone bool` to `Options` struct (lines 43-48).
- Populate `PlainTone: options[0].PlainTone` in `GetFresh` (line 140).
- Add `SetPlainTone(ctx context.Context, chatID int64, on bool, by int64) error` copying `SetPMLog` (lines 208-215):
```go
func SetPlainTone(ctx context.Context, chatID int64, on bool, by int64) error {
	row := models.ChatEventMessageOptions{PlainTone: on}
	if err := upsertOptions(ctx, chatID, by, &row, "write the plain tone option", "plain_tone"); err != nil {
		return err
	}
	cache.DeleteCache(CacheKey(chatID))
	return nil
}
```
- Also update `fuku/db/backup/event_messages_backup_test.go` (lines 46-47, 80, 103) to set `PlainTone: true` and verify `"plain_tone"` stays out of backups until Phase 30.

### 1.2 Complete Event Inventory (All 23 Events of Milestone v1.2)

Source of truth: `eventmsg.Events` (`fuku/db/eventmsg/events.go:91-115`) — **23 events total** across 4 families; **18 events** have PM copy specs (`eventPMSpecs`, `fuku/modules/event_message_pm.go:71-101`); **5 lockdown events** have no PM copy (`EVT-10`).

| # | Event Key | Family | Code | Group Default Builder (file:line) | Existing Group Keys (`en.yml`) | Has PM? | Existing PM Key (`en.yml`) | Extra Inputs / Appended Notes |
|---|-----------|--------|------|-----------------------------------|-------------------------------|---------|---------------------------|-------------------------------|
| 1 | `ban` | Manual | `bn` | `banDefaultReply` (`bans.go:160`) | `bans_ban_normal_ban` + `bans_ban_ban_reason` | Yes | `eventmsg_pm_ban` | Target, Reason, Unban row, Appeal |
| 2 | `tban` | Manual | `tb` | `tbanDefaultReply` (`bans.go:150`) | `bans_ban_tban` + `bans_ban_ban_reason` | Yes | `eventmsg_pm_tban` | Target, TimeVal/Duration, Until, Reason, Appeal |
| 3 | `kick` | Manual | `kk` | `kickDefaultReply` (`bans.go:52`) | `bans_kick_kicked_user` + `bans_kick_kicked_reason` | Yes | `eventmsg_pm_kick` | Target, Reason |
| 4 | `mute` | Manual | `mu` | `muteDefaultReply` (`mute.go:26`) | `mutes_mute_message` + `mutes_reason_suffix` | Yes | `eventmsg_pm_mute` | Target, Reason, Unmute row |
| 5 | `tmute` | Manual | `tm` | `tmuteDefaultReply` (`mute.go:47`) | `mutes_tmute_message` + `mutes_reason_suffix` | Yes | `eventmsg_pm_tmute` | Target, TimeVal/Duration, Until, Reason |
| 6 | `unban` | Manual | `ub` | `unbanDefaultReply` (`bans.go:499`) | `bans_unban_unbanned_user` | Yes | `eventmsg_pm_unban` | Target |
| 7 | `unmute` | Manual | `um` | `unmuteDefaultReply` (`mute.go:56`) | `mutes_unmute_message` (+ `mutes_unmute_lockdown_note`) | Yes | `eventmsg_pm_unmute` | Target, Note (`mutes_unmute_lockdown_note`), PMNote (`eventmsg_pm_unmute_lockdown`) |
| 8 | `warn` | Auto | `wn` | `warnDefaultReply` (`event_message_auto.go:31`) | `warns_user_warning` + `warns_warning_reason` | Yes | `eventmsg_pm_warn` | Target, Count, Limit, Reason, Remove warn + Rules rows |
| 9 | `warnlimit` | Auto | `wl` | `warnLimitDefaultReply` (`event_message_auto.go:67`) | `warns_limit_{kicked,muted,banned,tmuted,tbanned}` + `\n - <reason>` | Yes (`UntilLine`) | `eventmsg_pm_warnlimit` | Target, Count, Limit, Action, Secs/Duration, `in.Reasons` list, Appeal if ban/tban |
| 10 | `flood` | Auto | `fl` | `floodDefaultReply` (`event_message_auto.go:111`) | `antiflood_checkflood_{perform_action,timed_mute,timed_ban}` | Yes (`UntilLine`) | `eventmsg_pm_flood` | Target (full name), Action, Secs/Duration, Admin row, Appeal if ban/tban |
| 11 | `blacklist` | Auto | `bl` | `blacklistNoticeDefaultReply` (`event_message_auto.go:152`) | `blacklists_bl_watcher_{muted,kicked,banned,tmuted,tbanned}_user` | Yes (`UntilLine`) | `eventmsg_pm_blacklist` | Target (full name), Action, Secs/Duration, Reason, Appeal if ban/tban |
| 12 | `fwdblock` | Auto | `fb` | `blacklistNoticeDefaultReply` (`event_message_auto.go:152`) | same as blacklist + `staff_fwdblock_reason` | Yes (`UntilLine`) | `eventmsg_pm_fwdblock` | Target (full name), Action, Secs/Duration, fixed Reason (never names source/Staff Group!) |
| 13 | `captcha` | Auto | `cf` | `captchaFailureDefaultReply` (`event_message_auto.go:316`) | `captcha_timeout_failure_{kicked,banned,muted}` / `captcha_timeout_with_messages` | Yes | `eventmsg_pm_captcha` | Target (FirstName or "User"), Action, Count (deleted msgs), Appeal if ban |
| 14 | `aispam` | Auto | `ai` | `aispamNoticeDefaultReply` (`event_message_auto.go:219`) | `aispam_notice_deleted` | Yes | `eventmsg_pm_aispam` | Target (full name), never reveals provider/model/score |
| 15 | `mreqwarn` | Mreq | `rw` | `memberReqNoticeDefault(Warn)` (`memberreq_notice.go:169`) | `mreq_notice_warn` / `mreq_notice_warn_limit_{kick,ban,mute}` + remedies | Yes | `eventmsg_pm_mreqwarn` | Target (memberReqMention), Rules mask, Count, Limit, limit Action, remedies |
| 16 | `mreqkick` | Mreq | `rk` | `memberReqNoticeDefault(Kick)` (`memberreq_notice.go:169`) | `mreq_notice_kick` + remedies | Yes | `eventmsg_pm_mreqkick` | Target, Rules mask, remedies |
| 17 | `mreqban` | Mreq | `rb` | `memberReqNoticeDefault(Ban)` (`memberreq_notice.go:169`) | `mreq_notice_ban` | Yes | `eventmsg_pm_mreqban` | Target, Rules mask, Appeal |
| 18 | `mreqmute` | Mreq | `rm` | `memberReqNoticeDefault(Mute)` (`memberreq_notice.go:169`) | `mreq_notice_mute` + remedies | Yes | `eventmsg_pm_mreqmute` | Target, Rules mask, remedies, Required=true, "I fixed it" row |
| 19 | `lockdown` | Lockdown | `ld` | `lockdownStartDefaultReply` (`event_message_lockdown.go:165`) | `lockdown_started` + `lockdown_started_by` + reason + `lockdownStartNoteLines` | No | — | Starter Target, Reason, `lockdownFacts` (NoDelete, NoInvite) |
| 20 | `lockjoin` | Lockdown | `lj` | `lockdownAutoDefaultReply(JoinSurge)` (`event_message_lockdown.go:104`) | `lockdown_auto_started_join_surge` + `lockdownStartNoteLines` | No | — | Count, Window, `lockdownFacts` |
| 21 | `lockflood` | Lockdown | `lf` | `lockdownAutoDefaultReply(Flood)` (`event_message_lockdown.go:104`) | `lockdown_auto_started_flood` + `lockdownStartNoteLines` | No | — | Count, Window, Percent, `lockdownFacts` |
| 22 | `lockai` | Lockdown | `la` | `lockdownAutoDefaultReply(AIBurst)` (`event_message_lockdown.go:104`) | `lockdown_auto_started_ai_burst` + `lockdownStartNoteLines` | No | — | Count, Window, `lockdownFacts` |
| 23 | `locklift` | Lockdown | `lu` | `lockdownLiftDefaultReply` (`event_message_lockdown.go:150`) | `lockdown_lifted` + `lockdownLiftNoteLines` | No | — | Lifter Target, `lockdownFacts` (ManualChange, Finished, Unbanning) |

### 1.3 How Plain Tone Integrates with Zero Regression to MSG-11 Goldens

**CRITICAL:** Every existing site builds `r.Default` (themed/current default) and passes `eventReply` to `postEvent` / `postEventSent`.
To keep **tone off** 100% byte-identical with ZERO extra allocations or DB reads:

1. Add `PlainDefault func() string` (lazy builder) to `eventReply` (`fuku/modules/event_message.go:109-123`):
   - When `msgs.Options.PlainTone` is `false` (default, or read error): `r.PlainDefault` is **never called** -> `r.Default` is sent byte-for-byte identically to MSG-11 goldens!
   - In `postEventSent` (`fuku/modules/event_message.go:454-461`), right after `eventMessagesLookup` succeeds and before `postEventGroup`:
     ```go
     if msgs.Options.PlainTone && r.PlainDefault != nil {
         if plain := r.PlainDefault(); plain != "" {
             r.Default = plain
         }
     }
     ```
   - Look what this automatically guarantees in ONE place:
     - **No custom row (`!row.HasContent()`):** `postEventGroup` calls `sendDefaultPost(site, r)` -> sends plain default with the EXACT same `r.Opts`, bot rows (Unban, Unmute, Remove warn, Rules, "I fixed it"), appeal row, topic, reply-to, and deletion timer!
     - **Custom text set (`row.Text != ""`):** `composeEventMessage` renders `row.Text` -> custom text is completely untouched!
     - **Custom media/buttons ONLY (`row.HasContent() && row.Text == ""`):** `composeEventMessage` uses `r.Default` -> sends plain default text + custom media/buttons!
     - **Custom message refused by Telegram (`eventMessageRefused`):** fallback calls `sendDefaultPost(site, r)` -> falls back to plain default!
     - **`postLockdownLiftEvent(..., withDefault=false)` (group menu lift):** `site.SendDefault` ignores text and returns `nil, nil`, so group-menu lift behavior stays unchanged!
2. For **PM copies** (`fuku/modules/event_message_pm.go`):
   - Add `PlainKey string` to `eventPMSpec` (lines 40-44) and enforce `spec.PlainKey != ""` in `registerEventPM` (panic at init if missing!).
   - Pass `plain bool` (`msgs.Options.PlainTone`) into `composeEventPM` (line 391) and `eventPMDefaultText` (line 423):
     ```go
     key := spec.DefaultKey
     if plain && spec.PlainKey != "" {
         key = spec.PlainKey
     }
     text, _ := tr.GetString(key, params)
     ```
   - Pass `msgs.Options.PlainTone` in `offerReplyPM` (lines 592 & 610, so BOTH default PM AND custom-PM refusal fallback use `spec.PlainKey` when Plain tone is on!).
3. For **Messages page Previews** (`fuku/modules/settings_messages.go`):
   - Add `PlainDefault func(*i18n.Translator, eventDefaultInput) string` to `settingsMsgEvent` (lines 113-125).
   - Add `PlainTone bool` to `settingsEditState` (or pass `msgs.Options.PlainTone` via `settingsMsgStateFor` -> `st.PlainTone`).
   - In `settingsMsgPreviewReply` (line 976): when `plainTone` is true and `ev.PlainDefault != nil`, build `text = ev.PlainDefault(groupTr, in)` instead of `ev.Default(groupTr, in)`.
   - In `settingsMsgPMPreview` (line 1074): pass `st.PlainTone` into `composeEventPM`.
4. **AST Guard Trap (`TestEventEngineCallers` & `TestPMCopyCallers`):**
   - Put all 23 pure plain group default builders in a new file `fuku/modules/event_message_plain.go`.
   - **CRITICAL:** `event_message_plain.go` MUST NOT reference ANY of the 10 `eventEngineEntries` (`postEvent`, `postEventSent`, `postEventGroup`, `replyEvent`, `composeEventMessage`, `renderEventText`, `composeEventPM`, `offerEventPM`, `offerReplyPM`, `eventMessagesLookup`). It only defines pure `func(*i18n.Translator, eventDefaultInput) string` builders + a `plainEventDefault(event string, tr *i18n.Translator, in eventDefaultInput) string` helper! That way `TestEventEngineCallers` and `TestPMCopyCallers` need zero allow-list edits!
   - Also check `unmute`: `mute.go:324` appends `note` (`mutes_unmute_lockdown_note`) to `text` after calling `unmuteDefaultReply`. For `unmute` in plain tone, `r.PlainDefault` should append `"\n" + note` when `note != ""` (same `note` — `mutes_unmute_lockdown_note` is already neutral!).
   - For `mreq*`: `memberreq_notice.go` uses `memberReqMention` (`memberReqNamePlaceholder` spliced via `lockdownSplice`) and appends `memberReqNoticeRemedies` — reuse that exact helper in `memberReqPlainNoticeText` so HTML escaping, remedies, and the 520-code-point / 4096-limit hold!
   - For `lockdown*`: `event_message_lockdown.go` appends `lockdownStartNoteLines` (for `lockdown`, `lockjoin`, `lockflood`, `lockai`) and `lockdownLiftNoteLines` (for `locklift`). Plain lockdown defaults MUST keep the exact technical notes (with plain intro lines, emoji-free and neutral third-person — e.g. `"Group lockdown enabled. Only admins can send messages; new joiners are removed until /unlockdown."`, no `🔒`/`🔓` emojis and no `"I locked"` first-person voice!).

### 1.4 Plain Tone Locale Key Design & Style Rules

**Naming convention (consistent with `eventmsg_pm_*`):**
- Group plain defaults: `eventmsg_plain_ban`, `eventmsg_plain_tban`, `eventmsg_plain_kick`, `eventmsg_plain_mute`, `eventmsg_plain_tmute`, `eventmsg_plain_unban`, `eventmsg_plain_unmute`, `eventmsg_plain_warn`, `eventmsg_plain_warnlimit_kick`, `eventmsg_plain_warnlimit_mute`, `eventmsg_plain_warnlimit_ban`, `eventmsg_plain_warnlimit_tmute`, `eventmsg_plain_warnlimit_tban`, `eventmsg_plain_flood`, `eventmsg_plain_flood_tmute`, `eventmsg_plain_flood_tban`, `eventmsg_plain_blacklist`, `eventmsg_plain_blacklist_timed`, `eventmsg_plain_captcha`, `eventmsg_plain_captcha_with_messages`, `eventmsg_plain_aispam`, `eventmsg_plain_mreq_warn`, `eventmsg_plain_mreq_warn_limit_kick`, `eventmsg_plain_mreq_warn_limit_ban`, `eventmsg_plain_mreq_warn_limit_mute`, `eventmsg_plain_mreq_kick`, `eventmsg_plain_mreq_ban`, `eventmsg_plain_mreq_mute`, `eventmsg_plain_lockdown`, `eventmsg_plain_lockjoin`, `eventmsg_plain_lockflood`, `eventmsg_plain_lockai`, `eventmsg_plain_locklift` (plus shared `eventmsg_plain_reason` if needed — note `eventmsg_pm_reason: "<b>Reason:</b> {reason}"` is already completely neutral!).
  - **Note on `{action}` in plain group defaults (`flood`, `blacklist`, `fwdblock`, `captcha`):** Using `{action}` (via existing translated `eventActionWord(tr, action)`) avoids duplicating separate kick/mute/ban keys while keeping all 8 languages grammatical! For `warnlimit` and `mreq_warn_limit`, separate action keys or `{action}` — verify gender/conjugation in `es`, `fr`, `pt`, `ru`, `uk` so every language reads naturally!
- PM plain defaults (all 18 `eventPMSpecs` keys): `eventmsg_plain_pm_ban`, `eventmsg_plain_pm_tban`, `eventmsg_plain_pm_kick`, `eventmsg_plain_pm_mute`, `eventmsg_plain_pm_tmute`, `eventmsg_plain_pm_unban`, `eventmsg_plain_pm_unmute`, `eventmsg_plain_pm_warn`, `eventmsg_plain_pm_warnlimit`, `eventmsg_plain_pm_flood`, `eventmsg_plain_pm_blacklist`, `eventmsg_plain_pm_fwdblock`, `eventmsg_plain_pm_captcha`, `eventmsg_plain_pm_aispam`, `eventmsg_plain_pm_mreqwarn`, `eventmsg_plain_pm_mreqkick`, `eventmsg_plain_pm_mreqban`, `eventmsg_plain_pm_mreqmute` — exact same named placeholders (`{group}`, `{until}`, `{count}`, `{limit}`, `{action}`) as `eventmsg_pm_*`, neutral factual wording with NO leading emoji and NO first-person `"I removed"`!
- **Locale guards (`fuku/i18n/locale_style_test.go` & `locale_rewrite_guard_test.go`):**
  - `TestLocaleStyle` checks `uk` and `en`: NO markdown `*bold*` or `` `code` ``, NO `× ` bullets, NO double spaces, NO leading/trailing spaces (one leading `\n` allowed for reason fragments), NO 3 newlines in a row, balanced Telegram HTML tags only (`b, i, u, s, code, pre, a, blockquote, tg-spoiler`), NO two-line reply with a 1-4 word opener on line 1 (`"Done.\nFact."`).
  - `TestLocalePlaceholdersMatchOtherLanguages`: placeholder set in `en`, `es`, `fr`, `hi`, `id`, `pt`, `ru`, `uk` MUST match identically for every key!
  - All 8 languages: `en`, `es`, `fr`, `hi`, `id`, `pt`, `ru`, `uk`.

### 1.5 Plain Tone Switch on Messages Page (`settings_messages.go`)

**Analog:** `settingsMsgPMOptions` / `settingsMsgRunPMOption` (`fuku/modules/settings_messages.go:556-595, 786-815, 1128-1143`) and `applyEventPMLog` (`fuku/modules/event_message_apply.go:161-169`).

- **Free action code:** `settingsActMsgPlainTone = "ht"` (verified unused across all 28 pages and core actions!).
  - Fields: `[]string{"v"}` (exact `"0"` or `"1"` via `settingsEditSwitchValue(fields["v"])`, plus `settingsListPrivateField(fields)` so group-menu presses fail in step 1 with 0 Telegram calls!).
  - Right: `settingsRightChangeInfo` (matches every Messages write: `hq`, `hp`, `ho`, `hs`, `hr`).
- **Apply function (`fuku/modules/event_message_apply.go`):**
  - Add `PlainTone func(ctx context.Context, chatID int64, on bool, by int64) error` to `eventMsgWrites` (default `eventmsg.SetPlainTone`).
  - Add `applyEventPlainTone(ctx context.Context, chatID int64, on bool, by int64) eventMsgOptionsResult` calling `eventMsgOptionsReadBack(ctx, chatID, "plain tone", writeErr)`.
- **Hub layout (`renderSettingsMsgHub`, `fuku/modules/settings_messages.go:1111-1146`):**
  - Add a compact Plain tone line (e.g. `settings_msg_line_plain_tone` with `{state}`) and toggle button (`a=ht`, `v="1"`/`"0"`, label `settings_btn_msg_plain_tone` with `{state}` icon) on the hub!
  - Hub keyboard row widths change from `[2 2 1 1 2]` to **`[2 2 1 1 1 2]`** (4 families in 2x2 grid + Plain tone toggle + PM Details toggle + PM Log toggle + Back|Close).
  - **Existing tests that assert `[2 2 1 1 2]` (MUST update all 6):**
    1. `fuku/modules/settings_messages_test.go:93`
    2. `fuku/modules/settings_messages_auto_test.go:56`
    3. `fuku/modules/settings_messages_lockdown_test.go:55`
    4. `fuku/modules/settings_messages_pm_options_test.go:120, 148`
    5. `fuku/modules/settings_messages_screening_test.go:54`
    6. `fuku/modules/settings_messages_screens_test.go:173`
  - Also add `settingsActMsgPlainTone` to `settingsMsgValidPresses` in `fuku/modules/settings_messages_safety_test.go:25-36`.
  - Check hub total length in all 8 languages (`TestSettingsMessagesScreens` in `settings_messages_screens_test.go`) — keep the Plain tone line + short hint concise so all 8 locales stay well below 4096 UTF-16 units.
  - Check `settingsMsgMainState` (`settings_messages.go:695-704`): when `settingsMsgCountChanged(msgs) == 0 && msgs.Options.PlainTone`, return `settings_state_msg_plain` (or keep `settings_state_msg_n` when `n > 0`).

---

## Part 2: Copy Settings (TUNE-09) & Shared Validated Message Write Function (MSG-10)

### 2.1 Complete Table & Cache Key Inventory for Copy Settings

**Provisional Owner Defaults to document in `AGENTS.md` and `29-VERIFICATION.md`:**
- **`P29-01` (Full Mirror on Copy):** Copying overwrites the target's messages and tuning in scope to match the source completely. An empty/default source entry **clears** the target's custom message, Silent/PM switch, options, escalation ladder, manual defaults, reason shortcuts, or advanced thresholds so the target ends up identical to the source across every copied area.
- **`P29-02` (Authority in Both Groups):** Both source and target are verified live at ask AND again at Yes via `lockdownLiveMember` + `settingsGate` + `GetChat` (group/supergroup only) + non-Staff-Group check. For non-creator admins admitted by the owner gate (`chat_settings_access`), require BOTH `can_change_info` AND `can_restrict_members` in the target group (since copy writes both messages and punishment tuning). Any target-side refusal (bot not in chat, non-admin, owner gate refusal, Staff Group, channel/private, missing target right) returns one uniform opaque refusal (`settings_copy_unavailable` / `settings_pick_unavailable`) that reveals nothing about the target group.
- **`P29-03` (Blacklist Action Without Copying Words):** Blacklist words themselves are NOT copied. Only the source's blacklist action & timed duration (`action`, `action_secs` — from source's first row, or `"warn", 0` if source has no blacklist words) are applied to the target's existing blacklist words (if any) via `blacklists.SetBlacklistActionTimedContext`.
- **`P29-04` (Warns & Antiflood Tuning Scope):** Copies `warns_settings` tuning (`warn_mode`, `warn_mode_secs`, `warn_expiry_days` — preserving target's `warn_limit` or default 3, never touching `warns_users`) and `antiflood_settings` tuning (`action`, `action_secs` — preserving target's `flood_limit` and `delete_antiflood_message`, and inserting `flood_limit = 0` when target has no row so antiflood is NEVER accidentally enabled!).

**Exact Tables Copied (9 tables across 6 tuning/message areas named on the confirm card):**

| Confirm Card Area | Table(s) | Columns Copied | Cache Key(s) Dropped | In `skipLocal`? | Repository Helpers Needed |
|-------------------|----------|----------------|----------------------|-----------------|---------------------------|
| 1. Custom event messages | `chat_event_messages` | `event`, `dest`, `text`, `file_id`, `media_type`, `buttons`, `silent` (full replace via shared validated writer!) | `event_messages:<dst>` | Yes (`event_messages:`) | NEW `eventmsg.ReplaceAllValidated` |
| 2. Message switches & options (Silent, PM, Plain tone) | `chat_event_message_options` | `pm_details`, `pm_log`, `plain_tone` | `event_messages:<dst>` | Yes (`event_messages:`) | Included in `eventmsg.ReplaceAllValidated` (or `eventmsg.SetOptions`) |
| 3. Timed auto-punishments & warn expiry (Phase 26) | `warns_settings`, `antiflood_settings`, `blacklists` (action/secs only) | `warns_settings.{warn_mode, warn_mode_secs, warn_expiry_days}`; `antiflood_settings.{action, action_secs}`; `blacklists.{action, action_secs}` (existing target rows only) | `warn_settings:<dst>`, `antiflood:<dst>`, `blacklist:<dst>` | No (standard TTL; `DeleteCache` drops locally + Redis) | `warns.SetWarnModeContext` + `warns.SetWarnExpiryContext` (or atomic `warns.SetWarnTuningContext`); NEW `antiflood.SetFloodActionPreserveLimitContext` (TRAP: see below!); existing `blacklists.SetBlacklistActionTimedContext` |
| 4. Escalation ladder (Phase 26) | `chat_escalation` | `enabled`, `step1..5_action`, `step1..5_secs`, `forget_days` | `chat_escalation:<dst>` | Yes (`chat_escalation:`) | NEW `escalation.ReplaceLadder(ctx, dst, ladder, by)` (atomic single upsert of enabled + all 5 steps + forget_days, or delete when default) |
| 5. Manual defaults, appeal link & reason shortcuts (Phase 27) | `chat_manual_defaults`, `chat_reason_shortcuts` | `default_ban_secs`, `default_mute_secs`, `reason_required`, `appeal_url` (re-validated via `content.ValidButtonURL`); full replace of `(word, reason)` rows | `chat_manual_defaults:<dst>`, `chat_reason_shortcuts:<dst>` | Yes (both!) | NEW `modtuning.ReplaceDefaults(ctx, dst, defs, by)` & `modtuning.ReplaceShortcuts(ctx, dst, shortcuts, by)` (single tx: delete old + insert valid + drop cache) |
| 6. Advanced thresholds (Phase 28) | `chat_advanced_thresholds` | All 12 columns (validated against `advthresholds.SpecFor` bounds; `ResetAll` if all 0) | `chat_advanced_thresholds:<dst>` | Yes (`chat_advanced_thresholds:`) | NEW `advthresholds.ReplaceAll(ctx, dst, th, by)` (validates all 12 specs, upserts or calls `ResetAll` if zero, drops cache) |

**CRITICAL TRAP — `antiflood.SetFloodActionContext` (`fuku/db/antiflood/repository.go:33-42, 82-96`):**
- `models.AntifloodSettings` has `gorm:"column:flood_limit;default:5"` (`fuku/db/models/antiflood.go:11`).
- When a chat has NO row in `antiflood_settings`, `GetAntifloodSettingsContext` returns `Limit: 0` (antiflood OFF!).
- Calling `antiflood.SetFloodActionContext` on a target chat with no row runs `FirstOrCreate(&models.AntifloodSettings{})`, which inserts **`flood_limit = 5`**, **silently turning antiflood ON** in the target group!
- **Fix:** Add `antiflood.SetFloodActionPreserveOffContext(ctx, chatID, mode, secs)` using `clause.OnConflict{Columns: {chat_id}, DoUpdates: AssignmentColumns{"action", "action_secs", "updated_at"}}` on `&models.AntifloodSettings{ChatId: chatID, Limit: 0, Action: mode, ActionSecs: secs}` (explicitly using `.Select("chat_id", "flood_limit", "action", "action_secs")` or raw/map insert with `flood_limit: 0` so GORM does NOT substitute `default:5`!), then `cache.DeleteCache(cache.CacheKey("antiflood", chatID))`! Verify with a DB test that copying to a target with no `antiflood_settings` row leaves `Limit == 0`!

**Strictly Excluded Tables (NEVER copied, NEVER modified on target, verified by test):**
- `staff_groups`, `staff_group_links`, `staff_forward_blocks`, `staff_actions`
- `approvals`
- `warns_users` (member warn counts), `chat_offences` (member offence counts)
- `chat_lockdowns`, `lockdown_joiners`
- `greetings`, `rules`, `notes`, `filters`, `blacklists` (word list itself — words never added/deleted!), `locks`, `pins`, `reactions`, `reports`, `log_channels`, `captcha_settings`, `member_requirements`, `delete_rules`, `chat_conversations`, `chat_settings_access`, `chat_timezones`, `chats`

### 2.2 Shared Validated Message Write Function (Reused by Phase 30 `MSG-10`)

**Location:** `fuku/db/eventmsg/validate_write.go` (in package `eventmsg`, NOT `fuku/modules`, because `fuku/db/backup` imports `fuku/db/eventmsg` and cannot import `fuku/modules` without an import cycle!).

**Import safety verified:**
- `fuku/utils/content` imports only `fuku/db` and `fuku/i18n`
- `fuku/utils/formatting` imports `fuku/db`, `fuku/db/rules`, `fuku/i18n`
- `fuku/utils/keyboard` imports `fuku/config`, `fuku/db`, `fuku/i18n`, `fuku/utils/callbackcodec`, `fuku/utils/chat_status`
- None imports `fuku/db/eventmsg` -> **ZERO circular imports!**

**Validation rules (mirroring `fuku/modules/settings_edit_check.go` + `fuku/utils/content/content.go` + `fuku/modules/settings_messages.go`):**

```go
// EventMessageSpec constants (matching settings_messages.go & settings_edit_check.go):
const (
	MaxTextLength       = 4096 // Telegram text limit (code points & UTF-16 visible length)
	MaxCaptionLength    = 1024 // Telegram caption limit with media
	MaxButtons          = 100  // Telegram total inline keyboard buttons
	MaxRowButtons       = 8    // Telegram max buttons per row
	MreqTextReserve     = 600  // memberReqNoticeNoteReserve (group dest only)
	LockdownTextReserve = 400  // lockdownNoteReserve (group dest only)
)
```

For each candidate `Message` in `ValidateMessage(msg Message) (Message, string)` (returns normalized message and empty reason on OK, or rejection reason code):
1. **Event & Dest:** `ValidEvent(msg.Event)` and `ValidDest(msg.Dest)`. Reject `DestPM` for lockdown events (`EventLockdown`, `EventLockJoinSurge`, `EventLockFlood`, `EventLockAIBurst`, `EventLockLift` — `HasPM(event)` helper in `eventmsg`!).
2. **Media:**
   - If `msg.FileID == "" && msg.MediaType == 0`: valid (no media).
   - If `(msg.FileID == "") != (msg.MediaType == 0)`: reject `"invalid_media"`.
   - Allowed media types: `models.STICKER`, `models.DOCUMENT`, `models.PHOTO`, `models.AUDIO`, `models.VOICE`, `models.VIDEO`, `models.ANIMATION` (reject `models.TEXT`, `models.VIDEO_NOTE`, or any other int!).
   - For `NoMedia` events (`EventReqWarn`, `EventReqKick`, `EventReqBan`, `EventReqMute`) with `DestGroup`: reject `"media_not_allowed"` (matches `settingsMsgFamilies` `NoMedia: true`).
3. **Buttons:**
   - Every button must have `content.ValidButtonURL(btn.Url)` (`http://` or `https://` with non-empty host; rejects `javascript:`, `file:`, `tg:`, empty, scheme-less).
   - Every button name non-empty (or default `"Greeting"` if empty? Reject empty name or check length; editor defaults blank name to `"Greeting"`).
   - Total buttons + `rulesTokenBonus` (`{rules}`, `{rules:same}`, `{rules:up}` in `msg.Text`) + `ReservedButtonsFor(msg.Event, msg.Dest)` <= `MaxButtons` (100), and widest row via `keyboard.BuildKeyboard` <= `MaxRowButtons` (8).
   - Reserved buttons per `(event, dest)` (exact match to `settings_messages.go:150-247, 469-471`):
     - `ban|group`: 2 (Unban + Appeal), `tban|group`: 1 (Appeal), `mute|group`: 1 (Unmute), `warn|group`: 1 (Remove warn), `flood|group`: 1 (Unmute/Unban), `mreqban|group`: 1 (Appeal), `mreqmute|group`: 1 ("I fixed it")
     - `ban|pm`, `tban|pm`, `mreqban|pm`: 1 (Appeal)
4. **Text Length:**
   - Limit = `MaxTextLength` (4096) when no media, `MaxCaptionLength` (1024) when media is present.
   - Subtract `ReservedTextFor(msg.Event, msg.Dest)`: `600` for `mreq*` (`DestGroup`), `400` for `lock*` (`DestGroup`), `0` for all `DestPM` and other events.
   - Check BOTH raw rune count (`utf8.RuneCountInString(msg.Text) <= rawLimit`, matching `content.TextLengthRefusal`) AND visible UTF-16 length (`formatting.VisibleLength(msg.Text) <= effectiveLimit`, matching `settingsEditCheckGreetingText`).
5. **Silent Normalization:**
   - For `EventReqMute` (`DestGroup`): force `Silent = false` (cannot be silenced, `NoSilent`).
6. **Transactional Write (`ReplaceAllValidated`):**
   ```go
   type SkippedRow struct {
       Event  string
       Dest   string
       Reason string
   }
   type WriteReport struct {
       Written int
       Skipped []SkippedRow
   }
   func ReplaceAllValidated(ctx context.Context, chatID int64, rows []Message, opts Options, by int64) (WriteReport, error)
   ```
   - Validates all rows first (collecting valid rows & `Skipped` entries, deduplicating by `(event, dest)`).
   - Inside ONE `db.DB.WithContext(ctx).Transaction(...)`:
     - `DELETE FROM chat_event_messages WHERE chat_id = ?`
     - Insert every valid row (skipping rows that have `!row.HasContent() && row.Silent == DefaultSilentFor(row.Event, row.Dest)` to avoid useless empty rows!)
     - Upsert `chat_event_message_options` with `PMDetails`, `PMLog`, `PlainTone`, `UpdatedBy`, `UpdatedAt` (or delete if all three are `false`!).
   - After commit: `cache.DeleteCache(CacheKey(chatID))`.
   - **Drift guard test:** A test in `fuku/modules/` verifies `eventmsg.ReservedButtonsFor`, `eventmsg.ReservedTextFor`, `eventmsg.HasPM`, and `eventmsg.NoMedia` match `settingsMsgFamilies` & `eventPMSpecs` 1-to-1 so Phase 29/30 and the editor can NEVER drift!

### 2.3 Private Copy Settings Page (`settings_copy.go`)

**Analogs:**
- Page structure & step-1 validation: `fuku/modules/settings_advthresholds.go` (lines 212-265)
- Group list & paging: `fuku/modules/settings_hub.go` (lines 48-175 — reuses `settingsHubLookup`, `settingsHubSort`, `settingsHubLabel`, `settingsHubPages`, `settingsHubPageSize`)
- Opaque live target verification: `fuku/modules/settings_menu.go` (lines 472-542 — `lockdownLiveMember`, `isTelegramClientError`, `settingsGate`, `GetChatWithContext`)
- One-time Redis token: `fuku/modules/event_message_pm_details.go` (lines 76-143) & `fuku/modules/settings_input.go` (lines 38-61)

**Page registration & free codes (all verified unused!):**
- Page key: **`settingsPageCopy = "cp"`**
- Order: **`82`** (immediately after Advanced `av` Order 81, before Deletion `d` Order 85; leaves 83-84 for Phase 30 Backup)
- Icon: **`📋`**, `TitleKey: "settings_page_copy"`
- `PrivateOnly: true`, `PrivateNoteKey: "settings_copy_group_note"`
- `Hidden: settingsCopyHidden` (fresh `staff.GetStaffGroupFresh(group.Id)`: hidden in Staff Groups, error fails open to show or matches `settingsAdvHidden` — wait: check `settingsAdvHidden`: `err == nil && staffRow != nil`, and on actual copy execution Staff Group check fails closed!)
- `MainState: settingsCopyMainState` (returns static `"—"` or `"settings_state_copy"` with **0 DB reads and 0 Telegram calls** so opening Main stays fast and creates 0 rows!)
- **Action codes on page `cp`:**
  - `settingsActCopyPage = "cg"` — Fields: `[]string{"v"}` (page `1..999`), Right: `settingsRightAdmin`, private `c` required.
  - `settingsActCopyPick = "ck"` — Fields: `[]string{"t"}` (negative int64 target chat ID via `parseStaffChatIDArg`, `t != c`), Right: `settingsRightAdmin`, private `c` required.
  - `settingsActCopyYes = "cy"` — Fields: `[]string{"k"}` (16-char base64url one-time token), Right: `settingsRightAdmin`, private `c` required.

**Callback Data 64-Byte Cap Verification:**
| Action | Worst-case Callback (14-char supergroup ID) | Worst-case (`math.MinInt64` 20-char ID) | Limit |
|--------|---------------------------------------------|------------------------------------------|-------|
| Open `cp` | `settings\|v1\|a=op&c=-1009999999999&p=cp` (38 B) | `settings\|v1\|a=op&c=-9223372036854775808&p=cp` (44 B) | <= 64 B |
| Page `cg` | `settings\|v1\|a=cg&c=-1009999999999&v=999` (39 B) | `settings\|v1\|a=cg&c=-9223372036854775808&v=999` (45 B) | <= 64 B |
| Pick `ck` | `settings\|v1\|a=ck&c=-1009999999999&t=-1009999999998` (50 B) | `settings\|v1\|a=ck&c=-9223372036854775808&t=-9223372036854775807` (62 B) | <= 64 B |
| Yes `cy` | `settings\|v1\|a=cy&c=-1009999999999&k=0123456789abcdef` (52 B) | `settings\|v1\|a=cy&c=-9223372036854775808&k=0123456789abcdef` (58 B) | <= 64 B |
| Cancel `no` | `settings\|v1\|a=no&c=-1009999999999&p=cp` (38 B) | `settings\|v1\|a=no&c=-9223372036854775808&p=cp` (44 B) | <= 64 B |

**One-Time Confirm Card Token (Replay & Expiry Protection — Success Criterion 4):**
- Redis key: **`fuku:settings:copy:<token>`** (outside `fuku:cache:`, so `CLEAR_CACHE_ON_STARTUP` keeps it; document in `AGENTS.md`!).
- Token: 12 random bytes from `crypto/rand` encoded with `base64.RawURLEncoding` = **16 characters** (96 bits of entropy; strict validator checks `len(k) == 16` and every byte in `[A-Za-z0-9_-]`).
- Payload JSON: `{"u": userID, "s": sourceChatID, "t": targetChatID}`.
- Stored on `ck` (ask) via `SET NX` with TTL = `settingsCardLifetime` (5 minutes). If Redis is unavailable (`client == nil`), answer `settings_copy_unavailable` and draw no card.
- On `cy` (Yes):
  1. Peek record with `GET fuku:settings:copy:<token>`. Missing -> expired/already used -> answer `settings_card_expired` (or `settings_copy_expired`) and redraw `cp` (changes NOTHING!).
  2. Check `rec.UserID == p.query.From.Id && rec.SourceChatID == p.group.Id && rec.TargetChatID != rec.SourceChatID`. Mismatch -> answer `common_callback_invalid_request` (changes NOTHING, reveals NOTHING!).
  3. Live re-verify source (already done by `verify()` + check non-Staff-Group fresh + check target write rights if non-creator) AND live re-verify target (`lockdownLiveMember` + `settingsGate` + target write rights + `GetChatWithContext` group/supergroup + fresh `staff.GetStaffGroupFresh` == nil):
     - Transient error (429/5xx/timeout/DB error) -> `settings_check_failed` (changes NOTHING!).
     - Any authorization / missing-bot / non-group / Staff-Group refusal -> **uniform opaque refusal** (`settings_pick_unavailable`, changes NOTHING, reveals NOTHING about the target!).
  4. Atomically consume token via 2-line Lua script (`GET` + `DEL` on `KEYS[1]`): if nil (concurrent/replayed second press won the race!), answer `settings_card_expired` and change NOTHING!
  5. Execute `applyCopySettings(ctx, rec.SourceChatID, rec.TargetChatID, p.query.From.Id)` -> copies all 6 areas, drops all target caches, never touches source, links nothing afterwards!
  6. Redraw `cp` with success notice naming the target group title (escaped via `html.EscapeString`!).

**Confirm Card Text Requirements (Success Criterion 2):**
The confirm card drawn by `ck` MUST explicitly name:
1. Source group title (`html.EscapeString(p.group.Title)`) and Target group title (`html.EscapeString(targetChat.Title)`).
2. Exactly what WILL be overwritten in the target group:
   - Custom event messages (group and PM)
   - Silent, PM copy, and Plain tone switches & options
   - Timed automatic punishment durations & warn expiry (warn limit, flood, blacklist action)
   - Escalation ladder
   - Manual command defaults, appeal link, and reason shortcuts
   - Advanced thresholds
3. Explicitly what is NOT copied (Staff Group links, approvals, member warns/offences, bans/lockdowns, welcomes/rules/notes/filters/blacklist words).
4. One-time copy note ("This copies once and does not link the groups.").

---

## Shared Patterns

### Live Dual-Group Authorization & Opaque Refusal
**Source:** `fuku/modules/settings_menu.go` (lines 472-542) & `fuku/modules/settings_gate.go` (lines 89-116)
**Apply to:** `fuku/modules/settings_copy.go` (`ck` ask AND `cy` Yes)
```go
member, err := lockdownLiveMember(b, targetID, presser.Id)
if err != nil {
	if isTelegramClientError(err) {
		return copyTargetUnavailable // uniform opaque refusal: never reveals whether bot is in target
	}
	return copyTargetCheckFailed
}
if member.Status != gotgbot.ChatMemberStatusCreator && member.Status != gotgbot.ChatMemberStatusAdministrator {
	return copyTargetUnavailable
}
switch settingsGate(b, targetID, presser, member) {
case settingsGateIn:
case settingsGateOwnerOnly:
	return copyTargetUnavailable // uniform opaque refusal on target!
default:
	return copyTargetCheckFailed
}
// Non-creator admin must hold both can_change_info and can_restrict_members in target (P29-02).
if member.Status != gotgbot.ChatMemberStatusCreator && (!member.CanChangeInfo || !member.CanRestrictMembers) {
	return copyTargetUnavailable
}
// Verify target is a group/supergroup and NOT a Staff Group (fail closed on DB error).
```

### Column-Targeted Upserts + Cache Invalidation
**Source:** `fuku/db/eventmsg/repository.go` (lines 220-235), `fuku/db/modtuning/repository.go` (lines 156-169), `fuku/db/advthresholds/repository.go` (lines 457-473)
**Apply to:** Every write function in `eventmsg`, `antiflood`, `warns`, `blacklists`, `escalation`, `modtuning`, `advthresholds`.
- Never use `gorm:"default:..."` on boolean or zeroable integer columns.
- Always call `cache.DeleteCache(...)` after commit on every write.
- Never log group titles, user names, message texts, file IDs, button URLs, or appeal URLs — log chat IDs, user IDs, event keys, and error classes only.

### Settings Page Grid & Code Guard Tests to Update
**Source:** Phase 28 commit `8b854206`
**Apply to:** Adding page `cp` (Order `82`) and Messages hub toggle `ht`:
1. `fuku/modules/settings_main_test.go`:
   - `settingsMainShapeGroup` & `settingsMainShapePrivate`: insert `cp` immediately after `av` (Order 82, right before `d` Order 85).
     - Current (28 pages, 27 non-Staff): `"s|l / c|f / q|n / j|k / rs|b / i|w / g|r / e|x / h|m / av|d / t|u / z|o / y / p / a / close"` (`settingsMainOwnerRows = 16`)
     - New (29 pages, 28 non-Staff): `"s|l / c|f / q|n / j|k / rs|b / i|w / g|r / e|x / h|m / av|cp / d|t / u|z / o|y / p / a / close"` (**STILL 16 rows!** Wait: verify! `av|cp / d|t / u|z / o|y` has 4 rows of 2, whereas before `av|d / t|u / z|o / y` had 3 rows of 2 + 1 row of 1 = 4 rows! So `settingsMainOwnerRows` stays `16` and `staffOwnerRows` in `settings_fwdblock_test.go` stays `settingsMainOwnerRows - 3`! Planner MUST verify exact row count when running `TestSettingsMainGrid` and `TestSettingsFwdBlockMainShape`!)
   - `settingsMainTitleKeys`: insert `"settings_page_copy"` between `"settings_page_advanced"` and `"settings_page_del"`.
2. `fuku/modules/settings_phase19_test.go:96`: add `"cp"` to `want`.
3. `fuku/modules/settings_phase18_test.go:171`: note it was already behind Phases 27-28 (`q`, `rs`, `av`) — check whether `TestSettingsPhase18Codes` runs in `make test` and update `want` if needed so full suite is green!
4. `fuku/modules/settings_messages*_test.go`: update hub width `[2 2 1 1 2]` -> `[2 2 1 1 1 2]` in all 6 test files and add `settingsActMsgPlainTone` to `settings_messages_safety_test.go`.

---

## No Analog Found

None — every file in Phase 29 has an exact tracked analog in Phases 21–28.

---

## Metadata

**Analog search scope:** `fuku/db/{eventmsg,modtuning,advthresholds,escalation,warns,antiflood,blacklists,cache,models,backup}/`, `fuku/modules/`, `fuku/utils/content/`, `fuku/i18n/`, `migrations/`, `locales/`
**Files scanned:** 38 git-tracked source & test files
**Pattern extraction date:** 2026-11-23
