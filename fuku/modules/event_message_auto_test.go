//go:build testtools

package modules

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/uasneppy/fuku_robot/fuku/db/deletion"
	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/db/warns"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
)

// eventMsgWarnText is the custom warn text the warn tests store: a Welcome placeholder in markup and
// every event placeholder a warn fills.
const eventMsgWarnText = "<i>{mention}</i> {count}/{limit}: {reason} ({admin}, {action})"

// eventMsgWarnRemoveRow is the Remove warn row of the golden warn as decoded JSON, in the group's
// language (uk).
func eventMsgWarnRemoveRow(t *testing.T) string {
	t.Helper()
	text, _ := i18n.MustNewTranslator("uk").GetString("warns_remove_button")
	raw, err := json.Marshal([]map[string]string{{"callback_data": "rmWarn|v1|u=42", "text": text}})
	if err != nil {
		t.Fatalf("encode the Remove warn row: %v", err)
	}
	return string(raw)
}

// eventMsgWarnCustom checks that the site sent exactly one sendMessage, the custom warn with text
// want, the custom button row then the Remove warn row last, as a reply to message 101, and that it is
// queued under kind.
func eventMsgWarnCustom(t *testing.T, s *botSite, want, kind string) {
	t.Helper()
	sends := s.client.callsFor("sendMessage")
	if len(sends) != 1 {
		t.Fatalf("sendMessage calls = %d, want the one custom warn", len(sends))
	}
	params := rulesCallParams(t, sends[0])
	if params["text"] != want {
		t.Fatalf("text = %q, want %q", params["text"], want)
	}
	rows := eventMsgRows(params)
	if len(rows) != 2 || rows[0] != eventMsgSiteRow || rows[1] != eventMsgWarnRemoveRow(t) {
		t.Fatalf("keyboard rows = %v, want the custom button then the Remove warn row last", rows)
	}
	reply, _ := json.Marshal(params["reply_parameters"])
	if string(reply) != `{"allow_sending_without_reply":true,"message_id":101}` {
		t.Fatalf("reply_parameters = %s, want message 101 with allow_sending_without_reply", reply)
	}
	s.wantRow(s.chat.Id, 9001, kind, 600*time.Second)
}

// eventMsgStoreWarn stores the custom warn text with one URL button for the site's chat.
func eventMsgStoreWarn(t *testing.T, s *botSite) {
	t.Helper()
	eventMsgStoreRow(t, s.chat.Id, eventmsg.EventWarn, eventMsgWarnText, "", 0, []models.Button{eventMsgSiteButton})
}

func TestEventMessageWarn(t *testing.T) {
	withModerationGoldenLocales(t)
	tr := i18n.MustNewTranslator("uk")
	anon, _ := tr.GetString("eventmsg_admin_anon")
	warned, _ := tr.GetString("eventmsg_action_warn")

	t.Run("custom warn keeps the Remove warn row last", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotManual)
		eventMsgStoreWarn(t, s)
		noticeGoldenWarn(t, s, warnsModule.warnUser, "/warn spam & <b>")
		eventMsgWarnCustom(t, s, "<i>@ann_lee</i> 1/3: spam &amp; &lt;b&gt; (Telegram, "+warned+")", deletion.KindBotManual)
	})

	t.Run("blacklist warn reads an admin", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		eventMsgStoreWarn(t, s)
		noticeGoldenBlacklist(t, s, "warn", false, ext.EndGroups)
		eventMsgWarnCustom(t, s, "<i>@ann_lee</i> 1/3: Blacklisted word: &#39;spam&#39; ("+anon+", "+warned+")", deletion.KindBotAuto)
	})

	t.Run("silent stores the warn and posts nothing", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotManual)
		eventMsgStoreWarn(t, s)
		if err := eventmsg.SetSilent(t.Context(), s.chat.Id, eventmsg.EventWarn, eventmsg.DestGroup, true, 1); err != nil {
			t.Fatalf("SetSilent error = %v", err)
		}
		noticeGoldenWarn(t, s, warnsModule.warnUser, "/warn spam & <b>")
		if got, _ := warns.GetWarns(noticeGoldenTarget.Id, s.chat.Id); got != 1 {
			t.Fatalf("stored warns = %d, want 1", got)
		}
		if got := len(s.client.callsFor("sendMessage")); got != 0 {
			t.Fatalf("sendMessage calls = %d, want none when silent", got)
		}
		s.wantNoRows(s.chat.Id)
	})

	t.Run("a refused custom warn sends the default once", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotManual)
		eventMsgStoreWarn(t, s)
		eventMsgRefuseOnce(s, "sendMessage", &gotgbot.TelegramError{Method: "sendMessage", Code: 400, Description: "Bad Request: can't parse entities"})
		noticeGoldenWarn(t, s, warnsModule.warnUser, "/warn spam & <b>")
		sends := s.client.callsFor("sendMessage")
		if len(sends) != 2 {
			t.Fatalf("sendMessage calls = %d, want the refused custom warn and the default", len(sends))
		}
		got := strings.ReplaceAll(legacyRequestJSON(t, sends[1]), strconv.FormatInt(s.chat.Id, 10), "<chat>")
		if got != goldenNoticeWarnReason {
			t.Fatalf("fallback warn\n got %q\nwant %q", got, goldenNoticeWarnReason)
		}
		s.wantRow(s.chat.Id, 9001, deletion.KindBotManual, 600*time.Second)
	})
}

