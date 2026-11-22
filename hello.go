// Package hello is the official Go SDK for Hello, the WhatsApp Business API.
//
// It wraps the public /v1 Integration API: sending messages and templates,
// uploading media, managing message templates and webhook overrides. It uses
// only the standard library.
//
//	client, err := hello.NewClient() // reads HELLO_API_KEY
//	if err != nil { ... }
//	resp, err := client.Messages.SendText(ctx, phoneNumberID, "+16505551234", "Hi")
package hello

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Version is the SDK version, sent in the User-Agent header.
const Version = "0.1.1"

const (
	// DefaultBaseURL is the public Hello API host.
	DefaultBaseURL = "https://helloapi.lakshyapocketlab.com"
	// APIKeyEnvVar is the environment variable read when no API key option is given.
	APIKeyEnvVar = "HELLO_API_KEY"
	// BaseURLEnvVar is the environment variable that overrides the API host when
	// WithBaseURL is not given.
	BaseURLEnvVar = "HELLO_BASE_URL"

	defaultTimeout    = 60 * time.Second
	defaultMaxRetries = 2
)

// ErrMissingAPIKey is returned by NewClient when no API key was supplied
// through WithAPIKey or the HELLO_API_KEY environment variable.
var ErrMissingAPIKey = errors.New("hello: API key is required: use WithAPIKey or set " + APIKeyEnvVar)

// Client is a Hello API client. It is safe for concurrent use.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	maxRetries int

	// Test seams: how the client waits between retries and draws jitter.
	sleep func(ctx context.Context, d time.Duration) error
	rand  func() float64

	// Messages sends WhatsApp messages and reads their status.
	Messages *MessagesService
	// Media uploads and downloads WhatsApp media.
	Media *MediaService
	// PhoneNumbers lists the WhatsApp numbers connected to the organization.
	PhoneNumbers *PhoneNumbersService
	// Templates manages WhatsApp message templates.
	Templates *TemplatesService
	// WebhookOverride manages a number's direct (own-URL) webhook.
	WebhookOverride *WebhookOverrideService
}

type config struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	timeout    time.Duration
	timeoutSet bool
	maxRetries int
	err        error
}

// Option configures a Client.
type Option func(*config)

// WithAPIKey sets the API key (hello_live_...). Defaults to the HELLO_API_KEY
// environment variable.
func WithAPIKey(key string) Option {
	return func(c *config) { c.apiKey = key }
}

// WithBaseURL overrides the API host, for staging or self-hosting. It takes
// precedence over the HELLO_BASE_URL environment variable.
func WithBaseURL(baseURL string) Option {
	return func(c *config) {
		u, err := url.Parse(baseURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			c.err = fmt.Errorf("hello: invalid base URL %q", baseURL)
			return
		}
		c.baseURL = strings.TrimRight(baseURL, "/")
	}
}

// WithHTTPClient uses a custom *http.Client (proxies, tracing, custom
// transports). The client is copied before WithTimeout is applied to it.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *config) { c.httpClient = hc }
}

// WithTimeout sets the per-attempt timeout, which includes reading the
// response body. The default is 60 seconds; 0 disables it. When combined with
// WithHTTPClient it overrides that client's Timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *config) { c.timeout, c.timeoutSet = d, true }
}

// WithMaxRetries sets how many times a retryable request is retried after the
// first attempt. The default is 2; values below 0 mean 0.
func WithMaxRetries(n int) Option {
	return func(c *config) { c.maxRetries = max(n, 0) }
}

// NewClient builds a Client. It fails with ErrMissingAPIKey when no key is set.
func NewClient(opts ...Option) (*Client, error) {
	cfg := &config{baseURL: DefaultBaseURL, timeout: defaultTimeout, maxRetries: defaultMaxRetries}
	if env := strings.TrimSpace(os.Getenv(BaseURLEnvVar)); env != "" {
		WithBaseURL(env)(cfg)
	}
	for _, o := range opts {
		o(cfg)
	}
	if cfg.err != nil {
		return nil, cfg.err
	}
	if cfg.apiKey == "" {
		cfg.apiKey = os.Getenv(APIKeyEnvVar)
	}
	if strings.TrimSpace(cfg.apiKey) == "" {
		return nil, ErrMissingAPIKey
	}
	hc := cfg.httpClient
	switch {
	case hc == nil:
		hc = &http.Client{Timeout: cfg.timeout}
	case cfg.timeoutSet:
		cp := *hc
		cp.Timeout = cfg.timeout
		hc = &cp
	}
	c := &Client{
		apiKey:     strings.TrimSpace(cfg.apiKey),
		baseURL:    cfg.baseURL,
		httpClient: hc,
		maxRetries: cfg.maxRetries,
		sleep:      sleepContext,
		rand:       rand.Float64,
	}
	c.Messages = &MessagesService{c}
	c.Media = &MediaService{c}
	c.PhoneNumbers = &PhoneNumbersService{c}
	c.Templates = &TemplatesService{c}
	c.WebhookOverride = &WebhookOverrideService{c}
	return c, nil
}

// PingResponse is the result of Ping.
type PingResponse struct {
	Status   string `json:"status"`
	TestMode bool   `json:"test_mode"`
}

// Ping checks that the API key works (GET /v1/ping).
func (c *Client) Ping(ctx context.Context) (*PingResponse, error) {
	var out PingResponse
	if err := c.do(ctx, request{method: http.MethodGet, path: "/v1/ping", retry: true}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// pathSegment URL-escapes a path parameter and rejects empty ones.
func pathSegment(name, value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("hello: %s is required", name)
	}
	return url.PathEscape(value), nil
}
