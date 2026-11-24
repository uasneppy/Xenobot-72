# Phase 26: Timed Punishments, Escalation and Warn Expiry - Context

**Gathered:** 2026-11-22
**Status:** Ready for planning. Decisions marked "owner" were settled before this phase: the milestone decisions of 2026-11-18, the
roadmap's planning notes and success criteria for Phase 26, the requirements TUNE-01, TUNE-02 and TUNE-03, and the owner's standing rules for
this run (everything is set group by group in that group's settings in the bot's private chat, and there are no Staff Group defaults).
Decisions marked "default" are the planner's provisional defaults. I took them from the documents and the code without asking the owner,
because the owner authorized this run to proceed on its own. They can be reversed. The ones an owner would notice are listed at the end under
"Provisional decisions for owner confirmation" (P-01 to P-24).

<domain>
## Phase Boundary

Phase 26 changes how a group's automatic punishments behave. It changes no message engine, no Staff Group surface and no lockdown path:

- **Timed actions (TUNE-01).** Warn limit, blacklist and antiflood can apply a timed ban (`tban`) or a timed mute (`tmute`) with a duration
  the owner chooses. Each one is set from its settings page and from its command (`/setwarnmode`, `/blacklistaction`, `/setfloodmode`), and
  both go through one apply function. When a timed action fires, the member is restricted until the end date, Telegram itself ends the
  restriction at that date, and the message shows `{duration}` and `{until}`.
- **Escalation ladder (TUNE-02).** A new private settings page, **Escalation**, is off by default. On it the owner defines what a member's 1st,
  2nd and later automatic offences in that group get. An offence is a punishment by flood, blacklist or forward blocklist. The count is held
  in PostgreSQL, so every replica sees the same count. Offences in one 60 s window count once, and a count is forgotten after the period the
  owner picks.
- **Warn expiry (TUNE-03).** On the Warnings page the owner can make warns expire after N days. It is off by default. An expired warn no
  longer counts toward the limit, and `/warns`, `/rmwarn`, `/resetwarns`, the Remove warn button, the AI spam filter's warn count, the backup
  export and the limit check all count the same warns.

A group that sets none of this behaves exactly as it does today, and every argument the three commands accept today answers as before
(success criterion 4). This is proved by goldens recorded first (MSG-11) and by the existing goldens, which are never re-recorded.

Out of scope:
- timed actions for the Staff Group's forward blocklist list (its vocabulary stays `none warn mute kick ban`, D-07);
- the warn limit, member requirements, captcha and AI spam as ladder sources (D-24);
- a command for the ladder or the expiry (D-38);
- typed custom durations on the pages (P-04);
- a note on the Antiflood and Blacklist pages that the ladder overrides them;
- an `{offence}` placeholder;
- manual command defaults (Phase 27), global limits (Phase 28), plain tone and copy settings (Phase 29), backups of the ladder (Phase 30).

</domain>

<decisions>
## Implementation Decisions

### Scope rules (owner)
- **D-01 (owner):** Everything is set group by group in that group's settings in the bot's private chat. There are no Staff Group defaults and
  nothing is inherited. The ladder of the group where the member offended applies, never a Staff Group's.
- **D-02 (owner, MSG-11):** The first plan records characterization goldens of every command and send site this phase changes that no golden
  pins yet, and commits them alone before any production change. The existing goldens (`TestLegacyWarnSettingsRequests`,
  `TestLegacyBlacklistRequests`, `TestLegacyPunishmentNoticeRequests`, `TestLegacyScreeningNoticeRequests`, `TestLegacyModerationRequests`)
  are never re-recorded, weakened or edited, and every plan reruns them.
- **D-03 (owner, success criterion 4):** A group that sets none of this behaves exactly as today. Every argument the three commands accept
  today answers byte for byte as before, including the pinned refusals: `/setwarnmode tban` alone still answers `warns_mode_unknown`
  (`TestLegacyWarnSettingsRequests` `mode_tban_unknown`), and `/blacklistaction ban mute` still answers `choose_correct_option`
  (`action_two_arguments`). New arguments get new tests.
- **D-04 (owner, cross-cutting):** The punishment comes first, a message never delays or undoes it, and unknown never punishes. Deletion
  timers stay as they are: a timed action's notice is queued under the same kind as today's (`bot:punish_auto`, or `bot:punish_manual` for a
  manual `/warn` that reached the limit).

### Timed actions (TUNE-01, default)
- **D-05:** A duration is whole seconds, from `punishMinSecs` (60, the shortest `extraction.ParseDurationToken` token, `1m`) to
  `punishMaxSecs` (366 days, 31,622,400 s). Commands parse it with `extraction.ParseDurationToken` (the grammar of `/tban`, `/tmute` and the
  staff commands). A token over 366 days, a zero and anything that is not a token are refused with a new "bad duration" reply and nothing is
  written. Unlike the staff commands, a timed automatic action is never turned into a permanent one (P-01). The bounds are defined once, in
  `fuku/db/models/punish.go` (`models.PunishMinSecs`, `models.PunishMaxSecs`, `models.PunishDurationValid`), so the modules and the backup
  import check the same numbers.
- **D-06:** Storage, migration `20261122120000_add_timed_punishment_durations.sql`. It adds three columns, each `INTEGER NOT NULL DEFAULT 0`
  with an inline named CHECK of 0 to 31,622,400:
  - `warns_settings.warn_mode_secs` (`chk_warns_settings_mode_secs`);
  - `antiflood_settings.action_secs` (`chk_antiflood_action_secs`);
  - `blacklists.action_secs` (`chk_blacklists_action_secs`, one per word, like the action).

  The models gain `WarnModeSecs`, `ActionSecs` and `ActionSecs`, with no `default:` tag and with JSON `omitempty`, so an export of a group
  with no duration is unchanged.
- **D-07:** The action vocabularies stay the same. `warnSettingsModes` (`ban kick mute`), `floodModes` (`ban kick mute`) and
  `blacklistActionValues` (`mute kick warn ban none`, which `TestForwardBlockVocabulary` holds equal to `staff.ForwardBlockActions`) keep
  their contents. The timed modes `tban` and `tmute` are accepted beside them only with a valid duration. The forward blocklist (a Staff Group
  setting) gets no timed action; it can reach a timed one only through a linked group's ladder (D-23).
