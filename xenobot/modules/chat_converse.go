package modules

import (
	"context"
	"errors"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/config"
	"github.com/uasneppy/Xenobot-72/xenobot/db/chatconv"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/claude"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/featherless"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/gemini"
)

// The one place a provider call is made and measured. Every lane (conversation, brief, reaction,
// inspect, referendum, tie-breaker) goes through chatConverse, so the vision fallback, the
// duration and token log and the empty-answer rule exist once.

// chatCall names the lane and the chat of one provider call, for the log line only.
type chatCall struct {
	Lane   string
	ChatID int64
}

// chatEmptyAnswer reports whether err is a provider answer that finished normally with no visible
// text (reasoning that used the whole output budget, a blank completion). It is a capacity signal,
// not a provider fault: the caller shows its malfunction line, but nothing counts it against the
// provider's breaker.
func chatEmptyAnswer(err error) bool {
	return errors.Is(err, claude.ErrEmptyAnswer) || errors.Is(err, gemini.ErrEmptyAnswer) || errors.Is(err, featherless.ErrEmptyAnswer)
}

// chatModelName is the model a provider is configured with, for the log line.
func chatModelName(provider string) string {
	cfg := config.AppConfig
	if cfg == nil {
		return ""
	}
	switch provider {
	case chatconv.ProviderGoogle:
		return cfg.ChatGeminiModel
	case chatconv.ProviderFeatherless:
		return cfg.ChatFeatherlessModel
	default:
		return cfg.ChatAnthropicModel
	}
}

// chatConverse makes one provider call. When a Featherless model answers an image request with a
// 400 or 422, it retries once with the images dropped and marks the answer TextOnly: the model may
// simply not take images, but nothing is remembered, so a later request tries its image again and
// an unrelated 400 (context length, an unsupported parameter) never switches vision off for other
// chats. The first error is not returned, so it counts toward nothing; only the retry's result
// does. root is the pool's context: a shutdown skips the retry.
func chatConverse(ctx, root context.Context, call chatCall, provider string, req chatRequest) (chatAnswer, error) {
	start := time.Now()
	answer, err := chatProviderOf(provider).converse(ctx, req)
	if err != nil && root.Err() == nil && provider == chatconv.ProviderFeatherless &&
		len(req.Attachments) > 0 && chatIsFeatherlessVisionReject(err) {
		log.WithFields(log.Fields{"chat_id": call.ChatID, "lane": call.Lane}).
			Debugf("[Chat] featherless rejected the image request (%s), retrying without images", chatErrorClass(err))
		req.Attachments = nil
		answer, err = chatProviderOf(provider).converse(ctx, req)
		answer.TextOnly = err == nil
	}
	chatLogCall(call, provider, time.Since(start), answer, err)
	return answer, err
}

// chatLogCall writes the one line every provider call leaves: lane, provider, model, how long it
// took, the token counts (input, output, cached input, thinking), the outcome and, for an empty
// answer, its finish reason. IDs, numbers and an error class only; never a message, a name, a
// provider message or a key.
func chatLogCall(call chatCall, provider string, took time.Duration, answer chatAnswer, err error) {
	fields := log.Fields{
		"chat_id": call.ChatID, "lane": call.Lane, "provider": provider, "model": chatModelName(provider),
		"duration_ms": took.Milliseconds(),
	}
	if err != nil {
		fields["outcome"] = "error"
		fields["error"] = chatErrorClass(err)
		var gemEmpty *gemini.EmptyAnswerError
		var fEmpty *featherless.EmptyAnswerError
		switch {
		case errors.As(err, &gemEmpty):
			fields["outcome"] = "empty"
			fields["finish"] = gemEmpty.FinishReason
			fields["in"], fields["out"], fields["thoughts"] = gemEmpty.InputTokens, gemEmpty.OutputTokens, gemEmpty.ThoughtsTokens
		case errors.As(err, &fEmpty):
			fields["outcome"] = "empty"
			fields["finish"] = fEmpty.FinishReason
			fields["in"], fields["out"] = fEmpty.InputTokens, fEmpty.OutputTokens
		case chatEmptyAnswer(err):
			fields["outcome"] = "empty"
		}
		log.WithFields(fields).Debug("[Chat] provider call")
		return
	}
	switch answer.Outcome {
	case chatRefused:
		fields["outcome"] = "refused"
	case chatTruncated:
		fields["outcome"] = "truncated"
	default:
		fields["outcome"] = "answered"
	}
	fields["in"], fields["out"] = answer.InputTokens, answer.OutputTokens
	fields["cache_read"], fields["thoughts"] = answer.CacheReadTokens, answer.ThoughtTokens
	if answer.TextOnly {
		fields["text_only"] = true
	}
	log.WithFields(fields).Debug("[Chat] provider call")
}
