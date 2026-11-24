//go:build testtools

package modules

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/advthresholds"
	"github.com/uasneppy/Xenobot-72/xenobot/db/antiflood"
	"github.com/uasneppy/Xenobot-72/xenobot/db/approvals"
	"github.com/uasneppy/Xenobot-72/xenobot/db/blacklists"
	dbcache "github.com/uasneppy/Xenobot-72/xenobot/db/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/db/escalation"
	"github.com/uasneppy/Xenobot-72/xenobot/db/eventmsg"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/db/modtuning"
	"github.com/uasneppy/Xenobot-72/xenobot/db/staff"
	"github.com/uasneppy/Xenobot-72/xenobot/db/warns"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
)

// Phase 29 characterization goldens recorded on unchanged commit ca060519 (MSG-11).
// Frozen before any Phase 29 production change; never edit, weaken, or re-record.
var goldenP29GroupDefaults = map[string]string{
	eventmsg.EventBan:           "Removal from the facility complete: <a href=\"tg://user?id=7001\">Alice</a> is banned.\n<b>Reason:</b> spam",
	eventmsg.EventTimedBan:      "Temporary removal from the facility: <a href=\"tg://user?id=7001\">Alice</a> is banned for 1h.\n<b>Reason:</b> spam",
	eventmsg.EventKick:          "Kicked <a href=\"tg://user?id=7001\">Alice</a> from the facility.\n<b>Reason:</b> spam",
	eventmsg.EventMute:          "Silence order issued: <a href=\"tg://user?id=7001\">Alice</a> is muted.\n<b>Reason:</b> spam",
	eventmsg.EventTimedMute:     "Silence order issued: <a href=\"tg://user?id=7001\">Alice</a> is muted for 1h.\n<b>Reason:</b> spam",
	eventmsg.EventUnban:         "Access restored: <a href=\"tg://user?id=7001\">Alice</a> is unbanned.",
	eventmsg.EventUnmute:        "Silence order lifted: <a href=\"tg://user?id=7001\">Alice</a> may talk again.",
	eventmsg.EventWarn:          "Violation notice: <a href=\"tg://user?id=7001\">Alice</a>, proceed with care.\n<b>Warnings:</b> 2/3\n<b>Reason:</b> spam",
	eventmsg.EventWarnLimit:     "Warnings: 2/3. <a href=\"tg://user?id=7001\">Alice</a> has been muted.\n - spam",
	eventmsg.EventFlood:         "Flood detected. <a href=\"tg://user?id=7001\">Alice</a> has been muted.",
	eventmsg.EventBlacklist:     "Muted <a href=\"tg://user?id=7001\">Alice</a>. Reason: spam.",
	eventmsg.EventForwardBlock:  "Muted <a href=\"tg://user?id=7001\">Alice</a>. Reason: spam.",
	eventmsg.EventCaptcha:       "❌ <a href=\"tg://user?id=7001\">Alice</a> did not complete captcha verification and has been muted for 24 hours.\n\n📝 2 message(s) were sent before verification and deleted.",
	eventmsg.EventAISpam:        "🧠 I removed a message from <a href=\"tg://user?id=7001\">Alice</a>: it looked like spam.",
	eventmsg.EventReqWarn:       "❗ <a href=\"tg://user?id=7001\">\u2068Alice\u2069</a> broke this group's requirements: no username. That is 2 of 3 warnings, so they were muted. An admin can unmute them.",
	eventmsg.EventReqKick:       "👢 <a href=\"tg://user?id=7001\">\u2068Alice\u2069</a> was removed: no username. They can join again once they fix what is listed.",
	eventmsg.EventReqBan:        "⛔ <a href=\"tg://user?id=7001\">\u2068Alice\u2069</a> was banned: no username. An admin can unban them.",
	eventmsg.EventReqMute:       "🔇 <a href=\"tg://user?id=7001\">\u2068Alice\u2069</a>, you are muted: no username. Fix what is listed, then tap “I fixed it”.",
	eventmsg.EventLockdown:      "🔒 This group is in lockdown. Only admins can talk, and anyone who joins is removed until an admin runs /unlockdown.\nApproved users are muted too.\nLocked by <a href=\"tg://user?id=7001\">Alice</a>.\nReason: spam",
	eventmsg.EventLockJoinSurge: "🔒 I locked this group automatically: 2 people joined within 60 seconds. Only admins can talk, and anyone who joins is removed until an admin runs /unlockdown.\nApproved users are muted too.",
	eventmsg.EventLockFlood:     "🔒 I locked this group automatically: 2 messages arrived within 60 seconds, 60% of them from new members. Only admins can talk, and anyone who joins is removed until an admin runs /unlockdown.\nApproved users are muted too.",
	eventmsg.EventLockAIBurst:   "🔒 I locked this group automatically: 2 messages were judged spam within 60 seconds. Only admins can talk, and anyone who joins is removed until an admin runs /unlockdown.\nApproved users are muted too.",
	eventmsg.EventLockLift:      "🔓 Lockdown lifted by <a href=\"tg://user?id=7001\">Alice</a>. The group's permissions are back to what they were before the lockdown.",
}

