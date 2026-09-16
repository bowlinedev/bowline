package analyzer

import (
	"cmp"
	"fmt"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
)

type Diagnostic struct {
	Pos     token.Position
	Path    string
	Message string
	Fix     string
}

func (d Diagnostic) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s:%d:%d: ", filepath.ToSlash(d.Pos.Filename), d.Pos.Line, d.Pos.Column)
	if d.Path != "" {
		b.WriteString(d.Path)
		b.WriteString(": ")
	}
	b.WriteString(d.Message)
	if d.Fix != "" {
		b.WriteString(". ")
		b.WriteString(d.Fix)
	}
	return b.String()
}

func sortDiagnostics(diags []Diagnostic) {
	slices.SortStableFunc(diags, func(a, b Diagnostic) int {
		return cmp.Or(
			cmp.Compare(a.Pos.Filename, b.Pos.Filename),
			cmp.Compare(a.Pos.Line, b.Pos.Line),
			cmp.Compare(a.Pos.Column, b.Pos.Column),
			cmp.Compare(a.Message, b.Message),
		)
	})
}
