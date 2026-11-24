//go:build testtools

package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/aispam"
	dbcache "github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/captcha"
	"github.com/uasneppy/fuku_robot/fuku/db/deletion"
	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/db/memberreq"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
	"github.com/uasneppy/fuku_robot/fuku/utils/cache"
	"github.com/uasneppy/fuku_robot/fuku/utils/captchaweb"
	"github.com/uasneppy/fuku_robot/fuku/utils/formatting"
)

// The screening messages end to end (plan 24-08; EVT-04, EVT-05, EVT-09, the four success criteria of Phase 24).
// Everything is real but Telegram, Cloudflare and the AI provider: the owner works the Automatic and Member
// requirements families of the private Messages page through the deep link and the buttons the bot drew, every part
// is a typed private message, a joiner meets the captcha through a real chat_member update and the greetings module,
// a member's messages in the group break a requirement or are judged spam through the real dispatcher with the
// Captcha, MemberRequirements and AISpam modules, the held member taps "I fixed it" and a Turnstile check is posted
// to the verify route. Only the Bot API is the hand-written fake (wrapped by Phase 21's messagesE2EClient), and
// Cloudflare's siteverify and TypeSafe are httptest stand-ins. The sites' logs and privacy are swept by
// TestEventMessageScreeningLogsCarryIDsOnly and TestEventMessageScreeningPrivacy (plan 24-07) and not again here.

// screeningMessagesE2E is Phase 23's autoMessagesE2E (the settings environment, a live creator who works the private
// menu, a 600 s manual and a 900 s automatic Bot messages timer, every private copy job awaited) with the Greetings
// and MemberRequirements modules on its dispatcher (the settings environment already loads the Captcha module), the
// requirement pacer made fast, the enforcement switch on and every requirement job awaited, and the chat's captcha,
// requirement and AI spam rows removed when the test ends. The failed captcha's notice lives 20 ms, so its in-process
// delete runs inside the test.
type screeningMessagesE2E struct {
	*autoMessagesE2E
	mreq *memberReqEnv
}

