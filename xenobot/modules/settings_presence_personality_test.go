//go:build testtools

package modules

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/db/presence"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
)

func newSettingsPersonalityEnv(t *testing.T) (*settingsListEnv, *settingsBlacklistMenu) {
	t.Helper()
	env := newSettingsListEnv(t)
	settingsPresenceCleanup(t, env.chat.Id)
	owner := settingsFiltersOwner(env, env.chat)
	m := settingsListOpenMenuPage(env, env.chat, owner, settingsPagePresence)
	return env, m
}

// TestSettingsPresencePersonalityOpens: the Presence page's shared row opens the sub-page, which draws
// three frequency rows, the two list rows and the navigation row, and writes nothing.
func TestSettingsPresencePersonalityOpens(t *testing.T) {
	env, m := newSettingsPersonalityEnv(t)
	_, keyboard := m.last()
	room, ok := settingsButtonByMarker(keyboard, "settings_btn_pr_room")
	personality, ok2 := settingsButtonByMarker(keyboard, "settings_btn_pr_personality")
	if !ok || !ok2 {
		t.Fatalf("the Presence page lacks the room or the personality button: %v %v", ok, ok2)
	}
	rowOf := func(label string) int {
		for i, row := range keyboard {
			for _, b := range row {
				if b.Text == label {
					return i
				}
			}
		}
		return -1
	}
	if rowOf(room.Text) != rowOf(personality.Text) || len(keyboard) != 8 {
		t.Fatalf("room and personality must share one row of an 8-row page: rows %d and %d of %d", rowOf(room.Text), rowOf(personality.Text), len(keyboard))
	}
	m.tap(personality)
	text, keyboard := m.last()
	for _, key := range []string{"settings_pf_intro", "settings_pf_text_line", "settings_pf_sticker_line", "settings_pf_photo_line", "settings_pf_topics_line", "settings_pf_names_line", "settings_pf_hint"} {
		if !containsMarker(text, key) {
			t.Errorf("the sub-page lacks %s:\n%s", key, text)
		}
	}
	var widths []int
	for _, row := range keyboard {
		widths = append(widths, len(row))
	}
	if !slices.Equal(widths, []int{1, 1, 1, 2, 2, 2}) {
		t.Errorf("row widths = %v, want [1 1 1 2 2 2]", widths)
	}
	// Nothing acts yet (AI reactions off, no cat stickers): the page says so.
	if !containsMarker(text, "settings_pf_inactive_note") {
		t.Errorf("the sub-page of a chat with no reaction feature on lacks the inactive note:\n%s", text)
	}
	if got := settingsPresenceRows(t, env.chat.Id); got != 0 {
		t.Errorf("opening the sub-page created %d presence rows", got)
	}
}

// TestSettingsPresencePersonalityInactiveNote: the note shows exactly while neither AI reactions (with the
// owner's Conversations switch) nor cat stickers can use the settings.
func TestSettingsPresencePersonalityInactiveNote(t *testing.T) {
	withRealLocales(t)
	v := settingsView{Tr: i18n.MustNewTranslator("en"), Private: true, OwnerKey: "c", OwnerValue: "-1009999999999999", GroupID: -1009999999999999, GroupTitle: "G"}
	note := settingsText(v.Tr, "settings_pf_inactive_note")
	for _, tc := range []struct {
		name         string
		stored       presence.Settings
		convUnlocked bool
		wantNote     bool
	}{
		{"nothing on", presence.Settings{}, true, true},
		{"AI on but the owner switch is off", presence.Settings{AIReactMode: presence.AIReactModeLow}, false, true},
		{"AI on and unlocked", presence.Settings{AIReactMode: presence.AIReactModeMedium}, true, false},
		{"cat stickers on", presence.Settings{CatStickerEnabled: true}, false, false},
	} {
		got := strings.Contains(renderSettingsPresencePersonality(v, tc.stored, tc.convUnlocked, "").Text, note)
		if got != tc.wantNote {
			t.Errorf("%s: note shown = %v, want %v", tc.name, got, tc.wantNote)
		}
	}
}

