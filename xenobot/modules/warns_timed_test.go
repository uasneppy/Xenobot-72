//go:build testtools

package modules

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/deletion"
	"github.com/uasneppy/Xenobot-72/xenobot/db/eventmsg"
	"github.com/uasneppy/Xenobot-72/xenobot/db/memberreq"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/db/warns"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/formatting"
)

// warnTimedStored reads the chat's warns_settings row straight from the table; ok is false with no row.
func warnTimedStored(t *testing.T, chatID int64) (models.WarnSettings, bool) {
	t.Helper()
	var rows []models.WarnSettings
	if err := db.DB.Where("chat_id = ?", chatID).Find(&rows).Error; err != nil {
		t.Fatalf("read the warns_settings row of chat %d: %v", chatID, err)
	}
	if len(rows) == 0 {
		return models.WarnSettings{}, false
	}
	return rows[0], true
}

// warnTimedLastText is the text of the site's last sendMessage request.
func warnTimedLastText(t *testing.T, s *botSite) string {
	t.Helper()
	calls := s.client.callsFor("sendMessage")
	if len(calls) == 0 {
		t.Fatal("no sendMessage request")
	}
	return fmt.Sprint(calls[len(calls)-1].Params["text"])
}

// TestApplyWarnModeTimed holds the one write path of the warn mode with a duration: a valid timed mode
// stores both columns, a plain mode stores seconds 0, a timed mode with no duration is the pinned unknown
// outcome, a duration outside 60 s to 366 days is the bad-duration outcome, and neither refusal writes.
func TestApplyWarnModeTimed(t *testing.T) {
	ctx := context.Background()

	t.Run("a valid timed mode stores the mode and the seconds", func(t *testing.T) {
		chatID := warnApplyFreshChat(t)
		got := applyWarnMode(ctx, chatID, "tmute", 7200)
		if got.Outcome != warnSettingsApplied || got.Key != "warns_mode_updated_tmute" || got.Err != nil {
			t.Fatalf("applyWarnMode(tmute, 7200) = %+v, want warnSettingsApplied", got)
		}
		row, ok := warnTimedStored(t, chatID)
		if !ok || row.WarnMode != "tmute" || row.WarnModeSecs != 7200 {
			t.Fatalf("stored row = %+v (present %v), want tmute and 7200", row, ok)
		}
		if got.Stored == nil || got.Stored.WarnModeSecs != 7200 {
			t.Fatalf("read back = %+v, want seconds 7200", got.Stored)
		}
	})

	t.Run("a plain mode stores seconds 0", func(t *testing.T) {
		chatID := warnApplyFreshChat(t)
		if got := applyWarnMode(ctx, chatID, "tban", 86400); got.Outcome != warnSettingsApplied {
			t.Fatalf("seed applyWarnMode(tban, 86400) = %+v", got)
		}
		got := applyWarnMode(ctx, chatID, "ban", 0)
		if got.Outcome != warnSettingsApplied || got.Key != "warns_mode_updated_ban" {
			t.Fatalf("applyWarnMode(ban, 0) = %+v, want warnSettingsApplied", got)
		}
		if row, ok := warnTimedStored(t, chatID); !ok || row.WarnMode != "ban" || row.WarnModeSecs != 0 {
			t.Fatalf("stored row = %+v (present %v), want ban and 0", row, ok)
		}
	})

	t.Run("a timed mode with no duration is unknown and writes nothing", func(t *testing.T) {
		chatID := warnApplyFreshChat(t)
		got := applyWarnMode(ctx, chatID, "tban", 0)
		if got.Outcome != warnSettingsUnknownMode || got.Key != "warns_mode_unknown" || got.Stored != nil {
			t.Fatalf("applyWarnMode(tban, 0) = %+v, want warnSettingsUnknownMode", got)
		}
		if n := warnApplyRows(t, chatID); n != 0 {
			t.Fatalf("rows = %d, want none", n)
		}
	})

	t.Run("a duration out of bounds is a bad duration and writes nothing", func(t *testing.T) {
		for _, secs := range []int{59, punishMaxSecs + 1, -1} {
			chatID := warnApplyFreshChat(t)
			got := applyWarnMode(ctx, chatID, "tmute", secs)
			if got.Outcome != warnSettingsBadDuration || got.Key != "warns_mode_bad_duration" || got.Stored != nil {
				t.Fatalf("applyWarnMode(tmute, %d) = %+v, want warnSettingsBadDuration", secs, got)
			}
			if n := warnApplyRows(t, chatID); n != 0 {
				t.Fatalf("applyWarnMode(tmute, %d) left %d rows, want none", secs, n)
			}
		}
	})

	t.Run("a failed write is a save failure", func(t *testing.T) {
		chatID := warnApplyFreshChat(t)
		warnApplyFailingWrites(t)
		got := applyWarnMode(ctx, chatID, "tmute", 600)
		if got.Outcome != warnSettingsSaveFailed || got.Key != "common_settings_save_failed" || got.Err == nil {
			t.Fatalf("applyWarnMode(tmute, 600) = %+v, want warnSettingsSaveFailed", got)
		}
	})
}

