package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/bowlinedev/bowline"
	"github.com/bowlinedev/bowline/cmd/bowline/internal/cli"
)

const usage = `usage: bowline <command>

commands:
  gen [--from contract.json]
             analyze the module and write the contract and every target, or
             render the targets from an existing document such as a composed one
  dev [--playground addr]
             regenerate on every save; optionally serve the playground for the live contract
  check [--json]
             verify the committed contract and targets are up to date
             --against <ref> diffs the contract against that ref instead
             and fails on breaking changes unless --allow-breaking is set;
             --consumers dir annotates the report with recorded consumers
             --registry URL --service NAME [--strict] [--token T] asks a
             registry which consumers a change would break and fails when any would
  export openapi [-o path]
             write an OpenAPI 3.1 document derived from the contract
  export tools [--format anthropic|openai|json-schema] [--scope S] [--read-only] [--out path]
             write LLM tool definitions for every exposed procedure
  mcp --url <base> [--stdio] [--listen addr] [--scope S] [--read-only] [--rate N --burst B] [--header "K: v"]
             serve the exposed procedures to MCP clients over stdio or HTTP
  verify-consumers [dir]
             check recorded consumer interactions against the current contract
  mock [--addr :8090] [--seed N] [--record URL] [--replay] [--strict] [--fixtures dir] [--no-playground]
             serve generated or recorded responses from the contract alone
  eval record --script <path> --out <path> [--backend mock|replay|url] [--url <base>] [--fixtures dir] [--seed N] [--volatile key] [--header "K: v"] [--agent]
             run scripted tool calls and write a recording; the default backend is the in-process mock
  eval replay <recording> [--backend mock|replay|url] [--url <base>] [--fixtures dir] [--seed N] [--strict-messages] [--header "K: v"]
             re-run a recording and fail on any changed result
  gateway [-c bowline.gateway.json] [--token T]
             compose the configured services and proxy calls to their upstreams
  gateway compose [-c bowline.gateway.json] [--token T] -o composed.contract.json
             write the composed contract without serving
  registry serve --store <dir> [--listen :8095] [--token T] [--ui=false]
             serve the contract registry over HTTP from a directory of records
  publish --registry URL --service NAME [--tag main] [--ref SHA] [--contract PATH] [--token T]
             publish this module's contract as a version of a service
  publish --registry URL --consumer NAME --provider SERVICE --usage PATH [--token T]
             publish what a consumer uses of a service
  certify --target <name> --generator <command> [--config certify.json] [--out docs/certified.md] [--report]
             run a generator against the fidelity corpus and the target's toolchain
  migrate-contract [path]
             rewrite a contract document from an older format version
  diff <old> <new> [--format text|markdown|json] [--json] [--consumers dir]
             list semantic changes between two contract documents, naming affected consumers
  version    print the bowline version

exit codes:
  0 success; 1 the command ran and the answer is no; 2 the command line is wrong
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
		return cli.Gen(opts, args[1:])
	case "check":
		return cli.Check(opts, args[1:])
	case "export":
		return cli.Export(opts, args[1:])
	case "certify":
		return cli.Certify(opts, args[1:])
	case "migrate-contract":
		return cli.Migrate(opts, args[1:])
	case "diff":
		return cli.DiffCommand(opts, args[1:])
	case "verify-consumers":
		return cli.VerifyConsumers(opts, args[1:])
	case "mock":
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		opts.Stop = ctx.Done()
		return cli.MockCommand(opts, args[1:])
	case "eval":
		opts.Stdin = os.Stdin
		return cli.Eval(opts, args[1:])
	case "gateway":
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		opts.Stop = ctx.Done()
		return cli.Gateway(opts, args[1:])
	case "registry":
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		opts.Stop = ctx.Done()
		return cli.Registry(opts, args[1:])
	case "publish":
		return cli.Publish(opts, args[1:])
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
		devFlags := flag.NewFlagSet("dev", flag.ContinueOnError)
		devFlags.SetOutput(stderr)
		playgroundAddr := devFlags.String("playground", "", "serve the playground on this address, for example 127.0.0.1:8091")
		if err := devFlags.Parse(args[1:]); err != nil {
			return 2
		}
		return cli.Dev(cli.DevOptions{Options: opts, Stop: stop, Color: color, Playground: *playgroundAddr})
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
