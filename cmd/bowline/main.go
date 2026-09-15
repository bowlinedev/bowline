package main

import (
	"fmt"
	"io"
	"os"

	"github.com/bowlinedev/bowline"
)

const usage = `usage: bowline <command>

commands:
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
	switch args[0] {
	case "version":
		fmt.Fprintf(stdout, "bowline %s\n", bowline.Version)
		return 0
	default:
		fmt.Fprintf(stderr, "bowline: unknown command %q\n", args[0])
		fmt.Fprint(stderr, usage)
		return 2
	}
}