// newScreeningMessagesE2E builds one environment for one subtest; real installs the real locale files, for a subtest
// that compares a request with the goldens recorded in Ukrainian.
func newScreeningMessagesE2E(t *testing.T, real bool) *screeningMessagesE2E {
	t.Helper()
	// The failed captcha's notice deletes itself after this life, inside the test.
	mreqSwap(t, &captchaFailureNoticeLife, 20*time.Millisecond)
	e := newAutoMessagesE2E(t, real)
	withFastMemberReqPacer(t)
	withMemberReqLive(t, true)
	memberReqCleanup(t, []int64{e.chat.Id}, nil)
	chatID := e.chat.Id
	t.Cleanup(func() {
		db.DB.Where("chat_id = ?", chatID).Delete(&models.CaptchaSettings{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.StoredMessages{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.AISpamSettings{})
		dbcache.DeleteCache(dbcache.CacheKey("captcha_settings", chatID))
		dbcache.DeleteCache(dbcache.CacheKey("ai_spam_settings", chatID))
		dbcache.ResetLocalForTest()
	})
	// Registered after the cleanups above, so it runs before them: no requirement job is still writing.
	t.Cleanup(memberReqJobs.wait)
	LoadGreetings(e.dispatcher)
	LoadMemberRequirements(e.dispatcher)
	return &screeningMessagesE2E{autoMessagesE2E: e, mreq: &memberReqEnv{settingsEnv: e.settingsEnv}}
}

// screeningE2EClock is the deletion clock of one subtest: deletionNow and the date of every message the fake sends
// read one settable second.
type screeningE2EClock struct {
	at time.Time
	mu sync.Mutex
}

func (c *screeningE2EClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *screeningE2EClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = c.at.Add(d)
}

// clock puts the deletion clock and the fake's message dates under the test's control.
func (e *screeningMessagesE2E) clock() *screeningE2EClock {
	e.t.Helper()
	c := &screeningE2EClock{at: time.Now().UTC().Truncate(time.Second)}
	mreqSwap(e.t, &deletionNow, c.now)
	e.fake.sendDate = func() int64 { return c.now().Unix() }
	return c
}

// joiner makes a person who is not yet in the group: a fresh ID, the names getChatMember reports and a private chat
// getChat answers. lastName and username may be empty.
func (e *screeningMessagesE2E) joiner(first, lastName, username string) gotgbot.User {
	e.t.Helper()
	user := gotgbot.User{Id: uniqueLinkOwnerID(), FirstName: first, LastName: lastName, Username: username}
	e.client.register(user)
	e.fake.setProfile(user.Id, first, lastName, username)
	e.fake.setMember(e.chat.Id, user.Id, staffFakeMember{Status: gotgbot.ChatMemberStatusMember, IsMember: true, CanSendMessages: true})
	memberReqCleanup(e.t, nil, []int64{user.Id})
	return user
}

// openScreening walks Main, Messages, the family's cell and the event's cell of user's private menu, each a press of
// the drawn button.
func (e *screeningMessagesE2E) openScreening(user gotgbot.User, family, code string) *messagesE2EMenu {
	e.t.Helper()
	m := e.openHub(user)
	m.tap("a", settingsActMsgFamily, "t", family)
	m.tap("a", settingsActMsgEvent, settingsMsgEventField, code)
	return m
}

// setScreeningPart presses Set of part on the event's screen as the owner and sends msg as the typed reply.
func (e *screeningMessagesE2E) setScreeningPart(family, code, part string, msg gotgbot.Message) {
	e.t.Helper()
	e.openScreening(e.owner, family, code).tap("a", settingsActMsgSet, "t", part)
	if !e.redis.Exists(settingsInputKey(e.owner.Id)) {
		e.t.Fatalf("Set of %s/%s left no wait", code, part)
	}
	sendPrivateMessage(e.settingsEnv, e.owner, msg)
	if e.redis.Exists(settingsInputKey(e.owner.Id)) {
		e.t.Fatalf("the typed %s of %s did not finish the wait", part, code)
	}
}

// setScreeningText sets the event's text from a typed message whose first word is bold.
func (e *screeningMessagesE2E) setScreeningText(family, code, text string) {
	e.t.Helper()
	first := strings.IndexByte(text, ' ')
	if first < 0 {
		first = len(text)
	}
	e.setScreeningPart(family, code, settingsEditPartText, gotgbot.Message{
		Text: text, Entities: []gotgbot.MessageEntity{{Type: "bold", Offset: 0, Length: int64(first)}},
	})
}

// captchaOn turns the captcha on with the failure action kick, through the repository.
func (e *screeningMessagesE2E) captchaOn() {
	e.t.Helper()
	if err := captcha.SetCaptchaEnabled(e.chat.Id, true); err != nil {
		e.t.Fatalf("SetCaptchaEnabled error = %v", err)
	}
	if err := captcha.SetCaptchaFailureAction(e.chat.Id, "kick"); err != nil {
		e.t.Fatalf("SetCaptchaFailureAction error = %v", err)
	}
}

// joinCaptcha runs member's join as a chat_member update through the real dispatcher, which meets the captcha, and
// returns the attempt it opened.
func (e *screeningMessagesE2E) joinCaptcha(member gotgbot.User) *db.CaptchaAttempts {
	e.t.Helper()
	e.join(member, member, "")
	attempt, err := captcha.GetCaptchaAttempt(member.Id, e.chat.Id)
	if err != nil || attempt == nil {
		e.t.Fatalf("GetCaptchaAttempt(%d) = %v, %v, want the attempt the join opened", member.Id, attempt, err)
	}
	return attempt
}

// expire moves the attempt's expiry into the past and runs the timer's own path, which must claim it.
func (e *screeningMessagesE2E) expire(attempt *db.CaptchaAttempts) {
	e.t.Helper()
	past := time.Now().Add(-time.Second)
	if err := db.DB.Model(&db.CaptchaAttempts{}).Where("id = ?", attempt.ID).
		Updates(map[string]any{"created_at": past.Add(-time.Minute), "expires_at": past}).Error; err != nil {
		e.t.Fatalf("move the expiry: %v", err)
	}
	attempt.ExpiresAt = past
	if claimed, err := expireCaptchaAttempt(e.bot, attempt); !claimed || err != nil {
		e.t.Fatalf("expireCaptchaAttempt(%d) = (%v, %v), want claimed", attempt.UserID, claimed, err)
	}
}

// joinAndFail is a real join that meets the captcha, then the attempt expired through the timer's own path.
func (e *screeningMessagesE2E) joinAndFail(member gotgbot.User) {
	e.t.Helper()
	e.expire(e.joinCaptcha(member))
}

// kicks is the member-removing unbans of member in the group (the captcha's and the requirements' kick).
func (e *screeningMessagesE2E) kicks(member gotgbot.User) int {
	n := 0
	for _, call := range callsToChat(e.fake.staffBotClient, "unbanChatMember", e.chat.Id) {
		if staffParamInt(call.Params, "user_id") == member.Id && !staffParamBool(call.Params, "only_if_banned") {
			n++
		}
	}
	return n
}

// breakRule stores the last-name rule's punishment through the repository and runs a plain message of member (who
// has no last name) through the real dispatcher, waiting for the enforcement job.
func (e *screeningMessagesE2E) breakRule(member gotgbot.User, action int) {
	e.t.Helper()
	e.mreq.mreqRules(map[memberreq.Rule]int{memberreq.RuleLastName: action})
	e.mreq.mreqSend(member, "hello")
	memberReqJobs.wait()
}

// aispamOn turns /aispam on for the group with the TypeSafe stand-in judging every message spam, the notice pacer
// fast and the AI burst trigger off (several spammers would otherwise start a lockdown whose notice would count).
func (e *screeningMessagesE2E) aispamOn() {
	e.t.Helper()
	t := e.t
	resetAISpamRuntime()
	t.Cleanup(resetAISpamRuntime)
	withAISpamTestConfig(t, "typesafe-test-key", true, 0)
	withAISpamJevServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, aispamDecisionBody(0.95, "scam"))
	})
	withFastAISpamNoticePacer(t)
	mreqSwap(t, &raidSettingsLookup, func(context.Context, int64) (*models.ChatRaidDetection, error) {
		return &models.ChatRaidDetection{AIEnabled: false, AIThreshold: 5}, nil
	})
	if err := aispam.SetAISpamEnabled(e.chat.Id, true); err != nil {
		t.Fatalf("SetAISpamEnabled error = %v", err)
	}
	LoadAISpam(e.dispatcher)
}

