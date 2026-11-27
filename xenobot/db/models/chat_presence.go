package models

import "time"

// ChatPresence is one chat's admin-configurable Interactive Presence settings, separate from
// Owner-only ChatConversation. A chat without a row means ConsensusEnabled=false,
// ConsensusThreshold=3, RetractEnabled=true, AIReactMode="off", CatStickerEnabled=false,
// ArchiveEnabled=false, ArchiveChatID=0, ArchiveThreadID=0, ArchiveThreshold=5,
// ArchiveEmojiMode="positive", and ArchiveDelivery="copy". It is read through the
// skipLocal cache key chat_presence:<chat> (see xenobot/db/presence).
// All booleans carry NO GORM default: tag so a Go false is always written explicitly on
// Create and Save.
type ChatPresence struct {
	ChatID             int64     `gorm:"column:chat_id;primaryKey;autoIncrement:false" json:"chat_id"`
	ConsensusEnabled   bool      `gorm:"column:consensus_enabled;not null" json:"consensus_enabled"`
	ConsensusThreshold int16     `gorm:"column:consensus_threshold;not null;check:chk_chat_presence_consensus_threshold,consensus_threshold BETWEEN 2 AND 20" json:"consensus_threshold"`
	RetractEnabled     bool      `gorm:"column:retract_enabled;not null" json:"retract_enabled"`
	AIReactMode        string    `gorm:"column:ai_react_mode;not null;size:16;check:chk_chat_presence_ai_react_mode,ai_react_mode IN ('off', 'low', 'medium')" json:"ai_react_mode"`
	CatStickerEnabled  bool      `gorm:"column:cat_sticker_enabled;not null" json:"cat_sticker_enabled"`
	ArchiveEnabled     bool      `gorm:"column:archive_enabled;not null" json:"archive_enabled"`
	ArchiveChatID      int64     `gorm:"column:archive_chat_id;not null" json:"archive_chat_id"`
	ArchiveThreadID    int64     `gorm:"column:archive_thread_id;not null" json:"archive_thread_id"`
	ArchiveThreshold   int16     `gorm:"column:archive_threshold;not null;default:5;check:chk_chat_presence_archive_threshold,archive_threshold BETWEEN 2 AND 50" json:"archive_threshold"`
	ArchiveEmojiMode   string    `gorm:"column:archive_emoji_mode;not null;size:16;default:'positive';check:chk_chat_presence_archive_emoji_mode,archive_emoji_mode IN ('positive', 'all')" json:"archive_emoji_mode"`
	ArchiveDelivery    string    `gorm:"column:archive_delivery;not null;size:16;default:'copy';check:chk_chat_presence_archive_delivery,archive_delivery IN ('copy', 'forward')" json:"archive_delivery"`
	UpdatedBy          int64     `gorm:"column:updated_by;not null" json:"updated_by"`
	UpdatedAt          time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// TableName returns the migration's table name.
func (ChatPresence) TableName() string {
	return "chat_presence"
}