var goldenP29PMDefaults = map[string]string{
	eventmsg.EventBan:          "🚫 You were banned from <b>Demo Group</b>.\n<b>Reason:</b> spam",
	eventmsg.EventTimedBan:     "⏳ You were banned from <b>Demo Group</b> until 23.11.2026 12:00 (UTC).\n<b>Reason:</b> spam",
	eventmsg.EventKick:         "👢 You were removed from <b>Demo Group</b>. You can join again.\n<b>Reason:</b> spam",
	eventmsg.EventMute:         "🔇 You were muted in <b>Demo Group</b>.\n<b>Reason:</b> spam",
	eventmsg.EventTimedMute:    "🔕 You were muted in <b>Demo Group</b> until 23.11.2026 12:00 (UTC).\n<b>Reason:</b> spam",
	eventmsg.EventUnban:        "🔓 You were unbanned in <b>Demo Group</b>. You can join again.\n<b>Reason:</b> spam",
	eventmsg.EventUnmute:       "🔊 You can write in <b>Demo Group</b> again.\n<b>Reason:</b> spam",
	eventmsg.EventWarn:         "🚩 You were warned in <b>Demo Group</b> (2 of 3).\n<b>Reason:</b> spam",
	eventmsg.EventWarnLimit:    "⛔ You reached the warning limit in <b>Demo Group</b> (2 of 3). Action: muted.\n<b>Until:</b> 23.11.2026 12:00 (UTC)\n<b>Reason:</b> spam",
	eventmsg.EventFlood:        "🌊 The flood protection of <b>Demo Group</b> acted on your messages. Action: muted.\n<b>Until:</b> 23.11.2026 12:00 (UTC)\n<b>Reason:</b> spam",
	eventmsg.EventBlacklist:    "🔤 A message of yours in <b>Demo Group</b> was removed. Action: muted.\n<b>Until:</b> 23.11.2026 12:00 (UTC)\n<b>Reason:</b> spam",
	eventmsg.EventForwardBlock: "🚷 A message you forwarded to <b>Demo Group</b> was removed. Action: muted.\n<b>Until:</b> 23.11.2026 12:00 (UTC)\n<b>Reason:</b> spam",
	eventmsg.EventCaptcha:      "🧩 You did not complete the captcha in <b>Demo Group</b>. Action: muted.\n<b>Reason:</b> spam",
	eventmsg.EventAISpam:       "🧠 I removed a message of yours in <b>Demo Group</b>. Action: muted.\n<b>Reason:</b> spam",
	eventmsg.EventReqWarn:      "❗ You were warned in <b>Demo Group</b> (2 of 3) because you do not meet its requirements. Action: muted.\n<b>Reason:</b> spam",
	eventmsg.EventReqKick:      "👢 You were removed from <b>Demo Group</b> because you do not meet its requirements. You can join again once you fix what is listed.\n<b>Reason:</b> spam",
	eventmsg.EventReqBan:       "⛔ You were banned from <b>Demo Group</b> because you do not meet its requirements.\n<b>Reason:</b> spam",
	eventmsg.EventReqMute:      "🔇 You are muted in <b>Demo Group</b>: you do not meet its requirements. Fix what is listed, then tap “I fixed it” under my notice in the group.\n<b>Reason:</b> spam",
}

func phase29FixedDeterministicInput() eventDefaultInput {
	return eventDefaultInput{
		Target:  gotgbot.User{Id: 7001, FirstName: "Alice"},
		Reason:  "spam",
		TimeVal: "1h",
		Count:   2,
		Limit:   3,
		Action:  "mute",
		Reasons: []string{"spam"},
		Rules:   memberReqRuleBits("un"),
		Window:  60,
		Percent: 60,
	}
}

