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
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	logrustest "github.com/sirupsen/logrus/hooks/test"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/antiflood"
	dbcache "github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/deletion"
	"github.com/uasneppy/fuku_robot/fuku/db/escalation"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
)

// TestTimedNoticeTextsFitEveryLocale fills each timed default text with a 64-rune name full of markup, the longest
// duration label of the language and a 200-character reason: it fits 4096 UTF-16 units, leaves no placeholder
// unfilled and shows the name only escaped (D-15, D-17).
func TestTimedNoticeTextsFitEveryLocale(t *testing.T) {
	withRealLocales(t)
	name := strings.Repeat(`<&"'>`, 13)[:64]
	reason := strings.Repeat("р&<", 67)[:200]
	keys := []string{
		"warns_limit_tmuted", "warns_limit_tbanned",
		"antiflood_checkflood_timed_mute", "antiflood_checkflood_timed_ban",
		"blacklists_bl_watcher_tmuted_user", "blacklists_bl_watcher_tbanned_user",
	}
	for _, code := range []string{"uk", "en", "es", "fr", "hi", "id", "pt", "ru"} {
		t.Run(code, func(t *testing.T) {
			tr := i18n.MustNewTranslator(code)
			longest := ""
			for _, secs := range []int{models.PunishMinSecs, 3600, 86400, 604800, 2592000, models.PunishMaxSecs} {
				if label := punishDurationLabel(tr, secs); len([]rune(label)) > len([]rune(longest)) {
					longest = label
				}
			}
			for _, key := range keys {
				text, err := tr.GetString(key, i18n.TranslationParams{
					"user": html.EscapeString(name), "duration": longest, "reason": html.EscapeString(reason), "count": 100, "limit": 100,
				})
				if err != nil || text == "" {
					t.Errorf("%s/%s: %q, %v", code, key, text, err)
					continue
				}
				if units := panelUTF16Len(text); units > 4096 {
					t.Errorf("%s/%s is %d UTF-16 units, want at most 4096", code, key, units)
				}
				if strings.Contains(text, "{") || strings.Contains(text, "%") {
					t.Errorf("%s/%s left a placeholder unfilled: %q", code, key, text)
				}
				if strings.Contains(text, name) {
					t.Errorf("%s/%s shows the name unescaped", code, key)
				}
				if !strings.Contains(text, html.EscapeString(name)) {
					t.Errorf("%s/%s lost the name", code, key)
				}
			}
		})
	}
}

// timedSafetyForbidden is what no Phase 26 log line may carry: the sentinels of a reason, of the text of an internal
// error and of a member's name or message, and the text around a matched word.
//
// Telegram's own refusal text is not in the list: the untimed punishments have always logged it, it is the server's
// wording and never a member's, and a timed punishment logs it the same way.
var timedSafetyForbidden = []string{"ZZTIMEDREASON", "ZZTIMEDERR", "ZZTIMEDNAME", "ZZTIMEDTEXT", "has spam inside"}

