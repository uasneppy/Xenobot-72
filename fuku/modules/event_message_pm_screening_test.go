//go:build testtools

package modules

import (
	"html"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/fuku_robot/fuku/db/captcha"
	dbcache "github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/chats"
	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
)

// The private copies of the screening events (Phase 24 plan 24-03, D-34): every one is automatic, so the
// per-group cap of automatic copies bounds them. The captcha is the one screening site already switched to
// the engine (plan 24-02), so its copy is proved here through handleCaptchaTimeout; the requirement and AI
// spam copies are proved by their own site plans.

// newEventPMCaptchaSite is the captcha event site (captcha on with the kick action, every bot kind at 60 s,
// the golden member 42 answered by getChatMember) with miniredis, a fast private copy pacer, a client that
// answers the member's private chat, the group's stored title set to eventPMAutoTitle (the captcha site reads
// it from the chats row) and the captcha's private copy switched on with nothing stored. The event messages,
// the private copy keys and the chats row are removed when the test ends, after the jobs.
func newEventPMCaptchaSite(t *testing.T) *botSite {
	t.Helper()
	withMiniredisServer(t)
	withFastPMCopyPacer(t)
	s := eventMsgCaptchaSite(t)
	s.bot.BotClient = eventPMTestClient{BotClient: s.bot.BotClient, script: &eventPMScript{}}
	eventPMCleanup(t, s.chat.Id)
	t.Cleanup(pmCopyJobs.wait)
	chatID := s.chat.Id
	if err := chats.EnsureChatInDb(chatID, eventPMAutoTitle); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		dbcache.DeleteCache(dbcache.CacheKey("chat", chatID))
		dbcache.ResetLocalForTest()
	})
	storeEventPM(t, chatID, eventmsg.EventCaptcha, "", true)
	return s
}

// eventPMCaptchaFail seeds a pending attempt of member and runs handleCaptchaTimeout with the kick action,
// which must claim the attempt and apply the kick.
func eventPMCaptchaFail(t *testing.T, s *botSite, member int64) {
	t.Helper()
	attempt, err := captcha.CreateCaptchaAttemptPreMessage(member, s.chat.Id, "7", 2)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage(%d) error = %v", member, err)
	}
	claimed, applied := handleCaptchaTimeout(s.bot, s.chat.Id, member, attempt.ID, screenGoldenChallenge, "kick")
	if !claimed || !applied {
		t.Fatalf("handleCaptchaTimeout(%d) = (%v, %v), want claimed and applied", member, claimed, applied)
	}
}

// eventPMCaptchaKicked fails unless every member was kicked exactly once.
func eventPMCaptchaKicked(t *testing.T, s *botSite, members ...int64) {
	t.Helper()
	for _, member := range members {
		if got := kicksOf(s.client, member); got != 1 {
			t.Fatalf("kicks of %d = %d, want one: every member is punished", member, got)
		}
	}
}

func TestEventPMCaptcha(t *testing.T) {
	withModerationGoldenLocales(t)
	tr := i18n.MustNewTranslator("uk")

	// delivered: the member is kicked, the group notice is posted, then the member receives the default
	// captcha text in the group's language with the stored title and the kick word, then the reason line.
	t.Run("delivered", func(t *testing.T) {
		s := newEventPMCaptchaSite(t)
		eventPMCaptchaFail(t, s, noticeGoldenTarget.Id)
		eventPMCaptchaKicked(t, s, noticeGoldenTarget.Id)
		want := eventPMAutoDefault(t, "eventmsg_pm_captcha", i18n.TranslationParams{
			"group": eventPMAutoGroup, "action": eventActionWord(tr, "kick"),
		}, html.EscapeString(captchaEventReason(tr)))
		eventPMAutoWant(t, s, want)
		if group, _ := eventPMCalls(t, s); len(group) != 1 {
			t.Fatalf("group notices = %d, want the one captcha notice", len(group))
		}
	})

	// blocked: the member refuses private messages; the kick and the notice stand, one private send is
	// attempted and nothing else follows (no Details edit, no log post: both options are off).
	t.Run("blocked", func(t *testing.T) {
		s := newEventPMCaptchaSite(t)
		eventPMRefuse(t, s, &gotgbot.TelegramError{Method: "sendMessage", Code: 403, Description: "Forbidden: bot was blocked by the user"})
		eventPMCaptchaFail(t, s, noticeGoldenTarget.Id)
		eventPMCaptchaKicked(t, s, noticeGoldenTarget.Id)
		group, private := eventPMCalls(t, s)
		if len(group) != 1 || len(private) != 1 {
			t.Fatalf("group sends = %d, private sends = %d, want the notice and one attempted copy", len(group), len(private))
		}
		if private[0].Index < group[0].Index {
			t.Fatal("the private copy was attempted before the group notice")
		}
		if edits := s.client.callsFor("editMessageReplyMarkup"); len(edits) != 0 {
			t.Fatalf("Details edits = %d, want none", len(edits))
		}
	})

	// cap: three members fail in one minute with the cap at 2; every one is kicked and noticed and two copies
	// are sent. In the next minute, with the cap's Redis client taken away, a fourth member is kicked and
	// noticed and no copy is sent (fail closed).
	t.Run("cap", func(t *testing.T) {
		s := newEventPMCaptchaSite(t)
		var minutes atomic.Int64
		base := time.Date(2026, 11, 21, 12, 0, 5, 0, time.UTC)
		mreqSwap(t, &pmCopyNow, func() time.Time { return base.Add(time.Duration(minutes.Load()) * time.Minute) })
		mreqSwap(t, &pmCopyAutoCapPerMinute, 2)
		t.Cleanup(pmCopyJobs.wait)

		members := eventPMUsers(3)
		for _, member := range members {
			eventPMCaptchaFail(t, s, member)
		}
		eventPMCaptchaKicked(t, s, members...)
		if group, _ := eventPMCalls(t, s); len(group) != len(members) {
			t.Fatalf("group notices = %d, want one per member (%d)", len(group), len(members))
		}
		if n := eventPMSentTo(t, s, members); n != 2 {
			t.Fatalf("private copies sent in one minute = %d, want the cap of 2", n)
		}

		minutes.Store(1)
		eventPMAutoNoRedis(t)
		fourth := eventPMUsers(1)
		eventPMCaptchaFail(t, s, fourth[0])
		eventPMCaptchaKicked(t, s, fourth...)
		if group, _ := eventPMCalls(t, s); len(group) != len(members)+1 {
			t.Fatalf("group notices = %d, want one per member (%d)", len(group), len(members)+1)
		}
		if n := eventPMSentTo(t, s, fourth); n != 0 {
			t.Fatalf("private copies without Redis = %d, want none (fail closed)", n)
		}
	})
}
