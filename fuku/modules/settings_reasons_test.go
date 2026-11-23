//go:build testtools

package modules

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/fuku_robot/fuku/db"
	dbcache "github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/db/modtuning"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
)

func reasonsCleanup(t *testing.T, chatID int64) {
	t.Helper()
	t.Cleanup(func() {
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatReasonShortcut{})
		dbcache.DeleteCache(modtuning.ShortcutsCacheKey(chatID))
		dbcache.ResetLocalForTest()
	})
}

func newSettingsReasonsEnv(t *testing.T) *settingsListEnv {
	t.Helper()
	setCachedBotUsername(t, "FukuTestBot")
	env := newSettingsListEnv(t)
	reasonsCleanup(t, env.chat.Id)
	return env
}

func settingsReasonsSeed(t *testing.T, chatID int64, pairs ...[2]string) {
	t.Helper()
	for _, pair := range pairs {
		added, err := modtuning.AddShortcut(context.Background(), chatID, pair[0], pair[1], 1)
		if err != nil || !added {
			t.Fatalf("seed shortcut %q in chat %d: added=%v err=%v", pair[0], chatID, added, err)
		}
	}
}

func settingsReasonsStored(t *testing.T, chatID int64) map[string]modtuning.ShortcutEntry {
	t.Helper()
	rows, err := modtuning.ListShortcutsFresh(context.Background(), chatID)
	if err != nil {
		t.Fatalf("ListShortcutsFresh(%d): %v", chatID, err)
	}
	out := make(map[string]modtuning.ShortcutEntry, len(rows))
	for _, r := range rows {
		out[r.Word] = r
	}
	return out
}

func settingsReasonsClearWait(t *testing.T, env *settingsListEnv, userID int64) {
	t.Helper()
	env.redis.Del(settingsInputKey(userID))
	env.redis.Del(settingsInputDoneKey(userID))
}

