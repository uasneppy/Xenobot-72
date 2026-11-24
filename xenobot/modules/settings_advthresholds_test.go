//go:build testtools

package modules

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"unicode/utf8"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/advthresholds"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
)

// newPhase28SettingsEnv builds a settingsEnv backed by phase28InProcessMiniredis so every
// Advanced settings page test runs under -race with zero localhost TCP dials.
func newPhase28SettingsEnv(t *testing.T) *settingsEnv {
	t.Helper()
	setCachedBotUsername(t, "XenobotTestBot")
	base := newPhase28LockdownEnv(t)
	env := &settingsEnv{lockdownEnv: base}
	env.fake.setCreator(env.chat.Id, env.admin.Id)

	LoadSettings(env.dispatcher)
	LoadCaptcha(env.dispatcher)
	LoadAntiflood(env.dispatcher)
	LoadStaff(env.dispatcher)
	LoadHelp(env.dispatcher)
	LoadBotUpdates(env.dispatcher)

	settingsCleanup(t, env.chat.Id)
	cleanupAdvThresholdsChat(t, env.chat.Id)
	env.setAdminsAllowed(env.chat.Id, true)
	return env
}

func settingsAdvStored(t *testing.T, chatID int64) advthresholds.Thresholds {
	t.Helper()
	th, err := advthresholds.GetFresh(context.Background(), chatID)
	if err != nil {
		t.Fatalf("advthresholds.GetFresh(%d): %v", chatID, err)
	}
	return th
}

func settingsAdvCountRows(t *testing.T, chatID int64) int64 {
	t.Helper()
	var n int64
	if err := db.DB.Model(&models.ChatAdvancedThresholds{}).Where("chat_id = ?", chatID).Count(&n).Error; err != nil {
		t.Fatalf("count chat_advanced_thresholds: %v", err)
	}
	return n
}

func settingsAdvFindButton(t *testing.T, keyboard [][]gotgbot.InlineKeyboardButton, want map[string]string) (gotgbot.InlineKeyboardButton, bool) {
	t.Helper()
	for _, row := range keyboard {
		for _, btn := range row {
			if !strings.HasPrefix(btn.CallbackData, settingsCallbackNamespace+"|") {
				continue
			}
			fields := settingsButtonFields(t, btn)
			match := true
			for k, v := range want {
				if fields[k] != v {
					match = false
					break
				}
			}
			if match {
				return btn, true
			}
		}
	}
	return gotgbot.InlineKeyboardButton{}, false
}

