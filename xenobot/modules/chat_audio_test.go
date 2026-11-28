//go:build testtools

package modules

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/Xenobot-72/xenobot/config"
	"github.com/uasneppy/Xenobot-72/xenobot/db/chatconv"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/chatprompt"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/featherless"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/gemini"
)

// chatTestOgg is the start of an OGG container, which is what a Telegram voice message is.
func chatTestOgg(tag string) []byte {
	return append([]byte("OggS\x00\x02"), bytes.Repeat([]byte(tag+"."), 24)...)
}

func TestChatExtractAudioRefs(t *testing.T) {
	const mib = 1 << 20
	for _, tc := range []struct {
		name  string
		msg   gotgbot.Message
		want  chatAudioKind // chatAudioNone means no ref
		fileI string
	}{
		{"voice message", gotgbot.Message{Voice: &gotgbot.Voice{FileId: "v1", Duration: 12, FileSize: 40000}}, chatAudioVoice, "v1"},
		{"voice of exactly the longest allowed length", gotgbot.Message{Voice: &gotgbot.Voice{FileId: "v1", Duration: chatMaxAudioSeconds}}, chatAudioVoice, "v1"},
		{"voice one second over the length cap", gotgbot.Message{Voice: &gotgbot.Voice{FileId: "v1", Duration: chatMaxAudioSeconds + 1}}, chatAudioNone, ""},
		{"voice of exactly the size cap", gotgbot.Message{Voice: &gotgbot.Voice{FileId: "v1", Duration: 10, FileSize: 5 * mib}}, chatAudioVoice, "v1"},
		{"voice one byte over the size cap", gotgbot.Message{Voice: &gotgbot.Voice{FileId: "v1", Duration: 10, FileSize: 5*mib + 1}}, chatAudioNone, ""},
		{"voice with no file", gotgbot.Message{Voice: &gotgbot.Voice{Duration: 3}}, chatAudioNone, ""},
		{"audio file", gotgbot.Message{Audio: &gotgbot.Audio{FileId: "a1", Duration: 200, FileSize: 3 * mib}}, chatAudioFile, "a1"},
		{"audio file over the length cap", gotgbot.Message{Audio: &gotgbot.Audio{FileId: "a1", Duration: 3600}}, chatAudioNone, ""},
		{"audio document by its type", gotgbot.Message{Document: &gotgbot.Document{FileId: "d1", MimeType: "audio/mpeg", FileName: "song.mp3"}}, chatAudioFile, "d1"},
		{"audio document by its extension", gotgbot.Message{Document: &gotgbot.Document{FileId: "d1", MimeType: "application/octet-stream", FileName: "take.FLAC"}}, chatAudioFile, "d1"},
		{"audio document over the size cap", gotgbot.Message{Document: &gotgbot.Document{FileId: "d1", MimeType: "audio/mpeg", FileSize: 5*mib + 1}}, chatAudioNone, ""},
		{"unknown binary document", gotgbot.Message{Document: &gotgbot.Document{FileId: "d1", MimeType: "application/octet-stream", FileName: "blob.bin"}}, chatAudioNone, ""},
		{"PDF is not audio", gotgbot.Message{Document: &gotgbot.Document{FileId: "d1", MimeType: "application/pdf", FileName: "a.pdf"}}, chatAudioNone, ""},
		{"text", gotgbot.Message{Text: "hello"}, chatAudioNone, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ref, ok := chatExtractAudioRef(&tc.msg)
			if (tc.want != chatAudioNone) != ok {
				t.Fatalf("chatExtractAudioRef ok = %v, want %v", ok, tc.want != chatAudioNone)
			}
			if !ok {
				if ref2, ok2 := chatExtractMediaRef(&tc.msg); ok2 && ref2.Kind == chatMediaAudio {
					t.Fatalf("chatExtractMediaRef returned an audio ref the audio extractor refused: %+v", ref2)
				}
				return
			}
			if ref.Kind != chatMediaAudio || ref.Audio != tc.want || ref.FileID != tc.fileI {
				t.Fatalf("ref = %+v, want audio kind %v of file %q", ref, tc.want, tc.fileI)
			}
			if viaMedia, ok := chatExtractMediaRef(&tc.msg); !ok || viaMedia != ref {
				t.Errorf("chatExtractMediaRef = %+v (%v), want the same ref", viaMedia, ok)
			}
			if viaReaction, ok := chatExtractReactionMediaRef(&tc.msg); !ok || viaReaction != ref {
				t.Errorf("chatExtractReactionMediaRef = %+v (%v), want the same ref", viaReaction, ok)
			}
		})
	}
}

