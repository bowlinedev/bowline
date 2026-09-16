package analyzer

import (
	"fmt"
	"go/token"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"
)

const bowlinePath = "github.com/bowlinedev/bowline"

const loadMode = packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
	packages.NeedImports | packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo |
	packages.NeedModule

type Program struct {
	Fset   *token.FileSet
	Dir    string
	Pkgs   []*packages.Package
	byPath map[string]*packages.Package
	byDir  map[string]*packages.Package
	docs   map[*packages.Package]map[token.Pos]string
}

func Load(dir string, env []string, patterns ...string) (*Program, error) {
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	cfg := &packages.Config{Mode: loadMode, Dir: dir, Fset: token.NewFileSet(), Env: env}
	roots, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, fmt.Errorf("loading %s: %w", strings.Join(patterns, " "), err)
	}
	prog := &Program{
		Fset:   cfg.Fset,
		Dir:    dir,
		byPath: map[string]*packages.Package{},
		byDir:  map[string]*packages.Package{},
		docs:   map[*packages.Package]map[token.Pos]string{},
	}
	var problems []string
	packages.Visit(roots, nil, func(p *packages.Package) {
		prog.Pkgs = append(prog.Pkgs, p)
		prog.byPath[p.PkgPath] = p
		if len(p.GoFiles) > 0 {
			prog.byDir[filepath.Dir(p.GoFiles[0])] = p
		}
		for _, e := range p.Errors {
			problems = append(problems, e.Error())
		}
	})
	if len(problems) > 0 {
		return nil, fmt.Errorf("the module does not compile:\n  %s", strings.Join(problems, "\n  "))
	}
	return prog, nil
}

func (p *Program) Package(path string) *packages.Package {
	return p.byPath[path]
}

func (p *Program) PackageForDir(dir string) *packages.Package {
	return p.byDir[filepath.Clean(dir)]
}

func (p *Program) Position(pos token.Pos) token.Position {
	position := p.Fset.Position(pos)
	if rel, err := filepath.Rel(p.Dir, position.Filename); err == nil && !strings.HasPrefix(rel, "..") {
		position.Filename = rel
	}
	return position
}

func (p *Program) reachable(from *packages.Package) []*packages.Package {
	var out []*packages.Package
	seen := map[*packages.Package]bool{}
	var visit func(pkg *packages.Package)
	visit = func(pkg *packages.Package) {
		if pkg == nil || seen[pkg] {
			return
		}
		seen[pkg] = true
		out = append(out, pkg)
		paths := slices.Sorted(maps.Keys(pkg.Imports))
		for _, path := range paths {
			visit(p.byPath[pkg.Imports[path].PkgPath])
		}
	}
	visit(from)
	return out
}
