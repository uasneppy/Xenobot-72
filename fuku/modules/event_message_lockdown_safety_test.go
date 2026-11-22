//go:build testtools

package modules

import (
	"fmt"
	"html"
	"strconv"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/fuku_robot/fuku/db"
	dbcache "github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/chats"
	"github.com/uasneppy/fuku_robot/fuku/db/eventmsg"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/db/staff"
)

// The safeguards of the lockdown messages (Phase 25 plan 25-07): no log line of the five lockdown sites carries a stored
// text, a caption, a file ID, a button, the /lockdown reason, an admin's name or the group's title (D-37), and no lockdown
// message names the Staff Group or the log channel, also when the lift was pressed in the Staff Group (D-36). One
// environment per case, the marker locale, every raid post awaited (D-39).

// The sentinels of the log sweep. They carry no digit, so a search for one is unambiguous.
const (
	lockdownSafeText       = "ZZLOCKSAFETEXT {chatname} {admin}"
	lockdownSafeFile       = "ZZLOCKSAFEFILE"
	lockdownSafeButton     = "ZZLOCKSAFEBUTTON"
	lockdownSafeLink       = "https://zzlocksafe.example/link"
	lockdownSafeReason     = "zzlocksafereason"
	lockdownSafeAdminName  = "ZZLockSafeAdmin"
	lockdownSafeGroupTitle = "ZZ Lock Safe Group"
)

// lockdownSafeStore stores a whole custom group message of each event for chatID: the sentinel text as the caption of a
// photo with the sentinel file ID, and the sentinel button.
func lockdownSafeStore(t *testing.T, chatID int64, events ...string) {
	t.Helper()
	for _, event := range events {
		eventMsgStoreRow(t, chatID, event, lockdownSafeText, lockdownSafeFile, db.PHOTO,
			[]models.Button{{Name: lockdownSafeButton, Url: lockdownSafeLink}})
	}
}

// lockdownSafeRename gives the environment's group the sentinel title and its administrator the sentinel name, before
// anything is recorded.
func lockdownSafeRename(env *lockdownEnv) {
	env.chat.Title = lockdownSafeGroupTitle
	env.admin.FirstName = lockdownSafeAdminName
}

// lockdownSafeRefusePhoto is the refusal of a stored photo Telegram cannot send; its description holds no sentinel.
var lockdownSafeRefusePhoto = staffFakeError(400, "Bad Request: wrong file identifier")

// lockdownSafeBreak makes the next custom sends of chatID fail as the case asks: a read error makes the engine's read of
// the stored messages fail for the rest of the test, a refusal makes the next n sendPhoto requests to the chat fail.
func lockdownSafeBreak(t *testing.T, env *lockdownEnv, readError bool, n int) {
	t.Helper()
	if readError {
		screenSafeFailRead(t)
		return
	}
	for range n {
		env.fake.script("sendPhoto", env.chat.Id, lockdownSafeRefusePhoto)
	}
}

// lockdownSafeWant is the log line each failure must leave for event in chatID: the read error's or the refusal's, each
// naming the chat ID and the event only.
func lockdownSafeWant(readError bool, chatID int64, events ...string) []string {
	var lines []string
	for _, event := range events {
		if readError {
			lines = append(lines, fmt.Sprintf("messages of chat %d could not be read for %s", chatID, event))
		} else {
			lines = append(lines, fmt.Sprintf("custom %s message of chat %d was refused", event, chatID))
		}
	}
	return lines
}

// lockdownSafeAlertLift lets the worker post the alert of the env's active lockdown in staffChat and has the env's
// administrator press the alert's Lift and then the card's Confirm, both in the Staff Group.
func lockdownSafeAlertLift(t *testing.T, env *lockdownEnv, staffChat gotgbot.Chat) {
	t.Helper()
	env.cycle()
	alerts := env.fake.sentTo(staffChat.Id)
	if len(alerts) != 1 {
		t.Fatalf("%d messages reached the Staff Group, want exactly the alert", len(alerts))
	}
	pressLockdownButton(env, env.admin, staffChat, alerts[0].MessageID, lockdownButtonData(t, alerts[0].Params["reply_markup"], "lk"))
	posted := env.fake.sentTo(staffChat.Id)
	if len(posted) != 2 {
		t.Fatalf("%d messages in the Staff Group, want the alert and one confirm card", len(posted))
	}
	pressLockdownButton(env, env.admin, staffChat, posted[1].MessageID, lockdownButtonData(t, posted[1].Params["reply_markup"], "lc"))
	raidPosts.wait()
}

