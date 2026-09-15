package analyzer

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/packages"
)

func (p *Program) Doc(pkg *packages.Package, pos token.Pos) string {
	if pkg == nil {
		return ""
	}
	index, ok := p.docs[pkg]
	if !ok {
		index = buildDocIndex(pkg)
		p.docs[pkg] = index
	}
	return index[pos]
}

func buildDocIndex(pkg *packages.Package) map[token.Pos]string {
	index := map[token.Pos]string{}
	record := func(name *ast.Ident, groups ...*ast.CommentGroup) {
		for _, g := range groups {
			if text := strings.TrimSpace(g.Text()); text != "" {
				index[name.Pos()] = text
				return
			}
		}
	}
	for _, file := range pkg.Syntax {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				record(n.Name, n.Doc)
			case *ast.GenDecl:
				for _, spec := range n.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						record(s.Name, s.Doc, n.Doc)
					case *ast.ValueSpec:
						for _, name := range s.Names {
							record(name, s.Doc, s.Comment, n.Doc)
						}
					}
				}
			case *ast.Field:
				for _, name := range n.Names {
					record(name, n.Doc, n.Comment)
				}
			}
			return true
		})
	}
	return index
}
