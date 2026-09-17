package rust

import (
	"strconv"
	"strings"

	"github.com/bowlinedev/bowline/contract"
)

type class int

const (
	classOther class = iota
	classString
	classStringEnum
	classInt
	classIntEnum
	classFloat
	classBool
	classCollection
)

func (g *generator) primitiveName(t *contract.Type) string {
	switch t.Kind {
	case contract.Primitive:
		return t.Name
	case contract.Ref:
		if decl, ok := g.doc.Types[t.ID]; ok && decl.Kind == contract.Primitive {
			return decl.Primitive
		}
	}
	return ""
}

func (g *generator) classOf(t *contract.Type) class {
	switch t.Kind {
	case contract.Primitive:
		return primitiveClass(t.Name)
	case contract.Ref:
		decl, ok := g.doc.Types[t.ID]
		if !ok {
			return classOther
		}
		switch decl.Kind {
		case contract.Primitive:
			return primitiveClass(decl.Primitive)
		case contract.Enum:
			if decl.Base == "" || decl.Base == "string" {
				return classStringEnum
			}
			return classIntEnum
		}
	case contract.Array, contract.Map:
		return classCollection
	}
	return classOther
}

func primitiveClass(name string) class {
	switch name {
	case "string":
		return classString
	case "int8", "int16", "int32", "int64", "uint8", "uint16", "uint32", "uint64":
		return classInt
	case "float32", "float64":
		return classFloat
	case "bool":
		return classBool
	case "bytes":
		return classCollection
	}
	return classOther
}

func (g *generator) validatable(t *contract.Type) bool {
	if t == nil {
		return false
	}
	switch t.Kind {
	case contract.Ref:
		decl, ok := g.doc.Types[t.ID]
		return ok && (decl.Kind == contract.Struct || decl.Kind == contract.Generic)
	case contract.Struct:
		return len(t.Fields) > 0
	case contract.Array:
		return g.validatable(t.Elem)
	case contract.Map:
		return g.validatable(t.Value)
	case contract.Param:
		return true
	}
	return false
}

func (g *generator) writeValidate(b *strings.Builder, name string, fields []*contract.Field, infos []fieldInfo, params []string) {
	g.uses["Validate"] = true
	g.uses["Issue"] = true
	var body strings.Builder
	for i, f := range fields {
		g.fieldChecks(&body, f, infos[i])
	}
	impl := "impl"
	if len(params) > 0 {
		bounds := make([]string, len(params))
		keys := g.mapKeyParams(fields)
		for i, p := range params {
			if keys[p] {
				bounds[i] = p + ": Ord + ToString + Validate"
			} else {
				bounds[i] = p + ": Validate"
			}
		}
		impl = "impl<" + strings.Join(bounds, ", ") + ">"
	}
	b.WriteString(impl)
	b.WriteString(" Validate for ")
	b.WriteString(name)
	b.WriteString(generics(params, nil))
	b.WriteString(" {\n")
	if body.Len() == 0 {
		b.WriteString("    fn validate(&self, _path: &mut Vec<String>, _issues: &mut Vec<Issue>) {}\n}\n\n")
		return
	}
	g.uses["rules"] = true
	b.WriteString("    fn validate(&self, path: &mut Vec<String>, issues: &mut Vec<Issue>) {\n")
	b.WriteString(body.String())
	b.WriteString("    }\n}\n\n")
}

