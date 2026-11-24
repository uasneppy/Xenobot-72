//go:build testtools

package modules

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/blacklists"
	dbcache "github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
)

// settingsBlacklistTimedSeed stores one word per value of secs, all with action, in the order given, t1, t2, ...
func settingsBlacklistTimedSeed(t *testing.T, chatID int64, action string, secs ...int) {
	t.Helper()
	for i, s := range secs {
		if err := blacklists.AddBlacklistTimedContext(context.Background(), chatID, fmt.Sprintf("t%02d", i+1), action, s); err != nil {
			t.Fatalf("seed a %s word with %d seconds: %v", action, s, err)
		}
	}
}

// settingsBlacklistPresetOf finds the drawn duration preset of secs by its callback fields.
func settingsBlacklistPresetOf(t *testing.T, keyboard [][]gotgbot.InlineKeyboardButton, secs int) gotgbot.InlineKeyboardButton {
	t.Helper()
	for _, button := range settingsAllButtons(keyboard) {
		if f := settingsButtonFields(t, button); f["a"] == settingsActBlacklistTimed && f["v"] == strconv.Itoa(secs) {
			return button
		}
	}
	t.Fatalf("the duration screen has no preset of %d seconds", secs)
	return gotgbot.InlineKeyboardButton{}
}

// settingsBlacklistMarked lists the drawn buttons whose label starts with the selected prefix.
func settingsBlacklistMarked(keyboard [][]gotgbot.InlineKeyboardButton) []string {
	var marked []string
	for _, button := range settingsAllButtons(keyboard) {
		if strings.HasPrefix(button.Text, settingsSelectedPrefix) {
			marked = append(marked, button.Text)
		}
	}
	return marked
}

// settingsBlacklistTimedLine is the action line of a timed action in the marker locale.
func settingsBlacklistTimedLine(action string, secs int) string {
	tr := i18n.MustNewTranslator("en")
	word, _ := settingsTimedWord(tr, action, secs)
	return settingsText(tr, "settings_bl_action_line", i18n.TranslationParams{"action": word})
}

