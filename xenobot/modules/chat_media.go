package modules

import (
	"bytes"
	"context"
	"net/http"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/db/chatconv"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/chatprompt"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/claude"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/gemini"
)

// Image, sticker and file recognition for Conversations (Phase 20 + Featherless).
// Extraction (chatExtractMediaRef) is pure and makes no network call; downloading
// (chatDownloadMedia) runs inside the conversation worker after cooldown, breaker
// and budget checks, derives from the job context so StopChat cancels it, follows
// no redirect, sniffs bytes instead of trusting names or headers, and never logs
// a token, URL, file name or file body.

const (
	chatMaxImageBytes    = claude.MaxImageBytes
	chatMaxPDFBytes      = claude.MaxImageBytes
	chatMaxTextFileBytes = 64 << 10
	chatMaxAudioBytes    = gemini.MaxAudioBytes
	// chatMaxAudioSeconds is the longest voice message or audio file sent to a provider. Audio is
	// billed by its length (Gemini counts 32 tokens a second), so this bounds one clip at about
	// 9,600 tokens. A longer clip is not sent and the model is told it cannot hear it. Provisional
	// owner decision P-AUD-02 (AGENTS.md).
	chatMaxAudioSeconds = 300
)

// chatMediaKind classifies a downloadable conversation attachment.
type chatMediaKind int

const (
	chatMediaImage chatMediaKind = iota + 1
	chatMediaPDF
	chatMediaText
	chatMediaAudio
)

// chatAudioKind says what an audio attachment is, for the note that tells the model it can hear it.
// The zero value is not audio.
type chatAudioKind int

const (
	chatAudioNone chatAudioKind = iota
	chatAudioVoice
	chatAudioFile
)

// noun is the code-owned phrase the audio note names the clip with.
func (k chatAudioKind) noun() string {
	switch k {
	case chatAudioVoice:
		return "a voice message"
	case chatAudioFile:
		return "an audio file"
	default:
		return ""
	}
}

// chatFrameKind says what a still image was taken from. The bot sends a provider one preview
// frame (Telegram's own thumbnail) of a GIF, a video, a round video message or an animated sticker,
// never the moving picture, so the prompt says so and the model never describes motion it cannot
// see. The zero value is a real image: a photo, a static sticker or an image file.
type chatFrameKind int

const (
	chatFrameNone chatFrameKind = iota
	chatFrameGIF
	chatFrameVideo
	chatFrameVideoNote
	chatFrameSticker
)

// noun is the code-owned phrase the still-frame note names the moving picture with.
func (k chatFrameKind) noun() string {
	switch k {
	case chatFrameGIF:
		return "a GIF"
	case chatFrameVideo:
		return "a video"
	case chatFrameVideoNote:
		return "a round video message"
	case chatFrameSticker:
		return "an animated sticker"
	default:
		return ""
	}
}

// chatMediaRef identifies one attachment on an addressed or replied-to message.
type chatMediaRef struct {
	FileID   string
	Kind     chatMediaKind
	FileName string
	// Frame is set when the image is only a preview frame of a moving picture.
	Frame chatFrameKind
	// Audio is set for a voice message or an audio file (Kind chatMediaAudio).
	Audio chatAudioKind
}

// chatAttachment is one validated binary attachment ready for a provider call.
type chatAttachment struct {
	// MIMEType is image/jpeg, image/png, image/webp, application/pdf or an audio type.
	MIMEType string
	Data     []byte
	// Frame is carried from the ref so the prompt can say the image is a preview frame.
	Frame chatFrameKind
	// Audio is carried from the ref so the prompt can say the clip is a voice message or a file.
	Audio chatAudioKind
}

// isAudio reports whether the attachment is an audio clip. Only Google receives one: the other
// adapters drop it before the request is built.
func (a chatAttachment) isAudio() bool { return strings.HasPrefix(a.MIMEType, "audio/") }

// chatDownloadedMedia is what the worker fetched for one conversation job.
type chatDownloadedMedia struct {
	Attachments []chatAttachment
	Files       []chatprompt.File
}

