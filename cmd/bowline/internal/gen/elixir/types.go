package elixir

import (
	"strconv"
	"strings"

	"github.com/bowlinedev/bowline/contract"
)

type helperPool struct {
	names  map[string]bool
	defs   []string
	params []string
}

func (g *generator) extraParams(body string) []string {
	var extra []string
	for _, p := range g.helpers.params {
		for _, prefix := range []string{"decode", "encode", "validate"} {
			fn := paramFn(prefix, p)
			if strings.Contains(body, fn) {
				extra = append(extra, fn)
			}
		}
	}
	return extra
}

func (g *generator) fit(kind, atom, param, expr string, indent int) string {
	if indent+len(expr) <= lineWidth {
		return expr
	}
	base := kind + "_" + strings.TrimSuffix(atom, "_")
	name := base
	for i := 2; g.helpers.names[name]; i++ {
		name = base + "_" + strconv.Itoa(i)
	}
	g.helpers.names[name] = true
	args := append([]string{param}, g.extraParams(expr)...)
	g.helpers.defs = append(g.helpers.defs, "  defp "+name+"("+strings.Join(args, ", ")+") do\n    "+expr+"\n  end\n")
	return name + "(" + strings.Join(args, ", ") + ")"
}

func (g *generator) helper(kind, atom, body string) string {
	base := kind + "_" + strings.TrimSuffix(atom, "_")
	name := base
	for i := 2; g.helpers.names[name]; i++ {
		name = base + "_" + strconv.Itoa(i)
	}
	g.helpers.names[name] = true
	extra := g.extraParams(body)
	params := "value"
	capture := "&" + name + "/1"
	if len(extra) > 0 {
		params += ", " + strings.Join(extra, ", ")
		capture = "&" + name + "(&1, " + strings.Join(extra, ", ") + ")"
	}
	g.helpers.defs = append(g.helpers.defs, "  defp "+name+"("+params+") do\n    "+body+"\n  end\n")
	return capture
}

type inline struct {
	module string
	name   string
	fields []*contract.Field
}

func (g *generator) writeNominals(b *strings.Builder) {
	if len(g.nominals) == 0 {
		return
	}
	b.WriteString("defmodule " + g.types + " do\n")
	b.WriteString("  @moduledoc \"Named primitive types declared by the contract.\"\n")
	for _, id := range g.nominals {
		decl := g.doc.Types[id]
		b.WriteString("\n")
		if strings.TrimSpace(decl.Doc) != "" {
			writeModuleDoc(b, "  ", "@typedoc", decl.Doc)
		}
		b.WriteString("  @type " + nominalType(g.names[id]) + " :: " + g.primitiveSpec(decl.Primitive, "") + "\n")
	}
	b.WriteString("end\n\n")
}

func nominalType(name string) string {
	return fieldAtom(name)
}

func (g *generator) writeDecl(b *strings.Builder, id string, decl *contract.TypeDecl) {
	module := g.types + "." + g.names[id]
	switch decl.Kind {
	case contract.Struct:
		doc := decl.Doc
		if decl.Origin != "" {
			doc = docOr(doc, g.names[id]+" is an instantiation of "+decl.Origin+".")
		}
		g.writeStruct(b, module, g.names[id], doc, decl.Fields, nil, "")
	case contract.Generic:
		var fields []*contract.Field
		if decl.Body != nil {
			fields = decl.Body.Fields
		}
		g.writeStruct(b, module, g.names[id], decl.Doc, fields, decl.Params, "")
	case contract.Enum:
		g.writeEnum(b, module, g.names[id], decl)
	}
}

