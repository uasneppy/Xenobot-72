package featherless

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	maxMessageRunes = 300
	maxTokenRunes   = 64
	maxModelRunes   = 128
	maxParsedDelay  = 24 * time.Hour
	redacted        = "[REDACTED]"
)

// HTTPError is a non-2xx answer from the Featherless AI API. Its Error text
// holds the status, the status word and the error type only; the server
// message is kept in Message, truncated and with the key replaced, and is
// never part of Error.
type HTTPError struct {
	// Status is the HTTP status code.
	Status int
	// Type is the error envelope's type (or code when type is absent), for
	// example rate_limit_error or invalid_request_error.
	Type string
	// Code is the error envelope's machine-readable code, if any.
	Code string
	// Message is the server's message, truncated to 300 runes with every
	// occurrence of the key replaced by [REDACTED].
	Message string
	// RetryDelay is the wait from retry-after, else zero.
	RetryDelay time.Duration
}

// Error prints the status, its name and the error type, never the server
// message.
func (e *HTTPError) Error() string {
	text := "featherless: HTTP " + strconv.Itoa(e.Status)
	if word := http.StatusText(e.Status); word != "" {
		text += " " + word
	}
	if e.Type != "" {
		text += " " + e.Type
	}
	return text
}

// Retryable reports whether one more attempt is worth making: a 408, 429 or
// any 5xx (including 503 insufficient model capacity), except when the key or
// account itself was rejected (for example a 429 insufficient_quota).
func (e *HTTPError) Retryable() bool {
	if e.KeyRejected() {
		return false
	}
	return e.Status == http.StatusRequestTimeout || e.Status == http.StatusTooManyRequests || e.Status >= 500
}

// KeyRejected reports whether the key or its account is not accepted: a 401,
// 402 or 403, or an authentication, billing or permission error type or code.
func (e *HTTPError) KeyRejected() bool {
	switch e.Status {
	case http.StatusUnauthorized, http.StatusPaymentRequired, http.StatusForbidden:
		return true
	}
	switch e.Type {
	case "authentication_error", "billing_error", "permission_error":
		return true
	}
	switch e.Code {
	case "unauthorized", "invalid_api_key", "insufficient_quota":
		return true
	}
	return false
}

// errorEnvelope parses both the OpenAI-compatible {"error":{...}} payload and
// a flat FastAPI/gateway {"detail":"..."} payload.
type errorEnvelope struct {
	Error *struct {
		Type    string          `json:"type"`
		Code    json.RawMessage `json:"code"`
		Message string          `json:"message"`
	} `json:"error"`
	Detail string `json:"detail"`
}

func newHTTPError(status int, header http.Header, body []byte, key string) *HTTPError {
	httpErr := &HTTPError{Status: status}
	var envelope errorEnvelope
	if json.Unmarshal(body, &envelope) == nil {
		if envelope.Error != nil {
			httpErr.Type = truncateRunes(scrub(strings.TrimSpace(envelope.Error.Type), key), maxTokenRunes)
			httpErr.Code = truncateRunes(scrub(rawCodeString(envelope.Error.Code), key), maxTokenRunes)
			if httpErr.Type == "" {
				httpErr.Type = httpErr.Code
			}
			httpErr.Message = truncateRunes(scrub(strings.TrimSpace(envelope.Error.Message), key), maxMessageRunes)
		} else if envelope.Detail != "" {
			httpErr.Message = truncateRunes(scrub(strings.TrimSpace(envelope.Detail), key), maxMessageRunes)
		}
	}
	httpErr.RetryDelay = parseRetryAfter(header.Get("retry-after"))
	return httpErr
}

// rawCodeString decodes an OpenAI error.code field, which may be a JSON string,
// a number or null.
func rawCodeString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s)
	}
	var n int64
	if json.Unmarshal(raw, &n) == nil {
		return strconv.FormatInt(n, 10)
	}
	return ""
}

func scrub(s, key string) string {
	if key == "" {
		return s
	}
	return strings.ReplaceAll(s, key, redacted)
}

// parseRetryAfter reads the retry-after header in either seconds or HTTP-date
// form; anything unreadable is no delay.
func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		if seconds <= 0 {
			return 0
		}
		if seconds >= int64(maxParsedDelay/time.Second) {
			return maxParsedDelay
		}
		return time.Duration(seconds) * time.Second
	} else if errors.Is(err, strconv.ErrRange) && seconds > 0 {
		return maxParsedDelay
	}
	if at, err := http.ParseTime(value); err == nil {
		if wait := time.Until(at); wait > 0 {
			if wait > maxParsedDelay {
				return maxParsedDelay
			}
			return wait
		}
	}
	return 0
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	n := 0
	for i := range s {
		if n == max {
			return s[:i]
		}
		n++
	}
	return s
}
