package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func hashOf(t *testing.T, name string) string {
	t.Helper()
	hash, err := load(t, name).ComputeHash()
	if err != nil {
		t.Fatal(err)
	}
	return hash
}

func copyFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "compose", name))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestResolveLoadsFilesAndVerifiesPins(t *testing.T) {
	cfg := &Config{Services: map[string]Upstream{
		"ledger": {URL: "http://ledger", Contract: copyFixture(t, "ledger.contract.json"), Version: hashOf(t, "ledger.contract.json")},
	}}
	docs, err := ResolveWith(context.Background(), cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if docs["ledger"].Hash != cfg.Services["ledger"].Version {
		t.Fatalf("hash %q", docs["ledger"].Hash)
	}
}

func TestResolveFetchesFromARegistry(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "compose", "billing.contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	var seen []string
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}))
	defer registry.Close()

	cfg := &Config{Services: map[string]Upstream{
		"billing": {URL: "http://billing", Registry: registry.URL, Version: hashOf(t, "billing.contract.json")},
	}}
	docs, err := Resolve(context.Background(), cfg, registry.Client())
	if err != nil {
		t.Fatal(err)
	}
	if len(docs["billing"].Procedures) != 1 {
		t.Fatalf("procedures %d", len(docs["billing"].Procedures))
	}
	if !strings.HasPrefix(seen[0], "/v1/services/billing/versions/sha256:") {
		t.Fatalf("requested %q", seen[0])
	}

	cfg.Services["billing"] = Upstream{URL: "http://billing", Registry: registry.URL, Version: "main"}
	if _, err := Resolve(context.Background(), cfg, registry.Client()); err != nil {
		t.Fatal(err)
	}
	if seen[1] != "/v1/services/billing/latest?tag=main" {
		t.Fatalf("tag request %q", seen[1])
	}
}

func TestResolveReportsEveryMismatchAtOnce(t *testing.T) {
	cfg := &Config{Services: map[string]Upstream{
		"ledger":  {URL: "http://ledger", Contract: copyFixture(t, "ledger.contract.json"), Version: "sha256:deadbeef"},
		"billing": {URL: "http://billing", Contract: copyFixture(t, "billing.contract.json"), Version: "sha256:feedface"},
	}}
	_, err := ResolveWith(context.Background(), cfg, nil)
	if err == nil {
		t.Fatal("expected a mismatch")
	}
	for _, want := range []string{"ledger", "billing", "sha256:deadbeef", "sha256:feedface", hashOf(t, "ledger.contract.json"), hashOf(t, "billing.contract.json")} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q lacks %q", err, want)
		}
	}
}

func TestResolveReportsUnreadableAndUnparsableSources(t *testing.T) {
	cfg := &Config{Services: map[string]Upstream{
		"ledger": {URL: "http://ledger", Contract: filepath.Join(t.TempDir(), "missing.json"), Version: "sha256:1"},
	}}
	if _, err := ResolveWith(context.Background(), cfg, nil); err == nil || !strings.Contains(err.Error(), "reading") {
		t.Fatalf("missing file: %v", err)
	}

	broken := filepath.Join(t.TempDir(), "broken.json")
	if err := os.WriteFile(broken, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg.Services["ledger"] = Upstream{URL: "http://ledger", Contract: broken, Version: "sha256:1"}
	if _, err := ResolveWith(context.Background(), cfg, nil); err == nil || !strings.Contains(err.Error(), "ledger") {
		t.Fatalf("broken document: %v", err)
	}

	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer down.Close()
	cfg.Services["ledger"] = Upstream{URL: "http://ledger", Registry: down.URL, Version: "sha256:1"}
	if _, err := Resolve(context.Background(), cfg, down.Client()); err == nil || !strings.Contains(err.Error(), "answered 500") {
		t.Fatalf("registry failure: %v", err)
	}
}
