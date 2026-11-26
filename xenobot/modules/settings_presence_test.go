//go:build testtools

package modules

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"unicode/utf8"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/db/presence"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
)

func settingsPresenceCleanup(t *testing.T, chatID int64) {
	t.Helper()
	t.Cleanup(func() {
		if db.DB != nil {
			_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatPresence{}).Error
		}
		cache.DeleteCache(presence.CacheKey(chatID))
	})
}

func settingsPresenceStored(t *testing.T, chatID int64) presence.Settings {
	t.Helper()
	got, err := presence.GetFresh(context.Background(), chatID)
	if err != nil {
		t.Fatalf("presence.GetFresh(%d) = %v", chatID, err)
	}
	return got
}

func settingsPresenceRows(t *testing.T, chatID int64) int64 {
	t.Helper()
	var n int64
	if err := db.DB.Model(&models.ChatPresence{}).Where("chat_id = ?", chatID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func settingsPresenceOpenPrivate(t *testing.T, env *settingsEnv, user gotgbot.User) int64 {
	t.Helper()
	env.sendPrivate(user, fmt.Sprintf("/start settings_%d", env.chat.Id))
	msgID := settingsPrivateOnlySent(t, env, user).MessageID
	env.pressPrivate(user, msgID, env.privateFields(env.chat.Id, "a", settingsActOpen, "p", settingsPagePresence))
	return msgID
}

func TestSettingsPresenceCallbackBytes(t *testing.T) {
	withRealLocales(t)
	tr := i18n.MustNewTranslator("en")
	view := settingsView{
		Tr: tr, Private: true, OwnerKey: "c", OwnerValue: "-1001234567890",
		GroupID: -1001234567890, GroupTitle: "G", OpenerName: "N", Owner: true,
	}
	for _, consensus := range []bool{false, true} {
		for _, retract := range []bool{false, true} {
			for _, threshold := range []int{2, 3, 20} {
				stored := presence.Settings{
					ConsensusEnabled:   consensus,
					ConsensusThreshold: threshold,
					RetractEnabled:     retract,
				}
				screen := renderSettingsPresence(view, stored, "")
				label := fmt.Sprintf("cons=%t retract=%t thr=%d", consensus, retract, threshold)
				if screen.Omitted != 0 {
					t.Errorf("%s omitted %d buttons", label, screen.Omitted)
				}
				if got := len(screen.Keyboard.InlineKeyboard); got != 4 {
					t.Errorf("%s rows = %d, want 4", label, got)
				}
				for _, button := range settingsAllButtons(screen.Keyboard.InlineKeyboard) {
					if n := len(button.CallbackData); n < 1 || n > 64 {
						t.Errorf("%s button %q has %d bytes", label, button.Text, n)
					}
				}
			}
		}
	}
}

func TestSettingsPresencePageFitsEveryLocale(t *testing.T) {
	withRealLocales(t)
	title := strings.Repeat("&<ж😀", 16)
	states := []presence.Settings{
		{ConsensusEnabled: false, ConsensusThreshold: 2, RetractEnabled: false},
		{ConsensusEnabled: true, ConsensusThreshold: 20, RetractEnabled: true},
	}
	for _, code := range []string{"uk", "en", "es", "fr", "hi", "id", "pt", "ru"} {
		t.Run(code, func(t *testing.T) {
			tr := i18n.MustNewTranslator(code)
			notice := settingsNoticeWarn(settingsText(tr, "common_settings_save_failed"))
			view := settingsView{
				Tr: tr, Private: true, OwnerKey: "c", OwnerValue: "-1001234567890",
				GroupID: -1001234567890, GroupTitle: title, OpenerName: "N", Owner: true,
			}
			for _, stored := range states {
				screen := renderSettingsPresence(view, stored, notice)
				if screen.Omitted != 0 {
					t.Errorf("omitted %d buttons", screen.Omitted)
				}
				if rows := len(screen.Keyboard.InlineKeyboard); rows > 8 {
					t.Errorf("page has %d rows, want <= 8", rows)
				}
				if got := panelUTF16Len(screen.Text); got > 3800 {
					t.Errorf("screen is %d UTF-16 units, want <= 3800", got)
				}
				for _, button := range settingsAllButtons(screen.Keyboard.InlineKeyboard) {
					if n := utf8.RuneCountInString(button.Text); n == 0 || n > 40 {
						t.Errorf("button %q is %d code points, want 1..40", button.Text, n)
					}
				}
			}
		})
	}
}

func TestSettingsPresenceFlows(t *testing.T) {
	resetSettingsOwnerMemo(t)
	env := newSettingsEnv(t)
	settingsPresenceCleanup(t, env.chat.Id)
	owner := env.newOwner(env.chat)
	privateOwner := settingsPrivateChat(owner)

	// 1. Owner opens pr in private, toggles consensus & retract, steps threshold & bounds,
	//    verifies idempotency (zero extra writes) and write failure warning notice.
	t.Run("owner_controls_bounds_idempotency_and_save_failure", func(t *testing.T) {
		msgID := settingsPresenceOpenPrivate(t, env, owner)
		text, kb := env.lastEdit(owner.Id, msgID)
		for _, want := range []string{
			"settings_page_presence", "settings_pr_intro",
			"settings_pr_consensus_line", "settings_pr_threshold_line",
			"settings_pr_retract_line", "settings_pr_hint",
		} {
			if !strings.Contains(text, staffMarker(want)) {
				t.Fatalf("page pr missing %s:\n%s", want, text)
			}
		}
		if settingsPresenceRows(t, env.chat.Id) != 0 {
			t.Fatal("opening pr created a database row")
		}

		// Toggle Consensus On.
		consOn, ok := settingsChatFind(t, kb, func(f map[string]string) bool {
			return f["a"] == settingsActToggle && f["t"] == settingsPresenceControlConsensus
		})
		if !ok || settingsButtonFields(t, consOn)["v"] != "1" {
			t.Fatalf("consensus toggle button missing v=1: %+v", kb)
		}
		env.pressRaw(privateOwner, owner, msgID, consOn.CallbackData)
		if got := settingsPresenceStored(t, env.chat.Id); !got.ConsensusEnabled || got.ConsensusThreshold != 3 || !got.RetractEnabled {
			t.Fatalf("after consensus on: %+v, want ConsensusEnabled=true, Threshold=3, RetractEnabled=true", got)
		}

		// Step threshold +1 -> 4.
		_, kb = env.lastEdit(owner.Id, msgID)
		step4, ok := settingsChatFind(t, kb, func(f map[string]string) bool {
			return f["a"] == settingsActSet && f["t"] == settingsPresenceControlThreshold && f["v"] == "4"
		})
		if !ok {
			t.Fatalf("stepper missing +1 button (v=4): %+v", kb)
		}
		env.pressRaw(privateOwner, owner, msgID, step4.CallbackData)
		if got := settingsPresenceStored(t, env.chat.Id); got.ConsensusThreshold != 4 {
			t.Fatalf("after +1 step: threshold = %d, want 4", got.ConsensusThreshold)
		}

		// Step threshold -5 -> clamped to 2, and negative stepper buttons now show bound label "·".
		_, kb = env.lastEdit(owner.Id, msgID)
		step2, ok := settingsChatFind(t, kb, func(f map[string]string) bool {
			return f["a"] == settingsActSet && f["t"] == settingsPresenceControlThreshold && f["v"] == "2"
		})
		if !ok {
			t.Fatalf("stepper missing -5 clamped button (v=2): %+v", kb)
		}
		env.pressRaw(privateOwner, owner, msgID, step2.CallbackData)
		if got := settingsPresenceStored(t, env.chat.Id); got.ConsensusThreshold != 2 {
			t.Fatalf("after -5 step: threshold = %d, want 2", got.ConsensusThreshold)
		}
		_, kb = env.lastEdit(owner.Id, msgID)
		stepperRow := kb[1]
		if len(stepperRow) != 5 || stepperRow[0].Text != settingsBoundLabel || stepperRow[1].Text != settingsBoundLabel || stepperRow[2].Text != "2" {
			t.Errorf("stepper row at lower bound = %+v, want [· · 2 +1 +5]", stepperRow)
		}

		// Toggle 👎 Retraction Off (explicit false persisted!) and back On.
		retractOff, ok := settingsChatFind(t, kb, func(f map[string]string) bool {
			return f["a"] == settingsActToggle && f["t"] == settingsPresenceControlRetract
		})
		if !ok || settingsButtonFields(t, retractOff)["v"] != "0" {
			t.Fatalf("retract toggle missing v=0: %+v", kb)
		}
		env.pressRaw(privateOwner, owner, msgID, retractOff.CallbackData)
		if got := settingsPresenceStored(t, env.chat.Id); got.RetractEnabled {
			t.Fatalf("retract off did not persist explicit false: %+v", got)
		}
		_, kb = env.lastEdit(owner.Id, msgID)
		retractOn, ok := settingsChatFind(t, kb, func(f map[string]string) bool {
			return f["a"] == settingsActToggle && f["t"] == settingsPresenceControlRetract
		})
		if !ok || settingsButtonFields(t, retractOn)["v"] != "1" {
			t.Fatalf("retract off button missing v=1: %+v", kb)
		}
		env.pressRaw(privateOwner, owner, msgID, retractOn.CallbackData)
		if got := settingsPresenceStored(t, env.chat.Id); !got.RetractEnabled {
			t.Fatalf("retract on did not persist true: %+v", got)
		}

		// Idempotent press writes nothing.
		var writeCalls atomic.Int32
		origCons := presenceWrites.ConsensusEnabled
		origThr := presenceWrites.ConsensusThreshold
		origRet := presenceWrites.RetractEnabled
		presenceWrites.ConsensusEnabled = func(ctx context.Context, chatID int64, enabled bool, by int64) error {
			writeCalls.Add(1)
			return origCons(ctx, chatID, enabled, by)
		}
		presenceWrites.ConsensusThreshold = func(ctx context.Context, chatID int64, threshold int, by int64) error {
			writeCalls.Add(1)
			return origThr(ctx, chatID, threshold, by)
		}
		presenceWrites.RetractEnabled = func(ctx context.Context, chatID int64, enabled bool, by int64) error {
			writeCalls.Add(1)
			return origRet(ctx, chatID, enabled, by)
		}
		t.Cleanup(func() {
			presenceWrites.ConsensusEnabled = origCons
			presenceWrites.ConsensusThreshold = origThr
			presenceWrites.RetractEnabled = origRet
		})
		env.pressRaw(privateOwner, owner, msgID, consOn.CallbackData)    // already true
		env.pressRaw(privateOwner, owner, msgID, step2.CallbackData)     // already 2
		env.pressRaw(privateOwner, owner, msgID, retractOn.CallbackData) // already true
		if writeCalls.Load() != 0 {
			t.Errorf("idempotent presses made %d DB writes, want 0", writeCalls.Load())
		}

		// Write error displays common_settings_save_failed warning notice.
		presenceWrites.ConsensusEnabled = func(context.Context, int64, bool, int64) error {
			return errors.New("db write failed")
		}
		env.pressPrivate(owner, msgID, env.privateFields(env.chat.Id, "a", settingsActToggle, "p", settingsPagePresence, "t", settingsPresenceControlConsensus, "v", "0"))
		errText, _ := env.lastEdit(owner.Id, msgID)
		if !strings.Contains(errText, staffMarker("common_settings_save_failed")) {
			t.Errorf("write error screen = %q, want common_settings_save_failed notice", errText)
		}
	})

	// 2. Permitted admin with CanChangeInfo opens & writes; admin without CanChangeInfo is refused.
	t.Run("admin_with_and_without_can_change_info", func(t *testing.T) {
		env.setAdminsAllowed(env.chat.Id, true)

		allowedAdmin := gotgbot.User{Id: env.admin.Id + 10, FirstName: "AllowedAdmin"}
		env.fake.setMemberRaw(env.chat.Id, allowedAdmin.Id, copyAdminJSON(allowedAdmin.Id, true, false))
		msgID := settingsPresenceOpenPrivate(t, env, allowedAdmin)
		env.pressPrivate(allowedAdmin, msgID, env.privateFields(env.chat.Id, "a", settingsActSet, "p", settingsPagePresence, "t", settingsPresenceControlThreshold, "v", "7"))
		if got := settingsPresenceStored(t, env.chat.Id); got.ConsensusThreshold != 7 {
			t.Fatalf("allowed admin threshold = %d, want 7", got.ConsensusThreshold)
		}

		deniedAdmin := gotgbot.User{Id: env.admin.Id + 11, FirstName: "DeniedAdmin"}
		env.fake.setMemberRaw(env.chat.Id, deniedAdmin.Id, copyAdminJSON(deniedAdmin.Id, false, true))
		deniedMsg := settingsPresenceOpenPrivate(t, env, deniedAdmin)
		env.pressPrivate(deniedAdmin, deniedMsg, env.privateFields(env.chat.Id, "a", settingsActSet, "p", settingsPagePresence, "t", settingsPresenceControlThreshold, "v", "11"))
		if _, alert := env.lastAnswer(); !alert {
			t.Error("admin without CanChangeInfo did not get refusal alert")
		}
		if got := settingsPresenceStored(t, env.chat.Id); got.ConsensusThreshold != 7 {
			t.Fatalf("denied admin mutated threshold to %d, want 7", got.ConsensusThreshold)
		}
	})

	// 3. Group menu shows settings_pr_group_note with 0 DB reads; forged control from group menu refused.
	t.Run("group_menu_private_only_note_and_forged_control_refusal", func(t *testing.T) {
		var reads atomic.Int32
		origRead := settingsPresenceRead
		settingsPresenceRead = func(ctx context.Context, chatID int64) (presence.Settings, error) {
			reads.Add(1)
			return origRead(ctx, chatID)
		}
		t.Cleanup(func() { settingsPresenceRead = origRead })

		groupMsg := env.openMenu(owner)
		env.press(owner, groupMsg, env.groupFields(owner, "a", settingsActOpen, "p", settingsPagePresence))
		noteText, _ := env.lastEdit(env.chat.Id, groupMsg)
		if !strings.Contains(noteText, staffMarker("settings_pr_group_note")) {
			t.Errorf("group menu op p=pr drew:\n%s\nwant settings_pr_group_note", noteText)
		}
		if reads.Load() != 0 {
			t.Errorf("group menu op p=pr called settingsPresenceRead %d times, want 0", reads.Load())
		}

		before := settingsPresenceStored(t, env.chat.Id)
		for _, kv := range [][]string{
			{"a", settingsActToggle, "p", settingsPagePresence, "t", settingsPresenceControlConsensus, "v", "0"},
			{"a", settingsActSet, "p", settingsPagePresence, "t", settingsPresenceControlThreshold, "v", "9"},
			{"a", settingsActToggle, "p", settingsPagePresence, "t", settingsPresenceControlRetract, "v", "0"},
		} {
			env.press(owner, groupMsg, env.groupFields(owner, kv...))
			if ans, alert := env.lastAnswer(); !alert || !strings.Contains(ans, staffMarker("common_callback_invalid_request")) {
				t.Errorf("group forged control %v answered (%q, alert=%v), want invalid-request alert", kv, ans, alert)
			}
		}
		if reads.Load() != 0 {
			t.Errorf("group forged controls called settingsPresenceRead %d times, want 0", reads.Load())
		}
		if got := settingsPresenceStored(t, env.chat.Id); got != before {
			t.Errorf("group forged controls mutated DB: got %+v, want %+v", got, before)
		}
	})

	// 4. Forged out-of-range thresholds (1, 21, 03) rejected before any DB read/write.
	t.Run("forged_out_of_range_thresholds_rejected", func(t *testing.T) {
		var reads atomic.Int32
		origRead := settingsPresenceRead
		settingsPresenceRead = func(ctx context.Context, chatID int64) (presence.Settings, error) {
			reads.Add(1)
			return origRead(ctx, chatID)
		}
		t.Cleanup(func() { settingsPresenceRead = origRead })

		before := settingsPresenceStored(t, env.chat.Id)
		privMsg := env.updateID()
		for _, bad := range []string{"1", "21", "03", "-2", "abc"} {
			env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActSet, "p", settingsPagePresence, "t", settingsPresenceControlThreshold, "v", bad))
			if ans, alert := env.lastAnswer(); !alert || !strings.Contains(ans, staffMarker("common_callback_invalid_request")) {
				t.Errorf("forged threshold %q answered (%q, alert=%v), want invalid-request alert", bad, ans, alert)
			}
		}
		if reads.Load() != 0 {
			t.Errorf("invalid threshold presses called settingsPresenceRead %d times, want 0", reads.Load())
		}
		if got := settingsPresenceStored(t, env.chat.Id); got != before {
			t.Errorf("invalid threshold presses mutated DB: got %+v, want %+v", got, before)
		}
	})

	// 5. Staff Group hides pr from Main, refuses forged op/control without naming Staff Group,
	//    and Staff lookup error fails closed with cannot-open and zero writes.
	t.Run("staff_group_hidden_refused_and_lookup_error_fails_closed", func(t *testing.T) {
		staffChat := env.addChat()
		settingsPresenceCleanup(t, staffChat.Id)
		copySeedStaffGroup(t, staffChat.Id, owner.Id)
		env.fake.setMemberRaw(staffChat.Id, owner.Id, copyCreatorJSON(owner.Id))
		env.fake.setCreator(staffChat.Id, owner.Id)

		staffPrivMsg := env.updateID()
		env.pressPrivate(owner, staffPrivMsg, env.privateFields(staffChat.Id, "a", settingsActBack))
		staffMainText, staffMainKB := env.lastEdit(owner.Id, staffPrivMsg)
		if _, hasLine := settingsMainStateLine(staffMainText, "settings_page_presence"); hasLine {
			t.Error("Staff Group Main still shows settings_page_presence line")
		}
		if _, hasBtn := settingsPageButton(t, staffMainKB, settingsPagePresence); hasBtn {
			t.Error("Staff Group Main still has pr button")
		}

		for _, fields := range []map[string]string{
			env.privateFields(staffChat.Id, "a", settingsActOpen, "p", settingsPagePresence),
			env.privateFields(staffChat.Id, "a", settingsActToggle, "p", settingsPagePresence, "t", settingsPresenceControlConsensus, "v", "1"),
			env.privateFields(staffChat.Id, "a", settingsActSet, "p", settingsPagePresence, "t", settingsPresenceControlThreshold, "v", "5"),
			env.privateFields(staffChat.Id, "a", settingsActToggle, "p", settingsPagePresence, "t", settingsPresenceControlRetract, "v", "0"),
		} {
			env.pressPrivate(owner, staffPrivMsg, fields)
			ans, alert := env.lastAnswer()
			if !alert || !strings.Contains(ans, staffMarker("common_callback_invalid_request")) {
				t.Errorf("Staff Group %v answered (%q, alert=%v), want common_callback_invalid_request", fields, ans, alert)
			}
			if strings.Contains(strings.ToLower(ans), "staff") {
				t.Errorf("Staff Group refusal leaked Staff Group identity: %q", ans)
			}
		}
		if settingsPresenceRows(t, staffChat.Id) != 0 {
			t.Fatal("Staff Group forged presses wrote a presence row")
		}

		// Staff lookup error fails closed with cannot-open edit and zero writes.
		origStaff := settingsPresenceStaffLookup
		settingsPresenceStaffLookup = func(int64) (*models.StaffGroup, error) {
			return nil, errors.New("staff lookup down")
		}
		t.Cleanup(func() { settingsPresenceStaffLookup = origStaff })

		errChat := env.addChat()
		settingsPresenceCleanup(t, errChat.Id)
		env.fake.setMemberRaw(errChat.Id, owner.Id, copyCreatorJSON(owner.Id))
		env.fake.setCreator(errChat.Id, owner.Id)
		errMsg := env.updateID()

		env.pressPrivate(owner, errMsg, env.privateFields(errChat.Id, "a", settingsActOpen, "p", settingsPagePresence))
		if text, _ := env.lastEdit(owner.Id, errMsg); !strings.Contains(text, staffMarker("settings_cannot_open")) {
			t.Errorf("staff lookup error on open drew %q, want settings_cannot_open", text)
		}
		env.pressPrivate(owner, errMsg, env.privateFields(errChat.Id, "a", settingsActToggle, "p", settingsPagePresence, "t", settingsPresenceControlConsensus, "v", "1"))
		if text, _ := env.lastEdit(owner.Id, errMsg); !strings.Contains(text, staffMarker("settings_cannot_open")) {
			t.Errorf("staff lookup error on control drew %q, want settings_cannot_open", text)
		}
		if settingsPresenceRows(t, errChat.Id) != 0 {
			t.Fatal("staff lookup error still wrote a presence row")
		}
	})
}
