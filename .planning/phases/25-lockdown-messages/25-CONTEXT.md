# Phase 25: Lockdown Messages - Context

**Gathered:** 2026-11-22
**Status:** Ready for planning. The decisions marked "owner" were settled before this phase (the milestone decisions of 2026-11-18, the
roadmap's planning notes and success criteria for Phase 25, the requirements EVT-06, EVT-07, EVT-08 and EVT-10, and the owner's standing rules
for this run: everything per group in the bot's private chat, no Staff Group defaults, lockdown messages send no private copies, alerts,
raid-scale removals and the Staff Group stay untouched). The decisions marked "default" are the planner's provisional defaults, taken from the
documents and the code without asking the owner, because the owner authorized this run to proceed autonomously; they are reversible, and the
ones an owner would notice are listed under "Provisional decisions for owner confirmation" at the end (P1 to P15).

<domain>
## Phase Boundary

Phase 25 gives the lockdown's public messages their own custom message, through the event message engine of Phases 21 to 24:

- **Manual lockdown** (EVT-06): the reply `/lockdown` sends after it started a lockdown. The bot appends the technical notes after a custom text:
  approved users are muted too, and, when the bot lacks the right, it cannot delete join messages or manage join requests here.
- **Automatic lockdown** (EVT-07): the public notice `raidPostNotice` posts in the locked group when a join surge, a flood or an AI burst started a
  lockdown, one message per trigger, with `{count}`, `{window}` and the flood's `{percent}`. The same technical notes follow a custom text. The
  notice keeps running inside `raidPostAsync` and the `StopRaidPosts` drain: no drain and no second is added.
- **Lift** (EVT-08): the public notice of a recorded lift, from `/unlockdown`, the alert's Lift button and the settings menu (private and group
  menu alike). The lift's own facts (the permissions were changed by hand; joiners are being unbanned) follow a custom text.

The Messages page (Phase 21) gains a fourth family, **Lockdown**, with five events. Each opens the editor with a group message and a Silent
switch. No lockdown event has a private copy.

A group with nothing stored gets today's lockdown texts byte for byte (MSG-11, proved by goldens recorded first). A refused custom message falls
back to the default (MSG-04). The lock, the snapshot, the worker and the alert are never delayed by a message. Lockdown messages are never queued
for deletion today, and a custom one is not either (MSG-09).

EVT-10 is proved here by two guards: a structural one over every caller of the engine (no alert, reminder, raid-scale removal or its report,
`#LOCKDOWN_BAN` post, lift tally, cannot-lock warning, Staff Group surface, log post or captcha challenge reaches the renderer or the private copy
sender) and a behavioural one (those surfaces send exactly what they send today with every lockdown message stored).

Out of scope: the lockdown alert, its reminders, its card edits and its buttons; the cannot-lock warning; the joiner bans, declines and unbans;
"Ban N recent joiners", its card report and its `#LOCKDOWN_BAN` post; the lift tally; `/lockdownstatus`, `/autolockdown` and every refusal or
"already active" answer; the settings menu's own redraw notice; private copies of lockdown messages; any change to the lockdown logic itself;
timed punishments (Phase 26), manual command defaults (Phase 27), global limits (Phase 28), plain tone and copy settings (Phase 29), backups
(Phase 30), a custom message per language (MSG-F1).

</domain>

<decisions>
## Implementation Decisions

### Scope rules (owner)
- **D-01 (owner):** Everything is configured group by group in that group's settings in the bot's private chat. There are no Staff Group
  defaults and nothing is inherited. The Messages page is the only door: no command sets a lockdown message.
- **D-02 (owner):** A custom text is one text for every language. With no row (or a row with no text, media or buttons and Silent off), the bot
  sends today's default in the group's language.
- **D-03 (owner):** The lockdown comes first. A custom message never delays, undoes or skips the lock, the snapshot, the worker, the alert or the
  lift: every message is sent after the action it describes, from the same place today's default is sent. A custom message Telegram definitely
  refuses is replaced by the default, sent once (MSG-04).
- **D-04 (owner):** Lockdown messages send no private copies. No lockdown event has a private copy spec, so the page draws no private copy row,
  refuses every private copy press (`d=p`, `hp`) for a lockdown event, and the engine offers no copy (no spec, and no member to send one to).
  There is no Details button and no `#PM_COPY` line for a lockdown event.
- **D-05 (owner, roadmap EVT-10):** The lockdown alert and its reminders (posting, refresh, close), the cannot-lock warning, the joiner bans,
  declines and unbans, "Ban N recent joiners" with its card and its `#LOCKDOWN_BAN` post, the lift tally, every Staff Group surface (action cards,
  summaries, the panel, undo, staff log posts), log channel posts and the captcha challenge are not events: their texts and behaviour are
  unchanged, they never go through the renderer and never get a private copy.
- **D-06 (owner, MSG-11):** The first plan records characterization goldens of every send site this phase changes (the manual start reply, each
  automatic notice, the lift notice from each door, and the cannot-lock warning, which shares `raid_trigger.go` and `raidSend` with the automatic
  notice) and commits them alone, before any production change. They are never re-recorded or weakened.
