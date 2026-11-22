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
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/db/logchannels"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/db/modtuning"
	"github.com/uasneppy/fuku_robot/fuku/db/warns"
)

func newReasonShortcutSite(t *testing.T, logChannelID int64) *botSite {
	t.Helper()
	withFastPMCopyPacer(t)
	s := newModerationGoldenSite(t)
	s.bot.BotClient = eventPMTestClient{BotClient: s.bot.BotClient, script: &eventPMScript{}}
	cleanupModTuningChat(t, s.chat.Id)
	eventMsgCleanup(t, s.chat.Id)
	t.Cleanup(pmCopyJobs.wait)
	if logChannelID != 0 {
		if err := logchannels.Set(s.chat.Id, s.chat.Title, logChannelID); err != nil {
			t.Fatalf("logchannels.Set: %v", err)
		}
		t.Cleanup(func() { _ = logchannels.Unset(s.chat.Id) })
	}
	return s
}

func sendMessagesToChat(s *botSite, chatID int64) []moduleBotCall {
	want := strconv.FormatInt(chatID, 10)
	var out []moduleBotCall
	for _, call := range s.client.callsFor("sendMessage") {
		if fmt.Sprint(call.Params["chat_id"]) == want {
			out = append(out, call)
		}
	}
	return out
}

