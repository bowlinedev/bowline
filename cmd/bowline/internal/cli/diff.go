package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bowlinedev/bowline/contract"
)

func DiffCommand(opts Options, args []string) int {
	format := "text"
	var paths []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--format":
			if i+1 >= len(args) {
				fmt.Fprintln(opts.Stderr, "bowline: --format needs a value: text, markdown, or json")
				return 2
			}
			format = args[i+1]
			i++
		default:
			paths = append(paths, args[i])
		}
	}
	if len(paths) != 2 {
		fmt.Fprintln(opts.Stderr, "usage: bowline diff <old.json> <new.json> [--format text|markdown|json]")
		return 2
	}
	old, err := readContract(opts.Dir, paths[0])
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	current, err := readContract(opts.Dir, paths[1])
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	changes := contract.Diff(old, current)
	switch format {
	case "text":
		fmt.Fprint(opts.Stdout, contract.FormatText(changes))
	case "markdown":
		fmt.Fprint(opts.Stdout, contract.FormatMarkdown(changes))
	case "json":
		data, err := contract.FormatJSON(changes)
		if err != nil {
			fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
			return 1
		}
		opts.Stdout.Write(data)
	default:
		fmt.Fprintf(opts.Stderr, "bowline: unknown format %q; use text, markdown, or json\n", format)
		return 2
	}
	if changes.Breaking() {
		return 1
	}
	return 0
}

func readContract(dir, rel string) (*contract.Document, error) {
	path := rel
	if !filepath.IsAbs(rel) {
		path = filepath.Join(dir, filepath.FromSlash(rel))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return contract.Parse(data)
}
