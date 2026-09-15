package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

func checkDrift(opts Options, asJSON bool) int {
	files, diags, err := Produce(opts)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	if len(diags) > 0 {
		printDiagnostics(opts.Stderr, diags)
		return 1
	}
	report := &CheckReport{Mode: "drift"}
	drift := 0
	for _, rel := range sortedPaths(files) {
		existing, readErr := os.ReadFile(filepath.Join(opts.Dir, filepath.FromSlash(rel)))
		status := "ok"
		switch {
		case readErr != nil:
			status = "missing"
			drift++
		case !bytes.Equal(existing, files[rel]):
			status = "outdated"
			drift++
		}
		report.Files = append(report.Files, CheckFile{Path: rel, Status: status})
		if asJSON {
			continue
		}
		if status == "ok" {
			fmt.Fprintf(opts.Stdout, "ok        %s\n", rel)
		} else {
			fmt.Fprintf(opts.Stderr, "%-9s %s\n", status, rel)
		}
	}
	report.OK = drift == 0
	if asJSON {
		if err := writeReport(opts.Stdout, report); err != nil {
			fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
			return 1
		}
		if drift > 0 {
			return 1
		}
		return 0
	}
	if drift > 0 {
		fmt.Fprintf(opts.Stderr, "bowline: %d file(s) out of date; run bowline gen\n", drift)
		return 1
	}
	return 0
}
