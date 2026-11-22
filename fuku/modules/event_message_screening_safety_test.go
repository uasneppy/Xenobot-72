//go:build testtools

package modules

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/captcha"
	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/db/memberreq"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/db/staff"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
)

// The safeguards of the screening sends (Phase 24 plan 24-07): no log line of the captcha, requirement and AI spam sites
// carries content (D-37), and no group message, private copy, Details record or log line of these events names the Staff
// Group, the AI provider or the verdict, and no requirement request carries a bidirectional control from a member's name
// (D-15, D-36). One site per case, the real locales, every private copy job awaited (D-40).

// The sentinels of the log sweep: a stored text, a file ID, a button's name and link, and a message the member wrote
// while the captcha was pending. They carry no digit, so a search for one is unambiguous.
const (
	screenSafeText   = "ZZSCRTEXT {first}"
	screenSafeFile   = "ZZSCRFILE"
	screenSafeButton = "ZZSCRBUTTON"
	screenSafeLink   = "https://zzscr.example/link"
	screenSafeHeld   = "zzscrheldmessage"
)

// screenSafeStore stores a whole custom group message of event for chatID: the sentinel text, a photo with the sentinel
// file ID and the sentinel button.
func screenSafeStore(t *testing.T, chatID int64, event string) {
	t.Helper()
	eventMsgStoreRow(t, chatID, event, screenSafeText, screenSafeFile, db.PHOTO,
		[]models.Button{{Name: screenSafeButton, Url: screenSafeLink}})
}

// screenSafeFailRead makes the engine's read of the stored messages fail for the rest of the test.
func screenSafeFailRead(t *testing.T) {
	t.Helper()
	mreqSwap(t, &eventMessagesLookup, func(context.Context, int64) (*eventmsg.Messages, error) {
		return nil, errors.New("database is down")
	})
}

// screenSafeCaptchaFail seeds a pending attempt of the golden member with one held message holding the sentinel and runs
// it out through expireCaptchaAttempt, which must claim it.
func screenSafeCaptchaFail(t *testing.T, s *botSite) {
	t.Helper()
	attempt, err := captcha.CreateCaptchaAttemptPreMessage(noticeGoldenTarget.Id, s.chat.Id, "7", 2)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
	}
	if err := captcha.StoreMessageForCaptcha(noticeGoldenTarget.Id, s.chat.Id, attempt.ID, db.TEXT, screenSafeHeld, "", ""); err != nil {
		t.Fatalf("StoreMessageForCaptcha() error = %v", err)
	}
	if claimed, err := expireCaptchaAttempt(s.bot, attempt); !claimed || err != nil {
		t.Fatalf("expireCaptchaAttempt() = (%v, %v), want claimed", claimed, err)
	}
}

// screenSafeRefusePhoto is the refusal of a stored photo Telegram cannot send.
var screenSafeRefusePhoto = &gotgbot.TelegramError{Method: "sendPhoto", Code: 400, Description: "Bad Request: wrong file identifier"}

