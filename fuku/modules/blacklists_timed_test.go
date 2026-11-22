//go:build testtools

package modules

import (
	"context"
	"fmt"
	"html"
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
	"github.com/uasneppy/fuku_robot/fuku/db/deletion"
	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
	"github.com/uasneppy/fuku_robot/fuku/utils/formatting"
)

// blacklistTimedStored reads the chat's words straight from the table as word=action/secs, sorted.
func blacklistTimedStored(t *testing.T, chatID int64) []string {
	t.Helper()
	var rows []models.BlacklistSettings
	if err := db.DB.Where("chat_id = ?", chatID).Find(&rows).Error; err != nil {
		t.Fatalf("read blacklists: %v", err)
	}
	words := make([]string, 0, len(rows))
	for _, row := range rows {
		words = append(words, fmt.Sprintf("%s=%s/%d", row.Word, row.Action, row.ActionSecs))
	}
	slices.Sort(words)
	return words
}

// TestApplyBlacklistActionTimed holds the one write path of the blacklist action with a duration (D-09): a valid
// timed action stores both columns on every word, a plain action stores seconds 0, a timed action with no
// duration is the pinned unknown outcome, a duration outside 60 s to 366 days is the bad-duration outcome,
// neither refusal writes, and a chat with no words is the no-words outcome.
func TestApplyBlacklistActionTimed(t *testing.T) {
	ctx := context.Background()
	chatID := blacklistApplyChat(t)
	blacklistApplySeed(t, chatID, "warn", "spam", "eggs")

	if got := applyBlacklistAction(ctx, chatID, "tmute", 7200); got.Outcome != blacklistActionApplied || got.Rows != 2 {
		t.Fatalf("applyBlacklistAction(tmute, 7200) = %+v, want applied over 2 rows", got)
	}
	if got, want := blacklistTimedStored(t, chatID), []string{"eggs=tmute/7200", "spam=tmute/7200"}; !slices.Equal(got, want) {
		t.Fatalf("stored = %q, want %q", got, want)
	}

	// A plain action stores seconds 0, also when it is handed seconds.
	if got := applyBlacklistAction(ctx, chatID, "ban", 3600); got.Outcome != blacklistActionApplied {
		t.Fatalf("applyBlacklistAction(ban, 3600) = %+v, want applied", got)
	}
	if got, want := blacklistTimedStored(t, chatID), []string{"eggs=ban/0", "spam=ban/0"}; !slices.Equal(got, want) {
		t.Fatalf("stored after ban = %q, want %q", got, want)
	}

	for _, c := range []struct {
		action string
		secs   int
		want   blacklistActionOutcome
	}{
		{"tban", 0, blacklistActionUnknown},
		{"tmute", 0, blacklistActionUnknown},
		{"tmute", 59, blacklistActionBadDuration},
		{"tban", 31622401, blacklistActionBadDuration},
		{"tban", -1, blacklistActionBadDuration},
		{"delete", 3600, blacklistActionUnknown},
	} {
		if got := applyBlacklistAction(ctx, chatID, c.action, c.secs); got.Outcome != c.want || got.Rows != 0 || got.Err != nil {
			t.Errorf("applyBlacklistAction(%s, %d) = %+v, want outcome %v and nothing written", c.action, c.secs, got, c.want)
		}
	}
	if got, want := blacklistTimedStored(t, chatID), []string{"eggs=ban/0", "spam=ban/0"}; !slices.Equal(got, want) {
		t.Fatalf("a refusal changed the rows: %q, want %q", got, want)
	}

	empty := blacklistApplyChat(t)
	if got := applyBlacklistAction(ctx, empty, "tban", 604800); got.Outcome != blacklistActionNoWords || got.Rows != 0 {
		t.Errorf("a chat with no words = %+v, want no words over 0 rows", got)
	}
}