// TestSetWarnModeTimedCommand drives /setwarnmode's timed forms and /warnings through the handlers in
// Ukrainian with the real locales: the reply names the duration, a bad duration writes nothing, a plain
// mode with an extra word stores the plain mode, and /warnings shows the short form.
func TestSetWarnModeTimedCommand(t *testing.T) {
	withModerationGoldenLocales(t)
	tr := i18n.MustNewTranslator("uk")
	site := func(t *testing.T) *botSite {
		t.Helper()
		s := newNoticeGoldenSite(t, deletion.KindBotManual)
		timedGoldenCleanup(t, s.chat.Id)
		return s
	}

	t.Run("tmute 2h", func(t *testing.T) {
		s := site(t)
		timedGoldenCommand(t, s, warnsModule.setWarnMode, timedGoldenAdmin, "/setwarnmode tmute 2h", false)
		want, _ := tr.GetString("warns_mode_updated_tmute", i18n.TranslationParams{"duration": punishDurationLabel(tr, 7200)})
		if got := warnTimedLastText(t, s); got != want {
			t.Errorf("reply = %q, want %q", got, want)
		}
		if row, ok := warnTimedStored(t, s.chat.Id); !ok || row.WarnMode != "tmute" || row.WarnModeSecs != 7200 {
			t.Fatalf("stored row = %+v (present %v), want tmute and 7200", row, ok)
		}

		timedGoldenCommand(t, s, warnsModule.warnings, timedGoldenAdmin, "/warnings", false)
		display, _ := tr.GetString("warns_settings_display")
		if got, want := warnTimedLastText(t, s), fmt.Sprintf(display, 3, "tmute 2h"); got != want {
			t.Errorf("/warnings = %q, want %q", got, want)
		}
	})

	t.Run("TBAN 1d", func(t *testing.T) {
		s := site(t)
		timedGoldenCommand(t, s, warnsModule.setWarnMode, timedGoldenAdmin, "/setwarnmode TBAN 1d", false)
		want, _ := tr.GetString("warns_mode_updated_tban", i18n.TranslationParams{"duration": punishDurationLabel(tr, 86400)})
		if got := warnTimedLastText(t, s); got != want {
			t.Errorf("reply = %q, want %q", got, want)
		}
		if row, ok := warnTimedStored(t, s.chat.Id); !ok || row.WarnMode != "tban" || row.WarnModeSecs != 86400 {
			t.Fatalf("stored row = %+v (present %v), want tban and 86400", row, ok)
		}
	})

	t.Run("bad durations write nothing", func(t *testing.T) {
		for _, text := range []string{"/setwarnmode tmute xyz", "/setwarnmode tmute 0m", "/setwarnmode tmute 400d"} {
			s := site(t)
			timedGoldenCommand(t, s, warnsModule.setWarnMode, timedGoldenAdmin, text, false)
			want, _ := tr.GetString("warns_mode_bad_duration")
			if got := warnTimedLastText(t, s); got != want {
				t.Errorf("%s: reply = %q, want %q", text, got, want)
			}
			if row, ok := warnTimedStored(t, s.chat.Id); ok {
				t.Errorf("%s: a row was written: %+v", text, row)
			}
		}
	})

	t.Run("ban 1d stores plain ban", func(t *testing.T) {
		s := site(t)
		timedGoldenCommand(t, s, warnsModule.setWarnMode, timedGoldenAdmin, "/setwarnmode ban 1d", false)
		want, _ := tr.GetString("warns_mode_updated_ban")
		if got := warnTimedLastText(t, s); got != want {
			t.Errorf("reply = %q, want %q", got, want)
		}
		if row, ok := warnTimedStored(t, s.chat.Id); !ok || row.WarnMode != "ban" || row.WarnModeSecs != 0 {
			t.Fatalf("stored row = %+v (present %v), want ban and 0", row, ok)
		}
	})
}