func TestChatMessageHasAudio(t *testing.T) {
	for _, tc := range []struct {
		name string
		msg  gotgbot.Message
		want bool
	}{
		{"voice with no usable file still counts", gotgbot.Message{Voice: &gotgbot.Voice{Duration: 999}}, true},
		{"audio file", gotgbot.Message{Audio: &gotgbot.Audio{FileId: "a"}}, true},
		{"audio document", gotgbot.Message{Document: &gotgbot.Document{FileId: "d", MimeType: "audio/ogg"}}, true},
		{"PDF", gotgbot.Message{Document: &gotgbot.Document{FileId: "d", MimeType: "application/pdf"}}, false},
		{"round video message", gotgbot.Message{VideoNote: &gotgbot.VideoNote{FileId: "n"}}, false},
		{"text", gotgbot.Message{Text: "hi"}, false},
	} {
		if got := chatMessageHasAudio(&tc.msg); got != tc.want {
			t.Errorf("%s: chatMessageHasAudio = %v, want %v", tc.name, got, tc.want)
		}
	}
	if chatMessageHasAudio(nil) {
		t.Error("chatMessageHasAudio(nil) = true")
	}
}

// Only Google is sent audio: Anthropic has no audio input and Featherless cannot be fed one the bot
// is able to produce.
func TestChatProviderSupportsAudio(t *testing.T) {
	for provider, want := range map[string]bool{
		chatconv.ProviderGoogle:      true,
		chatconv.ProviderAnthropic:   false,
		chatconv.ProviderFeatherless: false,
		"":                           false,
	} {
		if got := chatProviderSupportsAudio(provider); got != want {
			t.Errorf("chatProviderSupportsAudio(%q) = %v, want %v", provider, got, want)
		}
	}
	audio := chatMediaRef{Kind: chatMediaAudio}
	image := chatMediaRef{Kind: chatMediaImage}
	if chatRefSendable(audio, chatconv.ProviderAnthropic) || !chatRefSendable(audio, chatconv.ProviderGoogle) {
		t.Error("an audio ref must be sendable to Google only")
	}
	if !chatRefSendable(image, chatconv.ProviderAnthropic) {
		t.Error("an image ref must be sendable to every provider")
	}
}

