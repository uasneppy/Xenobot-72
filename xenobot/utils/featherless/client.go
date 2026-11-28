// Package featherless is the Featherless AI OpenAI-compatible client for
// conversations, raid AI image checks and the AI spam-name check.
//
// It imports the standard library only, so a spike tool can use it without
// pulling in xenobot/config or xenobot/db, whose init functions connect to
// PostgreSQL before main. The API key travels only in the Authorization Bearer
// header, never in a URL, no redirect is followed, and nothing in this package
// logs: callers log, and every error returned here is free of the key and of
// the URL.
package featherless

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	// DefaultEndpoint is the Featherless AI API host.
	DefaultEndpoint = "https://api.featherless.ai"
	// Timeout bounds one whole Classify or ClassifyNames call, retry included.
	Timeout = 15 * time.Second
	// ConverseTimeout bounds one whole Converse call, retry included.
	ConverseTimeout = 40 * time.Second
	// MaxImageBytes is the largest image sent inline.
	MaxImageBytes = 5 << 20

	maxResponseBytes = 1 << 20
	maxModelBytes    = 128

	// authHeaderName is the HTTP header carrying the bearer credential.
	authHeaderName = "Authorization"
	bearerPrefix   = "Bearer "

	// retryWait is the wait before the one retry when the server names none.
	retryWait = 500 * time.Millisecond
	// maxRetryWait drops the retry rather than hold a worker on a long wait.
	maxRetryWait = 2 * time.Second
)

var (
	// ErrNotConfigured means the client has no API key.
	ErrNotConfigured = errors.New("featherless: no API key configured")
	// ErrInvalidModel means the model name is not a valid Hugging Face repo id.
	ErrInvalidModel = errors.New("featherless: invalid model name")
	// ErrUnsupportedImage means the image type or size cannot be sent.
	ErrUnsupportedImage = errors.New("featherless: unsupported image")
	// ErrUnsupportedNames means all three profile strings are empty.
	ErrUnsupportedNames = errors.New("featherless: no names to classify")
)

// modelPattern matches a Hugging Face repository id (owner/model) as used by
// Featherless AI's model catalog, rejecting bare keys, paths, queries and
// whitespace.
var modelPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}/[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// ValidModel reports whether name is a Featherless Hugging Face model id
// (owner/model, at most 128 ASCII bytes, no ".."). Requiring the slash also
// prevents a pasted API key from being accepted as a model id.
func ValidModel(name string) bool {
	if len(name) == 0 || len(name) > maxModelBytes || strings.Contains(name, "..") {
		return false
	}
	return modelPattern.MatchString(name)
}

// Image is one picture sent inline as a base64 data URL.
type Image struct {
	// MIMEType is image/jpeg, image/png or image/webp.
	MIMEType string
	Data     []byte
}

func imageSupported(img Image) bool {
	switch img.MIMEType {
	case "image/jpeg", "image/png", "image/webp":
	default:
		return false
	}
	return len(img.Data) > 0 && len(img.Data) <= MaxImageBytes
}

// Client calls the Featherless AI OpenAI-compatible chat completions API.
// It holds no state besides its fields and is safe for concurrent use.
type Client struct {
	// Endpoint is the API host, without a trailing /v1/chat/completions path
	// (a trailing /v1 is also trimmed). Empty means DefaultEndpoint; tests
	// point it at an httptest server.
	Endpoint string
	// APIKey is sent in the Authorization Bearer header only.
	APIKey string
	// Model is the Hugging Face model id, for example Qwen/Qwen3-32B.
	Model string
	// HTTP is the transport. Nil uses a client with Timeout (for Classify /
	// ClassifyNames) or ConverseTimeout (for Converse) that follows no
	// redirects.
	HTTP *http.Client
}

var defaultHTTP = &http.Client{
	Timeout: Timeout,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

var conversationHTTP = &http.Client{
	Timeout: ConverseTimeout,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func (c *Client) httpClient(fallback *http.Client) *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return fallback
}

func (c *Client) baseURL() string {
	base := c.Endpoint
	if base == "" {
		return DefaultEndpoint
	}
	base = strings.TrimRight(base, "/")
	base = strings.TrimSuffix(base, "/v1")
	return base
}

func (c *Client) completionsURL() string {
	return c.baseURL() + "/v1/chat/completions"
}

// exec makes the request and, for a retryable failure, one more attempt when
// the wait is at most maxRetryWait. The wait is the server's retry-after, else
// retryWait. Any other failure is final.
func (c *Client) exec(ctx context.Context, hc *http.Client, target string, payload []byte) ([]byte, error) {
	body, err := c.send(ctx, hc, target, payload)
	if err == nil {
		return body, nil
	}
	var tf *transportFailure
	if errors.As(err, &tf) && transportRetryable(ctx, err) {
		if !sleepContext(ctx, retryWait) {
			return nil, err
		}
		return c.send(ctx, hc, target, payload)
	}
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || !httpErr.Retryable() {
		return nil, err
	}
	wait := httpErr.RetryDelay
	if wait <= 0 {
		wait = retryWait
	}
	if wait > maxRetryWait || !sleepContext(ctx, wait) {
		return nil, err
	}
	return c.send(ctx, hc, target, payload)
}

func sleepContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// send makes one POST request and returns the body of a 2xx answer.
func (c *Client) send(ctx context.Context, hc *http.Client, target string, payload []byte) ([]byte, error) {
	// The request error is not wrapped: it would carry the URL.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		return nil, errors.New("featherless: build request")
	}
	req.Header.Set(authHeaderName, bearerPrefix+c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := hc.Do(req)
	if err != nil {
		return nil, c.transportError(err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes))
		_ = resp.Body.Close()
	}()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, c.transportError(err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, newHTTPError(resp.StatusCode, resp.Header, data, c.APIKey)
	}
	return data, nil
}

// transportError rebuilds a failed request's error without the URL that
// *url.Error carries.
func (c *Client) transportError(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) && ue.Err != nil {
		err = ue.Err
	}
	if c.APIKey != "" && strings.Contains(err.Error(), c.APIKey) {
		return &transportFailure{text: "featherless: request failed"}
	}
	return &transportFailure{text: "featherless: request failed: " + err.Error(), err: err}
}

// transportFailure is a request that failed before an HTTP answer was read:
// a reset or stale connection, a TLS error, a dropped body. Its text is what
// the client has always returned; the type lets exec retry it once.
type transportFailure struct {
	text string
	err  error
}

func (e *transportFailure) Error() string { return e.text }
func (e *transportFailure) Unwrap() error { return e.err }

// transportRetryable reports whether a transport failure is worth one more
// attempt: the caller's context is still live and the failure is not itself a
// cancellation or a deadline (a timed-out call is never repeated).
func transportRetryable(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return false
	}
	return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}
