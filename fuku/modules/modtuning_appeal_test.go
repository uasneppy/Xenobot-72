//go:build testtools

package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/db/modtuning"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
	"github.com/uasneppy/fuku_robot/fuku/utils/content"
)

// appealCallKeyboard decodes reply_markup from a captured Telegram API call.
func appealCallKeyboard(t *testing.T, call moduleBotCall) [][]gotgbot.InlineKeyboardButton {
	t.Helper()
	raw, ok := call.Params["reply_markup"]
	if !ok || raw == nil {
		return nil
	}
	var kb gotgbot.InlineKeyboardMarkup
	switch v := raw.(type) {
	case string:
		if v == "" {
			return nil
		}
		if err := json.Unmarshal([]byte(v), &kb); err != nil {
			t.Fatalf("unmarshal reply_markup %q: %v", v, err)
		}
	default:
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal reply_markup %T: %v", v, err)
		}
		if err := json.Unmarshal(b, &kb); err != nil {
			t.Fatalf("unmarshal reply_markup %s: %v", string(b), err)
		}
	}
	return kb.InlineKeyboard
}

func resetSiteCalls(s *botSite) {
	s.client.mu.Lock()
	s.client.calls = nil
	s.client.mu.Unlock()
}

// inMemoryPMRedisHook is a socket-free go-redis ProcessHook supporting SET NX and GET so
// pmCopyAddDetails runs end-to-end without binding localhost TCP sockets.
type inMemoryPMRedisHook struct {
	mu   sync.Mutex
	data map[string][]byte
}

func (h *inMemoryPMRedisHook) DialHook(next redis.DialHook) redis.DialHook { return next }

func (h *inMemoryPMRedisHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		switch c := cmd.(type) {
		case *redis.BoolCmd:
			args := c.Args()
			if len(args) >= 3 && strings.EqualFold(fmt.Sprint(args[0]), "set") {
				key := fmt.Sprint(args[1])
				var val []byte
				switch v := args[2].(type) {
				case []byte:
					val = append([]byte(nil), v...)
				default:
					val = []byte(fmt.Sprint(v))
				}
				h.mu.Lock()
				_, exists := h.data[key]
				if !exists {
					h.data[key] = val
				}
				h.mu.Unlock()
				c.SetVal(!exists)
				return nil
			}
		case *redis.StringCmd:
			args := c.Args()
			if len(args) >= 2 && strings.EqualFold(fmt.Sprint(args[0]), "get") {
				key := fmt.Sprint(args[1])
				h.mu.Lock()
				val, ok := h.data[key]
				h.mu.Unlock()
				if !ok {
					c.SetErr(redis.Nil)
					return redis.Nil
				}
				c.SetVal(string(val))
				return nil
			}
		}
		return next(ctx, cmd)
	}
}

