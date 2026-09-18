package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const singleFileGenerator = `package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func main() {
	body, _ := io.ReadAll(os.Stdin)
	var doc struct {
		Procedures []struct {
			Path string ` + "`json:\"path\"`" + `
		} ` + "`json:\"procedures\"`" + `
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("// version %s out %s\n", os.Getenv("BOWLINE_CONTRACT_VERSION"), os.Getenv("BOWLINE_OUT"))
	for _, p := range doc.Procedures {
		fmt.Printf("call %s\n", p.Path)
	}
}
`

const multiFileGenerator = `package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func main() {
	io.ReadAll(os.Stdin)
	fmt.Println("bowline-files/1")
	json.NewEncoder(os.Stdout).Encode(map[string]any{"files": map[string]string{
		"client.txt": "client\n",
		"types.txt":  "types\n",
	}})
}
`

const diagnosticsGenerator = `package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	io.ReadAll(os.Stdin)
	fmt.Fprintln(os.Stderr, "api/routes.go:12:4: invoices.list: unions are not representable. split the union into two procedures")
	fmt.Fprintln(os.Stderr, "api/routes.go:20:2: invoices.get: the same applies here")
	os.Exit(2)
}
`

const failingGenerator = `package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	io.ReadAll(os.Stdin)
	fmt.Println("half a file")
	fmt.Fprintln(os.Stderr, "the toolchain is not installed")
	os.Exit(7)
}
`

func buildGenerator(t *testing.T, dir, name, source string) string {
	t.Helper()
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "main.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "go.mod"), []byte("module gen.test\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(dir, name)
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = src
	build.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building %s: %v\n%s", name, err, out)
	}
	return binary
}

func externalOptions(t *testing.T, dir, binDir string) (Options, *strings.Builder, *strings.Builder) {
	t.Helper()
	var out, errOut strings.Builder
	env := append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	env = append(env, "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return Options{Dir: dir, Env: env, Stdout: &out, Stderr: &errOut}, &out, &errOut
}

func TestExternalGeneratorSingleFile(t *testing.T) {
	dir := fixtureCopy(t)
	binDir := t.TempDir()
	buildGenerator(t, binDir, "bowline-gen-fake", singleFileGenerator)
	os.WriteFile(filepath.Join(dir, "bowline.json"), []byte(`{"entry":"./rows/routing.Routes","targets":{"fake":{"command":"bowline-gen-fake","out":"out/client.txt"}}}`), 0o644)

	opts, out, errOut := externalOptions(t, dir, binDir)
	if code := Gen(opts, nil); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "wrote out/client.txt") {
		t.Fatalf("stdout %q", out.String())
	}
	body, err := os.ReadFile(filepath.Join(dir, "out", "client.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "// version 1.3 out out/client.txt") {
		t.Fatalf("the generator did not see the environment: %q", body)
	}
	if !strings.Contains(string(body), "call ") {
		t.Fatalf("the generator did not see the document on stdin: %q", body)
	}
}

func TestExternalGeneratorMultiFile(t *testing.T) {
	dir := fixtureCopy(t)
	binDir := t.TempDir()
	buildGenerator(t, binDir, "bowline-gen-multi", multiFileGenerator)
	os.WriteFile(filepath.Join(dir, "bowline.json"), []byte(`{"entry":"./rows/routing.Routes","targets":{"multi":{"command":"bowline-gen-multi","out":"out/client.txt"}}}`), 0o644)

	opts, _, errOut := externalOptions(t, dir, binDir)
	if code := Gen(opts, nil); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	for name, want := range map[string]string{"client.txt": "client\n", "types.txt": "types\n"} {
		body, err := os.ReadFile(filepath.Join(dir, "out", name))
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != want {
			t.Fatalf("%s is %q, want %q", name, body, want)
		}
	}
}

func TestExternalGeneratorDiagnosticsExitTwo(t *testing.T) {
	dir := fixtureCopy(t)
	binDir := t.TempDir()
	buildGenerator(t, binDir, "bowline-gen-diags", diagnosticsGenerator)
	os.WriteFile(filepath.Join(dir, "bowline.json"), []byte(`{"entry":"./rows/routing.Routes","targets":{"diags":{"command":"bowline-gen-diags","out":"out/client.txt"}}}`), 0o644)

	opts, _, errOut := externalOptions(t, dir, binDir)
	if code := Gen(opts, nil); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "api/routes.go:12:4: invoices.list: unions are not representable. split the union into two procedures") {
		t.Fatalf("stderr %q", errOut.String())
	}
	if !strings.Contains(errOut.String(), "2 problem(s); nothing written") {
		t.Fatalf("stderr %q", errOut.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "out", "client.txt")); err == nil {
		t.Fatal("nothing may be written when a generator reports diagnostics")
	}
	if _, err := os.Stat(filepath.Join(dir, "bowline.contract.json")); err == nil {
		t.Fatal("the contract must not be written when a generator reports diagnostics")
	}
}

