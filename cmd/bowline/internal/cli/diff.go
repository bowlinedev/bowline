package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bowlinedev/bowline/cmd/bowline/internal/consumers"
	"github.com/bowlinedev/bowline/contract"
)

func DiffCommand(opts Options, args []string) int {
	format := "text"
	consumersDir := ""
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
		case "--consumers":
			if i+1 >= len(args) {
				fmt.Fprintln(opts.Stderr, "bowline: --consumers needs a directory")
				return 2
			}
			consumersDir = args[i+1]
			i++
		case "--json":
			format = "json"
		default:
			paths = append(paths, args[i])
		}
	}
	if len(paths) != 2 {
		fmt.Fprintln(opts.Stderr, "usage: bowline diff <old.json> <new.json> [--format text|markdown|json] [--json] [--consumers dir]")
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
	list, err := consumerList(opts.Dir, consumersDir)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	switch format {
	case "text":
		fmt.Fprint(opts.Stdout, renderText(old, changes, list))
	case "markdown":
		fmt.Fprint(opts.Stdout, renderMarkdown(old, changes, list))
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

func consumerList(dir, explicit string) ([]consumers.Consumer, error) {
	target := explicit
	if target == "" {
		target = DefaultConsumersDir
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(target))); err != nil {
			return nil, nil
		}
	}
	list, err := consumers.Load(filepath.Join(dir, filepath.FromSlash(target)))
	if err != nil {
		return nil, fmt.Errorf("loading consumer contracts from %s: %w", target, err)
	}
	return list, nil
}

func renderText(old *contract.Document, changes contract.Changes, list []consumers.Consumer) string {
	if list == nil {
		return contract.FormatText(changes)
	}
	return consumers.FormatText(consumers.Annotate(old, changes, list))
}

func renderMarkdown(old *contract.Document, changes contract.Changes, list []consumers.Consumer) string {
	if list == nil {
		return contract.FormatMarkdown(changes)
	}
	return consumers.FormatMarkdown(consumers.Annotate(old, changes, list))
}
