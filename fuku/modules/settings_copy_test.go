//go:build testtools

package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"math"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/redis/go-redis/v9"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/adminindex"
	"github.com/uasneppy/fuku_robot/fuku/db/advthresholds"
	"github.com/uasneppy/fuku_robot/fuku/db/antiflood"
	"github.com/uasneppy/fuku_robot/fuku/db/blacklists"
	dbcache "github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/escalation"
	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/db/modtuning"
	"github.com/uasneppy/fuku_robot/fuku/db/warns"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
)

// copyTitleBotClient wraps lockdownFake so getChat returns custom per-chat titles (including
// HTML-special characters) without int64 precision loss, and allows injecting per-(method, chat)
// failures that take precedence even over memberRaw entries.
type copyTitleBotClient struct {
	*lockdownFake
	mu       sync.Mutex
	titles   map[int64]string
	failures map[string]error
}

func (c *copyTitleBotClient) setTitle(chatID int64, title string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.titles[chatID] = title
}

func (c *copyTitleBotClient) setMethodFailure(method string, chatID int64, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failures == nil {
		c.failures = map[string]error{}
	}
	c.failures[fmt.Sprintf("%s:%d", method, chatID)] = err
}

func (c *copyTitleBotClient) RequestWithContext(
	ctx context.Context,
	token, method string,
	params map[string]any,
	opts *gotgbot.RequestOpts,
) (json.RawMessage, error) {
	chatID := staffParamInt(params, "chat_id")
	c.mu.Lock()
	forcedErr := c.failures[fmt.Sprintf("%s:%d", method, chatID)]
	c.mu.Unlock()
	if forcedErr != nil {
		c.record(method, params)
		return nil, forcedErr
	}
	raw, err := c.lockdownFake.RequestWithContext(ctx, token, method, params, opts)
	if err != nil || method != "getChat" {
		return raw, err
	}
	c.mu.Lock()
	title, ok := c.titles[chatID]
	c.mu.Unlock()
	if !ok {
		return raw, nil
	}
	var payload map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if jsonErr := dec.Decode(&payload); jsonErr != nil {
		return raw, nil
	}
	payload["title"] = title
	patched, jsonErr := json.Marshal(payload)
	if jsonErr != nil {
		return raw, nil
	}
	return patched, nil
}

func copyAdminJSON(userID int64, canChangeInfo, canRestrict bool) string {
	return fmt.Sprintf(
		`{"status":"administrator","user":{"id":%d,"is_bot":false,"first_name":"Admin%d"},"can_be_edited":false,`+
			`"is_anonymous":false,"can_manage_chat":true,"can_delete_messages":false,"can_manage_video_chats":false,`+
			`"can_restrict_members":%t,"can_promote_members":false,"can_change_info":%t,"can_invite_users":false}`,
		userID, userID, canRestrict, canChangeInfo)
}

func copyMemberJSON(userID int64) string {
	return fmt.Sprintf(`{"status":"member","user":{"id":%d,"is_bot":false,"first_name":"Member%d"}}`, userID, userID)
}

func copyCreatorJSON(userID int64) string {
	return fmt.Sprintf(`{"status":"creator","user":{"id":%d,"is_bot":false,"first_name":"Creator%d"},"is_anonymous":false}`, userID, userID)
}

func copyLastAnswer(t *testing.T, env *settingsEnv) (string, bool) {
	t.Helper()
	calls := env.fake.callsFor("answerCallbackQuery")
	if len(calls) == 0 {
		t.Fatal("answerCallbackQuery was never called")
	}
	last := calls[len(calls)-1]
	text := ""
	if raw, ok := last.Params["text"]; ok && raw != nil {
		text = fmt.Sprint(raw)
	}
	alert := fmt.Sprint(last.Params["show_alert"]) == "true"
	return text, alert
}

