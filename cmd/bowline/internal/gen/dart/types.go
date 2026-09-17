package dart

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/bowlinedev/bowline/contract"
)

func (g *generator) primitiveType(name, encoding string) string {
	switch name {
	case "string":
		return "String"
	case "bool":
		return "bool"
	case "int8", "int16", "int32", "uint8", "uint16", "uint32":
		return "int"
	case "int64", "uint64":
		if encoding == "string" {
			return "BigInt"
		}
		return "int"
	case "float32", "float64":
		return "double"
	case "timestamp":
		return "DateTime"
	case "duration":
		return "DurationNs"
	case "bytes":
		g.uses["typed_data"] = true
		return "Uint8List"
	}
	return "Object?"
}

func (g *generator) dartType(t *contract.Type) string {
	var s string
	switch t.Kind {
	case contract.Primitive:
		s = g.primitiveType(t.Name, t.Encoding)
	case contract.Ref:
		decl := g.doc.Types[t.ID]
		if decl != nil && decl.Kind == contract.Primitive {
			s = g.names[t.ID]
		} else {
			s = g.names[t.ID]
			if len(t.Args) > 0 {
				args := make([]string, len(t.Args))
				for i, a := range t.Args {
					args[i] = g.dartType(a)
				}
				s += "<" + strings.Join(args, ", ") + ">"
			}
		}
	case contract.Array:
		s = "List<" + g.dartType(t.Elem) + ">"
	case contract.Map:
		s = "Map<String, " + g.dartType(t.Value) + ">"
	case contract.Struct:
		if len(t.Fields) == 0 {
			s = "Empty"
		} else {
			s = g.inlineType(t)
		}
	case contract.Param:
		s = t.Name
	default:
		s = "Object?"
	}
	if t.Nullable && !strings.HasSuffix(s, "?") {
		s += "?"
	}
	return s
}

func (g *generator) inlineType(t *contract.Type) string {
	decl := g.inline[t]
	if len(decl.params) == 0 {
		return decl.name
	}
	return decl.name + "<" + strings.Join(decl.params, ", ") + ">"
}

func (g *generator) fieldType(f *contract.Field) string {
	typ := g.dartType(f.Type)
	if (f.Nullable || f.Optional) && !strings.HasSuffix(typ, "?") {
		typ += "?"
	}
	return typ
}

func (g *generator) underlying(t *contract.Type) *contract.Type {
	if t.Kind == contract.Ref {
		if decl := g.doc.Types[t.ID]; decl != nil && decl.Kind == contract.Primitive {
			return &contract.Type{Kind: contract.Primitive, Name: decl.Primitive, Encoding: t.Encoding, Nullable: t.Nullable}
		}
	}
	return t
}

func (g *generator) decoder(t *contract.Type, key string) string {
	return g.decoderAt(t, key, 0)
}

func (g *generator) decoderAt(t *contract.Type, key string, depth int) string {
	if tearoff := g.tearoff(t); tearoff != "" {
		return tearoff
	}
	v := fmt.Sprintf("v%d", depth)
	return "(" + v + ") => " + g.decodeBodyAt(t, key, v, depth+1)
}

func (g *generator) tearoff(t *contract.Type) string {
	if t.Nullable {
		return ""
	}
	switch t.Kind {
	case contract.Primitive:
		return g.primitiveDecoder(t.Name, t.Encoding)
	case contract.Ref:
		if decl := g.doc.Types[t.ID]; decl != nil && decl.Kind == contract.Primitive {
			return g.primitiveDecoder(decl.Primitive, t.Encoding)
		}
	case contract.Struct:
		if len(t.Fields) == 0 {
			return "Empty.fromJson"
		}
	case contract.Param:
		return "fromJson" + t.Name
	}
	return ""
}

func (g *generator) decodeBody(t *contract.Type, key, v string) string {
	return g.decodeBodyAt(t, key, v, 0)
}

