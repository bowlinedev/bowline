package ts

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/bowlinedev/bowline/contract"
)

var bareAny = regexp.MustCompile(`(?:^|[^A-Za-z0-9_$."'])any(?:[^A-Za-z0-9_$]|$)`)

func fuzzSeedContracts(f *testing.F) [][]byte {
	patterns := []string{
		filepath.Join("testdata", "*.contract.json"),
		filepath.Join("..", "..", "..", "..", "..", "cmd", "bowline", "internal", "analyzer", "testdata", "*", "expected.contract.json"),
		filepath.Join("..", "..", "..", "..", "..", "gateway", "testdata", "compose", "*.golden.json"),
	}
	var out [][]byte
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, name := range matches {
			data, err := os.ReadFile(name)
			if err != nil {
				continue
			}
			out = append(out, data)
		}
	}
	return out
}

func FuzzTSGenerator(f *testing.F) {
	f.Add([]byte(`{"version":"1.2","types":{},"procedures":{}}`))
	for _, data := range fuzzSeedContracts(f) {
		f.Add(data)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		doc, err := contract.Parse(data)
		if err != nil {
			return
		}
		first, err := Generator{}.Generate(doc, "bowline.ts")
		if err != nil {
			return
		}
		second, err := Generator{}.Generate(doc, "bowline.ts")
		if err != nil {
			t.Fatalf("the generator succeeded once and then failed: %v", err)
		}
		if string(first) != string(second) {
			t.Fatal("the generator is not deterministic across two runs")
		}
		for i, line := range strings.Split(string(first), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "*") || strings.HasPrefix(trimmed, "//") {
				continue
			}
			if bareAny.MatchString(line) {
				t.Fatalf("line %d falls back to any: %q", i+1, line)
			}
		}
	})
}
