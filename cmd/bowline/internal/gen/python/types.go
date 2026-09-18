package python

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/bowlinedev/bowline/cmd/bowline/internal/gen/naming"
	"github.com/bowlinedev/bowline/contract"
)

var intBounds = map[string][2]string{
	"int8":   {"-128", "127"},
	"int16":  {"-32768", "32767"},
	"int32":  {"-2147483648", "2147483647"},
	"uint8":  {"0", "255"},
	"uint16": {"0", "65535"},
	"uint32": {"0", "4294967295"},
}

func (g *generator) primitive(name, encoding string) string {
	switch name {
	case "string":
		return "str"
	case "bool":
		return "bool"
	case "int64", "uint64":
		if encoding == "string" {
			g.uses["BigInt"] = true
			return "BigInt"
		}
		return "int"
	case "int8", "int16", "int32", "uint8", "uint16", "uint32":
		bounds := intBounds[name]
		g.uses["Annotated"] = true
		g.uses["Field"] = true
		return "Annotated[int, Field(ge=" + bounds[0] + ", le=" + bounds[1] + ")]"
	case "float32", "float64":
		return "float"
	case "timestamp":
		g.uses["datetime"] = true
		return "datetime"
	case "duration":
		g.uses["DurationNs"] = true
		return "DurationNs"
	case "bytes":
		g.uses["Base64Bytes"] = true
		return "Base64Bytes"
	}
	g.uses["JsonValue"] = true
	return "JsonValue"
}

func (g *generator) pyType(t *contract.Type, owner string) string {
	var s string
	switch t.Kind {
	case contract.Primitive:
		s = g.primitive(t.Name, t.Encoding)
	case contract.Ref:
		s = g.ref(t, owner)
	case contract.Array:
		elem := g.pyType(t.Elem, owner)
		s = "list[" + elem + "]"
		if t.Length > 0 {
			g.uses["Annotated"] = true
			g.uses["Field"] = true
			n := strconv.Itoa(t.Length)
			s = "Annotated[" + s + ", Field(min_length=" + n + ", max_length=" + n + ")]"
		}
	case contract.Map:
		s = "dict[str, " + g.pyType(t.Value, owner) + "]"
	case contract.Struct:
		if len(t.Fields) == 0 {
			g.uses["Empty"] = true
			s = "Empty"
		} else {
			s = g.registerInline(owner, t.Fields)
		}
	case contract.Param:
		s = t.Name
	default:
		g.uses["JsonValue"] = true
		s = "JsonValue"
	}
	if t.Nullable {
		s += " | None"
	}
	return s
}

func (g *generator) ref(t *contract.Type, owner string) string {
	decl, ok := g.doc.Types[t.ID]
	if !ok {
		return g.names[t.ID]
	}
	name := g.names[t.ID]
	if decl.Kind == contract.Primitive {
		return name
	}
	if len(t.Args) > 0 {
		args := make([]string, len(t.Args))
		for i, a := range t.Args {
			args[i] = g.pyType(a, owner)
		}
		return name + "[" + strings.Join(args, ", ") + "]"
	}
	return name
}

func (g *generator) registerInline(owner string, fields []*contract.Field) string {
	name := owner
	if _, exists := g.inline[name]; !exists {
		g.inline[name] = fields
		g.inlineOK = append(g.inlineOK, name)
	}
	return name
}

type fieldSpec struct {
	name     string
	typ      string
	kwargs   []string
	default_ bool
}

func attrName(name string) string {
	if isIdentifier(name) && !keywords[name] && !strings.HasPrefix(name, "model_") {
		return name
	}
	out := identifier(name)
	if strings.HasPrefix(out, "model_") {
		out = "field_" + out
	}
	return out
}

