// Package escalation stores each group's escalation ladder and its members' offence counts.
//
// The ladder (chat_escalation) is a cached per-chat setting with column-targeted writes. The
// counts (chat_offences) are written only by CountOffence, one conditional upsert that every
// replica shares, and are never cached.
package escalation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	fukuerrors "github.com/uasneppy/fuku_robot/fuku/utils/errors"
)

const (
	// MaxSteps is the most steps a ladder holds.
	MaxSteps = 5
	// SameOffenceWindow is how close to the last counted offence a new one is the same offence:
	// it is not counted and does not move the time, so a burst counts once a minute at most.
	SameOffenceWindow = 60 * time.Second
	// DefaultForgetDays is the forget period of a chat with no row, and of a row's first write.
	DefaultForgetDays = 30
	// MinForgetDays and MaxForgetDays bound the forget period, as the table's check does.
	MinForgetDays = 1
	MaxForgetDays = 365

	cacheModule = "chat_escalation"
	cacheTTL    = cache.DefaultCacheTTL
)

// The step actions a ladder may store.
const (
	ActionMute  = "mute"
	ActionTMute = "tmute"
	ActionKick  = "kick"
	ActionTBan  = "tban"
	ActionBan   = "ban"
)

var (
	errNoDatabase = errors.New("database not initialized")
	// ErrInvalidValue is returned, before any write, for a step, step number or forget period
	// outside the stored ones, and by CountOffence for a chat of 0, a user ID of 0 or less, or
	// a forget period under one second.
	ErrInvalidValue = errors.New("escalation: invalid value")
)

// Step is one step of a ladder: an action and, for tmute and tban only, its seconds.
type Step struct {
	Action string `msgpack:"a" json:"action"`
	Secs   int    `msgpack:"s" json:"secs,omitempty"`
}

// Ladder is a chat's escalation ladder. Steps runs from step 1 up to the first empty step.
type Ladder struct {
	Enabled    bool   `msgpack:"e" json:"enabled"`
	Steps      []Step `msgpack:"t" json:"steps,omitempty"`
	ForgetDays int    `msgpack:"f" json:"forget_days"`
}

// Defaults is the ladder of a chat with no row: off, no steps, DefaultForgetDays.
func Defaults() Ladder {
	return Ladder{ForgetDays: DefaultForgetDays}
}

// ValidStep reports whether s may be stored as a step: the action is one of mute, tmute, kick,
// tban and ban; a timed one has a duration models.PunishDurationValid accepts, an untimed one 0.
func ValidStep(s Step) bool {
	switch s.Action {
	case ActionTMute, ActionTBan:
		return models.PunishDurationValid(s.Secs)
	case ActionMute, ActionKick, ActionBan:
		return s.Secs == 0
	default:
		return false
	}
}

// ValidForgetDays reports whether days is a forget period the table accepts.
func ValidForgetDays(days int) bool {
	return days >= MinForgetDays && days <= MaxForgetDays
}

// CacheKey is the cache key of a chat's ladder. Its prefix is listed in skipLocal.
func CacheKey(chatID int64) string {
	return cache.CacheKey(cacheModule, chatID)
}

// Get returns the chat's ladder through the cache, the defaults for a chat with no row. It
// creates nothing and returns its error.
func Get(ctx context.Context, chatID int64) (*Ladder, error) {
	ladder, err := cache.GetFromCacheOrLoad(ctx, CacheKey(chatID), cacheTTL, func(ctx context.Context) (*Ladder, error) {
		return GetFresh(ctx, chatID)
	})
	if err != nil {
		return nil, err
	}
	if ladder == nil {
		d := Defaults()
		return &d, nil
	}
	return ladder, nil
}

// GetFresh is Get without the cache.
func GetFresh(ctx context.Context, chatID int64) (*Ladder, error) {
	if db.DB == nil {
		return nil, fukuerrors.Wrap(errNoDatabase, "read the escalation ladder")
	}
	var row models.ChatEscalation
	err := db.DB.WithContext(ctx).Where("chat_id = ?", chatID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		d := Defaults()
		return &d, nil
	}
	if err != nil {
		return nil, fukuerrors.Wrapf(err, "read the escalation ladder of chat %d", chatID)
	}
	return ladderOf(&row), nil
}

// ladderOf turns a row into a Ladder whose steps stop at the first empty one.
func ladderOf(row *models.ChatEscalation) *Ladder {
	ladder := &Ladder{Enabled: row.Enabled, ForgetDays: row.ForgetDays}
	for _, s := range rowSteps(row) {
		if s.Action == "" {
			break
		}
		ladder.Steps = append(ladder.Steps, s)
	}
	return ladder
}

