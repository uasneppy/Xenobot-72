//go:build testtools

package modules

import (
	"net/http"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/captcha"
	"github.com/uasneppy/fuku_robot/fuku/db/deletion"
	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
	"github.com/uasneppy/fuku_robot/fuku/utils/captchaweb"
)

// eventMsgCaptchaText is the custom captcha text the captcha tests store: a Welcome placeholder in
// markup and the two event placeholders a captcha failure fills.
const eventMsgCaptchaText = "<i>{mention}</i> {action} ({count})"

// eventMsgBotKinds are the five bot message kinds; the captcha tests give each its shortest delay, so a
// captcha notice queued under any of them would show.
var eventMsgBotKinds = []string{
	deletion.KindBotWelcome, deletion.KindBotGoodbye, deletion.KindBotRules, deletion.KindBotAuto, deletion.KindBotManual,
}

// eventMsgCaptchaSite is the screening golden site with captcha on (failure action kick), every bot kind
// at 60 s and the chat's captcha settings and event messages removed when the test ends.
func eventMsgCaptchaSite(t *testing.T) *botSite {
	t.Helper()
	s := newScreenGoldenSite(t)
	chatID := s.chat.Id
	eventMsgCleanup(t, chatID)
	t.Cleanup(func() { db.DB.Where("chat_id = ?", chatID).Delete(&models.CaptchaSettings{}) })
	if err := captcha.SetCaptchaEnabled(chatID, true); err != nil {
		t.Fatalf("SetCaptchaEnabled() error = %v", err)
	}
	if err := captcha.SetCaptchaFailureAction(chatID, "kick"); err != nil {
		t.Fatalf("SetCaptchaFailureAction() error = %v", err)
	}
	for _, kind := range eventMsgBotKinds {
		s.setRule(chatID, kind, 60)
	}
	return s
}

// eventMsgKickedFirst fails unless the member was kicked (one member-removing unban) before the first
// sendMessage, and returns the sendMessage calls.
func eventMsgKickedFirst(t *testing.T, s *botSite) []moduleBotCall {
	t.Helper()
	if got := kicksOf(s.client, noticeGoldenTarget.Id); got != 1 {
		t.Fatalf("kicks = %d, want one", got)
	}
	methods := eventMsgMethods(s.client)
	if send := eventMsgIndex(methods, "sendMessage"); send >= 0 && send < eventMsgIndex(methods, "unbanChatMember") {
		t.Fatalf("calls = %v, want the kick before any notice", methods)
	}
	return s.client.callsFor("sendMessage")
}

