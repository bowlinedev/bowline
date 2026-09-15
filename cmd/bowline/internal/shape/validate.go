package shape

import (
	"encoding/json"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/bowlinedev/bowline/contract"
)

type Issue struct {
	Path    []string `json:"path"`
	Rule    string   `json:"rule"`
	Message string   `json:"message"`
}

func Validate(doc *contract.Document, t *contract.Type, value any) []Issue {
	v := &validator{doc: doc}
	var issues []Issue
	v.check(t, value, nil, nil, &issues)
	return issues
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type validator struct {
	doc *contract.Document
}

func (v *validator) check(t *contract.Type, value any, env map[string]*contract.Type, path []string, issues *[]Issue) {
	switch t.Kind {
	case contract.Ref:
		decl, ok := v.doc.Types[t.ID]
		if !ok {
			return
		}
		switch decl.Kind {
		case contract.Struct:
			v.object(decl.Fields, value, nil, path, issues)
		case contract.Generic:
			inner := map[string]*contract.Type{}
			for i, p := range decl.Params {
				if i < len(t.Args) {
					inner[p] = Substitute(t.Args[i], env)
				}
			}
			if decl.Body != nil {
				v.object(decl.Body.Fields, value, inner, path, issues)
			}
		}
	case contract.Struct:
		v.object(t.Fields, value, env, path, issues)
	case contract.Array:
		list, ok := value.([]any)
		if !ok {
			return
		}
		for i, e := range list {
			v.check(t.Elem, e, env, append(path[:len(path):len(path)], strconv.Itoa(i)), issues)
		}
	case contract.Map:
		m, ok := value.(map[string]any)
		if !ok {
			return
		}
		for k, e := range m {
			v.check(t.Value, e, env, append(path[:len(path):len(path)], k), issues)
		}
	case contract.Param:
		if bound, ok := env[t.Name]; ok {
			v.check(bound, value, nil, path, issues)
		}
	}
}

func (v *validator) object(fields []*contract.Field, value any, env map[string]*contract.Type, path []string, issues *[]Issue) {
	obj, _ := value.(map[string]any)
	for _, f := range fields {
		fv, present := obj[f.Name]
		fp := append(path[:len(path):len(path)], f.Name)
		class := v.classOf(f.Type, env)
		if !present || fv == nil {
			for _, r := range f.Rules {
				if r.Rule == "required" {
					*issues = append(*issues, Issue{Path: fp, Rule: "required", Message: "is required"})
				}
			}
			continue
		}
		for _, r := range f.Rules {
			if msg := apply(r, class, fv); msg != "" {
				*issues = append(*issues, Issue{Path: fp, Rule: r.Rule, Message: msg})
			}
		}
		v.check(f.Type, fv, env, fp, issues)
	}
}

type class int

const (
	classOther class = iota
	classString
	classInteger
	classFloat
	classCollection
)

func (v *validator) classOf(t *contract.Type, env map[string]*contract.Type) class {
	switch t.Kind {
	case contract.Primitive:
		return primitiveClass(t.Name)
	case contract.Ref:
		if decl, ok := v.doc.Types[t.ID]; ok {
			switch decl.Kind {
			case contract.Primitive:
				return primitiveClass(decl.Primitive)
			case contract.Enum:
				if decl.Base == "string" || decl.Base == "" {
					return classString
				}
				return classInteger
			}
		}
	case contract.Array, contract.Map:
		return classCollection
	case contract.Param:
		if bound, ok := env[t.Name]; ok {
			return v.classOf(bound, nil)
		}
	}
	return classOther
}

func primitiveClass(name string) class {
	switch name {
	case "string":
		return classString
	case "int8", "int16", "int32", "int64", "uint8", "uint16", "uint32", "uint64", "duration":
		return classInteger
	case "float32", "float64":
		return classFloat
	}
	return classOther
}

func apply(r contract.Rule, c class, value any) string {
	switch r.Rule {
	case "required":
		switch x := value.(type) {
		case string:
			if x == "" {
				return "is required"
			}
		case json.Number:
			if f, _ := x.Float64(); f == 0 {
				return "is required"
			}
		case bool:
			if !x {
				return "is required"
			}
		case []any:
			if len(x) == 0 {
				return "is required"
			}
		case map[string]any:
			if len(x) == 0 && c == classCollection {
				return "is required"
			}
		}
	case "min", "max", "len":
		return compareSize(r, c, value)
	case "oneof":
		s := ""
		switch x := value.(type) {
		case string:
			s = x
		case json.Number:
			s = x.String()
		default:
			return ""
		}
		for _, option := range strings.Fields(r.Param) {
			if option == s {
				return ""
			}
		}
		return "must be one of " + r.Param
	case "email":
		s, _ := value.(string)
		addr, err := mail.ParseAddress(s)
		if err != nil || addr.Address != s {
			return "must be a valid email address"
		}
	case "url":
		s, _ := value.(string)
		u, err := url.Parse(s)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return "must be a valid URL"
		}
	case "uuid":
		s, _ := value.(string)
		if !uuidPattern.MatchString(s) {
			return "must be a valid UUID"
		}
	}
	return ""
}

func compareSize(r contract.Rule, c class, value any) string {
	bound, err := strconv.ParseFloat(r.Param, 64)
	if err != nil {
		return ""
	}
	var size float64
	unit := ""
	switch x := value.(type) {
	case string:
		size = float64(utf8.RuneCountInString(x))
		unit = " characters"
	case []any:
		size = float64(len(x))
		unit = " items"
	case map[string]any:
		if c != classCollection {
			return ""
		}
		size = float64(len(x))
		unit = " items"
	case json.Number:
		size, _ = x.Float64()
	default:
		return ""
	}
	switch r.Rule {
	case "min":
		if size < bound {
			return "must be at least " + r.Param + unit
		}
	case "max":
		if size > bound {
			return "must be at most " + r.Param + unit
		}
	case "len":
		if size != bound {
			return "must be exactly " + r.Param + unit
		}
	}
	return ""
}

func Substitute(t *contract.Type, env map[string]*contract.Type) *contract.Type {
	if t == nil || len(env) == 0 {
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
			copied.Args[i] = Substitute(a, env)
		}
	}
	copied.Elem = Substitute(t.Elem, env)
	copied.Value = Substitute(t.Value, env)
	copied.Key = Substitute(t.Key, env)
	return &copied
}
