package bowline

import (
	"context"
	"net/http"
)

type Next func(ctx context.Context, in any) (any, error)

type Middleware func(next Next) Next

type Call struct {
	Procedure *Procedure
	Request   *http.Request
}

type callKey struct{}

type callContext struct {
	context.Context
	call Call
}

func (c *callContext) Value(key any) any {
	if key == (callKey{}) {
		return &c.call
	}
	return c.Context.Value(key)
}

func withCall(ctx context.Context, c Call) context.Context {
	return &callContext{Context: ctx, call: c}
}

func CallFrom(ctx context.Context) *Call {
	c, _ := ctx.Value(callKey{}).(*Call)
	return c
}
