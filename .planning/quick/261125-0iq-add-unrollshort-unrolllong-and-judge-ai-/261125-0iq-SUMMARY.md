---
phase: quick-261125-0iq
plan: 01
status: complete
commits: 6
plan_head_before: 2294a42f536878fad75f8a903b404ebd8c9f9e23
plan_head_after: 129d200
tasks_done: [1, 2, 3]
tasks_pending: []
verification: human_needed
review: 5 warnings fixed (WR-01..WR-05), IN-02 fixed, IN-01/03/04 left as is
---

# Quick 261125-0iq: /unrollshort, /unrolllong, /judge

All three tasks are committed: `e2a219e` (Task 1, log and /unrollshort), `c1ec853` (Task 2, /unrolllong, /judge, destinations and the page's judge choice), `06a3490` (Task 3, help in 8 languages, docs, safeguards). Verification (`261125-0iq-VERIFICATION.md`) scored 8/8 must-haves, status human_needed (a real-provider check of /judge and the briefs). The code review (`261125-0iq-REVIEW.md`) found 5 warnings and 4 info items, no blockers; fixes are `6920605` (WR-05: the log migration test in the PostgreSQL integrity job), `e4b4ca8` (WR-01 caption argument, WR-02 unique labels and name cleaning, WR-03 cut mark, WR-04 anonymous requester) and `129d200` (IN-02 notice wording). IN-01 (bare-domain links, pre-existing filter), IN-03 (any admin may override the stored judge default with the argument) and IN-04 (a partial delivery is reported as a failure) are left as is.

Deviation left open: the generated Chat docs table shows "No description available" for the three commands (the generator reads only ": " or " - " separators); fixing the generator or freezing the page is the owner's call.

## Task 1 delivered

- Migration `20261125120000_add_chat_conversation_log.sql`: `log_enabled` and `judge_to` (inline named CHECK) on `chat_conversations`. Model, `chatconv.Settings.LogEnabled` (msgpack `l`), `SetLogEnabled`. `judge_to` is stored but not exposed yet (Task 2).
- `applyChatLog` and `chatWipeLog` (Off deletes the ring; `chatWipeMemory` and `chatWipeLog` share `chatWipeKeys`), `chatLogKey`.
- Conversations page: Message log toggle (control `l`, one tap On, Off deletes the log), status line, data notice, toggle row after the audience row.
- `chat_log.go`: Redis ring (Lua insert and edit scripts, `ZCOUNT` presence, `ZRANGE BYSCORE` reads), media tag mapper, group 14 watcher with edits.
- `chat_brief_prompt.go`: pure labels, stretch, input cap, transcript, request, parts and message assembly (short kind only).
- `chat_brief.go`: `/unrollshort`, gate, own queue (16) and one worker started inside `chatEnsureWorkers`, cooldown, budgets (`brief:b`), delivery as HTML with `AllowSendingWithoutReply` true.
- Plumbing: `chatRequest.MaxTokens`, `chatSanitizeText`, `chatJob.allowMissingReply`, `chatReserveWith`, `chatClaimCooldown`, brief counters on `/stats`.
- Locales in all 8 files (`settings_chat_log_*`, `settings_btn_chat_log`, nine `brief_*` keys, two reworded `settings_chat_*` keys), limits rows, `TestChatBriefLocaleKeys`.
- AGENTS.md: handler group 14, Chat module, `/unrollshort` bullet, Conversations page, `chat_conversations` data, Redis keys, StopChat, ENABLE_CHAT default, provider clients, test families.
- Tests: ring, media tag, watcher (skips, never ends update, all modules), prompt, parts, messages, short, refusals, limits, provider answers, queue, end to end, apply, settings page, repository log subtest, migration (statements and PostgreSQL-only defaults), provider `MaxTokens` on both stand-ins.

## Verify output

| Command | Result |
|---|---|
| `CGO_ENABLED=0 go build ./...` | pass |
| `go test -tags testtools -race -count=1 -run 'TestChatConversation\|TestChatConversationsStayOutOfBackup' ./xenobot/db/chatconv ./xenobot/db/migrations ./xenobot/db/backup` | pass |
| `go test -tags testtools -race -count=1 -run '^(TestChat\|TestApplyChat\|TestSettingsChat\|TestSettingsDriftChat\|TestShutdownEndsChat\|TestCommandRepliesStayInsideTheHandler\|TestNoRawNewCommand\|TestDisableRefusesNamesThatHaveNoCheck\|TestHelp\|TestSettingsControlActionAllowList\|TestSettingsMain\|TestSettingsStaffPrivacy)' ./xenobot/modules` | pass (25.6 s) |
| wider run (`TestSettings\|TestLoad\|TestModules\|TestEvent\|TestPMCopy\|TestNewDispatcher\|TestBotCommand\|TestAllModules\|TestHelp\|TestCommand\|TestDisable\|TestNoRaw\|TestStats\|TestDocs\|TestChat\|TestApply\|TestShutdown`) | pass (141 s) |
| `go test -tags testtools -race -count=1 ./xenobot/i18n` | pass |
| `make check-translations` | pass |
| `make check-docs` | pass (after `make generate-docs`) |
| `make lint` | the installed golangci-lint was built with go1.25 and refuses the go1.26 config; ran a locally built v2.15.0 (go1.26.11) with the repo config over `./...`: 0 issues |

## Deviations from Plan

1. **[Rule 3 - Blocking] lint tool.** `make lint` cannot run here (see above); lint was run with a locally built golangci-lint v2.15.0 in the scratchpad instead. It found one staticcheck issue (deprecated `ZRangeByScore`), fixed with `ZRangeArgs`.
2. **`unrollShortDesc` lives in `chat.go`**, not `chat_brief.go`. The docs generator matches the descriptor and the `WrapCommand` call within one file, so both sit beside `LoadChat`. Task 2 should put `unrollLongDesc` and `judgeDesc` there too.
3. **`chatBriefSelect(entries, maxRunes)`** counts a line as name plus text plus tag plus a fixed 24-rune overhead (an over-estimate), so it stays pure on the entries as planned.
4. **`chatBriefParts(text, maxUnits)` measures the HTML-escaped length** of each part (not the plain length), so a part of `&` characters cannot grow past a Telegram message after escaping. Parts are still at most 3000 units, at line breaks, at most 5.
5. **Provider error answer** uses the gated `brief_malfunction` line (once per 5 minutes per chat), as the conversation does, rather than an ungated line.
6. **`brief_*` key limits** (English, other): off_hint 200/280, unavailable 80/120, usage_short 200/280, too_old 220/300, header_short 60/80, ai_notice 100/140, malfunction 80/120, allowance 90/130, refused 70/100.

## Notes for the Task 2 and Task 3 executors

- The generated docs page `docs/src/content/docs/commands/chat/index.md` now has a generic "Available Commands" table listing only `/unrollshort` with "No description available" and Disableable no (the generator found the named descriptor; `/ask` has an inline descriptor and is not listed). Task 3 should update `chat_help_msg` and either make the help lines parseable by the generator or mark the page `<!-- MANUALLY MAINTAINED: do not regenerate -->`, then rerun `make generate-docs` and `make check-docs`.
- `judge_to` exists in the model and migration but nothing reads or writes it; AGENTS.md says so and Task 2 must update that sentence.
- `chatBriefKind` has only `chatBriefShort`; the instruction and token-cap helpers use `if kind == chatBriefShort` so Task 2 can turn them into switches.
- Test toolkit added: `chatEnv.fake`, `chatEnv.deliverEdit`, `chatEnv.briefCommand`, `chatEnv.markDeleted`, `chatEnv.logged`, `chatLogTurnOn`, `briefTalk`; `withChatRuntime` swaps the brief cooldown, budgets, `chatLogMax` and `chatBriefMaxInputRunes`.

## Known Stubs

None.

## Self-Check: PASSED

- Commit `e2a219e` is on `claude/confident-cerf-qy7qke`; all new files exist (migration, `chat_log.go`, `chat_brief.go`, `chat_brief_prompt.go`, the tests and the i18n test).
