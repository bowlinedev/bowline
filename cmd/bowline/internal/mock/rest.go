package mock

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/bowlinedev/bowline/contract"
)

type restRoute struct {
	segments []contract.PathSegment
	literals int
	proc     *contract.Procedure
}

func restRoutes(doc *contract.Document) []restRoute {
	var routes []restRoute
	for _, p := range doc.Procedures {
		if p.HTTPPath == "" {
			continue
		}
		segments, err := contract.ParsePath(p.HTTPPath)
		if err != nil {
			continue
		}
		literals := 0
		for _, seg := range segments {
			if !seg.Param {
				literals++
			}
		}
		routes = append(routes, restRoute{segments: segments, literals: literals, proc: p})
	}
	slices.SortStableFunc(routes, func(a, b restRoute) int {
		if n := len(b.segments) - len(a.segments); n != 0 {
			return n
		}
		return b.literals - a.literals
	})
	return routes
}

func splitPath(escaped string) ([]string, bool) {
	trimmed := strings.Trim(escaped, "/")
	if trimmed == "" {
		return nil, false
	}
	parts := strings.Split(trimmed, "/")
	for i, part := range parts {
		decoded, err := url.PathUnescape(part)
		if err != nil {
			return nil, false
		}
		parts[i] = decoded
	}
	return parts, true
}

func (h *handler) matchREST(escaped, method string) (*contract.Procedure, map[string]string, []string) {
	parts, ok := splitPath(escaped)
	if !ok {
		return nil, nil, nil
	}
	var allowed []string
	for _, route := range h.rest {
		if len(route.segments) > len(parts) {
			continue
		}
		tail := parts[len(parts)-len(route.segments):]
		params, matched := bindSegments(route.segments, tail)
		if !matched {
			continue
		}
		if route.proc.Method != method {
			if !slices.Contains(allowed, route.proc.Method) {
				allowed = append(allowed, route.proc.Method)
			}
			continue
		}
		return route.proc, params, nil
	}
	return nil, nil, allowed
}

func bindSegments(segments []contract.PathSegment, parts []string) (map[string]string, bool) {
	var params map[string]string
	for i, seg := range segments {
		if !seg.Param {
			if seg.Text != parts[i] {
				return nil, false
			}
			continue
		}
		if parts[i] == "" {
			return nil, false
		}
		if params == nil {
			params = make(map[string]string, len(segments))
		}
		params[seg.Text] = parts[i]
	}
	return params, true
}

func sendsBody(method string) bool {
	switch method {
	case http.MethodGet, http.MethodDelete, http.MethodHead:
		return false
	}
	return true
}

func (h *handler) mergeREST(p *contract.Procedure, raw []byte, params map[string]string, query url.Values) ([]byte, error) {
	input, err := decodeInput(raw)
	if err != nil {
		return nil, err
	}
	fields := h.inputFields(p)
	for name, value := range params {
		bound, err := h.coerce(name, fields[name], value)
		if err != nil {
			return nil, err
		}
		input[name] = bound
	}
	if !sendsBody(p.Method) {
		for name, values := range query {
			field, ok := fields[name]
			if !ok {
				continue
			}
			if field.Type != nil && field.Type.Kind == contract.Array {
				list := make([]any, len(values))
				for i, v := range values {
					bound, err := h.coerce(name, elemField(field), v)
					if err != nil {
						return nil, err
					}
					list[i] = bound
				}
				input[name] = list
				continue
			}
			bound, err := h.coerce(name, field, values[len(values)-1])
			if err != nil {
				return nil, err
			}
			input[name] = bound
		}
	}
	return json.Marshal(input)
}

func (h *handler) inputFields(p *contract.Procedure) map[string]*contract.Field {
	out := map[string]*contract.Field{}
	t := p.Input
	if t == nil {
		return out
	}
	var fields []*contract.Field
	switch t.Kind {
	case contract.Struct:
		fields = t.Fields
	case contract.Ref:
		if decl, ok := h.doc.Types[t.ID]; ok {
			fields = decl.Fields
		}
	}
	for _, f := range fields {
		out[f.Name] = f
	}
	return out
}

func elemField(field *contract.Field) *contract.Field {
	if field == nil || field.Type == nil || field.Type.Elem == nil {
		return nil
	}
	return &contract.Field{Name: field.Name, Type: field.Type.Elem}
}

func (h *handler) primitive(t *contract.Type) (string, bool) {
	if t == nil {
		return "", false
	}
	switch t.Kind {
	case contract.Primitive:
		return t.Name, t.Encoding == "string"
	case contract.Ref:
		if decl, ok := h.doc.Types[t.ID]; ok && decl.Kind == contract.Enum {
			return decl.Base, false
		}
	}
	return "", false
}

func (h *handler) coerce(name string, field *contract.Field, value string) (any, error) {
	var t *contract.Type
	if field != nil {
		t = field.Type
	}
	kind, encoded := h.primitive(t)
	switch kind {
	case "bool":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("%s: %q is not a boolean", name, value)
		}
		return b, nil
	case "int", "int8", "int16", "int32", "int64":
		if _, err := strconv.ParseInt(value, 10, 64); err != nil {
			return nil, fmt.Errorf("%s: %q is not a number", name, value)
		}
	case "uint", "uint8", "uint16", "uint32", "uint64":
		if _, err := strconv.ParseUint(value, 10, 64); err != nil {
			return nil, fmt.Errorf("%s: %q is not a number", name, value)
		}
	case "float32", "float64":
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return nil, fmt.Errorf("%s: %q is not a number", name, value)
		}
	default:
		return value, nil
	}
	if encoded {
		return value, nil
	}
	return json.Number(value), nil
}
