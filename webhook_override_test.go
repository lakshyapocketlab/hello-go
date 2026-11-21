package hello

import (
	"context"
	"testing"
)

const overrideJSON = `{"enabled":true,"url":"https://h.example/x","status":"active","plan_allows":true,"meta_enabled":true,"meta_host":"h.example","in_sync":true}`

func TestWebhookOverrideGet(t *testing.T) {
	f, c := newFake(t, jsonReply(200, overrideJSON))
	got, err := c.WebhookOverride.Get(context.Background(), "106", false)
	if err != nil || !got.Enabled || got.Status != "active" || got.MetaHost != "h.example" || !got.InSync || !got.PlanAllows {
		t.Fatalf("%+v %v", got, err)
	}
	expect(t, f.last(), "GET", "/v1/106/webhook-override")
	if f.last().Query != "" {
		t.Fatal("no refresh by default")
	}
	_, _ = c.WebhookOverride.Get(context.Background(), "106", true)
	if f.last().Query != "refresh=true" {
		t.Fatalf("query = %q", f.last().Query)
	}
	if _, err := c.WebhookOverride.Get(context.Background(), "", false); err == nil {
		t.Fatal("empty id")
	}
	_, c = newFake(t, jsonReply(404, `{"message":"x"}`))
	if _, err := c.WebhookOverride.Get(context.Background(), "1", false); !IsNotFound(err) {
		t.Fatal(err)
	}
}

func TestWebhookOverrideSet(t *testing.T) {
	f, c := newFake(t, jsonReply(200, overrideJSON))
	ctx := context.Background()
	p := SetWebhookOverrideParams{URL: "https://h.example/x", VerifyToken: "tok12345", AcknowledgeNoTracking: true}
	got, err := c.WebhookOverride.Set(ctx, "106", p)
	if err != nil || got.URL != "https://h.example/x" {
		t.Fatalf("%+v %v", got, err)
	}
	expect(t, f.last(), "PUT", "/v1/106/webhook-override")
	b := decodeBody(t, f.last())
	if b["url"] != p.URL || b["verify_token"] != "tok12345" || b["acknowledge_no_tracking"] != true {
		t.Fatalf("%v", b)
	}
	if _, err := c.WebhookOverride.Set(ctx, "", p); err == nil {
		t.Fatal("empty id")
	}
	if _, err := c.WebhookOverride.Set(ctx, "1", SetWebhookOverrideParams{VerifyToken: "x"}); err == nil {
		t.Fatal("empty url")
	}
	if _, err := c.WebhookOverride.Set(ctx, "1", SetWebhookOverrideParams{URL: "x"}); err == nil {
		t.Fatal("empty token")
	}
	_, c = newFake(t, jsonReply(403, `{"message":"plan","code":"PLAN_FEATURE_NOT_INCLUDED"}`))
	if _, err := c.WebhookOverride.Set(ctx, "1", p); err == nil {
		t.Fatal("api error")
	}
}

func TestWebhookOverrideRemoveReapply(t *testing.T) {
	f, c := newFake(t, jsonReply(200, overrideJSON))
	ctx := context.Background()
	if err := c.WebhookOverride.Remove(ctx, "106"); err != nil {
		t.Fatal(err)
	}
	expect(t, f.last(), "DELETE", "/v1/106/webhook-override")
	if c.WebhookOverride.Remove(ctx, "") == nil {
		t.Fatal("empty id")
	}

	got, err := c.WebhookOverride.Reapply(ctx, "106")
	if err != nil || got.Status != "active" {
		t.Fatalf("%+v %v", got, err)
	}
	expect(t, f.last(), "POST", "/v1/106/webhook-override/reapply")
	if _, err := c.WebhookOverride.Reapply(ctx, ""); err == nil {
		t.Fatal("empty id")
	}
	f, c = newFake(t, jsonReply(503, `{"message":"down","code":"META_UNAVAILABLE"}`))
	if _, err := c.WebhookOverride.Reapply(ctx, "1"); err == nil || f.count() != 3 {
		t.Fatalf("reapply must retry: %d", f.count())
	}
}
