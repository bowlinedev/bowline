package contract

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
)

const schemaID = "https://bowline.dev/spec/contract/1.1/contract.schema.json"

var kindValues = []string{"primitive", "ref", "array", "map", "struct", "enum", "generic", "param"}

type schemaNode = map[string]any

func Schema() ([]byte, error) {
	b := &schemaBuilder{defs: map[string]schemaNode{}}
	root := b.object(reflect.TypeOf(Document{}))
	root["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	root["$id"] = schemaID
	root["title"] = "Bowline contract document"
	root["$defs"] = b.defs
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(root); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type schemaBuilder struct {
	defs map[string]schemaNode
}

func (b *schemaBuilder) ref(t reflect.Type) schemaNode {
	name := t.Name()
	if _, done := b.defs[name]; !done {
		b.defs[name] = nil
		b.defs[name] = b.object(t)
	}
	return schemaNode{"$ref": "#/$defs/" + name}
}

func (b *schemaBuilder) object(t reflect.Type) schemaNode {
	props := map[string]any{}
	var required []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name, opts, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			continue
		}
		props[name] = b.node(f.Type, name)
		if !strings.Contains(opts, "omitempty") {
			required = append(required, name)
		}
	}
	node := schemaNode{
		"type":                 "object",
		"properties":           props,
		"additionalProperties": true,
	}
	if len(required) > 0 {
		node["required"] = required
	}
	return node
}

func (b *schemaBuilder) node(t reflect.Type, jsonName string) schemaNode {
	if t == reflect.TypeOf(Kind("")) {
		return schemaNode{"type": "string", "enum": kindValues}
	}
	switch t.Kind() {
	case reflect.Pointer:
		return b.node(t.Elem(), jsonName)
	case reflect.Struct:
		return b.ref(t)
	case reflect.Slice:
		if t == reflect.TypeOf(json.RawMessage{}) {
			return schemaNode{}
		}
		return schemaNode{"type": "array", "items": b.node(t.Elem(), jsonName)}
	case reflect.Map:
		return schemaNode{"type": "object", "additionalProperties": b.node(t.Elem(), jsonName)}
	case reflect.String:
		return schemaNode{"type": "string"}
	case reflect.Bool:
		return schemaNode{"type": "boolean"}
	case reflect.Int, reflect.Int64, reflect.Int32:
		return schemaNode{"type": "integer"}
	case reflect.Interface:
		if jsonName == "value" {
			return schemaNode{"type": []string{"string", "integer"}}
		}
		return schemaNode{}
	case reflect.Uint8:
		return schemaNode{"type": "integer"}
	default:
		panic("contract: unsupported schema field type " + t.String())
	}
}