func (g *generator) fieldChecks(b *strings.Builder, f *contract.Field, info fieldInfo) {
	c := g.classOf(f.Type)
	json := strconv.Quote(f.Name)
	access := "self." + info.name
	var checks []string
	for _, r := range f.Rules {
		if r.Rule == "required" && info.option {
			b.WriteString("        rules::required_some(&")
			b.WriteString(access)
			b.WriteString(", path, ")
			b.WriteString(json)
			b.WriteString(", issues);\n")
			continue
		}
		expr, val := "&"+access, access
		if info.option {
			expr, val = "v", "*v"
			if c == classIntEnum || c == classStringEnum || c == classCollection {
				val = "v"
			}
		}
		if check := g.check(r, c, g.primitiveName(f.Type), expr, val, json); check != "" {
			checks = append(checks, check)
		}
	}
	if len(checks) > 0 {
		if info.option {
			b.WriteString("        if let Some(v) = &")
			b.WriteString(access)
			b.WriteString(" {\n")
			for _, check := range checks {
				b.WriteString("            ")
				b.WriteString(check)
				b.WriteString("\n")
			}
			b.WriteString("        }\n")
		} else {
			for _, check := range checks {
				b.WriteString("        ")
				b.WriteString(check)
				b.WriteString("\n")
			}
		}
	}
	if g.validatable(f.Type) {
		b.WriteString("        rules::nested(&")
		b.WriteString(access)
		b.WriteString(", path, ")
		b.WriteString(json)
		b.WriteString(", issues);\n")
	}
}

func cast(val, prim, target string) string {
	if prim == target {
		return val
	}
	return val + " as " + map[string]string{"int64": "i64", "float64": "f64"}[target]
}

func (g *generator) check(r contract.Rule, c class, prim, expr, val, json string) string {
	tail := ", path, " + json + ", issues);"
	plain := strings.TrimPrefix(expr, "&")
	enum := plain + ".as_str()"
	switch r.Rule {
	case "required":
		switch c {
		case classString:
			return "rules::required_str(" + expr + tail
		case classInt:
			return "rules::required_int(" + cast(val, prim, "int64") + tail
		case classIntEnum:
			return "rules::required_int(" + val + ".value()" + tail
		case classFloat:
			return "rules::required_float(" + cast(val, prim, "float64") + tail
		case classBool:
			return "rules::required_bool(" + val + tail
		case classCollection:
			return "rules::required_len(" + plain + ".len()" + tail
		}
	case "min", "max", "len":
		fn := map[string]string{"min": "min", "max": "max", "len": "exact"}[r.Rule]
		switch c {
		case classString, classStringEnum:
			n := integerParam(r.Param)
			s := expr
			if c == classStringEnum {
				s = enum
			}
			return "rules::" + fn + "_len(rules::chars(" + s + "), " + n + ", \" characters\"" + tail
		case classCollection:
			return "rules::" + fn + "_len(" + plain + ".len(), " + integerParam(r.Param) + ", \" items\"" + tail
		case classInt:
			return "rules::" + fn + "_num(" + val + " as f64, " + floatParam(r.Param) + ", " + strconv.Quote(r.Param) + tail
		case classIntEnum:
			return "rules::" + fn + "_num(" + val + ".value() as f64, " + floatParam(r.Param) + ", " + strconv.Quote(r.Param) + tail
		case classFloat:
			return "rules::" + fn + "_num(" + cast(val, prim, "float64") + ", " + floatParam(r.Param) + ", " + strconv.Quote(r.Param) + tail
		}
	case "oneof":
		options := strings.Fields(r.Param)
		quoted := make([]string, len(options))
		for i, o := range options {
			quoted[i] = strconv.Quote(o)
		}
		list := "&[" + strings.Join(quoted, ", ") + "]"
		switch c {
		case classString:
			return "rules::one_of(" + expr + ", " + list + tail
		case classStringEnum:
			return "rules::one_of(" + enum + ", " + list + tail
		case classInt:
			return "rules::one_of_int(" + cast(val, prim, "int64") + ", " + list + tail
		case classIntEnum:
			return "rules::one_of_int(" + val + ".value(), " + list + tail
		}
	case "email", "url", "uuid":
		if c == classString {
			return "rules::" + r.Rule + "(" + expr + tail
		}
	}
	return ""
}

func integerParam(param string) string {
	if n, err := strconv.Atoi(param); err == nil && n >= 0 {
		return strconv.Itoa(n)
	}
	return "0"
}

func floatParam(param string) string {
	f, err := strconv.ParseFloat(param, 64)
	if err != nil {
		return "0.0"
	}
	s := strconv.FormatFloat(f, 'g', -1, 64)
	if !strings.ContainsAny(s, ".e") {
		s += ".0"
	}
	return s
}
