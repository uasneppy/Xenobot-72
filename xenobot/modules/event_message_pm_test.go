//go:build testtools

package modules

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"strconv"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/admin"
	"github.com/uasneppy/Xenobot-72/xenobot/db/deletion"
	"github.com/uasneppy/Xenobot-72/xenobot/db/eventmsg"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/cache"
)

// eventPMWantGroupGolden checks that the one send to the group is the golden default reply.
func eventPMWantGroupGolden(t *testing.T, s *botSite, golden string) {
	t.Helper()
	group, _ := eventPMCalls(t, s)
	if len(group) != 1 || group[0].Method != "sendMessage" {
		t.Fatalf("sends to the group = %d, want the one default reply", len(group))
	}
	got := strings.ReplaceAll(legacyRequestJSON(t, moduleBotCall{Method: group[0].Method, Params: group[0].Params}),
		strconv.FormatInt(s.chat.Id, 10), "<chat>")
	if got != golden {
		t.Fatalf("group reply changed\n got %q\nwant %q", got, golden)
	}
}

// eventPMWantBan checks that the member was banned once, before any reply.
func eventPMWantBan(t *testing.T, s *botSite) {
	t.Helper()
	bans := s.client.callsFor("banChatMember")
	if len(bans) != 1 || fmt.Sprint(bans[0].Params["user_id"]) != strconv.FormatInt(eventPMMember, 10) {
		t.Fatalf("banChatMember calls = %v, want one of user %d", bans, eventPMMember)
	}
	methods := eventMsgMethods(s.client)
	if ban, send := eventMsgIndex(methods, "banChatMember"), eventMsgIndex(methods, "sendMessage"); send >= 0 && send < ban {
		t.Fatalf("calls = %v, want the ban before every send", methods)
	}
}

func TestEventPMEndToEnd(t *testing.T) {
	withModerationGoldenLocales(t)

	t.Run("custom copy", func(t *testing.T) {
		s := newEventPMSite(t)
		storeEventPM(t, s.chat.Id, eventmsg.EventBan, "<b>{first}</b>, out of {chatname}: {reason} (by {admin})", true)
		if err := bansModule.ban(s.bot, s.commandCtx("/ban spam <b>", true, false)); err != ext.EndGroups {
			t.Fatalf("/ban error = %v, want EndGroups", err)
		}
		eventPMWantBan(t, s)
		eventPMWantGroupGolden(t, s, goldenModerationBanReason)
		params := eventPMOnly(t, s, "sendMessage")
		want := "<b>Ann &amp; &lt;Bo&gt;</b>, out of Bot Site Chat: spam &lt;b&gt; (by Telegram)"
		if params["text"] != want {
			t.Fatalf("private copy text = %q, want %q", params["text"], want)
		}
		if preview, _ := params["link_preview_options"].(map[string]any); preview["is_disabled"] != true {
			t.Fatalf("private copy link_preview_options = %v, want previews off", params["link_preview_options"])
		}
		if params["reply_markup"] != nil {
			t.Fatalf("private copy reply_markup = %v, want none (no Unban row)", params["reply_markup"])
		}
		client := cache.GetRedisClient()
		if n, err := client.Exists(context.Background(), "xenobot:pmcopy:pace:next").Result(); err != nil || n != 1 {
			t.Fatalf("the private copies' pacer key exists = %d (%v), want the copy paced on its own keys", n, err)
		}
	})

	t.Run("dban uses the ban copy", func(t *testing.T) {
		s := newEventPMSite(t)
		storeEventPM(t, s.chat.Id, eventmsg.EventBan, "out: {reason}", true)
		if err := bansModule.dBan(s.bot, s.commandCtx("/dban bad post", true, false)); err != ext.EndGroups {
			t.Fatalf("/dban error = %v, want EndGroups", err)
		}
		deleted := false
		for _, call := range s.client.callsFor("deleteMessage") {
			deleted = deleted || fmt.Sprint(call.Params["message_id"]) == "303"
		}
		if !deleted {
			t.Fatal("deleteMessage of message 303 not sent")
		}
		eventPMWantBan(t, s)
		eventPMWantGroupGolden(t, s, goldenModerationDban)
		if params := eventPMOnly(t, s, "sendMessage"); params["text"] != "out: bad post" {
			t.Fatalf("private copy text = %q, want the ban copy with the reason", params["text"])
		}
	})

	t.Run("off by default", func(t *testing.T) {
		s := newEventPMSite(t)
		storeEventPM(t, s.chat.Id, eventmsg.EventBan, "never sent {reason}", false)
		if err := bansModule.ban(s.bot, s.commandCtx("/ban spam <b>", true, false)); err != ext.EndGroups {
			t.Fatalf("/ban error = %v, want EndGroups", err)
		}
		eventPMWantBan(t, s)
		eventPMWantGroupGolden(t, s, goldenModerationBanReason)
		eventPMNone(t, s)
	})

	t.Run("no pm row", func(t *testing.T) {
		s := newEventPMSite(t)
		if err := bansModule.ban(s.bot, s.commandCtx("/ban spam <b>", true, false)); err != ext.EndGroups {
			t.Fatalf("/ban error = %v, want EndGroups", err)
		}
		eventPMWantGroupGolden(t, s, goldenModerationBanReason)
		eventPMNone(t, s)
	})

	t.Run("read error", func(t *testing.T) {
		s := newEventPMSite(t)
		storeEventPM(t, s.chat.Id, eventmsg.EventBan, "never sent", true)
		mreqSwap(t, &eventMessagesLookup, func(context.Context, int64) (*eventmsg.Messages, error) {
			return nil, errors.New("database unavailable")
		})
		if err := bansModule.ban(s.bot, s.commandCtx("/ban spam <b>", true, false)); err != ext.EndGroups {
			t.Fatalf("/ban error = %v, want EndGroups", err)
		}
		eventPMWantBan(t, s)
		eventPMWantGroupGolden(t, s, goldenModerationBanReason)
		eventPMNone(t, s)
	})

	t.Run("group silent", func(t *testing.T) {
		s := newEventPMSite(t)
		if err := eventmsg.SetSilent(t.Context(), s.chat.Id, eventmsg.EventBan, eventmsg.DestGroup, true, 1); err != nil {
			t.Fatalf("SetSilent(group) error = %v", err)
		}
		storeEventPM(t, s.chat.Id, eventmsg.EventBan, "out: {reason}", true)
		if err := bansModule.ban(s.bot, s.commandCtx("/ban spam", true, false)); err != ext.EndGroups {
			t.Fatalf("/ban error = %v, want EndGroups", err)
		}
		eventPMWantBan(t, s)
		if group, _ := eventPMCalls(t, s); len(group) != 0 {
			t.Fatalf("sends to the group = %d, want none when the group message is silent", len(group))
		}
		if params := eventPMOnly(t, s, "sendMessage"); params["text"] != "out: spam" {
			t.Fatalf("private copy text = %q, want the copy although the group message is silent", params["text"])
		}
	})
}

