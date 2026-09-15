package bowline

import (
	"context"
	"fmt"
	"reflect"
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

func Sensitive() ProcOption {
	return func(p *Procedure) { p.Sensitive = true }
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

func newProcedure[In, Out any](kind ProcedureKind, name string, fn func(context.Context, In) (Out, error), opts []ProcOption) *Procedure {
	if fn == nil {
		panic(fmt.Sprintf("bowline: %s %q: nil handler", kind, name))
	}
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
	p.newIn = func() any { return new(In) }
	p.deref = func(ptr any) any { return *ptr.(*In) }
	p.call = func(ctx context.Context, in any) (any, error) {
		return fn(ctx, in.(In))
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func isNamedOrEmptyStruct(t reflect.Type) bool {
	if t.Name() != "" {
		return true
	}
	return t.Kind() == reflect.Struct && t.NumField() == 0
}
