//go:build testtools

package modules

import (
	"encoding/json"
	"fmt"
	"html"
	"strconv"
	"strings"
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
	"github.com/uasneppy/Xenobot-72/xenobot/db/staff"
	"github.com/uasneppy/Xenobot-72/xenobot/db/warns"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/formatting"
)

// The send sites of the Automatic family other than the warn (Phase 23 plan 23-04): the warn limit, the
// flood notice, the blacklist notices and the forward blocklist notice, each through postEvent. Every
// subtest runs on its own notice golden site (fresh chat, real locales) and compares a group with nothing
// usable stored with the golden of TestLegacyPunishmentNoticeRequests.

// eventMsgAutoWarnLimitText is the custom warn limit text: every event placeholder the warn limit fills.
const eventMsgAutoWarnLimitText = "{mention}: {action} at {count}/{limit} ({reason}, {admin})"

// eventMsgAutoWarnSettings sets the site's warn limit and limit action.
func eventMsgAutoWarnSettings(t *testing.T, s *botSite, limit int, mode string) {
	t.Helper()
	if err := warns.SetWarnLimit(s.chat.Id, limit); err != nil {
		t.Fatalf("SetWarnLimit() error = %v", err)
	}
	if err := warns.SetWarnMode(s.chat.Id, mode); err != nil {
		t.Fatalf("SetWarnMode(%q) error = %v", mode, err)
	}
}

// eventMsgAutoSilent turns the site's group message of event Silent.
func eventMsgAutoSilent(t *testing.T, s *botSite, event string) {
	t.Helper()
	eventMsgCleanup(t, s.chat.Id)
	if err := eventmsg.SetSilent(t.Context(), s.chat.Id, event, eventmsg.DestGroup, true, 1); err != nil {
		t.Fatalf("SetSilent(%s) error = %v", event, err)
	}
}

// eventMsgAutoPunishedFirst fails unless the site made punishment before its first sendMessage (D-03).
func eventMsgAutoPunishedFirst(t *testing.T, s *botSite, punishment string) {
	t.Helper()
	methods := eventMsgMethods(s.client)
	punished, sent := eventMsgIndex(methods, punishment), eventMsgIndex(methods, "sendMessage")
	if punished < 0 || sent < 0 || punished > sent {
		t.Fatalf("calls = %v, want %s before the notice", methods, punishment)
	}
}

// eventMsgAutoCustom checks that the site sent exactly one sendMessage, with text want, the keyboard rows
// rows (each as JSON, none for no keyboard) and the reply parameters reply (as JSON, empty for none), and
// that it is queued under kind.
func eventMsgAutoCustom(t *testing.T, s *botSite, want string, rows []string, reply, kind string) {
	t.Helper()
	sends := s.client.callsFor("sendMessage")
	if len(sends) != 1 {
		t.Fatalf("sendMessage calls = %d, want the one custom message", len(sends))
	}
	params := rulesCallParams(t, sends[0])
	if params["text"] != want {
		t.Fatalf("text = %q, want %q", params["text"], want)
	}
	if got := eventMsgRows(params); strings.Join(got, "\n") != strings.Join(rows, "\n") {
		t.Fatalf("keyboard rows = %v, want %v", got, rows)
	}
	got := ""
	if raw, ok := params["reply_parameters"]; ok {
		encoded, _ := json.Marshal(raw)
		got = string(encoded)
	}
	if got != reply {
		t.Fatalf("reply_parameters = %q, want %q", got, reply)
	}
	s.wantRow(s.chat.Id, 9001, kind, 600*time.Second)
}

// eventMsgAutoGolden fails unless the site's last sendMessage, after sends of them in all, equals golden.
func eventMsgAutoGolden(t *testing.T, s *botSite, sends int, golden string) {
	t.Helper()
	calls := s.client.callsFor("sendMessage")
	if len(calls) != sends {
		t.Fatalf("sendMessage calls = %d, want %d", len(calls), sends)
	}
	got := strings.ReplaceAll(legacyRequestJSON(t, calls[len(calls)-1]), strconv.FormatInt(s.chat.Id, 10), "<chat>")
	if got != golden {
		t.Fatalf("request\n got %q\nwant %q", got, golden)
	}
}

