package hello

import (
	"context"
	"errors"
	"net/http"
	"net/url"
)

// WebhookOverrideService manages a number's direct webhook: Meta delivers the
// number's incoming messages, statuses and pricing straight to your URL instead
// of Hello, whose dashboard then stops tracking them. See the Hello docs on
// direct webhooks before using Set or Reapply.
type WebhookOverrideService struct{ c *Client }

// WebhookOverride is a number's direct-webhook state. The verify token is never returned.
type WebhookOverride struct {
	Enabled bool   `json:"enabled"`
	URL     string `json:"url,omitempty"`
	// Status is active, error or draft (saved in Hello but not applied on Meta;
	// use Reapply). Empty when disabled.
	Status        string `json:"status,omitempty"`
	LastError     string `json:"last_error,omitempty"`
	SetAt         string `json:"set_at,omitempty"`
	LastCheckedAt string `json:"last_checked_at,omitempty"`
	PlanAllows    bool   `json:"plan_allows"`
	// MetaEnabled, MetaHost and InSync are set only by Get with refresh=true.
	MetaEnabled bool   `json:"meta_enabled,omitempty"`
	MetaHost    string `json:"meta_host,omitempty"`
	InSync      bool   `json:"in_sync,omitempty"`
}

// Get returns the override. With refresh true it first checks Meta's live
// setting and records the result (GET /v1/{phone_number_id}/webhook-override).
func (s *WebhookOverrideService) Get(ctx context.Context, phoneNumberID string, refresh bool) (*WebhookOverride, error) {
	id, err := pathSegment("phoneNumberID", phoneNumberID)
	if err != nil {
		return nil, err
	}
	r := request{method: http.MethodGet, path: "/v1/" + id + "/webhook-override", retry: true}
	if refresh {
		r.query = url.Values{"refresh": {"true"}}
	}
	var out WebhookOverride
	if err := s.c.do(ctx, r, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetWebhookOverrideParams is the body of Set.
type SetWebhookOverrideParams struct {
	// URL is your https endpoint (at most 200 characters, publicly reachable).
	// It must answer Meta's verification handshake and should carry a long
	// random secret in its path.
	URL string `json:"url"`
	// VerifyToken is 8-100 printable characters without spaces; your server checks it.
	VerifyToken string `json:"verify_token"`
	// AcknowledgeNoTracking must be true: Hello will stop receiving and showing
	// this number's incoming messages, statuses and usage details.
	AcknowledgeNoTracking bool `json:"acknowledge_no_tracking"`
}

// Set points the number's webhook at your URL, replacing any existing override
// (PUT /v1/{phone_number_id}/webhook-override). Requires a plan that includes
// the feature. Setting the same value again is harmless, so it is retried.
func (s *WebhookOverrideService) Set(ctx context.Context, phoneNumberID string, p SetWebhookOverrideParams) (*WebhookOverride, error) {
	id, err := pathSegment("phoneNumberID", phoneNumberID)
	if err != nil {
		return nil, err
	}
	if p.URL == "" || p.VerifyToken == "" {
		return nil, errors.New("hello: URL and VerifyToken are required")
	}
	var out WebhookOverride
	if err := s.c.do(ctx, request{method: http.MethodPut, path: "/v1/" + id + "/webhook-override", body: p, retry: true}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Remove clears the override so Meta sends the number's events to Hello again
// (DELETE /v1/{phone_number_id}/webhook-override). Events delivered to your URL
// in the meantime are not replayed.
func (s *WebhookOverrideService) Remove(ctx context.Context, phoneNumberID string) error {
	id, err := pathSegment("phoneNumberID", phoneNumberID)
	if err != nil {
		return err
	}
	return s.c.do(ctx, request{method: http.MethodDelete, path: "/v1/" + id + "/webhook-override", retry: true}, nil)
}

// Reapply sets the stored override on Meta again, for a "draft" status
// (POST /v1/{phone_number_id}/webhook-override/reapply). The same consequences
// as Set apply. Re-applying the stored value is idempotent, so it is retried.
func (s *WebhookOverrideService) Reapply(ctx context.Context, phoneNumberID string) (*WebhookOverride, error) {
	id, err := pathSegment("phoneNumberID", phoneNumberID)
	if err != nil {
		return nil, err
	}
	var out WebhookOverride
	if err := s.c.do(ctx, request{method: http.MethodPost, path: "/v1/" + id + "/webhook-override/reapply", retry: true}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
