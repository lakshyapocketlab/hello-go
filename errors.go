package hello

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Sentinels matched by APIError through errors.Is. Prefer the Is* helpers.
var (
	ErrAuthentication = errors.New("hello: authentication failed")
	ErrNotFound       = errors.New("hello: not found")
	ErrRateLimit      = errors.New("hello: rate limited")
)

// APIError is an error response from the Hello API. It carries both error
// envelopes: Meta's {"error": {...}} used by the send and media routes (and the
// message_templates paths), and Hello's {"message", "code"}.
type APIError struct {
	// Status is the HTTP status code.
	Status int
	// Message is the human-readable error message.
	Message string
	// Code is Meta's numeric error code (error.code), 0 when absent.
	Code int
	// Subcode is Meta's error_subcode, 0 when absent.
	Subcode int
	// HelloCode is Hello's own code, e.g. "VALIDATION_ERROR" or "PLAN_DAILY_LIMIT_REACHED".
	// Branch on it for Hello-specific cases.
	HelloCode string
	// Type is Meta's error type, e.g. "OAuthException".
	Type string
	// FBTraceID is Meta's trace ID, or Hello's request ID. Quote it to support.
	FBTraceID string
	// Details is error_data.details.
	Details string
	// Data holds error_data (Meta envelope) or the whole body (Hello envelope),
	// including extras such as limit, used and resets_at.
	Data map[string]any
	// RequestID is the X-Request-ID response header.
	RequestID string
}

// Error implements error.
func (e *APIError) Error() string {
	code := e.HelloCode
	if code == "" && e.Code != 0 {
		code = fmt.Sprint(e.Code)
	}
	if code == "" {
		return fmt.Sprintf("hello: %d: %s", e.Status, e.Message)
	}
	return fmt.Sprintf("hello: %d %s: %s", e.Status, code, e.Message)
}

// Is lets errors.Is match the package sentinels.
func (e *APIError) Is(target error) bool {
	switch target {
	case ErrAuthentication:
		return e.Status == http.StatusUnauthorized || e.Code == 190
	case ErrNotFound:
		return e.Status == http.StatusNotFound
	case ErrRateLimit:
		return e.Status == http.StatusTooManyRequests
	}
	return false
}

// IsAuthentication reports whether err is an API error for a missing, wrong or
// disabled API key (401, or Meta code 190).
func IsAuthentication(err error) bool { return errors.Is(err, ErrAuthentication) }

// IsNotFound reports whether err is an API 404.
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

// IsRateLimit reports whether err is an API 429.
func IsRateLimit(err error) bool { return errors.Is(err, ErrRateLimit) }

// newAPIError reads and closes resp.Body and parses either error envelope.
func newAPIError(resp *http.Response) *APIError {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	_ = resp.Body.Close()
	e := &APIError{Status: resp.StatusCode, RequestID: resp.Header.Get("X-Request-ID")}

	var raw map[string]any
	if json.Unmarshal(body, &raw) == nil {
		if meta, ok := raw["error"].(map[string]any); ok {
			e.Message = str(meta, "message")
			e.Type = str(meta, "type")
			e.FBTraceID = str(meta, "fbtrace_id")
			e.setCode(meta["code"])
			e.Subcode = intVal(meta["error_subcode"])
			if data, ok := meta["error_data"].(map[string]any); ok {
				e.Data = data
				e.Details = str(data, "details")
				e.HelloCode = str(data, "hello_code")
			}
		} else {
			e.Message = str(raw, "message")
			e.Data = raw
			e.setCode(raw["code"])
		}
	} else {
		e.Message = strings.TrimSpace(string(body))
	}
	if e.Message == "" {
		e.Message = http.StatusText(resp.StatusCode)
	}
	return e
}

// setCode stores a Meta numeric code or a Hello string code.
func (e *APIError) setCode(v any) {
	switch c := v.(type) {
	case float64:
		e.Code = int(c)
	case string:
		e.HelloCode = c
	}
}

func str(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

func intVal(v any) int {
	f, _ := v.(float64)
	return int(f)
}
