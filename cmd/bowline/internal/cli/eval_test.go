package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func evalUpstream(t *testing.T, total *atomic.Value) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer dev" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":{"code":"UNAUTHENTICATED","message":"a bearer token is required"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/invoices.get" && strings.Contains(r.URL.RawQuery, "999") {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":{"code":"NOT_FOUND","message":"invoice 999 not found"}}`))
			return
		}
		w.Write([]byte(`{"items":[{"id":3,"total":"` + total.Load().(string) + `","updatedAt":"now"}]}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestEvalRecordThenReplay(t *testing.T) {
	dir := ledgerProject(t)
	var total atomic.Value
	total.Store("USD 1500.00")
	upstream := evalUpstream(t, &total)
	os.WriteFile(filepath.Join(dir, "script.json"), []byte(`{"volatile":["updatedAt"],"calls":[{"id":"1","tool":"invoices_list","input":{"limit":2}},{"id":"2","tool":"invoices_get","input":{"id":999}}]}`), 0o644)
	opts, out, errOut := testOptions(dir)
	code := Eval(opts, []string{"record", "--url", upstream.URL, "--script", "script.json", "--out", "evals/run.json", "--header", "Authorization: Bearer dev"})
	if code != 0 || !strings.Contains(out.String(), "recorded 2 steps (1 errors) to evals/run.json") {
		t.Fatalf("exit %d out %q err %q", code, out.String(), errOut.String())
	}
	data, _ := os.ReadFile(filepath.Join(dir, "evals", "run.json"))
	if !strings.Contains(string(data), `"volatile": [`) || strings.Contains(string(data), `"updatedAt": "now"`) {
		t.Fatalf("recording %s", data)
	}
	opts, out, errOut = testOptions(dir)
	code = Eval(opts, []string{"replay", "evals/run.json", "--url", upstream.URL, "--header", "Authorization: Bearer dev"})
	if code != 0 || !strings.Contains(out.String(), "ok        2 steps match evals/run.json") {
		t.Fatalf("exit %d out %q err %q", code, out.String(), errOut.String())
	}
	total.Store("USD 1600.00")
	opts, _, errOut = testOptions(dir)
	code = Eval(opts, []string{"replay", "evals/run.json", "--url", upstream.URL, "--header", "Authorization: Bearer dev"})
	if code != 1 || !strings.Contains(errOut.String(), "/steps/0/output/items/0/total") || !strings.Contains(errOut.String(), "1 mismatch(es)") {
		t.Fatalf("exit %d err %q", code, errOut.String())
	}
	total.Store("USD 1500.00")
	opts, _, errOut = testOptions(dir)
	code = Eval(opts, []string{"replay", "evals/run.json", "--url", upstream.URL})
	if code != 1 || !strings.Contains(errOut.String(), "/steps/0/isError") {
		t.Fatalf("without token exit %d err %q", code, errOut.String())
	}
}

func TestEvalAgentModeReadsStdin(t *testing.T) {
	dir := ledgerProject(t)
	var total atomic.Value
	total.Store("USD 1500.00")
	upstream := evalUpstream(t, &total)
	opts, out, errOut := testOptions(dir)
	opts.Stdin = strings.NewReader("{\"id\":\"a\",\"tool\":\"invoices_list\",\"input\":{\"limit\":1},\"output\":{},\"error\":null,\"durationMs\":1}\n")
	code := Eval(opts, []string{"record", "--url", upstream.URL, "--agent", "--out", "run.json", "--volatile", "updatedAt", "--header", "Authorization: Bearer dev"})
	if code != 0 || !strings.Contains(out.String(), "recorded 1 steps") {
		t.Fatalf("exit %d out %q err %q", code, out.String(), errOut.String())
	}
	data, _ := os.ReadFile(filepath.Join(dir, "run.json"))
	if !strings.Contains(string(data), `"id": "a"`) || strings.Contains(string(data), `"updatedAt": "now"`) {
		t.Fatalf("recording %s", data)
	}
}

func TestEvalUsage(t *testing.T) {
	dir := ledgerProject(t)
	for _, args := range [][]string{nil, {"bogus"}, {"record", "--url", "http://x"}, {"replay"}, {"replay", "run.json"}, {"record", "--url", "http://x", "--script", "s.json", "--agent", "--out", "o.json"}} {
		opts, _, _ := testOptions(dir)
		if code := Eval(opts, args); code != 2 {
			t.Fatalf("%v: exit %d", args, code)
		}
	}
}
