package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bowlinedev/bowline/contract"
	"github.com/bowlinedev/bowline/gateway"
)

func gatewayFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "gateway", "testdata", "compose", name+".contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func gatewayDocument(t *testing.T, name string) *contract.Document {
	t.Helper()
	doc, err := contract.Parse(gatewayFixture(t, name))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func gatewayHash(t *testing.T, name string) string {
	t.Helper()
	hash, err := gatewayDocument(t, name).ComputeHash()
	if err != nil {
		t.Fatal(err)
	}
	return hash
}

func gatewayProject(t *testing.T, listen string, upstreams map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	services := map[string]any{}
	for name, url := range upstreams {
		if err := os.WriteFile(filepath.Join(dir, name+".contract.json"), gatewayFixture(t, name), 0o644); err != nil {
			t.Fatal(err)
		}
		services[name] = map[string]any{
			"url":      url,
			"contract": name + ".contract.json",
			"version":  gatewayHash(t, name),
		}
	}
	cfg := map[string]any{"listen": listen, "prefix": "/api", "services": services}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, defaultGatewayConfig), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func upstreamEcho(t *testing.T, service string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/.bowline/health") {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"ok":true,"hash":%q}`, gatewayHash(t, service))
			return
		}
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"service":%q,"path":%q,"query":%q,"body":%q}`, service, r.URL.Path, r.URL.RawQuery, string(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestGatewayComposeWritesTheComposedDocument(t *testing.T) {
	ledger := upstreamEcho(t, "ledger")
	billing := upstreamEcho(t, "billing")
	dir := gatewayProject(t, "127.0.0.1:0", map[string]string{
		"ledger":  ledger.URL + "/api",
		"billing": billing.URL + "/api",
	})
	opts, out, errOut := testOptions(dir)
	if code := Gateway(opts, []string{"compose", "-o", "composed.contract.json"}); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "wrote composed.contract.json") {
		t.Fatalf("stdout %q", out.String())
	}
	got, err := os.ReadFile(filepath.Join(dir, "composed.contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	want, diags := gateway.Compose(map[string]*contract.Document{
		"ledger":  gatewayDocument(t, "ledger"),
		"billing": gatewayDocument(t, "billing"),
	})
	if len(diags) > 0 {
		t.Fatalf("compose diagnostics %v", diags)
	}
	wantData, err := want.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(wantData) {
		t.Fatalf("composed document differs from gateway.Compose\n%s", got)
	}
	doc, err := contract.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]bool{}
	for _, p := range doc.Procedures {
		paths[p.Path] = true
	}
	if !paths["ledger.invoices.get"] || !paths["billing.charges.create"] {
		t.Fatalf("composed paths %v", paths)
	}
}

func TestGatewayServesProxiesAndShutsDown(t *testing.T) {
	ledger := upstreamEcho(t, "ledger")
	billing := upstreamEcho(t, "billing")
	dir := gatewayProject(t, "127.0.0.1:0", map[string]string{
		"ledger":  ledger.URL + "/api",
		"billing": billing.URL + "/api",
	})
	opts, out, errOut := testOptions(dir)
	stop := make(chan struct{})
	ready := make(chan string, 1)
	opts.Stop = stop
	done := make(chan int, 1)
	go func() {
		done <- GatewayServe(GatewayOptions{Options: opts, Config: defaultGatewayConfig, Ready: ready})
	}()
	addr := <-ready
	base := "http://" + addr

	resp, err := http.Get(base + "/api/ledger.invoices.get?input=%7B%22id%22%3A3%7D")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), `"service":"ledger"`) || !strings.Contains(string(body), `"path":"/api/invoices.get"`) {
		t.Fatalf("proxied query: %d %s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), `"query":"input=%7B%22id%22%3A3%7D"`) {
		t.Fatalf("query string not preserved: %s", body)
	}

	resp, err = http.Post(base+"/api/billing.charges.create", "application/json", strings.NewReader(`{"invoiceId":3}`))
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), `"service":"billing"`) {
		t.Fatalf("proxied mutation: %d %s", resp.StatusCode, body)
	}

	resp, err = http.Get(base + "/api/.bowline/contract")
	if err != nil {
		t.Fatal(err)
	}
	composed, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(composed), "ledger.invoices.get") {
		t.Fatalf("composed contract: %d", resp.StatusCode)
	}

	resp, err = http.Get(base + "/api/.bowline/ready")
	if err != nil {
		t.Fatal(err)
	}
	readyBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(readyBody), `"ok":true`) {
		t.Fatalf("ready: %d %s", resp.StatusCode, readyBody)
	}

	close(stop)
	if code := <-done; code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "5 procedures from 2 service(s)") {
		t.Fatalf("banner %q", out.String())
	}
	if !strings.Contains(out.String(), "ledger       ready") || !strings.Contains(out.String(), "billing      ready") {
		t.Fatalf("readiness lines missing from %q", out.String())
	}
}

func TestGatewayRefusesAPinMismatchBeforeListening(t *testing.T) {
	ledger := upstreamEcho(t, "ledger")
	dir := gatewayProject(t, "127.0.0.1:0", map[string]string{"ledger": ledger.URL + "/api"})
	path := filepath.Join(dir, defaultGatewayConfig)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	wrong := strings.Replace(string(data), gatewayHash(t, "ledger"), "sha256:0000000000000000000000000000000000000000000000000000000000000000", 1)
	if err := os.WriteFile(path, []byte(wrong), 0o644); err != nil {
		t.Fatal(err)
	}
	opts, _, errOut := testOptions(dir)
	if code := Gateway(opts, nil); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(errOut.String(), "sha256:0000") || !strings.Contains(errOut.String(), gatewayHash(t, "ledger")) {
		t.Fatalf("stderr does not name both hashes: %q", errOut.String())
	}
}

func TestGatewayUsage(t *testing.T) {
	dir := gatewayProject(t, "127.0.0.1:0", map[string]string{"ledger": "http://127.0.0.1:1/api"})
	for _, args := range [][]string{
		{"--bogus"},
		{"compose"},
		{"compose", "--bogus"},
		{"extra"},
	} {
		opts, _, _ := testOptions(dir)
		if code := Gateway(opts, args); code != 2 {
			t.Fatalf("%v: exit %d", args, code)
		}
	}
	opts, _, errOut := testOptions(dir)
	if code := Gateway(opts, []string{"-c", "missing.json"}); code != 1 || !strings.Contains(errOut.String(), "missing.json") {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
}
