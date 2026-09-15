package rust

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/bowlinedev/bowline/contract"
)

const derives = "#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]"

type scope struct {
	owner  string
	params map[string]bool
	prefix string
}

func (g *generator) rustType(t *contract.Type, s scope, nested bool) string {
	var out string
	switch t.Kind {
	case contract.Primitive:
		out = g.primitive(t.Name, t.Encoding, nested)
	case contract.Ref:
		out = g.ref(t, s, nested)
	case contract.Array:
		if t.Length > 0 {
			out = "[" + g.rustType(t.Elem, s, true) + "; " + strconv.Itoa(t.Length) + "]"
		} else {
			out = "Vec<" + g.rustType(t.Elem, heap(s), true) + ">"
		}
	case contract.Map:
		out = "std::collections::BTreeMap<" + g.rustType(t.Key, heap(s), true) + ", " + g.rustType(t.Value, heap(s), true) + ">"
	case contract.Struct:
		out = g.synthesize(t.Fields, s)
	case contract.Param:
		out = t.Name
	default:
		out = "serde_json::Value"
	}
	if t.Nullable {
		out = "Option<" + out + ">"
	}
	return out
}

func heap(s scope) scope {
	return scope{params: s.params, prefix: s.prefix}
}

func (g *generator) primitive(name, encoding string, nested bool) string {
	switch name {
	case "string":
		return "String"
	case "bool":
		return "bool"
	case "int8", "int16", "int32", "int64":
		if encoding == "string" && name == "int64" && nested {
			return "bowline_client::StringInt"
		}
		return "i" + strings.TrimPrefix(name, "int")
	case "uint8", "uint16", "uint32", "uint64":
		if encoding == "string" && name == "uint64" && nested {
			return "bowline_client::StringUint"
		}
		return "u" + strings.TrimPrefix(name, "uint")
	case "float32":
		return "f32"
	case "float64":
		return "f64"
	case "timestamp":
		return "chrono::DateTime<chrono::Utc>"
	case "duration":
		return "bowline_client::DurationNs"
	case "bytes":
		if nested {
			return "bowline_client::Base64Bytes"
		}
		return "Vec<u8>"
	}
	return "serde_json::Value"
}

func (g *generator) ref(t *contract.Type, s scope, nested bool) string {
	decl, ok := g.doc.Types[t.ID]
	if !ok {
		return "serde_json::Value"
	}
	switch decl.Kind {
	case contract.Primitive:
		if t.Encoding == "string" || decl.Primitive == "bytes" {
			return g.primitive(decl.Primitive, t.Encoding, nested)
		}
		return g.names[t.ID]
	case contract.Generic:
		args := make([]string, len(t.Args))
		for i, a := range t.Args {
			args[i] = g.rustType(a, s, true)
		}
		return g.names[t.ID] + "<" + strings.Join(args, ", ") + ">"
	case contract.Struct:
		name := g.names[t.ID]
		if s.owner != "" && g.reaches(t.ID, s.owner, map[string]bool{}) {
			return "Box<" + name + ">"
		}
		return name
	}
	return g.names[t.ID]
}

func (g *generator) reaches(from, target string, seen map[string]bool) bool {
	if from == target {
		return true
	}
	if seen[from] {
		return false
	}
	seen[from] = true
	decl, ok := g.doc.Types[from]
	if !ok || decl.Kind != contract.Struct {
		return false
	}
	for _, f := range decl.Fields {
		for _, id := range directRefs(f.Type) {
			if g.reaches(id, target, seen) {
				return true
			}
		}
	}
	return false
}

func directRefs(t *contract.Type) []string {
	if t == nil {
		return nil
	}
	switch t.Kind {
	case contract.Ref:
		return []string{t.ID}
	case contract.Struct:
		var out []string
		for _, f := range t.Fields {
			out = append(out, directRefs(f.Type)...)
		}
		return out
	case contract.Array:
		if t.Length > 0 {
			return directRefs(t.Elem)
		}
	}
	return nil
}

func (g *generator) synthesize(fields []*contract.Field, s scope) string {
	name := s.prefix
	if name == "" {
		name = "Inline"
	}
	if _, done := g.synthBody[name]; !done {
		g.synthBody[name] = ""
		g.synthetic = append(g.synthetic, name)
		var b strings.Builder
		g.writeStruct(&b, name, "", fields, nil, scope{owner: s.owner, params: s.params, prefix: name})
		g.synthBody[name] = b.String()
	}
	return name
}