// TestSettingsBlacklistTimed drives the timed row of the Blacklist action screen (D-21): the screen has rows
// [3 2 2 2], Timed mute opens the duration screen, a preset stores the timed action and its seconds on every word,
// the action screen and the list page then read the timed word with Timed mute marked, words on different seconds
// read Mixed with a count line each, and a timed action stored with no duration reads as stored with the inert line.
func TestSettingsBlacklistTimed(t *testing.T) {
	t.Run("Timed mute and the 1 hour preset store tmute 3600 on every word", func(t *testing.T) {
		env := newSettingsListEnv(t)
		settingsListSeedBlacklist(t, env.chat.Id, "alpha", "beta")
		m := settingsBlacklistOpenMenu(env, env.chat)
		m.tap(m.button(staffMarker("settings_btn_bl_action")))
		_, keyboard := m.last()
		if got := settingsListRowWidths(keyboard); fmt.Sprint(got) != fmt.Sprint([]int{3, 2, 2, 2}) {
			t.Fatalf("action screen rows = %v, want [3 2 2 2]", got)
		}

		m.tap(m.button(staffMarker("settings_opt_tmute")))
		text, keyboard := m.last()
		if !strings.Contains(text, staffMarker("settings_duration_intro_tmute")) || !strings.Contains(text, staffMarker("settings_page_blacklist")) {
			t.Fatalf("Timed mute did not open the duration screen of the page:\n%s", text)
		}
		if marked := settingsBlacklistMarked(keyboard); len(marked) != 0 {
			t.Errorf("the duration screen marks %v for words on warn, want nothing", marked)
		}
		back := settingsAllButtons(keyboard)[len(settingsAllButtons(keyboard))-2]
		if f := settingsButtonFields(t, back); f["a"] != settingsActBlacklistOpen {
			t.Errorf("the duration screen's Back carries %v, want the action screen", f)
		}

		m.tap(settingsBlacklistPresetOf(t, keyboard, 3600))
		if got, want := blacklistTimedStored(t, env.chat.Id), []string{"alpha=tmute/3600", "beta=tmute/3600"}; !slices.Equal(got, want) {
			t.Fatalf("stored = %q, want %q", got, want)
		}
		text, keyboard = m.last()
		if !strings.Contains(text, staffMarker("settings_bl_action_intro")) || !strings.Contains(text, settingsBlacklistTimedLine("tmute", 3600)) {
			t.Errorf("the action screen after the preset does not read the timed word:\n%s", text)
		}
		if marked := settingsBlacklistMarked(keyboard); len(marked) != 1 || !strings.Contains(marked[0], staffMarker("settings_opt_tmute")) {
			t.Errorf("marked = %v, want Timed mute alone", marked)
		}
		if strings.Contains(text, staffMarker("settings_bl_action_inert")) {
			t.Errorf("an effective timed action drew the inert line:\n%s", text)
		}

		// The duration screen marks the stored preset on its own action only.
		m.tap(m.button(settingsSelectedPrefix + staffMarker("settings_opt_tmute")))
		if _, keyboard = m.last(); len(settingsBlacklistMarked(keyboard)) != 1 {
			t.Errorf("the Timed mute duration screen marks %v, want the 1 hour preset", settingsBlacklistMarked(keyboard))
		}
		m.press("a", settingsActBlacklistDuration, "k", settingsFloodTimedBan)
		if _, keyboard = m.last(); len(settingsBlacklistMarked(keyboard)) != 0 {
			t.Errorf("the Timed ban duration screen marks %v, want nothing", settingsBlacklistMarked(keyboard))
		}

		m.press("a", settingsActOpen, "p", settingsPageBlacklist)
		if text, _ = m.last(); !strings.Contains(text, settingsBlacklistTimedLine("tmute", 3600)) {
			t.Errorf("the list page does not read the timed word:\n%s", text)
		}

		// A plain choice clears the seconds.
		m.press("a", settingsActBlacklistSet, "v", "ban")
		if got, want := blacklistTimedStored(t, env.chat.Id), []string{"alpha=ban/0", "beta=ban/0"}; !slices.Equal(got, want) {
			t.Errorf("stored after Ban = %q, want %q", got, want)
		}
	})

	t.Run("words on different seconds read Mixed with a count line each", func(t *testing.T) {
		env := newSettingsListEnv(t)
		settingsBlacklistTimedSeed(t, env.chat.Id, "tmute", 3600, 21600)
		m := settingsBlacklistOpenMenu(env, env.chat)
		m.press("a", settingsActBlacklistOpen)
		text, keyboard := m.last()
		tr := i18n.MustNewTranslator("en")
		if !strings.Contains(text, staffMarker("settings_bl_action_line")+" "+staffMarker("settings_bl_action_mixed")) {
			t.Errorf("the screen does not read Mixed:\n%s", text)
		}
		for _, secs := range []int{3600, 21600} {
			word, _ := settingsTimedWord(tr, "tmute", secs)
			line := settingsText(tr, "settings_bl_action_count", i18n.TranslationParams{"action": word, "n": 1})
			if strings.Count(text, line) != 1 {
				t.Errorf("the count line %q is not drawn once:\n%s", line, text)
			}
		}
		if marked := settingsBlacklistMarked(keyboard); len(marked) != 0 {
			t.Errorf("a mixed list marks %v, want nothing", marked)
		}
	})

	t.Run("a stored tban with no duration reads as stored with the inert line", func(t *testing.T) {
		env := newSettingsListEnv(t)
		settingsBlacklistTimedSeed(t, env.chat.Id, "tban", 0)
		m := settingsBlacklistOpenMenu(env, env.chat)
		m.press("a", settingsActBlacklistOpen)
		text, keyboard := m.last()
		if !strings.Contains(text, staffMarker("settings_bl_action_line")+" tban") || !strings.Contains(text, staffMarker("settings_bl_action_inert")) {
			t.Errorf("the screen does not draw tban as stored with the inert line:\n%s", text)
		}
		if marked := settingsBlacklistMarked(keyboard); len(marked) != 0 {
			t.Errorf("an inert value marks %v, want nothing", marked)
		}
	})
}

