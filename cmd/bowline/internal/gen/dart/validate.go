package dart

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bowlinedev/bowline/contract"
)

type class int

const (
	classOther class = iota
	classString
	classNumber
	classBigInt
	classCollection
	classBool
)

func (g *generator) classOf(t *contract.Type) class {
	u := g.underlying(t)
	switch u.Kind {
	case contract.Primitive:
		switch u.Name {
		case "string":
			return classString
		case "bool":
			return classBool
		case "int8", "int16", "int32", "uint8", "uint16", "uint32", "float32", "float64", "duration":
			return classNumber
		case "int64", "uint64":
			if u.Encoding == "string" {
				return classBigInt
			}
			return classNumber
		}
	case contract.Ref:
		if decl := g.doc.Types[t.ID]; decl != nil && decl.Kind == contract.Enum {
			if decl.Base == "" || decl.Base == "string" {
				return classString
			}
			return classNumber
		}
	case contract.Array, contract.Map:
		return classCollection
	}
	return classOther
}

func pathLiteral(segments []string) string {
	return "[" + strings.Join(segments, ", ") + "]"
}

func issue(path []string, rule, message string) string {
	return "Issue(" + pathLiteral(path) + ", " + quote(rule) + ", " + quote(message) + ")"
}

func (g *generator) validations(fields []*contract.Field, members map[string]string) []string {
	var out []string
	for _, f := range fields {
		m := members[f.Name]
		path := []string{quote(f.Name)}
		nullable := f.Optional || f.Nullable || f.Type.Nullable
		var checks []string
		for _, r := range f.Rules {
			if c := g.rule(r, f.Type, m, path); c != "" {
				checks = append(checks, c)
			}
		}
		if hasRule(f.Rules, "required") && nullable {
			out = append(out, "if ("+m+" == null) "+issue(path, "required", "is required"))
		}
		nested := g.nested(f.Type, m, path, 0)
		if nullable {
			for _, c := range checks {
				out = append(out, "if ("+m+" != null) "+c)
			}
			for _, c := range nested {
				out = append(out, "if ("+m+" != null) "+c)
			}
			continue
		}
		out = append(out, checks...)
		out = append(out, nested...)
	}
	return out
}

func hasRule(rules []contract.Rule, name string) bool {
	for _, r := range rules {
		if r.Rule == name {
			return true
		}
	}
	return false
}

func (g *generator) rule(r contract.Rule, t *contract.Type, expr string, path []string) string {
	c := g.classOf(t)
	isEnum := t.Kind == contract.Ref && g.doc.Types[t.ID] != nil && g.doc.Types[t.ID].Kind == contract.Enum
	value := expr
	if isEnum {
		value = expr + ".value"
	}
	switch r.Rule {
	case "required":
		switch c {
		case classString:
			return "if (" + value + ".isEmpty) " + issue(path, "required", "is required")
		case classNumber:
			return "if (" + value + " == 0) " + issue(path, "required", "is required")
		case classBigInt:
			return "if (" + value + " == BigInt.zero) " + issue(path, "required", "is required")
		case classCollection:
			return "if (" + value + ".isEmpty) " + issue(path, "required", "is required")
		case classBool:
			return "if (!" + value + ") " + issue(path, "required", "is required")
		}
	case "min", "max", "len":
		return g.sizeRule(r, c, value, path)
	case "oneof":
		options := strings.Fields(r.Param)
		parts := make([]string, len(options))
		for i, o := range options {
			if c == classNumber {
				parts[i] = o
			} else {
				parts[i] = quote(o)
			}
		}
		return "if (!const {" + strings.Join(parts, ", ") + "}.contains(" + value + ")) " + issue(path, "oneof", "must be one of "+r.Param)
	case "email":
		return "if (!isEmail(" + value + ")) " + issue(path, "email", "must be a valid email address")
	case "url":
		return "if (!isUrl(" + value + ")) " + issue(path, "url", "must be a valid URL")
	case "uuid":
		return "if (!isUuid(" + value + ")) " + issue(path, "uuid", "must be a valid UUID")
	}
	return ""
}

