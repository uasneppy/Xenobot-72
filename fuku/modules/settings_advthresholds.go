package modules

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/fuku_robot/fuku/db/advthresholds"
	"github.com/uasneppy/fuku_robot/fuku/db/staff"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
)

// The Advanced thresholds page (page av, Phase 28, TUNE-07): exposes all 12 tunable thresholds
// across 6 areas in the bot's private chat (PrivateOnly: a group menu shows settings_adv_group_note
// with 0 DB reads) and hides itself in Staff Groups. Viewing av, aa, and ao needs live
// settingsRightAdmin + the settings owner gate; writes and resets (as, ar, ax) need live
// settingsRightRestrict (P28-03) and route through applyAdvancedThreshold, resetAdvancedThreshold,
// and resetAdvancedArea.

const (
	// settingsPageAdvanced is the p value of the Advanced thresholds page.
	settingsPageAdvanced = "av"

	// Action codes of the Advanced thresholds page.
	settingsActAdvArea      = "aa"
	settingsActAdvItem      = "ao"
	settingsActAdvSet       = "as"
	settingsActAdvReset     = "ar"
	settingsActAdvResetArea = "ax"
)

// settingsAdvUnit selects how a threshold's value, default, and allowed range are formatted.
type settingsAdvUnit int

const (
	advUnitSeconds settingsAdvUnit = iota
	advUnitMinutes
	advUnitHours
	advUnitPercent
	advUnitAccounts
	advUnitPerMin
	advUnitBar
	advUnitCaptchaMins
)

// settingsAdvAreaDef describes one of the 6 areas on the Advanced hub.
type settingsAdvAreaDef struct {
	Area     advthresholds.Area
	Icon     string
	NameKey  string
	IntroKey string
}

// settingsAdvItemDef describes one of the 12 tunable thresholds and its curated presets.
type settingsAdvItemDef struct {
	Field   advthresholds.Field
	Area    advthresholds.Area
	NameKey string
	DescKey string
	Unit    settingsAdvUnit
	Presets []int
}

var settingsAdvAreas = []settingsAdvAreaDef{
	{
		Area:     advthresholds.AreaRaidWindows,
		Icon:     "⏱",
		NameKey:  "settings_adv_area_rw",
		IntroKey: "settings_adv_area_intro_rw",
	},
	{
		Area:     advthresholds.AreaNewMember,
		Icon:     "🆕",
		NameKey:  "settings_adv_area_nm",
		IntroKey: "settings_adv_area_intro_nm",
	},
	{
		Area:     advthresholds.AreaAIBars,
		Icon:     "🤖",
		NameKey:  "settings_adv_area_ab",
		IntroKey: "settings_adv_area_intro_ab",
	},
	{
		Area:     advthresholds.AreaMemberReq,
		Icon:     "🛂",
		NameKey:  "settings_adv_area_mr",
		IntroKey: "settings_adv_area_intro_mr",
	},
	{
		Area:     advthresholds.AreaLockdownReminder,
		Icon:     "🔔",
		NameKey:  "settings_adv_area_lr",
		IntroKey: "settings_adv_area_intro_lr",
	},
	{
		Area:     advthresholds.AreaCaptchaTimeout,
		Icon:     "🧩",
		NameKey:  "settings_adv_area_ct",
		IntroKey: "settings_adv_area_intro_ct",
	},
}