// TestSettingsParityBlacklistTimed holds the timed press and /blacklistaction to one write: the same rows and the same
// blacklist cache state.
func TestSettingsParityBlacklistTimed(t *testing.T) {
	env := newSettingsListEnv(t)
	pageChat := settingsBlacklistWithBot(env, staffRoleAdmin)
	commandChat := settingsBlacklistWithBot(env, staffRoleAdmin)
	for _, chat := range []gotgbot.Chat{pageChat, commandChat} {
		settingsBlacklistSeedActions(t, chat.Id, "warn", "mute")
		blacklists.GetBlacklistSettingsContext(context.Background(), chat.Id)
	}
	m := settingsBlacklistOpenMenu(env, pageChat)
	m.press("a", settingsActBlacklistTimed, "k", settingsFloodTimedMute, "v", "3600")
	env.sendIn(commandChat, env.admin, "/blacklistaction tmute 1h")

	pageRows, commandRows := blacklistTimedStored(t, pageChat.Id), blacklistTimedStored(t, commandChat.Id)
	if !slices.Equal(pageRows, commandRows) || !slices.Equal(pageRows, []string{"w01=tmute/3600", "w02=tmute/3600"}) {
		t.Errorf("the page left %q and the command %q, want the same rows on tmute 3600", pageRows, commandRows)
	}
	pageKey, commandKey := dbcache.CacheKey("blacklist", pageChat.Id), dbcache.CacheKey("blacklist", commandChat.Id)
	if env.redis.Exists(pageKey) || env.redis.Exists(commandKey) {
		t.Errorf("cache state differs from dropped: page key present %v, command key present %v", env.redis.Exists(pageKey), env.redis.Exists(commandKey))
	}
}

// TestSettingsDriftBlacklistTimed holds the page to what the command stored, a duration no preset holds included, and
// the watcher to what the page stored.
func TestSettingsDriftBlacklistTimed(t *testing.T) {
	t.Run("a command's duration that is no preset is drawn and marks no preset", func(t *testing.T) {
		env := newSettingsListEnv(t)
		settingsListSeedBlacklist(t, env.chat.Id, "alpha")
		env.send(env.admin, "/blacklistaction tban 90m")
		m := settingsBlacklistOpenMenu(env, env.chat)
		if text, _ := m.last(); !strings.Contains(text, settingsBlacklistTimedLine("tban", 5400)) {
			t.Errorf("the list page does not draw tban 90 minutes:\n%s", text)
		}
		m.press("a", settingsActBlacklistDuration, "k", settingsFloodTimedBan)
		if _, keyboard := m.last(); len(settingsBlacklistMarked(keyboard)) != 0 {
			t.Errorf("the duration screen marks %v for 90 minutes, want no preset", settingsBlacklistMarked(keyboard))
		}
	})

	t.Run("a member who posts a word is muted for the page's duration", func(t *testing.T) {
		env := newSettingsListEnv(t)
		settingsListSeedBlacklist(t, env.chat.Id, "spam")
		now := time.Unix(1_900_000_000, 0)
		mreqSwap(t, &punishNow, func() time.Time { return now })
		member := gotgbot.User{Id: uniqueLinkOwnerID() + 45, FirstName: "Poster"}
		t.Cleanup(func() {
			db.DB.Where("user_id IN ?", []int64{member.Id}).Delete(&models.User{})
			dbcache.DeleteCache(dbcache.CacheKey("user", member.Id))
			dbcache.ResetLocalForTest()
		})
		env.fake.setMember(env.chat.Id, member.Id, staffFakeMember{Status: gotgbot.ChatMemberStatusMember})
		m := settingsBlacklistOpenMenu(env, env.chat)
		m.press("a", settingsActBlacklistTimed, "k", settingsFloodTimedMute, "v", "21600")
		if got := blacklistTimedStored(t, env.chat.Id); !slices.Equal(got, []string{"spam=tmute/21600"}) {
			t.Fatalf("stored = %q, want spam on tmute 21600", got)
		}

		if err := blacklistsModule.blacklistWatcher(env.bot, newModuleMessageContext(env.bot, env.chat, member, "this is spam")); err != ext.ContinueGroups {
			t.Fatalf("blacklistWatcher error = %v, want ContinueGroups", err)
		}
		var restricts []moduleBotCall
		for _, call := range env.fake.callsFor("restrictChatMember") {
			if staffParamInt(call.Params, "user_id") == member.Id {
				restricts = append(restricts, call)
			}
		}
		if len(restricts) != 1 {
			t.Fatalf("restrictChatMember calls about the member = %d, want 1", len(restricts))
		}
		if got, want := fmt.Sprint(restricts[0].Params["until_date"]), strconv.FormatInt(now.Unix()+21600, 10); got != want {
			t.Errorf("until_date = %s, want %s", got, want)
		}
	})
}

