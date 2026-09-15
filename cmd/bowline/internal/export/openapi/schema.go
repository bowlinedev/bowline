package openapi

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/bowlinedev/bowline/contract"
)

type schema = map[string]any

type schemas struct {
	doc      *contract.Document
	names    map[string]string
	defined  map[string]schema
	building map[string]bool
}

func newSchemas(doc *contract.Document) *schemas {
	return &schemas{doc: doc, names: assignNames(doc), defined: map[string]schema{}, building: map[string]bool{}}
}

func assignNames(doc *contract.Document) map[string]string {
	byName := map[string][]string{}
	for id, decl := range doc.Types {
		byName[decl.Name] = append(byName[decl.Name], id)
	}
	for id, decl := range doc.Errors {
		byName[decl.Name] = append(byName[decl.Name], id)
	}
	names := map[string]string{}
	for name, ids := range byName {
		if len(ids) == 1 {
			names[ids[0]] = identifier(name)
			continue
		}
		sort.Strings(ids)
		for _, id := range ids {
			names[id] = identifier(packageName(id) + "_" + name)
		}
	}
	return names
}

func packageName(id string) string {
	pkg := id
	if i := strings.LastIndex(id, "."); i >= 0 {
		pkg = id[:i]
	}
	if i := strings.LastIndex(pkg, "/"); i >= 0 {
		pkg = pkg[i+1:]
	}
	if pkg == "" {
		return "Pkg"
	}
	return strings.ToUpper(pkg[:1]) + pkg[1:]
}

