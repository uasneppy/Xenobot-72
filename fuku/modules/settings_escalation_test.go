//go:build testtools

package modules

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/fuku_robot/fuku/db"
	dbcache "github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/escalation"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
)

// settingsEscEnv is a settings environment for chat whose ladder rows and counts go when the test ends.
func settingsEscEnv(t *testing.T) *settingsEnv {
	t.Helper()
	resetSettingsOwnerMemo(t)
	env := newSettingsEnv(t)
	t.Cleanup(func() {
		db.DB.Where("chat_id = ?", env.chat.Id).Delete(&models.ChatEscalation{})
		db.DB.Where("chat_id = ?", env.chat.Id).Delete(&models.ChatOffence{})
		dbcache.DeleteCache(escalation.CacheKey(env.chat.Id))
	})
	return env
}

func settingsEscRows(t *testing.T, chatID int64) int64 {
	t.Helper()
	var n, m int64
	if err := db.DB.Model(&models.ChatEscalation{}).Where("chat_id = ?", chatID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.DB.Model(&models.ChatOffence{}).Where("chat_id = ?", chatID).Count(&m).Error; err != nil {
		t.Fatal(err)
	}
	return n + m
}

func settingsEscStored(t *testing.T, chatID int64) *escalation.Ladder {
	t.Helper()
	ladder, err := escalation.GetFresh(context.Background(), chatID)
	if err != nil {
		t.Fatal(err)
	}
	return ladder
}

// settingsEscOpenPrivate opens the private menu of env's group for user and presses the Escalation cell; it
// returns the message ID.
func settingsEscOpenPrivate(t *testing.T, env *settingsEnv, user gotgbot.User) int64 {
	t.Helper()
	env.sendPrivate(user, fmt.Sprintf("/start settings_%d", env.chat.Id))
	msgID := settingsPrivateOnlySent(t, env, user).MessageID
	env.pressPrivate(user, msgID, env.privateFields(env.chat.Id, "a", "op", "p", settingsPageEscalation))
	return msgID
}

func settingsEscRowShape(keyboard [][]gotgbot.InlineKeyboardButton) []int {
	shape := make([]int, 0, len(keyboard))
	for _, row := range keyboard {
		shape = append(shape, len(row))
	}
	return shape
}

func settingsEscWantShape(t *testing.T, keyboard [][]gotgbot.InlineKeyboardButton, want ...int) {
	t.Helper()
	if got := settingsEscRowShape(keyboard); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("rows = %v, want %v", got, want)
	}
}

