//go:build testtools

package modules

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/Xenobot-72/xenobot/db/chatconv"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/chatprompt"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/gemini"
)

// chatServeFiles serves Telegram file bodies under /files/<id> and returns the server's base URL.
func chatServeFiles(t *testing.T, files map[string][]byte) (*httptest.Server, string) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := strings.Index(r.URL.Path, "/files/")
		if idx < 0 {
			http.NotFound(w, r)
			return
		}
		body, ok := files[r.URL.Path[idx+1:]]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	return server, server.URL
}

const (
	audioNoteHeard   = "is a voice message, and you can hear it"
	audioNoteUnheard = "not available to you, so you cannot hear it"
)

// A voice message is a reaction candidate only where the group's provider can hear it. A clip that
// is not sent (too long, not fetchable, not audio, or refused by the provider) with no words to go on
// is shed, never reacted to from its [voice] label.
func TestChatReactAudio(t *testing.T) {
	env := newChatReactEnv(t)
	alice := env.member("Alice")
	ogg := chatTestOgg("react")
	env.mu.Lock()
	env.files["files/voice1"] = ogg
	env.files["files/notaudio"] = raidAITestJPEG("react-not-audio")
	env.mu.Unlock()
	setProvider := func(provider string) {
		env.mu.Lock()
		env.convSettings.Provider = provider
		env.mu.Unlock()
	}
	voice := func(id string, seconds int64, caption string) *gotgbot.Message {
		msg := env.message(alice, "")
		msg.Caption = caption
		msg.Voice = &gotgbot.Voice{FileId: id, Duration: seconds}
		return msg
	}

	t.Run("Google hears a voice message with no words", func(t *testing.T) {
		setProvider(chatconv.ProviderGoogle)
		before := len(env.provider.calls())
		env.provider.script(chatAnswerText(`{"emoji":"🔥"}`))
		env.deliver(voice("voice1", 9, ""))

		calls := env.provider.calls()
		if len(calls) != before+1 {
			t.Fatalf("provider calls = %d, want %d", len(calls), before+1)
		}
		got := calls[before]
		if len(got.Attachments) != 1 || got.Attachments[0].MIMEType != "audio/ogg" || got.Attachments[0].Audio != chatAudioVoice {
			t.Fatalf("attachments = %+v, want the one OGG voice clip", got.Attachments)
		}
		if !strings.Contains(got.User, audioNoteHeard) {
			t.Errorf("prompt does not say the clip is a voice message it can hear:\n%s", got.User)
		}
		if got.UserNoMedia == "" || strings.Contains(got.UserNoMedia, audioNoteHeard) || !strings.Contains(got.UserNoMedia, audioNoteUnheard) {
			t.Errorf("the prompt for a retry without media is wrong:\n%s", got.UserNoMedia)
		}
	})

	t.Run("a clip over the length cap with no words is not a job", func(t *testing.T) {
		setProvider(chatconv.ProviderGoogle)
		before := len(env.provider.calls())
		env.deliver(voice("voice1", chatMaxAudioSeconds+1, ""))
		if len(env.provider.calls()) != before {
			t.Fatal("a clip that was never sent reached the provider")
		}
	})

	t.Run("a clip that cannot be fetched, or is not audio, is shed rather than reacted to from its label", func(t *testing.T) {
		setProvider(chatconv.ProviderGoogle)
		for _, id := range []string{"missing-voice", "notaudio"} {
			before := len(env.provider.calls())
			shedBefore := chatReactShedCount.Load()
			env.deliver(voice(id, 4, ""))
			if len(env.provider.calls()) != before {
				t.Fatalf("%s: the provider was asked to react to a clip that was never attached", id)
			}
			if chatReactShedCount.Load() != shedBefore+1 {
				t.Fatalf("%s: shed count = %d, want %d", id, chatReactShedCount.Load(), shedBefore+1)
			}
		}
	})

	t.Run("a provider that refuses the clip is shed, never reacted to from the label", func(t *testing.T) {
		setProvider(chatconv.ProviderGoogle)
		before := len(env.provider.calls())
		reactsBefore := len(env.callsFor("setMessageReaction"))
		shedBefore := chatReactShedCount.Load()
		env.provider.script(
			chatAnswerErr(&gemini.HTTPError{Status: http.StatusBadRequest, GRPCStatus: "INVALID_ARGUMENT"}),
			chatAnswerText(`{"emoji":"🔥"}`),
		)
		env.deliver(voice("voice1", 5, ""))
		calls := env.provider.calls()
		if len(calls) != before+2 || len(calls[before+1].Attachments) != 0 {
			t.Fatalf("provider calls = %d, want the refused request and one retry without the clip", len(calls)-before)
		}
		if got := len(env.callsFor("setMessageReaction")); got != reactsBefore {
			t.Fatalf("setMessageReaction calls = %d, want %d: a reaction was set from a label", got, reactsBefore)
		}
		if chatReactShedCount.Load() != shedBefore+1 {
			t.Fatalf("shed count = %d, want %d", chatReactShedCount.Load(), shedBefore+1)
		}
	})

	t.Run("a provider that cannot hear gets no job for a bare voice message", func(t *testing.T) {
		for _, provider := range []string{chatconv.ProviderAnthropic, chatconv.ProviderFeatherless} {
			setProvider(provider)
			before := len(env.provider.calls())
			shedBefore := chatReactShedCount.Load()
			env.deliver(voice("voice1", 6, ""))
			if len(env.provider.calls()) != before {
				t.Fatalf("%s: a bare voice message reached the provider", provider)
			}
			if chatReactShedCount.Load() != shedBefore {
				t.Fatalf("%s: a job was started for a voice message nobody can hear", provider)
			}
		}
	})

	t.Run("a provider that cannot hear judges a captioned voice message on the words and says so", func(t *testing.T) {
		setProvider(chatconv.ProviderAnthropic)
		before := len(env.provider.calls())
		env.provider.script(chatAnswerText(`{"emoji":"👀"}`))
		env.deliver(voice("voice1", 6, "my cat sings"))
		calls := env.provider.calls()
		if len(calls) != before+1 {
			t.Fatalf("provider calls = %d, want %d", len(calls), before+1)
		}
		got := calls[before]
		if len(got.Attachments) != 0 || got.UserNoMedia != "" {
			t.Fatalf("attachments = %+v, no-media prompt = %q, want none", got.Attachments, got.UserNoMedia)
		}
		if !strings.Contains(got.User, "my cat sings") || !strings.Contains(got.User, audioNoteUnheard) {
			t.Errorf("prompt lacks the caption or the not-available note:\n%s", got.User)
		}
	})
}

