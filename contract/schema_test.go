package contract

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files under spec/")

func TestSchemaMatchesGolden(t *testing.T) {
	got, err := Schema()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("..", "spec", "contract.schema.json")
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update to create it)", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("spec/contract.schema.json is stale; run: go test ./contract -update")
	}
}

func TestSchemaShape(t *testing.T) {
	data, err := Schema()
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Schema string `json:"$schema"`
		ID     string `json:"$id"`
		Defs   map[string]struct {
			Properties map[string]json.RawMessage `json:"properties"`
			Required   []string                   `json:"required"`
		} `json:"$defs"`
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}
	if s.Schema != "https://json-schema.org/draft/2020-12/schema" {
		t.Fatalf("unexpected $schema %q", s.Schema)
	}
	if s.ID != "https://bowline.dev/spec/contract/1.2/contract.schema.json" {
		t.Fatalf("unexpected $id %q", s.ID)
	}
	for _, name := range []string{"TypeDecl", "Type", "Field", "Rule", "EnumValue", "ErrorDecl", "Tool", "Schemas", "Procedure", "Position"} {
		if _, ok := s.Defs[name]; !ok {
			t.Fatalf("missing $defs.%s", name)
		}
	}
	if _, ok := s.Properties["types"]; !ok {
		t.Fatal("missing top-level types property")
	}
	if got := s.Defs["Type"].Required; len(got) != 1 || got[0] != "kind" {
		t.Fatalf("Type.required = %v, want [kind]", got)
	}
	kind := string(s.Defs["Type"].Properties["kind"])
	for _, k := range []string{"primitive", "ref", "array", "map", "struct", "enum", "generic", "param"} {
		if !bytes.Contains([]byte(kind), []byte(`"`+k+`"`)) {
			t.Fatalf("kind enum missing %q in %s", k, kind)
		}
	}
}
