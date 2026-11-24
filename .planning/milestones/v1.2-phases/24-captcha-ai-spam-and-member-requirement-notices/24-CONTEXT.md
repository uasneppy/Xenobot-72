# Phase 24: Captcha, AI Spam and Member Requirement Notices - Context

**Gathered:** 2026-11-21
**Status:** Ready for planning. The decisions marked "owner" were settled before this phase (the milestone decisions of 2026-11-18, the
roadmap's planning notes and success criteria for Phase 24, the requirements EVT-04, EVT-05 and EVT-09, and the lessons of Phase 23). The
decisions marked "default" are the planner's provisional defaults, taken from the documents and the code without asking the owner, because
the owner authorized this run to proceed autonomously; they are reversible, and the ones an owner would notice are listed under "Provisional
decisions for owner confirmation" at the end (P1 to P16).

<domain>
## Phase Boundary

Phase 24 gives three screening surfaces their own custom message, through the event message engine of Phases 21 and 23 and the private copy
engine of Phase 22:

- **Failed captcha** (EVT-04): the notice `handleCaptchaTimeout` posts after the group's failure action (kick, ban or 24 hour mute) took effect.
  That one function serves the timer, the cleanup tick, restart recovery, the math captcha's last wrong answer, and the web captcha's `Fail` and
  `Expire`, so one message serves the timeout and the failed web check alike. The captcha challenge message is not an event and is unchanged.
- **Member requirement notices** (EVT-09): the warn, kick, ban and mute notices `memberReqAnnounce` posts. The bot appends the requirements line
  (the broken rules) and the remedy sentences after a custom text, and a mute notice always keeps its "I fixed it" button, which works as before
  (its 24 hour cap scheduling and its reschedule from the tap included).
- **AI spam notice** (EVT-05): new behaviour in `/aispam`'s delete branch. It is off by default in every group; when on, a deletion posts a notice,
  at most one per member per five minutes and three per group per minute, shed quietly beyond that. Every spam message is still deleted.

The Messages page (Phase 21) gains two events in the Automatic family (Phase 23) and a third family, **Member requirements**, with four events.
Each event opens the editor with a group message, a private copy (Phase 22) and a Silent switch (a Notice switch for the AI spam event, which is
off by default; none for the requirement mute, which must always be posted).

A group with nothing stored gets today's captcha and requirement notices byte for byte (MSG-11, proved by goldens recorded first), and the AI
spam filter still posts nothing in the group. A refused custom message falls back to the default (MSG-04). The deletion timers apply as before
(MSG-09). The punishment and the deletion always come first. The Turnstile and math captcha paths and the web captcha's verify route keep their
behaviour (the answers, the claims, the unmutes).

Out of scope: lockdown messages (Phase 25), timed punishments and escalation (Phase 26), the appeal button (Phase 27), global limits (Phase 28),
plain tone and copy settings (Phase 29), custom messages in backups (Phase 30, MSG-10), a custom message per language (MSG-F1), the captcha
challenge message, the captcha pass summary and success note, the requirement pause notice, the AI breaker notice, the owner-log dump post of an
AI deletion, and any change to the captcha, requirement or AI spam logic itself.

</domain>

<decisions>
## Implementation Decisions

### Scope rules (owner)
- **D-01 (owner):** Everything is configured group by group in that group's settings in the bot's private chat. There are no Staff Group
  defaults and nothing is inherited. The Messages page is the only door: no command sets a message or the AI notice switch.
- **D-02 (owner):** A custom text is one text for every language. With no row (or a row with no text, media or buttons and Silent off), the bot
  sends today's default in the group's language.
- **D-03 (owner):** The punishment comes first. A custom message, a private copy, a Details button or a log line never delays, undoes or skips the
  captcha failure action, a requirement warn, kick, ban or mute, or an AI deletion. A custom message Telegram refuses is replaced by the default,
  sent once (MSG-04).
- **D-04 (owner, roadmap):** The events are the failed captcha (timeout and failed web check alike), the AI spam notice, and the requirement warn,
  kick, ban and mute notices. Captcha and AI spam join the Automatic family; this phase creates the Requirements family (MSG-05). The captcha
  challenge message is not an event (EVT-10).
- **D-05 (owner, roadmap):** The bot appends the list of broken rules, the remedy sentences and the `mreq|v1` "I fixed it" button after a custom
  requirement text. The button is the member's only way out of a requirement mute: it survives any custom message, like Remove warn, through
  `KeepRows`, and the mute notice's 24 hour cap scheduling and reschedule-from-tap rules stay intact. A custom text plus the appended parts must
  still fit Telegram's limits.
- **D-06 (owner, roadmap):** The AI notice is new behaviour in `/aispam`'s delete branch, off by default per group, with Redis counters (one per
  person over a few minutes, one per group per minute) on their own keys outside the cache prefix; it sheds quietly over the cap.
  `aispamNotifyBreaker` stays separate. The deletion itself does not change.