func (g *generator) writeEnum(out *strings.Builder, module, name string, decl *contract.TypeDecl) {
	b := &strings.Builder{}
	defer func() { g.emitModule(out, module, moduleDoc(decl.Doc), b.String()) }()
	atoms := make([]string, len(decl.Values))
	used := map[string]bool{}
	for i, v := range decl.Values {
		atom := enumAtom(v.Value)
		for used[atom] {
			atom += "_"
		}
		used[atom] = true
		atoms[i] = atom
	}
	valueSpec := "String.t()"
	if decl.Base != "" && decl.Base != "string" {
		valueSpec = "integer()"
	}
	if len(atoms) == 0 {
		b.WriteString("  @type t :: " + valueSpec + "\n\n")
		b.WriteString("  @spec from_value(term()) :: t()\n")
		b.WriteString("  def from_value(value), do: value\n\n")
		b.WriteString("  @spec to_value(t()) :: " + valueSpec + "\n")
		b.WriteString("  def to_value(value), do: value\n")
		return
	}
	b.WriteString("  @type t :: " + strings.Join(atoms, " | ") + "\n\n")
	b.WriteString("  @spec from_value(term()) :: t()\n")
	for i, v := range decl.Values {
		b.WriteString("  def from_value(" + literal(v.Value) + "), do: " + atoms[i] + "\n")
	}
	names := make([]string, len(decl.Values))
	for i, v := range decl.Values {
		names[i] = literalText(v.Value)
	}
	b.WriteString("\n  def from_value(other) do\n")
	b.WriteString("    Read.mismatch(" + quote(name) + ", " + quote("one of "+strings.Join(names, ", ")) + ", other)\n")
	b.WriteString("  end\n\n")
	b.WriteString("  @spec to_value(t()) :: " + valueSpec + "\n")
	for i, v := range decl.Values {
		b.WriteString("  def to_value(" + atoms[i] + "), do: " + literal(v.Value) + "\n")
	}
}

func literalText(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return literal(v)
}

func (g *generator) writeStruct(b *strings.Builder, module, name, doc string, fields []*contract.Field, params []string, keyPrefix string) {
	atoms := fieldAtoms(fields)
	var inlines []inline
	for i, f := range fields {
		g.collectInlines(module, atoms[i], f.Type, &inlines)
	}
	for _, in := range inlines {
		g.writeStruct(b, in.module, in.name, "", in.fields, params, "")
	}
	g.helpers = helperPool{names: map[string]bool{}, params: params}
	specParams := make([]string, len(params))
	fns := map[string]bool{}
	for i, p := range params {
		specParams[i] = paramSpec(p)
		fns[p] = true
	}
	typeName := "t"
	ret := "t()"
	if len(params) > 0 {
		typeName = "t(" + strings.Join(specParams, ", ") + ")"
		ret = "t(" + strings.TrimPrefix(strings.Repeat(", term()", len(params)), ", ") + ")"
	}
	out := b
	b = &strings.Builder{}
	defer func() {
		g.appendHelpers(b)
		g.emitModule(out, module, moduleDoc(doc), b.String())
	}()
	if len(fields) == 0 {
		b.WriteString("  @type " + typeName + " :: %__MODULE__{}\n\n")
		b.WriteString("  defstruct []\n\n")
		writeSpec(b, "from_map", specArgs("term()", len(params), "(term() -> term())"), ret)
		b.WriteString("  def from_map(_json" + fnArgs("decode", params, true) + "), do: %__MODULE__{}\n\n")
		writeSpec(b, "to_map", specArgs(ret, len(params), "(term() -> term())"), "map()")
		b.WriteString("  def to_map(%__MODULE__{}" + fnArgs("encode", params, true) + "), do: %{}\n\n")
		writeSpec(b, "validate", specArgs(ret, len(params), "(term() -> [BowlineClient.Issue.t()])"), "[BowlineClient.Issue.t()]")
		b.WriteString("  def validate(%__MODULE__{}" + fnArgs("validate", params, true) + "), do: []\n")
		return
	}
	b.WriteString("  @type " + typeName + " :: %__MODULE__{\n")
	for i, f := range fields {
		b.WriteString("          " + atoms[i] + ": " + g.fieldSpec(f, module, atoms[i], params) + ",\n")
	}
	trimComma(b)
	b.WriteString("\n        }\n\n")
	b.WriteString("  defstruct [\n")
	for _, a := range atoms {
		b.WriteString("    :" + a + ",\n")
	}
	trimComma(b)
	b.WriteString("\n  ]\n\n")
	writeSpec(b, "from_map", specArgs("term()", len(params), "(term() -> term())"), ret)
	b.WriteString("  def from_map(json" + fnArgs("decode", params, false) + ") do\n")
	b.WriteString("    map = Read.object(json, " + quote(name) + ")\n\n")
	b.WriteString("    %__MODULE__{\n")
	for i, f := range fields {
		b.WriteString("      " + atoms[i] + ": " + g.fit("field", atoms[i], "map", g.fieldDecode(f, module, atoms[i]), 8+len(atoms[i])) + ",\n")
	}
	trimComma(b)
	b.WriteString("\n    }\n  end\n\n")
	writeSpec(b, "to_map", specArgs(ret, len(params), "(term() -> term())"), "map()")
	b.WriteString("  def to_map(%__MODULE__{} = v" + fnArgs("encode", params, false) + ") do\n")
	required, optional := 0, 0
	for _, f := range fields {
		if f.Optional {
			optional++
		} else {
			required++
		}
	}
	if required == 0 {
		b.WriteString("    %{}\n")
	} else {
		b.WriteString("    %{\n")
		for i, f := range fields {
			if f.Optional {
				continue
			}
			b.WriteString("      " + quote(f.Name) + " => " + g.fit("wire", atoms[i], "v", g.fieldEncode(f, module, atoms[i]), 10+len(quote(f.Name))) + ",\n")
		}
		trimComma(b)
		b.WriteString("\n    }\n")
	}
	for i, f := range fields {
		if !f.Optional {
			continue
		}
		b.WriteString("    |> " + g.fit("wire", atoms[i], "v", "Encode.optional("+quote(f.Name)+", v."+atoms[i]+", "+g.encoderFn(f.Type, module, atoms[i])+")", 7) + "\n")
	}
	b.WriteString("  end\n\n")
	writeSpec(b, "validate", specArgs(ret, len(params), "(term() -> [BowlineClient.Issue.t()])"), "[BowlineClient.Issue.t()]")
	var checks []string
	var checkOwner []int
	for i, f := range fields {
		for _, c := range g.fieldChecks(f, module, atoms[i]) {
			checks = append(checks, c)
			checkOwner = append(checkOwner, i)
		}
	}
	if len(checks) == 0 {
		b.WriteString("  def validate(%__MODULE__{}" + fnArgs("validate", params, true) + "), do: []\n")
	} else {
		b.WriteString("  def validate(%__MODULE__{} = v" + fnArgs("validate", params, false) + ") do\n")
		b.WriteString("    List.flatten([\n")
		for i, c := range checks {
			b.WriteString("      " + g.fit("check", atoms[checkOwner[i]], "v", c, 7) + ",\n")
		}
		trimComma(b)
		b.WriteString("\n    ])\n  end\n")
	}
}

