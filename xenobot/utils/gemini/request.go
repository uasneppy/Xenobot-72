package gemini

import "encoding/base64"

// MaxAttachmentBytes is the largest image or PDF sent inline.
const MaxAttachmentBytes = 5 << 20

// MaxAudioBytes is the largest audio clip sent inline. The whole request may not pass 20 MB (the
// base64 form counts), and a clip's cost is its length, so the caller also caps its duration.
const MaxAudioBytes = 5 << 20

// Attachment is one image, PDF or audio clip sent inline in the user message.
type Attachment struct {
	// MIMEType is image/jpeg, image/png, image/webp, application/pdf or one of the audio types
	// AudioMIMETypes lists.
	MIMEType string
	Data     []byte
}

// AudioMIMETypes are the audio types the client sends, spelled as Gemini's audio guide lists
// them. A Telegram voice message is OGG with Opus (audio/ogg).
var AudioMIMETypes = []string{
	"audio/ogg", "audio/mp3", "audio/wav", "audio/aac", "audio/flac", "audio/aiff", "audio/m4a",
}

// IsAudio reports whether the attachment is one of the audio types.
func (a Attachment) IsAudio() bool {
	for _, t := range AudioMIMETypes {
		if a.MIMEType == t {
			return true
		}
	}
	return false
}

func attachmentSupported(a Attachment) bool {
	switch {
	case a.IsAudio():
		return len(a.Data) > 0 && len(a.Data) <= MaxAudioBytes
	case a.MIMEType == "image/jpeg", a.MIMEType == "image/png", a.MIMEType == "image/webp", a.MIMEType == "application/pdf":
		return len(a.Data) > 0 && len(a.Data) <= MaxAttachmentBytes
	}
	return false
}

// safetyThreshold is the threshold of the four classic categories. The source
// material is black comedy that mentions cannibalism, so a stricter threshold
// would refuse lore questions; the persona rules and the bot's output filter do
// the rest.
const safetyThreshold = "BLOCK_ONLY_HIGH"

var safetyCategories = []string{
	"HARM_CATEGORY_HARASSMENT",
	"HARM_CATEGORY_HATE_SPEECH",
	"HARM_CATEGORY_SEXUALLY_EXPLICIT",
	"HARM_CATEGORY_DANGEROUS_CONTENT",
}

// Request is one conversation turn. Static is the persona and archive and must
// be the same bytes for every chat, so the implicit cache can hit; Dynamic is
// the short per-chat block after it; User is the one user message; Attachments
// holds optional inline images, PDFs or audio clips for the user turn.
type Request struct {
	Static      string
	Dynamic     string
	User        string
	MaxTokens   int
	Attachments []Attachment
}

type systemPart struct {
	Text string `json:"text"`
}

type systemContent struct {
	Parts []systemPart `json:"parts"`
}

type inlineData struct {
	MIMEType string `json:"mimeType"`
	Data     string `json:"data"`
}

type part struct {
	Text       string      `json:"text,omitempty"`
	InlineData *inlineData `json:"inlineData,omitempty"`
}

type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

type thinkingConfig struct {
	ThinkingLevel string `json:"thinkingLevel"`
}

type generationConfig struct {
	MaxOutputTokens int            `json:"maxOutputTokens"`
	ThinkingConfig  thinkingConfig `json:"thinkingConfig"`
}

type safetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

// body is the generateContent request in camelCase. It carries no
// temperature, topP, topK, candidateCount or thinkingBudget: Gemini 4 ignores
// or rejects them.
type body struct {
	SystemInstruction systemContent    `json:"systemInstruction"`
	Contents          []content        `json:"contents"`
	GenerationConfig  generationConfig `json:"generationConfig"`
	SafetySettings    []safetySetting  `json:"safetySettings"`
}

// buildBody assembles the request: the static text first and the dynamic text
// second, as two parts of the system instruction in that order, so the prefix
// is the same for every chat.
func buildBody(in Request, opts Options) *body {
	level := opts.ThinkingLevel
	if level == "" {
		level = DefaultThinkingLevel
	}
	settings := make([]safetySetting, 0, len(safetyCategories))
	for _, category := range safetyCategories {
		settings = append(settings, safetySetting{Category: category, Threshold: safetyThreshold})
	}
	userParts := make([]part, 0, 1+len(in.Attachments))
	userParts = append(userParts, part{Text: in.User})
	for _, a := range in.Attachments {
		userParts = append(userParts, part{
			InlineData: &inlineData{
				MIMEType: a.MIMEType,
				Data:     base64.StdEncoding.EncodeToString(a.Data),
			},
		})
	}
	return &body{
		SystemInstruction: systemContent{Parts: []systemPart{{Text: in.Static}, {Text: in.Dynamic}}},
		Contents:          []content{{Role: "user", Parts: userParts}},
		GenerationConfig: generationConfig{
			MaxOutputTokens: in.MaxTokens,
			ThinkingConfig:  thinkingConfig{ThinkingLevel: level},
		},
		SafetySettings: settings,
	}
}
