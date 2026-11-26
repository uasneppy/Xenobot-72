package models

import "time"

// ChatPresence is one chat's admin-configurable Interactive Presence settings, separate from
// Owner-only ChatConversation. A chat without a row means ConsensusEnabled=false,
// ConsensusThreshold=3, and RetractEnabled=true. It is read through the skipLocal cache key
// chat_presence:<chat> (see xenobot/db/presence).
// Both booleans carry NO GORM default: tag so a Go false is always written explicitly on
// Create and Save.
type ChatPresence struct {
	ChatID             int64     `gorm:"column:chat_id;primaryKey;autoIncrement:false" json:"chat_id"`
	ConsensusEnabled   bool      `gorm:"column:consensus_enabled;not null" json:"consensus_enabled"`
	ConsensusThreshold int16     `gorm:"column:consensus_threshold;not null;check:chk_chat_presence_consensus_threshold,consensus_threshold BETWEEN 2 AND 20" json:"consensus_threshold"`
	RetractEnabled     bool      `gorm:"column:retract_enabled;not null" json:"retract_enabled"`
	UpdatedBy          int64     `gorm:"column:updated_by;not null" json:"updated_by"`
	UpdatedAt          time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// TableName returns the migration's table name.
func (ChatPresence) TableName() string {
	return "chat_presence"
}
