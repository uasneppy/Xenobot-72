-- Per-group advanced threshold overrides (chat_advanced_thresholds).
--
-- Holds one row per group whose owner tuned any of the 12 Phase 28 thresholds across the 6 areas
-- (raid windows, new-member share & age, AI spam bars, member requirement cap & pause, lockdown
-- reminder interval, captcha extended timeout). A missing row or a 0 column value means unset /
-- use compiled defaults (or captcha_settings.timeout for captcha_timeout_mins).
--
-- Read through cache key chat_advanced_thresholds:<chat>, which skips the in-process layer so
-- changes act on the next event on every replica. Writes use column-targeted upserts and drop that
-- key. No foreign key on chats, and excluded from backup, export, import, and reset.
--
-- Every statement is idempotent with inline named CHECK constraints matching the Go bounds so both
-- migration appliers run the file unchanged in one transaction.

CREATE TABLE IF NOT EXISTS chat_advanced_thresholds (
    chat_id BIGINT NOT NULL,
    raid_join_window_secs INTEGER NOT NULL DEFAULT 0,
    raid_flood_window_secs INTEGER NOT NULL DEFAULT 0,
    raid_ai_window_secs INTEGER NOT NULL DEFAULT 0,
    raid_flood_new_percent INTEGER NOT NULL DEFAULT 0,
    raid_flood_min_accounts INTEGER NOT NULL DEFAULT 0,
    raid_recent_hours INTEGER NOT NULL DEFAULT 0,
    raid_ai_image_bar_pct INTEGER NOT NULL DEFAULT 0,
    member_req_ai_bar_pct INTEGER NOT NULL DEFAULT 0,
    member_req_cap_per_min INTEGER NOT NULL DEFAULT 0,
    member_req_pause_mins INTEGER NOT NULL DEFAULT 0,
    lockdown_reminder_mins INTEGER NOT NULL DEFAULT 0,
    captcha_timeout_mins INTEGER NOT NULL DEFAULT 0,
    updated_by BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT pk_chat_advanced_thresholds PRIMARY KEY (chat_id),
    CONSTRAINT chk_chat_adv_join_window CHECK (raid_join_window_secs = 0 OR raid_join_window_secs BETWEEN 15 AND 600),
    CONSTRAINT chk_chat_adv_flood_window CHECK (raid_flood_window_secs = 0 OR raid_flood_window_secs BETWEEN 10 AND 300),
    CONSTRAINT chk_chat_adv_ai_window CHECK (raid_ai_window_secs = 0 OR raid_ai_window_secs BETWEEN 30 AND 900),
    CONSTRAINT chk_chat_adv_flood_new_pct CHECK (raid_flood_new_percent = 0 OR raid_flood_new_percent BETWEEN 20 AND 100),
    CONSTRAINT chk_chat_adv_flood_min_accts CHECK (raid_flood_min_accounts = 0 OR raid_flood_min_accounts BETWEEN 2 AND 20),
    CONSTRAINT chk_chat_adv_recent_hours CHECK (raid_recent_hours = 0 OR raid_recent_hours BETWEEN 1 AND 168),
    CONSTRAINT chk_chat_adv_raid_ai_bar CHECK (raid_ai_image_bar_pct = 0 OR raid_ai_image_bar_pct BETWEEN 50 AND 99),
    CONSTRAINT chk_chat_adv_mreq_ai_bar CHECK (member_req_ai_bar_pct = 0 OR member_req_ai_bar_pct BETWEEN 50 AND 99),
    CONSTRAINT chk_chat_adv_mreq_cap CHECK (member_req_cap_per_min = 0 OR member_req_cap_per_min BETWEEN 3 AND 100),
    CONSTRAINT chk_chat_adv_mreq_pause CHECK (member_req_pause_mins = 0 OR member_req_pause_mins BETWEEN 1 AND 60),
    CONSTRAINT chk_chat_adv_lockdown_rem CHECK (lockdown_reminder_mins = 0 OR lockdown_reminder_mins BETWEEN 15 AND 1440),
    CONSTRAINT chk_chat_adv_captcha_timeout CHECK (captcha_timeout_mins = 0 OR captcha_timeout_mins BETWEEN 11 AND 60)
);
