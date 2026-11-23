// Package advthresholds stores each group's optional advanced threshold overrides across the 6
// Phase 28 tuning areas.
//
// The cache key (chat_advanced_thresholds:<chat>) is listed in fuku/db/cache/local.go skipLocal so
// writes take effect on the very next event on every replica. Reads never create rows; writes use
// column-targeted upserts and invalidate the cache key after commit.
package advthresholds

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	fukuerrors "github.com/uasneppy/fuku_robot/fuku/utils/errors"
)

const (
	cacheModule = "chat_advanced_thresholds"
	cacheTTL    = cache.DefaultCacheTTL
)

var (
	errNoDatabase = errors.New("database not initialized")
	// ErrInvalidField is returned before any DB write when a threshold field code is unknown.
	ErrInvalidField = errors.New("advthresholds: invalid field")
	// ErrInvalidArea is returned before any DB write when an area code is unknown.
	ErrInvalidArea = errors.New("advthresholds: invalid area")
	// ErrInvalidValue is returned before any DB write when a value is outside the field's bounds.
	ErrInvalidValue = errors.New("advthresholds: invalid value")
)

// Area identifies one of the 6 advanced threshold areas.
type Area string

const (
	AreaRaidWindows      Area = "rw"
	AreaNewMember        Area = "nm"
	AreaAIBars           Area = "ab"
	AreaMemberReq        Area = "mr"
	AreaLockdownReminder Area = "lr"
	AreaCaptchaTimeout   Area = "ct"
)

// Field identifies one of the 12 tunable thresholds.
type Field string

const (
	FieldRaidJoinWindow       Field = "jsw"
	FieldRaidFloodWindow      Field = "flw"
	FieldRaidAIWindow         Field = "aiw"
	FieldRaidFloodNewPercent  Field = "fnp"
	FieldRaidFloodMinAccounts Field = "fma"
	FieldRaidRecentHours      Field = "rna"
	FieldRaidAIImageBar       Field = "rib"
	FieldMemberReqAIBar       Field = "mrb"
	FieldMemberReqCap         Field = "mrc"
	FieldMemberReqPause       Field = "mrp"
	FieldLockdownReminder     Field = "lrm"
	FieldCaptchaTimeout       Field = "ctm"
)

// FieldSpec describes the area, SQL column, safe bounds, and compiled default of one threshold.
type FieldSpec struct {
	Field   Field
	Area    Area
	Column  string
	Min     int
	Max     int
	Default int
}

var allAreas = []Area{
	AreaRaidWindows,
	AreaNewMember,
	AreaAIBars,
	AreaMemberReq,
	AreaLockdownReminder,
	AreaCaptchaTimeout,
}

var allSpecs = []FieldSpec{
	{
		Field:   FieldRaidJoinWindow,
		Area:    AreaRaidWindows,
		Column:  "raid_join_window_secs",
		Min:     models.MinRaidJoinWindowSecs,
		Max:     models.MaxRaidJoinWindowSecs,
		Default: models.DefaultRaidJoinWindowSecs,
	},
	{
		Field:   FieldRaidFloodWindow,
		Area:    AreaRaidWindows,
		Column:  "raid_flood_window_secs",
		Min:     models.MinRaidFloodWindowSecs,
		Max:     models.MaxRaidFloodWindowSecs,
		Default: models.DefaultRaidFloodWindowSecs,
	},
	{
		Field:   FieldRaidAIWindow,
		Area:    AreaRaidWindows,
		Column:  "raid_ai_window_secs",
		Min:     models.MinRaidAIWindowSecs,
		Max:     models.MaxRaidAIWindowSecs,
		Default: models.DefaultRaidAIWindowSecs,
	},
	{
		Field:   FieldRaidFloodNewPercent,
		Area:    AreaNewMember,
		Column:  "raid_flood_new_percent",
		Min:     models.MinRaidFloodNewPercent,
		Max:     models.MaxRaidFloodNewPercent,
		Default: models.DefaultRaidFloodNewPercent,
	},
	{
		Field:   FieldRaidFloodMinAccounts,
		Area:    AreaNewMember,
		Column:  "raid_flood_min_accounts",
		Min:     models.MinRaidFloodMinAccounts,
		Max:     models.MaxRaidFloodMinAccounts,
		Default: models.DefaultRaidFloodMinAccounts,
	},
	{
		Field:   FieldRaidRecentHours,
		Area:    AreaNewMember,
		Column:  "raid_recent_hours",
		Min:     models.MinRaidRecentHours,
		Max:     models.MaxRaidRecentHours,
		Default: models.DefaultRaidRecentHours,
	},
	{
		Field:   FieldRaidAIImageBar,
		Area:    AreaAIBars,
		Column:  "raid_ai_image_bar_pct",
		Min:     models.MinRaidAIImageBarPct,
		Max:     models.MaxRaidAIImageBarPct,
		Default: models.DefaultRaidAIImageBarPct,
	},
	{
		Field:   FieldMemberReqAIBar,
		Area:    AreaAIBars,
		Column:  "member_req_ai_bar_pct",
		Min:     models.MinMemberReqAIBarPct,
		Max:     models.MaxMemberReqAIBarPct,
		Default: models.DefaultMemberReqAIBarPct,
	},
	{
		Field:   FieldMemberReqCap,
		Area:    AreaMemberReq,
		Column:  "member_req_cap_per_min",
		Min:     models.MinMemberReqCapPerMin,
		Max:     models.MaxMemberReqCapPerMin,
		Default: models.DefaultMemberReqCapPerMin,
	},
	{
		Field:   FieldMemberReqPause,
		Area:    AreaMemberReq,
		Column:  "member_req_pause_mins",
		Min:     models.MinMemberReqPauseMins,
		Max:     models.MaxMemberReqPauseMins,
		Default: models.DefaultMemberReqPauseMins,
	},
	{
		Field:   FieldLockdownReminder,
		Area:    AreaLockdownReminder,
		Column:  "lockdown_reminder_mins",
		Min:     models.MinLockdownReminderMins,
		Max:     models.MaxLockdownReminderMins,
		Default: models.DefaultLockdownReminderMins,
	},
	{
		Field:   FieldCaptchaTimeout,
		Area:    AreaCaptchaTimeout,
		Column:  "captcha_timeout_mins",
		Min:     models.MinCaptchaTimeoutMins,
		Max:     models.MaxCaptchaTimeoutMins,
		Default: models.DefaultCaptchaTimeoutMins,
	},
}

