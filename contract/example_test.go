package contract

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSpecExamplesAreCanonical(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "spec", "examples", "*.contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no examples found under spec/examples")
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := Parse(data)
			if err != nil {
				t.Fatal(err)
			}
			again, err := doc.Marshal()
			if err != nil {
				t.Fatal(err)
			}
			if *update {
				if err := doc.SetHash(); err != nil {
					t.Fatal(err)
				}
				again, err = doc.Marshal()
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, again, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := doc.ComputeHash()
			if err != nil {
				t.Fatal(err)
			}
			if doc.Hash != want {
				t.Fatalf("hash field %s does not match content hash %s", doc.Hash, want)
			}
			if !bytes.Equal(data, again) {
				t.Fatalf("%s is not in canonical form; run: go test ./contract -update", path)
			}
		})
	}
}
