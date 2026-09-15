package jsonschema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/bowlinedev/bowline/cmd/bowline/internal/gen/ts"
	"github.com/bowlinedev/bowline/contract"
)

type node = map[string]any

type builder struct {
	doc   *contract.Document
	names map[string]string
	defs  map[string]node
	stack map[string]bool
}

func DefName(tsName string) string {
	var b strings.Builder
	for _, r := range tsName {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		case r == ' ':
		default:
			b.WriteRune('_')
		}
	}
	return strings.Trim(b.String(), "_")
}

func Procedure(doc *contract.Document, p *contract.Procedure) (json.RawMessage, json.RawMessage, error) {
	names := ts.AssignNames(doc)
	input, err := Type(doc, p.Input, names)
	if err != nil {
		return nil, nil, fmt.Errorf("%s input: %w", p.Path, err)
	}
	output, err := Type(doc, p.Output, names)
	if err != nil {
		return nil, nil, fmt.Errorf("%s output: %w", p.Path, err)
	}
	return input, output, nil
}

func Type(doc *contract.Document, t *contract.Type, names map[string]string) (json.RawMessage, error) {
	b := &builder{doc: doc, names: names, defs: map[string]node{}, stack: map[string]bool{}}
	root, err := b.node(t, nil)
	if err != nil {
		return nil, err
	}
	root["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	if len(b.defs) > 0 {
		root["$defs"] = b.defs
	}
	return marshal(root)
}

