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
	"github.com/uasneppy/fuku_robot/fuku/db/captcha"
	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/utils/actionlog"
)

// The two sentinels every stored lockdown message carries: its text starts with lockdownSurfaceSentinel and
// its one URL button is labelled lockdownSurfaceLabel. No default text, alert, report or notice contains
// either, so finding one in a request means a stored lockdown message reached that request.
const (
	lockdownSurfaceSentinel = "ZZLOCKSURFACE"
	lockdownSurfaceLabel    = "ZZLOCKSURFACEBUTTON"
	lockdownSurfaceURL      = "https://example.com/zzlocksurface"
)

// lockdownSurfaceEvents are the five lockdown events.
var lockdownSurfaceEvents = []string{
	eventmsg.EventLockdown, eventmsg.EventLockJoinSurge, eventmsg.EventLockFlood,
	eventmsg.EventLockAIBurst, eventmsg.EventLockLift,
}

// lockdownSurfaceText is the stored group text of one lockdown event: the sentinel and the event, so a test
// can tell which event message a send carried.
func lockdownSurfaceText(event string) string {
	return lockdownSurfaceSentinel + "-" + event
}

// lockdownStoreEverySurfaceMessage stores, for every chat, each of the five lockdown events with the sentinel
// text and the sentinel button (Silent off), and a private copy row with the sentinel text switched on, and
// turns both private copy options (the Details button and the log line) on. The rows, their cached reads and
// every private copy key are removed when the test ends.
func lockdownStoreEverySurfaceMessage(t *testing.T, chatIDs ...int64) {
	t.Helper()
	eventPMCleanup(t, chatIDs...)
	buttons := []models.Button{{Name: lockdownSurfaceLabel, Url: lockdownSurfaceURL}}
	for _, chatID := range chatIDs {
		for _, event := range lockdownSurfaceEvents {
			if err := eventmsg.SetText(t.Context(), chatID, event, eventmsg.DestGroup, lockdownSurfaceText(event), 1); err != nil {
				t.Fatalf("SetText(%d, %s) error = %v", chatID, event, err)
			}
			if err := eventmsg.SetButtons(t.Context(), chatID, event, eventmsg.DestGroup, buttons, 1); err != nil {
				t.Fatalf("SetButtons(%d, %s) error = %v", chatID, event, err)
			}
			storeEventPM(t, chatID, event, lockdownSurfaceText(event)+"-pm", true)
		}
		if err := eventmsg.SetPMDetails(t.Context(), chatID, true, 1); err != nil {
			t.Fatalf("SetPMDetails(%d) error = %v", chatID, err)
		}
		if err := eventmsg.SetPMLog(t.Context(), chatID, true, 1); err != nil {
			t.Fatalf("SetPMLog(%d) error = %v", chatID, err)
		}
	}
}

// lockdownSurfaceCalls is every request the client recorded, in order, read once the raid posts and the
// private copy jobs are done.
func lockdownSurfaceCalls(client *moduleBotClient) []moduleBotCall {
	client.callsFor("") // waits for the raid AI jobs, the starts they asked for and the raid posts
	pmCopyJobs.wait()
	client.mu.Lock()
	defer client.mu.Unlock()
	return append([]moduleBotCall(nil), client.calls...)
}

// lockdownSurfaceRequest is one recorded request as text: its method and every parameter as lockdownParamText
// renders it (raw JSON and bytes as they are), so a sentinel inside a keyboard or a caption is found.
func lockdownSurfaceRequest(call moduleBotCall) string {
	var b strings.Builder
	b.WriteString(call.Method)
	for key, value := range call.Params {
		b.WriteString(" " + key + "=" + lockdownParamText(value))
	}
	return b.String()
}

// lockdownSurfaceHasSentinel reports whether the request carries either sentinel.
func lockdownSurfaceHasSentinel(call moduleBotCall) bool {
	request := lockdownSurfaceRequest(call)
	return strings.Contains(request, lockdownSurfaceSentinel) || strings.Contains(request, lockdownSurfaceLabel)
}

// lockdownWantNoSentinel fails for every request of calls that carries either sentinel.
func lockdownWantNoSentinel(t *testing.T, calls []moduleBotCall, what string) {
	t.Helper()
	for _, call := range calls {
		if lockdownSurfaceHasSentinel(call) {
			t.Errorf("%s: %s carries a stored lockdown message: %s", what, call.Method, lockdownSurfaceRequest(call))
		}
	}
}

