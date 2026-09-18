package bowline

import (
	"context"
	"fmt"
	"net/http"
	"reflect"

	"github.com/bowlinedev/bowline/internal/codec"
	"github.com/bowlinedev/bowline/internal/validate"
)

type Item interface {
	apply(r *Router)
}

type ProcOption func(*Procedure)

func Description(s string) ProcOption {
	return func(p *Procedure) { p.Description = s }
}

func Deprecated(reason string) ProcOption {
	return func(p *Procedure) { p.Deprecated = reason }
}

func Method(verb string) ProcOption {
	return func(pr *Procedure) { pr.HTTPMethod = verb }
}

func Path(p string) ProcOption {
	return func(pr *Procedure) { pr.HTTPPath = p }
}

func Sensitive() ProcOption {
	return func(p *Procedure) { p.Sensitive = true }
}

func MaxBody(n int64) ProcOption {
	return func(p *Procedure) { p.MaxBody = n }
}

func Meta(key, value string) ProcOption {
	return func(p *Procedure) { p.Meta[key] = value }
}

func Use(mw ...Middleware) ProcOption {
	return func(p *Procedure) { p.middleware = append(p.middleware, mw...) }
}

type procItem struct {
	proc *Procedure
}

func (i procItem) apply(r *Router) {
	r.add(entry{name: i.proc.Name, proc: i.proc})
}

func Query[In, Out any](name string, fn func(context.Context, In) (Out, error), opts ...ProcOption) Item {
	return procItem{newProcedure(KindQuery, name, fn, opts)}
}

func Mutation[In, Out any](name string, fn func(context.Context, In) (Out, error), opts ...ProcOption) Item {
	return procItem{newProcedure(KindMutation, name, fn, opts)}
}

func Subscription[In, Out any](name string, fn func(context.Context, In, *Stream[Out]) error, opts ...ProcOption) Item {
	if fn == nil {
		panic(fmt.Sprintf("bowline: subscription %q: nil handler", name))
	}
	p := prepare[In, Out](KindSubscription, name)
	p.attach = func(in any, emit func(any) error) any {
		return &subscriptionInput[In, Out]{in: in.(*In), stream: &Stream[Out]{emit: emit}}
	}
	p.call = func(ctx context.Context, in any) (any, error) {
		bound := in.(*subscriptionInput[In, Out])
		return nil, fn(ctx, *bound.in, bound.stream)
	}
	for _, opt := range opts {
		opt(p)
	}
	return procItem{p}
}

type subscriptionInput[In, Out any] struct {
	in     *In
	stream *Stream[Out]
}

func newProcedure[In, Out any](kind ProcedureKind, name string, fn func(context.Context, In) (Out, error), opts []ProcOption) *Procedure {
	if fn == nil {
		panic(fmt.Sprintf("bowline: %s %q: nil handler", kind, name))
	}
	p := prepare[In, Out](kind, name)
	p.call = func(ctx context.Context, in any) (any, error) {
		return fn(ctx, *in.(*In))
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func prepare[In, Out any](kind ProcedureKind, name string) *Procedure {
	p := &Procedure{
		Name: name,
		Kind: kind,
		Meta: map[string]string{},
		In:   reflect.TypeFor[In](),
		Out:  reflect.TypeFor[Out](),
	}
	for _, t := range []reflect.Type{p.In, p.Out} {
		if !isNamedOrEmptyStruct(t) {
			panic(fmt.Sprintf("bowline: %s %q: %s must be a named type or struct{}", kind, name, t))
		}
	}
	checker, err := validate.Compile(p.In)
	if err != nil {
		panic(fmt.Sprintf("bowline: %s %q: %v", kind, name, err))
	}
	p.checker = checker
	p.plan = codec.Compile(p.Out)
	p.newFrame = func(parent context.Context, call Call) (context.Context, any) {
		call.header = http.Header{}
		f := &frame[In]{}
		f.ctx.Context = parent
		f.ctx.call = call
		return &f.ctx, &f.in
	}
	return p
}

func isNamedOrEmptyStruct(t reflect.Type) bool {
	if t.Name() != "" {
		return true
	}
	return t.Kind() == reflect.Struct && t.NumField() == 0
}

type frame[In any] struct {
	ctx callContext
	in  In
}
