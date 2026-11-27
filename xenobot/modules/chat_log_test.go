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
		{MessageID: text.MessageId, UserID: alice.Id, Name: "Alice", Text: "hello there", At: text.Date},
		{MessageID: photo.MessageId, UserID: alice.Id, Name: "Alice", Text: "look at this", Media: "[photo]", At: photo.Date},
		{MessageID: sticker.MessageId, UserID: alice.Id, Name: "Alice", Media: "[sticker]", At: sticker.Date},
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
	// Nothing but the compact fields is stored: no username or chat ID.
	raw, _ := env.rc.ZRange(context.Background(), chatLogKey(env.chatID), 0, 0).Result()
	var fields map[string]any
	if err := json.Unmarshal([]byte(raw[0]), &fields); err != nil || len(fields) != 5 {
		t.Errorf("stored entry %s has %d fields, want m, u, n, t and at", raw[0], len(fields))
	}

	// An edit updates the stored line; an edit of a message that was never logged inserts nothing.
	edited := *text
	edited.Text = "hello again"
	env.deliverEdit(&edited)
	unknown := env.message(alice, "never seen")
	env.deliverEdit(unknown)
	got = env.logged()
	if len(got) != 3 || got[0].Text != "hello again" || !got[0].Edited || got[0].At != text.Date {
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

func TestChatLogRecentThread(t *testing.T) {
	ctx := context.Background()

	t.Run("nil client and non-positive limit return nil", func(t *testing.T) {
		_, client := withChatRedis(t)
		chatID := uniqueModuleChatID()
		if got, err := chatLogRecentThread(ctx, nil, chatID, 0, 100, 5); err != nil || got != nil {
			t.Fatalf("nil client: got (%v, %v), want (nil, nil)", got, err)
		}
		if got, err := chatLogRecentThread(ctx, client, chatID, 0, 100, 0); err != nil || got != nil {
			t.Fatalf("limit 0: got (%v, %v), want (nil, nil)", got, err)
		}
		if got, err := chatLogRecentThread(ctx, client, chatID, 0, 100, -3); err != nil || got != nil {
			t.Fatalf("limit -3: got (%v, %v), want (nil, nil)", got, err)
		}
	})

	t.Run("topic isolation, beforeMsgID filter, oldest-first cap, and malformed JSON skip", func(t *testing.T) {
		_, client := withChatRedis(t)
		chatID := uniqueModuleChatID()

		record := func(id, thread int64, text string) {
			t.Helper()
			e := chatLogEntry{MessageID: id, UserID: 5, Name: "Ann", Text: text, Thread: thread}
			if err := chatLogRecord(client, chatID, e); err != nil {
				t.Fatalf("chatLogRecord(%d): %v", id, err)
			}
		}

		// Seed 8 entries in thread 10 (IDs 1..8), interleaved with thread 20 and thread 0,
		// plus triggering message (ID 100) and newer message (ID 101) in thread 10.
		for id := int64(1); id <= 8; id++ {
			record(id, 10, "t10-"+strconv.FormatInt(id, 10))
		}
		record(20, 20, "t20-a")
		record(21, 20, "t20-b")
		record(30, 0, "general-a")
		record(31, 0, "general-b")
		record(100, 10, "trigger")
		record(101, 10, "newer")

		// Inject a malformed JSON member inside the newest scores to ensure it is skipped cleanly.
		if err := client.ZAdd(ctx, chatLogKey(chatID), redis.Z{Score: 99, Member: "{not-valid-json"}).Err(); err != nil {
			t.Fatalf("ZAdd malformed: %v", err)
		}

		got10, err := chatLogRecentThread(ctx, client, chatID, 10, 100, 5)
		if err != nil {
			t.Fatalf("chatLogRecentThread(thread=10): %v", err)
		}
		ids10 := make([]int64, len(got10))
		for i, e := range got10 {
			ids10[i] = e.MessageID
			if e.Thread != 10 {
				t.Errorf("thread 10 read leaked entry from thread %d: %+v", e.Thread, e)
			}
		}
		chatLogWantIDs(t, ids10, 4, 5, 6, 7, 8)

		got20, err := chatLogRecentThread(ctx, client, chatID, 20, 100, 5)
		if err != nil {
			t.Fatalf("chatLogRecentThread(thread=20): %v", err)
		}
		ids20 := make([]int64, len(got20))
		for i, e := range got20 {
			ids20[i] = e.MessageID
		}
		chatLogWantIDs(t, ids20, 20, 21)

		got0, err := chatLogRecentThread(ctx, client, chatID, 0, 100, 5)
		if err != nil {
			t.Fatalf("chatLogRecentThread(thread=0): %v", err)
		}
		ids0 := make([]int64, len(got0))
		for i, e := range got0 {
			ids0[i] = e.MessageID
		}
		chatLogWantIDs(t, ids0, 30, 31)

		// Thread with no matching entries returns (nil, nil).
		gotNone, err := chatLogRecentThread(ctx, client, chatID, 999, 100, 5)
		if err != nil || gotNone != nil {
			t.Fatalf("no matches: got (%v, %v), want (nil, nil)", gotNone, err)
		}
	})

	t.Run("bounded reverse scan stops at chatLogRecentScanLimit", func(t *testing.T) {
		_, client := withChatRedis(t)
		chatID := uniqueModuleChatID()

		// Old matching entry at score 1 in thread 77, followed by chatLogRecentScanLimit entries in thread 0.
		if err := chatLogRecord(client, chatID, chatLogEntry{MessageID: 1, UserID: 5, Name: "Ann", Text: "old", Thread: 77}); err != nil {
			t.Fatal(err)
		}
		for i := int64(2); i < 2+chatLogRecentScanLimit; i++ {
			if err := chatLogRecord(client, chatID, chatLogEntry{MessageID: i, UserID: 5, Name: "Ann", Text: "noise", Thread: 0}); err != nil {
				t.Fatal(err)
			}
		}

		// With 1 old + 50 newer entries, entry 1 is rank 50 (outside 0..49), so scan skips it.
		got, err := chatLogRecentThread(ctx, client, chatID, 77, 1000, 5)
		if err != nil || got != nil {
			t.Fatalf("beyond scan cap: got (%v, %v), want (nil, nil)", got, err)
		}
	})

	t.Run("redis error propagates", func(t *testing.T) {
		mr, client := withChatRedis(t)
		chatID := uniqueModuleChatID()
		mr.Close()

		got, err := chatLogRecentThread(ctx, client, chatID, 0, 100, 5)
		if err == nil || got != nil {
			t.Fatalf("closed redis: got (%v, %v), want (nil, error)", got, err)
		}
	})
}

func TestChatLogThreadAndTimeQueries(t *testing.T) {
	ctx := context.Background()

	t.Run("timestamp recording and edit preservation", func(t *testing.T) {
		env := newChatEnv(t)
		chatLogTurnOn(t, env)
		alice := env.member("Alice")

		msg := env.message(alice, "original text")
		msg.Date = 1700000100
		env.deliver(msg)

		raw, err := env.rc.ZRangeArgs(ctx, redis.ZRangeArgs{
			Key:     chatLogKey(env.chatID),
			Start:   strconv.FormatInt(msg.MessageId, 10),
			Stop:    strconv.FormatInt(msg.MessageId, 10),
			ByScore: true,
		}).Result()
		if err != nil || len(raw) != 1 {
			t.Fatalf("ZRangeArgs = %v, %v", raw, err)
		}
		var fields map[string]any
		if err := json.Unmarshal([]byte(raw[0]), &fields); err != nil {
			t.Fatalf("unmarshal raw JSON: %v", err)
		}
		if fields["at"] != float64(1700000100) {
			t.Fatalf("stored JSON at = %v (%s), want 1700000100", fields["at"], raw[0])
		}

		entry, ok, err := chatLogEntryAt(ctx, env.rc, env.chatID, msg.MessageId)
		if err != nil || !ok || entry.At != 1700000100 || entry.Edited {
			t.Fatalf("chatLogEntryAt before edit = (%+v, %v, %v)", entry, ok, err)
		}

		edited := *msg
		edited.Text = "edited via telegram"
		edited.EditDate = 1700000999
		env.deliverEdit(&edited)

		entry, ok, err = chatLogEntryAt(ctx, env.rc, env.chatID, msg.MessageId)
		if err != nil || !ok || entry.At != 1700000100 || !entry.Edited || entry.Text != "edited via telegram" {
			t.Fatalf("after deliverEdit = (%+v, %v, %v)", entry, ok, err)
		}

		for _, overrideAt := range []int64{1700000999, 0} {
			replaced, err := chatLogEdit(env.rc, env.chatID, chatLogEntry{
				MessageID: msg.MessageId,
				UserID:    alice.Id,
				Name:      "Alice",
				Text:      "direct edit " + strconv.FormatInt(overrideAt, 10),
				At:        overrideAt,
			})
			if err != nil || !replaced {
				t.Fatalf("chatLogEdit(At=%d) = (%v, %v)", overrideAt, replaced, err)
			}
			entry, ok, err = chatLogEntryAt(ctx, env.rc, env.chatID, msg.MessageId)
			if err != nil || !ok || entry.At != 1700000100 || !entry.Edited {
				t.Fatalf("after chatLogEdit(At=%d) = (%+v, %v, %v), want At=1700000100", overrideAt, entry, ok, err)
			}
		}

		// Absent, malformed, nil-client, and non-positive IDs return (zero, false, nil).
		if e, found, err := chatLogEntryAt(ctx, env.rc, env.chatID, 999999); err != nil || found || e.MessageID != 0 {
			t.Fatalf("absent chatLogEntryAt = (%+v, %v, %v)", e, found, err)
		}
		if err := env.rc.ZAdd(ctx, chatLogKey(env.chatID), redis.Z{Score: 888, Member: "{bad-json"}).Err(); err != nil {
			t.Fatal(err)
		}
		if e, found, err := chatLogEntryAt(ctx, env.rc, env.chatID, 888); err != nil || found || e.MessageID != 0 {
			t.Fatalf("malformed chatLogEntryAt = (%+v, %v, %v)", e, found, err)
		}
		if e, found, err := chatLogEntryAt(ctx, nil, env.chatID, msg.MessageId); err != nil || found || e.MessageID != 0 {
			t.Fatalf("nil client chatLogEntryAt = (%+v, %v, %v)", e, found, err)
		}
		if e, found, err := chatLogEntryAt(ctx, env.rc, env.chatID, 0); err != nil || found || e.MessageID != 0 {
			t.Fatalf("zero id chatLogEntryAt = (%+v, %v, %v)", e, found, err)
		}
	})

	t.Run("legacy At==0 entries", func(t *testing.T) {
		_, client := withChatRedis(t)
		chatID := uniqueModuleChatID()

		// Legacy pre-Phase-34 entries without "at" in JSON.
		if err := client.ZAdd(ctx, chatLogKey(chatID),
			redis.Z{Score: 10, Member: `{"m":10,"u":42,"n":"Legacy","t":"old message","th":5}`},
			redis.Z{Score: 18, Member: `{"m":18,"u":99,"n":"LegacyMid","t":"mid legacy","th":5}`},
		).Err(); err != nil {
			t.Fatal(err)
		}
		for _, e := range []chatLogEntry{
			{MessageID: 15, UserID: 7, Name: "New", Text: "at 200", Thread: 5, At: 1700000200},
			{MessageID: 20, UserID: 7, Name: "New", Text: "at 500", Thread: 5, At: 1700000500},
		} {
			if err := chatLogRecord(client, chatID, e); err != nil {
				t.Fatal(err)
			}
		}

		gotEntry, ok, err := chatLogEntryAt(ctx, client, chatID, 10)
		if err != nil || !ok || gotEntry.At != 0 || gotEntry.Text != "old message" {
			t.Fatalf("chatLogEntryAt legacy = (%+v, %v, %v)", gotEntry, ok, err)
		}

		recent, err := chatLogRecent(ctx, client, chatID, 5, 0, 10)
		if err != nil {
			t.Fatalf("chatLogRecent: %v", err)
		}
		chatLogWantIDs(t, chatLogTestIDs(recent), 10, 15, 18, 20)

		sinceUser, err := chatLogSinceUser(ctx, client, chatID, 5, 42, 0)
		if err != nil {
			t.Fatalf("chatLogSinceUser: %v", err)
		}
		chatLogWantIDs(t, chatLogTestIDs(sinceUser), 10, 15, 18, 20)

		// Legacy At==0 entries must be skipped by chatLogSinceTime without blocking older valid entries.
		sinceTime, err := chatLogSinceTime(ctx, client, chatID, 5, 1700000100, 0)
		if err != nil {
			t.Fatalf("chatLogSinceTime: %v", err)
		}
		chatLogWantIDs(t, chatLogTestIDs(sinceTime), 15, 20)

		sinceTimeNarrow, err := chatLogSinceTime(ctx, client, chatID, 5, 1700000300, 0)
		if err != nil {
			t.Fatalf("chatLogSinceTime narrow: %v", err)
		}
		chatLogWantIDs(t, chatLogTestIDs(sinceTimeNarrow), 20)
	})

	t.Run("strict thread isolation, beforeMsgID, and oldest-first ordering", func(t *testing.T) {
		_, client := withChatRedis(t)
		chatID := uniqueModuleChatID()
		const u1, u2, u3 = int64(100), int64(200), int64(300)

		seed := []chatLogEntry{
			{MessageID: 1, UserID: u1, Name: "U1", Text: "g1", Thread: 0, At: 1700000010},
			{MessageID: 2, UserID: u1, Name: "U1", Text: "t10-1", Thread: 10, At: 1700000020},
			{MessageID: 3, UserID: u1, Name: "U1", Text: "t20-1", Thread: 20, At: 1700000030},
			{MessageID: 4, UserID: u2, Name: "U2", Text: "t10-2", Thread: 10, At: 1700000040},
			{MessageID: 5, UserID: u2, Name: "U2", Text: "g2", Thread: 0, At: 1700000050},
			{MessageID: 6, UserID: u1, Name: "U1", Text: "t10-3", Thread: 10, At: 1700000060},
			{MessageID: 7, UserID: u3, Name: "U3", Text: "t20-2", Thread: 20, At: 1700000070},
			{MessageID: 8, UserID: u2, Name: "U2", Text: "t10-4", Thread: 10, At: 1700000080},
			{MessageID: 9, UserID: u1, Name: "U1", Text: "t10-trigger", Thread: 10, At: 1700000090},
		}
		for _, e := range seed {
			if err := chatLogRecord(client, chatID, e); err != nil {
				t.Fatalf("chatLogRecord(%d): %v", e.MessageID, err)
			}
		}
		if err := client.ZAdd(ctx, chatLogKey(chatID), redis.Z{Score: 99, Member: "{bad-json"}).Err(); err != nil {
			t.Fatal(err)
		}

		// chatLogRecent isolation + beforeMsgID + limit + oldest-first.
		got, err := chatLogRecent(ctx, client, chatID, 10, 9, 3)
		if err != nil {
			t.Fatal(err)
		}
		chatLogWantIDs(t, chatLogTestIDs(got), 4, 6, 8)

		got, err = chatLogRecent(ctx, client, chatID, 10, 0, 10)
		if err != nil {
			t.Fatal(err)
		}
		chatLogWantIDs(t, chatLogTestIDs(got), 2, 4, 6, 8, 9)

		got, err = chatLogRecent(ctx, client, chatID, 0, 9, 10)
		if err != nil {
			t.Fatal(err)
		}
		chatLogWantIDs(t, chatLogTestIDs(got), 1, 5)

		got, err = chatLogRecent(ctx, client, chatID, 20, 9, 10)
		if err != nil {
			t.Fatal(err)
		}
		chatLogWantIDs(t, chatLogTestIDs(got), 3, 7)

		// chatLogSinceUser isolation + beforeMsgID + anchor inclusivity.
		got, err = chatLogSinceUser(ctx, client, chatID, 10, u1, 9)
		if err != nil {
			t.Fatal(err)
		}
		chatLogWantIDs(t, chatLogTestIDs(got), 6, 8)

		got, err = chatLogSinceUser(ctx, client, chatID, 0, u1, 9)
		if err != nil {
			t.Fatal(err)
		}
		chatLogWantIDs(t, chatLogTestIDs(got), 1, 5)

		// u3 only spoke in thread 20, so thread 10 must return (nil, nil).
		got, err = chatLogSinceUser(ctx, client, chatID, 10, u3, 9)
		if err != nil || got != nil {
			t.Fatalf("u3 in thread 10 = (%v, %v), want (nil, nil)", got, err)
		}

		// chatLogSinceTime isolation + beforeMsgID + cutoff.
		got, err = chatLogSinceTime(ctx, client, chatID, 10, 1700000040, 9)
		if err != nil {
			t.Fatal(err)
		}
		chatLogWantIDs(t, chatLogTestIDs(got), 4, 6, 8)

		got, err = chatLogSinceTime(ctx, client, chatID, 20, 1700000040, 9)
		if err != nil {
			t.Fatal(err)
		}
		chatLogWantIDs(t, chatLogTestIDs(got), 7)

		got, err = chatLogSinceTime(ctx, client, chatID, 0, 1700000010, 9)
		if err != nil {
			t.Fatal(err)
		}
		chatLogWantIDs(t, chatLogTestIDs(got), 1, 5)

		got, err = chatLogSinceTime(ctx, client, chatID, 10, 1700009999, 0)
		if err != nil || got != nil {
			t.Fatalf("future cutoff = (%v, %v), want (nil, nil)", got, err)
		}

		// chatLogRecentThread strict thread isolation as well.
		got, err = chatLogRecentThread(ctx, client, chatID, 10, 9, 3)
		if err != nil {
			t.Fatal(err)
		}
		chatLogWantIDs(t, chatLogTestIDs(got), 4, 6, 8)

		got, err = chatLogRecentThread(ctx, client, chatID, 0, 9, 5)
		if err != nil {
			t.Fatal(err)
		}
		chatLogWantIDs(t, chatLogTestIDs(got), 1, 5)

		// Nil client / invalid input guards.
		if g, err := chatLogRecent(ctx, nil, chatID, 10, 9, 5); err != nil || g != nil {
			t.Fatalf("chatLogRecent nil client = (%v, %v)", g, err)
		}
		if g, err := chatLogRecent(ctx, client, chatID, 10, 9, 0); err != nil || g != nil {
			t.Fatalf("chatLogRecent limit 0 = (%v, %v)", g, err)
		}
		if g, err := chatLogSinceUser(ctx, nil, chatID, 10, u1, 9); err != nil || g != nil {
			t.Fatalf("chatLogSinceUser nil client = (%v, %v)", g, err)
		}
		if g, err := chatLogSinceUser(ctx, client, chatID, 10, 0, 9); err != nil || g != nil {
			t.Fatalf("chatLogSinceUser userID 0 = (%v, %v)", g, err)
		}
		if g, err := chatLogSinceTime(ctx, nil, chatID, 10, 1700000040, 9); err != nil || g != nil {
			t.Fatalf("chatLogSinceTime nil client = (%v, %v)", g, err)
		}
	})
}

func chatLogTestIDs(entries []chatLogEntry) []int64 {
	ids := make([]int64, len(entries))
	for i, e := range entries {
		ids[i] = e.MessageID
	}
	return ids
}