func TestEventMessageWarnLimit(t *testing.T) {
	withModerationGoldenLocales(t)
	tr := i18n.MustNewTranslator("uk")
	anon, _ := tr.GetString("eventmsg_admin_anon")
	const replyToCommand = `{"allow_sending_without_reply":true,"message_id":101}`

	t.Run("custom ban", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotManual)
		eventMsgAutoWarnSettings(t, s, 1, "ban")
		eventMsgStore(t, s.chat.Id, eventmsg.EventWarnLimit, eventMsgAutoWarnLimitText)
		noticeGoldenWarn(t, s, warnsModule.warnUser, "/warn spam & co")
		eventMsgAutoPunishedFirst(t, s, "banChatMember")
		want := "@ann_lee: " + eventActionWord(tr, "ban") + " at 1/1 (spam &amp; co, Telegram)"
		eventMsgAutoCustom(t, s, want, nil, replyToCommand, deletion.KindBotManual)
		if got, _ := warns.GetWarns(noticeGoldenTarget.Id, s.chat.Id); got != 0 {
			t.Fatalf("stored warns = %d, want 0 after the limit action", got)
		}
	})

	t.Run("automatic", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		eventMsgAutoWarnSettings(t, s, 1, "mute")
		eventMsgStore(t, s.chat.Id, eventmsg.EventWarnLimit, eventMsgAutoWarnLimitText)
		noticeGoldenBlacklist(t, s, "warn", false, ext.EndGroups)
		eventMsgAutoPunishedFirst(t, s, "restrictChatMember")
		want := "@ann_lee: " + eventActionWord(tr, "mute") + " at 1/1 (Blacklisted word: &#39;spam&#39;, " + anon + ")"
		eventMsgAutoCustom(t, s, want, nil, replyToCommand, deletion.KindBotAuto)
	})

	t.Run("inert stays", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotManual)
		eventMsgAutoWarnSettings(t, s, 1, "tban")
		eventMsgStore(t, s.chat.Id, eventmsg.EventWarnLimit, eventMsgAutoWarnLimitText)
		noticeGoldenWarn(t, s, warnsModule.warnUser, "/warn reason")
		eventMsgAutoGolden(t, s, 1, goldenNoticeWarnLimitInert)
		s.wantRow(s.chat.Id, 9001, deletion.KindBotManual, 600*time.Second)
	})

	t.Run("silent", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotManual)
		eventMsgAutoWarnSettings(t, s, 1, "kick")
		eventMsgAutoSilent(t, s, eventmsg.EventWarnLimit)
		noticeGoldenWarn(t, s, warnsModule.warnUser, "/warn reason")
		if got := len(s.client.callsFor("unbanChatMember")); got != 1 {
			t.Fatalf("unbanChatMember calls = %d, want the kick", got)
		}
		if got, _ := warns.GetWarns(noticeGoldenTarget.Id, s.chat.Id); got != 0 {
			t.Fatalf("stored warns = %d, want 0 after the limit action", got)
		}
		if got := len(s.client.callsFor("sendMessage")); got != 0 {
			t.Fatalf("sendMessage calls = %d, want none when silent", got)
		}
		s.wantNoRows(s.chat.Id)
	})
}

// eventMsgAutoFloodText is the custom flood text: every event placeholder the flood fills.
const eventMsgAutoFloodText = "{mention} {action} {count}/{limit}: {reason}"

// eventMsgAutoRow is one keyboard row of a single callback button as eventMsgRows reports it.
func eventMsgAutoRow(t *testing.T, tr *i18n.Translator, key, action string, id int64) string {
	t.Helper()
	text, _ := tr.GetString(key)
	data := encodeCallbackData("unrestrict", map[string]string{"a": action, "u": strconv.FormatInt(id, 10)})
	raw, err := json.Marshal([]map[string]string{{"callback_data": data, "text": text}})
	if err != nil {
		t.Fatalf("encode the %s row: %v", action, err)
	}
	return string(raw)
}

// eventMsgAutoFlood is noticeGoldenFlood with the target's messages in topic thread (0 for none): a flood
// limit of 1 with action, and messages 101 and 102 through checkFlood, so the second one floods.
func eventMsgAutoFlood(t *testing.T, s *botSite, action string, channel bool, thread int64) {
	t.Helper()
	resetAntifloodState(t)
	t.Cleanup(func() {
		db.DB.Where("chat_id = ?", s.chat.Id).Delete(&models.AntifloodSettings{})
		dbcache.DeleteCache(dbcache.CacheKey("antiflood", s.chat.Id))
		resetAntifloodState(t)
	})
	if err := antiflood.SetFlood(s.chat.Id, 1); err != nil {
		t.Fatalf("SetFlood() error = %v", err)
	}
	if err := antiflood.SetFloodMode(s.chat.Id, action); err != nil {
		t.Fatalf("SetFloodMode(%q) error = %v", action, err)
	}
	for _, id := range []int64{101, 102} {
		ctx := noticeGoldenMessage(s, id, "flood "+strconv.FormatInt(id, 10), channel)
		ctx.EffectiveMessage.MessageThreadId = thread
		ctx.EffectiveMessage.IsTopicMessage = thread != 0
		if err := antifloodModule.checkFlood(s.bot, ctx); err != ext.ContinueGroups {
			t.Fatalf("checkFlood(message %d) error = %v, want ContinueGroups", id, err)
		}
	}
}

