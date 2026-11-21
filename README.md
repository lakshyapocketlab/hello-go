# Hello Go SDK for the WhatsApp Business API

Official Go SDK for Hello, the WhatsApp Business API. Send messages, templates and webhooks.

[![CI](https://github.com/lakshyapocketlab/hello-go/actions/workflows/ci.yml/badge.svg)](https://github.com/lakshyapocketlab/hello-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/lakshyapocketlab/hello-go.svg)](https://pkg.go.dev/github.com/lakshyapocketlab/hello-go)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Standard library only, Go 1.22+, `context.Context` on every call, 100% test coverage.

## Install

```sh
go get github.com/lakshyapocketlab/hello-go
```

## Quickstart

```go
package main

import (
	"context"
	"fmt"
	"log"

	hello "github.com/lakshyapocketlab/hello-go"
)

func main() {
	// Reads HELLO_API_KEY (hello_live_...) unless hello.WithAPIKey is given.
	client, err := hello.NewClient()
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	numbers, err := client.PhoneNumbers.List(ctx)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := client.Messages.SendText(ctx, numbers[0].PhoneNumberID, "+16505551234", "Your order has shipped.")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Hello message ID:", resp.Messages[0].HelloID)
}
```

Runnable programs are in [`examples/`](examples): `send-text`, `send-template`, `upload-and-send-media`.

## Client

```go
client, err := hello.NewClient(
	hello.WithAPIKey("hello_live_..."),            // default: env HELLO_API_KEY; error if empty
	hello.WithBaseURL("https://api.lakshyapocketlab.com"), // default; override for staging/self-host
	hello.WithHTTPClient(&http.Client{}),          // custom transport, proxy, tracing
	hello.WithTimeout(30*time.Second),             // per attempt, default 60s, 0 = none
	hello.WithMaxRetries(2),                       // default 2
)
```

Requests carry `Authorization: Bearer <key>` and `User-Agent: hello-go/<version>`.
Path parameters are URL-escaped. A missing key returns `hello.ErrMissingAPIKey`.

## Methods

Every method takes a `context.Context` first.

| Method | Route |
|---|---|
| `client.Ping(ctx)` | `GET /v1/ping` |
| `client.PhoneNumbers.List(ctx)` | `GET /v1/phone_numbers` |
| `client.PhoneNumbers.ListForWABA(ctx, wabaID)` | `GET /v1/{waba_id}/phone_numbers` |
| `client.Messages.Send(ctx, phoneNumberID, body, opts...)` | `POST /v1/{phone_number_id}/messages` |
| `client.Messages.SendText(ctx, phoneNumberID, to, text, opts...)` | same |
| `client.Messages.SendTemplate(ctx, phoneNumberID, to, tpl, opts...)` | same |
| `client.Messages.SendMedia(ctx, phoneNumberID, to, media, opts...)` | same |
| `client.Messages.MarkAsRead(ctx, phoneNumberID, wamid, opts...)` | same (`status: read`) |
| `client.Messages.Get(ctx, id)` | `GET /v1/messages/{id}` |
| `client.Media.Upload(ctx, phoneNumberID, file, filename, mimeType)` | `POST /v1/{phone_number_id}/media` |
| `client.Media.Get(ctx, mediaID)` | `GET /v1/{media_id}` |
| `client.Media.Download(ctx, mediaID)` | `GET /v1/{media_id}/download` |
| `client.Templates.List(ctx, wabaID, params)` | `GET /v1/{waba_id}/message_templates` |
| `client.Templates.Get(ctx, templateID)` | `GET /v1/templates/{template_id}` |
| `client.Templates.Create(ctx, wabaID, template)` | `POST /v1/{waba_id}/message_templates` |
| `client.Templates.CreateBulk(ctx, wabaID, templates)` | `POST /v1/{waba_id}/templates/bulk` |
| `client.Templates.Edit(ctx, templateID, changes)` | `POST /v1/templates/{template_id}` |
| `client.Templates.Sync(ctx, wabaID)` | `POST /v1/{waba_id}/templates/sync` |
| `client.Templates.Delete(ctx, templateID)` | `DELETE /v1/templates/{template_id}` |
| `client.Templates.DeleteByName(ctx, wabaID, name)` | `DELETE /v1/{waba_id}/message_templates?name=` |
| `client.WebhookOverride.Get(ctx, phoneNumberID, refresh)` | `GET /v1/{phone_number_id}/webhook-override` |
| `client.WebhookOverride.Set(ctx, phoneNumberID, params)` | `PUT /v1/{phone_number_id}/webhook-override` |
| `client.WebhookOverride.Remove(ctx, phoneNumberID)` | `DELETE /v1/{phone_number_id}/webhook-override` |
| `client.WebhookOverride.Reapply(ctx, phoneNumberID)` | `POST /v1/{phone_number_id}/webhook-override/reapply` |

### Sending messages

`Send` takes the WhatsApp Cloud API message body (a map, a struct, anything that marshals to that
JSON), so every message type works. The helpers cover the common ones:

```go
// Text (needs the recipient to have messaged you in the last 24 hours).
client.Messages.SendText(ctx, pnid, "+16505551234", "Hi")

// Template: the only type that can start a conversation.
client.Messages.SendTemplate(ctx, pnid, "+16505551234", hello.TemplateMessage{
	Name:       "order_shipped",
	Language:   "en_US",
	Components: []any{hello.BodyParameters("Asha", "#10432")}, // {{1}}, {{2}}
})

// Media by uploaded ID or public link.
client.Messages.SendMedia(ctx, pnid, "+16505551234", hello.MediaMessage{
	Type: hello.MediaDocument, ID: mediaID, Caption: "Invoice", Filename: "invoice.pdf",
})

// Anything else: raw Cloud API body.
client.Messages.Send(ctx, pnid, map[string]any{
	"to": "+16505551234", "type": "reaction",
	"reaction": map[string]any{"message_id": "wamid...", "emoji": "\U0001F44D"},
})
```

Send options: `hello.WithSync()` (wait up to 25s for WhatsApp to accept; `messages[0].id` is the wamid),
`hello.WithAsync()` (return at once, queued; wins over sync) and `hello.WithIdempotencyKey(key)`.
Without sync/async the API key's default send mode applies. `messages[0].hello_id` is always set:
follow it with `client.Messages.Get`.

```go
msg, err := client.Messages.Get(ctx, resp.Messages[0].HelloID)
fmt.Println(msg.Status) // queued, sent, delivered, read, failed
```

`MarkAsRead(ctx, pnid, wamid, hello.WithTypingIndicator())` marks an inbound message as read.

### Media

```go
f, _ := os.Open("invoice.pdf")
media, err := client.Media.Upload(ctx, pnid, f, "invoice.pdf", "application/pdf") // media.ID

info, err := client.Media.Get(ctx, media.ID)          // mime type, size, sha256
body, err := client.Media.Download(ctx, media.ID)     // io.ReadCloser; close it
```

### Templates

```go
list, err := client.Templates.List(ctx, wabaID, &hello.ListTemplatesParams{Status: "APPROVED", Limit: 50})
for _, t := range list.Data { fmt.Println(t.Name, t.Language) }
next := list.Paging.Cursors.After // pass as ListTemplatesParams.After for the next page; params may be nil

created, err := client.Templates.Create(ctx, wabaID, map[string]any{
	"name": "order_shipped", "language": "en", "category": "UTILITY",
	"components": []any{map[string]any{"type": "BODY", "text": "Hi {{1}}, order {{2}} shipped."}},
})
res, err := client.Templates.CreateBulk(ctx, wabaID, []any{ /* up to 100 create bodies */ })
err = client.Templates.Edit(ctx, created.ID, map[string]any{"category": "UTILITY"})
sync, err := client.Templates.Sync(ctx, wabaID)
err = client.Templates.Delete(ctx, templateID)          // one language
err = client.Templates.DeleteByName(ctx, wabaID, name)  // every language
```

### Direct webhook (webhook override)

Points a number's Meta webhook straight at your URL. Hello's dashboard then stops tracking that
number's incoming messages and statuses, so read the Hello docs on direct mode first.

```go
ov, err := client.WebhookOverride.Set(ctx, pnid, hello.SetWebhookOverrideParams{
	URL: "https://example.com/hooks/meta/9f2c4e1a7b3d", VerifyToken: "a-long-token",
	AcknowledgeNoTracking: true,
})
ov, err = client.WebhookOverride.Get(ctx, pnid, true) // refresh=true checks Meta's live setting
ov, err = client.WebhookOverride.Reapply(ctx, pnid)   // when status is "draft"
err = client.WebhookOverride.Remove(ctx, pnid)
```

## Errors

Non-2xx responses return a `*hello.APIError`. Both envelopes are parsed: Meta-style
`{"error": {message, type, code, error_subcode, error_data: {details, hello_code}, fbtrace_id}}`
on sends, media and `message_templates`, and Hello-style `{message, code}` elsewhere.

```go
_, err := client.Messages.SendText(ctx, pnid, to, "hi")

var apiErr *hello.APIError
if errors.As(err, &apiErr) {
	fmt.Println(apiErr.Status, apiErr.Message)
	fmt.Println(apiErr.Code, apiErr.Subcode)   // Meta's numeric codes
	fmt.Println(apiErr.HelloCode)              // e.g. VALIDATION_ERROR, PLAN_DAILY_LIMIT_REACHED
	fmt.Println(apiErr.FBTraceID, apiErr.Details, apiErr.Data)
}

switch {
case hello.IsAuthentication(err): // 401 / Meta 190: bad or disabled key
case hello.IsNotFound(err):       // 404
case hello.IsRateLimit(err):      // 429
}
```

`errors.Is(err, hello.ErrNotFound)` (and `ErrAuthentication`, `ErrRateLimit`) works too. Branch on
`HelloCode` or `Code`, not on message text. Client-side validation problems (an empty ID, a missing
body) and transport failures are ordinary wrapped errors.

## Retries

Retryable requests are retried on HTTP 429, any 5xx and network errors, up to `WithMaxRetries`
(default 2) extra attempts, with exponential backoff (500ms base, 8s cap) and jitter.
A `Retry-After` header (seconds or HTTP date, capped at 60s) is honored instead of the backoff.
Context cancellation stops retrying at once.

- Safe to retry: every `GET`, every `DELETE`, `WebhookOverride.Set` (PUT), `Templates.Sync`, `Messages.MarkAsRead` and `WebhookOverride.Reapply`.
- Message **sends are retried only when you pass `hello.WithIdempotencyKey`**: Hello's `Idempotency-Key`
  header returns the original message for a repeated key, so a billed send is never duplicated.
- Not retried: `Media.Upload`, `Templates.Create`, `Templates.Edit` and `Templates.CreateBulk`.

## Development

```sh
go vet ./...
go test -race -coverprofile=cover.out .
go tool cover -func=cover.out | tail -1   # must be 100.0%
```

See [CONTRIBUTING.md](CONTRIBUTING.md), [SECURITY.md](SECURITY.md) and [CHANGELOG.md](CHANGELOG.md).
MIT licensed, copyright Lakshya Pocket Lab.

<!-- GitHub topics: whatsapp, whatsapp-business-api, whatsapp-api, whatsapp-cloud-api, mcp, golang -->
