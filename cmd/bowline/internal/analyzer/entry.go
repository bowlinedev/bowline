package analyzer

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bowlinedev/bowline/contract"
	"golang.org/x/tools/go/packages"
)

type procedureSpec struct {
	Path        string
	Kind        string
	Method      string
	In          types.Type
	Out         types.Type
	Pkg         *packages.Package
	FnPos       token.Pos
	Pos         token.Pos
	Description string
	Deprecated  string
	Sensitive   bool
	Idempotent  bool
	Tool        *contract.Tool
	Meta        map[string]string
	Errors      []errorRef
}

func (p *Program) resolveEntry(entry string) (*types.Func, *packages.Package, *Diagnostic) {
	dot := strings.LastIndex(entry, ".")
	if !strings.HasPrefix(entry, "./") || dot <= 1 {
		return nil, nil, &Diagnostic{Message: fmt.Sprintf("entry %q must look like ./dir.Func", entry), Fix: "set \"entry\" in bowline.json to the package directory and function that returns the root router"}
	}
	dir, name := entry[:dot], entry[dot+1:]
	pkg := p.PackageForDir(filepath.Join(p.Dir, filepath.FromSlash(dir)))
	if pkg == nil {
		return nil, nil, &Diagnostic{Message: fmt.Sprintf("entry %q: no package found in %s", entry, dir), Fix: "check the directory and that it is included in the analyzed patterns"}
	}
	obj := pkg.Types.Scope().Lookup(name)
	fn, ok := obj.(*types.Func)
	if !ok {
		return nil, nil, &Diagnostic{Pos: p.Position(pkg.Syntax[0].Pos()), Message: fmt.Sprintf("entry %q: %s has no function %s", entry, pkg.PkgPath, name), Fix: "export a function returning *bowline.Router"}
	}
	if !returnsRouter(fn) {
		return nil, nil, &Diagnostic{Pos: p.Position(fn.Pos()), Path: name, Message: "entry function must have no parameters and return *bowline.Router", Fix: "change the signature to func() *bowline.Router"}
	}
	return fn, pkg, nil
}

func returnsRouter(fn *types.Func) bool {
	sig := fn.Type().(*types.Signature)
	if sig.Params().Len() != 0 || sig.Results().Len() != 1 {
		return false
	}
	ptr, ok := sig.Results().At(0).Type().(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := ptr.Elem().(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == bowlinePath && named.Obj().Name() == "Router"
}

type evaluator struct {
	prog     *Program
	diags    []Diagnostic
	visiting map[types.Object]bool
}

func newEvaluator(p *Program) *evaluator {
	return &evaluator{prog: p, visiting: map[types.Object]bool{}}
}

func (e *evaluator) fail(pos token.Pos, path, message, fix string) {
	e.diags = append(e.diags, Diagnostic{Pos: e.prog.Position(pos), Path: path, Message: message, Fix: fix})
}

func (e *evaluator) routerFunc(fn *types.Func, prefix string, at token.Pos) []procedureSpec {
	if e.visiting[fn] {
		e.fail(at, prefix, "router functions form a cycle", "mount each router once")
		return nil
	}
	e.visiting[fn] = true
	defer delete(e.visiting, fn)
	pkg := e.prog.Package(fn.Pkg().Path())
	if pkg == nil || len(pkg.Syntax) == 0 {
		e.fail(at, prefix, fmt.Sprintf("router function %s is outside the analyzed module", fn.FullName()), "mount routers defined in this module")
		return nil
	}
	decl := funcDecl(pkg, fn)
	if decl == nil || decl.Body == nil {
		e.fail(fn.Pos(), prefix, "router function has no body", "define the function in this module")
		return nil
	}
	expr := returnedExpr(pkg, decl.Body)
	if expr == nil {
		e.fail(fn.Pos(), prefix, "router structure must be static: the function must end with return bowline.NewRouter(...)", "build the router in a single return expression")
		return nil
	}
	return e.routerExpr(pkg, expr, prefix)
}

func returnedExpr(pkg *packages.Package, body *ast.BlockStmt) ast.Expr {
	if len(body.List) == 0 {
		return nil
	}
	ret, ok := body.List[len(body.List)-1].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return nil
	}
	expr := ast.Unparen(ret.Results[0])
	if id, ok := expr.(*ast.Ident); ok {
		if v, ok := pkg.TypesInfo.Uses[id].(*types.Var); ok && !isPackageLevel(v) {
			return singleAssignment(pkg, body, v)
		}
	}
	return expr
}

func isPackageLevel(v *types.Var) bool {
	return v.Parent() != nil && v.Parent() == v.Pkg().Scope()
}

func singleAssignment(pkg *packages.Package, body *ast.BlockStmt, v *types.Var) ast.Expr {
	var found ast.Expr
	count := 0
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			for i, lhs := range n.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && (pkg.TypesInfo.Defs[id] == v || pkg.TypesInfo.Uses[id] == v) && i < len(n.Rhs) {
					found = n.Rhs[i]
					count++
				}
			}
		case *ast.ValueSpec:
			for i, id := range n.Names {
				if pkg.TypesInfo.Defs[id] == v && i < len(n.Values) {
					found = n.Values[i]
					count++
				}
			}
		}
		return true
	})
	if count != 1 {
		return nil
	}
	return found
}

