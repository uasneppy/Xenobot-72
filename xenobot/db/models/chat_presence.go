package models

import "time"

// ChatPresence is one chat's admin-configurable Interactive Presence settings, separate from
// Owner-only ChatConversation. A chat without a row means ConsensusEnabled=false,
// ConsensusThreshold=3, RetractEnabled=true, AIReactMode="off", CatStickerEnabled=false,
// ArchiveEnabled=false, ArchiveChatID=0, ArchiveThreadID=0, ArchiveThreshold=5,
// ArchiveEmojiMode="positive", ArchiveDelivery="copy", AmbientContext=false,
// VocativeEnabled=false, PMBriefReact=false, ReferendumEnabled=false, and InspectEnabled=false.
// It is read through the
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
	AmbientContext     bool      `gorm:"column:ambient_context;not null" json:"ambient_context"`
	VocativeEnabled    bool      `gorm:"column:vocative_enabled;not null" json:"vocative_enabled"`
	PMBriefReact       bool      `gorm:"column:pm_brief_react;not null" json:"pm_brief_react"`
	ReferendumEnabled  bool      `gorm:"column:referendum_enabled;not null" json:"referendum_enabled"`
	InspectEnabled     bool      `gorm:"column:inspect_enabled;not null" json:"inspect_enabled"`
	ReactTextFreq      int16     `gorm:"column:react_text_freq;not null;check:chk_chat_presence_react_text_freq,react_text_freq BETWEEN 0 AND 3" json:"react_text_freq"`
	ReactStickerFreq   int16     `gorm:"column:react_sticker_freq;not null;check:chk_chat_presence_react_sticker_freq,react_sticker_freq BETWEEN 0 AND 3" json:"react_sticker_freq"`
	ReactPhotoFreq     int16     `gorm:"column:react_photo_freq;not null;check:chk_chat_presence_react_photo_freq,react_photo_freq BETWEEN 0 AND 3" json:"react_photo_freq"`
	ReactTopics        string    `gorm:"column:react_topics;not null;type:text;check:chk_chat_presence_react_topics,length(react_topics) <= 400" json:"react_topics"`
	BotNames           string    `gorm:"column:bot_names;not null;type:text;check:chk_chat_presence_bot_names,length(bot_names) <= 400" json:"bot_names"`
	UpdatedBy          int64     `gorm:"column:updated_by;not null" json:"updated_by"`
	UpdatedAt          time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// TableName returns the migration's table name.
func (ChatPresence) TableName() string {
	return "chat_presence"
}
