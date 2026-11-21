package hello

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
)

// TemplatesService manages WhatsApp message templates. Templates belong to a
// WhatsApp Business Account (waba_id), not to a single number. Lists, creates and
// deletes by name use Meta's /{waba_id}/message_templates paths.
type TemplatesService struct{ c *Client }

// Template is a message template.
type Template struct {
	// ID is Meta's template ID (numeric string).
	ID           string         `json:"id"`
	WabaID       string         `json:"waba_id,omitempty"`
	Name         string         `json:"name"`
	Language     string         `json:"language"`
	Status       string         `json:"status"`
	Category     string         `json:"category,omitempty"`
	Components   []any          `json:"components,omitempty"`
	QualityScore map[string]any `json:"quality_score,omitempty"`
}

// TemplateList is one page of templates.
type TemplateList struct {
	Data   []Template `json:"data"`
	Paging struct {
		Cursors struct {
			Before string `json:"before"`
			After  string `json:"after"`
		} `json:"cursors"`
	} `json:"paging"`
	// Summary is set when ListTemplatesParams.Summary was requested.
	Summary *struct {
		TotalCount   int            `json:"total_count"`
		StatusCounts map[string]int `json:"status_counts,omitempty"`
	} `json:"summary,omitempty"`
}

// ListTemplatesParams filters and pages a template list. Zero values are omitted.
type ListTemplatesParams struct {
	Name      string // exact name
	Search    string // case-insensitive substring of the name
	Status    string // APPROVED, PENDING, REJECTED, PAUSED, DISABLED
	Category  string // MARKETING, UTILITY, AUTHENTICATION
	Language  string // e.g. en_US
	Limit     int    // 1-100, default 25
	After     string // cursor: the page after it
	Before    string // cursor: the page before it
	Summary   string // comma-separated: total_count, status_counts
	SortBy    string // created (default) or name
	SortOrder string // asc or desc (default)
}

func (p *ListTemplatesParams) values() url.Values {
	q := url.Values{}
	if p == nil {
		return q
	}
	for k, v := range map[string]string{
		"name": p.Name, "search": p.Search, "status": p.Status, "category": p.Category,
		"language": p.Language, "after": p.After, "before": p.Before, "summary": p.Summary,
		"sort_by": p.SortBy, "sort_order": p.SortOrder,
	} {
		if v != "" {
			q.Set(k, v)
		}
	}
	if p.Limit > 0 {
		q.Set("limit", strconv.Itoa(p.Limit))
	}
	return q
}

// List returns a page of the WABA's templates, newest first
// (GET /v1/{waba_id}/message_templates). params may be nil.
func (s *TemplatesService) List(ctx context.Context, wabaID string, params *ListTemplatesParams) (*TemplateList, error) {
	id, err := pathSegment("wabaID", wabaID)
	if err != nil {
		return nil, err
	}
	var out TemplateList
	r := request{method: http.MethodGet, path: "/v1/" + id + "/message_templates", query: params.values(), retry: true}
	if err := s.c.do(ctx, r, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get returns one template by Meta's template ID (GET /v1/templates/{template_id}).
func (s *TemplatesService) Get(ctx context.Context, templateID string) (*Template, error) {
	id, err := pathSegment("templateID", templateID)
	if err != nil {
		return nil, err
	}
	var out Template
	if err := s.c.do(ctx, request{method: http.MethodGet, path: "/v1/templates/" + id, retry: true}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreatedTemplate is Meta's create response.
type CreatedTemplate struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Category string `json:"category"`
}

// Create submits a template to Meta for review in Meta's create format (name,
// language, category, components): a map, a struct, or any JSON-marshalling
// value (POST /v1/{waba_id}/message_templates). It starts PENDING.
func (s *TemplatesService) Create(ctx context.Context, wabaID string, template any) (*CreatedTemplate, error) {
	id, err := pathSegment("wabaID", wabaID)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, errors.New("hello: template is required")
	}
	var out CreatedTemplate
	if err := s.c.do(ctx, request{method: http.MethodPost, path: "/v1/" + id + "/message_templates", body: template}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// BulkRowResult is the outcome of one row of CreateBulk.
type BulkRowResult struct {
	Index        int    `json:"index"`
	TemplateName string `json:"template_name"`
	Language     string `json:"language"`
	// Status is created, updated, failed or skipped.
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// BulkResult is the result of CreateBulk.
type BulkResult struct {
	TotalRows    int             `json:"total_rows"`
	CreatedCount int             `json:"created_count"`
	UpdatedCount int             `json:"updated_count"`
	FailedCount  int             `json:"failed_count"`
	SkippedCount int             `json:"skipped_count"`
	Results      []BulkRowResult `json:"results"`
}

// CreateBulk creates or edits up to 100 templates in one call; a row whose name
// and language already exist edits that template (POST /v1/{waba_id}/templates/bulk).
func (s *TemplatesService) CreateBulk(ctx context.Context, wabaID string, templates []any) (*BulkResult, error) {
	id, err := pathSegment("wabaID", wabaID)
	if err != nil {
		return nil, err
	}
	if len(templates) == 0 {
		return nil, errors.New("hello: templates is required")
	}
	var out BulkResult
	r := request{method: http.MethodPost, path: "/v1/" + id + "/templates/bulk", body: map[string]any{"templates": templates}}
	if err := s.c.do(ctx, r, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Edit changes a template on Meta (APPROVED, REJECTED or PAUSED ones only); its
// status resets to PENDING (POST /v1/templates/{template_id}).
func (s *TemplatesService) Edit(ctx context.Context, templateID string, changes any) error {
	id, err := pathSegment("templateID", templateID)
	if err != nil {
		return err
	}
	if changes == nil {
		return errors.New("hello: changes is required")
	}
	return s.c.do(ctx, request{method: http.MethodPost, path: "/v1/templates/" + id, body: changes}, nil)
}

// SyncResult is the result of Sync.
type SyncResult struct {
	Created int `json:"created"`
	Updated int `json:"updated"`
	Deleted int `json:"deleted"`
	Total   int `json:"total"`
}

// Sync re-reads every template of the WABA from Meta
// (POST /v1/{waba_id}/templates/sync). Re-reading is idempotent, so it is retried.
func (s *TemplatesService) Sync(ctx context.Context, wabaID string) (*SyncResult, error) {
	id, err := pathSegment("wabaID", wabaID)
	if err != nil {
		return nil, err
	}
	var out SyncResult
	if err := s.c.do(ctx, request{method: http.MethodPost, path: "/v1/" + id + "/templates/sync", retry: true}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete permanently deletes one language of a template, on Meta and in Hello
// (DELETE /v1/templates/{template_id}). It cannot be undone.
func (s *TemplatesService) Delete(ctx context.Context, templateID string) error {
	id, err := pathSegment("templateID", templateID)
	if err != nil {
		return err
	}
	return s.c.do(ctx, request{method: http.MethodDelete, path: "/v1/templates/" + id, retry: true}, nil)
}

// DeleteByName permanently deletes every language of a template name
// (DELETE /v1/{waba_id}/message_templates?name=). It cannot be undone.
func (s *TemplatesService) DeleteByName(ctx context.Context, wabaID, name string) error {
	id, err := pathSegment("wabaID", wabaID)
	if err != nil {
		return err
	}
	if name == "" {
		return errors.New("hello: name is required")
	}
	r := request{method: http.MethodDelete, path: "/v1/" + id + "/message_templates", query: url.Values{"name": {name}}, retry: true}
	return s.c.do(ctx, r, nil)
}
