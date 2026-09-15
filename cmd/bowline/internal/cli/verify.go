package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bowlinedev/bowline/cmd/bowline/internal/consumers"
)

const DefaultConsumersDir = "contracts/consumers"

func VerifyConsumers(opts Options, args []string) int {
	dir := DefaultConsumersDir
	if len(args) > 1 || (len(args) == 1 && len(args[0]) > 0 && args[0][0] == '-') {
		fmt.Fprintln(opts.Stderr, "usage: bowline verify-consumers [dir]")
		return 2
	}
	if len(args) == 1 {
		dir = args[0]
	}
	doc, _, err := loadContractDocument(opts.Dir)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	list, err := consumers.Load(filepath.Join(opts.Dir, filepath.FromSlash(dir)))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(opts.Stderr, "bowline: no consumer contracts under %s\n", dir)
			return 1
		}
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	if len(list) == 0 {
		fmt.Fprintf(opts.Stderr, "bowline: no consumer contracts under %s\n", dir)
		return 1
	}
	problems := consumers.Verify(doc, list)
	report := consumers.Report(problems, list)
	if len(problems) > 0 {
		fmt.Fprint(opts.Stderr, report)
		return 1
	}
	fmt.Fprint(opts.Stdout, report)
	return 0
}