func TestEventMessageScreeningLogsCarryIDsOnly(t *testing.T) {
	withModerationGoldenLocales(t)
	tr := i18n.MustNewTranslator("uk")
	member := noticeGoldenTarget
	names := []string{
		member.FirstName, html.EscapeString(member.FirstName), "<Bo>", member.Username,
	}
	content := []string{"ZZSCRTEXT", screenSafeFile, screenSafeButton, "zzscr.example", screenSafeHeld}
	forbidden := func(extra ...string) []string {
		return append(append(append([]string{}, content...), names...), extra...)
	}

	// The captcha: a failed read sends the default, a refused custom photo is replaced by the default; neither logs the
	// stored message, the held message, the member's names or the captcha reason.
	t.Run("captcha read error", func(t *testing.T) {
		hook := mreqLogCapture(t)
		s := eventMsgCaptchaSite(t)
		screenSafeStore(t, s.chat.Id, eventmsg.EventCaptcha)
		screenSafeFailRead(t)
		screenSafeCaptchaFail(t, s)
		if sends := s.client.callsFor("sendMessage"); len(sends) != 1 {
			t.Fatalf("sendMessage calls = %d, want the one default", len(sends))
		}
		mreqWantCleanLogs(t, hook, forbidden(captchaEventReason(tr)),
			fmt.Sprintf("messages of chat %d could not be read for %s", s.chat.Id, eventmsg.EventCaptcha))
	})
	t.Run("captcha refused", func(t *testing.T) {
		hook := mreqLogCapture(t)
		s := eventMsgCaptchaSite(t)
		screenSafeStore(t, s.chat.Id, eventmsg.EventCaptcha)
		eventMsgRefuseOnce(s, "sendPhoto", screenSafeRefusePhoto)
		screenSafeCaptchaFail(t, s)
		if photos, sends := s.client.callsFor("sendPhoto"), s.client.callsFor("sendMessage"); len(photos) != 1 || len(sends) != 1 {
			t.Fatalf("sendPhoto calls = %d and sendMessage calls = %d, want the refused custom notice and the default", len(photos), len(sends))
		}
		mreqWantCleanLogs(t, hook, forbidden(captchaEventReason(tr)),
			fmt.Sprintf("custom %s message of chat %d was refused", eventmsg.EventCaptcha, s.chat.Id))
	})

	// The requirement kick: the same two shapes; the rule phrase (the reason) never reaches a line. The requirement notice
	// is text only, so the custom message refused is its text, refused with its one retry before the default goes.
	mreqRun := func(t *testing.T, prepare func(env *mreqSchedEnv)) (*mreqSchedEnv, []string) {
		t.Helper()
		env := newMreqEventEnv(t)
		env.mreqRules(map[memberreq.Rule]int{memberreq.RuleLastName: models.RequirementActionKick})
		screenSafeStore(t, env.chat.Id, eventmsg.EventReqKick)
		prepare(env)
		target := screenGoldenReqMember(env)
		if got := env.mreqEnforce(env.mreqCase(target)); got.Outcome != memberReqRemoved {
			t.Fatalf("enforcement = %+v, want Removed", got)
		}
		phrase := mreqEventWord(t, env.translator(), "mreq_rule_lastname")
		return env, forbidden(phrase, "mreq_rule_", "mreq_notice")
	}
	t.Run("requirement read error", func(t *testing.T) {
		hook := mreqLogCapture(t)
		env, bad := mreqRun(t, func(*mreqSchedEnv) { screenSafeFailRead(t) })
		if notices := env.mreqNotices(); len(notices) != 1 {
			t.Fatalf("notices = %d, want the one default", len(notices))
		}
		mreqWantCleanLogs(t, hook, bad,
			fmt.Sprintf("messages of chat %d could not be read for %s", env.chat.Id, eventmsg.EventReqKick))
	})
	t.Run("requirement refused", func(t *testing.T) {
		hook := mreqLogCapture(t)
		env, bad := mreqRun(t, func(env *mreqSchedEnv) { env.fake.script("sendMessage", env.chat.Id, mreqRefusals(2)...) })
		if sends := env.calls("sendMessage"); len(sends) != 3 {
			t.Fatalf("sendMessage calls = %d, want the refused custom notice, its retry and the default", len(sends))
		}
		mreqWantCleanLogs(t, hook, bad,
			fmt.Sprintf("custom %s message of chat %d was refused", eventmsg.EventReqKick, env.chat.Id))
	})

	// The AI spam notice is off by default, so a failed read posts nothing; a refused custom photo is replaced by the
	// default. Neither logs the stored message, the spam message, the member's names or the AI reason.
	aiForbidden := forbidden(aispamEventReason(tr), "BUY CHEAP COINS")
	t.Run("ai spam read error", func(t *testing.T) {
		hook := mreqLogCapture(t)
		e := newAISpamNoticeEnv(t)
		screenSafeStore(t, e.chat.Id, eventmsg.EventAISpam)
		e.turnOn(t)
		screenSafeFailRead(t)
		e.wantDeleted(t, e.spam(t, member, 0))
		if notices := e.notices(t); len(notices) != 0 {
			t.Fatalf("notices = %d, want none: an off-by-default event posts nothing when its switch is unreadable", len(notices))
		}
		mreqWantCleanLogs(t, hook, aiForbidden,
			fmt.Sprintf("messages of chat %d could not be read for %s, nothing was posted", e.chat.Id, eventmsg.EventAISpam))
	})
	t.Run("ai spam refused", func(t *testing.T) {
		hook := mreqLogCapture(t)
		e := newAISpamNoticeEnv(t)
		screenSafeStore(t, e.chat.Id, eventmsg.EventAISpam)
		e.turnOn(t)
		eventMsgRefuseOnce(e.botSite, "sendPhoto", screenSafeRefusePhoto)
		e.wantDeleted(t, e.spam(t, member, 0))
		notices := e.notices(t)
		if len(notices) != 2 || notices[0].Method != "sendPhoto" || notices[1].Method != "sendMessage" {
			t.Fatalf("group sends = %+v, want the refused custom photo and the default", notices)
		}
		mreqWantCleanLogs(t, hook, aiForbidden,
			fmt.Sprintf("custom %s message of chat %d was refused", eventmsg.EventAISpam, e.chat.Id))
	})
}

