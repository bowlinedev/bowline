package consumers

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/bowlinedev/bowline/cmd/bowline/internal/shape"
	"github.com/bowlinedev/bowline/contract"
)

func shapeSubstitute(t *contract.Type, env map[string]*contract.Type) *contract.Type {
	return shape.Substitute(t, env)
}

type Impact struct {
	Consumer     string
	Interactions int
}

type Annotated struct {
	contract.Change
	Impacts []Impact
}

func (a Annotated) Note() string {
	if a.Category != contract.Breaking {
		return ""
	}
	if len(a.Impacts) == 0 {
		return "unused by consumers"
	}
	parts := make([]string, len(a.Impacts))
	for i, im := range a.Impacts {
		parts[i] = fmt.Sprintf("`%s` (%d interactions)", im.Consumer, im.Interactions)
	}
	return "breaks " + strings.Join(parts, ", ")
}

func Annotate(old *contract.Document, changes contract.Changes, list []Consumer) []Annotated {
	procs := map[string]*contract.Procedure{}
	for _, p := range old.Procedures {
		procs[p.Path] = p
	}
	out := make([]Annotated, 0, len(changes))
	for _, ch := range changes {
		a := Annotated{Change: ch}
		if ch.Category == contract.Breaking {
			procedure, side, path := parsePath(ch.Path)
			typeID, rel := locate(old, procs[procedure], side, path)
			for _, c := range list {
				var n int
				if typeID != "" {
					n = touchesType(old, procs, c, typeID, rel)
				} else {
					n = touches(c, procedure, side, path)
				}
				if n > 0 {
					a.Impacts = append(a.Impacts, Impact{Consumer: c.Consumer, Interactions: n})
				}
			}
			slices.SortFunc(a.Impacts, func(a, b Impact) int { return cmp.Compare(a.Consumer, b.Consumer) })
		}
		out = append(out, a)
	}
	return out
}

func locate(doc *contract.Document, p *contract.Procedure, side string, path []string) (string, []string) {
	if p == nil || len(path) == 0 {
		return "", nil
	}
	var t *contract.Type
	switch side {
	case "input":
		t = p.Input
	case "output":
		t = p.Output
	default:
		return "", nil
	}
	typeID := ""
	var rel []string
	env := map[string]*contract.Type{}
	for i := 0; i < len(path) && t != nil; i++ {
		t = resolve(doc, t, env, &typeID, &rel, &env)
		if t == nil {
			return "", nil
		}
		segment := path[i]
		switch segment {
		case "*":
			switch t.Kind {
			case contract.Array:
				t = t.Elem
			case contract.Map:
				t = t.Value
			default:
				return "", nil
			}
			rel = append(rel, "*")
		default:
			fields := fieldsOf(doc, t, env)
			var next *contract.Type
			for _, f := range fields {
				if f.Name == segment {
					next = f.Type
				}
			}
			if next == nil {
				return "", nil
			}
			rel = append(rel, segment)
			t = next
		}
	}
	return typeID, rel
}

func resolve(doc *contract.Document, t *contract.Type, env map[string]*contract.Type, typeID *string, rel *[]string, envOut *map[string]*contract.Type) *contract.Type {
	if t.Kind == contract.Param {
		if bound, ok := env[t.Name]; ok {
			return resolve(doc, bound, nil, typeID, rel, envOut)
		}
		return nil
	}
	if t.Kind != contract.Ref {
		return t
	}
	decl, ok := doc.Types[t.ID]
	if !ok {
		return nil
	}
	switch decl.Kind {
	case contract.Struct:
		*typeID = t.ID
		*rel = nil
		*envOut = nil
	case contract.Generic:
		*typeID = t.ID
		*rel = nil
		inner := map[string]*contract.Type{}
		for i, param := range decl.Params {
			if i < len(t.Args) {
				inner[param] = t.Args[i]
			}
		}
		*envOut = inner
	}
	return t
}

func fieldsOf(doc *contract.Document, t *contract.Type, env map[string]*contract.Type) []*contract.Field {
	switch t.Kind {
	case contract.Struct:
		return t.Fields
	case contract.Ref:
		decl, ok := doc.Types[t.ID]
		if !ok {
			return nil
		}
		if decl.Kind == contract.Generic && decl.Body != nil {
			return decl.Body.Fields
		}
		return decl.Fields
	}
	return nil
}

func touchesType(doc *contract.Document, procs map[string]*contract.Procedure, c Consumer, typeID string, rel []string) int {
	count := 0
	for _, in := range c.Interactions {
		p, ok := procs[in.Procedure]
		if !ok {
			continue
		}
		var raw []byte
		var t *contract.Type
		if in.Response.Status >= 400 {
			continue
		}
		raw, t = in.Response.Body, p.Output
		value, err := decode(raw)
		if err != nil {
			continue
		}
		if usesType(doc, t, value, nil, typeID, rel) {
			count++
			continue
		}
		input, err := decode(in.Input)
		if err == nil && usesType(doc, p.Input, input, nil, typeID, rel) {
			count++
		}
	}
	return count
}

