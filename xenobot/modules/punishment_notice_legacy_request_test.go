//go:build testtools

package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/antiflood"
	"github.com/uasneppy/Xenobot-72/xenobot/db/blacklists"
	dbcache "github.com/uasneppy/Xenobot-72/xenobot/db/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/db/deletion"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/db/rules"
	"github.com/uasneppy/Xenobot-72/xenobot/db/warns"
)

// Goldens recorded on the code as it was before Phase 23 (the warn, warn limit, flood, blacklist and
// forward blocklist messages); <chat> stands for the case's chat ID. They are never re-recorded: a
// later change that alters one of these requests for a group with nothing stored is a regression.
const (
	goldenNoticeWarnReason          = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_markup\":{\"inline_keyboard\":[[{\"text\":\"❌ Зняти попередження\",\"callback_data\":\"rmWarn|v1|u=42\"}]]},\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Попередження про порушення: \\u003ca href=\\\"tg://user?id=42\\\"\\u003eAnn \\u0026amp; \\u0026lt;Bo\\u0026gt;\\u003c/a\\u003e, будьте обережні.\\n\\u003cb\\u003eПопереджень:\\u003c/b\\u003e 1/3\\n\\u003cb\\u003eПричина:\\u003c/b\\u003e spam \\u0026amp; \\u0026lt;b\\u0026gt;\"}"
	goldenNoticeWarnRules           = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_markup\":{\"inline_keyboard\":[[{\"text\":\"❌ Зняти попередження\",\"callback_data\":\"rmWarn|v1|u=42\"},{\"text\":\"Правила 📝\",\"url\":\"t.me/XenobotTestBot?start=rules_<chat>\"}]]},\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Попередження про порушення: \\u003ca href=\\\"tg://user?id=42\\\"\\u003eAnn \\u0026amp; \\u0026lt;Bo\\u0026gt;\\u003c/a\\u003e, будьте обережні.\\n\\u003cb\\u003eПопереджень:\\u003c/b\\u003e 1/3\"}"
	goldenNoticeDwarn               = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_markup\":{\"inline_keyboard\":[[{\"text\":\"❌ Зняти попередження\",\"callback_data\":\"rmWarn|v1|u=42\"}]]},\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Попередження про порушення: \\u003ca href=\\\"tg://user?id=42\\\"\\u003eAnn \\u0026amp; \\u0026lt;Bo\\u0026gt;\\u003c/a\\u003e, будьте обережні.\\n\\u003cb\\u003eПопереджень:\\u003c/b\\u003e 1/3\\n\\u003cb\\u003eПричина:\\u003c/b\\u003e bad post\"}"
	goldenNoticeSwarn               = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_markup\":{\"inline_keyboard\":[[{\"text\":\"❌ Зняти попередження\",\"callback_data\":\"rmWarn|v1|u=42\"}]]},\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Попередження про порушення: \\u003ca href=\\\"tg://user?id=42\\\"\\u003eAnn \\u0026amp; \\u0026lt;Bo\\u0026gt;\\u003c/a\\u003e, будьте обережні.\\n\\u003cb\\u003eПопереджень:\\u003c/b\\u003e 1/3\\n\\u003cb\\u003eПричина:\\u003c/b\\u003e noise\"}"
	goldenNoticeWarnBlacklist       = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_markup\":{\"inline_keyboard\":[[{\"text\":\"❌ Зняти попередження\",\"callback_data\":\"rmWarn|v1|u=42\"}]]},\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Попередження про порушення: \\u003ca href=\\\"tg://user?id=42\\\"\\u003eAnn \\u0026amp; \\u0026lt;Bo\\u0026gt;\\u003c/a\\u003e, будьте обережні.\\n\\u003cb\\u003eПопереджень:\\u003c/b\\u003e 1/3\\n\\u003cb\\u003eПричина:\\u003c/b\\u003e Blacklisted word: \\u0026#39;spam\\u0026#39;\"}"
	goldenNoticeWarnForwardBlock    = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_markup\":{\"inline_keyboard\":[[{\"text\":\"❌ Зняти попередження\",\"callback_data\":\"rmWarn|v1|u=42\"}]]},\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Попередження про порушення: \\u003ca href=\\\"tg://user?id=42\\\"\\u003eAnn \\u0026amp; \\u0026lt;Bo\\u0026gt;\\u003c/a\\u003e, будьте обережні.\\n\\u003cb\\u003eПопереджень:\\u003c/b\\u003e 1/3\\n\\u003cb\\u003eПричина:\\u003c/b\\u003e пересилання із заблокованого джерела\"}"
	goldenNoticeWarnLimitMute       = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_markup\":{},\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Попереджень: 2/2. \\u003ca href=\\\"tg://user?id=42\\\"\\u003eAnn \\u0026amp; \\u0026lt;Bo\\u0026gt;\\u003c/a\\u003e отримує мут.\\n - first\\n - second\"}"
	goldenNoticeWarnLimitKick       = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_markup\":{},\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Попереджень: 1/1. \\u003ca href=\\\"tg://user?id=42\\\"\\u003eAnn \\u0026amp; \\u0026lt;Bo\\u0026gt;\\u003c/a\\u003e вигнано.\\n - reason\"}"
	goldenNoticeWarnLimitBan        = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_markup\":{},\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Попереджень: 1/1. \\u003ca href=\\\"tg://user?id=42\\\"\\u003eAnn \\u0026amp; \\u0026lt;Bo\\u0026gt;\\u003c/a\\u003e забанено.\\n - reason\"}"
	goldenNoticeWarnLimitInert      = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_markup\":{},\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"\\n - reason\"}"
	goldenNoticeFloodMute           = "{\"chat_id\":<chat>,\"parse_mode\":\"HTML\",\"reply_markup\":{\"inline_keyboard\":[[{\"text\":\"Зняти мут (лише адміни)\",\"callback_data\":\"unrestrict|v1|a=unmute\\u0026u=42\"}]]},\"text\":\"Виявлено флуд. \\u003ca href=\\\"tg://user?id=42\\\"\\u003eAnn \\u0026amp; \\u0026lt;Bo\\u0026gt; Lee\\u003c/a\\u003e — дію виконано: muted.\"}"
	goldenNoticeFloodKick           = "{\"chat_id\":<chat>,\"parse_mode\":\"HTML\",\"reply_markup\":{},\"text\":\"Виявлено флуд. \\u003ca href=\\\"tg://user?id=42\\\"\\u003eAnn \\u0026amp; \\u0026lt;Bo\\u0026gt; Lee\\u003c/a\\u003e — дію виконано: kicked.\"}"
	goldenNoticeFloodBan            = "{\"chat_id\":<chat>,\"parse_mode\":\"HTML\",\"reply_markup\":{},\"text\":\"Виявлено флуд. \\u003ca href=\\\"tg://user?id=42\\\"\\u003eAnn \\u0026amp; \\u0026lt;Bo\\u0026gt; Lee\\u003c/a\\u003e — дію виконано: banned.\"}"
	goldenNoticeFloodBanChannel     = "{\"chat_id\":<chat>,\"parse_mode\":\"HTML\",\"reply_markup\":{\"inline_keyboard\":[[{\"text\":\"Розбанити (лише адміни)\",\"callback_data\":\"unrestrict|v1|a=unban\\u0026u=-1001234567890\"}]]},\"text\":\"Виявлено флуд. \\u003ca href=\\\"tg://user?id=-1001234567890\\\"\\u003eSpam Channel\\u003c/a\\u003e — дію виконано: banned.\"}"
	goldenNoticeFloodInert          = "{\"chat_id\":<chat>,\"parse_mode\":\"HTML\",\"reply_markup\":{},\"text\":\"Виявлено флуд. \\u003ca href=\\\"tg://user?id=42\\\"\\u003eAnn \\u0026amp; \\u0026lt;Bo\\u0026gt; Lee\\u003c/a\\u003e — дію виконано: .\"}"
	goldenNoticeBlacklistMute       = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"allow_sending_without_reply\":true},\"text\":\"\\u003ca href=\\\"tg://user?id=42\\\"\\u003eAnn \\u0026amp; \\u0026lt;Bo\\u0026gt; Lee\\u003c/a\\u003e отримує мут. Причина: Blacklisted word: 'spam'.\"}"
	goldenNoticeBlacklistKick       = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"allow_sending_without_reply\":true},\"text\":\"\\u003ca href=\\\"tg://user?id=42\\\"\\u003eAnn \\u0026amp; \\u0026lt;Bo\\u0026gt; Lee\\u003c/a\\u003e вигнано. Причина: Blacklisted word: 'spam'.\"}"
	goldenNoticeBlacklistBan        = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"allow_sending_without_reply\":true},\"text\":\"\\u003ca href=\\\"tg://user?id=42\\\"\\u003eAnn \\u0026amp; \\u0026lt;Bo\\u0026gt; Lee\\u003c/a\\u003e забанено. Причина: Blacklisted word: 'spam'.\"}"
	goldenNoticeBlacklistBanChannel = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"allow_sending_without_reply\":true},\"text\":\"\\u003ca href=\\\"tg://user?id=-1001234567890\\\"\\u003eSpam Channel\\u003c/a\\u003e забанено. Причина: Blacklisted word: 'spam'.\"}"
	goldenNoticeForwardBlockMute    = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"allow_sending_without_reply\":true},\"text\":\"\\u003ca href=\\\"tg://user?id=42\\\"\\u003eAnn \\u0026amp; \\u0026lt;Bo\\u0026gt; Lee\\u003c/a\\u003e отримує мут. Причина: пересилання із заблокованого джерела.\"}"
	goldenNoticeForwardBlockKick    = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"allow_sending_without_reply\":true},\"text\":\"\\u003ca href=\\\"tg://user?id=42\\\"\\u003eAnn \\u0026amp; \\u0026lt;Bo\\u0026gt; Lee\\u003c/a\\u003e вигнано. Причина: пересилання із заблокованого джерела.\"}"
	goldenNoticeForwardBlockBan     = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"allow_sending_without_reply\":true},\"text\":\"\\u003ca href=\\\"tg://user?id=42\\\"\\u003eAnn \\u0026amp; \\u0026lt;Bo\\u0026gt; Lee\\u003c/a\\u003e забанено. Причина: пересилання із заблокованого джерела.\"}"
)

