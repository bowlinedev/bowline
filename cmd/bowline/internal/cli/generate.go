package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/bowlinedev/bowline/cmd/bowline/internal/analyzer"
	"github.com/bowlinedev/bowline/cmd/bowline/internal/gen/ts"
	"github.com/bowlinedev/bowline/contract"
)

type Generator interface {
	Generate(doc *contract.Document, out string) ([]byte, error)
}

var Generators = map[string]Generator{}

type Options struct {
	Dir    string
	Env    []string
	Stdout io.Writer
	Stderr io.Writer
}

func Produce(opts Options) (map[string][]byte, []analyzer.Diagnostic, error) {
	cfg, err := LoadConfig(opts.Dir)
	if err != nil {
		return nil, nil, err
	}
	for name := range cfg.Targets {
		if _, ok := Generators[name]; !ok {
			return nil, nil, fmt.Errorf("unknown target %q; available: %s", name, availableTargets())
		}
	}
	env := opts.Env
	if env == nil {
		env = os.Environ()
	}
	prog, err := analyzer.Load(opts.Dir, env)
	if err != nil {
		return nil, nil, err
	}
	doc, diags := analyzer.Analyze(prog, cfg.Entry)
	if len(diags) > 0 {
		return nil, diags, nil
	}
	files, err := render(doc, cfg)
	if err != nil {
		return nil, nil, err
	}
	return files, nil, nil
}

func render(doc *contract.Document, cfg *Config) (map[string][]byte, error) {
	files := map[string][]byte{}
	data, err := doc.Marshal()
	if err != nil {
		return nil, err
	}
	files[cfg.Contract] = data
	for name, target := range cfg.Targets {
		content, err := Generators[name].Generate(doc, target.Out)
		if err != nil {
			return nil, fmt.Errorf("target %s: %w", name, err)
		}
		files[target.Out] = content
		if target.Zod {
			zodOut, zod, err := zodOutput(doc, name, target.Out)
			if err != nil {
				return nil, fmt.Errorf("target %s: %w", name, err)
			}
			files[zodOut] = zod
		}
	}
	return files, nil
}

func zodOutput(doc *contract.Document, name, out string) (string, []byte, error) {
	if name != "ts" {
		return "", nil, fmt.Errorf("zod is only available on the ts target")
	}
	dir := path.Dir(out)
	zodOut := "bowline.zod.ts"
	if dir != "." {
		zodOut = dir + "/" + zodOut
	}
	zod, err := ts.Generator{}.GenerateZodFor(doc, path.Base(out))
	return zodOut, zod, err
}

func writeFiles(dir string, files map[string][]byte) ([]string, error) {
	var written []string
	for _, rel := range sortedPaths(files) {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		existing, err := os.ReadFile(path)
		if err == nil && bytes.Equal(existing, files[rel]) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return written, err
		}
		if err := os.WriteFile(path, files[rel], 0o644); err != nil {
			return written, err
		}
		written = append(written, rel)
	}
	return written, nil
}

func availableTargets() string {
	names := make([]string, 0, len(Generators))
	for name := range Generators {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return "none"
	}
	return fmt.Sprint(names)
}

func printDiagnostics(w io.Writer, diags []analyzer.Diagnostic) {
	for _, d := range diags {
		fmt.Fprintln(w, d.String())
	}
	fmt.Fprintf(w, "bowline: %d problem(s); nothing written\n", len(diags))
}

func sortedPaths(files map[string][]byte) []string {
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}
