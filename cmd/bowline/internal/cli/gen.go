package cli

import (
	"fmt"
)

func Gen(opts Options) int {
	files, diags, err := Produce(opts)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	if len(diags) > 0 {
		printDiagnostics(opts.Stderr, diags)
		return 1
	}
	written, err := writeFiles(opts.Dir, files)
	wrote := map[string]bool{}
	for _, rel := range written {
		wrote[rel] = true
	}
	for _, rel := range sortedPaths(files) {
		if wrote[rel] {
			fmt.Fprintf(opts.Stdout, "wrote %s\n", rel)
		} else {
			fmt.Fprintf(opts.Stdout, "unchanged %s\n", rel)
		}
	}
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	return 0
}
