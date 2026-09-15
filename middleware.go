package bowline

import (
	"context"
	"net/http"
)

type Next func(ctx context.Context, in any) (any, error)

type Middleware func(next Next) Next

type Call struct {
	Procedure Procedure
	Request   *http.Request
}

type callKey struct{}

func withCall(ctx context.Context, c *Call) context.Context {
	return context.WithValue(ctx, callKey{}, c)
}

func CallFrom(ctx context.Context) *Call {
	c, _ := ctx.Value(callKey{}).(*Call)
	return c
}
