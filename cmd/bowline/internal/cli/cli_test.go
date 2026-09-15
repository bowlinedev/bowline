package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureCopy(t *testing.T) string {
	t.Helper()
	src, err := filepath.Abs(filepath.Join("..", "analyzer", "testdata", "fidelity"))
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()
	if err := os.CopyFS(dst, os.DirFS(src)); err != nil {
		t.Fatal(err)
	}
	gomod := "module fidelity.test\n\ngo 1.24\n\nrequire github.com/bowlinedev/bowline v0.0.0\n\nreplace github.com/bowlinedev/bowline => " + filepath.ToSlash(root) + "\n"
	if err := os.WriteFile(filepath.Join(dst, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}
	return dst
}

func testOptions(dir string) (Options, *bytes.Buffer, *bytes.Buffer) {
	var out, errOut bytes.Buffer
	return Options{Dir: dir, Env: append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod"), Stdout: &out, Stderr: &errOut}, &out, &errOut
}

func TestGenThenCheck(t *testing.T) {
	dir := fixtureCopy(t)
	os.WriteFile(filepath.Join(dir, "bowline.json"), []byte(`{"entry":"./rows/routing.Routes","contract":"out/contract.json"}`), 0o644)
	opts, out, errOut := testOptions(dir)
	if code := Gen(opts); code != 0 {
		t.Fatalf("gen exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "wrote out/contract.json") {
		t.Fatalf("stdout %q", out.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "out", "contract.json")); err != nil {
		t.Fatal(err)
	}
	opts, _, errOut = testOptions(dir)
	if code := Check(opts, nil); code != 0 {
		t.Fatalf("check exit %d: %s", code, errOut.String())
	}
	opts, out, _ = testOptions(dir)
	Gen(opts)
	if !strings.Contains(out.String(), "unchanged out/contract.json") {
		t.Fatalf("second gen stdout %q", out.String())
	}
}

func TestCheckDetectsDrift(t *testing.T) {
	dir := fixtureCopy(t)
	os.WriteFile(filepath.Join(dir, "bowline.json"), []byte(`{"entry":"./rows/routing.Routes"}`), 0o644)
	opts, _, errOut := testOptions(dir)
	if code := Check(opts, nil); code != 1 || !strings.Contains(errOut.String(), "missing   bowline.contract.json") {
		t.Fatalf("exit %d stderr %s", code, errOut.String())
	}
	Gen(testOptionsOnly(dir))
	os.WriteFile(filepath.Join(dir, "bowline.contract.json"), []byte("{}"), 0o644)
	opts, _, errOut = testOptions(dir)
	if code := Check(opts, nil); code != 1 || !strings.Contains(errOut.String(), "outdated  bowline.contract.json") {
		t.Fatalf("exit %d stderr %s", code, errOut.String())
	}
}

func testOptionsOnly(dir string) Options {
	opts, _, _ := testOptions(dir)
	return opts
}

func TestGenReportsDiagnostics(t *testing.T) {
	dir := fixtureCopy(t)
	os.WriteFile(filepath.Join(dir, "bowline.json"), []byte(`{"entry":"./rows/reject-interface.Routes"}`), 0o644)
	opts, _, errOut := testOptions(dir)
	if code := Gen(opts); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(errOut.String(), "rows/reject-interface/api.go:") || !strings.Contains(errOut.String(), "nothing written") {
		t.Fatalf("stderr %s", errOut.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "bowline.contract.json")); err == nil {
		t.Fatal("contract must not be written on diagnostics")
	}
}

func TestUnknownTarget(t *testing.T) {
	dir := fixtureCopy(t)
	os.WriteFile(filepath.Join(dir, "bowline.json"), []byte(`{"entry":"./rows/routing.Routes","targets":{"cobol":{"out":"x"}}}`), 0o644)
	opts, _, errOut := testOptions(dir)
	if code := Gen(opts); code != 1 || !strings.Contains(errOut.String(), `unknown target "cobol"`) {
		t.Fatalf("exit %d stderr %s", code, errOut.String())
	}
}
