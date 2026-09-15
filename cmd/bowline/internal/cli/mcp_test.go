package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bowlinedev/bowline/mcp"
)

func ledgerProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "examples", "ledger", "api", "bowline.contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "bowline.contract.json"), data, 0o644)
	os.WriteFile(filepath.Join(dir, "bowline.json"), []byte(`{"entry":"./api.Routes"}`), 0o644)
	return dir
}

func echoUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer dev" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":{"code":"UNAUTHENTICATED","message":"a bearer token is required"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":3,"path":"` + r.URL.Path + `","method":"` + r.Method + `"}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestMCPStdioSession(t *testing.T) {
	dir := ledgerProject(t)
	upstream := echoUpstream(t)
	opts, out, errOut := testOptions(dir)
	opts.Stdin = strings.NewReader(strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"invoices_get","arguments":{"id":3}}}`,
	}, "\n") + "\n")
	code := MCP(opts, []string{"--url", upstream.URL + "/api", "--header", "Authorization: Bearer dev", "--scope", "billing"})
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 responses, got %d: %s", len(lines), out.String())
	}
	if !strings.Contains(lines[0], `"protocolVersion":"2025-06-18"`) {
		t.Fatalf("initialize %s", lines[0])
	}
	var list struct {
		Result struct {
			Tools []struct{ Name string }
		}
	}
	json.Unmarshal([]byte(lines[1]), &list)
	names := []string{}
	for _, tool := range list.Result.Tools {
		names = append(names, tool.Name)
	}
	if strings.Join(names, ",") != "invoices_get,invoices_list,invoices_void" {
		t.Fatalf("tools %v", names)
	}
	var call struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				ID     int
				Path   string
				Method string
			} `json:"structuredContent"`
		}
	}
	json.Unmarshal([]byte(lines[2]), &call)
	if call.Result.IsError || call.Result.StructuredContent.ID != 3 || call.Result.StructuredContent.Path != "/api/invoices.get" || call.Result.StructuredContent.Method != "GET" {
		t.Fatalf("call %s", lines[2])
	}
}

func TestMCPListenForwardsCallerAuthorization(t *testing.T) {
	dir := ledgerProject(t)
	upstream := echoUpstream(t)
	settings, err := parseMCPFlags([]string{"--url", upstream.URL, "--listen", "127.0.0.1:0"}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	server, options, err := buildMCPServer(testOptionsOnly(dir), settings)
	if err != nil {
		t.Fatal(err)
	}
	h := mcp.Serve(server, options...)
	call := func(auth string) string {
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"invoices_get","arguments":{"id":3}}}`))
		req.Header.Set("Content-Type", "application/json")
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Body.String()
	}
	if got := call("Bearer dev"); strings.Contains(got, `"isError":true`) || !strings.Contains(got, `"id":3`) {
		t.Fatalf("with token: %s", got)
	}
	if got := call(""); !strings.Contains(got, `"isError":true`) || !strings.Contains(got, "UNAUTHENTICATED: a bearer token is required") {
		t.Fatalf("without token: %s", got)
	}
}

func TestMCPUpstreamDown(t *testing.T) {
	dir := ledgerProject(t)
	upstream := httptest.NewServer(http.NotFoundHandler())
	upstream.Close()
	opts, out, errOut := testOptions(dir)
	opts.Stdin = strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"invoices_list","arguments":{"limit":2}}}` + "\n")
	if code := MCP(opts, []string{"--url", upstream.URL}); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), `"isError":true`) || !strings.Contains(out.String(), "UNAVAILABLE") {
		t.Fatalf("got %s", out.String())
	}
}

func TestMCPUsageErrors(t *testing.T) {
	dir := ledgerProject(t)
	opts, _, errOut := testOptions(dir)
	if code := MCP(opts, nil); code != 2 || !strings.Contains(errOut.String(), "--url is required") {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	opts, _, errOut = testOptions(dir)
	if code := MCP(opts, []string{"--url", "http://x", "--bogus"}); code != 2 || !strings.Contains(errOut.String(), "bogus") {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	opts, _, errOut = testOptions(dir)
	if code := MCP(opts, []string{"--url", "http://x", "--header", "nocolon"}); code != 2 || !strings.Contains(errOut.String(), "Name: value") {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	os.WriteFile(filepath.Join(dir, "bowline.contract.json"), []byte(`{"bowline":"1.1","types":{},"errors":{},"procedures":[{"path":"a","kind":"query","method":"GET","input":{"kind":"struct"},"output":{"kind":"struct"},"tool":{"readOnly":true}}]}`), 0o644)
	opts, _, errOut = testOptions(dir)
	if code := MCP(opts, []string{"--url", "http://x"}); code != 1 || !strings.Contains(errOut.String(), "schemas") {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
}
