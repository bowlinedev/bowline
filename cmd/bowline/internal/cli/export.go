package cli

import (
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/bowlinedev/bowline/cmd/bowline/internal/export/openapi"
	"github.com/bowlinedev/bowline/cmd/bowline/internal/tools"
	"github.com/bowlinedev/bowline/contract"
)

func Export(opts Options, args []string) int {
	if len(args) > 0 && args[0] == "tools" {
		return exportTools(opts, args[1:])
	}
	if len(args) == 0 || args[0] != "openapi" {
		fmt.Fprintln(opts.Stderr, "bowline: usage: bowline export openapi [-o path] | bowline export tools [--format anthropic|openai|json-schema] [--scope S]... [--read-only] [--out path]")
		return 2
	}
	flags := flag.NewFlagSet("export openapi", flag.ContinueOnError)
	flags.SetOutput(opts.Stderr)
	out := flags.String("o", "", "output path relative to the module root")
	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}
	cfg, err := LoadConfig(opts.Dir)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	files, diags, err := Produce(opts)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	if len(diags) > 0 {
		printDiagnostics(opts.Stderr, diags)
		return 1
	}
	doc, err := contract.Parse(files[cfg.Contract])
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	info := openapi.Info{}
	if cfg.OpenAPI != nil {
		info = openapi.Info{Title: cfg.OpenAPI.Title, Version: cfg.OpenAPI.Version, ServerURL: cfg.OpenAPI.ServerURL, AutoPatch: cfg.OpenAPI.AutoPatch, ETags: cfg.OpenAPI.ETags, Problem: cfg.OpenAPI.Problem}
	}
	data, err := openapi.Export(doc, info)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	rel := *out
	if rel == "" && cfg.OpenAPI != nil {
		rel = cfg.OpenAPI.Out
	}
	if rel == "" {
		rel = path.Join(path.Dir(cfg.Contract), "openapi.json")
	}
	target := filepath.Join(opts.Dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	if err := os.WriteFile(target, data, 0o644); err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	fmt.Fprintf(opts.Stdout, "wrote %s\n", rel)
	return 0
}

type scopeList []string

func (s *scopeList) String() string { return strings.Join(*s, ",") }

func (s *scopeList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func exportTools(opts Options, args []string) int {
	flags := flag.NewFlagSet("export tools", flag.ContinueOnError)
	flags.SetOutput(opts.Stderr)
	format := flags.String("format", tools.FormatJSONSchema, "anthropic, openai, or json-schema")
	readOnly := flags.Bool("read-only", false, "only tools with the read-only hint")
	out := flags.String("out", "", "output path relative to the module root; stdout when empty")
	var scopes scopeList
	flags.Var(&scopes, "scope", "only tools with this scope; repeatable")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	cfg, err := LoadConfig(opts.Dir)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	files, diags, err := Produce(opts)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	if len(diags) > 0 {
		printDiagnostics(opts.Stderr, diags)
		return 1
	}
	doc, err := contract.Parse(files[cfg.Contract])
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	list, err := tools.FromContract(doc, tools.Filter{Scopes: scopes, ReadOnly: *readOnly})
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	data, err := tools.Encode(list, *format)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	if *out == "" {
		_, _ = opts.Stdout.Write(data)
		return 0
	}
	target := filepath.Join(opts.Dir, filepath.FromSlash(*out))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	if err := os.WriteFile(target, data, 0o644); err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	fmt.Fprintf(opts.Stdout, "wrote %s\n", *out)
	return 0
}
