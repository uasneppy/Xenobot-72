//go:build testtools

package modules

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/db/presence"
	utilsCache "github.com/uasneppy/Xenobot-72/xenobot/utils/cache"
)

func presenceArchiveCleanup(t *testing.T, chatID int64) {
	t.Helper()
	presenceCleanup(t, chatID)
	t.Cleanup(func() {
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatPresenceArchived{})
	})
}

type archiveCommandEnv struct {
	t          *testing.T
	dispatcher *ext.Dispatcher
	bot        *gotgbot.Bot
	group      gotgbot.Chat
	staffGroup gotgbot.Chat
	channel    gotgbot.Chat
	admin      gotgbot.User
	member     gotgbot.User
	botCanPost bool

	mu    sync.Mutex
	calls []presenceWireCall
}

func newArchiveCommandEnv(t *testing.T) *archiveCommandEnv {
	t.Helper()
	withRealLocales(t)
	utilsCache.SetupTestMemoryMarshaler(t)

	groupID := uniqueModuleChatID()
	staffID := uniqueModuleChatID()
	channelID := uniqueModuleChatID()
	presenceArchiveCleanup(t, groupID)
	presenceArchiveCleanup(t, staffID)

	env := &archiveCommandEnv{
		t:          t,
		dispatcher: ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1}),
		group:      gotgbot.Chat{Id: groupID, Type: "supergroup", Title: "Archive Source"},
		staffGroup: gotgbot.Chat{Id: staffID, Type: "supergroup", Title: "Staff"},
		channel:    gotgbot.Chat{Id: channelID, Type: "channel", Title: "Highlights"},
		admin:      gotgbot.User{Id: 777000, FirstName: "Telegram"},
		member:     gotgbot.User{Id: 42, FirstName: "Member"},
		botCanPost: true,
	}

	chatSwap(t, &presenceStaffLookup, func(id int64) *models.StaffGroup {
		if id == env.staffGroup.Id {
			return &models.StaffGroup{ChatID: env.staffGroup.Id}
		}
		return nil
	})

	env.bot = newWireTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		params, err := decodeTelegramWireParams(r)
		env.mu.Lock()
		if err == nil {
			env.calls = append(env.calls, presenceWireCall{Path: r.URL.Path, Params: params})
		}
		canPost := env.botCanPost
		env.mu.Unlock()

		switch r.URL.Path {
		case "/bottest/getChat":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"id":` + params["chat_id"] + `,"type":"supergroup"}}`))
		case "/bottest/getChatAdministrators":
			_, _ = w.Write([]byte(`{"ok":true,"result":[{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"XenobotTestBot"},"can_change_info":true}]}`))
		case "/bottest/getChatMember":
			uid := params["user_id"]
			switch uid {
			case "999":
				if canPost {
					_, _ = w.Write([]byte(`{"ok":true,"result":{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"XenobotTestBot"},"can_post_messages":true,"can_manage_chat":true}}`))
				} else if params["chat_id"] == strconv.FormatInt(env.channel.Id, 10) {
					_, _ = w.Write([]byte(`{"ok":true,"result":{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"XenobotTestBot"},"can_post_messages":false,"can_manage_chat":true}}`))
				} else {
					_, _ = w.Write([]byte(`{"ok":true,"result":{"status":"restricted","user":{"id":999,"is_bot":true,"first_name":"XenobotTestBot"},"is_member":true,"can_send_messages":false}}`))
				}
			case "777000":
				_, _ = w.Write([]byte(`{"ok":true,"result":{"status":"creator","user":{"id":777000,"is_bot":false,"first_name":"Telegram"}}}`))
			default:
				_, _ = w.Write([]byte(`{"ok":true,"result":{"status":"member","user":{"id":42,"is_bot":false,"first_name":"Member"}}}`))
			}
		case "/bottest/sendMessage":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":9001,"date":1,"chat":{"id":` + params["chat_id"] + `,"type":"supergroup"}}}`))
		default:
			_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
		}
	})
	env.bot.User = gotgbot.User{Id: 999, IsBot: true, FirstName: "XenobotTestBot", Username: "XenobotTestBot"}

	LoadPresence(env.dispatcher)
	return env
}