- **D-07 (owner, EVT-06, EVT-07):** The technical notes are appended by the bot after a custom text: `lockdown_note_approved_muted` always,
  `lockdown_note_no_delete` when the bot cannot delete messages and `lockdown_note_no_invite` when it cannot manage join requests (read from the
  bot's live membership that `startLockdownRow` returned, exactly as today).
- **D-08 (owner, MSG-09):** Deletion timers stay as they are. No lockdown message is queued for deletion today (none is a named site of
  `scheduleBotMessage`), so a custom lockdown message is never queued either, custom or default.
- **D-09 (owner, roadmap):** The automatic notice, custom or default, runs inside the post `raidPostAsync` already starts and `StopRaidPosts`
  already drains. No shutdown drain, no handler registration and no second is added; the worst cases of `AGENTS.md` are unchanged.

### Which sends are events (default)
- **D-10:** Event keys, validated in Go by `eventmsg.ValidEvent` (appended to `eventmsg.Events` after `mreqmute`, no migration, no CHECK):
  `lockdown` (`eventmsg.EventLockdown`), `lockjoin` (`EventLockJoinSurge`), `lockflood` (`EventLockFlood`), `lockai` (`EventLockAIBurst`), `locklift`
  (`EventLockLift`). Page codes (the `k` field): `ld`, `lj`, `lf`, `la`, `lu`, so the wait keys are `hldg`, `hljg`, `hlfg`, `hlag`, `hlug` (the
  page's loop also registers the unreachable private copy definitions `hldp` ... `hlup`, as it does for every event). Family code `l` (the `t` of
  `hl`; the private copies block's `ho` also has an option `l`, a different action, so the two never meet).
- **D-11:** What each event covers:
  - `lockdown`: the reply of `/lockdown` whose `startLockdownRow` answered `lockdownStartStarted`, also after an anonymous admin's proof tap.
    Every refusal, `lockdown_already_active` and `lockdown_already_active_auto`, `lockdown_lock_unknown`, `lockdown_lock_failed`,
    `lockdown_already_lifted` and `lockdown_state_failed` keep today's reply and are not events (P2).
  - `lockjoin`, `lockflood`, `lockai`: the public notice of the call of `raidFire` that got `lockdownStartStarted`, chosen by the trigger kind; an
    unknown kind posts nothing, as today.
  - `locklift`: the notice of a lift this caller recorded: `/unlockdown`'s reply for `liftApplyDone`; the alert's Lift Confirm and the private
    menu's Lift Yes (`lockdownAnnounceLift`); the group menu's Lift Yes (D-22). `/unlockdown`'s other answers (not active, in progress, already
    lifted, restore or record failed, state failed) keep today's reply and are not events.
  - Not events: the cannot-lock warning (`raidWarnCannotLock`), the alert card's edits (`lockdown_alert_lift_done` and the rest), the settings
    menu's redrawn notice, the lift tally (`lockdownPostTally`, `lockdownPostInlineTally`), `/lockdownstatus`, `/autolockdown` and every alert,
    reminder and ban run text.

### Placeholders (default)
- **D-12:** The Welcome placeholders and the event placeholders take these values; every value is escaped, and empty renders as nothing (MSG-02):

  | Placeholder | lockdown (manual) | lockjoin | lockflood | lockai | locklift |
  |-------------|-------------------|----------|-----------|--------|----------|
  | `{admin}` | the starter's recorded name; "an admin" for an anonymous admin | "an admin" | "an admin" | "an admin" | the lifter's recorded name; "an admin" for an anonymous admin |
  | `{first}` `{fullname}` `{mention}` `{username}` `{id}` (`{last}` empty) | the starter (D-13) | the bot (D-13) | the bot | the bot | the lifter (D-13) |
  | `{chatname}` | the group's title | the group's title | same | same | the group's title (the stored `chat_title` of the lockdown row for a button lift, P15) |
  | `{reason}` | the `/lockdown` reason, empty when none | the trigger sentence (`lockdownTriggerSentence`) | same | same | empty |
  | `{count}` | empty | joins counted | messages counted | spam verdicts counted | empty |
  | `{window}` | empty | the window in seconds, as a number (`60`) | `30` | `120` | empty |
  | `{percent}` | empty | empty | the share of new members, as a number (`80`) | empty | empty |
  | `{action}` `{limit}` `{duration}` `{until}` | empty | empty | empty | empty | empty |

- **D-13:** The person placeholders of a lockdown message name the person the lockdown row records as the starter or the lifter: the name recorded
  (capped at `lockdownNameMaxRunes`, 64), with no last name and no username, so `{username}` reads as the mention, exactly the person and the
  name today's default names ("Locked by X", "Lockdown lifted by X"). For an automatic lockdown, whose row records the bot as the starter, and for
  an anonymous admin (the proof tap's mark, read by `actorIsAnonymous`, MSG-03), they name the bot itself, never the tapper. `{admin}` follows
  MSG-03: the same recorded name, or "an admin" (`eventmsg_admin_anon`) for an automatic action and an anonymous admin. Today's default text is
  unchanged in every case (it still names the tapper after a proof, as it does today).
- **D-14:** `{percent}` joins the engine's event placeholders (`eventTokenNames` gains `percent` after `window`, `eventValues` gains `Percent int`,
  0 renders as nothing) (P3). It is empty for every other event. A stored text of an earlier event that already holds the literal `{percent}` (it
  showed raw before) now renders it empty: no page ever offered it, so this changes nothing anyone set on purpose.
- **D-15:** `{reason}` of an automatic lockdown is `lockdownTriggerSentence` with the trigger's numbers (it holds only numbers and the translated
  sentence, never user text); `{reason}` of a manual lockdown is the stored `row.Reason`; the lift has none (P5).

