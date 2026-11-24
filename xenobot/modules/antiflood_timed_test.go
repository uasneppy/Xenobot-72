//go:build testtools

package modules

import (
	"context"
	"fmt"
	"html"
	"strconv"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/antiflood"
	dbcache "github.com/uasneppy/Xenobot-72/xenobot/db/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/db/deletion"
	"github.com/uasneppy/Xenobot-72/xenobot/db/eventmsg"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/formatting"
)

// TestApplyFloodModeTimed holds the one write path of the flood action with a duration: a valid timed action
// stores both columns, a plain action stores seconds 0, a timed action with no duration is the pinned unknown
// outcome, a duration outside 60 s to 366 days is the bad-duration outcome, neither refusal writes, and a failed
// write is a save failure.
func TestApplyFloodModeTimed(t *testing.T) {
	ctx := context.Background()

	t.Run("a valid timed action stores the action and the seconds", func(t *testing.T) {
		chatID := floodApplyFreshChat(t)
		got := applyFloodMode(ctx, chatID, "tmute", 1800)
		if got.Outcome != floodApplied || got.Err != nil {
			t.Fatalf("applyFloodMode(tmute, 1800) = %+v, want floodApplied", got)
		}
		if got.Stored == nil || got.Stored.Action != "tmute" || got.Stored.ActionSecs != 1800 {
			t.Fatalf("read back = %+v, want tmute and 1800", got.Stored)
		}
		if row := floodApplyStored(t, chatID); row.Action != "tmute" || row.ActionSecs != 1800 || row.Limit != 5 {
			t.Fatalf("stored row = %+v, want tmute, 1800 and the first-write limit 5", row)
		}
	})

	t.Run("a plain action stores seconds 0", func(t *testing.T) {
		chatID := floodApplyFreshChat(t)
		if got := applyFloodMode(ctx, chatID, "tban", 86400); got.Outcome != floodApplied {
			t.Fatalf("seed applyFloodMode(tban, 86400) = %+v", got)
		}
		got := applyFloodMode(ctx, chatID, "ban", 0)
		if got.Outcome != floodApplied || got.Stored == nil || got.Stored.Action != "ban" || got.Stored.ActionSecs != 0 {
			t.Fatalf("applyFloodMode(ban, 0) = %+v, want ban and 0", got)
		}
		// A plain action ignores the seconds it is handed.
		got = applyFloodMode(ctx, chatID, "kick", 3600)
		if got.Outcome != floodApplied || got.Stored == nil || got.Stored.Action != "kick" || got.Stored.ActionSecs != 0 {
			t.Fatalf("applyFloodMode(kick, 3600) = %+v, want kick and 0", got)
		}
	})

	t.Run("a timed action with no duration is unknown and writes nothing", func(t *testing.T) {
		for _, mode := range []string{"tban", "tmute"} {
			chatID := floodApplyFreshChat(t)
			got := applyFloodMode(ctx, chatID, mode, 0)
			if got.Outcome != floodUnknownMode || got.Stored != nil || got.Err != nil {
				t.Fatalf("applyFloodMode(%s, 0) = %+v, want floodUnknownMode", mode, got)
			}
			if n := floodApplyRowCount(t, chatID); n != 0 {
				t.Fatalf("applyFloodMode(%s, 0) left %d rows, want none", mode, n)
			}
		}
	})

	t.Run("a duration out of bounds is a bad duration and writes nothing", func(t *testing.T) {
		for _, c := range []struct {
			mode string
			secs int
		}{{"tmute", 59}, {"tban", 31622401}, {"tmute", -1}} {
			chatID := floodApplyFreshChat(t)
			got := applyFloodMode(ctx, chatID, c.mode, c.secs)
			if got.Outcome != floodBadDuration || got.Stored != nil || got.Err != nil {
				t.Fatalf("applyFloodMode(%s, %d) = %+v, want floodBadDuration", c.mode, c.secs, got)
			}
			if n := floodApplyRowCount(t, chatID); n != 0 {
				t.Fatalf("applyFloodMode(%s, %d) left %d rows, want none", c.mode, c.secs, n)
			}
		}
	})

	t.Run("a failed write is a save failure", func(t *testing.T) {
		floodApplyBrokenDB(t)
		got := applyFloodMode(ctx, uniqueModuleChatID(), "tmute", 600)
		if got.Outcome != floodSaveFailed || got.Err == nil || got.Stored != nil {
			t.Fatalf("applyFloodMode(tmute, 600) on a broken database = %+v, want floodSaveFailed", got)
		}
	})
}

