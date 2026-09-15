package cli

import (
	"errors"
	"flag"
	"fmt"
)

func Gen(opts Options, args []string) int {
	flags := flag.NewFlagSet("gen", flag.ContinueOnError)
	flags.SetOutput(opts.Stderr)
	from := flags.String("from", "", "generate the targets from this contract document instead of analyzing the module")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	files, diags, err := ProduceFrom(opts, *from)
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