func cleanupCopyPageChat(t *testing.T, chatID int64) {
	t.Helper()
	clean := func() {
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatEventMessage{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatEventMessageOptions{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.WarnSettings{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.AntifloodSettings{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.BlacklistSettings{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatEscalation{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatManualDefaults{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatReasonShortcut{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatAdvancedThresholds{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.StaffGroup{})
		db.DB.Where("group_chat_id = ?", chatID).Delete(&models.StaffGroupLink{})

		dbcache.DeleteCache(eventmsg.CacheKey(chatID))
		dbcache.DeleteCache(dbcache.CacheKey("warn_settings", chatID))
		dbcache.DeleteCache(dbcache.CacheKey("antiflood", chatID))
		dbcache.DeleteCache(dbcache.CacheKey("blacklist", chatID))
		dbcache.DeleteCache(escalation.CacheKey(chatID))
		dbcache.DeleteCache(modtuning.DefaultsCacheKey(chatID))
		dbcache.DeleteCache(modtuning.ShortcutsCacheKey(chatID))
		dbcache.DeleteCache(advthresholds.CacheKey(chatID))
	}
	clean()
	t.Cleanup(clean)
}

func copySeedStaffGroup(t *testing.T, chatID, ownerID int64) {
	t.Helper()
	row := models.StaffGroup{ChatID: chatID, OwnerUserID: ownerID, Title: "Staff HQ"}
	if err := db.DB.Create(&row).Error; err != nil {
		t.Fatalf("seed staff_groups(%d): %v", chatID, err)
	}
	t.Cleanup(func() {
		db.DB.Where("chat_id = ?", chatID).Delete(&models.StaffGroup{})
	})
}

func TestSettingsCopyPage(t *testing.T) {
	t.Run("open_private_and_group_note_and_staff_hidden", func(t *testing.T) {
		env := newPhase28SettingsEnv(t)
		owner := env.newOwner(env.chat)
		cleanupCopyPageChat(t, env.chat.Id)

		// Seed source group, 10 ordinary candidate groups, and 1 Staff Group in adminindex.
		settingsHubSeedGroup(t, env.chat, "Source Group", settingsHubCreator(owner))
		var targetIDs []int64
		for i := 1; i <= 10; i++ {
			c := env.addChat()
			cleanupCopyPageChat(t, c.Id)
			settingsHubSeedGroup(t, c, fmt.Sprintf("Group %02d", i), settingsHubCreator(owner))
			targetIDs = append(targetIDs, c.Id)
		}
		staffChat := env.addChat()
		cleanupCopyPageChat(t, staffChat.Id)
		settingsHubSeedGroup(t, staffChat, "00 Staff HQ", settingsHubCreator(owner))
		copySeedStaffGroup(t, staffChat.Id, owner.Id)

		// 1. Group menu shows Private only on Main, group note when opened, and refuses forged actions with 0 getChatMember calls.
		var hubCalls atomic.Int32
		origHub := settingsCopyHubLookup
		settingsCopyHubLookup = func(ctx context.Context, userID int64) ([]adminindex.ListedGroup, error) {
			hubCalls.Add(1)
			return origHub(ctx, userID)
		}
		t.Cleanup(func() { settingsCopyHubLookup = origHub })

		groupMsg := env.openMenu(owner)
		groupMain, _ := env.lastEdit(env.chat.Id, groupMsg)
		if line, ok := settingsMainStateLine(groupMain, "settings_page_copy"); !ok || !strings.Contains(line, staffMarker("settings_state_private_only")) {
			t.Errorf("group Main copy line = %q (ok=%v), want settings_state_private_only", line, ok)
		}
		env.press(owner, groupMsg, env.groupFields(owner, "a", settingsActOpen, "p", settingsPageCopy))
		noteText, _ := env.lastEdit(env.chat.Id, groupMsg)
		if !strings.Contains(noteText, staffMarker("settings_copy_group_note")) {
			t.Errorf("group cp screen = %q, want settings_copy_group_note", noteText)
		}
		if hubCalls.Load() != 0 {
			t.Errorf("group menu called hub lookup %d times, want 0", hubCalls.Load())
		}

		// Step-1 refusals: group-menu cg/ck/cy (using u), forged self-copy t==c, bad token k -> 0 getChatMember calls.
		beforeMembers := len(env.fake.callsFor("getChatMember"))
		for _, bad := range []struct {
			chat   gotgbot.Chat
			fields map[string]string
		}{
			{env.chat, env.groupFields(owner, "a", settingsActCopyPage, "v", "1")},
			{env.chat, env.groupFields(owner, "a", settingsActCopyPick, "t", strconv.FormatInt(targetIDs[0], 10))},
			{env.chat, env.groupFields(owner, "a", settingsActCopyYes, "k", "0123456789abcdef")},
			{settingsPrivateChat(owner), env.privateFields(env.chat.Id, "a", settingsActCopyPick, "t", strconv.FormatInt(env.chat.Id, 10))},
			{settingsPrivateChat(owner), env.privateFields(env.chat.Id, "a", settingsActCopyPick, "t", "12345")},
			{settingsPrivateChat(owner), env.privateFields(env.chat.Id, "a", settingsActCopyYes, "k", "too_short")},
			{settingsPrivateChat(owner), env.privateFields(env.chat.Id, "a", settingsActCopyPage, "v", "0")},
		} {
			data := encodeCallbackData(settingsCallbackNamespace, bad.fields)
			env.pressRaw(bad.chat, owner, groupMsg, data)
			ans, alert := copyLastAnswer(t, env)
			if !alert || !strings.Contains(ans, staffMarker("common_callback_invalid_request")) {
				t.Errorf("forged press %v answer = (%q, alert=%v), want common_callback_invalid_request alert", bad.fields, ans, alert)
			}
		}
		if got := len(env.fake.callsFor("getChatMember")) - beforeMembers; got != 0 {
			t.Errorf("step-1 forged presses made %d getChatMember calls, want 0", got)
		}

		// 2. Private menu shows static Main state, opens paginated picker excluding source & Staff Group.
		privMsg := env.updateID()
		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActBack))
		privMain, _ := env.lastEdit(owner.Id, privMsg)
		if line, ok := settingsMainStateLine(privMain, "settings_page_copy"); !ok || !strings.Contains(line, staffMarker("settings_state_copy")) {
			t.Errorf("private Main copy line = %q (ok=%v), want settings_state_copy", line, ok)
		}
		if hubCalls.Load() != 0 {
			t.Errorf("private Main called hub lookup %d times, want 0", hubCalls.Load())
		}

		env.pressPrivate(owner, privMsg, env.privateFields(env.chat.Id, "a", settingsActOpen, "p", settingsPageCopy))
		p1Text, p1KB := env.lastEdit(owner.Id, privMsg)
		if !strings.Contains(p1Text, staffMarker("settings_copy_intro")) || !strings.Contains(p1Text, staffMarker("staff_panel_page")) {
			t.Errorf("page 1 text missing intro/page marker:\n%s", p1Text)
		}
		var page1Targets []string
		for _, btn := range settingsAllButtons(p1KB) {
			f := settingsButtonFields(t, btn)
			if f["a"] == settingsActCopyPick {
				page1Targets = append(page1Targets, f["t"])
				if f["t"] == strconv.FormatInt(env.chat.Id, 10) {
					t.Error("picker offered source chat itself")
				}
				if f["t"] == strconv.FormatInt(staffChat.Id, 10) {
					t.Error("picker offered Staff Group as target")
				}
			}
		}
		if len(page1Targets) != settingsHubPageSize {
			t.Errorf("page 1 offered %d targets, want %d", len(page1Targets), settingsHubPageSize)
		}
		nextBtn, ok := settingsAdvFindButton(t, p1KB, map[string]string{"a": settingsActCopyPage, "v": "2"})
		if !ok {
			t.Fatalf("page 1 missing Next button (cg v=2)")
		}
		env.pressRaw(settingsPrivateChat(owner), owner, privMsg, nextBtn.CallbackData)
		_, p2KB := env.lastEdit(owner.Id, privMsg)
		var page2Targets int
		for _, btn := range settingsAllButtons(p2KB) {
			if settingsButtonFields(t, btn)["a"] == settingsActCopyPick {
				page2Targets++
			}
		}
		if page2Targets != 2 {
			t.Errorf("page 2 offered %d targets, want 2", page2Targets)
		}
		if _, ok := settingsAdvFindButton(t, p2KB, map[string]string{"a": settingsActCopyPage, "v": "1"}); !ok {
			t.Error("page 2 missing Prev button (cg v=1)")
		}

		// 3. Empty roster renders settings_copy_empty.
		lonely := gotgbot.User{Id: owner.Id + 99, FirstName: "Lonely"}
		env.fake.setMemberRaw(env.chat.Id, lonely.Id, copyCreatorJSON(lonely.Id))
		env.fake.setCreator(env.chat.Id, lonely.Id)
		lonelyMsg := env.updateID()
		env.pressPrivate(lonely, lonelyMsg, env.privateFields(env.chat.Id, "a", settingsActOpen, "p", settingsPageCopy))
		emptyText, _ := env.lastEdit(lonely.Id, lonelyMsg)
		if !strings.Contains(emptyText, staffMarker("settings_copy_empty")) {
			t.Errorf("empty picker text = %q, want settings_copy_empty", emptyText)
		}

		// 4. Staff Group hides cp from Main and refuses op/ck/cy.
		env.fake.setMemberRaw(staffChat.Id, owner.Id, copyCreatorJSON(owner.Id))
		env.fake.setCreator(staffChat.Id, owner.Id)
		staffPrivMsg := env.updateID()
		env.pressPrivate(owner, staffPrivMsg, env.privateFields(staffChat.Id, "a", settingsActBack))
		staffMainText, staffMainKB := env.lastEdit(owner.Id, staffPrivMsg)
		if _, hasLine := settingsMainStateLine(staffMainText, "settings_page_copy"); hasLine {
			t.Errorf("Staff Group Main still shows settings_page_copy line")
		}
		if _, hasBtn := settingsPageButton(t, staffMainKB, settingsPageCopy); hasBtn {
			t.Errorf("Staff Group Main still has cp button")
		}
		for _, fields := range []map[string]string{
			env.privateFields(staffChat.Id, "a", settingsActOpen, "p", settingsPageCopy),
			env.privateFields(staffChat.Id, "a", settingsActCopyPick, "t", strconv.FormatInt(targetIDs[0], 10)),
			env.privateFields(staffChat.Id, "a", settingsActCopyYes, "k", "0123456789abcdef"),
		} {
			env.pressPrivate(owner, staffPrivMsg, fields)
			ans, alert := copyLastAnswer(t, env)
			if !alert || !strings.Contains(ans, staffMarker("common_callback_invalid_request")) {
				t.Errorf("Staff Group %v answer = (%q, alert=%v), want invalid request alert", fields, ans, alert)
			}
		}
	})

	t.Run("confirm_card_names_all_areas_and_executes_copy", func(t *testing.T) {
		ctx := context.Background()
		env := newPhase28SettingsEnv(t)
		titleClient := &copyTitleBotClient{lockdownFake: env.fake, titles: map[int64]string{}}
		env.bot.BotClient = titleClient

		src := env.chat
		dst := env.addChat()
		cleanupCopyPageChat(t, src.Id)
		cleanupCopyPageChat(t, dst.Id)

		owner := env.newOwner(src)
		env.fake.setMemberRaw(dst.Id, owner.Id, copyCreatorJSON(owner.Id))
		env.fake.setCreator(dst.Id, owner.Id)

		srcTitle := "Src <script>& Co"
		dstTitle := "Dst <b>& Ltd"
		titleClient.setTitle(src.Id, srcTitle)
		titleClient.setTitle(dst.Id, dstTitle)

		settingsHubSeedGroup(t, src, srcTitle, settingsHubCreator(owner))
		settingsHubSeedGroup(t, dst, dstTitle, settingsHubCreator(owner))

		// Seed all 6 areas on source group.
		if _, err := eventmsg.ReplaceAllValidated(ctx, src.Id, []eventmsg.Message{
			{Event: eventmsg.EventBan, Dest: eventmsg.DestGroup, Text: "Custom ban notice", Silent: true},
		}, eventmsg.Options{PMDetails: true, PMLog: true, PlainTone: true}, owner.Id); err != nil {
			t.Fatalf("seed source eventmsg: %v", err)
		}
		if err := warns.SetWarnModeContext(ctx, src.Id, "tban", 7200); err != nil {
			t.Fatalf("seed source warn mode: %v", err)
		}
		if err := warns.SetWarnExpiryContext(ctx, src.Id, 14); err != nil {
			t.Fatalf("seed source warn expiry: %v", err)
		}
		if err := antiflood.SetFloodActionPreserveOffContext(ctx, src.Id, "tmute", 600); err != nil {
			t.Fatalf("seed source flood action: %v", err)
		}
		if err := blacklists.AddBlacklistTimedContext(ctx, src.Id, "srcword", "tban", 3600); err != nil {
			t.Fatalf("seed source blacklist: %v", err)
		}
		if err := escalation.ReplaceLadder(ctx, src.Id, escalation.Ladder{
			Enabled:    true,
			Steps:      []escalation.Step{{Action: "mute"}, {Action: "tban", Secs: 86400}},
			ForgetDays: 30,
		}, owner.Id); err != nil {
			t.Fatalf("seed source escalation: %v", err)
		}
		if err := modtuning.ReplaceDefaults(ctx, src.Id, modtuning.Defaults{
			DefaultBanSecs:  86400,
			DefaultMuteSecs: 3600,
			ReasonRequired:  true,
			AppealURL:       "https://example.com/appeal",
		}, owner.Id); err != nil {
			t.Fatalf("seed source defaults: %v", err)
		}
		if err := modtuning.ReplaceShortcuts(ctx, src.Id, []modtuning.ShortcutEntry{
			{Word: "spam", Reason: "Unsolicited spam"},
		}, owner.Id); err != nil {
			t.Fatalf("seed source shortcuts: %v", err)
		}
		if err := advthresholds.ReplaceAll(ctx, src.Id, advthresholds.Thresholds{RaidJoinWindowSecs: 180}, owner.Id); err != nil {
			t.Fatalf("seed source advthresholds: %v", err)
		}

		// Seed one existing blacklist word on target; leave antiflood absent on target (Limit == 0).
		if err := blacklists.AddBlacklistTimedContext(ctx, dst.Id, "dstword", "warn", 0); err != nil {
			t.Fatalf("seed target blacklist: %v", err)
		}

		privMsg := env.updateID()
		env.pressPrivate(owner, privMsg, env.privateFields(src.Id, "a", settingsActCopyPick, "t", strconv.FormatInt(dst.Id, 10)))
		cardText, cardKB := env.lastEdit(owner.Id, privMsg)

		// Verify escaped source & target titles and all required confirm card sections.
		for _, want := range []string{
			html.EscapeString(srcTitle),
			html.EscapeString(dstTitle),
			staffMarker("settings_copy_confirm_head"),
			staffMarker("settings_copy_confirm_areas"),
			staffMarker("settings_copy_confirm_keep"),
		} {
			if !strings.Contains(cardText, want) {
				t.Errorf("confirm card missing %q:\n%s", want, cardText)
			}
		}
		if strings.Contains(cardText, "<script>") {
			t.Errorf("confirm card contains unescaped <script>:\n%s", cardText)
		}

		yesBtn, ok := settingsAdvFindButton(t, cardKB, map[string]string{"a": settingsActCopyYes})
		if !ok {
			t.Fatalf("confirm card missing Yes (cy) button")
		}
		noBtn, ok := settingsAdvFindButton(t, cardKB, map[string]string{"a": settingsActCancel, "p": settingsPageCopy})
		if !ok {
			t.Fatalf("confirm card missing Cancel (no p=cp) button")
		}
		token := settingsButtonFields(t, yesBtn)["k"]
		if !settingsCopyTokenValid(token) {
			t.Fatalf("confirm card token %q is not a valid 16-char base64url token", token)
		}
		redisKey := settingsCopyTokenKey(token)
		if !env.redis.Exists(redisKey) {
			t.Fatalf("Redis key %q missing after ck", redisKey)
		}
		if ttl := env.redis.TTL(redisKey); ttl <= 0 || ttl > settingsCardLifetime {
			t.Errorf("Redis TTL = %v, want (0, %v]", ttl, settingsCardLifetime)
		}

		// Cancel returns to picker and writes nothing to target.
		env.pressRaw(settingsPrivateChat(owner), owner, privMsg, noBtn.CallbackData)
		cancelText, _ := env.lastEdit(owner.Id, privMsg)
		if !strings.Contains(cancelText, staffMarker("settings_copy_intro")) {
			t.Errorf("Cancel did not return to cp picker:\n%s", cancelText)
		}
		if th, _ := advthresholds.GetFresh(ctx, dst.Id); th.RaidJoinWindowSecs != 0 {
			t.Fatalf("Cancel unexpectedly modified target advthresholds: %+v", th)
		}

		// Press Yes with the valid token -> executes copy, deletes Redis key, mirrors all 6 areas.
		env.pressRaw(settingsPrivateChat(owner), owner, privMsg, yesBtn.CallbackData)
		if env.redis.Exists(redisKey) {
			t.Errorf("Redis key %q still exists after cy", redisKey)
		}
		doneText, _ := env.lastEdit(owner.Id, privMsg)
		if !strings.Contains(doneText, settingsNoticeOKIcon) ||
			!strings.Contains(doneText, staffMarker("settings_notice_copied")) ||
			!strings.Contains(doneText, html.EscapeString(dstTitle)) {
			t.Errorf("post-copy screen missing copied notice/escaped title:\n%s", doneText)
		}

		// Verify all 6 areas mirrored onto target, source untouched, and antiflood Limit stayed 0.
		dstMsgs, err := eventmsg.GetFresh(ctx, dst.Id)
		if err != nil {
			t.Fatalf("dst eventmsg.GetFresh: %v", err)
		}
		banRow := dstMsgs.Resolve(eventmsg.EventBan, eventmsg.DestGroup)
		if banRow.Text != "Custom ban notice" ||
			!banRow.Silent ||
			!dstMsgs.Options.PMDetails || !dstMsgs.Options.PMLog || !dstMsgs.Options.PlainTone {
			t.Errorf("dst eventmsg mismatch: row=%+v opts=%+v", banRow, dstMsgs.Options)
		}
		wMode, wSecs, wExp, err := loadCopyWarnSettingsFresh(ctx, dst.Id)
		if err != nil || wMode != "tban" || wSecs != 7200 || wExp != 14 {
			t.Errorf("dst warns = (%q,%d,%d,%v), want (tban,7200,14,nil)", wMode, wSecs, wExp, err)
		}
		dstFlood, _, err := antiflood.GetFloodFreshContext(ctx, dst.Id)
		if err != nil || dstFlood.Limit != 0 || dstFlood.Action != "tmute" || dstFlood.ActionSecs != 600 {
			t.Errorf("dst flood = %+v (err=%v), want Limit=0 Action=tmute ActionSecs=600", dstFlood, err)
		}
		dstBL, err := blacklists.ListBlacklistEntriesFresh(ctx, dst.Id)
		if err != nil || len(dstBL) != 1 || dstBL[0].Word != "dstword" || dstBL[0].Action != "tban" || dstBL[0].ActionSecs != 3600 {
			t.Errorf("dst blacklists = %+v (err=%v), want single dstword with tban/3600", dstBL, err)
		}
		dstEsc, err := escalation.GetFresh(ctx, dst.Id)
		if err != nil || !dstEsc.Enabled || len(dstEsc.Steps) != 2 || dstEsc.ForgetDays != 30 {
			t.Errorf("dst escalation = %+v (err=%v), want enabled 2 steps 30d", dstEsc, err)
		}
		dstDef, err := modtuning.GetDefaultsFresh(ctx, dst.Id)
		if err != nil || dstDef.DefaultBanSecs != 86400 || dstDef.DefaultMuteSecs != 3600 || !dstDef.ReasonRequired || dstDef.AppealURL != "https://example.com/appeal" {
			t.Errorf("dst defaults = %+v (err=%v)", dstDef, err)
		}
		dstShort, err := modtuning.ListShortcutsFresh(ctx, dst.Id)
		if err != nil || len(dstShort) != 1 || dstShort[0].Word != "spam" {
			t.Errorf("dst shortcuts = %+v (err=%v)", dstShort, err)
		}
		dstAdv, err := advthresholds.GetFresh(ctx, dst.Id)
		if err != nil || dstAdv.RaidJoinWindowSecs != 180 {
			t.Errorf("dst advthresholds = %+v (err=%v)", dstAdv, err)
		}

		// Source still intact.
		srcAdv, _ := advthresholds.GetFresh(ctx, src.Id)
		if srcAdv.RaidJoinWindowSecs != 180 {
			t.Errorf("source advthresholds changed: %+v", srcAdv)
		}
	})

	t.Run("replay_expired_wrong_user_wrong_source_and_no_redis", func(t *testing.T) {
		env := newPhase28SettingsEnv(t)
		src := env.chat
		dst := env.addChat()
		otherSrc := env.addChat()
		cleanupCopyPageChat(t, src.Id)
		cleanupCopyPageChat(t, dst.Id)
		cleanupCopyPageChat(t, otherSrc.Id)

		owner := env.newOwner(src)
		otherUser := gotgbot.User{Id: owner.Id + 50, FirstName: "OtherAdmin"}
		env.fake.setMemberRaw(src.Id, otherUser.Id, copyAdminJSON(otherUser.Id, true, true))
		env.fake.setMemberRaw(dst.Id, owner.Id, copyCreatorJSON(owner.Id))
		env.fake.setCreator(dst.Id, owner.Id)
		env.fake.setMemberRaw(otherSrc.Id, owner.Id, copyCreatorJSON(owner.Id))
		env.fake.setCreator(otherSrc.Id, owner.Id)

		var applyCalls atomic.Int32
		origApply := settingsCopyApply
		settingsCopyApply = func(ctx context.Context, srcID, dstID, by int64) copySettingsResult {
			applyCalls.Add(1)
			return origApply(ctx, srcID, dstID, by)
		}
		t.Cleanup(func() { settingsCopyApply = origApply })

		privMsg := env.updateID()
		env.pressPrivate(owner, privMsg, env.privateFields(src.Id, "a", settingsActCopyPick, "t", strconv.FormatInt(dst.Id, 10)))
		_, cardKB := env.lastEdit(owner.Id, privMsg)
		yesBtn, ok := settingsAdvFindButton(t, cardKB, map[string]string{"a": settingsActCopyYes})
		if !ok {
			t.Fatal("missing Yes button")
		}
		token := settingsButtonFields(t, yesBtn)["k"]

		// 1. Wrong user presents valid token -> invalid request, 0 writes, token NOT consumed!
		env.pressPrivate(otherUser, privMsg, env.privateFields(src.Id, "a", settingsActCopyYes, "k", token))
		ans, alert := copyLastAnswer(t, env)
		if !alert || !strings.Contains(ans, staffMarker("common_callback_invalid_request")) {
			t.Errorf("wrong user answer = (%q, alert=%v), want common_callback_invalid_request", ans, alert)
		}
		if applyCalls.Load() != 0 {
			t.Errorf("wrong user triggered applyCopySettings %d times, want 0", applyCalls.Load())
		}
		if !env.redis.Exists(settingsCopyTokenKey(token)) {
			t.Error("wrong user press invalidated owner's Redis token")
		}

		// 2. Wrong source chat presents valid token -> invalid request, 0 writes, token NOT consumed!
		env.pressPrivate(owner, privMsg, env.privateFields(otherSrc.Id, "a", settingsActCopyYes, "k", token))
		ans, alert = copyLastAnswer(t, env)
		if !alert || !strings.Contains(ans, staffMarker("common_callback_invalid_request")) {
			t.Errorf("wrong source answer = (%q, alert=%v), want common_callback_invalid_request", ans, alert)
		}
		if applyCalls.Load() != 0 {
			t.Errorf("wrong source triggered applyCopySettings %d times, want 0", applyCalls.Load())
		}
		if !env.redis.Exists(settingsCopyTokenKey(token)) {
			t.Error("wrong source press invalidated owner's Redis token")
		}

		// 3. Real owner presses Yes -> succeeds once (applyCalls == 1).
		env.pressRaw(settingsPrivateChat(owner), owner, privMsg, yesBtn.CallbackData)
		if applyCalls.Load() != 1 {
			t.Fatalf("valid Yes called applyCopySettings %d times, want 1", applyCalls.Load())
		}

		// 4. Replaying the same Yes callback -> settings_card_expired, 0 additional writes!
		env.pressRaw(settingsPrivateChat(owner), owner, privMsg, yesBtn.CallbackData)
		ans, alert = copyLastAnswer(t, env)
		if !alert || !strings.Contains(ans, staffMarker("settings_card_expired")) {
			t.Errorf("replay answer = (%q, alert=%v), want settings_card_expired", ans, alert)
		}
		if applyCalls.Load() != 1 {
			t.Errorf("replay called applyCopySettings (total=%d), want 1", applyCalls.Load())
		}

		// 5. Expiry via miniredis FastForward -> settings_card_expired, 0 writes!
		env.pressPrivate(owner, privMsg, env.privateFields(src.Id, "a", settingsActCopyPick, "t", strconv.FormatInt(dst.Id, 10)))
		_, cardKB2 := env.lastEdit(owner.Id, privMsg)
		yesBtn2, ok := settingsAdvFindButton(t, cardKB2, map[string]string{"a": settingsActCopyYes})
		if !ok {
			t.Fatal("missing second Yes button")
		}
		env.redis.FastForward(settingsCardLifetime + time.Second)
		env.pressRaw(settingsPrivateChat(owner), owner, privMsg, yesBtn2.CallbackData)
		ans, alert = copyLastAnswer(t, env)
		if !alert || !strings.Contains(ans, staffMarker("settings_card_expired")) {
			t.Errorf("expired card answer = (%q, alert=%v), want settings_card_expired", ans, alert)
		}
		if applyCalls.Load() != 1 {
			t.Errorf("expired card called applyCopySettings (total=%d), want 1", applyCalls.Load())
		}

		// 6. Redis unavailable (nil) -> ck answers settings_copy_unavailable and draws no card.
		origRedis := settingsCopyRedisClient
		settingsCopyRedisClient = func() *redis.Client { return nil }
		t.Cleanup(func() { settingsCopyRedisClient = origRedis })

		editsBefore := len(env.edits(owner.Id, privMsg))
		env.pressPrivate(owner, privMsg, env.privateFields(src.Id, "a", settingsActCopyPick, "t", strconv.FormatInt(dst.Id, 10)))
		ans, alert = copyLastAnswer(t, env)
		if !alert || !strings.Contains(ans, staffMarker("settings_copy_unavailable")) {
			t.Errorf("no-redis ck answer = (%q, alert=%v), want settings_copy_unavailable", ans, alert)
		}
		if len(env.edits(owner.Id, privMsg)) != editsBefore {
			t.Error("no-redis ck edited message, want no confirm card drawn")
		}
	})

	t.Run("live_dual_group_auth_and_uniform_opaque_refusal", func(t *testing.T) {
		env := newPhase28SettingsEnv(t)
		titleClient := &copyTitleBotClient{lockdownFake: env.fake, titles: map[int64]string{}}
		env.bot.BotClient = titleClient

		src := env.chat
		cleanupCopyPageChat(t, src.Id)
		admin := gotgbot.User{Id: uniqueLinkOwnerID() + 8_000_000, FirstName: "TargetAdmin"}
		env.fake.setMemberRaw(src.Id, admin.Id, copyAdminJSON(admin.Id, true, true))

		var applyCalls atomic.Int32
		origApply := settingsCopyApply
		settingsCopyApply = func(ctx context.Context, srcID, dstID, by int64) copySettingsResult {
			applyCalls.Add(1)
			return origApply(ctx, srcID, dstID, by)
		}
		t.Cleanup(func() { settingsCopyApply = origApply })

		wantOpaque := staffMarker("settings_pick_unavailable")
		privMsg := env.updateID()

		type refusalCase struct {
			name   string
			mutate func(t *testing.T, targetID int64)
		}
		cases := []refusalCase{
			{
				name: "bot_not_in_target_403",
				mutate: func(_ *testing.T, targetID int64) {
					titleClient.setMethodFailure("getChatMember", targetID, &gotgbot.TelegramError{Code: 403, Description: "Forbidden: bot was kicked from the supergroup chat"})
				},
			},
			{
				name: "bot_not_in_target_400_not_found",
				mutate: func(_ *testing.T, targetID int64) {
					titleClient.setMethodFailure("getChatMember", targetID, &gotgbot.TelegramError{Code: 400, Description: "Bad Request: chat not found"})
				},
			},
			{
				name: "presser_not_admin_in_target",
				mutate: func(_ *testing.T, targetID int64) {
					env.fake.setMemberRaw(targetID, admin.Id, copyMemberJSON(admin.Id))
				},
			},
			{
				name: "target_owner_only_blocks_admin",
				mutate: func(_ *testing.T, targetID int64) {
					env.setAdminsAllowed(targetID, false)
					env.fake.listAdministrator(targetID, admin.Id)
					env.fake.setMemberRaw(targetID, admin.Id, copyAdminJSON(admin.Id, true, true))
				},
			},
			{
				name: "target_admin_missing_can_change_info",
				mutate: func(_ *testing.T, targetID int64) {
					env.setAdminsAllowed(targetID, true)
					env.fake.setMemberRaw(targetID, admin.Id, copyAdminJSON(admin.Id, false, true))
				},
			},
			{
				name: "target_admin_missing_can_restrict_members",
				mutate: func(_ *testing.T, targetID int64) {
					env.setAdminsAllowed(targetID, true)
					env.fake.setMemberRaw(targetID, admin.Id, copyAdminJSON(admin.Id, true, false))
				},
			},
			{
				name: "target_is_channel",
				mutate: func(_ *testing.T, targetID int64) {
					env.fake.setChatType(targetID, "channel")
				},
			},
			{
				name: "target_is_staff_group",
				mutate: func(t *testing.T, targetID int64) {
					copySeedStaffGroup(t, targetID, admin.Id)
				},
			},
		}

		for _, tc := range cases {
			t.Run("ck_"+tc.name, func(t *testing.T) {
				target := env.addChat()
				cleanupCopyPageChat(t, target.Id)
				secretTitle := "SecretTarget-" + tc.name
				titleClient.setTitle(target.Id, secretTitle)
				env.setAdminsAllowed(target.Id, true)
				env.fake.setMemberRaw(target.Id, admin.Id, copyAdminJSON(admin.Id, true, true))
				tc.mutate(t, target.Id)

				env.pressPrivate(admin, privMsg, env.privateFields(src.Id, "a", settingsActCopyPick, "t", strconv.FormatInt(target.Id, 10)))
				ans, alert := copyLastAnswer(t, env)
				if !alert || ans != wantOpaque {
					t.Errorf("ck %s answer = (%q, alert=%v), want exact opaque (%q, true)", tc.name, ans, alert, wantOpaque)
				}
				if strings.Contains(ans, secretTitle) {
					t.Errorf("ck %s leaked target title in %q", tc.name, ans)
				}
				if applyCalls.Load() != 0 {
					t.Errorf("ck %s invoked applyCopySettings", tc.name)
				}
			})

			t.Run("cy_"+tc.name, func(t *testing.T) {
				target := env.addChat()
				cleanupCopyPageChat(t, target.Id)
				secretTitle := "SecretTargetCY-" + tc.name
				titleClient.setTitle(target.Id, secretTitle)
				env.setAdminsAllowed(target.Id, true)
				env.fake.setMemberRaw(target.Id, admin.Id, copyAdminJSON(admin.Id, true, true))

				// Step 1: authorized ck creates a valid confirm card.
				env.pressPrivate(admin, privMsg, env.privateFields(src.Id, "a", settingsActCopyPick, "t", strconv.FormatInt(target.Id, 10)))
				_, cardKB := env.lastEdit(admin.Id, privMsg)
				yesBtn, ok := settingsAdvFindButton(t, cardKB, map[string]string{"a": settingsActCopyYes})
				if !ok {
					t.Fatalf("authorized ck did not produce Yes button")
				}

				// Step 2: target state changes before Yes is pressed!
				tc.mutate(t, target.Id)
				env.pressRaw(settingsPrivateChat(admin), admin, privMsg, yesBtn.CallbackData)
				ans, alert := copyLastAnswer(t, env)
				if !alert || ans != wantOpaque {
					t.Errorf("cy %s answer = (%q, alert=%v), want exact opaque (%q, true)", tc.name, ans, alert, wantOpaque)
				}
				if strings.Contains(ans, secretTitle) {
					t.Errorf("cy %s leaked target title in %q", tc.name, ans)
				}
				if applyCalls.Load() != 0 {
					t.Errorf("cy %s invoked applyCopySettings despite revocation", tc.name)
				}
			})
		}

		// Transient Telegram 429 / 500 on target returns settings_check_failed and writes nothing.
		for _, tgErr := range []*gotgbot.TelegramError{
			{Code: 429, Description: "Too Many Requests: retry after 5"},
			{Code: 500, Description: "Internal Server Error"},
		} {
			target := env.addChat()
			cleanupCopyPageChat(t, target.Id)
			titleClient.setMethodFailure("getChatMember", target.Id, tgErr)

			env.pressPrivate(admin, privMsg, env.privateFields(src.Id, "a", settingsActCopyPick, "t", strconv.FormatInt(target.Id, 10)))
			ans, alert := copyLastAnswer(t, env)
			if !alert || !strings.Contains(ans, staffMarker("settings_check_failed")) {
				t.Errorf("transient %d answer = (%q, alert=%v), want settings_check_failed", tgErr.Code, ans, alert)
			}
		}

		// Positive P29-02 check: non-creator admin with BOTH can_change_info & can_restrict_members in admin-allowed target succeeds.
		okTarget := env.addChat()
		cleanupCopyPageChat(t, okTarget.Id)
		env.setAdminsAllowed(okTarget.Id, true)
		env.fake.setMemberRaw(okTarget.Id, admin.Id, copyAdminJSON(admin.Id, true, true))
		env.pressPrivate(admin, privMsg, env.privateFields(src.Id, "a", settingsActCopyPick, "t", strconv.FormatInt(okTarget.Id, 10)))
		_, okKB := env.lastEdit(admin.Id, privMsg)
		yesBtn, ok := settingsAdvFindButton(t, okKB, map[string]string{"a": settingsActCopyYes})
		if !ok {
			t.Fatal("admin with both rights should get confirm card")
		}
		env.pressRaw(settingsPrivateChat(admin), admin, privMsg, yesBtn.CallbackData)
		if applyCalls.Load() != 1 {
			t.Errorf("admin with both rights applyCalls = %d, want 1", applyCalls.Load())
		}
	})

	t.Run("worst_case_64_byte_callbacks", func(t *testing.T) {
		minChat := strconv.FormatInt(math.MinInt64, 10)
		otherMinChat := strconv.FormatInt(math.MinInt64+1, 10)
		cases := []struct {
			name   string
			fields map[string]string
		}{
			{"op_cp", map[string]string{"a": settingsActOpen, "c": minChat, "p": settingsPageCopy}},
			{"cg_999", map[string]string{"a": settingsActCopyPage, "c": minChat, "v": "999"}},
			{"ck_minint64", map[string]string{"a": settingsActCopyPick, "c": minChat, "t": otherMinChat}},
			{"cy_token16", map[string]string{"a": settingsActCopyYes, "c": minChat, "k": "AbCdEfGhIj0123_-"}},
			{"no_cp", map[string]string{"a": settingsActCancel, "c": minChat, "p": settingsPageCopy}},
		}
		for _, tc := range cases {
			data := encodeCallbackData(settingsCallbackNamespace, tc.fields)
			if data == "" {
				t.Fatalf("%s failed to encode", tc.name)
			}
			if len(data) > 64 {
				t.Errorf("%s callback %q is %d bytes, exceeds Telegram 64-byte cap", tc.name, data, len(data))
			}
			decoded, ok := decodeCallbackData(data, settingsCallbackNamespace)
			if !ok {
				t.Errorf("%s callback %q failed to decode", tc.name, data)
				continue
			}
			if !settingsCheckFields(decoded.Fields["a"], decoded.Fields) {
				t.Errorf("%s callback %q rejected by settingsCheckFields", tc.name, data)
			}
		}
	})

	t.Run("all_8_languages_screen_lengths", func(t *testing.T) {
		withRealLocales(t)
		titles := map[string]string{
			"ampersands": strings.Repeat("&", 64),
			"astral":     strings.Repeat("😀", 64),
			"multibyte":  strings.Repeat("Ж", 64),
		}
		for _, lang := range []string{"en", "es", "fr", "hi", "id", "pt", "ru", "uk"} {
			t.Run(lang, func(t *testing.T) {
				tr := i18n.MustNewTranslator(lang)
				for titleKind, title := range titles {
					view := settingsView{
						Tr:         tr,
						Private:    true,
						OwnerKey:   "c",
						OwnerValue: strconv.FormatInt(math.MinInt64, 10),
						GroupID:    math.MinInt64,
						GroupTitle: title,
						OpenerName: title,
					}

					// 1. Paginated picker with 10 worst-case groups (8 on page 1 + Next).
					groups := make([]adminindex.ListedGroup, 10)
					for i := range groups {
						groups[i] = adminindex.ListedGroup{
							ChatID: math.MinInt64 + int64(i+1),
							Title:  title,
						}
					}
					picker := renderSettingsCopyPicker(view, groups, 1, "")
					if picker.Omitted != 0 {
						t.Errorf("%s/%s picker omitted %d buttons", lang, titleKind, picker.Omitted)
					}
					if n := panelUTF16Len(picker.Text); n == 0 || n > 3800 {
						t.Errorf("%s/%s picker UTF-16 len = %d, want 1..3800", lang, titleKind, n)
					}

					// 2. Empty picker.
					empty := renderSettingsCopyPicker(view, nil, 1, "")
					if empty.Omitted != 0 {
						t.Errorf("%s/%s empty picker omitted %d buttons", lang, titleKind, empty.Omitted)
					}
					if n := panelUTF16Len(empty.Text); n == 0 || n > 3800 {
						t.Errorf("%s/%s empty picker UTF-16 len = %d, want 1..3800", lang, titleKind, n)
					}

					// 3. Confirm card with worst-case source and target titles.
					confirm := renderSettingsCopyConfirm(view, title, title, math.MinInt64, math.MinInt64+1, "AbCdEfGhIj0123_-")
					if confirm.Omitted != 0 {
						t.Errorf("%s/%s confirm card omitted %d buttons", lang, titleKind, confirm.Omitted)
					}
					if n := panelUTF16Len(confirm.Text); n == 0 || n > 3800 {
						t.Errorf("%s/%s confirm card UTF-16 len = %d, want 1..3800", lang, titleKind, n)
					}

					// 4. Picker with copied notices (plain and skipped-rows variant).
					escapedTarget := html.EscapeString(title)
					for _, notice := range []string{
						settingsNoticeOK(settingsText(tr, "settings_notice_copied", i18n.TranslationParams{"target": escapedTarget})),
						settingsNoticeOK(settingsText(tr, "settings_notice_copied_skipped", i18n.TranslationParams{"target": escapedTarget, "n": 12})),
					} {
						withNotice := renderSettingsCopyPicker(view, groups, 1, notice)
						if withNotice.Omitted != 0 {
							t.Errorf("%s/%s notice picker omitted %d buttons", lang, titleKind, withNotice.Omitted)
						}
						if n := panelUTF16Len(withNotice.Text); n == 0 || n > 3800 {
							t.Errorf("%s/%s notice picker UTF-16 len = %d, want 1..3800", lang, titleKind, n)
						}
					}
				}
			})
		}
	})
}