func TestLockdownMessageLogsCarryIDsOnly(t *testing.T) {
	forbidden := func(extra ...string) []string {
		return append([]string{
			"ZZLOCKSAFETEXT", lockdownSafeFile, lockdownSafeButton, "zzlocksafe.example", lockdownSafeReason,
			lockdownSafeAdminName, lockdownSafeGroupTitle, html.EscapeString(lockdownSafeGroupTitle),
			lockdownSurfaceSentinel, lockdownSurfaceLabel,
		}, extra...)
	}

	for _, readError := range []bool{true, false} {
		mode := "refused"
		if readError {
			mode = "read error"
		}

		// The manual start and the lift by command: /lockdown with a reason, then /unlockdown, each sending the default
		// after the failure.
		t.Run("command "+mode, func(t *testing.T) {
			env := lockdownEventEnv(t, false)
			lockdownSafeRename(env)
			lockdownSafeStore(t, env.chat.Id, eventmsg.EventLockdown, eventmsg.EventLockLift)
			hook := mreqLogCapture(t)
			lockdownSafeBreak(t, env, readError, 2)

			env.send(env.admin, "/lockdown "+lockdownSafeReason)
			row := lockdownEventWantStarted(t, env, env.admin.Id)
			env.send(env.admin, "/unlockdown")
			lockdownLiftWantLifted(t, env, row, env.admin.Id)

			lockdownGoldenWantSends(t, env, env.chat.Id, 2)
			lockdownWantNoSentinel(t, lockdownCallsTo(lockdownSurfaceCalls(env.fake.moduleBotClient), env.chat.Id), "the defaults")
			mreqWantCleanLogs(t, hook, forbidden(),
				lockdownSafeWant(readError, env.chat.Id, eventmsg.EventLockdown, eventmsg.EventLockLift)...)
		})

		// The join surge and the lift by the alert's button pressed in the Staff Group: the automatic notice and the lift
		// notice each send the default after the failure. The trigger sentence is the notice's reason.
		t.Run("automatic "+mode, func(t *testing.T) {
			env := lockdownAutoEnv(t, false, true, true)
			lockdownSafeRename(env)
			staffChat := linkLockdownStaffGroup(env)
			lockdownSafeStore(t, env.chat.Id, eventmsg.EventLockJoinSurge, eventmsg.EventLockLift)
			hook := mreqLogCapture(t)
			lockdownSafeBreak(t, env, readError, 2)

			raidFire(env.bot, &env.chat, raidTrigger{Kind: models.LockdownTriggerJoinSurge, Count: 15, Window: 60})
			raidPosts.wait()
			row := lockdownEventWantStarted(t, env, env.bot.Id)
			lockdownSafeAlertLift(t, env, staffChat)
			lockdownLiftWantLifted(t, env, row, env.admin.Id)

			lockdownGoldenWantSends(t, env, env.chat.Id, 2)
			lockdownWantNoSentinel(t, lockdownCallsTo(lockdownSurfaceCalls(env.fake.moduleBotClient), env.chat.Id), "the defaults")
			sentence := lockdownTriggerSentence(staffChatTranslator(env.chat.Id), models.LockdownTriggerJoinSurge, 15, 60, 0)
			mreqWantCleanLogs(t, hook, forbidden(sentence),
				lockdownSafeWant(readError, env.chat.Id, eventmsg.EventLockJoinSurge, eventmsg.EventLockLift)...)
		})

		// The lift from a menu opened in the group: its default is switched off, so the failure posts nothing; the engine's
		// line after a read error still says the default was sent (it carries IDs only and is accepted).
		t.Run("group menu "+mode, func(t *testing.T) {
			env := lockdownLiftSettingsEnv(t, false)
			lockdownSafeRename(env.lockdownEnv)
			row := env.lockAsAdmin(lockdownSafeReason)
			lockdownSafeStore(t, env.chat.Id, eventmsg.EventLockLift)
			hook := mreqLogCapture(t)
			lockdownSafeBreak(t, env.lockdownEnv, readError, 1)

			_, before := lockdownLiftGroupMenuYes(t, env)
			lockdownLiftWantLifted(t, env.lockdownEnv, row, env.admin.Id)

			lockdownGoldenWantSends(t, env.lockdownEnv, env.chat.Id, before)
			mreqWantCleanLogs(t, hook, forbidden(), lockdownSafeWant(readError, env.chat.Id, eventmsg.EventLockLift)...)
		})
	}
}

// The sentinels of the privacy run: the Staff Group's title and the log channel's title. They carry no digit.
const (
	lockdownSafeStaffTitle = "ZZ Lock Staff & <Sentinel>"
	lockdownSafeLogTitle   = "ZZ Lock Log Channel"
)

// lockdownSafeEveryPlaceholder is a custom lockdown text with every Welcome and event placeholder a lockdown event fills
// (no {rules}, which would create a rules row outside this test's cleanup).
const lockdownSafeEveryPlaceholder = "{first}|{last}|{fullname}|{username}|{mention}|{id}|{chatname}|" +
	"{admin}|{reason}|{count}|{window}|{percent}"

// lockdownSafeLinkStaff makes a Staff Group titled lockdownSafeStaffTitle, owned by the env's administrator, and links the
// env's group to it; the staff rows and the alert rows go when the test ends.
func lockdownSafeLinkStaff(t *testing.T, env *lockdownEnv) gotgbot.Chat {
	t.Helper()
	staffChat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: lockdownSafeStaffTitle}
	staffCleanup(t, staffChat.Id, env.chat.Id)
	t.Cleanup(func() { db.DB.Where("chat_id = ?", env.chat.Id).Delete(&models.ChatLockdownAlert{}) })
	if _, err := staff.CreateStaffGroup(staffChat.Id, env.admin.Id, staffChat.Title); err != nil {
		t.Fatalf("create the Staff Group: %v", err)
	}
	linkLockdownGroupTo(env, staffChat)
	return staffChat
}