// lockdownCallsTo is the requests of calls addressed to chatID.
func lockdownCallsTo(calls []moduleBotCall, chatID int64) []moduleBotCall {
	var matched []moduleBotCall
	for _, call := range calls {
		if _, ok := call.Params["chat_id"]; ok && staffParamInt(call.Params, "chat_id") == chatID {
			matched = append(matched, call)
		}
	}
	return matched
}

// lockdownWantNoPrivateSend fails for every request the client made to a private chat (a positive chat ID):
// no send, edit or copy, whatever its method.
func lockdownWantNoPrivateSend(t *testing.T, client *moduleBotClient, what string) {
	t.Helper()
	for _, call := range lockdownSurfaceCalls(client) {
		for _, key := range []string{"chat_id", "from_chat_id"} {
			if _, ok := call.Params[key]; ok && staffParamInt(call.Params, key) > 0 {
				t.Errorf("%s: %s went to the private chat %v, want nothing in private", what, call.Method, call.Params[key])
			}
		}
	}
}

// lockdownWantNoPMCopyTrace fails when a request carries a Details button or a #PM_COPY line, or when the
// client edited a keyboard (the Details button is added by an edit of the group message's keyboard).
func lockdownWantNoPMCopyTrace(t *testing.T, client *moduleBotClient, what string) {
	t.Helper()
	for _, call := range lockdownSurfaceCalls(client) {
		request := lockdownSurfaceRequest(call)
		if call.Method == "editMessageReplyMarkup" || strings.Contains(request, "pmcopy_") || strings.Contains(request, "#PM_COPY") {
			t.Errorf("%s: %s carries a private copy's trace: %s", what, call.Method, request)
		}
	}
}

// lockdownSurfaceTexts is the text of every message sent to chatID whose text carries the sentinel.
func lockdownSurfaceTexts(env *lockdownEnv, chatID int64) []string {
	var texts []string
	for _, sent := range env.fake.sentTo(chatID) {
		if text := fmt.Sprint(sent.Params["text"]); strings.Contains(text, lockdownSurfaceSentinel) {
			texts = append(texts, text)
		}
	}
	return texts
}

// lockdownWantSurfaceMessage fails unless text is the stored message of event, alone or followed by the
// technical notes on new lines.
func lockdownWantSurfaceMessage(t *testing.T, what, text, event string) {
	t.Helper()
	stored := lockdownSurfaceText(event)
	if text != stored && !strings.HasPrefix(text, stored+"\n") {
		t.Errorf("%s = %q, want the stored %s message %q", what, text, event, stored)
	}
}

