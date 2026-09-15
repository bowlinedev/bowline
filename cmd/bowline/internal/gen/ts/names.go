package ts

import (
	"sort"
	"strings"

	"github.com/bowlinedev/bowline/contract"
)

func AssignNames(doc *contract.Document) map[string]string {
	return assignNames(doc)
}

func assignNames(doc *contract.Document) map[string]string {
	byName := map[string][]string{}
	for id, decl := range doc.Types {
		byName[decl.Name] = append(byName[decl.Name], id)
	}
	for id, decl := range doc.Errors {
		byName[decl.Name] = append(byName[decl.Name], id)
	}
	names := map[string]string{}
	for name, ids := range byName {
		if len(ids) == 1 {
			names[ids[0]] = identifier(name)
			continue
		}
		sort.Strings(ids)
		for _, id := range ids {
			names[id] = identifier(packageName(id) + "_" + name)
		}
	}
	return names
}

func packageName(id string) string {
	pkg := id
	if i := strings.LastIndex(id, "."); i >= 0 {
		pkg = id[:i]
	}
	if i := strings.LastIndex(pkg, "/"); i >= 0 {
		pkg = pkg[i+1:]
	}
	if pkg == "" {
		return "Pkg"
	}
	return strings.ToUpper(pkg[:1]) + pkg[1:]
}

func identifier(s string) string {
	var b strings.Builder
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_', r == '$':
			b.WriteRune(r)
		case r >= '0' && r <= '9' && i > 0:
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

var reserved = map[string]bool{
	"break": true, "case": true, "catch": true, "class": true, "const": true, "continue": true, "debugger": true,
	"default": true, "delete": true, "do": true, "else": true, "enum": true, "export": true, "extends": true,
	"false": true, "finally": true, "for": true, "function": true, "if": true, "import": true, "in": true,
	"instanceof": true, "new": true, "null": true, "return": true, "super": true, "switch": true, "this": true,
	"throw": true, "true": true, "try": true, "typeof": true, "var": true, "void": true, "while": true, "with": true,
}

func propertyKey(name string) string {
	if reserved[name] || identifier(name) != name {
		return `"` + strings.ReplaceAll(name, `"`, `\"`) + `"`
	}
	return name
}
