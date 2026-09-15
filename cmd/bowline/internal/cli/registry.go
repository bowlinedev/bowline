package cli

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bowlinedev/bowline/registry"
)

const registryUsage = "usage: bowline registry serve --store <dir> [--listen :8095] [--token T]... [--ui=false]"

type RegistryOptions struct {
	Options
	Store  string
	Listen string
	Tokens []string
	UI     bool
	Ready  chan<- string
}

func Registry(opts Options, args []string) int {
	if len(args) == 0 || args[0] != "serve" {
		fmt.Fprintln(opts.Stderr, registryUsage)
		return 2
	}
	flags := flag.NewFlagSet("registry serve", flag.ContinueOnError)
	flags.SetOutput(opts.Stderr)
	r := &RegistryOptions{Options: opts}
	flags.StringVar(&r.Store, "store", "", "directory holding the registry records")
	flags.StringVar(&r.Listen, "listen", ":8095", "address to listen on")
	flags.BoolVar(&r.UI, "ui", true, "serve the browser UI at /")
	var tokens scopeList
	flags.Var(&tokens, "token", "bearer token accepted for writes; repeatable")
	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}
	if r.Store == "" || flags.NArg() > 0 {
		fmt.Fprintln(opts.Stderr, registryUsage)
		return 2
	}
	r.Tokens = tokens
	if len(r.Tokens) == 0 {
		r.Tokens = envTokens()
	}
	return RegistryServe(*r)
}

func envTokens() []string {
	raw := os.Getenv("BOWLINE_REGISTRY_TOKEN")
	if raw == "" {
		return nil
	}
	var tokens []string
	for _, token := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(token); trimmed != "" {
			tokens = append(tokens, trimmed)
		}
	}
	return tokens
}

func RegistryServe(opts RegistryOptions) int {
	root := opts.Store
	if !filepath.IsAbs(root) {
		root = filepath.Join(opts.Dir, filepath.FromSlash(root))
	}
	store, err := registry.NewFileStore(root)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	settings := registry.Options{Tokens: opts.Tokens}
	if opts.UI {
		settings.UI = registry.UI()
	}
	server := registry.NewServer(store, settings)
	listener, err := net.Listen("tcp", opts.Listen)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	addr := listener.Addr().String()
	mode := fmt.Sprintf("%d write token(s)", len(opts.Tokens))
	if len(opts.Tokens) == 0 {
		mode = "read only; no write tokens configured"
	}
	fmt.Fprintf(opts.Stdout, "bowline registry: %s at http://%s (%s)\n", opts.Store, addr, mode)
	if opts.Ready != nil {
		opts.Ready <- addr
	}
	srv := &http.Server{Handler: server.Handler()}
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
