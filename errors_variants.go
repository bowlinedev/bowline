package bowline

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/bowlinedev/bowline/internal/codec"
)

type Coded interface {
	error
	Code() Code
}

type variant struct {
	typ  reflect.Type
	name string
	plan *codec.Plan
}

var errorType = reflect.TypeFor[error]()

func Errors(variants ...Coded) ProcOption {
	compiled := make([]variant, 0, len(variants))
	for _, v := range variants {
		t := reflect.TypeOf(v)
		if t == nil {
			panic("bowline: Errors: nil variant")
		}
		if t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct || t.Name() == "" {
			panic(fmt.Sprintf("bowline: Errors: %s must be a named struct type", t))
		}
		compiled = append(compiled, variant{typ: t, name: t.Name(), plan: codec.Compile(t)})
	}
	return func(p *Procedure) { p.variants = append(p.variants, compiled...) }
}

func (v variant) match(err error) (Coded, any, bool) {
	if v.typ.Implements(errorType) {
		target := reflect.New(v.typ)
		if errors.As(err, target.Interface()) {
			value := target.Elem().Interface()
			return value.(Coded), value, true
		}
	}
	ptr := reflect.PointerTo(v.typ)
	if ptr.Implements(errorType) {
		target := reflect.New(ptr)
		if errors.As(err, target.Interface()) {
			pointer := target.Elem()
			if pointer.IsNil() {
				return nil, nil, false
			}
			return pointer.Interface().(Coded), pointer.Elem().Interface(), true
		}
	}
	return nil, nil, false
}

func (v variant) envelope(coded Coded, value any) wireEnvelope {
	normalized, err := v.plan.Normalize(value)
	if err != nil {
		normalized = value
	}
	details, err := json.Marshal(normalized)
	if err != nil {
		details = []byte("{}")
	}
	return wireEnvelope{wireError{Code: coded.Code(), Message: coded.Error(), Type: v.name, Details: json.RawMessage(details)}}
}