func (g *generator) emitModule(b *strings.Builder, module string, docAttr string, body string) {
	b.WriteString("defmodule " + module + " do\n")
	b.WriteString(docAttr)
	var aliases []string
	for _, a := range []string{"BowlineClient.Encode", "BowlineClient.Read", "BowlineClient.Rules", "BowlineClient.Transport", g.types} {
		short := a[strings.LastIndex(a, ".")+1:] + "."
		if strings.Contains(body, short) {
			aliases = append(aliases, a)
		}
	}
	for _, a := range aliases {
		b.WriteString("  alias " + a + "\n")
	}
	if len(aliases) > 0 {
		b.WriteString("\n")
	}
	b.WriteString(body)
	b.WriteString("end\n\n")
}

func moduleDoc(doc string) string {
	var b strings.Builder
	writeModuleDoc(&b, "  ", "@moduledoc", doc)
	b.WriteString("\n")
	return b.String()
}

func trimComma(b *strings.Builder) {
	s := strings.TrimSuffix(b.String(), ",\n")
	b.Reset()
	b.WriteString(s)
}

func specArgs(first string, n int, arg string) []string {
	args := []string{first}
	for i := 0; i < n; i++ {
		args = append(args, arg)
	}
	return args
}

func fnArgs(prefix string, params []string, unused bool) string {
	var b strings.Builder
	for _, p := range params {
		name := paramFn(prefix, p)
		if unused {
			name = "_" + name
		}
		b.WriteString(", " + name)
	}
	return b.String()
}