func identifier(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

func ref(name string) schema {
	return schema{"$ref": "#/components/schemas/" + name}
}

func (s *schemas) node(t *contract.Type, env map[string]*contract.Type) (schema, error) {
	var out schema
	var err error
	switch t.Kind {
	case contract.Primitive:
		out = primitive(t)
	case contract.Ref:
		name, buildErr := s.declare(t, env)
		if buildErr != nil {
			return nil, buildErr
		}
		out = ref(name)
	case contract.Array:
		elem, elemErr := s.node(t.Elem, env)
		if elemErr != nil {
			return nil, elemErr
		}
		out = schema{"type": "array", "items": elem}
		if t.Length > 0 {
			out["minItems"] = t.Length
			out["maxItems"] = t.Length
		}
	case contract.Map:
		value, valueErr := s.node(t.Value, env)
		if valueErr != nil {
			return nil, valueErr
		}
		out = schema{"type": "object", "additionalProperties": value}
	case contract.Struct:
		out, err = s.object(t.Fields, env)
		if err != nil {
			return nil, err
		}
	case contract.Param:
		bound, ok := env[t.Name]
		if !ok {
			return nil, fmt.Errorf("openapi: unbound type parameter %s", t.Name)
		}
		return s.node(bound, nil)
	default:
		return nil, fmt.Errorf("openapi: unsupported type kind %q", t.Kind)
	}
	if t.Nullable {
		out = nullable(out)
	}
	return out, nil
}

func nullable(in schema) schema {
	if _, isRef := in["$ref"]; isRef {
		return schema{"oneOf": []schema{in, {"type": "null"}}}
	}
	out := schema{}
	for k, v := range in {
		out[k] = v
	}
	switch typ := in["type"].(type) {
	case string:
		out["type"] = []string{typ, "null"}
	case []string:
		out["type"] = append(append([]string{}, typ...), "null")
	default:
		return schema{"oneOf": []schema{in, {"type": "null"}}}
	}
	return out
}

func primitive(t *contract.Type) schema {
	switch t.Name {
	case "string":
		return schema{"type": "string"}
	case "bool":
		return schema{"type": "boolean"}
	case "int8", "int16", "int32", "uint8", "uint16", "uint32":
		return schema{"type": "integer", "format": "int32"}
	case "int64", "uint64":
		if t.Encoding == "string" {
			return schema{"type": "string", "format": "int64"}
		}
		return schema{"type": "integer", "format": "int64"}
	case "float32":
		return schema{"type": "number", "format": "float"}
	case "float64":
		return schema{"type": "number", "format": "double"}
	case "timestamp":
		return schema{"type": "string", "format": "date-time"}
	case "duration":
		return schema{"type": "integer", "format": "int64", "description": "Duration in nanoseconds."}
	case "bytes":
		return schema{"type": "string", "contentEncoding": "base64"}
	case "raw":
		return schema{}
	}
	return schema{}
}

func (s *schemas) object(fields []*contract.Field, env map[string]*contract.Type) (schema, error) {
	properties := schema{}
	var required []string
	for _, f := range fields {
		prop, err := s.node(f.Type, env)
		if err != nil {
			return nil, err
		}
		if f.Nullable {
			prop = nullable(prop)
		}
		prop = applyRules(prop, f)
		properties[f.Name] = prop
		if !f.Optional {
			required = append(required, f.Name)
		}
	}
	out := schema{"type": "object", "properties": properties, "additionalProperties": false}
	if len(required) > 0 {
		out["required"] = required
	}
	return out, nil
}

func applyRules(prop schema, f *contract.Field) schema {
	if len(f.Rules) == 0 && f.Doc == "" {
		return prop
	}
	if _, isRef := prop["$ref"]; isRef && (len(f.Rules) > 0 || f.Doc != "") {
		prop = schema{"allOf": []schema{prop}}
	}
	out := schema{}
	for k, v := range prop {
		out[k] = v
	}
	if f.Doc != "" {
		out["description"] = f.Doc
	}
	class := classOf(f.Type)
	for _, r := range f.Rules {
		switch r.Rule {
		case "min":
			out[boundKey(class, "min")] = number(r.Param)
		case "max":
			out[boundKey(class, "max")] = number(r.Param)
		case "len":
			out[boundKey(class, "min")] = number(r.Param)
			out[boundKey(class, "max")] = number(r.Param)
		case "oneof":
			var values []any
			for _, v := range strings.Fields(r.Param) {
				if class == "number" {
					values = append(values, number(v))
				} else {
					values = append(values, v)
				}
			}
			out["enum"] = values
		case "email":
			out["format"] = "email"
		case "url":
			out["format"] = "uri"
		case "uuid":
			out["format"] = "uuid"
		}
	}
	return out
}

func classOf(t *contract.Type) string {
	switch t.Kind {
	case contract.Primitive:
		switch t.Name {
		case "string", "timestamp", "bytes":
			return "string"
		case "bool", "raw":
			return "other"
		}
		return "number"
	case contract.Array, contract.Map:
		return "collection"
	}
	return "other"
}

func boundKey(class, side string) string {
	switch class {
	case "string":
		if side == "min" {
			return "minLength"
		}
		return "maxLength"
	case "collection":
		if side == "min" {
			return "minItems"
		}
		return "maxItems"
	}
	if side == "min" {
		return "minimum"
	}
	return "maximum"
}

func number(s string) json.Number {
	if _, err := strconv.ParseFloat(s, 64); err != nil {
		return json.Number("0")
	}
	return json.Number(s)
}

func (s *schemas) declare(t *contract.Type, env map[string]*contract.Type) (string, error) {
	decl, ok := s.doc.Types[t.ID]
	if !ok {
		return "", fmt.Errorf("openapi: unknown type %s", t.ID)
	}
	name := s.names[t.ID]
	if decl.Kind == contract.Generic {
		args := make([]*contract.Type, len(t.Args))
		parts := make([]string, len(t.Args))
		for i, a := range t.Args {
			args[i] = substitute(a, env)
			parts[i] = s.argName(args[i])
		}
		name = identifier(name + "_" + strings.Join(parts, "_"))
		if _, done := s.defined[name]; done || s.building[name] {
			return name, nil
		}
		s.building[name] = true
		bodyEnv := map[string]*contract.Type{}
		for i, p := range decl.Params {
			if i < len(args) {
				bodyEnv[p] = args[i]
			}
		}
		body, err := s.object(decl.Body.Fields, bodyEnv)
		if err != nil {
			return "", err
		}
		if decl.Doc != "" {
			body["description"] = decl.Doc
		}
		s.defined[name] = body
		return name, nil
	}
	if _, done := s.defined[name]; done || s.building[name] {
		return name, nil
	}
	s.building[name] = true
	var body schema
	var err error
	switch decl.Kind {
	case contract.Struct:
		body, err = s.object(decl.Fields, nil)
		if err != nil {
			return "", err
		}
	case contract.Enum:
		values := make([]any, len(decl.Values))
		for i, v := range decl.Values {
			values[i] = v.Value
		}
		body = schema{"type": "string", "enum": values}
		if decl.Base != "string" {
			body["type"] = "integer"
		}
	case contract.Primitive:
		body = primitive(&contract.Type{Kind: contract.Primitive, Name: decl.Primitive})
	default:
		return "", fmt.Errorf("openapi: unsupported declaration kind %q for %s", decl.Kind, t.ID)
	}
	if decl.Doc != "" {
		body["description"] = decl.Doc
	}
	s.defined[name] = body
	return name, nil
}

func (s *schemas) argName(t *contract.Type) string {
	switch t.Kind {
	case contract.Ref:
		name := s.names[t.ID]
		if len(t.Args) > 0 {
			parts := make([]string, len(t.Args))
			for i, a := range t.Args {
				parts[i] = s.argName(a)
			}
			name += "_" + strings.Join(parts, "_")
		}
		return name
	case contract.Primitive:
		return t.Name
	case contract.Array:
		return "Array_" + s.argName(t.Elem)
	case contract.Map:
		return "Map_" + s.argName(t.Value)
	case contract.Param:
		return t.Name
	}
	return "Struct"
}

func substitute(t *contract.Type, env map[string]*contract.Type) *contract.Type {
	if len(env) == 0 || t == nil {
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

func (s *schemas) errorDetails(id string) (string, error) {
	decl, ok := s.doc.Errors[id]
	if !ok {
		return "", fmt.Errorf("openapi: unknown error variant %s", id)
	}
	name := s.names[id]
	if _, done := s.defined[name]; done {
		return name, nil
	}
	s.building[name] = true
	body, err := s.object(decl.Fields, nil)
	if err != nil {
		return "", err
	}
	if decl.Doc != "" {
		body["description"] = decl.Doc
	}
	s.defined[name] = body
	return name, nil
}
