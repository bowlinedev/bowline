package bowline

import (
	"context"
)

type TypedNext[In, Out any] func(ctx context.Context, in In) (Out, error)

func Typed[In, Out any](fn func(ctx context.Context, in In, next TypedNext[In, Out]) (Out, error)) Middleware {
	return func(next Next) Next {
		return func(ctx context.Context, in any) (any, error) {
			typed, ok := in.(In)
			pointer := false
			if !ok {
				ptr, isPtr := in.(*In)
				if !isPtr {
					var want In
					return nil, Errorf(Internal, "typed middleware expected %T but the procedure takes %T", want, in)
				}
				typed, pointer = *ptr, true
			}
			out, err := fn(ctx, typed, func(ctx context.Context, in In) (Out, error) {
				var forward any = in
				if pointer {
					forward = &in
				}
				raw, err := next(ctx, forward)
				if err != nil {
					var zero Out
					return zero, err
				}
				if value, ok := raw.(Out); ok {
					return value, nil
				}
				if ptr, ok := raw.(*Out); ok {
					return *ptr, nil
				}
				var want Out
				return want, Errorf(Internal, "typed middleware expected the procedure to return %T but it returned %T", want, raw)
			})
			if err != nil {
				return nil, err
			}
			return out, nil
		}
	}
}

type Observer interface {
	HandlerReady(procedures []*Procedure)
	CallStarted(ctx context.Context, call *Call) context.Context
	CallFinished(ctx context.Context, call *Call, err error)
}

func Observe(o Observer) HandlerOption {
	return func(h *handler) {
		if o != nil {
			h.observers = append(h.observers, o)
		}
	}
}

type observerFuncs struct {
	ready    func([]*Procedure)
	started  func(context.Context, *Call) context.Context
	finished func(context.Context, *Call, error)
}

func (o observerFuncs) HandlerReady(procedures []*Procedure) {
	if o.ready != nil {
		o.ready(procedures)
	}
}

func (o observerFuncs) CallStarted(ctx context.Context, call *Call) context.Context {
	if o.started != nil {
		return o.started(ctx, call)
	}
	return ctx
}

func (o observerFuncs) CallFinished(ctx context.Context, call *Call, err error) {
	if o.finished != nil {
		o.finished(ctx, call, err)
	}
}

func ObserveFunc(started func(ctx context.Context, call *Call) context.Context, finished func(ctx context.Context, call *Call, err error)) HandlerOption {
	return Observe(observerFuncs{started: started, finished: finished})
}

func OnHandlerReady(fn func(procedures []*Procedure)) HandlerOption {
	return Observe(observerFuncs{ready: fn})
}
