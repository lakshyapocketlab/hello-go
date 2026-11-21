package hello

import (
	"context"
	"net/url"
	"testing"
)

func TestTemplatesList(t *testing.T) {
	f, c := newFake(t, jsonReply(200, `{"data":[{"id":"1","name":"order_shipped","language":"en","status":"APPROVED","category":"UTILITY","components":[{"type":"BODY"}]}],"paging":{"cursors":{"before":"b","after":"a"}},"summary":{"total_count":42,"status_counts":{"APPROVED":38}}}`))
	ctx := context.Background()
	got, err := c.Templates.List(ctx, "102", &ListTemplatesParams{
		Name: "n", Search: "s", Status: "APPROVED", Category: "UTILITY", Language: "en_US", Limit: 20,
		After: "a1", Before: "b1", Summary: "total_count", SortBy: "name", SortOrder: "asc",
	})
	if err != nil || got.Data[0].Name != "order_shipped" || got.Paging.Cursors.After != "a" || got.Summary.TotalCount != 42 || got.Summary.StatusCounts["APPROVED"] != 38 {
		t.Fatalf("%+v %v", got, err)
	}
	r := f.last()
	expect(t, r, "GET", "/v1/102/message_templates")
	q, _ := url.ParseQuery(r.Query)
	want := map[string]string{"name": "n", "search": "s", "status": "APPROVED", "category": "UTILITY", "language": "en_US",
		"limit": "20", "after": "a1", "before": "b1", "summary": "total_count", "sort_by": "name", "sort_order": "asc"}
	if len(q) != len(want) {
		t.Fatalf("query = %v", q)
	}
	for k, v := range want {
		if q.Get(k) != v {
			t.Fatalf("%s = %q", k, q.Get(k))
		}
	}

	_, _ = c.Templates.List(ctx, "102", nil)
	if f.last().Query != "" {
		t.Fatal("nil params")
	}
	_, _ = c.Templates.List(ctx, "102", &ListTemplatesParams{})
	if f.last().Query != "" {
		t.Fatal("zero params")
	}
	if _, err := c.Templates.List(ctx, "", nil); err == nil {
		t.Fatal("empty waba")
	}
	_, c = newFake(t, jsonReply(404, `{"error":{"message":"x"}}`))
	if _, err := c.Templates.List(ctx, "1", nil); !IsNotFound(err) {
		t.Fatal(err)
	}
}

func TestTemplatesGet(t *testing.T) {
	f, c := newFake(t, jsonReply(200, `{"id":"7","name":"n","language":"en","status":"PENDING","quality_score":{"score":"GREEN"}}`))
	got, err := c.Templates.Get(context.Background(), "7")
	if err != nil || got.ID != "7" || got.QualityScore["score"] != "GREEN" {
		t.Fatalf("%+v %v", got, err)
	}
	expect(t, f.last(), "GET", "/v1/templates/7")
	if _, err := c.Templates.Get(context.Background(), ""); err == nil {
		t.Fatal("empty id")
	}
	_, c = newFake(t, jsonReply(404, `{"message":"x"}`))
	if _, err := c.Templates.Get(context.Background(), "7"); !IsNotFound(err) {
		t.Fatal(err)
	}
}

func TestTemplatesCreate(t *testing.T) {
	f, c := newFake(t, jsonReply(200, `{"id":"1234","status":"PENDING","category":"UTILITY"}`))
	tpl := map[string]any{"name": "order_shipped", "language": "en", "category": "UTILITY"}
	got, err := c.Templates.Create(context.Background(), "102", tpl)
	if err != nil || got.ID != "1234" || got.Status != "PENDING" {
		t.Fatalf("%+v %v", got, err)
	}
	expect(t, f.last(), "POST", "/v1/102/message_templates")
	if decodeBody(t, f.last())["name"] != "order_shipped" {
		t.Fatal("body")
	}
	if _, err := c.Templates.Create(context.Background(), "", tpl); err == nil {
		t.Fatal("empty waba")
	}
	if _, err := c.Templates.Create(context.Background(), "1", nil); err == nil {
		t.Fatal("nil template")
	}
	_, c = newFake(t, jsonReply(422, `{"error":{"message":"rejected","code":100,"error_data":{"hello_code":"META_REJECTED"}}}`))
	if _, err := c.Templates.Create(context.Background(), "1", tpl); err == nil {
		t.Fatal("api error")
	}
}

