//go:build testtools

package modules

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/lockdown"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
)

// Goldens recorded on the code as it was before Phase 25 (the /lockdown success reply, the
// automatic lockdown's public notice, the cannot-lock warning and the public lift notice from
// every door), in the group's default language with the real locale files. <chat> stands for
// the locked group's chat ID, <admin> for the administrator who ran the command or pressed the
// button, <tapper> for the anonymous admin's proof tapper, <staff> for the Staff Group and <log>
// for the log channel. They are never re-recorded: a later change that alters one of these
// requests for a group with nothing stored is a regression.
const (
	goldenLockdownStart                = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"🔒 Ця група під локдауном. Писати можуть лише адміни, а кожного, хто приєднається, буде вилучено, доки адмін не виконає /unlockdown.\\nСхвалені користувачі також не можуть писати.\\nХто заблокував: \\u003ca href=\\\"tg://user?id=<admin>\\\"\\u003eAd\\u0026lt;b\\u0026gt;min\\u003c/a\\u003e.\"}"
	goldenLockdownStartReasonNoRights  = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"🔒 Ця група під локдауном. Писати можуть лише адміни, а кожного, хто приєднається, буде вилучено, доки адмін не виконає /unlockdown.\\nСхвалені користувачі також не можуть писати.\\nХто заблокував: \\u003ca href=\\\"tg://user?id=<admin>\\\"\\u003eAd\\u0026lt;b\\u0026gt;min\\u003c/a\\u003e.\\nПричина: raid \\u0026amp; \\u0026lt;b\\u0026gt;spam\\u0026lt;/b\\u0026gt;\\nЯ не можу тут видаляти повідомлення, тож повідомлення про вступ залишаться.\\nЯ не можу тут керувати запитами на вступ, тож нові запити залишаються в очікуванні, а не відхиляються.\"}"
	goldenLockdownStartAnonymous       = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"🔒 Ця група під локдауном. Писати можуть лише адміни, а кожного, хто приєднається, буде вилучено, доки адмін не виконає /unlockdown.\\nСхвалені користувачі також не можуть писати.\\nХто заблокував: \\u003ca href=\\\"tg://user?id=<tapper>\\\"\\u003eTap\\u0026lt;per\\u003c/a\\u003e.\"}"
	goldenLockdownAutoJoinSurge        = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"text\":\"🔒 Групу заблоковано автоматично: за 60 с приєдналося 15 людей. Писати можуть лише адміни, а кожного, хто приєднається, буде вилучено, доки адмін не виконає /unlockdown.\\nСхвалені користувачі також не можуть писати.\"}"
	goldenLockdownAutoFloodNoDelete    = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"text\":\"🔒 Групу заблоковано автоматично: за 30 с надійшло 30 повідомлень, 80% із них від нових учасників. Писати можуть лише адміни, а кожного, хто приєднається, буде вилучено, доки адмін не виконає /unlockdown.\\nСхвалені користувачі також не можуть писати.\\nЯ не можу тут видаляти повідомлення, тож повідомлення про вступ залишаться.\"}"
	goldenLockdownAutoAINoInvite       = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"text\":\"🔒 Групу заблоковано автоматично: за 120 с 5 повідомлень визнано спамом. Писати можуть лише адміни, а кожного, хто приєднається, буде вилучено, доки адмін не виконає /unlockdown.\\nСхвалені користувачі також не можуть писати.\\nЯ не можу тут керувати запитами на вступ, тож нові запити залишаються в очікуванні, а не відхиляються.\"}"
	goldenLockdownCannotLockStaff      = "{\"chat_id\":<staff>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"text\":\"⚠️ Виявлено рейд у \\u003cb\\u003eLock Chat\\u003c/b\\u003e (Наплив вступів: 15 за 60 с.), але заблокувати групу не можу: я маю бути адміном із правом обмежувати учасників.\"}"
	goldenLockdownCannotLockLog        = "{\"chat_id\":<log>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"text\":\"\\u003cb\\u003eLock Chat\\u003c/b\\u003e (\\u003ccode\\u003e<chat>\\u003c/code\\u003e)\\n⚠️ Виявлено рейд у \\u003cb\\u003eLock Chat\\u003c/b\\u003e (Наплив вступів: 15 за 60 с.), але заблокувати групу не можу: я маю бути адміном із правом обмежувати учасників.\"}"
	goldenLockdownLiftCommand          = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"message_id\":102,\"allow_sending_without_reply\":true},\"text\":\"🔓 Локдаун знято. Хто зняв: \\u003ca href=\\\"tg://user?id=<admin>\\\"\\u003eAd\\u0026lt;b\\u0026gt;min\\u003c/a\\u003e. Дозволи групи повернуто до стану до локдауну.\"}"
	goldenLockdownLiftCommandUnbanning = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"message_id\":102,\"allow_sending_without_reply\":true},\"text\":\"🔓 Локдаун знято. Хто зняв: \\u003ca href=\\\"tg://user?id=<admin>\\\"\\u003eAd\\u0026lt;b\\u0026gt;min\\u003c/a\\u003e. Дозволи групи повернуто до стану до локдауну.\\nХтось вручну змінив дозволи групи під час локдауну; повернуто ті, що були до локдауну.\\nЗараз розбанюю вилучених: 2; напишу тут, коли закінчу.\"}"
	goldenLockdownLiftAlert            = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"text\":\"🔓 Локдаун знято. Хто зняв: \\u003ca href=\\\"tg://user?id=<admin>\\\"\\u003eAd\\u0026lt;b\\u0026gt;min\\u003c/a\\u003e. Дозволи групи повернуто до стану до локдауну.\"}"
	goldenLockdownLiftMenuPrivate      = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"text\":\"🔓 Локдаун знято. Хто зняв: \\u003ca href=\\\"tg://user?id=<admin>\\\"\\u003eAd\\u0026lt;b\\u0026gt;min\\u003c/a\\u003e. Дозволи групи повернуто до стану до локдауну.\"}"
)

