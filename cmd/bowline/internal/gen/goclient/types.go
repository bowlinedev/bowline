package goclient

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
	order      []string
	errorOrder []string
	uses       map[string]bool
}

func newGenerator(doc *contract.Document) *generator {
	g := &generator{doc: doc, names: assignNames(doc), uses: map[string]bool{}}
	for id := range doc.Types {
		g.order = append(g.order, id)
	}
	slices.SortFunc(g.order, func(a, b string) int { return cmp.Compare(g.names[a], g.names[b]) })
	for id := range doc.Errors {
		g.errorOrder = append(g.errorOrder, id)
	}
	slices.SortFunc(g.errorOrder, func(a, b string) int { return cmp.Compare(g.names[a], g.names[b]) })
	return g
}

func (g *generator) goType(t *contract.Type) string {
	var s string
	switch t.Kind {
	case contract.Primitive:
		s = g.primitive(t.Name)
	case contract.Ref:
		s = g.names[t.ID]
		if len(t.Args) > 0 {
			args := make([]string, len(t.Args))
			for i, a := range t.Args {
				args[i] = g.goType(a)
			}
			s += "[" + strings.Join(args, ", ") + "]"
		}
	case contract.Array:
		elem := g.goType(t.Elem)
		if t.Length > 0 {
			s = "[" + strconv.Itoa(t.Length) + "]" + elem
		} else {
			s = "[]" + elem
		}
	case contract.Map:
		s = "map[" + g.goType(t.Key) + "]" + g.goType(t.Value)
	case contract.Struct:
		s = g.inlineStruct(t.Fields)
	case contract.Param:
		s = t.Name
	default:
		s = "json.RawMessage"
	}
	if t.Nullable && !isReference(t) {
		s = "*" + s
	}
	return s
}

func isReference(t *contract.Type) bool {
	switch t.Kind {
	case contract.Map:
		return true
	case contract.Array:
		return t.Length == 0
	}
	return false
}

func (g *generator) primitive(name string) string {
	switch name {
	case "string":
		return "string"
	case "bool":
		return "bool"
	case "int8", "int16", "int32", "int64", "uint8", "uint16", "uint32", "uint64", "float32", "float64":
		return name
	case "timestamp":
		g.uses["time"] = true
		return "time.Time"
	case "duration":
		g.uses["time"] = true
		return "time.Duration"
	case "bytes":
		return "[]byte"
	case "raw":
		return "json.RawMessage"
	}
	return "json.RawMessage"
}

func (g *generator) inlineStruct(fields []*contract.Field) string {
	if len(fields) == 0 {
		return "struct{}"
	}
	var b strings.Builder
	b.WriteString("struct {\n")
	g.writeFields(&b, fields, "\t")
	b.WriteString("}")
	return b.String()
}

func (g *generator) writeFields(b *strings.Builder, fields []*contract.Field, indent string) {
	used := map[string]bool{}
	for _, f := range fields {
		name := fieldName(f.Name)
		for used[name] {
			name += "_"
		}
		used[name] = true
		writeDoc(b, indent, f.Doc, "")
		b.WriteString(indent)
		b.WriteString(name)
		b.WriteString(" ")
		b.WriteString(g.fieldType(f))
		b.WriteString(" `json:\"")
		b.WriteString(f.Name)
		b.WriteString(g.tagOptions(f))
		b.WriteString("\"`\n")
	}
}

func (g *generator) fieldType(f *contract.Field) string {
	typ := g.goType(f.Type)
	if strings.HasPrefix(typ, "*") || isReference(f.Type) {
		return typ
	}
	if f.Nullable || f.Optional {
		return "*" + typ
	}
	return typ
}

func (g *generator) tagOptions(f *contract.Field) string {
	opts := ""
	if f.Optional {
		opts += ",omitempty"
	}
	if f.Type.Encoding == "string" {
		opts += ",string"
	}
	return opts
}

