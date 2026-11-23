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

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/advthresholds"
	"github.com/uasneppy/fuku_robot/fuku/db/antiflood"
	"github.com/uasneppy/fuku_robot/fuku/db/blacklists"
	dbcache "github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/escalation"
	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/db/lang"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/db/modtuning"
	"github.com/uasneppy/fuku_robot/fuku/db/warns"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
)

// TestPhase29EndToEnd drives the complete Phase 29 user journey (TUNE-08 Plain tone and TUNE-09
// Copy Settings) through a real ext.Dispatcher backed by in-process miniredis
// (phase28InProcessMiniredis, zero localhost TCP dials) and a fake Telegram HTTP client.
func TestPhase29EndToEnd(t *testing.T) {
	ctx := context.Background()
	fixedNow := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	withFrozenPunishNow(t, fixedNow)
	withFastPMCopyPacer(t)
	t.Cleanup(pmCopyJobs.wait)

	env := newPhase28SettingsEnv(t)
	withRealLocales(t)

	LoadBans(env.dispatcher)
	LoadMutes(env.dispatcher)
	LoadWarns(env.dispatcher)

	titleClient := &copyTitleBotClient{lockdownFake: env.fake, titles: map[int64]string{}}
	e2eClient := newMessagesE2EClient(titleClient)
	env.bot.BotClient = e2eClient
	env.fake.sendDate = func() int64 { return fixedNow.Unix() }

	srcChat := env.chat
	srcChat.Title = "Source <Facility> & Co"
	env.chat.Title = srcChat.Title
	dstChat := env.addChat()
	dstChat.Title = "Target <Sector> & Co"

	titleClient.setTitle(srcChat.Id, srcChat.Title)
	titleClient.setTitle(dstChat.Id, dstChat.Title)

	cleanupCopyPageChat(t, srcChat.Id)
	cleanupCopyPageChat(t, dstChat.Id)
	settingsMsgCleanup(t, srcChat.Id)
	settingsMsgCleanup(t, dstChat.Id)
	eventPMCleanup(t, srcChat.Id)
	eventPMCleanup(t, dstChat.Id)
	cleanupModTuningChat(t, srcChat.Id)
	cleanupModTuningChat(t, dstChat.Id)

	if err := lang.ChangeGroupLanguage(srcChat.Id, "en"); err != nil {
		t.Fatalf("ChangeGroupLanguage(srcChat, en): %v", err)
	}
	if err := lang.ChangeGroupLanguage(dstChat.Id, "en"); err != nil {
		t.Fatalf("ChangeGroupLanguage(dstChat, en): %v", err)
	}
	t.Cleanup(func() {
		for _, cid := range []int64{srcChat.Id, dstChat.Id} {
			db.DB.Where("chat_id = ?", cid).Delete(&models.Chat{})
			db.DB.Where("chat_id = ?", cid).Delete(&models.Warns{})
			db.DB.Where("chat_id = ?", cid).Delete(&models.ApprovedUsers{})
			dbcache.DeleteCache(dbcache.CacheKey("chat_lang", cid))
			dbcache.DeleteCache(dbcache.CacheKey("approvals", cid))
		}
		dbcache.ResetLocalForTest()
	})

	owner := env.newOwner(srcChat)
	if err := lang.ChangeUserLanguage(owner.Id, "en"); err != nil {
		t.Fatalf("ChangeUserLanguage(owner, en): %v", err)
	}
	t.Cleanup(func() {
		db.DB.Where("user_id = ?", owner.Id).Delete(&models.User{})
		dbcache.DeleteCache(dbcache.CacheKey("user_lang", owner.Id))
	})

	for _, cid := range []int64{srcChat.Id, dstChat.Id} {
		env.fake.setBotRole(cid, staffRoleAdmin)
		env.fake.setMemberRaw(cid, owner.Id, copyCreatorJSON(owner.Id))
		env.fake.setCreator(cid, owner.Id)
		env.fake.setMember(cid, env.admin.Id, staffFakeMember{
			Status:             gotgbot.ChatMemberStatusAdministrator,
			CanRestrictMembers: true,
		})
		env.fake.listAdministrator(cid, env.admin.Id)
	}
	settingsHubSeedGroup(t, srcChat, srcChat.Title, settingsHubCreator(owner))
	settingsHubSeedGroup(t, dstChat, dstChat.Title, settingsHubCreator(owner))

	newTarget := func(chatID int64, first string) gotgbot.User {
		t.Helper()
		u := gotgbot.User{Id: uniqueLinkOwnerID(), FirstName: first}
		e2eClient.register(u)
		env.fake.setProfile(u.Id, u.FirstName, "", "")
		env.fake.setMember(chatID, u.Id, staffFakeMember{Status: gotgbot.ChatMemberStatusMember})
		memberReqCleanup(t, nil, []int64{u.Id})
		return u
	}

	sendGroupReply := func(chat gotgbot.Chat, from, repliedTo gotgbot.User, text string) {
		t.Helper()
		replied := &gotgbot.Message{
			MessageId: env.updateID(),
			Chat:      chat,
			From:      &repliedTo,
			Text:      "hello",
		}
		msg := gotgbot.Message{
			MessageId:      env.updateID(),
			Chat:           chat,
			From:           &from,
			Text:           text,
			ReplyToMessage: replied,
		}
		if err := env.dispatcher.ProcessUpdate(env.bot, &gotgbot.Update{
			UpdateId: env.updateID(),
			Message:  &msg,
		}, nil); err != nil {
			t.Fatalf("dispatch %q in %d: %v", text, chat.Id, err)
		}
	}

	ukTr := i18n.MustNewTranslator("uk")
	enTr := i18n.MustNewTranslator("en")

	// =========================================================================
	// Part 1: Plain Tone End-to-End (TUNE-08)
	// =========================================================================

	// Open private /settings for srcChat -> Messages hub (h) -> turn Plain tone ON via ht (v=1).
	privMsg := settingsPrivateOnlyMessage(t, env, owner)
	pressSrc := func(kv ...string) {
		t.Helper()
		env.pressPrivate(owner, privMsg, env.privateFields(srcChat.Id, kv...))
	}

	pressSrc("a", settingsActOpen, "p", settingsPageMessages)
	hubBeforeText, hubBeforeKB := env.lastEdit(owner.Id, privMsg)
	if _, ok := settingsAdvFindButton(t, hubBeforeKB, map[string]string{"a": settingsActMsgPlainTone, "v": "1"}); !ok {
		t.Fatalf("Messages hub missing ht v=1 button:\n%s", hubBeforeText)
	}

	pressSrc("a", settingsActMsgPlainTone, "v", "1")
	hubAfterText, hubAfterKB := env.lastEdit(owner.Id, privMsg)
	if _, ok := settingsAdvFindButton(t, hubAfterKB, map[string]string{"a": settingsActMsgPlainTone, "v": "0"}); !ok {
		t.Fatalf("Messages hub after ht v=1 missing ht v=0 toggle:\n%s", hubAfterText)
	}
	srcMsgs, err := eventmsg.GetFresh(ctx, srcChat.Id)
	if err != nil || !srcMsgs.Options.PlainTone {
		t.Fatalf("eventmsg.GetFresh after ht v=1: PlainTone=%v err=%v, want PlainTone=true", srcMsgs.Options.PlainTone, err)
	}

	// Enable PM copy for EventBan ("bn") via the real Messages page UI.
	pressSrc("a", settingsActMsgFamily, "t", "m")
	pressSrc("a", settingsActMsgEvent, settingsMsgEventField, "bn", settingsMsgDestField, settingsMsgDestPMCode)
	pressSrc("a", settingsActMsgPM, settingsMsgEventField, "bn", "v", "1")

	// Admin runs /ban in srcChat (no custom text yet): group reply uses neutral eventmsg_plain_ban
	// (with Unban row intact) and PM copy uses neutral eventmsg_plain_pm_ban.
	target1 := newTarget(srcChat.Id, "PlainTarget")
	srcGroupBefore := len(callsToChat(env.fake.staffBotClient, "sendMessage", srcChat.Id))
	t1PMBefore := len(callsToChat(env.fake.staffBotClient, "sendMessage", target1.Id))

	sendGroupReply(srcChat, env.admin, target1, "/ban spam rule")
	waitEventPM(t)

	srcGroupSends := callsToChat(env.fake.staffBotClient, "sendMessage", srcChat.Id)
	if len(srcGroupSends) != srcGroupBefore+1 {
		t.Fatalf("Part 1 /ban group sendMessage delta = %d, want 1", len(srcGroupSends)-srcGroupBefore)
	}
	banGroupCall := srcGroupSends[len(srcGroupSends)-1]
	banGroupText := fmt.Sprint(banGroupCall.Params["text"])
	wantPlainBan := banPlainDefaultReply(ukTr, eventDefaultInput{Target: target1, Reason: "spam rule"})
	if banGroupText != wantPlainBan {
		t.Errorf("Part 1 /ban group text = %q, want plain %q", banGroupText, wantPlainBan)
	}
	assertNoThemedEmoji(t, "Part 1 /ban group reply", banGroupText)
	// Verify Unban inline button row is preserved on plain-tone ban reply.
	banGroupKB := appealCallKeyboard(t, banGroupCall)
	if len(banGroupKB) == 0 || len(banGroupKB[0]) == 0 || !strings.HasPrefix(banGroupKB[0][0].CallbackData, "unrestrict|v1|a=unban") {
		t.Errorf("Part 1 /ban plain reply missing Unban button row: %+v", banGroupKB)
	}

	t1PMSends := callsToChat(env.fake.staffBotClient, "sendMessage", target1.Id)
	if len(t1PMSends) != t1PMBefore+1 {
		t.Fatalf("Part 1 /ban PM sendMessage delta = %d, want 1", len(t1PMSends)-t1PMBefore)
	}
	banPMText := fmt.Sprint(t1PMSends[len(t1PMSends)-1].Params["text"])
	assertNoThemedEmoji(t, "Part 1 /ban PM copy", banPMText)
	if !strings.Contains(banPMText, "<b>Source &lt;Facility&gt; &amp; Co</b>") || !strings.Contains(banPMText, "spam rule") {
		t.Errorf("Part 1 /ban PM text = %q, want plain PM copy with escaped title and reason", banPMText)
	}

	// Custom text precedence: store a custom EventBan group text on srcChat; verify /ban uses custom
	// text untouched while /kick still uses neutral plain tone (eventmsg_plain_kick).
	const customBanTemplate = "CustomBanNotice: {mention} — reason: {reason}"
	if err := eventmsg.SetText(ctx, srcChat.Id, eventmsg.EventBan, eventmsg.DestGroup, customBanTemplate, owner.Id); err != nil {
		t.Fatalf("eventmsg.SetText(EventBan): %v", err)
	}

	target2 := newTarget(srcChat.Id, "CustomTarget")
	srcGroupBefore = len(callsToChat(env.fake.staffBotClient, "sendMessage", srcChat.Id))
	sendGroupReply(srcChat, env.admin, target2, "/ban customcheck")
	waitEventPM(t)

	srcGroupSends = callsToChat(env.fake.staffBotClient, "sendMessage", srcChat.Id)
	if len(srcGroupSends) != srcGroupBefore+1 {
		t.Fatalf("Part 1 custom /ban group sendMessage delta = %d, want 1", len(srcGroupSends)-srcGroupBefore)
	}
	customBanGot := fmt.Sprint(srcGroupSends[len(srcGroupSends)-1].Params["text"])
	if !strings.HasPrefix(customBanGot, "CustomBanNotice:") || !strings.Contains(customBanGot, "customcheck") {
		t.Errorf("Part 1 custom /ban text = %q, want custom template", customBanGot)
	}

	target3 := newTarget(srcChat.Id, "KickTarget")
	srcGroupBefore = len(callsToChat(env.fake.staffBotClient, "sendMessage", srcChat.Id))
	sendGroupReply(srcChat, env.admin, target3, "/kick plainkick")
	srcGroupSends = callsToChat(env.fake.staffBotClient, "sendMessage", srcChat.Id)
	if len(srcGroupSends) != srcGroupBefore+1 {
		t.Fatalf("Part 1 /kick group sendMessage delta = %d, want 1", len(srcGroupSends)-srcGroupBefore)
	}
	kickGot := fmt.Sprint(srcGroupSends[len(srcGroupSends)-1].Params["text"])
	wantPlainKick := kickPlainDefaultReply(ukTr, eventDefaultInput{Target: target3, Reason: "plainkick"})
	if kickGot != wantPlainKick {
		t.Errorf("Part 1 /kick text = %q, want plain %q", kickGot, wantPlainKick)
	}

	// =========================================================================
	// Part 2: Copy Settings End-to-End (TUNE-09)
	// =========================================================================

	// Configure all remaining tuning areas on srcChat:
	// - Area 3: timed warn mode ("tban", 7200) + warn expiry (14 days), timed flood action ("tmute", 1800),
	//   blacklist word "srcword" with action ("tban", 3600).
	if err := warns.SetWarnLimit(srcChat.Id, 5); err != nil {
		t.Fatalf("src SetWarnLimit: %v", err)
	}
	if err := warns.SetWarnModeContext(ctx, srcChat.Id, "tban", 7200); err != nil {
		t.Fatalf("src SetWarnModeContext: %v", err)
	}
	if err := warns.SetWarnExpiryContext(ctx, srcChat.Id, 14); err != nil {
		t.Fatalf("src SetWarnExpiryContext: %v", err)
	}
	if err := antiflood.SetFloodContext(ctx, srcChat.Id, 6); err != nil {
		t.Fatalf("src SetFloodContext: %v", err)
	}
	if err := antiflood.SetFloodActionContext(ctx, srcChat.Id, "tmute", 1800); err != nil {
		t.Fatalf("src SetFloodActionContext: %v", err)
	}
	if err := blacklists.AddBlacklist(srcChat.Id, "srcword"); err != nil {
		t.Fatalf("src AddBlacklist: %v", err)
	}
	if _, err := blacklists.SetBlacklistActionTimedContext(ctx, srcChat.Id, "tban", 3600); err != nil {
		t.Fatalf("src SetBlacklistActionTimedContext: %v", err)
	}

	// - Area 4: Escalation ladder enabled with 2 steps and 30-day forget window.
	srcLadder := escalation.Ladder{
		Enabled:    true,
		Steps:      []escalation.Step{{Action: escalation.ActionTMute, Secs: 900}, {Action: escalation.ActionTBan, Secs: 3600}},
		ForgetDays: 30,
	}
	if err := escalation.ReplaceLadder(ctx, srcChat.Id, srcLadder, owner.Id); err != nil {
		t.Fatalf("src ReplaceLadder: %v", err)
	}

	// - Area 5: Manual defaults (appeal link + reason required) and reason shortcut "sp" -> "Spam link".
	const copiedAppealURL = "https://appeal.example.org/form"
	if err := modtuning.ReplaceDefaults(ctx, srcChat.Id, modtuning.Defaults{
		ReasonRequired: true,
		AppealURL:      copiedAppealURL,
	}, owner.Id); err != nil {
		t.Fatalf("src ReplaceDefaults: %v", err)
	}
	if err := modtuning.ReplaceShortcuts(ctx, srcChat.Id, []modtuning.ShortcutEntry{
		{Word: "sp", Reason: "Spam link"},
	}, owner.Id); err != nil {
		t.Fatalf("src ReplaceShortcuts: %v", err)
	}

	// - Area 6: Advanced threshold FieldRaidFloodWindow = 120.
	if err := advthresholds.SetField(ctx, srcChat.Id, advthresholds.FieldRaidFloodWindow, 120, owner.Id); err != nil {
		t.Fatalf("src advthresholds.SetField: %v", err)
	}

	// Seed dstChat pre-copy state:
	// - NO antiflood_settings row (Limit == 0, antiflood OFF!)
	// - Own blacklist word "dstword" (with default "warn", 0)
	// - Excluded tables: approvals row, Staff Group link, and warns_users row.
	if err := blacklists.AddBlacklist(dstChat.Id, "dstword"); err != nil {
		t.Fatalf("dst AddBlacklist: %v", err)
	}
	const approvedMemberID int64 = 880011
	dstApproval := models.ApprovedUsers{ChatID: dstChat.Id, UserID: approvedMemberID, Reason: "vip", ApprovedBy: owner.Id}
	if err := db.DB.Create(&dstApproval).Error; err != nil {
		t.Fatalf("dst ApprovedUsers create: %v", err)
	}
	const warnedMemberID int64 = 880022
	dstWarnRow := models.Warns{ChatId: dstChat.Id, UserId: warnedMemberID, NumWarns: 1, Reasons: models.StringArray{"prior dst warn"}}
	if err := db.DB.Create(&dstWarnRow).Error; err != nil {
		t.Fatalf("dst Warns create: %v", err)
	}
	dstStaffChat := settingsStaffTestOwnedStaffGroup(env, "Dst Linked Staff HQ")
	t.Cleanup(func() { cleanupCopyPageChat(t, dstStaffChat.Id) })
	dstLink := models.StaffGroupLink{GroupChatID: dstChat.Id, StaffChatID: dstStaffChat.Id, OwnerUserID: owner.Id, GroupTitle: "Dst", Health: models.StaffHealthOK}
	if err := db.DB.Create(&dstLink).Error; err != nil {
		t.Fatalf("dst StaffGroupLink create: %v", err)
	}

	// Owner opens private Copy Settings (cp) for srcChat -> picks dstChat (ck) -> verifies confirm card -> presses Yes (cy).
	pressSrc("a", settingsActOpen, "p", settingsPageCopy)
	pickerText, pickerKB := env.lastEdit(owner.Id, privMsg)
	pickBtn, ok := settingsAdvFindButton(t, pickerKB, map[string]string{
		"a": settingsActCopyPick,
		"t": strconv.FormatInt(dstChat.Id, 10),
	})
	if !ok {
		t.Fatalf("Copy Settings picker missing dstChat button:\n%s", pickerText)
	}

	env.pressRaw(settingsPrivateChat(owner), owner, privMsg, pickBtn.CallbackData)
	cardText, cardKB := env.lastEdit(owner.Id, privMsg)
	for _, mustContain := range []string{
		"Source &lt;Facility&gt; &amp; Co",
		"Target &lt;Sector&gt; &amp; Co",
		"Custom event messages (group and private)",
		"Silent, private copy, and Plain tone switches",
		"Timed automatic punishments and warn expiry",
		"Escalation ladder",
		"Manual defaults, appeal link, and reason shortcuts",
		"Advanced thresholds",
		"Staff Group links",
		"does not link the two groups",
	} {
		if !strings.Contains(cardText, mustContain) {
			t.Errorf("confirm card missing %q:\n%s", mustContain, cardText)
		}
	}
	yesBtn, ok := settingsAdvFindButton(t, cardKB, map[string]string{"a": settingsActCopyYes})
	if !ok {
		t.Fatalf("confirm card missing cy Yes button")
	}

	env.pressRaw(settingsPrivateChat(owner), owner, privMsg, yesBtn.CallbackData)
	doneText, _ := env.lastEdit(owner.Id, privMsg)
	if !strings.Contains(doneText, "Settings copied to <b>Target &lt;Sector&gt; &amp; Co</b> once.") {
		t.Fatalf("after cy, expected copied notice naming escaped target title, got:\n%s", doneText)
	}

	// Verify live moderation in dstChat through the real dispatcher:
	// 1. /kick in dstChat posts neutral plain tone (copied PlainTone=true) with expanded shortcut "sp" -> "Spam link".
	dstTarget1 := newTarget(dstChat.Id, "DstKickUser")
	dstGroupBefore := len(callsToChat(env.fake.staffBotClient, "sendMessage", dstChat.Id))
	sendGroupReply(dstChat, env.admin, dstTarget1, "/kick sp")
	dstGroupSends := callsToChat(env.fake.staffBotClient, "sendMessage", dstChat.Id)
	if len(dstGroupSends) != dstGroupBefore+1 {
		t.Fatalf("dstChat /kick sendMessage delta = %d, want 1", len(dstGroupSends)-dstGroupBefore)
	}
	dstKickText := fmt.Sprint(dstGroupSends[len(dstGroupSends)-1].Params["text"])
	wantDstKick := kickPlainDefaultReply(ukTr, eventDefaultInput{Target: dstTarget1, Reason: "Spam link"})
	if dstKickText != wantDstKick {
		t.Errorf("dstChat /kick text = %q, want copied plain tone + expanded shortcut %q", dstKickText, wantDstKick)
	}

	// 2. /ban without reason in dstChat is refused by copied ReasonRequired=true, and /ban sp posts
	//    copied custom ban message with expanded "Spam link" reason, appeal button, and PM copy.
	dstTarget2 := newTarget(dstChat.Id, "DstBanUser")
	dstBansBefore := len(callsToChat(env.fake.staffBotClient, "banChatMember", dstChat.Id))
	sendGroupReply(dstChat, env.admin, dstTarget2, "/ban")
	if got := len(callsToChat(env.fake.staffBotClient, "banChatMember", dstChat.Id)); got != dstBansBefore {
		t.Fatalf("dstChat /ban without reason called banChatMember %d times, want 0 (copied ReasonRequired)", got-dstBansBefore)
	}

	dstGroupBefore = len(callsToChat(env.fake.staffBotClient, "sendMessage", dstChat.Id))
	dstPMBefore := len(callsToChat(env.fake.staffBotClient, "sendMessage", dstTarget2.Id))
	sendGroupReply(dstChat, env.admin, dstTarget2, "/ban sp")
	waitEventPM(t)

	dstGroupSends = callsToChat(env.fake.staffBotClient, "sendMessage", dstChat.Id)
	if len(dstGroupSends) != dstGroupBefore+1 {
		t.Fatalf("dstChat /ban sp group sendMessage delta = %d, want 1", len(dstGroupSends)-dstGroupBefore)
	}
	dstBanCall := dstGroupSends[len(dstGroupSends)-1]
	dstBanText := fmt.Sprint(dstBanCall.Params["text"])
	if !strings.HasPrefix(dstBanText, "CustomBanNotice:") || !strings.Contains(dstBanText, "Spam link") {
		t.Errorf("dstChat /ban sp group text = %q, want copied custom ban template with expanded shortcut", dstBanText)
	}
	dstBanKB := appealCallKeyboard(t, dstBanCall)
	if len(dstBanKB) == 0 || dstBanKB[len(dstBanKB)-1][0].Url != copiedAppealURL {
		t.Errorf("dstChat /ban sp keyboard = %+v, want copied appeal URL button %q", dstBanKB, copiedAppealURL)
	}

	dstPMSends := callsToChat(env.fake.staffBotClient, "sendMessage", dstTarget2.Id)
	if len(dstPMSends) != dstPMBefore+1 {
		t.Fatalf("dstChat /ban sp PM copy delta = %d, want 1 (copied PM switch)", len(dstPMSends)-dstPMBefore)
	}
	dstPMText := fmt.Sprint(dstPMSends[len(dstPMSends)-1].Params["text"])
	assertNoThemedEmoji(t, "dstChat /ban sp PM copy", dstPMText)
	if !strings.Contains(dstPMText, "<b>Target &lt;Sector&gt; &amp; Co</b>") || !strings.Contains(dstPMText, "Spam link") {
		t.Errorf("dstChat /ban sp PM copy = %q, want plain PM copy with target title and expanded shortcut", dstPMText)
	}

	// 3. Verify P29-03, P29-04, escalation, advanced thresholds, and excluded tables on dstChat.
	dstFlood := antiflood.GetFloodContext(ctx, dstChat.Id)
	if dstFlood.Limit != 0 || dstFlood.Action != "tmute" || dstFlood.ActionSecs != 1800 {
		t.Errorf("dstChat antiflood = %+v, want Limit=0 (still OFF!) Action=tmute ActionSecs=1800", dstFlood)
	}
	dstWarns := warns.GetWarnSettingContext(ctx, dstChat.Id)
	if dstWarns.WarnLimit != 3 || dstWarns.WarnMode != "tban" || dstWarns.WarnModeSecs != 7200 || dstWarns.ExpiryDays != 14 {
		t.Errorf("dstChat warns_settings = %+v, want WarnLimit=3 (target preserved) Mode=tban/7200 Expiry=14", dstWarns)
	}
	dstBL := blacklists.GetBlacklistSettingsContext(ctx, dstChat.Id)
	if len(dstBL) != 1 || dstBL[0].Word != "dstword" || dstBL[0].Action != "tban" || dstBL[0].ActionSecs != 3600 {
		t.Errorf("dstChat blacklists = %+v, want only existing word dstword with copied tban/3600", dstBL)
	}
	dstEsc, err := escalation.GetFresh(ctx, dstChat.Id)
	if err != nil || !dstEsc.Enabled || len(dstEsc.Steps) != 2 || dstEsc.ForgetDays != 30 {
		t.Errorf("dstChat escalation = %+v (err=%v), want copied enabled 2-step ladder", dstEsc, err)
	}
	dstAdv, err := advthresholds.GetFresh(ctx, dstChat.Id)
	if err != nil || dstAdv.Value(advthresholds.FieldRaidFloodWindow) != 120 {
		t.Errorf("dstChat advthresholds = %+v (err=%v), want FieldRaidFloodWindow=120", dstAdv, err)
	}
	var gotAppr models.ApprovedUsers
	if err := db.DB.Where("chat_id = ? AND user_id = ?", dstChat.Id, approvedMemberID).Take(&gotAppr).Error; err != nil || gotAppr.Reason != "vip" {
		t.Errorf("dstChat approved_users altered: %+v (err=%v)", gotAppr, err)
	}
	var gotWarnUser models.Warns
	if err := db.DB.Where("chat_id = ? AND user_id = ?", dstChat.Id, warnedMemberID).Take(&gotWarnUser).Error; err != nil || gotWarnUser.NumWarns != 1 {
		t.Errorf("dstChat warns_users altered: %+v (err=%v)", gotWarnUser, err)
	}
	var linkCount int64
	if err := db.DB.Model(&models.StaffGroupLink{}).Where("group_chat_id = ? AND staff_chat_id = ?", dstChat.Id, dstStaffChat.Id).Count(&linkCount).Error; err != nil || linkCount != 1 {
		t.Errorf("dstChat StaffGroupLink count = %d (err=%v), want untouched 1", linkCount, err)
	}

	// 4. Verify non-linking: mutating dstChat afterwards leaves srcChat untouched, and replaying cy
	//    returns settings_card_expired without changing dstChat.
	if err := eventmsg.SetPlainTone(ctx, dstChat.Id, false, owner.Id); err != nil {
		t.Fatalf("dst SetPlainTone(false): %v", err)
	}
	srcMsgsAfter, err := eventmsg.GetFresh(ctx, srcChat.Id)
	if err != nil || !srcMsgsAfter.Options.PlainTone {
		t.Errorf("srcChat PlainTone after dstChat mutation = %v (err=%v), want true (groups not linked)", srcMsgsAfter.Options.PlainTone, err)
	}

	env.pressRaw(settingsPrivateChat(owner), owner, privMsg, yesBtn.CallbackData)
	replayAns, replayAlert := copyLastAnswer(t, env)
	wantExpired, _ := enTr.GetString("settings_card_expired")
	if !replayAlert || replayAns != wantExpired {
		t.Errorf("replayed cy answer = (%q, alert=%v), want (%q, true)", replayAns, replayAlert, wantExpired)
	}
	dstMsgsAfterReplay, err := eventmsg.GetFresh(ctx, dstChat.Id)
	if err != nil || dstMsgsAfterReplay.Options.PlainTone {
		t.Errorf("replayed cy modified dstChat PlainTone = %v (err=%v), want false", dstMsgsAfterReplay.Options.PlainTone, err)
	}
}