// TestSetFloodModeTimedCommand drives /setfloodmode's timed forms and /flood through the handlers in Ukrainian
// with the real locales: the reply names the short duration, a bad duration writes nothing, a plain action with
// an extra word stores the plain action, and /flood shows the timed word.
func TestSetFloodModeTimedCommand(t *testing.T) {
	withModerationGoldenLocales(t)
	tr := i18n.MustNewTranslator("uk")
	site := func(t *testing.T) *botSite {
		t.Helper()
		s := newNoticeGoldenSite(t, deletion.KindBotManual)
		timedGoldenCleanup(t, s.chat.Id)
		return s
	}
	success := func(shown string) string {
		temp, _ := tr.GetString("antiflood_setfloodmode_success")
		return fmt.Sprintf(temp, shown)
	}
	shows := func(limit int, word string) string {
		temp, _ := tr.GetString("antiflood_flood_show_settings")
		return fmt.Sprintf(temp, limit, word)
	}

	t.Run("tmute 30m", func(t *testing.T) {
		s := site(t)
		timedGoldenCommand(t, s, antifloodModule.setFloodMode, timedGoldenAdmin, "/setfloodmode tmute 30m", false)
		if got, want := warnTimedLastText(t, s), success("tmute 30m"); got != want {
			t.Errorf("reply = %q, want %q", got, want)
		}
		if row := floodApplyStored(t, s.chat.Id); row.Action != "tmute" || row.ActionSecs != 1800 {
			t.Fatalf("stored row = %+v, want tmute and 1800", row)
		}
		timedGoldenCommand(t, s, antifloodModule.flood, timedGoldenAdmin, "/flood", false)
		if got, want := warnTimedLastText(t, s), shows(5, "muted for 30m"); got != want {
			t.Errorf("/flood = %q, want %q", got, want)
		}
	})

	t.Run("TBAN 1d", func(t *testing.T) {
		s := site(t)
		timedGoldenCommand(t, s, antifloodModule.setFloodMode, timedGoldenAdmin, "/setfloodmode TBAN 1d", false)
		if got, want := warnTimedLastText(t, s), success("tban 1d"); got != want {
			t.Errorf("reply = %q, want %q", got, want)
		}
		if row := floodApplyStored(t, s.chat.Id); row.Action != "tban" || row.ActionSecs != 86400 {
			t.Fatalf("stored row = %+v, want tban and 86400", row)
		}
		timedGoldenCommand(t, s, antifloodModule.flood, timedGoldenAdmin, "/flood", false)
		if got, want := warnTimedLastText(t, s), shows(5, "banned for 1d"); got != want {
			t.Errorf("/flood = %q, want %q", got, want)
		}
	})

	t.Run("bad durations write nothing", func(t *testing.T) {
		want, _ := tr.GetString("antiflood_setfloodmode_bad_duration")
		for _, text := range []string{"/setfloodmode tmute 0m", "/setfloodmode tban xyz", "/setfloodmode tmute 400d"} {
			s := site(t)
			timedGoldenCommand(t, s, antifloodModule.setFloodMode, timedGoldenAdmin, text, false)
			if got := warnTimedLastText(t, s); got != want {
				t.Errorf("%s: reply = %q, want %q", text, got, want)
			}
			if n := floodApplyRowCount(t, s.chat.Id); n != 0 {
				t.Errorf("%s: %d rows were written, want none", text, n)
			}
		}
	})

	t.Run("ban x stores plain ban", func(t *testing.T) {
		s := site(t)
		if got := applyFloodMode(context.Background(), s.chat.Id, "tmute", 600); got.Outcome != floodApplied {
			t.Fatalf("seed applyFloodMode(tmute, 600) = %+v", got)
		}
		timedGoldenCommand(t, s, antifloodModule.setFloodMode, timedGoldenAdmin, "/setfloodmode ban x", false)
		if got, want := warnTimedLastText(t, s), success("ban"); got != want {
			t.Errorf("reply = %q, want %q", got, want)
		}
		if row := floodApplyStored(t, s.chat.Id); row.Action != "ban" || row.ActionSecs != 0 {
			t.Fatalf("stored row = %+v, want ban and 0", row)
		}
	})
}

// floodTimedFlood is noticeGoldenFlood with a duration: a flood limit of 1 with action and secs stored through
// the repository, and the target's messages 101 and 102 (posted as the sender chat with channel) through
// checkFlood, so the second one floods. The timed punishments' clock is fixed at the site's clock.
func floodTimedFlood(t *testing.T, s *botSite, action string, secs int, channel bool) {
	t.Helper()
	mreqSwap(t, &punishNow, func() time.Time { return s.now })
	resetAntifloodState(t)
	t.Cleanup(func() {
		db.DB.Where("chat_id = ?", s.chat.Id).Delete(&models.AntifloodSettings{})
		dbcache.DeleteCache(dbcache.CacheKey("antiflood", s.chat.Id))
		resetAntifloodState(t)
	})
	if err := antiflood.SetFlood(s.chat.Id, 1); err != nil {
		t.Fatalf("SetFlood() error = %v", err)
	}
	if err := antiflood.SetFloodActionContext(context.Background(), s.chat.Id, action, secs); err != nil {
		t.Fatalf("SetFloodActionContext(%q, %d) error = %v", action, secs, err)
	}
	for _, id := range []int64{101, 102} {
		if err := antifloodModule.checkFlood(s.bot, noticeGoldenMessage(s, id, "flood "+strconv.FormatInt(id, 10), channel)); err != ext.ContinueGroups {
			t.Fatalf("checkFlood(message %d) error = %v, want ContinueGroups", id, err)
		}
	}
}