// TestSettingsAuthorityParityBlacklistTimed holds the timed press to what /blacklistaction refuses, with the ninth right's
// four refusals, the owner gate's documented carve-out, and the duration screen to the live admin check alone.
func TestSettingsAuthorityParityBlacklistTimed(t *testing.T) {
	scenarios := []struct {
		name      string
		botRole   string
		member    *staffFakeMember
		wantKey   string
		wantAlert bool
		ownerGate bool
	}{
		{name: "the bot is a member", botRole: staffRoleMember, wantKey: "connections_is_user_connected_bot_not_admin"},
		{name: "the bot is an administrator without can_restrict_members", botRole: staffRoleAdminNoRestrict, wantKey: "chat_status_bot_restrict_error"},
		{name: "an administrator without can_restrict_members", botRole: staffRoleAdmin,
			member: &staffFakeMember{Status: gotgbot.ChatMemberStatusAdministrator}, wantKey: "settings_no_restrict", wantAlert: true},
		{name: "a plain member", botRole: staffRoleAdmin,
			member: &staffFakeMember{Status: gotgbot.ChatMemberStatusMember}, wantKey: "settings_not_admin", wantAlert: true},
		{name: "the owner gate refuses a non-owner administrator", botRole: staffRoleAdmin, wantKey: "settings_gate_owner_only", wantAlert: true, ownerGate: true},
		{name: "every check held", botRole: staffRoleAdmin},
	}
	env := newSettingsListEnv(t)
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			cmdChat := settingsBlacklistWithBot(env, sc.botRole, "alpha")
			pageChat := settingsBlacklistWithBot(env, sc.botRole, "alpha")
			user := env.admin
			if sc.member != nil {
				user = gotgbot.User{Id: uniqueLinkOwnerID(), FirstName: "Presser"}
				env.fake.setMember(cmdChat.Id, user.Id, *sc.member)
				env.fake.setMember(pageChat.Id, user.Id, *sc.member)
			}
			if sc.ownerGate {
				env.newOwner(pageChat)
				env.setAdminsAllowed(pageChat.Id, false)
			}

			// The command refuses for the same member and bot records, or writes; the owner gate gates the menu only.
			env.sendIn(cmdChat, user, "/blacklistaction tmute 1h")
			commandWrites := sc.wantKey == "" || sc.ownerGate
			if got := slices.Equal(blacklistTimedStored(t, cmdChat.Id), []string{"alpha=tmute/3600"}); got != commandWrites {
				t.Errorf("the command wrote %v, want %v (%q)", got, commandWrites, blacklistTimedStored(t, cmdChat.Id))
			}

			m := settingsBlacklistOpenMenuAs(env, pageChat, user)
			before := blacklistTimedStored(t, pageChat.Id)
			answers, edits := env.answerCount(), len(env.edits(user.Id, m.msgID))
			m.press("a", settingsActBlacklistTimed, "k", settingsFloodTimedMute, "v", "3600")
			if got := env.answerCount() - answers; got != 1 {
				t.Errorf("the press was answered %d times, want once", got)
			}
			after := blacklistTimedStored(t, pageChat.Id)
			switch {
			case sc.wantKey == "":
				if !slices.Equal(after, []string{"alpha=tmute/3600"}) {
					t.Errorf("the permitted press left %q, want alpha on tmute 3600", after)
				}
				return
			case sc.wantAlert:
				if text, alert := env.lastAnswer(); !strings.Contains(text, staffMarker(sc.wantKey)) || !alert {
					t.Errorf("answered %q (alert %v), want the alert %s", text, alert, sc.wantKey)
				}
				if got := len(env.edits(user.Id, m.msgID)); got != edits {
					t.Errorf("an alert refusal edited the menu")
				}
			default:
				if text, _ := m.last(); !strings.Contains(text, settingsNoticeWarnIcon+staffMarker(sc.wantKey)) || !strings.Contains(text, staffMarker("settings_bl_action_intro")) {
					t.Errorf("the notice refusal did not redraw the action screen with %s:\n%s", sc.wantKey, text)
				}
			}
			if !slices.Equal(before, after) {
				t.Errorf("a refused press wrote: %q -> %q", before, after)
			}
			// The duration screen needs only the live admin check, so a bot that cannot restrict still opens it.
			if sc.member == nil && !sc.ownerGate {
				m.press("a", settingsActBlacklistDuration, "k", settingsFloodTimedMute)
				if text, _ := m.last(); !strings.Contains(text, staffMarker("settings_duration_intro_tmute")) {
					t.Errorf("the duration screen did not open:\n%s", text)
				}
			}
		})
	}
}

