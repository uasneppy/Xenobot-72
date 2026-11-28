package modules

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/uasneppy/Xenobot-72/xenobot/config"
	"github.com/uasneppy/Xenobot-72/xenobot/db/chatconv"
	"github.com/uasneppy/Xenobot-72/xenobot/db/lang"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/chatprompt"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/claude"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/featherless"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/gemini"
)

// The request of one conversation turn and the one interface every provider sits behind, so the
// runtime never branches on the provider.

// chatMaxOutputTokens is the output cap of a conversation request on both providers. Thinking
// tokens count against it, which is why it is far above the visible reply.
const chatMaxOutputTokens = 2000

// chatOutcome is how a provider answer ended.
type chatOutcome int

const (
	// chatAnswered is a complete answer.
	chatAnswered chatOutcome = iota
	// chatRefused is the provider declining to answer.
	chatRefused
	// chatTruncated is an answer cut by the output cap that still has text.
	chatTruncated
)

// chatRequest is one turn as every provider takes it.
type chatRequest struct {
	Static  string
	Dynamic string
	User    string
	// MaxTokens is the output cap; 0 means chatMaxOutputTokens.
	MaxTokens int
	// Attachments holds validated inline images and PDFs for the user turn.
	Attachments []chatAttachment
}

// maxTokens is the request's output cap.
func (r chatRequest) maxTokens() int {
	if r.MaxTokens > 0 {
		return r.MaxTokens
	}
	return chatMaxOutputTokens
}

// chatAnswer is what the runtime reads from a provider.
type chatAnswer struct {
	Text         string
	Outcome      chatOutcome
	InputTokens  int
	OutputTokens int
	// CacheReadTokens is the part of InputTokens served from the provider's prompt cache.
	CacheReadTokens int
	// ThoughtTokens is the thinking the provider billed on top of OutputTokens (Gemini).
	ThoughtTokens int
	// TextOnly is set when the answer came from a retry that dropped the request's images.
	TextOnly bool
}

// chatProvider is one provider's conversation call.
type chatProvider interface {
	converse(ctx context.Context, req chatRequest) (chatAnswer, error)
}

var (
	// chatAnthropicEndpoint, chatGeminiEndpoint and chatFeatherlessEndpoint are the provider
	// hosts; variables only so a test can point them at an httptest server.
	chatAnthropicEndpoint   = claude.DefaultEndpoint
	chatGeminiEndpoint      = gemini.DefaultEndpoint
	chatFeatherlessEndpoint = featherless.DefaultEndpoint
	// chatProviderHTTP is the transport of every provider; nil is each client's own.
	chatProviderHTTP *http.Client
)

// chatProviderFor returns the adapter of a provider with this server's key and model.
func chatProviderFor(provider string) chatProvider {
	cfg := config.AppConfig
	switch provider {
	case chatconv.ProviderGoogle:
		return chatGeminiProvider{client: &gemini.Client{
			Endpoint: chatGeminiEndpoint, APIKey: cfg.GeminiAPIKey, Model: cfg.ChatGeminiModel, HTTP: chatProviderHTTP,
		}}
	case chatconv.ProviderFeatherless:
		return chatFeatherlessProvider{client: &featherless.Client{
			Endpoint: chatFeatherlessEndpoint, APIKey: cfg.FeatherlessAPIKey, Model: cfg.ChatFeatherlessModel, HTTP: chatProviderHTTP,
		}}
	default:
		return chatAnthropicProvider{client: &claude.Client{
			Endpoint: chatAnthropicEndpoint, APIKey: cfg.AnthropicAPIKey, Model: cfg.ChatAnthropicModel, HTTP: chatProviderHTTP,
		}}
	}
}

type chatAnthropicProvider struct{ client *claude.Client }

func (p chatAnthropicProvider) converse(ctx context.Context, req chatRequest) (chatAnswer, error) {
	var attachments []claude.ConverseAttachment
	for _, a := range req.Attachments {
		attachments = append(attachments, claude.ConverseAttachment{MIMEType: a.MIMEType, Data: a.Data})
	}
	reply, err := p.client.Converse(ctx, claude.ConverseRequest{
		Static: req.Static, Dynamic: req.Dynamic, User: req.User, MaxTokens: req.maxTokens(), Attachments: attachments,
	})
	if err != nil {
		return chatAnswer{}, err
	}
	answer := chatAnswerOf(reply.Text, reply.Outcome == claude.OutcomeRefused, reply.Outcome == claude.OutcomeTruncated,
		reply.InputTokens, reply.OutputTokens)
	answer.CacheReadTokens = reply.CacheReadTokens
	return answer, nil
}

type chatGeminiProvider struct{ client *gemini.Client }

func (p chatGeminiProvider) converse(ctx context.Context, req chatRequest) (chatAnswer, error) {
	var attachments []gemini.Attachment
	for _, a := range req.Attachments {
		attachments = append(attachments, gemini.Attachment{MIMEType: a.MIMEType, Data: a.Data})
	}
	reply, err := p.client.Generate(ctx, gemini.Request{
		Static: req.Static, Dynamic: req.Dynamic, User: req.User, MaxTokens: req.maxTokens(), Attachments: attachments,
	})
	if err != nil {
		return chatAnswer{}, err
	}
	answer := chatAnswerOf(reply.Text, reply.Outcome == gemini.OutcomeRefused, reply.Outcome == gemini.OutcomeTruncated,
		reply.InputTokens, reply.OutputTokens)
	answer.CacheReadTokens = reply.CacheReadTokens
	answer.ThoughtTokens = reply.ThoughtsTokens
	return answer, nil
}