// TestBlacklistActionTimedCommand drives /blacklistaction's timed form and /addblacklist through the handlers in
// Ukrainian with the real locales: the reply names the short duration, every word stores the seconds, a bad
// duration writes nothing, the current-mode reply shows the short form, and a new word takes the shared action
// and seconds, or warn and 0 once the words are mixed (A19-3 extended, D-21).
func TestBlacklistActionTimedCommand(t *testing.T) {
	withModerationGoldenLocales(t)
	tr := i18n.MustNewTranslator("uk")
	s := newNoticeGoldenSite(t, deletion.KindBotManual)
	timedGoldenCleanup(t, s.chat.Id)
	blacklistApplySeed(t, s.chat.Id, "warn", "spam", "eggs")
	changed := func(shown string) string {
		temp, _ := tr.GetString("blacklists_set_bl_action_changed_mode")
		return fmt.Sprintf(temp, shown)
	}

	timedGoldenCommand(t, s, blacklistsModule.setBlacklistAction, timedGoldenAdmin, "/blacklistaction tmute 2h", false)
	if got, want := warnTimedLastText(t, s), changed("tmute 2h"); got != want {
		t.Errorf("tmute 2h: reply = %q, want %q", got, want)
	}
	if got, want := blacklistTimedStored(t, s.chat.Id), []string{"eggs=tmute/7200", "spam=tmute/7200"}; !slices.Equal(got, want) {
		t.Fatalf("stored after tmute 2h = %q, want %q", got, want)
	}

	timedGoldenCommand(t, s, blacklistsModule.setBlacklistAction, timedGoldenAdmin, "/blacklistaction TBAN 1w", false)
	if got, want := warnTimedLastText(t, s), changed("tban 1w"); got != want {
		t.Errorf("TBAN 1w: reply = %q, want %q", got, want)
	}
	if got, want := blacklistTimedStored(t, s.chat.Id), []string{"eggs=tban/604800", "spam=tban/604800"}; !slices.Equal(got, want) {
		t.Fatalf("stored after TBAN 1w = %q, want %q", got, want)
	}

	bad, _ := tr.GetString("blacklists_set_bl_action_bad_duration")
	for _, text := range []string{"/blacklistaction tmute 0m", "/blacklistaction tban xyz", "/blacklistaction tmute 400d"} {
		timedGoldenCommand(t, s, blacklistsModule.setBlacklistAction, timedGoldenAdmin, text, false)
		if got := warnTimedLastText(t, s); got != bad {
			t.Errorf("%s: reply = %q, want %q", text, got, bad)
		}
	}
	if got, want := blacklistTimedStored(t, s.chat.Id), []string{"eggs=tban/604800", "spam=tban/604800"}; !slices.Equal(got, want) {
		t.Fatalf("a bad duration changed the rows: %q, want %q", got, want)
	}

	timedGoldenCommand(t, s, blacklistsModule.setBlacklistAction, timedGoldenAdmin, "/blacklistaction", false)
	current, _ := tr.GetString("blacklists_set_bl_action_current_mode")
	if got, want := warnTimedLastText(t, s), fmt.Sprintf(current, "tban 1w"); got != want {
		t.Errorf("current mode = %q, want %q", got, want)
	}

	timedGoldenCommand(t, s, blacklistsModule.addBlacklist, timedGoldenAdmin, "/addblacklist egg", false)
	if got := blacklistTimedStored(t, s.chat.Id); !slices.Contains(got, "egg=tban/604800") {
		t.Fatalf("stored after /addblacklist egg = %q, want egg with tban and 604800", got)
	}

	// Mixed seconds give a new word warn and 0.
	if err := db.DB.Model(&models.BlacklistSettings{}).Where("chat_id = ? AND word = ?", s.chat.Id, "egg").
		Update("action_secs", 3600).Error; err != nil {
		t.Fatalf("mix the seconds: %v", err)
	}
	blacklistTimedDropCache(s.chat.Id)
	timedGoldenCommand(t, s, blacklistsModule.addBlacklist, timedGoldenAdmin, "/addblacklist ham", false)
	if got := blacklistTimedStored(t, s.chat.Id); !slices.Contains(got, "ham=warn/0") {
		t.Fatalf("stored after /addblacklist ham on mixed words = %q, want ham with warn and 0", got)
	}
}

