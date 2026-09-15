package main

import (
	"fmt"
	"io"
	"os"

	"github.com/bowlinedev/bowline"
	"github.com/bowlinedev/bowline/cmd/bowline/internal/cli"
)

const usage = `usage: bowline <command>

commands:
  gen        analyze the module and write the contract and every target
  check      verify the committed contract and targets are up to date
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
		return cli.Check(opts)
	case "version":
		fmt.Fprintf(stdout, "bowline %s\n", bowline.Version)
		return 0
	default:
		fmt.Fprintf(stderr, "bowline: unknown command %q\n", args[0])
		fmt.Fprint(stderr, usage)
		return 2
	}
}
