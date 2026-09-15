package analyzer

import (
	"fmt"
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/packages"
)

func (c *collector) scanWireAs(from *packages.Package) {
	for _, pkg := range c.prog.reachable(from) {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || bowlineFunc(pkg, call) != "WireAs" {
					return true
				}
				inst, ok := pkg.TypesInfo.Instances[calleeIdent(call)]
				if !ok || inst.TypeArgs.Len() != 2 {
					return true
				}
				target := inst.TypeArgs.At(0)
				wire := inst.TypeArgs.At(1)
				key := goTypeName(target)
				if existing, dup := c.wireAs[key]; dup && !types.Identical(existing, wire) {
					c.fail(call.Pos(), key, fmt.Sprintf("WireAs declared twice with different wire types %s and %s", existing, wire), "keep one declaration")
					return true
				}
				c.wireAs[key] = wire
				return true
			})
		}
	}
}
