# Quick Task 261125-0iq: Add /unrollshort, /unrolllong and /judge AI chat briefs from a per-group rolling message log - Context

**Gathered:** 2026-11-25
**Status:** Ready for planning

<domain>
## Task Boundary

Three new group commands that summarise a stretch of a group's conversation with the group's AI provider:
- `/unrollshort` (reply to a message): a short brief of who talked and about what, from the replied-to message to now.
- `/unrolllong` (reply to a message): the same, more elaborate.
- `/judge` (reply to a message, admins only): a detailed brief plus a guilt score from 1 to 10 for every person who took part, each with a reason.

They read from a new per-group rolling message log that the bot keeps itself, because the Telegram Bot API has no method that returns chat history: a bot only ever sees a message once, as an update.

</domain>

<decisions>
## Implementation Decisions

### The log switch (owner decision)
- A per-group, owner-only "Message log" switch on the existing private Conversations settings page (page `p`, `settings_chat.go`), off by default. It is a new column on `chat_conversations` (new append-only migration, `log_enabled BOOLEAN NOT NULL DEFAULT FALSE`), read through the existing `chatconv` repository and cache key `chat_conv:<chat>` (already in `skipLocal`), written by a new column-targeted upsert and an apply function beside `applyChatEnabled` in `chat_settings.go`.
- Turning the log off deletes the group's stored log at once.
- The page's data notice must say that the log stores members' messages and sends a stretch of them to the provider when someone runs a command.
- The log switch is independent of the conversations on/off switch; both share the stored provider.

### Storage (owner decision)
- The bot continuously stores the newest 1000 messages per group in Redis; when it reaches 1000 the oldest is dropped one by one (a ring). Each entry is keyed by its Telegram message ID.
- Replying to a message sends the AI only that message and every logged message after it (message ID >= the replied-to ID).
- Redis only (like the other conversation keys, outside the `xenobot:cache:` prefix, chat ID as a hash tag). Without Redis the log and the commands are inert on that replica.

### Log content (owner decision)
- Text and captions, plus a short media tag such as `[photo]`, `[sticker]`, `[voice]` so the AI knows something was sent. Edits update the stored line. Deletions are not tracked.
- Only real member messages of a group or supergroup are logged (no bots, no service messages); a bounded length per entry.

### Out-of-log reply target (owner decision)
- If the replied-to message is not in the log (older than the newest 1000, or sent before the log was on), the command refuses with a hint ("that message is older than what I remember") and makes no AI call.

### Who may run the commands (owner decision)
- `/unrollshort` and `/unrolllong`: everyone, with a per-user cooldown and hourly budgets (per user, per group, fleet) so they cannot run up AI cost.
- `/judge`: administrators only, with anonymous-admin re-entry through `RegisterAnonymousAdminHandler` + `anonPipelineHandler`.

### /judge destination (owner decision)
- The admin chooses where the verdict goes: the group, the admin's own private chat, or the admin group (this bot's linked Staff Group, `staff.GetLinkOfGroupFresh`).
- Per-invocation argument: `/judge` uses the group's default, `/judge group`, `/judge pm`, `/judge staff` override it.
- Plus a per-group default on the Conversations page ("Judge verdict goes to": group / private / staff group), owner-only like the rest of the page, stored as a new column on `chat_conversations` (same migration).
- When sent to PM or the Staff Group, the group gets a short note; a PM the admin has not started, or no linked Staff Group, gets a clear reply in the group and nothing else is sent.

### Provider and runtime (owner decision)
- Reuse the group's stored conversation provider (Anthropic or Gemini) and the existing conversation runtime: `chatProvider` adapters, `chatBreakers`, the chat pacer, the chat pool and `StopChat` drain (no new shutdown drain).

### Claude's Discretion
- Exact cooldown, budgets, output token caps and prompt wording; splitting long answers over several Telegram messages; how members are labelled in the prompt (no user IDs or usernames to the provider, as the conversation feature already does); whether the commands are Disableable; locale wording in all 8 languages; settings page layout within the row budget.
- The log must judge no one by itself and store nothing for a group whose switch is off.

</decisions>

<specifics>
## Specific Ideas

- Project rules are in `AGENTS.md` (very long and strict): handler group numbering, WrapCommand, anonymous admin re-entry, callback codec, settings page framework and its tests, locale keys in 8 languages with limits tables, migration rules, `make test` / `make lint` / `make check-translations` / `make check-docs`, and the rule to update AGENTS.md in the same commit as any change it describes.
- Treat the logged transcript as untrusted data in the prompt (prompt injection).
- The /judge output is an AI opinion; the reply should say so.

</specifics>

<canonical_refs>
## Canonical References

- `AGENTS.md` (project rules)
- `xenobot/modules/chat*.go`, `xenobot/modules/settings_chat.go`, `xenobot/db/chatconv/`, `migrations/20261119120000_add_chat_conversations.sql`

</canonical_refs>
