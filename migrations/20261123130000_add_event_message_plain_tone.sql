-- Add plain_tone to chat_event_message_options (Phase 29, TUNE-08).
--
-- A group with plain_tone = TRUE receives neutral, emoji-free, third-person default event
-- messages (group replies and private copies) whenever no custom text is set for that event.
-- Custom messages always override Plain tone, and groups with no options row or plain_tone = FALSE
-- keep the existing defaults byte for byte.
--
-- The statement is idempotent and carries no transaction control: both the Go runner and
-- scripts/migrate_psql.sh execute this file once inside one transaction.

ALTER TABLE chat_event_message_options
    ADD COLUMN IF NOT EXISTS plain_tone BOOLEAN NOT NULL DEFAULT FALSE;
