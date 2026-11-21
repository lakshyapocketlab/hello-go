package hello

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// MessagesService sends WhatsApp messages and reads their delivery status.
type MessagesService struct{ c *Client }

// SendOption tunes a send.
type SendOption func(*sendConfig)

type sendConfig struct {
	sync, async    bool
	idempotencyKey string
}

// WithSync waits up to 25 seconds for WhatsApp to accept the message, so
// messages[0].id is WhatsApp's message ID (?sync=true).
func WithSync() SendOption { return func(s *sendConfig) { s.sync = true } }

// WithAsync returns at once with the message queued (?async=true). It wins over
// WithSync and the API key's default send mode.
func WithAsync() SendOption { return func(s *sendConfig) { s.async = true } }

// WithIdempotencyKey sets the Idempotency-Key header: a retry with the same key
// returns the original message instead of sending again. It is also what makes
// the SDK retry a send after a 429, 5xx or network error; without a key a send
// is never retried, because a billed message must not be duplicated.
func WithIdempotencyKey(key string) SendOption {
	return func(s *sendConfig) { s.idempotencyKey = key }
}

// Contact is a recipient in a send response.
type Contact struct {
	Input string `json:"input"`
	WaID  string `json:"wa_id"`
}

// SentMessage is one entry of SendResponse.Messages.
type SentMessage struct {
	// ID is WhatsApp's message ID (wamid): empty while the message is only queued.
	ID            string `json:"id"`
	MessageStatus string `json:"message_status,omitempty"`
	// HelloID is Hello's message ID, always set. Use it with Messages.Get.
	HelloID string `json:"hello_id,omitempty"`
}

// SendResponse is the result of a send, in the shape of the WhatsApp Cloud API.
type SendResponse struct {
	MessagingProduct string        `json:"messaging_product"`
	Contacts         []Contact     `json:"contacts"`
	Messages         []SentMessage `json:"messages"`
}

