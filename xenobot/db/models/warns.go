package models

import (
	"math"
	"time"
)

// WarnExpiryMaxDays is the longest warn expiry a group may set, in days (the CHECK of
// warns_settings.warn_expiry_days allows 0 to this value; 0 is off).
const WarnExpiryMaxDays = 365

// warnTimeUnknown dates a warn whose time cannot be told (a value cached by a build before warn
// times existed, which carries no update time): far in the future, so such a warn always counts and
// never expires on a guess.
const warnTimeUnknown = int64(math.MaxInt64)

// WarnSettings is a chat's warn limit and the action taken at it. WarnModeSecs is the duration of a
// timed limit action (tmute, tban) in seconds: 0 for a plain action, and a timed mode stored with 0 is
// inert. It has no default tag, so a Go 0 is always written, and omitempty keeps an export with no
// duration unchanged. ExpiryDays is the number of days after which a warn stops counting toward the
// limit: 0 (the default) is off and every warn counts.
type WarnSettings struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId       int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"_id,omitempty"`
	WarnLimit    int       `gorm:"column:warn_limit;default:3;check:chk_warn_limit,warn_limit > 0" json:"warn_limit" default:"3"`
	WarnMode     string    `gorm:"column:warn_mode;check:chk_warn_mode,warn_mode = '' OR warn_mode IN ('ban','kick','mute','tban','tmute')" json:"warn_mode,omitempty"`
	WarnModeSecs int       `gorm:"column:warn_mode_secs;not null;check:chk_warns_settings_mode_secs,warn_mode_secs BETWEEN 0 AND 31622400" json:"warn_mode_secs,omitempty"`
	ExpiryDays   int       `gorm:"column:warn_expiry_days;not null;check:chk_warns_settings_expiry,warn_expiry_days BETWEEN 0 AND 365" json:"warn_expiry_days,omitempty"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (WarnSettings) TableName() string {
	return "warns_settings"
}

// Warns is one member's warns in one chat. WarnTimes holds the unix seconds of each stored warn, in
// the order of the reasons; a row saved before warn times existed has fewer times than warns, and the
// missing ones are dated at the row's last change (NormalisedTimes). omitempty keeps an export with no
// times unchanged.
type Warns struct {
	ID        uint        `gorm:"primaryKey;autoIncrement" json:"-"`
	UserId    int64       `gorm:"column:user_id;not null;uniqueIndex:uk_warns_users_user_chat" json:"user_id,omitempty"`
	ChatId    int64       `gorm:"column:chat_id;not null;uniqueIndex:uk_warns_users_user_chat;index:idx_warns_users_chat_id" json:"chat_id,omitempty"`
	NumWarns  int         `gorm:"column:num_warns;default:0;check:chk_warns_num_warns,num_warns >= 0" json:"num_warns,omitempty"`
	Reasons   StringArray `gorm:"column:warns;type:jsonb" json:"warns" default:"[]"`
	WarnTimes Int64Array  `gorm:"column:warn_times;type:jsonb" json:"warn_times,omitempty"`
	CreatedAt time.Time   `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time   `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (Warns) TableName() string {
	return "warns_users"
}

// NormalisedTimes returns exactly NumWarns non-decreasing unix times, one per warn, oldest first.
// Missing times (a row saved before warn times existed) are padded at the front with the earlier of
// UpdatedAt and the first stored time, which is never earlier than those warns' real time; extra
// times are cut from the end; a time lower than the one before it is raised to it. A zero UpdatedAt
// with no stored time (a value cached by the previous build) pads with the far future, so such a
// warn counts and never expires on a guess. It never reads the clock.
func (w *Warns) NormalisedTimes() []int64 {
	if w == nil || w.NumWarns <= 0 {
		return []int64{}
	}
	stored := []int64(w.WarnTimes)
	if len(stored) > w.NumWarns {
		stored = stored[:w.NumWarns]
	}
	pad := warnTimeUnknown
	if !w.UpdatedAt.IsZero() {
		pad = w.UpdatedAt.Unix()
	}
	if len(stored) > 0 && stored[0] < pad {
		pad = stored[0]
	}
	out := make([]int64, 0, w.NumWarns)
	for range w.NumWarns - len(stored) {
		out = append(out, pad)
	}
	out = append(out, stored...)
	for i := 1; i < len(out); i++ {
		if out[i] < out[i-1] {
			out[i] = out[i-1]
		}
	}
	return out
}

// Counted returns how many of the member's warns count toward the limit, their reasons and their
// times. With expiry off (expiryDays 0 or less) every warn counts and the stored reasons are returned
// as they are. With expiry on, a warn counts when its time is after now minus expiryDays days; the
// counted warns are a suffix of the record (the times are non-decreasing), and their reasons are the
// last min(count, len(Reasons)) reasons. It is the one counting rule every warn reader and writer uses.
func (w *Warns) Counted(expiryDays int, now time.Time) (count int, reasons []string, times []int64) {
	if w == nil {
		return 0, []string{}, []int64{}
	}
	times = w.NormalisedTimes()
	if expiryDays <= 0 {
		return w.NumWarns, []string(w.Reasons), times
	}
	cutoff := now.Add(-time.Duration(expiryDays) * 24 * time.Hour).Unix()
	first := len(times)
	for i, t := range times {
		if t > cutoff {
			first = i
			break
		}
	}
	times = times[first:]
	count = len(times)
	n := min(count, len(w.Reasons))
	reasons = append([]string{}, w.Reasons[len(w.Reasons)-n:]...)
	return count, reasons, times
}
