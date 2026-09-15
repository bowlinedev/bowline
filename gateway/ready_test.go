package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bowlinedev/bowline/contract"
	"github.com/bowlinedev/bowline/signing"
)

func healthServer(t *testing.T, hash string, hits *atomic.Int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/.bowline/health") {
			http.NotFound(w, r)
			return
		}
		if hits != nil {
			hits.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ok":true,"hash":%q}`, hash)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func readyGateway(t *testing.T, services map[string]Upstream, docs map[string]*contract.Document) *Gateway {
	t.Helper()
	cfg := &Config{Prefix: "/api", Timeout: time.Second, ForwardHeaders: append([]string(nil), DefaultForwardHeaders...), Services: services}
	g, err := New(cfg, docs)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestReadyReportsHealthyServices(t *testing.T) {
	pin := hashOf(t, "ledger.contract.json")
	ledger := healthServer(t, pin, nil)
	g := readyGateway(t,
		map[string]Upstream{"ledger": {URL: ledger.URL + "/api", Version: pin}},
		map[string]*contract.Document{"ledger": load(t, "ledger.contract.json")})

	for name, err := range g.Ready(context.Background()) {
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}

	rec := httptest.NewRecorder()
	g.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/.bowline/ready", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		OK       bool              `json:"ok"`
		Services map[string]string `json:"services"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if !body.OK || body.Services["ledger"] != "ok" {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestReadyReportsPinMismatchAndDownUpstreams(t *testing.T) {
	ledger := healthServer(t, "sha256:something-else", nil)
	down := httptest.NewServer(http.NotFoundHandler())
	down.Close()

	g := readyGateway(t,
		map[string]Upstream{
			"ledger":  {URL: ledger.URL + "/api", Version: hashOf(t, "ledger.contract.json")},
			"billing": {URL: down.URL + "/api", Version: hashOf(t, "billing.contract.json")},
		},
		map[string]*contract.Document{
			"ledger":  load(t, "ledger.contract.json"),
			"billing": load(t, "billing.contract.json"),
		})

	probes := g.Ready(context.Background())
	if probes["ledger"] == nil || !strings.Contains(probes["ledger"].Error(), "pinned to") {
		t.Fatalf("ledger probe %v", probes["ledger"])
	}
	if probes["billing"] == nil || !strings.Contains(probes["billing"].Error(), "unreachable") {
		t.Fatalf("billing probe %v", probes["billing"])
	}

	rec := httptest.NewRecorder()
	g.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/.bowline/ready", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "pinned to") || !strings.Contains(rec.Body.String(), "unreachable") {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestReadyCachesForFiveSeconds(t *testing.T) {
	var hits atomic.Int32
	pin := hashOf(t, "ledger.contract.json")
	ledger := healthServer(t, pin, &hits)
	g := readyGateway(t,
		map[string]Upstream{"ledger": {URL: ledger.URL + "/api", Version: pin}},
		map[string]*contract.Document{"ledger": load(t, "ledger.contract.json")})

	now := time.Now()
	g.nowFunc = func() time.Time { return now }
	g.Ready(context.Background())
	g.Ready(context.Background())
	g.Ready(context.Background())
	if hits.Load() != 1 {
		t.Fatalf("probed %d times, want 1", hits.Load())
	}

	now = now.Add(6 * time.Second)
	g.Ready(context.Background())
	if hits.Load() != 2 {
		t.Fatalf("probed %d times after the cache expired, want 2", hits.Load())
	}
}

func TestReadyProbesConcurrentlyWithABoundedTimeout(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
	}))
	defer slow.Close()

	g := readyGateway(t,
		map[string]Upstream{
			"ledger":  {URL: slow.URL + "/api", Version: "sha256:a"},
			"billing": {URL: slow.URL + "/api", Version: "sha256:b"},
		},
		map[string]*contract.Document{
			"ledger":  load(t, "ledger.contract.json"),
			"billing": load(t, "billing.contract.json"),
		})

	start := time.Now()
	probes := g.Ready(context.Background())
	elapsed := time.Since(start)
	if elapsed > 2*readyTimeout {
		t.Fatalf("probes took %s; they did not run concurrently", elapsed)
	}
	if probes["ledger"] == nil || probes["billing"] == nil {
		t.Fatalf("probes %v", probes)
	}
}

func TestProbeSignsWhenTheUpstreamDeclaresAKey(t *testing.T) {
	secret := []byte("s3cret")
	pin := hashOf(t, "ledger.contract.json")
	var verified error
	var seen bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = true
		verified = signing.Verify(r.Context(), signing.StaticSecrets{"edge": secret}, r.Header.Get(signing.Header), r.Method, r.URL.RequestURI(), nil, time.Now())
		if verified != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ok":true,"hash":%q}`, pin)
	}))
	defer upstream.Close()
	g := readyGateway(t,
		map[string]Upstream{"ledger": {URL: upstream.URL + "/api", Version: pin, Signing: &Signing{KeyID: "edge", Secret: secret}}},
		map[string]*contract.Document{"ledger": load(t, "ledger.contract.json")})
	if err := g.Ready(context.Background())["ledger"]; err != nil {
		t.Fatalf("ready: %v", err)
	}
	if !seen || verified != nil {
		t.Fatalf("probe was not signed: seen=%v verified=%v", seen, verified)
	}
}

func TestParseConfigReadsTheSigningSecretFromTheEnvironment(t *testing.T) {
	t.Setenv("LEDGER_SIGNING_SECRET", "from-env")
	cfg, err := ParseConfig([]byte(`{"services":{"ledger":{"url":"http://x/api","contract":"c.json","version":"sha256:abc","signing":{"keyId":"edge","secretEnv":"LEDGER_SIGNING_SECRET"}}}}`), "test.json")
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.Services["ledger"].Signing
	if got == nil || got.KeyID != "edge" || string(got.Secret) != "from-env" {
		t.Fatalf("got %+v", got)
	}
	for _, bad := range []string{
		`{"services":{"ledger":{"url":"http://x/api","contract":"c.json","version":"sha256:a","signing":{"secret":"s"}}}}`,
		`{"services":{"ledger":{"url":"http://x/api","contract":"c.json","version":"sha256:a","signing":{"keyId":"edge"}}}}`,
		`{"services":{"ledger":{"url":"http://x/api","contract":"c.json","version":"sha256:a","signing":{"keyId":"edge","secret":"s","secretEnv":"LEDGER_SIGNING_SECRET"}}}}`,
	} {
		if _, err := ParseConfig([]byte(bad), "test.json"); err == nil {
			t.Fatalf("expected a rejection for %s", bad)
		}
	}
}
