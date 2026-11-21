//go:build testtools

package modules

import (
	"fmt"
	"html"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/redis/go-redis/v9"

	"github.com/uasneppy/fuku_robot/fuku/db/blacklists"
	"github.com/uasneppy/fuku_robot/fuku/db/deletion"
	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/db/staff"
	"github.com/uasneppy/fuku_robot/fuku/db/warns"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
)

// The private copies of the Automatic family (Phase 23 plan 23-05): the warn and the warn limit, whose
// specs are manual but whose rule-raised replies count against the per-group cap (eventPMOffer.ByRule),
// and the flood, blacklist and forward blocklist punishments, whose specs are automatic (D-28).

// newEventPMAutoSite is a notice golden site (a fresh supergroup whose client answers the member 42's
// getChatMember and getChat, the real locales, a 600 s rule of kind) with miniredis, a fast private copy
// pacer and a client that answers the member's private chat. The chat's event messages and the private
// copy keys are removed when the test ends, after the jobs.
func newEventPMAutoSite(t *testing.T, kind string) *botSite {
	t.Helper()
	withMiniredisServer(t)
	withFastPMCopyPacer(t)
	s := newNoticeGoldenSite(t, kind)
	s.bot.BotClient = eventPMTestClient{BotClient: s.bot.BotClient, script: &eventPMScript{}}
	eventPMCleanup(t, s.chat.Id)
	t.Cleanup(pmCopyJobs.wait)
	return s
}

// eventPMAutoDefault is an event's default private text as the group's language (uk) words it with params,
// followed by the reason line when reason is set.
func eventPMAutoDefault(t *testing.T, key string, params i18n.TranslationParams, reason string) string {
	t.Helper()
	tr := i18n.MustNewTranslator("uk")
	text, err := tr.GetString(key, params)
	if err != nil || text == "" {
		t.Fatalf("translate %s: %q, %v", key, text, err)
	}
	if reason != "" {
		line, _ := tr.GetString("eventmsg_pm_reason", i18n.TranslationParams{"reason": reason})
		text += "\n" + line
	}
	return text
}

// eventPMAutoTitle is the group title of the Automatic family's private copy tests and eventPMAutoGroup the way the
// default private texts show it (escaped).
const eventPMAutoTitle, eventPMAutoGroup = "Q&A <Group>", "Q&amp;A &lt;Group&gt;"

// eventPMAutoWant checks that the member received exactly one private copy after every group send, with text want
// and no keyboard, and that the site sent at least one group message (the notice comes before the copy).
func eventPMAutoWant(t *testing.T, s *botSite, want string) {
	t.Helper()
	params := eventPMOnly(t, s, "sendMessage")
	if params["text"] != want {
		t.Fatalf("private copy text = %q, want %q", params["text"], want)
	}
	if params["reply_markup"] != nil {
		t.Fatalf("private copy reply_markup = %v, want none", params["reply_markup"])
	}
	if group, _ := eventPMCalls(t, s); len(group) == 0 {
		t.Fatal("no message was posted in the group before the private copy")
	}
}

// eventPMAutoNone checks that no private copy reached the member and that the group message was still posted.
func eventPMAutoNone(t *testing.T, s *botSite) {
	t.Helper()
	group, private := eventPMCalls(t, s)
	if len(private) != 0 {
		t.Fatalf("sends to the member's private chat = %d, want none", len(private))
	}
	if len(group) == 0 {
		t.Fatal("no message was posted in the group")
	}
}

// eventPMAutoNoRedis takes the per-group cap's Redis client away for the rest of the test; the jobs are waited for
// before it is put back.
func eventPMAutoNoRedis(t *testing.T) {
	t.Helper()
	mreqSwap(t, &pmCopyRedisClient, func() *redis.Client { return nil })
	t.Cleanup(pmCopyJobs.wait)
}

