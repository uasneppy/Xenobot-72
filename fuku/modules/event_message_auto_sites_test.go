//go:build testtools

package modules

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/uasneppy/fuku_robot/fuku/db/deletion"
	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/db/warns"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
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
