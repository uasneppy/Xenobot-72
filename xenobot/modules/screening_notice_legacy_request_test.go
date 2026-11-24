//go:build testtools

package modules

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/aispam"
	dbcache "github.com/uasneppy/Xenobot-72/xenobot/db/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/db/captcha"
	"github.com/uasneppy/Xenobot-72/xenobot/db/deletion"
	"github.com/uasneppy/Xenobot-72/xenobot/db/memberreq"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/db/warns"
)

// Goldens recorded on the code as it was before Phase 24 (the captcha failure notice, the member
// requirement notices with the pause notice they share a sender with, and aispam's breaker notice):
// <chat> stands for the case's chat ID, <user> for the requirement member's ID and <hold> for the
// requirement hold's ID. They are never re-recorded: a later change that alters one of these requests
// for a group with nothing stored is a regression.
const (
	goldenScreenCaptchaKick      = "{\"chat_id\":<chat>,\"parse_mode\":\"HTML\",\"text\":\"❌ \\u003ca href=\\\"tg://user?id=42\\\"\\u003eAnn \\u0026amp; \\u0026lt;Bo\\u0026gt;\\u003c/a\\u003e: перевірку капчею не пройдено. Користувача вигнано.\"}"
	goldenScreenCaptchaBan       = "{\"chat_id\":<chat>,\"parse_mode\":\"HTML\",\"text\":\"❌ \\u003ca href=\\\"tg://user?id=42\\\"\\u003eAnn \\u0026amp; \\u0026lt;Bo\\u0026gt;\\u003c/a\\u003e: перевірку капчею не пройдено. Користувача забанено.\"}"
	goldenScreenCaptchaMute      = "{\"chat_id\":<chat>,\"parse_mode\":\"HTML\",\"text\":\"❌ \\u003ca href=\\\"tg://user?id=42\\\"\\u003eAnn \\u0026amp; \\u0026lt;Bo\\u0026gt;\\u003c/a\\u003e: перевірку капчею не пройдено. Користувач отримує мут.\"}"
	goldenScreenCaptchaKickHeld  = "{\"chat_id\":<chat>,\"parse_mode\":\"HTML\",\"text\":\"❌ \\u003ca href=\\\"tg://user?id=42\\\"\\u003eAnn \\u0026amp; \\u0026lt;Bo\\u0026gt;\\u003c/a\\u003e: капчу не пройдено, результат — вигнання.\\n\\n📝 Повідомлень, надісланих до перевірки, видалено: 2.\"}"
	goldenScreenReqWarn          = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"text\":\"❗ \\u003ca href=\\\"tg://user?id=<user>\\\"\\u003e\u2068Ann \\u0026amp; \\u0026lt;Bo\\u0026gt;\u2069\\u003c/a\\u003e, ви порушуєте вимоги групи: немає прізвища. Попередження 1 з 3.\"}"
	goldenScreenReqWarnLimitKick = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"text\":\"❗ \\u003ca href=\\\"tg://user?id=<user>\\\"\\u003e\u2068Ann \\u0026amp; \\u0026lt;Bo\\u0026gt;\u2069\\u003c/a\\u003e порушує вимоги групи: немає прізвища. Це 1 з 1 попереджень, тому учасника вигнано.\"}"
	goldenScreenReqWarnLimitBan  = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"text\":\"❗ \\u003ca href=\\\"tg://user?id=<user>\\\"\\u003e\u2068Ann \\u0026amp; \\u0026lt;Bo\\u0026gt;\u2069\\u003c/a\\u003e порушує вимоги групи: немає прізвища. Це 1 з 1 попереджень, тому учасника забанено.\"}"
	goldenScreenReqWarnLimitMute = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"text\":\"❗ \\u003ca href=\\\"tg://user?id=<user>\\\"\\u003e\u2068Ann \\u0026amp; \\u0026lt;Bo\\u0026gt;\u2069\\u003c/a\\u003e порушує вимоги групи: немає прізвища. Це 1 з 1 попереджень, тому учасник отримує мут. Адмін може зняти мут.\"}"
	goldenScreenReqKick          = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"text\":\"👢 Учасника \\u003ca href=\\\"tg://user?id=<user>\\\"\\u003e\u2068Ann \\u0026amp; \\u0026lt;Bo\\u0026gt;\u2069\\u003c/a\\u003e вигнано: немає прізвища. Повернутися можна, коли вказане буде виправлено.\"}"
	goldenScreenReqBan           = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"text\":\"⛔ Учасника \\u003ca href=\\\"tg://user?id=<user>\\\"\\u003e\u2068Ann \\u0026amp; \\u0026lt;Bo\\u0026gt;\u2069\\u003c/a\\u003e забанено: немає прізвища. Розбанити може адмін.\"}"
	goldenScreenReqMute          = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_markup\":{\"inline_keyboard\":[[{\"text\":\"✅ Виправлено\",\"callback_data\":\"mreq|v1|a=f\\u0026h=<hold>\\u0026u=<user>\"}]]},\"text\":\"🔇 \\u003ca href=\\\"tg://user?id=<user>\\\"\\u003e\u2068Ann \\u0026amp; \\u0026lt;Bo\\u0026gt;\u2069\\u003c/a\\u003e, вам видано мут: немає прізвища. Виправте вказане і натисніть «Виправлено».\"}"
	goldenScreenReqPaused        = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"text\":\"⏸ Покарання за вимогами тут призупинено на 10 хв: понад 1 учасників порушили вимогу за хвилину. Адмін може перевірити «Вимоги» в /settings.\"}"
	goldenScreenAISpamBreaker    = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"allow_sending_without_reply\":true},\"text\":\"⚠️ AI-перевірки спаму в цьому чаті призупинено на кілька хвилин через повторні збої AI-провайдера. Вони відновляться самі, а фільтр лишається увімкненим.\"}"
)

