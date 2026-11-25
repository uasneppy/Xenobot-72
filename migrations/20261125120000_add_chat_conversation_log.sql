-- The message log and the /judge destination: two more columns on chat_conversations.
--
--   log_enabled  the owner's Message log switch (0 off, the default): while it is on the bot
--                keeps the newest 1000 messages of the group in Redis so that /unrollshort,
--                /unrolllong and /judge can brief a stretch of the conversation
--   judge_to     where a /judge verdict goes by default: 0 the group, 1 the admin's private
--                chat, 2 the linked Staff Group
--
-- Both are read through the existing chat_conv cache key and are never part of backup,
-- export, import or reset. A chat that has no row, and a row written before this file,
-- reads the log off and the verdict going to the group. judge_to carries its named CHECK
-- inline because PostgreSQL has no ADD CONSTRAINT IF NOT EXISTS: ADD COLUMN IF NOT EXISTS
-- skips the whole clause, the constraint included, when the column already exists, which
-- keeps each statement idempotent.
--
-- The Go runner and scripts/migrate_psql.sh both run this file unchanged; the runner
-- applies each file once inside a transaction and records its sha256 in schema_migrations.

ALTER TABLE chat_conversations ADD COLUMN IF NOT EXISTS log_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE chat_conversations ADD COLUMN IF NOT EXISTS judge_to SMALLINT NOT NULL DEFAULT 0 CONSTRAINT chk_chat_conversations_judge_to CHECK (judge_to BETWEEN 0 AND 2);