func TestExternalGeneratorFailureWritesNothing(t *testing.T) {
	dir := fixtureCopy(t)
	binDir := t.TempDir()
	buildGenerator(t, binDir, "bowline-gen-broken", failingGenerator)
	os.WriteFile(filepath.Join(dir, "bowline.json"), []byte(`{"entry":"./rows/routing.Routes","targets":{"broken":{"command":"bowline-gen-broken","out":"out/client.txt"}}}`), 0o644)

	opts, _, errOut := externalOptions(t, dir, binDir)
	if code := Gen(opts, nil); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), `exited 7: the toolchain is not installed`) {
		t.Fatalf("stderr %q", errOut.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "out", "client.txt")); err == nil {
		t.Fatal("a failing generator must write nothing")
	}
	if _, err := os.Stat(filepath.Join(dir, "bowline.contract.json")); err == nil {
		t.Fatal("a failing generator must not leave the contract behind")
	}
}

func TestExternalCommandNotResolvedFromModuleDir(t *testing.T) {
	dir := fixtureCopy(t)
	buildGenerator(t, dir, "bowline-gen-local", singleFileGenerator)
	os.WriteFile(filepath.Join(dir, "bowline.json"), []byte(`{"entry":"./rows/routing.Routes","targets":{"local":{"command":"bowline-gen-local","out":"out/client.txt"}}}`), 0o644)

	opts, _, errOut := externalOptions(t, dir, t.TempDir())
	if code := Gen(opts, nil); code != 1 {
		t.Fatalf("exit %d, want 1; a command in the module directory must not be found", code)
	}
	if !strings.Contains(errOut.String(), `"bowline-gen-local" is not on PATH`) {
		t.Fatalf("stderr %q", errOut.String())
	}
}

func TestExternalCommandMustNotBeAPath(t *testing.T) {
	dir := fixtureCopy(t)
	os.WriteFile(filepath.Join(dir, "bowline.json"), []byte(`{"entry":"./rows/routing.Routes","targets":{"local":{"command":"./bowline-gen-local","out":"out/client.txt"}}}`), 0o644)
	opts, _, errOut := externalOptions(t, dir, t.TempDir())
	if code := Gen(opts, nil); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "must be a command name resolved on PATH, not a path") {
		t.Fatalf("stderr %q", errOut.String())
	}
}

func TestExternalCommandCannotShadowABuiltinTarget(t *testing.T) {
	dir := fixtureCopy(t)
	os.WriteFile(filepath.Join(dir, "bowline.json"), []byte(`{"entry":"./rows/routing.Routes","targets":{"ts":{"command":"bowline-gen-ts","out":"out/client.ts"}}}`), 0o644)
	opts, _, errOut := externalOptions(t, dir, t.TempDir())
	if code := Gen(opts, nil); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), `"ts" is built in`) {
		t.Fatalf("stderr %q", errOut.String())
	}
}

func TestCheckWithExternalTarget(t *testing.T) {
	dir := fixtureCopy(t)
	binDir := t.TempDir()
	buildGenerator(t, binDir, "bowline-gen-multi", multiFileGenerator)
	os.WriteFile(filepath.Join(dir, "bowline.json"), []byte(`{"entry":"./rows/routing.Routes","targets":{"multi":{"command":"bowline-gen-multi","out":"out/client.txt"}}}`), 0o644)

	opts, _, errOut := externalOptions(t, dir, binDir)
	if code := Check(opts, nil); code != 1 {
		t.Fatalf("exit %d, want 1 before generating", code)
	}
	if !strings.Contains(errOut.String(), "missing   out/client.txt") || !strings.Contains(errOut.String(), "missing   out/types.txt") {
		t.Fatalf("stderr %q", errOut.String())
	}

	opts, _, errOut = externalOptions(t, dir, binDir)
	if code := Gen(opts, nil); code != 0 {
		t.Fatalf("gen exit %d: %s", code, errOut.String())
	}
	opts, _, errOut = externalOptions(t, dir, binDir)
	if code := Check(opts, nil); code != 0 {
		t.Fatalf("check exit %d: %s", code, errOut.String())
	}

	os.WriteFile(filepath.Join(dir, "out", "types.txt"), []byte("stale\n"), 0o644)
	opts, _, errOut = externalOptions(t, dir, binDir)
	if code := Check(opts, nil); code != 1 {
		t.Fatalf("check exit %d, want 1 after drift", code)
	}
	if !strings.Contains(errOut.String(), "outdated  out/types.txt") {
		t.Fatalf("stderr %q", errOut.String())
	}
}

func TestExternalPathRejectsEscapes(t *testing.T) {
	for _, name := range []string{"../escape.txt", "/etc/passwd", ""} {
		if _, err := externalPath("out", name); err == nil {
			t.Errorf("externalPath accepted %q", name)
		}
	}
	got, err := externalPath("out", "nested/client.txt")
	if err != nil || got != "out/nested/client.txt" {
		t.Fatalf("externalPath = %q, %v", got, err)
	}
}
