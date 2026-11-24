//go:build testtools

package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/deletion"
	"github.com/uasneppy/Xenobot-72/xenobot/db/eventmsg"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/media"
)

var (
	plainAllLocales       = []string{"en", "es", "fr", "hi", "id", "pt", "ru", "uk"}
	plainPlaceholderRegex = regexp.MustCompile(`\{[a-z_]+\}`)
	plainForbiddenEmojis  = []string{
		"🔨", "👢", "🔇", "🔒", "🔓", "⚠️", "🚨", "🤖",
		"🚫", "⏳", "🔕", "🔊", "🚩", "⛔", "🌊", "🔤",
		"🚷", "🧩", "🧠", "❗", "❌", "📝",
	}
)

func assertNoThemedEmoji(t *testing.T, label, text string) {
	t.Helper()
	for _, emoji := range plainForbiddenEmojis {
		if strings.Contains(text, emoji) {
			t.Errorf("%s contains themed emoji %q in %q", label, emoji, text)
		}
	}
}

func TestPlainToneEveryEventEveryLocale(t *testing.T) {
	withRealLocales(t)

	// 1. Every one of the 23 milestone v1.2 events must exist in settingsMsgFamilies
	//    with non-nil Default and PlainDefault, and all 18 PM specs must carry both keys.
	byEvent := map[string]settingsMsgEvent{}
	for _, fam := range settingsMsgFamilies {
		for _, ev := range fam.Events {
			byEvent[ev.Key] = ev
		}
	}
	if len(eventmsg.Events) != 23 {
		t.Fatalf("eventmsg.Events count = %d, want 23", len(eventmsg.Events))
	}
	for _, evKey := range eventmsg.Events {
		ev, ok := byEvent[evKey]
		if !ok {
			t.Fatalf("event %q missing from settingsMsgFamilies", evKey)
		}
		if ev.Default == nil {
			t.Fatalf("event %q has nil Default", evKey)
		}
		if ev.PlainDefault == nil {
			t.Fatalf("event %q has nil PlainDefault", evKey)
		}
	}
	if len(eventPMSpecs) != 18 {
		t.Fatalf("eventPMSpecs count = %d, want 18", len(eventPMSpecs))
	}
	for evKey, spec := range eventPMSpecs {
		if spec.DefaultKey == "" || spec.PlainKey == "" {
			t.Fatalf("eventPMSpecs[%q] missing key: DefaultKey=%q PlainKey=%q", evKey, spec.DefaultKey, spec.PlainKey)
		}
	}

	const unsafeName = `Alice <script>&"1`
	const unsafeReason = `bad <script>&"reason`
	const unsafeGroup = `Group <script>&"Title`
	escapedName := html.EscapeString(unsafeName)
	escapedReason := html.EscapeString(unsafeReason)
	escapedGroup := html.EscapeString(unsafeGroup)
	target := gotgbot.User{Id: 7001, FirstName: unsafeName}
	chat := &gotgbot.Chat{Id: -1007777001, Type: gotgbot.ChatTypeSupergroup, Title: unsafeGroup}
	untilTime := time.Date(2026, 12, 1, 14, 30, 0, 0, time.UTC)

	// 2. Render every one of the 23 group events (plus variants) across all 8 languages and both tones.
	for _, lang := range plainAllLocales {
		tr := i18n.MustNewTranslator(lang)
		for _, evKey := range eventmsg.Events {
			ev := byEvent[evKey]
			in := eventDefaultInput{
				Target:  target,
				Reason:  unsafeReason,
				TimeVal: "2h",
				Count:   ev.Sample.Count,
				Limit:   ev.Sample.Limit,
				Action:  ev.Sample.Action,
				Rules:   ev.Sample.Rules,
				Window:  ev.Sample.Window,
				Percent: ev.Sample.Percent,
			}
			if evKey == eventmsg.EventWarnLimit {
				in.Reasons = []string{unsafeReason}
			}
			if evKey == eventmsg.EventForwardBlock {
				in.Reason, _ = tr.GetString("staff_fwdblock_reason")
			}
			if evKey == eventmsg.EventLockdown || evKey == eventmsg.EventLockJoinSurge ||
				evKey == eventmsg.EventLockFlood || evKey == eventmsg.EventLockAIBurst {
				in.Lockdown = lockdownFacts{NoDelete: true, NoInvite: true}
			}
			if evKey == eventmsg.EventLockLift {
				in.Reason = ""
				in.Lockdown = lockdownFacts{ManualChange: true, Unbanning: 4}
			}

			themedText, _ := ev.Default(tr, in)
			plainText := ev.PlainDefault(tr, in)

			for _, tc := range []struct {
				tone string
				text string
				isPl bool
			}{
				{"themed", themedText, false},
				{"plain", plainText, true},
			} {
				label := fmt.Sprintf("group[%s/%s/%s]", lang, evKey, tc.tone)
				if strings.TrimSpace(tc.text) == "" {
					t.Errorf("%s rendered empty", label)
					continue
				}
				if gotLen := utf16Len(tc.text); gotLen > 4096 {
					t.Errorf("%s UTF-16 len = %d, want <= 4096", label, gotLen)
				}
				if m := plainPlaceholderRegex.FindString(tc.text); m != "" {
					t.Errorf("%s has unexpanded placeholder %q in %q", label, m, tc.text)
				}
				if strings.Contains(tc.text, "<script>") {
					// Legacy untimed blacklistNoticeDefaultReply splices raw reason by design;
					// plain tone must ALWAYS escape everything (T-29-05).
					if tc.isPl || evKey != eventmsg.EventBlacklist {
						t.Errorf("%s leaked unescaped <script> in %q", label, tc.text)
					}
				}
				if tc.isPl {
					assertNoThemedEmoji(t, label, tc.text)
					if evKey != eventmsg.EventLockJoinSurge && evKey != eventmsg.EventLockFlood && evKey != eventmsg.EventLockAIBurst {
						if !strings.Contains(tc.text, escapedName) {
							t.Errorf("%s missing escaped target %q in %q", label, escapedName, tc.text)
						}
					}
					if evKey == eventmsg.EventBan || evKey == eventmsg.EventTimedBan || evKey == eventmsg.EventKick ||
						evKey == eventmsg.EventMute || evKey == eventmsg.EventTimedMute || evKey == eventmsg.EventWarn ||
						evKey == eventmsg.EventWarnLimit || evKey == eventmsg.EventBlacklist || evKey == eventmsg.EventLockdown {
						if !strings.Contains(tc.text, escapedReason) {
							t.Errorf("%s missing escaped reason %q in %q", label, escapedReason, tc.text)
						}
					}
				}
			}

			if lang == "en" && plainText == themedText {
				t.Errorf("en group event %q plain text equals themed text: %q", evKey, plainText)
			}
		}

		// Extra variants in every language: timed auto punishments, captcha with 0 msgs,
		// mreq warn-limit actions, and nameless lockdown/locklift.
		extraCases := []struct {
			name string
			text string
		}{
			{"warnlimit_kick", warnLimitPlainDefaultReply(tr, eventDefaultInput{Target: target, Count: 3, Limit: 3, Action: "kick", Reasons: []string{unsafeReason}})},
			{"warnlimit_ban", warnLimitPlainDefaultReply(tr, eventDefaultInput{Target: target, Count: 3, Limit: 3, Action: "ban"})},
			{"warnlimit_tmute", warnLimitPlainDefaultReply(tr, eventDefaultInput{Target: target, Count: 3, Limit: 3, Action: "tmute", Secs: 3600})},
			{"warnlimit_tban", warnLimitPlainDefaultReply(tr, eventDefaultInput{Target: target, Count: 3, Limit: 3, Action: "tban", Secs: 7200})},
			{"flood_tmute", floodPlainDefaultReply(tr, eventDefaultInput{Target: target, Action: "tmute", Secs: 1800})},
			{"flood_tban", floodPlainDefaultReply(tr, eventDefaultInput{Target: target, Action: "tban", Secs: 3600})},
			{"blacklist_tmute", blacklistPlainDefaultReply(tr, eventDefaultInput{Target: target, Action: "tmute", Secs: 1800, Reason: unsafeReason})},
			{"blacklist_tban", blacklistPlainDefaultReply(tr, eventDefaultInput{Target: target, Action: "tban", Secs: 3600, Reason: unsafeReason})},
			{"captcha_zero_msgs", captchaFailurePlainDefaultReply(tr, eventDefaultInput{Target: target, Action: "ban", Count: 0})},
			{"mreq_warn_limit_kick", memberReqPlainNoticeDefault(models.RequirementActionWarn)(tr, eventDefaultInput{Target: target, Count: 3, Limit: 3, Action: "kick", Rules: memberReqRuleBits("un", "ph")})},
			{"mreq_warn_limit_ban", memberReqPlainNoticeDefault(models.RequirementActionWarn)(tr, eventDefaultInput{Target: target, Count: 3, Limit: 3, Action: "ban", Rules: memberReqRuleBits("un")})},
			{"mreq_warn_limit_mute", memberReqPlainNoticeDefault(models.RequirementActionWarn)(tr, eventDefaultInput{Target: target, Count: 3, Limit: 3, Action: "mute", Rules: memberReqRuleBits("un")})},
			{"lockdown_no_admin", lockdownStartPlainDefaultReply(tr, eventDefaultInput{Reason: unsafeReason})},
			{"locklift_no_admin", lockdownLiftPlainDefaultReply(tr, eventDefaultInput{})},
		}
		for _, ec := range extraCases {
			label := fmt.Sprintf("extra[%s/%s]", lang, ec.name)
			if strings.TrimSpace(ec.text) == "" {
				t.Errorf("%s rendered empty", label)
			}
			if utf16Len(ec.text) > 4096 {
				t.Errorf("%s UTF-16 len = %d, want <= 4096", label, utf16Len(ec.text))
			}
			if m := plainPlaceholderRegex.FindString(ec.text); m != "" {
				t.Errorf("%s has unexpanded placeholder %q in %q", label, m, ec.text)
			}
			if strings.Contains(ec.text, "<script>") {
				t.Errorf("%s leaked unescaped <script> in %q", label, ec.text)
			}
			assertNoThemedEmoji(t, label, ec.text)
		}

		// Requirement notice length check: base single-rule notice <= 520 UTF-16 units,
		// and worst-case with remedies <= 4096.
		for _, action := range []int{
			models.RequirementActionWarn,
			models.RequirementActionKick,
			models.RequirementActionBan,
			models.RequirementActionMute,
		} {
			single := memberReqPlainNoticeDefault(action)(tr, eventDefaultInput{
				Target: gotgbot.User{Id: 7001, FirstName: "Alice"},
				Count:  1,
				Limit:  3,
				Rules:  memberReqRuleBits("un"),
			})
			if gotLen := utf16Len(single); gotLen > 520 {
				t.Errorf("mreq[%s/action=%d] single-rule UTF-16 len = %d, want <= 520: %q", lang, action, gotLen, single)
			}
		}

		// 3. Render every one of the 18 PM specs in both tones across all 8 languages.
		for evKey, spec := range eventPMSpecs {
			values := eventValues{
				Reason: unsafeReason,
				Count:  2,
				Limit:  3,
				Action: eventActionWord(tr, "mute"),
			}
			if evKey == eventmsg.EventTimedBan || evKey == eventmsg.EventTimedMute || spec.UntilLine {
				values.Until = untilTime
			}
			var pmNote string
			if evKey == eventmsg.EventUnmute {
				pmNote, _ = tr.GetString("eventmsg_pm_unmute_lockdown")
			}

			themedPM := eventPMDefaultText(tr, spec, chat, values, pmNote, false)
			plainPM := eventPMDefaultText(tr, spec, chat, values, pmNote, true)

			wantGroupTag := "<b>" + escapedGroup + "</b>"
			for _, tc := range []struct {
				tone string
				text string
				isPl bool
			}{
				{"themed", themedPM, false},
				{"plain", plainPM, true},
			} {
				label := fmt.Sprintf("pm[%s/%s/%s]", lang, evKey, tc.tone)
				if strings.TrimSpace(tc.text) == "" {
					t.Errorf("%s rendered empty", label)
					continue
				}
				if gotLen := utf16Len(tc.text); gotLen > 4096 {
					t.Errorf("%s UTF-16 len = %d, want <= 4096", label, gotLen)
				}
				if m := plainPlaceholderRegex.FindString(tc.text); m != "" {
					t.Errorf("%s has unexpanded placeholder %q in %q", label, m, tc.text)
				}
				if strings.Contains(tc.text, "<script>") {
					t.Errorf("%s leaked unescaped <script> in %q", label, tc.text)
				}
				if !strings.Contains(tc.text, wantGroupTag) {
					t.Errorf("%s missing %q in %q", label, wantGroupTag, tc.text)
				}
				if !strings.Contains(tc.text, escapedReason) {
					t.Errorf("%s missing escaped reason %q in %q", label, escapedReason, tc.text)
				}
				if tc.isPl {
					assertNoThemedEmoji(t, label, tc.text)
				}
			}

			// Verify identical filled dynamic values between themed and plain PM outputs.
			if values.Action != "" && strings.Contains(themedPM, values.Action) != strings.Contains(plainPM, values.Action) {
				t.Errorf("pm[%s/%s] action parity mismatch: themed=%q plain=%q", lang, evKey, themedPM, plainPM)
			}
			if !values.Until.IsZero() {
				untilStr := formatChatTime(values.Until, chatClockOf(chat.Id), "02.01.2006 15:04")
				if strings.Contains(themedPM, untilStr) != strings.Contains(plainPM, untilStr) {
					t.Errorf("pm[%s/%s] until parity mismatch: themed=%q plain=%q", lang, evKey, themedPM, plainPM)
				}
			}
			if evKey == eventmsg.EventWarn || evKey == eventmsg.EventWarnLimit || evKey == eventmsg.EventReqWarn {
				if !strings.Contains(plainPM, strconv.Itoa(values.Count)) || !strings.Contains(plainPM, strconv.Itoa(values.Limit)) {
					t.Errorf("pm[%s/%s] missing count/limit in plain PM: %q", lang, evKey, plainPM)
				}
			}
			if lang == "en" && plainPM == themedPM {
				t.Errorf("en PM event %q plain text equals themed text: %q", evKey, plainPM)
			}
		}
	}
}