- **D-08:** A timed mode with no stored duration (0) is inert, exactly as a stored `tban` or `tmute` is today:
  - the warn limit posts today's `warnPostLimit` reply and applies nothing (`TestLegacyPunishmentNoticeRequests` `warnlimit_inert`);
  - the flood posts today's inert notice;
  - the blacklist deletes the message and posts nothing.

  So no group's stored setting starts acting by itself. The effective rule is one pure function, `autoActionEffective(action, secs)`: `mute`,
  `kick` and `ban` count, `tmute` and `tban` count only with a valid duration, and everything else does not.
- **D-09:** One apply function per setting, beside its command. Each takes the action and the seconds, and each command and page calls it:
  - `applyWarnMode(ctx, chatID, mode, secs)`;
  - `applyFloodMode(ctx, chatID, mode, secs)`;
  - `applyBlacklistAction(ctx, chatID, action, secs)`.

  They behave the same way:
  - a plain action stores seconds 0 (it clears a duration);
  - a timed action with seconds 0 is the existing unknown-action outcome (the pinned reply);
  - a timed action with seconds outside the bounds is a new bad-duration outcome;
  - a valid timed action stores both columns in one write.

  The repository's two-argument wrappers keep their signatures and write seconds 0, because the goldens call them: `warns.SetWarnMode`,
  `antiflood.SetFloodMode`, `blacklists.SetBlacklistAction`, `blacklists.AddBlacklist`. The new writes are
  `warns.SetWarnModeContext(ctx, chatID, mode, secs)` (a column-targeted upsert of `warn_mode` and `warn_mode_secs`),
  `antiflood.SetFloodActionContext(ctx, chatID, mode, secs)` and `blacklists.SetBlacklistActionTimedContext(ctx, chatID, action, secs)`.
  Every write drops the keys it drops today (`warn_settings`, `antiflood`, `blacklist`).
- **D-10:** Command grammar. Each command accepts a new form: `/setwarnmode tmute 2h`, `/setwarnmode tban 1d`, `/setfloodmode tmute 30m`,
  `/blacklistaction tban 1w`. In detail:
  - A timed word alone (`tban`, `tmute`) answers exactly today's unknown or choose-an-option reply.
  - A timed word followed by a word that is not a valid duration answers the new key `warns_mode_bad_duration`,
    `antiflood_setfloodmode_bad_duration` or `blacklists_set_bl_action_bad_duration`.
  - A plain mode with extra words keeps today's behaviour: `/setwarnmode ban 1d` stores ban, and `/setfloodmode ban x` stores ban.
  - `/blacklistaction` keeps refusing two words whose first is not timed.
  - Success replies:
    - `/setwarnmode` uses `warns_mode_updated_tmute` or `warns_mode_updated_tban` with `{duration}`;
    - `/setfloodmode` and `/blacklistaction` reuse their `%s` success templates with the short form (`punishDurationShort`: `tmute 2h`);
    - `/blacklistaction` with no argument and `/warnings` show the short form for a timed action (`tmute 2h`);
    - `/flood` shows the command's English word, `for` and the short form (`muted for 2h`), the untranslated form it already uses.
