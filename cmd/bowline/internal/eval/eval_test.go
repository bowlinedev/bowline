package eval

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bowlinedev/bowline/cmd/bowline/internal/mcpproxy"
	"github.com/bowlinedev/bowline/cmd/bowline/internal/tools"
	"github.com/bowlinedev/bowline/contract"
)

type upstream struct {
	mu        sync.Mutex
	total     string
	updatedAt string
	code      string
	calls     int
}

func (u *upstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.calls++
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/invoices.list":
		w.Write([]byte(`{"items":[{"id":3,"total":"` + u.total + `","updatedAt":"` + u.updatedAt + `","amount":1500.50}],"nextCursor":"3"}`))
	case "/invoices.get":
		if strings.Contains(r.URL.RawQuery, "999") {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":{"code":"` + u.code + `","message":"invoice 999 not found at ` + u.updatedAt + `"}}`))
			return
		}
		w.Write([]byte(`{"id":3,"total":"` + u.total + `"}`))
	default:
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":{"code":"NOT_FOUND","message":"no such procedure"}}`))
	}
}

func ledgerRunner(t *testing.T, url string) *Runner {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "examples", "ledger", "api", "bowline.contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := contract.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	list, err := tools.FromContract(doc, tools.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	return &Runner{Dispatcher: mcpproxy.New(url, nil, nil), Tools: list, Contract: doc.Hash}
}

var script = []Call{
	{ID: "list", Tool: "invoices_list", Input: json.RawMessage(`{"limit": 2}`)},
	{ID: "get", Tool: "invoices_get", Input: json.RawMessage(`{"id":3}`)},
	{ID: "missing", Tool: "invoices_get", Input: json.RawMessage(`{"id":999}`)},
}

func TestRecordProducesCanonicalDocument(t *testing.T) {
	up := &upstream{total: "USD 1500.00", updatedAt: "2026-09-15T12:00:00Z", code: "NOT_FOUND"}
	srv := httptest.NewServer(up)
	defer srv.Close()
	runner := ledgerRunner(t, srv.URL)
	fixed := func() time.Time { return time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC) }
	rec, err := runner.Record(context.Background(), script, []string{"updatedAt"}, fixed)
	if err != nil {
		t.Fatal(err)
	}
	data, err := Encode(rec)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{`"eval": 1`, `"contract": "sha256:`, `"recordedAt": "2026-09-15T12:00:00Z"`, `"volatile": [`, "\"input\": {\n        \"limit\": 2\n      }", "\"amount\": 1500.5,\n            \"id\": 3,\n            \"total\": \"USD 1500.00\"", `"nextCursor": "3"`, `"code": "NOT_FOUND"`, `"isError": true`} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %s in\n%s", want, text)
		}
	}
	if strings.Contains(text, `"updatedAt": "`) {
		t.Fatalf("volatile key recorded:\n%s", text)
	}
	if rec.Steps[2].Output != nil || rec.Steps[0].Error != nil {
		t.Fatalf("steps %+v", rec.Steps)
	}
	parsed, err := Parse(data)
	if err != nil || len(parsed.Steps) != 3 {
		t.Fatalf("parse %v %+v", err, parsed)
	}
}

