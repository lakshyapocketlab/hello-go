package hello

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestAPIErrorMetaEnvelope(t *testing.T) {
	body := `{"error":{"message":"bad number","type":"OAuthException","code":100,"error_subcode":33,
	"error_data":{"messaging_product":"whatsapp","details":"the details","hello_code":"VALIDATION_ERROR","limit":5},
	"fbtrace_id":"trace-1"}}`
	_, c := newFake(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", "req-9")
		jsonReply(400, body)(w, r)
	})
	_, err := c.Ping(context.Background())
	var ae *APIError
	if !errors.As(err, &ae) {
		t.Fatalf("not an APIError: %v", err)
	}
	if ae.Status != 400 || ae.Message != "bad number" || ae.Type != "OAuthException" || ae.Code != 100 ||
		ae.Subcode != 33 || ae.HelloCode != "VALIDATION_ERROR" || ae.FBTraceID != "trace-1" ||
		ae.Details != "the details" || ae.RequestID != "req-9" || ae.Data["limit"] != float64(5) {
		t.Fatalf("%+v", ae)
	}
	if got := ae.Error(); got != "hello: 400 VALIDATION_ERROR: bad number" {
		t.Fatalf("Error() = %q", got)
	}
}

func TestAPIErrorHelloEnvelope(t *testing.T) {
	_, c := newFake(t, jsonReply(422, `{"message":"rejected","code":"META_REJECTED","meta_error":{"x":1}}`))
	_, err := c.Ping(context.Background())
	var ae *APIError
	if !errors.As(err, &ae) || ae.Status != 422 || ae.Message != "rejected" || ae.HelloCode != "META_REJECTED" ||
		ae.Code != 0 || ae.Data["meta_error"] == nil {
		t.Fatalf("%+v", ae)
	}

	// A numeric top-level code is Meta's.
	_, c = newFake(t, jsonReply(400, `{"message":"m","code":131047}`))
	_, err = c.Ping(context.Background())
	if !errors.As(err, &ae) || ae.Code != 131047 || ae.HelloCode != "" {
		t.Fatalf("%+v", ae)
	}
	if got := ae.Error(); got != "hello: 400 131047: m" {
		t.Fatalf("Error() = %q", got)
	}
}

func TestAPIErrorNonJSONAndEmpty(t *testing.T) {
	_, c := newFake(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(400)
		fmt.Fprint(w, "  plain failure \n")
	})
	_, err := c.Ping(context.Background())
	var ae *APIError
	if !errors.As(err, &ae) || ae.Message != "plain failure" || ae.Error() != "hello: 400: plain failure" {
		t.Fatalf("%+v", ae)
	}

	_, c = newFake(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(404) })
	_, err = c.Ping(context.Background())
	if !errors.As(err, &ae) || ae.Message != "Not Found" {
		t.Fatalf("%+v", ae)
	}
}

func TestSentinels(t *testing.T) {
	cases := []struct {
		name                 string
		err                  error
		auth, notFound, rate bool
	}{
		{"401", &APIError{Status: 401}, true, false, false},
		{"meta 190", &APIError{Status: 400, Code: 190}, true, false, false},
		{"404", &APIError{Status: 404}, false, true, false},
		{"429", &APIError{Status: 429}, false, false, true},
		{"500", &APIError{Status: 500}, false, false, false},
		{"wrapped", fmt.Errorf("ctx: %w", &APIError{Status: 404}), false, true, false},
		{"other", errors.New("x"), false, false, false},
		{"nil", nil, false, false, false},
	}
	for _, tc := range cases {
		if IsAuthentication(tc.err) != tc.auth || IsNotFound(tc.err) != tc.notFound || IsRateLimit(tc.err) != tc.rate {
			t.Errorf("%s: wrong classification", tc.name)
		}
	}
	if !errors.Is(&APIError{Status: 404}, ErrNotFound) || errors.Is(&APIError{Status: 404}, errors.New("x")) {
		t.Fatal("errors.Is sentinel mismatch")
	}
	if !strings.Contains(ErrRateLimit.Error(), "rate") {
		t.Fatal("sentinel text")
	}
}
