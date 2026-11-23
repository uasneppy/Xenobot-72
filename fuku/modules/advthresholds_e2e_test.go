//go:build testtools

package modules

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/advthresholds"
	dbcache "github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/captcha"
	"github.com/uasneppy/fuku_robot/fuku/db/lockdown"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/db/raiddetection"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
	"github.com/uasneppy/fuku_robot/fuku/utils/cache"
)

// TestAdvancedThresholdsEndToEnd drives the complete Phase 28 user journey (TUNE-07)
// through the real dispatcher with phase28InProcessMiniredis (zero localhost TCP dials):
//   1. Owner changes flood window to 120 s via private /settings (av -> aa rw -> ao flw -> as flw=120);
//      flood messages from 3 recent joiners spanning 50 s trigger an automatic flood lockdown
//      with TriggerKind == flood and TriggerWindow == 120.
//   2. Owner changes lockdown reminder interval to 15 min via private callback (as lrm=15);
//      aging the active lockdown alert by 16 min and running the worker cycle posts reminder seq = 1
//      to both the Staff Group and the log channel.
//   3. Owner changes captcha timeout override to 30 min via private callback (as ctm=30);
//      SendCaptcha for a joiner stores ExpiresAt ~30 min out and challenge text contains "30".
//   4. Owner resets area rw (ax) and items lrm & ctm (ar) via private callbacks;
//      chat_advanced_thresholds returns to 0 (Default on Main), flood items spanning 50 s no longer
//      trigger, a new lockdown aged 16 min plans 0 reminders, and the next captcha joiner gets the
//      default 2-minute timeout.
func TestAdvancedThresholdsEndToEnd(t *testing.T) {
	ctx := context.Background()
	env := newPhase28SettingsEnv(t)
	withRealLocales(t)
	tr := i18n.MustNewTranslator("uk")
	wantStr := func(key string) string {
		t.Helper()
		s, err := tr.GetString(key)
		if err != nil || s == "" {
			t.Fatalf("missing uk locale key %q: %v", key, err)
		}
		return s
	}

	if db.DB != nil && db.DB.Config != nil {
		prevNowFunc := db.DB.Config.NowFunc
		db.DB.Config.NowFunc = func() time.Time { return time.Now().UTC() }
		t.Cleanup(func() { db.DB.Config.NowFunc = prevNowFunc })
	}

	staffChat := lockdownBurstEnv(env.lockdownEnv)
	logChannelID := uniqueModuleChatID()
	setStaffLogChannel(t, env.chat.Id, logChannelID)

	owner := env.newOwner(env.chat)
	joiner1ID := uniqueLinkOwnerID()
	joiner2ID := uniqueLinkOwnerID()
	t.Cleanup(func() {
		if db.DB == nil {
			return
		}
		_ = db.DB.Where("chat_id = ?", env.chat.Id).Delete(&models.CaptchaAttempts{}).Error
		_ = db.DB.Where("chat_id = ?", env.chat.Id).Delete(&models.CaptchaSettings{}).Error
		_ = db.DB.Where("chat_id = ?", env.chat.Id).Delete(&models.Chat{}).Error
		_ = db.DB.Where("user_id IN ?", []int64{joiner1ID, joiner2ID}).Delete(&models.User{}).Error
		dbcache.DeleteCache(dbcache.CacheKey("captcha_settings", env.chat.Id))
		dbcache.DeleteCache(dbcache.CacheKey("captcha_pending", env.chat.Id))
		dbcache.DeleteCache(dbcache.CacheKey("user", joiner1ID))
		dbcache.DeleteCache(dbcache.CacheKey("user", joiner2ID))
		dbcache.ResetLocalForTest()
	})

	privMsg := settingsPrivateOnlyMessage(t, env, owner)
	press := func(kv ...string) {
		t.Helper()
		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, kv...))
	}

	// -------------------------------------------------------------------------
	// 1. Flood window via UI: av -> aa rw -> ao flw -> as flw=120
	// -------------------------------------------------------------------------
	if err := raiddetection.SetFloodEnabled(env.chat.Id, true); err != nil {
		t.Fatalf("SetFloodEnabled: %v", err)
	}
	if err := raiddetection.SetFloodThreshold(env.chat.Id, 6); err != nil {
		t.Fatalf("SetFloodThreshold(6): %v", err)
	}

	press("a", settingsActOpen, "p", settingsPageAdvanced)
	press("a", settingsActAdvArea, "k", string(advthresholds.AreaRaidWindows))
	press("a", settingsActAdvItem, "t", string(advthresholds.FieldRaidFloodWindow))
	press("a", settingsActAdvSet, "t", string(advthresholds.FieldRaidFloodWindow), "v", "120")

	if text, _ := env.lastEdit(owner.Id, privMsg); !strings.Contains(text, wantStr("settings_adv_saved")) {
		t.Fatalf("step 1: saving flw=120 missing saved notice:\n%s", text)
	}
	if th := settingsAdvStored(t, env.chat.Id); th.RaidFloodWindowSecs != 120 {
		t.Fatalf("step 1: stored RaidFloodWindowSecs = %d, want 120", th.RaidFloodWindowSecs)
	}

	base := time.Date(2026, 11, 23, 15, 0, 0, 0, time.UTC)
	now := phase28SwapRaidNow(t, base)

	floodUsers := make([]gotgbot.User, 3)
	for i := range floodUsers {
		u := env.newJoiner(fmt.Sprintf("FloodJoiner%d", i+1))
		env.fake.setMember(env.chat.Id, u.Id, staffFakeMember{Status: gotgbot.ChatMemberStatusMember})
		phase28SeedJoinedAt(t, env.chat.Id, u.Id, base.Add(-1*time.Hour))
		floodUsers[i] = u
	}

	// Send 6 items spanning 50 s (3 at base-50s, 3 at base): outside default 30 s, inside tuned 120 s.
	*now = base.Add(-50 * time.Second)
	env.sendItems(floodUsers, 3)
	*now = base
	env.sendItems(floodUsers, 3)
	raidPosts.wait()

	active, err := lockdown.GetActiveFresh(env.chat.Id)
	if err != nil {
		t.Fatalf("step 1: lockdown.GetActiveFresh: %v", err)
	}
	if active == nil {
		t.Fatal("step 1: expected automatic flood lockdown with flw=120, got nil")
	}
	if active.TriggerKind != models.LockdownTriggerFlood || active.TriggerWindow != 120 {
		t.Fatalf("step 1: active lockdown = (kind=%s, window=%d), want (flood, 120)",
			active.TriggerKind, active.TriggerWindow)
	}

	// Let the lockdown alert worker post the initial alert (seq 0) to Staff Group and log channel.
	env.cycle()
	if got := len(env.fake.sentTo(staffChat.Id)); got != 1 {
		t.Fatalf("step 1: initial Staff Group alerts = %d, want 1", got)
	}
	if got := len(env.fake.sentTo(logChannelID)); got != 1 {
		t.Fatalf("step 1: initial log channel alerts = %d, want 1", got)
	}

	// -------------------------------------------------------------------------
	// 2. Lockdown reminder interval via UI: av -> aa lr -> ao lrm -> as lrm=15
	// -------------------------------------------------------------------------
	press("a", settingsActOpen, "p", settingsPageAdvanced)
	press("a", settingsActAdvArea, "k", string(advthresholds.AreaLockdownReminder))
	press("a", settingsActAdvItem, "t", string(advthresholds.FieldLockdownReminder))
	press("a", settingsActAdvSet, "t", string(advthresholds.FieldLockdownReminder), "v", "15")

	if th := settingsAdvStored(t, env.chat.Id); th.LockdownReminderMins != 15 {
		t.Fatalf("step 2: stored LockdownReminderMins = %d, want 15", th.LockdownReminderMins)
	}

	ageAlert(t, active.ID, 16*time.Minute)
	env.cycle()

	staffMsgs := env.fake.sentTo(staffChat.Id)
	logMsgs := env.fake.sentTo(logChannelID)
	if len(staffMsgs) != 2 || len(logMsgs) != 2 {
		t.Fatalf("step 2: alerts posted staff=%d log=%d, want 2 each", len(staffMsgs), len(logMsgs))
	}
	wantReminderPrefix, _, _ := strings.Cut(wantStr("lockdown_reminder_title"), "{group}")
	for dest, msg := range map[string]staffSentMessage{"staff": staffMsgs[1], "log": logMsgs[1]} {
		text := fmt.Sprint(msg.Params["text"])
		if !strings.Contains(text, wantReminderPrefix) || !strings.Contains(text, "120") {
			t.Errorf("step 2: %s reminder missing reminder header %q or window 120: %q", dest, wantReminderPrefix, text)
		}
	}
	remRows := env.reminderRows()
	if len(remRows) != 2 {
		t.Fatalf("step 2: reminder rows = %d, want 2: %+v", len(remRows), remRows)
	}
	for _, r := range remRows {
		if r.LockdownID != active.ID || r.Seq != 1 || r.Kind != models.AlertKindReminder || r.State != models.AlertStatePosted {
			t.Errorf("step 2: reminder row = %+v, want posted reminder seq=1 for lockdown %d", r, active.ID)
		}
	}

	// -------------------------------------------------------------------------
	// 3. Extended captcha timeout via UI: av -> aa ct -> ao ctm -> as ctm=30
	// -------------------------------------------------------------------------
	if err := captcha.SetCaptchaEnabled(env.chat.Id, true); err != nil {
		t.Fatalf("step 3: SetCaptchaEnabled: %v", err)
	}
	if err := captcha.SetCaptchaMode(env.chat.Id, "math"); err != nil {
		t.Fatalf("step 3: SetCaptchaMode(math): %v", err)
	}

	press("a", settingsActOpen, "p", settingsPageAdvanced)
	press("a", settingsActAdvArea, "k", string(advthresholds.AreaCaptchaTimeout))
	press("a", settingsActAdvItem, "t", string(advthresholds.FieldCaptchaTimeout))
	press("a", settingsActAdvSet, "t", string(advthresholds.FieldCaptchaTimeout), "v", "30")

	if th := settingsAdvStored(t, env.chat.Id); th.CaptchaTimeoutMins != 30 {
		t.Fatalf("step 3: stored CaptchaTimeoutMins = %d, want 30", th.CaptchaTimeoutMins)
	}

	extractChallengeText := func(photosBefore, msgsBefore int) string {
		t.Helper()
		photos := env.fake.callsFor("sendPhoto")
		if len(photos) > photosBefore {
			return fmt.Sprint(photos[len(photos)-1].Params["caption"])
		}
		msgs := callsToChat(env.fake.staffBotClient, "sendMessage", env.chat.Id)
		if len(msgs) > msgsBefore {
			return fmt.Sprint(msgs[len(msgs)-1].Params["text"])
		}
		t.Fatal("SendCaptcha sent neither sendPhoto nor sendMessage")
		return ""
	}

	photosBefore := len(env.fake.callsFor("sendPhoto"))
	msgsBefore := len(callsToChat(env.fake.staffBotClient, "sendMessage", env.chat.Id))
	joinCtx1 := newModuleMessageContext(env.bot, env.chat, owner, "join1")
	before1 := time.Now().UTC()
	if err := SendCaptcha(env.bot, joinCtx1, joiner1ID, "JoinerOne"); err != nil {
		t.Fatalf("step 3: SendCaptcha(JoinerOne): %v", err)
	}
	after1 := time.Now().UTC()

	attempt1, err := captcha.GetCaptchaAttemptIncludingExpired(joiner1ID, env.chat.Id)
	if err != nil || attempt1 == nil {
		t.Fatalf("step 3: GetCaptchaAttemptIncludingExpired(joiner1) = (%+v, %v), want attempt", attempt1, err)
	}
	if attempt1.ExpiresAt.Before(before1.Add(29*time.Minute)) || attempt1.ExpiresAt.After(after1.Add(31*time.Minute)) {
		t.Fatalf("step 3: JoinerOne ExpiresAt = %s, want within [now+29m, now+31m]", attempt1.ExpiresAt.Format(time.RFC3339))
	}
	challenge1 := extractChallengeText(photosBefore, msgsBefore)
	if !strings.Contains(challenge1, "<b>30") {
		t.Fatalf("step 3: JoinerOne challenge text %q does not contain extended timeout 30", challenge1)
	}

	// -------------------------------------------------------------------------
	// 4. Reset via UI (ax on rw, ar on lrm and ctm) restores default behavior
	// -------------------------------------------------------------------------
	press("a", settingsActAdvResetArea, "k", string(advthresholds.AreaRaidWindows))
	if text, _ := env.lastEdit(owner.Id, privMsg); !strings.Contains(text, wantStr("settings_adv_reset_area_done")) {
		t.Fatalf("step 4: resetting area rw missing done notice:\n%s", text)
	}
	press("a", settingsActAdvReset, "t", string(advthresholds.FieldLockdownReminder))
	if text, _ := env.lastEdit(owner.Id, privMsg); !strings.Contains(text, wantStr("settings_adv_reset_item_done")) {
		t.Fatalf("step 4: resetting lrm missing done notice:\n%s", text)
	}
	press("a", settingsActAdvReset, "t", string(advthresholds.FieldCaptchaTimeout))
	if text, _ := env.lastEdit(owner.Id, privMsg); !strings.Contains(text, wantStr("settings_adv_reset_item_done")) {
		t.Fatalf("step 4: resetting ctm missing done notice:\n%s", text)
	}

	// Verify DB row returned to all 0 and Main reports Default.
	if th := settingsAdvStored(t, env.chat.Id); th.CustomizedCount() != 0 {
		t.Fatalf("step 4: stored thresholds after reset = %+v (customized=%d), want all 0", th, th.CustomizedCount())
	}
	press("a", settingsActOpen, "p", settingsPageAdvanced)
	press("a", settingsActBack)
	mainText, _ := env.lastEdit(owner.Id, privMsg)
	wantMainLine, _ := tr.GetString("settings_main_state_line", i18n.TranslationParams{
		"icon":  "🎛",
		"page":  wantStr("settings_page_advanced"),
		"state": wantStr("settings_state_default"),
	})
	if !strings.Contains(mainText, wantMainLine) {
		t.Fatalf("step 4: Main screen missing %q:\n%s", wantMainLine, mainText)
	}

	// 4a. Lift active lockdown, clear Redis flood counters/latch, and send 6 items spanning 50 s:
	//     with default 30 s flood window restored, the first 3 items age out and flood does NOT fire.
	env.liftUntilDone(active)
	env.dropRaidLatch()
	redisClient := cache.GetRedisClient()
	for _, part := range []string{raidPartFloodItems, raidPartFloodNew, raidPartFloodAccounts} {
		if err := redisClient.Del(ctx, raidKey(env.chat.Id, part)).Err(); err != nil {
			t.Fatalf("step 4a: clear %s: %v", part, err)
		}
	}

	base2 := base.Add(10 * time.Minute)
	*now = base2.Add(-50 * time.Second)
	env.sendItems(floodUsers, 3)
	*now = base2
	env.sendItems(floodUsers, 3)
	raidPosts.wait()

	if postResetActive, err := lockdown.GetActiveFresh(env.chat.Id); err != nil {
		t.Fatalf("step 4a: lockdown.GetActiveFresh: %v", err)
	} else if postResetActive != nil {
		t.Fatalf("step 4a: flood spanning 50 s unexpectedly fired after reset: %+v", postResetActive)
	}

	// 4b. Start a new lockdown, age it 16 min, run worker cycle -> 0 reminders planned (60 min default).
	row2 := env.lockAsAdmin("after_reset")
	env.cycle()
	ageAlert(t, row2.ID, 16*time.Minute)
	env.cycle()

	var row2Reminders int64
	if err := db.DB.Model(&models.ChatLockdownAlert{}).
		Where("chat_id = ? AND lockdown_id = ? AND kind = ?", env.chat.Id, row2.ID, models.AlertKindReminder).
		Count(&row2Reminders).Error; err != nil {
		t.Fatalf("step 4b: count reminders for row2: %v", err)
	}
	if row2Reminders != 0 {
		t.Fatalf("step 4b: reminders planned at 16m after reset = %d, want 0 (default 60m)", row2Reminders)
	}

	// 4c. Next captcha joiner receives the default 2-minute timeout.
	photosBefore = len(env.fake.callsFor("sendPhoto"))
	msgsBefore = len(callsToChat(env.fake.staffBotClient, "sendMessage", env.chat.Id))
	joinCtx2 := newModuleMessageContext(env.bot, env.chat, owner, "join2")
	before2 := time.Now().UTC()
	if err := SendCaptcha(env.bot, joinCtx2, joiner2ID, "JoinerTwo"); err != nil {
		t.Fatalf("step 4c: SendCaptcha(JoinerTwo): %v", err)
	}
	after2 := time.Now().UTC()

	attempt2, err := captcha.GetCaptchaAttemptIncludingExpired(joiner2ID, env.chat.Id)
	if err != nil || attempt2 == nil {
		t.Fatalf("step 4c: GetCaptchaAttemptIncludingExpired(joiner2) = (%+v, %v), want attempt", attempt2, err)
	}
	if attempt2.ExpiresAt.Before(before2.Add(1*time.Minute)) || attempt2.ExpiresAt.After(after2.Add(3*time.Minute)) {
		t.Fatalf("step 4c: JoinerTwo ExpiresAt = %s, want within [now+1m, now+3m] (default 2m)", attempt2.ExpiresAt.Format(time.RFC3339))
	}
	challenge2 := extractChallengeText(photosBefore, msgsBefore)
	if !strings.Contains(challenge2, "<b>2") || strings.Contains(challenge2, "<b>30") {
		t.Fatalf("step 4c: JoinerTwo challenge text %q want default timeout 2 and not 30", challenge2)
	}
}