// lockdownSafeLogChannel gives the env's group a log channel with every category on, known to the bot by the sentinel
// title lockdownSafeLogTitle; the link and the channel's chats row go when the test ends. It returns the channel's ID.
func lockdownSafeLogChannel(t *testing.T, env *lockdownEnv) int64 {
	t.Helper()
	channel := uniqueModuleChatID()
	setStaffLogChannel(t, env.chat.Id, channel)
	if err := chats.EnsureChatInDb(channel, lockdownSafeLogTitle); err != nil {
		t.Fatalf("record the log channel's title: %v", err)
	}
	t.Cleanup(func() {
		db.DB.Where("chat_id = ?", channel).Delete(&models.Chat{})
		dbcache.DeleteCache(dbcache.CacheKey("chat", channel))
		dbcache.ResetLocalForTest()
	})
	return channel
}

// lockdownSafeWantPrivate fails unless the group has received exactly n custom lockdown messages (texts with every
// placeholder of lockdownSafeEveryPlaceholder filled), each naming the locked group's own title as {chatname}, and unless
// no request the bot made to the locked group names the Staff Group or the log channel.
func lockdownSafeWantPrivate(t *testing.T, env *lockdownEnv, staffChat gotgbot.Chat, channel int64, n int) {
	t.Helper()
	var texts []string
	for _, sent := range env.fake.sentTo(env.chat.Id) {
		if text := fmt.Sprint(sent.Params["text"]); strings.Count(text, "|") == strings.Count(lockdownSafeEveryPlaceholder, "|") {
			texts = append(texts, text)
		}
	}
	if len(texts) != n {
		t.Fatalf("the group received %d custom lockdown messages %q, want %d", len(texts), texts, n)
	}
	for _, text := range texts {
		if fields := strings.Split(text, "|"); fields[6] != html.EscapeString(env.chat.Title) {
			t.Errorf("the custom lockdown message %q names %q as {chatname}, want the locked group %q", text, fields[6], env.chat.Title)
		}
	}
	leaks := []string{
		lockdownSafeStaffTitle, html.EscapeString(lockdownSafeStaffTitle), "ZZ Lock Staff",
		strings.TrimPrefix(strconv.FormatInt(staffChat.Id, 10), "-"),
		lockdownSafeLogTitle, strings.TrimPrefix(strconv.FormatInt(channel, 10), "-"),
	}
	for _, call := range lockdownCallsTo(lockdownSurfaceCalls(env.fake.moduleBotClient), env.chat.Id) {
		screenSafeWantClean(t, "a request to the locked group", lockdownSurfaceRequest(call), leaks)
	}
}

func TestLockdownMessagePrivacy(t *testing.T) {
	// A join surge locks the group and the alert's Lift is pressed in the Staff Group. A lift starts the triggers' quiet
	// period, so the automatic start comes first.
	t.Run("join surge and the alert's lift", func(t *testing.T) {
		env := lockdownAutoEnv(t, false, true, true)
		lockdownSafeRename(env)
		staffChat := lockdownSafeLinkStaff(t, env)
		channel := lockdownSafeLogChannel(t, env)
		eventMsgStore(t, env.chat.Id, eventmsg.EventLockJoinSurge, lockdownSafeEveryPlaceholder)
		eventMsgStore(t, env.chat.Id, eventmsg.EventLockLift, lockdownSafeEveryPlaceholder)

		raidFire(env.bot, &env.chat, raidTrigger{Kind: models.LockdownTriggerJoinSurge, Count: 15, Window: 60})
		raidPosts.wait()
		row := lockdownEventWantStarted(t, env, env.bot.Id)
		lockdownSafeWantPrivate(t, env, staffChat, channel, 1)

		lockdownSafeAlertLift(t, env, staffChat)
		lockdownLiftWantLifted(t, env, row, env.admin.Id)
		lockdownSafeWantPrivate(t, env, staffChat, channel, 2)
	})

	// /lockdown with a reason, then the lift from a menu opened in the group.
	t.Run("command and the group menu's lift", func(t *testing.T) {
		env := lockdownLiftSettingsEnv(t, false)
		lockdownSafeRename(env.lockdownEnv)
		staffChat := lockdownSafeLinkStaff(t, env.lockdownEnv)
		channel := lockdownSafeLogChannel(t, env.lockdownEnv)
		eventMsgStore(t, env.chat.Id, eventmsg.EventLockdown, lockdownSafeEveryPlaceholder)
		eventMsgStore(t, env.chat.Id, eventmsg.EventLockLift, lockdownSafeEveryPlaceholder)

		row := env.lockAsAdmin("raid & co")
		lockdownLiftGroupMenuYes(t, env)
		lockdownLiftWantLifted(t, env.lockdownEnv, row, env.admin.Id)
		lockdownSafeWantPrivate(t, env.lockdownEnv, staffChat, channel, 2)
	})
}
