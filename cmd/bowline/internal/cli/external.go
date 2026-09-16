package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/token"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/bowlinedev/bowline/cmd/bowline/internal/analyzer"
	"github.com/bowlinedev/bowline/contract"
)

const (
	multiFileHeader   = "bowline-files/1"
	externalTimeout   = 2 * time.Minute
	diagnosticsStatus = 2
)

type externalGenerator struct {
	name    string
	command string
	env     []string
}

func (g externalGenerator) Generate(doc *contract.Document, out string) ([]byte, error) {
	files, diags, err := g.GenerateFiles(doc, out)
	if err != nil {
		return nil, err
	}
	if len(diags) > 0 {
		return nil, diagnosticsError(diags)
	}
	content, ok := files[out]
	if !ok {
		return nil, fmt.Errorf("%s: wrote no content for %s", g.command, out)
	}
	return content, nil
}

func (g externalGenerator) GenerateFiles(doc *contract.Document, out string) (map[string][]byte, []analyzer.Diagnostic, error) {
	input, err := doc.Marshal()
	if err != nil {
		return nil, nil, err
	}
	binary, err := lookPath(g.command, g.env)
	if err != nil {
		return nil, nil, fmt.Errorf("target %s: %q is not on PATH: %w", g.name, g.command, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), externalTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary)
	cmd.Dir = ""
	cmd.Stdin = bytes.NewReader(input)
	cmd.Env = append(append([]string{}, g.env...),
		"BOWLINE_CONTRACT_VERSION="+doc.Bowline,
		"BOWLINE_OUT="+out,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	if ctx.Err() != nil {
		return nil, nil, fmt.Errorf("target %s: %q did not finish within %s", g.name, g.command, externalTimeout)
	}
	code := 0
	var exitErr *exec.ExitError
	switch {
	case runErr == nil:
	case errors.As(runErr, &exitErr):
		code = exitErr.ExitCode()
	default:
		return nil, nil, fmt.Errorf("target %s: running %q: %w", g.name, g.command, runErr)
	}
	switch code {
	case 0:
		files, err := decodeExternal(stdout.Bytes(), out)
		if err != nil {
			return nil, nil, fmt.Errorf("target %s: %q: %w", g.name, g.command, err)
		}
		return files, nil, nil
	case diagnosticsStatus:
		diags := parseDiagnostics(stderr.String())
		if len(diags) == 0 {
			return nil, nil, fmt.Errorf("target %s: %q exited 2 without writing a diagnostic to stderr", g.name, g.command)
		}
		return nil, diags, nil
	default:
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = "no output on stderr"
		}
		return nil, nil, fmt.Errorf("target %s: %q exited %d: %s", g.name, g.command, code, detail)
	}
}

func lookPath(command string, env []string) (string, error) {
	search := ""
	for _, entry := range env {
		if name, value, ok := strings.Cut(entry, "="); ok && name == "PATH" {
			search = value
		}
	}
	if search == "" {
		return exec.LookPath(command)
	}
	var first error
	for _, dir := range filepath.SplitList(search) {
		if dir == "" {
			continue
		}
		found, err := exec.LookPath(filepath.Join(dir, command))
		if err == nil {
			return found, nil
		}
		if first == nil {
			first = err
		}
	}
	if first == nil {
		first = exec.ErrNotFound
	}
	return "", first
}

func decodeExternal(stdout []byte, out string) (map[string][]byte, error) {
	header, rest, found := bytes.Cut(stdout, []byte("\n"))
	if !found || strings.TrimSpace(string(header)) != multiFileHeader {
		return map[string][]byte{out: stdout}, nil
	}
	var payload struct {
		Files map[string]string `json:"files"`
	}
	dec := json.NewDecoder(bytes.NewReader(rest))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&payload); err != nil {
		return nil, fmt.Errorf("after the %s header, stdout must be {\"files\": {...}}: %w", multiFileHeader, err)
	}
	if len(payload.Files) == 0 {
		return nil, fmt.Errorf("after the %s header, \"files\" is empty", multiFileHeader)
	}
	dir := path.Dir(out)
	files := make(map[string][]byte, len(payload.Files))
	for name, content := range payload.Files {
		clean, err := externalPath(dir, name)
		if err != nil {
			return nil, err
		}
		files[clean] = []byte(content)
	}
	return files, nil
}

func externalPath(dir, name string) (string, error) {
	if name == "" {
		return "", errors.New("a file path in \"files\" is empty")
	}
	if path.IsAbs(name) || strings.HasPrefix(name, "/") {
		return "", fmt.Errorf("file path %q must be relative", name)
	}
	if slices.Contains(strings.Split(name, "/"), "..") {
		return "", fmt.Errorf("file path %q must stay under the target's directory", name)
	}
	joined := name
	if dir != "." && dir != "" {
		joined = path.Join(dir, name)
	}
	return path.Clean(joined), nil
}

func parseDiagnostics(stderr string) []analyzer.Diagnostic {
	var diags []analyzer.Diagnostic
	for _, line := range strings.Split(stderr, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		diags = append(diags, parseDiagnostic(line))
	}
	return diags
}

func parseDiagnostic(line string) analyzer.Diagnostic {
	rest := line
	var pos token.Position
	if file, after, ok := strings.Cut(rest, ":"); ok {
		if lineNo, after2, ok := strings.Cut(after, ":"); ok {
			if col, after3, ok := strings.Cut(after2, ":"); ok {
				n, errLine := strconv.Atoi(strings.TrimSpace(lineNo))
				c, errCol := strconv.Atoi(strings.TrimSpace(col))
				if errLine == nil && errCol == nil {
					pos = token.Position{Filename: file, Line: n, Column: c}
					rest = strings.TrimSpace(after3)
				}
			}
		}
	}
	message := rest
	fix := ""
	if head, tail, ok := strings.Cut(rest, ". "); ok {
		message, fix = head, strings.TrimSpace(tail)
	}
	return analyzer.Diagnostic{Pos: pos, Message: message, Fix: fix}
}

func diagnosticsError(diags []analyzer.Diagnostic) error {
	lines := make([]string, 0, len(diags))
	for _, d := range diags {
		lines = append(lines, d.String())
	}
	return errors.New(strings.Join(lines, "\n"))
}