// lockdownGoldenChangedPerms is a live permission set that differs from the locked one, so a
// lift reads it as changed by hand while the lockdown ran.
const lockdownGoldenChangedPerms = `{"can_send_messages":true,"can_send_audios":false,"can_send_documents":false,` +
	`"can_send_photos":false,"can_send_videos":false,"can_send_video_notes":false,"can_send_voice_notes":false,` +
	`"can_send_polls":false,"can_send_other_messages":false,"can_add_web_page_previews":false,` +
	`"can_react_to_messages":false,"can_edit_tag":false,"can_change_info":false,"can_invite_users":false,` +
	`"can_pin_messages":false,"can_manage_topics":false}`

// newLockdownGoldenEnv is the lockdown environment with the real locale files and every
// deletion rule of its chat at the shortest delay that kind accepts, so a lockdown message
// that entered the deletion queue would leave a row.
func newLockdownGoldenEnv(t *testing.T) *lockdownEnv {
	t.Helper()
	env := newRealLocaleLockdownEnv(t)
	raidCleanup(t, env.chat.Id)
	deletionShortestRules(t, env.chat.Id)
	return env
}

// newLockdownGoldenSettingsEnv is the settings environment with the real locale files and
// every deletion rule of its chat at the shortest delay, for the lifts from the menu.
func newLockdownGoldenSettingsEnv(t *testing.T) *settingsEnv {
	t.Helper()
	env := newSettingsEnvWith(t, settingsEnvOptions{RealLocale: true})
	raidCleanup(t, env.chat.Id)
	deletionShortestRules(t, env.chat.Id)
	return env
}

// lockdownGoldenTokens replaces each chat or user ID of a serialised request with its token.
// The longer IDs are replaced first, so an ID that sits inside a longer one is never matched
// there.
func lockdownGoldenTokens(raw string, ids map[int64]string) string {
	keys := make([]string, 0, len(ids))
	tokens := make(map[string]string, len(ids))
	for id, token := range ids {
		key := strconv.FormatInt(id, 10)
		keys = append(keys, key)
		tokens[key] = token
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) > len(keys[j])
		}
		return keys[i] < keys[j]
	})
	for _, key := range keys {
		raw = strings.ReplaceAll(raw, key, tokens[key])
	}
	return raw
}