- **D-11:** The end date. It is computed once, immediately before the restrict or ban request, by `punishUntil(punishNow(), secs)`, which
  wraps `extraction.TemporaryUntilDate`. No pacer or wait lies between the computation and the request. The same value goes into the request
  and into `{until}`. Right before the request, `punishUntilTooClose` refuses locally when the end is less than 35 s away (`punishMinLead`,
  the staff rule's lead). In that case nothing is sent, nobody is punished and a gated Warn line is logged, so a timed action can never become
  permanent. With the 60 s minimum this never fires in practice; it holds Telegram's 30 s rule by construction.
- **D-12:** Requests. A timed mute is `restrictChatMember` with `MutedPermissions` and `until_date`. A timed ban is `banChatMember` with
  `until_date`. Both are sent through the one helper `applyTimedPunishment(b, chatID, userID, action, secs)`. The untimed branches of every
  site keep their exact calls, so `TestLegacyPunishmentNoticeRequests` holds. **The bound, stated exactly:** these requests take no context,
  as today's do, so an in-flight request is bounded only by the bot's HTTP client timeout (`constants.LongTimeout`, 30 s). No plan, comment
  or `AGENTS.md` sentence may claim a shorter per-request bound for them.
- **D-13:** Sender chats. A channel identity or a linked channel's post cannot be restricted for a time: `banChatSenderChat` has no end date.
  A timed action therefore does nothing to a sender chat and posts nothing, exactly as a mute does to one today (P-08). The untimed sender-chat
  ban paths are unchanged.
- **D-14:** Warn limit. `applyWarn` keeps its signature (the member requirements' tests call it). `warnApplyResult` gains `Secs` and `Until`,
  and `applyWarnLimitAction` gains the seconds and applies `tmute` or `tban` through `applyTimedPunishment`.
  The member requirements' warn path uses `applyWarn` too, so a timed warn-limit action reaches it:
  - `memberReqNoticeKey` and the site's action word map `tmute` to the mute variant and `tban` to the ban variant (no new requirement text,
    P-07);
  - `memberReqEnforce` counts a `tban` limit as a removal, like `ban`.
- **D-15:** Default texts. New keys in all 8 languages, each with `{name}` placeholders: `warns_limit_tmuted`, `warns_limit_tbanned`,
  `antiflood_checkflood_timed_mute`, `antiflood_checkflood_timed_ban`, `blacklists_bl_watcher_tmuted_user` and
  `blacklists_bl_watcher_tbanned_user`.
  - They name the duration (`{duration}`), and the blacklist ones also the reason, escaped (unlike the untimed default's kept quirk).
  - They are read only for an effective timed action, so every untimed default and every inert default stays byte for byte.
  - The blacklist keys also serve the forward blocklist's notice of a ladder step.
- **D-16:** Event values. For an effective timed action:
  - `{duration}` is `punishDurationLabel(tr, secs)`, the group's language form built from the existing `staff_act_duration_*` keys, using
    the largest unit that divides the seconds evenly (`2 год`, `1 week(s)`) (P-05);
  - `{until}` is the end date in the chat's zone, through the engine's existing `formatChatTime`;
  - `{action}` is the mute word for `tmute` and the ban word for `tban` (`eventActionWords` gains the two entries, P-06).

  Every other event and every untimed action leaves `{duration}` and `{until}` empty, as today. `eventDefaultInput` gains `Secs`, which only
  the timed branches of the default builders read.
- **D-17:** Private copies. `eventPMSpec` gains `UntilLine`, set for `warnlimit`, `flood`, `blacklist` and `fwdblock`. When `UntilLine` is
  set and the value of `{until}` is set, `eventPMDefaultText` appends the new `eventmsg_pm_until` line (`Until: {until}`) after the action
  text. Every other spec, and every untimed copy, is unchanged. The captcha's mute PM copy is not affected (its spec has no `UntilLine`).
- **D-18:** Messages page.
  - `settingsMsgSample` gains `Secs`, set to 3600 for `warnlimit`, `flood`, `blacklist` and `fwdblock`, so a preview of a custom text fills
    `{duration}` through `punishDurationLabel` and `{until}` one hour ahead. The default previews keep their untimed sample actions.
  - `settings_msg_wait_text_auto` is reworded in 8 languages to explain `{duration}` and `{until}` for a timed mute or ban, and its
    `TestSettingsEditLocaleTokens` row follows.

### The three pages (TUNE-01, default)
- **D-19:** A shared duration screen, `fuku/modules/settings_duration.go`.
  - `settingsDurationPresets` holds 10 min, 1 h, 6 h, 12 h, 1 d, 3 d, 7 d and 30 d (600 to 2,592,000 s, P-04), laid out two to a row.
  - Each cell is `punishDurationLabel`, and the stored value is marked `✅`.
  - Back returns to the page or screen it came from; Close closes the menu.
  - `settingsDurationPreset(v)` accepts exactly the canonical decimal seconds of a preset and nothing else, so a forged duration is an
    invalid request in step 1.
  - A duration a command stored that is not a preset is drawn in the page's action line by its label, and no cell is marked.
- **D-20:** The Warnings page (`n`) gains the row `[Timed mute | Timed ban]`, labelled with the existing `settings_opt_tmute` and
  `settings_opt_tban` and marked while that timed mode is effective.
  - Its page actions are `nd` (open the duration screen, field `k` = `m` or `b`, right `settingsRightAdmin`) and `nt` (set, fields `k` and
    `v`, right `settingsRightAdminFloodBot`, the right of `/setwarnmode`).
  - The action line of an effective timed mode reads `settings_opt_timed` (`{action}, {duration}`). The same word is the Main state's
    `{action}`.
- **D-21:** The Antiflood page (`f`) gains the same row with `fd` and `ft`, right `settingsRightAdminFloodBot`. The Blacklist action screen
  gains it with `bd` and `bz`, right `settingsRightRestrictBotRestrict`, and both carry the private owner field `c`, like every list action.
  - The Blacklist page's entries carry `Secs` (`settingsListEntry.Secs`, zero for every other list page). An action is uniform only when the
    action and the seconds are equal on every word.
  - A new word takes the action and the seconds every word shares (`blacklistSharedAction` returns both; A19-3 extended), else warn and 0.
  - The Blocked sources page shares the blacklist screen helpers and stays unchanged in what it draws.
- **D-22:** Parity, drift and authority, for each of the three settings (D-09):
  - parity: a page press and the command leave the same row and cache;
  - drift: the page draws what the command stored, including a non-preset duration such as `90m`, and the punishment applies what the page
    stored;
  - authority parity: the menu refuses what the command refuses, with the owner gate's documented carve-out.

  Every changed screen has a worst-case callback test (64 bytes) and a length test in 8 languages. The timed choices also work in a menu
  opened in the group, like the rest of the Warnings and Antiflood pages (P-21); the Blacklist page is private only, as today.

### Escalation ladder (TUNE-02, default)
- **D-23:** What counts. An offence is a punishment of a real user (a positive ID that is neither a Telegram service account nor the bot) by
  the flood, the blacklist or the forward blocklist whose own action is effective and is a punishment: `mute`, `kick`, `ban`, or `tmute` or
  `tban` with a duration (P-09).
  - A warn action, delete only (`none`) and an inert action are not offences.
  - A sender chat is never counted and keeps its rule's own action (D-13).
  - The warn limit, member requirements, captcha and AI spam are not ladder sources (D-24).
- **D-24:** The warn limit is not a ladder source (P-09). It already escalates by counting warns, its action is shared with the member
  requirements' warns (`applyWarn`), and the roadmap gives the requirement and captcha punishments their own rules. The blacklist's and the
  forward blocklist's warn actions issue warns and so never count.
- **D-25:** Steps. A ladder has 0 to 5 steps (`escalation.MaxSteps`). A step is an action (`mute`, `tmute`, `kick`, `tban`, `ban`) and, for a
  timed action, a preset duration. Offence N gets step N, and the last step repeats for every later offence. A ladder that is on with no step
  does nothing (P-13).
- **D-26:** Counting, migration `20261122140000_add_escalation_ladder.sql`. It creates two tables:
  - `chat_offences` (`chat_id`, `user_id`, `offences` with CHECK >= 0, `last_offence_unix` BIGINT, primary key `(chat_id, user_id)`). It
    has no foreign key, is read only by the conditional upsert, is never cached and is never part of backup, export, import or reset.
  - `chat_escalation`, described under D-28.

  `escalation.CountOffence(ctx, chat, user, now, forget)` is one statement, `INSERT ... ON CONFLICT (chat_id, user_id) DO UPDATE ...
  RETURNING offences`, which works on PostgreSQL and on SQLite. Its rules:
  - An offence within `escalation.SameOffenceWindow` (60 s) of the member's last counted offence is the same offence. It is not counted
    and does not move the time, so an album, a burst or a member who keeps offending is counted once a minute at most.
  - An offence after the forget period restarts at 1.
  - Anything else adds 1.

  Times are unix seconds (BIGINT), so SQLite never compares text times. PostgreSQL's row lock makes concurrent offences on two replicas one
  count each (`TestOffenceCountConcurrent` joins `make test-postgres-integrity`).
- **D-27:** Forgetting. A count is forgotten after the period the owner picks, with no offence in between: 1, 7, 30 or 90 days, default 30
  (P-11). This happens by comparison inside the upsert, so no worker and no sweep are needed. Turning the ladder off deletes the group's
  counts in the same apply function (P-22), so turning it on again starts fresh. Rows of members who never offend again stay, bounded by the
  members ever punished while the ladder was on (accepted).
- **D-28:** Settings storage is `chat_escalation`, one row per chat. Its columns:
  - `enabled`;
  - `step1_action` to `step5_action` (`VARCHAR(8) NOT NULL DEFAULT ''`) and `step1_secs` to `step5_secs`, with inline named CHECKs: the
    action in `'' mute tmute kick tban ban`, the seconds 0 to 31,622,400;
  - `forget_days` (`NOT NULL DEFAULT 30`, CHECK 1 to 365);
  - `updated_by` and `updated_at`.

  A chat without a row is off. The model has no `default:` tag on the boolean. The writes `SetEnabled`, `SetEnabledWithSteps`, `SetStep` and
  `SetForgetDays` are column-targeted upserts that drop `chat_escalation:<chat>`. Reads go through `cache.GetFromCacheOrLoad` under
  `cache.CacheKey("chat_escalation", chat)` (`escalation.Get`, which creates nothing and returns its error; `GetFresh` is the same query
  without the cache). The prefix is listed in `skipLocal`, because it is read at every automatic punishment and a change must act on every
  replica at the next one. The table is never part of backup, export, import or reset.
- **D-29:** The resolver is `escalate(ctx, chatID, userID, source, p autoPunishment) autoPunishment` in `fuku/modules/escalation.go`. It is
  called after the rule decided to punish and after the bot's restrict right was checked, and before the request:
  - a ladder that cannot be read: the rule's own action (today's behaviour) and one gated Warn;
  - off, or no step: the rule's own action;
  - a count that cannot be established (an error or the 2 s `escalationCountTimeout`): step 1, the step a first offence gets, so the ladder
    never punishes harder than its own first step on a guess (P-14);
  - otherwise step `min(count, len(steps))`.

  The count statement runs under its own 2 s context, which GORM hands to the driver, so that bound is enforced. The cached read uses the
  update's context. There is no goroutine, no Redis key and no shutdown drain.
- **D-30:** Sites. `checkFlood`, `blacklistWatcher` and `forwardBlockEnforce` (inside its punishment latch, so an album counts once) call the
  resolver, then apply the resolved action through one helper each. That helper takes the untimed branch's exact request or
  `applyTimedPunishment`. Each site's notice (`floodNotice`, `blacklistNotice`, `forwardBlockNotice`, the engine's existing callers, whose
  names stay) takes an `autoPunishment`, so `{action}`, `{duration}` and `{until}` and the default text name the action that was applied. The
  flood's Unmute row follows a mute and a timed mute. `checkFlood` (gocyclo 31 at b7662c7) and `blacklistWatcher` (30) end at no higher
  complexity, because their action switch moves into the helper. `forwardBlockMute`, `forwardBlockKick`, `forwardBlockBan` and
  `forwardBlockWarn` keep their names and signatures (the golden calls them).
