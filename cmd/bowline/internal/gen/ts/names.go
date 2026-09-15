package ts

import (
	"strings"

	"github.com/bowlinedev/bowline/cmd/bowline/internal/gen/naming"
	"github.com/bowlinedev/bowline/contract"
)

func AssignNames(doc *contract.Document) map[string]string {
	return assignNames(doc)
}

func assignNames(doc *contract.Document) map[string]string {
	return naming.Assign(doc, nil)
}

func identifier(s string) string {
	return naming.Identifier(s)
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
