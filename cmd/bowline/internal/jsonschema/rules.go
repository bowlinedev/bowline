package jsonschema

import (
	"maps"
	"strconv"
	"strings"

	"github.com/bowlinedev/bowline/contract"
)

func applyRules(n node, f *contract.Field) node {
	if len(f.Rules) == 0 {
		return n
	}
	var target node
	wrapped := false
	if anyOf, ok := n["anyOf"].([]node); ok && len(anyOf) == 2 {
		target = node{}
		maps.Copy(target, anyOf[0])
		wrapped = true
	} else {
		target = node{}
		maps.Copy(target, n)
	}
	class := classOf(target)
	for _, r := range f.Rules {
		switch r.Rule {
		case "required":
			switch class {
			case "string":
				target["minLength"] = 1
			case "array":
				target["minItems"] = 1
			}
		case "min", "max", "len":
			value, err := strconv.ParseFloat(r.Param, 64)
			if err != nil {
				continue
			}
			for _, key := range boundKeys(class, r.Rule) {
				target[key] = number(value)
			}
		case "oneof":
			values := strings.Fields(r.Param)
			list := make([]any, len(values))
			for i, v := range values {
				if class == "integer" {
					if n, err := strconv.ParseInt(v, 10, 64); err == nil {
						list[i] = n
						continue
					}
				}
				list[i] = v
			}
			target["enum"] = list
		case "email":
			target["format"] = "email"
		case "url":
			target["format"] = "uri"
		case "uuid":
			target["format"] = "uuid"
		}
	}
	if wrapped {
		return node{"anyOf": []node{target, {"type": "null"}}}
	}
	return target
}

func classOf(n node) string {
	switch typ := n["type"].(type) {
	case string:
		return typ
	case []string:
		if len(typ) > 0 {
			return typ[0]
		}
	}
	if _, ok := n["$ref"]; ok {
		return "ref"
	}
	return ""
}

func boundKeys(class, rule string) []string {
	var lower, upper string
	switch class {
	case "string":
		lower, upper = "minLength", "maxLength"
	case "array":
		lower, upper = "minItems", "maxItems"
	case "integer", "number":
		lower, upper = "minimum", "maximum"
	default:
		return nil
	}
	switch rule {
	case "min":
		return []string{lower}
	case "max":
		return []string{upper}
	}
	return []string{lower, upper}
}

func number(v float64) any {
	if v == float64(int64(v)) {
		return int64(v)
	}
	return v
}
