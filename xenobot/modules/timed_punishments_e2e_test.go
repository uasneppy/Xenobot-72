//go:build testtools

package modules

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/antiflood"
	dbcache "github.com/uasneppy/Xenobot-72/xenobot/db/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/db/escalation"
	"github.com/uasneppy/Xenobot-72/xenobot/db/eventmsg"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/db/warns"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
)

// Timed punishments, the escalation ladder and warn expiry end to end (plan 26-12; TUNE-01 to TUNE-03, the four
// success criteria of Phase 26). Everything is real but Telegram and the clock: the owner works the Warnings,
// Blacklist and Escalation pages of the private menu through the deep link and the buttons the bot drew, an
// administrator's /warn, /warns and /setfloodmode and the members' messages run through the real dispatcher with the Warns,
// Antiflood and Blacklists modules, and a second bot instance over the same database handles an offence. The clock is
// punishNow, moved by the test.

// timedE2EStart is the fixed time every subtest starts at.
var timedE2EStart = time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)

// timedE2E is Phase 23's autoMessagesE2E with a moveable punishNow and the rows of the phase removed when the test ends.
type timedE2E struct {
	*autoMessagesE2E
	clock *fwdClock
}

// newTimedE2E builds one environment for one subtest; real installs the real locale files, for a subtest that
// reads the real words or compares with the goldens recorded in Ukrainian.
func newTimedE2E(t *testing.T, real bool) *timedE2E {
	t.Helper()
	e := &timedE2E{autoMessagesE2E: newAutoMessagesE2E(t, real), clock: escalationClock(t, timedE2EStart)}
	escalationResetGates(t)
	t.Cleanup(func() {
		for _, model := range []any{&models.Warns{}, &models.WarnSettings{}, &models.AntifloodSettings{}, &models.ChatEscalation{}, &models.ChatOffence{}} {
			db.DB.Where("chat_id = ?", e.chat.Id).Delete(model)
		}
		dbcache.DeleteCache(escalation.CacheKey(e.chat.Id))
		dbcache.DeleteCache(dbcache.CacheKey("warn_settings", e.chat.Id))
		dbcache.DeleteCache(dbcache.CacheKey("antiflood", e.chat.Id))
		dbcache.ResetLocalForTest()
	})
	return e
}

// openPage opens the group's private menu for user and presses the page's cell on Main.
func (e *timedE2E) openPage(user gotgbot.User, page string) *messagesE2EMenu {
	e.t.Helper()
	m := e.openMain(user)
	m.tap("a", settingsActOpen, "p", page)
	return m
}

// setTimed presses the timed cell (k is m or b) of the open page's drawn row and then the preset of secs, the two
// steps of the shared duration screen. The page's own screen names the duration action in action.
func (e *timedE2E) setTimed(m *messagesE2EMenu, open, set, k string, secs int) {
	e.t.Helper()
	m.tap("a", open, "k", k)
	m.tap("a", set, "k", k, "v", fmt.Sprint(secs))
}

// punishCalls are the restrict and ban requests about member on env's bot in the group, in the order they were made.
func (e *timedE2E) punishCalls(env *settingsEnv, member gotgbot.User) []moduleBotCall {
	client := env.fake.moduleBotClient
	client.mu.Lock()
	defer client.mu.Unlock()
	var mine []moduleBotCall
	for _, call := range client.calls {
		if (call.Method == "restrictChatMember" || call.Method == "banChatMember") &&
			staffParamInt(call.Params, "chat_id") == e.chat.Id && staffParamInt(call.Params, "user_id") == member.Id {
			mine = append(mine, call)
		}
	}
	return mine
}

// wantPunishment fails unless the n-th punishment of member on env is method with until_date until (0 for no end).
func (e *timedE2E) wantPunishment(env *settingsEnv, member gotgbot.User, n int, method string, until int64, what string) {
	e.t.Helper()
	calls := e.punishCalls(env, member)
	if len(calls) <= n {
		e.t.Fatalf("%s: %d punishments, want at least %d", what, len(calls), n+1)
	}
	if calls[n].Method != method || escalationUntil(calls[n]) != until {
		e.t.Errorf("%s: punishment %d is %s until %d, want %s until %d", what, n, calls[n].Method, escalationUntil(calls[n]), method, until)
	}
}