func TestEventActionWords(t *testing.T) {
	withRealLocales(t)
	actions := map[string]string{
		"warn":   "eventmsg_action_warn",
		"mute":   "eventmsg_action_mute",
		"kick":   "eventmsg_action_kick",
		"ban":    "eventmsg_action_ban",
		"delete": "eventmsg_action_delete",
	}
	reasons := map[string]func(*i18n.Translator) string{
		"eventmsg_reason_flood":   floodEventReason,
		"eventmsg_reason_captcha": captchaEventReason,
		"eventmsg_reason_aispam":  aispamEventReason,
	}
	for _, lang := range []string{"uk", "en", "es", "fr", "hi", "id", "pt", "ru"} {
		t.Run(lang, func(t *testing.T) {
			tr := i18n.MustNewTranslator(lang)
			words := map[string]bool{}
			for action, key := range actions {
				want, _ := tr.GetString(key)
				got := eventActionWord(tr, action)
				if got != want || strings.TrimSpace(got) == "" || strings.ContainsAny(got, "<>&{}%") {
					t.Errorf("eventActionWord(%s) = %q, want the non-empty plain %s %q", action, got, key, want)
				}
				words[got] = true
			}
			if len(words) != len(actions) {
				t.Errorf("action words = %v, want %d different words", words, len(actions))
			}
			// A timed mute or ban reads the mute or ban word (Phase 26, D-16); its time is in {duration}.
			for timed, plain := range map[string]string{"tmute": "mute", "tban": "ban"} {
				if got, want := eventActionWord(tr, timed), eventActionWord(tr, plain); got != want || got == "" {
					t.Errorf("eventActionWord(%q) = %q, want the %s word %q", timed, got, plain, want)
				}
			}
			for _, inert := range []string{"", "none", "Mute", "Delete", "deleted", "Tmute"} {
				if got := eventActionWord(tr, inert); got != "" {
					t.Errorf("eventActionWord(%q) = %q, want no word", inert, got)
				}
			}
			for key, reason := range reasons {
				want, _ := tr.GetString(key)
				if got := reason(tr); got != want || strings.TrimSpace(got) == "" || strings.ContainsAny(got, "<>&{}%") {
					t.Errorf("the reason of %s = %q, want the non-empty plain %q", key, got, want)
				}
			}
		})
	}
}

// autoDefaultRequest sends a default reply's text and options to chat 0 through a recording client and
// returns the request as Telegram receives it, the form the goldens record.
func autoDefaultRequest(t *testing.T, text string, opts *gotgbot.SendMessageOpts) string {
	t.Helper()
	client := newModuleBotClient()
	if _, err := newModuleTestBot(client).SendMessage(0, text, opts); err != nil {
		t.Fatalf("SendMessage error = %v", err)
	}
	sends := client.callsFor("sendMessage")
	if len(sends) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(sends))
	}
	return legacyRequestJSON(t, sends[0])
}