// TestLockdownMessagesLeaveOtherSurfacesAlone is the behavioural half of EVT-10 (D-05, D-34); the structural
// half is TestEventEngineCallers, which pins every caller of the event message engine. With every lockdown
// message stored for the locked group, its Staff Group and its log channel, the alert's copies, a reminder,
// the alert's close at the lift, a "Ban N recent joiners" run with its card report and its #LOCKDOWN_BAN post,
// the lift tally, the cannot-lock warning, a Staff Group action's card and summary, a captcha challenge and a
// log channel post carry no sentinel, and the sentinel reaches the group only in the two event messages the
// flow posts (the start reply and the lift notice). Each case first proves it reached its surface.
func TestLockdownMessagesLeaveOtherSurfacesAlone(t *testing.T) {
	t.Run("lockdown flow", func(t *testing.T) {
		env, staffChat := banEnv(t)
		logChannel := uniqueModuleChatID()
		setStaffLogChannel(t, env.chat.Id, logChannel)
		lockdownStoreEverySurfaceMessage(t, env.chat.Id, staffChat.Id, logChannel)
		t.Cleanup(pmCopyJobs.wait)

		before := env.newJoiner("Before")
		env.join(before, before, "")
		row := env.lockAsAdmin("raid wave")
		time.Sleep(3 * time.Millisecond)
		raider := env.newJoiner("Raider")
		env.join(raider, raider, "")

		// The alert reaches the Staff Group and the log channel.
		env.cycle()
		if got := len(env.fake.sentTo(staffChat.Id)); got != 1 {
			t.Fatalf("%d messages reached the Staff Group, want the alert", got)
		}
		if got := len(env.fake.sentTo(logChannel)); got != 1 {
			t.Fatalf("%d messages reached the log channel, want the alert", got)
		}

		// A reminder is due and posted to both.
		ageAlert(t, row.ID, 61*time.Minute)
		env.cycle()
		if rows := env.reminderRows(); len(rows) != 2 {
			t.Fatalf("reminder rows = %+v, want one per destination", rows)
		}
		if got := len(env.fake.sentTo(staffChat.Id)); got != 2 {
			t.Fatalf("%d messages reached the Staff Group, want the alert and the reminder", got)
		}

		// "Ban N recent joiners": the card's report and the #LOCKDOWN_BAN post.
		a := lastAlert(t, env, staffChat)
		card := env.confirmBan(a, env.admin)
		env.cyclesUntilReported(8)
		if text := env.lastCardText(staffChat.Id, card.msg.MessageID); !strings.Contains(text, staffMarker("lockdown_ban_result")) {
			t.Fatalf("Ban card = %q, want the run's report", text)
		}
		if got := len(env.logBanPosts(logChannel)); got != 1 {
			t.Fatalf("%d #LOCKDOWN_BAN posts in the log channel, want 1", got)
		}

		// The alert's Lift and its Confirm, then the cycles that close the copies.
		pressLockdownButton(env, env.admin, staffChat, a.alert.MessageID, lockdownButtonData(t, a.alert.Params["reply_markup"], "lk"))
		posted := env.fake.sentTo(staffChat.Id)
		liftCard := posted[len(posted)-1]
		pressLockdownButton(env, env.admin, staffChat, liftCard.MessageID, lockdownButtonData(t, liftCard.Params["reply_markup"], "lc"))
		env.cycleUntilIdle()
		lockdownLiftWantLifted(t, env, row, env.admin.Id)

		var alerts int
		for _, alertRow := range env.alertRows() {
			if alertRow.Kind != models.AlertKindAlert {
				continue
			}
			alerts++
			if alertRow.ClosedAt == nil {
				t.Errorf("alert copy to %d = %+v, want it closed at the lift", alertRow.DestChatID, alertRow)
			}
		}
		if alerts != 2 {
			t.Fatalf("%d alert copies, want one in the Staff Group and one in the log channel", alerts)
		}
		edits := env.alertEdits(staffChat.Id, a.alert.MessageID)
		if len(edits) == 0 || !strings.Contains(fmt.Sprint(edits[len(edits)-1].Params["text"]), staffMarker("lockdown_alert_lifted_footer")) {
			t.Fatalf("alert edits = %d, want the last one with %s", len(edits), staffMarker("lockdown_alert_lifted_footer"))
		}
		tally := false
		for _, text := range env.textsTo(env.chat.Id) {
			if strings.Contains(text, staffMarker("lockdown_lift_tally")) {
				tally = true
			}
		}
		if !tally {
			t.Fatalf("group messages = %q, want the lift tally", env.textsTo(env.chat.Id))
		}

		calls := lockdownSurfaceCalls(env.fake.moduleBotClient)
		lockdownWantNoSentinel(t, lockdownCallsTo(calls, staffChat.Id), "the Staff Group")
		lockdownWantNoSentinel(t, lockdownCallsTo(calls, logChannel), "the log channel")
		texts := lockdownSurfaceTexts(env, env.chat.Id)
		if len(texts) != 2 {
			t.Fatalf("messages carrying a stored lockdown message in the group = %q, want the start reply and the lift notice", texts)
		}
		lockdownWantSurfaceMessage(t, "the start reply", texts[0], eventmsg.EventLockdown)
		lockdownWantSurfaceMessage(t, "the lift notice", texts[1], eventmsg.EventLockLift)
		var outside []moduleBotCall
		for _, call := range lockdownCallsTo(calls, env.chat.Id) {
			if !strings.HasPrefix(call.Method, "send") || !strings.Contains(fmt.Sprint(call.Params["text"]), lockdownSurfaceSentinel) {
				outside = append(outside, call)
			}
		}
		lockdownWantNoSentinel(t, outside, "the group's other requests (the tally, the bans and the edits)")
		lockdownWantNoPrivateSend(t, env.fake.moduleBotClient, "the lockdown flow")
	})

	t.Run("cannot lock", func(t *testing.T) {
		env, staffChat, channel := warnEnv(t)
		env.fake.setBotRole(env.chat.Id, staffRoleAdminNoRestrict)
		lockdownStoreEverySurfaceMessage(t, env.chat.Id, staffChat, channel)

		raidFire(env.bot, &env.chat, raidTrigger{Kind: models.LockdownTriggerJoinSurge, Count: 15, Window: 60})

		if row := env.activeLockdown(); row != nil {
			t.Fatalf("a lockdown exists although the bot cannot restrict members: %+v", row)
		}
		env.wantWarnings(staffChat, channel, 1)
		if !strings.Contains(env.textsTo(staffChat)[0], staffMarker("lockdown_auto_cant_lock_rights")) {
			t.Fatalf("Staff Group warning = %q, want the cannot-lock warning", env.textsTo(staffChat)[0])
		}
		if got := len(env.replies()); got != 0 {
			t.Fatalf("%d messages reached the group itself, want none", got)
		}
		lockdownWantNoSentinel(t, lockdownSurfaceCalls(env.fake.moduleBotClient), "the cannot-lock warning")
		lockdownWantNoPrivateSend(t, env.fake.moduleBotClient, "the cannot-lock warning")
	})

	t.Run("staff action", func(t *testing.T) {
		env := newStaffActionEnv(t, 2)
		lockdownStoreEverySurfaceMessage(t, append([]int64{env.staffChat}, env.groups...)...)
		for _, group := range env.groups {
			env.fake.setMember(group, staffTestTarget, staffFakeMember{Status: gotgbot.ChatMemberStatusMember})
		}

		summary, msgID := env.runStaffCommand("/mute 4242 flooding")
		if summary == "" || msgID == 0 || len(env.fake.sentTo(env.staffChat)) == 0 {
			t.Fatalf("the staff action produced no card or summary (%q, %d), so the case proves nothing", summary, msgID)
		}
		for _, group := range env.groups {
			if len(env.callsTo("restrictChatMember", group)) != 1 {
				t.Fatalf("group %d was not muted, so the case proves nothing", group)
			}
		}
		lockdownWantNoSentinel(t, lockdownSurfaceCalls(env.fake.moduleBotClient), "the staff action")
		lockdownWantNoPrivateSend(t, env.fake.moduleBotClient, "the staff action")
	})

	t.Run("captcha challenge", func(t *testing.T) {
		withMiniredis(t)
		s := newBotSite(t, "supergroup")
		lockdownStoreEverySurfaceMessage(t, s.chat.Id)
		t.Cleanup(func() {
			db.DB.Where("chat_id = ?", s.chat.Id).Delete(&models.CaptchaAttempts{})
			db.DB.Where("chat_id = ?", s.chat.Id).Delete(&models.CaptchaSettings{})
		})
		if err := captcha.SetCaptchaEnabled(s.chat.Id, true); err != nil {
			t.Fatalf("SetCaptchaEnabled() error = %v", err)
		}
		ctx := newModuleMessageContext(s.bot, s.chat, gotgbot.User{Id: 777000, FirstName: "Telegram"}, "join")
		ctx.EffectiveMessage.Date = s.now.Unix()

		if err := SendCaptcha(s.bot, ctx, uniqueLinkOwnerID(), "Member"); err != nil {
			t.Fatalf("SendCaptcha() error = %v", err)
		}
		if len(s.client.callsFor("sendPhoto"))+len(s.client.callsFor("sendMessage")) == 0 {
			t.Fatal("no challenge was sent, so the case proves nothing")
		}
		lockdownWantNoSentinel(t, lockdownSurfaceCalls(s.client), "the captcha challenge")
	})

	t.Run("log post", func(t *testing.T) {
		withMiniredis(t)
		s := newBotSite(t, "supergroup")
		channelID := uniqueModuleChatID()
		setStaffLogChannel(t, s.chat.Id, channelID)
		lockdownStoreEverySurfaceMessage(t, s.chat.Id, channelID)

		actionlog.Admin(s.bot, &s.chat, &gotgbot.User{Id: 55, FirstName: "Actor"}, "BAN", 42, "why")

		calls := lockdownSurfaceCalls(s.client)
		if got := len(lockdownCallsTo(calls, channelID)); got != 1 {
			t.Fatalf("requests to the log channel = %d, want the one post", got)
		}
		lockdownWantNoSentinel(t, calls, "the log post")
	})
}

