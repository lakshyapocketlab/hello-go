package hello

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	backoffBase   = 500 * time.Millisecond
	backoffMax    = 8 * time.Second
	retryAfterMax = 60 * time.Second
)

// request describes one API call.
type request struct {
	method string
	path   string
	query  url.Values
	header http.Header
	body   any // JSON-encoded when non-nil
	// raw is a pre-encoded body (multipart) sent with rawType.
	raw     []byte
	rawType string
	// retry marks the call safe to repeat after a 429, 5xx or network error.
	retry bool
}

// do runs req and decodes a 2xx JSON response into out (ignored when nil).
func (c *Client) do(ctx context.Context, r request, out any) error {
	resp, err := c.send(ctx, r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("hello: reading response: %w", err)
	}
	if out == nil || len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("hello: decoding response: %w", err)
	}
	return nil
}

// send performs the request with retries and returns a 2xx response, whose
// body the caller must close. Other statuses become *APIError.
func (c *Client) send(ctx context.Context, r request) (*http.Response, error) {
	payload, contentType := r.raw, r.rawType
	if r.body != nil {
		b, err := json.Marshal(r.body)
		if err != nil {
			return nil, fmt.Errorf("hello: encoding request body: %w", err)
		}
		payload, contentType = b, "application/json"
	}
	target := c.baseURL + r.path
	if len(r.query) > 0 {
		target += "?" + r.query.Encode()
	}

	for attempt := 0; ; attempt++ {
		var body io.Reader
		if payload != nil {
			body = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, r.method, target, body)
		if err != nil {
			return nil, fmt.Errorf("hello: building request: %w", err)
		}
		for k, v := range r.header {
			req.Header[k] = v
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("User-Agent", "hello-go/"+Version)
		req.Header.Set("Accept", "application/json")
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}

		var (
			lastErr    error
			retryAfter time.Duration
		)
		resp, err := c.httpClient.Do(req)
		switch {
		case err != nil:
			if ctx.Err() != nil {
				return nil, fmt.Errorf("hello: %w", ctx.Err())
			}
			lastErr = fmt.Errorf("hello: request failed: %w", err)
		case resp.StatusCode >= 200 && resp.StatusCode < 300:
			return resp, nil
		default:
			retryAfter = parseRetryAfter(resp.Header.Get("Retry-After"))
			apiErr := newAPIError(resp)
			if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500 {
				return nil, apiErr
			}
			lastErr = apiErr
		}

		if !r.retry || attempt >= c.maxRetries {
			return nil, lastErr
		}
		if err := c.sleep(ctx, c.backoff(attempt, retryAfter)); err != nil {
			return nil, fmt.Errorf("hello: %w", err)
		}
	}
}

// backoff is exponential with jitter, or the server's Retry-After when given.
func (c *Client) backoff(attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		return min(retryAfter, retryAfterMax)
	}
	d := min(backoffBase<<uint(attempt), backoffMax)
	return d/2 + time.Duration(c.rand()*float64(d/2))
}

// parseRetryAfter reads delta-seconds or an HTTP date; 0 when absent or past.
func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		return max(time.Until(t), 0)
	}
	return 0
}

func sleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