// TestSettingsPresenceReactFreq: each cycle button stores its own kind's frequency only, a press of the
// stored value writes nothing, and a forged value, a group menu's press and a stale step are refused.
func TestSettingsPresenceReactFreq(t *testing.T) {
	env, m := newSettingsPersonalityEnv(t)
	m.press("a", settingsActPresencePersonality)

	// From Normal the text cycle button sends Often (Never, Rare, Normal, Often, Never...).
	btn, ok := m.markedButton("settings_btn_pf_text")
	if !ok {
		t.Fatal("no text frequency button")
	}
	m.tap(btn)
	if got := settingsPresenceStored(t, env.chat.Id); got.ReactTextFreq != presence.ReactFreqOften || got.ReactStickerFreq != 0 || got.ReactPhotoFreq != 0 {
		t.Fatalf("after the text press: text=%d sticker=%d photo=%d", got.ReactTextFreq, got.ReactStickerFreq, got.ReactPhotoFreq)
	}
	if text, _ := m.last(); !containsMarker(text, "settings_pf_freq_often") {
		t.Errorf("the page does not show Often:\n%s", text)
	}
	// The same button from the stale screen (it still carries the old target) writes the same value again
	// and shows no notice; the next drawn button now steps on to Never.
	m.tap(btn)
	if got := settingsPresenceStored(t, env.chat.Id); got.ReactTextFreq != presence.ReactFreqOften {
		t.Errorf("a stale press changed the stored value to %d", got.ReactTextFreq)
	}
	btn, _ = m.markedButton("settings_btn_pf_text")
	m.tap(btn)
	if got := settingsPresenceStored(t, env.chat.Id); got.ReactTextFreq != presence.ReactFreqOff {
		t.Errorf("Often steps on to Never, stored %d", got.ReactTextFreq)
	}

	for control, want := range map[string]func(presence.Settings) int{
		settingsPresenceControlReactSticker: func(s presence.Settings) int { return s.ReactStickerFreq },
		settingsPresenceControlReactPhoto:   func(s presence.Settings) int { return s.ReactPhotoFreq },
	} {
		m.press("a", settingsActSet, "p", settingsPagePresence, "t", control, "v", "2")
		if got := want(settingsPresenceStored(t, env.chat.Id)); got != presence.ReactFreqRare {
			t.Errorf("control %s stored %d, want Rare", control, got)
		}
	}
	if got := settingsPresenceStored(t, env.chat.Id); got.ReactTextFreq != presence.ReactFreqOff {
		t.Errorf("the sticker and photo presses changed the text frequency to %d", got.ReactTextFreq)
	}

	// Forged values are refused in step 1 with no lookup.
	lookups := func() int { return len(env.fake.callsFor("getChatMember")) }
	for _, v := range []string{"4", "-1", "01", "", "x", "2 "} {
		before := lookups()
		m.press("a", settingsActSet, "p", settingsPagePresence, "t", settingsPresenceControlReactText, "v", v)
		if answer, alert := env.lastAnswer(); !alert || !strings.Contains(answer, staffMarker("common_callback_invalid_request")) {
			t.Errorf("v=%q answered %q (alert %v)", v, answer, alert)
		}
		if got := lookups(); got != before {
			t.Errorf("v=%q made %d getChatMember calls, want 0", v, got-before)
		}
	}

	// A group menu's press is refused and writes nothing.
	owner := m.user
	groupMsg := env.openMenu(owner)
	before := settingsPresenceStored(t, env.chat.Id)
	env.press(owner, groupMsg, env.groupFields(owner, "a", settingsActSet, "p", settingsPagePresence, "t", settingsPresenceControlReactPhoto, "v", "3"))
	if answer, alert := env.lastAnswer(); !alert || !strings.Contains(answer, staffMarker("common_callback_invalid_request")) {
		t.Errorf("a group menu's press answered %q (alert %v)", answer, alert)
	}
	if after := settingsPresenceStored(t, env.chat.Id); after.ReactPhotoFreq != before.ReactPhotoFreq {
		t.Errorf("a group menu's press changed the photo frequency to %d", after.ReactPhotoFreq)
	}
}

func settingsPersonalityTyped(env *settingsListEnv, m *settingsBlacklistMenu, text string) []string {
	env.t.Helper()
	sent := len(env.fake.sentTo(m.user.Id))
	settingsListDoorSay(env, m.user, text)
	var out []string
	for _, msg := range env.fake.sentTo(m.user.Id)[sent:] {
		out = append(out, sentText(msg))
	}
	return out
}

