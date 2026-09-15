package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bowlinedev/bowline/cmd/bowline/internal/analyzer"
	"github.com/bowlinedev/bowline/cmd/bowline/internal/watch"
)

type DevOptions struct {
	Options
	Interval time.Duration
	Changes  <-chan []watch.Change
	Stop     <-chan struct{}
	Color    bool
}

func Dev(opts DevOptions) int {
	cfg, err := LoadConfig(opts.Dir)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	for name := range cfg.Targets {
		if _, ok := Generators[name]; !ok {
			fmt.Fprintf(opts.Stderr, "bowline: unknown target %q; available: %s\n", name, availableTargets())
			return 1
		}
	}
	env := opts.Env
	if env == nil {
		env = os.Environ()
	}
	start := time.Now()
	session, err := analyzer.NewSession(opts.Dir, env)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	fmt.Fprintf(opts.Stdout, "loaded %d packages in %s\n", len(session.Program().Pkgs), time.Since(start).Round(time.Millisecond))
	lastHash := ""
	regenerate := func(stats analyzer.Stats, typeErrs []analyzer.Diagnostic) {
		if len(typeErrs) > 0 {
			printColored(opts, typeErrs)
			return
		}
		begin := time.Now()
		doc, diags := analyzer.Analyze(session.Program(), cfg.Entry)
		analyzeDur := time.Since(begin)
		if len(diags) > 0 {
			printColored(opts, diags)
			return
		}
		if doc.Hash == lastHash {
			fmt.Fprintf(opts.Stdout, "no contract change (%s)\n", (stats.Parse + stats.Check + analyzeDur).Round(time.Millisecond))
			return
		}
		begin = time.Now()
		files, err := render(doc, cfg)
		if err != nil {
			fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
			return
		}
		written, err := writeFiles(opts.Dir, files)
		genDur := time.Since(begin)
		if err != nil {
			fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
			return
		}
		lastHash = doc.Hash
		total := stats.Parse + stats.Check + analyzeDur + genDur
		for _, rel := range written {
			fmt.Fprintf(opts.Stdout, "wrote %s\n", rel)
		}
		fmt.Fprintf(opts.Stdout, "regenerated in %s (typecheck %s, analyze %s, gen %s)\n",
			total.Round(time.Millisecond), (stats.Parse + stats.Check).Round(time.Millisecond), analyzeDur.Round(time.Millisecond), genDur.Round(time.Millisecond))
	}
	regenerate(analyzer.Stats{}, nil)
	changes := opts.Changes
	if changes == nil {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ch := make(chan []watch.Change, 1)
		interval := opts.Interval
		if interval == 0 {
			interval = 200 * time.Millisecond
		}
		go watch.Run(ctx, opts.Dir, interval, func(c []watch.Change) { ch <- c })
		changes = ch
	}
	fmt.Fprintln(opts.Stdout, "watching for changes")
	for {
		select {
		case <-opts.Stop:
			return 0
		case batch := <-changes:
			var paths []string
			full := false
			for _, c := range batch {
				if c.Removed {
					full = true
				}
				paths = append(paths, c.Path)
			}
			var stats analyzer.Stats
			var typeErrs []analyzer.Diagnostic
			var err error
			if full {
				err = session.Reload()
				stats.Full = true
			} else {
				stats, typeErrs, err = session.Update(paths)
			}
			if err != nil {
				fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
				continue
			}
			regenerate(stats, typeErrs)
		}
	}
}

func printColored(opts DevOptions, diags []analyzer.Diagnostic) {
	red, reset := "", ""
	if opts.Color {
		red, reset = "\x1b[31m", "\x1b[0m"
	}
	for _, d := range diags {
		fmt.Fprintf(opts.Stderr, "%s%s%s\n", red, strings.TrimSpace(d.String()), reset)
	}
	fmt.Fprintf(opts.Stderr, "bowline: %d problem(s); waiting for changes\n", len(diags))
}