type plainRuntimeCall struct {
	Method string
	Params map[string]any
}

type plainRuntimeBotClient struct {
	mu       sync.Mutex
	calls    []plainRuntimeCall
	refusals map[int64][]error
}

func (c *plainRuntimeBotClient) RequestWithContext(_ context.Context, _ string, method string, params map[string]any, _ *gotgbot.RequestOpts) (json.RawMessage, error) {
	c.mu.Lock()
	copied := make(map[string]any, len(params))
	for k, v := range params {
		copied[k] = v
	}
	c.calls = append(c.calls, plainRuntimeCall{Method: method, Params: copied})
	chatID, _ := strconv.ParseInt(fmt.Sprint(params["chat_id"]), 10, 64)
	var refusal error
	if q := c.refusals[chatID]; len(q) > 0 {
		refusal, c.refusals[chatID] = q[0], q[1:]
	}
	c.mu.Unlock()
	if refusal != nil {
		return nil, refusal
	}
	if strings.HasPrefix(method, "send") {
		return json.RawMessage(fmt.Sprintf(`{"message_id":8001,"date":%d,"chat":{"id":%d,"type":"private"}}`, time.Now().Unix(), chatID)), nil
	}
	return json.RawMessage(`true`), nil
}

func (c *plainRuntimeBotClient) GetAPIURL(*gotgbot.RequestOpts) string {
	return "https://api.telegram.org"
}
func (c *plainRuntimeBotClient) FileURL(string, string, *gotgbot.RequestOpts) string {
	return ""
}