var settingsAdvItems = []settingsAdvItemDef{
	{
		Field:   advthresholds.FieldRaidJoinWindow,
		Area:    advthresholds.AreaRaidWindows,
		NameKey: "settings_adv_item_jsw",
		DescKey: "settings_adv_desc_jsw",
		Unit:    advUnitSeconds,
		Presets: []int{15, 30, 60, 120, 300, 600},
	},
	{
		Field:   advthresholds.FieldRaidFloodWindow,
		Area:    advthresholds.AreaRaidWindows,
		NameKey: "settings_adv_item_flw",
		DescKey: "settings_adv_desc_flw",
		Unit:    advUnitSeconds,
		Presets: []int{10, 15, 30, 60, 120, 300},
	},
	{
		Field:   advthresholds.FieldRaidAIWindow,
		Area:    advthresholds.AreaRaidWindows,
		NameKey: "settings_adv_item_aiw",
		DescKey: "settings_adv_desc_aiw",
		Unit:    advUnitSeconds,
		Presets: []int{30, 60, 120, 300, 600, 900},
	},
	{
		Field:   advthresholds.FieldRaidFloodNewPercent,
		Area:    advthresholds.AreaNewMember,
		NameKey: "settings_adv_item_fnp",
		DescKey: "settings_adv_desc_fnp",
		Unit:    advUnitPercent,
		Presets: []int{20, 40, 50, 60, 75, 90, 100},
	},
	{
		Field:   advthresholds.FieldRaidFloodMinAccounts,
		Area:    advthresholds.AreaNewMember,
		NameKey: "settings_adv_item_fma",
		DescKey: "settings_adv_desc_fma",
		Unit:    advUnitAccounts,
		Presets: []int{2, 3, 5, 8, 10, 15, 20},
	},
	{
		Field:   advthresholds.FieldRaidRecentHours,
		Area:    advthresholds.AreaNewMember,
		NameKey: "settings_adv_item_rna",
		DescKey: "settings_adv_desc_rna",
		Unit:    advUnitHours,
		Presets: []int{1, 6, 12, 24, 48, 72, 168},
	},
	{
		Field:   advthresholds.FieldRaidAIImageBar,
		Area:    advthresholds.AreaAIBars,
		NameKey: "settings_adv_item_rib",
		DescKey: "settings_adv_desc_rib",
		Unit:    advUnitBar,
		Presets: []int{60, 70, 75, 80, 85, 90, 95},
	},
	{
		Field:   advthresholds.FieldMemberReqAIBar,
		Area:    advthresholds.AreaAIBars,
		NameKey: "settings_adv_item_mrb",
		DescKey: "settings_adv_desc_mrb",
		Unit:    advUnitBar,
		Presets: []int{65, 75, 80, 85, 90, 95, 98},
	},
	{
		Field:   advthresholds.FieldMemberReqCap,
		Area:    advthresholds.AreaMemberReq,
		NameKey: "settings_adv_item_mrc",
		DescKey: "settings_adv_desc_mrc",
		Unit:    advUnitPerMin,
		Presets: []int{3, 5, 10, 20, 30, 50, 100},
	},
	{
		Field:   advthresholds.FieldMemberReqPause,
		Area:    advthresholds.AreaMemberReq,
		NameKey: "settings_adv_item_mrp",
		DescKey: "settings_adv_desc_mrp",
		Unit:    advUnitMinutes,
		Presets: []int{1, 5, 10, 15, 30, 60},
	},
	{
		Field:   advthresholds.FieldLockdownReminder,
		Area:    advthresholds.AreaLockdownReminder,
		NameKey: "settings_adv_item_lrm",
		DescKey: "settings_adv_desc_lrm",
		Unit:    advUnitMinutes,
		Presets: []int{15, 30, 60, 120, 240, 720, 1440},
	},
	{
		Field:   advthresholds.FieldCaptchaTimeout,
		Area:    advthresholds.AreaCaptchaTimeout,
		NameKey: "settings_adv_item_ctm",
		DescKey: "settings_adv_desc_ctm",
		Unit:    advUnitCaptchaMins,
		Presets: []int{15, 20, 30, 45, 60},
	},
}

// settingsAdvRead reads a chat's advanced thresholds fresh; a variable only so a test can fail or count it.
var settingsAdvRead = advthresholds.GetFresh

// settingsAdvStaffLookup reads a chat's Staff Group row fresh for Hidden; a variable only so a test can fail it.
var settingsAdvStaffLookup = staff.GetStaffGroupFresh

