package mcpproxy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDispatchForwardsHeadersAndShapesRequests(t *testing.T) {
	type seen struct {
		Method, Path, Query, Auth, Tenant, Body string
	}
	var got seen
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got = seen{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization"), r.Header.Get("X-Tenant"), string(body)}
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()
	static := http.Header{"Authorization": {"Bearer static"}, "X-Tenant": {"acme"}}
	d := New(upstream.URL+"/api/", static, nil)
	status, body, err := d.Dispatch(context.Background(), "invoices.get", http.MethodGet, json.RawMessage(`{"id":3}`), http.Header{"Authorization": {"Bearer caller"}})
	if err != nil || status != http.StatusTeapot || string(body) != `{"ok":true}` {
		t.Fatalf("status %d body %s err %v", status, body, err)
	}
	if got.Method != "GET" || got.Path != "/api/invoices.get" || got.Query != "input=%7B%22id%22%3A3%7D" {
		t.Fatalf("request %+v", got)
	}
	if got.Auth != "Bearer caller" || got.Tenant != "acme" {
		t.Fatalf("headers %+v", got)
	}
	if _, _, err := d.Dispatch(context.Background(), "invoices.void", http.MethodPost, nil, nil); err != nil {
		t.Fatal(err)
	}
	if got.Method != "POST" || got.Body != "{}" || got.Auth != "Bearer static" {
		t.Fatalf("post %+v", got)
	}
}

func TestDispatchReportsConnectionFailures(t *testing.T) {
	upstream := httptest.NewServer(http.NotFoundHandler())
	upstream.Close()
	d := New(upstream.URL, nil, nil)
	if _, _, err := d.Dispatch(context.Background(), "x", http.MethodPost, nil, nil); err == nil {
		t.Fatal("expected an error")
	}
}
