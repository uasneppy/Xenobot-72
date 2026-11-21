//go:build testtools

package modules

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
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
	"github.com/uasneppy/fuku_robot/fuku/utils/formatting"
)

// eventMsgCaptchaText is the custom captcha text the captcha tests store: a Welcome placeholder in
// markup and the event placeholders a captcha failure fills.
const eventMsgCaptchaText = "<i>{mention}</i> {action} ({count}): {reason}"

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
	tr := i18n.MustNewTranslator("uk")
	kicked := eventActionWord(tr, "kick")
	reason := captchaEventReason(tr)

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
		if want := "<i>@ann_lee</i> " + kicked + " (2): " + reason; params["text"] != want {
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

// eventMsgHookDefault is the default text of the engine hook tests' replies.
const eventMsgHookDefault = "hook default"

// eventMsgHookSite is a test-only site in the botSite's chat: no reply, no topic, the automatic kind,
// the default sent with SendMessage and the given admission gate (nil for none).
func eventMsgHookSite(s *botSite, admit func(context.Context) bool) eventSite {
	return eventSite{
		Bot:       s.bot,
		Chat:      &s.chat,
		Tr:        i18n.MustNewTranslator("en"),
		Kind:      deletion.KindBotAuto,
		Automatic: true,
		SendDefault: func(text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
			return s.bot.SendMessage(s.chat.Id, text, opts)
		},
		Admit: admit,
	}
}

// eventMsgHookReply is a reply of event to member 42 with the hook default.
func eventMsgHookReply(event string) eventReply {
	return eventReply{
		Event:   event,
		Target:  gotgbot.User{Id: 42, FirstName: "Ann"},
		Default: eventMsgHookDefault,
		Opts:    &gotgbot.SendMessageOpts{},
	}
}

// eventMsgHookTexts are the texts of the site's sendMessage calls, in order.
func eventMsgHookTexts(t *testing.T, s *botSite) []string {
	t.Helper()
	var texts []string
	for _, call := range s.client.callsFor("sendMessage") {
		texts = append(texts, rulesCallParams(t, call)["text"].(string))
	}
	return texts
}

func TestEventMessageEngineHooks(t *testing.T) {
	newSite := func(t *testing.T) *botSite {
		t.Helper()
		s := newBotSite(t, "supergroup")
		eventMsgCleanup(t, s.chat.Id)
		return s
	}
	post := func(t *testing.T, site eventSite, r eventReply) {
		t.Helper()
		if err := postEvent(site, r); err != nil {
			t.Fatalf("postEvent(%s) error = %v", r.Event, err)
		}
		pmCopyJobs.wait()
	}

	t.Run("required posts a silent row", func(t *testing.T) {
		s := newSite(t)
		eventMsgStore(t, s.chat.Id, eventmsg.EventBan, "required custom")
		if err := eventmsg.SetSilent(t.Context(), s.chat.Id, eventmsg.EventBan, eventmsg.DestGroup, true, 1); err != nil {
			t.Fatal(err)
		}
		r := eventMsgHookReply(eventmsg.EventBan)
		post(t, eventMsgHookSite(s, nil), r)
		if got := eventMsgHookTexts(t, s); len(got) != 0 {
			t.Fatalf("a silent row without Required sent %v", got)
		}
		r.Required = true
		post(t, eventMsgHookSite(s, nil), r)
		if got := eventMsgHookTexts(t, s); len(got) != 1 || got[0] != "required custom" {
			t.Fatalf("sends = %v, want the stored text despite Silent", got)
		}
	})

	t.Run("text only drops the stored file", func(t *testing.T) {
		s := newSite(t)
		eventMsgStoreRow(t, s.chat.Id, eventmsg.EventBan, "with a photo", "PHOTO-FILE", db.PHOTO, nil)
		r := eventMsgHookReply(eventmsg.EventBan)
		r.TextOnly = true
		post(t, eventMsgHookSite(s, nil), r)
		if photos := s.client.callsFor("sendPhoto"); len(photos) != 0 {
			t.Fatalf("sendPhoto calls = %d, want none for a text-only reply", len(photos))
		}
		if got := eventMsgHookTexts(t, s); len(got) != 1 || got[0] != "with a photo" {
			t.Fatalf("sends = %v, want the stored text alone", got)
		}
	})

	t.Run("admit", func(t *testing.T) {
		s := newSite(t)
		asked := 0
		refuse := func(context.Context) bool { asked++; return false }
		eventMsgStore(t, s.chat.Id, eventmsg.EventBan, "admitted custom")
		if err := eventmsg.SetText(t.Context(), s.chat.Id, eventmsg.EventBan, eventmsg.DestPM, "private", 1); err != nil {
			t.Fatal(err)
		}
		if err := eventmsg.SetSilent(t.Context(), s.chat.Id, eventmsg.EventBan, eventmsg.DestPM, false, 1); err != nil {
			t.Fatal(err)
		}
		post(t, eventMsgHookSite(s, refuse), eventMsgHookReply(eventmsg.EventBan))
		if asked != 1 {
			t.Fatalf("Admit asked %d times, want once", asked)
		}
		if calls := len(eventMsgMethods(s.client)); calls != 0 {
			t.Fatalf("a refused admission made %d calls (%v), want none: no group message, no private copy",
				calls, eventMsgMethods(s.client))
		}

		// With the group message Silent and the private copy off, nothing would be sent: Admit is not asked.
		if err := eventmsg.SetSilent(t.Context(), s.chat.Id, eventmsg.EventBan, eventmsg.DestGroup, true, 1); err != nil {
			t.Fatal(err)
		}
		if err := eventmsg.SetSilent(t.Context(), s.chat.Id, eventmsg.EventBan, eventmsg.DestPM, true, 1); err != nil {
			t.Fatal(err)
		}
		post(t, eventMsgHookSite(s, refuse), eventMsgHookReply(eventmsg.EventBan))
		if asked != 1 {
			t.Fatalf("Admit asked %d times, want no second ask when nothing would be sent", asked)
		}

		admitted := 0
		eventMsgStore(t, s.chat.Id, eventmsg.EventKick, "kick custom")
		post(t, eventMsgHookSite(s, func(context.Context) bool { admitted++; return true }), eventMsgHookReply(eventmsg.EventKick))
		if got := eventMsgHookTexts(t, s); admitted != 1 || len(got) != 1 || got[0] != "kick custom" {
			t.Fatalf("admitted %d times, sends = %v, want one ask and the custom message", admitted, got)
		}
	})

	t.Run("a read error", func(t *testing.T) {
		s := newSite(t)
		mreqSwap(t, &eventMessagesLookup, func(context.Context, int64) (*eventmsg.Messages, error) {
			return nil, errors.New("read failed")
		})
		post(t, eventMsgHookSite(s, nil), eventMsgHookReply(eventmsg.EventAISpam))
		if got := eventMsgHookTexts(t, s); len(got) != 0 {
			t.Fatalf("an unreadable off-by-default event sent %v, want nothing", got)
		}
		post(t, eventMsgHookSite(s, nil), eventMsgHookReply(eventmsg.EventBan))
		if got := eventMsgHookTexts(t, s); len(got) != 1 || got[0] != eventMsgHookDefault {
			t.Fatalf("an unreadable ban sent %v, want the default", got)
		}
	})

	t.Run("no row", func(t *testing.T) {
		s := newSite(t)
		post(t, eventMsgHookSite(s, nil), eventMsgHookReply(eventmsg.EventAISpam))
		if got := eventMsgHookTexts(t, s); len(got) != 0 {
			t.Fatalf("the AI spam notice with no row sent %v, want nothing (off by default)", got)
		}
		post(t, eventMsgHookSite(s, nil), eventMsgHookReply(eventmsg.EventBan))
		if got := eventMsgHookTexts(t, s); len(got) != 1 || got[0] != eventMsgHookDefault {
			t.Fatalf("a ban with no row sent %v, want the default", got)
		}
	})
}

// The requirement goldens' member and hold, as the default builder tests fill the <user> and <hold> tokens.
const (
	screenDefaultUser = 7771234
	screenDefaultHold = 98765
)

// screenDefaultWant is a golden with its tokens filled as autoDefaultRequest sends it: chat 0, the test's
// member and hold.
func screenDefaultWant(golden string) string {
	return strings.NewReplacer(
		"<chat>", "0",
		"<user>", strconv.Itoa(screenDefaultUser),
		"<hold>", strconv.Itoa(screenDefaultHold),
	).Replace(golden)
}

// screenNoticeInput is the default input of a requirement notice kind (mreqNoticeKinds' shapes) for the
// member, the broken rules and the mute's keyboard rows.
func screenNoticeInput(kind mreqNoticeKind, target gotgbot.User, rules int, rows [][]gotgbot.InlineKeyboardButton) eventDefaultInput {
	in := eventDefaultInput{Target: target, Rules: rules}
	if kind.warn != nil {
		in.Count, in.Limit, in.Action = kind.warn.Count, kind.warn.Limit, kind.warn.Mode
	}
	if kind.action == models.RequirementActionMute {
		in.Rows = rows
	}
	return in
}

func TestScreeningEventDefaults(t *testing.T) {
	withRealLocales(t)
	tr := i18n.MustNewTranslator("uk")
	member := gotgbot.User{Id: screenDefaultUser, FirstName: "Ann & <Bo>", Username: "ann_lee"}
	lastName := mreqRuleMask(t, "ln")
	fixed := memberReqFixedButton(tr, screenDefaultHold, screenDefaultUser).InlineKeyboard
	warn := func(count, limit int, action string) eventDefaultInput {
		return eventDefaultInput{Target: member, Rules: lastName, Count: count, Limit: limit, Action: action}
	}

	cases := []struct {
		name   string
		build  eventDefaultReply
		in     eventDefaultInput
		golden string
	}{
		{"captcha_kick", captchaFailureDefaultReply, eventDefaultInput{Target: noticeGoldenTarget, Action: "kick"}, goldenScreenCaptchaKick},
		{"captcha_ban", captchaFailureDefaultReply, eventDefaultInput{Target: noticeGoldenTarget, Action: "ban"}, goldenScreenCaptchaBan},
		{"captcha_mute", captchaFailureDefaultReply, eventDefaultInput{Target: noticeGoldenTarget, Action: "mute"}, goldenScreenCaptchaMute},
		{"captcha_kick_held", captchaFailureDefaultReply, eventDefaultInput{Target: noticeGoldenTarget, Action: "kick", Count: 2}, goldenScreenCaptchaKickHeld},
		{"req_warn", memberReqNoticeDefault(models.RequirementActionWarn), warn(1, 3, ""), goldenScreenReqWarn},
		{"req_warnlimit_kick", memberReqNoticeDefault(models.RequirementActionWarn), warn(1, 1, "kick"), goldenScreenReqWarnLimitKick},
		{"req_warnlimit_ban", memberReqNoticeDefault(models.RequirementActionWarn), warn(1, 1, "ban"), goldenScreenReqWarnLimitBan},
		{"req_warnlimit_mute", memberReqNoticeDefault(models.RequirementActionWarn), warn(1, 1, "mute"), goldenScreenReqWarnLimitMute},
		{"req_kick", memberReqNoticeDefault(models.RequirementActionKick), eventDefaultInput{Target: member, Rules: lastName}, goldenScreenReqKick},
		{"req_ban", memberReqNoticeDefault(models.RequirementActionBan), eventDefaultInput{Target: member, Rules: lastName}, goldenScreenReqBan},
		{"req_mute", memberReqNoticeDefault(models.RequirementActionMute), eventDefaultInput{Target: member, Rules: lastName, Rows: fixed}, goldenScreenReqMute},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text, opts := tc.build(tr, tc.in)
			if got, want := autoDefaultRequest(t, text, opts), screenDefaultWant(tc.golden); got != want {
				t.Fatalf("default request\n got %q\nwant %q", got, want)
			}
		})
	}

	t.Run("aispam_notice", func(t *testing.T) {
		target := gotgbot.User{Id: 42, FirstName: "Ann & <Bo>", LastName: "Lee"}
		text, opts := aispamNoticeDefaultReply(tr, eventDefaultInput{Target: target})
		mention := formatting.MentionHtml(42, "Ann & <Bo> Lee")
		want, _ := tr.GetString("aispam_notice_deleted", i18n.TranslationParams{"name": mention})
		if text != want || !strings.Contains(text, "Ann &amp; &lt;Bo&gt; Lee") || strings.Contains(text, "{name}") {
			t.Fatalf("text = %q, want %q with the escaped mention", text, want)
		}
		raw := autoDefaultRequest(t, text, opts)
		if !strings.Contains(raw, `"parse_mode":"HTML"`) || !strings.Contains(raw, `"link_preview_options":{"is_disabled":true}`) ||
			strings.Contains(raw, "reply_markup") || strings.Contains(raw, "reply_parameters") {
			t.Fatalf("request = %s, want HTML with link previews off, no keyboard and no reply", raw)
		}
	})

	t.Run("requirement text equals memberReqNoticeText", func(t *testing.T) {
		var masks []int
		for i, a := range memberReqRules {
			masks = append(masks, a.Bit)
			for _, b := range memberReqRules[i+1:] {
				masks = append(masks, a.Bit|b.Bit)
			}
		}
		masks = append(masks, mreqAllRulesMask())
		for _, kind := range mreqNoticeKinds() {
			for _, mask := range masks {
				got, _ := memberReqNoticeDefault(kind.action)(tr, screenNoticeInput(kind, member, mask, fixed))
				if want := memberReqNoticeText(tr, member, mask, kind.action, kind.warn); got != want {
					t.Fatalf("%s notice of mask %d\n got %q\nwant %q", kind.name, mask, got, want)
				}
			}
		}
	})
}

