package ts

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/bowlinedev/bowline/contract"
)

func (Generator) GenerateZod(doc *contract.Document) ([]byte, error) {
	return Generator{}.GenerateZodFor(doc, "bowline.ts")
}

func (Generator) GenerateZodFor(doc *contract.Document, _ string) ([]byte, error) {
	g := newGenerator(doc)
	z := &zodGenerator{generator: g, lazy: map[string]bool{}}
	z.plan()
	var b strings.Builder
	b.WriteString("import { z } from \"zod\";\n\n")
	for _, id := range z.order {
		b.WriteString(z.declaration(id))
	}
	b.WriteString(z.schemasObject())
	b.WriteString(z.inputsObject())
	b.WriteString(z.errorsObject())
	return []byte(b.String()), nil
}

type zodGenerator struct {
	*generator
	order []string
	lazy  map[string]bool
	visit map[string]int
}

func (z *zodGenerator) plan() {
	z.visit = map[string]int{}
	for _, id := range z.generator.order {
		z.dfs(id)
	}
}

func (z *zodGenerator) dfs(id string) {
	switch z.visit[id] {
	case 1:
		return
	case 2:
		return
	}
	z.visit[id] = 1
	for _, dep := range z.deps(id) {
		if z.visit[dep] == 1 {
			z.lazy[id] = true
			continue
		}
		z.dfs(dep)
	}
	z.visit[id] = 2
	z.order = append(z.order, id)
}

func (z *zodGenerator) deps(id string) []string {
	decl := z.doc.Types[id]
	seen := map[string]bool{}
	var out []string
	var walk func(t *contract.Type)
	walk = func(t *contract.Type) {
		if t == nil {
			return
		}
		switch t.Kind {
		case contract.Ref:
			if !seen[t.ID] {
				seen[t.ID] = true
				out = append(out, t.ID)
			}
			for _, a := range t.Args {
				walk(a)
			}
		case contract.Array:
			walk(t.Elem)
		case contract.Map:
			walk(t.Value)
		case contract.Struct:
			for _, f := range t.Fields {
				walk(f.Type)
			}
		}
	}
	for _, f := range decl.Fields {
		walk(f.Type)
	}
	if decl.Body != nil {
		walk(decl.Body)
	}
	sort.Strings(out)
	return out
}

func (z *zodGenerator) declaration(id string) string {
	decl := z.doc.Types[id]
	name := z.names[id]
	switch decl.Kind {
	case contract.Generic:
		params := make([]string, len(decl.Params))
		args := make([]string, len(decl.Params))
		env := map[string]string{}
		for i, p := range decl.Params {
			params[i] = p + " extends z.ZodTypeAny"
			args[i] = strings.ToLower(p) + ": " + p
			env[p] = strings.ToLower(p)
		}
		body := z.objectWith(decl.Body.Fields, env, "", z.lazy[id])
		return "const " + name + " = <" + strings.Join(params, ", ") + ">(" + strings.Join(args, ", ") + ") =>\n" + indent(body, "  ") + ";\n\n"
	case contract.Enum:
		return "const " + name + " = " + z.enumSchema(decl) + ";\n\n"
	case contract.Primitive:
		return "const " + name + " = " + z.primitiveSchema(&contract.Type{Kind: contract.Primitive, Name: decl.Primitive}) + ";\n\n"
	}
	return "const " + name + " = " + z.objectWith(decl.Fields, nil, "", z.lazy[id]) + ";\n\n"
}

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = prefix + l
		}
	}
	return strings.Join(lines, "\n")
}

func (z *zodGenerator) enumSchema(decl *contract.TypeDecl) string {
	if decl.Base == "string" {
		values := make([]string, len(decl.Values))
		for i, v := range decl.Values {
			values[i] = literal(v.Value)
		}
		return "z.enum([" + strings.Join(values, ", ") + "])"
	}
	return literalUnion(decl.Values)
}

func literalUnion(values []contract.EnumValue) string {
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = "z.literal(" + literal(v.Value) + ")"
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return "z.union([" + strings.Join(parts, ", ") + "])"
}

func (z *zodGenerator) object(fields []*contract.Field, env map[string]string, base string) string {
	return z.objectWith(fields, env, base, false)
}

func (z *zodGenerator) objectWith(fields []*contract.Field, env map[string]string, base string, recursive bool) string {
	if len(fields) == 0 {
		return "z.object({})"
	}
	var b strings.Builder
	b.WriteString("z.object({\n")
	for _, f := range fields {
		if recursive && hasRef(f.Type) {
			b.WriteString(base)
			b.WriteString("  get ")
			b.WriteString(propertyKey(f.Name))
			b.WriteString("() {\n")
			b.WriteString(base)
			b.WriteString("    return ")
			b.WriteString(z.field(f, env))
			b.WriteString(";\n")
			b.WriteString(base)
			b.WriteString("  },\n")
			continue
		}
		b.WriteString(base)
		b.WriteString("  ")
		b.WriteString(propertyKey(f.Name))
		b.WriteString(": ")
		b.WriteString(z.field(f, env))
		b.WriteString(",\n")
	}
	b.WriteString(base)
	b.WriteString("})")
	return b.String()
}

func hasRef(t *contract.Type) bool {
	if t == nil {
		return false
	}
	switch t.Kind {
	case contract.Ref:
		return true
	case contract.Array:
		return hasRef(t.Elem)
	case contract.Map:
		return hasRef(t.Value)
	case contract.Struct:
		for _, f := range t.Fields {
			if hasRef(f.Type) {
				return true
			}
		}
	}
	return false
}

