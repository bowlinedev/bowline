package goclient

import (
	"sort"
	"strings"
	"unicode"

	"github.com/bowlinedev/bowline/cmd/bowline/internal/gen/ts"
	"github.com/bowlinedev/bowline/contract"
)

var reservedNames = map[string]bool{
	"Client": true, "Option": true, "New": true, "WithHTTPClient": true, "WithHeaders": true,
	"ErrorDetails": true, "DetailsAs": true, "VariantOf": true,
}

var keywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true, "default": true, "defer": true,
	"else": true, "fallthrough": true, "for": true, "func": true, "go": true, "goto": true, "if": true,
	"import": true, "interface": true, "map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true, "var": true,
}

func assignNames(doc *contract.Document) map[string]string {
	base := ts.AssignNames(doc)
	ids := make([]string, 0, len(base))
	for id := range base {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	groups := map[string][]string{}
	for _, id := range ids {
		name := exported(base[id])
		groups[name] = append(groups[name], id)
	}
	names := map[string]string{}
	used := map[string]bool{}
	for _, id := range ids {
		name := exported(base[id])
		if len(groups[name]) > 1 {
			name = exported(packageOf(id) + "_" + declName(doc, id))
		}
		if reservedNames[name] {
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

func declName(doc *contract.Document, id string) string {
	if decl, ok := doc.Types[id]; ok {
		return decl.Name
	}
	if decl, ok := doc.Errors[id]; ok {
		return decl.Name
	}
	return id
}

func packageOf(id string) string {
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
	return pkg
}

func exported(s string) string {
	var b strings.Builder
	for i, r := range s {
		switch {
		case unicode.IsLetter(r), r == '_':
			b.WriteRune(r)
		case unicode.IsDigit(r) && i > 0:
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := b.String()
	if out == "" {
		return "X"
	}
	runes := []rune(out)
	if !unicode.IsUpper(runes[0]) {
		if unicode.IsLetter(runes[0]) {
			runes[0] = unicode.ToUpper(runes[0])
		} else {
			runes = append([]rune{'X'}, runes...)
		}
	}
	return string(runes)
}

var initialisms = map[string]string{
	"id": "ID", "url": "URL", "uri": "URI", "ip": "IP", "http": "HTTP", "json": "JSON", "html": "HTML",
	"api": "API", "uuid": "UUID", "sql": "SQL", "css": "CSS", "xml": "XML", "tls": "TLS", "ssh": "SSH",
}

func fieldName(jsonName string) string {
	parts := strings.FieldsFunc(jsonName, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	if len(parts) == 0 {
		return "Field"
	}
	var b strings.Builder
	for _, part := range parts {
		lower := strings.ToLower(part)
		if full, ok := initialisms[lower]; ok {
			b.WriteString(full)
			continue
		}
		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])
		b.WriteString(string(runes))
	}
	out := b.String()
	if unicode.IsDigit([]rune(out)[0]) {
		out = "F" + out
	}
	return out
}

func segmentsName(segments []string) string {
	var b strings.Builder
	for _, s := range segments {
		b.WriteString(exported(s))
	}
	return b.String()
}

func packageIdentifier(s string) string {
	var b strings.Builder
	for i, r := range s {
		switch {
		case unicode.IsLetter(r):
			b.WriteRune(unicode.ToLower(r))
		case unicode.IsDigit(r) && i > 0:
			b.WriteRune(r)
		case r == '_':
			b.WriteRune(r)
		}
	}
	out := b.String()
	if out == "" || keywords[out] || unicode.IsDigit([]rune(out)[0]) {
		return "apiclient"
	}
	return out
}
