# Quick Task 261125-0iq: Research

Source: gsd-phase-researcher report (the agent could not write files; the orchestrator saved it). Tags: [V] read in source (path:line), [R] AGENTS.md rule, [A] assumed.

## Summary
- Logger: a new handler at group **14** (nothing uses 14+). Before group 13 only the captcha hold (-10), join/service handlers and the private settings door (1) end a non-command message update, so group 14 sees every real member text, caption and edit that reached the chat watcher, including messages aispam/antiflood/locks/blacklist deleted. [V]
- Storage: one Redis sorted set per group, score = Telegram message ID, member = compact JSON, written by one Lua upsert script; precedent `aispamHistoryScript` (aispam.go:654-669), `raidFloodScript` (raid_flood.go:100). [V]
- Risks: command deletion removes the reply target after the handler returns; prompt size with up to 1000 entries; the brief gate must not reuse `chatAccept`; provider calls cap at 40 s; deleted spam stays in the log.

## 1. Logger placement and edits
- gotgbot: `ContinueGroups` tries the next handler of the same group; `EndGroups` stops every later group; any other error/nil leaves the current group only [V gotgbot dispatcher.go ~294-318]. Main's error hook returns Noop [V main.go:465-474].
- Commands (group 0) end updates, so command messages are never logged (fine). Filter (9), blacklist (7), report (8), aispam (3), antiflood (4), locks (5/6), raid watchers return ContinueGroups or a plain error.
- Wiring: `chatLogGroup = 14` beside `chatWatchGroup = 13` (chat_watch.go:32); register in `LoadChat` (chat.go:24-41) with `handlers.NewMessage(message.All, fn).SetAllowEdited(true)`; gotgbot uses EditedMessage only when AllowEdited and ctx.Message is nil [V ext/handlers/message.go:55-75], so one handler serves both; branch on `ctx.EditedMessage != nil`. Channel posts and bot senders are excluded unless their Allow setter is called.
- Reuse gates: `raidFloodSenderCounts` (raid_flood.go:229-245), `memberContentMessage` (member_deletion.go:382), `raidFloodIsSlashText`. Media tag: new pure mapper (photo, sticker, voice, video, video note, animation/GIF, audio, document, poll, location, contact, dice...); `raidFloodItemKey` (raid_flood.go:133-165) lists the fields.
- An edit of an ID not in the ring must not insert it.
- Test pattern: `TestChatWatcherGroupReachedByMention` (chat_group_test.go).

## 2. Redis ring
- Key `xenobot:chat:{<chat>}:log` (outside `xenobot:cache:`, chat ID hash tag, chat_settings.go:64-66 helpers).
- Score = message ID (exact in float64); member = JSON {m id, u user ID, n first name, t text capped, k media tag, e edited flag, at}.
- One Lua script for insert: `ZRANGEBYSCORE key id id` → `ZREM` any existing member at that score → `ZADD` → if `ZCARD > 1000` `ZREMRANGEBYRANK key 0 (n-1001)` → `EXPIRE`. Edit script: same but returns early when no member exists at that score.
- Reads: `ZSCORE key replyID` (missing → refuse, no AI call); `ZRANGEBYSCORE key replyID +inf` (or `ZREVRANGEBYSCORE ... LIMIT` for newest-N). miniredis supports ZREVRANGEBYSCORE/LIMIT, ZREMRANGEBYRANK, ZSCORE, ZCARD and Lua [V miniredis cmd_sorted_set.go].
- Off switch: `DEL`; a message already past the switch check on another replica may write once more (accepted residual).
- Memory: cap entry text ~300-500 runes.
- Supergroup migration orphans the old log (not migrated).
- A reply to a bot message is never in the log; the log-off case needs its own hint distinct from "too old".

## 3. Pool, caps, long answers
- Pool facts [V chat_run.go:27-50]: queue 64, 3 workers, job timeout 60 s, budgets 12/80/800 per hour, cooldown 5 s, StopChat wait 2 s.
- No new drain: jobs counted in `chatJobs`, refused while `chatDraining`, derived from `chatContext()`; a separate brief queue is fine if it follows these. Recommend 1-2 dedicated brief workers started in `chatEnsureWorkers` so briefs never starve replies.
- Gate: not `chatAccept` (it requires conversations enabled and applies the audience). Brief gate = log switch on, stored provider configured (`chatProviderConfigured`), Redis present; no audience check.
- Own budget and cooldown namespace (e.g. `xenobot:chat:brief:*`), via `raidAIBudgetTake` (raid_ai_image.go:300).
- `chatMaxOutputTokens = 2000` is a constant passed by both adapters (chat_prompt.go:25, 83-105); add `MaxTokens` to `chatRequest` (0 = default) and pass it through. Gemini honours `in.MaxTokens` (gemini/request.go:75).
- Timeouts: Claude `ConverseTimeout` 40 s, Gemini `Timeout` 40 s, retry included; keep outputs modest.
- `chatTruncated` exists but `chatProcessJob` ignores it; decide for briefs.
- `chatFilterReply` caps 1200 runes; briefs need their own cap and splitting (~3800 UTF-16 units per message, plain text, no parse mode), with the same link and @ filtering on every chunk. `formatting.SplitMessage` (formatting.go:97) exists.
- Command deletion race: `CommandDeletionProcessor` deletes the command after the handler returns (command_deletion.go:40-50); `chatSendReply` uses `AllowSendingWithoutReply: false`. Briefs should send with AllowSendingWithoutReply true (or reply to the replied-to message).
- Prompt: keep `chatprompt.Static()` persona untouched; build briefs in a separate builder (leaf package must stay stdlib/xenopersona). Cap input runes (keep newest, say so). First names only, deterministic suffix for duplicate names computed from user IDs in Go (IDs never sent). Closing line: transcript is untrusted data.