func TestSettingsEscalationOpens(t *testing.T) {
	env := settingsEscEnv(t)
	owner := env.newOwner(env.chat)

	t.Run("the private menu shows the cell after Warnings, opens it and turns the ladder on", func(t *testing.T) {
		env.sendPrivate(owner, fmt.Sprintf("/start settings_%d", env.chat.Id))
		sent := env.fake.sentTo(owner.Id)
		main := fmt.Sprint(sent[len(sent)-1].Params["text"])
		line, ok := settingsMainStateLine(main, "settings_page_esc")
		if !ok || !strings.Contains(line, staffMarker("settings_state_off_plain")) {
			t.Fatalf("Main line of the Escalation page = %q (found %v), want Off", line, ok)
		}
		warns, _ := settingsMainStateLine(main, "settings_page_warns")
		if strings.Index(main, line) < strings.Index(main, warns) {
			t.Error("the Escalation line comes before Warnings")
		}

		msgID := settingsPrivateOnlySent(t, env, owner).MessageID
		env.pressPrivate(owner, msgID, env.privateFields(env.chat.Id, "a", "op", "p", settingsPageEscalation))
		text, keyboard := env.lastEdit(owner.Id, msgID)
		for _, want := range []string{"settings_esc_intro", "settings_esc_state", "settings_esc_no_steps", "settings_esc_forget_line", "settings_esc_hint"} {
			if !strings.Contains(text, staffMarker(want)) {
				t.Errorf("the page lacks %s: %q", want, text)
			}
		}
		settingsEscWantShape(t, keyboard, 1, 1, 4, 2)

		env.pressPrivate(owner, msgID, env.privateFields(env.chat.Id, "a", settingsActEscToggle, "v", "1"))
		got := settingsEscStored(t, env.chat.Id)
		if !got.Enabled || len(got.Steps) != 3 || got.Steps[0] != (escalation.Step{Action: "tmute", Secs: 3600}) || got.Steps[2].Action != "ban" {
			t.Fatalf("stored ladder = %+v, want on with the example", got)
		}
		text, keyboard = env.lastEdit(owner.Id, msgID)
		if !strings.Contains(text, staffMarker("settings_esc_example_added")) || !strings.Contains(text, staffMarker("settings_esc_step_line_last")) {
			t.Errorf("the page after turning on = %q, want the example notice and a last-step line", text)
		}
		settingsEscWantShape(t, keyboard, 1, 3, 2, 4, 2)

		env.sendPrivate(owner, fmt.Sprintf("/start settings_%d", env.chat.Id))
		sent = env.fake.sentTo(owner.Id)
		main = fmt.Sprint(sent[len(sent)-1].Params["text"])
		if line, _ := settingsMainStateLine(main, "settings_page_esc"); !strings.Contains(line, staffMarker("settings_state_esc_steps")) {
			t.Errorf("Main line after turning on = %q, want the step count", line)
		}
	})

	t.Run("a group menu reads In private and opens the note", func(t *testing.T) {
		msgID := env.openMenu(owner)
		text, _ := env.lastEdit(env.chat.Id, msgID)
		if line, ok := settingsMainStateLine(text, "settings_page_esc"); !ok || !strings.Contains(line, staffMarker("settings_state_private_only")) {
			t.Errorf("group Main line = %q (found %v), want In private", line, ok)
		}
		env.press(owner, msgID, env.groupFields(owner, "a", "op", "p", settingsPageEscalation))
		text, _ = env.lastEdit(env.chat.Id, msgID)
		if !strings.Contains(text, staffMarker("settings_esc_group_note")) || strings.Contains(text, staffMarker("settings_esc_intro")) {
			t.Errorf("group menu opened %q, want the group note alone", text)
		}
	})

	t.Run("a Staff Group has no cell and refuses a forged open", func(t *testing.T) {
		staffEnv := settingsEscEnv(t)
		staffOwner := staffEnv.newOwner(staffEnv.chat)
		mreqSwap(t, &settingsEscStaffLookup, func(int64) (*models.StaffGroup, error) { return &models.StaffGroup{}, nil })
		msgID := staffEnv.openMenu(staffOwner)
		text, keyboard := staffEnv.lastEdit(staffEnv.chat.Id, msgID)
		if _, found := settingsMainStateLine(text, "settings_page_esc"); found {
			t.Error("a Staff Group's Main has the Escalation line")
		}
		if _, ok := settingsPageButton(t, keyboard, settingsPageEscalation); ok {
			t.Error("a Staff Group's Main has the Escalation cell")
		}
		staffEnv.press(staffOwner, msgID, staffEnv.groupFields(staffOwner, "a", "op", "p", settingsPageEscalation))
		if answer, alert := staffEnv.lastAnswer(); !alert || !strings.Contains(answer, staffMarker("common_callback_invalid_request")) {
			t.Errorf("a forged open answered %q (alert %v), want the invalid-request alert", answer, alert)
		}
	})
}