// Send sends a message given as a WhatsApp Cloud API body: a map, a struct, or
// anything that marshals to that JSON (POST /v1/{phone_number_id}/messages).
// The helpers SendText, SendTemplate and SendMedia cover common cases.
func (s *MessagesService) Send(ctx context.Context, phoneNumberID string, body any, opts ...SendOption) (*SendResponse, error) {
	id, err := pathSegment("phoneNumberID", phoneNumberID)
	if err != nil {
		return nil, err
	}
	if body == nil {
		return nil, errors.New("hello: message body is required")
	}
	var cfg sendConfig
	for _, o := range opts {
		o(&cfg)
	}
	r := request{method: http.MethodPost, path: "/v1/" + id + "/messages", body: body}
	switch {
	case cfg.async:
		r.query = url.Values{"async": {"true"}}
	case cfg.sync:
		r.query = url.Values{"sync": {"true"}}
	}
	if cfg.idempotencyKey != "" {
		r.header = http.Header{"Idempotency-Key": {cfg.idempotencyKey}}
		r.retry = true
	}
	var out SendResponse
	if err := s.c.do(ctx, r, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SendText sends a text message. "to" needs the country code, e.g. +16505551234.
// Outside the 24-hour customer-service window use SendTemplate.
func (s *MessagesService) SendText(ctx context.Context, phoneNumberID, to, text string, opts ...SendOption) (*SendResponse, error) {
	if to == "" || text == "" {
		return nil, errors.New("hello: to and text are required")
	}
	return s.Send(ctx, phoneNumberID, map[string]any{
		"messaging_product": "whatsapp",
		"to":                to,
		"type":              "text",
		"text":              map[string]any{"body": text},
	}, opts...)
}

// TemplateMessage names an approved template to send.
type TemplateMessage struct {
	Name string
	// Language is the template's language code, e.g. "en_US".
	Language string
	// Components are the Cloud API "components" with parameter values, e.g.
	// from BodyParameters. Omitted when empty.
	Components []any
}

// BodyParameters builds the body component of a template send, one text
// parameter per value, in order ({{1}}, {{2}}, ...).
func BodyParameters(values ...string) map[string]any {
	params := make([]any, len(values))
	for i, v := range values {
		params[i] = map[string]any{"type": "text", "text": v}
	}
	return map[string]any{"type": "body", "parameters": params}
}

// SendTemplate sends an approved template, the only message type that can
// start a conversation.
func (s *MessagesService) SendTemplate(ctx context.Context, phoneNumberID, to string, tpl TemplateMessage, opts ...SendOption) (*SendResponse, error) {
	if to == "" || tpl.Name == "" || tpl.Language == "" {
		return nil, errors.New("hello: to, template name and language are required")
	}
	t := map[string]any{"name": tpl.Name, "language": map[string]any{"code": tpl.Language}}
	if len(tpl.Components) > 0 {
		t["components"] = tpl.Components
	}
	return s.Send(ctx, phoneNumberID, map[string]any{
		"messaging_product": "whatsapp",
		"to":                to,
		"type":              "template",
		"template":          t,
	}, opts...)
}

// MediaType is the WhatsApp message type of a media message.
type MediaType string

// Media message types.
const (
	MediaImage    MediaType = "image"
	MediaVideo    MediaType = "video"
	MediaAudio    MediaType = "audio"
	MediaDocument MediaType = "document"
	MediaSticker  MediaType = "sticker"
)

// MediaMessage describes a media message. Set exactly one of ID (from
// Media.Upload) or Link (a public https URL).
type MediaMessage struct {
	Type MediaType
	ID   string
	Link string
	// Caption applies to image, video and document.
	Caption string
	// Filename applies to document and is shown to the recipient.
	Filename string
}

// SendMedia sends an image, video, audio, document or sticker.
func (s *MessagesService) SendMedia(ctx context.Context, phoneNumberID, to string, m MediaMessage, opts ...SendOption) (*SendResponse, error) {
	switch m.Type {
	case MediaImage, MediaVideo, MediaAudio, MediaDocument, MediaSticker:
	default:
		return nil, fmt.Errorf("hello: unsupported media type %q", m.Type)
	}
	if to == "" {
		return nil, errors.New("hello: to is required")
	}
	if (m.ID == "") == (m.Link == "") {
		return nil, errors.New("hello: set exactly one of media ID or Link")
	}
	obj := map[string]any{}
	if m.ID != "" {
		obj["id"] = m.ID
	} else {
		obj["link"] = m.Link
	}
	if m.Caption != "" {
		obj["caption"] = m.Caption
	}
	if m.Filename != "" {
		obj["filename"] = m.Filename
	}
	return s.Send(ctx, phoneNumberID, map[string]any{
		"messaging_product": "whatsapp",
		"to":                to,
		"type":              string(m.Type),
		string(m.Type):      obj,
	}, opts...)
}

// ReadOption tunes MarkAsRead.
type ReadOption func(*readConfig)

type readConfig struct{ typing bool }

// WithTypingIndicator also shows a typing indicator until you reply or 25 seconds pass.
func WithTypingIndicator() ReadOption { return func(r *readConfig) { r.typing = true } }

// MarkAsRead marks an inbound message (its wamid) as read. It is not queued and
// is idempotent, so it is retried like a GET.
func (s *MessagesService) MarkAsRead(ctx context.Context, phoneNumberID, messageID string, opts ...ReadOption) error {
	id, err := pathSegment("phoneNumberID", phoneNumberID)
	if err != nil {
		return err
	}
	if messageID == "" {
		return errors.New("hello: messageID is required")
	}
	var cfg readConfig
	for _, o := range opts {
		o(&cfg)
	}
	body := map[string]any{"messaging_product": "whatsapp", "status": "read", "message_id": messageID}
	if cfg.typing {
		body["typing_indicator"] = map[string]any{"type": "text"}
	}
	return s.c.do(ctx, request{method: http.MethodPost, path: "/v1/" + id + "/messages", body: body, retry: true}, nil)
}

// Message is a sent message with its delivery status (the message object).
type Message struct {
	// ID is Hello's message ID.
	ID        string `json:"id"`
	AccountID string `json:"account_id,omitempty"`
	// MessageID is WhatsApp's wamid; empty while queued.
	MessageID      string         `json:"message_id,omitempty"`
	ConversationID string         `json:"conversation_id,omitempty"`
	Direction      string         `json:"direction,omitempty"`
	MessageType    string         `json:"message_type,omitempty"`
	RecipientPhone string         `json:"recipient_phone,omitempty"`
	SenderPhone    string         `json:"sender_phone,omitempty"`
	TemplateName   string         `json:"template_name,omitempty"`
	Content        map[string]any `json:"content,omitempty"`
	Components     []any          `json:"components,omitempty"`
	// Status is queued, sent, delivered, read or failed.
	Status               string     `json:"status"`
	ErrorCode            string     `json:"error_code,omitempty"`
	ErrorMessage         string     `json:"error_message,omitempty"`
	ConversationCategory string     `json:"conversation_category,omitempty"`
	PricingType          string     `json:"pricing_type,omitempty"`
	SentAt               *time.Time `json:"sent_at,omitempty"`
	DeliveredAt          *time.Time `json:"delivered_at,omitempty"`
	ReadAt               *time.Time `json:"read_at,omitempty"`
	PlayedAt             *time.Time `json:"played_at,omitempty"`
	CreatedAt            *time.Time `json:"created_at,omitempty"`
}

// Get returns a sent message and its status by Hello's ID (messages[0].hello_id)
// or WhatsApp's wamid (GET /v1/messages/{id}).
func (s *MessagesService) Get(ctx context.Context, id string) (*Message, error) {
	seg, err := pathSegment("id", id)
	if err != nil {
		return nil, err
	}
	var out Message
	if err := s.c.do(ctx, request{method: http.MethodGet, path: "/v1/messages/" + seg, retry: true}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