// The sentinels of the privacy run: the Staff Group's title, the verdict's category and model and its probability. They
// carry no digit except the probability, whose text is searched as it would print.
const (
	screenSafeStaffTitle  = "ZZ Scr Staff & <Sentinel>"
	screenSafeCategory    = "zzscrcategory"
	screenSafeModel       = "zzscrmodel"
	screenSafeProbability = 0.9731
)

// screenSafeLinkStaff makes a Staff Group titled screenSafeStaffTitle and links groupChatID to it; both rows go when the
// test ends. It returns the Staff Group's chat ID.
func screenSafeLinkStaff(t *testing.T, groupChatID int64) int64 {
	t.Helper()
	staffChat, owner := uniqueModuleChatID(), uniqueLinkOwnerID()
	staffCleanup(t, staffChat, groupChatID)
	if _, err := staff.CreateStaffGroup(staffChat, owner, screenSafeStaffTitle); err != nil {
		t.Fatalf("create the Staff Group: %v", err)
	}
	if err := staff.CreateLink(&models.StaffGroupLink{GroupChatID: groupChatID, StaffChatID: staffChat, OwnerUserID: owner, GroupTitle: "Linked"}); err != nil {
		t.Fatalf("link the group: %v", err)
	}
	return staffChat
}

// screenSafeStaffLeaks are the texts that would name the Staff Group.
func screenSafeStaffLeaks(staffChat int64) []string {
	return []string{
		screenSafeStaffTitle, html.EscapeString(screenSafeStaffTitle), "ZZ Scr Staff",
		strings.TrimPrefix(strconv.FormatInt(staffChat, 10), "-"),
	}
}

// screenSafeRequests is every send and edit the client recorded, as JSON (reads are not messages), after the
// asynchronous posts.
func screenSafeRequests(t *testing.T, client *moduleBotClient) []string {
	t.Helper()
	client.callsFor("") // waits for the asynchronous posts, as every read of the fake does
	client.mu.Lock()
	defer client.mu.Unlock()
	var out []string
	for _, call := range client.calls {
		if !strings.HasPrefix(call.Method, "send") && !strings.HasPrefix(call.Method, "edit") {
			continue
		}
		raw, err := json.Marshal(call.Params)
		if err != nil {
			t.Fatalf("encode a %s request: %v", call.Method, err)
		}
		out = append(out, call.Method+" "+string(raw))
	}
	return out
}

// screenSafeWantClean fails when text holds any of leaks (each compared without regard to case).
func screenSafeWantClean(t *testing.T, label, text string, leaks []string) {
	t.Helper()
	lower := strings.ToLower(text)
	for _, leak := range leaks {
		if strings.Contains(lower, strings.ToLower(leak)) {
			t.Errorf("%s carries %q: %s", label, leak, text)
		}
	}
}

// screenSafeRequirementText is a custom requirement text with every Welcome and event placeholder a requirement event
// fills (no {rules}, which would create a rules row outside this test's cleanup).
const screenSafeRequirementText = "{first}|{last}|{fullname}|{username}|{mention}|{id}|{chatname}|" +
	"{admin}|{reason}|{action}|{count}|{limit}|{window}|{duration}|{until}"