// floodTimedNotice is the default notice of an applied timed flood action in Ukrainian: key with the mention of
// the target's full name and the duration label.
func floodTimedNotice(tr *i18n.Translator, key string, secs int) string {
	name := formatting.GetFullName(noticeGoldenTarget.FirstName, noticeGoldenTarget.LastName)
	text, _ := tr.GetString(key, i18n.TranslationParams{
		"user": formatting.MentionHtml(noticeGoldenTarget.Id, name), "duration": punishDurationLabel(tr, secs),
	})
	return text
}

// TestFloodTimed is the flood's timed punishment end to end (D-11 to D-17): a timed mute or ban is sent with the
// end date computed right before the request, its default notice names the duration (the timed mute keeping the
// Unmute row), a sender chat under a timed action gets nothing, a timed action with no duration keeps today's inert
// notice, a custom text fills {action}, {duration} and {until}, and the private copy adds the Until line.
func TestFloodTimed(t *testing.T) {
	withModerationGoldenLocales(t)
	tr := i18n.MustNewTranslator("uk")
	unmuteRow := eventMsgAutoRow(t, tr, "button_unmute_admins", "unmute", noticeGoldenTarget.Id)

	t.Run("tmute", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		floodTimedFlood(t, s, "tmute", 1800, false)

		call := warnTimedOnly(t, s, "restrictChatMember")
		if got, want := fmt.Sprint(call.Params["until_date"]), strconv.FormatInt(s.now.Unix()+1800, 10); got != want {
			t.Errorf("until_date = %s, want %s", got, want)
		}
		if got, ok := call.Params["permissions"].(gotgbot.ChatPermissions); !ok || got != MutedPermissions {
			t.Errorf("permissions = %#v, want MutedPermissions", call.Params["permissions"])
		}
		eventMsgAutoPunishedFirst(t, s, "restrictChatMember")
		eventMsgAutoCustom(t, s, floodTimedNotice(tr, "antiflood_checkflood_timed_mute", 1800), []string{unmuteRow}, "", deletion.KindBotAuto)
	})

	t.Run("tban", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		floodTimedFlood(t, s, "tban", 86400, false)

		call := warnTimedOnly(t, s, "banChatMember")
		if got, want := fmt.Sprint(call.Params["until_date"]), strconv.FormatInt(s.now.Unix()+86400, 10); got != want {
			t.Errorf("until_date = %s, want %s", got, want)
		}
		eventMsgAutoCustom(t, s, floodTimedNotice(tr, "antiflood_checkflood_timed_ban", 86400), nil, "", deletion.KindBotAuto)
	})

	t.Run("sender_chat_tban", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		floodTimedFlood(t, s, "tban", 86400, true)

		warnTimedOnly(t, s, "")
		if n := len(s.client.callsFor("sendMessage")); n != 0 {
			t.Fatalf("sendMessage calls = %d, want no notice for a sender chat", n)
		}
		s.wantNoRows(s.chat.Id)
	})

	t.Run("inert_no_duration", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		floodTimedFlood(t, s, "tmute", 0, false)

		warnTimedOnly(t, s, "")
		eventMsgAutoGolden(t, s, 1, goldenTimedFloodInertTmute)
	})

	t.Run("custom_text", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		eventMsgStore(t, s.chat.Id, eventmsg.EventFlood, "{action} / {duration} / {until}")
		floodTimedFlood(t, s, "tmute", 1800, false)

		warnTimedOnly(t, s, "restrictChatMember")
		until := formatChatTime(time.Unix(s.now.Unix()+1800, 0), chatClockOf(s.chat.Id), "02.01.2006 15:04")
		want := html.EscapeString(eventActionWord(tr, "mute")) + " / " + html.EscapeString(punishDurationLabel(tr, 1800)) + " / " + until
		eventMsgAutoCustom(t, s, want, []string{unmuteRow}, "", deletion.KindBotAuto)
	})

	t.Run("pm_until", func(t *testing.T) {
		s := newEventPMAutoSite(t, deletion.KindBotAuto)
		s.chat.Title = eventPMAutoTitle
		storeEventPM(t, s.chat.Id, eventmsg.EventFlood, "", true)
		floodTimedFlood(t, s, "tban", 3600, false)

		eventMsgAutoPunishedFirst(t, s, "banChatMember")
		text, _ := tr.GetString("eventmsg_pm_flood", i18n.TranslationParams{"group": eventPMAutoGroup, "action": eventActionWord(tr, "ban")})
		end := formatChatTime(time.Unix(s.now.Unix()+3600, 0), chatClockOf(s.chat.Id), "02.01.2006 15:04")
		line, _ := tr.GetString("eventmsg_pm_until", i18n.TranslationParams{"until": end})
		reason, _ := tr.GetString("eventmsg_pm_reason", i18n.TranslationParams{"reason": html.EscapeString(floodEventReason(tr))})
		eventPMAutoWant(t, s, text+"\n"+line+"\n"+reason)
	})
}
