---
phase: quick-261125-0iq
reviewed: 2026-11-25T02:19:04Z
depth: standard
files_reviewed: 12
files_reviewed_list:
  - migrations/20261125120000_add_chat_conversation_log.sql
  - xenobot/db/chatconv/repository.go
  - xenobot/db/models/chat_conversation.go
  - xenobot/modules/chat.go
  - xenobot/modules/chat_brief.go
  - xenobot/modules/chat_brief_prompt.go
  - xenobot/modules/chat_log.go
  - xenobot/modules/chat_prompt.go
  - xenobot/modules/chat_run.go
  - xenobot/modules/chat_send.go
  - xenobot/modules/chat_settings.go
  - xenobot/modules/settings_chat.go
findings:
  critical: 0
  warning: 5
  info: 4
  total: 9
  necessity: 0
status: issues_found
---

# Quick 261125-0iq: Code Review Report

**Reviewed:** 2026-11-25T02:19:04Z
**Depth:** standard
**Files Reviewed:** 12 production files (tests read only where they bear on reliability)
**Status:** issues_found

## Summary

Scope: `git diff 2294a42..06a3490 -- '*.go' migrations/` (message log, `/unrollshort`, `/unrolllong`,
`/judge`, the Conversations page's log and judge controls, `chatconv`, migration 20261125120000).

What holds up under tracing:

- `/judge` authority: `requireLockdownAuthority(false)` is the last required check, live, and is re-run
  for the tapper by `anonPipelineHandler`; `verifyAnonymousAdmin` clears `SenderChat`, so
  `raidFloodSenderCounts` accepts the tapper on re-entry. The `pm` recipient is the update's sender and
  the Staff Group comes from `staff.GetLinkOfGroupFresh`, read at the handler and again before the send.
- Settings: both new controls declare `settingsRightOwner`, carry strict `Validate`/`Actions`, and refuse
  a group-menu press (`settingsChatGroupRefused`) before any read or write. Writes are column-targeted
  upserts that drop the `chat_conv` key (in `skipLocal`).
- Lua: both ring scripts are single-key (hash-tagged), replace-at-score then trim by rank, and the edit
  guard returns before any write; `chatLogHas` correctly uses `ZCOUNT` on the score.
- UTF-16: `chatBriefParts`/`chatBriefCutLine` measure on the escaped text, cut at rune boundaries (no
  surrogate split), and each assembled message stays well under 4096 units.
- Shutdown: brief jobs are counted in `chatJobs` before `chatDraining` is read, derive from
  `chatContext()`, the worker recovers from panics, and queued jobs are shed once the root is cancelled.
- No user ID or username field reaches the provider; labels are first names; text is newline-free
  (`chatprompt.Clean`), so a member cannot start a transcript line of their own.

Findings below are verified against the source. None is a crash or a data-loss path; the most
consequential are a `/judge pm|staff` verdict that can land publicly in the group (WR-01) and labels
that are not unique, so the verdict can attribute one member's lines to another (WR-02).

## Warnings

### WR-01: `/judge` in a media caption ignores its destination word and posts to the stored default

**File:** `xenobot/modules/chat_brief.go:251`
**Issue:** The destination is parsed from `chatCommandArgument(c.Msg.Text)`. gotgbot's
`Command.checkMessage` matches the command on `msg.GetText()`, which falls back to the caption, so a
photo/screenshot captioned `/judge pm` (or `/judge staff`) replying to the start message runs the
handler, but `c.Msg.Text` is empty, the argument reads as none, and the stored default is used.
**Failure scenario:** owner leaves the default at Group (0). An admin attaches a screenshot as evidence
with caption `/judge pm` in reply to the first message of a dispute. The verdict — guilt scores 1–10
with reasons naming members — is posted publicly in the group instead of the admin's private chat. A
caption `/judge foo` is likewise not refused with `judge_usage`.
**Fix:**
```go
dest, ok := chatJudgeDestination(chatCommandArgument(chatMessageText(c.Msg)), gate.judgeTo)
```
(`chatMessageText` already returns the text or the caption.) Add a caption case to `TestChatJudge`.

### WR-02: Transcript labels are not unique; a member can collide with, or impersonate, another

**File:** `xenobot/modules/chat_brief_prompt.go:106-133` (and `chatBriefRoster` at 243-256)
**Issue:** `chatBriefLabels` groups by the lower-cased base name and appends ` #N` only inside a
group, but never checks the suffixed label against other bases. Members A="Bob", B="Bob", C="Bob #1"
yield labels A="Bob #1", B="Bob #2", C="Bob #1" — two people share one label. `chatBriefRoster` then
keys `shown` by label, so C's kept lines mark A as shown, and the roster lists "Bob #1" twice. Names may
also contain `": "` (e.g. first name `Alice: I`), so a line `Alice: I: <text>` reads as Alice speaking.
AGENTS.md promises "one label per person" and that the roster "lists every participant once".
**Failure scenario:** in a dispute, a member renames to `Bob #1` before an admin runs `/judge`. The model
receives two indistinguishable "Bob #1" speakers and a roster with a duplicate name; the verdict can
assign C's hostile lines and score to A. Because the stretch is computed from the newest name of each
user, the rename takes effect retroactively for all of C's earlier lines.
**Fix:** make labels injective over the stretch: after computing candidate labels, loop until no two
user IDs share a lower-cased label (e.g. keep a `used` set and bump the suffix until free), and strip or
replace `:` and `[`/`]` in names (`chatprompt.Name` output) before use, so a name cannot mimic the
`label: text` form or a bracketed system line. Add a test with `Bob`, `Bob`, `Bob #1`.

### WR-03: Answers longer than 5 parts are cut silently, dropping the end of a verdict

**File:** `xenobot/modules/chat_brief_prompt.go:343-345`
**Issue:** `chatBriefParts` keeps the first 5 parts of at most 3000 escaped UTF-16 units and discards the
rest with no marker. `/judge` allows 4000 output tokens (~16,000 Latin characters), and
`html.EscapeString` turns every `"` and `'` into 5 units (`&#34;`, `&#39;`) and `&` into 5, so an answer
well under the token cap can exceed 15,000 escaped units. The "…" appended for `chatTruncated` is in the
dropped tail too.
**Failure scenario:** a 40-name verdict with quotations near the token cap: the last roster entries are
never delivered, the closing block still says nothing is missing, and AGENTS.md's guarantee "nobody is
silently left out of a verdict" is false for those members.
**Fix:** when `len(parts) > chatBriefMaxParts`, truncate and append a visible mark (e.g. "…" plus a
translated `brief_cut_note`) to the last kept part, measured within the unit cap; or lower
`chatBriefMaxTokens(chatBriefJudge)` so 5 × 3000 escaped units always fit.

### WR-04: An anonymous admin's name is put on a `/judge staff` verdict

**File:** `xenobot/modules/chat_brief.go:268`, `xenobot/modules/chat_brief.go:466-473`
**Issue:** `job.requester = c.User.FirstName` and `judge_header_other` renders "Requested by {name}". On
the anonymous re-entry `c.User` is the tapper, so the Staff Group sees the real admin who chose to act
anonymously. `verifyAnonymousAdmin` marks the update (`markAnonymousActor`) precisely so visible text
reads "an admin" (D-12, D-13: never name the tapper); this site ignores `actorIsAnonymous(c.Ctx)`.
**Failure scenario:** an anonymous admin runs `/judge staff`; the Staff Group (whose members need not be
admins of the group) reads "Requested by <tapper's first name>".
**Fix:**
```go
job.requester = c.User.FirstName
if actorIsAnonymous(c.Ctx) {
    if anon, err := c.Tr.GetString("eventmsg_admin_anon"); err == nil && anon != "" {
        job.requester = anon // "an admin", in the group's language
    }
}
```
(the private destination may keep the name, since it goes to the tapper alone).

### WR-05: The judge_to CHECK migration test never runs on PostgreSQL in CI

**File:** `Makefile:76` (with `xenobot/db/migrations/chat_conversation_log_migration_test.go:70-77`)
**Issue:** `TestChatConversationLogMigrationDefaults` skips unless `XENOBOT_TEST_DATABASE=true`. In CI
that variable is set only for `make test-postgres-integrity`, whose `-run` list does not include the
test (it includes the sibling `TestChatConversationMigrationDefaults`). In `test-rest` it skips and the
skip gate accepts it (`postgresSkip` allows every migrations-package skip). AGENTS.md (Testing): "Add a
test there when a new guarantee rests on a partial unique index, a `CHECK` or a conditional update".
**Failure scenario:** a regression in `chk_chat_conversations_judge_to` or the statements' idempotency on
PostgreSQL (the only database that runs the SQL file) ships green.
**Fix:** append `|TestChatConversationLogMigrationDefaults` (and the `chatconv` log/judge upsert test if it
is a separate top-level name) to the `-run` pattern of `test-postgres-integrity`.

## Info

### IN-01: Bare-domain links survive the output filter and are amplified by briefs

**File:** `xenobot/modules/chat_send.go:55`
**Issue:** `chatLinkPattern` only removes `http(s)://`, `tg://`, `www.`, `t.me/` and `telegram.me/` tokens.
Telegram clients auto-link bare domains (`scam.example/path`) in HTML messages. Briefs reproduce member
text, including messages the blacklist/aispam deleted after they were logged, so a brief can repost a
clickable spam link in the bot's name (previews are off). Pre-existing filter, new exposure.
**Fix:** also replace `\b[\w-]+(\.[\w-]+)+/\S*` and known-TLD bare domains, or escape dots in the brief.

### IN-02: The data notice says usernames and phone numbers are never sent

**File:** `locales/en.yml:2390` (rendered by `settings_chat.go:174`)
**Issue:** account usernames are not sent, but member-typed text (`@handles`, phone numbers) is sent
verbatim in the transcript. The sentence overstates the guarantee to the owner.
**Fix:** say "never sent: user IDs, account usernames or phone numbers, other groups; message text is
sent as written".

### IN-03: The owner's destination default can be overridden by any admin

**File:** `xenobot/modules/chat_brief.go:220-240`
**Issue:** `chatJudgeDestination` always lets the argument override the stored `JudgeTo`; an owner who
sets Private chat cannot prevent `/judge group` from posting scores publicly, or `/judge staff` from a
linked-group admin who is not staff. Documented as intended, but the page presents it as a setting.
**Fix:** state on the page that it is only a default, or add an owner switch restricting destinations.

### IN-04: Partial delivery is reported as a full failure

**File:** `xenobot/modules/chat_brief.go:519-560`, `xenobot/modules/chat_brief.go:563-570`
**Issue:** `chatBriefSendAll` stops at the first failing message. If part 1 of a private or Staff Group
verdict is delivered and part 2 hits a 429 past the pacer, the group is told `brief_malfunction` /
`judge_staff_send_failed` and the counters say failed, though a truncated verdict was posted.
**Fix:** track how many parts were sent and, after a partial send, report a distinct line or count it
answered with a warning.

---

_Reviewed: 2026-11-25T02:19:04Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