### Notes, place, timer, fallback, Silent (default)
- **D-16:** After a custom text the bot appends, after one line break and joined by line breaks (`eventReply.Note`, which `composeEventMessage`
  already appends after a custom text):
  - start events (`lockdown`, `lockjoin`, `lockflood`, `lockai`): the technical notes of D-07, in today's order;
  - `locklift`: `lockdown_lifted_manual_change` when the permissions were changed by hand during the lockdown, then `lockdown_lifted_unbanning`
    with the count when joiners are still being unbanned (the lift's facts that today's default carries) (P7).
  The "Locked by X" line, the reason line and the automatic notice's numbers are not appended: the owner places `{admin}`, `{reason}`, `{count}`,
  `{window}` and `{percent}`. With no custom text (only media or buttons), the text is today's default text, notes included.
- **D-17:** The editor holds back `lockdownNoteReserve` (400 UTF-16 units) from the text of a lockdown event through the existing
  `settingsEditPageDef.ReservedText` (Phase 24), so the prompt shows `Up to 3696`, and `Up to 624` while a media item is stored (P8). The worst real
  note measured on the locale files at d8c98c6 is 255 units (French start notes with both rights missing; the worst lift note is 219, French with a
  five-digit count); a test holds every language's worst note to the reserve, and a translation that exceeds it is shortened, never the reserve
  raised.
- **D-18:** A custom message goes where today's default goes: `/lockdown`'s and `/unlockdown`'s as a reply to the command (in its topic for a topic
  message, `allow_sending_without_reply`), the automatic notice and a button lift's notice as a new message with no reply and no topic.