var specsByField = func() map[Field]FieldSpec {
	m := make(map[Field]FieldSpec, len(allSpecs))
	for _, s := range allSpecs {
		m[s.Field] = s
	}
	return m
}()

var specsByArea = func() map[Area][]FieldSpec {
	m := make(map[Area][]FieldSpec, len(allAreas))
	for _, a := range allAreas {
		m[a] = nil
	}
	for _, s := range allSpecs {
		m[s.Area] = append(m[s.Area], s)
	}
	return m
}()

// SpecFor returns the FieldSpec of field and true when field is known.
func SpecFor(field Field) (FieldSpec, bool) {
	spec, ok := specsByField[field]
	return spec, ok
}

// FieldsInArea returns a copy of the specs belonging to area, or nil when area is unknown.
func FieldsInArea(area Area) []FieldSpec {
	specs, ok := specsByArea[area]
	if !ok {
		return nil
	}
	out := make([]FieldSpec, len(specs))
	copy(out, specs)
	return out
}

// ValidArea reports whether area is one of the 6 known areas.
func ValidArea(area Area) bool {
	_, ok := specsByArea[area]
	return ok
}

// AllAreas returns a copy of the 6 areas in display order.
func AllAreas() []Area {
	out := make([]Area, len(allAreas))
	copy(out, allAreas)
	return out
}

// AllSpecs returns a copy of all 12 FieldSpecs in display order.
func AllSpecs() []FieldSpec {
	out := make([]FieldSpec, len(allSpecs))
	copy(out, allSpecs)
	return out
}

// Thresholds is a chat's stored advanced threshold overrides. A zero value in any field means
// unset / use the compiled default (or captcha_settings.timeout for CaptchaTimeoutMins).
type Thresholds struct {
	RaidJoinWindowSecs   int `msgpack:"jsw" json:"raid_join_window_secs"`
	RaidFloodWindowSecs  int `msgpack:"flw" json:"raid_flood_window_secs"`
	RaidAIWindowSecs     int `msgpack:"aiw" json:"raid_ai_window_secs"`
	RaidFloodNewPercent  int `msgpack:"fnp" json:"raid_flood_new_percent"`
	RaidFloodMinAccounts int `msgpack:"fma" json:"raid_flood_min_accounts"`
	RaidRecentHours      int `msgpack:"rna" json:"raid_recent_hours"`
	RaidAIImageBarPct    int `msgpack:"rib" json:"raid_ai_image_bar_pct"`
	MemberReqAIBarPct    int `msgpack:"mrb" json:"member_req_ai_bar_pct"`
	MemberReqCapPerMin   int `msgpack:"mrc" json:"member_req_cap_per_min"`
	MemberReqPauseMins   int `msgpack:"mrp" json:"member_req_pause_mins"`
	LockdownReminderMins int `msgpack:"lrm" json:"lockdown_reminder_mins"`
	CaptchaTimeoutMins   int `msgpack:"ctm" json:"captcha_timeout_mins"`
}

