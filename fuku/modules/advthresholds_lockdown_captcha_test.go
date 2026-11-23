//go:build testtools

package modules

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/advthresholds"
	dbcache "github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/captcha"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
)

// newPhase28LockdownEnv mirrors newLockdownEnv using phase28InProcessMiniredis so tests run
// with zero localhost TCP dials in sandboxed environments.
func newPhase28LockdownEnv(t *testing.T) *lockdownEnv {
	t.Helper()
	redisServer := phase28InProcessMiniredis(t)
	withStaffLocale(t)
	withFastLockdownPacer(t)

	fake := newLockdownFake()
	bot := newModuleTestBot(fake.moduleBotClient)
	bot.BotClient = fake

	env := &lockdownEnv{
		t:          t,
		redis:      redisServer,
		fake:       fake,
		bot:        bot,
		chat:       gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Lock Chat"},
		admin:      gotgbot.User{Id: uniqueLinkOwnerID(), FirstName: "Ad<b>min"},
		nextUpdate: 100,
	}
	lockdownCleanup(t, env.chat.Id)
	cleanupAdvThresholdsChat(t, env.chat.Id)

	fake.setMember(env.chat.Id, env.admin.Id, staffFakeMember{
		Status:             gotgbot.ChatMemberStatusAdministrator,
		CanRestrictMembers: true,
	})
	fake.setChatPermsRaw(env.chat.Id, lockdownTestPrePermissions)

	env.dispatcher = ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadLockdown(env.dispatcher)
	t.Cleanup(raidPosts.wait)
	return env
}

// newPhase28ReminderScene locks a group backed by in-process miniredis and lets the worker post
// the initial alert in both the Staff Group and log channel.
func newPhase28ReminderScene(t *testing.T, beforeLock func(env *lockdownEnv)) *reminderScene {
	t.Helper()
	env := newPhase28LockdownEnv(t)
	staffChat := lockdownBurstEnv(env)
	channel := uniqueModuleChatID()
	setStaffLogChannel(t, env.chat.Id, channel)
	if beforeLock != nil {
		beforeLock(env)
	}
	row := env.lockAsAdmin("raid")
	env.cycle()
	s := &reminderScene{env: env, staff: staffChat, channel: channel, row: row}
	s.wantPosted(1)
	return s
}

