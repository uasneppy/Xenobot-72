package featherless

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ErrNoReply is wrapped by every conversation answer that carries no usable
// text: a finish reason other than stop, length or content_filter, a blank
// answer, an unclosed reasoning block, or an unreadable body.
var ErrNoReply = errors.New("featherless: no reply")

// ErrEmptyAnswer is wrapped (together with ErrNoReply) by an answer that
// finished normally (stop or length) or came back with no choices, but had no
// visible text: reasoning that ran out of tokens before the answer, or a blank
// completion. It is a capacity signal, not a provider fault, so callers must
// not count it as a failure.
var ErrEmptyAnswer = fmt.Errorf("%w: empty answer", ErrNoReply)

// EmptyAnswerError describes an ErrEmptyAnswer and keeps the finish reason and
// token counts so a caller can log why.
type EmptyAnswerError struct {
	// FinishReason is stop, length, or empty when the answer had no choices.
	FinishReason string
	InputTokens  int
	OutputTokens int
}

func (e *EmptyAnswerError) Error() string {
	if e.FinishReason == "" {
		return ErrEmptyAnswer.Error() + ": no choices"
	}
	return ErrEmptyAnswer.Error() + ": finish " + e.FinishReason
}

// Unwrap lets errors.Is match ErrEmptyAnswer and ErrNoReply.
func (e *EmptyAnswerError) Unwrap() error { return ErrEmptyAnswer }

// Outcome is how a conversation answer ended.
type Outcome int

const (
	// OutcomeAnswered is a complete answer.
	OutcomeAnswered Outcome = iota
	// OutcomeRefused is the model or filter declining to answer; Reply.Text is empty.
	OutcomeRefused
	// OutcomeTruncated is an answer cut by max_completion_tokens that still has text.
	OutcomeTruncated
)

// ConverseRequest is one conversation turn. Static is the persona and archive
// and comes first in the system message so Featherless's automatic prefix
// cache can hit across groups; Dynamic follows it after a blank line; User is
// the user message; Images holds optional inline pictures for vision models.
type ConverseRequest struct {
	Static    string
	Dynamic   string
	User      string
	MaxTokens int
	Images    []Image
}

// Reply is what the bot reads from a conversation answer.
type Reply struct {
	Text            string
	Outcome         Outcome
	InputTokens     int
	OutputTokens    int
	CacheReadTokens int
	Model           string
}

type chatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type contentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type completionRequest struct {
	Model               string          `json:"model"`
	Messages            []chatMessage   `json:"messages"`
	MaxCompletionTokens int             `json:"max_completion_tokens"`
	ResponseFormat      *responseFormat `json:"response_format,omitempty"`
}

type completionResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message *struct {
			Content string `json:"content"`
			Refusal string `json:"refusal"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens        int `json:"prompt_tokens"`
		CompletionTokens    int `json:"completion_tokens"`
		CachedTokens        int `json:"cached_tokens"`
		PromptTokensDetails *struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
	} `json:"usage"`
}

func systemPrompt(static, dynamic string) string {
	if strings.TrimSpace(dynamic) == "" {
		return static
	}
	return static + "\n\n" + dynamic
}

// userContent builds a plain string when there are no images (for maximum
// compatibility with text-only open-weight templates) and an OpenAI
// multimodal array with text before images (per Featherless Vision docs) when
// images are present.
func userContent(text string, images []Image) any {
	if len(images) == 0 {
		return text
	}
	parts := make([]contentPart, 0, 1+len(images))
	parts = append(parts, contentPart{Type: "text", Text: text})
	for _, img := range images {
		parts = append(parts, contentPart{
			Type: "image_url",
			ImageURL: &imageURL{
				URL: "data:" + img.MIMEType + ";base64," + base64.StdEncoding.EncodeToString(img.Data),
			},
		})
	}
	return parts
}

func buildConverseBody(model string, in ConverseRequest) *completionRequest {
	return &completionRequest{
		Model:               model,
		MaxCompletionTokens: in.MaxTokens,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt(in.Static, in.Dynamic)},
			{Role: "user", Content: userContent(in.User, in.Images)},
		},
	}
}

// Converse sends one conversation turn and returns the reply. A refusal or
// content_filter finish is a Reply with OutcomeRefused and no error; every
// answer without usable text is an error wrapping ErrNoReply; an HTTP failure
// is an *HTTPError.
func (c *Client) Converse(ctx context.Context, in ConverseRequest) (*Reply, error) {
	if c.APIKey == "" {
		return nil, ErrNotConfigured
	}
	if !ValidModel(c.Model) {
		return nil, ErrInvalidModel
	}
	if in.MaxTokens <= 0 || strings.TrimSpace(in.Static) == "" || strings.TrimSpace(in.User) == "" {
		return nil, errors.New("featherless: incomplete conversation request")
	}
	for _, img := range in.Images {
		if !imageSupported(img) {
			return nil, ErrUnsupportedImage
		}
	}

	payload, err := json.Marshal(buildConverseBody(c.Model, in))
	if err != nil {
		return nil, errors.New("featherless: encode request")
	}

	ctx, cancel := context.WithTimeout(ctx, ConverseTimeout)
	defer cancel()

	body, err := c.exec(ctx, c.httpClient(conversationHTTP), c.completionsURL(), payload)
	if err != nil {
		return nil, err
	}
	return parseConverseReply(body)
}

// parseConverseReply reads one chat completion answer. A refusal field or
// content_filter finish is OutcomeRefused; stop is OutcomeAnswered; length is
// OutcomeTruncated; anything else, or blank text after stripping leading
// reasoning tags, wraps ErrNoReply.
func parseConverseReply(data []byte) (*Reply, error) {
	var raw completionResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("%w: unreadable answer", ErrNoReply)
	}

	reply := &Reply{Model: truncateRunes(raw.Model, maxModelRunes)}
	if u := raw.Usage; u != nil {
		reply.InputTokens = u.PromptTokens
		reply.OutputTokens = u.CompletionTokens
		if u.PromptTokensDetails != nil && u.PromptTokensDetails.CachedTokens > 0 {
			reply.CacheReadTokens = u.PromptTokensDetails.CachedTokens
		} else {
			reply.CacheReadTokens = u.CachedTokens
		}
	}

	if len(raw.Choices) == 0 {
		return nil, &EmptyAnswerError{InputTokens: reply.InputTokens, OutputTokens: reply.OutputTokens}
	}
	choice := raw.Choices[0]
	if choice.Message != nil && strings.TrimSpace(choice.Message.Refusal) != "" {
		reply.Outcome = OutcomeRefused
		return reply, nil
	}

	finish := strings.ToLower(strings.TrimSpace(choice.FinishReason))
	switch finish {
	case "content_filter":
		reply.Outcome = OutcomeRefused
		return reply, nil
	case "stop":
		reply.Outcome = OutcomeAnswered
	case "length":
		reply.Outcome = OutcomeTruncated
	default:
		return nil, fmt.Errorf("%w: finish %s", ErrNoReply, truncateRunes(finish, maxTokenRunes))
	}

	if choice.Message == nil {
		return nil, fmt.Errorf("%w: no message", ErrNoReply)
	}
	reply.Text = stripLeadingThink(choice.Message.Content)
	if reply.Text == "" {
		return nil, &EmptyAnswerError{
			FinishReason: finish,
			InputTokens:  reply.InputTokens,
			OutputTokens: reply.OutputTokens,
		}
	}
	return reply, nil
}

// stripLeadingThink removes the reasoning some open-weight models emit inside
// content when no server-side reasoning parser is active. It handles two
// shapes, matched case-insensitively: leading <think>...</think> blocks, and
// "reasoning...</think>answer" with no opening tag (chat templates that put
// <think> into the prompt produce this), where everything up to the last
// closing tag is dropped. An unclosed leading <think> block returns "" so raw
// reasoning never leaks to a group chat. Reasoning written as plain prose with
// no tag at all cannot be told apart from an answer here.
func stripLeadingThink(s string) string {
	s = strings.TrimSpace(s)
	for hasPrefixFold(s, "<think>") {
		end := indexFold(s, "</think>")
		if end < 0 {
			return ""
		}
		s = strings.TrimSpace(s[end+len("</think>"):])
	}
	if end := lastIndexFold(s, "</think>"); end >= 0 {
		s = strings.TrimSpace(s[end+len("</think>"):])
	}
	return s
}

func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}

// indexFold is strings.Index with ASCII case folding; sub must be ASCII.
func indexFold(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if strings.EqualFold(s[i:i+len(sub)], sub) {
			return i
		}
	}
	return -1
}

// lastIndexFold is strings.LastIndex with ASCII case folding; sub must be ASCII.
func lastIndexFold(s, sub string) int {
	for i := len(s) - len(sub); i >= 0; i-- {
		if strings.EqualFold(s[i:i+len(sub)], sub) {
			return i
		}
	}
	return -1
}