func writeDoc(b *strings.Builder, indent, doc, deprecated string) {
	doc = strings.TrimSpace(doc)
	if doc != "" {
		for line := range strings.SplitSeq(doc, "\n") {
			b.WriteString(indent)
			b.WriteString("// ")
			b.WriteString(strings.TrimSpace(line))
			b.WriteString("\n")
		}
	}
	if deprecated != "" {
		if doc != "" {
			b.WriteString(indent)
			b.WriteString("//\n")
		}
		b.WriteString(indent)
		b.WriteString("// Deprecated: ")
		b.WriteString(deprecated)
		b.WriteString("\n")
	}
}

func (g *generator) declarations() string {
	var b strings.Builder
	for _, id := range g.errorOrder {
		decl := g.doc.Errors[id]
		writeDoc(&b, "", docOr(decl.Doc, decl.Name+" is the details shape of the "+decl.Name+" error variant ("+decl.Code+")."), "")
		b.WriteString("type ")
		b.WriteString(g.names[id])
		b.WriteString(" struct {\n")
		g.writeFields(&b, decl.Fields, "\t")
		b.WriteString("}\n\n")
	}
	for _, id := range g.order {
		decl := g.doc.Types[id]
		name := g.names[id]
		switch decl.Kind {
		case contract.Struct:
			doc := decl.Doc
			if decl.Origin != "" {
				doc = docOr(doc, name+" is an instantiation of "+decl.Origin+".")
			}
			writeDoc(&b, "", doc, "")
			b.WriteString("type ")
			b.WriteString(name)
			b.WriteString(" struct {\n")
			g.writeFields(&b, decl.Fields, "\t")
			b.WriteString("}\n\n")
		case contract.Generic:
			writeDoc(&b, "", decl.Doc, "")
			b.WriteString("type ")
			b.WriteString(name)
			b.WriteString("[")
			b.WriteString(g.typeParams(decl))
			b.WriteString("] struct {\n")
			if decl.Body != nil {
				g.writeFields(&b, decl.Body.Fields, "\t")
			}
			b.WriteString("}\n\n")
		case contract.Enum:
			writeDoc(&b, "", decl.Doc, "")
			base := "string"
			if decl.Base != "" && decl.Base != "string" {
				base = g.primitive(decl.Base)
			}
			b.WriteString("type ")
			b.WriteString(name)
			b.WriteString(" ")
			b.WriteString(base)
			b.WriteString("\n\n")
			if len(decl.Values) > 0 {
				b.WriteString("const (\n")
				for _, v := range decl.Values {
					b.WriteString("\t")
					b.WriteString(exported(v.Name))
					b.WriteString(" ")
					b.WriteString(name)
					b.WriteString(" = ")
					b.WriteString(literal(v.Value))
					b.WriteString("\n")
				}
				b.WriteString(")\n\n")
			}
		case contract.Primitive:
			writeDoc(&b, "", decl.Doc, "")
			b.WriteString("type ")
			b.WriteString(name)
			b.WriteString(" ")
			b.WriteString(g.primitive(decl.Primitive))
			b.WriteString("\n\n")
		}
	}
	return b.String()
}

func docOr(doc, fallback string) string {
	if strings.TrimSpace(doc) != "" {
		return doc
	}
	return fallback
}

func (g *generator) typeParams(decl *contract.TypeDecl) string {
	keys := map[string]bool{}
	if decl.Body != nil {
		collectMapKeyParams(decl.Body, keys)
	}
	parts := make([]string, len(decl.Params))
	for i, p := range decl.Params {
		constraint := "any"
		if keys[p] {
			constraint = "comparable"
		}
		parts[i] = p + " " + constraint
	}
	return strings.Join(parts, ", ")
}

func collectMapKeyParams(t *contract.Type, keys map[string]bool) {
	if t == nil {
		return
	}
	if t.Kind == contract.Map && t.Key != nil && t.Key.Kind == contract.Param {
		keys[t.Key.Name] = true
	}
	for _, f := range t.Fields {
		collectMapKeyParams(f.Type, keys)
	}
	collectMapKeyParams(t.Elem, keys)
	collectMapKeyParams(t.Key, keys)
	collectMapKeyParams(t.Value, keys)
	for _, a := range t.Args {
		collectMapKeyParams(a, keys)
	}
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

func isEmptyStruct(t *contract.Type) bool {
	return t != nil && t.Kind == contract.Struct && len(t.Fields) == 0
}
