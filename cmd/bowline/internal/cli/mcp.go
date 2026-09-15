package cli

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bowlinedev/bowline/cmd/bowline/internal/mcpproxy"
	"github.com/bowlinedev/bowline/contract"
	"github.com/bowlinedev/bowline/mcp"
)

const maxStdioLine = 4 << 20

type headerList http.Header

func (h *headerList) String() string { return "" }

func (h *headerList) Set(v string) error {
	name, value, ok := strings.Cut(v, ":")
	if !ok || strings.TrimSpace(name) == "" {
		return fmt.Errorf("header %q must look like \"Name: value\"", v)
	}
	if *h == nil {
		*h = headerList{}
	}
	http.Header(*h).Add(strings.TrimSpace(name), strings.TrimSpace(value))
	return nil
}

type mcpSettings struct {
	url      string
	listen   string
	stdio    bool
	readOnly bool
	rate     int
	burst    int
	scopes   scopeList
	headers  headerList
}

func parseMCPFlags(args []string, stderr io.Writer) (*mcpSettings, error) {
	flags := flag.NewFlagSet("mcp", flag.ContinueOnError)
	flags.SetOutput(stderr)
	s := &mcpSettings{}
	flags.StringVar(&s.url, "url", "", "base URL of the running Bowline handler, for example http://localhost:8080/api")
	flags.BoolVar(&s.stdio, "stdio", false, "serve on stdin and stdout (the default)")
	flags.StringVar(&s.listen, "listen", "", "serve streamable HTTP on this address instead of stdio")
	flags.BoolVar(&s.readOnly, "read-only", false, "expose only tools with the read-only hint")
	flags.IntVar(&s.rate, "rate", 0, "calls per minute per client; 0 disables the limit")
	flags.IntVar(&s.burst, "burst", 0, "burst size for --rate; defaults to --rate")
	flags.Var(&s.scopes, "scope", "expose only tools with this scope; repeatable")
	flags.Var(&s.headers, "header", "static header sent upstream as \"Name: value\"; repeatable")
	if err := flags.Parse(args); err != nil {
		return nil, err
	}
	if s.url == "" {
		flags.Usage()
		return nil, errors.New("--url is required")
	}
	if s.stdio && s.listen != "" {
		return nil, errors.New("--stdio and --listen are mutually exclusive")
	}
	if s.rate > 0 && s.burst == 0 {
		s.burst = s.rate
	}
	return s, nil
}

func (s *mcpSettings) options() []mcp.Option {
	var opts []mcp.Option
	if len(s.scopes) > 0 {
		opts = append(opts, mcp.Scopes(s.scopes...))
	}
	if s.readOnly {
		opts = append(opts, mcp.ReadOnly())
	}
	if s.rate > 0 {
		opts = append(opts, mcp.RateLimit(s.rate, s.burst))
	}
	return opts
}

func loadTools(dir string) ([]mcp.Tool, error) {
	cfg, err := LoadConfig(dir)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(cfg.Contract)))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w; run bowline gen first", cfg.Contract, err)
	}
	doc, err := contract.Parse(data)
	if err != nil {
		return nil, err
	}
	source, err := mcp.SchemasFromContract(doc)
	if err != nil {
		return nil, err
	}
	return mcp.ToolsFromContract(doc, source)
}

func buildMCPServer(opts Options, s *mcpSettings) (*mcp.Server, []mcp.Option, error) {
	tools, err := loadTools(opts.Dir)
	if err != nil {
		return nil, nil, err
	}
	dispatcher := mcpproxy.New(s.url, http.Header(s.headers), nil)
	options := s.options()
	return mcp.NewServer(tools, dispatcher, options...), options, nil
}

func MCP(opts Options, args []string) int {
	s, err := parseMCPFlags(args, opts.Stderr)
	if err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		}
		return 2
	}
	server, options, err := buildMCPServer(opts, s)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	if s.listen != "" {
		return serveMCPHTTP(opts, s.listen, server, options)
	}
	stdin := opts.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	if err := serveMCPStdio(context.Background(), stdin, opts.Stdout, server); err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	return 0
}

func serveMCPStdio(ctx context.Context, in io.Reader, out io.Writer, server *mcp.Server) error {
	reader := bufio.NewReaderSize(in, 64<<10)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > maxStdioLine {
			return fmt.Errorf("message exceeds %d bytes", maxStdioLine)
		}
		trimmed := strings.TrimSpace(string(line))
		if trimmed != "" {
			resp, handleErr := server.Handle(ctx, []byte(trimmed))
			if handleErr != nil {
				return handleErr
			}
			if resp != nil {
				if _, werr := fmt.Fprintf(out, "%s\n", resp); werr != nil {
					return werr
				}
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

func serveMCPHTTP(opts Options, addr string, server *mcp.Server, options []mcp.Option) int {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	fmt.Fprintf(opts.Stderr, "bowline: mcp listening on http://%s\n", listener.Addr())
	srv := &http.Server{Handler: mcp.Serve(server, options...)}
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