// lockdownGoldenRequest is the n-th sendMessage (from 0) the fake answered for chatID,
// serialised with legacyRequestJSON. It fails the test when there are not that many.
func lockdownGoldenRequest(t *testing.T, env *lockdownEnv, chatID int64, n int) string {
	t.Helper()
	sent := env.fake.sentTo(chatID)
	if len(sent) <= n {
		t.Fatalf("chat %d received %d messages, want at least %d", chatID, len(sent), n+1)
	}
	return legacyRequestJSON(t, moduleBotCall{Method: "sendMessage", Params: sent[n].Params})
}

// lockdownGoldenWant fails unless got, with its IDs replaced by tokens, equals want.
func lockdownGoldenWant(t *testing.T, what, got string, ids map[int64]string, want string) {
	t.Helper()
	if tokened := lockdownGoldenTokens(got, ids); tokened != want {
		t.Fatalf("%s request changed\n got %q\nwant %q", what, tokened, want)
	}
}

// lockdownGoldenWantSends fails unless chatID received exactly want messages.
func lockdownGoldenWantSends(t *testing.T, env *lockdownEnv, chatID int64, want int) {
	t.Helper()
	if got := len(env.fake.sentTo(chatID)); got != want {
		t.Fatalf("chat %d received %d messages, want %d", chatID, got, want)
	}
}

