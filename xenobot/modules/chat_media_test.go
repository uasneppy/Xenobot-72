//go:build testtools

package modules

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/Xenobot-72/xenobot/config"
	"github.com/uasneppy/Xenobot-72/xenobot/db/chatconv"
)

// chatFileFake wraps chatTelegramFake to answer getFile and point FileURL at a test HTTP server.
type chatFileFake struct {
	*chatTelegramFake
	base string
}

func (f *chatFileFake) FileURL(token, filePath string, _ *gotgbot.RequestOpts) string {
	return f.base + "/file/bot" + token + "/" + filePath
}

func (f *chatFileFake) RequestWithContext(
	ctx context.Context, token, method string, params map[string]any, opts *gotgbot.RequestOpts,
) (json.RawMessage, error) {
	if method == "getFile" {
		id := fmt.Sprint(params["file_id"])
		return json.RawMessage(fmt.Sprintf(`{"file_id":%q,"file_unique_id":"u-%s","file_size":100,"file_path":"files/%s"}`, id, id, id)), nil
	}
	return f.chatTelegramFake.RequestWithContext(ctx, token, method, params, opts)
}

func TestChatExtractMediaRef(t *testing.T) {
	thumb := &gotgbot.PhotoSize{FileId: "thumb", Width: 90, Height: 90}
	cases := []struct {
		name   string
		msg    gotgbot.Message
		wantID string
		kind   chatMediaKind
	}{
		{"photo picks nearest 1024", gotgbot.Message{Photo: photoSizes()}, "p800", chatMediaImage},
		{"static sticker uses file", gotgbot.Message{Sticker: &gotgbot.Sticker{FileId: "stk", Type: "regular", Emoji: "😀"}}, "stk", chatMediaImage},
		{"animated sticker uses thumbnail", gotgbot.Message{Sticker: &gotgbot.Sticker{FileId: "tgs", IsAnimated: true, Thumbnail: thumb}}, "thumb", chatMediaImage},
		{"video sticker uses thumbnail", gotgbot.Message{Sticker: &gotgbot.Sticker{FileId: "webm", IsVideo: true, Thumbnail: thumb}}, "thumb", chatMediaImage},
		{"GIF uses thumbnail before document", gotgbot.Message{
			Animation: &gotgbot.Animation{FileId: "gif", Thumbnail: thumb},
			Document:  &gotgbot.Document{FileId: "gif-doc", MimeType: "video/mp4"},
		}, "thumb", chatMediaImage},
		{"video uses thumbnail", gotgbot.Message{Video: &gotgbot.Video{FileId: "vid", Thumbnail: thumb}}, "thumb", chatMediaImage},
		{"image document", gotgbot.Message{Document: &gotgbot.Document{FileId: "imgdoc", FileName: "pic.png", MimeType: "image/png"}}, "imgdoc", chatMediaImage},
		{"octet-stream image document", gotgbot.Message{Document: &gotgbot.Document{FileId: "octimg", FileName: "pic.jpg", MimeType: "application/octet-stream"}}, "octimg", chatMediaImage},
		{"PDF document", gotgbot.Message{Document: &gotgbot.Document{FileId: "pdfdoc", FileName: "report.pdf", MimeType: "application/pdf"}}, "pdfdoc", chatMediaPDF},
		{"octet-stream PDF document", gotgbot.Message{Document: &gotgbot.Document{FileId: "octpdf", FileName: "report.pdf", MimeType: "application/octet-stream"}}, "octpdf", chatMediaPDF},
		{"text code file", gotgbot.Message{Document: &gotgbot.Document{FileId: "pydoc", FileName: "script.py", MimeType: "application/octet-stream"}}, "pydoc", chatMediaText},
		{"octet-stream unknown binary skipped", gotgbot.Message{Document: &gotgbot.Document{FileId: "octbin", FileName: "blob.bin", MimeType: "application/octet-stream"}}, "", 0},
		{"json document", gotgbot.Message{Document: &gotgbot.Document{FileId: "jsondoc", FileName: "data.json", MimeType: "application/json"}}, "jsondoc", chatMediaText},
		{"oversized text file skipped", gotgbot.Message{Document: &gotgbot.Document{FileId: "bigtxt", FileName: "huge.txt", MimeType: "text/plain", FileSize: chatMaxTextFileBytes + 1}}, "", 0},
		{"zip archive skipped", gotgbot.Message{Document: &gotgbot.Document{FileId: "zip", FileName: "a.zip", MimeType: "application/zip"}}, "", 0},
		{"plain text has no media", gotgbot.Message{Text: "hello"}, "", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ref, ok := chatExtractMediaRef(&tc.msg)
			if tc.wantID == "" {
				if ok {
					t.Fatalf("chatExtractMediaRef = %+v, want none", ref)
				}
				return
			}
			if !ok || ref.FileID != tc.wantID || ref.Kind != tc.kind {
				t.Errorf("chatExtractMediaRef = %+v (ok %v), want id=%q kind=%v", ref, ok, tc.wantID, tc.kind)
			}
		})
	}
}

