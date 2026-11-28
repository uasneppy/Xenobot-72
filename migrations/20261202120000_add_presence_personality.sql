-- Per-group reaction personality on chat_presence: how often the bot reacts to each kind of message,
-- the topics an AI reaction is limited to, and the extra names it answers to.
--
-- Stored in admin-configurable chat_presence (strictly isolated from Owner-only chat_conversations).
-- Read through the skipLocal cache key chat_presence:<chat>; every write drops that key. Zero means
-- today's behaviour everywhere (frequency 0 is "normal", empty text is "no topics" and "no extra
-- names"), so a row written before this migration, and a value an older build cached, read exactly as
-- they did. Named CHECK constraints are inline on ADD COLUMN IF NOT EXISTS so re-running the migration
-- on PostgreSQL skips each clause idempotently.

ALTER TABLE chat_presence
    ADD COLUMN IF NOT EXISTS react_text_freq SMALLINT NOT NULL DEFAULT 0
    CONSTRAINT chk_chat_presence_react_text_freq CHECK (react_text_freq BETWEEN 0 AND 3);

ALTER TABLE chat_presence
    ADD COLUMN IF NOT EXISTS react_sticker_freq SMALLINT NOT NULL DEFAULT 0
    CONSTRAINT chk_chat_presence_react_sticker_freq CHECK (react_sticker_freq BETWEEN 0 AND 3);

ALTER TABLE chat_presence
    ADD COLUMN IF NOT EXISTS react_photo_freq SMALLINT NOT NULL DEFAULT 0
    CONSTRAINT chk_chat_presence_react_photo_freq CHECK (react_photo_freq BETWEEN 0 AND 3);

ALTER TABLE chat_presence
    ADD COLUMN IF NOT EXISTS react_topics TEXT NOT NULL DEFAULT ''
    CONSTRAINT chk_chat_presence_react_topics CHECK (char_length(react_topics) <= 400);

ALTER TABLE chat_presence
    ADD COLUMN IF NOT EXISTS bot_names TEXT NOT NULL DEFAULT ''
    CONSTRAINT chk_chat_presence_bot_names CHECK (char_length(bot_names) <= 400);