func (c *plainRuntimeBotClient) sentTo(chatID int64) []plainRuntimeCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	want := strconv.FormatInt(chatID, 10)
	var out []plainRuntimeCall
	for _, call := range c.calls {
		if strings.HasPrefix(call.Method, "send") && fmt.Sprint(call.Params["chat_id"]) == want {
			out = append(out, call)
		}
	}
	return out
}

func newPlainRuntimeEnv(t *testing.T, chatID int64) (*gotgbot.Bot, *plainRuntimeBotClient, *gotgbot.Chat, *i18n.Translator) {
	t.Helper()
	withRealLocales(t)
	_ = phase28InProcessMiniredis(t)
	withFastPMCopyPacer(t)
	settingsMsgCleanup(t, chatID)
	t.Cleanup(func() { waitEventPM(t) })

	client := &plainRuntimeBotClient{refusals: map[int64][]error{}}
	bot := &gotgbot.Bot{
		Token:     "123:plain-runtime-test",
		User:      gotgbot.User{Id: 999, IsBot: true, FirstName: "Bot", Username: "testbot"},
		BotClient: client,
	}
	chat := &gotgbot.Chat{Id: chatID, Type: gotgbot.ChatTypeSupergroup, Title: "Plain Test Group"}
	tr := i18n.MustNewTranslator("en")
	return bot, client, chat, tr
}