func TestReplayDetectsChanges(t *testing.T) {
	up := &upstream{total: "USD 1500.00", updatedAt: "2026-09-15T12:00:00Z", code: "NOT_FOUND"}
	srv := httptest.NewServer(up)
	defer srv.Close()
	runner := ledgerRunner(t, srv.URL)
	rec, err := runner.Record(context.Background(), script, []string{"updatedAt"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	same, err := runner.Replay(context.Background(), rec, false)
	if err != nil || len(same) != 0 {
		t.Fatalf("identical replay: %v %v", err, same)
	}
	up.mu.Lock()
	up.updatedAt = "2027-01-01T00:00:00Z"
	up.mu.Unlock()
	volatile, err := runner.Replay(context.Background(), rec, false)
	if err != nil || len(volatile) != 0 {
		t.Fatalf("volatile change: %v %v", err, volatile)
	}
	strict, err := runner.Replay(context.Background(), rec, true)
	if err != nil || len(strict) != 1 || strict[0].Path != "/steps/2/error/message" {
		t.Fatalf("strict messages: %v %v", err, strict)
	}
	up.mu.Lock()
	up.total = "USD 1600.00"
	up.mu.Unlock()
	changed, err := runner.Replay(context.Background(), rec, false)
	if err != nil || len(changed) != 2 {
		t.Fatalf("changed total: %v %v", err, changed)
	}
	if changed[0].Path != "/steps/0/output/items/0/total" || changed[0].Want != `"USD 1500.00"` || changed[0].Got != `"USD 1600.00"` || changed[1].Path != "/steps/1/output/total" {
		t.Fatalf("mismatches %v", changed)
	}
	up.mu.Lock()
	up.total = "USD 1500.00"
	up.code = "PERMISSION_DENIED"
	up.mu.Unlock()
	codes, err := runner.Replay(context.Background(), rec, false)
	if err != nil || len(codes) != 1 || codes[0].Path != "/steps/2/error/code" || codes[0].Got != "PERMISSION_DENIED" {
		t.Fatalf("changed code: %v %v", err, codes)
	}
}

func TestReplayRefusesOtherContracts(t *testing.T) {
	up := &upstream{total: "USD 1500.00", code: "NOT_FOUND"}
	srv := httptest.NewServer(up)
	defer srv.Close()
	runner := ledgerRunner(t, srv.URL)
	rec, _ := runner.Record(context.Background(), script[:1], nil, nil)
	rec.Contract = "sha256:other"
	before := up.calls
	_, err := runner.Replay(context.Background(), rec, false)
	if err == nil || !strings.Contains(err.Error(), "sha256:other") || !strings.Contains(err.Error(), runner.Contract) {
		t.Fatalf("got %v", err)
	}
	if up.calls != before {
		t.Fatal("replay called upstream despite the hash mismatch")
	}
}

func TestRecordMapsConnectionFailures(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	runner := ledgerRunner(t, srv.URL)
	rec, err := runner.Record(context.Background(), script[:1], nil, nil)
	if err != nil || !rec.Steps[0].IsError || rec.Steps[0].Error.Code != "UNAVAILABLE" {
		t.Fatalf("%v %+v", err, rec)
	}
	if _, err := runner.Record(context.Background(), []Call{{Tool: "nope"}}, nil, nil); err == nil || !strings.Contains(err.Error(), "unknown tool") {
		t.Fatalf("got %v", err)
	}
}

func TestNormalize(t *testing.T) {
	got := Normalize(json.RawMessage(`{"z":1.0,"a":{"createdAt":"x","n":12345678901234567890,"k":[{"createdAt":1},2.50]},"createdAt":"y"}`), []string{"createdAt"})
	want := `{"a":{"k":[{},2.5],"n":12345678901234567890},"z":1}`
	if string(got) != want {
		t.Fatalf("got %s", got)
	}
	if Normalize(nil, nil) != nil || string(Normalize(json.RawMessage("not json"), nil)) != "not json" {
		t.Fatal("edge cases")
	}
}

func TestParseCalls(t *testing.T) {
	calls, err := ParseCalls([]byte("{\"id\":\"1\",\"tool\":\"invoices_list\",\"input\":{\"limit\":2},\"output\":{},\"error\":null,\"durationMs\":3}\n{\"tool\":\"invoices_get\",\"input\":{\"id\":3}}\n"))
	if err != nil || len(calls) != 2 || calls[0].ID != "1" || calls[1].Tool != "invoices_get" {
		t.Fatalf("%v %+v", err, calls)
	}
	if _, err := ParseCalls([]byte("{\"input\":{}}\n")); err == nil {
		t.Fatal("expected an error for a call without a tool")
	}
}
