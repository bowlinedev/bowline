package mock

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/bowlinedev/bowline"
	"github.com/bowlinedev/bowline/contract"
)

func ledger(t *testing.T) *contract.Document {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "examples", "ledger", "api", "bowline.contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := contract.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func row(t *testing.T, name string) *contract.Document {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "analyzer", "testdata", "fidelity", "rows", name, "expected.contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := contract.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func call(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if method == http.MethodGet {
		query := ""
		if body != "" {
			query = "?input=" + body
		}
		req = httptest.NewRequest(method, "/"+path+query, nil)
	} else {
		req = httptest.NewRequest(method, "/"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("body %s: %v", rec.Body.String(), err)
	}
	return out
}

func TestCreateThenGetThenList(t *testing.T) {
	h := New(ledger(t), Options{Seed: 1})
	created := call(h, http.MethodPost, "invoices.create", `{"customerId":42,"lines":[{"description":"Consulting","quantity":3,"unitPrice":"USD 10.00"}],"note":"net 30"}`)
	if created.Code != 200 {
		t.Fatalf("create %d %s", created.Code, created.Body.String())
	}
	invoice := decode(t, created)
	if invoice["customerId"] != float64(42) || invoice["note"] != "net 30" {
		t.Fatalf("input fields not copied: %v", invoice)
	}
	id := invoice["id"]
	got := call(h, http.MethodGet, "invoices.get", fmt.Sprintf(`{"id":%v}`, id))
	if got.Code != 200 || decode(t, got)["customerId"] != float64(42) {
		t.Fatalf("get %d %s", got.Code, got.Body.String())
	}
	list := decode(t, call(h, http.MethodGet, "invoices?limit=10", ""))
	items, _ := list["items"].([]any)
	found := false
	for _, item := range items {
		if item.(map[string]any)["id"] == id {
			found = true
		}
	}
	if !found || len(items) < 1 {
		t.Fatalf("created invoice missing from list: %v", list)
	}
	unknown := call(h, http.MethodGet, "invoices.get", `{"id":777}`)
	if unknown.Code != 200 || decode(t, unknown)["id"] != float64(777) {
		t.Fatalf("unknown id should be generated with that id: %s", unknown.Body.String())
	}
	again := call(h, http.MethodGet, "invoices.get", `{"id":777}`)
	if again.Body.String() != unknown.Body.String() {
		t.Fatal("generated object was not stored")
	}
}

func TestMethodsAndSensitive(t *testing.T) {
	h := New(ledger(t), Options{})
	if rec := call(h, http.MethodGet, "customers.search", `{"query":"ada"}`); rec.Code != 405 || rec.Header().Get("Allow") != "POST" {
		t.Fatalf("sensitive query over GET: %d", rec.Code)
	}
	if rec := call(h, http.MethodPost, "customers.search", `{"query":"ada"}`); rec.Code != 200 {
		t.Fatalf("sensitive query over POST: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, "invoices.create", `{}`); rec.Code != 405 {
		t.Fatalf("mutation over GET: %d", rec.Code)
	}
	req := httptest.NewRequest(http.MethodPut, "/invoices.get", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 405 {
		t.Fatalf("PUT: %d", rec.Code)
	}
	if rec := call(h, http.MethodGet, "nope", `{}`); rec.Code != 404 || !strings.Contains(rec.Body.String(), `"UNIMPLEMENTED"`) {
		t.Fatalf("unknown: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodPost, "invoices.watch", `{}`); rec.Code != 404 || !strings.Contains(rec.Body.String(), "subscription") {
		t.Fatalf("subscription: %d %s", rec.Code, rec.Body.String())
	}
	bad := httptest.NewRequest(http.MethodPost, "/invoices.create", strings.NewReader("{}"))
	bad.Header.Set("Content-Type", "text/plain")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, bad)
	if rec.Code != 415 {
		t.Fatalf("content type: %d", rec.Code)
	}
}

type line struct {
	Description string `json:"description" validate:"required,max=200"`
	Quantity    int32  `json:"quantity" validate:"min=1"`
	UnitPrice   string `json:"unitPrice"`
}

type createInput struct {
	CustomerID int64   `json:"customerId" validate:"required"`
	Lines      []line  `json:"lines" validate:"required"`
	Note       *string `json:"note,omitempty" validate:"max=500"`
}

func TestValidationMatchesRuntimeByteForByte(t *testing.T) {
	mock := New(ledger(t), Options{})
	runtime := bowline.NewRouter(bowline.Mount("invoices", bowline.NewRouter(
		bowline.Mutation("create", func(ctx context.Context, in createInput) (struct{}, error) { return struct{}{}, nil }),
	))).Handler()
	for _, input := range []string{
		`{"lines":[{"description":"","quantity":0}]}`,
		`{"customerId":1,"lines":[]}`,
		`{"customerId":1,"lines":[{"description":"ok","quantity":2}],"note":"` + strings.Repeat("n", 501) + `"}`,
	} {
		a := call(mock, http.MethodPost, "invoices.create", input)
		b := call(runtime, http.MethodPost, "invoices.create", input)
		if a.Code != b.Code || a.Body.String() != b.Body.String() {
			t.Fatalf("input %s\nmock    %d %s\nruntime %d %s", input, a.Code, a.Body.String(), b.Code, b.Body.String())
		}
	}
}

func TestDeprecationHeaderAndGeneratedOutput(t *testing.T) {
	doc := row(t, "routing")
	h := New(doc, Options{Seed: 3})
	var deprecated *contract.Procedure
	for _, p := range doc.Procedures {
		if p.Deprecated != "" {
			deprecated = p
		}
	}
	if deprecated == nil {
		t.Fatal("routing row has no deprecated procedure")
	}
	rec := call(h, deprecated.Method, deprecated.Path, `{}`)
	if rec.Header().Get("Deprecation") != "true" {
		t.Fatalf("missing Deprecation header: %d %s", rec.Code, rec.Body.String())
	}
}

func TestConcurrentCreates(t *testing.T) {
	h := New(ledger(t), Options{})
	var wg sync.WaitGroup
	ids := make(chan any, 50)
	for range 50 {
		wg.Go(func() {
			rec := call(h, http.MethodPost, "invoices.create", `{"customerId":1,"lines":[{"description":"x","quantity":1,"unitPrice":"USD 1.00"}]}`)
			ids <- decode(t, rec)["id"]
		})
	}
	wg.Wait()
	close(ids)
	seen := map[any]bool{}
	for id := range ids {
		if seen[id] {
			t.Fatalf("duplicate id %v", id)
		}
		seen[id] = true
	}
	list := decode(t, call(h, http.MethodGet, "invoices?limit=100", ""))
	if items, _ := list["items"].([]any); len(items) != 50 {
		t.Fatalf("expected 50 stored invoices, got %d", len(items))
	}
}

func TestMutationWithIDUpdatesTheStoredObject(t *testing.T) {
	h := New(ledger(t), Options{Seed: 1})
	before := decode(t, call(h, http.MethodGet, "invoices.get", `{"id":3}`))
	voided := decode(t, call(h, http.MethodDelete, "invoices/3", ""))
	if voided["id"] != float64(3) || voided["customerId"] != before["customerId"] {
		t.Fatalf("void must return the stored invoice 3: %v vs %v", voided, before)
	}
	after := decode(t, call(h, http.MethodGet, "invoices.get", `{"id":3}`))
	if after["updatedAt"] != voided["updatedAt"] {
		t.Fatalf("stored object not updated: %v vs %v", after, voided)
	}
	list := decode(t, call(h, http.MethodGet, "invoices?limit=10", ""))
	items, _ := list["items"].([]any)
	count := 0
	for _, item := range items {
		if item.(map[string]any)["id"] == float64(3) {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("invoice 3 appears %d times after void", count)
	}
}

func TestRESTRoutes(t *testing.T) {
	h := New(ledger(t), Options{Seed: 1})
	got := decode(t, call(h, http.MethodGet, "invoices/3", ""))
	if got["id"] != float64(3) {
		t.Fatalf("GET invoices/3: %v", got)
	}
	if rec := call(h, http.MethodGet, "invoices?limit=2", ""); rec.Code != 200 {
		t.Fatalf("GET invoices?limit=2: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, "invoices?limit=notanumber", ""); rec.Code != 400 {
		t.Fatalf("the query string did not reach validation: %d %s", rec.Code, rec.Body.String())
	}
	rec := call(h, http.MethodPost, "invoices/3", "{}")
	if rec.Code != 405 || rec.Header().Get("Allow") == "" {
		t.Fatalf("POST invoices/3: %d %q", rec.Code, rec.Header().Get("Allow"))
	}
}