func (g *generator) decodeBodyAt(t *contract.Type, key, v string, depth int) string {
	if t.Nullable {
		plain := *t
		plain.Nullable = false
		return "asNullable(" + v + ", " + g.decoderAt(&plain, key, depth) + ")"
	}
	if tearoff := g.tearoff(t); tearoff != "" {
		return tearoff + "(" + v + ")"
	}
	switch t.Kind {
	case contract.Ref:
		decl := g.doc.Types[t.ID]
		if decl == nil {
			return g.names[t.ID] + ".fromJson(asObject(" + v + "))"
		}
		switch decl.Kind {
		case contract.Enum:
			return g.names[t.ID] + ".fromValue(" + v + ")"
		case contract.Generic:
			args := make([]string, len(t.Args))
			for i, a := range t.Args {
				args[i] = g.decoderAt(a, key, depth)
			}
			return g.names[t.ID] + ".fromJson(asObject(" + v + "), " + strings.Join(args, ", ") + ")"
		default:
			return g.names[t.ID] + ".fromJson(asObject(" + v + "))"
		}
	case contract.Array:
		return "asList(" + v + ", " + g.decoderAt(t.Elem, key, depth) + ")"
	case contract.Map:
		return "asMap(" + v + ", " + g.decoderAt(t.Value, key, depth) + ")"
	case contract.Struct:
		return g.inline[t].name + ".fromJson(asObject(" + v + "))"
	}
	return "asRaw(" + v + ")"
}

func (g *generator) primitiveDecoder(name, encoding string) string {
	switch name {
	case "string":
		return "asString"
	case "bool":
		return "asBool"
	case "int8", "int16", "int32", "uint8", "uint16", "uint32", "duration":
		return "asInt"
	case "int64", "uint64":
		if encoding == "string" {
			return "asBigInt"
		}
		return "asInt"
	case "float32", "float64":
		return "asDouble"
	case "timestamp":
		return "asTimestamp"
	case "bytes":
		return "asBytes"
	}
	return "asRaw"
}

func (g *generator) fieldDecode(f *contract.Field) string {
	key := quote(f.Name)
	t := f.Type
	optional := f.Optional || f.Nullable
	if u := g.underlying(t); u.Kind == contract.Primitive && !t.Nullable {
		reader := g.primitiveReader(u.Name, u.Encoding)
		if reader != "" {
			if optional {
				return "readOptional" + reader + "(json, " + key + ")"
			}
			return "read" + reader + "(json, " + key + ")"
		}
		if u.Name == "raw" {
			return "readRaw(json, " + key + ")"
		}
	}
	if optional && !t.Nullable {
		return "asNullable(json[" + key + "], " + g.decoder(t, f.Name) + ")"
	}
	if t.Kind == contract.Array && !t.Nullable {
		return "readList(json, " + key + ", " + g.decoder(t.Elem, f.Name) + ")"
	}
	if t.Kind == contract.Map && !t.Nullable {
		return "readMap(json, " + key + ", " + g.decoder(t.Value, f.Name) + ")"
	}
	return g.decodeBody(t, f.Name, "json["+key+"]")
}

func (g *generator) primitiveReader(name, encoding string) string {
	switch name {
	case "string":
		return "String"
	case "bool":
		return "Bool"
	case "int8", "int16", "int32", "uint8", "uint16", "uint32", "duration":
		return "Int"
	case "int64", "uint64":
		if encoding == "string" {
			return "BigInt"
		}
		return "Int"
	case "float32", "float64":
		return "Double"
	case "timestamp":
		return "Timestamp"
	case "bytes":
		return "Bytes"
	}
	return ""
}