// heardAudio is how many audio clips the provider receives.
func (m chatDownloadedMedia) heardAudio() int {
	n := 0
	for _, a := range m.Attachments {
		if a.isAudio() {
			n++
		}
	}
	return n
}

// mediaNotes returns the bot's own notes about what the provider receives, in the order of the
// attachments: one still-frame note for each preview frame and one audio note for each clip, each
// naming its place among the attachments. audioSeen is how many voice messages or audio files the
// turn carries; when the provider got fewer (it cannot hear audio, the clip was too long or its
// download failed) one more note says the audio is not available, so the model never answers from
// the bare [voice] label. Empty when every attachment is a real image, a PDF or there are none and
// no audio was missed, so those prompts stay as they were.
func (m chatDownloadedMedia) mediaNotes(audioSeen int) []string {
	var notes []string
	for i, a := range m.Attachments {
		switch {
		case a.isAudio():
			notes = append(notes, chatprompt.AudioNote(i+1, len(m.Attachments), a.Audio.noun()))
		case a.Frame != chatFrameNone:
			notes = append(notes, chatprompt.FrameNote(i+1, len(m.Attachments), a.Frame.noun()))
		}
	}
	if audioSeen > m.heardAudio() {
		notes = append(notes, chatprompt.UnheardAudioNote)
	}
	return notes
}

// noMediaNotes is mediaNotes for the request sent again without any attachment after the provider
// rejected them: the notes say the pictures and the audio are not available instead of describing
// attachments that are no longer there. audioSeen is as for mediaNotes.
func (m chatDownloadedMedia) noMediaNotes(audioSeen int) []string {
	var notes []string
	for _, a := range m.Attachments {
		if strings.HasPrefix(a.MIMEType, "image/") {
			notes = append(notes, chatprompt.UnseenPictureNote)
			break
		}
	}
	if max(audioSeen, m.heardAudio()) > 0 {
		notes = append(notes, chatprompt.UnheardAudioNote)
	}
	return notes
}

var chatTextFileExtensions = map[string]bool{
	".txt": true, ".md": true, ".markdown": true, ".json": true, ".jsonl": true,
	".yaml": true, ".yml": true, ".toml": true, ".csv": true, ".tsv": true,
	".xml": true, ".html": true, ".htm": true, ".css": true, ".js": true,
	".mjs": true, ".cjs": true, ".ts": true, ".tsx": true, ".jsx": true,
	".go": true, ".py": true, ".rs": true, ".rb": true, ".php": true,
	".java": true, ".kt": true, ".c": true, ".h": true, ".cpp": true,
	".hpp": true, ".cs": true, ".swift": true, ".sh": true, ".bash": true,
	".zsh": true, ".sql": true, ".log": true, ".ini": true, ".cfg": true,
	".conf": true, ".diff": true, ".patch": true, ".dockerfile": true,
}

var chatTextMIMETypes = map[string]bool{
	"application/json":         true,
	"application/xml":          true,
	"application/javascript":   true,
	"application/x-javascript": true,
	"application/ecmascript":   true,
	"application/yaml":         true,
	"application/x-yaml":       true,
	"application/toml":         true,
	"application/sql":          true,
	"application/x-sh":         true,
	"application/x-python":     true,
	"application/csv":          true,
}

func chatNormaliseMIME(raw string) string {
	mime, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(raw)), ";")
	return strings.TrimSpace(mime)
}

func chatDocExt(fileName string) string {
	return strings.ToLower(path.Ext(strings.TrimSpace(fileName)))
}

func chatIsImageDocument(mimeType, fileName string) bool {
	switch chatNormaliseMIME(mimeType) {
	case "image/jpeg", "image/jpg", "image/png", "image/webp":
		return true
	case "", "application/octet-stream":
		switch chatDocExt(fileName) {
		case ".jpg", ".jpeg", ".png", ".webp":
			return true
		}
	}
	return false
}

func chatIsPDFDocument(mimeType, fileName string) bool {
	switch chatNormaliseMIME(mimeType) {
	case "application/pdf":
		return true
	case "", "application/octet-stream":
		return chatDocExt(fileName) == ".pdf"
	}
	return false
}

