//go:build testtools

package modules

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
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

func decodeTelegramWireParams(r *http.Request) (map[string]string, error) {
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			return nil, err
		}
		out := make(map[string]string, len(r.MultipartForm.Value))
		for k, vals := range r.MultipartForm.Value {
			if len(vals) > 0 {
				out[k] = vals[0]
			}
		}
		return out, nil
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		out[k] = string(v)
	}
	return out, nil
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
		mu        sync.Mutex
		gotPaths  []string
		gotParams []map[string]string
		parseErr  error
	)
	bot := newWireTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		params, err := decodeTelegramWireParams(r)
		mu.Lock()
		if err != nil && parseErr == nil {
			parseErr = err
		}
		gotPaths = append(gotPaths, r.URL.Path)
		gotParams = append(gotParams, params)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	})

	ok := presenceClearReaction(context.Background(), bot, chatID, msgID)
	if !ok {
		t.Fatal("presenceClearReaction() = false, want true")
	}

	// Negative control: nil Reaction slice omits "reaction" on the wire in gotgbot/v2.
	if _, err := bot.SetMessageReactionWithContext(context.Background(), chatID, msgID, &gotgbot.SetMessageReactionOpts{
		Reaction: nil,
	}); err != nil {
		t.Fatalf("nil-reaction control call failed: %v", err)
	}

	mu.Lock()
	err := parseErr
	paths := append([]string(nil), gotPaths...)
	params := append([]map[string]string(nil), gotParams...)
	mu.Unlock()

	if err != nil {
		t.Fatalf("decodeTelegramWireParams failed: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("HTTP request count = %d, want 2", len(paths))
	}
	if paths[0] != "/bottest/setMessageReaction" {
		t.Fatalf("HTTP path = %q, want /bottest/setMessageReaction", paths[0])
	}

	clearParams := params[0]
	reactionRaw, present := clearParams["reaction"]
	if !present {
		t.Fatalf("clear wire params = %v, missing explicit \"reaction\" field", clearParams)
	}
	if reactionRaw != "[]" {
		t.Fatalf("clear wire \"reaction\" = %q, want \"[]\"", reactionRaw)
	}
	wireJSON, err := json.Marshal(map[string]json.RawMessage{"reaction": json.RawMessage(reactionRaw)})
	if err != nil || !strings.Contains(string(wireJSON), `"reaction":[]`) {
		t.Fatalf("serialized wire reaction = %s (err=%v), want \"reaction\":[]", string(wireJSON), err)
	}
	if clearParams["chat_id"] != "-100777001" || clearParams["message_id"] != "42" {
		t.Errorf("clear wire chat_id/message_id = (%q, %q), want (-100777001, 42)", clearParams["chat_id"], clearParams["message_id"])
	}

	if _, nilPresent := params[1]["reaction"]; nilPresent {
		t.Fatalf("negative control (Reaction: nil) unexpectedly sent \"reaction\"=%q", params[1]["reaction"])
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
			setParams   []map[string]string
			parseErr    error
			getChatHits int
		)
		bot := newWireTestBot(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/bottest/getChat":
				mu.Lock()
				getChatHits++
				mu.Unlock()
				_, _ = w.Write([]byte(`{"ok":true,"result":{"id":-100777010,"type":"supergroup"}}`))
			case "/bottest/setMessageReaction":
				params, err := decodeTelegramWireParams(r)
				mu.Lock()
				if err != nil && parseErr == nil {
					parseErr = err
				}
				setParams = append(setParams, params)
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
		err := parseErr
		hits := getChatHits
		captured := append([]map[string]string(nil), setParams...)
		mu.Unlock()

		if err != nil {
			t.Fatalf("decodeTelegramWireParams failed: %v", err)
		}
		if hits != 1 {
			t.Fatalf("getChat calls = %d, want 1", hits)
		}
		if len(captured) != 1 {
			t.Fatalf("setMessageReaction calls = %d, want 1", len(captured))
		}
		params := captured[0]
		if params["chat_id"] != "-100777010" || params["message_id"] != "101" {
			t.Errorf("chat_id/message_id = (%q, %q), want (-100777010, 101)", params["chat_id"], params["message_id"])
		}
		if isBig, set := params["is_big"]; set && isBig != "false" {
			t.Errorf("is_big = %q, want absent or false", isBig)
		}
		reactionRaw := params["reaction"]
		if strings.Contains(reactionRaw, "\ufe0f") {
			t.Fatalf("wire reaction still contains U+FE0F: %s", reactionRaw)
		}
		var reactions []struct {
			Type  string `json:"type"`
			Emoji string `json:"emoji"`
		}
		if err := json.Unmarshal([]byte(reactionRaw), &reactions); err != nil {
			t.Fatalf("decode wire reaction %q: %v", reactionRaw, err)
		}
		if len(reactions) != 1 || reactions[0].Type != "emoji" || reactions[0].Emoji != "❤" {
			t.Errorf("reaction = %+v, want [{Type:emoji Emoji:❤}]", reactions)
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

func TestPresenceAllowedPalette(t *testing.T) {
	t.Run("nil bot fails closed", func(t *testing.T) {
		palette, ok := presenceAllowedPalette(context.Background(), nil, -100777040)
		if ok || palette != nil {
			t.Fatalf("presenceAllowedPalette(nil bot) = (%v, %v), want (nil, false)", palette, ok)
		}
	})

	t.Run("empty available_reactions returns all 73 emojis and caches for 20m", func(t *testing.T) {
		withFastPresencePacer(t)
		utilsCache.SetupTestMemoryMarshaler(t)
		withChatRedis(t)

		const chatID int64 = -100777041
		var getChatCalls atomic.Int32
		bot := newWireTestBot(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/bottest/getChat" {
				getChatCalls.Add(1)
				_, _ = w.Write([]byte(`{"ok":true,"result":{"id":-100777041,"type":"supergroup"}}`))
				return
			}
			http.NotFound(w, r)
		})

		first, ok := presenceAllowedPalette(context.Background(), bot, chatID)
		if !ok {
			t.Fatal("1st presenceAllowedPalette() ok = false, want true")
		}
		if len(first) != 73 || !slices.Equal(first, supportedReactionEmoji) {
			t.Fatalf("1st palette len=%d equal=%v, want all 73 supportedReactionEmoji", len(first), slices.Equal(first, supportedReactionEmoji))
		}
		// Mutating the returned slice must never corrupt supportedReactionEmoji or cache.
		first[0] = "MUTATED"

		second, ok := presenceAllowedPalette(context.Background(), bot, chatID)
		if !ok {
			t.Fatal("2nd presenceAllowedPalette() ok = false, want true")
		}
		if !slices.Equal(second, supportedReactionEmoji) {
			t.Fatalf("2nd palette was mutated or drifted: first=%q want=%q", second[0], supportedReactionEmoji[0])
		}
		if got := getChatCalls.Load(); got != 1 {
			t.Fatalf("getChat calls = %d, want 1 (2nd call must hit 20m cache)", got)
		}
	})

	t.Run("restricted standard and custom list returns only normalized standard emojis", func(t *testing.T) {
		withFastPresencePacer(t)
		utilsCache.SetupTestMemoryMarshaler(t)
		withChatRedis(t)

		const chatID int64 = -100777042
		var getChatCalls atomic.Int32
		bot := newWireTestBot(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/bottest/getChat" {
				getChatCalls.Add(1)
				_, _ = w.Write([]byte(`{"ok":true,"result":{"id":-100777042,"type":"supergroup","available_reactions":[{"type":"emoji","emoji":"❤️"},{"type":"emoji","emoji":"🔥"},{"type":"emoji","emoji":"😂"},{"type":"custom_emoji","custom_emoji_id":"999"},{"type":"emoji","emoji":"❤"}]}}`))
				return
			}
			http.NotFound(w, r)
		})

		palette, ok := presenceAllowedPalette(context.Background(), bot, chatID)
		want := []string{"❤", "🔥"}
		if !ok || !slices.Equal(palette, want) {
			t.Fatalf("presenceAllowedPalette() = (%v, %v), want (%v, true)", palette, ok, want)
		}
		// Mutating returned slice must not pollute cached Allowed slice.
		palette[0] = "MUTATED"
		cached, ok := presenceAllowedPalette(context.Background(), bot, chatID)
		if !ok || !slices.Equal(cached, want) {
			t.Fatalf("cached presenceAllowedPalette() = (%v, %v), want (%v, true)", cached, ok, want)
		}
		if got := getChatCalls.Load(); got != 1 {
			t.Fatalf("getChat calls = %d, want 1", got)
		}
	})

	t.Run("custom-only available_reactions returns nil and false", func(t *testing.T) {
		withFastPresencePacer(t)
		utilsCache.SetupTestMemoryMarshaler(t)
		withChatRedis(t)

		const chatID int64 = -100777043
		bot := newWireTestBot(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/bottest/getChat" {
				_, _ = w.Write([]byte(`{"ok":true,"result":{"id":-100777043,"type":"supergroup","available_reactions":[{"type":"custom_emoji","custom_emoji_id":"12345"}]}}`))
				return
			}
			http.NotFound(w, r)
		})

		palette, ok := presenceAllowedPalette(context.Background(), bot, chatID)
		if ok || palette != nil {
			t.Fatalf("custom-only presenceAllowedPalette() = (%v, %v), want (nil, false)", palette, ok)
		}
	})

	t.Run("getChat HTTP 500 error fails closed without touching chatBreakers", func(t *testing.T) {
		withFastPresencePacer(t)
		utilsCache.SetupTestMemoryMarshaler(t)
		withChatRedis(t)
		chatSwap(t, &chatBreakers, map[string]*memberReqAIBreaker{
			chatconv.ProviderAnthropic:   newMemberReqAIBreaker(),
			chatconv.ProviderGoogle:      newMemberReqAIBreaker(),
			chatconv.ProviderFeatherless: newMemberReqAIBreaker(),
		})

		const chatID int64 = -100777044
		bot := newWireTestBot(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/bottest/getChat" {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"ok":false,"error_code":500,"description":"Internal Server Error"}`))
				return
			}
			http.NotFound(w, r)
		})

		palette, ok := presenceAllowedPalette(context.Background(), bot, chatID)
		if ok || palette != nil {
			t.Fatalf("getChat error presenceAllowedPalette() = (%v, %v), want (nil, false)", palette, ok)
		}
		assertChatBreakersClean(t)
	})
}