func (g *generator) encode(t *contract.Type, expr string, depth int) string {
	if t.Nullable {
		plain := *t
		plain.Nullable = false
		inner := g.encode(&plain, expr, depth)
		if inner == expr {
			return expr
		}
		return expr + " == null ? null : " + inner
	}
	switch t.Kind {
	case contract.Primitive:
		return g.primitiveEncode(t.Name, t.Encoding, expr)
	case contract.Ref:
		decl := g.doc.Types[t.ID]
		if decl == nil {
			return expr + ".toJson()"
		}
		switch decl.Kind {
		case contract.Primitive:
			return g.primitiveEncode(decl.Primitive, t.Encoding, expr)
		case contract.Enum:
			return expr + ".value"
		case contract.Generic:
			args := make([]string, len(t.Args))
			for i, a := range t.Args {
				v := fmt.Sprintf("e%d", depth)
				args[i] = "(" + v + ") => " + g.encode(a, v, depth+1)
			}
			return expr + ".toJson(" + strings.Join(args, ", ") + ")"
		default:
			return expr + ".toJson()"
		}
	case contract.Array:
		v := fmt.Sprintf("e%d", depth)
		inner := g.encode(t.Elem, v, depth+1)
		if inner == v {
			return expr
		}
		return "[for (final " + v + " in " + expr + ") " + inner + "]"
	case contract.Map:
		k := fmt.Sprintf("k%d", depth)
		v := fmt.Sprintf("v%d", depth)
		inner := g.encode(t.Value, v, depth+1)
		if inner == v {
			return expr
		}
		return "{for (final MapEntry(key: " + k + ", value: " + v + ") in " + expr + ".entries) " + k + ": " + inner + "}"
	case contract.Struct:
		return expr + ".toJson()"
	case contract.Param:
		return "toJson" + t.Name + "(" + expr + ")"
	}
	return expr
}

func (g *generator) primitiveEncode(name, encoding, expr string) string {
	switch name {
	case "timestamp":
		return "encodeTimestamp(" + expr + ")"
	case "bytes":
		return "encodeBytes(" + expr + ")"
	case "int64", "uint64":
		if encoding == "string" {
			return expr + ".toString()"
		}
	}
	return expr
}

func (g *generator) fieldEncode(f *contract.Field, member string) string {
	t := *f.Type
	if f.Optional {
		t.Nullable = false
	} else if f.Nullable {
		t.Nullable = true
	}
	return g.encode(&t, member, 0)
}

