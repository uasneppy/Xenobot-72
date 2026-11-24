//go:build testtools

package modules

import (
	"context"
	"fmt"
	"html"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	dbcache "github.com/uasneppy/Xenobot-72/xenobot/db/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/db/eventmsg"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/db/modtuning"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
)

// settingsPhase29Code is one action code Phase 29 added: its page and a valid private-menu press of it.
// Every Phase 29 action (ht on h; cg, ck, cy on cp) requires the private owner field c.
type settingsPhase29Code struct {
	code, page string
	valid      []string
}

func settingsPhase29Codes() []settingsPhase29Code {
	return []settingsPhase29Code{
		{settingsActMsgPlainTone, settingsPageMessages, []string{"v", "1"}},
		{settingsActCopyPage, settingsPageCopy, []string{"v", "1"}},
		{settingsActCopyPick, settingsPageCopy, []string{"t", "-100123"}},
		{settingsActCopyYes, settingsPageCopy, []string{"k", "0123456789abcdef"}},
	}
}

// TestPhase29Codes verifies all 29 registered pages, action code ht on h and cg/ck/cy on private-only
// page cp, zero collisions with core or other page codes, and step-1 refusal of extra unknown fields
// or group-menu presses (u instead of c) with 0 getChatMember calls.
func TestPhase29Codes(t *testing.T) {
	allPages := []string{
		"s", "v", "l", "c", "f", "q", "n", "j", "k", "rs",
		"b", "i", "w", "g", "r", "e", "x", "h", "m", "av",
		"cp", "ba", "d", "t", "u", "z", "o", "y", "p", "a",
	}
	if len(allPages) != 30 {
		t.Fatalf("expected 30 registered pages in test list, got %d", len(allPages))
	}
	if len(settingsPages) != len(allPages) {
		t.Errorf("registered settingsPages count = %d, want %d", len(settingsPages), len(allPages))
	}
	seenPages := map[string]bool{}
	for _, key := range allPages {
		if seenPages[key] {
			t.Errorf("duplicate page key %q in test list", key)
		}
		seenPages[key] = true
		if _, ok := settingsPageByKey(key); !ok {
			t.Errorf("page %q is not registered", key)
		}
	}

	msgPage, ok := settingsPageByKey(settingsPageMessages)
	if !ok {
		t.Fatalf("page %q is not registered", settingsPageMessages)
	}
	if !msgPage.PrivateOnly {
		t.Errorf("page %q PrivateOnly = false, want true", settingsPageMessages)
	}

	cpPage, ok := settingsPageByKey(settingsPageCopy)
	if !ok {
		t.Fatalf("page %q is not registered", settingsPageCopy)
	}
	if !cpPage.PrivateOnly {
		t.Errorf("page %q PrivateOnly = false, want true", settingsPageCopy)
	}
	wantCopyCodes := []string{settingsActCopyPage, settingsActCopyPick, settingsActCopyYes}
	if len(cpPage.Actions) != len(wantCopyCodes) {
		t.Errorf("page %q registers %d action codes, want %d (%v)", settingsPageCopy, len(cpPage.Actions), len(wantCopyCodes), wantCopyCodes)
	}

	env := newPhase28SettingsEnv(t)
	codes := settingsPhase29Codes()

	for _, row := range codes {
		entry, ok := settingsActions[row.code]
		if !ok || entry.pageKey != row.page {
			t.Errorf("code %q maps to %+v (registered %v), want page %q", row.code, entry, ok, row.page)
		}
		pg, _ := settingsPageByKey(row.page)
		if _, inPage := pg.Actions[row.code]; !inPage {
			t.Errorf("code %q missing from page %q Actions", row.code, row.page)
		}
		if slices.Contains(settingsCoreActions, row.code) {
			t.Errorf("code %q collides with settingsCoreActions", row.code)
		}
		// Ensure no other registered page claims this action code.
		for otherKey, otherPage := range settingsPages {
			if otherKey == row.page {
				continue
			}
			if _, dup := otherPage.Actions[row.code]; dup {
				t.Errorf("code %q collides with page %q", row.code, otherKey)
			}
		}
	}

	owner := env.newOwner(env.chat)
	privateMsg := settingsPrivateOnlyMessage(t, env, owner)
	groupMsg := env.openMenu(owner)
	lookups := func() int { return len(env.fake.callsFor("getChatMember")) }
	refused := func(name string, press func()) {
		t.Helper()
		before := lookups()
		press()
		if answer, alert := env.lastAnswer(); !alert || !strings.Contains(answer, staffMarker("common_callback_invalid_request")) {
			t.Errorf("%s answered %q (alert %v), want the invalid-request alert", name, answer, alert)
		}
		if got := lookups(); got != before {
			t.Errorf("%s made %d getChatMember calls, want 0 (refused in step 1)", name, got-before)
		}
	}

	for _, row := range codes {
		kv := append([]string{"a", row.code}, row.valid...)
		withExtra := append(slices.Clone(kv), "x", "1")
		refused(row.code+" with an extra key", func() {
			env.pressPrivate(owner, privateMsg, env.privateFields(env.chat.Id, withExtra...))
		})
		refused(row.code+" from a group menu", func() {
			env.press(owner, groupMsg, env.groupFields(owner, kv...))
		})
	}
}

