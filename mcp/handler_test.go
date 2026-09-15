package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bowlinedev/bowline"
)

func contractBytes(t *testing.T) []byte {
	t.Helper()
	data, err := json.Marshal(testDocument())
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func testHandler(t *testing.T, opts ...Option) http.Handler {
	t.Helper()
	h, err := Handler(testRouter(), contractBytes(t), opts...)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func post(h http.Handler, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func callBody(id int, tool, arguments string) string {
	return `{"jsonrpc":"2.0","id":` + itoa(id) + `,"method":"tools/call","params":{"name":"` + tool + `","arguments":` + arguments + `}}`
}

func itoa(n int) string {
	data, _ := json.Marshal(n)
	return string(data)
}

func decodeOne(t *testing.T, rec *httptest.ResponseRecorder) reply {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type %q", ct)
	}
	var r reply
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatalf("decode %s: %v", rec.Body.String(), err)
	}
	return r
}

func listNames(t *testing.T, h http.Handler) []string {
	t.Helper()
	r := decodeOne(t, post(h, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, nil))
	var result struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(r.Result, &result); err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(result.Tools))
	for _, tool := range result.Tools {
		names = append(names, tool.Name)
	}
	return names
}

func TestHandlerPostRoundTrip(t *testing.T) {
	h := testHandler(t)
	r := decodeOne(t, post(h, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`, nil))
	if r.Error != nil {
		t.Fatalf("initialize: %+v", r.Error)
	}
	rec := post(h, `{"jsonrpc":"2.0","method":"notifications/initialized"}`, nil)
	if rec.Code != http.StatusAccepted || rec.Body.Len() != 0 {
		t.Fatalf("notification: %d %q", rec.Code, rec.Body.String())
	}
	out := decodeCall(t, decodeOne(t, post(h, callBody(2, "invoices_get", `{"id":3}`), map[string]string{"Authorization": "Bearer x"})))
	if out.IsError || string(out.StructuredContent) != `{"id":3,"title":"Invoice 3"}` {
		t.Errorf("call = %+v", out)
	}
}

func TestHandlerRejectsGet(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rec := httptest.NewRecorder()
	testHandler(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "POST" {
		t.Fatalf("status %d allow %q", rec.Code, rec.Header().Get("Allow"))
	}
}

func TestHandlerBatch(t *testing.T) {
	h := testHandler(t)
	body := `[{"jsonrpc":"2.0","id":1,"method":"ping"},{"jsonrpc":"2.0","method":"notifications/initialized"},{"jsonrpc":"2.0","id":2,"method":"tools/list"}]`
	rec := post(h, body, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var replies []reply
	if err := json.Unmarshal(rec.Body.Bytes(), &replies); err != nil {
		t.Fatalf("decode %s: %v", rec.Body.String(), err)
	}
	if len(replies) != 2 || string(replies[0].ID) != "1" || string(replies[1].ID) != "2" {
		t.Errorf("replies = %+v", replies)
	}
	rec = post(h, `[{"jsonrpc":"2.0","method":"notifications/initialized"}]`, nil)
	if rec.Code != http.StatusAccepted {
		t.Errorf("all notifications: status %d", rec.Code)
	}
	r := decodeOne(t, post(h, `{"jsonrpc":"2.0","id":`, nil))
	if r.Error == nil || r.Error.Code != codeParse {
		t.Errorf("malformed = %+v", r.Error)
	}
}

func TestHandlerForwardsListedHeadersOnly(t *testing.T) {
	seen := http.Header{}
	capture := func(next bowline.Next) bowline.Next {
		return func(ctx context.Context, in any) (any, error) {
			seen = bowline.CallFrom(ctx).Request.Header.Clone()
			return next(ctx, in)
		}
	}
	r := testRouter()
	r.Use(capture)
	h, err := Handler(r, contractBytes(t), ForwardHeaders("Authorization", "X-Tenant"))
	if err != nil {
		t.Fatal(err)
	}
	headers := map[string]string{"Authorization": "Bearer x", "X-Tenant": "acme", "Cookie": "session=1", "X-Other": "no"}
	out := decodeCall(t, decodeOne(t, post(h, callBody(1, "invoices_get", `{"id":1}`), headers)))
	if out.IsError {
		t.Fatalf("call failed: %s", out.Content[0].Text)
	}
	if seen.Get("Authorization") != "Bearer x" || seen.Get("X-Tenant") != "acme" {
		t.Errorf("listed headers not forwarded: %v", seen)
	}
	if seen.Get("Cookie") != "" || seen.Get("X-Other") != "" {
		t.Errorf("unlisted headers forwarded: %v", seen)
	}
	r2 := testRouter()
	r2.Use(capture)
	h, err = Handler(r2, contractBytes(t))
	if err != nil {
		t.Fatal(err)
	}
	decodeCall(t, decodeOne(t, post(h, callBody(1, "invoices_get", `{"id":1}`), headers)))
	if seen.Get("Cookie") != "session=1" || seen.Get("X-Tenant") != "" {
		t.Errorf("default forwarding = %v", seen)
	}
}

func TestHandlerScopesAndReadOnly(t *testing.T) {
	names := listNames(t, testHandler(t, Scopes("invoices:read")))
	if len(names) != 1 || names[0] != "invoices_get" {
		t.Errorf("scoped = %v", names)
	}
	names = listNames(t, testHandler(t, ReadOnly()))
	if len(names) != 1 || names[0] != "invoices_get" {
		t.Errorf("read-only = %v", names)
	}
	names = listNames(t, testHandler(t, Scopes("invoices:write"), ReadOnly()))
	if len(names) != 0 {
		t.Errorf("write scope with read-only = %v", names)
	}
	h := testHandler(t, ReadOnly())
	r := decodeOne(t, post(h, callBody(1, "invoices_create", `{"title":"Rent"}`), map[string]string{"Authorization": "Bearer x"}))
	if r.Error == nil || r.Error.Code != codeInvalidParams {
		t.Errorf("hidden tool call = %+v", r.Error)
	}
}

func TestHandlerRateLimit(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	h := testHandler(t, RateLimit(60, 2), withClock(func() time.Time { return now }))
	auth := map[string]string{"Authorization": "Bearer a"}
	for i := 0; i < 2; i++ {
		out := decodeCall(t, decodeOne(t, post(h, callBody(i, "invoices_get", `{"id":1}`), auth)))
		if out.IsError {
			t.Fatalf("call %d within burst failed: %s", i, out.Content[0].Text)
		}
	}
	out := decodeCall(t, decodeOne(t, post(h, callBody(3, "invoices_get", `{"id":1}`), auth)))
	if !out.IsError || out.Content[0].Text != "RESOURCE_EXHAUSTED: rate limit exceeded" {
		t.Fatalf("exhausted = %+v", out)
	}
	out = decodeCall(t, decodeOne(t, post(h, callBody(4, "invoices_get", `{"id":1}`), map[string]string{"Authorization": "Bearer b"})))
	if out.IsError {
		t.Errorf("separate client limited: %s", out.Content[0].Text)
	}
	now = now.Add(time.Second)
	out = decodeCall(t, decodeOne(t, post(h, callBody(5, "invoices_get", `{"id":1}`), auth)))
	if out.IsError {
		t.Errorf("refill did not allow call: %s", out.Content[0].Text)
	}
}

func TestHandlerRejectsContractWithoutSchemas(t *testing.T) {
	doc := testDocument()
	for _, p := range doc.Procedures {
		p.Schemas = nil
	}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Handler(testRouter(), data); err == nil {
		t.Fatal("expected error")
	}
	if _, err := Handler(testRouter(), []byte(`{"bowline":"9.0"}`)); err == nil {
		t.Fatal("expected version error")
	}
}