// Value returns the stored integer for field, or 0 when unset or unknown.
func (t Thresholds) Value(field Field) int {
	switch field {
	case FieldRaidJoinWindow:
		return t.RaidJoinWindowSecs
	case FieldRaidFloodWindow:
		return t.RaidFloodWindowSecs
	case FieldRaidAIWindow:
		return t.RaidAIWindowSecs
	case FieldRaidFloodNewPercent:
		return t.RaidFloodNewPercent
	case FieldRaidFloodMinAccounts:
		return t.RaidFloodMinAccounts
	case FieldRaidRecentHours:
		return t.RaidRecentHours
	case FieldRaidAIImageBar:
		return t.RaidAIImageBarPct
	case FieldMemberReqAIBar:
		return t.MemberReqAIBarPct
	case FieldMemberReqCap:
		return t.MemberReqCapPerMin
	case FieldMemberReqPause:
		return t.MemberReqPauseMins
	case FieldLockdownReminder:
		return t.LockdownReminderMins
	case FieldCaptchaTimeout:
		return t.CaptchaTimeoutMins
	default:
		return 0
	}
}

// IsCustomized reports whether field carries a non-zero override.
func (t Thresholds) IsCustomized(field Field) bool {
	return t.Value(field) != 0
}

// CustomizedCount returns how many of the 12 thresholds carry a non-zero override.
func (t Thresholds) CustomizedCount() int {
	n := 0
	for _, spec := range allSpecs {
		if t.IsCustomized(spec.Field) {
			n++
		}
	}
	return n
}

// AreaCustomizedCount returns how many thresholds in area carry a non-zero override.
func (t Thresholds) AreaCustomizedCount(area Area) int {
	n := 0
	for _, spec := range specsByArea[area] {
		if t.IsCustomized(spec.Field) {
			n++
		}
	}
	return n
}

// CacheKey is the cache key of a chat's advanced thresholds. Its prefix is in skipLocal.
func CacheKey(chatID int64) string {
	return cache.CacheKey(cacheModule, chatID)
}

// Get returns the chat's advanced thresholds through the cache (skipLocal), or the zero
// Thresholds for a chat with no row. It creates nothing and returns DB/cache load errors.
func Get(ctx context.Context, chatID int64) (Thresholds, error) {
	th, err := cache.GetFromCacheOrLoad(ctx, CacheKey(chatID), cacheTTL, func(ctx context.Context) (*Thresholds, error) {
		v, err := GetFresh(ctx, chatID)
		if err != nil {
			return nil, err
		}
		return &v, nil
	})
	if err != nil {
		return Thresholds{}, err
	}
	if th == nil {
		return Thresholds{}, nil
	}
	return *th, nil
}

// GetFresh reads the chat's advanced thresholds directly from PostgreSQL without the cache. A
// chat with no row returns Thresholds{}, nil and creates nothing.
func GetFresh(ctx context.Context, chatID int64) (Thresholds, error) {
	if db.DB == nil {
		return Thresholds{}, fukuerrors.Wrap(errNoDatabase, "read advanced thresholds")
	}
	var row models.ChatAdvancedThresholds
	err := db.DB.WithContext(ctx).Where("chat_id = ?", chatID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Thresholds{}, nil
	}
	if err != nil {
		return Thresholds{}, fukuerrors.Wrapf(err, "read advanced thresholds of chat %d", chatID)
	}
	return Thresholds{
		RaidJoinWindowSecs:   row.RaidJoinWindowSecs,
		RaidFloodWindowSecs:  row.RaidFloodWindowSecs,
		RaidAIWindowSecs:     row.RaidAIWindowSecs,
		RaidFloodNewPercent:  row.RaidFloodNewPercent,
		RaidFloodMinAccounts: row.RaidFloodMinAccounts,
		RaidRecentHours:      row.RaidRecentHours,
		RaidAIImageBarPct:    row.RaidAIImageBarPct,
		MemberReqAIBarPct:    row.MemberReqAIBarPct,
		MemberReqCapPerMin:   row.MemberReqCapPerMin,
		MemberReqPauseMins:   row.MemberReqPauseMins,
		LockdownReminderMins: row.LockdownReminderMins,
		CaptchaTimeoutMins:   row.CaptchaTimeoutMins,
	}, nil
}

