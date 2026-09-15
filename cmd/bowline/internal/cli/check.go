package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

func Check(opts Options) int {
	files, diags, err := Produce(opts)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	if len(diags) > 0 {
		printDiagnostics(opts.Stderr, diags)
		return 1
	}
	drift := 0
	for _, rel := range sortedPaths(files) {
		existing, readErr := os.ReadFile(filepath.Join(opts.Dir, filepath.FromSlash(rel)))
		switch {
		case readErr != nil:
			fmt.Fprintf(opts.Stderr, "missing   %s\n", rel)
			drift++
		case !bytes.Equal(existing, files[rel]):
			fmt.Fprintf(opts.Stderr, "outdated  %s\n", rel)
			drift++
		default:
			fmt.Fprintf(opts.Stdout, "ok        %s\n", rel)
		}
	}
	if drift > 0 {
		fmt.Fprintf(opts.Stderr, "bowline: %d file(s) out of date; run bowline gen\n", drift)
		return 1
	}
	return 0
}