type chatFeatherlessProvider struct{ client *featherless.Client }

func (p chatFeatherlessProvider) converse(ctx context.Context, req chatRequest) (chatAnswer, error) {
	var images []featherless.Image
	for _, a := range req.Attachments {
		if a.MIMEType == "image/jpeg" || a.MIMEType == "image/png" || a.MIMEType == "image/webp" {
			images = append(images, featherless.Image{MIMEType: a.MIMEType, Data: a.Data})
		}
	}
	reply, err := p.client.Converse(ctx, featherless.ConverseRequest{
		Static: req.Static, Dynamic: req.Dynamic, User: req.User, MaxTokens: req.maxTokens(), Images: images,
	})
	if err != nil {
		return chatAnswer{}, err
	}
	answer := chatAnswerOf(reply.Text, reply.Outcome == featherless.OutcomeRefused, reply.Outcome == featherless.OutcomeTruncated,
		reply.InputTokens, reply.OutputTokens)
	answer.CacheReadTokens = reply.CacheReadTokens
	return answer, nil
}

// chatAnswerOf builds the runtime's answer from a provider's reply.
func chatAnswerOf(text string, refused, truncated bool, input, output int) chatAnswer {
	answer := chatAnswer{Text: text, Outcome: chatAnswered, InputTokens: input, OutputTokens: output}
	switch {
	case refused:
		answer.Outcome = chatRefused
	case truncated:
		answer.Outcome = chatTruncated
	}
	return answer
}

// chatErrorClass names a provider error for a log line and the breaker: the HTTP status and the
// error type or status, never the message, which can quote the request.
func chatErrorClass(err error) string {
	var claudeErr *claude.HTTPError
	if errors.As(err, &claudeErr) {
		return "HTTP " + strconv.Itoa(claudeErr.Status) + " " + claudeErr.Type
	}
	var geminiErr *gemini.HTTPError
	if errors.As(err, &geminiErr) {
		return "HTTP " + strconv.Itoa(geminiErr.Status) + " " + geminiErr.GRPCStatus
	}
	var featherlessErr *featherless.HTTPError
	if errors.As(err, &featherlessErr) {
		return strings.TrimSpace("HTTP " + strconv.Itoa(featherlessErr.Status) + " " + featherlessErr.Type)
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "cancelled"
	case chatEmptyAnswer(err):
		return "empty answer"
	case errors.Is(err, claude.ErrNoReply), errors.Is(err, gemini.ErrNoReply), errors.Is(err, featherless.ErrNoReply):
		return "no reply"
	}
	return "transport error"
}

// chatProviderSpan says whether err calls for a pause of the provider for every group, and for
// how long: a rejected key or account, a 429 whose retry delay is over 2 s (at most 5 minutes) and
// Gemini's per-day quota.
func chatProviderSpan(err error) time.Duration {
	var claudeErr *claude.HTTPError
	if errors.As(err, &claudeErr) {
		if claudeErr.KeyRejected() {
			return memberReqAIKeyPause
		}
		if claudeErr.Status == http.StatusTooManyRequests && claudeErr.RetryDelay > memberReqAILongRateLimit {
			return min(claudeErr.RetryDelay, memberReqAIMaxPause)
		}
		return 0
	}
	var geminiErr *gemini.HTTPError
	if errors.As(err, &geminiErr) {
		if geminiErr.KeyRejected() {
			return memberReqAIKeyPause
		}
		if geminiErr.Status == http.StatusTooManyRequests {
			if geminiErr.DailyQuota {
				return memberReqAIMaxPause
			}
			if geminiErr.RetryDelay > memberReqAILongRateLimit {
				return min(geminiErr.RetryDelay, memberReqAIMaxPause)
			}
		}
		return 0
	}
	var featherlessErr *featherless.HTTPError
	if errors.As(err, &featherlessErr) {
		if featherlessErr.KeyRejected() {
			return memberReqAIKeyPause
		}
		if featherlessErr.Status == http.StatusTooManyRequests && featherlessErr.RetryDelay > memberReqAILongRateLimit {
			return min(featherlessErr.RetryDelay, memberReqAIMaxPause)
		}
	}
	return 0
}

// chatLanguageName is the group language's own name, for the dynamic system text.
func chatLanguageName(ctx context.Context, chatID int64) (code, name string) {
	code = lang.GetGroupLanguage(ctx, chatID)
	name, err := i18n.MustNewTranslator(code).GetString("language_name")
	if err != nil || strings.TrimSpace(name) == "" {
		name = code
	}
	return code, name
}

// chatBuildRequest assembles the three strings and any validated binary attachments of a turn.
// The static text is the same bytes for every chat; the dynamic text holds the group's title,
// language, today's date in the group's zone and the bot's username; the user text holds any
// ambient topic chatter, the transcript, the quoted message, the message to answer and any
// attached text files. No user ID, username or chat ID is in any of them.
func chatBuildRequest(ctx context.Context, job chatJob, botUsername string, ambient, history []chatprompt.Turn, media chatDownloadedMedia) chatRequest {
	_, language := chatLanguageName(ctx, job.chatID)
	date := time.Now().In(chatClock(ctx, job.chatID)).Format("2006-01-02")
	return chatRequest{
		Static: chatprompt.Static(),
		Dynamic: chatprompt.Dynamic(chatprompt.Chat{
			Title: job.chatTitle, Language: language, Date: date, BotUsername: botUsername,
		}),
		User:        chatprompt.UserWithMemory(job.memoryNotes, chatprompt.UserWithMedia(ambient, history, job.quoted, job.turn, media.Files, media.frameNotes())),
		Attachments: media.Attachments,
	}
}
