package elixir

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/bowlinedev/bowline/cmd/bowline/internal/gen/naming"
	"github.com/bowlinedev/bowline/contract"
)

var reservedAtoms = map[string]bool{
	"do": true, "end": true, "fn": true, "when": true, "and": true, "or": true, "not": true, "in": true,
	"true": true, "false": true, "nil": true, "after": true, "else": true, "catch": true, "rescue": true,
	"__MODULE__": true, "__struct__": true,
}

func assignNames(doc *contract.Document) map[string]string {
	base := naming.Assign(doc, nil)
	names := map[string]string{}
	used := map[string]bool{}
	ids := make([]string, 0, len(base))
	for id := range base {
		ids = append(ids, id)
	}
	sortStrings(ids)
	for _, id := range ids {
		name := naming.UpperCamel(base[id])
		if name == "Client" || name == "Types" {
			name += "Type"
		}
		for used[name] {
			name += "_"
		}
		used[name] = true
		names[id] = name
	}
	return names
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func fieldAtom(jsonName string) string {
	name := naming.Snake(jsonName)
	if reservedAtoms[name] {
		name += "_"
	}
	return name
}

func fieldAtoms(fields []*contract.Field) []string {
	used := map[string]bool{}
	out := make([]string, len(fields))
	for i, f := range fields {
		name := fieldAtom(f.Name)
		for used[name] {
			name += "_"
		}
		used[name] = true
		out[i] = name
	}
	return out
}

func enumAtom(v any) string {
	if x, ok := v.(string); ok {
		return ":" + naming.Snake(x)
	}
	return literal(v)
}

func literal(v any) string {
	switch x := v.(type) {
	case string:
		return quote(x)
	case json.Number:
		return x.String()
	case int64:
		return strconv.FormatInt(x, 10)
	case uint64:
		return strconv.FormatUint(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	}
	return fmt.Sprint(v)
}

func paramSpec(name string) string {
	return "param_" + strings.ToLower(naming.Snake(name))
}

func paramFn(prefix, name string) string {
	return prefix + "_" + strings.ToLower(naming.Snake(name))
}

func segmentsModule(root string, segments []string) string {
	parts := make([]string, len(segments))
	for i, s := range segments {
		parts[i] = naming.UpperCamel(s)
	}
	return root + "." + strings.Join(parts, ".")
}

func functionName(segment string) string {
	name := naming.Snake(segment)
	if reservedAtoms[name] {
		name += "_"
	}
	return name
}
