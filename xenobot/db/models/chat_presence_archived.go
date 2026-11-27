package models

import "time"

// ChatPresenceArchived records that a viral message in a chat has already been sent to the
// chat's Highlights Archive destination: one row per (chat_id, message_id), kept for good.
// The composite primary key ensures that concurrent replicas racing on the same reaction surge
// agree on exactly one winner via ON CONFLICT DO NOTHING, surviving Redis restarts.
//
// There is deliberately no foreign key and no GORM default tag on the primary key columns.
type ChatPresenceArchived struct {
	ChatID     int64     `gorm:"column:chat_id;primaryKey;autoIncrement:false"`
	MessageID  int64     `gorm:"column:message_id;primaryKey;autoIncrement:false"`
	ArchivedAt time.Time `gorm:"column:archived_at;not null"`
}

// TableName returns the migration's table name.
func (ChatPresenceArchived) TableName() string {
	return "chat_presence_archived"
}