func TestSettingsAdvancedThresholds(t *testing.T) {
	t.Run("hub_area_and_item_display", func(t *testing.T) {
		env := newPhase28SettingsEnv(t)
		owner := env.newOwner(env.chat)

		// Group menu shows Private only with 0 settingsAdvRead calls.
		var reads atomic.Int32
		origRead := settingsAdvRead
		settingsAdvRead = func(ctx context.Context, chatID int64) (advthresholds.Thresholds, error) {
			reads.Add(1)
			return origRead(ctx, chatID)
		}
		t.Cleanup(func() { settingsAdvRead = origRead })

		groupMsg := env.openMenu(owner)
		groupMain, _ := env.lastEdit(env.chat.Id, groupMsg)
		if line, ok := settingsMainStateLine(groupMain, "settings_page_advanced"); !ok || !strings.Contains(line, staffMarker("settings_state_private_only")) {
			t.Errorf("group Main line = %q (ok=%v), want settings_state_private_only", line, ok)
		}
		if reads.Load() != 0 {
			t.Errorf("group Main called settingsAdvRead %d times, want 0", reads.Load())
		}

		// Private Main shows Default, opening av lists all 6 areas and creates 0 DB rows.
		privMsg := env.updateID()
		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActBack))
		privMain, _ := env.lastEdit(owner.Id, privMsg)
		if line, ok := settingsMainStateLine(privMain, "settings_page_advanced"); !ok || !strings.Contains(line, staffMarker("settings_state_default")) {
			t.Errorf("private Main line = %q (ok=%v), want settings_state_default", line, ok)
		}

		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActOpen, "p", settingsPageAdvanced))
		hubText, hubKB := env.lastEdit(owner.Id, privMsg)
		if !strings.Contains(hubText, staffMarker("settings_page_advanced")) || !strings.Contains(hubText, staffMarker("settings_adv_intro")) {
			t.Errorf("hub screen missing title/intro:\n%s", hubText)
		}
		for _, area := range settingsAdvAreas {
			if !strings.Contains(hubText, staffMarker(area.NameKey)) {
				t.Errorf("hub text missing area %s:\n%s", area.NameKey, hubText)
			}
			if _, ok := settingsAdvFindButton(t, hubKB, map[string]string{"a": settingsActAdvArea, "k": string(area.Area)}); !ok {
				t.Errorf("hub keyboard missing button for area %s", area.Area)
			}
		}

		// Open each of the 6 areas and verify every item displays current, default, and range plus Reset area.
		for _, area := range settingsAdvAreas {
			env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActAdvArea, "k", string(area.Area)))
			areaText, areaKB := env.lastEdit(owner.Id, privMsg)
			if !strings.Contains(areaText, staffMarker(area.NameKey)) || !strings.Contains(areaText, staffMarker(area.IntroKey)) {
				t.Errorf("area %s screen missing title/intro:\n%s", area.Area, areaText)
			}
			for _, item := range settingsAdvItemsForArea(area.Area) {
				spec, _ := advthresholds.SpecFor(item.Field)
				if !strings.Contains(areaText, staffMarker(item.NameKey)) {
					t.Errorf("area %s missing item %s:\n%s", area.Area, item.Field, areaText)
				}
				if !strings.Contains(areaText, staffMarker("settings_adv_item_line")) {
					t.Errorf("area %s missing item line marker:\n%s", area.Area, areaText)
				}
				// Verify range endpoints appear in the area text.
				if item.Unit == advUnitBar {
					wantMin := fmt.Sprintf("%.2f", float64(spec.Min)/100.0)
					wantMax := fmt.Sprintf("%.2f", float64(spec.Max)/100.0)
					if !strings.Contains(areaText, wantMin) || !strings.Contains(areaText, wantMax) {
						t.Errorf("area %s item %s missing range %s..%s:\n%s", area.Area, item.Field, wantMin, wantMax, areaText)
					}
				} else {
					if !strings.Contains(areaText, strconv.Itoa(spec.Min)) || !strings.Contains(areaText, strconv.Itoa(spec.Max)) {
						t.Errorf("area %s item %s missing range %d..%d:\n%s", area.Area, item.Field, spec.Min, spec.Max, areaText)
					}
				}
				if _, ok := settingsAdvFindButton(t, areaKB, map[string]string{"a": settingsActAdvItem, "t": string(item.Field)}); !ok {
					t.Errorf("area %s keyboard missing item button %s", area.Area, item.Field)
				}
			}
			if _, ok := settingsAdvFindButton(t, areaKB, map[string]string{"a": settingsActAdvResetArea, "k": string(area.Area)}); !ok {
				t.Errorf("area %s keyboard missing Reset area button", area.Area)
			}
		}

		// Open item jsw (default 60): verify description, detail, active ✅ preset, and Reset button.
		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActAdvItem, "t", string(advthresholds.FieldRaidJoinWindow)))
		itemText, itemKB := env.lastEdit(owner.Id, privMsg)
		for _, want := range []string{
			staffMarker("settings_adv_item_jsw"),
			staffMarker("settings_adv_desc_jsw"),
			staffMarker("settings_adv_item_detail"),
			"60",
			"15",
			"600",
		} {
			if !strings.Contains(itemText, want) {
				t.Errorf("jsw item screen missing %q:\n%s", want, itemText)
			}
		}
		btn60, ok := settingsAdvFindButton(t, itemKB, map[string]string{"a": settingsActAdvSet, "t": "jsw", "v": "60"})
		if !ok || !strings.HasPrefix(btn60.Text, settingsSelectedPrefix) {
			t.Errorf("jsw default preset button 60 = %+v (found=%v), want ✅ prefix", btn60, ok)
		}
		btn15, ok := settingsAdvFindButton(t, itemKB, map[string]string{"a": settingsActAdvSet, "t": "jsw", "v": "15"})
		if !ok || strings.HasPrefix(btn15.Text, settingsSelectedPrefix) {
			t.Errorf("jsw non-active preset button 15 = %+v (found=%v), want no ✅ prefix", btn15, ok)
		}
		if _, ok := settingsAdvFindButton(t, itemKB, map[string]string{"a": settingsActAdvReset, "t": "jsw"}); !ok {
			t.Errorf("jsw item screen missing Reset to default button")
		}

		// Open item ctm (unset -> Captcha page default, no preset selected).
		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActAdvItem, "t", string(advthresholds.FieldCaptchaTimeout)))
		ctmText, ctmKB := env.lastEdit(owner.Id, privMsg)
		if !strings.Contains(ctmText, staffMarker("settings_adv_captcha_default")) {
			t.Errorf("ctm item screen missing settings_adv_captcha_default:\n%s", ctmText)
		}
		for _, preset := range []string{"15", "20", "30", "45", "60"} {
			btn, found := settingsAdvFindButton(t, ctmKB, map[string]string{"a": settingsActAdvSet, "t": "ctm", "v": preset})
			if !found || strings.HasPrefix(btn.Text, settingsSelectedPrefix) {
				t.Errorf("unset ctm preset %s = %+v (found=%v), want unselected", preset, btn, found)
			}
		}

		if rows := settingsAdvCountRows(t, env.chat.Id); rows != 0 {
			t.Errorf("viewing Main, hub, all 6 areas, and items created %d DB rows, want 0", rows)
		}
	})

	t.Run("set_preset_default_normalization_and_resets", func(t *testing.T) {
		env := newPhase28SettingsEnv(t)
		owner := env.newOwner(env.chat)
		privMsg := env.updateID()

		// Set flw = 120 and jsw = 15.
		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActAdvSet, "t", "flw", "v", "120"))
		text, kb := env.lastEdit(owner.Id, privMsg)
		if !strings.Contains(text, staffMarker("settings_adv_saved")) {
			t.Errorf("saving flw=120 missing saved notice:\n%s", text)
		}
		if btn, ok := settingsAdvFindButton(t, kb, map[string]string{"a": settingsActAdvSet, "t": "flw", "v": "120"}); !ok || !strings.HasPrefix(btn.Text, settingsSelectedPrefix) {
			t.Errorf("flw=120 button = %+v (found=%v), want ✅ prefix", btn, ok)
		}

		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActAdvSet, "t", "jsw", "v", "15"))
		if th := settingsAdvStored(t, env.chat.Id); th.RaidFloodWindowSecs != 120 || th.RaidJoinWindowSecs != 15 || th.CustomizedCount() != 2 {
			t.Fatalf("after flw=120, jsw=15 stored = %+v", th)
		}

		// Main updates to 2 custom, hub Raid windows shows 2 custom.
		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActBack))
		mainText, _ := env.lastEdit(owner.Id, privMsg)
		if line, ok := settingsMainStateLine(mainText, "settings_page_advanced"); !ok || !strings.Contains(line, staffMarker("settings_state_adv_n")+" 2") {
			t.Errorf("Main line (2 custom) = %q (ok=%v), want settings_state_adv_n 2", line, ok)
		}
		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActOpen, "p", settingsPageAdvanced))
		hubText, _ := env.lastEdit(owner.Id, privMsg)
		if !strings.Contains(hubText, staffMarker("settings_adv_area_rw")) || !strings.Contains(hubText, staffMarker("settings_state_adv_n")+" 2") {
			t.Errorf("hub Raid windows line did not show 2 custom:\n%s", hubText)
		}

		// Selecting jsw's default preset (60) normalizes DB column to 0 (P28-01).
		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActAdvSet, "t", "jsw", "v", "60"))
		if th := settingsAdvStored(t, env.chat.Id); th.RaidJoinWindowSecs != 0 || th.RaidFloodWindowSecs != 120 || th.CustomizedCount() != 1 {
			t.Fatalf("selecting default preset jsw=60 stored = %+v, want jsw=0, flw=120", th)
		}

		// Item reset (ar) on flw sets it back to 0.
		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActAdvReset, "t", "flw"))
		text, _ = env.lastEdit(owner.Id, privMsg)
		if !strings.Contains(text, staffMarker("settings_adv_reset_item_done")) {
			t.Errorf("resetting flw missing done notice:\n%s", text)
		}
		if th := settingsAdvStored(t, env.chat.Id); th.RaidFloodWindowSecs != 0 || th.CustomizedCount() != 0 {
			t.Fatalf("after ar flw stored = %+v, want all 0", th)
		}

		// Set fnp=40, rna=6 (area nm) and lrm=15 (area lr), then reset area nm via ax: preserves lrm.
		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActAdvSet, "t", "fnp", "v", "40"))
		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActAdvSet, "t", "rna", "v", "6"))
		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActAdvSet, "t", "lrm", "v", "15"))
		if th := settingsAdvStored(t, env.chat.Id); th.RaidFloodNewPercent != 40 || th.RaidRecentHours != 6 || th.LockdownReminderMins != 15 {
			t.Fatalf("before area reset stored = %+v", th)
		}

		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActAdvResetArea, "k", "nm"))
		text, _ = env.lastEdit(owner.Id, privMsg)
		if !strings.Contains(text, staffMarker("settings_adv_reset_area_done")) {
			t.Errorf("resetting area nm missing done notice:\n%s", text)
		}
		if th := settingsAdvStored(t, env.chat.Id); th.RaidFloodNewPercent != 0 || th.RaidRecentHours != 0 || th.LockdownReminderMins != 15 {
			t.Fatalf("after ax nm stored = %+v, want nm cleared and lrm=15 preserved", th)
		}
	})

	t.Run("live_authority_and_owner_gate", func(t *testing.T) {
		env := newPhase28SettingsEnv(t)
		owner := env.newOwner(env.chat)
		admin := env.admin
		privMsg := env.updateID()

		// 1. Admin with can_restrict_members + owner switch on can view and write.
		env.setAdminsAllowed(env.chat.Id, true)
		env.fake.setMember(env.chat.Id, admin.Id, staffFakeMember{
			Status:             gotgbot.ChatMemberStatusAdministrator,
			CanRestrictMembers: true,
		})
		env.pressPrivate(admin, privMsg, env.privateFields(env.chat.Id, "a", settingsActOpen, "p", settingsPageAdvanced))
		if hubText, _ := env.lastEdit(admin.Id, privMsg); !strings.Contains(hubText, staffMarker("settings_adv_intro")) {
			t.Fatalf("restrict admin could not open av:\n%s", hubText)
		}
		env.pressPrivate(admin, privMsg, env.privateFields(env.chat.Id, "a", settingsActAdvSet, "t", "jsw", "v", "120"))
		if th := settingsAdvStored(t, env.chat.Id); th.RaidJoinWindowSecs != 120 {
			t.Fatalf("restrict admin write jsw=120 failed, stored = %+v", th)
		}

		// 2. Admin WITHOUT can_restrict_members can view av/aa/ao, but is refused on as/ar/ax with 0 DB writes.
		env.fake.setMember(env.chat.Id, admin.Id, staffFakeMember{
			Status:             gotgbot.ChatMemberStatusAdministrator,
			CanRestrictMembers: false,
		})
		env.pressPrivate(admin, privMsg, env.privateFields(env.chat.Id, "a", settingsActOpen, "p", settingsPageAdvanced))
		if text, _ := env.lastEdit(admin.Id, privMsg); !strings.Contains(text, staffMarker("settings_adv_intro")) {
			t.Errorf("non-restrict admin could not view av:\n%s", text)
		}
		env.pressPrivate(admin, privMsg, env.privateFields(env.chat.Id, "a", settingsActAdvArea, "k", "rw"))
		if text, _ := env.lastEdit(admin.Id, privMsg); !strings.Contains(text, staffMarker("settings_adv_area_intro_rw")) {
			t.Errorf("non-restrict admin could not view aa:\n%s", text)
		}
		env.pressPrivate(admin, privMsg, env.privateFields(env.chat.Id, "a", settingsActAdvItem, "t", "jsw"))
		if text, _ := env.lastEdit(admin.Id, privMsg); !strings.Contains(text, staffMarker("settings_adv_desc_jsw")) {
			t.Errorf("non-restrict admin could not view ao:\n%s", text)
		}

		for _, writeKV := range [][]string{
			{"a", settingsActAdvSet, "t", "jsw", "v", "300"},
			{"a", settingsActAdvReset, "t", "jsw"},
			{"a", settingsActAdvResetArea, "k", "rw"},
		} {
			env.pressPrivate(admin, privMsg, env.privateFields(env.chat.Id, writeKV...))
			if ans, alert := env.lastAnswer(); !alert || !strings.Contains(ans, staffMarker("settings_no_restrict")) {
				t.Errorf("non-restrict admin %v answered %q (alert %v), want settings_no_restrict", writeKV, ans, alert)
			}
		}
		if th := settingsAdvStored(t, env.chat.Id); th.RaidJoinWindowSecs != 120 {
			t.Errorf("non-restrict admin writes mutated DB: %+v", th)
		}

		// 3. Demoted admin is refused on view and write.
		env.fake.setMember(env.chat.Id, admin.Id, staffFakeMember{Status: gotgbot.ChatMemberStatusMember})
		for _, kv := range [][]string{
			{"a", settingsActOpen, "p", settingsPageAdvanced},
			{"a", settingsActAdvSet, "t", "jsw", "v", "300"},
		} {
			env.pressPrivate(admin, privMsg, env.privateFields(env.chat.Id, kv...))
			if ans, alert := env.lastAnswer(); !alert || !strings.Contains(ans, staffMarker("settings_not_admin")) {
				t.Errorf("demoted admin %v answered %q (alert %v), want settings_not_admin", kv, ans, alert)
			}
		}

		// 4. Admin when owner switch is off is refused on view and write.
		env.setAdminsAllowed(env.chat.Id, false)
		env.fake.setMember(env.chat.Id, admin.Id, staffFakeMember{
			Status:             gotgbot.ChatMemberStatusAdministrator,
			CanRestrictMembers: true,
		})
		for _, kv := range [][]string{
			{"a", settingsActOpen, "p", settingsPageAdvanced},
			{"a", settingsActAdvArea, "k", "rw"},
			{"a", settingsActAdvSet, "t", "jsw", "v", "300"},
		} {
			env.pressPrivate(admin, privMsg, env.privateFields(env.chat.Id, kv...))
			if ans, alert := env.lastAnswer(); !alert || !strings.Contains(ans, staffMarker("settings_gate_owner_only")) {
				t.Errorf("owner-gated admin %v answered %q (alert %v), want settings_gate_owner_only", kv, ans, alert)
			}
		}
		if th := settingsAdvStored(t, env.chat.Id); th.RaidJoinWindowSecs != 120 {
			t.Errorf("refused admin presses mutated DB: %+v", th)
		}

		// 5. Group-menu press of op p=av shows settings_adv_group_note with 0 DB reads.
		var reads atomic.Int32
		origRead := settingsAdvRead
		settingsAdvRead = func(ctx context.Context, chatID int64) (advthresholds.Thresholds, error) {
			reads.Add(1)
			return origRead(ctx, chatID)
		}
		t.Cleanup(func() { settingsAdvRead = origRead })

		groupMsg := env.openMenu(owner)
		env.press(owner, groupMsg, env.groupFields(owner, "a", settingsActOpen, "p", settingsPageAdvanced))
		groupPage, _ := env.lastEdit(env.chat.Id, groupMsg)
		if !strings.Contains(groupPage, staffMarker("settings_adv_group_note")) {
			t.Errorf("group menu op p=av drew:\n%s\nwant settings_adv_group_note", groupPage)
		}
		if reads.Load() != 0 {
			t.Errorf("group menu op p=av called settingsAdvRead %d times, want 0", reads.Load())
		}
	})

	t.Run("forged_and_out_of_range_presses_refused_in_step1", func(t *testing.T) {
		env := newPhase28SettingsEnv(t)
		owner := env.newOwner(env.chat)
		privMsg := env.updateID()
		groupMsg := env.openMenu(owner)

		var reads atomic.Int32
		origRead := settingsAdvRead
		settingsAdvRead = func(ctx context.Context, chatID int64) (advthresholds.Thresholds, error) {
			reads.Add(1)
			return origRead(ctx, chatID)
		}
		t.Cleanup(func() { settingsAdvRead = origRead })

		beforeMembers := len(callsToChat(env.fake.staffBotClient, "getChatMember", env.chat.Id))

		forgedPrivate := [][]string{
			// Out-of-range, non-canonical, non-numeric, and non-preset values on as.
			{"a", settingsActAdvSet, "t", "jsw", "v", "5"},
			{"a", settingsActAdvSet, "t", "jsw", "v", "9999"},
			{"a", settingsActAdvSet, "t", "jsw", "v", "-1"},
			{"a", settingsActAdvSet, "t", "jsw", "v", "abc"},
			{"a", settingsActAdvSet, "t", "jsw", "v", "0"},
			{"a", settingsActAdvSet, "t", "jsw", "v", "060"},
			{"a", settingsActAdvSet, "t", "jsw", "v", "61"},
			{"a", settingsActAdvSet, "t", "ctm", "v", "10"},
			// Unknown area / item codes.
			{"a", settingsActAdvArea, "k", "zz"},
			{"a", settingsActAdvResetArea, "k", "zz"},
			{"a", settingsActAdvItem, "t", "zzz"},
			{"a", settingsActAdvSet, "t", "zzz", "v", "60"},
			{"a", settingsActAdvReset, "t", "zzz"},
			// Missing required fields.
			{"a", settingsActAdvArea},
			{"a", settingsActAdvItem},
			{"a", settingsActAdvSet, "t", "jsw"},
			{"a", settingsActAdvReset},
			{"a", settingsActAdvResetArea},
			// Extra key x=1 on every action code.
			{"a", settingsActAdvArea, "k", "rw", "x", "1"},
			{"a", settingsActAdvItem, "t", "jsw", "x", "1"},
			{"a", settingsActAdvSet, "t", "jsw", "v", "60", "x", "1"},
			{"a", settingsActAdvReset, "t", "jsw", "x", "1"},
			{"a", settingsActAdvResetArea, "k", "rw", "x", "1"},
		}
		for _, kv := range forgedPrivate {
			env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, kv...))
			if ans, alert := env.lastAnswer(); !alert || !strings.Contains(ans, staffMarker("common_callback_invalid_request")) {
				t.Errorf("forged private %v answered %q (alert %v), want invalid request alert", kv, ans, alert)
			}
		}

		// Group-menu presses (u instead of c) on all 5 action codes must fail in step 1.
		forgedGroup := [][]string{
			{"a", settingsActAdvArea, "k", "rw"},
			{"a", settingsActAdvItem, "t", "jsw"},
			{"a", settingsActAdvSet, "t", "jsw", "v", "120"},
			{"a", settingsActAdvReset, "t", "jsw"},
			{"a", settingsActAdvResetArea, "k", "rw"},
		}
		for _, kv := range forgedGroup {
			env.press(owner, groupMsg, env.groupFields(owner, kv...))
			if ans, alert := env.lastAnswer(); !alert || !strings.Contains(ans, staffMarker("common_callback_invalid_request")) {
				t.Errorf("forged group %v answered %q (alert %v), want invalid request alert", kv, ans, alert)
			}
		}

		afterMembers := len(callsToChat(env.fake.staffBotClient, "getChatMember", env.chat.Id))
		if afterMembers != beforeMembers {
			t.Errorf("forged/group presses made %d getChatMember calls, want 0", afterMembers-beforeMembers)
		}
		if reads.Load() != 0 {
			t.Errorf("forged/group presses made %d settingsAdvRead calls, want 0", reads.Load())
		}
		if rows := settingsAdvCountRows(t, env.chat.Id); rows != 0 {
			t.Errorf("forged/group presses created %d DB rows, want 0", rows)
		}
	})

	t.Run("worst_case_callbacks_and_8_locales", func(t *testing.T) {
		withRealLocales(t)
		trEn := i18n.MustNewTranslator("en")
		worstView := settingsView{
			Tr: trEn, Private: true, OwnerKey: "c", OwnerValue: "-1009999999999",
			GroupID: -1009999999999, GroupTitle: "G", OpenerName: "G",
		}
		allCustom := advthresholds.Thresholds{
			RaidJoinWindowSecs:   600,
			RaidFloodWindowSecs:  300,
			RaidAIWindowSecs:     900,
			RaidFloodNewPercent:  100,
			RaidFloodMinAccounts: 20,
			RaidRecentHours:      168,
			RaidAIImageBarPct:    95,
			MemberReqAIBarPct:    98,
			MemberReqCapPerMin:   100,
			MemberReqPauseMins:   60,
			LockdownReminderMins: 1440,
			CaptchaTimeoutMins:   60,
		}

		// Measure every callback button on hub, all 6 areas, and all 12 items with c=-1009999999999.
		screens := map[string]settingsScreen{
			"hub": renderSettingsAdvHub(worstView, allCustom, ""),
		}
		for _, area := range settingsAdvAreas {
			screens["area_"+string(area.Area)] = renderSettingsAdvArea(worstView, area, allCustom, "")
		}
		for _, item := range settingsAdvItems {
			screens["item_"+string(item.Field)] = renderSettingsAdvItem(worstView, item, allCustom, "")
		}
		largest := 0
		for name, screen := range screens {
			if screen.Omitted != 0 {
				t.Errorf("%s omitted %d buttons, want 0", name, screen.Omitted)
			}
			for _, row := range screen.Keyboard.InlineKeyboard {
				for _, btn := range row {
					largest = max(largest, len(btn.CallbackData))
					if len(btn.CallbackData) > 64 {
						t.Errorf("%s: button %q callback %q is %d bytes (> 64)", name, btn.Text, btn.CallbackData, len(btn.CallbackData))
					}
				}
			}
		}
		if largest == 0 {
			t.Fatal("no callback buttons measured")
		}

		// Render hub, all 6 areas, and representative items across all 8 real locales.
		sampleFields := []advthresholds.Field{
			advthresholds.FieldRaidJoinWindow,
			advthresholds.FieldRaidFloodMinAccounts,
			advthresholds.FieldRaidAIImageBar,
			advthresholds.FieldMemberReqCap,
			advthresholds.FieldLockdownReminder,
			advthresholds.FieldCaptchaTimeout,
		}
		longTitle := strings.Repeat("Ж", 64)
		for _, lang := range []string{"uk", "en", "ru", "es", "fr", "pt", "id", "hi"} {
			tr := i18n.MustNewTranslator(lang)
			v := settingsView{
				Tr: tr, Private: true, OwnerKey: "c", OwnerValue: "-1009999999999",
				GroupID: -1009999999999, GroupTitle: longTitle, OpenerName: longTitle,
			}
			localeScreens := map[string]settingsScreen{
				"hub_default": renderSettingsAdvHub(v, advthresholds.Thresholds{}, ""),
				"hub_custom":  renderSettingsAdvHub(v, allCustom, settingsNoticeOK(settingsText(tr, "settings_adv_saved"))),
			}
			for _, area := range settingsAdvAreas {
				localeScreens["area_"+string(area.Area)] = renderSettingsAdvArea(v, area, allCustom, settingsNoticeOK(settingsText(tr, "settings_adv_reset_area_done")))
			}
			for _, field := range sampleFields {
				item, _ := settingsAdvItemByCode(string(field))
				localeScreens["item_"+string(field)+"_def"] = renderSettingsAdvItem(v, item, advthresholds.Thresholds{}, "")
				localeScreens["item_"+string(field)+"_set"] = renderSettingsAdvItem(v, item, allCustom, settingsNoticeOK(settingsText(tr, "settings_adv_saved")))
			}
			for name, screen := range localeScreens {
				label := fmt.Sprintf("%s %s", lang, name)
				if runes := utf8.RuneCountInString(screen.Text); runes == 0 || runes > 4096 {
					t.Errorf("%s text length = %d runes, want 1..4096", label, runes)
				}
				if units := panelUTF16Len(screen.Text); units == 0 || units > 3800 {
					t.Errorf("%s text length = %d UTF-16 units, want 1..3800", label, units)
				}
				if screen.Omitted != 0 {
					t.Errorf("%s omitted %d buttons, want 0", label, screen.Omitted)
				}
				for _, row := range screen.Keyboard.InlineKeyboard {
					for _, btn := range row {
						runes := utf8.RuneCountInString(btn.Text)
						if runes == 0 {
							t.Errorf("%s has empty button label", label)
						}
						limit := 40
						if len(row) > 1 {
							limit = 22
						}
						if runes > limit {
							t.Errorf("%s: button %q has %d runes (limit %d)", label, btn.Text, runes, limit)
						}
					}
				}
			}
		}
	})

	t.Run("staff_group_hides_advanced_page", func(t *testing.T) {
		env := newPhase28SettingsEnv(t)
		owner := env.newOwner(env.chat)
		origStaff := settingsAdvStaffLookup
		settingsAdvStaffLookup = func(chatID int64) (*models.StaffGroup, error) {
			if chatID == env.chat.Id {
				return &models.StaffGroup{ChatID: chatID}, nil
			}
			return nil, nil
		}
		t.Cleanup(func() { settingsAdvStaffLookup = origStaff })

		// Private Main hides av.
		privMsg := env.updateID()
		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActBack))
		staffMain, staffKB := env.lastEdit(owner.Id, privMsg)
		if _, ok := settingsMainStateLine(staffMain, "settings_page_advanced"); ok {
			t.Errorf("Staff Group private Main still listed settings_page_advanced:\n%s", staffMain)
		}
		if _, found := settingsAdvFindButton(t, staffKB, map[string]string{"a": settingsActOpen, "p": settingsPageAdvanced}); found {
			t.Errorf("Staff Group private Main still carried button for page av")
		}

		// Group Main hides av.
		groupMsg := env.openMenu(owner)
		groupMain, groupKB := env.lastEdit(env.chat.Id, groupMsg)
		if _, ok := settingsMainStateLine(groupMain, "settings_page_advanced"); ok {
			t.Errorf("Staff Group group Main still listed settings_page_advanced:\n%s", groupMain)
		}
		if _, found := settingsAdvFindButton(t, groupKB, map[string]string{"a": settingsActOpen, "p": settingsPageAdvanced}); found {
			t.Errorf("Staff Group group Main still carried button for page av")
		}

		// Direct op p=av and actions are refused with 0 DB writes.
		for _, kv := range [][]string{
			{"a", settingsActOpen, "p", settingsPageAdvanced},
			{"a", settingsActAdvArea, "k", "rw"},
			{"a", settingsActAdvSet, "t", "jsw", "v", "120"},
		} {
			env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, kv...))
			if ans, alert := env.lastAnswer(); !alert || !strings.Contains(ans, staffMarker("common_callback_invalid_request")) {
				t.Errorf("Staff Group %v answered %q (alert %v), want invalid request alert", kv, ans, alert)
			}
		}
		if rows := settingsAdvCountRows(t, env.chat.Id); rows != 0 {
			t.Errorf("Staff Group presses created %d DB rows, want 0", rows)
		}
	})
}