func (e *archiveCommandEnv) process(u *gotgbot.Update) {
	e.t.Helper()
	if err := e.dispatcher.ProcessUpdate(e.bot, u, nil); err != nil {
		e.t.Fatalf("ProcessUpdate(%d): %v", u.UpdateId, err)
	}
}

func (e *archiveCommandEnv) lastText(chatID int64) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	wantID := strconv.FormatInt(chatID, 10)
	var last string
	for _, c := range e.calls {
		if c.Path == "/bottest/sendMessage" && c.Params["chat_id"] == wantID {
			last = c.Params["text"]
		}
	}
	return last
}

func (e *archiveCommandEnv) settings(chatID int64) presence.Settings {
	e.t.Helper()
	s, err := presence.GetFresh(context.Background(), chatID)
	if err != nil {
		e.t.Fatalf("presence.GetFresh: %v", err)
	}
	return s
}

func TestPresenceArchiveSetAndUnset(t *testing.T) {
	for _, store := range []string{"memory", "miniredis"} {
		t.Run(store, func(t *testing.T) {
			if store == "miniredis" {
				withMiniredis(t)
			}

			t.Run("2-step channel proof binds, preserves non-admin token, rejects replay and unmarked", func(t *testing.T) {
				env := newArchiveCommandEnv(t)

				post := channelPostUpdate(env.channel, "/setarchive")
				env.process(post)
				if got := env.settings(env.group.Id).ArchiveChatID; got != 0 {
					t.Fatalf("after channel post ArchiveChatID = %d, want 0", got)
				}
				if got, want := env.lastText(env.channel.Id), ukText(t, "archive_forward_to_group"); got != want {
					t.Fatalf("channel reply = %q, want %q", got, want)
				}

				// Non-admin forward must fail without burning the pending marker.
				env.process(forwardedPostUpdate(env.group, env.member, post, post.ChannelPost.MessageId))
				if got := env.settings(env.group.Id).ArchiveChatID; got != 0 {
					t.Fatalf("non-admin forward bound %d", got)
				}

				// Unmarked forward fails and still keeps the real marker intact.
				env.process(forwardedPostUpdate(env.group, env.admin, post, post.ChannelPost.MessageId+99))
				if got := env.settings(env.group.Id).ArchiveChatID; got != 0 {
					t.Fatalf("unmarked forward bound %d", got)
				}
				if got, want := env.lastText(env.group.Id), ukText(t, "archive_need_target"); got != want {
					t.Fatalf("unmarked reply = %q, want %q", got, want)
				}

				// Admin forward binds the destination channel.
				env.process(forwardedPostUpdate(env.group, env.admin, post, post.ChannelPost.MessageId))
				got := env.settings(env.group.Id)
				if got.ArchiveChatID != env.channel.Id || got.ArchiveThreadID != 0 || !got.ArchiveEnabled {
					t.Fatalf("bound settings = %+v, want ArchiveChatID=%d ArchiveThreadID=0 ArchiveEnabled=true", got, env.channel.Id)
				}
				if gotText, want := env.lastText(env.group.Id), ukText(t, "archive_set_ok"); gotText != want {
					t.Fatalf("bind reply = %q, want %q", gotText, want)
				}

				// Unset and verify replay of already-consumed marker is refused.
				env.process(textUpdate(env.group, env.admin, "/unsetarchive"))
				if got, want := env.lastText(env.group.Id), ukText(t, "archive_unset_ok"); got != want {
					t.Fatalf("unset reply = %q, want %q", got, want)
				}
				if after := env.settings(env.group.Id); after.ArchiveChatID != 0 || after.ArchiveEnabled {
					t.Fatalf("after unset = %+v, want unlinked and disabled", after)
				}
				env.process(forwardedPostUpdate(env.group, env.admin, post, post.ChannelPost.MessageId))
				if afterReplay := env.settings(env.group.Id); afterReplay.ArchiveChatID != 0 {
					t.Fatalf("replayed forward bound %d", afterReplay.ArchiveChatID)
				}
			})

			t.Run("plain forward without command text binds via group-11 capture", func(t *testing.T) {
				env := newArchiveCommandEnv(t)
				post := channelPostUpdate(env.channel, "/setarchive")
				env.process(post)

				id := nextFlowID()
				plainFwd := &gotgbot.Update{
					UpdateId: id,
					Message: &gotgbot.Message{
						MessageId:     id,
						Date:          1,
						Chat:          env.group,
						From:          &env.admin,
						Text:          "forwarded without slash command",
						ForwardOrigin: gotgbot.MessageOriginChannel{Date: 1, Chat: env.channel, MessageId: post.ChannelPost.MessageId},
					},
				}
				env.process(plainFwd)
				if got := env.settings(env.group.Id).ArchiveChatID; got != env.channel.Id {
					t.Fatalf("plain forward ArchiveChatID = %d, want %d", got, env.channel.Id)
				}
			})

			t.Run("Staff Group refused as both source and destination without burning token", func(t *testing.T) {
				env := newArchiveCommandEnv(t)
				post := channelPostUpdate(env.channel, "/setarchive")
				env.process(post)

				// Staff Group as source: refused, marker preserved for real group.
				env.process(forwardedPostUpdate(env.staffGroup, env.admin, post, post.ChannelPost.MessageId))
				if got := env.settings(env.staffGroup.Id).ArchiveChatID; got != 0 {
					t.Fatalf("staff group source bound %d", got)
				}
				if got, want := env.lastText(env.staffGroup.Id), ukText(t, "archive_staff_refused"); got != want {
					t.Fatalf("staff source reply = %q, want %q", got, want)
				}
				// Verify marker survived by binding real group afterward.
				env.process(forwardedPostUpdate(env.group, env.admin, post, post.ChannelPost.MessageId))
				if got := env.settings(env.group.Id).ArchiveChatID; got != env.channel.Id {
					t.Fatalf("real group after staff refusal ArchiveChatID = %d, want %d", got, env.channel.Id)
				}

				// Staff Group as destination (even if a marker existed): refused.
				staffPostID := nextFlowID()
				if err := storePendingMarker(setarchivePendingKey(env.staffGroup.Id, staffPostID), []byte("1")); err != nil {
					t.Fatalf("storePendingMarker: %v", err)
				}
				group2 := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Clean Group"}
				presenceArchiveCleanup(t, group2.Id)
				env.process(&gotgbot.Update{
					UpdateId: nextFlowID(),
					Message: &gotgbot.Message{
						MessageId:     nextFlowID(),
						Date:          1,
						Chat:          group2,
						From:          &env.admin,
						Text:          "/setarchive",
						Entities:      commandEntity("/setarchive"),
						ForwardOrigin: gotgbot.MessageOriginChannel{Date: 1, Chat: env.staffGroup, MessageId: staffPostID},
					},
				})
				if got := env.settings(group2.Id).ArchiveChatID; got != 0 {
					t.Fatalf("staff group destination bound %d", got)
				}
				if got, want := env.lastText(group2.Id), ukText(t, "archive_staff_refused"); got != want {
					t.Fatalf("staff dest reply = %q, want %q", got, want)
				}
			})

			t.Run("same-group forum topic and bot post permission checks", func(t *testing.T) {
				env := newArchiveCommandEnv(t)
				const topicID int64 = 77

				// Bot cannot post -> refused without binding.
				env.mu.Lock()
				env.botCanPost = false
				env.mu.Unlock()
				topicCmd := textUpdate(env.group, env.admin, "/setarchive")
				topicCmd.Message.IsTopicMessage = true
				topicCmd.Message.MessageThreadId = topicID
				env.process(topicCmd)
				if got := env.settings(env.group.Id).ArchiveChatID; got != 0 {
					t.Fatalf("bound when bot cannot post: %d", got)
				}
				if got, want := env.lastText(env.group.Id), ukText(t, "archive_bot_cannot_post"); got != want {
					t.Fatalf("cannot post reply = %q, want %q", got, want)
				}

				// Bot can post -> binds (group.Id, topicID).
				env.mu.Lock()
				env.botCanPost = true
				env.mu.Unlock()
				env.process(topicCmd)
				got := env.settings(env.group.Id)
				if got.ArchiveChatID != env.group.Id || got.ArchiveThreadID != topicID || !got.ArchiveEnabled {
					t.Fatalf("topic bound = %+v, want (%d, %d, true)", got, env.group.Id, topicID)
				}

				// Channel without can_post_messages -> refused without burning pending marker.
				post := channelPostUpdate(env.channel, "/setarchive")
				env.process(post)
				env.mu.Lock()
				env.botCanPost = false
				env.mu.Unlock()
				env.process(forwardedPostUpdate(env.group, env.admin, post, post.ChannelPost.MessageId))
				if got, want := env.lastText(env.group.Id), ukText(t, "archive_bot_cannot_post"); got != want {
					t.Fatalf("channel cannot post reply = %q, want %q", got, want)
				}
				if _, ok := peekPendingMarker(setarchivePendingKey(env.channel.Id, post.ChannelPost.MessageId)); !ok {
					t.Fatal("pending marker was burned when bot lacked channel post permission")
				}
			})

			t.Run("save failure restores marker and unsetarchive on unlinked returns not_set", func(t *testing.T) {
				env := newArchiveCommandEnv(t)

				env.process(textUpdate(env.group, env.admin, "/unsetarchive"))
				if got, want := env.lastText(env.group.Id), ukText(t, "archive_not_set"); got != want {
					t.Fatalf("unset unlinked reply = %q, want %q", got, want)
				}

				post := channelPostUpdate(env.channel, "/setarchive")
				env.process(post)
				chatSwap(t, &presenceWrites.ArchiveTarget, func(context.Context, int64, int64, int64, int64) error {
					return errors.New("db write error")
				})
				env.process(forwardedPostUpdate(env.group, env.admin, post, post.ChannelPost.MessageId))
				if got, want := env.lastText(env.group.Id), ukText(t, "common_settings_save_failed"); got != want {
					t.Fatalf("save failed reply = %q, want %q", got, want)
				}
				if _, ok := peekPendingMarker(setarchivePendingKey(env.channel.Id, post.ChannelPost.MessageId)); !ok {
					t.Fatal("pending marker was not restored after DB save failure")
				}
			})
		})
	}
}