func TestReasonShortcutValidationAndApply(t *testing.T) {
	ctx := context.Background()

	t.Run("word_normalisation_and_validation", func(t *testing.T) {
		if got := normaliseReasonShortcutWord("  SpAm\t\n"); got != "spam" {
			t.Fatalf("normaliseReasonShortcutWord ASCII = %q, want %q", got, "spam")
		}
		if got := normaliseReasonShortcutWord("  СПАМ "); got != "спам" {
			t.Fatalf("normaliseReasonShortcutWord Cyrillic = %q, want %q", got, "спам")
		}

		cases := []struct {
			name string
			word string
			want reasonWordOutcome
		}{
			{"empty", "", reasonWordBlank},
			{"multi_word_space", "two words", reasonWordMultiWord},
			{"multi_word_tab", "two\twords", reasonWordMultiWord},
			{"multi_word_newline", "two\nwords", reasonWordMultiWord},
			{"leading_slash", "/spam", reasonWordLeadingSlash},
			{"bare_slash", "/", reasonWordLeadingSlash},
			{"exact_32_runes_cyrillic", strings.Repeat("я", models.ReasonShortcutWordMaxRunes), reasonWordOK},
			{"33_runes_cyrillic", strings.Repeat("я", models.ReasonShortcutWordMaxRunes+1), reasonWordTooLong},
			{"valid_ascii", "spam", reasonWordOK},
			{"valid_punctuation", "rule-1_a", reasonWordOK},
		}
		for _, tc := range cases {
			if got := checkReasonShortcutWord(tc.word); got != tc.want {
				t.Errorf("checkReasonShortcutWord(%q) [%s] = %v, want %v", tc.word, tc.name, got, tc.want)
			}
		}
	})

	t.Run("text_validation", func(t *testing.T) {
		if trimmed, ok, tooLong := checkReasonShortcutText("   \t\n "); ok || tooLong || trimmed != "" {
			t.Fatalf("blank text = (%q, %v, %v), want ('', false, false)", trimmed, ok, tooLong)
		}
		if trimmed, ok, tooLong := checkReasonShortcutText("  Repeated <promo> & links  "); !ok || tooLong || trimmed != "Repeated <promo> & links" {
			t.Fatalf("valid text = (%q, %v, %v), want trimmed ok", trimmed, ok, tooLong)
		}
		maxText := strings.Repeat("ї", models.ReasonShortcutTextMaxRunes)
		if trimmed, ok, tooLong := checkReasonShortcutText(maxText); !ok || tooLong || trimmed != maxText {
			t.Fatalf("500-rune text = (len=%d, %v, %v), want ok", len(trimmed), ok, tooLong)
		}
		overText := strings.Repeat("ї", models.ReasonShortcutTextMaxRunes+1)
		if trimmed, ok, tooLong := checkReasonShortcutText(overText); ok || !tooLong || trimmed != "" {
			t.Fatalf("501-rune text = (%q, %v, %v), want ('', false, true)", trimmed, ok, tooLong)
		}
	})

	t.Run("apply_add_duplicate_cap_and_foreign_remove", func(t *testing.T) {
		const chatA int64 = -1009911001
		const chatB int64 = -1009911002
		cleanupModTuningChat(t, chatA)
		cleanupModTuningChat(t, chatB)

		// Invalid inputs never hit storage.
		for _, bad := range []struct{ word, text string }{
			{"", "valid"},
			{"two words", "valid"},
			{"/cmd", "valid"},
			{strings.Repeat("a", models.ReasonShortcutWordMaxRunes+1), "valid"},
			{"ok", "   "},
			{"ok", strings.Repeat("b", models.ReasonShortcutTextMaxRunes+1)},
		} {
			if got := applyReasonShortcutAdd(ctx, chatA, bad.word, bad.text, 7); got != reasonShortcutInvalid {
				t.Errorf("applyReasonShortcutAdd(%q, len=%d) = %v, want reasonShortcutInvalid", bad.word, len(bad.text), got)
			}
		}

		// Save and refuse case-insensitive duplicate without overwriting.
		if got := applyReasonShortcutAdd(ctx, chatA, "  Spam ", "  Original <text> & more ", 7); got != reasonShortcutSaved {
			t.Fatalf("first add = %v, want reasonShortcutSaved", got)
		}
		if got := applyReasonShortcutAdd(ctx, chatA, "SPAM", "Replacement text", 8); got != reasonShortcutExists {
			t.Fatalf("duplicate add = %v, want reasonShortcutExists", got)
		}
		entries, err := modtuning.ListShortcutsFresh(ctx, chatA)
		if err != nil || len(entries) != 1 {
			t.Fatalf("ListShortcutsFresh = (%+v, %v), want 1 entry", entries, err)
		}
		if entries[0].Word != "spam" || entries[0].Reason != "Original <text> & more" {
			t.Fatalf("stored entry = %+v, want original untouched", entries[0])
		}

		// Foreign-chat remove must refuse as gone and leave chatA's row intact.
		if word, out := applyReasonShortcutRemoveByID(ctx, chatB, entries[0].ID); out != reasonShortcutGone || word != "" {
			t.Fatalf("foreign remove = (%q, %v), want ('', reasonShortcutGone)", word, out)
		}
		if exists, err := modtuning.ExistsShortcutFresh(ctx, chatA, "spam"); err != nil || !exists {
			t.Fatalf("chatA row vanished after foreign remove: exists=%v err=%v", exists, err)
		}

		// Fill chatA to cap (50 total: 1 existing + 49 more) and verify 51st is refused.
		for i := 1; i < models.ReasonShortcutsCapMax; i++ {
			w := fmt.Sprintf("w%02d", i)
			if got := applyReasonShortcutAdd(ctx, chatA, w, "Reason "+w, 7); got != reasonShortcutSaved {
				t.Fatalf("add #%d (%s) = %v, want reasonShortcutSaved", i+1, w, got)
			}
		}
		if got := applyReasonShortcutAdd(ctx, chatA, "overcap", "51st shortcut", 7); got != reasonShortcutCap {
			t.Fatalf("51st add = %v, want reasonShortcutCap", got)
		}

		// Owning-chat remove succeeds and repeat remove reports gone.
		if word, out := applyReasonShortcutRemoveByID(ctx, chatA, entries[0].ID); out != reasonShortcutRemoved || word != "spam" {
			t.Fatalf("owner remove = (%q, %v), want ('spam', reasonShortcutRemoved)", word, out)
		}
		if word, out := applyReasonShortcutRemoveByID(ctx, chatA, entries[0].ID); out != reasonShortcutGone || word != "" {
			t.Fatalf("repeat remove = (%q, %v), want ('', reasonShortcutGone)", word, out)
		}
	})

	t.Run("failure_paths_and_ids_only_logging", func(t *testing.T) {
		const chatID int64 = -1009911003
		const secretWord = "secretword99"
		const secretReason = "top-secret-reason-body-never-logged"

		var logBuf bytes.Buffer
		prevOut := log.StandardLogger().Out
		log.SetOutput(&logBuf)
		t.Cleanup(func() { log.SetOutput(prevOut) })

		// Stage race on Add (returns added=false, err=nil after fresh checks pass).
		prevWrites := reasonShortcutWrites
		t.Cleanup(func() { reasonShortcutWrites = prevWrites })
		reasonShortcutWrites.ExistsFresh = func(context.Context, int64, string) (bool, error) { return false, nil }
		reasonShortcutWrites.CountFresh = func(context.Context, int64) (int, error) { return 0, nil }
		reasonShortcutWrites.Add = func(context.Context, int64, string, string, int64) (bool, error) { return false, nil }
		if got := applyReasonShortcutAdd(ctx, chatID, secretWord, secretReason, 42); got != reasonShortcutExists {
			t.Fatalf("raced Add = %v, want reasonShortcutExists", got)
		}

		// Stage DB errors containing secret text to prove logs never echo errors or user text.
		leakyErr := fmt.Errorf("db error mentioning %s and %s", secretWord, secretReason)
		reasonShortcutWrites.ExistsFresh = func(context.Context, int64, string) (bool, error) { return false, leakyErr }
		if got := applyReasonShortcutAdd(ctx, chatID, secretWord, secretReason, 42); got != reasonShortcutFailed {
			t.Fatalf("ExistsFresh err = %v, want reasonShortcutFailed", got)
		}

		reasonShortcutWrites.ExistsFresh = func(context.Context, int64, string) (bool, error) { return false, nil }
		reasonShortcutWrites.CountFresh = func(context.Context, int64) (int, error) { return 0, leakyErr }
		if got := applyReasonShortcutAdd(ctx, chatID, secretWord, secretReason, 42); got != reasonShortcutFailed {
			t.Fatalf("CountFresh err = %v, want reasonShortcutFailed", got)
		}

		reasonShortcutWrites.CountFresh = func(context.Context, int64) (int, error) { return 0, nil }
		reasonShortcutWrites.Add = func(context.Context, int64, string, string, int64) (bool, error) { return false, leakyErr }
		if got := applyReasonShortcutAdd(ctx, chatID, secretWord, secretReason, 42); got != reasonShortcutFailed {
			t.Fatalf("Add err = %v, want reasonShortcutFailed", got)
		}

		reasonShortcutWrites.RemoveByID = func(context.Context, int64, int64) (string, bool, error) { return "", false, leakyErr }
		if word, out := applyReasonShortcutRemoveByID(ctx, chatID, 77); out != reasonShortcutRemoveFailed || word != "" {
			t.Fatalf("RemoveByID err = (%q, %v), want ('', reasonShortcutRemoveFailed)", word, out)
		}

		prevLookup := manualShortcutsLookup
		manualShortcutsLookup = func(context.Context, int64) (map[string]string, error) { return nil, leakyErr }
		t.Cleanup(func() { manualShortcutsLookup = prevLookup })
		if got := expandManualReason(chatID, secretWord); got != secretWord {
			t.Fatalf("expandManualReason on error = %q, want %q", got, secretWord)
		}

		logged := logBuf.String()
		if !strings.Contains(logged, strconv.FormatInt(chatID, 10)) {
			t.Errorf("expected Warn logs naming chat %d, got %q", chatID, logged)
		}
		if strings.Contains(logged, secretWord) {
			t.Errorf("log leaked shortcut word %q: %s", secretWord, logged)
		}
		if strings.Contains(logged, secretReason) {
			t.Errorf("log leaked shortcut text %q: %s", secretReason, logged)
		}
	})
}