// blacklistTimedDropCache drops the chat's cached blacklist after a row was written straight to the table.
func blacklistTimedDropCache(chatID int64) {
	dbcache.DeleteCache(dbcache.CacheKey("blacklist", chatID))
	dbcache.ResetLocalForTest()
}

// blacklistTimedShtmlReply is the reply parameters of formatting.Shtml(), the options of every default blacklist
// notice, timed or not.
const blacklistTimedShtmlReply = `{"allow_sending_without_reply":true}`

// blacklistTimedReason is the stored reason of the timed watcher cases: markup that the timed default must escape.
const blacklistTimedReason = "Bad <b>%s</b>"

// blacklistTimedRun is noticeGoldenBlacklist with a duration and a reason holding markup: the word "spam" stored
// with action and secs, and the target's message 101 (posted as the sender chat with channel) through
// blacklistWatcher. The timed punishments' clock is fixed at the site's clock.
func blacklistTimedRun(t *testing.T, s *botSite, action string, secs int, channel bool) {
	t.Helper()
	mreqSwap(t, &punishNow, func() time.Time { return s.now })
	t.Cleanup(func() {
		if err := blacklists.RemoveAllBlacklist(s.chat.Id); err != nil {
			t.Errorf("RemoveAllBlacklist error = %v", err)
		}
	})
	if err := blacklists.AddBlacklistTimedContext(context.Background(), s.chat.Id, "spam", action, secs); err != nil {
		t.Fatalf("AddBlacklistTimedContext(%q, %d) error = %v", action, secs, err)
	}
	if err := db.DB.Model(&models.BlacklistSettings{}).Where("chat_id = ?", s.chat.Id).Update("reason", blacklistTimedReason).Error; err != nil {
		t.Fatalf("store the reason: %v", err)
	}
	blacklistTimedDropCache(s.chat.Id)
	if err := blacklistsModule.blacklistWatcher(s.bot, noticeGoldenMessage(s, 101, "this has spam inside", channel)); err != ext.ContinueGroups {
		t.Fatalf("blacklistWatcher error = %v, want ContinueGroups", err)
	}
}

// blacklistTimedDeleted fails unless message 101 was deleted.
func blacklistTimedDeleted(t *testing.T, s *botSite) {
	t.Helper()
	for _, call := range s.client.callsFor("deleteMessage") {
		if fmt.Sprint(call.Params["message_id"]) == "101" {
			return
		}
	}
	t.Fatal("the matching message 101 was not deleted")
}

// blacklistTimedNotice is the default notice of an applied timed blacklist action in Ukrainian: key with the
// mention of the target's full name, the duration label and the reason escaped.
func blacklistTimedNotice(tr *i18n.Translator, key string, secs int) string {
	name := formatting.GetFullName(noticeGoldenTarget.FirstName, noticeGoldenTarget.LastName)
	text, _ := tr.GetString(key, i18n.TranslationParams{
		"user":     formatting.MentionHtml(noticeGoldenTarget.Id, name),
		"duration": punishDurationLabel(tr, secs),
		"reason":   html.EscapeString(fmt.Sprintf(blacklistTimedReason, "spam")),
	})
	return text
}

