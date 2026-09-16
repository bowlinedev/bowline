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
		if decl.Kind == contract.Generic && decl.Body == nil {
			return fmt.Errorf("%s: type %s is generic with no body; the document is malformed", target, id)
		}
		if decl.Body != nil {
			if err := walkIn(doc, decl.Body, id, target); err != nil {
				return err
			}
		}
		for i, f := range decl.Fields {
			if f == nil {
				return fmt.Errorf("%s: type %s field %d is null; the document is malformed", target, id, i)
			}
			if err := walkIn(doc, f.Type, id+"."+f.Name, target); err != nil {
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
		for i, f := range doc.Errors[id].Fields {
			if f == nil {
				return fmt.Errorf("%s: error %s field %d is null; the document is malformed", target, id, i)
			}
			if err := walkIn(doc, f.Type, id+"."+f.Name, target); err != nil {
				return err
			}
		}
	}
	for i, p := range doc.Procedures {
		if p == nil {
			return fmt.Errorf("%s: procedure %d is null; the document is malformed", target, i)
		}
		if p.Path == "" {
			return fmt.Errorf("%s: procedure %d has no path; the document is malformed", target, i)
		}
		if p.Input == nil {
			return fmt.Errorf("%s: procedure %s has no input; the document is malformed", target, p.Path)
		}
		if p.Output == nil {
			return fmt.Errorf("%s: procedure %s has no output; the document is malformed", target, p.Path)
		}
		if err := walkIn(doc, p.Input, p.Path+" input", target); err != nil {
			return err
		}
		if err := walkIn(doc, p.Output, p.Path+" output", target); err != nil {
			return err
		}
		for _, id := range p.Errors {
			if _, ok := doc.Errors[id]; !ok {
				return fmt.Errorf("%s: procedure %s declares the undeclared error %s; the document is malformed", target, p.Path, id)
			}
		}
	}
	return nil
}

func walkIn(doc *contract.Document, t *contract.Type, where, target string) error {
	if t == nil {
		return fmt.Errorf("%s: %s is null; the document is malformed", target, where)
	}
	if !knownKinds[t.Kind] {
		return fmt.Errorf("%s: %s uses the node kind %q, which this generator cannot represent; regenerate with a newer bowline", target, where, t.Kind)
	}
	if t.Kind == contract.Primitive && !knownPrimitives[t.Name] {
		return fmt.Errorf("%s: %s uses the primitive %q, which this generator cannot represent; regenerate with a newer bowline", target, where, t.Name)
	}
	if t.Kind == contract.Ref {
		if t.ID == "" {
			return fmt.Errorf("%s: %s is a reference with no id; the document is malformed", target, where)
		}
		if _, ok := doc.Types[t.ID]; !ok {
			return fmt.Errorf("%s: %s refers to the undeclared type %s; the document is malformed", target, where, t.ID)
		}
	}
	if t.Kind == contract.Array && t.Elem == nil {
		return fmt.Errorf("%s: %s is an array with no element type; the document is malformed", target, where)
	}
	if t.Kind == contract.Map && (t.Key == nil || t.Value == nil) {
		return fmt.Errorf("%s: %s is a map with no key or value type; the document is malformed", target, where)
	}
	for _, child := range []*contract.Type{t.Elem, t.Key, t.Value} {
		if child == nil {
			continue
		}
		if err := walkIn(doc, child, where, target); err != nil {
			return err
		}
	}
	for i, arg := range t.Args {
		if err := walkIn(doc, arg, fmt.Sprintf("%s argument %d", where, i), target); err != nil {
			return err
		}
	}
	for i, f := range t.Fields {
		if f == nil {
			return fmt.Errorf("%s: %s field %d is null; the document is malformed", target, where, i)
		}
		if err := walkIn(doc, f.Type, where+"."+f.Name, target); err != nil {
			return err
		}
	}
	return nil
}
