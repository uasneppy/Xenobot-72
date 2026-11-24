//go:build testtools

package modules

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/deletion"
	"github.com/uasneppy/Xenobot-72/xenobot/db/eventmsg"
	"github.com/uasneppy/Xenobot-72/xenobot/db/memberreq"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/db/modtuning"
	"github.com/uasneppy/Xenobot-72/xenobot/db/warns"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
)

// The requirement site of the event engine (plan 24-05): a group's custom requirement notice, the
// requirements line after it, the "I fixed it" button kept below the custom buttons, the 24 hour cap
// and the private copy. Each subtest runs one requirement environment with the real locales (the
// group's language is Ukrainian) and the last-name rule only; the remedies are proved purely in
// TestScreeningEventDefaults and TestMemberReqNoticeNoteFitsReserve.

// mreqEventWarnText is a custom requirement text with every placeholder the requirement events fill.
const mreqEventWarnText = "<b>{mention}</b> {action} {count}/{limit}: {reason}"

// mreqEventMuteText is a custom requirement mute text.
const mreqEventMuteText = "<b>{mention}</b> {action}: {reason}"

// newMreqEventEnv is the screening golden requirement environment (real locales, the deletion clock
// fixed, a 600 s bot:punish_auto rule) with a fast private copy pacer; the chat's event messages and the
// private copy keys are removed when the test ends, after every private copy job.
func newMreqEventEnv(t *testing.T) *mreqSchedEnv {
	t.Helper()
	withFastPMCopyPacer(t)
	env := newScreenGoldenMreqEnv(t)
	eventPMCleanup(t, env.chat.Id)
	t.Cleanup(pmCopyJobs.wait)
	return env
}

// mreqEventWord is the translated word of a key in the group's language.
func mreqEventWord(t *testing.T, tr *i18n.Translator, key string, params ...i18n.TranslationParams) string {
	t.Helper()
	text, err := tr.GetString(key, params...)
	if err != nil || text == "" {
		t.Fatalf("translate %s: %q, %v", key, text, err)
	}
	return text
}

// mreqEventNote is what follows a custom text for a broken last name: a blank line, then the
// requirements line naming the rule (the last-name rule carries no remedy).
func mreqEventNote(t *testing.T, tr *i18n.Translator) string {
	t.Helper()
	phrase := mreqEventWord(t, tr, "mreq_rule_lastname")
	return "\n\n" + mreqEventWord(t, tr, "eventmsg_mreq_rules", i18n.TranslationParams{"rules": phrase})
}

// mreqEventOrder is the index of the first recorded call of method addressed to chatID (any chat when
// chatID is 0) among every call of the fake, or -1.
func mreqEventOrder(env *mreqSchedEnv, method string, chatID int64) int {
	env.fake.callsFor("") // waits for the asynchronous posts, as every read of the fake does
	env.fake.mu.Lock()
	defer env.fake.mu.Unlock()
	for i, call := range env.fake.calls {
		if call.Method == method && (chatID == 0 || fmt.Sprint(call.Params["chat_id"]) == strconv.FormatInt(chatID, 10)) {
			return i
		}
	}
	return -1
}

// mreqEventOnlyNotice fails unless exactly one message reached the group and returns it.
func mreqEventOnlyNotice(t *testing.T, env *mreqSchedEnv) staffSentMessage {
	t.Helper()
	notices := env.mreqNotices()
	if len(notices) != 1 {
		t.Fatalf("messages posted in the group = %d, want the one notice", len(notices))
	}
	return notices[0]
}

// mreqEventWantNotice checks the posted notice's text and keyboard, that it replies to nothing and has
// no topic, and that the hold stored its message ID.
func mreqEventWantNotice(t *testing.T, env *mreqSchedEnv, notice staffSentMessage, member gotgbot.User, text string, rows [][]gotgbot.InlineKeyboardButton) {
	t.Helper()
	if got := fmt.Sprint(notice.Params["text"]); got != text {
		t.Errorf("notice text = %q, want %q", got, text)
	}
	got := staffKeyboardOf(notice.Params["reply_markup"])
	if fmt.Sprint(got) != fmt.Sprint(rows) {
		t.Errorf("notice keyboard = %+v, want %+v", got, rows)
	}
	for _, key := range []string{"reply_parameters", "message_thread_id"} {
		if value, ok := notice.Params[key]; ok && fmt.Sprint(value) != "0" {
			t.Errorf("notice %s = %v, want none (a new message in the group)", key, value)
		}
	}
	if hold := env.mreqHold(member.Id); hold == nil || hold.NoticeMsgID != notice.MessageID {
		t.Errorf("hold = %+v, want its notice ID %d", hold, notice.MessageID)
	}
}