func rowSteps(row *models.ChatEscalation) [MaxSteps]Step {
	return [MaxSteps]Step{
		{Action: row.Step1Action, Secs: row.Step1Secs},
		{Action: row.Step2Action, Secs: row.Step2Secs},
		{Action: row.Step3Action, Secs: row.Step3Secs},
		{Action: row.Step4Action, Secs: row.Step4Secs},
		{Action: row.Step5Action, Secs: row.Step5Secs},
	}
}

// setRowStep stores s as step n (1..MaxSteps) of row.
func setRowStep(row *models.ChatEscalation, n int, s Step) {
	switch n {
	case 1:
		row.Step1Action, row.Step1Secs = s.Action, s.Secs
	case 2:
		row.Step2Action, row.Step2Secs = s.Action, s.Secs
	case 3:
		row.Step3Action, row.Step3Secs = s.Action, s.Secs
	case 4:
		row.Step4Action, row.Step4Secs = s.Action, s.Secs
	case 5:
		row.Step5Action, row.Step5Secs = s.Action, s.Secs
	}
}

func stepColumns(n int) []string {
	return []string{fmt.Sprintf("step%d_action", n), fmt.Sprintf("step%d_secs", n)}
}

// SetEnabled writes the on/off switch with one column-targeted upsert and drops the key.
func SetEnabled(ctx context.Context, chatID int64, on bool, by int64) error {
	row := defaultRow(chatID, by)
	row.Enabled = on
	return upsert(ctx, row, "enabled")
}

// SetEnabledWithSteps turns the ladder on and stores steps as steps 1 to len(steps), clearing
// the rest, in one upsert. Every step must be valid and there may be at most MaxSteps; anything
// else writes nothing.
func SetEnabledWithSteps(ctx context.Context, chatID int64, steps []Step, by int64) error {
	if len(steps) > MaxSteps {
		return ErrInvalidValue
	}
	for _, s := range steps {
		if !ValidStep(s) {
			return ErrInvalidValue
		}
	}
	row := defaultRow(chatID, by)
	row.Enabled = true
	columns := []string{"enabled"}
	for n := 1; n <= MaxSteps; n++ {
		if n <= len(steps) {
			setRowStep(row, n, steps[n-1])
		}
		columns = append(columns, stepColumns(n)...)
	}
	return upsert(ctx, row, columns...)
}

// SetStep writes step n (1..MaxSteps). An empty Step clears it; any other must be valid. A step
// number or step outside those writes nothing.
func SetStep(ctx context.Context, chatID int64, n int, s Step, by int64) error {
	if n < 1 || n > MaxSteps {
		return ErrInvalidValue
	}
	if s != (Step{}) && !ValidStep(s) {
		return ErrInvalidValue
	}
	row := defaultRow(chatID, by)
	setRowStep(row, n, s)
	return upsert(ctx, row, stepColumns(n)...)
}

// SetForgetDays writes the forget period (MinForgetDays..MaxForgetDays); anything else writes
// nothing.
func SetForgetDays(ctx context.Context, chatID int64, days int, by int64) error {
	if !ValidForgetDays(days) {
		return ErrInvalidValue
	}
	row := defaultRow(chatID, by)
	row.ForgetDays = days
	return upsert(ctx, row, "forget_days")
}

// ReplaceLadderTx validates ladder and replaces chatID's entire escalation ladder inside
// caller-supplied transaction tx. The caller is responsible for dropping CacheKey(chatID) after
// commit.
func ReplaceLadderTx(tx *gorm.DB, chatID int64, ladder Ladder, by int64) error {
	forgetDays := ladder.ForgetDays
	if forgetDays == 0 {
		forgetDays = DefaultForgetDays
	}
	if !ValidForgetDays(forgetDays) || len(ladder.Steps) > MaxSteps {
		return ErrInvalidValue
	}
	hasSteps := false
	seenEmpty := false
	for _, s := range ladder.Steps {
		if s == (Step{}) {
			seenEmpty = true
			continue
		}
		if seenEmpty || !ValidStep(s) {
			return ErrInvalidValue
		}
		hasSteps = true
	}
	if tx == nil {
		return fukuerrors.Wrap(errNoDatabase, "write the escalation ladder")
	}
	if !ladder.Enabled && !hasSteps && forgetDays == DefaultForgetDays {
		if err := tx.Where("chat_id = ?", chatID).Delete(&models.ChatEscalation{}).Error; err != nil {
			return fukuerrors.Wrapf(err, "reset the escalation ladder of chat %d", chatID)
		}
		return nil
	}
	row := defaultRow(chatID, by)
	row.Enabled = ladder.Enabled
	row.ForgetDays = forgetDays
	columns := make([]string, 0, 2+2*MaxSteps)
	columns = append(columns, "enabled", "forget_days")
	for n := 1; n <= MaxSteps; n++ {
		if n <= len(ladder.Steps) {
			setRowStep(row, n, ladder.Steps[n-1])
		}
		columns = append(columns, stepColumns(n)...)
	}
	return upsertTx(tx, row, columns...)
}

