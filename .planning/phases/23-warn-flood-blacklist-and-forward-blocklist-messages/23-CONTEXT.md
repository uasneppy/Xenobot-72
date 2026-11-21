# Phase 23: Warn, Flood, Blacklist and Forward Blocklist Messages - Context

**Gathered:** 2026-11-21
**Status:** Ready for planning. The decisions marked "owner" were settled before this phase (the milestone decisions of 2026-11-18, the
roadmap's planning notes and success criteria for Phase 23, and the requirements EVT-02 and EVT-03) and are not reopened. The decisions
marked "default" are the planner's provisional defaults, taken from the documents and the code without asking the owner, because the owner
authorized this run to proceed autonomously; they are reversible, and the ones an owner would notice are listed under "Provisional decisions
for owner confirmation" at the end.

<domain>
## Phase Boundary

Phase 23 gives five automatic or rule-driven punishment replies their own custom message, through the event message engine of Phase 21
and the private copy engine of Phase 22:

- **Warn** (EVT-02): every warning `warnThisUser` posts below the warn limit: `/warn`, `/dwarn`, `/swarn`, the blacklist's warn action and the
  forward blocklist's warn action. The custom message keeps the bot's Remove warn button, added after any custom buttons.
- **Warn limit** (EVT-02): the message `warnThisUser` posts when a member reaches the limit and the limit action (kick, mute, ban) took effect,
  with `{action}` naming what was done.
- **Flood action, blacklist action, forward blocklist action** (EVT-03): the notices of `checkFlood`, of the blacklist watcher and of
  `forwardBlockNotice`, with `{action}` and `{reason}`. The forward blocklist message never names the blocked source or the Staff Group.

The Messages page (Phase 21) gains its second family, **Automatic**, with these five events; each opens the editor with a group message, a
private copy (Phase 22) and a Silent switch. Phase 24 appends captcha and AI spam events to the same family.

A group with nothing stored gets today's text for all five, byte for byte (MSG-11, proved by goldens recorded first). A refused custom
message falls back to the default (MSG-04). The message keeps its Bot messages timer (MSG-09). The punishment always comes first.

Out of scope: timed punishments, an escalation ladder and expiring warns (Phase 26, which will fill `{duration}` and `{until}` for these
events), the captcha, AI spam and requirement notices (Phase 24), lockdown messages (Phase 25), the appeal button (Phase 27), plain tone and
copy settings (Phase 29), custom messages in backups (Phase 30, MSG-10), a custom message per language (MSG-F1), and any change to the warn,
flood, blacklist or forward blocklist logic itself.

</domain>

<decisions>
## Implementation Decisions

### Scope rules (owner)
- **D-01 (owner):** Everything is configured group by group in that group's settings in the bot's private chat. There are no Staff Group
  defaults and nothing is inherited; the forward blocklist's message is set in each linked group's own Messages page, never in the Staff Group
  that keeps the source list. The Messages page is the only door: no command sets a message.
- **D-02 (owner):** A custom text is one text for every language. With no row (or a row with no text, media or buttons and Silent off), the
  bot sends today's default reply in the group's language.
- **D-03 (owner):** The punishment comes first. A custom message, a private copy, a Details button or a log line never delays, undoes or skips
  a warn, a warn reset, a mute, a kick or a ban. A custom message Telegram refuses is replaced by the default, sent once (MSG-04).
- **D-04 (owner, roadmap):** The five events are warn, warn limit, flood action, blacklist action and forward blocklist action. This phase
  creates the Automatic family on the Messages page; Phase 24 adds more events to it.
- **D-05 (owner, roadmap):** The warn message keeps its Remove warn button, added by the bot after any custom buttons. A custom message cannot
  remove or replace it.
- **D-06 (owner, roadmap):** The forward blocklist reason and notice name neither the source nor the Staff Group, and no placeholder adds
  them. The notice stays in `forwardblock_notice.go` (not a `staff*` file), because `TestMemberDeletionExcludedMessages` forbids a deletion
  producer in a staff file.
- **D-07 (owner, roadmap):** Private copies of these events follow Phase 22's rules, including the per-group cap for automatic events: a
  flood of offenders produces at most the capped number of private copies a minute, and every punishment still happens. No private copy is
  ever sent for a Staff Group action or a raid-scale removal.
- **D-08 (owner, MSG-11):** The first plan records characterization goldens of every send site this phase changes and commits them alone,
  before any production change. They are never re-recorded or weakened.

### Which sends are events (default)
- **D-09:** Event keys, validated in Go by `eventmsg.ValidEvent` (appended to `eventmsg.Events`, no migration, no CHECK): `warn`
  (`eventmsg.EventWarn`), `warnlimit` (`EventWarnLimit`), `flood` (`EventFlood`), `blacklist` (`EventBlacklist`) and `fwdblock`
  (`EventForwardBlock`). The Messages page codes (the `k` field) are `wn`, `wl`, `fl`, `bl` and `fb`, so the wait keys are `hwng`, `hwlg`,
  `hflg`, `hblg`, `hfbg` (group) and `hwnp` ... `hfbp` (private copy).
- **D-10:** What each event covers (provisional where marked P1):
  - `warn`: the one message `warnThisUser` sends below the limit, for `/warn`, `/dwarn`, `/swarn` (it deletes the command but posts the
    warning today, so it is not a silent variant in EVT-01's sense; P1), the blacklist's warn action and the forward blocklist's warn action.
    The source (`warnSource`) decides the deletion kind, the private copy cap and `{admin}`.
  - `warnlimit`: the message `warnThisUser` sends at the limit when the limit action took effect with a known mode (kick, mute, ban). A stored
    mode the code applies nothing for (`tban`, `tmute` or the empty mode, which the table's check allows and only a database edit can store)
    keeps today's reply exactly and is not an event (P1). A limit action Telegram refused posts nothing, as today.
  - `flood`: the notice `checkFlood` posts after a mute, a kick or a ban (of a user, or of a sender chat). A stored inert action (`warn`, `tban`,
    `tmute`) applies nothing and keeps today's notice exactly; it is not an event (P1).
  - `blacklist`: the mute, kick and ban notices of the blacklist watcher. Its warn action is the `warn` event; `none` and the inert actions post
    nothing, as today.
  - `fwdblock`: the mute, kick and ban notices of the forward blocklist watcher (`forwardBlockNotice`). Its warn action is the `warn` event (with
    the reason `staff_fwdblock_reason`); `none` posts nothing, as today.
  - Not events: the admin-target refusal of `warnThisUser` (`warns_admin_warning_error`), its `common_settings_save_failed` answer, every
    command reply of the warn, antiflood, blacklist and forward blocklist modules, and every Staff Group message.

### Placeholders (default)
- **D-11:** The Welcome placeholders (`{first}`, `{last}`, `{fullname}`, `{username}`, `{mention}`, `{id}`, `{chatname}`, `{rules}`) are filled
  for the punished member and the group, as in Phase 21. The event placeholders take these values (empty means the placeholder renders as
  nothing, never raw; MSG-02); every value from a user is escaped:

  | Placeholder | warn | warnlimit | flood | blacklist | fwdblock |
  |-------------|------|-----------|-------|-----------|----------|
  | `{reason}` | the warn's reason: the typed one, or the blacklist's or forward blocklist's reason for an automatic warn | the reason of the warn that reached the limit (P4) | `eventmsg_reason_flood`, new and translated (P4) | the reason the default notice shows: the stored reason filled with the matched word (P3) | `staff_fwdblock_reason` |
  | `{admin}` | the issuer (D-12) | the issuer of the warn that reached the limit | "an admin" | "an admin" | "an admin" |
  | `{action}` | `eventmsg_action_warn` | the word of the limit action | the word of the flood action | the word of the list action | the word of the list's action |
  | `{count}` | the member's warns after this one | the member's warns | the messages counted in the burst | empty | empty |
  | `{limit}` | the group's warn limit | the group's warn limit | the flood limit | empty | empty |
  | `{window}`, `{duration}`, `{until}` | empty | empty | empty | empty | empty |

  Every punishment of these five events is permanent today, so `{duration}` and `{until}` are empty until Phase 26 adds timed actions; the flood
  counter has no fixed window (it resets after 60 s of quiet), so `{window}` is empty. `{count}` is never the member count (Phase 21's rule).
- **D-12:** `{admin}` is the acting admin's escaped full name for `/warn`, `/dwarn` and `/swarn` (the translated `eventmsg_admin_anon` for an
  anonymous admin, also after the proof tap, through `actorIsAnonymous`); for an automatic action (the blacklist's or forward blocklist's warn,
  a limit those warns reach, a flood, blacklist or forward blocklist punishment) it is the translated `eventmsg_admin_anon` (MSG-03: "an admin"
  for an automatic action; P2). It never names the Staff Group.
- **D-13:** `{action}` is a translated word in the group's language: four new keys `eventmsg_action_warn`, `eventmsg_action_mute`,
  `eventmsg_action_kick` and `eventmsg_action_ban`, each a short participle that reads after the member's name or after a label ("Action:
  muted"), so no language needs to inflect it inside a sentence. The default texts are unchanged: the flood's default keeps splicing today's
  untranslated English word (`muted`, `kicked`, `banned`), pinned by the goldens.
- **D-14:** `{reason}` per event as in the table. The blacklist's default reason today is the stored reason `Blacklisted word: '%s'` filled
  with the matched word and spliced unescaped into the default notice (pinned, not fixed); `{reason}` is the same text, escaped (P3). The flood
  has no reason today; `eventmsg_reason_flood` is new (P4).

### Keyboard, place, timer, fallback, Silent (default)
- **D-15:** Keyboards of a custom group message:
  - warn: the custom buttons and the `{rules}` button, then the bot's Remove warn row last (D-05; `ReservedButtons` 1, so the editor counts it
    against Telegram's 100). The default's own Rules button (shown when the group's rules are set) is not kept on a custom message: the owner
    adds `{rules}` where wanted (P5).
  - warnlimit: no bot row (the default carries an empty keyboard).
  - flood: the default's admin row (Unmute for a mute, Unban for a banned sender chat) is kept first, as Phase 21 keeps the Unban row of a ban
    (`ReservedButtons` 1; P5).
  - blacklist and fwdblock: no bot row.
  - A private copy never carries a bot row (they are group callbacks). The Details button keeps the rows the group message was sent with.
- **D-16:** A custom message is posted exactly where today's default goes (P6): warn and warnlimit as a reply to the triggering message (the
  command, or the member's own message for an automatic warn) with `allow_sending_without_reply`; flood in the message's thread with no reply
  (today's `message_thread_id`); blacklist and fwdblock with no reply and no thread.
- **D-17:** Deletion kinds stay as they are (MSG-09): warn and warnlimit by the warn's source (`bot:punish_manual` for `/warn`, `/dwarn`,
  `/swarn`; `bot:punish_auto` for the blacklist's and forward blocklist's warns); flood, blacklist and fwdblock `bot:punish_auto`. A custom
  message, and the default that replaces a refused one, are queued under the same kind; Silent queues nothing.
- **D-18:** Fallback (MSG-04): a custom message Telegram definitely refuses (a 4xx other than 429, or `media.ErrNoPermission`) is replaced by
  the default, sent once through the site's own default send; a 429, a 5xx or a transport error is returned as the default's send error is
  today. Each site keeps its own error log line and return value (`warnThisUser` returns the error, `checkFlood` returns it, the blacklist
  watcher returns it, `forwardBlockNotice` returns it to the watcher's punishment log). The flood's default keeps going through
  `helpers.SendMessageWithErrorHandling` (its restricted-chat skip included).
- **D-19:** Silent posts nothing in the group for that event: the warn is stored, the limit action and the warn reset happen, the member is
  muted, kicked or banned. A Silent warn has no Remove warn button (admins use `/rmwarn`). The private copy is still offered (Phase 22 D-11).

### Engine (default)
- **D-20:** The engine becomes generic. `eventSite` describes where and how a site posts one reply (`Bot`, `Chat`, `Tr`, `Actor`, `ReplyTo`,
  `ThreadID`, `Kind`, `Automatic`, `SendDefault`), and `postEvent(site, r)` runs the one read, the group step and the private copy offer for any
  site. `(c *moderationCtx) replyEvent` becomes a thin wrapper that builds the moderation site; its requests stay byte-identical
  (`TestLegacyModerationRequests`). `replyEventGroup` becomes `postEventGroup(site, r, msgs)`, `offerReplyPM` takes the site, and
  `sendDefaultPost` becomes the site's `SendDefault` followed by `scheduleBotMessage(sent, site.Kind)` (nil-safe).
- **D-21:** `eventReply` gains `KeepRows` (the bot rows a custom message keeps; nil means the rows of `Opts`, Phase 21's behaviour) and
  `KeepRowsLast` (put them after the custom and `{rules}` buttons). `eventValues` gains `Action` (the translated word), `Count`, `Limit` (0 renders
  empty) and `Window`. `eventDefaultInput` gains `Count`, `Limit`, `Action` (the mode word: warn, mute, kick, ban), `Reasons` and `Rows` (the bot
  rows the site computed), so the default builders stay pure and the page's previews can call them.
- **D-22:** The default builders live in one new file, `fuku/modules/event_message_auto.go` (the Automatic family's events): `warnDefaultReply`,
  `warnLimitDefaultReply`, `floodDefaultReply` and `blacklistNoticeDefaultReply` (shared by the blacklist and forward blocklist events, which
  send the same template with a different reason), plus `eventActionWord` and the flood's reason. Each returns today's text and options byte for
  byte; a pure test compares them with the goldens before any site is switched.
- **D-23:** No new table, migration, Redis key, handler group, shutdown drain, callback namespace or dependency. `go.mod`, `go.sum`, `main.go`
  and `migrations/` are unchanged.

### Messages page (default)
- **D-24:** The Automatic family: code `a`, icon 🤖, name key `settings_msg_fam_auto` (English "Automatic actions"; the hub cell "icon name" at
  most 20 code points), drawn after the manual family. Its events in order, each with the icon of its settings page (P7): warn 🚩, warn limit ⛔,
  flood 🌊, blacklist 🔤, forward blocklist 🚷. `ReservedButtons`: warn 1, flood 1, the others 0. None is timed. Hub rows become `[2 1 1 2]` (two
  family cells, the two private copy options, Back and Close); the Automatic list is `[2 2 1 2]`; event and private copy screens keep Phase 21's
  and 22's shapes.
- **D-25:** Family strings: `settings_msg_fam_auto`, `settings_msg_fam_auto_intro` (says the warn covers `/warn` and the rules' warns, the warn
  message keeps its Remove warn button, and the forward blocklist message never names the source), `settings_msg_wait_text_auto` (the text prompt:
  the Welcome placeholders and `{admin}` `{reason}` `{action}` `{count}` `{limit}`, `{rules}` for a Rules button, the Remove warn button added by
  the bot, and the reason line not added to a custom text) and `settings_msg_hint_auto`; five `settings_msg_ev_*` names and five
  `settings_msg_desc_*` descriptions. `settings_msg_intro`, `settings_msg_preview_header` and `settings_msg_preview_header_pm` are reworded in
  all 8 languages to cover automatic actions and "sample values".
- **D-26:** Rights are Phase 21's and 22's: viewing the hub, the family, an event screen, a private copy screen and the previews needs the live
  admin check and the owner gate; Set, Remove, Silent and the private copy switch need `can_change_info`, judged live on every press and every
  typed message (MSG-08). There is no command, so each control keeps its drift and authority tests in place of a parity test.
- **D-27:** Previews go through the send path's composers with the presser as the member and the admin and the event's sample values (P10):
  warn count 1 of limit 3; warn limit 3 of 3, action mute; flood 6 messages of limit 5, action mute; blacklist and forward blocklist action mute.
  The reason is `settings_msg_sample_reason` for warn, warn limit and blacklist, and the event's own fixed reason for flood
  (`eventmsg_reason_flood`) and forward blocklist (`staff_fwdblock_reason`). A preview never carries a bot row (Remove warn and Unmute are live
  callbacks) and never posts in the group.

### Private copies (default)
- **D-28:** Specs: `warn` and `warnlimit` are registered `Automatic: false`, but a reply raised by an automatic rule counts against the
  per-group cap: `eventSite.Automatic` (the blacklist's and forward blocklist's warns, and the limit those warns reach) reaches the offer
  (`eventPMOffer.ByRule`), and the job checks the cap when the spec or the site says automatic (P8). `flood`, `blacklist` and `fwdblock` are
  `Automatic: true`. `/warn` is manual and never capped.
- **D-29:** Default private texts `eventmsg_pm_warn`, `eventmsg_pm_warnlimit`, `eventmsg_pm_flood`, `eventmsg_pm_blacklist` and
  `eventmsg_pm_fwdblock`, each naming `<b>{group}</b>` (`TestEventMsgLocaleKeys`), with `{count}`, `{limit}` and `{action}` where they apply and
  `{action}` after a label (P9). They do not repeat the reason, because the reason line (`eventmsg_pm_reason`) follows whenever the event has
  one. `eventPMDefaultText` passes `count`, `limit` and `action` besides `group` and `until` (empty when the event has none).
- **D-30:** The forward blocklist watcher lives in a staff file, but its punishments are the linked group's own automatic actions (like a
  blacklist word), not a Staff Group action: "Staff Group action" in PMSG-05 is the StaffActions fan-out, which never reaches the engine. The
  watcher reaches the engine only through `forwardblock_notice.go` and `warns.go`; no staff, lockdown or raid file names `postEvent` or any
  identifier `TestPMCopyCallers` forbids.

### Security and privacy (default)
- **D-31:** Nothing about the blocked source or the Staff Group ever reaches an `eventReply` of the forward blocklist: the site has no such value
  to put in, the reason is the fixed translated text, and `{chatname}` is the linked group's own title. A sentinel source title and Staff Group
  title prove it on the group message, the private copy, the Details record and the log line.
- **D-32:** No log line of these paths carries a stored or typed text, a caption, a file ID, a button, a reason, a matched word or a name: chat
  ID, user ID, event and outcome class only.
- **D-33:** Guards: `TestPMCopyCallers` tracks the users of `postEvent` with an allow-list (`event_message.go`, `warns.go`, `antiflood.go`,
  `blacklists.go`, `forwardblock_notice.go`, each with its reason) and forbids `postEvent` in every staff, lockdown or raid file;
  `TestMemberDeletionExcludedMessages` is unchanged (`forwardblock_notice.go` is outside its scan, and `staff_forwardblock_watch.go` changes only
  the argument it passes to `forwardBlockNotice`). `TestCommandRepliesStayInsideTheHandler` keeps its empty allow-list.

### Tests (default)
- **D-34:** The goldens are one test, `TestLegacyPunishmentNoticeRequests` in `fuku/modules/punishment_notice_legacy_request_test.go`, one subtest
  per case (the warn family with and without a reason, with the Rules button, `/dwarn`, `/swarn`, the blacklist's and forward blocklist's warns;
  the warn limit at kick, mute, ban and an inert mode; flood mute, kick, ban, a banned sender chat and an inert action; blacklist mute, kick, ban
  and a banned sender chat; forward blocklist mute, kick, ban), each pinning the sendMessage request, the punishment call and the queue row. It
  calls only entry points whose signatures every later plan keeps (`warnsModule.warnUser`, `dWarnUser`, `sWarnUser`, `warnThisUser`,
  `antifloodModule.checkFlood`, `blacklistsModule.blacklistWatcher`, `forwardBlockMute`, `forwardBlockKick`, `forwardBlockBan`,
  `forwardBlockWarn`).
- **D-35:** `fuku/modules` takes about 38 minutes against a 40-minute limit, so every new test is light: worst cases, not cross products; language
  sweeps over pure renders only; one environment per dispatcher subtest; real locales only where a default text is compared; fresh chat and user
  IDs; cleanup in `t.Cleanup`; no `t.Skip`, no `t.Parallel`, no mock library. The closing plan runs the full package once and records its time.
- **D-36:** No PostgreSQL guarantee is added (no migration, no index, no CHECK), so nothing joins `make test-postgres-integrity`.

### Repo rules that bind
- **D-37:** New locale keys go in all 8 files. `eventmsg_` keys get rows in `eventMsgLocaleLimits` (`TestEventMsgLocaleKeys`); `settings_` keys get
  rows in `settingsLocaleLimits` and label groups in `TestSettingsLabelLengths`. Ukrainian and English follow `locale-style.md`
  (`TestLocaleStyle`), placeholders match across languages (`TestLocalePlaceholdersMatchOtherLanguages`), HTML only.
- **D-38:** AGENTS.md is updated in the same commit as the change it describes (the generic engine, the event list in Data, the Event messages and
  PM copies bullets, the Messages page bullet, the test families).
- **D-39:** Every new production identifier has a production caller in the plan that adds it, or a dated `//nolint:unused // first production
  caller arrives in plan 23-NN` that the named plan removes. Lint gates: golangci-lint v2.13.1 (`/root/go/bin/golangci-lint`, `CGO_ENABLED=0`)
  `run --new-from-rev=origin/main ./...`, the same with `--build-tags=notesttools`, and `run --new=false` on the touched packages filtered to the
  touched files. At 1a382c3 that last gate already reports four findings in files this phase touches (the `dupl` of `blacklists.go` with
  `approvals.go`, and `gocyclo` of `checkFlood` at 31, `addBlacklist` at 23 and `blacklistWatcher` at 35); the gate allows those four and nothing
  else, and the two watchers' complexity must not rise. No AI model is named in code, docs or commit subjects.

### Planner's and Executor's Discretion
Function, field and test names inside the prefixes above, the exact strings (inside their limits and the style guide), the order of lines inside a
screen body, how a site's default send closure is written, and how a test's cases are split into subtests.

</decisions>

<open_questions_resolved>
## Roadmap Planning Notes and Open Questions, Resolved

| Question | Resolution |
|----------|------------|
| The Automatic family's code and its five events | D-09, D-24: family `a`; keys `warn warnlimit flood blacklist fwdblock`; codes `wn wl fl bl fb` |
| How the Remove warn button survives a custom message | D-15, D-21: `KeepRows` with `KeepRowsLast`, counted by `ReservedButtons` like the Unban row |
| What `{count}`, `{limit}`, `{action}`, `{reason}`, `{window}` take per event | D-11 to D-14 |
| `{admin}` for automatic actions | D-12: the translated "an admin" (MSG-03) |
| Deletion kinds | D-17: unchanged, by the warn's source; flood, blacklist, forward blocklist `bot:punish_auto` |
| Private copies; `/warn` manual | D-28: warn and warnlimit manual specs with a per-reply automatic flag; flood, blacklist, fwdblock automatic |
| The forward blocklist never names the source or the Staff Group | D-06, D-31; the notice stays outside staff files |
| `TestPMCopyCallers` and `TestMemberDeletionExcludedMessages` | D-33: `postEvent` allow-list with reasons, forbidden in staff, lockdown and raid files; the never-delete scan unchanged |
| Goldens first | D-08, D-34 |
| The engine is bound to the moderation pipeline today | D-20: a generic `eventSite` and `postEvent`; `replyEvent` stays as a wrapper |
| Where the default builders live | D-22: `event_message_auto.go`, proven against the goldens before the sites switch |
| The inert modes (a stored `tban`, a flood `warn`) | D-10: keep today's text, not events (P1) |

</open_questions_resolved>

<screens_and_codes>
## Screens and Callback Codes

No new action code: the Automatic family reuses the Messages page's codes (`hl he hs hr hc hv hf hq hp ho`) with new values only.

- **Hub** (`a=op&p=h`): the intro, the manual family, then the Automatic family (bold title with 🤖 and one state line per event, with the private
  copy mark ` · 📩` while its copy is on), then the private copies block. Rows `[2 1 1 2]`: the two family cells, the Details and Log toggles,
  Back and Close.
- **Automatic list** (`a=hl&t=a`): its intro, five event lines, the list hint; rows `[2 2 1 2]`.
- **Event screen** (`a=he&k=wn|wl|fl|bl|fb`): Phase 21's screen with the family's hint, plus Phase 22's private copy line and row once the event has
  a private copy spec. Worst `[2 2 2 2 2 1 1 2]`.
- **Private copy screen** (`a=he&k=<code>&d=p`): Phase 22's. Worst `[2 2 2 2 2 1 2]`.

Worst callback sizes at a 14-character chat ID are Phase 22's: `hs hr hc hv` with `d=p` 46 bytes, `he` and `hf` with `d=p` 42, `hp` 42, `ho` 41;
the two-letter event codes have the length of the manual ones.

</screens_and_codes>

<code_anchors>
## Code Anchors (at 1a382c3)

- `fuku/modules/warns.go`: `applyWarn` (line 193, never sends; `warnApplyResult` with `Count`, `Limit`, `Mode`, `LimitReached`, `LimitApplied`,
  `Reasons`), `applyWarnLimitAction` (232), `warnSource` and `deletionKind` (262 to 279), `warnThisUser` (281: the admin refusal, `applyWarn` with the
  `dwarn`/`swarn` hook, the limit text from `warns_limit_kicked|muted|banned` plus the reasons list, the warn text `warns_user_warning` plus
  `warns_warning_reason`, the keyboard [Remove warn | Rules] built when the rules are set, one `b.SendMessage` with reply parameters to
  `msg.MessageId` and `ReplyMarkup: &keyboard`, then `scheduleBotMessage(sent, source.deletionKind())`), `dispatchWarn` (412).
- `fuku/modules/antiflood.go`: `floodControl` (48, `messageCount`), `checkFlood` (244: exemptions, the deletes, the switch on `flood.Action` with
  the Unmute row for a mute and the Unban row for a banned sender chat, the notice through `helpers.SendMessageWithErrorHandling` with
  `antiflood_checkflood_perform_action`, the English `fmode` word and `MessageThreadId: msg.MessageThreadId`, then `scheduleBotMessage(sent,
  deletion.KindBotAuto)`).
- `fuku/modules/blacklists.go`: `blacklistWatcher` (676: the reason `matched.Reason` filled with the matched word, the delete, then per action a
  `b.SendMessage(chat.Id, text, formatting.Shtml())` with `blacklists_bl_watcher_{muted,banned,kicked}_user` and `scheduleBotMessage(sent,
  deletion.KindBotAuto)`; the warn action calls `warnsModule.warnThisUser(..., warnSourceAuto)`).
- `fuku/modules/forwardblock_notice.go`: `forwardBlockNotice(b, ctx, userID, key)` (22), reusing the blacklist templates with
  `forwardBlockReason(tr)`.
- `fuku/modules/staff_forwardblock_watch.go`: `forwardBlockEnforce` (300), `forwardBlockReason` (344), `forwardBlockWarn` (350), `forwardBlockMute`,
  `forwardBlockKick`, `forwardBlockBan` (358 to 381).
- `fuku/modules/event_message.go`: `eventValues` (41), `eventDefaultInput` (56), `eventReply` (70), `eventPlaceholderValues` (93), `eventAdminName`
  (110), `renderEventText` (155), `eventBotRows` (169), `composeEventMessage` (190), `replyEvent` (233), `replyEventGroup` (260), `sendDefaultPost`
  (294), `offerReplyPM` (312).
- `fuku/modules/event_message_pm.go`: `eventPMSpec` (37), `registerEventPM` (48), the manual specs (67), `eventPMOffer` (131), `offerEventPM` (278),
  `runEventPM` (312), `composeEventPM` (357), `eventPMDefaultText` (384).
- `fuku/modules/settings_messages.go`: `settingsMsgEvent` (89), `settingsMsgManualEvents` (108), `settingsMsgFamilies` (142), the page's `init`
  (198, which registers both editor definitions of every event of every family), `settingsMsgPreviewReply` (751), `settingsMsgPreview` (768),
  `settingsMsgPMPreview` (819), `renderSettingsMsgHub` (876), `renderSettingsMsgFamily` (915).
- `fuku/db/eventmsg/events.go`: `Events`, `ValidEvent`.
- Tests: `moderation_legacy_request_test.go` (the golden pattern, `moderationGoldenTarget`, `withModerationGoldenLocales`), `deletion_bot_sites_test.go`
  (`botSite`, `commandCtx`, `TestWarnThisUserSource`), `deletion_bot_auto_test.go` (`autoFloodSite`, `autoBlacklistSite`,
  `TestBotAutoPunishmentNotices`), `event_message_env_test.go`, `event_message_pm_env_test.go`, `staff_forwardblock_env_test.go` (`newFwdWatchEnv`),
  `settings_messages_env_test.go`, `settings_messages_screens_test.go` (`settingsMsgScreens`), `messages_e2e_test.go`, `pm_copies_e2e_test.go`,
  `pm_copy_callers_test.go`, `member_deletion_e2e_test.go` (`memberExcludedProducers`), `fuku/i18n/eventmsg_locale_test.go`,
  `fuku/i18n/settings_locale_test.go`.

</code_anchors>

<canonical_refs>
## Canonical References

- `AGENTS.md`: Data (`chat_event_messages`, `chat_event_message_options`, the `fuku:pmcopy:*` keys), Subsystem traps (Event messages, PM copies,
  Scheduling the bot's own messages, the forward blocklist watcher), Callbacks (the Messages page, the message editor), Permissions, Testing.
- `.planning/phases/21-message-engine-messages-page-and-manual-punishments/` (`21-CONTEXT.md`, the summaries, `21-VERIFICATION.md`) and
  `.planning/phases/22-pm-copies/` (`22-CONTEXT.md`, the summaries, `22-VERIFICATION.md`).
- `.planning/ROADMAP.md` Phase 23 planning notes and success criteria; `.planning/REQUIREMENTS.md` EVT-02, EVT-03 and the cross-cutting MSG and
  PMSG rules.

</canonical_refs>

<owner_visible>
## Behaviour an Owner Would Notice

Nothing changes for a group that sets nothing: every reply of the five sites is byte-identical. When the owner sets messages:
- the Messages page shows a second section, Automatic, with warn, warn limit, flood, blacklist and forward blocklist;
- a custom warn message keeps the Remove warn button under the custom buttons, and loses the default's Rules button unless `{rules}` is in the text;
- a custom flood message keeps the admins' Unmute or Unban button above the custom buttons;
- `{admin}` reads "an admin" on every automatic action;
- a Silent warn posts nothing, so there is no Remove warn button for it;
- private copies of the blacklist's and forward blocklist's warns count against the per-group cap, while `/warn` copies never do.

</owner_visible>

<provisional>
## Provisional Decisions for Owner Confirmation

| Id | Decision | Where | Alternative the owner may prefer |
|----|----------|-------|----------------------------------|
| P1 | `/swarn` posts the custom warn message (it posts the warning today); a stored inert warn-limit mode or flood action keeps today's text and is not an event | D-10 | treat the inert cases as events with an empty `{action}` |
| P2 | `{admin}` is the translated "an admin" for every automatic action (MSG-03's wording) | D-12 | a separate word such as "the bot" (one more key) |
| P3 | The blacklist's `{reason}` is today's default reason, which names the matched word and is not translated | D-14 | a translated phrase that does not repeat the word |
| P4 | The warn limit's `{reason}` is the last warn's reason only; the flood gets a new translated reason | D-11, D-14 | the limit's reason as every stored reason joined; no flood reason |
| P5 | The flood's admin row stays above custom buttons (like the Unban row), the Remove warn row goes below them (roadmap); a custom warn drops the default's Rules button | D-15 | both bot rows last; keep the Rules button |
| P6 | A custom message goes exactly where the default goes (blacklist and forward blocklist notices: no reply, no topic) | D-16 | reply to nothing but post in the member's topic |
| P7 | Family name "Automatic actions", icons 🤖 🚩 ⛔ 🌊 🔤 🚷, the event order, codes `wn wl fl bl fb` | D-09, D-24 | other names or order |
| P8 | Automatic warns (blacklist, forward blocklist) count against the per-group private copy cap; `/warn` never | D-28 | cap by event only (no cap for any warn) |
| P9 | The default private texts name the count, the limit and the action where they apply | D-29 | shorter texts without numbers |
| P10 | Preview sample values (warn 1 of 3, warn limit 3 of 3 with mute, flood 6 of 5 with mute, blacklist and forward blocklist mute) | D-27 | other samples |

</provisional>

<deferred>
## Deferred Ideas

`{duration}` and `{until}` for timed warn-limit, flood and blacklist actions (Phase 26); a per-event `{reasons}` list for the warn limit; a translated
blacklist reason without the word; the captcha, AI spam and requirement events of the Automatic family (Phase 24); a custom message per language
(MSG-F1); custom messages in backups (Phase 30).

</deferred>

---
*Phase: 23-warn-flood-blacklist-and-forward-blocklist-messages*
*Context gathered: 2026-11-21*
