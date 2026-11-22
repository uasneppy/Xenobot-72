//go:build testtools

package modules

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/db/lockdown"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/utils/formatting"
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
