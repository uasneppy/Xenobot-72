//go:build testtools

package modules

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/Xenobot-72/xenobot/db/chatconv"
	utilsCache "github.com/uasneppy/Xenobot-72/xenobot/utils/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/ratelimit"
)

func withFastPresencePacer(t *testing.T) {
	t.Helper()
	chatSwap(t, &presenceReactionPacer, ratelimit.NewTelegramPacer(ratelimit.TelegramPacerOptions{
		NextKey:        "xenobot:presence:pace:test:next",
		BlockKey:       "xenobot:presence:pace:test:block",
		Interval:       time.Millisecond,
		MaxRetries:     1,
		MaxWait:        time.Second,
		RetryAfterUnit: 5 * time.Millisecond,
	}))
}

func newWireTestBot(t *testing.T, handler http.HandlerFunc) *gotgbot.Bot {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &gotgbot.Bot{
		Token: "test",
		BotClient: &gotgbot.BaseBotClient{
			Client: *srv.Client(),
			DefaultRequestOpts: &gotgbot.RequestOpts{
				APIURL: srv.URL,
			},
		},
	}
}

func assertChatBreakersClean(t *testing.T) {
	t.Helper()
	for provider, br := range chatBreakers {
		br.mu.Lock()
		groups := len(br.groups)
		till := br.providerTill
		br.mu.Unlock()
		if groups != 0 || !till.IsZero() {
			t.Errorf("chatBreakers[%s] poisoned: groups=%d providerTill=%v", provider, groups, till)
		}
	}
}

func TestPresenceClearReactionWireJSON(t *testing.T) {
	withFastPresencePacer(t)
	chatSwap(t, &chatBreakers, map[string]*memberReqAIBreaker{
		chatconv.ProviderAnthropic:   newMemberReqAIBreaker(),
		chatconv.ProviderGoogle:      newMemberReqAIBreaker(),
		chatconv.ProviderFeatherless: newMemberReqAIBreaker(),
	})

	const (
		chatID int64 = -100777001
		msgID  int64 = 42
	)

	var (
		mu       sync.Mutex
		gotPath  string
		gotBody  string
		gotCount int
	)
	bot := newWireTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		gotPath = r.URL.Path
		gotBody = string(body)
		gotCount++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	})

	ok := presenceClearReaction(context.Background(), bot, chatID, msgID)
	if !ok {
		t.Fatal("presenceClearReaction() = false, want true")
	}

	mu.Lock()
	path, body, count := gotPath, gotBody, gotCount
	mu.Unlock()

	if count != 1 {
		t.Fatalf("HTTP request count = %d, want 1", count)
	}
	if path != "/bottest/setMessageReaction" {
		t.Fatalf("HTTP path = %q, want /bottest/setMessageReaction", path)
	}
	if !strings.Contains(body, `"reaction":[]`) {
		t.Fatalf("wire body = %s, want explicit \"reaction\":[] (never omitted)", body)
	}

	var decoded map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatalf("wire body is not valid JSON (%s): %v", body, err)
	}
	if string(decoded["reaction"]) != "[]" {
		t.Fatalf("decoded reaction = %s, want []", string(decoded["reaction"]))
	}
	assertChatBreakersClean(t)
}