// /inspect on a voice message: heard on Google; with nothing to judge on a provider that cannot hear
// it, a clip that is too long or unfetchable, no verdict is invented.
func TestInspectAudio(t *testing.T) {
	env := newChatInspectEnv(t)
	chatSwap(t, &chatToyUserCooldown, 0)
	chatSwap(t, &chatToyChatCooldown, 0)
	alice := env.member("Alice")
	bob := env.member("Bob")
	env.mu.Lock()
	env.files["files/voice1"] = chatTestOgg("inspect")
	env.mu.Unlock()
	setProvider := func(provider string) {
		env.mu.Lock()
		env.convSettings.Provider = provider
		env.mu.Unlock()
	}
	target := func(id string, seconds int64, caption string) *gotgbot.Message {
		msg := env.message(bob, 0, "")
		msg.Caption = caption
		msg.Voice = &gotgbot.Voice{FileId: id, Duration: seconds}
		return msg
	}
	lastReply := func() string {
		msgs := env.callsFor("sendMessage")
		return unquoteWireParam(msgs[len(msgs)-1].Params["text"])
	}

	t.Run("Google hears the clip", func(t *testing.T) {
		setProvider(chatconv.ProviderGoogle)
		before := len(env.provider.calls())
		env.provider.script(chatAnswerText(`{"verdict":"Clip filed.","emoji":"👀"}`))
		env.deliverMessage(env.inspectCmd(alice, 0, target("voice1", 8, "")))
		calls := env.provider.calls()
		if len(calls) != before+1 {
			t.Fatalf("provider calls = %d, want %d", len(calls), before+1)
		}
		got := calls[before]
		if len(got.Attachments) != 1 || got.Attachments[0].MIMEType != "audio/ogg" {
			t.Fatalf("attachments = %+v, want the one OGG clip", got.Attachments)
		}
		if !strings.Contains(got.User, audioNoteHeard) {
			t.Errorf("prompt does not say the clip is a voice message it can hear: %s", got.User)
		}
	})

	t.Run("a provider that cannot hear, with no words, is not asked", func(t *testing.T) {
		for _, provider := range []string{chatconv.ProviderAnthropic, chatconv.ProviderFeatherless} {
			setProvider(provider)
			before := len(env.provider.calls())
			env.deliverMessage(env.inspectCmd(alice, 0, target("voice1", 8, "")))
			if len(env.provider.calls()) != before {
				t.Fatalf("%s: a clip nobody could play reached the provider", provider)
			}
			if got := lastReply(); got != chatCanned(t, "inspect_usage") {
				t.Fatalf("%s: reply = %q, want inspect_usage", provider, got)
			}
		}
	})

	t.Run("a clip over the length cap, with no words, is not judged", func(t *testing.T) {
		setProvider(chatconv.ProviderGoogle)
		before := len(env.provider.calls())
		env.deliverMessage(env.inspectCmd(alice, 0, target("voice1", chatMaxAudioSeconds+1, "")))
		if len(env.provider.calls()) != before {
			t.Fatal("a clip that was never sent reached the provider")
		}
		if got := lastReply(); got != chatCanned(t, "inspect_usage") {
			t.Fatalf("reply = %q, want inspect_usage", got)
		}
	})

	t.Run("a captioned clip on a provider that cannot hear is judged on the words and the prompt says so", func(t *testing.T) {
		setProvider(chatconv.ProviderAnthropic)
		before := len(env.provider.calls())
		env.provider.script(chatAnswerText(`{"verdict":"Words filed.","emoji":"👀"}`))
		env.deliverMessage(env.inspectCmd(alice, 0, target("voice1", 8, "behold my singing")))
		calls := env.provider.calls()
		if len(calls) != before+1 {
			t.Fatalf("provider calls = %d, want %d", len(calls), before+1)
		}
		if len(calls[before].Attachments) != 0 {
			t.Fatalf("attachments = %+v, want none", calls[before].Attachments)
		}
		if !strings.Contains(calls[before].User, "behold my singing") || !strings.Contains(calls[before].User, chatprompt.UnheardAudioNote) {
			t.Errorf("prompt lacks the caption or the not-available note: %s", calls[before].User)
		}
	})

	t.Run("a clip that cannot be fetched is a malfunction, never an invented verdict", func(t *testing.T) {
		setProvider(chatconv.ProviderGoogle)
		before := len(env.provider.calls())
		reactsBefore := len(env.callsFor("setMessageReaction"))
		env.deliverMessage(env.inspectCmd(alice, 0, target("missing-voice", 8, "")))
		if len(env.provider.calls()) != before {
			t.Fatal("the provider was asked about a clip that was never attached")
		}
		if got := lastReply(); got != chatCanned(t, "inspect_malfunction") {
			t.Fatalf("reply = %q, want inspect_malfunction", got)
		}
		if got := len(env.callsFor("setMessageReaction")); got != reactsBefore {
			t.Fatalf("setMessageReaction calls = %d, want %d", got, reactsBefore)
		}
	})

	t.Run("a provider that refuses the clip is a malfunction, never a verdict from the label", func(t *testing.T) {
		setProvider(chatconv.ProviderGoogle)
		before := len(env.provider.calls())
		reactsBefore := len(env.callsFor("setMessageReaction"))
		env.provider.script(
			chatAnswerErr(&gemini.HTTPError{Status: http.StatusBadRequest, GRPCStatus: "INVALID_ARGUMENT"}),
			chatAnswerText(`{"verdict":"Invented.","emoji":"👀"}`),
		)
		env.deliverMessage(env.inspectCmd(alice, 0, target("voice1", 8, "")))
		if got := len(env.provider.calls()); got != before+2 {
			t.Fatalf("provider calls = %d, want the refused request and one retry", got-before)
		}
		if got := lastReply(); got != chatCanned(t, "inspect_malfunction") {
			t.Fatalf("reply = %q, want inspect_malfunction", got)
		}
		if got := len(env.callsFor("setMessageReaction")); got != reactsBefore {
			t.Fatalf("setMessageReaction calls = %d, want %d: a verdict was posted for a clip the model never heard", got, reactsBefore)
		}
	})
}

