//go:build testtools

package modules

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	dbcache "github.com/uasneppy/Xenobot-72/xenobot/db/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/db/modtuning"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
)

func modDefaultsCleanup(t *testing.T, chatID int64) {
	t.Helper()
	t.Cleanup(func() {
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatManualDefaults{})
		dbcache.DeleteCache(modtuning.DefaultsCacheKey(chatID))
	})
}

func newSettingsModDefaultsEnv(t *testing.T) *settingsListEnv {
	t.Helper()
	setCachedBotUsername(t, "XenobotTestBot")
	env := newSettingsListEnv(t)
	modDefaultsCleanup(t, env.chat.Id)
	return env
}

func settingsModDefaultsStored(t *testing.T, chatID int64) modtuning.Defaults {
	t.Helper()
	defs, err := modtuning.GetDefaultsFresh(context.Background(), chatID)
	if err != nil {
		t.Fatalf("GetDefaultsFresh(%d): %v", chatID, err)
	}
	return defs
}

func settingsModDefaultsFindButton(t *testing.T, keyboard [][]gotgbot.InlineKeyboardButton, want map[string]string) (gotgbot.InlineKeyboardButton, bool) {
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

func TestSettingsModDefaults(t *testing.T) {
	t.Run("visibility_private_group_and_staff_group", func(t *testing.T) {
		env := newSettingsModDefaultsEnv(t)
		owner := settingsFiltersOwner(env, env.chat)

		// Private chat opens page q without creating a DB row.
		_, text, keyboard := settingsListOpenPrivate(env, owner, settingsPageModDefaults)
		for _, want := range []string{
			staffMarker("settings_page_moddefaults"),
			staffMarker("settings_moddef_intro"),
			staffMarker("settings_moddef_line_ban"),
			staffMarker("settings_moddef_line_mute"),
			staffMarker("settings_moddef_line_reason"),
			staffMarker("settings_moddef_line_appeal"),
			staffMarker("settings_moddef_permanent"),
			staffMarker("settings_moddef_appeal_none"),
		} {
			if !strings.Contains(text, want) {
				t.Errorf("page q lacks %q:\n%s", want, text)
			}
		}
		if _, found := settingsModDefaultsFindButton(t, keyboard, map[string]string{"a": settingsActModDefAppealRemove}); found {
			t.Errorf("unset appeal link unexpectedly showed Remove button")
		}
		var rows int64
		db.DB.Model(&models.ChatManualDefaults{}).Where("chat_id = ?", env.chat.Id).Count(&rows)
		if rows != 0 {
			t.Errorf("opening page q created %d DB rows, want 0", rows)
		}

		// Main state shows Default when nothing is set, then count when active settings exist.
		msgID := env.updateID()
		env.pressPrivate(owner, msgID, env.privateFields(env.chat.Id, "a", settingsActBack))
		mainText, _ := env.lastEdit(owner.Id, msgID)
		if line, ok := settingsMainStateLine(mainText, "settings_page_moddefaults"); !ok || !strings.Contains(line, staffMarker("settings_state_default")) {
			t.Errorf("Main line (unset) = %q (ok=%v), want settings_state_default", line, ok)
		}

		if outcome := applyDefaultBanSecs(context.Background(), env.chat.Id, 3600, owner.Id); outcome != modTuningSaved {
			t.Fatalf("applyDefaultBanSecs: %v", outcome)
		}
		if outcome := applyReasonRequired(context.Background(), env.chat.Id, true, owner.Id); outcome != modTuningSaved {
			t.Fatalf("applyReasonRequired: %v", outcome)
		}
		env.pressPrivate(owner, msgID, env.privateFields(env.chat.Id, "a", settingsActBack))
		mainText, _ = env.lastEdit(owner.Id, msgID)
		if line, ok := settingsMainStateLine(mainText, "settings_page_moddefaults"); !ok || !strings.Contains(line, staffMarker("settings_state_moddef_n")+" 2") {
			t.Errorf("Main line (2 set) = %q (ok=%v), want settings_state_moddef_n 2", line, ok)
		}

		// Group menu shows settings_moddef_group_note without calling settingsModDefaultsRead.
		var reads atomic.Int32
		origRead := settingsModDefaultsRead
		settingsModDefaultsRead = func(ctx context.Context, chatID int64) (modtuning.Defaults, error) {
			reads.Add(1)
			return origRead(ctx, chatID)
		}
		t.Cleanup(func() { settingsModDefaultsRead = origRead })

		groupMsg := env.openMenu(owner)
		groupMain, _ := env.lastEdit(env.chat.Id, groupMsg)
		if line, ok := settingsMainStateLine(groupMain, "settings_page_moddefaults"); !ok || !strings.Contains(line, staffMarker("settings_state_private_only")) {
			t.Errorf("group Main line = %q (ok=%v), want settings_state_private_only", line, ok)
		}
		env.press(owner, groupMsg, env.groupFields(owner, "a", settingsActOpen, "p", settingsPageModDefaults))
		groupPage, _ := env.lastEdit(env.chat.Id, groupMsg)
		if !strings.Contains(groupPage, staffMarker("settings_moddef_group_note")) {
			t.Errorf("group page q drew:\n%s\nwant settings_moddef_group_note", groupPage)
		}
		if reads.Load() != 0 {
			t.Errorf("group menu called settingsModDefaultsRead %d times, want 0", reads.Load())
		}

		// Staff Group hides page q on Main and refuses op&p=q.
		origStaff := settingsModDefaultsStaffLookup
		settingsModDefaultsStaffLookup = func(chatID int64) (*models.StaffGroup, error) {
			if chatID == env.chat.Id {
				return &models.StaffGroup{ChatID: chatID}, nil
			}
			return nil, nil
		}
		t.Cleanup(func() { settingsModDefaultsStaffLookup = origStaff })

		env.pressPrivate(owner, msgID, env.privateFields(env.chat.Id, "a", settingsActBack))
		staffMain, staffKB := env.lastEdit(owner.Id, msgID)
		if _, ok := settingsMainStateLine(staffMain, "settings_page_moddefaults"); ok {
			t.Errorf("Staff Group Main still listed settings_page_moddefaults:\n%s", staffMain)
		}
		if _, found := settingsModDefaultsFindButton(t, staffKB, map[string]string{"a": settingsActOpen, "p": settingsPageModDefaults}); found {
			t.Errorf("Staff Group Main still carried button for page q")
		}
		env.pressPrivate(owner, msgID, env.privateFields(env.chat.Id, "a", settingsActOpen, "p", settingsPageModDefaults))
		if ans, alert := env.lastAnswer(); !alert || !strings.Contains(ans, staffMarker("common_callback_invalid_request")) {
			t.Errorf("Staff Group op&p=q answered %q (alert %v), want invalid request alert", ans, alert)
		}
	})

	t.Run("duration_ban_and_mute_presets_and_off", func(t *testing.T) {
		env := newSettingsModDefaultsEnv(t)
		owner := settingsFiltersOwner(env, env.chat)
		m := settingsListOpenMenuPage(env, env.chat, owner, settingsPageModDefaults)

		// Open ban duration screen: Off/Permanent is selected, 8 presets present.
		m.press("a", settingsActModDefDur, "k", settingsModDefKindBan)
		text, kb := m.last()
		if !strings.Contains(text, staffMarker("settings_moddef_dur_intro_ban")) {
			t.Fatalf("ban duration screen lacks intro:\n%s", text)
		}
		offBtn, ok := settingsModDefaultsFindButton(t, kb, map[string]string{"a": settingsActModDefSetDur, "k": settingsModDefKindBan, "v": "0"})
		if !ok || !strings.HasPrefix(offBtn.Text, settingsSelectedPrefix) {
			t.Errorf("ban Off button = %+v (found=%v), want selected prefix", offBtn, ok)
		}

		// Select 3600 (1 hour) for ban.
		m.press("a", settingsActModDefSetDur, "k", settingsModDefKindBan, "v", "3600")
		text, _ = m.last()
		if !strings.Contains(text, staffMarker("settings_moddef_saved_dur")) {
			t.Errorf("after saving ban duration, redraw lacks saved notice:\n%s", text)
		}
		if defs := settingsModDefaultsStored(t, env.chat.Id); defs.DefaultBanSecs != 3600 || defs.DefaultMuteSecs != 0 {
			t.Fatalf("after ban=3600, stored = %+v", defs)
		}

		// Reopen ban duration screen: 3600 is selected, Off is not.
		m.press("a", settingsActModDefDur, "k", settingsModDefKindBan)
		_, kb = m.last()
		btn3600, ok := settingsModDefaultsFindButton(t, kb, map[string]string{"a": settingsActModDefSetDur, "k": settingsModDefKindBan, "v": "3600"})
		if !ok || !strings.HasPrefix(btn3600.Text, settingsSelectedPrefix) {
			t.Errorf("ban 3600 button = %+v (found=%v), want selected prefix", btn3600, ok)
		}
		offBtn, ok = settingsModDefaultsFindButton(t, kb, map[string]string{"a": settingsActModDefSetDur, "k": settingsModDefKindBan, "v": "0"})
		if !ok || strings.HasPrefix(offBtn.Text, settingsSelectedPrefix) {
			t.Errorf("ban Off button unexpectedly marked selected: %+v", offBtn)
		}

		// Set mute duration to 86400 (1 day): must NOT clobber default_ban_secs.
		m.press("a", settingsActModDefDur, "k", settingsModDefKindMute)
		text, _ = m.last()
		if !strings.Contains(text, staffMarker("settings_moddef_dur_intro_mute")) {
			t.Errorf("mute duration screen lacks intro:\n%s", text)
		}
		m.press("a", settingsActModDefSetDur, "k", settingsModDefKindMute, "v", "86400")
		if defs := settingsModDefaultsStored(t, env.chat.Id); defs.DefaultBanSecs != 3600 || defs.DefaultMuteSecs != 86400 {
			t.Fatalf("after mute=86400, stored = %+v, want ban=3600 mute=86400", defs)
		}

		// Reset ban to Permanent (0): mute must stay 86400.
		m.press("a", settingsActModDefSetDur, "k", settingsModDefKindBan, "v", "0")
		if defs := settingsModDefaultsStored(t, env.chat.Id); defs.DefaultBanSecs != 0 || defs.DefaultMuteSecs != 86400 {
			t.Fatalf("after ban=0, stored = %+v, want ban=0 mute=86400", defs)
		}

		// Reset mute to Permanent (0).
		m.press("a", settingsActModDefSetDur, "k", settingsModDefKindMute, "v", "0")
		if defs := settingsModDefaultsStored(t, env.chat.Id); defs.DefaultBanSecs != 0 || defs.DefaultMuteSecs != 0 {
			t.Fatalf("after mute=0, stored = %+v, want ban=0 mute=0", defs)
		}
	})

	t.Run("reason_required_toggle", func(t *testing.T) {
		env := newSettingsModDefaultsEnv(t)
		owner := settingsFiltersOwner(env, env.chat)
		m := settingsListOpenMenuPage(env, env.chat, owner, settingsPageModDefaults)

		m.press("a", settingsActModDefReason, "v", "1")
		text, _ := m.last()
		if !strings.Contains(text, staffMarker("settings_moddef_saved_reason")) {
			t.Errorf("toggle on redraw lacks saved notice:\n%s", text)
		}
		if defs := settingsModDefaultsStored(t, env.chat.Id); !defs.ReasonRequired {
			t.Fatalf("reason_required = false, want true")
		}

		m.press("a", settingsActModDefReason, "v", "0")
		if defs := settingsModDefaultsStored(t, env.chat.Id); defs.ReasonRequired {
			t.Fatalf("reason_required = true, want false")
		}
	})

	t.Run("appeal_link_typed_wait_refusal_save_and_remove", func(t *testing.T) {
		env := newSettingsModDefaultsEnv(t)
		owner := settingsFiltersOwner(env, env.chat)
		m := settingsListOpenMenuPage(env, env.chat, owner, settingsPageModDefaults)

		// Press Set appeal link -> opens prompt & creates Redis wait.
		m.press("a", settingsActModDefAppealPrompt)
		promptText, promptKB := m.last()
		for _, want := range []string{
			staffMarker("settings_moddef_appeal_wait"),
			staffMarker("settings_moddef_appeal_wait_rules"),
			staffMarker("settings_edit_wait_footer"),
		} {
			if !strings.Contains(promptText, want) {
				t.Errorf("appeal prompt lacks %q:\n%s", want, promptText)
			}
		}
		if _, found := settingsModDefaultsFindButton(t, promptKB, map[string]string{"a": settingsActModDefAppealCancel, "t": settingsModDefAppealPart}); !found {
			t.Errorf("appeal prompt lacks Cancel button")
		}
		if !env.redis.Exists(settingsInputKey(owner.Id)) {
			t.Fatalf("appeal prompt did not write Redis wait")
		}

		// Invalid URL refused and wait kept.
		sentBefore := len(env.fake.sentTo(owner.Id))
		settingsListDoorSay(env, owner, "javascript:alert(1)")
		replies := env.fake.sentTo(owner.Id)[sentBefore:]
		if len(replies) != 1 || !strings.Contains(sentText(replies[0]), staffMarker("settings_moddef_appeal_refuse_url")) {
			t.Errorf("invalid URL answered %+v, want settings_moddef_appeal_refuse_url", replies)
		}
		if !env.redis.Exists(settingsInputKey(owner.Id)) {
			t.Fatalf("invalid URL refusal cleared wait")
		}

		// Non-text photo (even with caption) refused and wait kept.
		sentBefore = len(env.fake.sentTo(owner.Id))
		sendPrivateMessage(env.settingsEnv, owner, gotgbot.Message{
			Photo:   []gotgbot.PhotoSize{{FileId: "photo1"}},
			Caption: "https://example.com/appeal",
		})
		replies = env.fake.sentTo(owner.Id)[sentBefore:]
		if len(replies) != 1 || !strings.Contains(sentText(replies[0]), staffMarker("settings_edit_refuse_need_text")) {
			t.Errorf("photo reply answered %+v, want settings_edit_refuse_need_text", replies)
		}
		if !env.redis.Exists(settingsInputKey(owner.Id)) {
			t.Fatalf("photo refusal cleared wait")
		}

		// Valid URL saved, wait cleared, refreshed page sent with escaped URL and Remove button.
		sentBefore = len(env.fake.sentTo(owner.Id))
		settingsListDoorSay(env, owner, "https://example.com/appeal?a=1&b=2")
		if defs := settingsModDefaultsStored(t, env.chat.Id); defs.AppealURL != "https://example.com/appeal?a=1&b=2" {
			t.Fatalf("stored AppealURL = %q, want https://example.com/appeal?a=1&b=2", defs.AppealURL)
		}
		if env.redis.Exists(settingsInputKey(owner.Id)) {
			t.Errorf("wait still exists after saving appeal URL")
		}
		replies = env.fake.sentTo(owner.Id)[sentBefore:]
		if len(replies) == 0 {
			t.Fatalf("no refreshed page sent after saving appeal URL")
		}
		savedText := sentText(replies[len(replies)-1])
		for _, want := range []string{
			staffMarker("settings_moddef_saved_appeal"),
			"https://example.com/appeal?a=1&amp;b=2",
		} {
			if !strings.Contains(savedText, want) {
				t.Errorf("saved page lacks %q:\n%s", want, savedText)
			}
		}

		// Reopen page q and confirm Remove button appears, then press Remove.
		m.press("a", settingsActOpen, "p", settingsPageModDefaults)
		_, pageKB := m.last()
		if _, found := settingsModDefaultsFindButton(t, pageKB, map[string]string{"a": settingsActModDefAppealRemove}); !found {
			t.Fatalf("page q lacks Remove button when appeal link is set")
		}
		m.press("a", settingsActModDefAppealRemove)
		text, pageKB := m.last()
		if !strings.Contains(text, staffMarker("settings_moddef_removed_appeal")) || !strings.Contains(text, staffMarker("settings_moddef_appeal_none")) {
			t.Errorf("after Remove, page drew:\n%s", text)
		}
		if defs := settingsModDefaultsStored(t, env.chat.Id); defs.AppealURL != "" {
			t.Fatalf("after Remove, stored AppealURL = %q, want empty", defs.AppealURL)
		}
		if _, found := settingsModDefaultsFindButton(t, pageKB, map[string]string{"a": settingsActModDefAppealRemove}); found {
			t.Errorf("Remove button still shown after appeal link was removed")
		}

		// Pressing Remove again when already empty safely redraws with no error.
		m.press("a", settingsActModDefAppealRemove)
		if defs := settingsModDefaultsStored(t, env.chat.Id); defs.AppealURL != "" {
			t.Errorf("second Remove changed AppealURL to %q", defs.AppealURL)
		}
	})

	t.Run("appeal_link_post_save_read_error_sends_notice_only", func(t *testing.T) {
		env := &settingsListEnv{settingsEnv: newPhase28SettingsEnv(t)}
		env.fake.setBotRole(env.chat.Id, staffRoleAdmin)
		modDefaultsCleanup(t, env.chat.Id)
		owner := settingsFiltersOwner(env, env.chat)
		if err := modtuning.SetDefaultBanSecs(context.Background(), env.chat.Id, 3600, owner.Id); err != nil {
			t.Fatalf("seed DefaultBanSecs: %v", err)
		}
		if err := modtuning.SetReasonRequired(context.Background(), env.chat.Id, true, owner.Id); err != nil {
			t.Fatalf("seed ReasonRequired: %v", err)
		}

		m := settingsListOpenMenuPage(env, env.chat, owner, settingsPageModDefaults)
		m.press("a", settingsActModDefAppealPrompt)

		prevRead := settingsModDefaultsRead
		settingsModDefaultsRead = func(context.Context, int64) (modtuning.Defaults, error) {
			return modtuning.Defaults{}, errors.New("read failed")
		}
		t.Cleanup(func() { settingsModDefaultsRead = prevRead })

		sentBefore := len(env.fake.sentTo(owner.Id))
		settingsListDoorSay(env, owner, "https://example.com/appeal-ok")
		replies := env.fake.sentTo(owner.Id)[sentBefore:]
		if len(replies) != 1 {
			t.Fatalf("post-save read error replies = %d, want 1", len(replies))
		}
		got := sentText(replies[0])
		if !strings.Contains(got, staffMarker("settings_moddef_saved_appeal")) {
			t.Errorf("notice-only reply lacks saved confirmation: %q", got)
		}
		if strings.Contains(got, staffMarker("settings_moddef_permanent")) || strings.Contains(got, staffMarker("settings_state_off")) {
			t.Errorf("post-save read error rendered fabricated zero defaults: %q", got)
		}
		if defs := settingsModDefaultsStored(t, env.chat.Id); defs.AppealURL != "https://example.com/appeal-ok" {
			t.Errorf("stored AppealURL = %q, want saved URL", defs.AppealURL)
		}
	})

	t.Run("appeal_link_cancel_command_and_expired_wait", func(t *testing.T) {
		env := newSettingsModDefaultsEnv(t)
		owner := settingsFiltersOwner(env, env.chat)
		m := settingsListOpenMenuPage(env, env.chat, owner, settingsPageModDefaults)

		// Cancel via button.
		m.press("a", settingsActModDefAppealPrompt)
		if !env.redis.Exists(settingsInputKey(owner.Id)) {
			t.Fatalf("wait was not created")
		}
		m.press("a", settingsActModDefAppealCancel, "t", settingsModDefAppealPart)
		text, _ := m.last()
		if !strings.Contains(text, staffMarker("settings_edit_cancelled")) {
			t.Errorf("Cancel redraw lacks settings_edit_cancelled:\n%s", text)
		}
		if env.redis.Exists(settingsInputKey(owner.Id)) {
			t.Errorf("wait still exists after Cancel button")
		}

		// Cancel via /cancel command.
		m.press("a", settingsActModDefAppealPrompt)
		if !env.redis.Exists(settingsInputKey(owner.Id)) {
			t.Fatalf("wait was not created for /cancel test")
		}
		settingsListDoorSay(env, owner, "/cancel")
		if env.redis.Exists(settingsInputKey(owner.Id)) {
			t.Errorf("wait still exists after /cancel")
		}

		// Expired wait marker replies with settings_moddef_appeal_wait_ended (NOT settings_tz_wait_ended_note).
		m.press("a", settingsActModDefAppealPrompt)
		env.redis.Del(settingsInputKey(owner.Id)) // simulate TTL expiry of wait hash while done marker remains
		sentBefore := len(env.fake.sentTo(owner.Id))
		settingsListDoorSay(env, owner, "https://example.com/late")
		replies := env.fake.sentTo(owner.Id)[sentBefore:]
		if len(replies) != 1 {
			t.Fatalf("expired wait replies = %d, want 1", len(replies))
		}
		got := sentText(replies[0])
		if !strings.Contains(got, staffMarker("settings_moddef_appeal_wait_ended")) {
			t.Errorf("expired wait reply = %q, want settings_moddef_appeal_wait_ended", got)
		}
		if strings.Contains(got, staffMarker("settings_tz_wait_ended_note")) {
			t.Errorf("expired appeal wait mistakenly used settings_tz_wait_ended_note: %q", got)
		}
		if defs := settingsModDefaultsStored(t, env.chat.Id); defs.AppealURL != "" {
			t.Errorf("expired wait saved AppealURL = %q", defs.AppealURL)
		}

		// Timezone wait-ended fallback still uses settings_tz_wait_ended_note.
		if err := env.redis.Set(settingsInputDoneKey(owner.Id), settingsInputMarkerValue(env.chat.Id, settingsPageTimezone, settingsTimezonePart)); err != nil {
			t.Fatalf("set the wait marker: %v", err)
		}
		sentBefore = len(env.fake.sentTo(owner.Id))
		settingsListDoorSay(env, owner, "Europe/Kyiv")
		replies = env.fake.sentTo(owner.Id)[sentBefore:]
		if len(replies) != 1 || !strings.Contains(sentText(replies[0]), staffMarker("settings_tz_wait_ended_note")) {
			t.Errorf("expired timezone wait reply = %+v, want settings_tz_wait_ended_note", replies)
		}
	})

	t.Run("live_authority_and_owner_gate", func(t *testing.T) {
		env := newSettingsModDefaultsEnv(t)
		settingsFiltersOwner(env, env.chat)
		admin := env.admin

		// Owner gate blocks admins when admins_allowed = false.
		env.setAdminsAllowed(env.chat.Id, false)
		env.fake.setMember(env.chat.Id, admin.Id, staffFakeMember{
			Status:             gotgbot.ChatMemberStatusAdministrator,
			CanRestrictMembers: true,
		})
		msgID := env.updateID()
		env.pressPrivate(admin, msgID, env.privateFields(env.chat.Id, "a", settingsActOpen, "p", settingsPageModDefaults))
		if ans, alert := env.lastAnswer(); !alert || !strings.Contains(ans, staffMarker("settings_gate_owner_only")) {
			t.Errorf("owner-gated admin open answered %q (alert %v), want settings_gate_owner_only", ans, alert)
		}

		// With admins_allowed = true, admin WITHOUT can_restrict_members can view page & duration screen,
		// but is refused on every write action (mt, mr, ma, mx).
		env.setAdminsAllowed(env.chat.Id, true)
		env.fake.setMember(env.chat.Id, admin.Id, staffFakeMember{
			Status:             gotgbot.ChatMemberStatusAdministrator,
			CanRestrictMembers: false,
		})
		m := settingsListOpenMenuPage(env, env.chat, admin, settingsPageModDefaults)
		if text, _ := m.last(); !strings.Contains(text, staffMarker("settings_moddef_intro")) {
			t.Fatalf("admin without restrict could not view page q:\n%s", text)
		}
		m.press("a", settingsActModDefDur, "k", settingsModDefKindBan)
		if text, _ := m.last(); !strings.Contains(text, staffMarker("settings_moddef_dur_intro_ban")) {
			t.Errorf("admin without restrict could not view duration screen:\n%s", text)
		}
		for _, write := range [][]string{
			{"a", settingsActModDefSetDur, "k", settingsModDefKindBan, "v", "3600"},
			{"a", settingsActModDefReason, "v", "1"},
			{"a", settingsActModDefAppealPrompt},
			{"a", settingsActModDefAppealRemove},
		} {
			m.press(write...)
			if ans, alert := env.lastAnswer(); !alert || !strings.Contains(ans, staffMarker("settings_no_restrict")) {
				t.Errorf("write %v without restrict answered %q (alert %v), want settings_no_restrict", write, ans, alert)
			}
		}
		if defs := settingsModDefaultsStored(t, env.chat.Id); defs != (modtuning.Defaults{}) {
			t.Fatalf("refused writes modified DB: %+v", defs)
		}

		// Mid-wait demotion: admin starts appeal wait with can_restrict_members, loses right before typing.
		env.fake.setMember(env.chat.Id, admin.Id, staffFakeMember{
			Status:             gotgbot.ChatMemberStatusAdministrator,
			CanRestrictMembers: true,
		})
		m.press("a", settingsActModDefAppealPrompt)
		if !env.redis.Exists(settingsInputKey(admin.Id)) {
			t.Fatalf("admin with restrict could not start appeal wait")
		}
		env.fake.setMember(env.chat.Id, admin.Id, staffFakeMember{
			Status:             gotgbot.ChatMemberStatusAdministrator,
			CanRestrictMembers: false,
		})
		sentBefore := len(env.fake.sentTo(admin.Id))
		settingsListDoorSay(env, admin, "https://example.com/demoted")
		replies := env.fake.sentTo(admin.Id)[sentBefore:]
		if len(replies) == 0 || !strings.Contains(sentText(replies[0]), staffMarker("settings_no_restrict")) {
			t.Errorf("demoted mid-wait reply answered %+v, want settings_no_restrict", replies)
		}
		if defs := settingsModDefaultsStored(t, env.chat.Id); defs.AppealURL != "" {
			t.Errorf("demoted mid-wait reply saved AppealURL = %q", defs.AppealURL)
		}
	})

	t.Run("forged_and_invalid_presses", func(t *testing.T) {
		env := newSettingsModDefaultsEnv(t)
		owner := settingsFiltersOwner(env, env.chat)
		msgID := env.updateID()

		before := len(callsToChat(env.fake.staffBotClient, "getChatMember", env.chat.Id))

		// Forged private presses (bad k, invalid duration, non-canonical 00, bad v, bad t, extra key).
		forgedPrivate := [][]string{
			{"a", settingsActModDefDur, "k", "x"},
			{"a", settingsActModDefDur},
			{"a", settingsActModDefSetDur, "k", "x", "v", "3600"},
			{"a", settingsActModDefSetDur, "k", "b", "v", "999"},
			{"a", settingsActModDefSetDur, "k", "b", "v", "00"},
			{"a", settingsActModDefSetDur, "k", "b", "v", "1h"},
			{"a", settingsActModDefReason, "v", "2"},
			{"a", settingsActModDefReason, "v", "1", "x", "1"},
			{"a", settingsActModDefAppealCancel, "t", "z"},
			{"a", settingsActModDefAppealPrompt, "x", "1"},
			{"a", settingsActModDefAppealRemove, "x", "1"},
		}
		for _, kv := range forgedPrivate {
			env.pressPrivate(owner, msgID, env.privateFields(env.chat.Id, kv...))
			if ans, alert := env.lastAnswer(); !alert || !strings.Contains(ans, staffMarker("common_callback_invalid_request")) {
				t.Errorf("forged private %v answered %q (alert %v), want invalid request", kv, ans, alert)
			}
		}

		// Group-menu presses (u instead of c) on every private-only action code must fail step 1.
		forgedGroup := [][]string{
			{"a", settingsActModDefDur, "k", "b"},
			{"a", settingsActModDefSetDur, "k", "b", "v", "3600"},
			{"a", settingsActModDefReason, "v", "1"},
			{"a", settingsActModDefAppealPrompt},
			{"a", settingsActModDefAppealCancel, "t", "a"},
			{"a", settingsActModDefAppealRemove},
		}
		for _, kv := range forgedGroup {
			env.press(owner, msgID, env.groupFields(owner, kv...))
			if ans, alert := env.lastAnswer(); !alert || !strings.Contains(ans, staffMarker("common_callback_invalid_request")) {
				t.Errorf("forged group %v answered %q (alert %v), want invalid request", kv, ans, alert)
			}
		}

		after := len(callsToChat(env.fake.staffBotClient, "getChatMember", env.chat.Id))
		if after != before {
			t.Errorf("forged/group presses made %d getChatMember calls, want 0", after-before)
		}
	})

	t.Run("worst_case_callback_bytes", func(t *testing.T) {
		newSettingsEnv(t)
		tr := i18n.MustNewTranslator("en")
		v := settingsView{
			Tr: tr, Private: true, OwnerKey: "c", OwnerValue: "-1001234567890",
			GroupID: -1001234567890, GroupTitle: "G", OpenerName: "G",
		}
		mainEntries := []settingsMainEntry{
			{Key: settingsPageModDefaults, Icon: "⚖️", Title: settingsText(tr, "settings_page_moddefaults"), State: settingsText(tr, "settings_state_default"), Known: true},
		}
		screens := map[string]settingsScreen{
			"main":          renderSettingsMain(v, mainEntries, ""),
			"page_unset":    renderSettingsModDefaults(v, modtuning.Defaults{}, ""),
			"page_set":      renderSettingsModDefaults(v, modtuning.Defaults{DefaultBanSecs: 2592000, DefaultMuteSecs: 2592000, ReasonRequired: true, AppealURL: "https://example.com/appeal"}, ""),
			"dur_ban":       renderSettingsModDefDuration(v, settingsModDefKindBan, 2592000, ""),
			"dur_mute":      renderSettingsModDefDuration(v, settingsModDefKindMute, 0, ""),
			"appeal_prompt": renderSettingsModDefAppealPrompt(v, time.Now().Add(settingsInputTTL), true),
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
			t.Fatal("no callback data measured")
		}
	})

	t.Run("all_8_languages_screen_lengths_and_labels", func(t *testing.T) {
		withRealLocales(t)
		longURL := "https://example.com/appeal/" + strings.Repeat("a", 160) + "?x=<script>&y=1"
		if utf8.RuneCountInString(longURL) < 200 {
			longURL += strings.Repeat("z", 200-utf8.RuneCountInString(longURL))
		}
		titles := map[string]string{
			"ampersands": strings.Repeat("&", 64),
			"astral":     strings.Repeat("😀", 64),
			"multibyte":  strings.Repeat("Ж", 64),
		}
		for _, code := range []string{"uk", "en", "es", "fr", "hi", "id", "pt", "ru"} {
			tr := i18n.MustNewTranslator(code)
			for titleName, title := range titles {
				v := settingsView{
					Tr: tr, Private: true, OwnerKey: "c", OwnerValue: "-1001234567890",
					GroupID: -1001234567890, GroupTitle: title, OpenerName: title,
				}
				screens := map[string]settingsScreen{
					"page_default": renderSettingsModDefaults(v, modtuning.Defaults{}, settingsNoticeOK(settingsText(tr, "settings_moddef_removed_appeal"))),
					"page_full": renderSettingsModDefaults(v, modtuning.Defaults{
						DefaultBanSecs:  2592000,
						DefaultMuteSecs: 604800,
						ReasonRequired:  true,
						AppealURL:       longURL,
					}, settingsNoticeOK(settingsText(tr, "settings_moddef_saved_appeal"))),
					"dur_ban":       renderSettingsModDefDuration(v, settingsModDefKindBan, 2592000, ""),
					"dur_mute":      renderSettingsModDefDuration(v, settingsModDefKindMute, 0, ""),
					"appeal_prompt": renderSettingsModDefAppealPrompt(v, time.Now().Add(settingsInputTTL), true),
				}
				for name, screen := range screens {
					label := fmt.Sprintf("%s %s %s", code, titleName, name)
					if units := panelUTF16Len(screen.Text); units == 0 || units > 3800 {
						t.Errorf("%s is %d UTF-16 units, want 1..3800", label, units)
					}
					if screen.Omitted != 0 {
						t.Errorf("%s omitted %d buttons", label, screen.Omitted)
					}
					if strings.Contains(screen.Text, "<script>") {
						t.Errorf("%s rendered unescaped HTML from appeal URL", label)
					}
					for _, row := range screen.Keyboard.InlineKeyboard {
						for _, btn := range row {
							limit := 40
							if len(row) > 1 {
								limit = 20
							}
							if runes := utf8.RuneCountInString(btn.Text); runes > limit {
								t.Errorf("%s: button %q has %d code points, limit %d", label, btn.Text, runes, limit)
							}
							if strings.Contains(btn.CallbackData, "example.com") {
								t.Errorf("%s: appeal URL leaked into callback data %q", label, btn.CallbackData)
							}
						}
					}
				}
			}
		}
	})
}
