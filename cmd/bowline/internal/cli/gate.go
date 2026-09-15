package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/bowlinedev/bowline/contract"
)

func Check(opts Options, args []string) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	against := fs.String("against", "", "")
	allowBreaking := fs.Bool("allow-breaking", false, "")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: check: %v\nusage: bowline check [--against <git-ref>] [--allow-breaking]\n", err)
		return 2
	}
	if *against == "" {
		return checkDrift(opts)
	}
	return checkAgainst(opts, *against, *allowBreaking)
}

func checkAgainst(opts Options, ref string, allowBreaking bool) int {
	cfg, err := LoadConfig(opts.Dir)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	cmd := exec.Command("git", "show", ref+":"+cfg.Contract)
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
	if len(changes) == 0 {
		fmt.Fprintln(opts.Stdout, "no contract changes")
		return 0
	}
	fmt.Fprint(opts.Stdout, contract.FormatMarkdown(changes))
	breaking := 0
	for _, c := range changes {
		if c.Category == contract.Breaking {
			breaking++
		}
	}
	fmt.Fprintf(opts.Stderr, "bowline: %d contract change(s), %d breaking\n", len(changes), breaking)
	if breaking > 0 && !allowBreaking {
		return 1
	}
	return 0
}
