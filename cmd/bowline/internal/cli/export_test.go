package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportOpenAPIWritesDocument(t *testing.T) {
	dir := fixtureCopy(t)
	os.WriteFile(filepath.Join(dir, "bowline.json"), []byte(`{"entry":"./rows/routing.Routes","contract":"api/contract.json","openapi":{"title":"Routing","version":"2.0.0","serverUrl":"http://localhost:8080/api"}}`), 0o644)
	opts, out, errOut := testOptions(dir)
	if code := Export(opts, []string{"openapi"}); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "wrote api/openapi.json") {
		t.Fatalf("stdout %q", out.String())
	}
	data, err := os.ReadFile(filepath.Join(dir, "api", "openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		OpenAPI string            `json:"openapi"`
		Info    map[string]string `json:"info"`
		Servers []map[string]any  `json:"servers"`
		Paths   map[string]any    `json:"paths"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.OpenAPI != "3.1.0" || parsed.Info["title"] != "Routing" || parsed.Info["version"] != "2.0.0" || parsed.Servers[0]["url"] != "http://localhost:8080/api" {
		t.Fatalf("%+v", parsed)
	}
	for _, p := range []string{"/get", "/search", "/admin.purge", "/sub.remove"} {
		if _, ok := parsed.Paths[p]; !ok {
			t.Fatalf("missing path %s", p)
		}
	}
	opts, out, errOut = testOptions(dir)
	if code := Export(opts, []string{"openapi", "-o", "docs/api.json"}); code != 0 || !strings.Contains(out.String(), "wrote docs/api.json") {
		t.Fatalf("exit %d out %q err %q", code, out.String(), errOut.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "docs", "api.json")); err != nil {
		t.Fatal(err)
	}
}

func TestExportRequiresFormat(t *testing.T) {
	opts, _, errOut := testOptions(t.TempDir())
	if code := Export(opts, nil); code != 2 || !strings.Contains(errOut.String(), "usage") {
		t.Fatalf("exit %d err %q", code, errOut.String())
	}
}
