package analyzer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func BenchmarkDevLoopUpdate(b *testing.B) {
	dir := b.TempDir()
	writeSynth(b, dir, 20, 25, 10)
	s, err := NewSession(dir, testEnv(), "./...")
	if err != nil {
		b.Fatal(err)
	}
	if _, diags := Analyze(s.Program(), "./api.Routes"); len(diags) > 0 {
		b.Fatal(diags)
	}
	file := filepath.Join(dir, "p10", "p10.go")
	src, err := os.ReadFile(file)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tag := "name"
		if i%2 == 0 {
			tag = "label"
		}
		edited := strings.Replace(string(src), "`json:\"name\" validate:\"required\"`", "`json:\""+tag+"\" validate:\"required\"`", 1)
		if err := os.WriteFile(file, []byte(edited), 0o644); err != nil {
			b.Fatal(err)
		}
		if _, typeErrs, err := s.Update([]string{file}); err != nil || len(typeErrs) > 0 {
			b.Fatal(err, typeErrs)
		}
		if _, diags := Analyze(s.Program(), "./api.Routes"); len(diags) > 0 {
			b.Fatal(diags)
		}
	}
}
