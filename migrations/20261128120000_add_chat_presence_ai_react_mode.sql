-- Per-group Contextual AI Emoji Reactions mode (ai_react_mode) on chat_presence.
--
-- Stored in admin-configurable chat_presence (strictly isolated from Owner-only
-- chat_conversations per PRES-04). A chat without a row, or a row written before this
-- migration, defaults to 'off'. Allowed values are 'off', 'low', and 'medium'.
--
-- Read through the skipLocal cache key chat_presence:<chat>; every write drops that key.
-- The named CHECK constraint is placed inline on ADD COLUMN IF NOT EXISTS so re-running
-- the migration on PostgreSQL skips the whole clause idempotently.

ALTER TABLE chat_presence
    ADD COLUMN IF NOT EXISTS ai_react_mode VARCHAR(16) NOT NULL DEFAULT 'off'
    CONSTRAINT chk_chat_presence_ai_react_mode CHECK (ai_react_mode IN ('off', 'low', 'medium'));