func TestEventPMSpecs(t *testing.T) {
	manual := []string{
		eventmsg.EventBan, eventmsg.EventTimedBan, eventmsg.EventKick, eventmsg.EventMute,
		eventmsg.EventTimedMute, eventmsg.EventUnban, eventmsg.EventUnmute,
	}
	// Phase 23 (D-28): the warn and the warn limit are manual specs (a rule-raised reply is capped through
	// eventPMOffer.ByRule), and the flood, blacklist and forward blocklist punishments are automatic.
	manual = append(manual, eventmsg.EventWarn, eventmsg.EventWarnLimit)
	automatic := []string{eventmsg.EventFlood, eventmsg.EventBlacklist, eventmsg.EventForwardBlock}
	// Phase 24 (D-34): the failed captcha, the AI spam notice and the four requirement notices are automatic, so the
	// per-group cap bounds their copies.
	automatic = append(automatic, eventmsg.EventCaptcha, eventmsg.EventAISpam, eventmsg.EventReqWarn,
		eventmsg.EventReqKick, eventmsg.EventReqBan, eventmsg.EventReqMute)
	if len(eventPMSpecs) != len(manual)+len(automatic) {
		t.Fatalf("specs = %d, want the %d manual and %d automatic events", len(eventPMSpecs), len(manual), len(automatic))
	}
	for _, event := range manual {
		spec, ok := eventPMSpecFor(event)
		if !ok || spec.DefaultKey != "eventmsg_pm_"+event || spec.Automatic {
			t.Fatalf("spec of %s = %+v (%v), want the key eventmsg_pm_%s and not automatic", event, spec, ok, event)
		}
	}
	for _, event := range automatic {
		spec, ok := eventPMSpecFor(event)
		if !ok || spec.DefaultKey != "eventmsg_pm_"+event || !spec.Automatic {
			t.Fatalf("spec of %s = %+v (%v), want the key eventmsg_pm_%s and automatic", event, spec, ok, event)
		}
	}
	if _, ok := eventPMSpecFor("warning"); ok {
		t.Fatal("spec of an unknown event exists, want none")
	}
	// Phase 25 (D-04, D-35): no lockdown event has a private copy; the count above stays eighteen.
	for _, event := range []string{
		eventmsg.EventLockdown,
		eventmsg.EventLockJoinSurge,
		eventmsg.EventLockFlood,
		eventmsg.EventLockAIBurst,
		eventmsg.EventLockLift,
	} {
		if spec, ok := eventPMSpecFor(event); ok {
			t.Fatalf("spec of the lockdown event %s = %+v, want none", event, spec)
		}
	}

	panics := func(t *testing.T, name string, register func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Fatalf("%s did not panic", name)
			}
		}()
		register()
	}
	saved := eventPMSpecs
	t.Cleanup(func() { eventPMSpecs = saved })
	eventPMSpecs = map[string]eventPMSpec{}
	panics(t, "an unknown event", func() {
		registerEventPM("nope", eventPMSpec{DefaultKey: "eventmsg_pm_ban", PlainKey: "eventmsg_plain_pm_ban"})
	})
	panics(t, "an empty key", func() { registerEventPM(eventmsg.EventBan, eventPMSpec{}) })
	panics(t, "an empty plain key", func() { registerEventPM(eventmsg.EventBan, eventPMSpec{DefaultKey: "eventmsg_pm_ban"}) })
	registerEventPM(eventmsg.EventBan, eventPMSpec{DefaultKey: "eventmsg_pm_ban", PlainKey: "eventmsg_plain_pm_ban"})
	panics(t, "a duplicate", func() {
		registerEventPM(eventmsg.EventBan, eventPMSpec{DefaultKey: "eventmsg_pm_ban", PlainKey: "eventmsg_plain_pm_ban"})
	})
	if len(eventPMSpecs) != 1 {
		t.Fatalf("specs after the refused registrations = %d, want 1", len(eventPMSpecs))
	}
}

