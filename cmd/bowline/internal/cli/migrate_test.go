package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bowlinedev/bowline/contract"
)

func TestMigrateCommandRewritesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bowline.contract.json")
	os.WriteFile(path, []byte(`{"bowline":"0.1","types":{},"procedures":[]}`), 0o644)
	opts, out, errOut := testOptions(dir)
	if code := Migrate(opts, []string{"bowline.contract.json"}); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "migrated bowline.contract.json to "+contract.Version) {
		t.Fatalf("stdout %q", out.String())
	}
	data, _ := os.ReadFile(path)
	if _, err := contract.Parse(data); err != nil {
		t.Fatal(err)
	}
	opts, out, _ = testOptions(dir)
	Migrate(opts, []string{"bowline.contract.json"})
	if !strings.Contains(out.String(), "unchanged") {
		t.Fatalf("second run stdout %q", out.String())
	}
}

func TestMigrateCommandUsesConfig(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bowline.json"), []byte(`{"entry":"./x.Routes","contract":"api/c.json"}`), 0o644)
	os.MkdirAll(filepath.Join(dir, "api"), 0o755)
	os.WriteFile(filepath.Join(dir, "api", "c.json"), []byte(`{"bowline":"0.1","types":{},"procedures":[]}`), 0o644)
	opts, out, errOut := testOptions(dir)
	if code := Migrate(opts, nil); code != 0 || !strings.Contains(out.String(), "migrated api/c.json") {
		t.Fatalf("exit %d out %q err %q", code, out.String(), errOut.String())
	}
}