func TestAutoEventDefaults(t *testing.T) {
	withRealLocales(t)
	tr := i18n.MustNewTranslator("uk")
	target := noticeGoldenTarget
	channel := gotgbot.User{Id: noticeGoldenChannel.Id, FirstName: noticeGoldenChannel.Title}
	removeText, _ := tr.GetString("warns_remove_button")
	rulesText, _ := tr.GetString("common_rules_button_emoji")
	removeWarn := gotgbot.InlineKeyboardButton{Text: removeText, CallbackData: encodeCallbackData("rmWarn", map[string]string{"u": "42"})}
	rulesButton := gotgbot.InlineKeyboardButton{Text: rulesText, Url: "t.me/FukuTestBot?start=rules_0"}
	unmute := [][]gotgbot.InlineKeyboardButton{{{
		Text:         trS(tr, "button_unmute_admins"),
		CallbackData: encodeCallbackData("unrestrict", map[string]string{"a": "unmute", "u": "42"}),
	}}}
	unban := [][]gotgbot.InlineKeyboardButton{{{
		Text:         trS(tr, "antiflood_button_unban_admins"),
		CallbackData: encodeCallbackData("unrestrict", map[string]string{"a": "unban", "u": strconv.FormatInt(noticeGoldenChannel.Id, 10)}),
	}}}
	listed, _ := tr.GetString("staff_fwdblock_reason")
	const blacklisted = "Blacklisted word: 'spam'"

	cases := []struct {
		name    string
		build   eventDefaultReply
		in      eventDefaultInput
		replyTo bool // the warn and the warn limit sites add the reply to the triggering message
		golden  string
	}{
		{"warn_reason", warnDefaultReply, eventDefaultInput{Target: target, Reason: "spam & <b>", Count: 1, Limit: 3,
			Rows: [][]gotgbot.InlineKeyboardButton{{removeWarn}}}, true, goldenNoticeWarnReason},
		{"warn_rules", warnDefaultReply, eventDefaultInput{Target: target, Count: 1, Limit: 3,
			Rows: [][]gotgbot.InlineKeyboardButton{{removeWarn, rulesButton}}}, true, goldenNoticeWarnRules},
		{"warnlimit_mute", warnLimitDefaultReply, eventDefaultInput{Target: target, Count: 2, Limit: 2, Action: "mute",
			Reasons: []string{"first", "second"}}, true, goldenNoticeWarnLimitMute},
		{"warnlimit_kick", warnLimitDefaultReply, eventDefaultInput{Target: target, Count: 1, Limit: 1, Action: "kick",
			Reasons: []string{"reason"}}, true, goldenNoticeWarnLimitKick},
		{"warnlimit_ban", warnLimitDefaultReply, eventDefaultInput{Target: target, Count: 1, Limit: 1, Action: "ban",
			Reasons: []string{"reason"}}, true, goldenNoticeWarnLimitBan},
		{"warnlimit_inert", warnLimitDefaultReply, eventDefaultInput{Target: target, Count: 1, Limit: 1, Action: "tban",
			Reasons: []string{"reason"}}, true, goldenNoticeWarnLimitInert},
		{"flood_mute", floodDefaultReply, eventDefaultInput{Target: target, Action: "mute", Rows: unmute}, false, goldenNoticeFloodMute},
		{"flood_kick", floodDefaultReply, eventDefaultInput{Target: target, Action: "kick"}, false, goldenNoticeFloodKick},
		{"flood_ban", floodDefaultReply, eventDefaultInput{Target: target, Action: "ban"}, false, goldenNoticeFloodBan},
		{"flood_ban_channel", floodDefaultReply, eventDefaultInput{Target: channel, Action: "ban", Rows: unban}, false, goldenNoticeFloodBanChannel},
		{"flood_inert", floodDefaultReply, eventDefaultInput{Target: target, Action: "warn"}, false, goldenNoticeFloodInert},
		{"blacklist_mute", blacklistNoticeDefaultReply, eventDefaultInput{Target: target, Action: "mute", Reason: blacklisted}, false, goldenNoticeBlacklistMute},
		{"blacklist_kick", blacklistNoticeDefaultReply, eventDefaultInput{Target: target, Action: "kick", Reason: blacklisted}, false, goldenNoticeBlacklistKick},
		{"blacklist_ban", blacklistNoticeDefaultReply, eventDefaultInput{Target: target, Action: "ban", Reason: blacklisted}, false, goldenNoticeBlacklistBan},
		{"blacklist_ban_channel", blacklistNoticeDefaultReply, eventDefaultInput{Target: channel, Action: "ban", Reason: blacklisted}, false, goldenNoticeBlacklistBanChannel},
		{"forward_block_mute", blacklistNoticeDefaultReply, eventDefaultInput{Target: target, Action: "mute", Reason: listed}, false, goldenNoticeForwardBlockMute},
		{"forward_block_kick", blacklistNoticeDefaultReply, eventDefaultInput{Target: target, Action: "kick", Reason: listed}, false, goldenNoticeForwardBlockKick},
		{"forward_block_ban", blacklistNoticeDefaultReply, eventDefaultInput{Target: target, Action: "ban", Reason: listed}, false, goldenNoticeForwardBlockBan},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text, opts := tc.build(tr, tc.in)
			if tc.replyTo {
				opts.ReplyParameters = &gotgbot.ReplyParameters{MessageId: 101, AllowSendingWithoutReply: true}
			}
			want := strings.ReplaceAll(tc.golden, "<chat>", "0")
			if got := autoDefaultRequest(t, text, opts); got != want {
				t.Fatalf("default request\n got %q\nwant %q", got, want)
			}
		})
	}
}
