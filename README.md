# Hello Go SDK for the WhatsApp Business API

Official Go SDK for Hello, the WhatsApp Business API. Send messages, templates and webhooks.

[![CI](https://github.com/lakshyapocketlab/hello-go/actions/workflows/ci.yml/badge.svg)](https://github.com/lakshyapocketlab/hello-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/lakshyapocketlab/hello-go.svg)](https://pkg.go.dev/github.com/lakshyapocketlab/hello-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/lakshyapocketlab/hello-go)](https://goreportcard.com/report/github.com/lakshyapocketlab/hello-go)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Latest tag](https://img.shields.io/github/v/tag/lakshyapocketlab/hello-go)](https://github.com/lakshyapocketlab/hello-go/tags)

Standard library only, Go 1.22+, `context.Context` on every call, 100% test coverage.

## Quick start

**1. Install**

```sh
go get github.com/lakshyapocketlab/hello-go
```

**2. Get an API key.** Create one (`hello_live_...`) on the API Keys page of the [Hello dashboard](https://hello.lakshyapocketlab.com), then export it:

```sh
export HELLO_API_KEY=hello_live_...
```

**3. Send your first message.** Replace `PHONE_NUMBER_ID` with your sending number's ID (see [Setup](#phone-number-id-and-waba-id)). Free-form text needs the recipient to have messaged you in the last 24 hours; otherwise send an approved template.

```go
package main

import (
	"context"
	"fmt"
	"log"

	hello "github.com/lakshyapocketlab/hello-go"
)

func main() {
	client, err := hello.NewClient() // reads HELLO_API_KEY
	if err != nil { log.Fatal(err) }
	resp, err := client.Messages.SendText(context.Background(), "PHONE_NUMBER_ID", "+16505551234", "Your order has shipped.")
	if err != nil { log.Fatal(err) }
	fmt.Println("Hello ID:", resp.Messages[0].HelloID)
}
```

The runnable version is [`examples/send-text`](examples/send-text/main.go).

## Setup

### Prerequisites

- Go 1.22 or newer. The SDK uses only the standard library.

### Phone number ID and WABA ID

- `phone_number_id` identifies the sending number. Messages and media use it.
- `waba_id` (WhatsApp Business Account ID) identifies the account. Templates use it.

Both are shown for each number in the Hello dashboard. You can also read them in code:

```go
numbers, err := client.PhoneNumbers.List(ctx)
// numbers[0].PhoneNumberID -> Messages.*, Media.Upload
// numbers[0].WabaID        -> Templates.*
```

### API key and base URL

Pass the key explicitly, or leave it out and set `HELLO_API_KEY`:

```go
client, err := hello.NewClient(hello.WithAPIKey("hello_live_..."))  // explicit option wins
client, err := hello.NewClient()                                    // falls back to HELLO_API_KEY
```

With neither set, `NewClient` returns `hello.ErrMissingAPIKey`.

The API host defaults to `https://helloapi.lakshyapocketlab.com`. Override it with `hello.WithBaseURL(...)` or the `HELLO_BASE_URL` environment variable (the option wins over the variable). Use this for staging or a self-hosted gateway.

### Client options

```go
client, err := hello.NewClient(
	hello.WithAPIKey("hello_live_..."),                          // default: env HELLO_API_KEY
	hello.WithBaseURL("https://helloapi.lakshyapocketlab.com"), // default; env HELLO_BASE_URL
	hello.WithHTTPClient(&http.Client{}),                       // custom transport, proxy, tracing
	hello.WithTimeout(30*time.Second),                          // per attempt; default 60s, 0 = none
	hello.WithMaxRetries(2),                                    // default 2
)
```

Requests send `Authorization: Bearer <key>` and `User-Agent: hello-go/<version>`. Path parameters are URL-escaped.

## Usage

Every method takes a `context.Context` first. The examples below use `ctx := context.Background()` and a `client` built as in [Setup](#setup).

### Send a text message

```go
resp, err := client.Messages.SendText(ctx, pnid, "+16505551234", "Hi Asha")
fmt.Println(resp.Messages[0].HelloID) // follow it with Messages.Get
```

### Send a template

Templates are the only type that can start a conversation. Body variables (`{{1}}`, `{{2}}`) go in `BodyParameters`, in order:

```go
resp, err := client.Messages.SendTemplate(ctx, pnid, "+16505551234", hello.TemplateMessage{
	Name:       "order_shipped",
	Language:   "en_US",
	Components: []any{hello.BodyParameters("Asha", "#10432")},
})
```

### Send media (upload, then send)

Media uploaded from one number can only be sent from that number.

```go
f, err := os.Open("invoice.pdf")
if err != nil { log.Fatal(err) }
defer f.Close()

media, err := client.Media.Upload(ctx, pnid, f, "invoice.pdf", "application/pdf") // media.ID
resp, err := client.Messages.SendMedia(ctx, pnid, "+16505551234", hello.MediaMessage{
	Type: hello.MediaDocument, ID: media.ID, Caption: "Invoice", Filename: "invoice.pdf",
})
```

Media that already lives at a public URL needs no upload: set `Link` on `hello.MediaMessage` instead of `ID`. For other message types (reaction, location, interactive, ...), call `Messages.Send` with the raw Cloud API body.

To download inbound media, use `Media.Download`. It returns an `io.ReadCloser` that you must close:

```go
info, err := client.Media.Get(ctx, mediaID)  // mime type, size, sha256
body, err := client.Media.Download(ctx, mediaID)
defer body.Close()
```

### Mark as read

```go
err := client.Messages.MarkAsRead(ctx, pnid, inboundWamid)
err = client.Messages.MarkAsRead(ctx, pnid, inboundWamid, hello.WithTypingIndicator())
```

### Message status

```go
msg, err := client.Messages.Get(ctx, resp.Messages[0].HelloID)
fmt.Println(msg.Status) // queued, sent, delivered, read or failed
```

### Send options

| Option | Effect |
|---|---|
| `hello.WithSync()` | Wait up to 25 s for WhatsApp to accept; `messages[0].id` is then the wamid. |
| `hello.WithAsync()` | Return at once, queued. Wins over sync. |
| `hello.WithIdempotencyKey(key)` | Makes the send safe to retry. See [Retries](#idempotency-and-retries). |

Without sync or async, the API key's default send mode applies. `messages[0].hello_id` is always set.

### Templates

```go
list, err := client.Templates.List(ctx, wabaID, &hello.ListTemplatesParams{Status: "APPROVED", Limit: 50})
for _, t := range list.Data { fmt.Println(t.Name, t.Language) }
next := list.Paging.Cursors.After // pass as ListTemplatesParams.After for the next page; params may be nil

created, err := client.Templates.Create(ctx, wabaID, map[string]any{
	"name": "order_shipped", "language": "en", "category": "UTILITY",
	"components": []any{map[string]any{"type": "BODY", "text": "Hi {{1}}, order {{2}} shipped."}},
})
err = client.Templates.Edit(ctx, created.ID, map[string]any{"category": "UTILITY"})
res, err := client.Templates.CreateBulk(ctx, wabaID, []any{ /* up to 100 create bodies */ })
sync, err := client.Templates.Sync(ctx, wabaID)
err = client.Templates.Delete(ctx, templateID)          // one language
err = client.Templates.DeleteByName(ctx, wabaID, name)  // every language
```

A full paging loop is in [`examples/list-templates`](examples/list-templates/main.go).

### Phone numbers

```go
numbers, err := client.PhoneNumbers.List(ctx)                 // all numbers of the key's organization
waba, err := client.PhoneNumbers.ListForWABA(ctx, wabaID)     // numbers of one WABA
```

### Direct webhook (webhook override)

This points a number's Meta webhook straight at your URL. Hello's dashboard then stops tracking that number's incoming messages and statuses, so read the Hello docs on direct mode before you enable it.

```go
ov, err := client.WebhookOverride.Set(ctx, pnid, hello.SetWebhookOverrideParams{
	URL: "https://example.com/hooks/meta/9f2c4e1a7b3d", VerifyToken: "a-long-token",
	AcknowledgeNoTracking: true,
})
ov, err = client.WebhookOverride.Get(ctx, pnid, true) // refresh=true checks Meta's live setting
ov, err = client.WebhookOverride.Reapply(ctx, pnid)   // when status is "draft"
err = client.WebhookOverride.Remove(ctx, pnid)
```

### Errors

Non-2xx responses return a `*hello.APIError`. Both server error formats are parsed (Meta-style and Hello-style).

| Field | Meaning |
|---|---|
| `Status` | HTTP status code |
| `Message` | Human-readable message |
| `HelloCode` | Hello's code, e.g. `VALIDATION_ERROR`, `PLAN_DAILY_LIMIT_REACHED` |
| `Code`, `Subcode` | Meta's numeric error code and subcode (0 when absent) |
| `Type`, `Details` | Meta's error type and details text |
| `FBTraceID`, `RequestID` | Quote these when you contact support |
| `Data` | The raw error payload |

```go
_, err := client.Messages.SendText(ctx, pnid, to, "hi")

var apiErr *hello.APIError
if errors.As(err, &apiErr) {
	fmt.Println(apiErr.Status, apiErr.HelloCode, apiErr.FBTraceID, apiErr.Message)
}

switch {
case hello.IsAuthentication(err): // 401 / Meta 190: bad or disabled key
case hello.IsNotFound(err):       // 404
case hello.IsRateLimit(err):      // 429
}
```

`errors.Is(err, hello.ErrNotFound)` (and `ErrAuthentication`, `ErrRateLimit`) also work. Branch on `HelloCode` or `Code`, not on message text. Client-side validation problems (an empty ID, a missing body) and transport failures are ordinary wrapped errors.

A complete program is in [`examples/error-handling`](examples/error-handling/main.go).

### Idempotency and retries

Retryable requests are retried on HTTP 429, any 5xx and network errors, up to `WithMaxRetries` extra attempts (default 2). Backoff is exponential (500 ms base, 8 s cap) with jitter. A `Retry-After` header (seconds or HTTP date, capped at 60 s) replaces the backoff.

- **Always retried:** every `GET`, every `DELETE`, `WebhookOverride.Set` (PUT), `Templates.Sync`, `Messages.MarkAsRead` and `WebhookOverride.Reapply`.
- **Message sends are retried only with an idempotency key.** Pass `hello.WithIdempotencyKey(key)`. Hello returns the original message for a repeated key, so a billed send is never duplicated. Reuse the same key for the same logical message, for example `fmt.Sprintf("order-%d-shipped", orderID)`.
- **Never retried:** `Media.Upload`, `Templates.Create`, `Templates.Edit` and `Templates.CreateBulk`.

```go
resp, err := client.Messages.SendText(ctx, pnid, to, "Order shipped",
	hello.WithIdempotencyKey(fmt.Sprintf("order-%d-shipped", orderID)))
```

### Context and cancellation

Every call takes a `context.Context`. Use it for deadlines and cancellation; cancelling it stops in-flight requests and any pending retry at once:

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

list, err := client.Templates.List(ctx, wabaID, nil)
```

## Examples

Runnable programs live in [`examples/`](examples). Each one reads `HELLO_API_KEY` and prints its usage when run without arguments.

| Example | What it shows | Run |
|---|---|---|
| [`send-text`](examples/send-text/main.go) | Send a text message | `go run ./examples/send-text <phone_number_id> <to> "Hello"` |
| [`send-template`](examples/send-template/main.go) | Send an approved template with body variables and an idempotency key | `go run ./examples/send-template <phone_number_id> <to> <template> <language> [param...]` |
| [`upload-and-send-media`](examples/upload-and-send-media/main.go) | Upload a file, then send it as a document | `go run ./examples/upload-and-send-media <phone_number_id> <to> ./invoice.pdf application/pdf` |
| [`error-handling`](examples/error-handling/main.go) | Branch on `IsAuthentication`, `IsRateLimit`, `IsNotFound` and `APIError` | `go run ./examples/error-handling <phone_number_id> <to> "Hello"` |
| [`list-templates`](examples/list-templates/main.go) | List approved templates across all pages | `go run ./examples/list-templates <waba_id>` |

More examples: [hello-examples](https://github.com/lakshyapocketlab/hello-examples).

## API reference

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

## Development

```sh
go vet ./...
go test -race -coverprofile=cover.out .
go tool cover -func=cover.out | tail -1   # must be 100.0%
```

## Links

- Docs: [lakshyapocketlab.com/docs](https://lakshyapocketlab.com/docs/)
- Examples: [hello-examples](https://github.com/lakshyapocketlab/hello-examples)
- MCP server: [hello-mcp](https://github.com/lakshyapocketlab/hello-mcp)
- Support: support@lakshyapocketlab.com
- [CONTRIBUTING.md](CONTRIBUTING.md) · [SECURITY.md](SECURITY.md) · [CHANGELOG.md](CHANGELOG.md)

## License

MIT, copyright Lakshya Pocket Lab.

<!-- GitHub topics: whatsapp, whatsapp-business-api, whatsapp-api, whatsapp-cloud-api, mcp, golang -->