type archiveReactionEnv struct {
	t          *testing.T
	mr         *miniredis.Miniredis
	rc         *redis.Client
	bot        *gotgbot.Bot
	dispatcher *ext.Dispatcher
	nextUpdate atomic.Int64

	mu    sync.Mutex
	calls []presenceWireCall
	// copyCode, when set, makes copyMessage fail with this Telegram error code and copyDesc.
	copyCode int
	copyDesc string
}

func newArchiveReactionEnv(t *testing.T) *archiveReactionEnv {
	t.Helper()
	withRealLocales(t)
	withFastPresencePacer(t)
	utilsCache.SetupTestMemoryMarshaler(t)
	mr, rc := withChatRedis(t)
	chatJobs.wait()
	t.Cleanup(chatJobs.wait)

	chatSwap(t, &presenceStaffLookup, func(int64) *models.StaffGroup { return nil })
	chatSwap(t, &presenceLockdownLookup, func(int64) (*models.ChatLockdown, error) { return nil, nil })

	env := &archiveReactionEnv{
		t:          t,
		mr:         mr,
		rc:         rc,
		dispatcher: ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1}),
	}
	env.bot = newWireTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		params, err := decodeTelegramWireParams(r)
		if err == nil {
			env.mu.Lock()
			env.calls = append(env.calls, presenceWireCall{Path: r.URL.Path, Params: params})
			env.mu.Unlock()
		}
		switch r.URL.Path {
		case "/bottest/copyMessage":
			env.mu.Lock()
			code, desc := env.copyCode, env.copyDesc
			env.mu.Unlock()
			if code != 0 {
				_, _ = fmt.Fprintf(w, `{"ok":false,"error_code":%d,"description":%q}`, code, desc)
				return
			}
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":9991}}`))
		case "/bottest/forwardMessage":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":9992,"date":1,"chat":{"id":-1002,"type":"channel"}}}`))
		default:
			_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
		}
	})
	LoadPresence(env.dispatcher)
	return env
}