- **D-31:** The Escalation page, `fuku/modules/settings_escalation.go`.
  - Page key `j`, icon 🪜, `settings_page_esc`, `Order` 43 (after Warnings).
  - It is `PrivateOnly`, and `Hidden` in a Staff Group through its own fresh seam (`settingsEscStaffLookup`; an unreadable row shows the
    page), like the Messages page (P-15).
  - Its body: the intro (which rules count, the 60 s rule), the state, one line per step (`{n}. {action}`, the last one `{n} and later:
    {action}`), the forget line and the hint.
  - Its keyboard: the toggle, one row of step buttons `1` to `n`, `[➕ Add step | ➖ Remove last]`, the forget choice
    `[1 d | 7 d | 30 d | 90 d]`, then Back and Close, 5 rows.
  - A step screen offers `[Mute | Kick | Ban]` and `[Timed mute | Timed ban]`, each timed choice opening the shared duration screen.
  - Page actions, each requiring the private owner field `c`:

    | Code | What it does | Fields | Right |
    |------|--------------|--------|-------|
    | `je` | toggle | `v` 0 or 1 | `settingsRightRestrict` |
    | `jf` | forget days | `v` a preset | `settingsRightRestrict` |
    | `js` | open a step screen | `t` 1..5 | `settingsRightAdmin` |
    | `ja` | set an untimed step | `t`, `v` mute, kick or ban | `settingsRightRestrict` |
    | `jd` | open a step's duration screen | `t`, `k` m or b | `settingsRightAdmin` |
    | `jt` | set a timed step | `t`, `k`, `v` | `settingsRightRestrict` |
    | `jr` | remove the last step | `t`, which must equal the last step at apply time, else a stale notice | `settingsRightRestrict` |

    Every write judges `settingsRightRestrict`, the right of punishment choices; viewing and opening a screen need only the live admin check.
  - Turning the ladder on with no step stores the example ladder from the requirement (timed mute 1 h, timed mute 1 day, ban) in the same
    write, with a notice (P-13). Removing the only step while the ladder is on is refused with a notice.
  - Main state: Off, `{n} steps`, or the unknown mark.
  - The worst callback is `a=jt&c=<14-char chat>&k=m&t=5&v=2592000`, 51 bytes.
- **D-32:** The ladder's apply functions are in `fuku/modules/escalation_settings.go`: `applyEscalationEnabled`, `applyEscalationStep`,
  `applyEscalationRemoveStep` and `applyEscalationForget`. Each returns a typed result, judges no person's rights and logs IDs only. No
  command exists, so the page carries drift and authority tests in place of parity tests (D-38).

### Warn expiry (TUNE-03, default)
- **D-33:** Storage, migration `20261122130000_add_warn_expiry.sql`:
  - `warns_settings.warn_expiry_days INTEGER NOT NULL DEFAULT 0` (`chk_warns_settings_expiry`, 0 to 365; 0 is off);
  - `warns_users.warn_times JSONB NOT NULL DEFAULT '[]'`: the unix seconds of each stored warn, in the order of the reasons.

  The models gain `ExpiryDays` and `WarnTimes models.Int64Array`, with JSON `omitempty`. The migration backfills nothing.