func TestEventMessageFlood(t *testing.T) {
	withModerationGoldenLocales(t)
	tr := i18n.MustNewTranslator("uk")
	reason := html.EscapeString(floodEventReason(tr))

	t.Run("custom mute", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		eventMsgStoreRow(t, s.chat.Id, eventmsg.EventFlood, eventMsgAutoFloodText, "", 0, []models.Button{eventMsgSiteButton})
		eventMsgAutoFlood(t, s, "mute", false, 7)
		if !eventMsgAutoDeleted(s, 102) {
			t.Fatal("deleteMessage of message 102 not sent")
		}
		eventMsgAutoPunishedFirst(t, s, "restrictChatMember")
		unmute := eventMsgAutoRow(t, tr, "button_unmute_admins", "unmute", noticeGoldenTarget.Id)
		want := "@ann_lee " + eventActionWord(tr, "mute") + " 2/1: " + reason
		eventMsgAutoCustom(t, s, want, []string{unmute, eventMsgSiteRow}, "", deletion.KindBotAuto)
		params := rulesCallParams(t, s.client.callsFor("sendMessage")[0])
		if got := fmt.Sprint(params["message_thread_id"]); got != "7" {
			t.Fatalf("message_thread_id = %s, want the flooding message's topic 7", got)
		}
	})

	t.Run("channel ban", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		eventMsgStoreRow(t, s.chat.Id, eventmsg.EventFlood, eventMsgAutoFloodText, "", 0, []models.Button{eventMsgSiteButton})
		eventMsgAutoFlood(t, s, "ban", true, 0)
		eventMsgAutoPunishedFirst(t, s, "banChatSenderChat")
		unban := eventMsgAutoRow(t, tr, "antiflood_button_unban_admins", "unban", noticeGoldenChannel.Id)
		want := formatting.MentionHtml(noticeGoldenChannel.Id, noticeGoldenChannel.Title) + " " + eventActionWord(tr, "ban") + " 2/1: " + reason
		eventMsgAutoCustom(t, s, want, []string{unban, eventMsgSiteRow}, "", deletion.KindBotAuto)
	})

	t.Run("inert stays", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		eventMsgStore(t, s.chat.Id, eventmsg.EventFlood, eventMsgAutoFloodText)
		noticeGoldenFlood(t, s, "warn", false)
		eventMsgAutoGolden(t, s, 1, goldenNoticeFloodInert)
		s.wantRow(s.chat.Id, 9001, deletion.KindBotAuto, 600*time.Second)
	})

	t.Run("refused", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		eventMsgStore(t, s.chat.Id, eventmsg.EventFlood, eventMsgAutoFloodText)
		eventMsgRefuseOnce(s, "sendMessage", &gotgbot.TelegramError{Method: "sendMessage", Code: 400, Description: "Bad Request: can't parse entities"})
		noticeGoldenFlood(t, s, "mute", false)
		eventMsgAutoPunishedFirst(t, s, "restrictChatMember")
		eventMsgAutoGolden(t, s, 2, goldenNoticeFloodMute)
		s.wantRow(s.chat.Id, 9001, deletion.KindBotAuto, 600*time.Second)
	})
}

// eventMsgAutoDeleted reports whether the site deleted message id.
func eventMsgAutoDeleted(s *botSite, id int64) bool {
	for _, call := range s.client.callsFor("deleteMessage") {
		if fmt.Sprint(call.Params["message_id"]) == strconv.FormatInt(id, 10) {
			return true
		}
	}
	return false
}

// eventMsgAutoListText is the custom blacklist and forward blocklist text.
const eventMsgAutoListText = "{mention} {action}: {reason}"

