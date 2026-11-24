-- The escalation ladder (chat_escalation) and the offence counts behind it (chat_offences).
--
-- chat_escalation holds one chat's ladder, one row per chat whose owner changed it in the
-- private settings menu: whether it is on, up to five steps (an action of '', mute, tmute, kick,
-- tban or ban, and its seconds, 0 for an untimed action, at most 366 days) and the number of days
-- after which a member's count is forgotten (1 to 365, default 30). A chat without a row means
-- off, no steps, 30 days. It is read through the cache key chat_escalation:<chat>, which skips the
-- in-process layer, and every write drops that key.
--
-- chat_offences holds one member's offence count in one chat and the unix second of the last
-- counted offence. It is written only by the one conditional upsert of escalation.CountOffence,
-- so every replica counts one offence once, is never cached, and is never pruned by a worker: a
-- count is forgotten by comparison inside that upsert. Times are unix seconds, so no dialect
-- compares text times.
--
-- Neither table has a foreign key on the chat (the staff and settings precedent) or is part of
-- backup, export, import or reset. Both statements are idempotent and their constraints are
-- inline, because both migration appliers swallow duplicate_object on a bare constraint addition;
-- the file runs unchanged in one transaction under either applier.

CREATE TABLE IF NOT EXISTS chat_escalation (
    chat_id BIGINT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    step1_action VARCHAR(8) NOT NULL DEFAULT '',
    step1_secs INTEGER NOT NULL DEFAULT 0,
    step2_action VARCHAR(8) NOT NULL DEFAULT '',
    step2_secs INTEGER NOT NULL DEFAULT 0,
    step3_action VARCHAR(8) NOT NULL DEFAULT '',
    step3_secs INTEGER NOT NULL DEFAULT 0,
    step4_action VARCHAR(8) NOT NULL DEFAULT '',
    step4_secs INTEGER NOT NULL DEFAULT 0,
    step5_action VARCHAR(8) NOT NULL DEFAULT '',
    step5_secs INTEGER NOT NULL DEFAULT 0,
    forget_days INTEGER NOT NULL DEFAULT 30,
    updated_by BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT pk_chat_escalation PRIMARY KEY (chat_id),
    CONSTRAINT chk_chat_escalation_step1 CHECK (step1_action IN ('', 'mute', 'tmute', 'kick', 'tban', 'ban') AND step1_secs BETWEEN 0 AND 31622400),
    CONSTRAINT chk_chat_escalation_step2 CHECK (step2_action IN ('', 'mute', 'tmute', 'kick', 'tban', 'ban') AND step2_secs BETWEEN 0 AND 31622400),
    CONSTRAINT chk_chat_escalation_step3 CHECK (step3_action IN ('', 'mute', 'tmute', 'kick', 'tban', 'ban') AND step3_secs BETWEEN 0 AND 31622400),
    CONSTRAINT chk_chat_escalation_step4 CHECK (step4_action IN ('', 'mute', 'tmute', 'kick', 'tban', 'ban') AND step4_secs BETWEEN 0 AND 31622400),
    CONSTRAINT chk_chat_escalation_step5 CHECK (step5_action IN ('', 'mute', 'tmute', 'kick', 'tban', 'ban') AND step5_secs BETWEEN 0 AND 31622400),
    CONSTRAINT chk_chat_escalation_forget CHECK (forget_days BETWEEN 1 AND 365)
);

CREATE TABLE IF NOT EXISTS chat_offences (
    chat_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    offences INTEGER NOT NULL DEFAULT 0,
    last_offence_unix BIGINT NOT NULL DEFAULT 0,
    CONSTRAINT pk_chat_offences PRIMARY KEY (chat_id, user_id),
    CONSTRAINT chk_chat_offences_count CHECK (offences >= 0)
);
