//go:build testtools

package modules

import (
	"html"
	"strings"
	"testing"
	"time"

	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
)

// lockdownAutoEnv is a lockdown environment for the automatic notice: every deletion rule of its chat at
// the shortest delay, its raid keys and stored event messages removed when the test ends, and the bot an
// administrator who can restrict and holds the delete and invite rights as asked.
func lockdownAutoEnv(t *testing.T, golden, canDelete, canInvite bool) *lockdownEnv {
	t.Helper()
	env := lockdownEventEnv(t, golden)
	raidCleanup(t, env.chat.Id)
	env.fake.setBotMember(env.chat.Id, lockdownBotAdminJSON(true, canDelete, canInvite))
	return env
}

// lockdownAutoNotes is the line break and the technical notes a custom automatic notice ends with, in the
// marker locale.
func lockdownAutoNotes(keys ...string) string {
	var b strings.Builder
	for _, key := range append([]string{"lockdown_note_approved_muted"}, keys...) {
		b.WriteString("\n" + staffMarker(key))
	}
	return b.String()
}

func TestEventMessageLockdownAuto(t *testing.T) {
	// customNotice fires the trigger directly, waits for the post and checks the one message of the group,
	// the lockdown it announces and that nothing entered the deletion queue or reached a private chat.
	customNotice := func(t *testing.T, env *lockdownEnv, trig raidTrigger, event, stored, want string) {
		t.Helper()
		eventMsgStore(t, env.chat.Id, event, stored)

		raidFire(env.bot, &env.chat, trig)
		raidPosts.wait()

		row := lockdownEventWantStarted(t, env, env.bot.Id)
		if row.TriggerKind != trig.Kind {
			t.Fatalf("trigger kind = %q, want %q", row.TriggerKind, trig.Kind)
		}
		methods := eventMsgMethods(env.fake.moduleBotClient)
		if lock, send := eventMsgIndex(methods, "setChatPermissions"), eventMsgIndex(methods, "sendMessage"); lock < 0 || send < lock {
			t.Fatalf("calls = %v, want the lock before the message", methods)
		}
		text, raw := lockdownEventOnlySend(t, env)
		if text != want {
			t.Fatalf("custom notice\n got %q\nwant %q", text, want)
		}
		if strings.Contains(raw, "reply_parameters") || strings.Contains(raw, "message_thread_id") {
			t.Fatalf("request = %s, want a new message with no reply and no topic", raw)
		}
		lockdownGoldenNoQueue(t, env.chat.Id)
		lockdownGoldenNoPrivate(t, env)
	}

	t.Run("join surge", func(t *testing.T) {
		env := lockdownAutoEnv(t, false, true, true)
		trig := raidTrigger{Kind: models.LockdownTriggerJoinSurge, Count: 15, Window: 60}
		sentence := lockdownTriggerSentence(staffChatTranslator(env.chat.Id), trig.Kind, 15, 60, 0)
		if sentence == "" {
			t.Fatal("setup: the join surge has no trigger sentence")
		}
		want := "15 joined in 60 s: " + html.EscapeString(sentence) +
			" (" + staffMarker("eventmsg_admin_anon") + ", @" + env.bot.Username + ")" + lockdownAutoNotes()
		customNotice(t, env, trig, eventmsg.EventLockJoinSurge, "{count} joined in {window} s: {reason} ({admin}, {mention})", want)
	})

	t.Run("flood notes", func(t *testing.T) {
		env := lockdownAutoEnv(t, false, false, true)
		trig := raidTrigger{Kind: models.LockdownTriggerFlood, Count: 30, Window: 30, Percent: 80}
		customNotice(t, env, trig, eventmsg.EventLockFlood, "{count}/{window}/{percent}",
			"30/30/80"+lockdownAutoNotes("lockdown_note_no_delete"))
	})

	t.Run("ai burst notes", func(t *testing.T) {
		env := lockdownAutoEnv(t, false, true, false)
		trig := raidTrigger{Kind: models.LockdownTriggerAIBurst, Count: 5, Window: 120}
		customNotice(t, env, trig, eventmsg.EventLockAIBurst, "AI <b>{count}</b> in {window}",
			"AI <b>5</b> in 120"+lockdownAutoNotes("lockdown_note_no_invite"))
	})

	t.Run("real surge", func(t *testing.T) {
		env := lockdownAutoEnv(t, false, true, true)
		linkLockdownStaffGroup(env)
		eventMsgStore(t, env.chat.Id, eventmsg.EventLockJoinSurge, "surge {count} in {window}")

		env.selfJoins(15)
		if row := env.activeLockdown(); row == nil || row.LockedAt == nil {
			t.Fatalf("active lockdown = %+v, want one confirmed by the join surge", row)
		}
		if text, want := env.wantOneNoticeIn(), "surge 15 in 60"+lockdownAutoNotes(); text != want {
			t.Fatalf("custom notice\n got %q\nwant %q", text, want)
		}

		// The latch gone, more joins find the lockdown active and post nothing more.
		env.dropRaidLatch()
		env.selfJoins(3)
		if got := len(env.replies()); got != 1 {
			t.Fatalf("%d messages reached the locked group after more joins, want still 1", got)
		}
		lockdownGoldenNoQueue(t, env.chat.Id)
	})

	t.Run("silent", func(t *testing.T) {
		env := lockdownAutoEnv(t, false, true, true)
		staffChat := linkLockdownStaffGroup(env)
		if err := eventmsg.SetSilent(t.Context(), env.chat.Id, eventmsg.EventLockFlood, eventmsg.DestGroup, true, 1); err != nil {
			t.Fatalf("SetSilent error = %v", err)
		}

		raidFire(env.bot, &env.chat, raidTrigger{Kind: models.LockdownTriggerFlood, Count: 30, Window: 30, Percent: 80})
		raidPosts.wait()

		lockdownEventWantStarted(t, env, env.bot.Id)
		lockdownGoldenWantSends(t, env, env.chat.Id, 0)
		env.cycle()
		if got := len(env.fake.sentTo(staffChat.Id)); got != 1 {
			t.Fatalf("the Staff Group received %d messages after the worker cycle, want the alert", got)
		}
		lockdownGoldenWantSends(t, env, env.chat.Id, 0)
		lockdownGoldenNoQueue(t, env.chat.Id)
	})

	t.Run("refused", func(t *testing.T) {
		env := lockdownAutoEnv(t, true, true, true)
		eventMsgStore(t, env.chat.Id, eventmsg.EventLockJoinSurge, "custom {count}")
		env.fake.script("sendMessage", env.chat.Id, staffFakeError(400, "Bad Request: the custom message was refused"))

		raidFire(env.bot, &env.chat, raidTrigger{Kind: models.LockdownTriggerJoinSurge, Count: 15, Window: 60})
		raidPosts.wait()

		lockdownEventWantStarted(t, env, env.bot.Id)
		if got := len(env.fake.callsFor("sendMessage")); got != 2 {
			t.Fatalf("sendMessage calls = %d, want the refused custom message and the default", got)
		}
		lockdownGoldenWantSends(t, env, env.chat.Id, 1)
		ids := map[int64]string{env.chat.Id: "<chat>"}
		lockdownGoldenWant(t, "the default after the refusal", lockdownGoldenRequest(t, env, env.chat.Id, 0), ids, goldenLockdownAutoJoinSurge)
		lockdownGoldenNoQueue(t, env.chat.Id)
	})

	t.Run("drain", func(t *testing.T) {
		env := lockdownAutoEnv(t, false, true, true)
		eventMsgStore(t, env.chat.Id, eventmsg.EventLockJoinSurge, "held {count}")
		gated := gateSends(env)

		raidFire(env.bot, &env.chat, raidTrigger{Kind: models.LockdownTriggerJoinSurge, Count: 15, Window: 60})
		select {
		case <-gated.entered:
		case <-time.After(10 * time.Second):
			t.Fatal("the custom notice never reached its send")
		}

		previous := raidPostStopWait
		raidPostStopWait = 20 * time.Millisecond
		t.Cleanup(func() { raidPostStopWait = previous })
		if StopRaidPosts() {
			t.Fatal("StopRaidPosts reported the posts done while the custom send was held back")
		}

		raidPostStopWait = 10 * time.Second
		gated.open()
		if !StopRaidPosts() {
			t.Fatal("StopRaidPosts did not report the posts done after the custom send went through")
		}
		if text, want := env.wantOneNoticeIn(), "held 15"+lockdownAutoNotes(); text != want {
			t.Fatalf("custom notice\n got %q\nwant %q", text, want)
		}
	})
}
