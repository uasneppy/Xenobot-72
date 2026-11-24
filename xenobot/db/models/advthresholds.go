package models

import "time"

// Default, minimum, and maximum bounds for every per-group advanced threshold in
// chat_advanced_thresholds. Storing 0 in any column means unset / use the compiled default
// (or captcha_settings.timeout for CaptchaTimeoutMins). Every bound mirrors the SQL CHECK
// constraints in migrations/20261123120000_add_advanced_thresholds.sql and is drift-checked
// against the compiled module constants by TestAdvancedThresholdDefaultsMatchConstants.
const (
	DefaultRaidJoinWindowSecs = 60
	MinRaidJoinWindowSecs     = 15
	MaxRaidJoinWindowSecs     = 600

	DefaultRaidFloodWindowSecs = 30
	MinRaidFloodWindowSecs     = 10
	MaxRaidFloodWindowSecs     = 300

	DefaultRaidAIWindowSecs = 120
	MinRaidAIWindowSecs     = 30
	MaxRaidAIWindowSecs     = 900

	DefaultRaidFloodNewPercent = 60
	MinRaidFloodNewPercent     = 20
	MaxRaidFloodNewPercent     = 100

	DefaultRaidFloodMinAccounts = 3
	MinRaidFloodMinAccounts     = 2
	MaxRaidFloodMinAccounts     = 20

	DefaultRaidRecentHours = 24
	MinRaidRecentHours     = 1
	MaxRaidRecentHours     = 168

	DefaultRaidAIImageBarPct = 85
	MinRaidAIImageBarPct     = 50
	MaxRaidAIImageBarPct     = 99

	DefaultMemberReqAIBarPct = 90
	MinMemberReqAIBarPct     = 50
	MaxMemberReqAIBarPct     = 99

	DefaultMemberReqCapPerMin = 10
	MinMemberReqCapPerMin     = 3
	MaxMemberReqCapPerMin     = 100

	DefaultMemberReqPauseMins = 10
	MinMemberReqPauseMins     = 1
	MaxMemberReqPauseMins     = 60

	DefaultLockdownReminderMins = 60
	MinLockdownReminderMins     = 15
	MaxLockdownReminderMins     = 1440

	DefaultCaptchaTimeoutMins = 0
	MinCaptchaTimeoutMins     = 11
	MaxCaptchaTimeoutMins     = 60
)

// ChatAdvancedThresholds stores one group's optional overrides for the 12 Phase 28 thresholds.
// A chat with no row or a column set to 0 uses the compiled default (or captcha_settings.timeout
// for CaptchaTimeoutMins). Check tags mirror the migration so SQLite tests enforce the same bounds.
type ChatAdvancedThresholds struct {
	ChatId               int64     `gorm:"column:chat_id;primaryKey;autoIncrement:false" json:"chat_id"`
	RaidJoinWindowSecs   int       `gorm:"column:raid_join_window_secs;not null;check:chk_chat_adv_join_window,raid_join_window_secs = 0 OR raid_join_window_secs BETWEEN 15 AND 600" json:"raid_join_window_secs"`
	RaidFloodWindowSecs  int       `gorm:"column:raid_flood_window_secs;not null;check:chk_chat_adv_flood_window,raid_flood_window_secs = 0 OR raid_flood_window_secs BETWEEN 10 AND 300" json:"raid_flood_window_secs"`
	RaidAIWindowSecs     int       `gorm:"column:raid_ai_window_secs;not null;check:chk_chat_adv_ai_window,raid_ai_window_secs = 0 OR raid_ai_window_secs BETWEEN 30 AND 900" json:"raid_ai_window_secs"`
	RaidFloodNewPercent  int       `gorm:"column:raid_flood_new_percent;not null;check:chk_chat_adv_flood_new_pct,raid_flood_new_percent = 0 OR raid_flood_new_percent BETWEEN 20 AND 100" json:"raid_flood_new_percent"`
	RaidFloodMinAccounts int       `gorm:"column:raid_flood_min_accounts;not null;check:chk_chat_adv_flood_min_accts,raid_flood_min_accounts = 0 OR raid_flood_min_accounts BETWEEN 2 AND 20" json:"raid_flood_min_accounts"`
	RaidRecentHours      int       `gorm:"column:raid_recent_hours;not null;check:chk_chat_adv_recent_hours,raid_recent_hours = 0 OR raid_recent_hours BETWEEN 1 AND 168" json:"raid_recent_hours"`
	RaidAIImageBarPct    int       `gorm:"column:raid_ai_image_bar_pct;not null;check:chk_chat_adv_raid_ai_bar,raid_ai_image_bar_pct = 0 OR raid_ai_image_bar_pct BETWEEN 50 AND 99" json:"raid_ai_image_bar_pct"`
	MemberReqAIBarPct    int       `gorm:"column:member_req_ai_bar_pct;not null;check:chk_chat_adv_mreq_ai_bar,member_req_ai_bar_pct = 0 OR member_req_ai_bar_pct BETWEEN 50 AND 99" json:"member_req_ai_bar_pct"`
	MemberReqCapPerMin   int       `gorm:"column:member_req_cap_per_min;not null;check:chk_chat_adv_mreq_cap,member_req_cap_per_min = 0 OR member_req_cap_per_min BETWEEN 3 AND 100" json:"member_req_cap_per_min"`
	MemberReqPauseMins   int       `gorm:"column:member_req_pause_mins;not null;check:chk_chat_adv_mreq_pause,member_req_pause_mins = 0 OR member_req_pause_mins BETWEEN 1 AND 60" json:"member_req_pause_mins"`
	LockdownReminderMins int       `gorm:"column:lockdown_reminder_mins;not null;check:chk_chat_adv_lockdown_rem,lockdown_reminder_mins = 0 OR lockdown_reminder_mins BETWEEN 15 AND 1440" json:"lockdown_reminder_mins"`
	CaptchaTimeoutMins   int       `gorm:"column:captcha_timeout_mins;not null;check:chk_chat_adv_captcha_timeout,captcha_timeout_mins = 0 OR captcha_timeout_mins BETWEEN 11 AND 60" json:"captcha_timeout_mins"`
	UpdatedBy            int64     `gorm:"column:updated_by;not null" json:"updated_by"`
	UpdatedAt            time.Time `gorm:"column:updated_at;not null" json:"updated_at,omitempty"`
}

// TableName returns the migration's table name.
func (ChatAdvancedThresholds) TableName() string {
	return "chat_advanced_thresholds"
}
