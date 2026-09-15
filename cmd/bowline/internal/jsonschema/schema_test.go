package jsonschema

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bowlinedev/bowline/contract"
)

var update = flag.Bool("update", false, "rewrite golden files")

func TestGoldens(t *testing.T) {
	inputs, err := filepath.Glob(filepath.Join("..", "analyzer", "testdata", "fidelity", "rows", "*", "expected.contract.json"))
	if err != nil || len(inputs) == 0 {
		t.Fatalf("no contracts: %v", err)
	}
	for _, input := range inputs {
		row := filepath.Base(filepath.Dir(input))
		t.Run(row, func(t *testing.T) {
			data, _ := os.ReadFile(input)
			doc, err := contract.Parse(data)
			if err != nil {
				t.Fatal(err)
			}
			out := map[string]map[string]json.RawMessage{}
			for _, p := range doc.Procedures {
				in, o, err := Procedure(doc, p)
				if err != nil {
					t.Fatal(err)
				}
				out[p.Path] = map[string]json.RawMessage{"input": in, "output": o}
				check(t, in)
				check(t, o)
			}
			got, _ := json.MarshalIndent(out, "", "  ")
			got = append(got, '\n')
			golden := filepath.Join("testdata", row+".golden.json")
			if *update {
				os.WriteFile(golden, got, 0o644)
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("%v\n%s", err, got)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("golden mismatch for %s; run go test ./internal/jsonschema -update after review", row)
			}
		})
	}
}

func check(t *testing.T, raw json.RawMessage) {
	t.Helper()
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	defs, _ := schema["$defs"].(map[string]any)
	var walk func(path string, n any)
	walk = func(path string, n any) {
		switch v := n.(type) {
		case map[string]any:
			if ref, ok := v["$ref"].(string); ok {
				name := strings.TrimPrefix(ref, "#/$defs/")
				if _, exists := defs[name]; !exists {
					t.Errorf("%s: dangling $ref %s", path, ref)
				}
			}
			if req, ok := v["required"].([]any); ok {
				props, _ := v["properties"].(map[string]any)
				for _, r := range req {
					if _, exists := props[r.(string)]; !exists {
						t.Errorf("%s: required %v not in properties", path, r)
					}
				}
			}
			if enum, ok := v["enum"].([]any); ok {
				typ, _ := v["type"].(string)
				for _, e := range enum {
					_, isString := e.(string)
					_, isNumber := e.(float64)
					if (typ == "string" && !isString) || (typ == "integer" && !isNumber) {
						t.Errorf("%s: enum value %v does not match type %s", path, e, typ)
					}
				}
			}
			for k, child := range v {
				walk(path+"/"+k, child)
			}
		case []any:
			for i, child := range v {
				walk(fmt.Sprintf("%s/%d", path, i), child)
			}
		}
	}
	walk("", schema)
}

func TestGenericInstantiationsAreDistinct(t *testing.T) {
	data, _ := os.ReadFile(filepath.Join("..", "analyzer", "testdata", "fidelity", "rows", "generics", "expected.contract.json"))
	doc, err := contract.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	_, out, err := Procedure(doc, doc.Procedures[0])
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{`"Page_User"`, `"Page_Page_User"`, `"Tree_User"`, `"Pair_string_User"`, `"Range_int64"`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing def %s", want)
		}
	}
	if strings.Contains(s, `"param"`) {
		t.Error("param nodes remain in the schema")
	}
}