func TestPlainToneRuntime(t *testing.T) {
	target := gotgbot.User{Id: 4242, FirstName: "Bob"}
	admin := gotgbot.User{Id: 1001, FirstName: "Mod"}

	t.Run("plain_tone_off_matches_default", func(t *testing.T) {
		const chatID int64 = -100290301
		bot, _, chat, tr := newPlainRuntimeEnv(t, chatID)
		banIn := eventDefaultInput{Target: target, Reason: "spam"}
		defText, defOpts := banDefaultReply(tr, banIn)

		plainCalls := 0
		var gotText string
		site := eventSite{
			Bot:   bot,
			Chat:  chat,
			Tr:    tr,
			Actor: eventActor{User: &admin},
			Kind:  deletion.KindBotManual,
			SendDefault: func(text string, _ *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
				gotText = text
				return &gotgbot.Message{MessageId: 101, Chat: *chat}, nil
			},
			Schedule: func(*gotgbot.Message) {},
		}
		_, err := postEventSent(site, eventReply{
			Event:   eventmsg.EventBan,
			Target:  target,
			Values:  eventValues{Reason: "spam"},
			Default: defText,
			PlainDefault: func() string {
				plainCalls++
				return banPlainDefaultReply(tr, banIn)
			},
			Opts: defOpts,
		})
		if err != nil {
			t.Fatalf("postEventSent: %v", err)
		}
		if plainCalls != 0 {
			t.Fatalf("PlainDefault invoked %d times when PlainTone=false, want 0", plainCalls)
		}
		if gotText != defText {
			t.Fatalf("sent text = %q, want default %q", gotText, defText)
		}
	})

	t.Run("plain_tone_on_replaces_default_and_preserves_buttons_and_timers", func(t *testing.T) {
		const chatID int64 = -100290302
		bot, _, chat, tr := newPlainRuntimeEnv(t, chatID)
		if err := eventmsg.SetPlainTone(context.Background(), chatID, true, admin.Id); err != nil {
			t.Fatalf("SetPlainTone: %v", err)
		}

		// 1. Manual ban keeps Unban button + Appeal row + manual deletion kind.
		banIn := eventDefaultInput{Target: target, Reason: "spam"}
		defText, defOpts := banDefaultReply(tr, banIn)
		wantBanPlain := banPlainDefaultReply(tr, banIn)
		var gotText string
		var gotOpts *gotgbot.SendMessageOpts
		scheduledCount := 0
		site := eventSite{
			Bot:   bot,
			Chat:  chat,
			Tr:    tr,
			Actor: eventActor{User: &admin},
			Kind:  deletion.KindBotManual,
			SendDefault: func(text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
				gotText, gotOpts = text, opts
				return &gotgbot.Message{MessageId: 201, Chat: *chat}, nil
			},
			Schedule: func(m *gotgbot.Message) {
				if m != nil && m.MessageId == 201 {
					scheduledCount++
				}
			},
		}
		_, err := postEventSent(site, eventReply{
			Event:        eventmsg.EventBan,
			Target:       target,
			Values:       eventValues{Reason: "spam"},
			Default:      defText,
			PlainDefault: func() string { return banPlainDefaultReply(tr, banIn) },
			Opts:         defOpts,
			Appeal:       true,
			AppealURL:    "https://example.com/appeal",
		})
		if err != nil {
			t.Fatalf("postEventSent(ban): %v", err)
		}
		if gotText != wantBanPlain {
			t.Fatalf("plain ban text = %q, want %q", gotText, wantBanPlain)
		}
		if scheduledCount != 1 {
			t.Fatalf("Schedule calls = %d, want 1", scheduledCount)
		}
		kb, ok := gotOpts.ReplyMarkup.(gotgbot.InlineKeyboardMarkup)
		if !ok || len(kb.InlineKeyboard) != 2 {
			t.Fatalf("ban plain reply_markup rows = %#v, want 2 rows (Unban + Appeal)", gotOpts.ReplyMarkup)
		}

		// 2. Automatic mreqmute keeps "I fixed it" row and auto schedule.
		fixRow := [][]gotgbot.InlineKeyboardButton{{{Text: "I fixed it", CallbackData: "mreq:fix:4242"}}}
		mreqIn := eventDefaultInput{Target: target, Rules: memberReqRuleBits("un"), Rows: fixRow}
		mreqDef, mreqOpts := memberReqNoticeDefault(models.RequirementActionMute)(tr, mreqIn)
		wantMreqPlain := memberReqPlainNoticeDefault(models.RequirementActionMute)(tr, mreqIn)
		scheduledAuto := 0
		autoSite := eventSite{
			Bot:       bot,
			Chat:      chat,
			Tr:        tr,
			Kind:      deletion.KindBotAuto,
			Automatic: true,
			SendDefault: func(text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
				gotText, gotOpts = text, opts
				return &gotgbot.Message{MessageId: 202, Chat: *chat}, nil
			},
			Schedule: func(m *gotgbot.Message) {
				if m != nil && m.MessageId == 202 {
					scheduledAuto++
				}
			},
		}
		_, err = postEventSent(autoSite, eventReply{
			Event:        eventmsg.EventReqMute,
			Target:       target,
			Default:      mreqDef,
			PlainDefault: func() string { return memberReqPlainNoticeDefault(models.RequirementActionMute)(tr, mreqIn) },
			Opts:         mreqOpts,
			KeepRows:     fixRow,
			KeepRowsLast: true,
			Required:     true,
		})
		if err != nil {
			t.Fatalf("postEventSent(mreqmute): %v", err)
		}
		if gotText != wantMreqPlain {
			t.Fatalf("plain mreqmute text = %q, want %q", gotText, wantMreqPlain)
		}
		if scheduledAuto != 1 {
			t.Fatalf("auto Schedule calls = %d, want 1", scheduledAuto)
		}
		mreqKB, ok := gotOpts.ReplyMarkup.(gotgbot.InlineKeyboardMarkup)
		if !ok || len(mreqKB.InlineKeyboard) != 1 || mreqKB.InlineKeyboard[0][0].Text != "I fixed it" {
			t.Fatalf("mreqmute plain reply_markup = %#v, want 'I fixed it' row", gotOpts.ReplyMarkup)
		}
	})

	t.Run("custom_text_overrides_plain_tone", func(t *testing.T) {
		const chatID int64 = -100290303
		bot, client, chat, tr := newPlainRuntimeEnv(t, chatID)
		if err := eventmsg.SetPlainTone(context.Background(), chatID, true, admin.Id); err != nil {
			t.Fatalf("SetPlainTone: %v", err)
		}
		if err := eventmsg.SetSilent(context.Background(), chatID, eventmsg.EventBan, eventmsg.DestPM, false, admin.Id); err != nil {
			t.Fatalf("enable PM: %v", err)
		}
		if err := eventmsg.SetText(context.Background(), chatID, eventmsg.EventBan, eventmsg.DestGroup, "Custom group: {first}", admin.Id); err != nil {
			t.Fatalf("SetText group: %v", err)
		}
		if err := eventmsg.SetText(context.Background(), chatID, eventmsg.EventBan, eventmsg.DestPM, "Custom PM: {first}", admin.Id); err != nil {
			t.Fatalf("SetText PM: %v", err)
		}

		banIn := eventDefaultInput{Target: target, Reason: "spam"}
		defText, defOpts := banDefaultReply(tr, banIn)
		var gotGroup string
		site := eventSite{
			Bot:   bot,
			Chat:  chat,
			Tr:    tr,
			Actor: eventActor{User: &admin},
			Kind:  deletion.KindBotManual,
			SendDefault: func(text string, _ *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
				gotGroup = text
				return &gotgbot.Message{MessageId: 301, Chat: *chat}, nil
			},
			SendCustom: func(msg media.GreetingMessage) (*gotgbot.Message, error) {
				gotGroup = msg.Text
				return &gotgbot.Message{MessageId: 302, Chat: *chat}, nil
			},
			Schedule: func(*gotgbot.Message) {},
		}
		_, err := postEventSent(site, eventReply{
			Event:        eventmsg.EventBan,
			Target:       target,
			Values:       eventValues{Reason: "spam"},
			Default:      defText,
			PlainDefault: func() string { return banPlainDefaultReply(tr, banIn) },
			Opts:         defOpts,
		})
		if err != nil {
			t.Fatalf("postEventSent: %v", err)
		}
		waitEventPM(t)
		if gotGroup != "Custom group: Bob" {
			t.Fatalf("group custom override = %q, want %q", gotGroup, "Custom group: Bob")
		}
		pmCalls := client.sentTo(target.Id)
		if len(pmCalls) != 1 || fmt.Sprint(pmCalls[0].Params["text"]) != "Custom PM: Bob" {
			t.Fatalf("PM custom override calls = %+v, want single 'Custom PM: Bob'", pmCalls)
		}
	})

	t.Run("custom_media_without_text_uses_plain_default", func(t *testing.T) {
		const chatID int64 = -100290304
		bot, _, chat, tr := newPlainRuntimeEnv(t, chatID)
		if err := eventmsg.SetPlainTone(context.Background(), chatID, true, admin.Id); err != nil {
			t.Fatalf("SetPlainTone: %v", err)
		}
		if err := eventmsg.SetMedia(context.Background(), chatID, eventmsg.EventBan, eventmsg.DestGroup, "photo-file-id-1", db.PHOTO, admin.Id); err != nil {
			t.Fatalf("SetMedia: %v", err)
		}
		if err := eventmsg.SetButtons(context.Background(), chatID, eventmsg.EventBan, eventmsg.DestGroup, []models.Button{{Name: "Rules", Url: "https://example.com/rules"}}, admin.Id); err != nil {
			t.Fatalf("SetButtons: %v", err)
		}

		banIn := eventDefaultInput{Target: target, Reason: "spam"}
		defText, defOpts := banDefaultReply(tr, banIn)
		wantPlain := banPlainDefaultReply(tr, banIn)
		var gotCustom media.GreetingMessage
		site := eventSite{
			Bot:   bot,
			Chat:  chat,
			Tr:    tr,
			Actor: eventActor{User: &admin},
			Kind:  deletion.KindBotManual,
			SendDefault: func(string, *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
				t.Fatal("SendDefault called; expected SendCustom with plain text + custom media/buttons")
				return nil, nil
			},
			SendCustom: func(msg media.GreetingMessage) (*gotgbot.Message, error) {
				gotCustom = msg
				return &gotgbot.Message{MessageId: 401, Chat: *chat}, nil
			},
			Schedule: func(*gotgbot.Message) {},
		}
		_, err := postEventSent(site, eventReply{
			Event:        eventmsg.EventBan,
			Target:       target,
			Values:       eventValues{Reason: "spam"},
			Default:      defText,
			PlainDefault: func() string { return banPlainDefaultReply(tr, banIn) },
			Opts:         defOpts,
		})
		if err != nil {
			t.Fatalf("postEventSent: %v", err)
		}
		if gotCustom.Text != wantPlain {
			t.Fatalf("composed text = %q, want plain default %q", gotCustom.Text, wantPlain)
		}
		if gotCustom.FileID != "photo-file-id-1" || gotCustom.MsgType != db.PHOTO {
			t.Fatalf("composed media = (%q, %d), want photo-file-id-1", gotCustom.FileID, gotCustom.MsgType)
		}
		if gotCustom.Keyboard == nil || len(gotCustom.Keyboard.InlineKeyboard) < 2 {
			t.Fatalf("composed keyboard = %#v, want bot Unban row + custom Rules button", gotCustom.Keyboard)
		}
	})

	t.Run("refused_custom_message_falls_back_to_plain_default", func(t *testing.T) {
		const chatID int64 = -100290305
		bot, client, chat, tr := newPlainRuntimeEnv(t, chatID)
		if err := eventmsg.SetPlainTone(context.Background(), chatID, true, admin.Id); err != nil {
			t.Fatalf("SetPlainTone: %v", err)
		}
		if err := eventmsg.SetSilent(context.Background(), chatID, eventmsg.EventBan, eventmsg.DestPM, false, admin.Id); err != nil {
			t.Fatalf("enable PM: %v", err)
		}
		if err := eventmsg.SetText(context.Background(), chatID, eventmsg.EventBan, eventmsg.DestGroup, "Broken custom group", admin.Id); err != nil {
			t.Fatalf("SetText group: %v", err)
		}
		if err := eventmsg.SetText(context.Background(), chatID, eventmsg.EventBan, eventmsg.DestPM, "Broken custom PM", admin.Id); err != nil {
			t.Fatalf("SetText PM: %v", err)
		}

		// Queue a Telegram 400 Bad Request on the first PM send so PM falls back to plain default.
		client.mu.Lock()
		client.refusals[target.Id] = []error{&gotgbot.TelegramError{Code: 400, Description: "Bad Request: can't parse entities"}}
		client.mu.Unlock()

		banIn := eventDefaultInput{Target: target, Reason: "spam"}
		defText, defOpts := banDefaultReply(tr, banIn)
		wantGroupPlain := banPlainDefaultReply(tr, banIn)
		spec, _ := eventPMSpecFor(eventmsg.EventBan)
		wantPMPlain := eventPMDefaultText(tr, spec, chat, eventValues{Reason: "spam"}, "", true)

		var gotFallbackGroup string
		site := eventSite{
			Bot:   bot,
			Chat:  chat,
			Tr:    tr,
			Actor: eventActor{User: &admin},
			Kind:  deletion.KindBotManual,
			SendDefault: func(text string, _ *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
				gotFallbackGroup = text
				return &gotgbot.Message{MessageId: 501, Chat: *chat}, nil
			},
			SendCustom: func(media.GreetingMessage) (*gotgbot.Message, error) {
				return nil, &gotgbot.TelegramError{Code: 400, Description: "Bad Request: invalid media"}
			},
			Schedule: func(*gotgbot.Message) {},
		}
		_, err := postEventSent(site, eventReply{
			Event:        eventmsg.EventBan,
			Target:       target,
			Values:       eventValues{Reason: "spam"},
			Default:      defText,
			PlainDefault: func() string { return banPlainDefaultReply(tr, banIn) },
			Opts:         defOpts,
		})
		if err != nil {
			t.Fatalf("postEventSent: %v", err)
		}
		waitEventPM(t)
		if gotFallbackGroup != wantGroupPlain {
			t.Fatalf("group fallback = %q, want plain default %q", gotFallbackGroup, wantGroupPlain)
		}
		pmCalls := client.sentTo(target.Id)
		if len(pmCalls) != 2 {
			t.Fatalf("PM send attempts = %d, want 2 (refused custom + plain fallback)", len(pmCalls))
		}
		if got := fmt.Sprint(pmCalls[1].Params["text"]); got != wantPMPlain {
			t.Fatalf("PM fallback text = %q, want plain default %q", got, wantPMPlain)
		}
	})

	t.Run("fwdblock_and_aispam_plain_privacy", func(t *testing.T) {
		withRealLocales(t)
		for _, lang := range plainAllLocales {
			tr := i18n.MustNewTranslator(lang)
			fwdReason, _ := tr.GetString("staff_fwdblock_reason")
			fwdGroup := blacklistPlainDefaultReply(tr, eventDefaultInput{
				Target: target,
				Action: "mute",
				Reason: fwdReason,
			})
			fwdSpec, _ := eventPMSpecFor(eventmsg.EventForwardBlock)
			fwdPM := eventPMDefaultText(tr, fwdSpec, &gotgbot.Chat{Id: -1001, Title: "Main Group"}, eventValues{
				Action: eventActionWord(tr, "mute"),
				Reason: fwdReason,
			}, "", true)
			for _, text := range []string{fwdGroup, fwdPM} {
				lower := strings.ToLower(text)
				for _, secret := range []string{"staff", "channel_id", "-100999", "secret_source"} {
					if strings.Contains(lower, secret) {
						t.Errorf("fwdblock[%s] leaked %q in %q", lang, secret, text)
					}
				}
			}

			aiGroup := aispamPlainDefaultReply(tr, eventDefaultInput{Target: target})
			aiSpec, _ := eventPMSpecFor(eventmsg.EventAISpam)
			aiPM := eventPMDefaultText(tr, aiSpec, &gotgbot.Chat{Id: -1001, Title: "Main Group"}, eventValues{
				Action: eventActionWord(tr, "delete"),
				Reason: aispamEventReason(tr),
			}, "", true)
			for _, text := range []string{aiGroup, aiPM} {
				lower := strings.ToLower(text)
				for _, secret := range []string{"openai", "gemini", "claude", "gpt", "model", "score", "0.9", "probability"} {
					if strings.Contains(lower, secret) {
						t.Errorf("aispam[%s] leaked %q in %q", lang, secret, text)
					}
				}
			}
		}
	})
}