func chatIsTextDocument(mimeType, fileName string) bool {
	mime := chatNormaliseMIME(mimeType)
	if strings.HasPrefix(mime, "text/") || chatTextMIMETypes[mime] {
		return true
	}
	if mime == "" || mime == "application/octet-stream" {
		return chatTextFileExtensions[chatDocExt(fileName)]
	}
	return false
}

var chatAudioFileExtensions = map[string]bool{
	".mp3": true, ".m4a": true, ".ogg": true, ".oga": true, ".opus": true,
	".wav": true, ".flac": true, ".aac": true, ".aiff": true, ".aif": true,
}

// chatIsAudioDocument reports whether a file sent as a plain document is an audio file.
func chatIsAudioDocument(mimeType, fileName string) bool {
	mime := chatNormaliseMIME(mimeType)
	if strings.HasPrefix(mime, "audio/") {
		return true
	}
	if mime == "" || mime == "application/octet-stream" {
		return chatAudioFileExtensions[chatDocExt(fileName)]
	}
	return false
}

// chatExtractAudioRef picks the clip of a voice message, an audio file or an audio document, with
// no I/O. A clip whose declared length is over chatMaxAudioSeconds, or whose declared size is over
// chatMaxAudioBytes, gives no ref: the model is then told the audio is not available. A document
// has no declared length, so only its size bounds it.
func chatExtractAudioRef(msg *gotgbot.Message) (chatMediaRef, bool) {
	if msg == nil {
		return chatMediaRef{}, false
	}
	switch {
	case msg.Voice != nil:
		v := msg.Voice
		if v.FileId == "" || v.FileSize > chatMaxAudioBytes || v.Duration > chatMaxAudioSeconds {
			return chatMediaRef{}, false
		}
		return chatMediaRef{FileID: v.FileId, Kind: chatMediaAudio, Audio: chatAudioVoice}, true
	case msg.Audio != nil:
		a := msg.Audio
		if a.FileId == "" || a.FileSize > chatMaxAudioBytes || a.Duration > chatMaxAudioSeconds {
			return chatMediaRef{}, false
		}
		return chatMediaRef{FileID: a.FileId, Kind: chatMediaAudio, Audio: chatAudioFile}, true
	case msg.Document != nil:
		d := msg.Document
		if d.FileId == "" || !chatIsAudioDocument(d.MimeType, d.FileName) || d.FileSize > chatMaxAudioBytes {
			return chatMediaRef{}, false
		}
		return chatMediaRef{FileID: d.FileId, Kind: chatMediaAudio, Audio: chatAudioFile}, true
	}
	return chatMediaRef{}, false
}

// chatMessageHasAudio reports whether msg carries a voice message, an audio file or an audio
// document, whether or not its clip can be sent. A caller that finds audio but no ref knows the
// audio is unavailable and must not judge it from its label.
func chatMessageHasAudio(msg *gotgbot.Message) bool {
	if msg == nil {
		return false
	}
	if msg.Voice != nil || msg.Audio != nil {
		return true
	}
	return msg.Document != nil && chatIsAudioDocument(msg.Document.MimeType, msg.Document.FileName)
}

// chatProviderSupportsAudio reports whether a provider is sent audio. Only Gemini takes it: the
// Anthropic Messages API has no audio input, and Featherless depends on the model and takes only
// WAV or MP3 where it takes any, which the bot cannot produce (no ffmpeg in the image).
func chatProviderSupportsAudio(provider string) bool {
	return provider == chatconv.ProviderGoogle
}