func init() {
	private := func(check func(map[string]string) bool) func(map[string]string) bool {
		return func(fields map[string]string) bool { return fields["c"] != "" && check(fields) }
	}
	registerSettingsPage(settingsPage{
		Key:            settingsPageAdvanced,
		Order:          81,
		Icon:           "🎛",
		TitleKey:       "settings_page_advanced",
		MainState:      settingsAdvMainState,
		Open:           settingsAdvOpen,
		Hidden:         settingsAdvHidden,
		PrivateOnly:    true,
		PrivateNoteKey: "settings_adv_group_note",
		Actions: map[string]settingsPageAction{
			settingsActAdvArea: {
				Fields: []string{"k"}, Right: settingsRightAdmin, Run: settingsAdvRunArea,
				Validate: private(func(f map[string]string) bool {
					_, ok := settingsAdvAreaByCode(f["k"])
					return ok
				}),
			},
			settingsActAdvItem: {
				Fields: []string{"t"}, Right: settingsRightAdmin, Run: settingsAdvRunItem,
				Validate: private(func(f map[string]string) bool {
					_, ok := settingsAdvItemByCode(f["t"])
					return ok
				}),
			},
			settingsActAdvSet: {
				Fields: []string{"t", "v"}, Right: settingsRightRestrict, Run: settingsAdvRunSet,
				Validate: private(func(f map[string]string) bool {
					item, ok := settingsAdvItemByCode(f["t"])
					if !ok {
						return false
					}
					_, valid := settingsAdvPresetValue(item, f["v"])
					return valid
				}),
			},
			settingsActAdvReset: {
				Fields: []string{"t"}, Right: settingsRightRestrict, Run: settingsAdvRunReset,
				Validate: private(func(f map[string]string) bool {
					_, ok := settingsAdvItemByCode(f["t"])
					return ok
				}),
			},
			settingsActAdvResetArea: {
				Fields: []string{"k"}, Right: settingsRightRestrict, Run: settingsAdvRunResetArea,
				Validate: private(func(f map[string]string) bool {
					_, ok := settingsAdvAreaByCode(f["k"])
					return ok
				}),
			},
		},
	})
}

func settingsAdvAreaByCode(code string) (settingsAdvAreaDef, bool) {
	for _, area := range settingsAdvAreas {
		if string(area.Area) == code {
			return area, true
		}
	}
	return settingsAdvAreaDef{}, false
}

func settingsAdvItemByCode(code string) (settingsAdvItemDef, bool) {
	for _, item := range settingsAdvItems {
		if string(item.Field) == code {
			return item, true
		}
	}
	return settingsAdvItemDef{}, false
}

func settingsAdvItemsForArea(area advthresholds.Area) []settingsAdvItemDef {
	var out []settingsAdvItemDef
	for _, item := range settingsAdvItems {
		if item.Area == area {
			out = append(out, item)
		}
	}
	return out
}

// settingsAdvPresetValue parses a canonical decimal preset value belonging to item and inside Go bounds.
func settingsAdvPresetValue(item settingsAdvItemDef, raw string) (int, bool) {
	val, ok := settingsParseNumber(raw, 4)
	if !ok || strconv.Itoa(val) != raw {
		return 0, false
	}
	spec, specOK := advthresholds.SpecFor(item.Field)
	if !specOK || val < spec.Min || val > spec.Max {
		return 0, false
	}
	if !slices.Contains(item.Presets, val) {
		return 0, false
	}
	return val, true
}

func settingsAdvContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), settingsReadTimeout)
}

func settingsAdvReadNow(chatID int64) (advthresholds.Thresholds, error) {
	ctx, cancel := settingsAdvContext()
	defer cancel()
	return settingsAdvRead(ctx, chatID)
}

// settingsAdvHidden hides the page in Staff Groups (definite fresh row); an error shows the page.
func settingsAdvHidden(group *gotgbot.Chat) bool {
	row, err := settingsAdvStaffLookup(group.Id)
	return err == nil && row != nil
}

// settingsAdvStateLabel formats a customization count: Default when 0, "{n} custom" otherwise.
func settingsAdvStateLabel(tr *i18n.Translator, count int) string {
	if count == 0 {
		return settingsText(tr, "settings_state_default")
	}
	return settingsText(tr, "settings_state_adv_n", i18n.TranslationParams{"n": count})
}

// settingsAdvMainState shows Default when no thresholds are customized, or "{n} custom".
// PrivateOnly short-circuits group menus before MainState is called, so group menus make 0 DB reads.
func settingsAdvMainState(_ *gotgbot.Bot, tr *i18n.Translator, group *gotgbot.Chat) (string, bool) {
	th, err := settingsAdvReadNow(group.Id)
	if err != nil {
		return "", false
	}
	return settingsAdvStateLabel(tr, th.CustomizedCount()), true
}

