package shape

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/bowlinedev/bowline/contract"
)

type Mismatch struct {
	Path   []string
	Reason string
}

func Check(doc *contract.Document, t *contract.Type, value any) []Mismatch {
	c := &checker{doc: doc}
	var out []Mismatch
	c.node(t, value, nil, nil, &out)
	return out
}

type checker struct {
	doc *contract.Document
}

func (c *checker) node(t *contract.Type, value any, env map[string]*contract.Type, path []string, out *[]Mismatch) {
	if value == nil || t == nil {
		return
	}
	switch t.Kind {
	case contract.Primitive:
		c.primitive(t.Name, t.Encoding, value, path, out)
	case contract.Ref:
		decl, ok := c.doc.Types[t.ID]
		if !ok {
			*out = append(*out, Mismatch{path, "refers to unknown type " + t.ID})
			return
		}
		switch decl.Kind {
		case contract.Primitive:
			c.primitive(decl.Primitive, t.Encoding, value, path, out)
		case contract.Enum:
			for _, ev := range decl.Values {
				if fmt.Sprint(ev.Value) == fmt.Sprint(value) {
					return
				}
			}
			*out = append(*out, Mismatch{path, fmt.Sprintf("value %v is no longer a member of enum %s", value, decl.Name)})
		case contract.Struct:
			c.object(decl.Fields, value, nil, path, out)
		case contract.Generic:
			inner := map[string]*contract.Type{}
			for i, p := range decl.Params {
				if i < len(t.Args) {
					inner[p] = Substitute(t.Args[i], env)
				}
			}
			if decl.Body != nil {
				c.object(decl.Body.Fields, value, inner, path, out)
			}
		}
	case contract.Struct:
		c.object(t.Fields, value, env, path, out)
	case contract.Array:
		list, ok := value.([]any)
		if !ok {
			*out = append(*out, Mismatch{path, fmt.Sprintf("expected an array, recorded %s", describe(value))})
			return
		}
		for i, e := range list {
			c.node(t.Elem, e, env, append(path[:len(path):len(path)], strconv.Itoa(i)), out)
		}
	case contract.Map:
		m, ok := value.(map[string]any)
		if !ok {
			*out = append(*out, Mismatch{path, fmt.Sprintf("expected an object, recorded %s", describe(value))})
			return
		}
		for k, e := range m {
			c.node(t.Value, e, env, append(path[:len(path):len(path)], k), out)
		}
	case contract.Param:
		if bound, ok := env[t.Name]; ok {
			c.node(bound, value, nil, path, out)
		}
	}
}

func (c *checker) object(fields []*contract.Field, value any, env map[string]*contract.Type, path []string, out *[]Mismatch) {
	obj, ok := value.(map[string]any)
	if !ok {
		*out = append(*out, Mismatch{path, fmt.Sprintf("expected an object, recorded %s", describe(value))})
		return
	}
	byName := map[string]*contract.Field{}
	for _, f := range fields {
		byName[f.Name] = f
	}
	for name, fv := range obj {
		fp := append(path[:len(path):len(path)], name)
		f, exists := byName[name]
		if !exists {
			*out = append(*out, Mismatch{fp, "field removed"})
			continue
		}
		if fv == nil {
			if !f.Nullable && !f.Optional {
				*out = append(*out, Mismatch{fp, "recorded null but the field no longer accepts null"})
			}
			continue
		}
		c.node(f.Type, fv, env, fp, out)
	}
}

func (c *checker) primitive(name, encoding string, value any, path []string, out *[]Mismatch) {
	want := ""
	switch name {
	case "string", "timestamp", "bytes":
		want = "string"
	case "bool":
		want = "boolean"
	case "int8", "int16", "int32", "int64", "uint8", "uint16", "uint32", "uint64", "float32", "float64", "duration":
		want = "number"
		if encoding == "string" {
			want = "string"
		}
	case "raw":
		return
	}
	got := describe(value)
	if want != "" && got != want {
		*out = append(*out, Mismatch{path, fmt.Sprintf("expected %s (%s), recorded %s", want, name, got)})
	}
}

func describe(value any) string {
	switch value.(type) {
	case string:
		return "string"
	case bool:
		return "boolean"
	case json.Number, float64, int64:
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	case nil:
		return "null"
	}
	return fmt.Sprintf("%T", value)
}