// lockdownNoCopyEnv is a lockdown environment for the private copy cases: every lockdown message stored for
// its chat, a private copy row of each switched on with both options on, a log channel taking the admin
// category, a fast private copy pacer and the private copy jobs awaited when the test ends.
func lockdownNoCopyEnv(t *testing.T) *lockdownEnv {
	t.Helper()
	env := newLockdownEnv(t)
	withFastPMCopyPacer(t)
	raidCleanup(t, env.chat.Id)
	setStaffLogChannel(t, env.chat.Id, uniqueModuleChatID())
	lockdownStoreEverySurfaceMessage(t, env.chat.Id)
	env.fake.setBotMember(env.chat.Id, lockdownBotAdminJSON(true, true, true))
	t.Cleanup(pmCopyJobs.wait)
	return env
}

// lockdownWantNoCopy fails unless the group received a message carrying the stored message of event (the
// site was reached) and nothing was sent, edited or copied in private, with no Details button and no
// #PM_COPY line.
func lockdownWantNoCopy(t *testing.T, env *lockdownEnv, event, what string) {
	t.Helper()
	waitEventPM(t)
	found := false
	for _, text := range lockdownSurfaceTexts(env, env.chat.Id) {
		if text == lockdownSurfaceText(event) || strings.HasPrefix(text, lockdownSurfaceText(event)+"\n") {
			found = true
		}
	}
	if !found {
		t.Fatalf("%s: the group received %q, want the stored %s message, so the case proves nothing",
			what, lockdownSurfaceTexts(env, env.chat.Id), event)
	}
	lockdownWantNoPrivateSend(t, env.fake.moduleBotClient, what)
	lockdownWantNoPMCopyTrace(t, env.fake.moduleBotClient, what)
}