func TestReasonShortcutExpansion(t *testing.T) {
	withModerationGoldenLocales(t)
	ctx := context.Background()

	const (
		shortcutWord = "spam"
		fullReason   = "Repeated unsolicited <promo> & links"
		escapedText  = "Repeated unsolicited &lt;promo&gt; &amp; links"
		logChannelID = int64(-100777888999)
	)

	t.Run("ban_default_custom_pm_and_log_html_escaped", func(t *testing.T) {
		t.Run("default_reply_pm_and_log_case_insensitive", func(t *testing.T) {
			s := newReasonShortcutSite(t, logChannelID)
			if got := applyReasonShortcutAdd(ctx, s.chat.Id, shortcutWord, fullReason, 1); got != reasonShortcutSaved {
				t.Fatalf("seed shortcut = %v", got)
			}
			storeEventPM(t, s.chat.Id, eventmsg.EventBan, "", true)

			if err := bansModule.ban(s.bot, s.commandCtx("/ban SPAM", true, false)); err != ext.EndGroups {
				t.Fatalf("/ban SPAM error = %v, want EndGroups", err)
			}
			waitEventPM(t)

			groupSends := sendMessagesToChat(s, s.chat.Id)
			if len(groupSends) != 1 {
				t.Fatalf("group sendMessage calls = %d, want 1", len(groupSends))
			}
			groupText := fmt.Sprint(groupSends[0].Params["text"])
			if !strings.Contains(groupText, escapedText) || strings.Contains(groupText, "<promo>") {
				t.Fatalf("group default reply = %q, want HTML-escaped %q", groupText, escapedText)
			}

			pmSends := sendMessagesToChat(s, eventPMMember)
			if len(pmSends) != 1 {
				t.Fatalf("PM copy sendMessage calls = %d, want 1", len(pmSends))
			}
			pmText := fmt.Sprint(pmSends[0].Params["text"])
			if !strings.Contains(pmText, escapedText) || strings.Contains(pmText, "<promo>") {
				t.Fatalf("PM default copy = %q, want HTML-escaped %q", pmText, escapedText)
			}

			logSends := sendMessagesToChat(s, logChannelID)
			if len(logSends) != 1 {
				t.Fatalf("log channel sendMessage calls = %d, want 1", len(logSends))
			}
			logText := fmt.Sprint(logSends[0].Params["text"])
			if !strings.Contains(logText, escapedText) || strings.Contains(logText, "<promo>") {
				t.Fatalf("log channel post = %q, want HTML-escaped %q", logText, escapedText)
			}
		})

		t.Run("custom_ban_message_and_custom_pm_copy", func(t *testing.T) {
			s := newReasonShortcutSite(t, logChannelID)
			if got := applyReasonShortcutAdd(ctx, s.chat.Id, shortcutWord, fullReason, 1); got != reasonShortcutSaved {
				t.Fatalf("seed shortcut = %v", got)
			}
			eventMsgStore(t, s.chat.Id, eventmsg.EventBan, "BANNED: {reason}")
			storeEventPM(t, s.chat.Id, eventmsg.EventBan, "PM BANNED: {reason}", true)

			if err := bansModule.ban(s.bot, s.commandCtx("/ban spam", true, false)); err != ext.EndGroups {
				t.Fatalf("/ban spam error = %v, want EndGroups", err)
			}
			waitEventPM(t)

			groupSends := sendMessagesToChat(s, s.chat.Id)
			if len(groupSends) != 1 || fmt.Sprint(groupSends[0].Params["text"]) != "BANNED: "+escapedText {
				t.Fatalf("custom group reply = %v, want %q", groupSends, "BANNED: "+escapedText)
			}
			pmSends := sendMessagesToChat(s, eventPMMember)
			if len(pmSends) != 1 || fmt.Sprint(pmSends[0].Params["text"]) != "PM BANNED: "+escapedText {
				t.Fatalf("custom PM copy = %v, want %q", pmSends, "PM BANNED: "+escapedText)
			}
			logSends := sendMessagesToChat(s, logChannelID)
			if len(logSends) != 1 || !strings.Contains(fmt.Sprint(logSends[0].Params["text"]), escapedText) {
				t.Fatalf("log channel post = %v, want %q", logSends, escapedText)
			}
		})
	})

	t.Run("tban_and_tmute_preserve_duration_and_expand", func(t *testing.T) {
		t.Run("tban_2h_spam", func(t *testing.T) {
			s := newReasonShortcutSite(t, logChannelID)
			if got := applyReasonShortcutAdd(ctx, s.chat.Id, shortcutWord, fullReason, 1); got != reasonShortcutSaved {
				t.Fatalf("seed shortcut = %v", got)
			}
			storeEventPM(t, s.chat.Id, eventmsg.EventTimedBan, "", true)

			before := time.Now().Unix()
			if err := bansModule.tBan(s.bot, s.commandCtx("/tban 2h spam", true, false)); err != ext.EndGroups {
				t.Fatalf("/tban 2h spam error = %v, want EndGroups", err)
			}
			after := time.Now().Unix()
			waitEventPM(t)

			bans := s.client.callsFor("banChatMember")
			if len(bans) != 1 {
				t.Fatalf("banChatMember calls = %d, want 1", len(bans))
			}
			until := modTuningUntilDate(t, bans[0])
			if until < before+7200-5 || until > after+7200+5 {
				t.Fatalf("/tban until_date = %d, want ~now+7200", until)
			}

			for name, chatID := range map[string]int64{"group": s.chat.Id, "pm": eventPMMember, "log": logChannelID} {
				msgs := sendMessagesToChat(s, chatID)
				if len(msgs) != 1 {
					t.Fatalf("%s sends = %d, want 1", name, len(msgs))
				}
				txt := fmt.Sprint(msgs[0].Params["text"])
				if !strings.Contains(txt, escapedText) || strings.Contains(txt, "<promo>") {
					t.Fatalf("%s text = %q, want HTML-escaped %q", name, txt, escapedText)
				}
			}
		})

		t.Run("tmute_30m_SPAM", func(t *testing.T) {
			s := newReasonShortcutSite(t, logChannelID)
			if got := applyReasonShortcutAdd(ctx, s.chat.Id, shortcutWord, fullReason, 1); got != reasonShortcutSaved {
				t.Fatalf("seed shortcut = %v", got)
			}
			storeEventPM(t, s.chat.Id, eventmsg.EventTimedMute, "", true)

			before := time.Now().Unix()
			if err := mutesModule.tMute(s.bot, s.commandCtx("/tmute 30m SPAM", true, false)); err != ext.EndGroups {
				t.Fatalf("/tmute 30m SPAM error = %v, want EndGroups", err)
			}
			after := time.Now().Unix()
			waitEventPM(t)

			mutes := s.client.callsFor("restrictChatMember")
			if len(mutes) != 1 {
				t.Fatalf("restrictChatMember calls = %d, want 1", len(mutes))
			}
			until := modTuningUntilDate(t, mutes[0])
			if until < before+1800-5 || until > after+1800+5 {
				t.Fatalf("/tmute until_date = %d, want ~now+1800", until)
			}

			for name, chatID := range map[string]int64{"group": s.chat.Id, "pm": eventPMMember, "log": logChannelID} {
				msgs := sendMessagesToChat(s, chatID)
				if len(msgs) != 1 {
					t.Fatalf("%s sends = %d, want 1", name, len(msgs))
				}
				txt := fmt.Sprint(msgs[0].Params["text"])
				if !strings.Contains(txt, escapedText) || strings.Contains(txt, "<promo>") {
					t.Fatalf("%s text = %q, want HTML-escaped %q", name, txt, escapedText)
				}
			}
		})
	})

	t.Run("kick_mute_dban_and_silent_commands_expand", func(t *testing.T) {
		cases := []struct {
			name   string
			text   string
			reply  bool
			silent bool
			call   func(b *gotgbot.Bot, ctx *ext.Context) error
		}{
			{"kick", "/kick spam", true, false, bansModule.kick},
			{"dkick", "/dkick spam", true, false, bansModule.dkick},
			{"mute", "/mute spam", true, false, mutesModule.mute},
			{"dmute", "/dmute spam", true, false, mutesModule.dMute},
			{"dban", "/dban spam", true, false, bansModule.dBan},
			{"sban", "/sban 42 spam", false, true, bansModule.sBan},
			{"skick", "/skick 42 spam", false, true, bansModule.sKick},
			{"smute", "/smute 42 spam", false, true, mutesModule.sMute},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				s := newReasonShortcutSite(t, logChannelID)
				if got := applyReasonShortcutAdd(ctx, s.chat.Id, shortcutWord, fullReason, 1); got != reasonShortcutSaved {
					t.Fatalf("seed shortcut = %v", got)
				}
				if err := tc.call(s.bot, s.commandCtx(tc.text, tc.reply, false)); err != ext.EndGroups {
					t.Fatalf("%s error = %v, want EndGroups", tc.text, err)
				}

				groupSends := sendMessagesToChat(s, s.chat.Id)
				if tc.silent {
					if len(groupSends) != 0 {
						t.Fatalf("%s silent command sent %d group messages, want 0", tc.name, len(groupSends))
					}
				} else {
					if len(groupSends) != 1 {
						t.Fatalf("%s group sends = %d, want 1", tc.name, len(groupSends))
					}
					groupText := fmt.Sprint(groupSends[0].Params["text"])
					if !strings.Contains(groupText, escapedText) || strings.Contains(groupText, "<promo>") {
						t.Fatalf("%s group reply = %q, want HTML-escaped %q", tc.name, groupText, escapedText)
					}
				}

				logSends := sendMessagesToChat(s, logChannelID)
				if len(logSends) != 1 {
					t.Fatalf("%s log sends = %d, want 1", tc.name, len(logSends))
				}
				logText := fmt.Sprint(logSends[0].Params["text"])
				if !strings.Contains(logText, escapedText) || strings.Contains(logText, "<promo>") {
					t.Fatalf("%s log post = %q, want HTML-escaped %q", tc.name, logText, escapedText)
				}
			})
		}
	})

	t.Run("warn_dwarn_swarn_expand_store_pm_and_limit_escape", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			text string
			call func(b *gotgbot.Bot, ctx *ext.Context) error
		}{
			{"warn", "/warn spam", warnsModule.warnUser},
			{"dwarn", "/dwarn spam", warnsModule.dWarnUser},
			{"swarn", "/swarn SPAM", warnsModule.sWarnUser},
		} {
			t.Run(tc.name, func(t *testing.T) {
				s := newReasonShortcutSite(t, 0)
				if got := applyReasonShortcutAdd(ctx, s.chat.Id, shortcutWord, fullReason, 1); got != reasonShortcutSaved {
					t.Fatalf("seed shortcut = %v", got)
				}
				storeEventPM(t, s.chat.Id, eventmsg.EventWarn, "", true)

				if err := tc.call(s.bot, s.commandCtx(tc.text, true, false)); err != ext.EndGroups {
					t.Fatalf("%s error = %v, want EndGroups", tc.text, err)
				}
				waitEventPM(t)

				groupSends := sendMessagesToChat(s, s.chat.Id)
				if len(groupSends) != 1 {
					t.Fatalf("%s group sends = %d, want 1", tc.name, len(groupSends))
				}
				groupText := fmt.Sprint(groupSends[0].Params["text"])
				if !strings.Contains(groupText, escapedText) || strings.Contains(groupText, "<promo>") {
					t.Fatalf("%s warn reply = %q, want HTML-escaped %q", tc.name, groupText, escapedText)
				}

				pmSends := sendMessagesToChat(s, eventPMMember)
				if len(pmSends) != 1 {
					t.Fatalf("%s PM sends = %d, want 1", tc.name, len(pmSends))
				}
				pmText := fmt.Sprint(pmSends[0].Params["text"])
				if !strings.Contains(pmText, escapedText) || strings.Contains(pmText, "<promo>") {
					t.Fatalf("%s PM copy = %q, want HTML-escaped %q", tc.name, pmText, escapedText)
				}

				count, reasons := warns.GetWarns(moderationGoldenTarget.Id, s.chat.Id)
				if count != 1 || len(reasons) != 1 || reasons[0] != fullReason {
					t.Fatalf("%s stored warns = (%d, %v), want (1, [%q])", tc.name, count, reasons, fullReason)
				}
			})
		}

		t.Run("warns_command_plain_and_warn_limit_escaped", func(t *testing.T) {
			s := newReasonShortcutSite(t, 0)
			if got := applyReasonShortcutAdd(ctx, s.chat.Id, shortcutWord, fullReason, 1); got != reasonShortcutSaved {
				t.Fatalf("seed shortcut = %v", got)
			}

			// First warn stores raw expanded text.
			if err := warnsModule.warnUser(s.bot, s.commandCtx("/warn spam", true, false)); err != ext.EndGroups {
				t.Fatalf("/warn #1 error = %v", err)
			}
			// /warns prints plain text (no parse_mode).
			if err := warnsModule.warns(s.bot, s.commandCtx("/warns", true, false)); err != ext.EndGroups {
				t.Fatalf("/warns error = %v", err)
			}
			groupSends := sendMessagesToChat(s, s.chat.Id)
			if len(groupSends) != 2 {
				t.Fatalf("group sends after /warn + /warns = %d, want 2", len(groupSends))
			}
			warnsText := fmt.Sprint(groupSends[1].Params["text"])
			if !strings.Contains(warnsText, fullReason) {
				t.Fatalf("/warns output = %q, want raw plain text %q", warnsText, fullReason)
			}
			if pm, ok := groupSends[1].Params["parse_mode"]; ok && pm != "" {
				t.Fatalf("/warns parse_mode = %v, want empty (plain text)", pm)
			}

			// Reach default warn limit (3) and verify warn-limit reply HTML-escapes stored reasons.
			if err := warnsModule.warnUser(s.bot, s.commandCtx("/warn spam", true, false)); err != ext.EndGroups {
				t.Fatalf("/warn #2 error = %v", err)
			}
			if err := warnsModule.warnUser(s.bot, s.commandCtx("/warn spam", true, false)); err != ext.EndGroups {
				t.Fatalf("/warn #3 error = %v", err)
			}
			groupSends = sendMessagesToChat(s, s.chat.Id)
			limitText := fmt.Sprint(groupSends[len(groupSends)-1].Params["text"])
			if !strings.Contains(limitText, escapedText) || strings.Contains(limitText, "<promo>") {
				t.Fatalf("warn-limit reply = %q, want HTML-escaped %q", limitText, escapedText)
			}
		})
	})

	t.Run("unknown_word_and_multiword_verbatim_with_zero_lookup_on_multiword", func(t *testing.T) {
		s := newReasonShortcutSite(t, logChannelID)
		if got := applyReasonShortcutAdd(ctx, s.chat.Id, shortcutWord, fullReason, 1); got != reasonShortcutSaved {
			t.Fatalf("seed shortcut = %v", got)
		}

		var lookups atomic.Int32
		prevLookup := manualShortcutsLookup
		manualShortcutsLookup = func(c context.Context, chatID int64) (map[string]string, error) {
			lookups.Add(1)
			return prevLookup(c, chatID)
		}
		t.Cleanup(func() { manualShortcutsLookup = prevLookup })

		// Unknown single word -> 1 lookup, kept verbatim.
		if err := bansModule.ban(s.bot, s.commandCtx("/ban other", true, false)); err != ext.EndGroups {
			t.Fatalf("/ban other error = %v", err)
		}
		if got := lookups.Load(); got != 1 {
			t.Fatalf("lookup count after unknown single word = %d, want 1", got)
		}
		groupSends := sendMessagesToChat(s, s.chat.Id)
		if len(groupSends) != 1 || !strings.Contains(fmt.Sprint(groupSends[0].Params["text"]), "other") {
			t.Fatalf("unknown word reply = %v, want 'other'", groupSends)
		}

		// Multi-word reason -> 0 additional lookups, kept verbatim.
		lookups.Store(0)
		if err := bansModule.ban(s.bot, s.commandCtx("/ban spam extra", true, false)); err != ext.EndGroups {
			t.Fatalf("/ban spam extra error = %v", err)
		}
		if got := lookups.Load(); got != 0 {
			t.Fatalf("lookup count for multi-word reason = %d, want 0", got)
		}
		groupSends = sendMessagesToChat(s, s.chat.Id)
		lastGroup := fmt.Sprint(groupSends[len(groupSends)-1].Params["text"])
		if !strings.Contains(lastGroup, "spam extra") || strings.Contains(lastGroup, escapedText) {
			t.Fatalf("multi-word reply = %q, want verbatim 'spam extra'", lastGroup)
		}
		logSends := sendMessagesToChat(s, logChannelID)
		lastLog := fmt.Sprint(logSends[len(logSends)-1].Params["text"])
		if !strings.Contains(lastLog, "spam extra") || strings.Contains(lastLog, escapedText) {
			t.Fatalf("multi-word log = %q, want verbatim 'spam extra'", lastLog)
		}

		// Empty reason -> 0 lookups.
		lookups.Store(0)
		if err := bansModule.ban(s.bot, s.commandCtx("/ban", true, false)); err != ext.EndGroups {
			t.Fatalf("/ban (empty) error = %v", err)
		}
		if got := lookups.Load(); got != 0 {
			t.Fatalf("lookup count for empty reason = %d, want 0", got)
		}
	})

	t.Run("fail_open_on_db_read_error", func(t *testing.T) {
		s := newReasonShortcutSite(t, 0)
		if got := applyReasonShortcutAdd(ctx, s.chat.Id, shortcutWord, fullReason, 1); got != reasonShortcutSaved {
			t.Fatalf("seed shortcut = %v", got)
		}

		prevLookup := manualShortcutsLookup
		manualShortcutsLookup = func(context.Context, int64) (map[string]string, error) {
			return nil, errors.New("simulated shortcuts outage")
		}
		t.Cleanup(func() { manualShortcutsLookup = prevLookup })

		var logBuf bytes.Buffer
		prevOut := log.StandardLogger().Out
		log.SetOutput(&logBuf)
		t.Cleanup(func() { log.SetOutput(prevOut) })

		if err := bansModule.ban(s.bot, s.commandCtx("/ban spam", true, false)); err != ext.EndGroups {
			t.Fatalf("/ban spam on error = %v, want EndGroups", err)
		}
		if got := len(s.client.callsFor("banChatMember")); got != 1 {
			t.Fatalf("banChatMember calls = %d, want 1 (fail-open)", got)
		}
		groupSends := sendMessagesToChat(s, s.chat.Id)
		if len(groupSends) != 1 {
			t.Fatalf("group sends = %d, want 1", len(groupSends))
		}
		groupText := fmt.Sprint(groupSends[0].Params["text"])
		if !strings.Contains(groupText, "spam") || strings.Contains(groupText, escapedText) {
			t.Fatalf("fail-open reply = %q, want literal 'spam'", groupText)
		}
		logged := logBuf.String()
		if !strings.Contains(logged, strconv.FormatInt(s.chat.Id, 10)) {
			t.Errorf("expected Warn log naming chat %d, got %q", s.chat.Id, logged)
		}
		if strings.Contains(logged, shortcutWord) || strings.Contains(logged, fullReason) {
			t.Errorf("log leaked shortcut word/text: %s", logged)
		}
	})

	t.Run("auto_warn_and_staff_group_never_expand", func(t *testing.T) {
		s := newReasonShortcutSite(t, 0)
		if got := applyReasonShortcutAdd(ctx, s.chat.Id, shortcutWord, fullReason, 1); got != reasonShortcutSaved {
			t.Fatalf("seed shortcut = %v", got)
		}

		// Automatic warn (warnSourceAuto) must never expand reason shortcuts.
		if err := warnsModule.warnThisUser(s.bot, s.commandCtx("/warn spam", true, false), moderationGoldenTarget.Id, shortcutWord, "warn", warnSourceAuto); err != ext.EndGroups {
			t.Fatalf("warnSourceAuto error = %v, want EndGroups", err)
		}
		count, reasons := warns.GetWarns(moderationGoldenTarget.Id, s.chat.Id)
		if count != 1 || len(reasons) != 1 || reasons[0] != shortcutWord {
			t.Fatalf("warnSourceAuto stored warns = (%d, %v), want (1, [%q])", count, reasons, shortcutWord)
		}
		groupSends := sendMessagesToChat(s, s.chat.Id)
		if len(groupSends) != 1 || strings.Contains(fmt.Sprint(groupSends[0].Params["text"]), escapedText) {
			t.Fatalf("warnSourceAuto reply expanded shortcut unexpectedly: %v", groupSends)
		}

		// Staff Group commands never expand reason shortcuts (EVT-10).
		banSpec := staffCommandSpec{Name: "ban", Kind: staffKindBan, Duration: staffDurationOptional}
		banReq, banRes := parseStaffActionArgs(&gotgbot.Message{Text: "/ban 4242 spam"}, banSpec)
		if banRes != staffParseOK || banReq.Reason != shortcutWord {
			t.Fatalf("Staff /ban parse = (%+v, %d), want Reason=%q untouched", banReq, banRes, shortcutWord)
		}
		staffFiles, err := filepath.Glob("staff*.go")
		if err != nil {
			t.Fatalf("glob staff*.go: %v", err)
		}
		if len(staffFiles) == 0 {
			t.Fatal("expected staff*.go files in fuku/modules")
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
			for _, sym := range []string{"expandManualReason", "manualShortcutsLookup", "fuku/db/modtuning"} {
				if strings.Contains(src, sym) {
					t.Errorf("Staff Group file %s must never reference %s (EVT-10)", path, sym)
				}
			}
		}
	})
}