// noticeGoldenTarget is the member the notice goldens punish, with the name of moderationGoldenTarget:
// the ampersand and the angle brackets pin how today's mentions escape it. The warn's mention uses the
// first name (from getChatMember), the flood, blacklist and forward blocklist mentions the full name
// (from the message's sender).
var noticeGoldenTarget = gotgbot.User{Id: 42, FirstName: "Ann & <Bo>", LastName: "Lee", Username: "ann_lee"}

// noticeGoldenChannel is the sender chat of the cases that punish a channel: neither the group's linked
// channel nor an anonymous administrator, so the flood and blacklist watchers take their sender-chat
// ban branches.
var noticeGoldenChannel = gotgbot.Chat{Id: -1001234567890, Type: "channel", Title: "Spam Channel"}

// noticeGoldenClient is the site's scripted client with two changes: a getChatMember of the target
// answers the target, with its golden name, as a plain member, and a getChat of the target's ID answers
// the target as a private chat. Every request is recorded by the embedded client first, and every other
// request is the embedded client's.
type noticeGoldenClient struct {
	*moduleBotClient
}

func (c noticeGoldenClient) RequestWithContext(ctx context.Context, token string, method string, params map[string]any, opts *gotgbot.RequestOpts) (json.RawMessage, error) {
	response, err := c.moduleBotClient.RequestWithContext(ctx, token, method, params, opts)
	if err != nil {
		return response, err
	}
	target := strconv.FormatInt(noticeGoldenTarget.Id, 10)
	switch {
	case method == "getChatMember" && fmt.Sprint(params["user_id"]) == target:
		return json.Marshal(map[string]any{"status": "member", "user": noticeGoldenTarget})
	case method == "getChat" && fmt.Sprint(params["chat_id"]) == target:
		return json.Marshal(gotgbot.Chat{
			Id: noticeGoldenTarget.Id, Type: "private", FirstName: noticeGoldenTarget.FirstName,
			LastName: noticeGoldenTarget.LastName, Username: noticeGoldenTarget.Username,
		})
	}
	return response, err
}

