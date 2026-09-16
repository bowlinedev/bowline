package bowline_test

import (
	"flag"
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var updateAPI = flag.Bool("update-api", false, "rewrite docs/api-freeze.md from the current exported surface")

const freezeList = "docs/api-freeze.md"

var frozenPackages = []struct {
	Dir    string
	Import string
}{
	{".", "github.com/bowlinedev/bowline"},
	{"contract", "github.com/bowlinedev/bowline/contract"},
	{"signing", "github.com/bowlinedev/bowline/signing"},
}

func TestPublicIdentifiersMatchFreezeList(t *testing.T) {
	current := map[string][]string{}
	for _, pkg := range frozenPackages {
		current[pkg.Import] = exportedIdentifiers(t, pkg.Dir)
	}
	if *updateAPI {
		if err := os.WriteFile(freezeList, renderFreezeList(current), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Log("rewrote " + freezeList)
		return
	}
	frozen := parseFreezeList(t)
	for _, pkg := range frozenPackages {
		want, have := frozen[pkg.Import], current[pkg.Import]
		if want == nil {
			t.Errorf("%s: %s lists no identifiers for this package", freezeList, pkg.Import)
			continue
		}
		for _, id := range diff(want, have) {
			t.Errorf("%s: %s is frozen in %s but no longer exported; removing it breaks the 1.x guarantee", pkg.Import, id, freezeList)
		}
		for _, id := range diff(have, want) {
			t.Errorf("%s: %s is exported but missing from %s; add it there with `go test -run TestPublicIdentifiersMatchFreezeList -update-api`", pkg.Import, id, freezeList)
		}
	}
}

func parseFreezeList(t *testing.T) map[string][]string {
	t.Helper()
	data, err := os.ReadFile(freezeList)
	if err != nil {
		t.Fatal(err)
	}
	lists := map[string][]string{}
	var pkg string
	var inBlock bool
	for line := range strings.SplitSeq(string(data), "\n") {
		switch {
		case strings.HasPrefix(line, "## "):
			pkg = strings.TrimSpace(strings.TrimPrefix(line, "## "))
		case strings.HasPrefix(line, "```"):
			inBlock = !inBlock
		case inBlock && pkg != "" && strings.TrimSpace(line) != "":
			lists[pkg] = append(lists[pkg], strings.TrimSpace(line))
		}
	}
	for _, ids := range lists {
		sort.Strings(ids)
	}
	return lists
}

func diff(from, minus []string) []string {
	have := map[string]bool{}
	for _, id := range minus {
		have[id] = true
	}
	var only []string
	for _, id := range from {
		if !have[id] {
			only = append(only, id)
		}
	}
	return only
}

func exportedIdentifiers(t *testing.T, dir string) []string {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		t.Fatalf("parsing %s: %v", dir, err)
	}
	var ids []string
	for name, pkg := range pkgs {
		if strings.HasSuffix(name, "_test") {
			continue
		}
		ids = append(ids, identifiersOf(doc.New(pkg, filepath.ToSlash(dir), 0))...)
	}
	sort.Strings(ids)
	return ids
}

func identifiersOf(d *doc.Package) []string {
	var ids []string
	add := func(kind, name string) {
		if ast.IsExported(strings.SplitN(name, ".", 2)[0]) {
			ids = append(ids, kind+" "+name)
		}
	}
	for _, c := range d.Consts {
		for _, n := range c.Names {
			add("const", n)
		}
	}
	for _, v := range d.Vars {
		for _, n := range v.Names {
			add("var", n)
		}
	}
	for _, f := range d.Funcs {
		add("func", f.Name)
	}
	for _, typ := range d.Types {
		if !ast.IsExported(typ.Name) {
			continue
		}
		add("type", typ.Name)
		for _, c := range typ.Consts {
			for _, n := range c.Names {
				add("const", n)
			}
		}
		for _, v := range typ.Vars {
			for _, n := range v.Names {
				add("var", n)
			}
		}
		for _, f := range typ.Funcs {
			add("func", f.Name)
		}
		for _, m := range typ.Methods {
			add("method", typ.Name+"."+m.Name)
		}
		ids = append(ids, membersOf(typ)...)
	}
	return ids
}

func membersOf(typ *doc.Type) []string {
	spec, ok := typ.Decl.Specs[0].(*ast.TypeSpec)
	if !ok {
		return nil
	}
	var ids []string
	switch underlying := spec.Type.(type) {
	case *ast.StructType:
		for _, field := range underlying.Fields.List {
			for _, name := range field.Names {
				if ast.IsExported(name.Name) {
					ids = append(ids, "field "+typ.Name+"."+name.Name)
				}
			}
		}
	case *ast.InterfaceType:
		for _, method := range underlying.Methods.List {
			for _, name := range method.Names {
				if ast.IsExported(name.Name) {
					ids = append(ids, "method "+typ.Name+"."+name.Name)
				}
			}
		}
	}
	return ids
}

func renderFreezeList(current map[string][]string) []byte {
	var out strings.Builder
	out.WriteString(freezeHeader)
	for _, pkg := range frozenPackages {
		fmt.Fprintf(&out, "\n## %s\n\n```\n", pkg.Import)
		for _, id := range current[pkg.Import] {
			out.WriteString(id + "\n")
		}
		out.WriteString("```\n")
	}
	return []byte(out.String())
}

const freezeHeader = `# Frozen public API

Every identifier below is guaranteed for the life of the major version, as ` + "`docs/stability.md`" + ` describes: it is not removed, its signature does not change, and an exported field does not change type. New identifiers may be added in a minor release, which is why this list grows but never shrinks.

` + "`TestPublicIdentifiersMatchFreezeList`" + ` compares this file with the real exported surface and fails in both directions, so the list cannot drift. Regenerate it with:

` + "```bash\ngo test -run TestPublicIdentifiersMatchFreezeList -update-api\n```" + `

` + "`scripts/apidiff.sh`" + ` is the other half: it compares the working tree against the previous release tag and fails on an incompatible change. This file says what is promised; the script says whether the promise was kept.

Packages under ` + "`internal/`" + ` are absent by design and carry no guarantee. ` + "`docs/security/api-audit.md`" + ` records why each identifier is here.
`