func (g *generator) collectInlines(module, atom string, t *contract.Type, out *[]inline) {
	if t == nil {
		return
	}
	switch t.Kind {
	case contract.Struct:
		if len(t.Fields) > 0 {
			name := inlineName(atom)
			*out = append(*out, inline{module: module + "." + name, name: name, fields: t.Fields})
		}
	case contract.Array:
		g.collectInlines(module, atom, t.Elem, out)
	case contract.Map:
		g.collectInlines(module, atom, t.Value, out)
	case contract.Ref:
		for _, a := range t.Args {
			g.collectInlines(module, atom, a, out)
		}
	}
}

func inlineName(atom string) string {
	parts := strings.Split(strings.TrimSuffix(atom, "_"), "_")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}

func (g *generator) fieldSpec(f *contract.Field, module, atom string, params []string) string {
	spec := g.typeSpec(f.Type, module, atom)
	if f.Nullable || f.Optional {
		spec += " | nil"
	}
	return spec
}

func (g *generator) typeSpec(t *contract.Type, module, atom string) string {
	var spec string
	switch t.Kind {
	case contract.Primitive:
		spec = g.primitiveSpec(t.Name, t.Encoding)
	case contract.Ref:
		decl, ok := g.doc.Types[t.ID]
		if !ok {
			spec = "term()"
			break
		}
		switch decl.Kind {
		case contract.Primitive:
			spec = "Types." + nominalType(g.names[t.ID]) + "()"
		case contract.Generic:
			args := make([]string, len(t.Args))
			for i, a := range t.Args {
				args[i] = g.typeSpec(a, module, atom)
			}
			spec = "Types." + g.names[t.ID] + ".t(" + strings.Join(args, ", ") + ")"
		default:
			spec = "Types." + g.names[t.ID] + ".t()"
		}
	case contract.Array:
		spec = "[" + g.typeSpec(t.Elem, module, atom) + "]"
	case contract.Map:
		spec = "%{optional(String.t()) => " + g.typeSpec(t.Value, module, atom) + "}"
	case contract.Struct:
		if len(t.Fields) == 0 {
			spec = "BowlineClient.Empty.t()"
		} else {
			spec = module + "." + inlineName(atom) + ".t()"
		}
	case contract.Param:
		spec = paramSpec(t.Name)
	default:
		spec = "term()"
	}
	if t.Nullable {
		spec += " | nil"
	}
	return spec
}

func (g *generator) primitiveSpec(name, encoding string) string {
	switch name {
	case "string", "bytes":
		if name == "bytes" {
			return "binary()"
		}
		return "String.t()"
	case "bool":
		return "boolean()"
	case "int8", "int16", "int32", "int64", "uint8", "uint16", "uint32", "uint64", "duration":
		return "integer()"
	case "float32", "float64":
		return "float()"
	case "timestamp":
		return "DateTime.t()"
	}
	return "term()"
}

func (g *generator) fieldDecode(f *contract.Field, module, atom string) string {
	value := "Map.get(map, " + quote(f.Name) + ")"
	if f.Nullable || f.Optional {
		return "Read.optional_value(" + value + ", " + g.decoderFn(f.Type, module, atom, f.Name) + ")"
	}
	return g.decodeExpr(f.Type, module, atom, f.Name, value)
}

func (g *generator) decodeExpr(t *contract.Type, module, atom, key, value string) string {
	if t.Nullable {
		return "Read.optional_value(" + value + ", " + g.decoderFn(t, module, atom, key) + ")"
	}
	return g.decodeBare(t, module, atom, key, value)
}

