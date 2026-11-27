-- Per-group Cat-Sticker Micro-Triggers and Highlights Archive settings on chat_presence,
-- plus durable once-only deduplication in chat_presence_archived (COMM-02, COMM-03, COMM-04).
--
-- Stored in admin-configurable chat_presence (strictly isolated from Owner-only
-- chat_conversations per PRES-04). Read through the skipLocal cache key chat_presence:<chat>;
-- every write drops that key. Named CHECK constraints are inline on ADD COLUMN IF NOT EXISTS so
-- re-running the migration on PostgreSQL skips each clause idempotently.

ALTER TABLE chat_presence
    ADD COLUMN IF NOT EXISTS cat_sticker_enabled BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE chat_presence
    ADD COLUMN IF NOT EXISTS archive_enabled BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE chat_presence
    ADD COLUMN IF NOT EXISTS archive_chat_id BIGINT NOT NULL DEFAULT 0;

ALTER TABLE chat_presence
    ADD COLUMN IF NOT EXISTS archive_thread_id BIGINT NOT NULL DEFAULT 0;

ALTER TABLE chat_presence
    ADD COLUMN IF NOT EXISTS archive_threshold SMALLINT NOT NULL DEFAULT 5
    CONSTRAINT chk_chat_presence_archive_threshold CHECK (archive_threshold BETWEEN 2 AND 50);

ALTER TABLE chat_presence
    ADD COLUMN IF NOT EXISTS archive_emoji_mode VARCHAR(16) NOT NULL DEFAULT 'positive'
    CONSTRAINT chk_chat_presence_archive_emoji_mode CHECK (archive_emoji_mode IN ('positive', 'all'));

ALTER TABLE chat_presence
    ADD COLUMN IF NOT EXISTS archive_delivery VARCHAR(16) NOT NULL DEFAULT 'copy'
    CONSTRAINT chk_chat_presence_archive_delivery CHECK (archive_delivery IN ('copy', 'forward'));

CREATE TABLE IF NOT EXISTS chat_presence_archived (
    chat_id BIGINT NOT NULL,
    message_id BIGINT NOT NULL,
    archived_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT pk_chat_presence_archived PRIMARY KEY (chat_id, message_id)
);
