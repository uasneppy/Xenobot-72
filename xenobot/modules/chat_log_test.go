package modules

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/redis/go-redis/v9"

	"github.com/uasneppy/Xenobot-72/xenobot/db/chatconv"
)

func chatLogTestEntry(id, user int64, name, text string) chatLogEntry {
	return chatLogEntry{MessageID: id, UserID: user, Name: name, Text: text}
}

func chatLogIDs(t *testing.T, client *redis.Client, chatID int64) []int64 {
	t.Helper()
	entries, err := chatLogSince(context.Background(), client, chatID, 0)
	if err != nil {
		t.Fatalf("chatLogSince: %v", err)
	}
	ids := make([]int64, len(entries))
	for i, e := range entries {
		ids[i] = e.MessageID
	}
	return ids
}

func chatLogWantIDs(t *testing.T, got []int64, want ...int64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("ring holds %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ring holds %v, want %v", got, want)
		}
	}
}

// TestChatLogRing proves the ring: order by message ID, the oldest dropped one by one, a redelivery
// replacing instead of duplicating, an edit updating in place and never inserting, presence and
// reads from an ID, the idle expiry, and one chat never touching another.
func TestChatLogRing(t *testing.T) {
	mr, client := withChatRedis(t)
	chatSwap(t, &chatLogMax, 3)
	ctx := context.Background()
	chatID := uniqueModuleChatID()
	other := chatID - 1

	for _, id := range []int64{12, 10, 11} { // out of order: the score orders them
		if err := chatLogRecord(client, chatID, chatLogTestEntry(id, 5, "Ann", "line "+strconv.FormatInt(id, 10))); err != nil {
			t.Fatal(err)
		}
	}
	chatLogWantIDs(t, chatLogIDs(t, client, chatID), 10, 11, 12)
	if err := chatLogRecord(client, other, chatLogTestEntry(10, 6, "Bo", "other chat")); err != nil {
		t.Fatal(err)
	}

	// The oldest is dropped one by one, and a late message older than the ring is dropped itself.
	if err := chatLogRecord(client, chatID, chatLogTestEntry(13, 5, "Ann", "new")); err != nil {
		t.Fatal(err)
	}
	chatLogWantIDs(t, chatLogIDs(t, client, chatID), 11, 12, 13)
	if err := chatLogRecord(client, chatID, chatLogTestEntry(5, 5, "Ann", "ancient")); err != nil {
		t.Fatal(err)
	}
	chatLogWantIDs(t, chatLogIDs(t, client, chatID), 11, 12, 13)
	if has, _ := chatLogHas(ctx, client, chatID, 10); has {
		t.Error("an entry that left the ring is still present")
	}
	if has, err := chatLogHas(ctx, client, chatID, 12); err != nil || !has {
		t.Errorf("chatLogHas(12) = %v, %v", has, err)
	}

	// The same ID replaces.
	if err := chatLogRecord(client, chatID, chatLogTestEntry(12, 5, "Ann", "redelivered")); err != nil {
		t.Fatal(err)
	}
	since, _ := chatLogSince(ctx, client, chatID, 12)
	if len(since) != 2 || since[0].MessageID != 12 || since[0].Text != "redelivered" || since[1].MessageID != 13 {
		t.Fatalf("since 12 = %+v", since)
	}

	// An edit updates in place and marks the entry; one of an absent ID inserts nothing.
	if replaced, err := chatLogEdit(client, chatID, chatLogTestEntry(12, 5, "Ann", "edited text")); err != nil || !replaced {
		t.Fatalf("edit = %v, %v", replaced, err)
	}
	since, _ = chatLogSince(ctx, client, chatID, 12)
	if since[0].Text != "edited text" || !since[0].Edited || since[1].Edited {
		t.Fatalf("after the edit: %+v", since)
	}
	if replaced, err := chatLogEdit(client, chatID, chatLogTestEntry(99, 5, "Ann", "never logged")); err != nil || replaced {
		t.Fatalf("edit of an absent ID = %v, %v", replaced, err)
	}
	chatLogWantIDs(t, chatLogIDs(t, client, chatID), 11, 12, 13)

	if ttl := mr.TTL(chatLogKey(chatID)); ttl <= 0 || ttl > chatLogTTL {
		t.Errorf("ring TTL = %v, want within the idle expiry", ttl)
	}
	chatLogWantIDs(t, chatLogIDs(t, client, other), 10)
}