func (e *evaluator) routerExpr(pkg *packages.Package, expr ast.Expr, prefix string) []procedureSpec {
	expr = ast.Unparen(expr)
	switch x := expr.(type) {
	case *ast.CallExpr:
		if sel, ok := ast.Unparen(x.Fun).(*ast.SelectorExpr); ok {
			if fn, ok := pkg.TypesInfo.Uses[sel.Sel].(*types.Func); ok && isBowlineMethod(fn, "Use") {
				return e.routerExpr(pkg, sel.X, prefix)
			}
		}
		callee := bowlineFunc(pkg, x)
		if callee == "NewRouter" {
			if x.Ellipsis.IsValid() {
				e.fail(x.Ellipsis, prefix, "spread arguments to NewRouter are not supported", "list each procedure and mount explicitly")
				return nil
			}
			var specs []procedureSpec
			for _, arg := range x.Args {
				specs = append(specs, e.item(pkg, arg, prefix)...)
			}
			return specs
		}
		if fn, ok := objectOf(pkg, x.Fun).(*types.Func); ok && len(x.Args) == 0 && returnsRouter(fn) {
			return e.routerFunc(fn, prefix, x.Pos())
		}
	case *ast.Ident, *ast.SelectorExpr:
		if v, ok := objectOf(pkg, x).(*types.Var); ok && isPackageLevel(v) {
			vpkg := e.prog.Package(v.Pkg().Path())
			if init := packageVarInit(vpkg, v); init != nil {
				return e.routerExpr(vpkg, init, prefix)
			}
		}
	}
	e.fail(expr.Pos(), prefix, "router structure must be static: expected bowline.NewRouter(...), a call to a function returning *bowline.Router, or a package-level router variable", "build routers from literal bowline.NewRouter expressions")
	return nil
}

func packageVarInit(pkg *packages.Package, v *types.Var) ast.Expr {
	if pkg == nil {
		return nil
	}
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, name := range vs.Names {
					if pkg.TypesInfo.Defs[name] == v && i < len(vs.Values) {
						return vs.Values[i]
					}
				}
			}
		}
	}
	return nil
}