// TestBlacklistTimed is the blacklist's timed punishment end to end (D-11 to D-17): the matching message is
// deleted, a timed mute or ban is sent with the end date computed right before the request, its default notice
// names the duration and escapes the reason, a sender chat under a timed action gets nothing, a timed action with
// no duration deletes and posts nothing, a custom text fills {action}, {duration} and {until}, and the private
// copy adds the Until line.
func TestBlacklistTimed(t *testing.T) {
	withModerationGoldenLocales(t)
	tr := i18n.MustNewTranslator("uk")

	t.Run("tmute", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		blacklistTimedRun(t, s, "tmute", 7200, false)

		blacklistTimedDeleted(t, s)
		call := warnTimedOnly(t, s, "restrictChatMember")
		if got, want := fmt.Sprint(call.Params["until_date"]), strconv.FormatInt(s.now.Unix()+7200, 10); got != want {
			t.Errorf("until_date = %s, want %s", got, want)
		}
		if got, ok := call.Params["permissions"].(gotgbot.ChatPermissions); !ok || got != MutedPermissions {
			t.Errorf("permissions = %#v, want MutedPermissions", call.Params["permissions"])
		}
		eventMsgAutoPunishedFirst(t, s, "restrictChatMember")
		want := blacklistTimedNotice(tr, "blacklists_bl_watcher_tmuted_user", 7200)
		if want == "" || !strings.Contains(want, "&lt;b&gt;spam&lt;/b&gt;") {
			t.Fatalf("expected notice %q does not carry the escaped reason", want)
		}
		eventMsgAutoCustom(t, s, want, nil, blacklistTimedShtmlReply, deletion.KindBotAuto)
	})

	t.Run("tban", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		blacklistTimedRun(t, s, "tban", 604800, false)

		blacklistTimedDeleted(t, s)
		call := warnTimedOnly(t, s, "banChatMember")
		if got, want := fmt.Sprint(call.Params["until_date"]), strconv.FormatInt(s.now.Unix()+604800, 10); got != want {
			t.Errorf("until_date = %s, want %s", got, want)
		}
		eventMsgAutoCustom(t, s, blacklistTimedNotice(tr, "blacklists_bl_watcher_tbanned_user", 604800), nil, blacklistTimedShtmlReply, deletion.KindBotAuto)
	})

	t.Run("sender_chat_tmute", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		blacklistTimedRun(t, s, "tmute", 7200, true)

		blacklistTimedDeleted(t, s)
		warnTimedOnly(t, s, "")
		if n := len(s.client.callsFor("sendMessage")); n != 0 {
			t.Fatalf("sendMessage calls = %d, want no notice for a sender chat", n)
		}
		s.wantNoRows(s.chat.Id)
	})

	t.Run("inert_no_duration", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		blacklistTimedRun(t, s, "tban", 0, false)

		blacklistTimedDeleted(t, s)
		warnTimedOnly(t, s, "")
		if n := len(s.client.callsFor("sendMessage")); n != 0 {
			t.Fatalf("sendMessage calls = %d, want none for a timed action with no duration", n)
		}
		s.wantNoRows(s.chat.Id)
	})

	t.Run("custom_text", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		eventMsgStore(t, s.chat.Id, eventmsg.EventBlacklist, "{action} / {duration} / {until}")
		blacklistTimedRun(t, s, "tmute", 7200, false)

		warnTimedOnly(t, s, "restrictChatMember")
		until := formatChatTime(time.Unix(s.now.Unix()+7200, 0), chatClockOf(s.chat.Id), "02.01.2006 15:04")
		want := html.EscapeString(eventActionWord(tr, "mute")) + " / " + html.EscapeString(punishDurationLabel(tr, 7200)) + " / " + until
		eventMsgAutoCustom(t, s, want, nil, "", deletion.KindBotAuto)
	})

	t.Run("pm_until", func(t *testing.T) {
		s := newEventPMAutoSite(t, deletion.KindBotAuto)
		s.chat.Title = eventPMAutoTitle
		storeEventPM(t, s.chat.Id, eventmsg.EventBlacklist, "", true)
		blacklistTimedRun(t, s, "tban", 3600, false)

		eventMsgAutoPunishedFirst(t, s, "banChatMember")
		text, _ := tr.GetString("eventmsg_pm_blacklist", i18n.TranslationParams{"group": eventPMAutoGroup, "action": eventActionWord(tr, "ban")})
		end := formatChatTime(time.Unix(s.now.Unix()+3600, 0), chatClockOf(s.chat.Id), "02.01.2006 15:04")
		line, _ := tr.GetString("eventmsg_pm_until", i18n.TranslationParams{"until": end})
		reason, _ := tr.GetString("eventmsg_pm_reason", i18n.TranslationParams{"reason": html.EscapeString(fmt.Sprintf(blacklistTimedReason, "spam"))})
		eventPMAutoWant(t, s, text+"\n"+line+"\n"+reason)
	})
}
