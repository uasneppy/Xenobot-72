//go:build testtools

package modules

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	logrustest "github.com/sirupsen/logrus/hooks/test"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/antiflood"
	"github.com/uasneppy/fuku_robot/fuku/db/blacklists"
	dbcache "github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/deletion"
	"github.com/uasneppy/fuku_robot/fuku/db/escalation"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
)

// escalationLadder stores a ladder through the repository and removes it, its counts and its cache
// key when the test ends.
func escalationLadder(t *testing.T, chatID int64, steps ...escalation.Step) {
	t.Helper()
	ctx := context.Background()
	t.Cleanup(func() {
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatEscalation{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatOffence{})
		dbcache.DeleteCache(escalation.CacheKey(chatID))
	})
	if len(steps) == 0 {
		if err := escalation.SetEnabled(ctx, chatID, true, 1); err != nil {
			t.Fatalf("SetEnabled: %v", err)
		}
		return
	}
	if err := escalation.SetEnabledWithSteps(ctx, chatID, steps, 1); err != nil {
		t.Fatalf("SetEnabledWithSteps: %v", err)
	}
}

// escalationOffenceRows reads the chat's stored offence counts.
func escalationOffenceRows(t *testing.T, chatID int64) []models.ChatOffence {
	t.Helper()
	var rows []models.ChatOffence
	if err := db.DB.Where("chat_id = ?", chatID).Find(&rows).Error; err != nil {
		t.Fatalf("read the offence rows: %v", err)
	}
	return rows
}

// escalationClock moves punishNow, the clock of the ladder's count and of the timed end dates.
func escalationClock(t *testing.T, start time.Time) *fwdClock {
	t.Helper()
	clock := &fwdClock{now: start}
	mreqSwap(t, &punishNow, clock.Now)
	return clock
}

// escalationBlacklist lists the word spam with action and secs for the site's chat.
func escalationBlacklist(t *testing.T, s *botSite, action string, secs int) {
	t.Helper()
	t.Cleanup(func() {
		if err := blacklists.RemoveAllBlacklist(s.chat.Id); err != nil {
			t.Errorf("RemoveAllBlacklist error = %v", err)
		}
		blacklistTimedDropCache(s.chat.Id)
	})
	if err := blacklists.AddBlacklistTimedContext(context.Background(), s.chat.Id, "spam", action, secs); err != nil {
		t.Fatalf("AddBlacklistTimedContext(%q, %d) error = %v", action, secs, err)
	}
	blacklistTimedDropCache(s.chat.Id)
}

// escalationMatch sends one matching message through the blacklist watcher with bot b.
func escalationMatch(t *testing.T, s *botSite, b *gotgbot.Bot, id int64, channel bool) {
	t.Helper()
	if err := blacklistsModule.blacklistWatcher(b, noticeGoldenMessage(s, id, "this has spam inside", channel)); err != ext.ContinueGroups {
		t.Fatalf("blacklistWatcher(message %d) error = %v, want ContinueGroups", id, err)
	}
}

// escalationUntil is the until_date of a recorded punishment call.
func escalationUntil(call moduleBotCall) int64 {
	return staffParamInt(call.Params, "until_date")
}

// escalationPunishCalls returns the recorded restrict and ban requests in order.
func escalationPunishCalls(s *botSite) []moduleBotCall {
	var out []moduleBotCall
	s.client.mu.Lock()
	defer s.client.mu.Unlock()
	for _, call := range s.client.calls {
		if call.Method == "restrictChatMember" || call.Method == "banChatMember" || call.Method == "banChatSenderChat" {
			out = append(out, call)
		}
	}
	return out
}

// escalationWarns returns the logged lines of the escalation resolver at Warn level or above.
func escalationWarns(hook *logrustest.Hook) []string {
	var lines []string
	for _, entry := range hook.AllEntries() {
		if strings.Contains(entry.Message, "[Escalation]") && entry.Level <= 3 {
			lines = append(lines, entry.Message)
		}
	}
	return lines
}

func escalationResetGates(t *testing.T) {
	t.Helper()
	reset := func() {
		for i := range escalationWarnGates {
			escalationWarnGates[i].mu.Lock()
			escalationWarnGates[i].last = nil
			escalationWarnGates[i].capWarned = false
			escalationWarnGates[i].mu.Unlock()
		}
	}
	reset()
	t.Cleanup(reset)
}