// settingsAdvOpen draws the Advanced hub (page av). A read error produces the cannot-open edit.
func settingsAdvOpen(p *settingsPress, notice string) (settingsScreen, error) {
	th, err := settingsAdvReadNow(p.group.Id)
	if err != nil {
		return settingsScreen{}, err
	}
	return renderSettingsAdvHub(p.view(), th, notice), nil
}

func settingsAdvFormatValue(tr *i18n.Translator, unit settingsAdvUnit, value int) string {
	switch unit {
	case advUnitSeconds:
		return settingsText(tr, "settings_adv_unit_s", i18n.TranslationParams{"n": value})
	case advUnitMinutes:
		return settingsText(tr, "settings_adv_unit_m", i18n.TranslationParams{"n": value})
	case advUnitHours:
		return settingsText(tr, "settings_adv_unit_h", i18n.TranslationParams{"n": value})
	case advUnitPercent:
		return settingsText(tr, "settings_adv_unit_pct", i18n.TranslationParams{"n": value})
	case advUnitAccounts:
		return settingsText(tr, "settings_adv_unit_accts", i18n.TranslationParams{"n": value})
	case advUnitPerMin:
		return settingsText(tr, "settings_adv_unit_per_min", i18n.TranslationParams{"n": value})
	case advUnitBar:
		return settingsText(tr, "settings_adv_unit_bar", i18n.TranslationParams{"n": fmt.Sprintf("%.2f", float64(value)/100.0)})
	case advUnitCaptchaMins:
		if value == 0 {
			return settingsText(tr, "settings_adv_captcha_default")
		}
		return settingsText(tr, "settings_adv_unit_m", i18n.TranslationParams{"n": value})
	default:
		return strconv.Itoa(value)
	}
}

func settingsAdvFormatRange(tr *i18n.Translator, unit settingsAdvUnit, minVal, maxVal int) string {
	return settingsAdvFormatValue(tr, unit, minVal) + "–" + settingsAdvFormatValue(tr, unit, maxVal)
}

func settingsAdvEffectiveValue(th advthresholds.Thresholds, item settingsAdvItemDef, spec advthresholds.FieldSpec) int {
	if v := th.Value(item.Field); v > 0 {
		return v
	}
	return spec.Default
}

// renderSettingsAdvHub draws the Advanced hub (av): intro, status line for all 6 areas, 2-column
// area buttons, and Back to Main + Close.
func renderSettingsAdvHub(v settingsView, th advthresholds.Thresholds, notice string) settingsScreen {
	body := make([]string, 0, len(settingsAdvAreas)+2)
	body = append(body, settingsText(v.Tr, "settings_adv_intro"), "")
	for _, area := range settingsAdvAreas {
		state := settingsAdvStateLabel(v.Tr, th.AreaCustomizedCount(area.Area))
		body = append(body, settingsText(v.Tr, "settings_adv_area_line", i18n.TranslationParams{
			"icon":  area.Icon,
			"name":  settingsText(v.Tr, area.NameKey),
			"state": state,
		}))
	}

	k := newSettingsKeyboard(v)
	for i := 0; i < len(settingsAdvAreas); i += 2 {
		var row []gotgbot.InlineKeyboardButton
		for _, area := range settingsAdvAreas[i:min(i+2, len(settingsAdvAreas))] {
			label := area.Icon + " " + settingsText(v.Tr, area.NameKey)
			if btn, ok := k.button(label, map[string]string{"a": settingsActAdvArea, "k": string(area.Area)}); ok {
				row = append(row, btn)
			}
		}
		k.row(row...)
	}
	k.navRow(false)
	return k.screen(settingsCompose(v, "settings_page_advanced", body, notice, false))
}

