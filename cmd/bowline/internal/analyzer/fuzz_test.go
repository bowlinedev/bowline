package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var fuzzTypeSwap = []string{
	"string", "bool", "int", "int8", "int16", "int32", "int64",
	"uint8", "uint16", "uint32", "uint64", "float32", "float64",
	"[]string", "[]byte", "map[string]int", "*string", "any",
	"time.Time", "time.Duration", "struct{}", "chan int", "func()",
	"complex64", "uintptr", "error",
}

func fuzzRowSources(t *testing.T) map[string]string {
	t.Helper()
	rows, err := filepath.Glob(filepath.Join("testdata", "fidelity", "rows", "*", "api.go"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, name := range rows {
		data, err := os.ReadFile(name)
		if err != nil {
			continue
		}
		out[filepath.Base(filepath.Dir(name))] = string(data)
	}
	return out
}

func fuzzMutate(source, replacement string, seed int) string {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "api.go", source, parser.SkipObjectResolution)
	if err != nil {
		return ""
	}
	var targets []*ast.Field
	ast.Inspect(file, func(n ast.Node) bool {
		st, ok := n.(*ast.StructType)
		if !ok || st.Fields == nil {
			return true
		}
		targets = append(targets, st.Fields.List...)
		return true
	})
	if len(targets) == 0 {
		return ""
	}
	field := targets[seed%len(targets)]
	start := fset.Position(field.Type.Pos()).Offset
	end := fset.Position(field.Type.End()).Offset
	if start <= 0 || end > len(source) || start >= end {
		return ""
	}
	return source[:start] + replacement + source[end:]
}

func FuzzAnalyzerMutatesFidelityRows(f *testing.F) {
	for _, row := range []string{"basics", "collections", "enums", "embedded", "aliases"} {
		for i := range 4 {
			f.Add(row, i, i*3)
		}
	}

	f.Fuzz(func(t *testing.T, row string, swap, field int) {
		sources := fuzzRowSources(t)
		if len(sources) == 0 {
			t.Skip("no fidelity rows")
		}
		source, ok := sources[row]
		if !ok {
			return
		}
		if swap < 0 || field < 0 {
			return
		}
		mutated := fuzzMutate(source, fuzzTypeSwap[swap%len(fuzzTypeSwap)], field)
		if mutated == "" {
			return
		}

		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "api.go"), []byte(mutated), 0o600); err != nil {
			t.Fatal(err)
		}
		gomod, err := os.ReadFile(filepath.Join("testdata", "fidelity", "go.mod"))
		if err != nil {
			t.Fatal(err)
		}
		repo, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
		if err != nil {
			t.Fatal(err)
		}
		rewritten := strings.Replace(string(gomod), "../../../../../..", repo, 1)
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(rewritten), 0o600); err != nil {
			t.Fatal(err)
		}

		prog, err := Load(dir, append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod"), "./...")
		if err != nil {
			return
		}
		for _, pkg := range prog.Pkgs {
			if len(pkg.Errors) > 0 {
				return
			}
		}

		doc, diags := Analyze(prog, "Routes")
		if doc == nil && len(diags) == 0 {
			t.Fatalf("row %q with %q at field %d produced no document and no diagnostics", row, fuzzTypeSwap[swap%len(fuzzTypeSwap)], field)
		}
		for _, d := range diags {
			if d.Message == "" {
				t.Fatalf("row %q produced a diagnostic with no message", row)
			}
		}
		if doc == nil {
			return
		}
		for _, p := range doc.Procedures {
			if p == nil {
				t.Fatalf("row %q produced a null procedure", row)
			}
			if p.Input == nil || p.Output == nil {
				t.Fatalf("row %q produced procedure %q with a missing input or output", row, p.Path)
			}
		}
	})
}
