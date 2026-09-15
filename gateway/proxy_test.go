package gateway

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bowlinedev/bowline/contract"
)

type capture struct {
	Method string
	Path   string
	Query  string
	Header http.Header
	Body   string
}

func upstreamServer(t *testing.T, seen *capture, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if seen != nil {
			*seen = capture{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Header: r.Header.Clone(), Body: string(body)}
		}
		if handler != nil {
			handler(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func testGateway(t *testing.T, cfg *Config, docs map[string]*contract.Document) http.Handler {
	t.Helper()
	g, err := New(cfg, docs)
	if err != nil {
		t.Fatal(err)
	}
	return g.Handler()
}

func twoServiceGateway(t *testing.T, ledgerURL, billingURL string) (http.Handler, *Config) {
	t.Helper()
	cfg := &Config{
		Prefix:         "/api",
		Timeout:        2 * time.Second,
		ForwardHeaders: append([]string(nil), DefaultForwardHeaders...),
		Services: map[string]Upstream{
			"ledger":  {URL: ledgerURL + "/api", Version: "sha256:x", Retries: DefaultRetries},
			"billing": {URL: billingURL + "/api", Version: "sha256:y", Retries: DefaultRetries},
		},
	}
	docs := map[string]*contract.Document{
		"ledger":  load(t, "ledger.contract.json"),
		"billing": load(t, "billing.contract.json"),
	}
	return testGateway(t, cfg, docs), cfg
}

func TestProxyRewritesPathsAndKeepsQueries(t *testing.T) {
	var seen capture
	ledger := upstreamServer(t, &seen, nil)
	billing := upstreamServer(t, nil, nil)
	handler, _ := twoServiceGateway(t, ledger.URL, billing.URL)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/ledger.invoices.get?input=%7B%22id%22%3A3%7D", nil))
	if rec.Code != 200 || rec.Body.String() != `{"ok":true}` {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if seen.Path != "/api/invoices.get" {
		t.Fatalf("upstream path %q", seen.Path)
	}
	if seen.Query != "input=%7B%22id%22%3A3%7D" {
		t.Fatalf("query %q", seen.Query)
	}
}

func TestProxyForwardsOnlyAllowlistedHeaders(t *testing.T) {
	var seen capture
	ledger := upstreamServer(t, &seen, nil)
	handler, _ := twoServiceGateway(t, ledger.URL, ledger.URL)

	req := httptest.NewRequest(http.MethodPost, "/api/billing.charges.create", strings.NewReader(`{"invoiceId":3}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Idempotency-Key", "abc")
	req.Header.Set("Bowline-Signature", "v1,t=1")
	req.Header.Set("Cookie", "session=1")
	req.Header.Set("X-Secret", "leak")
	req.Header.Set("Connection", "keep-alive")
	req.RemoteAddr = "203.0.113.7:4321"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	for _, name := range []string{"Authorization", "Idempotency-Key", "Bowline-Signature", "Cookie", "Content-Type"} {
		if seen.Header.Get(name) == "" {
			t.Fatalf("header %s was not forwarded", name)
		}
	}
	if seen.Header.Get("X-Secret") != "" {
		t.Fatal("a header outside the allowlist was forwarded")
	}
	if seen.Header.Get("Connection") == "keep-alive" {
		t.Fatal("hop-by-hop header forwarded")
	}
	if seen.Header.Get("X-Forwarded-For") != "203.0.113.7" {
		t.Fatalf("X-Forwarded-For %q", seen.Header.Get("X-Forwarded-For"))
	}
	if seen.Header.Get("X-Forwarded-Proto") != "http" || seen.Header.Get("X-Forwarded-Host") == "" {
		t.Fatalf("forwarded headers %v", seen.Header)
	}
	if seen.Body != `{"invoiceId":3}` {
		t.Fatalf("body %q", seen.Body)
	}
}

func TestProxyPassesResponseHeadersAndErrorBodies(t *testing.T) {
	ledger := upstreamServer(t, nil, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Deprecation", "true")
		w.Header().Set("Sunset", "Wed, 01 Jan 2027 00:00:00 GMT")
		w.Header().Set("ETag", `"abc"`)
		w.Header().Set("X-Internal", "hidden")
		w.WriteHeader(http.StatusPreconditionFailed)
		w.Write([]byte(`{"error":{"code":"FAILED_PRECONDITION","message":"locked","type":"InvoiceLocked"}}`))
	})
	handler, _ := twoServiceGateway(t, ledger.URL, ledger.URL)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/ledger.invoices.get", nil))
	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status %d", rec.Code)
	}
	if rec.Body.String() != `{"error":{"code":"FAILED_PRECONDITION","message":"locked","type":"InvoiceLocked"}}` {
		t.Fatalf("body %s", rec.Body.String())
	}
	for _, name := range []string{"Deprecation", "Sunset", "ETag"} {
		if rec.Header().Get(name) == "" {
			t.Fatalf("response header %s dropped", name)
		}
	}
	if rec.Header().Get("X-Internal") != "" {
		t.Fatal("unlisted response header passed through")
	}
}

func TestProxyUnknownProcedure(t *testing.T) {
	ledger := upstreamServer(t, nil, nil)
	handler, _ := twoServiceGateway(t, ledger.URL, ledger.URL)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/nope.thing", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d", rec.Code)
	}
	var env struct {
		Error struct{ Code, Message string } `json:"error"`
	}
	json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Error.Code != "UNIMPLEMENTED" || !strings.Contains(env.Error.Message, "nope.thing") {
		t.Fatalf("envelope %s", rec.Body.String())
	}
}

func TestProxyUnreachableUpstream(t *testing.T) {
	down := httptest.NewServer(http.NotFoundHandler())
	down.Close()
	handler, _ := twoServiceGateway(t, down.URL, down.URL)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/billing.charges.create", strings.NewReader("{}")))
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), `service \"billing\" is unavailable`) {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestProxyTimeout(t *testing.T) {
	slow := upstreamServer(t, nil, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	})
	cfg := &Config{
		Prefix:         "/api",
		Timeout:        80 * time.Millisecond,
		ForwardHeaders: append([]string(nil), DefaultForwardHeaders...),
		Services: map[string]Upstream{
			"billing": {URL: slow.URL + "/api", Version: "sha256:y"},
		},
	}
	handler := testGateway(t, cfg, map[string]*contract.Document{"billing": load(t, "billing.contract.json")})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/billing.charges.create", strings.NewReader("{}")))
	if rec.Code != http.StatusRequestTimeout || !strings.Contains(rec.Body.String(), "DEADLINE_EXCEEDED") {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestShouldRetryPolicy(t *testing.T) {
	cases := []struct {
		name    string
		method  string
		kind    string
		attempt int
		retries int
		status  int
		err     error
		want    bool
	}{
		{"get query on 503", http.MethodGet, "query", 0, 2, 503, nil, true},
		{"get query on 502", http.MethodGet, "query", 1, 2, 502, nil, true},
		{"get query on 504", http.MethodGet, "query", 0, 2, 504, nil, true},
		{"get query on connection error", http.MethodGet, "query", 0, 2, 0, io.ErrUnexpectedEOF, true},
		{"attempts exhausted", http.MethodGet, "query", 2, 2, 503, nil, false},
		{"retries disabled", http.MethodGet, "query", 0, 0, 503, nil, false},
		{"get query on 500", http.MethodGet, "query", 0, 2, 500, nil, false},
		{"get query on 200", http.MethodGet, "query", 0, 2, 200, nil, false},
		{"post mutation", http.MethodPost, "mutation", 0, 2, 503, nil, false},
		{"post query", http.MethodPost, "query", 0, 2, 503, nil, false},
		{"upload", http.MethodPost, "upload", 0, 2, 503, nil, false},
		{"subscription", http.MethodGet, "subscription", 0, 2, 503, nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := shouldRetry(c.method, c.kind, c.attempt, c.retries, c.status, c.err); got != c.want {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestProxyRetriesQueriesAndNotMutations(t *testing.T) {
	var queries, mutations atomic.Int32
	ledger := upstreamServer(t, nil, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "invoices.get") {
			if queries.Add(1) < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.Write([]byte(`{"recovered":true}`))
			return
		}
		mutations.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"error":{"code":"UNAVAILABLE","message":"nope"}}`))
	})
	handler, _ := twoServiceGateway(t, ledger.URL, ledger.URL)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/ledger.invoices.get", nil))
	if rec.Code != 200 || rec.Body.String() != `{"recovered":true}` {
		t.Fatalf("retried query: %d %s", rec.Code, rec.Body.String())
	}
	if queries.Load() != 3 {
		t.Fatalf("query attempts %d, want 3", queries.Load())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/billing.charges.create", strings.NewReader("{}")))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("mutation status %d", rec.Code)
	}
	if mutations.Load() != 1 {
		t.Fatalf("mutation attempts %d, want exactly 1", mutations.Load())
	}
}

func TestGatewayServesTheComposedContractAndHealth(t *testing.T) {
	ledger := upstreamServer(t, nil, nil)
	handler, _ := twoServiceGateway(t, ledger.URL, ledger.URL)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/.bowline/contract", nil))
	if rec.Code != 200 || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("contract %d %v", rec.Code, rec.Header())
	}
	doc, err := contract.Parse(rec.Body.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Procedures) != 5 {
		t.Fatalf("composed procedures %d", len(doc.Procedures))
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/.bowline/health", nil))
	var health struct {
		OK   bool   `json:"ok"`
		Hash string `json:"hash"`
	}
	json.Unmarshal(rec.Body.Bytes(), &health)
	if !health.OK || health.Hash != doc.Hash {
		t.Fatalf("health %s", rec.Body.String())
	}
}

func TestNewRejectsBadConfiguration(t *testing.T) {
	docs := map[string]*contract.Document{"ledger": load(t, "ledger.contract.json")}
	if _, err := New(nil, docs); err == nil {
		t.Fatal("expected an error for a nil config")
	}
	cfg := &Config{Prefix: "/api", Services: map[string]Upstream{"ledger": {URL: "://bad"}}}
	if _, err := New(cfg, docs); err == nil {
		t.Fatal("expected an error for an invalid url")
	}
	bad := map[string]*contract.Document{"led.ger": load(t, "ledger.contract.json")}
	if _, err := New(&Config{Prefix: "/api", Services: map[string]Upstream{"led.ger": {URL: "http://x"}}}, bad); err == nil || !strings.Contains(err.Error(), "procedure segment") {
		t.Fatalf("composition diagnostics: %v", err)
	}
}
