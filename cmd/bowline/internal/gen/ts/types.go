package ts

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/bowlinedev/bowline/contract"
)

type generator struct {
	doc        *contract.Document
	names      map[string]string
	needs      map[string]bool
	hydrators  map[string][]hydrateEntry
	order      []string
	errorOrder []string
}

func newGenerator(doc *contract.Document) *generator {
	g := &generator{doc: doc, names: assignNames(doc), needs: map[string]bool{}, hydrators: map[string][]hydrateEntry{}}
	for id := range doc.Types {
		g.order = append(g.order, id)
	}
	slices.SortFunc(g.order, func(a, b string) int {
		return cmp.Or(cmp.Compare(g.names[a], g.names[b]), cmp.Compare(a, b))
	})
	for id := range doc.Errors {
		g.errorOrder = append(g.errorOrder, id)
	}
	slices.SortFunc(g.errorOrder, func(a, b string) int {
		return cmp.Or(cmp.Compare(g.names[a], g.names[b]), cmp.Compare(a, b))
	})
	return g
}

func (g *generator) tsType(t *contract.Type, inArray bool) string {
	var s string
	switch t.Kind {
	case contract.Primitive:
		s = g.primitive(t)
	case contract.Ref:
		s = g.names[t.ID]
		if len(t.Args) > 0 {
			args := make([]string, len(t.Args))
			for i, a := range t.Args {
				args[i] = g.tsType(a, false)
			}
			s += "<" + strings.Join(args, ", ") + ">"
		}
	case contract.Array:
		s = g.tsType(t.Elem, true) + "[]"
	case contract.Map:
		s = "Record<string, " + g.tsType(t.Value, false) + ">"
	case contract.Struct:
		s = g.inlineStruct(t.Fields)
	case contract.Param:
		s = t.Name
	default:
		s = "unknown"
	}
	if t.Nullable {
		s += " | null"
		if inArray {
			s = "(" + s + ")"
		}
	}
	return s
}

func (g *generator) primitive(t *contract.Type) string {
	switch t.Name {
	case "string":
		return "string"
	case "bool":
		return "boolean"
	case "int64", "uint64":
		if t.Encoding == "string" {
			return "bigint"
		}
		return "number"
	case "int8", "int16", "int32", "uint8", "uint16", "uint32", "float32", "float64":
		return "number"
	case "timestamp":
		return "Date"
	case "duration":
		g.needs["DurationNs"] = true
		return "DurationNs"
	case "bytes":
		g.needs["Base64"] = true
		return "Base64"
	case "raw":
		return "unknown"
	}
	return "unknown"
}

func (g *generator) inlineStruct(fields []*contract.Field) string {
	if len(fields) == 0 {
		return "Record<string, never>"
	}
	var b strings.Builder
	b.WriteString("{ ")
	for i, f := range fields {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(g.fieldSignature(f))
	}
	b.WriteString(" }")
	return b.String()
}

func (g *generator) fieldSignature(f *contract.Field) string {
	opt := ""
	if f.Optional {
		opt = "?"
	}
	typ := g.tsType(f.Type, false)
	if f.Nullable {
		typ += " | null"
	}
	return propertyKey(f.Name) + opt + ": " + typ + ";"
}

func jsdoc(indent string, lines ...string) string {
	var content []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			content = append(content, strings.Split(l, "\n")...)
		}
	}
	if len(content) == 0 {
		return ""
	}
	if len(content) == 1 {
		return indent + "/** " + content[0] + " */\n"
	}
	var b strings.Builder
	b.WriteString(indent)
	b.WriteString("/**\n")
	for _, l := range content {
		b.WriteString(indent)
		b.WriteString(" * ")
		b.WriteString(l)
		b.WriteString("\n")
	}
	b.WriteString(indent)
	b.WriteString(" */\n")
	return b.String()
}

func (g *generator) declarations() string {
	var b strings.Builder
	for _, id := range g.errorOrder {
		decl := g.doc.Errors[id]
		b.WriteString(jsdoc("", decl.Doc))
		b.WriteString("export interface ")
		b.WriteString(g.names[id])
		b.WriteString(" {\n")
		for _, f := range decl.Fields {
			b.WriteString(jsdoc("  ", f.Doc))
			b.WriteString("  ")
			b.WriteString(g.fieldSignature(f))
			b.WriteString("\n")
		}
		b.WriteString("}\n\n")
	}
	for _, id := range g.order {
		decl := g.doc.Types[id]
		name := g.names[id]
		b.WriteString(jsdoc("", decl.Doc))
		switch decl.Kind {
		case contract.Struct:
			b.WriteString("export interface ")
			b.WriteString(name)
			b.WriteString(" {\n")
			for _, f := range decl.Fields {
				b.WriteString(jsdoc("  ", f.Doc))
				b.WriteString("  ")
				b.WriteString(g.fieldSignature(f))
				b.WriteString("\n")
			}
			b.WriteString("}\n\n")
		case contract.Generic:
			b.WriteString("export interface ")
			b.WriteString(name)
			b.WriteString("<")
			b.WriteString(strings.Join(decl.Params, ", "))
			b.WriteString("> {\n")
			for _, f := range decl.Body.Fields {
				b.WriteString(jsdoc("  ", f.Doc))
				b.WriteString("  ")
				b.WriteString(g.fieldSignature(f))
				b.WriteString("\n")
			}
			b.WriteString("}\n\n")
		case contract.Enum:
			values := make([]string, len(decl.Values))
			for i, v := range decl.Values {
				values[i] = literal(v.Value)
			}
			b.WriteString("export type ")
			b.WriteString(name)
			b.WriteString(" = ")
			b.WriteString(strings.Join(values, " | "))
			b.WriteString(";\n\n")
		case contract.Primitive:
			b.WriteString("export type ")
			b.WriteString(name)
			b.WriteString(" = ")
			b.WriteString(g.primitive(&contract.Type{Kind: contract.Primitive, Name: decl.Primitive}))
			b.WriteString(";\n\n")
		}
	}
	return b.String()
}

func literal(v any) string {
	switch x := v.(type) {
	case string:
		return strconv.Quote(x)
	case json.Number:
		return x.String()
	case int64:
		return strconv.FormatInt(x, 10)
	case uint64:
		return strconv.FormatUint(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	}
	return fmt.Sprint(v)
}