func (e *evaluator) item(pkg *packages.Package, expr ast.Expr, prefix string) []procedureSpec {
	call, ok := ast.Unparen(expr).(*ast.CallExpr)
	if !ok {
		e.fail(expr.Pos(), prefix, "router items must be bowline.Query, bowline.Mutation, or bowline.Mount calls", "replace the expression with a literal call")
		return nil
	}
	switch bowlineFunc(pkg, call) {
	case "Query", "Mutation", "Subscription", "Upload":
		return e.procedure(pkg, call, prefix)
	case "Mount":
		if len(call.Args) != 2 {
			e.fail(call.Pos(), prefix, "Mount takes a name and a router", "")
			return nil
		}
		name, ok := constString(pkg, call.Args[0])
		if !ok {
			e.fail(call.Args[0].Pos(), prefix, "mount name must be a string constant", "use a string literal or a const")
			return nil
		}
		return e.routerExpr(pkg, call.Args[1], joinPath(prefix, name))
	}
	e.fail(call.Pos(), prefix, "router items must be bowline.Query, bowline.Mutation, or bowline.Mount calls", "")
	return nil
}

func (e *evaluator) procedure(pkg *packages.Package, call *ast.CallExpr, prefix string) []procedureSpec {
	kind := strings.ToLower(bowlineFunc(pkg, call))
	if len(call.Args) < 2 {
		e.fail(call.Pos(), prefix, kind+" takes a name and a handler", "")
		return nil
	}
	name, ok := constString(pkg, call.Args[0])
	if !ok {
		e.fail(call.Args[0].Pos(), prefix, "procedure name must be a string constant", "use a string literal or a const")
		return nil
	}
	id := calleeIdent(call)
	inst, ok := pkg.TypesInfo.Instances[id]
	if !ok || inst.TypeArgs.Len() != 2 {
		e.fail(call.Pos(), joinPath(prefix, name), "could not determine input and output types", "pass a func(context.Context, In) (Out, error)")
		return nil
	}
	spec := procedureSpec{
		Path:   joinPath(prefix, name),
		Kind:   kind,
		Method: "POST",
		In:     inst.TypeArgs.At(0),
		Out:    inst.TypeArgs.At(1),
		Pkg:    pkg,
		Pos:    call.Pos(),
		Meta:   map[string]string{},
	}
	if fn, ok := objectOf(pkg, call.Args[1]).(*types.Func); ok {
		spec.FnPos = fn.Pos()
		spec.Pkg = e.prog.Package(fn.Pkg().Path())
		if spec.Pkg == nil {
			spec.Pkg = pkg
		}
	}
	for _, opt := range call.Args[2:] {
		e.option(pkg, opt, &spec)
	}
	if (kind == "query" || kind == "subscription") && !spec.Sensitive {
		spec.Method = "GET"
	}
	return []procedureSpec{spec}
}

func (e *evaluator) option(pkg *packages.Package, expr ast.Expr, spec *procedureSpec) {
	call, ok := ast.Unparen(expr).(*ast.CallExpr)
	if !ok {
		e.fail(expr.Pos(), spec.Path, "procedure options must be literal bowline option calls", "use bowline.Description, Deprecated, Sensitive, Meta, or Use")
		return
	}
	switch bowlineFunc(pkg, call) {
	case "Description":
		spec.Description, _ = e.constArg(pkg, call, 0, spec.Path)
	case "Deprecated":
		spec.Deprecated, _ = e.constArg(pkg, call, 0, spec.Path)
	case "Sensitive":
		spec.Sensitive = true
	case "Idempotent":
		spec.Idempotent = true
	case "Tool":
		if spec.Kind == "subscription" || spec.Kind == "upload" {
			e.fail(call.Pos(), spec.Path, "subscriptions and uploads cannot be exposed as tools", "expose a query that returns a snapshot instead")
			return
		}
		tool := &contract.Tool{ReadOnly: spec.Kind == "query"}
		for _, arg := range call.Args {
			e.toolOption(pkg, arg, spec, tool)
		}
		spec.Tool = tool
	case "Meta":
		k, ok1 := e.constArg(pkg, call, 0, spec.Path)
		v, ok2 := e.constArg(pkg, call, 1, spec.Path)
		if ok1 && ok2 {
			spec.Meta[k] = v
		}
	case "Errors":
		for _, arg := range call.Args {
			tv, ok := pkg.TypesInfo.Types[arg]
			if !ok || tv.Type == nil {
				e.fail(arg.Pos(), spec.Path, "could not determine the error variant type", "pass a value of the variant type, such as InvoiceLocked{}")
				continue
			}
			spec.Errors = append(spec.Errors, errorRef{Type: tv.Type, Pos: arg.Pos()})
		}
	case "Use":
	default:
		e.fail(call.Pos(), spec.Path, "unsupported procedure option", "use bowline.Description, Deprecated, Sensitive, Idempotent, Tool, Meta, Errors, or Use")
	}
}

