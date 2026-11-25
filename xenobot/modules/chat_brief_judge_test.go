package modules

import (
	"context"
	"fmt"
	"html"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/Xenobot-72/xenobot/db/chatconv"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/db/staff"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/chat_status"
)

// commandMessage builds a command message (text starts with the command) from from, replying to
// target (nil for no reply).
func (e *chatEnv) commandMessage(from *gotgbot.User, text string, target *gotgbot.Message) *gotgbot.Message {
	msg := e.message(from, text)
	name, _, _ := strings.Cut(text, " ")
	msg.Entities = []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: int64(len(name))}}
	msg.ReplyToMessage = target
	return msg
}

// judge delivers text as a command of from replying to target, then lets the brief cooldown pass.
func (e *chatEnv) judge(from *gotgbot.User, text string, target *gotgbot.Message) {
	e.t.Helper()
	e.deliver(e.commandMessage(from, text, target))
	e.mr.FastForward(chatBriefCooldown + time.Second)
}

// sendsTo returns the texts the bot sent to chatID, delivered or refused, in order.
func (e *chatEnv) sendsTo(chatID int64) []string {
	var texts []string
	for _, method := range []string{"sendMessage", "sendMessage (refused)"} {
		for _, call := range e.client.callsFor(method) {
			if fmt.Sprint(call.Params["chat_id"]) == fmt.Sprint(chatID) {
				texts = append(texts, fmt.Sprint(call.Params["text"]))
			}
		}
	}
	return texts
}

// judgeAdmin returns a fresh member whose live status is administrator.
func (e *chatEnv) judgeAdmin(name string) *gotgbot.User {
	user := e.member(name)
	e.fake.setStatus(user.Id, gotgbot.ChatMemberStatusAdministrator)
	return user
}

// judgeLinkStaff links the env's group to a fresh Staff Group and returns its chat ID and the link.
func judgeLinkStaff(t *testing.T, e *chatEnv, title string) (int64, *models.StaffGroupLink) {
	t.Helper()
	staffChat, owner := uniqueModuleChatID(), uniqueLinkOwnerID()
	staffCleanup(t, staffChat, e.chatID)
	if _, err := staff.CreateStaffGroup(staffChat, owner, title); err != nil {
		t.Fatalf("create Staff Group: %v", err)
	}
	link := &models.StaffGroupLink{GroupChatID: e.chatID, StaffChatID: staffChat, OwnerUserID: owner, GroupTitle: "Facility 7"}
	if err := staff.CreateLink(link); err != nil {
		t.Fatalf("create link: %v", err)
	}
	return staffChat, link
}

// judgeBudgetKeys counts the brief budget keys in Redis.
func judgeBudgetKeys(e *chatEnv) int {
	n := 0
	for _, key := range e.mr.Keys() {
		if strings.HasPrefix(key, chatKeyPrefix+"brief:b:") {
			n++
		}
	}
	return n
}

