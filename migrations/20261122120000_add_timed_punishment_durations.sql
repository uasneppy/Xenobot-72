-- Timed punishments: the duration of a timed mute or ban for the three automatic rules.
--
-- Each setting that can name a timed action (tmute, tban) gets one column holding its
-- duration in whole seconds:
--
--   warns_settings.warn_mode_secs  the duration of the warn limit's action
--   antiflood_settings.action_secs the duration of the flood action
--   blacklists.action_secs         the duration of the blacklist action, one per word like
--                                  the action itself
--
-- Each column is INTEGER NOT NULL DEFAULT 0 under its own named CHECK of 0 to 31622400 (366
-- days, Telegram's longest finite restriction). A row written before this file, and a timed
-- mode stored with no duration, therefore read 0, which is inert: a stored tmute or tban with
-- no duration applies nothing, exactly as before, so no group's setting starts acting by
-- itself. A plain action (mute, kick, ban) always stores 0.
--
-- The CHECK is part of the column definition because PostgreSQL has no ADD CONSTRAINT IF NOT
-- EXISTS, which is what keeps each statement idempotent: ADD COLUMN IF NOT EXISTS skips the
-- whole clause, the constraint included, when the column already exists.
--
-- The tables are still read through the cache, and every write still invalidates its key.
--
-- The Go runner and scripts/migrate_psql.sh both run this file unchanged; the runner
-- applies each file once inside a transaction and records its sha256 in schema_migrations.

ALTER TABLE warns_settings ADD COLUMN IF NOT EXISTS warn_mode_secs INTEGER NOT NULL DEFAULT 0 CONSTRAINT chk_warns_settings_mode_secs CHECK (warn_mode_secs BETWEEN 0 AND 31622400);
ALTER TABLE antiflood_settings ADD COLUMN IF NOT EXISTS action_secs INTEGER NOT NULL DEFAULT 0 CONSTRAINT chk_antiflood_action_secs CHECK (action_secs BETWEEN 0 AND 31622400);
ALTER TABLE blacklists ADD COLUMN IF NOT EXISTS action_secs INTEGER NOT NULL DEFAULT 0 CONSTRAINT chk_blacklists_action_secs CHECK (action_secs BETWEEN 0 AND 31622400);