func usesType(doc *contract.Document, t *contract.Type, value any, env map[string]*contract.Type, typeID string, rel []string) bool {
	if t == nil || value == nil {
		return false
	}
	switch t.Kind {
	case contract.Param:
		if bound, ok := env[t.Name]; ok {
			return usesType(doc, bound, value, nil, typeID, rel)
		}
		return false
	case contract.Array:
		list, _ := value.([]any)
		for _, e := range list {
			if usesType(doc, t.Elem, e, env, typeID, rel) {
				return true
			}
		}
		return false
	case contract.Map:
		m, _ := value.(map[string]any)
		for _, e := range m {
			if usesType(doc, t.Value, e, env, typeID, rel) {
				return true
			}
		}
		return false
	case contract.Struct:
		return walkFields(doc, t.Fields, value, env, typeID, rel)
	case contract.Ref:
		decl, ok := doc.Types[t.ID]
		if !ok {
			return false
		}
		var inner map[string]*contract.Type
		fields := decl.Fields
		if decl.Kind == contract.Generic {
			inner = map[string]*contract.Type{}
			for i, param := range decl.Params {
				if i < len(t.Args) {
					inner[param] = shapeSubstitute(t.Args[i], env)
				}
			}
			if decl.Body != nil {
				fields = decl.Body.Fields
			}
		}
		if t.ID == typeID && present(value, rel) {
			return true
		}
		return walkFields(doc, fields, value, inner, typeID, rel)
	}
	return false
}

func walkFields(doc *contract.Document, fields []*contract.Field, value any, env map[string]*contract.Type, typeID string, rel []string) bool {
	obj, ok := value.(map[string]any)
	if !ok {
		return false
	}
	for _, f := range fields {
		if fv, present := obj[f.Name]; present && usesType(doc, f.Type, fv, env, typeID, rel) {
			return true
		}
	}
	return false
}

func parsePath(changePath string) (procedure, side string, path []string) {
	tokens := strings.Fields(changePath)
	if len(tokens) < 2 || tokens[0] != "procedure" {
		return "", "", nil
	}
	procedure = tokens[1]
	rest := tokens[2:]
	if len(rest) == 0 {
		return procedure, "", nil
	}
	switch rest[0] {
	case "input", "output":
		side = rest[0]
		rest = rest[1:]
	case "error":
		if len(rest) > 1 {
			return procedure, "error", []string{rest[1]}
		}
		return procedure, "error", nil
	default:
		return procedure, "", nil
	}
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "field":
			if i+1 < len(rest) {
				path = append(path, rest[i+1])
				i++
			}
		case "elem", "value":
			path = append(path, "*")
		}
	}
	return procedure, side, path
}

func touches(c Consumer, procedure, side string, path []string) int {
	switch side {
	case "input", "output":
		return Touches(c, procedure, side, path)
	case "error":
		count := 0
		for _, in := range c.Interactions {
			if in.Procedure != procedure || in.Response.Status < 400 {
				continue
			}
			body, err := decode(in.Response.Body)
			if err != nil {
				continue
			}
			obj, _ := body.(map[string]any)
			env, _ := obj["error"].(map[string]any)
			if len(path) == 0 || env["type"] == path[0] {
				count++
			}
		}
		return count
	default:
		count := 0
		for _, in := range c.Interactions {
			if in.Procedure == procedure {
				count++
			}
		}
		return count
	}
}

func FormatText(list []Annotated) string {
	if len(list) == 0 {
		return "no contract changes\n"
	}
	var b strings.Builder
	for _, a := range list {
		fmt.Fprintf(&b, "%-9s %s: %s", a.Category, a.Path, a.Message)
		if note := a.Note(); note != "" {
			fmt.Fprintf(&b, "; %s", strings.ReplaceAll(note, "`", ""))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func FormatMarkdown(list []Annotated) string {
	if len(list) == 0 {
		return "no contract changes\n"
	}
	breaking := 0
	for _, a := range list {
		if a.Category == contract.Breaking {
			breaking++
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "**%d contract change(s), %d breaking**\n\n", len(list), breaking)
	b.WriteString("| Category | Path | Change | Consumers |\n|---|---|---|---|\n")
	for _, a := range list {
		note := a.Note()
		if note == "" {
			note = "—"
		}
		fmt.Fprintf(&b, "| %s | `%s` | %s | %s |\n", a.Category, a.Path, a.Message, note)
	}
	return b.String()
}