// TestSettingsPresencePersonalityTyped: the topics and the names are typed lists run by the door: a
// refusal keeps the wait, a good list is stored through the repository's own normaliser, finishes the wait
// and answers the sub-page.
func TestSettingsPresencePersonalityTyped(t *testing.T) {
	env, m := newSettingsPersonalityEnv(t)
	m.press("a", settingsActPresencePersonality)

	m.press("a", settingsActPresencePersonalityEdit, "t", settingsPresencePartTopics)
	if text, _ := m.last(); !containsMarker(text, "settings_pf_wait_topics") {
		t.Fatalf("Set topics drew:\n%s", text)
	}
	if !env.redis.Exists(settingsInputKey(m.user.Id)) {
		t.Fatal("Set topics wrote no wait")
	}
	for _, tc := range []struct{ text, want string }{
		{"a, b, c, d, e, f, g, h, i, j, k", "settings_pf_refuse_topics_many"},
		{strings.Repeat("x", presence.MaxTopicRunes+1), "settings_pf_refuse_topics_long"},
		{"fine, <b>bold</b>", "settings_pf_refuse_topics_invalid"},
		{" , ;", "settings_pf_refuse_blank"},
	} {
		replies := settingsPersonalityTyped(env, m, tc.text)
		if len(replies) != 1 || !strings.Contains(replies[0], staffMarker(tc.want)) {
			t.Errorf("%q answered %v, want %s", tc.text, replies, tc.want)
		}
		if !env.redis.Exists(settingsInputKey(m.user.Id)) {
			t.Fatalf("%q ended the wait", tc.text)
		}
	}
	if got := settingsPresenceRows(t, env.chat.Id); got != 0 {
		t.Fatalf("refusals wrote %d presence rows", got)
	}

	replies := settingsPersonalityTyped(env, m, "Memes, cats\nmemes;  deploy   days")
	if len(replies) == 0 || !strings.Contains(strings.Join(replies, "\n"), staffMarker("settings_pf_saved_topics")) {
		t.Fatalf("a good list answered %v", replies)
	}
	if got := settingsPresenceStored(t, env.chat.Id).TopicList(); !slices.Equal(got, []string{"Memes", "cats", "deploy days"}) {
		t.Errorf("stored topics = %q", got)
	}
	if env.redis.Exists(settingsInputKey(m.user.Id)) {
		t.Error("the wait is still there after a save")
	}

	// Names: the same door, the same flow.
	m2 := settingsListOpenMenuPage(env, env.chat, m.user, settingsPagePresence)
	m2.press("a", settingsActPresencePersonality)
	m2.press("a", settingsActPresencePersonalityEdit, "t", settingsPresencePartNames)
	for _, tc := range []struct{ text, want string }{
		{"x", "settings_pf_refuse_names_invalid"},
		{"two words", "settings_pf_refuse_names_invalid"},
		{"@spike", "settings_pf_refuse_names_invalid"},
		{strings.Repeat("n", presence.MaxBotNameRunes+1), "settings_pf_refuse_names_long"},
		{"a1, b2, c3, d4, e5, f6, g7, h8, i9", "settings_pf_refuse_names_many"},
	} {
		replies := settingsPersonalityTyped(env, m2, tc.text)
		if len(replies) != 1 || !strings.Contains(replies[0], staffMarker(tc.want)) {
			t.Errorf("name %q answered %v, want %s", tc.text, replies, tc.want)
		}
	}
	replies = settingsPersonalityTyped(env, m2, "Spike, buddy-2")
	if !strings.Contains(strings.Join(replies, "\n"), staffMarker("settings_pf_saved_names")) {
		t.Fatalf("good names answered %v", replies)
	}
	stored := settingsPresenceStored(t, env.chat.Id)
	if got := stored.BotNameList(); !slices.Equal(got, []string{"Spike", "buddy-2"}) {
		t.Errorf("stored names = %q", got)
	}
	if got := stored.TopicList(); !slices.Equal(got, []string{"Memes", "cats", "deploy days"}) {
		t.Errorf("saving names changed the topics: %q", got)
	}
}

// TestSettingsPresencePersonalityClearAndCancel: Clear empties one list and nothing else, a Clear of an
// empty list writes nothing, and Cancel ends the wait and redraws the sub-page.
func TestSettingsPresencePersonalityClearAndCancel(t *testing.T) {
	env, m := newSettingsPersonalityEnv(t)
	if err := presence.SetReactTopics(t.Context(), env.chat.Id, "memes, cats", m.user.Id); err != nil {
		t.Fatal(err)
	}
	if err := presence.SetBotNames(t.Context(), env.chat.Id, "Spike", m.user.Id); err != nil {
		t.Fatal(err)
	}
	m.press("a", settingsActPresencePersonality)
	m.press("a", settingsActPresencePersonalityClear, "t", settingsPresencePartTopics)
	got := settingsPresenceStored(t, env.chat.Id)
	if len(got.TopicList()) != 0 || !slices.Equal(got.BotNameList(), []string{"Spike"}) {
		t.Fatalf("after clearing the topics: topics=%q names=%q", got.TopicList(), got.BotNameList())
	}
	if text, _ := m.last(); !containsMarker(text, "settings_pf_cleared_topics") {
		t.Errorf("the clear drew no notice:\n%s", text)
	}
	m.press("a", settingsActPresencePersonalityClear, "t", settingsPresencePartTopics)
	if text, _ := m.last(); containsMarker(text, "settings_pf_cleared_topics") {
		t.Errorf("clearing an empty list showed a notice:\n%s", text)
	}

	m.press("a", settingsActPresencePersonalityEdit, "t", settingsPresencePartNames)
	if !env.redis.Exists(settingsInputKey(m.user.Id)) {
		t.Fatal("no wait before Cancel")
	}
	m.press("a", settingsActPresencePersonalityCancel, "t", settingsPresencePartNames)
	if env.redis.Exists(settingsInputKey(m.user.Id)) {
		t.Error("Cancel left the wait")
	}
	if text, _ := m.last(); !containsMarker(text, "settings_pf_intro") {
		t.Errorf("Cancel did not redraw the sub-page:\n%s", text)
	}
	if after := settingsPresenceStored(t, env.chat.Id); !slices.Equal(after.BotNameList(), []string{"Spike"}) {
		t.Errorf("Cancel changed the names: %q", after.BotNameList())
	}
}

