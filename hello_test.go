package hello

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// recorded is one request seen by the test server.
type recorded struct {
	Method, Path, RawPath, Query string
	Header                       http.Header
	Body                         []byte
}

type fakeAPI struct {
	srv  *httptest.Server
	mu   sync.Mutex
	reqs []recorded
}

func (f *fakeAPI) last() recorded {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reqs[len(f.reqs)-1]
}

func (f *fakeAPI) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.reqs)
}

// newFake starts a server that records requests and delegates to h.
func newFake(t *testing.T, h http.HandlerFunc) (*fakeAPI, *Client) {
	t.Helper()
	f := &fakeAPI{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.reqs = append(f.reqs, recorded{r.Method, r.URL.Path, r.URL.EscapedPath(), r.URL.RawQuery, r.Header.Clone(), b})
		f.mu.Unlock()
		h(w, r)
	}))
	t.Cleanup(f.srv.Close)
	c, err := NewClient(WithAPIKey("hello_live_test"), WithBaseURL(f.srv.URL+"/"))
	if err != nil {
		t.Fatal(err)
	}
	c.sleep = func(context.Context, time.Duration) error { return nil }
	return f, c
}

func jsonReply(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

func decodeBody(t *testing.T, r recorded) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(r.Body, &m); err != nil {
		t.Fatalf("body %q: %v", r.Body, err)
	}
	return m
}

func expect(t *testing.T, r recorded, method, path string) {
	t.Helper()
	if r.Method != method || r.RawPath != path {
		t.Fatalf("got %s %s, want %s %s", r.Method, r.RawPath, method, path)
	}
	if got := r.Header.Get("Authorization"); got != "Bearer hello_live_test" {
		t.Fatalf("Authorization = %q", got)
	}
	if got := r.Header.Get("User-Agent"); got != "hello-go/"+Version {
		t.Fatalf("User-Agent = %q", got)
	}
}

func TestNewClientOptions(t *testing.T) {
	t.Setenv(APIKeyEnvVar, "")
	if _, err := NewClient(); err != ErrMissingAPIKey {
		t.Fatalf("err = %v, want ErrMissingAPIKey", err)
	}
	if _, err := NewClient(WithAPIKey("  ")); err != ErrMissingAPIKey {
		t.Fatalf("whitespace key: err = %v", err)
	}
	for _, bad := range []string{"", "ftp://x", "http://", "://bad", "no-scheme.com"} {
		if _, err := NewClient(WithAPIKey("k"), WithBaseURL(bad)); err == nil || !strings.Contains(err.Error(), "invalid base URL") {
			t.Fatalf("base %q: err = %v", bad, err)
		}
	}

	c, err := NewClient(WithAPIKey(" k "))
	if err != nil {
		t.Fatal(err)
	}
	if c.apiKey != "k" || c.baseURL != DefaultBaseURL || c.maxRetries != 2 || c.httpClient.Timeout != 60*time.Second {
		t.Fatalf("defaults wrong: %+v", c)
	}

	t.Setenv(APIKeyEnvVar, "hello_live_env")
	c, err = NewClient()
	if err != nil || c.apiKey != "hello_live_env" {
		t.Fatalf("env fallback: %v %v", c, err)
	}
	c, _ = NewClient(WithAPIKey("explicit"))
	if c.apiKey != "explicit" {
		t.Fatal("explicit key must win over env")
	}

	t.Setenv(BaseURLEnvVar, "https://staging.example.com/")
	c, err = NewClient(WithAPIKey("k"))
	if err != nil || c.baseURL != "https://staging.example.com" {
		t.Fatalf("env base URL: %v %v", c, err)
	}
	c, err = NewClient(WithAPIKey("k"), WithBaseURL("https://other.example.com"))
	if err != nil || c.baseURL != "https://other.example.com" {
		t.Fatalf("WithBaseURL must win over env: %v %v", c, err)
	}
	t.Setenv(BaseURLEnvVar, "ftp://bad")
	if _, err = NewClient(WithAPIKey("k")); err == nil || !strings.Contains(err.Error(), "invalid base URL") {
		t.Fatalf("invalid env base URL: err = %v", err)
	}
	if c, err = NewClient(WithAPIKey("k"), WithBaseURL("https://explicit.example.com")); err != nil || c.baseURL != "https://explicit.example.com" {
		t.Fatalf("explicit WithBaseURL must ignore an invalid env value: %v %v", c, err)
	}
	t.Setenv(BaseURLEnvVar, "")

	custom := &http.Client{Timeout: 5 * time.Second}
	c, _ = NewClient(WithHTTPClient(custom))
	if c.httpClient != custom {
		t.Fatal("custom client without WithTimeout must be used as is")
	}
	c, _ = NewClient(WithHTTPClient(custom), WithTimeout(time.Second), WithMaxRetries(-3))
	if c.httpClient == custom || c.httpClient.Timeout != time.Second || custom.Timeout != 5*time.Second || c.maxRetries != 0 {
		t.Fatal("WithTimeout must apply to a copy")
	}
	c, _ = NewClient(WithTimeout(0), WithMaxRetries(5), WithBaseURL("https://example.test/"))
	if c.httpClient.Timeout != 0 || c.maxRetries != 5 || c.baseURL != "https://example.test" {
		t.Fatalf("options not applied: %+v", c)
	}
}

func TestPing(t *testing.T) {
	f, c := newFake(t, jsonReply(200, `{"status":"ok","test_mode":false}`))
	got, err := c.Ping(context.Background())
	if err != nil || got.Status != "ok" || got.TestMode {
		t.Fatalf("%+v %v", got, err)
	}
	r := f.last()
	expect(t, r, "GET", "/v1/ping")
	if r.Header.Get("Accept") != "application/json" {
		t.Fatal("missing Accept")
	}

	_, c = newFake(t, jsonReply(401, `{"message":"bad key","code":"UNAUTHORIZED"}`))
	if _, err := c.Ping(context.Background()); !IsAuthentication(err) {
		t.Fatalf("err = %v", err)
	}
}

func TestPathSegment(t *testing.T) {
	if _, err := pathSegment("x", " "); err == nil || err.Error() != "hello: x is required" {
		t.Fatalf("err = %v", err)
	}
	if got, _ := pathSegment("x", "a/b c"); got != "a%2Fb%20c" {
		t.Fatalf("got %q", got)
	}
}