func (e *evaluator) toolOption(pkg *packages.Package, expr ast.Expr, spec *procedureSpec, tool *contract.Tool) {
	call, ok := ast.Unparen(expr).(*ast.CallExpr)
	if !ok {
		e.fail(expr.Pos(), spec.Path, "tool options must be literal bowline.Scope or bowline.Destructive calls", "")
		return
	}
	switch bowlineFunc(pkg, call) {
	case "Scope":
		for i := range call.Args {
			name, ok := e.constArg(pkg, call, i, spec.Path)
			if !ok {
				continue
			}
			if name == "" {
				e.fail(call.Args[i].Pos(), spec.Path, "scope names must not be empty", "")
				continue
			}
			if !slices.Contains(tool.Scopes, name) {
				tool.Scopes = append(tool.Scopes, name)
			}
		}
	case "Destructive":
		tool.Destructive = true
	default:
		e.fail(call.Pos(), spec.Path, "unsupported tool option", "use bowline.Scope or bowline.Destructive")
	}
}

func (e *evaluator) constArg(pkg *packages.Package, call *ast.CallExpr, i int, path string) (string, bool) {
	if i >= len(call.Args) {
		e.fail(call.Pos(), path, "missing option argument", "")
		return "", false
	}
	s, ok := constString(pkg, call.Args[i])
	if !ok {
		e.fail(call.Args[i].Pos(), path, "option arguments must be string constants", "use a string literal or a const")
	}
	return s, ok
}

func constString(pkg *packages.Package, expr ast.Expr) (string, bool) {
	tv, ok := pkg.TypesInfo.Types[expr]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(tv.Value), true
}

func calleeIdent(call *ast.CallExpr) *ast.Ident {
	switch f := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		return f
	case *ast.SelectorExpr:
		return f.Sel
	case *ast.IndexExpr:
		return calleeIdent(&ast.CallExpr{Fun: f.X})
	case *ast.IndexListExpr:
		return calleeIdent(&ast.CallExpr{Fun: f.X})
	}
	return nil
}

func bowlineFunc(pkg *packages.Package, call *ast.CallExpr) string {
	id := calleeIdent(call)
	if id == nil {
		return ""
	}
	fn, ok := pkg.TypesInfo.Uses[id].(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != bowlinePath || fn.Type().(*types.Signature).Recv() != nil {
		return ""
	}
	return fn.Name()
}

func isBowlineMethod(fn *types.Func, name string) bool {
	return fn.Pkg() != nil && fn.Pkg().Path() == bowlinePath && fn.Name() == name && fn.Type().(*types.Signature).Recv() != nil
}

func objectOf(pkg *packages.Package, expr ast.Expr) types.Object {
	switch x := ast.Unparen(expr).(type) {
	case *ast.Ident:
		return pkg.TypesInfo.Uses[x]
	case *ast.SelectorExpr:
		return pkg.TypesInfo.Uses[x.Sel]
	}
	return nil
}

func funcDecl(pkg *packages.Package, fn *types.Func) *ast.FuncDecl {
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			if fd, ok := decl.(*ast.FuncDecl); ok && pkg.TypesInfo.Defs[fd.Name] == fn {
				return fd
			}
		}
	}
	return nil
}

func joinPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}
