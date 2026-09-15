package analyzer

import (
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"testing"
)

func testEnv() []string {
	return append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
}

func fixtureDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("testdata", "fidelity"))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadFixtureModule(t *testing.T) {
	prog, err := Load(fixtureDir(t), testEnv(), "./rows/...")
	if err != nil {
		t.Fatal(err)
	}
	pkg := prog.Package("fidelity.test/rows/basics")
	if pkg == nil || pkg.Types == nil || len(pkg.Syntax) == 0 {
		t.Fatal("basics package not loaded with syntax and types")
	}
	if prog.PackageForDir(filepath.Join(fixtureDir(t), "rows", "basics")) != pkg {
		t.Fatal("PackageForDir mismatch")
	}
	if bl := prog.Package("github.com/bowlinedev/bowline"); bl == nil {
		t.Fatal("bowline dependency not visited")
	}
	routerObj := pkg.Types.Scope().Lookup("Routes").(*types.Func)
	if !returnsRouter(routerObj) {
		t.Fatal("dependency types from export data are not usable")
	}
	obj := pkg.Types.Scope().Lookup("Routes")
	pos := prog.Position(obj.Pos())
	if pos.Filename != filepath.Join("rows", "basics", "api.go") || pos.Line != 26 {
		t.Fatalf("position %+v", pos)
	}
}

func TestLoadReportsTypeErrors(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module broken.test\n\ngo 1.24\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n\nvar x int = \"s\"\n"), 0o644)
	if _, err := Load(dir, testEnv(), "./..."); err == nil {
		t.Fatal("expected a load error")
	}
}

func TestDiagnosticString(t *testing.T) {
	d := Diagnostic{Pos: tokenPosition("users/user.go", 14, 2), Path: "User.Meta", Message: "map[string]any is not supported", Fix: "use a struct, or json.RawMessage for an untyped payload"}
	want := "users/user.go:14:2: User.Meta: map[string]any is not supported. use a struct, or json.RawMessage for an untyped payload"
	if got := d.String(); got != want {
		t.Fatalf("got %q", got)
	}
}

func tokenPosition(file string, line, col int) token.Position {
	return token.Position{Filename: file, Line: line, Column: col}
}
