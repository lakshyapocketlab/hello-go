package hello

import (
	"context"
	"testing"
)

const sendOK = `{"messaging_product":"whatsapp","contacts":[{"input":"+1","wa_id":"1"}],"messages":[{"id":"wamid.X=","message_status":"accepted","hello_id":"uuid-1"}]}`

func TestSendRawAndOptions(t *testing.T) {
	f, c := newFake(t, jsonReply(200, sendOK))
	ctx := context.Background()
	body := map[string]any{"to": "+1", "type": "text", "text": map[string]any{"body": "hi"}}

	got, err := c.Messages.Send(ctx, "106/5", body)
	if err != nil || got.Messages[0].ID != "wamid.X=" || got.Messages[0].HelloID != "uuid-1" || got.Contacts[0].WaID != "1" {
		t.Fatalf("%+v %v", got, err)
	}
	r := f.last()
	expect(t, r, "POST", "/v1/106%2F5/messages")
	if r.Query != "" || r.Header.Get("Idempotency-Key") != "" || r.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("unexpected: %+v", r)
	}
	if decodeBody(t, r)["type"] != "text" {
		t.Fatal("body not forwarded")
	}

	_, _ = c.Messages.Send(ctx, "1", body, WithSync())
	if f.last().Query != "sync=true" {
		t.Fatalf("query = %q", f.last().Query)
	}
	_, _ = c.Messages.Send(ctx, "1", body, WithSync(), WithAsync())
	if f.last().Query != "async=true" {
		t.Fatalf("async must win: %q", f.last().Query)
	}
	_, _ = c.Messages.Send(ctx, "1", body, WithIdempotencyKey("k-1"))
	if f.last().Header.Get("Idempotency-Key") != "k-1" {
		t.Fatal("missing idempotency key")
	}

	if _, err := c.Messages.Send(ctx, "", body); err == nil {
		t.Fatal("empty phone number id")
	}
	if _, err := c.Messages.Send(ctx, "1", nil); err == nil {
		t.Fatal("nil body")
	}
	if _, err := c.Messages.Send(ctx, "1", make(chan int)); err == nil {
		t.Fatal("bad body")
	}
}

func TestSendRetriesOnlyWithIdempotencyKey(t *testing.T) {
	f, c := newFake(t, jsonReply(502, `{"error":{"message":"meta down","code":2,"error_data":{"hello_code":"META_UNAVAILABLE"}}}`))
	ctx := context.Background()
	_, err := c.Messages.SendText(ctx, "1", "+1", "hi")
	if err == nil || f.count() != 1 {
		t.Fatalf("no key: attempts=%d", f.count())
	}
	_, err = c.Messages.SendText(ctx, "1", "+1", "hi", WithIdempotencyKey("k"))
	if err == nil || f.count() != 1+3 {
		t.Fatalf("with key: attempts=%d", f.count())
	}
}

func TestSendText(t *testing.T) {
	f, c := newFake(t, jsonReply(202, sendOK))
	if _, err := c.Messages.SendText(context.Background(), "1", "+16505551234", "Hello", WithAsync()); err != nil {
		t.Fatal(err)
	}
	r := f.last()
	expect(t, r, "POST", "/v1/1/messages")
	b := decodeBody(t, r)
	if b["messaging_product"] != "whatsapp" || b["to"] != "+16505551234" || b["type"] != "text" ||
		b["text"].(map[string]any)["body"] != "Hello" {
		t.Fatalf("%v", b)
	}
	if _, err := c.Messages.SendText(context.Background(), "1", "", "x"); err == nil {
		t.Fatal("empty to")
	}
	if _, err := c.Messages.SendText(context.Background(), "1", "+1", ""); err == nil {
		t.Fatal("empty text")
	}
}

func TestSendTemplate(t *testing.T) {
	f, c := newFake(t, jsonReply(200, sendOK))
	ctx := context.Background()
	_, err := c.Messages.SendTemplate(ctx, "1", "+1", TemplateMessage{
		Name: "order_shipped", Language: "en_US", Components: []any{BodyParameters("Asha", "#1")},
	})
	if err != nil {
		t.Fatal(err)
	}
	tpl := decodeBody(t, f.last())["template"].(map[string]any)
	comp := tpl["components"].([]any)[0].(map[string]any)
	params := comp["parameters"].([]any)
	if tpl["name"] != "order_shipped" || tpl["language"].(map[string]any)["code"] != "en_US" ||
		comp["type"] != "body" || len(params) != 2 || params[1].(map[string]any)["text"] != "#1" {
		t.Fatalf("%v", tpl)
	}
	if decodeBody(t, f.last())["type"] != "template" {
		t.Fatal("type")
	}

	_, _ = c.Messages.SendTemplate(ctx, "1", "+1", TemplateMessage{Name: "n", Language: "en"})
	if _, ok := decodeBody(t, f.last())["template"].(map[string]any)["components"]; ok {
		t.Fatal("components must be omitted when empty")
	}
	for _, tm := range []TemplateMessage{{Language: "en"}, {Name: "n"}} {
		if _, err := c.Messages.SendTemplate(ctx, "1", "+1", tm); err == nil {
			t.Fatal("missing fields")
		}
	}
	if _, err := c.Messages.SendTemplate(ctx, "1", "", TemplateMessage{Name: "n", Language: "en"}); err == nil {
		t.Fatal("missing to")
	}
}

