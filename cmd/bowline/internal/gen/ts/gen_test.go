package ts

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/bowlinedev/bowline/contract"
)

var update = flag.Bool("update", false, "rewrite golden files")

func TestGoldens(t *testing.T) {
	inputs, err := filepath.Glob(filepath.Join("..", "..", "analyzer", "testdata", "fidelity", "rows", "*", "expected.contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) == 0 {
		t.Fatal("no contracts found; run the analyzer fidelity suite first")
	}
	for _, input := range inputs {
		row := filepath.Base(filepath.Dir(input))
		t.Run(row, func(t *testing.T) {
			data, err := os.ReadFile(input)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := contract.Parse(data)
			if err != nil {
				t.Fatal(err)
			}
			got, err := Generator{}.Generate(doc, "bowline.ts")
			if err != nil {
				t.Fatal(err)
			}
			golden := filepath.Join("testdata", row+".golden.ts")
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
				t.Fatalf("golden mismatch; run go test ./internal/gen/ts -update after review:\n%s", got)
			}
		})
	}
}

func TestNameCollisions(t *testing.T) {
	doc := &contract.Document{Bowline: contract.Version, Types: map[string]*contract.TypeDecl{
		"example.com/app/users.Event": {Kind: contract.Struct, Name: "Event"},
		"example.com/app/audit.Event": {Kind: contract.Struct, Name: "Event"},
		"example.com/app/audit.Log":   {Kind: contract.Struct, Name: "Log"},
	}}
	names := assignNames(doc)
	if names["example.com/app/users.Event"] != "Users_Event" || names["example.com/app/audit.Event"] != "Audit_Event" || names["example.com/app/audit.Log"] != "Log" {
		t.Fatalf("%v", names)
	}
}