func (g *generator) field(f *contract.Field, owner string) fieldSpec {
	spec := fieldSpec{name: attrName(f.Name)}
	if spec.name != f.Name {
		spec.kwargs = append(spec.kwargs, "alias="+strconv.Quote(f.Name))
	}
	typ := g.pyType(f.Type, owner+"_"+naming.UpperCamel(f.Name))
	class := g.classOf(f.Type)
	if bounds, sized := g.sizedInt(f.Type); sized && len(f.Rules) > 0 {
		typ = "int"
		spec.kwargs = append(spec.kwargs, "ge="+bounds[0], "le="+bounds[1])
	}
	var literal []string
	for _, r := range f.Rules {
		switch r.Rule {
		case "required":
			switch class {
			case "string", "collection":
				spec.kwargs = append(spec.kwargs, "min_length=1")
			case "number":
				spec.kwargs = append(spec.kwargs, "gt=0")
			}
		case "min", "max", "len":
			spec.kwargs = append(spec.kwargs, boundKwargs(class, r.Rule, r.Param)...)
		case "oneof":
			literal = g.literal(f.Type, strings.Fields(r.Param))
		case "email":
			spec.kwargs = append(spec.kwargs, "pattern=r\"^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$\"")
		case "url":
			if class == "string" {
				g.uses["AnyUrl"] = true
				typ = "AnyUrl"
			}
		case "uuid":
			if class == "string" {
				g.uses["UUID"] = true
				typ = "UUID"
			}
		}
	}
	if literal != nil {
		g.uses["Literal"] = true
		typ = "Literal[" + strings.Join(literal, ", ") + "]"
	}
	if f.Nullable || f.Optional {
		typ += " | None"
	}
	if f.Optional {
		spec.default_ = true
	}
	if strings.TrimSpace(f.Doc) != "" {
		spec.kwargs = append(spec.kwargs, "description="+strconv.Quote(strings.TrimSpace(f.Doc)))
	}
	spec.kwargs = mergeKwargs(spec.kwargs)
	spec.typ = typ
	return spec
}

func (g *generator) sizedInt(t *contract.Type) ([2]string, bool) {
	if t.Nullable {
		return [2]string{}, false
	}
	name := ""
	switch t.Kind {
	case contract.Primitive:
		name = t.Name
	case contract.Ref:
		if decl, ok := g.doc.Types[t.ID]; ok && decl.Kind == contract.Primitive {
			return [2]string{}, false
		}
	}
	bounds, ok := intBounds[name]
	return bounds, ok
}

func mergeKwargs(kwargs []string) []string {
	values := map[string]string{}
	var order []string
	for _, kw := range kwargs {
		key, value, _ := strings.Cut(kw, "=")
		prev, seen := values[key]
		if !seen {
			order = append(order, key)
			values[key] = value
			continue
		}
		switch key {
		case "min_length", "ge", "gt":
			values[key] = maxNumber(prev, value)
		case "max_length", "le", "lt":
			values[key] = minNumber(prev, value)
		default:
			values[key] = value
		}
	}
	out := make([]string, 0, len(order))
	for _, key := range order {
		out = append(out, key+"="+values[key])
	}
	return out
}

func maxNumber(a, b string) string {
	if compareNumbers(a, b) >= 0 {
		return a
	}
	return b
}

func minNumber(a, b string) string {
	if compareNumbers(a, b) <= 0 {
		return a
	}
	return b
}

func compareNumbers(a, b string) int {
	x, errA := strconv.ParseFloat(a, 64)
	y, errB := strconv.ParseFloat(b, 64)
	if errA != nil || errB != nil {
		return strings.Compare(a, b)
	}
	switch {
	case x < y:
		return -1
	case x > y:
		return 1
	}
	return 0
}

func boundKwargs(class, rule, param string) []string {
	switch class {
	case "string", "collection":
		switch rule {
		case "min":
			return []string{"min_length=" + param}
		case "max":
			return []string{"max_length=" + param}
		case "len":
			return []string{"min_length=" + param, "max_length=" + param}
		}
	case "number":
		switch rule {
		case "min":
			return []string{"ge=" + param}
		case "max":
			return []string{"le=" + param}
		case "len":
			return []string{"ge=" + param, "le=" + param}
		}
	}
	return nil
}

func (g *generator) classOf(t *contract.Type) string {
	switch t.Kind {
	case contract.Primitive:
		return primitiveClass(t.Name)
	case contract.Ref:
		if decl, ok := g.doc.Types[t.ID]; ok {
			switch decl.Kind {
			case contract.Primitive:
				return primitiveClass(decl.Primitive)
			case contract.Enum:
				if decl.Base == "" || decl.Base == "string" {
					return "string"
				}
				return "number"
			}
		}
	case contract.Array, contract.Map:
		return "collection"
	}
	return ""
}

func primitiveClass(name string) string {
	switch name {
	case "string":
		return "string"
	case "int8", "int16", "int32", "int64", "uint8", "uint16", "uint32", "uint64", "float32", "float64", "duration":
		return "number"
	}
	return ""
}

func (g *generator) literal(t *contract.Type, options []string) []string {
	if t.Kind == contract.Ref {
		if decl, ok := g.doc.Types[t.ID]; ok && decl.Kind == contract.Enum {
			var out []string
			for _, option := range options {
				for _, v := range decl.Values {
					if literalValue(v.Value) == option || strconv.Quote(option) == literalValue(v.Value) {
						out = append(out, g.names[t.ID]+"."+memberName(v.Name))
					}
				}
			}
			return out
		}
	}
	out := make([]string, len(options))
	for i, option := range options {
		if g.classOf(t) == "number" {
			out[i] = option
		} else {
			out[i] = strconv.Quote(option)
		}
	}
	return out
}

