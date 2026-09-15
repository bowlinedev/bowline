package fake

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/bowlinedev/bowline/contract"
)

var update = flag.Bool("update", false, "rewrite golden files")

func rows(t *testing.T) map[string]*contract.Document {
	t.Helper()
	root := filepath.Join("..", "analyzer", "testdata", "fidelity", "rows")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	docs := map[string]*contract.Document{}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(root, e.Name(), "expected.contract.json"))
		if err != nil {
			continue
		}
		doc, err := contract.Parse(data)
		if err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		docs[e.Name()] = doc
	}
	return docs
}

func encode(t *testing.T, v any) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestGoldens(t *testing.T) {
	for name, doc := range rows(t) {
		t.Run(name, func(t *testing.T) {
			g := New(doc, 1)
			out := map[string]any{}
			for _, p := range doc.Procedures {
				out[p.Path] = g.Output(p)
			}
			got := encode(t, out)
			golden := filepath.Join("testdata", name+".golden.json")
			if *update {
				os.WriteFile(golden, got, 0o644)
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("%v\n%s", err, got)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("golden mismatch; run go test ./internal/fake -update after reviewing\n%s", got)
			}
			for _, p := range doc.Procedures {
				check(t, doc, p.Output, out[p.Path], p.Path)
			}
		})
	}
}

func check(t *testing.T, doc *contract.Document, typ *contract.Type, v any, path string) {
	t.Helper()
	if v == nil {
		return
	}
	switch typ.Kind {
	case contract.Primitive:
		checkPrimitive(t, typ.Name, typ.Encoding, v, path)
	case contract.Ref:
		decl := doc.Types[typ.ID]
		switch decl.Kind {
		case contract.Primitive:
			checkPrimitive(t, decl.Primitive, typ.Encoding, v, path)
		case contract.Enum:
			found := false
			for _, ev := range decl.Values {
				if fmt.Sprint(ev.Value) == fmt.Sprint(v) {
					found = true
				}
			}
			if !found {
				t.Fatalf("%s: %v is not a member of %s", path, v, decl.Name)
			}
		case contract.Struct:
			checkObject(t, doc, decl.Fields, v, path)
		case contract.Generic:
			if _, ok := v.(map[string]any); !ok {
				t.Fatalf("%s: expected object, got %T", path, v)
			}
		}
	case contract.Array:
		list, ok := v.([]any)
		if !ok {
			t.Fatalf("%s: expected array, got %T", path, v)
		}
		if typ.Length > 0 && len(list) != typ.Length {
			t.Fatalf("%s: expected %d elements, got %d", path, typ.Length, len(list))
		}
		for i, e := range list {
			check(t, doc, typ.Elem, e, path+"."+strconv.Itoa(i))
		}
	case contract.Map:
		m, ok := v.(map[string]any)
		if !ok {
			t.Fatalf("%s: expected map, got %T", path, v)
		}
		for k, e := range m {
			check(t, doc, typ.Value, e, path+"."+k)
		}
	case contract.Struct:
		checkObject(t, doc, typ.Fields, v, path)
	}
}

func checkObject(t *testing.T, doc *contract.Document, fields []*contract.Field, v any, path string) {
	t.Helper()
	obj, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("%s: expected object, got %T", path, v)
	}
	for _, f := range fields {
		fv, present := obj[f.Name]
		fp := path + "." + f.Name
		if !present {
			if !f.Optional {
				t.Fatalf("%s: required field missing", fp)
			}
			continue
		}
		if fv == nil {
			if !f.Nullable && f.Type.Kind == contract.Primitive {
				t.Fatalf("%s: null for a non-nullable field", fp)
			}
			continue
		}
		if f.Example != nil {
			if fmt.Sprint(fv) != fmt.Sprint(f.Example) {
				t.Fatalf("%s: example %v was not used, got %v", fp, f.Example, fv)
			}
			continue
		}
		checkRules(t, f, fv, fp)
		check(t, doc, f.Type, fv, fp)
	}
}

func checkRules(t *testing.T, f *contract.Field, v any, path string) {
	t.Helper()
	size := func() (float64, bool) {
		switch x := v.(type) {
		case string:
			return float64(len(x)), true
		case []any:
			return float64(len(x)), true
		case map[string]any:
			return float64(len(x)), true
		case int64:
			return float64(x), true
		case float64:
			return x, true
		}
		return 0, false
	}
	for _, r := range f.Rules {
		switch r.Rule {
		case "required":
			if n, ok := size(); ok {
				if _, isNumber := v.(int64); !isNumber {
					if _, isFloat := v.(float64); !isFloat && n == 0 {
						t.Fatalf("%s: required but empty", path)
					}
				}
			}
		case "min", "max", "len":
			bound, _ := strconv.ParseFloat(r.Param, 64)
			n, ok := size()
			if !ok {
				continue
			}
			if (r.Rule == "min" && n < bound) || (r.Rule == "max" && n > bound) || (r.Rule == "len" && n != bound) {
				t.Fatalf("%s: %v violates %s=%s", path, v, r.Rule, r.Param)
			}
		case "oneof":
			found := false
			for _, option := range strings.Fields(r.Param) {
				if fmt.Sprint(v) == option {
					found = true
				}
			}
			if !found {
				t.Fatalf("%s: %v is not one of %s", path, v, r.Param)
			}
		case "email":
			if s, _ := v.(string); !strings.HasSuffix(s, "@example.com") {
				t.Fatalf("%s: %v is not an email", path, v)
			}
		case "url":
			if s, _ := v.(string); !strings.HasPrefix(s, "https://") {
				t.Fatalf("%s: %v is not a url", path, v)
			}
		case "uuid":
			if s, _ := v.(string); len(s) != 36 || s[14] != '4' {
				t.Fatalf("%s: %v is not a version 4 uuid", path, v)
			}
		}
	}
}

