//go:build testtools

package modules

import (
	"context"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/advthresholds"
	"github.com/uasneppy/fuku_robot/fuku/db/antiflood"
	"github.com/uasneppy/fuku_robot/fuku/db/blacklists"
	"github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/escalation"
	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/db/modtuning"
	"github.com/uasneppy/fuku_robot/fuku/db/warns"
	utilscache "github.com/uasneppy/fuku_robot/fuku/utils/cache"
)

var copySettingsTestSeq atomic.Int64

func freshCopyChat(t *testing.T) int64 {
	t.Helper()
	chatID := -time.Now().UnixNano() - copySettingsTestSeq.Add(1)*1000
	t.Cleanup(func() {
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatEventMessage{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatEventMessageOptions{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.WarnSettings{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.Warns{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.AntifloodSettings{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.BlacklistSettings{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatEscalation{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatOffence{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatManualDefaults{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatReasonShortcut{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatAdvancedThresholds{})
		db.DB.Where("group_chat_id = ?", chatID).Delete(&models.StaffGroupLink{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ApprovedUsers{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.GreetingSettings{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.RulesSettings{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.Notes{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatFilters{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatSettingsAccess{})

		cache.DeleteCache(eventmsg.CacheKey(chatID))
		cache.DeleteCache(cache.CacheKey("warn_settings", chatID))
		cache.DeleteCache(cache.CacheKey("antiflood", chatID))
		cache.DeleteCache(cache.CacheKey("blacklist", chatID))
		cache.DeleteCache(escalation.CacheKey(chatID))
		cache.DeleteCache(modtuning.DefaultsCacheKey(chatID))
		cache.DeleteCache(modtuning.ShortcutsCacheKey(chatID))
		cache.DeleteCache(advthresholds.CacheKey(chatID))
	})
	return chatID
}

// TestEventMsgValidateWriteMatchesSettingsPage guards against any drift between
// fuku/db/eventmsg/validate_write.go (shared by Copy Settings and Phase 30 backup import) and
// the Messages settings editor definitions in settings_messages.go / event_message_pm.go.
func TestEventMsgValidateWriteMatchesSettingsPage(t *testing.T) {
	if eventmsg.MaxTextLength != settingsEditTextLimit {
		t.Errorf("MaxTextLength = %d, want %d", eventmsg.MaxTextLength, settingsEditTextLimit)
	}
	if eventmsg.MaxCaptionLength != settingsEditCaptionLimit {
		t.Errorf("MaxCaptionLength = %d, want %d", eventmsg.MaxCaptionLength, settingsEditCaptionLimit)
	}
	if eventmsg.MaxButtons != settingsEditMaxButtons {
		t.Errorf("MaxButtons = %d, want %d", eventmsg.MaxButtons, settingsEditMaxButtons)
	}
	if eventmsg.MaxRowButtons != settingsEditMaxRowButtons {
		t.Errorf("MaxRowButtons = %d, want %d", eventmsg.MaxRowButtons, settingsEditMaxRowButtons)
	}
	if eventmsg.MreqTextReserve != memberReqNoticeNoteReserve {
		t.Errorf("MreqTextReserve = %d, want %d", eventmsg.MreqTextReserve, memberReqNoticeNoteReserve)
	}
	if eventmsg.LockdownTextReserve != lockdownNoteReserve {
		t.Errorf("LockdownTextReserve = %d, want %d", eventmsg.LockdownTextReserve, lockdownNoteReserve)
	}

	seen := make(map[string]bool, len(eventmsg.Events))
	for _, fam := range settingsMsgFamilies {
		for _, ev := range fam.Events {
			seen[ev.Key] = true

			groupDef, ok := settingsEditPageByKey(settingsMsgWaitPage(ev))
			if !ok {
				t.Fatalf("missing group edit def for event %q", ev.Key)
			}
			_, hasPM := eventPMSpecFor(ev.Key)

			if got := eventmsg.HasPM(ev.Key); got != hasPM {
				t.Errorf("event %q: HasPM = %v, want %v (eventPMSpecs)", ev.Key, got, hasPM)
			}
			if got := eventmsg.NoMedia(ev.Key, eventmsg.DestGroup); got != fam.NoMedia {
				t.Errorf("event %q: NoMedia(group) = %v, want %v (settingsMsgFamilies)", ev.Key, got, fam.NoMedia)
			}
			if got := eventmsg.NoMedia(ev.Key, eventmsg.DestGroup); got != (settingsEditPartByCode(groupDef, settingsEditPartMedia) == nil) {
				t.Errorf("event %q: NoMedia(group) = %v, disagrees with groupDef media part", ev.Key, got)
			}
			if got := eventmsg.NoSilent(ev.Key, eventmsg.DestGroup); got != ev.NoSilent {
				t.Errorf("event %q: NoSilent(group) = %v, want %v", ev.Key, got, ev.NoSilent)
			}
			if got := eventmsg.NoSilent(ev.Key, eventmsg.DestPM); got {
				t.Errorf("event %q: NoSilent(pm) = true, want false", ev.Key)
			}
			if got := eventmsg.ReservedButtonsFor(ev.Key, eventmsg.DestGroup); got != groupDef.ReservedButtons {
				t.Errorf("event %q: ReservedButtonsFor(group) = %d, want %d", ev.Key, got, groupDef.ReservedButtons)
			}
			if got := eventmsg.ReservedTextFor(ev.Key, eventmsg.DestGroup); got != groupDef.ReservedText {
				t.Errorf("event %q: ReservedTextFor(group) = %d, want %d", ev.Key, got, groupDef.ReservedText)
			}
			if hasPM {
				pmDef, ok := settingsEditPageByKey(settingsMsgPMWaitPage(ev))
				if !ok {
					t.Fatalf("missing PM edit def for event %q", ev.Key)
				}
				if got := eventmsg.ReservedButtonsFor(ev.Key, eventmsg.DestPM); got != pmDef.ReservedButtons {
					t.Errorf("event %q: ReservedButtonsFor(pm) = %d, want %d", ev.Key, got, pmDef.ReservedButtons)
				}
				if got := eventmsg.ReservedTextFor(ev.Key, eventmsg.DestPM); got != pmDef.ReservedText {
					t.Errorf("event %q: ReservedTextFor(pm) = %d, want %d", ev.Key, got, pmDef.ReservedText)
				}
			}
		}
	}

	for _, ev := range eventmsg.Events {
		if !seen[ev] {
			t.Errorf("eventmsg.Events entry %q missing from settingsMsgFamilies", ev)
		}
	}
	if len(seen) != len(eventmsg.Events) {
		t.Errorf("settingsMsgFamilies covers %d events, eventmsg.Events has %d", len(seen), len(eventmsg.Events))
	}
	for ev := range eventPMSpecs {
		if !eventmsg.HasPM(ev) {
			t.Errorf("eventPMSpecs entry %q has HasPM == false", ev)
		}
	}
	if eventmsg.HasPM("unknown_event") {
		t.Errorf("HasPM(unknown_event) = true, want false")
	}
}

func TestCopySettingsMirrorAndExclusions(t *testing.T) {
	ctx := context.Background()

	t.Run("full_mirror_populated_source", func(t *testing.T) {
		utilscache.SetupTestMemoryMarshaler(t)
		srcChat := freshCopyChat(t)
		dstChat := freshCopyChat(t)
		const actorID = int64(5001)

		// Same-chat guard refuses immediately.
		if res := applyCopySettings(ctx, srcChat, srcChat, actorID); res.Outcome != copySettingsFailed {
			t.Fatalf("same-chat copy outcome = %v, want copySettingsFailed", res.Outcome)
		}

		// Populate all 6 areas in srcChat.
		if err := eventmsg.SetText(ctx, srcChat, eventmsg.EventBan, eventmsg.DestGroup, "<b>Src ban</b>", actorID); err != nil {
			t.Fatal(err)
		}
		if err := eventmsg.SetButtons(ctx, srcChat, eventmsg.EventBan, eventmsg.DestGroup, []models.Button{{Name: "Rules", Url: "https://example.com/rules"}}, actorID); err != nil {
			t.Fatal(err)
		}
		if err := eventmsg.SetText(ctx, srcChat, eventmsg.EventBan, eventmsg.DestPM, "Src PM ban", actorID); err != nil {
			t.Fatal(err)
		}
		if err := eventmsg.SetSilent(ctx, srcChat, eventmsg.EventBan, eventmsg.DestPM, false, actorID); err != nil {
			t.Fatal(err)
		}
		if err := eventmsg.SetSilent(ctx, srcChat, eventmsg.EventKick, eventmsg.DestGroup, true, actorID); err != nil {
			t.Fatal(err)
		}
		if err := eventmsg.SetPMDetails(ctx, srcChat, true, actorID); err != nil {
			t.Fatal(err)
		}
		if err := eventmsg.SetPMLog(ctx, srcChat, true, actorID); err != nil {
			t.Fatal(err)
		}
		if err := eventmsg.SetPlainTone(ctx, srcChat, true, actorID); err != nil {
			t.Fatal(err)
		}
		if err := warns.SetWarnLimit(srcChat, 9); err != nil {
			t.Fatal(err)
		}
		if err := warns.SetWarnModeContext(ctx, srcChat, "tban", 86400); err != nil {
			t.Fatal(err)
		}
		if err := warns.SetWarnExpiryContext(ctx, srcChat, 14); err != nil {
			t.Fatal(err)
		}
		if err := antiflood.SetFloodContext(ctx, srcChat, 6); err != nil {
			t.Fatal(err)
		}
		if err := antiflood.SetFloodActionContext(ctx, srcChat, "tmute", 1800); err != nil {
			t.Fatal(err)
		}
		if err := blacklists.AddBlacklist(srcChat, "srcword"); err != nil {
			t.Fatal(err)
		}
		if _, err := blacklists.SetBlacklistActionTimedContext(ctx, srcChat, "tban", 7200); err != nil {
			t.Fatal(err)
		}
		srcLadder := escalation.Ladder{
			Enabled: true,
			Steps: []escalation.Step{
				{Action: escalation.ActionTMute, Secs: 600},
				{Action: escalation.ActionKick},
				{Action: escalation.ActionTBan, Secs: 86400},
			},
			ForgetDays: 30,
		}
		if err := escalation.ReplaceLadder(ctx, srcChat, srcLadder, actorID); err != nil {
			t.Fatal(err)
		}
		srcDefs := modtuning.Defaults{
			DefaultBanSecs:  172800,
			DefaultMuteSecs: 3600,
			ReasonRequired:  true,
			AppealURL:       "https://example.com/appeal",
		}
		if err := modtuning.ReplaceDefaults(ctx, srcChat, srcDefs, actorID); err != nil {
			t.Fatal(err)
		}
		srcShortcuts := []modtuning.ShortcutEntry{
			{Word: "spam", Reason: "Unsolicited promotion"},
			{Word: "rude", Reason: "Abusive conduct"},
		}
		if err := modtuning.ReplaceShortcuts(ctx, srcChat, srcShortcuts, actorID); err != nil {
			t.Fatal(err)
		}
		srcThresh := advthresholds.Thresholds{
			RaidJoinWindowSecs:   45,
			RaidFloodNewPercent:  60,
			MemberReqCapPerMin:   12,
			LockdownReminderMins: 45,
		}
		if err := advthresholds.ReplaceAll(ctx, srcChat, srcThresh, actorID); err != nil {
			t.Fatal(err)
		}

		// Seed dstChat with different in-scope settings (keeping antiflood_settings ABSENT -> Limit == 0!)
		// plus a target warn_limit=5 and a target blacklist word "dstword".
		if err := eventmsg.SetText(ctx, dstChat, eventmsg.EventMute, eventmsg.DestGroup, "Old dst mute", actorID); err != nil {
			t.Fatal(err)
		}
		if err := warns.SetWarnLimit(dstChat, 5); err != nil {
			t.Fatal(err)
		}
		if err := warns.SetWarnModeContext(ctx, dstChat, "kick", 0); err != nil {
			t.Fatal(err)
		}
		if err := blacklists.AddBlacklist(dstChat, "dstword"); err != nil {
			t.Fatal(err)
		}
		if err := escalation.SetEnabled(ctx, dstChat, false, actorID); err != nil {
			t.Fatal(err)
		}
		if err := modtuning.SetReasonRequired(ctx, dstChat, false, actorID); err != nil {
			t.Fatal(err)
		}
		if _, err := modtuning.AddShortcut(ctx, dstChat, "stale", "Old dst shortcut", actorID); err != nil {
			t.Fatal(err)
		}
		if err := advthresholds.SetField(ctx, dstChat, advthresholds.FieldCaptchaTimeout, 20, actorID); err != nil {
			t.Fatal(err)
		}

		// Seed excluded tables on dstChat.
		staffLink := models.StaffGroupLink{GroupChatID: dstChat, StaffChatID: dstChat - 77, OwnerUserID: actorID, GroupTitle: "Dst", Health: models.StaffHealthOK}
		if err := db.DB.Create(&staffLink).Error; err != nil {
			t.Fatal(err)
		}
		approval := models.ApprovedUsers{ChatID: dstChat, UserID: 9001, Reason: "vip", ApprovedBy: actorID}
		if err := db.DB.Create(&approval).Error; err != nil {
			t.Fatal(err)
		}
		userWarn := models.Warns{ChatId: dstChat, UserId: 9002, NumWarns: 2, Reasons: models.StringArray{"r1", "r2"}}
		if err := db.DB.Create(&userWarn).Error; err != nil {
			t.Fatal(err)
		}
		offence := models.ChatOffence{ChatId: dstChat, UserId: 9003, Offences: 3, LastOffenceUnix: 1700000000}
		if err := db.DB.Create(&offence).Error; err != nil {
			t.Fatal(err)
		}
		greeting := models.GreetingSettings{ChatID: dstChat, WelcomeSettings: &models.WelcomeSettings{WelcomeText: "Dst welcome", ShouldWelcome: true, WelcomeType: db.TEXT}}
		if err := db.DB.Create(&greeting).Error; err != nil {
			t.Fatal(err)
		}
		rule := models.RulesSettings{ChatId: dstChat, Rules: "Dst rules"}
		if err := db.DB.Create(&rule).Error; err != nil {
			t.Fatal(err)
		}
		note := models.Notes{ChatId: dstChat, NoteName: "dstnote", NoteContent: "Dst note body", MsgType: db.TEXT}
		if err := db.DB.Create(&note).Error; err != nil {
			t.Fatal(err)
		}
		filter := models.ChatFilters{ChatId: dstChat, KeyWord: "dstflt", FilterReply: "Dst filter reply", MsgType: db.TEXT}
		if err := db.DB.Create(&filter).Error; err != nil {
			t.Fatal(err)
		}
		access := models.ChatSettingsAccess{ChatID: dstChat, AdminsAllowed: true, UpdatedBy: actorID}
		if err := db.DB.Create(&access).Error; err != nil {
			t.Fatal(err)
		}

		// Prime dstChat caches before copying to prove cache invalidation.
		_, _ = eventmsg.Get(ctx, dstChat)
		_ = warns.GetWarnSettingContext(ctx, dstChat)
		_ = antiflood.GetFloodContext(ctx, dstChat)
		_ = blacklists.GetBlacklistSettingsContext(ctx, dstChat)
		_, _ = escalation.Get(ctx, dstChat)
		_, _ = modtuning.GetDefaults(ctx, dstChat)
		_, _ = modtuning.GetShortcuts(ctx, dstChat)
		_, _ = advthresholds.Get(ctx, dstChat)

		res := applyCopySettings(ctx, srcChat, dstChat, actorID)
		if res.Outcome != copySettingsSaved || res.Err != nil {
			t.Fatalf("applyCopySettings = %+v, want copySettingsSaved", res)
		}
		if res.MessagesWritten != 3 || res.MessagesSkipped != 0 {
			t.Fatalf("MessagesWritten=%d MessagesSkipped=%d, want 3, 0", res.MessagesWritten, res.MessagesSkipped)
		}

		// 1 & 2. Event messages + options mirrored and stale dst mute message cleared.
		dstMsgs, err := eventmsg.Get(ctx, dstChat)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := dstMsgs.Lookup(eventmsg.EventMute, eventmsg.DestGroup); ok {
			t.Errorf("stale dst mute message survived copy")
		}
		if ban, ok := dstMsgs.Lookup(eventmsg.EventBan, eventmsg.DestGroup); !ok || ban.Text != "<b>Src ban</b>" || len(ban.Buttons) != 1 {
			t.Errorf("dst ban group = %+v, %v", ban, ok)
		}
		if pm, ok := dstMsgs.Lookup(eventmsg.EventBan, eventmsg.DestPM); !ok || pm.Text != "Src PM ban" || pm.Silent {
			t.Errorf("dst ban pm = %+v, %v", pm, ok)
		}
		if kick, ok := dstMsgs.Lookup(eventmsg.EventKick, eventmsg.DestGroup); !ok || !kick.Silent {
			t.Errorf("dst kick group = %+v, %v", kick, ok)
		}
		if !dstMsgs.Options.PMDetails || !dstMsgs.Options.PMLog || !dstMsgs.Options.PlainTone {
			t.Errorf("dst options = %+v, want all true", dstMsgs.Options)
		}

		// 3. Warns tuning mirrored while preserving dst warn_limit == 5.
		dstWarn := warns.GetWarnSettingContext(ctx, dstChat)
		if dstWarn.WarnLimit != 5 || dstWarn.WarnMode != "tban" || dstWarn.WarnModeSecs != 86400 || dstWarn.ExpiryDays != 14 {
			t.Errorf("dst warns_settings = %+v, want Limit=5 Mode=tban Secs=86400 Expiry=14", dstWarn)
		}

		// 3b. Antiflood action mirrored while keeping antiflood OFF (Limit == 0)!
		dstFlood := antiflood.GetFloodContext(ctx, dstChat)
		if dstFlood.Limit != 0 || dstFlood.Action != "tmute" || dstFlood.ActionSecs != 1800 {
			t.Errorf("dst antiflood_settings = %+v, want Limit=0 tmute/1800", dstFlood)
		}

		// 3c. Blacklist action applied ONLY to existing dstword (srcword never copied!).
		dstBL, err := blacklists.ListBlacklistEntriesFresh(ctx, dstChat)
		if err != nil {
			t.Fatal(err)
		}
		if len(dstBL) != 1 || dstBL[0].Word != "dstword" || dstBL[0].Action != "tban" || dstBL[0].ActionSecs != 7200 {
			t.Errorf("dst blacklists = %+v, want only dstword with tban/7200", dstBL)
		}

		// 4. Escalation ladder mirrored.
		dstLadder, err := escalation.Get(ctx, dstChat)
		if err != nil || !reflect.DeepEqual(*dstLadder, srcLadder) {
			t.Errorf("dst escalation = %+v, %v; want %+v", dstLadder, err, srcLadder)
		}

		// 5. Manual defaults & reason shortcuts mirrored (stale shortcut gone).
		dstDefs, err := modtuning.GetDefaults(ctx, dstChat)
		if err != nil || dstDefs != srcDefs {
			t.Errorf("dst manual defaults = %+v, %v; want %+v", dstDefs, err, srcDefs)
		}
		dstShortcuts, err := modtuning.GetShortcuts(ctx, dstChat)
		if err != nil || len(dstShortcuts) != 2 || dstShortcuts["spam"] != "Unsolicited promotion" || dstShortcuts["rude"] != "Abusive conduct" {
			t.Errorf("dst shortcuts = %v, %v", dstShortcuts, err)
		}

		// 6. Advanced thresholds mirrored (old CaptchaTimeout=20 cleared to 0).
		dstThresh, err := advthresholds.Get(ctx, dstChat)
		if err != nil || dstThresh != srcThresh {
			t.Errorf("dst advthresholds = %+v, %v; want %+v", dstThresh, err, srcThresh)
		}

		// Excluded tables on dstChat are 100% untouched.
		var gotLink models.StaffGroupLink
		if err := db.DB.Where("group_chat_id = ?", dstChat).Take(&gotLink).Error; err != nil || gotLink.StaffChatID != staffLink.StaffChatID {
			t.Errorf("staff_group_links altered: %+v, %v", gotLink, err)
		}
		var gotAppr models.ApprovedUsers
		if err := db.DB.Where("chat_id = ? AND user_id = ?", dstChat, 9001).Take(&gotAppr).Error; err != nil || gotAppr.Reason != "vip" {
			t.Errorf("approved_users altered: %+v, %v", gotAppr, err)
		}
		var gotWarnUser models.Warns
		if err := db.DB.Where("chat_id = ? AND user_id = ?", dstChat, 9002).Take(&gotWarnUser).Error; err != nil || gotWarnUser.NumWarns != 2 {
			t.Errorf("warns_users altered: %+v, %v", gotWarnUser, err)
		}
		var gotOffence models.ChatOffence
		if err := db.DB.Where("chat_id = ? AND user_id = ?", dstChat, 9003).Take(&gotOffence).Error; err != nil || gotOffence.Offences != 3 {
			t.Errorf("chat_offences altered: %+v, %v", gotOffence, err)
		}
		var gotGreet models.GreetingSettings
		if err := db.DB.Where("chat_id = ?", dstChat).Take(&gotGreet).Error; err != nil || gotGreet.WelcomeSettings.WelcomeText != "Dst welcome" {
			t.Errorf("greetings altered: %+v, %v", gotGreet, err)
		}
		var gotRule models.RulesSettings
		if err := db.DB.Where("chat_id = ?", dstChat).Take(&gotRule).Error; err != nil || gotRule.Rules != "Dst rules" {
			t.Errorf("rules altered: %+v, %v", gotRule, err)
		}
		var gotNote models.Notes
		if err := db.DB.Where("chat_id = ? AND note_name = ?", dstChat, "dstnote").Take(&gotNote).Error; err != nil || gotNote.NoteContent != "Dst note body" {
			t.Errorf("notes altered: %+v, %v", gotNote, err)
		}
		var gotFilter models.ChatFilters
		if err := db.DB.Where("chat_id = ? AND keyword = ?", dstChat, "dstflt").Take(&gotFilter).Error; err != nil || gotFilter.FilterReply != "Dst filter reply" {
			t.Errorf("filters altered: %+v, %v", gotFilter, err)
		}
		var gotAccess models.ChatSettingsAccess
		if err := db.DB.Where("chat_id = ?", dstChat).Take(&gotAccess).Error; err != nil || !gotAccess.AdminsAllowed {
			t.Errorf("chat_settings_access altered: %+v, %v", gotAccess, err)
		}

		// Editing dstChat afterwards does NOT affect srcChat (no ongoing link).
		if err := eventmsg.SetText(ctx, dstChat, eventmsg.EventBan, eventmsg.DestGroup, "Edited dst only", actorID); err != nil {
			t.Fatal(err)
		}
		if err := eventmsg.SetPlainTone(ctx, dstChat, false, actorID); err != nil {
			t.Fatal(err)
		}
		if err := advthresholds.ResetAll(ctx, dstChat); err != nil {
			t.Fatal(err)
		}
		srcMsgsAfter, err := eventmsg.GetFresh(ctx, srcChat)
		if err != nil {
			t.Fatal(err)
		}
		if ban, _ := srcMsgsAfter.Lookup(eventmsg.EventBan, eventmsg.DestGroup); ban.Text != "<b>Src ban</b>" || !srcMsgsAfter.Options.PlainTone {
			t.Errorf("srcChat was mutated after editing dstChat: ban=%+v opts=%+v", ban, srcMsgsAfter.Options)
		}
		if srcThAfter, err := advthresholds.GetFresh(ctx, srcChat); err != nil || srcThAfter != srcThresh {
			t.Errorf("srcChat advthresholds mutated: %+v, %v", srcThAfter, err)
		}
	})

	t.Run("empty_source_clears_target_full_mirror", func(t *testing.T) {
		utilscache.SetupTestMemoryMarshaler(t)
		emptySrc := freshCopyChat(t)
		dstChat := freshCopyChat(t)
		const actorID = int64(5002)

		// Heavily configure dstChat across all 6 areas.
		if err := eventmsg.SetText(ctx, dstChat, eventmsg.EventBan, eventmsg.DestGroup, "Custom ban", actorID); err != nil {
			t.Fatal(err)
		}
		if err := eventmsg.SetPMDetails(ctx, dstChat, true, actorID); err != nil {
			t.Fatal(err)
		}
		if err := eventmsg.SetPMLog(ctx, dstChat, true, actorID); err != nil {
			t.Fatal(err)
		}
		if err := eventmsg.SetPlainTone(ctx, dstChat, true, actorID); err != nil {
			t.Fatal(err)
		}
		if err := warns.SetWarnLimit(dstChat, 6); err != nil {
			t.Fatal(err)
		}
		if err := warns.SetWarnModeContext(ctx, dstChat, "tban", 3600); err != nil {
			t.Fatal(err)
		}
		if err := warns.SetWarnExpiryContext(ctx, dstChat, 7); err != nil {
			t.Fatal(err)
		}
		if err := antiflood.SetFloodContext(ctx, dstChat, 8); err != nil {
			t.Fatal(err)
		}
		if err := antiflood.SetFloodActionContext(ctx, dstChat, "tban", 3600); err != nil {
			t.Fatal(err)
		}
		if err := blacklists.AddBlacklist(dstChat, "keepword"); err != nil {
			t.Fatal(err)
		}
		if _, err := blacklists.SetBlacklistActionTimedContext(ctx, dstChat, "tmute", 600); err != nil {
			t.Fatal(err)
		}
		if err := escalation.SetEnabledWithSteps(ctx, dstChat, []escalation.Step{{Action: escalation.ActionBan}}, actorID); err != nil {
			t.Fatal(err)
		}
		if err := modtuning.ReplaceDefaults(ctx, dstChat, modtuning.Defaults{DefaultBanSecs: 3600, ReasonRequired: true, AppealURL: "https://example.com"}, actorID); err != nil {
			t.Fatal(err)
		}
		if _, err := modtuning.AddShortcut(ctx, dstChat, "ad", "Ad spam", actorID); err != nil {
			t.Fatal(err)
		}
		if err := advthresholds.SetField(ctx, dstChat, advthresholds.FieldCaptchaTimeout, 20, actorID); err != nil {
			t.Fatal(err)
		}

		res := applyCopySettings(ctx, emptySrc, dstChat, actorID)
		if res.Outcome != copySettingsSaved || res.Err != nil {
			t.Fatalf("applyCopySettings(empty->dst) = %+v, want saved", res)
		}

		// Target cleared back to defaults across all 6 areas.
		dstMsgs, err := eventmsg.Get(ctx, dstChat)
		if err != nil || len(dstMsgs.Rows) != 0 || dstMsgs.Options != (eventmsg.Options{}) {
			t.Errorf("dst eventmsg not cleared: rows=%d opts=%+v err=%v", len(dstMsgs.Rows), dstMsgs.Options, err)
		}
		dstWarn := warns.GetWarnSettingContext(ctx, dstChat)
		if dstWarn.WarnLimit != 6 || dstWarn.WarnMode != "mute" || dstWarn.WarnModeSecs != 0 || dstWarn.ExpiryDays != 0 {
			t.Errorf("dst warns = %+v, want Limit=6 mute/0 expiry=0", dstWarn)
		}
		dstFlood := antiflood.GetFloodContext(ctx, dstChat)
		if dstFlood.Limit != 8 || dstFlood.Action != "mute" || dstFlood.ActionSecs != 0 {
			t.Errorf("dst antiflood = %+v, want Limit=8 mute/0", dstFlood)
		}
		dstBL, err := blacklists.ListBlacklistEntriesFresh(ctx, dstChat)
		if err != nil || len(dstBL) != 1 || dstBL[0].Word != "keepword" || dstBL[0].Action != "warn" || dstBL[0].ActionSecs != 0 {
			t.Errorf("dst blacklists = %+v, %v; want keepword reset to warn/0", dstBL, err)
		}
		dstLadder, err := escalation.Get(ctx, dstChat)
		if err != nil || !reflect.DeepEqual(*dstLadder, escalation.Defaults()) {
			t.Errorf("dst escalation = %+v, %v; want defaults", dstLadder, err)
		}
		dstDefs, err := modtuning.GetDefaults(ctx, dstChat)
		if err != nil || dstDefs != (modtuning.Defaults{}) {
			t.Errorf("dst manual defaults = %+v, %v; want zero", dstDefs, err)
		}
		dstShortcuts, err := modtuning.GetShortcuts(ctx, dstChat)
		if err != nil || len(dstShortcuts) != 0 {
			t.Errorf("dst shortcuts = %v, %v; want empty", dstShortcuts, err)
		}
		dstThresh, err := advthresholds.Get(ctx, dstChat)
		if err != nil || dstThresh != (advthresholds.Thresholds{}) {
			t.Errorf("dst advthresholds = %+v, %v; want zero", dstThresh, err)
		}

		// emptySrc gained 0 rows across every table!
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
		}
		for _, tb := range tables {
			var n int64
			if err := db.DB.Model(tb.model).Where("chat_id = ?", emptySrc).Count(&n).Error; err != nil {
				t.Fatalf("count %s: %v", tb.name, err)
			}
			if n != 0 {
				t.Errorf("emptySrc gained %d row(s) in %s", n, tb.name)
			}
		}
	})

	t.Run("invalid_source_message_row_skipped_by_validated_writer", func(t *testing.T) {
		utilscache.SetupTestMemoryMarshaler(t)
		srcChat := freshCopyChat(t)
		dstChat := freshCopyChat(t)
		const actorID = int64(5003)

		// Seed one valid row and one raw malformed row (javascript: button URL) directly into DB.
		if err := eventmsg.SetText(ctx, srcChat, eventmsg.EventKick, eventmsg.DestGroup, "Valid kick", actorID); err != nil {
			t.Fatal(err)
		}
		rawBad := models.ChatEventMessage{
			ChatID:    srcChat,
			Event:     eventmsg.EventBan,
			Dest:      eventmsg.DestGroup,
			Text:      "Bad ban",
			Buttons:   models.ButtonArray{{Name: "Click", Url: "javascript:alert(1)"}},
			UpdatedBy: actorID,
			UpdatedAt: time.Now(),
		}
		if err := db.DB.Create(&rawBad).Error; err != nil {
			t.Fatalf("insert raw malformed row: %v", err)
		}
		// Also seed a raw malformed appeal_url on srcChat to confirm defensive re-validation.
		rawBadAppeal := models.ChatManualDefaults{
			ChatId:    srcChat,
			AppealURL: "javascript:alert(2)",
			UpdatedBy: actorID,
			UpdatedAt: time.Now(),
		}
		if err := db.DB.Create(&rawBadAppeal).Error; err != nil {
			t.Fatalf("insert raw bad appeal_url: %v", err)
		}

		res := applyCopySettings(ctx, srcChat, dstChat, actorID)
		if res.Outcome != copySettingsSaved || res.Err != nil {
			t.Fatalf("applyCopySettings = %+v, want saved", res)
		}
		if res.MessagesSkipped != 1 || res.MessagesWritten != 1 {
			t.Fatalf("MessagesWritten=%d MessagesSkipped=%d, want 1, 1", res.MessagesWritten, res.MessagesSkipped)
		}

		dstMsgs, err := eventmsg.GetFresh(ctx, dstChat)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := dstMsgs.Lookup(eventmsg.EventBan, eventmsg.DestGroup); ok {
			t.Errorf("malformed javascript: button row was copied to dstChat")
		}
		if kick, ok := dstMsgs.Lookup(eventmsg.EventKick, eventmsg.DestGroup); !ok || kick.Text != "Valid kick" {
			t.Errorf("valid kick row missing on dstChat: %+v, %v", kick, ok)
		}
		dstDefs, err := modtuning.GetDefaultsFresh(ctx, dstChat)
		if err != nil {
			t.Fatal(err)
		}
		if dstDefs.AppealURL != "" {
			t.Errorf("malformed appeal_url copied to dstChat: %q", dstDefs.AppealURL)
		}
	})
}
