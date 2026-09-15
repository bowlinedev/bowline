package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/bowlinedev/bowline"
	"github.com/bowlinedev/bowline/cmd/bowline/internal/cli"
)

const usage = `usage: bowline <command>

commands:
  gen        analyze the module and write the contract and every target
  check      verify the committed contract and targets are up to date
             --against <git-ref> diffs the contract against that ref instead
             and fails on breaking changes unless --allow-breaking is set
  export openapi [-o path]
             write an OpenAPI 3.1 document derived from the contract
  export tools [--format anthropic|openai|json-schema] [--scope S] [--read-only] [--out path]
             write LLM tool definitions for every exposed procedure
  mcp --url <base> [--listen addr] [--scope S] [--read-only] [--rate N --burst B] [--header "K: v"]
             serve the exposed procedures to MCP clients over stdio or HTTP
  migrate-contract [path]
             rewrite a contract document from an older format version
  diff <old> <new> [--format text|markdown|json]
             list semantic changes between two contract documents
  version    print the bowline version
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "bowline: %v\n", err)
		return 1
	}
	opts := cli.Options{Dir: dir, Stdout: stdout, Stderr: stderr}
	switch args[0] {
	case "gen":
		return cli.Gen(opts)
	case "check":
		return cli.Check(opts, args[1:])
	case "export":
		return cli.Export(opts, args[1:])
	case "migrate-contract":
		return cli.Migrate(opts, args[1:])
	case "diff":
		return cli.DiffCommand(opts, args[1:])
	case "mcp":
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		opts.Stdin = os.Stdin
		opts.Stop = ctx.Done()
		return cli.MCP(opts, args[1:])
	case "dev":
		stop := make(chan struct{})
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		go func() {
			<-ctx.Done()
			close(stop)
		}()
		color := os.Getenv("NO_COLOR") == "" && isTerminal(os.Stderr)
		return cli.Dev(cli.DevOptions{Options: opts, Stop: stop, Color: color})
	case "version":
		fmt.Fprintf(stdout, "bowline %s\n", bowline.Version)
		return 0
	default:
		fmt.Fprintf(stderr, "bowline: unknown command %q\n", args[0])
		fmt.Fprint(stderr, usage)
		return 2
	}
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
