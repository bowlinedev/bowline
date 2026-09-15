package analyzer

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"github.com/bowlinedev/bowline/contract"
)

type errorRef struct {
	Type types.Type
	Pos  token.Pos
}

func (c *collector) errorDecl(ref errorRef, path string) (string, bool) {
	t := types.Unalias(ref.Type)
	if p, ok := t.(*types.Pointer); ok {
		t = types.Unalias(p.Elem())
	}
	named, ok := t.(*types.Named)
	if !ok {
		c.fail(ref.Pos, path, fmt.Sprintf("error variant %s must be a named struct type", ref.Type), "declare a struct type with Error and Code methods")
		return "", false
	}
	st, ok := named.Underlying().(*types.Struct)
	if !ok {
		c.fail(ref.Pos, path, fmt.Sprintf("error variant %s must be a struct type", named.Obj().Name()), "wrap the value in a struct")
		return "", false
	}
	id := goTypeName(named)
	if _, done := c.doc.Errors[id]; done {
		return id, true
	}
	obj := named.Obj()
	pkg := c.prog.Package(pkgPath(obj))
	code, ok := c.constantCode(named, path)
	if !ok {
		return "", false
	}
	fields, ok := c.fields(st, obj.Pos(), obj.Name())
	if !ok {
		return "", false
	}
	if len(fields) == 0 {
		c.fail(ref.Pos, path, fmt.Sprintf("error variant %s has no exported fields, so its details would be empty", obj.Name()), "export the fields the client needs, or return bowline.Errorf instead")
		return "", false
	}
	c.doc.Errors[id] = &contract.ErrorDecl{Name: obj.Name(), Code: code, Doc: c.prog.Doc(pkg, obj.Pos()), Fields: fields}
	c.recordPosition(id, obj.Pos())
	return id, true
}

func (c *collector) constantCode(named *types.Named, path string) (string, bool) {
	obj, _, _ := types.LookupFieldOrMethod(named, true, nil, "Code")
	fn, ok := obj.(*types.Func)
	if !ok {
		c.fail(named.Obj().Pos(), path, fmt.Sprintf("error variant %s has no Code method", named.Obj().Name()), "add func (e T) Code() bowline.Code returning a constant")
		return "", false
	}
	pkg := c.prog.Package(pkgPath(fn))
	if pkg == nil || len(pkg.Syntax) == 0 {
		c.fail(named.Obj().Pos(), path, "error variants must be declared in the analyzed module", "")
		return "", false
	}
	decl := funcDecl(pkg, fn)
	if decl == nil || decl.Body == nil || len(decl.Body.List) != 1 {
		return c.failCode(named, path)
	}
	ret, ok := decl.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return c.failCode(named, path)
	}
	tv, ok := pkg.TypesInfo.Types[ret.Results[0]]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return c.failCode(named, path)
	}
	return constant.StringVal(tv.Value), true
}

func (c *collector) failCode(named *types.Named, path string) (string, bool) {
	c.fail(named.Obj().Pos(), path, fmt.Sprintf("the Code method of %s must be a single return of a bowline.Code constant", named.Obj().Name()), "return one constant such as bowline.NotFound; branch in the handler instead")
	return "", false
}