- **D-19:** Sends. The command replies are not paced, as today: the default is today's `msg.Reply(b, text, formatting.Shtml())` and a custom message
  is `media.SendGreetingMessage` as a reply. The automatic notice and a button lift's notice go through `lockdownPaced`, as today: the default is
  today's exact call (`raidSend` for the automatic notice; the paced `SendMessageWithContext` of `lockdownPostLiftNotice` for a lift), and a custom
  message is `media.SendGreetingMessage` inside one paced unit. **The bound, stated exactly (the Phase 24 verifier's advisory):** the pacer's wait
  derives from the post's context (`raidPostTimeout` for the automatic notice, which `StopRaidPosts` gives up on after 2 s anyway); the default's
  request keeps its per-call `lockdownCallTimeout` (10 s) through `SendMessageWithContext`; but `media.SendGreetingMessage` takes no context, so a
  custom request already in flight is bounded only by the bot's HTTP client timeout (`constants.LongTimeout`, 30 s), not by the 10 s call
  timeout. No plan, comment or `AGENTS.md` sentence may claim that the per-call timeout covers a custom lockdown send.
- **D-20:** Every lockdown event can be silenced (P10). Silent posts nothing in the group: the lockdown or the lift still happens, the alert and the
  worker are untouched, the lift tally (a raid-scale report, D-05) is still posted after a lift that finished inline. A Silent `lockdown` or
  `locklift` means `/lockdown` or `/unlockdown` answers nothing in the group, like a Silent `/kick`.
- **D-21:** Fallback (MSG-04): a custom message Telegram definitely refuses (`eventMessageRefused`: a 4xx other than 429, or
  `media.ErrNoPermission`) is replaced by the default, sent once through the site's own default send; a 429, a 5xx or a transport error is logged
  as today's default failure is and changes nothing else. Each site keeps its log line (`[Lockdown] reply: %v`, `[RaidDetection] the notice of the
  automatic lockdown in chat %d was not delivered: %v`, `[Lockdown] lift notice for chat %d could not be posted: %v`).
- **D-22:** A lift from a settings menu opened in the group posts the custom lifted message as a new message in the group when one is stored (content
  and Silent off) and posts nothing otherwise: with nothing stored it stays exactly as today (the redrawn menu, in that same chat, is the notice, and
  a second public default would say it twice); a refused custom message, a read error and Silent post nothing more (P11). The menu's redraw, its
  notice and the worker wake are unchanged.
- **D-23:** `lockdownAnnounceLift` wakes the worker before it posts the notice (today it wakes after), so neither the message read nor a custom send
  delays the worker's close step; `liftLockdownRow` still wakes it first when joiners are left. The notice itself, the inline tally after it and the
  order of the group's messages are unchanged.

### Engine (default)
- **D-24:** The engine changes only by vocabulary: `eventValues.Percent` and the `percent` token (D-14), and three fields of `eventDefaultInput` read
  only by the lockdown builders: `Window int`, `Percent int` and `Lockdown lockdownFacts` (the bot's missing rights for a start; the manual change,
  whether the lift finished, and the joiners being unbanned for a lift). No new hook, no change to `postEventSent`, `postEventGroup` or
  `composeEventMessage`. Every Phase 21 to 24 request stays byte-identical (`TestLegacyModerationRequests`, `TestLegacyPunishmentNoticeRequests`,
  `TestLegacyScreeningNoticeRequests`).
- **D-25:** Default builders, in `fuku/modules/event_message_lockdown.go`, each returning today's text and options byte for byte (proven against the
  goldens before a site switches): `lockdownStartDefaultReply` (the manual reply: started, approved muted, locked by, the reason line, the rights
  notes, with `formatting.Shtml()`), `lockdownAutoDefaultReply(kind) eventDefaultReply` (the trigger's intro with its numbers, then the notes, HTML
  with link previews off) and `lockdownLiftDefaultReply` (`lockdownLiftedText`, with `formatting.Shtml()`); `lockdownStartNotes` and
  `lockdownLiftNotes` build the appended notes and the defaults alike, so the two can never drift.
- **D-26:** Sites. `TestPMCopyCallers` forbids every engine entry in files named `staff*`, `lockdown*` and `raid*`, so the lockdown files reach the
  engine only through the non-lockdown adapter file `fuku/modules/event_message_lockdown.go` (the way `forwardblock_notice.go` serves the forward
  blocklist watcher): `postLockdownStartEvent` (called by `lockdown.go`'s `lockdown`), `postLockdownAutoEvent` (`raid_trigger.go`'s
  `raidPostNotice`), `postLockdownLiftReplyEvent` (`lockdown.go`'s `unlockdown`) and `postLockdownLiftEvent` (`lockdown_alert_callbacks.go`'s
  `lockdownAnnounceLift`, and `settings_lockdown.go`'s `settingsLockdownLiftYes` for the group menu with the default switched off). Each builds an
  `eventSite` whose `Schedule` is a no-op (D-08), whose `SendDefault` is today's exact call and whose `Automatic` is true only for the triggers, and
  calls `postEvent`. `event_message_lockdown.go` joins the `postEvent` allow-list of `TestPMCopyCallers` with its reason; no lockdown, raid or staff
  file names an engine entry.
- **D-27:** No new table, migration, handler group, shutdown drain, callback namespace, Redis key or dependency. `go.mod`, `go.sum`, `main.go` and
  `migrations/` are unchanged.

### Messages page (default)
- **D-28:** The Lockdown family is the fourth row of `settingsMsgFamilies`, after Member requirements (P1): `{Code: "l", Icon: "🚨", NameKey:
  "settings_msg_fam_lockdown", IntroKey: "settings_msg_fam_lockdown_intro", WaitTextKey: "settings_msg_wait_text_lockdown", HintKey:
  "settings_msg_hint_lockdown", ReservedText: lockdownNoteReserve}`, with `lockdown` 🔒 (`ld`), `lockjoin` 👥 (`lj`), `lockflood` 💬 (`lf`), `lockai`
  🧠 (`la`) and `locklift` 🔓 (`lu`). Its list is `[2 2 1 2]`; the hub becomes `[2 2 1 1 2]` (four family cells, the two private copy options, Back
  and Close). An event screen is the manual family's shape without the private copy row: `[1 1 1 1 1 2]` with nothing stored and `[2 2 2 2 2 1 2]`
  with every part set.
- **D-29:** The lockdown events have the text, media and buttons parts (P9), a Silent switch, no reserved buttons (no bot row) and no private copy
  row.
- **D-30:** Strings (all 8 languages): `settings_msg_fam_lockdown`, `settings_msg_fam_lockdown_intro`, `settings_msg_wait_text_lockdown` (it lists
  `{first}` `{last}` `{fullname}` `{username}` `{mention}` `{id}` `{chatname}` `{admin}` `{reason}` `{count}` `{window}` `{percent}` `{rules}`, says
  that the person placeholders name the admin who locked or lifted, or the bot for an automatic lockdown, and that the bot adds the technical notes
  itself), `settings_msg_hint_lockdown` (the notes are added, these messages are never deleted automatically and send no private copy), five
  `settings_msg_ev_lock*` and five `settings_msg_desc_lock*`; reworded: `settings_msg_intro` (it names lockdowns starting and being lifted). No
  `eventmsg_` key is added (no private text, no action word).
- **D-31:** Rights are Phase 21's: viewing needs the live admin check and the owner gate; Set, Remove and Silent need `can_change_info`, judged live
  on every press and every typed message (MSG-08). No command exists, so the page keeps drift and authority tests in place of a parity test.
- **D-32:** Previews go through the send path's composer with the presser as the starter or lifter and the admin (P13): the manual start with the
  sample reason; the join surge with 15 joins in 60 s, the flood with 30 messages in 30 s and 80 %, the AI burst with 5 verdicts in 120 s, each with
  its trigger sentence as `{reason}`; the lift with no reason. A start event's Full preview of a custom text shows `lockdown_note_approved_muted`
  after it (the sample bot holds both rights) through `settingsMsgEvent.Appended`; the lift shows no note. `settingsMsgSample` gains `Window`,
  `Percent`, `Trigger` (the trigger kind whose sentence fills `{reason}`) and `NoReason` (the lift).

### Guards (default)
- **D-33:** The structural EVT-10 guard is `TestEventEngineCallers` (`fuku/modules/event_engine_callers_test.go`, added with the first adapter in
  plan 25-02 and extended by plans 25-04 and 25-05). It parses every non-test file of the package and:
  1. maps every function that names an engine entry (`postEvent`, `postEventSent`, `postEventGroup`, `replyEvent`, `composeEventMessage`,
     `renderEventText`, `composeEventPM`, `offerEventPM`, `offerReplyPM`, `eventMessagesLookup`, and every `postLockdown*Event` adapter) outside the
     engine's own files (`event_message.go`, `event_message_pm.go`) to `file: Function` and compares the set with an exact allow-list, each entry
     with its reason; a new caller and a stale entry both fail, and the only callers of the `postLockdown*Event` adapters are `lockdown.go:
     moduleStruct.lockdown`, `lockdown.go: moduleStruct.unlockdown`, `raid_trigger.go: raidPostNotice`, `lockdown_alert_callbacks.go:
     lockdownAnnounceLift` and `settings_lockdown.go: settingsLockdownLiftYes`;
  2. follows the same-package calls of each never-root by name (plain functions, and methods on the caller's own receiver, function literals in a
     body included, the resolution of `command_replies_guard_test.go`) and fails when any engine entry is reachable. The roots, each of which must
     exist: `runLockdownCycle` (alerts, reminders, sync and close steps, joiner bans, declines, unbans, the tally, the ban runs, their reports and
     `#LOCKDOWN_BAN`), `raidWarnCannotLock`, `lockdownPostInlineTally`, `moduleStruct.lockdownLiftAsk`, `moduleStruct.lockdownBanAsk`,
     `moduleStruct.lockdownBanConfirm`, `moduleStruct.lockdownRevokeAsk`, `moduleStruct.lockdownRevokeConfirm`, `moduleStruct.lockdownCardCancel`,
     `moduleStruct.handleStaffAction`, `moduleStruct.staffActionConfirm`, `moduleStruct.staffActionCancel`, `moduleStruct.staffUndoAsk`,
     `moduleStruct.staffUndoConfirm`, `startStaffRun`, `runStaffActionInGroup`, `sendStaffLogPost`, `buildStaffPanel`, `SendCaptcha` and
     `aispamMirrorDeletion`. A log channel post through `fuku/utils/actionlog` cannot reach the engine at all (that package cannot import
     `fuku/modules`), which the test's comment states.
- **D-34:** The behavioural EVT-10 guard is `TestLockdownMessagesLeaveOtherSurfacesAlone` (plan 25-06): with every lockdown event message stored
  (a sentinel text, a button and Silent off, also for the Staff Group and the log channel) and a private copy row switched on for each lockdown event,
  an alert copy, a reminder, a lift's alert close, a "Ban N recent joiners" run with its card report and `#LOCKDOWN_BAN`, the lift tally, the
  cannot-lock warning, a staff action card and summary, a captcha challenge and a log post carry no sentinel, and the bot sends nothing to any private
  chat.
- **D-35:** No private copy, proved by `TestLockdownEventsSendNoPrivateCopy` (plan 25-06): `eventPMSpecFor` is false for the five keys (and
  `TestEventPMSpecs` still counts eighteen specs), the page refuses `he`, `hs`, `hv`, `hf` with `d=p` and `hp` for every lockdown code before any
  lookup, and with a private copy row stored and switched on for each lockdown event a manual start, an automatic start and a lift by each door send
  nothing to any private chat.

### Security and privacy (default)
- **D-36:** No lockdown message, preview or screen names the Staff Group, its title or ID, the log channel or another group. `{chatname}` is the locked
  group's own title. A lift pressed in the Staff Group names the lifter (as today's default does), never the chat the button sat in.
