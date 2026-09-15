package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bowlinedev/bowline/contract"
	"github.com/bowlinedev/bowline/gateway"
	"github.com/bowlinedev/bowline/registry"
)

const (
	defaultGatewayConfig = "bowline.gateway.json"
	defaultGatewayListen = ":8090"
)

type GatewayOptions struct {
	Options
	Config string
	Out    string
	Token  string
	Ready  chan<- string
}

func Gateway(opts Options, args []string) int {
	if len(args) > 0 && args[0] == "compose" {
		return gatewayCompose(opts, args[1:])
	}
	g, code := parseGatewayFlags("gateway", opts, args, false)
	if code != 0 {
		return code
	}
	return GatewayServe(*g)
}

func parseGatewayFlags(name string, opts Options, args []string, wantOut bool) (*GatewayOptions, int) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(opts.Stderr)
	g := &GatewayOptions{Options: opts}
	flags.StringVar(&g.Config, "c", defaultGatewayConfig, "gateway configuration file")
	flags.StringVar(&g.Token, "token", "", "bearer token for registry upstreams; defaults to BOWLINE_REGISTRY_TOKEN")
	if wantOut {
		flags.StringVar(&g.Out, "o", "", "path of the composed contract to write")
	}
	if err := flags.Parse(args); err != nil {
		return nil, 2
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(opts.Stderr, "bowline: unexpected argument %q\n", flags.Arg(0))
		return nil, 2
	}
	if wantOut && g.Out == "" {
		fmt.Fprintln(opts.Stderr, "usage: bowline gateway compose [-c bowline.gateway.json] -o composed.contract.json")
		return nil, 2
	}
	if g.Token == "" {
		g.Token = os.Getenv("BOWLINE_REGISTRY_TOKEN")
	}
	return g, 0
}

func gatewayCompose(opts Options, args []string) int {
	g, code := parseGatewayFlags("gateway compose", opts, args, true)
	if code != 0 {
		return code
	}
	_, docs, exit := resolveGateway(*g)
	if exit != 0 {
		return exit
	}
	composed, diags := gateway.Compose(docs)
	if len(diags) > 0 {
		for _, d := range diags {
			fmt.Fprintf(g.Stderr, "bowline: %s\n", d)
		}
		return 1
	}
	data, err := composed.Marshal()
	if err != nil {
		fmt.Fprintf(g.Stderr, "bowline: %v\n", err)
		return 1
	}
	written, err := writeFiles(g.Dir, map[string][]byte{g.Out: data})
	if err != nil {
		fmt.Fprintf(g.Stderr, "bowline: %v\n", err)
		return 1
	}
	for _, rel := range written {
		fmt.Fprintf(g.Stdout, "wrote %s\n", rel)
	}
	if len(written) == 0 {
		fmt.Fprintf(g.Stdout, "unchanged %s\n", g.Out)
	}
	return 0
}

func GatewayServe(opts GatewayOptions) int {
	cfg, docs, exit := resolveGateway(opts)
	if exit != 0 {
		return exit
	}
	g, err := gateway.New(cfg, docs)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	addr := cfg.Listen
	if addr == "" {
		addr = defaultGatewayListen
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	bound := listener.Addr().String()
	fmt.Fprintf(opts.Stdout, "bowline gateway: %d procedures from %d service(s) at http://%s%s\n",
		len(g.Contract().Procedures), len(cfg.Services), bound, "/"+strings.Trim(cfg.Prefix, "/"))
	for _, line := range readinessLines(g.Ready(context.Background())) {
		fmt.Fprintln(opts.Stdout, line)
	}
	if opts.Ready != nil {
		opts.Ready <- bound
	}
	srv := &http.Server{Handler: g.Handler()}
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

func readinessLines(probes map[string]error) []string {
	names := make([]string, 0, len(probes))
	for name := range probes {
		names = append(names, name)
	}
	sort.Strings(names)
	lines := make([]string, 0, len(names))
	for _, name := range names {
		if err := probes[name]; err != nil {
			lines = append(lines, fmt.Sprintf("  %-12s unready: %v", name, err))
			continue
		}
		lines = append(lines, fmt.Sprintf("  %-12s ready", name))
	}
	return lines
}

func resolveGateway(opts GatewayOptions) (*gateway.Config, map[string]*contract.Document, int) {
	path := opts.Config
	if !filepath.IsAbs(path) {
		path = filepath.Join(opts.Dir, filepath.FromSlash(path))
	}
	cfg, err := gateway.LoadConfig(path)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return nil, nil, 1
	}
	base := filepath.Dir(path)
	for name, up := range cfg.Services {
		if up.Contract != "" && !filepath.IsAbs(up.Contract) {
			up.Contract = filepath.Join(base, filepath.FromSlash(up.Contract))
			cfg.Services[name] = up
		}
	}
	docs, err := gateway.ResolveWith(context.Background(), cfg, registryFetcher{token: opts.Token})
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return nil, nil, 1
	}
	return cfg, docs, 0
}

type registryFetcher struct {
	token string
}

func (f registryFetcher) Fetch(ctx context.Context, registryURL, service, version string) ([]byte, error) {
	client := &registry.Client{URL: registryURL, Token: f.token}
	var doc *contract.Document
	var err error
	if strings.HasPrefix(version, "sha256:") {
		doc, err = client.Version(ctx, service, version)
	} else {
		doc, err = client.Latest(ctx, service, version)
	}
	if err != nil {
		return nil, err
	}
	return doc.Marshal()
}