func TestEventMessageCaptcha(t *testing.T) {
	withModerationGoldenLocales(t)
	kicked := eventActionWord(i18n.MustNewTranslator("uk"), "kick")

	t.Run("timer custom", func(t *testing.T) {
		mreqSwap(t, &captchaFailureNoticeLife, 50*time.Millisecond)
		s := eventMsgCaptchaSite(t)
		eventMsgStore(t, s.chat.Id, eventmsg.EventCaptcha, eventMsgCaptchaText)
		attempt, err := captcha.CreateCaptchaAttemptPreMessage(noticeGoldenTarget.Id, s.chat.Id, "7", 2)
		if err != nil {
			t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
		}
		for _, text := range []string{"first held", "second held"} {
			if err := captcha.StoreMessageForCaptcha(noticeGoldenTarget.Id, s.chat.Id, attempt.ID, db.TEXT, text, "", ""); err != nil {
				t.Fatalf("StoreMessageForCaptcha() error = %v", err)
			}
		}

		if claimed, err := expireCaptchaAttempt(s.bot, attempt); !claimed || err != nil {
			t.Fatalf("expireCaptchaAttempt() = (%v, %v), want claimed", claimed, err)
		}

		sends := eventMsgKickedFirst(t, s)
		if len(sends) != 1 {
			t.Fatalf("sendMessage calls = %d, want the one custom notice", len(sends))
		}
		params := rulesCallParams(t, sends[0])
		if want := "<i>@ann_lee</i> " + kicked + " (2)"; params["text"] != want {
			t.Fatalf("text = %q, want %q", params["text"], want)
		}
		if _, ok := params["reply_parameters"]; ok {
			t.Fatalf("the notice is a reply: %v", params["reply_parameters"])
		}
		if params["reply_markup"] != nil {
			t.Fatalf("the notice has a keyboard: %v", params["reply_markup"])
		}
		s.wantNoRows(s.chat.Id)

		deadline := time.Now().Add(2 * time.Second)
		for len(callsWithMessageID(s.client, "deleteMessage", 9001)) == 0 && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		time.Sleep(100 * time.Millisecond)
		if got := len(callsWithMessageID(s.client, "deleteMessage", 9001)); got != 1 {
			t.Fatalf("deletes of the notice = %d, want one after its life", got)
		}
		s.wantNoRows(s.chat.Id)
	})

	t.Run("web fail custom", func(t *testing.T) {
		member := gotgbot.User{Id: 42, FirstName: "Member"}
		env := newCaptchaWebEnv(t)
		if err := captcha.SetCaptchaFailureAction(env.chat.Id, "kick"); err != nil {
			t.Fatalf("SetCaptchaFailureAction() error = %v", err)
		}
		eventMsgStore(t, env.chat.Id, eventmsg.EventCaptcha, "custom captcha <i>{first}</i>")
		token := env.join(member)
		env.setSiteverify(captchaweb.SiteverifyAnswerForTest(http.StatusOK, `{"success":false,"error-codes":["invalid-input-response"]}`))

		status, answer := env.verify(member, token)
		if status != http.StatusOK || answer.Result != captchaweb.ResultFailed || answer.Action != "kick" {
			t.Fatalf("verify = %d %+v, want 200 failed with kick (the action applied)", status, answer)
		}
		if got := kicksOf(env.client, member.Id); got != 1 {
			t.Fatalf("kicks = %d, want one", got)
		}
		if got := sentTextContaining(env.client, "custom captcha <i>"); got != 1 {
			t.Fatalf("custom notices = %d, want one", got)
		}
		if got := sentTextContaining(env.client, staffMarker("captcha_timeout_failure_kicked")); got != 0 {
			t.Fatalf("default notices = %d, want none beside the custom one", got)
		}
	})

	t.Run("nothing stored", func(t *testing.T) {
		s := eventMsgCaptchaSite(t)
		screenGoldenCaptcha(t, s, "kick", false)
		sends := eventMsgKickedFirst(t, s)
		if len(sends) != 1 {
			t.Fatalf("sendMessage calls = %d, want the default notice", len(sends))
		}
		if got := screenGoldenTokens(legacyRequestJSON(t, sends[0]), s.chat.Id, 0, 0); got != goldenScreenCaptchaKick {
			t.Fatalf("default notice\n got %q\nwant %q", got, goldenScreenCaptchaKick)
		}
		s.wantNoRows(s.chat.Id)
	})

	t.Run("refused", func(t *testing.T) {
		s := eventMsgCaptchaSite(t)
		eventMsgStore(t, s.chat.Id, eventmsg.EventCaptcha, eventMsgCaptchaText)
		eventMsgRefuseOnce(s, "sendMessage", &gotgbot.TelegramError{Method: "sendMessage", Code: 400, Description: "Bad Request: can't parse entities"})
		screenGoldenCaptcha(t, s, "kick", false)
		sends := eventMsgKickedFirst(t, s)
		if len(sends) != 2 {
			t.Fatalf("sendMessage calls = %d, want the refused custom notice and the default", len(sends))
		}
		if got := screenGoldenTokens(legacyRequestJSON(t, sends[1]), s.chat.Id, 0, 0); got != goldenScreenCaptchaKick {
			t.Fatalf("fallback notice\n got %q\nwant %q", got, goldenScreenCaptchaKick)
		}
		s.wantNoRows(s.chat.Id)
	})

	t.Run("silent", func(t *testing.T) {
		s := eventMsgCaptchaSite(t)
		eventMsgStore(t, s.chat.Id, eventmsg.EventCaptcha, eventMsgCaptchaText)
		if err := eventmsg.SetSilent(t.Context(), s.chat.Id, eventmsg.EventCaptcha, eventmsg.DestGroup, true, 1); err != nil {
			t.Fatalf("SetSilent error = %v", err)
		}
		screenGoldenCaptcha(t, s, "kick", false)
		if sends := eventMsgKickedFirst(t, s); len(sends) != 0 {
			t.Fatalf("sendMessage calls = %d, want none when silent", len(sends))
		}
		s.wantNoRows(s.chat.Id)
	})

	t.Run("the captcha key", func(t *testing.T) {
		if !eventmsg.ValidEvent("captcha") || eventmsg.EventCaptcha != "captcha" {
			t.Fatalf("ValidEvent(captcha) = %v, EventCaptcha = %q", eventmsg.ValidEvent("captcha"), eventmsg.EventCaptcha)
		}
		for _, word := range []string{"Captcha", "captchas", "warning", "dban"} {
			if eventmsg.ValidEvent(word) {
				t.Errorf("ValidEvent(%q) = true", word)
			}
		}
	})
}
