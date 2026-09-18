package analyzer

import (
	"cmp"
	"fmt"
	"go/types"
	"slices"
	"strings"

	"github.com/bowlinedev/bowline/contract"
)

func Analyze(prog *Program, entry string) (*contract.Document, []Diagnostic) {
	doc := &contract.Document{
		Bowline:   contract.Version,
		Types:     map[string]*contract.TypeDecl{},
		Errors:    map[string]*contract.ErrorDecl{},
		Positions: map[string]contract.Position{},
	}
	fn, entryPkg, diag := prog.resolveEntry(entry)
	if diag != nil {
		return nil, []Diagnostic{*diag}
	}
	c := newCollector(prog, doc)
	c.scanWireAs(entryPkg)
	ev := newEvaluator(prog)
	specs := ev.routerFunc(fn, "", fn.Pos())
	diags := append([]Diagnostic{}, ev.diags...)
	if len(ev.schemes) > 0 {
		doc.Security = ev.schemes
	}
	seen := map[string]bool{}
	toolNames := map[string]string{}
	for _, spec := range specs {
		if spec.Tool != nil {
			name := strings.ReplaceAll(spec.Path, ".", "_")
			if other, dup := toolNames[name]; dup {
				diags = append(diags, Diagnostic{Pos: prog.Position(spec.Pos), Path: spec.Path, Message: fmt.Sprintf("tool name %q collides with procedure %s", name, other), Fix: "rename one procedure so the underscore-joined names differ"})
			} else {
				toolNames[name] = spec.Path
			}
		}
		if seen[spec.Path] {
			diags = append(diags, Diagnostic{Pos: prog.Position(spec.Pos), Path: spec.Path, Message: "duplicate procedure path", Fix: "rename one of the procedures"})
			continue
		}
		seen[spec.Path] = true
		proc := &contract.Procedure{
			Path:       spec.Path,
			Kind:       spec.Kind,
			Method:     spec.Method,
			GoInput:    goTypeName(spec.In),
			GoOutput:   goTypeName(spec.Out),
			Idempotent: spec.Idempotent,
			Tool:       spec.Tool,
			Deprecated: spec.Deprecated,
			Security:   sortedSecurity(spec.Security),
		}
		for _, name := range spec.Security {
			if _, ok := ev.schemes[name]; !ok {
				diags = append(diags, Diagnostic{Pos: prog.Position(spec.Pos), Path: spec.Path, Message: fmt.Sprintf("security scheme %q is required but never declared", name), Fix: "declare it with Secure(\"" + name + "\", ...) on a router, or drop the Requires option"})
			}
		}
		if len(spec.Meta) > 0 {
			proc.Meta = spec.Meta
		}
		proc.Doc = spec.Description
		if proc.Doc == "" && spec.FnPos.IsValid() {
			proc.Doc = prog.Doc(spec.Pkg, spec.FnPos)
		}
		proc.Input = c.procedureType(spec.In, spec, "input")
		proc.Output = c.procedureType(spec.Out, spec, "output")
		if proc.Input == nil || proc.Output == nil {
			continue
		}
		proc.HTTPPath = spec.HTTPPath
		if spec.HTTPPath != "" {
			if d := checkPathParams(doc, proc, spec, prog); d != nil {
				diags = append(diags, *d)
				continue
			}
			if d := checkQueryFields(doc, proc, spec, prog); d != nil {
				diags = append(diags, *d)
				continue
			}
		}
		seenErrors := map[string]bool{}
		for _, ref := range spec.Errors {
			id, ok := c.errorDecl(ref, spec.Path)
			if !ok || seenErrors[id] {
				continue
			}
			seenErrors[id] = true
			proc.Errors = append(proc.Errors, id)
		}
		slices.Sort(proc.Errors)
		c.recordPosition(spec.Path, spec.Pos)
		doc.Procedures = append(doc.Procedures, proc)
	}
	diags = append(diags, c.diags...)
	if len(diags) > 0 {
		sortDiagnostics(diags)
		return nil, diags
	}
	slices.SortFunc(doc.Procedures, func(a, b *contract.Procedure) int { return cmp.Compare(a.Path, b.Path) })
	if err := doc.SetHash(); err != nil {
		return nil, []Diagnostic{{Message: fmt.Sprintf("hashing contract: %v", err)}}
	}
	return doc, nil
}

func (c *collector) procedureType(t types.Type, spec procedureSpec, role string) *contract.Type {
	u := types.Unalias(t)
	if st, ok := u.(*types.Struct); ok && st.NumFields() == 0 {
		return &contract.Type{Kind: contract.Struct}
	}
	if _, ok := u.(*types.Named); !ok {
		return c.fail(spec.Pos, spec.Path, fmt.Sprintf("%s type %s must be a named type or struct{}", role, t), "declare a named struct for it")
	}
	return c.typeNode(u, spec.Pos, spec.Path+" "+role)
}