// TestEventPMAutoEvents sends each of the five events' private copies through its real site, with the copy switched on
// and nothing stored: the default private text in the group's language after the punishment and the group message.
func TestEventPMAutoEvents(t *testing.T) {
	withModerationGoldenLocales(t)
	tr := i18n.MustNewTranslator("uk")
	const blacklistReason = "Blacklisted word: &#39;spam&#39;"

	t.Run("warn", func(t *testing.T) {
		s := newEventPMAutoSite(t, deletion.KindBotManual)
		s.chat.Title = eventPMAutoTitle
		storeEventPM(t, s.chat.Id, eventmsg.EventWarn, "", true)
		// A /warn is an admin's act: its copy reads no cap counter, so it is sent without Redis too.
		eventPMAutoNoRedis(t)
		noticeGoldenWarn(t, s, warnsModule.warnUser, "/warn spam <b>")
		if n, _ := warns.GetWarns(noticeGoldenTarget.Id, s.chat.Id); n != 1 {
			t.Fatalf("stored warns = %d, want 1", n)
		}
		eventPMAutoWant(t, s, eventPMAutoDefault(t, "eventmsg_pm_warn",
			i18n.TranslationParams{"group": eventPMAutoGroup, "count": "1", "limit": "3"}, "spam &lt;b&gt;"))
	})

	t.Run("automatic warn", func(t *testing.T) {
		s := newEventPMAutoSite(t, deletion.KindBotAuto)
		s.chat.Title = eventPMAutoTitle
		storeEventPM(t, s.chat.Id, eventmsg.EventWarn, "", true)
		noticeGoldenBlacklist(t, s, "warn", false, ext.EndGroups)
		eventPMAutoWant(t, s, eventPMAutoDefault(t, "eventmsg_pm_warn",
			i18n.TranslationParams{"group": eventPMAutoGroup, "count": "1", "limit": "3"}, blacklistReason))
	})

	t.Run("automatic warn without redis", func(t *testing.T) {
		s := newEventPMAutoSite(t, deletion.KindBotAuto)
		storeEventPM(t, s.chat.Id, eventmsg.EventWarn, "", true)
		eventPMAutoNoRedis(t)
		noticeGoldenBlacklist(t, s, "warn", false, ext.EndGroups)
		if n, _ := warns.GetWarns(noticeGoldenTarget.Id, s.chat.Id); n != 1 {
			t.Fatalf("stored warns = %d, want 1: the warn still happens", n)
		}
		// A warn the blacklist raised is capped like an automatic event (ByRule), and the cap fails closed.
		eventPMAutoNone(t, s)
	})

	t.Run("flood", func(t *testing.T) {
		s := newEventPMAutoSite(t, deletion.KindBotAuto)
		s.chat.Title = eventPMAutoTitle
		storeEventPM(t, s.chat.Id, eventmsg.EventFlood, "", true)
		noticeGoldenFlood(t, s, "mute", false)
		eventMsgAutoPunishedFirst(t, s, "restrictChatMember")
		eventPMAutoWant(t, s, eventPMAutoDefault(t, "eventmsg_pm_flood",
			i18n.TranslationParams{"group": eventPMAutoGroup, "action": eventActionWord(tr, "mute")}, html.EscapeString(floodEventReason(tr))))
	})

	t.Run("flood by a sender chat", func(t *testing.T) {
		s := newEventPMAutoSite(t, deletion.KindBotAuto)
		storeEventPM(t, s.chat.Id, eventmsg.EventFlood, "", true)
		noticeGoldenFlood(t, s, "ban", true)
		eventMsgAutoPunishedFirst(t, s, "banChatSenderChat")
		// A sender chat's negative ID can never receive a copy: pmCopySkipTarget refuses it and nothing is sent to it.
		if !pmCopySkipTarget(s.bot, noticeGoldenChannel.Id) {
			t.Fatal("pmCopySkipTarget accepts the sender chat's ID")
		}
		if n := eventPMSentTo(t, s, []int64{noticeGoldenChannel.Id}); n != 0 {
			t.Fatalf("sends to the sender chat = %d, want none", n)
		}
		eventPMAutoNone(t, s)
	})

	t.Run("blacklist", func(t *testing.T) {
		s := newEventPMAutoSite(t, deletion.KindBotAuto)
		s.chat.Title = eventPMAutoTitle
		storeEventPM(t, s.chat.Id, eventmsg.EventBlacklist, "", true)
		noticeGoldenBlacklist(t, s, "kick", false, ext.ContinueGroups)
		eventMsgAutoPunishedFirst(t, s, "unbanChatMember")
		eventPMAutoWant(t, s, eventPMAutoDefault(t, "eventmsg_pm_blacklist",
			i18n.TranslationParams{"group": eventPMAutoGroup, "action": eventActionWord(tr, "kick")}, blacklistReason))
	})

	t.Run("forward block", func(t *testing.T) {
		s := newEventPMAutoSite(t, deletion.KindBotAuto)
		s.chat.Title = eventPMAutoTitle
		storeEventPM(t, s.chat.Id, eventmsg.EventForwardBlock, "", true)
		// A Staff Group with a sentinel title keeps the site's group, and the member's message is a forward of a source
		// with a sentinel title: neither may reach the private copy (D-31).
		const staffTitle, sourceTitle = "Sentinel Staff HQ", "Sentinel Source"
		staffChat, source, owner := uniqueModuleChatID(), uniqueModuleChatID(), uniqueLinkOwnerID()
		staffCleanup(t, staffChat, s.chat.Id)
		if _, err := staff.CreateStaffGroup(staffChat, owner, staffTitle); err != nil {
			t.Fatalf("create the Staff Group: %v", err)
		}
		if err := staff.CreateLink(&models.StaffGroupLink{GroupChatID: s.chat.Id, StaffChatID: staffChat, OwnerUserID: owner, GroupTitle: s.chat.Title}); err != nil {
			t.Fatalf("link the group: %v", err)
		}
		ctx := noticeGoldenMessage(s, 101, "forwarded post", false)
		ctx.EffectiveMessage.ForwardOrigin = gotgbot.MessageOriginChannel{
			Date: 1, Chat: gotgbot.Chat{Id: source, Type: "channel", Title: sourceTitle}, MessageId: 5,
		}
		if err := forwardBlockBan(s.bot, ctx, noticeGoldenTarget.Id); err != nil {
			t.Fatalf("forwardBlockBan error = %v", err)
		}
		eventMsgAutoPunishedFirst(t, s, "banChatMember")
		eventPMAutoWant(t, s, eventPMAutoDefault(t, "eventmsg_pm_fwdblock",
			i18n.TranslationParams{"group": eventPMAutoGroup, "action": eventActionWord(tr, "ban")}, html.EscapeString(forwardBlockReason(tr))))
		_, private := eventPMCalls(t, s)
		request := legacyRequestJSON(t, moduleBotCall{Method: private[0].Method, Params: private[0].Params})
		for _, forbidden := range []string{staffTitle, sourceTitle, strconv.FormatInt(source, 10), strconv.FormatInt(staffChat, 10)} {
			if strings.Contains(request, forbidden) {
				t.Fatalf("the forward blocklist's private copy names %q: %s", forbidden, request)
			}
		}
	})
}

