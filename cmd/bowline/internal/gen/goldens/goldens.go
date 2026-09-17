package goldens

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/bowlinedev/bowline/contract"
)

var Update = flag.Bool("update", false, "rewrite golden files")

type Generator interface {
	Generate(doc *contract.Document, out string) ([]byte, error)
}

func Rows(t *testing.T) map[string]*contract.Document {
	t.Helper()
	inputs, err := filepath.Glob(filepath.Join("..", "..", "analyzer", "testdata", "fidelity", "rows", "*", "expected.contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) == 0 {
		t.Fatal("no contracts found; run the analyzer fidelity suite first")
	}
	docs := map[string]*contract.Document{}
	for _, input := range inputs {
		data, err := os.ReadFile(input)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := contract.Parse(data)
		if err != nil {
			t.Fatalf("%s: %v", input, err)
		}
		docs[filepath.Base(filepath.Dir(input))] = doc
	}
	return docs
}

func Run(t *testing.T, target, ext string, gen Generator) {
	t.Helper()
	for row, doc := range Rows(t) {
		t.Run(row, func(t *testing.T) {
			got, err := gen.Generate(doc, "bowline."+ext)
			if err != nil {
				t.Fatal(err)
			}
			Compare(t, target, filepath.Join("testdata", row+".golden."+ext), got)
		})
	}
}

func Compare(t *testing.T, target, golden string, got []byte) {
	t.Helper()
	if *Update {
		if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v\n%s", err, got)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("golden mismatch; run go test ./internal/gen/%s -update after review:\n%s", target, got)
	}
}