type fieldInfo struct {
	name   string
	typ    string
	option bool
	attrs  []string
}

func (g *generator) field(f *contract.Field, s scope) fieldInfo {
	info := fieldInfo{name: fieldName(f.Name)}
	inner := scope{owner: s.owner, params: s.params, prefix: s.prefix + typeName(f.Name)}
	typ := g.rustType(f.Type, inner, false)
	option := f.Type.Nullable
	if (f.Nullable || f.Optional) && !option {
		typ = "Option<" + typ + ">"
		option = true
	}
	info.typ = typ
	info.option = option
	if info.name != f.Name && strings.TrimPrefix(info.name, "r#") != f.Name {
		info.attrs = append(info.attrs, "rename = "+strconv.Quote(f.Name))
	}
	if f.Optional {
		info.attrs = append(info.attrs, "default", `skip_serializing_if = "Option::is_none"`)
	}
	if with := g.with(f.Type, option); with != "" {
		if !f.Optional && option {
			info.attrs = append(info.attrs, "default")
		}
		info.attrs = append(info.attrs, "with = "+strconv.Quote(with))
	}
	return info
}

func (g *generator) with(t *contract.Type, option bool) string {
	name, encoding := "", t.Encoding
	switch t.Kind {
	case contract.Primitive:
		name = t.Name
	case contract.Ref:
		if decl, ok := g.doc.Types[t.ID]; ok && decl.Kind == contract.Primitive {
			name = decl.Primitive
		}
	}
	suffix := ""
	if option {
		suffix = "_option"
	}
	switch {
	case name == "bytes":
		return "bowline_client::codec::base64" + suffix
	case name == "int64" && encoding == "string":
		return "bowline_client::codec::string_int" + suffix
	case name == "uint64" && encoding == "string":
		return "bowline_client::codec::string_uint" + suffix
	}
	return ""
}

func (g *generator) writeStruct(b *strings.Builder, name, doc string, fields []*contract.Field, params []string, s scope) {
	g.uses["serde"] = true
	writeDoc(b, "", doc)
	b.WriteString(derives + "\n")
	b.WriteString("pub struct " + name + generics(params, g.mapKeyParams(fields)) + " {\n")
	used := map[string]bool{}
	var infos []fieldInfo
	for _, f := range fields {
		info := g.field(f, s)
		for used[info.name] {
			info.name += "_"
		}
		used[info.name] = true
		infos = append(infos, info)
		writeDoc(b, "    ", f.Doc)
		if len(info.attrs) > 0 {
			b.WriteString("    #[serde(" + strings.Join(info.attrs, ", ") + ")]\n")
		}
		b.WriteString("    pub " + info.name + ": " + info.typ + ",\n")
	}
	b.WriteString("}\n\n")
	g.writeValidate(b, name, fields, infos, params, s)
}

func generics(params []string, keys map[string]bool) string {
	if len(params) == 0 {
		return ""
	}
	parts := make([]string, len(params))
	for i, p := range params {
		if keys[p] {
			parts[i] = p + ": Ord"
		} else {
			parts[i] = p
		}
	}
	return "<" + strings.Join(parts, ", ") + ">"
}