func (g *generator) decodeBare(t *contract.Type, module, atom, key, value string) string {
	switch t.Kind {
	case contract.Primitive:
		return primitiveDecode(t.Name, t.Encoding, key, value)
	case contract.Ref:
		decl, ok := g.doc.Types[t.ID]
		if !ok {
			return value
		}
		switch decl.Kind {
		case contract.Primitive:
			return primitiveDecode(decl.Primitive, t.Encoding, key, value)
		case contract.Enum:
			return "Types." + g.names[t.ID] + ".from_value(" + value + ")"
		case contract.Generic:
			args := make([]string, len(t.Args))
			for i, a := range t.Args {
				args[i] = g.decoderFn(a, module, atom, key)
			}
			return "Types." + g.names[t.ID] + ".from_map(" + value + ", " + strings.Join(args, ", ") + ")"
		default:
			return "Types." + g.names[t.ID] + ".from_map(" + value + ")"
		}
	case contract.Array:
		return "Read.list_value(" + value + ", " + quote(key) + ", " + g.decoderFn(t.Elem, module, atom, key) + ")"
	case contract.Map:
		return "Read.map_value(" + value + ", " + quote(key) + ", " + g.decoderFn(t.Value, module, atom, key) + ")"
	case contract.Struct:
		if len(t.Fields) == 0 {
			return "BowlineClient.Empty.from_map(" + value + ")"
		}
		return module + "." + inlineName(atom) + ".from_map(" + value + ")"
	case contract.Param:
		return paramFn("decode", t.Name) + ".(" + value + ")"
	}
	return value
}

func primitiveDecode(name, encoding, key, value string) string {
	switch name {
	case "string":
		return "Read.string_value(" + value + ", " + quote(key) + ")"
	case "bool":
		return "Read.bool_value(" + value + ", " + quote(key) + ")"
	case "int8", "int16", "int32", "int64", "uint8", "uint16", "uint32", "uint64", "duration":
		if encoding == "string" {
			return "Read.big_int_value(" + value + ", " + quote(key) + ")"
		}
		return "Read.int_value(" + value + ", " + quote(key) + ")"
	case "float32", "float64":
		return "Read.float_value(" + value + ", " + quote(key) + ")"
	case "timestamp":
		return "Read.timestamp_value(" + value + ", " + quote(key) + ")"
	case "bytes":
		return "Read.bytes_value(" + value + ", " + quote(key) + ")"
	}
	return value
}

func (g *generator) decoderFn(t *contract.Type, module, atom, key string) string {
	if t.Nullable {
		inner := *t
		inner.Nullable = false
		return g.helper("decode", atom, "Read.optional_value(value, "+g.decoderFn(&inner, module, atom, key)+")")
	}
	switch t.Kind {
	case contract.Ref:
		decl, ok := g.doc.Types[t.ID]
		if ok && len(t.Args) == 0 {
			switch decl.Kind {
			case contract.Struct:
				return "&" + "Types." + g.names[t.ID] + ".from_map/1"
			case contract.Enum:
				return "&" + "Types." + g.names[t.ID] + ".from_value/1"
			}
		}
	case contract.Struct:
		if len(t.Fields) == 0 {
			return "&BowlineClient.Empty.from_map/1"
		}
		return "&" + module + "." + inlineName(atom) + ".from_map/1"
	case contract.Param:
		return paramFn("decode", t.Name)
	case contract.Primitive:
		if t.Name == "raw" {
			return "&Function.identity/1"
		}
		return "&" + primitiveDecode(t.Name, t.Encoding, key, "&1")
	}
	return g.helper("decode", atom, g.decodeBare(t, module, atom, key, "value"))
}

func (g *generator) fieldEncode(f *contract.Field, module, atom string) string {
	value := "v." + atom
	if f.Nullable {
		return "Encode.nullable(" + value + ", " + g.encoderFn(f.Type, module, atom) + ")"
	}
	return g.encodeExpr(f.Type, module, atom, value)
}

func (g *generator) encodeExpr(t *contract.Type, module, atom, value string) string {
	if t.Nullable {
		return "Encode.nullable(" + value + ", " + g.encoderFn(t, module, atom) + ")"
	}
	return g.encodeBare(t, module, atom, value)
}

