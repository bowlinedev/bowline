package cli

import "github.com/bowlinedev/bowline/cmd/bowline/internal/gen/ts"

func init() {
	Generators["ts"] = ts.Generator{}
}
