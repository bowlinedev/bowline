package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const routingUsageFile = `{
  "bowline": "1.2",
  "consumer": "web",
  "provider": "routing",
  "interactions": [
    {
      "procedure": "get",
      "method": "GET",
      "input": {"id": 3},
      "response": {"status": 200, "body": {"name": "widget"}}
    }
  ]
}`

type registryFixture struct {
	dir   string
	url   string
	token string
}

func startRegistry(t *testing.T) registryFixture {
	t.Helper()
	dir := fixtureCopy(t)
	if err := os.WriteFile(filepath.Join(dir, "bowline.json"), []byte(`{"entry":"./rows/routing.Routes"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	opts, _, errOut := testOptions(dir)
	stop := make(chan struct{})
	ready := make(chan string, 1)
	opts.Stop = stop
	done := make(chan int, 1)
	go func() {
		done <- RegistryServe(RegistryOptions{
			Options: opts,
			Store:   "registry-data",
			Listen:  "127.0.0.1:0",
			Tokens:  []string{"s3cret"},
			Ready:   ready,
		})
	}()
	addr := <-ready
	t.Cleanup(func() {
		close(stop)
		if code := <-done; code != 0 {
			t.Errorf("registry exited %d: %s", code, errOut.String())
		}
	})
	return registryFixture{dir: dir, url: "http://" + addr, token: "s3cret"}
}

func publishRoutingService(t *testing.T, f registryFixture) {
	t.Helper()
	opts, out, errOut := testOptions(f.dir)
	code := Publish(opts, []string{"--registry", f.url, "--service", "routing", "--token", f.token})
	if code != 0 {
		t.Fatalf("publish service exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "published routing sha256:") || !strings.Contains(out.String(), "tagged main") {
		t.Fatalf("publish output %q", out.String())
	}
}

func publishWebConsumer(t *testing.T, f registryFixture) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(f.dir, "usage.json"), []byte(routingUsageFile), 0o644); err != nil {
		t.Fatal(err)
	}
	opts, out, errOut := testOptions(f.dir)
	code := Publish(opts, []string{"--registry", f.url, "--consumer", "web", "--provider", "routing", "--usage", "usage.json", "--token", f.token})
	if code != 0 {
		t.Fatalf("publish consumer exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "published consumer web of routing") {
		t.Fatalf("publish output %q", out.String())
	}
}

func editRouting(t *testing.T, dir, old, replacement string) {
	t.Helper()
	path := filepath.Join(dir, "rows", "routing", "api.go")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), old) {
		t.Fatalf("fixture does not contain %q", old)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(src), old, replacement, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckRegistryNamesTheAffectedConsumer(t *testing.T) {
	f := startRegistry(t)
	publishRoutingService(t, f)
	publishWebConsumer(t, f)

	opts, out, errOut := testOptions(f.dir)
	if code := Check(opts, []string{"--registry", f.url, "--service", "routing"}); code != 0 {
		t.Fatalf("unchanged contract exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "no contract changes") {
		t.Fatalf("stdout %q", out.String())
	}

	editRouting(t, f.dir, "Name string `json:\"name\"`", "Label string `json:\"label\"`")
	opts, _, errOut = testOptions(f.dir)
	if code := Check(opts, []string{"--registry", f.url, "--service", "routing"}); code != 1 {
		t.Fatalf("breaking change exit %d: %s", code, errOut.String())
	}
	report := errOut.String()
	if !strings.Contains(report, "breaks    web") {
		t.Fatalf("report does not name the consumer: %q", report)
	}
	if !strings.Contains(report, "name") {
		t.Fatalf("report does not mention the removed field: %q", report)
	}
	if !strings.Contains(report, "consumer break(s)") {
		t.Fatalf("report has no summary: %q", report)
	}
}

func TestCheckRegistryStrictFailsOnUnattributedBreakage(t *testing.T) {
	f := startRegistry(t)
	publishRoutingService(t, f)
	publishWebConsumer(t, f)

	editRouting(t, f.dir, "\t\tbowline.Mount(\"admin\", admin),\n", "")

	opts, out, errOut := testOptions(f.dir)
	if code := Check(opts, []string{"--registry", f.url, "--service", "routing"}); code != 0 {
		t.Fatalf("unattributed change without --strict exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "unused    procedure admin.purge") {
		t.Fatalf("stdout %q", out.String())
	}

	opts, _, errOut = testOptions(f.dir)
	if code := Check(opts, []string{"--registry", f.url, "--service", "routing", "--strict"}); code != 1 {
		t.Fatalf("--strict exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "unused    procedure admin.purge") {
		t.Fatalf("strict report %q", errOut.String())
	}
}

func TestPublishReadsTheTokenFromTheEnvironment(t *testing.T) {
	f := startRegistry(t)
	opts, _, errOut := testOptions(f.dir)
	if code := Publish(opts, []string{"--registry", f.url, "--service", "routing"}); code != 1 {
		t.Fatalf("expected an unauthenticated failure, exit %d", code)
	}
	if !strings.Contains(errOut.String(), "UNAUTHENTICATED") {
		t.Fatalf("stderr %q", errOut.String())
	}

	t.Setenv("BOWLINE_REGISTRY_TOKEN", "s3cret")
	opts, out, errOut := testOptions(f.dir)
	if code := Publish(opts, []string{"--registry", f.url, "--service", "routing"}); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "published routing") {
		t.Fatalf("stdout %q", out.String())
	}
}

func TestRegistryServeReadsTokensFromTheEnvironment(t *testing.T) {
	t.Setenv("BOWLINE_REGISTRY_TOKEN", " one , two ")
	if got := envTokens(); len(got) != 2 || got[0] != "one" || got[1] != "two" {
		t.Fatalf("got %v", got)
	}
	t.Setenv("BOWLINE_REGISTRY_TOKEN", "")
	if got := envTokens(); got != nil {
		t.Fatalf("got %v", got)
	}
}

func TestRegistryAndPublishUsage(t *testing.T) {
	dir := t.TempDir()
	for _, args := range [][]string{
		nil,
		{"bogus"},
		{"serve"},
		{"serve", "--store", "data", "extra"},
		{"serve", "--bogus"},
	} {
		opts, _, _ := testOptions(dir)
		if code := Registry(opts, args); code != 2 {
			t.Fatalf("registry %v: exit %d", args, code)
		}
	}
	for _, args := range [][]string{
		nil,
		{"--registry", "http://x"},
		{"--service", "routing"},
		{"--registry", "http://x", "--service", "a", "--consumer", "b"},
		{"--registry", "http://x", "--consumer", "web", "--provider", "routing"},
		{"--registry", "http://x", "--bogus"},
	} {
		opts, _, _ := testOptions(dir)
		if code := Publish(opts, args); code != 2 {
			t.Fatalf("publish %v: exit %d", args, code)
		}
	}
	opts, _, errOut := testOptions(dir)
	if code := Check(opts, []string{"--registry", "http://x"}); code != 2 || !strings.Contains(errOut.String(), "needs --service") {
		t.Fatalf("check exit %d: %s", code, errOut.String())
	}
	opts, _, errOut = testOptions(dir)
	if code := Check(opts, []string{"--strict"}); code != 2 || !strings.Contains(errOut.String(), "need --registry") {
		t.Fatalf("check exit %d: %s", code, errOut.String())
	}
}
