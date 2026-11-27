-- Per-group Interactive Group Toys settings on chat_presence (TOYS-01, TOYS-03).
--
-- Stored in admin-configurable chat_presence (strictly isolated from Owner-only
-- chat_conversations per PRES-04). Read through the skipLocal cache key chat_presence:<chat>;
-- every write drops that key. Idempotent ADD COLUMN IF NOT EXISTS statements keep rolling
-- upgrades safe without table locks or explicit transactions.

ALTER TABLE chat_presence
    ADD COLUMN IF NOT EXISTS referendum_enabled BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE chat_presence
    ADD COLUMN IF NOT EXISTS inspect_enabled BOOLEAN NOT NULL DEFAULT FALSE;