// mreqEventSiteRows is the custom button row a stored message with eventMsgSiteButton composes.
func mreqEventSiteRows() [][]gotgbot.InlineKeyboardButton {
	return [][]gotgbot.InlineKeyboardButton{{{Text: eventMsgSiteButton.Name, Url: eventMsgSiteButton.Url}}}
}

// mreqEventWantPM fails unless exactly one message reached the member's private chat, after the group
// notice, carrying text and no keyboard.
func mreqEventWantPM(t *testing.T, env *mreqSchedEnv, member gotgbot.User, text string) {
	t.Helper()
	waitEventPM(t)
	private := env.fake.sentTo(member.Id)
	if len(private) != 1 {
		t.Fatalf("private messages to the member = %d, want the one private copy", len(private))
	}
	if got := fmt.Sprint(private[0].Params["text"]); got != text {
		t.Errorf("private copy = %q, want %q", got, text)
	}
	if rows := staffKeyboardOf(private[0].Params["reply_markup"]); len(rows) != 0 {
		t.Errorf("private copy keyboard = %+v, want none (the button is a group callback)", rows)
	}
	if group, pm := mreqEventOrder(env, "sendMessage", env.chat.Id), mreqEventOrder(env, "sendMessage", member.Id); group < 0 || pm < group {
		t.Errorf("the private copy (call %d) came before the group notice (call %d)", pm, group)
	}
}

// mreqEventPMText is the default private text of event in the group's language with the reason line
// naming the last-name rule.
func mreqEventPMText(t *testing.T, env *mreqSchedEnv, tr *i18n.Translator, key string) string {
	t.Helper()
	text := mreqEventWord(t, tr, key, i18n.TranslationParams{"group": staffDisplayTitle(env.chat.Title)})
	return text + "\n" + mreqEventWord(t, tr, "eventmsg_pm_reason", i18n.TranslationParams{"reason": mreqEventWord(t, tr, "mreq_rule_lastname")})
}