func (h *inMemoryPMRedisHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func newInMemoryPMRedis(t *testing.T) (*redis.Client, *inMemoryPMRedisHook) {
	t.Helper()
	hook := &inMemoryPMRedisHook{data: map[string][]byte{}}
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	client.AddHook(hook)
	t.Cleanup(func() { _ = client.Close() })
	return client, hook
}

func newAppealTestSite(t *testing.T) *botSite {
	t.Helper()
	withFastPMCopyPacer(t)
	s := newModerationGoldenSite(t)
	s.bot.BotClient = eventPMTestClient{BotClient: s.bot.BotClient, script: &eventPMScript{}}
	cleanupModTuningChat(t, s.chat.Id)
	eventMsgCleanup(t, s.chat.Id)
	t.Cleanup(pmCopyJobs.wait)
	return s
}

func TestAppealURLValidationAndApply(t *testing.T) {
	ctx := context.Background()
	const chatID int64 = -1009944001
	cleanupModTuningChat(t, chatID)

	t.Run("accepts_valid_and_clear", func(t *testing.T) {
		exact2048 := "https://example.com/" + strings.Repeat("a", models.ManualAppealURLMaxRunes-len("https://example.com/"))
		valid := []struct {
			in   string
			want string
		}{
			{"https://example.com/appeal", "https://example.com/appeal"},
			{"http://example.com/form?id=1", "http://example.com/form?id=1"},
			{"  https://example.com/trimmed \t\n", "https://example.com/trimmed"},
			{exact2048, exact2048},
			{"", ""},
		}
		for _, tc := range valid {
			if got := applyAppealURL(ctx, chatID, tc.in, 77); got != modTuningSaved {
				t.Fatalf("applyAppealURL(%q) = %v, want modTuningSaved", tc.in, got)
			}
			defs, err := modtuning.GetDefaultsFresh(ctx, chatID)
			if err != nil {
				t.Fatalf("GetDefaultsFresh: %v", err)
			}
			if defs.AppealURL != tc.want {
				t.Fatalf("stored AppealURL = %q, want %q", defs.AppealURL, tc.want)
			}
		}
	})

	t.Run("rejects_unsafe_malformed_whitespace_and_overlength_without_writing", func(t *testing.T) {
		const initial = "https://example.com/original"
		if got := applyAppealURL(ctx, chatID, initial, 77); got != modTuningSaved {
			t.Fatalf("seed applyAppealURL = %v, want modTuningSaved", got)
		}
		over2048 := "https://example.com/" + strings.Repeat("a", models.ManualAppealURLMaxRunes-len("https://example.com/")+1)
		invalid := []struct {
			name string
			raw  string
		}{
			{"javascript", "javascript:alert(1)"},
			{"tg_scheme", "tg://user?id=1"},
			{"mailto", "mailto:mod@example.com"},
			{"ftp", "ftp://example.com/appeal"},
			{"bare_host", "example.com/appeal"},
			{"no_host", "https://"},
			{"whitespace_only", "   \t\n "},
			{"internal_space", "https://example.com/a b"},
			{"internal_newline", "https://example.com/a\nb"},
			{"over_2048", over2048},
		}
		for _, tc := range invalid {
			if got := applyAppealURL(ctx, chatID, tc.raw, 77); got != modTuningInvalid {
				t.Errorf("%s: applyAppealURL(%q) = %v, want modTuningInvalid", tc.name, tc.raw, got)
			}
		}
		defs, err := modtuning.GetDefaultsFresh(ctx, chatID)
		if err != nil || defs.AppealURL != initial {
			t.Fatalf("stored AppealURL after rejections = (%q, %v), want %q untouched", defs.AppealURL, err, initial)
		}
	})

	t.Run("write_error_and_logs_never_contain_url", func(t *testing.T) {
		const secretURL = "https://appeal-secret-token.example.com/private-form?k=xyz987"
		prevWrite := modTuningWrites.SetAppealURL
		modTuningWrites.SetAppealURL = func(context.Context, int64, string, int64) error {
			return errors.New("write failed: " + secretURL)
		}
		t.Cleanup(func() { modTuningWrites.SetAppealURL = prevWrite })

		var buf bytes.Buffer
		prevOut := log.StandardLogger().Out
		prevLevel := log.GetLevel()
		log.SetOutput(&buf)
		log.SetLevel(log.DebugLevel)
		t.Cleanup(func() {
			log.SetOutput(prevOut)
			log.SetLevel(prevLevel)
		})

		if got := applyAppealURL(ctx, chatID, secretURL, 99); got != modTuningFailed {
			t.Fatalf("applyAppealURL on write error = %v, want modTuningFailed", got)
		}
		logged := buf.String()
		if strings.Contains(logged, "appeal-secret-token") || strings.Contains(logged, "xyz987") {
			t.Fatalf("appeal URL leaked into logs: %q", logged)
		}
		if !strings.Contains(logged, strconv.FormatInt(chatID, 10)) {
			t.Fatalf("expected chat ID %d in warning log, got %q", chatID, logged)
		}
	})

	t.Run("defensive_read_rechecks_stored_url", func(t *testing.T) {
		prev := manualDefaultsLookup
		manualDefaultsLookup = func(context.Context, int64) (modtuning.Defaults, error) {
			return modtuning.Defaults{AppealURL: "javascript:alert(1)"}, nil
		}
		t.Cleanup(func() { manualDefaultsLookup = prev })
		if got := manualAppealURLFor(chatID); got != "" {
			t.Fatalf("manualAppealURLFor(bad stored) = %q, want empty", got)
		}
	})
}

func TestAppealButtonOnBanMessages(t *testing.T) {
	ctx := context.Background()
	const appealURL = "https://example.com/appeal"
	withModerationGoldenLocales(t)
	ukTr := i18n.MustNewTranslator("uk")
	wantAppealLabel := trS(ukTr, "eventmsg_btn_appeal")
	if wantAppealLabel == "" {
		t.Fatal("eventmsg_btn_appeal missing in uk locale")
	}

	assertNoAppeal := func(t *testing.T, rows [][]gotgbot.InlineKeyboardButton) {
		t.Helper()
		for _, row := range rows {
			for _, btn := range row {
				if btn.Url == appealURL || btn.Text == wantAppealLabel {
					t.Fatalf("unexpected appeal button in keyboard: %+v", rows)
				}
			}
		}
	}

	assertLastRowAppeal := func(t *testing.T, rows [][]gotgbot.InlineKeyboardButton) {
		t.Helper()
		if len(rows) == 0 {
			t.Fatalf("expected keyboard with appeal button, got empty keyboard")
		}
		last := rows[len(rows)-1]
		if len(last) != 1 || last[0].Text != wantAppealLabel || last[0].Url != appealURL {
			t.Fatalf("last row = %+v, want single appeal button {%q, %q}", last, wantAppealLabel, appealURL)
		}
	}

	t.Run("unset_ban_and_tban_no_appeal_button", func(t *testing.T) {
		s := newAppealTestSite(t)
		if err := bansModule.ban(s.bot, s.commandCtx("/ban spam", true, false)); err != ext.EndGroups {
			t.Fatalf("/ban err = %v, want EndGroups", err)
		}
		sends := sendMessagesToChat(s, s.chat.Id)
		if len(sends) != 1 {
			t.Fatalf("/ban sends = %d, want 1", len(sends))
		}
		rows := appealCallKeyboard(t, sends[0])
		if len(rows) != 1 || len(rows[0]) != 1 {
			t.Fatalf("unset /ban keyboard = %+v, want only Unban row", rows)
		}
		assertNoAppeal(t, rows)

		resetSiteCalls(s)
		if err := bansModule.tBan(s.bot, s.commandCtx("/tban 2h spam", true, false)); err != ext.EndGroups {
			t.Fatalf("/tban err = %v, want EndGroups", err)
		}
		sends = sendMessagesToChat(s, s.chat.Id)
		if len(sends) != 1 {
			t.Fatalf("/tban sends = %d, want 1", len(sends))
		}
		if rows := appealCallKeyboard(t, sends[0]); len(rows) != 0 {
			t.Fatalf("unset /tban keyboard = %+v, want none", rows)
		}
	})

	t.Run("default_ban_tban_default_duration_and_lookup_fallback", func(t *testing.T) {
		s := newAppealTestSite(t)
		if err := modtuning.SetAppealURL(ctx, s.chat.Id, appealURL, 1); err != nil {
			t.Fatalf("SetAppealURL: %v", err)
		}

		// Default /ban: row 0 = Unban, row 1 = Appeal.
		if err := bansModule.ban(s.bot, s.commandCtx("/ban spam", true, false)); err != ext.EndGroups {
			t.Fatalf("/ban err = %v", err)
		}
		sends := sendMessagesToChat(s, s.chat.Id)
		if len(sends) != 1 {
			t.Fatalf("/ban sends = %d, want 1", len(sends))
		}
		rows := appealCallKeyboard(t, sends[0])
		if len(rows) != 2 || len(rows[0]) != 1 || rows[0][0].CallbackData == "" {
			t.Fatalf("default /ban keyboard = %+v, want [Unban, Appeal]", rows)
		}
		assertLastRowAppeal(t, rows)

		// Default /tban: row 0 = Appeal.
		resetSiteCalls(s)
		if err := bansModule.tBan(s.bot, s.commandCtx("/tban 2h spam", true, false)); err != ext.EndGroups {
			t.Fatalf("/tban err = %v", err)
		}
		sends = sendMessagesToChat(s, s.chat.Id)
		if len(sends) != 1 {
			t.Fatalf("/tban sends = %d, want 1", len(sends))
		}
		rows = appealCallKeyboard(t, sends[0])
		if len(rows) != 1 {
			t.Fatalf("default /tban keyboard = %+v, want [Appeal]", rows)
		}
		assertLastRowAppeal(t, rows)

		// Default /ban with default_ban_secs (EventTimedBan): row 0 = Appeal.
		if err := modtuning.SetDefaultBanSecs(ctx, s.chat.Id, 3600, 1); err != nil {
			t.Fatalf("SetDefaultBanSecs: %v", err)
		}
		resetSiteCalls(s)
		if err := bansModule.ban(s.bot, s.commandCtx("/ban spam", true, false)); err != ext.EndGroups {
			t.Fatalf("/ban with default duration err = %v", err)
		}
		sends = sendMessagesToChat(s, s.chat.Id)
		if len(sends) != 1 {
			t.Fatalf("/ban with default duration sends = %d, want 1", len(sends))
		}
		rows = appealCallKeyboard(t, sends[0])
		if len(rows) != 1 {
			t.Fatalf("default-duration /ban keyboard = %+v, want [Appeal]", rows)
		}
		assertLastRowAppeal(t, rows)

		// eventMessagesLookup failure still falls back to default WITH Appeal button.
		prevLookup := eventMessagesLookup
		eventMessagesLookup = func(context.Context, int64) (*eventmsg.Messages, error) {
			return nil, errors.New("db outage")
		}
		t.Cleanup(func() { eventMessagesLookup = prevLookup })
		resetSiteCalls(s)
		if err := bansModule.ban(s.bot, s.commandCtx("/ban spam", true, false)); err != ext.EndGroups {
			t.Fatalf("fallback /ban err = %v", err)
		}
		sends = sendMessagesToChat(s, s.chat.Id)
		if len(sends) != 1 {
			t.Fatalf("fallback /ban sends = %d, want 1", len(sends))
		}
		assertLastRowAppeal(t, appealCallKeyboard(t, sends[0]))
	})

	t.Run("custom_ban_places_appeal_after_custom_buttons", func(t *testing.T) {
		s := newAppealTestSite(t)
		if err := modtuning.SetAppealURL(ctx, s.chat.Id, appealURL, 1); err != nil {
			t.Fatalf("SetAppealURL: %v", err)
		}
		if err := eventmsg.SetText(ctx, s.chat.Id, eventmsg.EventBan, eventmsg.DestGroup, "Banned {first} ({reason})", 1); err != nil {
			t.Fatalf("SetText: %v", err)
		}
		if err := eventmsg.SetButtons(ctx, s.chat.Id, eventmsg.EventBan, eventmsg.DestGroup, []models.Button{
			{Name: "Rules", Url: "https://example.com/rules"},
		}, 1); err != nil {
			t.Fatalf("SetButtons: %v", err)
		}

		if err := bansModule.ban(s.bot, s.commandCtx("/ban spam", true, false)); err != ext.EndGroups {
			t.Fatalf("custom /ban err = %v", err)
		}
		sends := sendMessagesToChat(s, s.chat.Id)
		if len(sends) != 1 {
			t.Fatalf("custom /ban sends = %d, want 1", len(sends))
		}
		rows := appealCallKeyboard(t, sends[0])
		if len(rows) != 3 {
			t.Fatalf("custom /ban keyboard rows = %d (%+v), want 3 (Unban, Rules, Appeal)", len(rows), rows)
		}
		if rows[0][0].CallbackData == "" {
			t.Fatalf("row 0 = %+v, want Unban callback", rows[0])
		}
		if rows[1][0].Text != "Rules" || rows[1][0].Url != "https://example.com/rules" {
			t.Fatalf("row 1 = %+v, want custom Rules button", rows[1])
		}
		assertLastRowAppeal(t, rows)
	})

	t.Run("pm_copy_default_custom_and_refused_fallback", func(t *testing.T) {
		s := newAppealTestSite(t)
		if err := modtuning.SetAppealURL(ctx, s.chat.Id, appealURL, 1); err != nil {
			t.Fatalf("SetAppealURL: %v", err)
		}
		if err := eventmsg.SetSilent(ctx, s.chat.Id, eventmsg.EventBan, eventmsg.DestPM, false, 1); err != nil {
			t.Fatalf("enable PM copy: %v", err)
		}

		// Default PM copy carries Appeal button.
		if err := bansModule.ban(s.bot, s.commandCtx("/ban spam", true, false)); err != ext.EndGroups {
			t.Fatalf("/ban err = %v", err)
		}
		waitEventPM(t)
		pms := sendMessagesToChat(s, eventPMMember)
		if len(pms) != 1 {
			t.Fatalf("default PM sends = %d, want 1", len(pms))
		}
		rows := appealCallKeyboard(t, pms[0])
		if len(rows) != 1 {
			t.Fatalf("default PM keyboard = %+v, want [Appeal]", rows)
		}
		assertLastRowAppeal(t, rows)

		// Custom PM copy places Appeal button after custom button.
		if err := eventmsg.SetText(ctx, s.chat.Id, eventmsg.EventBan, eventmsg.DestPM, "Custom PM: {reason}", 1); err != nil {
			t.Fatalf("SetText PM: %v", err)
		}
		if err := eventmsg.SetButtons(ctx, s.chat.Id, eventmsg.EventBan, eventmsg.DestPM, []models.Button{
			{Name: "Guide", Url: "https://example.com/guide"},
		}, 1); err != nil {
			t.Fatalf("SetButtons PM: %v", err)
		}
		resetSiteCalls(s)
		if err := bansModule.ban(s.bot, s.commandCtx("/ban spam", true, false)); err != ext.EndGroups {
			t.Fatalf("custom PM /ban err = %v", err)
		}
		waitEventPM(t)
		pms = sendMessagesToChat(s, eventPMMember)
		if len(pms) != 1 {
			t.Fatalf("custom PM sends = %d, want 1", len(pms))
		}
		rows = appealCallKeyboard(t, pms[0])
		if len(rows) != 2 || rows[0][0].Text != "Guide" {
			t.Fatalf("custom PM keyboard = %+v, want [Guide, Appeal]", rows)
		}
		assertLastRowAppeal(t, rows)

		// Refused custom PM copy falls back to default PM text WITH Appeal button.
		resetSiteCalls(s)
		eventPMRefuse(t, s, &gotgbot.TelegramError{Code: 400, Description: "Bad Request: bad markup"})
		if err := bansModule.ban(s.bot, s.commandCtx("/ban spam", true, false)); err != ext.EndGroups {
			t.Fatalf("refused custom PM /ban err = %v", err)
		}
		waitEventPM(t)
		pms = sendMessagesToChat(s, eventPMMember)
		if len(pms) != 2 {
			t.Fatalf("refused custom PM sends = %d, want 2 (refused custom + fallback)", len(pms))
		}
		fallbackRows := appealCallKeyboard(t, pms[1])
		if len(fallbackRows) != 1 {
			t.Fatalf("fallback PM keyboard = %+v, want [Appeal]", fallbackRows)
		}
		assertLastRowAppeal(t, fallbackRows)
	})

	t.Run("undelivered_pm_with_details_preserves_appeal_button", func(t *testing.T) {
		s := newAppealTestSite(t)
		client, hook := newInMemoryPMRedis(t)
		prevRedis := pmCopyRedisClient
		pmCopyRedisClient = func() *redis.Client { return client }
		t.Cleanup(func() { pmCopyRedisClient = prevRedis })
		prevUser := pmCopyBotUsername
		pmCopyBotUsername = func(*gotgbot.Bot) string { return "xenobot72_test" }
		t.Cleanup(func() { pmCopyBotUsername = prevUser })

		if err := modtuning.SetAppealURL(ctx, s.chat.Id, appealURL, 1); err != nil {
			t.Fatalf("SetAppealURL: %v", err)
		}
		if err := eventmsg.SetSilent(ctx, s.chat.Id, eventmsg.EventBan, eventmsg.DestPM, false, 1); err != nil {
			t.Fatalf("enable PM copy: %v", err)
		}
		if err := eventmsg.SetPMDetails(ctx, s.chat.Id, true, 1); err != nil {
			t.Fatalf("SetPMDetails: %v", err)
		}

		// User blocked bot (403) so Details button is appended to group message.
		eventPMRefuse(t, s, &gotgbot.TelegramError{Code: 403, Description: "Forbidden: bot was blocked"})
		if err := bansModule.ban(s.bot, s.commandCtx("/ban spam", true, false)); err != ext.EndGroups {
			t.Fatalf("/ban err = %v", err)
		}
		waitEventPM(t)

		edits := s.client.callsFor("editMessageReplyMarkup")
		if len(edits) != 1 {
			t.Fatalf("editMessageReplyMarkup calls = %d, want 1", len(edits))
		}
		rows := appealCallKeyboard(t, edits[0])
		if len(rows) != 3 {
			t.Fatalf("edited group keyboard = %+v, want 3 rows (Unban, Appeal, Details)", rows)
		}
		if rows[0][0].CallbackData == "" {
			t.Fatalf("row 0 = %+v, want Unban", rows[0])
		}
		if rows[1][0].Text != wantAppealLabel || rows[1][0].Url != appealURL {
			t.Fatalf("row 1 = %+v, want preserved Appeal button", rows[1])
		}
		if !strings.Contains(rows[2][0].Url, "https://t.me/xenobot72_test?start=pmcopy_") {
			t.Fatalf("row 2 = %+v, want Details deep link button", rows[2])
		}
		// Stored Details record also preserves the PM copy's Appeal button.
		hook.mu.Lock()
		nRecs := len(hook.data)
		var recBytes []byte
		for _, v := range hook.data {
			recBytes = v
		}
		hook.mu.Unlock()
		if nRecs != 1 {
			t.Fatalf("stored Details records = %d, want 1", nRecs)
		}
		var rec pmCopyDetailsRecord
		if err := json.Unmarshal(recBytes, &rec); err != nil {
			t.Fatalf("unmarshal Details record: %v", err)
		}
		assertLastRowAppeal(t, rec.Buttons)
	})

	t.Run("automatic_bans_carry_appeal_and_non_bans_do_not", func(t *testing.T) {
		s := newAppealTestSite(t)
		if err := modtuning.SetAppealURL(ctx, s.chat.Id, appealURL, 1); err != nil {
			t.Fatalf("SetAppealURL: %v", err)
		}
		target := gotgbot.User{Id: 42, FirstName: "Ann"}
		userSender := &gotgbot.Sender{User: &target}
		chanSender := &gotgbot.Sender{Chat: &gotgbot.Chat{Id: -100555, Type: "channel", Title: "SpamChan"}}
		now := time.Now().Unix()

		banCases := []struct {
			name string
			run  func() error
		}{
			{"warnlimit_ban", func() error {
				r := warnLimitEventReply(ukTr, 10, target, "spam", warnApplyResult{Count: 3, Limit: 3, Mode: "ban"})
				return postEvent(listNoticeSite(s.bot, &s.chat, ukTr), r)
			}},
			{"warnlimit_tban", func() error {
				r := warnLimitEventReply(ukTr, 10, target, "spam", warnApplyResult{Count: 3, Limit: 3, Mode: "tban", Secs: 3600, Until: now + 3600})
				return postEvent(listNoticeSite(s.bot, &s.chat, ukTr), r)
			}},
			{"mreq_ban", func() error {
				text, opts := memberReqNoticeDefault(models.RequirementActionBan)(ukTr, eventDefaultInput{Target: target, Rules: memberReqRuleBits("un")})
				return postEvent(listNoticeSite(s.bot, &s.chat, ukTr), eventReply{
					Event: eventmsg.EventReqBan, Target: target, Default: text, Opts: opts,
				})
			}},
			{"blacklist_ban", func() error {
				r := listNoticeReply(ukTr, eventmsg.EventBlacklist, target, autoPunishment{Action: "ban"}, "bad word")
				return postEvent(listNoticeSite(s.bot, &s.chat, ukTr), r)
			}},
			{"blacklist_tban", func() error {
				r := listNoticeReply(ukTr, eventmsg.EventBlacklist, target, autoPunishment{Action: "tban", Secs: 3600, Until: now + 3600}, "bad word")
				return postEvent(listNoticeSite(s.bot, &s.chat, ukTr), r)
			}},
			{"fwdblock_ban", func() error {
				r := listNoticeReply(ukTr, eventmsg.EventForwardBlock, target, autoPunishment{Action: "ban"}, "blocked forward")
				return postEvent(listNoticeSite(s.bot, &s.chat, ukTr), r)
			}},
			{"flood_ban", func() error {
				return floodNotice(s.bot, &s.chat, &gotgbot.Message{MessageId: 10}, ukTr, userSender, autoPunishment{Action: "ban"}, 5, 6, nil)
			}},
			{"captcha_ban", func() error {
				prevLife := captchaFailureNoticeLife
				captchaFailureNoticeLife = time.Hour
				t.Cleanup(func() { captchaFailureNoticeLife = prevLife })
				return postCaptchaFailureNotice(s.bot, s.chat.Id, ukTr, target, "ban", 0)
			}},
		}
		for _, tc := range banCases {
			resetSiteCalls(s)
			if err := tc.run(); err != nil {
				t.Fatalf("%s: unexpected err: %v", tc.name, err)
			}
			sends := sendMessagesToChat(s, s.chat.Id)
			if len(sends) != 1 {
				t.Fatalf("%s: sends = %d, want 1", tc.name, len(sends))
			}
			assertLastRowAppeal(t, appealCallKeyboard(t, sends[0]))
		}

		nonBanCases := []struct {
			name string
			run  func() error
		}{
			{"warnlimit_mute", func() error {
				r := warnLimitEventReply(ukTr, 10, target, "spam", warnApplyResult{Count: 3, Limit: 3, Mode: "mute"})
				return postEvent(listNoticeSite(s.bot, &s.chat, ukTr), r)
			}},
			{"flood_mute", func() error {
				return floodNotice(s.bot, &s.chat, &gotgbot.Message{MessageId: 10}, ukTr, userSender, autoPunishment{Action: "mute"}, 5, 6, nil)
			}},
			{"flood_ban_channel", func() error {
				return floodNotice(s.bot, &s.chat, &gotgbot.Message{MessageId: 10}, ukTr, chanSender, autoPunishment{Action: "ban"}, 5, 6, nil)
			}},
			{"blacklist_mute", func() error {
				r := listNoticeReply(ukTr, eventmsg.EventBlacklist, target, autoPunishment{Action: "mute"}, "bad word")
				return postEvent(listNoticeSite(s.bot, &s.chat, ukTr), r)
			}},
			{"blacklist_ban_channel", func() error {
				r := listNoticeReply(ukTr, eventmsg.EventBlacklist, eventSenderTarget(chanSender), autoPunishment{Action: "ban"}, "bad word")
				return postEvent(listNoticeSite(s.bot, &s.chat, ukTr), r)
			}},
			{"captcha_kick", func() error {
				prevLife := captchaFailureNoticeLife
				captchaFailureNoticeLife = time.Hour
				t.Cleanup(func() { captchaFailureNoticeLife = prevLife })
				return postCaptchaFailureNotice(s.bot, s.chat.Id, ukTr, target, "kick", 0)
			}},
		}
		for _, tc := range nonBanCases {
			resetSiteCalls(s)
			if err := tc.run(); err != nil {
				t.Fatalf("%s: unexpected err: %v", tc.name, err)
			}
			sends := sendMessagesToChat(s, s.chat.Id)
			if len(sends) != 1 {
				t.Fatalf("%s: sends = %d, want 1", tc.name, len(sends))
			}
			assertNoAppeal(t, appealCallKeyboard(t, sends[0]))
		}
	})

	t.Run("non_ban_manual_commands_never_carry_appeal_and_non_ban_events_make_zero_lookups", func(t *testing.T) {
		s := newAppealTestSite(t)
		if err := modtuning.SetAppealURL(ctx, s.chat.Id, appealURL, 1); err != nil {
			t.Fatalf("SetAppealURL: %v", err)
		}

		cmdRuns := []struct {
			name string
			run  func() error
		}{
			{"mute", func() error { return mutesModule.mute(s.bot, s.commandCtx("/mute noise", true, false)) }},
			{"kick", func() error { return bansModule.kick(s.bot, s.commandCtx("/kick bye", true, false)) }},
			{"warn", func() error { return warnsModule.warnUser(s.bot, s.commandCtx("/warn careful", true, false)) }},
			{"unban", func() error { return bansModule.unban(s.bot, s.commandCtx("/unban 42", false, false)) }},
			{"unmute", func() error { return mutesModule.unmute(s.bot, s.commandCtx("/unmute 42", false, false)) }},
			{"ban_channel", func() error { return bansModule.ban(s.bot, s.commandCtx("/ban -1001234567890", true, true)) }},
		}
		for _, tc := range cmdRuns {
			resetSiteCalls(s)
			if err := tc.run(); err != ext.EndGroups {
				t.Fatalf("%s err = %v, want EndGroups", tc.name, err)
			}
			for _, call := range sendMessagesToChat(s, s.chat.Id) {
				assertNoAppeal(t, appealCallKeyboard(t, call))
			}
		}

		// Silent ban sends no group message at all.
		resetSiteCalls(s)
		if err := bansModule.sBan(s.bot, s.commandCtx("/sban spam", true, false)); err != ext.EndGroups {
			t.Fatalf("/sban err = %v, want EndGroups", err)
		}
		if got := sendMessagesToChat(s, s.chat.Id); len(got) != 0 {
			t.Fatalf("/sban sent %d messages, want 0", len(got))
		}

		// Direct postEvent calls for non-ban events perform zero manualDefaultsLookup calls.
		var lookups atomic.Int32
		prevLookup := manualDefaultsLookup
		manualDefaultsLookup = func(c context.Context, chatID int64) (modtuning.Defaults, error) {
			lookups.Add(1)
			return prevLookup(c, chatID)
		}
		t.Cleanup(func() { manualDefaultsLookup = prevLookup })

		target := gotgbot.User{Id: 42, FirstName: "Ann"}
		for _, ev := range []string{
			eventmsg.EventKick, eventmsg.EventMute, eventmsg.EventTimedMute,
			eventmsg.EventUnban, eventmsg.EventUnmute, eventmsg.EventWarn,
		} {
			if err := postEvent(listNoticeSite(s.bot, &s.chat, ukTr), eventReply{
				Event: ev, Target: target, Default: "notice",
			}); err != nil {
				t.Fatalf("postEvent(%s): %v", ev, err)
			}
		}
		if n := lookups.Load(); n != 0 {
			t.Fatalf("non-ban postEvent performed %d manualDefaultsLookup calls, want 0", n)
		}
	})

	t.Run("hundred_button_cap_omits_appeal_button", func(t *testing.T) {
		s := newAppealTestSite(t)
		if err := modtuning.SetAppealURL(ctx, s.chat.Id, appealURL, 1); err != nil {
			t.Fatalf("SetAppealURL: %v", err)
		}
		makeButtons := func(n int) []models.Button {
			out := make([]models.Button, n)
			for i := range n {
				out[i] = models.Button{
					Name:     fmt.Sprintf("B%d", i+1),
					Url:      fmt.Sprintf("https://example.com/%d", i+1),
					SameLine: i%4 != 0,
				}
			}
			return out
		}

		// 99 custom buttons + 1 Unban = 100 buttons -> Appeal omitted (stays 100, never 101).
		if err := eventmsg.SetButtons(ctx, s.chat.Id, eventmsg.EventBan, eventmsg.DestGroup, makeButtons(99), 1); err != nil {
			t.Fatalf("SetButtons(99): %v", err)
		}
		if err := bansModule.ban(s.bot, s.commandCtx("/ban spam", true, false)); err != ext.EndGroups {
			t.Fatalf("99-button /ban err = %v", err)
		}
		sends := sendMessagesToChat(s, s.chat.Id)
		if len(sends) != 1 {
			t.Fatalf("99-button /ban sends = %d, want 1", len(sends))
		}
		rows := appealCallKeyboard(t, sends[0])
		if got := pmCopyButtonCount(rows); got != 100 {
			t.Fatalf("99 custom + Unban button count = %d, want 100", got)
		}
		assertNoAppeal(t, rows)

		// 98 custom buttons + 1 Unban = 99 buttons -> Appeal appended (100 total).
		if err := eventmsg.SetButtons(ctx, s.chat.Id, eventmsg.EventBan, eventmsg.DestGroup, makeButtons(98), 1); err != nil {
			t.Fatalf("SetButtons(98): %v", err)
		}
		resetSiteCalls(s)
		if err := bansModule.ban(s.bot, s.commandCtx("/ban spam", true, false)); err != ext.EndGroups {
			t.Fatalf("98-button /ban err = %v", err)
		}
		sends = sendMessagesToChat(s, s.chat.Id)
		if len(sends) != 1 {
			t.Fatalf("98-button /ban sends = %d, want 1", len(sends))
		}
		rows = appealCallKeyboard(t, sends[0])
		if got := pmCopyButtonCount(rows); got != 100 {
			t.Fatalf("98 custom + Unban + Appeal button count = %d, want 100", got)
		}
		assertLastRowAppeal(t, rows)

		// PM copy with 100 custom buttons -> Appeal omitted (stays 100).
		spec, _ := eventPMSpecFor(eventmsg.EventBan)
		pmMsg := composeEventPM(s.bot, &s.chat, ukTr, spec, eventReply{
			Event:     eventmsg.EventBan,
			Target:    gotgbot.User{Id: 42, FirstName: "Ann"},
			AppealURL: appealURL,
		}, eventmsg.Message{Buttons: makeButtons(100)}, eventActor{})
		if pmMsg.Keyboard == nil || pmCopyButtonCount(pmMsg.Keyboard.InlineKeyboard) != 100 {
			t.Fatalf("100-button PM copy count = %d, want 100", pmCopyButtonCount(pmMsg.Keyboard.InlineKeyboard))
		}
		assertNoAppeal(t, pmMsg.Keyboard.InlineKeyboard)
	})

	t.Run("messages_page_previews_and_reserved_buttons", func(t *testing.T) {
		// Verify ReservedButtons on group & PM definitions.
		wantGroupReserve := map[string]int{
			"bn": 2, // Unban + Appeal
			"tb": 1, // Appeal
			"rb": 1, // Appeal
			"mu": 1, // Unmute only
			"kk": 0,
		}
		for code, want := range wantGroupReserve {
			fam, ev, ok := settingsMsgEventByCode(code)
			if !ok {
				t.Fatalf("settingsMsgEventByCode(%q) not found", code)
			}
			def := settingsMsgEditDef(fam, ev)
			if def.ReservedButtons != want {
				t.Errorf("group %s ReservedButtons = %d, want %d", code, def.ReservedButtons, want)
			}
			pmDef := settingsMsgPMEditDef(fam, ev)
			wantPM := 0
			if code == "bn" || code == "tb" || code == "rb" {
				wantPM = 1
			}
			if pmDef.ReservedButtons != wantPM {
				t.Errorf("PM %s ReservedButtons = %d, want %d", code, pmDef.ReservedButtons, wantPM)
			}
		}

		// Preview tests (zero sockets, direct hook invocation).
		s := newAppealTestSite(t)
		presser := gotgbot.User{Id: 501, FirstName: "Owner"}
		p := &settingsPress{
			b:     s.bot,
			group: &s.chat,
			query: &gotgbot.CallbackQuery{From: presser},
		}
		fam, banEv, _ := settingsMsgEventByCode("bn")
		_, muteEv, _ := settingsMsgEventByCode("mu")
		banDef := settingsMsgEditDef(fam, banEv)
		banPMDef := settingsMsgPMEditDef(fam, banEv)
		st := settingsEditState{
			Text:    "Custom ban {first}",
			Buttons: []models.Button{{Name: "Rules", Url: "https://example.com/rules"}},
		}

		// Unset appeal URL: Full group preview and Full PM preview omit Appeal button.
		if err := settingsMsgPreview(banEv)(p, banDef, st, ""); err != nil {
			t.Fatalf("unset group preview: %v", err)
		}
		if err := settingsMsgPMPreview(banEv)(p, banPMDef, st, ""); err != nil {
			t.Fatalf("unset PM preview: %v", err)
		}
		previews := sendMessagesToChat(s, presser.Id)
		if len(previews) != 2 {
			t.Fatalf("unset previews sent = %d, want 2", len(previews))
		}
		assertNoAppeal(t, appealCallKeyboard(t, previews[0]))
		assertNoAppeal(t, appealCallKeyboard(t, previews[1]))

		// Set appeal URL: Full group preview and Full PM preview include Appeal button after custom button.
		if err := modtuning.SetAppealURL(ctx, s.chat.Id, appealURL, 1); err != nil {
			t.Fatalf("SetAppealURL: %v", err)
		}
		resetSiteCalls(s)
		if err := settingsMsgPreview(banEv)(p, banDef, st, ""); err != nil {
			t.Fatalf("set group preview: %v", err)
		}
		if err := settingsMsgPMPreview(banEv)(p, banPMDef, st, ""); err != nil {
			t.Fatalf("set PM preview: %v", err)
		}
		previews = sendMessagesToChat(s, presser.Id)
		if len(previews) != 2 {
			t.Fatalf("set previews sent = %d, want 2", len(previews))
		}
		groupPrevRows := appealCallKeyboard(t, previews[0])
		if len(groupPrevRows) != 2 || groupPrevRows[0][0].Text != "Rules" {
			t.Fatalf("group preview rows = %+v, want [Rules, Appeal]", groupPrevRows)
		}
		assertLastRowAppeal(t, groupPrevRows)
		pmPrevRows := appealCallKeyboard(t, previews[1])
		if len(pmPrevRows) != 2 || pmPrevRows[0][0].Text != "Rules" {
			t.Fatalf("PM preview rows = %+v, want [Rules, Appeal]", pmPrevRows)
		}
		assertLastRowAppeal(t, pmPrevRows)

		// Text-only preview and non-ban (Mute) preview do not attach Appeal button.
		resetSiteCalls(s)
		if err := settingsMsgPreview(banEv)(p, banDef, st, settingsEditPartText); err != nil {
			t.Fatalf("text-only group preview: %v", err)
		}
		if err := settingsMsgPreview(muteEv)(p, settingsMsgEditDef(fam, muteEv), st, ""); err != nil {
			t.Fatalf("mute group preview: %v", err)
		}
		for _, call := range sendMessagesToChat(s, presser.Id) {
			assertNoAppeal(t, appealCallKeyboard(t, call))
		}
	})

	t.Run("staff_group_and_raid_isolation", func(t *testing.T) {
		entries, err := os.ReadDir(".")
		if err != nil {
			t.Fatalf("ReadDir: %v", err)
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			if !strings.HasPrefix(name, "staff") && !strings.HasPrefix(name, "lockdown") {
				continue
			}
			raw, err := os.ReadFile(filepath.Clean(name))
			if err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			src := string(raw)
			for _, forbidden := range []string{"manualAppealURLFor", "eventAppealRow", "eventAppendAppealRow"} {
				if strings.Contains(src, forbidden) {
					t.Errorf("%s must stay isolated from appeal helpers, found %q", name, forbidden)
				}
			}
		}
		if !content.ValidButtonURL(appealURL) {
			t.Fatalf("content.ValidButtonURL(%q) = false", appealURL)
		}
	})
}