- **D-34:** Old warns. A row saved before this phase has fewer times than reasons, so `(*models.Warns).NormalisedTimes` dates every missing
  time at the row's `updated_at`. That is never earlier than the warn's real time, so no old warn expires before its real age (P-18).
  - Missing times are padded at the front with `min(updated_at, the first stored time)`, and extra times are cut from the end, so the times
  stay non-decreasing.
  - A cached value of the previous build has no times and no update time; it counts every warn (it never expires one on a guess).
  - Rolling deploy: a replica on the old code that appends or removes a reason without a time can misalign the times for that member until
    their next warn change (accepted).
- **D-35:** Counting.
  - `(*models.Warns).Counted(expiryDays, now)` returns the counted reasons and times: with expiry off, every warn; with expiry on, the warns
    whose time is after `now - expiryDays*24h`. They are a suffix of the record, because the times are non-decreasing.
  - Every write (`WarnUserAt`, `RemoveWarnAt`) drops the expired warns from the record in its transaction, so a record never grows without
    bound, and `num_warns` stays the stored length.
  - Raising the expiry later never brings back a warn already dropped, and with expiry off nothing is ever dropped (P-17).
  - `WarnUserAt` counts and returns the counted warns, so the limit compares the counted number.
  - `RemoveWarnAt` removes the newest counted warn and answers removed only when one counted.
  - `ResetUserWarnsAt` deletes the row and answers removed only when a warn counted.
  - `GetWarnsAt` (`/warns`, the AI spam warn count) reads the settings through `LoadWarnSettingsContext`, which creates no row, and returns
    the counted warns.
  - The old names (`WarnUser`, `RemoveWarn`, `ResetUserWarns`, `GetWarns`, `GetWarnsContext`) stay as wrappers on `time.Now()`.
- **D-36:** What the commands show.
  - `/warns` shows the counted number and reasons, plus the new line `warns_expiry_note` only while expiry is on.
  - `/warnings` adds `warns_settings_expiry` only while expiry is on.
  - A group with expiry off sees byte-identical replies (the plan 26-01 goldens).
  - `/resetallwarns` keeps counting rows to decide whether to show its confirm card (accepted).
- **D-37:** Backup.
  - The warns export carries the counted warns only (`num_warns`, the reasons and `warn_times`, filtered with the group's expiry and the
    current time), so a group importing them counts what the source counted. With expiry off, every warn is exported.
  - An import keeps `warn_times` only when there is one time per reason, each positive, non-decreasing and not in the future. Otherwise it
    drops them, and the warns are then dated at the import.
  - Durations (D-06) and the expiry are exported with JSON `omitempty`. An import stores 0 for a duration outside the bounds, for a timed
    mode with no duration or for an expiry outside 0 to 365, and never refuses a file over them.
  - The ladder and the counts are never exported, imported or reset (`TestEscalationStaysOutOfBackup`).
- **D-38:** No command for the expiry or the ladder. The Warnings page's expiry screen (`ne` to open, right `settingsRightAdmin`; `nx` to set,
  field `v` in `0 1 3 7 14 30 90` days, right `settingsRightAdminFloodBot`) writes through `applyWarnExpiry(ctx, chatID, days)` in `warns.go`.
  It also works in a menu opened in the group. The Warnings page shows the expiry line and a button `🕒 {state}` (P-16, P-20).

### Clock, workers, shutdown (default)
- **D-39:** One clock seam, `punishNow` (`time.Now`), serves the end dates, the ladder and the warn expiry, so a test can move time.
- **D-40:** No worker, goroutine, Redis key family, handler group, callback namespace or shutdown drain is added. Telegram ends a timed
  restriction itself, warns expire by comparison at every read and write, and offence counts age out by comparison. The 70 s webhook-mode worst
  case and `stop_grace_period: 75s` in `AGENTS.md` are unchanged.

### Security and privacy (default)
- **D-41:** No new screen, notice, alert, log line or callback data names the Staff Group, its title or ID, another group or the log channel.
  A ladder step applied to a forward blocklist offence is the linked group's own action. Its notice names neither the source nor the Staff
  Group, as today.
- **D-42:** No log line of these paths carries a reason, a blacklisted word, a name or a stored text. Lines carry chat and user IDs, the
  action, the step, the count and an error class only. The count and resolver failures go through an hourly per-chat gate
  (`escalationWarnGates`, the type `memberDeleteWarnGate`).
- **D-43:** gosec runs in CI, and G101 flags any constant or variable holding a string whose name contains token, secret, password, pass, pwd,
  cred or bearer. No new identifier uses those words (the short duration form is `punishDurationShort`). Every plan that changes production code
  runs `/root/go/bin/gosec -quiet ./fuku/...` and shows no finding in a file it touched (the baseline is recorded by plan 26-01).

### Tests (default)
- **D-44:** Goldens (plan 26-01). There are two files, committed together and alone.
  - `fuku/modules/timed_punishment_legacy_request_test.go` (`TestLegacyTimedPunishmentRequests`) covers:
    - `/setfloodmode` and `/flood`;
    - `/setwarnmode tmute` alone, `/warnings`;
    - `/blacklistaction tmute` and `tban` alone and its current mode;
    - `/warns`, `/rmwarn`, the Remove warn button and `/resetwarns`;
    - the inert timed settings of the warn limit, the flood, the blacklist and a member requirement warn reaching an inert limit.
  - `fuku/db/backup/punish_export_legacy_test.go` (`TestLegacyPunishmentExport`) pins the export JSON of the warns, antiflood and blacklists
    modules for today's rows.
- **D-45:** Test time. The full `fuku/modules` run takes about 6 minutes, so the phase is budgeted at about 35 s of added `fuku/modules` test
  time, shared out in each plan. Each plan records its measured time in its SUMMARY. Tests in `fuku/db/*` run in `test-rest` and do not count.
  Every new test is light:
  - worst cases, never cross products;
  - language sweeps over pure renders only;
  - one environment per dispatcher subtest;
  - real locales only where a default text is compared;
  - fresh chat and user IDs, cleanup in `t.Cleanup`;
  - no `t.Skip` (except the PostgreSQL-only migration tests), no `t.Parallel`, no mock library.