func literalValue(v any) string {
	switch x := v.(type) {
	case string:
		return strconv.Quote(x)
	case json.Number:
		return x.String()
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	}
	return fmt.Sprint(v)
}

func isIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_', r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}

func (g *generator) writeModel(b *strings.Builder, name, doc string, fields []*contract.Field, generic []string) {
	base := "BaseModel"
	if len(generic) > 0 {
		g.uses["Generic"] = true
		base = "BaseModel, Generic[" + strings.Join(generic, ", ") + "]"
	}
	b.WriteString("class ")
	b.WriteString(name)
	b.WriteString("(")
	b.WriteString(base)
	b.WriteString("):\n")
	docstring(b, "    ", doc, "")
	if doc != "" {
		b.WriteString("\n")
	}
	specs := make([]fieldSpec, len(fields))
	aliased := false
	for i, f := range fields {
		specs[i] = g.field(f, name)
		for _, kw := range specs[i].kwargs {
			if strings.HasPrefix(kw, "alias=") {
				aliased = true
			}
		}
	}
	config := "extra=\"ignore\""
	if aliased {
		config += ", populate_by_name=True"
	}
	b.WriteString("    model_config = ConfigDict(")
	b.WriteString(config)
	b.WriteString(")\n")
	if len(specs) > 0 {
		b.WriteString("\n")
	}
	for _, spec := range specs {
		b.WriteString("    ")
		b.WriteString(spec.name)
		b.WriteString(": ")
		b.WriteString(spec.typ)
		switch {
		case len(spec.kwargs) > 0:
			g.uses["Field"] = true
			args := spec.kwargs
			if spec.default_ {
				args = append([]string{"default=None"}, args...)
			}
			b.WriteString(" = Field(")
			b.WriteString(strings.Join(args, ", "))
			b.WriteString(")")
		case spec.default_:
			b.WriteString(" = None")
		}
		b.WriteString("\n")
	}
	b.WriteString("\n\n")
}

type declaration struct {
	id   string
	name string
	deps []string
	emit func(b *strings.Builder)
	kind contract.Kind
}

func (g *generator) declarations() string {
	var decls []*declaration
	byName := map[string]*declaration{}
	add := func(d *declaration) {
		decls = append(decls, d)
		byName[d.name] = d
	}
	for id, decl := range g.doc.Types {
		name := g.names[id]
		switch decl.Kind {
		case contract.Struct:
			doc := decl.Doc
			if decl.Origin != "" && strings.TrimSpace(doc) == "" {
				doc = name + " is an instantiation of " + decl.Origin + "."
			}
			add(&declaration{id: id, name: name, kind: decl.Kind, emit: func(b *strings.Builder) {
				g.writeModel(b, name, doc, decl.Fields, nil)
			}})
		case contract.Generic:
			add(&declaration{id: id, name: name, kind: decl.Kind, emit: func(b *strings.Builder) {
				var fields []*contract.Field
				if decl.Body != nil {
					fields = decl.Body.Fields
				}
				g.writeModel(b, name, decl.Doc, fields, decl.Params)
			}})
		case contract.Enum:
			add(&declaration{id: id, name: name, kind: decl.Kind, emit: func(b *strings.Builder) {
				base := "str, Enum"
				if decl.Base != "" && decl.Base != "string" {
					base = "IntEnum"
					g.uses["IntEnum"] = true
				} else {
					g.uses["Enum"] = true
				}
				b.WriteString("class ")
				b.WriteString(name)
				b.WriteString("(")
				b.WriteString(base)
				b.WriteString("):\n")
				docstring(b, "    ", decl.Doc, "")
				if strings.TrimSpace(decl.Doc) != "" && len(decl.Values) > 0 {
					b.WriteString("\n")
				}
				if len(decl.Values) == 0 {
					b.WriteString("    pass\n")
				}
				for _, v := range decl.Values {
					b.WriteString("    ")
					b.WriteString(memberName(v.Name))
					b.WriteString(" = ")
					b.WriteString(literalValue(v.Value))
					b.WriteString("\n")
				}
				b.WriteString("\n\n")
			}})
		case contract.Primitive:
			add(&declaration{id: id, name: name, kind: decl.Kind, emit: func(b *strings.Builder) {
				g.uses["NewType"] = true
				b.WriteString(name)
				b.WriteString(" = NewType(\"")
				b.WriteString(name)
				b.WriteString("\", ")
				b.WriteString(g.primitive(decl.Primitive, ""))
				b.WriteString(")\n\n\n")
			}})
		}
	}
	for id, decl := range g.doc.Errors {
		name := g.names[id]
		doc := decl.Doc
		if strings.TrimSpace(doc) == "" {
			doc = name + " is the details shape of the " + name + " error variant (" + decl.Code + ")."
		}
		add(&declaration{id: id, name: name, kind: contract.Struct, emit: func(b *strings.Builder) {
			g.writeModel(b, name, doc, decl.Fields, nil)
		}})
	}
	rendered := map[string]string{}
	for _, d := range decls {
		var b strings.Builder
		d.emit(&b)
		rendered[d.name] = b.String()
	}
	for i := 0; i < len(g.inlineOK); i++ {
		owner := g.inlineOK[i]
		fields := g.inline[owner]
		d := &declaration{id: owner, name: owner, kind: contract.Struct}
		add(d)
		var b strings.Builder
		g.writeModel(&b, owner, "", fields, nil)
		rendered[owner] = b.String()
	}
	for _, d := range decls {
		d.deps = g.depsOf(d.id)
	}
	order, cyclic := topo(decls)
	var b strings.Builder
	for _, d := range order {
		b.WriteString(rendered[d.name])
	}
	if len(cyclic) > 0 {
		for _, d := range cyclic {
			b.WriteString(d.name)
			b.WriteString(".model_rebuild()\n")
		}
		b.WriteString("\n\n")
	}
	return b.String()
}