func TestEventMessageScreeningPrivacy(t *testing.T) {
	withModerationGoldenLocales(t)

	for _, custom := range []bool{true, false} {
		name := "defaults"
		if custom {
			name = "custom"
		}

		// The captcha in a group linked to a Staff Group, both private copy options on and the member refusing private
		// messages: the group notice, the Details edit and record and the #PM_COPY line exist, and none of them, nor any
		// log line, names the Staff Group.
		t.Run("captcha "+name, func(t *testing.T) {
			s := newEventPMCaptchaSite(t)
			hook := mreqLogCapture(t)
			staffChat := screenSafeLinkStaff(t, s.chat.Id)
			channel := eventPMLogLink(t, s)
			if custom {
				eventMsgStore(t, s.chat.Id, eventmsg.EventCaptcha, eventMsgAutoEveryPlaceholder)
				storeEventPM(t, s.chat.Id, eventmsg.EventCaptcha, eventMsgAutoEveryPlaceholder, true)
			}
			eventPMDetailsOn(t, s.chat.Id)
			eventPMLogOn(t, s.chat.Id)
			eventPMRefuse(t, s, &gotgbot.TelegramError{Method: "sendMessage", Code: 403, Description: "Forbidden: bot was blocked by the user"})

			eventPMCaptchaFail(t, s, noticeGoldenTarget.Id)
			eventPMCaptchaKicked(t, s, noticeGoldenTarget.Id)
			if group, private := eventPMCalls(t, s); len(group) != 1 || len(private) != 1 {
				t.Fatalf("group sends = %d, private sends = %d, want the notice and the one refused copy", len(group), len(private))
			}
			if edits := eventPMDetailsEdits(t, s); len(edits) != 1 {
				t.Fatalf("Details edits = %d, want one", len(edits))
			}
			if posts := eventPMLogPosts(t, s, channel); len(posts) != 1 {
				t.Fatalf("#PM_COPY posts = %d, want one", len(posts))
			}
			tokens := eventPMDetailsTokens(t)
			if len(tokens) != 1 {
				t.Fatalf("Details records = %d, want one", len(tokens))
			}
			rec, _ := eventPMDetailsRecord(t, tokens[0])
			raw, err := json.Marshal(rec)
			if err != nil {
				t.Fatalf("encode the Details record: %v", err)
			}
			if rec.UserID != noticeGoldenTarget.Id || rec.Text == "" {
				t.Fatalf("the Details record %s is not the member's copy", raw)
			}

			leaks := screenSafeStaffLeaks(staffChat)
			for _, request := range screenSafeRequests(t, s.client) {
				screenSafeWantClean(t, "a request", request, leaks)
			}
			screenSafeWantClean(t, "the Details record", string(raw), leaks)
			for _, line := range mreqLogLines(hook) {
				screenSafeWantClean(t, "a log line", line, leaks)
			}
		})

		// The AI spam notice and its private copy, the verdict carrying a sentinel category and model: no Telegram request
		// and no log line of the notice names the provider, the model, the category or the probability. The verdict line
		// of /aispam itself is the operator's record of the verdict and keeps them (the trust boundary keeps the verdict's
		// details in the logs); it is the one line exempt, and it must carry the sentinel, which proves the sentinel was
		// in flight.
		t.Run("ai spam "+name, func(t *testing.T) {
			e := newAISpamNoticeEnv(t)
			hook := mreqLogCapture(t)
			body := strings.Replace(aispamDecisionBody(screenSafeProbability, screenSafeCategory), "jev-1.13.0", screenSafeModel, 1)
			withAISpamJevServer(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprint(w, body) })
			staffChat := screenSafeLinkStaff(t, e.chat.Id)
			if custom {
				eventMsgStore(t, e.chat.Id, eventmsg.EventAISpam, eventMsgAutoEveryPlaceholder)
				storeEventPM(t, e.chat.Id, eventmsg.EventAISpam, eventMsgAutoEveryPlaceholder, true)
			} else {
				storeEventPM(t, e.chat.Id, eventmsg.EventAISpam, "", true)
			}
			e.turnOn(t)

			e.wantDeleted(t, e.spam(t, noticeGoldenTarget, 0))
			group, private := eventPMCalls(t, e.botSite)
			if len(group) != 1 || len(private) != 1 {
				t.Fatalf("group sends = %d, private sends = %d, want the notice and its private copy", len(group), len(private))
			}

			leaks := append(screenSafeStaffLeaks(staffChat), screenSafeCategory, screenSafeModel, "typesafe",
				strconv.FormatFloat(screenSafeProbability, 'f', -1, 64), "97.31", "97%")
			for _, request := range screenSafeRequests(t, e.client) {
				screenSafeWantClean(t, "a request", request, leaks)
			}
			verdict := false
			for _, line := range mreqLogLines(hook) {
				if strings.HasPrefix(line, "[AISpam] verdict") {
					verdict = verdict || strings.Contains(line, screenSafeCategory)
					continue
				}
				screenSafeWantClean(t, "a log line", line, leaks)
			}
			if !verdict {
				t.Errorf("no verdict line carries the sentinel category, so the run did not judge with it")
			}
		})

		// A requirement mute in a group linked to a Staff Group, for a member whose first name holds U+202E and U+200F and
		// who has no username (so {mention} is the name): no group notice, private copy or log line carries either control
		// or names the Staff Group. The default mention's own isolate (U+2068 ... U+2069) is the requirement mention's
		// design and is not searched.
		t.Run("requirement "+name, func(t *testing.T) {
			env := newMreqEventEnv(t)
			hook := mreqLogCapture(t)
			staffChat := screenSafeLinkStaff(t, env.chat.Id)
			env.mreqRules(map[memberreq.Rule]int{memberreq.RuleLastName: models.RequirementActionMute})
			if custom {
				eventMsgStore(t, env.chat.Id, eventmsg.EventReqMute, screenSafeRequirementText)
				storeEventPM(t, env.chat.Id, eventmsg.EventReqMute, screenSafeRequirementText, true)
			} else {
				storeEventPM(t, env.chat.Id, eventmsg.EventReqMute, "", true)
			}
			target := env.mreqMember("ZZRTL‮evil‏", "", "")

			if got := env.mreqEnforce(env.mreqCase(target)); got.Outcome != memberReqMuted {
				t.Fatalf("enforcement = %+v, want Muted", got)
			}
			waitEventPM(t)
			notice := mreqEventOnlyNotice(t, env)
			if text := fmt.Sprint(notice.Params["text"]); !strings.Contains(text, "ZZRTL") {
				t.Fatalf("the notice %q does not name the member", text)
			}
			private := env.fake.sentTo(target.Id)
			if len(private) != 1 {
				t.Fatalf("private copies = %d, want one", len(private))
			}
			if names := strings.Contains(fmt.Sprint(private[0].Params["text"]), "ZZRTL"); names != custom {
				t.Fatalf("the private copy %q names the member %v, want %v (only the custom text names them)", private[0].Params["text"], names, custom)
			}

			leaks := append(screenSafeStaffLeaks(staffChat), "‮", "‏")
			for _, request := range screenSafeRequests(t, env.fake.moduleBotClient) {
				screenSafeWantClean(t, "a request", request, leaks)
			}
			for _, line := range mreqLogLines(hook) {
				screenSafeWantClean(t, "a log line", line, leaks)
			}
		})
	}
}