- **D-46:** PostgreSQL integrity. These join `make test-postgres-integrity`, and `./fuku/db/escalation` joins its package list:
  - `TestTimedPunishmentMigrationDefaults`, `TestWarnExpiryMigrationDefaults`, `TestEscalationMigrationDefaults` (`./fuku/db/migrations`);
  - `TestSetWarnModeDuration`, `TestWarnExpiryWrites` (`./fuku/db/warns`);
  - `TestEscalationSettings`, `TestOffenceCountConcurrent` (`./fuku/db/escalation`).

  The database-free halves (`...MigrationStatements`) run in `make test`.

### Repo rules that bind
- **D-47:** New locale keys go in all 8 files.
  - `settings_` keys get rows in `settingsLocaleLimits`, and new cells get groups in `TestSettingsLabelLengths` (a cell is at most 20 code
    points).
  - `eventmsg_pm_until` gets a row in `eventMsgLocaleLimits`.
  - Ukrainian and English follow `locale-style.md` (`TestLocaleStyle`), placeholders match across languages
    (`TestLocalePlaceholdersMatchOtherLanguages`), HTML only.
  - The pinned texts (`warns_specify_action`, `warns_mode_unknown`, `antiflood_setfloodmode_specify_action`,
    `antiflood_setfloodmode_unknown_type`, `blacklists_set_bl_action_choose_correct_option`, `warns_settings_display`,
    `warns_user_warnings_list`, `antiflood_flood_show_settings` and every default notice) are never edited.
  - The new forms are explained in `warns_help_msg`, `antiflood_help_msg` and `blacklists_help_msg` (`TestHelpPagesKeepTheirCommands` still
    passes), followed by `make generate-docs`.
- **D-48:** `AGENTS.md` is updated in the same commit as the change it describes. The touched sections:
  - Data: the new columns and tables, `skipLocal`, backup;
  - Subsystem traps: timed actions, the ladder, warn expiry, Antiflood first write;
  - Callbacks: the Warnings, Antiflood, Blacklist and Escalation pages, the Main grid, the Messages page sample;
  - Permissions: the rights of the new controls;
  - Testing: the families, the PostgreSQL list;
  - Shutdown: the sentence that nothing is added.
- **D-49:** Every new production identifier has a production caller in the plan that adds it, or a dated
  `//nolint:unused // first production caller arrives in plan 26-NN` that the named plan removes. Lint gates:
  - golangci-lint v2 (`/root/go/bin/golangci-lint`, `CGO_ENABLED=0`) `run --new-from-rev=origin/main ./...`, and the same with
    `--build-tags=notesttools`;
  - `run --new=false` filtered to the touched files.

  Baseline at b7662c7, recorded by the planner, in the files this phase touches: `gocyclo` of `checkFlood` 31, `blacklistWatcher` 30,
  `addBlacklist` 23 and `clearModuleData` 21; `dupl` of `rmAllBlacklists`; `unused` `checkWarns`. The gate allows those at no higher
  complexity and nothing new.

### Planner's and Executor's Discretion
Within the prefixes above, the executor chooses:
- function, field and test names;
- the exact strings, inside their limits and the style guide;
- the order of lines inside a screen body;
- how a site's dispatch helper is shaped;
- how a test's cases are split into subtests.

</decisions>

<open_questions_resolved>
## Roadmap Planning Notes and Open Questions, Resolved

| Question | Resolution |
|----------|------------|
| tban/tmute pass the checks but nothing applies them | D-07 to D-09: they act with a valid duration; with none they stay inert, exactly as today |
| Commands and pages through one apply function, goldens not re-recorded | D-09, D-10, D-22; the bare timed word keeps its pinned reply |
| Duration grammar and the 30-second rule | D-05, D-11: `ParseDurationToken`, computed once right before the request, refused locally under 35 s |
| Where offence counts live | D-26: PostgreSQL, one conditional upsert, shared by every replica, never cached, never in backup |
| Which automatic sources count | D-23, D-24: flood, blacklist, forward blocklist punishments; not warn limit, requirements, captcha, AI spam |
| When a count resets | D-26, D-27: offences within 60 s are one; forgotten after 1/7/30/90 days (default 30); cleared when turned off |
| Warn expiry needs a per-warn time | D-33, D-34: `warn_times` jsonb, old rows dated at their last change |
| `/warns`, `/rmwarn`, `/resetwarns`, backup and the limit agree | D-35 to D-37: one pure `Counted` rule in the model, used by every reader and writer |
| Expiry worker and shutdown | D-40: none; comparison at read and write |

</open_questions_resolved>

<screens_and_codes>
## Screens and Callback Codes

New page action codes, all two letters and checked free at b7662c7. `registerSettingsPage` panics at init on a duplicate, and
`TestSettingsDeletionMemberCodesUnique` holds them apart from the Deletion row codes.