func TestAdvancedThresholdsLockdownAndCaptcha(t *testing.T) {
	t.Run("lockdown_reminder_shorter_interval_15m", func(t *testing.T) {
		s := newPhase28ReminderScene(t, func(env *lockdownEnv) {
			if err := advthresholds.SetField(context.Background(), env.chat.Id, advthresholds.FieldLockdownReminder, 15, env.admin.Id); err != nil {
				t.Fatalf("advthresholds.SetField(lockdown_reminder_mins=15) error = %v", err)
			}
		})
		env := s.env

		ageAlert(t, s.row.ID, 16*time.Minute)
		env.cycle()

		s.wantPosted(2)
		toStaff, toLog := s.posted()
		for name, sent := range map[string]staffSentMessage{"staff": toStaff[1], "log": toLog[1]} {
			text := fmt.Sprint(sent.Params["text"])
			if !strings.Contains(text, staffMarker("lockdown_reminder_title")) {
				t.Errorf("%s reminder %q does not contain the reminder title", name, text)
			}
		}
		rows := env.reminderRows()
		if len(rows) != 2 {
			t.Fatalf("reminder rows = %d, want 2: %+v", len(rows), rows)
		}
		for _, r := range rows {
			if r.Seq != 1 || r.State != models.AlertStatePosted || r.Kind != models.AlertKindReminder {
				t.Errorf("reminder row = %+v, want posted reminder of seq 1", r)
			}
		}

		// Re-running the cycle at the same age must not re-plan or duplicate seq 1.
		env.cycle()
		s.wantPosted(2)
	})

	t.Run("lockdown_reminder_longer_interval_120m", func(t *testing.T) {
		s := newPhase28ReminderScene(t, func(env *lockdownEnv) {
			if err := advthresholds.SetField(context.Background(), env.chat.Id, advthresholds.FieldLockdownReminder, 120, env.admin.Id); err != nil {
				t.Fatalf("advthresholds.SetField(lockdown_reminder_mins=120) error = %v", err)
			}
		})
		env := s.env

		ageAlert(t, s.row.ID, 61*time.Minute)
		env.cycle()

		s.wantPosted(1)
		if rows := env.reminderRows(); len(rows) != 0 {
			t.Fatalf("reminder rows at 61m with 120m interval = %d, want 0: %+v", len(rows), rows)
		}

		ageAlert(t, s.row.ID, 121*time.Minute)
		env.cycle()

		s.wantPosted(2)
		rows := env.reminderRows()
		if len(rows) != 2 {
			t.Fatalf("reminder rows at 121m = %d, want 2: %+v", len(rows), rows)
		}
		for _, r := range rows {
			if r.Seq != 1 || r.State != models.AlertStatePosted || r.Kind != models.AlertKindReminder {
				t.Errorf("reminder row = %+v, want posted reminder of seq 1", r)
			}
		}
	})

	t.Run("lockdown_reminder_preserves_package_var_seam_when_unset", func(t *testing.T) {
		prevEvery := lockdownReminderEvery
		lockdownReminderEvery = 20 * time.Minute
		t.Cleanup(func() { lockdownReminderEvery = prevEvery })

		s := newPhase28ReminderScene(t, nil)
		env := s.env

		th, err := advthresholds.Get(context.Background(), env.chat.Id)
		if err != nil {
			t.Fatalf("advthresholds.Get() error = %v", err)
		}
		if th.LockdownReminderMins != 0 {
			t.Fatalf("LockdownReminderMins = %d, want 0 (unset)", th.LockdownReminderMins)
		}

		ageAlert(t, s.row.ID, 21*time.Minute)
		env.cycle()

		s.wantPosted(2)
		rows := env.reminderRows()
		if len(rows) != 2 {
			t.Fatalf("reminder rows at 21m with swapped 20m seam = %d, want 2: %+v", len(rows), rows)
		}
		for _, r := range rows {
			if r.Seq != 1 || r.State != models.AlertStatePosted || r.Kind != models.AlertKindReminder {
				t.Errorf("reminder row = %+v, want posted reminder of seq 1", r)
			}
		}
	})

	t.Run("captcha_timeout_extended_30m_and_reset", func(t *testing.T) {
		phase28InProcessMiniredis(t)

		rawLocale, err := os.ReadFile("../../locales/en.yml")
		if err != nil {
			t.Fatalf("read locales/en.yml error = %v", err)
		}
		restoreI18n, err := i18n.OverrideManagerForTest(string(rawLocale))
		if err != nil {
			t.Fatalf("OverrideManagerForTest() error = %v", err)
		}
		t.Cleanup(restoreI18n)

		if db.DB != nil && db.DB.Config != nil {
			prevNowFunc := db.DB.Config.NowFunc
			db.DB.Config.NowFunc = func() time.Time { return time.Now().UTC() }
			t.Cleanup(func() { db.DB.Config.NowFunc = prevNowFunc })
		}

		chat := gotgbot.Chat{
			Id:    uniqueModuleChatID(),
			Type:  "supergroup",
			Title: "Phase28 Extended Captcha",
		}
		chatID := chat.Id
		t.Cleanup(func() {
			if db.DB == nil {
				return
			}
			_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.CaptchaAttempts{}).Error
			_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.CaptchaSettings{}).Error
			_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatAdvancedThresholds{}).Error
			_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
			dbcache.DeleteCache(dbcache.CacheKey("captcha_settings", chatID))
			dbcache.DeleteCache(dbcache.CacheKey("captcha_pending", chatID))
			dbcache.DeleteCache(advthresholds.CacheKey(chatID))
		})

		if err := captcha.SetCaptchaEnabled(chatID, true); err != nil {
			t.Fatalf("SetCaptchaEnabled() error = %v", err)
		}
		if err := captcha.SetCaptchaMode(chatID, "math"); err != nil {
			t.Fatalf("SetCaptchaMode(math) error = %v", err)
		}
		if err := captcha.SetCaptchaTimeout(chatID, 3); err != nil {
			t.Fatalf("SetCaptchaTimeout(3) error = %v", err)
		}
		if err := advthresholds.SetField(context.Background(), chatID, advthresholds.FieldCaptchaTimeout, 30, 777000); err != nil {
			t.Fatalf("advthresholds.SetField(captcha_timeout_mins=30) error = %v", err)
		}

		botClient := newModuleBotClient()
		bot := newModuleTestBot(botClient)
		admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

		joinCtx1 := newModuleMessageContext(bot, chat, admin, "join")
		before1 := time.Now().UTC()
		if err := SendCaptcha(bot, joinCtx1, 5001, "JoinerOne"); err != nil {
			t.Fatalf("SendCaptcha(JoinerOne) error = %v", err)
		}
		after1 := time.Now().UTC()

		attempt1, err := captcha.GetCaptchaAttemptIncludingExpired(5001, chatID)
		if err != nil {
			t.Fatalf("GetCaptchaAttemptIncludingExpired(5001) error = %v", err)
		}
		if attempt1 == nil {
			t.Fatal("captcha attempt for JoinerOne not found")
		}
		if attempt1.ExpiresAt.Before(before1.Add(29*time.Minute)) || attempt1.ExpiresAt.After(after1.Add(31*time.Minute)) {
			t.Fatalf("JoinerOne ExpiresAt = %s, want within [now+29m, now+31m] (before=%s, after=%s)",
				attempt1.ExpiresAt.Format(time.RFC3339), before1.Format(time.RFC3339), after1.Format(time.RFC3339))
		}

		var challengeText1 string
		if photos := botClient.callsFor("sendPhoto"); len(photos) > 0 {
			challengeText1 = fmt.Sprint(photos[len(photos)-1].Params["caption"])
		} else if msgs := botClient.callsFor("sendMessage"); len(msgs) > 0 {
			challengeText1 = fmt.Sprint(msgs[len(msgs)-1].Params["text"])
		} else {
			t.Fatal("SendCaptcha(JoinerOne) sent neither sendPhoto nor sendMessage")
		}
		if !strings.Contains(challengeText1, "30") {
			t.Fatalf("JoinerOne challenge text %q does not contain extended timeout 30", challengeText1)
		}

		// /captcha status must display the effective 30-minute timeout.
		msgCountBeforeStatus := len(botClient.callsFor("sendMessage"))
		statusCtx := newModuleMessageContext(bot, chat, admin, "/captcha")
		if err := captchaModule.captchaCommand(bot, statusCtx); err != nil {
			t.Fatalf("captchaCommand(/captcha) error = %v", err)
		}
		msgsAfterStatus := botClient.callsFor("sendMessage")
		if len(msgsAfterStatus) != msgCountBeforeStatus+1 {
			t.Fatalf("sendMessage count after /captcha = %d, want %d", len(msgsAfterStatus), msgCountBeforeStatus+1)
		}
		statusText := fmt.Sprint(msgsAfterStatus[len(msgsAfterStatus)-1].Params["text"])
		if !strings.Contains(statusText, "30") {
			t.Fatalf("/captcha status text %q does not show effective timeout 30", statusText)
		}

		// Reset captcha_timeout_mins back to 0 and verify the next joiner uses captcha_settings.timeout = 3.
		if err := advthresholds.ResetField(context.Background(), chatID, advthresholds.FieldCaptchaTimeout, 777000); err != nil {
			t.Fatalf("advthresholds.ResetField(captcha_timeout_mins) error = %v", err)
		}

		joinCtx2 := newModuleMessageContext(bot, chat, admin, "join")
		before2 := time.Now().UTC()
		if err := SendCaptcha(bot, joinCtx2, 5002, "JoinerTwo"); err != nil {
			t.Fatalf("SendCaptcha(JoinerTwo) error = %v", err)
		}
		after2 := time.Now().UTC()

		attempt2, err := captcha.GetCaptchaAttemptIncludingExpired(5002, chatID)
		if err != nil {
			t.Fatalf("GetCaptchaAttemptIncludingExpired(5002) error = %v", err)
		}
		if attempt2 == nil {
			t.Fatal("captcha attempt for JoinerTwo not found")
		}
		if attempt2.ExpiresAt.Before(before2.Add(2*time.Minute)) || attempt2.ExpiresAt.After(after2.Add(4*time.Minute)) {
			t.Fatalf("JoinerTwo ExpiresAt = %s, want within [now+2m, now+4m] (before=%s, after=%s)",
				attempt2.ExpiresAt.Format(time.RFC3339), before2.Format(time.RFC3339), after2.Format(time.RFC3339))
		}

		storedSettings, err := captcha.GetCaptchaSettings(chatID)
		if err != nil {
			t.Fatalf("GetCaptchaSettings() error = %v", err)
		}
		if storedSettings.Timeout != 3 {
			t.Fatalf("captcha_settings.timeout = %d, want 3 (must never be clobbered)", storedSettings.Timeout)
		}
	})

	t.Run("captcha_command_still_rejects_above_10", func(t *testing.T) {
		phase28InProcessMiniredis(t)

		chatID := uniqueModuleChatID()
		t.Cleanup(func() {
			if db.DB != nil {
				_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.CaptchaSettings{}).Error
			}
			dbcache.DeleteCache(dbcache.CacheKey("captcha_settings", chatID))
		})

		res := applyCaptchaTimeout(context.Background(), chatID, 11)
		if res.Outcome != captchaInvalid {
			t.Fatalf("applyCaptchaTimeout(11).Outcome = %v, want captchaInvalid (%v)", res.Outcome, captchaInvalid)
		}
		if res.Key != "captcha_timeout_invalid" {
			t.Fatalf("applyCaptchaTimeout(11).Key = %q, want %q", res.Key, "captcha_timeout_invalid")
		}
		if err := captcha.SetCaptchaTimeout(chatID, 11); !errors.Is(err, captcha.ErrInvalidTimeout) {
			t.Fatalf("captcha.SetCaptchaTimeout(11) err = %v, want ErrInvalidTimeout", err)
		}
	})
}
