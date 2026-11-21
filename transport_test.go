package hello

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetryGetOn5xxThenSuccess(t *testing.T) {
	var n atomic.Int32
	f, c := newFake(t, func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) < 3 {
			jsonReply(503, `{"message":"down"}`)(w, r)
			return
		}
		jsonReply(200, `{"status":"ok"}`)(w, r)
	})
	var sleeps []time.Duration
	c.sleep = func(_ context.Context, d time.Duration) error { sleeps = append(sleeps, d); return nil }
	c.rand = func() float64 { return 0.5 }
	if _, err := c.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.count() != 3 {
		t.Fatalf("attempts = %d", f.count())
	}
	// attempt 0: base 500ms -> 250ms + 0.5*250ms = 375ms; attempt 1: 1s -> 750ms.
	if len(sleeps) != 2 || sleeps[0] != 375*time.Millisecond || sleeps[1] != 750*time.Millisecond {
		t.Fatalf("sleeps = %v", sleeps)
	}
}

func TestRetryExhaustedReturnsLastError(t *testing.T) {
	f, c := newFake(t, jsonReply(500, `{"message":"boom"}`))
	_, err := c.Ping(context.Background())
	var ae *APIError
	if !errors.As(err, &ae) || ae.Status != 500 {
		t.Fatalf("err = %v", err)
	}
	if f.count() != 3 { // 1 + default 2 retries
		t.Fatalf("attempts = %d", f.count())
	}
	c.maxRetries = 0
	before := f.count()
	_, _ = c.Ping(context.Background())
	if f.count()-before != 1 {
		t.Fatal("maxRetries 0 must not retry")
	}
}

func TestNoRetryOnClientErrorsOrNonRetryable(t *testing.T) {
	f, c := newFake(t, jsonReply(400, `{"message":"bad"}`))
	_, _ = c.Ping(context.Background())
	if f.count() != 1 {
		t.Fatalf("400 retried: %d", f.count())
	}
	f, c = newFake(t, jsonReply(500, `{"message":"bad"}`))
	_, err := c.Messages.SendText(context.Background(), "1", "+1", "hi") // POST, no key
	if err == nil || f.count() != 1 {
		t.Fatalf("send without key retried: %d", f.count())
	}
}

func TestRetryAfterHeader(t *testing.T) {
	var n atomic.Int32
	_, c := newFake(t, func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) == 1 {
			w.Header().Set("Retry-After", "7")
			jsonReply(429, `{"message":"slow down","code":"RATE_LIMITED"}`)(w, r)
			return
		}
		jsonReply(200, `{"status":"ok"}`)(w, r)
	})
	var sleeps []time.Duration
	c.sleep = func(_ context.Context, d time.Duration) error { sleeps = append(sleeps, d); return nil }
	if _, err := c.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sleeps) != 1 || sleeps[0] != 7*time.Second {
		t.Fatalf("sleeps = %v", sleeps)
	}

	// 429 that keeps failing surfaces as a rate-limit error.
	_, c = newFake(t, jsonReply(429, `{"message":"slow"}`))
	c.maxRetries = 0
	if _, err := c.Ping(context.Background()); !IsRateLimit(err) {
		t.Fatalf("err = %v", err)
	}
}

func TestParseRetryAfter(t *testing.T) {
	if parseRetryAfter("") != 0 || parseRetryAfter("0") != 0 || parseRetryAfter("junk") != 0 || parseRetryAfter("-3") != 0 {
		t.Fatal("expected 0")
	}
	if parseRetryAfter(" 12 ") != 12*time.Second {
		t.Fatal("seconds")
	}
	future := time.Now().Add(30 * time.Second).UTC().Format(http.TimeFormat)
	if d := parseRetryAfter(future); d <= 20*time.Second || d > 30*time.Second {
		t.Fatalf("http date: %v", d)
	}
	past := time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)
	if parseRetryAfter(past) != 0 {
		t.Fatal("past date")
	}
	c := &Client{rand: func() float64 { return 0 }}
	if c.backoff(0, time.Hour) != retryAfterMax {
		t.Fatal("Retry-After must be capped")
	}
	if c.backoff(10, 0) != backoffMax/2 {
		t.Fatalf("backoff cap: %v", c.backoff(10, 0))
	}
}

func TestRetryOnNetworkError(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) == 1 {
			hj, _ := w.(http.Hijacker)
			conn, _, _ := hj.Hijack()
			conn.Close() // abort mid-request
			return
		}
		jsonReply(200, `{"status":"ok"}`)(w, r)
	}))
	defer srv.Close()
	c, _ := NewClient(WithAPIKey("k"), WithBaseURL(srv.URL), WithHTTPClient(&http.Client{Transport: &http.Transport{DisableKeepAlives: true}}))
	c.sleep = func(context.Context, time.Duration) error { return nil }
	if _, err := c.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n.Load() != 2 {
		t.Fatalf("attempts = %d", n.Load())
	}

	// A permanently failing network on a non-retryable call returns the wrapped error.
	srv.Close()
	_, err := c.Messages.SendText(context.Background(), "1", "+1", "x")
	if err == nil || IsNotFound(err) {
		t.Fatalf("err = %v", err)
	}
}

func TestContextCancelAndSleepError(t *testing.T) {
	_, c := newFake(t, jsonReply(200, `{}`))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Ping(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}

	_, c = newFake(t, jsonReply(503, `{}`))
	c.sleep = func(context.Context, time.Duration) error { return context.DeadlineExceeded }
	if _, err := c.Ping(context.Background()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v", err)
	}
}

func TestSleepContext(t *testing.T) {
	if err := sleepContext(context.Background(), time.Millisecond); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sleepContext(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}

func TestSendInternalErrors(t *testing.T) {
	_, c := newFake(t, jsonReply(200, `{}`))
	var nilCtx context.Context //nolint
	if _, err := c.send(nilCtx, request{method: "GET", path: "/x"}); err == nil {
		t.Fatal("nil context must fail")
	}
	if _, err := c.send(context.Background(), request{method: "POST", path: "/x", body: make(chan int)}); err == nil {
		t.Fatal("unmarshalable body must fail")
	}
	// Invalid JSON in a 2xx response.
	_, c = newFake(t, jsonReply(200, `not json`))
	if _, err := c.Ping(context.Background()); err == nil {
		t.Fatal("bad json must fail")
	}
	// Truncated body: Content-Length larger than what is sent.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(100))
		_, _ = io.WriteString(w, "short")
	}))
	defer srv.Close()
	c2, _ := NewClient(WithAPIKey("k"), WithBaseURL(srv.URL))
	if _, err := c2.Ping(context.Background()); err == nil {
		t.Fatal("truncated body must fail")
	}
}
