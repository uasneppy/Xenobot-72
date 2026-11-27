package models

import "time"

// Reactions stores one chat's keyword-to-emoji auto-reaction rule and its per-rule options.
// WordBoundary and MatchCaption carry NO GORM default: tag so a Go false is always written
// explicitly on Create and Save.
type Reactions struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatID       int64     `gorm:"column:chat_id;not null;uniqueIndex:idx_reactions_chat_keyword" json:"chat_id,omitempty"`
	Keyword      string    `gorm:"column:keyword;not null;uniqueIndex:idx_reactions_chat_keyword" json:"keyword,omitempty"`
	Emoji        string    `gorm:"column:emoji;not null" json:"emoji,omitempty"`
	WordBoundary bool      `gorm:"column:word_boundary;not null" json:"word_boundary,omitempty"`
	MatchCaption bool      `gorm:"column:match_caption;not null" json:"match_caption,omitempty"`
	ChancePct    int16     `gorm:"column:chance_pct;not null;default:100;check:chk_reactions_chance_pct,chance_pct BETWEEN 1 AND 100" json:"chance_pct,omitempty"`
	CooldownSecs int       `gorm:"column:cooldown_secs;not null;default:0;check:chk_reactions_cooldown_secs,cooldown_secs BETWEEN 0 AND 86400" json:"cooldown_secs,omitempty"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (Reactions) TableName() string {
	return "reactions"
}