func (e *archiveReactionEnv) callsFor(method string) []presenceWireCall {
	e.mu.Lock()
	defer e.mu.Unlock()
	want := "/bottest/" + method
	var out []presenceWireCall
	for _, c := range e.calls {
		if c.Path == want {
			out = append(out, c)
		}
	}
	return out
}

func (e *archiveReactionEnv) react(chat gotgbot.Chat, msgID int64, user *gotgbot.User, oldEmojis, newEmojis []string) {
	e.t.Helper()
	upd := &gotgbot.Update{
		UpdateId: e.nextUpdate.Add(1),
		MessageReaction: &gotgbot.MessageReactionUpdated{
			Chat:        chat,
			MessageId:   msgID,
			User:        user,
			OldReaction: emojiReactions(oldEmojis...),
			NewReaction: emojiReactions(newEmojis...),
		},
	}
	if err := e.dispatcher.ProcessUpdate(e.bot, upd, nil); err != nil {
		e.t.Fatalf("ProcessUpdate: %v", err)
	}
}

func TestPresenceArchiveReactions(t *testing.T) {
	const targetChatID int64 = -100999000111

	t.Run("filters author, bots, hostile, non-positive, Premium stacking, SREM, and deduplicates across Redis flush", func(t *testing.T) {
		env := newArchiveReactionEnv(t)
		chatID := -1000000000000 - (nextFlowID() % 1000000000)
		presenceArchiveCleanup(t, chatID)
		chat := gotgbot.Chat{Id: chatID, Type: "supergroup", Title: "Lab"}

		const msgID int64 = 555
		const authorID int64 = 101
		if err := chatLogRecord(env.rc, chatID, chatLogEntry{MessageID: msgID, UserID: authorID, Name: "Author", Text: "standout"}); err != nil {
			t.Fatalf("chatLogRecord: %v", err)
		}

		chatSwap(t, &presenceSettingsGet, func(context.Context, int64) (presence.Settings, error) {
			return presence.Settings{
				ArchiveEnabled:   true,
				ArchiveChatID:    targetChatID,
				ArchiveThreadID:  19,
				ArchiveThreshold: 3,
				ArchiveEmojiMode: presence.ArchiveEmojiModePositive,
				ArchiveDelivery:  presence.ArchiveDeliveryCopy,
			}, nil
		})

		// Author self-reaction, bot, anonymous actor, hostile emoji, and non-positive emoji ("🤔") are ignored.
		env.react(chat, msgID, &gotgbot.User{Id: authorID, FirstName: "Author"}, nil, []string{"🔥"})
		env.react(chat, msgID, &gotgbot.User{Id: 202, FirstName: "Bot", IsBot: true}, nil, []string{"🔥"})
		env.react(chat, msgID, nil, nil, []string{"🔥"})
		env.react(chat, msgID, &gotgbot.User{Id: 301, FirstName: "Hater"}, nil, []string{"👎", "🤡"})
		env.react(chat, msgID, &gotgbot.User{Id: 302, FirstName: "Thinker"}, nil, []string{"🤔"})

		// User 401 stacks multiple positive emojis (Premium) -> counts as 1 reactor.
		env.react(chat, msgID, &gotgbot.User{Id: 401, FirstName: "Prem"}, nil, []string{"🔥"})
		env.react(chat, msgID, &gotgbot.User{Id: 401, FirstName: "Prem"}, []string{"🔥"}, []string{"🔥", "👍", "❤"})
		// User 402 adds and removes before threshold -> SREM brings count back to 1.
		env.react(chat, msgID, &gotgbot.User{Id: 402, FirstName: "Flip"}, nil, []string{"🔥"})
		env.react(chat, msgID, &gotgbot.User{Id: 402, FirstName: "Flip"}, []string{"🔥"}, nil)

		if got := len(env.callsFor("copyMessage")); got != 0 {
			t.Fatalf("premature copyMessage count = %d, want 0", got)
		}

		// Two more distinct members (403, 404) cross threshold=3 -> CopyMessage with jump button.
		env.react(chat, msgID, &gotgbot.User{Id: 403, FirstName: "M3"}, nil, []string{"🎉"})
		env.react(chat, msgID, &gotgbot.User{Id: 404, FirstName: "M4"}, nil, []string{"🔥"})

		copies := env.callsFor("copyMessage")
		if len(copies) != 1 {
			t.Fatalf("copyMessage count = %d, want 1", len(copies))
		}
		if copies[0].Params["chat_id"] != fmt.Sprint(targetChatID) || copies[0].Params["message_thread_id"] != "19" {
			t.Fatalf("copyMessage params = %+v", copies[0].Params)
		}
		var markup gotgbot.InlineKeyboardMarkup
		if err := json.Unmarshal([]byte(copies[0].Params["reply_markup"]), &markup); err != nil {
			t.Fatalf("decode reply_markup: %v", err)
		}
		wantURL := fmt.Sprintf("https://t.me/c/%s/%d", strings.TrimPrefix(strconv.FormatInt(chatID, 10), "-100"), msgID)
		if len(markup.InlineKeyboard) != 1 || len(markup.InlineKeyboard[0]) != 1 || markup.InlineKeyboard[0][0].Url != wantURL {
			t.Fatalf("jump markup = %+v, want URL %q", markup, wantURL)
		}

		// Flush Redis and send fresh reactions -> Postgres chat_presence_archived blocks duplicate archive!
		env.mr.FlushAll()
		env.react(chat, msgID, &gotgbot.User{Id: 405, FirstName: "M5"}, nil, []string{"🔥"})
		env.react(chat, msgID, &gotgbot.User{Id: 406, FirstName: "M6"}, nil, []string{"🔥"})
		env.react(chat, msgID, &gotgbot.User{Id: 407, FirstName: "M7"}, nil, []string{"🔥"})
		if got := len(env.callsFor("copyMessage")); got != 1 {
			t.Fatalf("after Redis flush copyMessage count = %d, want 1 (durable DB dedup)", got)
		}
	})

	t.Run("legacy group omits jump button, forward mode forwards, all mode allows non-hostile emojis, bot_react excludes author", func(t *testing.T) {
		env := newArchiveReactionEnv(t)

		// 1. Legacy "group" (negative ID without -100 prefix) omits reply_markup.
		legacyID := -55512345
		presenceArchiveCleanup(t, int64(legacyID))
		legacyChat := gotgbot.Chat{Id: int64(legacyID), Type: "group", Title: "Legacy"}
		chatSwap(t, &presenceSettingsGet, func(context.Context, int64) (presence.Settings, error) {
			return presence.Settings{
				ArchiveEnabled:   true,
				ArchiveChatID:    targetChatID,
				ArchiveThreshold: 2,
				ArchiveEmojiMode: presence.ArchiveEmojiModeAll,
				ArchiveDelivery:  presence.ArchiveDeliveryCopy,
			}, nil
		})
		// Author resolved from bot_react fallback is excluded.
		presenceWriteBotReact(context.Background(), env.rc, int64(legacyID), 10, presenceBotReact{Emoji: "🔥", AuthorID: 700, Source: "static"})
		env.react(legacyChat, 10, &gotgbot.User{Id: 700, FirstName: "Author"}, nil, []string{"🤔"})
		env.react(legacyChat, 10, &gotgbot.User{Id: 701, FirstName: "U1"}, nil, []string{"🤔"})
		if got := len(env.callsFor("copyMessage")); got != 0 {
			t.Fatalf("bot_react author counted toward threshold")
		}
		env.react(legacyChat, 10, &gotgbot.User{Id: 702, FirstName: "U2"}, nil, []string{"🤯"})
		copies := env.callsFor("copyMessage")
		if len(copies) != 1 {
			t.Fatalf("legacy copyMessage count = %d, want 1", len(copies))
		}
		if rawMarkup := copies[0].Params["reply_markup"]; rawMarkup != "" {
			t.Fatalf("legacy group included reply_markup = %q, want omitted", rawMarkup)
		}

		// 2. Forward mode + public supergroup.
		fwdChatID := uniqueModuleChatID()
		presenceArchiveCleanup(t, fwdChatID)
		fwdChat := gotgbot.Chat{Id: fwdChatID, Type: "supergroup", Username: "publiclab"}
		chatSwap(t, &presenceSettingsGet, func(context.Context, int64) (presence.Settings, error) {
			return presence.Settings{
				ArchiveEnabled:   true,
				ArchiveChatID:    targetChatID,
				ArchiveThreadID:  55,
				ArchiveThreshold: 2,
				ArchiveEmojiMode: presence.ArchiveEmojiModePositive,
				ArchiveDelivery:  presence.ArchiveDeliveryForward,
			}, nil
		})
		env.react(fwdChat, 20, &gotgbot.User{Id: 801, FirstName: "U1"}, nil, []string{"🔥"})
		env.react(fwdChat, 20, &gotgbot.User{Id: 802, FirstName: "U2"}, nil, []string{"👍"})
		fwds := env.callsFor("forwardMessage")
		if len(fwds) != 1 || fwds[0].Params["message_thread_id"] != "55" {
			t.Fatalf("forwardMessage calls = %+v, want 1 with thread 55", fwds)
		}
	})

	t.Run("active lockdown and Staff destination fail closed without burning DB claim", func(t *testing.T) {
		env := newArchiveReactionEnv(t)
		chatID := uniqueModuleChatID()
		presenceArchiveCleanup(t, chatID)
		chat := gotgbot.Chat{Id: chatID, Type: "supergroup"}

		var claims atomic.Int32
		chatSwap(t, &presenceArchiveClaim, func(ctx context.Context, cid, mid int64) (bool, error) {
			claims.Add(1)
			return presence.TryClaimArchived(ctx, cid, mid)
		})
		chatSwap(t, &presenceLockdownLookup, func(int64) (*models.ChatLockdown, error) {
			return &models.ChatLockdown{ChatID: chatID, State: models.LockdownStateActive}, nil
		})
		chatSwap(t, &presenceSettingsGet, func(context.Context, int64) (presence.Settings, error) {
			return presence.Settings{
				ArchiveEnabled:   true,
				ArchiveChatID:    targetChatID,
				ArchiveThreshold: 2,
				ArchiveEmojiMode: presence.ArchiveEmojiModePositive,
			}, nil
		})

		env.react(chat, 30, &gotgbot.User{Id: 901, FirstName: "U1"}, nil, []string{"🔥"})
		env.react(chat, 30, &gotgbot.User{Id: 902, FirstName: "U2"}, nil, []string{"👍"})
		if claims.Load() != 0 || len(env.callsFor("copyMessage")) != 0 {
			t.Fatalf("lockdown did not block: claims=%d copies=%d", claims.Load(), len(env.callsFor("copyMessage")))
		}
		if env.mr.Exists(presenceArchiveDoneKey(chatID, 30)) {
			t.Fatal("lockdown burned done latch")
		}

		// Staff destination refused defensively before any Redis write.
		staffDest := uniqueModuleChatID()
		chatSwap(t, &presenceStaffLookup, func(id int64) *models.StaffGroup {
			if id == staffDest {
				return &models.StaffGroup{ChatID: staffDest}
			}
			return nil
		})
		chatSwap(t, &presenceSettingsGet, func(context.Context, int64) (presence.Settings, error) {
			return presence.Settings{
				ArchiveEnabled:   true,
				ArchiveChatID:    staffDest,
				ArchiveThreshold: 2,
			}, nil
		})
		env.react(chat, 31, &gotgbot.User{Id: 901, FirstName: "U1"}, nil, []string{"🔥"})
		if env.mr.Exists(presenceArchiveUsersKey(chatID, 31)) {
			t.Fatal("staff destination wrote archive reactor set")
		}
	})
}