// warnTimedSite is a notice golden site (real locales, a fresh supergroup, member 42 answered as Ann & <Bo>)
// with a clean chat and the timed punishments' clock fixed at the site's clock.
func warnTimedSite(t *testing.T) *botSite {
	t.Helper()
	s := newNoticeGoldenSite(t, deletion.KindBotManual)
	timedGoldenCleanup(t, s.chat.Id)
	mreqSwap(t, &punishNow, func() time.Time { return s.now })
	return s
}

// warnTimedOnly returns the one call of method about member 42 and fails unless the other punishment
// calls about it are absent.
func warnTimedOnly(t *testing.T, s *botSite, method string) moduleBotCall {
	t.Helper()
	var only moduleBotCall
	for _, m := range noticeGoldenPunishments {
		calls := s.client.callsFor(m)
		want := 0
		if m == method {
			want = 1
		}
		if len(calls) != want {
			t.Fatalf("%s calls = %d, want %d", m, len(calls), want)
		}
		if want == 1 {
			only = calls[0]
		}
	}
	return only
}

// warnTimedLimitText is the default warn limit text of a timed action in Ukrainian: key with the count, the
// limit, member 42's mention and the duration label, then each reason on its own line.
func warnTimedLimitText(tr *i18n.Translator, key string, count, limit, secs int, reasons ...string) string {
	text, _ := tr.GetString(key, i18n.TranslationParams{
		"count": count, "limit": limit, "user": formatting.MentionHtml(noticeGoldenTarget.Id, noticeGoldenTarget.FirstName),
		"duration": punishDurationLabel(tr, secs),
	})
	for _, reason := range reasons {
		text += "\n - " + html.EscapeString(reason)
	}
	return text
}