// TestSettingsBlacklistTimedForged refuses every forged bd and bz press in step 1, before any lookup.
func TestSettingsBlacklistTimedForged(t *testing.T) {
	env := newSettingsListEnv(t)
	settingsListSeedBlacklist(t, env.chat.Id, "alpha")
	m := settingsBlacklistOpenMenu(env, env.chat)
	groupMsg := env.openMenu(env.admin)
	refused := func(t *testing.T, name string, press func()) {
		t.Helper()
		members, chats := len(env.fake.callsFor("getChatMember")), len(env.fake.callsFor("getChat"))
		answers := env.answerCount()
		press()
		if got := env.answerCount() - answers; got != 1 {
			t.Errorf("%s: answered %d times, want once", name, got)
		}
		if text, alert := env.lastAnswer(); !strings.Contains(text, staffMarker("common_callback_invalid_request")) || !alert {
			t.Errorf("%s: answered %q (alert %v), want the invalid request alert", name, text, alert)
		}
		if got := len(env.fake.callsFor("getChatMember")) - members; got != 0 {
			t.Errorf("%s: made %d getChatMember calls, want none", name, got)
		}
		if got := len(env.fake.callsFor("getChat")) - chats; got != 0 {
			t.Errorf("%s: made %d getChat calls, want none", name, got)
		}
	}
	forged := map[string][]string{
		"bz v 1800 (no preset)": {"a", "bz", "k", "m", "v", "1800"},
		"bz v 0":                {"a", "bz", "k", "m", "v", "0"},
		"bz v 0600":             {"a", "bz", "k", "b", "v", "0600"},
		"bz v 600x":             {"a", "bz", "k", "m", "v", "600x"},
		"bz k x":                {"a", "bz", "k", "x", "v", "3600"},
		"bz no k":               {"a", "bz", "v", "3600"},
		"bz no v":               {"a", "bz", "k", "m"},
		"bz an extra key":       {"a", "bz", "k", "m", "v", "3600", "p", "b"},
		"bz with u as well":     {"a", "bz", "k", "m", "v", "3600", "u", "5"},
		"bd k z":                {"a", "bd", "k", "z"},
		"bd no k":               {"a", "bd"},
		"bd an extra key":       {"a", "bd", "k", "m", "v", "3600"},
	}
	names := slices.Sorted(maps.Keys(forged))
	for _, name := range names {
		refused(t, name, func() { m.press(forged[name]...) })
	}
	for _, kv := range [][]string{{"a", "bz", "k", "m", "v", "3600"}, {"a", "bd", "k", "m"}} {
		refused(t, kv[1]+" from a group menu", func() { env.press(env.admin, groupMsg, env.groupFields(env.admin, kv...)) })
	}
	if got := blacklistTimedStored(t, env.chat.Id); !slices.Equal(got, []string{"alpha=warn/0"}) {
		t.Errorf("a forged press changed the list: %q", got)
	}
}
