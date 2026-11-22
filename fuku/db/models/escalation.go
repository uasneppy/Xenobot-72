package models

import "time"

// ChatEscalation is one chat's escalation ladder: whether it is on, up to five steps (an action
// and, for a timed one, its seconds) and the days after which a member's offence count is
// forgotten. A chat without a row means off, no steps, 30 days. It is read through the cache key
// chat_escalation:<chat> (see fuku/db/escalation). The boolean carries no default tag, so a Go
// false is always written; the check tags build the migration's checks on SQLite too.
type ChatEscalation struct {
	ChatId      int64     `gorm:"column:chat_id;primaryKey;autoIncrement:false" json:"chat_id"`
	Enabled     bool      `gorm:"column:enabled;not null" json:"enabled"`
	Step1Action string    `gorm:"column:step1_action;not null;size:8;check:chk_chat_escalation_step1,step1_action IN ('', 'mute', 'tmute', 'kick', 'tban', 'ban') AND step1_secs BETWEEN 0 AND 31622400" json:"step1_action"`
	Step1Secs   int       `gorm:"column:step1_secs;not null" json:"step1_secs"`
	Step2Action string    `gorm:"column:step2_action;not null;size:8;check:chk_chat_escalation_step2,step2_action IN ('', 'mute', 'tmute', 'kick', 'tban', 'ban') AND step2_secs BETWEEN 0 AND 31622400" json:"step2_action"`
	Step2Secs   int       `gorm:"column:step2_secs;not null" json:"step2_secs"`
	Step3Action string    `gorm:"column:step3_action;not null;size:8;check:chk_chat_escalation_step3,step3_action IN ('', 'mute', 'tmute', 'kick', 'tban', 'ban') AND step3_secs BETWEEN 0 AND 31622400" json:"step3_action"`
	Step3Secs   int       `gorm:"column:step3_secs;not null" json:"step3_secs"`
	Step4Action string    `gorm:"column:step4_action;not null;size:8;check:chk_chat_escalation_step4,step4_action IN ('', 'mute', 'tmute', 'kick', 'tban', 'ban') AND step4_secs BETWEEN 0 AND 31622400" json:"step4_action"`
	Step4Secs   int       `gorm:"column:step4_secs;not null" json:"step4_secs"`
	Step5Action string    `gorm:"column:step5_action;not null;size:8;check:chk_chat_escalation_step5,step5_action IN ('', 'mute', 'tmute', 'kick', 'tban', 'ban') AND step5_secs BETWEEN 0 AND 31622400" json:"step5_action"`
	Step5Secs   int       `gorm:"column:step5_secs;not null" json:"step5_secs"`
	ForgetDays  int       `gorm:"column:forget_days;not null;check:chk_chat_escalation_forget,forget_days BETWEEN 1 AND 365" json:"forget_days"`
	UpdatedBy   int64     `gorm:"column:updated_by;not null" json:"updated_by"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// TableName returns the migration's table name.
func (ChatEscalation) TableName() string {
	return "chat_escalation"
}

// ChatOffence is one member's offence count in one chat and the unix second of the last counted
// offence. It is written only by escalation.CountOffence's conditional upsert, never cached, and
// never part of backup, export, import or reset.
type ChatOffence struct {
	ChatId          int64 `gorm:"column:chat_id;primaryKey;autoIncrement:false" json:"chat_id"`
	UserId          int64 `gorm:"column:user_id;primaryKey;autoIncrement:false" json:"user_id"`
	Offences        int   `gorm:"column:offences;not null;check:chk_chat_offences_count,offences >= 0" json:"offences"`
	LastOffenceUnix int64 `gorm:"column:last_offence_unix;not null" json:"last_offence_unix"`
}

// TableName returns the migration's table name.
func (ChatOffence) TableName() string {
	return "chat_offences"
}
