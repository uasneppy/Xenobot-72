//go:build testtools

package modules

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/eventmsg"
	"github.com/uasneppy/Xenobot-72/xenobot/db/lockdown"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/formatting"
)

// lockdownEventEnv is a lockdown environment whose chat has every deletion rule at its shortest delay (so a
// lockdown message that entered the deletion queue would leave a row) and whose stored event messages are
// removed when the test ends.
func lockdownEventEnv(t *testing.T, golden bool) *lockdownEnv {
	t.Helper()
	var env *lockdownEnv
	if golden {
		env = newLockdownGoldenEnv(t)
	} else {
		env = newLockdownEnv(t)
		deletionShortestRules(t, env.chat.Id)
	}
	eventMsgCleanup(t, env.chat.Id)
	return env
}

// lockdownEventWantStarted fails unless the chat's lockdown is active and confirmed, started by starterID,
// and the group's permissions are the locked set.
func lockdownEventWantStarted(t *testing.T, env *lockdownEnv, starterID int64) *models.ChatLockdown {
	t.Helper()
	row, err := lockdown.GetActiveFresh(env.chat.Id)
	if err != nil || row == nil || row.LockedAt == nil || row.StartedBy != starterID {
		t.Fatalf("lockdown = %+v, %v, want one confirmed and started by %d", row, err, starterID)
	}
	var got, want map[string]bool
	if err := json.Unmarshal([]byte(env.fake.chatPermsRaw(env.chat.Id)), &got); err != nil {
		t.Fatalf("live permissions: %v", err)
	}
	if err := json.Unmarshal([]byte(lockdownLockedPermissions), &want); err != nil {
		t.Fatalf("locked set: %v", err)
	}
	for right, value := range want {
		if got[right] != value {
			t.Fatalf("live permission %s = %v, want the locked set %s", right, got[right], lockdownLockedPermissions)
		}
	}
	return row
}

// lockdownEventOnlySend is the one message the group received, serialised; it fails unless there is
// exactly one.
func lockdownEventOnlySend(t *testing.T, env *lockdownEnv) (text, raw string) {
	t.Helper()
	sent := env.fake.sentTo(env.chat.Id)
	if len(sent) != 1 {
		t.Fatalf("the group received %d messages, want exactly one", len(sent))
	}
	return fmt.Sprint(sent[0].Params["text"]), lockdownGoldenRequest(t, env, env.chat.Id, 0)
}

