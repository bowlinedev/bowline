package naming

import (
	"slices"
	"strings"
	"unicode"

	"github.com/bowlinedev/bowline/contract"
)

func Assign(doc *contract.Document, reserved map[string]bool) map[string]string {
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
			names[ids[0]] = escape(Identifier(name), reserved)
			continue
		}
		slices.Sort(ids)
		for _, id := range ids {
			names[id] = escape(Identifier(PackageName(id)+"_"+name), reserved)
		}
	}
	return names
}

func escape(name string, reserved map[string]bool) string {
	if reserved[name] {
		return name + "_"
	}
	return name
}

func PackageName(id string) string {
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

func Identifier(s string) string {
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

func words(s string) []string {
	var out []string
	var current strings.Builder
	flush := func() {
		if current.Len() > 0 {
			out = append(out, current.String())
			current.Reset()
		}
	}
	runes := []rune(s)
	for i, r := range runes {
		switch {
		case !unicode.IsLetter(r) && !unicode.IsDigit(r):
			flush()
		case unicode.IsUpper(r):
			prevLower := i > 0 && (unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1]))
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if prevLower || (nextLower && current.Len() > 0) {
				flush()
			}
			current.WriteRune(unicode.ToLower(r))
		default:
			current.WriteRune(unicode.ToLower(r))
		}
	}
	flush()
	return out
}

func LowerCamel(json string) string {
	parts := words(json)
	if len(parts) == 0 {
		return "_"
	}
	var b strings.Builder
	for i, p := range parts {
		if i == 0 {
			b.WriteString(p)
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		b.WriteString(p[1:])
	}
	return leadingDigit(b.String())
}

func UpperCamel(json string) string {
	parts := words(json)
	if len(parts) == 0 {
		return "X"
	}
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(strings.ToUpper(p[:1]))
		b.WriteString(p[1:])
	}
	return leadingDigit(b.String())
}

func Snake(json string) string {
	parts := words(json)
	if len(parts) == 0 {
		return "_"
	}
	return leadingDigit(strings.Join(parts, "_"))
}

func leadingDigit(s string) string {
	if s != "" && s[0] >= '0' && s[0] <= '9' {
		return "_" + s
	}
	return s
}