func TestSettingsReasons(t *testing.T) {
	t.Run("empty_browse_paging_and_escaping", func(t *testing.T) {
		env := newSettingsReasonsEnv(t)
		owner := settingsFiltersOwner(env, env.chat)

		// Empty state creates 0 DB rows and shows empty line + Add button.
		m := settingsListOpenMenuPage(env, env.chat, owner, settingsPageReasons)
		text, kb := m.last()
		for _, want := range []string{
			staffMarker("settings_header"),
			staffMarker("settings_page_reasons"),
			staffMarker("settings_rsn_intro"),
			staffMarker("settings_rsn_empty"),
		} {
			if !strings.Contains(text, want) {
				t.Errorf("empty page lacks %q:\n%s", want, text)
			}
		}
		if strings.Contains(text, staffMarker("settings_rsn_count")) {
			t.Errorf("empty page unexpectedly showed count line:\n%s", text)
		}
		if got := settingsListRowWidths(kb); fmt.Sprint(got) != fmt.Sprint([]int{1, 2}) {
			t.Errorf("empty row widths = %v, want [1 2]", got)
		}
		if _, ok := m.markedButton("settings_btn_rsn_add"); !ok {
			t.Errorf("empty page lacks Add button")
		}
		if stored := settingsReasonsStored(t, env.chat.Id); len(stored) != 0 {
			t.Fatalf("opening empty page created %d rows", len(stored))
		}

		// Seed 10 entries including HTML characters in both word and detail.
		pairs := [][2]string{
			{"a&<b>", "Use <rules> & stay polite"},
			{"w02", "Reason two"},
			{"w03", "Reason three"},
			{"w04", "Reason four"},
			{"w05", "Reason five"},
			{"w06", "Reason six"},
			{"w07", "Reason seven"},
			{"w08", "Reason eight"},
			{"w09", "Reason nine"},
			{"w10", "Reason ten"},
		}
		settingsReasonsSeed(t, env.chat.Id, pairs...)

		// Main state shows 10/50 in private, unknown on count failure, and In private (0 reads) in group.
		m.press("a", settingsActBack)
		mainText, mainKB := m.last()
		if line, ok := settingsMainStateLine(mainText, "settings_page_reasons"); !ok || !strings.HasSuffix(line, " 10/50") {
			t.Errorf("private Main line = %q (ok=%v), want suffix 10/50", line, ok)
		}
		if _, ok := settingsButtonByMarker(mainKB, "settings_page_reasons"); !ok {
			t.Errorf("private Main lacks Reasons button")
		}

		var reads, counts atomic.Int32
		origRead, origCount := settingsReasonsRead, settingsReasonsCount
		settingsReasonsRead = func(ctx context.Context, chatID int64) ([]settingsListEntry, error) {
			reads.Add(1)
			return origRead(ctx, chatID)
		}
		settingsReasonsCount = func(ctx context.Context, chatID int64) (int, error) {
			counts.Add(1)
			return origCount(ctx, chatID)
		}
		t.Cleanup(func() { settingsReasonsRead, settingsReasonsCount = origRead, origCount })

		groupMsg := env.openMenu(owner)
		groupMain, _ := env.lastEdit(env.chat.Id, groupMsg)
		if line, ok := settingsMainStateLine(groupMain, "settings_page_reasons"); !ok || !strings.HasSuffix(line, staffMarker("settings_state_private_only")) {
			t.Errorf("group Main line = %q (ok=%v), want private-only", line, ok)
		}
		if reads.Load() != 0 || counts.Load() != 0 {
			t.Errorf("group Main called reads=%d counts=%d, want 0/0", reads.Load(), counts.Load())
		}

		settingsReasonsCount = func(context.Context, int64) (int, error) { return 0, errors.New("boom") }
		m.press("a", settingsActBack)
		failedMain, _ := m.last()
		if line, ok := settingsMainStateLine(failedMain, "settings_page_reasons"); !ok || !strings.HasSuffix(line, settingsUnknownState) {
			t.Errorf("failing count Main line = %q (ok=%v), want %q", line, ok, settingsUnknownState)
		}
		settingsReasonsCount = origCount
		settingsReasonsRead = origRead

		// Page 1 shows 8 rows with escaped word -> escaped detail and Next pager (sg, v=2).
		m.press("a", settingsActOpen, "p", settingsPageReasons)
		p1Text, p1KB := m.last()
		if !strings.Contains(p1Text, staffMarker("settings_rsn_count")+" 10 50") {
			t.Errorf("page 1 lacks count 10/50:\n%s", p1Text)
		}
		wantEscaped := "1. <code>a&amp;&lt;b&gt;</code>" + settingsListDetailSep + "Use &lt;rules&gt; &amp; stay polite"
		if !strings.Contains(p1Text, wantEscaped) {
			t.Errorf("page 1 lacks escaped line %q:\n%s", wantEscaped, p1Text)
		}
		if strings.Contains(p1Text, "<rules>") || strings.Contains(p1Text, "<b>") && strings.Contains(p1Text, "a&<b>") {
			t.Errorf("page 1 leaked raw HTML:\n%s", p1Text)
		}
		if strings.Contains(p1Text, "<code>w09</code>") {
			t.Errorf("page 1 unexpectedly showed entry 9:\n%s", p1Text)
		}
		nextBtn, ok := settingsModDefaultsFindButton(t, p1KB, map[string]string{"a": "sg", "v": "2"})
		if !ok {
			t.Fatalf("page 1 lacks Next pager button (a=sg, v=2): %+v", p1KB)
		}
		for _, btn := range settingsAllButtons(p1KB) {
			if strings.Contains(btn.CallbackData, "w02") || strings.Contains(btn.CallbackData, "rules") {
				t.Errorf("user text leaked into callback data %q", btn.CallbackData)
			}
		}

		// Tap Next -> page 2 shows entries 9 and 10 and Prev pager (sg, v=1).
		m.tap(nextBtn)
		p2Text, p2KB := m.last()
		for _, want := range []string{
			"9. <code>w09</code>" + settingsListDetailSep + "Reason nine",
			"10. <code>w10</code>" + settingsListDetailSep + "Reason ten",
		} {
			if !strings.Contains(p2Text, want) {
				t.Errorf("page 2 lacks %q:\n%s", want, p2Text)
			}
		}
		if _, ok := settingsModDefaultsFindButton(t, p2KB, map[string]string{"a": "sg", "v": "1"}); !ok {
			t.Errorf("page 2 lacks Prev pager button (a=sg, v=1)")
		}
	})

	t.Run("two_step_add_happy_path", func(t *testing.T) {
		env := newSettingsReasonsEnv(t)
		owner := settingsFiltersOwner(env, env.chat)
		t.Cleanup(func() { settingsReasonsClearWait(t, env, owner.Id) })

		m := settingsListOpenMenuPage(env, env.chat, owner, settingsPageReasons)
		addBtn, ok := m.markedButton("settings_btn_rsn_add")
		if !ok {
			t.Fatalf("Add button missing")
		}
		m.tap(addBtn)

		// Step 1 prompt: word prompt + word rules + step 1/2 + Cancel (sc, t=w), no content limits.
		prompt1, kb1 := m.last()
		for _, want := range []string{
			staffMarker("settings_rsn_wait_word"),
			staffMarker("settings_rsn_wait_word_rules") + " 32",
			staffMarker("settings_entries_step") + " 1 2",
		} {
			if !strings.Contains(prompt1, want) {
				t.Errorf("step-1 prompt lacks %q:\n%s", want, prompt1)
			}
		}
		if _, ok := settingsModDefaultsFindButton(t, kb1, map[string]string{"a": "sc", "t": "w"}); !ok {
			t.Errorf("step-1 prompt lacks Cancel (a=sc, t=w)")
		}

		// Step 1 typed reply: "Spam" -> normalised to "spam", advances to step 2 ("r").
		before := len(env.fake.sentTo(owner.Id))
		step1ID := settingsListDoorSay(env, owner, "  Spam  ")
		reply1 := settingsListDoorReply(t, env, owner, before,
			"settings_rsn_word_line",
			"settings_rsn_wait_text",
			"settings_rsn_wait_text_rules",
			"settings_entries_step",
		)
		step2Text := sentText(reply1)
		if !strings.Contains(step2Text, staffMarker("settings_rsn_word_line")+" spam") {
			t.Errorf("step-2 prompt does not show normalised word 'spam':\n%s", step2Text)
		}
		if strings.Contains(step2Text, staffMarker("settings_entries_wait_content_limits")) {
			t.Errorf("plain-text step-2 prompt must NOT show settings_entries_wait_content_limits:\n%s", step2Text)
		}
		if got := env.redis.HGet(settingsInputKey(owner.Id), "t"); got != "r" {
			t.Fatalf("wait part = %q, want r", got)
		}
		if got := env.redis.HGet(settingsInputKey(owner.Id), "w"); got != "spam" {
			t.Fatalf("wait pending = %q, want spam", got)
		}
		if got := env.redis.HGet(settingsInputKey(owner.Id), "q"); got != strconv.FormatInt(step1ID, 10) {
			t.Fatalf("wait StepMsgID = %q, want %d", got, step1ID)
		}

		// Step 2 typed reply: plain full text with HTML chars -> stored raw, rendered escaped, wait closed.
		before = len(env.fake.sentTo(owner.Id))
		settingsListDoorSay(env, owner, "Repeated <promo> & links")
		reply2 := settingsListDoorReply(t, env, owner, before, "settings_rsn_saved", "settings_rsn_count")
		savedText := sentText(reply2)
		if !strings.Contains(savedText, settingsNoticeOKIcon+staffMarker("settings_rsn_saved")+" spam") {
			t.Errorf("saved list lacks saved notice naming spam:\n%s", savedText)
		}
		if !strings.Contains(savedText, "1. <code>spam</code>"+settingsListDetailSep+"Repeated &lt;promo&gt; &amp; links") {
			t.Errorf("saved list lacks escaped entry line:\n%s", savedText)
		}
		if strings.Contains(savedText, "<promo>") {
			t.Errorf("saved list leaked raw HTML:\n%s", savedText)
		}
		settingsEditWaitGone(t, env.settingsEnv, owner)

		stored := settingsReasonsStored(t, env.chat.Id)
		entry, ok := stored["spam"]
		if !ok || entry.Reason != "Repeated <promo> & links" {
			t.Fatalf("DB stored = %+v, want spam -> Repeated <promo> & links", stored)
		}
	})

	t.Run("step1_and_step2_refusals_redelivery_cap_cancel_and_expired_wait", func(t *testing.T) {
		env := newSettingsReasonsEnv(t)
		owner := settingsFiltersOwner(env, env.chat)
		t.Cleanup(func() { settingsReasonsClearWait(t, env, owner.Id) })

		settingsReasonsSeed(t, env.chat.Id, [2]string{"existing", "Original text"})

		m := settingsListOpenMenuPage(env, env.chat, owner, settingsPageReasons)
		addBtn, _ := m.markedButton("settings_btn_rsn_add")
		m.tap(addBtn)

		// Step 1 refusals keep the wait at part "w".
		step1Cases := []struct {
			name    string
			msg     gotgbot.Message
			wantKey string
		}{
			{"media_only", settingsListDoorPhoto(), "settings_edit_refuse_need_text"},
			{"blank_spaces", gotgbot.Message{Text: "   \u3000  "}, "settings_entries_refuse_blank"},
			{"multi_word", gotgbot.Message{Text: "two words"}, "settings_rsn_refuse_word"},
			{"leading_slash_with_space", gotgbot.Message{Text: " /slash"}, "settings_rsn_refuse_word"},
			{"slash_command", gotgbot.Message{Text: "/other"}, "settings_edit_refuse_slash"},
			{"too_long_word", gotgbot.Message{Text: strings.Repeat("a", models.ReasonShortcutWordMaxRunes+1)}, "settings_rsn_refuse_word"},
			{"duplicate_word", gotgbot.Message{Text: "EXISTING"}, "settings_rsn_refuse_exists"},
		}
		for _, tc := range step1Cases {
			before := len(env.fake.sentTo(owner.Id))
			sendPrivateMessage(env.settingsEnv, owner, tc.msg)
			settingsListDoorReply(t, env, owner, before, tc.wantKey)
			settingsEditWaitKept(t, env.settingsEnv, owner)
			if got := env.redis.HGet(settingsInputKey(owner.Id), "t"); got != "w" {
				t.Errorf("%s: wait moved to %q, want w", tc.name, got)
			}
		}
		if got := settingsReasonsStored(t, env.chat.Id)["existing"].Reason; got != "Original text" {
			t.Fatalf("duplicate step-1 attempt modified existing entry: %q", got)
		}

		// Valid step 1 advances to step 2 ("r").
		before := len(env.fake.sentTo(owner.Id))
		step1ID := settingsListDoorSay(env, owner, "promo")
		settingsListDoorReply(t, env, owner, before, "settings_rsn_wait_text")

		// Step 1 redelivery (MessageId <= StepMsgID) is ignored silently with 0 replies and wait kept at "r".
		beforeRedelivery := len(env.fake.sentTo(owner.Id))
		redelivery := &gotgbot.Update{
			UpdateId: env.updateID(),
			Message: &gotgbot.Message{
				MessageId: step1ID,
				Date:      1,
				Chat:      gotgbot.Chat{Id: owner.Id, Type: "private"},
				From:      &owner,
				Text:      "promo",
			},
		}
		if err := env.dispatcher.ProcessUpdate(env.bot, redelivery, nil); err != nil {
			t.Fatalf("redelivery ProcessUpdate: %v", err)
		}
		if got := len(env.fake.sentTo(owner.Id)); got != beforeRedelivery {
			t.Errorf("redelivery sent %d messages, want 0", got-beforeRedelivery)
		}
		if got := env.redis.HGet(settingsInputKey(owner.Id), "t"); got != "r" {
			t.Errorf("after redelivery wait part = %q, want r", got)
		}
		if _, exists := settingsReasonsStored(t, env.chat.Id)["promo"]; exists {
			t.Fatalf("redelivery saved 'promo' using step-1 text")
		}

		// Step 2 refusals keep the wait at part "r" with Pending == "promo".
		photoWithCaption := settingsListDoorPhoto()
		photoWithCaption.Caption = "caption should be refused"
		step2Cases := []struct {
			name    string
			msg     gotgbot.Message
			wantKey string
		}{
			{"media_only_with_caption", photoWithCaption, "settings_edit_refuse_need_text"},
			{"blank_text", gotgbot.Message{Text: "   \t\n  "}, "settings_entries_refuse_blank"},
			{"too_long_text", gotgbot.Message{Text: strings.Repeat("x", models.ReasonShortcutTextMaxRunes+1)}, "settings_rsn_refuse_text_long"},
		}
		for _, tc := range step2Cases {
			before := len(env.fake.sentTo(owner.Id))
			sendPrivateMessage(env.settingsEnv, owner, tc.msg)
			settingsListDoorReply(t, env, owner, before, tc.wantKey)
			settingsEditWaitKept(t, env.settingsEnv, owner)
			if env.redis.HGet(settingsInputKey(owner.Id), "t") != "r" || env.redis.HGet(settingsInputKey(owner.Id), "w") != "promo" {
				t.Errorf("%s: wait mutated after step-2 refusal", tc.name)
			}
		}

		// Race between steps: another admin inserts "promo" before step 2 completes -> ends wait without overwriting.
		settingsReasonsSeed(t, env.chat.Id, [2]string{"promo", "Raced original"})
		before = len(env.fake.sentTo(owner.Id))
		settingsListDoorSay(env, owner, "Overwriting attempt")
		settingsListDoorReply(t, env, owner, before, "settings_rsn_refuse_exists", "settings_edit_wait_stopped")
		settingsEditWaitGone(t, env.settingsEnv, owner)
		if got := settingsReasonsStored(t, env.chat.Id)["promo"].Reason; got != "Raced original" {
			t.Fatalf("raced step-2 overwrote entry: %q", got)
		}

		// Cancel button (sc) ends a step-1 wait cleanly.
		m = settingsListOpenMenuPage(env, env.chat, owner, settingsPageReasons)
		addBtn, _ = m.markedButton("settings_btn_rsn_add")
		m.tap(addBtn)
		cancelBtn, ok := m.markedButton("settings_btn_edit_cancel")
		if !ok {
			t.Fatalf("Cancel button missing on prompt")
		}
		m.tap(cancelBtn)
		cancelText, _ := m.last()
		if !strings.Contains(cancelText, settingsNoticeOKIcon+staffMarker("settings_edit_cancelled")) {
			t.Errorf("Cancel did not show cancelled notice:\n%s", cancelText)
		}
		settingsEditWaitGone(t, env.settingsEnv, owner)

		// /cancel ends a step-2 wait cleanly.
		m.tap(addBtn)
		before = len(env.fake.sentTo(owner.Id))
		settingsListDoorSay(env, owner, "tempword")
		settingsListDoorReply(t, env, owner, before, "settings_rsn_wait_text")
		before = len(env.fake.sentTo(owner.Id))
		settingsListDoorSay(env, owner, "/cancel")
		settingsListDoorReply(t, env, owner, before, "settings_edit_cancelled")
		settingsEditWaitGone(t, env.settingsEnv, owner)

		// Expired wait markers for part "w" and part "r" reply with settings_entries_wait_ended_note naming each part.
		for _, tc := range []struct {
			part    string
			wordKey string
		}{
			{"w", "settings_entries_part_reason_word"},
			{"r", "settings_entries_part_reason_text"},
		} {
			settingsReasonsClearWait(t, env, owner.Id)
			if err := env.redis.Set(settingsInputDoneKey(owner.Id), settingsInputMarkerValue(env.chat.Id, settingsPageReasons, tc.part)); err != nil {
				t.Fatalf("set the wait marker: %v", err)
			}
			before = len(env.fake.sentTo(owner.Id))
			settingsListDoorSay(env, owner, "late message")
			reply := settingsListDoorReply(t, env, owner, before, "settings_entries_wait_ended_note", tc.wordKey)
			if !strings.Contains(sentText(reply), staffMarker(tc.wordKey)) {
				t.Errorf("expired wait part %q did not name %s: %q", tc.part, tc.wordKey, sentText(reply))
			}
		}

		// Cap at 50 blocks Add via AddGate (redraws page 1 with cap notice and creates no wait).
		existingCount := len(settingsReasonsStored(t, env.chat.Id))
		for i := existingCount; i < models.ReasonShortcutsCapMax; i++ {
			settingsReasonsSeed(t, env.chat.Id, [2]string{fmt.Sprintf("cap%02d", i), "Cap reason"})
		}
		m = settingsListOpenMenuPage(env, env.chat, owner, settingsPageReasons)
		addBtn, _ = m.markedButton("settings_btn_rsn_add")
		m.tap(addBtn)
		capText, _ := m.last()
		if !strings.Contains(capText, settingsNoticeWarnIcon+staffMarker("settings_rsn_refuse_cap")+" 50") {
			t.Errorf("Add at cap 50 drew:\n%s", capText)
		}
		if env.redis.Exists(settingsInputKey(owner.Id)) {
			t.Errorf("Add at cap 50 unexpectedly created a wait")
		}
	})

	t.Run("two_tap_delete_foreign_id_and_stale_yes", func(t *testing.T) {
		env := newSettingsReasonsEnv(t)
		owner := settingsFiltersOwner(env, env.chat)
		other := env.addChat()
		reasonsCleanup(t, other.Id)

		settingsReasonsSeed(t, env.chat.Id,
			[2]string{"a<tag>", "Escaped reason"},
			[2]string{"keep", "Keep this"},
		)
		settingsReasonsSeed(t, other.Id, [2]string{"foreign", "Secret other chat reason"})

		// Prime shortcuts cache so we verify delete drops it.
		if _, err := modtuning.GetShortcuts(context.Background(), env.chat.Id); err != nil {
			t.Fatalf("prime cache: %v", err)
		}
		cacheKey := modtuning.ShortcutsCacheKey(env.chat.Id)
		if !env.redis.Exists(cacheKey) {
			t.Fatalf("shortcuts cache key %s was not primed", cacheKey)
		}

		targetID := settingsReasonsStored(t, env.chat.Id)["a<tag>"].ID
		foreignID := settingsReasonsStored(t, other.Id)["foreign"].ID

		m := settingsListOpenMenuPage(env, env.chat, owner, settingsPageReasons)

		// Tap delete (sx) -> shows confirm card naming escaped word; DB untouched.
		m.tap(m.button("🗑 1"))
		cardText, cardKB := m.last()
		if !strings.Contains(cardText, staffMarker("settings_rsn_delete_ask")+" a&lt;tag&gt;") {
			t.Fatalf("delete card lacks escaped word:\n%s", cardText)
		}
		if strings.Contains(cardText, "a<tag>") {
			t.Errorf("delete card leaked raw HTML:\n%s", cardText)
		}
		yesBtn, ok := m.markedButton("settings_btn_yes")
		if !ok {
			t.Fatalf("delete card lacks Yes button: %+v", cardKB)
		}
		yesFields := settingsButtonFields(t, yesBtn)
		if yesFields["a"] != "sy" || yesFields["r"] != strconv.FormatInt(targetID, 10) || yesFields["e"] == "" {
			t.Errorf("Yes fields = %v, want a=sy r=%d e=...", yesFields, targetID)
		}
		noBtn, ok := m.markedButton("settings_btn_no")
		if !ok {
			t.Fatalf("delete card lacks No button")
		}
		if noFields := settingsButtonFields(t, noBtn); noFields["a"] != "sg" || noFields["v"] != "1" {
			t.Errorf("No fields = %v, want a=sg v=1", noFields)
		}
		if len(settingsReasonsStored(t, env.chat.Id)) != 2 {
			t.Fatalf("Ask mutated DB before Yes")
		}

		// Tap No -> returns to list without deleting.
		m.tap(noBtn)
		if len(settingsReasonsStored(t, env.chat.Id)) != 2 {
			t.Fatalf("No deleted a row")
		}

		// Expired Yes minute refuses safely without deleting.
		m.press("a", "sy", "r", strconv.FormatInt(targetID, 10), "e", settingsCardExpiry(time.Now().Add(-10*time.Minute)))
		expiredText, _ := m.last()
		if !strings.Contains(expiredText, staffMarker("settings_card_expired")) {
			t.Errorf("expired Yes drew:\n%s", expiredText)
		}
		if len(settingsReasonsStored(t, env.chat.Id)) != 2 {
			t.Fatalf("expired Yes deleted a row")
		}

		// Tap delete then Yes -> deletes row, drops cache, shows removed notice, redraws list.
		m.tap(m.button("🗑 1"))
		yesBtn, _ = m.markedButton("settings_btn_yes")
		m.tap(yesBtn)
		afterText, _ := m.last()
		if !strings.Contains(afterText, settingsNoticeOKIcon+staffMarker("settings_rsn_removed")+" a&lt;tag&gt;") {
			t.Errorf("after Yes, page lacks removed notice:\n%s", afterText)
		}
		if _, still := settingsReasonsStored(t, env.chat.Id)["a<tag>"]; still {
			t.Errorf("deleted shortcut still present in DB")
		}
		if env.redis.Exists(cacheKey) {
			t.Errorf("delete did not invalidate cache key %s", cacheKey)
		}

		// Stale Yes after row already deleted is safe.
		m.press("a", "sy", "r", strconv.FormatInt(targetID, 10), "e", settingsCardExpiry(time.Now()))
		staleText, _ := m.last()
		if !strings.Contains(staleText, staffMarker("settings_entries_already_removed")) {
			t.Errorf("stale Yes drew:\n%s", staleText)
		}

		// Foreign-chat row ID on Ask (sx) and Yes (sy) is refused as gone without disclosing or deleting it.
		m.press("a", "sx", "r", strconv.FormatInt(foreignID, 10), "v", "1")
		foreignAsk, _ := m.last()
		if !strings.Contains(foreignAsk, staffMarker("settings_entries_already_removed")) ||
			strings.Contains(foreignAsk, "foreign") ||
			strings.Contains(foreignAsk, staffMarker("settings_rsn_delete_ask")) {
			t.Errorf("foreign Ask drew:\n%s", foreignAsk)
		}
		m.press("a", "sy", "r", strconv.FormatInt(foreignID, 10), "e", settingsCardExpiry(time.Now()))
		foreignYes, _ := m.last()
		if !strings.Contains(foreignYes, staffMarker("settings_entries_already_removed")) || strings.Contains(foreignYes, "foreign") {
			t.Errorf("foreign Yes drew:\n%s", foreignYes)
		}
		if _, ok := settingsReasonsStored(t, other.Id)["foreign"]; !ok {
			t.Fatalf("foreign Yes deleted another chat's shortcut!")
		}
	})

	t.Run("authority_owner_gate_group_menu_and_staff_group", func(t *testing.T) {
		env := newSettingsReasonsEnv(t)
		owner := settingsFiltersOwner(env, env.chat)
		admin := env.admin
		t.Cleanup(func() {
			settingsReasonsClearWait(t, env, owner.Id)
			settingsReasonsClearWait(t, env, admin.Id)
		})

		settingsReasonsSeed(t, env.chat.Id, [2]string{"alpha", "Alpha reason"})
		alphaID := settingsReasonsStored(t, env.chat.Id)["alpha"].ID

		// Admin admitted by owner gate, WITHOUT can_restrict_members: can browse (op, sg) but refused on sa/sx/sy.
		env.setAdminsAllowed(env.chat.Id, true)
		env.fake.setMember(env.chat.Id, admin.Id, staffFakeMember{
			Status:             gotgbot.ChatMemberStatusAdministrator,
			CanRestrictMembers: false,
		})
		m := settingsListOpenMenuPage(env, env.chat, admin, settingsPageReasons)
		browseText, _ := m.last()
		if !strings.Contains(browseText, "1. <code>alpha</code>") {
			t.Fatalf("admin without restrict could not browse:\n%s", browseText)
		}
		m.press("a", "sg", "v", "1")
		if pageText, _ := m.last(); !strings.Contains(pageText, "1. <code>alpha</code>") {
			t.Errorf("admin without restrict could not use pager:\n%s", pageText)
		}

		for name, kv := range map[string][]string{
			"Add": {"a", "sa"},
			"Ask": {"a", "sx", "r", strconv.FormatInt(alphaID, 10), "v", "1"},
			"Yes": {"a", "sy", "r", strconv.FormatInt(alphaID, 10), "e", settingsCardExpiry(time.Now())},
		} {
			editsBefore := len(env.edits(admin.Id, m.msgID))
			m.press(kv...)
			if ans, alert := env.lastAnswer(); !alert || !strings.Contains(ans, staffMarker("settings_no_restrict")) {
				t.Errorf("admin without restrict %s answered %q (alert %v), want settings_no_restrict", name, ans, alert)
			}
			if len(env.edits(admin.Id, m.msgID)) != editsBefore {
				t.Errorf("refused %s edited the menu", name)
			}
		}
		if env.redis.Exists(settingsInputKey(admin.Id)) {
			t.Errorf("refused Add created a wait")
		}

		// Mid-wait demotion at step 1 and step 2: admin gets restrict, starts wait, loses restrict before typing.
		env.fake.setMember(env.chat.Id, admin.Id, staffFakeMember{
			Status:             gotgbot.ChatMemberStatusAdministrator,
			CanRestrictMembers: true,
		})
		m.press("a", "sa")
		if !env.redis.Exists(settingsInputKey(admin.Id)) {
			t.Fatalf("admin with restrict could not start wait")
		}
		// Demote before step 1 reply.
		env.fake.setMember(env.chat.Id, admin.Id, staffFakeMember{
			Status:             gotgbot.ChatMemberStatusAdministrator,
			CanRestrictMembers: false,
		})
		before := len(env.fake.sentTo(admin.Id))
		settingsListDoorSay(env, admin, "demoted1")
		settingsListDoorReply(t, env, admin, before, "settings_no_restrict", "settings_edit_wait_stopped")
		settingsEditWaitGone(t, env.settingsEnv, admin)

		// Re-grant restrict, complete step 1, then demote before step 2 reply.
		env.fake.setMember(env.chat.Id, admin.Id, staffFakeMember{
			Status:             gotgbot.ChatMemberStatusAdministrator,
			CanRestrictMembers: true,
		})
		m = settingsListOpenMenuPage(env, env.chat, admin, settingsPageReasons)
		m.press("a", "sa")
		before = len(env.fake.sentTo(admin.Id))
		settingsListDoorSay(env, admin, "demoted2")
		settingsListDoorReply(t, env, admin, before, "settings_rsn_wait_text")
		env.fake.setMember(env.chat.Id, admin.Id, staffFakeMember{
			Status:             gotgbot.ChatMemberStatusAdministrator,
			CanRestrictMembers: false,
		})
		before = len(env.fake.sentTo(admin.Id))
		settingsListDoorSay(env, admin, "Should not be saved")
		settingsListDoorReply(t, env, admin, before, "settings_no_restrict", "settings_edit_wait_stopped")
		settingsEditWaitGone(t, env.settingsEnv, admin)
		if _, exists := settingsReasonsStored(t, env.chat.Id)["demoted2"]; exists {
			t.Fatalf("demoted mid-wait step-2 saved a shortcut!")
		}

		// Owner gate refuses non-owner admin even with can_restrict_members.
		env.setAdminsAllowed(env.chat.Id, false)
		env.fake.setMember(env.chat.Id, admin.Id, staffFakeMember{
			Status:             gotgbot.ChatMemberStatusAdministrator,
			CanRestrictMembers: true,
		})
		m.press("a", "sa")
		if ans, alert := env.lastAnswer(); !alert || !strings.Contains(ans, staffMarker("settings_gate_owner_only")) {
			t.Errorf("owner gate answered %q (alert %v), want settings_gate_owner_only", ans, alert)
		}

		// Group menu shows private-only note on op&p=rs (0 DB reads) and refuses all s* codes in step 1 (0 getChatMember calls).
		var reads atomic.Int32
		origRead := settingsReasonsRead
		settingsReasonsRead = func(ctx context.Context, chatID int64) ([]settingsListEntry, error) {
			reads.Add(1)
			return origRead(ctx, chatID)
		}
		t.Cleanup(func() { settingsReasonsRead = origRead })

		groupMsg := env.openMenu(owner)
		env.press(owner, groupMsg, env.groupFields(owner, "a", settingsActOpen, "p", settingsPageReasons))
		groupNote, _ := env.lastEdit(env.chat.Id, groupMsg)
		if !strings.Contains(groupNote, staffMarker("settings_entries_group_note")) {
			t.Errorf("group menu op&p=rs drew:\n%s", groupNote)
		}
		if reads.Load() != 0 {
			t.Errorf("group menu op&p=rs called Read %d times, want 0", reads.Load())
		}

		gcmBefore := len(callsToChat(env.fake.staffBotClient, "getChatMember", env.chat.Id))
		for _, kv := range [][]string{
			{"a", "sg", "v", "1"},
			{"a", "sx", "r", strconv.FormatInt(alphaID, 10), "v", "1"},
			{"a", "sy", "r", strconv.FormatInt(alphaID, 10), "e", settingsCardExpiry(time.Now())},
			{"a", "sa"},
			{"a", "sc", "t", "w"},
		} {
			env.press(owner, groupMsg, env.groupFields(owner, kv...))
			if ans, alert := env.lastAnswer(); !alert || !strings.Contains(ans, staffMarker("common_callback_invalid_request")) {
				t.Errorf("group menu %v answered %q (alert %v), want invalid request", kv, ans, alert)
			}
		}
		for _, kv := range [][]string{
			{"a", "sg", "v", "abc"},
			{"a", "sx", "r", "012", "v", "1"},
			{"a", "sy", "r", "1"},
			{"a", "sa", "x", "1"},
			{"a", "sc", "t", "k"},
		} {
			env.pressPrivate(owner, groupMsg, env.privateFields(env.chat.Id, kv...))
			if ans, alert := env.lastAnswer(); !alert || !strings.Contains(ans, staffMarker("common_callback_invalid_request")) {
				t.Errorf("forged private %v answered %q (alert %v), want invalid request", kv, ans, alert)
			}
		}
		if gcmAfter := len(callsToChat(env.fake.staffBotClient, "getChatMember", env.chat.Id)); gcmAfter != gcmBefore {
			t.Errorf("group/forged presses made %d getChatMember calls, want 0", gcmAfter-gcmBefore)
		}

		// Staff Group hides rs from Main and refuses op&p=rs and all s* codes.
		origStaff := settingsReasonsStaffLookup
		settingsReasonsStaffLookup = func(chatID int64) (*models.StaffGroup, error) {
			if chatID == env.chat.Id {
				return &models.StaffGroup{ChatID: chatID}, nil
			}
			return nil, nil
		}
		t.Cleanup(func() { settingsReasonsStaffLookup = origStaff })

		staffMsg := env.updateID()
		env.pressPrivate(owner, staffMsg, env.privateFields(env.chat.Id, "a", settingsActBack))
		staffMain, staffKB := env.lastEdit(owner.Id, staffMsg)
		if _, ok := settingsMainStateLine(staffMain, "settings_page_reasons"); ok {
			t.Errorf("Staff Group Main still listed settings_page_reasons:\n%s", staffMain)
		}
		if _, found := settingsModDefaultsFindButton(t, staffKB, map[string]string{"a": settingsActOpen, "p": settingsPageReasons}); found {
			t.Errorf("Staff Group Main still carried op&p=rs button")
		}
		for _, kv := range [][]string{
			{"a", settingsActOpen, "p", settingsPageReasons},
			{"a", "sg", "v", "1"},
			{"a", "sx", "r", strconv.FormatInt(alphaID, 10), "v", "1"},
			{"a", "sy", "r", strconv.FormatInt(alphaID, 10), "e", settingsCardExpiry(time.Now())},
			{"a", "sa"},
			{"a", "sc", "t", "w"},
			{"a", "sc", "t", "r"},
		} {
			env.pressPrivate(owner, staffMsg, env.privateFields(env.chat.Id, kv...))
			if ans, alert := env.lastAnswer(); !alert || !strings.Contains(ans, staffMarker("common_callback_invalid_request")) {
				t.Errorf("Staff Group %v answered %q (alert %v), want invalid request", kv, ans, alert)
			}
		}
	})

	t.Run("worst_case_callback_bytes", func(t *testing.T) {
		withRealLocales(t)
		tr := i18n.MustNewTranslator("en")
		views := settingsBlacklistWorstViews(tr)
		private := views["private"]
		private.OwnerValue = "-1001234567890"
		private.GroupID = -1001234567890
		def := settingsReasonsDef()

		// 17-digit row IDs (99999999999999999) + 4-digit pager cursor (9999 pages).
		entries := make([]settingsListEntry, 0, settingsListPageSize*9999)
		for i := 0; i < settingsListPageSize*9999; i++ {
			id := int64(99999999999999999) - int64(i)
			entries = append(entries, settingsListEntry{ID: id, Text: fmt.Sprintf("w%06d", i), Detail: "Full reason text"})
		}

		seenCodes := map[string]int{}
		measure := func(name string, screen settingsScreen) {
			t.Helper()
			if screen.Omitted != 0 {
				t.Errorf("%s: Omitted = %d, want 0", name, screen.Omitted)
			}
			for _, btn := range settingsAllButtons(screen.Keyboard.InlineKeyboard) {
				if btn.CallbackData == "" {
					continue
				}
				if len(btn.CallbackData) > 64 {
					t.Errorf("%s: button %q callback %q is %d bytes (> 64)", name, btn.Text, btn.CallbackData, len(btn.CallbackData))
				}
				if !strings.HasPrefix(btn.CallbackData, settingsCallbackNamespace+"|") {
					continue
				}
				code := settingsButtonFields(t, btn)["a"]
				seenCodes[code] = max(seenCodes[code], len(btn.CallbackData))
			}
		}

		measure("main_open", renderSettingsMain(private, []settingsMainEntry{
			{Key: settingsPageReasons, Icon: def.Icon, Title: settingsText(tr, def.TitleKey), State: "50/50", Known: true},
		}, ""))
		measure("list_middle", renderSettingsList(private, def, entries, 5000, ""))
		measure("list_last", renderSettingsList(private, def, entries, 9999, ""))
		measure("list_empty", renderSettingsList(private, def, nil, 1, ""))
		measure("delete_card", renderSettingsListCard(private, def, settingsListEntry{ID: 99999999999999999, Text: "spam"}, 9999))
		until := time.Now().Add(settingsInputTTL)
		measure("prompt_step1", renderSettingsListPrompt(private, def, "w", until, true, ""))
		measure("prompt_step2", renderSettingsListPrompt(private, def, "r", until, true, "spam"))

		for _, code := range []string{settingsActOpen, "sg", "sx", "sy", "sa", "sc"} {
			if seenCodes[code] == 0 {
				t.Errorf("code %q was never measured", code)
			}
		}

		// 18-digit row ID is omitted rather than emitting an over-64-byte callback.
		over := []settingsListEntry{
			{ID: 1, Text: "ok", Detail: "r"},
			{ID: 123456789012345678, Text: "over", Detail: "r"},
		}
		if s := renderSettingsList(private, def, over, 1, ""); s.Omitted != 1 {
			t.Errorf("18-digit row ID Omitted = %d, want 1", s.Omitted)
		}
	})

	t.Run("all_8_languages_screen_lengths", func(t *testing.T) {
		withRealLocales(t)
		def := settingsReasonsDef()
		title := strings.Repeat("Ж", 64)
		longWord := strings.Repeat("&", models.ReasonShortcutWordMaxRunes)
		longReason := strings.Repeat("<script>&", 56)[:models.ReasonShortcutTextMaxRunes]

		entries := make([]settingsListEntry, 0, 16)
		for i := 0; i < 16; i++ {
			entries = append(entries, settingsListEntry{
				ID:     99999999999999900 + int64(i),
				Text:   longWord,
				Detail: longReason,
			})
		}

		for _, code := range memberReqScreenLanguages {
			t.Run(code, func(t *testing.T) {
				tr := i18n.MustNewTranslator(code)
				v := settingsView{
					Tr: tr, Private: true, OwnerKey: "c", OwnerValue: "-1001234567890",
					GroupID: -1001234567890, GroupTitle: title, OpenerName: title,
				}
				until := time.Now().Add(settingsInputTTL)
				screens := map[string]settingsScreen{
					"empty_list":   renderSettingsList(v, def, nil, 1, ""),
					"full_page":    renderSettingsList(v, def, entries, 1, settingsNoticeOK(settingsListEntryText(tr, def.RemovedKey, def.EntryParam, longWord))),
					"saved_page":   renderSettingsList(v, def, entries, 1, settingsNoticeOK(settingsListEntryText(tr, def.TwoStep.SavedKey, def.EntryParam, longWord))),
					"cap_page":     renderSettingsList(v, def, entries, 1, settingsNoticeWarn(settingsReasonsCapText(tr))),
					"delete_card":  renderSettingsListCard(v, def, entries[0], 2),
					"prompt_step1": renderSettingsListPrompt(v, def, "w", until, true, ""),
					"prompt_step2": renderSettingsListPrompt(v, def, "r", until, true, longWord),
					"refuse_word":  renderSettingsInputRefusalCode(v, def.CancelCode, "w", settingsText(tr, "settings_rsn_refuse_word", i18n.TranslationParams{"max": models.ReasonShortcutWordMaxRunes})),
					"refuse_long":  renderSettingsInputRefusalCode(v, def.CancelCode, "r", settingsText(tr, "settings_rsn_refuse_text_long", i18n.TranslationParams{"max": models.ReasonShortcutTextMaxRunes})),
				}
				for name, screen := range screens {
					if units := panelUTF16Len(screen.Text); units == 0 || units > 3800 {
						t.Errorf("%s/%s is %d UTF-16 units, want 1..3800", code, name, units)
					}
					if screen.Omitted != 0 {
						t.Errorf("%s/%s omitted %d buttons", code, name, screen.Omitted)
					}
					if strings.Contains(screen.Text, "<script>") {
						t.Errorf("%s/%s rendered unescaped HTML", code, name)
					}
					for _, row := range screen.Keyboard.InlineKeyboard {
						for _, btn := range row {
							if len(btn.CallbackData) > 64 {
								t.Errorf("%s/%s button %q callback %d bytes (> 64)", code, name, btn.Text, len(btn.CallbackData))
							}
							if strings.Contains(btn.CallbackData, "script") {
								t.Errorf("%s/%s leaked user text into callback %q", code, name, btn.CallbackData)
							}
							if !strings.HasPrefix(btn.Text, "🗑") && btn.Url == "" {
								limit := 40
								if len(row) > 1 {
									limit = 20
								}
								if runes := utf8.RuneCountInString(btn.Text); runes > limit {
									t.Errorf("%s/%s button %q has %d runes (> %d)", code, name, btn.Text, runes, limit)
								}
							}
						}
					}
				}
			})
		}
	})
}