// TestWarnLimitTimed is the tracer of the timed punishments: a timed warn mode set by /setwarnmode is
// applied at the warn limit with an end date computed right before the request, its message names the
// duration (default and custom), its private copy adds the Until line, a timed mode with no duration stays
// inert, an end date too close is never sent, and the member requirements' warn path follows the timed limit.
func TestWarnLimitTimed(t *testing.T) {
	withModerationGoldenLocales(t)
	tr := i18n.MustNewTranslator("uk")

	t.Run("tmute_default", func(t *testing.T) {
		s := warnTimedSite(t)
		if err := warns.SetWarnLimit(s.chat.Id, 2); err != nil {
			t.Fatalf("SetWarnLimit() error = %v", err)
		}
		timedGoldenCommand(t, s, warnsModule.setWarnMode, timedGoldenAdmin, "/setwarnmode tmute 2h", false)
		noticeGoldenWarn(t, s, warnsModule.warnUser, "/warn first", "/warn second <b>")

		call := warnTimedOnly(t, s, "restrictChatMember")
		if got, want := fmt.Sprint(call.Params["until_date"]), strconv.FormatInt(s.now.Unix()+7200, 10); got != want {
			t.Errorf("until_date = %s, want %s", got, want)
		}
		if got := fmt.Sprint(call.Params["user_id"]); got != "42" {
			t.Errorf("user_id = %s, want 42", got)
		}
		if got, ok := call.Params["permissions"].(gotgbot.ChatPermissions); !ok || got != MutedPermissions {
			t.Errorf("permissions = %#v, want MutedPermissions", call.Params["permissions"])
		}
		want := warnTimedLimitText(tr, "warns_limit_tmuted", 2, 2, 7200, "first", "second <b>")
		if got := warnTimedLastText(t, s); got != want {
			t.Errorf("warn limit message = %q, want %q", got, want)
		}
		if n, _ := warns.GetWarns(noticeGoldenTarget.Id, s.chat.Id); n != 0 {
			t.Errorf("warns after the limit = %d, want them reset", n)
		}
		s.wantRow(s.chat.Id, 9001, deletion.KindBotManual, 600*time.Second)
	})

	t.Run("tban_default", func(t *testing.T) {
		s := warnTimedSite(t)
		if err := warns.SetWarnLimit(s.chat.Id, 1); err != nil {
			t.Fatalf("SetWarnLimit() error = %v", err)
		}
		timedGoldenCommand(t, s, warnsModule.setWarnMode, timedGoldenAdmin, "/setwarnmode tban 2h", false)
		noticeGoldenWarn(t, s, warnsModule.warnUser, "/warn spam")

		call := warnTimedOnly(t, s, "banChatMember")
		if got, want := fmt.Sprint(call.Params["until_date"]), strconv.FormatInt(s.now.Unix()+7200, 10); got != want {
			t.Errorf("until_date = %s, want %s", got, want)
		}
		if got, want := warnTimedLastText(t, s), warnTimedLimitText(tr, "warns_limit_tbanned", 1, 1, 7200, "spam"); got != want {
			t.Errorf("warn limit message = %q, want %q", got, want)
		}
	})

	t.Run("tmute_custom", func(t *testing.T) {
		s := warnTimedSite(t)
		if err := warns.SetWarnLimit(s.chat.Id, 1); err != nil {
			t.Fatalf("SetWarnLimit() error = %v", err)
		}
		if err := warns.SetWarnModeContext(t.Context(), s.chat.Id, "tmute", 7200); err != nil {
			t.Fatalf("SetWarnModeContext() error = %v", err)
		}
		eventMsgStore(t, s.chat.Id, eventmsg.EventWarnLimit, "{action} / {duration} / {until}")
		noticeGoldenWarn(t, s, warnsModule.warnUser, "/warn spam")

		warnTimedOnly(t, s, "restrictChatMember")
		until := formatChatTime(time.Unix(s.now.Unix()+7200, 0), chatClockOf(s.chat.Id), "02.01.2006 15:04")
		want := html.EscapeString(eventActionWord(tr, "mute")) + " / " + html.EscapeString(punishDurationLabel(tr, 7200)) +
			" / " + until
		if got := warnTimedLastText(t, s); got != want {
			t.Errorf("custom warn limit message = %q, want %q", got, want)
		}
	})

	t.Run("pm_until", func(t *testing.T) {
		for _, c := range []struct {
			name, mode string
			secs       int
			action     string
		}{{"timed", "tmute", 7200, "mute"}, {"untimed", "mute", 0, "mute"}} {
			t.Run(c.name, func(t *testing.T) {
				s := newEventPMAutoSite(t, deletion.KindBotManual)
				timedGoldenCleanup(t, s.chat.Id)
				mreqSwap(t, &punishNow, func() time.Time { return s.now })
				s.chat.Title = eventPMAutoTitle
				if err := warns.SetWarnLimit(s.chat.Id, 1); err != nil {
					t.Fatalf("SetWarnLimit() error = %v", err)
				}
				if err := warns.SetWarnModeContext(t.Context(), s.chat.Id, c.mode, c.secs); err != nil {
					t.Fatalf("SetWarnModeContext() error = %v", err)
				}
				storeEventPM(t, s.chat.Id, eventmsg.EventWarnLimit, "", true)
				noticeGoldenWarn(t, s, warnsModule.warnUser, "/warn spam")

				text, _ := tr.GetString("eventmsg_pm_warnlimit", i18n.TranslationParams{
					"group": eventPMAutoGroup, "count": "1", "limit": "1", "action": eventActionWord(tr, c.action),
				})
				if c.secs > 0 {
					end := formatChatTime(time.Unix(s.now.Unix()+int64(c.secs), 0), chatClockOf(s.chat.Id), "02.01.2006 15:04")
					line, _ := tr.GetString("eventmsg_pm_until", i18n.TranslationParams{"until": end})
					text += "\n" + line
				}
				reason, _ := tr.GetString("eventmsg_pm_reason", i18n.TranslationParams{"reason": "spam"})
				eventPMAutoWant(t, s, text+"\n"+reason)
			})
		}
	})

	t.Run("inert_no_duration", func(t *testing.T) {
		s := warnTimedSite(t)
		if err := warns.SetWarnLimit(s.chat.Id, 1); err != nil {
			t.Fatalf("SetWarnLimit() error = %v", err)
		}
		if err := warns.SetWarnMode(s.chat.Id, "tmute"); err != nil {
			t.Fatalf("SetWarnMode() error = %v", err)
		}
		noticeGoldenWarn(t, s, warnsModule.warnUser, "/warn reason")

		warnTimedOnly(t, s, "")
		calls := s.client.callsFor("sendMessage")
		got := timedGoldenTokens(legacyRequestJSON(t, calls[len(calls)-1]), map[int64]string{s.chat.Id: "<chat>", timedGoldenAdmin.Id: "<admin>"})
		if got != goldenTimedWarnLimitInertTmute {
			t.Errorf("inert warn limit request\n got %q\nwant %q", got, goldenTimedWarnLimitInertTmute)
		}
		if n, _ := warns.GetWarns(noticeGoldenTarget.Id, s.chat.Id); n != 1 {
			t.Errorf("warns = %d, want the warn kept", n)
		}
	})

	t.Run("too_close", func(t *testing.T) {
		s := warnTimedSite(t)
		hook := captureLogs(t, log.WarnLevel)
		until, err := applyTimedPunishment(s.bot, s.chat.Id, noticeGoldenTarget.Id, "tmute", 34)
		if !errors.Is(err, errPunishTimeRanOut) || until != 0 {
			t.Fatalf("applyTimedPunishment(34 s) = %d, %v; want 0, errPunishTimeRanOut", until, err)
		}
		if _, err := applyTimedPunishment(s.bot, s.chat.Id, noticeGoldenTarget.Id, "mute", 3600); err == nil || errors.Is(err, errPunishTimeRanOut) {
			t.Fatalf("applyTimedPunishment(mute) error = %v, want a programming error", err)
		}
		warnTimedOnly(t, s, "")
		var lines []string
		for _, entry := range hook.AllEntries() {
			if entry.Level == log.WarnLevel {
				lines = append(lines, entry.Message)
			}
		}
		if len(lines) != 1 || !strings.Contains(lines[0], strconv.FormatInt(s.chat.Id, 10)) || strings.Contains(lines[0], "user") {
			t.Fatalf("warn lines = %q, want one line naming the chat only", lines)
		}
	})

	t.Run("mreq_timed_limit", func(t *testing.T) {
		for _, c := range []struct {
			mode, method string
			outcome      memberReqOutcome
		}{{"tmute", "restrictChatMember", memberReqWarned}, {"tban", "banChatMember", memberReqRemoved}} {
			t.Run(c.mode, func(t *testing.T) {
				env := newScreenGoldenMreqEnv(t)
				timedGoldenCleanup(t, env.chat.Id)
				mreqSwap(t, &punishNow, func() time.Time { return time.Unix(env.clock.Load(), 0) })
				env.mreqRules(map[memberreq.Rule]int{memberreq.RuleLastName: models.RequirementActionWarn})
				if err := warns.SetWarnLimit(env.chat.Id, 1); err != nil {
					t.Fatalf("SetWarnLimit() error = %v", err)
				}
				if err := warns.SetWarnModeContext(t.Context(), env.chat.Id, c.mode, 3600); err != nil {
					t.Fatalf("SetWarnModeContext() error = %v", err)
				}
				member := screenGoldenReqMember(env)

				got := env.mreqEnforce(env.mreqCase(member))
				if got.Outcome != c.outcome || got.Hold == nil {
					t.Fatalf("enforcement = %+v, want outcome %v with a hold", got, c.outcome)
				}
				call := screenGoldenWantAction(t, env.calls, c.method, member.Id)
				if got, want := fmt.Sprint(call.Params["until_date"]), strconv.FormatInt(env.clock.Load()+3600, 10); got != want {
					t.Errorf("until_date = %s, want %s", got, want)
				}
				notices := env.mreqNotices()
				if len(notices) != 1 {
					t.Fatalf("notices = %d, want one", len(notices))
				}
				plain := memberReqLimitMode(c.mode)
				want := memberReqNoticeText(tr, member, got.Hold.Rules, models.RequirementActionWarn,
					&warnApplyResult{Count: 1, Limit: 1, Mode: plain, LimitReached: true, LimitApplied: true})
				if text := fmt.Sprint(notices[0].Params["text"]); text != want {
					t.Errorf("notice = %q, want the %s warn-limit notice %q", text, plain, want)
				}
			})
		}
	})
}
