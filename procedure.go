package bowline

import (
	"context"
	"reflect"
)

type ProcedureKind string

const (
	KindQuery    ProcedureKind = "query"
	KindMutation ProcedureKind = "mutation"
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
	newIn      func() any
	deref      func(ptr any) any
}

func (p Procedure) Method() string {
	if p.Kind == KindQuery && !p.Sensitive {
		return "GET"
	}
	return "POST"
}