// offenceRows counts the group's chat_offences rows.
func (e *timedE2E) offenceRows() int64 {
	e.t.Helper()
	var n int64
	if err := db.DB.Model(&models.ChatOffence{}).Where("chat_id = ?", e.chat.Id).Count(&n).Error; err != nil {
		e.t.Fatal(err)
	}
	return n
}

// floodBurst sends four messages of member, which tips a limit of 3.
func (e *timedE2E) floodBurst(env *settingsEnv, member gotgbot.User) {
	e.t.Helper()
	for i := range 4 {
		env.sendIn(e.chat, member, fmt.Sprintf("hello %d %d", i, e.clock.Now().Unix()))
	}
}

// secondReplica is a second bot over the same database with the Blacklists module, the group's bot an administrator
// and member known to it.
func (e *timedE2E) secondReplica(member gotgbot.User) *settingsEnv {
	e.t.Helper()
	replica := e.otherReplica()
	LoadBlacklists(replica.dispatcher)
	replica.fake.setBotRole(e.chat.Id, staffRoleAdmin)
	replica.fake.sendDate = func() int64 { return time.Now().Unix() }
	replica.fake.setProfile(member.Id, member.FirstName, member.LastName, "")
	replica.fake.setMember(e.chat.Id, member.Id, staffFakeMember{Status: gotgbot.ChatMemberStatusMember})
	client := newMessagesE2EClient(replica.bot.BotClient)
	replica.bot.BotClient = client
	client.register(member)
	return replica
}