func TestEventMessageBlacklist(t *testing.T) {
	withModerationGoldenLocales(t)
	tr := i18n.MustNewTranslator("uk")
	anon, _ := tr.GetString("eventmsg_admin_anon")
	const reason = "Blacklisted word: &#39;spam&#39;"

	t.Run("blacklist custom ban", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		eventMsgStore(t, s.chat.Id, eventmsg.EventBlacklist, eventMsgAutoListText)
		noticeGoldenBlacklist(t, s, "ban", false, ext.ContinueGroups)
		if !eventMsgAutoDeleted(s, 101) {
			t.Fatal("deleteMessage of message 101 not sent")
		}
		eventMsgAutoPunishedFirst(t, s, "banChatMember")
		eventMsgAutoCustom(t, s, "@ann_lee "+eventActionWord(tr, "ban")+": "+reason, nil, "", deletion.KindBotAuto)
	})

	t.Run("blacklist channel ban", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		eventMsgStore(t, s.chat.Id, eventmsg.EventBlacklist, eventMsgAutoListText)
		noticeGoldenBlacklist(t, s, "ban", true, ext.ContinueGroups)
		eventMsgAutoPunishedFirst(t, s, "banChatSenderChat")
		want := formatting.MentionHtml(noticeGoldenChannel.Id, noticeGoldenChannel.Title) + " " + eventActionWord(tr, "ban") + ": " + reason
		eventMsgAutoCustom(t, s, want, nil, "", deletion.KindBotAuto)
	})

	t.Run("blacklist warn is the warn event", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		eventMsgStoreWarn(t, s)
		eventMsgStore(t, s.chat.Id, eventmsg.EventBlacklist, eventMsgAutoListText)
		noticeGoldenBlacklist(t, s, "warn", false, ext.EndGroups)
		warned, _ := tr.GetString("eventmsg_action_warn")
		eventMsgWarnCustom(t, s, "<i>@ann_lee</i> 1/3: "+reason+" ("+anon+", "+warned+")", deletion.KindBotAuto)
	})

	t.Run("blacklist nothing stored", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		eventMsgCleanup(t, s.chat.Id)
		noticeGoldenBlacklist(t, s, "mute", false, ext.ContinueGroups)
		eventMsgAutoPunishedFirst(t, s, "restrictChatMember")
		eventMsgAutoGolden(t, s, 1, goldenNoticeBlacklistMute)
		s.wantRow(s.chat.Id, 9001, deletion.KindBotAuto, 600*time.Second)
	})
}

// eventMsgAutoEveryPlaceholder is a stored text with every Welcome and event placeholder.
const eventMsgAutoEveryPlaceholder = "{first}|{last}|{fullname}|{username}|{mention}|{id}|{chatname}|{rules}|" +
	"{admin}|{reason}|{action}|{count}|{limit}|{window}|{duration}|{until}"

func TestEventMessageForwardBlock(t *testing.T) {
	withModerationGoldenLocales(t)
	tr := i18n.MustNewTranslator("uk")
	anon, _ := tr.GetString("eventmsg_admin_anon")
	reason := html.EscapeString(forwardBlockReason(tr))

	t.Run("forward block custom", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		eventMsgStore(t, s.chat.Id, eventmsg.EventForwardBlock, eventMsgAutoEveryPlaceholder)
		// A Staff Group with a sentinel title keeps the site's group, and the member's message is a
		// forward of a source with a sentinel title: neither may reach the notice.
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
		if err := forwardBlockKick(s.bot, ctx, noticeGoldenTarget.Id); err != nil {
			t.Fatalf("forwardBlockKick error = %v", err)
		}
		eventMsgAutoPunishedFirst(t, s, "unbanChatMember")
		want := "Ann &amp; &lt;Bo&gt;|Lee|Ann &amp; &lt;Bo&gt; Lee|@ann_lee|@ann_lee|42|" + s.chat.Title + "||" +
			anon + "|" + reason + "|" + eventActionWord(tr, "kick") + "|||||"
		eventMsgAutoCustom(t, s, want, nil, "", deletion.KindBotAuto)
		request := legacyRequestJSON(t, s.client.callsFor("sendMessage")[0])
		for _, forbidden := range []string{staffTitle, sourceTitle, strconv.FormatInt(source, 10), strconv.FormatInt(staffChat, 10)} {
			if strings.Contains(request, forbidden) {
				t.Fatalf("the forward blocklist message names %q: %s", forbidden, request)
			}
		}
	})

	t.Run("forward block through the watcher", func(t *testing.T) {
		env := newFwdWatchEnv(t, "mute")
		eventMsgStore(t, env.group, eventmsg.EventForwardBlock, eventMsgAutoListText)
		member := env.member()
		env.forward(member)
		if calls := env.callsTo("restrictChatMember", env.group); len(calls) != 1 {
			t.Fatalf("restrictChatMember calls = %d, want the one mute", len(calls))
		}
		sent := env.fake.sentTo(env.group)
		if len(sent) != 1 {
			t.Fatalf("messages sent to the linked group = %d, want the one custom notice", len(sent))
		}
		text := fmt.Sprint(sent[0].Params["text"])
		// The staff environment installs its own locale, so the words come from it.
		envTr := i18n.MustNewTranslator("uk")
		want := formatting.MentionHtml(member.Id, member.FirstName) + " " + eventActionWord(envTr, "mute") + ": " +
			html.EscapeString(forwardBlockReason(envTr))
		if text != want {
			t.Fatalf("text = %q, want %q", text, want)
		}
		fwdForbiddenInText(t, env, "the custom notice", text)
	})
}