// renderSettingsAdvArea draws one area screen (aa): intro, every threshold with its current value,
// default, and allowed range, item buttons, Reset area button, and Back to Advanced + Close.
func renderSettingsAdvArea(v settingsView, area settingsAdvAreaDef, th advthresholds.Thresholds, notice string) settingsScreen {
	items := settingsAdvItemsForArea(area.Area)
	body := make([]string, 0, len(items)+2)
	body = append(body, settingsText(v.Tr, area.IntroKey), "")
	for _, item := range items {
		spec, _ := advthresholds.SpecFor(item.Field)
		effective := settingsAdvEffectiveValue(th, item, spec)
		body = append(body, settingsText(v.Tr, "settings_adv_item_line", i18n.TranslationParams{
			"name":    settingsText(v.Tr, item.NameKey),
			"value":   settingsAdvFormatValue(v.Tr, item.Unit, effective),
			"default": settingsAdvFormatValue(v.Tr, item.Unit, spec.Default),
			"range":   settingsAdvFormatRange(v.Tr, item.Unit, spec.Min, spec.Max),
		}))
	}

	k := newSettingsKeyboard(v)
	for _, item := range items {
		if btn, ok := k.button(settingsText(v.Tr, item.NameKey),
			map[string]string{"a": settingsActAdvItem, "t": string(item.Field)}); ok {
			k.row(btn)
		}
	}
	if resetBtn, ok := k.button(settingsText(v.Tr, "settings_btn_adv_reset_area"),
		map[string]string{"a": settingsActAdvResetArea, "k": string(area.Area)}); ok {
		k.row(resetBtn)
	}
	var nav []gotgbot.InlineKeyboardButton
	if back, ok := k.button(settingsText(v.Tr, "settings_btn_back"),
		map[string]string{"a": settingsActOpen, "p": settingsPageAdvanced}); ok {
		nav = append(nav, back)
	}
	if closeBtn, ok := k.button(settingsText(v.Tr, "settings_btn_close"),
		map[string]string{"a": settingsActClose}); ok {
		nav = append(nav, closeBtn)
	}
	k.row(nav...)
	return k.screen(settingsCompose(v, area.NameKey, body, notice, false))
}

// renderSettingsAdvItem draws one threshold screen (ao): description, current/default/range detail,
// curated preset buttons (active value prefixed with settingsSelectedPrefix), Reset to default,
// and Back to Area + Close.
func renderSettingsAdvItem(v settingsView, item settingsAdvItemDef, th advthresholds.Thresholds, notice string) settingsScreen {
	spec, _ := advthresholds.SpecFor(item.Field)
	effective := settingsAdvEffectiveValue(th, item, spec)
	body := []string{
		settingsText(v.Tr, item.DescKey),
		"",
		settingsText(v.Tr, "settings_adv_item_detail", i18n.TranslationParams{
			"value":   settingsAdvFormatValue(v.Tr, item.Unit, effective),
			"default": settingsAdvFormatValue(v.Tr, item.Unit, spec.Default),
			"range":   settingsAdvFormatRange(v.Tr, item.Unit, spec.Min, spec.Max),
		}),
	}

	k := newSettingsKeyboard(v)
	for i := 0; i < len(item.Presets); i += 3 {
		var row []gotgbot.InlineKeyboardButton
		for _, preset := range item.Presets[i:min(i+3, len(item.Presets))] {
			label := settingsAdvFormatValue(v.Tr, item.Unit, preset)
			if effective > 0 && preset == effective {
				label = settingsSelectedPrefix + label
			}
			if btn, ok := k.button(label, map[string]string{
				"a": settingsActAdvSet,
				"t": string(item.Field),
				"v": strconv.Itoa(preset),
			}); ok {
				row = append(row, btn)
			}
		}
		k.row(row...)
	}
	if resetBtn, ok := k.button(settingsText(v.Tr, "settings_btn_adv_reset_item"),
		map[string]string{"a": settingsActAdvReset, "t": string(item.Field)}); ok {
		k.row(resetBtn)
	}
	var nav []gotgbot.InlineKeyboardButton
	if back, ok := k.button(settingsText(v.Tr, "settings_btn_back"),
		map[string]string{"a": settingsActAdvArea, "k": string(item.Area)}); ok {
		nav = append(nav, back)
	}
	if closeBtn, ok := k.button(settingsText(v.Tr, "settings_btn_close"),
		map[string]string{"a": settingsActClose}); ok {
		nav = append(nav, closeBtn)
	}
	k.row(nav...)
	return k.screen(settingsCompose(v, item.NameKey, body, notice, false))
}