func TestSettingsEscalationSteps(t *testing.T) {
	env := settingsEscEnv(t)
	owner := env.newOwner(env.chat)
	msgID := settingsEscOpenPrivate(t, env, owner)
	press := func(kv ...string) {
		t.Helper()
		env.pressPrivate(owner, msgID, env.privateFields(env.chat.Id, kv...))
	}
	press("a", settingsActEscToggle, "v", "1")

	// Step 2 opens with the current action marked, and Ban replaces it.
	press("a", settingsActEscStep, "t", "2")
	text, keyboard := env.lastEdit(owner.Id, msgID)
	if !strings.Contains(text, staffMarker("settings_esc_step_intro")) {
		t.Errorf("step screen = %q", text)
	}
	settingsEscWantShape(t, keyboard, 3, 2, 2)
	press("a", settingsActEscSet, "t", "2", "v", "ban")
	if got := settingsEscStored(t, env.chat.Id); got.Steps[1] != (escalation.Step{Action: "ban"}) {
		t.Fatalf("step 2 = %+v, want ban", got.Steps[1])
	}

	// Add a fourth step: step screen, duration screen, preset.
	press("a", settingsActEscStep, "t", "4")
	press("a", settingsActEscDuration, "t", "4", "k", "m")
	text, keyboard = env.lastEdit(owner.Id, msgID)
	if !strings.Contains(text, staffMarker("settings_duration_intro_tmute")) {
		t.Errorf("duration screen = %q", text)
	}
	settingsEscWantShape(t, keyboard, 2, 2, 2, 2, 2)
	press("a", settingsActEscTimed, "t", "4", "k", "m", "v", "21600")
	if got := settingsEscStored(t, env.chat.Id); len(got.Steps) != 4 || got.Steps[3] != (escalation.Step{Action: "tmute", Secs: 21600}) {
		t.Fatalf("steps = %+v, want a fourth timed step of 6 hours", got.Steps)
	}

	// Remove last: a stale index writes nothing, the right one removes the last step.
	press("a", settingsActEscRemove, "t", "2")
	if text, _ := env.lastEdit(owner.Id, msgID); !strings.Contains(text, staffMarker("settings_esc_stale")) {
		t.Errorf("a stale remove drew %q, want the stale notice", text)
	}
	if got := settingsEscStored(t, env.chat.Id); len(got.Steps) != 4 {
		t.Fatalf("a stale remove changed the ladder: %+v", got.Steps)
	}
	press("a", settingsActEscRemove, "t", "4")
	if got := settingsEscStored(t, env.chat.Id); len(got.Steps) != 3 {
		t.Fatalf("steps after Remove last = %+v, want 3", got.Steps)
	}

	// A step screen for an index beyond the next free one is stale too.
	press("a", settingsActEscStep, "t", "5")
	if text, _ := env.lastEdit(owner.Id, msgID); !strings.Contains(text, staffMarker("settings_esc_stale")) {
		t.Errorf("a stale step drew %q, want the stale notice", text)
	}

	// The only step of a ladder that is on is not removed.
	press("a", settingsActEscRemove, "t", "3")
	press("a", settingsActEscRemove, "t", "2")
	press("a", settingsActEscRemove, "t", "1")
	if text, _ := env.lastEdit(owner.Id, msgID); !strings.Contains(text, staffMarker("settings_esc_refuse_last_step")) {
		t.Errorf("removing the only step drew %q, want the refusal", text)
	}
	if got := settingsEscStored(t, env.chat.Id); len(got.Steps) != 1 {
		t.Fatalf("steps = %+v, want the one step kept", got.Steps)
	}

	press("a", settingsActEscForget, "v", "7")
	if got := settingsEscStored(t, env.chat.Id); got.ForgetDays != 7 {
		t.Errorf("forget days = %d, want 7", got.ForgetDays)
	}

	// Turning it off clears the counts.
	if _, err := escalation.CountOffence(context.Background(), env.chat.Id, 7, time.Now(), 24*time.Hour); err != nil {
		t.Fatal(err)
	}
	press("a", settingsActEscToggle, "v", "0")
	if got := settingsEscStored(t, env.chat.Id); got.Enabled {
		t.Error("the ladder is still on")
	}
	var counts int64
	db.DB.Model(&models.ChatOffence{}).Where("chat_id = ?", env.chat.Id).Count(&counts)
	if counts != 0 {
		t.Errorf("%d offence rows left after turning off", counts)
	}
}

// TestSettingsDriftEscalation: the page draws what the repository stores, and what the page stores is what the
// resolver reads at the next offence (the real flood, blacklist and forward blocklist sites are TestEscalation's).
func TestSettingsDriftEscalation(t *testing.T) {
	env := settingsEscEnv(t)
	owner := env.newOwner(env.chat)

	if err := escalation.SetEnabledWithSteps(context.Background(), env.chat.Id,
		[]escalation.Step{{Action: "kick"}, {Action: "tban", Secs: 86400}}, owner.Id); err != nil {
		t.Fatal(err)
	}
	msgID := settingsEscOpenPrivate(t, env, owner)
	text, _ := env.lastEdit(owner.Id, msgID)
	if !strings.Contains(text, staffMarker("settings_esc_step_line")) || !strings.Contains(text, staffMarker("settings_esc_step_line_last")) {
		t.Errorf("the page does not draw the stored steps: %q", text)
	}

	// Reset and set the ladder through the page, then ask the resolver.
	if err := escalation.SetEnabled(context.Background(), env.chat.Id, false, owner.Id); err != nil {
		t.Fatal(err)
	}
	for _, step := range []string{"3", "4"} {
		_ = step
	}
	press := func(kv ...string) { env.pressPrivate(owner, msgID, env.privateFields(env.chat.Id, kv...)) }
	press("a", settingsActEscRemove, "t", "2")
	press("a", settingsActEscTimed, "t", "1", "k", "m", "v", "3600")
	press("a", settingsActEscSet, "t", "2", "v", "ban")
	press("a", settingsActEscToggle, "v", "1")
	if got := settingsEscStored(t, env.chat.Id); !got.Enabled || len(got.Steps) != 2 {
		t.Fatalf("ladder = %+v", got)
	}

	start := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	clock := escalationClock(t, start)
	member := uniqueLinkOwnerID()
	own := autoPunishment{Action: "mute"}
	first := escalate(context.Background(), env.bot, env.chat.Id, member, "flood", own)
	if first != (autoPunishment{Action: "tmute", Secs: 3600}) {
		t.Errorf("first offence = %+v, want the page's step 1", first)
	}
	clock.advance(61 * time.Second)
	if second := escalate(context.Background(), env.bot, env.chat.Id, member, "flood", own); second != (autoPunishment{Action: "ban"}) {
		t.Errorf("second offence = %+v, want the page's step 2", second)
	}
}