// newNoticeGoldenSite is a bot site in a fresh supergroup whose client answers the target, with the real
// locales, a 600 s rule for kind and a 900 s rule for the other bot punishment kind, so a row queued under
// the wrong kind shows. The warns, the requirement rows and the target's users row are removed when the
// test ends.
func newNoticeGoldenSite(t *testing.T, kind string) *botSite {
	t.Helper()
	if moderationGoldenLocales == 0 {
		withRealLocales(t)
	}
	s := newBotSite(t, "supergroup")
	s.bot.BotClient = noticeGoldenClient{moduleBotClient: s.client}
	other := deletion.KindBotAuto
	if kind == deletion.KindBotAuto {
		other = deletion.KindBotManual
	}
	s.setRule(s.chat.Id, kind, 600)
	s.setRule(s.chat.Id, other, 900)
	botSiteWarnsCleanup(t, s.chat.Id)
	memberReqCleanup(t, []int64{s.chat.Id}, []int64{noticeGoldenTarget.Id})
	return s
}

// noticeGoldenMessage is a message of the target (or, with channel, posted as noticeGoldenChannel) in the
// site's chat, dated by the fixed clock.
func noticeGoldenMessage(s *botSite, id int64, text string, channel bool) *ext.Context {
	from := noticeGoldenTarget
	msg := &gotgbot.Message{MessageId: id, Date: s.now.Unix(), Chat: s.chat, From: &from, Text: text}
	if channel {
		sender := noticeGoldenChannel
		msg.SenderChat = &sender
	}
	return ext.NewContext(s.bot, &gotgbot.Update{UpdateId: id, Message: msg}, nil)
}