// lockdownGoldenNoQueue fails when the chat has a scheduled_deletions row.
func lockdownGoldenNoQueue(t *testing.T, chatID int64) {
	t.Helper()
	var rows []models.ScheduledDeletion
	if err := db.DB.Where("chat_id = ?", chatID).Find(&rows).Error; err != nil {
		t.Fatalf("read the deletion queue: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("deletion queue rows of chat %d = %+v, want none", chatID, rows)
	}
}

// lockdownGoldenNoPrivate fails when any request the fake recorded names a positive chat ID,
// that is a private chat.
func lockdownGoldenNoPrivate(t *testing.T, env *lockdownEnv) {
	t.Helper()
	client := env.fake.moduleBotClient
	raidAIJobs.wait()
	raidAIFires.wait()
	raidPosts.wait()
	client.mu.Lock()
	defer client.mu.Unlock()
	for _, call := range client.calls {
		if _, ok := call.Params["chat_id"]; !ok {
			continue
		}
		if staffParamInt(call.Params, "chat_id") > 0 {
			t.Fatalf("%s went to the private chat %v, want no private request", call.Method, call.Params["chat_id"])
		}
	}
}

// lockdownGoldenSettingsButton returns the button of keyboard whose settings data carries the
// action act, failing the test when there is none.
func lockdownGoldenSettingsButton(t *testing.T, keyboard [][]gotgbot.InlineKeyboardButton, act string) gotgbot.InlineKeyboardButton {
	t.Helper()
	for _, button := range settingsAllButtons(keyboard) {
		decoded, ok := decodeCallbackData(button.CallbackData, settingsCallbackNamespace)
		if ok && decoded.Fields["a"] == act {
			return button
		}
	}
	t.Fatalf("keyboard %+v has no settings button for action %q", keyboard, act)
	return gotgbot.InlineKeyboardButton{}
}

// lockdownGoldenWantLifted fails unless the chat's lockdown row was lifted.
func lockdownGoldenWantLifted(t *testing.T, env *lockdownEnv, row *models.ChatLockdown) {
	t.Helper()
	lifted := env.freshRow(row.ID)
	if lifted.LiftedBy == nil || lifted.State == models.LockdownStateActive {
		t.Fatalf("lockdown row = %+v, want it lifted", lifted)
	}
}

// lockdownGoldenAutoCase is one automatic notice case of TestLegacyLockdownMessageRequests.
type lockdownGoldenAutoCase struct {
	name      string
	trigger   raidTrigger
	canDelete bool
	canInvite bool
	golden    string
}

func TestLegacyLockdownMessageRequests(t *testing.T) {
	t.Run("start", func(t *testing.T) {
		env := newLockdownGoldenEnv(t)
		env.fake.setBotMember(env.chat.Id, lockdownBotAdminJSON(true, true, true))

		env.send(env.admin, "/lockdown")

		if row, err := lockdown.GetActiveFresh(env.chat.Id); err != nil || row == nil || row.LockedAt == nil {
			t.Fatalf("lockdown = %+v, %v, want one confirmed", row, err)
		}
		lockdownGoldenWantSends(t, env, env.chat.Id, 1)
		ids := map[int64]string{env.chat.Id: "<chat>", env.admin.Id: "<admin>"}
		lockdownGoldenWant(t, "/lockdown reply", lockdownGoldenRequest(t, env, env.chat.Id, 0), ids, goldenLockdownStart)
		lockdownGoldenNoQueue(t, env.chat.Id)
		lockdownGoldenNoPrivate(t, env)
	})

	t.Run("start_reason_no_rights", func(t *testing.T) {
		env := newLockdownGoldenEnv(t)
		env.fake.setBotMember(env.chat.Id, lockdownBotAdminJSON(true, false, false))

		env.send(env.admin, "/lockdown raid & <b>spam</b>")

		if row, err := lockdown.GetActiveFresh(env.chat.Id); err != nil || row == nil || row.LockedAt == nil {
			t.Fatalf("lockdown = %+v, %v, want one confirmed", row, err)
		}
		lockdownGoldenWantSends(t, env, env.chat.Id, 1)
		ids := map[int64]string{env.chat.Id: "<chat>", env.admin.Id: "<admin>"}
		lockdownGoldenWant(t, "/lockdown reply", lockdownGoldenRequest(t, env, env.chat.Id, 0), ids, goldenLockdownStartReasonNoRights)
		lockdownGoldenNoQueue(t, env.chat.Id)
		lockdownGoldenNoPrivate(t, env)
	})

	t.Run("start_anonymous", func(t *testing.T) {
		env := newLockdownGoldenEnv(t)
		LoadBotUpdates(env.dispatcher)
		env.fake.setBotMember(env.chat.Id, lockdownBotAdminJSON(true, true, true))
		tapper := gotgbot.User{Id: uniqueLinkOwnerID(), FirstName: "Tap<per"}
		env.fake.setMember(env.chat.Id, tapper.Id, staffFakeMember{
			Status:             gotgbot.ChatMemberStatusAdministrator,
			CanRestrictMembers: true,
		})
		env.fake.listAdministrator(env.chat.Id, tapper.Id)

		env.sendAnonymous("/lockdown")
		env.tapProof(tapper)

		row, err := lockdown.GetActiveFresh(env.chat.Id)
		if err != nil || row == nil || row.LockedAt == nil || row.StartedBy != tapper.Id {
			t.Fatalf("lockdown = %+v, %v, want one confirmed and started by the tapper", row, err)
		}
		sent := env.fake.sentTo(env.chat.Id)
		if len(sent) < 2 {
			t.Fatalf("the group received %d messages, want the proof prompt and the reply", len(sent))
		}
		ids := map[int64]string{env.chat.Id: "<chat>", tapper.Id: "<tapper>"}
		lockdownGoldenWant(t, "/lockdown reply after the proof", lockdownGoldenRequest(t, env, env.chat.Id, len(sent)-1), ids, goldenLockdownStartAnonymous)
		lockdownGoldenNoQueue(t, env.chat.Id)
		lockdownGoldenNoPrivate(t, env)
	})

	for _, tc := range []lockdownGoldenAutoCase{
		{name: "auto_join_surge", trigger: raidTrigger{Kind: models.LockdownTriggerJoinSurge, Count: 15, Window: 60},
			canDelete: true, canInvite: true, golden: goldenLockdownAutoJoinSurge},
		{name: "auto_flood_no_delete", trigger: raidTrigger{Kind: models.LockdownTriggerFlood, Count: 30, Window: 30, Percent: 80},
			canDelete: false, canInvite: true, golden: goldenLockdownAutoFloodNoDelete},
		{name: "auto_ai_no_invite", trigger: raidTrigger{Kind: models.LockdownTriggerAIBurst, Count: 5, Window: 120},
			canDelete: true, canInvite: false, golden: goldenLockdownAutoAINoInvite},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := newLockdownGoldenEnv(t)
			env.fake.setBotMember(env.chat.Id, lockdownBotAdminJSON(true, tc.canDelete, tc.canInvite))

			raidFire(env.bot, &env.chat, tc.trigger)
			raidPosts.wait()

			row, err := lockdown.GetActiveFresh(env.chat.Id)
			if err != nil || row == nil || row.LockedAt == nil || row.TriggerKind != tc.trigger.Kind {
				t.Fatalf("lockdown = %+v, %v, want one confirmed by the %s trigger", row, err, tc.trigger.Kind)
			}
			lockdownGoldenWantSends(t, env, env.chat.Id, 1)
			ids := map[int64]string{env.chat.Id: "<chat>"}
			lockdownGoldenWant(t, "automatic notice", lockdownGoldenRequest(t, env, env.chat.Id, 0), ids, tc.golden)
			lockdownGoldenNoQueue(t, env.chat.Id)
			lockdownGoldenNoPrivate(t, env)
		})
	}

	t.Run("cannot_lock", func(t *testing.T) {
		env := newLockdownGoldenEnv(t)
		staffChat := linkLockdownStaffGroup(env)
		channel := uniqueModuleChatID()
		setStaffLogChannel(t, env.chat.Id, channel)
		env.fake.setBotMember(env.chat.Id, lockdownBotAdminJSON(false, true, true))

		raidFire(env.bot, &env.chat, raidTrigger{Kind: models.LockdownTriggerJoinSurge, Count: 15, Window: 60})
		raidPosts.wait()

		if row, err := lockdown.GetActiveFresh(env.chat.Id); err != nil || row != nil {
			t.Fatalf("lockdown = %+v, %v, want none: the bot cannot restrict members", row, err)
		}
		lockdownGoldenWantSends(t, env, env.chat.Id, 0)
		lockdownGoldenWantSends(t, env, staffChat.Id, 1)
		lockdownGoldenWantSends(t, env, channel, 1)
		ids := map[int64]string{env.chat.Id: "<chat>", staffChat.Id: "<staff>", channel: "<log>"}
		lockdownGoldenWant(t, "Staff Group warning", lockdownGoldenRequest(t, env, staffChat.Id, 0), ids, goldenLockdownCannotLockStaff)
		lockdownGoldenWant(t, "log channel warning", lockdownGoldenRequest(t, env, channel, 0), ids, goldenLockdownCannotLockLog)
		lockdownGoldenNoQueue(t, env.chat.Id)
		lockdownGoldenNoPrivate(t, env)
	})

	t.Run("lift_command", func(t *testing.T) {
		env := newLockdownGoldenEnv(t)
		row := env.lockAsAdmin("raid")

		env.send(env.admin, "/unlockdown")

		lockdownGoldenWantLifted(t, env, row)
		lockdownGoldenWantSends(t, env, env.chat.Id, 2)
		ids := map[int64]string{env.chat.Id: "<chat>", env.admin.Id: "<admin>"}
		lockdownGoldenWant(t, "/unlockdown reply", lockdownGoldenRequest(t, env, env.chat.Id, 1), ids, goldenLockdownLiftCommand)
		lockdownGoldenNoQueue(t, env.chat.Id)
		lockdownGoldenNoPrivate(t, env)
	})

	t.Run("lift_command_unbanning", func(t *testing.T) {
		env := newLockdownGoldenEnv(t)
		row := env.lockAsAdmin("raid")
		seedLockdownJoiner(t, row.ID, env.chat.Id, uniqueLinkOwnerID(), models.JoinerStateBanned)
		seedLockdownJoiner(t, row.ID, env.chat.Id, uniqueLinkOwnerID(), models.JoinerStateBanned)
		env.fake.setChatPermsRaw(env.chat.Id, lockdownGoldenChangedPerms)

		env.send(env.admin, "/unlockdown")

		lifted := env.freshRow(row.ID)
		if lifted.LiftedBy == nil || !lifted.ManualChange {
			t.Fatalf("lockdown row = %+v, want a lift recorded with a manual change", lifted)
		}
		lockdownGoldenWantSends(t, env, env.chat.Id, 2)
		ids := map[int64]string{env.chat.Id: "<chat>", env.admin.Id: "<admin>"}
		lockdownGoldenWant(t, "/unlockdown reply", lockdownGoldenRequest(t, env, env.chat.Id, 1), ids, goldenLockdownLiftCommandUnbanning)
		lockdownGoldenNoQueue(t, env.chat.Id)
		lockdownGoldenNoPrivate(t, env)
	})

	t.Run("lift_alert", func(t *testing.T) {
		env := newLockdownGoldenEnv(t)
		card := askToLiftLockdown(t, env)

		pressLockdownButton(env, env.admin, card.staffChat, card.msgID, card.confirm)

		lockdownGoldenWantLifted(t, env, card.row)
		lockdownGoldenWantSends(t, env, env.chat.Id, 2)
		ids := map[int64]string{env.chat.Id: "<chat>", env.admin.Id: "<admin>", card.staffChat.Id: "<staff>"}
		lockdownGoldenWant(t, "alert lift notice", lockdownGoldenRequest(t, env, env.chat.Id, 1), ids, goldenLockdownLiftAlert)
		lockdownGoldenNoQueue(t, env.chat.Id)
		lockdownGoldenNoPrivate(t, env)
	})

	t.Run("lift_menu_private", func(t *testing.T) {
		env := newLockdownGoldenSettingsEnv(t)
		row := env.lockAsAdmin("raid")
		env.sendPrivate(env.admin, fmt.Sprintf("/start settings_%d", env.chat.Id))
		privateMsgID := settingsPrivateOnlySent(t, env, env.admin).MessageID
		private := settingsPrivateChat(env.admin)
		env.pressPrivate(env.admin, privateMsgID, env.privateFields(env.chat.Id, "a", "op", "p", settingsPageLockdown))
		_, keyboard := env.lastEdit(env.admin.Id, privateMsgID)
		env.pressRaw(private, env.admin, privateMsgID, lockdownGoldenSettingsButton(t, keyboard, "ls").CallbackData)
		_, card := env.lastEdit(env.admin.Id, privateMsgID)

		env.pressRaw(private, env.admin, privateMsgID, lockdownGoldenSettingsButton(t, card, "ly").CallbackData)

		lockdownGoldenWantLifted(t, env.lockdownEnv, row)
		lockdownGoldenWantSends(t, env.lockdownEnv, env.chat.Id, 2)
		ids := map[int64]string{env.chat.Id: "<chat>", env.admin.Id: "<admin>"}
		lockdownGoldenWant(t, "private menu lift notice", lockdownGoldenRequest(t, env.lockdownEnv, env.chat.Id, 1), ids, goldenLockdownLiftMenuPrivate)
		lockdownGoldenNoQueue(t, env.chat.Id)
	})

	t.Run("lift_menu_group", func(t *testing.T) {
		env := newLockdownGoldenSettingsEnv(t)
		row := env.lockAsAdmin("raid")
		msgID := settingsLockdownTestOpen(t, env)
		_, keyboard := env.lastEdit(env.chat.Id, msgID)
		env.pressRaw(env.chat, env.admin, msgID, lockdownGoldenSettingsButton(t, keyboard, "ls").CallbackData)
		_, card := env.lastEdit(env.chat.Id, msgID)
		before := len(env.fake.sentTo(env.chat.Id))
		editsBefore := len(env.edits(env.chat.Id, msgID))

		env.pressRaw(env.chat, env.admin, msgID, lockdownGoldenSettingsButton(t, card, "ly").CallbackData)

		lockdownGoldenWantLifted(t, env.lockdownEnv, row)
		// The /lockdown reply and the /settings choice message, which became the menu.
		if before != 2 {
			t.Fatalf("the group received %d messages before the Yes, want the /lockdown reply and the menu", before)
		}
		lockdownGoldenWantSends(t, env.lockdownEnv, env.chat.Id, before)
		if got := len(env.edits(env.chat.Id, msgID)); got <= editsBefore {
			t.Fatalf("the menu was edited %d times after the Yes, want the redraw", got-editsBefore)
		}
		lockdownGoldenNoQueue(t, env.chat.Id)
		lockdownGoldenNoPrivate(t, env.lockdownEnv)
	})
}
