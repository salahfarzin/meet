// Package gooyai is a client for the Gooyai Partner API
// (https://docs.gooyai.ir/docs/intro), the AI phone receptionist that answers
// clinic calls and books appointments into its own providers/slots/bookings.
//
// It deliberately imports nothing from the meet service so other services can
// reuse it. Each Client is bound to one tenant (clinic) API key; keys must stay
// server-side and are never logged or echoed by this package.
package gooyai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Env selects the API path prefix. Sandbox and production share request and
// response shapes; only the prefix and the API key differ.
type Env string

const (
	EnvSandbox    Env = "sandbox"
	EnvProduction Env = "prod"
)

const (
	DefaultBaseURL = "https://api.gooyai.ir"
	defaultTimeout = 15 * time.Second
	defaultRetries = 2
	defaultBackoff = 300 * time.Millisecond
	// maxErrorBody caps how much of an error response is read, so a
	// misbehaving upstream can't make us buffer an arbitrarily large body.
	maxErrorBody = 64 << 10
)

// pathPrefix returns the partner API prefix for env. Anything other than
// EnvProduction falls back to the sandbox, so a typo can't hit live data.
func (e Env) pathPrefix() string {
	if e == EnvProduction {
		return "/api/v1/partners"
	}
	return "/api/sandbox/v1/partners"
}

// Client calls the Partner API on behalf of one tenant.
type Client struct {
	httpClient *http.Client
	baseURL    string
	env        Env
	apiKey     string
	retries    int
	backoff    time.Duration
}

// Option customizes a Client.
type Option func(*Client)

// WithBaseURL overrides DefaultBaseURL (tests, proxies).
func WithBaseURL(baseURL string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(baseURL, "/") }
}

// WithEnv selects sandbox (default) or production.
func WithEnv(env Env) Option {
	return func(c *Client) { c.env = env }
}

// WithHTTPClient replaces the default http.Client (default timeout 15s).
// Share one across Clients so tenants reuse the same connection pool.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithRetry sets how many times an idempotent (GET) request is retried on a
// 5xx or transport error, and the base delay between attempts (multiplied by
// the attempt number). Writes are never retried: Gooyai has no idempotency
// keys, so a retried POST could create a duplicate slot or booking.
func WithRetry(retries int, backoff time.Duration) Option {
	return func(c *Client) {
		c.retries = max(retries, 0)
		c.backoff = backoff
	}
}

// New returns a Client for the tenant identified by apiKey.
func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{Timeout: defaultTimeout},
		baseURL:    DefaultBaseURL,
		env:        EnvSandbox,
		apiKey:     apiKey,
		retries:    defaultRetries,
		backoff:    defaultBackoff,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// envelope is the success shape every endpoint returns:
// {"status":"success","message":"...","data":...}.
type envelope struct {
	Data json.RawMessage `json:"data"`
}

// get issues a GET and decodes the envelope's data into out.
func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	return c.do(ctx, http.MethodGet, path, query, nil, out)
}

// do sends one request, retrying GETs per WithRetry, and decodes the
// envelope's data into out (skipped when out is nil).
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	attempts := 1
	if method == http.MethodGet {
		attempts += c.retries
	}

	var err error
	for attempt := 1; attempt <= attempts; attempt++ {
		if attempt > 1 {
			if werr := sleep(ctx, c.backoff*time.Duration(attempt-1)); werr != nil {
				return werr
			}
		}
		err = c.once(ctx, method, path, query, body, out)
		if !retryable(err) {
			return err
		}
	}
	return err
}

func (c *Client) once(ctx context.Context, method, path string, query url.Values, body, out any) error {
	req, err := c.newRequest(ctx, method, path, query, body)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &transportError{err: err}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decodeError(resp)
	}
	if out == nil {
		return nil
	}

	var env envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return fmt.Errorf("gooyai: decode %s %s response: %w", method, path, err)
	}
	if len(env.Data) == 0 || string(env.Data) == jsonNull {
		return nil
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return fmt.Errorf("gooyai: decode %s %s data: %w", method, path, err)
	}
	return nil
}

func (c *Client) newRequest(ctx context.Context, method, path string, query url.Values, body any) (*http.Request, error) {
	u := c.baseURL + c.env.pathPrefix() + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("gooyai: encode %s %s body: %w", method, path, err)
		}
		reader = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return nil, fmt.Errorf("gooyai: build %s %s request: %w", method, path, err)
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
