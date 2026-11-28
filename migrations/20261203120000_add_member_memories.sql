-- Per-member conversation memories (Phase 36).
--
-- chat_member_memories holds, for one member of one group, the short notes the conversation feature
-- keeps so it can be a little more personal with that member in that group only: one row per
-- (chat_id, user_id), the notes joined by new lines (at most 12 notes of at most 200 characters each,
-- so the CHECK below is the table's own ceiling). It has no foreign key, is read fresh (never cached, so
-- no cache key needs dropping and a delete is seen by every replica at once), and is never part of
-- backup, export, import or reset.
--
-- chat_conversations.memories_enabled is the group owner's switch for it (off by default, no row means
-- off). Turning it off, or turning conversations off, deletes the group's rows in application code.

ALTER TABLE chat_conversations
    ADD COLUMN IF NOT EXISTS memories_enabled BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS chat_member_memories (
    chat_id    BIGINT      NOT NULL,
    user_id    BIGINT      NOT NULL,
    notes      TEXT        NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT pk_chat_member_memories PRIMARY KEY (chat_id, user_id),
    CONSTRAINT chk_chat_member_memories_notes CHECK (char_length(notes) <= 2500)
);