// TestEscalation is the escalation ladder end to end (TUNE-02, D-23 to D-30): the flood, the blacklist
// and the forward blocklist punish a member's 1st, 2nd and later offences by the group's ladder, an
// offence within 60 s counts once, a ladder that is off, empty or unreadable leaves each rule's own
// action, a count that fails applies step 1, and a sender chat, a warn and a delete-only action are
// never counted.
func TestEscalation(t *testing.T) {
	withModerationGoldenLocales(t)
	tr := i18n.MustNewTranslator("uk")
	start := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	ladder := []escalation.Step{{Action: "tmute", Secs: 3600}, {Action: "tmute", Secs: 86400}, {Action: "ban"}}

	t.Run("steps_in_order", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		clock := escalationClock(t, start)
		escalationLadder(t, s.chat.Id, ladder...)
		escalationBlacklist(t, s, "mute", 0)
		if err := db.DB.Model(&models.BlacklistSettings{}).Where("chat_id = ?", s.chat.Id).Update("reason", blacklistTimedReason).Error; err != nil {
			t.Fatal(err)
		}
		blacklistTimedDropCache(s.chat.Id)

		for i := 0; i < 4; i++ {
			escalationMatch(t, s, s.bot, int64(101+i), false)
			clock.advance(61 * time.Second)
		}
		calls := escalationPunishCalls(s)
		if len(calls) != 4 {
			t.Fatalf("punishment calls = %d, want 4: %+v", len(calls), calls)
		}
		if calls[0].Method != "restrictChatMember" || escalationUntil(calls[0]) != start.Unix()+3600 {
			t.Errorf("offence 1 = %s until %d, want a mute until %d", calls[0].Method, escalationUntil(calls[0]), start.Unix()+3600)
		}
		second := start.Add(61 * time.Second).Unix()
		if calls[1].Method != "restrictChatMember" || escalationUntil(calls[1]) != second+86400 {
			t.Errorf("offence 2 = %s until %d, want a mute until %d", calls[1].Method, escalationUntil(calls[1]), second+86400)
		}
		for i := 2; i < 4; i++ {
			if calls[i].Method != "banChatMember" || escalationUntil(calls[i]) != 0 {
				t.Errorf("offence %d = %s until %d, want a ban with no end date", i+1, calls[i].Method, escalationUntil(calls[i]))
			}
		}
		sent := s.client.callsFor("sendMessage")
		if len(sent) != 4 {
			t.Fatalf("notices = %d, want 4", len(sent))
		}
		if got, want := fmt.Sprint(sent[0].Params["text"]), blacklistTimedNotice(tr, "blacklists_bl_watcher_tmuted_user", 3600); got != want {
			t.Errorf("notice 1 = %q, want %q", got, want)
		}
		if got, want := fmt.Sprint(sent[1].Params["text"]), blacklistTimedNotice(tr, "blacklists_bl_watcher_tmuted_user", 86400); got != want {
			t.Errorf("notice 2 = %q, want %q", got, want)
		}
		if got := fmt.Sprint(sent[2].Params["text"]); got == fmt.Sprint(sent[1].Params["text"]) || strings.Contains(got, punishDurationLabel(tr, 86400)) {
			t.Errorf("notice 3 = %q, want the untimed ban notice", got)
		}
		if rows := escalationOffenceRows(t, s.chat.Id); len(rows) != 1 || rows[0].Offences != 4 {
			t.Errorf("offence rows = %+v, want one row of 4", rows)
		}
	})

	t.Run("flood_steps", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		clock := escalationClock(t, start)
		escalationLadder(t, s.chat.Id, escalation.Step{Action: "tmute", Secs: 1800}, escalation.Step{Action: "ban"})
		resetAntifloodState(t)
		t.Cleanup(func() {
			db.DB.Where("chat_id = ?", s.chat.Id).Delete(&models.AntifloodSettings{})
			dbcache.DeleteCache(dbcache.CacheKey("antiflood", s.chat.Id))
			resetAntifloodState(t)
		})
		if err := antiflood.SetFlood(s.chat.Id, 1); err != nil {
			t.Fatal(err)
		}
		if err := antiflood.SetFloodMode(s.chat.Id, "mute"); err != nil {
			t.Fatal(err)
		}
		id := int64(200)
		for offence := 0; offence < 2; offence++ {
			for k := 0; k < 2; k++ {
				id++
				if err := antifloodModule.checkFlood(s.bot, noticeGoldenMessage(s, id, "flood "+strconv.FormatInt(id, 10), false)); err != ext.ContinueGroups {
					t.Fatalf("checkFlood(%d) error = %v", id, err)
				}
			}
			clock.advance(61 * time.Second)
			resetAntifloodState(t)
		}
		calls := escalationPunishCalls(s)
		if len(calls) != 2 || calls[0].Method != "restrictChatMember" || escalationUntil(calls[0]) != start.Unix()+1800 || calls[1].Method != "banChatMember" {
			t.Fatalf("flood punishments = %+v, want a timed mute then a ban", calls)
		}
	})

	t.Run("same_offence", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		clock := escalationClock(t, start)
		escalationLadder(t, s.chat.Id, ladder...)
		escalationBlacklist(t, s, "mute", 0)
		escalationMatch(t, s, s.bot, 101, false)
		clock.advance(10 * time.Second)
		escalationMatch(t, s, s.bot, 102, false)
		calls := escalationPunishCalls(s)
		if len(calls) != 2 || calls[0].Method != "restrictChatMember" || calls[1].Method != "restrictChatMember" {
			t.Fatalf("punishments = %+v, want two timed mutes", calls)
		}
		if got, want := escalationUntil(calls[1])-escalationUntil(calls[0]), int64(10); got != want {
			t.Errorf("both offences got step 1 (3600 s from their own time), end dates differ by %d, want %d", got, want)
		}
		if rows := escalationOffenceRows(t, s.chat.Id); len(rows) != 1 || rows[0].Offences != 1 || rows[0].LastOffenceUnix != start.Unix() {
			t.Errorf("offence rows = %+v, want one row of 1 at the first offence", rows)
		}
	})

	t.Run("forget", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		clock := escalationClock(t, start)
		escalationLadder(t, s.chat.Id, ladder...)
		escalationBlacklist(t, s, "mute", 0)
		escalationMatch(t, s, s.bot, 101, false)
		clock.advance(61 * time.Second)
		escalationMatch(t, s, s.bot, 102, false)
		clock.advance(31 * 24 * time.Hour)
		escalationMatch(t, s, s.bot, 103, false)
		calls := escalationPunishCalls(s)
		if len(calls) != 3 {
			t.Fatalf("punishments = %d, want 3", len(calls))
		}
		if got, want := escalationUntil(calls[2]), clock.Now().Unix()+3600; got != want {
			t.Errorf("an offence after the forget period got until %d, want step 1 (%d)", got, want)
		}
	})

	t.Run("off", func(t *testing.T) {
		for _, name := range []string{"on with no step", "off"} {
			s := newNoticeGoldenSite(t, deletion.KindBotAuto)
			escalationClock(t, start)
			escalationLadder(t, s.chat.Id)
			if name == "off" {
				if err := escalation.SetEnabledWithSteps(context.Background(), s.chat.Id, ladder, 1); err != nil {
					t.Fatal(err)
				}
				if err := escalation.SetEnabled(context.Background(), s.chat.Id, false, 1); err != nil {
					t.Fatal(err)
				}
			}
			escalationBlacklist(t, s, "mute", 0)
			escalationMatch(t, s, s.bot, 101, false)
			calls := escalationPunishCalls(s)
			if len(calls) != 1 || calls[0].Method != "restrictChatMember" || escalationUntil(calls[0]) != 0 {
				t.Fatalf("%s: punishments = %+v, want the rule's own permanent mute", name, calls)
			}
			if rows := escalationOffenceRows(t, s.chat.Id); len(rows) != 0 {
				t.Errorf("%s: offence rows = %+v, want none", name, rows)
			}
		}
	})

	t.Run("unreadable_ladder", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		escalationClock(t, start)
		escalationResetGates(t)
		hook := logrustest.NewGlobal()
		t.Cleanup(hook.Reset)
		mreqSwap(t, &escalationLadderLookup, func(context.Context, int64) (*escalation.Ladder, error) {
			return nil, errors.New("secret-reason-" + "ladder")
		})
		escalationBlacklist(t, s, "mute", 0)
		escalationMatch(t, s, s.bot, 101, false)
		escalationMatch(t, s, s.bot, 102, false)
		calls := escalationPunishCalls(s)
		if len(calls) != 2 || calls[0].Method != "restrictChatMember" || escalationUntil(calls[0]) != 0 {
			t.Fatalf("punishments = %+v, want the rule's own mutes", calls)
		}
		lines := escalationWarns(hook)
		if len(lines) != 1 || !strings.Contains(lines[0], strconv.FormatInt(s.chat.Id, 10)) || strings.Contains(lines[0], "secret-reason") {
			t.Fatalf("warn lines = %q, want one naming the chat and not the error text", lines)
		}
	})

	t.Run("count_failed", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		escalationClock(t, start)
		escalationResetGates(t)
		hook := logrustest.NewGlobal()
		t.Cleanup(hook.Reset)
		escalationLadder(t, s.chat.Id, ladder...)
		mreqSwap(t, &escalationCount, func(context.Context, int64, int64, time.Time, time.Duration) (int, error) {
			return 0, errors.New("secret-reason-count")
		})
		escalationBlacklist(t, s, "mute", 0)
		escalationMatch(t, s, s.bot, 101, false)
		calls := escalationPunishCalls(s)
		if len(calls) != 1 || calls[0].Method != "restrictChatMember" || escalationUntil(calls[0]) != start.Unix()+3600 {
			t.Fatalf("punishments = %+v, want step 1, a timed mute of an hour", calls)
		}
		lines := escalationWarns(hook)
		user := strconv.FormatInt(noticeGoldenTarget.Id, 10)
		if len(lines) != 1 || !strings.Contains(lines[0], user) || !strings.Contains(lines[0], strconv.FormatInt(s.chat.Id, 10)) || strings.Contains(lines[0], "secret-reason") {
			t.Fatalf("warn lines = %q, want one naming the chat and the user only", lines)
		}
	})

	t.Run("sender_chat", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		escalationClock(t, start)
		escalationLadder(t, s.chat.Id, ladder...)
		escalationBlacklist(t, s, "ban", 0)
		escalationMatch(t, s, s.bot, 101, true)
		calls := escalationPunishCalls(s)
		if len(calls) != 1 || calls[0].Method != "banChatSenderChat" {
			t.Fatalf("punishments = %+v, want today's sender-chat ban", calls)
		}
		if rows := escalationOffenceRows(t, s.chat.Id); len(rows) != 0 {
			t.Errorf("offence rows = %+v, want none for a sender chat", rows)
		}
	})

	t.Run("not_offences", func(t *testing.T) {
		for _, action := range []string{"warn", "none"} {
			s := newNoticeGoldenSite(t, deletion.KindBotAuto)
			escalationClock(t, start)
			escalationLadder(t, s.chat.Id, ladder...)
			escalationBlacklist(t, s, action, 0)
			// A warn action ends the update through the warn module, so the result is not checked.
			_ = blacklistsModule.blacklistWatcher(s.bot, noticeGoldenMessage(s, 101, "this has spam inside", false))
			if calls := escalationPunishCalls(s); len(calls) != 0 {
				t.Errorf("%s: punishments = %+v, want none", action, calls)
			}
			if rows := escalationOffenceRows(t, s.chat.Id); len(rows) != 0 {
				t.Errorf("%s: offence rows = %+v, want none", action, rows)
			}
		}
	})

	t.Run("second_bot_instance", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		clock := escalationClock(t, start)
		escalationLadder(t, s.chat.Id, ladder...)
		escalationBlacklist(t, s, "mute", 0)
		other := &gotgbot.Bot{User: s.bot.User, BotClient: s.bot.BotClient}
		escalationMatch(t, s, s.bot, 101, false)
		clock.advance(61 * time.Second)
		escalationMatch(t, s, other, 102, false)
		calls := escalationPunishCalls(s)
		second := start.Add(61 * time.Second).Unix()
		if len(calls) != 2 || escalationUntil(calls[0]) != start.Unix()+3600 || escalationUntil(calls[1]) != second+86400 {
			t.Fatalf("punishments = %+v, want steps 1 and 2 across two bot instances", calls)
		}
	})

	t.Run("forward_album", func(t *testing.T) {
		env := newFwdWatchEnv(t, "mute")
		mreqSwap(t, &punishNow, env.clock.Now)
		env.fake.sendDate = func() int64 { return env.clock.Now().Unix() }
		escalationLadder(t, env.group, ladder...)
		member := env.member()
		for i := 0; i < 3; i++ {
			update := env.forwardUpdate(env.groupChat, &member, channelOrigin(env.source))
			update.Message.MediaGroupId = "album-esc"
			env.process(update)
		}
		restricts := env.callsTo("restrictChatMember", env.group)
		if len(restricts) != 1 || staffParamInt(restricts[0].Params, "until_date") != env.clock.Now().Unix()+3600 {
			t.Fatalf("restrict calls = %+v, want one timed mute of an hour for the whole album", restricts)
		}
		if rows := escalationOffenceRows(t, env.group); len(rows) != 1 || rows[0].Offences != 1 {
			t.Fatalf("offence rows = %+v, want one count", rows)
		}
		env.clock.advance(2 * forwardBlockPunishWindow)
		env.forward(member)
		if got := env.callsTo("restrictChatMember", env.group); len(got) != 2 ||
			staffParamInt(got[1].Params, "until_date") != env.clock.Now().Unix()+86400 {
			t.Fatalf("restrict calls = %+v, want step 2 for the next offence", got)
		}
		env.clock.advance(2 * forwardBlockPunishWindow)
		env.forward(member)
		if got := env.callsTo("banChatMember", env.group); len(got) != 1 || staffParamInt(got[0].Params, "until_date") != 0 {
			t.Fatalf("ban calls = %+v, want step 3, a ban", got)
		}
	})
}