// noticeGoldenWarn runs each warn command as the site's admin, replying to the target's message, and
// fails unless every one ends the update.
func noticeGoldenWarn(t *testing.T, s *botSite, call func(*gotgbot.Bot, *ext.Context) error, texts ...string) {
	t.Helper()
	for _, text := range texts {
		if err := call(s.bot, s.commandCtx(text, true, false)); err != ext.EndGroups {
			t.Fatalf("%s error = %v, want EndGroups", text, err)
		}
	}
}

// noticeGoldenFlood sets a flood limit of 1 with action and sends the target's messages 101 and 102
// through checkFlood, so the second one floods.
func noticeGoldenFlood(t *testing.T, s *botSite, action string, channel bool) {
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
		if err := antifloodModule.checkFlood(s.bot, noticeGoldenMessage(s, id, "flood "+strconv.FormatInt(id, 10), channel)); err != ext.ContinueGroups {
			t.Fatalf("checkFlood(message %d) error = %v, want ContinueGroups", id, err)
		}
	}
}

// noticeGoldenBlacklist lists the word spam with action and sends the target's matching message 101
// through the blacklist watcher, which must return want.
func noticeGoldenBlacklist(t *testing.T, s *botSite, action string, channel bool, want error) {
	t.Helper()
	t.Cleanup(func() {
		if err := blacklists.RemoveAllBlacklist(s.chat.Id); err != nil {
			t.Errorf("RemoveAllBlacklist error = %v", err)
		}
	})
	if err := blacklists.AddBlacklist(s.chat.Id, "spam"); err != nil {
		t.Fatalf("AddBlacklist error = %v", err)
	}
	if err := blacklists.SetBlacklistAction(s.chat.Id, action); err != nil {
		t.Fatalf("SetBlacklistAction(%q) error = %v", action, err)
	}
	if err := blacklistsModule.blacklistWatcher(s.bot, noticeGoldenMessage(s, 101, "this has spam inside", channel)); err != want {
		t.Fatalf("blacklistWatcher error = %v, want %v", err, want)
	}
}

// noticeGoldenForwardBlock runs one punishment of the forward blocklist on the target's message 101:
// the notice path without the watcher's link and authority steps.
func noticeGoldenForwardBlock(t *testing.T, s *botSite, punish func(*gotgbot.Bot, *ext.Context, int64) error) {
	t.Helper()
	if err := punish(s.bot, noticeGoldenMessage(s, 101, "forwarded post", false), noticeGoldenTarget.Id); err != nil {
		t.Fatalf("forward blocklist punishment error = %v", err)
	}
}

// noticeGoldenCase is one send of TestLegacyPunishmentNoticeRequests.
type noticeGoldenCase struct {
	name    string
	kind    string // the deletion kind the message is queued under
	run     func(t *testing.T, s *botSite)
	method  string // the punishment call; empty for none
	idKey   string // the parameter naming who is punished
	wantID  int64
	deletes int64  // the message deleted before the notice; 0 for none checked
	sends   int    // the sendMessage requests; the last one is pinned
	golden  string // the pinned sendMessage request
	// warnsLeft is the target's stored warns after the case, checked when checkWarns is set.
	warnsLeft  int
	checkWarns bool
}

// noticeGoldenPunishments are the calls a punishment can make; a case makes exactly its own and none of
// the others.
var noticeGoldenPunishments = []string{"restrictChatMember", "banChatMember", "unbanChatMember", "banChatSenderChat"}