| Page | Code | Fields | Right | What |
|------|------|--------|-------|------|
| Warnings `n` | `nd` | `k` (m, b) | admin | duration screen of a timed mode |
| Warnings `n` | `nt` | `k`, `v` (a preset) | admin + bot admin (`/setwarnmode`'s) | set the timed mode |
| Warnings `n` | `ne` | none | admin | expiry screen |
| Warnings `n` | `nx` | `v` (0 1 3 7 14 30 90) | admin + bot admin | set the expiry |
| Antiflood `f` | `fd` | `k` | admin | duration screen |
| Antiflood `f` | `ft` | `k`, `v` | admin + bot admin (`/setfloodmode`'s) | set the timed action |
| Blacklist `b` | `bd` | `c`, `k` | admin | duration screen |
| Blacklist `b` | `bz` | `c`, `k`, `v` | the ninth right (`/blacklistaction`'s) | set the timed action for every word |
| Escalation `j` | `je` `jf` `js` `ja` `jd` `jt` `jr` | as D-31 | as D-31 | the ladder |

Row shapes:
- Warnings page `[5 3 2 1 2]`.
- Antiflood page `[5 3 2 1 2]`.
- Blacklist action screen `[3 2 2 2]` with words.
- Duration screen `[2 2 2 2 2]`.
- Expiry screen `[2 2 2 1 2]`.
- Escalation page `[1 n 2 4 2]` with `n` from 0 to 5, and the step-button row is left out when `n` is 0.
- Step screen `[3 2 2]`.

Main grid outside a Staff Group: `s|l / c|f / n|j / k|b / i|w / g|r / e|x / h|m / d|t / u|z / o|y`, then Conversations and Access for the
owner. It is still 14 rows for the owner and 12 for an administrator. A Staff Group's Main is unchanged, because the Escalation page is
hidden there.

</screens_and_codes>

<code_anchors>
## Code Anchors (at b7662c7)

- `fuku/modules/warns.go`:
  - `warnSettingsModes` (46), `warnSettingsWrites` (79), `applyWarnLimit` (96), `applyWarnMode` (108), `setWarnMode` (119);
  - `warnApplyResult` (165), `applyWarn` (189), `applyWarnLimitAction` (234);
  - `warnThisUser` (284), `warnLimitEventReply` (415), `warnPostLimit` (429);
  - `warnings` (523), `warns` (553), `rmWarnButton` (628), `resetWarns` (765), `removeWarn` (952).
- `fuku/db/warns/repository.go`: `WarnUser` (90), `RemoveWarn` (183), `ResetUserWarns` (232), `GetWarnsContext` (249), `SetWarnMode` (284),
  `LoadWarnSettingsContext` (332). The models are in `fuku/db/models/warns.go`.
- `fuku/modules/antiflood.go`: `checkFlood` (245; the action switch 344 to 399), `floodNotice` (413), `floodModes` (450), `applyFloodMode`
  (511), `flood` (579), `setFloodMode` (618). `fuku/db/antiflood/optimized.go` holds `GetAntifloodSettingsContext`, whose explicit `Select`
  must gain `action_secs`. `fuku/db/antiflood/repository.go` holds `SetFloodModeContext` (71).
- `fuku/modules/blacklists.go`: `blacklistActionValues` (39), `blacklistWrites` (52), `blacklistSharedAction` (165), `applyBlacklistAdd`
  (186), `applyBlacklistAction` (271), `setBlacklistAction` (512), `blacklistWatcher` (676; the switch 729 to 762), `blacklistNotice` (780).
  `fuku/db/blacklists/repository.go` holds `AddBlacklistContext` (25) and `SetBlacklistActionContext` (85).
- `fuku/modules/staff_forwardblock_watch.go`: `forwardBlockEnforce` (300), `forwardBlockPunishments` (335). `fuku/modules/forwardblock_notice.go`
  holds `forwardBlockNotice`.
- `fuku/modules/event_message_auto.go`: `warnLimitKeys`, `warnLimitDefaultReply` (57), `floodWords`, `floodDefaultReply` (87),
  `blacklistNoticeDefaultReply` (110), `eventActionWords`, `eventActionWord` (144), `listNoticeSite` (209), `listNoticeReply` (226).
- `fuku/modules/event_message.go`: `eventValues` (46), `eventDefaultInput` (68).
- `fuku/modules/event_message_pm.go`: `eventPMSpec` (37), the automatic registrations (83 to 86), `eventPMDefaultText` (410).
- `fuku/modules/memberreq_notice.go`: `memberReqNoticeKey` (113), `memberReqNoticeWarnOf` (173), the site's action switch (355 to 375).
  `fuku/modules/memberreq_enforce.go` has the warn branch (440 to 463).
- Settings:
  - `settings_warns.go`; `settings_antiflood.go` (`settingsFloodSettable`, `renderSettingsAntiflood`);
  - `settings_blacklist.go` (`renderSettingsBlacklistAction` 196, `settingsBlacklistActions` 277);
  - `settings_list.go` (`settingsListEntry` 52, `settingsListPrivateField` 266);
  - `settings_messages.go` (`settingsMsgSample` 87, `settingsMsgAutoEvents` 181, the preview values 948, `settingsMsgHidden` 699);
  - `settings_pages.go` (`settingsPage` 350, `settingsPageAction` 407, `settingsChoiceRow` 314);
  - `settings_main_test.go` (`settingsMainShapeGroup`, `settingsMainShapePrivate`, `settingsMainOwnerRows`).
- Helpers:
  - `fuku/utils/extraction/extraction.go`: `TemporaryUntilDate` (43), `ParseDurationToken` (76);
  - `fuku/modules/staff_action_summary.go`: `staffDurationLabel` (the `staff_act_duration_*` keys);
  - `fuku/modules/staff_action_run.go`: `staffUntilMinLead` (41);
  - `fuku/modules/chat_clock.go`: `formatChatTime`.
- Backup: `fuku/db/backup/backup.go` has `exportWarnsData` (471), `importWarns` (879), `importAntiflood` (604) and `importBlacklists` (655).
  `roundtrip_test.go` has `TestAllModulesRoundTripEveryMeaningfulField`, which seeds explicit values.
- The cache is in `fuku/db/cache/local.go` (`skipLocal`). The test harness: `fuku/modules/test_harness_test.go` (the AutoMigrate list),
  `fuku/db/warns/repository_test.go` (`TestMain`), `fuku/db/backup/testmain_test.go`.
- Goldens and toolkits to reuse by name:
  - `rules_legacy_request_test.go` `legacyRequestJSON`, `warns_legacy_request_test.go` `withRealLocales`;
  - `punishment_notice_legacy_request_test.go`: `newNoticeGoldenSite`, `noticeGoldenTarget`, `noticeGoldenMessage`, `noticeGoldenWarn`,
    `noticeGoldenFlood`, `noticeGoldenBlacklist`;
  - `screening_notice_legacy_request_test.go`: the requirement golden environment and token helper;
  - `newModuleBotClient`, `newModuleTestBot`, `newModuleMessageContext`, `resetAntifloodState`;
  - `settings_env_test.go`: `newSettingsEnv`, `newSettingsEnvWith`, `otherReplica`;
  - `staff_forwardblock_env_test.go` `newFwdWatchEnv`, `messages_e2e_test.go` `newMessagesE2E`, `deletion_e2e_test.go`
    `deletionShortestRules`.

</code_anchors>

<canonical_refs>
## Canonical References

- `AGENTS.md`:
  - Data (`warns_settings`, `chat_requirements` for the column pattern, `skipLocal`, backup rules);
  - Subsystem traps: Antiflood first write (D-27 of Phase 18), the event messages and PM copies bullets, the forward blocklist watcher, Scheduling
    the bot's own messages, the member requirements executor;
  - Callbacks (the Warnings, Antiflood, Blacklist and Messages pages, the Main grid);
  - Permissions (the settings rights);
  - Migrations; Testing.
- `.planning/phases/25-lockdown-messages/` and `.planning/phases/24-captcha-ai-spam-and-member-requirement-notices/` (the closest models and
  the Phase 24 advisory on per-send bounds); `.planning/phases/23-warn-flood-blacklist-and-forward-blocklist-messages/` (the sites this phase
  extends).
- `.planning/ROADMAP.md`, the Phase 26 planning notes and success criteria; `.planning/REQUIREMENTS.md`, TUNE-01 to TUNE-03.

</canonical_refs>

<owner_visible>
## Behaviour an Owner Would Notice

Nothing changes for a group that sets nothing. When the owner sets things:
- the Warnings, Antiflood and Blacklist pages offer Timed mute and Timed ban with a duration from 10 minutes to 30 days, and the commands take
  any duration from 1 minute to 366 days (`/setwarnmode tmute 2h`);
- a timed punishment restricts the member until the end date, and the group message says how long, through `{duration}` and `{until}` in a
  custom text. The private copy adds an "Until" line;
- the new Escalation page (private only) turns on a ladder: the 1st, 2nd and later flood, blacklist or forward blocklist punishments of a member
  get the owner's steps. Offences within a minute count once, and counts are forgotten after the chosen period;
- the Warnings page can make warns expire after 1 to 90 days. An expired warn stops counting, and `/warns` shows only what counts.

</owner_visible>

<provisional>
## Provisional Decisions for Owner Confirmation

| Id | Decision | Where | Alternative the owner may prefer |
|----|----------|-------|----------------------------------|
| P-01 | A duration is 1 minute to 366 days; a longer one is refused, never turned into a permanent action | D-05 | treat over 366 days as permanent, like the staff commands |
| P-02 | A timed word with a bad duration gets a new short reply; a timed word alone keeps today's pinned "unknown" reply | D-10 | a reply that also lists the timed forms (impossible without editing pinned texts) |
| P-03 | Durations are stored per setting: one for the warn limit, one for the flood, one per blacklist word (shared by every word) | D-06, D-21 | one per-group duration for all three |
| P-04 | Pages offer 8 presets (10 min, 1 h, 6 h, 12 h, 1 d, 3 d, 7 d, 30 d); any other duration only by command | D-19 | a typed duration on the page |
| P-05 | `{duration}` of an automatic event is the group's language form (`2 год`), not the English form `/tban` uses | D-16 | the English form, for consistency with `/tban` |
| P-06 | `{action}` of a timed mute or ban is the mute or ban word; the time is in `{duration}` and `{until}` | D-16 | separate "muted for a time" words |
| P-07 | A member requirement warn reaching a timed limit uses the existing mute or ban notice; a timed ban counts as a removal | D-14 | new requirement notices naming the duration |
| P-08 | A timed action does nothing to a channel identity (no notice); the ladder never counts one | D-13, D-23 | a permanent sender-chat ban for a timed ban |
| P-09 | Ladder sources are flood, blacklist and forward blocklist punishments; the warn limit is not one | D-23, D-24 | count reaching the warn limit as an offence too |
| P-10 | Counts live in PostgreSQL, per member per group, never in backups | D-26 | Redis with a TTL |
| P-11 | Offences within 60 s count once; a count is forgotten after 1, 7, 30 or 90 days without an offence (default 30) | D-26, D-27 | another window, other periods, never forgotten |
| P-12 | Up to 5 steps; actions mute, timed mute, kick, timed ban, ban; the last step repeats | D-25 | warn as a step action, more steps |
| P-13 | Turning the ladder on with no steps adds the example ladder (timed mute 1 h, timed mute 1 day, ban); removing the only step while on is refused | D-31 | refuse "on" until a step exists |
| P-14 | An unreadable ladder applies each rule's own action; an unreadable count applies step 1 | D-29 | the rule's own action in both cases |
| P-15 | The Escalation page: key `j`, 🪜, after Warnings, private only, hidden in a Staff Group | D-31 | another icon or place; shown in a Staff Group |
| P-16 | Warn expiry presets Off, 1, 3, 7, 14, 30, 90 days, page only | D-38 | a `/setwarnexpiry` command, other presets |
| P-17 | An expired warn is dropped from the record at the member's next warn change; raising the expiry never brings it back | D-35 | keep every warn and only filter |
| P-18 | A warn saved before this phase is dated at its record's last change | D-34 | treat old warns as never expiring |
| P-19 | Backups carry durations, the expiry and warn times (counted warns only); the ladder and counts are not exported | D-37 | export every stored warn |
| P-20 | `/warnings` and `/warns` show the expiry only while it is on | D-36 | always show it (changes today's replies) |
| P-21 | The timed choices and the expiry screen also work in a menu opened in the group | D-22, D-38 | private only |
| P-22 | Turning the ladder off clears the group's counts | D-27 | keep counts until they age out |
| P-23 | A timed mute replaces the member's whole restriction (like today's mute), and when it ends the member has the group's default rights | D-12 | refuse a timed mute over another restriction (a live read per punishment) |
| P-24 | The new timed default texts escape the blacklist reason (the untimed default splices it as stored, a kept quirk) | D-15 | splice it as stored, for parity |

</provisional>

<deferred>
## Deferred Ideas

- a note on the Antiflood and Blacklist pages that the ladder replaces their action;
- an `{offence}` placeholder (the member's offence number);
- the warn limit as a ladder source;
- timed actions for the Staff Group's forward blocklist list;
- typed custom durations on the pages;
- a command for the ladder or the expiry;
- the ladder in backups and in Phase 29's copy settings (Phase 29 decides).

</deferred>

---
*Phase: 26-timed-punishments-escalation-and-warn-expiry*
*Context gathered: 2026-11-22*