func (g *generator) sizeRule(r contract.Rule, c class, expr string, path []string) string {
	bound := r.Param
	if _, err := strconv.ParseFloat(bound, 64); err != nil {
		return ""
	}
	var size, unit string
	switch c {
	case classString:
		size = expr + ".runes.length"
		unit = " characters"
	case classCollection:
		size = expr + ".length"
		unit = " items"
	case classNumber:
		size = expr
	case classBigInt:
		size = expr
		bound = "BigInt.parse(" + quote(r.Param) + ")"
	default:
		return ""
	}
	switch r.Rule {
	case "min":
		return "if (" + size + " < " + bound + ") " + issue(path, "min", "must be at least "+r.Param+unit)
	case "max":
		return "if (" + size + " > " + bound + ") " + issue(path, "max", "must be at most "+r.Param+unit)
	case "len":
		return "if (" + size + " != " + bound + ") " + issue(path, "len", "must be exactly "+r.Param+unit)
	}
	return ""
}

func (g *generator) nested(t *contract.Type, expr string, path []string, depth int) []string {
	switch t.Kind {
	case contract.Ref:
		decl := g.doc.Types[t.ID]
		if decl == nil {
			if _, ok := g.doc.Errors[t.ID]; ok {
				return []string{g.guard(t, expr, "...prefixed("+pathLiteral(path)+", "+expr+".validate())")}
			}
			return nil
		}
		switch decl.Kind {
		case contract.Struct:
			return []string{g.guard(t, expr, "...prefixed("+pathLiteral(path)+", "+expr+".validate())")}
		case contract.Generic:
			args := make([]string, len(t.Args))
			for i, a := range t.Args {
				v := fmt.Sprintf("v%d", depth)
				inner := g.nested(a, v, nil, depth+1)
				if len(inner) == 0 {
					args[i] = "(" + v + ") => const []"
				} else {
					args[i] = "(" + v + ") => [" + strings.Join(inner, ", ") + "]"
				}
			}
			return []string{g.guard(t, expr, "...prefixed("+pathLiteral(path)+", "+expr+".validate("+strings.Join(args, ", ")+"))")}
		}
		return nil
	case contract.Struct:
		if len(t.Fields) == 0 {
			return nil
		}
		return []string{g.guard(t, expr, "...prefixed("+pathLiteral(path)+", "+expr+".validate())")}
	case contract.Array:
		var out []string
		if t.Length > 0 {
			out = append(out, "if ("+expr+".length != "+strconv.Itoa(t.Length)+") "+issue(path, "len", "must be exactly "+strconv.Itoa(t.Length)+" items"))
		}
		i := fmt.Sprintf("i%d", depth)
		e := fmt.Sprintf("e%d", depth)
		inner := g.nested(t.Elem, e, append(append([]string{}, path...), "'$"+i+"'"), depth+1)
		if len(inner) > 0 {
			out = append(out, "for (final ("+i+", "+e+") in "+expr+".indexed) ..."+"["+strings.Join(inner, ", ")+"]")
		}
		return out
	case contract.Map:
		k := fmt.Sprintf("k%d", depth)
		v := fmt.Sprintf("v%d", depth)
		inner := g.nested(t.Value, v, append(append([]string{}, path...), k), depth+1)
		if len(inner) == 0 {
			return nil
		}
		return []string{"for (final MapEntry(key: " + k + ", value: " + v + ") in " + expr + ".entries) ...[" + strings.Join(inner, ", ") + "]"}
	case contract.Param:
		return []string{g.guard(t, expr, "...prefixed("+pathLiteral(path)+", validate"+t.Name+"("+expr+"))")}
	}
	return nil
}

func (g *generator) guard(t *contract.Type, expr, check string) string {
	if t.Nullable {
		return "if (" + expr + " != null) " + check
	}
	return check
}