// A conversation turn whose clip Gemini refuses is answered from the words, with a prompt that says
// the clip is not available instead of one that claims it can be heard, and the refusal is not a
// failure of the provider.
func TestChatConversationAudioRejected(t *testing.T) {
	env := newChatEnv(t)
	if err := chatconv.SetProvider(t.Context(), env.chatID, chatconv.ProviderGoogle, 1); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	fake := &chatFakeProvider{}
	chatSwap(t, &chatProviderOf, func(string) chatProvider { return fake })
	server, files := chatServeFiles(t, map[string][]byte{"files/voice1": chatTestOgg("rejected")})
	_ = server
	env.bot.BotClient = &chatFileFake{chatTelegramFake: env.fake, base: files}

	fake.script(
		chatAnswerErr(&gemini.HTTPError{Status: http.StatusBadRequest, GRPCStatus: "INVALID_ARGUMENT"}),
		chatAnswerText("I could not play that one."),
	)
	handle := "@" + env.bot.Username
	msg := env.message(env.member("Alice"), "")
	msg.Caption = handle + " what did I say?"
	msg.CaptionEntities = []gotgbot.MessageEntity{{Type: "mention", Offset: 0, Length: int64(len(handle))}}
	msg.Voice = &gotgbot.Voice{FileId: "voice1", Duration: 5}
	env.deliver(msg)

	calls := fake.calls()
	if len(calls) != 2 {
		t.Fatalf("provider calls = %d, want the refused request and one retry", len(calls))
	}
	if len(calls[0].Attachments) != 1 || !strings.Contains(calls[0].User, audioNoteHeard) {
		t.Errorf("first request = %d attachments, prompt:\n%s", len(calls[0].Attachments), calls[0].User)
	}
	if len(calls[1].Attachments) != 0 || strings.Contains(calls[1].User, audioNoteHeard) || !strings.Contains(calls[1].User, audioNoteUnheard) {
		t.Errorf("retry = %d attachments, prompt:\n%s", len(calls[1].Attachments), calls[1].User)
	}
	if sends := env.sends(); len(sends) != 1 || sends[0] != "I could not play that one." {
		t.Fatalf("sends = %q, want the retry's answer", sends)
	}
	if chatBreaker(chatconv.ProviderGoogle).isPaused(env.chatID, raidNow()) {
		t.Error("a refused clip counted toward pausing the provider")
	}
}