// spam runs one message of member through the real dispatcher that the TypeSafe stand-in judges spam, waits for the
// check and returns the message's ID.
func (e *screeningMessagesE2E) spam(member gotgbot.User) int64 {
	e.t.Helper()
	id := e.sendIn(e.chat, member, aispamNoticeSpamText)
	waitAISpamChecks()
	return id
}

// captchaGolden compares a captcha notice with its golden, which names member 42: the joiner's ID and the chat are put
// in.
func (e *screeningMessagesE2E) captchaGolden(call moduleBotCall, golden string, member gotgbot.User) {
	e.t.Helper()
	got := screenGoldenTokens(legacyRequestJSON(e.t, call), e.chat.Id, 0, 0)
	got = strings.ReplaceAll(got, "tg://user?id="+strconv.FormatInt(member.Id, 10), "tg://user?id=42")
	if call.Method != "sendMessage" || got != golden {
		e.t.Errorf("the captcha notice is %s\n got %q\nwant %q", call.Method, got, golden)
	}
}

// reqGolden compares a requirement notice with its golden, the chat, member and hold put in.
func (e *screeningMessagesE2E) reqGolden(notice staffSentMessage, golden string, member gotgbot.User) {
	e.t.Helper()
	hold := e.mreq.mreqHold(member.Id)
	if hold == nil {
		e.t.Fatalf("no hold of member %d after the notice", member.Id)
	}
	raw := legacyRequestJSON(e.t, moduleBotCall{Method: "sendMessage", Params: notice.Params})
	if got := screenGoldenTokens(raw, e.chat.Id, member.Id, hold.ID); got != golden {
		e.t.Errorf("the requirement notice changed\n got %q\nwant %q", got, golden)
	}
}

// wantNotQueued fails when message msgID of the group has a queue row.
func (e *screeningMessagesE2E) wantNotQueued(msgID int64, what string) {
	e.t.Helper()
	if row, ok := e.queued(msgID); ok {
		e.t.Errorf("%s (message %d) is queued for deletion as %+v, want no row", what, msgID, row)
	}
}

// screeningE2ETurnstile is the web captcha's runtime over the environment's bot, with a fake Cloudflare.
type screeningE2ETurnstile struct {
	e  *screeningMessagesE2E
	rt *captchaWebRuntime
}

// turnstile installs the web captcha's runtime over the environment's bot and Redis, with a Cloudflare stand-in whose
// every answer is a failed solve, and switches the group to the Turnstile mode.
func (e *screeningMessagesE2E) turnstile() *screeningE2ETurnstile {
	e.t.Helper()
	t := e.t
	_, _, rc := cache.GetCacheState()
	if rc == nil {
		t.Fatal("the environment has no Redis client")
	}
	fake := captchaweb.SiteverifyServerForTest(t, captchaweb.SiteverifyAnswerForTest(http.StatusOK,
		`{"success":false,"error-codes":["invalid-input-response"]}`))
	rt, err := newCaptchaWebRuntime(e.bot, CaptchaWebConfig{
		BaseURL: "https://" + captchaWebTestHost, ShortName: "verify", SiteKey: "test-site-key",
		Secret: fake.Secret, Redis: rc, Siteverify: fake,
	})
	if err != nil {
		t.Fatalf("newCaptchaWebRuntime() error = %v", err)
	}
	captchaWebCurrent.Store(rt)
	captchaWebProbe.Store(nil)
	t.Cleanup(func() {
		captchaWebCurrent.Store(nil)
		captchaWebProbe.Store(nil)
	})
	if err := captcha.SetCaptchaMode(e.chat.Id, "turnstile"); err != nil {
		t.Fatalf("SetCaptchaMode(turnstile) error = %v", err)
	}
	return &screeningE2ETurnstile{e: e, rt: rt}
}