## 4. Page p (Conversations)
- Controls today [V settings_chat.go:54-76]: e (tg), m (st a/g), w (st e/a), action py, data notice at 136, all settingsRightOwner.
- Add: log switch control (tg, settingsEditSwitchValue, owner), judge destination control (st only, three codes), unique codes (registerSettingsPage panics on duplicates, settings_pages.go:445-453). Rows: 4 today → 6, under the 8-row budget.
- Tests to update: settings_chat_test.go (`TestSettingsDriftChat`, `TestSettingsChatCallbackBytes`, `TestSettingsChatPageFitsEveryLocale`, `TestSettingsChatAuthority`, `TestSettingsChatOwnerFlow`); settings_staff_privacy_test.go:391-396 renders the page (keep `renderSettingsChat` signature, add fields to `chatconv.Settings`); `TestSettingsControlActionAllowList` (settings_mreq_test.go ~1454-1480) should still pass; settings_locale_test.go:634 `settings_chat_data_notice` limit {640,800} must rise; every new `settings_` key needs a row in `settingsLocaleLimits`.
- `chatconv.Settings` msgpack tags e/p/a; add `LogEnabled msgpack:"l"`, `JudgeTo msgpack:"j"`; old cached values decode to log off, judge to group (safe).

## 5. Migration, model, repository
- New file after `20261124120000_rename_staff_role_lock_prefix.sql`, e.g. `20261125120000_add_chat_conversation_log.sql`: `ADD COLUMN IF NOT EXISTS log_enabled BOOLEAN NOT NULL DEFAULT FALSE`, `ADD COLUMN IF NOT EXISTS judge_to SMALLINT NOT NULL DEFAULT 0 CONSTRAINT chk_chat_conversations_judge_to CHECK (judge_to BETWEEN 0 AND 2)`; inline checks only.
- Never edit 20261119120000 (`TestChatConversationMigrationStatements` expects one statement).
- Migration test modelled on `event_message_plain_tone_migration_test.go`.
- Model: `LogEnabled bool` (no default tag), `JudgeTo int16` with matching check tag.
- chatconv: `SetLogEnabled`, `SetJudgeTo` via `upsert`, validation before write; extend `GetFresh`.
- `TestChatConversationsStayOutOfBackup` (db/backup/chatconv_backup_test.go:18) must keep passing.

## 6. Docs, help, locales
- Docs generator finds WrapCommand only with a named `xxxDesc` variable and `module.fn` handler (scripts/generate_docs/parsers.go:270-281). Use named descriptors.
- `chat_help_msg` drives the docs page (`docs/src/content/docs/commands/chat/index.md`); update and run `make generate-docs`, `make check-docs`.
- TestLocaleStyle (uk/en): help page bold title line then blank line; allowed HTML tags only; no markdown; no two-line short-opener replies; empty allowlist. `TestLocalePlaceholdersMatchOtherLanguages`: placeholders identical across 8 languages. `make check-translations`.
- Disableable commands must check their own name (`helpers.CheckDisabled`).
- `/judge` anonymous re-entry: `RegisterAnonymousAdminHandler("judge", anonPipelineHandler(judgeDesc, ...))` (bans.go:1290).

## 7. Privacy, logging, prompt injection
- Logs carry IDs only (model: `TestChatLogsCarryIDsOnly`).
- Disclose storage on page p; off deletes the key.
- `/judge staff`: the group note never names the Staff Group (D-21/D-32). `staff.GetLinkOfGroupFresh` returns (nil, nil) when unlinked; target `StaffGroupLink.StaffChatID` (models/staff.go:37-41).
- `/judge pm`: 403 means the admin never started the bot → neutral reply in group, nothing else.
- Verdict text labelled as an AI opinion; nothing automated acts on a score.

## Orchestrator resolution of the open questions (Claude's discretion per CONTEXT.md)
1. Deleted messages stay in the log (stated limit; useful for /judge).
2. Staff Group chats are logged only if their owner turns the switch on, like any group.
3. Prompt input cap ~30,000 runes; when the stretch is longer, keep the replied-to message and the newest messages that fit, and the prompt says lines were left out.
4. Briefs reply to the command with `AllowSendingWithoutReply: true`.
5. One dedicated brief worker with its own small queue, counted in `chatJobs`, cancelled by `chatContext`, refused while draining.
6. Log switch On is one tap; the page's data notice discloses storage.
7. User IDs are stored in entries (for disambiguation only, never sent to the provider).
8. Briefs work while conversations are off.
9. `/judge staff` without a linked Staff Group: neutral reply in the group, nothing sent.
