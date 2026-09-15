package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/bowlinedev/bowline/contract"
	"github.com/bowlinedev/bowline/registry"
)

func Check(opts Options, args []string) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	against := fs.String("against", "", "")
	allowBreaking := fs.Bool("allow-breaking", false, "")
	consumersDir := fs.String("consumers", "", "")
	registryURL := fs.String("registry", "", "")
	service := fs.String("service", "", "")
	strict := fs.Bool("strict", false, "")
	token := fs.String("token", "", "")
	asJSON := fs.Bool("json", false, "")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: check: %v\n%s\n", err, checkUsage)
		return 2
	}
	if *registryURL != "" {
		return checkRegistry(opts, *registryURL, *service, *token, *strict, *asJSON)
	}
	if *service != "" || *strict {
		fmt.Fprintf(opts.Stderr, "bowline: check: --service and --strict need --registry\n%s\n", checkUsage)
		return 2
	}
	if *against == "" {
		return checkDrift(opts, *asJSON)
	}
	return checkAgainst(opts, *against, *allowBreaking, *consumersDir, *asJSON)
}

const checkUsage = "usage: bowline check [--against <ref>] [--allow-breaking] [--consumers dir] [--json]\n       bowline check --registry URL --service NAME [--strict] [--token T] [--json]"

func checkRegistry(opts Options, registryURL, service, token string, strict, asJSON bool) int {
	if service == "" {
		fmt.Fprintf(opts.Stderr, "bowline: check: --registry needs --service\n%s\n", checkUsage)
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
	candidate, err := contract.Parse(files[cfg.Contract])
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	if token == "" {
		if tokens := envTokens(); len(tokens) > 0 {
			token = tokens[0]
		}
	}
	client := &registry.Client{URL: registryURL, Token: token}
	report, err := client.Impact(context.Background(), service, candidate, strict)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	if asJSON {
		if err := writeReport(opts.Stdout, &CheckReport{Mode: "registry", OK: report.OK, Impact: report}); err != nil {
			fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
			return 1
		}
		if report.OK {
			return 0
		}
		return 1
	}
	if report.OK {
		fmt.Fprint(opts.Stdout, registry.FormatText(report))
		return 0
	}
	fmt.Fprint(opts.Stderr, registry.FormatText(report))
	return 1
}

func checkAgainst(opts Options, ref string, allowBreaking bool, consumersDir string, asJSON bool) int {
	cfg, err := LoadConfig(opts.Dir)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	cmd := exec.Command("git", "show", ref+":./"+cfg.Contract)
	cmd.Dir = opts.Dir
	oldRaw, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		detail := err.Error()
		if errors.As(err, &exitErr) {
			detail = strings.TrimSpace(string(exitErr.Stderr))
		}
		fmt.Fprintf(opts.Stderr, "bowline: reading %s at %s: %s\n", cfg.Contract, ref, detail)
		return 1
	}
	oldMigrated, err := contract.Migrate(oldRaw)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	old, err := contract.Parse(oldMigrated)
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
	current, err := contract.Parse(files[cfg.Contract])
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	changes := contract.Diff(old, current)
	breaking := 0
	for _, c := range changes {
		if c.Category == contract.Breaking {
			breaking++
		}
	}
	failed := breaking > 0 && !allowBreaking
	if asJSON {
		report := &CheckReport{Mode: "against", OK: !failed, Ref: ref, Changes: changes, Breaking: breaking}
		if report.Changes == nil {
			report.Changes = []contract.Change{}
		}
		if err := writeReport(opts.Stdout, report); err != nil {
			fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
			return 1
		}
		if failed {
			return 1
		}
		return 0
	}
	if len(changes) == 0 {
		fmt.Fprintln(opts.Stdout, "no contract changes")
		return 0
	}
	list, err := consumerList(opts.Dir, consumersDir)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	fmt.Fprint(opts.Stdout, renderMarkdown(old, changes, list))
	fmt.Fprintf(opts.Stderr, "bowline: %d contract change(s), %d breaking\n", len(changes), breaking)
	if failed {
		return 1
	}
	return 0
}
