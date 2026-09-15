package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitRepo(t *testing.T) string {
	t.Helper()
	dir := fixtureCopy(t)
	os.WriteFile(filepath.Join(dir, "bowline.json"), []byte(`{"entry":"./rows/routing.Routes"}`), 0o644)
	if code := Gen(testOptionsOnly(dir)); code != 0 {
		t.Fatal("gen failed")
	}
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"config", "user.email", "t@example.com"}, {"config", "user.name", "t"}, {"add", "bowline.contract.json"}, {"commit", "-q", "-m", "base"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	return dir
}

func TestCheckAgainstNoChanges(t *testing.T) {
	dir := gitRepo(t)
	opts, out, _ := testOptions(dir)
	if code := Check(opts, []string{"--against", "HEAD"}); code != 0 || !strings.Contains(out.String(), "no contract changes") {
		t.Fatalf("exit %d out %q", code, out.String())
	}
}

func TestCheckAgainstReportsBreaking(t *testing.T) {
	dir := gitRepo(t)
	api := filepath.Join(dir, "rows", "routing", "api.go")
	src, _ := os.ReadFile(api)
	os.WriteFile(api, []byte(strings.Replace(string(src), "Name string `json:\"name\"`", "Label string `json:\"label\"`", 1)), 0o644)
	opts, out, errOut := testOptions(dir)
	code := Check(opts, []string{"--against", "HEAD"})
	if code != 1 || !strings.Contains(out.String(), "breaking") || !strings.Contains(errOut.String(), "breaking") {
		t.Fatalf("exit %d out %q err %q", code, out.String(), errOut.String())
	}
	opts, _, _ = testOptions(dir)
	if code := Check(opts, []string{"--against", "HEAD", "--allow-breaking"}); code != 0 {
		t.Fatalf("allow-breaking exit %d", code)
	}
}

func TestCheckAgainstMissingRef(t *testing.T) {
	dir := gitRepo(t)
	opts, _, errOut := testOptions(dir)
	if code := Check(opts, []string{"--against", "nope"}); code != 1 || !strings.Contains(errOut.String(), "nope") {
		t.Fatalf("exit %d err %q", code, errOut.String())
	}
}

func TestCheckAgainstFromASubdirectory(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "services", "billing")
	os.MkdirAll(filepath.Dir(dir), 0o755)
	os.Rename(fixtureCopy(t), dir)
	os.WriteFile(filepath.Join(dir, "bowline.json"), []byte(`{"entry":"./rows/routing.Routes"}`), 0o644)
	if code := Gen(testOptionsOnly(dir)); code != 0 {
		t.Fatal("gen failed")
	}
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"config", "user.email", "t@example.com"}, {"config", "user.name", "t"}, {"add", "services/billing/bowline.contract.json"}, {"commit", "-q", "-m", "base"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	opts, out, errOut := testOptions(dir)
	if code := Check(opts, []string{"--against", "HEAD"}); code != 0 || !strings.Contains(out.String(), "no contract changes") {
		t.Fatalf("exit %d out %q err %q", code, out.String(), errOut.String())
	}
}