func (g *generator) writeClass(b *strings.Builder, name string, params []string, fields []*contract.Field, doc string) {
	members := memberNames(fields)
	writeDoc(b, "", doc)
	typeParams := ""
	if len(params) > 0 {
		typeParams = "<" + strings.Join(params, ", ") + ">"
	}
	b.WriteString("class ")
	b.WriteString(name)
	b.WriteString(typeParams)
	b.WriteString(" {\n")
	var ctor []string
	for _, f := range fields {
		m := members[f.Name]
		if g.isOptionalMember(f) {
			ctor = append(ctor, "this."+m)
		} else {
			ctor = append(ctor, "required this."+m)
		}
	}
	if len(fields) == 0 {
		b.WriteString("  const ")
		b.WriteString(name)
		b.WriteString("();\n")
	} else {
		b.WriteString("  ")
		b.WriteString(name)
		b.WriteString("({")
		b.WriteString(strings.Join(ctor, ", "))
		b.WriteString("});\n")
	}
	var fromArgs, toArgs, validateArgs strings.Builder
	for _, p := range params {
		fromArgs.WriteString(", ")
		fromArgs.WriteString(p)
		fromArgs.WriteString(" Function(Object?) fromJson")
		fromArgs.WriteString(p)
		toArgs.WriteString(", Object? Function(")
		toArgs.WriteString(p)
		toArgs.WriteString(") toJson")
		toArgs.WriteString(p)
		validateArgs.WriteString(", List<Issue> Function(")
		validateArgs.WriteString(p)
		validateArgs.WriteString(") validate")
		validateArgs.WriteString(p)
	}
	b.WriteString("\n  factory ")
	b.WriteString(name)
	b.WriteString(".fromJson(Map<String, Object?> json")
	b.WriteString(fromArgs.String())
	b.WriteString(") => ")
	b.WriteString(name)
	b.WriteString("(")
	if len(fields) > 0 {
		b.WriteString("\n")
		for _, f := range fields {
			b.WriteString("        ")
			b.WriteString(members[f.Name])
			b.WriteString(": ")
			b.WriteString(g.fieldDecode(f))
			b.WriteString(",\n")
		}
		b.WriteString("      ")
	}
	b.WriteString(");\n")
	for _, f := range fields {
		b.WriteString("\n")
		writeDoc(b, "  ", f.Doc)
		b.WriteString("  final ")
		b.WriteString(g.fieldType(f))
		b.WriteString(" ")
		b.WriteString(members[f.Name])
		b.WriteString(";\n")
	}
	var entries strings.Builder
	var jsonLocals []string
	for _, f := range fields {
		m := members[f.Name]
		enc := g.fieldEncode(f, m)
		if f.Optional {
			entries.WriteString("        if (")
			entries.WriteString(m)
			entries.WriteString(" != null) ")
			entries.WriteString(quote(f.Name))
			entries.WriteString(": ")
			entries.WriteString(enc)
			entries.WriteString(",\n")
		} else {
			entries.WriteString("        ")
			entries.WriteString(quote(f.Name))
			entries.WriteString(": ")
			entries.WriteString(enc)
			entries.WriteString(",\n")
		}
		if g.isOptionalMember(f) && enc != m {
			jsonLocals = append(jsonLocals, m)
		}
	}
	b.WriteString("\n  Map<String, Object?> toJson(")
	b.WriteString(strings.TrimPrefix(toArgs.String(), ", "))
	b.WriteString(")")
	if len(jsonLocals) == 0 {
		if len(fields) == 0 {
			b.WriteString(" => {};\n")
		} else {
			b.WriteString(" => {\n")
			b.WriteString(entries.String())
			b.WriteString("      };\n")
		}
	} else {
		b.WriteString(" {\n")
		b.WriteString(locals(jsonLocals))
		b.WriteString("    return {\n")
		b.WriteString(entries.String())
		b.WriteString("    };\n  }\n")
	}
	checks := g.validations(fields, members)
	var checkLocals []string
	for _, f := range fields {
		m := members[f.Name]
		if !g.isOptionalMember(f) {
			continue
		}
		for _, c := range checks {
			if mentions(c, m) {
				checkLocals = append(checkLocals, m)
				break
			}
		}
	}
	b.WriteString("\n  List<Issue> validate(")
	b.WriteString(strings.TrimPrefix(validateArgs.String(), ", "))
	b.WriteString(")")
	switch {
	case len(checks) == 0:
		b.WriteString(" => const [];\n")
	case len(checkLocals) == 0:
		b.WriteString(" => [\n")
		for _, c := range checks {
			b.WriteString("        ")
			b.WriteString(c)
			b.WriteString(",\n")
		}
		b.WriteString("      ];\n")
	default:
		b.WriteString(" {\n")
		b.WriteString(locals(checkLocals))
		b.WriteString("    return [\n")
		for _, c := range checks {
			b.WriteString("      ")
			b.WriteString(c)
			b.WriteString(",\n")
		}
		b.WriteString("    ];\n  }\n")
	}
	b.WriteString("}\n\n")
}

func (g *generator) isOptionalMember(f *contract.Field) bool {
	return f.Optional || f.Nullable || f.Type.Nullable
}

