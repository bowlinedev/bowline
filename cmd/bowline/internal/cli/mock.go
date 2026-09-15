package cli

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/bowlinedev/bowline/cmd/bowline/internal/mock"
	"github.com/bowlinedev/bowline/contract"
	"github.com/bowlinedev/bowline/playground"
)

type MockOptions struct {
	Options
	Addr       string
	Seed       uint64
	Record     string
	Replay     bool
	Strict     bool
	Playground bool
	Fixtures   string
	Ready      chan<- string
}

func parseMockFlags(args []string, stderr interface{ Write([]byte) (int, error) }) (*MockOptions, error) {
	flags := flag.NewFlagSet("mock", flag.ContinueOnError)
	flags.SetOutput(stderr)
	m := &MockOptions{}
	flags.StringVar(&m.Addr, "addr", ":8090", "address to listen on")
	flags.Uint64Var(&m.Seed, "seed", 1, "seed for generated data")
	flags.StringVar(&m.Record, "record", "", "proxy every request to this upstream URL and record fixtures")
	flags.BoolVar(&m.Replay, "replay", false, "serve recorded fixtures before generated data")
	flags.BoolVar(&m.Strict, "strict", false, "with --replay, answer UNIMPLEMENTED instead of generating on a miss")
	flags.StringVar(&m.Fixtures, "fixtures", "mocks", "directory of recorded fixtures")
	noPlayground := flags.Bool("no-playground", false, "do not serve the playground at /_playground/")
	if err := flags.Parse(args); err != nil {
		return nil, err
	}
	m.Playground = !*noPlayground
	if m.Record != "" && m.Replay {
		return nil, errors.New("--record and --replay are mutually exclusive")
	}
	if m.Strict && !m.Replay {
		return nil, errors.New("--strict needs --replay")
	}
	return m, nil
}

func loadContractDocument(dir string) (*contract.Document, []byte, error) {
	cfg, err := LoadConfig(dir)
	if err != nil {
		return nil, nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(cfg.Contract)))
	if err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w; run bowline gen first", cfg.Contract, err)
	}
	doc, err := contract.Parse(data)
	if err != nil {
		return nil, nil, err
	}
	return doc, data, nil
}

func Mock(opts MockOptions) int {
	doc, data, err := loadContractDocument(opts.Dir)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	var api http.Handler
	mode := "generated data"
	switch {
	case opts.Record != "":
		upstream, err := url.Parse(opts.Record)
		if err != nil || upstream.Scheme == "" || upstream.Host == "" {
			fmt.Fprintf(opts.Stderr, "bowline: --record needs an absolute URL such as http://localhost:8080/api\n")
			return 2
		}
		api = mock.Recorder(doc, upstream, filepath.Join(opts.Dir, filepath.FromSlash(opts.Fixtures)), nil)
		mode = "recording from " + opts.Record + " into " + opts.Fixtures
	case opts.Replay:
		dir := filepath.Join(opts.Dir, filepath.FromSlash(opts.Fixtures))
		if _, err := os.Stat(dir); err != nil {
			fmt.Fprintf(opts.Stderr, "bowline: fixtures directory %s not found; run bowline mock --record first\n", opts.Fixtures)
			return 1
		}
		api = mock.New(doc, mock.Options{Seed: opts.Seed, Fixtures: os.DirFS(dir), Strict: opts.Strict})
		mode = "replaying " + opts.Fixtures
		if opts.Strict {
			mode += " (strict)"
		}
	default:
		api = mock.New(doc, mock.Options{Seed: opts.Seed})
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", api))
	mux.Handle("/", api)
	if opts.Playground {
		mux.Handle("/_playground/", http.StripPrefix("/_playground", playground.New(data, playground.WithUpstream("/api"), playground.WithTitle("Bowline mock"))))
	}
	listener, err := net.Listen("tcp", opts.Addr)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	served := 0
	for _, p := range doc.Procedures {
		if p.Kind == "query" || p.Kind == "mutation" {
			served++
		}
	}
	addr := listener.Addr().String()
	fmt.Fprintf(opts.Stdout, "bowline mock: %d procedures at http://%s/api (%s)\n", served, addr, mode)
	if opts.Playground {
		fmt.Fprintf(opts.Stdout, "bowline mock: playground at http://%s/_playground/\n", addr)
	}
	if opts.Ready != nil {
		opts.Ready <- addr
	}
	srv := &http.Server{Handler: mux}
	if opts.Stop != nil {
		go func() {
			<-opts.Stop
			srv.Close()
		}()
	}
	if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	return 0
}

func MockCommand(opts Options, args []string) int {
	m, err := parseMockFlags(args, opts.Stderr)
	if err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		}
		return 2
	}
	m.Options = opts
	return Mock(*m)
}