// TestChatExtractFrameRefs holds which messages give the AI a preview frame, and which kind: a GIF,
// a video, a round video message, an animated sticker, or a GIF or video sent as a plain file. A
// real image carries no frame kind. Extraction is pure, so this table makes no request.
func TestChatExtractFrameRefs(t *testing.T) {
	thumb := &gotgbot.PhotoSize{FileId: "thumb", Width: 90, Height: 90}
	huge := &gotgbot.PhotoSize{FileId: "huge", Width: 90, Height: 90, FileSize: chatMaxImageBytes + 1}
	cases := []struct {
		name      string
		msg       gotgbot.Message
		wantID    string
		wantFrame chatFrameKind
	}{
		{"photo is a real image", gotgbot.Message{Photo: photoSizes()}, "p800", chatFrameNone},
		{"static sticker is a real image", gotgbot.Message{Sticker: &gotgbot.Sticker{FileId: "stk", Type: "regular"}}, "stk", chatFrameNone},
		{"animated sticker", gotgbot.Message{Sticker: &gotgbot.Sticker{FileId: "tgs", IsAnimated: true, Thumbnail: thumb}}, "thumb", chatFrameSticker},
		{"video sticker", gotgbot.Message{Sticker: &gotgbot.Sticker{FileId: "webm", IsVideo: true, Thumbnail: thumb}}, "thumb", chatFrameSticker},
		{"animated sticker with no thumbnail gives nothing", gotgbot.Message{Sticker: &gotgbot.Sticker{FileId: "tgs", IsAnimated: true}}, "", chatFrameNone},
		{"GIF", gotgbot.Message{Animation: &gotgbot.Animation{FileId: "gif", Thumbnail: thumb}}, "thumb", chatFrameGIF},
		{"GIF is not mistaken for its document", gotgbot.Message{
			Animation: &gotgbot.Animation{FileId: "gif", Thumbnail: thumb},
			Document:  &gotgbot.Document{FileId: "gif-doc", MimeType: "video/mp4", Thumbnail: &gotgbot.PhotoSize{FileId: "doc-thumb"}},
		}, "thumb", chatFrameGIF},
		{"GIF with no thumbnail gives nothing", gotgbot.Message{Animation: &gotgbot.Animation{FileId: "gif"}}, "", chatFrameNone},
		{"video", gotgbot.Message{Video: &gotgbot.Video{FileId: "vid", Thumbnail: thumb}}, "thumb", chatFrameVideo},
		{"video prefers the cover its sender chose", gotgbot.Message{Video: &gotgbot.Video{
			FileId: "vid", Thumbnail: thumb, Cover: []gotgbot.PhotoSize{{FileId: "cover", Width: 1000, Height: 1000}},
		}}, "cover", chatFrameVideo},
		{"video with no thumbnail gives nothing", gotgbot.Message{Video: &gotgbot.Video{FileId: "vid"}}, "", chatFrameNone},
		{"video with an oversize thumbnail gives nothing", gotgbot.Message{Video: &gotgbot.Video{FileId: "vid", Thumbnail: huge}}, "", chatFrameNone},
		{"round video message", gotgbot.Message{VideoNote: &gotgbot.VideoNote{FileId: "note", Thumbnail: thumb}}, "thumb", chatFrameVideoNote},
		{"round video message with no thumbnail gives nothing", gotgbot.Message{VideoNote: &gotgbot.VideoNote{FileId: "note"}}, "", chatFrameNone},
		{"mp4 file", gotgbot.Message{Document: &gotgbot.Document{FileId: "mp4", FileName: "clip.mp4", MimeType: "video/mp4", Thumbnail: thumb}}, "thumb", chatFrameVideo},
		{"unnamed mime mov file", gotgbot.Message{Document: &gotgbot.Document{FileId: "mov", FileName: "clip.MOV", MimeType: "application/octet-stream", Thumbnail: thumb}}, "thumb", chatFrameVideo},
		{"GIF file", gotgbot.Message{Document: &gotgbot.Document{FileId: "gifdoc", FileName: "funny.gif", MimeType: "image/gif", Thumbnail: thumb}}, "thumb", chatFrameGIF},
		{"GIF file with no thumbnail is not sent as an image", gotgbot.Message{Document: &gotgbot.Document{FileId: "gifdoc", FileName: "funny.gif", MimeType: "image/gif"}}, "", chatFrameNone},
		{"video file with no thumbnail gives nothing", gotgbot.Message{Document: &gotgbot.Document{FileId: "mp4", FileName: "clip.mp4", MimeType: "video/mp4"}}, "", chatFrameNone},
		{"PDF is not a frame", gotgbot.Message{Document: &gotgbot.Document{FileId: "pdf", FileName: "a.pdf", MimeType: "application/pdf", Thumbnail: thumb}}, "pdf", chatFrameNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ref, ok := chatExtractMediaRef(&tc.msg)
			if tc.wantID == "" {
				if ok {
					t.Fatalf("chatExtractMediaRef = %+v, want none", ref)
				}
				return
			}
			if !ok || ref.FileID != tc.wantID || ref.Frame != tc.wantFrame {
				t.Fatalf("chatExtractMediaRef = %+v (ok %v), want id=%q frame=%v", ref, ok, tc.wantID, tc.wantFrame)
			}
		})
	}

	t.Run("reactions and inspect see the same frames, never a PDF or a text file", func(t *testing.T) {
		for _, tc := range cases {
			ref, ok := chatExtractReactionMediaRef(&tc.msg)
			wantOK := tc.wantID != "" && tc.wantID != "pdf"
			if ok != wantOK || (ok && (ref.FileID != tc.wantID || ref.Frame != tc.wantFrame)) {
				t.Errorf("%s: chatExtractReactionMediaRef = %+v (ok %v), want id=%q frame=%v ok=%v", tc.name, ref, ok, tc.wantID, tc.wantFrame, wantOK)
			}
		}
	})
}