func (g *generator) writeEnum(b *strings.Builder, name string, decl *contract.TypeDecl) {
	writeDoc(b, "", decl.Doc)
	base := "String"
	if decl.Base != "" && decl.Base != "string" {
		base = "int"
	}
	b.WriteString("enum ")
	b.WriteString(name)
	b.WriteString(" {\n")
	used := map[string]bool{}
	for i, v := range decl.Values {
		member := enumMember(v.Name, decl.Name)
		for used[member] {
			member += "_"
		}
		used[member] = true
		sep := ","
		if i == len(decl.Values)-1 {
			sep = ";"
		}
		b.WriteString("  ")
		b.WriteString(member)
		b.WriteString("(")
		b.WriteString(literal(v.Value))
		b.WriteString(")")
		b.WriteString(sep)
		b.WriteString("\n")
	}
	b.WriteString("\n  const ")
	b.WriteString(name)
	b.WriteString("(this.value);\n\n")
	b.WriteString("  final ")
	b.WriteString(base)
	b.WriteString(" value;\n\n")
	b.WriteString("  static ")
	b.WriteString(name)
	b.WriteString(" fromValue(Object? value) => values.firstWhere(\n")
	b.WriteString("        (v) => v.value == value,\n")
	b.WriteString("        orElse: () => throw FormatException('")
	b.WriteString(name)
	b.WriteString(": unknown value $value'),\n")
	b.WriteString("      );\n}\n\n")
}

func enumMember(constName, typeName string) string {
	name := constName
	if len(constName) > len(typeName) && strings.EqualFold(constName[:len(typeName)], typeName) {
		name = constName[len(typeName):]
	}
	member := fieldName(name)
	if reservedMembers[member] || reserved[member] {
		member += "_"
	}
	return member
}

func literal(v any) string {
	switch x := v.(type) {
	case string:
		return quote(x)
	case json.Number:
		return x.String()
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	}
	return fmt.Sprint(v)
}

func (g *generator) declarations(b *strings.Builder) error {
	for _, id := range g.order {
		decl := g.doc.Types[id]
		name := g.names[id]
		switch decl.Kind {
		case contract.Struct:
			doc := decl.Doc
			if decl.Origin != "" && strings.TrimSpace(doc) == "" {
				doc = name + " is an instantiation of " + decl.Origin + "."
			}
			g.writeClass(b, name, nil, decl.Fields, doc)
		case contract.Generic:
			var fields []*contract.Field
			if decl.Body != nil {
				fields = decl.Body.Fields
			}
			g.writeClass(b, name, decl.Params, fields, decl.Doc)
		case contract.Enum:
			g.writeEnum(b, name, decl)
		case contract.Primitive:
			writeDoc(b, "", decl.Doc)
			b.WriteString("typedef ")
			b.WriteString(name)
			b.WriteString(" = ")
			b.WriteString(g.primitiveType(decl.Primitive, ""))
			b.WriteString(";\n\n")
		default:
			return fmt.Errorf("dart: unsupported declaration kind %q for %s", decl.Kind, id)
		}
	}
	for _, id := range g.errorOrder() {
		decl := g.doc.Errors[id]
		doc := decl.Doc
		if strings.TrimSpace(doc) == "" {
			doc = decl.Name + " is the details shape of the " + decl.Name + " error variant (" + decl.Code + ")."
		}
		g.writeClass(b, g.names[id], nil, decl.Fields, doc)
	}
	for _, inline := range g.inlines {
		g.writeClass(b, inline.name, inline.params, inline.fields, "")
	}
	return nil
}

func locals(names []string) string {
	var b strings.Builder
	for _, n := range names {
		b.WriteString("    final ")
		b.WriteString(n)
		b.WriteString(" = this.")
		b.WriteString(n)
		b.WriteString(";\n")
	}
	return b.String()
}

func mentions(code, name string) bool {
	for i := 0; ; {
		j := strings.Index(code[i:], name)
		if j < 0 {
			return false
		}
		start := i + j
		end := start + len(name)
		before := start == 0 || !isIdent(code[start-1])
		after := end == len(code) || !isIdent(code[end])
		if before && after {
			return true
		}
		i = end
	}
}

func isIdent(c byte) bool {
	return c == '_' || c == '$' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}
