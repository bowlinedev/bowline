package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSynth(t *testing.T, dir string, packages, typesPerPackage, procsPerPackage int) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	must := func(path, content string) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	must(filepath.Join(dir, "go.mod"), "module synth.test\n\ngo 1.24\n\nrequire github.com/bowlinedev/bowline v0.0.0\n\nreplace github.com/bowlinedev/bowline => "+filepath.ToSlash(root)+"\n")
	var mounts []string
	var imports []string
	for p := 0; p < packages; p++ {
		name := fmt.Sprintf("p%02d", p)
		var b strings.Builder
		fmt.Fprintf(&b, "package %s\n\nimport (\n\t\"context\"\n\t\"time\"\n\n\t\"github.com/bowlinedev/bowline\"\n", name)
		if p > 0 {
			fmt.Fprintf(&b, "\t\"synth.test/p%02d\"\n", p-1)
		}
		b.WriteString(")\n\n")
		for i := 0; i < typesPerPackage; i++ {
			fmt.Fprintf(&b, "type T%d struct {\n\tID int64 `json:\"id\"`\n\tName string `json:\"name\" validate:\"required\"`\n\tAt time.Time `json:\"at\"`\n\tTags []string `json:\"tags\"`\n", i)
			if i > 0 {
				fmt.Fprintf(&b, "\tPrev *T%d `json:\"prev,omitempty\"`\n", i-1)
			}
			if p > 0 && i == 0 {
				fmt.Fprintf(&b, "\tUpstream p%02d.T0 `json:\"upstream\"`\n", p-1)
			}
			b.WriteString("}\n\n")
		}
		for i := 0; i < procsPerPackage; i++ {
			out := (i + 1) % typesPerPackage
			fmt.Fprintf(&b, "func Get%d(ctx context.Context, in T%d) (T%d, error) { return T%d{}, nil }\n\n", i, i%typesPerPackage, out, out)
		}
		b.WriteString("func Router() *bowline.Router {\n\treturn bowline.NewRouter(\n")
		for i := 0; i < procsPerPackage; i++ {
			fmt.Fprintf(&b, "\t\tbowline.Query(\"get%d\", Get%d),\n", i, i)
		}
		b.WriteString("\t)\n}\n")
		must(filepath.Join(dir, name, name+".go"), b.String())
		mounts = append(mounts, fmt.Sprintf("\t\tbowline.Mount(%q, %s.Router()),", name, name))
		imports = append(imports, fmt.Sprintf("\t\"synth.test/%s\"", name))
	}
	must(filepath.Join(dir, "api", "api.go"), "package api\n\nimport (\n\t\"github.com/bowlinedev/bowline\"\n\n"+strings.Join(imports, "\n")+"\n)\n\nfunc Routes() *bowline.Router {\n\treturn bowline.NewRouter(\n"+strings.Join(mounts, "\n")+"\n\t)\n}\n")
}
