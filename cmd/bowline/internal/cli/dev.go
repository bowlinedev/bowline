package cli

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bowlinedev/bowline/cmd/bowline/internal/analyzer"
	"github.com/bowlinedev/bowline/cmd/bowline/internal/watch"
	"github.com/bowlinedev/bowline/playground"
)

type DevOptions struct {
	Options
	Interval   time.Duration
	Changes    <-chan []watch.Change
	Stop       <-chan struct{}
	Color      bool
	Playground string
	Ready      chan<- string
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
	var latest atomic.Pointer[[]byte]
	if data, err := os.ReadFile(filepath.Join(opts.Dir, filepath.FromSlash(cfg.Contract))); err == nil {
		latest.Store(&data)
	}
	if opts.Playground != "" {
		stop, err := servePlayground(opts, cfg, &latest)
		if err != nil {
			fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
			return 1
		}
		defer stop()
	}
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
		files, targetDiags, err := render(doc, cfg, opts.Env)
		if err != nil {
			fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
			return
		}
		if len(targetDiags) > 0 {
			printColored(opts, targetDiags)
			return
		}
		written, err := writeFiles(opts.Dir, files)
		genDur := time.Since(begin)
		if err != nil {
			fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
			return
		}
		if data, ok := files[cfg.Contract]; ok {
			latest.Store(&data)
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

func servePlayground(opts DevOptions, cfg *Config, latest *atomic.Pointer[[]byte]) (func(), error) {
	listener, err := net.Listen("tcp", opts.Playground)
	if err != nil {
		return nil, err
	}
	var pgOpts []playground.Option
	if cfg.Dev != nil && cfg.Dev.App != "" {
		pgOpts = append(pgOpts, playground.WithUpstream(cfg.Dev.App))
	}
	handler := playground.NewDynamic(func() []byte {
		if data := latest.Load(); data != nil {
			return *data
		}
		return nil
	}, pgOpts...)
	srv := &http.Server{Handler: handler}
	go func() { _ = srv.Serve(listener) }()
	addr := listener.Addr().String()
	fmt.Fprintf(opts.Stdout, "playground at http://%s/\n", addr)
	if opts.Ready != nil {
		opts.Ready <- addr
	}
	return func() { srv.Close() }, nil
}