func (g *generator) encodeBare(t *contract.Type, module, atom, value string) string {
	switch t.Kind {
	case contract.Primitive:
		return primitiveEncode(t.Name, t.Encoding, value)
	case contract.Ref:
		decl, ok := g.doc.Types[t.ID]
		if !ok {
			return value
		}
		switch decl.Kind {
		case contract.Primitive:
			return primitiveEncode(decl.Primitive, t.Encoding, value)
		case contract.Enum:
			return "Types." + g.names[t.ID] + ".to_value(" + value + ")"
		case contract.Generic:
			args := make([]string, len(t.Args))
			for i, a := range t.Args {
				args[i] = g.encoderFn(a, module, atom)
			}
			return "Types." + g.names[t.ID] + ".to_map(" + value + ", " + strings.Join(args, ", ") + ")"
		default:
			return "Types." + g.names[t.ID] + ".to_map(" + value + ")"
		}
	case contract.Array:
		return "Encode.list(" + value + ", " + g.encoderFn(t.Elem, module, atom) + ")"
	case contract.Map:
		return "Encode.map(" + value + ", " + g.encoderFn(t.Value, module, atom) + ")"
	case contract.Struct:
		if len(t.Fields) == 0 {
			return "BowlineClient.Empty.to_map(" + value + ")"
		}
		return module + "." + inlineName(atom) + ".to_map(" + value + ")"
	case contract.Param:
		return paramFn("encode", t.Name) + ".(" + value + ")"
	}
	return value
}

func primitiveEncode(name, encoding, value string) string {
	switch name {
	case "int8", "int16", "int32", "int64", "uint8", "uint16", "uint32", "uint64":
		if encoding == "string" {
			return "Encode.big_int(" + value + ")"
		}
	case "timestamp":
		return "Encode.timestamp(" + value + ")"
	case "bytes":
		return "Encode.bytes(" + value + ")"
	}
	return value
}

func (g *generator) encoderFn(t *contract.Type, module, atom string) string {
	if t.Nullable {
		inner := *t
		inner.Nullable = false
		return g.helper("encode", atom, "Encode.nullable(value, "+g.encoderFn(&inner, module, atom)+")")
	}
	switch t.Kind {
	case contract.Primitive:
		switch t.Name {
		case "timestamp":
			return "&Encode.timestamp/1"
		case "bytes":
			return "&Encode.bytes/1"
		case "int8", "int16", "int32", "int64", "uint8", "uint16", "uint32", "uint64":
			if t.Encoding == "string" {
				return "&Encode.big_int/1"
			}
		}
		return "&Function.identity/1"
	case contract.Ref:
		decl, ok := g.doc.Types[t.ID]
		if ok && len(t.Args) == 0 {
			switch decl.Kind {
			case contract.Struct:
				return "&" + "Types." + g.names[t.ID] + ".to_map/1"
			case contract.Enum:
				return "&" + "Types." + g.names[t.ID] + ".to_value/1"
			case contract.Primitive:
				return g.encoderFn(&contract.Type{Kind: contract.Primitive, Name: decl.Primitive, Encoding: t.Encoding}, module, atom)
			}
		}
	case contract.Struct:
		if len(t.Fields) == 0 {
			return "&BowlineClient.Empty.to_map/1"
		}
		return "&" + module + "." + inlineName(atom) + ".to_map/1"
	case contract.Param:
		return paramFn("encode", t.Name)
	}
	return g.helper("encode", atom, g.encodeBare(t, module, atom, "value"))
}

func (g *generator) fieldChecks(f *contract.Field, module, atom string) []string {
	var checks []string
	path := "[" + quote(f.Name) + "]"
	value := "v." + atom
	for _, r := range f.Rules {
		switch r.Rule {
		case "required":
			checks = append(checks, "Rules.required("+path+", "+g.ruleValue(f, value)+")")
		case "min", "max", "len":
			checks = append(checks, "Rules."+r.Rule+"("+path+", "+g.ruleValue(f, value)+", "+numberLiteral(r.Param)+")")
		case "oneof":
			options := strings.Fields(r.Param)
			quoted := make([]string, len(options))
			for i, o := range options {
				quoted[i] = quote(o)
			}
			checks = append(checks, "Rules.one_of("+path+", "+g.ruleValue(f, value)+", ["+strings.Join(quoted, ", ")+"])")
		case "email", "url", "uuid":
			checks = append(checks, "Rules."+r.Rule+"("+path+", "+g.ruleValue(f, value)+")")
		}
	}
	if f.Nullable || f.Optional || f.Type.Nullable {
		if nested := g.validateExpr(f.Type, module, atom, path, "value"); nested != "" {
			checks = append(checks, "Read.validate_optional("+value+", "+g.helper("validate", atom, nested)+")")
		}
	} else if nested := g.validateExpr(f.Type, module, atom, path, value); nested != "" {
		checks = append(checks, nested)
	}
	return checks
}

