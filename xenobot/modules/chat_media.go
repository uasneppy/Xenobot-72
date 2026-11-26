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
)

// chatMediaKind classifies a downloadable conversation attachment.
type chatMediaKind int

const (
	chatMediaImage chatMediaKind = iota + 1
	chatMediaPDF
	chatMediaText
)

// chatMediaRef identifies one attachment on an addressed or replied-to message.
type chatMediaRef struct {
	FileID   string
	Kind     chatMediaKind
	FileName string
}

// chatAttachment is one validated binary attachment ready for a provider call.
type chatAttachment struct {
	// MIMEType is image/jpeg, image/png, image/webp or application/pdf.
	MIMEType string
	Data     []byte
}

// chatDownloadedMedia is what the worker fetched for one conversation job.
type chatDownloadedMedia struct {
	Attachments []chatAttachment
	Files       []chatprompt.File
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

func chatExtractThumbRef(t *gotgbot.PhotoSize) (chatMediaRef, bool) {
	if t == nil || t.FileId == "" || t.FileSize > chatMaxImageBytes {
		return chatMediaRef{}, false
	}
	return chatMediaRef{FileID: t.FileId, Kind: chatMediaImage}, true
}

func chatExtractStickerRef(s *gotgbot.Sticker) (chatMediaRef, bool) {
	if s.IsAnimated || s.IsVideo {
		return chatExtractThumbRef(s.Thumbnail)
	}
	if s.FileId == "" || s.FileSize > chatMaxImageBytes {
		return chatMediaRef{}, false
	}
	return chatMediaRef{FileID: s.FileId, Kind: chatMediaImage}, true
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

// chatExtractMediaRef picks one downloadable photo, sticker, animation/video
// thumbnail, image file, PDF or text file from msg, without making any I/O.
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
	case msg.Animation != nil:
		// Checked before Document because Telegram also sets Document on GIFs.
		return chatExtractThumbRef(msg.Animation.Thumbnail)
	case msg.Video != nil:
		return chatExtractThumbRef(msg.Video.Thumbnail)
	case msg.Document != nil:
		return chatExtractDocumentRef(msg.Document)
	}
	return chatMediaRef{}, false
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
	case chatMediaPDF:
		return chatMaxPDFBytes
	case chatMediaText:
		return chatMaxTextFileBytes
	default:
		return chatMaxImageBytes
	}
}

// chatDownloadMedia fetches and validates each ref in job order, skipping kinds
// the target provider cannot accept natively (such as raw PDFs on Featherless).
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
				out.Attachments = append(out.Attachments, chatAttachment{MIMEType: mimeType, Data: data})
			default:
				log.WithFields(log.Fields{"chat_id": chatID, "user_id": userID}).Debug("[Chat] attachment download skipped: unsupported image bytes")
			}
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
