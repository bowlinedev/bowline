package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bowlinedev/bowline/examples/ledger/api"
)

func TestMCPMountHonorsTheTokenMiddleware(t *testing.T) {
	t.Setenv("LEDGER_TOKEN", "dev")
	t.Setenv("LEDGER_FIXED_TIME", "2026-09-15T12:00:00Z")
	handler, err := newHandler(api.Routes(), false)
	if err != nil {
		t.Fatal(err)
	}
	call := func(body, auth string) string {
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
		return rec.Body.String()
	}
	list := call(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, "")
	for _, name := range []string{"customers_search", "invoices_get", "invoices_list", "invoices_void"} {
		if !strings.Contains(list, `"name":"`+name+`"`) {
			t.Fatalf("missing %s in %s", name, list)
		}
	}
	if strings.Contains(list, "invoices_create") || strings.Contains(list, "health") {
		t.Fatalf("unexposed procedure listed: %s", list)
	}
	get := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"invoices_get","arguments":{"id":3}}}`
	with := call(get, "Bearer dev")
	if strings.Contains(with, `"isError":true`) || !strings.Contains(with, `"createdAt":"2026-09-15T12:00:00Z"`) {
		t.Fatalf("with token: %s", with)
	}
	without := call(get, "")
	if !strings.Contains(without, `"isError":true`) || !strings.Contains(without, `"text":"UNAUTHENTICATED: a bearer token is required"`) {
		t.Fatalf("without token: %s", without)
	}
	open := call(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"customers_search","arguments":{"query":"ada"}}}`, "")
	if strings.Contains(open, `"isError":true`) || !strings.Contains(open, "Ada") {
		t.Fatalf("customers without token: %s", open)
	}
}