func TestTimedPunishmentsEndToEnd(t *testing.T) {
	t.Run("warn_limit_page", func(t *testing.T) {
		e := newTimedE2E(t, true)
		tr := i18n.MustNewTranslator("uk")
		member := e.autoMember("Ann")
		if err := warns.SetWarnLimit(e.chat.Id, 2); err != nil {
			t.Fatal(err)
		}
		e.setAutoText("wl", "Out {first} for {duration} until {until}")

		// Warnings, Timed mute, 6 hours: three presses of buttons the bot drew.
		m := e.openPage(e.owner, settingsPageWarns)
		e.setTimed(m, settingsActWarnsDuration, settingsActWarnsTimed, settingsWarnsTimedMute, 21600)
		if got, err := warns.LoadWarnSettingsContext(context.Background(), e.chat.Id); err != nil || got.WarnMode != "tmute" || got.WarnModeSecs != 21600 {
			t.Fatalf("stored warn settings = %+v (%v), want tmute 21600", got, err)
		}

		e.command(e.settingsEnv, e.admin, "/warn spam", member)
		if calls := e.punishCalls(e.settingsEnv, member); len(calls) != 0 {
			t.Fatalf("the first warn punished: %+v", calls)
		}
		e.command(e.settingsEnv, e.admin, "/warn spam again", member)
		until := e.clock.Now().Unix() + 21600
		e.wantPunishment(e.settingsEnv, member, 0, "restrictChatMember", until, "the warn limit")
		want := "<b>Out</b> Ann for " + punishDurationLabel(tr, 21600) + " until " +
			formatChatTime(time.Unix(until, 0), chatClockOf(e.chat.Id), "02.01.2006 15:04")
		if got := fmt.Sprint(e.lastGroup().Params["text"]); got != want {
			t.Errorf("the custom warn limit message = %q, want %q", got, want)
		}
	})

	t.Run("flood_command", func(t *testing.T) {
		e := newTimedE2E(t, true)
		tr := i18n.MustNewTranslator("uk")
		member := e.autoMember("Ann")
		if err := antiflood.SetFlood(e.chat.Id, 3); err != nil {
			t.Fatal(err)
		}
		e.command(e.settingsEnv, e.admin, "/setfloodmode tban 1d", member)
		if got, err := antiflood.GetAntifloodSettingsContext(context.Background(), e.chat.Id); err != nil || got.Action != "tban" || got.ActionSecs != 86400 {
			t.Fatalf("stored antiflood settings = %+v (%v), want tban 86400", got, err)
		}
		e.floodBurst(e.settingsEnv, member)
		e.wantPunishment(e.settingsEnv, member, 0, "banChatMember", e.clock.Now().Unix()+86400, "the flood")
		if text := fmt.Sprint(e.lastGroup().Params["text"]); !strings.Contains(text, punishDurationLabel(tr, 86400)) {
			t.Errorf("the timed ban notice %q does not show the duration %q", text, punishDurationLabel(tr, 86400))
		}
	})

	t.Run("blacklist_page", func(t *testing.T) {
		e := newTimedE2E(t, true)
		member := e.autoMember("Ann")
		settingsListSeedBlacklistAction(t, e.chat.Id, "spam", "warn")
		m := e.openPage(e.owner, "b")
		m.tap("a", settingsActBlacklistOpen)
		e.setTimed(m, settingsActBlacklistDuration, settingsActBlacklistTimed, settingsWarnsTimedMute, 3600)

		id := e.memberSays(member, "buy spam now")
		if !e.deleted(id) {
			t.Error("the message with the word was not deleted")
		}
		e.wantPunishment(e.settingsEnv, member, 0, "restrictChatMember", e.clock.Now().Unix()+3600, "the blacklist")
	})

	t.Run("ladder", func(t *testing.T) {
		e := newTimedE2E(t, false)
		member := e.autoMember("Ann")
		settingsListSeedBlacklistAction(t, e.chat.Id, "spam", "mute")
		e.flood(3, "mute", false)
		replica := e.secondReplica(member)

		// The owner turns the ladder on from the Escalation page: the example ladder is stored.
		m := e.openPage(e.owner, settingsPageEscalation)
		m.tap("a", settingsActEscToggle, "v", "1")
		got, err := escalation.GetFresh(context.Background(), e.chat.Id)
		if err != nil || !got.Enabled || len(got.Steps) != 3 {
			t.Fatalf("stored ladder = %+v (%v), want the three example steps", got, err)
		}

		// Offence 1 (the flood) is a timed mute of 1 hour; offence 2, 61 s later and on the second bot instance
		// (a blacklist word), a timed mute of 1 day; offence 3 (the flood again), a ban.
		start := e.clock.Now().Unix()
		e.floodBurst(e.settingsEnv, member)
		e.wantPunishment(e.settingsEnv, member, 0, "restrictChatMember", start+3600, "offence 1")

		e.clock.advance(61 * time.Second)
		second := e.clock.Now().Unix()
		replica.sendIn(e.chat, member, "buy spam now")
		e.wantPunishment(replica, member, 0, "restrictChatMember", second+86400, "offence 2 on the second replica")

		e.clock.advance(61 * time.Second)
		e.floodBurst(e.settingsEnv, member)
		e.wantPunishment(e.settingsEnv, member, 1, "banChatMember", 0, "offence 3")

		// 31 days later the offences are forgotten: the next one is step 1 again.
		e.clock.advance(31 * 24 * time.Hour)
		later := e.clock.Now().Unix()
		before := len(e.punishCalls(e.settingsEnv, member))
		e.memberSays(member, "buy spam now")
		calls := e.punishCalls(e.settingsEnv, member)
		if len(calls) != before+1 || calls[len(calls)-1].Method != "restrictChatMember" || escalationUntil(calls[len(calls)-1]) != later+3600 {
			t.Fatalf("the offence after the forget period = %+v, want a timed mute of an hour", calls[before:])
		}

		// The owner turns the ladder off: the rule's own action applies and no count is written.
		m = e.openPage(e.owner, settingsPageEscalation)
		m.tap("a", settingsActEscToggle, "v", "0")
		if n := e.offenceRows(); n != 0 {
			t.Fatalf("turning the ladder off left %d offence rows", n)
		}
		e.clock.advance(time.Hour)
		before = len(e.punishCalls(e.settingsEnv, member))
		e.memberSays(member, "buy spam now")
		calls = e.punishCalls(e.settingsEnv, member)
		if len(calls) != before+1 || calls[len(calls)-1].Method != "restrictChatMember" || escalationUntil(calls[len(calls)-1]) != 0 {
			t.Fatalf("the offence with the ladder off = %+v, want the blacklist's own plain mute", calls[before:])
		}
		if n := e.offenceRows(); n != 0 {
			t.Errorf("an offence with the ladder off wrote %d rows", n)
		}
	})

	t.Run("warn_expiry", func(t *testing.T) {
		e := newTimedE2E(t, true)
		tr := i18n.MustNewTranslator("uk")
		a, b := e.autoMember("Ann"), e.autoMember("Bo")
		if err := warns.SetWarnLimit(e.chat.Id, 5); err != nil {
			t.Fatal(err)
		}
		// Two warns eight days ago and one now, for each of two members, stored while expiry is off.
		for _, member := range []gotgbot.User{a, b} {
			e.clock.advance(-8 * 24 * time.Hour)
			e.command(e.settingsEnv, e.admin, "/warn old one", member)
			e.command(e.settingsEnv, e.admin, "/warn old two", member)
			e.clock.advance(8 * 24 * time.Hour)
			e.command(e.settingsEnv, e.admin, "/warn new one", member)
		}

		m := e.openPage(e.owner, settingsPageWarns)
		m.tap("a", settingsActWarnsExpiryOpen)
		m.tap("a", settingsActWarnsExpiry, "v", "7")
		note, _ := tr.GetString("warns_expiry_note", i18n.TranslationParams{"duration": punishDurationLabel(tr, 7*86400)})

		e.command(e.settingsEnv, e.admin, "/warns", a)
		text := fmt.Sprint(e.lastGroup().Params["text"])
		if !strings.Contains(text, "new one") || strings.Contains(text, "old one") || !strings.Contains(text, note) {
			t.Errorf("/warns with the expiry on = %q, want the warn that counts and the expiry note only", text)
		}

		// With a limit of 2 the next warn reaches it by the counted warns only: 1 + 1.
		if err := warns.SetWarnLimit(e.chat.Id, 2); err != nil {
			t.Fatal(err)
		}
		e.command(e.settingsEnv, e.admin, "/warn once more", a)
		if got := e.punished(e.settingsEnv, "restrictChatMember", a); got != 1 {
			t.Fatalf("the limit restricted %d times, want once at the second counted warn", got)
		}

		// Back to Off: the second member's old warns count again, and no expiry note is shown.
		m = e.openPage(e.owner, settingsPageWarns)
		m.tap("a", settingsActWarnsExpiryOpen)
		m.tap("a", settingsActWarnsExpiry, "v", "0")
		e.command(e.settingsEnv, e.admin, "/warns", b)
		text = fmt.Sprint(e.lastGroup().Params["text"])
		if !strings.Contains(text, "old one") || !strings.Contains(text, "new one") || strings.Contains(text, note) {
			t.Errorf("/warns with the expiry off = %q, want every warn and no expiry note", text)
		}
	})

	t.Run("pm_until", func(t *testing.T) {
		e := newTimedE2E(t, false)
		member := e.autoMember("Ann")
		if err := antiflood.SetFlood(e.chat.Id, 3); err != nil {
			t.Fatal(err)
		}
		e.command(e.settingsEnv, e.admin, "/setfloodmode tmute 1h", member)
		e.switchAutoPM("fl", eventmsg.EventFlood)

		e.floodBurst(e.settingsEnv, member)
		e.wantPunishment(e.settingsEnv, member, 0, "restrictChatMember", e.clock.Now().Unix()+3600, "the flood")
		copies := e.privateSends(e.settingsEnv, member)
		one := pmCopiesE2EOne(t, copies, "sendMessage", "the member's private chat")
		if text := fmt.Sprint(one["text"]); !containsMarker(text, "eventmsg_pm_until") {
			t.Errorf("the private copy %q has no Until line", text)
		}
	})

	t.Run("staff_group", func(t *testing.T) {
		// A Staff Group's own menu has no Escalation cell.
		e := newTimedE2E(t, false)
		mreqSwap(t, &settingsEscStaffLookup, func(int64) (*models.StaffGroup, error) { return &models.StaffGroup{}, nil })
		main := e.openMain(e.owner)
		if _, found := settingsMainStateLine(main.text, "settings_page_esc"); found {
			t.Error("a Staff Group's Main has the Escalation line")
		}

		// A staff /ban fan-out to a linked group whose ladder is on writes no offence row: it is no offence.
		withFastPMCopyPacer(t)
		env := newStaffActionEnv(t, 1)
		group := env.groups[0]
		escalationLadder(t, group, escalation.Step{Action: "ban"})
		env.fake.setMember(group, staffTestTarget, staffFakeMember{Status: gotgbot.ChatMemberStatusMember})
		summary, _ := env.runStaffCommand(fmt.Sprintf("/ban %d spam", staffTestTarget))
		wantDoneLine(t, summary, "Group A")
		var rows int64
		if err := db.DB.Model(&models.ChatOffence{}).Where("chat_id = ?", group).Count(&rows).Error; err != nil {
			t.Fatal(err)
		}
		if rows != 0 {
			t.Errorf("a staff /ban wrote %d offence rows", rows)
		}
	})

	t.Run("nothing_set", func(t *testing.T) {
		e := newTimedE2E(t, true)
		warned, listed, flooder := e.autoMember("Ann & <Bo>"), e.autoMember("Ann & <Bo>"), e.autoMember("Ann & <Bo>")
		if err := warns.SetWarnLimit(e.chat.Id, 2); err != nil {
			t.Fatal(err)
		}
		settingsListSeedBlacklistAction(t, e.chat.Id, "spam", "mute")
		e.flood(3, "mute", false)

		e.command(e.settingsEnv, e.admin, "/warn first", warned)
		mark := settingsEditMarkSends(e.settingsEnv, e.chat.Id)
		id := e.command(e.settingsEnv, e.admin, "/warn second", warned)
		sends := e.groupSends(e.settingsEnv, mark)
		if len(sends) != 1 {
			t.Fatalf("the group got %+v at the warn limit, want the one default", sends)
		}
		e.wantGolden(sends[0], goldenNoticeWarnLimitMute, warned, id)

		mark = settingsEditMarkSends(e.settingsEnv, e.chat.Id)
		e.memberSays(listed, "buy spam now")
		if sends = e.groupSends(e.settingsEnv, mark); len(sends) != 1 {
			t.Fatalf("the group got %+v after the word, want the one default", sends)
		}
		e.wantGolden(sends[0], goldenNoticeBlacklistMute, listed, 0)

		mark = settingsEditMarkSends(e.settingsEnv, e.chat.Id)
		e.floodBurst(e.settingsEnv, flooder)
		if sends = e.groupSends(e.settingsEnv, mark); len(sends) != 1 {
			t.Fatalf("the group got %+v after the flood, want the one default", sends)
		}
		e.wantGolden(sends[0], goldenNoticeFloodMute, flooder, 0)

		for _, member := range []gotgbot.User{warned, listed, flooder} {
			for _, call := range e.punishCalls(e.settingsEnv, member) {
				if call.Method != "restrictChatMember" || escalationUntil(call) != 0 {
					t.Errorf("a group that set nothing punished %d with %s until %d, want a plain mute", member.Id, call.Method, escalationUntil(call))
				}
			}
		}
		if n := e.offenceRows(); n != 0 {
			t.Errorf("a group that set nothing wrote %d offence rows", n)
		}
	})
}
