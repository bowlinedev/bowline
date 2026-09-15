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
	"github.com/bowlinedev/bowline/cmd/bowline/internal/export/openapi"
	"github.com/bowlinedev/bowline/cmd/bowline/internal/gen/ts"
	"github.com/bowlinedev/bowline/cmd/bowline/internal/jsonschema"
	"github.com/bowlinedev/bowline/cmd/bowline/internal/tools"
	"github.com/bowlinedev/bowline/contract"
)

type Generator interface {
	Generate(doc *contract.Document, out string) ([]byte, error)
}

type PackageAware interface {
	WithPackage(pkg string) any
}

var Generators = map[string]Generator{}

type Options struct {
	Dir    string
	Env    []string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Stop   <-chan struct{}
}

func Produce(opts Options) (map[string][]byte, []analyzer.Diagnostic, error) {
	cfg, err := LoadConfig(opts.Dir)
	if err != nil {
		return nil, nil, err
	}
	for name := range cfg.Targets {
		if _, ok := Generators[name]; !ok && name != "tools" {
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
	if cfg.Schemas {
		for _, p := range doc.Procedures {
			if p.Tool == nil {
				continue
			}
			input, output, err := jsonschema.Procedure(doc, p)
			if err != nil {
				return nil, fmt.Errorf("schemas: %w", err)
			}
			p.Schemas = &contract.Schemas{Input: input, Output: output}
		}
		if err := doc.SetHash(); err != nil {
			return nil, err
		}
	}
	data, err := doc.Marshal()
	if err != nil {
		return nil, err
	}
	files[cfg.Contract] = data
	if cfg.OpenAPI != nil {
		out := cfg.OpenAPI.Out
		if out == "" {
			out = path.Join(path.Dir(cfg.Contract), "openapi.json")
		}
		spec, err := openapi.Export(doc, openapi.Info{Title: cfg.OpenAPI.Title, Version: cfg.OpenAPI.Version, ServerURL: cfg.OpenAPI.ServerURL})
		if err != nil {
			return nil, fmt.Errorf("openapi: %w", err)
		}
		files[out] = spec
	}
	for name, target := range cfg.Targets {
		if name == "tools" {
			format := target.Format
			if format == "" {
				format = tools.FormatJSONSchema
			}
			list, err := tools.FromContract(doc, tools.Filter{})
			if err != nil {
				return nil, fmt.Errorf("target tools: %w", err)
			}
			content, err := tools.Encode(list, format)
			if err != nil {
				return nil, fmt.Errorf("target tools: %w", err)
			}
			files[target.Out] = content
			continue
		}
		generator := Generators[name]
		if aware, ok := generator.(PackageAware); ok && target.Package != "" {
			if configured, ok := aware.WithPackage(target.Package).(Generator); ok {
				generator = configured
			}
		}
		content, err := generator.Generate(doc, target.Out)
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
	names := make([]string, 0, len(Generators)+1)
	for name := range Generators {
		names = append(names, name)
	}
	names = append(names, "tools")
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
