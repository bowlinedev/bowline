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
	if code := Gen(opts, nil); code != 0 {
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
	Gen(opts, nil)
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
	Gen(testOptionsOnly(dir), nil)
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
	if code := Gen(opts, nil); code != 1 {
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
	if code := Gen(opts, nil); code != 1 || !strings.Contains(errOut.String(), `unknown target "cobol"`) {
		t.Fatalf("exit %d stderr %s", code, errOut.String())
	}
}

func TestGenWritesOpenAPIWhenConfigured(t *testing.T) {
	dir := fixtureCopy(t)
	os.WriteFile(filepath.Join(dir, "bowline.json"), []byte(`{"entry":"./rows/routing.Routes","contract":"api/c.json","openapi":{"title":"Routing","version":"1.2.3"}}`), 0o644)
	opts, out, errOut := testOptions(dir)
	if code := Gen(opts, nil); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "wrote api/openapi.json") {
		t.Fatalf("stdout %q", out.String())
	}
	data, _ := os.ReadFile(filepath.Join(dir, "api", "openapi.json"))
	if !strings.Contains(string(data), `"title": "Routing"`) || !strings.Contains(string(data), `"1.2.3"`) {
		t.Fatalf("openapi %s", data[:200])
	}
	opts, _, errOut = testOptions(dir)
	if code := Check(opts, nil); code != 0 {
		t.Fatalf("check exit %d: %s", code, errOut.String())
	}
}

func TestExportToolsToStdoutAndFile(t *testing.T) {
	dir := fixtureCopy(t)
	os.WriteFile(filepath.Join(dir, "bowline.json"), []byte(`{"entry":"./rows/tools.Routes"}`), 0o644)
	opts, out, errOut := testOptions(dir)
	if code := Export(opts, []string{"tools", "--format", "anthropic", "--scope", "crm"}); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), `"input_schema"`) || strings.Contains(out.String(), `"name": "get"`) || !strings.Contains(out.String(), `"name": "search"`) {
		t.Fatalf("stdout %s", out.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "tools.json")); err == nil {
		t.Fatal("nothing must be written without --out")
	}
	opts, out, _ = testOptions(dir)
	if code := Export(opts, []string{"tools", "--out", "agent/tools.json", "--read-only"}); code != 0 || !strings.Contains(out.String(), "wrote agent/tools.json") {
		t.Fatalf("exit %d out %q", code, out.String())
	}
	data, _ := os.ReadFile(filepath.Join(dir, "agent", "tools.json"))
	if strings.Contains(string(data), `"remove"`) || !strings.Contains(string(data), `"outputSchema"`) {
		t.Fatalf("file %s", data[:200])
	}
}

func TestToolsTargetAndSchemas(t *testing.T) {
	dir := fixtureCopy(t)
	os.WriteFile(filepath.Join(dir, "bowline.json"), []byte(`{"entry":"./rows/tools.Routes","schemas":true,"targets":{"tools":{"out":"tools.json","format":"openai"}}}`), 0o644)
	opts, _, errOut := testOptions(dir)
	if code := Gen(opts, nil); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	data, _ := os.ReadFile(filepath.Join(dir, "tools.json"))
	if !strings.Contains(string(data), `"type": "function"`) {
		t.Fatalf("tools %s", data[:120])
	}
	contractData, _ := os.ReadFile(filepath.Join(dir, "bowline.contract.json"))
	if !strings.Contains(string(contractData), `"schemas"`) || !strings.Contains(string(contractData), `"$defs"`) {
		t.Fatal("contract lacks embedded schemas")
	}
	opts, _, errOut = testOptions(dir)
	if code := Check(opts, nil); code != 0 {
		t.Fatalf("check exit %d: %s", code, errOut.String())
	}
}

func TestGenFromAnExistingDocument(t *testing.T) {
	dir := fixtureCopy(t)
	os.WriteFile(filepath.Join(dir, "bowline.json"), []byte(`{"entry":"./rows/routing.Routes","targets":{"ts":{"out":"out/bowline.ts"}}}`), 0o644)
	if code := Gen(testOptionsOnly(dir), nil); code != 0 {
		t.Fatal("gen failed")
	}
	source, err := os.ReadFile(filepath.Join(dir, "bowline.contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(dir, "composed"), 0o755)
	os.WriteFile(filepath.Join(dir, "composed", "contract.json"), source, 0o644)
	os.Remove(filepath.Join(dir, "out", "bowline.ts"))
	opts, out, errOut := testOptions(dir)
	if code := Gen(opts, []string{"--from", "composed/contract.json"}); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "wrote out/bowline.ts") || strings.Contains(out.String(), "bowline.contract.json") {
		t.Fatalf("stdout %q", out.String())
	}
	generated, err := os.ReadFile(filepath.Join(dir, "out", "bowline.ts"))
	if err != nil || !strings.Contains(string(generated), "createClient") {
		t.Fatalf("%v %s", err, generated)
	}
	opts, _, errOut = testOptions(dir)
	if code := Gen(opts, []string{"--from", "nowhere.json"}); code != 1 || !strings.Contains(errOut.String(), "nowhere.json") {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
}