// TestTimedPunishmentLogsCarryIDsOnly drives every failing path of the phase under a debug-level hook, with sentinels
// in the errors, the member's message and the text around a matched word: no line holds a sentinel,
// and every failure line names the chat (D-42).
func TestTimedPunishmentLogsCarryIDsOnly(t *testing.T) {
	chatLine := func(chatID int64) string { return strconv.FormatInt(chatID, 10) }
	wantChatOnEveryFailure := func(t *testing.T, hook *logrustest.Hook, chatID int64) {
		t.Helper()
		for _, line := range mreqLogLines(hook) {
			if strings.Contains(line, "[Escalation]") || strings.Contains(line, "[Punish]") {
				if !strings.Contains(line, chatLine(chatID)) {
					t.Errorf("a failure line does not name the chat: %q", line)
				}
			}
		}
	}

	t.Run("unreadable ladder and failed count", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		escalationClock(t, time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC))
		escalationResetGates(t)
		hook := mreqLogCapture(t)
		mreqSwap(t, &escalationLadderLookup, func(context.Context, int64) (*escalation.Ladder, error) {
			return nil, errors.New("ZZTIMEDERR ladder")
		})
		p := autoPunishment{Action: "mute"}
		if got := escalate(context.Background(), s.bot, s.chat.Id, noticeGoldenTarget.Id, "flood", p); got != p {
			t.Fatalf("escalate with an unreadable ladder = %+v, want the rule's own action", got)
		}
		mreqSwap(t, &escalationLadderLookup, func(context.Context, int64) (*escalation.Ladder, error) {
			return &escalation.Ladder{Enabled: true, ForgetDays: 30, Steps: []escalation.Step{{Action: "kick"}}}, nil
		})
		mreqSwap(t, &escalationCount, func(context.Context, int64, int64, time.Time, time.Duration) (int, error) {
			return 0, errors.New("ZZTIMEDERR count")
		})
		escalate(context.Background(), s.bot, s.chat.Id, noticeGoldenTarget.Id, "flood", p)
		mreqWantCleanLogs(t, hook, timedSafetyForbidden,
			fmt.Sprintf("the ladder of chat %d could not be read", s.chat.Id), "could not be counted")
		wantChatOnEveryFailure(t, hook, s.chat.Id)
	})

	t.Run("an end date refused as too close", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		clock := escalationClock(t, time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC))
		_ = clock
		punishTimeRanOutWarn.mu.Lock()
		punishTimeRanOutWarn.last = nil
		punishTimeRanOutWarn.capWarned = false
		punishTimeRanOutWarn.mu.Unlock()
		hook := mreqLogCapture(t)
		if _, err := applyTimedPunishment(s.bot, s.chat.Id, noticeGoldenTarget.Id, "tmute", 20); !errors.Is(err, errPunishTimeRanOut) {
			t.Fatalf("applyTimedPunishment(20 s) error = %v, want errPunishTimeRanOut", err)
		}
		mreqWantCleanLogs(t, hook, timedSafetyForbidden, fmt.Sprintf("in chat %d cannot be sent", s.chat.Id))
		wantChatOnEveryFailure(t, hook, s.chat.Id)
	})

	t.Run("a timed restrict Telegram refuses", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		hook := mreqLogCapture(t)
		mreqSwap(t, &punishNow, func() time.Time { return s.now })
		resetAntifloodState(t)
		t.Cleanup(func() {
			db.DB.Where("chat_id = ?", s.chat.Id).Delete(&models.AntifloodSettings{})
			dbcache.DeleteCache(dbcache.CacheKey("antiflood", s.chat.Id))
			resetAntifloodState(t)
		})
		if err := antiflood.SetFlood(s.chat.Id, 1); err != nil {
			t.Fatal(err)
		}
		if err := antiflood.SetFloodActionContext(context.Background(), s.chat.Id, "tmute", 3600); err != nil {
			t.Fatal(err)
		}
		eventMsgRefuseOnce(s, "restrictChatMember", &gotgbot.TelegramError{Method: "restrictChatMember", Code: 400, Description: "Bad Request: not enough rights"})
		var refused error
		for _, id := range []int64{101, 102} {
			if err := antifloodModule.checkFlood(s.bot, noticeGoldenMessage(s, id, "ZZTIMEDTEXT flood", false)); err != nil && err != ext.ContinueGroups {
				refused = err
			}
		}
		if refused == nil {
			t.Fatal("the refused restrict was not returned to the dispatcher, so the site was not reached")
		}
		mreqWantCleanLogs(t, hook, timedSafetyForbidden)
	})

	t.Run("a timed restrict Telegram refuses at the blacklist", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		escalationClock(t, time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC))
		hook := mreqLogCapture(t)
		escalationBlacklist(t, s, "tmute", 3600)
		eventMsgRefuseOnce(s, "restrictChatMember", &gotgbot.TelegramError{Method: "restrictChatMember", Code: 400, Description: "Bad Request: not enough rights"})
		if err := blacklistsModule.blacklistWatcher(s.bot, noticeGoldenMessage(s, 101, "this has spam inside", false)); err == nil {
			t.Fatal("the refused restrict was not returned to the dispatcher, so the site was not reached")
		}
		mreqWantCleanLogs(t, hook, timedSafetyForbidden)
	})

	t.Run("a failed save of every apply function", func(t *testing.T) {
		chatID := uniqueModuleChatID()
		ctx := context.Background()
		hook := mreqLogCapture(t)
		boom := errors.New("ZZTIMEDERR save")
		mreqSwap(t, &warnSettingsWrites, warnSettingsWriter{
			Limit:  func(int64, int) error { return boom },
			Mode:   func(context.Context, int64, string, int) error { return boom },
			Expiry: func(context.Context, int64, int) error { return boom },
		})
		mreqSwap(t, &blacklistWrites.SetAction, func(context.Context, int64, string, int) (int64, error) { return 0, boom })
		mreqSwap(t, &escalationWrites.SetEnabled, func(context.Context, int64, bool, int64) error { return boom })
		mreqSwap(t, &escalationWrites.SetEnabledWithSteps, func(context.Context, int64, []escalation.Step, int64) error { return boom })
		mreqSwap(t, &escalationWrites.SetStep, func(context.Context, int64, int, escalation.Step, int64) error { return boom })
		mreqSwap(t, &escalationWrites.SetForgetDays, func(context.Context, int64, int, int64) error { return boom })
		mreqSwap(t, &escalationRead, func(context.Context, int64) (*escalation.Ladder, error) {
			return &escalation.Ladder{Enabled: true, ForgetDays: 30, Steps: []escalation.Step{{Action: "ban"}, {Action: "ban"}}}, nil
		})

		if r := applyWarnMode(ctx, chatID, "tmute", 3600); r.Outcome != warnSettingsSaveFailed {
			t.Errorf("applyWarnMode outcome = %v", r.Outcome)
		}
		if r := applyWarnExpiry(ctx, chatID, 7); r.Outcome != warnSettingsSaveFailed {
			t.Errorf("applyWarnExpiry outcome = %v", r.Outcome)
		}
		if r := applyBlacklistAction(ctx, chatID, "tmute", 3600); r.Outcome != blacklistActionSaveFailed {
			t.Errorf("applyBlacklistAction outcome = %v", r.Outcome)
		}
		for name, r := range map[string]escalationApplyResult{
			"enabled": applyEscalationEnabled(ctx, chatID, false, 1),
			"step":    applyEscalationStep(ctx, chatID, 2, escalation.Step{Action: "kick"}, 1),
			"remove":  applyEscalationRemoveStep(ctx, chatID, 2, 1),
			"forget":  applyEscalationForget(ctx, chatID, 7, 1),
		} {
			if r.Outcome != escalationSaveFailed {
				t.Errorf("escalation %s outcome = %v, want a save failure", name, r.Outcome)
			}
		}
		mreqWantCleanLogs(t, hook, timedSafetyForbidden, fmt.Sprintf("the ladder of chat %d could not be saved", chatID))
		wantChatOnEveryFailure(t, hook, chatID)
	})

	t.Run("an unreadable ladder before a change", func(t *testing.T) {
		chatID := uniqueModuleChatID()
		hook := mreqLogCapture(t)
		mreqSwap(t, &escalationRead, func(context.Context, int64) (*escalation.Ladder, error) {
			return nil, errors.New("ZZTIMEDERR read")
		})
		if r := applyEscalationForget(context.Background(), chatID, 7, 1); r.Outcome != escalationSaveFailed {
			t.Errorf("outcome = %v, want a save failure", r.Outcome)
		}
		mreqWantCleanLogs(t, hook, timedSafetyForbidden, fmt.Sprintf("the ladder of chat %d could not be read before a change", chatID))
		wantChatOnEveryFailure(t, hook, chatID)
	})
}