// chatSniffAudio names the audio container a clip starts with, spelled as Gemini lists its types,
// or "" for anything else. It reads the bytes and trusts neither the declared type nor the file name.
func chatSniffAudio(data []byte) string {
	for _, magic := range []struct{ prefix, mime string }{
		{"OggS", "audio/ogg"},
		{"fLaC", "audio/flac"},
		{"ID3", "audio/mp3"},
	} {
		if bytes.HasPrefix(data, []byte(magic.prefix)) {
			return magic.mime
		}
	}
	if len(data) >= 12 {
		form, brand := string(data[:4]), string(data[8:12])
		switch {
		case form == "RIFF" && brand == "WAVE":
			return "audio/wav"
		case form == "FORM" && (brand == "AIFF" || brand == "AIFC"):
			return "audio/aiff"
		case string(data[4:8]) == "ftyp" && (brand == "M4A " || brand == "M4B "):
			return "audio/m4a"
		}
	}
	return chatSniffAudioFrame(data)
}

// chatSniffAudioFrame names a bare MPEG audio or ADTS (AAC) stream by its first frame header: 11 set
// sync bits, then layer bits 00 for ADTS, or for MPEG audio a layer and a version that are not the
// reserved values.
func chatSniffAudioFrame(data []byte) string {
	if len(data) < 2 || data[0] != 0xFF || data[1]&0xE0 != 0xE0 {
		return ""
	}
	layer, version := (data[1]>>1)&3, (data[1]>>3)&3
	switch {
	case data[1]&0xF6 == 0xF0:
		return "audio/aac"
	case layer != 0 && version != 1:
		return "audio/mp3"
	}
	return ""
}

// chatExtractThumbRef is the preview frame of a moving picture, refused when Telegram sent none or
// it declares a size over the largest image the providers take.
func chatExtractThumbRef(t *gotgbot.PhotoSize, frame chatFrameKind) (chatMediaRef, bool) {
	if t == nil || t.FileId == "" || t.FileSize > chatMaxImageBytes {
		return chatMediaRef{}, false
	}
	return chatMediaRef{FileID: t.FileId, Kind: chatMediaImage, Frame: frame}, true
}

func chatExtractStickerRef(s *gotgbot.Sticker) (chatMediaRef, bool) {
	if s.IsAnimated || s.IsVideo {
		return chatExtractThumbRef(s.Thumbnail, chatFrameSticker)
	}
	if s.FileId == "" || s.FileSize > chatMaxImageBytes {
		return chatMediaRef{}, false
	}
	return chatMediaRef{FileID: s.FileId, Kind: chatMediaImage}, true
}

// chatMovingDocumentKind says whether a file is a GIF or a video that Telegram sent as a plain
// document (it then has no animation or video field, only a thumbnail).
func chatMovingDocumentKind(mimeType, fileName string) chatFrameKind {
	mime := chatNormaliseMIME(mimeType)
	switch {
	case mime == "image/gif":
		return chatFrameGIF
	case strings.HasPrefix(mime, "video/"):
		return chatFrameVideo
	case mime == "" || mime == "application/octet-stream":
		switch chatDocExt(fileName) {
		case ".gif":
			return chatFrameGIF
		case ".mp4", ".m4v", ".mov", ".webm", ".mkv", ".avi":
			return chatFrameVideo
		}
	}
	return chatFrameNone
}

// chatExtractFrameRef picks the preview frame of a GIF, video, round video message, or a GIF or
// video sent as a file, with no I/O. A video's cover (the frame its sender chose) beats the small
// thumbnail when Telegram sent one.
func chatExtractFrameRef(msg *gotgbot.Message) (chatMediaRef, bool) {
	if msg == nil {
		return chatMediaRef{}, false
	}
	switch {
	case msg.Animation != nil:
		// Checked before Document because Telegram also sets Document on GIFs.
		return chatExtractThumbRef(msg.Animation.Thumbnail, chatFrameGIF)
	case msg.Video != nil:
		if cover, ok := raidAIPhotoRef(msg.Video.Cover); ok {
			return chatMediaRef{FileID: cover.FileID, Kind: chatMediaImage, Frame: chatFrameVideo}, true
		}
		return chatExtractThumbRef(msg.Video.Thumbnail, chatFrameVideo)
	case msg.VideoNote != nil:
		return chatExtractThumbRef(msg.VideoNote.Thumbnail, chatFrameVideoNote)
	case msg.Document != nil:
		if frame := chatMovingDocumentKind(msg.Document.MimeType, msg.Document.FileName); frame != chatFrameNone {
			return chatExtractThumbRef(msg.Document.Thumbnail, frame)
		}
	}
	return chatMediaRef{}, false
}