func TestEventMessageLockdownStart(t *testing.T) {
	const stored = "<b>{admin}</b> locked {chatname}: {reason} ({mention})"

	customReply := func(t *testing.T, canDelete, canInvite bool, notes ...string) {
		t.Helper()
		env := lockdownEventEnv(t, false)
		env.fake.setBotMember(env.chat.Id, lockdownBotAdminJSON(true, canDelete, canInvite))
		eventMsgStore(t, env.chat.Id, eventmsg.EventLockdown, stored)

		env.send(env.admin, "/lockdown raid & co")

		lockdownEventWantStarted(t, env, env.admin.Id)
		methods := eventMsgMethods(env.fake.moduleBotClient)
		if lock, send := eventMsgIndex(methods, "setChatPermissions"), eventMsgIndex(methods, "sendMessage"); lock < 0 || send < lock {
			t.Fatalf("calls = %v, want the lock before the message", methods)
		}
		text, raw := lockdownEventOnlySend(t, env)
		want := "<b>Ad&lt;b&gt;min</b> locked Lock Chat: raid &amp; co (" + formatting.MentionHtml(env.admin.Id, "Ad<b>min") + ")"
		for _, key := range append([]string{"lockdown_note_approved_muted"}, notes...) {
			want += "\n" + staffMarker(key)
		}
		if text != want {
			t.Fatalf("custom reply\n got %q\nwant %q", text, want)
		}
		if replyTo := `"reply_parameters":{"message_id":101,"allow_sending_without_reply":true}`; !strings.Contains(raw, replyTo) {
			t.Fatalf("request = %s, want a reply to the command (%s)", raw, replyTo)
		}
		lockdownGoldenNoQueue(t, env.chat.Id)
		lockdownGoldenNoPrivate(t, env)
	}

	t.Run("custom", func(t *testing.T) {
		customReply(t, true, true)
	})

	t.Run("custom notes", func(t *testing.T) {
		customReply(t, false, false, "lockdown_note_no_delete", "lockdown_note_no_invite")
	})

	t.Run("stored sticker falls back to text", func(t *testing.T) {
		env := lockdownEventEnv(t, false)
		env.fake.setBotMember(env.chat.Id, lockdownBotAdminJSON(true, false, false))
		eventMsgStore(t, env.chat.Id, eventmsg.EventLockdown, stored)
		if err := eventmsg.SetMedia(t.Context(), env.chat.Id, eventmsg.EventLockdown, eventmsg.DestGroup, "sticker-legacy", db.STICKER, 1); err != nil {
			t.Fatalf("SetMedia error = %v", err)
		}

		env.send(env.admin, "/lockdown raid & co")

		lockdownEventWantStarted(t, env, env.admin.Id)
		if got := len(env.fake.callsFor("sendSticker")); got != 0 {
			t.Fatalf("sendSticker calls = %d, want 0", got)
		}
		text, _ := lockdownEventOnlySend(t, env)
		want := "<b>Ad&lt;b&gt;min</b> locked Lock Chat: raid &amp; co (" + formatting.MentionHtml(env.admin.Id, "Ad<b>min") + ")" +
			"\n" + staffMarker("lockdown_note_approved_muted") +
			"\n" + staffMarker("lockdown_note_no_delete") +
			"\n" + staffMarker("lockdown_note_no_invite")
		if text != want {
			t.Fatalf("sticker fallback reply\n got %q\nwant %q", text, want)
		}
	})

	t.Run("silent", func(t *testing.T) {
		env := lockdownEventEnv(t, false)
		staffChat := linkLockdownStaffGroup(env)
		env.fake.setBotMember(env.chat.Id, lockdownBotAdminJSON(true, true, true))
		if err := eventmsg.SetSilent(t.Context(), env.chat.Id, eventmsg.EventLockdown, eventmsg.DestGroup, true, 1); err != nil {
			t.Fatalf("SetSilent error = %v", err)
		}

		env.send(env.admin, "/lockdown")

		lockdownEventWantStarted(t, env, env.admin.Id)
		lockdownGoldenWantSends(t, env, env.chat.Id, 0)
		env.cycle()
		if got := len(env.fake.sentTo(staffChat.Id)); got != 1 {
			t.Fatalf("the Staff Group received %d messages after the worker cycle, want the alert", got)
		}
		lockdownGoldenWantSends(t, env, env.chat.Id, 0)
		lockdownGoldenNoQueue(t, env.chat.Id)
	})

	t.Run("refused", func(t *testing.T) {
		env := lockdownEventEnv(t, true)
		env.fake.setBotMember(env.chat.Id, lockdownBotAdminJSON(true, true, true))
		eventMsgStore(t, env.chat.Id, eventmsg.EventLockdown, "custom {admin}")
		env.fake.script("sendMessage", env.chat.Id, staffFakeError(400, "Bad Request: the custom message was refused"))

		env.send(env.admin, "/lockdown")

		lockdownEventWantStarted(t, env, env.admin.Id)
		if got := len(env.fake.callsFor("sendMessage")); got != 2 {
			t.Fatalf("sendMessage calls = %d, want the refused custom message and the default", got)
		}
		lockdownGoldenWantSends(t, env, env.chat.Id, 1)
		ids := map[int64]string{env.chat.Id: "<chat>", env.admin.Id: "<admin>"}
		lockdownGoldenWant(t, "the default after the refusal", lockdownGoldenRequest(t, env, env.chat.Id, 0), ids, goldenLockdownStart)
		lockdownGoldenNoQueue(t, env.chat.Id)
	})

	t.Run("anonymous", func(t *testing.T) {
		env := lockdownEventEnv(t, false)
		LoadBotUpdates(env.dispatcher)
		env.fake.setBotMember(env.chat.Id, lockdownBotAdminJSON(true, true, true))
		tapper := gotgbot.User{Id: uniqueLinkOwnerID(), FirstName: "Tap<per"}
		env.fake.setMember(env.chat.Id, tapper.Id, staffFakeMember{
			Status:             gotgbot.ChatMemberStatusAdministrator,
			CanRestrictMembers: true,
		})
		env.fake.listAdministrator(env.chat.Id, tapper.Id)
		eventMsgStore(t, env.chat.Id, eventmsg.EventLockdown, "{admin} {mention} {id}")

		env.sendAnonymous("/lockdown")
		env.tapProof(tapper)

		lockdownEventWantStarted(t, env, tapper.Id)
		sent := env.fake.sentTo(env.chat.Id)
		if len(sent) < 2 {
			t.Fatalf("the group received %d messages, want the proof prompt and the reply", len(sent))
		}
		text := fmt.Sprint(sent[len(sent)-1].Params["text"])
		// The bot has a username, so {mention} reads as it; {id} is the bot's own ID.
		want := staffMarker("eventmsg_admin_anon") + " @" + env.bot.Username + " " + strconv.FormatInt(env.bot.Id, 10) +
			"\n" + staffMarker("lockdown_note_approved_muted")
		if text != want {
			t.Fatalf("custom reply after the proof\n got %q\nwant %q", text, want)
		}
		if strings.Contains(text, strconv.FormatInt(tapper.Id, 10)) || strings.Contains(text, "Tap") {
			t.Fatalf("custom reply %q names the tapper", text)
		}
		lockdownGoldenNoQueue(t, env.chat.Id)
	})
}

