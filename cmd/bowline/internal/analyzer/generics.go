package analyzer

import (
	"fmt"
	"go/token"
	"go/types"
	"strings"

	"github.com/bowlinedev/bowline/contract"
)

func (c *collector) generic(t *types.Named, pos token.Pos, path string) *contract.Type {
	origin := t.Origin()
	if t.TypeArgs() == nil || t.TypeArgs().Len() == 0 {
		return c.fail(pos, path, fmt.Sprintf("%s must be instantiated", origin.Obj().Name()), "supply type arguments")
	}
	if needsMonomorphization(origin) {
		return c.monomorphize(t, pos, path)
	}
	obj := origin.Obj()
	id := pkgPath(obj) + "." + obj.Name()
	if _, done := c.doc.Types[id]; !done {
		c.declareGeneric(origin, id)
	}
	args := make([]*contract.Type, 0, t.TypeArgs().Len())
	for typeArg := range t.TypeArgs().Types() {
		arg := c.typeNode(typeArg, pos, path)
		if arg == nil {
			return nil
		}
		args = append(args, arg)
	}
	return &contract.Type{Kind: contract.Ref, ID: id, Args: args}
}

func (c *collector) declareGeneric(origin *types.Named, id string) {
	obj := origin.Obj()
	pkg := c.prog.Package(pkgPath(obj))
	decl := &contract.TypeDecl{Kind: contract.Generic, Name: obj.Name(), Doc: c.prog.Doc(pkg, obj.Pos())}
	for tparam := range origin.TypeParams().TypeParams() {
		decl.Params = append(decl.Params, tparam.Obj().Name())
	}
	c.doc.Types[id] = decl
	c.recordPosition(id, obj.Pos())
	st, ok := origin.Underlying().(*types.Struct)
	if !ok {
		c.fail(obj.Pos(), obj.Name(), "generic types must be structs", "wrap the value in a struct")
		delete(c.doc.Types, id)
		return
	}
	fields, _ := c.fields(st, obj.Pos(), obj.Name())
	decl.Body = &contract.Type{Kind: contract.Struct, Fields: fields}
}

func needsMonomorphization(origin *types.Named) bool {
	for tparam := range origin.TypeParams().TypeParams() {
		iface, ok := tparam.Constraint().Underlying().(*types.Interface)
		if ok && hasTypeTerms(iface, map[*types.Interface]bool{}) {
			return true
		}
	}
	return false
}

func hasTypeTerms(iface *types.Interface, seen map[*types.Interface]bool) bool {
	if seen[iface] {
		return false
	}
	seen[iface] = true
	for embedded := range iface.EmbeddedTypes() {
		switch e := embedded.(type) {
		case *types.Union:
			return true
		default:
			inner, ok := e.Underlying().(*types.Interface)
			if !ok {
				return true
			}
			if hasTypeTerms(inner, seen) {
				return true
			}
		}
	}
	return false
}

func (c *collector) monomorphize(t *types.Named, pos token.Pos, path string) *contract.Type {
	id := goTypeName(t)
	if _, done := c.doc.Types[id]; done {
		return &contract.Type{Kind: contract.Ref, ID: id}
	}
	origin := t.Origin()
	obj := origin.Obj()
	pkg := c.prog.Package(pkgPath(obj))
	var suffix []string
	for typeArg := range t.TypeArgs().Types() {
		suffix = append(suffix, identifierFor(typeArg))
	}
	decl := &contract.TypeDecl{
		Kind:   contract.Struct,
		Name:   obj.Name() + "_" + strings.Join(suffix, "_"),
		Doc:    c.prog.Doc(pkg, obj.Pos()),
		Origin: pkgPath(obj) + "." + obj.Name(),
	}
	c.doc.Types[id] = decl
	c.recordPosition(id, obj.Pos())
	st, ok := t.Underlying().(*types.Struct)
	if !ok {
		c.fail(pos, path, "generic types must be structs", "wrap the value in a struct")
		delete(c.doc.Types, id)
		return nil
	}
	fields, _ := c.fields(st, obj.Pos(), decl.Name)
	decl.Fields = fields
	return &contract.Type{Kind: contract.Ref, ID: id}
}

func identifierFor(t types.Type) string {
	name := types.TypeString(types.Unalias(t), func(p *types.Package) string { return p.Name() })
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return strings.Trim(b.String(), "_")
}