// TestEventMessageMemberReqMuteWaitsOutRateLimit closes the gap plan 24-05 left (D-19): a custom requirement mute notice
// that the requirement pacer gives up on (four 429 answers, ratelimit.ErrRateLimited) is sent again once the block passes,
// still the custom text with its "I fixed it" row, and the mute is never lifted, exactly as TestMemberReqMuteNoticeRetry
// proves for the default.
func TestEventMessageMemberReqMuteWaitsOutRateLimit(t *testing.T) {
	mreqShortenNoticeWait(t)
	env := newMreqEventEnv(t)
	tr := env.translator()
	env.mreqRules(map[memberreq.Rule]int{memberreq.RuleLastName: models.RequirementActionMute})
	eventMsgStoreRow(t, env.chat.Id, eventmsg.EventReqMute, mreqEventMuteText, "", 0, []models.Button{eventMsgSiteButton})
	env.fake.script("sendMessage", env.chat.Id, mreqRateLimited()...)
	member := screenGoldenReqMember(env)

	got := env.mreqEnforce(env.mreqCase(member))
	if got.Outcome != memberReqMuted || got.Hold == nil {
		t.Fatalf("enforcement = %+v, want Muted with a hold", got)
	}
	want := "<b>@ann_lee</b> " + mreqEventWord(t, tr, "eventmsg_action_mute") + ": " +
		mreqEventWord(t, tr, "mreq_rule_lastname") + mreqEventNote(t, tr)
	sends := env.calls("sendMessage")
	if len(sends) != 5 {
		t.Fatalf("sendMessage calls = %d, want the four the pacer gave up on and one more", len(sends))
	}
	for i, send := range sends {
		if text := fmt.Sprint(send.Params["text"]); text != want {
			t.Errorf("send %d carried %q, want the custom notice every time (a rate limit is not a refusal)", i, text)
		}
	}
	mreqEventWantNotice(t, env, mreqEventOnlyNotice(t, env), member, want, mreqEventMuteRows(tr, got.Hold, member))
	if n := len(env.calls("restrictChatMember")); n != 1 {
		t.Errorf("restrictChatMember calls = %d, want only the mute (nothing lifted)", n)
	}
	if hold := env.mreqHold(member.Id); hold == nil || hold.PunishedAt == nil {
		t.Errorf("hold = %+v, want punished", hold)
	}
}
