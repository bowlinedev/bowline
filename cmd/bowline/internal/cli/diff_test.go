package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const oldContract = `{"bowline":"1.0","types":{"p.In":{"kind":"struct","name":"In","fields":[{"name":"id","type":{"kind":"primitive","name":"int64"}}]},"p.Out":{"kind":"struct","name":"Out","fields":[{"name":"total","type":{"kind":"primitive","name":"string"}}]}},"errors":{},"procedures":[{"path":"get","kind":"query","method":"GET","input":{"kind":"ref","id":"p.In"},"output":{"kind":"ref","id":"p.Out"}}]}`

const newContract = `{"bowline":"1.0","types":{"p.In":{"kind":"struct","name":"In","fields":[{"name":"id","type":{"kind":"primitive","name":"int64"}}]},"p.Out":{"kind":"struct","name":"Out","fields":[{"name":"amount","type":{"kind":"primitive","name":"string"}}]}},"errors":{},"procedures":[{"path":"get","kind":"query","method":"GET","input":{"kind":"ref","id":"p.In"},"output":{"kind":"ref","id":"p.Out"}}]}`

func writeContracts(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "old.json"), []byte(oldContract), 0o644)
	os.WriteFile(filepath.Join(dir, "new.json"), []byte(newContract), 0o644)
	return dir
}

func TestDiffCommandReportsBreaking(t *testing.T) {
	dir := writeContracts(t)
	opts, out, errOut := testOptions(dir)
	code := DiffCommand(opts, []string{"old.json", "new.json"})
	if code != 1 {
		t.Fatalf("exit %d stderr %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "breaking  procedure get output field total: field removed") || !strings.Contains(out.String(), "added     procedure get output field amount: field added") {
		t.Fatalf("stdout %q", out.String())
	}
}

func TestDiffCommandFormats(t *testing.T) {
	dir := writeContracts(t)
	opts, out, _ := testOptions(dir)
	DiffCommand(opts, []string{"--format", "markdown", "old.json", "new.json"})
	if !strings.Contains(out.String(), "- **breaking** `procedure get output field total`: field removed") {
		t.Fatalf("markdown %q", out.String())
	}
	opts, out, _ = testOptions(dir)
	DiffCommand(opts, []string{"old.json", "new.json", "--format", "json"})
	if !strings.Contains(out.String(), `"category": "breaking"`) {
		t.Fatalf("json %q", out.String())
	}
	opts, _, errOut := testOptions(dir)
	if code := DiffCommand(opts, []string{"old.json", "new.json", "--format", "yaml"}); code != 2 || !strings.Contains(errOut.String(), "unknown format") {
		t.Fatalf("exit %d stderr %q", code, errOut.String())
	}
}

func TestDiffCommandNoChanges(t *testing.T) {
	dir := writeContracts(t)
	opts, out, _ := testOptions(dir)
	if code := DiffCommand(opts, []string{"old.json", "old.json"}); code != 0 || out.String() != "no contract changes\n" {
		t.Fatalf("exit %d stdout %q", code, out.String())
	}
}

func TestDiffCommandUsage(t *testing.T) {
	dir := writeContracts(t)
	opts, _, errOut := testOptions(dir)
	if code := DiffCommand(opts, []string{"old.json"}); code != 2 || !strings.Contains(errOut.String(), "usage:") {
		t.Fatalf("exit %d stderr %q", code, errOut.String())
	}
	opts, _, errOut = testOptions(dir)
	if code := DiffCommand(opts, []string{"old.json", "missing.json"}); code != 1 || errOut.String() == "" {
		t.Fatalf("exit %d stderr %q", code, errOut.String())
	}
}
