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

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/blacklists"
	dbcache "github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/deletion"
	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/db/logchannels"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
)

// The safeguards of the Automatic family's sends (Phase 23 plan 23-06): no log line of the five sites carries content
// (D-32), and the forward blocklist never names the blocked source or the Staff Group in any message, record or log line
// (D-31).

// The sentinels of the log sweep: a stored text (which is also the caption of the stored photo), a file ID, a button's name
// and link, a typed reason and a matched blacklist word. They carry no digit, so a search for one is unambiguous.
const (
	eventMsgAutoSecretText   = "ZZAUTOTEXT {first}"
	eventMsgAutoSecretFile   = "ZZAUTOFILE"
	eventMsgAutoSecretButton = "ZZAUTOBUTTON"
	eventMsgAutoSecretLink   = "https://zzauto.example/link"
	eventMsgAutoSecretReason = "zzautoreason"
	eventMsgAutoSecretWord   = "zzautoword"
)

// eventMsgAutoSafetyBlacklist lists word with the mute action and sends the target's message 101 holding it through the
// blacklist watcher.
func eventMsgAutoSafetyBlacklist(t *testing.T, s *botSite, word string) {
	t.Helper()
	t.Cleanup(func() {
		if err := blacklists.RemoveAllBlacklist(s.chat.Id); err != nil {
			t.Errorf("RemoveAllBlacklist error = %v", err)
		}
	})
	if err := blacklists.AddBlacklist(s.chat.Id, word); err != nil {
		t.Fatalf("AddBlacklist error = %v", err)
	}
	if err := blacklists.SetBlacklistAction(s.chat.Id, "mute"); err != nil {
		t.Fatalf("SetBlacklistAction error = %v", err)
	}
	if err := blacklistsModule.blacklistWatcher(s.bot, noticeGoldenMessage(s, 101, "this has "+word+" inside", false)); err != ext.ContinueGroups {
		t.Fatalf("blacklistWatcher error = %v, want ContinueGroups", err)
	}
}

func TestEventMessageAutoLogsCarryIDsOnly(t *testing.T) {
	withModerationGoldenLocales(t)
	tr := i18n.MustNewTranslator("uk")
	forbidden := []string{
		"ZZAUTOTEXT", eventMsgAutoSecretFile, eventMsgAutoSecretButton, "zzauto.example", eventMsgAutoSecretReason,
		eventMsgAutoSecretWord, noticeGoldenTarget.FirstName, html.EscapeString(noticeGoldenTarget.FirstName), "<Bo>",
		noticeGoldenTarget.Username, floodEventReason(tr), forwardBlockReason(tr),
	}
	sites := []struct {
		name  string
		event string
		kind  string
		run   func(t *testing.T, s *botSite)
	}{
		{"warn", eventmsg.EventWarn, deletion.KindBotManual, func(t *testing.T, s *botSite) {
			noticeGoldenWarn(t, s, warnsModule.warnUser, "/warn "+eventMsgAutoSecretReason)
		}},
		{"warn limit", eventmsg.EventWarnLimit, deletion.KindBotManual, func(t *testing.T, s *botSite) {
			eventMsgAutoWarnSettings(t, s, 1, "ban")
			noticeGoldenWarn(t, s, warnsModule.warnUser, "/warn "+eventMsgAutoSecretReason)
		}},
		{"flood", eventmsg.EventFlood, deletion.KindBotAuto, func(t *testing.T, s *botSite) {
			noticeGoldenFlood(t, s, "mute", false)
		}},
		{"blacklist", eventmsg.EventBlacklist, deletion.KindBotAuto, func(t *testing.T, s *botSite) {
			eventMsgAutoSafetyBlacklist(t, s, eventMsgAutoSecretWord)
		}},
		{"forward block", eventmsg.EventForwardBlock, deletion.KindBotAuto, func(t *testing.T, s *botSite) {
			noticeGoldenForwardBlock(t, s, forwardBlockMute)
		}},
	}
	for _, site := range sites {
		store := func(t *testing.T, s *botSite) {
			t.Helper()
			eventMsgStoreRow(t, s.chat.Id, site.event, eventMsgAutoSecretText, eventMsgAutoSecretFile, db.PHOTO,
				[]models.Button{{Name: eventMsgAutoSecretButton, Url: eventMsgAutoSecretLink}})
		}

		t.Run(site.name+" read error", func(t *testing.T) {
			hook := mreqLogCapture(t)
			s := newNoticeGoldenSite(t, site.kind)
			store(t, s)
			mreqSwap(t, &eventMessagesLookup, func(context.Context, int64) (*eventmsg.Messages, error) {
				return nil, errors.New("database is down")
			})
			site.run(t, s)
			if sends := s.client.callsFor("sendMessage"); len(sends) != 1 {
				t.Fatalf("sendMessage calls = %d, want the one default", len(sends))
			}
			mreqWantCleanLogs(t, hook, forbidden,
				fmt.Sprintf("messages of chat %d could not be read for %s", s.chat.Id, site.event))
		})

		t.Run(site.name+" refused", func(t *testing.T) {
			hook := mreqLogCapture(t)
			s := newNoticeGoldenSite(t, site.kind)
			store(t, s)
			eventMsgRefuseOnce(s, "sendPhoto", &gotgbot.TelegramError{Method: "sendPhoto", Code: 400, Description: "Bad Request: wrong file identifier"})
			site.run(t, s)
			if photos, sends := s.client.callsFor("sendPhoto"), s.client.callsFor("sendMessage"); len(photos) != 1 || len(sends) != 1 {
				t.Fatalf("sendPhoto calls = %d and sendMessage calls = %d, want the refused custom message and the default", len(photos), len(sends))
			}
			mreqWantCleanLogs(t, hook, forbidden,
				fmt.Sprintf("custom %s message of chat %d was refused", site.event, s.chat.Id))
		})
	}
}