// TestLockdownEventsSendNoPrivateCopy proves D-04 and D-35 (PMSG-05): no lockdown event has a private copy
// spec, and with a private copy row of each lockdown event stored and switched on (both options on), a manual
// start, an automatic start and a lift by /unlockdown and by the alert's button post their group message and
// send nothing to any private chat, add no Details button and post no #PM_COPY line.
func TestLockdownEventsSendNoPrivateCopy(t *testing.T) {
	t.Run("no spec", func(t *testing.T) {
		env := lockdownNoCopyEnv(t)
		storeEventPM(t, env.chat.Id, eventmsg.EventBan, "control", true)
		msgs, err := eventmsg.GetFresh(context.Background(), env.chat.Id)
		if err != nil {
			t.Fatalf("GetFresh error = %v", err)
		}
		site := eventSite{Bot: env.bot, Chat: &env.chat}
		target := gotgbot.User{Id: env.admin.Id, FirstName: env.admin.FirstName}
		if !eventPMWouldOffer(site, eventReply{Event: eventmsg.EventBan, Target: target}, msgs) {
			t.Fatal("setup: a ban with its private copy on would offer no copy, so the case proves nothing")
		}
		for _, event := range lockdownSurfaceEvents {
			if spec, ok := eventPMSpecFor(event); ok {
				t.Errorf("spec of %s = %+v, want none", event, spec)
			}
			if msgs.Resolve(event, eventmsg.DestPM).Silent {
				t.Fatalf("setup: the private copy row of %s is not switched on", event)
			}
			if eventPMWouldOffer(site, eventReply{Event: event, Target: target}, msgs) {
				t.Errorf("a %s reply would offer a private copy, want none", event)
			}
		}
	})

	t.Run("manual start", func(t *testing.T) {
		env := lockdownNoCopyEnv(t)

		env.send(env.admin, "/lockdown raid")

		lockdownEventWantStarted(t, env, env.admin.Id)
		lockdownWantNoCopy(t, env, eventmsg.EventLockdown, "the manual start")
	})

	t.Run("automatic start", func(t *testing.T) {
		env := lockdownNoCopyEnv(t)

		raidFire(env.bot, &env.chat, raidTrigger{Kind: models.LockdownTriggerFlood, Count: 30, Window: 30, Percent: 80})
		raidPosts.wait()

		lockdownEventWantStarted(t, env, env.bot.Id)
		lockdownWantNoCopy(t, env, eventmsg.EventLockFlood, "the automatic start")
	})

	t.Run("lift by command", func(t *testing.T) {
		env := lockdownNoCopyEnv(t)
		row := env.lockAsAdmin("raid")

		env.send(env.admin, "/unlockdown")

		lockdownLiftWantLifted(t, env, row, env.admin.Id)
		lockdownWantNoCopy(t, env, eventmsg.EventLockLift, "the lift by command")
	})

	t.Run("lift by button", func(t *testing.T) {
		env := lockdownNoCopyEnv(t)
		card := askToLiftLockdown(t, env)

		pressLockdownButton(env, env.admin, card.staffChat, card.msgID, card.confirm)

		lockdownLiftWantLifted(t, env, card.row, env.admin.Id)
		lockdownWantNoCopy(t, env, eventmsg.EventLockLift, "the lift by the alert's button")
	})
}