- **D-37:** No log line of these paths carries a stored or typed text, a caption, a file ID, a button, the `/lockdown` reason or a name: chat ID, event
  and outcome class only (the engine's existing lines, and the sites' existing lines, which already carry the chat ID and Telegram's error).

### Tests (default)
- **D-38:** The goldens are one test, `TestLegacyLockdownMessageRequests` in `fuku/modules/lockdown_message_legacy_request_test.go`: the manual start
  (all rights; a reason with markup and both rights missing; an anonymous admin after the proof tap), each automatic notice (join surge with all
  rights, flood without the delete right, AI burst without the invite right), the cannot-lock warning (its Staff Group copy and its log channel copy),
  the lift by `/unlockdown` (finished; with joiners being unbanned and a manual change), by the alert's Lift Confirm and by the private menu, and the
  group menu's lift (no group message). Each pins the request with the IDs replaced by tokens, no deletion queue row with every bot kind's rule at
  its shortest delay, and no send to a private chat where the flow sends none.
- **D-39:** The full `fuku/modules` run takes about 6 minutes, so the phase is budgeted at about 28 seconds of added test time; each plan states its
  share and records the measured time of its new tests in its SUMMARY. Every new test is light: worst cases, never cross products; language sweeps
  over pure renders only; one environment per dispatcher subtest; real locales only where a default text is compared; fresh IDs; cleanup in
  `t.Cleanup`; no `t.Skip`, no `t.Parallel`, no mock library. The closing plan runs the full package once and records its time.
- **D-40:** No PostgreSQL guarantee is added (no migration, index or CHECK), so nothing joins `make test-postgres-integrity`.

### Repo rules that bind
- **D-41:** New locale keys go in all 8 files; `settings_` keys get rows in `settingsLocaleLimits`, label groups in `TestSettingsLabelLengths` (a cell is
  the icon, a space and the name, at most 20 code points) and the text prompt's tokens a row in `TestSettingsEditLocaleTokens`. Ukrainian and English
  follow `locale-style.md` (`TestLocaleStyle`), placeholders match across languages (`TestLocalePlaceholdersMatchOtherLanguages`), HTML only.
- **D-42:** `AGENTS.md` is updated in the same commit as the change it describes: the Data bullet of `chat_event_messages` (the five keys), the Event
  messages bullet (the lockdown sites, the adapter file, `{percent}`, the notes, no queue, the honest send bound), the PM copies bullet (no lockdown
  spec), the `TestPMCopyCallers` sentence, the automatic announcement bullet under Data, the `StopRaidPosts` bullet, the Lockdown page bullet (the group
  menu's custom lift), the Messages page bullet, the message editor bullet, the Scheduling bullet (lockdown messages are not a named site) and the
  Testing families.
- **D-43:** Every new production identifier has a production caller in the plan that adds it, or a dated `//nolint:unused // first production caller
  arrives in plan 25-NN` that the named plan removes. Lint gates: golangci-lint v2.13.1 (`/root/go/bin/golangci-lint`, `CGO_ENABLED=0`) `run
  --new-from-rev=origin/main ./...` (0 issues at d8c98c6), the same with `--build-tags=notesttools`, and `run --new=false ./fuku/modules/...
  ./fuku/db/eventmsg/... ./fuku/i18n/...` filtered to the touched files. Baseline at d8c98c6 (recorded by the planner): in the files this phase
  touches, only two findings exist, `gocyclo` of `(moduleStruct).lockdown` at 24 (`lockdown.go`) and of `(moduleStruct).lockdownAlertCallback` at 21
  (`lockdown_alert_callbacks.go`); `fuku/db/eventmsg` and `fuku/i18n` report 0. The gate allows those two at no higher complexity and nothing else,
  and no AI model is named in code, docs or commit subjects.

### Planner's and Executor's Discretion
Function, field and test names inside the prefixes above (the adapters keep the `postLockdown` prefix the guard tracks), the exact strings (inside their
limits and the style guide), the order of lines inside a screen body, how a site's default send closure is written, whether `lockdownAnnounceLift`
keeps its signature (if it changes, only the two calls in `lockdown_apply_test.go` follow, with no assertion changed), and how a test's cases are split
into subtests.

</decisions>

<open_questions_resolved>
## Roadmap Planning Notes and Open Questions, Resolved

| Question | Resolution |
|----------|------------|
| Send sites | D-11, D-26: `/lockdown`, `raidPostNotice` per trigger, the lift from `/unlockdown`, `lockdownAnnounceLift` and the settings menu |
| The automatic notice through `raidPostAsync` and `StopRaidPosts` | D-09: the engine call runs inside the same tracked post; no drain |
| Technical notes after the custom text | D-07, D-16: `eventReply.Note`, one line break, today's order; 400 units held back (D-17) |
| `{percent}` (EVT-07's "other numbers") | D-14: joins the engine's placeholders (P3) |
| The lift from a group menu ("whoever lifted it") | D-22: a stored custom message is posted; nothing stored stays as today (P11) |
| EVT-10 guard over every engine caller | D-33 (structural, from plan 25-02), D-34 (behavioural), D-35 (no private copy) |
| Goldens first | D-06, D-38 |
| Lockdown messages send no private copies | D-04, D-35 |
| The Phase 24 advisory on the send bound | D-19: stated exactly, never overstated |

</open_questions_resolved>

<screens_and_codes>
## Screens and Callback Codes

No new action code: the lockdown events reuse the Messages page's codes (`hl he hs hr hc hv hf hq`) with new values only; `hp` and every `d=p` form
are refused for a lockdown code.

- **Hub** (`a=op&p=h`): the intro, the manual, Automatic and Member requirements families, then the Lockdown family (five state lines), then the
  private copies block. Rows `[2 2 1 1 2]`.
- **Lockdown list** (`a=hl&t=l`): five event cells, rows `[2 2 1 2]`.
- **Event screen** (`a=he&k=<code>`): Phase 21's screen with no private copy line or row: `[1 1 1 1 1 2]` empty, `[2 2 2 2 2 1 2]` full.

Worst callback sizes at a 14-character chat ID are unchanged: the two-letter codes have the length of the manual ones (`he` 38, `hs hr hc hv` 42 in
the group form, `hq` 42, `hl` 37).

</screens_and_codes>

<code_anchors>
## Code Anchors (at d8c98c6)

- `fuku/modules/lockdown.go`: `lockdownSplice` (229), `lockdownJoinLines` (238), `lockdownMention` (249), `replyLockdown` (257),
  `lockdownReasonLine` (276), `lockdownAlreadyActiveText` (287), `lockdown` (312; the start at 358, the success parts at 387 to 404),
  `unlockdown` (421; the reply at 430), `lockdownPostInlineTally` (442), `applyLockdownLift` (491), `applyLockdownLiftOf` (500),
  `lockdownLiftApplyText` (532), `liftLockdownRow` (599), `lockdownLiftedText` (658), `LoadLockdown` (672). Caps: `lockdownReasonMaxRunes` 300,
  `lockdownNameMaxRunes` 64, `lockdownTitleMaxRunes` 128.
- `fuku/modules/raid_trigger.go`: `raidPostTimeout` (45), `raidPostAsync` (127), `StopRaidPosts` (141), `raidTrigger` (198), `raidFire` (224; the
  start at 293, the post at 316), `raidSend` (324), `lockdownTriggerSentence` (344), `raidPostNotice` (373), `raidWarnCannotLock` (429).
- `fuku/modules/lockdown_alert_callbacks.go`: `lockdownAlertCallback` (31), `lockdownLiftAsk` (258), `lockdownLiftConfirm` (308; the announce at 360),
  `lockdownAnnounceLift` (370), `lockdownBanAsk` (391), `lockdownBanConfirm` (458), `lockdownRevokeAsk` (606), `lockdownRevokeConfirm` (673),
  `lockdownPostLiftNotice` (758), `lockdownCardCancel` (778).
- `fuku/modules/settings_lockdown.go`: `settingsLockdownRedraw` (213), `settingsLockdownLiftAsk` (307), `settingsLockdownLiftYes` (344; the private and
  group branches at 363 to 367).
- `fuku/modules/lockdown_worker.go`: `lockdownPaced` (48), `wakeLockdownWorker` (199), `runLockdownCycle` (290), `lockdownPostTally` (682);
  `lockdown_perms.go`: `lockdownCallTimeout` (30, 10 s); `lockdown_start.go`: `lockdownStartParams` (56), `lockdownStartResult` (73, `BotMember`),
  `startLockdownRow` (86); `lockdown_status.go`: `lockdownStartedAutomatically` (129).
- `fuku/modules/event_message.go`: `eventValues` (46), `eventDefaultInput` (68), `eventReply` (94), `eventSite` (122), `eventTokenNames` (163),
  `eventPlaceholderValues` (174), `actorIsAnonymous` (229), `renderEventText` (244), `composeEventMessage` (280), `postEvent` (357), `postEventSent`
  (371), `postEventGroup` (439), `sendDefaultPost` (471), `offerReplyPM` (491).
- `fuku/modules/settings_messages.go`: `settingsMsgSample` (83), `settingsMsgEvent` (100), `settingsMsgFamily` (121), `settingsMsgFamilies` (238), the
  page's `init` (306), `settingsMsgEditDef` (388), `settingsMsgDefByFields` (441), `settingsMsgStateOf` (572), `settingsMsgCountChanged` (611),
  `settingsMsgPreviewReply` (883), `renderSettingsMsgHub` (1024), `renderSettingsMsgFamily` (1063), `renderSettingsMsgEvent` (1137).
- `fuku/db/eventmsg/events.go`: `Events`, `ValidEvent`; `fuku/db/eventmsg/repository_test.go`: the event table subtest (218).
- Tests: `lockdown_fake_test.go` (`newLockdownEnv`, `lockdownCleanup`, `seedLockdownJoiner`, `withFastLockdownPacer`), `settings_env_test.go`
  (`newSettingsEnvWith` with `RealLocale`, `newRealLocaleLockdownEnv`), `lockdown_status_test.go` (`lockAsAdmin`), `lockdown_alert_test.go`
  (`linkLockdownStaffGroup`), `lockdown_alert_lift_test.go` (the Lift press and Confirm), `lockdown_anon_test.go` (`sendAnonymous`, `tapProof`),
  `raid_notice_test.go`, `raid_join_surge_test.go` (`selfJoins`, `activeLockdown`), `raid_ai_burst_test.go` (a direct `raidPostNotice` call),
  `lockdown_apply_test.go` (`TestLockdownAnnounceLift`), `settings_lockdown_lift_test.go`, `deletion_e2e_test.go` (`deletionShortestRules`,
  `TestDeletionExcludedMessagesNeverScheduled`), `rules_legacy_request_test.go` (`legacyRequestJSON`), `screening_notice_legacy_request_test.go` (the
  golden pattern and its token helper), `event_message_env_test.go`, `settings_messages_env_test.go`, `settings_messages_screens_test.go`,
  `settings_messages_safety_test.go`, `settings_staff_privacy_test.go`, `messages_e2e_test.go` (`newMessagesE2E`), `pm_copy_callers_test.go`,
  `command_replies_guard_test.go` (the name resolution the new guard reuses), `event_message_pm_test.go` (`TestEventPMSpecs`),
  `fuku/i18n/settings_locale_test.go`.

</code_anchors>

<canonical_refs>
## Canonical References

- `AGENTS.md`: Startup and shutdown (`StopRaidPosts`), Data (`chat_event_messages`, the automatic announcement bullet, `chat_lockdowns`), Subsystem
  traps (Event messages, PM copies, Scheduling the bot's own messages, every lockdown bullet: `startLockdownRow`, the alert, reminders, the lift, the
  ban run), Callbacks (the Messages page, the Lockdown page, the message editor), Permissions (`/lockdown`), Testing.
- `.planning/phases/24-captcha-ai-spam-and-member-requirement-notices/` (the closest model: `24-CONTEXT.md`, the eight plans, the summaries,
  `24-VERIFICATION.md` and its advisory on the send bound), `.planning/phases/23-warn-flood-blacklist-and-forward-blocklist-messages/` and
  `.planning/phases/21-message-engine-messages-page-and-manual-punishments/`.
- `.planning/ROADMAP.md` Phase 25 planning notes and success criteria; `.planning/REQUIREMENTS.md` EVT-06, EVT-07, EVT-08, EVT-10 and the cross-cutting
  MSG rules.

</canonical_refs>

<owner_visible>
## Behaviour an Owner Would Notice

Nothing changes for a group that sets nothing: every lockdown and lift text is byte-identical, and a lift from a menu opened in the group still posts
no second message. When the owner sets messages:
- the Messages page gains a Lockdown section with five events: by an admin, join surge, flood, AI burst, and lifted;
- a custom start text is always followed by "Approved users are muted too." and, when the bot lacks the right, the no-delete and no-invite notes; a
  custom lift text by the manual-change and unbanning lines when they apply;
- `{count}`, `{window}` and `{percent}` carry the trigger's numbers, `{reason}` the `/lockdown` reason or the trigger's sentence;
- a lift from a menu opened in the group posts the custom lifted message;
- each event can be silenced (then `/lockdown` and `/unlockdown` answer nothing in the group);
- lockdown messages are never deleted automatically and never sent in private;
- alerts, reminders, "Ban N recent joiners", the cannot-lock warning and everything in the Staff Group look exactly as before.

</owner_visible>

<provisional>
## Provisional Decisions for Owner Confirmation

| Id | Decision | Where | Alternative the owner may prefer |
|----|----------|-------|----------------------------------|
| P1 | Event keys `lockdown lockjoin lockflood lockai locklift`, codes `ld lj lf la lu`; a Lockdown family `l` 🚨 last in the hub; icons 🔒 👥 💬 🧠 🔓 | D-10, D-28 | other names, icons, or the family before Member requirements |
| P2 | Only the reply of a `/lockdown` that started a lockdown is the event; refusals and "already active" keep today's text | D-11 | make "already active" an event too |
| P3 | `{percent}` joins the placeholders of every event (empty except for the flood trigger) | D-14 | no `{percent}`, the flood's share only in the default text |
| P4 | `{window}` is the number of seconds alone (`60`) and `{count}` the trigger's count | D-12 | a worded window ("60 seconds") |
| P5 | `{reason}`: the `/lockdown` reason; the trigger sentence for an automatic lockdown; nothing for a lift | D-12, D-15 | the lockdown's reason or trigger on the lift too |
| P6 | The person placeholders name the recorded starter or lifter (recorded name only), or the bot for an automatic lockdown and an anonymous admin | D-13 | always the bot; or the tapper after a proof, as the default text does |
| P7 | After a custom text the bot adds the technical notes after one line break; after a custom lift text the manual-change and unbanning lines | D-16 | no lift lines; a blank line before the notes |
| P8 | 400 UTF-16 units are held back for the notes, so the prompt says "Up to 3696" (624 with media) | D-17 | a smaller reserve |
| P9 | Lockdown messages may carry one media item | D-29 | text and buttons only |
| P10 | Every lockdown event can be silenced; a Silent `/lockdown` or `/unlockdown` answers nothing in the group | D-20 | no Silent on the manual start and the lift |
| P11 | A lift from a menu opened in the group posts the custom lifted message (only when one is stored) | D-22 | keep that path silent whatever is stored |
| P12 | Lockdown messages are never deleted automatically, custom or default (as today) | D-08 | put them on a Bot messages timer |
| P13 | Preview samples: the sample reason; 15 joins in 60 s; 30 messages in 30 s at 80 %; 5 verdicts in 120 s; a lift with no reason | D-32 | other samples |
| P14 | The settings intro is reworded to name lockdowns in all 8 languages | D-30 | keep the intro and rely on the section's own intro |
| P15 | A button lift's `{chatname}` is the lockdown row's stored title (no Telegram call) | D-12 | one `getChat` per lift |

</provisional>

<deferred>
## Deferred Ideas

A custom text for the cannot-lock warning, the alert, its reminders or the lift tally (EVT-10 keeps them unchanged); "already active" as an event; a
private copy of a lockdown message; a Bot messages timer for lockdown messages; a per-language custom message (MSG-F1); custom messages in backups
(Phase 30).

</deferred>

---
*Phase: 25-lockdown-messages*
*Context gathered: 2026-11-22*