// screenGoldenChallenge is the captcha challenge message the captcha cases delete.
const screenGoldenChallenge = 77

// screenGoldenSpam is the message the AI spam case's verdict deletes.
const screenGoldenSpam = 501

// newScreenGoldenSite is newNoticeGoldenSite for the screening goldens: a bot site in a fresh
// supergroup whose client answers the golden member 42 (Ann & <Bo>), with the real locales, a 600 s
// bot:punish_auto rule and a 900 s bot:punish_manual rule, so a row queued under either kind shows.
// The captcha attempts and held messages, the requirement rows, the AI spam switch and the member's
// users row are removed when the test ends.
func newScreenGoldenSite(t *testing.T) *botSite {
	t.Helper()
	if moderationGoldenLocales == 0 {
		withRealLocales(t)
	}
	s := newBotSite(t, "supergroup")
	s.bot.BotClient = noticeGoldenClient{moduleBotClient: s.client}
	s.setRule(s.chat.Id, deletion.KindBotAuto, 600)
	s.setRule(s.chat.Id, deletion.KindBotManual, 900)
	memberReqCleanup(t, []int64{s.chat.Id}, []int64{noticeGoldenTarget.Id})
	chatID := s.chat.Id
	t.Cleanup(func() {
		db.DB.Where("chat_id = ?", chatID).Delete(&models.StoredMessages{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.AISpamSettings{})
		dbcache.DeleteCache(dbcache.CacheKey("ai_spam_settings", chatID))
		dbcache.ResetLocalForTest()
	})
	return s
}

// newScreenGoldenMreqEnv is the member requirements environment with the real locales, the deletion
// clock fixed at a whole second (as newMreqSchedEnv fixes it), a 600 s bot:punish_auto rule and a
// 900 s bot:punish_manual rule.
func newScreenGoldenMreqEnv(t *testing.T) *mreqSchedEnv {
	t.Helper()
	env := &mreqSchedEnv{memberReqEnv: newMemberReqEnvWith(t, settingsEnvOptions{RealLocale: true}), clock: &atomic.Int64{}}
	env.clock.Store(time.Now().UTC().Truncate(time.Second).Unix())
	mreqSwap(t, &deletionNow, func() time.Time { return time.Unix(env.clock.Load(), 0).UTC() })
	env.fake.sendDate = func() int64 { return env.clock.Load() - env.sentBack.Load() }
	deletionCleanup(t, env.chat.Id)
	env.setRule(deletion.KindBotAuto, 600)
	env.setRule(deletion.KindBotManual, 900)
	return env
}

// screenGoldenReqMember mints the golden requirement member: the first name Ann & <Bo>, no last
// name (so the last-name rule is broken) and the username ann_lee, with a fresh user ID.
func screenGoldenReqMember(env *mreqSchedEnv) gotgbot.User {
	return env.mreqMember("Ann & <Bo>", "", "ann_lee")
}

// screenGoldenTokens replaces the chat, member and hold IDs of a serialised request with <chat>,
// <user> and <hold>. The hold ID is replaced only where the "I fixed it" data names it, because a
// small number would also match inside the text; a zero ID replaces nothing.
func screenGoldenTokens(raw string, chatID, userID, holdID int64) string {
	raw = strings.ReplaceAll(raw, strconv.FormatInt(chatID, 10), "<chat>")
	if holdID != 0 {
		raw = strings.ReplaceAll(raw, "h="+strconv.FormatInt(holdID, 10), "h=<hold>")
	}
	if userID != 0 {
		raw = strings.ReplaceAll(raw, strconv.FormatInt(userID, 10), "<user>")
	}
	return raw
}

// screenGoldenCaptcha seeds a pending attempt of member 42 (with two held messages when held) and
// runs handleCaptchaTimeout with action, which must claim the attempt and apply the action.
func screenGoldenCaptcha(t *testing.T, s *botSite, action string, held bool) {
	t.Helper()
	attempt, err := captcha.CreateCaptchaAttemptPreMessage(noticeGoldenTarget.Id, s.chat.Id, "7", 2)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
	}
	if held {
		for _, text := range []string{"first held", "second held"} {
			if err := captcha.StoreMessageForCaptcha(noticeGoldenTarget.Id, s.chat.Id, attempt.ID, db.TEXT, text, "", ""); err != nil {
				t.Fatalf("StoreMessageForCaptcha() error = %v", err)
			}
		}
	}
	claimed, applied := handleCaptchaTimeout(s.bot, s.chat.Id, noticeGoldenTarget.Id, attempt.ID, screenGoldenChallenge, action)
	if !claimed || !applied {
		t.Fatalf("handleCaptchaTimeout(%s) = (%v, %v), want claimed and applied", action, claimed, applied)
	}
}

