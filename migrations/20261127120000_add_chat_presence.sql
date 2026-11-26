-- Per-group Interactive Presence settings (chat_presence).
--
-- One row per chat whose admin changed an Interactive Presence setting in the private settings
-- menu (separate from Owner-only chat_conversations). A chat without a row means Emoji Consensus
-- off with threshold 3 and thumbs-down retraction on. It is read through the cache key
-- chat_presence:<chat>, which skips the in-process layer, and every write drops that key. It has
-- no foreign key on the chat (the staff, settings and chat_conversations precedent).
--
-- The statement is idempotent and its constraint is inline, because both migration appliers
-- swallow duplicate_object on a bare constraint addition.

CREATE TABLE IF NOT EXISTS chat_presence (
    chat_id BIGINT PRIMARY KEY,
    consensus_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    consensus_threshold SMALLINT NOT NULL DEFAULT 3,
    retract_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    updated_by BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT chk_chat_presence_consensus_threshold CHECK (consensus_threshold BETWEEN 2 AND 20)
);