func TestSettingsAuthorityEscalation(t *testing.T) {
	t.Run("an administrator without can_restrict_members opens the screens and is refused every write", func(t *testing.T) {
		env := settingsEscEnv(t)
		helper := env.newOutsider()
		env.fake.setMember(env.chat.Id, helper.Id, staffFakeMember{Status: gotgbot.ChatMemberStatusAdministrator, CanRestrictMembers: false})
		env.listAdmin(env.chat.Id, helper.Id)
		env.sendPrivate(helper, fmt.Sprintf("/start settings_%d", env.chat.Id))
		msgID := settingsPrivateOnlySent(t, env, helper).MessageID
		for _, open := range [][]string{
			{"a", "op", "p", settingsPageEscalation},
			{"a", settingsActEscStep, "t", "1"},
			{"a", settingsActEscDuration, "t", "1", "k", "b"},
		} {
			env.pressPrivate(helper, msgID, env.privateFields(env.chat.Id, open...))
			if text, _ := env.lastEdit(helper.Id, msgID); containsMarker(text, "common_callback_invalid_request") {
				t.Errorf("%v drew %q", open, text)
			}
		}
		for _, write := range [][]string{
			{"a", settingsActEscToggle, "v", "1"},
			{"a", settingsActEscForget, "v", "7"},
			{"a", settingsActEscSet, "t", "1", "v", "ban"},
			{"a", settingsActEscTimed, "t", "1", "k", "m", "v", "3600"},
			{"a", settingsActEscRemove, "t", "1"},
		} {
			edits := len(env.edits(helper.Id, msgID))
			env.pressPrivate(helper, msgID, env.privateFields(env.chat.Id, write...))
			if answer, alert := env.lastAnswer(); !alert || !strings.Contains(answer, staffMarker("settings_no_restrict")) {
				t.Errorf("%v answered %q (alert %v), want settings_no_restrict", write, answer, alert)
			}
			if got := len(env.edits(helper.Id, msgID)); got != edits {
				t.Errorf("%v edited the menu", write)
			}
		}
		if n := settingsEscRows(t, env.chat.Id); n != 0 {
			t.Errorf("a refused press wrote %d rows", n)
		}
	})

	t.Run("a plain member is not an administrator", func(t *testing.T) {
		env := settingsEscEnv(t)
		member := env.newOutsider()
		env.fake.setMember(env.chat.Id, member.Id, staffFakeMember{Status: gotgbot.ChatMemberStatusMember})
		env.pressPrivate(member, 77, env.privateFields(env.chat.Id, "a", settingsActEscToggle, "v", "1"))
		if answer, alert := env.lastAnswer(); !alert || !strings.Contains(answer, staffMarker("settings_not_admin")) {
			t.Errorf("answered %q (alert %v), want settings_not_admin", answer, alert)
		}
		if n := settingsEscRows(t, env.chat.Id); n != 0 {
			t.Errorf("a member's press wrote %d rows", n)
		}
	})

	t.Run("an administrator the owner gate refuses is refused at the gate", func(t *testing.T) {
		env := settingsEscEnv(t)
		env.newOwner(env.chat)
		env.setAdminsAllowed(env.chat.Id, false)
		env.pressPrivate(env.admin, 78, env.privateFields(env.chat.Id, "a", settingsActEscToggle, "v", "1"))
		if answer, alert := env.lastAnswer(); !alert || !strings.Contains(answer, staffMarker("settings_gate_owner_only")) {
			t.Errorf("answered %q (alert %v), want settings_gate_owner_only", answer, alert)
		}
		if n := settingsEscRows(t, env.chat.Id); n != 0 {
			t.Errorf("a gated administrator's press wrote %d rows", n)
		}
	})

	t.Run("the live creator writes", func(t *testing.T) {
		env := settingsEscEnv(t)
		owner := env.newOwner(env.chat)
		msgID := settingsEscOpenPrivate(t, env, owner)
		env.pressPrivate(owner, msgID, env.privateFields(env.chat.Id, "a", settingsActEscForget, "v", "90"))
		if got := settingsEscStored(t, env.chat.Id); got.ForgetDays != 90 {
			t.Errorf("forget days = %d, want 90", got.ForgetDays)
		}
	})
}