func TestPresenceReactAPI(t *testing.T) {
	t.Run("normalize FE0F, IsBig false, and store 24h bot_react outside cache", func(t *testing.T) {
		withFastPresencePacer(t)
		utilsCache.SetupTestMemoryMarshaler(t)
		mr, _ := withChatRedis(t)

		const (
			chatID   int64 = -100777010
			msgID    int64 = 101
			authorID int64 = 9001
		)

		var (
			mu          sync.Mutex
			setBodies   []string
			getChatHits int
		)
		bot := newWireTestBot(t, func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/bottest/getChat":
				mu.Lock()
				getChatHits++
				mu.Unlock()
				_, _ = w.Write([]byte(`{"ok":true,"result":{"id":-100777010,"type":"supergroup"}}`))
			case "/bottest/setMessageReaction":
				mu.Lock()
				setBodies = append(setBodies, string(body))
				mu.Unlock()
				_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
			default:
				http.NotFound(w, r)
			}
		})

		ok := presenceSetReaction(context.Background(), bot, chatID, msgID, authorID, "❤️", "cons")
		if !ok {
			t.Fatal("presenceSetReaction(\"❤️\") = false, want true")
		}

		mu.Lock()
		hits := getChatHits
		bodies := append([]string(nil), setBodies...)
		mu.Unlock()

		if hits != 1 {
			t.Fatalf("getChat calls = %d, want 1", hits)
		}
		if len(bodies) != 1 {
			t.Fatalf("setMessageReaction calls = %d, want 1", len(bodies))
		}
		if strings.Contains(bodies[0], "\ufe0f") {
			t.Fatalf("wire body still contains U+FE0F: %s", bodies[0])
		}
		var payload struct {
			ChatID    int64 `json:"chat_id"`
			MessageID int64 `json:"message_id"`
			IsBig     bool  `json:"is_big"`
			Reaction  []struct {
				Type  string `json:"type"`
				Emoji string `json:"emoji"`
			} `json:"reaction"`
		}
		if err := json.Unmarshal([]byte(bodies[0]), &payload); err != nil {
			t.Fatalf("decode wire body %s: %v", bodies[0], err)
		}
		if payload.ChatID != chatID || payload.MessageID != msgID {
			t.Errorf("chat_id/message_id = (%d, %d), want (%d, %d)", payload.ChatID, payload.MessageID, chatID, msgID)
		}
		if payload.IsBig {
			t.Errorf("is_big = true, want false")
		}
		if len(payload.Reaction) != 1 || payload.Reaction[0].Type != "emoji" || payload.Reaction[0].Emoji != "❤" {
			t.Errorf("reaction = %+v, want [{Type:emoji Emoji:❤}]", payload.Reaction)
		}

		key := presenceBotReactKey(chatID, msgID)
		if strings.HasPrefix(key, utilsCache.DataCachePrefix) {
			t.Fatalf("bot_react key %q must sit outside %q", key, utilsCache.DataCachePrefix)
		}
		val, err := mr.Get(key)
		if err != nil {
			t.Fatalf("bot_react key %q missing in Redis: %v", key, err)
		}
		var rec presenceBotReact
		if err := json.Unmarshal([]byte(val), &rec); err != nil {
			t.Fatalf("bot_react JSON %q: %v", val, err)
		}
		if rec != (presenceBotReact{Emoji: "❤", AuthorID: authorID, Source: "cons"}) {
			t.Errorf("bot_react = %+v, want {Emoji:❤ AuthorID:%d Source:cons}", rec, authorID)
		}
		if ttl := mr.TTL(key); ttl < 23*time.Hour || ttl > 24*time.Hour {
			t.Errorf("bot_react TTL = %v, want ~24h", ttl)
		}
	})

	t.Run("rejects non-standard emoji with zero telegram calls", func(t *testing.T) {
		withFastPresencePacer(t)
		utilsCache.SetupTestMemoryMarshaler(t)
		withChatRedis(t)

		var calls atomic.Int32
		bot := newWireTestBot(t, func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
		})

		if ok := presenceSetReaction(context.Background(), bot, -100777011, 10, 5, "😂", "cons"); ok {
			t.Fatal("presenceSetReaction(\"😂\") = true, want false")
		}
		if got := calls.Load(); got != 0 {
			t.Fatalf("Telegram HTTP calls for unsupported emoji = %d, want 0", got)
		}
	})

	t.Run("silent degradation on 400 errors and zero chatBreakers poisoning", func(t *testing.T) {
		withFastPresencePacer(t)
		utilsCache.SetupTestMemoryMarshaler(t)
		mr, _ := withChatRedis(t)
		chatSwap(t, &chatBreakers, map[string]*memberReqAIBreaker{
			chatconv.ProviderAnthropic:   newMemberReqAIBreaker(),
			chatconv.ProviderGoogle:      newMemberReqAIBreaker(),
			chatconv.ProviderFeatherless: newMemberReqAIBreaker(),
		})

		const (
			chatID int64 = -100777012
			msgID  int64 = 202
		)

		var (
			getChatCalls atomic.Int32
			reactErrDesc atomic.Value
		)
		reactErrDesc.Store("Bad Request: REACTION_INVALID")

		bot := newWireTestBot(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/bottest/getChat":
				getChatCalls.Add(1)
				_, _ = w.Write([]byte(`{"ok":true,"result":{"id":-100777012,"type":"supergroup"}}`))
			case "/bottest/setMessageReaction":
				w.WriteHeader(http.StatusBadRequest)
				desc := reactErrDesc.Load().(string)
				resp, _ := json.Marshal(map[string]any{
					"ok":          false,
					"error_code":  400,
					"description": desc,
				})
				_, _ = w.Write(resp)
			default:
				http.NotFound(w, r)
			}
		})

		// 1. REACTION_INVALID invalidates avail_react cache, returns false, writes no bot_react, leaves breakers clean.
		if ok := presenceSetReaction(context.Background(), bot, chatID, msgID, 12, "🔥", "cons"); ok {
			t.Fatal("REACTION_INVALID: got true, want false")
		}
		if mr.Exists(presenceBotReactKey(chatID, msgID)) {
			t.Error("REACTION_INVALID wrote bot_react metadata")
		}
		if got := getChatCalls.Load(); got != 1 {
			t.Fatalf("getChat calls after 1st attempt = %d, want 1", got)
		}

		// Because REACTION_INVALID evicted avail_react, a second attempt must re-fetch getChat.
		reactErrDesc.Store("Bad Request: MESSAGE_ID_INVALID")
		if ok := presenceSetReaction(context.Background(), bot, chatID, msgID+1, 12, "🔥", "cons"); ok {
			t.Fatal("MESSAGE_ID_INVALID: got true, want false")
		}
		if got := getChatCalls.Load(); got != 2 {
			t.Fatalf("getChat calls after REACTION_INVALID eviction = %d, want 2", got)
		}

		// MESSAGE_ID_INVALID must NOT evict avail_react, so a third attempt hits cache (still 2 getChat calls).
		if ok := presenceSetReaction(context.Background(), bot, chatID, msgID+2, 12, "🔥", "cons"); ok {
			t.Fatal("2nd MESSAGE_ID_INVALID: got true, want false")
		}
		if got := getChatCalls.Load(); got != 2 {
			t.Fatalf("getChat calls after MESSAGE_ID_INVALID = %d, want 2 (cache kept)", got)
		}

		assertChatBreakersClean(t)
	})
}