func TestEventMessageMemberReq(t *testing.T) {
	t.Run("warn custom", func(t *testing.T) {
		env := newMreqEventEnv(t)
		tr := env.translator()
		env.mreqRules(map[memberreq.Rule]int{memberreq.RuleLastName: models.RequirementActionWarn})
		eventMsgStoreRow(t, env.chat.Id, eventmsg.EventReqWarn, mreqEventWarnText, "", 0, []models.Button{eventMsgSiteButton})
		member := screenGoldenReqMember(env)

		if got := env.mreqEnforce(env.mreqCase(member)); got.Outcome != memberReqWarned {
			t.Fatalf("enforcement = %+v, want Warned", got)
		}

		if n, _ := warns.GetWarns(member.Id, env.chat.Id); n != 1 {
			t.Fatalf("stored warns = %d, want 1", n)
		}
		notice := mreqEventOnlyNotice(t, env)
		want := "<b>@ann_lee</b> " + mreqEventWord(t, tr, "eventmsg_action_warn") + " 1/3: " +
			mreqEventWord(t, tr, "mreq_rule_lastname") + mreqEventNote(t, tr)
		mreqEventWantNotice(t, env, notice, member, want, mreqEventSiteRows())
		env.wantRowFor(notice.MessageID, deletion.KindBotAuto, env.now().Add(600*time.Second))
	})

	t.Run("warn limit custom", func(t *testing.T) {
		env := newMreqEventEnv(t)
		tr := env.translator()
		env.mreqRules(map[memberreq.Rule]int{memberreq.RuleLastName: models.RequirementActionWarn})
		if err := warns.SetWarnLimit(env.chat.Id, 1); err != nil {
			t.Fatalf("SetWarnLimit() error = %v", err)
		}
		if err := warns.SetWarnMode(env.chat.Id, "ban"); err != nil {
			t.Fatalf("SetWarnMode() error = %v", err)
		}
		eventMsgStore(t, env.chat.Id, eventmsg.EventReqWarn, mreqEventWarnText)
		member := screenGoldenReqMember(env)

		if got := env.mreqEnforce(env.mreqCase(member)); got.Outcome != memberReqRemoved {
			t.Fatalf("enforcement = %+v, want Removed (the limit's ban)", got)
		}

		screenGoldenWantAction(t, env.calls, "banChatMember", member.Id)
		if ban, send := mreqEventOrder(env, "banChatMember", env.chat.Id), mreqEventOrder(env, "sendMessage", env.chat.Id); ban < 0 || send < ban {
			t.Errorf("the notice (call %d) came before the ban (call %d)", send, ban)
		}
		notice := mreqEventOnlyNotice(t, env)
		want := "<b>@ann_lee</b> " + mreqEventWord(t, tr, "eventmsg_action_ban") + " 1/1: " +
			mreqEventWord(t, tr, "mreq_rule_lastname") + mreqEventNote(t, tr)
		mreqEventWantNotice(t, env, notice, member, want, nil)
		env.wantRowFor(notice.MessageID, deletion.KindBotAuto, env.now().Add(600*time.Second))
	})

	t.Run("kick silent", func(t *testing.T) {
		env := newMreqEventEnv(t)
		env.mreqRules(map[memberreq.Rule]int{memberreq.RuleLastName: models.RequirementActionKick})
		eventMsgCleanup(t, env.chat.Id)
		if err := eventmsg.SetSilent(t.Context(), env.chat.Id, eventmsg.EventReqKick, eventmsg.DestGroup, true, 1); err != nil {
			t.Fatalf("SetSilent error = %v", err)
		}
		member := screenGoldenReqMember(env)

		if got := env.mreqEnforce(env.mreqCase(member)); got.Outcome != memberReqRemoved {
			t.Fatalf("enforcement = %+v, want Removed", got)
		}

		screenGoldenWantAction(t, env.calls, "unbanChatMember", member.Id)
		if notices := env.mreqNotices(); len(notices) != 0 {
			t.Fatalf("messages posted in the group = %+v, want none (Silent)", notices)
		}
		env.wantNoRows()
		hold := env.mreqHold(member.Id)
		if hold == nil || hold.PunishedAt == nil || hold.NoticeMsgID != 0 {
			t.Fatalf("hold = %+v, want punished with no notice ID", hold)
		}
	})

	t.Run("refused", func(t *testing.T) {
		env := newMreqEventEnv(t)
		env.mreqRules(map[memberreq.Rule]int{memberreq.RuleLastName: models.RequirementActionKick})
		eventMsgStore(t, env.chat.Id, eventmsg.EventReqKick, mreqEventWarnText)
		// The custom send and its one retry are refused; the default goes through.
		env.fake.script("sendMessage", env.chat.Id, mreqRefusals(2)...)
		member := screenGoldenReqMember(env)

		got := env.mreqEnforce(env.mreqCase(member))
		if got.Outcome != memberReqRemoved || got.Hold == nil {
			t.Fatalf("enforcement = %+v, want Removed with a hold", got)
		}

		screenGoldenWantAction(t, env.calls, "unbanChatMember", member.Id)
		if kick, send := mreqEventOrder(env, "unbanChatMember", env.chat.Id), mreqEventOrder(env, "sendMessage", env.chat.Id); kick < 0 || send < kick {
			t.Errorf("the first send (call %d) came before the kick (call %d)", send, kick)
		}
		if sends := env.calls("sendMessage"); len(sends) != 3 {
			t.Fatalf("sendMessage calls = %d, want the refused custom notice, its retry and the default", len(sends))
		}
		notice := mreqEventOnlyNotice(t, env)
		raw := legacyRequestJSON(t, moduleBotCall{Method: "sendMessage", Params: notice.Params})
		if tokens := screenGoldenTokens(raw, env.chat.Id, member.Id, got.Hold.ID); tokens != goldenScreenReqKick {
			t.Fatalf("default after the refusal\n got %q\nwant %q", tokens, goldenScreenReqKick)
		}
		env.wantRowFor(notice.MessageID, deletion.KindBotAuto, env.now().Add(600*time.Second))
		if hold := env.mreqHold(member.Id); hold == nil || hold.NoticeMsgID != notice.MessageID {
			t.Errorf("hold = %+v, want the default's notice ID %d", hold, notice.MessageID)
		}
	})

	t.Run("private copy", func(t *testing.T) {
		env := newMreqEventEnv(t)
		tr := env.translator()
		env.mreqRules(map[memberreq.Rule]int{memberreq.RuleLastName: models.RequirementActionBan})
		storeEventPM(t, env.chat.Id, eventmsg.EventReqBan, "", true)
		member := screenGoldenReqMember(env)

		if got := env.mreqEnforce(env.mreqCase(member)); got.Outcome != memberReqRemoved {
			t.Fatalf("enforcement = %+v, want Removed", got)
		}

		screenGoldenWantAction(t, env.calls, "banChatMember", member.Id)
		notice := mreqEventOnlyNotice(t, env)
		if ban, send := mreqEventOrder(env, "banChatMember", env.chat.Id), mreqEventOrder(env, "sendMessage", env.chat.Id); ban < 0 || send < ban {
			t.Errorf("the group notice (call %d) came before the ban (call %d)", send, ban)
		}
		if got := fmt.Sprint(notice.Params["text"]); got == "" {
			t.Fatal("the group notice has no text")
		}
		mreqEventWantPM(t, env, member, mreqEventPMText(t, env, tr, "eventmsg_pm_mreqban"))
	})
}

