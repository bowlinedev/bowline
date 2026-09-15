package analyzer

import (
	"fmt"
	"go/token"
	"path/filepath"
	"sort"
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
		b.WriteString(d.Path + ": ")
	}
	b.WriteString(d.Message)
	if d.Fix != "" {
		b.WriteString(". " + d.Fix)
	}
	return b.String()
}

func sortDiagnostics(diags []Diagnostic) {
	sort.SliceStable(diags, func(i, j int) bool {
		a, b := diags[i], diags[j]
		if a.Pos.Filename != b.Pos.Filename {
			return a.Pos.Filename < b.Pos.Filename
		}
		if a.Pos.Line != b.Pos.Line {
			return a.Pos.Line < b.Pos.Line
		}
		if a.Pos.Column != b.Pos.Column {
			return a.Pos.Column < b.Pos.Column
		}
		return a.Message < b.Message
	})
}