// The IDs the default builders are proven with; lockdownDefaultIDs maps them to the golden tokens.
const (
	lockdownDefaultChat   int64 = -1009925020200
	lockdownDefaultAdmin  int64 = 925020201
	lockdownDefaultTapper int64 = 925020202
)

var lockdownDefaultIDs = map[int64]string{
	lockdownDefaultChat: "<chat>", lockdownDefaultAdmin: "<admin>", lockdownDefaultTapper: "<tapper>",
}

// lockdownDefaultRequest sends a default builder's text and options through the call its site's default
// makes, on an in-memory recording client, and returns the request serialised: a reply to replyTo through
// msg.Reply (the command replies), or a new message through SendMessage when replyTo is 0 (the paced
// notices, whose options the caller passes).
func lockdownDefaultRequest(t *testing.T, text string, opts *gotgbot.SendMessageOpts, replyTo int64) string {
	t.Helper()
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	var err error
	if replyTo != 0 {
		msg := gotgbot.Message{MessageId: replyTo, Chat: gotgbot.Chat{Id: lockdownDefaultChat, Type: "supergroup"}}
		_, err = msg.Reply(bot, text, opts)
	} else {
		_, err = bot.SendMessage(lockdownDefaultChat, text, opts)
	}
	if err != nil {
		t.Fatalf("send error = %v", err)
	}
	sends := client.callsFor("sendMessage")
	if len(sends) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(sends))
	}
	return legacyRequestJSON(t, sends[0])
}