func TestChatLogMediaTag(t *testing.T) {
	cases := []struct {
		name string
		msg  gotgbot.Message
		want string
	}{
		{"none", gotgbot.Message{Text: "hi"}, ""},
		{"gif before document", gotgbot.Message{Animation: &gotgbot.Animation{}, Document: &gotgbot.Document{}}, "[GIF]"},
		{"sticker", gotgbot.Message{Sticker: &gotgbot.Sticker{}}, "[sticker]"},
		{"photo", gotgbot.Message{Photo: []gotgbot.PhotoSize{{}}}, "[photo]"},
		{"video", gotgbot.Message{Video: &gotgbot.Video{}}, "[video]"},
		{"video note", gotgbot.Message{VideoNote: &gotgbot.VideoNote{}}, "[video note]"},
		{"voice", gotgbot.Message{Voice: &gotgbot.Voice{}}, "[voice]"},
		{"audio", gotgbot.Message{Audio: &gotgbot.Audio{}}, "[audio]"},
		{"document", gotgbot.Message{Document: &gotgbot.Document{}}, "[file]"},
		{"poll", gotgbot.Message{Poll: &gotgbot.Poll{}}, "[poll]"},
		{"contact", gotgbot.Message{Contact: &gotgbot.Contact{}}, "[contact]"},
		{"location", gotgbot.Message{Location: &gotgbot.Location{}}, "[location]"},
		{"venue", gotgbot.Message{Venue: &gotgbot.Venue{}}, "[location]"},
		{"dice", gotgbot.Message{Dice: &gotgbot.Dice{}}, "[dice]"},
		{"story", gotgbot.Message{Story: &gotgbot.Story{}}, "[story]"},
		{"game", gotgbot.Message{Game: &gotgbot.Game{}}, "[game]"},
	}
	for _, tc := range cases {
		if got := chatLogMediaTag(&tc.msg); got != tc.want {
			t.Errorf("%s: tag %q, want %q", tc.name, got, tc.want)
		}
	}
}

func chatLogTurnOn(t *testing.T, env *chatEnv) {
	t.Helper()
	if err := chatconv.SetLogEnabled(context.Background(), env.chatID, true, 1); err != nil {
		t.Fatalf("SetLogEnabled: %v", err)
	}
}

func (e *chatEnv) logged() []chatLogEntry {
	e.t.Helper()
	entries, err := chatLogSince(context.Background(), e.rc, e.chatID, 0)
	if err != nil {
		e.t.Fatalf("chatLogSince: %v", err)
	}
	return entries
}

// TestChatLogWatcher drives the real dispatcher: a text, a caption with a photo, a sticker and an
// edit are logged with the first name and user ID and nothing else; every message the log must
// not keep is skipped.
func TestChatLogWatcher(t *testing.T) {
	env := newChatEnv(t)
	chatLogTurnOn(t, env)
	alice := env.member("Alice")

	text := env.message(alice, "hello there")
	photo := env.message(alice, "")
	photo.Caption, photo.Photo = "look at this", []gotgbot.PhotoSize{{FileId: "f", FileUniqueId: "u", Width: 1, Height: 1}}
	sticker := env.message(alice, "")
	sticker.Sticker = &gotgbot.Sticker{FileId: "s", FileUniqueId: "su"}
	for _, msg := range []*gotgbot.Message{text, photo, sticker} {
		env.deliver(msg)
	}
	want := []chatLogEntry{
		{MessageID: text.MessageId, UserID: alice.Id, Name: "Alice", Text: "hello there"},
		{MessageID: photo.MessageId, UserID: alice.Id, Name: "Alice", Text: "look at this", Media: "[photo]"},
		{MessageID: sticker.MessageId, UserID: alice.Id, Name: "Alice", Media: "[sticker]"},
	}
	got := env.logged()
	if len(got) != len(want) {
		t.Fatalf("logged %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	// Nothing but the compact fields is stored: no username, chat ID or date.
	raw, _ := env.rc.ZRange(context.Background(), chatLogKey(env.chatID), 0, 0).Result()
	var fields map[string]any
	if err := json.Unmarshal([]byte(raw[0]), &fields); err != nil || len(fields) != 4 {
		t.Errorf("stored entry %s has %d fields, want m, u, n and t", raw[0], len(fields))
	}

	// An edit updates the stored line; an edit of a message that was never logged inserts nothing.
	edited := *text
	edited.Text = "hello again"
	env.deliverEdit(&edited)
	unknown := env.message(alice, "never seen")
	env.deliverEdit(unknown)
	got = env.logged()
	if len(got) != 3 || got[0].Text != "hello again" || !got[0].Edited {
		t.Fatalf("after the edits: %+v", got)
	}

	skipped := []struct {
		name  string
		build func() *gotgbot.Message
		setup func(t *testing.T)
	}{
		{name: "bot sender", build: func() *gotgbot.Message {
			return env.message(&gotgbot.User{Id: uniqueLinkOwnerID(), IsBot: true, FirstName: "Robot"}, "beep")
		}},
		{name: "anonymous admin", build: func() *gotgbot.Message {
			m := env.message(&gotgbot.User{Id: 1087968824, IsBot: true, FirstName: "Group"}, "as the group")
			m.SenderChat = &gotgbot.Chat{Id: env.chatID, Type: "supergroup"}
			return m
		}},
		{name: "service message", build: func() *gotgbot.Message {
			m := env.message(alice, "")
			m.NewChatMembers = []gotgbot.User{*alice}
			return m
		}},
		{name: "slash text", build: func() *gotgbot.Message {
			m := env.message(alice, "/nosuchcommand please")
			m.Entities = []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: 14}}
			return m
		}},
		{name: "private chat", build: func() *gotgbot.Message {
			m := env.message(alice, "private words")
			m.Chat = gotgbot.Chat{Id: alice.Id, Type: "private"}
			return m
		}},
		{name: "switch off", setup: func(t *testing.T) {
			if err := chatconv.SetLogEnabled(context.Background(), env.chatID, false, 1); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = chatconv.SetLogEnabled(context.Background(), env.chatID, true, 1) })
		}},
		{name: "provider not configured", setup: func(t *testing.T) { withChatProviders(t, true, false, false) }},
		{name: "ENABLE_CHAT off", setup: func(t *testing.T) { withChatProviders(t, false, true, true) }},
		{name: "settings read failure", setup: func(t *testing.T) {
			chatSwap(t, &chatSettingsLookup, func(context.Context, int64) (chatconv.Settings, error) {
				return chatconv.Settings{}, context.DeadlineExceeded
			})
		}},
		{name: "no Redis", setup: func(t *testing.T) { chatSwap(t, &chatRedisClient, func() *redis.Client { return nil }) }},
	}
	for _, tc := range skipped {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(t)
			}
			msg := env.message(alice, "should not be kept")
			if tc.build != nil {
				msg = tc.build()
			}
			env.deliver(msg)
			if n := len(env.logged()); n != 3 {
				t.Fatalf("%s: the ring holds %d entries, want the 3 it had", tc.name, n)
			}
		})
	}

	// A channel post is no member message.
	post := env.message(alice, "channel words")
	env.nextUpdate++
	if err := env.dispatcher.ProcessUpdate(env.bot, &gotgbot.Update{UpdateId: env.nextUpdate, ChannelPost: post}, nil); err != nil {
		t.Fatal(err)
	}
	if n := len(env.logged()); n != 3 {
		t.Fatalf("a channel post was logged: %d entries", n)
	}
}