// screenGoldenActions are the calls a punishment can make; a case makes exactly its own and none of
// the others.
var screenGoldenActions = []string{"restrictChatMember", "banChatMember", "unbanChatMember"}

// screenGoldenWantAction fails unless calls holds exactly one call of method about userID (none when
// method is empty) and none of the other punishment calls, and returns that call.
func screenGoldenWantAction(t *testing.T, callsFor func(string) []moduleBotCall, method string, userID int64) moduleBotCall {
	t.Helper()
	var only moduleBotCall
	for _, m := range screenGoldenActions {
		var about []moduleBotCall
		for _, call := range callsFor(m) {
			if fmt.Sprint(call.Params["user_id"]) == strconv.FormatInt(userID, 10) {
				about = append(about, call)
			}
		}
		want := 0
		if m == method {
			want = 1
		}
		if len(about) != want {
			t.Fatalf("%s calls about %d = %d, want %d", m, userID, len(about), want)
		}
		if want == 1 {
			only = about[0]
		}
	}
	return only
}

// screenGoldenCaptchaCase is one captcha case of TestLegacyScreeningNoticeRequests.
type screenGoldenCaptchaCase struct {
	name   string
	action string
	held   bool
	method string // the failure action's call
	golden string
}

// screenGoldenReqCase is one requirement case of TestLegacyScreeningNoticeRequests.
type screenGoldenReqCase struct {
	name      string
	action    int    // the last-name rule's punishment
	warnMode  string // with a warn: the warn limit is 1 and this its mode; empty keeps the default limit
	method    string // the punishment call; empty for none
	warnsLeft int    // the member's stored warns after the case (checked for a warn)
	delay     time.Duration
	golden    string
}