func TestMemberReqNoticeNoteFitsReserve(t *testing.T) {
	withRealLocales(t)
	all := mreqAllRulesMask()
	for _, lang := range []string{"uk", "en", "es", "fr", "hi", "id", "pt", "ru"} {
		tr := i18n.MustNewTranslator(lang)
		note := memberReqNoticeNote(tr, all, "mreq_notice_mute")
		line, _ := tr.GetString("eventmsg_mreq_rules", i18n.TranslationParams{"rules": memberReqRulesText(tr, all)})
		want := "\n" + line
		for _, remedy := range memberReqNoticeRemedies("mreq_notice_mute", all) {
			sentence, _ := tr.GetString(remedy)
			want += " " + sentence
		}
		if len(memberReqNoticeRemedies("mreq_notice_mute", all)) != 2 {
			t.Fatalf("remedies of every rule = %v, want the photo and the AI remedy", memberReqNoticeRemedies("mreq_notice_mute", all))
		}
		if note != want || !strings.HasPrefix(note, "\n") || strings.Contains(note, "{rules}") {
			t.Errorf("%s: note = %q, want %q", lang, note, want)
		}
		for _, rule := range memberReqRules {
			phrase, _ := tr.GetString(rule.PhraseKey)
			if !strings.Contains(note, phrase) {
				t.Errorf("%s: the note misses the phrase of %s", lang, rule.PhraseKey)
			}
		}
		n := formatting.VisibleLength(note)
		if n > memberReqNoticeNoteReserve {
			t.Errorf("%s: the worst note is %d UTF-16 units, over the reserve %d", lang, n, memberReqNoticeNoteReserve)
		}
		t.Logf("%s: the worst note is %d UTF-16 units", lang, n)
		if ban := memberReqNoticeNote(tr, all, "mreq_notice_ban"); ban != "\n"+line {
			t.Errorf("%s: the ban note = %q, want the requirements line alone", lang, ban)
		}
	}
}
