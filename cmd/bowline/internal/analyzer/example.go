package analyzer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/bowlinedev/bowline/contract"
)

func (c *collector) example(tag string, node *contract.Type) (any, error) {
	if node.Kind == contract.Ref {
		if decl, ok := c.doc.Types[node.ID]; ok {
			switch decl.Kind {
			case contract.Enum:
				return enumExample(tag, decl)
			case contract.Primitive:
				return primitiveExample(tag, decl.Primitive, "")
			}
		}
	}
	switch node.Kind {
	case contract.Primitive:
		return primitiveExample(tag, node.Name, node.Encoding)
	case contract.Array:
		return jsonExample(tag, '[', "an array")
	case contract.Map, contract.Struct, contract.Ref, contract.Generic:
		return jsonExample(tag, '{', "an object")
	default:
		return jsonExample(tag, 0, "")
	}
}

func primitiveExample(tag, name, encoding string) (any, error) {
	switch name {
	case "string":
		return tag, nil
	case "bool":
		v, err := strconv.ParseBool(tag)
		if err != nil {
			return nil, fmt.Errorf("example %q is not a boolean", tag)
		}
		return v, nil
	case "int8", "int16", "int32", "int64":
		bits, _ := strconv.Atoi(strings.TrimPrefix(name, "int"))
		v, err := strconv.ParseInt(tag, 10, bits)
		if err != nil {
			return nil, fmt.Errorf("example %q is not a valid %s", tag, name)
		}
		if encoding == "string" {
			return tag, nil
		}
		return v, nil
	case "uint8", "uint16", "uint32", "uint64":
		bits, _ := strconv.Atoi(strings.TrimPrefix(name, "uint"))
		v, err := strconv.ParseUint(tag, 10, bits)
		if err != nil {
			return nil, fmt.Errorf("example %q is not a valid %s", tag, name)
		}
		if encoding == "string" {
			return tag, nil
		}
		return v, nil
	case "float32", "float64":
		v, err := strconv.ParseFloat(tag, 64)
		if err != nil || math.IsInf(v, 0) || math.IsNaN(v) {
			return nil, fmt.Errorf("example %q is not a finite number", tag)
		}
		return json.Number(tag), nil
	case "timestamp":
		if _, err := time.Parse(time.RFC3339Nano, tag); err != nil {
			return nil, fmt.Errorf("example %q is not an RFC 3339 timestamp", tag)
		}
		return tag, nil
	case "duration":
		v, err := strconv.ParseInt(tag, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("example %q is not a duration in nanoseconds", tag)
		}
		return v, nil
	case "bytes":
		return tag, nil
	default:
		return jsonExample(tag, 0, "")
	}
}

func enumExample(tag string, decl *contract.TypeDecl) (any, error) {
	for _, v := range decl.Values {
		switch value := v.Value.(type) {
		case string:
			if value == tag {
				return value, nil
			}
		default:
			if fmt.Sprint(value) == tag {
				return value, nil
			}
		}
	}
	names := make([]string, 0, len(decl.Values))
	for _, v := range decl.Values {
		names = append(names, fmt.Sprint(v.Value))
	}
	return nil, fmt.Errorf("example %q is not a value of %s (%s)", tag, decl.Name, strings.Join(names, ", "))
}

func jsonExample(tag string, first byte, want string) (any, error) {
	trimmed := bytes.TrimSpace([]byte(tag))
	if first != 0 && (len(trimmed) == 0 || trimmed[0] != first) {
		return nil, fmt.Errorf("example %q must be %s in JSON", tag, want)
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, fmt.Errorf("example %q is not valid JSON", tag)
	}
	if dec.More() {
		return nil, fmt.Errorf("example %q is not a single JSON value", tag)
	}
	return value, nil
}