func TestLegacyScreeningNoticeRequests(t *testing.T) {
	withModerationGoldenLocales(t)

	for _, tc := range []screenGoldenCaptchaCase{
		{name: "captcha_kick", action: "kick", method: "unbanChatMember", golden: goldenScreenCaptchaKick},
		{name: "captcha_ban", action: "ban", method: "banChatMember", golden: goldenScreenCaptchaBan},
		{name: "captcha_mute", action: "mute", method: "restrictChatMember", golden: goldenScreenCaptchaMute},
		{name: "captcha_kick_held", action: "kick", held: true, method: "unbanChatMember", golden: goldenScreenCaptchaKickHeld},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newScreenGoldenSite(t)

			screenGoldenCaptcha(t, s, tc.action, tc.held)

			deletes := s.client.callsFor("deleteMessage")
			if len(deletes) != 1 || fmt.Sprint(deletes[0].Params["message_id"]) != strconv.Itoa(screenGoldenChallenge) {
				t.Fatalf("deleteMessage calls = %+v, want one of the challenge %d", deletes, screenGoldenChallenge)
			}
			screenGoldenWantAction(t, s.client.callsFor, tc.method, noticeGoldenTarget.Id)
			sends := s.client.callsFor("sendMessage")
			if len(sends) != 1 {
				t.Fatalf("sendMessage calls = %d, want the one notice", len(sends))
			}
			if got := screenGoldenTokens(legacyRequestJSON(t, sends[0]), s.chat.Id, 0, 0); got != tc.golden {
				t.Fatalf("sendMessage request changed\n got %q\nwant %q", got, tc.golden)
			}
			// The notice deletes itself 30 s later in process and never enters the deletion queue,
			// whatever either bot kind's rule says.
			s.wantNoRows(s.chat.Id)
		})
	}

	for _, tc := range []screenGoldenReqCase{
		{name: "req_warn", action: models.RequirementActionWarn, warnsLeft: 1,
			delay: 600 * time.Second, golden: goldenScreenReqWarn},
		{name: "req_warnlimit_kick", action: models.RequirementActionWarn, warnMode: "kick", method: "unbanChatMember",
			delay: 600 * time.Second, golden: goldenScreenReqWarnLimitKick},
		{name: "req_warnlimit_ban", action: models.RequirementActionWarn, warnMode: "ban", method: "banChatMember",
			delay: 600 * time.Second, golden: goldenScreenReqWarnLimitBan},
		{name: "req_warnlimit_mute", action: models.RequirementActionWarn, warnMode: "mute", method: "restrictChatMember",
			delay: 600 * time.Second, golden: goldenScreenReqWarnLimitMute},
		{name: "req_kick", action: models.RequirementActionKick, method: "unbanChatMember",
			delay: 600 * time.Second, golden: goldenScreenReqKick},
		{name: "req_ban", action: models.RequirementActionBan, method: "banChatMember",
			delay: 600 * time.Second, golden: goldenScreenReqBan},
		{name: "req_mute", action: models.RequirementActionMute, method: "restrictChatMember",
			delay: 24 * time.Hour, golden: goldenScreenReqMute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := newScreenGoldenMreqEnv(t)
			env.mreqRules(map[memberreq.Rule]int{memberreq.RuleLastName: tc.action})
			if tc.warnMode != "" {
				if err := warns.SetWarnLimit(env.chat.Id, 1); err != nil {
					t.Fatalf("SetWarnLimit() error = %v", err)
				}
				if err := warns.SetWarnMode(env.chat.Id, tc.warnMode); err != nil {
					t.Fatalf("SetWarnMode(%q) error = %v", tc.warnMode, err)
				}
			}
			member := screenGoldenReqMember(env)

			got := env.mreqEnforce(env.mreqCase(member))
			if got.Outcome == memberReqNone || got.Hold == nil {
				t.Fatalf("enforcement = %+v, want a punishment with a hold", got)
			}

			call := screenGoldenWantAction(t, env.calls, tc.method, member.Id)
			if tc.action == models.RequirementActionMute {
				// The requirement mute carries the hold's marker as its end date.
				if until := fmt.Sprint(call.Params["until_date"]); until != strconv.FormatInt(got.Hold.MuteUntil, 10) {
					t.Fatalf("restrictChatMember until_date = %s, want the hold's marker %d", until, got.Hold.MuteUntil)
				}
			}
			if tc.action == models.RequirementActionWarn {
				if n, _ := warns.GetWarns(member.Id, env.chat.Id); n != tc.warnsLeft {
					t.Fatalf("stored warns = %d, want %d", n, tc.warnsLeft)
				}
			}
			notices := env.mreqNotices()
			if len(notices) != 1 {
				t.Fatalf("notices = %d, want the one notice", len(notices))
			}
			raw := legacyRequestJSON(t, moduleBotCall{Method: "sendMessage", Params: notices[0].Params})
			if got := screenGoldenTokens(raw, env.chat.Id, member.Id, got.Hold.ID); got != tc.golden {
				t.Fatalf("sendMessage request changed\n got %q\nwant %q", got, tc.golden)
			}
			env.wantRowFor(notices[0].MessageID, deletion.KindBotAuto, env.now().Add(tc.delay))
		})
	}

	t.Run("req_paused", func(t *testing.T) {
		mreqSwap(t, &memberReqCapPerMinute, 1)
		mreqFixedClock(t)
		env := newScreenGoldenMreqEnv(t)
		env.mreqRules(map[memberreq.Rule]int{memberreq.RuleLastName: models.RequirementActionKick})

		first := screenGoldenReqMember(env)
		firstGot := env.mreqEnforce(env.mreqCase(first))
		if firstGot.Outcome != memberReqRemoved || firstGot.Hold == nil {
			t.Fatalf("first member = %+v, want Kicked", firstGot)
		}
		second := screenGoldenReqMember(env)
		if got := env.mreqEnforce(env.mreqCase(second)); got.Outcome != memberReqNone {
			t.Fatalf("second member = %+v, want None (the cap refuses)", got)
		}

		screenGoldenWantAction(t, env.calls, "unbanChatMember", first.Id)
		screenGoldenWantAction(t, env.calls, "", second.Id)
		notices := env.mreqNotices()
		if len(notices) != 2 {
			t.Fatalf("notices = %d, want the kick notice and the pause notice", len(notices))
		}
		kick := legacyRequestJSON(t, moduleBotCall{Method: "sendMessage", Params: notices[0].Params})
		if got := screenGoldenTokens(kick, env.chat.Id, first.Id, firstGot.Hold.ID); got != goldenScreenReqKick {
			t.Fatalf("kick notice changed\n got %q\nwant %q", got, goldenScreenReqKick)
		}
		paused := legacyRequestJSON(t, moduleBotCall{Method: "sendMessage", Params: notices[1].Params})
		if got := screenGoldenTokens(paused, env.chat.Id, 0, 0); got != goldenScreenReqPaused {
			t.Fatalf("pause notice changed\n got %q\nwant %q", got, goldenScreenReqPaused)
		}
		rows := env.queueRows()
		if len(rows) != 2 {
			t.Fatalf("queue rows = %+v, want one for the kick notice and one for the pause notice", rows)
		}
		for _, notice := range notices {
			row, ok := rows[notice.MessageID]
			if !ok || row.Kind != deletion.KindBotAuto || !row.DeleteAt.Equal(env.now().Add(600*time.Second)) {
				t.Fatalf("notice %d row = %+v (%v), want a bot:punish_auto row due in 600 s", notice.MessageID, row, ok)
			}
		}
	})

	t.Run("aispam_delete_silent", func(t *testing.T) {
		resetAISpamRuntime()
		t.Cleanup(resetAISpamRuntime)
		withAISpamTestConfig(t, "typesafe-test-key", true, 0)
		withAISpamJevServer(t, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprint(w, aispamDecisionBody(0.95, "scam"))
		})
		s := newScreenGoldenSite(t)
		if err := aispam.SetAISpamEnabled(s.chat.Id, true); err != nil {
			t.Fatalf("SetAISpamEnabled() error = %v", err)
		}
		dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
		LoadAISpam(dispatcher)

		aispamProcessUpdate(t, dispatcher, s.bot, s.chat, noticeGoldenTarget, screenGoldenSpam, "BUY CHEAP COINS NOW t.me/scam")
		waitAISpamChecks()

		deletes := s.client.callsFor("deleteMessage")
		if len(deletes) != 1 || fmt.Sprint(deletes[0].Params["message_id"]) != strconv.Itoa(screenGoldenSpam) {
			t.Fatalf("deleteMessage calls = %+v, want one of the flagged message %d", deletes, screenGoldenSpam)
		}
		if sends := s.client.callsFor("sendMessage"); len(sends) != 0 {
			t.Fatalf("sendMessage calls = %d, want none: a delete verdict posts nothing in the group", len(sends))
		}
		s.wantNoRows(s.chat.Id)
	})

	t.Run("aispam_breaker", func(t *testing.T) {
		s := newScreenGoldenSite(t)

		aispamNotifyBreaker(s.bot, s.chat.Id, "uk")

		sends := s.client.callsFor("sendMessage")
		if len(sends) != 1 {
			t.Fatalf("sendMessage calls = %d, want the one breaker notice", len(sends))
		}
		if got := screenGoldenTokens(legacyRequestJSON(t, sends[0]), s.chat.Id, 0, 0); got != goldenScreenAISpamBreaker {
			t.Fatalf("sendMessage request changed\n got %q\nwant %q", got, goldenScreenAISpamBreaker)
		}
		s.wantRow(s.chat.Id, 9001, deletion.KindBotAuto, 600*time.Second)
	})
}