// TestPhase29ReadsCreateNothing proves opening Main, Messages hub (h), and Copy Settings (cp / cg)
// on an unconfigured chat (both private and group menus) creates 0 rows across all 10 tables in scope.
func TestPhase29ReadsCreateNothing(t *testing.T) {
	tables := []struct {
		name  string
		model any
	}{
		{"chat_event_messages", &models.ChatEventMessage{}},
		{"chat_event_message_options", &models.ChatEventMessageOptions{}},
		{"warns_settings", &models.WarnSettings{}},
		{"antiflood_settings", &models.AntifloodSettings{}},
		{"blacklists", &models.BlacklistSettings{}},
		{"chat_escalation", &models.ChatEscalation{}},
		{"chat_manual_defaults", &models.ChatManualDefaults{}},
		{"chat_reason_shortcuts", &models.ChatReasonShortcut{}},
		{"chat_advanced_thresholds", &models.ChatAdvancedThresholds{}},
		{"chat_settings_access", &models.ChatSettingsAccess{}},
	}
	rows := func(chatID int64) map[string]int64 {
		out := map[string]int64{}
		for _, table := range tables {
			var n int64
			if err := db.DB.Model(table.model).Where("chat_id = ?", chatID).Count(&n).Error; err != nil {
				t.Fatalf("count %s: %v", table.name, err)
			}
			if n != 0 {
				out[table.name] = n
			}
		}
		return out
	}

	env := newPhase28SettingsEnv(t)
	cleanupCopyPageChat(t, env.chat.Id)
	db.DB.Where("chat_id = ?", env.chat.Id).Delete(&models.ChatSettingsAccess{})
	dbcache.DeleteCache(dbcache.CacheKey("chat_settings_access", env.chat.Id))
	t.Cleanup(func() {
		db.DB.Where("chat_id = ?", env.chat.Id).Delete(&models.ChatSettingsAccess{})
		db.DB.Where("chat_id = ?", env.chat.Id).Delete(&models.Chat{})
		dbcache.DeleteCache(dbcache.CacheKey("chat_settings_access", env.chat.Id))
		dbcache.ResetLocalForTest()
	})

	owner := env.newOwner(env.chat)
	// Seed a second candidate group in adminindex (not in any of the 10 settings tables) so cp/cg
	// exercise the populated picker render path too.
	candChat := env.addChat()
	cleanupCopyPageChat(t, candChat.Id)
	settingsHubSeedGroup(t, candChat, "Candidate Group", settingsHubCreator(owner))

	privateMsg := settingsPrivateOnlyMessage(t, env, owner)
	groupMsg := env.openMenu(owner)

	private := func(kv ...string) { env.pressPrivate(owner, privateMsg, env.privateFields(env.chat.Id, kv...)) }
	group := func(kv ...string) { env.press(owner, groupMsg, env.groupFields(owner, kv...)) }

	// Group menu opens Main, h (private-only note), cp (private-only note), Back.
	group("a", settingsActOpen, "p", settingsPageMessages)
	group("a", settingsActBack)
	group("a", settingsActOpen, "p", settingsPageCopy)
	group("a", settingsActBack)

	// Private menu opens Main, Messages hub (h), Copy Settings (cp), Copy Settings page 1 (cg), Back.
	private("a", settingsActBack)
	private("a", settingsActOpen, "p", settingsPageMessages)
	private("a", settingsActBack)
	private("a", settingsActOpen, "p", settingsPageCopy)
	private("a", settingsActCopyPage, "v", "1")
	private("a", settingsActBack)

	if got := rows(env.chat.Id); len(got) != 0 {
		t.Errorf("opening Phase 29 screens on unconfigured chat created rows %v, want none", got)
	}
	if got := rows(candChat.Id); len(got) != 0 {
		t.Errorf("opening Phase 29 screens created rows on candidate chat %v, want none", got)
	}
}

