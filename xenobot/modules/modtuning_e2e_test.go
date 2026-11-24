//go:build testtools

package modules

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	dbcache "github.com/uasneppy/Xenobot-72/xenobot/db/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/db/logchannels"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/db/modtuning"
	"github.com/uasneppy/Xenobot-72/xenobot/db/warns"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
)

// TestManualDefaultsEndToEnd drives the complete Phase 27 user journey (TUNE-04, TUNE-05, TUNE-06)
// through the real dispatcher across private /settings (pages q and rs) and group moderation commands
// (/ban, /warn, /tban) with PM copy and log channel enabled, then cleans up and verifies byte-identical
// default permanent ban behavior after clearing all settings.
func TestManualDefaultsEndToEnd(t *testing.T) {
	ctx := context.Background()
	fixedNow := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	withFrozenPunishNow(t, fixedNow)

	e := newPMCopiesE2E(t, true)
	LoadWarns(e.dispatcher)
	cleanupModTuningChat(t, e.chat.Id)

	// Enable a log channel for the supergroup.
	logChannelID := -1000000000000 - uniqueLinkOwnerID()
	if err := logchannels.Set(e.chat.Id, e.chat.Title, logChannelID); err != nil {
		t.Fatalf("logchannels.Set: %v", err)
	}
	t.Cleanup(func() {
		_ = logchannels.Unset(e.chat.Id)
		db.DB.Where("chat_id = ?", e.chat.Id).Delete(&models.Warns{})
		dbcache.ResetLocalForTest()
	})

	// Enable PM copy for both EventTimedBan ("tb") and EventBan ("bn") via the real settings buttons.
	e.switchPM("tb", true)
	e.switchPM("bn", true)

	tr, err := i18n.NewTranslator("uk")
	if err != nil {
		t.Fatalf("i18n.NewTranslator(uk): %v", err)
	}
	wantReasonRefusal := trS(tr, "moddef_reason_required")
	wantAppealLabel := trS(tr, "eventmsg_btn_appeal")
	if wantReasonRefusal == "" || wantAppealLabel == "" {
		t.Fatalf("missing uk translations: reason_refusal=%q appeal_label=%q", wantReasonRefusal, wantAppealLabel)
	}

	const (
		appealURL      = "https://example.com/appeal"
		shortcutWord   = "spam"
		shortcutReason = "Repeated <promo> & links"
		escapedReason  = "Repeated &lt;promo&gt; &amp; links"
	)

	newTarget := func(first string) gotgbot.User {
		t.Helper()
		u := gotgbot.User{Id: uniqueLinkOwnerID(), FirstName: first, LastName: "User"}
		e.client.register(u)
		e.fake.setProfile(u.Id, u.FirstName, u.LastName, "")
		e.fake.setMember(e.chat.Id, u.Id, staffFakeMember{Status: gotgbot.ChatMemberStatusMember})
		memberReqCleanup(t, nil, []int64{u.Id})
		t.Cleanup(func() {
			db.DB.Where("user_id = ? AND chat_id = ?", u.Id, e.chat.Id).Delete(&models.Warns{})
			dbcache.DeleteCache(dbcache.CacheKey("warns", u.Id, e.chat.Id))
		})
		return u
	}

	sendGroupReply := func(from, repliedTo gotgbot.User, text string) {
		t.Helper()
		replied := &gotgbot.Message{
			MessageId: e.updateID(),
			Chat:      e.chat,
			From:      &repliedTo,
			Text:      "member message",
		}
		msg := gotgbot.Message{
			MessageId:      e.updateID(),
			Chat:           e.chat,
			From:           &from,
			Text:           text,
			ReplyToMessage: replied,
		}
		if err := e.dispatcher.ProcessUpdate(e.bot, &gotgbot.Update{
			UpdateId: e.updateID(),
			Message:  &msg,
		}, nil); err != nil {
			t.Fatalf("dispatch %q: %v", text, err)
		}
	}

	hasAppealLastRow := func(where string, rows [][]gotgbot.InlineKeyboardButton) {
		t.Helper()
		if len(rows) == 0 {
			t.Fatalf("%s: expected keyboard with Appeal URL button, got none", where)
		}
		last := rows[len(rows)-1]
		if len(last) != 1 || last[0].Text != wantAppealLabel || last[0].Url != appealURL {
			t.Fatalf("%s: last keyboard row = %+v, want single appeal button {%q, %q}", where, last, wantAppealLabel, appealURL)
		}
	}

	assertNoAppealButton := func(where string, rows [][]gotgbot.InlineKeyboardButton) {
		t.Helper()
		for _, row := range rows {
			for _, btn := range row {
				if btn.Url == appealURL || btn.Text == wantAppealLabel {
					t.Fatalf("%s: unexpected appeal button in keyboard: %+v", where, rows)
				}
			}
		}
	}

	// 1. Owner opens private /settings -> Manual defaults (q), sets default /ban duration = 3600 (1h),
	//    enables Reason required (v=1), and sets Appeal link via typed reply.
	menu := e.openMain(e.owner)
	menu.tap("a", settingsActOpen, "p", settingsPageModDefaults)
	menu.tap("a", settingsActModDefDur, "k", settingsModDefKindBan)
	menu.tap("a", settingsActModDefSetDur, "k", settingsModDefKindBan, "v", "3600")
	menu.tap("a", settingsActModDefReason, "v", "1")
	menu.tap("a", settingsActModDefAppealPrompt)
	if !e.redis.Exists(settingsInputKey(e.owner.Id)) {
		t.Fatal("step 1: tapping Appeal link left no active typed wait")
	}
	e.sendPrivate(e.owner, appealURL)
	if e.redis.Exists(settingsInputKey(e.owner.Id)) {
		t.Fatal("step 1: typing appeal URL did not finish the wait")
	}
	menu = e.lastPrivate(e.owner)

	defs, err := modtuning.GetDefaultsFresh(ctx, e.chat.Id)
	if err != nil {
		t.Fatalf("GetDefaultsFresh after step 1: %v", err)
	}
	if defs.DefaultBanSecs != 3600 || !defs.ReasonRequired || defs.AppealURL != appealURL {
		t.Fatalf("step 1 stored defaults = %+v, want ban=3600 reason_required=true appeal=%q", defs, appealURL)
	}

	// 2. Owner opens Reason shortcuts (rs), adds spam -> "Repeated <promo> & links" via two-step typed
	//    reply, and verifies the rendered list line is HTML-escaped.
	menu.tap("a", settingsActBack)
	menu.tap("a", settingsActOpen, "p", settingsPageReasons)
	menu.tap("a", "sm")
	if !e.redis.Exists(settingsInputKey(e.owner.Id)) {
		t.Fatal("step 2: tapping Add on page rs left no active typed wait")
	}
	e.sendPrivate(e.owner, shortcutWord)
	if !e.redis.Exists(settingsInputKey(e.owner.Id)) {
		t.Fatal("step 2: step 1 word did not advance to step 2 wait")
	}
	e.sendPrivate(e.owner, shortcutReason)
	if e.redis.Exists(settingsInputKey(e.owner.Id)) {
		t.Fatal("step 2: step 2 reason did not finish the wait")
	}
	rsMenu := e.lastPrivate(e.owner)
	wantListLine := "<code>spam</code> → " + escapedReason
	if !strings.Contains(rsMenu.text, wantListLine) {
		t.Fatalf("step 2: page rs list lacks %q:\n%s", wantListLine, rsMenu.text)
	}
	if strings.Contains(rsMenu.text, "<promo>") {
		t.Fatalf("step 2: page rs leaked raw HTML <promo>:\n%s", rsMenu.text)
	}

	// 3. Admin runs /ban (reply to member, NO reason): refused with moddef_reason_required,
	//    0 banChatMember calls, 0 PM copies, 0 log posts.
	target1 := newTarget("Alice")
	bansBefore := len(callsToChat(e.fake.staffBotClient, "banChatMember", e.chat.Id))
	groupBefore := len(callsToChat(e.fake.staffBotClient, "sendMessage", e.chat.Id))
	pmBefore := len(callsToChat(e.fake.staffBotClient, "sendMessage", target1.Id))
	logsBefore := len(callsToChat(e.fake.staffBotClient, "sendMessage", logChannelID))

	sendGroupReply(e.admin, target1, "/ban")
	waitEventPM(t)

	if got := len(callsToChat(e.fake.staffBotClient, "banChatMember", e.chat.Id)); got != bansBefore {
		t.Fatalf("step 3: /ban without reason called banChatMember %d times, want 0", got-bansBefore)
	}
	if got := len(callsToChat(e.fake.staffBotClient, "sendMessage", target1.Id)); got != pmBefore {
		t.Fatalf("step 3: /ban without reason sent %d PM copies, want 0", got-pmBefore)
	}
	if got := len(callsToChat(e.fake.staffBotClient, "sendMessage", logChannelID)); got != logsBefore {
		t.Fatalf("step 3: /ban without reason sent %d log posts, want 0", got-logsBefore)
	}
	groupSends := callsToChat(e.fake.staffBotClient, "sendMessage", e.chat.Id)
	if len(groupSends) != groupBefore+1 {
		t.Fatalf("step 3: group sendMessage delta = %d, want 1 refusal", len(groupSends)-groupBefore)
	}
	if refusalText := fmt.Sprint(groupSends[len(groupSends)-1].Params["text"]); !strings.Contains(refusalText, wantReasonRefusal) {
		t.Fatalf("step 3: refusal = %q, want %q", refusalText, wantReasonRefusal)
	}

	// 4. Admin runs /ban spam (reply to member): banChatMember receives until_date = fixedNow + 3600;
	//    group reply uses EventTimedBan with escaped expanded reason + Appeal URL button;
	//    PM copy delivers escaped full reason + Appeal URL button; log channel receives escaped full reason.
	bansBefore = len(callsToChat(e.fake.staffBotClient, "banChatMember", e.chat.Id))
	groupBefore = len(callsToChat(e.fake.staffBotClient, "sendMessage", e.chat.Id))
	pmBefore = len(callsToChat(e.fake.staffBotClient, "sendMessage", target1.Id))
	logsBefore = len(callsToChat(e.fake.staffBotClient, "sendMessage", logChannelID))

	sendGroupReply(e.admin, target1, "/ban spam")
	waitEventPM(t)

	banCalls := callsToChat(e.fake.staffBotClient, "banChatMember", e.chat.Id)
	if len(banCalls) != bansBefore+1 {
		t.Fatalf("step 4: banChatMember delta = %d, want 1", len(banCalls)-bansBefore)
	}
	wantDefaultUntil := fixedNow.Unix() + 3600
	if gotUntil := modTuningUntilDate(t, banCalls[len(banCalls)-1]); gotUntil != wantDefaultUntil {
		t.Fatalf("step 4: banChatMember until_date = %d, want %d", gotUntil, wantDefaultUntil)
	}

	groupSends = callsToChat(e.fake.staffBotClient, "sendMessage", e.chat.Id)
	if len(groupSends) != groupBefore+1 {
		t.Fatalf("step 4: group sendMessage delta = %d, want 1", len(groupSends)-groupBefore)
	}
	step4Group := groupSends[len(groupSends)-1]
	step4GroupText := fmt.Sprint(step4Group.Params["text"])
	if !strings.Contains(step4GroupText, "Тимчасове видалення") || !strings.Contains(step4GroupText, "1 год") {
		t.Fatalf("step 4: group reply = %q, want EventTimedBan wording with 1 год", step4GroupText)
	}
	if !strings.Contains(step4GroupText, escapedReason) || strings.Contains(step4GroupText, "<promo>") {
		t.Fatalf("step 4: group reply = %q, want HTML-escaped reason %q", step4GroupText, escapedReason)
	}
	hasAppealLastRow("step 4 group reply", appealCallKeyboard(t, step4Group))

	pmSends := callsToChat(e.fake.staffBotClient, "sendMessage", target1.Id)
	if len(pmSends) != pmBefore+1 {
		t.Fatalf("step 4: PM copy sendMessage delta = %d, want 1", len(pmSends)-pmBefore)
	}
	step4PM := pmSends[len(pmSends)-1]
	step4PMText := fmt.Sprint(step4PM.Params["text"])
	if !strings.Contains(step4PMText, escapedReason) || strings.Contains(step4PMText, "<promo>") {
		t.Fatalf("step 4: PM copy = %q, want HTML-escaped reason %q", step4PMText, escapedReason)
	}
	hasAppealLastRow("step 4 PM copy", appealCallKeyboard(t, step4PM))

	logSends := callsToChat(e.fake.staffBotClient, "sendMessage", logChannelID)
	if len(logSends) != logsBefore+1 {
		t.Fatalf("step 4: log channel sendMessage delta = %d, want 1", len(logSends)-logsBefore)
	}
	step4LogText := fmt.Sprint(logSends[len(logSends)-1].Params["text"])
	if !strings.Contains(step4LogText, escapedReason) || strings.Contains(step4LogText, "<promo>") {
		t.Fatalf("step 4: log channel post = %q, want HTML-escaped reason %q", step4LogText, escapedReason)
	}

	// 5. Admin runs /warn spam (reply to member): warn reply and stored warn record carry the expanded reason.
	target2 := newTarget("Boris")
	groupBefore = len(callsToChat(e.fake.staffBotClient, "sendMessage", e.chat.Id))

	sendGroupReply(e.admin, target2, "/warn spam")

	groupSends = callsToChat(e.fake.staffBotClient, "sendMessage", e.chat.Id)
	if len(groupSends) != groupBefore+1 {
		t.Fatalf("step 5: /warn group sendMessage delta = %d, want 1", len(groupSends)-groupBefore)
	}
	step5WarnText := fmt.Sprint(groupSends[len(groupSends)-1].Params["text"])
	if !strings.Contains(step5WarnText, escapedReason) || strings.Contains(step5WarnText, "<promo>") {
		t.Fatalf("step 5: /warn reply = %q, want HTML-escaped reason %q", step5WarnText, escapedReason)
	}
	warnCount, warnReasons := warns.GetWarns(target2.Id, e.chat.Id)
	if warnCount != 1 || len(warnReasons) != 1 || warnReasons[0] != shortcutReason {
		t.Fatalf("step 5: stored warns = (count=%d, reasons=%q), want 1 with %q", warnCount, warnReasons, shortcutReason)
	}

	// 6. Admin runs /tban 2h other: uses explicit 7200 s (not default 3600 s), keeps unknown word "other"
	//    verbatim, and still carries the Appeal URL button.
	target3 := newTarget("Clara")
	bansBefore = len(callsToChat(e.fake.staffBotClient, "banChatMember", e.chat.Id))
	groupBefore = len(callsToChat(e.fake.staffBotClient, "sendMessage", e.chat.Id))
	pmBefore = len(callsToChat(e.fake.staffBotClient, "sendMessage", target3.Id))

	beforeTban := time.Now().Unix()
	sendGroupReply(e.admin, target3, "/tban 2h other")
	afterTban := time.Now().Unix()
	waitEventPM(t)

	banCalls = callsToChat(e.fake.staffBotClient, "banChatMember", e.chat.Id)
	if len(banCalls) != bansBefore+1 {
		t.Fatalf("step 6: /tban banChatMember delta = %d, want 1", len(banCalls)-bansBefore)
	}
	tbanUntil := modTuningUntilDate(t, banCalls[len(banCalls)-1])
	if tbanUntil < beforeTban+7200 || tbanUntil > afterTban+7200 || tbanUntil == wantDefaultUntil {
		t.Fatalf("step 6: /tban until_date = %d, want in [%d, %d] and != default %d",
			tbanUntil, beforeTban+7200, afterTban+7200, wantDefaultUntil)
	}
	groupSends = callsToChat(e.fake.staffBotClient, "sendMessage", e.chat.Id)
	if len(groupSends) != groupBefore+1 {
		t.Fatalf("step 6: /tban group sendMessage delta = %d, want 1", len(groupSends)-groupBefore)
	}
	step6Group := groupSends[len(groupSends)-1]
	step6GroupText := fmt.Sprint(step6Group.Params["text"])
	if !strings.Contains(step6GroupText, "other") || strings.Contains(step6GroupText, "Repeated") {
		t.Fatalf("step 6: /tban group reply = %q, want verbatim 'other' without shortcut expansion", step6GroupText)
	}
	hasAppealLastRow("step 6 group reply", appealCallKeyboard(t, step6Group))

	pmSends = callsToChat(e.fake.staffBotClient, "sendMessage", target3.Id)
	if len(pmSends) != pmBefore+1 {
		t.Fatalf("step 6: /tban PM copy delta = %d, want 1", len(pmSends)-pmBefore)
	}
	step6PM := pmSends[len(pmSends)-1]
	if step6PMText := fmt.Sprint(step6PM.Params["text"]); !strings.Contains(step6PMText, "other") {
		t.Fatalf("step 6: /tban PM copy = %q, want verbatim 'other'", step6PMText)
	}
	hasAppealLastRow("step 6 PM copy", appealCallKeyboard(t, step6PM))

	// 7. Owner deletes spam shortcut behind two-tap card (sx -> sy), resets default /ban duration to
	//    Permanent (0), turns Reason required off (0), and removes Appeal link (mx).
	storedShortcuts, err := modtuning.ListShortcutsFresh(ctx, e.chat.Id)
	if err != nil || len(storedShortcuts) != 1 || storedShortcuts[0].Word != shortcutWord {
		t.Fatalf("step 7: ListShortcutsFresh = %+v (%v), want 1 shortcut %q", storedShortcuts, err, shortcutWord)
	}
	idStr := strconv.FormatInt(storedShortcuts[0].ID, 10)
	rsMenu.tap("a", "sx", "r", idStr, "v", "1")
	rsMenu.tap("a", "sy", "r", idStr)
	if remaining, err := modtuning.ListShortcutsFresh(ctx, e.chat.Id); err != nil || len(remaining) != 0 {
		t.Fatalf("step 7: shortcuts after delete = %+v (%v), want 0", remaining, err)
	}

	rsMenu.tap("a", settingsActBack)
	rsMenu.tap("a", settingsActOpen, "p", settingsPageModDefaults)
	rsMenu.tap("a", settingsActModDefDur, "k", settingsModDefKindBan)
	rsMenu.tap("a", settingsActModDefSetDur, "k", settingsModDefKindBan, "v", "0")
	rsMenu.tap("a", settingsActModDefReason, "v", "0")
	rsMenu.tap("a", settingsActModDefAppealRemove)

	cleared, err := modtuning.GetDefaultsFresh(ctx, e.chat.Id)
	if err != nil {
		t.Fatalf("step 7: GetDefaultsFresh after clear: %v", err)
	}
	if cleared.DefaultBanSecs != 0 || cleared.DefaultMuteSecs != 0 || cleared.ReasonRequired || cleared.AppealURL != "" {
		t.Fatalf("step 7: cleared defaults = %+v, want all zero", cleared)
	}

	// 8. Admin runs /ban (reply, no reason): permanent banChatMember (no until_date), default EventBan
	//    reply with Unban button only (no Appeal button), and PM copy with no Appeal button.
	target4 := newTarget("Daria")
	bansBefore = len(callsToChat(e.fake.staffBotClient, "banChatMember", e.chat.Id))
	groupBefore = len(callsToChat(e.fake.staffBotClient, "sendMessage", e.chat.Id))
	pmBefore = len(callsToChat(e.fake.staffBotClient, "sendMessage", target4.Id))

	sendGroupReply(e.admin, target4, "/ban")
	waitEventPM(t)

	banCalls = callsToChat(e.fake.staffBotClient, "banChatMember", e.chat.Id)
	if len(banCalls) != bansBefore+1 {
		t.Fatalf("step 8: /ban banChatMember delta = %d, want 1", len(banCalls)-bansBefore)
	}
	step8Ban := banCalls[len(banCalls)-1]
	if gotUntil := modTuningUntilDate(t, step8Ban); gotUntil != 0 {
		t.Fatalf("step 8: cleared /ban until_date = %d, want 0 (permanent)", gotUntil)
	}
	if _, hasUntil := step8Ban.Params["until_date"]; hasUntil {
		t.Fatalf("step 8: cleared /ban unexpectedly passed until_date param: %+v", step8Ban.Params)
	}

	groupSends = callsToChat(e.fake.staffBotClient, "sendMessage", e.chat.Id)
	if len(groupSends) != groupBefore+1 {
		t.Fatalf("step 8: /ban group sendMessage delta = %d, want 1", len(groupSends)-groupBefore)
	}
	step8Group := groupSends[len(groupSends)-1]
	step8GroupText := fmt.Sprint(step8Group.Params["text"])
	if strings.Contains(step8GroupText, "Тимчасове видалення") {
		t.Fatalf("step 8: cleared /ban used EventTimedBan wording: %q", step8GroupText)
	}
	step8KB := appealCallKeyboard(t, step8Group)
	assertNoAppealButton("step 8 group reply", step8KB)
	wantUnbanData := fmt.Sprintf("unrestrict|v1|a=unban&u=%d", target4.Id)
	if len(step8KB) != 1 || len(step8KB[0]) != 1 || step8KB[0][0].CallbackData != wantUnbanData || step8KB[0][0].Url != "" {
		t.Fatalf("step 8: cleared /ban keyboard = %+v, want Unban button (%q) only", step8KB, wantUnbanData)
	}

	pmSends = callsToChat(e.fake.staffBotClient, "sendMessage", target4.Id)
	if len(pmSends) != pmBefore+1 {
		t.Fatalf("step 8: /ban PM copy delta = %d, want 1", len(pmSends)-pmBefore)
	}
	assertNoAppealButton("step 8 PM copy", appealCallKeyboard(t, pmSends[len(pmSends)-1]))
}
