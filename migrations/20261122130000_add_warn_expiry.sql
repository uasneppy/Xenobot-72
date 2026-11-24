-- Warn expiry: a group can let old warns stop counting toward the warn limit.
--
--   warns_settings.warn_expiry_days  the number of days after which a warn no longer counts:
--                                    0 (the default) is off, so every warn counts as before;
--                                    1 to 365 turns expiry on. Named CHECK 0 to 365.
--   warns_users.warn_times           the unix seconds of each stored warn, in the order of the
--                                    reasons (jsonb, '[]' by default).
--
-- Old rows get no backfill: a row saved before this file has no times, and the Go code dates
-- every missing time at the row's last change (updated_at), which is never earlier than the
-- warn's real time, so no old warn expires before its real age. Every write after this file
-- stores one time per warn and, while expiry is on, drops the warns that no longer count.
--
-- The CHECK is part of the column definition because PostgreSQL has no ADD CONSTRAINT IF NOT
-- EXISTS: the IF NOT EXISTS column form skips the whole clause, the constraint included, when the
-- column already exists, which keeps each statement idempotent.
--
-- Both tables are still read through the cache, and every write still invalidates its key.
--
-- The Go runner and scripts/migrate_psql.sh both run this file unchanged; the runner applies
-- each file once inside a transaction and records its sha256 in schema_migrations.

ALTER TABLE warns_settings ADD COLUMN IF NOT EXISTS warn_expiry_days INTEGER NOT NULL DEFAULT 0 CONSTRAINT chk_warns_settings_expiry CHECK (warn_expiry_days BETWEEN 0 AND 365);
ALTER TABLE warns_users ADD COLUMN IF NOT EXISTS warn_times JSONB NOT NULL DEFAULT '[]';
