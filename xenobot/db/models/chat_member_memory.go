package models

import "time"

// ChatMemberMemory is the notes the conversation feature keeps about one member of one group, used
// only in that group. Notes is the notes joined by new lines (at most 12 notes of at most 200
// characters, see xenobot/db/membermem). It is read fresh, never cached, and is never part of a
// backup.
type ChatMemberMemory struct {
	ChatID    int64     `gorm:"column:chat_id;primaryKey;autoIncrement:false" json:"chat_id"`
	UserID    int64     `gorm:"column:user_id;primaryKey;autoIncrement:false" json:"user_id"`
	Notes     string    `gorm:"column:notes;not null;type:text;check:chk_chat_member_memories_notes,length(notes) <= 2500" json:"notes"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

// TableName returns the migration's table name.
func (ChatMemberMemory) TableName() string {
	return "chat_member_memories"
}
