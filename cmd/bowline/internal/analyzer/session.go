package analyzer

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"time"

	"golang.org/x/tools/go/packages"
)

type Session struct {
	dir      string
	env      []string
	patterns []string
	prog     *Program
	module   map[string]bool
	reverse  map[string][]string
	files    map[string]string
}

type Stats struct {
	Full      bool
	Rechecked []string
	Parse     time.Duration
	Check     time.Duration
}

func NewSession(dir string, env []string, patterns ...string) (*Session, error) {
	s := &Session{dir: dir, env: env, patterns: patterns}
	if err := s.Reload(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Session) Program() *Program {
	return s.prog
}

func (s *Session) Reload() error {
	prog, err := Load(s.dir, s.env, s.patterns...)
	if err != nil {
		return err
	}
	s.prog = prog
	s.module = map[string]bool{}
	s.reverse = map[string][]string{}
	s.files = map[string]string{}
	for _, pkg := range prog.Pkgs {
		if len(pkg.Syntax) == 0 {
			continue
		}
		s.module[pkg.PkgPath] = true
		for _, f := range pkg.CompiledGoFiles {
			s.files[canonicalPath(f)] = pkg.PkgPath
		}
	}
	for _, pkg := range prog.Pkgs {
		if !s.module[pkg.PkgPath] {
			continue
		}
		for _, imp := range pkg.Imports {
			if s.module[imp.PkgPath] {
				s.reverse[imp.PkgPath] = append(s.reverse[imp.PkgPath], pkg.PkgPath)
			}
		}
	}
	return nil
}

func (s *Session) Update(changed []string) (Stats, []Diagnostic, error) {
	roots := map[string]bool{}
	for _, f := range changed {
		pkgPath, ok := s.files[canonicalPath(f)]
		if !ok {
			if err := s.Reload(); err != nil {
				return Stats{Full: true}, nil, err
			}
			return Stats{Full: true}, nil, nil
		}
		roots[pkgPath] = true
	}
	affected := s.dependents(roots)
	order := s.topological(affected)
	stats := Stats{}
	var typeErrs []Diagnostic
	for _, pkgPath := range order {
		pkg := s.prog.byPath[pkgPath]
		parseDur, checkDur, diags, err := s.recheck(pkg)
		stats.Parse += parseDur
		stats.Check += checkDur
		stats.Rechecked = append(stats.Rechecked, pkgPath)
		if err != nil {
			return stats, nil, err
		}
		if len(diags) > 0 {
			typeErrs = append(typeErrs, diags...)
			break
		}
	}
	sortDiagnostics(typeErrs)
	return stats, typeErrs, nil
}

func (s *Session) dependents(roots map[string]bool) map[string]bool {
	out := map[string]bool{}
	var stack []string
	for r := range roots {
		stack = append(stack, r)
	}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if out[p] {
			continue
		}
		out[p] = true
		stack = append(stack, s.reverse[p]...)
	}
	return out
}

func (s *Session) topological(set map[string]bool) []string {
	indegree := map[string]int{}
	for p := range set {
		indegree[p] += 0
		for _, dep := range s.reverse[p] {
			if set[dep] {
				indegree[dep]++
			}
		}
	}
	var ready []string
	for p, n := range indegree {
		if n == 0 {
			ready = append(ready, p)
		}
	}
	slices.Sort(ready)
	var order []string
	for len(ready) > 0 {
		p := ready[0]
		ready = ready[1:]
		order = append(order, p)
		var next []string
		for _, dep := range s.reverse[p] {
			if !set[dep] {
				continue
			}
			indegree[dep]--
			if indegree[dep] == 0 {
				next = append(next, dep)
			}
		}
		slices.Sort(next)
		ready = append(ready, next...)
	}
	return order
}

func (s *Session) recheck(pkg *packages.Package) (time.Duration, time.Duration, []Diagnostic, error) {
	start := time.Now()
	files := make([]*ast.File, 0, len(pkg.CompiledGoFiles))
	for _, name := range pkg.CompiledGoFiles {
		f, err := parser.ParseFile(s.prog.Fset, name, nil, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			rel, relErr := filepath.Rel(s.prog.Dir, name)
			if relErr != nil {
				rel = name
			}
			return time.Since(start), 0, []Diagnostic{{Pos: token.Position{Filename: rel}, Message: err.Error(), Fix: "fix the syntax error"}}, nil
		}
		files = append(files, f)
	}
	parseDur := time.Since(start)
	start = time.Now()
	var diags []Diagnostic
	conf := types.Config{
		Importer: importerFunc(func(path string) (*types.Package, error) {
			if imp, ok := pkg.Imports[path]; ok && imp.Types != nil {
				return imp.Types, nil
			}
			return nil, fmt.Errorf("import %q not available", path)
		}),
		Error: func(err error) {
			if te, ok := errors.AsType[types.Error](err); ok {
				diags = append(diags, Diagnostic{Pos: s.prog.Position(te.Pos), Message: te.Msg, Fix: "fix the compile error"})
				return
			}
			diags = append(diags, Diagnostic{Message: err.Error()})
		},
		Sizes: pkg.TypesSizes,
	}
	if pkg.Module != nil && pkg.Module.GoVersion != "" {
		conf.GoVersion = "go" + pkg.Module.GoVersion
	}
	info := &types.Info{
		Types:        map[ast.Expr]types.TypeAndValue{},
		Defs:         map[*ast.Ident]types.Object{},
		Uses:         map[*ast.Ident]types.Object{},
		Implicits:    map[ast.Node]types.Object{},
		Selections:   map[*ast.SelectorExpr]*types.Selection{},
		Scopes:       map[ast.Node]*types.Scope{},
		Instances:    map[*ast.Ident]types.Instance{},
		FileVersions: map[*ast.File]string{},
	}
	tpkg, _ := conf.Check(pkg.PkgPath, s.prog.Fset, files, info)
	checkDur := time.Since(start)
	if len(diags) > 0 {
		return parseDur, checkDur, diags, nil
	}
	pkg.Syntax = files
	pkg.Types = tpkg
	pkg.TypesInfo = info
	pkg.IllTyped = false
	delete(s.prog.docs, pkg)
	return parseDur, checkDur, nil, nil
}

func canonicalPath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(path)
}

type importerFunc func(path string) (*types.Package, error)

func (f importerFunc) Import(path string) (*types.Package, error) {
	return f(path)
}