// eventPMDefault is an event's default private text as the group's language (uk) words it, with the
// group's escaped title and, when set, the end, the reason line and the note.
func eventPMDefault(t *testing.T, key, group, until, reason, note string) string {
	t.Helper()
	tr := i18n.MustNewTranslator("uk")
	text, err := tr.GetString(key, i18n.TranslationParams{"group": group, "until": until})
	if err != nil || text == "" {
		t.Fatalf("translate %s: %q, %v", key, text, err)
	}
	if reason != "" {
		line, _ := tr.GetString("eventmsg_pm_reason", i18n.TranslationParams{"reason": reason})
		text += "\n" + line
	}
	if note != "" {
		text += "\n" + note
	}
	return text
}

func TestEventPMDefaultTexts(t *testing.T) {
	withModerationGoldenLocales(t)
	const title, escaped = "Q&A <Group>", "Q&amp;A &lt;Group&gt;"
	note, _ := i18n.MustNewTranslator("uk").GetString("eventmsg_pm_unmute_lockdown")
	cases := []struct {
		name, text, event, method, key, reason, custom string
		reply, lockdown, timed                         bool
		call                                           func(b *gotgbot.Bot, ctx *ext.Context) error
	}{
		{name: "ban with a reason", text: "/ban spam <b>", reply: true, call: bansModule.ban, event: eventmsg.EventBan,
			method: "banChatMember", key: "eventmsg_pm_ban", reason: "spam &lt;b&gt;"},
		{name: "ban", text: "/ban", reply: true, call: bansModule.ban, event: eventmsg.EventBan,
			method: "banChatMember", key: "eventmsg_pm_ban"},
		{name: "tban", text: "/tban 2h", reply: true, call: bansModule.tBan, event: eventmsg.EventTimedBan,
			method: "banChatMember", key: "eventmsg_pm_tban", timed: true},
		{name: "kick", text: "/kick", reply: true, call: bansModule.kick, event: eventmsg.EventKick,
			method: "unbanChatMember", key: "eventmsg_pm_kick"},
		{name: "mute", text: "/mute", reply: true, call: mutesModule.mute, event: eventmsg.EventMute,
			method: "restrictChatMember", key: "eventmsg_pm_mute"},
		{name: "tmute", text: "/tmute 30m", reply: true, call: mutesModule.tMute, event: eventmsg.EventTimedMute,
			method: "restrictChatMember", key: "eventmsg_pm_tmute", timed: true},
		{name: "unban", text: "/unban 42", call: bansModule.unban, event: eventmsg.EventUnban,
			method: "unbanChatMember", key: "eventmsg_pm_unban"},
		{name: "unmute", text: "/unmute 42", call: mutesModule.unmute, event: eventmsg.EventUnmute,
			method: "restrictChatMember", key: "eventmsg_pm_unmute"},
		{name: "unmute during a lockdown", text: "/unmute 42", call: mutesModule.unmute, event: eventmsg.EventUnmute,
			method: "restrictChatMember", key: "eventmsg_pm_unmute", lockdown: true},
		{name: "custom unmute during a lockdown", text: "/unmute 42", call: mutesModule.unmute, event: eventmsg.EventUnmute,
			method: "restrictChatMember", lockdown: true, custom: "{first} can write"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newEventPMSite(t)
			s.chat.Title = title
			if tc.lockdown {
				moderationGoldenActiveLockdown(t, s.chat.Id)
			}
			storeEventPM(t, s.chat.Id, tc.event, tc.custom, true)
			if err := tc.call(s.bot, s.commandCtx(tc.text, tc.reply, false)); err != ext.EndGroups {
				t.Fatalf("%s error = %v, want EndGroups", tc.text, err)
			}
			until := eventMsgSiteUntil(t, s, tc.method)
			if tc.timed == (until == "") {
				t.Fatalf("%s until_date %q, timed %v", tc.method, until, tc.timed)
			}
			lockdownNote := ""
			if tc.lockdown {
				lockdownNote = note
			}
			want := "Ann &amp; &lt;Bo&gt; can write\n" + lockdownNote
			if tc.custom == "" {
				want = eventPMDefault(t, tc.key, escaped, until, tc.reason, lockdownNote)
			}
			params := eventPMOnly(t, s, "sendMessage")
			if params["text"] != want {
				t.Fatalf("private copy text = %q, want %q", params["text"], want)
			}
			if strings.Contains(fmt.Sprint(params["text"]), botSiteAdmin.FirstName) {
				t.Fatalf("private copy %q names the admin", params["text"])
			}
			if params["reply_markup"] != nil {
				t.Fatalf("private copy reply_markup = %v, want none", params["reply_markup"])
			}
		})
	}

	// The warn limit's default fills the count, the limit and the action word of the group's language,
	// then the reason line (Phase 23, D-29).
	t.Run("warn limit", func(t *testing.T) {
		s := newEventPMAutoSite(t, deletion.KindBotManual)
		s.chat.Title = title
		eventMsgAutoWarnSettings(t, s, 3, "ban")
		storeEventPM(t, s.chat.Id, eventmsg.EventWarnLimit, "", true)
		noticeGoldenWarn(t, s, warnsModule.warnUser, "/warn first", "/warn second", "/warn spam <b>")
		tr := i18n.MustNewTranslator("uk")
		want := eventPMAutoDefault(t, "eventmsg_pm_warnlimit", i18n.TranslationParams{
			"group": escaped, "count": "3", "limit": "3", "action": eventActionWord(tr, "ban"),
		}, "spam &lt;b&gt;")
		params := eventPMOnly(t, s, "sendMessage")
		if params["text"] != want {
			t.Fatalf("private copy text = %q, want %q", params["text"], want)
		}
	})

	// The requirement warn's default names the count and the limit and the action word, then the reason line, which
	// for a requirement event lists the broken rules (Phase 24, D-14, D-34). Its site switches in plan 24-05, so the
	// text is rendered here as the job renders it.
	t.Run("requirement warn", func(t *testing.T) {
		tr := i18n.MustNewTranslator("uk")
		spec, ok := eventPMSpecFor(eventmsg.EventReqWarn)
		if !ok {
			t.Fatal("the requirement warn has no private copy spec")
		}
		rules := memberReqRulesText(tr, 2|4)
		if !strings.Contains(rules, memberReqRulesSeparator) {
			t.Fatalf("the rules text %q does not list two rules", rules)
		}
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: title}
		action := eventActionWord(tr, "warn")
		got := eventPMDefaultText(tr, spec, &chat, eventValues{Reason: rules, Count: 1, Limit: 3, Action: action}, "", false)
		want := eventPMAutoDefault(t, "eventmsg_pm_mreqwarn", i18n.TranslationParams{
			"group": escaped, "count": "1", "limit": "3", "action": action,
		}, html.EscapeString(rules))
		if got != want {
			t.Fatalf("requirement warn default = %q, want %q", got, want)
		}
	})
}

