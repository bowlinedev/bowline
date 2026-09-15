package registry

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestUIEmbedsThePlaceholder(t *testing.T) {
	data, err := fs.ReadFile(UI(), "placeholder.html")
	if err != nil {
		t.Fatalf("reading the embedded placeholder: %v", err)
	}
	if !strings.Contains(string(data), "pnpm --filter @bowline/registry-ui build") {
		t.Fatalf("the placeholder does not carry the build instruction: %q", data)
	}
	if _, err := fs.Stat(UI(), "ui/dist"); err == nil {
		t.Fatal("UI() should be rooted at ui/dist, not above it")
	}
}

func TestServerServesThePlaceholderUntilTheBundleIsBuilt(t *testing.T) {
	if UIBuilt() {
		t.Skip("the bundle is built; the placeholder is not reachable")
	}
	server := NewServer(NewMemStore(), Options{UI: UI()})
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / answered %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "registry UI bundle is not built") {
		t.Fatalf("body %q", rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Fatalf("content type %q", got)
	}
}

func TestServerServesABuiltBundleAtTheRoot(t *testing.T) {
	assets := fstest.MapFS{
		"index.html":     {Data: []byte("<!doctype html><title>registry</title>")},
		"assets/main.js": {Data: []byte("export {}\n")},
	}
	server := NewServer(NewMemStore(), Options{UI: assets})
	handler := server.Handler()
	for path, want := range map[string]string{"/": "<title>registry</title>", "/assets/main.js": "export {}"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s answered %d", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("GET %s body %q", path, rec.Body.String())
		}
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/services", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "[]") {
		t.Fatalf("the API is shadowed by the UI: %d %q", rec.Code, rec.Body.String())
	}
}
