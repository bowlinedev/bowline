package openapi

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bowlinedev/bowline/contract"
)

var update = flag.Bool("update", false, "rewrite golden files")

const fixtureAPIVersion = "1.0.0"

func fixtures(t *testing.T) []string {
	t.Helper()
	inputs, err := filepath.Glob(filepath.Join("..", "..", "analyzer", "testdata", "fidelity", "rows", "*", "expected.contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) == 0 {
		t.Fatal("no contracts found; run the analyzer fidelity suite first")
	}
	return inputs
}

func load(t *testing.T, path string) *contract.Document {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := contract.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestGoldens(t *testing.T) {
	for _, input := range fixtures(t) {
		row := filepath.Base(filepath.Dir(input))
		t.Run(row, func(t *testing.T) {
			got, err := Export(load(t, input), Info{Title: row, Version: fixtureAPIVersion, ServerURL: "http://localhost:8080/api"})
			if err != nil {
				t.Fatal(err)
			}
			var parsed map[string]any
			if err := json.Unmarshal(got, &parsed); err != nil {
				t.Fatalf("output is not JSON: %v", err)
			}
			if parsed["openapi"] != "3.1.0" {
				t.Fatalf("openapi version %v", parsed["openapi"])
			}
			golden := filepath.Join("testdata", row+".golden.json")
			if *update {
				os.MkdirAll("testdata", 0o755)
				if err := os.WriteFile(golden, got, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("%v\n%s", err, got)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("golden mismatch; run go test ./internal/export/openapi -update after review:\n%s", got)
			}
		})
	}
}

func TestSchemasAreDeterministic(t *testing.T) {
	for _, input := range fixtures(t) {
		doc := load(t, input)
		first, err := Export(doc, Info{})
		if err != nil {
			t.Fatal(err)
		}
		second, err := Export(doc, Info{})
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(first, second) {
			t.Fatalf("%s: two runs differ", input)
		}
	}
}

func TestErrorVariantsAreResponses(t *testing.T) {
	doc := load(t, filepath.Join("..", "..", "analyzer", "testdata", "fidelity", "rows", "errors", "expected.contract.json"))
	got, err := Export(doc, Info{})
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Paths map[string]map[string]struct {
			Responses map[string]json.RawMessage `json:"responses"`
		} `json:"paths"`
		Components struct {
			Responses map[string]json.RawMessage `json:"responses"`
			Schemas   map[string]json.RawMessage `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(got, &parsed); err != nil {
		t.Fatal(err)
	}
	if _, ok := parsed.Components.Responses["InvoiceLocked"]; !ok {
		t.Fatal("missing InvoiceLocked response")
	}
	if _, ok := parsed.Components.Schemas["InvoiceLocked"]; !ok {
		t.Fatal("missing InvoiceLocked details schema")
	}
	void := parsed.Paths["/void"]["post"].Responses
	if !bytes.Contains(void["412"], []byte("InvoiceLocked")) || !bytes.Contains(void["429"], []byte("QuotaExceeded")) || !bytes.Contains(void["400"], []byte("INVALID_ARGUMENT")) {
		t.Fatalf("void responses %v", void)
	}
}

func TestDefaultsAndServers(t *testing.T) {
	got, err := Export(&contract.Document{Bowline: contract.Version}, Info{})
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	json.Unmarshal(got, &parsed)
	info := parsed["info"].(map[string]any)
	if info["title"] != "API" || info["version"] != "0.0.0" {
		t.Fatalf("info %v", info)
	}
	if _, has := parsed["servers"]; has {
		t.Fatal("servers must be omitted without a URL")
	}
}

func TestAutoPatchOperationsAreDocumented(t *testing.T) {
	doc := &contract.Document{
		Bowline: contract.Version,
		Types:   map[string]*contract.TypeDecl{},
		Errors:  map[string]*contract.ErrorDecl{},
		Procedures: []*contract.Procedure{
			{Path: "things.get", Kind: "query", Method: "GET", HTTPPath: "things/{id}",
				Input:  &contract.Type{Kind: contract.Struct, Fields: []*contract.Field{{Name: "id", Type: &contract.Type{Kind: contract.Primitive, Name: "int64"}}}},
				Output: &contract.Type{Kind: contract.Struct}},
			{Path: "things.save", Kind: "mutation", Method: "PUT", HTTPPath: "things/{id}", Security: []string{"bearer"},
				Input:  &contract.Type{Kind: contract.Struct, Fields: []*contract.Field{{Name: "id", Type: &contract.Type{Kind: contract.Primitive, Name: "int64"}}}},
				Output: &contract.Type{Kind: contract.Struct}},
		},
		Security: map[string]*contract.SecurityScheme{"bearer": {Kind: "http", Scheme: "bearer"}},
	}
	raw, err := Export(doc, Info{Title: "t", Version: "1", AutoPatch: true, ETags: true})
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	path, ok := out["paths"].(map[string]any)["/things/{id}"].(map[string]any)
	if !ok {
		t.Fatalf("no /things/{id} path: %s", raw)
	}
	patch, ok := path["patch"].(map[string]any)
	if !ok {
		t.Fatalf("AutoPatch was configured but no patch operation was documented: %v", path)
	}
	body := patch["requestBody"].(map[string]any)["content"].(map[string]any)
	for _, media := range []string{"application/merge-patch+json", "application/json-patch+json"} {
		if _, ok := body[media]; !ok {
			t.Errorf("the patch body does not advertise %s", media)
		}
	}
	responses := patch["responses"].(map[string]any)
	for _, code := range []string{"412", "428", "415"} {
		if _, ok := responses[code]; !ok {
			t.Errorf("the patch operation does not document a %s response", code)
		}
	}
	if patch["security"] == nil {
		t.Error("the patch operation must inherit the write procedure's security")
	}
	if patch["parameters"] == nil {
		t.Error("the patch operation must carry the path parameters")
	}
}

func TestPatchIsAbsentWhenNotConfigured(t *testing.T) {
	doc := &contract.Document{
		Bowline: contract.Version, Types: map[string]*contract.TypeDecl{}, Errors: map[string]*contract.ErrorDecl{},
		Procedures: []*contract.Procedure{
			{Path: "things.get", Kind: "query", Method: "GET", HTTPPath: "things/{id}",
				Input: &contract.Type{Kind: contract.Struct}, Output: &contract.Type{Kind: contract.Struct}},
			{Path: "things.save", Kind: "mutation", Method: "PUT", HTTPPath: "things/{id}",
				Input: &contract.Type{Kind: contract.Struct}, Output: &contract.Type{Kind: contract.Struct}},
		},
	}
	raw, _ := Export(doc, Info{Title: "t", Version: "1"})
	if strings.Contains(string(raw), `"patch"`) {
		t.Fatal("a patch operation was documented without AutoPatch")
	}
}