func TestPresenceAvailReactionsCache(t *testing.T) {
	t.Run("nil available_reactions caches under xenobot:cache: and allows all standard emojis", func(t *testing.T) {
		withFastPresencePacer(t)
		utilsCache.SetupTestMemoryMarshaler(t)
		withChatRedis(t)

		const chatID int64 = -100777020
		if !strings.HasPrefix(presenceAvailReactKey(chatID), utilsCache.DataCachePrefix) {
			t.Fatalf("avail_react key %q must live inside %q", presenceAvailReactKey(chatID), utilsCache.DataCachePrefix)
		}

		var getChatCalls, setCalls atomic.Int32
		bot := newWireTestBot(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/bottest/getChat":
				getChatCalls.Add(1)
				_, _ = w.Write([]byte(`{"ok":true,"result":{"id":-100777020,"type":"supergroup"}}`))
			case "/bottest/setMessageReaction":
				setCalls.Add(1)
				_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
			}
		})

		if !presenceSetReaction(context.Background(), bot, chatID, 1, 7, "🔥", "cons") {
			t.Fatal("1st presenceSetReaction failed")
		}
		if !presenceSetReaction(context.Background(), bot, chatID, 2, 7, "👍", "cons") {
			t.Fatal("2nd presenceSetReaction failed")
		}
		if got := getChatCalls.Load(); got != 1 {
			t.Fatalf("getChat calls = %d, want 1 (2nd call must hit cache)", got)
		}
		if got := setCalls.Load(); got != 2 {
			t.Fatalf("setMessageReaction calls = %d, want 2", got)
		}
	})

	t.Run("restricted available_reactions intersects with supportedReactionEmoji stripping FE0F", func(t *testing.T) {
		withFastPresencePacer(t)
		utilsCache.SetupTestMemoryMarshaler(t)
		withChatRedis(t)

		const chatID int64 = -100777021
		var getChatCalls, setCalls atomic.Int32
		bot := newWireTestBot(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/bottest/getChat":
				getChatCalls.Add(1)
				_, _ = w.Write([]byte(`{"ok":true,"result":{"id":-100777021,"type":"supergroup","available_reactions":[{"type":"emoji","emoji":"❤️"},{"type":"emoji","emoji":"🔥"},{"type":"emoji","emoji":"😂"},{"type":"custom_emoji","custom_emoji_id":"999"}]}}`))
			case "/bottest/setMessageReaction":
				setCalls.Add(1)
				_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
			}
		})

		if !presenceSetReaction(context.Background(), bot, chatID, 1, 7, "❤", "cons") {
			t.Error("normalized ❤ should be allowed")
		}
		if !presenceSetReaction(context.Background(), bot, chatID, 2, 7, "❤️", "cons") {
			t.Error("❤️ (with FE0F) should normalize and be allowed")
		}
		if presenceSetReaction(context.Background(), bot, chatID, 3, 7, "👍", "cons") {
			t.Error("👍 not in chat's available_reactions: got true, want false")
		}
		if presenceSetReaction(context.Background(), bot, chatID, 4, 7, "😂", "cons") {
			t.Error("😂 not in supportedReactionEmoji: got true, want false")
		}
		if got := getChatCalls.Load(); got != 1 {
			t.Errorf("getChat calls = %d, want 1", got)
		}
		if got := setCalls.Load(); got != 2 {
			t.Errorf("setMessageReaction calls = %d, want 2", got)
		}
	})

	t.Run("custom-only available_reactions skips setMessageReaction completely", func(t *testing.T) {
		withFastPresencePacer(t)
		utilsCache.SetupTestMemoryMarshaler(t)
		withChatRedis(t)

		const chatID int64 = -100777022
		var setCalls atomic.Int32
		bot := newWireTestBot(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/bottest/getChat":
				_, _ = w.Write([]byte(`{"ok":true,"result":{"id":-100777022,"type":"supergroup","available_reactions":[{"type":"custom_emoji","custom_emoji_id":"12345"}]}}`))
			case "/bottest/setMessageReaction":
				setCalls.Add(1)
				_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
			}
		})

		if presenceSetReaction(context.Background(), bot, chatID, 1, 7, "🔥", "cons") {
			t.Fatal("custom-only chat: presenceSetReaction() = true, want false")
		}
		if got := setCalls.Load(); got != 0 {
			t.Fatalf("custom-only chat made %d setMessageReaction calls, want 0", got)
		}
	})

	t.Run("getChat error fails closed", func(t *testing.T) {
		withFastPresencePacer(t)
		utilsCache.SetupTestMemoryMarshaler(t)
		withChatRedis(t)

		const chatID int64 = -100777023
		var setCalls atomic.Int32
		bot := newWireTestBot(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/bottest/getChat":
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"ok":false,"error_code":500,"description":"Internal Server Error"}`))
			case "/bottest/setMessageReaction":
				setCalls.Add(1)
				_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
			}
		})

		if presenceSetReaction(context.Background(), bot, chatID, 1, 7, "🔥", "cons") {
			t.Fatal("getChat error: presenceSetReaction() = true, want false")
		}
		if got := setCalls.Load(); got != 0 {
			t.Fatalf("getChat error made %d setMessageReaction calls, want 0", got)
		}
	})
}

func TestPresenceBotReact(t *testing.T) {
	mr, client := withChatRedis(t)
	ctx := context.Background()

	const (
		chatID   int64 = -100777030
		msgID    int64 = 88
		authorID int64 = 555
		userID   int64 = 777
	)

	want := presenceBotReact{Emoji: "🔥", AuthorID: authorID, Source: "cons"}
	presenceWriteBotReact(ctx, client, chatID, msgID, want)

	got, ok := presenceReadBotReact(ctx, client, chatID, msgID)
	if !ok || got != want {
		t.Fatalf("presenceReadBotReact() = (%+v, %v), want (%+v, true)", got, ok, want)
	}

	// Concurrent claim: exactly one caller wins, the second gets false.
	start := make(chan struct{})
	var wins atomic.Int32
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if presenceClaimBotReact(ctx, client, chatID, msgID) {
				wins.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if gotWins := wins.Load(); gotWins != 1 {
		t.Fatalf("concurrent presenceClaimBotReact winners = %d, want 1", gotWins)
	}
	if _, ok := presenceReadBotReact(ctx, client, chatID, msgID); ok {
		t.Error("bot_react key still present after claim")
	}

	// Cooldown extension sets 24h key outside xenobot:cache:.
	presenceExtendUserCooldown(ctx, client, chatID, userID)
	cdKey := presenceUserCooldownKey(chatID, userID)
	if val, err := mr.Get(cdKey); err != nil || val != "1" {
		t.Fatalf("cooldown key %q = (%q, %v), want (\"1\", nil)", cdKey, val, err)
	}
	if ttl := mr.TTL(cdKey); ttl < 23*time.Hour || ttl > 24*time.Hour {
		t.Errorf("cooldown TTL = %v, want ~24h", ttl)
	}
}