func TestChatMessageHasPicture(t *testing.T) {
	thumb := &gotgbot.PhotoSize{FileId: "thumb"}
	for _, tc := range []struct {
		name string
		msg  gotgbot.Message
		want bool
	}{
		{"text", gotgbot.Message{Text: "hello"}, false},
		{"photo", gotgbot.Message{Photo: photoSizes()}, true},
		{"sticker", gotgbot.Message{Sticker: &gotgbot.Sticker{FileId: "s"}}, true},
		{"GIF with no thumbnail still counts", gotgbot.Message{Animation: &gotgbot.Animation{FileId: "g"}}, true},
		{"video", gotgbot.Message{Video: &gotgbot.Video{FileId: "v", Thumbnail: thumb}}, true},
		{"round video message", gotgbot.Message{VideoNote: &gotgbot.VideoNote{FileId: "n"}}, true},
		{"mp4 file", gotgbot.Message{Document: &gotgbot.Document{FileId: "d", FileName: "a.mp4", MimeType: "video/mp4"}}, true},
		{"image file", gotgbot.Message{Document: &gotgbot.Document{FileId: "d", FileName: "a.png", MimeType: "image/png"}}, true},
		{"PDF", gotgbot.Message{Document: &gotgbot.Document{FileId: "d", FileName: "a.pdf", MimeType: "application/pdf"}}, false},
		{"voice", gotgbot.Message{Voice: &gotgbot.Voice{FileId: "v"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := chatMessageHasPicture(&tc.msg); got != tc.want {
				t.Errorf("chatMessageHasPicture = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestChatFrameNotes holds what the model is told about attachments that are only a preview frame:
// nothing for real images and PDFs, one note per frame that names its place among the attachments.
func TestChatFrameNotes(t *testing.T) {
	if notes := (chatDownloadedMedia{Attachments: []chatAttachment{{MIMEType: "image/jpeg"}, {MIMEType: "application/pdf"}}}).mediaNotes(0); len(notes) != 0 {
		t.Fatalf("real images and PDFs got notes %q, want none", notes)
	}
	single := chatDownloadedMedia{Attachments: []chatAttachment{{MIMEType: "image/jpeg", Frame: chatFrameVideoNote}}}.mediaNotes(0)
	if len(single) != 1 || !strings.HasPrefix(single[0], "The attached image is only one still preview frame of a round video message.") {
		t.Fatalf("single frame note = %q", single)
	}
	both := chatDownloadedMedia{Attachments: []chatAttachment{
		{MIMEType: "image/jpeg"},
		{MIMEType: "image/jpeg", Frame: chatFrameGIF},
	}}.mediaNotes(0)
	if len(both) != 1 || !strings.HasPrefix(both[0], "Attachment 2 of 2 is only one still preview frame of a GIF.") {
		t.Fatalf("second attachment note = %q, want it to name attachment 2 of 2", both)
	}
}

// TestChatMediaRecognitionAllProviders drives photos, stickers, text files and PDFs end-to-end
// through the real Anthropic, Google and Featherless conversation adapters.
func TestChatMediaRecognitionAllProviders(t *testing.T) {
	jpegBytes := raidAITestJPEG("chat-photo")
	webpBytes := raidAITestWebP("chat-sticker")
	pdfBytes := []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\n")
	codeBytes := []byte("func main() {\n\tprintln(\"tempe\")\n}\n")
	oggBytes := chatTestOgg("e2e")
	mp3Bytes := append([]byte("ID3\x04\x00\x00"), bytes.Repeat([]byte("mp3."), 24)...)

	servedFiles := map[string][]byte{
		"files/p800":     jpegBytes,
		"files/frame1":   jpegBytes,
		"files/stk1":     webpBytes,
		"files/code1":    codeBytes,
		"files/docpdf":   pdfBytes,
		"files/bin1":     {0x00, 0x01, 0x02, 0x03},
		"files/voice1":   oggBytes,
		"files/music1":   mp3Bytes,
		"files/notaudio": jpegBytes,
		"files/blanktxt": []byte("   \n\t\x01  \r\n"),
	}
	fileServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := strings.Index(r.URL.Path, "/files/")
		if idx < 0 {
			http.NotFound(w, r)
			return
		}
		key := r.URL.Path[idx+1:]
		body, ok := servedFiles[key]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(fileServer.Close)

	standIn := &chatProviderStandIn{}
	providerServer := httptest.NewServer(http.HandlerFunc(standIn.handler))
	t.Cleanup(providerServer.Close)

	installMediaEnv := func(t *testing.T, provider string) *chatEnv {
		t.Helper()
		env := newChatEnv(t)
		env.bot.BotClient = &chatFileFake{chatTelegramFake: env.fake, base: fileServer.URL}
		chatSwap(t, &chatProviderOf, chatProviderFor)
		chatSwap(t, &chatAnthropicEndpoint, providerServer.URL)
		chatSwap(t, &chatGeminiEndpoint, providerServer.URL)
		chatSwap(t, &chatFeatherlessEndpoint, providerServer.URL)
		chatSwap(t, &chatProviderHTTP, providerServer.Client())
		cfg := config.AppConfig
		chatSwap(t, &cfg.ChatAnthropicModel, "claude-haiku-5-5")
		chatSwap(t, &cfg.ChatGeminiModel, "gemini-4-hydrogen")
		chatSwap(t, &cfg.ChatFeatherlessModel, "google/gemma-3-27b-it")
		if err := chatconv.SetProvider(context.Background(), env.chatID, provider, 1); err != nil {
			t.Fatalf("SetProvider(%s): %v", provider, err)
		}
		return env
	}

	b64JPEG := base64.StdEncoding.EncodeToString(jpegBytes)
	b64WebP := base64.StdEncoding.EncodeToString(webpBytes)
	b64PDF := base64.StdEncoding.EncodeToString(pdfBytes)

	for _, tc := range []struct {
		provider   string
		wantReply  string
		wantImgTag string
		expectPDF  bool
	}{
		{chatconv.ProviderAnthropic, "Claude unit on duty.", `"type":"image"`, true},
		{chatconv.ProviderGoogle, "Gemini unit on duty.", `"inlineData"`, true},
		{chatconv.ProviderFeatherless, "Featherless unit on duty.", `"image_url"`, false},
	} {
		t.Run(tc.provider+" photo, sticker, text file and PDF", func(t *testing.T) {
			env := installMediaEnv(t, tc.provider)
			before := len(standIn.bodies)

			// 1. Photo with @mention in caption.
			handle := "@" + env.bot.Username
			photoMsg := env.message(env.member("Alice"), "")
			photoMsg.Caption = handle + " what is in this photo?"
			photoMsg.CaptionEntities = []gotgbot.MessageEntity{{Type: "mention", Offset: 0, Length: int64(len(handle))}}
			photoMsg.Photo = photoSizes()
			env.deliver(photoMsg)

			// 2. Bare static sticker replying to the bot's own conversation reply (no text at all!).
			stickerMsg := env.message(env.member("Bob"), "")
			stickerMsg.Sticker = &gotgbot.Sticker{FileId: "stk1", FileUniqueId: "ustk1", Type: "regular", Emoji: "🤖"}
			stickerMsg.ReplyToMessage = &gotgbot.Message{MessageId: 9001, Date: 1, Chat: stickerMsg.Chat, From: &env.bot.User, Text: tc.wantReply}
			env.deliver(stickerMsg)

			// 3. Bare /ask replying to someone else's code file.
			fileOrigin := env.message(env.member("Carol"), "")
			fileOrigin.Document = &gotgbot.Document{FileId: "code1", FileName: "main.go", MimeType: "text/plain"}
			askFile := env.message(env.member("Dave"), "/ask")
			askFile.Entities = []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: 4}}
			askFile.ReplyToMessage = fileOrigin
			env.deliver(askFile)

			// 4. /ask with text replying to a PDF document.
			pdfOrigin := env.message(env.member("Eve"), "")
			pdfOrigin.Document = &gotgbot.Document{FileId: "docpdf", FileName: "spec.pdf", MimeType: "application/pdf"}
			askPDF := env.message(env.member("Frank"), "/ask summarize")
			askPDF.Entities = []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: 4}}
			askPDF.ReplyToMessage = pdfOrigin
			env.deliver(askPDF)

			gotBodies := standIn.bodies[before:]
			if len(gotBodies) != 4 {
				t.Fatalf("%s bodies = %d, want 4", tc.provider, len(gotBodies))
			}
			if sends := env.sends(); len(sends) != 4 {
				t.Fatalf("%s sends = %d (%q), want 4", tc.provider, len(sends), sends)
			}

			// Photo check
			if !strings.Contains(gotBodies[0], tc.wantImgTag) || !strings.Contains(gotBodies[0], b64JPEG) {
				t.Errorf("%s photo request missing image block or JPEG bytes", tc.provider)
			}
			// Sticker check
			if !strings.Contains(gotBodies[1], tc.wantImgTag) || !strings.Contains(gotBodies[1], b64WebP) {
				t.Errorf("%s sticker request missing image block or WebP bytes", tc.provider)
			}
			// Code file check (untrusted framing with per-line "| " prefix and closing marker on all providers)
			if !strings.Contains(gotBodies[2], "main.go") ||
				!strings.Contains(gotBodies[2], `| \tprintln(\"tempe\")`) ||
				!strings.Contains(gotBodies[2], "[End of attached file]") {
				t.Errorf("%s text file request missing framed file content: %s", tc.provider, gotBodies[2])
			}
			// PDF check (native on Anthropic & Google, skipped safely on Featherless)
			hasPDF := strings.Contains(gotBodies[3], b64PDF)
			if hasPDF != tc.expectPDF {
				t.Errorf("%s PDF bytes included = %v, want %v", tc.provider, hasPDF, tc.expectPDF)
			}

			// History in Redis keeps only short labels, never raw base64 or file content.
			historyRaw, err := env.mr.List(chatHistoryKey(env.chatID))
			if err != nil {
				t.Fatalf("read history: %v", err)
			}
			joinedHistory := strings.Join(historyRaw, "\n")
			if strings.Contains(joinedHistory, b64JPEG) || strings.Contains(joinedHistory, b64WebP) || strings.Contains(joinedHistory, `println("tempe")`) {
				t.Error("conversation history in Redis stored raw attachment bytes or file content")
			}
			if !strings.Contains(joinedHistory, "[sticker 🤖]") {
				t.Errorf("history missing sticker label: %s", joinedHistory)
			}
		})
	}

	for _, provider := range []string{chatconv.ProviderAnthropic, chatconv.ProviderGoogle, chatconv.ProviderFeatherless} {
		t.Run(provider+" GIF, video and round video message are seen as one preview frame", func(t *testing.T) {
			env := installMediaEnv(t, provider)
			before := len(standIn.bodies)
			frame := &gotgbot.PhotoSize{FileId: "frame1", FileUniqueId: "uframe1", Width: 90, Height: 90}

			// 1. A GIF with the bot mentioned in its caption.
			handle := "@" + env.bot.Username
			gifMsg := env.message(env.member("Alice"), "")
			gifMsg.Caption = handle + " what is happening here?"
			gifMsg.CaptionEntities = []gotgbot.MessageEntity{{Type: "mention", Offset: 0, Length: int64(len(handle))}}
			gifMsg.Animation = &gotgbot.Animation{FileId: "gif1", Thumbnail: frame}
			env.deliver(gifMsg)

			// 2. A bare round video message replying to the bot's own earlier reply: nothing but the picture.
			noteMsg := env.message(env.member("Bob"), "")
			noteMsg.VideoNote = &gotgbot.VideoNote{FileId: "note1", Thumbnail: frame}
			noteMsg.ReplyToMessage = &gotgbot.Message{MessageId: 9001, Date: 1, Chat: noteMsg.Chat, From: &env.bot.User, Text: "earlier reply"}
			env.deliver(noteMsg)

			// 3. /ask replying to someone else's video.
			videoOrigin := env.message(env.member("Carol"), "")
			videoOrigin.Video = &gotgbot.Video{FileId: "vid1", Thumbnail: frame}
			askVideo := env.message(env.member("Dave"), "/ask what is this?")
			askVideo.Entities = []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: 4}}
			askVideo.ReplyToMessage = videoOrigin
			env.deliver(askVideo)

			gotBodies := standIn.bodies[before:]
			if len(gotBodies) != 3 {
				t.Fatalf("%s bodies = %d, want 3", provider, len(gotBodies))
			}
			imageTag := map[string]string{
				chatconv.ProviderAnthropic:   `"type":"image"`,
				chatconv.ProviderGoogle:      `"inlineData"`,
				chatconv.ProviderFeatherless: `"image_url"`,
			}[provider]
			b64 := base64.StdEncoding.EncodeToString(jpegBytes)
			for i, want := range []string{"a GIF", "a round video message", "a video"} {
				body := gotBodies[i]
				if !strings.Contains(body, imageTag) || !strings.Contains(body, b64) {
					t.Errorf("%s request %d has no image block carrying the preview frame", provider, i+1)
				}
				if !strings.Contains(body, "is only one still preview frame of "+want) {
					t.Errorf("%s request %d does not say the image is a preview frame of %s", provider, i+1, want)
				}
			}
			if !strings.Contains(gotBodies[1], "Bob: [video note]") {
				t.Errorf("%s bare round video request lacks its [video note] label: %s", provider, gotBodies[1])
			}
			historyRaw, err := env.mr.List(chatHistoryKey(env.chatID))
			if err != nil {
				t.Fatalf("read history: %v", err)
			}
			if strings.Contains(strings.Join(historyRaw, "\n"), b64) {
				t.Error("conversation history in Redis stored the preview frame's bytes")
			}
		})
	}

	// A voice message, an audio file, and what each provider is told when it cannot hear them.
	for _, provider := range []string{chatconv.ProviderAnthropic, chatconv.ProviderGoogle, chatconv.ProviderFeatherless} {
		t.Run(provider+" voice messages and audio files", func(t *testing.T) {
			env := installMediaEnv(t, provider)
			before := len(standIn.bodies)
			b64Ogg := base64.StdEncoding.EncodeToString(oggBytes)
			b64MP3 := base64.StdEncoding.EncodeToString(mp3Bytes)

			// 1. A voice message with the bot mentioned in its caption.
			handle := "@" + env.bot.Username
			voiceMsg := env.message(env.member("Alice"), "")
			voiceMsg.Caption = handle + " what did I just say?"
			voiceMsg.CaptionEntities = []gotgbot.MessageEntity{{Type: "mention", Offset: 0, Length: int64(len(handle))}}
			voiceMsg.Voice = &gotgbot.Voice{FileId: "voice1", Duration: 7, FileSize: int64(len(oggBytes))}
			env.deliver(voiceMsg)

			// 2. A bare voice message replying to the bot's own reply: nothing but the clip.
			bareVoice := env.message(env.member("Bob"), "")
			bareVoice.Voice = &gotgbot.Voice{FileId: "voice1", Duration: 7}
			bareVoice.ReplyToMessage = &gotgbot.Message{MessageId: 9001, Date: 1, Chat: bareVoice.Chat, From: &env.bot.User, Text: "earlier reply"}
			env.deliver(bareVoice)

			// 3. /ask replying to someone else's audio file.
			musicOrigin := env.message(env.member("Carol"), "")
			musicOrigin.Audio = &gotgbot.Audio{FileId: "music1", Duration: 180, Title: "tempe song"}
			askMusic := env.message(env.member("Dave"), "/ask what is this song?")
			askMusic.Entities = []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: 4}}
			askMusic.ReplyToMessage = musicOrigin
			env.deliver(askMusic)

			// 4. A voice message that is too long to send, with the bot mentioned.
			longVoice := env.message(env.member("Eve"), "")
			longVoice.Caption = handle + " summarize this"
			longVoice.CaptionEntities = []gotgbot.MessageEntity{{Type: "mention", Offset: 0, Length: int64(len(handle))}}
			longVoice.Voice = &gotgbot.Voice{FileId: "voice1", Duration: chatMaxAudioSeconds + 1}
			env.deliver(longVoice)

			// 5. A "voice message" whose bytes are a picture: never sent as audio.
			fakeVoice := env.message(env.member("Mal"), "")
			fakeVoice.Caption = handle + " listen"
			fakeVoice.CaptionEntities = []gotgbot.MessageEntity{{Type: "mention", Offset: 0, Length: int64(len(handle))}}
			fakeVoice.Voice = &gotgbot.Voice{FileId: "notaudio", Duration: 3}
			env.deliver(fakeVoice)

			gotBodies := standIn.bodies[before:]
			if len(gotBodies) != 5 {
				t.Fatalf("%s bodies = %d, want 5", provider, len(gotBodies))
			}
			for i, body := range gotBodies {
				if provider != chatconv.ProviderGoogle && (strings.Contains(body, b64Ogg) || strings.Contains(body, b64MP3) || strings.Contains(body, "audio/ogg") || strings.Contains(body, "audio/mp3")) {
					t.Errorf("%s request %d carried audio to a provider that cannot hear it", provider, i+1)
				}
				if i >= 3 && (strings.Contains(body, b64Ogg) || strings.Contains(body, b64MP3)) {
					t.Errorf("%s request %d carried a clip it must not (too long, or not audio)", provider, i+1)
				}
			}
			if provider == chatconv.ProviderGoogle {
				for i, want := range []string{"a voice message", "a voice message", "an audio file"} {
					if !strings.Contains(gotBodies[i], `"mimeType":"audio/`) {
						t.Errorf("google request %d has no audio inlineData part: %s", i+1, gotBodies[i])
					}
					if !strings.Contains(gotBodies[i], "is "+want+", and you can hear it") {
						t.Errorf("google request %d does not say the clip is %s it can hear", i+1, want)
					}
				}
				if !strings.Contains(gotBodies[0], b64Ogg) || !strings.Contains(gotBodies[1], b64Ogg) || !strings.Contains(gotBodies[2], b64MP3) {
					t.Error("google requests 1 to 3 do not carry the clips' bytes")
				}
				if !strings.Contains(gotBodies[2], `"mimeType":"audio/mp3"`) {
					t.Errorf("the ID3 file was not sent as audio/mp3: %s", gotBodies[2])
				}
			}
			unheard := "not available to you, so you cannot hear it"
			if provider != chatconv.ProviderGoogle {
				for i := 0; i < 3; i++ {
					if !strings.Contains(gotBodies[i], unheard) {
						t.Errorf("%s request %d does not say the audio is not available", provider, i+1)
					}
				}
				if !strings.Contains(gotBodies[1], "Bob: [voice]") {
					t.Errorf("%s bare voice request lacks its [voice] label: %s", provider, gotBodies[1])
				}
			}
			// The too long clip and the picture dressed as a clip: never sent, and the model is told.
			for _, i := range []int{3, 4} {
				if !strings.Contains(gotBodies[i], unheard) {
					t.Errorf("%s request %d does not say the audio is not available", provider, i+1)
				}
			}
			if sends := env.sends(); len(sends) != len(gotBodies) {
				t.Errorf("%s sends = %d, want one reply per request (%d)", provider, len(sends), len(gotBodies))
			}
			historyRaw, err := env.mr.List(chatHistoryKey(env.chatID))
			if err != nil {
				t.Fatalf("read history: %v", err)
			}
			joined := strings.Join(historyRaw, "\n")
			if strings.Contains(joined, b64Ogg) || strings.Contains(joined, b64MP3) {
				t.Error("conversation history in Redis stored the audio's bytes")
			}
		})
	}

	t.Run("binary file disguised as .txt with no text fails safely without hallucinating", func(t *testing.T) {
		env := installMediaEnv(t, chatconv.ProviderFeatherless)
		before := len(standIn.bodies)
		binOrigin := env.message(env.member("Mal"), "")
		binOrigin.Document = &gotgbot.Document{FileId: "bin1", FileName: "fake.txt", MimeType: "text/plain"}
		askBin := env.message(env.member("Bob"), "/ask")
		askBin.Entities = []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: 4}}
		askBin.ReplyToMessage = binOrigin
		env.deliver(askBin)

		if len(standIn.bodies) != before {
			t.Errorf("provider was called for a rejected binary attachment with no text")
		}
		if sends := env.sends(); len(sends) != 1 || sends[0] != chatCanned(t, "chat_reply_malfunction") {
			t.Errorf("sends = %q, want malfunction notice", sends)
		}
	})

	t.Run("bare /ask on PDF with Featherless fails closed without calling provider", func(t *testing.T) {
		env := installMediaEnv(t, chatconv.ProviderFeatherless)
		before := len(standIn.bodies)
		pdfOrigin := env.message(env.member("Eve"), "")
		pdfOrigin.Document = &gotgbot.Document{FileId: "docpdf", FileName: "spec.pdf", MimeType: "application/pdf"}
		askPDF := env.message(env.member("Frank"), "/ask")
		askPDF.Entities = []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: 4}}
		askPDF.ReplyToMessage = pdfOrigin
		env.deliver(askPDF)

		if len(standIn.bodies) != before {
			t.Errorf("Featherless was called for bare PDF attachment with no text")
		}
		if sends := env.sends(); len(sends) != 1 || sends[0] != chatCanned(t, "chat_reply_malfunction") {
			t.Errorf("sends = %q, want malfunction notice", sends)
		}
	})

	t.Run("bare /ask on blank text file fails closed without calling provider", func(t *testing.T) {
		env := installMediaEnv(t, chatconv.ProviderFeatherless)
		before := len(standIn.bodies)
		blankOrigin := env.message(env.member("Eve"), "")
		blankOrigin.Document = &gotgbot.Document{FileId: "blanktxt", FileName: "empty.txt", MimeType: "text/plain"}
		askBlank := env.message(env.member("Frank"), "/ask")
		askBlank.Entities = []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: 4}}
		askBlank.ReplyToMessage = blankOrigin
		env.deliver(askBlank)

		if len(standIn.bodies) != before {
			t.Errorf("provider was called for blank text attachment with no text")
		}
		if sends := env.sends(); len(sends) != 1 || sends[0] != chatCanned(t, "chat_reply_malfunction") {
			t.Errorf("sends = %q, want malfunction notice", sends)
		}
	})

	t.Run("/ask @bot mention-only shows usage without media and uses media label with photo", func(t *testing.T) {
		env := installMediaEnv(t, chatconv.ProviderFeatherless)
		before := len(standIn.bodies)

		// 1. /ask @bot with no media shows usage and makes no provider call.
		askMentionOnly := env.message(env.member("Alice"), "/ask @"+env.bot.Username)
		askMentionOnly.Entities = []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: 4}}
		env.deliver(askMentionOnly)

		if len(standIn.bodies) != before {
			t.Fatalf("provider called for mention-only /ask without media")
		}
		if sends := env.sends(); len(sends) != 1 || sends[0] != chatCanned(t, "chat_ask_usage") {
			t.Fatalf("sends = %q, want chat_ask_usage", sends)
		}

		// 2. Photo with caption "/ask @bot" falls back to "[photo]" label instead of "(no text)".
		photoAsk := env.message(env.member("Bob"), "")
		photoAsk.Caption = "/ask @" + env.bot.Username
		photoAsk.CaptionEntities = []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: 4}}
		photoAsk.Photo = photoSizes()
		env.deliver(photoAsk)

		gotBodies := standIn.bodies[before:]
		if len(gotBodies) != 1 {
			t.Fatalf("provider calls = %d, want 1", len(gotBodies))
		}
		if !strings.Contains(gotBodies[0], "Bob: [photo]") {
			t.Errorf("provider request missing [photo] fallback label: %s", gotBodies[0])
		}
		if strings.Contains(gotBodies[0], "(no text)") {
			t.Errorf("provider request sent (no text) instead of [photo]: %s", gotBodies[0])
		}
		historyRaw, err := env.mr.List(chatHistoryKey(env.chatID))
		if err != nil {
			t.Fatalf("read history: %v", err)
		}
		if !strings.Contains(strings.Join(historyRaw, "\n"), "[photo]") {
			t.Errorf("history missing [photo] label: %v", historyRaw)
		}
	})
}

