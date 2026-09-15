package analyzer

import (
	"go/types"
	"strings"
)

func goTypeName(t types.Type) string {
	t = types.Unalias(t)
	if st, ok := t.(*types.Struct); ok && st.NumFields() == 0 {
		return "struct{}"
	}
	return strings.ReplaceAll(types.TypeString(t, func(p *types.Package) string { return p.Path() }), ", ", ",")
}
