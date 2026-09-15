package docs

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var sourceLine = regexp.MustCompile(`^source: ([^:\s]+):(\d+)-(\d+)$`)

func TestFrameworkGuideSnippetsMatchTheirSources(t *testing.T) {
	guides, err := filepath.Glob(filepath.Join("guides", "frameworks", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(guides) == 0 {
		t.Fatal("no framework guides found")
	}
	for _, guide := range guides {
		if filepath.Base(guide) == "README.md" {
			continue
		}
		checkGuide(t, guide)
	}
}

func checkGuide(t *testing.T, guide string) {
	t.Helper()
	f, err := os.Open(guide)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64<<10), 1<<20)
	var pending []string
	var snippet []string
	inside := false
	snippets := 0
	line := 0
	for scanner.Scan() {
		line++
		text := scanner.Text()
		switch {
		case inside && strings.HasPrefix(text, "```"):
			inside = false
			compare(t, guide, line, pending, snippet)
			pending, snippet = nil, nil
			snippets++
		case inside:
			snippet = append(snippet, text)
		case pending != nil && strings.HasPrefix(text, "```"):
			inside = true
		case pending != nil && strings.TrimSpace(text) == "":
		case sourceLine.MatchString(text):
			pending = sourceLine.FindStringSubmatch(text)
		default:
			if pending != nil {
				t.Errorf("%s:%d: source line must be followed by a fenced block", guide, line)
				pending = nil
			}
		}
	}
	if snippets == 0 {
		t.Errorf("%s: no verified snippet; every framework guide needs at least one source: block", guide)
	}
}

func compare(t *testing.T, guide string, line int, match []string, snippet []string) {
	t.Helper()
	path, start, end := match[1], atoi(match[2]), atoi(match[3])
	data, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(path)))
	if err != nil {
		t.Errorf("%s:%d: %v", guide, line, err)
		return
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if start < 1 || end > len(lines) || start > end {
		t.Errorf("%s:%d: range %d-%d is outside %s (%d lines)", guide, line, start, end, path, len(lines))
		return
	}
	want := strings.Join(lines[start-1:end], "\n")
	got := strings.Join(snippet, "\n")
	if want != got {
		t.Errorf("%s:%d: snippet differs from %s:%d-%d\nwant:\n%s\ngot:\n%s", guide, line, path, start, end, want, got)
	}
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