// SetField validates field and value (0 or within [spec.Min, spec.Max]) before touching the DB,
// writes only that column via a column-targeted upsert, and drops the cache key after commit.
func SetField(ctx context.Context, chatID int64, field Field, value int, updatedBy int64) error {
	spec, ok := SpecFor(field)
	if !ok {
		return ErrInvalidField
	}
	if value != 0 && (value < spec.Min || value > spec.Max) {
		return ErrInvalidValue
	}
	row := baseRow(chatID, updatedBy)
	setRowField(row, field, value)
	return upsertColumns(ctx, row, spec.Column)
}

// ResetField sets one threshold column back to 0 (default) via a column-targeted upsert and drops
// the cache key.
func ResetField(ctx context.Context, chatID int64, field Field, updatedBy int64) error {
	return SetField(ctx, chatID, field, 0, updatedBy)
}

// ResetArea zeroes every threshold column belonging to area in one column-targeted upsert while
// leaving other areas untouched, and drops the cache key after commit.
func ResetArea(ctx context.Context, chatID int64, area Area, updatedBy int64) error {
	specs, ok := specsByArea[area]
	if !ok {
		return ErrInvalidArea
	}
	cols := make([]string, len(specs))
	for i, spec := range specs {
		cols[i] = spec.Column
	}
	row := baseRow(chatID, updatedBy)
	return upsertColumns(ctx, row, cols...)
}

// ResetAll deletes the chat's advanced thresholds row and drops the cache key.
func ResetAll(ctx context.Context, chatID int64) error {
	if db.DB == nil {
		return fukuerrors.Wrap(errNoDatabase, "reset advanced thresholds")
	}
	if err := db.DB.WithContext(ctx).Where("chat_id = ?", chatID).Delete(&models.ChatAdvancedThresholds{}).Error; err != nil {
		return fukuerrors.Wrapf(err, "reset advanced thresholds of chat %d", chatID)
	}
	cache.DeleteCache(CacheKey(chatID))
	return nil
}

// ReplaceAll validates all 12 threshold fields against their specs and atomically replaces chatID's
// chat_advanced_thresholds row (or deletes it via ResetAll when th has no customized fields),
// dropping chat_advanced_thresholds:<chat> after commit.
func ReplaceAll(ctx context.Context, chatID int64, th Thresholds, by int64) error {
	for _, spec := range allSpecs {
		v := th.Value(spec.Field)
		if v != 0 && (v < spec.Min || v > spec.Max) {
			return ErrInvalidValue
		}
	}
	if th.CustomizedCount() == 0 {
		return ResetAll(ctx, chatID)
	}
	row := baseRow(chatID, by)
	cols := make([]string, len(allSpecs))
	for i, spec := range allSpecs {
		setRowField(row, spec.Field, th.Value(spec.Field))
		cols[i] = spec.Column
	}
	return upsertColumns(ctx, row, cols...)
}

func baseRow(chatID, updatedBy int64) *models.ChatAdvancedThresholds {
	return &models.ChatAdvancedThresholds{
		ChatId:    chatID,
		UpdatedBy: updatedBy,
		UpdatedAt: time.Now(),
	}
}

func setRowField(row *models.ChatAdvancedThresholds, field Field, value int) {
	switch field {
	case FieldRaidJoinWindow:
		row.RaidJoinWindowSecs = value
	case FieldRaidFloodWindow:
		row.RaidFloodWindowSecs = value
	case FieldRaidAIWindow:
		row.RaidAIWindowSecs = value
	case FieldRaidFloodNewPercent:
		row.RaidFloodNewPercent = value
	case FieldRaidFloodMinAccounts:
		row.RaidFloodMinAccounts = value
	case FieldRaidRecentHours:
		row.RaidRecentHours = value
	case FieldRaidAIImageBar:
		row.RaidAIImageBarPct = value
	case FieldMemberReqAIBar:
		row.MemberReqAIBarPct = value
	case FieldMemberReqCap:
		row.MemberReqCapPerMin = value
	case FieldMemberReqPause:
		row.MemberReqPauseMins = value
	case FieldLockdownReminder:
		row.LockdownReminderMins = value
	case FieldCaptchaTimeout:
		row.CaptchaTimeoutMins = value
	}
}

func upsertColumns(ctx context.Context, row *models.ChatAdvancedThresholds, columns ...string) error {
	if db.DB == nil {
		return fukuerrors.Wrap(errNoDatabase, "write advanced thresholds")
	}
	updateCols := make([]string, 0, len(columns)+2)
	updateCols = append(updateCols, columns...)
	updateCols = append(updateCols, "updated_by", "updated_at")
	err := db.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.AssignmentColumns(updateCols),
	}).Create(row).Error
	if err != nil {
		return fukuerrors.Wrapf(err, "write advanced thresholds of chat %d", row.ChatId)
	}
	cache.DeleteCache(CacheKey(row.ChatId))
	return nil
}