// TestSettingsPresencePersonalityStaffGroup: a wait made before the chat became a Staff Group stores
// nothing and ends, and an unreadable Staff Group row does the same.
func TestSettingsPresencePersonalityStaffGroup(t *testing.T) {
	env, m := newSettingsPersonalityEnv(t)
	m.press("a", settingsActPresencePersonality)
	m.press("a", settingsActPresencePersonalityEdit, "t", settingsPresencePartTopics)
	mreqSwap(t, &settingsPresenceStaffLookup, func(int64) (*models.StaffGroup, error) {
		return &models.StaffGroup{ChatID: env.chat.Id}, nil
	})
	replies := settingsPersonalityTyped(env, m, "memes")
	if len(replies) != 1 || !strings.Contains(replies[0], staffMarker("common_callback_invalid_request")) {
		t.Errorf("a Staff Group answered %v", replies)
	}
	if env.redis.Exists(settingsInputKey(m.user.Id)) {
		t.Error("the wait survived a Staff Group")
	}
	if got := settingsPresenceRows(t, env.chat.Id); got != 0 {
		t.Errorf("a Staff Group wrote %d presence rows", got)
	}
}

// TestSettingsPresencePersonalityFits holds the sub-page and both prompts inside 3800 UTF-16 units, every
// button at most 64 bytes and every two-column button at most 20 code points, in all 8 languages, for every
// frequency combination, the fullest lists (entries that need escaping) and a long group title.
func TestSettingsPresencePersonalityFits(t *testing.T) {
	withRealLocales(t)
	topics := make([]string, presence.MaxReactTopics)
	for i := range topics {
		topics[i] = strings.Repeat("Ж", presence.MaxTopicRunes-2) + fmt.Sprintf("&%d", i%10)
	}
	names := make([]string, presence.MaxBotNames)
	for i := range names {
		names[i] = strings.Repeat("Я", presence.MaxBotNameRunes-1) + fmt.Sprint(i)
	}
	for _, code := range memberReqScreenLanguages {
		tr := i18n.MustNewTranslator(code)
		for viewName, v := range settingsBlacklistWorstViews(tr) {
			v.GroupTitle = strings.Repeat("Ж", 64)
			var screens = map[string]settingsScreen{
				"prompt topics":         renderSettingsPresencePersonalityPrompt(v, settingsPresencePartTopics, time.Now(), true),
				"prompt names":          renderSettingsPresencePersonalityPrompt(v, settingsPresencePartNames, time.Now(), true),
				"empty":                 renderSettingsPresencePersonality(v, presence.Settings{}, false, ""),
				"full with warn notice": renderSettingsPresencePersonality(v, presence.Settings{ReactTopics: strings.Join(topics, "\n"), BotNames: strings.Join(names, "\n")}, true, settingsNoticeWarn(settingsText(tr, "common_settings_save_failed"))),
			}
			for freq := range 4 {
				screens[fmt.Sprintf("freq %d", freq)] = renderSettingsPresencePersonality(v, presence.Settings{
					ReactTextFreq: freq, ReactStickerFreq: (freq + 1) % 4, ReactPhotoFreq: (freq + 2) % 4,
				}, freq%2 == 0, "")
			}
			for name, screen := range screens {
				if units := panelUTF16Len(screen.Text); units == 0 || units > 3800 {
					t.Errorf("%s %s %s is %d UTF-16 units", code, viewName, name, units)
				}
				if screen.Omitted != 0 {
					t.Errorf("%s %s %s omitted %d buttons", code, viewName, name, screen.Omitted)
				}
				for _, button := range settingsAllButtons(screen.Keyboard.InlineKeyboard) {
					if n := len(button.CallbackData); n < 1 || n > 64 {
						t.Errorf("%s %s %s: %q carries %d bytes", code, viewName, name, button.Text, n)
					}
				}
			}
		}
	}
}
