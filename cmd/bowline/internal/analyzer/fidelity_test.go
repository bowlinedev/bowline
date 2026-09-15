package analyzer

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files")

func TestFidelity(t *testing.T) {
	dir := fixtureDir(t)
	prog, err := Load(dir, testEnv(), "./rows/...")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := os.ReadDir(filepath.Join(dir, "rows"))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if !row.IsDir() {
			continue
		}
		t.Run(row.Name(), func(t *testing.T) {
			rowDir := filepath.Join(dir, "rows", row.Name())
			doc, diags := Analyze(prog, "./rows/"+row.Name()+".Routes")
			diagPath := filepath.Join(rowDir, "expected.diag.txt")
			contractPath := filepath.Join(rowDir, "expected.contract.json")
			var got []byte
			var goldenPath string
			if len(diags) > 0 {
				var b strings.Builder
				for _, d := range diags {
					b.WriteString(filepath.ToSlash(d.String()))
					b.WriteString("\n")
				}
				got = []byte(b.String())
				goldenPath = diagPath
			} else {
				got, err = doc.Marshal()
				if err != nil {
					t.Fatal(err)
				}
				goldenPath = contractPath
			}
			if *update {
				os.Remove(diagPath)
				os.Remove(contractPath)
				if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("%v; the analyzer produced:\n%s", err, got)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("golden mismatch for %s; run go test ./internal/analyzer -update after reviewing:\n%s", row.Name(), got)
			}
		})
	}
}
