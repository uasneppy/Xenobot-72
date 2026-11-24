-- Per-group manual command tuning: defaults & appeal link (chat_manual_defaults) and reason
-- shortcuts (chat_reason_shortcuts).
--
-- chat_manual_defaults holds one row per group whose owner configured default ban/mute durations,
-- the reason-required switch, or an appeal button URL. A group without a row means no default
-- durations (0 = permanent, today's behavior), reason not required, and no appeal link. It is
-- read through the cache key chat_manual_defaults:<chat>, which skips the in-process layer, and
-- every write uses a column-targeted upsert and drops that key.
--
-- chat_reason_shortcuts holds a group's short words and the full reason texts they expand to in
-- manual moderation commands. It is read through the cache key chat_reason_shortcuts:<chat>,
-- which also skips the in-process layer, and every add/remove drops that key.
--
-- Neither table has a foreign key on chats or is part of backup, export, import or reset. Every
-- statement is idempotent with inline named constraints so both migration appliers run the file
-- unchanged in one transaction.

CREATE TABLE IF NOT EXISTS chat_manual_defaults (
    chat_id BIGINT NOT NULL,
    default_ban_secs INTEGER NOT NULL DEFAULT 0,
    default_mute_secs INTEGER NOT NULL DEFAULT 0,
    reason_required BOOLEAN NOT NULL DEFAULT FALSE,
    appeal_url TEXT NOT NULL DEFAULT '',
    updated_by BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT pk_chat_manual_defaults PRIMARY KEY (chat_id),
    CONSTRAINT chk_chat_manual_defaults_ban CHECK (default_ban_secs = 0 OR default_ban_secs BETWEEN 60 AND 31622400),
    CONSTRAINT chk_chat_manual_defaults_mute CHECK (default_mute_secs = 0 OR default_mute_secs BETWEEN 60 AND 31622400),
    CONSTRAINT chk_chat_manual_defaults_appeal CHECK (length(appeal_url) <= 2048)
);

CREATE TABLE IF NOT EXISTS chat_reason_shortcuts (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    word VARCHAR(32) NOT NULL,
    reason TEXT NOT NULL,
    created_by BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT uk_chat_reason_shortcuts_word UNIQUE (chat_id, word),
    CONSTRAINT chk_chat_reason_shortcuts_word CHECK (word <> ''),
    CONSTRAINT chk_chat_reason_shortcuts_reason CHECK (reason <> '' AND length(reason) <= 500)
);

CREATE INDEX IF NOT EXISTS idx_chat_reason_shortcuts_chat ON chat_reason_shortcuts (chat_id);