// timedSafetyMethods counts the Bot API methods the environment's bot has called so far, by name.
func timedSafetyMethods(env *settingsEnv) map[string]int {
	client := env.fake.moduleBotClient
	client.mu.Lock()
	defer client.mu.Unlock()
	out := map[string]int{}
	for _, call := range client.calls {
		out[call.Method]++
	}
	return out
}

// timedSafetyNewCalls is what the environment's bot called since before, without the edit and the answer of the press.
func timedSafetyNewCalls(env *settingsEnv, before map[string]int) map[string]int {
	out := map[string]int{}
	for method, n := range timedSafetyMethods(env) {
		if method == "editMessageText" || method == "answerCallbackQuery" {
			continue
		}
		if n > before[method] {
			out[method] = n - before[method]
		}
	}
	return out
}

// TestPhase26Cost holds what the new paths cost (D-29, D-40): a flood offence with the ladder off counts nothing and
// writes no offence row, with it on counts exactly once and writes one row; drawing the Escalation page and its step
// screen costs the press pipeline's own two lookups (the presser's getChatMember and the group's getChat) and no
// more; a Warnings timed press costs the pipeline's lookups too.
func TestPhase26Cost(t *testing.T) {
	t.Run("the ladder off costs no count and on costs one", func(t *testing.T) {
		s := newNoticeGoldenSite(t, deletion.KindBotAuto)
		escalationClock(t, time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC))
		escalationResetGates(t)
		counts := 0
		mreqSwap(t, &escalationCount, func(ctx context.Context, chatID, userID int64, now time.Time, forget time.Duration) (int, error) {
			counts++
			return escalation.CountOffence(ctx, chatID, userID, now, forget)
		})
		t.Cleanup(func() { db.DB.Where("chat_id = ?", s.chat.Id).Delete(&models.ChatOffence{}) })
		offenceRows := func() int64 {
			var n int64
			if err := db.DB.Model(&models.ChatOffence{}).Where("chat_id = ?", s.chat.Id).Count(&n).Error; err != nil {
				t.Fatal(err)
			}
			return n
		}
		p := autoPunishment{Action: "mute"}
		escalate(context.Background(), s.bot, s.chat.Id, noticeGoldenTarget.Id, "flood", p)
		if counts != 0 || offenceRows() != 0 {
			t.Fatalf("ladder off: %d counts and %d offence rows, want none", counts, offenceRows())
		}
		escalationLadder(t, s.chat.Id, escalation.Step{Action: "kick"}, escalation.Step{Action: "ban"})
		escalate(context.Background(), s.bot, s.chat.Id, noticeGoldenTarget.Id, "flood", p)
		if counts != 1 || offenceRows() != 1 {
			t.Fatalf("ladder on: %d counts and %d offence rows, want exactly one of each", counts, offenceRows())
		}
	})

	t.Run("the Escalation screens and a Warnings timed press cost the pipeline's lookups", func(t *testing.T) {
		env := settingsEscEnv(t)
		t.Cleanup(func() { db.DB.Where("chat_id = ?", env.chat.Id).Delete(&models.WarnSettings{}) })
		owner := env.newOwner(env.chat)
		msgID := settingsPrivateOnlyMessage(t, env, owner)
		// Each Escalation screen costs the pipeline's two lookups and nothing else.
		for name, kv := range map[string][]string{
			"the page":      {"a", "op", "p", settingsPageEscalation},
			"a step screen": {"a", settingsActEscStep, "t", "1"},
			"a duration":    {"a", settingsActEscDuration, "t", "1", "k", "m"},
		} {
			before := timedSafetyMethods(env)
			env.pressPrivate(owner, msgID, env.privateFields(env.chat.Id, kv...))
			for method, n := range timedSafetyNewCalls(env, before) {
				if (method != "getChatMember" && method != "getChat") || n > 1 {
					t.Errorf("%s made %d %s calls beyond the pipeline's own lookups", name, n, method)
				}
			}
		}
		// A Warnings timed press adds the cached bot-administrator check its right asks for (a cold admin cache is one
		// getChatMember of the bot and one getChatAdministrators), and writes through applyWarnMode.
		before := timedSafetyMethods(env)
		env.pressPrivate(owner, msgID, env.privateFields(env.chat.Id, "a", settingsActWarnsTimed, "k", "m", "v", "3600"))
		for method, n := range timedSafetyNewCalls(env, before) {
			switch method {
			case "getChatMember", "getChat":
				if n > 2 {
					t.Errorf("the warns timed press made %d %s calls, want at most 2", n, method)
				}
			case "getChatAdministrators":
				if n > 1 {
					t.Errorf("the warns timed press made %d %s calls, want at most 1", n, method)
				}
			default:
				t.Errorf("the warns timed press called %s %d times", method, n)
			}
		}
	})
}
