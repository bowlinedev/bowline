package cli

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/bowlinedev/bowline/contract"
)

func fidelityCorpus(tb testing.TB) []*contract.Document {
	tb.Helper()
	paths, err := filepath.Glob(filepath.Join("..", "analyzer", "testdata", "fidelity", "rows", "*", "expected.contract.json"))
	if err != nil {
		tb.Fatal(err)
	}
	slices.Sort(paths)
	docs := make([]*contract.Document, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			tb.Fatal(err)
		}
		doc, err := contract.Parse(data)
		if err != nil {
			tb.Fatalf("%s: %v", path, err)
		}
		docs = append(docs, doc)
	}
	if len(docs) == 0 {
		tb.Fatal("no fidelity contracts found")
	}
	return docs
}

func BenchmarkGenerators(b *testing.B) {
	docs := fidelityCorpus(b)
	names := slices.Sorted(maps.Keys(Generators))
	for _, name := range names {
		generator := Generators[name]
		out := "bench/out" + generatorExtension(name)
		usable := make([]*contract.Document, 0, len(docs))
		for _, doc := range docs {
			if _, err := generator.Generate(doc, out); err == nil {
				usable = append(usable, doc)
			}
		}
		if len(usable) == 0 {
			b.Fatalf("%s generated nothing from the corpus", name)
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				doc := usable[i%len(usable)]
				if _, err := generator.Generate(doc, out); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(len(usable)), "contracts")
		})
	}
}

func generatorExtension(target string) string {
	switch target {
	case "ts":
		return ".ts"
	case "go":
		return ".go"
	case "dart":
		return ".dart"
	case "python":
		return ".py"
	case "rust":
		return ".rs"
	case "elixir":
		return ".ex"
	}
	return ".txt"
}
