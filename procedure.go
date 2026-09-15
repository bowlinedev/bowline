package bowline

import (
	"context"
	"reflect"

	"github.com/bowlinedev/bowline/internal/codec"
	"github.com/bowlinedev/bowline/internal/validate"
)

type ProcedureKind string

const (
	KindQuery        ProcedureKind = "query"
	KindMutation     ProcedureKind = "mutation"
	KindSubscription ProcedureKind = "subscription"
)

type Procedure struct {
	Name        string
	Path        string
	Kind        ProcedureKind
	Description string
	Deprecated  string
	Sensitive   bool
	Meta        map[string]string
	In          reflect.Type
	Out         reflect.Type

	middleware []Middleware
	call       func(ctx context.Context, in any) (any, error)
	newFrame   func(parent context.Context, call Call) (context.Context, any)
	plan       *codec.Plan
	checker    *validate.Checker
	variants   []variant
	attach     func(in any, emit func(any) error) any
}

func (p Procedure) Method() string {
	if (p.Kind == KindQuery || p.Kind == KindSubscription) && !p.Sensitive {
		return "GET"
	}
	return "POST"
}
