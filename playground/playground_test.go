package playground

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func get(h http.Handler, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestPlaceholderWhenBundleAbsent(t *testing.T) {
	h := New([]byte(`{}`))
	for _, target := range []string{"/", "/index.html", "/some/client/route"} {
		rec := get(h, target)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), "pnpm --filter @bowline/playground build") {
			t.Fatalf("%s: %d %s", target, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "<title>Bowline playground</title>") {
			t.Fatalf("%s: default title missing", target)
		}
	}
	if rec := get(h, "/placeholder.html"); rec.Code != 200 || !strings.Contains(rec.Body.String(), "<html") {
		t.Fatalf("static asset: %d", rec.Code)
	}
}

func TestTitleReplacesToken(t *testing.T) {
	rec := get(New([]byte(`{}`), WithTitle("Ledger API")), "/")
	if !strings.Contains(rec.Body.String(), "<title>Ledger API</title>") || strings.Contains(rec.Body.String(), titleToken) {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestContractAndDynamicContract(t *testing.T) {
	rec := get(New([]byte(`{"bowline":"1.2"}`)), "/contract.json")
	if rec.Code != 200 || rec.Body.String() != `{"bowline":"1.2"}` || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("%d %s %s", rec.Code, rec.Body.String(), rec.Header().Get("Content-Type"))
	}
	var mu sync.Mutex
	current := []byte(`{"v":1}`)
	h := NewDynamic(func() []byte {
		mu.Lock()
		defer mu.Unlock()
		return current
	})
	if got := get(h, "/contract.json").Body.String(); got != `{"v":1}` {
		t.Fatalf("first %s", got)
	}
	mu.Lock()
	current = []byte(`{"v":2}`)
	mu.Unlock()
	if got := get(h, "/contract.json").Body.String(); got != `{"v":2}` {
		t.Fatalf("second %s", got)
	}
}

type seen struct {
	method, path, query, body string
	headers                   http.Header
}

func upstream(t *testing.T, record *seen) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		*record = seen{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, body: string(body), headers: r.Header.Clone()}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Deprecation", "true")
		w.Header().Set("Set-Cookie", "session=1")
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestProxyForwardsAllowlistedHeadersOnly(t *testing.T) {
	var record seen
	srv := upstream(t, &record)
	h := http.StripPrefix("/playground", New([]byte(`{}`), WithUpstream(srv.URL+"/api/"), WithHeaderAllowlist("Accept-Language")))
	req := httptest.NewRequest(http.MethodPost, "/playground/proxy/invoices.create?input=%7B%22id%22%3A3%7D", strings.NewReader(`{"id":3}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer dev")
	req.Header.Set("Idempotency-Key", "k1")
	req.Header.Set("X-Tenant", "acme")
	req.Header.Set("Accept-Language", "en")
	req.Header.Set("Cookie", "session=abc")
	req.Header.Set("Referer", "http://evil.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTeapot || rec.Body.String() != `{"ok":true}` {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Deprecation") != "true" || rec.Header().Get("Content-Type") != "application/json; charset=utf-8" || rec.Header().Get("Set-Cookie") != "" {
		t.Fatalf("response headers %v", rec.Header())
	}
	if record.method != http.MethodPost || record.path != "/api/invoices.create" || record.query != "input=%7B%22id%22%3A3%7D" || record.body != `{"id":3}` {
		t.Fatalf("request %+v", record)
	}
	for _, name := range []string{"Authorization", "Content-Type", "Idempotency-Key", "X-Tenant", "Accept-Language"} {
		if record.headers.Get(name) == "" {
			t.Fatalf("%s was not forwarded: %v", name, record.headers)
		}
	}
	for _, name := range []string{"Cookie", "Referer"} {
		if record.headers.Get(name) != "" {
			t.Fatalf("%s must be stripped: %v", name, record.headers)
		}
	}
}

func TestProxyResolvesSameOriginPath(t *testing.T) {
	var record seen
	srv := upstream(t, &record)
	h := New([]byte(`{}`), WithUpstream("/api"))
	req := httptest.NewRequest(http.MethodGet, "/proxy/health", nil)
	req.Host = strings.TrimPrefix(srv.URL, "http://")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTeapot || record.path != "/api/health" || record.method != http.MethodGet {
		t.Fatalf("%d %+v", rec.Code, record)
	}
}

func TestProxyWithoutUpstream(t *testing.T) {
	rec := get(New([]byte(`{}`)), "/proxy/health")
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), `"UNAVAILABLE"`) {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	down := httptest.NewServer(http.NotFoundHandler())
	down.Close()
	rec = get(New([]byte(`{}`), WithUpstream(down.URL)), "/proxy/health")
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "unreachable") {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}