func TestChatJudge(t *testing.T) {
	env := newChatEnv(t)
	chatLogTurnOn(t, env)
	chatSwap(t, &chatBriefUserHourlyBudget, int64(100))
	t.Cleanup(func() { cache.InvalidateAdminCache(env.chatID) })
	alice, bob, carol := env.member("Alice"), env.member("Bob"), env.member("Carol")
	msgs := briefTalk(env, alice, bob, carol)
	ctx := context.Background()
	notice := chatCanned(t, "judge_ai_notice")
	verdicts := func(chatID int64) int {
		n := 0
		for _, text := range env.sendsTo(chatID) {
			if strings.Contains(text, notice) {
				n++
			}
		}
		return n
	}

	t.Run("a non-admin, and an admin the cache names but whose live record is a member, are refused", func(t *testing.T) {
		dave := env.member("Dave")
		env.judge(dave, "/judge", msgs[1])
		eve := env.member("Eve")
		env.fake.listAdmins(eve.Id)
		if !chat_status.IsUserAdmin(env.bot, env.chatID, eve.Id) {
			t.Fatal("the cached admin list does not name Eve")
		}
		env.judge(eve, "/judge", msgs[1])
		if n := len(env.provider.calls()); n != 0 {
			t.Fatalf("a refused /judge made %d provider calls", n)
		}
		if verdicts(env.chatID) != 0 {
			t.Fatal("a verdict was posted")
		}
		asked := map[string]bool{}
		for _, call := range env.client.callsFor("getChatMember") {
			asked[fmt.Sprint(call.Params["user_id"])] = true
		}
		if !asked[strconv.FormatInt(eve.Id, 10)] {
			t.Error("Eve was not checked live")
		}
	})

	admin := env.judgeAdmin("Ada")
	t.Run("an admin's verdict in the group", func(t *testing.T) {
		env.provider.script(chatAnswerText("Bob: 2/10 & calm. Carol: 1/10."))
		before := len(env.sendsTo(env.chatID))
		env.judge(admin, "/judge", msgs[1])
		calls := env.provider.calls()
		if len(calls) != 1 || calls[0].MaxTokens != chatBriefMaxTokens(chatBriefJudge) || calls[0].Static != chatBriefInstruction(chatBriefJudge) {
			t.Fatalf("provider calls %+v, want one judge request", calls)
		}
		sent := env.sendsTo(env.chatID)[before:]
		if len(sent) != 1 {
			t.Fatalf("sent %q, want one verdict", sent)
		}
		for _, want := range []string{chatCanned(t, "judge_header"), "Bob: 2/10 &amp; calm.", notice} {
			if !strings.Contains(sent[0], want) {
				t.Errorf("the verdict %q lacks %q", sent[0], want)
			}
		}
	})

	t.Run("an unknown or extra argument answers the usage and spends nothing", func(t *testing.T) {
		calls, budget := len(env.provider.calls()), judgeBudgetKeys(env)
		before := len(env.sendsTo(env.chatID))
		env.judge(admin, "/judge everyone", msgs[1])
		env.judge(admin, "/judge pm now", msgs[1])
		if got := env.sendsTo(env.chatID)[before:]; len(got) != 2 || got[0] != chatCanned(t, "judge_usage") || got[1] != chatCanned(t, "judge_usage") {
			t.Fatalf("sends %q, want the usage twice", got)
		}
		if len(env.provider.calls()) != calls || judgeBudgetKeys(env) != budget {
			t.Fatal("a bad argument made a provider call or spent a budget unit")
		}
	})

	t.Run("pm goes to the requester's own chat with the group and the requester named", func(t *testing.T) {
		groupBefore := len(env.sendsTo(env.chatID))
		env.judge(admin, "/judge PM", msgs[1])
		private := env.sendsTo(admin.Id)
		if len(private) != 1 || !strings.Contains(private[0], "Facility 7") || !strings.Contains(private[0], "Ada") || !strings.Contains(private[0], notice) {
			t.Fatalf("private sends %q", private)
		}
		if got := env.sendsTo(env.chatID)[groupBefore:]; len(got) != 1 || got[0] != chatCanned(t, "judge_note_pm") {
			t.Fatalf("group sends %q, want the pm note", got)
		}
	})

	t.Run("the argument overrides the stored default in every direction", func(t *testing.T) {
		staffChat, _ := judgeLinkStaff(t, env, "Secret HQ")
		if err := chatconv.SetJudgeTo(ctx, env.chatID, chatconv.JudgeToPrivate, 1); err != nil {
			t.Fatal(err)
		}
		groupVerdicts, privateVerdicts, staffVerdicts := verdicts(env.chatID), verdicts(admin.Id), verdicts(staffChat)
		env.judge(admin, "/judge group", msgs[1])
		env.judge(admin, "/judge staff", msgs[1])
		if err := chatconv.SetJudgeTo(ctx, env.chatID, chatconv.JudgeToStaff, 1); err != nil {
			t.Fatal(err)
		}
		env.judge(admin, "/judge pm", msgs[1])
		env.judge(admin, "/judge", msgs[1])
		if verdicts(env.chatID) != groupVerdicts+1 || verdicts(admin.Id) != privateVerdicts+1 || verdicts(staffChat) != staffVerdicts+2 {
			t.Fatalf("verdicts group %d, private %d, staff %d", verdicts(env.chatID)-groupVerdicts, verdicts(admin.Id)-privateVerdicts, verdicts(staffChat)-staffVerdicts)
		}
		if err := chatconv.SetJudgeTo(ctx, env.chatID, chatconv.JudgeToGroup, 1); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("a blocked private chat answers in the group and sends nothing else", func(t *testing.T) {
		blocked := env.judgeAdmin("Bea")
		env.fake.refuse(blocked.Id)
		groupBefore := len(env.sendsTo(env.chatID))
		env.judge(blocked, "/judge pm", msgs[1])
		if got := env.sendsTo(env.chatID)[groupBefore:]; len(got) != 1 || got[0] != chatCanned(t, "judge_pm_blocked") {
			t.Fatalf("group sends %q, want judge_pm_blocked alone", got)
		}
		if got := env.sendsTo(blocked.Id); len(got) != 1 {
			t.Fatalf("private attempts %q, want the one refused send", got)
		}
	})
}

func TestChatJudgeStaff(t *testing.T) {
	env := newChatEnv(t)
	chatLogTurnOn(t, env)
	chatSwap(t, &chatBriefUserHourlyBudget, int64(100))
	alice, bob, carol := env.member("Alice"), env.member("Bob"), env.member("Carol")
	msgs := briefTalk(env, alice, bob, carol)
	admin := env.judgeAdmin("Ada")
	const staffTitle = "Secret <HQ> 7"
	staffChat, link := judgeLinkStaff(t, env, staffTitle)
	notice := chatCanned(t, "judge_ai_notice")

	groupBefore := len(env.sendsTo(env.chatID))
	env.judge(admin, "/judge staff", msgs[1])
	toStaff := env.sendsTo(staffChat)
	if len(toStaff) != 1 || !strings.Contains(toStaff[0], notice) || !strings.Contains(toStaff[0], "Facility 7") {
		t.Fatalf("Staff Group sends %q", toStaff)
	}
	group := env.sendsTo(env.chatID)[groupBefore:]
	if len(group) != 1 || group[0] != chatCanned(t, "judge_note_staff") {
		t.Fatalf("group sends %q, want the staff note", group)
	}
	for _, text := range group {
		if strings.Contains(text, "Secret") || strings.Contains(text, strconv.FormatInt(staffChat, 10)) {
			t.Fatalf("the group note names the Staff Group: %q", text)
		}
	}

	t.Run("a link removed between the handler and the send answers no staff and sends nothing", func(t *testing.T) {
		env.provider.script(func(context.Context, chatRequest) (chatAnswer, error) {
			if _, err := staff.DeleteLink(link.ID); err != nil {
				t.Errorf("delete link: %v", err)
			}
			return chatAnswer{Text: "Bob: 3/10."}, nil
		})
		staffBefore, groupBefore := len(env.sendsTo(staffChat)), len(env.sendsTo(env.chatID))
		env.judge(admin, "/judge staff", msgs[1])
		if got := env.sendsTo(staffChat); len(got) != staffBefore {
			t.Fatalf("the Staff Group got %q after the link was removed", got[staffBefore:])
		}
		if got := env.sendsTo(env.chatID)[groupBefore:]; len(got) != 1 || got[0] != chatCanned(t, "judge_no_staff") {
			t.Fatalf("group sends %q, want judge_no_staff", got)
		}
	})

	t.Run("an unlinked group answers no staff with no provider call", func(t *testing.T) {
		calls := len(env.provider.calls())
		groupBefore := len(env.sendsTo(env.chatID))
		env.judge(admin, "/judge staff", msgs[1])
		if len(env.provider.calls()) != calls {
			t.Fatal("an unlinked group made a provider call")
		}
		if got := env.sendsTo(env.chatID)[groupBefore:]; len(got) != 1 || got[0] != chatCanned(t, "judge_no_staff") {
			t.Fatalf("group sends %q", got)
		}
	})

	t.Run("a link that cannot be read answers in the group with no provider call", func(t *testing.T) {
		chatSwap(t, &chatJudgeStaffLink, func(int64) (*models.StaffGroupLink, error) { return nil, fmt.Errorf("db down") })
		calls := len(env.provider.calls())
		groupBefore := len(env.sendsTo(env.chatID))
		env.judge(admin, "/judge staff", msgs[1])
		if len(env.provider.calls()) != calls {
			t.Fatal("an unreadable link made a provider call")
		}
		if got := env.sendsTo(env.chatID)[groupBefore:]; len(got) != 1 || got[0] != chatCanned(t, "judge_staff_check_failed") {
			t.Fatalf("group sends %q", got)
		}
	})
}

// TestChatJudgeRoster shrinks the input cap and the roster so a handful of messages exercises both:
// a participant who only wrote in the dropped middle is marked on the roster, the participants
// beyond the roster are named in the closing block, and a short stretch has no unscored line.
func TestChatJudgeRoster(t *testing.T) {
	env := newChatEnv(t)
	chatLogTurnOn(t, env)
	chatSwap(t, &chatBriefMaxRoster, 3)
	chatSwap(t, &chatBriefMaxInputRunes, chatBriefRosterReserve()+260)
	admin := env.judgeAdmin("Ada")
	alice, dora := env.member("Alice"), env.member("Dora")
	bob, carol, eve, fay := env.member("Bob"), env.member("Carol"), env.member("Eve"), env.member("Fay <3")
	first := env.message(alice, "it started here")
	env.deliver(first)
	for range 4 {
		env.deliver(env.message(dora, strings.Repeat("long middle line ", 6)))
	}
	late := env.message(bob, "stop it")
	for _, msg := range []*gotgbot.Message{late, env.message(carol, "agreed"), env.message(eve, "me too"), env.message(fay, "same"), env.message(alice, "fine")} {
		env.deliver(msg)
	}

	env.judge(admin, "/judge", first)
	calls := env.provider.calls()
	if len(calls) != 1 {
		t.Fatalf("provider calls = %d", len(calls))
	}
	user := calls[0].User
	if !strings.Contains(user, "messages were left out here") {
		t.Fatalf("the cap left nothing out:\n%s", user)
	}
	_, roster, _ := strings.Cut(user, "[The roster")
	roster, _, _ = strings.Cut(roster, "\n\n")
	for _, want := range []string{"\nDora" + chatBriefNoMessagesShown, "\nAlice\n", "\nBob"} {
		if !strings.Contains(roster+"\n", want) {
			t.Errorf("the roster lacks %q:\n%s", want, roster)
		}
	}
	for _, unwanted := range []string{"Carol", "Eve", "Fay", "Bob" + chatBriefNoMessagesShown} {
		if strings.Contains(roster, unwanted) {
			t.Errorf("the roster holds %q:\n%s", unwanted, roster)
		}
	}
	for _, text := range []string{calls[0].Dynamic, user} {
		for _, id := range []int64{admin.Id, alice.Id, dora.Id, bob.Id, carol.Id, eve.Id, fay.Id, env.chatID} {
			if strings.Contains(text, strconv.FormatInt(id, 10)) {
				t.Fatalf("the ID %d reached the provider", id)
			}
		}
	}

	sent := env.sendsTo(env.chatID)
	last := sent[len(sent)-1]
	note := chatCannedTextWith(env.chatID, "judge_unscored_note", i18n.TranslationParams{"names": "Carol, Eve, " + html.EscapeString("Fay <3")})
	noticeAt, noteAt := strings.Index(last, chatCanned(t, "judge_ai_notice")), strings.Index(last, note)
	if noteAt < 0 || noticeAt < noteAt {
		t.Fatalf("the verdict's last message %q lacks the unscored note before the AI notice (%q)", last, note)
	}
	for _, text := range sent {
		if chatUTF16Len(text) > 4096 {
			t.Errorf("a message is %d UTF-16 units", chatUTF16Len(text))
		}
	}

	env.judge(admin, "/judge", late)
	sent = env.sendsTo(env.chatID)
	if last := sent[len(sent)-1]; !strings.Contains(last, chatCanned(t, "judge_ai_notice")) || strings.Contains(last, html.EscapeString("Fay <3")+".") {
		t.Fatalf("a stretch whose participants fit the roster names unscored people: %q", last)
	}
	if strings.Contains(sent[len(sent)-1], "Carol, Eve") {
		t.Fatalf("a short stretch has an unscored line: %q", sent[len(sent)-1])
	}
}

// TestChatJudgeAnonymousAdmin proves the proof comes first and the tapper is checked live.
func TestChatJudgeAnonymousAdmin(t *testing.T) {
	env := newChatEnv(t)
	chatLogTurnOn(t, env)
	LoadBotUpdates(env.dispatcher)
	t.Cleanup(func() { cache.InvalidateAdminCache(env.chatID) })
	alice, bob, carol := env.member("Alice"), env.member("Bob"), env.member("Carol")
	msgs := briefTalk(env, alice, bob, carol)
	tapper, demoted := env.member("Tia"), env.member("Dan")
	env.fake.listAdmins(tapper.Id, demoted.Id)
	env.fake.setStatus(tapper.Id, gotgbot.ChatMemberStatusAdministrator)
	env.fake.setStatus(demoted.Id, gotgbot.ChatMemberStatusMember)

	anonymous := func() {
		msg := env.commandMessage(&gotgbot.User{Id: 1087968824, IsBot: true, FirstName: "Group", Username: "GroupAnonymousBot"}, "/judge pm", msgs[1])
		msg.SenderChat = &gotgbot.Chat{Id: env.chatID, Type: "supergroup", Title: "Facility 7"}
		env.deliver(msg)
	}
	proof := func() string {
		calls := env.client.callsFor("sendMessage")
		for i := len(calls) - 1; i >= 0; i-- {
			for _, row := range staffKeyboardOf(calls[i].Params["reply_markup"]) {
				for _, button := range row {
					if _, ok := decodeCallbackData(button.CallbackData, "anon_admin"); ok {
						return button.CallbackData
					}
				}
			}
		}
		t.Fatal("no proof prompt was sent")
		return ""
	}
	tap := func(from *gotgbot.User, data string) {
		env.nextUpdate++
		update := &gotgbot.Update{UpdateId: env.nextUpdate, CallbackQuery: &gotgbot.CallbackQuery{
			Id: fmt.Sprintf("judge-%d", env.nextUpdate), From: *from, Data: data, ChatInstance: "judge-anon",
			Message: gotgbot.Message{MessageId: 9001, Date: 1, Chat: gotgbot.Chat{Id: env.chatID, Type: "supergroup"}},
		}}
		if err := env.dispatcher.ProcessUpdate(env.bot, update, nil); err != nil {
			t.Fatalf("ProcessUpdate: %v", err)
		}
		chatJobs.wait()
	}

	anonymous()
	data := proof()
	if n := len(env.provider.calls()); n != 0 {
		t.Fatalf("the anonymous /judge made %d provider calls before the proof", n)
	}

	t.Run("a tapper the cache names but whose live record is a member is refused", func(t *testing.T) {
		tap(demoted, data)
		if n := len(env.provider.calls()); n != 0 {
			t.Fatalf("a demoted tapper made %d provider calls", n)
		}
		if got := env.sendsTo(demoted.Id); len(got) != 0 {
			t.Fatalf("the demoted tapper got %q", got)
		}
	})

	t.Run("an admin tapper is checked live and gets the verdict in private", func(t *testing.T) {
		anonymous()
		tap(tapper, proof())
		asked := false
		for _, call := range env.client.callsFor("getChatMember") {
			if fmt.Sprint(call.Params["user_id"]) == strconv.FormatInt(tapper.Id, 10) {
				asked = true
			}
		}
		if !asked {
			t.Error("the tapper was not checked live")
		}
		if len(env.provider.calls()) != 1 {
			t.Fatalf("provider calls = %d, want 1", len(env.provider.calls()))
		}
		private := env.sendsTo(tapper.Id)
		if len(private) != 1 || !strings.Contains(private[0], chatCanned(t, "judge_ai_notice")) {
			t.Fatalf("the tapper's private sends %q", private)
		}
	})
}

// TestChatJudgeNeverActs proves a verdict naming a member and a score acts on nobody.
func TestChatJudgeNeverActs(t *testing.T) {
	env := newChatEnv(t)
	chatLogTurnOn(t, env)
	alice, bob, carol := env.member("Alice"), env.member("Bob"), env.member("Carol")
	msgs := briefTalk(env, alice, bob, carol)
	admin := env.judgeAdmin("Ada")
	env.provider.script(chatAnswerText("Bob: 10/10, he should be banned. Carol: 9/10, warn her."))
	env.judge(admin, "/judge", msgs[1])
	if len(env.provider.calls()) != 1 {
		t.Fatal("no verdict was written")
	}
	for _, method := range []string{"restrictChatMember", "banChatMember", "unbanChatMember", "banChatSenderChat", "approveChatJoinRequest", "declineChatJoinRequest", "deleteMessage"} {
		if calls := env.client.callsFor(method); len(calls) != 0 {
			t.Errorf("a verdict made %d %s calls", len(calls), method)
		}
	}
}