func bindablePrimitive(name string) bool {
	switch name {
	case "string", "int8", "int16", "int32", "int64", "uint8", "uint16", "uint32", "uint64":
		return true
	}
	return false
}

func inputFields(doc *contract.Document, node *contract.Type) []*contract.Field {
	for range 8 {
		if node == nil {
			return nil
		}
		switch node.Kind {
		case "struct":
			return node.Fields
		case "ref":
			decl, ok := doc.Types[node.ID]
			if !ok {
				return nil
			}
			if decl.Kind == "struct" {
				return decl.Fields
			}
			if decl.Kind == "generic" {
				node = decl.Body
				continue
			}
			return nil
		default:
			return nil
		}
	}
	return nil
}

func sortedSecurity(names []string) []string {
	if len(names) == 0 {
		return nil
	}
	out := slices.Clone(names)
	slices.Sort(out)
	return out
}

func sendsBody(method string) bool {
	switch method {
	case "GET", "DELETE", "HEAD":
		return false
	}
	return true
}

func queryBindable(doc *contract.Document, t *contract.Type) bool {
	if t == nil {
		return false
	}
	node := t
	if node.Kind == "array" {
		node = node.Elem
	}
	if node == nil {
		return false
	}
	if node.Kind == "ref" {
		decl, ok := doc.Types[node.ID]
		if !ok {
			return false
		}
		switch decl.Kind {
		case "enum":
			return true
		case "primitive":
			return scalarPrimitive(decl.Primitive)
		}
		return false
	}
	if node.Kind != "primitive" {
		return false
	}
	return scalarPrimitive(node.Name)
}

func scalarPrimitive(name string) bool {
	switch name {
	case "string", "bool", "int8", "int16", "int32", "int64",
		"uint8", "uint16", "uint32", "uint64", "float32", "float64":
		return true
	}
	return false
}

func checkQueryFields(doc *contract.Document, proc *contract.Procedure, spec procedureSpec, prog *Program) *Diagnostic {
	if spec.HTTPPath == "" || sendsBody(proc.Method) {
		return nil
	}
	params, err := contract.PathParams(spec.HTTPPath)
	if err != nil {
		return nil
	}
	consumed := map[string]bool{}
	for _, name := range params {
		consumed[name] = true
	}
	for _, f := range inputFields(doc, proc.Input) {
		if consumed[f.Name] || queryBindable(doc, f.Type) {
			continue
		}
		return &Diagnostic{
			Pos: prog.Position(spec.Pos), Path: spec.Path,
			Message: fmt.Sprintf("field %q cannot travel in a query string on a %s", f.Name, proc.Method),
			Fix:     "a field on a path without a body must be a string, boolean, number, or a list of those; make it a path parameter, or use a method that sends a body",
		}
	}
	return nil
}

func checkPathParams(doc *contract.Document, proc *contract.Procedure, spec procedureSpec, prog *Program) *Diagnostic {
	params, err := contract.PathParams(spec.HTTPPath)
	if err != nil {
		return &Diagnostic{Pos: prog.Position(spec.Pos), Path: spec.Path, Message: err.Error(), Fix: "use segments separated by slashes, with parameters wrapped in braces, as in invoices/{id}"}
	}
	if len(params) == 0 {
		return nil
	}
	fields := inputFields(doc, proc.Input)
	if fields == nil {
		return &Diagnostic{Pos: prog.Position(spec.Pos), Path: spec.Path, Message: fmt.Sprintf("path %q has parameters but the input is not a struct", spec.HTTPPath), Fix: "give the procedure a named struct input with one field per path parameter"}
	}
	byName := map[string]*contract.Field{}
	for _, f := range fields {
		byName[f.Name] = f
	}
	for _, name := range params {
		f, ok := byName[name]
		if !ok {
			return &Diagnostic{Pos: prog.Position(spec.Pos), Path: spec.Path, Message: fmt.Sprintf("path parameter %q has no matching input field", name), Fix: fmt.Sprintf("add a field with the JSON name %q to the input struct, or rename the parameter", name)}
		}
		if f.Type == nil || f.Type.Kind != "primitive" || !bindablePrimitive(f.Type.Name) {
			return &Diagnostic{Pos: prog.Position(spec.Pos), Path: spec.Path, Message: fmt.Sprintf("path parameter %q maps to a field that is not a string or an integer", name), Fix: "a path parameter must be carried by a string or integer field, because it travels as one path segment"}
		}
		if f.Optional || f.Nullable {
			return &Diagnostic{Pos: prog.Position(spec.Pos), Path: spec.Path, Message: fmt.Sprintf("path parameter %q maps to an optional or nullable field", name), Fix: "a path parameter is always present, so make the field required"}
		}
	}
	return nil
}