// mreqEventMuteRows is the custom button row then the full-width "I fixed it" row of the member's hold.
func mreqEventMuteRows(tr *i18n.Translator, hold *models.ChatRequirementHold, member gotgbot.User) [][]gotgbot.InlineKeyboardButton {
	rows := mreqEventSiteRows()
	return append(rows, memberReqFixedButton(tr, hold.ID, member.Id).InlineKeyboard...)
}

func TestEventMessageMemberReqMute(t *testing.T) {
	t.Run("custom keeps the button", func(t *testing.T) {
		const tap = 20 * time.Minute
		env := newMreqEventEnv(t)
		env.loadMemberReq()
		tr := env.translator()
		env.mreqRules(map[memberreq.Rule]int{memberreq.RuleLastName: models.RequirementActionMute})
		eventMsgStoreRow(t, env.chat.Id, eventmsg.EventReqMute, mreqEventMuteText, "", 0, []models.Button{eventMsgSiteButton})
		member := screenGoldenReqMember(env)

		got := env.mreqEnforce(env.mreqCase(member))
		if got.Outcome != memberReqMuted || got.Hold == nil {
			t.Fatalf("enforcement = %+v, want Muted with a hold", got)
		}

		notice := mreqEventOnlyNotice(t, env)
		want := "<b>@ann_lee</b> " + mreqEventWord(t, tr, "eventmsg_action_mute") + ": " +
			mreqEventWord(t, tr, "mreq_rule_lastname") + mreqEventNote(t, tr)
		rows := mreqEventMuteRows(tr, got.Hold, member)
		if len(rows) != 2 || len(rows[1]) != 1 || rows[1][0].CallbackData == "" {
			t.Fatalf("expected rows = %+v, want the custom row and one I fixed it button", rows)
		}
		mreqEventWantNotice(t, env, notice, member, want, rows)
		env.wantRowFor(notice.MessageID, deletion.KindBotAuto, env.now().Add(24*time.Hour))

		member = env.mreqFix(member)
		tapped := env.now().Add(tap)
		env.advance(tap)
		restrictsBefore, answersBefore := len(env.mreqRestricts(member.Id)), len(env.mreqAnswers())

		env.mreqPressData(member, notice.MessageID, rows[1][0].CallbackData)

		mreqWantFixedUnmute(t, env.memberReqEnv, member, notice, restrictsBefore, answersBefore, lockdownSnapshotPermissions(t),
			mreqAnswer{Text: mreqEventWord(t, tr, "mreq_fixed_done")},
			mreqFixedNoticeText(t, tr, "mreq_notice_fixed", member))
		env.wantRowFor(notice.MessageID, deletion.KindBotAuto, tapped.Add(600*time.Second))
	})

	t.Run("silent and media are ignored", func(t *testing.T) {
		env := newMreqEventEnv(t)
		tr := env.translator()
		env.mreqRules(map[memberreq.Rule]int{memberreq.RuleLastName: models.RequirementActionMute})
		eventMsgStoreRow(t, env.chat.Id, eventmsg.EventReqMute, mreqEventMuteText, "photo-file-id", db.PHOTO, nil)
		if err := eventmsg.SetSilent(t.Context(), env.chat.Id, eventmsg.EventReqMute, eventmsg.DestGroup, true, 1); err != nil {
			t.Fatalf("SetSilent error = %v", err)
		}
		member := screenGoldenReqMember(env)

		got := env.mreqEnforce(env.mreqCase(member))
		if got.Outcome != memberReqMuted || got.Hold == nil {
			t.Fatalf("enforcement = %+v, want Muted with a hold", got)
		}

		if photos := env.calls("sendPhoto"); len(photos) != 0 {
			t.Fatalf("sendPhoto calls = %d, want none (the requirement notice is text only)", len(photos))
		}
		notice := mreqEventOnlyNotice(t, env)
		want := "<b>@ann_lee</b> " + mreqEventWord(t, tr, "eventmsg_action_mute") + ": " +
			mreqEventWord(t, tr, "mreq_rule_lastname") + mreqEventNote(t, tr)
		mreqEventWantNotice(t, env, notice, member, want, memberReqFixedButton(tr, got.Hold.ID, member.Id).InlineKeyboard)
		env.wantRowFor(notice.MessageID, deletion.KindBotAuto, env.now().Add(24*time.Hour))
	})

	t.Run("undelivered", func(t *testing.T) {
		env := newMreqEventEnv(t)
		env.mreqRules(map[memberreq.Rule]int{memberreq.RuleLastName: models.RequirementActionMute})
		eventMsgStore(t, env.chat.Id, eventmsg.EventReqMute, mreqEventMuteText)
		env.fake.setChatPermsRaw(env.chat.Id, `{"can_send_messages":true,"can_pin_messages":true}`)
		// The custom notice, its retry, the default and its retry are all refused.
		env.fake.script("sendMessage", env.chat.Id, mreqRefusals(4)...)
		member := screenGoldenReqMember(env)

		if got := env.mreqEnforce(env.mreqCase(member)); got.Outcome != memberReqNotApplied {
			t.Fatalf("enforcement = %+v, want NotApplied (the mute was lifted again)", got)
		}

		if n := len(env.calls("sendMessage")); n != 4 {
			t.Errorf("sendMessage calls = %d, want the custom notice, the default and one retry of each", n)
		}
		if n := len(env.mreqNotices()); n != 0 {
			t.Errorf("messages posted = %d, want nothing posted", n)
		}
		if n := len(env.calls("restrictChatMember")); n != 2 {
			t.Fatalf("restrictChatMember calls = %d, want the mute and its lift", n)
		}
		lifted, lift := mreqRestrictPermissions(t, env.memberReqEnv, 1)
		if want := (gotgbot.ChatPermissions{CanSendMessages: true, CanPinMessages: true}); lifted != want {
			t.Errorf("lift permissions = %+v, want the group's live defaults %+v", lifted, want)
		}
		if until := staffParamInt(lift.Params, "until_date"); until != 0 {
			t.Errorf("lift until_date = %d, want none", until)
		}
		hold := env.mreqHold(member.Id)
		if hold == nil || hold.PunishedAt != nil || hold.NoticeMsgID != 0 {
			t.Fatalf("hold = %+v, want a pending backoff row with no notice", hold)
		}
		env.wantNoRows()
	})

	t.Run("private copy", func(t *testing.T) {
		env := newMreqEventEnv(t)
		tr := env.translator()
		env.mreqRules(map[memberreq.Rule]int{memberreq.RuleLastName: models.RequirementActionMute})
		storeEventPM(t, env.chat.Id, eventmsg.EventReqMute, "", true)
		member := screenGoldenReqMember(env)

		got := env.mreqEnforce(env.mreqCase(member))
		if got.Outcome != memberReqMuted || got.Hold == nil {
			t.Fatalf("enforcement = %+v, want Muted with a hold", got)
		}

		if mute, send := mreqEventOrder(env, "restrictChatMember", env.chat.Id), mreqEventOrder(env, "sendMessage", env.chat.Id); mute < 0 || send < mute {
			t.Errorf("the group notice (call %d) came before the mute (call %d)", send, mute)
		}
		notice := mreqEventOnlyNotice(t, env)
		if rows := staffKeyboardOf(notice.Params["reply_markup"]); len(rows) != 1 {
			t.Errorf("group notice keyboard = %+v, want the I fixed it row", rows)
		}
		mreqEventWantPM(t, env, member, mreqEventPMText(t, env, tr, "eventmsg_pm_mreqmute"))
	})
}