func noticeGoldenCases() []noticeGoldenCase {
	manual, auto := deletion.KindBotManual, deletion.KindBotAuto
	user, channel := noticeGoldenTarget.Id, noticeGoldenChannel.Id
	warn := func(texts ...string) func(*testing.T, *botSite) {
		return func(t *testing.T, s *botSite) { noticeGoldenWarn(t, s, warnsModule.warnUser, texts...) }
	}
	warnLimit := func(limit int, mode string, texts ...string) func(*testing.T, *botSite) {
		return func(t *testing.T, s *botSite) {
			t.Helper()
			if err := warns.SetWarnLimit(s.chat.Id, limit); err != nil {
				t.Fatalf("SetWarnLimit() error = %v", err)
			}
			if err := warns.SetWarnMode(s.chat.Id, mode); err != nil {
				t.Fatalf("SetWarnMode(%q) error = %v", mode, err)
			}
			noticeGoldenWarn(t, s, warnsModule.warnUser, texts...)
		}
	}
	flood := func(action string, channel bool) func(*testing.T, *botSite) {
		return func(t *testing.T, s *botSite) { noticeGoldenFlood(t, s, action, channel) }
	}
	blacklist := func(action string, channel bool, want error) func(*testing.T, *botSite) {
		return func(t *testing.T, s *botSite) { noticeGoldenBlacklist(t, s, action, channel, want) }
	}
	forwardBlock := func(punish func(*gotgbot.Bot, *ext.Context, int64) error) func(*testing.T, *botSite) {
		return func(t *testing.T, s *botSite) { noticeGoldenForwardBlock(t, s, punish) }
	}
	return []noticeGoldenCase{
		{name: "warn_reason", kind: manual, run: warn("/warn spam & <b>"),
			sends: 1, golden: goldenNoticeWarnReason, warnsLeft: 1, checkWarns: true},
		{name: "warn_rules", kind: manual, run: func(t *testing.T, s *botSite) {
			t.Helper()
			mustLegacy(t, rules.SetChatRules(s.chat.Id, "Be kind"))
			noticeGoldenWarn(t, s, warnsModule.warnUser, "/warn")
		}, sends: 1, golden: goldenNoticeWarnRules, warnsLeft: 1, checkWarns: true},
		{name: "dwarn", kind: manual, run: func(t *testing.T, s *botSite) {
			noticeGoldenWarn(t, s, warnsModule.dWarnUser, "/dwarn bad post")
		}, deletes: 303, sends: 1, golden: goldenNoticeDwarn, warnsLeft: 1, checkWarns: true},
		{name: "swarn", kind: manual, run: func(t *testing.T, s *botSite) {
			noticeGoldenWarn(t, s, warnsModule.sWarnUser, "/swarn noise")
		}, deletes: 101, sends: 1, golden: goldenNoticeSwarn, warnsLeft: 1, checkWarns: true},
		{name: "warn_blacklist", kind: auto, run: blacklist("warn", false, ext.EndGroups),
			deletes: 101, sends: 1, golden: goldenNoticeWarnBlacklist, warnsLeft: 1, checkWarns: true},
		{name: "warn_forward_block", kind: auto, run: forwardBlock(forwardBlockWarn),
			sends: 1, golden: goldenNoticeWarnForwardBlock, warnsLeft: 1, checkWarns: true},
		{name: "warnlimit_mute", kind: manual, run: warnLimit(2, "mute", "/warn first", "/warn second"),
			method: "restrictChatMember", idKey: "user_id", wantID: user,
			sends: 2, golden: goldenNoticeWarnLimitMute, warnsLeft: 0, checkWarns: true},
		{name: "warnlimit_kick", kind: manual, run: warnLimit(1, "kick", "/warn reason"),
			method: "unbanChatMember", idKey: "user_id", wantID: user,
			sends: 1, golden: goldenNoticeWarnLimitKick, warnsLeft: 0, checkWarns: true},
		{name: "warnlimit_ban", kind: manual, run: warnLimit(1, "ban", "/warn reason"),
			method: "banChatMember", idKey: "user_id", wantID: user,
			sends: 1, golden: goldenNoticeWarnLimitBan, warnsLeft: 0, checkWarns: true},
		{name: "warnlimit_inert", kind: manual, run: warnLimit(1, "tban", "/warn reason"),
			sends: 1, golden: goldenNoticeWarnLimitInert, warnsLeft: 1, checkWarns: true},
		{name: "flood_mute", kind: auto, run: flood("mute", false),
			method: "restrictChatMember", idKey: "user_id", wantID: user,
			deletes: 102, sends: 1, golden: goldenNoticeFloodMute},
		{name: "flood_kick", kind: auto, run: flood("kick", false),
			method: "unbanChatMember", idKey: "user_id", wantID: user,
			deletes: 102, sends: 1, golden: goldenNoticeFloodKick},
		{name: "flood_ban", kind: auto, run: flood("ban", false),
			method: "banChatMember", idKey: "user_id", wantID: user,
			deletes: 102, sends: 1, golden: goldenNoticeFloodBan},
		{name: "flood_ban_channel", kind: auto, run: flood("ban", true),
			method: "banChatSenderChat", idKey: "sender_chat_id", wantID: channel,
			deletes: 102, sends: 1, golden: goldenNoticeFloodBanChannel},
		{name: "flood_inert", kind: auto, run: flood("warn", false),
			deletes: 102, sends: 1, golden: goldenNoticeFloodInert},
		{name: "blacklist_mute", kind: auto, run: blacklist("mute", false, ext.ContinueGroups),
			method: "restrictChatMember", idKey: "user_id", wantID: user,
			deletes: 101, sends: 1, golden: goldenNoticeBlacklistMute},
		{name: "blacklist_kick", kind: auto, run: blacklist("kick", false, ext.ContinueGroups),
			method: "unbanChatMember", idKey: "user_id", wantID: user,
			deletes: 101, sends: 1, golden: goldenNoticeBlacklistKick},
		{name: "blacklist_ban", kind: auto, run: blacklist("ban", false, ext.ContinueGroups),
			method: "banChatMember", idKey: "user_id", wantID: user,
			deletes: 101, sends: 1, golden: goldenNoticeBlacklistBan},
		{name: "blacklist_ban_channel", kind: auto, run: blacklist("ban", true, ext.ContinueGroups),
			method: "banChatSenderChat", idKey: "sender_chat_id", wantID: channel,
			deletes: 101, sends: 1, golden: goldenNoticeBlacklistBanChannel},
		{name: "forward_block_mute", kind: auto, run: forwardBlock(forwardBlockMute),
			method: "restrictChatMember", idKey: "user_id", wantID: user,
			sends: 1, golden: goldenNoticeForwardBlockMute},
		{name: "forward_block_kick", kind: auto, run: forwardBlock(forwardBlockKick),
			method: "unbanChatMember", idKey: "user_id", wantID: user,
			sends: 1, golden: goldenNoticeForwardBlockKick},
		{name: "forward_block_ban", kind: auto, run: forwardBlock(forwardBlockBan),
			method: "banChatMember", idKey: "user_id", wantID: user,
			sends: 1, golden: goldenNoticeForwardBlockBan},
	}
}