func TestChatDownloadBytesMidBodyCancelAndTimeout(t *testing.T) {
	started := make(chan struct{}, 2)
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		started <- struct{}{}
		<-r.Context().Done()
	}))
	t.Cleanup(slowServer.Close)

	bot := &gotgbot.Bot{
		Token: "999:secret-token-must-not-leak",
		BotClient: &chatFileFake{
			chatTelegramFake: &chatTelegramFake{},
			base:             slowServer.URL,
		},
	}

	t.Run("mid-body cancel reports cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			<-started
			cancel()
		}()
		_, class, err := downloadTelegramFileBytes(ctx, bot, "slow1", chatMaxTextFileBytes)
		if class != "cancelled" {
			t.Errorf("class = %q, want cancelled", class)
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, want context.Canceled", err)
		}
		if err != nil && (strings.Contains(err.Error(), bot.Token) || strings.Contains(err.Error(), "/file/bot")) {
			t.Errorf("error leaked bot token or URL: %v", err)
		}
	})

	t.Run("mid-body deadline reports timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		_, class, err := downloadTelegramFileBytes(ctx, bot, "slow2", chatMaxTextFileBytes)
		// Drain the started signal if the server reached it.
		select {
		case <-started:
		default:
		}
		if class != "timeout" {
			t.Errorf("class = %q, want timeout", class)
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("err = %v, want context.DeadlineExceeded", err)
		}
		if err != nil && (strings.Contains(err.Error(), bot.Token) || strings.Contains(err.Error(), "/file/bot")) {
			t.Errorf("error leaked bot token or URL: %v", err)
		}
	})
}