func (g *generator) validateExpr(t *contract.Type, module, atom, path, value string) string {
	switch t.Kind {
	case contract.Ref:
		decl, ok := g.doc.Types[t.ID]
		if !ok {
			return ""
		}
		switch decl.Kind {
		case contract.Struct:
			return "Read.prefixed(" + path + ", " + "Types." + g.names[t.ID] + ".validate(" + value + "))"
		case contract.Generic:
			return "Read.prefixed(" + path + ", " + "Types." + g.names[t.ID] + ".validate(" + value + ", " + g.genericValidators(t, module, atom) + "))"
		}
	case contract.Array:
		elem := g.validatorFn(t.Elem, module, atom)
		if elem == "" {
			return ""
		}
		return "Read.validate_list(" + path + ", " + value + ", " + elem + ")"
	case contract.Map:
		elem := g.validatorFn(t.Value, module, atom)
		if elem == "" {
			return ""
		}
		return "Read.validate_map(" + path + ", " + value + ", " + elem + ")"
	case contract.Struct:
		if len(t.Fields) == 0 {
			return ""
		}
		return "Read.prefixed(" + path + ", " + module + "." + inlineName(atom) + ".validate(" + value + "))"
	case contract.Param:
		return "Read.prefixed(" + path + ", " + paramFn("validate", t.Name) + ".(" + value + "))"
	}
	return ""
}

func (g *generator) genericValidators(t *contract.Type, module, atom string) string {
	args := make([]string, len(t.Args))
	for i, a := range t.Args {
		fn := g.validatorFn(a, module, atom)
		if fn == "" {
			fn = "fn _ -> [] end"
		}
		args[i] = fn
	}
	return strings.Join(args, ", ")
}

func (g *generator) ruleValue(f *contract.Field, value string) string {
	if f.Type.Kind == contract.Ref {
		if decl, ok := g.doc.Types[f.Type.ID]; ok && decl.Kind == contract.Enum {
			if f.Nullable || f.Optional || f.Type.Nullable {
				return "Encode.nullable(" + value + ", &Types." + g.names[f.Type.ID] + ".to_value/1)"
			}
			return "Types." + g.names[f.Type.ID] + ".to_value(" + value + ")"
		}
	}
	return value
}

func numberLiteral(param string) string {
	if _, err := strconv.ParseInt(param, 10, 64); err == nil {
		return param
	}
	if f, err := strconv.ParseFloat(param, 64); err == nil {
		return strconv.FormatFloat(f, 'g', -1, 64)
	}
	return "0"
}

func (g *generator) validatorFn(t *contract.Type, module, atom string) string {
	if t.Nullable {
		inner := *t
		inner.Nullable = false
		fn := g.validatorFn(&inner, module, atom)
		if fn == "" {
			return ""
		}
		return g.helper("validate", atom, "Read.validate_optional(value, "+fn+")")
	}
	switch t.Kind {
	case contract.Ref:
		decl, ok := g.doc.Types[t.ID]
		if !ok {
			return ""
		}
		switch decl.Kind {
		case contract.Struct:
			return "&" + "Types." + g.names[t.ID] + ".validate/1"
		case contract.Generic:
			return g.helper("validate", atom, "Types."+g.names[t.ID]+".validate(value, "+g.genericValidators(t, module, atom)+")")
		}
	case contract.Array, contract.Map:
		expr := g.validateExpr(t, module, atom, "[]", "value")
		if expr == "" {
			return ""
		}
		return g.helper("validate", atom, expr)
	case contract.Struct:
		if len(t.Fields) == 0 {
			return ""
		}
		return "&" + module + "." + inlineName(atom) + ".validate/1"
	case contract.Param:
		return paramFn("validate", t.Name)
	}
	return ""
}
