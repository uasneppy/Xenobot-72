package models

import "time"

// ChatConversation is one chat's conversation setting: whether the bot answers members who
// address it, which provider it asks, and who may talk to it. A chat without a row means off,
// anthropic, everyone, the message log off and a /judge verdict going to the group. LogEnabled is
// the owner's Message log switch and JudgeTo the default destination of a /judge verdict (0 the
// group, 1 the admin's private chat, 2 the linked Staff Group). It is read through the cache key chat_conv:<chat> (see xenobot/db/chatconv).
// The boolean carries no default tag, so a Go false is always written.
type ChatConversation struct {
	ChatID     int64     `gorm:"column:chat_id;primaryKey;autoIncrement:false" json:"chat_id"`
	Enabled    bool      `gorm:"column:enabled;not null" json:"enabled"`
	Provider   string    `gorm:"column:provider;not null;size:16;check:chk_chat_conversations_provider,provider IN ('anthropic', 'google')" json:"provider"`
	Audience   int16     `gorm:"column:audience;not null;check:chk_chat_conversations_audience,audience BETWEEN 0 AND 1" json:"audience"`
	LogEnabled bool      `gorm:"column:log_enabled;not null" json:"log_enabled"`
	JudgeTo    int16     `gorm:"column:judge_to;not null;check:chk_chat_conversations_judge_to,judge_to BETWEEN 0 AND 2" json:"judge_to"`
	UpdatedBy  int64     `gorm:"column:updated_by;not null" json:"updated_by"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// TableName returns the migration's table name.
func (ChatConversation) TableName() string {
	return "chat_conversations"
}
