package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bowlinedev/bowline/contract"
	"github.com/bowlinedev/bowline/examples/ledger/api"
)

func TestReservedContractAndHealthEndpoints(t *testing.T) {
	handler, err := newHandler(api.Routes(), false)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/.bowline/contract", nil))
	if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), api.Contract) {
		t.Fatalf("contract endpoint: status %d", rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("cache control %q", cc)
	}
	doc, err := contract.Parse(api.Contract)
	if err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/.bowline/health", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), doc.Hash) {
		t.Fatalf("health endpoint: status %d body %s", rec.Code, rec.Body.String())
	}
}

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

func TestPlaygroundMountOutsideProduction(t *testing.T) {
	handler, err := newHandler(api.Routes(), false)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/playground/", "/playground/contract.json"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d", path, rec.Code)
		}
	}
	srv := httptest.NewServer(handler)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/playground/proxy/health")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"ok":true`) {
		t.Fatalf("proxy: %d %s", resp.StatusCode, body)
	}
	production, _ := newHandler(api.Routes(), true)
	rec := httptest.NewRecorder()
	production.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/playground/", nil))
	if rec.Code == http.StatusOK {
		t.Fatal("playground must not be mounted in production")
	}
}
