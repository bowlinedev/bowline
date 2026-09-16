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
