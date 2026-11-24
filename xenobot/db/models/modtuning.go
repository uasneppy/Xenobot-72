package models

import "time"

const (
	// ManualAppealURLMaxRunes bounds appeal_url in chat_manual_defaults (matching the SQL CHECK).
	ManualAppealURLMaxRunes = 2048
	// ReasonShortcutWordMaxRunes bounds word in chat_reason_shortcuts (matching VARCHAR(32)).
	ReasonShortcutWordMaxRunes = 32
	// ReasonShortcutTextMaxRunes bounds reason in chat_reason_shortcuts (matching the SQL CHECK).
	ReasonShortcutTextMaxRunes = 500
	// ReasonShortcutsCapMax is the maximum number of reason shortcuts a chat may store.
	ReasonShortcutsCapMax = 50
)

// ChatManualDefaults stores one group's manual moderation defaults and appeal link. A chat with no
// row uses zero defaults (permanent /ban and /mute, reason not required, no appeal link). The
// boolean carries no default tag so Go false is always written; check tags mirror the migration on
// SQLite too.
type ChatManualDefaults struct {
	ChatId          int64     `gorm:"column:chat_id;primaryKey;autoIncrement:false" json:"chat_id"`
	DefaultBanSecs  int       `gorm:"column:default_ban_secs;not null;check:chk_chat_manual_defaults_ban,default_ban_secs = 0 OR default_ban_secs BETWEEN 60 AND 31622400" json:"default_ban_secs"`
	DefaultMuteSecs int       `gorm:"column:default_mute_secs;not null;check:chk_chat_manual_defaults_mute,default_mute_secs = 0 OR default_mute_secs BETWEEN 60 AND 31622400" json:"default_mute_secs"`
	ReasonRequired  bool      `gorm:"column:reason_required;not null" json:"reason_required"`
	AppealURL       string    `gorm:"column:appeal_url;type:text;not null;check:chk_chat_manual_defaults_appeal,length(appeal_url) <= 2048" json:"appeal_url"`
	UpdatedBy       int64     `gorm:"column:updated_by;not null" json:"updated_by"`
	UpdatedAt       time.Time `gorm:"column:updated_at;not null" json:"updated_at,omitempty"`
}

// TableName returns the migration's table name.
func (ChatManualDefaults) TableName() string {
	return "chat_manual_defaults"
}

// ChatReasonShortcut stores one short word and its expanded plain-text reason for manual
// moderation commands in a chat.
type ChatReasonShortcut struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ChatId    int64     `gorm:"column:chat_id;not null;uniqueIndex:uk_chat_reason_shortcuts_word,priority:1;index:idx_chat_reason_shortcuts_chat" json:"chat_id"`
	Word      string    `gorm:"column:word;size:32;not null;uniqueIndex:uk_chat_reason_shortcuts_word,priority:2;check:chk_chat_reason_shortcuts_word,word <> ''" json:"word"`
	Reason    string    `gorm:"column:reason;type:text;not null;check:chk_chat_reason_shortcuts_reason,reason <> '' AND length(reason) <= 500" json:"reason"`
	CreatedBy int64     `gorm:"column:created_by;not null" json:"created_by"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"created_at,omitempty"`
}

// TableName returns the migration's table name.
func (ChatReasonShortcut) TableName() string {
	return "chat_reason_shortcuts"
}
