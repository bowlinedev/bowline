package analyzer

import (
	"cmp"
	"go/constant"
	"go/token"
	"go/types"
	"slices"

	"github.com/bowlinedev/bowline/contract"
)

func (c *collector) basicDecl(t *types.Named, basic *types.Basic, decl *contract.TypeDecl) {
	values := enumValues(t, basic)
	if len(values) == 0 {
		decl.Kind = contract.Primitive
		decl.Primitive = basicNames[basic.Kind()]
		return
	}
	decl.Kind = contract.Enum
	decl.Base = basicNames[basic.Kind()]
	decl.Values = values
}

func enumValues(t *types.Named, basic *types.Basic) []contract.EnumValue {
	pkg := t.Obj().Pkg()
	if pkg == nil || basic.Info()&(types.IsString|types.IsInteger) == 0 {
		return nil
	}
	type positioned struct {
		pos   token.Pos
		value contract.EnumValue
	}
	var found []positioned
	scope := pkg.Scope()
	for _, name := range scope.Names() {
		cst, ok := scope.Lookup(name).(*types.Const)
		if !ok || !types.Identical(cst.Type(), t) {
			continue
		}
		var value any
		switch {
		case basic.Info()&types.IsString != 0:
			value = constant.StringVal(cst.Val())
		case basic.Info()&types.IsUnsigned != 0:
			value, _ = constant.Uint64Val(cst.Val())
		default:
			value, _ = constant.Int64Val(cst.Val())
		}
		found = append(found, positioned{pos: cst.Pos(), value: contract.EnumValue{Name: name, Value: value}})
	}
	slices.SortFunc(found, func(a, b positioned) int { return cmp.Compare(a.pos, b.pos) })
	out := make([]contract.EnumValue, len(found))
	for i, f := range found {
		out[i] = f.value
	}
	return out
}
