-- Allow Featherless AI as a conversation provider (chat_conversations.provider).
--
-- A group's owner can now pick Featherless AI next to Anthropic and Google, so the
-- column accepts 'featherless' as well. The column is VARCHAR(16) and the new value has
-- 11 characters, so only the CHECK changes.
--
-- Both migration appliers swallow duplicate_object around a top-level ADD CONSTRAINT, so
-- an ADD that ran while the old check still existed would be a silent no-op and
-- 'featherless' would stay refused in production. The order is therefore:
--
--   1. drop chk_chat_conversations_provider if it exists
--   2. drop every other CHECK on the table whose definition mentions provider
--   3. add chk_chat_conversations_provider again, with featherless
--
-- The re-added check is NOT VALID: it skips a scan of the existing rows (all of them are
-- anthropic or google, so they would pass) and every new or updated row is checked.
--
-- Every statement is idempotent; the Go runner applies each file once inside a
-- transaction and records its sha256 in schema_migrations.

ALTER TABLE chat_conversations DROP CONSTRAINT IF EXISTS chk_chat_conversations_provider;

DO $$
DECLARE
    legacy RECORD;
BEGIN
    FOR legacy IN
        SELECT conname
        FROM pg_constraint
        WHERE conrelid = 'chat_conversations'::regclass
          AND contype = 'c'
          AND conname <> 'chk_chat_conversations_provider'
          AND pg_get_constraintdef(oid) LIKE '%provider%'
    LOOP
        EXECUTE format('ALTER TABLE chat_conversations DROP CONSTRAINT %I', legacy.conname);
    END LOOP;
END $$;

ALTER TABLE chat_conversations
ADD CONSTRAINT chk_chat_conversations_provider
CHECK (provider IN ('anthropic', 'google', 'featherless')) NOT VALID;