func TestTemplatesCreateBulk(t *testing.T) {
	f, c := newFake(t, jsonReply(200, `{"total_rows":2,"created_count":1,"updated_count":1,"failed_count":0,"skipped_count":0,"results":[{"index":0,"template_name":"a","language":"en","status":"updated"},{"index":1,"template_name":"b","language":"en","status":"created"}]}`))
	got, err := c.Templates.CreateBulk(context.Background(), "102", []any{map[string]any{"name": "a"}, map[string]any{"name": "b"}})
	if err != nil || got.TotalRows != 2 || got.CreatedCount != 1 || got.Results[1].Status != "created" {
		t.Fatalf("%+v %v", got, err)
	}
	expect(t, f.last(), "POST", "/v1/102/templates/bulk")
	if rows := decodeBody(t, f.last())["templates"].([]any); len(rows) != 2 {
		t.Fatal("rows")
	}
	if _, err := c.Templates.CreateBulk(context.Background(), "", []any{1}); err == nil {
		t.Fatal("empty waba")
	}
	if _, err := c.Templates.CreateBulk(context.Background(), "1", nil); err == nil {
		t.Fatal("no rows")
	}
	_, c = newFake(t, jsonReply(400, `{"message":"x"}`))
	if _, err := c.Templates.CreateBulk(context.Background(), "1", []any{1}); err == nil {
		t.Fatal("api error")
	}
}

func TestTemplatesEditDeleteSync(t *testing.T) {
	f, c := newFake(t, jsonReply(200, `{"success":true}`))
	ctx := context.Background()

	if err := c.Templates.Edit(ctx, "7", map[string]any{"category": "UTILITY"}); err != nil {
		t.Fatal(err)
	}
	expect(t, f.last(), "POST", "/v1/templates/7")
	if decodeBody(t, f.last())["category"] != "UTILITY" {
		t.Fatal("edit body")
	}
	if c.Templates.Edit(ctx, "", map[string]any{}) == nil || c.Templates.Edit(ctx, "7", nil) == nil {
		t.Fatal("edit validation")
	}

	if err := c.Templates.Delete(ctx, "7"); err != nil {
		t.Fatal(err)
	}
	expect(t, f.last(), "DELETE", "/v1/templates/7")
	if c.Templates.Delete(ctx, "") == nil {
		t.Fatal("delete validation")
	}

	if err := c.Templates.DeleteByName(ctx, "102", "order shipped"); err != nil {
		t.Fatal(err)
	}
	expect(t, f.last(), "DELETE", "/v1/102/message_templates")
	if f.last().Query != "name=order+shipped" {
		t.Fatalf("query = %q", f.last().Query)
	}
	if c.Templates.DeleteByName(ctx, "", "n") == nil || c.Templates.DeleteByName(ctx, "1", "") == nil {
		t.Fatal("delete by name validation")
	}

	_, c = newFake(t, jsonReply(404, `{"message":"x"}`))
	if !IsNotFound(c.Templates.Edit(ctx, "7", map[string]any{"a": 1})) || !IsNotFound(c.Templates.Delete(ctx, "7")) {
		t.Fatal("not found")
	}
}

func TestTemplatesSync(t *testing.T) {
	f, c := newFake(t, jsonReply(200, `{"created":2,"updated":5,"deleted":1,"total":41}`))
	got, err := c.Templates.Sync(context.Background(), "102")
	if err != nil || got.Created != 2 || got.Updated != 5 || got.Deleted != 1 || got.Total != 41 {
		t.Fatalf("%+v %v", got, err)
	}
	expect(t, f.last(), "POST", "/v1/102/templates/sync")
	if _, err := c.Templates.Sync(context.Background(), ""); err == nil {
		t.Fatal("empty waba")
	}
	f, c = newFake(t, jsonReply(502, `{"message":"meta","code":"META_UNAVAILABLE"}`))
	if _, err := c.Templates.Sync(context.Background(), "1"); err == nil || f.count() != 3 {
		t.Fatalf("sync should retry: %d %v", f.count(), err)
	}
}