func marshal(v any) (json.RawMessage, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func (b *builder) node(t *contract.Type, env map[string]*contract.Type) (node, error) {
	var out node
	var err error
	switch t.Kind {
	case contract.Primitive:
		out = primitive(t)
	case contract.Ref:
		out, err = b.ref(t, env)
	case contract.Array:
		elem, e := b.node(t.Elem, env)
		if e != nil {
			return nil, e
		}
		out = node{"type": "array", "items": elem}
		if t.Length > 0 {
			out["minItems"] = t.Length
			out["maxItems"] = t.Length
		}
	case contract.Map:
		value, e := b.node(t.Value, env)
		if e != nil {
			return nil, e
		}
		out = node{"type": "object", "additionalProperties": value}
	case contract.Struct:
		out, err = b.object(t.Fields, "", env)
	case contract.Param:
		bound, ok := env[t.Name]
		if !ok {
			return nil, fmt.Errorf("unbound type parameter %s", t.Name)
		}
		return b.node(bound, nil)
	default:
		return nil, fmt.Errorf("unsupported type kind %q", t.Kind)
	}
	if err != nil {
		return nil, err
	}
	if t.Nullable {
		out = nullable(out)
	}
	return out, nil
}

func primitive(t *contract.Type) node {
	switch t.Name {
	case "string":
		return node{"type": "string"}
	case "bool":
		return node{"type": "boolean"}
	case "int8":
		return node{"type": "integer", "minimum": -128, "maximum": 127}
	case "int16":
		return node{"type": "integer", "minimum": -32768, "maximum": 32767}
	case "int32":
		return node{"type": "integer", "minimum": -2147483648, "maximum": 2147483647}
	case "uint8":
		return node{"type": "integer", "minimum": 0, "maximum": 255}
	case "uint16":
		return node{"type": "integer", "minimum": 0, "maximum": 65535}
	case "uint32":
		return node{"type": "integer", "minimum": 0, "maximum": 4294967295}
	case "int64":
		if t.Encoding == "string" {
			return node{"type": "string", "pattern": "^-?[0-9]+$"}
		}
		return node{"type": "integer", "minimum": -9007199254740991, "maximum": 9007199254740991}
	case "uint64":
		if t.Encoding == "string" {
			return node{"type": "string", "pattern": "^[0-9]+$"}
		}
		return node{"type": "integer", "minimum": 0, "maximum": 9007199254740991}
	case "float32", "float64":
		return node{"type": "number"}
	case "timestamp":
		return node{"type": "string", "format": "date-time"}
	case "duration":
		return node{"type": "integer", "description": "duration in nanoseconds"}
	case "bytes":
		return node{"type": "string", "contentEncoding": "base64"}
	case "raw":
		return node{}
	}
	return node{}
}

func nullable(n node) node {
	if typ, ok := n["type"].(string); ok && len(n) <= 6 && n["$ref"] == nil && n["anyOf"] == nil && n["properties"] == nil && n["items"] == nil && n["additionalProperties"] == nil && n["enum"] == nil {
		copied := node{}
		for k, v := range n {
			copied[k] = v
		}
		copied["type"] = []string{typ, "null"}
		return copied
	}
	return node{"anyOf": []node{n, {"type": "null"}}}
}

func (b *builder) ref(t *contract.Type, env map[string]*contract.Type) (node, error) {
	decl, ok := b.doc.Types[t.ID]
	if !ok {
		return nil, fmt.Errorf("unknown type %s", t.ID)
	}
	switch decl.Kind {
	case contract.Primitive:
		return primitive(&contract.Type{Kind: contract.Primitive, Name: decl.Primitive}), nil
	case contract.Enum:
		name := DefName(b.names[t.ID])
		if _, done := b.defs[name]; !done {
			values := make([]any, len(decl.Values))
			for i, v := range decl.Values {
				values[i] = v.Value
			}
			typ := "string"
			if decl.Base != "string" {
				typ = "integer"
			}
			def := node{"type": typ, "enum": values}
			if decl.Doc != "" {
				def["description"] = decl.Doc
			}
			b.defs[name] = def
		}
		return node{"$ref": "#/$defs/" + name}, nil
	case contract.Struct:
		name := DefName(b.names[t.ID])
		if _, done := b.defs[name]; !done && !b.stack[name] {
			b.stack[name] = true
			obj, err := b.object(decl.Fields, decl.Doc, nil)
			delete(b.stack, name)
			if err != nil {
				return nil, err
			}
			b.defs[name] = obj
		}
		return node{"$ref": "#/$defs/" + name}, nil
	case contract.Generic:
		args := make([]*contract.Type, len(t.Args))
		argNames := make([]string, len(t.Args))
		for i, a := range t.Args {
			args[i] = substitute(a, env)
			argNames[i] = b.typeName(args[i])
		}
		name := DefName(b.names[t.ID] + "<" + strings.Join(argNames, ", ") + ">")
		if _, done := b.defs[name]; !done && !b.stack[name] {
			b.stack[name] = true
			inner := map[string]*contract.Type{}
			for i, p := range decl.Params {
				if i < len(args) {
					inner[p] = args[i]
				}
			}
			obj, err := b.object(decl.Body.Fields, decl.Doc, inner)
			delete(b.stack, name)
			if err != nil {
				return nil, err
			}
			b.defs[name] = obj
		}
		return node{"$ref": "#/$defs/" + name}, nil
	}
	return nil, fmt.Errorf("unsupported declaration kind %q for %s", decl.Kind, t.ID)
}

func (b *builder) typeName(t *contract.Type) string {
	switch t.Kind {
	case contract.Ref:
		name := b.names[t.ID]
		if len(t.Args) > 0 {
			parts := make([]string, len(t.Args))
			for i, a := range t.Args {
				parts[i] = b.typeName(a)
			}
			name += "<" + strings.Join(parts, ", ") + ">"
		}
		return name
	case contract.Primitive:
		return t.Name
	case contract.Array:
		return b.typeName(t.Elem) + "[]"
	case contract.Map:
		return "Record<string, " + b.typeName(t.Value) + ">"
	case contract.Param:
		return t.Name
	}
	return string(t.Kind)
}

func substitute(t *contract.Type, env map[string]*contract.Type) *contract.Type {
	if t == nil || len(env) == 0 {
		return t
	}
	if t.Kind == contract.Param {
		if bound, ok := env[t.Name]; ok {
			return bound
		}
		return t
	}
	copied := *t
	if len(t.Args) > 0 {
		copied.Args = make([]*contract.Type, len(t.Args))
		for i, a := range t.Args {
			copied.Args[i] = substitute(a, env)
		}
	}
	copied.Elem = substitute(t.Elem, env)
	copied.Value = substitute(t.Value, env)
	copied.Key = substitute(t.Key, env)
	return &copied
}

func (b *builder) object(fields []*contract.Field, doc string, env map[string]*contract.Type) (node, error) {
	props := node{}
	var required []string
	for _, f := range fields {
		prop, err := b.node(f.Type, env)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", f.Name, err)
		}
		if f.Nullable {
			prop = nullable(prop)
		}
		if f.Doc != "" {
			prop = withDescription(prop, f.Doc)
		}
		prop = applyRules(prop, f)
		if f.Example != nil {
			prop = withExample(prop, f.Example)
		}
		props[f.Name] = prop
		if !f.Optional {
			required = append(required, f.Name)
		}
	}
	out := node{"type": "object", "properties": props, "additionalProperties": false}
	if len(required) > 0 {
		sort.Strings(required)
		out["required"] = required
	}
	if doc != "" {
		out["description"] = doc
	}
	return out, nil
}

func withDescription(n node, doc string) node {
	copied := node{}
	for k, v := range n {
		copied[k] = v
	}
	copied["description"] = doc
	return copied
}

func withExample(n node, example any) node {
	copied := node{}
	for k, v := range n {
		copied[k] = v
	}
	copied["examples"] = []any{example}
	return copied
}
