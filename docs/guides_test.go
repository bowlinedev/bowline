package docs

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var sourceLine = regexp.MustCompile(`(?m)^source: ([^:\s]+):(\d+)-(\d+)$`)

var sketchLine = regexp.MustCompile(`^sketch: (\S.*)$`)

var checkedLanguages = map[string]bool{"go": true, "ts": true, "tsx": true}

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
		checkGuide(t, guide, true)
	}
}

func TestEveryGuideSnippetMatchesItsSource(t *testing.T) {
	guides, err := filepath.Glob(filepath.Join("guides", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(guides) == 0 {
		t.Fatal("no guides found")
	}
	for _, guide := range guides {
		data, err := os.ReadFile(guide)
		if err != nil {
			t.Fatal(err)
		}
		if !sourceLine.MatchString(string(data)) && !strings.Contains(string(data), "\nsource: ") {
			continue
		}
		checkGuide(t, guide, true)
	}
}

func TestDocsSnippetsComeFromSources(t *testing.T) {
	pages := documentationPages(t)
	if len(pages) == 0 {
		t.Fatal("no documentation pages found")
	}
	checked := 0
	for _, page := range pages {
		checked += requireAnnotatedSnippets(t, page)
	}
	if checked == 0 {
		t.Fatal("no go, ts, or tsx snippet found on any page")
	}
}

func documentationPages(t *testing.T) []string {
	t.Helper()
	out, err := exec.Command("git", "ls-files", "--", "*.md").Output()
	if err != nil {
		return walkPages(t)
	}
	var pages []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			pages = append(pages, filepath.FromSlash(line))
		}
	}
	if len(pages) == 0 {
		return walkPages(t)
	}
	return pages
}

func walkPages(t *testing.T) []string {
	t.Helper()
	var pages []string
	err := filepath.WalkDir(".", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(path, ".md") {
			pages = append(pages, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return pages
}

func requireAnnotatedSnippets(t *testing.T, page string) int {
	t.Helper()
	f, err := os.Open(page)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64<<10), 1<<20)
	annotated := false
	inside := false
	language := ""
	opened := 0
	line := 0
	checked := 0
	for scanner.Scan() {
		line++
		text := scanner.Text()
		if inside {
			if strings.HasPrefix(text, "```") {
				inside = false
				if checkedLanguages[language] {
					checked++
					if !annotated {
						t.Errorf("%s:%d: fenced %s block has neither a source: nor a sketch: line", page, opened, language)
					}
				}
				annotated = false
			}
			continue
		}
		switch {
		case strings.HasPrefix(text, "```"):
			inside = true
			language = strings.TrimSpace(strings.TrimPrefix(text, "```"))
			opened = line
		case sourceLine.MatchString(text), sketchLine.MatchString(text):
			annotated = true
		case annotated && strings.TrimSpace(text) == "":
		default:
			annotated = false
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return checked
}

func checkGuide(t *testing.T, guide string, required bool) {
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
	if snippets == 0 && required {
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
