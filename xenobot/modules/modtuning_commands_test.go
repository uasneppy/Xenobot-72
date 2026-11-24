//go:build testtools

package modules

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/db/eventmsg"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/db/modtuning"
	"github.com/uasneppy/Xenobot-72/xenobot/db/warns"
)

func cleanupModTuningChat(t *testing.T, chatID int64) {
	t.Helper()
	t.Cleanup(func() {
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatManualDefaults{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatReasonShortcut{})
		cache.DeleteCache(modtuning.DefaultsCacheKey(chatID))
		cache.DeleteCache(modtuning.ShortcutsCacheKey(chatID))
		cache.ResetLocalForTest()
	})
}

func modTuningUntilDate(t *testing.T, call moduleBotCall) int64 {
	t.Helper()
	raw, ok := call.Params["until_date"]
	if !ok || raw == nil {
		return 0
	}
	switch v := raw.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	case string:
		if v == "" {
			return 0
		}
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			t.Fatalf("parse until_date %q: %v", v, err)
		}
		return n
	default:
		t.Fatalf("unexpected until_date type %T (%#v)", raw, raw)
		return 0
	}
}

func withFrozenPunishNow(t *testing.T, fixed time.Time) {
	t.Helper()
	prev := punishNow
	punishNow = func() time.Time { return fixed }
	t.Cleanup(func() { punishNow = prev })
}

