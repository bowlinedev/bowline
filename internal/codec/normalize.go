package codec

import (
	"encoding"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

const maxSafeInteger = 1<<53 - 1

type RangeError struct {
	Path  string
	Value string
}

func (e *RangeError) Error() string {
	return fmt.Sprintf("%s: integer %s is outside the JSON safe range; tag the field json:\",string\" to carry it as a bigint", e.Path, e.Value)
}

type Plan struct {
	root *node
}

func (p *Plan) Active() bool {
	return p.root.active
}

type nodeKind uint8

const (
	leaf nodeKind = iota
	checkInt
	checkUint
	pointer
	slice
	array
	mapping
	structure
)

type node struct {
	kind   nodeKind
	typ    reflect.Type
	elem   *node
	fields []fieldPlan
	active bool
}

type fieldPlan struct {
	index int
	name  string
	node  *node
}

var (
	marshalerType     = reflect.TypeFor[json.Marshaler]()
	textMarshalerType = reflect.TypeFor[encoding.TextMarshaler]()
	plans             sync.Map
)

func Compile(t reflect.Type) *Plan {
	if cached, ok := plans.Load(t); ok {
		return cached.(*Plan)
	}
	c := &compiler{nodes: map[reflect.Type]*node{}}
	root := c.build(t)
	c.mark()
	actual, _ := plans.LoadOrStore(t, &Plan{root: root})
	return actual.(*Plan)
}

type compiler struct {
	nodes map[reflect.Type]*node
}

func isMarshaler(t reflect.Type) bool {
	return t.Implements(marshalerType) || reflect.PointerTo(t).Implements(marshalerType) ||
		t.Implements(textMarshalerType) || reflect.PointerTo(t).Implements(textMarshalerType)
}

func (c *compiler) build(t reflect.Type) *node {
	if n, ok := c.nodes[t]; ok {
		return n
	}
	n := &node{typ: t}
	c.nodes[t] = n
	if isMarshaler(t) {
		return n
	}
	switch t.Kind() {
	case reflect.Int, reflect.Int64:
		n.kind = checkInt
	case reflect.Uint, reflect.Uint64, reflect.Uintptr:
		n.kind = checkUint
	case reflect.Pointer:
		n.kind = pointer
		n.elem = c.build(t.Elem())
	case reflect.Slice:
		n.kind = slice
		n.elem = c.build(t.Elem())
	case reflect.Array:
		n.kind = array
		n.elem = c.build(t.Elem())
	case reflect.Map:
		n.kind = mapping
		n.elem = c.build(t.Elem())
	case reflect.Struct:
		n.kind = structure
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			name, opts, _ := strings.Cut(f.Tag.Get("json"), ",")
			if name == "-" {
				continue
			}
			if hasOption(opts, "omitempty") || hasOption(opts, "omitzero") {
				continue
			}
			if hasOption(opts, "string") && isInteger(f.Type) {
				continue
			}
			if name == "" {
				name = f.Name
			}
			n.fields = append(n.fields, fieldPlan{index: i, name: name, node: c.build(f.Type)})
		}
	}
	return n
}

func (c *compiler) mark() {
	for _, n := range c.nodes {
		switch n.kind {
		case checkInt, checkUint, slice, mapping:
			n.active = true
		}
	}
	for changed := true; changed; {
		changed = false
		for _, n := range c.nodes {
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

func hasOption(opts, name string) bool {
	for _, o := range strings.Split(opts, ",") {
		if o == name {
			return true
		}
	}
	return false
}

func isInteger(t reflect.Type) bool {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return true
	}
	return false
}

func (p *Plan) Normalize(v any) (any, error) {
	if !p.root.active || v == nil {
		return v, nil
	}
	out, changed, err := normalize(p.root, reflect.ValueOf(v), "")
	if err != nil {
		return nil, err
	}
	if !changed {
		return v, nil
	}
	return out.Interface(), nil
}

func normalize(n *node, v reflect.Value, path string) (reflect.Value, bool, error) {
	if !n.active {
		return v, false, nil
	}
	switch n.kind {
	case checkInt:
		i := v.Int()
		if i > maxSafeInteger || i < -maxSafeInteger {
			return v, false, &RangeError{Path: path, Value: strconv.FormatInt(i, 10)}
		}
	case checkUint:
		u := v.Uint()
		if u > maxSafeInteger {
			return v, false, &RangeError{Path: path, Value: strconv.FormatUint(u, 10)}
		}
	case pointer:
		if v.IsNil() {
			return v, false, nil
		}
		elem, changed, err := normalize(n.elem, v.Elem(), path)
		if err != nil || !changed {
			return v, false, err
		}
		fresh := reflect.New(n.typ.Elem())
		fresh.Elem().Set(elem)
		return fresh, true, nil
	case slice:
		if v.IsNil() {
			return reflect.MakeSlice(n.typ, 0, 0), true, nil
		}
		return normalizeElements(n, v, path, func(length int) reflect.Value {
			out := reflect.MakeSlice(n.typ, length, length)
			reflect.Copy(out, v)
			return out
		})
	case array:
		return normalizeElements(n, v, path, func(int) reflect.Value {
			out := reflect.New(n.typ).Elem()
			out.Set(v)
			return out
		})
	case mapping:
		if v.IsNil() {
			return reflect.MakeMap(n.typ), true, nil
		}
		if !n.elem.active {
			return v, false, nil
		}
		var out reflect.Value
		changed := false
		iter := v.MapRange()
		for iter.Next() {
			key := iter.Key()
			elem, ch, err := normalize(n.elem, iter.Value(), path+"["+fmt.Sprint(key.Interface())+"]")
			if err != nil {
				return v, false, err
			}
			if ch && !changed {
				out = reflect.MakeMapWithSize(n.typ, v.Len())
				copyMap(out, v)
				changed = true
			}
			if changed {
				out.SetMapIndex(key, elem)
			}
		}
		if !changed {
			return v, false, nil
		}
		return out, true, nil
	case structure:
		var out reflect.Value
		changed := false
		for _, f := range n.fields {
			elem, ch, err := normalize(f.node, v.Field(f.index), joinPath(path, f.name))
			if err != nil {
				return v, false, err
			}
			if !ch {
				continue
			}
			if !changed {
				out = reflect.New(n.typ).Elem()
				out.Set(v)
				changed = true
			}
			out.Field(f.index).Set(elem)
		}
		if !changed {
			return v, false, nil
		}
		return out, true, nil
	}
	return v, false, nil
}

func normalizeElements(n *node, v reflect.Value, path string, clone func(length int) reflect.Value) (reflect.Value, bool, error) {
	if !n.elem.active {
		return v, false, nil
	}
	var out reflect.Value
	changed := false
	for i := 0; i < v.Len(); i++ {
		elem, ch, err := normalize(n.elem, v.Index(i), path+"["+strconv.Itoa(i)+"]")
		if err != nil {
			return v, false, err
		}
		if !ch {
			continue
		}
		if !changed {
			out = clone(v.Len())
			changed = true
		}
		out.Index(i).Set(elem)
	}
	if !changed {
		return v, false, nil
	}
	return out, true, nil
}

func copyMap(dst, src reflect.Value) {
	iter := src.MapRange()
	for iter.Next() {
		dst.SetMapIndex(iter.Key(), iter.Value())
	}
}

func joinPath(base, name string) string {
	if base == "" {
		return name
	}
	return base + "." + name
}
