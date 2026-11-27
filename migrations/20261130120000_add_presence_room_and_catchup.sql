-- Per-group Room-Aware Conversations & Frictionless Catch-Up settings on chat_presence
-- (CHAT-02, CHAT-03, CHAT-05).
--
-- Stored in admin-configurable chat_presence (strictly isolated from Owner-only
-- chat_conversations per PRES-04). Read through the skipLocal cache key chat_presence:<chat>;
-- every write drops that key. Idempotent ADD COLUMN IF NOT EXISTS statements keep rolling
-- upgrades safe without table locks or explicit transactions.

ALTER TABLE chat_presence
    ADD COLUMN IF NOT EXISTS ambient_context BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE chat_presence
    ADD COLUMN IF NOT EXISTS vocative_enabled BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE chat_presence
    ADD COLUMN IF NOT EXISTS pm_brief_react BOOLEAN NOT NULL DEFAULT FALSE;
