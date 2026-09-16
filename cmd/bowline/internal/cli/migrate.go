package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bowlinedev/bowline/contract"
)

func Migrate(opts Options, args []string) int {
	var rel string
	if len(args) > 0 {
		rel = args[0]
	} else {
		cfg, err := LoadConfig(opts.Dir)
		if err != nil {
			fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
			return 1
		}
		rel = cfg.Contract
	}
	path := filepath.Join(opts.Dir, filepath.FromSlash(rel))
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	migrated, err := contract.Migrate(data)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	if bytes.Equal(data, migrated) {
		fmt.Fprintf(opts.Stdout, "unchanged %s\n", rel)
		return 0
	}
	if err := os.WriteFile(path, migrated, 0o644); err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	fmt.Fprintf(opts.Stdout, "migrated %s to %s\n", rel, contract.Version)
	return 0
}