func (g *generator) mapKeyParams(fields []*contract.Field) map[string]bool {
	keys := map[string]bool{}
	for _, f := range fields {
		collectMapKeyParams(f.Type, keys)
	}
	return keys
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

func (g *generator) declarations() string {
	var b strings.Builder
	for _, id := range g.errorOrder {
		decl := g.doc.Errors[id]
		doc := decl.Doc
		if strings.TrimSpace(doc) == "" {
			doc = decl.Name + " is the details shape of the " + decl.Name + " error variant (" + decl.Code + ")."
		}
		g.writeStruct(&b, g.names[id], doc, decl.Fields, nil, scope{owner: id, prefix: g.names[id]})
	}
	for _, id := range g.order {
		decl := g.doc.Types[id]
		name := g.names[id]
		switch decl.Kind {
		case contract.Struct:
			doc := decl.Doc
			if decl.Origin != "" && strings.TrimSpace(doc) == "" {
				doc = name + " is an instantiation of " + decl.Origin + "."
			}
			g.writeStruct(&b, name, doc, decl.Fields, nil, scope{owner: id, prefix: name})
		case contract.Generic:
			var fields []*contract.Field
			if decl.Body != nil {
				fields = decl.Body.Fields
			}
			params := map[string]bool{}
			for _, p := range decl.Params {
				params[p] = true
			}
			g.writeStruct(&b, name, decl.Doc, fields, decl.Params, scope{owner: id, params: params, prefix: name})
		case contract.Enum:
			g.writeEnum(&b, name, decl)
		case contract.Primitive:
			writeDoc(&b, "", decl.Doc)
			b.WriteString("pub type " + name + " = " + g.primitive(decl.Primitive, "", true) + ";\n\n")
		}
	}
	for _, name := range g.synthetic {
		b.WriteString(g.synthBody[name])
	}
	return b.String()
}

func (g *generator) writeEnum(b *strings.Builder, name string, decl *contract.TypeDecl) {
	g.uses["serde"] = true
	writeDoc(b, "", decl.Doc)
	isString := decl.Base == "" || decl.Base == "string"
	if isString {
		b.WriteString("#[derive(Clone, Copy, Debug, PartialEq, Eq, PartialOrd, Ord, Hash, Serialize, Deserialize)]\n")
	} else {
		b.WriteString("#[derive(Clone, Copy, Debug, PartialEq, Eq, PartialOrd, Ord, Hash)]\n")
	}
	b.WriteString("pub enum " + name + " {\n")
	variants := make([]string, len(decl.Values))
	used := map[string]bool{}
	for i, v := range decl.Values {
		variant := typeName(v.Name)
		if trimmed := strings.TrimPrefix(variant, name); trimmed != variant && trimmed != "" && !strings.HasPrefix(trimmed, "_") {
			variant = typeName(trimmed)
		}
		for used[variant] {
			variant += "_"
		}
		used[variant] = true
		variants[i] = variant
		if isString {
			b.WriteString("    #[serde(rename = " + literal(v.Value) + ")]\n")
		}
		b.WriteString("    " + variant + ",\n")
	}
	b.WriteString("}\n\n")
	b.WriteString("impl " + name + " {\n")
	if isString {
		b.WriteString("    pub fn as_str(&self) -> &'static str {\n        match self {\n")
		for i, v := range decl.Values {
			b.WriteString("            " + name + "::" + variants[i] + " => " + literal(v.Value) + ",\n")
		}
		b.WriteString("        }\n    }\n}\n\n")
		b.WriteString("impl Validate for " + name + " {\n    fn validate(&self, _path: &mut Vec<String>, _issues: &mut Vec<Issue>) {}\n}\n\n")
		g.uses["Validate"] = true
		g.uses["Issue"] = true
		return
	}
	b.WriteString("    pub fn value(&self) -> i64 {\n        match self {\n")
	for i, v := range decl.Values {
		b.WriteString("            " + name + "::" + variants[i] + " => " + literal(v.Value) + ",\n")
	}
	b.WriteString("        }\n    }\n}\n\n")
	b.WriteString("impl Serialize for " + name + " {\n")
	b.WriteString("    fn serialize<S: serde::Serializer>(&self, serializer: S) -> Result<S::Ok, S::Error> {\n")
	b.WriteString("        serializer.serialize_i64(self.value())\n    }\n}\n\n")
	b.WriteString("impl<'de> Deserialize<'de> for " + name + " {\n")
	b.WriteString("    fn deserialize<D: serde::Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {\n")
	b.WriteString("        match i64::deserialize(deserializer)? {\n")
	for i, v := range decl.Values {
		b.WriteString("            " + literal(v.Value) + " => Ok(" + name + "::" + variants[i] + "),\n")
	}
	b.WriteString("            other => Err(serde::de::Error::custom(format!(\"unknown " + name + " value {other}\"))),\n")
	b.WriteString("        }\n    }\n}\n\n")
	b.WriteString("impl Validate for " + name + " {\n    fn validate(&self, _path: &mut Vec<String>, _issues: &mut Vec<Issue>) {}\n}\n\n")
	g.uses["Validate"] = true
	g.uses["Issue"] = true
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