func TestLockdownEventDefaults(t *testing.T) {
	withRealLocales(t)
	tr := i18n.MustNewTranslator("uk")
	admin := gotgbot.User{Id: lockdownDefaultAdmin, FirstName: "Ad<b>min"}
	tapper := gotgbot.User{Id: lockdownDefaultTapper, FirstName: "Tap<per"}
	// The paced lift notice's options, lockdownPostLiftNotice's: no reply parameters.
	pacedLift := &gotgbot.SendMessageOpts{ParseMode: formatting.HTML, LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true}}

	cases := []struct {
		name    string
		build   eventDefaultReply
		in      eventDefaultInput
		replyTo int64
		paced   *gotgbot.SendMessageOpts
		golden  string
	}{
		{"start", lockdownStartDefaultReply, eventDefaultInput{Target: admin}, 101, nil, goldenLockdownStart},
		{"start_reason_no_rights", lockdownStartDefaultReply,
			eventDefaultInput{Target: admin, Reason: "raid & <b>spam</b>", Lockdown: lockdownFacts{NoDelete: true, NoInvite: true}},
			101, nil, goldenLockdownStartReasonNoRights},
		{"start_anonymous", lockdownStartDefaultReply, eventDefaultInput{Target: tapper}, 101, nil, goldenLockdownStartAnonymous},
		{"auto_join_surge", lockdownAutoDefaultReply(models.LockdownTriggerJoinSurge),
			eventDefaultInput{Count: 15, Window: 60}, 0, nil, goldenLockdownAutoJoinSurge},
		{"auto_flood_no_delete", lockdownAutoDefaultReply(models.LockdownTriggerFlood),
			eventDefaultInput{Count: 30, Window: 30, Percent: 80, Lockdown: lockdownFacts{NoDelete: true}}, 0, nil, goldenLockdownAutoFloodNoDelete},
		{"auto_ai_no_invite", lockdownAutoDefaultReply(models.LockdownTriggerAIBurst),
			eventDefaultInput{Count: 5, Window: 120, Lockdown: lockdownFacts{NoInvite: true}}, 0, nil, goldenLockdownAutoAINoInvite},
		{"lift_command", lockdownLiftDefaultReply, eventDefaultInput{Target: admin, Lockdown: lockdownFacts{Finished: true}},
			102, nil, goldenLockdownLiftCommand},
		{"lift_command_unbanning", lockdownLiftDefaultReply,
			eventDefaultInput{Target: admin, Lockdown: lockdownFacts{ManualChange: true, Unbanning: 2}}, 102, nil, goldenLockdownLiftCommandUnbanning},
		{"lift_alert", lockdownLiftDefaultReply, eventDefaultInput{Target: admin, Lockdown: lockdownFacts{Finished: true}},
			0, pacedLift, goldenLockdownLiftAlert},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text, opts := tc.build(tr, tc.in)
			if tc.paced != nil {
				opts = tc.paced
			}
			lockdownGoldenWant(t, "default", lockdownDefaultRequest(t, text, opts, tc.replyTo), lockdownDefaultIDs, tc.golden)
		})
	}

	t.Run("an unknown trigger kind builds nothing", func(t *testing.T) {
		for _, kind := range []string{models.LockdownTriggerManual, "", "raid"} {
			if text, _ := lockdownAutoDefaultReply(kind)(tr, eventDefaultInput{Count: 15, Window: 60}); text != "" {
				t.Errorf("kind %q built %q, want nothing", kind, text)
			}
		}
	})

	t.Run("the defaults end with the notes a custom text gets", func(t *testing.T) {
		facts := lockdownFacts{NoDelete: true, NoInvite: true}
		auto, _ := lockdownAutoDefaultReply(models.LockdownTriggerFlood)(tr, eventDefaultInput{Count: 30, Window: 30, Percent: 80, Lockdown: facts})
		if notes := lockdownStartNotes(tr, facts); !strings.HasSuffix(auto, "\n"+notes) {
			t.Fatalf("automatic default %q does not end with the start notes %q", auto, notes)
		}
		start, _ := lockdownStartDefaultReply(tr, eventDefaultInput{Target: admin, Lockdown: facts})
		for _, line := range lockdownStartNoteLines(tr, facts) {
			if !strings.Contains(start, "\n"+line) {
				t.Fatalf("start default %q lacks the note %q", start, line)
			}
		}
		lift := lockdownFacts{ManualChange: true, Unbanning: 99999}
		first, _ := lockdownLiftDefaultReply(tr, eventDefaultInput{Target: admin, Lockdown: lockdownFacts{Finished: true}})
		full, _ := lockdownLiftDefaultReply(tr, eventDefaultInput{Target: admin, Lockdown: lift})
		if want := first + "\n" + lockdownLiftNotes(tr, lift); full != want {
			t.Fatalf("lift default\n got %q\nwant %q", full, want)
		}
		if notes := lockdownLiftNotes(tr, lockdownFacts{Finished: true, Unbanning: 3}); notes != "" {
			t.Fatalf("a finished lift has the notes %q, want none", notes)
		}
	})

	t.Run("the percent placeholder", func(t *testing.T) {
		values := eventPlaceholderValues(tr, lockdownDefaultChat, eventActor{}, eventValues{Percent: 80})
		if values["percent"] != "80" {
			t.Fatalf("{percent} = %q, want 80", values["percent"])
		}
		if empty := eventPlaceholderValues(tr, lockdownDefaultChat, eventActor{}, eventValues{}); empty["percent"] != "" {
			t.Fatalf("{percent} with no value = %q, want empty", empty["percent"])
		}
		bot := newModuleTestBot(newModuleBotClient())
		chat := &gotgbot.Chat{Id: lockdownDefaultChat, Type: "supergroup", Title: "Lock Chat"}
		target := admin
		filled := eventPlaceholderValues(tr, chat.Id, eventActor{}, eventValues{Count: 30, Window: "30", Percent: 80})
		if got, _ := renderEventText(bot, chat, &target, "{count}/{window}/{percent}", nil, filled); got != "30/30/80" {
			t.Fatalf("rendered %q, want 30/30/80", got)
		}
		manual := eventPlaceholderValues(tr, chat.Id, eventActor{}, eventValues{Reason: "spam"})
		if got, _ := renderEventText(bot, chat, &target, "banned: {reason} {percent}!", nil, manual); got != "banned: spam !" {
			t.Fatalf("a manual event's {percent} rendered %q, want it empty", got)
		}
	})
}

func TestLockdownNoteFitsReserve(t *testing.T) {
	withRealLocales(t)
	if lockdownNoteReserve > 1024-600 {
		t.Fatalf("lockdownNoteReserve = %d leaves less than 600 units of a 1024-unit caption", lockdownNoteReserve)
	}
	units := func(s string) int { return len(utf16.Encode([]rune(s))) }
	for _, lang := range []string{"uk", "en", "es", "fr", "hi", "id", "pt", "ru"} {
		tr := i18n.MustNewTranslator(lang)
		start := lockdownStartNotes(tr, lockdownFacts{NoDelete: true, NoInvite: true})
		lift := lockdownLiftNotes(tr, lockdownFacts{ManualChange: true, Unbanning: 99999})
		if len(lockdownStartNoteLines(tr, lockdownFacts{NoDelete: true, NoInvite: true})) != 3 || strings.Count(lift, "\n") != 1 {
			t.Fatalf("%s: the worst notes are incomplete: start %q, lift %q", lang, start, lift)
		}
		for name, note := range map[string]string{"start": start, "lift": lift} {
			if strings.Contains(note, "{") {
				t.Errorf("%s %s notes %q hold an unfilled placeholder", lang, name, note)
			}
			if n := units("\n" + note); n > lockdownNoteReserve {
				t.Errorf("%s %s notes are %d UTF-16 units, over the reserve of %d: shorten the translation", lang, name, n, lockdownNoteReserve)
			}
		}
	}
}