func TestEventPMComposition(t *testing.T) {
	withModerationGoldenLocales(t)
	stored := func(t *testing.T, s *botSite, text, fileID string, mediaType int, buttons []models.Button) {
		t.Helper()
		storeEventPM(t, s.chat.Id, eventmsg.EventBan, text, false)
		if fileID != "" {
			if err := eventmsg.SetMedia(t.Context(), s.chat.Id, eventmsg.EventBan, eventmsg.DestPM, fileID, mediaType, 1); err != nil {
				t.Fatalf("SetMedia(pm) error = %v", err)
			}
		}
		if len(buttons) > 0 {
			if err := eventmsg.SetButtons(t.Context(), s.chat.Id, eventmsg.EventBan, eventmsg.DestPM, buttons, 1); err != nil {
				t.Fatalf("SetButtons(pm) error = %v", err)
			}
		}
		storeEventPM(t, s.chat.Id, eventmsg.EventBan, "", true)
	}
	keyboardOf := func(t *testing.T, params map[string]any) string {
		t.Helper()
		markup, _ := params["reply_markup"].(map[string]any)
		raw, err := json.Marshal(markup["inline_keyboard"])
		if err != nil {
			t.Fatalf("encode the keyboard: %v", err)
		}
		return string(raw)
	}

	t.Run("a photo with a text and buttons", func(t *testing.T) {
		s := newEventPMSite(t)
		stored(t, s, "<b>{first}</b>: {reason}", "photo-file", db.PHOTO, []models.Button{eventMsgSiteButton})
		if err := bansModule.ban(s.bot, s.commandCtx("/ban spam", true, false)); err != ext.EndGroups {
			t.Fatalf("/ban error = %v, want EndGroups", err)
		}
		params := eventPMOnly(t, s, "sendPhoto")
		if params["caption"] != "<b>Ann &amp; &lt;Bo&gt;</b>: spam" || params["photo"] != "photo-file" {
			t.Fatalf("sendPhoto caption %q photo %v, want the filled text and the stored file", params["caption"], params["photo"])
		}
		if got := keyboardOf(t, params); got != "["+eventMsgSiteRow+"]" {
			t.Fatalf("keyboard = %s, want the stored button only (no Unban row)", got)
		}
	})

	t.Run("buttons with no text", func(t *testing.T) {
		s := newEventPMSite(t)
		stored(t, s, "", "", 0, []models.Button{eventMsgSiteButton})
		if err := bansModule.ban(s.bot, s.commandCtx("/ban", true, false)); err != ext.EndGroups {
			t.Fatalf("/ban error = %v, want EndGroups", err)
		}
		params := eventPMOnly(t, s, "sendMessage")
		if want := eventPMDefault(t, "eventmsg_pm_ban", "Bot Site Chat", "", "", ""); params["text"] != want {
			t.Fatalf("text = %q, want the default private text %q", params["text"], want)
		}
		if got := keyboardOf(t, params); got != "["+eventMsgSiteRow+"]" {
			t.Fatalf("keyboard = %s, want the stored button only", got)
		}
	})

	t.Run("the rules button", func(t *testing.T) {
		s := newEventPMSite(t)
		rulesSeed(t, s.chat.Id, "Be kind", "", 0, nil)
		stored(t, s, "read {rules}", "", 0, nil)
		if err := bansModule.ban(s.bot, s.commandCtx("/ban", true, false)); err != ext.EndGroups {
			t.Fatalf("/ban error = %v, want EndGroups", err)
		}
		params := eventPMOnly(t, s, "sendMessage")
		got := keyboardOf(t, params)
		if params["text"] != "read " || !strings.Contains(got, "start=rules_"+strconv.FormatInt(s.chat.Id, 10)) || strings.Contains(got, "unrestrict") {
			t.Fatalf("text %q keyboard %s, want the rules button alone", params["text"], got)
		}
	})

	t.Run("an anonymous admin in AnonAdmin mode", func(t *testing.T) {
		s := newEventPMSite(t)
		stored(t, s, "out (by {admin})", "", 0, nil)
		if err := admin.SetAnonAdminMode(s.chat.Id, true); err != nil {
			t.Fatalf("SetAnonAdminMode error = %v", err)
		}
		t.Cleanup(func() { db.DB.Where("chat_id = ?", s.chat.Id).Delete(&models.AdminSettings{}) })
		if err := bansModule.ban(s.bot, eventMsgAnonCtx(s, "/ban spam")); err != ext.EndGroups {
			t.Fatalf("/ban error = %v, want EndGroups", err)
		}
		if params := eventPMOnly(t, s, "sendMessage"); params["text"] != "out (by адмін)" {
			t.Fatalf("text = %q, want {admin} as the group language's \"an admin\"", params["text"])
		}
	})

	t.Run("after the proof tap", func(t *testing.T) {
		s := newEventPMSite(t)
		stored(t, s, "out (by {admin})", "", 0, nil)
		eventMsgProofTap(t, s, "/ban spam")
		if params := eventPMOnly(t, s, "sendMessage"); params["text"] != "out (by адмін)" {
			t.Fatalf("text = %q, want {admin} as \"an admin\", not the tapper", params["text"])
		}
	})
}