func TestLegacyPunishmentNoticeRequests(t *testing.T) {
	withModerationGoldenLocales(t)
	for _, tc := range noticeGoldenCases() {
		t.Run(tc.name, func(t *testing.T) {
			s := newNoticeGoldenSite(t, tc.kind)

			tc.run(t, s)

			for _, method := range noticeGoldenPunishments {
				calls := s.client.callsFor(method)
				want := 0
				if method == tc.method {
					want = 1
				}
				if len(calls) != want {
					t.Fatalf("%s calls = %d, want %d", method, len(calls), want)
				}
				if want == 1 {
					if got := fmt.Sprint(calls[0].Params[tc.idKey]); got != strconv.FormatInt(tc.wantID, 10) {
						t.Fatalf("%s %s = %s, want %d", method, tc.idKey, got, tc.wantID)
					}
				}
			}
			if tc.deletes != 0 {
				deleted := false
				for _, call := range s.client.callsFor("deleteMessage") {
					if fmt.Sprint(call.Params["message_id"]) == strconv.FormatInt(tc.deletes, 10) {
						deleted = true
					}
				}
				if !deleted {
					t.Fatalf("deleteMessage of message %d not sent", tc.deletes)
				}
			}
			if tc.checkWarns {
				if got, _ := warns.GetWarns(noticeGoldenTarget.Id, s.chat.Id); got != tc.warnsLeft {
					t.Fatalf("stored warns = %d, want %d", got, tc.warnsLeft)
				}
			}

			sends := s.client.callsFor("sendMessage")
			if len(sends) != tc.sends {
				t.Fatalf("sendMessage calls = %d, want %d", len(sends), tc.sends)
			}
			got := strings.ReplaceAll(legacyRequestJSON(t, sends[len(sends)-1]), strconv.FormatInt(s.chat.Id, 10), "<chat>")
			if got != tc.golden {
				t.Fatalf("sendMessage request changed\n got %q\nwant %q", got, tc.golden)
			}
			s.wantRow(s.chat.Id, 9001, tc.kind, 600*time.Second)
		})
	}
}