func chatExtractDocumentRef(doc *gotgbot.Document) (chatMediaRef, bool) {
	if doc.FileId == "" {
		return chatMediaRef{}, false
	}
	name := chatprompt.Clean(doc.FileName, chatprompt.MaxFileNameRunes)
	switch {
	case chatIsImageDocument(doc.MimeType, doc.FileName) && doc.FileSize <= chatMaxImageBytes:
		return chatMediaRef{FileID: doc.FileId, Kind: chatMediaImage, FileName: name}, true
	case chatIsPDFDocument(doc.MimeType, doc.FileName) && doc.FileSize <= chatMaxPDFBytes:
		return chatMediaRef{FileID: doc.FileId, Kind: chatMediaPDF, FileName: name}, true
	case chatIsTextDocument(doc.MimeType, doc.FileName) && doc.FileSize <= chatMaxTextFileBytes:
		return chatMediaRef{FileID: doc.FileId, Kind: chatMediaText, FileName: name}, true
	default:
		return chatMediaRef{}, false
	}
}

// chatExtractMediaRef picks one downloadable photo, sticker, GIF/video/round-video preview frame,
// voice message or audio file, image file, PDF, text file, or the preview frame of a GIF or video
// file from msg, without making any I/O.
func chatExtractMediaRef(msg *gotgbot.Message) (chatMediaRef, bool) {
	if msg == nil {
		return chatMediaRef{}, false
	}
	switch {
	case len(msg.Photo) > 0:
		if ref, ok := raidAIPhotoRef(msg.Photo); ok {
			return chatMediaRef{FileID: ref.FileID, Kind: chatMediaImage}, true
		}
	case msg.Sticker != nil:
		return chatExtractStickerRef(msg.Sticker)
	case msg.Animation != nil, msg.Video != nil, msg.VideoNote != nil:
		return chatExtractFrameRef(msg)
	case msg.Voice != nil, msg.Audio != nil:
		return chatExtractAudioRef(msg)
	case msg.Document != nil:
		if ref, ok := chatExtractFrameRef(msg); ok {
			return ref, true
		}
		if ref, ok := chatExtractAudioRef(msg); ok {
			return ref, true
		}
		return chatExtractDocumentRef(msg.Document)
	}
	return chatMediaRef{}, false
}

// chatMessageHasPicture reports whether msg carries something the AI should be able to look at
// (a photo, sticker, GIF, video, round video or an image, GIF or video file), whether or not a
// frame of it can be downloaded. A caller that finds a picture but no ref knows the picture is
// unavailable and must not judge it from its label.
func chatMessageHasPicture(msg *gotgbot.Message) bool {
	if msg == nil {
		return false
	}
	if len(msg.Photo) > 0 || msg.Sticker != nil || msg.Animation != nil || msg.Video != nil || msg.VideoNote != nil {
		return true
	}
	if doc := msg.Document; doc != nil {
		return chatIsImageDocument(doc.MimeType, doc.FileName) ||
			chatMovingDocumentKind(doc.MimeType, doc.FileName) != chatFrameNone
	}
	return false
}

// chatMediaLabel returns a compact label for a message's media (for turns
// without text and for conversation history, which never stores raw bytes).
func chatMediaLabel(msg *gotgbot.Message) string {
	if msg == nil {
		return ""
	}
	switch {
	case msg.Sticker != nil:
		if emoji := chatprompt.Clean(msg.Sticker.Emoji, 8); emoji != "" {
			return "[sticker " + emoji + "]"
		}
		return "[sticker]"
	case msg.Animation != nil:
		return "[GIF]"
	case msg.Document != nil:
		if chatMovingDocumentKind(msg.Document.MimeType, msg.Document.FileName) == chatFrameGIF {
			return "[GIF]"
		}
		if name := chatprompt.Clean(msg.Document.FileName, chatprompt.MaxFileNameRunes); name != "" {
			return "[file: " + name + "]"
		}
		return "[file]"
	default:
		return chatLogMediaTag(msg)
	}
}

