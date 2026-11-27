-- Per-rule static reaction options on reactions (COMM-01).
--
-- Pre-existing rows and legacy backups default to substring matching (word_boundary=false),
-- message text only (match_caption=false), 100% trigger chance (chance_pct=100), and zero
-- cooldown (cooldown_secs=0).
--
-- Named CHECK constraints are placed inline on ADD COLUMN IF NOT EXISTS so re-running the
-- migration on PostgreSQL skips each clause idempotently.

ALTER TABLE reactions
    ADD COLUMN IF NOT EXISTS word_boundary BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE reactions
    ADD COLUMN IF NOT EXISTS match_caption BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE reactions
    ADD COLUMN IF NOT EXISTS chance_pct SMALLINT NOT NULL DEFAULT 100
    CONSTRAINT chk_reactions_chance_pct CHECK (chance_pct BETWEEN 1 AND 100);

ALTER TABLE reactions
    ADD COLUMN IF NOT EXISTS cooldown_secs INTEGER NOT NULL DEFAULT 0
    CONSTRAINT chk_reactions_cooldown_secs CHECK (cooldown_secs BETWEEN 0 AND 86400);