func phase29WithFixedUTCClock(t *testing.T, now time.Time) {
	t.Helper()
	prevNow := punishNow
	punishNow = func() time.Time { return now }
	t.Cleanup(func() { punishNow = prevNow })

	prevTZ := chatTimezoneLookup
	chatTimezoneLookup = func(context.Context, int64) (string, error) { return "UTC", nil }
	t.Cleanup(func() { chatTimezoneLookup = prevTZ })
}

func TestLegacyPhase29Baseline(t *testing.T) {
	withRealLocales(t)
	tr := i18n.MustNewTranslator("en")
	fixedTime := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	phase29WithFixedUTCClock(t, fixedTime)

	t.Run("tone_off_all_23_group_defaults", func(t *testing.T) {
		in := phase29FixedDeterministicInput()
		seen := 0
		for _, fam := range settingsMsgFamilies {
			for _, ev := range fam.Events {
				seen++
				got, _ := ev.Default(tr, in)
				want, ok := goldenP29GroupDefaults[ev.Key]
				if !ok {
					t.Errorf("missing goldenP29GroupDefaults entry for %s", ev.Key)
					continue
				}
				if got != want {
					t.Errorf("tone-off group default for %s changed\n got: %q\nwant: %q", ev.Key, got, want)
				}
			}
		}
		if seen != 23 {
			t.Fatalf("rendered group events = %d, want 23", seen)
		}
		if len(goldenP29GroupDefaults) != 23 {
			t.Fatalf("goldenP29GroupDefaults entries = %d, want 23", len(goldenP29GroupDefaults))
		}
	})

	t.Run("tone_off_all_18_pm_defaults", func(t *testing.T) {
		phase28InProcessMiniredis(t)
		withFastPMCopyPacer(t)
		prevCap := pmCopyAutoCapPerMinute
		pmCopyAutoCapPerMinute = 100
		t.Cleanup(func() { pmCopyAutoCapPerMinute = prevCap })
		t.Cleanup(pmCopyJobs.wait)

		const chatID int64 = -100290101
		eventPMCleanup(t, chatID)

		ctx := context.Background()
		for _, ev := range eventmsg.Events {
			if _, ok := eventPMSpecFor(ev); !ok {
				continue
			}
			if err := eventmsg.SetSilent(ctx, chatID, ev, eventmsg.DestPM, false, 1); err != nil {
				t.Fatalf("SetSilent(%d, %s, pm, false) error = %v", chatID, ev, err)
			}
		}

		client := newModuleBotClient()
		bot := &gotgbot.Bot{
			User:      gotgbot.User{Id: 999029},
			Token:     "test-token",
			BotClient: client,
		}
		chat := &gotgbot.Chat{
			Id:    chatID,
			Type:  "supergroup",
			Title: "Demo Group",
		}
		target := gotgbot.User{Id: 7001, FirstName: "Alice"}
		targetIDStr := strconv.FormatInt(target.Id, 10)
		actionWord := eventActionWord(tr, "mute")

		seen := 0
		prevSends := 0
		for _, ev := range eventmsg.Events {
			if _, ok := eventPMSpecFor(ev); !ok {
				continue
			}
			seen++
			site := eventSite{
				Bot:  bot,
				Chat: chat,
				Tr:   tr,
				SendDefault: func(text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
					return &gotgbot.Message{MessageId: 101}, nil
				},
			}
			r := eventReply{
				Event:  ev,
				Target: target,
				Values: eventValues{
					Reason:   "spam",
					Duration: "1h",
					Until:    fixedTime,
					Count:    2,
					Limit:    3,
					Action:   actionWord,
				},
				Default: "group-default",
			}
			if _, err := postEventSent(site, r); err != nil {
				t.Fatalf("postEventSent(%s) error = %v", ev, err)
			}
			waitEventPM(t)

			allSends := client.callsFor("sendMessage")
			var pmTexts []string
			for _, call := range allSends[prevSends:] {
				if fmt.Sprint(call.Params["chat_id"]) == targetIDStr {
					params := rulesCallParams(t, call)
					text, _ := params["text"].(string)
					pmTexts = append(pmTexts, text)
				}
			}
			prevSends = len(allSends)

			if len(pmTexts) != 1 {
				t.Fatalf("event %s PM sendMessage count = %d, want 1", ev, len(pmTexts))
			}
			got := pmTexts[0]
			want, ok := goldenP29PMDefaults[ev]
			if !ok {
				t.Errorf("missing goldenP29PMDefaults entry for %s", ev)
				continue
			}
			if got != want {
				t.Errorf("tone-off PM default for %s changed\n got: %q\nwant: %q", ev, got, want)
			}
		}
		if seen != 18 {
			t.Fatalf("rendered PM events = %d, want 18", seen)
		}
		if len(goldenP29PMDefaults) != 18 {
			t.Fatalf("goldenP29PMDefaults entries = %d, want 18", len(goldenP29PMDefaults))
		}

		var optCount int64
		if err := db.DB.Model(&models.ChatEventMessageOptions{}).Where("chat_id = ?", chatID).Count(&optCount).Error; err != nil {
			t.Fatalf("count chat_event_message_options: %v", err)
		}
		if optCount != 0 {
			t.Fatalf("chat_event_message_options rows = %d, want 0", optCount)
		}
	})

	t.Run("tone_off_post_event_sends_unchanged_default", func(t *testing.T) {
		const chatID int64 = -100290102
		eventMsgCleanup(t, chatID)

		in := phase29FixedDeterministicInput()
		defaultText, defaultOpts := banDefaultReply(tr, in)

		var sentText string
		sendCount := 0
		site := eventSite{
			Bot:  &gotgbot.Bot{User: gotgbot.User{Id: 999029}, Token: "test-token", BotClient: newModuleBotClient()},
			Chat: &gotgbot.Chat{Id: chatID, Type: "supergroup", Title: "Unconfigured Group"},
			Tr:   tr,
			SendDefault: func(text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
				sendCount++
				sentText = text
				return &gotgbot.Message{MessageId: 202}, nil
			},
		}
		r := eventReply{
			Event:   eventmsg.EventBan,
			Target:  in.Target,
			Values:  eventValues{Reason: in.Reason},
			Default: defaultText,
			Opts:    defaultOpts,
		}
		if _, err := postEventSent(site, r); err != nil {
			t.Fatalf("postEventSent(ban) error = %v", err)
		}
		if sendCount != 1 {
			t.Fatalf("SendDefault calls = %d, want 1", sendCount)
		}
		if want := goldenP29GroupDefaults[eventmsg.EventBan]; sentText != want {
			t.Fatalf("sent tone-off ban default changed\n got: %q\nwant: %q", sentText, want)
		}

		var msgCount, optCount int64
		if err := db.DB.Model(&models.ChatEventMessage{}).Where("chat_id = ?", chatID).Count(&msgCount).Error; err != nil {
			t.Fatalf("count chat_event_messages: %v", err)
		}
		if err := db.DB.Model(&models.ChatEventMessageOptions{}).Where("chat_id = ?", chatID).Count(&optCount).Error; err != nil {
			t.Fatalf("count chat_event_message_options: %v", err)
		}
		if msgCount != 0 || optCount != 0 {
			t.Fatalf("unconfigured chat created rows: messages=%d options=%d, want 0 and 0", msgCount, optCount)
		}

		msgs, err := eventmsg.GetFresh(context.Background(), chatID)
		if err != nil {
			t.Fatalf("eventmsg.GetFresh error = %v", err)
		}
		if msgs.Options.PMDetails || msgs.Options.PMLog {
			t.Fatalf("default options = %+v, want all false", msgs.Options)
		}
	})

	t.Run("uncopied_group_isolation_baseline", func(t *testing.T) {
		const (
			srcChat int64 = -100290201
			dstChat int64 = -100290202
		)
		eventMsgCleanup(t, srcChat, dstChat)
		t.Cleanup(func() {
			for _, cid := range []int64{srcChat, dstChat} {
				_ = db.DB.Where("chat_id = ?", cid).Delete(&models.WarnSettings{}).Error
				_ = db.DB.Where("chat_id = ?", cid).Delete(&models.AntifloodSettings{}).Error
				_ = blacklists.RemoveAllBlacklist(cid)
				_ = db.DB.Where("chat_id = ?", cid).Delete(&models.ChatEscalation{}).Error
				_ = db.DB.Where("chat_id = ?", cid).Delete(&models.ChatManualDefaults{}).Error
				_ = db.DB.Where("chat_id = ?", cid).Delete(&models.ChatReasonShortcut{}).Error
				_ = db.DB.Where("chat_id = ?", cid).Delete(&models.ChatAdvancedThresholds{}).Error
				_ = approvals.RemoveAllApprovedUsers(cid)
				_ = db.DB.Where("group_chat_id = ? OR staff_chat_id = ?", cid, cid).Delete(&models.StaffGroupLink{}).Error

				dbcache.DeleteCache(dbcache.CacheKey("warn_settings", cid))
				dbcache.DeleteCache(dbcache.CacheKey("antiflood", cid))
				dbcache.DeleteCache(dbcache.CacheKey("blacklist", cid))
				dbcache.DeleteCache(escalation.CacheKey(cid))
				dbcache.DeleteCache(modtuning.DefaultsCacheKey(cid))
				dbcache.DeleteCache(modtuning.ShortcutsCacheKey(cid))
				dbcache.DeleteCache(advthresholds.CacheKey(cid))
			}
			dbcache.ResetLocalForTest()
		})

		ctx := context.Background()

		// Seed srcChat across all copy-scope areas.
		if err := eventmsg.SetText(ctx, srcChat, eventmsg.EventBan, eventmsg.DestGroup, "Custom ban: {reason}", 1); err != nil {
			t.Fatalf("seed src ban text: %v", err)
		}
		if err := eventmsg.SetPMDetails(ctx, srcChat, true, 1); err != nil {
			t.Fatalf("seed src pm_details: %v", err)
		}
		if err := eventmsg.SetPMLog(ctx, srcChat, true, 1); err != nil {
			t.Fatalf("seed src pm_log: %v", err)
		}
		if err := warns.SetWarnModeContext(ctx, srcChat, "tban", 3600); err != nil {
			t.Fatalf("seed src warn mode: %v", err)
		}
		if err := warns.SetWarnExpiryContext(ctx, srcChat, 7); err != nil {
			t.Fatalf("seed src warn expiry: %v", err)
		}
		if err := antiflood.SetFloodContext(ctx, srcChat, 5); err != nil {
			t.Fatalf("seed src flood limit: %v", err)
		}
		if err := antiflood.SetFloodActionContext(ctx, srcChat, "tmute", 600); err != nil {
			t.Fatalf("seed src flood action: %v", err)
		}
		if err := blacklists.AddBlacklistTimedContext(ctx, srcChat, "spamword", "tban", 3600); err != nil {
			t.Fatalf("seed src blacklist word: %v", err)
		}
		if err := escalation.SetEnabled(ctx, srcChat, true, 1); err != nil {
			t.Fatalf("seed src escalation: %v", err)
		}
		if err := modtuning.SetDefaultBanSecs(ctx, srcChat, 3600, 1); err != nil {
			t.Fatalf("seed src default ban secs: %v", err)
		}
		if err := modtuning.SetReasonRequired(ctx, srcChat, true, 1); err != nil {
			t.Fatalf("seed src reason required: %v", err)
		}
		if err := modtuning.SetAppealURL(ctx, srcChat, "https://example.org/appeal", 1); err != nil {
			t.Fatalf("seed src appeal url: %v", err)
		}
		if added, err := modtuning.AddShortcut(ctx, srcChat, "sp", "Spam", 1); err != nil || !added {
			t.Fatalf("seed src shortcut: added=%v err=%v", added, err)
		}
		if err := advthresholds.SetField(ctx, srcChat, advthresholds.FieldRaidFloodWindow, 120, 1); err != nil {
			t.Fatalf("seed src advthreshold: %v", err)
		}

		// Seed dstChat only with an approval row and one blacklist word.
		if err := approvals.AddApprovedUser(dstChat, 8001, 1, "trusted"); err != nil {
			t.Fatalf("seed dst approval: %v", err)
		}
		if err := blacklists.AddBlacklistTimedContext(ctx, dstChat, "keepword", "warn", 0); err != nil {
			t.Fatalf("seed dst blacklist word: %v", err)
		}

		// Read srcChat via fresh getters.
		srcMsgs, err := eventmsg.GetFresh(ctx, srcChat)
		if err != nil {
			t.Fatalf("src eventmsg.GetFresh: %v", err)
		}
		if row, ok := srcMsgs.Lookup(eventmsg.EventBan, eventmsg.DestGroup); !ok || row.Text != "Custom ban: {reason}" {
			t.Fatalf("src ban row = (%+v, %v), want custom ban text", row, ok)
		}
		if !srcMsgs.Options.PMDetails || !srcMsgs.Options.PMLog {
			t.Fatalf("src options = %+v, want PMDetails=true PMLog=true", srcMsgs.Options)
		}
		srcWarn, err := warns.LoadWarnSettingsContext(ctx, srcChat)
		if err != nil || srcWarn.WarnMode != "tban" || srcWarn.WarnModeSecs != 3600 || srcWarn.ExpiryDays != 7 {
			t.Fatalf("src warn settings = (%+v, %v), want tban/3600/7", srcWarn, err)
		}
		if srcFlood := antiflood.GetFloodContext(ctx, srcChat); srcFlood.Limit != 5 || srcFlood.Action != "tmute" || srcFlood.ActionSecs != 600 {
			t.Fatalf("src flood = %+v, want limit=5 tmute/600", srcFlood)
		}
		srcBL, err := blacklists.ListBlacklistEntriesFresh(ctx, srcChat)
		if err != nil || len(srcBL) != 1 || srcBL[0].Word != "spamword" || srcBL[0].Action != "tban" || srcBL[0].ActionSecs != 3600 {
			t.Fatalf("src blacklist = (%+v, %v), want spamword/tban/3600", srcBL, err)
		}
		srcEsc, err := escalation.GetFresh(ctx, srcChat)
		if err != nil || !srcEsc.Enabled {
			t.Fatalf("src escalation = (%+v, %v), want enabled", srcEsc, err)
		}
		srcDefs, err := modtuning.GetDefaultsFresh(ctx, srcChat)
		if err != nil || srcDefs.DefaultBanSecs != 3600 || !srcDefs.ReasonRequired || srcDefs.AppealURL != "https://example.org/appeal" {
			t.Fatalf("src manual defaults = (%+v, %v), want ban=3600 req=true appeal", srcDefs, err)
		}
		srcShortcuts, err := modtuning.ListShortcutsFresh(ctx, srcChat)
		if err != nil || len(srcShortcuts) != 1 || srcShortcuts[0].Word != "sp" || srcShortcuts[0].Reason != "Spam" {
			t.Fatalf("src shortcuts = (%+v, %v), want sp->Spam", srcShortcuts, err)
		}
		srcAdv, err := advthresholds.GetFresh(ctx, srcChat)
		if err != nil || srcAdv.Value(advthresholds.FieldRaidFloodWindow) != 120 {
			t.Fatalf("src advthresholds = (%+v, %v), want raid_flood_window=120", srcAdv, err)
		}

		// Verify dstChat remains completely untouched and has 0 rows in every unconfigured table.
		assertZeroRows := func(label string, model any, where string, arg int64) {
			t.Helper()
			var count int64
			if err := db.DB.Model(model).Where(where, arg).Count(&count).Error; err != nil {
				t.Fatalf("count %s: %v", label, err)
			}
			if count != 0 {
				t.Fatalf("dst %s rows = %d, want 0", label, count)
			}
		}
		assertZeroRows("chat_event_messages", &models.ChatEventMessage{}, "chat_id = ?", dstChat)
		assertZeroRows("chat_event_message_options", &models.ChatEventMessageOptions{}, "chat_id = ?", dstChat)
		assertZeroRows("antiflood_settings", &models.AntifloodSettings{}, "chat_id = ?", dstChat)
		assertZeroRows("warns_settings", &models.WarnSettings{}, "chat_id = ?", dstChat)
		assertZeroRows("chat_escalation", &models.ChatEscalation{}, "chat_id = ?", dstChat)
		assertZeroRows("chat_manual_defaults", &models.ChatManualDefaults{}, "chat_id = ?", dstChat)
		assertZeroRows("chat_reason_shortcuts", &models.ChatReasonShortcut{}, "chat_id = ?", dstChat)
		assertZeroRows("chat_advanced_thresholds", &models.ChatAdvancedThresholds{}, "chat_id = ?", dstChat)
		assertZeroRows("staff_group_links", &models.StaffGroupLink{}, "group_chat_id = ?", dstChat)

		if dstFlood := antiflood.GetFloodContext(ctx, dstChat); dstFlood.Limit != 0 {
			t.Fatalf("dst antiflood Limit = %d, want 0", dstFlood.Limit)
		}
		if dstLink, err := staff.GetLinkOfGroupFresh(dstChat); err != nil || dstLink != nil {
			t.Fatalf("dst staff link = (%+v, %v), want (nil, nil)", dstLink, err)
		}

		dstBL, err := blacklists.ListBlacklistEntriesFresh(ctx, dstChat)
		if err != nil || len(dstBL) != 1 || dstBL[0].Word != "keepword" || dstBL[0].Action != "warn" || dstBL[0].ActionSecs != 0 {
			t.Fatalf("dst blacklist = (%+v, %v), want keepword/warn/0", dstBL, err)
		}
		approved, err := approvals.IsUserApprovedContext(ctx, dstChat, 8001)
		if err != nil || !approved {
			t.Fatalf("dst approval for 8001 = (%v, %v), want (true, nil)", approved, err)
		}
	})
}
