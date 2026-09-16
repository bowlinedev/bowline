package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const anyTypeGenerator = `package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
)

func main() {
	body, _ := io.ReadAll(os.Stdin)
	var doc struct {
		Types map[string]json.RawMessage ` + "`json:\"types\"`" + `
	}
	json.Unmarshal(body, &doc)
	names := make([]string, 0, len(doc.Types))
	for id := range doc.Types {
		names = append(names, id)
	}
	sort.Strings(names)
	fmt.Println("// generated")
	for i := range names {
		fmt.Printf("export type T%d = any;\n", i)
	}
}
`

const nondeterministicGenerator = `package main

import (
	"fmt"
	"io"
	"os"
	"time"
)

func main() {
	io.ReadAll(os.Stdin)
	fmt.Printf("// generated at %d\n", time.Now().UnixNano())
}
`

func certifyOptions(t *testing.T, dir, binDir string) (Options, *strings.Builder, *strings.Builder) {
	t.Helper()
	var out, errOut strings.Builder
	env := append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	if binDir != "" {
		env = append(env, "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	return Options{Dir: dir, Env: env, Stdout: &out, Stderr: &errOut}, &out, &errOut
}

func TestCertifyPassesBuiltinTS(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "certify.json"), []byte(`{"target":"ts","generator":"ts","repository":"built in","compile":["true"],"test":["true"]}`), 0o644)
	opts, out, errOut := certifyOptions(t, dir, "")
	if code := Certify(opts, nil); code != 0 {
		t.Fatalf("exit %d\nstdout %s\nstderr %s", code, out.String(), errOut.String())
	}
	for _, want := range []string{
		"ok        fidelity rows generate",
		"ok        no escape-hatch types",
		"ok        generator is deterministic",
		"ok        reject rows stop at the analyzer",
		"ok        goldens compile",
		"ok        runtime tests",
		"certified ts with ts",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("stdout is missing %q:\n%s", want, out.String())
		}
	}
}

func TestCertifyPassesBuiltinRustDespiteRawPrimitive(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "certify.json"), []byte(`{"target":"rust","generator":"rust","compile":["true"],"test":["true"]}`), 0o644)
	opts, out, errOut := certifyOptions(t, dir, "")
	if code := Certify(opts, nil); code != 0 {
		t.Fatalf("exit %d\nstdout %s\nstderr %s", code, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), "ok        no escape-hatch types") {
		t.Fatalf("serde_json::Value is allowed where the contract has a raw primitive:\n%s", out.String())
	}
}

func TestCertifyFailsOnAnyType(t *testing.T) {
	dir := t.TempDir()
	binDir := t.TempDir()
	buildGenerator(t, binDir, "bowline-gen-any", anyTypeGenerator)
	os.WriteFile(filepath.Join(dir, "certify.json"), []byte(`{"target":"ts","generator":"bowline-gen-any","compile":["true"],"test":["true"]}`), 0o644)
	opts, _, errOut := certifyOptions(t, dir, binDir)
	if code := Certify(opts, nil); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "failed    no escape-hatch types") {
		t.Fatalf("stderr %q", errOut.String())
	}
	if !strings.Contains(errOut.String(), `uses "any"`) {
		t.Fatalf("stderr %q", errOut.String())
	}
}

func TestCertifyFailsOnNondeterminism(t *testing.T) {
	dir := t.TempDir()
	binDir := t.TempDir()
	buildGenerator(t, binDir, "bowline-gen-clock", nondeterministicGenerator)
	os.WriteFile(filepath.Join(dir, "certify.json"), []byte(`{"target":"kotlin","generator":"bowline-gen-clock","compile":["true"],"test":["true"]}`), 0o644)
	opts, _, errOut := certifyOptions(t, dir, binDir)
	if code := Certify(opts, nil); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "failed    generator is deterministic") {
		t.Fatalf("stderr %q", errOut.String())
	}
	if !strings.Contains(errOut.String(), "differs between two runs") {
		t.Fatalf("stderr %q", errOut.String())
	}
}

func TestCertifyFailsWhenTheToolchainIsNotDeclared(t *testing.T) {
	dir := t.TempDir()
	opts, out, errOut := certifyOptions(t, dir, "")
	if code := Certify(opts, []string{"--target", "ts", "--generator", "ts"}); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(out.String(), "skip      goldens compile: certify.json declares no command") {
		t.Fatalf("stdout %q", out.String())
	}
	if !strings.Contains(errOut.String(), "2 check(s) could not run") {
		t.Fatalf("stderr %q", errOut.String())
	}
}

func TestCertifyFailsWhenTheToolchainFails(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "certify.json"), []byte(`{"target":"ts","generator":"ts","compile":["false"],"test":["true"]}`), 0o644)
	opts, _, errOut := certifyOptions(t, dir, "")
	if code := Certify(opts, nil); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "failed    goldens compile") {
		t.Fatalf("stderr %q", errOut.String())
	}
}

func TestCertifyReportFormat(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "certify.json"), []byte(`{"target":"ts","generator":"ts","repository":"built in","version":"9.9.9","compile":["true"],"test":["true"]}`), 0o644)
	opts, out, errOut := certifyOptions(t, dir, "")
	if code := Certify(opts, []string{"--report", "--out", "certified.md"}); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "wrote certified.md") {
		t.Fatalf("stdout %q", out.String())
	}
	body, err := os.ReadFile(filepath.Join(dir, "certified.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "# Certified generators") {
		t.Fatalf("missing header:\n%s", text)
	}
	if !strings.Contains(text, "| Target | Generator | Version certified | Bowline version | Date |") {
		t.Fatalf("missing table head:\n%s", text)
	}
	if !strings.Contains(text, "| `ts` | built in | 9.9.9 |") {
		t.Fatalf("missing row:\n%s", text)
	}

	opts, _, errOut = certifyOptions(t, dir, "")
	if code := Certify(opts, []string{"--report", "--out", "certified.md"}); code != 0 {
		t.Fatalf("second run exit %d: %s", code, errOut.String())
	}
	second, err := os.ReadFile(filepath.Join(dir, "certified.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(second), "| `ts` |") != 1 {
		t.Fatalf("re-certifying must replace the row, not append one:\n%s", second)
	}
}

func TestCertifyNeedsATargetAndGenerator(t *testing.T) {
	dir := t.TempDir()
	opts, _, errOut := certifyOptions(t, dir, "")
	if code := Certify(opts, []string{"--target", "ts"}); code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "--generator is required") {
		t.Fatalf("stderr %q", errOut.String())
	}
}