func (g *generator) depsOf(id string) []string {
	seen := map[string]bool{}
	var out []string
	visit := func(name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	var walk func(t *contract.Type, owner string)
	walk = func(t *contract.Type, owner string) {
		if t == nil {
			return
		}
		switch t.Kind {
		case contract.Ref:
			visit(g.names[t.ID])
			for _, a := range t.Args {
				walk(a, owner)
			}
		case contract.Array:
			walk(t.Elem, owner)
		case contract.Map:
			walk(t.Value, owner)
		case contract.Struct:
			if len(t.Fields) > 0 {
				visit(owner)
			}
		}
	}
	walkFields := func(fields []*contract.Field, owner string) {
		for _, f := range fields {
			walk(f.Type, owner+"_"+naming.UpperCamel(f.Name))
		}
	}
	if decl, ok := g.doc.Types[id]; ok {
		if decl.Kind == contract.Generic && decl.Body != nil {
			walkFields(decl.Body.Fields, g.names[id])
		} else {
			walkFields(decl.Fields, g.names[id])
		}
	} else if decl, ok := g.doc.Errors[id]; ok {
		walkFields(decl.Fields, g.names[id])
	} else if fields, ok := g.inline[id]; ok {
		walkFields(fields, id)
	}
	slices.Sort(out)
	return out
}

func topo(decls []*declaration) (order []*declaration, cyclic []*declaration) {
	byName := map[string]*declaration{}
	for _, d := range decls {
		byName[d.name] = d
	}
	indegree := map[string]int{}
	dependents := map[string][]string{}
	for _, d := range decls {
		indegree[d.name] += 0
		for _, dep := range d.deps {
			if dep == d.name {
				continue
			}
			if _, ok := byName[dep]; !ok {
				continue
			}
			indegree[d.name]++
			dependents[dep] = append(dependents[dep], d.name)
		}
	}
	var ready []string
	for name, n := range indegree {
		if n == 0 {
			ready = append(ready, name)
		}
	}
	slices.Sort(ready)
	done := map[string]bool{}
	for len(ready) > 0 {
		name := ready[0]
		ready = ready[1:]
		done[name] = true
		order = append(order, byName[name])
		for _, dep := range dependents[name] {
			indegree[dep]--
			if indegree[dep] == 0 {
				ready = append(ready, dep)
				slices.Sort(ready)
			}
		}
	}
	var rest []string
	for _, d := range decls {
		if !done[d.name] {
			rest = append(rest, d.name)
		}
	}
	slices.Sort(rest)
	for _, name := range rest {
		order = append(order, byName[name])
		cyclic = append(cyclic, byName[name])
	}
	for _, d := range decls {
		if done[d.name] {
			if slices.Contains(d.deps, d.name) {
				cyclic = append(cyclic, d)
			}
		}
	}
	slices.SortFunc(cyclic, func(a, b *declaration) int { return cmp.Compare(a.name, b.name) })
	return order, cyclic
}