// The sentinels of the forward blocklist's privacy run: the Staff Group's title and the blocked source's title as the
// forward carries it. They carry no digit; the two chat IDs are searched by their digits.
const (
	eventMsgAutoStaffTitle  = "ZZ Fwd Staff & <Sentinel>"
	eventMsgAutoSourceTitle = "ZZ Fwd Source & <Sentinel>"
)

// eventMsgAutoPrivacyCalls is every send and edit the forward blocklist environment's bot made, as JSON: the requests that
// carry text or a keyboard to a chat (reads such as getChatMember of the Staff Group's owner are not messages).
func eventMsgAutoPrivacyCalls(t *testing.T, env *fwdWatchEnv) []string {
	t.Helper()
	client := env.fake.moduleBotClient
	client.mu.Lock()
	defer client.mu.Unlock()
	var out []string
	for _, call := range client.calls {
		if !strings.HasPrefix(call.Method, "send") && !strings.HasPrefix(call.Method, "edit") {
			continue
		}
		raw, err := json.Marshal(call.Params)
		if err != nil {
			t.Fatalf("encode a %s request: %v", call.Method, err)
		}
		out = append(out, call.Method+" "+string(raw))
	}
	return out
}

// TestEventMessageAutoPrivacy forwards a listed source into a linked group through the real watcher, with both private copy
// options on and the member refusing private messages, once with a custom group message and a custom private copy that use
// every placeholder and once with nothing stored: the member is punished once, the group message, the Details edit and
// record and the #PM_COPY line all exist, and none of them, nor any log line, names the source or the Staff Group.
func TestEventMessageAutoPrivacy(t *testing.T) {
	for _, custom := range []bool{true, false} {
		name := "defaults"
		if custom {
			name = "custom"
		}
		t.Run(name, func(t *testing.T) {
			env := newFwdWatchEnv(t, "mute")
			withFastPMCopyPacer(t)
			eventPMCleanup(t, env.group)
			t.Cleanup(pmCopyJobs.wait)
			hook := mreqLogCapture(t)

			if err := db.DB.Model(&models.StaffGroup{}).Where("chat_id = ?", env.staffChat).Update("title", eventMsgAutoStaffTitle).Error; err != nil {
				t.Fatalf("rename the Staff Group: %v", err)
			}
			dbcache.ResetLocalForTest()
			channel := uniqueModuleChatID()
			if err := logchannels.Set(env.group, env.groupChat.Title, channel); err != nil {
				t.Fatalf("link the log channel: %v", err)
			}
			t.Cleanup(func() { db.DB.Where("chat_id = ?", env.group).Delete(&models.LogChannel{}) })
			if custom {
				eventMsgStore(t, env.group, eventmsg.EventForwardBlock, eventMsgAutoEveryPlaceholder)
				storeEventPM(t, env.group, eventmsg.EventForwardBlock, eventMsgAutoEveryPlaceholder, true)
			} else {
				storeEventPM(t, env.group, eventmsg.EventForwardBlock, "", true)
			}
			eventPMDetailsOn(t, env.group)
			eventPMLogOn(t, env.group)

			member := env.member()
			env.fake.script("sendMessage", member.Id, staffFakeError(403, "Forbidden: bot was blocked by the user"))
			origin := gotgbot.MessageOriginChannel{Date: 1, Chat: gotgbot.Chat{Id: env.source, Type: "channel", Title: eventMsgAutoSourceTitle}, MessageId: 5}
			env.process(env.forwardUpdate(env.groupChat, &member, origin))
			waitEventPM(t)

			if calls := env.callsTo("restrictChatMember", env.group); len(calls) != 1 {
				t.Fatalf("restrictChatMember calls = %d, want the one mute", len(calls))
			}
			if sent := env.fake.sentTo(env.group); len(sent) != 1 {
				t.Fatalf("messages sent to the linked group = %d, want the one notice", len(sent))
			}
			if edits := env.callsTo("editMessageReplyMarkup", env.group); len(edits) != 1 {
				t.Fatalf("Details edits = %d, want one", len(edits))
			}
			var posts int
			for _, sent := range env.fake.sentTo(channel) {
				if strings.Contains(fmt.Sprint(sent.Params["text"]), pmCopyLogTag) {
					posts++
				}
			}
			if posts != 1 {
				t.Fatalf("#PM_COPY posts = %d, want one", posts)
			}
			if attempts := env.callsTo("sendMessage", member.Id); len(attempts) != 1 {
				t.Fatalf("sends to the member's private chat = %d, want the one refused copy", len(attempts))
			}

			leaks := []string{
				eventMsgAutoStaffTitle, html.EscapeString(eventMsgAutoStaffTitle), "ZZ Fwd Staff", "Staff HQ",
				eventMsgAutoSourceTitle, html.EscapeString(eventMsgAutoSourceTitle), "ZZ Fwd Source",
				fwdSourceTitle, html.EscapeString(fwdSourceTitle),
				strings.TrimPrefix(strconv.FormatInt(env.source, 10), "-"),
				strings.TrimPrefix(strconv.FormatInt(env.staffChat, 10), "-"),
			}
			wantClean := func(label, text string) {
				t.Helper()
				for _, leak := range leaks {
					if strings.Contains(text, leak) {
						t.Errorf("%s names the source or the Staff Group (%q): %s", label, leak, text)
					}
				}
			}
			for _, request := range eventMsgAutoPrivacyCalls(t, env) {
				wantClean("a request", request)
			}
			tokens := eventPMDetailsTokens(t)
			if len(tokens) != 1 {
				t.Fatalf("Details records = %d, want one", len(tokens))
			}
			rec, _ := eventPMDetailsRecord(t, tokens[0])
			raw, err := json.Marshal(rec)
			if err != nil {
				t.Fatalf("encode the Details record: %v", err)
			}
			if rec.UserID != member.Id || rec.Text == "" {
				t.Fatalf("the Details record %s is not the member's copy", raw)
			}
			wantClean("the Details record", string(raw))
			for _, line := range mreqLogLines(hook) {
				wantClean("a log line", line)
			}
		})
	}
}