- **D-07 (owner, MSG-11):** The first plan records characterization goldens of every send site this phase changes (the captcha failure notice,
  the requirement notices and their pause notice, which share `memberReqPostNotice`, and aispam's breaker notice and silent delete branch, because
  `aispam.go` changes) and commits them alone, before any production change. They are never re-recorded or weakened.
- **D-08 (owner):** The Turnstile and math captcha paths and the web captcha's verify route do not change behaviour: the same answers, claims,
  actions and unmutes. The only addition on the verify route is the engine's one bounded cached read (`eventMessageReadTimeout`, 2 s) inside
  `Fail` and `Expire`, before the notice.
- **D-09 (owner, roadmap):** Private copies of these events follow Phase 22's rules, including the per-group cap of automatic events. No private
  copy is ever sent for a Staff Group action or a raid-scale removal.

### Which sends are events (default)
- **D-10:** Event keys, validated in Go by `eventmsg.ValidEvent` (appended to `eventmsg.Events` after `fwdblock`, no migration, no CHECK):
  `captcha` (`eventmsg.EventCaptcha`), `aispam` (`EventAISpam`), `mreqwarn` (`EventReqWarn`), `mreqkick` (`EventReqKick`), `mreqban`
  (`EventReqBan`), `mreqmute` (`EventReqMute`). Page codes (the `k` field): `cf`, `ai`, `rw`, `rk`, `rb`, `rm`, so the wait keys are `hcfg`, `haig`,
  `hrwg`, `hrkg`, `hrbg`, `hrmg` (group) and `hcfp` ... `hrmp` (private copy). Family codes: Automatic `a` (Phase 23), Requirements `r`.
- **D-11:** What each event covers:
  - `captcha`: the notice `handleCaptchaTimeout` posts once the failure action took effect (`captcha_timeout_failure_*`, or
    `captcha_timeout_with_messages` when held messages were removed). An action that did not take effect posts nothing, as today; a released
    member (approved, or the captcha turned off) posts nothing, as today.
  - `aispam`: a new notice posted after `deleteMessage` of a delete verdict succeeded (D-23).
  - `mreqwarn`: the requirement warn notice, including its warn-limit variants (`mreq_notice_warn_limit_kick|ban|mute`, chosen by
    `memberReqNoticeKey` when the limit action took effect), with `{action}` naming the limit action then (P2).
  - `mreqkick`, `mreqban`, `mreqmute`: the kick, ban and mute notices.
  - Not events: the requirement pause notice (`mreq_notice_paused`), the AI breaker notice (`aispam_breaker_notice`), the "I fixed it" edit
    (`mreq_notice_fixed`, `mreq_notice_fixed_lockdown`), the captcha challenge, the captcha pass summary and success note, the captcha callback
    answers, the owner-log dump post of an AI deletion, every command reply and every Staff Group message (P3).

### Placeholders (default)
- **D-12:** The Welcome placeholders (`{first}`, `{last}`, `{fullname}`, `{username}`, `{mention}`, `{id}`, `{chatname}`, `{rules}`) are filled for
  the member and the group, as in Phases 21 and 23. The event placeholders take these values (empty renders as nothing, never raw; MSG-02); every
  value is escaped:

  | Placeholder | captcha | aispam | mreqwarn | mreqkick | mreqban | mreqmute |
  |-------------|---------|--------|----------|----------|---------|----------|
  | `{reason}` | `eventmsg_reason_captcha` (new) | `eventmsg_reason_aispam` (new) | the broken rules' phrases (P7) | same | same | same |
  | `{admin}` | "an admin" | "an admin" | "an admin" | "an admin" | "an admin" | "an admin" |
  | `{action}` | the failure action's word | `eventmsg_action_delete` (new) | the warn word, or the limit action's word when the limit action took effect | kick word | ban word | mute word |
  | `{count}` | messages removed while pending (0 renders empty) | empty | the member's warns | empty | empty | empty |
  | `{limit}` | empty | empty | the group's warn limit | empty | empty | empty |
  | `{until}` | the 24 hour unmute time of a mute, in the chat's zone; else empty | empty | empty | empty | empty | empty (the 330 day marker is not a real end, P7) |
  | `{duration}`, `{window}` | empty | empty | empty | empty | empty | empty |

- **D-13:** `{admin}` is the translated `eventmsg_admin_anon` for all six events: each is an automatic action (MSG-03). It never names the Staff
  Group.