// The bytes decide the type, never the declared type or the file name.
func TestChatSniffAudio(t *testing.T) {
	pad := func(head []byte) []byte { return append(append([]byte{}, head...), make([]byte, 32)...) }
	for _, tc := range []struct {
		name string
		data []byte
		want string
	}{
		{"ogg opus voice message", chatTestOgg("op"), "audio/ogg"},
		{"flac", pad([]byte("fLaC")), "audio/flac"},
		{"mp3 with an ID3 tag", pad([]byte("ID3\x04\x00")), "audio/mp3"},
		{"mp3 frame, MPEG 1 layer III", pad([]byte{0xFF, 0xFB, 0x90, 0x00}), "audio/mp3"},
		{"mp3 frame, MPEG 2 layer III", pad([]byte{0xFF, 0xF3, 0x90, 0x00}), "audio/mp3"},
		{"aac in ADTS", pad([]byte{0xFF, 0xF1, 0x50, 0x80}), "audio/aac"},
		{"wav", pad([]byte("RIFF\x00\x00\x00\x00WAVEfmt ")), "audio/wav"},
		{"aiff", pad([]byte("FORM\x00\x00\x00\x00AIFF")), "audio/aiff"},
		{"m4a", pad([]byte("\x00\x00\x00\x20ftypM4A \x00\x00\x00\x00")), "audio/m4a"},
		{"mp4 video is not m4a", pad([]byte("\x00\x00\x00\x20ftypisom\x00\x00\x00\x00")), ""},
		{"avi in a RIFF container is not wav", pad([]byte("RIFF\x00\x00\x00\x00AVI LIST")), ""},
		{"png", pad([]byte("\x89PNG\r\n\x1a\n")), ""},
		{"jpeg", pad([]byte{0xFF, 0xD8, 0xFF, 0xE0}), ""},
		{"aac in ADTS, MPEG 2 without a CRC", pad([]byte{0xFF, 0xF9, 0x50, 0x80}), "audio/aac"},
		{"reserved MPEG version", pad([]byte{0xFF, 0xEB, 0x90, 0x00}), ""},
		{"text that merely mentions OggS", []byte("this is plain text that mentions OggS in the middle"), ""},
		{"one byte", []byte{0xFF}, ""},
		{"empty", nil, ""},
	} {
		if got := chatSniffAudio(tc.data); got != tc.want {
			t.Errorf("%s: chatSniffAudio = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// Every type the sniffer can return is one the Gemini client accepts, so a clip that survives the
// download is never refused by the client.
func TestChatSniffedAudioTypesAreSendable(t *testing.T) {
	for _, mime := range []string{"audio/ogg", "audio/flac", "audio/mp3", "audio/aac", "audio/wav", "audio/aiff", "audio/m4a"} {
		if !(gemini.Attachment{MIMEType: mime, Data: []byte{1}}).IsAudio() {
			t.Errorf("%s is not an audio type of the Gemini client", mime)
		}
	}
}

func TestChatAudioMediaNotes(t *testing.T) {
	voice := chatAttachment{MIMEType: "audio/ogg", Audio: chatAudioVoice}
	file := chatAttachment{MIMEType: "audio/mp3", Audio: chatAudioFile}
	frame := chatAttachment{MIMEType: "image/jpeg", Frame: chatFrameGIF}
	image := chatAttachment{MIMEType: "image/jpeg"}

	single := chatDownloadedMedia{Attachments: []chatAttachment{voice}}.mediaNotes(1)
	if len(single) != 1 || single[0] != chatprompt.AudioNote(1, 1, "a voice message") {
		t.Fatalf("single voice notes = %q", single)
	}
	if got := (chatDownloadedMedia{Attachments: []chatAttachment{file}}).mediaNotes(1); len(got) != 1 || !strings.Contains(got[0], "is an audio file, and you can hear it") {
		t.Fatalf("audio file notes = %q", got)
	}
	if got := (chatDownloadedMedia{}).mediaNotes(1); len(got) != 1 || got[0] != chatprompt.UnheardAudioNote {
		t.Fatalf("audio the provider did not get: notes = %q, want the unheard note", got)
	}
	if got := (chatDownloadedMedia{}).mediaNotes(0); len(got) != 0 {
		t.Fatalf("no audio and no attachments: notes = %q, want none", got)
	}
	// A reply to a voice message that is itself a voice message: one clip heard, one missed.
	if got := (chatDownloadedMedia{Attachments: []chatAttachment{voice}}).mediaNotes(2); len(got) != 2 || got[1] != chatprompt.UnheardAudioNote {
		t.Fatalf("one of two clips heard: notes = %q", got)
	}
	mixed := chatDownloadedMedia{Attachments: []chatAttachment{frame, voice}}.mediaNotes(1)
	if len(mixed) != 2 ||
		!strings.HasPrefix(mixed[0], "Attachment 1 of 2 is only one still preview frame of a GIF.") ||
		!strings.HasPrefix(mixed[1], "Attachment 2 of 2 is a voice message, and you can hear it.") {
		t.Fatalf("mixed notes = %q, want the frame note then the audio note, each naming its place", mixed)
	}
	if got := (chatDownloadedMedia{Attachments: []chatAttachment{image}}).mediaNotes(0); len(got) != 0 {
		t.Fatalf("a real image got notes %q", got)
	}

	// The request that is sent again without any attachment says the media is unavailable instead
	// of describing attachments that are gone.
	gone := chatDownloadedMedia{Attachments: []chatAttachment{frame, voice}}.noMediaNotes(1)
	if len(gone) != 2 || gone[0] != chatprompt.UnseenPictureNote || gone[1] != chatprompt.UnheardAudioNote {
		t.Fatalf("no-media notes = %q", gone)
	}
	if got := (chatDownloadedMedia{Attachments: []chatAttachment{{MIMEType: "application/pdf"}}}).noMediaNotes(0); len(got) != 0 {
		t.Fatalf("a PDF alone got no-media notes %q", got)
	}
}

// One audio block fails the whole Anthropic call (and would count toward the breaker), so the
// adapter drops a clip it is handed and still sends the pictures.
func TestChatAnthropicAdapterDropsAudio(t *testing.T) {
	withChatProviders(t, true, true, true)
	standIn := &chatProviderStandIn{}
	server := httptest.NewServer(http.HandlerFunc(standIn.handler))
	t.Cleanup(server.Close)
	chatSwap(t, &chatAnthropicEndpoint, server.URL)
	chatSwap(t, &chatProviderHTTP, server.Client())
	cfg := config.AppConfig
	chatSwap(t, &cfg.ChatAnthropicModel, "claude-haiku-5-5")

	jpeg := raidAITestJPEG("adapter")
	ogg := chatTestOgg("adapter")
	answer, err := chatProviderFor(chatconv.ProviderAnthropic).converse(context.Background(), chatRequest{
		Static: "static", Dynamic: "dynamic", User: "user",
		Attachments: []chatAttachment{{MIMEType: "audio/ogg", Data: ogg, Audio: chatAudioVoice}, {MIMEType: "image/jpeg", Data: jpeg}},
	})
	if err != nil {
		t.Fatalf("the adapter failed on a request that carried audio: %v", err)
	}
	if answer.Text == "" {
		t.Fatal("no answer")
	}
	if len(standIn.bodies) != 1 {
		t.Fatalf("provider calls = %d, want 1", len(standIn.bodies))
	}
	body := standIn.bodies[0]
	if strings.Contains(body, base64.StdEncoding.EncodeToString(ogg)) || strings.Contains(body, "audio/ogg") {
		t.Error("the Anthropic request carried the audio clip")
	}
	if !strings.Contains(body, base64.StdEncoding.EncodeToString(jpeg)) {
		t.Error("the Anthropic request lost the picture that came with the clip")
	}
}

// When the provider refuses a request that carries audio, it is sent again once without any
// attachment and with the prompt that says the media is unavailable.
func TestChatConverseRetriesWithoutRejectedMedia(t *testing.T) {
	audio := chatAttachment{MIMEType: "audio/ogg", Data: chatTestOgg("retry"), Audio: chatAudioVoice}
	image := chatAttachment{MIMEType: "image/jpeg", Data: raidAITestJPEG("retry")}
	badRequest := &gemini.HTTPError{Status: http.StatusBadRequest, GRPCStatus: "INVALID_ARGUMENT"}

	run := func(t *testing.T, provider string, req chatRequest, steps ...func(context.Context, chatRequest) (chatAnswer, error)) (chatAnswer, []chatRequest, error) {
		t.Helper()
		fake := &chatFakeProvider{}
		fake.script(steps...)
		chatSwap(t, &chatProviderOf, func(string) chatProvider { return fake })
		answer, err := chatConverse(context.Background(), context.Background(), chatCall{Lane: "conversation", ChatID: -1}, provider, req)
		return answer, fake.calls(), err
	}

	t.Run("Gemini rejects a clip: one retry with no attachment and the unavailable prompt", func(t *testing.T) {
		req := chatRequest{Static: "s", User: "with notes", UserNoMedia: "without media", Attachments: []chatAttachment{image, audio}}
		answer, calls, err := run(t, chatconv.ProviderGoogle, req, chatAnswerErr(badRequest), chatAnswerText("heard nothing"))
		if err != nil || answer.Text != "heard nothing" || !answer.TextOnly {
			t.Fatalf("answer = %+v, err = %v, want the retry's text marked TextOnly", answer, err)
		}
		if len(calls) != 2 {
			t.Fatalf("provider calls = %d, want 2", len(calls))
		}
		if len(calls[0].Attachments) != 2 || calls[0].User != "with notes" {
			t.Errorf("first call = %+v, want both attachments and the prompt with notes", calls[0])
		}
		if len(calls[1].Attachments) != 0 || calls[1].User != "without media" {
			t.Errorf("retry = %+v, want no attachment and the prompt without media", calls[1])
		}
	})

	t.Run("a retry that also fails is the retry's error and is not TextOnly", func(t *testing.T) {
		req := chatRequest{Static: "s", User: "u", Attachments: []chatAttachment{audio}}
		answer, calls, err := run(t, chatconv.ProviderGoogle, req, chatAnswerErr(badRequest), chatAnswerErr(&gemini.HTTPError{Status: http.StatusServiceUnavailable}))
		if err == nil || answer.TextOnly || len(calls) != 2 {
			t.Fatalf("answer = %+v, err = %v, calls = %d, want the retry's error after 2 calls", answer, err, len(calls))
		}
	})

	t.Run("no UserNoMedia keeps the prompt", func(t *testing.T) {
		req := chatRequest{Static: "s", User: "same", Attachments: []chatAttachment{audio}}
		_, calls, _ := run(t, chatconv.ProviderGoogle, req, chatAnswerErr(badRequest), chatAnswerText("ok"))
		if len(calls) != 2 || calls[1].User != "same" {
			t.Fatalf("calls = %+v, want the retry to keep the prompt", calls)
		}
	})

	for _, tc := range []struct {
		name     string
		provider string
		req      chatRequest
		err      error
	}{
		{"a 400 on a request with only pictures is not a media rejection", chatconv.ProviderGoogle,
			chatRequest{Static: "s", User: "u", Attachments: []chatAttachment{image}}, badRequest},
		{"a rejected key is never retried", chatconv.ProviderGoogle,
			chatRequest{Static: "s", User: "u", Attachments: []chatAttachment{audio}},
			&gemini.HTTPError{Status: http.StatusBadRequest, Reason: "API_KEY_INVALID"}},
		{"a 5xx is not a media rejection", chatconv.ProviderGoogle,
			chatRequest{Static: "s", User: "u", Attachments: []chatAttachment{audio}}, &gemini.HTTPError{Status: http.StatusInternalServerError}},
		{"a request with no attachment is never retried", chatconv.ProviderGoogle,
			chatRequest{Static: "s", User: "u"}, badRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, calls, err := run(t, tc.provider, tc.req, chatAnswerErr(tc.err))
			if err == nil || len(calls) != 1 {
				t.Fatalf("err = %v, calls = %d, want the first error after 1 call", err, len(calls))
			}
		})
	}

	t.Run("Featherless keeps its image retry and now sends the prompt without media notes", func(t *testing.T) {
		req := chatRequest{Static: "s", User: "with frame note", UserNoMedia: "picture unavailable", Attachments: []chatAttachment{image}}
		answer, calls, err := run(t, chatconv.ProviderFeatherless, req,
			chatAnswerErr(&featherless.HTTPError{Status: http.StatusBadRequest}), chatAnswerText("ok"))
		if err != nil || !answer.TextOnly || len(calls) != 2 {
			t.Fatalf("answer = %+v, err = %v, calls = %d", answer, err, len(calls))
		}
		if len(calls[1].Attachments) != 0 || calls[1].User != "picture unavailable" {
			t.Errorf("retry = %+v, want no image and the unavailable prompt", calls[1])
		}
	})

	t.Run("a shutdown skips the retry", func(t *testing.T) {
		fake := &chatFakeProvider{}
		fake.script(chatAnswerErr(badRequest))
		chatSwap(t, &chatProviderOf, func(string) chatProvider { return fake })
		root, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := chatConverse(context.Background(), root, chatCall{Lane: "conversation"}, chatconv.ProviderGoogle,
			chatRequest{Static: "s", User: "u", Attachments: []chatAttachment{audio}})
		if err == nil || len(fake.calls()) != 1 {
			t.Fatalf("err = %v, calls = %d, want 1 call and the error", err, len(fake.calls()))
		}
	})
}