func TestManualCommandDefaults(t *testing.T) {
	withModerationGoldenLocales(t)
	ctx := context.Background()
	fixedNow := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)

	t.Run("default_ban_duration", func(t *testing.T) {
		withFrozenPunishNow(t, fixedNow)
		wantUntil := fixedNow.Unix() + 3600

		for _, tc := range []struct {
			name    string
			text    string
			reply   bool
			call    func(b *gotgbot.Bot, ctx *ext.Context) error
			silent  bool
			deletes int64
		}{
			{name: "ban", text: "/ban spam", reply: true, call: bansModule.ban},
			{name: "dban", text: "/dban spam", reply: true, call: bansModule.dBan, deletes: 303},
			{name: "sban", text: "/sban 42", reply: false, call: bansModule.sBan, silent: true, deletes: 101},
		} {
			t.Run(tc.name, func(t *testing.T) {
				s := newModerationGoldenSite(t)
				cleanupModTuningChat(t, s.chat.Id)
				if err := modtuning.SetDefaultBanSecs(ctx, s.chat.Id, 3600, 1); err != nil {
					t.Fatal(err)
				}

				if err := tc.call(s.bot, s.commandCtx(tc.text, tc.reply, false)); err != ext.EndGroups {
					t.Fatalf("%s error = %v, want EndGroups", tc.text, err)
				}

				bans := s.client.callsFor("banChatMember")
				if len(bans) != 1 {
					t.Fatalf("banChatMember calls = %d, want 1", len(bans))
				}
				if got := modTuningUntilDate(t, bans[0]); got != wantUntil {
					t.Fatalf("banChatMember until_date = %d, want %d", got, wantUntil)
				}

				if tc.deletes != 0 {
					deleted := false
					for _, d := range s.client.callsFor("deleteMessage") {
						if fmt.Sprint(d.Params["message_id"]) == strconv.FormatInt(tc.deletes, 10) {
							deleted = true
						}
					}
					if !deleted {
						t.Fatalf("expected deleteMessage for %d", tc.deletes)
					}
				}

				sends := s.client.callsFor("sendMessage")
				if tc.silent {
					if len(sends) != 0 {
						t.Fatalf("silent command sent %d messages, want 0", len(sends))
					}
					return
				}
				if len(sends) != 1 {
					t.Fatalf("sendMessage calls = %d, want 1", len(sends))
				}
				// Default timed ban reply includes the localized duration ("1 год" in uk).
				gotText := fmt.Sprint(sends[0].Params["text"])
				if !strings.Contains(gotText, "Тимчасове видалення") || !strings.Contains(gotText, "1 год") {
					t.Fatalf("timed ban default reply = %q, want timed ban template with 1 год", gotText)
				}
			})
		}

		t.Run("custom_timed_ban_message_shows_duration_and_until", func(t *testing.T) {
			s := newModerationGoldenSite(t)
			cleanupModTuningChat(t, s.chat.Id)
			if err := modtuning.SetDefaultBanSecs(ctx, s.chat.Id, 3600, 1); err != nil {
				t.Fatal(err)
			}
			eventMsgStore(t, s.chat.Id, eventmsg.EventTimedBan, "TBAN dur={duration} until={until} reason={reason}")

			if err := bansModule.ban(s.bot, s.commandCtx("/ban spam", true, false)); err != ext.EndGroups {
				t.Fatalf("/ban error = %v, want EndGroups", err)
			}
			sends := s.client.callsFor("sendMessage")
			if len(sends) != 1 {
				t.Fatalf("sendMessage calls = %d, want 1", len(sends))
			}
			gotText := fmt.Sprint(sends[0].Params["text"])
			if !strings.Contains(gotText, "TBAN dur=1 год") || !strings.Contains(gotText, "22.11.2026") || !strings.Contains(gotText, "reason=spam") {
				t.Fatalf("custom EventTimedBan text = %q, want duration, until date, and reason", gotText)
			}
		})

		t.Run("channel_ban_stays_permanent_sender_chat_ban", func(t *testing.T) {
			s := newModerationGoldenSite(t)
			cleanupModTuningChat(t, s.chat.Id)
			if err := modtuning.SetDefaultBanSecs(ctx, s.chat.Id, 3600, 1); err != nil {
				t.Fatal(err)
			}

			if err := bansModule.ban(s.bot, s.commandCtx("/ban -1001234567890", true, true)); err != ext.EndGroups {
				t.Fatalf("channel /ban error = %v, want EndGroups", err)
			}
			if got := len(s.client.callsFor("banChatMember")); got != 0 {
				t.Fatalf("banChatMember calls = %d, want 0 for channel ban", got)
			}
			if got := len(s.client.callsFor("banChatSenderChat")); got != 1 {
				t.Fatalf("banChatSenderChat calls = %d, want 1", got)
			}
		})

		t.Run("tban_keeps_explicit_duration", func(t *testing.T) {
			s := newModerationGoldenSite(t)
			cleanupModTuningChat(t, s.chat.Id)
			if err := modtuning.SetDefaultBanSecs(ctx, s.chat.Id, 3600, 1); err != nil {
				t.Fatal(err)
			}

			before := time.Now().Unix()
			if err := bansModule.tBan(s.bot, s.commandCtx("/tban 2h flood", true, false)); err != ext.EndGroups {
				t.Fatalf("/tban error = %v, want EndGroups", err)
			}
			after := time.Now().Unix()

			bans := s.client.callsFor("banChatMember")
			if len(bans) != 1 {
				t.Fatalf("banChatMember calls = %d, want 1", len(bans))
			}
			until := modTuningUntilDate(t, bans[0])
			if until < before+7200-5 || until > after+7200+5 {
				t.Fatalf("/tban until_date = %d, want ~now+7200 (not default 3600)", until)
			}
		})
	})

	t.Run("default_mute_duration", func(t *testing.T) {
		withFrozenPunishNow(t, fixedNow)
		wantUntil := fixedNow.Unix() + 1800

		for _, tc := range []struct {
			name    string
			text    string
			reply   bool
			call    func(b *gotgbot.Bot, ctx *ext.Context) error
			silent  bool
			deletes int64
		}{
			{name: "mute", text: "/mute noise", reply: true, call: mutesModule.mute},
			{name: "dmute", text: "/dmute noise", reply: true, call: mutesModule.dMute, deletes: 303},
			{name: "smute", text: "/smute 42", reply: false, call: mutesModule.sMute, silent: true, deletes: 101},
		} {
			t.Run(tc.name, func(t *testing.T) {
				s := newModerationGoldenSite(t)
				cleanupModTuningChat(t, s.chat.Id)
				if err := modtuning.SetDefaultMuteSecs(ctx, s.chat.Id, 1800, 1); err != nil {
					t.Fatal(err)
				}

				if err := tc.call(s.bot, s.commandCtx(tc.text, tc.reply, false)); err != ext.EndGroups {
					t.Fatalf("%s error = %v, want EndGroups", tc.text, err)
				}

				mutes := s.client.callsFor("restrictChatMember")
				if len(mutes) != 1 {
					t.Fatalf("restrictChatMember calls = %d, want 1", len(mutes))
				}
				if got := modTuningUntilDate(t, mutes[0]); got != wantUntil {
					t.Fatalf("restrictChatMember until_date = %d, want %d", got, wantUntil)
				}

				if tc.deletes != 0 {
					deleted := false
					for _, d := range s.client.callsFor("deleteMessage") {
						if fmt.Sprint(d.Params["message_id"]) == strconv.FormatInt(tc.deletes, 10) {
							deleted = true
						}
					}
					if !deleted {
						t.Fatalf("expected deleteMessage for %d", tc.deletes)
					}
				}

				sends := s.client.callsFor("sendMessage")
				if tc.silent {
					if len(sends) != 0 {
						t.Fatalf("silent command sent %d messages, want 0", len(sends))
					}
					return
				}
				if len(sends) != 1 {
					t.Fatalf("sendMessage calls = %d, want 1", len(sends))
				}
				gotText := fmt.Sprint(sends[0].Params["text"])
				if !strings.Contains(gotText, "30 хв") {
					t.Fatalf("timed mute default reply = %q, want 30 хв", gotText)
				}
			})
		}

		t.Run("custom_timed_mute_message_shows_duration_and_until", func(t *testing.T) {
			s := newModerationGoldenSite(t)
			cleanupModTuningChat(t, s.chat.Id)
			if err := modtuning.SetDefaultMuteSecs(ctx, s.chat.Id, 1800, 1); err != nil {
				t.Fatal(err)
			}
			eventMsgStore(t, s.chat.Id, eventmsg.EventTimedMute, "TMUTE dur={duration} until={until} reason={reason}")

			if err := mutesModule.mute(s.bot, s.commandCtx("/mute noise", true, false)); err != ext.EndGroups {
				t.Fatalf("/mute error = %v, want EndGroups", err)
			}
			sends := s.client.callsFor("sendMessage")
			if len(sends) != 1 {
				t.Fatalf("sendMessage calls = %d, want 1", len(sends))
			}
			gotText := fmt.Sprint(sends[0].Params["text"])
			if !strings.Contains(gotText, "TMUTE dur=30 хв") || !strings.Contains(gotText, "22.11.2026") || !strings.Contains(gotText, "reason=noise") {
				t.Fatalf("custom EventTimedMute text = %q, want duration, until date, and reason", gotText)
			}
		})

		t.Run("tmute_keeps_explicit_duration", func(t *testing.T) {
			s := newModerationGoldenSite(t)
			cleanupModTuningChat(t, s.chat.Id)
			if err := modtuning.SetDefaultMuteSecs(ctx, s.chat.Id, 1800, 1); err != nil {
				t.Fatal(err)
			}

			before := time.Now().Unix()
			if err := mutesModule.tMute(s.bot, s.commandCtx("/tmute 2h noise", true, false)); err != ext.EndGroups {
				t.Fatalf("/tmute error = %v, want EndGroups", err)
			}
			after := time.Now().Unix()

			mutes := s.client.callsFor("restrictChatMember")
			if len(mutes) != 1 {
				t.Fatalf("restrictChatMember calls = %d, want 1", len(mutes))
			}
			until := modTuningUntilDate(t, mutes[0])
			if until < before+7200-5 || until > after+7200+5 {
				t.Fatalf("/tmute until_date = %d, want ~now+7200 (not default 1800)", until)
			}
		})
	})

	t.Run("reason_required_refuses_all_manual_punishments_without_reason", func(t *testing.T) {
		const wantRefusal = "Вкажіть причину для цього покарання."
		cases := []struct {
			name  string
			text  string
			reply bool
			call  func(b *gotgbot.Bot, ctx *ext.Context) error
		}{
			{"ban", "/ban", true, bansModule.ban},
			{"dban", "/dban", true, bansModule.dBan},
			{"sban", "/sban 42", false, bansModule.sBan},
			{"tban_no_reason", "/tban 2h", true, bansModule.tBan},
			{"kick", "/kick", true, bansModule.kick},
			{"dkick", "/dkick", true, bansModule.dkick},
			{"skick", "/skick 42", false, bansModule.sKick},
			{"mute", "/mute", true, mutesModule.mute},
			{"dmute", "/dmute", true, mutesModule.dMute},
			{"smute", "/smute 42", false, mutesModule.sMute},
			{"tmute_no_reason", "/tmute 30m", true, mutesModule.tMute},
			{"warn", "/warn", true, warnsModule.warnUser},
			{"dwarn", "/dwarn", true, warnsModule.dWarnUser},
			{"swarn", "/swarn", true, warnsModule.sWarnUser},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				s := newModerationGoldenSite(t)
				cleanupModTuningChat(t, s.chat.Id)
				if err := modtuning.SetReasonRequired(ctx, s.chat.Id, true, 1); err != nil {
					t.Fatal(err)
				}

				if err := tc.call(s.bot, s.commandCtx(tc.text, tc.reply, false)); err != ext.EndGroups {
					t.Fatalf("%s error = %v, want EndGroups", tc.text, err)
				}

				for _, method := range []string{"banChatMember", "restrictChatMember", "unbanChatMember", "deleteMessage"} {
					if got := len(s.client.callsFor(method)); got != 0 {
						t.Fatalf("%s made %d %s calls on refusal, want 0", tc.name, got, method)
					}
				}
				if count, _ := warns.GetWarns(moderationGoldenTarget.Id, s.chat.Id); count != 0 {
					t.Fatalf("%s recorded %d warns on refusal, want 0", tc.name, count)
				}
				s.wantNoRows(s.chat.Id)

				sends := s.client.callsFor("sendMessage")
				if len(sends) != 1 {
					t.Fatalf("%s sendMessage calls = %d, want 1 refusal", tc.name, len(sends))
				}
				if got := fmt.Sprint(sends[0].Params["text"]); got != wantRefusal {
					t.Fatalf("%s refusal text = %q, want %q", tc.name, got, wantRefusal)
				}
			})
		}
	})

	t.Run("reason_required_allows_all_manual_punishments_with_reason", func(t *testing.T) {
		cases := []struct {
			name   string
			text   string
			reply  bool
			call   func(b *gotgbot.Bot, ctx *ext.Context) error
			method string
			warn   bool
		}{
			{"ban", "/ban spam", true, bansModule.ban, "banChatMember", false},
			{"dban", "/dban spam", true, bansModule.dBan, "banChatMember", false},
			{"sban", "/sban 42 spam", false, bansModule.sBan, "banChatMember", false},
			{"tban", "/tban 2h spam", true, bansModule.tBan, "banChatMember", false},
			{"kick", "/kick spam", true, bansModule.kick, "unbanChatMember", false},
			{"dkick", "/dkick spam", true, bansModule.dkick, "unbanChatMember", false},
			{"skick", "/skick 42 spam", false, bansModule.sKick, "unbanChatMember", false},
			{"mute", "/mute noise", true, mutesModule.mute, "restrictChatMember", false},
			{"dmute", "/dmute noise", true, mutesModule.dMute, "restrictChatMember", false},
			{"smute", "/smute 42 noise", false, mutesModule.sMute, "restrictChatMember", false},
			{"tmute", "/tmute 30m noise", true, mutesModule.tMute, "restrictChatMember", false},
			{"warn", "/warn spam", true, warnsModule.warnUser, "", true},
			{"dwarn", "/dwarn spam", true, warnsModule.dWarnUser, "", true},
			{"swarn", "/swarn spam", true, warnsModule.sWarnUser, "", true},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				s := newModerationGoldenSite(t)
				cleanupModTuningChat(t, s.chat.Id)
				if err := modtuning.SetReasonRequired(ctx, s.chat.Id, true, 1); err != nil {
					t.Fatal(err)
				}

				if err := tc.call(s.bot, s.commandCtx(tc.text, tc.reply, false)); err != ext.EndGroups {
					t.Fatalf("%s error = %v, want EndGroups", tc.text, err)
				}
				if tc.method != "" {
					if got := len(s.client.callsFor(tc.method)); got != 1 {
						t.Fatalf("%s %s calls = %d, want 1", tc.name, tc.method, got)
					}
				}
				if tc.warn {
					if count, _ := warns.GetWarns(moderationGoldenTarget.Id, s.chat.Id); count != 1 {
						t.Fatalf("%s warn count = %d, want 1", tc.name, count)
					}
				}
			})
		}
	})

	t.Run("reason_required_exemptions", func(t *testing.T) {
		t.Run("unban_unmute_channel_ban_and_auto_warn", func(t *testing.T) {
			s := newModerationGoldenSite(t)
			cleanupModTuningChat(t, s.chat.Id)
			if err := modtuning.SetReasonRequired(ctx, s.chat.Id, true, 1); err != nil {
				t.Fatal(err)
			}

			if err := bansModule.unban(s.bot, s.commandCtx("/unban 42", false, false)); err != ext.EndGroups {
				t.Fatalf("/unban error = %v", err)
			}
			if got := len(s.client.callsFor("unbanChatMember")); got != 1 {
				t.Fatalf("/unban unbanChatMember calls = %d, want 1", got)
			}

			if err := mutesModule.unmute(s.bot, s.commandCtx("/unmute 42", false, false)); err != ext.EndGroups {
				t.Fatalf("/unmute error = %v", err)
			}
			if got := len(s.client.callsFor("restrictChatMember")); got != 1 {
				t.Fatalf("/unmute restrictChatMember calls = %d, want 1", got)
			}

			if err := bansModule.ban(s.bot, s.commandCtx("/ban -1001234567890", true, true)); err != ext.EndGroups {
				t.Fatalf("channel /ban error = %v", err)
			}
			if got := len(s.client.callsFor("banChatSenderChat")); got != 1 {
				t.Fatalf("channel /ban banChatSenderChat calls = %d, want 1", got)
			}

			// Automatic warn (warnSourceAuto) must never be blocked by reason_required.
			if err := warnsModule.warnThisUser(s.bot, s.commandCtx("/warn", true, false), moderationGoldenTarget.Id, "", "warn", warnSourceAuto); err != ext.EndGroups {
				t.Fatalf("warnSourceAuto error = %v", err)
			}
			if count, _ := warns.GetWarns(moderationGoldenTarget.Id, s.chat.Id); count != 1 {
				t.Fatalf("warnSourceAuto count = %d, want 1", count)
			}
		})

		t.Run("kickme_not_blocked", func(t *testing.T) {
			s := newModerationGoldenSite(t)
			cleanupModTuningChat(t, s.chat.Id)
			if err := modtuning.SetReasonRequired(ctx, s.chat.Id, true, 1); err != nil {
				t.Fatal(err)
			}
			cmdCtx := newModuleMessageContext(s.bot, s.chat, botSiteTarget, "/kickme")
			cmdCtx.EffectiveMessage.Date = s.now.Unix()

			if err := bansModule.kickme(s.bot, cmdCtx); err != ext.EndGroups {
				t.Fatalf("/kickme error = %v", err)
			}
			if got := len(s.client.callsFor("unbanChatMember")); got != 1 {
				t.Fatalf("/kickme unbanChatMember calls = %d, want 1", got)
			}
		})
	})

	t.Run("fail_open_on_db_error_logs_chat_id_only", func(t *testing.T) {
		s := newModerationGoldenSite(t)
		cleanupModTuningChat(t, s.chat.Id)

		prevLookup := manualDefaultsLookup
		manualDefaultsLookup = func(context.Context, int64) (modtuning.Defaults, error) {
			return modtuning.Defaults{}, errors.New("simulated db outage")
		}
		t.Cleanup(func() { manualDefaultsLookup = prevLookup })

		var logBuf bytes.Buffer
		prevOut := log.StandardLogger().Out
		log.SetOutput(&logBuf)
		t.Cleanup(func() { log.SetOutput(prevOut) })

		const secretText = "secret-user-text-never-logged"
		if err := bansModule.ban(s.bot, s.commandCtx("/ban "+secretText, true, false)); err != ext.EndGroups {
			t.Fatalf("/ban on db error = %v, want EndGroups", err)
		}
		bans := s.client.callsFor("banChatMember")
		if len(bans) != 1 {
			t.Fatalf("banChatMember calls = %d, want 1 (fail-open)", len(bans))
		}
		phase27AssertNoUntilDate(t, bans[0])

		if err := warnsModule.warnUser(s.bot, s.commandCtx("/warn", true, false)); err != ext.EndGroups {
			t.Fatalf("/warn on db error = %v, want EndGroups", err)
		}
		if count, _ := warns.GetWarns(moderationGoldenTarget.Id, s.chat.Id); count != 1 {
			t.Fatalf("warn count on db error = %d, want 1 (fail-open)", count)
		}

		logged := logBuf.String()
		if !strings.Contains(logged, strconv.FormatInt(s.chat.Id, 10)) {
			t.Errorf("expected Warn log mentioning chat %d, got %q", s.chat.Id, logged)
		}
		if strings.Contains(logged, secretText) {
			t.Errorf("log leaked user text %q: %s", secretText, logged)
		}
	})

	t.Run("staff_group_isolation", func(t *testing.T) {
		s := newModerationGoldenSite(t)
		cleanupModTuningChat(t, s.chat.Id)
		if err := modtuning.SetDefaultBanSecs(ctx, s.chat.Id, 3600, 1); err != nil {
			t.Fatal(err)
		}
		if err := modtuning.SetDefaultMuteSecs(ctx, s.chat.Id, 1800, 1); err != nil {
			t.Fatal(err)
		}
		if err := modtuning.SetReasonRequired(ctx, s.chat.Id, true, 1); err != nil {
			t.Fatal(err)
		}

		// Staff Group /ban and /mute without duration or reason parse as permanent with empty reason.
		banSpec := staffCommandSpec{Name: "ban", Kind: staffKindBan, Duration: staffDurationOptional}
		banReq, banRes := parseStaffActionArgs(&gotgbot.Message{Text: "/ban 4242"}, banSpec)
		if banRes != staffParseOK || banReq.DurationSec != 0 || banReq.Reason != "" || banReq.Target.UserID != 4242 {
			t.Fatalf("Staff /ban parse = (%+v, %d), want OK with DurationSec=0 Reason='' Target=4242", banReq, banRes)
		}
		muteSpec := staffCommandSpec{Name: "mute", Kind: staffKindMute, Duration: staffDurationOptional}
		muteReq, muteRes := parseStaffActionArgs(&gotgbot.Message{Text: "/mute 4242"}, muteSpec)
		if muteRes != staffParseOK || muteReq.DurationSec != 0 || muteReq.Reason != "" || muteReq.Target.UserID != 4242 {
			t.Fatalf("Staff /mute parse = (%+v, %d), want OK with DurationSec=0 Reason='' Target=4242", muteReq, muteRes)
		}

		// Structurally guarantee no staff*.go production file reads modtuning or manual defaults.
		staffFiles, err := filepath.Glob("staff*.go")
		if err != nil {
			t.Fatalf("glob staff*.go: %v", err)
		}
		if len(staffFiles) == 0 {
			t.Fatal("expected staff*.go files in xenobot/modules")
		}
		forbidden := []string{
			"xenobot/db/modtuning",
			"manualDefaultsFor",
			"manualDefaultUntil",
			"manualApplyReasonRequired",
			"manualBanMember",
			"manualMuteMember",
		}
		for _, path := range staffFiles {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			raw, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("read %s: %v", path, readErr)
			}
			src := string(raw)
			for _, sym := range forbidden {
				if strings.Contains(src, sym) {
					t.Errorf("Staff Group file %s must never reference %s (EVT-10)", path, sym)
				}
			}
		}
	})

	t.Run("apply_helpers_validation", func(t *testing.T) {
		s := newModerationGoldenSite(t)
		cleanupModTuningChat(t, s.chat.Id)

		for _, bad := range []int{-1, 59, models.PunishMaxSecs + 1} {
			if got := applyDefaultBanSecs(ctx, s.chat.Id, bad, 7); got != modTuningInvalid {
				t.Errorf("applyDefaultBanSecs(%d) = %v, want modTuningInvalid", bad, got)
			}
			if got := applyDefaultMuteSecs(ctx, s.chat.Id, bad, 7); got != modTuningInvalid {
				t.Errorf("applyDefaultMuteSecs(%d) = %v, want modTuningInvalid", bad, got)
			}
		}
		if got := applyDefaultBanSecs(ctx, s.chat.Id, 3600, 7); got != modTuningSaved {
			t.Errorf("applyDefaultBanSecs(3600) = %v, want modTuningSaved", got)
		}
		if got := applyDefaultMuteSecs(ctx, s.chat.Id, 0, 7); got != modTuningSaved {
			t.Errorf("applyDefaultMuteSecs(0) = %v, want modTuningSaved", got)
		}
		if got := applyReasonRequired(ctx, s.chat.Id, true, 7); got != modTuningSaved {
			t.Errorf("applyReasonRequired(true) = %v, want modTuningSaved", got)
		}

		prevWrites := modTuningWrites
		modTuningWrites.SetDefaultBanSecs = func(context.Context, int64, int, int64) error { return errors.New("boom") }
		modTuningWrites.SetDefaultMuteSecs = func(context.Context, int64, int, int64) error { return errors.New("boom") }
		modTuningWrites.SetReasonRequired = func(context.Context, int64, bool, int64) error { return errors.New("boom") }
		t.Cleanup(func() { modTuningWrites = prevWrites })

		if got := applyDefaultBanSecs(ctx, s.chat.Id, 3600, 7); got != modTuningFailed {
			t.Errorf("applyDefaultBanSecs on err = %v, want modTuningFailed", got)
		}
		if got := applyDefaultMuteSecs(ctx, s.chat.Id, 3600, 7); got != modTuningFailed {
			t.Errorf("applyDefaultMuteSecs on err = %v, want modTuningFailed", got)
		}
		if got := applyReasonRequired(ctx, s.chat.Id, false, 7); got != modTuningFailed {
			t.Errorf("applyReasonRequired on err = %v, want modTuningFailed", got)
		}
	})

	t.Run("invalid_positive_default_fails_safe", func(t *testing.T) {
		s := newModerationGoldenSite(t)
		cleanupModTuningChat(t, s.chat.Id)

		prevLookup := manualDefaultsLookup
		manualDefaultsLookup = func(context.Context, int64) (modtuning.Defaults, error) {
			return modtuning.Defaults{DefaultBanSecs: 10, DefaultMuteSecs: 10}, nil
		}
		t.Cleanup(func() { manualDefaultsLookup = prevLookup })

		c := &moderationCtx{Bot: s.bot, Chat: &s.chat}
		tgt := &target{userID: noticeGoldenTarget.Id}
		if err := manualBanMember(c, tgt); !errors.Is(err, errPunishTimeRanOut) {
			t.Fatalf("manualBanMember with invalid positive default = %v, want errPunishTimeRanOut", err)
		}
		if err := manualMuteMember(c, tgt); !errors.Is(err, errPunishTimeRanOut) {
			t.Fatalf("manualMuteMember with invalid positive default = %v, want errPunishTimeRanOut", err)
		}
		if bans := s.client.callsFor("banChatMember"); len(bans) != 0 {
			t.Fatalf("expected 0 banChatMember calls on failed default expiry, got %d", len(bans))
		}
		if restricts := s.client.callsFor("restrictChatMember"); len(restricts) != 0 {
			t.Fatalf("expected 0 restrictChatMember calls on failed default expiry, got %d", len(restricts))
		}
	})
}
