package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"github.com/bowlinedev/bowline/cmd/bowline/internal/eval"
	"github.com/bowlinedev/bowline/cmd/bowline/internal/mcpproxy"
	"github.com/bowlinedev/bowline/cmd/bowline/internal/mock"
	"github.com/bowlinedev/bowline/cmd/bowline/internal/tools"
	"github.com/bowlinedev/bowline/contract"
)

const evalUsage = "usage: bowline eval record [--backend mock|replay|url] [--url U] --script path --out path [--volatile key]... [--header \"K: v\"]... [--agent]\n       bowline eval replay <recording> [--backend mock|replay|url] [--url U] [--strict-messages] [--header \"K: v\"]..."

func Eval(opts Options, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(opts.Stderr, evalUsage)
		return 2
	}
	switch args[0] {
	case "record":
		return evalRecord(opts, args[1:])
	case "replay":
		return evalReplay(opts, args[1:])
	default:
		fmt.Fprintf(opts.Stderr, "bowline: unknown eval subcommand %q\n%s\n", args[0], evalUsage)
		return 2
	}
}

type evalBackend struct {
	kind     string
	url      string
	fixtures string
	seed     uint64
}

func (b *evalBackend) validate() error {
	switch b.kind {
	case "mock", "replay":
		if b.url != "" {
			return errors.New("--url is only used with --backend url")
		}
	case "url":
		if b.url == "" {
			return errors.New("--backend url needs --url")
		}
	default:
		return fmt.Errorf("unknown backend %q; use mock, replay, or url", b.kind)
	}
	return nil
}

func addBackendFlags(flags *flag.FlagSet, b *evalBackend) {
	flags.StringVar(&b.kind, "backend", "mock", "mock (generated data, no provider), replay (recorded fixtures), or url (a running handler)")
	flags.StringVar(&b.url, "url", "", "base URL of the running Bowline handler for --backend url")
	flags.StringVar(&b.fixtures, "fixtures", "mocks", "fixture directory for --backend replay")
	flags.Uint64Var(&b.seed, "seed", 1, "seed for --backend mock")
}

func evalRunner(opts Options, backend evalBackend, headers http.Header) (*eval.Runner, func(), error) {
	cfg, err := LoadConfig(opts.Dir)
	if err != nil {
		return nil, nil, err
	}
	data, err := os.ReadFile(filepath.Join(opts.Dir, filepath.FromSlash(cfg.Contract)))
	if err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w; run bowline gen first", cfg.Contract, err)
	}
	doc, err := contract.Parse(data)
	if err != nil {
		return nil, nil, err
	}
	list, err := tools.FromContract(doc, tools.Filter{})
	if err != nil {
		return nil, nil, err
	}
	stop := func() {}
	target := backend.url
	switch backend.kind {
	case "mock":
		srv := httptest.NewServer(mock.New(doc, mock.Options{Seed: backend.seed, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}))
		target, stop = srv.URL, srv.Close
	case "replay":
		dir := filepath.Join(opts.Dir, filepath.FromSlash(backend.fixtures))
		if _, err := os.Stat(dir); err != nil {
			return nil, nil, fmt.Errorf("fixtures directory %s not found; run bowline mock --record first", backend.fixtures)
		}
		srv := httptest.NewServer(mock.New(doc, mock.Options{Seed: backend.seed, Fixtures: os.DirFS(dir), Strict: true, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}))
		target, stop = srv.URL, srv.Close
	}
	return &eval.Runner{Dispatcher: mcpproxy.New(target, headers, nil), Tools: list, Contract: doc.Hash}, stop, nil
}

func evalRecord(opts Options, args []string) int {
	flags := flag.NewFlagSet("eval record", flag.ContinueOnError)
	flags.SetOutput(opts.Stderr)
	var backend evalBackend
	addBackendFlags(flags, &backend)
	script := flags.String("script", "", "path to a script of calls")
	out := flags.String("out", "", "path of the recording to write")
	agent := flags.Bool("agent", false, "read JSON lines of calls from stdin instead of --script")
	var volatile scopeList
	flags.Var(&volatile, "volatile", "key removed from outputs at any depth before comparing; repeatable")
	var headers headerList
	flags.Var(&headers, "header", "static header sent upstream as \"Name: value\"; repeatable")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *out == "" || (*script == "") == !*agent {
		fmt.Fprintln(opts.Stderr, evalUsage)
		return 2
	}
	if err := backend.validate(); err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n%s\n", err, evalUsage)
		return 2
	}
	var calls []eval.Call
	if *agent {
		stdin := opts.Stdin
		if stdin == nil {
			stdin = os.Stdin
		}
		data, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
			return 1
		}
		calls, err = eval.ParseCalls(data)
		if err != nil {
			fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
			return 1
		}
	} else {
		data, err := os.ReadFile(filepath.Join(opts.Dir, filepath.FromSlash(*script)))
		if err != nil {
			fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
			return 1
		}
		parsed, err := eval.ParseScript(data)
		if err != nil {
			fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
			return 1
		}
		calls = parsed.Calls
		volatile = append(volatile, parsed.Volatile...)
	}
	runner, stop, err := evalRunner(opts, backend, http.Header(headers))
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	defer stop()
	rec, err := runner.Record(context.Background(), calls, volatile, time.Now)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	data, err := eval.Encode(rec)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	target := filepath.Join(opts.Dir, filepath.FromSlash(*out))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	if err := os.WriteFile(target, data, 0o644); err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	errorsSeen := 0
	for _, step := range rec.Steps {
		if step.IsError {
			errorsSeen++
		}
	}
	fmt.Fprintf(opts.Stdout, "recorded %d steps (%d errors) to %s\n", len(rec.Steps), errorsSeen, *out)
	return 0
}

func evalReplay(opts Options, args []string) int {
	if len(args) == 0 || len(args[0]) == 0 || args[0][0] == '-' {
		fmt.Fprintln(opts.Stderr, evalUsage)
		return 2
	}
	recordingPath := args[0]
	flags := flag.NewFlagSet("eval replay", flag.ContinueOnError)
	flags.SetOutput(opts.Stderr)
	var backend evalBackend
	addBackendFlags(flags, &backend)
	strict := flags.Bool("strict-messages", false, "compare error messages as well as codes")
	var headers headerList
	flags.Var(&headers, "header", "static header sent upstream as \"Name: value\"; repeatable")
	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}
	if err := backend.validate(); err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n%s\n", err, evalUsage)
		return 2
	}
	data, err := os.ReadFile(filepath.Join(opts.Dir, filepath.FromSlash(recordingPath)))
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	rec, err := eval.Parse(data)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	runner, stop, err := evalRunner(opts, backend, http.Header(headers))
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	defer stop()
	mismatches, err := runner.Replay(context.Background(), rec, *strict)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	if len(mismatches) == 0 {
		fmt.Fprintf(opts.Stdout, "ok        %d steps match %s\n", len(rec.Steps), recordingPath)
		return 0
	}
	for _, m := range mismatches {
		fmt.Fprintf(opts.Stderr, "mismatch  %s\n", m)
	}
	fmt.Fprintf(opts.Stderr, "bowline: %d mismatch(es) replaying %s\n", len(mismatches), recordingPath)
	return 1
}
