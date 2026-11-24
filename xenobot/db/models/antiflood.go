package models

import "time"

// AntifloodSettings is a chat's flood limit and action. ActionSecs is the duration of a timed action
// (tmute, tban) in seconds: 0 for a plain action, and a timed action stored with 0 is inert. It has no
// default tag, so a Go 0 is always written, and omitempty keeps an export with no duration unchanged.
type AntifloodSettings struct {
	ID                     uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId                 int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id,omitempty"`
	Limit                  int       `gorm:"column:flood_limit;default:5;check:chk_antiflood_limit,flood_limit >= 0" json:"limit,omitempty"`
	Action                 string    `gorm:"column:action;default:'mute';check:chk_antiflood_action,action IN ('mute','ban','kick','warn','tban','tmute')" json:"action,omitempty"`
	ActionSecs             int       `gorm:"column:action_secs;not null;check:chk_antiflood_action_secs,action_secs BETWEEN 0 AND 31622400" json:"action_secs,omitempty"`
	DeleteAntifloodMessage bool      `gorm:"column:delete_antiflood_message;default:false" json:"delete_antiflood_message,omitempty"`
	CreatedAt              time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt              time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (AntifloodSettings) TableName() string {
	return "antiflood_settings"
}