func settingsAdvNotice(tr *i18n.Translator, outcome advThresholdOutcome, savedKey string) string {
	switch outcome {
	case advThresholdSaved:
		return settingsNoticeOK(settingsText(tr, savedKey))
	case advThresholdInvalid:
		return settingsNoticeWarn(settingsText(tr, "common_callback_invalid_request"))
	default:
		return settingsNoticeWarn(settingsText(tr, "common_settings_save_failed"))
	}
}

func settingsAdvShowArea(p *settingsPress, area settingsAdvAreaDef, notice string) *settingsScreen {
	th, err := settingsAdvReadNow(p.group.Id)
	if err != nil {
		log.Warnf("[Settings] advanced thresholds of chat %d could not be read: %v", p.group.Id, err)
		p.cannotOpen()
		return nil
	}
	screen := renderSettingsAdvArea(p.view(), area, th, notice)
	return &screen
}

func settingsAdvShowItem(p *settingsPress, item settingsAdvItemDef, notice string) *settingsScreen {
	th, err := settingsAdvReadNow(p.group.Id)
	if err != nil {
		log.Warnf("[Settings] advanced thresholds of chat %d could not be read: %v", p.group.Id, err)
		p.cannotOpen()
		return nil
	}
	screen := renderSettingsAdvItem(p.view(), item, th, notice)
	return &screen
}

// settingsAdvRunArea serves aa: opens one of the 6 area screens.
func settingsAdvRunArea(p *settingsPress) *settingsScreen {
	area, ok := settingsAdvAreaByCode(p.fields["k"])
	if !ok {
		p.answer(settingsText(p.tr, "common_callback_invalid_request"), true)
		return nil
	}
	return settingsAdvShowArea(p, area, "")
}

// settingsAdvRunItem serves ao: opens one of the 12 threshold screens.
func settingsAdvRunItem(p *settingsPress) *settingsScreen {
	item, ok := settingsAdvItemByCode(p.fields["t"])
	if !ok {
		p.answer(settingsText(p.tr, "common_callback_invalid_request"), true)
		return nil
	}
	return settingsAdvShowItem(p, item, "")
}

// settingsAdvRunSet serves as: saves a curated preset value and redraws the item screen.
func settingsAdvRunSet(p *settingsPress) *settingsScreen {
	item, ok := settingsAdvItemByCode(p.fields["t"])
	if !ok {
		p.answer(settingsText(p.tr, "common_callback_invalid_request"), true)
		return nil
	}
	val, valid := settingsAdvPresetValue(item, p.fields["v"])
	if !valid {
		p.answer(settingsText(p.tr, "common_callback_invalid_request"), true)
		return nil
	}
	ctx, cancel := settingsAdvContext()
	defer cancel()
	outcome := applyAdvancedThreshold(ctx, p.group.Id, item.Field, val, p.query.From.Id)
	return settingsAdvShowItem(p, item, settingsAdvNotice(p.tr, outcome, "settings_adv_saved"))
}

// settingsAdvRunReset serves ar: resets one threshold to default and redraws the item screen.
func settingsAdvRunReset(p *settingsPress) *settingsScreen {
	item, ok := settingsAdvItemByCode(p.fields["t"])
	if !ok {
		p.answer(settingsText(p.tr, "common_callback_invalid_request"), true)
		return nil
	}
	ctx, cancel := settingsAdvContext()
	defer cancel()
	outcome := resetAdvancedThreshold(ctx, p.group.Id, item.Field, p.query.From.Id)
	return settingsAdvShowItem(p, item, settingsAdvNotice(p.tr, outcome, "settings_adv_reset_item_done"))
}

// settingsAdvRunResetArea serves ax: resets all thresholds in an area to default and redraws the area screen.
func settingsAdvRunResetArea(p *settingsPress) *settingsScreen {
	area, ok := settingsAdvAreaByCode(p.fields["k"])
	if !ok {
		p.answer(settingsText(p.tr, "common_callback_invalid_request"), true)
		return nil
	}
	ctx, cancel := settingsAdvContext()
	defer cancel()
	outcome := resetAdvancedArea(ctx, p.group.Id, area.Area, p.query.From.Id)
	return settingsAdvShowArea(p, area, settingsAdvNotice(p.tr, outcome, "settings_adv_reset_area_done"))
}
