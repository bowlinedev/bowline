package support

import (
	"fmt"
	"sort"

	"github.com/bowlinedev/bowline/contract"
)

var knownKinds = map[contract.Kind]bool{
	contract.Primitive: true,
	contract.Ref:       true,
	contract.Array:     true,
	contract.Map:       true,
	contract.Struct:    true,
	contract.Enum:      true,
	contract.Generic:   true,
	contract.Param:     true,
}

var knownPrimitives = map[string]bool{
	"string": true, "bool": true,
	"int8": true, "int16": true, "int32": true, "int64": true,
	"uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"float32": true, "float64": true,
	"timestamp": true, "duration": true, "bytes": true, "raw": true,
}

var knownDeclKinds = map[contract.Kind]bool{
	contract.Struct:    true,
	contract.Enum:      true,
	contract.Generic:   true,
	contract.Primitive: true,
}

func Reject(doc *contract.Document, target string) error {
	ids := make([]string, 0, len(doc.Types))
	for id := range doc.Types {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		decl := doc.Types[id]
		if !knownDeclKinds[decl.Kind] {
			return fmt.Errorf("%s: type %s has declaration kind %q, which this generator cannot represent; regenerate with a newer bowline", target, id, decl.Kind)
		}
		if decl.Kind == contract.Primitive && !knownPrimitives[decl.Primitive] {
			return fmt.Errorf("%s: type %s is the primitive %q, which this generator cannot represent; regenerate with a newer bowline", target, id, decl.Primitive)
		}
		if err := walk(decl.Body, id, target); err != nil {
			return err
		}
		for _, f := range decl.Fields {
			if err := walk(f.Type, id+"."+f.Name, target); err != nil {
				return err
			}
		}
	}
	errorIDs := make([]string, 0, len(doc.Errors))
	for id := range doc.Errors {
		errorIDs = append(errorIDs, id)
	}
	sort.Strings(errorIDs)
	for _, id := range errorIDs {
		for _, f := range doc.Errors[id].Fields {
			if err := walk(f.Type, id+"."+f.Name, target); err != nil {
				return err
			}
		}
	}
	for _, p := range doc.Procedures {
		if err := walk(p.Input, p.Path+" input", target); err != nil {
			return err
		}
		if err := walk(p.Output, p.Path+" output", target); err != nil {
			return err
		}
	}
	return nil
}

func walk(t *contract.Type, where, target string) error {
	if t == nil {
		return nil
	}
	if !knownKinds[t.Kind] {
		return fmt.Errorf("%s: %s uses the node kind %q, which this generator cannot represent; regenerate with a newer bowline", target, where, t.Kind)
	}
	if t.Kind == contract.Primitive && !knownPrimitives[t.Name] {
		return fmt.Errorf("%s: %s uses the primitive %q, which this generator cannot represent; regenerate with a newer bowline", target, where, t.Name)
	}
	for _, child := range []*contract.Type{t.Elem, t.Key, t.Value} {
		if err := walk(child, where, target); err != nil {
			return err
		}
	}
	for _, arg := range t.Args {
		if err := walk(arg, where, target); err != nil {
			return err
		}
	}
	for _, f := range t.Fields {
		if err := walk(f.Type, where+"."+f.Name, target); err != nil {
			return err
		}
	}
	return nil
}
