package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/bowlinedev/bowline"
)

func TestVersionCommand(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{"version"}, &out, &out)
	if code != 0 {
		t.Fatalf("exit code %d, output %q", code, out.String())
	}
	if got, want := out.String(), "bowline "+bowline.Version+"\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestUnknownCommand(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{"nope"}, &out, &out)
	if code != 2 {
		t.Fatalf("exit code %d, want 2", code)
	}
}

func TestNoArgsPrintsUsage(t *testing.T) {
	var out bytes.Buffer
	code := run(nil, &out, &out)
	if code != 2 {
		t.Fatalf("exit code %d, want 2", code)
	}
	if !bytes.Contains(out.Bytes(), []byte("usage: bowline <command>")) {
		t.Fatalf("usage missing from %q", out.String())
	}
}

var (
	docFlag    = regexp.MustCompile(`--[a-z][a-z-]*`)
	docCommand = regexp.MustCompile("(?m)^### `bowline ([a-z-]+(?: [a-z-]+)?)`")
)

func usageText(t *testing.T) string {
	t.Helper()
	var out bytes.Buffer
	if code := run(nil, &out, &out); code != 2 {
		t.Fatalf("usage run exit %d, want 2", code)
	}
	return out.String()
}

func cliReference(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "cli.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestEveryDocumentedCommandExistsInUsage(t *testing.T) {
	usage := usageText(t)
	reference := cliReference(t)
	matches := docCommand.FindAllStringSubmatch(reference, -1)
	if len(matches) == 0 {
		t.Fatal("docs/cli.md documents no commands")
	}
	for _, match := range matches {
		command := match[1]
		if !strings.Contains(usage, "  "+command+" ") && !strings.Contains(usage, "  "+command+"\n") {
			t.Errorf("docs/cli.md documents %q, which the usage text does not list", command)
		}
	}
}

func TestEveryDocumentedFlagExistsInUsage(t *testing.T) {
	usage := usageText(t)
	documented := map[string]bool{}
	for _, flag := range docFlag.FindAllString(cliReference(t), -1) {
		documented[flag] = true
	}
	if len(documented) == 0 {
		t.Fatal("docs/cli.md documents no flags")
	}
	missing := make([]string, 0, len(documented))
	for flag := range documented {
		if !strings.Contains(usage, flag) {
			missing = append(missing, flag)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("docs/cli.md documents flags the usage text does not mention: %s", strings.Join(missing, " "))
	}
}

func TestEveryUsageCommandIsDocumented(t *testing.T) {
	reference := cliReference(t)
	for _, command := range []string{"gen", "dev", "check", "export openapi", "export tools", "mcp", "verify-consumers", "mock", "eval record", "eval replay", "gateway", "registry serve", "publish", "migrate-contract", "diff", "version"} {
		if !strings.Contains(reference, "### `bowline "+command+"`") {
			t.Errorf("the usage text lists %q, which docs/cli.md does not document", command)
		}
	}
}