// TestChatLogWatcherNeverEndsUpdate holds that a recorder at the next group sees every message, new
// and edited, also when the watcher panics.
func TestChatLogWatcherNeverEndsUpdate(t *testing.T) {
	env := newChatEnv(t)
	chatLogTurnOn(t, env)
	var seen []int64
	env.dispatcher.AddHandlerToGroup(chatGroupRecorder{seen: &seen}, chatLogGroup+1)
	alice := env.member("Alice")

	first := env.message(alice, "one")
	env.deliver(first)
	edited := *first
	edited.Text = "one, edited"
	env.deliverEdit(&edited)
	chatSwap(t, &chatSettingsLookup, func(context.Context, int64) (chatconv.Settings, error) { panic("settings exploded") })
	second := env.message(alice, "two")
	env.deliver(second)

	want := []int64{first.MessageId, first.MessageId, second.MessageId}
	if len(seen) != len(want) {
		t.Fatalf("the next group saw %v, want %v", seen, want)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("the next group saw %v, want %v", seen, want)
		}
	}
}

// TestChatLogWatcherGroupReached loads every module: no handler before group 14 ends a member's
// text, caption or edit, so a recorder there sees all three.
func TestChatLogWatcherGroupReached(t *testing.T) {
	withRealLocales(t)
	withFreshHelpRegistry(t)

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadAllModules(dispatcher)
	var seen []int64
	dispatcher.AddHandlerToGroup(chatGroupRecorder{seen: &seen}, chatLogGroup)

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chatID := uniqueModuleChatID()
	userID := uniqueLinkOwnerID()
	memberReqCleanup(t, []int64{chatID}, []int64{userID})
	chat := gotgbot.Chat{Id: chatID, Type: "supergroup", Title: "Facility"}
	from := &gotgbot.User{Id: userID, FirstName: "Resident"}
	date := time.Now().Unix()

	updates := []gotgbot.Update{
		{UpdateId: 800, Message: &gotgbot.Message{MessageId: 201, Date: date, Chat: chat, From: from, Text: "plain words"}},
		{UpdateId: 801, Message: &gotgbot.Message{
			MessageId: 202, Date: date, Chat: chat, From: from, Caption: "a caption",
			Photo: []gotgbot.PhotoSize{{FileId: "p", FileUniqueId: "pu", Width: 10, Height: 10}},
		}},
		{UpdateId: 802, EditedMessage: &gotgbot.Message{MessageId: 201, Date: date, EditDate: date + 5, Chat: chat, From: from, Text: "plain words, edited"}},
	}
	for i := range updates {
		if err := dispatcher.ProcessUpdate(bot, &updates[i], nil); err != nil {
			t.Fatalf("ProcessUpdate(%d): %v", updates[i].UpdateId, err)
		}
	}
	want := []int64{201, 202, 201}
	if len(seen) != len(want) {
		t.Fatalf("group %d saw messages %v, want %v", chatLogGroup, seen, want)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("group %d saw messages %v, want %v", chatLogGroup, seen, want)
		}
	}
}