func (z *zodGenerator) field(f *contract.Field, env map[string]string) string {
	s := z.schemaWithRules(f.Type, f.Rules, env)
	if f.Nullable {
		s += ".nullable()"
	}
	if f.Optional {
		s += ".optional()"
	}
	return s
}

func (z *zodGenerator) schemaWithRules(t *contract.Type, rules []contract.Rule, env map[string]string) string {
	if oneof := findRule(rules, "oneof"); oneof != nil {
		return z.oneof(t, oneof.Param)
	}
	var b strings.Builder
	b.WriteString(z.schema(t, env))
	class := zodClass(t)
	for _, r := range rules {
		switch r.Rule {
		case "required":
			if (class == "string" || class == "array") && findRule(rules, "min") == nil {
				b.WriteString(".min(1)")
			}
		case "min":
			b.WriteString(".min(")
			b.WriteString(r.Param)
			b.WriteString(")")
		case "max":
			b.WriteString(".max(")
			b.WriteString(r.Param)
			b.WriteString(")")
		case "len":
			if class == "number" {
				b.WriteString(".min(")
				b.WriteString(r.Param)
				b.WriteString(").max(")
				b.WriteString(r.Param)
				b.WriteString(")")
			} else {
				b.WriteString(".length(")
				b.WriteString(r.Param)
				b.WriteString(")")
			}
		case "email":
			b.WriteString(".email()")
		case "url":
			b.WriteString(".url()")
		case "uuid":
			b.WriteString(".uuid()")
		}
	}
	return b.String()
}

func findRule(rules []contract.Rule, name string) *contract.Rule {
	for i := range rules {
		if rules[i].Rule == name {
			return &rules[i]
		}
	}
	return nil
}

func (z *zodGenerator) oneof(t *contract.Type, param string) string {
	options := strings.Fields(param)
	if zodClass(t) == "number" {
		values := make([]contract.EnumValue, len(options))
		for i, o := range options {
			values[i] = contract.EnumValue{Value: json.Number(o)}
		}
		s := literalUnion(values)
		if t.Nullable {
			s += ".nullable()"
		}
		return s
	}
	quoted := make([]string, len(options))
	for i, o := range options {
		quoted[i] = strconv.Quote(o)
	}
	s := "z.enum([" + strings.Join(quoted, ", ") + "])"
	if t.Nullable {
		s += ".nullable()"
	}
	return s
}

func zodClass(t *contract.Type) string {
	switch t.Kind {
	case contract.Primitive:
		switch t.Name {
		case "string", "bytes":
			return "string"
		case "bool", "timestamp", "raw":
			return "other"
		}
		if t.Encoding == "string" {
			return "bigint"
		}
		return "number"
	case contract.Array:
		return "array"
	}
	return "other"
}

func (z *zodGenerator) schema(t *contract.Type, env map[string]string) string {
	var s string
	switch t.Kind {
	case contract.Primitive:
		s = z.primitiveSchema(t)
	case contract.Ref:
		s = z.names[t.ID]
		if len(t.Args) > 0 {
			args := make([]string, len(t.Args))
			for i, a := range t.Args {
				args[i] = z.schema(a, env)
			}
			s += "(" + strings.Join(args, ", ") + ")"
		}
	case contract.Array:
		s = "z.array(" + z.schema(t.Elem, env) + ")"
	case contract.Map:
		s = "z.record(z.string(), " + z.schema(t.Value, env) + ")"
	case contract.Struct:
		s = z.object(t.Fields, env, "")
	case contract.Param:
		s = env[t.Name]
	default:
		s = "z.unknown()"
	}
	if t.Nullable {
		s += ".nullable()"
	}
	return s
}

func (z *zodGenerator) primitiveSchema(t *contract.Type) string {
	switch t.Name {
	case "string", "bytes":
		return "z.string()"
	case "bool":
		return "z.boolean()"
	case "int64", "uint64":
		if t.Encoding == "string" {
			return "z.bigint()"
		}
		return "z.number().int()"
	case "int8", "int16", "int32", "uint8", "uint16", "uint32", "duration":
		return "z.number().int()"
	case "float32", "float64":
		return "z.number()"
	case "timestamp":
		return "z.date()"
	}
	return "z.unknown()"
}

func (z *zodGenerator) schemasObject() string {
	var b strings.Builder
	b.WriteString("export const schemas = {\n")
	for _, id := range z.generator.order {
		b.WriteString("  ")
		b.WriteString(z.names[id])
		b.WriteString(",\n")
	}
	b.WriteString("} as const;\n\n")
	return b.String()
}

func (z *zodGenerator) inputsObject() string {
	var b strings.Builder
	b.WriteString("export const inputs = {\n")
	for _, p := range z.doc.Procedures {
		b.WriteString("  ")
		b.WriteString(strconv.Quote(p.Path))
		b.WriteString(": ")
		b.WriteString(z.schema(p.Input, nil))
		b.WriteString(",\n")
	}
	b.WriteString("} as const;\n\n")
	return b.String()
}

func (z *zodGenerator) errorsObject() string {
	var b strings.Builder
	b.WriteString("export const errors = {\n")
	for _, id := range z.errorOrder {
		b.WriteString("  ")
		b.WriteString(z.names[id])
		b.WriteString(": ")
		b.WriteString(z.object(z.doc.Errors[id].Fields, nil, "  "))
		b.WriteString(",\n")
	}
	b.WriteString("} as const;\n")
	return b.String()
}