func TestMemberReqWarnLimitBanAppeal(t *testing.T) {
	_ = phase28InProcessMiniredis(t)
	withFastPMCopyPacer(t)
	s := newScreenGoldenSite(t)
	s.bot.BotClient = eventPMTestClient{BotClient: s.bot.BotClient, script: &eventPMScript{}}
	eventPMCleanup(t, s.chat.Id)
	t.Cleanup(pmCopyJobs.wait)

	const appealURL = "https://example.com/appeal"
	if err := modtuning.SetAppealURL(t.Context(), s.chat.Id, appealURL, 1); err != nil {
		t.Fatalf("SetAppealURL() error = %v", err)
	}
	storeEventPM(t, s.chat.Id, eventmsg.EventReqWarn, "", true)
	tr := i18n.MustNewTranslator("en")
	member := gotgbot.User{Id: 42, FirstName: "Ann", Username: "ann_lee"}
	wantRow := [][]gotgbot.InlineKeyboardButton{{{Text: mreqEventWord(t, tr, "eventmsg_btn_appeal"), Url: appealURL}}}

	for _, tc := range []struct {
		mode       string
		wantAppeal bool
	}{
		{mode: "ban", wantAppeal: true},
		{mode: "tban", wantAppeal: true},
		{mode: "kick", wantAppeal: false},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			beforeGroup, beforePM := eventPMCalls(t, s)
			hold := &models.ChatRequirementHold{
				ID:     1,
				ChatID: s.chat.Id,
				UserID: member.Id,
				Action: models.RequirementActionWarn,
				Rules:  memberReqRuleBits("lastname"),
			}
			warn := &warnApplyResult{Count: 3, Limit: 3, Mode: tc.mode, LimitReached: true, LimitApplied: true}
			msgID, err := memberReqPostEventNotice(t.Context(), s.bot, memberReqCase{Chat: s.chat}, tr, member, hold, warn)
			if err != nil || msgID == 0 {
				t.Fatalf("memberReqPostEventNotice(%s) = (%d, %v), want sent message", tc.mode, msgID, err)
			}
			waitEventPM(t)
			group, pm := eventPMCalls(t, s)
			newGroup := group[len(beforeGroup):]
			newPM := pm[len(beforePM):]
			if len(newGroup) != 1 {
				t.Fatalf("group notices = %d, want 1", len(newGroup))
			}
			if len(newPM) != 1 {
				t.Fatalf("private copies = %d, want 1", len(newPM))
			}
			var want [][]gotgbot.InlineKeyboardButton
			if tc.wantAppeal {
				want = wantRow
			}
			if got := staffKeyboardOf(newGroup[0].Params["reply_markup"]); fmt.Sprint(got) != fmt.Sprint(want) {
				t.Errorf("mode=%s group notice keyboard = %+v, want %+v", tc.mode, got, want)
			}
			if got := staffKeyboardOf(newPM[0].Params["reply_markup"]); fmt.Sprint(got) != fmt.Sprint(want) {
				t.Errorf("mode=%s private copy keyboard = %+v, want %+v", tc.mode, got, want)
			}
		})
	}
}