- **D-14:** New words: `eventmsg_action_delete` (the participle `deleted`, joining Phase 23's four action words in `eventActionWords`),
  `eventmsg_reason_captcha` and `eventmsg_reason_aispam` (short phrases), `eventmsg_mreq_rules` (the appended requirements line, `{rules}` the
  phrases). A requirement event's `{reason}` is `memberReqRulesText` (the phrases of the broken rules joined by `, `), so the default private text's
  reason line lists them too.
- **D-15:** The requirement site hands the engine a copy of the member with the names stripped of bidirectional controls (`memberReqStripBidi`)
  and cut to `memberReqNameCap`, so a name cannot reorder a custom requirement text either; the names the rules judge are not touched.

### Keyboard, place, timer, fallback, Silent (default)
- **D-16:** Keyboards of a custom group message: `mreqmute` keeps the "I fixed it" row after the custom and `{rules}` buttons (`KeepRows` with
  `KeepRowsLast`, `ReservedButtons` 1; P5). The other five events have no bot row (today's defaults carry none). A preview and a private copy never
  carry a bot row (the button is a group callback).
- **D-17:** A custom message goes where today's default goes: captcha as a new message with no reply and no topic; requirement notices as a new
  message (never a reply: the member's message may be deleted) with no topic; the AI notice (new) as a new message in the deleted message's topic
  with no reply (P11).
- **D-18:** Deletion (MSG-09): the captcha notice keeps today's own 30 second self-delete (`time.AfterFunc`, now `captchaFailureNoticeLife`), custom
  and default alike, and never enters the deletion queue (P9); requirement notices are `bot:punish_auto` by the timer, the mute notice at the 24 hour
  cap (`scheduleBotMessageCapped`), and "I fixed it" reschedules the notice from the tap when the pressed message is the hold's stored notice, which
  is now whichever message was posted (custom or default); the AI notice is `bot:punish_auto` (P11). Silent queues nothing.
- **D-19:** Fallback (MSG-04): a custom message Telegram definitely refuses is replaced by the default, sent once through the site's own default
  send. The requirement site sends both through the requirement pacer and its retry loop (a mute notice is never dropped by pacing), and when
  neither is delivered a mute is lifted by `memberReqUndeliveredMute` exactly as today. The AI notice's sends go through its own pacer; a shed send
  is quiet. Each site keeps its log line and return value.
- **D-20:** Silent posts nothing in the group: the action and the deletion still happen, the private copy is still offered (Phase 22 D-11).
  `mreqmute` cannot be silenced (no Silent switch on its screen, an `hq` press naming it is an invalid request, and the engine ignores a stored
  Silent through `eventReply.Required`), because its notice carries the member's only way out (P6). The AI notice's Silent switch is its on/off
  switch (D-21).
- **D-21:** The AI notice is off by default through the event's own default: `eventmsg.OffByDefault(event)` (true for `aispam` only) and
  `eventmsg.DefaultSilentFor(event, dest)` (`DefaultSilent(dest)` or a group message of an off-by-default event). `Messages.Resolve` and the part
  writers' insert default use it, so storing a text alone never turns the notice on. The engine's group step reads `Resolve` instead of `Lookup`
  (byte-identical for every earlier event), and a read error of an off-by-default event posts nothing (unknown never sends).

### AI notice limits (default)
- **D-22:** The cap is one Lua script (`aispamNoticeScript`) over two keys that share the hash tag `{<chat>}`: `fuku:aispam:notice:{<chat>}:u:<user>`
  (`SET NX EX 300`, `aispamNoticeUserWindow`: one notice per member per five minutes) and `fuku:aispam:notice:{<chat>}:m:<unix minute>` (`INCR`,
  `EXPIRE 120`, at most `aispamNoticePerMinute` = 3 a group a minute; the minute from `aispamNoticeNow`). It fails closed: no Redis or a Redis error
  posts no notice. The engine consults it through `eventSite.Admit` after the read and only when the notice or its private copy would be sent, so a
  group with the notice off spends no Redis command; a refusal sends nothing (no notice, no private copy) and logs one Debug line with the IDs (P10).
- **D-23:** The notice is posted only after the delete verdict's `deleteMessage` returned no error, after `aispamMirrorDeletion` and
  `raidAIRecordVerdict`, in the aispam worker. Every send goes through its own fleet pacer (`aispamNoticePacer`: `fuku:aispam:notice:pace:next` and
  `:block`, Interval 200 ms, MaxRetries 1, MaxWait 2 s), so a wave across many groups is bounded and never holds a worker for long. A check whose
  `aispamContext()` is done posts nothing. No shutdown drain is added: `DrainAISpamChecks`' own 9 s wait covers the notice, and the worst case of
  `AGENTS.md` (70 s) is unchanged.

### Engine (default)
- **D-24:** The engine gains, all optional with today's behaviour as the zero value: `eventSite.SendCustom` (nil: `media.SendGreetingMessage(site.Bot,
  site.Chat.Id, msg)`), `eventSite.Schedule` (nil: `scheduleBotMessage(sent, site.Kind)`), `eventSite.Admit` (nil: always); `eventReply.Required`
  (ignore a stored Silent) and `eventReply.TextOnly` (ignore a stored media file); `eventDefaultInput.Rules` (a requirement rule mask). `postEventSent(site,
  r) (eventGroupPost, error)` returns what the group step posted, and `postEvent` wraps it. Every Phase 21 to 23 request stays byte-identical
  (`TestLegacyModerationRequests`, `TestLegacyPunishmentNoticeRequests`).
- **D-25:** Default builders, each returning today's text and options byte for byte (proven against the goldens before a site switches):
  `captchaFailureDefaultReply` and `aispamNoticeDefaultReply` (new, the `aispam_notice_deleted` text) in `event_message_auto.go`;
  `memberReqNoticeDefault(action int) eventDefaultReply` and `memberReqNoticeNote` (the appended part) in `memberreq_notice.go`.
- **D-26:** Sites: `postCaptchaFailureNotice` in `event_message_auto.go` (so no `captcha*.go` file names the engine; `captcha.go` calls only this
  function); `postAISpamNotice` in the new `aispam_notice.go`; `memberReqPostEventNotice` in `memberreq_notice.go`, called by `memberReqAnnounce`.
  `memberReqPostNotice` stays for the pause notice, and its paced retry loop is shared through one helper so the custom message, the default and the
  pause notice retry alike. The captcha site's chat title comes from the stored `chats` row (`chats.GetChatSettings`, no Telegram call; P16).
- **D-27:** No new table, migration, handler group, shutdown drain, callback namespace or dependency. `go.mod`, `go.sum`, `main.go` and
  `migrations/` are unchanged. The AI notice's keys sit in the existing `fuku:aispam:*` family, outside the cache prefix, and are documented in the
  Data section of `AGENTS.md`.

### Messages page (default)
- **D-28:** The Automatic family gains `captcha` 🧩 (`cf`) and `aispam` 🧠 (`ai`) after `fwdblock`; its list becomes `[2 2 2 1 2]`. The new
  Requirements family is `{Code: "r", Icon: "🛂", NameKey: "settings_msg_fam_mreq", IntroKey: "settings_msg_fam_mreq_intro", WaitTextKey:
  "settings_msg_wait_text_mreq", HintKey: "settings_msg_hint_mreq"}` after the Automatic one, with `mreqwarn` ❗ (`rw`), `mreqkick` 👢 (`rk`),
  `mreqban` ⛔ (`rb`) and `mreqmute` 🔇 (`rm`); its list is `[2 2 2]`. The hub becomes `[2 1 1 1 2]` (three family cells, the two private copy
  options, Back and Close) (P1).
- **D-29:** The Requirements family has the text and buttons parts only (no media part, P4); its group definitions hold back
  `memberReqNoticeNoteReserve` (600 UTF-16 units) from the text limit through a new `settingsEditPageDef.ReservedText`, so its prompt shows
  `Up to 3496` and the check refuses beyond it (P14); its private copy definitions hold back nothing (a private copy appends no requirements line,
  D-35). `mreqmute` has `ReservedButtons` 1 and `NoSilent`.
- **D-30:** The AI spam event's screen draws a Notice switch (`settings_btn_msg_notice`, line `settings_msg_line_notice`, hint
  `settings_msg_hint_notice`) in place of the Silent switch, its state the opposite of Silent, pressing the same `hq` with the Silent target; the hub
  and the family list read `common_status_off` while it is off; Main counts an off-by-default event only when its notice or its private copy is on.
- **D-31:** Strings: `settings_msg_ev_captcha`, `settings_msg_desc_captcha`, `settings_msg_ev_aispam`, `settings_msg_desc_aispam`,
  `settings_msg_fam_mreq`, `settings_msg_fam_mreq_intro`, `settings_msg_wait_text_mreq`, `settings_msg_hint_mreq`, four `settings_msg_ev_mreq*` and
  four `settings_msg_desc_mreq*`, `settings_btn_msg_notice`, `settings_msg_line_notice`, `settings_msg_hint_notice`; reworded in all 8 languages:
  `settings_msg_intro`, `settings_msg_fam_auto_intro` and `settings_msg_wait_text_auto` (it now lists `{until}` for the captcha mute).
- **D-32:** Rights are Phase 21's and 22's: viewing needs the live admin check and the owner gate; Set, Remove, Silent (or Notice) and the private
  copy switch need `can_change_info`, judged live on every press and every typed message (MSG-08). No command exists, so each control keeps drift and
  authority tests in place of a parity test.
- **D-33:** Previews go through the send path's composers with the presser as the member and sample values (P15): captcha kick with 2 removed
  messages; AI spam the delete word; requirement warn 1 of 3 with the username rule, kick and ban with the username rule, mute with the username and
  photo rules (so the remedy shows). A requirement event's Full preview shows the custom text followed by the requirements line and remedies the site
  appends (`settingsMsgEvent.Appended`), never the "I fixed it" row.

### Private copies (default)
- **D-34:** Specs, all `Automatic: true` (capped by the per-group cap of 10 a minute, fail closed): `captcha` (`eventmsg_pm_captcha`), `aispam`
  (`eventmsg_pm_aispam`), `mreqwarn` (`eventmsg_pm_mreqwarn`), `mreqkick`, `mreqban`, `mreqmute` (P13). Each default private text names
  `<b>{group}</b>`, puts `{action}` after a label, uses `{count}` and `{limit}` in the requirement warn's text, and never repeats the reason: the
  reason line (`eventmsg_pm_reason`) follows, which for a requirement event lists the broken rules.
- **D-35:** A private copy of a requirement event appends no requirements line to a custom text (the owner may use `{reason}`) and never carries
  "I fixed it"; `eventmsg_pm_mreqmute` tells the member to fix what is listed and tap "I fixed it" under the group's notice.

### Security and privacy (default)
- **D-36:** No message, private copy, Details record, log line or screen of these events names the Staff Group, the group's linked channel, the AI
  provider, a model, or a verdict's probability or category. The AI notice says only that a message was removed.
- **D-37:** No log line of these paths carries a stored or typed text, a caption, a file ID, a button, a reason, a rule phrase, a message text or a
  name: chat ID, user ID, event and outcome class only.
- **D-38:** Guards: `TestPMCopyCallers` tracks `postEventSent` beside `postEvent` (both forbidden in every staff, lockdown and raid file) with
  allow-list entries for `event_message_auto.go`, `aispam_notice.go` and `memberreq_notice.go`, each with its reason;
  `TestMemberDeletionExcludedMessages` is unchanged (no `captcha*.go` file names a deletion producer, and a behavioural test proves the captcha notice
  never enters the queue with every bot kind at its shortest delay); `TestCommandRepliesStayInsideTheHandler` keeps its empty allow-list (the AI
  notice and the requirement notices are new messages, never replies).

### Tests (default)
- **D-39:** The goldens are one test, `TestLegacyScreeningNoticeRequests` in `fuku/modules/screening_notice_legacy_request_test.go`: captcha kick,
  ban, mute and kick with removed messages (through `handleCaptchaTimeout`); requirement warn, warn limit at kick, ban and mute, kick, ban, mute (with
  its button) and the pause notice (through the executor); aispam's silent delete branch and its breaker notice. Each pins the request, the action
  call and the queue row (or its absence), with the member and hold IDs replaced by tokens.
- **D-40:** `fuku/modules` takes about 38.5 minutes against a 40-minute limit and another task is trimming old slow tests now, so the whole phase is
  budgeted at about 30 seconds of added test time; each plan states its share and records the measured time of its new tests in its SUMMARY. Every
  new test is light: worst cases, never cross products; language sweeps over pure renders only; one environment per dispatcher subtest; real locales
  only where a default text is compared; fresh IDs; cleanup in `t.Cleanup`; no `t.Skip`, no `t.Parallel`, no mock library. The closing plan runs the
  full package once and records its time.
- **D-41:** No PostgreSQL guarantee is added (no migration, index or CHECK), so nothing joins `make test-postgres-integrity`.

### Repo rules that bind
- **D-42:** New locale keys go in all 8 files. `eventmsg_` keys get rows in `eventMsgLocaleLimits`; `settings_` keys rows in `settingsLocaleLimits` and
  label groups in `TestSettingsLabelLengths`; a text that names the "I fixed it" button joins `memberReqQuotingKeys`. Ukrainian and English follow
  `locale-style.md` (`TestLocaleStyle`), placeholders match across languages, HTML only. The rewording of `aispam_help_msg` (it no longer says no
  notice is ever posted) regenerates `docs/src/content/docs/commands/aispam/index.md` through `make generate-docs`.
- **D-43:** `AGENTS.md` is updated in the same commit as the change it describes (the engine hooks, the event list in Data, the AI notice keys, the
  Event messages, PM copies, member requirements, captcha and aispam bullets, the Scheduling bullet's sentence about aispam, the Messages page bullet,
  the test families).
- **D-44:** Every new production identifier has a production caller in the plan that adds it, or a dated `//nolint:unused // first production caller
  arrives in plan 24-NN` that the named plan removes. Lint gates: golangci-lint v2.13.1 (`/root/go/bin/golangci-lint`, `CGO_ENABLED=0`) `run
  --new-from-rev=origin/main ./...`, the same with `--build-tags=notesttools`, and `run --new=false ./fuku/modules/... ./fuku/db/eventmsg/...
  ./fuku/i18n/...` filtered to the touched files. Baseline at 86baf1a (recorded by the planner): in the files this phase touches, only `captcha.go` has
  findings, four of them (`gocyclo` of `captchaVerifyCallback` at 28, `captchaRefreshCallback` at 40 and `handlePendingCaptchaMessage` at 24, and
  staticcheck QF1003 at line 1220); `fuku/i18n` and `fuku/db/eventmsg` report 0 issues. The gate allows those four and nothing else (line numbers may
  move), `handleCaptchaTimeout` and `processAISpam` must not appear, and no AI model is named in code, docs or commit subjects.

### Planner's and Executor's Discretion
Function, field and test names inside the prefixes above, the exact strings (inside their limits and the style guide), the order of lines inside a
screen body, how a site's default send closure is written, and how a test's cases are split into subtests.

</decisions>

<open_questions_resolved>
## Roadmap Planning Notes and Open Questions, Resolved

| Question | Resolution |
|----------|------------|
| One message for the timeout and the failed web check | D-11: the notice of `handleCaptchaTimeout`, which every failure path reaches |
| The captcha challenge message | not an event, unchanged (D-04) |
| How "I fixed it" survives a custom message | D-16: `KeepRows` with `KeepRowsLast`, `ReservedButtons` 1, `Required` so the mute notice cannot be silenced (D-20) |
| The rules list and remedies after a custom text, inside Telegram's limits | D-14, D-29: `memberReqNoticeNote` through `eventReply.Note`; 600 units held back by the editor, the worst real note measured in 8 languages; no media part (P4) |
| 24 hour cap and reschedule from the tap | D-18: the requirement site's `Schedule` hook is `memberReqScheduleNotice`; the stored notice ID is the posted message's |
| The AI notice off by default | D-21: `eventmsg.OffByDefault`, `DefaultSilentFor`, `Resolve` in the engine |
| The AI notice counters and shedding | D-22, D-23: one script over a per-member and a per-minute key, fail closed, through `eventSite.Admit`; its own pacer |
| `aispamNotifyBreaker` stays separate | D-11 (not an event), pinned by the goldens (D-07) |
| Goldens first | D-07, D-39 |
| Which files may call the engine | D-26, D-38 |
| PM specs | D-34: all six `Automatic: true` |
| The verify route's behaviour | D-08: unchanged answers; one bounded cached read added before the notice |

</open_questions_resolved>

<screens_and_codes>
## Screens and Callback Codes

No new action code: the new events reuse the Messages page's codes (`hl he hs hr hc hv hf hq hp ho`) with new values only.

- **Hub** (`a=op&p=h`): the intro, the manual family, the Automatic family (seven state lines), the Requirements family (four state lines), then the
  private copies block. Rows `[2 1 1 1 2]`.
- **Automatic list** (`a=hl&t=a`): seven event cells, rows `[2 2 2 1 2]`. **Requirements list** (`a=hl&t=r`): four cells, rows `[2 2 2]`.
- **Event screen** (`a=he&k=<code>`): Phase 21's screen. The AI spam screen draws the Notice switch in place of Silent. The requirement screens have no
  media rows; `rm` has no Silent row. Worst shapes: Automatic events `[2 2 2 2 2 1 1 2]`; `rw rk rb` `[2 2 2 1 1 1 2]`; `rm` `[2 2 2 1 1 2]`.
- **Private copy screen** (`a=he&k=<code>&d=p`): Phase 22's. Requirement events have no media rows: worst `[2 2 2 1 1 2]`.

Worst callback sizes at a 14-character chat ID are Phase 22's: `hs hr hc hv` with `d=p` 46 bytes, `he` and `hf` with `d=p` 42, `hp` 42, `ho` 41,
`hq` 42; the two-letter event codes have the length of the manual ones.

</screens_and_codes>

<code_anchors>
## Code Anchors (at 86baf1a)

- `fuku/modules/captcha.go`: `runOrphanedCaptchaRecovery` (253, calls `expireCaptchaAttempt` at 296), `expireCaptchaAttempt` (1447),
  `expireCaptchaAttemptApplied` (1455), `settleCaptchaAttempt` (1480), `handleCaptchaTimeout` (1512: the claim, the challenge delete, the
  `getChatMember` name with the "User" fallback, `executeCaptchaFailureAction`, `settleFailedCaptchaAction`, then the notice from
  `buildCaptchaFailureMessage` sent with `helpers.SendMessageWithErrorHandling(bot, chatID, text, &gotgbot.SendMessageOpts{ParseMode:
  formatting.HTML})` and deleted after 30 s by `time.AfterFunc`, lines 1568 to 1582), `buildCaptchaFailureMessage` (1586), the math captcha's last
  wrong answer (1965), `cleanupExpiredCaptchaAttempts` (2311, 2346).
- `fuku/modules/captcha_web.go`: `captchaWebBackend.Fail` (376), `Expire` (424), `decidedElsewhere` (457).
- `fuku/modules/memberreq_notice.go`: `memberReqRulesText` (61), `memberReqNoticeParams` (94), `memberReqNoticeKey` (110), `memberReqNoticeText`
  (136), `memberReqNoticeRemedies` (150), `memberReqPostNotice` (174: the paced send and its retry loop, then `memberReqScheduleNotice`),
  `memberReqScheduleNotice` (217), `memberReqUndeliveredMute` (263), `memberReqFixedButton` (300), `memberReqNoticeKeyboard` (317).
- `fuku/modules/memberreq_enforce.go`: `memberReqTranslator` (153), `memberReqAnnounce` (559: text, `memberReqPostNotice`, the undelivered mute,
  `memberreq.SetHoldNotice`), `memberReqPunish` (579), `memberReqEnforce` (630). `memberreq_limits.go`: `memberReqPauseChat` (103).
  `memberreq_fixed.go`: `memberReqIsOwnNotice` (226), `memberReqFixedEdit` (301). `memberreq_rules.go`: the rule table (41), `memberReqStripBidi` (353),
  `memberReqMention` (371).
- `fuku/modules/aispam.go`: `aispamTask` (113), `checkAISpam` (157), `processAISpam` (264: the delete at 328, `aispamMirrorDeletion` at 336,
  `raidAIRecordVerdict` at 341), `aispamNotifyBreaker` (371), `aispamContext` (435), `DrainAISpamChecks` (518).
- `fuku/modules/event_message.go`: `eventValues` (46), `eventDefaultInput` (67), `eventReply` (89), `eventSite` (106), `composeEventMessage` (237),
  `postEvent` (317), `postEventGroup` (346, reads `Lookup`), `sendDefaultPost` (377), `offerReplyPM` (397).
- `fuku/modules/event_message_auto.go`: the Phase 23 builders, `eventActionWords` (124), `eventActionWord` (135), `listNoticeSite` (172).
- `fuku/modules/event_message_pm.go`: the spec init (67 to 87), `offerEventPM` (292), `eventPMDefaultText` (400).
- `fuku/modules/settings_messages.go`: `settingsMsgSample` (79), `settingsMsgEvent` (95), `settingsMsgFamily` (111), `settingsMsgAutoEvents` (154),
  `settingsMsgFamilies` (183), `settingsMsgPartsOf` (206), the page's `init` (243), `settingsMsgEditDef` (325), `settingsMsgSilentFields` (413),
  `settingsMsgStateOf` (499, reads `Lookup`), `settingsMsgCountChanged` (535), `settingsMsgPreviewReply` (800), `renderSettingsMsgHub` (934),
  `settingsMsgSwitchRow` (1007), `renderSettingsMsgEvent` (1028).
- `fuku/modules/settings_edit.go`: `settingsEditPageDef` (90, `ReservedButtons` at 112). `settings_edit_check.go`: `settingsEditCheckGreetingText`
  (127), `settingsEditMediaCaptionRefusal` (290). `settings_edit_actions.go`: `settingsEditPromptMax` (42).
- `fuku/db/eventmsg/events.go`: `Events`, `ValidEvent`, `DefaultSilent`; `repository.go`: `Resolve` (73), `upsert` (242).
- Tests: `punishment_notice_legacy_request_test.go` (`noticeGoldenClient`, `newNoticeGoldenSite`, the golden pattern), `memberreq_notice_schedule_test.go`
  (`newMreqSchedEnv`, `wantRowFor`, `mreqSchedMuted`), `memberreq_env_test.go`, `captcha_web_outcomes_test.go` (`TestCaptchaWebFailedSolveAppliesAction`),
  `captcha_test.go` (`TestHandleCaptchaTimeoutAppliesKickMuteAndSkipsStaleAttempts`), `aispam_test.go` (`withAISpamTestConfig`, `withAISpamJevServer`,
  `aispamProcessUpdate`, `waitAISpamChecks`), `event_message_pm_env_test.go`, `settings_messages_env_test.go`, `settings_messages_screens_test.go`
  (`settingsMsgScreens`, `settingsMsgFamilyWidths`), `settings_messages_safety_test.go`, `auto_messages_e2e_test.go`, `pm_copy_callers_test.go`,
  `member_deletion_e2e_test.go` (`memberExcludedProducers`), `fuku/i18n/eventmsg_locale_test.go`, `fuku/i18n/settings_locale_test.go`,
  `fuku/i18n/memberreq_locale_test.go` (`memberReqQuotingKeys`).

</code_anchors>

<canonical_refs>
## Canonical References

- `AGENTS.md`: Data (`chat_event_messages`, the `fuku:pmcopy:*` and `fuku:memberreq:*` keys), Subsystem traps (Event messages, PM copies, the member
  requirements executor and "I fixed it", Turnstile captcha, Scheduling the bot's own messages, the aispam and raid AI bullets), Callbacks (`mreq|v1`, the
  Messages page, the message editor), Permissions, Testing.
- `.planning/phases/23-warn-flood-blacklist-and-forward-blocklist-messages/` (the closest model: `23-CONTEXT.md`, the seven plans, the summaries,
  `23-VERIFICATION.md`), `.planning/phases/22-pm-copies/` and `.planning/phases/21-message-engine-messages-page-and-manual-punishments/`.
- `.planning/ROADMAP.md` Phase 24 planning notes and success criteria; `.planning/REQUIREMENTS.md` EVT-04, EVT-05, EVT-09 and the cross-cutting MSG
  and PMSG rules.

</canonical_refs>

<owner_visible>
## Behaviour an Owner Would Notice

Nothing changes for a group that sets nothing: the captcha and requirement notices are byte-identical and the AI spam filter still posts nothing.
When the owner sets messages:
- the Automatic section gains Failed captcha and AI spam, and a new Member requirements section lists warn, kick, ban and mute;
- a custom requirement text is always followed by the requirements line (and the remedy sentences), and a custom mute notice keeps "I fixed it" under
  the custom buttons; the mute notice has no Silent switch and no media part, and the requirement texts are held to 3496 characters;
- the AI spam event has a Notice switch, off until the owner turns it on; then a deletion posts a notice, at most one per member every five minutes and
  three per group a minute;
- the captcha message, custom or not, still deletes itself after 30 seconds;
- `{admin}` reads "an admin" on every one of these events.

</owner_visible>

<provisional>
## Provisional Decisions for Owner Confirmation

| Id | Decision | Where | Alternative the owner may prefer |
|----|----------|-------|----------------------------------|
| P1 | Event codes `cf ai rw rk rb rm`, keys `captcha aispam mreqwarn mreqkick mreqban mreqmute`; Requirements family `r` 🛂 "Member requirements"; icons 🧩 🧠 ❗ 👢 ⛔ 🔇 | D-10, D-28 | other names, icons or order |
| P2 | The requirement warn event also covers the warn-limit notices, with `{action}` naming the limit action | D-11 | a fifth event, "requirement warn limit" |
| P3 | The requirement pause notice and the AI breaker notice are not events and keep today's text | D-11 | make them events too |
| P4 | Requirement events have no media part (a media item could not carry the requirements line, a sticker drops the text and "I fixed it" edits text) | D-29 | allow media and edit the caption on "I fixed it" |
| P5 | The requirements line and remedies follow a custom text after a blank line; "I fixed it" goes below the custom buttons | D-16, D-14 | the button above the custom buttons |
| P6 | The requirement mute notice cannot be silenced; warn, kick and ban can | D-20 | allow Silent and lift the mute instead (no notice, no mute) |
| P7 | A requirement event's `{reason}` is the broken rules' phrases; `{until}` stays empty for a requirement mute | D-12, D-14 | a fixed reason phrase; the marker date |
| P8 | Captcha `{action}` is the failure action's word, `{count}` the removed messages, `{until}` the 24 hour unmute of a mute, `{reason}` a new phrase, `{duration}` empty | D-12 | `{duration}` "24 hours" |
| P9 | The captcha message keeps today's 30 second self-delete, custom or default, and never uses the Bot messages timer | D-18 | the automatic punishments timer |
| P10 | AI notice caps: one per member per 5 minutes, 3 per group per minute, fail closed without Redis; a refusal also sheds its private copy | D-22 | other numbers; send the private copy anyway |
| P11 | The AI notice is a new message in the deleted message's topic, never a reply, deleted by the automatic punishments timer | D-17, D-18 | no topic; no deletion |
| P12 | The AI notice's default text names the member with a mention ("I removed a message from {name}: it looked like spam") | D-25 | an anonymous "I removed a spam message" |
| P13 | All six events have a private copy, each automatic (capped), off by default | D-34 | no private copy for the AI notice |
| P14 | 600 UTF-16 units are held back from a requirement text, so the prompt says "Up to 3496" | D-29 | a smaller reserve, measured per group |
| P15 | Preview samples: captcha kick with 2 removed messages, AI delete, requirement warn 1 of 3 (username), kick and ban (username), mute (username and photo) | D-33 | other samples |
| P16 | The captcha message's `{chatname}` and the private copy's group name come from the stored `chats` row (no Telegram call) | D-26 | one `getChat` per failure |

</provisional>

<deferred>
## Deferred Ideas

A requirement warn-limit event of its own; custom text for the pause notice and the AI breaker notice; media on requirement notices; a per-language
custom message (MSG-F1); custom messages in backups (Phase 30); the lockdown messages (Phase 25).

</deferred>

---
*Phase: 24-captcha-ai-spam-and-member-requirement-notices*
*Context gathered: 2026-11-21*