func TestSettingsEscalationForged(t *testing.T) {
	env := settingsEscEnv(t)
	owner := env.newOwner(env.chat)
	msgID := settingsEscOpenPrivate(t, env, owner)
	forged := [][]string{
		{"a", settingsActEscToggle, "v", "2"},
		{"a", settingsActEscToggle},
		{"a", settingsActEscForget, "v", "2"},
		{"a", settingsActEscForget, "v", "07"},
		{"a", settingsActEscStep, "t", "0"},
		{"a", settingsActEscStep, "t", "6"},
		{"a", settingsActEscStep, "t", "x"},
		{"a", settingsActEscSet, "t", "1", "v", "warn"},
		{"a", settingsActEscSet, "t", "1", "v", "tmute"},
		{"a", settingsActEscDuration, "t", "1", "k", "x"},
		{"a", settingsActEscTimed, "t", "1", "k", "m", "v", "7201"},
		{"a", settingsActEscTimed, "t", "1", "k", "b", "v", "0"},
		{"a", settingsActEscRemove, "t", "x"},
		{"a", settingsActEscStep, "t", "1", "x", "1"},
	}
	before := len(callsToChat(env.fake.staffBotClient, "getChatMember", env.chat.Id))
	for _, kv := range forged {
		env.pressPrivate(owner, msgID, env.privateFields(env.chat.Id, kv...))
		if answer, alert := env.lastAnswer(); !alert || !strings.Contains(answer, staffMarker("common_callback_invalid_request")) {
			t.Errorf("%v answered %q (alert %v), want the invalid-request alert", kv, answer, alert)
		}
	}
	// The same actions from a group menu have no owner field c.
	groupMsg := env.openMenu(owner)
	afterOpen := len(callsToChat(env.fake.staffBotClient, "getChatMember", env.chat.Id))
	for _, kv := range [][]string{
		{"a", settingsActEscToggle, "v", "1"}, {"a", settingsActEscForget, "v", "7"}, {"a", settingsActEscStep, "t", "1"},
		{"a", settingsActEscSet, "t", "1", "v", "ban"}, {"a", settingsActEscDuration, "t", "1", "k", "m"},
		{"a", settingsActEscTimed, "t", "1", "k", "m", "v", "3600"}, {"a", settingsActEscRemove, "t", "1"},
	} {
		env.press(owner, groupMsg, env.groupFields(owner, kv...))
		if answer, alert := env.lastAnswer(); !alert || !strings.Contains(answer, staffMarker("common_callback_invalid_request")) {
			t.Errorf("group %v answered %q (alert %v), want the invalid-request alert", kv, answer, alert)
		}
	}
	if got := len(callsToChat(env.fake.staffBotClient, "getChatMember", env.chat.Id)); got != afterOpen {
		t.Errorf("forged group presses made %d getChatMember calls", got-afterOpen)
	}
	_ = before
	if n := settingsEscRows(t, env.chat.Id); n != 0 {
		t.Errorf("forged presses wrote %d rows", n)
	}
}

func TestSettingsEscalationReadsCreateNothing(t *testing.T) {
	env := settingsEscEnv(t)
	owner := env.newOwner(env.chat)
	msgID := settingsEscOpenPrivate(t, env, owner)
	for _, kv := range [][]string{
		{"a", "bk"},
		{"a", "op", "p", settingsPageEscalation},
		{"a", settingsActEscStep, "t", "1"},
		{"a", settingsActEscDuration, "t", "1", "k", "m"},
	} {
		env.pressPrivate(owner, msgID, env.privateFields(env.chat.Id, kv...))
	}
	env.sendPrivate(owner, fmt.Sprintf("/start settings_%d", env.chat.Id))
	if n := settingsEscRows(t, env.chat.Id); n != 0 {
		t.Errorf("reading created %d rows", n)
	}
}
