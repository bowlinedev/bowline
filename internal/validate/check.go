package validate

import (
	"fmt"
	"net/mail"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

type Issue struct {
	Path    []string
	Rule    string
	Message string
}

type Checker struct {
	root *cnode
}

func (c *Checker) Active() bool {
	return c.root.active
}

type cnode struct {
	typ    reflect.Type
	class  Class
	rules  []compiled
	elem   *cnode
	fields []cfield
	active bool
}

type cfield struct {
	index int
	name  string
	node  *cnode
}

type compiled struct {
	rule   Rule
	number float64
	set    map[string]bool
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func Compile(t reflect.Type) (*Checker, error) {
	b := &builder{nodes: map[reflect.Type]*cnode{}}
	root, err := b.build(t, nil)
	if err != nil {
		return nil, err
	}
	b.mark()
	return &Checker{root: root}, nil
}

type builder struct {
	nodes map[reflect.Type]*cnode
	all   []*cnode
}

func (b *builder) build(t reflect.Type, rules []Rule) (*cnode, error) {
	if len(rules) == 0 {
		if n, ok := b.nodes[t]; ok {
			return n, nil
		}
	}
	n := &cnode{typ: t, class: ClassOf(t)}
	b.all = append(b.all, n)
	if len(rules) == 0 {
		b.nodes[t] = n
	}
	for _, r := range rules {
		if !Applies(r.Name, n.class) {
			return nil, fmt.Errorf("validation rule %q does not apply to type %s", r.Name, t)
		}
		c := compiled{rule: r}
		switch r.Name {
		case "min", "max", "len":
			c.number, _ = strconv.ParseFloat(r.Param, 64)
		case "oneof":
			c.set = map[string]bool{}
			for _, v := range strings.Fields(r.Param) {
				c.set[v] = true
			}
		}
		n.rules = append(n.rules, c)
	}
	inner := t
	for inner.Kind() == reflect.Pointer {
		inner = inner.Elem()
	}
	switch inner.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map:
		elem, err := b.build(inner.Elem(), nil)
		if err != nil {
			return nil, err
		}
		n.elem = elem
	case reflect.Struct:
		for i := 0; i < inner.NumField(); i++ {
			f := inner.Field(i)
			if !f.IsExported() {
				continue
			}
			name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			if name == "-" {
				continue
			}
			if name == "" {
				name = f.Name
			}
			fieldRules, err := ParseTag(f.Tag.Get("validate"))
			if err != nil {
				return nil, fmt.Errorf("%s.%s: %w", inner, f.Name, err)
			}
			child, err := b.build(f.Type, fieldRules)
			if err != nil {
				return nil, fmt.Errorf("%s.%s: %w", inner, f.Name, err)
			}
			n.fields = append(n.fields, cfield{index: i, name: name, node: child})
		}
	}
	return n, nil
}

func (b *builder) mark() {
	for _, n := range b.all {
		n.active = len(n.rules) > 0
	}
	for changed := true; changed; {
		changed = false
		for _, n := range b.all {
			if n.active {
				continue
			}
			if n.elem != nil && n.elem.active {
				n.active = true
				changed = true
				continue
			}
			for _, f := range n.fields {
				if f.node.active {
					n.active = true
					changed = true
					break
				}
			}
		}
	}
}

func (c *Checker) Check(v any) []Issue {
	if !c.root.active {
		return nil
	}
	var issues []Issue
	check(c.root, reflect.ValueOf(v), nil, &issues)
	return issues
}

func check(n *cnode, v reflect.Value, path []string, issues *[]Issue) {
	if !n.active {
		return
	}
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			for _, r := range n.rules {
				if r.rule.Name == "required" {
					add(issues, path, "required", "is required")
				}
			}
			return
		}
		v = v.Elem()
	}
	for _, r := range n.rules {
		if msg := apply(r, n.class, v); msg != "" {
			add(issues, path, r.rule.Name, msg)
		}
	}
	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		if n.elem != nil {
			for i := 0; i < v.Len(); i++ {
				check(n.elem, v.Index(i), append(path[:len(path):len(path)], strconv.Itoa(i)), issues)
			}
		}
	case reflect.Map:
		if n.elem != nil {
			iter := v.MapRange()
			for iter.Next() {
				check(n.elem, iter.Value(), append(path[:len(path):len(path)], fmt.Sprint(iter.Key().Interface())), issues)
			}
		}
	case reflect.Struct:
		for _, f := range n.fields {
			check(f.node, v.Field(f.index), append(path[:len(path):len(path)], f.name), issues)
		}
	}
}

func add(issues *[]Issue, path []string, rule, message string) {
	copied := make([]string, len(path))
	copy(copied, path)
	*issues = append(*issues, Issue{Path: copied, Rule: rule, Message: message})
}

func apply(r compiled, class Class, v reflect.Value) string {
	switch r.rule.Name {
	case "required":
		if class == Collection {
			if v.Len() == 0 {
				return "is required"
			}
			return ""
		}
		if v.IsZero() {
			return "is required"
		}
	case "min", "max", "len":
		return compareSize(r, class, v)
	case "oneof":
		var s string
		if class == Integer {
			if v.CanInt() {
				s = strconv.FormatInt(v.Int(), 10)
			} else {
				s = strconv.FormatUint(v.Uint(), 10)
			}
		} else {
			s = v.String()
		}
		if !r.set[s] {
			return "must be one of " + r.rule.Param
		}
	case "email":
		addr, err := mail.ParseAddress(v.String())
		if err != nil || addr.Address != v.String() {
			return "must be a valid email address"
		}
	case "url":
		u, err := url.Parse(v.String())
		if err != nil || u.Scheme == "" || u.Host == "" {
			return "must be a valid URL"
		}
	case "uuid":
		if !uuidPattern.MatchString(v.String()) {
			return "must be a valid UUID"
		}
	}
	return ""
}

func compareSize(r compiled, class Class, v reflect.Value) string {
	var size float64
	var unit string
	switch class {
	case String:
		size = float64(utf8.RuneCountInString(v.String()))
		unit = " characters"
	case Collection:
		size = float64(v.Len())
		unit = " items"
	case Integer:
		if v.CanInt() {
			size = float64(v.Int())
		} else {
			size = float64(v.Uint())
		}
	case Float:
		size = v.Float()
	}
	switch r.rule.Name {
	case "min":
		if size < r.number {
			return "must be at least " + r.rule.Param + unit
		}
	case "max":
		if size > r.number {
			return "must be at most " + r.rule.Param + unit
		}
	case "len":
		if size != r.number {
			return "must be exactly " + r.rule.Param + unit
		}
	}
	return ""
}