func checkPrimitive(t *testing.T, name, encoding string, v any, path string) {
	t.Helper()
	switch name {
	case "string", "timestamp", "bytes":
		if _, ok := v.(string); !ok {
			t.Fatalf("%s: expected string, got %T", path, v)
		}
	case "bool":
		if _, ok := v.(bool); !ok {
			t.Fatalf("%s: expected bool, got %T", path, v)
		}
	case "int8", "int16", "int32", "int64", "uint8", "uint16", "uint32", "uint64", "duration":
		if encoding == "string" {
			if _, ok := v.(string); !ok {
				t.Fatalf("%s: expected string-encoded integer, got %T", path, v)
			}
			return
		}
		n, ok := v.(int64)
		if !ok {
			t.Fatalf("%s: expected integer, got %T", path, v)
		}
		if n > safeInteger || n < -safeInteger {
			t.Fatalf("%s: %d exceeds the safe integer range", path, n)
		}
	case "float32", "float64":
		if _, ok := v.(float64); !ok {
			t.Fatalf("%s: expected float, got %T", path, v)
		}
	}
}

func TestDeterministicAcrossGenerators(t *testing.T) {
	doc := rows(t)["basics"]
	a := encode(t, New(doc, 7).Output(doc.Procedures[0]))
	b := encode(t, New(doc, 7).Output(doc.Procedures[0]))
	c := encode(t, New(doc, 8).Output(doc.Procedures[0]))
	if !bytes.Equal(a, b) {
		t.Fatal("same seed produced different output")
	}
	if bytes.Equal(a, c) {
		t.Fatal("different seeds produced the same output")
	}
}

func TestSiblingFieldDoesNotChangeExistingValues(t *testing.T) {
	doc := rows(t)["structs"]
	before := New(doc, 1).Output(doc.Procedures[0]).(map[string]any)
	for _, decl := range doc.Types {
		if decl.Kind == contract.Struct {
			decl.Fields = append(decl.Fields, &contract.Field{Name: "zzzAdded", Type: &contract.Type{Kind: contract.Primitive, Name: "string"}})
		}
	}
	after := New(doc, 1).Output(doc.Procedures[0]).(map[string]any)
	strip(after)
	if a, b := encode(t, before), encode(t, after); !bytes.Equal(a, b) {
		t.Fatalf("values changed after adding a sibling field:\n%s\n%s", a, b)
	}
}

func strip(v any) {
	switch x := v.(type) {
	case map[string]any:
		delete(x, "zzzAdded")
		for _, e := range x {
			strip(e)
		}
	case []any:
		for _, e := range x {
			strip(e)
		}
	}
}

func TestRecursionIsCapped(t *testing.T) {
	doc := rows(t)["recursive"]
	for _, p := range doc.Procedures {
		data := encode(t, New(doc, 1).Output(p))
		if depth := maxNesting(data); depth > 40 {
			t.Fatalf("%s nests %d levels", p.Path, depth)
		}
	}
}

func maxNesting(data []byte) int {
	depth, deepest := 0, 0
	for _, b := range data {
		switch b {
		case '{', '[':
			depth++
			if depth > deepest {
				deepest = depth
			}
		case '}', ']':
			depth--
		}
	}
	return deepest
}

func TestSafeIntegerBound(t *testing.T) {
	doc := &contract.Document{Bowline: contract.Version, Types: map[string]*contract.TypeDecl{}, Errors: map[string]*contract.ErrorDecl{}}
	f := &contract.Field{Name: "n", Type: &contract.Type{Kind: contract.Primitive, Name: "int64"}, Rules: []contract.Rule{{Rule: "min", Param: "9007199254740000"}, {Rule: "max", Param: "18446744073709551615"}}}
	typ := &contract.Type{Kind: contract.Struct, Fields: []*contract.Field{f}}
	for seed := uint64(1); seed < 50; seed++ {
		v := New(doc, seed).Value(typ, "p", nil).(map[string]any)["n"].(int64)
		if v > safeInteger || float64(v) > math.Pow(2, 53) {
			t.Fatalf("seed %d produced %d", seed, v)
		}
	}
	u := &contract.Field{Name: "u", Type: &contract.Type{Kind: contract.Primitive, Name: "uint64"}}
	v := New(doc, 3).Value(&contract.Type{Kind: contract.Struct, Fields: []*contract.Field{u}}, "p", nil).(map[string]any)["u"].(int64)
	if v < 1 || v > 1000 {
		t.Fatalf("default range violated: %d", v)
	}
}