// TestPhase29StaffPrivacyAndLogs proves neither Plain tone nor Copy Settings ever reveals a Staff
// Group title/ID, that target refusals are uniformly opaque, and that a full log capture across
// Plain tone toggles, plain renders, and Copy Settings ask/confirm/refusal/copy contains ZERO
// group titles, user names, message texts, file IDs, button URLs, appeal URLs, shortcut reasons,
// or Redis copy tokens.
func TestPhase29StaffPrivacyAndLogs(t *testing.T) {
	const (
		sentinelSrcTitle   = "SECRET-SRC-GROUP-P29"
		sentinelDstTitle   = "SECRET-DST-GROUP-P29"
		sentinelStaffTitle = "SECRET-STAFF-HQ-P29"
		sentinelOwnerName  = "SECRET-OWNER-P29"
		sentinelMsgText    = "SECRET-MSG-TEXT-P29"
		sentinelFileID     = "SECRET-FILE-ID-P29"
		sentinelBtnURL     = "https://secret-p29.example.org/btn"
		sentinelAppealURL  = "https://secret-p29.example.org/appeal"
		sentinelShortcut   = "SECRET-REASON-P29"
	)

	ctx := context.Background()
	env := newPhase28SettingsEnv(t)
	titleClient := &copyTitleBotClient{lockdownFake: env.fake, titles: map[int64]string{}}
	env.bot.BotClient = titleClient

	src := env.chat
	src.Title = sentinelSrcTitle
	env.chat.Title = sentinelSrcTitle
	dst := env.addChat()
	dst.Title = sentinelDstTitle
	cleanupCopyPageChat(t, src.Id)
	cleanupCopyPageChat(t, dst.Id)

	staffChat := settingsStaffTestOwnedStaffGroup(env, sentinelStaffTitle)
	t.Cleanup(func() { cleanupCopyPageChat(t, staffChat.Id) })
	linkLockdownGroupTo(env.lockdownEnv, staffChat)

	owner := gotgbot.User{Id: uniqueLinkOwnerID(), FirstName: sentinelOwnerName}
	t.Cleanup(func() {
		db.DB.Where("user_id = ?", owner.Id).Delete(&models.User{})
		dbcache.DeleteCache(dbcache.CacheKey("user", owner.Id))
		dbcache.ResetLocalForTest()
	})
	for _, cid := range []int64{src.Id, dst.Id, staffChat.Id} {
		env.fake.setMemberRaw(cid, owner.Id, copyCreatorJSON(owner.Id))
		env.fake.setCreator(cid, owner.Id)
	}
	titleClient.setTitle(src.Id, sentinelSrcTitle)
	titleClient.setTitle(dst.Id, sentinelDstTitle)
	titleClient.setTitle(staffChat.Id, sentinelStaffTitle)

	settingsHubSeedGroup(t, src, sentinelSrcTitle, settingsHubCreator(owner))
	settingsHubSeedGroup(t, dst, sentinelDstTitle, settingsHubCreator(owner))
	settingsHubSeedGroup(t, staffChat, sentinelStaffTitle, settingsHubCreator(owner))

	// Seed source group with sentinel custom message (text + photo file_id + button URL),
	// sentinel appeal URL, and sentinel reason shortcut.
	if _, err := eventmsg.ReplaceAllValidated(ctx, src.Id, []eventmsg.Message{
		{
			Event:     eventmsg.EventBan,
			Dest:      eventmsg.DestGroup,
			Text:      sentinelMsgText,
			FileID:    sentinelFileID,
			MediaType: db.PHOTO,
			Buttons:   []models.Button{{Name: "Btn", Url: sentinelBtnURL}},
		},
	}, eventmsg.Options{}, owner.Id); err != nil {
		t.Fatalf("seed source eventmsg: %v", err)
	}
	if err := modtuning.ReplaceDefaults(ctx, src.Id, modtuning.Defaults{
		AppealURL: sentinelAppealURL,
	}, owner.Id); err != nil {
		t.Fatalf("seed source defaults: %v", err)
	}
	if err := modtuning.ReplaceShortcuts(ctx, src.Id, []modtuning.ShortcutEntry{
		{Word: "sp", Reason: sentinelShortcut},
	}, owner.Id); err != nil {
		t.Fatalf("seed source shortcuts: %v", err)
	}

	// Capture every logrus line across the entire Phase 29 flow.
	logger := log.StandardLogger()
	prevOut := logger.Out
	prevLevel := logger.GetLevel()
	logBuf := &phase28LogBuffer{}
	logger.SetOutput(logBuf)
	logger.SetLevel(log.DebugLevel)
	t.Cleanup(func() {
		logger.SetOutput(prevOut)
		logger.SetLevel(prevLevel)
	})

	msgID := settingsPrivateOnlyMessage(t, env, owner)
	press := func(kv ...string) { env.pressPrivate(owner, msgID, env.privateFields(src.Id, kv...)) }

	// 1. Toggle Plain tone ON -> OFF -> ON on Messages hub (ht).
	press("a", settingsActOpen, "p", settingsPageMessages)
	press("a", settingsActMsgPlainTone, "v", "1")
	press("a", settingsActMsgPlainTone, "v", "0")
	press("a", settingsActMsgPlainTone, "v", "1")

	// Render a plain-tone default and confirm it never mentions the Staff Group.
	plainTr := i18n.MustNewTranslator("en")
	plainSample := banPlainDefaultReply(plainTr, eventDefaultInput{
		Target: gotgbot.User{Id: 9001, FirstName: "TargetUser"},
		Reason: "Rule violation",
	})

	// 2. Open Copy Settings (cp) and attempt ck on Staff Group -> uniform opaque refusal.
	press("a", settingsActOpen, "p", settingsPageCopy)
	press("a", settingsActCopyPick, "t", strconv.FormatInt(staffChat.Id, 10))
	staffRefusalAns, staffRefusalAlert := copyLastAnswer(t, env)
	wantOpaque := staffMarker("settings_pick_unavailable")
	if !staffRefusalAlert || staffRefusalAns != wantOpaque {
		t.Errorf("ck on Staff Group answer = (%q, alert=%v), want exact opaque (%q, true)", staffRefusalAns, staffRefusalAlert, wantOpaque)
	}

	// Compare against a non-existent / bot-kicked target refusal to prove uniformity.
	inaccessible := env.addChat()
	cleanupCopyPageChat(t, inaccessible.Id)
	titleClient.setMethodFailure("getChatMember", inaccessible.Id, &gotgbot.TelegramError{Code: 403, Description: "Forbidden: bot was kicked"})
	press("a", settingsActCopyPick, "t", strconv.FormatInt(inaccessible.Id, 10))
	inaccAns, inaccAlert := copyLastAnswer(t, env)
	if inaccAns != staffRefusalAns || inaccAlert != staffRefusalAlert {
		t.Errorf("target refusals differ: staff=(%q,%v) vs 403=(%q,%v)", staffRefusalAns, staffRefusalAlert, inaccAns, inaccAlert)
	}

	// 3. Pick valid target group (ck) -> capture 16-char Redis copy token -> press Yes (cy).
	press("a", settingsActCopyPick, "t", strconv.FormatInt(dst.Id, 10))
	_, cardKB := env.lastEdit(owner.Id, msgID)
	yesBtn, ok := settingsAdvFindButton(t, cardKB, map[string]string{"a": settingsActCopyYes})
	if !ok {
		t.Fatalf("confirm card missing Yes (cy) button")
	}
	copyToken := settingsButtonFields(t, yesBtn)["k"]
	if !settingsCopyTokenValid(copyToken) {
		t.Fatalf("unexpected copy token %q", copyToken)
	}

	env.pressRaw(settingsPrivateChat(owner), owner, msgID, yesBtn.CallbackData)
	copiedText, _ := env.lastEdit(owner.Id, msgID)
	if !strings.Contains(copiedText, staffMarker("settings_notice_copied")) {
		t.Fatalf("cy did not show copied notice:\n%s", copiedText)
	}

	// Verify the sentinel payload was genuinely copied to dst so the log sweep is meaningful.
	dstMsgs, err := eventmsg.GetFresh(ctx, dst.Id)
	if err != nil {
		t.Fatalf("dst eventmsg.GetFresh: %v", err)
	}
	if row := dstMsgs.Resolve(eventmsg.EventBan, eventmsg.DestGroup); row.Text != sentinelMsgText || row.FileID != sentinelFileID || !dstMsgs.Options.PlainTone {
		t.Fatalf("dst eventmsg not copied as expected: row=%+v opts=%+v", row, dstMsgs.Options)
	}

	// 4. Verify UI output never leaks Staff Group title or ID anywhere (screens, confirm card, notices, answers, plain render).
	var seen []string
	seen = append(seen, plainSample)
	for _, edit := range env.edits(owner.Id, msgID) {
		seen = append(seen, fmt.Sprint(edit.Params["text"]))
		for _, button := range settingsAllButtons(staffKeyboardOf(edit.Params["reply_markup"])) {
			seen = append(seen, button.Text, button.CallbackData, button.Url)
		}
	}
	for _, sent := range env.fake.sentTo(owner.Id) {
		seen = append(seen, fmt.Sprint(sent.Params["text"]))
		for _, button := range settingsAllButtons(staffKeyboardOf(sent.Params["reply_markup"])) {
			seen = append(seen, button.Text, button.CallbackData, button.Url)
		}
	}
	for _, sent := range env.fake.sentTo(src.Id) {
		seen = append(seen, fmt.Sprint(sent.Params["text"]))
		for _, button := range settingsAllButtons(staffKeyboardOf(sent.Params["reply_markup"])) {
			seen = append(seen, button.Text, button.CallbackData, button.Url)
		}
	}
	for _, answer := range env.fake.callsFor("answerCallbackQuery") {
		seen = append(seen, fmt.Sprint(answer.Params["text"]))
	}
	everything := strings.Join(seen, "\n")

	staffDigits := strings.TrimPrefix(strconv.FormatInt(staffChat.Id, 10), "-")
	for _, leak := range []string{
		sentinelStaffTitle,
		html.EscapeString(sentinelStaffTitle),
		staffDigits,
	} {
		if strings.Contains(everything, leak) {
			t.Errorf("Phase 29 UI output leaked Staff Group %q", leak)
		}
	}

	// 5. Verify captured logrus output was active and contains ZERO sentinels or the copy token.
	logs := logBuf.String()
	if !strings.Contains(logs, "[CopySettings] completed copy") {
		t.Errorf("expected CopySettings completion log line so log capture is proven active; logs:\n%s", logs)
	}
	for _, secret := range []string{
		sentinelSrcTitle,
		sentinelDstTitle,
		sentinelStaffTitle,
		sentinelOwnerName,
		sentinelMsgText,
		sentinelFileID,
		sentinelBtnURL,
		sentinelAppealURL,
		sentinelShortcut,
		copyToken,
	} {
		if strings.Contains(logs, secret) {
			t.Errorf("captured logrus output leaked %q:\n%s", secret, logs)
		}
	}

	// 6. Staff Group hides cp from Main in both private and group /settings.
	beforeStaffPriv := len(env.fake.sentTo(owner.Id))
	env.sendPrivate(owner, fmt.Sprintf("/start settings_%d", staffChat.Id))
	staffPrivSent := env.fake.sentTo(owner.Id)
	if len(staffPrivSent) <= beforeStaffPriv {
		t.Fatalf("Staff Group private /start sent no message")
	}
	staffPrivateMain := staffPrivSent[len(staffPrivSent)-1]
	checkStaffMain := func(where, text string, kb [][]gotgbot.InlineKeyboardButton) {
		t.Helper()
		if containsMarker(text, "settings_page_copy") {
			t.Errorf("%s Staff Group Main text unexpectedly contains settings_page_copy:\n%s", where, text)
		}
		for _, btn := range settingsAllButtons(kb) {
			if !strings.HasPrefix(btn.CallbackData, settingsCallbackNamespace+"|") {
				continue
			}
			fields := settingsButtonFields(t, btn)
			if fields["a"] == settingsActOpen && fields["p"] == settingsPageCopy {
				t.Errorf("%s Staff Group Main unexpectedly has button for page cp: %+v", where, btn)
			}
		}
	}
	checkStaffMain("private", fmt.Sprint(staffPrivateMain.Params["text"]), staffKeyboardOf(staffPrivateMain.Params["reply_markup"]))

	env.fake.setBotRole(staffChat.Id, staffRoleAdmin)
	beforeStaffSent := len(env.fake.sentTo(staffChat.Id))
	staffSettingsMsg := gotgbot.Message{
		MessageId: env.updateID(),
		Chat:      staffChat,
		From:      &owner,
		Text:      "/settings",
	}
	if err := env.dispatcher.ProcessUpdate(env.bot, &gotgbot.Update{
		UpdateId: env.updateID(),
		Message:  &staffSettingsMsg,
	}, nil); err != nil {
		t.Fatalf("Staff Group /settings error: %v", err)
	}
	staffSent := env.fake.sentTo(staffChat.Id)
	if len(staffSent) <= beforeStaffSent {
		t.Fatalf("Staff Group /settings sent no message")
	}
	lastStaff := staffSent[len(staffSent)-1]
	checkStaffMain("group", fmt.Sprint(lastStaff.Params["text"]), staffKeyboardOf(lastStaff.Params["reply_markup"]))
}
