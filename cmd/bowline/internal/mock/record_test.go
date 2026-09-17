package mock

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecordThenReplay(t *testing.T) {
	doc := ledger(t)
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Deprecation", "true")
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.RawQuery, "999") {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":{"code":"NOT_FOUND","message":"invoice 999 not found"}}`))
			return
		}
		w.Write([]byte(`{"id":3,"echo":` + strings.TrimSpace(string(body)+"0") + `,"path":"` + r.URL.Path + `"}`))
	}))
	defer upstream.Close()
	u, _ := url.Parse(upstream.URL + "/api")
	dir := t.TempDir()
	rec := Recorder(doc, u, dir, nil)
	get := call(rec, http.MethodGet, "invoices.get", url.QueryEscape(`{"id":3}`))
	if get.Code != 200 || !strings.Contains(get.Body.String(), `"path":"/api/invoices.get"`) {
		t.Fatalf("proxied get: %d %s", get.Code, get.Body.String())
	}
	call(rec, http.MethodGet, "invoices.get", url.QueryEscape(`{ "id" : 3 }`))
	call(rec, http.MethodGet, "invoices.get", url.QueryEscape(`{"id":999}`))
	create := call(rec, http.MethodPost, "invoices.create", `{"customerId":1,"lines":[]}`)
	if create.Code != 200 {
		t.Fatalf("proxied create: %d %s", create.Code, create.Body.String())
	}
	if calls != 4 {
		t.Fatalf("upstream calls %d", calls)
	}
	var files []string
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if !d.IsDir() {
			rel, _ := filepath.Rel(dir, p)
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if len(files) != 3 {
		t.Fatalf("expected 3 fixtures (whitespace variant shares one), got %v", files)
	}
	for _, f := range files {
		if !strings.HasPrefix(f, "invoices.get/") && !strings.HasPrefix(f, "invoices.create/") {
			t.Fatalf("unexpected fixture path %s", f)
		}
	}
	_, hash, _ := CanonicalInput([]byte(`{"id":3}`))
	data, err := os.ReadFile(filepath.Join(dir, "invoices.get", hash+".json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"procedure": "invoices.get"`, `"method": "GET"`, `"status": 200`, `"Deprecation": "true"`, `"recordedAt"`} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("fixture lacks %s:\n%s", want, data)
		}
	}
	upstream.Close()
	replay := New(doc, Options{Fixtures: os.DirFS(dir), Strict: true})
	hit := call(replay, http.MethodGet, "invoices.get", url.QueryEscape(`{"id": 3}`))
	if hit.Code != 200 || hit.Header().Get("Deprecation") != "true" || !strings.Contains(hit.Body.String(), `"path":"/api/invoices.get"`) {
		t.Fatalf("replay: %d %s %v", hit.Code, hit.Body.String(), hit.Header())
	}
	missing := call(replay, http.MethodGet, "invoices.get", url.QueryEscape(`{"id":999}`))
	if missing.Code != 404 || !strings.Contains(missing.Body.String(), "NOT_FOUND") {
		t.Fatalf("replayed error: %d %s", missing.Code, missing.Body.String())
	}
	strict := call(replay, http.MethodGet, "invoices.get", url.QueryEscape(`{"id":5}`))
	if strict.Code != 404 || !strings.Contains(strict.Body.String(), "UNIMPLEMENTED") || !strings.Contains(strict.Body.String(), "no recorded fixture") {
		t.Fatalf("strict miss: %d %s", strict.Code, strict.Body.String())
	}
	lenient := New(doc, Options{Fixtures: os.DirFS(dir)})
	fallback := call(lenient, http.MethodGet, "invoices.get", url.QueryEscape(`{"id":5}`))
	if fallback.Code != 200 || !strings.Contains(fallback.Body.String(), `"id":5`) {
		t.Fatalf("lenient miss: %d %s", fallback.Code, fallback.Body.String())
	}
}

func TestCanonicalInput(t *testing.T) {
	a, ha, _ := CanonicalInput([]byte(`{ "b": 1, "a": [1, 2.50] }`))
	b, hb, _ := CanonicalInput([]byte(`{"a":[1,2.50],"b":1}`))
	if string(a) != `{"a":[1,2.50],"b":1}` || !bytes.Equal(a, b) || ha != hb {
		t.Fatalf("%s %s", a, b)
	}
	empty, _, _ := CanonicalInput(nil)
	if string(empty) != "{}" {
		t.Fatalf("empty %s", empty)
	}
	if _, _, err := CanonicalInput([]byte("{")); err == nil {
		t.Fatal("expected an error")
	}
}