// eventPMAutoOffender is a message of member in the site's chat, dated by the fixed clock.
func eventPMAutoOffender(s *botSite, id int64, member gotgbot.User, text string) *ext.Context {
	msg := &gotgbot.Message{MessageId: id, Date: s.now.Unix(), Chat: s.chat, From: &member, Text: text}
	return ext.NewContext(s.bot, &gotgbot.Update{UpdateId: id, Message: msg}, nil)
}

// TestEventPMAutoCap floods one group with blacklist offenders in one minute: every one is muted and gets the group
// notice, but only the per-group cap's number of private copies is sent, while a /warn's copy in the same minute is
// never capped (D-07, D-28).
func TestEventPMAutoCap(t *testing.T) {
	withModerationGoldenLocales(t)
	s := newEventPMAutoSite(t, deletion.KindBotAuto)
	t.Cleanup(func() {
		if err := blacklists.RemoveAllBlacklist(s.chat.Id); err != nil {
			t.Errorf("RemoveAllBlacklist error = %v", err)
		}
	})
	if err := blacklists.AddBlacklist(s.chat.Id, "spam"); err != nil {
		t.Fatalf("AddBlacklist error = %v", err)
	}
	if err := blacklists.SetBlacklistAction(s.chat.Id, "mute"); err != nil {
		t.Fatalf("SetBlacklistAction error = %v", err)
	}
	storeEventPM(t, s.chat.Id, eventmsg.EventBlacklist, "", true)
	storeEventPM(t, s.chat.Id, eventmsg.EventWarn, "", true)
	now := time.Date(2026, 11, 21, 12, 0, 5, 0, time.UTC)
	mreqSwap(t, &pmCopyNow, func() time.Time { return now })
	mreqSwap(t, &pmCopyAutoCapPerMinute, 2)
	t.Cleanup(pmCopyJobs.wait)

	offenders := eventPMUsers(5)
	for i, id := range offenders {
		member := gotgbot.User{Id: id, FirstName: "Offender"}
		if err := blacklistsModule.blacklistWatcher(s.bot, eventPMAutoOffender(s, int64(201+i), member, "buy spam now")); err != ext.ContinueGroups {
			t.Fatalf("blacklistWatcher(offender %d) error = %v, want ContinueGroups", i, err)
		}
	}
	muted := map[string]bool{}
	for _, call := range s.client.callsFor("restrictChatMember") {
		muted[fmt.Sprint(call.Params["user_id"])] = true
	}
	for _, id := range offenders {
		if !muted[strconv.FormatInt(id, 10)] {
			t.Fatalf("offender %d was not muted: every punishment must happen", id)
		}
	}
	if group, _ := eventPMCalls(t, s); len(group) != len(offenders) {
		t.Fatalf("group notices = %d, want one per offender (%d)", len(group), len(offenders))
	}
	if n := eventPMSentTo(t, s, offenders); n != 2 {
		t.Fatalf("private copies sent in one minute = %d, want the cap of 2", n)
	}

	noticeGoldenWarn(t, s, warnsModule.warnUser, "/warn spam")
	if n := eventPMSentTo(t, s, []int64{eventPMMember}); n != 1 {
		t.Fatalf("private copies of the /warn = %d, want 1: a /warn is never capped", n)
	}
}
