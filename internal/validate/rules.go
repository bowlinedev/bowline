package validate

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

type Rule struct {
	Name  string
	Param string
}

var vocabulary = map[string]bool{
	"required": false, "min": true, "max": true, "len": true, "oneof": true,
	"email": false, "url": false, "uuid": false,
}

func ParseTag(tag string) ([]Rule, error) {
	if strings.TrimSpace(tag) == "" {
		return nil, nil
	}
	var rules []Rule
	for term := range strings.SplitSeq(tag, ",") {
		term = strings.TrimSpace(term)
		if term == "" {
			continue
		}
		name, param, hasParam := strings.Cut(term, "=")
		wantsParam, ok := vocabulary[name]
		if !ok {
			return nil, fmt.Errorf("unsupported validation rule %q; supported: required, min, max, len, oneof, email, url, uuid", name)
		}
		if wantsParam != hasParam || (hasParam && param == "") {
			return nil, fmt.Errorf("validation rule %q: parameter %s", name, map[bool]string{true: "required", false: "not allowed"}[wantsParam])
		}
		if name == "min" || name == "max" || name == "len" {
			if _, err := strconv.ParseFloat(param, 64); err != nil {
				return nil, fmt.Errorf("validation rule %s=%s: parameter must be numeric", name, param)
			}
		}
		rules = append(rules, Rule{Name: name, Param: param})
	}
	return rules, nil
}

type Class int

const (
	Other Class = iota
	String
	Integer
	Float
	Collection
	Bool
	Struct
)

func ClassOf(t reflect.Type) Class {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.String:
		return String
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return Integer
	case reflect.Float32, reflect.Float64:
		return Float
	case reflect.Slice, reflect.Array, reflect.Map:
		return Collection
	case reflect.Bool:
		return Bool
	case reflect.Struct:
		return Struct
	}
	return Other
}

func Applies(rule string, c Class) bool {
	switch rule {
	case "required":
		return true
	case "min", "max", "len":
		return c == String || c == Integer || c == Float || c == Collection
	case "oneof":
		return c == String || c == Integer
	case "email", "url", "uuid":
		return c == String
	}
	return false
}