// ReplaceLadder atomically replaces chatID's entire escalation ladder (enabled, all five steps,
// and forget_days) and drops chat_escalation:<chat>. ForgetDays == 0 is normalized to
// DefaultForgetDays. Invalid values return ErrInvalidValue before touching the database. When the
// normalized ladder matches Defaults() (disabled, no steps, DefaultForgetDays), any existing row
// is deleted so the target mirrors an unconfigured source cleanly.
func ReplaceLadder(ctx context.Context, chatID int64, ladder Ladder, by int64) error {
	var tx *gorm.DB
	if db.DB != nil {
		tx = db.DB.WithContext(ctx)
	}
	if err := ReplaceLadderTx(tx, chatID, ladder, by); err != nil {
		return err
	}
	cache.DeleteCache(CacheKey(chatID))
	return nil
}

func defaultRow(chatID, by int64) *models.ChatEscalation {
	return &models.ChatEscalation{ChatId: chatID, ForgetDays: DefaultForgetDays, UpdatedBy: by, UpdatedAt: time.Now()}
}

func upsertTx(tx *gorm.DB, row *models.ChatEscalation, columns ...string) error {
	if tx == nil {
		return fukuerrors.Wrap(errNoDatabase, "write the escalation ladder")
	}
	err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.AssignmentColumns(append(columns, "updated_by", "updated_at")),
	}).Create(row).Error
	if err != nil {
		return fukuerrors.Wrapf(err, "write the escalation ladder of chat %d", row.ChatId)
	}
	return nil
}

// upsert inserts the row or updates only columns, updated_by and updated_at, so two admins
// changing two settings never clobber each other, and drops the cache key.
func upsert(ctx context.Context, row *models.ChatEscalation, columns ...string) error {
	if db.DB == nil {
		return fukuerrors.Wrap(errNoDatabase, "write the escalation ladder")
	}
	if err := upsertTx(db.DB.WithContext(ctx), row, columns...); err != nil {
		return err
	}
	cache.DeleteCache(CacheKey(row.ChatId))
	return nil
}

// countOffenceSQL counts one offence in one statement. A new member starts at 1. For an existing
// row, in this order: an offence less than the same-offence window after the last counted one
// (a clock behind included) is the same offence and keeps the count and the time; an offence at
// least the forget period after it restarts at 1; anything else adds 1. A stored count of 0 is
// never "the same offence", so it restarts at 1. PostgreSQL's row lock makes concurrent calls for
// one member wait for each other, so each offence is counted once across replicas.
const countOffenceSQL = `INSERT INTO chat_offences (chat_id, user_id, offences, last_offence_unix)
VALUES (?, ?, 1, ?)
ON CONFLICT (chat_id, user_id) DO UPDATE SET
	offences = CASE
		WHEN chat_offences.offences > 0 AND excluded.last_offence_unix - chat_offences.last_offence_unix < ? THEN chat_offences.offences
		WHEN excluded.last_offence_unix - chat_offences.last_offence_unix >= ? THEN 1
		ELSE chat_offences.offences + 1
	END,
	last_offence_unix = CASE
		WHEN chat_offences.offences > 0 AND excluded.last_offence_unix - chat_offences.last_offence_unix < ? THEN chat_offences.last_offence_unix
		ELSE excluded.last_offence_unix
	END
RETURNING offences`

// CountOffence counts one offence of userID in chatID at now and returns the member's count:
// 1 for a first offence, the same count for an offence within SameOffenceWindow of the last
// counted one (whose time does not move), 1 again after forget with no counted offence, else one
// more. It is one conditional upsert, never cached, and refuses a chat of 0, a user ID of 0 or
// less and a forget period under one second before any write.
func CountOffence(ctx context.Context, chatID, userID int64, now time.Time, forget time.Duration) (int, error) {
	if chatID == 0 || userID <= 0 || forget < time.Second {
		return 0, ErrInvalidValue
	}
	if db.DB == nil {
		return 0, fukuerrors.Wrap(errNoDatabase, "count an offence")
	}
	window := int64(SameOffenceWindow / time.Second)
	forgetSecs := int64(forget / time.Second)
	var count int
	err := db.DB.WithContext(ctx).Raw(countOffenceSQL, chatID, userID, now.Unix(), window, forgetSecs, window).Scan(&count).Error
	if err != nil {
		return 0, fukuerrors.Wrapf(err, "count an offence of user %d in chat %d", userID, chatID)
	}
	return count, nil
}

// ClearOffences deletes every offence count of the chat.
func ClearOffences(ctx context.Context, chatID int64) error {
	if db.DB == nil {
		return fukuerrors.Wrap(errNoDatabase, "clear the offence counts")
	}
	if err := db.DB.WithContext(ctx).Where("chat_id = ?", chatID).Delete(&models.ChatOffence{}).Error; err != nil {
		return fukuerrors.Wrapf(err, "clear the offence counts of chat %d", chatID)
	}
	return nil
}