func chatProviderSupportsPDF(provider string) bool {
	return provider == chatconv.ProviderAnthropic || provider == chatconv.ProviderGoogle
}

func chatMediaMaxBytes(kind chatMediaKind) int64 {
	switch kind {
	case chatMediaAudio:
		return chatMaxAudioBytes
	case chatMediaPDF:
		return chatMaxPDFBytes
	case chatMediaText:
		return chatMaxTextFileBytes
	default:
		return chatMaxImageBytes
	}
}

// chatDownloadMedia fetches and validates each ref in job order, skipping kinds
// the target provider cannot accept natively (raw PDFs on Featherless, audio on everything but Google).
// Every error is logged at Debug level by class only, never naming a URL, bot
// token, file name or content.
func chatDownloadMedia(ctx context.Context, b *gotgbot.Bot, chatID, userID int64, provider string, refs []chatMediaRef) chatDownloadedMedia {
	var out chatDownloadedMedia
	for _, ref := range refs {
		if ctx.Err() != nil {
			return out
		}
		if ref.Kind == chatMediaPDF && !chatProviderSupportsPDF(provider) {
			continue
		}
		if ref.Kind == chatMediaAudio && !chatProviderSupportsAudio(provider) {
			continue
		}
		data, errClass := chatDownloadBytes(ctx, b, ref.FileID, chatMediaMaxBytes(ref.Kind))
		if errClass != "" {
			log.WithFields(log.Fields{"chat_id": chatID, "user_id": userID}).Debugf("[Chat] attachment download skipped: %s", errClass)
			continue
		}
		switch ref.Kind {
		case chatMediaImage:
			mimeType, _, _ := strings.Cut(http.DetectContentType(data), ";")
			switch mimeType {
			case "image/jpeg", "image/png", "image/webp":
				out.Attachments = append(out.Attachments, chatAttachment{MIMEType: mimeType, Data: data, Frame: ref.Frame})
			default:
				log.WithFields(log.Fields{"chat_id": chatID, "user_id": userID}).Debug("[Chat] attachment download skipped: unsupported image bytes")
			}
		case chatMediaAudio:
			mimeType := chatSniffAudio(data)
			if mimeType == "" {
				log.WithFields(log.Fields{"chat_id": chatID, "user_id": userID}).Debug("[Chat] attachment download skipped: unsupported audio bytes")
				continue
			}
			out.Attachments = append(out.Attachments, chatAttachment{MIMEType: mimeType, Data: data, Audio: ref.Audio})
		case chatMediaPDF:
			if !bytes.HasPrefix(data, []byte("%PDF-")) {
				log.WithFields(log.Fields{"chat_id": chatID, "user_id": userID}).Debug("[Chat] attachment download skipped: unsupported pdf bytes")
				continue
			}
			out.Attachments = append(out.Attachments, chatAttachment{MIMEType: "application/pdf", Data: data})
		case chatMediaText:
			if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
				log.WithFields(log.Fields{"chat_id": chatID, "user_id": userID}).Debug("[Chat] attachment download skipped: non-text bytes")
				continue
			}
			cleaned := chatprompt.CleanFileContent(string(data), chatprompt.MaxFileContentRunes)
			if cleaned == "" {
				log.WithFields(log.Fields{"chat_id": chatID, "user_id": userID}).Debug("[Chat] attachment download skipped: blank text file")
				continue
			}
			out.Files = append(out.Files, chatprompt.File{Name: ref.FileName, Content: cleaned})
		}
	}
	return out
}

// chatDownloadBytes fetches one Telegram file up to maxBytes using the shared
// no-redirect downloader and returns a safe error class (never carrying a URL
// or bot token).
func chatDownloadBytes(ctx context.Context, b *gotgbot.Bot, fileID string, maxBytes int64) ([]byte, string) {
	data, errClass, _ := downloadTelegramFileBytes(ctx, b, fileID, maxBytes)
	return data, errClass
}
