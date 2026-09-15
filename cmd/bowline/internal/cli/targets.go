package cli

import (
	"github.com/bowlinedev/bowline/cmd/bowline/internal/gen/dart"
	"github.com/bowlinedev/bowline/cmd/bowline/internal/gen/goclient"
	"github.com/bowlinedev/bowline/cmd/bowline/internal/gen/ts"
)

func init() {
	Generators["ts"] = ts.Generator{}
	Generators["go"] = goclient.Generator{}
	Generators["dart"] = dart.Generator{}
}