func TestSendMedia(t *testing.T) {
	f, c := newFake(t, jsonReply(200, sendOK))
	ctx := context.Background()
	_, err := c.Messages.SendMedia(ctx, "1", "+1", MediaMessage{Type: MediaDocument, ID: "m1", Caption: "Invoice", Filename: "inv.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	b := decodeBody(t, f.last())
	doc := b["document"].(map[string]any)
	if b["type"] != "document" || doc["id"] != "m1" || doc["caption"] != "Invoice" || doc["filename"] != "inv.pdf" || doc["link"] != nil {
		t.Fatalf("%v", b)
	}
	_, _ = c.Messages.SendMedia(ctx, "1", "+1", MediaMessage{Type: MediaImage, Link: "https://x/y.png"})
	img := decodeBody(t, f.last())["image"].(map[string]any)
	if img["link"] != "https://x/y.png" || img["id"] != nil || img["caption"] != nil {
		t.Fatalf("%v", img)
	}
	for _, ty := range []MediaType{MediaVideo, MediaAudio, MediaSticker} {
		if _, err := c.Messages.SendMedia(ctx, "1", "+1", MediaMessage{Type: ty, ID: "x"}); err != nil {
			t.Fatal(ty, err)
		}
	}
	bad := []struct {
		to string
		m  MediaMessage
	}{
		{"+1", MediaMessage{Type: "gif", ID: "x"}},
		{"", MediaMessage{Type: MediaImage, ID: "x"}},
		{"+1", MediaMessage{Type: MediaImage}},
		{"+1", MediaMessage{Type: MediaImage, ID: "x", Link: "y"}},
	}
	for _, tc := range bad {
		if _, err := c.Messages.SendMedia(ctx, "1", tc.to, tc.m); err == nil {
			t.Fatalf("expected error for %+v", tc)
		}
	}
}

func TestMarkAsRead(t *testing.T) {
	f, c := newFake(t, jsonReply(200, `{"success":true}`))
	ctx := context.Background()
	if err := c.Messages.MarkAsRead(ctx, "1", "wamid.A"); err != nil {
		t.Fatal(err)
	}
	expect(t, f.last(), "POST", "/v1/1/messages")
	b := decodeBody(t, f.last())
	if b["status"] != "read" || b["message_id"] != "wamid.A" || b["messaging_product"] != "whatsapp" || b["typing_indicator"] != nil {
		t.Fatalf("%v", b)
	}
	if err := c.Messages.MarkAsRead(ctx, "1", "wamid.A", WithTypingIndicator()); err != nil {
		t.Fatal(err)
	}
	if decodeBody(t, f.last())["typing_indicator"].(map[string]any)["type"] != "text" {
		t.Fatal("typing indicator")
	}
	if c.Messages.MarkAsRead(ctx, "", "w") == nil || c.Messages.MarkAsRead(ctx, "1", "") == nil {
		t.Fatal("validation")
	}
	_, c = newFake(t, jsonReply(422, `{"message":"unknown message","code":"SEND_FAILED"}`))
	if c.Messages.MarkAsRead(ctx, "1", "w") == nil {
		t.Fatal("api error")
	}
}

func TestMessagesGet(t *testing.T) {
	f, c := newFake(t, jsonReply(200, `{"id":"u1","message_id":"wamid.A=","status":"delivered","message_type":"template","sent_at":"2026-11-01T10:00:00Z","content":{"to":"1"},"error_code":"131047"}`))
	got, err := c.Messages.Get(context.Background(), "wamid.A=/x")
	if err != nil || got.ID != "u1" || got.Status != "delivered" || got.SentAt == nil || got.SentAt.Year() != 2026 || got.ErrorCode != "131047" {
		t.Fatalf("%+v %v", got, err)
	}
	expect(t, f.last(), "GET", "/v1/messages/wamid.A=%2Fx")
	if _, err := c.Messages.Get(context.Background(), ""); err == nil {
		t.Fatal("empty id")
	}
	_, c = newFake(t, jsonReply(404, `{"message":"nope","code":"NOT_FOUND"}`))
	if _, err := c.Messages.Get(context.Background(), "x"); !IsNotFound(err) {
		t.Fatal(err)
	}
}