// token is the startapp value of the newest URL button the bot sent to the group.
func (w *screeningE2ETurnstile) token() string {
	w.e.t.Helper()
	sent := w.e.fake.sentTo(w.e.chat.Id)
	for i := len(sent) - 1; i >= 0; i-- {
		for _, row := range staffKeyboardOf(sent[i].Params["reply_markup"]) {
			for _, button := range row {
				if button.Url == "" {
					continue
				}
				parsed, err := url.Parse(button.Url)
				if err != nil {
					w.e.t.Fatalf("button url %q does not parse: %v", button.Url, err)
				}
				return parsed.Query().Get("startapp")
			}
		}
	}
	w.e.t.Fatal("the bot sent no Mini App button to the group")
	return ""
}

// verify posts the body a real page would post for member and token and returns the status and the answer.
func (w *screeningE2ETurnstile) verify(member gotgbot.User, token string) (int, captchaweb.Answer) {
	w.e.t.Helper()
	initData := captchaweb.InitDataForTest(w.e.bot.Token, member.Id, member.FirstName, "en", token, time.Now())
	body, err := json.Marshal(map[string]string{
		"init_data": initData, "turnstile_token": captchaweb.TurnstileTokenForTest(token), "challenge": token,
	})
	if err != nil {
		w.e.t.Fatalf("marshal the verify body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/captcha/verify", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	w.rt.handler.Verify(rec, req)
	var answer captchaweb.Answer
	if err := json.Unmarshal(rec.Body.Bytes(), &answer); err != nil {
		w.e.t.Fatalf("verify answer %q is not JSON: %v", rec.Body.String(), err)
	}
	return rec.Code, answer
}

// screeningE2ECaptchaText is what the owner types as the failed captcha's text: its first word is made bold.
const screeningE2ECaptchaText = "{mention} {action} ({count})"

func TestScreeningMessagesEndToEnd(t *testing.T) {
	t.Run("owner sets the captcha message", func(t *testing.T) {
		e := newScreeningMessagesE2E(t, false)
		e.captchaOn()
		e.setScreeningText("a", "cf", screeningE2ECaptchaText)
		if row, ok := settingsMsgStored(t, e.chat.Id, eventmsg.EventCaptcha); !ok || row.Text != "<b>{mention}</b> {action} ({count})" {
			t.Fatalf("the stored captcha message is %+v (row %v), want the typed text", row, ok)
		}
		hub := e.openHub(e.owner)
		if line, ok := settingsMainStateLine(hub.text, "settings_msg_ev_captcha"); !ok || !containsMarker(line, "settings_msg_state_custom") {
			t.Errorf("the hub's failed captcha line = %q, want Custom", line)
		}

		member := e.joiner("Ann & <Bo>", "", "")
		mark := settingsEditMarkSends(e.settingsEnv, e.chat.Id)
		attempt := e.joinCaptcha(member)
		challenges := e.groupSends(e.settingsEnv, mark)
		if len(challenges) != 1 || challenges[0].Method != "sendPhoto" {
			t.Fatalf("the join sent %+v to the group, want the one math challenge", challenges)
		}
		challenge := rulesCallParams(t, challenges[0])
		if caption := fmt.Sprint(challenge["caption"]); !containsMarker(caption, "captcha_welcome_math_image") || strings.Contains(caption, "{action}") {
			t.Errorf("the challenge caption = %q, want the math captcha's own text", caption)
		}

		// The member speaks: the message is held and deleted.
		held := e.memberSays(member, "hello before the captcha")
		if !e.deleted(held) {
			t.Errorf("the message before the captcha was not deleted")
		}

		mark, calls := settingsEditMarkSends(e.settingsEnv, e.chat.Id), len(eventMsgMethods(e.fake.moduleBotClient))
		e.expire(attempt)
		if got := e.kicks(member); got != 1 {
			t.Fatalf("the captcha kicked %d times, want once", got)
		}
		e.wantPunishedFirst(calls, "unbanChatMember", "sendMessage")
		params := pmCopiesE2EOne(t, e.groupSends(e.settingsEnv, mark), "sendMessage", "the group after the timeout")
		tr := i18n.MustNewTranslator("uk")
		want := "<b>" + formatting.MentionHtml(member.Id, member.FirstName) + "</b> " + eventActionWord(tr, "kick") + " (1)"
		if params["text"] != want {
			t.Errorf("the captcha notice reads %q, want %q", params["text"], want)
		}
		if _, ok := params["reply_parameters"]; ok {
			t.Errorf("the captcha notice is a reply: %v", params["reply_parameters"])
		}
		notice := e.lastGroup()
		e.wantNotQueued(notice.MessageID, "the captcha notice")

		deadline := time.Now().Add(2 * time.Second)
		for !e.deleted(notice.MessageID) && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		if !e.deleted(notice.MessageID) {
			t.Errorf("the captcha notice was not deleted after its life")
		}
		e.wantNotQueued(notice.MessageID, "the captcha notice after its life")
	})

	t.Run("web check", func(t *testing.T) {
		e := newScreeningMessagesE2E(t, false)
		e.captchaOn()
		web := e.turnstile()
		e.setScreeningText("a", "cf", screeningE2ECaptchaText)
		member := e.joiner("Ann & <Bo>", "", "")

		mark := settingsEditMarkSends(e.settingsEnv, e.chat.Id)
		e.joinCaptcha(member)
		challenges := e.groupSends(e.settingsEnv, mark)
		if len(challenges) != 1 || challenges[0].Method != "sendMessage" ||
			!containsMarker(fmt.Sprint(rulesCallParams(t, challenges[0])["text"]), "captcha_web_welcome") {
			t.Fatalf("the join sent %+v to the group, want the one Turnstile challenge", challenges)
		}
		token := web.token()
		if held := e.memberSays(member, "hello before the check"); !e.deleted(held) {
			t.Errorf("the message before the check was not deleted")
		}

		mark, calls := settingsEditMarkSends(e.settingsEnv, e.chat.Id), len(eventMsgMethods(e.fake.moduleBotClient))
		status, answer := web.verify(member, token)
		if status != http.StatusOK || answer.Result != captchaweb.ResultFailed || answer.Action != "kick" {
			t.Fatalf("verify = %d %+v, want 200 failed with kick, as before", status, answer)
		}
		if got := e.kicks(member); got != 1 {
			t.Fatalf("the failed check kicked %d times, want once", got)
		}
		e.wantPunishedFirst(calls, "unbanChatMember", "sendMessage")
		params := pmCopiesE2EOne(t, e.groupSends(e.settingsEnv, mark), "sendMessage", "the group after the failed check")
		tr := i18n.MustNewTranslator("uk")
		if want := "<b>" + formatting.MentionHtml(member.Id, member.FirstName) + "</b> " + eventActionWord(tr, "kick") + " (1)"; params["text"] != want {
			t.Errorf("the captcha notice reads %q, want %q", params["text"], want)
		}

		// The same check again is answered from the decided-answer marker: the same answer, nothing new.
		mark = settingsEditMarkSends(e.settingsEnv, e.chat.Id)
		if status, again := web.verify(member, token); status != http.StatusOK || again != answer {
			t.Errorf("the repeated verify = %d %+v, want the same answer %+v", status, again, answer)
		}
		if sends := e.groupSends(e.settingsEnv, mark); len(sends) != 0 || e.kicks(member) != 1 {
			t.Errorf("the repeated verify posted %+v and kicked %d times, want nothing new", sends, e.kicks(member))
		}
	})

	t.Run("requirement mute", func(t *testing.T) {
		e := newScreeningMessagesE2E(t, false)
		clock := e.clock()
		screen := e.openScreening(e.owner, "r", "rm")
		if screen.has("a", settingsActMsgSet, "t", settingsEditPartMedia) || screen.has("a", settingsActMsgSilent) {
			t.Errorf("the requirement mute screen offers a media part or a Silent switch: %+v", screen.keyboard)
		}
		e.setScreeningText("r", "rm", "{mention} {action}: {reason}")
		e.setScreeningPart("r", "rm", settingsEditPartButtons, gotgbot.Message{Text: "[Rules](buttonurl://https://example.com/rules)"})
		if row, ok := settingsMsgStored(t, e.chat.Id, eventmsg.EventReqMute); !ok || row.Text != "<b>{mention}</b> {action}: {reason}" || len(row.Buttons) != 1 {
			t.Fatalf("the stored requirement mute message is %+v (row %v), want the typed text and one button", row, ok)
		}

		member := e.joiner("Ann & <Bo>", "", "")
		mark, calls := settingsEditMarkSends(e.settingsEnv, e.chat.Id), len(eventMsgMethods(e.fake.moduleBotClient))
		e.breakRule(member, models.RequirementActionMute)
		e.wantPunishedFirst(calls, "restrictChatMember", "sendMessage")
		hold := e.mreq.mreqHold(member.Id)
		if hold == nil || hold.Action != models.RequirementActionMute || hold.PunishedAt == nil {
			t.Fatalf("hold = %+v, want a punished mute hold", hold)
		}
		params := pmCopiesE2EOne(t, e.groupSends(e.settingsEnv, mark), "sendMessage", "the group after the message")
		tr := i18n.MustNewTranslator("uk")
		rules := memberReqRuleBits("ln")
		want := "<b>" + formatting.MentionHtml(member.Id, member.FirstName) + "</b> " + eventActionWord(tr, "mute") + ": " +
			memberReqRulesText(tr, rules) + "\n" + memberReqNoticeNote(tr, rules, "mreq_notice_mute")
		if params["text"] != want {
			t.Errorf("the requirement mute notice reads %q, want %q", params["text"], want)
		}
		rows := settingsMsgSentKeyboard(t, params)
		fixed := encodeCallbackData("mreq", map[string]string{"a": "f", "h": strconv.FormatInt(hold.ID, 10), "u": strconv.FormatInt(member.Id, 10)})
		if len(rows) != 2 || len(rows[0]) != 1 || len(rows[1]) != 1 || rows[0][0].Url != "https://example.com/rules" || rows[1][0].CallbackData != fixed {
			t.Fatalf("the requirement mute keyboard = %+v, want the custom button, then \"I fixed it\"", rows)
		}
		notice := e.lastGroup()
		if hold.NoticeMsgID != notice.MessageID {
			t.Errorf("the hold names notice %d, want %d", hold.NoticeMsgID, notice.MessageID)
		}
		e.wantQueuedAs(notice.MessageID, deletion.KindBotAuto, int((24 * time.Hour).Seconds()), "the requirement mute notice")

		// The member adds a last name and taps "I fixed it" 20 minutes later: unmuted, the notice edited, its row
		// moved up to the tap plus the timer.
		member = e.mreq.mreqFix(member)
		clock.advance(20 * time.Minute)
		restricts := len(e.mreq.mreqRestricts(member.Id))
		e.mreq.mreqPressData(member, notice.MessageID, fixed)
		if hold := e.mreq.mreqHold(member.Id); hold != nil {
			t.Errorf("hold after the tap = %+v, want none", hold)
		}
		after := e.mreq.mreqRestricts(member.Id)
		if len(after) != restricts+1 {
			t.Fatalf("restrictChatMember calls of the member = %d, want one more (the unmute)", len(after))
		}
		if perms := permissionsOf(t, after[len(after)-1]); !perms.CanSendMessages {
			t.Errorf("the tap restored %+v, want the member able to send", perms)
		}
		if text, keyboard := e.lastEdit(e.chat.Id, notice.MessageID); !containsMarker(text, "mreq_notice_fixed") || len(keyboard) != 0 {
			t.Errorf("the edited notice = %q with %+v, want the fix text and no keyboard", text, keyboard)
		}
		row, ok := e.queued(notice.MessageID)
		if want := clock.now().Add(autoMessagesE2EAutoDelay * time.Second); !ok || !row.DeleteAt.Equal(want) {
			t.Errorf("the notice's row = %+v (%v), want it due at the tap plus the timer, %s", row, ok, want)
		}
	})

	t.Run("requirement warn", func(t *testing.T) {
		e := newScreeningMessagesE2E(t, false)
		e.setScreeningText("r", "rw", "Warned {first}: {reason}")
		member := e.joiner("Ann & <Bo>", "", "")

		mark := settingsEditMarkSends(e.settingsEnv, e.chat.Id)
		e.breakRule(member, models.RequirementActionWarn)
		if got := e.warnsOf(member); got != 1 {
			t.Fatalf("stored warns = %d, want 1", got)
		}
		params := pmCopiesE2EOne(t, e.groupSends(e.settingsEnv, mark), "sendMessage", "the group after the message")
		tr := i18n.MustNewTranslator("uk")
		rules := memberReqRuleBits("ln")
		want := "<b>Warned</b> Ann &amp; &lt;Bo&gt;: " + memberReqRulesText(tr, rules) + "\n" + memberReqNoticeNote(tr, rules, "mreq_notice_warn")
		if params["text"] != want {
			t.Errorf("the requirement warn notice reads %q, want %q", params["text"], want)
		}
		if rows := settingsMsgSentKeyboard(t, params); len(rows) != 0 {
			t.Errorf("the requirement warn notice carries %+v, want no keyboard", rows)
		}
		e.wantQueuedAs(e.lastGroup().MessageID, deletion.KindBotAuto, autoMessagesE2EAutoDelay, "the requirement warn notice")
	})

	t.Run("ai notice", func(t *testing.T) {
		e := newScreeningMessagesE2E(t, false)
		e.aispamOn()
		minute := time.Date(2026, 11, 22, 12, 0, 5, 0, time.UTC)
		mreqSwap(t, &aispamNoticeNow, func() time.Time { return minute })
		tr := i18n.MustNewTranslator("uk")
		notice := func(member gotgbot.User) string {
			text, _ := tr.GetString("aispam_notice_deleted", i18n.TranslationParams{
				"name": formatting.MentionHtml(member.Id, formatting.GetFullName(member.FirstName, member.LastName)),
			})
			return text
		}
		member := e.joiner("Spammer", "Lee", "")

		// Nothing stored: the spam is deleted and nothing is posted, and the notice spends no Redis command.
		mark := settingsEditMarkSends(e.settingsEnv, e.chat.Id)
		ids := []int64{e.spam(member)}
		if sends := e.groupSends(e.settingsEnv, mark); len(sends) != 0 {
			t.Fatalf("the spam posted %+v with the notice off, want nothing", sends)
		}
		for _, key := range e.redis.Keys() {
			if strings.HasPrefix(key, "fuku:aispam:notice:") {
				t.Errorf("the notice off wrote the key %s", key)
			}
		}

		// The owner turns the notice on with the drawn Notice switch.
		e.openScreening(e.owner, "a", "ai").tap("a", settingsActMsgSilent, settingsMsgEventField, "ai", "v", "0")
		if row, ok := settingsMsgStored(t, e.chat.Id, eventmsg.EventAISpam); !ok || row.Silent {
			t.Fatalf("after the Notice press the aispam row is %+v (row %v), want the notice on", row, ok)
		}

		// The same member's next spam: deleted, then one notice; the third inside five minutes: deleted, no notice.
		mark, calls := settingsEditMarkSends(e.settingsEnv, e.chat.Id), len(eventMsgMethods(e.fake.moduleBotClient))
		ids = append(ids, e.spam(member))
		e.wantPunishedFirst(calls, "deleteMessage", "sendMessage")
		params := pmCopiesE2EOne(t, e.groupSends(e.settingsEnv, mark), "sendMessage", "the group after the second spam")
		if params["text"] != notice(member) {
			t.Errorf("the AI spam notice reads %q, want %q", params["text"], notice(member))
		}
		if _, ok := params["reply_parameters"]; ok {
			t.Errorf("the AI spam notice is a reply: %v", params["reply_parameters"])
		}
		e.wantQueuedAs(e.lastGroup().MessageID, deletion.KindBotAuto, autoMessagesE2EAutoDelay, "the AI spam notice")
		mark = settingsEditMarkSends(e.settingsEnv, e.chat.Id)
		ids = append(ids, e.spam(member))
		if sends := e.groupSends(e.settingsEnv, mark); len(sends) != 0 {
			t.Errorf("the member's third spam inside five minutes posted %+v, want nothing", sends)
		}

		// Three other members each spam once: every message is deleted and two more notices fill the minute's three.
		mark = settingsEditMarkSends(e.settingsEnv, e.chat.Id)
		for _, name := range []string{"Bob", "Cid", "Dee"} {
			ids = append(ids, e.spam(e.joiner(name, "Lee", "")))
		}
		if sends := e.groupSends(e.settingsEnv, mark); len(sends) != aispamNoticePerMinute-1 {
			t.Errorf("notices of the three other members = %d, want %d (three in the minute)", len(sends), aispamNoticePerMinute-1)
		}
		for _, id := range ids {
			if !e.deleted(id) {
				t.Errorf("spam message %d was not deleted", id)
			}
		}
	})

	t.Run("nothing stored", func(t *testing.T) {
		e := newScreeningMessagesE2E(t, true)
		e.captchaOn()

		joiner := e.joiner("Ann & <Bo>", "", "")
		attempt := e.joinCaptcha(joiner)
		mark := settingsEditMarkSends(e.settingsEnv, e.chat.Id)
		e.expire(attempt)
		sends := e.groupSends(e.settingsEnv, mark)
		if len(sends) != 1 || e.kicks(joiner) != 1 {
			t.Fatalf("the captcha timeout posted %+v and kicked %d times, want the one default after one kick", sends, e.kicks(joiner))
		}
		e.captchaGolden(sends[0], goldenScreenCaptchaKick, joiner)
		e.wantNotQueued(e.lastGroup().MessageID, "the default captcha notice")

		kicked := e.joiner("Ann & <Bo>", "", "ann_lee")
		mark = settingsEditMarkSends(e.settingsEnv, e.chat.Id)
		e.breakRule(kicked, models.RequirementActionKick)
		if sends := e.groupSends(e.settingsEnv, mark); len(sends) != 1 || e.kicks(kicked) != 1 {
			t.Fatalf("the requirement kick posted %+v and kicked %d times, want the one default after one kick", sends, e.kicks(kicked))
		}
		e.reqGolden(e.lastGroup(), goldenScreenReqKick, kicked)
		e.wantQueuedAs(e.lastGroup().MessageID, deletion.KindBotAuto, autoMessagesE2EAutoDelay, "the default requirement kick notice")

		muted := e.joiner("Ann & <Bo>", "", "ann_lee")
		mark = settingsEditMarkSends(e.settingsEnv, e.chat.Id)
		e.breakRule(muted, models.RequirementActionMute)
		if sends := e.groupSends(e.settingsEnv, mark); len(sends) != 1 || e.punished(e.settingsEnv, "restrictChatMember", muted) != 1 {
			t.Fatalf("the requirement mute posted %+v, want the one default after one mute", sends)
		}
		e.reqGolden(e.lastGroup(), goldenScreenReqMute, muted)
		e.wantQueuedAs(e.lastGroup().MessageID, deletion.KindBotAuto, int((24 * time.Hour).Seconds()), "the default requirement mute notice")
	})

	t.Run("refused", func(t *testing.T) {
		e := newScreeningMessagesE2E(t, true)
		e.captchaOn()
		e.setScreeningText("a", "cf", screeningE2ECaptchaText)
		e.setScreeningPart("a", "cf", settingsEditPartMedia, settingsMediaItemID("photo", "captcha-photo"))
		joiner := e.joiner("Ann & <Bo>", "", "")
		attempt := e.joinCaptcha(joiner)
		e.fake.script("sendPhoto", e.chat.Id, staffFakeError(400, "Bad Request: wrong file identifier/HTTP URL specified"))

		mark, calls := settingsEditMarkSends(e.settingsEnv, e.chat.Id), len(eventMsgMethods(e.fake.moduleBotClient))
		e.expire(attempt)
		methods := methodsSince(e.settingsEnv, calls)
		kick, photo, text := eventMsgIndex(methods, "unbanChatMember"), eventMsgIndex(methods, "sendPhoto"), eventMsgIndex(methods, "sendMessage")
		if kick < 0 || photo < kick || text < photo {
			t.Fatalf("methods = %v, want the kick, then the refused photo, then the default", methods)
		}
		sends := e.groupSends(e.settingsEnv, mark)
		var defaults []moduleBotCall
		for _, call := range sends {
			if call.Method == "sendMessage" {
				defaults = append(defaults, call)
			}
		}
		if len(sends) != 2 || len(defaults) != 1 {
			t.Fatalf("the group got %+v, want the refused photo and one default", sends)
		}
		e.captchaGolden(defaults[0], goldenScreenCaptchaKick, joiner)
		e.wantNotQueued(e.lastGroup().MessageID, "the default after the refusal")
	})

	t.Run("pm copies", func(t *testing.T) {
		e := newScreeningMessagesE2E(t, false)
		tr := i18n.MustNewTranslator("uk")

		// The requirement mute's private copy, switched on from its private copy screen.
		m := e.openScreening(e.owner, "r", "rm")
		m.tap("a", settingsActMsgEvent, settingsMsgDestField, settingsMsgDestPMCode)
		m.tap("a", settingsActMsgPM, "v", "1")
		if row, _ := settingsMsgStoredPM(t, e.chat.Id, eventmsg.EventReqMute); row.Silent {
			t.Fatal("after the switch the requirement mute's private copy is still off")
		}
		member := e.joiner("Ann & <Bo>", "", "")
		e.breakRule(member, models.RequirementActionMute)
		params := pmCopiesE2EOne(t, e.privateSends(e.settingsEnv, member), "sendMessage", "the member's private chat")
		want := eventPMAutoDefault(t, "eventmsg_pm_mreqmute", i18n.TranslationParams{"group": staffDisplayTitle(e.chat.Title)},
			memberReqRulesText(tr, memberReqRuleBits("ln")))
		if params["text"] != want {
			t.Errorf("the requirement mute's private copy reads %q, want %q", params["text"], want)
		}
		if rows := settingsMsgSentKeyboard(t, params); len(rows) != 0 {
			t.Errorf("the private copy carries %+v, want no keyboard", rows)
		}
		if notices := e.mreq.mreqNotices(); len(notices) != 1 || e.punished(e.settingsEnv, "restrictChatMember", member) != 1 {
			t.Errorf("group notices = %d, want the one mute notice after the one mute", len(notices))
		}

		// The failed captcha's private copy on, the cap at 2 in one stopped minute: three joiners fail the captcha,
		// every one is kicked and noticed, and exactly two copies are sent.
		m = e.openScreening(e.owner, "a", "cf")
		m.tap("a", settingsActMsgEvent, settingsMsgDestField, settingsMsgDestPMCode)
		m.tap("a", settingsActMsgPM, "v", "1")
		e.captchaOn()
		now := time.Date(2026, 11, 22, 12, 0, 5, 0, time.UTC)
		mreqSwap(t, &pmCopyNow, func() time.Time { return now })
		mreqSwap(t, &pmCopyAutoCapPerMinute, 2)
		t.Cleanup(pmCopyJobs.wait)

		mark := settingsEditMarkSends(e.settingsEnv, e.chat.Id)
		copies := 0
		var joiners []gotgbot.User
		for _, name := range []string{"Ann", "Bob", "Cid"} {
			joiner := e.joiner(name, "Lee", "")
			joiners = append(joiners, joiner)
			e.joinAndFail(joiner)
		}
		waitEventPM(t)
		for _, joiner := range joiners {
			if got := e.kicks(joiner); got != 1 {
				t.Errorf("joiner %d was kicked %d times, want once", joiner.Id, got)
			}
			copies += len(e.privateSends(e.settingsEnv, joiner))
		}
		notices := 0
		for _, call := range e.groupSends(e.settingsEnv, mark) {
			if call.Method == "sendMessage" {
				notices++
			}
		}
		if notices != len(joiners) {
			t.Errorf("captcha notices = %d, want one per joiner (%d)", notices, len(joiners))
		}
		if copies != 2 {
			t.Errorf("captcha private copies in one minute = %d, want the cap of 2", copies)
		}
	})
}
